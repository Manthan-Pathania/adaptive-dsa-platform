package identity

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"net/http"
	"time"
)

const (
	HeaderXCSRFToken = "X-CSRF-Token"
	CookieCSRFName   = "csrf_token"
)

// GenerateSessionCSRFToken computes a cryptographically secure CSRF token
// tied specifically to the authenticated user's session token hash.
func GenerateSessionCSRFToken(sessionHash, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(sessionHash))
	return hex.EncodeToString(mac.Sum(nil))
}

// VerifySessionCSRFToken verifies that a candidate CSRF token matches the session hash.
func VerifySessionCSRFToken(candidateToken, sessionHash, secret string) bool {
	if candidateToken == "" || sessionHash == "" || secret == "" {
		return false
	}
	expected := GenerateSessionCSRFToken(sessionHash, secret)
	return subtle.ConstantTimeCompare([]byte(expected), []byte(candidateToken)) == 1
}

// GeneratePreAuthCSRFToken generates a standalone cryptographically random CSRF token.
func GeneratePreAuthCSRFToken() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

// CreateCSRFCookie creates a cookie for the client containing the CSRF token.
// Note: HttpOnly is false so that single-page applications can read and attach it to request headers.
func CreateCSRFCookie(token string, expiry time.Duration, secure bool, domain string) *http.Cookie {
	return &http.Cookie{
		Name:     CookieCSRFName,
		Value:    token,
		Path:     "/",
		Domain:   domain,
		MaxAge:   int(expiry.Seconds()),
		Expires:  time.Now().Add(expiry),
		HttpOnly: false, // Required for SPA client to read into request header
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
	}
}
