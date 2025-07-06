package auth

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v4"
	"github.com/redis/go-redis/v9"
)

// TenantContextKey is used for storing tenant information in context
type TenantContextKey string

const (
	SchoolIDContextKey TenantContextKey = "school_id"
	RealmContextKey    TenantContextKey = "realm"
)

// TenantMiddleware extracts tenant information from JWT claims and enforces multi-tenant isolation
func TenantMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get full claims from context (set by JWT middleware)
		claimsInterface, exists := c.Get("claims")
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
			c.Abort()
			return
		}

		userClaims, ok := claimsInterface.(jwt.MapClaims)
		if !ok {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid claims format"})
			c.Abort()
			return
		}

		// Extract tenant_id from JWT claims
		tenantID, err := extractTenantIDFromClaims(userClaims)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid or missing tenant_id in token"})
			c.Abort()
			return
		}

		// Check if this is a tenant-specific route (/t/{tenant_id}/...)
		if pathTenantID := c.Param("tenant_id"); pathTenantID != "" {
			// Validate that the JWT tenant_id matches the route tenant_id
			if tenantID != pathTenantID {
				c.JSON(http.StatusForbidden, gin.H{
					"error": "Access denied: token tenant_id does not match requested tenant",
					"details": gin.H{
						"token_tenant_id":     tenantID,
						"requested_tenant_id": pathTenantID,
					},
				})
				c.Abort()
				return
			}
		}

		// Extract realm from iss claim (issuer) for backward compatibility
		realm := extractRealmFromIssuer(userClaims)
		if realm == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid realm in token"})
			c.Abort()
			return
		}

		// Extract user roles for admin check
		roles := extractRoles(userClaims)
		isAdmin := isAdminUser(roles)

		// Extract SchoolID from tenant_id for backward compatibility
		var schoolID uint
		var parseErr error

		// Convert tenant_id to school_id (for backward compatibility)
		// In a real system, you might have a mapping table or use tenant_id directly
		schoolID, parseErr = convertTenantIDToSchoolID(tenantID)

		if isAdmin {
			// Admin users: Allow override via query parameter, but use JWT claims as fallback
			if schoolParam := c.Query("school_id"); schoolParam != "" {
				if parsedSchoolID, err := strconv.ParseUint(schoolParam, 10, 32); err == nil {
					schoolID = uint(parsedSchoolID)
				} else {
					c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid school_id parameter"})
					c.Abort()
					return
				}
			} else if parseErr != nil {
				// Admin users without valid tenant_id default to school 1
				// This maintains backward compatibility for legacy admin tokens
				schoolID = 1
			}
		} else {
			// Regular users: Must have valid tenant_id in token claims
			if parseErr != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid tenant_id in token"})
				c.Abort()
				return
			}
		}

		// Store tenant information in context
		ctx := context.WithValue(c.Request.Context(), SchoolIDContextKey, schoolID)
		ctx = context.WithValue(ctx, RealmContextKey, realm)
		c.Request = c.Request.WithContext(ctx)

		// Add tenant info to Gin context for easy access
		c.Set("school_id", schoolID)
		c.Set("tenant_id", tenantID)
		c.Set("realm", realm)

		c.Next()
	}
}

// extractRealmFromIssuer extracts the realm name from the issuer URL
// Expected format: http://localhost:8080/auth/realms/school-abc
func extractRealmFromIssuer(claims jwt.MapClaims) string {
	iss, ok := claims["iss"].(string)
	if !ok {
		return ""
	}

	// Extract realm from issuer URL
	// Example: http://localhost:8080/auth/realms/orbis-dev -> orbis-dev
	const realmPrefix = "/realms/"
	if idx := strings.Index(iss, realmPrefix); idx != -1 {
		realmStart := idx + len(realmPrefix)
		// Find the end of the realm (next '/' or end of string)
		realmEnd := strings.Index(iss[realmStart:], "/")
		if realmEnd == -1 {
			return iss[realmStart:]
		}
		return iss[realmStart : realmStart+realmEnd]
	}
	return ""
}

// extractSchoolIDFromClaims extracts school_id from JWT custom claims
func extractSchoolIDFromClaims(claims jwt.MapClaims) (uint, error) {
	// Try different possible claim names for school_id
	possibleKeys := []string{"school_id", "schoolId", "school", "tenant_id", "tenantId"}

	for _, key := range possibleKeys {
		if schoolIDClaim, exists := claims[key]; exists {
			switch v := schoolIDClaim.(type) {
			case float64:
				return uint(v), nil
			case int:
				return uint(v), nil
			case string:
				if id, err := strconv.ParseUint(v, 10, 32); err == nil {
					return uint(id), nil
				}
			}
		}
	}

	return 0, jwt.ErrInvalidKey
}

// extractTenantIDFromClaims extracts tenant_id from JWT custom claims
func extractTenantIDFromClaims(claims jwt.MapClaims) (string, error) {
	// Try different possible claim names for tenant_id
	possibleKeys := []string{"tenant_id", "tenantId", "tenant"}

	for _, key := range possibleKeys {
		if tenantIDClaim, exists := claims[key]; exists {
			if tenantID, ok := tenantIDClaim.(string); ok && tenantID != "" {
				return tenantID, nil
			}
		}
	}

	return "", fmt.Errorf("tenant_id not found in claims")
}

// convertTenantIDToSchoolID converts tenant_id to school_id for backward compatibility
// In this implementation, we assume tenant1 -> school_id 1, tenant2 -> school_id 2, etc.
func convertTenantIDToSchoolID(tenantID string) (uint, error) {
	switch tenantID {
	case "tenant1":
		return 1, nil
	case "tenant2":
		return 2, nil
	default:
		// For other tenant IDs, try to extract numeric part
		if strings.HasPrefix(tenantID, "tenant") {
			numStr := strings.TrimPrefix(tenantID, "tenant")
			if id, err := strconv.ParseUint(numStr, 10, 32); err == nil {
				return uint(id), nil
			}
		}
		return 0, fmt.Errorf("unable to convert tenant_id %s to school_id", tenantID)
	}
}

// GetSchoolIDFromContext retrieves the school ID from the request context
func GetSchoolIDFromContext(c *gin.Context) (uint, bool) {
	if schoolID, exists := c.Get("school_id"); exists {
		if id, ok := schoolID.(uint); ok {
			return id, true
		}
	}
	return 0, false
}

// GetRealmFromContext retrieves the realm from the request context
func GetRealmFromContext(c *gin.Context) (string, bool) {
	if realm, exists := c.Get("realm"); exists {
		if r, ok := realm.(string); ok {
			return r, true
		}
	}
	return "", false
}

// GetTenantIDFromContext retrieves the tenant ID from the request context
func GetTenantIDFromContext(c *gin.Context) (string, bool) {
	if tenantID, exists := c.Get("tenant_id"); exists {
		if id, ok := tenantID.(string); ok {
			return id, true
		}
	}
	return "", false
}

// RequireSchoolAccess middleware ensures user has access to the specified school
func RequireSchoolAccess() gin.HandlerFunc {
	return func(c *gin.Context) {
		userSchoolID, exists := GetSchoolIDFromContext(c)
		if !exists {
			c.JSON(http.StatusForbidden, gin.H{"error": "School access required"})
			c.Abort()
			return
		}

		// Check if the requested resource belongs to the user's school
		// This can be enhanced to check specific resource access
		if requestedSchoolID := c.Param("school_id"); requestedSchoolID != "" {
			if parsedID, err := strconv.ParseUint(requestedSchoolID, 10, 32); err == nil {
				if uint(parsedID) != userSchoolID {
					c.JSON(http.StatusForbidden, gin.H{"error": "Access denied to this school's resources"})
					c.Abort()
					return
				}
			}
		}

		c.Next()
	}
}

// GetTenantInfo returns both school ID and realm from context
func GetTenantInfo(c *gin.Context) (schoolID uint, realm string, exists bool) {
	var schoolIDExists, realmExists bool
	schoolID, schoolIDExists = GetSchoolIDFromContext(c)
	realm, realmExists = GetRealmFromContext(c)
	return schoolID, realm, schoolIDExists && realmExists
}

// isAdminUser checks if the user has admin privileges using configured roles
func isAdminUser(roles []string) bool {
	// Get admin roles from configuration
	roleManager := NewRoleManager()
	configuredAdminRoles := roleManager.config.AdminRoles

	// Add standard Keycloak admin roles
	adminRoles := append(configuredAdminRoles,
		"manage-realm",
		"manage-users",
		"realm-admin",
	)

	for _, userRole := range roles {
		for _, adminRole := range adminRoles {
			if userRole == adminRole {
				return true
			}
		}
	}

	return false
}

// TenantRateLimitMiddleware enforces per-tenant rate limiting
func TenantRateLimitMiddleware(redisClient *redis.Client, rateLimitCache RateLimitCache) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Skip rate limiting if Redis is not available
		if redisClient == nil {
			c.Next()
			return
		}

		// Get tenant ID from context (set by TenantMiddleware)
		tenantID, exists := c.Get("tenant_id")
		if !exists {
			// If no tenant ID, skip rate limiting
			c.Next()
			return
		}

		tenantIDStr, ok := tenantID.(string)
		if !ok || tenantIDStr == "" {
			c.Next()
			return
		}

		// Get rate limit for this tenant from cache
		rateLimit := 1000.0 // default
		if rateLimitCache != nil {
			if limit, exists := rateLimitCache.GetTenantRateLimitCached(tenantIDStr); exists {
				rateLimit = limit
			}
		}

		// Create rate limit key for this tenant
		rateLimitKey := fmt.Sprintf("ratelimit:golang-mt:%s", tenantIDStr)

		// Check rate limit using token bucket algorithm
		ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
		defer cancel()

		allowed, err := allowRequest(ctx, redisClient, rateLimitKey, rateLimit, int(rateLimit*2), 1*time.Hour)
		if err != nil {
			// If rate limiting fails, log error but allow request to continue
			// This prevents Redis issues from blocking all traffic
			c.Next()
			return
		}

		if !allowed {
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error":      "Rate limit exceeded",
				"tenant_id":  tenantIDStr,
				"rate_limit": rateLimit,
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

// RateLimitCache interface for accessing rate limit configuration
type RateLimitCache interface {
	GetTenantRateLimitCached(tenantID string) (float64, bool)
}

// allowRequest checks and updates the token bucket for rate limiting
func allowRequest(ctx context.Context, rdb *redis.Client, key string, rate float64, burst int, expiry time.Duration) (bool, error) {
	// Lua script for atomic token bucket
	tokenBucketLua := `
local key = KEYS[1]
local rate = tonumber(ARGV[1])
local burst = tonumber(ARGV[2])
local now = tonumber(ARGV[3])
local expiry = tonumber(ARGV[4])
local tokens = tonumber(redis.call('get', key) or burst)
local last_refill = tonumber(redis.call('get', key..':last_refill') or now)
local elapsed = now - last_refill
tokens = math.min(burst, tokens + elapsed * rate)
if tokens >= 1 then
  tokens = tokens - 1
  redis.call('set', key, tokens, 'EX', expiry)
  redis.call('set', key..':last_refill', now, 'EX', expiry)
  return 1
else
  redis.call('set', key, tokens, 'EX', expiry)
  redis.call('set', key..':last_refill', now, 'EX', expiry)
  return 0
end
`

	now := time.Now().Unix()
	result, err := rdb.Eval(ctx, tokenBucketLua, []string{key}, rate, burst, now, int(expiry.Seconds())).Result()
	if err != nil {
		return false, fmt.Errorf("redis token bucket error: %w", err)
	}
	allowed, ok := result.(int64)
	return ok && allowed == 1, nil
}
