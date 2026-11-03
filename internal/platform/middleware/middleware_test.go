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

func TestSecurityHeaders_SetsDefensiveHeaders(t *testing.T) {
	// 1. Development mode
	handlerDev := SecurityHeaders(false)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()
	handlerDev.ServeHTTP(w, req)
	resp := w.Result()

	assert.Equal(t, "nosniff", resp.Header.Get("X-Content-Type-Options"))
	assert.Equal(t, "DENY", resp.Header.Get("X-Frame-Options"))
	assert.Equal(t, "strict-origin-when-cross-origin", resp.Header.Get("Referrer-Policy"))
	assert.Empty(t, resp.Header.Get("Strict-Transport-Security"), "HSTS should not be set in dev")

	// 2. Production mode
	handlerProd := SecurityHeaders(true)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	wProd := httptest.NewRecorder()
	handlerProd.ServeHTTP(wProd, req)
	respProd := wProd.Result()

	assert.NotEmpty(t, respProd.Header.Get("Strict-Transport-Security"), "HSTS must be set in production")
}

func TestBodyLimit_EnforcesMaxBytes(t *testing.T) {
	handler := BodyLimit(10)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusRequestEntityTooLarge)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))

	// Payload with 15 bytes exceeds limit of 10
	largeBody := bytes.NewReader([]byte("123456789012345"))
	req := httptest.NewRequest(http.MethodPost, "/upload", largeBody)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusRequestEntityTooLarge, w.Result().StatusCode)
}

func TestCORS_HandlesPreflightAndAllowedOrigins(t *testing.T) {
	allowed := []string{"http://localhost:3000", "https://app.example.com"}
	handler := CORS(allowed)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// 1. Allowed origin preflight OPTIONS
	reqPreflight := httptest.NewRequest(http.MethodOptions, "/api/v1/auth/me", nil)
	reqPreflight.Header.Set("Origin", "http://localhost:3000")
	wPreflight := httptest.NewRecorder()
	handler.ServeHTTP(wPreflight, reqPreflight)

	resPreflight := wPreflight.Result()
	assert.Equal(t, http.StatusNoContent, resPreflight.StatusCode)
	assert.Equal(t, "http://localhost:3000", resPreflight.Header.Get("Access-Control-Allow-Origin"))
	assert.Equal(t, "true", resPreflight.Header.Get("Access-Control-Allow-Credentials"))

	// 2. Disallowed origin
	reqBad := httptest.NewRequest(http.MethodGet, "/api/v1/questions", nil)
	reqBad.Header.Set("Origin", "https://evil-site.com")
	wBad := httptest.NewRecorder()
	handler.ServeHTTP(wBad, reqBad)

	resBad := wBad.Result()
	assert.Empty(t, resBad.Header.Get("Access-Control-Allow-Origin"))
}
