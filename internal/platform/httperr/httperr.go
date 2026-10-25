package httperr

import (
	"encoding/json"
	"log/slog"
	"net/http"
)

// ProblemDetail conforms to RFC 9457 Problem Details for HTTP APIs.
type ProblemDetail struct {
	Type          string            `json:"type"`
	Title         string            `json:"title"`
	Status        int               `json:"status"`
	Detail        string            `json:"detail"`
	Instance      string            `json:"instance,omitempty"`
	Code          string            `json:"code,omitempty"`
	InvalidParams map[string]string `json:"invalid_params,omitempty"`
}

// WriteJSON sends a standardized RFC 9457 JSON problem details response.
func WriteJSON(w http.ResponseWriter, r *http.Request, status int, title, detail, code string) {
	instance := ""
	if r != nil {
		instance = r.URL.Path
	}

	problem := ProblemDetail{
		Type:     "about:blank",
		Title:    title,
		Status:   status,
		Detail:   detail,
		Instance: instance,
		Code:     code,
	}

	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(problem)
}

// BadRequest writes a 400 Bad Request problem response.
func BadRequest(w http.ResponseWriter, r *http.Request, detail string) {
	WriteJSON(w, r, http.StatusBadRequest, "Bad Request", detail, "BAD_REQUEST")
}

// NotFound writes a 404 Not Found problem response.
func NotFound(w http.ResponseWriter, r *http.Request, detail string) {
	WriteJSON(w, r, http.StatusNotFound, "Not Found", detail, "RESOURCE_NOT_FOUND")
}

// InternalServerError writes a safe 500 response, logging the raw error server-side while
// preventing internal stack traces or connection strings from leaking to the client.
func InternalServerError(w http.ResponseWriter, r *http.Request, err error, log *slog.Logger) {
	if log != nil && err != nil {
		ctx := r.Context()
		log.ErrorContext(ctx, "internal server error encountered",
			"error", err.Error(),
			"path", r.URL.Path,
			"method", r.Method,
		)
	}

	WriteJSON(w, r, http.StatusInternalServerError, "Internal Server Error",
		"An unexpected internal error occurred. Please try again later.",
		"INTERNAL_SERVER_ERROR",
	)
}

// ServiceUnavailable writes a 503 Service Unavailable problem response.
func ServiceUnavailable(w http.ResponseWriter, r *http.Request, detail string) {
	WriteJSON(w, r, http.StatusServiceUnavailable, "Service Unavailable", detail, "SERVICE_UNAVAILABLE")
}
