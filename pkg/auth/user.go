package auth

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// User represents a user in the system
type User struct {
	ID                string    `json:"id"`
	Username          string    `json:"username"`
	Email             string    `json:"email"`
	FirstName         string    `json:"first_name"`
	LastName          string    `json:"last_name"`
	Enabled           bool      `json:"enabled"`
	EmailVerified     bool      `json:"email_verified"`
	TenantID          string    `json:"tenant_id"`
	Realm             string    `json:"realm"`
	Roles             []string  `json:"roles"`
	CreatedTimestamp  int64     `json:"created_timestamp"`
	LastLogin         *time.Time `json:"last_login,omitempty"`
}

// UserProfile represents a user's profile information
type UserProfile struct {
	Username      string            `json:"username"`
	Email         string            `json:"email"`
	FirstName     string            `json:"first_name"`
	LastName      string            `json:"last_name"`
	Attributes    map[string][]string `json:"attributes,omitempty"`
	TenantID      string            `json:"tenant_id"`
	Realm         string            `json:"realm"`
}

// UserManager provides user management operations
type UserManager struct {
	registry *JWKSRegistry
}

// NewUserManager creates a new user manager
func NewUserManager(registry *JWKSRegistry) *UserManager {
	return &UserManager{
		registry: registry,
	}
}

// GetCurrentUser returns the current user information from the JWT claims
func (um *UserManager) GetCurrentUser(c *gin.Context) (*User, error) {
	claims, exists := GetUserFromContext(c)
	if !exists {
		return nil, fmt.Errorf("no user context found")
	}

	user := &User{
		ID:               claims.Subject,
		Username:         claims.PreferredUsername,
		TenantID:         claims.GetTenantID(),
		Realm:            extractRealmFromClaims(claims),
		Roles:            GetUserRoles(claims),
		CreatedTimestamp: claims.IssuedAt.Unix(),
	}

	// Extract additional user information from claims if available
	if claims.RegisteredClaims.Issuer != "" {
		user.Realm = extractRealmFromClaims(claims)
	}

	return user, nil
}

// GetUserProfile returns the user's profile information
func (um *UserManager) GetUserProfile(c *gin.Context) (*UserProfile, error) {
	claims, exists := GetUserFromContext(c)
	if !exists {
		return nil, fmt.Errorf("no user context found")
	}

	profile := &UserProfile{
		Username:  claims.PreferredUsername,
		TenantID:  claims.GetTenantID(),
		Realm:     extractRealmFromClaims(claims),
	}

	// Try to extract additional profile information from claims
	// This would typically come from custom claims in the JWT
	profile.Email = claims.Subject // Subject often contains email in some setups

	return profile, nil
}

// ValidateUserAccess checks if the current user has access to perform operations on behalf of another user
func (um *UserManager) ValidateUserAccess(c *gin.Context, targetUserID string) error {
	claims, exists := GetUserFromContext(c)
	if !exists {
		return fmt.Errorf("no user context found")
	}

	// Users can always access their own data
	if claims.Subject == targetUserID {
		return nil
	}

	// Admins can access other users in the same tenant
	if IsAdmin(claims) {
		return nil
	}

	// Super admins can access any user
	if claims.HasRole(RoleSuperAdmin) || claims.HasRole(RoleGlobalAdmin) {
		return nil
	}

	return fmt.Errorf("access denied: insufficient permissions to access user %s", targetUserID)
}

// RequireUserAccess creates a middleware that validates user access
func RequireUserAccess(userIDParam string) gin.HandlerFunc {
	return func(c *gin.Context) {
		targetUserID := c.Param(userIDParam)
		if targetUserID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "User ID parameter required"})
			c.Abort()
			return
		}

		um := NewUserManager(Registry)
		if err := um.ValidateUserAccess(c, targetUserID); err != nil {
			c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
			c.Abort()
			return
		}

		c.Next()
	}
}

// GetUserInfo returns basic user information for API responses
func GetUserInfo(c *gin.Context) gin.H {
	claims, exists := GetUserFromContext(c)
	if !exists {
		return gin.H{"authenticated": false}
	}

	return gin.H{
		"authenticated": true,
		"user_id":      claims.Subject,
		"username":     claims.PreferredUsername,
		"tenant_id":    claims.GetTenantID(),
		"realm":        extractRealmFromClaims(claims),
		"roles":        GetUserRoles(claims),
		"permissions":  GetUserHighestPermission(claims),
		"issued_at":    claims.IssuedAt.Unix(),
		"expires_at":   claims.ExpiresAt.Unix(),
	}
}

// WhoAmI handler returns information about the current user
func WhoAmIHandler(c *gin.Context) {
	userInfo := GetUserInfo(c)
	c.JSON(http.StatusOK, userInfo)
}

// UserContextMiddleware adds user context information to responses
func UserContextMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Set user context in response headers for debugging (in development only)
		if gin.Mode() != gin.ReleaseMode {
			if userID, exists := GetUserIDFromContext(c); exists {
				c.Header("X-User-ID", userID)
			}
			if tenantID, exists := GetTenantIDFromContext(c); exists {
				c.Header("X-Tenant-ID", tenantID)
			}
		}

		c.Next()
	}
}

// SessionInfo represents session information for a user
type SessionInfo struct {
	UserID    string    `json:"user_id"`
	Username  string    `json:"username"`
	TenantID  string    `json:"tenant_id"`
	Realm     string    `json:"realm"`
	IssuedAt  time.Time `json:"issued_at"`
	ExpiresAt time.Time `json:"expires_at"`
	Active    bool      `json:"active"`
}

// GetSessionInfo returns session information for the current user
func GetSessionInfo(c *gin.Context) (*SessionInfo, error) {
	claims, exists := GetUserFromContext(c)
	if !exists {
		return nil, fmt.Errorf("no user context found")
	}

	return &SessionInfo{
		UserID:    claims.Subject,
		Username:  claims.PreferredUsername,
		TenantID:  claims.GetTenantID(),
		Realm:     extractRealmFromClaims(claims),
		IssuedAt:  claims.IssuedAt.Time,
		ExpiresAt: claims.ExpiresAt.Time,
		Active:    claims.ExpiresAt.Time.After(time.Now()),
	}, nil
}

// SessionInfoHandler returns session information for the current user
func SessionInfoHandler(c *gin.Context) {
	sessionInfo, err := GetSessionInfo(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, sessionInfo)
}
