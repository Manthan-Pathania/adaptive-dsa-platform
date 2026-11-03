package catalog

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/xuri/excelize/v2"
)

type roadmapHeaderIndices struct {
	ID         int
	Pattern    int
	Subpattern int
	Difficulty int
	Problem    int
	LeetCodeID int
}

func parseRoadmapHeaders(row []string) (*roadmapHeaderIndices, error) {
	h := &roadmapHeaderIndices{
		ID:         -1,
		Pattern:    -1,
		Subpattern: -1,
		Difficulty: -1,
		Problem:    -1,
		LeetCodeID: -1,
	}

	for idx, col := range row {
		clean := strings.ToLower(strings.TrimSpace(col))
		clean = strings.ReplaceAll(clean, "_", " ")
		switch clean {
		case "id", "s.no", "sno", "sr no", "sr.no", "no", "order", "index", "#":
			if h.ID == -1 {
				h.ID = idx
			}
		case "pattern", "core pattern", "primary pattern":
			if h.Pattern == -1 {
				h.Pattern = idx
			}
		case "subpattern", "sub pattern", "variation", "secondary pattern":
			if h.Subpattern == -1 {
				h.Subpattern = idx
			}
		case "difficulty", "diff", "level":
			if h.Difficulty == -1 {
				h.Difficulty = idx
			}
		case "problem", "title", "problem title", "question title", "question", "problem name", "name":
			if h.Problem == -1 {
				h.Problem = idx
			}
		case "leetcode id", "lc id", "leetcodeid", "lcid", "problem id":
			if h.LeetCodeID == -1 {
				h.LeetCodeID = idx
			}
		}
	}

	var missing []string
	if h.ID == -1 {
		missing = append(missing, "ID")
	}
	if h.Pattern == -1 {
		missing = append(missing, "Pattern")
	}
	if h.Subpattern == -1 {
		missing = append(missing, "Subpattern")
	}
	if h.Difficulty == -1 {
		missing = append(missing, "Difficulty")
	}
	if h.Problem == -1 {
		missing = append(missing, "Problem")
	}
	if h.LeetCodeID == -1 {
		missing = append(missing, "LeetCode ID")
	}

	if len(missing) > 0 {
		return nil, fmt.Errorf("missing required columns in Excel sheet: %s. Required columns: ID, Pattern, Subpattern, Difficulty, Problem, LeetCode ID", strings.Join(missing, ", "))
	}

	return h, nil
}

type parsedRoadmapRow struct {
	IDNum          int
	Pattern        string
	Subpattern     string
	Difficulty     Difficulty
	Problem        string
	LeetCodeID     string
	OriginalLineNo int
}

func parseDifficulty(raw string) Difficulty {
	clean := strings.ToUpper(strings.TrimSpace(raw))
	switch clean {
	case "EASY":
		return DifficultyEasy
	case "HARD":
		return DifficultyHard
	default:
		return DifficultyMedium
	}
}

// ImportRoadmapFromExcel parses an Excel workbook, validates required columns, matches/creates questions, and persists the custom roadmap.
func (s *service) ImportRoadmapFromExcel(ctx context.Context, userID uuid.UUID, params ImportRoadmapParams, reader io.Reader) (*Roadmap, error) {
	if reader == nil {
		return nil, errors.New("no excel file content provided")
	}

	title := strings.TrimSpace(params.Title)
	if title == "" {
		return nil, errors.New("roadmap title cannot be empty")
	}

	targetDays := params.TargetDays
	if targetDays <= 0 {
		targetDays = 30
	}

	f, err := excelize.OpenReader(reader)
	if err != nil {
		return nil, fmt.Errorf("failed to open excel workbook: %w", err)
	}
	defer f.Close()

	sheets := f.GetSheetList()
	if len(sheets) == 0 {
		return nil, errors.New("excel workbook contains no sheets")
	}

	// Pick target worksheet (prefer 'Master Plan', 'Roadmap', 'Plan', 'Questions', or first sheet)
	targetSheet := sheets[0]
	for _, name := range sheets {
		lower := strings.ToLower(strings.TrimSpace(name))
		if lower == "master plan" || lower == "roadmap" || lower == "plan" || lower == "questions" || lower == "problems" {
			targetSheet = name
			break
		}
	}

	rows, err := f.GetRows(targetSheet)
	if err != nil {
		return nil, fmt.Errorf("failed to read sheet %q: %w", targetSheet, err)
	}

	if len(rows) < 2 {
		return nil, errors.New("sheet must contain a header row and at least one problem row")
	}

	headerMap, err := parseRoadmapHeaders(rows[0])
	if err != nil {
		return nil, err
	}

	var parsedRows []parsedRoadmapRow
	for i := 1; i < len(rows); i++ {
		row := rows[i]
		lineNo := i + 1

		// Check if row is completely empty
		allEmpty := true
		for _, cell := range row {
			if strings.TrimSpace(cell) != "" {
				allEmpty = false
				break
			}
		}
		if allEmpty {
			continue
		}

		getCell := func(idx int) string {
			if idx >= 0 && idx < len(row) {
				return strings.TrimSpace(row[idx])
			}
			return ""
		}

		rawID := getCell(headerMap.ID)
		idNum, _ := strconv.Atoi(rawID)
		if idNum <= 0 {
			idNum = len(parsedRows) + 1
		}

		pattern := getCell(headerMap.Pattern)
		subpattern := getCell(headerMap.Subpattern)
		diff := parseDifficulty(getCell(headerMap.Difficulty))
		problem := getCell(headerMap.Problem)
		leetcodeID := getCell(headerMap.LeetCodeID)

		if problem == "" {
			continue
		}

		if pattern == "" {
			pattern = "General Algorithms"
		}

		parsedRows = append(parsedRows, parsedRoadmapRow{
			IDNum:          idNum,
			Pattern:        pattern,
			Subpattern:     subpattern,
			Difficulty:     diff,
			Problem:        problem,
			LeetCodeID:     leetcodeID,
			OriginalLineNo: lineNo,
		})
	}

	if len(parsedRows) == 0 {
		return nil, errors.New("no valid questions found in the spreadsheet")
	}

	// Ordering based on user preference
	arrangeBy := strings.ToLower(strings.TrimSpace(params.ArrangeBy))
	if arrangeBy == "pattern" {
		// Group and sort pattern-wise
		sort.SliceStable(parsedRows, func(i, j int) bool {
			if parsedRows[i].Pattern != parsedRows[j].Pattern {
				return parsedRows[i].Pattern < parsedRows[j].Pattern
			}
			if parsedRows[i].Subpattern != parsedRows[j].Subpattern {
				return parsedRows[i].Subpattern < parsedRows[j].Subpattern
			}
			return parsedRows[i].IDNum < parsedRows[j].IDNum
		})
	} else {
		// Excel order (preserve sequential row / ID order)
		sort.SliceStable(parsedRows, func(i, j int) bool {
			return parsedRows[i].IDNum < parsedRows[j].IDNum
		})
	}

	slug := fmt.Sprintf("%s-%d-%s", Slugify(title), time.Now().Unix(), uuid.New().String()[:6])
	desc := params.Description
	if desc == nil || *desc == "" {
		d := fmt.Sprintf("Imported curriculum comprising %d curated problems.", len(parsedRows))
		desc = &d
	}

	roadmap := &Roadmap{
		Title:            title,
		Slug:             slug,
		Description:      desc,
		TargetDays:       targetDays,
		UserID:           &userID,
		IsSystemTemplate: false,
	}

	var createdRoadmap *Roadmap
	var finalItems []RoadmapItem

	// Execute transactional match/creation and roadmap link
	txErr := s.repo.ExecTx(ctx, func(txRepo Repository) error {
		// 1. Ensure Default Topic exists
		defaultTopic, err := txRepo.GetTopicByName(ctx, "Algorithms")
		var topicID uuid.UUID
		if err != nil {
			ct, terr := txRepo.CreateTopic(ctx, &Topic{
				Name: "Algorithms",
				Slug: "algorithms",
			})
			if terr != nil {
				return fmt.Errorf("create topic: %w", terr)
			}
			topicID = ct.ID
		} else {
			topicID = defaultTopic.ID
		}

		patternCache := make(map[string]uuid.UUID)
		seenQuestionIDs := make(map[uuid.UUID]bool)

		orderIdx := int32(0)

		for _, row := range parsedRows {
			probSlug := Slugify(row.Problem)

			// Try to find existing question in database
			q, err := txRepo.FindQuestionForImport(ctx, row.Problem, row.LeetCodeID, probSlug)
			var questionID uuid.UUID
			if err == nil && q != nil {
				questionID = q.ID
			} else {
				// Create new question if it does not exist yet
				extURL := fmt.Sprintf("https://leetcode.com/problems/%s/", probSlug)
				var lcID *string
				if row.LeetCodeID != "" {
					lcID = &row.LeetCodeID
				}
				newQ := &Question{
					Slug:                probSlug,
					Title:               row.Problem,
					ExternalURL:         extURL,
					Difficulty:          row.Difficulty,
					IsCore:              false,
					IsActive:            true,
					LeetCodeID:          lcID,
					SampleTimeLimitMins: 30,
					ExcelOrder:          int32(row.IDNum),
				}
				createdQ, qErr := txRepo.UpsertQuestion(ctx, newQ)
				if qErr != nil {
					// In case of conflict, try finding again
					fQ, fErr := txRepo.FindQuestionForImport(ctx, row.Problem, row.LeetCodeID, probSlug)
					if fErr != nil {
						return fmt.Errorf("row %d create question %q: %w", row.OriginalLineNo, row.Problem, qErr)
					}
					questionID = fQ.ID
				} else {
					questionID = createdQ.ID
				}
			}

			// Ensure Pattern exists
			patSlug := Slugify(row.Pattern)
			patID, hasPat := patternCache[patSlug]
			if !hasPat {
				p, pErr := txRepo.GetPatternByName(ctx, row.Pattern)
				if pErr != nil {
					createdPat, cpErr := txRepo.CreatePattern(ctx, &Pattern{
						TopicID: &topicID,
						Name:    row.Pattern,
						Slug:    patSlug,
					})
					if cpErr != nil {
						return fmt.Errorf("row %d create pattern %q: %w", row.OriginalLineNo, row.Pattern, cpErr)
					}
					patID = createdPat.ID
				} else {
					patID = p.ID
				}
				patternCache[patSlug] = patID
			}

			// Link primary pattern
			_ = txRepo.LinkQuestionPattern(ctx, questionID, patID, true)

			// Ensure Subpattern exists (if present)
			if row.Subpattern != "" {
				subSlug := Slugify(row.Subpattern)
				subID, hasSub := patternCache[subSlug]
				if !hasSub {
					sp, spErr := txRepo.GetPatternByName(ctx, row.Subpattern)
					if spErr != nil {
						createdSub, cspErr := txRepo.CreatePattern(ctx, &Pattern{
							TopicID:  &topicID,
							ParentID: &patID,
							Name:     row.Subpattern,
							Slug:     subSlug,
						})
						if cspErr == nil {
							subID = createdSub.ID
						}
					} else {
						subID = sp.ID
					}
					if subID != uuid.Nil {
						patternCache[subSlug] = subID
					}
				}
				if subID != uuid.Nil {
					_ = txRepo.LinkQuestionPattern(ctx, questionID, subID, false)
				}
			}

			// Avoid duplicate question IDs in the same roadmap
			if seenQuestionIDs[questionID] {
				continue
			}
			seenQuestionIDs[questionID] = true

			finalItems = append(finalItems, RoadmapItem{
				QuestionID:     questionID,
				StageNumber:    1,
				OrderIndex:     orderIdx,
				IsMandatory:    true,
				QuestionTitle:  row.Problem,
				QuestionSlug:   probSlug,
				Difficulty:     row.Difficulty,
				ExcelOrder:     int32(row.IDNum),
				PatternName:    row.Pattern,
				SubpatternName: row.Subpattern,
			})
			orderIdx++
		}

		if len(finalItems) == 0 {
			return errors.New("no roadmap items could be assembled")
		}

		// 2. Create Roadmap record
		rd, err := txRepo.CreateRoadmap(ctx, roadmap)
		if err != nil {
			return fmt.Errorf("create roadmap: %w", err)
		}
		createdRoadmap = rd

		// 3. Insert Roadmap Items
		for _, it := range finalItems {
			if err := txRepo.AddRoadmapItem(ctx, createdRoadmap.ID, it.QuestionID, it.StageNumber, it.OrderIndex, it.IsMandatory); err != nil {
				return fmt.Errorf("add roadmap item: %w", err)
			}
		}

		return nil
	})

	if txErr != nil {
		return nil, txErr
	}

	createdRoadmap.Items = finalItems
	return createdRoadmap, nil
}

// GenerateRoadmapTemplate creates an Excel template with the mandatory 6 columns and sample rows.
func (s *service) GenerateRoadmapTemplate(ctx context.Context) ([]byte, error) {
	f := excelize.NewFile()
	defer f.Close()

	sheetName := "Roadmap Plan"
	f.SetSheetName("Sheet1", sheetName)

	headers := []string{"ID", "Pattern", "Subpattern", "Difficulty", "Problem", "LeetCode ID"}
	for colIdx, h := range headers {
		cell, _ := excelize.CoordinatesToCellName(colIdx+1, 1)
		_ = f.SetCellValue(sheetName, cell, h)
	}

	sampleRows := [][]interface{}{
		{1, "Arrays & Hashing", "Array traversal", "Easy", "Second Largest Element", ""},
		{2, "Arrays & Hashing", "Two-pointer swap", "Easy", "Move Zeroes", 283},
		{3, "Two Pointers", "Sorted two-sum", "Medium", "Two Sum II - Input Array Is Sorted", 167},
		{4, "Sliding Window", "Dynamic window", "Medium", "Longest Substring Without Repeating Characters", 3},
		{5, "Stack", "Matching pairs", "Easy", "Valid Parentheses", 20},
		{6, "Binary Search", "Standard bounds", "Easy", "Binary Search", 704},
		{7, "Linked List", "Fast & Slow pointers", "Easy", "Linked List Cycle", 141},
		{8, "Trees", "Depth-first search", "Easy", "Maximum Depth of Binary Tree", 104},
		{9, "Dynamic Programming", "1D state transition", "Easy", "Climbing Stairs", 70},
	}

	for rowIdx, r := range sampleRows {
		for colIdx, val := range r {
			cell, _ := excelize.CoordinatesToCellName(colIdx+1, rowIdx+2)
			_ = f.SetCellValue(sheetName, cell, val)
		}
	}

	// Header styling
	headerStyle, _ := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{
			Bold:  true,
			Color: "#FFFFFF",
		},
		Fill: excelize.Fill{
			Type:    "pattern",
			Color:   []string{"#0F172A"},
			Pattern: 1,
		},
		Alignment: &excelize.Alignment{
			Horizontal: "center",
			Vertical:   "center",
		},
	})
	_ = f.SetRowStyle(sheetName, 1, 1, headerStyle)

	// Set column widths
	_ = f.SetColWidth(sheetName, "A", "A", 8)
	_ = f.SetColWidth(sheetName, "B", "B", 25)
	_ = f.SetColWidth(sheetName, "C", "C", 25)
	_ = f.SetColWidth(sheetName, "D", "D", 14)
	_ = f.SetColWidth(sheetName, "E", "E", 40)
	_ = f.SetColWidth(sheetName, "F", "F", 16)

	var buf bytes.Buffer
	if err := f.Write(&buf); err != nil {
		return nil, fmt.Errorf("failed to write excel buffer: %w", err)
	}

	return buf.Bytes(), nil
}

