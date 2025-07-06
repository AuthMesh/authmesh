package auth

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// Role definitions for RBAC
const (
	RoleAdmin     = "admin"
	RoleEditor    = "editor"
	RoleViewer    = "viewer"
	RoleSuspended = "suspended"
	RoleSuperAdmin = "super-admin"
	RoleGlobalAdmin = "global-admin"
)

// Permission levels
const (
	PermissionAdmin = "admin"
	PermissionWrite = "write" 
	PermissionRead  = "read"
	PermissionNone  = "none"
)

// RBAC mapping for roles to permissions
var rolePermissions = map[string]string{
	RoleSuperAdmin:  PermissionAdmin,
	RoleGlobalAdmin: PermissionAdmin,
	RoleAdmin:       PermissionAdmin,
	RoleEditor:      PermissionWrite,
	RoleViewer:      PermissionRead,
	RoleSuspended:   PermissionNone,
}

// RoleManager handles role-based permissions
type RoleManager struct {
	customRoles map[string]string // Custom role to permission mappings
}

// NewRoleManager creates a new role manager
func NewRoleManager() *RoleManager {
	return &RoleManager{
		customRoles: make(map[string]string),
	}
}

// Global role manager instance
var defaultRoleManager = NewRoleManager()

// AddCustomRole adds a custom role with specified permission level
func (rm *RoleManager) AddCustomRole(role, permission string) {
	rm.customRoles[role] = permission
}

// GetPermissionLevel returns the permission level for a given role
func (rm *RoleManager) GetPermissionLevel(role string) string {
	// Check custom roles first
	if permission, exists := rm.customRoles[role]; exists {
		return permission
	}

	// Check default roles
	if permission, exists := rolePermissions[role]; exists {
		return permission
	}

	return PermissionNone
}

// HasPermission checks if a role has the required permission level
func (rm *RoleManager) HasPermission(role, requiredPermission string) bool {
	userPermission := rm.GetPermissionLevel(role)

	switch requiredPermission {
	case PermissionAdmin:
		return userPermission == PermissionAdmin
	case PermissionWrite:
		return userPermission == PermissionAdmin || userPermission == PermissionWrite
	case PermissionRead:
		return userPermission == PermissionAdmin || userPermission == PermissionWrite || userPermission == PermissionRead
	case PermissionNone:
		return true // Everyone has "none" permission
	}

	return false
}

// RequirePermission creates a middleware that requires a specific permission level
func RequirePermission(requiredPermission string) gin.HandlerFunc {
	return RequirePermissionWithManager(defaultRoleManager, requiredPermission)
}

// RequirePermissionWithManager creates a middleware with a custom role manager
func RequirePermissionWithManager(rm *RoleManager, requiredPermission string) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get user claims from context
		claims, exists := GetUserFromContext(c)
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
			c.Abort()
			return
		}

		// Check permissions for all user roles
		hasPermission := false
		if claims.RealmAccess != nil {
			if roles, ok := claims.RealmAccess["roles"].([]interface{}); ok {
				for _, roleInterface := range roles {
					if role, ok := roleInterface.(string); ok {
						if rm.HasPermission(role, requiredPermission) {
							hasPermission = true
							break
						}
					}
				}
			}
		}

		if !hasPermission {
			c.JSON(http.StatusForbidden, gin.H{
				"error": "Insufficient permissions",
				"required": requiredPermission,
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

// GetUserRoles extracts all roles from user claims
func GetUserRoles(claims *TokenClaims) []string {
	var roles []string

	if claims.RealmAccess != nil {
		if roleList, ok := claims.RealmAccess["roles"].([]interface{}); ok {
			for _, roleInterface := range roleList {
				if role, ok := roleInterface.(string); ok {
					roles = append(roles, role)
				}
			}
		}
	}

	return roles
}

// GetUserHighestPermission returns the highest permission level for a user
func GetUserHighestPermission(claims *TokenClaims) string {
	return GetUserHighestPermissionWithManager(defaultRoleManager, claims)
}

// GetUserHighestPermissionWithManager returns the highest permission level using a custom role manager
func GetUserHighestPermissionWithManager(rm *RoleManager, claims *TokenClaims) string {
	roles := GetUserRoles(claims)
	highestPermission := PermissionNone

	for _, role := range roles {
		permission := rm.GetPermissionLevel(role)
		
		// Admin is highest
		if permission == PermissionAdmin {
			return PermissionAdmin
		}
		
		// Write is higher than read
		if permission == PermissionWrite && highestPermission != PermissionAdmin {
			highestPermission = PermissionWrite
		}
		
		// Read is higher than none
		if permission == PermissionRead && highestPermission == PermissionNone {
			highestPermission = PermissionRead
		}
	}

	return highestPermission
}

// IsAdmin checks if the user has admin privileges
func IsAdmin(claims *TokenClaims) bool {
	return GetUserHighestPermission(claims) == PermissionAdmin
}

// CanWrite checks if the user has write privileges
func CanWrite(claims *TokenClaims) bool {
	permission := GetUserHighestPermission(claims)
	return permission == PermissionAdmin || permission == PermissionWrite
}

// CanRead checks if the user has read privileges
func CanRead(claims *TokenClaims) bool {
	permission := GetUserHighestPermission(claims)
	return permission == PermissionAdmin || permission == PermissionWrite || permission == PermissionRead
}

// RoleBasedAccessMiddleware creates a middleware that checks user roles against allowed roles
func RoleBasedAccessMiddleware(allowedRoles ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get user claims from context
		claims, exists := GetUserFromContext(c)
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
			c.Abort()
			return
		}

		// Get user roles
		userRoles := GetUserRoles(claims)

		// Check if user has any of the allowed roles
		hasAllowedRole := false
		for _, userRole := range userRoles {
			for _, allowedRole := range allowedRoles {
				if strings.EqualFold(userRole, allowedRole) {
					hasAllowedRole = true
					break
				}
			}
			if hasAllowedRole {
				break
			}
		}

		if !hasAllowedRole {
			c.JSON(http.StatusForbidden, gin.H{
				"error": "Access denied",
				"allowed_roles": allowedRoles,
				"user_roles": userRoles,
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

// AdminOnlyMiddleware creates a middleware that only allows admin users
func AdminOnlyMiddleware() gin.HandlerFunc {
	return RoleBasedAccessMiddleware(RoleAdmin, RoleSuperAdmin, RoleGlobalAdmin)
}

// EditorOrAboveMiddleware creates a middleware that allows editors and admins
func EditorOrAboveMiddleware() gin.HandlerFunc {
	return RoleBasedAccessMiddleware(RoleEditor, RoleAdmin, RoleSuperAdmin, RoleGlobalAdmin)
}

// ViewerOrAboveMiddleware creates a middleware that allows viewers, editors, and admins
func ViewerOrAboveMiddleware() gin.HandlerFunc {
	return RoleBasedAccessMiddleware(RoleViewer, RoleEditor, RoleAdmin, RoleSuperAdmin, RoleGlobalAdmin)
}
