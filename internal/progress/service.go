package progress

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

type RecordAttemptRequest struct {
	UserID                uuid.UUID           `json:"user_id"`
	QuestionID            uuid.UUID           `json:"question_id"`
	RoadmapID             *uuid.UUID          `json:"roadmap_id,omitempty"`
	Outcome               AttemptOutcome      `json:"outcome"`
	TimeSpentSecs         int32               `json:"time_spent_secs"`
	DifficultyExperienced PerceivedDifficulty `json:"difficulty_experienced"`
	QualityRating         int32               `json:"quality_rating"`
	MistakeTags           []string            `json:"mistake_tags,omitempty"`
	ApproachSummary       *string             `json:"approach_summary,omitempty"`
	CodeSnippet           *string             `json:"code_snippet,omitempty"`
}

type UserProgressSummary struct {
	Stats          UserProgressStats `json:"stats"`
	RecentAttempts []QuestionAttempt `json:"recent_attempts"`
}

// Service defines high-level progress tracking and spaced repetition operations.
type Service interface {
	RecordAttempt(ctx context.Context, req RecordAttemptRequest) (*QuestionAttempt, *UserQuestionProgress, *RevisionSchedule, error)
	GetUserProgressSummary(ctx context.Context, userID uuid.UUID) (*UserProgressSummary, error)
	GetQuestionProgress(ctx context.Context, userID, questionID uuid.UUID) (*UserQuestionProgress, error)
	ListUserProgress(ctx context.Context, userID uuid.UUID) ([]UserQuestionProgress, error)
	QuickMark(ctx context.Context, req QuickMarkRequest) (*UserQuestionProgress, error)
	UnmarkProgress(ctx context.Context, userID, questionID uuid.UUID) error
	ListDueRevisions(ctx context.Context, userID uuid.UUID, limit, offset int32) ([]RevisionSchedule, int64, error)
	ExportUserData(ctx context.Context, userID uuid.UUID) (*UserExportData, error)
}

type service struct {
	repo Repository
}

// NewService instantiates a progress domain service.
func NewService(repo Repository) Service {
	return &service{repo: repo}
}

func (s *service) RecordAttempt(ctx context.Context, req RecordAttemptRequest) (*QuestionAttempt, *UserQuestionProgress, *RevisionSchedule, error) {
	if !req.Outcome.IsValid() {
		return nil, nil, nil, ErrInvalidOutcome
	}
	if req.QualityRating < 0 || req.QualityRating > 5 {
		return nil, nil, nil, ErrInvalidQuality
	}
	if !req.DifficultyExperienced.IsValid() {
		req.DifficultyExperienced = PerceivedJustRight
	}

	var recordedAttempt *QuestionAttempt
	var updatedProgress *UserQuestionProgress
	var updatedSchedule *RevisionSchedule

	err := s.repo.ExecTx(ctx, func(txRepo Repository) error {
		now := time.Now().UTC()

		// 1. Create immutable question attempt
		attempt := &QuestionAttempt{
			UserID:                req.UserID,
			QuestionID:            req.QuestionID,
			RoadmapID:             req.RoadmapID,
			Outcome:               req.Outcome,
			TimeSpentSecs:         req.TimeSpentSecs,
			DifficultyExperienced: req.DifficultyExperienced,
			QualityRating:         req.QualityRating,
			MistakeTags:           req.MistakeTags,
			ApproachSummary:       req.ApproachSummary,
			CodeSnippet:           req.CodeSnippet,
			CreatedAt:             now,
		}
		var err error
		recordedAttempt, err = txRepo.CreateQuestionAttempt(ctx, attempt)
		if err != nil {
			return fmt.Errorf("create attempt: %w", err)
		}

		// 2. Fetch existing user progress (if any) to determine new state
		existingProgress, err := txRepo.GetUserQuestionProgress(ctx, req.UserID, req.QuestionID)
		if err != nil && err != ErrProgressNotFound {
			return fmt.Errorf("get progress: %w", err)
		}

		var newStatus ProgressStatus = StatusInProgress
		var solveDelta int32 = 0
		var indepDelta int32 = 0
		var firstSolvedAt *time.Time
		var lastSolvedAt *time.Time

		if existingProgress != nil {
			newStatus = existingProgress.Status
			firstSolvedAt = existingProgress.FirstSolvedAt
			lastSolvedAt = existingProgress.LastSolvedAt
		}

		if req.Outcome.IsSolved() {
			solveDelta = 1
			lastSolvedAt = &now
			if firstSolvedAt == nil {
				firstSolvedAt = &now
			}

			if req.Outcome == OutcomeSolvedIndependently {
				indepDelta = 1
			}

			totalIndep := indepDelta
			if existingProgress != nil {
				totalIndep += existingProgress.IndependentSolveCount
			}

			if totalIndep >= 2 {
				newStatus = StatusMastered
			} else {
				newStatus = StatusSolved
			}
		} else {
			if existingProgress == nil || existingProgress.Status == StatusNotStarted {
				newStatus = StatusInProgress
			}
		}

		prog := &UserQuestionProgress{
			UserID:                req.UserID,
			QuestionID:            req.QuestionID,
			Status:                newStatus,
			FirstSolvedAt:         firstSolvedAt,
			LastSolvedAt:          lastSolvedAt,
			SolveCount:            solveDelta,
			IndependentSolveCount: indepDelta,
			TotalTimeSpentSecs:    req.TimeSpentSecs,
		}

		updatedProgress, err = txRepo.UpsertUserQuestionProgress(ctx, prog)
		if err != nil {
			return fmt.Errorf("upsert progress: %w", err)
		}

		// 3. Compute and update SM-2 Spaced Repetition schedule
		existingSchedule, err := txRepo.GetRevisionSchedule(ctx, req.UserID, req.QuestionID)
		if err != nil {
			return fmt.Errorf("get revision schedule: %w", err)
		}

		var curReps int32 = 0
		var curEF float64 = 2.50
		var curInterval int32 = 1

		if existingSchedule != nil {
			curReps = existingSchedule.RepetitionCount
			curEF = existingSchedule.EaseFactor
			curInterval = existingSchedule.IntervalDays
		}

		sm2 := CalculateSM2(curReps, curEF, curInterval, req.QualityRating, now)

		sched := &RevisionSchedule{
			UserID:          req.UserID,
			QuestionID:      req.QuestionID,
			Stability:       float64(sm2.IntervalDays),
			Difficulty:      curEF,
			RepetitionCount: sm2.RepetitionCount,
			EaseFactor:      sm2.EaseFactor,
			IntervalDays:    sm2.IntervalDays,
			NextReviewAt:    sm2.NextReviewAt,
			LastReviewedAt:  &now,
		}

		updatedSchedule, err = txRepo.UpsertRevisionSchedule(ctx, sched)
		if err != nil {
			return fmt.Errorf("upsert revision schedule: %w", err)
		}

		return nil
	})

	if err != nil {
		return nil, nil, nil, err
	}

	return recordedAttempt, updatedProgress, updatedSchedule, nil
}

func (s *service) GetUserProgressSummary(ctx context.Context, userID uuid.UUID) (*UserProgressSummary, error) {
	stats, err := s.repo.GetUserProgressStats(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("get progress stats: %w", err)
	}

	recentAttempts, err := s.repo.ListUserRecentAttempts(ctx, userID, 10)
	if err != nil {
		return nil, fmt.Errorf("list recent attempts: %w", err)
	}

	return &UserProgressSummary{
		Stats:          *stats,
		RecentAttempts: recentAttempts,
	}, nil
}

func (s *service) GetQuestionProgress(ctx context.Context, userID, questionID uuid.UUID) (*UserQuestionProgress, error) {
	return s.repo.GetUserQuestionProgress(ctx, userID, questionID)
}

func (s *service) ListDueRevisions(ctx context.Context, userID uuid.UUID, limit, offset int32) ([]RevisionSchedule, int64, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}

	now := time.Now().UTC()
	return s.repo.ListDueRevisions(ctx, userID, now, limit, offset)
}

func (s *service) ExportUserData(ctx context.Context, userID uuid.UUID) (*UserExportData, error) {
	return s.repo.ExportUserData(ctx, userID)
}

func (s *service) ListUserProgress(ctx context.Context, userID uuid.UUID) ([]UserQuestionProgress, error) {
	return s.repo.ListUserQuestionProgress(ctx, userID)
}

func (s *service) QuickMark(ctx context.Context, req QuickMarkRequest) (*UserQuestionProgress, error) {
	outcome := req.Outcome
	if !outcome.IsValid() {
		outcome = OutcomeSolvedIndependently
	}
	diff := req.DifficultyExperienced
	if !diff.IsValid() {
		diff = PerceivedJustRight
	}

	quality := int32(4)
	if req.NeedsRevision {
		quality = 3
	} else if req.ConfidenceFirstTry {
		quality = 5
	}

	var notes *string
	if strings.TrimSpace(req.Notes) != "" {
		cleaned := strings.TrimSpace(req.Notes)
		notes = &cleaned
	}

	_, prog, _, err := s.RecordAttempt(ctx, RecordAttemptRequest{
		UserID:                req.UserID,
		QuestionID:            req.QuestionID,
		Outcome:               outcome,
		TimeSpentSecs:         0,
		DifficultyExperienced: diff,
		QualityRating:         quality,
		ApproachSummary:       notes,
	})
	return prog, err
}

func (s *service) UnmarkProgress(ctx context.Context, userID, questionID uuid.UUID) error {
	return s.repo.DeleteUserQuestionProgress(ctx, userID, questionID)
}

