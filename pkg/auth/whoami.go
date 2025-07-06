package auth

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
)

func WhoAmI(c *gin.Context) {
	// Record telemetry for whoami endpoint
	ctx := c.Request.Context()

	// Use the new utility function to get user info
	userInfo, exists := GetUserFromContext(c)
	if !exists {
		if GlobalTelemetry != nil {
			GlobalTelemetry.RecordAuthRequest(ctx, "", "", "", "whoami_no_context")
		}
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "No user context found",
			"code":  "NO_USER_CONTEXT",
		})
		return
	}

	// Log what we actually get from userInfo for debugging
	log.Printf("[DEBUG] whoami userInfo: %+v", userInfo)

	// Extract user_id and tenant_id from userInfo (which is populated from JWT claims)
	userID := ""
	tenantID := ""

	// Try to get user_id from sub claim first, fallback to username
	if id, ok := userInfo["sub"].(string); ok && id != "" {
		userID = id
		log.Printf("[DEBUG] whoami userID from sub: %s", userID)
	} else if username, ok := userInfo["username"].(string); ok && username != "" {
		userID = username // Use username as user_id if no sub claim
		log.Printf("[DEBUG] whoami userID from username: %s", userID)
	}

	if tid, ok := userInfo["tenant_id"].(string); ok {
		tenantID = tid
		log.Printf("[DEBUG] whoami tenantID from tenant_id: %s", tenantID)
	}

	// If tenant_id is still empty, try to extract from realm context
	if tenantID == "" {
		if realm, realmExists := GetRealmFromContext(c); realmExists && realm != "" {
			tenantID = realm
			log.Printf("[DEBUG] whoami tenantID from realm: %s", tenantID)
		}
	}

	// Get user roles using the utility function
	roles, _ := GetUserRoles(c)

	// Get realm information from context (set by TenantMiddleware)
	realm, _ := GetRealmFromContext(c)

	// Record successful whoami request
	if GlobalTelemetry != nil {
		roleStr := ""
		if len(roles) > 0 {
			roleStr = roles[0] // Use first role for telemetry
		}
		GlobalTelemetry.RecordAuthRequest(ctx, tenantID, userID, roleStr, "whoami_success")
	}

	// Return standard format: {"user_id": string, "tenant_id": string, "roles": []string}
	// Plus additional information for compatibility
	response := gin.H{
		"user_id":   userID,
		"tenant_id": tenantID,
		"roles":     roles,
		// Additional information for enhanced debugging/compatibility
		"user":  userInfo,
		"realm": realm,
		"permissions": gin.H{
			"can_manage_students": HasWriteRole(c),
			"can_view_students":   HasReadRole(c),
			"is_admin":            HasAdminRole(c),
		},
	}

	c.JSON(http.StatusOK, response)
}

// WhoAmIHandler is an alias for WhoAmI to maintain compatibility
func WhoAmIHandler(c *gin.Context) {
	WhoAmI(c)
}
