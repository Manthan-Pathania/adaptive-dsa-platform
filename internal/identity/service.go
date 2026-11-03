package identity

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/adaptive-dsa/platform/internal/config"
	"github.com/google/uuid"
)

// Service defines user authentication and authorization application use cases.
type Service interface {
	Register(ctx context.Context, email, password, fullName, ip, userAgent string) (*User, *Session, string, string, error)
	Login(ctx context.Context, email, password, ip, userAgent string) (*User, *Session, string, string, error)
	Logout(ctx context.Context, rawToken, ip, userAgent string) error
	ValidateSession(ctx context.Context, rawToken string) (*Session, error)
	CreateAdmin(ctx context.Context, email, password, fullName, operatorIP string) (*User, error)
	AuditLog(ctx context.Context, actorID *uuid.UUID, action, targetType string, targetID *uuid.UUID, ip, userAgent string, metadata map[string]interface{})
	GetProfile(ctx context.Context, userID uuid.UUID) (*UserProfile, error)
	UpdateProfile(ctx context.Context, profile *UserProfile) (*UserProfile, error)
	SetOnUserRegistered(fn func(ctx context.Context, userID uuid.UUID) error)
}

// IdentityService coordinates authentication workflows, password security, and audit logging.
type IdentityService struct {
	repo             Repository
	cfg              *config.Config
	logger           *slog.Logger
	rateLimiter      *MemoryRateLimiter
	onUserRegistered func(ctx context.Context, userID uuid.UUID) error
}

// SetOnUserRegistered sets an optional post-registration hook (e.g. for welcome credit grants).
func (s *IdentityService) SetOnUserRegistered(fn func(ctx context.Context, userID uuid.UUID) error) {
	s.onUserRegistered = fn
}

// NewService instantiates a new IdentityService.
func NewService(repo Repository, cfg *config.Config, log *slog.Logger) *IdentityService {
	if log == nil {
		log = slog.Default()
	}

	rpm := 15
	if cfg != nil && cfg.Auth.RateLimitAuthRPM > 0 {
		rpm = cfg.Auth.RateLimitAuthRPM
	}

	return &IdentityService{
		repo:        repo,
		cfg:         cfg,
		logger:      log,
		rateLimiter: NewRateLimiter(rpm, 1*time.Minute),
	}
}

// Register creates a new candidate user account and establishes an initial session.
func (s *IdentityService) Register(ctx context.Context, email, password, fullName, ip, userAgent string) (*User, *Session, string, string, error) {
	// Rate limiting on registration attempts by IP
	if ip != "" && !s.rateLimiter.Allow("reg:"+ip) {
		return nil, nil, "", "", ErrRateLimited
	}

	cleanEmail, err := NormalizeEmail(email)
	if err != nil {
		return nil, nil, "", "", fmt.Errorf("%w: %s", ErrInvalidCredentials, err.Error())
	}

	cleanFullName := strings.TrimSpace(fullName)
	if cleanFullName == "" {
		return nil, nil, "", "", errors.New("full name cannot be empty")
	}

	if err := ValidatePassword(password); err != nil {
		return nil, nil, "", "", err
	}

	passwordHash, err := HashPassword(password)
	if err != nil {
		return nil, nil, "", "", fmt.Errorf("failed to hash password: %w", err)
	}

	// Standard registration ALWAYS assigns CANDIDATE role within atomic transaction
	var user *User
	var session *Session
	var rawToken string
	var csrfToken string

	err = s.repo.ExecTx(ctx, func(txRepo Repository) error {
		var err error
		user, err = txRepo.CreateUser(ctx, cleanEmail, passwordHash, cleanFullName, RoleCandidate)
		if err != nil {
			return err
		}

		session, rawToken, csrfToken, err = s.createSessionWithRepo(ctx, txRepo, user.ID, ip, userAgent)
		if err != nil {
			return fmt.Errorf("session creation failed: %w", err)
		}
		session.User = user

		_, err = txRepo.CreateAuditLog(ctx, &user.ID, "USER_REGISTER", "USER", &user.ID, ip, userAgent, map[string]interface{}{
			"email": user.Email,
			"role":  string(user.Role),
		})
		if err != nil {
			return fmt.Errorf("audit log failed: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, nil, "", "", err
	}

	if s.onUserRegistered != nil {
		if hookErr := s.onUserRegistered(ctx, user.ID); hookErr != nil {
			s.logger.WarnContext(ctx, "post-registration hook error", "user_id", user.ID.String(), "error", hookErr.Error())
		}
	}

	s.logger.InfoContext(ctx, "user registration successful",
		"user_id", user.ID.String(),
		"role", string(user.Role),
	)

	return user, session, rawToken, csrfToken, nil
}

// Login verifies candidate credentials, rotates sessions, and issues anti-CSRF tokens.
func (s *IdentityService) Login(ctx context.Context, email, password, ip, userAgent string) (*User, *Session, string, string, error) {
	// Rate limiting on login attempts by IP
	rateKey := "login:" + ip
	if ip != "" && !s.rateLimiter.Allow(rateKey) {
		s.AuditLog(ctx, nil, "LOGIN_RATE_LIMITED", "IP", nil, ip, userAgent, map[string]interface{}{
			"email_attempted": email,
		})
		return nil, nil, "", "", ErrRateLimited
	}

	cleanEmail, err := NormalizeEmail(email)
	if err != nil {
		return nil, nil, "", "", ErrInvalidCredentials
	}

	user, passwordHash, err := s.repo.GetUserByEmail(ctx, cleanEmail)
	if err != nil {
		// Constant-time dummy verification or generic error to prevent account enumeration
		s.AuditLog(ctx, nil, "USER_LOGIN_FAILED", "USER", nil, ip, userAgent, map[string]interface{}{
			"reason": "account not found",
		})
		return nil, nil, "", "", ErrInvalidCredentials
	}

	if !user.IsActive {
		s.AuditLog(ctx, &user.ID, "USER_LOGIN_REJECTED_INACTIVE", "USER", &user.ID, ip, userAgent, nil)
		return nil, nil, "", "", ErrUserInactive
	}

	match, err := VerifyPassword(password, passwordHash)
	if err != nil || !match {
		s.AuditLog(ctx, &user.ID, "USER_LOGIN_FAILED", "USER", &user.ID, ip, userAgent, map[string]interface{}{
			"reason": "password mismatch",
		})
		return nil, nil, "", "", ErrInvalidCredentials
	}

	// Login succeeded: reset rate limit for this IP
	if ip != "" {
		s.rateLimiter.Reset(rateKey)
	}

	// Session rotation: issue fresh session token and record audit log inside atomic transaction
	var session *Session
	var rawToken string
	var csrfToken string
	err = s.repo.ExecTx(ctx, func(txRepo Repository) error {
		var err error
		session, rawToken, csrfToken, err = s.createSessionWithRepo(ctx, txRepo, user.ID, ip, userAgent)
		if err != nil {
			return fmt.Errorf("session creation failed: %w", err)
		}
		session.User = user

		_, err = txRepo.CreateAuditLog(ctx, &user.ID, "USER_LOGIN_SUCCESS", "USER", &user.ID, ip, userAgent, map[string]interface{}{
			"session_id": session.ID.String(),
		})
		if err != nil {
			return fmt.Errorf("audit log failed: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, nil, "", "", fmt.Errorf("login failed during session generation: %w", err)
	}

	s.logger.InfoContext(ctx, "user login successful",
		"user_id", user.ID.String(),
		"role", string(user.Role),
	)

	return user, session, rawToken, csrfToken, nil
}

// Logout destroys the active server-side session and records audit event within a transaction.
func (s *IdentityService) Logout(ctx context.Context, rawToken, ip, userAgent string) error {
	if rawToken == "" {
		return nil
	}

	tokenHash := HashSessionToken(rawToken)
	session, err := s.repo.GetSessionByTokenHash(ctx, tokenHash)
	if err != nil {
		// Session already invalid or expired
		return nil
	}

	err = s.repo.ExecTx(ctx, func(txRepo Repository) error {
		if err := txRepo.DeleteSessionByTokenHash(ctx, tokenHash); err != nil {
			return err
		}
		_, err := txRepo.CreateAuditLog(ctx, &session.UserID, "USER_LOGOUT", "SESSION", &session.ID, ip, userAgent, nil)
		return err
	})
	if err != nil {
		return fmt.Errorf("failed to invalidate session: %w", err)
	}

	s.logger.InfoContext(ctx, "user session invalidated",
		"user_id", session.UserID.String(),
		"session_id", session.ID.String(),
	)

	return nil
}

// ValidateSession inspects a raw session token and returns the authenticated session and active user.
func (s *IdentityService) ValidateSession(ctx context.Context, rawToken string) (*Session, error) {
	if strings.TrimSpace(rawToken) == "" {
		return nil, ErrUnauthorized
	}

	tokenHash := HashSessionToken(rawToken)
	session, err := s.repo.GetSessionByTokenHash(ctx, tokenHash)
	if err != nil {
		return nil, ErrUnauthorized
	}

	if session.IsExpired() {
		// Clean up expired session asynchronously
		go func(h string) {
			_ = s.repo.DeleteSessionByTokenHash(context.Background(), h)
		}(tokenHash)
		return nil, ErrUnauthorized
	}

	// Inactive users cannot continue using existing active sessions
	if session.User != nil && !session.User.IsActive {
		return nil, ErrUnauthorized
	}

	// Update last active timestamp asynchronously
	go func(id uuid.UUID) {
		_ = s.repo.TouchSession(context.Background(), id)
	}(session.ID)

	return session, nil
}

// CreateAdmin securely provisions an administrator account with explicit role assignment and audit record atomically.
func (s *IdentityService) CreateAdmin(ctx context.Context, email, password, fullName, operatorIP string) (*User, error) {
	cleanEmail, err := NormalizeEmail(email)
	if err != nil {
		return nil, fmt.Errorf("invalid admin email: %w", err)
	}

	cleanFullName := strings.TrimSpace(fullName)
	if cleanFullName == "" {
		return nil, errors.New("admin full name cannot be empty")
	}

	if err := ValidatePassword(password); err != nil {
		return nil, fmt.Errorf("invalid admin password: %w", err)
	}

	passwordHash, err := HashPassword(password)
	if err != nil {
		return nil, fmt.Errorf("failed to hash admin password: %w", err)
	}

	var user *User
	err = s.repo.ExecTx(ctx, func(txRepo Repository) error {
		var err error
		user, err = txRepo.CreateUser(ctx, cleanEmail, passwordHash, cleanFullName, RoleAdmin)
		if err != nil {
			return err
		}

		_, err = txRepo.CreateAuditLog(ctx, &user.ID, "ADMIN_USER_PROVISIONED", "USER", &user.ID, operatorIP, "CLI", map[string]interface{}{
			"email": user.Email,
			"role":  string(user.Role),
		})
		if err != nil {
			return fmt.Errorf("audit log failed: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	s.logger.InfoContext(ctx, "administrator provisioned successfully",
		"admin_user_id", user.ID.String(),
		"email", user.Email,
	)

	return user, nil
}

// AuditLog records security-critical events with structured metadata.
func (s *IdentityService) AuditLog(ctx context.Context, actorID *uuid.UUID, action, targetType string, targetID *uuid.UUID, ip, userAgent string, metadata map[string]interface{}) {
	_, err := s.repo.CreateAuditLog(ctx, actorID, action, targetType, targetID, ip, userAgent, metadata)
	if err != nil {
		s.logger.ErrorContext(ctx, "failed to write audit log entry",
			"error", err.Error(),
			"action", action,
		)
	}
}

func (s *IdentityService) createSessionWithRepo(ctx context.Context, repo Repository, userID uuid.UUID, ip, userAgent string) (*Session, string, string, error) {
	rawToken, err := GenerateSessionToken()
	if err != nil {
		return nil, "", "", err
	}

	tokenHash := HashSessionToken(rawToken)

	expiry := 336 * time.Hour
	if s.cfg != nil && s.cfg.Session.CookieExpiry > 0 {
		expiry = s.cfg.Session.CookieExpiry
	}
	expiresAt := time.Now().UTC().Add(expiry)

	session, err := repo.CreateSession(ctx, userID, tokenHash, ip, userAgent, expiresAt)
	if err != nil {
		return nil, "", "", err
	}

	csrfSecret := "super-secret-csrf-key-min-32-bytes!"
	if s.cfg != nil && s.cfg.Auth.CSRFSecret != "" {
		csrfSecret = s.cfg.Auth.CSRFSecret
	}
	csrfToken := GenerateSessionCSRFToken(tokenHash, csrfSecret)

	return session, rawToken, csrfToken, nil
}

// GetProfile retrieves user profile preferences or returns sensible defaults for first-time onboarding.
func (s *IdentityService) GetProfile(ctx context.Context, userID uuid.UUID) (*UserProfile, error) {
	p, err := s.repo.GetUserProfile(ctx, userID)
	if err != nil {
		if errors.Is(err, ErrProfileNotFound) {
			return &UserProfile{
				UserID:                userID,
				TargetCompanies:       []string{},
				TimelineLabel:         "3_months",
				DailyStudyMins:        60,
				PreferredDays:         []string{"Mon", "Tue", "Wed", "Thu", "Fri", "Sat", "Sun"},
				ExperienceLevel:       "intermediate",
				FamiliarPatterns:      []string{},
				DailyGoalQuestions:    2,
				IsOnboardingCompleted: false,
			}, nil
		}
		return nil, err
	}
	return p, nil
}

// UpdateProfile updates or creates the candidate preparation profile and onboarding state.
func (s *IdentityService) UpdateProfile(ctx context.Context, profile *UserProfile) (*UserProfile, error) {
	if profile == nil || profile.UserID == uuid.Nil {
		return nil, errors.New("user ID is required for profile update")
	}

	existing, err := s.repo.GetUserProfile(ctx, profile.UserID)
	if err != nil && !errors.Is(err, ErrProfileNotFound) {
		return nil, err
	}

	if existing != nil {
		if profile.TargetCompanies != nil {
			existing.TargetCompanies = profile.TargetCompanies
		}
		if profile.TargetDate != nil {
			existing.TargetDate = profile.TargetDate
		}
		if profile.TimelineLabel != "" {
			existing.TimelineLabel = profile.TimelineLabel
		}
		if profile.DailyStudyMins > 0 {
			existing.DailyStudyMins = profile.DailyStudyMins
		}
		if profile.PreferredDays != nil {
			existing.PreferredDays = profile.PreferredDays
		}
		if profile.ExperienceLevel != "" {
			existing.ExperienceLevel = profile.ExperienceLevel
		}
		if profile.FamiliarPatterns != nil {
			existing.FamiliarPatterns = profile.FamiliarPatterns
		}
		if profile.DailyGoalQuestions > 0 {
			existing.DailyGoalQuestions = profile.DailyGoalQuestions
		}
		if profile.SelectedRoadmapID != nil {
			existing.SelectedRoadmapID = profile.SelectedRoadmapID
		}
		if profile.DiagnosticResults != nil {
			existing.DiagnosticResults = profile.DiagnosticResults
		}
		existing.IsOnboardingCompleted = profile.IsOnboardingCompleted
		profile = existing
	} else {
		if profile.TargetCompanies == nil {
			profile.TargetCompanies = []string{}
		}
		if profile.TimelineLabel == "" {
			profile.TimelineLabel = "3_MONTHS"
		}
		if profile.DailyStudyMins == 0 {
			profile.DailyStudyMins = 60
		}
		if profile.PreferredDays == nil {
			profile.PreferredDays = []string{"MON", "TUE", "WED", "THU", "FRI", "SAT", "SUN"}
		}
		if profile.ExperienceLevel == "" {
			profile.ExperienceLevel = "INTERMEDIATE"
		}
		if profile.FamiliarPatterns == nil {
			profile.FamiliarPatterns = []string{}
		}
		if profile.DailyGoalQuestions == 0 {
			profile.DailyGoalQuestions = 3
		}
	}

	return s.repo.UpsertUserProfile(ctx, profile)
}

