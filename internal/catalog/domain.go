package catalog

import (
	"errors"
	"fmt"
	"io"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
)

var (
	ErrQuestionNotFound           = errors.New("question not found")
	ErrTopicNotFound              = errors.New("topic not found")
	ErrPatternNotFound            = errors.New("pattern not found")
	ErrRoadmapNotFound            = errors.New("roadmap not found")
	ErrInvalidInput               = errors.New("invalid input data")
	ErrCannotDeleteSystemTemplate = errors.New("cannot delete system template roadmap")
	ErrUnauthorizedRoadmapAction  = errors.New("unauthorized roadmap action")
)

type Difficulty string

const (
	DifficultyEasy   Difficulty = "EASY"
	DifficultyMedium Difficulty = "MEDIUM"
	DifficultyHard   Difficulty = "HARD"
)

func (d Difficulty) IsValid() bool {
	switch d {
	case DifficultyEasy, DifficultyMedium, DifficultyHard:
		return true
	default:
		return false
	}
}

func ParseDifficulty(s string) (Difficulty, error) {
	upper := strings.ToUpper(strings.TrimSpace(s))
	switch upper {
	case "EASY":
		return DifficultyEasy, nil
	case "MED", "MEDIUM":
		return DifficultyMedium, nil
	case "HARD":
		return DifficultyHard, nil
	default:
		return "", errors.New("invalid difficulty: must be EASY, MEDIUM, or HARD")
	}
}

type Topic struct {
	ID           uuid.UUID `json:"id"`
	Name         string    `json:"name"`
	Slug         string    `json:"slug"`
	Description  *string   `json:"description,omitempty"`
	DisplayOrder int32     `json:"display_order"`
	CreatedAt    time.Time `json:"created_at"`
}

type Pattern struct {
	ID           uuid.UUID  `json:"id"`
	TopicID      *uuid.UUID `json:"topic_id,omitempty"`
	ParentID     *uuid.UUID `json:"parent_id,omitempty"`
	Name         string     `json:"name"`
	Slug         string     `json:"slug"`
	Description  *string    `json:"description,omitempty"`
	DisplayOrder int32      `json:"display_order"`
	CreatedAt    time.Time  `json:"created_at"`
	TopicName    *string    `json:"topic_name,omitempty"`
	TopicSlug    *string    `json:"topic_slug,omitempty"`
}

type Question struct {
	ID                  uuid.UUID  `json:"id"`
	Slug                string     `json:"slug"`
	Title               string     `json:"title"`
	ExternalURL         string     `json:"external_url"`
	Difficulty          Difficulty `json:"difficulty"`
	IsCore              bool       `json:"is_core"`
	IsActive            bool       `json:"is_active"`
	LeetCodeID          *string    `json:"leetcode_id,omitempty"`
	EditorialSummary    *string    `json:"editorial_summary,omitempty"`
	SampleTimeLimitMins int32      `json:"sample_time_limit_mins"`
	ExcelOrder          int32      `json:"excel_order"`
	CreatedAt           time.Time  `json:"created_at"`
	UpdatedAt           time.Time  `json:"updated_at"`
	Patterns            []Pattern  `json:"patterns,omitempty"`
	Companies           []string   `json:"companies,omitempty"`
}

type PatternProgressItem struct {
	PatternID      uuid.UUID `json:"pattern_id"`
	PatternName    string    `json:"pattern_name"`
	PatternSlug    string    `json:"pattern_slug"`
	TopicName      string    `json:"topic_name"`
	TotalQuestions int32     `json:"total_questions"`
	SolvedCount    int32     `json:"solved_count"`
	Percent        float32   `json:"percent"`
}

type Roadmap struct {
	ID               uuid.UUID     `json:"id"`
	UserID           *uuid.UUID    `json:"user_id,omitempty"`
	Title            string        `json:"title"`
	Slug             string        `json:"slug"`
	Description      *string       `json:"description,omitempty"`
	TargetDays       int32         `json:"target_days"`
	IsSystemTemplate bool          `json:"is_system_template"`
	TotalQuestions   int32         `json:"total_questions"`
	CreatedAt        time.Time     `json:"created_at"`
	UpdatedAt        time.Time     `json:"updated_at"`
	Items            []RoadmapItem `json:"items,omitempty"`
}

type RoadmapItem struct {
	ID             uuid.UUID  `json:"id"`
	RoadmapID      uuid.UUID  `json:"roadmap_id"`
	QuestionID     uuid.UUID  `json:"question_id"`
	StageNumber    int32      `json:"stage_number"`
	OrderIndex     int32      `json:"order_index"`
	IsMandatory    bool       `json:"is_mandatory"`
	QuestionTitle  string     `json:"question_title"`
	QuestionSlug   string     `json:"question_slug"`
	Difficulty     Difficulty `json:"difficulty"`
	IsCore         bool       `json:"is_core"`
	ExcelOrder     int32      `json:"excel_order,omitempty"`
	PatternName    string     `json:"pattern_name,omitempty"`
	SubpatternName string     `json:"subpattern_name,omitempty"`
	ProgressStatus string     `json:"progress_status,omitempty"`
	SolveCount     int32      `json:"solve_count,omitempty"`
	LastSolvedAt   *time.Time `json:"last_solved_at,omitempty"`
}

type ImportRoadmapParams struct {
	Title       string
	Description *string
	TargetDays  int32
	ArrangeBy   string // "excel_order" or "pattern"
}

type ListQuestionsFilter struct {
	SearchQuery      string
	Difficulty       string
	TopicOrPatternID *uuid.UUID
	IsActive         *bool
	Limit            int32
	Offset           int32
}

type ImportHistory struct {
	ID                 uuid.UUID  `json:"id"`
	Filename           string     `json:"filename"`
	TotalRows          int32      `json:"total_rows"`
	ValidRows          int32      `json:"valid_rows"`
	CreatedQuestions   int32      `json:"created_questions"`
	UpdatedQuestions   int32      `json:"updated_questions"`
	UnchangedQuestions int32      `json:"unchanged_questions"`
	InvalidRows        int32      `json:"invalid_rows"`
	Status             string     `json:"status"`
	ImportedBy         *uuid.UUID `json:"imported_by,omitempty"`
	CreatedAt          time.Time  `json:"created_at"`
}

type CreateQuestionParams struct {
	Title               string      `json:"title"`
	Slug                string      `json:"slug,omitempty"`
	ExternalURL         string      `json:"external_url,omitempty"`
	Difficulty          Difficulty  `json:"difficulty"`
	IsCore              bool        `json:"is_core"`
	IsActive            bool        `json:"is_active"`
	LeetCodeID          *string     `json:"leetcode_id,omitempty"`
	EditorialSummary    *string     `json:"editorial_summary,omitempty"`
	SampleTimeLimitMins int32       `json:"sample_time_limit_mins,omitempty"`
	PatternIDs          []uuid.UUID `json:"pattern_ids,omitempty"`
}

type UpdateQuestionParams struct {
	Title               string      `json:"title"`
	Difficulty          Difficulty  `json:"difficulty"`
	IsCore              bool        `json:"is_core"`
	LeetCodeID          *string     `json:"leetcode_id,omitempty"`
	EditorialSummary    *string     `json:"editorial_summary,omitempty"`
	SampleTimeLimitMins int32       `json:"sample_time_limit_mins,omitempty"`
	PatternIDs          []uuid.UUID `json:"pattern_ids,omitempty"`
}

// RowError models a validation error pinpointed to a specific spreadsheet row.
type RowError struct {
	Row     int    `json:"row"`
	Column  string `json:"column"`
	Message string `json:"message"`
}

func (e RowError) Error() string {
	if e.Column != "" {
		return fmt.Sprintf("Row %d [%s]: %s", e.Row, e.Column, e.Message)
	}
	return fmt.Sprintf("Row %d: %s", e.Row, e.Message)
}

// SamplePreviewRow models a single sample row for the UI preview table.
type SamplePreviewRow struct {
	RowNumber   int        `json:"row_number"`
	Title       string     `json:"title"`
	Difficulty  Difficulty `json:"difficulty"`
	Pattern     string     `json:"pattern"`
	Subpattern  string     `json:"subpattern,omitempty"`
	LeetCodeID  string     `json:"leetcode_id,omitempty"`
	ExternalURL string     `json:"external_url"`
	Action      string     `json:"action"` // "CREATE", "UPDATE", "UNCHANGED", "CROSS_LIST"
}

// ImportPreviewResult provides a pre-import validation scorecard.
type ImportPreviewResult struct {
	DetectedSheet      string             `json:"detected_sheet"`
	TotalRowsRead      int                `json:"total_rows_read"`
	ValidRows          int                `json:"valid_rows"`
	InvalidRows        int                `json:"invalid_rows"`
	UniqueQuestions    int                `json:"unique_questions"`
	CrossListedRows    int                `json:"cross_listed_rows"`
	PlannedCreates     int                `json:"planned_creates"`
	PlannedUpdates     int                `json:"planned_updates"`
	PlannedUnchanged   int                `json:"planned_unchanged"`
	SampleRows         []SamplePreviewRow `json:"sample_rows"`
	RowErrors          []RowError         `json:"row_errors,omitempty"`
	Warnings           []string           `json:"warnings,omitempty"`
}

// ImportOptions configures the importer execution.
type ImportOptions struct {
	FilePath    string
	Reader      io.Reader
	DryRun      bool
	Environment string
	ConfirmProd bool
}

// ImportResult provides a comprehensive report of the import run.
type ImportResult struct {
	IsDryRun           bool       `json:"is_dry_run"`
	TotalRowsRead      int        `json:"total_rows_read"`
	ValidRows          int        `json:"valid_rows"`
	InvalidRows        int        `json:"invalid_rows"`
	CreatedQuestions   int        `json:"created_questions"`
	UpdatedQuestions   int        `json:"updated_questions"`
	UnchangedQuestions int        `json:"unchanged_questions"`
	CrossListedRows    int        `json:"cross_listed_rows"`
	CreatedTopics      int        `json:"created_topics"`
	CreatedPatterns    int        `json:"created_patterns"`
	CreatedCompanies   int        `json:"created_companies"`
	RowErrors          []RowError `json:"row_errors"`
	Warnings           []string   `json:"warnings"`
}

var nonAlphanumericRegex = regexp.MustCompile(`[^a-z0-9]+`)

// Slugify transforms a human-readable title into a clean URL-friendly slug.
func Slugify(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = nonAlphanumericRegex.ReplaceAllString(s, "-")
	return strings.Trim(s, "-")
}

// NormalizeExternalURL removes tracking UTM parameters, query fragments, and trailing slashes.
func NormalizeExternalURL(rawURL string) (string, error) {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return "", errors.New("external url cannot be empty")
	}

	parsed, err := url.Parse(rawURL)
	if err != nil {
		return "", err
	}

	if parsed.Scheme == "" {
		parsed.Scheme = "https"
	}
	parsed.Scheme = strings.ToLower(parsed.Scheme)
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", errors.New("invalid url scheme: must be http or https")
	}
	parsed.Host = strings.ToLower(parsed.Host)
	if parsed.Host == "" {
		return "", errors.New("url host cannot be empty")
	}

	// Clean tracking params
	q := parsed.Query()
	for k := range q {
		if strings.HasPrefix(strings.ToLower(k), "utm_") || strings.ToLower(k) == "ref" {
			q.Del(k)
		}
	}
	parsed.RawQuery = q.Encode()
	parsed.Fragment = ""

	cleaned := parsed.String()
	// Strip trailing slash from path if path is not root
	if len(parsed.Path) > 1 && strings.HasSuffix(cleaned, "/") {
		cleaned = strings.TrimSuffix(cleaned, "/")
	}

	return cleaned, nil
}
