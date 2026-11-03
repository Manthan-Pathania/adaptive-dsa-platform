package catalog

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestService_ListQuestions_Filters(t *testing.T) {
	repo := NewInMemoryRepository()
	svc := NewService(repo)
	ctx := context.Background()

	// Seed questions
	q1, err := repo.UpsertQuestion(ctx, &Question{
		Title:       "Two Sum",
		Slug:        "two-sum",
		ExternalURL: "https://leetcode.com/problems/two-sum",
		Difficulty:  DifficultyEasy,
		IsCore:      true,
	})
	require.NoError(t, err)

	q2, err := repo.UpsertQuestion(ctx, &Question{
		Title:       "3Sum",
		Slug:        "3sum",
		ExternalURL: "https://leetcode.com/problems/3sum",
		Difficulty:  DifficultyMedium,
		IsCore:      true,
	})
	require.NoError(t, err)

	_, err = repo.UpsertQuestion(ctx, &Question{
		Title:       "Trapping Rain Water",
		Slug:        "trapping-rain-water",
		ExternalURL: "https://leetcode.com/problems/trapping-rain-water",
		Difficulty:  DifficultyHard,
		IsCore:      true,
	})
	require.NoError(t, err)

	// Filter by search query
	res, total, err := svc.ListQuestions(ctx, ListQuestionsFilter{SearchQuery: "sum"})
	require.NoError(t, err)
	assert.Equal(t, int64(2), total)
	assert.Equal(t, 2, len(res))

	// Filter by difficulty
	resDiff, totalDiff, err := svc.ListQuestions(ctx, ListQuestionsFilter{Difficulty: "HARD"})
	require.NoError(t, err)
	assert.Equal(t, int64(1), totalDiff)
	assert.Equal(t, "Trapping Rain Water", resDiff[0].Title)

	// Get by ID
	fetched, err := svc.GetQuestion(ctx, q1.ID.String())
	require.NoError(t, err)
	assert.Equal(t, "Two Sum", fetched.Title)

	// Get by Slug
	fetchedSlug, err := svc.GetQuestion(ctx, "3sum")
	require.NoError(t, err)
	assert.Equal(t, q2.ID, fetchedSlug.ID)
}

func TestService_RoadmapWithOrderedQuestions(t *testing.T) {
	repo := NewInMemoryRepository()
	svc := NewService(repo)
	ctx := context.Background()

	q1, _ := repo.UpsertQuestion(ctx, &Question{
		Title:       "Two Sum",
		Slug:        "two-sum",
		ExternalURL: "https://leetcode.com/problems/two-sum",
		Difficulty:  DifficultyEasy,
	})
	q2, _ := repo.UpsertQuestion(ctx, &Question{
		Title:       "Valid Palindrome",
		Slug:        "valid-palindrome",
		ExternalURL: "https://leetcode.com/problems/valid-palindrome",
		Difficulty:  DifficultyEasy,
	})

	rd, err := repo.CreateRoadmap(ctx, &Roadmap{
		Title:            "Blind 75 Curated",
		Slug:             "blind-75",
		IsSystemTemplate: true,
	})
	require.NoError(t, err)

	err = repo.AddRoadmapItem(ctx, rd.ID, q1.ID, 1, 1, true)
	require.NoError(t, err)
	err = repo.AddRoadmapItem(ctx, rd.ID, q2.ID, 1, 2, true)
	require.NoError(t, err)

	fetchedRd, err := svc.GetRoadmap(ctx, "blind-75")
	require.NoError(t, err)
	assert.Equal(t, "Blind 75 Curated", fetchedRd.Title)
	require.Equal(t, 2, len(fetchedRd.Items))
	assert.Equal(t, "Two Sum", fetchedRd.Items[0].QuestionTitle)
	assert.Equal(t, "Valid Palindrome", fetchedRd.Items[1].QuestionTitle)
}

func TestNormalizeExternalURL(t *testing.T) {
	cases := []struct {
		input    string
		expected string
		hasErr   bool
	}{
		{
			input:    "https://leetcode.com/problems/two-sum/",
			expected: "https://leetcode.com/problems/two-sum",
		},
		{
			input:    "https://leetcode.com/problems/two-sum/?utm_source=twitter&ref=share",
			expected: "https://leetcode.com/problems/two-sum",
		},
		{
			input:    "http://leetcode.com/problems/two-sum",
			expected: "http://leetcode.com/problems/two-sum",
		},
		{
			input:  "",
			hasErr: true,
		},
	}

	for _, tc := range cases {
		out, err := NormalizeExternalURL(tc.input)
		if tc.hasErr {
			assert.Error(t, err)
		} else {
			require.NoError(t, err)
			assert.Equal(t, tc.expected, out)
		}
	}
}

func TestService_AdminOperations(t *testing.T) {
	repo := NewInMemoryRepository()
	svc := NewService(repo)
	ctx := context.Background()

	// 1. Create Question
	created, err := svc.CreateQuestion(ctx, CreateQuestionParams{
		Title:      "Reverse Linked List",
		Difficulty: DifficultyEasy,
		IsCore:     true,
	})
	require.NoError(t, err)
	assert.Equal(t, "Reverse Linked List", created.Title)
	assert.True(t, created.IsActive)
	assert.Equal(t, "https://leetcode.com/problems/reverse-linked-list", created.ExternalURL)

	// 2. Update Question
	newTitle := "Reverse Linked List (Iterative & Recursive)"
	updated, err := svc.UpdateQuestion(ctx, created.ID, UpdateQuestionParams{
		Title:      newTitle,
		Difficulty: DifficultyMedium,
		IsCore:     false,
	})
	require.NoError(t, err)
	assert.Equal(t, newTitle, updated.Title)
	assert.Equal(t, DifficultyMedium, updated.Difficulty)

	// 3. Archive Question
	err = svc.SetQuestionActive(ctx, created.ID, false)
	require.NoError(t, err)

	archivedQ, err := svc.GetQuestion(ctx, created.ID.String())
	require.NoError(t, err)
	assert.False(t, archivedQ.IsActive)

	// Candidate list should NOT see archived question
	activeOnly := true
	candList, total, err := svc.ListQuestions(ctx, ListQuestionsFilter{IsActive: &activeOnly})
	require.NoError(t, err)
	assert.Equal(t, int64(0), total)
	assert.Empty(t, candList)

	// Admin list (nil filter) DOES see archived question
	adminList, totalAdmin, err := svc.ListQuestions(ctx, ListQuestionsFilter{IsActive: nil})
	require.NoError(t, err)
	assert.Equal(t, int64(1), totalAdmin)
	assert.Equal(t, 1, len(adminList))

	// 4. Restore Question
	err = svc.SetQuestionActive(ctx, created.ID, true)
	require.NoError(t, err)
	candListRestored, totalRestored, err := svc.ListQuestions(ctx, ListQuestionsFilter{IsActive: &activeOnly})
	require.NoError(t, err)
	assert.Equal(t, int64(1), totalRestored)
	assert.Equal(t, 1, len(candListRestored))
}
