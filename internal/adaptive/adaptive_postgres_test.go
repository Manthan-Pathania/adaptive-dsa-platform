package adaptive

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/adaptive-dsa/platform/internal/catalog"
	"github.com/adaptive-dsa/platform/internal/config"
	"github.com/adaptive-dsa/platform/internal/platform/database"
	"github.com/adaptive-dsa/platform/internal/progress"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupPostgresAdaptive(t *testing.T) (Service, *database.DB, uuid.UUID) {
	t.Helper()

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://postgres@127.0.0.1:5432/adaptive_dsa_test?sslmode=disable"
	}

	cfg := config.DatabaseConfig{
		URL:               dbURL,
		MaxConns:          25,
		MinConns:          5,
		MaxConnLifetime:   5 * time.Minute,
		MaxConnIdleTime:   1 * time.Minute,
		HealthCheckPeriod: 30 * time.Second,
		ConnectTimeout:    5 * time.Second,
	}

	ctx := context.Background()
	db, err := database.New(ctx, cfg, nil)
	if err != nil {
		t.Skipf("skipping postgres integration test: database unavailable: %v", err)
	}

	testUserID := uuid.New()
	email := fmt.Sprintf("adaptive_test_%s@example.com", testUserID.String()[:8])
	_, err = db.Pool().Exec(ctx, `
		INSERT INTO users (id, email, password_hash, full_name, role)
		VALUES ($1, $2, 'dummy_hash', 'Adaptive Test Candidate', 'CANDIDATE')
	`, testUserID, email)
	require.NoError(t, err)

	t.Cleanup(func() {
		_, _ = db.Pool().Exec(context.Background(), "DELETE FROM users WHERE id = $1", testUserID)
		db.Close()
	})

	adaptiveRepo := NewPostgresRepository(db.Pool())
	progressRepo := progress.NewPostgresRepository(db.Pool())
	progressSvc := progress.NewService(progressRepo)
	catalogRepo := catalog.NewPostgresRepository(db.Pool())
	catalogSvc := catalog.NewService(catalogRepo)

	svc := NewService(adaptiveRepo, progressSvc, catalogSvc)
	return svc, db, testUserID
}

func TestAdaptive_Postgres_RealPatternMetricsAndDailyPlan(t *testing.T) {
	svc, db, userID := setupPostgresAdaptive(t)
	ctx := context.Background()

	// 1. Create a Topic and Pattern in PostgreSQL
	topicID := uuid.New()
	topicSlug := fmt.Sprintf("topic-%s", topicID.String()[:8])
	topicName := fmt.Sprintf("Topic-%s", topicID.String()[:8])
	_, err := db.Pool().Exec(ctx, `
		INSERT INTO topics (id, name, slug) VALUES ($1, $2, $3)
	`, topicID, topicName, topicSlug)
	require.NoError(t, err)

	patternID := uuid.New()
	patternSlug := fmt.Sprintf("pattern-%s", patternID.String()[:8])
	patternName := fmt.Sprintf("Monotonic Queue %s", patternID.String()[:8])
	_, err = db.Pool().Exec(ctx, `
		INSERT INTO patterns (id, topic_id, name, slug) VALUES ($1, $2, $3, $4)
	`, patternID, topicID, patternName, patternSlug)
	require.NoError(t, err)

	// 2. Create Question 1 and Link to Pattern
	q1ID := uuid.New()
	q1Slug := fmt.Sprintf("q1-%s", q1ID.String()[:8])
	q1URL := fmt.Sprintf("https://leetcode.com/problems/%s", q1Slug)
	_, err = db.Pool().Exec(ctx, `
		INSERT INTO questions (id, slug, title, external_url, difficulty, is_core)
		VALUES ($1, $2, 'Sliding Window Maximum', $3, 'HARD', true)
	`, q1ID, q1Slug, q1URL)
	require.NoError(t, err)

	_, err = db.Pool().Exec(ctx, `
		INSERT INTO question_patterns (question_id, pattern_id, is_primary)
		VALUES ($1, $2, true)
	`, q1ID, patternID)
	require.NoError(t, err)

	// 3. Create Question 2 (Unsolved in same pattern)
	q2ID := uuid.New()
	q2Slug := fmt.Sprintf("q2-%s", q2ID.String()[:8])
	q2URL := fmt.Sprintf("https://leetcode.com/problems/%s", q2Slug)
	_, err = db.Pool().Exec(ctx, `
		INSERT INTO questions (id, slug, title, external_url, difficulty, is_core)
		VALUES ($1, $2, 'Shortest Subarray with Sum at Least K', $3, 'HARD', false)
	`, q2ID, q2Slug, q2URL)
	require.NoError(t, err)

	_, err = db.Pool().Exec(ctx, `
		INSERT INTO question_patterns (question_id, pattern_id, is_primary)
		VALUES ($1, $2, true)
	`, q2ID, patternID)
	require.NoError(t, err)

	// 4. Log 2 FAILED attempts for Question 1 to establish a Weak Pattern
	_, err = db.Pool().Exec(ctx, `
		INSERT INTO question_attempts (user_id, question_id, outcome, time_spent_secs, difficulty_experienced, quality_rating, mistake_tags)
		VALUES ($1, $2, 'UNRESOLVED_FAILED', 1800, 'CHALLENGING', 1, '{"TIME_LIMIT_EXCEEDED"}'),
		       ($1, $2, 'UNRESOLVED_FAILED', 1500, 'OVERWHELMING', 0, '{"WRONG_DATA_STRUCTURE"}')
	`, userID, q1ID)
	require.NoError(t, err)

	// 5. Query Weak Patterns
	weak, err := svc.GetWeakPatterns(ctx, userID)
	require.NoError(t, err)
	require.NotEmpty(t, weak)
	assert.Equal(t, patternID, weak[0].PatternID)
	assert.Equal(t, patternName, weak[0].PatternName)
	assert.Equal(t, int64(2), weak[0].FailedAttempts)
	assert.True(t, weak[0].IsWeak)

	// 6. Generate Daily Plan: Should recommend Question 2 under Weak Pattern Reinforcement
	plan, err := svc.GetDailyPlan(ctx, userID, nil, time.Now().UTC())
	require.NoError(t, err)
	require.NotEmpty(t, plan.Recommendations)

	var foundWeakRec bool
	for _, rec := range plan.Recommendations {
		if rec.Category == CategoryWeakPattern && rec.QuestionID == q2ID {
			foundWeakRec = true
			assert.Contains(t, rec.Rationale, patternName)
			assert.GreaterOrEqual(t, rec.PriorityScore, int32(500))
		}
	}
	assert.True(t, foundWeakRec, "unsolved question 2 must be recommended to reinforce weak pattern")
}
