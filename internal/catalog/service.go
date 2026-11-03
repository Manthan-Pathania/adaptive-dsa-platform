package catalog

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/google/uuid"
)

// QuestionImporter defines the spreadsheet parsing & validation operations.
type QuestionImporter interface {
	Preview(ctx context.Context, opts ImportOptions) (*ImportPreviewResult, error)
	Execute(ctx context.Context, opts ImportOptions) (*ImportResult, error)
}

// Service defines high-level catalog domain operations.
type Service interface {
	ListQuestions(ctx context.Context, filter ListQuestionsFilter) ([]Question, int64, error)
	GetQuestion(ctx context.Context, idOrSlug string) (*Question, error)
	ListTopics(ctx context.Context) ([]Topic, error)
	ListPatterns(ctx context.Context) ([]Pattern, error)
	ListRoadmaps(ctx context.Context) ([]Roadmap, error)
	ListRoadmapsForUser(ctx context.Context, userID *uuid.UUID) ([]Roadmap, error)
	GetRoadmap(ctx context.Context, idOrSlug string) (*Roadmap, error)
	GetPatternsProgress(ctx context.Context, userID uuid.UUID) ([]PatternProgressItem, error)
	CreateUserRoadmap(ctx context.Context, r *Roadmap, items []RoadmapItem) (*Roadmap, error)
	DeleteRoadmap(ctx context.Context, userID uuid.UUID, userRole string, roadmapID uuid.UUID) error
	ImportRoadmapFromExcel(ctx context.Context, userID uuid.UUID, params ImportRoadmapParams, reader io.Reader) (*Roadmap, error)
	GenerateRoadmapTemplate(ctx context.Context) ([]byte, error)

	// Admin Question Management
	CreateQuestion(ctx context.Context, p CreateQuestionParams) (*Question, error)
	UpdateQuestion(ctx context.Context, id uuid.UUID, p UpdateQuestionParams) (*Question, error)
	SetQuestionActive(ctx context.Context, id uuid.UUID, isActive bool) error

	// Bulk Import Management
	PreviewImport(ctx context.Context, reader io.Reader, filePath string) (*ImportPreviewResult, error)
	CommitImport(ctx context.Context, reader io.Reader, filePath string, filename string, adminID *uuid.UUID, appEnv string) (*ImportResult, error)
	ListImportHistory(ctx context.Context, limit, offset int32) ([]ImportHistory, error)
}

type service struct {
	repo     Repository
	importer QuestionImporter
}

// NewService instantiates a catalog domain service with optional importer.
func NewService(repo Repository, imp ...QuestionImporter) Service {
	var importer QuestionImporter
	if len(imp) > 0 {
		importer = imp[0]
	}
	return &service{
		repo:     repo,
		importer: importer,
	}
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

func (s *service) ListPatterns(ctx context.Context) ([]Pattern, error) {
	return s.repo.ListPatterns(ctx)
}

func (s *service) ListRoadmaps(ctx context.Context) ([]Roadmap, error) {
	return s.repo.ListCuratedRoadmaps(ctx)
}

func (s *service) ListRoadmapsForUser(ctx context.Context, userID *uuid.UUID) ([]Roadmap, error) {
	return s.repo.ListRoadmapsForUser(ctx, userID)
}

func (s *service) GetPatternsProgress(ctx context.Context, userID uuid.UUID) ([]PatternProgressItem, error) {
	return s.repo.GetPatternsProgress(ctx, userID)
}

func (s *service) CreateUserRoadmap(ctx context.Context, r *Roadmap, items []RoadmapItem) (*Roadmap, error) {
	var createdRoadmap *Roadmap
	err := s.repo.ExecTx(ctx, func(txRepo Repository) error {
		created, err := txRepo.CreateRoadmap(ctx, r)
		if err != nil {
			return err
		}
		for _, it := range items {
			if err := txRepo.AddRoadmapItem(ctx, created.ID, it.QuestionID, it.StageNumber, it.OrderIndex, it.IsMandatory); err != nil {
				return err
			}
		}
		createdRoadmap = created
		return nil
	})
	if err != nil {
		return nil, err
	}
	createdRoadmap.Items = items
	return createdRoadmap, nil
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

func (s *service) DeleteRoadmap(ctx context.Context, userID uuid.UUID, userRole string, roadmapID uuid.UUID) error {
	r, err := s.repo.GetRoadmapByID(ctx, roadmapID)
	if err != nil {
		return err
	}

	if r.IsSystemTemplate {
		return ErrCannotDeleteSystemTemplate
	}

	if userRole != "ADMIN" {
		if r.UserID == nil || *r.UserID != userID {
			return ErrUnauthorizedRoadmapAction
		}
	}

	return s.repo.DeleteRoadmap(ctx, roadmapID)
}

func (s *service) CreateQuestion(ctx context.Context, p CreateQuestionParams) (*Question, error) {
	if p.Title == "" {
		return nil, errors.New("title cannot be empty")
	}
	if !p.Difficulty.IsValid() {
		return nil, errors.New("invalid difficulty")
	}

	slug := p.Slug
	if slug == "" {
		slug = Slugify(p.Title)
	}

	rawURL := p.ExternalURL
	if rawURL == "" {
		rawURL = fmt.Sprintf("https://leetcode.com/problems/%s/", slug)
	}
	cleanURL, err := NormalizeExternalURL(rawURL)
	if err != nil {
		return nil, fmt.Errorf("invalid external url: %w", err)
	}

	timeLimit := p.SampleTimeLimitMins
	if timeLimit <= 0 {
		timeLimit = 30
	}

	q := &Question{
		Slug:                slug,
		Title:               p.Title,
		ExternalURL:         cleanURL,
		Difficulty:          p.Difficulty,
		IsCore:              p.IsCore,
		IsActive:            true,
		LeetCodeID:          p.LeetCodeID,
		EditorialSummary:    p.EditorialSummary,
		SampleTimeLimitMins: timeLimit,
	}

	var created *Question
	err = s.repo.ExecTx(ctx, func(txRepo Repository) error {
		var txErr error
		created, txErr = txRepo.UpsertQuestion(ctx, q)
		if txErr != nil {
			return txErr
		}

		for i, patID := range p.PatternIDs {
			isPrimary := (i == 0)
			if err := txRepo.LinkQuestionPattern(ctx, created.ID, patID, isPrimary); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("create question: %w", err)
	}

	patterns, _ := s.repo.GetQuestionPatterns(ctx, created.ID)
	created.Patterns = patterns
	return created, nil
}

func (s *service) UpdateQuestion(ctx context.Context, id uuid.UUID, p UpdateQuestionParams) (*Question, error) {
	if p.Title == "" {
		return nil, errors.New("title cannot be empty")
	}
	if !p.Difficulty.IsValid() {
		return nil, errors.New("invalid difficulty")
	}

	var updated *Question
	err := s.repo.ExecTx(ctx, func(txRepo Repository) error {
		var txErr error
		updated, txErr = txRepo.UpdateQuestion(ctx, id, p)
		if txErr != nil {
			return txErr
		}

		if len(p.PatternIDs) > 0 {
			if err := txRepo.RemoveQuestionPatterns(ctx, id); err != nil {
				return err
			}
			for i, patID := range p.PatternIDs {
				isPrimary := (i == 0)
				if err := txRepo.LinkQuestionPattern(ctx, id, patID, isPrimary); err != nil {
					return err
				}
			}
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("update question: %w", err)
	}

	patterns, _ := s.repo.GetQuestionPatterns(ctx, updated.ID)
	updated.Patterns = patterns
	return updated, nil
}

func (s *service) SetQuestionActive(ctx context.Context, id uuid.UUID, isActive bool) error {
	return s.repo.SetQuestionActive(ctx, id, isActive)
}

func (s *service) PreviewImport(ctx context.Context, reader io.Reader, filePath string) (*ImportPreviewResult, error) {
	if s.importer == nil {
		return nil, errors.New("question importer service not configured")
	}
	return s.importer.Preview(ctx, ImportOptions{
		Reader:   reader,
		FilePath: filePath,
	})
}

func (s *service) CommitImport(ctx context.Context, reader io.Reader, filePath string, filename string, adminID *uuid.UUID, appEnv string) (*ImportResult, error) {
	if s.importer == nil {
		return nil, errors.New("question importer service not configured")
	}

	res, err := s.importer.Execute(ctx, ImportOptions{
		Reader:      reader,
		FilePath:    filePath,
		DryRun:      false,
		Environment: appEnv,
		ConfirmProd: true,
	})
	if err != nil {
		return res, err
	}

	// Persist record in import history
	status := "COMPLETED"
	if res.InvalidRows > 0 {
		status = "PARTIAL_SUCCESS"
	}

	_, _ = s.repo.RecordImportHistory(ctx, &ImportHistory{
		Filename:           filename,
		TotalRows:          int32(res.TotalRowsRead),
		ValidRows:          int32(res.ValidRows),
		CreatedQuestions:   int32(res.CreatedQuestions),
		UpdatedQuestions:   int32(res.UpdatedQuestions),
		UnchangedQuestions: int32(res.UnchangedQuestions),
		InvalidRows:        int32(res.InvalidRows),
		Status:             status,
		ImportedBy:         adminID,
	})

	return res, nil
}

func (s *service) ListImportHistory(ctx context.Context, limit, offset int32) ([]ImportHistory, error) {
	if limit <= 0 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}
	return s.repo.ListImportHistory(ctx, limit, offset)
}
