package adaptive

import (
	"context"
	"fmt"
	"sync"

	"github.com/adaptive-dsa/platform/internal/database/generated"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository interface {
	GetUserPatternMetrics(ctx context.Context, userID uuid.UUID) ([]PatternMetric, error)
	GetUnsolvedQuestionsForPattern(ctx context.Context, userID, patternID uuid.UUID, limit int32) ([]RecommendedQuestion, error)
}

type PostgresRepository struct {
	pool    *pgxpool.Pool
	queries *generated.Queries
}

func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{
		pool:    pool,
		queries: generated.New(pool),
	}
}

func (r *PostgresRepository) GetUserPatternMetrics(ctx context.Context, userID uuid.UUID) ([]PatternMetric, error) {
	rows, err := r.queries.GetUserPatternMetrics(ctx, pgtype.UUID{Bytes: userID, Valid: true})
	if err != nil {
		return nil, fmt.Errorf("adaptive: get pattern metrics: %w", err)
	}

	metrics := make([]PatternMetric, len(rows))
	for i, row := range rows {
		isWeak := row.FailedAttempts > 0 || (row.TotalAttempts > 0 && row.AvgQualityRating < 3.0)
		metrics[i] = PatternMetric{
			PatternID:         uuid.UUID(row.PatternID.Bytes),
			PatternName:       row.PatternName,
			PatternSlug:       row.PatternSlug,
			TotalAttempts:     row.TotalAttempts,
			FailedAttempts:    row.FailedAttempts,
			IndependentSolves: row.IndependentSolves,
			HintSolves:        row.HintSolves,
			AvgQualityRating:  row.AvgQualityRating,
			IsWeak:            isWeak,
		}
	}
	return metrics, nil
}

func (r *PostgresRepository) GetUnsolvedQuestionsForPattern(ctx context.Context, userID, patternID uuid.UUID, limit int32) ([]RecommendedQuestion, error) {
	rows, err := r.queries.GetUnsolvedQuestionsForPattern(ctx, generated.GetUnsolvedQuestionsForPatternParams{
		UserID: pgtype.UUID{Bytes: userID, Valid: true},
		ID:     pgtype.UUID{Bytes: patternID, Valid: true},
		Limit:  limit,
	})
	if err != nil {
		return nil, fmt.Errorf("adaptive: get unsolved questions for pattern: %w", err)
	}

	out := make([]RecommendedQuestion, len(rows))
	for i, row := range rows {
		out[i] = RecommendedQuestion{
			QuestionID:  uuid.UUID(row.ID.Bytes),
			Title:       row.Title,
			Slug:        row.Slug,
			Difficulty:  string(row.Difficulty),
			PatternName: row.PatternName,
		}
	}
	return out, nil
}

// InMemoryRepository for unit testing
type InMemoryRepository struct {
	mu          sync.RWMutex
	metrics     map[uuid.UUID][]PatternMetric
	patternPool map[uuid.UUID][]RecommendedQuestion
}

func NewInMemoryRepository() *InMemoryRepository {
	return &InMemoryRepository{
		metrics:     make(map[uuid.UUID][]PatternMetric),
		patternPool: make(map[uuid.UUID][]RecommendedQuestion),
	}
}

func (m *InMemoryRepository) SetPatternMetrics(userID uuid.UUID, metrics []PatternMetric) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.metrics[userID] = metrics
}

func (m *InMemoryRepository) SetPatternQuestions(patternID uuid.UUID, questions []RecommendedQuestion) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.patternPool[patternID] = questions
}

func (m *InMemoryRepository) GetUserPatternMetrics(ctx context.Context, userID uuid.UUID) ([]PatternMetric, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.metrics[userID], nil
}

func (m *InMemoryRepository) GetUnsolvedQuestionsForPattern(ctx context.Context, userID, patternID uuid.UUID, limit int32) ([]RecommendedQuestion, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	pool := m.patternPool[patternID]
	if int32(len(pool)) > limit {
		return pool[:limit], nil
	}
	return pool, nil
}
