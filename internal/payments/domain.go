package payments

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
)

var (
	ErrInvalidPackage          = errors.New("invalid credit package selected")
	ErrInvalidAmount           = errors.New("invalid payment amount")
	ErrOrderNotFound           = errors.New("payment order not found")
	ErrOrderAlreadyPaid        = errors.New("payment order has already been fulfilled")
	ErrInvalidSignature        = errors.New("invalid payment signature")
	ErrInvalidWebhookSignature = errors.New("invalid webhook signature")
	ErrDuplicateWebhook        = errors.New("duplicate webhook event")
	ErrProviderUnavailable     = errors.New("payment gateway provider unavailable")
	ErrMismatchedPaymentData   = errors.New("mismatch in payment amount, currency, or order reference")
	ErrPaymentNotCaptured      = errors.New("payment entity is not in captured status")
)

type PaymentOrderStatus string

const (
	StatusCreated   PaymentOrderStatus = "CREATED"
	StatusAttempted PaymentOrderStatus = "ATTEMPTED"
	StatusPaid      PaymentOrderStatus = "PAID"
	StatusFailed    PaymentOrderStatus = "FAILED"
)

func (s PaymentOrderStatus) IsValid() bool {
	switch s {
	case StatusCreated, StatusAttempted, StatusPaid, StatusFailed:
		return true
	default:
		return false
	}
}

// CreditPackage models an immutable backend-defined credit purchasing tier.
type CreditPackage struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Credits     int32  `json:"credits"`
	AmountPaise int32  `json:"amount_paise"` // Price in INR paise (integer, never float)
	Currency    string `json:"currency"`     // Strictly "INR"
	Description string `json:"description"`
	Popular     bool   `json:"popular,omitempty"`
}

var AvailablePackages = []CreditPackage{
	{
		ID:          "pkg_starter",
		Name:        "Starter Pack",
		Credits:     50,
		AmountPaise: 9900, // ₹99.00
		Currency:    "INR",
		Description: "50 AI Credits — Ideal for targeted pattern practice and initial hints.",
		Popular:     false,
	},
	{
		ID:          "pkg_pro",
		Name:        "Pro Preparation Pack",
		Credits:     150,
		AmountPaise: 24900, // ₹249.00
		Currency:    "INR",
		Description: "150 AI Credits — Best value for comprehensive roadmap mastery and code reviews.",
		Popular:     true,
	},
	{
		ID:          "pkg_master",
		Name:        "Interview Mastery Pack",
		Credits:     400,
		AmountPaise: 59900, // ₹599.00
		Currency:    "INR",
		Description: "400 AI Credits — Unlimited practice, in-depth architectural reviews, and concept Q&A.",
		Popular:     false,
	},
}

// FindPackage retrieves a package by ID or returns ErrInvalidPackage.
func FindPackage(packageID string) (*CreditPackage, error) {
	for _, p := range AvailablePackages {
		if p.ID == packageID {
			return &p, nil
		}
	}
	return nil, ErrInvalidPackage
}

// PaymentOrder models a persisted customer payment intent.
type PaymentOrder struct {
	ID                uuid.UUID          `json:"id"`
	UserID            uuid.UUID          `json:"user_id"`
	PackageID         string             `json:"package_id"`
	AmountPaise       int32              `json:"amount_paise"`
	Currency          string             `json:"currency"`
	CreditsAmount     int32              `json:"credits_amount"`
	Status            PaymentOrderStatus `json:"status"`
	RazorpayOrderID   string             `json:"razorpay_order_id,omitempty"`
	RazorpayPaymentID string             `json:"razorpay_payment_id,omitempty"`
	RazorpaySignature string             `json:"razorpay_signature,omitempty"`
	IdempotencyKey    string             `json:"idempotency_key,omitempty"`
	CreatedAt         time.Time          `json:"created_at"`
	UpdatedAt         time.Time          `json:"updated_at"`
}

// WebhookEvent models a recorded payment webhook delivery for audit and deduplication.
type WebhookEvent struct {
	ID           uuid.UUID  `json:"id"`
	EventID      string     `json:"event_id"`
	EventType    string     `json:"event_type"`
	Payload      []byte     `json:"payload"`
	Status       string     `json:"status"` // 'RECEIVED', 'PROCESSED', 'FAILED', 'IGNORED'
	ErrorMessage string     `json:"error_message,omitempty"`
	ProcessedAt  *time.Time `json:"processed_at,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
}

// CreateProviderOrderRequest specifies order creation parameters sent to Razorpay.
type CreateProviderOrderRequest struct {
	AmountPaise int32             `json:"amount"`
	Currency    string            `json:"currency"`
	Receipt     string            `json:"receipt"`
	Notes       map[string]string `json:"notes"`
}

// ProviderOrderResponse holds the public response returned by the payment gateway.
type ProviderOrderResponse struct {
	ID       string `json:"id"` // Razorpay order ID (e.g., order_EKwxwAgItmmXdp)
	Amount   int32  `json:"amount"`
	Currency string `json:"currency"`
	Status   string `json:"status"`
}

// PaymentDetails captures authoritative payment state returned by the gateway.
type PaymentDetails struct {
	ID          string `json:"id"`
	OrderID     string `json:"order_id"`
	AmountPaise int32  `json:"amount"`
	Currency    string `json:"currency"`
	Status      string `json:"status"` // "captured", "authorized", "failed"
}

// PaymentProvider abstracts payment gateway network calls and cryptographic signature verifications.
type PaymentProvider interface {
	CreateOrder(ctx context.Context, req CreateProviderOrderRequest) (*ProviderOrderResponse, error)
	GetPayment(ctx context.Context, paymentID string) (*PaymentDetails, error)
	VerifyCheckoutSignature(orderID, paymentID, signature string) bool
	VerifyWebhookSignature(rawBody []byte, signature string) bool
	KeyID() string
}
