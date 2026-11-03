-- name: GetUserPatternMetrics :many
SELECT p.id as pattern_id,
       p.name as pattern_name,
       p.slug as pattern_slug,
       COUNT(a.id)::BIGINT as total_attempts,
       COUNT(CASE WHEN a.outcome = 'UNRESOLVED_FAILED' THEN 1 END)::BIGINT as failed_attempts,
       COUNT(CASE WHEN a.outcome = 'SOLVED_INDEPENDENTLY' THEN 1 END)::BIGINT as independent_solves,
       COUNT(CASE WHEN a.outcome = 'SOLVED_WITH_HINTS' THEN 1 END)::BIGINT as hint_solves,
       COALESCE(AVG(a.quality_rating), 0)::FLOAT as avg_quality_rating
FROM question_attempts a
JOIN question_patterns qp ON a.question_id = qp.question_id
JOIN patterns p ON qp.pattern_id = p.id
WHERE a.user_id = $1
GROUP BY p.id, p.name, p.slug
ORDER BY failed_attempts DESC, avg_quality_rating ASC;

-- name: GetUnsolvedQuestionsForPattern :many
SELECT q.id, q.title, q.slug, q.difficulty, q.is_core,
       p.name as pattern_name, p.slug as pattern_slug
FROM questions q
JOIN question_patterns qp ON q.id = qp.question_id
JOIN patterns p ON qp.pattern_id = p.id
LEFT JOIN user_question_progress uqp ON q.id = uqp.question_id AND uqp.user_id = $1
WHERE p.id = $2
  AND (uqp.status IS NULL OR uqp.status != 'SOLVED' AND uqp.status != 'MASTERED')
ORDER BY q.is_core DESC, q.created_at ASC
LIMIT $3;
