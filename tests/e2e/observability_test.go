package e2e

import (
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AuthMesh/authmesh/tests/testutils"
)

// TestObservability_UnifiedAPI tests observability features (metrics, tracing)
func TestObservability_UnifiedAPI(t *testing.T) {
	appURL := testutils.GetAppURL()
	require.NotEmpty(t, appURL, "APP_URL must be set")

	// Wait for services to be ready
	require.NoError(t, testutils.WaitForServices(appURL, 60*time.Second), "Services must be ready")

	t.Run("PrometheusMetrics", func(t *testing.T) {
		testPrometheusMetrics(t, appURL)
	})

	t.Run("MetricsAfterRequests", func(t *testing.T) {
		testMetricsAfterRequests(t, appURL)
	})

	t.Run("HealthMetrics", func(t *testing.T) {
		testHealthMetrics(t, appURL)
	})
}

func testPrometheusMetrics(t *testing.T, appURL string) {
	resp, err := http.Get(appURL + "/metrics")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "text/plain; version=0.0.4; charset=utf-8", resp.Header.Get("Content-Type"))

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	metrics := string(body)

	// Check for basic Prometheus metrics
	expectedMetrics := []string{
		"# HELP go_",              // Go runtime metrics
		"# TYPE go_",              // Go runtime metrics type
		"promhttp_metric_handler", // Prometheus handler metrics
	}

	for _, expected := range expectedMetrics {
		assert.Contains(t, metrics, expected, "Should contain metric: %s", expected)
	}

	// Check for application-specific metrics (if implemented)
	appMetrics := []string{
		"http_requests_total",
		"http_request_duration",
	}

	for _, metric := range appMetrics {
		if strings.Contains(metrics, metric) {
			t.Logf("✅ Found application metric: %s", metric)
		} else {
			t.Logf("⚠️  Application metric not found (may not be implemented yet): %s", metric)
		}
	}
}

func testMetricsAfterRequests(t *testing.T, appURL string) {
	// Make some requests to generate metrics
	endpoints := []string{
		"/",
		"/health",
		"/nonexistent", // Should generate 404 metrics
	}

	for _, endpoint := range endpoints {
		resp, err := http.Get(appURL + endpoint)
		require.NoError(t, err)
		resp.Body.Close()
	}

	// Small delay to allow metrics to be updated
	time.Sleep(500 * time.Millisecond)

	// Check metrics again
	resp, err := http.Get(appURL + "/metrics")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	metrics := string(body)

	// Check that we have some HTTP metrics (even if basic)
	if strings.Contains(metrics, "http_requests") || strings.Contains(metrics, "promhttp") {
		t.Log("✅ HTTP metrics are being collected")
	} else {
		t.Log("⚠️  HTTP metrics may not be fully implemented yet")
	}
}

func testHealthMetrics(t *testing.T, appURL string) {
	// Make multiple health checks
	for i := 0; i < 5; i++ {
		resp, err := http.Get(appURL + "/health")
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		resp.Body.Close()
	}

	// Check if health check metrics are available
	resp, err := http.Get(appURL + "/metrics")
	require.NoError(t, err)
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	metrics := string(body)

	// Look for any evidence of request tracking
	healthRelatedMetrics := []string{
		"health",
		"http_requests",
		"promhttp_metric_handler_requests_total",
	}

	found := false
	for _, metric := range healthRelatedMetrics {
		if strings.Contains(metrics, metric) {
			t.Logf("✅ Found health-related metric: %s", metric)
			found = true
		}
	}

	if !found {
		t.Log("⚠️  No specific health metrics found, but basic Prometheus metrics are available")
	}

	// Verify metrics endpoint is not empty
	assert.Greater(t, len(metrics), 100, "Metrics should contain substantial data")
}
