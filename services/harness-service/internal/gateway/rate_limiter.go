package gateway

import (
	"sync"
	"time"
)

// RateLimiter implements a token bucket rate limiter for gateway providers
type RateLimiter struct {
	buckets map[string]*tokenBucket
	mu      sync.Mutex
}

type tokenBucket struct {
	tokens     float64
	maxTokens  float64
	refillRate float64 // tokens per second
	lastRefill time.Time
}

// NewRateLimiter creates a new rate limiter
func NewRateLimiter() *RateLimiter {
	return &RateLimiter{
		buckets: make(map[string]*tokenBucket),
	}
}

// AllowRequest checks if a request is allowed under the rate limit
func (rl *RateLimiter) AllowRequest(provider string, rateLimit int) bool {
	// If rateLimit <= 0, allow all requests
	if rateLimit <= 0 {
		return true
	}

	rl.mu.Lock()
	defer rl.mu.Unlock()

	bucket, ok := rl.buckets[provider]
	if !ok {
		// No bucket configured, allow the request
		return true
	}

	// Refill tokens based on elapsed time
	now := time.Now()
	elapsed := now.Sub(bucket.lastRefill).Seconds()
	bucket.tokens += elapsed * bucket.refillRate
	if bucket.tokens > bucket.maxTokens {
		bucket.tokens = bucket.maxTokens
	}
	bucket.lastRefill = now

	// Check if we have a token available
	if bucket.tokens >= 1 {
		bucket.tokens--
		return true
	}

	return false
}

// Configure creates or updates a token bucket for a provider
func (rl *RateLimiter) Configure(provider string, rateLimit int) {
	if rateLimit <= 0 {
		// Remove bucket if rate limiting is disabled
		rl.mu.Lock()
		defer rl.mu.Unlock()
		delete(rl.buckets, provider)
		return
	}

	rl.mu.Lock()
	defer rl.mu.Unlock()

	maxTokens := float64(rateLimit)
	refillRate := float64(rateLimit) / 60.0 // rateLimit is per minute, convert to per second

	if bucket, ok := rl.buckets[provider]; ok {
		// Update existing bucket
		bucket.maxTokens = maxTokens
		bucket.refillRate = refillRate
		if bucket.tokens > maxTokens {
			bucket.tokens = maxTokens
		}
	} else {
		// Create new bucket
		rl.buckets[provider] = &tokenBucket{
			tokens:     maxTokens, // Start with full bucket
			maxTokens:  maxTokens,
			refillRate: refillRate,
			lastRefill: time.Now(),
		}
	}
}

// Remove removes a provider from the rate limiter
func (rl *RateLimiter) Remove(provider string) {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	delete(rl.buckets, provider)
}
