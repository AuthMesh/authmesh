package middleware

import (
	"context"
	"fmt"
	"time"

	"github.com/AuthMesh/authmesh/pkg/config"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// SetupCORS configures CORS middleware using configuration
func SetupCORS() gin.HandlerFunc {
	cfg := config.Load()

	corsConfig := cors.Config{
		AllowOrigins:     cfg.CORSAllowOrigins,
		AllowMethods:     cfg.CORSAllowMethods,
		AllowHeaders:     cfg.CORSAllowHeaders,
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
	}

	// Parse max age from string to duration
	if cfg.CORSMaxAge != "" {
		if duration, err := time.ParseDuration(cfg.CORSMaxAge + "s"); err == nil {
			corsConfig.MaxAge = duration
		}
	}

	return cors.New(corsConfig)
}

// SetupGroupRateLimit configures group-based rate limiting middleware for specific routes
// This middleware checks group rate limits based on the user's group membership
func SetupGroupRateLimit(redisClient *redis.Client, logger *zap.Logger) gin.HandlerFunc {
	return gin.HandlerFunc(func(c *gin.Context) {
		// Extract tenant ID and user groups from context
		tenantID := c.Param("tenant_id")
		if tenantID == "" {
			// For non-tenant routes, check if tenant_id exists in context from JWT
			if contextTenantID, exists := c.Get("tenant_id"); exists {
				if tid, ok := contextTenantID.(string); ok {
					tenantID = tid
				}
			}
		}

		if tenantID == "" {
			logger.Debug("No tenant ID found for group rate limiting, skipping")
			c.Next()
			return
		}

		// Get user groups from JWT claims
		userGroups := extractUserGroupsFromContext(c)
		if len(userGroups) == 0 {
			logger.Debug("No user groups found for group rate limiting, skipping")
			c.Next()
			return
		}

		// Check Redis health first
		ctx := context.Background()
		if err := redisClient.Ping(ctx).Err(); err != nil {
			logger.Error("Redis health check failed for group rate limiting", zap.Error(err))
			// Return 503 on complete failure as specified in milestone
			c.JSON(503, gin.H{"error": "Service temporarily unavailable"})
			c.Abort()
			return
		}

		// Check rate limits for each group the user belongs to
		now := time.Now()
		hourWindow := now.Format("2006010215") // YYYYMMDDHH format

		for _, groupID := range userGroups {
			// Get group rate limit configuration from Redis
			configKey := fmt.Sprintf("ratelimit_config:golang-mt:%s:%s", tenantID, groupID)
			limitStr, err := redisClient.Get(ctx, configKey).Result()
			if err != nil && err != redis.Nil {
				logger.Error("Failed to get group rate limit config from Redis",
					zap.String("tenant_id", tenantID),
					zap.String("group_id", groupID),
					zap.Error(err))
				continue
			}

			var groupLimit int
			if err == redis.Nil {
				// No config found, use default
				groupLimit = getDefaultGroupRateLimit()
			} else {
				// Parse the limit from Redis
				if parsedLimit, parseErr := fmt.Sscanf(limitStr, "%d", &groupLimit); parseErr != nil || parsedLimit != 1 {
					logger.Warn("Invalid group rate limit value in Redis",
						zap.String("tenant_id", tenantID),
						zap.String("group_id", groupID),
						zap.String("value", limitStr))
					groupLimit = getDefaultGroupRateLimit()
				}
			}

			// Create Redis key for group rate limiting
			redisKey := fmt.Sprintf("ratelimit:golang-mt:%s:%s:%s", tenantID, groupID, hourWindow)

			// Get current request count for this group
			currentCount, err := redisClient.Get(ctx, redisKey).Int()
			if err != nil && err != redis.Nil {
				logger.Error("Failed to get group rate limit count from Redis",
					zap.String("key", redisKey),
					zap.Error(err))
				continue
			}

			// Check if group limit exceeded
			if currentCount >= groupLimit {
				// Calculate retry-after based on group rate limit (convert hourly to seconds)
				retryAfterSeconds := 3600 // 1 hour default
				if groupLimit > 0 {
					retryAfterSeconds = 3600 / groupLimit * 60 // Rough estimation
					if retryAfterSeconds < 60 {
						retryAfterSeconds = 60 // Minimum 1 minute
					} else if retryAfterSeconds > 3600 {
						retryAfterSeconds = 3600 // Maximum 1 hour
					}
				}

				c.Header("Retry-After", fmt.Sprintf("%d", retryAfterSeconds))
				c.JSON(429, gin.H{
					"error":     "Group rate limit exceeded",
					"tenant_id": tenantID,
					"group_id":  groupID,
					"limit":     groupLimit,
				})
				c.Abort()
				return
			}

			// Increment counter for this group
			pipe := redisClient.Pipeline()
			pipe.Incr(ctx, redisKey)
			pipe.Expire(ctx, redisKey, 2*time.Hour) // 2-hour expiry as specified

			if _, err := pipe.Exec(ctx); err != nil {
				logger.Error("Failed to update group rate limit count in Redis",
					zap.String("key", redisKey),
					zap.Error(err))
			}
		}

		c.Next()
	})
}

// SecurityHeadersMiddleware adds security-related HTTP headers
func SecurityHeadersMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("X-Frame-Options", "DENY")
		c.Header("X-XSS-Protection", "1; mode=block")
		c.Header("Referrer-Policy", "strict-origin-when-cross-origin")
		c.Header("Content-Security-Policy", "default-src 'self'")
		c.Next()
	}
}

// CalculateRetryAfterSeconds calculates the proper retry time based on rate limit configuration
// Returns the time window in seconds for the rate limit to reset
func CalculateRetryAfterSeconds(rateLimitPerSecond float64) int {
	// Validate rate limit to avoid division by zero
	if rateLimitPerSecond <= 0 {
		// Default to 60 seconds if rate limit is invalid
		return 60
	}

	// Calculate the rate limit window: 1 second / requests_per_second
	// For example: 10 requests/second = 1/10 = 0.1 seconds minimum between requests
	// But for Retry-After, we want to suggest a reasonable retry window
	// Use 1 minute as the typical rate limit window for better UX
	retrySeconds := int(60.0 / rateLimitPerSecond)

	// Ensure minimum retry time of 1 second and maximum of 300 seconds (5 minutes)
	if retrySeconds < 1 {
		retrySeconds = 1
	} else if retrySeconds > 300 {
		retrySeconds = 300
	}

	return retrySeconds
}

// extractUserIDFromContext extracts user ID from JWT claims in the context
func extractUserIDFromContext(c *gin.Context) string {
	// Try to get user ID from JWT claims stored in context
	if claims, exists := c.Get("claims"); exists {
		if claimsMap, ok := claims.(map[string]interface{}); ok {
			if userID, exists := claimsMap["sub"]; exists {
				if userIDStr, ok := userID.(string); ok {
					return userIDStr
				}
			}
		}
	}

	// Fallback: try to get from user_id header or parameter
	if userID := c.GetHeader("X-User-ID"); userID != "" {
		return userID
	}

	return ""
}

// extractUserGroupsFromContext extracts user groups from JWT claims in the context
func extractUserGroupsFromContext(c *gin.Context) []string {
	// Try to get groups from JWT claims stored in context
	if claims, exists := c.Get("claims"); exists {
		if claimsMap, ok := claims.(map[string]interface{}); ok {
			if groups, exists := claimsMap["groups"]; exists {
				// Handle both []string and []interface{} cases
				switch groupsVal := groups.(type) {
				case []string:
					return groupsVal
				case []interface{}:
					var stringGroups []string
					for _, group := range groupsVal {
						if groupStr, ok := group.(string); ok {
							stringGroups = append(stringGroups, groupStr)
						}
					}
					return stringGroups
				}
			}
		}
	}

	return []string{}
}

// getDefaultGroupRateLimit returns the default group rate limit from environment or fallback
func getDefaultGroupRateLimit() int {
	cfg := config.Load()
	// Use the same default as individual rate limits for groups
	return int(cfg.RateLimitPerSecond * 3600) // Convert per-second to per-hour
}
