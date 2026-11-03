package ai

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/adaptive-dsa/platform/internal/config"
	"github.com/adaptive-dsa/platform/internal/credits"
	"github.com/adaptive-dsa/platform/internal/platform/database"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupPostgresAI(t *testing.T) (Service, credits.Service, *database.DB, uuid.UUID) {
	t.Helper()

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://postgres@127.0.0.1:5432/adaptive_dsa_test?sslmode=disable"
	}

	cfg := config.DatabaseConfig{
		URL:               dbURL,
		MaxConns:          25,
		MinConns:          5,
		MaxConnLifetime:   5 * time.Minute,
		MaxConnIdleTime:   1 * time.Minute,
		HealthCheckPeriod: 30 * time.Second,
		ConnectTimeout:    5 * time.Second,
	}

	ctx := context.Background()
	db, err := database.New(ctx, cfg, nil)
	if err != nil {
		t.Skipf("skipping postgres integration test: database unavailable: %v", err)
	}

	// Create test user in users table
	testUserID := uuid.New()
	email := fmt.Sprintf("ai_test_%s@example.com", testUserID.String()[:8])
	_, err = db.Pool().Exec(ctx, `
		INSERT INTO users (id, email, password_hash, full_name, role)
		VALUES ($1, $2, 'dummy_hash', 'AI Test Candidate', 'CANDIDATE')
	`, testUserID, email)
	require.NoError(t, err)

	t.Cleanup(func() {
		_, _ = db.Pool().Exec(context.Background(), "DELETE FROM users WHERE id = $1", testUserID)
		db.Close()
	})

	creditsRepo := credits.NewPostgresRepository(db.Pool())
	creditSvc := credits.NewService(creditsRepo)

	aiRepo := NewPostgresRepository(db.Pool())
	mockProvider := NewMockAIProvider()
	aiSvc := NewService(aiRepo, mockProvider, creditSvc, nil)

	return aiSvc, creditSvc, db, testUserID
}

func TestAI_Postgres_HintRequest_WithLiveCreditDeduction(t *testing.T) {
	aiSvc, creditSvc, _, userID := setupPostgresAI(t)
	ctx := context.Background()

	// Provision user with 50 credits in real PostgreSQL
	_, err := creditSvc.EnsureAccount(ctx, userID, 50)
	require.NoError(t, err)

	// Request Level 3 Hint (Costs 4 credits)
	res, err := aiSvc.RequestHint(ctx, userID, GenerateHintRequest{
		QuestionTitle: "Binary Tree Maximum Path Sum",
		PatternName:   "Tree DFS",
		Difficulty:    "HARD",
		Level:         HintLevelInvariant,
	})
	require.NoError(t, err)
	assert.Equal(t, int32(4), res.CreditsDeducted)
	assert.Equal(t, int32(46), res.RemainingBalance)
	assert.Contains(t, res.HintText, "invariant")

	// Verify credit ledger and cached balance in PostgreSQL
	bal, err := creditSvc.GetBalance(ctx, userID)
	require.NoError(t, err)
	assert.Equal(t, int32(46), bal)

	consistent, cached, sum, err := creditSvc.ReconcileBalance(ctx, userID)
	require.NoError(t, err)
	assert.True(t, consistent)
	assert.Equal(t, int32(46), cached)
	assert.Equal(t, int64(46), sum)

	// Verify ai_requests table row in PostgreSQL
	reqs, total, err := aiSvc.ListUserRequests(ctx, userID, 10, 0)
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	assert.Equal(t, "SUCCESS", reqs[0].Status)
	assert.Equal(t, int32(4), reqs[0].CreditsCharged)
	assert.Equal(t, FeatureProgressiveHint, reqs[0].Feature)
}

func TestAI_Postgres_ZeroChargeGuarantee_OnProviderFailure(t *testing.T) {
	aiSvc, creditSvc, db, userID := setupPostgresAI(t)
	ctx := context.Background()

	_, err := creditSvc.EnsureAccount(ctx, userID, 25)
	require.NoError(t, err)

	// Inject failure
	aiRepo := NewPostgresRepository(db.Pool())
	failingMock := NewMockAIProvider()
	failingMock.SetForcedError(ErrProviderUnavailable)
	failingAISvc := NewService(aiRepo, failingMock, creditSvc, nil)

	_, err = failingAISvc.RequestHint(ctx, userID, GenerateHintRequest{
		QuestionTitle: "Word Ladder",
		PatternName:   "BFS",
		Difficulty:    "HARD",
		Level:         HintLevelPseudocode, // 4 credits
	})
	assert.ErrorIs(t, err, ErrProviderUnavailable)

	// Balance MUST be exactly 25
	bal, err := creditSvc.GetBalance(ctx, userID)
	require.NoError(t, err)
	assert.Equal(t, int32(25), bal)

	// Verify ledger entries: only welcome bonus (amount = 25)
	entries, total, err := creditSvc.GetLedger(ctx, userID, 10, 0)
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	assert.Equal(t, int32(25), entries[0].Amount)

	// Verify FAILED request logged in PostgreSQL with credits_charged = 0
	reqs, total, err := aiSvc.ListUserRequests(ctx, userID, 10, 0)
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	assert.Equal(t, "FAILED", reqs[0].Status)
	assert.Equal(t, int32(0), reqs[0].CreditsCharged)
}
