package payments_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/adaptive-dsa/platform/internal/config"
	"github.com/adaptive-dsa/platform/internal/credits"
	"github.com/adaptive-dsa/platform/internal/payments"
	"github.com/adaptive-dsa/platform/internal/platform/database"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupPostgresPayments(t *testing.T) (*database.DB, payments.Repository, credits.Service, payments.Service, *payments.MockPaymentProvider, uuid.UUID) {
	t.Helper()

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://postgres@127.0.0.1:5432/adaptive_dsa_test?sslmode=disable"
	}

	cfg := config.DatabaseConfig{
		URL:               dbURL,
		MaxConns:          25,
		MinConns:          5,
		MaxConnLifetime:   5 * time.Minute,
		MaxConnIdleTime:   1 * time.Minute,
		HealthCheckPeriod: 30 * time.Second,
		ConnectTimeout:    5 * time.Second,
	}

	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	db, err := database.New(ctx, cfg, logger)
	if err != nil {
		t.Skipf("skipping postgres integration test: database unavailable: %v", err)
	}

	testUserID := uuid.New()
	email := fmt.Sprintf("pay_test_%s@example.com", testUserID.String()[:8])
	_, err = db.Pool().Exec(ctx, `
		INSERT INTO users (id, email, password_hash, full_name, role)
		VALUES ($1, $2, 'dummy_hash', 'Payments Test User', 'CANDIDATE')
	`, testUserID, email)
	require.NoError(t, err)

	t.Cleanup(func() {
		_, _ = db.Pool().Exec(context.Background(), "DELETE FROM payment_orders WHERE user_id = $1", testUserID)
		_, _ = db.Pool().Exec(context.Background(), "DELETE FROM credit_ledger WHERE user_id = $1", testUserID)
		_, _ = db.Pool().Exec(context.Background(), "DELETE FROM credit_accounts WHERE user_id = $1", testUserID)
		_, _ = db.Pool().Exec(context.Background(), "DELETE FROM users WHERE id = $1", testUserID)
		_, _ = db.Pool().Exec(context.Background(), "DELETE FROM background_jobs WHERE job_type LIKE 'TEST_%'")
		db.Close()
	})

	creditsRepo := credits.NewPostgresRepository(db.Pool())
	creditsSvc := credits.NewService(creditsRepo)

	// Ensure credit account exists for user
	_, err = creditsSvc.EnsureAccount(ctx, testUserID, 0)
	require.NoError(t, err)

	paymentsRepo := payments.NewPostgresRepository(db.Pool())
	mockProvider := payments.NewMockPaymentProvider()
	paymentsSvc := payments.NewService(paymentsRepo, mockProvider, creditsSvc, logger)

	return db, paymentsRepo, creditsSvc, paymentsSvc, mockProvider, testUserID
}

func TestPayments_Postgres_ConcurrentCheckoutAndWebhook_ExactlyOnceCreditGrant(t *testing.T) {
	ctx := context.Background()
	_, _, creditsSvc, paymentsSvc, provider, userID := setupPostgresPayments(t)

	// 1. Create payment order for pkg_pro (150 credits, 24900 paise)
	res, err := paymentsSvc.CreatePaymentOrder(ctx, userID, "pkg_pro", "idempotency_order_pro_1")
	require.NoError(t, err)
	require.NotEmpty(t, res.RazorpayOrderID)

	rpOrderID := res.RazorpayOrderID
	rpPaymentID := fmt.Sprintf("pay_concurrent_%s", uuid.New().String()[:8])

	checkoutSig := provider.ComputeCheckoutSignature(rpOrderID, rpPaymentID)

	webhookPayload := map[string]any{
		"event": "payment.captured",
		"payload": map[string]any{
			"payment": map[string]any{
				"entity": map[string]any{
					"id":       rpPaymentID,
					"order_id": rpOrderID,
					"amount":   24900,
					"currency": "INR",
					"status":   "captured",
				},
			},
			"order": map[string]any{
				"entity": map[string]any{
					"id":          rpOrderID,
					"amount":      24900,
					"amount_paid": 24900,
					"status":      "paid",
				},
			},
		},
	}
	webhookBytes, err := json.Marshal(webhookPayload)
	require.NoError(t, err)
	webhookSig := provider.ComputeWebhookSignature(webhookBytes)

	// Concurrently invoke:
	// Goroutine 1: VerifyCheckout
	// Goroutine 2: ProcessWebhook
	// Goroutine 3: Repeated VerifyCheckout
	// Goroutine 4: Repeated ProcessWebhook
	var wg sync.WaitGroup
	var successCheckoutCount int32
	var successWebhookCount int32

	concurrency := 6
	wg.Add(concurrency)

	for i := 0; i < concurrency; i++ {
		go func(idx int) {
			defer wg.Done()
			if idx%2 == 0 {
				order, err := paymentsSvc.VerifyCheckout(ctx, userID, payments.VerifyCheckoutRequest{
					RazorpayOrderID:   rpOrderID,
					RazorpayPaymentID: rpPaymentID,
					RazorpaySignature: checkoutSig,
				})
				if err == nil && order.Status == payments.StatusPaid {
					atomic.AddInt32(&successCheckoutCount, 1)
				}
			} else {
				wRes, err := paymentsSvc.ProcessWebhook(ctx, webhookBytes, webhookSig, fmt.Sprintf("evt_concurrent_%s", rpPaymentID))
				if err == nil && (wRes.Status == "PROCESSED" || wRes.Status == "DUPLICATE_IGNORED") {
					atomic.AddInt32(&successWebhookCount, 1)
				}
			}
		}(i)
	}

	wg.Wait()

	// Balance MUST be exactly 150 credits!
	bal, err := creditsSvc.GetBalance(ctx, userID)
	require.NoError(t, err)
	assert.Equal(t, int32(150), bal, "Balance must be exactly 150 credits after concurrent checkout & webhook")

	// Ledger reconciliation check: cached_balance == SUM(ledger)
	reconciled, cached, sum, err := creditsSvc.ReconcileBalance(ctx, userID)
	require.NoError(t, err)
	assert.True(t, reconciled, "Credit balance must be reconciled with ledger")
	assert.Equal(t, int32(150), cached)
	assert.Equal(t, int64(150), sum)
}

func TestPayments_Postgres_ConcurrentDuplicateWebhooks_Idempotency(t *testing.T) {
	ctx := context.Background()
	_, _, creditsSvc, paymentsSvc, provider, userID := setupPostgresPayments(t)

	// Create payment order for pkg_master (400 credits, 59900 paise)
	res, err := paymentsSvc.CreatePaymentOrder(ctx, userID, "pkg_master", "")
	require.NoError(t, err)

	rpOrderID := res.RazorpayOrderID
	rpPaymentID := fmt.Sprintf("pay_dup_%s", uuid.New().String()[:8])
	officialEventID := fmt.Sprintf("evt_official_%s", uuid.New().String()[:10])

	webhookPayload := map[string]any{
		"event": "payment.captured",
		"payload": map[string]any{
			"payment": map[string]any{
				"entity": map[string]any{
					"id":       rpPaymentID,
					"order_id": rpOrderID,
					"amount":   59900,
					"currency": "INR",
					"status":   "captured",
				},
			},
		},
	}
	webhookBytes, err := json.Marshal(webhookPayload)
	require.NoError(t, err)
	webhookSig := provider.ComputeWebhookSignature(webhookBytes)

	// Deliver identical webhook 10 times concurrently with X-Razorpay-Event-Id header
	concurrency := 10
	var wg sync.WaitGroup
	var processedCount int32
	var duplicateCount int32

	wg.Add(concurrency)
	for i := 0; i < concurrency; i++ {
		go func() {
			defer wg.Done()
			wRes, err := paymentsSvc.ProcessWebhook(ctx, webhookBytes, webhookSig, officialEventID)
			if err != nil {
				return
			}
			if wRes.Status == "PROCESSED" {
				atomic.AddInt32(&processedCount, 1)
			} else if wRes.Status == "DUPLICATE_IGNORED" {
				atomic.AddInt32(&duplicateCount, 1)
			}
		}()
	}
	wg.Wait()

	assert.Equal(t, int32(1), processedCount, "Exactly 1 webhook delivery should be marked PROCESSED")
	assert.Equal(t, int32(concurrency-1), duplicateCount, "All other concurrent deliveries must be DUPLICATE_IGNORED")

	// Verify credits were granted exactly once (400 credits)
	bal, err := creditsSvc.GetBalance(ctx, userID)
	require.NoError(t, err)
	assert.Equal(t, int32(400), bal)
}

func TestPayments_Postgres_Checkout_RejectNonCapturedStatus(t *testing.T) {
	ctx := context.Background()
	_, _, creditsSvc, paymentsSvc, provider, userID := setupPostgresPayments(t)

	orderRes, err := paymentsSvc.CreatePaymentOrder(ctx, userID, "pkg_starter", "")
	require.NoError(t, err)

	paymentID := "pay_auth_only_999"
	sig := provider.ComputeCheckoutSignature(orderRes.RazorpayOrderID, paymentID)

	// Provider returns status "authorized" (not captured)
	provider.MockPaymentStatus = "authorized"
	_, err = paymentsSvc.VerifyCheckout(ctx, userID, payments.VerifyCheckoutRequest{
		RazorpayOrderID:   orderRes.RazorpayOrderID,
		RazorpayPaymentID: paymentID,
		RazorpaySignature: sig,
	})
	require.ErrorIs(t, err, payments.ErrPaymentNotCaptured)

	// Ensure balance remains 0
	bal, err := creditsSvc.GetBalance(ctx, userID)
	require.NoError(t, err)
	assert.Equal(t, int32(0), bal)

	// Once captured, checkout succeeds
	provider.MockPaymentStatus = "captured"
	verified, err := paymentsSvc.VerifyCheckout(ctx, userID, payments.VerifyCheckoutRequest{
		RazorpayOrderID:   orderRes.RazorpayOrderID,
		RazorpayPaymentID: paymentID,
		RazorpaySignature: sig,
	})
	require.NoError(t, err)
	assert.Equal(t, payments.StatusPaid, verified.Status)

	bal, err = creditsSvc.GetBalance(ctx, userID)
	require.NoError(t, err)
	assert.Equal(t, int32(50), bal)
}

func TestPayments_Postgres_OutOfOrderWebhookEvents(t *testing.T) {
	ctx := context.Background()
	_, repo, creditsSvc, paymentsSvc, provider, userID := setupPostgresPayments(t)

	// 1. payment.captured arrives first, followed by delayed payment.failed
	order1, err := paymentsSvc.CreatePaymentOrder(ctx, userID, "pkg_starter", "")
	require.NoError(t, err)

	capturedPayload := map[string]any{
		"event": "payment.captured",
		"payload": map[string]any{
			"payment": map[string]any{
				"entity": map[string]any{
					"id":       "pay_cap_pg_1",
					"amount":   9900,
					"currency": "INR",
					"status":   "captured",
					"order_id": order1.RazorpayOrderID,
				},
			},
		},
		"created_at": 1700000100,
	}
	capBytes, _ := json.Marshal(capturedPayload)
	capSig := provider.ComputeWebhookSignature(capBytes)

	evtCap1 := fmt.Sprintf("evt_cap_pg_%s", uuid.New().String()[:8])
	res, err := paymentsSvc.ProcessWebhook(ctx, capBytes, capSig, evtCap1)
	require.NoError(t, err)
	assert.Equal(t, "PROCESSED", res.Status)

	bal, _ := creditsSvc.GetBalance(ctx, userID)
	assert.Equal(t, int32(50), bal)

	// Stale failure arrives for the captured order
	failedPayload := map[string]any{
		"event": "payment.failed",
		"payload": map[string]any{
			"payment": map[string]any{
				"entity": map[string]any{
					"id":       "pay_fail_stale",
					"amount":   9900,
					"currency": "INR",
					"order_id": order1.RazorpayOrderID,
				},
			},
		},
		"created_at": 1700000090,
	}
	failBytes, _ := json.Marshal(failedPayload)
	failSig := provider.ComputeWebhookSignature(failBytes)

	evtFailStale := fmt.Sprintf("evt_fail_stale_%s", uuid.New().String()[:8])
	resFail, err := paymentsSvc.ProcessWebhook(ctx, failBytes, failSig, evtFailStale)
	require.NoError(t, err)
	assert.Equal(t, "PROCESSED", resFail.Status)

	// Invariant: Status remains PAID, balance remains 50
	saved, _ := repo.GetOrderByRazorpayOrderID(ctx, order1.RazorpayOrderID)
	assert.Equal(t, payments.StatusPaid, saved.Status)
	balAfterStale, _ := creditsSvc.GetBalance(ctx, userID)
	assert.Equal(t, int32(50), balAfterStale)

	// 2. Initial failure followed by successful capture on retry
	order2, err := paymentsSvc.CreatePaymentOrder(ctx, userID, "pkg_pro", "")
	require.NoError(t, err)

	failedPayload2 := map[string]any{
		"event": "payment.failed",
		"payload": map[string]any{
			"payment": map[string]any{
				"entity": map[string]any{
					"id":       "pay_fail_first_pg",
					"amount":   24900,
					"currency": "INR",
					"order_id": order2.RazorpayOrderID,
				},
			},
		},
		"created_at": 1700000200,
	}
	failBytes2, _ := json.Marshal(failedPayload2)
	failSig2 := provider.ComputeWebhookSignature(failBytes2)

	evtFailFirst := fmt.Sprintf("evt_fail_first_pg_%s", uuid.New().String()[:8])
	_, err = paymentsSvc.ProcessWebhook(ctx, failBytes2, failSig2, evtFailFirst)
	require.NoError(t, err)
	savedFailed, _ := repo.GetOrderByRazorpayOrderID(ctx, order2.RazorpayOrderID)
	assert.Equal(t, payments.StatusFailed, savedFailed.Status)

	// Retry succeeds
	capturedPayload2 := map[string]any{
		"event": "payment.captured",
		"payload": map[string]any{
			"payment": map[string]any{
				"entity": map[string]any{
					"id":       "pay_success_retry_pg",
					"amount":   24900,
					"currency": "INR",
					"status":   "captured",
					"order_id": order2.RazorpayOrderID,
				},
			},
		},
		"created_at": 1700000205,
	}
	capBytes2, _ := json.Marshal(capturedPayload2)
	capSig2 := provider.ComputeWebhookSignature(capBytes2)

	evtSuccessRetry := fmt.Sprintf("evt_success_retry_pg_%s", uuid.New().String()[:8])
	resSuccess, err := paymentsSvc.ProcessWebhook(ctx, capBytes2, capSig2, evtSuccessRetry)
	require.NoError(t, err)
	assert.Equal(t, "PROCESSED", resSuccess.Status)

	savedSuccess, _ := repo.GetOrderByRazorpayOrderID(ctx, order2.RazorpayOrderID)
	assert.Equal(t, payments.StatusPaid, savedSuccess.Status)

	balFinal, _ := creditsSvc.GetBalance(ctx, userID)
	assert.Equal(t, int32(200), balFinal) // 50 + 150 = 200
}

func TestPayments_Postgres_Worker_CrashRecoveryAndBoundedRetries(t *testing.T) {
	ctx := context.Background()
	db, repo, _, _, _, _ := setupPostgresPayments(t)

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	worker := payments.NewWorker(repo, logger)

	// 1. Worker crash recovery: Job stranded in RUNNING for 6 minutes is claimed and completed
	crashJobType := "TEST_CRASH_RECOVERY_JOB"
	var recoveredJob atomic.Bool

	worker.RegisterHandler(crashJobType, func(ctx context.Context, job payments.Job) error {
		recoveredJob.Store(true)
		return nil
	})

	dbPool := db.Pool()
	staleJobID := uuid.New()
	_, err := dbPool.Exec(ctx, `
		INSERT INTO background_jobs (id, job_type, payload, status, retry_count, max_retries, scheduled_at, created_at, updated_at)
		VALUES ($1, $2, '{}', 'RUNNING', 0, 3, NOW() - INTERVAL '10 minutes', NOW() - INTERVAL '10 minutes', NOW() - INTERVAL '6 minutes')
	`, staleJobID, crashJobType)
	require.NoError(t, err)

	processed, err := worker.ProcessBatch(ctx)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, processed, 1)
	assert.True(t, recoveredJob.Load())

	// Verify status in PostgreSQL became COMPLETED
	var status string
	err = dbPool.QueryRow(ctx, "SELECT status FROM background_jobs WHERE id = $1", staleJobID).Scan(&status)
	require.NoError(t, err)
	assert.Equal(t, "COMPLETED", status)

	// 2. Failing job with bounded retry
	failingJobType := "TEST_FAILING_JOB"
	var failAttempts int32

	worker.RegisterHandler(failingJobType, func(ctx context.Context, job payments.Job) error {
		attempt := atomic.AddInt32(&failAttempts, 1)
		if attempt < 3 {
			return fmt.Errorf("simulated temporary network timeout")
		}
		return nil
	})

	err = repo.EnqueueJob(ctx, failingJobType, map[string]string{"job": "fail"})
	require.NoError(t, err)

	// Attempt 1: job fails, retry_count becomes 1
	processed, err = worker.ProcessBatch(ctx)
	require.NoError(t, err)
	assert.Equal(t, 0, processed, "Failed job is not counted as successfully processed")
	assert.Equal(t, int32(1), atomic.LoadInt32(&failAttempts))

	// Fast-forward scheduled_at for the pending retry job in postgres
	_, err = dbPool.Exec(ctx, "UPDATE background_jobs SET scheduled_at = NOW() - INTERVAL '1 second' WHERE job_type = $1", failingJobType)
	require.NoError(t, err)

	// Attempt 2: job fails again, retry_count becomes 2
	processed, err = worker.ProcessBatch(ctx)
	require.NoError(t, err)
	assert.Equal(t, 0, processed)
	assert.Equal(t, int32(2), atomic.LoadInt32(&failAttempts))

	// Fast-forward scheduled_at again
	_, err = dbPool.Exec(ctx, "UPDATE background_jobs SET scheduled_at = NOW() - INTERVAL '1 second' WHERE job_type = $1", failingJobType)
	require.NoError(t, err)

	// Attempt 3: job succeeds!
	processed, err = worker.ProcessBatch(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, processed, "Job should succeed on attempt 3")
	assert.Equal(t, int32(3), atomic.LoadInt32(&failAttempts))
}

func TestPayments_Postgres_ListOrders_UserAndAdmin(t *testing.T) {
	ctx := context.Background()
	_, _, _, paymentsSvc, _, userID := setupPostgresPayments(t)

	// Create 3 orders
	_, err := paymentsSvc.CreatePaymentOrder(ctx, userID, "pkg_starter", "")
	require.NoError(t, err)
	_, err = paymentsSvc.CreatePaymentOrder(ctx, userID, "pkg_pro", "")
	require.NoError(t, err)
	_, err = paymentsSvc.CreatePaymentOrder(ctx, userID, "pkg_master", "")
	require.NoError(t, err)

	// List user orders
	userOrders, total, err := paymentsSvc.GetUserPaymentHistory(ctx, userID, 10, 0)
	require.NoError(t, err)
	assert.Equal(t, int64(3), total)
	assert.Len(t, userOrders, 3)

	// List admin orders
	allOrders, adminTotal, err := paymentsSvc.AdminListPayments(ctx, 10, 0)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, adminTotal, int64(3))
	assert.NotEmpty(t, allOrders)
}
