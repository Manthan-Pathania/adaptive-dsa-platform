package database

import (
	"context"
	"io"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/adaptive-dsa/platform/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDB_NilPoolHealth(t *testing.T) {
	db := &DB{pool: nil, logger: slog.Default()}
	status := db.Health(context.Background())

	assert.False(t, status.Available)
	assert.Equal(t, "database pool is nil", status.Message)
	assert.Error(t, db.Ping(context.Background()))
}

func TestDB_InvalidURL(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	cfg := config.DatabaseConfig{
		URL:            "postgres://invalid:url:with:too:many:colons",
		ConnectTimeout: 100 * time.Millisecond,
	}

	db, err := New(context.Background(), cfg, logger)
	assert.Error(t, err)
	assert.Nil(t, db)
}

func TestDB_ConnectionFailure_UnreachableHost(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	cfg := config.DatabaseConfig{
		// 192.0.2.1 is reserved for documentation (TEST-NET-1), safe non-routable blackhole
		URL:            "postgres://dsa_user:dsa_pass@192.0.2.1:5432/dsa_db?sslmode=disable",
		ConnectTimeout: 200 * time.Millisecond,
		MaxConns:       5,
		MinConns:       1,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	start := time.Now()
	db, err := New(ctx, cfg, logger)
	duration := time.Since(start)

	assert.Error(t, err)
	assert.Nil(t, db)
	// Must fail fast within timeout, not hang indefinitely
	assert.Less(t, duration, 1*time.Second)
}

// Integration test that runs against a real PostgreSQL instance if available.
func TestDB_RealPostgres_Integration(t *testing.T) {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		t.Skip("skipping database integration test: DATABASE_URL not set")
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	cfg := config.DatabaseConfig{
		URL:               dbURL,
		MaxConns:          5,
		MinConns:          2,
		MaxConnLifetime:   5 * time.Minute,
		MaxConnIdleTime:   1 * time.Minute,
		HealthCheckPeriod: 30 * time.Second,
		ConnectTimeout:    3 * time.Second,
	}

	ctx := context.Background()
	db, err := New(ctx, cfg, logger)
	require.NoError(t, err)
	defer db.Close()

	err = db.Ping(ctx)
	assert.NoError(t, err)

	health := db.Health(ctx)
	assert.True(t, health.Available)
	assert.Equal(t, "healthy", health.Message)
	assert.GreaterOrEqual(t, health.TotalConns, int32(1))
}
