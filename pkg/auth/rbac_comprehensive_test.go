package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v4"
	"github.com/stretchr/testify/assert"
)

func TestNewRoleManager(t *testing.T) {
	rm := NewRoleManager()
	assert.NotNil(t, rm)
	assert.NotNil(t, rm.config)
}

func TestGetRolePermission(t *testing.T) {
	assert.Equal(t, PermissionAdmin, GetRolePermission(RoleAdmin))
	assert.Equal(t, PermissionWrite, GetRolePermission(RoleEditor))
	assert.Equal(t, PermissionRead, GetRolePermission(RoleViewer))
	assert.Equal(t, PermissionNone, GetRolePermission(RoleSuspended))
	assert.Equal(t, PermissionNone, GetRolePermission("unknown"))
}

func TestHasPermissionComprehensive(t *testing.T) {
	tests := []struct {
		name       string
		roles      []string
		permission string
		expected   bool
	}{
		{"admin has admin", []string{"admin"}, PermissionAdmin, true},
		{"admin has write", []string{"admin"}, PermissionWrite, true},
		{"admin has read", []string{"admin"}, PermissionRead, true},
		{"editor has write", []string{"editor"}, PermissionWrite, true},
		{"editor has read", []string{"editor"}, PermissionRead, true},
		{"editor no admin", []string{"editor"}, PermissionAdmin, false},
		{"viewer has read", []string{"viewer"}, PermissionRead, true},
		{"viewer no write", []string{"viewer"}, PermissionWrite, false},
		{"viewer no admin", []string{"viewer"}, PermissionAdmin, false},
		{"suspended has nothing", []string{"suspended"}, PermissionRead, false},
		{"empty roles", []string{}, PermissionRead, false},
		{"multi-role escalation", []string{"viewer", "editor"}, PermissionWrite, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := HasPermission(tt.roles, tt.permission)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestIsAdminRole(t *testing.T) {
	assert.True(t, IsAdminRole("admin"))
	assert.False(t, IsAdminRole("editor"))
	assert.False(t, IsAdminRole("viewer"))
}

func TestIsWriteRole(t *testing.T) {
	assert.True(t, IsWriteRole("admin"))
	assert.True(t, IsWriteRole("editor"))
	assert.False(t, IsWriteRole("viewer"))
}

func TestIsReadOnlyRole(t *testing.T) {
	assert.True(t, IsReadOnlyRole("viewer"))
	assert.False(t, IsReadOnlyRole("admin"))
	assert.False(t, IsReadOnlyRole("editor"))
}

func TestIsSuspendedRole(t *testing.T) {
	assert.True(t, IsSuspendedRole("suspended"))
	assert.False(t, IsSuspendedRole("admin"))
	assert.False(t, IsSuspendedRole("viewer"))
}

func TestGetAdminRoles(t *testing.T) {
	roles := GetAdminRoles()
	assert.NotNil(t, roles)
}

func TestGetWriteRoles(t *testing.T) {
	roles := GetWriteRoles()
	assert.NotNil(t, roles)
}

func TestGetReadOnlyRoles(t *testing.T) {
	roles := GetReadOnlyRoles()
	assert.NotNil(t, roles)
}

func TestGetAllReadRoles(t *testing.T) {
	roles := GetAllReadRoles()
	assert.NotNil(t, roles)
}

func TestRequirePermissionLevel(t *testing.T) {
	t.Run("admin permission granted", func(t *testing.T) {
		router := gin.New()
		router.Use(func(c *gin.Context) {
			c.Set("user_info", map[string]interface{}{"roles": []string{"admin"}})
			c.Set("claims", jwt.MapClaims{
				"realm_access": map[string]interface{}{"roles": []interface{}{"admin"}},
			})
			c.Next()
		})
		router.Use(RequirePermissionLevel(PermissionAdmin))
		router.GET("/test", func(c *gin.Context) { c.String(200, "ok") })

		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/test", nil)
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("insufficient permission", func(t *testing.T) {
		router := gin.New()
		router.Use(func(c *gin.Context) {
			c.Set("user_info", map[string]interface{}{"roles": []string{"viewer"}})
			c.Set("claims", jwt.MapClaims{
				"realm_access": map[string]interface{}{"roles": []interface{}{"viewer"}},
			})
			c.Next()
		})
		router.Use(RequirePermissionLevel(PermissionAdmin))
		router.GET("/test", func(c *gin.Context) { c.String(200, "ok") })

		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/test", nil)
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusForbidden, w.Code)
	})

	t.Run("suspended user blocked", func(t *testing.T) {
		router := gin.New()
		router.Use(func(c *gin.Context) {
			c.Set("user_info", map[string]interface{}{"roles": []string{"suspended"}})
			c.Set("claims", jwt.MapClaims{
				"realm_access": map[string]interface{}{"roles": []interface{}{"suspended"}},
			})
			c.Next()
		})
		router.Use(RequirePermissionLevel(PermissionRead))
		router.GET("/test", func(c *gin.Context) { c.String(200, "ok") })

		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/test", nil)
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusForbidden, w.Code)
	})

	t.Run("no roles unauthorized", func(t *testing.T) {
		router := gin.New()
		router.Use(RequirePermissionLevel(PermissionRead))
		router.GET("/test", func(c *gin.Context) { c.String(200, "ok") })

		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/test", nil)
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})
}

func TestRequireSpecificRole(t *testing.T) {
	t.Run("has specific role", func(t *testing.T) {
		router := gin.New()
		router.Use(func(c *gin.Context) {
			c.Set("user_info", map[string]interface{}{"roles": []string{"editor"}})
			c.Next()
		})
		router.Use(RequireSpecificRole("editor"))
		router.GET("/test", func(c *gin.Context) { c.String(200, "ok") })

		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/test", nil)
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("missing specific role", func(t *testing.T) {
		router := gin.New()
		router.Use(func(c *gin.Context) {
			c.Set("user_info", map[string]interface{}{"roles": []string{"viewer"}})
			c.Next()
		})
		router.Use(RequireSpecificRole("admin"))
		router.GET("/test", func(c *gin.Context) { c.String(200, "ok") })

		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/test", nil)
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusForbidden, w.Code)
	})

	t.Run("case insensitive match", func(t *testing.T) {
		router := gin.New()
		router.Use(func(c *gin.Context) {
			c.Set("user_info", map[string]interface{}{"roles": []string{"Admin"}})
			c.Next()
		})
		router.Use(RequireSpecificRole("admin"))
		router.GET("/test", func(c *gin.Context) { c.String(200, "ok") })

		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/test", nil)
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
	})
}

func TestMasterAdminMiddleware(t *testing.T) {
	t.Run("master admin-cli access", func(t *testing.T) {
		router := gin.New()
		router.Use(func(c *gin.Context) {
			claims := jwt.MapClaims{
				"iss": "http://keycloak:8080/realms/master",
				"azp": "admin-cli",
			}
			token := &jwt.Token{Claims: claims, Valid: true}
			c.Set("token", token)
			c.Next()
		})
		router.Use(MasterAdminMiddleware())
		router.GET("/test", func(c *gin.Context) { c.String(200, "ok") })

		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/test", nil)
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("non-master realm rejected", func(t *testing.T) {
		router := gin.New()
		router.Use(func(c *gin.Context) {
			claims := jwt.MapClaims{
				"iss": "http://keycloak:8080/realms/tenant1",
				"azp": "admin-cli",
			}
			token := &jwt.Token{Claims: claims, Valid: true}
			c.Set("token", token)
			c.Next()
		})
		router.Use(MasterAdminMiddleware())
		router.GET("/test", func(c *gin.Context) { c.String(200, "ok") })

		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/test", nil)
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusForbidden, w.Code)
	})

	t.Run("non admin-cli client rejected", func(t *testing.T) {
		router := gin.New()
		router.Use(func(c *gin.Context) {
			claims := jwt.MapClaims{
				"iss": "http://keycloak:8080/realms/master",
				"azp": "other-client",
			}
			token := &jwt.Token{Claims: claims, Valid: true}
			c.Set("token", token)
			c.Next()
		})
		router.Use(MasterAdminMiddleware())
		router.GET("/test", func(c *gin.Context) { c.String(200, "ok") })

		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/test", nil)
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusForbidden, w.Code)
	})

	t.Run("no token rejected", func(t *testing.T) {
		router := gin.New()
		router.Use(MasterAdminMiddleware())
		router.GET("/test", func(c *gin.Context) { c.String(200, "ok") })

		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/test", nil)
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})
}

func TestSuperAdminMiddleware(t *testing.T) {
	t.Run("master admin-cli grants access", func(t *testing.T) {
		router := gin.New()
		router.Use(func(c *gin.Context) {
			claims := jwt.MapClaims{
				"iss": "http://keycloak:8080/realms/master",
				"azp": "admin-cli",
			}
			token := &jwt.Token{Claims: claims, Valid: true}
			c.Set("token", token)
			c.Next()
		})
		router.Use(SuperAdminMiddleware())
		router.GET("/test", func(c *gin.Context) { c.String(200, "ok") })

		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/test", nil)
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("admin role grants access", func(t *testing.T) {
		router := gin.New()
		router.Use(func(c *gin.Context) {
			claims := jwt.MapClaims{
				"iss": "http://keycloak:8080/realms/tenant1",
				"azp": "my-app",
			}
			token := &jwt.Token{Claims: claims, Valid: true}
			c.Set("token", token)
			c.Set("user_info", map[string]interface{}{"roles": []string{"admin"}})
			c.Set("claims", claims)
			c.Next()
		})
		router.Use(SuperAdminMiddleware())
		router.GET("/test", func(c *gin.Context) { c.String(200, "ok") })

		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/test", nil)
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("viewer role denied", func(t *testing.T) {
		router := gin.New()
		router.Use(func(c *gin.Context) {
			claims := jwt.MapClaims{
				"iss": "http://keycloak:8080/realms/tenant1",
				"azp": "my-app",
			}
			token := &jwt.Token{Claims: claims, Valid: true}
			c.Set("token", token)
			c.Set("user_info", map[string]interface{}{"roles": []string{"viewer"}})
			c.Set("claims", claims)
			c.Next()
		})
		router.Use(SuperAdminMiddleware())
		router.GET("/test", func(c *gin.Context) { c.String(200, "ok") })

		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/test", nil)
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusForbidden, w.Code)
	})
}

func TestRequireAdminPermission(t *testing.T) {
	mw := RequireAdminPermission()
	assert.NotNil(t, mw)
}

func TestRequireWritePermission(t *testing.T) {
	mw := RequireWritePermission()
	assert.NotNil(t, mw)
}

func TestRequireReadPermission(t *testing.T) {
	mw := RequireReadPermission()
	assert.NotNil(t, mw)
}

func TestRequireAdminRole(t *testing.T) {
	mw := RequireAdminRole()
	assert.NotNil(t, mw)
}
