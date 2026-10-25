package identity

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"net/http"
	"time"
)

const SessionTokenBytes = 32

// GenerateSessionToken creates a cryptographically secure, high-entropy opaque string.
func GenerateSessionToken() (string, error) {
	bytes := make([]byte, SessionTokenBytes)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("failed to generate random session token: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(bytes), nil
}

// HashSessionToken computes the SHA-256 hex digest of an opaque raw session token.
// Only this hash is stored in PostgreSQL.
func HashSessionToken(rawToken string) string {
	hash := sha256.Sum256([]byte(rawToken))
	return hex.EncodeToString(hash[:])
}

// CreateSessionCookie generates a hardened http.Cookie containing the raw token.
func CreateSessionCookie(name, rawToken string, expiry time.Duration, secure bool, domain string) *http.Cookie {
	return &http.Cookie{
		Name:     name,
		Value:    rawToken,
		Path:     "/",
		Domain:   domain,
		MaxAge:   int(expiry.Seconds()),
		Expires:  time.Now().Add(expiry),
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
	}
}

// ExpireSessionCookie generates an invalidated cookie to instruct the browser to purge it.
func ExpireSessionCookie(name, domain string) *http.Cookie {
	return &http.Cookie{
		Name:     name,
		Value:    "",
		Path:     "/",
		Domain:   domain,
		MaxAge:   -1,
		Expires:  time.Unix(0, 0),
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	}
}
