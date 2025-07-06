package platform

import (
	"fmt"
	"time"
	"context"

	"github.com/AuthMesh/authmesh/pkg/auth"
	"github.com/AuthMesh/authmesh/pkg/middleware"
	"github.com/AuthMesh/authmesh/pkg/observability"
	"github.com/AuthMesh/authmesh/pkg/ratelimit"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// Config represents the unified platform configuration
type Config struct {
	Keycloak     KeycloakConfig     `json:"keycloak"`
	Redis        RedisConfig        `json:"redis"`
	Security     SecurityConfig     `json:"security"`
	CORS         CORSConfig         `json:"cors"`
	RateLimit    RateLimitConfig    `json:"rate_limit"`
	Observability ObservabilityConfig `json:"observability"`
}

// KeycloakConfig represents Keycloak configuration
type KeycloakConfig struct {
	URL            string   `json:"url"`
	Realm          string   `json:"realm"`
	TrustedIssuers []string `json:"trusted_issuers"`
}

// RedisConfig represents Redis configuration
type RedisConfig struct {
	URL      string `json:"url"`
	Password string `json:"password"`
	DB       int    `json:"db"`
}

// SecurityConfig represents security configuration
type SecurityConfig struct {
	EnableSSRFProtection   bool     `json:"enable_ssrf_protection"`
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
	Enabled       bool    `json:"enabled"`
	RequestsPerSecond float64 `json:"requests_per_second"`
	BurstSize     int     `json:"burst_size"`
	TenantEnabled bool    `json:"tenant_enabled"`
	TenantRequestsPerSecond float64 `json:"tenant_requests_per_second"`
	TenantBurstSize int     `json:"tenant_burst_size"`
}

// ObservabilityConfig represents observability configuration
type ObservabilityConfig struct {
	EnableMetrics bool   `json:"enable_metrics"`
	EnableTracing bool   `json:"enable_tracing"`
	AppName      string `json:"app_name"`
	AppVersion   string `json:"app_version"`
	TracingConfig TracingConfig `json:"tracing"`
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

// Platform represents the unified authentication platform
type Platform struct {
	config       Config
	jwksRegistry *auth.JWKSRegistry
	redisClient  *redis.Client
	metrics      *observability.Metrics
	tracing      *observability.TracingProvider
}

// New creates a new Platform instance with the given configuration
func New(cfg Config) (*Platform, error) {
	p := &Platform{
		config: cfg,
	}

	// Initialize JWKS registry
	p.jwksRegistry = auth.NewJWKSRegistry(
		cfg.Keycloak.URL,
		cfg.Keycloak.Realm,
		cfg.Keycloak.TrustedIssuers,
	)

	// Initialize Redis client if configured
	if cfg.Redis.URL != "" {
		opts, err := redis.ParseURL(cfg.Redis.URL)
		if err != nil {
			return nil, err
		}
		
		if cfg.Redis.Password != "" {
			opts.Password = cfg.Redis.Password
		}
		opts.DB = cfg.Redis.DB
		
		p.redisClient = redis.NewClient(opts)
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
		
		var err error
		p.tracing, err = observability.NewTracingProvider(tracingConfig)
		if err != nil {
			return nil, err
		}
	}

	// Initialize JWKS
	jwksURL := cfg.Keycloak.URL + "/realms/" + cfg.Keycloak.Realm + "/protocol/openid-connect/certs"
	if err := auth.InitJWKS(jwksURL); err != nil {
		return nil, err
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
			EnableSSRFProtection:   true,
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
			MaxAge:          12 * time.Hour,
		},
		RateLimit: RateLimitConfig{
			Enabled:           true,
			RequestsPerSecond: 100,
			BurstSize:        200,
			TenantEnabled:    true,
			TenantRequestsPerSecond: 50,
			TenantBurstSize:  100,
		},
		Observability: ObservabilityConfig{
			EnableMetrics: true,
			EnableTracing: false,
			AppName:      "authmesh-app",
			AppVersion:   "1.0.0",
			TracingConfig: TracingConfig{
				ServiceName:    "authmesh-app",
				ServiceVersion: "1.0.0",
				Environment:    "development",
				OTLPEndpoint:   "http://localhost:4318",
				SampleRate:     0.1,
			},
		},
	}
}

// SetupMiddleware configures all middleware for a Gin router
func (p *Platform) SetupMiddleware(router *gin.Engine) {
	// Recovery middleware (should be first)
	if p.config.Observability.EnableMetrics {
		logger, _ := zap.NewProduction()
		router.Use(middleware.RecoveryWithLoggingMiddleware(logger))
	} else {
		router.Use(gin.Recovery())
	}
	
	// Request ID middleware  
	router.Use(middleware.RequestIDMiddleware())
	
	// Tracing middleware (early in the chain)
	if p.config.Observability.EnableTracing && p.tracing != nil {
		router.Use(p.tracing.TracingMiddleware())
	}
	
	// Security middleware
	securityConfig := middleware.SecurityConfig{
		EnableSSRFProtection:   p.config.Security.EnableSSRFProtection,
		AllowedHosts:          p.config.Security.AllowedHosts,
		BlockPrivateIPs:       p.config.Security.BlockPrivateIPs,
		EnableSecurityHeaders: p.config.Security.EnableSecurityHeaders,
	}
	router.Use(middleware.SecurityMiddleware(securityConfig))
	
	// CORS middleware
	corsConfig := middleware.CORSConfig{
		AllowOrigins:     p.config.CORS.AllowOrigins,
		AllowMethods:     p.config.CORS.AllowMethods,
		AllowHeaders:     p.config.CORS.AllowHeaders,
		ExposeHeaders:    p.config.CORS.ExposeHeaders,
		AllowCredentials: p.config.CORS.AllowCredentials,
		MaxAge:          p.config.CORS.MaxAge,
	}
	router.Use(middleware.SetupCORS(corsConfig))
	
	// Enhanced rate limiting middleware (if enabled)
	if p.config.RateLimit.Enabled {
		rateLimitConfig := ratelimit.Config{
			RedisClient:             p.redisClient,
			RequestsPerSecond:       p.config.RateLimit.RequestsPerSecond,
			BurstSize:               p.config.RateLimit.BurstSize,
			TenantEnabled:           p.config.RateLimit.TenantEnabled,
			TenantRequestsPerSecond: p.config.RateLimit.TenantRequestsPerSecond,
			TenantBurstSize:         p.config.RateLimit.TenantBurstSize,
			Algorithm:               ratelimit.TokenBucketAlgorithm,
			WindowSize:              time.Minute,
		}
		router.Use(ratelimit.Middleware(rateLimitConfig))
	}
	
	// Structured logging middleware (if enabled)
	if p.config.Observability.EnableMetrics {
		logger, _ := zap.NewProduction()
		loggingConfig := middleware.LoggingConfig{
			Logger:        logger,
			SkipPaths:     []string{"/health", "/ready", "/metrics"},
			EnableBody:    false,
			EnableHeaders: false,
			MaxBodySize:   1024,
		}
		router.Use(middleware.StructuredLoggingMiddleware(loggingConfig))
		
		// Audit logging for sensitive operations
		router.Use(middleware.AuditLoggingMiddleware(logger))
	}
	
	// Metrics middleware (if enabled)
	if p.config.Observability.EnableMetrics && p.metrics != nil {
		router.Use(observability.MetricsMiddleware(p.metrics))
	}
}

// AuthMiddleware returns a JWT authentication middleware
func (p *Platform) AuthMiddleware(requiredRole ...string) gin.HandlerFunc {
	role := ""
	if len(requiredRole) > 0 {
		role = requiredRole[0]
	}
	return auth.JWTMiddleware(p.jwksRegistry, role)
}

// OptionalAuthMiddleware returns an optional JWT authentication middleware
func (p *Platform) OptionalAuthMiddleware() gin.HandlerFunc {
	return auth.OptionalJWTMiddleware(p.jwksRegistry)
}

// TenantMiddleware returns a tenant isolation middleware
func (p *Platform) TenantMiddleware() gin.HandlerFunc {
	return auth.TenantMiddleware()
}

// RateLimitMiddleware returns a rate limiting middleware
func (p *Platform) RateLimitMiddleware() gin.HandlerFunc {
	if !p.config.RateLimit.Enabled || p.redisClient == nil {
		// Return a no-op middleware if rate limiting is disabled
		return func(c *gin.Context) { c.Next() }
	}
	
	// Use the rate limiting package
	limiterConfig := ratelimit.Config{
		RedisClient:    p.redisClient,
		RequestsPerSecond: p.config.RateLimit.RequestsPerSecond,
		BurstSize:      p.config.RateLimit.BurstSize,
	}
	
	return ratelimit.Middleware(limiterConfig)
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
	// Health check routes (already handled by middleware, but explicit routes)
	router.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"status":    "healthy",
			"timestamp": time.Now().UTC().Format(time.RFC3339),
		})
	})
	
	router.GET("/ready", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"status":    "ready", 
			"timestamp": time.Now().UTC().Format(time.RFC3339),
		})
	})
	
	// Metrics endpoint (if enabled)
	if p.config.Observability.EnableMetrics {
		observability.SetupMetricsEndpoint(router)
	}
	
	// User info endpoint
	router.GET("/whoami", p.AuthMiddleware(), auth.WhoAmIHandler)
	router.GET("/session", p.AuthMiddleware(), auth.SessionInfoHandler)
}

// GetConfig returns the platform configuration
func (p *Platform) GetConfig() Config {
	return p.config
}

// GetJWKSRegistry returns the JWKS registry
func (p *Platform) GetJWKSRegistry() *auth.JWKSRegistry {
	return p.jwksRegistry
}

// GetRedisClient returns the Redis client
func (p *Platform) GetRedisClient() *redis.Client {
	return p.redisClient
}

// GetMetrics returns the metrics instance
func (p *Platform) GetMetrics() *observability.Metrics {
	return p.metrics
}

// GetTracingProvider returns the tracing provider
func (p *Platform) GetTracingProvider() *observability.TracingProvider {
	return p.tracing
}

// Shutdown gracefully shuts down the platform resources
func (p *Platform) Shutdown(ctx context.Context) error {
	if p.tracing != nil {
		if err := p.tracing.Shutdown(ctx); err != nil {
			return fmt.Errorf("failed to shutdown tracing: %w", err)
		}
	}
	
	if p.redisClient != nil {
		if err := p.redisClient.Close(); err != nil {
			return fmt.Errorf("failed to close Redis client: %w", err)
		}
	}
	
	return nil
}
