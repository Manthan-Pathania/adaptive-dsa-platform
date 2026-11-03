-- +goose Up
-- SQL in this section is executed when the migration is applied.

-- 1. Add is_active and leetcode_id to questions
ALTER TABLE questions 
    ADD COLUMN IF NOT EXISTS is_active BOOLEAN NOT NULL DEFAULT TRUE,
    ADD COLUMN IF NOT EXISTS leetcode_id VARCHAR(50);

CREATE INDEX IF NOT EXISTS idx_questions_is_active ON questions(is_active);
CREATE INDEX IF NOT EXISTS idx_questions_leetcode_id ON questions(leetcode_id);

-- 2. Question import history table
CREATE TABLE IF NOT EXISTS question_import_history (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    filename VARCHAR(255) NOT NULL,
    total_rows INT NOT NULL,
    valid_rows INT NOT NULL,
    created_questions INT NOT NULL,
    updated_questions INT NOT NULL,
    unchanged_questions INT NOT NULL,
    invalid_rows INT NOT NULL,
    status VARCHAR(50) NOT NULL,
    imported_by UUID REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_import_history_created_at ON question_import_history(created_at DESC);

-- +goose Down
-- SQL in this section is executed when the migration is rolled back.

DROP TABLE IF EXISTS question_import_history;

DROP INDEX IF EXISTS idx_questions_leetcode_id;
DROP INDEX IF EXISTS idx_questions_is_active;

ALTER TABLE questions 
    DROP COLUMN IF EXISTS leetcode_id,
    DROP COLUMN IF EXISTS is_active;
