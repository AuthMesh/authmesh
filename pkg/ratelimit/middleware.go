package ratelimit

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

// Config represents rate limiting configuration
type Config struct {
	RedisClient             *redis.Client
	RequestsPerSecond       float64
	BurstSize               int
	TenantEnabled           bool
	TenantRequestsPerSecond float64
	TenantBurstSize         int
	Algorithm               AlgorithmType
	WindowSize              time.Duration
}

// DefaultConfig returns a default rate limiting configuration
func DefaultConfig() Config {
	return Config{
		RequestsPerSecond:       10.0,
		BurstSize:               20,
		TenantEnabled:           true,
		TenantRequestsPerSecond: 50.0,
		TenantBurstSize:         100,
		Algorithm:               TokenBucketAlgorithm,
		WindowSize:              time.Minute,
	}
}

// Middleware creates a rate limiting middleware
func Middleware(config Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := context.Background()
		
		// Extract tenant and user information
		tenantID := extractTenantID(c)
		userID := extractUserID(c)
		
		// Apply rate limiting based on tenant awareness
		if config.TenantEnabled && tenantID != "" {
			// Tenant-specific rate limiting
			if !checkTenantRateLimit(ctx, config, tenantID, userID, c) {
				return // Request was rate limited
			}
		} else {
			// Global rate limiting
			if !checkGlobalRateLimit(ctx, config, c) {
				return // Request was rate limited
			}
		}
		
		c.Next()
	}
}

// checkTenantRateLimit applies tenant-specific rate limiting
func checkTenantRateLimit(ctx context.Context, config Config, tenantID, userID string, c *gin.Context) bool {
	// Per-user rate limiting within tenant
	if userID != "" {
		userKey := fmt.Sprintf("ratelimit:authmesh:tenant:%s:user:%s", tenantID, userID)
		if !isRequestAllowed(ctx, config, userKey, config.RequestsPerSecond, config.BurstSize, c) {
			return false
		}
	}
	
	// Per-tenant rate limiting
	tenantKey := fmt.Sprintf("ratelimit:authmesh:tenant:%s", tenantID)
	return isRequestAllowed(ctx, config, tenantKey, config.TenantRequestsPerSecond, config.TenantBurstSize, c)
}

// checkGlobalRateLimit applies global rate limiting
func checkGlobalRateLimit(ctx context.Context, config Config, c *gin.Context) bool {
	clientID := getClientID(c)
	globalKey := fmt.Sprintf("ratelimit:authmesh:global:%s", clientID)
	return isRequestAllowed(ctx, config, globalKey, config.RequestsPerSecond, config.BurstSize, c)
}

// isRequestAllowed checks if a request is allowed using the configured algorithm
func isRequestAllowed(ctx context.Context, config Config, key string, rate float64, burst int, c *gin.Context) bool {
	if config.RedisClient != nil {
		// Use Redis-based rate limiting
		limiter := NewRedisLimiter(config.RedisClient)
		allowed, err := limiter.Allow(ctx, key, rate, burst, config.WindowSize)
		if err != nil {
			// Log error and allow request to proceed (fail open)
			// In production, you might want to fail closed depending on requirements
			return true
		}
		
		if !allowed {
			rateLimitResponse(c, rate)
			return false
		}
		return true
	} else {
		// Fall back to in-memory rate limiting
		bucket := getOrCreateBucket(key, rate, burst)
		if !bucket.Allow() {
			rateLimitResponse(c, rate)
			return false
		}
		return true
	}
}

// rateLimitResponse sends a rate limit exceeded response
func rateLimitResponse(c *gin.Context, rate float64) {
	retryAfter := calculateRetryAfter(rate)
	c.Header("Retry-After", strconv.Itoa(retryAfter))
	c.Header("X-RateLimit-Limit", fmt.Sprintf("%.0f", rate))
	c.Header("X-RateLimit-Remaining", "0")
	c.Header("X-RateLimit-Reset", strconv.FormatInt(time.Now().Add(time.Duration(retryAfter)*time.Second).Unix(), 10))
	
	c.JSON(http.StatusTooManyRequests, gin.H{
		"error":   "Rate limit exceeded",
		"message": "Too many requests. Please try again later.",
		"retry_after": retryAfter,
	})
	c.Abort()
}

// calculateRetryAfter calculates retry time based on rate limit
func calculateRetryAfter(rate float64) int {
	if rate <= 0 {
		return 60
	}
	
	retrySeconds := int(60.0 / rate)
	if retrySeconds < 1 {
		retrySeconds = 1
	} else if retrySeconds > 300 {
		retrySeconds = 300
	}
	
	return retrySeconds
}

// extractTenantID extracts tenant ID from various sources
func extractTenantID(c *gin.Context) string {
	// Try parameter first
	if tenantID := c.Param("tenant_id"); tenantID != "" {
		return tenantID
	}
	
	// Try header
	if tenantID := c.GetHeader("X-Tenant-ID"); tenantID != "" {
		return tenantID
	}
	
	// Try to extract from path
	path := c.Request.URL.Path
	if strings.HasPrefix(path, "/t/") {
		parts := strings.Split(strings.TrimPrefix(path, "/t/"), "/")
		if len(parts) > 0 && parts[0] != "" {
			return parts[0]
		}
	}
	
	// Try context (set by auth middleware)
	if tenantID, exists := c.Get("tenant_id"); exists {
		if tid, ok := tenantID.(string); ok {
			return tid
		}
	}
	
	return ""
}

// extractUserID extracts user ID from context or headers
func extractUserID(c *gin.Context) string {
	// Try context first (set by auth middleware)
	if userID, exists := c.Get("user_id"); exists {
		if uid, ok := userID.(string); ok {
			return uid
		}
	}
	
	// Try JWT claims
	if claims, exists := c.Get("claims"); exists {
		if claimsMap, ok := claims.(map[string]interface{}); ok {
			if sub, exists := claimsMap["sub"]; exists {
				if subStr, ok := sub.(string); ok {
					return subStr
				}
			}
		}
	}
	
	// Try header
	if userID := c.GetHeader("X-User-ID"); userID != "" {
		return userID
	}
	
	return ""
}

// Simple in-memory store for buckets (fallback when Redis is not available)
var bucketStore = make(map[string]*TokenBucket)
var bucketMutex sync.RWMutex

func getClientID(c *gin.Context) string {
	// Try to get user ID first, then fall back to IP
	if userID := extractUserID(c); userID != "" {
		return userID
	}
	return c.ClientIP()
}

func getOrCreateBucket(key string, rate float64, burst int) *TokenBucket {
	bucketMutex.RLock()
	bucket, exists := bucketStore[key]
	bucketMutex.RUnlock()
	
	if exists {
		return bucket
	}
	
	// Create new bucket
	bucketMutex.Lock()
	defer bucketMutex.Unlock()
	
	// Double-check after acquiring write lock
	if bucket, exists := bucketStore[key]; exists {
		return bucket
	}
	
	// Create new token bucket
	bucket = NewTokenBucket(int(rate), burst)
	bucketStore[key] = bucket
	
	return bucket
}
