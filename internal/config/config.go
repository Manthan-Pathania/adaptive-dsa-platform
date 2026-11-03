package config

import (
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/caarlos0/env/v11"
)

// Environment constants
const (
	EnvDevelopment = "development"
	EnvTest        = "test"
	EnvProduction  = "production"
)

// Config holds the validated runtime configuration for the platform.
type Config struct {
	AppEnv   string `env:"APP_ENV" envDefault:"development"`
	Server   ServerConfig
	Database DatabaseConfig
	Log      LogConfig
	Session  SessionConfig
	Auth     AuthConfig
	AI       AIConfig
	Payments PaymentsConfig
}

// PaymentsConfig holds Razorpay test-mode API credentials and webhook verification secrets.
type PaymentsConfig struct {
	RazorpayKeyID         string `env:"RAZORPAY_KEY_ID" envDefault:"rzp_test_placeholder_key_id"`
	RazorpayKeySecret     string `env:"RAZORPAY_KEY_SECRET" envDefault:"rzp_test_placeholder_key_secret"`
	RazorpayWebhookSecret string `env:"RAZORPAY_WEBHOOK_SECRET" envDefault:"rzp_test_placeholder_webhook_secret"`
}

// ServerConfig defines HTTP server transport, security and timeout settings.
type ServerConfig struct {
	Host               string        `env:"SERVER_HOST" envDefault:"0.0.0.0"`
	Port               int           `env:"SERVER_PORT" envDefault:"8080"`
	ReadTimeout        time.Duration `env:"SERVER_READ_TIMEOUT" envDefault:"5s"`
	WriteTimeout       time.Duration `env:"SERVER_WRITE_TIMEOUT" envDefault:"10s"`
	IdleTimeout        time.Duration `env:"SERVER_IDLE_TIMEOUT" envDefault:"120s"`
	ShutdownTimeout    time.Duration `env:"SERVER_SHUTDOWN_TIMEOUT" envDefault:"10s"`
	CORSAllowedOrigins string        `env:"CORS_ALLOWED_ORIGINS" envDefault:"http://localhost:3000,http://127.0.0.1:3000"`
	MaxBodyBytes       int64         `env:"SERVER_MAX_BODY_BYTES" envDefault:"1048576"` // 1MB default
}

// DatabaseConfig defines PostgreSQL connection pooling parameters.
type DatabaseConfig struct {
	URL               string        `env:"DATABASE_URL,required"`
	MaxConns          int32         `env:"DB_MAX_CONNS" envDefault:"25"`
	MinConns          int32         `env:"DB_MIN_CONNS" envDefault:"5"`
	MaxConnLifetime   time.Duration `env:"DB_MAX_CONN_LIFETIME" envDefault:"30m"`
	MaxConnIdleTime   time.Duration `env:"DB_MAX_CONN_IDLE_TIME" envDefault:"5m"`
	HealthCheckPeriod time.Duration `env:"DB_HEALTH_CHECK_PERIOD" envDefault:"1m"`
	ConnectTimeout    time.Duration `env:"DB_CONNECT_TIMEOUT" envDefault:"5s"`
}

// LogConfig defines structured logging parameters.
type LogConfig struct {
	Level  string `env:"LOG_LEVEL" envDefault:"info"`
	Format string `env:"LOG_FORMAT" envDefault:"json"`
}

// SessionConfig defines cookie and session parameters.
type SessionConfig struct {
	CookieName   string        `env:"SESSION_COOKIE_NAME" envDefault:"session_token"`
	CookieExpiry time.Duration `env:"SESSION_COOKIE_EXPIRY" envDefault:"336h"` // 14 days
	CookieSecure bool          `env:"SESSION_COOKIE_SECURE" envDefault:"false"`
	CookieDomain string        `env:"SESSION_COOKIE_DOMAIN" envDefault:""`
}

// AuthConfig defines rate limits and cryptographic secrets for auth and CSRF.
type AuthConfig struct {
	CSRFSecret       string `env:"CSRF_SECRET" envDefault:"super-secret-csrf-key-min-32-bytes!"`
	RateLimitAuthRPM int    `env:"AUTH_RATE_LIMIT_RPM" envDefault:"15"`
}

// AIConfig defines Google Gemini AI tutor and credit allocation parameters.
type AIConfig struct {
	GeminiAPIKey   string        `env:"GEMINI_API_KEY" envDefault:""`
	GeminiModel    string        `env:"GEMINI_MODEL" envDefault:"gemini-3.8-flash"`
	GeminiTimeout  time.Duration `env:"GEMINI_TIMEOUT" envDefault:"15s"`
	GeminiRPM      int           `env:"GEMINI_RPM" envDefault:"15"`
	WelcomeCredits int32         `env:"WELCOME_CREDITS" envDefault:"50"`
}

// Load parses environment variables and validates configuration.
func Load() (*Config, error) {
	var cfg Config
	if err := env.Parse(&cfg); err != nil {
		return nil, fmt.Errorf("config: parsing environment variables failed: %w", err)
	}

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("config: validation failed: %w", err)
	}

	return &cfg, nil
}

// Validate checks that all required configuration constraints and bounds are satisfied.
func (c *Config) Validate() error {
	switch c.AppEnv {
	case EnvDevelopment, EnvTest, EnvProduction:
	default:
		return fmt.Errorf("invalid APP_ENV '%s': must be development, test, or production", c.AppEnv)
	}

	if c.Server.Port < 1 || c.Server.Port > 65535 {
		return fmt.Errorf("invalid SERVER_PORT %d: must be between 1 and 65535", c.Server.Port)
	}

	if c.Server.ReadTimeout <= 0 {
		return fmt.Errorf("SERVER_READ_TIMEOUT must be positive, got %s", c.Server.ReadTimeout)
	}
	if c.Server.WriteTimeout <= 0 {
		return fmt.Errorf("SERVER_WRITE_TIMEOUT must be positive, got %s", c.Server.WriteTimeout)
	}
	if c.Server.ShutdownTimeout <= 0 {
		return fmt.Errorf("SERVER_SHUTDOWN_TIMEOUT must be positive, got %s", c.Server.ShutdownTimeout)
	}

	// Database validation
	if strings.TrimSpace(c.Database.URL) == "" {
		return fmt.Errorf("DATABASE_URL must not be empty")
	}

	parsedURL, err := url.Parse(c.Database.URL)
	if err != nil || (parsedURL.Scheme != "postgres" && parsedURL.Scheme != "postgresql") {
		return fmt.Errorf("invalid DATABASE_URL scheme: must start with postgres:// or postgresql://")
	}

	if c.Database.MaxConns <= 0 {
		return fmt.Errorf("DB_MAX_CONNS must be greater than 0, got %d", c.Database.MaxConns)
	}
	if c.Database.MinConns < 0 {
		return fmt.Errorf("DB_MIN_CONNS must be non-negative, got %d", c.Database.MinConns)
	}
	if c.Database.MinConns > c.Database.MaxConns {
		return fmt.Errorf("DB_MIN_CONNS (%d) cannot exceed DB_MAX_CONNS (%d)", c.Database.MinConns, c.Database.MaxConns)
	}

	// Log validation
	switch strings.ToLower(c.Log.Level) {
	case "debug", "info", "warn", "error":
	default:
		return fmt.Errorf("invalid LOG_LEVEL '%s': must be debug, info, warn, or error", c.Log.Level)
	}

	switch strings.ToLower(c.Log.Format) {
	case "json", "text":
	default:
		return fmt.Errorf("invalid LOG_FORMAT '%s': must be json or text", c.Log.Format)
	}

	// Session validation
	if strings.TrimSpace(c.Session.CookieName) == "" {
		return fmt.Errorf("SESSION_COOKIE_NAME must not be empty")
	}
	if c.Session.CookieExpiry <= 0 {
		return fmt.Errorf("SESSION_COOKIE_EXPIRY must be positive, got %s", c.Session.CookieExpiry)
	}
	if c.AppEnv == EnvProduction {
		if !c.Session.CookieSecure {
			return fmt.Errorf("SESSION_COOKIE_SECURE must be true in production")
		}
		if c.Log.Format != "json" {
			return fmt.Errorf("LOG_FORMAT must be json in production, got '%s'", c.Log.Format)
		}
		if c.Auth.CSRFSecret == "super-secret-csrf-key-min-32-bytes!" || strings.Contains(strings.ToLower(c.Auth.CSRFSecret), "placeholder") {
			return fmt.Errorf("insecure default or placeholder CSRF_SECRET is not permitted in production")
		}
		if strings.Contains(c.Server.CORSAllowedOrigins, "*") {
			return fmt.Errorf("wildcard CORS origins are strictly forbidden in production")
		}
	}

	// Auth & CSRF validation
	if c.Auth.RateLimitAuthRPM <= 0 {
		return fmt.Errorf("AUTH_RATE_LIMIT_RPM must be positive, got %d", c.Auth.RateLimitAuthRPM)
	}
	if len(c.Auth.CSRFSecret) < 32 {
		return fmt.Errorf("CSRF_SECRET must be at least 32 characters, got %d", len(c.Auth.CSRFSecret))
	}

	// AI validation
	if c.AI.GeminiTimeout <= 0 {
		return fmt.Errorf("GEMINI_TIMEOUT must be positive, got %s", c.AI.GeminiTimeout)
	}
	if c.AI.GeminiRPM <= 0 {
		return fmt.Errorf("GEMINI_RPM must be positive, got %d", c.AI.GeminiRPM)
	}
	if c.AI.WelcomeCredits < 0 {
		return fmt.Errorf("WELCOME_CREDITS must be non-negative, got %d", c.AI.WelcomeCredits)
	}

	return nil
}

// AllowedOriginsList returns parsed slice of allowed CORS origins.
func (s *ServerConfig) AllowedOriginsList() []string {
	parts := strings.Split(s.CORSAllowedOrigins, ",")
	var result []string
	for _, p := range parts {
		trimmed := strings.TrimSpace(p)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

// RedactedDatabaseURL returns the connection URL with sensitive credentials masked.
func (d *DatabaseConfig) RedactedDatabaseURL() string {
	parsed, err := url.Parse(d.URL)
	if err != nil {
		return "[malformed database URL]"
	}
	return parsed.Redacted()
}

// Address returns host:port formatted string.
func (s *ServerConfig) Address() string {
	return fmt.Sprintf("%s:%d", s.Host, s.Port)
}
