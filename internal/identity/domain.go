package identity

import (
	"errors"
	"fmt"
	"net/mail"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Role defines role-based access levels.
type Role string

const (
	RoleCandidate Role = "CANDIDATE"
	RoleAdmin     Role = "ADMIN"
)

// Common domain errors.
var (
	ErrInvalidCredentials = errors.New("invalid email or password")
	ErrDuplicateEmail     = errors.New("email address is already registered")
	ErrUserNotFound       = errors.New("user account not found")
	ErrUserInactive       = errors.New("user account is deactivated")
	ErrSessionNotFound    = errors.New("session not found or expired")
	ErrUnauthorized       = errors.New("unauthorized: valid session required")
	ErrForbidden          = errors.New("forbidden: administrator privileges required")
	ErrInvalidCSRF        = errors.New("invalid or missing CSRF token")
	ErrRateLimited        = errors.New("too many authentication attempts: please try again later")
)

// User represents a registered platform account.
type User struct {
	ID        uuid.UUID `json:"id"`
	Email     string    `json:"email"`
	FullName  string    `json:"full_name"`
	Role      Role      `json:"role"`
	IsActive  bool      `json:"is_active"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Session represents an authenticated server-side user session.
type Session struct {
	ID               uuid.UUID `json:"id"`
	UserID           uuid.UUID `json:"user_id"`
	SessionTokenHash string    `json:"-"` // Never serialized to JSON
	IPAddress        string    `json:"ip_address,omitempty"`
	UserAgent        string    `json:"user_agent,omitempty"`
	ExpiresAt        time.Time `json:"expires_at"`
	CreatedAt        time.Time `json:"created_at"`
	LastActiveAt     time.Time `json:"last_active_at"`
	User             *User     `json:"user,omitempty"`
}

// IsExpired checks whether the session has passed its expiration time.
func (s *Session) IsExpired() bool {
	return time.Now().UTC().After(s.ExpiresAt)
}

// AuditLog captures immutable security and governance records.
type AuditLog struct {
	ID          uuid.UUID              `json:"id"`
	ActorUserID *uuid.UUID             `json:"actor_user_id,omitempty"`
	Action      string                 `json:"action"`
	TargetType  string                 `json:"target_type"`
	TargetID    *uuid.UUID             `json:"target_id,omitempty"`
	IPAddress   string                 `json:"ip_address,omitempty"`
	UserAgent   string                 `json:"user_agent,omitempty"`
	Metadata    map[string]interface{} `json:"metadata"`
	CreatedAt   time.Time              `json:"created_at"`
}

// NormalizeEmail cleans and lowercases an email address for consistent uniqueness.
func NormalizeEmail(email string) (string, error) {
	cleaned := strings.ToLower(strings.TrimSpace(email))
	if cleaned == "" {
		return "", errors.New("email address cannot be empty")
	}
	parsed, err := mail.ParseAddress(cleaned)
	if err != nil {
		return "", fmt.Errorf("invalid email address format: %w", err)
	}
	return parsed.Address, nil
}

// ValidatePassword enforces security constraints on plaintext passwords.
func ValidatePassword(password string) error {
	if len(password) < 8 {
		return errors.New("password must be at least 8 characters in length")
	}
	if len(password) > 72 {
		return errors.New("password cannot exceed 72 characters in length")
	}
	return nil
}
