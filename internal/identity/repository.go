package identity

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/adaptive-dsa/platform/internal/database/generated"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Repository defines data-access operations for identity and sessions.
type Repository interface {
	CreateUser(ctx context.Context, email, passwordHash, fullName string, role Role) (*User, error)
	GetUserByEmail(ctx context.Context, email string) (*User, string, error)
	GetUserByID(ctx context.Context, id uuid.UUID) (*User, error)
	UpdateUserRole(ctx context.Context, id uuid.UUID, role Role) (*User, error)

	CreateSession(ctx context.Context, userID uuid.UUID, tokenHash, ip, userAgent string, expiresAt time.Time) (*Session, error)
	GetSessionByTokenHash(ctx context.Context, tokenHash string) (*Session, error)
	TouchSession(ctx context.Context, sessionID uuid.UUID) error
	DeleteSessionByTokenHash(ctx context.Context, tokenHash string) error
	DeleteUserSessions(ctx context.Context, userID uuid.UUID) error

	CreateAuditLog(ctx context.Context, actorID *uuid.UUID, action, targetType string, targetID *uuid.UUID, ip, userAgent string, metadata map[string]interface{}) (*AuditLog, error)
	ExecTx(ctx context.Context, fn func(repo Repository) error) error
}

// SQLRepository implements Repository using generated sqlc queries on pgxpool.
type SQLRepository struct {
	pool    *pgxpool.Pool
	queries *generated.Queries
}

// NewSQLRepository instantiates a new SQLRepository.
func NewSQLRepository(pool *pgxpool.Pool) *SQLRepository {
	return &SQLRepository{
		pool:    pool,
		queries: generated.New(pool),
	}
}

// ExecTx executes a function within a database transaction.
func (r *SQLRepository) ExecTx(ctx context.Context, fn func(repo Repository) error) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("repository: begin transaction failed: %w", err)
	}
	defer tx.Rollback(ctx)

	txRepo := &SQLRepository{
		pool:    r.pool,
		queries: r.queries.WithTx(tx),
	}

	if err := fn(txRepo); err != nil {
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("repository: commit transaction failed: %w", err)
	}
	return nil
}

func (r *SQLRepository) CreateUser(ctx context.Context, email, passwordHash, fullName string, role Role) (*User, error) {
	row, err := r.queries.CreateUser(ctx, generated.CreateUserParams{
		Email:        email,
		PasswordHash: passwordHash,
		FullName:     fullName,
		Role:         generated.UserRole(role),
	})
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return nil, ErrDuplicateEmail
		}
		return nil, fmt.Errorf("repository: create user failed: %w", err)
	}

	userID, err := uuid.FromBytes(row.ID.Bytes[:])
	if err != nil {
		return nil, fmt.Errorf("repository: invalid user uuid: %w", err)
	}

	return &User{
		ID:        userID,
		Email:     row.Email,
		FullName:  row.FullName,
		Role:      Role(row.Role),
		IsActive:  row.IsActive,
		CreatedAt: row.CreatedAt.Time,
		UpdatedAt: row.UpdatedAt.Time,
	}, nil
}

func (r *SQLRepository) GetUserByEmail(ctx context.Context, email string) (*User, string, error) {
	row, err := r.queries.GetUserByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, "", ErrUserNotFound
		}
		return nil, "", fmt.Errorf("repository: get user by email failed: %w", err)
	}

	userID, err := uuid.FromBytes(row.ID.Bytes[:])
	if err != nil {
		return nil, "", fmt.Errorf("repository: invalid user uuid: %w", err)
	}

	user := &User{
		ID:        userID,
		Email:     row.Email,
		FullName:  row.FullName,
		Role:      Role(row.Role),
		IsActive:  row.IsActive,
		CreatedAt: row.CreatedAt.Time,
		UpdatedAt: row.UpdatedAt.Time,
	}

	return user, row.PasswordHash, nil
}

func (r *SQLRepository) GetUserByID(ctx context.Context, id uuid.UUID) (*User, error) {
	pgUUID := pgtype.UUID{Bytes: id, Valid: true}
	row, err := r.queries.GetUserByID(ctx, pgUUID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("repository: get user by id failed: %w", err)
	}

	userID, err := uuid.FromBytes(row.ID.Bytes[:])
	if err != nil {
		return nil, fmt.Errorf("repository: invalid user uuid: %w", err)
	}

	return &User{
		ID:        userID,
		Email:     row.Email,
		FullName:  row.FullName,
		Role:      Role(row.Role),
		IsActive:  row.IsActive,
		CreatedAt: row.CreatedAt.Time,
		UpdatedAt: row.UpdatedAt.Time,
	}, nil
}

func (r *SQLRepository) UpdateUserRole(ctx context.Context, id uuid.UUID, role Role) (*User, error) {
	pgUUID := pgtype.UUID{Bytes: id, Valid: true}
	row, err := r.queries.UpdateUserRole(ctx, generated.UpdateUserRoleParams{
		ID:   pgUUID,
		Role: generated.UserRole(role),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("repository: update user role failed: %w", err)
	}

	userID, err := uuid.FromBytes(row.ID.Bytes[:])
	if err != nil {
		return nil, fmt.Errorf("repository: invalid user uuid: %w", err)
	}

	return &User{
		ID:        userID,
		Email:     row.Email,
		FullName:  row.FullName,
		Role:      Role(row.Role),
		IsActive:  row.IsActive,
		CreatedAt: row.CreatedAt.Time,
		UpdatedAt: row.UpdatedAt.Time,
	}, nil
}

func (r *SQLRepository) CreateSession(ctx context.Context, userID uuid.UUID, tokenHash, ip, userAgent string, expiresAt time.Time) (*Session, error) {
	pgUserID := pgtype.UUID{Bytes: userID, Valid: true}
	row, err := r.queries.CreateSession(ctx, generated.CreateSessionParams{
		UserID:           pgUserID,
		SessionTokenHash: tokenHash,
		IpAddress:        pgtype.Text{String: ip, Valid: ip != ""},
		UserAgent:        pgtype.Text{String: userAgent, Valid: userAgent != ""},
		ExpiresAt:        pgtype.Timestamptz{Time: expiresAt, Valid: true},
	})
	if err != nil {
		return nil, fmt.Errorf("repository: create session failed: %w", err)
	}

	sessID, err := uuid.FromBytes(row.ID.Bytes[:])
	if err != nil {
		return nil, fmt.Errorf("repository: invalid session uuid: %w", err)
	}

	return &Session{
		ID:               sessID,
		UserID:           userID,
		SessionTokenHash: row.SessionTokenHash,
		IPAddress:        row.IpAddress.String,
		UserAgent:        row.UserAgent.String,
		ExpiresAt:        row.ExpiresAt.Time,
		CreatedAt:        row.CreatedAt.Time,
		LastActiveAt:     row.LastActiveAt.Time,
	}, nil
}

func (r *SQLRepository) GetSessionByTokenHash(ctx context.Context, tokenHash string) (*Session, error) {
	row, err := r.queries.GetSessionWithUser(ctx, tokenHash)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrSessionNotFound
		}
		return nil, fmt.Errorf("repository: get session failed: %w", err)
	}

	sessID, err := uuid.FromBytes(row.SessionID.Bytes[:])
	if err != nil {
		return nil, fmt.Errorf("repository: invalid session uuid: %w", err)
	}

	userID, err := uuid.FromBytes(row.UserID.Bytes[:])
	if err != nil {
		return nil, fmt.Errorf("repository: invalid user uuid: %w", err)
	}

	return &Session{
		ID:               sessID,
		UserID:           userID,
		SessionTokenHash: row.SessionTokenHash,
		IPAddress:        row.IpAddress.String,
		UserAgent:        row.UserAgent.String,
		ExpiresAt:        row.ExpiresAt.Time,
		CreatedAt:        row.SessionCreatedAt.Time,
		LastActiveAt:     row.LastActiveAt.Time,
		User: &User{
			ID:        userID,
			Email:     row.Email,
			FullName:  row.FullName,
			Role:      Role(row.Role),
			IsActive:  row.IsActive,
			CreatedAt: row.UserCreatedAt.Time,
			UpdatedAt: row.UserUpdatedAt.Time,
		},
	}, nil
}

func (r *SQLRepository) TouchSession(ctx context.Context, sessionID uuid.UUID) error {
	pgUUID := pgtype.UUID{Bytes: sessionID, Valid: true}
	return r.queries.TouchSession(ctx, pgUUID)
}

func (r *SQLRepository) DeleteSessionByTokenHash(ctx context.Context, tokenHash string) error {
	return r.queries.DeleteSessionByTokenHash(ctx, tokenHash)
}

func (r *SQLRepository) DeleteUserSessions(ctx context.Context, userID uuid.UUID) error {
	pgUUID := pgtype.UUID{Bytes: userID, Valid: true}
	return r.queries.DeleteUserSessions(ctx, pgUUID)
}

func (r *SQLRepository) CreateAuditLog(ctx context.Context, actorID *uuid.UUID, action, targetType string, targetID *uuid.UUID, ip, userAgent string, metadata map[string]interface{}) (*AuditLog, error) {
	var pgActorID pgtype.UUID
	if actorID != nil {
		pgActorID = pgtype.UUID{Bytes: *actorID, Valid: true}
	}

	var pgTargetID pgtype.UUID
	if targetID != nil {
		pgTargetID = pgtype.UUID{Bytes: *targetID, Valid: true}
	}

	metaBytes := []byte("{}")
	if metadata != nil {
		var err error
		metaBytes, err = json.Marshal(metadata)
		if err != nil {
			return nil, fmt.Errorf("repository: marshal audit metadata failed: %w", err)
		}
	}

	row, err := r.queries.CreateAuditLog(ctx, generated.CreateAuditLogParams{
		ActorUserID: pgActorID,
		Action:      action,
		TargetType:  targetType,
		TargetID:    pgTargetID,
		IpAddress:   pgtype.Text{String: ip, Valid: ip != ""},
		UserAgent:   pgtype.Text{String: userAgent, Valid: userAgent != ""},
		Metadata:    metaBytes,
	})
	if err != nil {
		return nil, fmt.Errorf("repository: create audit log failed: %w", err)
	}

	logID, err := uuid.FromBytes(row.ID.Bytes[:])
	if err != nil {
		return nil, fmt.Errorf("repository: invalid audit log uuid: %w", err)
	}

	var unmarshaledMeta map[string]interface{}
	_ = json.Unmarshal(row.Metadata, &unmarshaledMeta)

	return &AuditLog{
		ID:          logID,
		ActorUserID: actorID,
		Action:      row.Action,
		TargetType:  row.TargetType,
		TargetID:    targetID,
		IPAddress:   row.IpAddress.String,
		UserAgent:   row.UserAgent.String,
		Metadata:    unmarshaledMeta,
		CreatedAt:   row.CreatedAt.Time,
	}, nil
}
