package payments

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/adaptive-dsa/platform/internal/identity"
	"github.com/adaptive-dsa/platform/internal/platform/httperr"
	"github.com/go-chi/chi/v5"
)

type Handler struct {
	service Service
	logger  *slog.Logger
}

func NewHandler(service Service, logger *slog.Logger) *Handler {
	if logger == nil {
		logger = slog.Default()
	}
	return &Handler{
		service: service,
		logger:  logger,
	}
}

func (h *Handler) RegisterRoutes(
	r chi.Router,
	authMw func(http.Handler) http.Handler,
	requireAuth func(http.Handler) http.Handler,
	csrfMw func(http.Handler) http.Handler,
	requireAdmin func(http.Handler) http.Handler,
) {
	// 1. Public package catalogue endpoint
	r.Get("/payments/packages", h.HandleGetPackages)

	// 2. Public webhook receiver (protected by HMAC-SHA256 signature verification)
	r.Post("/payments/webhook", h.HandleWebhook)

	// 3. Authenticated customer endpoints
	r.Group(func(r chi.Router) {
		r.Use(authMw)
		r.Use(requireAuth)

		r.Get("/payments/history", h.HandleGetUserHistory)

		// Mutating payment endpoints protected by double-submit Anti-CSRF
		r.Group(func(r chi.Router) {
			r.Use(csrfMw)
			r.Post("/payments/orders", h.HandleCreateOrder)
			r.Post("/payments/verify", h.HandleVerifyCheckout)
		})
	})

	// 4. Admin endpoints
	r.Group(func(r chi.Router) {
		r.Use(authMw)
		r.Use(requireAuth)
		r.Use(requireAdmin)

		r.Get("/admin/payments", h.HandleAdminListPayments)
	})
}

// HandleGetPackages returns the list of available backend-defined credit packages.
func (h *Handler) HandleGetPackages(w http.ResponseWriter, r *http.Request) {
	packages := h.service.GetPackages()
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"packages": packages,
	})
}

type CreateOrderRequestBody struct {
	PackageID      string `json:"package_id"`
	IdempotencyKey string `json:"idempotency_key,omitempty"`
}

// HandleCreateOrder creates a server-side payment order.
func (h *Handler) HandleCreateOrder(w http.ResponseWriter, r *http.Request) {
	user, ok := identity.UserFromContext(r.Context())
	if !ok || user == nil {
		httperr.WriteJSON(w, r, http.StatusUnauthorized, "Unauthorized", "Authentication required", "UNAUTHORIZED")
		return
	}

	var reqBody CreateOrderRequestBody
	if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
		httperr.BadRequest(w, r, "Invalid JSON payload")
		return
	}

	if reqBody.PackageID == "" {
		httperr.BadRequest(w, r, "package_id is required")
		return
	}

	res, err := h.service.CreatePaymentOrder(r.Context(), user.ID, reqBody.PackageID, reqBody.IdempotencyKey)
	if err != nil {
		if errors.Is(err, ErrInvalidPackage) {
			httperr.BadRequest(w, r, "Selected credit package does not exist")
			return
		}
		h.logger.ErrorContext(r.Context(), "failed to create payment order", "error", err, "user_id", user.ID)
		httperr.InternalServerError(w, r, err, h.logger)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(res)
}

// HandleVerifyCheckout verifies client-submitted Razorpay Checkout modal signatures.
func (h *Handler) HandleVerifyCheckout(w http.ResponseWriter, r *http.Request) {
	user, ok := identity.UserFromContext(r.Context())
	if !ok || user == nil {
		httperr.WriteJSON(w, r, http.StatusUnauthorized, "Unauthorized", "Authentication required", "UNAUTHORIZED")
		return
	}

	var req VerifyCheckoutRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httperr.BadRequest(w, r, "Invalid JSON payload")
		return
	}

	if req.RazorpayOrderID == "" || req.RazorpayPaymentID == "" || req.RazorpaySignature == "" {
		httperr.BadRequest(w, r, "razorpay_order_id, razorpay_payment_id, and razorpay_signature are required")
		return
	}

	order, err := h.service.VerifyCheckout(r.Context(), user.ID, req)
	if err != nil {
		if errors.Is(err, ErrInvalidSignature) {
			httperr.BadRequest(w, r, "Cryptographic payment signature mismatch")
			return
		}
		if errors.Is(err, ErrPaymentNotCaptured) {
			httperr.BadRequest(w, r, "Payment has not been confirmed as captured by payment gateway")
			return
		}
		if errors.Is(err, ErrOrderNotFound) {
			httperr.NotFound(w, r, "Associated payment order was not found")
			return
		}
		h.logger.ErrorContext(r.Context(), "checkout verification error", "error", err, "user_id", user.ID)
		httperr.InternalServerError(w, r, err, h.logger)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"status":          "success",
		"message":         "Payment verified and credits added to account",
		"order":           order,
		"credits_granted": order.CreditsAmount,
	})
}

// HandleWebhook processes server-to-server Razorpay webhook events.
func (h *Handler) HandleWebhook(w http.ResponseWriter, r *http.Request) {
	// Limit request body to 1MB to prevent memory exhaustion
	r.Body = http.MaxBytesReader(w, r.Body, 1024*1024)

	rawBody, err := io.ReadAll(r.Body)
	if err != nil {
		httperr.BadRequest(w, r, "Failed to read request payload")
		return
	}

	sig := r.Header.Get("X-Razorpay-Signature")
	if sig == "" {
		httperr.BadRequest(w, r, "Missing X-Razorpay-Signature header")
		return
	}

	eventIDHeader := r.Header.Get("X-Razorpay-Event-Id")
	res, err := h.service.ProcessWebhook(r.Context(), rawBody, sig, eventIDHeader)
	if err != nil {
		if errors.Is(err, ErrInvalidWebhookSignature) {
			httperr.BadRequest(w, r, "Invalid webhook signature")
			return
		}
		if errors.Is(err, ErrMismatchedPaymentData) {
			httperr.BadRequest(w, r, "Mismatched payment data")
			return
		}
		h.logger.ErrorContext(r.Context(), "webhook processing error", "error", err)
		httperr.InternalServerError(w, r, err, h.logger)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(res)
}

// HandleGetUserHistory returns the authenticated user's payment orders.
func (h *Handler) HandleGetUserHistory(w http.ResponseWriter, r *http.Request) {
	user, ok := identity.UserFromContext(r.Context())
	if !ok || user == nil {
		httperr.WriteJSON(w, r, http.StatusUnauthorized, "Unauthorized", "Authentication required", "UNAUTHORIZED")
		return
	}

	limit := int32(20)
	offset := int32(0)
	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 {
			limit = int32(parsed)
		}
	}
	if o := r.URL.Query().Get("offset"); o != "" {
		if parsed, err := strconv.Atoi(o); err == nil && parsed >= 0 {
			offset = int32(parsed)
		}
	}

	orders, total, err := h.service.GetUserPaymentHistory(r.Context(), user.ID, limit, offset)
	if err != nil {
		h.logger.ErrorContext(r.Context(), "failed to list user payments", "error", err, "user_id", user.ID)
		httperr.InternalServerError(w, r, err, h.logger)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"orders": orders,
		"total":  total,
		"limit":  limit,
		"offset": offset,
	})
}

// HandleAdminListPayments returns all platform payment orders for administrator auditing.
func (h *Handler) HandleAdminListPayments(w http.ResponseWriter, r *http.Request) {
	limit := int32(20)
	offset := int32(0)
	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 {
			limit = int32(parsed)
		}
	}
	if o := r.URL.Query().Get("offset"); o != "" {
		if parsed, err := strconv.Atoi(o); err == nil && parsed >= 0 {
			offset = int32(parsed)
		}
	}

	orders, total, err := h.service.AdminListPayments(r.Context(), limit, offset)
	if err != nil {
		h.logger.ErrorContext(r.Context(), "admin list payments failed", "error", err)
		httperr.InternalServerError(w, r, err, h.logger)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"orders": orders,
		"total":  total,
		"limit":  limit,
		"offset": offset,
	})
}
