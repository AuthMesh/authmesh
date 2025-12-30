package config

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLoad(t *testing.T) {
	// Save original environment
	originalEnv := map[string]string{}
	envVars := []string{
		"PORT", "KEYCLOAK_URL", "REALM", "FRONTEND_URL", "CORS_ALLOW_ORIGINS",
		"CORS_ALLOW_METHODS", "CORS_ALLOW_HEADERS", "CORS_MAX_AGE",
		"RATE_LIMIT_PER_SECOND", "RATE_LIMIT_BURST", "RATE_LIMIT_TENANT_PER_SECOND",
		"RATE_LIMIT_TENANT_BURST", "RATE_LIMIT_MESSAGE", "TRUSTED_JWT_ISSUERS",
		"ADMIN_ROLES", "READ_ONLY_ROLES", "WRITE_ROLES", "REDIS_URL",
		"RATE_LIMIT", "REFRESH_TOKEN_TTL", "OTEL_COLLECTOR_URL",
	}

	for _, key := range envVars {
		originalEnv[key] = os.Getenv(key)
		os.Unsetenv(key)
	}
	defer func() {
		for key, value := range originalEnv {
			if value != "" {
				os.Setenv(key, value)
			}
		}
	}()

	tests := []struct {
		name     string
		envVars  map[string]string
		expected *Config
	}{
		{
			name:    "default configuration",
			envVars: map[string]string{},
			expected: &Config{
				Port:        "8081",
				KeycloakURL: "https://localhost:9443",
				Realm:       "tenant1",
				CORSAllowOrigins: []string{
					"http://localhost:3000",
					"http://localhost:3000",
					"http://localhost:3001",
				},
				CORSAllowMethods: []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
				CORSAllowHeaders: []string{
					"Origin", "Content-Type", "Accept", "Authorization", "X-Request-ID",
					"X-Tenant-ID", "Content-Length", "Accept-Encoding", "X-CSRF-Token",
				},
				CORSMaxAge:               "86400",
				RateLimitPerSecond:       10.0,
				RateLimitBurst:           20,
				RateLimitTenantPerSecond: 5000.0,
				RateLimitTenantBurst:     5500,
				RateLimitMessage:         "Rate limit exceeded. Please try again later.",
				TrustedIssuers: []string{
					"https://localhost:9443",
					"https://localhost:9443",
					"https://localhost:9444",
				},
				FrontendURL:      "http://localhost:3000",
				AdminRoles:       []string{"admin", "super-admin", "platform-admin"},
				ReadOnlyRoles:    []string{"viewer", "read-only", "tenant-user"},
				WriteRoles:       []string{"admin", "tenant-admin", "platform-admin"},
				RedisURL:         "redis://localhost:6379",
				RateLimit:        "10:60",
				RefreshTokenTTL:  "3600",
				OTelCollectorURL: "http://localhost:4317",
			},
		},
		{
			name: "custom configuration",
			envVars: map[string]string{
				"PORT":                         "9090",
				"KEYCLOAK_URL":                 "https://keycloak.example.com",
				"REALM":                        "custom-realm",
				"FRONTEND_URL":                 "https://app.example.com",
				"CORS_ALLOW_ORIGINS":           "https://app.example.com,https://admin.example.com",
				"CORS_ALLOW_METHODS":           "GET,POST,PUT,DELETE",
				"CORS_ALLOW_HEADERS":           "Content-Type,Authorization",
				"CORS_MAX_AGE":                 "3600",
				"RATE_LIMIT_PER_SECOND":        "50.5",
				"RATE_LIMIT_BURST":             "100",
				"RATE_LIMIT_TENANT_PER_SECOND": "1000.0",
				"RATE_LIMIT_TENANT_BURST":      "1200",
				"RATE_LIMIT_MESSAGE":           "Custom rate limit message",
				"TRUSTED_JWT_ISSUERS":          "https://keycloak.example.com,https://auth.example.com",
				"ADMIN_ROLES":                  "admin,superuser",
				"READ_ONLY_ROLES":              "viewer,guest",
				"WRITE_ROLES":                  "editor,admin",
				"REDIS_URL":                    "redis://redis.example.com:6380",
				"RATE_LIMIT":                   "100:3600",
				"REFRESH_TOKEN_TTL":            "7200",
				"OTEL_COLLECTOR_URL":           "http://otel.example.com:4318",
			},
			expected: &Config{
				Port:        "9090",
				KeycloakURL: "https://keycloak.example.com",
				Realm:       "custom-realm",
				CORSAllowOrigins: []string{
					"https://app.example.com",
					"https://admin.example.com",
				},
				CORSAllowMethods:         []string{"GET", "POST", "PUT", "DELETE"},
				CORSAllowHeaders:         []string{"Content-Type", "Authorization"},
				CORSMaxAge:               "3600",
				RateLimitPerSecond:       50.5,
				RateLimitBurst:           100,
				RateLimitTenantPerSecond: 1000.0,
				RateLimitTenantBurst:     1200,
				RateLimitMessage:         "Custom rate limit message",
				TrustedIssuers: []string{
					"https://keycloak.example.com",
					"https://auth.example.com",
				},
				FrontendURL:      "https://app.example.com",
				AdminRoles:       []string{"admin", "superuser"},
				ReadOnlyRoles:    []string{"viewer", "guest"},
				WriteRoles:       []string{"editor", "admin"},
				RedisURL:         "redis://redis.example.com:6380",
				RateLimit:        "100:3600",
				RefreshTokenTTL:  "7200",
				OTelCollectorURL: "http://otel.example.com:4318",
			},
		},
		{
			name: "invalid numeric values fall back to defaults",
			envVars: map[string]string{
				"RATE_LIMIT_PER_SECOND":        "invalid",
				"RATE_LIMIT_BURST":             "not-a-number",
				"RATE_LIMIT_TENANT_PER_SECOND": "also-invalid",
				"RATE_LIMIT_TENANT_BURST":      "non-numeric",
			},
			expected: &Config{
				Port:                     "8081",
				KeycloakURL:              "https://localhost:9443",
				Realm:                    "tenant1",
				CORSAllowOrigins:         []string{"http://localhost:3000"},
				CORSAllowMethods:         []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
				CORSAllowHeaders:         []string{"Content-Type", "Authorization"},
				CORSMaxAge:               "86400",
				RateLimitPerSecond:       10.0,   // fallback
				RateLimitBurst:           20,     // fallback
				RateLimitTenantPerSecond: 5000.0, // fallback
				RateLimitTenantBurst:     5500,   // fallback
				TrustedIssuers:           []string{"https://localhost:9443", "https://localhost:9443", "https://localhost:9444"},
				FrontendURL:              "http://localhost:3000",
				AdminRoles:               []string{"admin"},
				ReadOnlyRoles:            []string{"viewer"},
				WriteRoles:               []string{"editor"},
				RateLimitMessage:         "Rate limit exceeded. Please try again later.",
				RedisURL:                 "redis://localhost:6379",
				RateLimit:                "10:60",
				RefreshTokenTTL:          "3600",
				OTelCollectorURL:         "http://localhost:4317",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clean environment
			for _, key := range envVars {
				os.Unsetenv(key)
			}

			// Set test environment variables
			for key, value := range tt.envVars {
				os.Setenv(key, value)
			}

			// Load configuration
			config := Load()

			// Assert specific fields
			assert.Equal(t, tt.expected.Port, config.Port)
			assert.Equal(t, tt.expected.KeycloakURL, config.KeycloakURL)
			assert.Equal(t, tt.expected.Realm, config.Realm)
			assert.Equal(t, tt.expected.FrontendURL, config.FrontendURL)
			assert.Equal(t, tt.expected.CORSMaxAge, config.CORSMaxAge)
			assert.Equal(t, tt.expected.RateLimitPerSecond, config.RateLimitPerSecond)
			assert.Equal(t, tt.expected.RateLimitBurst, config.RateLimitBurst)
			assert.Equal(t, tt.expected.RateLimitTenantPerSecond, config.RateLimitTenantPerSecond)
			assert.Equal(t, tt.expected.RateLimitTenantBurst, config.RateLimitTenantBurst)
			assert.Equal(t, tt.expected.RateLimitMessage, config.RateLimitMessage)
			assert.Equal(t, tt.expected.RedisURL, config.RedisURL)
			assert.Equal(t, tt.expected.RateLimit, config.RateLimit)
			assert.Equal(t, tt.expected.RefreshTokenTTL, config.RefreshTokenTTL)
			assert.Equal(t, tt.expected.OTelCollectorURL, config.OTelCollectorURL)

			// Check slice fields
			if tt.name == "custom configuration" {
				assert.Equal(t, tt.expected.CORSAllowOrigins, config.CORSAllowOrigins)
				assert.Equal(t, tt.expected.CORSAllowMethods, config.CORSAllowMethods)
				assert.Equal(t, tt.expected.CORSAllowHeaders, config.CORSAllowHeaders)
				assert.Equal(t, tt.expected.TrustedIssuers, config.TrustedIssuers)
				assert.Equal(t, tt.expected.AdminRoles, config.AdminRoles)
				assert.Equal(t, tt.expected.ReadOnlyRoles, config.ReadOnlyRoles)
				assert.Equal(t, tt.expected.WriteRoles, config.WriteRoles)
			}
		})
	}
}

func TestGetCORSOrigins(t *testing.T) {
	tests := []struct {
		name        string
		envValue    string
		frontendURL string
		expected    []string
	}{
		{
			name:        "default origins",
			envValue:    "",
			frontendURL: "http://localhost:3000",
			expected:    []string{"http://localhost:3000", "http://localhost:3000", "http://localhost:3001"},
		},
		{
			name:        "custom origins",
			envValue:    "https://app.com,https://admin.com",
			frontendURL: "https://app.com",
			expected:    []string{"https://app.com", "https://admin.com"},
		},
		{
			name:        "single origin",
			envValue:    "https://single.com",
			frontendURL: "https://app.com",
			expected:    []string{"https://single.com"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.envValue != "" {
				os.Setenv("CORS_ALLOW_ORIGINS", tt.envValue)
			} else {
				os.Unsetenv("CORS_ALLOW_ORIGINS")
			}

			result := getCORSOrigins(tt.frontendURL)
			assert.Equal(t, tt.expected, result)

			os.Unsetenv("CORS_ALLOW_ORIGINS")
		})
	}
}

func TestGetCORSMethods(t *testing.T) {
	tests := []struct {
		name     string
		envValue string
		expected []string
	}{
		{
			name:     "default methods",
			envValue: "",
			expected: []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		},
		{
			name:     "custom methods",
			envValue: "GET,POST,PUT",
			expected: []string{"GET", "POST", "PUT"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.envValue != "" {
				os.Setenv("CORS_ALLOW_METHODS", tt.envValue)
			} else {
				os.Unsetenv("CORS_ALLOW_METHODS")
			}

			result := getCORSMethods()
			assert.Equal(t, tt.expected, result)

			os.Unsetenv("CORS_ALLOW_METHODS")
		})
	}
}

func TestGetCORSHeaders(t *testing.T) {
	tests := []struct {
		name     string
		envValue string
		expected []string
	}{
		{
			name:     "default headers",
			envValue: "",
			expected: []string{
				"Origin", "Content-Type", "Accept", "Authorization", "X-Request-ID",
				"X-Tenant-ID", "Content-Length", "Accept-Encoding", "X-CSRF-Token",
			},
		},
		{
			name:     "custom headers",
			envValue: "Content-Type,Authorization",
			expected: []string{"Content-Type", "Authorization"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.envValue != "" {
				os.Setenv("CORS_ALLOW_HEADERS", tt.envValue)
			} else {
				os.Unsetenv("CORS_ALLOW_HEADERS")
			}

			result := getCORSHeaders()
			assert.Equal(t, tt.expected, result)

			os.Unsetenv("CORS_ALLOW_HEADERS")
		})
	}
}

func TestGetRateLimitPerSecond(t *testing.T) {
	tests := []struct {
		name     string
		envValue string
		expected float64
	}{
		{
			name:     "default rate limit",
			envValue: "",
			expected: 10.0,
		},
		{
			name:     "custom rate limit",
			envValue: "25.5",
			expected: 25.5,
		},
		{
			name:     "invalid rate limit",
			envValue: "invalid",
			expected: 10.0, // fallback
		},
		{
			name:     "zero rate limit",
			envValue: "0",
			expected: 0.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.envValue != "" {
				os.Setenv("RATE_LIMIT_PER_SECOND", tt.envValue)
			} else {
				os.Unsetenv("RATE_LIMIT_PER_SECOND")
			}

			result := getRateLimitPerSecond()
			assert.Equal(t, tt.expected, result)

			os.Unsetenv("RATE_LIMIT_PER_SECOND")
		})
	}
}

func TestGetRateLimitBurst(t *testing.T) {
	tests := []struct {
		name     string
		envValue string
		expected int
	}{
		{
			name:     "default burst",
			envValue: "",
			expected: 20,
		},
		{
			name:     "custom burst",
			envValue: "50",
			expected: 50,
		},
		{
			name:     "invalid burst",
			envValue: "invalid",
			expected: 20, // fallback
		},
		{
			name:     "zero burst",
			envValue: "0",
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.envValue != "" {
				os.Setenv("RATE_LIMIT_BURST", tt.envValue)
			} else {
				os.Unsetenv("RATE_LIMIT_BURST")
			}

			result := getRateLimitBurst()
			assert.Equal(t, tt.expected, result)

			os.Unsetenv("RATE_LIMIT_BURST")
		})
	}
}

func TestGetRoles(t *testing.T) {
	tests := []struct {
		name         string
		envKey       string
		envValue     string
		defaultRoles []string
		expected     []string
	}{
		{
			name:         "default roles",
			envKey:       "TEST_ROLES",
			envValue:     "",
			defaultRoles: []string{"role1", "role2"},
			expected:     []string{"role1", "role2"},
		},
		{
			name:         "custom roles",
			envKey:       "TEST_ROLES",
			envValue:     "admin,user,guest",
			defaultRoles: []string{"role1", "role2"},
			expected:     []string{"admin", "user", "guest"},
		},
		{
			name:         "single role",
			envKey:       "TEST_ROLES",
			envValue:     "admin",
			defaultRoles: []string{"role1", "role2"},
			expected:     []string{"admin"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.envValue != "" {
				os.Setenv(tt.envKey, tt.envValue)
			} else {
				os.Unsetenv(tt.envKey)
			}

			result := getRoles(tt.envKey, tt.defaultRoles)
			assert.Equal(t, tt.expected, result)

			os.Unsetenv(tt.envKey)
		})
	}
}

func TestGetTrustedIssuers(t *testing.T) {
	tests := []struct {
		name        string
		envValue    string
		keycloakURL string
		expected    []string
	}{
		{
			name:        "default issuers",
			envValue:    "",
			keycloakURL: "https://auth.example.com",
			expected: []string{
				"https://auth.example.com",
				"https://localhost:9443",
				"https://localhost:9444",
			},
		},
		{
			name:        "custom issuers",
			envValue:    "https://issuer1.com,https://issuer2.com",
			keycloakURL: "https://auth.example.com",
			expected:    []string{"https://issuer1.com", "https://issuer2.com"},
		},
		{
			name:        "custom issuers with whitespace",
			envValue:    " https://issuer1.com , https://issuer2.com ",
			keycloakURL: "https://auth.example.com",
			expected:    []string{"https://issuer1.com", "https://issuer2.com"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.envValue != "" {
				os.Setenv("TRUSTED_JWT_ISSUERS", tt.envValue)
			} else {
				os.Unsetenv("TRUSTED_JWT_ISSUERS")
			}

			result := getTrustedIssuers(tt.keycloakURL)
			assert.Equal(t, tt.expected, result)

			os.Unsetenv("TRUSTED_JWT_ISSUERS")
		})
	}
}

func TestGetRateLimitTenantPerSecond(t *testing.T) {
	tests := []struct {
		name     string
		envValue string
		expected float64
	}{
		{
			name:     "default tenant rate limit",
			envValue: "",
			expected: 5000.0,
		},
		{
			name:     "custom tenant rate limit",
			envValue: "1000.5",
			expected: 1000.5,
		},
		{
			name:     "invalid tenant rate limit",
			envValue: "invalid",
			expected: 5000.0, // fallback
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.envValue != "" {
				os.Setenv("RATE_LIMIT_TENANT_PER_SECOND", tt.envValue)
			} else {
				os.Unsetenv("RATE_LIMIT_TENANT_PER_SECOND")
			}

			result := getRateLimitTenantPerSecond()
			assert.Equal(t, tt.expected, result)

			os.Unsetenv("RATE_LIMIT_TENANT_PER_SECOND")
		})
	}
}

func TestGetRateLimitTenantBurst(t *testing.T) {
	tests := []struct {
		name     string
		envValue string
		expected int
	}{
		{
			name:     "default tenant burst",
			envValue: "",
			expected: 5500,
		},
		{
			name:     "custom tenant burst",
			envValue: "1200",
			expected: 1200,
		},
		{
			name:     "invalid tenant burst",
			envValue: "invalid",
			expected: 5500, // fallback
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.envValue != "" {
				os.Setenv("RATE_LIMIT_TENANT_BURST", tt.envValue)
			} else {
				os.Unsetenv("RATE_LIMIT_TENANT_BURST")
			}

			result := getRateLimitTenantBurst()
			assert.Equal(t, tt.expected, result)

			os.Unsetenv("RATE_LIMIT_TENANT_BURST")
		})
	}
}

// Test edge cases and integration scenarios
func TestConfig_Integration(t *testing.T) {
	// Test with a realistic configuration
	envVars := map[string]string{
		"PORT":                  "8080",
		"KEYCLOAK_URL":          "https://keycloak.prod.com",
		"REALM":                 "production",
		"FRONTEND_URL":          "https://app.prod.com",
		"CORS_ALLOW_ORIGINS":    "https://app.prod.com,https://admin.prod.com",
		"RATE_LIMIT_PER_SECOND": "100.0",
		"TRUSTED_JWT_ISSUERS":   "https://keycloak.prod.com,https://backup-auth.prod.com",
		"ADMIN_ROLES":           "admin,platform-admin",
		"REDIS_URL":             "redis://redis.prod.com:6379",
	}

	// Clean environment first
	allKeys := []string{
		"PORT", "KEYCLOAK_URL", "REALM", "FRONTEND_URL", "CORS_ALLOW_ORIGINS",
		"CORS_ALLOW_METHODS", "CORS_ALLOW_HEADERS", "CORS_MAX_AGE",
		"RATE_LIMIT_PER_SECOND", "RATE_LIMIT_BURST", "RATE_LIMIT_TENANT_PER_SECOND",
		"RATE_LIMIT_TENANT_BURST", "RATE_LIMIT_MESSAGE", "TRUSTED_JWT_ISSUERS",
		"ADMIN_ROLES", "READ_ONLY_ROLES", "WRITE_ROLES", "REDIS_URL",
		"RATE_LIMIT", "REFRESH_TOKEN_TTL", "OTEL_COLLECTOR_URL",
	}

	for _, key := range allKeys {
		os.Unsetenv(key)
	}

	// Set test environment
	for key, value := range envVars {
		os.Setenv(key, value)
	}
	defer func() {
		for key := range envVars {
			os.Unsetenv(key)
		}
	}()

	config := Load()

	// Validate production-like configuration
	assert.Equal(t, "8080", config.Port)
	assert.Equal(t, "https://keycloak.prod.com", config.KeycloakURL)
	assert.Equal(t, "production", config.Realm)
	assert.Equal(t, "https://app.prod.com", config.FrontendURL)
	assert.Equal(t, []string{"https://app.prod.com", "https://admin.prod.com"}, config.CORSAllowOrigins)
	assert.Equal(t, 100.0, config.RateLimitPerSecond)
	assert.Equal(t, []string{"https://keycloak.prod.com", "https://backup-auth.prod.com"}, config.TrustedIssuers)
	assert.Equal(t, []string{"admin", "platform-admin"}, config.AdminRoles)
	assert.Equal(t, "redis://redis.prod.com:6379", config.RedisURL)

	// Validate default values for unset fields
	assert.Equal(t, 20, config.RateLimitBurst)                                            // default
	assert.Equal(t, 5000.0, config.RateLimitTenantPerSecond)                              // default
	assert.Equal(t, []string{"viewer", "read-only", "tenant-user"}, config.ReadOnlyRoles) // default
}

// Benchmark the configuration loading
func BenchmarkLoad(b *testing.B) {
	// Set up some environment variables
	os.Setenv("PORT", "8080")
	os.Setenv("KEYCLOAK_URL", "https://keycloak.example.com")
	os.Setenv("REALM", "test-realm")
	defer func() {
		os.Unsetenv("PORT")
		os.Unsetenv("KEYCLOAK_URL")
		os.Unsetenv("REALM")
	}()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = Load()
	}
}
