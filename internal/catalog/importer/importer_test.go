package importer

import (
	"bytes"
	"context"
	"testing"

	"github.com/adaptive-dsa/platform/internal/catalog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xuri/excelize/v2"
)

func createTestExcel(t *testing.T, headers []string, data [][]string) *bytes.Buffer {
	t.Helper()
	f := excelize.NewFile()
	sheet := "Sheet1"

	for colIdx, h := range headers {
		cell, err := excelize.CoordinatesToCellName(colIdx+1, 1)
		require.NoError(t, err)
		err = f.SetCellValue(sheet, cell, h)
		require.NoError(t, err)
	}

	for rowIdx, row := range data {
		for colIdx, val := range row {
			cell, err := excelize.CoordinatesToCellName(colIdx+1, rowIdx+2)
			require.NoError(t, err)
			err = f.SetCellValue(sheet, cell, val)
			require.NoError(t, err)
		}
	}

	buf := new(bytes.Buffer)
	err := f.Write(buf)
	require.NoError(t, err)
	return buf
}

func standardHeaders() []string {
	return []string{
		"Title",
		"External URL",
		"Difficulty",
		"Topic",
		"Primary Pattern",
		"Subpattern",
		"Core",
		"Company Tags",
		"Editorial Summary",
		"Time Limit",
	}
}

func TestImporter_DryRun(t *testing.T) {
	repo := catalog.NewInMemoryRepository()
	imp := New(repo)

	data := [][]string{
		{
			"Two Sum",
			"https://leetcode.com/problems/two-sum/",
			"EASY",
			"Arrays & Hashing",
			"Hash Map",
			"Direct Lookup",
			"Core",
			"Google, Meta",
			"Use a hash map to lookup complement in O(1) time.",
			"15",
		},
		{
			"3Sum",
			"https://leetcode.com/problems/3sum/",
			"MEDIUM",
			"Two Pointers",
			"Opposite Direction",
			"",
			"Core",
			"Amazon, Apple",
			"Sort array and use two pointers for each fixed number.",
			"25",
		},
	}

	excelBuf := createTestExcel(t, standardHeaders(), data)

	opts := ImportOptions{
		Reader:      excelBuf,
		DryRun:      true,
		Environment: "development",
	}

	res, err := imp.Execute(context.Background(), opts)
	require.NoError(t, err)
	assert.True(t, res.IsDryRun)
	assert.Equal(t, 2, res.TotalRowsRead)
	assert.Equal(t, 2, res.ValidRows)
	assert.Equal(t, 0, res.InvalidRows)
	assert.Equal(t, 2, res.CreatedQuestions)

	// In dry run, database must NOT be mutated
	questions, total, err := repo.ListQuestions(context.Background(), catalog.ListQuestionsFilter{Limit: 10})
	require.NoError(t, err)
	assert.Equal(t, int64(0), total)
	assert.Empty(t, questions)
}

func TestImporter_AtomicCommit_Success(t *testing.T) {
	repo := catalog.NewInMemoryRepository()
	imp := New(repo)

	data := [][]string{
		{
			"Two Sum",
			"https://leetcode.com/problems/two-sum/",
			"EASY",
			"Arrays & Hashing",
			"Hash Map",
			"Direct Lookup",
			"Core",
			"Google, Meta",
			"Use a hash map to lookup complement.",
			"15",
		},
		{
			"Valid Anagram",
			"https://leetcode.com/problems/valid-anagram/",
			"EASY",
			"Arrays & Hashing",
			"Hash Map",
			"",
			"Core",
			"Uber",
			"Count frequency of each character.",
			"15",
		},
	}

	excelBuf := createTestExcel(t, standardHeaders(), data)

	opts := ImportOptions{
		Reader:      excelBuf,
		DryRun:      false,
		Environment: "development",
	}

	res, err := imp.Execute(context.Background(), opts)
	require.NoError(t, err)
	assert.False(t, res.IsDryRun)
	assert.Equal(t, 2, res.ValidRows)
	assert.Equal(t, 2, res.CreatedQuestions)
	assert.Equal(t, 1, res.CreatedTopics)

	// Verify questions now exist in repository
	q1, err := repo.GetQuestionBySlug(context.Background(), "two-sum")
	require.NoError(t, err)
	assert.Equal(t, "Two Sum", q1.Title)
	assert.Equal(t, catalog.DifficultyEasy, q1.Difficulty)
	assert.True(t, q1.IsCore)

	q2, err := repo.GetQuestionBySlug(context.Background(), "valid-anagram")
	require.NoError(t, err)
	assert.Equal(t, "Valid Anagram", q2.Title)
}

func TestImporter_IdempotentRepeatedImports(t *testing.T) {
	repo := catalog.NewInMemoryRepository()
	imp := New(repo)

	data := [][]string{
		{
			"Two Sum",
			"https://leetcode.com/problems/two-sum/",
			"EASY",
			"Arrays & Hashing",
			"Hash Map",
			"",
			"Core",
			"Google",
			"Summary v1",
			"20",
		},
	}

	// First Import
	buf1 := createTestExcel(t, standardHeaders(), data)
	res1, err := imp.Execute(context.Background(), ImportOptions{
		Reader:      buf1,
		DryRun:      false,
		Environment: "development",
	})
	require.NoError(t, err)
	assert.Equal(t, 1, res1.CreatedQuestions)
	assert.Equal(t, 0, res1.UpdatedQuestions)

	// Second Import of the exact same spreadsheet: must be idempotent
	buf2 := createTestExcel(t, standardHeaders(), data)
	res2, err := imp.Execute(context.Background(), ImportOptions{
		Reader:      buf2,
		DryRun:      false,
		Environment: "development",
	})
	require.NoError(t, err)
	assert.Equal(t, 0, res2.CreatedQuestions)
	assert.Equal(t, 1, res2.UnchangedQuestions)

	// Third Import with updated title: updates without creating duplicate
	data[0][0] = "Two Sum (Updated)"
	buf3 := createTestExcel(t, standardHeaders(), data)
	res3, err := imp.Execute(context.Background(), ImportOptions{
		Reader:      buf3,
		DryRun:      false,
		Environment: "development",
	})
	require.NoError(t, err)
	assert.Equal(t, 0, res3.CreatedQuestions)
	assert.Equal(t, 1, res3.UpdatedQuestions)

	// Only 1 question exists in total
	_, total, err := repo.ListQuestions(context.Background(), catalog.ListQuestionsFilter{Limit: 10})
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)

	q, err := repo.GetQuestionByExternalURL(context.Background(), "https://leetcode.com/problems/two-sum")
	require.NoError(t, err)
	assert.Equal(t, "Two Sum (Updated)", q.Title)
}

func TestImporter_RowValidationFailures(t *testing.T) {
	repo := catalog.NewInMemoryRepository()
	imp := New(repo)

	data := [][]string{
		{
			"", // Empty Title
			"https://leetcode.com/problems/two-sum/",
			"EASY",
			"Arrays",
			"Hash Map",
			"", "Core", "", "", "15",
		},
		{
			"Invalid Difficulty Problem",
			"https://leetcode.com/problems/valid-anagram/",
			"SUPER_HARD", // Invalid Difficulty
			"Arrays",
			"Hash Map",
			"", "Core", "", "", "15",
		},
		{
			"Invalid URL Problem",
			"not-a-valid-url:bad:path", // Invalid URL
			"MEDIUM",
			"Arrays",
			"Hash Map",
			"", "Core", "", "", "15",
		},
		{
			"Duplicate URL Problem",
			"https://leetcode.com/problems/two-sum/", // Duplicate URL (matches row 2)
			"MEDIUM",
			"Arrays",
			"Hash Map",
			"", "Core", "", "", "15",
		},
	}

	excelBuf := createTestExcel(t, standardHeaders(), data)

	res, err := imp.Execute(context.Background(), ImportOptions{
		Reader:      excelBuf,
		DryRun:      false,
		Environment: "development",
	})

	assert.Error(t, err)
	require.NotNil(t, res)
	assert.Equal(t, 4, res.InvalidRows)
	assert.True(t, len(res.RowErrors) >= 4)

	// No data committed
	_, total, err := repo.ListQuestions(context.Background(), catalog.ListQuestionsFilter{Limit: 10})
	require.NoError(t, err)
	assert.Equal(t, int64(0), total)
}

func TestImporter_MissingHeaders_Error(t *testing.T) {
	repo := catalog.NewInMemoryRepository()
	imp := New(repo)

	badHeaders := []string{"Some Column", "Another Column"}
	excelBuf := createTestExcel(t, badHeaders, [][]string{{"val1", "val2"}})

	_, err := imp.Execute(context.Background(), ImportOptions{
		Reader:      excelBuf,
		DryRun:      false,
		Environment: "development",
	})

	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrMissingHeaders)
}

func TestImporter_ProductionSafetyGuard(t *testing.T) {
	repo := catalog.NewInMemoryRepository()
	imp := New(repo)

	excelBuf := createTestExcel(t, standardHeaders(), [][]string{
		{"Two Sum", "https://leetcode.com/problems/two-sum/", "EASY", "Arrays", "Hash Map", "", "Core", "", "", "15"},
	})

	// Run in production without confirm flag
	_, err := imp.Execute(context.Background(), ImportOptions{
		Reader:      excelBuf,
		DryRun:      false,
		Environment: "production",
		ConfirmProd: false,
	})

	assert.ErrorIs(t, err, ErrProductionSafety)

	// Run in production WITH confirm flag
	excelBuf2 := createTestExcel(t, standardHeaders(), [][]string{
		{"Two Sum", "https://leetcode.com/problems/two-sum/", "EASY", "Arrays", "Hash Map", "", "Core", "", "", "15"},
	})
	res, err := imp.Execute(context.Background(), ImportOptions{
		Reader:      excelBuf2,
		DryRun:      false,
		Environment: "production",
		ConfirmProd: true,
	})

	require.NoError(t, err)
	assert.Equal(t, 1, res.CreatedQuestions)
}

func TestImporter_MasterPlanFormat_PreviewAndExecute(t *testing.T) {
	repo := catalog.NewInMemoryRepository()
	imp := New(repo)

	// User's master plan headers
	headers := []string{"ID", "Pattern", "Subpattern", "Difficulty", "Problem", "LeetCode ID", "Progress", "Notes"}
	data := [][]string{
		{"1", "Arrays & Hashing", "Hash maps", "Easy", "Two Sum", "1", "Done", "Use map for complement"},
		{"2", "Arrays & Hashing", "In-place modification", "Easy", "Move Zeroes", "283", "Attempted", "Swap non-zeros forward"},
		{"3", "Two Pointers & Sliding Window", "Two pointers", "Easy", "Move Zeroes", "283", "Done", "Cross-listed under two pointers"},
		{"4", "Dynamic Programming: 1D", "Tabulation", "Medium", "House Robber", "198", "Not Started", "dp[i] = max(dp[i-1], dp[i-2]+nums[i])"},
	}

	excelBuf := createTestExcel(t, headers, data)

	// 1. Preview
	preview, err := imp.Preview(context.Background(), ImportOptions{
		Reader: bytes.NewReader(excelBuf.Bytes()),
	})
	require.NoError(t, err)
	assert.Equal(t, 4, preview.TotalRowsRead)
	assert.Equal(t, 4, preview.ValidRows)
	assert.Equal(t, 0, preview.InvalidRows)
	assert.Equal(t, 3, preview.UniqueQuestions)
	assert.Equal(t, 1, preview.CrossListedRows)
	assert.Equal(t, 3, preview.PlannedCreates)
	assert.Equal(t, 4, len(preview.SampleRows))

	// 2. Execute
	res, err := imp.Execute(context.Background(), ImportOptions{
		Reader:      bytes.NewReader(excelBuf.Bytes()),
		DryRun:      false,
		Environment: "development",
	})
	require.NoError(t, err)
	assert.Equal(t, 4, res.TotalRowsRead)
	assert.Equal(t, 4, res.ValidRows)
	assert.Equal(t, 3, res.CreatedQuestions)
	assert.Equal(t, 1, res.CrossListedRows)

	// 3. Verify canonical URL and pattern cross-listing
	moveZeroes, err := repo.GetQuestionBySlug(context.Background(), "move-zeroes")
	require.NoError(t, err)
	assert.Equal(t, "Move Zeroes", moveZeroes.Title)
	assert.Equal(t, catalog.DifficultyEasy, moveZeroes.Difficulty)
	assert.Equal(t, "https://leetcode.com/problems/move-zeroes/", moveZeroes.ExternalURL)
	require.NotNil(t, moveZeroes.LeetCodeID)
	assert.Equal(t, "283", *moveZeroes.LeetCodeID)

	patterns, err := repo.GetQuestionPatterns(context.Background(), moveZeroes.ID)
	require.NoError(t, err)
	// Both "Arrays & Hashing" and "Two Pointers & Sliding Window" (and their subpatterns) linked!
	assert.True(t, len(patterns) >= 2)
}
