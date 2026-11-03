package progress

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/adaptive-dsa/platform/internal/identity"
	"github.com/adaptive-dsa/platform/internal/platform/httperr"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// Handler serves HTTP endpoints for attempts, user progress, and spaced repetition revisions.
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

// RegisterRoutes mounts progress and revision routes.
func (h *Handler) RegisterRoutes(r chi.Router, authMiddleware, requireAuth func(http.Handler) http.Handler) {
	r.Group(func(r chi.Router) {
		r.Use(authMiddleware)
		r.Use(requireAuth)

		r.Post("/questions/{id}/attempts", h.HandleRecordAttempt)
		r.Get("/questions/{id}/progress", h.HandleGetQuestionProgress)
		r.Get("/progress", h.HandleGetProgressSummary)
		r.Get("/progress/all", h.HandleListAllUserProgress)
		r.Post("/progress/quick-mark", h.HandleQuickMark)
		r.Post("/progress/unmark", h.HandleUnmark)
		r.Get("/revisions/due", h.HandleListDueRevisions)
		r.Get("/user/export", h.HandleExportUserData)
	})
}

type RecordAttemptPayload struct {
	RoadmapID             *string  `json:"roadmap_id,omitempty"`
	Outcome               string   `json:"outcome"`
	TimeSpentSecs         int32    `json:"time_spent_secs"`
	DifficultyExperienced string   `json:"difficulty_experienced"`
	QualityRating         int32    `json:"quality_rating"`
	MistakeTags           []string `json:"mistake_tags"`
	ApproachSummary       *string  `json:"approach_summary,omitempty"`
	CodeSnippet           *string  `json:"code_snippet,omitempty"`
}

type RecordAttemptResponse struct {
	Attempt  *QuestionAttempt      `json:"attempt"`
	Progress *UserQuestionProgress `json:"progress"`
	Revision *RevisionSchedule     `json:"revision"`
}

func (h *Handler) HandleRecordAttempt(w http.ResponseWriter, r *http.Request) {
	user, ok := identity.UserFromContext(r.Context())
	if !ok || user == nil {
		httperr.WriteJSON(w, r, http.StatusUnauthorized, "Unauthorized", "Authentication required", "UNAUTHORIZED")
		return
	}

	qIDParam := chi.URLParam(r, "id")
	questionID, err := uuid.Parse(qIDParam)
	if err != nil {
		httperr.BadRequest(w, r, "Invalid question ID format; must be UUID")
		return
	}

	ct := r.Header.Get("Content-Type")
	if !strings.HasPrefix(strings.ToLower(ct), "application/json") {
		httperr.BadRequest(w, r, "Content-Type must be application/json")
		return
	}

	var payload RecordAttemptPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		httperr.BadRequest(w, r, "Malformed JSON request payload")
		return
	}

	outcome := AttemptOutcome(payload.Outcome)
	if !outcome.IsValid() {
		httperr.BadRequest(w, r, "Invalid outcome; must be SOLVED_INDEPENDENTLY, SOLVED_WITH_HINTS, SOLVED_AFTER_EDITORIAL, or UNRESOLVED_FAILED")
		return
	}

	if payload.QualityRating < 0 || payload.QualityRating > 5 {
		httperr.BadRequest(w, r, "Quality rating must be between 0 and 5")
		return
	}

	diffExp := PerceivedDifficulty(payload.DifficultyExperienced)
	if diffExp == "" {
		diffExp = PerceivedJustRight
	} else if !diffExp.IsValid() {
		httperr.BadRequest(w, r, "Invalid difficulty_experienced; must be EASY, JUST_RIGHT, CHALLENGING, or OVERWHELMING")
		return
	}

	var roadmapID *uuid.UUID
	if payload.RoadmapID != nil && *payload.RoadmapID != "" {
		if rid, err := uuid.Parse(*payload.RoadmapID); err == nil {
			roadmapID = &rid
		}
	}

	req := RecordAttemptRequest{
		UserID:                user.ID,
		QuestionID:            questionID,
		RoadmapID:             roadmapID,
		Outcome:               outcome,
		TimeSpentSecs:         payload.TimeSpentSecs,
		DifficultyExperienced: diffExp,
		QualityRating:         payload.QualityRating,
		MistakeTags:           payload.MistakeTags,
		ApproachSummary:       payload.ApproachSummary,
		CodeSnippet:           payload.CodeSnippet,
	}

	attempt, progress, revision, err := h.service.RecordAttempt(r.Context(), req)
	if err != nil {
		httperr.InternalServerError(w, r, err, h.logger)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(RecordAttemptResponse{
		Attempt:  attempt,
		Progress: progress,
		Revision: revision,
	})
}

func (h *Handler) HandleGetQuestionProgress(w http.ResponseWriter, r *http.Request) {
	user, ok := identity.UserFromContext(r.Context())
	if !ok || user == nil {
		httperr.WriteJSON(w, r, http.StatusUnauthorized, "Unauthorized", "Authentication required", "UNAUTHORIZED")
		return
	}

	qIDParam := chi.URLParam(r, "id")
	questionID, err := uuid.Parse(qIDParam)
	if err != nil {
		httperr.BadRequest(w, r, "Invalid question ID format")
		return
	}

	progress, err := h.service.GetQuestionProgress(r.Context(), user.ID, questionID)
	if err != nil {
		if errors.Is(err, ErrProgressNotFound) {
			httperr.NotFound(w, r, "No progress recorded yet for this question")
			return
		}
		httperr.InternalServerError(w, r, err, h.logger)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(progress)
}

func (h *Handler) HandleGetProgressSummary(w http.ResponseWriter, r *http.Request) {
	user, ok := identity.UserFromContext(r.Context())
	if !ok || user == nil {
		httperr.WriteJSON(w, r, http.StatusUnauthorized, "Unauthorized", "Authentication required", "UNAUTHORIZED")
		return
	}

	summary, err := h.service.GetUserProgressSummary(r.Context(), user.ID)
	if err != nil {
		httperr.InternalServerError(w, r, err, h.logger)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(summary)
}

type DueRevisionsResponse struct {
	Revisions []RevisionSchedule `json:"revisions"`
	Total     int64              `json:"total"`
	Limit     int32              `json:"limit"`
	Offset    int32              `json:"offset"`
	Page      int32              `json:"page"`
	PerPage   int32              `json:"per_page"`
}

func (h *Handler) HandleListDueRevisions(w http.ResponseWriter, r *http.Request) {
	user, ok := identity.UserFromContext(r.Context())
	if !ok || user == nil {
		httperr.WriteJSON(w, r, http.StatusUnauthorized, "Unauthorized", "Authentication required", "UNAUTHORIZED")
		return
	}

	q := r.URL.Query()
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

	revisions, total, err := h.service.ListDueRevisions(r.Context(), user.ID, limit, offset)
	if err != nil {
		httperr.InternalServerError(w, r, err, h.logger)
		return
	}

	page := (offset / limit) + 1

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(DueRevisionsResponse{
		Revisions: revisions,
		Total:     total,
		Limit:     limit,
		Offset:    offset,
		Page:      page,
		PerPage:   limit,
	})
}

// HandleExportUserData serializes candidate learning history and exports it as an attachment.
func (h *Handler) HandleExportUserData(w http.ResponseWriter, r *http.Request) {
	user, ok := identity.UserFromContext(r.Context())
	if !ok || user == nil {
		httperr.WriteJSON(w, r, http.StatusUnauthorized, "Unauthorized", "Authentication required", "UNAUTHORIZED")
		return
	}

	data, err := h.service.ExportUserData(r.Context(), user.ID)
	if err != nil {
		httperr.InternalServerError(w, r, err, h.logger)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Disposition", `attachment; filename="adaptive_dsa_learning_export.json"`)
	_ = json.NewEncoder(w).Encode(data)
}

type QuickMarkPayload struct {
	QuestionID            string `json:"question_id"`
	Outcome               string `json:"outcome"`
	DifficultyExperienced string `json:"difficulty_experienced"`
	NeedsRevision         bool   `json:"needs_revision"`
	ConfidenceFirstTry    bool   `json:"confidence_first_try"`
	Notes                 string `json:"notes"`
}

func (h *Handler) HandleQuickMark(w http.ResponseWriter, r *http.Request) {
	user, ok := identity.UserFromContext(r.Context())
	if !ok || user == nil {
		httperr.WriteJSON(w, r, http.StatusUnauthorized, "Unauthorized", "Authentication required", "UNAUTHORIZED")
		return
	}

	var payload QuickMarkPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		httperr.BadRequest(w, r, "Malformed JSON request payload")
		return
	}

	qID, err := uuid.Parse(payload.QuestionID)
	if err != nil {
		httperr.BadRequest(w, r, "Invalid question_id format")
		return
	}

	outcome := AttemptOutcome(payload.Outcome)
	if !outcome.IsValid() {
		outcome = OutcomeSolvedIndependently
	}

	diffExp := PerceivedDifficulty(payload.DifficultyExperienced)
	if !diffExp.IsValid() {
		diffExp = PerceivedJustRight
	}

	prog, err := h.service.QuickMark(r.Context(), QuickMarkRequest{
		UserID:                user.ID,
		QuestionID:            qID,
		Outcome:               outcome,
		DifficultyExperienced: diffExp,
		NeedsRevision:         payload.NeedsRevision,
		ConfidenceFirstTry:    payload.ConfidenceFirstTry,
		Notes:                 payload.Notes,
	})
	if err != nil {
		httperr.InternalServerError(w, r, err, h.logger)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"status":   "ok",
		"progress": prog,
	})
}

type UnmarkPayload struct {
	QuestionID string `json:"question_id"`
}

func (h *Handler) HandleUnmark(w http.ResponseWriter, r *http.Request) {
	user, ok := identity.UserFromContext(r.Context())
	if !ok || user == nil {
		httperr.WriteJSON(w, r, http.StatusUnauthorized, "Unauthorized", "Authentication required", "UNAUTHORIZED")
		return
	}

	var payload UnmarkPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		httperr.BadRequest(w, r, "Malformed JSON request payload")
		return
	}

	qID, err := uuid.Parse(payload.QuestionID)
	if err != nil {
		httperr.BadRequest(w, r, "Invalid question_id format")
		return
	}

	if err := h.service.UnmarkProgress(r.Context(), user.ID, qID); err != nil {
		httperr.InternalServerError(w, r, err, h.logger)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "ok",
	})
}

func (h *Handler) HandleListAllUserProgress(w http.ResponseWriter, r *http.Request) {
	user, ok := identity.UserFromContext(r.Context())
	if !ok || user == nil {
		httperr.WriteJSON(w, r, http.StatusUnauthorized, "Unauthorized", "Authentication required", "UNAUTHORIZED")
		return
	}

	items, err := h.service.ListUserProgress(r.Context(), user.ID)
	if err != nil {
		httperr.InternalServerError(w, r, err, h.logger)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"progress": items,
	})
}

