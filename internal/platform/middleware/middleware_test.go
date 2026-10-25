package middleware

import (
	"bytes"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/adaptive-dsa/platform/internal/platform/logger"
	"github.com/stretchr/testify/assert"
)

func TestRequestID_GeneratesWhenMissing(t *testing.T) {
	var capturedID string
	handler := RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedID = logger.RequestIDFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	resp := w.Result()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.NotEmpty(t, capturedID)
	assert.Equal(t, capturedID, resp.Header.Get(HeaderXRequestID))
}

func TestRequestID_PreservesExisting(t *testing.T) {
	var capturedID string
	handler := RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedID = logger.RequestIDFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set(HeaderXRequestID, "client-trace-12345")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	resp := w.Result()
	assert.Equal(t, "client-trace-12345", capturedID)
	assert.Equal(t, "client-trace-12345", resp.Header.Get(HeaderXRequestID))
}

func TestRecoverer_HandlesPanicGracefully(t *testing.T) {
	buf := new(bytes.Buffer)
	log := slog.New(slog.NewTextHandler(buf, nil))

	handler := Recoverer(log)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("unexpected nil pointer dereference")
	}))

	req := httptest.NewRequest(http.MethodGet, "/boom", nil)
	w := httptest.NewRecorder()

	assert.NotPanics(t, func() {
		handler.ServeHTTP(w, req)
	})

	resp := w.Result()
	assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
	assert.Equal(t, "application/problem+json", resp.Header.Get("Content-Type"))
	assert.Contains(t, buf.String(), "panic recovered in http handler")
	assert.Contains(t, buf.String(), "unexpected nil pointer dereference")
}

func TestRequestLogger_LogsCompletion(t *testing.T) {
	buf := new(bytes.Buffer)
	log := slog.New(slog.NewTextHandler(buf, nil))

	handler := RequestLogger(log)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		_, _ = io.WriteString(w, "created")
	}))

	req := httptest.NewRequest(http.MethodPost, "/items", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	resp := w.Result()
	assert.Equal(t, http.StatusCreated, resp.StatusCode)
	assert.Contains(t, buf.String(), "http request completed")
	assert.Contains(t, buf.String(), "status=201")
	assert.Contains(t, buf.String(), "method=POST")
}
