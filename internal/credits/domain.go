package credits

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

var (
	ErrInsufficientCredits       = errors.New("insufficient credit balance")
	ErrAccountNotFound           = errors.New("credit account not found")
	ErrInvalidAmount             = errors.New("credit amount must be positive")
	ErrNegativeBalanceProhibited = errors.New("credit balance cannot be negative")
	ErrInvalidLedgerEntryType    = errors.New("invalid ledger entry type")
)

type LedgerEntryType string

const (
	EntryTypePurchase         LedgerEntryType = "PURCHASE"
	EntryTypeAIUsage          LedgerEntryType = "AI_USAGE"
	EntryTypePromotionalGrant LedgerEntryType = "PROMOTIONAL_GRANT"
	EntryTypeRefund           LedgerEntryType = "REFUND"
	EntryTypeAdminAdjustment  LedgerEntryType = "ADMIN_ADJUSTMENT"
)

func (t LedgerEntryType) IsValid() bool {
	switch t {
	case EntryTypePurchase, EntryTypeAIUsage, EntryTypePromotionalGrant, EntryTypeRefund, EntryTypeAdminAdjustment:
		return true
	default:
		return false
	}
}

// CreditAccount holds the materialized cached balance for fast O(1) reads and row-level locking.
type CreditAccount struct {
	UserID        uuid.UUID `json:"user_id"`
	CachedBalance int32     `json:"cached_balance"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// LedgerEntry is an immutable record of a balance delta.
type LedgerEntry struct {
	ID            uuid.UUID       `json:"id"`
	UserID        uuid.UUID       `json:"user_id"`
	Amount        int32           `json:"amount"` // Positive for additions, negative for deductions
	EntryType     LedgerEntryType `json:"entry_type"`
	ReferenceType string          `json:"reference_type,omitempty"`
	ReferenceID   *uuid.UUID      `json:"reference_id,omitempty"`
	Description   string          `json:"description"`
	CreatedAt     time.Time       `json:"created_at"`
}

// AccountSummary provides balance and pagination metadata.
type AccountSummary struct {
	Balance       int32         `json:"balance"`
	RecentEntries []LedgerEntry `json:"recent_entries"`
	TotalEntries  int64         `json:"total_entries"`
}
