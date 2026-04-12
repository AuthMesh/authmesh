package e2e

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AuthMesh/authmesh/tests/testutils"
)

// TestUnifiedAPI_BasicFunctionality tests the core functionality of the unified API
func TestUnifiedAPI_BasicFunctionality(t *testing.T) {
	appURL := testutils.GetAppURL()
	require.NotEmpty(t, appURL, "APP_URL must be set")

	// Wait for services to be ready
	require.NoError(t, testutils.WaitForServices(appURL, 60*time.Second), "Services must be ready")

	t.Run("HealthCheck", func(t *testing.T) {
		testHealthCheck(t, appURL)
	})

	t.Run("MetricsEndpoint", func(t *testing.T) {
		testMetricsEndpoint(t, appURL)
	})

	t.Run("PublicEndpoints", func(t *testing.T) {
		testPublicEndpoints(t, appURL)
	})

	t.Run("CORSHeaders", func(t *testing.T) {
		testCORSHeaders(t, appURL)
	})

	t.Run("SecurityHeaders", func(t *testing.T) {
		testSecurityHeaders(t, appURL)
	})
}

func testHealthCheck(t *testing.T, appURL string) {
	resp, err := http.Get(appURL + "/health")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var health map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&health)
	require.NoError(t, err)

	assert.Equal(t, "healthy", health["status"])
	assert.NotEmpty(t, health["timestamp"])
}

func testMetricsEndpoint(t *testing.T, appURL string) {
	resp, err := http.Get(appURL + "/metrics")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "text/plain; version=0.0.4; charset=utf-8", resp.Header.Get("Content-Type"))
}

func testPublicEndpoints(t *testing.T, appURL string) {
	// Test the main public endpoint
	resp, err := http.Get(appURL + "/")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var response map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&response)
	require.NoError(t, err)

	assert.Contains(t, response, "message")
}

func testCORSHeaders(t *testing.T, appURL string) {
	// Make a CORS preflight request
	req, err := http.NewRequest("OPTIONS", appURL+"/", nil)
	require.NoError(t, err)
	
	req.Header.Set("Origin", "http://localhost:3000")
	req.Header.Set("Access-Control-Request-Method", "GET")
	req.Header.Set("Access-Control-Request-Headers", "Authorization")

	client := &http.Client{}
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusNoContent, resp.StatusCode)
	assert.NotEmpty(t, resp.Header.Get("Access-Control-Allow-Origin"))
	assert.NotEmpty(t, resp.Header.Get("Access-Control-Allow-Methods"))
}

func testSecurityHeaders(t *testing.T, appURL string) {
	resp, err := http.Get(appURL + "/")
	require.NoError(t, err)
	defer resp.Body.Close()

	// Check for security headers
	assert.NotEmpty(t, resp.Header.Get("X-Request-Id"), "X-Request-Id should be present")
	
	// Check for standard security headers if enabled
	securityHeaders := []string{
		"X-Content-Type-Options",
		"X-Frame-Options",
		"X-XSS-Protection",
	}

	for _, header := range securityHeaders {
		// Headers might be present depending on configuration
		// We just verify the middleware is working, headers are optional
		t.Logf("Security header %s: %s", header, resp.Header.Get(header))
	}
}
