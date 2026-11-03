package catalog

import (
	"bytes"
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/xuri/excelize/v2"
)

func TestParseRoadmapHeaders(t *testing.T) {
	tests := []struct {
		name        string
		row         []string
		expectError bool
	}{
		{
			name:        "all required columns present in exact order",
			row:         []string{"ID", "Pattern", "Subpattern", "Difficulty", "Problem", "LeetCode ID"},
			expectError: false,
		},
		{
			name:        "all required columns present in different order",
			row:         []string{"Problem", "Difficulty", "Pattern", "LeetCode ID", "Subpattern", "ID"},
			expectError: false,
		},
		{
			name:        "missing leetcode id",
			row:         []string{"ID", "Pattern", "Subpattern", "Difficulty", "Problem"},
			expectError: true,
		},
		{
			name:        "missing difficulty",
			row:         []string{"ID", "Pattern", "Subpattern", "Problem", "LeetCode ID"},
			expectError: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			indices, err := parseRoadmapHeaders(tc.row)
			if tc.expectError {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
			} else {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if indices.ID < 0 || indices.Pattern < 0 || indices.Subpattern < 0 ||
					indices.Difficulty < 0 || indices.Problem < 0 || indices.LeetCodeID < 0 {
					t.Fatalf("expected all column indices >= 0, got %+v", indices)
				}
			}
		})
	}
}

func TestGenerateRoadmapTemplate(t *testing.T) {
	repo := NewInMemoryRepository()
	svc := NewService(repo)
	templateBytes, err := svc.GenerateRoadmapTemplate(context.Background())
	if err != nil {
		t.Fatalf("failed to generate roadmap template: %v", err)
	}

	f, err := excelize.OpenReader(bytes.NewReader(templateBytes))
	if err != nil {
		t.Fatalf("failed to open generated template: %v", err)
	}
	defer f.Close()

	rows, err := f.GetRows("Roadmap Plan")
	if err != nil {
		t.Fatalf("failed to read 'Roadmap Plan' sheet: %v", err)
	}

	if len(rows) < 2 {
		t.Fatalf("expected header + sample rows, got %d rows", len(rows))
	}

	headers := rows[0]
	indices, err := parseRoadmapHeaders(headers)
	if err != nil {
		t.Fatalf("generated template headers validation failed: %v", err)
	}

	if indices.ID < 0 || indices.Pattern < 0 || indices.Subpattern < 0 ||
		indices.Difficulty < 0 || indices.Problem < 0 || indices.LeetCodeID < 0 {
		t.Fatalf("template missing required columns: %+v", indices)
	}
}

func TestImportRoadmapFromExcel_InMemory(t *testing.T) {
	// 1. Build an Excel file in-memory
	f := excelize.NewFile()
	sheetName := "Sheet1"

	headers := []string{"ID", "Pattern", "Subpattern", "Difficulty", "Problem", "LeetCode ID"}
	for colIdx, h := range headers {
		cell, _ := excelize.CoordinatesToCellName(colIdx+1, 1)
		_ = f.SetCellValue(sheetName, cell, h)
	}

	testRows := [][]string{
		{"2", "Binary Search", "Find Minimum", "Medium", "Find Minimum in Rotated Sorted Array", "153"},
		{"1", "Arrays & Hashing", "Prefix Sum", "Easy", "Subarray Sum Equals K", "560"},
		{"3", "Arrays & Hashing", "Hash Map", "Easy", "Two Sum", "1"},
	}

	for rowIdx, row := range testRows {
		for colIdx, val := range row {
			cell, _ := excelize.CoordinatesToCellName(colIdx+1, rowIdx+2)
			_ = f.SetCellValue(sheetName, cell, val)
		}
	}

	var buf bytes.Buffer
	if err := f.Write(&buf); err != nil {
		t.Fatalf("failed to write excel buffer: %v", err)
	}

	// 2. Initialize InMemory service
	repo := NewInMemoryRepository()
	svc := NewService(repo)
	userID := uuid.New()

	// 3. Test import with ArrangeBy = "excel_order"
	params := ImportRoadmapParams{
		Title:       "Test Curriculum 1",
		TargetDays:  30,
		ArrangeBy:   "excel_order",
		Description: nil,
	}

	rm1, err := svc.ImportRoadmapFromExcel(context.Background(), userID, params, bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatalf("failed to import roadmap (excel_order): %v", err)
	}

	if rm1.Title != "Test Curriculum 1" {
		t.Errorf("expected title 'Test Curriculum 1', got %q", rm1.Title)
	}
	if len(rm1.Items) != 3 {
		t.Fatalf("expected 3 roadmap items, got %d", len(rm1.Items))
	}

	// First item by IDNum should be ID 1 ("Subarray Sum Equals K")
	if rm1.Items[0].QuestionTitle != "Subarray Sum Equals K" {
		t.Errorf("expected first item in excel_order to be 'Subarray Sum Equals K', got %q", rm1.Items[0].QuestionTitle)
	}

	// 4. Test import with ArrangeBy = "pattern"
	params2 := ImportRoadmapParams{
		Title:      "Test Curriculum 2",
		TargetDays: 45,
		ArrangeBy:  "pattern",
	}

	rm2, err := svc.ImportRoadmapFromExcel(context.Background(), userID, params2, bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatalf("failed to import roadmap (pattern): %v", err)
	}

	if len(rm2.Items) != 3 {
		t.Fatalf("expected 3 roadmap items, got %d", len(rm2.Items))
	}
	// "Arrays & Hashing" comes before "Binary Search" alphabetically
	if rm2.Items[0].QuestionTitle != "Two Sum" && rm2.Items[0].QuestionTitle != "Subarray Sum Equals K" {
		t.Errorf("expected first item to belong to Arrays & Hashing pattern, got %q", rm2.Items[0].QuestionTitle)
	}
}
