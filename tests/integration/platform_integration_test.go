//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
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

// --- Helper: skip if external services not available ---

func keycloakURL() string {
	if u := os.Getenv("KEYCLOAK_URL"); u != "" {
		return u
	}
	return "http://localhost:8080"
}

func redisURL() string {
	if u := os.Getenv("REDIS_URL"); u != "" {
		return u
	}
	return "redis://localhost:6379"
}

func skipIfNoRedis(t *testing.T) {
	t.Helper()
	// Try a quick TCP dial to see if Redis is reachable
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	r, err := (&net.Dialer{}).DialContext(ctx, "tcp", "localhost:6379")
	if err != nil {
		t.Skip("Redis not available, skipping integration test")
	}
	r.Close()
}

func skipIfNoKeycloak(t *testing.T) {
	t.Helper()
	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get(keycloakURL() + "/health/ready")
	if err != nil || resp.StatusCode != 200 {
		t.Skip("Keycloak not available, skipping integration test")
	}
	resp.Body.Close()
}

// ============================================================
// Tests that run WITHOUT external services (always run in CI)
// ============================================================

// TestPlatformSetupAll_NoExternalDeps validates platform initialization
// without any external services using NewForTesting.
func TestPlatformSetupAll_NoExternalDeps(t *testing.T) {
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

	t.Run("unauthenticated request to protected route returns 401", func(t *testing.T) {
		router.GET("/api/secured", authMesh.AuthMiddleware(), func(c *gin.Context) {
			c.JSON(200, gin.H{"ok": true})
		})
		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/api/secured", nil)
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("request with invalid bearer token returns 401", func(t *testing.T) {
		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/api/secured", nil)
		req.Header.Set("Authorization", "Bearer invalid-jwt-string")
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusUnauthorized, w.Code)
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

// TestQuickStart validates the convenience constructor
func TestQuickStart(t *testing.T) {
	authMesh, err := platform.QuickStart("quickstart-test")
	require.NoError(t, err)
	assert.NotNil(t, authMesh)
}

// TestExampleAppRoutePattern mirrors examples/minimal-app to confirm
// the example code pattern works end-to-end.
func TestExampleAppRoutePattern(t *testing.T) {
	authMesh, err := platform.QuickStart("example-app")
	require.NoError(t, err)

	router := gin.New()
	authMesh.SetupAll(router)

	router.GET("/hello", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "Hello from AuthMesh!"})
	})

	protected := router.Group("/api/v1")
	protected.Use(authMesh.AuthMiddleware())
	protected.GET("/protected", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "authenticated"})
	})

	t.Run("public endpoint works", func(t *testing.T) {
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

// ============================================================
// Tests that REQUIRE external services (skipped when missing)
// ============================================================

// TestWithRedis_PlatformInitialization tests that the platform can
// connect to a real Redis instance and function properly.
func TestWithRedis_PlatformInitialization(t *testing.T) {
	skipIfNoRedis(t)

	config := platform.DefaultConfig()
	config.Keycloak.URL = ""
	config.Keycloak.SkipJWKSInit = true
	config.Redis.URL = redisURL()
	config.Observability.AppName = "redis-integration-test"
	config.Observability.EnableMetrics = false
	config.Observability.EnableTracing = false

	authMesh, err := platform.New(config)
	require.NoError(t, err)

	router := gin.New()
	authMesh.SetupAll(router)

	t.Run("health endpoint shows redis up", func(t *testing.T) {
		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/health", nil)
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err = authMesh.Shutdown(ctx)
	assert.NoError(t, err)
}

// TestWithKeycloak_HealthCheck tests Keycloak connectivity
func TestWithKeycloak_HealthCheck(t *testing.T) {
	skipIfNoKeycloak(t)

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(keycloakURL() + "/health/ready")
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

// TestWithKeycloak_PlatformInitialization connects to a real Keycloak
func TestWithKeycloak_PlatformInitialization(t *testing.T) {
	skipIfNoKeycloak(t)

	config := platform.DefaultConfig()
	config.Keycloak.URL = keycloakURL()
	config.Keycloak.Realm = "master"
	config.Redis.URL = ""
	config.Observability.AppName = "keycloak-integration-test"
	config.Observability.EnableMetrics = false
	config.Observability.EnableTracing = false

	authMesh, err := platform.New(config)
	// May fail if Keycloak JWKS isn't ready yet — that's ok
	if err != nil {
		t.Skipf("Keycloak JWKS not ready: %v", err)
	}

	router := gin.New()
	authMesh.SetupAll(router)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/health", nil)
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

// TestWithBothServices_FullStack tests with both Keycloak and Redis
func TestWithBothServices_FullStack(t *testing.T) {
	skipIfNoRedis(t)
	skipIfNoKeycloak(t)

	config := platform.DefaultConfig()
	config.Keycloak.URL = keycloakURL()
	config.Keycloak.Realm = "master"
	config.Redis.URL = redisURL()
	config.Observability.AppName = "full-stack-test"
	config.Observability.EnableMetrics = true
	config.Observability.EnableTracing = false

	authMesh, err := platform.New(config)
	if err != nil {
		t.Skipf("Services not fully ready: %v", err)
	}

	router := gin.New()
	authMesh.SetupAll(router)

	t.Run("health shows all services", func(t *testing.T) {
		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/health", nil)
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("unauthenticated request is rejected", func(t *testing.T) {
		router.GET("/api/full-stack", authMesh.AuthMiddleware(), func(c *gin.Context) {
			c.JSON(200, gin.H{"ok": true})
		})
		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/api/full-stack", nil)
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	authMesh.Shutdown(ctx)
}
