-- name: CreateQuestionAttempt :one
INSERT INTO question_attempts (
    user_id,
    question_id,
    roadmap_id,
    outcome,
    time_spent_secs,
    difficulty_experienced,
    quality_rating,
    mistake_tags,
    approach_summary,
    code_snippet
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10
) RETURNING id, user_id, question_id, roadmap_id, outcome, time_spent_secs, difficulty_experienced, quality_rating, mistake_tags, approach_summary, code_snippet, created_at;

-- name: ListQuestionAttempts :many
SELECT id, user_id, question_id, roadmap_id, outcome, time_spent_secs, difficulty_experienced, quality_rating, mistake_tags, approach_summary, code_snippet, created_at
FROM question_attempts
WHERE user_id = $1 AND question_id = $2
ORDER BY created_at DESC;

-- name: ListUserRecentAttempts :many
SELECT a.id, a.user_id, a.question_id, a.roadmap_id, a.outcome, a.time_spent_secs, a.difficulty_experienced, a.quality_rating, a.mistake_tags, a.approach_summary, a.code_snippet, a.created_at,
       q.title as question_title, q.slug as question_slug, q.difficulty as question_difficulty
FROM question_attempts a
JOIN questions q ON a.question_id = q.id
WHERE a.user_id = $1
ORDER BY a.created_at DESC
LIMIT $2;

-- name: GetUserQuestionProgress :one
SELECT id, user_id, question_id, status, first_solved_at, last_solved_at, solve_count, independent_solve_count, total_time_spent_secs, is_bookmarked, personal_notes, created_at, updated_at
FROM user_question_progress
WHERE user_id = $1 AND question_id = $2;

-- name: UpsertUserQuestionProgress :one
INSERT INTO user_question_progress (
    user_id,
    question_id,
    status,
    first_solved_at,
    last_solved_at,
    solve_count,
    independent_solve_count,
    total_time_spent_secs,
    personal_notes
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9
) ON CONFLICT (user_id, question_id) DO UPDATE
SET status = EXCLUDED.status,
    first_solved_at = COALESCE(user_question_progress.first_solved_at, EXCLUDED.first_solved_at),
    last_solved_at = EXCLUDED.last_solved_at,
    solve_count = user_question_progress.solve_count + EXCLUDED.solve_count,
    independent_solve_count = user_question_progress.independent_solve_count + EXCLUDED.independent_solve_count,
    total_time_spent_secs = user_question_progress.total_time_spent_secs + EXCLUDED.total_time_spent_secs,
    personal_notes = COALESCE(EXCLUDED.personal_notes, user_question_progress.personal_notes),
    updated_at = CURRENT_TIMESTAMP
RETURNING id, user_id, question_id, status, first_solved_at, last_solved_at, solve_count, independent_solve_count, total_time_spent_secs, is_bookmarked, personal_notes, created_at, updated_at;

-- name: GetUserProgressStats :many
SELECT status, COUNT(*) as count
FROM user_question_progress
WHERE user_id = $1
GROUP BY status;

-- name: ListRoadmapProgress :many
SELECT ri.id as roadmap_item_id, ri.roadmap_id, ri.question_id, ri.stage_number, ri.order_index, ri.is_mandatory,
       q.title, q.slug as question_slug, q.difficulty, q.is_core,
       COALESCE(p.status, 'NOT_STARTED'::progress_status) as progress_status,
       COALESCE(p.solve_count, 0) as solve_count,
       p.last_solved_at
FROM roadmap_items ri
JOIN questions q ON ri.question_id = q.id
LEFT JOIN user_question_progress p ON p.question_id = q.id AND p.user_id = $2
WHERE ri.roadmap_id = $1
ORDER BY ri.stage_number ASC, ri.order_index ASC;

-- name: GetRevisionSchedule :one
SELECT id, user_id, question_id, stability, difficulty, repetition_count, ease_factor, interval_days, next_review_at, last_reviewed_at, created_at, updated_at
FROM revision_schedules
WHERE user_id = $1 AND question_id = $2;

-- name: UpsertRevisionSchedule :one
INSERT INTO revision_schedules (
    user_id,
    question_id,
    stability,
    difficulty,
    repetition_count,
    ease_factor,
    interval_days,
    next_review_at,
    last_reviewed_at
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9
) ON CONFLICT (user_id, question_id) DO UPDATE
SET stability = EXCLUDED.stability,
    difficulty = EXCLUDED.difficulty,
    repetition_count = EXCLUDED.repetition_count,
    ease_factor = EXCLUDED.ease_factor,
    interval_days = EXCLUDED.interval_days,
    next_review_at = EXCLUDED.next_review_at,
    last_reviewed_at = EXCLUDED.last_reviewed_at,
    updated_at = CURRENT_TIMESTAMP
RETURNING id, user_id, question_id, stability, difficulty, repetition_count, ease_factor, interval_days, next_review_at, last_reviewed_at, created_at, updated_at;

-- name: ListDueRevisions :many
SELECT rs.id, rs.user_id, rs.question_id, rs.stability, rs.difficulty, rs.repetition_count, rs.ease_factor, rs.interval_days, rs.next_review_at, rs.last_reviewed_at,
       q.title as question_title, q.slug as question_slug, q.difficulty as question_difficulty
FROM revision_schedules rs
JOIN questions q ON rs.question_id = q.id
WHERE rs.user_id = $1 AND rs.next_review_at <= $2
ORDER BY rs.next_review_at ASC
LIMIT $3 OFFSET $4;

-- name: CountDueRevisions :one
SELECT COUNT(*)
FROM revision_schedules rs
WHERE rs.user_id = $1 AND rs.next_review_at <= $2;
