package ai

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
)

var (
	ErrQuotaExceeded       = errors.New("ai tutor quota temporarily exceeded; retry in 60 seconds")
	ErrProviderUnavailable = errors.New("ai tutor provider unavailable")
	ErrInvalidHintLevel    = errors.New("invalid hint level: must be 1 (Nudge), 2 (Structural), 3 (Invariant), or 4 (Pseudocode)")
	ErrEmptyInput          = errors.New("input text cannot be empty")
)

type HintLevel int

const (
	HintLevelNudge      HintLevel = 1 // High-level conceptual direction
	HintLevelStructural HintLevel = 2 // Pattern & data structure identification
	HintLevelInvariant  HintLevel = 3 // Loop invariant / state transition logic
	HintLevelPseudocode HintLevel = 4 // Algorithmic walkthrough / edge cases
)

func (h HintLevel) IsValid() bool {
	return h >= HintLevelNudge && h <= HintLevelPseudocode
}

func (h HintLevel) CreditCost() int32 {
	switch h {
	case HintLevelNudge, HintLevelStructural:
		return 2
	case HintLevelInvariant, HintLevelPseudocode:
		return 4
	default:
		return 4
	}
}

const (
	CostApproachReview = 5
	CostConceptExplain = 3
)

type FeatureType string

const (
	FeatureProgressiveHint FeatureType = "PROGRESSIVE_HINT"
	FeatureApproachReview  FeatureType = "APPROACH_REVIEW"
	FeatureConceptExplain  FeatureType = "CONCEPT_EXPLANATION"
	FeatureMistakeClassify FeatureType = "MISTAKE_CLASSIFICATION"
	FeatureRevisionSummary FeatureType = "REVISION_SUMMARY"
)

type GenerateHintRequest struct {
	QuestionID     uuid.UUID `json:"question_id"`
	QuestionTitle  string    `json:"question_title"`
	PatternName    string    `json:"pattern_name"`
	Difficulty     string    `json:"difficulty"`
	CandidateNotes string    `json:"candidate_notes"`
	Level          HintLevel `json:"level"`
}

type ReviewApproachRequest struct {
	QuestionID    uuid.UUID `json:"question_id"`
	QuestionTitle string    `json:"question_title"`
	CandidateCode string    `json:"candidate_code"`
	ProposedTime  string    `json:"proposed_time"`
	ProposedSpace string    `json:"proposed_space"`
}

type ExplainConceptRequest struct {
	TopicName      string `json:"topic_name"`
	PatternName    string `json:"pattern_name"`
	CandidateQuery string `json:"candidate_query"`
}

type RoadmapQuestionItem struct {
	ID         uuid.UUID `json:"id"`
	Title      string    `json:"title"`
	Pattern    string    `json:"pattern"`
	Subpattern string    `json:"subpattern"`
	Difficulty string    `json:"difficulty"`
	ExcelOrder int32     `json:"excel_order"`
}

type GenerateRoadmapRequest struct {
	TargetCompanies  []string              `json:"target_companies"`
	TimelineDays     int32                 `json:"timeline_days"`
	DailyHours       float32               `json:"daily_hours"`
	ExperienceLevel  string                `json:"experience_level"`
	FamiliarPatterns []string              `json:"familiar_patterns"`
	FocusNotes       string                `json:"focus_notes"`
	Questions        []RoadmapQuestionItem `json:"-"`
}

type GeneratedRoadmapStage struct {
	StageNumber int32       `json:"stage_number"`
	Title       string      `json:"title"`
	QuestionIDs []uuid.UUID `json:"question_ids"`
}

type GenerateRoadmapResponse struct {
	Title       string                  `json:"title"`
	Description string                  `json:"description"`
	TargetDays  int32                   `json:"target_days"`
	Stages      []GeneratedRoadmapStage `json:"stages"`
}

type AIResponse struct {
	Content          string `json:"content"`
	PromptTokens     int32  `json:"prompt_tokens"`
	CompletionTokens int32  `json:"completion_tokens"`
	ModelUsed        string `json:"model_used"`
}

// AIProvider abstracts the external LLM provider.
type AIProvider interface {
	GenerateHint(ctx context.Context, req GenerateHintRequest) (*AIResponse, error)
	ReviewApproach(ctx context.Context, req ReviewApproachRequest) (*AIResponse, error)
	ExplainConcept(ctx context.Context, req ExplainConceptRequest) (*AIResponse, error)
	GenerateCustomRoadmap(ctx context.Context, req GenerateRoadmapRequest) (*GenerateRoadmapResponse, error)
}

// AIRequestRecord models the persisted AI request log in the database.
type AIRequestRecord struct {
	ID                     uuid.UUID   `json:"id"`
	UserID                 uuid.UUID   `json:"user_id"`
	Feature                FeatureType `json:"feature"`
	Model                  string      `json:"model"`
	PromptTokens           int32       `json:"prompt_tokens"`
	CompletionTokens       int32       `json:"completion_tokens"`
	EstimatedCostUsdMicros int32       `json:"estimated_cost_usd_micros"`
	CreditsCharged         int32       `json:"credits_charged"`
	Status                 string      `json:"status"` // 'SUCCESS', 'FAILED', 'TIMEOUT'
	LatencyMs              int32       `json:"latency_ms"`
	CreatedAt              time.Time   `json:"created_at"`
}

type HintResult struct {
	Level            HintLevel `json:"level"`
	HintText         string    `json:"hint_text"`
	CreditsDeducted  int32     `json:"credits_deducted"`
	RemainingBalance int32     `json:"remaining_balance"`
	ModelUsed        string    `json:"model_used"`
}

type ReviewResult struct {
	ReviewText       string `json:"review_text"`
	CreditsDeducted  int32  `json:"credits_deducted"`
	RemainingBalance int32  `json:"remaining_balance"`
	ModelUsed        string `json:"model_used"`
}

type ExplainResult struct {
	Explanation      string `json:"explanation"`
	CreditsDeducted  int32  `json:"credits_deducted"`
	RemainingBalance int32  `json:"remaining_balance"`
	ModelUsed        string `json:"model_used"`
}
