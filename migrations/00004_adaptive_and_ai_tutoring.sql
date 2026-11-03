-- +goose Up
-- SQL in this section is executed after the migration is applied.

-- 1. Create Enums for Credit Ledger and AI Usage
CREATE TYPE ledger_entry_type AS ENUM (
    'PURCHASE',           -- User bought credit pack
    'AI_USAGE',           -- Deducted for progressive hint, review, etc.
    'PROMOTIONAL_GRANT',  -- Free welcome credits or reward
    'REFUND',             -- Compensatory credit return
    'ADMIN_ADJUSTMENT'    -- Manual admin debit/credit with audit reason
);

CREATE TYPE ai_feature_type AS ENUM (
    'PROGRESSIVE_HINT',
    'CONCEPT_EXPLANATION',
    'APPROACH_REVIEW',
    'MISTAKE_CLASSIFICATION',
    'REVISION_SUMMARY'
);

-- 2. Credit Accounts (Materialized balance cache for fast O(1) checks and row locking)
CREATE TABLE credit_accounts (
    user_id UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    cached_balance INT NOT NULL DEFAULT 0,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT chk_positive_balance CHECK (cached_balance >= 0)
);

-- 3. Immutable Append-Only Credit Ledger
CREATE TABLE credit_ledger_entries (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    amount INT NOT NULL, -- Positive for credits added, negative for credits deducted
    entry_type ledger_entry_type NOT NULL,
    reference_type VARCHAR(50), -- e.g., 'PAYMENT_TRANSACTION', 'AI_REQUEST', 'ADMIN_GRANT', 'WELCOME_BONUS'
    reference_id UUID,          -- Links to ai_requests.id, payment_transactions.id, etc.
    description TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_credit_ledger_user ON credit_ledger_entries(user_id, created_at DESC);

-- 4. Metered AI Requests
CREATE TABLE ai_requests (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    feature ai_feature_type NOT NULL,
    model VARCHAR(50) NOT NULL,            -- e.g., 'gemini-1.5-flash'
    prompt_tokens INT NOT NULL DEFAULT 0,
    completion_tokens INT NOT NULL DEFAULT 0,
    estimated_cost_usd_micros INT NOT NULL DEFAULT 0, -- USD millionths (integer)
    credits_charged INT NOT NULL DEFAULT 0,
    status VARCHAR(20) NOT NULL DEFAULT 'SUCCESS',    -- 'SUCCESS', 'TIMEOUT', 'FAILED'
    latency_ms INT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_ai_requests_user ON ai_requests(user_id, created_at DESC);

-- +goose Down
-- SQL in this section is executed when the migration is rolled back.

DROP TABLE IF EXISTS ai_requests CASCADE;
DROP TABLE IF EXISTS credit_ledger_entries CASCADE;
DROP TABLE IF EXISTS credit_accounts CASCADE;
DROP TYPE IF EXISTS ai_feature_type CASCADE;
DROP TYPE IF EXISTS ledger_entry_type CASCADE;
