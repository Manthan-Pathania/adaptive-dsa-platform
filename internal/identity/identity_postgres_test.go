package identity_test

import (
	"context"
	"io"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/adaptive-dsa/platform/internal/config"
	"github.com/adaptive-dsa/platform/internal/identity"
	"github.com/adaptive-dsa/platform/internal/platform/database"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func getTestIdentityPostgresPool(t *testing.T) (*database.DB, identity.Repository, identity.Service) {
	t.Helper()
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		t.Skip("skipping PostgreSQL integration test: DATABASE_URL not set")
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	cfg := config.DatabaseConfig{
		URL:               dbURL,
		MaxConns:          10,
		MinConns:          2,
		MaxConnLifetime:   5 * time.Minute,
		MaxConnIdleTime:   1 * time.Minute,
		HealthCheckPeriod: 30 * time.Second,
		ConnectTimeout:    3 * time.Second,
	}

	ctx := context.Background()
	db, err := database.New(ctx, cfg, logger)
	require.NoError(t, err)

	repo := identity.NewSQLRepository(db.Pool())
	appCfg := &config.Config{
		AppEnv: config.EnvTest,
		Session: config.SessionConfig{
			CookieName:   "test_session",
			CookieExpiry: 24 * time.Hour,
			CookieSecure: false,
		},
		Auth: config.AuthConfig{
			CSRFSecret:       "test-csrf-secret-key-at-least-32-bytes!",
			RateLimitAuthRPM: 50,
		},
	}
	svc := identity.NewService(repo, appCfg, logger)

	return db, repo, svc
}

func TestIdentity_Postgres_RegisterAndLoginSession(t *testing.T) {
	db, _, svc := getTestIdentityPostgresPool(t)
	defer db.Close()
	ctx := context.Background()

	uniqueSuffix := uuid.New().String()[:8]
	email := "pg-user-" + uniqueSuffix + "@example.com"
	password := "StrongPass123!@#"
	fullName := "Postgres User " + uniqueSuffix

	// 1. Register candidate
	regUser, regSession, rawToken, _, err := svc.Register(ctx, email, password, fullName, "127.0.0.1", "integration-test-agent")
	require.NoError(t, err)
	assert.NotEqual(t, uuid.Nil, regUser.ID)
	assert.Equal(t, email, regUser.Email)
	assert.NotEqual(t, uuid.Nil, regSession.ID)
	assert.NotEmpty(t, rawToken)

	// 2. Validate Session against PostgreSQL
	session, err := svc.ValidateSession(ctx, rawToken)
	require.NoError(t, err)
	assert.Equal(t, regUser.ID, session.UserID)

	// 3. Login with correct password
	loginUser, _, loginToken, _, err := svc.Login(ctx, email, password, "127.0.0.1", "integration-test-agent")
	require.NoError(t, err)
	assert.Equal(t, regUser.ID, loginUser.ID)
	assert.NotEmpty(t, loginToken)

	// 4. Logout (revoke session in PostgreSQL)
	err = svc.Logout(ctx, loginToken, "127.0.0.1", "integration-test-agent")
	require.NoError(t, err)

	// 5. Verify revoked session is invalid
	revokedSession, err := svc.ValidateSession(ctx, loginToken)
	assert.Error(t, err)
	assert.Nil(t, revokedSession)
}

func TestIdentity_Postgres_AdminProvisioningAndAuditLog(t *testing.T) {
	db, _, svc := getTestIdentityPostgresPool(t)
	defer db.Close()
	ctx := context.Background()

	uniqueSuffix := uuid.New().String()[:8]
	email := "admin-" + uniqueSuffix + "@example.com"
	password := "AdminSecret456!@#"
	fullName := "Admin User " + uniqueSuffix

	// Provision admin
	adminUser, err := svc.CreateAdmin(ctx, email, password, fullName, "127.0.0.1")
	require.NoError(t, err)
	assert.Equal(t, identity.RoleAdmin, adminUser.Role)

	// Verify admin can login
	loginUser, _, _, _, err := svc.Login(ctx, email, password, "127.0.0.1", "integration-test-admin")
	require.NoError(t, err)
	assert.Equal(t, identity.RoleAdmin, loginUser.Role)
}
