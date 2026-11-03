package credits

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCredits_EnsureAccountAndWelcomeBonus(t *testing.T) {
	repo := NewInMemoryRepository()
	svc := NewService(repo)
	ctx := context.Background()
	userID := uuid.New()

	// Initial balance should be 0 before account exists
	bal, err := svc.GetBalance(ctx, userID)
	require.NoError(t, err)
	assert.Equal(t, int32(0), bal)

	// Ensure account with 50 welcome credits
	acc, err := svc.EnsureAccount(ctx, userID, 50)
	require.NoError(t, err)
	assert.Equal(t, int32(50), acc.CachedBalance)

	// Read balance
	bal, err = svc.GetBalance(ctx, userID)
	require.NoError(t, err)
	assert.Equal(t, int32(50), bal)

	// Invariant check: cached_balance == SUM(ledger)
	consistent, cached, sum, err := svc.ReconcileBalance(ctx, userID)
	require.NoError(t, err)
	assert.True(t, consistent)
	assert.Equal(t, int32(50), cached)
	assert.Equal(t, int64(50), sum)
}

func TestCredits_CheckAndReserve(t *testing.T) {
	repo := NewInMemoryRepository()
	svc := NewService(repo)
	ctx := context.Background()
	userID := uuid.New()

	_, err := svc.EnsureAccount(ctx, userID, 10)
	require.NoError(t, err)

	// Check with sufficient credits (<= 10)
	assert.NoError(t, svc.CheckAndReserve(ctx, userID, 5))
	assert.NoError(t, svc.CheckAndReserve(ctx, userID, 10))

	// Check with insufficient credits (> 10)
	assert.ErrorIs(t, svc.CheckAndReserve(ctx, userID, 11), ErrInsufficientCredits)
}

func TestCredits_CommitDeduction_DoubleEntry(t *testing.T) {
	repo := NewInMemoryRepository()
	svc := NewService(repo)
	ctx := context.Background()
	userID := uuid.New()
	refID := uuid.New()

	_, err := svc.EnsureAccount(ctx, userID, 50)
	require.NoError(t, err)

	// Deduct 4 credits for hint
	entry, err := svc.CommitDeduction(ctx, userID, 4, EntryTypeAIUsage, "AI_REQUEST", &refID, "Progressive hint Level 3")
	require.NoError(t, err)
	assert.Equal(t, int32(-4), entry.Amount)

	bal, err := svc.GetBalance(ctx, userID)
	require.NoError(t, err)
	assert.Equal(t, int32(46), bal)

	// Deduct another 5 credits
	_, err = svc.CommitDeduction(ctx, userID, 5, EntryTypeAIUsage, "AI_REQUEST", nil, "Approach review")
	require.NoError(t, err)

	bal, err = svc.GetBalance(ctx, userID)
	require.NoError(t, err)
	assert.Equal(t, int32(41), bal)

	// Double-entry reconciliation check
	consistent, cached, sum, err := svc.ReconcileBalance(ctx, userID)
	require.NoError(t, err)
	assert.True(t, consistent)
	assert.Equal(t, int32(41), cached)
	assert.Equal(t, int64(41), sum) // 50 - 4 - 5 = 41
}

func TestCredits_CommitDeduction_InsufficientBalance_Fails(t *testing.T) {
	repo := NewInMemoryRepository()
	svc := NewService(repo)
	ctx := context.Background()
	userID := uuid.New()

	_, err := svc.EnsureAccount(ctx, userID, 2)
	require.NoError(t, err)

	// Attempting to deduct 5 credits should fail with ErrInsufficientCredits
	_, err = svc.CommitDeduction(ctx, userID, 5, EntryTypeAIUsage, "AI_REQUEST", nil, "Too expensive")
	assert.ErrorIs(t, err, ErrInsufficientCredits)

	// Balance remains untouched
	bal, err := svc.GetBalance(ctx, userID)
	require.NoError(t, err)
	assert.Equal(t, int32(2), bal)
}

func TestCredits_AdminPromotionalGrant(t *testing.T) {
	repo := NewInMemoryRepository()
	svc := NewService(repo)
	ctx := context.Background()
	adminID := uuid.New()
	targetUserID := uuid.New()

	// Grant 100 promotional credits to user who had no account yet
	entry, err := svc.GrantPromotionalCredits(ctx, adminID, targetUserID, 100, "Beta tester grant")
	require.NoError(t, err)
	assert.Equal(t, int32(100), entry.Amount)
	assert.Equal(t, EntryTypePromotionalGrant, entry.EntryType)

	bal, err := svc.GetBalance(ctx, targetUserID)
	require.NoError(t, err)
	assert.Equal(t, int32(100), bal)

	// Invariant check
	consistent, cached, sum, err := svc.ReconcileBalance(ctx, targetUserID)
	require.NoError(t, err)
	assert.True(t, consistent)
	assert.Equal(t, int32(100), cached)
	assert.Equal(t, int64(100), sum)
}
