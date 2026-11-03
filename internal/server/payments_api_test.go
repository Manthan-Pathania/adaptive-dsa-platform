package server

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/adaptive-dsa/platform/internal/config"
	"github.com/adaptive-dsa/platform/internal/credits"
	"github.com/adaptive-dsa/platform/internal/identity"
	"github.com/adaptive-dsa/platform/internal/payments"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockPaymentsRepo struct {
	orders        map[uuid.UUID]*payments.PaymentOrder
	ordersByRP    map[string]*payments.PaymentOrder
	webhookEvents map[string]*payments.WebhookEvent
}

func newMockPaymentsRepo() *mockPaymentsRepo {
	return &mockPaymentsRepo{
		orders:        make(map[uuid.UUID]*payments.PaymentOrder),
		ordersByRP:    make(map[string]*payments.PaymentOrder),
		webhookEvents: make(map[string]*payments.WebhookEvent),
	}
}

func (m *mockPaymentsRepo) CreateOrder(_ context.Context, order *payments.PaymentOrder) (*payments.PaymentOrder, error) {
	if order.ID == uuid.Nil {
		order.ID = uuid.New()
	}
	order.CreatedAt = time.Now().UTC()
	order.UpdatedAt = time.Now().UTC()
	m.orders[order.ID] = order
	if order.RazorpayOrderID != "" {
		m.ordersByRP[order.RazorpayOrderID] = order
	}
	return order, nil
}

func (m *mockPaymentsRepo) GetOrderByID(_ context.Context, id uuid.UUID) (*payments.PaymentOrder, error) {
	o, ok := m.orders[id]
	if !ok {
		return nil, payments.ErrOrderNotFound
	}
	return o, nil
}

func (m *mockPaymentsRepo) GetOrderByIDForUpdate(ctx context.Context, id uuid.UUID) (*payments.PaymentOrder, error) {
	return m.GetOrderByID(ctx, id)
}

func (m *mockPaymentsRepo) GetOrderByRazorpayOrderID(_ context.Context, rpOrderID string) (*payments.PaymentOrder, error) {
	o, ok := m.ordersByRP[rpOrderID]
	if !ok {
		return nil, payments.ErrOrderNotFound
	}
	return o, nil
}

func (m *mockPaymentsRepo) GetOrderByRazorpayOrderIDForUpdate(ctx context.Context, rpOrderID string) (*payments.PaymentOrder, error) {
	return m.GetOrderByRazorpayOrderID(ctx, rpOrderID)
}

func (m *mockPaymentsRepo) GetOrderByIdempotencyKey(_ context.Context, _ string) (*payments.PaymentOrder, error) {
	return nil, payments.ErrOrderNotFound
}

func (m *mockPaymentsRepo) UpdateOrder(_ context.Context, order *payments.PaymentOrder) error {
	order.UpdatedAt = time.Now().UTC()
	m.orders[order.ID] = order
	if order.RazorpayOrderID != "" {
		m.ordersByRP[order.RazorpayOrderID] = order
	}
	return nil
}

func (m *mockPaymentsRepo) ListOrdersByUser(_ context.Context, userID uuid.UUID, _, _ int32) ([]payments.PaymentOrder, int64, error) {
	var res []payments.PaymentOrder
	for _, o := range m.orders {
		if o.UserID == userID {
			res = append(res, *o)
		}
	}
	return res, int64(len(res)), nil
}

func (m *mockPaymentsRepo) ListAllOrders(_ context.Context, _, _ int32) ([]payments.PaymentOrder, int64, error) {
	var res []payments.PaymentOrder
	for _, o := range m.orders {
		res = append(res, *o)
	}
	return res, int64(len(res)), nil
}

func (m *mockPaymentsRepo) RecordWebhookEvent(_ context.Context, event *payments.WebhookEvent) (bool, error) {
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

func (m *mockPaymentsRepo) UpdateWebhookEventStatus(_ context.Context, eventID string, status string, errMsg string) error {
	if e, exists := m.webhookEvents[eventID]; exists {
		e.Status = status
		now := time.Now().UTC()
		e.ProcessedAt = &now
		if errMsg != "" {
			e.ErrorMessage = errMsg
		}
	}
	return nil
}

func (m *mockPaymentsRepo) EnqueueJob(_ context.Context, _ string, _ any) error {
	return nil
}

func (m *mockPaymentsRepo) FetchPendingJobs(_ context.Context, _ int) ([]payments.Job, error) {
	return nil, nil
}

func (m *mockPaymentsRepo) UpdateJob(_ context.Context, _ *payments.Job) error {
	return nil
}

func (m *mockPaymentsRepo) GetJobStats(_ context.Context) (map[string]int64, error) {
	return map[string]int64{
		"PENDING":   0,
		"RUNNING":   0,
		"COMPLETED": 0,
		"FAILED":    0,
	}, nil
}

func (m *mockPaymentsRepo) ExecTx(_ context.Context, fn func(payments.Repository) error) error {
	return fn(m)
}

func TestPaymentsAPI_Endpoints(t *testing.T) {
	cfg := &config.Config{
		AppEnv: "testing",
		Server: config.ServerConfig{
			Port:            8080,
			ReadTimeout:     5 * time.Second,
			WriteTimeout:    5 * time.Second,
			IdleTimeout:     30 * time.Second,
			ShutdownTimeout: 2 * time.Second,
		},
		Session: config.SessionConfig{
			CookieName: "session_token",
		},
		Auth: config.AuthConfig{
			CSRFSecret: "test-secret-min-32-chars-long-abc!",
		},
		Payments: config.PaymentsConfig{
			RazorpayKeyID:         "rzp_test_apikey123",
			RazorpayKeySecret:     "rzp_test_secret456",
			RazorpayWebhookSecret: "rzp_test_whsec789",
		},
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	idRepo := identity.NewInMemoryRepository()
	idSvc := identity.NewService(idRepo, cfg, logger)

	creditsRepo := credits.NewInMemoryRepository()
	creditsSvc := credits.NewService(creditsRepo)

	paymentsRepo := newMockPaymentsRepo()
	mockProvider := payments.NewMockPaymentProvider()
	paymentsSvc := payments.NewService(paymentsRepo, mockProvider, creditsSvc, logger)

	ctx := context.Background()
	// Candidate user
	candUser, err := idRepo.CreateUser(ctx, "candidate@payments.com", "$argon2id$mockhash", "Candidate User", identity.RoleCandidate)
	require.NoError(t, err)
	_, err = creditsSvc.EnsureAccount(ctx, candUser.ID, 0)
	require.NoError(t, err)

	// Admin user
	adminUser, err := idRepo.CreateUser(ctx, "admin@payments.com", "$argon2id$mockhash", "Admin User", identity.RoleAdmin)
	require.NoError(t, err)

	// Create sessions
	candHash := identity.HashSessionToken("cand-token")
	adminHash := identity.HashSessionToken("admin-token")
	_, err = idRepo.CreateSession(ctx, candUser.ID, candHash, "127.0.0.1", "test-agent", time.Now().Add(24*time.Hour))
	require.NoError(t, err)
	_, err = idRepo.CreateSession(ctx, adminUser.ID, adminHash, "127.0.0.1", "test-agent", time.Now().Add(24*time.Hour))
	require.NoError(t, err)

	candCookie := &http.Cookie{
		Name:     cfg.Session.CookieName,
		Value:    "cand-token",
		Path:     "/",
		HttpOnly: true,
	}
	adminCookie := &http.Cookie{
		Name:     cfg.Session.CookieName,
		Value:    "admin-token",
		Path:     "/",
		HttpOnly: true,
	}

	candCSRFToken := identity.GenerateSessionCSRFToken(candHash, cfg.Auth.CSRFSecret)

	srv := NewServer(cfg, logger, nil, idSvc, nil, nil, creditsSvc, nil, nil, paymentsSvc)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	client := ts.Client()

	// 1. GET /api/v1/payments/packages (Public endpoint)
	t.Run("GET /api/v1/payments/packages returns available credit tiers", func(t *testing.T) {
		resp, err := client.Get(ts.URL + "/api/v1/payments/packages")
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)
		var body map[string][]payments.CreditPackage
		err = json.NewDecoder(resp.Body).Decode(&body)
		require.NoError(t, err)

		pkgs := body["packages"]
		assert.Len(t, pkgs, 3)
		assert.Equal(t, "pkg_starter", pkgs[0].ID)
		assert.Equal(t, int32(50), pkgs[0].Credits)
		assert.Equal(t, int32(9900), pkgs[0].AmountPaise)
		assert.Equal(t, "INR", pkgs[0].Currency)

		assert.Equal(t, "pkg_pro", pkgs[1].ID)
		assert.Equal(t, int32(150), pkgs[1].Credits)
		assert.Equal(t, int32(24900), pkgs[1].AmountPaise)

		assert.Equal(t, "pkg_master", pkgs[2].ID)
		assert.Equal(t, int32(400), pkgs[2].Credits)
		assert.Equal(t, int32(59900), pkgs[2].AmountPaise)
	})

	// 2. POST /api/v1/payments/orders without auth -> 401
	t.Run("POST /api/v1/payments/orders requires authentication", func(t *testing.T) {
		body, _ := json.Marshal(map[string]string{"package_id": "pkg_pro"})
		req, _ := http.NewRequest(http.MethodPost, ts.URL+"/api/v1/payments/orders", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")

		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	})

	// 3. POST /api/v1/payments/orders without CSRF token -> 403
	t.Run("POST /api/v1/payments/orders requires CSRF token", func(t *testing.T) {
		body, _ := json.Marshal(map[string]string{"package_id": "pkg_pro"})
		req, _ := http.NewRequest(http.MethodPost, ts.URL+"/api/v1/payments/orders", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.AddCookie(candCookie)

		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusForbidden, resp.StatusCode)
	})

	// 4. POST /api/v1/payments/orders with invalid package -> 400
	t.Run("POST /api/v1/payments/orders rejects invalid package", func(t *testing.T) {
		body, _ := json.Marshal(map[string]string{"package_id": "invalid_pack_999"})
		req, _ := http.NewRequest(http.MethodPost, ts.URL+"/api/v1/payments/orders", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-CSRF-Token", candCSRFToken)
		req.AddCookie(candCookie)

		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	})

	// 5. POST /api/v1/payments/orders successfully creates order with backend-enforced price
	var createdOrder payments.CreateOrderResult
	t.Run("POST /api/v1/payments/orders enforces backend pricing and returns razorpay order", func(t *testing.T) {
		// Client maliciously sends amount: 1 and credits: 99999, which server must ignore!
		body, _ := json.Marshal(map[string]any{
			"package_id": "pkg_pro",
			"amount":     1,
			"credits":    99999,
		})
		req, _ := http.NewRequest(http.MethodPost, ts.URL+"/api/v1/payments/orders", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-CSRF-Token", candCSRFToken)
		req.AddCookie(candCookie)

		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusCreated, resp.StatusCode)
		err = json.NewDecoder(resp.Body).Decode(&createdOrder)
		require.NoError(t, err)

		assert.NotEmpty(t, createdOrder.OrderID)
		assert.NotEmpty(t, createdOrder.RazorpayOrderID)
		assert.Equal(t, int32(24900), createdOrder.AmountPaise, "Must strictly enforce backend price 24900 paise")
		assert.Equal(t, int32(150), createdOrder.CreditsAmount, "Must strictly enforce 150 credits")
		assert.Equal(t, "INR", createdOrder.Currency)
		assert.Equal(t, "rzp_test_mock_key_id", createdOrder.KeyID)
	})

	// 6. POST /api/v1/payments/verify with invalid signature -> 400
	t.Run("POST /api/v1/payments/verify rejects forged signature", func(t *testing.T) {
		body, _ := json.Marshal(payments.VerifyCheckoutRequest{
			RazorpayOrderID:   createdOrder.RazorpayOrderID,
			RazorpayPaymentID: "pay_tampered_123",
			RazorpaySignature: "forged_signature_hex_value",
		})
		req, _ := http.NewRequest(http.MethodPost, ts.URL+"/api/v1/payments/verify", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-CSRF-Token", candCSRFToken)
		req.AddCookie(candCookie)

		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	})

	// 7. POST /api/v1/payments/verify with valid signature -> checks captured status before granting credits
	t.Run("POST /api/v1/payments/verify validates captured status and fulfills order", func(t *testing.T) {
		rpPaymentID := "pay_valid_checkout_777"
		validSig := mockProvider.ComputeCheckoutSignature(createdOrder.RazorpayOrderID, rpPaymentID)

		body, _ := json.Marshal(payments.VerifyCheckoutRequest{
			RazorpayOrderID:   createdOrder.RazorpayOrderID,
			RazorpayPaymentID: rpPaymentID,
			RazorpaySignature: validSig,
		})

		// 7a. Reject if payment status is "authorized" rather than "captured"
		mockProvider.MockPaymentStatus = "authorized"
		reqAuth, _ := http.NewRequest(http.MethodPost, ts.URL+"/api/v1/payments/verify", bytes.NewReader(body))
		reqAuth.Header.Set("Content-Type", "application/json")
		reqAuth.Header.Set("X-CSRF-Token", candCSRFToken)
		reqAuth.AddCookie(candCookie)

		respAuth, err := client.Do(reqAuth)
		require.NoError(t, err)
		assert.Equal(t, http.StatusBadRequest, respAuth.StatusCode)
		respAuth.Body.Close()

		// 7b. Fulfill when status is confirmed "captured"
		mockProvider.MockPaymentStatus = "captured"
		req, _ := http.NewRequest(http.MethodPost, ts.URL+"/api/v1/payments/verify", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-CSRF-Token", candCSRFToken)
		req.AddCookie(candCookie)

		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)
		var verifyResp struct {
			Status         string                `json:"status"`
			Message        string                `json:"message"`
			Order          payments.PaymentOrder `json:"order"`
			CreditsGranted int32                 `json:"credits_granted"`
		}
		err = json.NewDecoder(resp.Body).Decode(&verifyResp)
		require.NoError(t, err)
		assert.Equal(t, "success", verifyResp.Status)
		assert.Equal(t, payments.StatusPaid, verifyResp.Order.Status)
		assert.Equal(t, int32(150), verifyResp.CreditsGranted)

		// Check credit balance was incremented by 150
		bal, err := creditsSvc.GetBalance(ctx, candUser.ID)
		require.NoError(t, err)
		assert.Equal(t, int32(150), bal)
	})

	// 8. POST /api/v1/payments/webhook
	t.Run("POST /api/v1/payments/webhook validates signature and is idempotent", func(t *testing.T) {
		// Create new order for webhook test
		orderRes, err := paymentsSvc.CreatePaymentOrder(ctx, candUser.ID, "pkg_starter", "")
		require.NoError(t, err)

		webhookPaymentID := "pay_wh_888"
		whPayload := map[string]any{
			"event": "payment.captured",
			"payload": map[string]any{
				"payment": map[string]any{
					"entity": map[string]any{
						"id":       webhookPaymentID,
						"order_id": orderRes.RazorpayOrderID,
						"amount":   9900,
						"currency": "INR",
						"status":   "captured",
					},
				},
			},
		}
		whBytes, _ := json.Marshal(whPayload)

		// 8a. Invalid signature -> 400
		req1, _ := http.NewRequest(http.MethodPost, ts.URL+"/api/v1/payments/webhook", bytes.NewReader(whBytes))
		req1.Header.Set("Content-Type", "application/json")
		req1.Header.Set("X-Razorpay-Signature", "invalid_webhook_signature")

		resp1, err := client.Do(req1)
		require.NoError(t, err)
		assert.Equal(t, http.StatusBadRequest, resp1.StatusCode)
		resp1.Body.Close()

		// 8b. Valid signature with documented X-Razorpay-Event-Id header -> 200 with status PROCESSED
		validWhSig := mockProvider.ComputeWebhookSignature(whBytes)
		officialEventID := "evt_official_api_123"

		req2, _ := http.NewRequest(http.MethodPost, ts.URL+"/api/v1/payments/webhook", bytes.NewReader(whBytes))
		req2.Header.Set("Content-Type", "application/json")
		req2.Header.Set("X-Razorpay-Signature", validWhSig)
		req2.Header.Set("X-Razorpay-Event-Id", officialEventID)

		resp2, err := client.Do(req2)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp2.StatusCode)
		var whResult payments.WebhookResult
		err = json.NewDecoder(resp2.Body).Decode(&whResult)
		require.NoError(t, err)
		assert.Equal(t, "PROCESSED", whResult.Status)
		assert.Equal(t, officialEventID, whResult.EventID)
		resp2.Body.Close()

		// 8c. Duplicate webhook with identical X-Razorpay-Event-Id -> 200 with status DUPLICATE_IGNORED
		req3, _ := http.NewRequest(http.MethodPost, ts.URL+"/api/v1/payments/webhook", bytes.NewReader(whBytes))
		req3.Header.Set("Content-Type", "application/json")
		req3.Header.Set("X-Razorpay-Signature", validWhSig)
		req3.Header.Set("X-Razorpay-Event-Id", officialEventID)

		resp3, err := client.Do(req3)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp3.StatusCode)
		var dupResult payments.WebhookResult
		err = json.NewDecoder(resp3.Body).Decode(&dupResult)
		require.NoError(t, err)
		assert.Equal(t, "DUPLICATE_IGNORED", dupResult.Status)
		resp3.Body.Close()

		// Verify candidate balance: 150 (from checkout) + 50 (from starter package) = 200
		bal, err := creditsSvc.GetBalance(ctx, candUser.ID)
		require.NoError(t, err)
		assert.Equal(t, int32(200), bal)
	})

	// 9. GET /api/v1/payments/history
	t.Run("GET /api/v1/payments/history returns user order history", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodGet, ts.URL+"/api/v1/payments/history", nil)
		req.AddCookie(candCookie)

		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)
		var body map[string]any
		err = json.NewDecoder(resp.Body).Decode(&body)
		require.NoError(t, err)
		assert.NotNil(t, body["orders"])
		assert.GreaterOrEqual(t, int(body["total"].(float64)), 2)
	})

	// 10. GET /api/v1/admin/payments RBAC
	t.Run("GET /api/v1/admin/payments enforces admin role", func(t *testing.T) {
		// Candidate role -> 403 Forbidden
		candReq, _ := http.NewRequest(http.MethodGet, ts.URL+"/api/v1/admin/payments", nil)
		candReq.AddCookie(candCookie)

		candResp, err := client.Do(candReq)
		require.NoError(t, err)
		assert.Equal(t, http.StatusForbidden, candResp.StatusCode)
		candResp.Body.Close()

		// Admin role -> 200 OK
		adminReq, _ := http.NewRequest(http.MethodGet, ts.URL+"/api/v1/admin/payments", nil)
		adminReq.AddCookie(adminCookie)

		adminResp, err := client.Do(adminReq)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, adminResp.StatusCode)
		var body map[string]any
		err = json.NewDecoder(adminResp.Body).Decode(&body)
		require.NoError(t, err)
		assert.NotNil(t, body["orders"])
		adminResp.Body.Close()
	})

	// 11. GET /api/v1/admin/metrics RBAC and Diagnostics
	t.Run("GET /api/v1/admin/metrics returns operational diagnostics for admin", func(t *testing.T) {
		// Candidate role -> 403 Forbidden
		candReq, _ := http.NewRequest(http.MethodGet, ts.URL+"/api/v1/admin/metrics", nil)
		candReq.AddCookie(candCookie)

		candResp, err := client.Do(candReq)
		require.NoError(t, err)
		assert.Equal(t, http.StatusForbidden, candResp.StatusCode)
		candResp.Body.Close()

		// Admin role -> 200 OK with metrics
		adminReq, _ := http.NewRequest(http.MethodGet, ts.URL+"/api/v1/admin/metrics", nil)
		adminReq.AddCookie(adminCookie)

		adminResp, err := client.Do(adminReq)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, adminResp.StatusCode)
		var body map[string]any
		err = json.NewDecoder(adminResp.Body).Decode(&body)
		require.NoError(t, err)
		assert.Equal(t, "testing", body["app_env"])
		assert.NotNil(t, body["runtime"])
		assert.NotNil(t, body["background_jobs"])
		adminResp.Body.Close()
	})
}
