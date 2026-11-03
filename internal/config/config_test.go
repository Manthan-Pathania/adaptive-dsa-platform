package config

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func validTestConfig() Config {
	return Config{
		AppEnv: EnvDevelopment,
		Server: ServerConfig{
			Host:            "0.0.0.0",
			Port:            8080,
			ReadTimeout:     5 * time.Second,
			WriteTimeout:    10 * time.Second,
			ShutdownTimeout: 5 * time.Second,
		},
		Database: DatabaseConfig{
			URL:      "postgres://user:pass@localhost:5432/testdb?sslmode=disable",
			MaxConns: 25,
			MinConns: 5,
		},
		Log: LogConfig{
			Level:  "info",
			Format: "json",
		},
		Session: SessionConfig{
			CookieName:   "session_token",
			CookieExpiry: 336 * time.Hour,
			CookieSecure: false,
		},
		Auth: AuthConfig{
			CSRFSecret:       "super-secret-csrf-key-min-32-bytes!",
			RateLimitAuthRPM: 15,
		},
		AI: AIConfig{
			GeminiAPIKey:   "",
			GeminiModel:    "gemini-3.8-flash",
			GeminiTimeout:  15 * time.Second,
			GeminiRPM:      15,
			WelcomeCredits: 50,
		},
		Payments: PaymentsConfig{
			RazorpayKeyID:         "rzp_test_placeholder_key_id",
			RazorpayKeySecret:     "rzp_test_placeholder_key_secret",
			RazorpayWebhookSecret: "rzp_test_placeholder_webhook_secret",
		},
	}
}

func TestConfig_Validation(t *testing.T) {
	tests := []struct {
		name        string
		mutate      func(c *Config)
		expectedErr string
	}{
		{
			name:        "valid default configuration",
			mutate:      func(c *Config) {},
			expectedErr: "",
		},
		{
			name: "invalid app env",
			mutate: func(c *Config) {
				c.AppEnv = "staging"
			},
			expectedErr: "invalid APP_ENV 'staging'",
		},
		{
			name: "invalid port zero",
			mutate: func(c *Config) {
				c.Server.Port = 0
			},
			expectedErr: "invalid SERVER_PORT 0",
		},
		{
			name: "invalid port too high",
			mutate: func(c *Config) {
				c.Server.Port = 70000
			},
			expectedErr: "invalid SERVER_PORT 70000",
		},
		{
			name: "empty database URL",
			mutate: func(c *Config) {
				c.Database.URL = ""
			},
			expectedErr: "DATABASE_URL must not be empty",
		},
		{
			name: "invalid database URL scheme",
			mutate: func(c *Config) {
				c.Database.URL = "http://localhost:5432/db"
			},
			expectedErr: "invalid DATABASE_URL scheme",
		},
		{
			name: "min conns exceeds max conns",
			mutate: func(c *Config) {
				c.Database.MaxConns = 5
				c.Database.MinConns = 10
			},
			expectedErr: "DB_MIN_CONNS (10) cannot exceed DB_MAX_CONNS (5)",
		},
		{
			name: "invalid log level",
			mutate: func(c *Config) {
				c.Log.Level = "verbose"
			},
			expectedErr: "invalid LOG_LEVEL 'verbose'",
		},
		{
			name: "invalid log format",
			mutate: func(c *Config) {
				c.Log.Format = "yaml"
			},
			expectedErr: "invalid LOG_FORMAT 'yaml'",
		},
		{
			name: "empty session cookie name",
			mutate: func(c *Config) {
				c.Session.CookieName = "   "
			},
			expectedErr: "SESSION_COOKIE_NAME must not be empty",
		},
		{
			name: "invalid session cookie expiry",
			mutate: func(c *Config) {
				c.Session.CookieExpiry = 0
			},
			expectedErr: "SESSION_COOKIE_EXPIRY must be positive",
		},
		{
			name: "production requires secure cookie",
			mutate: func(c *Config) {
				c.AppEnv = EnvProduction
				c.Session.CookieSecure = false
			},
			expectedErr: "SESSION_COOKIE_SECURE must be true in production",
		},
		{
			name: "production requires json log format",
			mutate: func(c *Config) {
				c.AppEnv = EnvProduction
				c.Session.CookieSecure = true
				c.Log.Format = "text"
			},
			expectedErr: "LOG_FORMAT must be json in production",
		},
		{
			name: "production rejects default csrf secret",
			mutate: func(c *Config) {
				c.AppEnv = EnvProduction
				c.Session.CookieSecure = true
				c.Log.Format = "json"
				c.Auth.CSRFSecret = "super-secret-csrf-key-min-32-bytes!"
			},
			expectedErr: "insecure default or placeholder CSRF_SECRET is not permitted in production",
		},
		{
			name: "production rejects wildcard cors origins",
			mutate: func(c *Config) {
				c.AppEnv = EnvProduction
				c.Session.CookieSecure = true
				c.Log.Format = "json"
				c.Auth.CSRFSecret = "a-real-production-secret-min-32-characters-long!"
				c.Server.CORSAllowedOrigins = "*"
			},
			expectedErr: "wildcard CORS origins are strictly forbidden in production",
		},
		{
			name: "production valid configuration",
			mutate: func(c *Config) {
				c.AppEnv = EnvProduction
				c.Session.CookieSecure = true
				c.Log.Format = "json"
				c.Auth.CSRFSecret = "a-real-production-secret-min-32-characters-long!"
				c.Server.CORSAllowedOrigins = "https://app.example.com,https://dsa.example.com"
			},
			expectedErr: "",
		},
		{
			name: "csrf secret too short",
			mutate: func(c *Config) {
				c.Auth.CSRFSecret = "too-short"
			},
			expectedErr: "CSRF_SECRET must be at least 32 characters",
		},
		{
			name: "invalid auth rate limit",
			mutate: func(c *Config) {
				c.Auth.RateLimitAuthRPM = 0
			},
			expectedErr: "AUTH_RATE_LIMIT_RPM must be positive",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := validTestConfig()
			tt.mutate(&cfg)
			err := cfg.Validate()
			if tt.expectedErr == "" {
				assert.NoError(t, err)
			} else {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedErr)
			}
		})
	}
}

func TestConfig_AllowedOriginsList(t *testing.T) {
	s := ServerConfig{
		CORSAllowedOrigins: " https://app.example.com , https://api.example.com , ",
	}
	origins := s.AllowedOriginsList()
	assert.Equal(t, []string{"https://app.example.com", "https://api.example.com"}, origins)
}

func TestConfig_RedactedDatabaseURL(t *testing.T) {
	dbCfg := DatabaseConfig{
		URL: "postgres://admin:SuperSecretPassword123@db.example.com:5432/prod_dsa?sslmode=require",
	}

	redacted := dbCfg.RedactedDatabaseURL()
	assert.NotContains(t, redacted, "SuperSecretPassword123")
	assert.Contains(t, redacted, "admin:xxxxx@db.example.com:5432/prod_dsa")
}

func TestConfig_LoadFromEnv(t *testing.T) {
	// Set required env vars
	os.Setenv("DATABASE_URL", "postgres://test_user:test_pass@localhost:5432/test_db?sslmode=disable")
	os.Setenv("SERVER_PORT", "9090")
	os.Setenv("APP_ENV", "test")
	defer func() {
		os.Unsetenv("DATABASE_URL")
		os.Unsetenv("SERVER_PORT")
		os.Unsetenv("APP_ENV")
	}()

	cfg, err := Load()
	require.NoError(t, err)
	assert.Equal(t, 9090, cfg.Server.Port)
	assert.Equal(t, EnvTest, cfg.AppEnv)
	assert.Equal(t, "postgres://test_user:test_pass@localhost:5432/test_db?sslmode=disable", cfg.Database.URL)
	assert.Equal(t, "0.0.0.0:9090", cfg.Server.Address())
	assert.Equal(t, "session_token", cfg.Session.CookieName)
	assert.Equal(t, 15, cfg.Auth.RateLimitAuthRPM)
}
