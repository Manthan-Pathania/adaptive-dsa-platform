package adaptive

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/adaptive-dsa/platform/internal/catalog"
	"github.com/adaptive-dsa/platform/internal/progress"
	"github.com/google/uuid"
)

type Service interface {
	GetDailyPlan(ctx context.Context, userID uuid.UUID, roadmapID *uuid.UUID, asOf time.Time) (*DailyPlan, error)
	GetWeakPatterns(ctx context.Context, userID uuid.UUID) ([]PatternMetric, error)
}

type service struct {
	repo        Repository
	progressSvc progress.Service
	catalogSvc  catalog.Service
}

func NewService(repo Repository, progressSvc progress.Service, catalogSvc catalog.Service) Service {
	return &service{
		repo:        repo,
		progressSvc: progressSvc,
		catalogSvc:  catalogSvc,
	}
}

func (s *service) GetWeakPatterns(ctx context.Context, userID uuid.UUID) ([]PatternMetric, error) {
	metrics, err := s.repo.GetUserPatternMetrics(ctx, userID)
	if err != nil {
		return nil, err
	}
	var weak []PatternMetric
	for _, m := range metrics {
		if m.IsWeak {
			weak = append(weak, m)
		}
	}
	return weak, nil
}

func (s *service) GetDailyPlan(ctx context.Context, userID uuid.UUID, roadmapID *uuid.UUID, asOf time.Time) (*DailyPlan, error) {
	if asOf.IsZero() {
		asOf = time.Now().UTC()
	}

	dateStr := asOf.Format("2006-01-02")
	var candidates []RecommendedQuestion
	seen := make(map[uuid.UUID]bool)

	// 1. Spaced Repetition Due Revisions (Rank 1: Highest retention urgency)
	var dueCount int64
	var overdueCount int64

	if s.progressSvc != nil {
		revisions, totalDue, err := s.progressSvc.ListDueRevisions(ctx, userID, 10, 0)
		if err == nil {
			dueCount = totalDue
			for _, rev := range revisions {
				var daysOverdue int32 = 0
				if asOf.After(rev.NextReviewAt) {
					daysOverdue = int32(asOf.Sub(rev.NextReviewAt).Hours() / 24)
					if daysOverdue > 0 {
						overdueCount++
					}
				}

				score := int32(1000 + daysOverdue*25)
				rationale := fmt.Sprintf("Spaced repetition review due (ease factor: %.2f, interval: %dd)", rev.EaseFactor, rev.IntervalDays)
				if daysOverdue > 0 {
					rationale = fmt.Sprintf("Overdue for spaced repetition by %d days (ease factor: %.2f)", daysOverdue, rev.EaseFactor)
				}

				if !seen[rev.QuestionID] {
					seen[rev.QuestionID] = true
					candidates = append(candidates, RecommendedQuestion{
						QuestionID:    rev.QuestionID,
						Title:         rev.QuestionTitle,
						Slug:          rev.QuestionSlug,
						Difficulty:    rev.QuestionDifficulty,
						Category:      CategoryDueRevision,
						PriorityScore: score,
						Rationale:     rationale,
						DaysOverdue:   daysOverdue,
					})
				}
			}
		}
	}

	// 2. Weak Pattern Reinforcement (Rank 2: Addressing struggles)
	weakMetrics, err := s.GetWeakPatterns(ctx, userID)
	if err == nil && len(weakMetrics) > 0 {
		for _, wm := range weakMetrics {
			unsolved, err := s.repo.GetUnsolvedQuestionsForPattern(ctx, userID, wm.PatternID, 2)
			if err == nil {
				score := int32(500 + wm.FailedAttempts*30 + int64((5.0-wm.AvgQualityRating)*20))
				for _, uq := range unsolved {
					if !seen[uq.QuestionID] {
						seen[uq.QuestionID] = true
						candidates = append(candidates, RecommendedQuestion{
							QuestionID:    uq.QuestionID,
							Title:         uq.Title,
							Slug:          uq.Slug,
							Difficulty:    uq.Difficulty,
							PatternName:   wm.PatternName,
							Category:      CategoryWeakPattern,
							PriorityScore: score,
							Rationale:     fmt.Sprintf("Strengthens '%s' pattern (%d unresolved failures, %.1f avg rating)", wm.PatternName, wm.FailedAttempts, wm.AvgQualityRating),
						})
					}
				}
			}
		}
	}

	// 3. Roadmap Next Step (Rank 3: Milestone progression)
	if s.catalogSvc != nil && s.progressSvc != nil {
		var targetRoadmap *catalog.Roadmap
		if roadmapID != nil && *roadmapID != uuid.Nil {
			targetRoadmap, _ = s.catalogSvc.GetRoadmap(ctx, roadmapID.String())
		} else {
			// Pull system roadmap
			roadmaps, err := s.catalogSvc.ListRoadmaps(ctx)
			if err == nil && len(roadmaps) > 0 {
				targetRoadmap, _ = s.catalogSvc.GetRoadmap(ctx, roadmaps[0].ID.String())
			}
		}

		if targetRoadmap != nil {
			for _, item := range targetRoadmap.Items {
				prog, _ := s.progressSvc.GetQuestionProgress(ctx, userID, item.QuestionID)
				isSolved := prog != nil && (prog.Status == progress.StatusSolved || prog.Status == progress.StatusMastered)
				if !isSolved && !seen[item.QuestionID] {
					seen[item.QuestionID] = true
					score := int32(300 - item.OrderIndex)
					candidates = append(candidates, RecommendedQuestion{
						QuestionID:    item.QuestionID,
						Title:         item.QuestionTitle,
						Slug:          item.QuestionSlug,
						Category:      CategoryRoadmapNext,
						PriorityScore: score,
						Rationale:     fmt.Sprintf("Stage %d mandatory milestone in '%s'", item.StageNumber, targetRoadmap.Title),
					})
					break // Pick next single milestone
				}
			}
		}
	}

	// 4. Core Question Exploration (Rank 4: Breadth)
	if len(candidates) < 5 && s.catalogSvc != nil {
		questions, _, err := s.catalogSvc.ListQuestions(ctx, catalog.ListQuestionsFilter{Limit: 10, Offset: 0})
		if err == nil {
			for _, q := range questions {
				if q.IsCore && !seen[q.ID] {
					var isSolved bool
					if s.progressSvc != nil {
						prog, _ := s.progressSvc.GetQuestionProgress(ctx, userID, q.ID)
						isSolved = prog != nil && (prog.Status == progress.StatusSolved || prog.Status == progress.StatusMastered)
					}
					if !isSolved {
						seen[q.ID] = true
						var patternName string
						if len(q.Patterns) > 0 {
							patternName = q.Patterns[0].Name
						}
						candidates = append(candidates, RecommendedQuestion{
							QuestionID:    q.ID,
							Title:         q.Title,
							Slug:          q.Slug,
							Difficulty:    string(q.Difficulty),
							PatternName:   patternName,
							Category:      CategoryExploration,
							PriorityScore: 100,
							Rationale:     "Core foundational problem for technical interview pattern breadth",
						})
						if len(candidates) >= 5 {
							break
						}
					}
				}
			}
		}
	}

	// Sort deterministically: Highest priority first, tie-break by Title ASC
	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].PriorityScore != candidates[j].PriorityScore {
			return candidates[i].PriorityScore > candidates[j].PriorityScore
		}
		return candidates[i].Title < candidates[j].Title
	})

	if len(candidates) > 5 {
		candidates = candidates[:5]
	}

	return &DailyPlan{
		Date:                  dateStr,
		Recommendations:       candidates,
		WeakPatterns:          weakMetrics,
		DueRevisionsCount:     dueCount,
		OverdueRevisionsCount: overdueCount,
		DailyGoalMet:          false,
		GeneratedAt:           asOf,
	}, nil
}
