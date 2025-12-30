package platform

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	assert.Equal(t, "http://localhost:9443", config.Keycloak.URL)
	assert.Equal(t, "master", config.Keycloak.Realm)
	assert.Equal(t, "redis://localhost:6379", config.Redis.URL)
	assert.Equal(t, 0, config.Redis.DB)
	assert.True(t, config.Security.EnableSSRFProtection)
	assert.True(t, config.Security.BlockPrivateIPs)
	assert.True(t, config.Security.EnableSecurityHeaders)
	assert.Contains(t, config.CORS.AllowOrigins, "http://localhost:3000")
	assert.Contains(t, config.CORS.AllowMethods, "GET")
	assert.Contains(t, config.CORS.AllowMethods, "POST")
	assert.True(t, config.RateLimit.Enabled)
	assert.Equal(t, 100.0, config.RateLimit.RequestsPerSecond)
	assert.Equal(t, 200, config.RateLimit.BurstSize)
	assert.True(t, config.Observability.EnableMetrics)
	assert.True(t, config.Routes.EnableStandardHealthRoutes)
	assert.True(t, config.Routes.EnableRootWelcomeRoute)
}

func TestNewForTesting(t *testing.T) {
	serviceName := "test-service"
	platform, err := NewForTesting(serviceName)

	assert.NoError(t, err)
	assert.NotNil(t, platform)
	assert.Equal(t, serviceName, platform.config.Observability.AppName)
	assert.True(t, platform.config.Keycloak.SkipJWKSInit)
	assert.False(t, platform.config.Observability.EnableMetrics) // NewForTesting disables metrics
}

func TestNewWithDefaults(t *testing.T) {
	t.Skip("Skipping NewWithDefaults test - requires external network connections")

	// This test would require real Keycloak and Redis servers
	// Uncomment to test with real services:
	// keycloakURL := "https://keycloak.test.com"
	// redisURL := "redis://redis.test.com:6379"
	// platform, err := NewWithDefaults(keycloakURL, redisURL)
	// assert.Error(t, err) // Expected to fail without real services
}

func TestPlatform_GetConfig(t *testing.T) {
	platform, err := NewForTesting("test-service")
	require.NoError(t, err)

	config := platform.GetConfig()
	assert.Equal(t, platform.config, config)
}

func TestPlatform_GetJWKSRegistry(t *testing.T) {
	platform, err := NewForTesting("test-service")
	require.NoError(t, err)

	registry := platform.GetJWKSRegistry()
	assert.NotNil(t, registry)
}

func TestPlatform_GetRedisClient(t *testing.T) {
	// Test with no Redis configuration
	platform, err := NewForTesting("test-service")
	require.NoError(t, err)

	client := platform.GetRedisClient()
	assert.Nil(t, client) // Should be nil for testing config
}

func TestPlatform_Shutdown(t *testing.T) {
	platform, err := NewForTesting("test-service")
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = platform.Shutdown(ctx)
	assert.NoError(t, err)
}

func TestPlatform_SetupMiddleware(t *testing.T) {
	platform, err := NewForTesting("test-service")
	require.NoError(t, err)

	gin.SetMode(gin.TestMode)
	router := gin.New()

	platform.SetupMiddleware(router)

	// Test that middleware is set up by making a request
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	req, err := http.NewRequest("GET", "/test", nil)
	require.NoError(t, err)

	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestPlatform_SetupRoutes(t *testing.T) {
	platform, err := NewForTesting("test-service")
	require.NoError(t, err)

	gin.SetMode(gin.TestMode)
	router := gin.New()

	platform.SetupRoutes(router)

	// Test health endpoints
	healthEndpoints := []string{"/health", "/healthz", "/ready", "/readyz"}
	for _, endpoint := range healthEndpoints {
		t.Run("health_endpoint_"+endpoint, func(t *testing.T) {
			req, err := http.NewRequest("GET", endpoint, nil)
			require.NoError(t, err)

			rr := httptest.NewRecorder()
			router.ServeHTTP(rr, req)

			assert.Equal(t, http.StatusOK, rr.Code)
		})
	}

	// Test root welcome route
	t.Run("root_welcome_route", func(t *testing.T) {
		req, err := http.NewRequest("GET", "/", nil)
		require.NoError(t, err)

		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)

		var response map[string]interface{}
		err = json.Unmarshal(rr.Body.Bytes(), &response)
		assert.NoError(t, err)
		assert.Contains(t, response, "message")
		assert.Contains(t, response, "version")
		assert.Contains(t, response, "powered_by")
	})

	// Test metrics endpoint
	t.Run("metrics_endpoint", func(t *testing.T) {
		req, err := http.NewRequest("GET", "/metrics", nil)
		require.NoError(t, err)

		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		// Check for 200 OR 404 (since metrics might not be set up in test mode)
		assert.True(t, rr.Code == http.StatusOK || rr.Code == http.StatusNotFound)
		if rr.Code == http.StatusOK {
			assert.Contains(t, rr.Header().Get("Content-Type"), "text/plain")
		}
	})
}

func TestPlatform_SetupAll(t *testing.T) {
	platform, err := NewForTesting("test-service")
	require.NoError(t, err)

	gin.SetMode(gin.TestMode)
	router := gin.New()

	platform.SetupAll(router)

	// Test that both middleware and routes are set up
	req, err := http.NewRequest("GET", "/health", nil)
	require.NoError(t, err)

	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestTracingConfig_Validation(t *testing.T) {
	tests := []struct {
		name   string
		config TracingConfig
		valid  bool
	}{
		{
			name: "valid config",
			config: TracingConfig{
				ServiceName:    "test-service",
				ServiceVersion: "1.0.0",
				Environment:    "test",
				JaegerURL:      "http://localhost:14268/api/traces",
				OTLPEndpoint:   "http://localhost:4317",
				SampleRate:     0.1,
			},
			valid: true,
		},
		{
			name: "empty service name",
			config: TracingConfig{
				ServiceVersion: "1.0.0",
				Environment:    "test",
				SampleRate:     0.1,
			},
			valid: true, // Should still be valid, defaults will be used
		},
		{
			name: "zero sample rate",
			config: TracingConfig{
				ServiceName:    "test-service",
				ServiceVersion: "1.0.0",
				Environment:    "test",
				SampleRate:     0.0,
			},
			valid: true,
		},
		{
			name: "max sample rate",
			config: TracingConfig{
				ServiceName:    "test-service",
				ServiceVersion: "1.0.0",
				Environment:    "test",
				SampleRate:     1.0,
			},
			valid: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test that the config can be used to create a platform
			cfg := DefaultConfig()
			cfg.Observability.EnableTracing = true
			cfg.Observability.TracingConfig = tt.config
			cfg.Keycloak.SkipJWKSInit = true

			platform, err := New(cfg)
			if tt.valid {
				assert.NoError(t, err)
				assert.NotNil(t, platform)
			} else {
				// Note: Current implementation doesn't validate tracing config
				// This could be enhanced in the future
				assert.NoError(t, err)
				assert.NotNil(t, platform)
			}
		})
	}
}

func TestConfig_Validation(t *testing.T) {
	tests := []struct {
		name     string
		config   Config
		valid    bool
		errorMsg string
	}{
		{
			name: "default config with JWKS skipped",
			config: func() Config {
				cfg := DefaultConfig()
				cfg.Keycloak.SkipJWKSInit = true // Skip JWKS to avoid network calls
				return cfg
			}(),
			valid: true,
		},
		{
			name: "empty keycloak URL",
			config: Config{
				Keycloak: KeycloakConfig{
					URL:          "",
					Realm:        "master",
					SkipJWKSInit: true,
				},
				Redis: RedisConfig{
					URL: "redis://localhost:6379",
				},
				Observability: ObservabilityConfig{
					EnableMetrics: true,
					AppName:       "test",
					AppVersion:    "1.0.0",
				},
			},
			valid: true, // Should be valid, JWKS init will be skipped
		},
		{
			name: "minimal config",
			config: Config{
				Keycloak: KeycloakConfig{
					SkipJWKSInit: true,
				},
				Observability: ObservabilityConfig{
					EnableMetrics: false,
				},
			},
			valid: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			platform, err := New(tt.config)

			if tt.valid {
				assert.NoError(t, err)
				assert.NotNil(t, platform)

				// Test shutdown
				ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
				defer cancel()
				platform.Shutdown(ctx)
			} else {
				assert.Error(t, err)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
			}
		})
	}
}

func TestPlatform_HealthCheckIntegration(t *testing.T) {
	platform, err := NewForTesting("health-test-service")
	require.NoError(t, err)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	platform.SetupAll(router)

	// Test comprehensive health check
	req, err := http.NewRequest("GET", "/health", nil)
	require.NoError(t, err)

	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var healthResponse map[string]interface{}
	err = json.Unmarshal(rr.Body.Bytes(), &healthResponse)
	require.NoError(t, err)

	// Verify health response structure
	assert.Contains(t, healthResponse, "status")
	assert.Contains(t, healthResponse, "timestamp")
	assert.Equal(t, "healthy", healthResponse["status"])
	// Note: service and version are not included in basic health check
}

func TestCORSConfig_Validation(t *testing.T) {
	tests := []struct {
		name       string
		corsConfig CORSConfig
		valid      bool
	}{
		{
			name: "valid CORS config",
			corsConfig: CORSConfig{
				AllowOrigins:     []string{"https://example.com", "https://app.example.com"},
				AllowMethods:     []string{"GET", "POST", "PUT", "DELETE"},
				AllowHeaders:     []string{"Content-Type", "Authorization"},
				AllowCredentials: true,
				MaxAge:           time.Hour,
			},
			valid: true,
		},
		{
			name: "wildcard origin",
			corsConfig: CORSConfig{
				AllowOrigins: []string{"*"},
				AllowMethods: []string{"GET", "POST"},
				AllowHeaders: []string{"Content-Type"},
			},
			valid: true,
		},
		{
			name: "empty origins",
			corsConfig: CORSConfig{
				AllowOrigins: []string{},
				AllowMethods: []string{"GET"},
			},
			valid: true, // Should use defaults
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := DefaultConfig()
			config.CORS = tt.corsConfig
			config.Keycloak.SkipJWKSInit = true

			platform, err := New(config)
			assert.NoError(t, err)
			assert.NotNil(t, platform)

			// Test CORS setup
			gin.SetMode(gin.TestMode)
			router := gin.New()
			platform.SetupMiddleware(router)

			router.GET("/test", func(c *gin.Context) {
				c.JSON(http.StatusOK, gin.H{"status": "ok"})
			})

			// Test OPTIONS request (CORS preflight)
			req, err := http.NewRequest("OPTIONS", "/test", nil)
			require.NoError(t, err)
			req.Header.Set("Origin", "https://example.com")
			req.Header.Set("Access-Control-Request-Method", "GET")

			rr := httptest.NewRecorder()
			router.ServeHTTP(rr, req)

			// Should handle CORS preflight
			assert.Contains(t, []int{http.StatusOK, http.StatusNoContent}, rr.Code)
		})
	}
}

func TestPlatform_Metrics_Integration(t *testing.T) {
	// Create a config with metrics enabled
	config := DefaultConfig()
	config.Keycloak.SkipJWKSInit = true
	config.Observability.EnableMetrics = true
	config.Observability.AppName = "metrics-test-service"
	config.Redis.URL = "" // No Redis to keep test simple

	platform, err := New(config)
	require.NoError(t, err)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	platform.SetupAll(router)

	// Make some requests to generate metrics
	endpoints := []string{"/health", "/healthz", "/ready", "/"}
	for _, endpoint := range endpoints {
		req, err := http.NewRequest("GET", endpoint, nil)
		require.NoError(t, err)

		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
	}

	// Check metrics endpoint
	req, err := http.NewRequest("GET", "/metrics", nil)
	require.NoError(t, err)

	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	body := rr.Body.String()

	// Verify expected metrics are present
	assert.Contains(t, body, "go_goroutines", "Go runtime metrics should be present")
	assert.Contains(t, body, "app_info", "Application info metrics should be present")
}

// Benchmark tests
func BenchmarkNewForTesting(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		platform, err := NewForTesting("benchmark-service")
		if err != nil {
			b.Fatal(err)
		}
		platform.Shutdown(context.Background())
	}
}

func BenchmarkPlatform_HealthCheck(b *testing.B) {
	platform, err := NewForTesting("benchmark-health-service")
	if err != nil {
		b.Fatal(err)
	}

	gin.SetMode(gin.TestMode)
	router := gin.New()
	platform.SetupAll(router)

	req, _ := http.NewRequest("GET", "/health", nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			b.Fatal("Expected 200, got", rr.Code)
		}
	}
}
