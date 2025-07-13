package auth

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v4"
)

// TenantMiddleware extracts tenant information from JWT claims and enforces multi-tenant isolation
func TenantMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		fmt.Printf("DEBUG: TenantMiddleware function called for path: %s\n", c.Request.URL.Path)
		// Get full claims from context (set by JWT middleware)
		claimsInterface, exists := c.Get("claims")
		fmt.Printf("DEBUG: TenantMiddleware - claims exists: %v\n", exists)
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

		// Check if this is a tenant-specific route (/t/{tenant}/...)
		if pathTenantID := c.Param("tenant"); pathTenantID != "" {
			fmt.Printf("DEBUG: TenantMiddleware - pathTenantID from :tenant param: '%s'\n", pathTenantID)
			fmt.Printf("DEBUG: TenantMiddleware - tenantID from JWT: '%s'\n", tenantID)
			// Validate that the JWT tenant_id matches the route tenant_id
			if tenantID != pathTenantID {
				fmt.Printf("DEBUG: TenantMiddleware - TENANT MISMATCH - denying access\n")
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
			fmt.Printf("DEBUG: TenantMiddleware - TENANT MATCH - allowing access\n")
		}

		// Set context values for downstream handlers
		c.Set("tenant_id", tenantID)
		c.Next()
	}
}

// Helper function to extract tenant_id from claims
func extractTenantIDFromClaims(claims jwt.MapClaims) (string, error) {
	if tenantID, ok := claims["tenant_id"].(string); ok && tenantID != "" {
		return tenantID, nil
	}
	return "", fmt.Errorf("tenant_id not found or empty in token claims")
}
