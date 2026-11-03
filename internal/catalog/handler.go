package catalog

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/adaptive-dsa/platform/internal/identity"
	"github.com/adaptive-dsa/platform/internal/platform/httperr"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// Handler serves HTTP endpoints for the question catalog and roadmaps.
type Handler struct {
	service Service
	logger  *slog.Logger
	appEnv  string
}

func NewHandler(service Service, log *slog.Logger, appEnv ...string) *Handler {
	if log == nil {
		log = slog.Default()
	}
	env := "development"
	if len(appEnv) > 0 && appEnv[0] != "" {
		env = appEnv[0]
	}
	return &Handler{
		service: service,
		logger:  log,
		appEnv:  env,
	}
}

// RegisterRoutes registers public / candidate catalog endpoints.
func (h *Handler) RegisterRoutes(r chi.Router, authMw ...func(http.Handler) http.Handler) {
	if len(authMw) > 0 && authMw[0] != nil {
		r.With(authMw[0]).Get("/roadmaps", h.HandleListRoadmaps)
		r.With(authMw[0]).Get("/catalog/patterns-progress", h.HandlePatternsProgress)
		r.With(authMw[0]).Post("/roadmaps/import", h.HandleImportRoadmap)
		r.With(authMw[0]).Delete("/roadmaps/{id}", h.HandleDeleteRoadmap)
	} else {
		r.Get("/roadmaps", h.HandleListRoadmaps)
		r.Get("/catalog/patterns-progress", h.HandlePatternsProgress)
		r.Post("/roadmaps/import", h.HandleImportRoadmap)
		r.Delete("/roadmaps/{id}", h.HandleDeleteRoadmap)
	}
	r.Get("/roadmaps/template", h.HandleDownloadRoadmapTemplate)
	r.Get("/questions", h.HandleListQuestions)
	r.Get("/questions/{id}", h.HandleGetQuestion)
	r.Get("/topics", h.HandleListTopics)
	r.Get("/patterns", h.HandleListPatterns)
	r.Get("/roadmaps/{id}", h.HandleGetRoadmap)
}

// RegisterAdminRoutes registers administrative catalog and bulk import endpoints.
func (h *Handler) RegisterAdminRoutes(r chi.Router, authMw, requireAuth, requireAdmin func(http.Handler) http.Handler) {
	r.Route("/admin/questions", func(r chi.Router) {
		r.Use(authMw)
		r.Use(requireAuth)
		r.Use(requireAdmin)

		r.Get("/", h.HandleAdminListQuestions)
		r.Post("/", h.HandleAdminCreateQuestion)
		r.Get("/import/history", h.HandleAdminListImportHistory)
		r.Post("/import/preview", h.HandleAdminImportPreview)
		r.Post("/import/commit", h.HandleAdminImportCommit)
		r.Get("/{id}", h.HandleGetQuestion)
		r.Put("/{id}", h.HandleAdminUpdateQuestion)
		r.Patch("/{id}/archive", h.HandleAdminToggleArchive)
	})
}

type PaginatedQuestionsResponse struct {
	Questions []Question `json:"questions"`
	Total     int64      `json:"total"`
	Limit     int32      `json:"limit"`
	Offset    int32      `json:"offset"`
	Page      int32      `json:"page"`
	PerPage   int32      `json:"per_page"`
}

// HandleListQuestions returns questions for candidates (only active questions).
func (h *Handler) HandleListQuestions(w http.ResponseWriter, r *http.Request) {
	active := true
	h.serveQuestionsList(w, r, &active)
}

// HandleAdminListQuestions returns questions for admins with full filtering including inactive/archived.
func (h *Handler) HandleAdminListQuestions(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	var isActive *bool
	activeParam := strings.ToLower(q.Get("is_active"))
	if activeParam == "true" || activeParam == "1" || activeParam == "active" {
		t := true
		isActive = &t
	} else if activeParam == "false" || activeParam == "0" || activeParam == "archived" {
		f := false
		isActive = &f
	}
	// If empty or "all", isActive remains nil (returns both active and archived)
	h.serveQuestionsList(w, r, isActive)
}

func (h *Handler) serveQuestionsList(w http.ResponseWriter, r *http.Request, isActive *bool) {
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
		IsActive:         isActive,
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

func (h *Handler) HandleListPatterns(w http.ResponseWriter, r *http.Request) {
	patterns, err := h.service.ListPatterns(r.Context())
	if err != nil {
		httperr.InternalServerError(w, r, err, h.logger)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"patterns": patterns,
	})
}

func (h *Handler) HandleListRoadmaps(w http.ResponseWriter, r *http.Request) {
	var userID *uuid.UUID
	if u, ok := identity.UserFromContext(r.Context()); ok && u != nil {
		userID = &u.ID
	}

	roadmaps, err := h.service.ListRoadmapsForUser(r.Context(), userID)
	if err != nil {
		httperr.InternalServerError(w, r, err, h.logger)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"roadmaps": roadmaps,
	})
}

func (h *Handler) HandlePatternsProgress(w http.ResponseWriter, r *http.Request) {
	var userID uuid.UUID
	if u, ok := identity.UserFromContext(r.Context()); ok && u != nil {
		userID = u.ID
	}

	progress, err := h.service.GetPatternsProgress(r.Context(), userID)
	if err != nil {
		httperr.InternalServerError(w, r, err, h.logger)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"patterns": progress,
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

// HandleDeleteRoadmap deletes a candidate's custom or imported roadmap. System template roadmaps are protected.
func (h *Handler) HandleDeleteRoadmap(w http.ResponseWriter, r *http.Request) {
	user, ok := identity.UserFromContext(r.Context())
	if !ok || user == nil {
		httperr.WriteJSON(w, r, http.StatusUnauthorized, "Unauthorized", "Authentication required to delete roadmaps", "UNAUTHORIZED")
		return
	}

	idStr := chi.URLParam(r, "id")
	roadmapID, err := uuid.Parse(idStr)
	if err != nil {
		httperr.BadRequest(w, r, "Invalid roadmap ID format")
		return
	}

	err = h.service.DeleteRoadmap(r.Context(), user.ID, string(user.Role), roadmapID)
	if err != nil {
		if errors.Is(err, ErrRoadmapNotFound) {
			httperr.NotFound(w, r, "Roadmap not found")
			return
		}
		if errors.Is(err, ErrCannotDeleteSystemTemplate) {
			httperr.WriteJSON(w, r, http.StatusForbidden, "Forbidden", "Canonical Master Roadmaps cannot be deleted", "CANNOT_DELETE_SYSTEM_ROADMAP")
			return
		}
		if errors.Is(err, ErrUnauthorizedRoadmapAction) {
			httperr.WriteJSON(w, r, http.StatusForbidden, "Forbidden", "You are not authorized to delete this roadmap", "FORBIDDEN")
			return
		}
		httperr.InternalServerError(w, r, err, h.logger)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// HandleDownloadRoadmapTemplate generates and streams the sample Excel template with the mandatory 6 columns.
func (h *Handler) HandleDownloadRoadmapTemplate(w http.ResponseWriter, r *http.Request) {
	templateBytes, err := h.service.GenerateRoadmapTemplate(r.Context())
	if err != nil {
		h.logger.Error("failed to generate roadmap template", "error", err)
		httperr.InternalServerError(w, r, err, h.logger)
		return
	}

	w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	w.Header().Set("Content-Disposition", "attachment; filename=\"Roadmap_Import_Template.xlsx\"")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(templateBytes)
}

// HandleImportRoadmap parses uploaded Excel sheet, creates a custom roadmap, and links questions.
func (h *Handler) HandleImportRoadmap(w http.ResponseWriter, r *http.Request) {
	user, ok := identity.UserFromContext(r.Context())
	if !ok || user == nil {
		httperr.WriteJSON(w, r, http.StatusUnauthorized, "Unauthorized", "Authentication required to import roadmaps", "UNAUTHORIZED")
		return
	}

	// Limit multipart memory to 20MB
	if err := r.ParseMultipartForm(20 << 20); err != nil {
		httperr.BadRequest(w, r, "Failed to parse multipart upload: "+err.Error())
		return
	}

	title := strings.TrimSpace(r.FormValue("title"))
	if title == "" {
		httperr.BadRequest(w, r, "Roadmap title is required")
		return
	}

	description := strings.TrimSpace(r.FormValue("description"))
	var descPtr *string
	if description != "" {
		descPtr = &description
	}

	targetDaysStr := strings.TrimSpace(r.FormValue("target_days"))
	targetDays := int32(30)
	if targetDaysStr != "" {
		if td, err := strconv.Atoi(targetDaysStr); err == nil && td > 0 {
			targetDays = int32(td)
		}
	}

	arrangeBy := strings.TrimSpace(r.FormValue("arrange_by"))
	if arrangeBy == "" {
		arrangeBy = "excel_order"
	}

	file, fileHeader, err := r.FormFile("file")
	if err != nil {
		httperr.BadRequest(w, r, "An Excel file (.xlsx or .xls) must be provided in the 'file' field")
		return
	}
	defer file.Close()

	fn := strings.ToLower(fileHeader.Filename)
	if !strings.HasSuffix(fn, ".xlsx") && !strings.HasSuffix(fn, ".xls") {
		httperr.BadRequest(w, r, "Invalid file format. Please upload an Excel workbook (.xlsx or .xls)")
		return
	}

	params := ImportRoadmapParams{
		Title:       title,
		Description: descPtr,
		TargetDays:  targetDays,
		ArrangeBy:   arrangeBy,
	}

	createdRoadmap, err := h.service.ImportRoadmapFromExcel(r.Context(), user.ID, params, file)
	if err != nil {
		h.logger.Warn("roadmap import rejected", "error", err, "user_id", user.ID)
		httperr.BadRequest(w, r, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(createdRoadmap)
}

// HandleAdminCreateQuestion handles manual creation of a question via admin form.
func (h *Handler) HandleAdminCreateQuestion(w http.ResponseWriter, r *http.Request) {
	var body CreateQuestionParams
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httperr.BadRequest(w, r, "Invalid request body")
		return
	}

	created, err := h.service.CreateQuestion(r.Context(), body)
	if err != nil {
		httperr.BadRequest(w, r, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(created)
}

// HandleAdminUpdateQuestion updates question fields.
func (h *Handler) HandleAdminUpdateQuestion(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		httperr.BadRequest(w, r, "Invalid question UUID")
		return
	}

	var body UpdateQuestionParams
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httperr.BadRequest(w, r, "Invalid request body")
		return
	}

	updated, err := h.service.UpdateQuestion(r.Context(), id, body)
	if err != nil {
		if errors.Is(err, ErrQuestionNotFound) {
			httperr.NotFound(w, r, "Question not found")
			return
		}
		httperr.BadRequest(w, r, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(updated)
}

// HandleAdminToggleArchive archives or restores a question without deleting history.
func (h *Handler) HandleAdminToggleArchive(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		httperr.BadRequest(w, r, "Invalid question UUID")
		return
	}

	// Fetch current status
	existing, err := h.service.GetQuestion(r.Context(), idStr)
	if err != nil {
		if errors.Is(err, ErrQuestionNotFound) {
			httperr.NotFound(w, r, "Question not found")
			return
		}
		httperr.InternalServerError(w, r, err, h.logger)
		return
	}

	targetActive := !existing.IsActive

	// If request body specified is_active explicitly, honor it
	var body struct {
		IsActive *bool `json:"is_active"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err == nil && body.IsActive != nil {
		targetActive = *body.IsActive
	}

	if err := h.service.SetQuestionActive(r.Context(), id, targetActive); err != nil {
		httperr.InternalServerError(w, r, err, h.logger)
		return
	}

	existing.IsActive = targetActive
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(existing)
}

// HandleAdminImportPreview inspects an uploaded workbook or file path and returns validation results.
func (h *Handler) HandleAdminImportPreview(w http.ResponseWriter, r *http.Request) {
	reader, filePath, _, cleanup, err := parseImportPayload(r)
	if err != nil {
		httperr.BadRequest(w, r, err.Error())
		return
	}
	defer cleanup()

	preview, err := h.service.PreviewImport(r.Context(), reader, filePath)
	if err != nil {
		httperr.BadRequest(w, r, fmt.Sprintf("Import preview failed: %v", err))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(preview)
}

// HandleAdminImportCommit executes the transactional import after admin confirms.
func (h *Handler) HandleAdminImportCommit(w http.ResponseWriter, r *http.Request) {
	user, _ := identity.UserFromContext(r.Context())
	var adminID *uuid.UUID
	if user != nil {
		adminID = &user.ID
	}

	reader, filePath, filename, cleanup, err := parseImportPayload(r)
	if err != nil {
		httperr.BadRequest(w, r, err.Error())
		return
	}
	defer cleanup()

	result, err := h.service.CommitImport(r.Context(), reader, filePath, filename, adminID, h.appEnv)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnprocessableEntity)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"error":   err.Error(),
			"results": result,
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(result)
}

// HandleAdminListImportHistory lists past question import runs.
func (h *Handler) HandleAdminListImportHistory(w http.ResponseWriter, r *http.Request) {
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

	history, err := h.service.ListImportHistory(r.Context(), limit, offset)
	if err != nil {
		httperr.InternalServerError(w, r, err, h.logger)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"history": history,
	})
}

// parseImportPayload handles both multipart file uploads from browser and JSON file_path payloads.
func parseImportPayload(r *http.Request) (io.Reader, string, string, func(), error) {
	contentType := r.Header.Get("Content-Type")

	if strings.Contains(contentType, "multipart/form-data") {
		// Max 32MB upload
		if err := r.ParseMultipartForm(32 << 20); err != nil {
			return nil, "", "", func() {}, fmt.Errorf("failed to parse multipart upload: %w", err)
		}
		file, header, err := r.FormFile("file")
		if err != nil {
			return nil, "", "", func() {}, errors.New("multipart upload missing 'file' field")
		}
		cleanup := func() { _ = file.Close() }
		return file, "", header.Filename, cleanup, nil
	}

	// JSON payload with file_path
	var body struct {
		FilePath string `json:"file_path"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		return nil, "", "", func() {}, errors.New("invalid request: expected multipart form with 'file' or JSON with 'file_path'")
	}

	body.FilePath = strings.TrimSpace(body.FilePath)
	if body.FilePath == "" {
		return nil, "", "", func() {}, errors.New("'file_path' cannot be empty")
	}

	filename := body.FilePath
	if idx := strings.LastIndexAny(filename, `/\`); idx != -1 {
		filename = filename[idx+1:]
	}

	return nil, body.FilePath, filename, func() {}, nil
}
