package auth

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// JWTMiddleware creates a JWT authentication middleware for Gin
func JWTMiddleware(registry *JWKSRegistry, requiredRole string) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Extract token from Authorization header
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": ErrMissingAuth})
			c.Abort()
			return
		}

		// Check for Bearer token format
		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || parts[0] != "Bearer" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": ErrMissingAuth})
			c.Abort()
			return
		}

		tokenString := parts[1]

		// Get realm from context or use default
		realm := c.GetString("realm")
		if realm == "" {
			realm = registry.DefaultRealm
		}

		// Validate the token
		claims, err := registry.ValidateToken(tokenString, realm)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": ErrInvalidToken, "details": err.Error()})
			c.Abort()
			return
		}

		// Check required role if specified
		if requiredRole != "" && !claims.HasRole(requiredRole) {
			c.JSON(http.StatusForbidden, gin.H{"error": ErrInsufficientRole})
			c.Abort()
			return
		}

		// Set user context for downstream handlers
		c.Set("user_claims", claims)
		c.Set("user_id", claims.Subject)
		c.Set("username", claims.PreferredUsername)
		c.Set("tenant_id", claims.GetTenantID())

		// Add security headers
		c.Header("X-Content-Type-Options", XContentTypeOptions)
		c.Header("X-Frame-Options", XFrameOptions)
		c.Header("X-XSS-Protection", XSSProtection)

		c.Next()
	}
}

// OptionalJWTMiddleware creates a JWT middleware that doesn't require authentication
// but validates tokens if present
func OptionalJWTMiddleware(registry *JWKSRegistry) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Extract token from Authorization header
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			// No token provided, continue without authentication
			c.Next()
			return
		}

		// Check for Bearer token format
		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || parts[0] != "Bearer" {
			// Invalid format, continue without authentication
			c.Next()
			return
		}

		tokenString := parts[1]

		// Get realm from context or use default
		realm := c.GetString("realm")
		if realm == "" {
			realm = registry.DefaultRealm
		}

		// Validate the token if present
		claims, err := registry.ValidateToken(tokenString, realm)
		if err != nil {
			// Invalid token, continue without authentication but log the error
			// In production, you might want to log this for security monitoring
			c.Next()
			return
		}

		// Set user context for downstream handlers
		c.Set("user_claims", claims)
		c.Set("user_id", claims.Subject)
		c.Set("username", claims.PreferredUsername)
		c.Set("tenant_id", claims.GetTenantID())

		c.Next()
	}
}

// MultiTenantJWTMiddleware creates a JWT middleware that validates tokens against
// the appropriate issuer based on tenant detection
func MultiTenantJWTMiddleware(registry *JWKSRegistry, requiredRole string) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Extract token from Authorization header
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": ErrMissingAuth})
			c.Abort()
			return
		}

		// Check for Bearer token format
		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || parts[0] != "Bearer" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": ErrMissingAuth})
			c.Abort()
			return
		}

		tokenString := parts[1]

		// Get realm and issuer from context
		realm := c.GetString("realm")
		if realm == "" {
			realm = registry.DefaultRealm
		}

		issuerBaseURL := c.GetString("issuer_base_url")
		var claims *TokenClaims
		var err error

		// Validate against specific issuer if provided, otherwise use default
		if issuerBaseURL != "" {
			claims, err = registry.ValidateTokenForIssuer(tokenString, realm, issuerBaseURL)
		} else {
			claims, err = registry.ValidateToken(tokenString, realm)
		}

		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": ErrInvalidToken, "details": err.Error()})
			c.Abort()
			return
		}

		// Check required role if specified
		if requiredRole != "" && !claims.HasRole(requiredRole) {
			c.JSON(http.StatusForbidden, gin.H{"error": ErrInsufficientRole})
			c.Abort()
			return
		}

		// Set user context for downstream handlers
		c.Set("user_claims", claims)
		c.Set("user_id", claims.Subject)
		c.Set("username", claims.PreferredUsername)
		c.Set("tenant_id", claims.GetTenantID())

		// Add security headers
		c.Header("X-Content-Type-Options", XContentTypeOptions)
		c.Header("X-Frame-Options", XFrameOptions)
		c.Header("X-XSS-Protection", XSSProtection)

		c.Next()
	}
}

// GetUserFromContext extracts user claims from Gin context
func GetUserFromContext(c *gin.Context) (*TokenClaims, bool) {
	claims, exists := c.Get("user_claims")
	if !exists {
		return nil, false
	}

	userClaims, ok := claims.(*TokenClaims)
	return userClaims, ok
}

// GetUserIDFromContext extracts user ID from Gin context
func GetUserIDFromContext(c *gin.Context) (string, bool) {
	userID, exists := c.Get("user_id")
	if !exists {
		return "", false
	}

	id, ok := userID.(string)
	return id, ok
}

// GetTenantIDFromContext extracts tenant ID from Gin context
func GetTenantIDFromContext(c *gin.Context) (string, bool) {
	tenantID, exists := c.Get("tenant_id")
	if !exists {
		return "", false
	}

	id, ok := tenantID.(string)
	return id, ok
}

// RequireRole creates a middleware that requires a specific role
func RequireRole(role string) gin.HandlerFunc {
	return func(c *gin.Context) {
		claims, exists := GetUserFromContext(c)
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{"error": ErrNoUserContext})
			c.Abort()
			return
		}

		if !claims.HasRole(role) {
			c.JSON(http.StatusForbidden, gin.H{"error": ErrInsufficientRole})
			c.Abort()
			return
		}

		c.Next()
	}
}
