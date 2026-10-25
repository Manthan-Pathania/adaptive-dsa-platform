-- name: CreateUser :one
INSERT INTO users (
    email,
    password_hash,
    full_name,
    role
) VALUES (
    LOWER(TRIM(@email::text)),
    @password_hash,
    @full_name,
    @role
) RETURNING id, email, full_name, role, is_active, created_at, updated_at;

-- name: GetUserByEmail :one
SELECT id, email, password_hash, full_name, role, is_active, created_at, updated_at
FROM users
WHERE email = LOWER(TRIM(@email::text));

-- name: GetUserByID :one
SELECT id, email, password_hash, full_name, role, is_active, created_at, updated_at
FROM users
WHERE id = $1;

-- name: UpdateUserRole :one
UPDATE users
SET role = $2, updated_at = CURRENT_TIMESTAMP
WHERE id = $1
RETURNING id, email, full_name, role, is_active, created_at, updated_at;

-- name: CreateSession :one
INSERT INTO sessions (
    user_id,
    session_token_hash,
    ip_address,
    user_agent,
    expires_at
) VALUES (
    $1,
    $2,
    $3,
    $4,
    $5
) RETURNING id, user_id, session_token_hash, ip_address, user_agent, expires_at, created_at, last_active_at;

-- name: GetSessionWithUser :one
SELECT 
    s.id AS session_id,
    s.session_token_hash,
    s.ip_address,
    s.user_agent,
    s.expires_at,
    s.created_at AS session_created_at,
    s.last_active_at,
    u.id AS user_id,
    u.email,
    u.full_name,
    u.role,
    u.is_active,
    u.created_at AS user_created_at,
    u.updated_at AS user_updated_at
FROM sessions s
JOIN users u ON s.user_id = u.id
WHERE s.session_token_hash = $1
  AND s.expires_at > CURRENT_TIMESTAMP
  AND u.is_active = TRUE;

-- name: TouchSession :exec
UPDATE sessions
SET last_active_at = CURRENT_TIMESTAMP
WHERE id = $1;

-- name: DeleteSessionByTokenHash :exec
DELETE FROM sessions
WHERE session_token_hash = $1;

-- name: DeleteUserSessions :exec
DELETE FROM sessions
WHERE user_id = $1;

-- name: DeleteExpiredSessions :exec
DELETE FROM sessions
WHERE expires_at <= CURRENT_TIMESTAMP;

-- name: CreateAuditLog :one
INSERT INTO audit_logs (
    actor_user_id,
    action,
    target_type,
    target_id,
    ip_address,
    user_agent,
    metadata
) VALUES (
    $1,
    $2,
    $3,
    $4,
    $5,
    $6,
    $7
) RETURNING id, actor_user_id, action, target_type, target_id, ip_address, user_agent, metadata, created_at;

-- name: ListRecentAuditLogs :many
SELECT id, actor_user_id, action, target_type, target_id, ip_address, user_agent, metadata, created_at
FROM audit_logs
ORDER BY created_at DESC
LIMIT $1;
