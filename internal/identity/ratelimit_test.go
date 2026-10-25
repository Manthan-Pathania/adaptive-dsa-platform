package identity

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestMemoryRateLimiter_SlidingWindow(t *testing.T) {
	limit := 3
	window := 200 * time.Millisecond
	limiter := NewRateLimiter(limit, window)
	defer limiter.Stop()

	clientKey := "client-ip-127.0.0.1"

	// First 3 requests permitted
	assert.True(t, limiter.Allow(clientKey))
	assert.True(t, limiter.Allow(clientKey))
	assert.True(t, limiter.Allow(clientKey))

	// 4th request exceeds limit
	assert.False(t, limiter.Allow(clientKey))
	assert.False(t, limiter.Allow(clientKey))

	// Different client key permitted
	assert.True(t, limiter.Allow("different-ip-10.0.0.1"))

	// Manual reset permits immediately
	limiter.Reset(clientKey)
	assert.True(t, limiter.Allow(clientKey))
}
