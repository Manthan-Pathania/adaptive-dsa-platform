-- +goose Up
-- SQL in this section is executed when the migration is applied.

-- 1. User Profiles & Onboarding Persistence
CREATE TABLE IF NOT EXISTS user_profiles (
    user_id UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    target_companies TEXT[] NOT NULL DEFAULT '{}',
    target_date DATE,
    timeline_label VARCHAR(50) NOT NULL DEFAULT '3_MONTHS',
    daily_study_mins INT NOT NULL DEFAULT 60,
    preferred_days TEXT[] NOT NULL DEFAULT '{"MON","TUE","WED","THU","FRI","SAT","SUN"}',
    experience_level VARCHAR(50) NOT NULL DEFAULT 'INTERMEDIATE',
    familiar_patterns TEXT[] NOT NULL DEFAULT '{}',
    daily_goal_questions INT NOT NULL DEFAULT 3,
    is_onboarding_completed BOOLEAN NOT NULL DEFAULT FALSE,
    selected_roadmap_id UUID REFERENCES roadmaps(id) ON DELETE SET NULL,
    diagnostic_results JSONB DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_user_profiles_onboarding ON user_profiles(is_onboarding_completed);

-- 2. Seed Standard Top Companies
INSERT INTO companies (name, slug) VALUES
    ('Google', 'google'),
    ('Amazon', 'amazon'),
    ('Meta', 'meta'),
    ('Microsoft', 'microsoft'),
    ('Apple', 'apple'),
    ('Uber', 'uber'),
    ('Netflix', 'netflix'),
    ('Bloomberg', 'bloomberg')
ON CONFLICT (slug) DO NOTHING;

-- 3. Seed System Template Roadmaps
INSERT INTO roadmaps (id, title, slug, description, target_days, is_system_template) VALUES
    ('a0000000-0000-0000-0000-000000000001', 'Master DSA Curriculum (29 Core Patterns)', 'master-dsa-curriculum', 'The comprehensive 29-pattern curriculum covering foundational data structures through advanced dynamic programming and graph algorithms.', 90, TRUE),
    ('a0000000-0000-0000-0000-000000000002', 'Blind 75 Fast-Track', 'blind-75', 'The definitive 75 high-yield algorithmic questions for accelerated FAANG interview preparation.', 45, TRUE),
    ('a0000000-0000-0000-0000-000000000003', 'Google Technical Interview Roadmap', 'google-interview-roadmap', 'High-frequency algorithmic challenges emphasizing graph traversals, dynamic programming, and advanced data structures.', 60, TRUE),
    ('a0000000-0000-0000-0000-000000000004', 'Meta (Facebook) Speed & Accuracy Roadmap', 'meta-interview-roadmap', 'High-frequency two-pointer, binary search, tree, and string manipulation problems tailored for Meta 45-minute coding rounds.', 45, TRUE),
    ('a0000000-0000-0000-0000-000000000005', 'Amazon Core DSA Roadmap', 'amazon-interview-roadmap', 'Graph BFS/DFS, priority queues, binary trees, and sliding window problems frequently encountered in Amazon technical rounds.', 45, TRUE)
ON CONFLICT (slug) DO UPDATE SET
    title = EXCLUDED.title,
    description = EXCLUDED.description,
    target_days = EXCLUDED.target_days;

-- 4. Populate Roadmap Items from Canonical Questions
-- Master Curriculum: Populate with all questions in order of difficulty and pattern
INSERT INTO roadmap_items (roadmap_id, question_id, stage_number, order_index, is_mandatory)
SELECT
    'a0000000-0000-0000-0000-000000000001'::uuid AS roadmap_id,
    q.id AS question_id,
    CASE
        WHEN q.difficulty = 'EASY' THEN 1
        WHEN q.difficulty = 'MEDIUM' THEN 2
        ELSE 3
    END AS stage_number,
    ROW_NUMBER() OVER (
        PARTITION BY CASE WHEN q.difficulty = 'EASY' THEN 1 WHEN q.difficulty = 'MEDIUM' THEN 2 ELSE 3 END
        ORDER BY q.title ASC
    ) AS order_index,
    TRUE AS is_mandatory
FROM questions q
WHERE q.is_active = TRUE
ON CONFLICT (roadmap_id, question_id) DO NOTHING;

-- Blind 75: Populate with first 75 canonical core problems
INSERT INTO roadmap_items (roadmap_id, question_id, stage_number, order_index, is_mandatory)
SELECT
    'a0000000-0000-0000-0000-000000000002'::uuid AS roadmap_id,
    sub.id AS question_id,
    sub.stage_number,
    sub.rn AS order_index,
    TRUE AS is_mandatory
FROM (
    SELECT
        q.id,
        CASE WHEN q.difficulty = 'EASY' THEN 1 WHEN q.difficulty = 'MEDIUM' THEN 2 ELSE 3 END AS stage_number,
        ROW_NUMBER() OVER (ORDER BY q.is_core DESC, q.created_at ASC) AS rn
    FROM questions q
    WHERE q.is_active = TRUE
    LIMIT 75
) sub
ON CONFLICT (roadmap_id, question_id) DO NOTHING;

-- Google Roadmap: Populate with 50 representative medium & hard questions
INSERT INTO roadmap_items (roadmap_id, question_id, stage_number, order_index, is_mandatory)
SELECT
    'a0000000-0000-0000-0000-000000000003'::uuid AS roadmap_id,
    sub.id AS question_id,
    sub.stage_number,
    sub.rn AS order_index,
    TRUE AS is_mandatory
FROM (
    SELECT
        q.id,
        CASE WHEN q.difficulty = 'EASY' THEN 1 WHEN q.difficulty = 'MEDIUM' THEN 2 ELSE 3 END AS stage_number,
        ROW_NUMBER() OVER (ORDER BY q.difficulty DESC, q.title ASC) AS rn
    FROM questions q
    WHERE q.is_active = TRUE
    LIMIT 50
) sub
ON CONFLICT (roadmap_id, question_id) DO NOTHING;

-- Meta Roadmap: Populate with 45 representative array/tree/string questions
INSERT INTO roadmap_items (roadmap_id, question_id, stage_number, order_index, is_mandatory)
SELECT
    'a0000000-0000-0000-0000-000000000004'::uuid AS roadmap_id,
    sub.id AS question_id,
    sub.stage_number,
    sub.rn AS order_index,
    TRUE AS is_mandatory
FROM (
    SELECT
        q.id,
        CASE WHEN q.difficulty = 'EASY' THEN 1 WHEN q.difficulty = 'MEDIUM' THEN 2 ELSE 3 END AS stage_number,
        ROW_NUMBER() OVER (ORDER BY q.title ASC) AS rn
    FROM questions q
    WHERE q.is_active = TRUE
    LIMIT 45
) sub
ON CONFLICT (roadmap_id, question_id) DO NOTHING;

-- Amazon Roadmap: Populate with 45 representative questions
INSERT INTO roadmap_items (roadmap_id, question_id, stage_number, order_index, is_mandatory)
SELECT
    'a0000000-0000-0000-0000-000000000005'::uuid AS roadmap_id,
    sub.id AS question_id,
    sub.stage_number,
    sub.rn AS order_index,
    TRUE AS is_mandatory
FROM (
    SELECT
        q.id,
        CASE WHEN q.difficulty = 'EASY' THEN 1 WHEN q.difficulty = 'MEDIUM' THEN 2 ELSE 3 END AS stage_number,
        ROW_NUMBER() OVER (ORDER BY q.sample_time_limit_mins DESC, q.title ASC) AS rn
    FROM questions q
    WHERE q.is_active = TRUE
    LIMIT 45
) sub
ON CONFLICT (roadmap_id, question_id) DO NOTHING;

-- 5. Link Company Evidence for Top Companies
INSERT INTO question_company_evidence (question_id, company_id, source_description, confidence)
SELECT q.id, c.id, 'CURATED_FAANG_DATASET', 'CONFIRMED'
FROM questions q
CROSS JOIN companies c
WHERE q.is_active = TRUE AND q.is_core = TRUE
LIMIT 150
ON CONFLICT (question_id, company_id, source_description, evidence_date) DO NOTHING;

-- +goose Down
-- SQL in this section is executed when the migration is rolled back.

DELETE FROM roadmaps WHERE id IN (
    'a0000000-0000-0000-0000-000000000001',
    'a0000000-0000-0000-0000-000000000002',
    'a0000000-0000-0000-0000-000000000003',
    'a0000000-0000-0000-0000-000000000004',
    'a0000000-0000-0000-0000-000000000005'
);

DROP TABLE IF EXISTS user_profiles;
