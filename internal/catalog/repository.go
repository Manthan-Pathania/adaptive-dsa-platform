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
	ListQuestions(ctx context.Context, filter ListQuestionsFilter) ([]Question, int64, error)

	LinkQuestionPattern(ctx context.Context, questionID, patternID uuid.UUID, isPrimary bool) error
	GetQuestionPatterns(ctx context.Context, questionID uuid.UUID) ([]Pattern, error)

	CreateCompany(ctx context.Context, name, slug string) (uuid.UUID, error)
	GetCompanyByName(ctx context.Context, name string) (uuid.UUID, error)
	LinkQuestionCompany(ctx context.Context, questionID, companyID uuid.UUID, source, confidence string) error
	GetQuestionCompanies(ctx context.Context, questionID uuid.UUID) ([]string, error)

	CreateRoadmap(ctx context.Context, r *Roadmap) (*Roadmap, error)
	GetRoadmapBySlug(ctx context.Context, slug string) (*Roadmap, error)
	GetRoadmapByID(ctx context.Context, id uuid.UUID) (*Roadmap, error)
	ListCuratedRoadmaps(ctx context.Context) ([]Roadmap, error)
	AddRoadmapItem(ctx context.Context, roadmapID, questionID uuid.UUID, stageNumber, orderIndex int32, isMandatory bool) error
	ListRoadmapItems(ctx context.Context, roadmapID uuid.UUID) ([]RoadmapItem, error)

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

	res, err := r.q.UpsertQuestion(ctx, generated.UpsertQuestionParams{
		Slug:                q.Slug,
		Title:               q.Title,
		ExternalUrl:         q.ExternalURL,
		Difficulty:          generated.QuestionDifficulty(q.Difficulty),
		IsCore:              q.IsCore,
		EditorialSummary:    edSummary,
		SampleTimeLimitMins: q.SampleTimeLimitMins,
	})
	if err != nil {
		return nil, fmt.Errorf("upsert question: %w", err)
	}

	var resSummary *string
	if res.EditorialSummary.Valid {
		resSummary = &res.EditorialSummary.String
	}

	return &Question{
		ID:                  toUUID(res.ID),
		Slug:                res.Slug,
		Title:               res.Title,
		ExternalURL:         res.ExternalUrl,
		Difficulty:          Difficulty(res.Difficulty),
		IsCore:              res.IsCore,
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
	return &Question{
		ID:                  toUUID(res.ID),
		Slug:                res.Slug,
		Title:               res.Title,
		ExternalURL:         res.ExternalUrl,
		Difficulty:          Difficulty(res.Difficulty),
		IsCore:              res.IsCore,
		EditorialSummary:    resSummary,
		SampleTimeLimitMins: res.SampleTimeLimitMins,
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
	return &Question{
		ID:                  toUUID(res.ID),
		Slug:                res.Slug,
		Title:               res.Title,
		ExternalURL:         res.ExternalUrl,
		Difficulty:          Difficulty(res.Difficulty),
		IsCore:              res.IsCore,
		EditorialSummary:    resSummary,
		SampleTimeLimitMins: res.SampleTimeLimitMins,
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
	return &Question{
		ID:                  toUUID(res.ID),
		Slug:                res.Slug,
		Title:               res.Title,
		ExternalURL:         res.ExternalUrl,
		Difficulty:          Difficulty(res.Difficulty),
		IsCore:              res.IsCore,
		EditorialSummary:    resSummary,
		SampleTimeLimitMins: res.SampleTimeLimitMins,
		CreatedAt:           res.CreatedAt.Time,
		UpdatedAt:           res.UpdatedAt.Time,
	}, nil
}

func (r *pgxRepository) ListQuestions(ctx context.Context, filter ListQuestionsFilter) ([]Question, int64, error) {
	var topicPatternUUID pgtype.UUID
	if filter.TopicOrPatternID != nil {
		topicPatternUUID = pgtype.UUID{Bytes: *filter.TopicOrPatternID, Valid: true}
	}

	rows, err := r.q.ListQuestions(ctx, generated.ListQuestionsParams{
		Column1: filter.SearchQuery,
		Column2: filter.Difficulty,
		Column3: topicPatternUUID,
		Limit:   filter.Limit,
		Offset:  filter.Offset,
	})
	if err != nil {
		return nil, 0, err
	}

	count, err := r.q.CountQuestions(ctx, generated.CountQuestionsParams{
		Column1: filter.SearchQuery,
		Column2: filter.Difficulty,
		Column3: topicPatternUUID,
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
		questions[i] = Question{
			ID:                  toUUID(res.ID),
			Slug:                res.Slug,
			Title:               res.Title,
			ExternalURL:         res.ExternalUrl,
			Difficulty:          Difficulty(res.Difficulty),
			IsCore:              res.IsCore,
			EditorialSummary:    resSummary,
			SampleTimeLimitMins: res.SampleTimeLimitMins,
			CreatedAt:           res.CreatedAt.Time,
			UpdatedAt:           res.UpdatedAt.Time,
		}
	}
	return questions, count, nil
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
	rows, err := r.q.ListCuratedRoadmaps(ctx)
	if err != nil {
		return nil, err
	}
	roadmaps := make([]Roadmap, len(rows))
	for i, res := range rows {
		var resDesc *string
		if res.Description.Valid {
			resDesc = &res.Description.String
		}
		var resUID *uuid.UUID
		if res.UserID.Valid {
			u := uuid.UUID(res.UserID.Bytes)
			resUID = &u
		}
		roadmaps[i] = Roadmap{
			ID:               toUUID(res.ID),
			UserID:           resUID,
			Title:            res.Title,
			Slug:             res.Slug,
			Description:      resDesc,
			TargetDays:       res.TargetDays,
			IsSystemTemplate: res.IsSystemTemplate,
			CreatedAt:        res.CreatedAt.Time,
			UpdatedAt:        res.UpdatedAt.Time,
		}
	}
	return roadmaps, nil
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
	rows, err := r.q.ListRoadmapItems(ctx, toPgUUID(roadmapID))
	if err != nil {
		return nil, err
	}
	items := make([]RoadmapItem, len(rows))
	for i, row := range rows {
		items[i] = RoadmapItem{
			ID:            toUUID(row.ID),
			RoadmapID:     toUUID(row.RoadmapID),
			QuestionID:    toUUID(row.QuestionID),
			StageNumber:   row.StageNumber,
			OrderIndex:    row.OrderIndex,
			IsMandatory:   row.IsMandatory,
			QuestionTitle: row.Title,
			QuestionSlug:  row.QuestionSlug,
			Difficulty:    Difficulty(row.Difficulty),
			IsCore:        row.IsCore,
		}
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
	now := time.Now().UTC()
	created := Question{
		ID:                  id,
		Slug:                q.Slug,
		Title:               q.Title,
		ExternalURL:         q.ExternalURL,
		Difficulty:          q.Difficulty,
		IsCore:              q.IsCore,
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

func (m *InMemoryRepository) ListQuestions(ctx context.Context, filter ListQuestionsFilter) ([]Question, int64, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var matched []Question
	for _, q := range m.questions {
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
			list = append(list, r)
		}
	}
	return list, nil
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
		m.mu.Unlock()
		return err
	}
	return nil
}
