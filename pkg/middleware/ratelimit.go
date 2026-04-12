package middleware

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"github.com/AuthMesh/authmesh/pkg/ratelimit"
)

// RateLimitMiddleware creates a rate limiting middleware using Redis token bucket
func RateLimitMiddleware(redisClient *redis.Client, logger *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := context.Background()
		
		// Extract tenant from URL path (e.g., /t/tenant1/...)
		path := c.Request.URL.Path
		var tenantID string
		
		if strings.HasPrefix(path, "/t/") {
			parts := strings.Split(path, "/")
			if len(parts) >= 3 {
				tenantID = parts[2]
			}
		}
		
		if tenantID == "" {
			// No tenant-specific rate limiting for non-tenant endpoints
			c.Next()
			return
		}
		
		// Get tenant rate limit from Redis
		redisKey := fmt.Sprintf("ratelimit_tenant:golang-mt:%s", tenantID)
		rateLimit, err := redisClient.Get(ctx, redisKey).Float64()
		if err != nil {
			// Use default rate limit if not configured
			rateLimit = 1000.0
		}
		
		// Use token bucket rate limiting
		bucketKey := fmt.Sprintf("bucket:tenant:%s", tenantID)
		burst := int(rateLimit * 2) // Burst is 2x the rate limit
		expiry := 1 * time.Hour
		
		allowed, err := ratelimit.Allow(ctx, redisClient, bucketKey, rateLimit, burst, expiry)
		if err != nil {
			logger.Error("Rate limiting error", zap.Error(err))
			// Allow request on error to avoid breaking functionality
			c.Next()
			return
		}
		
		if !allowed {
			c.Header("Retry-After", "1")
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error": "Rate limit exceeded",
				"tenant": tenantID,
				"limit": rateLimit,
			})
			c.Abort()
			return
		}
		
		c.Next()
	}
}

// SetupPerUserRateLimit creates a per-user rate limiting middleware
func SetupPerUserRateLimit(redisClient *redis.Client, logger *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := context.Background()
		
		// Extract user from JWT token if available
		userSub, exists := c.Get("user_sub")
		if !exists {
			// No user authentication, skip per-user rate limiting
			c.Next()
			return
		}
		
		userID, ok := userSub.(string)
		if !ok {
			c.Next()
			return
		}
		
		// Per-user rate limiting: 10 req/s with burst of 20
		bucketKey := fmt.Sprintf("bucket:user:%s", userID)
		rateLimit := 10.0
		burst := 20
		expiry := 1 * time.Hour
		
		allowed, err := ratelimit.Allow(ctx, redisClient, bucketKey, rateLimit, burst, expiry)
		if err != nil {
			logger.Error("Per-user rate limiting error", zap.Error(err))
			// Allow request on error to avoid breaking functionality
			c.Next()
			return
		}
		
		if !allowed {
			c.Header("Retry-After", "1")
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error": "Per-user rate limit exceeded",
				"user": userID,
				"limit": rateLimit,
			})
			c.Abort()
			return
		}
		
		c.Next()
	}
}
