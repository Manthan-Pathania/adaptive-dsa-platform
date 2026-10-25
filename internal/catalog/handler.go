package catalog

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/adaptive-dsa/platform/internal/platform/httperr"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// Handler serves HTTP endpoints for the question catalog and roadmaps.
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

// RegisterRoutes registers catalog endpoints on a chi.Router.
func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Get("/questions", h.HandleListQuestions)
	r.Get("/questions/{id}", h.HandleGetQuestion)
	r.Get("/topics", h.HandleListTopics)
	r.Get("/roadmaps", h.HandleListRoadmaps)
	r.Get("/roadmaps/{id}", h.HandleGetRoadmap)
}

type PaginatedQuestionsResponse struct {
	Questions []Question `json:"questions"`
	Total     int64      `json:"total"`
	Limit     int32      `json:"limit"`
	Offset    int32      `json:"offset"`
	Page      int32      `json:"page"`
	PerPage   int32      `json:"per_page"`
}

func (h *Handler) HandleListQuestions(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	search := q.Get("search")
	diff := q.Get("difficulty")

	var topicOrPatternID *uuid.UUID
	if tID := q.Get("topic_id"); tID != "" {
		if u, err := uuid.Parse(tID); err == nil {
			topicOrPatternID = &u
		}
	} else if pID := q.Get("pattern_id"); pID != "" {
		if u, err := uuid.Parse(pID); err == nil {
			topicOrPatternID = &u
		}
	}

	limit := int32(20)
	if l := q.Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 {
			limit = int32(parsed)
		}
	} else if pp := q.Get("per_page"); pp != "" {
		if parsed, err := strconv.Atoi(pp); err == nil && parsed > 0 {
			limit = int32(parsed)
		}
	}
	if limit > 100 {
		limit = 100
	}

	offset := int32(0)
	if o := q.Get("offset"); o != "" {
		if parsed, err := strconv.Atoi(o); err == nil && parsed >= 0 {
			offset = int32(parsed)
		}
	} else if p := q.Get("page"); p != "" {
		if parsed, err := strconv.Atoi(p); err == nil && parsed > 1 {
			offset = (int32(parsed) - 1) * limit
		}
	}

	filter := ListQuestionsFilter{
		SearchQuery:      search,
		Difficulty:       diff,
		TopicOrPatternID: topicOrPatternID,
		Limit:            limit,
		Offset:           offset,
	}

	questions, total, err := h.service.ListQuestions(r.Context(), filter)
	if err != nil {
		httperr.InternalServerError(w, r, err, h.logger)
		return
	}

	page := (offset / limit) + 1

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(PaginatedQuestionsResponse{
		Questions: questions,
		Total:     total,
		Limit:     limit,
		Offset:    offset,
		Page:      page,
		PerPage:   limit,
	})
}

func (h *Handler) HandleGetQuestion(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		httperr.BadRequest(w, r, "Question ID or slug is required")
		return
	}

	q, err := h.service.GetQuestion(r.Context(), id)
	if err != nil {
		if errors.Is(err, ErrQuestionNotFound) {
			httperr.NotFound(w, r, "Question not found")
			return
		}
		httperr.InternalServerError(w, r, err, h.logger)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(q)
}

func (h *Handler) HandleListTopics(w http.ResponseWriter, r *http.Request) {
	topics, err := h.service.ListTopics(r.Context())
	if err != nil {
		httperr.InternalServerError(w, r, err, h.logger)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"topics": topics,
	})
}

func (h *Handler) HandleListRoadmaps(w http.ResponseWriter, r *http.Request) {
	roadmaps, err := h.service.ListRoadmaps(r.Context())
	if err != nil {
		httperr.InternalServerError(w, r, err, h.logger)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"roadmaps": roadmaps,
	})
}

func (h *Handler) HandleGetRoadmap(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		httperr.BadRequest(w, r, "Roadmap ID or slug is required")
		return
	}

	roadmap, err := h.service.GetRoadmap(r.Context(), id)
	if err != nil {
		if errors.Is(err, ErrRoadmapNotFound) {
			httperr.NotFound(w, r, "Roadmap not found")
			return
		}
		httperr.InternalServerError(w, r, err, h.logger)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(roadmap)
}
