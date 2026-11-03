package payments

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/adaptive-dsa/platform/internal/credits"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockCreditsService implements credits.Service for unit tests
type MockCreditsService struct {
	mu           sync.Mutex
	balances     map[uuid.UUID]int32
	grantHistory []credits.LedgerEntry
}

func NewMockCreditsService() *MockCreditsService {
	return &MockCreditsService{
		balances: make(map[uuid.UUID]int32),
	}
}

func (m *MockCreditsService) GetBalance(_ context.Context, userID uuid.UUID) (int32, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.balances[userID], nil
}

func (m *MockCreditsService) GetLedger(_ context.Context, userID uuid.UUID, _, _ int32) ([]credits.LedgerEntry, int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var userEntries []credits.LedgerEntry
	for _, e := range m.grantHistory {
		if e.UserID == userID {
			userEntries = append(userEntries, e)
		}
	}
	return userEntries, int64(len(userEntries)), nil
}

func (m *MockCreditsService) EnsureAccount(_ context.Context, userID uuid.UUID, initialCredits int32) (*credits.CreditAccount, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.balances[userID] = initialCredits
	return &credits.CreditAccount{UserID: userID, CachedBalance: initialCredits}, nil
}

func (m *MockCreditsService) CheckAndReserve(_ context.Context, userID uuid.UUID, requiredCredits int32) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.balances[userID] < requiredCredits {
		return credits.ErrInsufficientCredits
	}
	return nil
}

func (m *MockCreditsService) CommitDeduction(_ context.Context, userID uuid.UUID, amount int32, _ credits.LedgerEntryType, _ string, _ *uuid.UUID, _ string) (*credits.LedgerEntry, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.balances[userID] < amount {
		return nil, credits.ErrInsufficientCredits
	}
	m.balances[userID] -= amount
	entry := credits.LedgerEntry{
		ID:        uuid.New(),
		UserID:    userID,
		Amount:    -amount,
		CreatedAt: time.Now().UTC(),
	}
	m.grantHistory = append(m.grantHistory, entry)
	return &entry, nil
}

func (m *MockCreditsService) GrantPromotionalCredits(_ context.Context, _ uuid.UUID, targetUserID uuid.UUID, amount int32, desc string) (*credits.LedgerEntry, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.balances[targetUserID] += amount
	entry := credits.LedgerEntry{
		ID:          uuid.New(),
		UserID:      targetUserID,
		Amount:      amount,
		EntryType:   credits.EntryTypePromotionalGrant,
		Description: desc,
		CreatedAt:   time.Now().UTC(),
	}
	m.grantHistory = append(m.grantHistory, entry)
	return &entry, nil
}

func (m *MockCreditsService) GrantPurchasedCredits(_ context.Context, userID uuid.UUID, amount int32, paymentOrderID uuid.UUID, desc string) (*credits.LedgerEntry, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.balances[userID] += amount
	entry := credits.LedgerEntry{
		ID:            uuid.New(),
		UserID:        userID,
		Amount:        amount,
		EntryType:     credits.EntryTypePurchase,
		ReferenceType: "RAZORPAY_PAYMENT",
		ReferenceID:   &paymentOrderID,
		Description:   desc,
		CreatedAt:     time.Now().UTC(),
	}
	m.grantHistory = append(m.grantHistory, entry)
	return &entry, nil
}

func (m *MockCreditsService) RefundCredits(_ context.Context, targetUserID uuid.UUID, amount int32, desc string, _ *uuid.UUID) (*credits.LedgerEntry, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.balances[targetUserID] += amount
	return &credits.LedgerEntry{ID: uuid.New(), UserID: targetUserID, Amount: amount, Description: desc}, nil
}

func (m *MockCreditsService) ReconcileBalance(_ context.Context, userID uuid.UUID) (bool, int32, int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var sum int64
	for _, e := range m.grantHistory {
		if e.UserID == userID {
			sum += int64(e.Amount)
		}
	}
	cached := m.balances[userID]
	return int64(cached) == sum, cached, sum, nil
}

// In-memory mock repository for service unit tests
type MockRepository struct {
	mu            sync.Mutex
	orders        map[uuid.UUID]*PaymentOrder
	ordersByRP    map[string]*PaymentOrder
	ordersByIdemp map[string]*PaymentOrder
	webhookEvents map[string]*WebhookEvent
	jobs          []Job
}

func NewMockRepository() *MockRepository {
	return &MockRepository{
		orders:        make(map[uuid.UUID]*PaymentOrder),
		ordersByRP:    make(map[string]*PaymentOrder),
		ordersByIdemp: make(map[string]*PaymentOrder),
		webhookEvents: make(map[string]*WebhookEvent),
	}
}

func (m *MockRepository) CreateOrder(_ context.Context, order *PaymentOrder) (*PaymentOrder, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if order.ID == uuid.Nil {
		order.ID = uuid.New()
	}
	now := time.Now().UTC()
	order.CreatedAt = now
	order.UpdatedAt = now
	m.orders[order.ID] = order
	if order.RazorpayOrderID != "" {
		m.ordersByRP[order.RazorpayOrderID] = order
	}
	if order.IdempotencyKey != "" {
		m.ordersByIdemp[order.IdempotencyKey] = order
	}
	return order, nil
}

func (m *MockRepository) GetOrderByID(_ context.Context, id uuid.UUID) (*PaymentOrder, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	o, ok := m.orders[id]
	if !ok {
		return nil, ErrOrderNotFound
	}
	return o, nil
}

func (m *MockRepository) GetOrderByIDForUpdate(ctx context.Context, id uuid.UUID) (*PaymentOrder, error) {
	return m.GetOrderByID(ctx, id)
}

func (m *MockRepository) GetOrderByRazorpayOrderID(_ context.Context, rpOrderID string) (*PaymentOrder, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	o, ok := m.ordersByRP[rpOrderID]
	if !ok {
		return nil, ErrOrderNotFound
	}
	return o, nil
}

func (m *MockRepository) GetOrderByRazorpayOrderIDForUpdate(ctx context.Context, rpOrderID string) (*PaymentOrder, error) {
	return m.GetOrderByRazorpayOrderID(ctx, rpOrderID)
}

func (m *MockRepository) GetOrderByIdempotencyKey(_ context.Context, key string) (*PaymentOrder, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	o, ok := m.ordersByIdemp[key]
	if !ok {
		return nil, ErrOrderNotFound
	}
	return o, nil
}

func (m *MockRepository) UpdateOrder(_ context.Context, order *PaymentOrder) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	order.UpdatedAt = time.Now().UTC()
	m.orders[order.ID] = order
	if order.RazorpayOrderID != "" {
		m.ordersByRP[order.RazorpayOrderID] = order
	}
	return nil
}

func (m *MockRepository) ListOrdersByUser(_ context.Context, userID uuid.UUID, _, _ int32) ([]PaymentOrder, int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var res []PaymentOrder
	for _, o := range m.orders {
		if o.UserID == userID {
			res = append(res, *o)
		}
	}
	return res, int64(len(res)), nil
}

func (m *MockRepository) ListAllOrders(_ context.Context, _, _ int32) ([]PaymentOrder, int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var res []PaymentOrder
	for _, o := range m.orders {
		res = append(res, *o)
	}
	return res, int64(len(res)), nil
}

func (m *MockRepository) RecordWebhookEvent(_ context.Context, event *WebhookEvent) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, exists := m.webhookEvents[event.EventID]; exists {
		return false, nil
	}
	if event.ID == uuid.Nil {
		event.ID = uuid.New()
	}
	event.CreatedAt = time.Now().UTC()
	m.webhookEvents[event.EventID] = event
	return true, nil
}

func (m *MockRepository) UpdateWebhookEventStatus(_ context.Context, eventID string, status string, errMsg string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if e, ok := m.webhookEvents[eventID]; ok {
		e.Status = status
		now := time.Now().UTC()
		e.ProcessedAt = &now
		if errMsg != "" {
			e.ErrorMessage = errMsg
		}
	}
	return nil
}

func (m *MockRepository) EnqueueJob(_ context.Context, jobType string, payload any) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	b, _ := json.Marshal(payload)
	now := time.Now().UTC()
	m.jobs = append(m.jobs, Job{
		ID:          uuid.New(),
		JobType:     jobType,
		Payload:     b,
		Status:      "PENDING",
		RetryCount:  0,
		MaxRetries:  3,
		ScheduledAt: now,
		CreatedAt:   now,
		UpdatedAt:   now,
	})
	return nil
}

func (m *MockRepository) FetchPendingJobs(_ context.Context, limit int) ([]Job, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var claimed []Job
	now := time.Now().UTC()
	staleThreshold := now.Add(-5 * time.Minute)

	for i := range m.jobs {
		isPending := m.jobs[i].Status == "PENDING" && !m.jobs[i].ScheduledAt.After(now)
		isStaleRunning := m.jobs[i].Status == "RUNNING" && m.jobs[i].UpdatedAt.Before(staleThreshold)

		if isPending || isStaleRunning {
			m.jobs[i].Status = "RUNNING"
			m.jobs[i].UpdatedAt = now
			claimed = append(claimed, m.jobs[i])
			if len(claimed) >= limit {
				break
			}
		}
	}
	return claimed, nil
}

func (m *MockRepository) UpdateJob(_ context.Context, job *Job) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i, j := range m.jobs {
		if j.ID == job.ID {
			m.jobs[i] = *job
			return nil
		}
	}
	return nil
}

func (m *MockRepository) GetJobStats(_ context.Context) (map[string]int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	stats := map[string]int64{
		"PENDING":   0,
		"RUNNING":   0,
		"COMPLETED": 0,
		"FAILED":    0,
	}
	for _, j := range m.jobs {
		stats[j.Status]++
	}
	return stats, nil
}

func (m *MockRepository) ExecTx(_ context.Context, fn func(Repository) error) error {
	return fn(m)
}

func TestPayments_OrderCreation_BackendPricingEnforcement(t *testing.T) {
	repo := NewMockRepository()
	provider := NewMockPaymentProvider()
	creditsSvc := NewMockCreditsService()
	svc := NewService(repo, provider, creditsSvc, nil)

	ctx := context.Background()
	userID := uuid.New()

	// 1. Valid package selection
	res, err := svc.CreatePaymentOrder(ctx, userID, "pkg_pro", "")
	require.NoError(t, err)
	assert.Equal(t, int32(24900), res.AmountPaise) // ₹249.00
	assert.Equal(t, int32(150), res.CreditsAmount)
	assert.Equal(t, "INR", res.Currency)
	assert.Equal(t, "pkg_pro", res.PackageID)
	assert.NotEmpty(t, res.RazorpayOrderID)

	// 2. Invalid package rejection (client manipulation)
	_, err = svc.CreatePaymentOrder(ctx, userID, "pkg_non_existent", "")
	require.ErrorIs(t, err, ErrInvalidPackage)

	// 3. Idempotent repeated order creation
	idempKey := "key_test_12345"
	res1, err := svc.CreatePaymentOrder(ctx, userID, "pkg_starter", idempKey)
	require.NoError(t, err)
	res2, err := svc.CreatePaymentOrder(ctx, userID, "pkg_starter", idempKey)
	require.NoError(t, err)
	assert.Equal(t, res1.OrderID, res2.OrderID)
	assert.Equal(t, res1.RazorpayOrderID, res2.RazorpayOrderID)
}

func TestPayments_VerifyCheckout_SignatureValidationAndCreditGrant(t *testing.T) {
	repo := NewMockRepository()
	provider := NewMockPaymentProvider()
	creditsSvc := NewMockCreditsService()
	svc := NewService(repo, provider, creditsSvc, nil)

	ctx := context.Background()
	userID := uuid.New()

	// Create order for Pro pack (150 credits)
	orderRes, err := svc.CreatePaymentOrder(ctx, userID, "pkg_pro", "")
	require.NoError(t, err)

	paymentID := "pay_mock_test_12345"

	// 1. Invalid signature rejection
	badReq := VerifyCheckoutRequest{
		RazorpayOrderID:   orderRes.RazorpayOrderID,
		RazorpayPaymentID: paymentID,
		RazorpaySignature: "invalid_tampered_signature",
	}
	_, err = svc.VerifyCheckout(ctx, userID, badReq)
	require.ErrorIs(t, err, ErrInvalidSignature)
	bal, _ := creditsSvc.GetBalance(ctx, userID)
	assert.Equal(t, int32(0), bal) // Zero credits granted on invalid signature

	// 2. Reject payment if provider reports status "authorized" but not yet captured
	validSig := provider.ComputeCheckoutSignature(orderRes.RazorpayOrderID, paymentID)
	goodReq := VerifyCheckoutRequest{
		RazorpayOrderID:   orderRes.RazorpayOrderID,
		RazorpayPaymentID: paymentID,
		RazorpaySignature: validSig,
	}

	provider.MockPaymentStatus = "authorized"
	_, err = svc.VerifyCheckout(ctx, userID, goodReq)
	require.ErrorIs(t, err, ErrPaymentNotCaptured, "Must reject checkout verification if payment is only authorized and not captured")
	bal, _ = creditsSvc.GetBalance(ctx, userID)
	assert.Equal(t, int32(0), bal)

	// 3. Valid signature verification and confirmed captured status -> fulfill credits
	provider.MockPaymentStatus = "captured"
	verifiedOrder, err := svc.VerifyCheckout(ctx, userID, goodReq)
	require.NoError(t, err)
	assert.Equal(t, StatusPaid, verifiedOrder.Status)

	bal, _ = creditsSvc.GetBalance(ctx, userID)
	assert.Equal(t, int32(150), bal) // Exactly 150 credits granted

	// 4. Idempotent duplicate verification call produces no duplicate credits
	duplicateOrder, err := svc.VerifyCheckout(ctx, userID, goodReq)
	require.NoError(t, err)
	assert.Equal(t, StatusPaid, duplicateOrder.Status)
	balAfterDuplicate, _ := creditsSvc.GetBalance(ctx, userID)
	assert.Equal(t, int32(150), balAfterDuplicate) // Remains strictly 150 credits
}

func TestPayments_WebhookProcessing_Captured_And_DuplicateIdempotency(t *testing.T) {
	repo := NewMockRepository()
	provider := NewMockPaymentProvider()
	creditsSvc := NewMockCreditsService()
	svc := NewService(repo, provider, creditsSvc, nil)

	ctx := context.Background()
	userID := uuid.New()

	// Create Master pack order (400 credits - ₹599 = 59900 paise)
	orderRes, err := svc.CreatePaymentOrder(ctx, userID, "pkg_master", "")
	require.NoError(t, err)

	paymentID := "pay_rp_webhook_99999"
	officialEventID := "evt_official_rzp_999999"

	webhookPayload := map[string]interface{}{
		"entity":     "event",
		"account_id": "acc_test",
		"event":      "payment.captured",
		"contains":   []string{"payment"},
		"payload": map[string]interface{}{
			"payment": map[string]interface{}{
				"entity": map[string]interface{}{
					"id":       paymentID,
					"amount":   59900,
					"currency": "INR",
					"status":   "captured",
					"order_id": orderRes.RazorpayOrderID,
					"captured": true,
				},
			},
		},
		"created_at": 1700000000,
	}
	rawBytes, _ := json.Marshal(webhookPayload)

	// 1. Invalid webhook signature rejection
	_, err = svc.ProcessWebhook(ctx, rawBytes, "bad_webhook_sig", officialEventID)
	require.ErrorIs(t, err, ErrInvalidWebhookSignature)
	bal, _ := creditsSvc.GetBalance(ctx, userID)
	assert.Equal(t, int32(0), bal)

	// 2. Valid webhook signature processing with x-razorpay-event-id header
	validSig := provider.ComputeWebhookSignature(rawBytes)
	res, err := svc.ProcessWebhook(ctx, rawBytes, validSig, officialEventID)
	require.NoError(t, err)
	assert.Equal(t, "PROCESSED", res.Status)
	assert.Equal(t, officialEventID, res.EventID)

	bal, _ = creditsSvc.GetBalance(ctx, userID)
	assert.Equal(t, int32(400), bal) // 400 credits granted

	// 3. Repeated webhook delivery with identical event ID header
	duplicateRes, err := svc.ProcessWebhook(ctx, rawBytes, validSig, officialEventID)
	require.NoError(t, err)
	assert.Equal(t, "DUPLICATE_IGNORED", duplicateRes.Status)

	balAfterDup, _ := creditsSvc.GetBalance(ctx, userID)
	assert.Equal(t, int32(400), balAfterDup) // Exactly-once guarantee: balance remains 400
}

func TestPayments_WebhookProcessing_AmountMismatchRejection(t *testing.T) {
	repo := NewMockRepository()
	provider := NewMockPaymentProvider()
	creditsSvc := NewMockCreditsService()
	svc := NewService(repo, provider, creditsSvc, nil)

	ctx := context.Background()
	userID := uuid.New()

	// Create Starter pack order (50 credits - 9900 paise)
	orderRes, err := svc.CreatePaymentOrder(ctx, userID, "pkg_starter", "")
	require.NoError(t, err)

	// Webhook payload attempting to claim payment with mismatched amount (1000 paise instead of 9900)
	tamperedPayload := map[string]interface{}{
		"entity":     "event",
		"account_id": "acc_test",
		"event":      "payment.captured",
		"contains":   []string{"payment"},
		"payload": map[string]interface{}{
			"payment": map[string]interface{}{
				"entity": map[string]interface{}{
					"id":       "pay_tampered_123",
					"amount":   1000, // Mismatched!
					"currency": "INR",
					"status":   "captured",
					"order_id": orderRes.RazorpayOrderID,
					"captured": true,
				},
			},
		},
		"created_at": 1700000001,
	}
	rawBytes, _ := json.Marshal(tamperedPayload)
	sig := provider.ComputeWebhookSignature(rawBytes)

	_, err = svc.ProcessWebhook(ctx, rawBytes, sig, "")
	require.ErrorIs(t, err, ErrMismatchedPaymentData)

	bal, _ := creditsSvc.GetBalance(ctx, userID)
	assert.Equal(t, int32(0), bal) // 0 credits granted
}

func TestPayments_WebhookProcessing_OutOfOrderEvents(t *testing.T) {
	repo := NewMockRepository()
	provider := NewMockPaymentProvider()
	creditsSvc := NewMockCreditsService()
	svc := NewService(repo, provider, creditsSvc, nil)

	ctx := context.Background()
	userID := uuid.New()

	// 1. Out-of-order: payment.captured arrives first, followed by delayed payment.failed
	order1, err := svc.CreatePaymentOrder(ctx, userID, "pkg_starter", "")
	require.NoError(t, err)

	capturedPayload := map[string]interface{}{
		"event": "payment.captured",
		"payload": map[string]interface{}{
			"payment": map[string]interface{}{
				"entity": map[string]interface{}{
					"id":       "pay_captured_first",
					"amount":   9900,
					"currency": "INR",
					"status":   "captured",
					"order_id": order1.RazorpayOrderID,
				},
			},
		},
		"created_at": 1700000010,
	}
	capBytes, _ := json.Marshal(capturedPayload)
	capSig := provider.ComputeWebhookSignature(capBytes)

	res, err := svc.ProcessWebhook(ctx, capBytes, capSig, "evt_cap_first")
	require.NoError(t, err)
	assert.Equal(t, "PROCESSED", res.Status)

	bal, _ := creditsSvc.GetBalance(ctx, userID)
	assert.Equal(t, int32(50), bal)

	// Delayed failure arrives for the same order (e.g. out of order delivery from earlier declined attempt)
	failedPayload := map[string]interface{}{
		"event": "payment.failed",
		"payload": map[string]interface{}{
			"payment": map[string]interface{}{
				"entity": map[string]interface{}{
					"id":       "pay_declined_earlier",
					"amount":   9900,
					"currency": "INR",
					"order_id": order1.RazorpayOrderID,
				},
			},
		},
		"created_at": 1700000005,
	}
	failBytes, _ := json.Marshal(failedPayload)
	failSig := provider.ComputeWebhookSignature(failBytes)

	resFail, err := svc.ProcessWebhook(ctx, failBytes, failSig, "evt_fail_delayed")
	require.NoError(t, err)
	assert.Equal(t, "PROCESSED", resFail.Status)

	// Invariant check: Stale failure must NEVER downgrade a PAID order or revoke credits!
	savedOrder, _ := repo.GetOrderByRazorpayOrderID(ctx, order1.RazorpayOrderID)
	assert.Equal(t, StatusPaid, savedOrder.Status, "Order must remain PAID despite stale failure event")

	balAfterStaleFail, _ := creditsSvc.GetBalance(ctx, userID)
	assert.Equal(t, int32(50), balAfterStaleFail, "Balance must remain 50 credits")

	// Delayed authorization arrives for the same paid order
	authPayload := map[string]interface{}{
		"event": "payment.authorized",
		"payload": map[string]interface{}{
			"payment": map[string]interface{}{
				"entity": map[string]interface{}{
					"id":       "pay_auth_delayed",
					"amount":   9900,
					"currency": "INR",
					"order_id": order1.RazorpayOrderID,
				},
			},
		},
		"created_at": 1700000008,
	}
	authBytes, _ := json.Marshal(authPayload)
	authSig := provider.ComputeWebhookSignature(authBytes)

	resAuth, err := svc.ProcessWebhook(ctx, authBytes, authSig, "evt_auth_delayed")
	require.NoError(t, err)
	assert.Equal(t, "PROCESSED", resAuth.Status)

	savedOrder2, _ := repo.GetOrderByRazorpayOrderID(ctx, order1.RazorpayOrderID)
	assert.Equal(t, StatusPaid, savedOrder2.Status, "Order must remain PAID despite stale authorization event")
	balAfterStaleAuth, _ := creditsSvc.GetBalance(ctx, userID)
	assert.Equal(t, int32(50), balAfterStaleAuth)

	// 2. Out-of-order: initial failure followed by successful capture on retry
	order2, err := svc.CreatePaymentOrder(ctx, userID, "pkg_pro", "")
	require.NoError(t, err)

	failedAttemptPayload := map[string]interface{}{
		"event": "payment.failed",
		"payload": map[string]interface{}{
			"payment": map[string]interface{}{
				"entity": map[string]interface{}{
					"id":       "pay_retry_failed_first",
					"amount":   24900,
					"currency": "INR",
					"order_id": order2.RazorpayOrderID,
				},
			},
		},
		"created_at": 1700000020,
	}
	fail2Bytes, _ := json.Marshal(failedAttemptPayload)
	fail2Sig := provider.ComputeWebhookSignature(fail2Bytes)

	_, err = svc.ProcessWebhook(ctx, fail2Bytes, fail2Sig, "evt_fail_retry_1")
	require.NoError(t, err)
	savedOrderFailed, _ := repo.GetOrderByRazorpayOrderID(ctx, order2.RazorpayOrderID)
	assert.Equal(t, StatusFailed, savedOrderFailed.Status)

	// Successful captured webhook arrives after retry
	successAttemptPayload := map[string]interface{}{
		"event": "payment.captured",
		"payload": map[string]interface{}{
			"payment": map[string]interface{}{
				"entity": map[string]interface{}{
					"id":       "pay_retry_success",
					"amount":   24900,
					"currency": "INR",
					"status":   "captured",
					"order_id": order2.RazorpayOrderID,
				},
			},
		},
		"created_at": 1700000025,
	}
	successBytes, _ := json.Marshal(successAttemptPayload)
	successSig := provider.ComputeWebhookSignature(successBytes)

	resSuccess, err := svc.ProcessWebhook(ctx, successBytes, successSig, "evt_success_retry")
	require.NoError(t, err)
	assert.Equal(t, "PROCESSED", resSuccess.Status)

	savedOrderSuccess, _ := repo.GetOrderByRazorpayOrderID(ctx, order2.RazorpayOrderID)
	assert.Equal(t, StatusPaid, savedOrderSuccess.Status, "Order should transition from FAILED to PAID upon confirmed capture")

	// Total balance: 50 (from order 1) + 150 (from order 2) = 200 credits
	balFinal, _ := creditsSvc.GetBalance(ctx, userID)
	assert.Equal(t, int32(200), balFinal)
}

func TestPayments_BackgroundWorker_RetryAndCompletion(t *testing.T) {
	repo := NewMockRepository()
	worker := NewWorker(repo, nil)

	ctx := context.Background()

	// 1. Enqueue test job and verify normal execution
	err := repo.EnqueueJob(ctx, JobTypePurchaseConfirmation, map[string]string{"order_id": "123"})
	require.NoError(t, err)

	// Process batch with registered default handler
	processed, err := worker.ProcessBatch(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, processed)

	// Verify job is now completed
	assert.Equal(t, "COMPLETED", repo.jobs[0].Status)
	assert.Nil(t, repo.jobs[0].LastError)

	// 2. Test worker crash recovery (stale RUNNING job)
	staleJobID := uuid.New()
	repo.jobs = append(repo.jobs, Job{
		ID:          staleJobID,
		JobType:     JobTypePurchaseConfirmation,
		Payload:     []byte(`{"order_id":"crashed_worker"}`),
		Status:      "RUNNING", // Stranded in RUNNING because previous worker crashed
		RetryCount:  0,
		MaxRetries:  3,
		ScheduledAt: time.Now().UTC().Add(-10 * time.Minute),
		CreatedAt:   time.Now().UTC().Add(-10 * time.Minute),
		UpdatedAt:   time.Now().UTC().Add(-6 * time.Minute), // Stale (> 5 minutes ago)
	})

	processedStale, err := worker.ProcessBatch(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, processedStale, "Stale RUNNING job should be reclaimed and completed")
	assert.Equal(t, "COMPLETED", repo.jobs[1].Status)

	// 3. Test failing job handler with retry and permanent failure
	failJobType := "TEST_FAIL_JOB"
	attempts := 0
	worker.RegisterHandler(failJobType, func(_ context.Context, _ Job) error {
		attempts++
		return fmt.Errorf("persistent database error")
	})

	_ = repo.EnqueueJob(ctx, failJobType, map[string]string{"foo": "bar"})
	jobIdx := 2

	// Attempt 1: Fails, scheduled for retry
	_, _ = worker.ProcessBatch(ctx)
	assert.Equal(t, "PENDING", repo.jobs[jobIdx].Status)
	assert.Equal(t, int32(1), repo.jobs[jobIdx].RetryCount)
	assert.NotNil(t, repo.jobs[jobIdx].LastError)

	repo.jobs[jobIdx].ScheduledAt = time.Now().UTC().Add(-1 * time.Second)

	// Attempt 2: Fails, scheduled for retry
	_, _ = worker.ProcessBatch(ctx)
	assert.Equal(t, "PENDING", repo.jobs[jobIdx].Status)
	assert.Equal(t, int32(2), repo.jobs[jobIdx].RetryCount)

	repo.jobs[jobIdx].ScheduledAt = time.Now().UTC().Add(-1 * time.Second)

	// Attempt 3: Max retries (3) reached -> becomes FAILED permanently
	_, _ = worker.ProcessBatch(ctx)
	assert.Equal(t, "FAILED", repo.jobs[jobIdx].Status)
	assert.Equal(t, int32(3), repo.jobs[jobIdx].RetryCount)
}
