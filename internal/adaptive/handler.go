package adaptive

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

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

		r.Get("/adaptive/daily", h.HandleGetDailyPlan)
		r.Get("/adaptive/weak-patterns", h.HandleGetWeakPatterns)
	})
}

func (h *Handler) HandleGetDailyPlan(w http.ResponseWriter, r *http.Request) {
	user, ok := identity.UserFromContext(r.Context())
	if !ok || user == nil {
		httperr.WriteJSON(w, r, http.StatusUnauthorized, "Unauthorized", "Authentication required", "UNAUTHORIZED")
		return
	}

	var roadmapID *uuid.UUID
	if rIDStr := r.URL.Query().Get("roadmap_id"); rIDStr != "" {
		if parsed, err := uuid.Parse(rIDStr); err == nil {
			roadmapID = &parsed
		}
	}

	plan, err := h.service.GetDailyPlan(r.Context(), user.ID, roadmapID, time.Now().UTC())
	if err != nil {
		h.logger.Error("failed to generate daily plan", "error", err, "user_id", user.ID)
		httperr.InternalServerError(w, r, err, h.logger)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(plan)
}

func (h *Handler) HandleGetWeakPatterns(w http.ResponseWriter, r *http.Request) {
	user, ok := identity.UserFromContext(r.Context())
	if !ok || user == nil {
		httperr.WriteJSON(w, r, http.StatusUnauthorized, "Unauthorized", "Authentication required", "UNAUTHORIZED")
		return
	}

	weak, err := h.service.GetWeakPatterns(r.Context(), user.ID)
	if err != nil {
		h.logger.Error("failed to get weak patterns", "error", err, "user_id", user.ID)
		httperr.InternalServerError(w, r, err, h.logger)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"weak_patterns": weak,
		"count":         len(weak),
	})
}
