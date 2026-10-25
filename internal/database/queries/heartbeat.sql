-- name: RecordHeartbeat :one
INSERT INTO infrastructure_heartbeats (component)
VALUES ($1)
RETURNING id, component, recorded_at;

-- name: GetLatestHeartbeat :one
SELECT id, component, recorded_at
FROM infrastructure_heartbeats
ORDER BY recorded_at DESC
LIMIT 1;
