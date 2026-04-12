package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/MicahParks/keyfunc"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v4"
	"github.com/stretchr/testify/assert"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// helper to create a gin context with claims set
func newContextWithClaims(claims jwt.MapClaims) *gin.Context {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/test", nil)
	c.Set("claims", claims)
	token := &jwt.Token{Claims: claims, Valid: true}
	c.Set("token", token)
	return c
}

func newContextWithUserInfo(info map[string]interface{}) *gin.Context {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/test", nil)
	c.Set("user_info", info)
	return c
}

func TestSanitizeLogValue(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"clean string", "hello world", "hello world"},
		{"newline injection", "hello\nworld", "helloworld"},
		{"carriage return", "hello\rworld", "helloworld"},
		{"tab replacement", "hello\tworld", "hello world"},
		{"mixed control chars", "line1\r\nline2\ttab", "line1line2 tab"},
		{"empty string", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sanitizeLogValue(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestExtractRoles(t *testing.T) {
	tests := []struct {
		name     string
		claims   jwt.MapClaims
		expected []string
	}{
		{
			name: "keycloak realm_access format",
			claims: jwt.MapClaims{
				"realm_access": map[string]interface{}{
					"roles": []interface{}{"admin", "user"},
				},
			},
			expected: []string{"admin", "user"},
		},
		{
			name: "top-level roles as interface array",
			claims: jwt.MapClaims{
				"roles": []interface{}{"editor", "viewer"},
			},
			expected: []string{"editor", "viewer"},
		},
		{
			name: "top-level roles as string array",
			claims: jwt.MapClaims{
				"roles": []string{"admin"},
			},
			expected: []string{"admin"},
		},
		{
			name:     "no roles",
			claims:   jwt.MapClaims{},
			expected: []string{},
		},
		{
			name: "realm_access takes priority",
			claims: jwt.MapClaims{
				"realm_access": map[string]interface{}{
					"roles": []interface{}{"from-realm"},
				},
				"roles": []interface{}{"from-top"},
			},
			expected: []string{"from-realm"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractRoles(tt.claims)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestExtractUserInfo(t *testing.T) {
	claims := jwt.MapClaims{
		"sub":                "user-123",
		"email":              "test@example.com",
		"preferred_username": "testuser",
		"name":               "Test User",
		"tenant_id":          "tenant-1",
		"realm_access": map[string]interface{}{
			"roles": []interface{}{"admin"},
		},
	}

	info := extractUserInfo(claims)
	assert.Equal(t, "user-123", info["sub"])
	assert.Equal(t, "test@example.com", info["email"])
	assert.Equal(t, "testuser", info["username"])
	assert.Equal(t, "Test User", info["name"])
	assert.Equal(t, "tenant-1", info["tenant_id"])
	assert.Equal(t, []string{"admin"}, info["roles"])
}

func TestGetUserFromContext(t *testing.T) {
	t.Run("valid user info", func(t *testing.T) {
		info := map[string]interface{}{"sub": "user-1"}
		c := newContextWithUserInfo(info)
		result, exists := GetUserFromContext(c)
		assert.True(t, exists)
		assert.Equal(t, "user-1", result["sub"])
	})

	t.Run("no user info", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
		_, exists := GetUserFromContext(c)
		assert.False(t, exists)
	})

	t.Run("invalid user info type", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
		c.Set("user_info", "not-a-map")
		_, exists := GetUserFromContext(c)
		assert.False(t, exists)
	})
}

func TestGetUserRoles(t *testing.T) {
	t.Run("roles from claims", func(t *testing.T) {
		claims := jwt.MapClaims{
			"realm_access": map[string]interface{}{
				"roles": []interface{}{"admin", "user"},
			},
		}
		c := newContextWithClaims(claims)
		roles, exists := GetUserRoles(c)
		assert.True(t, exists)
		assert.Equal(t, []string{"admin", "user"}, roles)
	})

	t.Run("no claims", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
		_, exists := GetUserRoles(c)
		assert.False(t, exists)
	})
}

func TestHasRole(t *testing.T) {
	claims := jwt.MapClaims{
		"realm_access": map[string]interface{}{
			"roles": []interface{}{"admin", "user"},
		},
	}
	c := newContextWithClaims(claims)

	assert.True(t, HasRole(c, "admin"))
	assert.True(t, HasRole(c, "user"))
	assert.False(t, HasRole(c, "superadmin"))
}

func TestHasAnyRole(t *testing.T) {
	claims := jwt.MapClaims{
		"realm_access": map[string]interface{}{
			"roles": []interface{}{"editor"},
		},
	}
	c := newContextWithClaims(claims)

	assert.True(t, HasAnyRole(c, "admin", "editor"))
	assert.False(t, HasAnyRole(c, "admin", "superadmin"))
}

func TestGetRolesFromContext(t *testing.T) {
	t.Run("from user_info", func(t *testing.T) {
		info := map[string]interface{}{"roles": []string{"admin", "user"}}
		c := newContextWithUserInfo(info)
		roles, exists := GetRolesFromContext(c)
		assert.True(t, exists)
		assert.Equal(t, []string{"admin", "user"}, roles)
	})

	t.Run("fallback from claims", func(t *testing.T) {
		claims := jwt.MapClaims{
			"realm_access": map[string]interface{}{
				"roles": []interface{}{"viewer"},
			},
		}
		c := newContextWithClaims(claims)
		roles, exists := GetRolesFromContext(c)
		assert.True(t, exists)
		assert.Equal(t, []string{"viewer"}, roles)
	})
}

func TestGetUserSubjectFromContext(t *testing.T) {
	t.Run("subject present", func(t *testing.T) {
		claims := jwt.MapClaims{"sub": "user-456"}
		c := newContextWithClaims(claims)
		sub, exists := GetUserSubjectFromContext(c)
		assert.True(t, exists)
		assert.Equal(t, "user-456", sub)
	})

	t.Run("subject missing", func(t *testing.T) {
		claims := jwt.MapClaims{}
		c := newContextWithClaims(claims)
		_, exists := GetUserSubjectFromContext(c)
		assert.False(t, exists)
	})
}

func TestGetTenantIDFromJWTClaims(t *testing.T) {
	t.Run("tenant present", func(t *testing.T) {
		claims := jwt.MapClaims{"tenant_id": "tenant-abc"}
		c := newContextWithClaims(claims)
		tid, exists := GetTenantIDFromJWTClaims(c)
		assert.True(t, exists)
		assert.Equal(t, "tenant-abc", tid)
	})

	t.Run("tenant missing", func(t *testing.T) {
		claims := jwt.MapClaims{}
		c := newContextWithClaims(claims)
		_, exists := GetTenantIDFromJWTClaims(c)
		assert.False(t, exists)
	})
}

func TestGetRealmFromContext(t *testing.T) {
	t.Run("realm set", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Set("tenant_id", "my-realm")
		realm, exists := GetRealmFromContext(c)
		assert.True(t, exists)
		assert.Equal(t, "my-realm", realm)
	})

	t.Run("realm not set", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		_, exists := GetRealmFromContext(c)
		assert.False(t, exists)
	})
}

func TestIsJWKSReady(t *testing.T) {
	// Save and restore registry state
	origURL := Registry.KeycloakURL
	origRealm := Registry.DefaultRealm
	origStore := Registry.JwksStore
	defer func() {
		Registry.KeycloakURL = origURL
		Registry.DefaultRealm = origRealm
		Registry.JwksStore = origStore
	}()

	t.Run("not ready - empty store", func(t *testing.T) {
		Registry.JwksStore = make(map[string]*keyfunc.JWKS)
		Registry.DefaultRealm = ""
		assert.False(t, IsJWKSReady())
	})

	t.Run("not ready - no realm", func(t *testing.T) {
		Registry.JwksStore = map[string]*keyfunc.JWKS{"test": nil}
		Registry.DefaultRealm = ""
		assert.False(t, IsJWKSReady())
	})
}

func TestGetJWKSStatus(t *testing.T) {
	origURL := Registry.KeycloakURL
	origRealm := Registry.DefaultRealm
	origStore := Registry.JwksStore
	defer func() {
		Registry.KeycloakURL = origURL
		Registry.DefaultRealm = origRealm
		Registry.JwksStore = origStore
	}()

	Registry.KeycloakURL = "http://test:8080"
	Registry.DefaultRealm = "master"
	Registry.JwksStore = make(map[string]*keyfunc.JWKS)

	status := GetJWKSStatus()
	assert.Equal(t, "ready", status["status"])
	assert.Equal(t, 0, status["realms"])
	assert.Equal(t, "master", status["default_realm"])
}

func TestValidateTrustedIssuer(t *testing.T) {
	r := &JWKSRegistry{
		TrustedIssuers: []string{"https://auth.example.com", "https://backup.example.com"},
	}

	assert.True(t, r.validateTrustedIssuer("https://auth.example.com"))
	assert.True(t, r.validateTrustedIssuer("https://backup.example.com"))
	assert.False(t, r.validateTrustedIssuer("https://evil.example.com"))
}

func TestValidateTrustedIssuerEmpty(t *testing.T) {
	r := &JWKSRegistry{TrustedIssuers: []string{}}
	// Empty trusted issuers = allow all (with warning)
	assert.True(t, r.validateTrustedIssuer("https://anything.com"))
}

func TestSetTrustedIssuers(t *testing.T) {
	r := &JWKSRegistry{}
	r.SetTrustedIssuers([]string{"https://new.example.com"})
	assert.Equal(t, []string{"https://new.example.com"}, r.TrustedIssuers)
}

func TestSecurityMiddleware(t *testing.T) {
	router := gin.New()
	router.Use(SecurityMiddleware())
	router.GET("/test", func(c *gin.Context) {
		c.String(200, "ok")
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/test", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)
	assert.Equal(t, "nosniff", w.Header().Get("X-Content-Type-Options"))
	assert.Equal(t, "DENY", w.Header().Get("X-Frame-Options"))
	assert.Equal(t, "1; mode=block", w.Header().Get("X-XSS-Protection"))
	assert.Equal(t, "strict-origin-when-cross-origin", w.Header().Get("Referrer-Policy"))
}

func TestRequireRoleMiddleware(t *testing.T) {
	t.Run("no claims - unauthorized", func(t *testing.T) {
		router := gin.New()
		router.Use(RequireRole("admin"))
		router.GET("/test", func(c *gin.Context) { c.String(200, "ok") })

		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/test", nil)
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("has required role", func(t *testing.T) {
		router := gin.New()
		router.Use(func(c *gin.Context) {
			c.Set("claims", jwt.MapClaims{
				"realm_access": map[string]interface{}{
					"roles": []interface{}{"admin", "user"},
				},
			})
			c.Next()
		})
		router.Use(RequireRole("admin"))
		router.GET("/test", func(c *gin.Context) { c.String(200, "ok") })

		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/test", nil)
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("missing required role", func(t *testing.T) {
		router := gin.New()
		router.Use(func(c *gin.Context) {
			c.Set("claims", jwt.MapClaims{
				"realm_access": map[string]interface{}{
					"roles": []interface{}{"viewer"},
				},
			})
			c.Next()
		})
		router.Use(RequireRole("admin"))
		router.GET("/test", func(c *gin.Context) { c.String(200, "ok") })

		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/test", nil)
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusForbidden, w.Code)
	})

	t.Run("suspended user", func(t *testing.T) {
		router := gin.New()
		router.Use(func(c *gin.Context) {
			c.Set("claims", jwt.MapClaims{
				"realm_access": map[string]interface{}{
					"roles": []interface{}{"suspended", "admin"},
				},
			})
			c.Next()
		})
		router.Use(RequireRole("admin"))
		router.GET("/test", func(c *gin.Context) { c.String(200, "ok") })

		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/test", nil)
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusForbidden, w.Code)
	})
}

func TestRequireAnyRoleMiddleware(t *testing.T) {
	t.Run("has one of allowed roles", func(t *testing.T) {
		router := gin.New()
		router.Use(func(c *gin.Context) {
			c.Set("claims", jwt.MapClaims{
				"realm_access": map[string]interface{}{
					"roles": []interface{}{"editor"},
				},
			})
			c.Next()
		})
		router.Use(RequireAnyRole("admin", "editor"))
		router.GET("/test", func(c *gin.Context) { c.String(200, "ok") })

		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/test", nil)
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("has none of allowed roles", func(t *testing.T) {
		router := gin.New()
		router.Use(func(c *gin.Context) {
			c.Set("claims", jwt.MapClaims{
				"realm_access": map[string]interface{}{
					"roles": []interface{}{"viewer"},
				},
			})
			c.Next()
		})
		router.Use(RequireAnyRole("admin", "editor"))
		router.GET("/test", func(c *gin.Context) { c.String(200, "ok") })

		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/test", nil)
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusForbidden, w.Code)
	})
}
