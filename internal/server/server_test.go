package server

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/adaptive-dsa/platform/internal/config"
	"github.com/adaptive-dsa/platform/internal/platform/database"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeDBChecker implements database.Checker for unit testing.
type fakeDBChecker struct {
	available bool
	message   string
	err       error
}

func (f *fakeDBChecker) Ping(ctx context.Context) error {
	return f.err
}

func (f *fakeDBChecker) Health(ctx context.Context) database.HealthStatus {
	return database.HealthStatus{
		Available:     f.available,
		Message:       f.message,
		Latency:       1500 * time.Microsecond,
		TotalConns:    5,
		IdleConns:     4,
		AcquiredConns: 1,
		MaxConns:      25,
	}
}

func (f *fakeDBChecker) Close() {}

func setupTestServer(db database.Checker) *Server {
	cfg := &config.Config{
		AppEnv: config.EnvTest,
		Server: config.ServerConfig{
			Host:            "127.0.0.1",
			Port:            8080,
			ReadTimeout:     5 * time.Second,
			WriteTimeout:    5 * time.Second,
			IdleTimeout:     30 * time.Second,
			ShutdownTimeout: 2 * time.Second,
		},
		Database: config.DatabaseConfig{
			URL: "postgres://user:pass@localhost:5432/db",
		},
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	return NewServer(cfg, logger, db, nil, nil, nil)
}

func TestServer_Healthz(t *testing.T) {
	srv := setupTestServer(nil)
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	w := httptest.NewRecorder()

	srv.Handler().ServeHTTP(w, req)

	resp := w.Result()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "application/json", resp.Header.Get("Content-Type"))

	var body LivenessResponse
	err := json.NewDecoder(resp.Body).Decode(&body)
	require.NoError(t, err)

	assert.Equal(t, "healthy", body.Status)
	assert.GreaterOrEqual(t, body.UptimeSeconds, int64(0))
	assert.NotEmpty(t, body.Timestamp)
}

func TestServer_Readyz_WhenDatabaseHealthy(t *testing.T) {
	fakeDB := &fakeDBChecker{available: true, message: "healthy"}
	srv := setupTestServer(fakeDB)

	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	w := httptest.NewRecorder()

	srv.Handler().ServeHTTP(w, req)

	resp := w.Result()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "application/json", resp.Header.Get("Content-Type"))

	var body ReadinessResponse
	err := json.NewDecoder(resp.Body).Decode(&body)
	require.NoError(t, err)

	assert.Equal(t, "ready", body.Status)
	dbCheck, ok := body.Checks["database"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, true, dbCheck["available"])
	assert.Equal(t, "healthy", dbCheck["message"])
}

func TestServer_Readyz_WhenDatabaseUnhealthy(t *testing.T) {
	fakeDB := &fakeDBChecker{
		available: false,
		message:   "connection refused",
		err:       errors.New("dial tcp 127.0.0.1:5432: connect: connection refused"),
	}
	srv := setupTestServer(fakeDB)

	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	w := httptest.NewRecorder()

	srv.Handler().ServeHTTP(w, req)

	resp := w.Result()
	assert.Equal(t, http.StatusServiceUnavailable, resp.StatusCode)
	assert.Equal(t, "application/json", resp.Header.Get("Content-Type"))

	var body ReadinessResponse
	err := json.NewDecoder(resp.Body).Decode(&body)
	require.NoError(t, err)

	assert.Equal(t, "unavailable", body.Status)
	dbCheck, ok := body.Checks["database"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, false, dbCheck["available"])
	assert.Equal(t, "connection refused", dbCheck["message"])
}

func TestServer_Readyz_WhenDatabaseNil(t *testing.T) {
	srv := setupTestServer(nil)

	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	w := httptest.NewRecorder()

	srv.Handler().ServeHTTP(w, req)

	resp := w.Result()
	assert.Equal(t, http.StatusServiceUnavailable, resp.StatusCode)

	var body ReadinessResponse
	err := json.NewDecoder(resp.Body).Decode(&body)
	require.NoError(t, err)

	assert.Equal(t, "unavailable", body.Status)
}

func TestServer_ApiPing(t *testing.T) {
	srv := setupTestServer(nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/ping", nil)
	w := httptest.NewRecorder()

	srv.Handler().ServeHTTP(w, req)

	resp := w.Result()
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var body map[string]string
	err := json.NewDecoder(resp.Body).Decode(&body)
	require.NoError(t, err)
	assert.Equal(t, "pong", body["message"])
}
