package auth

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

// TenantContextKey is used for storing tenant information in context
type TenantContextKey string

const (
	TenantIDContextKey TenantContextKey = "tenant_id"
	RealmContextKey    TenantContextKey = "realm"
)

// TenantMiddleware extracts tenant information from JWT claims and enforces multi-tenant isolation
func TenantMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get user claims from context (set by JWT middleware)
		claims, exists := GetUserFromContext(c)
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
			c.Abort()
			return
		}

		// Extract tenant_id from JWT claims
		tenantID := claims.GetTenantID()
		if tenantID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid or missing tenant_id in token"})
			c.Abort()
			return
		}

		// Check if this is a tenant-specific route (/t/{tenant_id}/...)
		if pathTenantID := c.Param("tenant_id"); pathTenantID != "" {
			// Validate that the JWT tenant_id matches the route tenant_id
			if tenantID != pathTenantID {
				c.JSON(http.StatusForbidden, gin.H{
					"error": "Access denied: tenant mismatch",
					"details": fmt.Sprintf("Token tenant '%s' does not match route tenant '%s'", 
						tenantID, pathTenantID),
				})
				c.Abort()
				return
			}
		}

		// Set tenant context for downstream handlers
		c.Set(string(TenantIDContextKey), tenantID)
		c.Set(string(RealmContextKey), extractRealmFromClaims(claims))

		c.Next()
	}
}

// SuperAdminMiddleware allows access to cross-tenant operations for super admins
func SuperAdminMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get user claims from context
		claims, exists := GetUserFromContext(c)
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
			c.Abort()
			return
		}

		// Check for super admin role
		if !claims.HasRole("super-admin") && !claims.HasRole("global-admin") {
			c.JSON(http.StatusForbidden, gin.H{"error": "Super admin access required"})
			c.Abort()
			return
		}

		c.Next()
	}
}

// TenantIsolationMiddleware enforces strict tenant isolation for database operations
func TenantIsolationMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get tenant ID from context
		tenantID, exists := c.Get(string(TenantIDContextKey))
		if !exists {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Tenant context not found"})
			c.Abort()
			return
		}

		// Validate tenant ID format (should be numeric for school_id)
		if _, err := strconv.Atoi(tenantID.(string)); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid tenant ID format"})
			c.Abort()
			return
		}

		// Add tenant ID to request headers for downstream services
		c.Header("X-Tenant-ID", tenantID.(string))

		c.Next()
	}
}

// extractRealmFromClaims extracts the realm from JWT claims
func extractRealmFromClaims(claims *TokenClaims) string {
	if claims.Realm != "" {
		return claims.Realm
	}

	// Fallback: extract from issuer
	if claims.Issuer != "" {
		parts := strings.Split(claims.Issuer, "/realms/")
		if len(parts) == 2 {
			return parts[1]
		}
	}

	return ""
}

// GetTenantFromContext extracts tenant ID from Gin context
func GetTenantFromContext(c *gin.Context) (string, bool) {
	tenantID, exists := c.Get(string(TenantIDContextKey))
	if !exists {
		return "", false
	}

	id, ok := tenantID.(string)
	return id, ok
}

// GetRealmFromContext extracts realm from Gin context
func GetRealmFromContext(c *gin.Context) (string, bool) {
	realm, exists := c.Get(string(RealmContextKey))
	if !exists {
		return "", false
	}

	r, ok := realm.(string)
	return r, ok
}

// ValidateTenantAccess checks if the current user has access to the specified tenant
func ValidateTenantAccess(c *gin.Context, targetTenantID string) error {
	// Get current user's tenant
	currentTenantID, exists := GetTenantFromContext(c)
	if !exists {
		return fmt.Errorf("no tenant context found")
	}

	// Get user claims to check for admin roles
	claims, exists := GetUserFromContext(c)
	if !exists {
		return fmt.Errorf("no user context found")
	}

	// Super admins can access any tenant
	if claims.HasRole("super-admin") || claims.HasRole("global-admin") {
		return nil
	}

	// Regular users can only access their own tenant
	if currentTenantID != targetTenantID {
		return fmt.Errorf("access denied: cannot access tenant %s from tenant %s", 
			targetTenantID, currentTenantID)
	}

	return nil
}

// RequireTenant creates a middleware that requires a specific tenant ID
func RequireTenant(requiredTenantID string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if err := ValidateTenantAccess(c, requiredTenantID); err != nil {
			c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
			c.Abort()
			return
		}

		c.Next()
	}
}
