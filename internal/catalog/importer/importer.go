package importer

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/adaptive-dsa/platform/internal/catalog"
	"github.com/google/uuid"
	"github.com/xuri/excelize/v2"
)

var (
	ErrEmptyFile        = errors.New("spreadsheet contains no sheets or data")
	ErrMissingHeaders   = errors.New("spreadsheet missing required column headers")
	ErrProductionSafety = errors.New("importing into production requires explicit confirmation (--confirm-prod)")
)

// Type aliases from catalog domain
type ImportOptions = catalog.ImportOptions

// Type aliases from catalog domain
type RowError = catalog.RowError
type SamplePreviewRow = catalog.SamplePreviewRow
type ImportPreviewResult = catalog.ImportPreviewResult
type ImportResult = catalog.ImportResult

// ParsedQuestionRow models the validated data of a single spreadsheet row.
type ParsedQuestionRow struct {
	RowNumber           int
	Title               string
	ExternalURL         string
	Slug                string
	Difficulty          catalog.Difficulty
	Topic               string
	PrimaryPattern      string
	Subpattern          string
	IsCore              bool
	LeetCodeID          string
	Companies           []string
	EditorialSummary    *string
	SampleTimeLimitMins int32
	IsCrossListed       bool
}

// Importer coordinates parsing, validation, and safe transactional persistence.
type Importer struct {
	repo catalog.Repository
}

// New creates a new Importer instance.
func New(repo catalog.Repository) *Importer {
	return &Importer{repo: repo}
}

func openWorkbook(opts ImportOptions) (*excelize.File, func(), error) {
	var f *excelize.File
	var err error

	if opts.Reader != nil {
		f, err = excelize.OpenReader(opts.Reader)
	} else if opts.FilePath != "" {
		f, err = excelize.OpenFile(opts.FilePath)
	} else {
		return nil, nil, errors.New("neither file path nor reader provided")
	}

	if err != nil {
		return nil, nil, fmt.Errorf("failed to open excel workbook: %w", err)
	}

	cleanup := func() { _ = f.Close() }
	return f, cleanup, nil
}

func findWorksheet(f *excelize.File) (string, error) {
	sheetList := f.GetSheetList()
	if len(sheetList) == 0 {
		return "", ErrEmptyFile
	}
	// Prefer known catalog sheets
	for _, name := range sheetList {
		lower := strings.ToLower(strings.TrimSpace(name))
		if lower == "master plan" || lower == "master_plan" || lower == "questions" || lower == "problems" || lower == "dsa plan" {
			return name, nil
		}
	}
	return sheetList[0], nil
}

type seenQuestionInfo struct {
	rowNumber int
	title     string
}

// Preview parses and validates the spreadsheet without making any database modifications.
func (imp *Importer) Preview(ctx context.Context, opts ImportOptions) (*ImportPreviewResult, error) {
	f, cleanup, err := openWorkbook(opts)
	if err != nil {
		return nil, err
	}
	defer cleanup()

	sheetName, err := findWorksheet(f)
	if err != nil {
		return nil, err
	}

	rows, err := f.GetRows(sheetName)
	if err != nil {
		return nil, fmt.Errorf("failed to read sheet %q: %w", sheetName, err)
	}

	if len(rows) < 2 {
		return nil, ErrEmptyFile
	}

	headerMap, err := parseHeaders(rows[0])
	if err != nil {
		return nil, fmt.Errorf("header validation: %w", err)
	}

	preview := &ImportPreviewResult{
		DetectedSheet: sheetName,
	}

	seenURLs := make(map[string]seenQuestionInfo)
	var parsedRows []ParsedQuestionRow

	for i := 1; i < len(rows); i++ {
		rowIdx := i + 1
		row := rows[i]

		if isEmptyRow(row) {
			continue
		}

		preview.TotalRowsRead++

		parsed, rowErrs := validateRow(rowIdx, row, headerMap)
		if len(rowErrs) > 0 {
			preview.InvalidRows++
			preview.RowErrors = append(preview.RowErrors, rowErrs...)
			if headerMap.URL >= 0 && headerMap.URL < len(row) {
				if cleanURL, urlErr := catalog.NormalizeExternalURL(row[headerMap.URL]); urlErr == nil {
					if _, exists := seenURLs[cleanURL]; !exists {
						seenURLs[cleanURL] = seenQuestionInfo{rowNumber: rowIdx, title: ""}
					}
				}
			}
			continue
		}

		// Check if URL was already seen
		if prev, exists := seenURLs[parsed.ExternalURL]; exists {
			// If titles differ, it's a conflicting URL error
			if !strings.EqualFold(prev.title, parsed.Title) {
				preview.InvalidRows++
				preview.RowErrors = append(preview.RowErrors, RowError{
					Row:     rowIdx,
					Column:  "External URL",
					Message: fmt.Sprintf("Duplicate URL in spreadsheet; previously defined on row %d", prev.rowNumber),
				})
				continue
			}
			// Same title under another pattern => Cross-listing
			parsed.IsCrossListed = true
			preview.CrossListedRows++
		} else {
			seenURLs[parsed.ExternalURL] = seenQuestionInfo{
				rowNumber: rowIdx,
				title:     parsed.Title,
			}
			preview.UniqueQuestions++
		}

		preview.ValidRows++
		parsedRows = append(parsedRows, *parsed)
	}

	// Inspect existing records in repository to forecast changes
	sampleCount := 0
	for _, pr := range parsedRows {
		action := "CREATE"
		if pr.IsCrossListed {
			action = "CROSS_LIST"
		} else {
			existing, getErr := imp.repo.GetQuestionByExternalURL(ctx, pr.ExternalURL)
			if getErr != nil {
				preview.PlannedCreates++
				action = "CREATE"
			} else {
				if existing.Title != pr.Title ||
					existing.Difficulty != pr.Difficulty ||
					existing.IsCore != pr.IsCore ||
					existing.SampleTimeLimitMins != pr.SampleTimeLimitMins {
					preview.PlannedUpdates++
					action = "UPDATE"
				} else {
					preview.PlannedUnchanged++
					action = "UNCHANGED"
				}
			}
		}

		if sampleCount < 20 {
			preview.SampleRows = append(preview.SampleRows, SamplePreviewRow{
				RowNumber:   pr.RowNumber,
				Title:       pr.Title,
				Difficulty:  pr.Difficulty,
				Pattern:     pr.PrimaryPattern,
				Subpattern:  pr.Subpattern,
				LeetCodeID:  pr.LeetCodeID,
				ExternalURL: pr.ExternalURL,
				Action:      action,
			})
			sampleCount++
		}
	}

	return preview, nil
}

// Execute runs the import process according to the provided options.
func (imp *Importer) Execute(ctx context.Context, opts ImportOptions) (*ImportResult, error) {
	// Guard against accidental production runs
	if strings.EqualFold(opts.Environment, "production") && !opts.ConfirmProd {
		return nil, ErrProductionSafety
	}

	f, cleanup, err := openWorkbook(opts)
	if err != nil {
		return nil, err
	}
	defer cleanup()

	sheetName, err := findWorksheet(f)
	if err != nil {
		return nil, err
	}

	rows, err := f.GetRows(sheetName)
	if err != nil {
		return nil, fmt.Errorf("failed to read sheet %q: %w", sheetName, err)
	}

	if len(rows) < 2 {
		return nil, ErrEmptyFile
	}

	headerMap, err := parseHeaders(rows[0])
	if err != nil {
		return nil, fmt.Errorf("header validation: %w", err)
	}

	result := &ImportResult{
		IsDryRun: opts.DryRun,
	}

	var parsedRows []ParsedQuestionRow
	seenURLs := make(map[string]seenQuestionInfo)

	// Validate all rows
	for i := 1; i < len(rows); i++ {
		rowIdx := i + 1 // 1-based spreadsheet row
		row := rows[i]

		// Skip completely empty trailing rows
		if isEmptyRow(row) {
			continue
		}

		result.TotalRowsRead++

		parsed, rowErrs := validateRow(rowIdx, row, headerMap)
		if len(rowErrs) > 0 {
			result.InvalidRows++
			result.RowErrors = append(result.RowErrors, rowErrs...)
			if headerMap.URL >= 0 && headerMap.URL < len(row) {
				if cleanURL, urlErr := catalog.NormalizeExternalURL(row[headerMap.URL]); urlErr == nil {
					if _, exists := seenURLs[cleanURL]; !exists {
						seenURLs[cleanURL] = seenQuestionInfo{rowNumber: rowIdx, title: ""}
					}
				}
			}
			continue
		}

		// Check for duplicate / cross-listed URLs within the same file
		if prev, exists := seenURLs[parsed.ExternalURL]; exists {
			// If titles differ, it is an invalid conflicting URL
			if !strings.EqualFold(prev.title, parsed.Title) {
				result.InvalidRows++
				result.RowErrors = append(result.RowErrors, RowError{
					Row:     rowIdx,
					Column:  "External URL",
					Message: fmt.Sprintf("Duplicate URL in spreadsheet; previously defined on row %d", prev.rowNumber),
				})
				continue
			}
			// Same title under different pattern => Cross-listing
			parsed.IsCrossListed = true
			result.CrossListedRows++
		} else {
			seenURLs[parsed.ExternalURL] = seenQuestionInfo{
				rowNumber: rowIdx,
				title:     parsed.Title,
			}
		}

		result.ValidRows++
		parsedRows = append(parsedRows, *parsed)
	}

	// In Dry-Run mode, inspect existing records to report proposed changes without mutating the database
	if opts.DryRun {
		for _, pr := range parsedRows {
			if pr.IsCrossListed {
				continue
			}
			existing, getErr := imp.repo.GetQuestionByExternalURL(ctx, pr.ExternalURL)
			if getErr != nil {
				result.CreatedQuestions++
			} else {
				// Check if any attributes changed
				if existing.Title != pr.Title ||
					existing.Difficulty != pr.Difficulty ||
					existing.IsCore != pr.IsCore ||
					existing.SampleTimeLimitMins != pr.SampleTimeLimitMins {
					result.UpdatedQuestions++
				} else {
					result.UnchangedQuestions++
				}
			}
		}
		return result, nil
	}

	// If there were fatal validation errors in the sheet, do not proceed with database commit
	if result.InvalidRows > 0 {
		return result, fmt.Errorf("import aborted: spreadsheet contains %d invalid rows", result.InvalidRows)
	}

	// Execute atomic transactional import
	txErr := imp.repo.ExecTx(ctx, func(txRepo catalog.Repository) error {
		topicCache := make(map[string]uuid.UUID)
		patternCache := make(map[string]uuid.UUID)
		companyCache := make(map[string]uuid.UUID)

		for _, pr := range parsedRows {
			// 1. Ensure Topic exists
			topicSlug := catalog.Slugify(pr.Topic)
			topicID, hasTopic := topicCache[topicSlug]
			if !hasTopic {
				t, err := txRepo.GetTopicByName(ctx, pr.Topic)
				if err != nil {
					createdTopic, err := txRepo.CreateTopic(ctx, &catalog.Topic{
						Name: pr.Topic,
						Slug: topicSlug,
					})
					if err != nil {
						return fmt.Errorf("row %d create topic %q: %w", pr.RowNumber, pr.Topic, err)
					}
					topicID = createdTopic.ID
					result.CreatedTopics++
				} else {
					topicID = t.ID
				}
				topicCache[topicSlug] = topicID
			}

			// 2. Ensure Primary Pattern exists
			patternSlug := catalog.Slugify(pr.PrimaryPattern)
			patternID, hasPattern := patternCache[patternSlug]
			if !hasPattern {
				p, err := txRepo.GetPatternByName(ctx, pr.PrimaryPattern)
				if err != nil {
					createdPattern, err := txRepo.CreatePattern(ctx, &catalog.Pattern{
						TopicID: &topicID,
						Name:    pr.PrimaryPattern,
						Slug:    patternSlug,
					})
					if err != nil {
						return fmt.Errorf("row %d create pattern %q: %w", pr.RowNumber, pr.PrimaryPattern, err)
					}
					patternID = createdPattern.ID
					result.CreatedPatterns++
				} else {
					patternID = p.ID
				}
				patternCache[patternSlug] = patternID
			}

			// 3. Ensure Subpattern exists (if specified)
			var subpatternID *uuid.UUID
			if pr.Subpattern != "" {
				subSlug := catalog.Slugify(pr.Subpattern)
				if sid, hasSub := patternCache[subSlug]; hasSub {
					subpatternID = &sid
				} else {
					subP, err := txRepo.GetPatternByName(ctx, pr.Subpattern)
					if err != nil {
						createdSub, err := txRepo.CreatePattern(ctx, &catalog.Pattern{
							TopicID:  &topicID,
							ParentID: &patternID,
							Name:     pr.Subpattern,
							Slug:     subSlug,
						})
						if err != nil {
							return fmt.Errorf("row %d create subpattern %q: %w", pr.RowNumber, pr.Subpattern, err)
						}
						subpatternID = &createdSub.ID
						result.CreatedPatterns++
					} else {
						subpatternID = &subP.ID
					}
					patternCache[subSlug] = *subpatternID
				}
			}

			// 4. Upsert Question (Canonical Identity preserved by external_url)
			var leetCodeID *string
			if pr.LeetCodeID != "" {
				leetCodeID = &pr.LeetCodeID
			}

			existing, getErr := txRepo.GetQuestionByExternalURL(ctx, pr.ExternalURL)
			upserted, err := txRepo.UpsertQuestion(ctx, &catalog.Question{
				Slug:                pr.Slug,
				Title:               pr.Title,
				ExternalURL:         pr.ExternalURL,
				Difficulty:          pr.Difficulty,
				IsCore:              pr.IsCore,
				IsActive:            true,
				LeetCodeID:          leetCodeID,
				EditorialSummary:    pr.EditorialSummary,
				SampleTimeLimitMins: pr.SampleTimeLimitMins,
			})
			if err != nil {
				return fmt.Errorf("row %d upsert question: %w", pr.RowNumber, err)
			}

			if !pr.IsCrossListed {
				if getErr != nil {
					result.CreatedQuestions++
				} else {
					if existing.Title != pr.Title || existing.Difficulty != pr.Difficulty || existing.IsCore != pr.IsCore {
						result.UpdatedQuestions++
					} else {
						result.UnchangedQuestions++
					}
				}
			}

			// 5. Link Pattern
			// First occurrence gets primary flag; cross-listed occurrences link as secondary
			isPrimary := !pr.IsCrossListed
			if err := txRepo.LinkQuestionPattern(ctx, upserted.ID, patternID, isPrimary); err != nil {
				return fmt.Errorf("row %d link pattern: %w", pr.RowNumber, err)
			}

			// 6. Link Subpattern (if present)
			if subpatternID != nil {
				if err := txRepo.LinkQuestionPattern(ctx, upserted.ID, *subpatternID, false); err != nil {
					return fmt.Errorf("row %d link subpattern: %w", pr.RowNumber, err)
				}
			}

			// 7. Ensure Companies exist and link evidence
			for _, companyName := range pr.Companies {
				compSlug := catalog.Slugify(companyName)
				compID, hasComp := companyCache[compSlug]
				if !hasComp {
					var err error
					compID, err = txRepo.GetCompanyByName(ctx, companyName)
					if err != nil {
						compID, err = txRepo.CreateCompany(ctx, companyName, compSlug)
						if err != nil {
							return fmt.Errorf("row %d create company %q: %w", pr.RowNumber, companyName, err)
						}
						result.CreatedCompanies++
					}
					companyCache[compSlug] = compID
				}

				if err := txRepo.LinkQuestionCompany(ctx, upserted.ID, compID, "SPREADSHEET_IMPORT", "CONFIRMED"); err != nil {
					return fmt.Errorf("row %d link company %q: %w", pr.RowNumber, companyName, err)
				}
			}
		}

		return nil
	})

	if txErr != nil {
		return result, fmt.Errorf("import transaction failed: %w", txErr)
	}

	return result, nil
}

type headerIndices struct {
	Title          int
	URL            int
	Difficulty     int
	Topic          int
	PrimaryPattern int
	Subpattern     int
	Core           int
	Companies      int
	Editorial      int
	TimeLimit      int
	LeetCodeID     int
	Progress       int
}

func parseHeaders(headerRow []string) (*headerIndices, error) {
	h := &headerIndices{
		Title:          -1,
		URL:            -1,
		Difficulty:     -1,
		Topic:          -1,
		PrimaryPattern: -1,
		Subpattern:     -1,
		Core:           -1,
		Companies:      -1,
		Editorial:      -1,
		TimeLimit:      -1,
		LeetCodeID:     -1,
		Progress:       -1,
	}

	for idx, col := range headerRow {
		clean := strings.ToLower(strings.TrimSpace(col))
		switch clean {
		case "problem", "title", "problem title", "question title", "name", "problem name":
			h.Title = idx
		case "url", "external url", "problem url", "problem link", "link", "leetcode url":
			h.URL = idx
		case "difficulty", "level", "diff":
			h.Difficulty = idx
		case "topic", "category", "tag":
			h.Topic = idx
		case "pattern", "primary pattern", "core pattern":
			h.PrimaryPattern = idx
		case "subpattern", "variation", "secondary pattern":
			h.Subpattern = idx
		case "leetcode id", "leetcode_id", "lc id", "lc_id", "problem id", "id":
			h.LeetCodeID = idx
		case "notes", "editorial", "editorial summary", "editorial notes", "hints", "summary":
			h.Editorial = idx
		case "progress", "status":
			h.Progress = idx
		case "core", "is core", "core/optional", "type":
			h.Core = idx
		case "company", "companies", "target companies", "company tags":
			h.Companies = idx
		case "time limit", "time limit (mins)", "time (mins)", "time_mins":
			h.TimeLimit = idx
		}
	}

	var missing []string
	if h.Title == -1 {
		missing = append(missing, "Title / Problem")
	}
	if h.Difficulty == -1 {
		missing = append(missing, "Difficulty")
	}
	if h.PrimaryPattern == -1 {
		missing = append(missing, "Pattern / Primary Pattern")
	}

	if len(missing) > 0 {
		return nil, fmt.Errorf("%w: %s", ErrMissingHeaders, strings.Join(missing, ", "))
	}

	return h, nil
}

func validateRow(rowIdx int, row []string, h *headerIndices) (*ParsedQuestionRow, []RowError) {
	var errs []RowError

	getVal := func(idx int) string {
		if idx >= 0 && idx < len(row) {
			return strings.TrimSpace(row[idx])
		}
		return ""
	}

	title := getVal(h.Title)
	if title == "" {
		errs = append(errs, RowError{Row: rowIdx, Column: "Title", Message: "Title cannot be empty"})
	}

	slug := catalog.Slugify(title)

	// URL Handling: If URL column missing or empty, synthesize canonical LeetCode URL
	var cleanURL string
	rawURL := getVal(h.URL)
	if rawURL != "" {
		var urlErr error
		cleanURL, urlErr = catalog.NormalizeExternalURL(rawURL)
		if urlErr != nil {
			errs = append(errs, RowError{Row: rowIdx, Column: "External URL", Message: fmt.Sprintf("Invalid URL: %v", urlErr)})
		}
	} else if slug != "" {
		cleanURL = fmt.Sprintf("https://leetcode.com/problems/%s/", slug)
	}

	rawDiff := getVal(h.Difficulty)
	diff, diffErr := catalog.ParseDifficulty(rawDiff)
	if diffErr != nil {
		errs = append(errs, RowError{Row: rowIdx, Column: "Difficulty", Message: fmt.Sprintf("Invalid difficulty %q: must be EASY, MEDIUM, or HARD", rawDiff)})
	}

	pattern := getVal(h.PrimaryPattern)
	if pattern == "" {
		errs = append(errs, RowError{Row: rowIdx, Column: "Primary Pattern", Message: "Primary Pattern cannot be empty"})
	}

	// Topic Handling: If Topic column missing or empty, derive from Pattern
	topic := getVal(h.Topic)
	if topic == "" {
		topic = pattern
	}

	if len(errs) > 0 {
		return nil, errs
	}

	subpattern := getVal(h.Subpattern)

	// Parse Core flag
	rawCore := strings.ToLower(getVal(h.Core))
	isCore := true
	if rawCore == "optional" || rawCore == "false" || rawCore == "no" || rawCore == "0" {
		isCore = false
	}

	// Parse LeetCode ID
	leetCodeID := getVal(h.LeetCodeID)

	// Parse Companies
	var companies []string
	rawCompanies := getVal(h.Companies)
	if rawCompanies != "" {
		parts := strings.Split(rawCompanies, ",")
		for _, p := range parts {
			c := strings.TrimSpace(p)
			if c != "" {
				companies = append(companies, c)
			}
		}
	}

	// Parse Editorial / Notes
	var editorialSummary *string
	rawEditorial := getVal(h.Editorial)
	if rawEditorial != "" {
		editorialSummary = &rawEditorial
	}

	// Parse Time Limit
	var timeLimit int32 = 30
	rawTime := getVal(h.TimeLimit)
	if rawTime != "" {
		if parsedTime, err := strconv.Atoi(rawTime); err == nil && parsedTime > 0 {
			timeLimit = int32(parsedTime)
		}
	}

	return &ParsedQuestionRow{
		RowNumber:           rowIdx,
		Title:               title,
		ExternalURL:         cleanURL,
		Slug:                slug,
		Difficulty:          diff,
		Topic:               topic,
		PrimaryPattern:      pattern,
		Subpattern:          subpattern,
		IsCore:              isCore,
		LeetCodeID:          leetCodeID,
		Companies:           companies,
		EditorialSummary:    editorialSummary,
		SampleTimeLimitMins: timeLimit,
	}, nil
}

func isEmptyRow(row []string) bool {
	for _, cell := range row {
		if strings.TrimSpace(cell) != "" {
			return false
		}
	}
	return true
}
