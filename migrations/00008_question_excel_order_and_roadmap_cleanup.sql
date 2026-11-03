-- +goose Up
-- SQL in this section is executed when the migration is applied.

-- 1. Add excel_order column to questions
ALTER TABLE questions ADD COLUMN IF NOT EXISTS excel_order INT NOT NULL DEFAULT 0;
CREATE INDEX IF NOT EXISTS idx_questions_excel_order ON questions(excel_order ASC);

-- 2. Remove static company roadmap templates so only Master Question Bank / Curriculum exists by default
DELETE FROM roadmap_items WHERE roadmap_id IN (
    SELECT id FROM roadmaps WHERE slug IN ('blind-75', 'google-interview-roadmap', 'meta-interview-roadmap', 'amazon-interview-roadmap')
);
DELETE FROM roadmaps WHERE slug IN ('blind-75', 'google-interview-roadmap', 'meta-interview-roadmap', 'amazon-interview-roadmap');

-- 3. Reset any user_profiles referencing the removed roadmap IDs
UPDATE user_profiles
SET selected_roadmap_id = 'a0000000-0000-0000-0000-000000000001'
WHERE selected_roadmap_id NOT IN (SELECT id FROM roadmaps);

-- +goose Down
-- SQL in this section is executed when the migration is rolled back.
DROP INDEX IF EXISTS idx_questions_excel_order;
ALTER TABLE questions DROP COLUMN IF EXISTS excel_order;
