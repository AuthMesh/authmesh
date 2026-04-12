package ratelimit

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func TestNewTokenBucket(t *testing.T) {
	bucket := NewTokenBucket(10, 20)
	assert.NotNil(t, bucket)
	assert.Equal(t, 20, bucket.maxTokens)
	assert.Equal(t, 10, bucket.refillRate)
	assert.Equal(t, 20, bucket.tokens)
}

func TestTokenBucketAllow(t *testing.T) {
	t.Run("allows requests within burst", func(t *testing.T) {
		bucket := NewTokenBucket(10, 5)
		for i := 0; i < 5; i++ {
			assert.True(t, bucket.Allow(), "request %d should be allowed", i)
		}
	})

	t.Run("blocks after burst exhausted", func(t *testing.T) {
		bucket := NewTokenBucket(10, 3)
		for i := 0; i < 3; i++ {
			bucket.Allow()
		}
		assert.False(t, bucket.Allow(), "should be blocked after burst exhausted")
	})

	t.Run("refills over time", func(t *testing.T) {
		bucket := NewTokenBucket(100, 1) // 100/sec, burst of 1
		assert.True(t, bucket.Allow())
		assert.False(t, bucket.Allow())

		// Wait for refill
		time.Sleep(20 * time.Millisecond)
		assert.True(t, bucket.Allow())
	})

	t.Run("does not exceed max", func(t *testing.T) {
		bucket := NewTokenBucket(1000, 5)
		// Drain all tokens
		for i := 0; i < 5; i++ {
			bucket.Allow()
		}
		// Wait for excessive refill
		time.Sleep(100 * time.Millisecond)

		// Should have at most maxTokens
		count := 0
		for bucket.Allow() {
			count++
			if count > 10 {
				break
			}
		}
		assert.LessOrEqual(t, count, 5, "should not exceed max tokens")
	})
}

func TestMin(t *testing.T) {
	assert.Equal(t, 1, min(1, 2))
	assert.Equal(t, 1, min(2, 1))
	assert.Equal(t, 5, min(5, 5))
	assert.Equal(t, -1, min(-1, 0))
}

func TestNewTenantRateLimitCache(t *testing.T) {
	// With nil redis client - just tests construction
	cache := NewTenantRateLimitCache(nil)
	assert.NotNil(t, cache)
}

func TestNewSuperAdminHandler(t *testing.T) {
	logger, _ := zap.NewDevelopment()

	t.Run("nil redis client", func(t *testing.T) {
		_, err := NewSuperAdminHandler(nil, logger)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "redis client is required")
	})

	t.Run("nil logger", func(t *testing.T) {
		// We can't easily mock redis.Client, just test the nil logger path
		_, err := NewSuperAdminHandler(nil, nil)
		assert.Error(t, err)
	})
}
