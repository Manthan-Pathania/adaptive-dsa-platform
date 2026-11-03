package credits

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/adaptive-dsa/platform/internal/identity"
	"github.com/adaptive-dsa/platform/internal/platform/httperr"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type Handler struct {
	service Service
	logger  *slog.Logger
}

func NewHandler(service Service, log *slog.Logger) *Handler {
	if log == nil {
		log = slog.Default()
	}
	return &Handler{
		service: service,
		logger:  log,
	}
}

func (h *Handler) RegisterRoutes(r chi.Router, authMiddleware, requireAuth func(http.Handler) http.Handler) {
	r.Group(func(r chi.Router) {
		r.Use(authMiddleware)
		r.Use(requireAuth)

		r.Get("/credits/balance", h.HandleGetBalance)
		r.Get("/credits/ledger", h.HandleGetLedger)
	})
}

func (h *Handler) RegisterAdminRoutes(r chi.Router, authMiddleware, requireAuth, requireAdmin func(http.Handler) http.Handler) {
	r.Group(func(r chi.Router) {
		r.Use(authMiddleware)
		r.Use(requireAuth)
		r.Use(requireAdmin)

		r.Post("/admin/users/{id}/credits", h.HandleAdminGrantCredits)
	})
}

type BalanceResponse struct {
	Balance       int32         `json:"balance"`
	RecentEntries []LedgerEntry `json:"recent_entries"`
}

func (h *Handler) HandleGetBalance(w http.ResponseWriter, r *http.Request) {
	user, ok := identity.UserFromContext(r.Context())
	if !ok || user == nil {
		httperr.WriteJSON(w, r, http.StatusUnauthorized, "Unauthorized", "Authentication required", "UNAUTHORIZED")
		return
	}

	acc, err := h.service.EnsureAccount(r.Context(), user.ID, 50)
	if err != nil {
		h.logger.Error("failed to get or ensure credit account", "error", err, "user_id", user.ID)
		httperr.InternalServerError(w, r, err, h.logger)
		return
	}
	bal := acc.CachedBalance

	entries, _, _ := h.service.GetLedger(r.Context(), user.ID, 5, 0)

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(BalanceResponse{
		Balance:       bal,
		RecentEntries: entries,
	})
}

type LedgerListResponse struct {
	Entries []LedgerEntry `json:"entries"`
	Total   int64         `json:"total"`
	Limit   int32         `json:"limit"`
	Offset  int32         `json:"offset"`
}

func (h *Handler) HandleGetLedger(w http.ResponseWriter, r *http.Request) {
	user, ok := identity.UserFromContext(r.Context())
	if !ok || user == nil {
		httperr.WriteJSON(w, r, http.StatusUnauthorized, "Unauthorized", "Authentication required", "UNAUTHORIZED")
		return
	}

	var limit int32 = 20
	var offset int32 = 0

	q := r.URL.Query()
	if l := q.Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 {
			limit = int32(parsed)
		}
	}
	if o := q.Get("offset"); o != "" {
		if parsed, err := strconv.Atoi(o); err == nil && parsed >= 0 {
			offset = int32(parsed)
		}
	} else if pageStr := q.Get("page"); pageStr != "" {
		page, _ := strconv.Atoi(pageStr)
		if page < 1 {
			page = 1
		}
		perPage := int(limit)
		if ppStr := q.Get("per_page"); ppStr != "" {
			if pp, err := strconv.Atoi(ppStr); err == nil && pp > 0 {
				perPage = pp
				limit = int32(pp)
			}
		}
		offset = int32((page - 1) * perPage)
	}

	entries, total, err := h.service.GetLedger(r.Context(), user.ID, limit, offset)
	if err != nil {
		h.logger.Error("failed to list ledger entries", "error", err, "user_id", user.ID)
		httperr.InternalServerError(w, r, err, h.logger)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(LedgerListResponse{
		Entries: entries,
		Total:   total,
		Limit:   limit,
		Offset:  offset,
	})
}

type AdminGrantPayload struct {
	Amount int32  `json:"amount"`
	Reason string `json:"reason"`
}

func (h *Handler) HandleAdminGrantCredits(w http.ResponseWriter, r *http.Request) {
	admin, ok := identity.UserFromContext(r.Context())
	if !ok || admin == nil || admin.Role != identity.RoleAdmin {
		httperr.WriteJSON(w, r, http.StatusForbidden, "Forbidden", "Admin privilege required", "FORBIDDEN")
		return
	}

	targetIDParam := chi.URLParam(r, "id")
	targetUserID, err := uuid.Parse(targetIDParam)
	if err != nil {
		httperr.BadRequest(w, r, "Invalid target user ID format; must be UUID")
		return
	}

	ct := r.Header.Get("Content-Type")
	if !strings.HasPrefix(strings.ToLower(ct), "application/json") {
		httperr.BadRequest(w, r, "Content-Type must be application/json")
		return
	}

	var payload AdminGrantPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		httperr.BadRequest(w, r, "Malformed JSON request payload")
		return
	}

	if payload.Amount <= 0 {
		httperr.BadRequest(w, r, "Amount must be greater than zero")
		return
	}
	if strings.TrimSpace(payload.Reason) == "" {
		httperr.BadRequest(w, r, "Reason must not be empty")
		return
	}

	entry, err := h.service.GrantPromotionalCredits(r.Context(), admin.ID, targetUserID, payload.Amount, payload.Reason)
	if err != nil {
		h.logger.Error("failed to grant promotional credits", "error", err, "admin_id", admin.ID, "target_id", targetUserID)
		httperr.InternalServerError(w, r, err, h.logger)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"entry":   entry,
		"message": "Credits granted successfully",
	})
}
