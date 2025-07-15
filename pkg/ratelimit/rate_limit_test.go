package ratelimit

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
)

func TestPerUserRateLimit_Success(t *testing.T) {
	// Setup mini Redis for testing
	miniRedis := miniredis.RunT(t)
	defer miniRedis.Close()

	// Create Redis client
	rdb := redis.NewClient(&redis.Options{
		Addr: miniRedis.Addr(),
	})
	defer rdb.Close()

	ctx := context.Background()
	key := "ratelimit:authmesh:tenant1:user123"
	rate := 5.0  // 5 req/s for testing
	burst := 10  // burst of 10
	expiry := 60 * time.Second

	// Test: Send 5 requests (within rate)
	for i := 0; i < 5; i++ {
		allowed, err := Allow(ctx, rdb, key, rate, burst, expiry)
		assert.NoError(t, err)
		assert.True(t, allowed, "Request %d should be allowed", i+1)
	}

	t.Log("✅ Per-user rate limit success test passed")
}

func TestPerUserRateLimit_Burst(t *testing.T) {
	// Setup mini Redis for testing
	miniRedis := miniredis.RunT(t)
	defer miniRedis.Close()

	rdb := redis.NewClient(&redis.Options{
		Addr: miniRedis.Addr(),
	})
	defer rdb.Close()

	ctx := context.Background()
	key := "ratelimit:authmesh:tenant1:user123"
	rate := 5.0  // 5 req/s
	burst := 10  // burst of 10
	expiry := 60 * time.Second

	// Test: Send burst amount (10 requests quickly)
	for i := 0; i < 10; i++ {
		allowed, err := Allow(ctx, rdb, key, rate, burst, expiry)
		assert.NoError(t, err)
		assert.True(t, allowed, "Burst request %d should be allowed", i+1)
	}

	// 11th request should be denied
	allowed, err := Allow(ctx, rdb, key, rate, burst, expiry)
	assert.NoError(t, err)
	assert.False(t, allowed, "11th request should be denied")

	t.Log("✅ Per-user rate limit burst test passed")
}

func TestPerUserRateLimit_OverTime(t *testing.T) {
	// Setup mini Redis for testing
	miniRedis := miniredis.RunT(t)
	defer miniRedis.Close()

	rdb := redis.NewClient(&redis.Options{
		Addr: miniRedis.Addr(),
	})
	defer rdb.Close()

	ctx := context.Background()
	key := "ratelimit:authmesh:tenant1:user123"
	rate := 5.0  // 5 req/s
	burst := 10
	expiry := 60 * time.Second

	// Consume burst
	for i := 0; i < 10; i++ {
		allowed, err := Allow(ctx, rdb, key, rate, burst, expiry)
		assert.NoError(t, err)
		assert.True(t, allowed)
	}

	// Wait for token replenishment
	time.Sleep(1 * time.Second)

	// Should be able to make more requests due to token replenishment
	for i := 0; i < 5; i++ {
		allowed, err := Allow(ctx, rdb, key, rate, burst, expiry)
		assert.NoError(t, err)
		assert.True(t, allowed, "Request after replenishment %d should be allowed", i+1)
	}

	t.Log("✅ Per-user rate limit over time test passed")
}

func TestPerTenantRateLimit_Success(t *testing.T) {
	// Setup mini Redis for testing
	miniRedis := miniredis.RunT(t)
	defer miniRedis.Close()

	rdb := redis.NewClient(&redis.Options{
		Addr: miniRedis.Addr(),
	})
	defer rdb.Close()

	ctx := context.Background()
	key := "ratelimit:authmesh:tenant1"
	rate := 100.0  // 100 req/s for testing
	burst := 150   // burst of 150
	expiry := 60 * time.Second

	// Test: Send 100 requests (within rate)
	for i := 0; i < 100; i++ {
		allowed, err := Allow(ctx, rdb, key, rate, burst, expiry)
		assert.NoError(t, err)
		assert.True(t, allowed, "Request %d should be allowed", i+1)
	}

	t.Log("✅ Per-tenant rate limit success test passed")
}

func TestPerTenantRateLimit_Burst(t *testing.T) {
	// Setup mini Redis for testing
	miniRedis := miniredis.RunT(t)
	defer miniRedis.Close()

	rdb := redis.NewClient(&redis.Options{
		Addr: miniRedis.Addr(),
	})
	defer rdb.Close()

	ctx := context.Background()
	key := "ratelimit:authmesh:tenant1"
	rate := 0.0    // 0 req/s refill rate to test pure burst capacity
	burst := 10    // burst of 10 tokens
	expiry := 60 * time.Second

	// Test: Send burst amount (10 requests quickly)
	for i := 0; i < 10; i++ {
		allowed, err := Allow(ctx, rdb, key, rate, burst, expiry)
		assert.NoError(t, err)
		assert.True(t, allowed, "Burst request %d should be allowed", i+1)
	}

	// 11th request should be denied (no refill rate)
	allowed, err := Allow(ctx, rdb, key, rate, burst, expiry)
	assert.NoError(t, err)
	assert.False(t, allowed, "11th request should be denied")

	t.Log("✅ Per-tenant rate limit burst test passed")
}

func TestPerTenantRateLimit_OverTime(t *testing.T) {
	// Setup mini Redis for testing
	miniRedis := miniredis.RunT(t)
	defer miniRedis.Close()

	rdb := redis.NewClient(&redis.Options{
		Addr: miniRedis.Addr(),
	})
	defer rdb.Close()

	ctx := context.Background()
	key := "ratelimit:authmesh:tenant1"
	rate := 100.0  // 100 req/s
	burst := 150
	expiry := 60 * time.Second

	// Consume burst
	for i := 0; i < 150; i++ {
		allowed, err := Allow(ctx, rdb, key, rate, burst, expiry)
		assert.NoError(t, err)
		assert.True(t, allowed)
	}

	// Wait for token replenishment
	time.Sleep(1 * time.Second)

	// Should be able to make more requests due to token replenishment
	for i := 0; i < 100; i++ {
		allowed, err := Allow(ctx, rdb, key, rate, burst, expiry)
		assert.NoError(t, err)
		assert.True(t, allowed, "Request after replenishment %d should be allowed", i+1)
	}

	t.Log("✅ Per-tenant rate limit over time test passed")
}

func TestRedisPipelining(t *testing.T) {
	// Test that Redis operations are efficient for per-user and per-tenant checks
	miniRedis := miniredis.RunT(t)
	defer miniRedis.Close()

	rdb := redis.NewClient(&redis.Options{
		Addr: miniRedis.Addr(),
	})
	defer rdb.Close()

	ctx := context.Background()

	userKey := "ratelimit:authmesh:tenant1:user123"
	tenantKey := "ratelimit:authmesh:tenant1"
	rate := 5.0
	burst := 10
	expiry := 60 * time.Second

	// Test multiple checks happen quickly
	start := time.Now()
	
	userAllowed, err := Allow(ctx, rdb, userKey, rate, burst, expiry)
	assert.NoError(t, err)
	
	tenantAllowed, err := Allow(ctx, rdb, tenantKey, 100.0, 150, expiry)
	assert.NoError(t, err)
	
	elapsed := time.Since(start)
	
	assert.True(t, userAllowed, "User request should be allowed")
	assert.True(t, tenantAllowed, "Tenant request should be allowed")
	assert.Less(t, elapsed, 10*time.Millisecond, "Redis operations should be fast")
	
	t.Log("✅ Redis efficiency test passed")
}

func TestTokenBucketAlgorithm_EdgeCases(t *testing.T) {
	miniRedis := miniredis.RunT(t)
	defer miniRedis.Close()

	rdb := redis.NewClient(&redis.Options{
		Addr: miniRedis.Addr(),
	})
	defer rdb.Close()

	ctx := context.Background()

	t.Run("Zero_Rate_Limit", func(t *testing.T) {
		key := "ratelimit:authmesh:test:zero"
		rate := 0.0  // No refill
		burst := 1   // Only 1 token initially
		expiry := 60 * time.Second

		// First request should be allowed
		allowed, err := Allow(ctx, rdb, key, rate, burst, expiry)
		assert.NoError(t, err)
		assert.True(t, allowed, "First request should be allowed")

		// Second request should be denied (no refill)
		allowed, err = Allow(ctx, rdb, key, rate, burst, expiry)
		assert.NoError(t, err)
		assert.False(t, allowed, "Second request should be denied")
	})

	t.Run("Very_High_Rate_Limit", func(t *testing.T) {
		key := "ratelimit:authmesh:test:high"
		rate := 1000000.0  // Very high rate
		burst := 1000000   // Very high burst
		expiry := 60 * time.Second

		// Should handle high rates without issues
		for i := 0; i < 1000; i++ {
			allowed, err := Allow(ctx, rdb, key, rate, burst, expiry)
			assert.NoError(t, err)
			assert.True(t, allowed, "High rate request %d should be allowed", i+1)
		}
	})

	t.Run("Burst_Equals_One", func(t *testing.T) {
		key := "ratelimit:authmesh:test:one"
		rate := 1.0  // 1 req/s
		burst := 1   // Only 1 token max
		expiry := 60 * time.Second

		// First request should be allowed
		allowed, err := Allow(ctx, rdb, key, rate, burst, expiry)
		assert.NoError(t, err)
		assert.True(t, allowed, "Request with burst=1 should be allowed")

		// Second request should be denied immediately
		allowed, err = Allow(ctx, rdb, key, rate, burst, expiry)
		assert.NoError(t, err)
		assert.False(t, allowed, "Second request with burst=1 should be denied")
	})
}
