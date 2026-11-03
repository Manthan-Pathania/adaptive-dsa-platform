package ai

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/adaptive-dsa/platform/internal/catalog"
	"github.com/adaptive-dsa/platform/internal/credits"
	"github.com/google/uuid"
)

// Service defines high-level AI tutoring workflows with credit metering and provider abstraction.
type Service interface {
	RequestHint(ctx context.Context, userID uuid.UUID, req GenerateHintRequest) (*HintResult, error)
	ReviewApproach(ctx context.Context, userID uuid.UUID, req ReviewApproachRequest) (*ReviewResult, error)
	ExplainConcept(ctx context.Context, userID uuid.UUID, req ExplainConceptRequest) (*ExplainResult, error)
	GenerateCustomRoadmap(ctx context.Context, userID uuid.UUID, req GenerateRoadmapRequest) (*GenerateRoadmapResponse, error)
	CreateCustomRoadmap(ctx context.Context, userID uuid.UUID, req GenerateRoadmapRequest) (*catalog.Roadmap, error)
	ListUserRequests(ctx context.Context, userID uuid.UUID, limit, offset int32) ([]AIRequestRecord, int64, error)
}

type service struct {
	aiRepo     Repository
	provider   AIProvider
	creditSvc  credits.Service
	catalogSvc catalog.Service
}

func NewService(
	aiRepo Repository,
	provider AIProvider,
	creditSvc credits.Service,
	catalogSvc catalog.Service,
) Service {
	return &service{
		aiRepo:     aiRepo,
		provider:   provider,
		creditSvc:  creditSvc,
		catalogSvc: catalogSvc,
	}
}

func (s *service) RequestHint(ctx context.Context, userID uuid.UUID, req GenerateHintRequest) (*HintResult, error) {
	if !req.Level.IsValid() {
		return nil, ErrInvalidHintLevel
	}

	// 1. Authoritative question lookup from catalog
	if req.QuestionID != uuid.Nil && s.catalogSvc != nil {
		q, err := s.catalogSvc.GetQuestion(ctx, req.QuestionID.String())
		if err == nil && q != nil {
			req.QuestionTitle = q.Title
			req.Difficulty = string(q.Difficulty)
			if len(q.Patterns) > 0 {
				req.PatternName = q.Patterns[0].Name
			}
		}
	}

	cost := req.Level.CreditCost()

	// 2. Pre-check credit balance (reservation check)
	if err := s.creditSvc.CheckAndReserve(ctx, userID, cost); err != nil {
		return nil, err
	}

	// 3. Invoke AI Provider with latency metering
	start := time.Now()
	resp, aiErr := s.provider.GenerateHint(ctx, req)
	latency := int32(time.Since(start).Milliseconds())

	// 4a. Zero-charge guarantee: If provider fails or times out, charge 0 credits
	if aiErr != nil {
		_, _ = s.aiRepo.CreateRequest(ctx, &AIRequestRecord{
			UserID:                 userID,
			Feature:                FeatureProgressiveHint,
			Model:                  "gemini-3.8-flash",
			PromptTokens:           0,
			CompletionTokens:       0,
			EstimatedCostUsdMicros: 0,
			CreditsCharged:         0,
			Status:                 "FAILED",
			LatencyMs:              latency,
			CreatedAt:              time.Now().UTC(),
		})
		return nil, aiErr
	}

	// 4b. On success, atomically commit credit deduction
	desc := fmt.Sprintf("AI Hint Level %d for '%s'", req.Level, req.QuestionTitle)
	entry, deductErr := s.creditSvc.CommitDeduction(
		ctx,
		userID,
		cost,
		credits.EntryTypeAIUsage,
		"AI_HINT",
		&req.QuestionID,
		desc,
	)
	if deductErr != nil {
		return nil, fmt.Errorf("credit deduction failed: %w", deductErr)
	}

	// 5. Persist successful AI audit log
	reqRec, _ := s.aiRepo.CreateRequest(ctx, &AIRequestRecord{
		UserID:                 userID,
		Feature:                FeatureProgressiveHint,
		Model:                  resp.ModelUsed,
		PromptTokens:           resp.PromptTokens,
		CompletionTokens:       resp.CompletionTokens,
		EstimatedCostUsdMicros: (resp.PromptTokens*15 + resp.CompletionTokens*60) / 1000,
		CreditsCharged:         cost,
		Status:                 "SUCCESS",
		LatencyMs:              latency,
		CreatedAt:              time.Now().UTC(),
	})
	_ = reqRec
	_ = entry

	remBalance, _ := s.creditSvc.GetBalance(ctx, userID)

	return &HintResult{
		Level:            req.Level,
		HintText:         resp.Content,
		CreditsDeducted:  cost,
		RemainingBalance: remBalance,
		ModelUsed:        resp.ModelUsed,
	}, nil
}

func (s *service) ReviewApproach(ctx context.Context, userID uuid.UUID, req ReviewApproachRequest) (*ReviewResult, error) {
	if req.CandidateCode == "" {
		return nil, ErrEmptyInput
	}

	// Authoritative question lookup
	if req.QuestionID != uuid.Nil && s.catalogSvc != nil {
		q, err := s.catalogSvc.GetQuestion(ctx, req.QuestionID.String())
		if err == nil && q != nil {
			req.QuestionTitle = q.Title
		}
	}

	cost := int32(CostApproachReview)

	// Pre-check balance
	if err := s.creditSvc.CheckAndReserve(ctx, userID, cost); err != nil {
		return nil, err
	}

	start := time.Now()
	resp, aiErr := s.provider.ReviewApproach(ctx, req)
	latency := int32(time.Since(start).Milliseconds())

	if aiErr != nil {
		_, _ = s.aiRepo.CreateRequest(ctx, &AIRequestRecord{
			UserID:         userID,
			Feature:        FeatureApproachReview,
			Model:          "gemini-3.8-flash",
			CreditsCharged: 0,
			Status:         "FAILED",
			LatencyMs:      latency,
			CreatedAt:      time.Now().UTC(),
		})
		return nil, aiErr
	}

	desc := fmt.Sprintf("AI Approach Review for '%s'", req.QuestionTitle)
	_, deductErr := s.creditSvc.CommitDeduction(
		ctx,
		userID,
		cost,
		credits.EntryTypeAIUsage,
		"AI_REVIEW",
		&req.QuestionID,
		desc,
	)
	if deductErr != nil {
		return nil, fmt.Errorf("credit deduction failed: %w", deductErr)
	}

	_, _ = s.aiRepo.CreateRequest(ctx, &AIRequestRecord{
		UserID:                 userID,
		Feature:                FeatureApproachReview,
		Model:                  resp.ModelUsed,
		PromptTokens:           resp.PromptTokens,
		CompletionTokens:       resp.CompletionTokens,
		EstimatedCostUsdMicros: (resp.PromptTokens*15 + resp.CompletionTokens*60) / 1000,
		CreditsCharged:         cost,
		Status:                 "SUCCESS",
		LatencyMs:              latency,
		CreatedAt:              time.Now().UTC(),
	})

	remBalance, _ := s.creditSvc.GetBalance(ctx, userID)

	return &ReviewResult{
		ReviewText:       resp.Content,
		CreditsDeducted:  cost,
		RemainingBalance: remBalance,
		ModelUsed:        resp.ModelUsed,
	}, nil
}

func (s *service) ExplainConcept(ctx context.Context, userID uuid.UUID, req ExplainConceptRequest) (*ExplainResult, error) {
	if req.CandidateQuery == "" {
		return nil, ErrEmptyInput
	}

	cost := int32(CostConceptExplain)

	// Pre-check balance
	if err := s.creditSvc.CheckAndReserve(ctx, userID, cost); err != nil {
		return nil, err
	}

	start := time.Now()
	resp, aiErr := s.provider.ExplainConcept(ctx, req)
	latency := int32(time.Since(start).Milliseconds())

	if aiErr != nil {
		_, _ = s.aiRepo.CreateRequest(ctx, &AIRequestRecord{
			UserID:         userID,
			Feature:        FeatureConceptExplain,
			Model:          "gemini-3.8-flash",
			CreditsCharged: 0,
			Status:         "FAILED",
			LatencyMs:      latency,
			CreatedAt:      time.Now().UTC(),
		})
		return nil, aiErr
	}

	desc := fmt.Sprintf("AI Concept Explanation: %s", req.PatternName)
	_, deductErr := s.creditSvc.CommitDeduction(
		ctx,
		userID,
		cost,
		credits.EntryTypeAIUsage,
		"AI_EXPLAIN",
		nil,
		desc,
	)
	if deductErr != nil {
		return nil, fmt.Errorf("credit deduction failed: %w", deductErr)
	}

	_, _ = s.aiRepo.CreateRequest(ctx, &AIRequestRecord{
		UserID:                 userID,
		Feature:                FeatureConceptExplain,
		Model:                  resp.ModelUsed,
		PromptTokens:           resp.PromptTokens,
		CompletionTokens:       resp.CompletionTokens,
		EstimatedCostUsdMicros: (resp.PromptTokens*15 + resp.CompletionTokens*60) / 1000,
		CreditsCharged:         cost,
		Status:                 "SUCCESS",
		LatencyMs:              latency,
		CreatedAt:              time.Now().UTC(),
	})

	remBalance, _ := s.creditSvc.GetBalance(ctx, userID)

	return &ExplainResult{
		Explanation:      resp.Content,
		CreditsDeducted:  cost,
		RemainingBalance: remBalance,
		ModelUsed:        resp.ModelUsed,
	}, nil
}

func (s *service) ListUserRequests(ctx context.Context, userID uuid.UUID, limit, offset int32) ([]AIRequestRecord, int64, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}
	return s.aiRepo.ListRequestsByUser(ctx, userID, limit, offset)
}

func (s *service) GenerateCustomRoadmap(ctx context.Context, userID uuid.UUID, req GenerateRoadmapRequest) (*GenerateRoadmapResponse, error) {
	if len(req.Questions) == 0 && s.catalogSvc != nil {
		allQuestions, _, err := s.catalogSvc.ListQuestions(ctx, catalog.ListQuestionsFilter{
			Limit: 1000,
		})
		if err == nil {
			for _, q := range allQuestions {
				patName := "General"
				subpatName := ""
				if len(q.Patterns) > 0 {
					patName = q.Patterns[0].Name
					if len(q.Patterns) > 1 {
						subpatName = q.Patterns[1].Name
					}
				}
				req.Questions = append(req.Questions, RoadmapQuestionItem{
					ID:         q.ID,
					Title:      q.Title,
					Pattern:    patName,
					Subpattern: subpatName,
					Difficulty: string(q.Difficulty),
					ExcelOrder: q.ExcelOrder,
				})
			}
		}
	}

	return s.provider.GenerateCustomRoadmap(ctx, req)
}

func (s *service) CreateCustomRoadmap(ctx context.Context, userID uuid.UUID, req GenerateRoadmapRequest) (*catalog.Roadmap, error) {
	resp, err := s.GenerateCustomRoadmap(ctx, userID, req)
	if err != nil {
		return nil, fmt.Errorf("generate custom roadmap: %w", err)
	}

	title := strings.TrimSpace(resp.Title)
	if title == "" {
		title = "Custom Interview Roadmap"
	}
	desc := resp.Description
	slug := fmt.Sprintf("custom-%d-%s", time.Now().Unix(), uuid.New().String()[:8])

	r := &catalog.Roadmap{
		Title:            title,
		Slug:             slug,
		Description:      &desc,
		UserID:           &userID,
		IsSystemTemplate: false,
	}

	var items []catalog.RoadmapItem
	orderIdx := int32(0)
	for _, stage := range resp.Stages {
		stageNum := stage.StageNumber
		if stageNum <= 0 {
			stageNum = 1
		}
		for _, qID := range stage.QuestionIDs {
			if qID == uuid.Nil {
				continue
			}
			items = append(items, catalog.RoadmapItem{
				QuestionID:  qID,
				StageNumber: stageNum,
				OrderIndex:  orderIdx,
				IsMandatory: true,
			})
			orderIdx++
		}
	}

	if len(items) == 0 && len(req.Questions) > 0 {
		for i, q := range req.Questions {
			if i >= 30 {
				break
			}
			stageNum := int32(1)
			if i >= 10 {
				stageNum = 2
			}
			if i >= 20 {
				stageNum = 3
			}
			items = append(items, catalog.RoadmapItem{
				QuestionID:  q.ID,
				StageNumber: stageNum,
				OrderIndex:  int32(i),
				IsMandatory: true,
			})
		}
	}

	if s.catalogSvc == nil {
		return nil, errors.New("catalog service not available")
	}

	return s.catalogSvc.CreateUserRoadmap(ctx, r, items)
}
