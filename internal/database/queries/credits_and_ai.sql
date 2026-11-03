-- name: GetCreditAccount :one
SELECT user_id, cached_balance, updated_at
FROM credit_accounts
WHERE user_id = $1;

-- name: GetCreditAccountForUpdate :one
SELECT user_id, cached_balance, updated_at
FROM credit_accounts
WHERE user_id = $1
FOR UPDATE;

-- name: UpsertCreditAccount :one
INSERT INTO credit_accounts (
    user_id,
    cached_balance,
    updated_at
) VALUES (
    $1, $2, CURRENT_TIMESTAMP
) ON CONFLICT (user_id) DO UPDATE
SET cached_balance = EXCLUDED.cached_balance,
    updated_at = CURRENT_TIMESTAMP
RETURNING user_id, cached_balance, updated_at;

-- name: UpdateCreditBalance :one
UPDATE credit_accounts
SET cached_balance = cached_balance + $2,
    updated_at = CURRENT_TIMESTAMP
WHERE user_id = $1
RETURNING user_id, cached_balance, updated_at;

-- name: CreateCreditLedgerEntry :one
INSERT INTO credit_ledger_entries (
    user_id,
    amount,
    entry_type,
    reference_type,
    reference_id,
    description
) VALUES (
    $1, $2, $3, $4, $5, $6
) RETURNING id, user_id, amount, entry_type, reference_type, reference_id, description, created_at;

-- name: ListCreditLedgerEntries :many
SELECT id, user_id, amount, entry_type, reference_type, reference_id, description, created_at
FROM credit_ledger_entries
WHERE user_id = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: CountCreditLedgerEntries :one
SELECT COUNT(*)
FROM credit_ledger_entries
WHERE user_id = $1;

-- name: SumCreditLedgerEntries :one
SELECT COALESCE(SUM(amount), 0)::BIGINT AS total
FROM credit_ledger_entries
WHERE user_id = $1;

-- name: CreateAIRequest :one
INSERT INTO ai_requests (
    user_id,
    feature,
    model,
    prompt_tokens,
    completion_tokens,
    estimated_cost_usd_micros,
    credits_charged,
    status,
    latency_ms
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9
) RETURNING id, user_id, feature, model, prompt_tokens, completion_tokens, estimated_cost_usd_micros, credits_charged, status, latency_ms, created_at;

-- name: ListAIRequestsByUser :many
SELECT id, user_id, feature, model, prompt_tokens, completion_tokens, estimated_cost_usd_micros, credits_charged, status, latency_ms, created_at
FROM ai_requests
WHERE user_id = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: CountAIRequestsByUser :one
SELECT COUNT(*)
FROM ai_requests
WHERE user_id = $1;
