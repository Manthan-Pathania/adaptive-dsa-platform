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

	GetUserProfile(ctx context.Context, userID uuid.UUID) (*UserProfile, error)
	UpsertUserProfile(ctx context.Context, profile *UserProfile) (*UserProfile, error)
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

func (r *SQLRepository) GetUserProfile(ctx context.Context, userID uuid.UUID) (*UserProfile, error) {
	query := `
		SELECT user_id, target_companies, target_date, timeline_label, daily_study_mins, preferred_days,
		       experience_level, familiar_patterns, daily_goal_questions, is_onboarding_completed,
		       selected_roadmap_id, diagnostic_results, created_at, updated_at
		FROM user_profiles
		WHERE user_id = $1`

	var profile UserProfile
	var targetDate pgtype.Date
	var roadmapID pgtype.UUID
	var diagBytes []byte

	err := r.pool.QueryRow(ctx, query, userID).Scan(
		&profile.UserID,
		&profile.TargetCompanies,
		&targetDate,
		&profile.TimelineLabel,
		&profile.DailyStudyMins,
		&profile.PreferredDays,
		&profile.ExperienceLevel,
		&profile.FamiliarPatterns,
		&profile.DailyGoalQuestions,
		&profile.IsOnboardingCompleted,
		&roadmapID,
		&diagBytes,
		&profile.CreatedAt,
		&profile.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrProfileNotFound
		}
		return nil, fmt.Errorf("get user profile: %w", err)
	}
	if targetDate.Valid {
		t := targetDate.Time
		profile.TargetDate = &t
	}
	if roadmapID.Valid {
		rid := uuid.UUID(roadmapID.Bytes)
		profile.SelectedRoadmapID = &rid
	}
	if len(diagBytes) > 0 {
		_ = json.Unmarshal(diagBytes, &profile.DiagnosticResults)
	}
	return &profile, nil
}

func (r *SQLRepository) UpsertUserProfile(ctx context.Context, p *UserProfile) (*UserProfile, error) {
	query := `
		INSERT INTO user_profiles (
			user_id, target_companies, target_date, timeline_label, daily_study_mins, preferred_days,
			experience_level, familiar_patterns, daily_goal_questions, is_onboarding_completed,
			selected_roadmap_id, diagnostic_results, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, CURRENT_TIMESTAMP
		) ON CONFLICT (user_id) DO UPDATE SET
			target_companies = EXCLUDED.target_companies,
			target_date = EXCLUDED.target_date,
			timeline_label = EXCLUDED.timeline_label,
			daily_study_mins = EXCLUDED.daily_study_mins,
			preferred_days = EXCLUDED.preferred_days,
			experience_level = EXCLUDED.experience_level,
			familiar_patterns = EXCLUDED.familiar_patterns,
			daily_goal_questions = EXCLUDED.daily_goal_questions,
			is_onboarding_completed = EXCLUDED.is_onboarding_completed,
			selected_roadmap_id = EXCLUDED.selected_roadmap_id,
			diagnostic_results = EXCLUDED.diagnostic_results,
			updated_at = CURRENT_TIMESTAMP
		RETURNING user_id, target_companies, target_date, timeline_label, daily_study_mins, preferred_days,
		          experience_level, familiar_patterns, daily_goal_questions, is_onboarding_completed,
		          selected_roadmap_id, diagnostic_results, created_at, updated_at`

	if p.TargetCompanies == nil {
		p.TargetCompanies = []string{}
	}
	if p.PreferredDays == nil {
		p.PreferredDays = []string{"MON", "TUE", "WED", "THU", "FRI", "SAT", "SUN"}
	}
	if p.FamiliarPatterns == nil {
		p.FamiliarPatterns = []string{}
	}

	var targetDate pgtype.Date
	if p.TargetDate != nil {
		targetDate = pgtype.Date{Time: *p.TargetDate, Valid: true}
	}
	var roadmapID pgtype.UUID
	if p.SelectedRoadmapID != nil {
		roadmapID = pgtype.UUID{Bytes: *p.SelectedRoadmapID, Valid: true}
	}
	diagBytes := []byte("{}")
	if p.DiagnosticResults != nil {
		diagBytes, _ = json.Marshal(p.DiagnosticResults)
	}

	var res UserProfile
	var resTargetDate pgtype.Date
	var resRoadmapID pgtype.UUID
	var resDiagBytes []byte

	err := r.pool.QueryRow(ctx, query,
		p.UserID,
		p.TargetCompanies,
		targetDate,
		p.TimelineLabel,
		p.DailyStudyMins,
		p.PreferredDays,
		p.ExperienceLevel,
		p.FamiliarPatterns,
		p.DailyGoalQuestions,
		p.IsOnboardingCompleted,
		roadmapID,
		diagBytes,
	).Scan(
		&res.UserID,
		&res.TargetCompanies,
		&resTargetDate,
		&res.TimelineLabel,
		&res.DailyStudyMins,
		&res.PreferredDays,
		&res.ExperienceLevel,
		&res.FamiliarPatterns,
		&res.DailyGoalQuestions,
		&res.IsOnboardingCompleted,
		&resRoadmapID,
		&resDiagBytes,
		&res.CreatedAt,
		&res.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("upsert user profile: %w", err)
	}
	if resTargetDate.Valid {
		t := resTargetDate.Time
		res.TargetDate = &t
	}
	if resRoadmapID.Valid {
		rid := uuid.UUID(resRoadmapID.Bytes)
		res.SelectedRoadmapID = &rid
	}
	if len(resDiagBytes) > 0 {
		_ = json.Unmarshal(resDiagBytes, &res.DiagnosticResults)
	}
	return &res, nil
}
