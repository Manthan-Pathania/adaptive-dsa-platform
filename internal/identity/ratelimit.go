package identity

import (
	"sync"
	"time"
)

// clientBucket tracks request timestamps within a sliding window.
type clientBucket struct {
	timestamps []time.Time
}

// MemoryRateLimiter implements a thread-safe in-memory sliding window rate limiter.
type MemoryRateLimiter struct {
	mu          sync.Mutex
	buckets     map[string]*clientBucket
	limit       int
	window      time.Duration
	stopCleanup chan struct{}
}

// NewRateLimiter initializes an in-memory rate limiter with automated background eviction.
func NewRateLimiter(limitPerMinute int, window time.Duration) *MemoryRateLimiter {
	limiter := &MemoryRateLimiter{
		buckets:     make(map[string]*clientBucket),
		limit:       limitPerMinute,
		window:      window,
		stopCleanup: make(chan struct{}),
	}

	go limiter.cleanupLoop(window * 2)
	return limiter
}

// Allow determines if a request for the given key (e.g., client IP) is permitted.
func (r *MemoryRateLimiter) Allow(key string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-r.window)

	bucket, exists := r.buckets[key]
	if !exists {
		bucket = &clientBucket{timestamps: make([]time.Time, 0, r.limit)}
		r.buckets[key] = bucket
	}

	// Filter out expired timestamps
	validIndex := 0
	for i, t := range bucket.timestamps {
		if t.After(cutoff) {
			validIndex = i
			break
		}
		if i == len(bucket.timestamps)-1 {
			validIndex = len(bucket.timestamps)
		}
	}
	bucket.timestamps = bucket.timestamps[validIndex:]

	if len(bucket.timestamps) >= r.limit {
		return false
	}

	bucket.timestamps = append(bucket.timestamps, now)
	return true
}

// Reset clears recorded attempts for a specific key (e.g. on successful login).
func (r *MemoryRateLimiter) Reset(key string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.buckets, key)
}

// Stop shuts down the background eviction goroutine.
func (r *MemoryRateLimiter) Stop() {
	close(r.stopCleanup)
}

func (r *MemoryRateLimiter) cleanupLoop(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-r.stopCleanup:
			return
		case now := <-ticker.C:
			r.mu.Lock()
			cutoff := now.Add(-r.window)
			for key, bucket := range r.buckets {
				if len(bucket.timestamps) == 0 || bucket.timestamps[len(bucket.timestamps)-1].Before(cutoff) {
					delete(r.buckets, key)
				}
			}
			r.mu.Unlock()
		}
	}
}
