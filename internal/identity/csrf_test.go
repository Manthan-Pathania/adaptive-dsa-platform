package identity

import (
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSessionCSRFToken_Lifecycle(t *testing.T) {
	secret := "test-csrf-secret-key-at-least-32-bytes!"
	sessionHash := "a1b2c3d4e5f67890123456789abcdef0123456789abcdef0123456789abcdef0"

	csrfToken := GenerateSessionCSRFToken(sessionHash, secret)
	assert.NotEmpty(t, csrfToken)

	// Valid token verification
	assert.True(t, VerifySessionCSRFToken(csrfToken, sessionHash, secret))

	// Invalid token
	assert.False(t, VerifySessionCSRFToken("tampered-token", sessionHash, secret))

	// Mismatched session hash (different user session)
	diffHash := "0000000000000000000000000000000000000000000000000000000000000000"
	assert.False(t, VerifySessionCSRFToken(csrfToken, diffHash, secret))

	// Empty inputs
	assert.False(t, VerifySessionCSRFToken("", sessionHash, secret))
	assert.False(t, VerifySessionCSRFToken(csrfToken, "", secret))
	assert.False(t, VerifySessionCSRFToken(csrfToken, sessionHash, ""))
}

func TestGeneratePreAuthCSRFToken(t *testing.T) {
	token1, err := GeneratePreAuthCSRFToken()
	require.NoError(t, err)
	token2, err := GeneratePreAuthCSRFToken()
	require.NoError(t, err)

	assert.NotEqual(t, token1, token2)
	assert.Len(t, token1, 64)
}

func TestCreateCSRFCookie(t *testing.T) {
	cookie := CreateCSRFCookie("test-csrf-token", 1*time.Hour, false, "")
	assert.Equal(t, CookieCSRFName, cookie.Name)
	assert.Equal(t, "test-csrf-token", cookie.Value)
	assert.False(t, cookie.HttpOnly, "CSRF cookie must be readable by frontend JavaScript")
	assert.Equal(t, http.SameSiteLaxMode, cookie.SameSite)
}
