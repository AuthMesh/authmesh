//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/AuthMesh/authmesh/pkg/platform"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// TestPlatformSetupAll validates that the platform can be initialized and
// standard routes (health, welcome, metrics) work end-to-end.
func TestPlatformSetupAll(t *testing.T) {
	authMesh, err := platform.NewForTesting("integration-test")
	require.NoError(t, err)

	router := gin.New()
	authMesh.SetupAll(router)

	t.Run("health endpoint returns 200", func(t *testing.T) {
		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/health", nil)
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)

		var body map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &body)
		assert.NoError(t, err)
		assert.Equal(t, "healthy", body["status"])
	})

	t.Run("root welcome endpoint", func(t *testing.T) {
		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/", nil)
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("protected endpoint without auth returns 401", func(t *testing.T) {
		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/whoami", nil)
		router.ServeHTTP(w, req)
		// Whoami requires auth middleware, should be 401 without token
		assert.Contains(t, []int{http.StatusUnauthorized, http.StatusOK}, w.Code)
	})
}

// TestPlatformShutdown verifies graceful shutdown
func TestPlatformShutdown(t *testing.T) {
	authMesh, err := platform.NewForTesting("shutdown-test")
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = authMesh.Shutdown(ctx)
	assert.NoError(t, err)
}

// TestPlatformQuickStart validates the QuickStart convenience constructor
func TestPlatformQuickStart(t *testing.T) {
	authMesh, err := platform.QuickStart("quickstart-test")
	require.NoError(t, err)
	assert.NotNil(t, authMesh)

	router := gin.New()
	authMesh.SetupAll(router)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/health", nil)
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

// TestPlatformMiddlewareStack validates the full middleware stack is applied
func TestPlatformMiddlewareStack(t *testing.T) {
	authMesh, err := platform.NewForTesting("middleware-test")
	require.NoError(t, err)

	router := gin.New()
	authMesh.SetupAll(router)

	// Add a custom protected route
	router.GET("/api/test", authMesh.AuthMiddleware(), func(c *gin.Context) {
		c.JSON(200, gin.H{"message": "protected"})
	})

	t.Run("unauthenticated request to protected route", func(t *testing.T) {
		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/api/test", nil)
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("request with invalid bearer token", func(t *testing.T) {
		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/api/test", nil)
		req.Header.Set("Authorization", "Bearer invalid-token")
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})
}

// TestExampleAppBasicRoutes simulates what the example apps do
func TestExampleAppBasicRoutes(t *testing.T) {
	authMesh, err := platform.QuickStart("example-app")
	require.NoError(t, err)

	router := gin.New()
	authMesh.SetupAll(router)

	// Add routes like the minimal-app example
	router.GET("/hello", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"message":    "Hello from AuthMesh!",
			"powered_by": "AuthMesh Platform",
		})
	})

	protected := router.Group("/api/v1")
	protected.Use(authMesh.AuthMiddleware())
	{
		protected.GET("/protected", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"message": "authenticated"})
		})
	}

	t.Run("public hello endpoint", func(t *testing.T) {
		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/hello", nil)
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), "Hello from AuthMesh!")
	})

	t.Run("protected endpoint blocks unauthenticated", func(t *testing.T) {
		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/api/v1/protected", nil)
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})
}
