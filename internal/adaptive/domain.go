package adaptive

import (
	"time"

	"github.com/google/uuid"
)

type RecommendationCategory string

const (
	CategoryDueRevision RecommendationCategory = "DUE_REVISION"
	CategoryWeakPattern RecommendationCategory = "WEAK_PATTERN_REINFORCEMENT"
	CategoryRoadmapNext RecommendationCategory = "ROADMAP_NEXT_STEP"
	CategoryExploration RecommendationCategory = "CURATED_EXPLORATION"
)

type RecommendedQuestion struct {
	QuestionID    uuid.UUID              `json:"question_id"`
	Title         string                 `json:"title"`
	Slug          string                 `json:"slug"`
	Difficulty    string                 `json:"difficulty"`
	PatternName   string                 `json:"pattern_name"`
	Category      RecommendationCategory `json:"category"`
	PriorityScore int32                  `json:"priority_score"`
	Rationale     string                 `json:"rationale"`
	DaysOverdue   int32                  `json:"days_overdue,omitempty"`
}

type PatternMetric struct {
	PatternID         uuid.UUID `json:"pattern_id"`
	PatternName       string    `json:"pattern_name"`
	PatternSlug       string    `json:"pattern_slug"`
	TotalAttempts     int64     `json:"total_attempts"`
	FailedAttempts    int64     `json:"failed_attempts"`
	IndependentSolves int64     `json:"independent_solves"`
	HintSolves        int64     `json:"hint_solves"`
	AvgQualityRating  float64   `json:"avg_quality_rating"`
	IsWeak            bool      `json:"is_weak"`
}

type DailyPlan struct {
	Date                  string                `json:"date"` // RFC 3339 YYYY-MM-DD
	Recommendations       []RecommendedQuestion `json:"recommendations"`
	WeakPatterns          []PatternMetric       `json:"weak_patterns"`
	DueRevisionsCount     int64                 `json:"due_revisions_count"`
	OverdueRevisionsCount int64                 `json:"overdue_revisions_count"`
	DailyGoalMet          bool                  `json:"daily_goal_met"`
	GeneratedAt           time.Time             `json:"generated_at"`
}
