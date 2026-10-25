package identity

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHashAndVerifyPassword(t *testing.T) {
	password := "CorrectHorseBatteryStaple123!"

	hash, err := HashPassword(password)
	require.NoError(t, err)
	assert.NotEmpty(t, hash)
	assert.True(t, strings.HasPrefix(hash, "$argon2id$v=19$m=65536,t=3,p=2$"))

	// Positive verification
	match, err := VerifyPassword(password, hash)
	require.NoError(t, err)
	assert.True(t, match)

	// Negative verification - wrong password
	matchWrong, err := VerifyPassword("WrongPassword123!", hash)
	require.NoError(t, err)
	assert.False(t, matchWrong)
}

func TestValidatePassword_LengthConstraints(t *testing.T) {
	// Too short (< 8)
	assert.Error(t, ValidatePassword("short7!"))

	// Minimum valid length (8)
	assert.NoError(t, ValidatePassword("valid08!"))

	// Maximum valid length (72)
	assert.NoError(t, ValidatePassword(strings.Repeat("A", 72)))

	// Too long (> 72)
	assert.Error(t, ValidatePassword(strings.Repeat("A", 73)))
}

func TestVerifyPassword_MalformedHash(t *testing.T) {
	// Not enough parts
	_, err := VerifyPassword("password123", "$invalid$hash")
	assert.Error(t, err)

	// Wrong algorithm
	_, err = VerifyPassword("password123", "$bcrypt$v=19$m=65536,t=3,p=2$salt$hash")
	assert.Error(t, err)

	// Corrupted base64
	_, err = VerifyPassword("password123", "$argon2id$v=19$m=65536,t=3,p=2$invalid_b64$invalid_b64")
	assert.Error(t, err)
}
