package identity

import (
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateSessionToken(t *testing.T) {
	token1, err := GenerateSessionToken()
	require.NoError(t, err)
	assert.NotEmpty(t, token1)

	token2, err := GenerateSessionToken()
	require.NoError(t, err)
	assert.NotEmpty(t, token2)

	assert.NotEqual(t, token1, token2, "session tokens must be cryptographically unique")
	assert.GreaterOrEqual(t, len(token1), 40)
}

func TestHashSessionToken(t *testing.T) {
	raw := "test-session-token-raw-value"
	hash1 := HashSessionToken(raw)
	hash2 := HashSessionToken(raw)

	assert.Equal(t, hash1, hash2, "hash function must be deterministic")
	assert.Len(t, hash1, 64, "SHA-256 hex digest must be 64 characters")

	diffHash := HashSessionToken(raw + "-different")
	assert.NotEqual(t, hash1, diffHash)
}

func TestCreateSessionCookie(t *testing.T) {
	rawToken := "opaque-session-token-xyz"
	expiry := 14 * 24 * time.Hour
	cookie := CreateSessionCookie("custom_session", rawToken, expiry, true, "example.com")

	assert.Equal(t, "custom_session", cookie.Name)
	assert.Equal(t, rawToken, cookie.Value)
	assert.Equal(t, "/", cookie.Path)
	assert.Equal(t, "example.com", cookie.Domain)
	assert.True(t, cookie.HttpOnly, "session cookies must be HttpOnly")
	assert.True(t, cookie.Secure, "secure flag must match configuration")
	assert.Equal(t, http.SameSiteLaxMode, cookie.SameSite)
	assert.Equal(t, int(expiry.Seconds()), cookie.MaxAge)
}

func TestExpireSessionCookie(t *testing.T) {
	cookie := ExpireSessionCookie("custom_session", "example.com")

	assert.Equal(t, "custom_session", cookie.Name)
	assert.Empty(t, cookie.Value)
	assert.Equal(t, -1, cookie.MaxAge)
	assert.True(t, cookie.Expires.Before(time.Now()))
}
