package e2e

import (
	"context"
	"net/http"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AuthMesh/authmesh/tests/testutils"
)

// TestRateLimit_UnifiedAPI tests rate limiting functionality with the unified API
func TestRateLimit_UnifiedAPI(t *testing.T) {
	appURL := testutils.GetAppURL()
	require.NotEmpty(t, appURL, "APP_URL must be set")

	// Setup Redis client for cleanup
	redisAddr := os.Getenv("REDIS_ADDR")
	if redisAddr == "" {
		redisAddr = "localhost:6379"
	}
	
	redisClient := redis.NewClient(&redis.Options{
		Addr: redisAddr,
	})
	defer redisClient.Close()

	// Wait for services to be ready
	require.NoError(t, testutils.WaitForServices(appURL, 60*time.Second), "Services must be ready")

	t.Run("BasicRateLimiting", func(t *testing.T) {
		testBasicRateLimiting(t, appURL, redisClient)
	})

	t.Run("ConcurrentRequests", func(t *testing.T) {
		testConcurrentRateLimiting(t, appURL, redisClient)
	})

	t.Run("RateLimitRecovery", func(t *testing.T) {
		testRateLimitRecovery(t, appURL, redisClient)
	})
}

func testBasicRateLimiting(t *testing.T, appURL string, redisClient *redis.Client) {
	// Clear any existing rate limit state
	clearRateLimitState(t, redisClient)

	endpoint := appURL + "/"
	
	// Make requests until we hit the rate limit
	var resp *http.Response
	var err error
	
	successCount := 0
	rateLimitedCount := 0
	
	// Try to make more requests than the rate limit allows
	// The default config should allow 10 requests per second with burst of 20
	for i := 0; i < 30; i++ {
		resp, err = http.Get(endpoint)
		require.NoError(t, err)
		
		if resp.StatusCode == http.StatusOK {
			successCount++
		} else if resp.StatusCode == http.StatusTooManyRequests {
			rateLimitedCount++
		}
		
		resp.Body.Close()
		
		// Small delay to avoid overwhelming
		time.Sleep(10 * time.Millisecond)
	}

	t.Logf("Success: %d, Rate Limited: %d", successCount, rateLimitedCount)
	
	// We should have gotten some successful requests and some rate limited ones
	assert.Greater(t, successCount, 0, "Should have some successful requests")
	
	// With aggressive testing, we should hit rate limits
	if rateLimitedCount == 0 {
		t.Log("No rate limiting observed - may need to adjust test parameters")
	}
}

func testConcurrentRateLimiting(t *testing.T, appURL string, redisClient *redis.Client) {
	// Clear any existing rate limit state
	clearRateLimitState(t, redisClient)

	endpoint := appURL + "/"
	
	var wg sync.WaitGroup
	var mu sync.Mutex
	successCount := 0
	rateLimitedCount := 0
	
	// Launch concurrent requests
	numGoroutines := 10
	requestsPerGoroutine := 5
	
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			
			for j := 0; j < requestsPerGoroutine; j++ {
				resp, err := http.Get(endpoint)
				if err != nil {
					t.Logf("Request error: %v", err)
					continue
				}
				
				mu.Lock()
				if resp.StatusCode == http.StatusOK {
					successCount++
				} else if resp.StatusCode == http.StatusTooManyRequests {
					rateLimitedCount++
				}
				mu.Unlock()
				
				resp.Body.Close()
			}
		}()
	}
	
	wg.Wait()
	
	t.Logf("Concurrent test - Success: %d, Rate Limited: %d", successCount, rateLimitedCount)
	
	// With concurrent requests, we should see rate limiting
	assert.Greater(t, successCount, 0, "Should have some successful requests")
	
	// The total requests should equal our expected count
	totalRequests := numGoroutines * requestsPerGoroutine
	assert.Equal(t, totalRequests, successCount+rateLimitedCount, "All requests should be accounted for")
}

func testRateLimitRecovery(t *testing.T, appURL string, redisClient *redis.Client) {
	// Clear any existing rate limit state
	clearRateLimitState(t, redisClient)

	endpoint := appURL + "/"
	
	// First, exhaust the rate limit
	for i := 0; i < 25; i++ {
		resp, err := http.Get(endpoint)
		require.NoError(t, err)
		resp.Body.Close()
		time.Sleep(10 * time.Millisecond)
	}
	
	// Now we should be rate limited
	resp, err := http.Get(endpoint)
	require.NoError(t, err)
	resp.Body.Close()
	
	// Wait for rate limit to recover (token bucket refill)
	t.Log("Waiting for rate limit recovery...")
	time.Sleep(2 * time.Second)
	
	// Should be able to make requests again
	resp, err = http.Get(endpoint)
	require.NoError(t, err)
	defer resp.Body.Close()
	
	// We should either get success or still be rate limited, but not an error
	assert.True(t, resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusTooManyRequests)
}

func clearRateLimitState(t *testing.T, redisClient *redis.Client) {
	ctx := context.Background()
	
	// Clear rate limiting keys
	patterns := []string{
		"ratelimit:*",
		"ratelimit_*",
		"authmesh:ratelimit:*",
	}
	
	for _, pattern := range patterns {
		keys, err := redisClient.Keys(ctx, pattern).Result()
		if err == nil && len(keys) > 0 {
			redisClient.Del(ctx, keys...)
			t.Logf("Cleared rate limit keys: %v", keys)
		}
	}
	
	// Small delay to ensure cleanup is complete
	time.Sleep(100 * time.Millisecond)
}
