package credits

import (
	"context"
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/adaptive-dsa/platform/internal/config"
	"github.com/adaptive-dsa/platform/internal/platform/database"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupPostgresCredits(t *testing.T) (Service, Repository, *database.DB, uuid.UUID) {
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

	// Create test candidate user in users table
	testUserID := uuid.New()
	email := fmt.Sprintf("credit_test_%s@example.com", testUserID.String()[:8])
	_, err = db.Pool().Exec(ctx, `
		INSERT INTO users (id, email, password_hash, full_name, role)
		VALUES ($1, $2, 'dummy_hash', 'Credits Test Candidate', 'CANDIDATE')
	`, testUserID, email)
	require.NoError(t, err)

	t.Cleanup(func() {
		_, _ = db.Pool().Exec(context.Background(), "DELETE FROM users WHERE id = $1", testUserID)
		db.Close()
	})

	repo := NewPostgresRepository(db.Pool())
	svc := NewService(repo)
	return svc, repo, db, testUserID
}

func TestCredits_Postgres_EnsureAccountAndDoubleEntry(t *testing.T) {
	svc, _, _, userID := setupPostgresCredits(t)
	ctx := context.Background()

	// 1. Ensure account with 50 credits
	acc, err := svc.EnsureAccount(ctx, userID, 50)
	require.NoError(t, err)
	assert.Equal(t, int32(50), acc.CachedBalance)

	// 2. Deduct 10 credits
	refID := uuid.New()
	entry, err := svc.CommitDeduction(ctx, userID, 10, EntryTypeAIUsage, "AI_REQUEST", &refID, "Level 2 hint")
	require.NoError(t, err)
	assert.Equal(t, int32(-10), entry.Amount)

	bal, err := svc.GetBalance(ctx, userID)
	require.NoError(t, err)
	assert.Equal(t, int32(40), bal)

	// 3. Reconcile balance against ledger sum
	consistent, cached, sum, err := svc.ReconcileBalance(ctx, userID)
	require.NoError(t, err)
	assert.True(t, consistent)
	assert.Equal(t, int32(40), cached)
	assert.Equal(t, int64(40), sum)
}

func TestCredits_Postgres_ConcurrentDeductions_NoLostUpdatesOrUnderflow(t *testing.T) {
	svc, _, _, userID := setupPostgresCredits(t)
	ctx := context.Background()

	// Initial balance: exactly 30 credits
	_, err := svc.EnsureAccount(ctx, userID, 30)
	require.NoError(t, err)

	// Launch 20 concurrent goroutines, each trying to deduct 2 credits (total requested = 40 credits)
	// Since initial balance is 30, exactly 15 goroutines must succeed and exactly 5 must fail with ErrInsufficientCredits.
	const goroutines = 20
	const deductAmount = 2

	var successCount int32
	var failCount int32
	var wg sync.WaitGroup

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			_, deductErr := svc.CommitDeduction(
				ctx,
				userID,
				deductAmount,
				EntryTypeAIUsage,
				"AI_REQUEST",
				nil,
				fmt.Sprintf("Concurrent hint %d", idx),
			)
			if deductErr == nil {
				atomic.AddInt32(&successCount, 1)
			} else if deductErr == ErrInsufficientCredits {
				atomic.AddInt32(&failCount, 1)
			}
		}(i)
	}

	wg.Wait()

	assert.Equal(t, int32(15), successCount, "exactly 15 deductions should succeed")
	assert.Equal(t, int32(5), failCount, "exactly 5 deductions should fail due to insufficient credits")

	finalBal, err := svc.GetBalance(ctx, userID)
	require.NoError(t, err)
	assert.Equal(t, int32(0), finalBal, "remaining balance must be exactly 0")

	// Verify ledger sum is 0
	consistent, cached, sum, err := svc.ReconcileBalance(ctx, userID)
	require.NoError(t, err)
	assert.True(t, consistent)
	assert.Equal(t, int32(0), cached)
	assert.Equal(t, int64(0), sum)
}

func TestCredits_Postgres_TransactionRollback(t *testing.T) {
	svc, repo, _, userID := setupPostgresCredits(t)
	ctx := context.Background()

	_, err := svc.EnsureAccount(ctx, userID, 50)
	require.NoError(t, err)

	// Attempt a transaction that creates a ledger entry and updates balance, but fails deliberately
	txErr := repo.ExecTx(ctx, func(txRepo Repository) error {
		_, err := txRepo.UpdateBalance(ctx, userID, -20)
		if err != nil {
			return err
		}
		_, err = txRepo.CreateLedgerEntry(ctx, &LedgerEntry{
			UserID:      userID,
			Amount:      -20,
			EntryType:   EntryTypeAIUsage,
			Description: "Deliberately failing tx",
		})
		if err != nil {
			return err
		}
		return fmt.Errorf("simulated failure forcing rollback")
	})

	assert.Error(t, txErr)

	// Balance must still be 50, with no ghost ledger entries
	bal, err := svc.GetBalance(ctx, userID)
	require.NoError(t, err)
	assert.Equal(t, int32(50), bal)

	entries, total, err := svc.GetLedger(ctx, userID, 10, 0)
	require.NoError(t, err)
	assert.Equal(t, int64(1), total) // only the initial welcome grant
	assert.Equal(t, 1, len(entries))
}

func TestCredits_Postgres_ConcurrentDeductions_Refunds_And_Grants(t *testing.T) {
	svc, _, _, userID := setupPostgresCredits(t)
	ctx := context.Background()
	adminID := uuid.New()

	// Initial balance: 50 credits
	_, err := svc.EnsureAccount(ctx, userID, 50)
	require.NoError(t, err)

	// Concurrently run:
	// - 10 deductions of 2 credits each: total -20
	// - 5 refunds of 4 credits each: total +20
	// - 2 admin grants of 15 credits each: total +30
	// Expected net delta: -20 + 20 + 30 = +30 credits.
	// Expected final balance: 50 + 30 = 80 credits.

	var wg sync.WaitGroup

	// Deductions
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			_, dErr := svc.CommitDeduction(ctx, userID, 2, EntryTypeAIUsage, "AI_REQUEST", nil, fmt.Sprintf("Deduction %d", idx))
			assert.NoError(t, dErr)
		}(i)
	}

	// Refunds
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			_, rErr := svc.RefundCredits(ctx, userID, 4, fmt.Sprintf("Refund %d", idx), nil)
			assert.NoError(t, rErr)
		}(i)
	}

	// Admin Grants
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			_, gErr := svc.GrantPromotionalCredits(ctx, adminID, userID, 15, fmt.Sprintf("Admin grant %d", idx))
			assert.NoError(t, gErr)
		}(i)
	}

	wg.Wait()

	finalBal, err := svc.GetBalance(ctx, userID)
	require.NoError(t, err)
	assert.Equal(t, int32(80), finalBal)

	// Reconcile balance against full ledger sum
	consistent, cached, sum, err := svc.ReconcileBalance(ctx, userID)
	require.NoError(t, err)
	assert.True(t, consistent, "cached balance must match ledger sum exactly under concurrency")
	assert.Equal(t, int32(80), cached)
	assert.Equal(t, int64(80), sum)
}
