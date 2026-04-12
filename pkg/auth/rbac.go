package auth

import (
	"log"
	"net/http"
	"strings"

	"github.com/AuthMesh/authmesh/pkg/config"
	"github.com/golang-jwt/jwt/v4"

	"github.com/gin-gonic/gin"
)

// Role definitions for RBAC
const (
	RoleAdmin     = "admin"
	RoleEditor    = "editor"
	RoleViewer    = "viewer"
	RoleSuspended = "suspended"
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
	RoleAdmin:     PermissionAdmin,
	RoleEditor:    PermissionWrite,
	RoleViewer:    PermissionRead,
	RoleSuspended: PermissionNone,
}

// RoleManager handles role-based permissions using configuration
type RoleManager struct {
	config *config.Config
}

// NewRoleManager creates a new role manager with current configuration
func NewRoleManager() *RoleManager {
	return &RoleManager{
		config: config.Load(),
	}
}

// Global role manager instance
var roleManager = NewRoleManager()

// GetRolePermission returns the permission level for a given role
func GetRolePermission(role string) string {
	if permission, exists := rolePermissions[role]; exists {
		return permission
	}
	return PermissionNone
}

// HasPermission checks if any of the user's roles have the required permission level
func HasPermission(userRoles []string, requiredPermission string) bool {
	for _, role := range userRoles {
		rolePermission := GetRolePermission(role)

		switch requiredPermission {
		case PermissionAdmin:
			if rolePermission == PermissionAdmin {
				return true
			}
		case PermissionWrite:
			if rolePermission == PermissionAdmin || rolePermission == PermissionWrite {
				return true
			}
		case PermissionRead:
			if rolePermission == PermissionAdmin || rolePermission == PermissionWrite || rolePermission == PermissionRead {
				return true
			}
		}
	}
	return false
}

// IsAdminRole checks if a role is considered an admin role
func IsAdminRole(role string) bool {
	return role == RoleAdmin || GetRolePermission(role) == PermissionAdmin
}

// IsWriteRole checks if a role has write permissions
func IsWriteRole(role string) bool {
	permission := GetRolePermission(role)
	return permission == PermissionAdmin || permission == PermissionWrite
}

// IsReadOnlyRole checks if a role has read-only permissions
func IsReadOnlyRole(role string) bool {
	return GetRolePermission(role) == PermissionRead
}

// IsSuspendedRole checks if a role is suspended
func IsSuspendedRole(role string) bool {
	// Only explicitly suspended roles should be treated as suspended
	return role == RoleSuspended
}

// HasAdminRole checks if the current user has any admin role
func HasAdminRole(c *gin.Context) bool {
	return HasAnyRole(c, roleManager.config.AdminRoles...)
}

// HasWriteRole checks if the current user has any write role
func HasWriteRole(c *gin.Context) bool {
	return HasAnyRole(c, roleManager.config.WriteRoles...)
}

// HasReadRole checks if the current user has any read role (includes write roles)
func HasReadRole(c *gin.Context) bool {
	// Combine read-only and write roles for read permissions
	allReadRoles := append(roleManager.config.ReadOnlyRoles, roleManager.config.WriteRoles...)
	return HasAnyRole(c, allReadRoles...)
}

// HasSuperAdminRole checks if the current user has super admin role
func HasSuperAdminRole(c *gin.Context) bool {
	return HasAnyRole(c, "super-admin")
}

// GetAdminRoles returns the list of admin roles from configuration
func GetAdminRoles() []string {
	return roleManager.config.AdminRoles
}

// GetWriteRoles returns the list of write roles from configuration
func GetWriteRoles() []string {
	return roleManager.config.WriteRoles
}

// GetReadOnlyRoles returns the list of read-only roles from configuration
func GetReadOnlyRoles() []string {
	return roleManager.config.ReadOnlyRoles
}

// GetAllReadRoles returns combined read-only and write roles
func GetAllReadRoles() []string {
	return append(roleManager.config.ReadOnlyRoles, roleManager.config.WriteRoles...)
}

// RequireAdminPermission middleware that requires admin permission level
func RequireAdminPermission() gin.HandlerFunc {
	return RequirePermissionLevel(PermissionAdmin)
}

// RequireWritePermission middleware that requires write permission level
func RequireWritePermission() gin.HandlerFunc {
	return RequirePermissionLevel(PermissionWrite)
}

// RequireReadPermission middleware that requires read permission level
func RequireReadPermission() gin.HandlerFunc {
	return RequirePermissionLevel(PermissionRead)
}

// RequirePermissionLevel creates middleware that validates permission level
func RequirePermissionLevel(requiredPermission string) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get user roles from context
		roles, exists := GetRolesFromContext(c)
		if !exists || len(roles) == 0 {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "No roles found in token"})
			c.Abort()
			return
		}

		// Check if user is explicitly suspended first
		for _, role := range roles {
			if role == RoleSuspended {
				c.JSON(http.StatusForbidden, gin.H{"error": "Account suspended"})
				c.Abort()
				return
			}
		}

		// Check if user has required permission
		if !HasPermission(roles, requiredPermission) {
			c.JSON(http.StatusForbidden, gin.H{
				"error":               "Insufficient permissions",
				"required_permission": requiredPermission,
				"user_roles":          roles,
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

// RequireSpecificRole creates middleware that requires a specific role
func RequireSpecificRole(requiredRole string) gin.HandlerFunc {
	return func(c *gin.Context) {
		roles, exists := GetRolesFromContext(c)
		if !exists || len(roles) == 0 {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "No roles found in token"})
			c.Abort()
			return
		}

		// Check for explicitly suspended role first
		for _, role := range roles {
			if role == RoleSuspended {
				c.JSON(http.StatusForbidden, gin.H{"error": "Account suspended"})
				c.Abort()
				return
			}
		}

		// Check for required role
		hasRole := false
		for _, role := range roles {
			if strings.EqualFold(role, requiredRole) {
				hasRole = true
				break
			}
		}

		if !hasRole {
			c.JSON(http.StatusForbidden, gin.H{
				"error":         "Required role not found",
				"required_role": requiredRole,
				"user_roles":    roles,
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

// RequireAdminRole middleware that requires admin role specifically
func RequireAdminRole() gin.HandlerFunc {
	return RequireSpecificRole(RoleAdmin)
}

// MasterAdminMiddleware validates access for master realm admin-cli client
// This is for the special case where the master admin doesn't have roles in the JWT
func MasterAdminMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		log.Printf("[DEBUG] MasterAdmin: Starting master admin middleware check")

		// Extract JWT claims from context (set by earlier middleware)
		token, exists := c.Get("token")
		if !exists {
			log.Printf("[DEBUG] MasterAdmin: No token found in context")
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized: missing authentication"})
			c.Abort()
			return
		}

		parsedToken, ok := token.(*jwt.Token)
		if !ok {
			log.Printf("[DEBUG] MasterAdmin: Invalid token type in context")
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized: invalid token"})
			c.Abort()
			return
		}

		mapClaims, ok := parsedToken.Claims.(jwt.MapClaims)
		if !ok {
			log.Printf("[DEBUG] MasterAdmin: Invalid claims type")
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized: invalid claims"})
			c.Abort()
			return
		}

		// Check that the issuer is from the master realm
		issuer, ok := mapClaims["iss"].(string)
		if !ok {
			log.Printf("[DEBUG] MasterAdmin: No issuer found in token")
			c.JSON(http.StatusForbidden, gin.H{"error": "Access denied: missing issuer"})
			c.Abort()
			return
		}

		log.Printf("[DEBUG] MasterAdmin: Token issuer: '%s'", issuer)

		// Extract realm from issuer URL (e.g., "http://localhost:8080/realms/master")
		parts := strings.Split(issuer, "/realms/")
		log.Printf("[DEBUG] MasterAdmin: Split parts: %v", parts)
		if len(parts) != 2 {
			log.Printf("[DEBUG] MasterAdmin: Invalid issuer format, parts length: %d", len(parts))
			c.JSON(http.StatusForbidden, gin.H{"error": "Access denied: invalid issuer format"})
			c.Abort()
			return
		}

		realmFromIssuer := strings.Split(parts[1], "/")[0]
		log.Printf("[DEBUG] MasterAdmin: Extracted realm: '%s'", realmFromIssuer)
		if realmFromIssuer != "master" {
			log.Printf("[DEBUG] MasterAdmin: Realm mismatch - expected 'master', got '%s'", realmFromIssuer)
			c.JSON(http.StatusForbidden, gin.H{"error": "Access denied: must be authenticated via master realm"})
			c.Abort()
			return
		}

		// Check that the client is admin-cli (master realm admin client)
		clientID, ok := mapClaims["azp"].(string)
		if !ok || clientID != "admin-cli" {
			log.Printf("[DEBUG] MasterAdmin: Invalid client - expected 'admin-cli', got '%s'", clientID)
			c.JSON(http.StatusForbidden, gin.H{"error": "Access denied: must use master realm admin client"})
			c.Abort()
			return
		}

		log.Printf("[DEBUG] MasterAdmin: Master realm admin access validated, proceeding to next middleware")
		c.Next()
	}
}

// SuperAdminMiddleware handles both regular superadmin roles and master admin client
// This supports both legacy admin-cli access and modern role-based access
func SuperAdminMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		log.Printf("[DEBUG] SuperAdmin: Starting superadmin middleware check")

		// First check if this is the master admin client (admin-cli)
		token, exists := c.Get("token")
		log.Printf("[DEBUG] SuperAdmin: Token exists in context: %v", exists)
		if exists {
			log.Printf("[DEBUG] SuperAdmin: Token type: %T", token)
			if parsedToken, ok := token.(*jwt.Token); ok {
				log.Printf("[DEBUG] SuperAdmin: Token is JWT, checking claims")
				if mapClaims, ok := parsedToken.Claims.(jwt.MapClaims); ok {
					// DEBUG: Log all the claims to understand what we have
					log.Printf("[DEBUG] SuperAdmin: All token claims: %+v", mapClaims)
					
					// Check for master realm + admin-cli combination
					if issuer, ok := mapClaims["iss"].(string); ok {
						log.Printf("[DEBUG] SuperAdmin: Token issuer: %s", issuer)
						if clientID, ok := mapClaims["azp"].(string); ok {
							log.Printf("[DEBUG] SuperAdmin: Token client_id (azp): %s", clientID)
							// Check if this is master realm admin-cli client
							if strings.Contains(issuer, "/realms/master") && clientID == "admin-cli" {
								log.Printf("[DEBUG] SuperAdmin: Master admin client detected, granting access")
								c.Next()
								return
							}
						} else {
							log.Printf("[DEBUG] SuperAdmin: No azp claim found in token")
						}
					} else {
						log.Printf("[DEBUG] SuperAdmin: No issuer claim found in token")
					}
				} else {
					log.Printf("[DEBUG] SuperAdmin: Invalid claims type: %T", parsedToken.Claims)
				}
			} else {
				log.Printf("[DEBUG] SuperAdmin: Invalid token type: %T", token)
			}
		}

		// If not master admin, check for regular superadmin roles
		log.Printf("[DEBUG] SuperAdmin: Not master admin, checking for superadmin roles")
		roles, exists := GetRolesFromContext(c)
		if !exists || len(roles) == 0 {
			log.Printf("[DEBUG] SuperAdmin: No roles found in token")
			c.JSON(http.StatusUnauthorized, gin.H{"error": "No roles found in token"})
			c.Abort()
			return
		}

		// Check for admin or superadmin roles
		hasAccess := false
		for _, role := range roles {
			if role == "admin" || role == "superadmin" || role == "super-admin" {
				hasAccess = true
				break
			}
		}

		if !hasAccess {
			log.Printf("[DEBUG] SuperAdmin: Insufficient permissions - roles: %v", roles)
			c.JSON(http.StatusForbidden, gin.H{
				"error":      "Insufficient permissions for superadmin access",
				"user_roles": roles,
			})
			c.Abort()
			return
		}

		log.Printf("[DEBUG] SuperAdmin: Access granted with roles: %v", roles)
		c.Next()
	}
}
