package catalog

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/adaptive-dsa/platform/internal/database/generated"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Repository defines data access operations for the catalog domain.
type Repository interface {
	CreateTopic(ctx context.Context, t *Topic) (*Topic, error)
	GetTopicBySlug(ctx context.Context, slug string) (*Topic, error)
	GetTopicByName(ctx context.Context, name string) (*Topic, error)
	ListTopics(ctx context.Context) ([]Topic, error)

	CreatePattern(ctx context.Context, p *Pattern) (*Pattern, error)
	GetPatternBySlug(ctx context.Context, slug string) (*Pattern, error)
	GetPatternByName(ctx context.Context, name string) (*Pattern, error)
	ListPatterns(ctx context.Context) ([]Pattern, error)
	ListPatternsByTopic(ctx context.Context, topicID uuid.UUID) ([]Pattern, error)

	UpsertQuestion(ctx context.Context, q *Question) (*Question, error)
	GetQuestionByID(ctx context.Context, id uuid.UUID) (*Question, error)
	GetQuestionBySlug(ctx context.Context, slug string) (*Question, error)
	GetQuestionByExternalURL(ctx context.Context, externalURL string) (*Question, error)
	FindQuestionForImport(ctx context.Context, title, leetcodeID, slug string) (*Question, error)
	ListQuestions(ctx context.Context, filter ListQuestionsFilter) ([]Question, int64, error)
	UpdateQuestion(ctx context.Context, id uuid.UUID, p UpdateQuestionParams) (*Question, error)
	SetQuestionActive(ctx context.Context, id uuid.UUID, isActive bool) error
	RemoveQuestionPatterns(ctx context.Context, questionID uuid.UUID) error

	LinkQuestionPattern(ctx context.Context, questionID, patternID uuid.UUID, isPrimary bool) error
	GetQuestionPatterns(ctx context.Context, questionID uuid.UUID) ([]Pattern, error)

	RecordImportHistory(ctx context.Context, h *ImportHistory) (*ImportHistory, error)
	ListImportHistory(ctx context.Context, limit, offset int32) ([]ImportHistory, error)

	CreateCompany(ctx context.Context, name, slug string) (uuid.UUID, error)
	GetCompanyByName(ctx context.Context, name string) (uuid.UUID, error)
	LinkQuestionCompany(ctx context.Context, questionID, companyID uuid.UUID, source, confidence string) error
	GetQuestionCompanies(ctx context.Context, questionID uuid.UUID) ([]string, error)

	CreateRoadmap(ctx context.Context, r *Roadmap) (*Roadmap, error)
	GetRoadmapBySlug(ctx context.Context, slug string) (*Roadmap, error)
	GetRoadmapByID(ctx context.Context, id uuid.UUID) (*Roadmap, error)
	ListCuratedRoadmaps(ctx context.Context) ([]Roadmap, error)
	ListRoadmapsForUser(ctx context.Context, userID *uuid.UUID) ([]Roadmap, error)
	DeleteRoadmap(ctx context.Context, id uuid.UUID) error
	AddRoadmapItem(ctx context.Context, roadmapID, questionID uuid.UUID, stageNumber, orderIndex int32, isMandatory bool) error
	ListRoadmapItems(ctx context.Context, roadmapID uuid.UUID) ([]RoadmapItem, error)
	GetPatternsProgress(ctx context.Context, userID uuid.UUID) ([]PatternProgressItem, error)

	ExecTx(ctx context.Context, fn func(repo Repository) error) error
}

type pgxRepository struct {
	pool    *pgxpool.Pool
	queries *generated.Queries
	q       generated.Querier
}

// NewPostgresRepository creates a production Postgres repository backed by pgxpool.
func NewPostgresRepository(pool *pgxpool.Pool) Repository {
	queries := generated.New(pool)
	return &pgxRepository{
		pool:    pool,
		queries: queries,
		q:       queries,
	}
}

func toUUID(p pgtype.UUID) uuid.UUID {
	return uuid.UUID(p.Bytes)
}

func toPgUUID(u uuid.UUID) pgtype.UUID {
	if u == uuid.Nil {
		return pgtype.UUID{Valid: false}
	}
	return pgtype.UUID{Bytes: u, Valid: true}
}

func (r *pgxRepository) CreateTopic(ctx context.Context, t *Topic) (*Topic, error) {
	var desc pgtype.Text
	if t.Description != nil {
		desc = pgtype.Text{String: *t.Description, Valid: true}
	}
	res, err := r.q.CreateTopic(ctx, generated.CreateTopicParams{
		Name:         t.Name,
		Slug:         t.Slug,
		Description:  desc,
		DisplayOrder: t.DisplayOrder,
	})
	if err != nil {
		return nil, fmt.Errorf("create topic: %w", err)
	}
	var resDesc *string
	if res.Description.Valid {
		resDesc = &res.Description.String
	}
	return &Topic{
		ID:           toUUID(res.ID),
		Name:         res.Name,
		Slug:         res.Slug,
		Description:  resDesc,
		DisplayOrder: res.DisplayOrder,
		CreatedAt:    res.CreatedAt.Time,
	}, nil
}

func (r *pgxRepository) GetTopicBySlug(ctx context.Context, slug string) (*Topic, error) {
	res, err := r.q.GetTopicBySlug(ctx, slug)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrTopicNotFound
		}
		return nil, err
	}
	var desc *string
	if res.Description.Valid {
		desc = &res.Description.String
	}
	return &Topic{
		ID:           toUUID(res.ID),
		Name:         res.Name,
		Slug:         res.Slug,
		Description:  desc,
		DisplayOrder: res.DisplayOrder,
		CreatedAt:    res.CreatedAt.Time,
	}, nil
}

func (r *pgxRepository) GetTopicByName(ctx context.Context, name string) (*Topic, error) {
	res, err := r.q.GetTopicByName(ctx, name)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrTopicNotFound
		}
		return nil, err
	}
	var desc *string
	if res.Description.Valid {
		desc = &res.Description.String
	}
	return &Topic{
		ID:           toUUID(res.ID),
		Name:         res.Name,
		Slug:         res.Slug,
		Description:  desc,
		DisplayOrder: res.DisplayOrder,
		CreatedAt:    res.CreatedAt.Time,
	}, nil
}

func (r *pgxRepository) ListTopics(ctx context.Context) ([]Topic, error) {
	rows, err := r.q.ListTopics(ctx)
	if err != nil {
		return nil, err
	}
	topics := make([]Topic, len(rows))
	for i, row := range rows {
		var desc *string
		if row.Description.Valid {
			desc = &row.Description.String
		}
		topics[i] = Topic{
			ID:           toUUID(row.ID),
			Name:         row.Name,
			Slug:         row.Slug,
			Description:  desc,
			DisplayOrder: row.DisplayOrder,
			CreatedAt:    row.CreatedAt.Time,
		}
	}
	return topics, nil
}

func (r *pgxRepository) CreatePattern(ctx context.Context, p *Pattern) (*Pattern, error) {
	var topicID pgtype.UUID
	if p.TopicID != nil {
		topicID = pgtype.UUID{Bytes: *p.TopicID, Valid: true}
	}
	var parentID pgtype.UUID
	if p.ParentID != nil {
		parentID = pgtype.UUID{Bytes: *p.ParentID, Valid: true}
	}
	var desc pgtype.Text
	if p.Description != nil {
		desc = pgtype.Text{String: *p.Description, Valid: true}
	}

	res, err := r.q.CreatePattern(ctx, generated.CreatePatternParams{
		TopicID:      topicID,
		ParentID:     parentID,
		Name:         p.Name,
		Slug:         p.Slug,
		Description:  desc,
		DisplayOrder: p.DisplayOrder,
	})
	if err != nil {
		return nil, fmt.Errorf("create pattern: %w", err)
	}

	var resDesc *string
	if res.Description.Valid {
		resDesc = &res.Description.String
	}
	var resTopicID *uuid.UUID
	if res.TopicID.Valid {
		u := uuid.UUID(res.TopicID.Bytes)
		resTopicID = &u
	}
	var resParentID *uuid.UUID
	if res.ParentID.Valid {
		u := uuid.UUID(res.ParentID.Bytes)
		resParentID = &u
	}

	return &Pattern{
		ID:           toUUID(res.ID),
		TopicID:      resTopicID,
		ParentID:     resParentID,
		Name:         res.Name,
		Slug:         res.Slug,
		Description:  resDesc,
		DisplayOrder: res.DisplayOrder,
		CreatedAt:    res.CreatedAt.Time,
	}, nil
}

func (r *pgxRepository) GetPatternBySlug(ctx context.Context, slug string) (*Pattern, error) {
	res, err := r.q.GetPatternBySlug(ctx, slug)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrPatternNotFound
		}
		return nil, err
	}
	var desc *string
	if res.Description.Valid {
		desc = &res.Description.String
	}
	var resTopicID *uuid.UUID
	if res.TopicID.Valid {
		u := uuid.UUID(res.TopicID.Bytes)
		resTopicID = &u
	}
	var resParentID *uuid.UUID
	if res.ParentID.Valid {
		u := uuid.UUID(res.ParentID.Bytes)
		resParentID = &u
	}
	return &Pattern{
		ID:           toUUID(res.ID),
		TopicID:      resTopicID,
		ParentID:     resParentID,
		Name:         res.Name,
		Slug:         res.Slug,
		Description:  desc,
		DisplayOrder: res.DisplayOrder,
		CreatedAt:    res.CreatedAt.Time,
	}, nil
}

func (r *pgxRepository) GetPatternByName(ctx context.Context, name string) (*Pattern, error) {
	res, err := r.q.GetPatternByName(ctx, name)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrPatternNotFound
		}
		return nil, err
	}
	var desc *string
	if res.Description.Valid {
		desc = &res.Description.String
	}
	var resTopicID *uuid.UUID
	if res.TopicID.Valid {
		u := uuid.UUID(res.TopicID.Bytes)
		resTopicID = &u
	}
	var resParentID *uuid.UUID
	if res.ParentID.Valid {
		u := uuid.UUID(res.ParentID.Bytes)
		resParentID = &u
	}
	return &Pattern{
		ID:           toUUID(res.ID),
		TopicID:      resTopicID,
		ParentID:     resParentID,
		Name:         res.Name,
		Slug:         res.Slug,
		Description:  desc,
		DisplayOrder: res.DisplayOrder,
		CreatedAt:    res.CreatedAt.Time,
	}, nil
}

func (r *pgxRepository) ListPatterns(ctx context.Context) ([]Pattern, error) {
	rows, err := r.q.ListPatterns(ctx)
	if err != nil {
		return nil, err
	}
	patterns := make([]Pattern, len(rows))
	for i, res := range rows {
		var desc *string
		if res.Description.Valid {
			desc = &res.Description.String
		}
		var resTopicID *uuid.UUID
		if res.TopicID.Valid {
			u := uuid.UUID(res.TopicID.Bytes)
			resTopicID = &u
		}
		var resParentID *uuid.UUID
		if res.ParentID.Valid {
			u := uuid.UUID(res.ParentID.Bytes)
			resParentID = &u
		}
		patterns[i] = Pattern{
			ID:           toUUID(res.ID),
			TopicID:      resTopicID,
			ParentID:     resParentID,
			Name:         res.Name,
			Slug:         res.Slug,
			Description:  desc,
			DisplayOrder: res.DisplayOrder,
			CreatedAt:    res.CreatedAt.Time,
		}
	}
	return patterns, nil
}

func (r *pgxRepository) ListPatternsByTopic(ctx context.Context, topicID uuid.UUID) ([]Pattern, error) {
	rows, err := r.q.ListPatternsByTopic(ctx, pgtype.UUID{Bytes: topicID, Valid: true})
	if err != nil {
		return nil, err
	}
	patterns := make([]Pattern, len(rows))
	for i, res := range rows {
		var desc *string
		if res.Description.Valid {
			desc = &res.Description.String
		}
		var resTopicID *uuid.UUID
		if res.TopicID.Valid {
			u := uuid.UUID(res.TopicID.Bytes)
			resTopicID = &u
		}
		var resParentID *uuid.UUID
		if res.ParentID.Valid {
			u := uuid.UUID(res.ParentID.Bytes)
			resParentID = &u
		}
		patterns[i] = Pattern{
			ID:           toUUID(res.ID),
			TopicID:      resTopicID,
			ParentID:     resParentID,
			Name:         res.Name,
			Slug:         res.Slug,
			Description:  desc,
			DisplayOrder: res.DisplayOrder,
			CreatedAt:    res.CreatedAt.Time,
		}
	}
	return patterns, nil
}

func (r *pgxRepository) UpsertQuestion(ctx context.Context, q *Question) (*Question, error) {
	var edSummary pgtype.Text
	if q.EditorialSummary != nil {
		edSummary = pgtype.Text{String: *q.EditorialSummary, Valid: true}
	}
	var leetcodeID pgtype.Text
	if q.LeetCodeID != nil && *q.LeetCodeID != "" {
		leetcodeID = pgtype.Text{String: *q.LeetCodeID, Valid: true}
	}

	res, err := r.q.UpsertQuestion(ctx, generated.UpsertQuestionParams{
		Slug:                q.Slug,
		Title:               q.Title,
		ExternalUrl:         q.ExternalURL,
		Difficulty:          generated.QuestionDifficulty(q.Difficulty),
		IsCore:              q.IsCore,
		EditorialSummary:    edSummary,
		SampleTimeLimitMins: q.SampleTimeLimitMins,
		LeetcodeID:          leetcodeID,
		IsActive:            q.IsActive,
	})
	if err != nil {
		return nil, fmt.Errorf("upsert question: %w", err)
	}

	var resSummary *string
	if res.EditorialSummary.Valid {
		resSummary = &res.EditorialSummary.String
	}
	var resLeetCodeID *string
	if res.LeetcodeID.Valid {
		resLeetCodeID = &res.LeetcodeID.String
	}

	return &Question{
		ID:                  toUUID(res.ID),
		Slug:                res.Slug,
		Title:               res.Title,
		ExternalURL:         res.ExternalUrl,
		Difficulty:          Difficulty(res.Difficulty),
		IsCore:              res.IsCore,
		IsActive:            res.IsActive,
		LeetCodeID:          resLeetCodeID,
		EditorialSummary:    resSummary,
		SampleTimeLimitMins: res.SampleTimeLimitMins,
		CreatedAt:           res.CreatedAt.Time,
		UpdatedAt:           res.UpdatedAt.Time,
	}, nil
}

func (r *pgxRepository) GetQuestionByID(ctx context.Context, id uuid.UUID) (*Question, error) {
	res, err := r.q.GetQuestionByID(ctx, toPgUUID(id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrQuestionNotFound
		}
		return nil, err
	}
	var resSummary *string
	if res.EditorialSummary.Valid {
		resSummary = &res.EditorialSummary.String
	}
	var resLeetCodeID *string
	if res.LeetcodeID.Valid {
		resLeetCodeID = &res.LeetcodeID.String
	}
	return &Question{
		ID:                  toUUID(res.ID),
		Slug:                res.Slug,
		Title:               res.Title,
		ExternalURL:         res.ExternalUrl,
		Difficulty:          Difficulty(res.Difficulty),
		IsCore:              res.IsCore,
		IsActive:            res.IsActive,
		LeetCodeID:          resLeetCodeID,
		EditorialSummary:    resSummary,
		SampleTimeLimitMins: res.SampleTimeLimitMins,
		ExcelOrder:          res.ExcelOrder,
		CreatedAt:           res.CreatedAt.Time,
		UpdatedAt:           res.UpdatedAt.Time,
	}, nil
}

func (r *pgxRepository) GetQuestionBySlug(ctx context.Context, slug string) (*Question, error) {
	res, err := r.q.GetQuestionBySlug(ctx, slug)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrQuestionNotFound
		}
		return nil, err
	}
	var resSummary *string
	if res.EditorialSummary.Valid {
		resSummary = &res.EditorialSummary.String
	}
	var resLeetCodeID *string
	if res.LeetcodeID.Valid {
		resLeetCodeID = &res.LeetcodeID.String
	}
	return &Question{
		ID:                  toUUID(res.ID),
		Slug:                res.Slug,
		Title:               res.Title,
		ExternalURL:         res.ExternalUrl,
		Difficulty:          Difficulty(res.Difficulty),
		IsCore:              res.IsCore,
		IsActive:            res.IsActive,
		LeetCodeID:          resLeetCodeID,
		EditorialSummary:    resSummary,
		SampleTimeLimitMins: res.SampleTimeLimitMins,
		ExcelOrder:          res.ExcelOrder,
		CreatedAt:           res.CreatedAt.Time,
		UpdatedAt:           res.UpdatedAt.Time,
	}, nil
}

func (r *pgxRepository) GetQuestionByExternalURL(ctx context.Context, externalURL string) (*Question, error) {
	res, err := r.q.GetQuestionByExternalURL(ctx, externalURL)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrQuestionNotFound
		}
		return nil, err
	}
	var resSummary *string
	if res.EditorialSummary.Valid {
		resSummary = &res.EditorialSummary.String
	}
	var resLeetCodeID *string
	if res.LeetcodeID.Valid {
		resLeetCodeID = &res.LeetcodeID.String
	}
	return &Question{
		ID:                  toUUID(res.ID),
		Slug:                res.Slug,
		Title:               res.Title,
		ExternalURL:         res.ExternalUrl,
		Difficulty:          Difficulty(res.Difficulty),
		IsCore:              res.IsCore,
		IsActive:            res.IsActive,
		LeetCodeID:          resLeetCodeID,
		EditorialSummary:    resSummary,
		SampleTimeLimitMins: res.SampleTimeLimitMins,
		ExcelOrder:          res.ExcelOrder,
		CreatedAt:           res.CreatedAt.Time,
		UpdatedAt:           res.UpdatedAt.Time,
	}, nil
}

func (r *pgxRepository) FindQuestionForImport(ctx context.Context, title, leetcodeID, slug string) (*Question, error) {
	query := `
		SELECT id, slug, title, external_url, difficulty, is_core, is_active, leetcode_id, editorial_summary, sample_time_limit_mins, excel_order, created_at, updated_at
		FROM questions
		WHERE LOWER(title) = LOWER($1)
		   OR (NULLIF($2, '') IS NOT NULL AND leetcode_id = $2)
		   OR (NULLIF($3, '') IS NOT NULL AND slug = $3)
		LIMIT 1;
	`
	var q Question
	var diff string
	var resLC pgtype.Text
	var resSummary pgtype.Text
	var resCreatedAt pgtype.Timestamptz
	var resUpdatedAt pgtype.Timestamptz
	err := r.pool.QueryRow(ctx, query, title, leetcodeID, slug).Scan(
		&q.ID,
		&q.Slug,
		&q.Title,
		&q.ExternalURL,
		&diff,
		&q.IsCore,
		&q.IsActive,
		&resLC,
		&resSummary,
		&q.SampleTimeLimitMins,
		&q.ExcelOrder,
		&resCreatedAt,
		&resUpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrQuestionNotFound
		}
		return nil, err
	}
	q.Difficulty = Difficulty(diff)
	if resLC.Valid {
		q.LeetCodeID = &resLC.String
	}
	if resSummary.Valid {
		q.EditorialSummary = &resSummary.String
	}
	q.CreatedAt = resCreatedAt.Time
	q.UpdatedAt = resUpdatedAt.Time
	return &q, nil
}

func (r *pgxRepository) ListQuestions(ctx context.Context, filter ListQuestionsFilter) ([]Question, int64, error) {
	var topicPatternUUID pgtype.UUID
	if filter.TopicOrPatternID != nil {
		topicPatternUUID = pgtype.UUID{Bytes: *filter.TopicOrPatternID, Valid: true}
	}
	var isActive pgtype.Bool
	if filter.IsActive != nil {
		isActive = pgtype.Bool{Bool: *filter.IsActive, Valid: true}
	}

	rows, err := r.q.ListQuestions(ctx, generated.ListQuestionsParams{
		Column1:  filter.SearchQuery,
		Column2:  filter.Difficulty,
		Column3:  topicPatternUUID,
		Limit:    filter.Limit,
		Offset:   filter.Offset,
		IsActive: isActive,
	})
	if err != nil {
		return nil, 0, err
	}

	count, err := r.q.CountQuestions(ctx, generated.CountQuestionsParams{
		Column1:  filter.SearchQuery,
		Column2:  filter.Difficulty,
		Column3:  topicPatternUUID,
		IsActive: isActive,
	})
	if err != nil {
		return nil, 0, err
	}

	questions := make([]Question, len(rows))
	for i, res := range rows {
		var resSummary *string
		if res.EditorialSummary.Valid {
			resSummary = &res.EditorialSummary.String
		}
		var resLeetCodeID *string
		if res.LeetcodeID.Valid {
			resLeetCodeID = &res.LeetcodeID.String
		}
		questions[i] = Question{
			ID:                  toUUID(res.ID),
			Slug:                res.Slug,
			Title:               res.Title,
			ExternalURL:         res.ExternalUrl,
			Difficulty:          Difficulty(res.Difficulty),
			IsCore:              res.IsCore,
			IsActive:            res.IsActive,
			LeetCodeID:          resLeetCodeID,
			EditorialSummary:    resSummary,
			SampleTimeLimitMins: res.SampleTimeLimitMins,
			ExcelOrder:          res.ExcelOrder,
			CreatedAt:           res.CreatedAt.Time,
			UpdatedAt:           res.UpdatedAt.Time,
		}
	}
	return questions, count, nil
}

func (r *pgxRepository) UpdateQuestion(ctx context.Context, id uuid.UUID, p UpdateQuestionParams) (*Question, error) {
	var edSummary pgtype.Text
	if p.EditorialSummary != nil {
		edSummary = pgtype.Text{String: *p.EditorialSummary, Valid: true}
	}
	var leetcodeID pgtype.Text
	if p.LeetCodeID != nil && *p.LeetCodeID != "" {
		leetcodeID = pgtype.Text{String: *p.LeetCodeID, Valid: true}
	}

	res, err := r.q.UpdateQuestion(ctx, generated.UpdateQuestionParams{
		ID:                  toPgUUID(id),
		Title:               p.Title,
		Difficulty:          generated.QuestionDifficulty(p.Difficulty),
		IsCore:              p.IsCore,
		EditorialSummary:    edSummary,
		SampleTimeLimitMins: p.SampleTimeLimitMins,
		LeetcodeID:          leetcodeID,
	})
	if err != nil {
		return nil, fmt.Errorf("update question: %w", err)
	}

	var resSummary *string
	if res.EditorialSummary.Valid {
		resSummary = &res.EditorialSummary.String
	}
	var resLeetCodeID *string
	if res.LeetcodeID.Valid {
		resLeetCodeID = &res.LeetcodeID.String
	}

	return &Question{
		ID:                  toUUID(res.ID),
		Slug:                res.Slug,
		Title:               res.Title,
		ExternalURL:         res.ExternalUrl,
		Difficulty:          Difficulty(res.Difficulty),
		IsCore:              res.IsCore,
		IsActive:            res.IsActive,
		LeetCodeID:          resLeetCodeID,
		EditorialSummary:    resSummary,
		SampleTimeLimitMins: res.SampleTimeLimitMins,
		CreatedAt:           res.CreatedAt.Time,
		UpdatedAt:           res.UpdatedAt.Time,
	}, nil
}

func (r *pgxRepository) SetQuestionActive(ctx context.Context, id uuid.UUID, isActive bool) error {
	return r.q.SetQuestionActive(ctx, generated.SetQuestionActiveParams{
		ID:       toPgUUID(id),
		IsActive: isActive,
	})
}

func (r *pgxRepository) RemoveQuestionPatterns(ctx context.Context, questionID uuid.UUID) error {
	return r.q.RemoveQuestionPatterns(ctx, toPgUUID(questionID))
}

func (r *pgxRepository) RecordImportHistory(ctx context.Context, h *ImportHistory) (*ImportHistory, error) {
	var importedBy pgtype.UUID
	if h.ImportedBy != nil {
		importedBy = toPgUUID(*h.ImportedBy)
	}
	res, err := r.q.RecordImportHistory(ctx, generated.RecordImportHistoryParams{
		Filename:           h.Filename,
		TotalRows:          h.TotalRows,
		ValidRows:          h.ValidRows,
		CreatedQuestions:   h.CreatedQuestions,
		UpdatedQuestions:   h.UpdatedQuestions,
		UnchangedQuestions: h.UnchangedQuestions,
		InvalidRows:        h.InvalidRows,
		Status:             h.Status,
		ImportedBy:         importedBy,
	})
	if err != nil {
		return nil, fmt.Errorf("record import history: %w", err)
	}

	var resImportedBy *uuid.UUID
	if res.ImportedBy.Valid {
		u := toUUID(res.ImportedBy)
		resImportedBy = &u
	}

	return &ImportHistory{
		ID:                 toUUID(res.ID),
		Filename:           res.Filename,
		TotalRows:          res.TotalRows,
		ValidRows:          res.ValidRows,
		CreatedQuestions:   res.CreatedQuestions,
		UpdatedQuestions:   res.UpdatedQuestions,
		UnchangedQuestions: res.UnchangedQuestions,
		InvalidRows:        res.InvalidRows,
		Status:             res.Status,
		ImportedBy:         resImportedBy,
		CreatedAt:          res.CreatedAt.Time,
	}, nil
}

func (r *pgxRepository) ListImportHistory(ctx context.Context, limit, offset int32) ([]ImportHistory, error) {
	rows, err := r.q.ListImportHistory(ctx, generated.ListImportHistoryParams{
		Limit:  limit,
		Offset: offset,
	})
	if err != nil {
		return nil, fmt.Errorf("list import history: %w", err)
	}

	history := make([]ImportHistory, len(rows))
	for i, res := range rows {
		var resImportedBy *uuid.UUID
		if res.ImportedBy.Valid {
			u := toUUID(res.ImportedBy)
			resImportedBy = &u
		}
		history[i] = ImportHistory{
			ID:                 toUUID(res.ID),
			Filename:           res.Filename,
			TotalRows:          res.TotalRows,
			ValidRows:          res.ValidRows,
			CreatedQuestions:   res.CreatedQuestions,
			UpdatedQuestions:   res.UpdatedQuestions,
			UnchangedQuestions: res.UnchangedQuestions,
			InvalidRows:        res.InvalidRows,
			Status:             res.Status,
			ImportedBy:         resImportedBy,
			CreatedAt:          res.CreatedAt.Time,
		}
	}
	return history, nil
}

func (r *pgxRepository) LinkQuestionPattern(ctx context.Context, questionID, patternID uuid.UUID, isPrimary bool) error {
	return r.q.LinkQuestionPattern(ctx, generated.LinkQuestionPatternParams{
		QuestionID: toPgUUID(questionID),
		PatternID:  toPgUUID(patternID),
		IsPrimary:  isPrimary,
	})
}

func (r *pgxRepository) GetQuestionPatterns(ctx context.Context, questionID uuid.UUID) ([]Pattern, error) {
	rows, err := r.q.GetQuestionPatterns(ctx, toPgUUID(questionID))
	if err != nil {
		return nil, err
	}
	patterns := make([]Pattern, len(rows))
	for i, res := range rows {
		var topicName, topicSlug *string
		if res.TopicName.Valid {
			topicName = &res.TopicName.String
		}
		if res.TopicSlug.Valid {
			topicSlug = &res.TopicSlug.String
		}
		var topicID *uuid.UUID
		if res.TopicID.Valid {
			u := uuid.UUID(res.TopicID.Bytes)
			topicID = &u
		}
		patterns[i] = Pattern{
			ID:        toUUID(res.ID),
			TopicID:   topicID,
			Name:      res.Name,
			Slug:      res.Slug,
			TopicName: topicName,
			TopicSlug: topicSlug,
		}
	}
	return patterns, nil
}

func (r *pgxRepository) CreateCompany(ctx context.Context, name, slug string) (uuid.UUID, error) {
	res, err := r.q.CreateCompany(ctx, generated.CreateCompanyParams{
		Name: name,
		Slug: slug,
	})
	if err != nil {
		return uuid.Nil, err
	}
	return toUUID(res.ID), nil
}

func (r *pgxRepository) GetCompanyByName(ctx context.Context, name string) (uuid.UUID, error) {
	res, err := r.q.GetCompanyByName(ctx, name)
	if err != nil {
		return uuid.Nil, err
	}
	return toUUID(res.ID), nil
}

func (r *pgxRepository) LinkQuestionCompany(ctx context.Context, questionID, companyID uuid.UUID, source, confidence string) error {
	conf := generated.EvidenceConfidence(confidence)
	if err := conf.Scan(confidence); err != nil {
		conf = generated.EvidenceConfidenceCONFIRMED
	}
	return r.q.LinkQuestionCompany(ctx, generated.LinkQuestionCompanyParams{
		QuestionID:        toPgUUID(questionID),
		CompanyID:         toPgUUID(companyID),
		SourceDescription: source,
		Confidence:        conf,
	})
}

func (r *pgxRepository) GetQuestionCompanies(ctx context.Context, questionID uuid.UUID) ([]string, error) {
	rows, err := r.q.GetQuestionCompanies(ctx, toPgUUID(questionID))
	if err != nil {
		return nil, err
	}
	companies := make([]string, len(rows))
	for i, row := range rows {
		companies[i] = row.Name
	}
	return companies, nil
}

func (r *pgxRepository) CreateRoadmap(ctx context.Context, rd *Roadmap) (*Roadmap, error) {
	var uid pgtype.UUID
	if rd.UserID != nil {
		uid = pgtype.UUID{Bytes: *rd.UserID, Valid: true}
	}
	var desc pgtype.Text
	if rd.Description != nil {
		desc = pgtype.Text{String: *rd.Description, Valid: true}
	}

	res, err := r.q.CreateRoadmap(ctx, generated.CreateRoadmapParams{
		UserID:           uid,
		Title:            rd.Title,
		Slug:             rd.Slug,
		Description:      desc,
		TargetDays:       rd.TargetDays,
		IsSystemTemplate: rd.IsSystemTemplate,
	})
	if err != nil {
		return nil, fmt.Errorf("create roadmap: %w", err)
	}

	var resDesc *string
	if res.Description.Valid {
		resDesc = &res.Description.String
	}
	var resUID *uuid.UUID
	if res.UserID.Valid {
		u := uuid.UUID(res.UserID.Bytes)
		resUID = &u
	}

	return &Roadmap{
		ID:               toUUID(res.ID),
		UserID:           resUID,
		Title:            res.Title,
		Slug:             res.Slug,
		Description:      resDesc,
		TargetDays:       res.TargetDays,
		IsSystemTemplate: res.IsSystemTemplate,
		CreatedAt:        res.CreatedAt.Time,
		UpdatedAt:        res.UpdatedAt.Time,
	}, nil
}

func (r *pgxRepository) GetRoadmapBySlug(ctx context.Context, slug string) (*Roadmap, error) {
	res, err := r.q.GetRoadmapBySlug(ctx, slug)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrRoadmapNotFound
		}
		return nil, err
	}
	var resDesc *string
	if res.Description.Valid {
		resDesc = &res.Description.String
	}
	var resUID *uuid.UUID
	if res.UserID.Valid {
		u := uuid.UUID(res.UserID.Bytes)
		resUID = &u
	}
	return &Roadmap{
		ID:               toUUID(res.ID),
		UserID:           resUID,
		Title:            res.Title,
		Slug:             res.Slug,
		Description:      resDesc,
		TargetDays:       res.TargetDays,
		IsSystemTemplate: res.IsSystemTemplate,
		CreatedAt:        res.CreatedAt.Time,
		UpdatedAt:        res.UpdatedAt.Time,
	}, nil
}

func (r *pgxRepository) GetRoadmapByID(ctx context.Context, id uuid.UUID) (*Roadmap, error) {
	res, err := r.q.GetRoadmapByID(ctx, toPgUUID(id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrRoadmapNotFound
		}
		return nil, err
	}
	var resDesc *string
	if res.Description.Valid {
		resDesc = &res.Description.String
	}
	var resUID *uuid.UUID
	if res.UserID.Valid {
		u := uuid.UUID(res.UserID.Bytes)
		resUID = &u
	}
	return &Roadmap{
		ID:               toUUID(res.ID),
		UserID:           resUID,
		Title:            res.Title,
		Slug:             res.Slug,
		Description:      resDesc,
		TargetDays:       res.TargetDays,
		IsSystemTemplate: res.IsSystemTemplate,
		CreatedAt:        res.CreatedAt.Time,
		UpdatedAt:        res.UpdatedAt.Time,
	}, nil
}

func (r *pgxRepository) ListCuratedRoadmaps(ctx context.Context) ([]Roadmap, error) {
	query := `
		SELECT r.id, r.user_id, r.title, r.slug, r.description, r.target_days, r.is_system_template,
		       COALESCE((SELECT COUNT(*) FROM roadmap_items ri WHERE ri.roadmap_id = r.id), 0)::int as total_questions,
		       r.created_at, r.updated_at
		FROM roadmaps r
		WHERE r.is_system_template = TRUE
		ORDER BY r.created_at ASC;
	`
	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("list curated roadmaps: %w", err)
	}
	defer rows.Close()

	var roadmaps []Roadmap
	for rows.Next() {
		var rd Roadmap
		var id, uID pgtype.UUID
		var desc pgtype.Text
		if err := rows.Scan(
			&id,
			&uID,
			&rd.Title,
			&rd.Slug,
			&desc,
			&rd.TargetDays,
			&rd.IsSystemTemplate,
			&rd.TotalQuestions,
			&rd.CreatedAt,
			&rd.UpdatedAt,
		); err != nil {
			return nil, err
		}
		rd.ID = toUUID(id)
		if uID.Valid {
			u := toUUID(uID)
			rd.UserID = &u
		}
		if desc.Valid {
			rd.Description = &desc.String
		}
		roadmaps = append(roadmaps, rd)
	}
	return roadmaps, nil
}

func (r *pgxRepository) ListRoadmapsForUser(ctx context.Context, userID *uuid.UUID) ([]Roadmap, error) {
	var uid pgtype.UUID
	if userID != nil {
		uid = pgtype.UUID{Bytes: *userID, Valid: true}
	}
	query := `
		SELECT r.id, r.user_id, r.title, r.slug, r.description, r.target_days, r.is_system_template,
		       COALESCE((SELECT COUNT(*) FROM roadmap_items ri WHERE ri.roadmap_id = r.id), 0)::int as total_questions,
		       r.created_at, r.updated_at
		FROM roadmaps r
		WHERE r.is_system_template = TRUE OR (r.user_id IS NOT NULL AND r.user_id = $1)
		ORDER BY r.is_system_template DESC, r.created_at DESC;
	`
	rows, err := r.pool.Query(ctx, query, uid)
	if err != nil {
		return nil, fmt.Errorf("list roadmaps for user: %w", err)
	}
	defer rows.Close()

	var roadmaps []Roadmap
	for rows.Next() {
		var rd Roadmap
		var id, uID pgtype.UUID
		var desc pgtype.Text
		if err := rows.Scan(
			&id,
			&uID,
			&rd.Title,
			&rd.Slug,
			&desc,
			&rd.TargetDays,
			&rd.IsSystemTemplate,
			&rd.TotalQuestions,
			&rd.CreatedAt,
			&rd.UpdatedAt,
		); err != nil {
			return nil, err
		}
		rd.ID = toUUID(id)
		if uID.Valid {
			u := toUUID(uID)
			rd.UserID = &u
		}
		if desc.Valid {
			rd.Description = &desc.String
		}
		roadmaps = append(roadmaps, rd)
	}
	return roadmaps, nil
}

func (r *pgxRepository) DeleteRoadmap(ctx context.Context, id uuid.UUID) error {
	tag, err := r.pool.Exec(ctx, "DELETE FROM roadmaps WHERE id = $1", id)
	if err != nil {
		return fmt.Errorf("delete roadmap: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrRoadmapNotFound
	}
	return nil
}

func (r *pgxRepository) GetPatternsProgress(ctx context.Context, userID uuid.UUID) ([]PatternProgressItem, error) {
	query := `
		SELECT 
			p.id,
			p.name as pattern_name,
			p.slug as pattern_slug,
			COALESCE(t.name, 'General') as topic_name,
			COUNT(DISTINCT qp.question_id)::int as total_questions,
			COUNT(DISTINCT CASE WHEN uqp.status IN ('SOLVED', 'MASTERED') THEN uqp.question_id END)::int as solved_count
		FROM patterns p
		LEFT JOIN topics t ON p.topic_id = t.id
		JOIN question_patterns qp ON p.id = qp.pattern_id
		JOIN questions q ON qp.question_id = q.id AND q.is_active = true
		LEFT JOIN user_question_progress uqp ON qp.question_id = uqp.question_id AND uqp.user_id = $1
		WHERE p.parent_id IS NULL
		GROUP BY p.id, p.name, p.slug, t.name, p.display_order
		ORDER BY p.display_order ASC, p.name ASC;
	`
	rows, err := r.pool.Query(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("get patterns progress: %w", err)
	}
	defer rows.Close()

	var items []PatternProgressItem
	for rows.Next() {
		var item PatternProgressItem
		var pid pgtype.UUID
		if err := rows.Scan(
			&pid,
			&item.PatternName,
			&item.PatternSlug,
			&item.TopicName,
			&item.TotalQuestions,
			&item.SolvedCount,
		); err != nil {
			return nil, err
		}
		item.PatternID = toUUID(pid)
		if item.TotalQuestions > 0 {
			item.Percent = float32(item.SolvedCount) / float32(item.TotalQuestions) * 100.0
		}
		items = append(items, item)
	}
	return items, nil
}

func (r *pgxRepository) AddRoadmapItem(ctx context.Context, roadmapID, questionID uuid.UUID, stageNumber, orderIndex int32, isMandatory bool) error {
	return r.q.AddRoadmapItem(ctx, generated.AddRoadmapItemParams{
		RoadmapID:   toPgUUID(roadmapID),
		QuestionID:  toPgUUID(questionID),
		StageNumber: stageNumber,
		OrderIndex:  orderIndex,
		IsMandatory: isMandatory,
	})
}

func (r *pgxRepository) ListRoadmapItems(ctx context.Context, roadmapID uuid.UUID) ([]RoadmapItem, error) {
	query := `
		SELECT ri.id, ri.roadmap_id, ri.question_id, ri.stage_number, ri.order_index, ri.is_mandatory,
		       q.title, q.slug as question_slug, q.difficulty, q.is_core,
		       q.excel_order,
		       COALESCE(
		           (SELECT p.name FROM question_patterns qp JOIN patterns p ON qp.pattern_id = p.id WHERE qp.question_id = q.id AND p.parent_id IS NULL LIMIT 1),
		           (SELECT parent.name FROM question_patterns qp JOIN patterns p ON qp.pattern_id = p.id JOIN patterns parent ON p.parent_id = parent.id WHERE qp.question_id = q.id AND parent.parent_id IS NULL LIMIT 1),
		           (SELECT p.name FROM question_patterns qp JOIN patterns p ON qp.pattern_id = p.id WHERE qp.question_id = q.id LIMIT 1),
		           'General'
		       ) AS pattern_name,
		       COALESCE(
		           (SELECT sp.name FROM question_patterns qp JOIN patterns sp ON qp.pattern_id = sp.id WHERE qp.question_id = q.id AND sp.parent_id IS NOT NULL LIMIT 1),
		           ''
		       ) AS subpattern_name
		FROM roadmap_items ri
		JOIN questions q ON ri.question_id = q.id
		WHERE ri.roadmap_id = $1
		ORDER BY ri.stage_number ASC, ri.order_index ASC;
	`
	rows, err := r.pool.Query(ctx, query, roadmapID)
	if err != nil {
		return nil, fmt.Errorf("list roadmap items: %w", err)
	}
	defer rows.Close()

	var items []RoadmapItem
	for rows.Next() {
		var it RoadmapItem
		var diff string
		if err := rows.Scan(
			&it.ID,
			&it.RoadmapID,
			&it.QuestionID,
			&it.StageNumber,
			&it.OrderIndex,
			&it.IsMandatory,
			&it.QuestionTitle,
			&it.QuestionSlug,
			&diff,
			&it.IsCore,
			&it.ExcelOrder,
			&it.PatternName,
			&it.SubpatternName,
		); err != nil {
			return nil, fmt.Errorf("scan roadmap item: %w", err)
		}
		it.Difficulty = Difficulty(diff)
		items = append(items, it)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

func (r *pgxRepository) ExecTx(ctx context.Context, fn func(repo Repository) error) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	txQueries := r.queries.WithTx(tx)
	txRepo := &pgxRepository{
		pool:    r.pool,
		queries: r.queries,
		q:       txQueries,
	}

	if err := fn(txRepo); err != nil {
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}
	return nil
}

// InMemoryRepository provides an in-memory repository for high-speed unit tests.
type InMemoryRepository struct {
	mu         sync.RWMutex
	txMu       sync.Mutex
	topics     map[uuid.UUID]Topic
	patterns   map[uuid.UUID]Pattern
	questions  map[uuid.UUID]Question
	qPatterns  map[uuid.UUID][]Pattern
	companies  map[uuid.UUID]string
	qCompanies map[uuid.UUID][]string
	roadmaps   map[uuid.UUID]Roadmap
	roadItems  map[uuid.UUID][]RoadmapItem
	history    []ImportHistory
}

func NewInMemoryRepository() *InMemoryRepository {
	return &InMemoryRepository{
		topics:     make(map[uuid.UUID]Topic),
		patterns:   make(map[uuid.UUID]Pattern),
		questions:  make(map[uuid.UUID]Question),
		qPatterns:  make(map[uuid.UUID][]Pattern),
		companies:  make(map[uuid.UUID]string),
		qCompanies: make(map[uuid.UUID][]string),
		roadmaps:   make(map[uuid.UUID]Roadmap),
		roadItems:  make(map[uuid.UUID][]RoadmapItem),
		history:    make([]ImportHistory, 0),
	}
}

func (m *InMemoryRepository) CreateTopic(ctx context.Context, t *Topic) (*Topic, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, existing := range m.topics {
		if strings.EqualFold(existing.Name, t.Name) {
			existing.Description = t.Description
			existing.DisplayOrder = t.DisplayOrder
			m.topics[existing.ID] = existing
			res := existing
			return &res, nil
		}
	}

	id := t.ID
	if id == uuid.Nil {
		id = uuid.New()
	}
	created := Topic{
		ID:           id,
		Name:         t.Name,
		Slug:         t.Slug,
		Description:  t.Description,
		DisplayOrder: t.DisplayOrder,
		CreatedAt:    time.Now().UTC(),
	}
	m.topics[id] = created
	return &created, nil
}

func (m *InMemoryRepository) GetTopicBySlug(ctx context.Context, slug string) (*Topic, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, t := range m.topics {
		if t.Slug == slug {
			res := t
			return &res, nil
		}
	}
	return nil, ErrTopicNotFound
}

func (m *InMemoryRepository) GetTopicByName(ctx context.Context, name string) (*Topic, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, t := range m.topics {
		if strings.EqualFold(t.Name, name) {
			res := t
			return &res, nil
		}
	}
	return nil, ErrTopicNotFound
}

func (m *InMemoryRepository) ListTopics(ctx context.Context) ([]Topic, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var list []Topic
	for _, t := range m.topics {
		list = append(list, t)
	}
	return list, nil
}

func (m *InMemoryRepository) CreatePattern(ctx context.Context, p *Pattern) (*Pattern, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, existing := range m.patterns {
		if existing.Slug == p.Slug {
			existing.Name = p.Name
			existing.Description = p.Description
			existing.DisplayOrder = p.DisplayOrder
			m.patterns[existing.ID] = existing
			res := existing
			return &res, nil
		}
	}

	id := p.ID
	if id == uuid.Nil {
		id = uuid.New()
	}
	created := Pattern{
		ID:           id,
		TopicID:      p.TopicID,
		ParentID:     p.ParentID,
		Name:         p.Name,
		Slug:         p.Slug,
		Description:  p.Description,
		DisplayOrder: p.DisplayOrder,
		CreatedAt:    time.Now().UTC(),
	}
	m.patterns[id] = created
	return &created, nil
}

func (m *InMemoryRepository) GetPatternBySlug(ctx context.Context, slug string) (*Pattern, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, p := range m.patterns {
		if p.Slug == slug {
			res := p
			return &res, nil
		}
	}
	return nil, ErrPatternNotFound
}

func (m *InMemoryRepository) GetPatternByName(ctx context.Context, name string) (*Pattern, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, p := range m.patterns {
		if strings.EqualFold(p.Name, name) {
			res := p
			return &res, nil
		}
	}
	return nil, ErrPatternNotFound
}

func (m *InMemoryRepository) ListPatterns(ctx context.Context) ([]Pattern, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var list []Pattern
	for _, p := range m.patterns {
		list = append(list, p)
	}
	return list, nil
}

func (m *InMemoryRepository) ListPatternsByTopic(ctx context.Context, topicID uuid.UUID) ([]Pattern, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var list []Pattern
	for _, p := range m.patterns {
		if p.TopicID != nil && *p.TopicID == topicID {
			list = append(list, p)
		}
	}
	return list, nil
}

func (m *InMemoryRepository) UpsertQuestion(ctx context.Context, q *Question) (*Question, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, existing := range m.questions {
		if existing.ExternalURL == q.ExternalURL {
			existing.Title = q.Title
			existing.Difficulty = q.Difficulty
			existing.IsCore = q.IsCore
			if q.LeetCodeID != nil {
				existing.LeetCodeID = q.LeetCodeID
			}
			existing.EditorialSummary = q.EditorialSummary
			existing.SampleTimeLimitMins = q.SampleTimeLimitMins
			existing.UpdatedAt = time.Now().UTC()
			m.questions[existing.ID] = existing
			res := existing
			return &res, nil
		}
	}

	id := q.ID
	if id == uuid.Nil {
		id = uuid.New()
	}
	isActive := q.IsActive
	// Default to active on creation unless explicitly false when ID was already set
	if !isActive {
		isActive = true
	}
	now := time.Now().UTC()
	created := Question{
		ID:                  id,
		Slug:                q.Slug,
		Title:               q.Title,
		ExternalURL:         q.ExternalURL,
		Difficulty:          q.Difficulty,
		IsCore:              q.IsCore,
		IsActive:            isActive,
		LeetCodeID:          q.LeetCodeID,
		EditorialSummary:    q.EditorialSummary,
		SampleTimeLimitMins: q.SampleTimeLimitMins,
		CreatedAt:           now,
		UpdatedAt:           now,
	}
	m.questions[id] = created
	return &created, nil
}

func (m *InMemoryRepository) GetQuestionByID(ctx context.Context, id uuid.UUID) (*Question, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	q, ok := m.questions[id]
	if !ok {
		return nil, ErrQuestionNotFound
	}
	res := q
	return &res, nil
}

func (m *InMemoryRepository) GetQuestionBySlug(ctx context.Context, slug string) (*Question, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, q := range m.questions {
		if q.Slug == slug {
			res := q
			return &res, nil
		}
	}
	return nil, ErrQuestionNotFound
}

func (m *InMemoryRepository) GetQuestionByExternalURL(ctx context.Context, url string) (*Question, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, q := range m.questions {
		if q.ExternalURL == url {
			res := q
			return &res, nil
		}
	}
	return nil, ErrQuestionNotFound
}

func (m *InMemoryRepository) FindQuestionForImport(ctx context.Context, title, leetcodeID, slug string) (*Question, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, q := range m.questions {
		if strings.EqualFold(q.Title, title) {
			res := q
			return &res, nil
		}
		if leetcodeID != "" && q.LeetCodeID != nil && *q.LeetCodeID == leetcodeID {
			res := q
			return &res, nil
		}
		if slug != "" && q.Slug == slug {
			res := q
			return &res, nil
		}
	}
	return nil, ErrQuestionNotFound
}

func (m *InMemoryRepository) ListQuestions(ctx context.Context, filter ListQuestionsFilter) ([]Question, int64, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var matched []Question
	for _, q := range m.questions {
		if filter.IsActive != nil && q.IsActive != *filter.IsActive {
			continue
		}
		if filter.SearchQuery != "" && !strings.Contains(strings.ToLower(q.Title), strings.ToLower(filter.SearchQuery)) {
			continue
		}
		if filter.Difficulty != "" && string(q.Difficulty) != filter.Difficulty {
			continue
		}
		matched = append(matched, q)
	}

	total := int64(len(matched))
	start := int(filter.Offset)
	if start > len(matched) {
		start = len(matched)
	}
	end := start + int(filter.Limit)
	if end > len(matched) {
		end = len(matched)
	}

	return matched[start:end], total, nil
}

func (m *InMemoryRepository) UpdateQuestion(ctx context.Context, id uuid.UUID, p UpdateQuestionParams) (*Question, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	q, ok := m.questions[id]
	if !ok {
		return nil, ErrQuestionNotFound
	}
	q.Title = p.Title
	q.Difficulty = p.Difficulty
	q.IsCore = p.IsCore
	if p.LeetCodeID != nil {
		q.LeetCodeID = p.LeetCodeID
	}
	q.EditorialSummary = p.EditorialSummary
	q.SampleTimeLimitMins = p.SampleTimeLimitMins
	q.UpdatedAt = time.Now().UTC()
	m.questions[id] = q
	res := q
	return &res, nil
}

func (m *InMemoryRepository) SetQuestionActive(ctx context.Context, id uuid.UUID, isActive bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	q, ok := m.questions[id]
	if !ok {
		return ErrQuestionNotFound
	}
	q.IsActive = isActive
	q.UpdatedAt = time.Now().UTC()
	m.questions[id] = q
	return nil
}

func (m *InMemoryRepository) RemoveQuestionPatterns(ctx context.Context, questionID uuid.UUID) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.qPatterns, questionID)
	return nil
}

func (m *InMemoryRepository) RecordImportHistory(ctx context.Context, h *ImportHistory) (*ImportHistory, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	id := h.ID
	if id == uuid.Nil {
		id = uuid.New()
	}
	rec := ImportHistory{
		ID:                 id,
		Filename:           h.Filename,
		TotalRows:          h.TotalRows,
		ValidRows:          h.ValidRows,
		CreatedQuestions:   h.CreatedQuestions,
		UpdatedQuestions:   h.UpdatedQuestions,
		UnchangedQuestions: h.UnchangedQuestions,
		InvalidRows:        h.InvalidRows,
		Status:             h.Status,
		ImportedBy:         h.ImportedBy,
		CreatedAt:          time.Now().UTC(),
	}
	m.history = append([]ImportHistory{rec}, m.history...)
	return &rec, nil
}

func (m *InMemoryRepository) ListImportHistory(ctx context.Context, limit, offset int32) ([]ImportHistory, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	total := len(m.history)
	start := int(offset)
	if start > total {
		start = total
	}
	end := start + int(limit)
	if end > total {
		end = total
	}
	res := make([]ImportHistory, end-start)
	copy(res, m.history[start:end])
	return res, nil
}

func (m *InMemoryRepository) LinkQuestionPattern(ctx context.Context, questionID, patternID uuid.UUID, isPrimary bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	p, ok := m.patterns[patternID]
	if !ok {
		return ErrPatternNotFound
	}
	m.qPatterns[questionID] = append(m.qPatterns[questionID], p)
	return nil
}

func (m *InMemoryRepository) GetQuestionPatterns(ctx context.Context, questionID uuid.UUID) ([]Pattern, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.qPatterns[questionID], nil
}

func (m *InMemoryRepository) CreateCompany(ctx context.Context, name, slug string) (uuid.UUID, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for id, n := range m.companies {
		if strings.EqualFold(n, name) {
			return id, nil
		}
	}
	id := uuid.New()
	m.companies[id] = name
	return id, nil
}

func (m *InMemoryRepository) GetCompanyByName(ctx context.Context, name string) (uuid.UUID, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for id, n := range m.companies {
		if strings.EqualFold(n, name) {
			return id, nil
		}
	}
	return uuid.Nil, errors.New("company not found")
}

func (m *InMemoryRepository) LinkQuestionCompany(ctx context.Context, questionID, companyID uuid.UUID, source, confidence string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	name := m.companies[companyID]
	m.qCompanies[questionID] = append(m.qCompanies[questionID], name)
	return nil
}

func (m *InMemoryRepository) GetQuestionCompanies(ctx context.Context, questionID uuid.UUID) ([]string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.qCompanies[questionID], nil
}

func (m *InMemoryRepository) CreateRoadmap(ctx context.Context, r *Roadmap) (*Roadmap, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	id := r.ID
	if id == uuid.Nil {
		id = uuid.New()
	}
	now := time.Now().UTC()
	created := Roadmap{
		ID:               id,
		UserID:           r.UserID,
		Title:            r.Title,
		Slug:             r.Slug,
		Description:      r.Description,
		TargetDays:       r.TargetDays,
		IsSystemTemplate: r.IsSystemTemplate,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	m.roadmaps[id] = created
	return &created, nil
}

func (m *InMemoryRepository) GetRoadmapBySlug(ctx context.Context, slug string) (*Roadmap, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, r := range m.roadmaps {
		if r.Slug == slug {
			res := r
			return &res, nil
		}
	}
	return nil, ErrRoadmapNotFound
}

func (m *InMemoryRepository) GetRoadmapByID(ctx context.Context, id uuid.UUID) (*Roadmap, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	r, ok := m.roadmaps[id]
	if !ok {
		return nil, ErrRoadmapNotFound
	}
	res := r
	return &res, nil
}

func (m *InMemoryRepository) ListCuratedRoadmaps(ctx context.Context) ([]Roadmap, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var list []Roadmap
	for _, r := range m.roadmaps {
		if r.IsSystemTemplate {
			rd := r
			rd.TotalQuestions = int32(len(m.roadItems[r.ID]))
			list = append(list, rd)
		}
	}
	return list, nil
}

func (m *InMemoryRepository) ListRoadmapsForUser(ctx context.Context, userID *uuid.UUID) ([]Roadmap, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var list []Roadmap
	for _, r := range m.roadmaps {
		if r.IsSystemTemplate || (userID != nil && r.UserID != nil && *r.UserID == *userID) {
			rd := r
			rd.TotalQuestions = int32(len(m.roadItems[r.ID]))
			list = append(list, rd)
		}
	}
	return list, nil
}

func (m *InMemoryRepository) DeleteRoadmap(ctx context.Context, id uuid.UUID) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.roadmaps[id]; !ok {
		return ErrRoadmapNotFound
	}
	delete(m.roadmaps, id)
	delete(m.roadItems, id)
	return nil
}

func (m *InMemoryRepository) GetPatternsProgress(ctx context.Context, userID uuid.UUID) ([]PatternProgressItem, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var items []PatternProgressItem
	for _, p := range m.patterns {
		if p.ParentID == nil {
			items = append(items, PatternProgressItem{
				PatternID:      p.ID,
				PatternName:    p.Name,
				PatternSlug:    p.Slug,
				TopicName:      "General",
				TotalQuestions: 0,
				SolvedCount:    0,
				Percent:        0,
			})
		}
	}
	return items, nil
}

func (m *InMemoryRepository) AddRoadmapItem(ctx context.Context, roadmapID, questionID uuid.UUID, stageNumber, orderIndex int32, isMandatory bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	q, ok := m.questions[questionID]
	if !ok {
		return ErrQuestionNotFound
	}
	item := RoadmapItem{
		ID:            uuid.New(),
		RoadmapID:     roadmapID,
		QuestionID:    questionID,
		StageNumber:   stageNumber,
		OrderIndex:    orderIndex,
		IsMandatory:   isMandatory,
		QuestionTitle: q.Title,
		QuestionSlug:  q.Slug,
		Difficulty:    q.Difficulty,
		IsCore:        q.IsCore,
	}
	m.roadItems[roadmapID] = append(m.roadItems[roadmapID], item)
	return nil
}

func (m *InMemoryRepository) ListRoadmapItems(ctx context.Context, roadmapID uuid.UUID) ([]RoadmapItem, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.roadItems[roadmapID], nil
}

func (m *InMemoryRepository) ExecTx(ctx context.Context, fn func(repo Repository) error) error {
	m.txMu.Lock()
	defer m.txMu.Unlock()

	// Snapshot complete state for rollback
	m.mu.Lock()
	backupTopics := make(map[uuid.UUID]Topic, len(m.topics))
	for k, v := range m.topics {
		backupTopics[k] = v
	}
	backupQuestions := make(map[uuid.UUID]Question, len(m.questions))
	for k, v := range m.questions {
		backupQuestions[k] = v
	}
	backupPatterns := make(map[uuid.UUID]Pattern, len(m.patterns))
	for k, v := range m.patterns {
		backupPatterns[k] = v
	}
	backupQPatterns := make(map[uuid.UUID][]Pattern, len(m.qPatterns))
	for k, v := range m.qPatterns {
		backupQPatterns[k] = append([]Pattern(nil), v...)
	}
	backupCompanies := make(map[uuid.UUID]string, len(m.companies))
	for k, v := range m.companies {
		backupCompanies[k] = v
	}
	backupQCompanies := make(map[uuid.UUID][]string, len(m.qCompanies))
	for k, v := range m.qCompanies {
		backupQCompanies[k] = append([]string(nil), v...)
	}
	backupRoadmaps := make(map[uuid.UUID]Roadmap, len(m.roadmaps))
	for k, v := range m.roadmaps {
		backupRoadmaps[k] = v
	}
	backupRoadItems := make(map[uuid.UUID][]RoadmapItem, len(m.roadItems))
	for k, v := range m.roadItems {
		backupRoadItems[k] = append([]RoadmapItem(nil), v...)
	}
	backupHistory := make([]ImportHistory, len(m.history))
	copy(backupHistory, m.history)
	m.mu.Unlock()

	if err := fn(m); err != nil {
		m.mu.Lock()
		m.topics = backupTopics
		m.questions = backupQuestions
		m.patterns = backupPatterns
		m.qPatterns = backupQPatterns
		m.companies = backupCompanies
		m.qCompanies = backupQCompanies
		m.roadmaps = backupRoadmaps
		m.roadItems = backupRoadItems
		m.history = backupHistory
		m.mu.Unlock()
		return err
	}
	return nil
}
