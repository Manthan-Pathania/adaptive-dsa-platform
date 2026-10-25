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
