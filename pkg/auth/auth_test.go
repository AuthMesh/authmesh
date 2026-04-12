package auth

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRolePermissions(t *testing.T) {
	testCases := []struct {
		role       string
		permission string
		expected   string
	}{
		{"admin", "", "admin"},
		{"editor", "", "write"},
		{"viewer", "", "read"},
		{"suspended", "", "none"},
		{"unknown-role", "", "none"},
	}

	for _, tc := range testCases {
		t.Run(tc.role, func(t *testing.T) {
			permission := GetRolePermission(tc.role)
			assert.Equal(t, tc.expected, permission, 
				"Role %s should have permission %s", tc.role, tc.expected)
		})
	}
}

func TestHasPermission(t *testing.T) {
	testCases := []struct {
		name               string
		userRoles          []string
		requiredPermission string
		expected           bool
	}{
		{
			name:               "Admin_Has_Admin_Permission",
			userRoles:          []string{"admin"},
			requiredPermission: "admin",
			expected:           true,
		},
		{
			name:               "Admin_Has_Write_Permission",
			userRoles:          []string{"admin"},
			requiredPermission: "write",
			expected:           true,
		},
		{
			name:               "Admin_Has_Read_Permission",
			userRoles:          []string{"admin"},
			requiredPermission: "read",
			expected:           true,
		},
		{
			name:               "Editor_Has_Write_Permission",
			userRoles:          []string{"editor"},
			requiredPermission: "write",
			expected:           true,
		},
		{
			name:               "Editor_Has_Read_Permission",
			userRoles:          []string{"editor"},
			requiredPermission: "read",
			expected:           true,
		},
		{
			name:               "Editor_Cannot_Admin",
			userRoles:          []string{"editor"},
			requiredPermission: "admin",
			expected:           false,
		},
		{
			name:               "Viewer_Cannot_Write",
			userRoles:          []string{"viewer"},
			requiredPermission: "write",
			expected:           false,
		},
		{
			name:               "Viewer_Can_Read",
			userRoles:          []string{"viewer"},
			requiredPermission: "read",
			expected:           true,
		},
		{
			name:               "Suspended_Cannot_Access",
			userRoles:          []string{"suspended"},
			requiredPermission: "read",
			expected:           false,
		},
		{
			name:               "Multiple_Roles_Admin_Wins",
			userRoles:          []string{"viewer", "admin", "suspended"},
			requiredPermission: "admin",
			expected:           true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := HasPermission(tc.userRoles, tc.requiredPermission)
			assert.Equal(t, tc.expected, result,
				"User with roles %v should %s have permission %s",
				tc.userRoles,
				map[bool]string{true: "", false: "not"}[tc.expected],
				tc.requiredPermission)
		})
	}
}

func TestRoleCheckers(t *testing.T) {
	t.Run("Admin_Role_Detection", func(t *testing.T) {
		// Test the hardcoded admin role
		assert.True(t, IsAdminRole("admin"), "Role admin should be admin")
		
		// Test unknown roles
		nonAdminRoles := []string{"editor", "viewer", "suspended"}
		for _, role := range nonAdminRoles {
			assert.False(t, IsAdminRole(role), "Role %s should not be admin", role)
		}
	})

	t.Run("Write_Role_Detection", func(t *testing.T) {
		// Test the hardcoded write roles
		assert.True(t, IsWriteRole("editor"), "Role editor should be write")
		
		// Test non-write roles (note: admin has write permission but is primarily admin role)
		nonWriteRoles := []string{"viewer", "suspended"}
		for _, role := range nonWriteRoles {
			assert.False(t, IsWriteRole(role), "Role %s should not be write", role)
		}
	})

	t.Run("Read_Only_Role_Detection", func(t *testing.T) {
		// Test the hardcoded read-only role
		assert.True(t, IsReadOnlyRole("viewer"), "Role viewer should be read-only")
		
		// Test non-read-only roles
		nonReadOnlyRoles := []string{"admin", "editor", "suspended"}
		for _, role := range nonReadOnlyRoles {
			assert.False(t, IsReadOnlyRole(role), "Role %s should not be read-only", role)
		}
	})

	t.Run("Suspended_Role_Detection", func(t *testing.T) {
		assert.True(t, IsSuspendedRole("suspended"), "suspended should be suspended")
		
		nonSuspendedRoles := []string{"admin", "editor", "viewer"}
		for _, role := range nonSuspendedRoles {
			assert.False(t, IsSuspendedRole(role), "Role %s should not be suspended", role)
		}
	})
}

func TestRoleCollections(t *testing.T) {
	t.Run("Role_Collections_Exist", func(t *testing.T) {
		// Test that the role collection functions exist and return something
		adminRoles := GetAdminRoles()
		writeRoles := GetWriteRoles()
		readOnlyRoles := GetReadOnlyRoles()
		allReadRoles := GetAllReadRoles()
		
		// These should be slices (may be empty depending on config)
		assert.IsType(t, []string{}, adminRoles, "GetAdminRoles should return []string")
		assert.IsType(t, []string{}, writeRoles, "GetWriteRoles should return []string")
		assert.IsType(t, []string{}, readOnlyRoles, "GetReadOnlyRoles should return []string")
		assert.IsType(t, []string{}, allReadRoles, "GetAllReadRoles should return []string")
		
		t.Logf("Admin roles from config: %v", adminRoles)
		t.Logf("Write roles from config: %v", writeRoles)
		t.Logf("Read-only roles from config: %v", readOnlyRoles)
		t.Logf("All read roles from config: %v", allReadRoles)
	})
}

func TestTenantExtraction(t *testing.T) {
	testCases := []struct {
		name     string
		claims   map[string]interface{}
		expected string
		hasError bool
	}{
		{
			name: "Valid_Tenant_ID_In_Claims",
			claims: map[string]interface{}{
				"tenant_id": "tenant123",
				"sub":       "user456",
			},
			expected: "tenant123",
			hasError: false,
		},
		{
			name: "Missing_Tenant_ID",
			claims: map[string]interface{}{
				"sub": "user456",
			},
			expected: "",
			hasError: true,
		},
		{
			name: "Invalid_Tenant_ID_Type",
			claims: map[string]interface{}{
				"tenant_id": 123, // numeric instead of string
				"sub":       "user456",
			},
			expected: "",
			hasError: true,
		},
		{
			name: "Empty_Tenant_ID",
			claims: map[string]interface{}{
				"tenant_id": "",
				"sub":       "user456",
			},
			expected: "",
			hasError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tenantID, err := extractTenantIDFromClaims(tc.claims)
			
			if tc.hasError {
				assert.Error(t, err, "Should return error for case: %s", tc.name)
				assert.Empty(t, tenantID, "Tenant ID should be empty on error")
			} else {
				assert.NoError(t, err, "Should not return error for case: %s", tc.name)
				assert.Equal(t, tc.expected, tenantID, "Should extract correct tenant ID")
			}
		})
	}
}
