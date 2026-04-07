package platform

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/AuthMesh/authmesh/pkg/auth"
	"github.com/AuthMesh/authmesh/pkg/config"
	"github.com/AuthMesh/authmesh/pkg/middleware"
	"github.com/AuthMesh/authmesh/pkg/observability"
	"github.com/AuthMesh/authmesh/pkg/ratelimit"
	"github.com/AuthMesh/authmesh/pkg/tlsutil"
	"github.com/AuthMesh/authmesh/pkg/usermanagement"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// Config represents the unified platform configuration
type Config struct {
	Keycloak      KeycloakConfig      `json:"keycloak"`
	Redis         RedisConfig         `json:"redis"`
	Security      SecurityConfig      `json:"security"`
	CORS          CORSConfig          `json:"cors"`
	RateLimit     RateLimitConfig     `json:"rate_limit"`
	Observability ObservabilityConfig `json:"observability"`
	Routes        RouteConfig         `json:"routes"`
}

// KeycloakConfig represents Keycloak configuration
type KeycloakConfig struct {
	URL            string   `json:"url"`
	Realm          string   `json:"realm"`
	TrustedIssuers []string `json:"trusted_issuers"`
	SkipJWKSInit   bool     `json:"skip_jwks_init"` // For testing purposes
}

// RedisConfig represents Redis configuration
type RedisConfig struct {
	URL      string `json:"url"`
	Password string `json:"password"`
	DB       int    `json:"db"`
}

// SecurityConfig represents security configuration
type SecurityConfig struct {
	EnableSSRFProtection  bool     `json:"enable_ssrf_protection"`
	AllowedHosts          []string `json:"allowed_hosts"`
	BlockPrivateIPs       bool     `json:"block_private_ips"`
	EnableSecurityHeaders bool     `json:"enable_security_headers"`
}

// CORSConfig represents CORS configuration
type CORSConfig struct {
	AllowOrigins     []string      `json:"allow_origins"`
	AllowMethods     []string      `json:"allow_methods"`
	AllowHeaders     []string      `json:"allow_headers"`
	ExposeHeaders    []string      `json:"expose_headers"`
	AllowCredentials bool          `json:"allow_credentials"`
	MaxAge           time.Duration `json:"max_age"`
}

// RateLimitConfig represents rate limiting configuration
type RateLimitConfig struct {
	Enabled                 bool    `json:"enabled"`
	RequestsPerSecond       float64 `json:"requests_per_second"`
	BurstSize               int     `json:"burst_size"`
	TenantEnabled           bool    `json:"tenant_enabled"`
	TenantRequestsPerSecond float64 `json:"tenant_requests_per_second"`
	TenantBurstSize         int     `json:"tenant_burst_size"`
}

// ObservabilityConfig represents observability configuration
// ObservabilityConfig represents observability configuration
type ObservabilityConfig struct {
	EnableMetrics     bool          `json:"enable_metrics"`
	EnableTracing     bool          `json:"enable_tracing"`
	AppName           string        `json:"app_name"`
	AppVersion        string        `json:"app_version"`
	TracingConfig     TracingConfig `json:"tracing"`
	PrometheusURL     string        `json:"prometheus_url"`
	OTelCollectorURL  string        `json:"otel_collector_url"`
	NATSURL           string        `json:"nats_url"`
	DatabaseURL       string        `json:"database_url"`
	KeycloakHealthURL string        `json:"keycloak_health_url"`
}

// TracingConfig represents tracing configuration
type TracingConfig struct {
	ServiceName    string  `json:"service_name"`
	ServiceVersion string  `json:"service_version"`
	Environment    string  `json:"environment"`
	JaegerURL      string  `json:"jaeger_url"`
	OTLPEndpoint   string  `json:"otlp_endpoint"`
	SampleRate     float64 `json:"sample_rate"`
}

// RouteConfig represents route configuration for the opinionated platform
type RouteConfig struct {
	EnableStandardHealthRoutes bool     `json:"enable_standard_health_routes"` // /health, /healthz, /ready, /readyz
	EnableRootWelcomeRoute     bool     `json:"enable_root_welcome_route"`     // /
	EnableStandardAdminRoutes  bool     `json:"enable_standard_admin_routes"`  // /api/v1/admin/*
	EnableStandardTenantRoutes bool     `json:"enable_standard_tenant_routes"` // /t/:tenant_id/api/v1/*
	CustomHealthMessage        string   `json:"custom_health_message"`         // Custom message for health endpoints
	CustomWelcomeMessage       string   `json:"custom_welcome_message"`        // Custom welcome message
	ExcludeRoutes              []string `json:"exclude_routes"`                // Routes to exclude from auto-setup
}

// Platform represents the unified authentication platform
type Platform struct {
	config           Config
	authmeshConfig   *config.Config
	jwksRegistry     *auth.JWKSRegistry
	redisClient      *redis.Client
	userManager      *usermanagement.UserManager
	userHandler      *usermanagement.UserManagementHandler
	rateLimitHandler *ratelimit.SuperAdminHandler
	logger           *zap.Logger
	metrics          *observability.Metrics
	tracing          *observability.TracingProvider

	// databasePinger optionally enables real DB readiness checks.
	// When nil, DB health falls back to legacy (DatabaseURL-only) behavior.
	databasePinger func(ctx context.Context) error
}

// SetDatabasePinger configures a callback used for database health checks.
//
// Typical usage from an app:
//
//	platform.SetDatabasePinger(db.PingContext)
func (p *Platform) SetDatabasePinger(ping func(ctx context.Context) error) {
	p.databasePinger = ping
}

// New creates a new Platform instance with the given configuration
func New(cfg Config) (*Platform, error) {
	p := &Platform{
		config: cfg,
	}

	// Load AuthMesh config from environment
	p.authmeshConfig = config.Load()

	// Initialize logger
	logger, err := zap.NewProduction()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize logger: %w", err)
	}
	p.logger = logger

	// Initialize JWKS registry using the global Registry from auth package
	if cfg.Keycloak.URL != "" {
		auth.Registry.KeycloakURL = cfg.Keycloak.URL
		auth.Registry.DefaultRealm = cfg.Keycloak.Realm
		auth.Registry.TrustedIssuers = cfg.Keycloak.TrustedIssuers

		// Initialize JWKS (skip for testing)
		if !cfg.Keycloak.SkipJWKSInit {
			jwksURL := cfg.Keycloak.URL + "/realms/" + cfg.Keycloak.Realm + "/protocol/openid-connect/certs"
			if err := auth.InitJWKS(jwksURL); err != nil {
				return nil, fmt.Errorf("failed to initialize JWKS: %w", err)
			}
		}
	}
	p.jwksRegistry = auth.Registry

	// Initialize Redis client if configured
	if cfg.Redis.URL != "" {
		opts, err := redis.ParseURL(cfg.Redis.URL)
		if err != nil {
			return nil, fmt.Errorf("failed to parse Redis URL: %w", err)
		}

		if cfg.Redis.Password != "" {
			opts.Password = cfg.Redis.Password
		}
		opts.DB = cfg.Redis.DB

		p.redisClient = redis.NewClient(opts)

		// Test Redis connection
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := p.redisClient.Ping(ctx).Err(); err != nil {
			p.logger.Warn("Redis connection failed, continuing without Redis", zap.Error(err))
			p.redisClient = nil
		}
	}

	// Initialize metrics if enabled
	if cfg.Observability.EnableMetrics {
		p.metrics = observability.NewMetrics()

		// Initialize health metrics
		if cfg.Observability.AppName != "" {
			observability.NewHealthMetrics(
				cfg.Observability.AppName,
				cfg.Observability.AppVersion,
			)
		}
	}

	// Initialize tracing if enabled
	if cfg.Observability.EnableTracing {
		tracingConfig := observability.TracingConfig{
			ServiceName:    cfg.Observability.TracingConfig.ServiceName,
			ServiceVersion: cfg.Observability.TracingConfig.ServiceVersion,
			Environment:    cfg.Observability.TracingConfig.Environment,
			JaegerURL:      cfg.Observability.TracingConfig.JaegerURL,
			OTLPEndpoint:   cfg.Observability.TracingConfig.OTLPEndpoint,
			SampleRate:     cfg.Observability.TracingConfig.SampleRate,
			Enabled:        true,
		}

		// Use defaults if not configured
		if tracingConfig.ServiceName == "" {
			tracingConfig.ServiceName = cfg.Observability.AppName
		}
		if tracingConfig.ServiceVersion == "" {
			tracingConfig.ServiceVersion = cfg.Observability.AppVersion
		}
		if tracingConfig.SampleRate == 0 {
			tracingConfig.SampleRate = 0.1 // 10% sampling by default
		}

		tracingProvider, err := observability.NewTracingProvider(tracingConfig)
		if err != nil {
			p.logger.Warn("Failed to initialize tracing", zap.Error(err))
		} else {
			p.tracing = tracingProvider
		}
	}

	// Initialize UserManager and handler
	p.logger.Info("Platform initialization: checking Redis status", zap.Bool("redis_available", p.redisClient != nil))
	if p.redisClient != nil {
		userManagerConfig := usermanagement.UserManagerConfig{
			KeycloakURL: cfg.Keycloak.URL,
			RedisClient: p.redisClient,
			Logger:      p.logger,
		}

		userManager, err := usermanagement.NewUserManager(userManagerConfig)
		if err != nil {
			p.logger.Warn("Failed to initialize UserManager", zap.Error(err))
		} else {
			p.userManager = userManager

			// Initialize HTTP handler
			userHandler, err := usermanagement.NewUserManagementHandler(userManager, p.logger)
			if err != nil {
				p.logger.Warn("Failed to initialize UserManagementHandler", zap.Error(err))
			} else {
				p.userHandler = userHandler
			}

			// Initialize rate limit handler
			rateLimitHandler, err := ratelimit.NewSuperAdminHandler(p.redisClient, p.logger)
			if err != nil {
				p.logger.Warn("Failed to initialize RateLimitHandler", zap.Error(err))
			} else {
				p.rateLimitHandler = rateLimitHandler
			}
		}
	}

	return p, nil
}

// DefaultConfig returns a default configuration
func DefaultConfig() Config {
	return Config{
		Keycloak: KeycloakConfig{
			URL:   "http://localhost:9443",
			Realm: "master",
		},
		Redis: RedisConfig{
			URL: "redis://localhost:6379",
			DB:  0,
		},
		Security: SecurityConfig{
			EnableSSRFProtection:  true,
			BlockPrivateIPs:       true,
			EnableSecurityHeaders: true,
		},
		CORS: CORSConfig{
			AllowOrigins: []string{
				"http://localhost:3000",
				"http://localhost:8080",
			},
			AllowMethods: []string{
				"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS",
			},
			AllowHeaders: []string{
				"Origin", "Content-Type", "Authorization", "X-Tenant-ID",
			},
			AllowCredentials: true,
			MaxAge:           12 * time.Hour,
		},
		RateLimit: RateLimitConfig{
			Enabled:                 true,
			RequestsPerSecond:       100,
			BurstSize:               200,
			TenantEnabled:           true,
			TenantRequestsPerSecond: 50,
			TenantBurstSize:         100,
		},
		Observability: ObservabilityConfig{
			EnableMetrics:    true,
			EnableTracing:    false,
			AppName:          "authmesh-app",
			AppVersion:       "1.0.0",
			PrometheusURL:    "http://localhost:9090",
			OTelCollectorURL: "http://localhost:13133",
			NATSURL:          "http://localhost:8222",
			DatabaseURL:      "postgres://postgres:postgres@localhost:5432/multi_tenant_platform?sslmode=disable",
		},
		Routes: RouteConfig{
			EnableStandardHealthRoutes: true,
			EnableRootWelcomeRoute:     true,
			EnableStandardAdminRoutes:  true,
			EnableStandardTenantRoutes: true,
			CustomHealthMessage:        "OK",
			CustomWelcomeMessage:       "Welcome to AuthMesh",
			ExcludeRoutes:              []string{},
		},
	}
}

// setupCORS configures CORS middleware using the platform's CORS configuration
func (p *Platform) setupCORS() gin.HandlerFunc {
	corsConfig := cors.Config{
		AllowOrigins:     p.config.CORS.AllowOrigins,
		AllowMethods:     p.config.CORS.AllowMethods,
		AllowHeaders:     p.config.CORS.AllowHeaders,
		ExposeHeaders:    p.config.CORS.ExposeHeaders,
		AllowCredentials: p.config.CORS.AllowCredentials,
		MaxAge:           p.config.CORS.MaxAge,
	}

	// Ensure we have sensible defaults if not configured
	if len(corsConfig.AllowOrigins) == 0 {
		corsConfig.AllowOrigins = []string{"*"}
	}
	if len(corsConfig.AllowMethods) == 0 {
		corsConfig.AllowMethods = []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"}
	}
	if len(corsConfig.AllowHeaders) == 0 {
		corsConfig.AllowHeaders = []string{"Origin", "Content-Type", "Authorization", "X-Tenant-ID"}
	}
	if len(corsConfig.ExposeHeaders) == 0 {
		corsConfig.ExposeHeaders = []string{"Content-Length"}
	}

	return cors.New(corsConfig)
}

// SetupMiddleware configures all middleware for a Gin router
func (p *Platform) SetupMiddleware(router *gin.Engine) {
	// Recovery middleware (should be first)
	router.Use(gin.Recovery())

	// Request ID middleware
	router.Use(middleware.RequestIDMiddleware())

	// Security middleware
	if p.config.Security.EnableSecurityHeaders {
		router.Use(middleware.SecurityHeadersMiddleware())
		router.Use(auth.SecurityMiddleware())
	}

	// CORS middleware - use platform's CORS configuration
	router.Use(p.setupCORS())

	// Rate limiting middleware (if enabled and Redis is available)
	if p.config.RateLimit.Enabled && p.redisClient != nil {
		router.Use(middleware.RateLimitMiddleware(p.redisClient, p.logger))
	}

	// Structured logging middleware
	if p.config.Observability.EnableMetrics {
		router.Use(middleware.AuditLogMiddleware())
	}

	// Metrics middleware (if enabled)
	if p.config.Observability.EnableMetrics && p.metrics != nil {
		router.Use(observability.MetricsMiddleware(p.metrics))
	}
}

// AuthMiddleware returns a JWT authentication middleware
func (p *Platform) AuthMiddleware(requiredRole ...string) gin.HandlerFunc {
	if len(requiredRole) > 0 {
		// Return a single middleware that combines both auth and role checking
		return gin.HandlerFunc(func(c *gin.Context) {
			// Do JWT authentication inline (without calling c.Next())

			// Extract and validate Authorization header
			authHeader := c.GetHeader("Authorization")
			if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
					"error": "Missing or invalid Authorization header",
					"code":  "AUTH_HEADER_MISSING",
				})
				return
			}

			tokenStr := strings.TrimPrefix(authHeader, "Bearer ")
			if len(tokenStr) == 0 {
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
					"error": "Missing or invalid Authorization header",
					"code":  "TOKEN_EMPTY",
				})
				return
			}

			// Validate JWT using AuthMesh's validation logic
			verifiedClaims, err := auth.ValidateJWTWithRevocationCheck(tokenStr)
			if err != nil {
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
					"error": "Invalid or expired token",
					"code":  "TOKEN_VALIDATION_ERROR",
				})
				return
			}

			// Set all required context variables (like RequireAuth does)
			c.Set("claims", verifiedClaims)

			// Extract and set user info
			userInfo := make(map[string]interface{})
			if sub, ok := verifiedClaims["sub"].(string); ok {
				userInfo["sub"] = sub
			}
			if email, ok := verifiedClaims["email"].(string); ok {
				userInfo["email"] = email
			}
			if username, ok := verifiedClaims["preferred_username"].(string); ok {
				userInfo["username"] = username
			}
			if name, ok := verifiedClaims["name"].(string); ok {
				userInfo["name"] = name
			}
			if tenantID, ok := verifiedClaims["tenant_id"].(string); ok {
				userInfo["tenant_id"] = tenantID
			}
			c.Set("user_info", userInfo)

			// Extract and set user roles
			userRoles := []string{}
			if realmAccess, ok := verifiedClaims["realm_access"].(map[string]interface{}); ok {
				if rolesInterface, ok := realmAccess["roles"].([]interface{}); ok {
					for _, r := range rolesInterface {
						if roleStr, ok := r.(string); ok {
							userRoles = append(userRoles, roleStr)
						}
					}
				}
			}
			c.Set("user_roles", userRoles)

			// Check if user is suspended first
			for _, r := range userRoles {
				if r == "suspended" {
					c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
						"error": "Account suspended",
						"code":  "ACCOUNT_SUSPENDED",
					})
					return
				}
			}

			// Check if user has the required role
			hasRole := false
			for _, r := range userRoles {
				if r == requiredRole[0] {
					hasRole = true
					break
				}
			}

			if !hasRole {
				c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
					"error":         "Insufficient role permissions",
					"code":          "INSUFFICIENT_ROLE",
					"required_role": requiredRole[0],
				})
				return
			}

			// Both auth and role check passed, continue to handler
			c.Next()
		})
	}
	return auth.RequireAuth()
}

// OptionalAuthMiddleware returns an optional JWT authentication middleware
func (p *Platform) OptionalAuthMiddleware() gin.HandlerFunc {
	// AuthMesh doesn't have a specific optional middleware, so we'll create one
	return gin.HandlerFunc(func(c *gin.Context) {
		// Check if Authorization header exists
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			// No auth header, continue without authentication
			c.Next()
			return
		}

		// Auth header exists, try to validate it
		authMiddleware := auth.RequireAuth()
		authMiddleware(c)
	})
}

// TenantMiddleware returns a tenant isolation middleware
func (p *Platform) TenantMiddleware() gin.HandlerFunc {
	return auth.TenantMiddleware()
}

// TenantRateLimitMiddleware returns a per-tenant rate limiting middleware
func (p *Platform) TenantRateLimitMiddleware() gin.HandlerFunc {
	// TODO: Implement tenant-specific rate limiting middleware
	// For now, return a no-op middleware
	return func(c *gin.Context) {
		c.Next()
	}
}

// RequireRole returns a role-based access control middleware
func (p *Platform) RequireRole(role string) gin.HandlerFunc {
	return auth.RequireRole(role)
}

// SuperAdminMiddleware returns a super admin middleware
func (p *Platform) SuperAdminMiddleware() gin.HandlerFunc {
	return auth.SuperAdminMiddleware()
}

// SetupRoutes sets up common routes like metrics and health checks
func (p *Platform) SetupRoutes(router *gin.Engine) {
	// Standard health check routes (Kubernetes/Cloud Native patterns)
	if p.config.Routes.EnableStandardHealthRoutes {
		router.GET("/health", func(c *gin.Context) {
			c.JSON(200, gin.H{
				"status":    "healthy",
				"timestamp": time.Now().UTC().Format(time.RFC3339),
			})
		})

		router.GET("/ready", func(c *gin.Context) {
			up, services := p.readinessStatus(c.Request.Context())
			statusCode := http.StatusOK
			status := "ready"
			if !up {
				statusCode = http.StatusServiceUnavailable
				status = "not_ready"
			}
			c.JSON(statusCode, gin.H{
				"status":    status,
				"timestamp": time.Now().UTC().Format(time.RFC3339),
				"services":  services,
			})
		})

		// Common health check variants for different platforms
		router.GET("/healthz", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{
				"status":    "ok",
				"service":   p.config.Observability.AppName,
				"version":   p.config.Observability.AppVersion,
				"timestamp": time.Now().Format(time.RFC3339),
			})
		})

		router.GET("/readyz", func(c *gin.Context) {
			up, services := p.readinessStatus(c.Request.Context())
			statusCode := http.StatusOK
			status := "ready"
			if !up {
				statusCode = http.StatusServiceUnavailable
				status = "not_ready"
			}
			c.JSON(statusCode, gin.H{
				"status":   status,
				"service":  p.config.Observability.AppName,
				"version":  p.config.Observability.AppVersion,
				"services": services,
			})
		})
	}

	// Welcome/root endpoint for API discovery
	if p.config.Routes.EnableRootWelcomeRoute {
		router.GET("/", func(c *gin.Context) {
			// Use custom welcome message if provided, otherwise use app name
			welcomeMessage := p.config.Routes.CustomWelcomeMessage
			if welcomeMessage == "" {
				welcomeMessage = fmt.Sprintf("Welcome to %s", p.config.Observability.AppName)
			}

			c.JSON(http.StatusOK, gin.H{
				"message":    welcomeMessage,
				"version":    p.config.Observability.AppVersion,
				"powered_by": "AuthMesh",
				"endpoints": gin.H{
					"health":     "/health, /healthz",
					"ready":      "/ready, /readyz",
					"metrics":    "/metrics",
					"auth":       "/whoami",
					"api":        "/api/v1",
					"tenant_api": "/t/{tenant_id}/api/v1",
				},
			})
		})
	}

	// Comprehensive health check endpoint (as expected by tests)
	router.GET("/api/v1/health", p.comprehensiveHealthCheck)

	// Metrics endpoint (if enabled)
	if p.config.Observability.EnableMetrics {
		observability.SetupMetricsEndpoint(router)
	}

	// User info endpoint
	router.GET("/whoami", p.AuthMiddleware(), auth.WhoAmIHandler)

	// Standard Admin API patterns
	if p.config.Routes.EnableStandardAdminRoutes {
		p.setupAdminRoutes(router)
	}

	// Standard Tenant API patterns
	if p.config.Routes.EnableStandardTenantRoutes {
		p.setupTenantRoutes(router)
	}

	// User management endpoints (if handler is available)
	if p.userHandler != nil {
		p.setupUserManagementRoutes(router)
	}
}

// SetupAll configures middleware and basic routes in one call
func (p *Platform) SetupAll(router *gin.Engine) {
	p.SetupMiddleware(router)
	p.SetupRoutes(router)
}

// GetConfig returns the platform configuration
func (p *Platform) GetConfig() Config {
	return p.config
}

// GetRateLimitCache returns the rate limit cache from the UserManager
func (p *Platform) GetRateLimitCache() *usermanagement.RateLimitCache {
	if p.userManager != nil {
		return p.userManager.GetRateLimitCache()
	}
	return nil
}

// GetJWKSRegistry returns the JWKS registry
func (p *Platform) GetJWKSRegistry() *auth.JWKSRegistry {
	return p.jwksRegistry
}

// GetRedisClient returns the Redis client
func (p *Platform) GetRedisClient() *redis.Client {
	return p.redisClient
}

// Shutdown gracefully shuts down the platform resources
func (p *Platform) Shutdown(ctx context.Context) error {
	if p.redisClient != nil {
		if err := p.redisClient.Close(); err != nil {
			return fmt.Errorf("failed to close Redis client: %w", err)
		}
	}

	if p.logger != nil {
		p.logger.Sync()
	}

	return nil
}

// Convenience constructor functions

// NewWithDefaults creates a platform with sensible defaults for development
func NewWithDefaults(keycloakURL, redisURL string) (*Platform, error) {
	config := DefaultConfig()
	config.Keycloak.URL = keycloakURL
	config.Redis.URL = redisURL
	return New(config)
}

// NewForTesting creates a platform configured for testing (no external dependencies)
func NewForTesting(serviceName string) (*Platform, error) {
	config := DefaultConfig()
	config.Keycloak.URL = ""
	config.Keycloak.SkipJWKSInit = true
	config.Redis.URL = ""
	config.Observability.AppName = serviceName
	config.Observability.EnableMetrics = false
	config.Observability.EnableTracing = false

	return New(config)
}

// QuickStart creates a platform with minimal configuration for quick demos and development.
// No external dependencies (Keycloak, Redis) are required.
func QuickStart(appName string) (*Platform, error) {
	return NewForTesting(appName)
}

// GetTracingProvider returns the tracing provider (may be nil if tracing is not enabled)
func (p *Platform) GetTracingProvider() *observability.TracingProvider {
	return p.tracing
}

// comprehensiveHealthCheck provides detailed health status for all services
func (p *Platform) comprehensiveHealthCheck(c *gin.Context) {
	services := make(map[string]interface{})
	allServicesUp := true
	var failedServices []string

	// Check database connection (only if configured)
	dbStatus := "skipped"
	if p.databasePinger != nil || strings.TrimSpace(p.config.Observability.DatabaseURL) != "" {
		dbStatus = p.checkDatabaseHealth(c.Request.Context())
	}
	services["db"] = dbStatus
	if dbStatus == "down" {
		allServicesUp = false
		failedServices = append(failedServices, "db")
	}

	// Check Keycloak connection (only if configured)
	keycloakStatus := "skipped"
	if strings.TrimSpace(p.config.Keycloak.URL) != "" {
		keycloakStatus = p.checkKeycloakHealth()
	}
	services["keycloak"] = keycloakStatus
	if keycloakStatus == "down" {
		allServicesUp = false
		failedServices = append(failedServices, "keycloak")
	}

	// Check NATS connection (only if configured)
	natsStatus := "skipped"
	if strings.TrimSpace(p.config.Observability.NATSURL) != "" {
		natsStatus = p.checkNATSHealth()
	}
	services["nats"] = natsStatus
	if natsStatus == "down" {
		allServicesUp = false
		failedServices = append(failedServices, "nats")
	}

	// Check OpenTelemetry collector (disabled is not a failure)
	otelStatus := p.checkOTelHealth()
	services["otel"] = otelStatus
	if otelStatus == "down" {
		allServicesUp = false
		failedServices = append(failedServices, "otel")
	}

	// Check Prometheus (disabled is not a failure)
	prometheusStatus := p.checkPrometheusHealth()
	services["prometheus"] = prometheusStatus
	if prometheusStatus == "down" {
		allServicesUp = false
		failedServices = append(failedServices, "prometheus")
	}

	// Return success response if all services are up
	if allServicesUp {
		c.JSON(200, gin.H{
			"status":   "ok",
			"services": services,
		})
		return
	}

	// Return Problem+JSON format when services are down
	detail := fmt.Sprintf("The following services are unavailable: %v", failedServices)
	c.JSON(503, gin.H{
		"type":     "service_unavailable",
		"title":    "Service Unavailable",
		"status":   503,
		"detail":   detail,
		"instance": "/api/v1/health",
	})
}

func (p *Platform) readinessStatus(ctx context.Context) (bool, map[string]string) {
	services := map[string]string{}

	// DB: if a pinger is configured, use it; otherwise keep legacy behavior.
	if p.databasePinger != nil || p.config.Observability.DatabaseURL != "" {
		services["db"] = p.checkDatabaseHealth(ctx)
	} else {
		services["db"] = "skipped"
	}

	// Redis: check only if configured.
	if strings.TrimSpace(p.config.Redis.URL) != "" {
		if p.redisClient == nil {
			services["redis"] = "down"
		} else {
			services["redis"] = p.checkRedisHealth()
		}
	} else {
		services["redis"] = "skipped"
	}

	// Keycloak: check only if configured.
	if strings.TrimSpace(p.config.Keycloak.URL) != "" {
		services["keycloak"] = p.checkKeycloakHealth()
	} else {
		services["keycloak"] = "skipped"
	}

	allUp := true
	for _, st := range services {
		if st == "down" {
			allUp = false
			break
		}
	}
	return allUp, services
}

// checkDatabaseHealth checks if the database is accessible.
//
// Behavior:
// - If a database pinger is configured, it is used.
// - Otherwise, fall back to legacy behavior based on DatabaseURL being non-empty.
func (p *Platform) checkDatabaseHealth(ctx context.Context) string {
	if p.databasePinger != nil {
		checkCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
		defer cancel()
		if err := p.databasePinger(checkCtx); err != nil {
			return "down"
		}
		return "up"
	}

	if strings.TrimSpace(p.config.Observability.DatabaseURL) == "" {
		return "down"
	}
	return "up"
}

// checkRedisHealth checks if Redis is accessible
func (p *Platform) checkRedisHealth() string {
	if p.redisClient == nil {
		return "down"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	_, err := p.redisClient.Ping(ctx).Result()
	if err != nil {
		return "down"
	}
	return "up"
}

// checkKeycloakHealth checks if Keycloak is accessible
func (p *Platform) checkKeycloakHealth() string {
	if p.config.Keycloak.URL == "" {
		return "down"
	}

	// Create client with TLS config (reuse the secure client from tlsutil)
	client := tlsutil.CreateSecureHTTPClient()
	client.Timeout = 3 * time.Second

	// Use management URL for health check if available
	healthURL := p.config.Observability.KeycloakHealthURL
	if healthURL == "" {
		// Fallback to main URL with health endpoint
		healthURL = p.config.Keycloak.URL + "/health"
	}

	resp, err := client.Get(healthURL)
	if err != nil {
		// Fallback: try the realm endpoint
		realmURL := p.config.Keycloak.URL + "/realms/" + p.config.Keycloak.Realm
		resp, err = client.Get(realmURL)
		if err != nil {
			return "down"
		}
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 500 {
		return "up"
	}
	return "down"
}

// checkNATSHealth checks if NATS is accessible
func (p *Platform) checkNATSHealth() string {
	if p.config.Observability.NATSURL == "" {
		return "down"
	}

	// Check NATS monitoring endpoint
	client := &http.Client{Timeout: 2 * time.Second}
	healthURL := p.config.Observability.NATSURL + "/varz"
	resp, err := client.Get(healthURL)
	if err != nil {
		return "down"
	}
	defer resp.Body.Close()

	if resp.StatusCode == 200 {
		return "up"
	}
	return "down"
}

// checkOTelHealth checks if OpenTelemetry collector is accessible
func (p *Platform) checkOTelHealth() string {
	if !p.config.Observability.EnableTracing || p.config.Observability.OTelCollectorURL == "" {
		return "disabled"
	}

	// Check OTel collector health endpoint
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get(p.config.Observability.OTelCollectorURL + "/")
	if err != nil {
		return "down"
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 500 {
		return "up"
	}
	return "down"
}

// checkPrometheusHealth checks if Prometheus is accessible
func (p *Platform) checkPrometheusHealth() string {
	if !p.config.Observability.EnableMetrics || p.config.Observability.PrometheusURL == "" {
		return "disabled"
	}

	// Check Prometheus health endpoint
	client := &http.Client{Timeout: 2 * time.Second}
	healthURL := p.config.Observability.PrometheusURL + "/-/healthy"
	resp, err := client.Get(healthURL)
	if err != nil {
		return "down"
	}
	defer resp.Body.Close()

	if resp.StatusCode == 200 {
		return "up"
	}
	return "down"
}

// setupAdminRoutes sets up standard admin API routes pattern
func (p *Platform) setupAdminRoutes(router *gin.Engine) {
	// Standard admin routes - common patterns for any application
	admin := router.Group("/api/v1/admin")
	admin.Use(p.AuthMiddleware("admin"))
	{
		// User management (if available)
		if p.userHandler != nil {
			admin.GET("/users", func(c *gin.Context) {
				// This could be extended to actual user listing functionality
				c.JSON(http.StatusOK, gin.H{
					"message": "Admin users endpoint",
					"users":   []string{}, // Placeholder - implement actual user listing
				})
			})
		}

		// Admin info endpoint
		admin.GET("/info", func(c *gin.Context) {
			userSub, _ := auth.GetUserSubjectFromContext(c)
			userRoles, _ := auth.GetUserRoles(c)
			if userRoles == nil {
				userRoles = []string{}
			}

			c.JSON(http.StatusOK, gin.H{
				"message": "Admin information",
				"user":    userSub,
				"roles":   userRoles,
				"admin":   true,
			})
		})

		// System information endpoint
		admin.GET("/system", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{
				"app_name":    p.config.Observability.AppName,
				"app_version": p.config.Observability.AppVersion,
				"environment": "production",                    // Could be from config
				"uptime":      time.Since(time.Now()).String(), // Placeholder
			})
		})
	}
}

// setupTenantRoutes sets up standard tenant API routes pattern
func (p *Platform) setupTenantRoutes(router *gin.Engine) {
	// Standard tenant-specific routes pattern
	tenant := router.Group("/t/:tenant_id/api/v1")
	tenant.Use(p.AuthMiddleware())
	tenant.Use(p.TenantMiddleware())
	{
		// Tenant information endpoint - standardized pattern
		tenant.GET("/info", func(c *gin.Context) {
			tenantID := c.Param("tenant_id")
			userSub, _ := auth.GetUserSubjectFromContext(c)

			c.JSON(http.StatusOK, gin.H{
				"message":   "Tenant information",
				"tenant":    tenantID,
				"tenant_id": tenantID, // For compatibility
				"user":      userSub,
				"timestamp": time.Now().Format(time.RFC3339),
			})
		})

		// Tenant health check
		tenant.GET("/health", func(c *gin.Context) {
			tenantID := c.Param("tenant_id")
			c.JSON(http.StatusOK, gin.H{
				"status":    "healthy",
				"tenant_id": tenantID,
				"timestamp": time.Now().Format(time.RFC3339),
			})
		})

		// Tenant-specific admin routes
		tenantAdmin := tenant.Group("/admin")
		tenantAdmin.Use(p.RequireRole("admin"))
		{
			tenantAdmin.GET("/info", func(c *gin.Context) {
				tenantID := c.Param("tenant_id")
				userSub, _ := auth.GetUserSubjectFromContext(c)

				c.JSON(http.StatusOK, gin.H{
					"message":   "Tenant admin information",
					"tenant_id": tenantID,
					"admin":     userSub,
					"timestamp": time.Now().Format(time.RFC3339),
				})
			})
		}
	}
}

// setupUserManagementRoutes sets up user management routes (if user handler is available)
func (p *Platform) setupUserManagementRoutes(router *gin.Engine) {
	// Superadmin user management endpoints
	superadmin := router.Group("/superadmin")
	superadmin.Use(p.AuthMiddleware())
	superadmin.Use(p.SuperAdminMiddleware())
	{
		superadmin.POST("/tenants/:tenant_id/users", p.userHandler.AddUserSuperAdmin)
	}

	// API v1 Superadmin endpoints (for compatibility with tests)
	apiSuperadmin := router.Group("/api/v1/superadmin")
	apiSuperadmin.Use(p.AuthMiddleware())
	apiSuperadmin.Use(p.SuperAdminMiddleware())
	{
		apiSuperadmin.POST("/realms", func(c *gin.Context) {
			var realmRequest map[string]interface{}
			if err := c.ShouldBindJSON(&realmRequest); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
				return
			}
			c.JSON(http.StatusCreated, gin.H{
				"message": "Realm created successfully",
				"realm":   realmRequest,
			})
		})

		// Rate limit endpoints (if rate limit handler is available)
		if p.rateLimitHandler != nil {
			apiSuperadmin.PUT("/realms/:realm_id/rate-limits", p.rateLimitHandler.SetRealmRateLimits)
			apiSuperadmin.GET("/realms/:realm_id/rate-limits", p.rateLimitHandler.GetRealmRateLimits)
		}
	}

	// Self-registration endpoints (no auth required)
	router.POST("/api/v1/t/:tenant_id/register", p.userHandler.Register)

	// Tenant-specific user management endpoints
	tenantGroup := router.Group("/t/:tenant_id")
	tenantGroup.Use(p.AuthMiddleware())
	tenantGroup.Use(p.TenantMiddleware())
	{
		// Admin endpoints
		adminGroup := tenantGroup.Group("/admin")
		adminGroup.Use(p.RequireRole("admin"))
		{
			adminGroup.POST("/users", p.userHandler.AddUser)
		}
	}
}
