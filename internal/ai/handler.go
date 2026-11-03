package ai

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/adaptive-dsa/platform/internal/credits"
	"github.com/adaptive-dsa/platform/internal/identity"
	"github.com/adaptive-dsa/platform/internal/platform/httperr"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type Handler struct {
	service         Service
	identityService identity.Service
	logger          *slog.Logger
}

func NewHandler(service Service, log *slog.Logger, idSvc ...identity.Service) *Handler {
	if log == nil {
		log = slog.Default()
	}
	var identitySvc identity.Service
	if len(idSvc) > 0 {
		identitySvc = idSvc[0]
	}
	return &Handler{
		service:         service,
		identityService: identitySvc,
		logger:          log,
	}
}

func (h *Handler) RegisterRoutes(r chi.Router, authMiddleware, requireAuth func(http.Handler) http.Handler) {
	// Public pricing discovery endpoint
	r.Get("/ai/pricing", h.HandleGetPricing)

	r.Group(func(r chi.Router) {
		r.Use(authMiddleware)
		r.Use(requireAuth)

		r.Post("/ai/hints", h.HandleRequestHint)
		r.Post("/ai/reviews", h.HandleReviewApproach)
		r.Post("/ai/explain", h.HandleExplainConcept)
		r.Get("/ai/requests", h.HandleListRequests)
		r.Post("/ai/roadmaps/generate", h.HandleGenerateRoadmap)
		r.Post("/roadmaps/generate", h.HandleGenerateRoadmap)
	})
}

type PricingResponse struct {
	Hints          map[string]int32 `json:"hints"`
	ApproachReview int32            `json:"approach_review"`
	ConceptExplain int32            `json:"concept_explain"`
}

func (h *Handler) HandleGetPricing(w http.ResponseWriter, r *http.Request) {
	resp := PricingResponse{
		Hints: map[string]int32{
			"1": HintLevelNudge.CreditCost(),
			"2": HintLevelStructural.CreditCost(),
			"3": HintLevelInvariant.CreditCost(),
			"4": HintLevelPseudocode.CreditCost(),
		},
		ApproachReview: CostApproachReview,
		ConceptExplain: CostConceptExplain,
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

type HintRequestPayload struct {
	QuestionID     string `json:"question_id"`
	Level          int    `json:"level"`
	CandidateNotes string `json:"candidate_notes"`
}

func (h *Handler) HandleRequestHint(w http.ResponseWriter, r *http.Request) {
	user, ok := identity.UserFromContext(r.Context())
	if !ok || user == nil {
		httperr.WriteJSON(w, r, http.StatusUnauthorized, "Unauthorized", "Authentication required", "UNAUTHORIZED")
		return
	}

	ct := r.Header.Get("Content-Type")
	if !strings.HasPrefix(strings.ToLower(ct), "application/json") {
		httperr.BadRequest(w, r, "Content-Type must be application/json")
		return
	}

	var payload HintRequestPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		httperr.BadRequest(w, r, "Malformed JSON request payload")
		return
	}

	var qID uuid.UUID
	if payload.QuestionID != "" {
		parsed, err := uuid.Parse(payload.QuestionID)
		if err != nil {
			httperr.BadRequest(w, r, "Invalid question_id format; must be UUID")
			return
		}
		qID = parsed
	}

	hintLevel := HintLevel(payload.Level)
	if !hintLevel.IsValid() {
		httperr.BadRequest(w, r, "Invalid level: must be 1 (Nudge), 2 (Structural), 3 (Invariant), or 4 (Pseudocode)")
		return
	}

	result, err := h.service.RequestHint(r.Context(), user.ID, GenerateHintRequest{
		QuestionID:     qID,
		Level:          hintLevel,
		CandidateNotes: payload.CandidateNotes,
	})
	if err != nil {
		h.handleAIError(w, r, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(result)
}

type ReviewRequestPayload struct {
	QuestionID    string `json:"question_id"`
	CandidateCode string `json:"candidate_code"`
	ProposedTime  string `json:"proposed_time"`
	ProposedSpace string `json:"proposed_space"`
}

func (h *Handler) HandleReviewApproach(w http.ResponseWriter, r *http.Request) {
	user, ok := identity.UserFromContext(r.Context())
	if !ok || user == nil {
		httperr.WriteJSON(w, r, http.StatusUnauthorized, "Unauthorized", "Authentication required", "UNAUTHORIZED")
		return
	}

	var payload ReviewRequestPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		httperr.BadRequest(w, r, "Malformed JSON request payload")
		return
	}

	var qID uuid.UUID
	if payload.QuestionID != "" {
		if parsed, err := uuid.Parse(payload.QuestionID); err == nil {
			qID = parsed
		}
	}

	if strings.TrimSpace(payload.CandidateCode) == "" {
		httperr.BadRequest(w, r, "Candidate code or approach description cannot be empty")
		return
	}

	result, err := h.service.ReviewApproach(r.Context(), user.ID, ReviewApproachRequest{
		QuestionID:    qID,
		CandidateCode: payload.CandidateCode,
		ProposedTime:  payload.ProposedTime,
		ProposedSpace: payload.ProposedSpace,
	})
	if err != nil {
		h.handleAIError(w, r, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(result)
}

type ExplainRequestPayload struct {
	TopicName      string `json:"topic_name"`
	PatternName    string `json:"pattern_name"`
	CandidateQuery string `json:"candidate_query"`
}

func (h *Handler) HandleExplainConcept(w http.ResponseWriter, r *http.Request) {
	user, ok := identity.UserFromContext(r.Context())
	if !ok || user == nil {
		httperr.WriteJSON(w, r, http.StatusUnauthorized, "Unauthorized", "Authentication required", "UNAUTHORIZED")
		return
	}

	var payload ExplainRequestPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		httperr.BadRequest(w, r, "Malformed JSON request payload")
		return
	}

	if strings.TrimSpace(payload.CandidateQuery) == "" {
		httperr.BadRequest(w, r, "Candidate query cannot be empty")
		return
	}

	result, err := h.service.ExplainConcept(r.Context(), user.ID, ExplainConceptRequest{
		TopicName:      payload.TopicName,
		PatternName:    payload.PatternName,
		CandidateQuery: payload.CandidateQuery,
	})
	if err != nil {
		h.handleAIError(w, r, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(result)
}

func (h *Handler) HandleListRequests(w http.ResponseWriter, r *http.Request) {
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
	}

	reqs, total, err := h.service.ListUserRequests(r.Context(), user.ID, limit, offset)
	if err != nil {
		h.logger.Error("failed to list user ai requests", "error", err, "user_id", user.ID)
		httperr.InternalServerError(w, r, err, h.logger)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"requests": reqs,
		"total":    total,
		"limit":    limit,
		"offset":   offset,
	})
}

func (h *Handler) handleAIError(w http.ResponseWriter, r *http.Request, err error) {
	if errors.Is(err, credits.ErrInsufficientCredits) {
		httperr.WriteJSON(w, r, http.StatusPaymentRequired, "Payment Required", "Insufficient credit balance to request AI tutoring. Please recharge credits.", "INSUFFICIENT_CREDITS")
		return
	}
	if errors.Is(err, ErrQuotaExceeded) {
		httperr.WriteJSON(w, r, http.StatusTooManyRequests, "AI Quota Exceeded", "AI tutoring is temporarily rate-limited. Please retry in 60 seconds.", "AI_RATE_LIMIT")
		return
	}
	if errors.Is(err, ErrProviderUnavailable) {
		httperr.WriteJSON(w, r, http.StatusServiceUnavailable, "AI Service Unavailable", "AI tutor provider is temporarily unavailable. Zero credits were charged.", "AI_UNAVAILABLE")
		return
	}

	h.logger.Error("unhandled ai error", "error", err)
	httperr.InternalServerError(w, r, err, h.logger)
}

func (h *Handler) HandleGenerateRoadmap(w http.ResponseWriter, r *http.Request) {
	user, ok := identity.UserFromContext(r.Context())
	if !ok || user == nil {
		httperr.WriteJSON(w, r, http.StatusUnauthorized, "Unauthorized", "Authentication required", "UNAUTHORIZED")
		return
	}

	var req GenerateRoadmapRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httperr.BadRequest(w, r, "Invalid request payload format")
		return
	}

	roadmap, err := h.service.CreateCustomRoadmap(r.Context(), user.ID, req)
	if err != nil {
		h.logger.Error("failed to generate custom roadmap", "error", err, "user_id", user.ID)
		httperr.InternalServerError(w, r, err, h.logger)
		return
	}

	if h.identityService != nil {
		prof, err := h.identityService.GetProfile(r.Context(), user.ID)
		if err == nil && prof != nil {
			prof.SelectedRoadmapID = &roadmap.ID
			_, _ = h.identityService.UpdateProfile(r.Context(), prof)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(roadmap)
}
