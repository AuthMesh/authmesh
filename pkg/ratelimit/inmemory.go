package ratelimit

import (
	"sync"
	"time"
)

type TokenBucket struct {
	tokens      int
	maxTokens   int
	refillRate  int
	lastRefill  time.Time
	mu          sync.Mutex
}

func NewTokenBucket(rate, burst int) *TokenBucket {
	return &TokenBucket{
		tokens:     burst,
		maxTokens:  burst,
		refillRate: rate,
		lastRefill: time.Now(),
	}
}

func (b *TokenBucket) Allow() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	now := time.Now()
	elapsed := now.Sub(b.lastRefill).Seconds()
	refill := int(elapsed * float64(b.refillRate))
	if refill > 0 {
		b.tokens = min(b.tokens+refill, b.maxTokens)
		b.lastRefill = now
	}
	if b.tokens > 0 {
		b.tokens--
		return true
	}
	return false
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
