-- name: CreateTopic :one
INSERT INTO topics (
    name,
    slug,
    description,
    display_order
) VALUES (
    $1, $2, $3, $4
) ON CONFLICT (name) DO UPDATE
SET description = EXCLUDED.description,
    display_order = EXCLUDED.display_order
RETURNING id, name, slug, description, display_order, created_at;

-- name: GetTopicBySlug :one
SELECT id, name, slug, description, display_order, created_at
FROM topics
WHERE slug = $1;

-- name: GetTopicByName :one
SELECT id, name, slug, description, display_order, created_at
FROM topics
WHERE LOWER(name) = LOWER($1);

-- name: ListTopics :many
SELECT id, name, slug, description, display_order, created_at
FROM topics
ORDER BY display_order ASC, name ASC;

-- name: CreatePattern :one
INSERT INTO patterns (
    topic_id,
    parent_id,
    name,
    slug,
    description,
    display_order
) VALUES (
    $1, $2, $3, $4, $5, $6
) ON CONFLICT (slug) DO UPDATE
SET name = EXCLUDED.name,
    description = EXCLUDED.description,
    display_order = EXCLUDED.display_order
RETURNING id, topic_id, parent_id, name, slug, description, display_order, created_at;

-- name: GetPatternBySlug :one
SELECT id, topic_id, parent_id, name, slug, description, display_order, created_at
FROM patterns
WHERE slug = $1;

-- name: GetPatternByName :one
SELECT id, topic_id, parent_id, name, slug, description, display_order, created_at
FROM patterns
WHERE LOWER(name) = LOWER($1);

-- name: ListPatterns :many
SELECT id, topic_id, parent_id, name, slug, description, display_order, created_at
FROM patterns
ORDER BY display_order ASC, name ASC;

-- name: ListPatternsByTopic :many
SELECT id, topic_id, parent_id, name, slug, description, display_order, created_at
FROM patterns
WHERE topic_id = $1
ORDER BY display_order ASC, name ASC;

-- name: UpsertQuestion :one
INSERT INTO questions (
    slug,
    title,
    external_url,
    difficulty,
    is_core,
    editorial_summary,
    sample_time_limit_mins
) VALUES (
    $1, $2, $3, $4, $5, $6, $7
) ON CONFLICT (external_url) DO UPDATE
SET title = EXCLUDED.title,
    difficulty = EXCLUDED.difficulty,
    is_core = EXCLUDED.is_core,
    editorial_summary = EXCLUDED.editorial_summary,
    sample_time_limit_mins = EXCLUDED.sample_time_limit_mins,
    updated_at = CURRENT_TIMESTAMP
RETURNING id, slug, title, external_url, difficulty, is_core, editorial_summary, sample_time_limit_mins, created_at, updated_at;

-- name: GetQuestionByID :one
SELECT id, slug, title, external_url, difficulty, is_core, editorial_summary, sample_time_limit_mins, created_at, updated_at
FROM questions
WHERE id = $1;

-- name: GetQuestionBySlug :one
SELECT id, slug, title, external_url, difficulty, is_core, editorial_summary, sample_time_limit_mins, created_at, updated_at
FROM questions
WHERE slug = $1;

-- name: GetQuestionByExternalURL :one
SELECT id, slug, title, external_url, difficulty, is_core, editorial_summary, sample_time_limit_mins, created_at, updated_at
FROM questions
WHERE external_url = $1;

-- name: ListQuestions :many
SELECT q.id, q.slug, q.title, q.external_url, q.difficulty, q.is_core, q.editorial_summary, q.sample_time_limit_mins, q.created_at, q.updated_at
FROM questions q
WHERE ($1::text = '' OR q.title ILIKE '%' || $1 || '%')
  AND ($2::text = '' OR q.difficulty::text = $2)
  AND (
      $3::uuid IS NULL OR EXISTS (
          SELECT 1 FROM question_patterns qp
          JOIN patterns p ON qp.pattern_id = p.id
          WHERE qp.question_id = q.id AND (p.id = $3 OR p.topic_id = $3)
      )
  )
ORDER BY q.title ASC
LIMIT $4 OFFSET $5;

-- name: CountQuestions :one
SELECT COUNT(q.id)
FROM questions q
WHERE ($1::text = '' OR q.title ILIKE '%' || $1 || '%')
  AND ($2::text = '' OR q.difficulty::text = $2)
  AND (
      $3::uuid IS NULL OR EXISTS (
          SELECT 1 FROM question_patterns qp
          JOIN patterns p ON qp.pattern_id = p.id
          WHERE qp.question_id = q.id AND (p.id = $3 OR p.topic_id = $3)
      )
  );

-- name: LinkQuestionPattern :exec
INSERT INTO question_patterns (
    question_id,
    pattern_id,
    is_primary
) VALUES (
    $1, $2, $3
) ON CONFLICT (question_id, pattern_id) DO UPDATE
SET is_primary = EXCLUDED.is_primary;

-- name: GetQuestionPatterns :many
SELECT p.id, p.topic_id, p.name, p.slug, qp.is_primary, t.name as topic_name, t.slug as topic_slug
FROM question_patterns qp
JOIN patterns p ON qp.pattern_id = p.id
LEFT JOIN topics t ON p.topic_id = t.id
WHERE qp.question_id = $1
ORDER BY qp.is_primary DESC, p.name ASC;

-- name: CreateCompany :one
INSERT INTO companies (
    name,
    slug
) VALUES (
    $1, $2
) ON CONFLICT (name) DO UPDATE
SET slug = EXCLUDED.slug
RETURNING id, name, slug, created_at;

-- name: GetCompanyByName :one
SELECT id, name, slug, created_at
FROM companies
WHERE LOWER(name) = LOWER($1);

-- name: LinkQuestionCompany :exec
INSERT INTO question_company_evidence (
    question_id,
    company_id,
    source_description,
    confidence
) VALUES (
    $1, $2, $3, $4
) ON CONFLICT (question_id, company_id, source_description, evidence_date) DO NOTHING;

-- name: GetQuestionCompanies :many
SELECT c.id, c.name, c.slug, e.confidence, e.source_description
FROM question_company_evidence e
JOIN companies c ON e.company_id = c.id
WHERE e.question_id = $1
ORDER BY c.name ASC;

-- name: CreateRoadmap :one
INSERT INTO roadmaps (
    user_id,
    title,
    slug,
    description,
    target_days,
    is_system_template
) VALUES (
    $1, $2, $3, $4, $5, $6
) ON CONFLICT (slug) DO UPDATE
SET title = EXCLUDED.title,
    description = EXCLUDED.description,
    target_days = EXCLUDED.target_days,
    updated_at = CURRENT_TIMESTAMP
RETURNING id, user_id, title, slug, description, target_days, is_system_template, created_at, updated_at;

-- name: GetRoadmapBySlug :one
SELECT id, user_id, title, slug, description, target_days, is_system_template, created_at, updated_at
FROM roadmaps
WHERE slug = $1;

-- name: GetRoadmapByID :one
SELECT id, user_id, title, slug, description, target_days, is_system_template, created_at, updated_at
FROM roadmaps
WHERE id = $1;

-- name: ListCuratedRoadmaps :many
SELECT id, user_id, title, slug, description, target_days, is_system_template, created_at, updated_at
FROM roadmaps
WHERE is_system_template = TRUE
ORDER BY title ASC;

-- name: AddRoadmapItem :exec
INSERT INTO roadmap_items (
    roadmap_id,
    question_id,
    stage_number,
    order_index,
    is_mandatory
) VALUES (
    $1, $2, $3, $4, $5
) ON CONFLICT (roadmap_id, question_id) DO UPDATE
SET stage_number = EXCLUDED.stage_number,
    order_index = EXCLUDED.order_index,
    is_mandatory = EXCLUDED.is_mandatory;

-- name: ListRoadmapItems :many
SELECT ri.id, ri.roadmap_id, ri.question_id, ri.stage_number, ri.order_index, ri.is_mandatory,
       q.title, q.slug as question_slug, q.difficulty, q.is_core, q.sample_time_limit_mins
FROM roadmap_items ri
JOIN questions q ON ri.question_id = q.id
WHERE ri.roadmap_id = $1
ORDER BY ri.stage_number ASC, ri.order_index ASC;
