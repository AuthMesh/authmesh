package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v4"
	"github.com/stretchr/testify/assert"
)

func TestTenantMiddleware(t *testing.T) {
	t.Run("no claims - unauthorized", func(t *testing.T) {
		router := gin.New()
		router.Use(TenantMiddleware())
		router.GET("/t/:tenant_id/data", func(c *gin.Context) { c.String(200, "ok") })

		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/t/tenant1/data", nil)
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("invalid claims type", func(t *testing.T) {
		router := gin.New()
		router.Use(func(c *gin.Context) {
			c.Set("claims", "not-jwt-claims")
			c.Next()
		})
		router.Use(TenantMiddleware())
		router.GET("/t/:tenant_id/data", func(c *gin.Context) { c.String(200, "ok") })

		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/t/tenant1/data", nil)
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("matching tenant - allowed", func(t *testing.T) {
		router := gin.New()
		router.Use(func(c *gin.Context) {
			c.Set("claims", jwt.MapClaims{"tenant_id": "tenant1"})
			c.Next()
		})
		router.Use(TenantMiddleware())
		router.GET("/t/:tenant_id/data", func(c *gin.Context) { c.String(200, "ok") })

		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/t/tenant1/data", nil)
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("mismatched tenant - forbidden", func(t *testing.T) {
		router := gin.New()
		router.Use(func(c *gin.Context) {
			c.Set("claims", jwt.MapClaims{"tenant_id": "tenant1"})
			c.Next()
		})
		router.Use(TenantMiddleware())
		router.GET("/t/:tenant_id/data", func(c *gin.Context) { c.String(200, "ok") })

		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/t/tenant2/data", nil)
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusForbidden, w.Code)
	})

	t.Run("tenant from issuer claim", func(t *testing.T) {
		router := gin.New()
		router.Use(func(c *gin.Context) {
			c.Set("claims", jwt.MapClaims{
				"iss": "http://keycloak:8080/realms/tenant1",
			})
			c.Next()
		})
		router.Use(TenantMiddleware())
		router.GET("/t/:tenant_id/data", func(c *gin.Context) { c.String(200, "ok") })

		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/t/tenant1/data", nil)
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("no tenant_id in path - sets context", func(t *testing.T) {
		router := gin.New()
		router.Use(func(c *gin.Context) {
			c.Set("claims", jwt.MapClaims{"tenant_id": "tenant1"})
			c.Next()
		})
		router.Use(TenantMiddleware())
		router.GET("/general", func(c *gin.Context) {
			tid, _ := c.Get("tenant_id")
			c.String(200, tid.(string))
		})

		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/general", nil)
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "tenant1", w.Body.String())
	})

	t.Run("no tenant_id anywhere - bad request", func(t *testing.T) {
		router := gin.New()
		router.Use(func(c *gin.Context) {
			c.Set("claims", jwt.MapClaims{"sub": "user-1"})
			c.Next()
		})
		router.Use(TenantMiddleware())
		router.GET("/data", func(c *gin.Context) { c.String(200, "ok") })

		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/data", nil)
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}

func TestWhoAmI(t *testing.T) {
	t.Run("authenticated user", func(t *testing.T) {
		router := gin.New()
		router.Use(func(c *gin.Context) {
			c.Set("user_info", map[string]interface{}{
				"sub":       "user-123",
				"username":  "testuser",
				"tenant_id": "tenant-1",
			})
			c.Set("claims", jwt.MapClaims{
				"sub":       "user-123",
				"tenant_id": "tenant-1",
				"realm_access": map[string]interface{}{
					"roles": []interface{}{"admin"},
				},
			})
			c.Set("tenant_id", "tenant-1")
			c.Next()
		})
		router.GET("/whoami", WhoAmI)

		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/whoami", nil)
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), "user-123")
	})

	t.Run("unauthenticated user", func(t *testing.T) {
		router := gin.New()
		router.GET("/whoami", WhoAmI)

		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/whoami", nil)
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})
}

func TestWhoAmIHandler(t *testing.T) {
	// WhoAmIHandler is an alias for WhoAmI
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("user_info", map[string]interface{}{
			"sub":      "user-1",
			"username": "test",
		})
		c.Set("claims", jwt.MapClaims{"sub": "user-1"})
		c.Next()
	})
	router.GET("/whoami", WhoAmIHandler)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/whoami", nil)
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}
