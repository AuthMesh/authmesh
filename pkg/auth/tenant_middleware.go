package auth

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v4"
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
		pathTenantID := c.Param("tenant_id")

		if pathTenantID != "" {
			// Validate that the JWT tenant_id matches the route tenant_id
			if tenantID != pathTenantID {
				c.JSON(http.StatusForbidden, gin.H{
					"error": "Access denied: token tenant_id does not match requested tenant",
				})
				c.Abort()
				return
			}
		}

		// Set context values for downstream handlers
		c.Set("tenant_id", tenantID)
		c.Next()
	}
}

// Helper function to extract tenant_id from claims
func extractTenantIDFromClaims(claims jwt.MapClaims) (string, error) {
	// First try to get tenant_id from custom claims
	if tenantID, ok := claims["tenant_id"].(string); ok && tenantID != "" {
		return tenantID, nil
	}

	// Fallback: extract realm from issuer claim (standard approach for Keycloak)
	if iss, ok := claims["iss"].(string); ok {
		// Extract realm from issuer URL
		// Example: http://localhost:9443/realms/tenant1 -> tenant1
		const realmPrefix = "/realms/"
		if idx := strings.Index(iss, realmPrefix); idx != -1 {
			realmStart := idx + len(realmPrefix)
			// Find the end of the realm (next '/' or end of string)
			realmEnd := strings.Index(iss[realmStart:], "/")
			if realmEnd == -1 {
				return iss[realmStart:], nil
			}
			return iss[realmStart : realmStart+realmEnd], nil
		}
	}

	return "", fmt.Errorf("tenant_id not found in token claims or issuer")
}
