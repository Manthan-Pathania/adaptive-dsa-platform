package catalog

import (
	"errors"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
)

var (
	ErrQuestionNotFound = errors.New("question not found")
	ErrTopicNotFound    = errors.New("topic not found")
	ErrPatternNotFound  = errors.New("pattern not found")
	ErrRoadmapNotFound  = errors.New("roadmap not found")
	ErrInvalidInput     = errors.New("invalid input data")
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
	EditorialSummary    *string    `json:"editorial_summary,omitempty"`
	SampleTimeLimitMins int32      `json:"sample_time_limit_mins"`
	CreatedAt           time.Time  `json:"created_at"`
	UpdatedAt           time.Time  `json:"updated_at"`
	Patterns            []Pattern  `json:"patterns,omitempty"`
	Companies           []string   `json:"companies,omitempty"`
}

type Roadmap struct {
	ID               uuid.UUID     `json:"id"`
	UserID           *uuid.UUID    `json:"user_id,omitempty"`
	Title            string        `json:"title"`
	Slug             string        `json:"slug"`
	Description      *string       `json:"description,omitempty"`
	TargetDays       int32         `json:"target_days"`
	IsSystemTemplate bool          `json:"is_system_template"`
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
	ProgressStatus string     `json:"progress_status,omitempty"`
	SolveCount     int32      `json:"solve_count,omitempty"`
	LastSolvedAt   *time.Time `json:"last_solved_at,omitempty"`
}

type ListQuestionsFilter struct {
	SearchQuery      string
	Difficulty       string
	TopicOrPatternID *uuid.UUID
	Limit            int32
	Offset           int32
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
