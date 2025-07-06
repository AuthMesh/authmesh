package config

import (
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

// Config holds all configuration values for the application
type Config struct {
	Port        string
	KeycloakURL string
	Realm       string

	// CORS Configuration
	CORSAllowOrigins []string
	CORSAllowMethods []string
	CORSAllowHeaders []string
	CORSMaxAge       string

	// Security Configuration
	RateLimitPerSecond       float64
	RateLimitBurst           int
	RateLimitTenantPerSecond float64
	RateLimitTenantBurst     int
	RateLimitMessage         string
	TrustedIssuers           []string // Whitelist of trusted JWT issuer base URLs

	// Frontend Configuration
	FrontendURL string

	// Role Configuration
	AdminRoles    []string
	ReadOnlyRoles []string
	WriteRoles    []string

	// Stage 4: Redis, Rate Limiting, and OpenTelemetry Configuration
	RedisURL         string
	RateLimit        string
	RefreshTokenTTL  string
	OTelCollectorURL string
}

// Load reads configuration from environment variables and returns a Config struct
func Load() *Config {
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using system env")
	}

	// Core server configuration
	port := os.Getenv("PORT")
	if port == "" {
		port = "8081"
	}

	keycloakURL := os.Getenv("KEYCLOAK_URL")
	if keycloakURL == "" {
		keycloakURL = "https://localhost:9443"
	}

	realm := os.Getenv("REALM")
	if realm == "" {
		realm = "tenant1" // Default to tenant1 for multi-tenant platform
	}

	// Frontend URL configuration
	frontendURL := os.Getenv("FRONTEND_URL")
	if frontendURL == "" {
		frontendURL = "http://localhost:3000"
	}

	// CORS configuration
	corsOrigins := getCORSOrigins(frontendURL)
	corsMethods := getCORSMethods()
	corsHeaders := getCORSHeaders()
	corsMaxAge := os.Getenv("CORS_MAX_AGE")
	if corsMaxAge == "" {
		corsMaxAge = "86400" // 24 hours
	}

	// Rate limiting configuration
	rateLimitPerSecond := getRateLimitPerSecond()
	rateLimitBurst := getRateLimitBurst()
	rateLimitTenantPerSecond := getRateLimitTenantPerSecond()
	rateLimitTenantBurst := getRateLimitTenantBurst()
	rateLimitMessage := os.Getenv("RATE_LIMIT_MESSAGE")
	if rateLimitMessage == "" {
		rateLimitMessage = "Rate limit exceeded. Please try again later."
	}

	// Trusted issuers configuration - SECURITY: prevent SSRF attacks
	trustedIssuers := getTrustedIssuers(keycloakURL)

	// Role configuration
	adminRoles := getRoles("ADMIN_ROLES", []string{"admin", "super-admin", "platform-admin"})
	readOnlyRoles := getRoles("READ_ONLY_ROLES", []string{"viewer", "read-only", "tenant-user"})
	writeRoles := getRoles("WRITE_ROLES", []string{"admin", "tenant-admin", "platform-admin"})

	// Stage 4 configuration
	redisURL := os.Getenv("REDIS_URL")
	if redisURL == "" {
		redisURL = "redis://localhost:6379"
	}

	rateLimit := os.Getenv("RATE_LIMIT")
	if rateLimit == "" {
		rateLimit = "10:60" // Default: 10 requests per minute
	}

	refreshTokenTTL := os.Getenv("REFRESH_TOKEN_TTL")
	if refreshTokenTTL == "" {
		refreshTokenTTL = "3600" // Default: 1 hour
	}

	otelCollectorURL := os.Getenv("OTEL_COLLECTOR_URL")
	if otelCollectorURL == "" {
		otelCollectorURL = "http://localhost:4317"
	}

	return &Config{
		Port:        port,
		KeycloakURL: keycloakURL,
		Realm:       realm,

		CORSAllowOrigins: corsOrigins,
		CORSAllowMethods: corsMethods,
		CORSAllowHeaders: corsHeaders,
		CORSMaxAge:       corsMaxAge,

		RateLimitPerSecond:       rateLimitPerSecond,
		RateLimitBurst:           rateLimitBurst,
		RateLimitTenantPerSecond: rateLimitTenantPerSecond,
		RateLimitTenantBurst:     rateLimitTenantBurst,
		RateLimitMessage:         rateLimitMessage,
		TrustedIssuers:           trustedIssuers,

		FrontendURL: frontendURL,

		AdminRoles:    adminRoles,
		ReadOnlyRoles: readOnlyRoles,
		WriteRoles:    writeRoles,

		RedisURL:         redisURL,
		RateLimit:        rateLimit,
		RefreshTokenTTL:  refreshTokenTTL,
		OTelCollectorURL: otelCollectorURL,
	}
}

// getCORSOrigins parses CORS allowed origins from environment
func getCORSOrigins(frontendURL string) []string {
	corsOrigins := os.Getenv("CORS_ALLOW_ORIGINS")
	if corsOrigins == "" {
		// Default to frontend URL + common development URLs
		return []string{frontendURL, "http://localhost:3000", "http://localhost:3001"}
	}
	return strings.Split(corsOrigins, ",")
}

// getCORSMethods returns allowed CORS methods
func getCORSMethods() []string {
	corsMethods := os.Getenv("CORS_ALLOW_METHODS")
	if corsMethods == "" {
		return []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"}
	}
	return strings.Split(corsMethods, ",")
}

// getCORSHeaders returns allowed CORS headers
func getCORSHeaders() []string {
	corsHeaders := os.Getenv("CORS_ALLOW_HEADERS")
	if corsHeaders == "" {
		return []string{
			"Origin", "Content-Type", "Accept", "Authorization", "X-Request-ID",
			"X-Tenant-ID", "Content-Length", "Accept-Encoding", "X-CSRF-Token",
		}
	}
	return strings.Split(corsHeaders, ",")
}

// getRateLimitPerSecond parses rate limit per second from environment
func getRateLimitPerSecond() float64 {
	rateLimitStr := os.Getenv("RATE_LIMIT_PER_SECOND")
	if rateLimitStr == "" {
		return 10.0 // Default: 10 requests per second
	}

	rateLimit, err := strconv.ParseFloat(rateLimitStr, 64)
	if err != nil {
		log.Printf("Invalid RATE_LIMIT_PER_SECOND value: %s, using default 10.0", rateLimitStr)
		return 10.0
	}
	return rateLimit
}

// getRateLimitBurst parses rate limit burst from environment
func getRateLimitBurst() int {
	rateLimitBurstStr := os.Getenv("RATE_LIMIT_BURST")
	if rateLimitBurstStr == "" {
		return 20 // Default: burst of 20 requests
	}

	rateLimitBurst, err := strconv.Atoi(rateLimitBurstStr)
	if err != nil {
		log.Printf("Invalid RATE_LIMIT_BURST value: %s, using default 20", rateLimitBurstStr)
		return 20
	}
	return rateLimitBurst
}

// getRoles parses role configuration from environment
func getRoles(envKey string, defaultRoles []string) []string {
	rolesStr := os.Getenv(envKey)
	if rolesStr == "" {
		return defaultRoles
	}
	return strings.Split(rolesStr, ",")
}

// getTrustedIssuers parses trusted JWT issuer base URLs from environment
// SECURITY: This prevents SSRF attacks by whitelisting trusted issuer URLs
func getTrustedIssuers(keycloakURL string) []string {
	trustedIssuersStr := os.Getenv("TRUSTED_JWT_ISSUERS")
	if trustedIssuersStr == "" {
		// Default trusted issuers based on Keycloak URL
		defaultIssuers := []string{
			keycloakURL,
			"https://localhost:9443",
			"https://localhost:9444",
		}
		log.Printf("[SECURITY] Using default trusted issuers: %v", defaultIssuers)
		return defaultIssuers
	}

	trustedIssuers := strings.Split(trustedIssuersStr, ",")
	// Trim whitespace from each issuer
	for i, issuer := range trustedIssuers {
		trustedIssuers[i] = strings.TrimSpace(issuer)
	}

	log.Printf("[SECURITY] Loaded trusted issuers from env: %v", trustedIssuers)
	return trustedIssuers
}

// getRateLimitTenantPerSecond parses tenant rate limit per second from environment
func getRateLimitTenantPerSecond() float64 {
	rateLimitStr := os.Getenv("RATE_LIMIT_TENANT_PER_SECOND")
	if rateLimitStr == "" {
		return 5000.0 // Default: 5000 requests per second per tenant
	}

	rateLimit, err := strconv.ParseFloat(rateLimitStr, 64)
	if err != nil {
		log.Printf("Invalid RATE_LIMIT_TENANT_PER_SECOND value: %s, using default 5000.0", rateLimitStr)
		return 5000.0
	}
	return rateLimit
}

// getRateLimitTenantBurst parses tenant rate limit burst from environment
func getRateLimitTenantBurst() int {
	rateLimitBurstStr := os.Getenv("RATE_LIMIT_TENANT_BURST")
	if rateLimitBurstStr == "" {
		return 5500 // Default: burst of 5500 requests per tenant
	}

	rateLimitBurst, err := strconv.Atoi(rateLimitBurstStr)
	if err != nil {
		log.Printf("Invalid RATE_LIMIT_TENANT_BURST value: %s, using default 5500", rateLimitBurstStr)
		return 5500
	}
	return rateLimitBurst
}
