-- +goose Up
-- SQL in this section is executed when the migration is applied.

-- PostgreSQL 16 includes gen_random_uuid() natively in core (no extensions required).
-- UUIDv7 primary keys for domain models are generated application-side in Go via github.com/google/uuid (v1.6+).

-- Initial infrastructure heartbeats table to verify schema migration & write connectivity
CREATE TABLE IF NOT EXISTS infrastructure_heartbeats (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    component VARCHAR(50) NOT NULL,
    recorded_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- +goose Down
-- SQL in this section is executed when the migration is rolled back.
DROP TABLE IF EXISTS infrastructure_heartbeats;
