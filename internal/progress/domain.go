package progress

import (
	"errors"
	"math"
	"time"

	"github.com/google/uuid"
)

var (
	ErrAttemptNotFound  = errors.New("attempt not found")
	ErrProgressNotFound = errors.New("progress not found")
	ErrInvalidQuality   = errors.New("quality rating must be between 0 and 5")
	ErrInvalidOutcome   = errors.New("invalid attempt outcome")
)

type AttemptOutcome string

const (
	OutcomeSolvedIndependently  AttemptOutcome = "SOLVED_INDEPENDENTLY"
	OutcomeSolvedWithHints      AttemptOutcome = "SOLVED_WITH_HINTS"
	OutcomeSolvedAfterEditorial AttemptOutcome = "SOLVED_AFTER_EDITORIAL"
	OutcomeUnresolvedFailed     AttemptOutcome = "UNRESOLVED_FAILED"
)

func (o AttemptOutcome) IsValid() bool {
	switch o {
	case OutcomeSolvedIndependently, OutcomeSolvedWithHints, OutcomeSolvedAfterEditorial, OutcomeUnresolvedFailed:
		return true
	default:
		return false
	}
}

func (o AttemptOutcome) IsSolved() bool {
	return o == OutcomeSolvedIndependently || o == OutcomeSolvedWithHints || o == OutcomeSolvedAfterEditorial
}

type PerceivedDifficulty string

const (
	PerceivedEasy         PerceivedDifficulty = "EASY"
	PerceivedJustRight    PerceivedDifficulty = "JUST_RIGHT"
	PerceivedChallenging  PerceivedDifficulty = "CHALLENGING"
	PerceivedOverwhelming PerceivedDifficulty = "OVERWHELMING"
)

func (p PerceivedDifficulty) IsValid() bool {
	switch p {
	case PerceivedEasy, PerceivedJustRight, PerceivedChallenging, PerceivedOverwhelming:
		return true
	default:
		return false
	}
}

type ProgressStatus string

const (
	StatusNotStarted ProgressStatus = "NOT_STARTED"
	StatusInProgress ProgressStatus = "IN_PROGRESS"
	StatusSolved     ProgressStatus = "SOLVED"
	StatusMastered   ProgressStatus = "MASTERED"
)

func (s ProgressStatus) IsValid() bool {
	switch s {
	case StatusNotStarted, StatusInProgress, StatusSolved, StatusMastered:
		return true
	default:
		return false
	}
}

// QuestionAttempt models an immutable attempt record.
type QuestionAttempt struct {
	ID                    uuid.UUID           `json:"id"`
	UserID                uuid.UUID           `json:"user_id"`
	QuestionID            uuid.UUID           `json:"question_id"`
	RoadmapID             *uuid.UUID          `json:"roadmap_id,omitempty"`
	Outcome               AttemptOutcome      `json:"outcome"`
	TimeSpentSecs         int32               `json:"time_spent_secs"`
	DifficultyExperienced PerceivedDifficulty `json:"difficulty_experienced"`
	QualityRating         int32               `json:"quality_rating"`
	MistakeTags           []string            `json:"mistake_tags"`
	ApproachSummary       *string             `json:"approach_summary,omitempty"`
	CodeSnippet           *string             `json:"code_snippet,omitempty"`
	CreatedAt             time.Time           `json:"created_at"`
	QuestionTitle         string              `json:"question_title,omitempty"`
	QuestionSlug          string              `json:"question_slug,omitempty"`
	QuestionDifficulty    string              `json:"question_difficulty,omitempty"`
}

// UserQuestionProgress models the aggregated canonical progress for a question across all roadmaps.
type UserQuestionProgress struct {
	ID                    uuid.UUID      `json:"id"`
	UserID                uuid.UUID      `json:"user_id"`
	QuestionID            uuid.UUID      `json:"question_id"`
	Status                ProgressStatus `json:"status"`
	FirstSolvedAt         *time.Time     `json:"first_solved_at,omitempty"`
	LastSolvedAt          *time.Time     `json:"last_solved_at,omitempty"`
	SolveCount            int32          `json:"solve_count"`
	IndependentSolveCount int32          `json:"independent_solve_count"`
	TotalTimeSpentSecs    int32          `json:"total_time_spent_secs"`
	IsBookmarked          bool           `json:"is_bookmarked"`
	PersonalNotes         *string        `json:"personal_notes,omitempty"`
	CreatedAt             time.Time      `json:"created_at"`
	UpdatedAt             time.Time      `json:"updated_at"`
}

// RevisionSchedule tracks the spaced repetition state (SM-2 / SuperMemo 2).
type RevisionSchedule struct {
	ID                 uuid.UUID  `json:"id"`
	UserID             uuid.UUID  `json:"user_id"`
	QuestionID         uuid.UUID  `json:"question_id"`
	Stability          float64    `json:"stability"`
	Difficulty         float64    `json:"difficulty"`
	RepetitionCount    int32      `json:"repetition_count"`
	EaseFactor         float64    `json:"ease_factor"`
	IntervalDays       int32      `json:"interval_days"`
	NextReviewAt       time.Time  `json:"next_review_at"`
	LastReviewedAt     *time.Time `json:"last_reviewed_at,omitempty"`
	CreatedAt          time.Time  `json:"created_at"`
	UpdatedAt          time.Time  `json:"updated_at"`
	QuestionTitle      string     `json:"question_title,omitempty"`
	QuestionSlug       string     `json:"question_slug,omitempty"`
	QuestionDifficulty string     `json:"question_difficulty,omitempty"`
}

type UserProgressStats struct {
	TotalSolved   int64 `json:"total_solved"`
	TotalMastered int64 `json:"total_mastered"`
	TotalInFlight int64 `json:"total_in_flight"`
}

// SM2Result contains the updated spaced repetition scheduling parameters.
type SM2Result struct {
	RepetitionCount int32
	EaseFactor      float64
	IntervalDays    int32
	NextReviewAt    time.Time
}

// CalculateSM2 computes the next spaced repetition interval based on the standard SuperMemo 2 formula.
// quality: 0 to 5 scale (where 0-2 = failed/review needed, 3-5 = successful recall).
func CalculateSM2(currentReps int32, currentEF float64, currentInterval int32, quality int32, now time.Time) SM2Result {
	if currentEF < 1.30 {
		currentEF = 2.50
	}
	if quality < 0 {
		quality = 0
	}
	if quality > 5 {
		quality = 5
	}

	var nextReps int32
	var nextInterval int32
	var nextEF float64

	if quality >= 3 {
		// Successful review
		if currentReps == 0 {
			nextInterval = 1
		} else if currentReps == 1 {
			nextInterval = 6
		} else {
			nextInterval = int32(math.Round(float64(currentInterval) * currentEF))
			if nextInterval <= currentInterval {
				nextInterval = currentInterval + 1
			}
		}
		nextReps = currentReps + 1

		// EF' = EF + (0.1 - (5 - q) * (0.08 + (5 - q) * 0.02))
		qDiff := float64(5 - quality)
		nextEF = currentEF + (0.1 - qDiff*(0.08+qDiff*0.02))
		if nextEF < 1.30 {
			nextEF = 1.30
		}
	} else {
		// Failed or struggled heavily: reset repetition chain
		nextReps = 0
		nextInterval = 1
		nextEF = currentEF
	}

	// Round ease factor to 2 decimal places
	nextEF = math.Round(nextEF*100) / 100

	nextReview := now.Add(time.Duration(nextInterval) * 24 * time.Hour)

	return SM2Result{
		RepetitionCount: nextReps,
		EaseFactor:      nextEF,
		IntervalDays:    nextInterval,
		NextReviewAt:    nextReview,
	}
}
