package database

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/adaptive-dsa/platform/internal/config"
	"github.com/jackc/pgx/v5/pgxpool"
)

// HealthStatus represents the connectivity and metric state of the database pool.
type HealthStatus struct {
	Available     bool          `json:"available"`
	Latency       time.Duration `json:"latency_ms"`
	TotalConns    int32         `json:"total_conns"`
	IdleConns     int32         `json:"idle_conns"`
	AcquiredConns int32         `json:"acquired_conns"`
	MaxConns      int32         `json:"max_conns"`
	Message       string        `json:"message,omitempty"`
}

// Checker defines the interface required for readiness verification.
type Checker interface {
	Ping(ctx context.Context) error
	Health(ctx context.Context) HealthStatus
	Close()
}

// DB wraps pgxpool.Pool and implements Checker.
type DB struct {
	pool   *pgxpool.Pool
	logger *slog.Logger
}

// New establishes a validated pgxpool.Pool connection based on DatabaseConfig.
func New(ctx context.Context, cfg config.DatabaseConfig, log *slog.Logger) (*DB, error) {
	if log == nil {
		log = slog.Default()
	}

	log.InfoContext(ctx, "initializing postgresql connection pool",
		"url", cfg.RedactedDatabaseURL(),
		"max_conns", cfg.MaxConns,
		"min_conns", cfg.MinConns,
		"max_lifetime", cfg.MaxConnLifetime,
		"max_idle_time", cfg.MaxConnIdleTime,
	)

	poolConfig, err := pgxpool.ParseConfig(cfg.URL)
	if err != nil {
		return nil, fmt.Errorf("database: parse config failed: %w", err)
	}

	poolConfig.MaxConns = cfg.MaxConns
	poolConfig.MinConns = cfg.MinConns
	poolConfig.MaxConnLifetime = cfg.MaxConnLifetime
	poolConfig.MaxConnIdleTime = cfg.MaxConnIdleTime
	if cfg.HealthCheckPeriod > 0 {
		poolConfig.HealthCheckPeriod = cfg.HealthCheckPeriod
	} else {
		poolConfig.HealthCheckPeriod = 1 * time.Minute
	}

	// Bounded connect timeout
	connectCtx, cancel := context.WithTimeout(ctx, cfg.ConnectTimeout)
	defer cancel()

	pool, err := pgxpool.NewWithConfig(connectCtx, poolConfig)
	if err != nil {
		return nil, fmt.Errorf("database: create pool failed: %w", err)
	}

	// Verify connectivity
	pingCtx, pingCancel := context.WithTimeout(ctx, cfg.ConnectTimeout)
	defer pingCancel()

	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("database: initial ping failed (%s): %w", cfg.RedactedDatabaseURL(), err)
	}

	log.InfoContext(ctx, "postgresql connection pool established successfully")
	return &DB{pool: pool, logger: log}, nil
}

// Pool returns the underlying pgxpool.Pool instance for query execution.
func (d *DB) Pool() *pgxpool.Pool {
	return d.pool
}

// Ping checks if the database is responding to queries.
func (d *DB) Ping(ctx context.Context) error {
	if d.pool == nil {
		return fmt.Errorf("database: pool is not initialized")
	}
	return d.pool.Ping(ctx)
}

// Health inspects pool metrics and verifies query execution latency.
func (d *DB) Health(ctx context.Context) HealthStatus {
	if d.pool == nil {
		return HealthStatus{
			Available: false,
			Message:   "database pool is nil",
		}
	}

	start := time.Now()
	err := d.pool.Ping(ctx)
	latency := time.Since(start)

	stat := d.pool.Stat()
	status := HealthStatus{
		Available:     err == nil,
		Latency:       latency,
		TotalConns:    stat.TotalConns(),
		IdleConns:     stat.IdleConns(),
		AcquiredConns: stat.AcquiredConns(),
		MaxConns:      stat.MaxConns(),
	}

	if err != nil {
		status.Message = err.Error()
	} else {
		status.Message = "healthy"
	}

	return status
}

// Close gracefully closes all idle and active connections in the pool.
func (d *DB) Close() {
	if d.pool != nil {
		d.logger.Info("closing postgresql connection pool")
		d.pool.Close()
	}
}
