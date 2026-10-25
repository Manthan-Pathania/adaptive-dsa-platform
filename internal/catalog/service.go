package catalog

import (
	"context"
	"fmt"

	"github.com/google/uuid"
)

// Service defines high-level catalog domain operations.
type Service interface {
	ListQuestions(ctx context.Context, filter ListQuestionsFilter) ([]Question, int64, error)
	GetQuestion(ctx context.Context, idOrSlug string) (*Question, error)
	ListTopics(ctx context.Context) ([]Topic, error)
	ListRoadmaps(ctx context.Context) ([]Roadmap, error)
	GetRoadmap(ctx context.Context, idOrSlug string) (*Roadmap, error)
}

type service struct {
	repo Repository
}

// NewService instantiates a catalog domain service.
func NewService(repo Repository) Service {
	return &service{repo: repo}
}

func (s *service) ListQuestions(ctx context.Context, filter ListQuestionsFilter) ([]Question, int64, error) {
	if filter.Limit <= 0 {
		filter.Limit = 20
	}
	if filter.Limit > 100 {
		filter.Limit = 100
	}
	if filter.Offset < 0 {
		filter.Offset = 0
	}

	questions, total, err := s.repo.ListQuestions(ctx, filter)
	if err != nil {
		return nil, 0, fmt.Errorf("list questions: %w", err)
	}

	// Enrich questions with pattern tags
	for i := range questions {
		patterns, err := s.repo.GetQuestionPatterns(ctx, questions[i].ID)
		if err == nil {
			questions[i].Patterns = patterns
		}
		companies, err := s.repo.GetQuestionCompanies(ctx, questions[i].ID)
		if err == nil {
			questions[i].Companies = companies
		}
	}

	return questions, total, nil
}

func (s *service) GetQuestion(ctx context.Context, idOrSlug string) (*Question, error) {
	var q *Question
	var err error

	if parsedID, parseErr := uuid.Parse(idOrSlug); parseErr == nil {
		q, err = s.repo.GetQuestionByID(ctx, parsedID)
	} else {
		q, err = s.repo.GetQuestionBySlug(ctx, idOrSlug)
	}

	if err != nil {
		return nil, err
	}

	patterns, err := s.repo.GetQuestionPatterns(ctx, q.ID)
	if err == nil {
		q.Patterns = patterns
	}
	companies, err := s.repo.GetQuestionCompanies(ctx, q.ID)
	if err == nil {
		q.Companies = companies
	}

	return q, nil
}

func (s *service) ListTopics(ctx context.Context) ([]Topic, error) {
	return s.repo.ListTopics(ctx)
}

func (s *service) ListRoadmaps(ctx context.Context) ([]Roadmap, error) {
	return s.repo.ListCuratedRoadmaps(ctx)
}

func (s *service) GetRoadmap(ctx context.Context, idOrSlug string) (*Roadmap, error) {
	var r *Roadmap
	var err error

	if parsedID, parseErr := uuid.Parse(idOrSlug); parseErr == nil {
		r, err = s.repo.GetRoadmapByID(ctx, parsedID)
	} else {
		r, err = s.repo.GetRoadmapBySlug(ctx, idOrSlug)
	}

	if err != nil {
		return nil, err
	}

	items, err := s.repo.ListRoadmapItems(ctx, r.ID)
	if err != nil {
		return nil, fmt.Errorf("list roadmap items: %w", err)
	}
	r.Items = items

	return r, nil
}
