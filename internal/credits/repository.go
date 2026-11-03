package credits

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/adaptive-dsa/platform/internal/database/generated"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Repository specifies the persistence contract for credit accounts and double-entry ledger.
type Repository interface {
	GetAccount(ctx context.Context, userID uuid.UUID) (*CreditAccount, error)
	GetAccountForUpdate(ctx context.Context, userID uuid.UUID) (*CreditAccount, error)
	UpsertAccount(ctx context.Context, userID uuid.UUID, initialBalance int32) (*CreditAccount, error)
	UpdateBalance(ctx context.Context, userID uuid.UUID, delta int32) (*CreditAccount, error)
	CreateLedgerEntry(ctx context.Context, entry *LedgerEntry) (*LedgerEntry, error)
	ListLedgerEntries(ctx context.Context, userID uuid.UUID, limit, offset int32) ([]LedgerEntry, int64, error)
	SumLedgerEntries(ctx context.Context, userID uuid.UUID) (int64, error)
	ExecTx(ctx context.Context, fn func(Repository) error) error
}

// PostgresRepository implements Repository using pgxpool.Pool and sqlc.
type PostgresRepository struct {
	pool    *pgxpool.Pool
	queries *generated.Queries
	db      generated.DBTX
}

func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{
		pool:    pool,
		queries: generated.New(pool),
		db:      pool,
	}
}

func (r *PostgresRepository) withTx(tx pgx.Tx) *PostgresRepository {
	return &PostgresRepository{
		pool:    r.pool,
		queries: generated.New(tx),
		db:      tx,
	}
}

func (r *PostgresRepository) ExecTx(ctx context.Context, fn func(Repository) error) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("credits: begin tx: %w", err)
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	txRepo := r.withTx(tx)
	if err := fn(txRepo); err != nil {
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("credits: commit tx: %w", err)
	}
	return nil
}

func (r *PostgresRepository) GetAccount(ctx context.Context, userID uuid.UUID) (*CreditAccount, error) {
	row, err := r.queries.GetCreditAccount(ctx, pgtype.UUID{Bytes: userID, Valid: true})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrAccountNotFound
		}
		return nil, fmt.Errorf("credits: get account: %w", err)
	}
	return &CreditAccount{
		UserID:        userID,
		CachedBalance: row.CachedBalance,
		UpdatedAt:     row.UpdatedAt.Time,
	}, nil
}

func (r *PostgresRepository) GetAccountForUpdate(ctx context.Context, userID uuid.UUID) (*CreditAccount, error) {
	row, err := r.queries.GetCreditAccountForUpdate(ctx, pgtype.UUID{Bytes: userID, Valid: true})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrAccountNotFound
		}
		return nil, fmt.Errorf("credits: get account for update: %w", err)
	}
	return &CreditAccount{
		UserID:        userID,
		CachedBalance: row.CachedBalance,
		UpdatedAt:     row.UpdatedAt.Time,
	}, nil
}

func (r *PostgresRepository) UpsertAccount(ctx context.Context, userID uuid.UUID, initialBalance int32) (*CreditAccount, error) {
	row, err := r.queries.UpsertCreditAccount(ctx, generated.UpsertCreditAccountParams{
		UserID:        pgtype.UUID{Bytes: userID, Valid: true},
		CachedBalance: initialBalance,
	})
	if err != nil {
		return nil, fmt.Errorf("credits: upsert account: %w", err)
	}
	return &CreditAccount{
		UserID:        userID,
		CachedBalance: row.CachedBalance,
		UpdatedAt:     row.UpdatedAt.Time,
	}, nil
}

func (r *PostgresRepository) UpdateBalance(ctx context.Context, userID uuid.UUID, delta int32) (*CreditAccount, error) {
	row, err := r.queries.UpdateCreditBalance(ctx, generated.UpdateCreditBalanceParams{
		UserID:        pgtype.UUID{Bytes: userID, Valid: true},
		CachedBalance: delta,
	})
	if err != nil {
		return nil, fmt.Errorf("credits: update balance: %w", err)
	}
	return &CreditAccount{
		UserID:        userID,
		CachedBalance: row.CachedBalance,
		UpdatedAt:     row.UpdatedAt.Time,
	}, nil
}

func (r *PostgresRepository) CreateLedgerEntry(ctx context.Context, entry *LedgerEntry) (*LedgerEntry, error) {
	var refUUID pgtype.UUID
	if entry.ReferenceID != nil {
		refUUID = pgtype.UUID{Bytes: *entry.ReferenceID, Valid: true}
	}

	var refType pgtype.Text
	if entry.ReferenceType != "" {
		refType = pgtype.Text{String: entry.ReferenceType, Valid: true}
	}

	row, err := r.queries.CreateCreditLedgerEntry(ctx, generated.CreateCreditLedgerEntryParams{
		UserID:        pgtype.UUID{Bytes: entry.UserID, Valid: true},
		Amount:        entry.Amount,
		EntryType:     generated.LedgerEntryType(entry.EntryType),
		ReferenceType: refType,
		ReferenceID:   refUUID,
		Description:   entry.Description,
	})
	if err != nil {
		return nil, fmt.Errorf("credits: create ledger entry: %w", err)
	}

	var refIDOut *uuid.UUID
	if row.ReferenceID.Valid {
		id := uuid.UUID(row.ReferenceID.Bytes)
		refIDOut = &id
	}

	return &LedgerEntry{
		ID:            uuid.UUID(row.ID.Bytes),
		UserID:        uuid.UUID(row.UserID.Bytes),
		Amount:        row.Amount,
		EntryType:     LedgerEntryType(row.EntryType),
		ReferenceType: row.ReferenceType.String,
		ReferenceID:   refIDOut,
		Description:   row.Description,
		CreatedAt:     row.CreatedAt.Time,
	}, nil
}

func (r *PostgresRepository) ListLedgerEntries(ctx context.Context, userID uuid.UUID, limit, offset int32) ([]LedgerEntry, int64, error) {
	uid := pgtype.UUID{Bytes: userID, Valid: true}
	total, err := r.queries.CountCreditLedgerEntries(ctx, uid)
	if err != nil {
		return nil, 0, fmt.Errorf("credits: count ledger: %w", err)
	}

	rows, err := r.queries.ListCreditLedgerEntries(ctx, generated.ListCreditLedgerEntriesParams{
		UserID: uid,
		Limit:  limit,
		Offset: offset,
	})
	if err != nil {
		return nil, 0, fmt.Errorf("credits: list ledger: %w", err)
	}

	entries := make([]LedgerEntry, len(rows))
	for i, row := range rows {
		var refIDOut *uuid.UUID
		if row.ReferenceID.Valid {
			id := uuid.UUID(row.ReferenceID.Bytes)
			refIDOut = &id
		}
		entries[i] = LedgerEntry{
			ID:            uuid.UUID(row.ID.Bytes),
			UserID:        uuid.UUID(row.UserID.Bytes),
			Amount:        row.Amount,
			EntryType:     LedgerEntryType(row.EntryType),
			ReferenceType: row.ReferenceType.String,
			ReferenceID:   refIDOut,
			Description:   row.Description,
			CreatedAt:     row.CreatedAt.Time,
		}
	}
	return entries, total, nil
}

func (r *PostgresRepository) SumLedgerEntries(ctx context.Context, userID uuid.UUID) (int64, error) {
	sum, err := r.queries.SumCreditLedgerEntries(ctx, pgtype.UUID{Bytes: userID, Valid: true})
	if err != nil {
		return 0, fmt.Errorf("credits: sum ledger: %w", err)
	}
	return sum, nil
}

// InMemoryRepository provides thread-safe double-entry ledger testing double.
type InMemoryRepository struct {
	mu       sync.RWMutex
	txMu     sync.Mutex
	accounts map[uuid.UUID]*CreditAccount
	ledger   map[uuid.UUID][]LedgerEntry
}

func NewInMemoryRepository() *InMemoryRepository {
	return &InMemoryRepository{
		accounts: make(map[uuid.UUID]*CreditAccount),
		ledger:   make(map[uuid.UUID][]LedgerEntry),
	}
}

func (r *InMemoryRepository) ExecTx(ctx context.Context, fn func(Repository) error) error {
	r.txMu.Lock()
	defer r.txMu.Unlock()

	r.mu.Lock()
	// Snapshot for rollback
	accountsSnap := make(map[uuid.UUID]*CreditAccount, len(r.accounts))
	for k, v := range r.accounts {
		accCopy := *v
		accountsSnap[k] = &accCopy
	}
	ledgerSnap := make(map[uuid.UUID][]LedgerEntry, len(r.ledger))
	for k, v := range r.ledger {
		lCopy := make([]LedgerEntry, len(v))
		copy(lCopy, v)
		ledgerSnap[k] = lCopy
	}
	r.mu.Unlock()

	if err := fn(r); err != nil {
		r.mu.Lock()
		r.accounts = accountsSnap
		r.ledger = ledgerSnap
		r.mu.Unlock()
		return err
	}
	return nil
}

func (r *InMemoryRepository) GetAccount(ctx context.Context, userID uuid.UUID) (*CreditAccount, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	acc, exists := r.accounts[userID]
	if !exists {
		return nil, ErrAccountNotFound
	}
	accCopy := *acc
	return &accCopy, nil
}

func (r *InMemoryRepository) GetAccountForUpdate(ctx context.Context, userID uuid.UUID) (*CreditAccount, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	acc, exists := r.accounts[userID]
	if !exists {
		return nil, ErrAccountNotFound
	}
	accCopy := *acc
	return &accCopy, nil
}

func (r *InMemoryRepository) UpsertAccount(ctx context.Context, userID uuid.UUID, initialBalance int32) (*CreditAccount, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	acc, exists := r.accounts[userID]
	if !exists {
		acc = &CreditAccount{
			UserID:        userID,
			CachedBalance: initialBalance,
			UpdatedAt:     time.Now().UTC(),
		}
		r.accounts[userID] = acc
	} else {
		acc.CachedBalance = initialBalance
		acc.UpdatedAt = time.Now().UTC()
	}
	accCopy := *acc
	return &accCopy, nil
}

func (r *InMemoryRepository) UpdateBalance(ctx context.Context, userID uuid.UUID, delta int32) (*CreditAccount, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	acc, exists := r.accounts[userID]
	if !exists {
		return nil, ErrAccountNotFound
	}
	if acc.CachedBalance+delta < 0 {
		return nil, ErrNegativeBalanceProhibited
	}
	acc.CachedBalance += delta
	acc.UpdatedAt = time.Now().UTC()
	accCopy := *acc
	return &accCopy, nil
}

func (r *InMemoryRepository) CreateLedgerEntry(ctx context.Context, entry *LedgerEntry) (*LedgerEntry, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	e := *entry
	if e.ID == uuid.Nil {
		e.ID = uuid.New()
	}
	if e.CreatedAt.IsZero() {
		e.CreatedAt = time.Now().UTC()
	}
	r.ledger[e.UserID] = append([]LedgerEntry{e}, r.ledger[e.UserID]...)
	return &e, nil
}

func (r *InMemoryRepository) ListLedgerEntries(ctx context.Context, userID uuid.UUID, limit, offset int32) ([]LedgerEntry, int64, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	entries, exists := r.ledger[userID]
	if !exists {
		return []LedgerEntry{}, 0, nil
	}
	total := int64(len(entries))
	start := int(offset)
	if start > len(entries) {
		return []LedgerEntry{}, total, nil
	}
	end := start + int(limit)
	if end > len(entries) {
		end = len(entries)
	}
	out := make([]LedgerEntry, end-start)
	copy(out, entries[start:end])
	return out, total, nil
}

func (r *InMemoryRepository) SumLedgerEntries(ctx context.Context, userID uuid.UUID) (int64, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	entries, exists := r.ledger[userID]
	if !exists {
		return 0, nil
	}
	var total int64
	for _, e := range entries {
		total += int64(e.Amount)
	}
	return total, nil
}
