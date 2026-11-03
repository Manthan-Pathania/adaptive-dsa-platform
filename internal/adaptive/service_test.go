package adaptive

import (
	"context"
	"testing"
	"time"

	"github.com/adaptive-dsa/platform/internal/catalog"
	"github.com/adaptive-dsa/platform/internal/progress"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAdaptive_DeterministicRankingOrder(t *testing.T) {
	adaptiveRepo := NewInMemoryRepository()
	progressRepo := progress.NewInMemoryRepository()
	progressSvc := progress.NewService(progressRepo)
	catalogRepo := catalog.NewInMemoryRepository()
	catalogSvc := catalog.NewService(catalogRepo)

	adaptiveSvc := NewService(adaptiveRepo, progressSvc, catalogSvc)
	ctx := context.Background()
	userID := uuid.New()
	asOf := time.Date(2026, 10, 25, 10, 0, 0, 0, time.UTC)

	// 1. Setup Spaced Repetition Due Revision: "LRU Cache"
	lruID := uuid.New()
	_, _ = catalogRepo.UpsertQuestion(ctx, &catalog.Question{
		ID:          lruID,
		Title:       "LRU Cache",
		Slug:        "lru-cache",
		ExternalURL: "https://leetcode.com/problems/lru-cache",
		Difficulty:  catalog.DifficultyMedium,
		IsCore:      true,
	})
	// Schedule due 2 days ago
	_, err := progressRepo.UpsertRevisionSchedule(ctx, &progress.RevisionSchedule{
		UserID:          userID,
		QuestionID:      lruID,
		RepetitionCount: 2,
		EaseFactor:      2.50,
		IntervalDays:    6,
		NextReviewAt:    asOf.Add(-48 * time.Hour), // 2 days overdue
		QuestionTitle:   "LRU Cache",
		QuestionSlug:    "lru-cache",
	})
	require.NoError(t, err)

	// 2. Setup Weak Pattern: "Sliding Window" with 3 failed attempts
	slidingPatternID := uuid.New()
	adaptiveRepo.SetPatternMetrics(userID, []PatternMetric{
		{
			PatternID:        slidingPatternID,
			PatternName:      "Sliding Window",
			PatternSlug:      "sliding-window",
			TotalAttempts:    4,
			FailedAttempts:   3,
			AvgQualityRating: 1.5,
			IsWeak:           true,
		},
	})
	minWindowID := uuid.New()
	adaptiveRepo.SetPatternQuestions(slidingPatternID, []RecommendedQuestion{
		{
			QuestionID:  minWindowID,
			Title:       "Minimum Window Substring",
			Slug:        "minimum-window-substring",
			Difficulty:  "HARD",
			PatternName: "Sliding Window",
		},
	})

	// 3. Setup Roadmap: "Google 60-Day" with "Two Sum"
	twoSumID := uuid.New()
	_, _ = catalogRepo.UpsertQuestion(ctx, &catalog.Question{
		ID:          twoSumID,
		Title:       "Two Sum",
		Slug:        "two-sum",
		ExternalURL: "https://leetcode.com/problems/two-sum",
		Difficulty:  catalog.DifficultyEasy,
		IsCore:      true,
	})
	rmap, err := catalogRepo.CreateRoadmap(ctx, &catalog.Roadmap{
		Title:            "Google 60-Day",
		Slug:             "google-60-day",
		IsSystemTemplate: true,
	})
	require.NoError(t, err)
	_ = catalogRepo.AddRoadmapItem(ctx, rmap.ID, twoSumID, 1, 1, true)

	// 4. Setup Core Question: "Trapping Rain Water"
	trapID := uuid.New()
	_, _ = catalogRepo.UpsertQuestion(ctx, &catalog.Question{
		ID:          trapID,
		Title:       "Trapping Rain Water",
		Slug:        "trapping-rain-water",
		ExternalURL: "https://leetcode.com/problems/trapping-rain-water",
		Difficulty:  catalog.DifficultyHard,
		IsCore:      true,
	})

	// Generate Daily Plan
	plan, err := adaptiveSvc.GetDailyPlan(ctx, userID, &rmap.ID, asOf)
	require.NoError(t, err)

	require.NotEmpty(t, plan.Recommendations)
	assert.Equal(t, "2026-10-25", plan.Date)

	// Check Rank 1: Spaced Repetition Due Revision
	assert.Equal(t, CategoryDueRevision, plan.Recommendations[0].Category)
	assert.Equal(t, lruID, plan.Recommendations[0].QuestionID)
	assert.GreaterOrEqual(t, plan.Recommendations[0].PriorityScore, int32(1000))
	assert.Contains(t, plan.Recommendations[0].Rationale, "Overdue for spaced repetition")

	// Check Rank 2: Weak Pattern Reinforcement
	assert.Equal(t, CategoryWeakPattern, plan.Recommendations[1].Category)
	assert.Equal(t, minWindowID, plan.Recommendations[1].QuestionID)
	assert.GreaterOrEqual(t, plan.Recommendations[1].PriorityScore, int32(500))
	assert.Contains(t, plan.Recommendations[1].Rationale, "Sliding Window")

	// Check Rank 3: Roadmap Next Step
	assert.Equal(t, CategoryRoadmapNext, plan.Recommendations[2].Category)
	assert.Equal(t, twoSumID, plan.Recommendations[2].QuestionID)
	assert.GreaterOrEqual(t, plan.Recommendations[2].PriorityScore, int32(200))

	// Verify Determinism: repeated calls must produce identical rankings
	plan2, err := adaptiveSvc.GetDailyPlan(ctx, userID, &rmap.ID, asOf)
	require.NoError(t, err)
	require.Equal(t, len(plan.Recommendations), len(plan2.Recommendations))
	for i := range plan.Recommendations {
		assert.Equal(t, plan.Recommendations[i].QuestionID, plan2.Recommendations[i].QuestionID)
		assert.Equal(t, plan.Recommendations[i].PriorityScore, plan2.Recommendations[i].PriorityScore)
	}
}

func TestAdaptive_WeakPatternDetection(t *testing.T) {
	adaptiveRepo := NewInMemoryRepository()
	adaptiveSvc := NewService(adaptiveRepo, nil, nil)
	ctx := context.Background()
	userID := uuid.New()

	p1 := uuid.New()
	p2 := uuid.New()

	adaptiveRepo.SetPatternMetrics(userID, []PatternMetric{
		{
			PatternID:        p1,
			PatternName:      "Dynamic Programming",
			TotalAttempts:    5,
			FailedAttempts:   2,
			AvgQualityRating: 2.0,
			IsWeak:           true,
		},
		{
			PatternID:        p2,
			PatternName:      "Two Pointers",
			TotalAttempts:    6,
			FailedAttempts:   0,
			AvgQualityRating: 4.5,
			IsWeak:           false,
		},
	})

	weak, err := adaptiveSvc.GetWeakPatterns(ctx, userID)
	require.NoError(t, err)
	assert.Equal(t, 1, len(weak))
	assert.Equal(t, "Dynamic Programming", weak[0].PatternName)
}
