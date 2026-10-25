package progress_test

import (
	"context"
	"io"
	"log/slog"
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

func getTestProgressPool(t *testing.T) (*database.DB, progress.Repository, progress.Service, catalog.Repository) {
	t.Helper()
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		t.Skip("skipping PostgreSQL integration test: DATABASE_URL not set")
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	cfg := config.DatabaseConfig{
		URL:               dbURL,
		MaxConns:          25,
		MinConns:          5,
		MaxConnLifetime:   5 * time.Minute,
		MaxConnIdleTime:   1 * time.Minute,
		HealthCheckPeriod: 30 * time.Second,
		ConnectTimeout:    3 * time.Second,
	}

	ctx := context.Background()
	db, err := database.New(ctx, cfg, logger)
	require.NoError(t, err)

	progRepo := progress.NewPostgresRepository(db.Pool())
	progSvc := progress.NewService(progRepo)
	catRepo := catalog.NewPostgresRepository(db.Pool())

	return db, progRepo, progSvc, catRepo
}

func createTestUser(t *testing.T, db *database.DB) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	userID := uuid.New()
	email := "test-" + userID.String()[:8] + "@example.com"
	_, err := db.Pool().Exec(ctx, `
		INSERT INTO users (id, email, password_hash, full_name, role)
		VALUES ($1, $2, 'dummy-hash', 'Test User', 'CANDIDATE')
	`, userID, email)
	require.NoError(t, err)
	return userID
}

func TestProgress_Postgres_RecordAttemptAndSM2(t *testing.T) {
	db, _, svc, catRepo := getTestProgressPool(t)
	defer db.Close()
	ctx := context.Background()

	userID := createTestUser(t, db)
	uniqueSuffix := uuid.New().String()[:8]

	// Create question in catalog
	q, err := catRepo.UpsertQuestion(ctx, &catalog.Question{
		Title:       "Progress Question " + uniqueSuffix,
		Slug:        "progress-question-" + uniqueSuffix,
		ExternalURL: "https://leetcode.com/problems/progress-question-" + uniqueSuffix + "/",
		Difficulty:  catalog.DifficultyMedium,
	})
	require.NoError(t, err)

	// 1. First attempt: Solved with Hints (quality rating 3)
	att1, prog1, rev1, err := svc.RecordAttempt(ctx, progress.RecordAttemptRequest{
		UserID:                userID,
		QuestionID:            q.ID,
		Outcome:               progress.OutcomeSolvedWithHints,
		TimeSpentSecs:         1200,
		DifficultyExperienced: progress.PerceivedJustRight,
		QualityRating:         3,
		MistakeTags:           []string{"CORNER_CASE_MISSED"},
	})
	require.NoError(t, err)
	assert.NotEqual(t, uuid.Nil, att1.ID)
	assert.Equal(t, progress.OutcomeSolvedWithHints, att1.Outcome)
	assert.Equal(t, progress.StatusSolved, prog1.Status)
	assert.Equal(t, int32(1), prog1.SolveCount)
	assert.Equal(t, int32(0), prog1.IndependentSolveCount)
	assert.Equal(t, int32(1), rev1.RepetitionCount)
	assert.Equal(t, int32(1), rev1.IntervalDays)

	// 2. Second attempt: Solved Independently (quality rating 5)
	att2, prog2, rev2, err := svc.RecordAttempt(ctx, progress.RecordAttemptRequest{
		UserID:                userID,
		QuestionID:            q.ID,
		Outcome:               progress.OutcomeSolvedIndependently,
		TimeSpentSecs:         600,
		DifficultyExperienced: progress.PerceivedEasy,
		QualityRating:         5,
	})
	require.NoError(t, err)
	assert.Equal(t, progress.OutcomeSolvedIndependently, att2.Outcome)
	assert.Equal(t, progress.StatusSolved, prog2.Status)
	assert.Equal(t, int32(2), prog2.SolveCount)
	assert.Equal(t, int32(1), prog2.IndependentSolveCount)
	assert.Equal(t, int32(1800), prog2.TotalTimeSpentSecs) // 1200 + 600
	assert.Equal(t, int32(2), rev2.RepetitionCount)
	assert.Equal(t, int32(6), rev2.IntervalDays) // Second successful repetition interval = 6 days

	// 3. Third attempt: Solved Independently second time -> Status progresses to MASTERED!
	_, prog3, rev3, err := svc.RecordAttempt(ctx, progress.RecordAttemptRequest{
		UserID:                userID,
		QuestionID:            q.ID,
		Outcome:               progress.OutcomeSolvedIndependently,
		TimeSpentSecs:         400,
		DifficultyExperienced: progress.PerceivedEasy,
		QualityRating:         5,
	})
	require.NoError(t, err)
	assert.Equal(t, progress.StatusMastered, prog3.Status)
	assert.Equal(t, int32(3), prog3.SolveCount)
	assert.Equal(t, int32(2), prog3.IndependentSolveCount)
	assert.Equal(t, int32(3), rev3.RepetitionCount)
	assert.Greater(t, rev3.IntervalDays, int32(6)) // Expands interval past 6 days
}

func TestProgress_Postgres_CrossRoadmapProgressSharing(t *testing.T) {
	db, _, svc, catRepo := getTestProgressPool(t)
	defer db.Close()
	ctx := context.Background()

	userID := createTestUser(t, db)
	uniqueSuffix := uuid.New().String()[:8]

	q, err := catRepo.UpsertQuestion(ctx, &catalog.Question{
		Title:       "Cross Roadmap Q " + uniqueSuffix,
		Slug:        "cross-roadmap-q-" + uniqueSuffix,
		ExternalURL: "https://leetcode.com/problems/cross-roadmap-q-" + uniqueSuffix + "/",
		Difficulty:  catalog.DifficultyEasy,
	})
	require.NoError(t, err)

	// Create 2 roadmaps
	rA, err := catRepo.CreateRoadmap(ctx, &catalog.Roadmap{
		Title: "Roadmap A " + uniqueSuffix,
		Slug:  "roadmap-a-" + uniqueSuffix,
	})
	require.NoError(t, err)

	rB, err := catRepo.CreateRoadmap(ctx, &catalog.Roadmap{
		Title: "Roadmap B " + uniqueSuffix,
		Slug:  "roadmap-b-" + uniqueSuffix,
	})
	require.NoError(t, err)

	// Link same question to both roadmaps
	err = catRepo.AddRoadmapItem(ctx, rA.ID, q.ID, 1, 1, true)
	require.NoError(t, err)
	err = catRepo.AddRoadmapItem(ctx, rB.ID, q.ID, 1, 1, true)
	require.NoError(t, err)

	// Record solve from Roadmap A
	_, progA, _, err := svc.RecordAttempt(ctx, progress.RecordAttemptRequest{
		UserID:        userID,
		QuestionID:    q.ID,
		RoadmapID:     &rA.ID,
		Outcome:       progress.OutcomeSolvedIndependently,
		TimeSpentSecs: 500,
		QualityRating: 5,
	})
	require.NoError(t, err)
	assert.Equal(t, progress.StatusSolved, progA.Status)

	// Query progress directly: shows StatusSolved regardless of Roadmap B
	canonicalProg, err := svc.GetQuestionProgress(ctx, userID, q.ID)
	require.NoError(t, err)
	assert.Equal(t, progress.StatusSolved, canonicalProg.Status)
	assert.Equal(t, int32(1), canonicalProg.SolveCount)
}

func TestProgress_Postgres_ConcurrentAttempts(t *testing.T) {
	db, repo, svc, catRepo := getTestProgressPool(t)
	defer db.Close()
	ctx := context.Background()

	userID := createTestUser(t, db)
	uniqueSuffix := uuid.New().String()[:8]

	q, err := catRepo.UpsertQuestion(ctx, &catalog.Question{
		Title:       "Concurrent Q " + uniqueSuffix,
		Slug:        "concurrent-q-" + uniqueSuffix,
		ExternalURL: "https://leetcode.com/problems/concurrent-q-" + uniqueSuffix + "/",
		Difficulty:  catalog.DifficultyMedium,
	})
	require.NoError(t, err)

	const concurrency = 15
	errChan := make(chan error, concurrency)

	for i := 0; i < concurrency; i++ {
		go func() {
			_, _, _, err := svc.RecordAttempt(ctx, progress.RecordAttemptRequest{
				UserID:                userID,
				QuestionID:            q.ID,
				Outcome:               progress.OutcomeSolvedIndependently,
				TimeSpentSecs:         100,
				DifficultyExperienced: progress.PerceivedJustRight,
				QualityRating:         4,
			})
			errChan <- err
		}()
	}

	for i := 0; i < concurrency; i++ {
		err := <-errChan
		require.NoError(t, err)
	}

	// 1. Verify all attempts exist in PostgreSQL table
	attempts, err := repo.ListQuestionAttempts(ctx, userID, q.ID)
	require.NoError(t, err)
	assert.Equal(t, concurrency, len(attempts))

	// 2. Verify canonical progress has atomic totals without lost updates
	prog, err := svc.GetQuestionProgress(ctx, userID, q.ID)
	require.NoError(t, err)
	assert.Equal(t, progress.StatusMastered, prog.Status)
	assert.Equal(t, int32(concurrency), prog.SolveCount)
	assert.Equal(t, int32(concurrency), prog.IndependentSolveCount)
	assert.Equal(t, int32(concurrency*100), prog.TotalTimeSpentSecs)
}
