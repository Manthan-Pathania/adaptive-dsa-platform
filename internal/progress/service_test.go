package progress

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCalculateSM2(t *testing.T) {
	now := time.Date(2026, 10, 24, 12, 0, 0, 0, time.UTC)

	// Case 1: First attempt, perfect score (quality 5)
	res1 := CalculateSM2(0, 2.50, 1, 5, now)
	assert.Equal(t, int32(1), res1.RepetitionCount)
	assert.Equal(t, int32(1), res1.IntervalDays)
	assert.Equal(t, 2.60, res1.EaseFactor) // EF + (0.1 - 0) = 2.60
	assert.Equal(t, now.Add(24*time.Hour), res1.NextReviewAt)

	// Case 2: Second attempt, good score (quality 4)
	res2 := CalculateSM2(res1.RepetitionCount, res1.EaseFactor, res1.IntervalDays, 4, now)
	assert.Equal(t, int32(2), res2.RepetitionCount)
	assert.Equal(t, int32(6), res2.IntervalDays) // Rep 2 always interval 6
	assert.Equal(t, 2.60, res2.EaseFactor)       // EF + (0.1 - 1*(0.08+0.02)) = 2.60

	// Case 3: Third attempt, good score (quality 4)
	res3 := CalculateSM2(res2.RepetitionCount, res2.EaseFactor, res2.IntervalDays, 4, now)
	assert.Equal(t, int32(3), res3.RepetitionCount)
	assert.Equal(t, int32(16), res3.IntervalDays) // Round(6 * 2.60) = 16

	// Case 4: Fourth attempt, failure/struggle (quality 2) -> resets repetition chain!
	res4 := CalculateSM2(res3.RepetitionCount, res3.EaseFactor, res3.IntervalDays, 2, now)
	assert.Equal(t, int32(0), res4.RepetitionCount) // Reset to 0
	assert.Equal(t, int32(1), res4.IntervalDays)    // Reset to 1 day
	assert.Equal(t, res3.EaseFactor, res4.EaseFactor)
}

func TestService_RecordAttempt_HistoryPreservation(t *testing.T) {
	repo := NewInMemoryRepository()
	svc := NewService(repo)

	userID := uuid.New()
	questionID := uuid.New()

	ctx := context.Background()

	// 1. First attempt: Failed
	req1 := RecordAttemptRequest{
		UserID:                userID,
		QuestionID:            questionID,
		Outcome:               OutcomeUnresolvedFailed,
		TimeSpentSecs:         1800,
		DifficultyExperienced: PerceivedChallenging,
		QualityRating:         1,
	}
	att1, prog1, rev1, err := svc.RecordAttempt(ctx, req1)
	require.NoError(t, err)
	assert.Equal(t, OutcomeUnresolvedFailed, att1.Outcome)
	assert.Equal(t, StatusInProgress, prog1.Status)
	assert.Equal(t, int32(0), prog1.SolveCount)
	assert.Equal(t, int32(0), rev1.RepetitionCount)

	// 2. Second attempt: Solved with Hints
	req2 := RecordAttemptRequest{
		UserID:                userID,
		QuestionID:            questionID,
		Outcome:               OutcomeSolvedWithHints,
		TimeSpentSecs:         1200,
		DifficultyExperienced: PerceivedJustRight,
		QualityRating:         3,
	}
	att2, prog2, rev2, err := svc.RecordAttempt(ctx, req2)
	require.NoError(t, err)
	assert.Equal(t, OutcomeSolvedWithHints, att2.Outcome)
	assert.Equal(t, StatusSolved, prog2.Status)
	assert.Equal(t, int32(1), prog2.SolveCount)
	assert.Equal(t, int32(0), prog2.IndependentSolveCount)
	assert.Equal(t, int32(3000), prog2.TotalTimeSpentSecs) // 1800 + 1200
	assert.Equal(t, int32(1), rev2.RepetitionCount)

	// 3. Third attempt: Solved Independently
	req3 := RecordAttemptRequest{
		UserID:                userID,
		QuestionID:            questionID,
		Outcome:               OutcomeSolvedIndependently,
		TimeSpentSecs:         600,
		DifficultyExperienced: PerceivedEasy,
		QualityRating:         5,
	}
	att3, prog3, rev3, err := svc.RecordAttempt(ctx, req3)
	require.NoError(t, err)
	assert.Equal(t, OutcomeSolvedIndependently, att3.Outcome)
	assert.Equal(t, StatusSolved, prog3.Status)
	assert.Equal(t, int32(2), prog3.SolveCount)
	assert.Equal(t, int32(1), prog3.IndependentSolveCount)
	assert.Equal(t, int32(2), rev3.RepetitionCount)

	// 4. Fourth attempt: Solved Independently second time -> Reaches MASTERED!
	req4 := RecordAttemptRequest{
		UserID:                userID,
		QuestionID:            questionID,
		Outcome:               OutcomeSolvedIndependently,
		TimeSpentSecs:         400,
		DifficultyExperienced: PerceivedEasy,
		QualityRating:         5,
	}
	_, prog4, _, err := svc.RecordAttempt(ctx, req4)
	require.NoError(t, err)
	assert.Equal(t, StatusMastered, prog4.Status)
	assert.Equal(t, int32(3), prog4.SolveCount)
	assert.Equal(t, int32(2), prog4.IndependentSolveCount)

	// Verify all 4 attempts are preserved in chronological history (none overwritten)
	attempts, err := repo.ListQuestionAttempts(ctx, userID, questionID)
	require.NoError(t, err)
	assert.Equal(t, 4, len(attempts))
}

func TestService_CrossRoadmapProgressSharing(t *testing.T) {
	repo := NewInMemoryRepository()
	svc := NewService(repo)

	userID := uuid.New()
	questionID := uuid.New()
	roadmapA := uuid.New()
	roadmapB := uuid.New()

	ctx := context.Background()

	// Attempt recorded from Roadmap A
	_, progA, _, err := svc.RecordAttempt(ctx, RecordAttemptRequest{
		UserID:        userID,
		QuestionID:    questionID,
		RoadmapID:     &roadmapA,
		Outcome:       OutcomeSolvedIndependently,
		TimeSpentSecs: 900,
		QualityRating: 4,
	})
	require.NoError(t, err)
	assert.Equal(t, StatusSolved, progA.Status)

	// Check progress on the question directly: it reflects StatusSolved regardless of roadmap
	canonicalProgress, err := svc.GetQuestionProgress(ctx, userID, questionID)
	require.NoError(t, err)
	assert.Equal(t, StatusSolved, canonicalProgress.Status)
	assert.Equal(t, int32(1), canonicalProgress.SolveCount)

	// Attempt recorded from Roadmap B on the same question
	_, progB, _, err := svc.RecordAttempt(ctx, RecordAttemptRequest{
		UserID:        userID,
		QuestionID:    questionID,
		RoadmapID:     &roadmapB,
		Outcome:       OutcomeSolvedIndependently,
		TimeSpentSecs: 700,
		QualityRating: 5,
	})
	require.NoError(t, err)
	// Now user has 2 independent solves -> status is MASTERED across all roadmaps!
	assert.Equal(t, StatusMastered, progB.Status)
	assert.Equal(t, int32(2), progB.SolveCount)

	// Progress query proves single canonical row
	finalProgress, err := svc.GetQuestionProgress(ctx, userID, questionID)
	require.NoError(t, err)
	assert.Equal(t, StatusMastered, finalProgress.Status)
}

func TestService_UserIsolation(t *testing.T) {
	repo := NewInMemoryRepository()
	svc := NewService(repo)

	userAlice := uuid.New()
	userBob := uuid.New()
	questionID := uuid.New()

	ctx := context.Background()

	// Alice solves the question
	_, _, _, err := svc.RecordAttempt(ctx, RecordAttemptRequest{
		UserID:        userAlice,
		QuestionID:    questionID,
		Outcome:       OutcomeSolvedIndependently,
		TimeSpentSecs: 600,
		QualityRating: 5,
	})
	require.NoError(t, err)

	// Alice has progress
	aliceProg, err := svc.GetQuestionProgress(ctx, userAlice, questionID)
	require.NoError(t, err)
	assert.Equal(t, StatusSolved, aliceProg.Status)

	// Bob does NOT have progress
	bobProg, err := svc.GetQuestionProgress(ctx, userBob, questionID)
	assert.ErrorIs(t, err, ErrProgressNotFound)
	assert.Nil(t, bobProg)
}

func TestService_DueRevisions(t *testing.T) {
	repo := NewInMemoryRepository()
	svc := NewService(repo)

	userID := uuid.New()
	q1 := uuid.New()
	q2 := uuid.New()

	ctx := context.Background()

	// Solved with low score -> due in 1 day
	_, _, _, err := svc.RecordAttempt(ctx, RecordAttemptRequest{
		UserID:        userID,
		QuestionID:    q1,
		Outcome:       OutcomeSolvedAfterEditorial,
		TimeSpentSecs: 1800,
		QualityRating: 2,
	})
	require.NoError(t, err)

	// Solved with high score -> due later
	_, _, _, err = svc.RecordAttempt(ctx, RecordAttemptRequest{
		UserID:        userID,
		QuestionID:    q2,
		Outcome:       OutcomeSolvedIndependently,
		TimeSpentSecs: 600,
		QualityRating: 5,
	})
	require.NoError(t, err)

	// Due revisions 2 days in the future should include q1 and q2
	due, total, err := repo.ListDueRevisions(ctx, userID, time.Now().UTC().Add(48*time.Hour), 10, 0)
	require.NoError(t, err)
	assert.Equal(t, int64(2), total)
	assert.Equal(t, 2, len(due))
}

func TestService_ConcurrentAttempts_NoLostUpdates(t *testing.T) {
	repo := NewInMemoryRepository()
	svc := NewService(repo)

	userID := uuid.New()
	questionID := uuid.New()
	ctx := context.Background()

	const concurrency = 20
	errChan := make(chan error, concurrency)

	for i := 0; i < concurrency; i++ {
		go func(idx int) {
			_, _, _, err := svc.RecordAttempt(ctx, RecordAttemptRequest{
				UserID:                userID,
				QuestionID:            questionID,
				Outcome:               OutcomeSolvedIndependently,
				TimeSpentSecs:         100,
				DifficultyExperienced: PerceivedJustRight,
				QualityRating:         4,
			})
			errChan <- err
		}(i)
	}

	for i := 0; i < concurrency; i++ {
		err := <-errChan
		require.NoError(t, err)
	}

	// 1. Exactly 20 attempts preserved in append-only history
	attempts, err := repo.ListQuestionAttempts(ctx, userID, questionID)
	require.NoError(t, err)
	assert.Equal(t, concurrency, len(attempts))

	// 2. Canonical progress reflects all 20 attempts without lost updates
	progress, err := svc.GetQuestionProgress(ctx, userID, questionID)
	require.NoError(t, err)
	assert.Equal(t, StatusMastered, progress.Status)
	assert.Equal(t, int32(concurrency), progress.SolveCount)
	assert.Equal(t, int32(concurrency), progress.IndependentSolveCount)
	assert.Equal(t, int32(concurrency*100), progress.TotalTimeSpentSecs)

	// 3. Revision schedule exists and repetition count was incremented
	schedule, err := repo.GetRevisionSchedule(ctx, userID, questionID)
	require.NoError(t, err)
	require.NotNil(t, schedule)
	assert.GreaterOrEqual(t, schedule.RepetitionCount, int32(1))
}

func TestService_ExportUserData(t *testing.T) {
	repo := NewInMemoryRepository()
	svc := NewService(repo)

	userID := uuid.New()
	q1 := uuid.New()
	q2 := uuid.New()
	ctx := context.Background()

	// Record an attempt for q1
	_, _, _, err := svc.RecordAttempt(ctx, RecordAttemptRequest{
		UserID:                userID,
		QuestionID:            q1,
		Outcome:               OutcomeSolvedIndependently,
		TimeSpentSecs:         1200,
		DifficultyExperienced: PerceivedJustRight,
		QualityRating:         4,
		MistakeTags:           []string{"off_by_one"},
	})
	require.NoError(t, err)

	// Record an attempt for q2
	_, _, _, err = svc.RecordAttempt(ctx, RecordAttemptRequest{
		UserID:                userID,
		QuestionID:            q2,
		Outcome:               OutcomeSolvedWithHints,
		TimeSpentSecs:         1800,
		DifficultyExperienced: PerceivedChallenging,
		QualityRating:         3,
		MistakeTags:           []string{"edge_case"},
	})
	require.NoError(t, err)

	// Export
	export, err := svc.ExportUserData(ctx, userID)
	require.NoError(t, err)
	require.NotNil(t, export)
	assert.Equal(t, userID, export.UserID)
	assert.Equal(t, 2, len(export.Attempts))
	assert.Equal(t, 2, len(export.Progress))
	assert.Equal(t, 2, len(export.Revisions))
	assert.False(t, export.ExportedAt.IsZero())
}
