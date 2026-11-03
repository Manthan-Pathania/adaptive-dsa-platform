package payments

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/adaptive-dsa/platform/internal/credits"
	"github.com/google/uuid"
)

type CreateOrderResult struct {
	OrderID         uuid.UUID `json:"order_id"`
	RazorpayOrderID string    `json:"razorpay_order_id"`
	AmountPaise     int32     `json:"amount_paise"`
	Currency        string    `json:"currency"`
	CreditsAmount   int32     `json:"credits_amount"`
	PackageID       string    `json:"package_id"`
	PackageName     string    `json:"package_name"`
	KeyID           string    `json:"key_id"` // Public Razorpay Key ID for checkout initialization
}

type VerifyCheckoutRequest struct {
	RazorpayOrderID   string `json:"razorpay_order_id"`
	RazorpayPaymentID string `json:"razorpay_payment_id"`
	RazorpaySignature string `json:"razorpay_signature"`
}

type WebhookResult struct {
	EventID string `json:"event_id"`
	Status  string `json:"status"` // 'PROCESSED', 'DUPLICATE_IGNORED', 'IGNORED'
}

type Service interface {
	GetPackages() []CreditPackage
	CreatePaymentOrder(ctx context.Context, userID uuid.UUID, packageID string, idempotencyKey string) (*CreateOrderResult, error)
	VerifyCheckout(ctx context.Context, userID uuid.UUID, req VerifyCheckoutRequest) (*PaymentOrder, error)
	ProcessWebhook(ctx context.Context, rawBody []byte, signature string, eventIDHeader string) (*WebhookResult, error)
	GetUserPaymentHistory(ctx context.Context, userID uuid.UUID, limit, offset int32) ([]PaymentOrder, int64, error)
	AdminListPayments(ctx context.Context, limit, offset int32) ([]PaymentOrder, int64, error)
	GetJobStats(ctx context.Context) (map[string]int64, error)
}

type service struct {
	repo       Repository
	provider   PaymentProvider
	creditsSvc credits.Service
	logger     *slog.Logger
}

func NewService(repo Repository, provider PaymentProvider, creditsSvc credits.Service, logger *slog.Logger) Service {
	if logger == nil {
		logger = slog.Default()
	}
	return &service{
		repo:       repo,
		provider:   provider,
		creditsSvc: creditsSvc,
		logger:     logger,
	}
}

func (s *service) GetPackages() []CreditPackage {
	return AvailablePackages
}

func (s *service) CreatePaymentOrder(
	ctx context.Context,
	userID uuid.UUID,
	packageID string,
	idempotencyKey string,
) (*CreateOrderResult, error) {
	pkg, err := FindPackage(packageID)
	if err != nil {
		return nil, err
	}

	// 1. Idempotency check if key provided
	if idempotencyKey != "" {
		existing, err := s.repo.GetOrderByIdempotencyKey(ctx, idempotencyKey)
		if err == nil && existing != nil {
			if existing.UserID != userID {
				return nil, fmt.Errorf("idempotency key belongs to another user")
			}
			return &CreateOrderResult{
				OrderID:         existing.ID,
				RazorpayOrderID: existing.RazorpayOrderID,
				AmountPaise:     existing.AmountPaise,
				Currency:        existing.Currency,
				CreditsAmount:   existing.CreditsAmount,
				PackageID:       existing.PackageID,
				PackageName:     pkg.Name,
				KeyID:           s.provider.KeyID(),
			}, nil
		}
	}

	orderID := uuid.New()
	receipt := fmt.Sprintf("rcpt_%s", orderID.String()[:8])

	// 2. Create remote order via payment provider
	provReq := CreateProviderOrderRequest{
		AmountPaise: pkg.AmountPaise,
		Currency:    pkg.Currency,
		Receipt:     receipt,
		Notes: map[string]string{
			"user_id":    userID.String(),
			"package_id": pkg.ID,
			"order_id":   orderID.String(),
		},
	}

	provResp, err := s.provider.CreateOrder(ctx, provReq)
	if err != nil {
		return nil, fmt.Errorf("failed to create payment provider order: %w", err)
	}

	// 3. Persist payment order in database
	order := &PaymentOrder{
		ID:              orderID,
		UserID:          userID,
		PackageID:       pkg.ID,
		AmountPaise:     pkg.AmountPaise,
		Currency:        pkg.Currency,
		CreditsAmount:   pkg.Credits,
		Status:          StatusCreated,
		RazorpayOrderID: provResp.ID,
		IdempotencyKey:  idempotencyKey,
	}

	saved, err := s.repo.CreateOrder(ctx, order)
	if err != nil {
		return nil, fmt.Errorf("failed to save payment order: %w", err)
	}

	s.logger.InfoContext(ctx, "created server-side payment order",
		"order_id", saved.ID,
		"rp_order_id", saved.RazorpayOrderID,
		"package_id", saved.PackageID,
		"amount_paise", saved.AmountPaise,
	)

	return &CreateOrderResult{
		OrderID:         saved.ID,
		RazorpayOrderID: saved.RazorpayOrderID,
		AmountPaise:     saved.AmountPaise,
		Currency:        saved.Currency,
		CreditsAmount:   saved.CreditsAmount,
		PackageID:       saved.PackageID,
		PackageName:     pkg.Name,
		KeyID:           s.provider.KeyID(),
	}, nil
}

func (s *service) VerifyCheckout(
	ctx context.Context,
	userID uuid.UUID,
	req VerifyCheckoutRequest,
) (*PaymentOrder, error) {
	if req.RazorpayOrderID == "" || req.RazorpayPaymentID == "" || req.RazorpaySignature == "" {
		return nil, fmt.Errorf("missing required razorpay checkout verification fields")
	}

	// 1. Verify cryptographic signature
	if !s.provider.VerifyCheckoutSignature(req.RazorpayOrderID, req.RazorpayPaymentID, req.RazorpaySignature) {
		s.logger.WarnContext(ctx, "rejected invalid razorpay checkout signature",
			"rp_order_id", req.RazorpayOrderID,
			"rp_payment_id", req.RazorpayPaymentID,
		)
		return nil, ErrInvalidSignature
	}

	// 2. Fetch authoritative payment status directly from gateway provider
	paymentDetails, err := s.provider.GetPayment(ctx, req.RazorpayPaymentID)
	if err != nil {
		s.logger.ErrorContext(ctx, "failed to verify payment status with gateway provider",
			"rp_payment_id", req.RazorpayPaymentID,
			"error", err,
		)
		return nil, fmt.Errorf("gateway payment status check failed: %w", err)
	}

	// Ensure payment is actually in captured status
	if paymentDetails.Status != "captured" {
		s.logger.WarnContext(ctx, "payment is not in captured status",
			"rp_payment_id", req.RazorpayPaymentID,
			"status", paymentDetails.Status,
		)
		return nil, fmt.Errorf("%w: payment status is '%s'", ErrPaymentNotCaptured, paymentDetails.Status)
	}

	// If gateway returned order association, verify it matches
	if paymentDetails.OrderID != "" && paymentDetails.OrderID != req.RazorpayOrderID {
		s.logger.ErrorContext(ctx, "payment order mismatch between gateway and request",
			"expected_order_id", req.RazorpayOrderID,
			"gateway_order_id", paymentDetails.OrderID,
		)
		return nil, ErrMismatchedPaymentData
	}

	var updatedOrder *PaymentOrder

	// 3. Atomically transition order and fulfill credits inside a database transaction
	err = s.repo.ExecTx(ctx, func(txRepo Repository) error {
		order, err := txRepo.GetOrderByRazorpayOrderIDForUpdate(ctx, req.RazorpayOrderID)
		if err != nil {
			return err
		}

		if order.UserID != userID {
			return fmt.Errorf("payment order does not belong to authenticated user")
		}

		// Idempotency check: If already fulfilled, return immediately without duplicate grant
		if order.Status == StatusPaid {
			updatedOrder = order
			return nil
		}

		// Verify amount & currency if returned by provider
		if paymentDetails.AmountPaise > 0 && paymentDetails.AmountPaise != order.AmountPaise {
			return fmt.Errorf("%w: gateway amount %d != order amount %d", ErrMismatchedPaymentData, paymentDetails.AmountPaise, order.AmountPaise)
		}
		if paymentDetails.Currency != "" && paymentDetails.Currency != order.Currency {
			return fmt.Errorf("%w: gateway currency %s != order currency %s", ErrMismatchedPaymentData, paymentDetails.Currency, order.Currency)
		}

		order.Status = StatusPaid
		order.RazorpayPaymentID = req.RazorpayPaymentID
		order.RazorpaySignature = req.RazorpaySignature

		if err := txRepo.UpdateOrder(ctx, order); err != nil {
			return fmt.Errorf("update order to paid: %w", err)
		}

		// Grant credits through existing ledger service
		pkg, _ := FindPackage(order.PackageID)
		pkgName := order.PackageID
		if pkg != nil {
			pkgName = pkg.Name
		}
		desc := fmt.Sprintf("Purchased %s (%d credits via Razorpay %s)", pkgName, order.CreditsAmount, req.RazorpayPaymentID)

		_, err = s.creditsSvc.GrantPurchasedCredits(ctx, order.UserID, order.CreditsAmount, order.ID, desc)
		if err != nil {
			return fmt.Errorf("grant purchased credits: %w", err)
		}

		// Enqueue durable background confirmation job
		_ = txRepo.EnqueueJob(ctx, JobTypePurchaseConfirmation, map[string]interface{}{
			"order_id":   order.ID.String(),
			"user_id":    order.UserID.String(),
			"credits":    order.CreditsAmount,
			"payment_id": req.RazorpayPaymentID,
		})

		updatedOrder = order
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("checkout verification failed: %w", err)
	}

	s.logger.InfoContext(ctx, "payment verified and credits successfully granted",
		"order_id", updatedOrder.ID,
		"user_id", updatedOrder.UserID,
		"credits_granted", updatedOrder.CreditsAmount,
		"rp_payment_id", req.RazorpayPaymentID,
	)

	return updatedOrder, nil
}

type razorpayWebhookPayload struct {
	Entity    string   `json:"entity"`
	AccountID string   `json:"account_id"`
	Event     string   `json:"event"`
	Contains  []string `json:"contains"`
	Payload   struct {
		Payment struct {
			Entity struct {
				ID       string `json:"id"`
				Amount   int32  `json:"amount"`
				Currency string `json:"currency"`
				Status   string `json:"status"`
				OrderID  string `json:"order_id"`
				Captured bool   `json:"captured"`
			} `json:"entity"`
		} `json:"payment"`
		Order struct {
			Entity struct {
				ID         string `json:"id"`
				Amount     int32  `json:"amount"`
				AmountPaid int32  `json:"amount_paid"`
				Currency   string `json:"currency"`
				Status     string `json:"status"`
			} `json:"entity"`
		} `json:"order"`
	} `json:"payload"`
	CreatedAt int64 `json:"created_at"`
}

func (s *service) ProcessWebhook(ctx context.Context, rawBody []byte, signature string, eventIDHeader string) (*WebhookResult, error) {
	// 1. Verify webhook signature
	if !s.provider.VerifyWebhookSignature(rawBody, signature) {
		s.logger.WarnContext(ctx, "rejected razorpay webhook with invalid signature")
		return nil, ErrInvalidWebhookSignature
	}

	var payload razorpayWebhookPayload
	if err := json.Unmarshal(rawBody, &payload); err != nil {
		return nil, fmt.Errorf("invalid webhook JSON payload: %w", err)
	}

	// Prefer documented x-razorpay-event-id header; fallback to deterministic payload ID if header is absent
	eventID := eventIDHeader
	if eventID == "" {
		eventID = fmt.Sprintf("%s_%s_%d", payload.Event, payload.Payload.Payment.Entity.ID, payload.CreatedAt)
		if payload.Payload.Payment.Entity.ID == "" && payload.Payload.Order.Entity.ID != "" {
			eventID = fmt.Sprintf("%s_%s_%d", payload.Event, payload.Payload.Order.Entity.ID, payload.CreatedAt)
		}
	}

	// 2. Deduplication check via database constraint
	isNew, err := s.repo.RecordWebhookEvent(ctx, &WebhookEvent{
		EventID:   eventID,
		EventType: payload.Event,
		Payload:   rawBody,
		Status:    "RECEIVED",
	})
	if err != nil {
		return nil, fmt.Errorf("record webhook event: %w", err)
	}

	if !isNew {
		s.logger.InfoContext(ctx, "duplicate webhook delivery ignored", "event_id", eventID, "event_type", payload.Event)
		return &WebhookResult{EventID: eventID, Status: "DUPLICATE_IGNORED"}, nil
	}

	// 3. Process event semantics with strict state transition guards
	switch payload.Event {
	case "payment.captured", "order.paid":
		rpOrderID := payload.Payload.Payment.Entity.OrderID
		if rpOrderID == "" {
			rpOrderID = payload.Payload.Order.Entity.ID
		}
		rpPaymentID := payload.Payload.Payment.Entity.ID
		paymentAmount := payload.Payload.Payment.Entity.Amount
		currency := payload.Payload.Payment.Entity.Currency

		if rpOrderID == "" {
			_ = s.repo.UpdateWebhookEventStatus(ctx, eventID, "FAILED", "missing order_id in payment webhook")
			return nil, fmt.Errorf("webhook: missing order_id in payment.captured payload")
		}

		err = s.repo.ExecTx(ctx, func(txRepo Repository) error {
			order, err := txRepo.GetOrderByRazorpayOrderIDForUpdate(ctx, rpOrderID)
			if err != nil {
				return err
			}

			// Validate amount and currency
			if order.AmountPaise != paymentAmount || order.Currency != currency {
				s.logger.ErrorContext(ctx, "webhook amount/currency mismatch with order",
					"order_id", order.ID,
					"expected_amount", order.AmountPaise,
					"webhook_amount", paymentAmount,
					"expected_currency", order.Currency,
					"webhook_currency", currency,
				)
				return ErrMismatchedPaymentData
			}

			// Exactly-once guarantee: If already paid, do not re-grant
			if order.Status == StatusPaid {
				return nil
			}

			order.Status = StatusPaid
			if rpPaymentID != "" {
				order.RazorpayPaymentID = rpPaymentID
			}
			if err := txRepo.UpdateOrder(ctx, order); err != nil {
				return fmt.Errorf("webhook update order: %w", err)
			}

			pkg, _ := FindPackage(order.PackageID)
			pkgName := order.PackageID
			if pkg != nil {
				pkgName = pkg.Name
			}
			desc := fmt.Sprintf("Purchased %s (%d credits via Razorpay webhook %s)", pkgName, order.CreditsAmount, rpPaymentID)

			_, err = s.creditsSvc.GrantPurchasedCredits(ctx, order.UserID, order.CreditsAmount, order.ID, desc)
			if err != nil {
				return fmt.Errorf("webhook grant credits: %w", err)
			}

			_ = txRepo.EnqueueJob(ctx, JobTypePurchaseConfirmation, map[string]interface{}{
				"order_id":   order.ID.String(),
				"user_id":    order.UserID.String(),
				"credits":    order.CreditsAmount,
				"payment_id": rpPaymentID,
			})

			return nil
		})

		if err != nil {
			_ = s.repo.UpdateWebhookEventStatus(ctx, eventID, "FAILED", err.Error())
			return nil, err
		}

		_ = s.repo.UpdateWebhookEventStatus(ctx, eventID, "PROCESSED", "")
		s.logger.InfoContext(ctx, "successfully fulfilled payment via webhook", "event_id", eventID, "rp_order_id", rpOrderID)
		return &WebhookResult{EventID: eventID, Status: "PROCESSED"}, nil

	case "payment.authorized":
		// Razorpay payment authorized but not yet captured.
		// Move CREATED to ATTEMPTED. Do NOT grant credits. Never downgrade PAID.
		rpOrderID := payload.Payload.Payment.Entity.OrderID
		if rpOrderID != "" {
			_ = s.repo.ExecTx(ctx, func(txRepo Repository) error {
				order, err := txRepo.GetOrderByRazorpayOrderIDForUpdate(ctx, rpOrderID)
				if err == nil && order.Status == StatusCreated {
					order.Status = StatusAttempted
					_ = txRepo.UpdateOrder(ctx, order)
				}
				return nil
			})
		}
		_ = s.repo.UpdateWebhookEventStatus(ctx, eventID, "PROCESSED", "payment authorization recorded")
		return &WebhookResult{EventID: eventID, Status: "PROCESSED"}, nil

	case "payment.failed":
		// Razorpay payment attempt failed.
		// Move CREATED or ATTEMPTED to FAILED.
		// Crucial safety check: A stale failure event must NEVER downgrade an already PAID order!
		rpOrderID := payload.Payload.Payment.Entity.OrderID
		if rpOrderID != "" {
			_ = s.repo.ExecTx(ctx, func(txRepo Repository) error {
				order, err := txRepo.GetOrderByRazorpayOrderIDForUpdate(ctx, rpOrderID)
				if err == nil && order.Status != StatusPaid {
					order.Status = StatusFailed
					_ = txRepo.UpdateOrder(ctx, order)
				}
				return nil
			})
		}
		_ = s.repo.UpdateWebhookEventStatus(ctx, eventID, "PROCESSED", "payment failure recorded")
		return &WebhookResult{EventID: eventID, Status: "PROCESSED"}, nil

	default:
		_ = s.repo.UpdateWebhookEventStatus(ctx, eventID, "IGNORED", "unhandled event type")
		return &WebhookResult{EventID: eventID, Status: "IGNORED"}, nil
	}
}

func (s *service) GetUserPaymentHistory(
	ctx context.Context,
	userID uuid.UUID,
	limit, offset int32,
) ([]PaymentOrder, int64, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}
	return s.repo.ListOrdersByUser(ctx, userID, limit, offset)
}

func (s *service) AdminListPayments(
	ctx context.Context,
	limit, offset int32,
) ([]PaymentOrder, int64, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}
	return s.repo.ListAllOrders(ctx, limit, offset)
}

func (s *service) GetJobStats(ctx context.Context) (map[string]int64, error) {
	return s.repo.GetJobStats(ctx)
}
