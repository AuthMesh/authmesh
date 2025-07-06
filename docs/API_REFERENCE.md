# API Reference

## Overview

AuthMesh provides a comprehensive API for multi-tenant authentication and authorization. This reference covers all public interfaces, configuration options, and usage patterns.

## Core Package: `platform`

### QuickStart

The simplest way to get started with AuthMesh.

```go
func QuickStart(appName string) (*Platform, error)
```

**Example:**
```go
authMesh, err := platform.QuickStart("myapp")
if err != nil {
    log.Fatal(err)
}

router := gin.New()
authMesh.SetupAll(router)
```

### Constructors

#### NewWithDefaults

```go
func NewWithDefaults(appName string, opts ...Option) (*Platform, error)
```

Creates a platform with sensible defaults and optional customizations.

**Example:**
```go
authMesh, err := platform.NewWithDefaults("myapp",
    platform.WithKeycloak("https://auth.example.com"),
    platform.WithRateLimiting(platform.RateLimitConfig{
        Global:  1000,
        PerUser: 100,
    }),
)
```

#### NewWithObservability

```go
func NewWithObservability(appName string, config Config) (*Platform, error)
```

Creates a platform with full observability stack (metrics, tracing, logging).

**Example:**
```go
config := platform.Config{
    KeycloakURL: "https://auth.example.com",
    ObservabilityConfig: platform.ObservabilityConfig{
        MetricsEnabled: true,
        TracingEnabled: true,
        LogLevel:      "info",
    },
}

authMesh, err := platform.NewWithObservability("myapp", config)
```

#### NewProduction

```go
func NewProduction(config Config) (*Platform, error)
```

Creates a production-ready platform with security hardening and performance optimizations.

**Example:**
```go
config := platform.Config{
    KeycloakURL: "https://auth.example.com",
    TrustedIssuers: []string{"https://auth.example.com"},
    HTTPTimeout: 5 * time.Second,
    TLSConfig: &tls.Config{
        MinVersion: tls.VersionTLS12,
    },
    SecurityConfig: platform.SecurityConfig{
        EnableCORS:        true,
        EnableCSP:         true,
        EnableRateLimiting: true,
    },
}

authMesh, err := platform.NewProduction(config)
```

#### NewForTesting

```go
func NewForTesting(testName string) (*Platform, error)
```

Creates a platform optimized for testing with mocked dependencies and faster timeouts.

**Example:**
```go
// In your test files
func TestMyHandler(t *testing.T) {
    authMesh, err := platform.NewForTesting("handler-test")
    require.NoError(t, err)
    
    router := gin.New()
    authMesh.SetupMiddleware(router)
    // ... test code
}
```

### Configuration

#### Config Structure

```go
type Config struct {
    // Core settings
    KeycloakURL     string        `json:"keycloak_url"`
    DefaultRealm    string        `json:"default_realm"`
    TrustedIssuers  []string      `json:"trusted_issuers"`
    HTTPTimeout     time.Duration `json:"http_timeout"`
    
    // Security settings
    TLSConfig      *tls.Config    `json:"-"`
    SecurityConfig SecurityConfig `json:"security_config"`
    
    // Rate limiting
    RateLimiting RateLimitConfig `json:"rate_limiting"`
    
    // Observability
    ObservabilityConfig ObservabilityConfig `json:"observability_config"`
    
    // Testing options
    SkipJWKSInit bool `json:"skip_jwks_init"`
}
```

#### SecurityConfig

```go
type SecurityConfig struct {
    EnableCORS         bool              `json:"enable_cors"`
    EnableCSP          bool              `json:"enable_csp"`
    EnableRateLimiting bool              `json:"enable_rate_limiting"`
    EnableSSRFProtection bool            `json:"enable_ssrf_protection"`
    AllowedOrigins     []string          `json:"allowed_origins"`
    TrustedHosts       []string          `json:"trusted_hosts"`
    CSPPolicy          string            `json:"csp_policy"`
}
```

#### RateLimitConfig

```go
type RateLimitConfig struct {
    Enabled   bool `json:"enabled"`
    Global    int  `json:"global"`     // Requests per minute globally
    PerUser   int  `json:"per_user"`   // Per authenticated user
    PerIP     int  `json:"per_ip"`     // Per IP address
    PerTenant int  `json:"per_tenant"` // Per tenant
}
```

#### ObservabilityConfig

```go
type ObservabilityConfig struct {
    MetricsEnabled    bool   `json:"metrics_enabled"`
    TracingEnabled    bool   `json:"tracing_enabled"`
    LoggingEnabled    bool   `json:"logging_enabled"`
    LogLevel         string `json:"log_level"`
    MetricsAddr      string `json:"metrics_addr"`
    TracingEndpoint  string `json:"tracing_endpoint"`
}
```

### Platform Methods

#### SetupAll

```go
func (p *Platform) SetupAll(router *gin.Engine)
```

Sets up all middleware and routes including authentication, authorization, rate limiting, observability, and health checks.

**Example:**
```go
router := gin.New()
authMesh.SetupAll(router)

// All routes are now protected and monitored
router.GET("/api/users", getUsersHandler)
```

#### SetupCore

```go
func (p *Platform) SetupCore(router *gin.Engine)
```

Sets up only essential middleware (authentication, basic logging).

#### SetupMiddleware

```go
func (p *Platform) SetupMiddleware(router *gin.Engine)
```

Sets up middleware without health/metrics endpoints.

#### SetupRoutes

```go
func (p *Platform) SetupRoutes(router *gin.Engine)
```

Sets up only health and metrics endpoints.

### Middleware Methods

#### RequireAuth

```go
func (p *Platform) RequireAuth() gin.HandlerFunc
```

Requires valid JWT authentication.

**Example:**
```go
api := router.Group("/api")
api.Use(authMesh.RequireAuth())
api.GET("/protected", protectedHandler)
```

#### RequireRole

```go
func (p *Platform) RequireRole(role string) gin.HandlerFunc
```

Requires specific role in JWT claims.

**Example:**
```go
admin := router.Group("/admin")
admin.Use(authMesh.RequireAuth())
admin.Use(authMesh.RequireRole("admin"))
admin.DELETE("/users/:id", deleteUserHandler)
```

#### RequireAnyRole

```go
func (p *Platform) RequireAnyRole(roles ...string) gin.HandlerFunc
```

Requires any of the specified roles.

**Example:**
```go
moderation := router.Group("/moderation")
moderation.Use(authMesh.RequireAuth())
moderation.Use(authMesh.RequireAnyRole("admin", "moderator"))
moderation.POST("/ban", banUserHandler)
```

#### TenantIsolation

```go
func (p *Platform) TenantIsolation() gin.HandlerFunc
```

Enforces tenant isolation for multi-tenant applications.

**Example:**
```go
tenantAPI := router.Group("/tenant/:tenantId")
tenantAPI.Use(authMesh.RequireAuth())
tenantAPI.Use(authMesh.TenantIsolation())
tenantAPI.GET("/data", getTenantDataHandler)
```

### Context Helpers

#### GetUserContext

```go
func GetUserContext(c *gin.Context) (*UserContext, bool)
```

Retrieves user information from authenticated request.

**Example:**
```go
func getUserProfile(c *gin.Context) {
    user, exists := platform.GetUserContext(c)
    if !exists {
        c.JSON(401, gin.H{"error": "unauthorized"})
        return
    }
    
    c.JSON(200, gin.H{
        "user_id": user.UserID,
        "tenant":  user.TenantID,
        "roles":   user.Roles,
    })
}
```

#### UserContext Structure

```go
type UserContext struct {
    UserID   string   `json:"user_id"`
    TenantID string   `json:"tenant_id"`
    Roles    []string `json:"roles"`
    Email    string   `json:"email"`
    Username string   `json:"username"`
    Claims   map[string]interface{} `json:"claims"`
}
```

## Authentication Package: `auth`

### JWT Validation

#### NewJWKSRegistry

```go
func NewJWKSRegistry(keycloakURL, defaultRealm string, trustedIssuers []string) *JWKSRegistry
```

Creates a new JWKS registry for JWT validation.

**Example:**
```go
registry := auth.NewJWKSRegistry(
    "https://auth.example.com",
    "master",
    []string{"https://auth.example.com"},
)
```

#### ValidateToken

```go
func (j *JWKSRegistry) ValidateToken(tokenString string) (*jwt.Token, error)
```

Validates a JWT token using JWKS.

### RBAC

#### HasRole

```go
func HasRole(claims jwt.MapClaims, role string) bool
```

Checks if JWT claims contain a specific role.

**Example:**
```go
if auth.HasRole(claims, "admin") {
    // Grant admin access
}
```

#### HasAnyRole

```go
func HasAnyRole(claims jwt.MapClaims, roles ...string) bool
```

Checks if JWT claims contain any of the specified roles.

#### GetTenantFromClaims

```go
func GetTenantFromClaims(claims jwt.MapClaims) (string, error)
```

Extracts tenant ID from JWT claims.

## Rate Limiting Package: `ratelimit`

### In-Memory Rate Limiting

#### NewTokenBucket

```go
func NewTokenBucket(rate, burst int) *TokenBucket
```

Creates a new token bucket rate limiter.

**Example:**
```go
limiter := ratelimit.NewTokenBucket(100, 200) // 100/sec, burst 200
if limiter.Allow() {
    // Process request
} else {
    // Rate limited
}
```

### Redis Rate Limiting

#### NewRedisLimiter

```go
func NewRedisLimiter(client *redis.Client) *RedisLimiter
```

Creates a Redis-based distributed rate limiter.

**Example:**
```go
redisClient := redis.NewClient(&redis.Options{
    Addr: "localhost:6379",
})

limiter := ratelimit.NewRedisLimiter(redisClient)
allowed, remaining, resetTime := limiter.Allow(ctx, "user:123", 100, time.Minute)
```

### Middleware

#### RateLimitMiddleware

```go
func RateLimitMiddleware(limiter Limiter, keyFunc KeyFunction) gin.HandlerFunc
```

Creates rate limiting middleware.

**Example:**
```go
// Rate limit by IP
ipLimiter := ratelimit.RateLimitMiddleware(
    limiter,
    func(c *gin.Context) string {
        return c.ClientIP()
    },
)

router.Use(ipLimiter)
```

## Keycloak Package: `keycloak`

### Client

#### NewClient

```go
func NewClient(config ClientConfig) (*Client, error)
```

Creates a new Keycloak client.

**Example:**
```go
client, err := keycloak.NewClient(keycloak.ClientConfig{
    BaseURL:  "https://auth.example.com",
    Username: "admin",
    Password: "admin123",
    Realm:    "master",
})
```

### User Management

#### CreateUser

```go
func (c *Client) CreateUser(ctx context.Context, realm string, user User) (string, error)
```

Creates a new user in Keycloak.

**Example:**
```go
user := keycloak.User{
    Username: "john.doe",
    Email:    "john@example.com",
    Enabled:  true,
    Attributes: map[string][]string{
        "tenant": {"tenant1"},
    },
}

userID, err := client.CreateUser(ctx, "master", user)
```

#### GetUser

```go
func (c *Client) GetUser(ctx context.Context, realm, userID string) (*User, error)
```

#### UpdateUser

```go
func (c *Client) UpdateUser(ctx context.Context, realm, userID string, user User) error
```

#### DeleteUser

```go
func (c *Client) DeleteUser(ctx context.Context, realm, userID string) error
```

### Role Management

#### CreateRole

```go
func (c *Client) CreateRole(ctx context.Context, realm string, role Role) error
```

#### AssignRole

```go
func (c *Client) AssignRole(ctx context.Context, realm, userID, roleName string) error
```

#### RemoveRole

```go
func (c *Client) RemoveRole(ctx context.Context, realm, userID, roleName string) error
```

## Observability Package: `observability`

### Metrics

#### SetupMetrics

```go
func SetupMetrics(registry *prometheus.Registry) *Metrics
```

Sets up Prometheus metrics collection.

**Example:**
```go
registry := prometheus.NewRegistry()
metrics := observability.SetupMetrics(registry)

// Metrics are automatically collected by middleware
```

### Tracing

#### SetupTracing

```go
func SetupTracing(serviceName, endpoint string) (trace.TracerProvider, error)
```

Sets up distributed tracing with OTLP.

**Example:**
```go
tracerProvider, err := observability.SetupTracing(
    "myapp",
    "http://jaeger:14268/api/traces",
)
```

### Health Checks

#### HealthCheck

```go
func HealthCheck() gin.HandlerFunc
```

Returns a health check handler.

**Example:**
```go
router.GET("/health", observability.HealthCheck())
```

## Configuration Package: `config`

### Environment Configuration

#### LoadFromEnv

```go
func LoadFromEnv() (*Config, error)
```

Loads configuration from environment variables.

**Supported Environment Variables:**

| Variable | Description | Default |
|----------|-------------|---------|
| `KEYCLOAK_URL` | Keycloak server URL | Required |
| `TRUSTED_ISSUERS` | Comma-separated trusted issuers | Same as KEYCLOAK_URL |
| `DEFAULT_REALM` | Default Keycloak realm | "master" |
| `HTTP_TIMEOUT` | HTTP client timeout | "5s" |
| `RATE_LIMIT_ENABLED` | Enable rate limiting | "true" |
| `RATE_LIMIT_GLOBAL` | Global rate limit | "1000" |
| `RATE_LIMIT_PER_USER` | Per-user rate limit | "100" |
| `RATE_LIMIT_PER_IP` | Per-IP rate limit | "50" |
| `METRICS_ENABLED` | Enable metrics | "true" |
| `TRACING_ENABLED` | Enable tracing | "false" |
| `LOG_LEVEL` | Log level | "info" |

**Example:**
```bash
export KEYCLOAK_URL="https://auth.example.com"
export TRUSTED_ISSUERS="https://auth.example.com,https://auth2.example.com"
export RATE_LIMIT_GLOBAL="2000"
```

```go
config, err := config.LoadFromEnv()
if err != nil {
    log.Fatal(err)
}

authMesh, err := platform.NewProduction(*config)
```

## Error Handling

### Common Errors

#### AuthenticationError

```go
type AuthenticationError struct {
    Message string
    Code    int
}
```

Returned when JWT validation fails.

#### AuthorizationError

```go
type AuthorizationError struct {
    RequiredRole string
    UserRoles    []string
}
```

Returned when user lacks required permissions.

#### RateLimitError

```go
type RateLimitError struct {
    Limit     int
    Remaining int
    ResetTime time.Time
}
```

Returned when rate limit is exceeded.

### Error Handling Example

```go
func handleErrors() gin.HandlerFunc {
    return func(c *gin.Context) {
        c.Next()
        
        for _, err := range c.Errors {
            switch e := err.Err.(type) {
            case *platform.AuthenticationError:
                c.JSON(401, gin.H{
                    "error": "authentication_failed",
                    "message": e.Message,
                })
            case *platform.AuthorizationError:
                c.JSON(403, gin.H{
                    "error": "insufficient_permissions",
                    "required_role": e.RequiredRole,
                })
            case *platform.RateLimitError:
                c.Header("X-RateLimit-Limit", strconv.Itoa(e.Limit))
                c.Header("X-RateLimit-Remaining", strconv.Itoa(e.Remaining))
                c.Header("X-RateLimit-Reset", strconv.FormatInt(e.ResetTime.Unix(), 10))
                c.JSON(429, gin.H{
                    "error": "rate_limit_exceeded",
                    "retry_after": e.ResetTime.Sub(time.Now()).Seconds(),
                })
            default:
                c.JSON(500, gin.H{
                    "error": "internal_server_error",
                })
            }
            return
        }
    }
}
```

## Testing Utilities

### Test Helpers

#### CreateTestToken

```go
func CreateTestToken(claims map[string]interface{}) (string, error)
```

Creates a test JWT token for testing.

**Example:**
```go
token, err := testutils.CreateTestToken(map[string]interface{}{
    "sub": "user123",
    "tenant": "tenant1",
    "roles": []string{"user", "admin"},
    "exp": time.Now().Add(time.Hour).Unix(),
})
```

#### WaitForServices

```go
func WaitForServices(baseURL string, timeout time.Duration) error
```

Waits for services to be ready (useful in E2E tests).

### Mock Client

```go
type MockKeycloakClient struct {
    Users map[string]User
    Roles map[string]Role
}
```

Mock Keycloak client for testing.

## Version Information

```go
// Version returns the current AuthMesh version
func Version() string

// APIVersion returns the API version
func APIVersion() string

// VersionInfo returns formatted version information
func VersionInfo() string
```

**Example:**
```go
fmt.Printf("Running %s\n", authmesh.VersionInfo())
// Output: AuthMesh v1.0.0 (commit: abc123, built: 2024-01-01, go: 1.21+)
```

## Best Practices

### Configuration

1. **Use environment variables** for configuration in production
2. **Enable observability** for monitoring and debugging
3. **Configure rate limiting** appropriate to your use case
4. **Use TLS 1.2+** for all connections
5. **Set reasonable timeouts** for HTTP clients

### Security

1. **Validate all JWT claims** including issuer and expiration
2. **Use role-based authorization** for granular access control
3. **Enable CORS protection** for web applications
4. **Implement rate limiting** to prevent abuse
5. **Monitor authentication failures** for security threats

### Performance

1. **Cache JWKS** to reduce external calls
2. **Use Redis** for distributed rate limiting
3. **Enable connection pooling** for HTTP clients
4. **Profile regularly** to identify bottlenecks
5. **Monitor metrics** for performance degradation

### Testing

1. **Use NewForTesting** in test environments
2. **Mock external dependencies** for unit tests
3. **Test authorization logic** thoroughly
4. **Validate rate limiting** behavior
5. **Include load testing** in CI/CD pipeline
