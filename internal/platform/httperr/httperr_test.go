package httperr

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBadRequest(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/test", nil)
	w := httptest.NewRecorder()

	BadRequest(w, req, "invalid pagination parameter: limit must be positive")

	resp := w.Result()
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	assert.Equal(t, "application/problem+json", resp.Header.Get("Content-Type"))

	var problem ProblemDetail
	err := json.NewDecoder(resp.Body).Decode(&problem)
	require.NoError(t, err)

	assert.Equal(t, 400, problem.Status)
	assert.Equal(t, "Bad Request", problem.Title)
	assert.Equal(t, "invalid pagination parameter: limit must be positive", problem.Detail)
	assert.Equal(t, "/api/v1/test", problem.Instance)
	assert.Equal(t, "BAD_REQUEST", problem.Code)
}

func TestNotFound(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/questions/q_123", nil)
	w := httptest.NewRecorder()

	NotFound(w, req, "question with id 'q_123' does not exist")

	resp := w.Result()
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)

	var problem ProblemDetail
	err := json.NewDecoder(resp.Body).Decode(&problem)
	require.NoError(t, err)

	assert.Equal(t, 404, problem.Status)
	assert.Equal(t, "Not Found", problem.Title)
	assert.Equal(t, "RESOURCE_NOT_FOUND", problem.Code)
}

func TestInternalServerError_MasksSensitiveDetails(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/v1/orders", nil)
	w := httptest.NewRecorder()

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	secretErr := errors.New("pq: connection to postgres://admin:SuperSecretPass@db:5432 failed")

	InternalServerError(w, req, secretErr, logger)

	resp := w.Result()
	assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)

	var problem ProblemDetail
	err := json.NewDecoder(resp.Body).Decode(&problem)
	require.NoError(t, err)

	assert.Equal(t, 500, problem.Status)
	assert.Equal(t, "Internal Server Error", problem.Title)
	assert.NotContains(t, problem.Detail, "SuperSecretPass")
	assert.Equal(t, "An unexpected internal error occurred. Please try again later.", problem.Detail)
	assert.Equal(t, "INTERNAL_SERVER_ERROR", problem.Code)
}
