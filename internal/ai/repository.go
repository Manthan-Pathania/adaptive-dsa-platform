package ai

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/adaptive-dsa/platform/internal/database/generated"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Repository specifies the persistence contract for AI usage telemetry and audit records.
type Repository interface {
	CreateRequest(ctx context.Context, req *AIRequestRecord) (*AIRequestRecord, error)
	ListRequestsByUser(ctx context.Context, userID uuid.UUID, limit, offset int32) ([]AIRequestRecord, int64, error)
}

// PostgresRepository implements Repository using PostgreSQL.
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

func (r *PostgresRepository) CreateRequest(ctx context.Context, req *AIRequestRecord) (*AIRequestRecord, error) {
	row, err := r.queries.CreateAIRequest(ctx, generated.CreateAIRequestParams{
		UserID:                 pgtype.UUID{Bytes: req.UserID, Valid: true},
		Feature:                generated.AiFeatureType(req.Feature),
		Model:                  req.Model,
		PromptTokens:           req.PromptTokens,
		CompletionTokens:       req.CompletionTokens,
		EstimatedCostUsdMicros: req.EstimatedCostUsdMicros,
		CreditsCharged:         req.CreditsCharged,
		Status:                 req.Status,
		LatencyMs:              req.LatencyMs,
	})
	if err != nil {
		return nil, fmt.Errorf("ai repo: create request: %w", err)
	}

	return &AIRequestRecord{
		ID:                     uuid.UUID(row.ID.Bytes),
		UserID:                 uuid.UUID(row.UserID.Bytes),
		Feature:                FeatureType(row.Feature),
		Model:                  row.Model,
		PromptTokens:           row.PromptTokens,
		CompletionTokens:       row.CompletionTokens,
		EstimatedCostUsdMicros: row.EstimatedCostUsdMicros,
		CreditsCharged:         row.CreditsCharged,
		Status:                 row.Status,
		LatencyMs:              row.LatencyMs,
		CreatedAt:              row.CreatedAt.Time,
	}, nil
}

func (r *PostgresRepository) ListRequestsByUser(ctx context.Context, userID uuid.UUID, limit, offset int32) ([]AIRequestRecord, int64, error) {
	uid := pgtype.UUID{Bytes: userID, Valid: true}
	total, err := r.queries.CountAIRequestsByUser(ctx, uid)
	if err != nil {
		return nil, 0, fmt.Errorf("ai repo: count requests: %w", err)
	}

	rows, err := r.queries.ListAIRequestsByUser(ctx, generated.ListAIRequestsByUserParams{
		UserID: uid,
		Limit:  limit,
		Offset: offset,
	})
	if err != nil {
		return nil, 0, fmt.Errorf("ai repo: list requests: %w", err)
	}

	out := make([]AIRequestRecord, len(rows))
	for i, row := range rows {
		out[i] = AIRequestRecord{
			ID:                     uuid.UUID(row.ID.Bytes),
			UserID:                 uuid.UUID(row.UserID.Bytes),
			Feature:                FeatureType(row.Feature),
			Model:                  row.Model,
			PromptTokens:           row.PromptTokens,
			CompletionTokens:       row.CompletionTokens,
			EstimatedCostUsdMicros: row.EstimatedCostUsdMicros,
			CreditsCharged:         row.CreditsCharged,
			Status:                 row.Status,
			LatencyMs:              row.LatencyMs,
			CreatedAt:              row.CreatedAt.Time,
		}
	}
	return out, total, nil
}

// InMemoryRepository provides thread-safe in-memory store for unit tests.
type InMemoryRepository struct {
	mu       sync.RWMutex
	requests []AIRequestRecord
}

func NewInMemoryRepository() *InMemoryRepository {
	return &InMemoryRepository{
		requests: make([]AIRequestRecord, 0),
	}
}

func (r *InMemoryRepository) CreateRequest(ctx context.Context, req *AIRequestRecord) (*AIRequestRecord, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	rec := *req
	if rec.ID == uuid.Nil {
		rec.ID = uuid.New()
	}
	if rec.CreatedAt.IsZero() {
		rec.CreatedAt = time.Now().UTC()
	}
	r.requests = append([]AIRequestRecord{rec}, r.requests...)
	return &rec, nil
}

func (r *InMemoryRepository) ListRequestsByUser(ctx context.Context, userID uuid.UUID, limit, offset int32) ([]AIRequestRecord, int64, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var userReqs []AIRequestRecord
	for _, req := range r.requests {
		if req.UserID == userID {
			userReqs = append(userReqs, req)
		}
	}

	total := int64(len(userReqs))
	start := int(offset)
	if start > len(userReqs) {
		return []AIRequestRecord{}, total, nil
	}
	end := start + int(limit)
	if end > len(userReqs) {
		end = len(userReqs)
	}
	out := make([]AIRequestRecord, end-start)
	copy(out, userReqs[start:end])
	return out, total, nil
}
