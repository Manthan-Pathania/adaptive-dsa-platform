-- +goose Up
-- SQL in this section is executed when the migration is applied.

-- Core Question Catalog Enums
CREATE TYPE question_difficulty AS ENUM ('EASY', 'MEDIUM', 'HARD');
CREATE TYPE evidence_confidence AS ENUM ('LOW', 'MEDIUM', 'HIGH', 'CONFIRMED');
CREATE TYPE progress_status AS ENUM ('NOT_STARTED', 'IN_PROGRESS', 'SOLVED', 'MASTERED');
CREATE TYPE attempt_outcome AS ENUM (
    'SOLVED_INDEPENDENTLY',
    'SOLVED_WITH_HINTS',
    'SOLVED_AFTER_EDITORIAL',
    'UNRESOLVED_FAILED'
);
CREATE TYPE perceived_difficulty AS ENUM ('EASY', 'JUST_RIGHT', 'CHALLENGING', 'OVERWHELMING');

-- 1. Topics table (e.g. Arrays, Trees, Dynamic Programming)
CREATE TABLE IF NOT EXISTS topics (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(100) NOT NULL UNIQUE,
    slug VARCHAR(100) NOT NULL UNIQUE,
    description TEXT,
    display_order INT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_topics_display_order ON topics(display_order);

-- 2. Patterns table (Hierarchical pattern tree linked to topics)
CREATE TABLE IF NOT EXISTS patterns (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    topic_id UUID REFERENCES topics(id) ON DELETE CASCADE,
    parent_id UUID REFERENCES patterns(id) ON DELETE CASCADE,
    name VARCHAR(100) NOT NULL,
    slug VARCHAR(100) NOT NULL UNIQUE,
    description TEXT,
    display_order INT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_patterns_topic_id ON patterns(topic_id);
CREATE INDEX IF NOT EXISTS idx_patterns_parent_id ON patterns(parent_id);

-- 3. Canonical Questions Bank (Single source of truth)
CREATE TABLE IF NOT EXISTS questions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    slug VARCHAR(150) NOT NULL UNIQUE,
    title VARCHAR(255) NOT NULL,
    external_url TEXT NOT NULL UNIQUE,
    difficulty question_difficulty NOT NULL,
    is_core BOOLEAN NOT NULL DEFAULT FALSE,
    editorial_summary TEXT,
    sample_time_limit_mins INT NOT NULL DEFAULT 30,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_questions_difficulty ON questions(difficulty);
CREATE INDEX IF NOT EXISTS idx_questions_slug ON questions(slug);

-- 4. Question to Patterns many-to-many junction
CREATE TABLE IF NOT EXISTS question_patterns (
    question_id UUID NOT NULL REFERENCES questions(id) ON DELETE CASCADE,
    pattern_id UUID NOT NULL REFERENCES patterns(id) ON DELETE CASCADE,
    is_primary BOOLEAN NOT NULL DEFAULT FALSE,
    PRIMARY KEY (question_id, pattern_id)
);

CREATE INDEX IF NOT EXISTS idx_question_patterns_pattern ON question_patterns(pattern_id);

-- 5. Question Prerequisites
CREATE TABLE IF NOT EXISTS question_prerequisites (
    question_id UUID NOT NULL REFERENCES questions(id) ON DELETE CASCADE,
    prerequisite_question_id UUID NOT NULL REFERENCES questions(id) ON DELETE CASCADE,
    reason VARCHAR(255),
    PRIMARY KEY (question_id, prerequisite_question_id),
    CONSTRAINT chk_no_self_prereq CHECK (question_id <> prerequisite_question_id)
);

-- 6. Companies
CREATE TABLE IF NOT EXISTS companies (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(100) NOT NULL UNIQUE,
    slug VARCHAR(100) NOT NULL UNIQUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- 7. Question Company Evidence
CREATE TABLE IF NOT EXISTS question_company_evidence (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    question_id UUID NOT NULL REFERENCES questions(id) ON DELETE CASCADE,
    company_id UUID NOT NULL REFERENCES companies(id) ON DELETE CASCADE,
    source_description VARCHAR(255) NOT NULL DEFAULT 'COMMUNITY_REPORTED',
    evidence_date DATE NOT NULL DEFAULT CURRENT_DATE,
    confidence evidence_confidence NOT NULL DEFAULT 'CONFIRMED',
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (question_id, company_id, source_description, evidence_date)
);

CREATE INDEX IF NOT EXISTS idx_company_evidence_lookup ON question_company_evidence(company_id, question_id);

-- 8. Curated and User Roadmaps
CREATE TABLE IF NOT EXISTS roadmaps (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID REFERENCES users(id) ON DELETE CASCADE,
    title VARCHAR(150) NOT NULL,
    slug VARCHAR(150) NOT NULL UNIQUE,
    description TEXT,
    target_days INT NOT NULL DEFAULT 60,
    is_system_template BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- 9. Roadmap Items (Ordered mapping to questions)
CREATE TABLE IF NOT EXISTS roadmap_items (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    roadmap_id UUID NOT NULL REFERENCES roadmaps(id) ON DELETE CASCADE,
    question_id UUID NOT NULL REFERENCES questions(id) ON DELETE CASCADE,
    stage_number INT NOT NULL DEFAULT 1,
    order_index INT NOT NULL,
    is_mandatory BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (roadmap_id, question_id),
    UNIQUE (roadmap_id, stage_number, order_index)
);

CREATE INDEX IF NOT EXISTS idx_roadmap_items_roadmap ON roadmap_items(roadmap_id, stage_number, order_index);

-- 10. Canonical User Question Progress (Shared across all roadmaps)
CREATE TABLE IF NOT EXISTS user_question_progress (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    question_id UUID NOT NULL REFERENCES questions(id) ON DELETE CASCADE,
    status progress_status NOT NULL DEFAULT 'NOT_STARTED',
    first_solved_at TIMESTAMPTZ,
    last_solved_at TIMESTAMPTZ,
    solve_count INT NOT NULL DEFAULT 0,
    independent_solve_count INT NOT NULL DEFAULT 0,
    total_time_spent_secs INT NOT NULL DEFAULT 0,
    is_bookmarked BOOLEAN NOT NULL DEFAULT FALSE,
    personal_notes TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (user_id, question_id)
);

CREATE INDEX IF NOT EXISTS idx_user_progress_user_id ON user_question_progress(user_id);
CREATE INDEX IF NOT EXISTS idx_user_progress_status ON user_question_progress(user_id, status);

-- 11. Question Attempt History (Immutable append-only log)
CREATE TABLE IF NOT EXISTS question_attempts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    question_id UUID NOT NULL REFERENCES questions(id) ON DELETE CASCADE,
    roadmap_id UUID REFERENCES roadmaps(id) ON DELETE SET NULL,
    outcome attempt_outcome NOT NULL,
    time_spent_secs INT NOT NULL DEFAULT 0,
    difficulty_experienced perceived_difficulty NOT NULL DEFAULT 'JUST_RIGHT',
    quality_rating INT NOT NULL DEFAULT 3,
    mistake_tags TEXT[] NOT NULL DEFAULT '{}',
    approach_summary TEXT,
    code_snippet TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_attempts_user_question ON question_attempts(user_id, question_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_attempts_user_created ON question_attempts(user_id, created_at DESC);

-- 12. Revision Schedules (SM-2 Spaced Repetition Engine)
CREATE TABLE IF NOT EXISTS revision_schedules (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    question_id UUID NOT NULL REFERENCES questions(id) ON DELETE CASCADE,
    stability FLOAT8 NOT NULL DEFAULT 1.0,
    difficulty FLOAT8 NOT NULL DEFAULT 5.0,
    repetition_count INT NOT NULL DEFAULT 0,
    ease_factor NUMERIC(4,2) NOT NULL DEFAULT 2.50,
    interval_days INT NOT NULL DEFAULT 1,
    next_review_at TIMESTAMPTZ NOT NULL,
    last_reviewed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (user_id, question_id)
);

CREATE INDEX IF NOT EXISTS idx_revision_due ON revision_schedules(user_id, next_review_at);

-- +goose Down
-- SQL in this section is executed when the migration is rolled back.

DROP TABLE IF EXISTS revision_schedules;
DROP TABLE IF EXISTS question_attempts;
DROP TABLE IF EXISTS user_question_progress;
DROP TABLE IF EXISTS roadmap_items;
DROP TABLE IF EXISTS roadmaps;
DROP TABLE IF EXISTS question_company_evidence;
DROP TABLE IF EXISTS companies;
DROP TABLE IF EXISTS question_prerequisites;
DROP TABLE IF EXISTS question_patterns;
DROP TABLE IF EXISTS questions;
DROP TABLE IF EXISTS patterns;
DROP TABLE IF EXISTS topics;

DROP TYPE IF EXISTS perceived_difficulty;
DROP TYPE IF EXISTS attempt_outcome;
DROP TYPE IF EXISTS progress_status;
DROP TYPE IF EXISTS evidence_confidence;
DROP TYPE IF EXISTS question_difficulty;
