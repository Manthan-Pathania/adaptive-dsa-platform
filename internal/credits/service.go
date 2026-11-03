package credits

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// Service specifies high-level credit balance and double-entry ledger operations.
type Service interface {
	GetBalance(ctx context.Context, userID uuid.UUID) (int32, error)
	GetLedger(ctx context.Context, userID uuid.UUID, limit, offset int32) ([]LedgerEntry, int64, error)
	EnsureAccount(ctx context.Context, userID uuid.UUID, initialCredits int32) (*CreditAccount, error)
	CheckAndReserve(ctx context.Context, userID uuid.UUID, requiredCredits int32) error
	CommitDeduction(ctx context.Context, userID uuid.UUID, amount int32, entryType LedgerEntryType, refType string, refID *uuid.UUID, description string) (*LedgerEntry, error)
	GrantPromotionalCredits(ctx context.Context, actorID uuid.UUID, targetUserID uuid.UUID, amount int32, reason string) (*LedgerEntry, error)
	GrantPurchasedCredits(ctx context.Context, userID uuid.UUID, amount int32, paymentOrderID uuid.UUID, description string) (*LedgerEntry, error)
	RefundCredits(ctx context.Context, targetUserID uuid.UUID, amount int32, reason string, refID *uuid.UUID) (*LedgerEntry, error)
	ReconcileBalance(ctx context.Context, userID uuid.UUID) (bool, int32, int64, error)
}

type service struct {
	repo Repository
}

func NewService(repo Repository) Service {
	return &service{repo: repo}
}

func (s *service) GetBalance(ctx context.Context, userID uuid.UUID) (int32, error) {
	acc, err := s.repo.GetAccount(ctx, userID)
	if err != nil {
		if err == ErrAccountNotFound {
			return 0, nil
		}
		return 0, fmt.Errorf("get balance: %w", err)
	}
	return acc.CachedBalance, nil
}

func (s *service) GetLedger(ctx context.Context, userID uuid.UUID, limit, offset int32) ([]LedgerEntry, int64, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}
	return s.repo.ListLedgerEntries(ctx, userID, limit, offset)
}

// EnsureAccount initializes a credit account if it does not yet exist.
func (s *service) EnsureAccount(ctx context.Context, userID uuid.UUID, initialCredits int32) (*CreditAccount, error) {
	var account *CreditAccount
	err := s.repo.ExecTx(ctx, func(txRepo Repository) error {
		acc, err := txRepo.GetAccount(ctx, userID)
		if err == nil {
			account = acc
			return nil
		}
		if err != ErrAccountNotFound {
			return err
		}

		// Create account
		acc, err = txRepo.UpsertAccount(ctx, userID, initialCredits)
		if err != nil {
			return err
		}

		if initialCredits > 0 {
			_, err = txRepo.CreateLedgerEntry(ctx, &LedgerEntry{
				UserID:        userID,
				Amount:        initialCredits,
				EntryType:     EntryTypePromotionalGrant,
				ReferenceType: "WELCOME_BONUS",
				Description:   "Welcome promotional bonus credits",
				CreatedAt:     time.Now().UTC(),
			})
			if err != nil {
				return err
			}
		}

		account = acc
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("ensure account: %w", err)
	}
	return account, nil
}

// CheckAndReserve verifies that the user possesses sufficient credits before triggering an AI request.
func (s *service) CheckAndReserve(ctx context.Context, userID uuid.UUID, requiredCredits int32) error {
	if requiredCredits <= 0 {
		return nil
	}
	acc, err := s.repo.GetAccount(ctx, userID)
	if err != nil {
		if err == ErrAccountNotFound {
			return ErrInsufficientCredits
		}
		return fmt.Errorf("check balance: %w", err)
	}
	if acc.CachedBalance < requiredCredits {
		return ErrInsufficientCredits
	}
	return nil
}

// CommitDeduction atomically records a ledger entry and updates cached_balance with row locking.
func (s *service) CommitDeduction(
	ctx context.Context,
	userID uuid.UUID,
	amount int32,
	entryType LedgerEntryType,
	refType string,
	refID *uuid.UUID,
	description string,
) (*LedgerEntry, error) {
	if amount <= 0 {
		return nil, ErrInvalidAmount
	}
	if !entryType.IsValid() {
		return nil, ErrInvalidLedgerEntryType
	}

	var recordedEntry *LedgerEntry
	err := s.repo.ExecTx(ctx, func(txRepo Repository) error {
		// Acquire exclusive row lock on credit_accounts to serialize concurrent balance mutations
		acc, err := txRepo.GetAccountForUpdate(ctx, userID)
		if err != nil {
			if err == ErrAccountNotFound {
				return ErrInsufficientCredits
			}
			return fmt.Errorf("lock account: %w", err)
		}

		if acc.CachedBalance < amount {
			return ErrInsufficientCredits
		}

		// Deduct balance
		_, err = txRepo.UpdateBalance(ctx, userID, -amount)
		if err != nil {
			return fmt.Errorf("deduct balance: %w", err)
		}

		// Record negative ledger entry
		entry := &LedgerEntry{
			UserID:        userID,
			Amount:        -amount,
			EntryType:     entryType,
			ReferenceType: refType,
			ReferenceID:   refID,
			Description:   description,
			CreatedAt:     time.Now().UTC(),
		}
		recordedEntry, err = txRepo.CreateLedgerEntry(ctx, entry)
		if err != nil {
			return fmt.Errorf("create ledger entry: %w", err)
		}

		return nil
	})
	if err != nil {
		return nil, err
	}
	return recordedEntry, nil
}

// GrantPromotionalCredits allows administrators or system rewards to credit a candidate's balance.
func (s *service) GrantPromotionalCredits(
	ctx context.Context,
	actorID uuid.UUID,
	targetUserID uuid.UUID,
	amount int32,
	reason string,
) (*LedgerEntry, error) {
	if amount <= 0 {
		return nil, ErrInvalidAmount
	}

	var recordedEntry *LedgerEntry
	err := s.repo.ExecTx(ctx, func(txRepo Repository) error {
		// Check or create account
		acc, err := txRepo.GetAccountForUpdate(ctx, targetUserID)
		if err != nil && err != ErrAccountNotFound {
			return fmt.Errorf("get target account: %w", err)
		}

		if err == ErrAccountNotFound {
			_, err = txRepo.UpsertAccount(ctx, targetUserID, amount)
			if err != nil {
				return fmt.Errorf("upsert target account: %w", err)
			}
		} else {
			_, err = txRepo.UpdateBalance(ctx, targetUserID, amount)
			if err != nil {
				return fmt.Errorf("update target balance: %w", err)
			}
		}

		entry := &LedgerEntry{
			UserID:        targetUserID,
			Amount:        amount,
			EntryType:     EntryTypePromotionalGrant,
			ReferenceType: "ADMIN_GRANT",
			ReferenceID:   &actorID,
			Description:   reason,
			CreatedAt:     time.Now().UTC(),
		}
		recordedEntry, err = txRepo.CreateLedgerEntry(ctx, entry)
		if err != nil {
			return fmt.Errorf("create ledger grant entry: %w", err)
		}

		_ = acc
		return nil
	})
	if err != nil {
		return nil, err
	}
	return recordedEntry, nil
}

// GrantPurchasedCredits records an EntryTypePurchase ledger entry and updates cached_balance atomically.
func (s *service) GrantPurchasedCredits(
	ctx context.Context,
	userID uuid.UUID,
	amount int32,
	paymentOrderID uuid.UUID,
	description string,
) (*LedgerEntry, error) {
	if amount <= 0 {
		return nil, ErrInvalidAmount
	}

	var recordedEntry *LedgerEntry
	err := s.repo.ExecTx(ctx, func(txRepo Repository) error {
		// Acquire exclusive row lock on credit_accounts or upsert if account not created yet
		acc, err := txRepo.GetAccountForUpdate(ctx, userID)
		if err != nil && err != ErrAccountNotFound {
			return fmt.Errorf("get credit account: %w", err)
		}

		if err == ErrAccountNotFound {
			_, err = txRepo.UpsertAccount(ctx, userID, amount)
			if err != nil {
				return fmt.Errorf("upsert credit account: %w", err)
			}
		} else {
			_, err = txRepo.UpdateBalance(ctx, userID, amount)
			if err != nil {
				return fmt.Errorf("update credit balance: %w", err)
			}
		}

		entry := &LedgerEntry{
			UserID:        userID,
			Amount:        amount,
			EntryType:     EntryTypePurchase,
			ReferenceType: "RAZORPAY_PAYMENT",
			ReferenceID:   &paymentOrderID,
			Description:   description,
			CreatedAt:     time.Now().UTC(),
		}
		recordedEntry, err = txRepo.CreateLedgerEntry(ctx, entry)
		if err != nil {
			return fmt.Errorf("create purchase ledger entry: %w", err)
		}

		_ = acc
		return nil
	})
	if err != nil {
		return nil, err
	}
	return recordedEntry, nil
}

// RefundCredits adds credits back to a user's account for cancelled or failed operations.
func (s *service) RefundCredits(
	ctx context.Context,
	targetUserID uuid.UUID,
	amount int32,
	reason string,
	refID *uuid.UUID,
) (*LedgerEntry, error) {
	if amount <= 0 {
		return nil, ErrInvalidAmount
	}

	var recordedEntry *LedgerEntry
	err := s.repo.ExecTx(ctx, func(txRepo Repository) error {
		_, err := txRepo.GetAccountForUpdate(ctx, targetUserID)
		if err != nil {
			return fmt.Errorf("lock target account: %w", err)
		}

		_, err = txRepo.UpdateBalance(ctx, targetUserID, amount)
		if err != nil {
			return fmt.Errorf("credit refund balance: %w", err)
		}

		entry := &LedgerEntry{
			UserID:        targetUserID,
			Amount:        amount,
			EntryType:     EntryTypeRefund,
			ReferenceType: "REFUND",
			ReferenceID:   refID,
			Description:   reason,
			CreatedAt:     time.Now().UTC(),
		}
		recordedEntry, err = txRepo.CreateLedgerEntry(ctx, entry)
		if err != nil {
			return fmt.Errorf("create ledger refund entry: %w", err)
		}

		return nil
	})
	if err != nil {
		return nil, err
	}
	return recordedEntry, nil
}

// ReconcileBalance computes the sum of all historical ledger entries and compares with cached_balance.
func (s *service) ReconcileBalance(ctx context.Context, userID uuid.UUID) (bool, int32, int64, error) {
	acc, err := s.repo.GetAccount(ctx, userID)
	if err != nil {
		if err == ErrAccountNotFound {
			return true, 0, 0, nil
		}
		return false, 0, 0, fmt.Errorf("reconcile: get account: %w", err)
	}

	sum, err := s.repo.SumLedgerEntries(ctx, userID)
	if err != nil {
		return false, 0, 0, fmt.Errorf("reconcile: sum entries: %w", err)
	}

	isConsistent := int64(acc.CachedBalance) == sum
	return isConsistent, acc.CachedBalance, sum, nil
}
