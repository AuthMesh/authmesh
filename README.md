# AuthMesh - Go Multi-Tenant Authentication Library

AuthMesh is a production-ready Go library that provides comprehensive multi-tenant authentication, authorization, and security features. Built on battle-tested components, it offers JWT validation, role-based access control, rate limiting, and observability out of the box.

## Features

🔐 **JWT Authentication**
- Multi-tenant JWT validation with Keycloak integration
- Automatic JWKS key rotation and caching
- Support for multiple realms and issuers

🛡️ **Security & Authorization**
- Role-based access control (RBAC)
- Tenant isolation middleware
- SSRF protection and security headers
- Comprehensive input validation and sanitization
- Request ID tracking for audit trails

🚦 **Advanced Rate Limiting** ⭐ *Stage 2*
- Redis-based distributed rate limiting
- Per-tenant and per-user rate limiting
- Multiple algorithms: Token Bucket, Sliding Window, Fixed Window
- Burst handling and graceful degradation

📊 **Observability & Monitoring** ⭐ *Stage 2*
- Prometheus metrics for all operations
- OpenTelemetry distributed tracing support
- Request tracing and performance monitoring
- Health checks and readiness probes
- Structured logging with audit trails

� **Enhanced Security** ⭐ *Stage 2*
- CORS configuration with advanced options
- Security headers (HSTS, CSP, X-Frame-Options, etc.)
- Request/response sanitization
- Recovery middleware with structured logging

�🚀 **Developer Experience**
- Simple, unified API
- Gin middleware integration
- Comprehensive examples and documentation
- Docker Compose setup with full observability stack

## Quick Start

### Installation

```bash
go get github.com/AuthMesh/authmesh
```

### Basic Usage

```go
package main

import (
    "log"
    "github.com/AuthMesh/authmesh/pkg/platform"
    "github.com/gin-gonic/gin"
)

func main() {
    // Create platform with default configuration
    config := platform.DefaultConfig()
    config.Keycloak.URL = "http://localhost:9443"
    config.Keycloak.Realm = "your-realm"
    
    authMesh, err := platform.New(config)
    if err != nil {
        log.Fatal(err)
    }
    
    // Setup Gin router with middleware
    router := gin.Default()
    authMesh.SetupMiddleware(router)
    authMesh.SetupRoutes(router)
    
    // Protected route
    router.GET("/protected", authMesh.AuthMiddleware(), func(c *gin.Context) {
        c.JSON(200, gin.H{"message": "Hello authenticated user!"})
    })
    
    // Admin-only route
    router.GET("/admin", authMesh.AuthMiddleware("admin"), func(c *gin.Context) {
        c.JSON(200, gin.H{"message": "Admin access granted"})
    })
    
    // Tenant-specific routes
    tenantGroup := router.Group("/t/:tenant_id")
    tenantGroup.Use(authMesh.AuthMiddleware())
    tenantGroup.Use(authMesh.TenantMiddleware())
    {
        tenantGroup.GET("/dashboard", func(c *gin.Context) {
            tenantID := c.Param("tenant_id")
            c.JSON(200, gin.H{"tenant": tenantID, "message": "Tenant dashboard"})
        })
    }
    
    router.Run(":8080")
}
```

### Configuration

AuthMesh supports comprehensive configuration for production deployments:

```go
config := platform.Config{
    Keycloak: platform.KeycloakConfig{
        URL:            "https://your-keycloak.com",
        Realm:          "production",
        TrustedIssuers: []string{"https://your-keycloak.com"},
    },
    Redis: platform.RedisConfig{
        URL:      "redis://redis:6379",
        Password: "your-password",
        DB:       0,
    },
    Security: platform.SecurityConfig{
        EnableSSRFProtection:   true,
        AllowedHosts:          []string{"yourdomain.com"},
        BlockPrivateIPs:       true,
        EnableSecurityHeaders: true,
    },
    CORS: platform.CORSConfig{
        AllowOrigins:     []string{"https://yourdomain.com"},
        AllowMethods:     []string{"GET", "POST", "PUT", "DELETE"},
        AllowCredentials: true,
    },
    RateLimit: platform.RateLimitConfig{
        Enabled:           true,
        RequestsPerSecond: 100,
        BurstSize:        200,
        TenantEnabled:    true,
    },
    Observability: platform.ObservabilityConfig{
        EnableMetrics: true,
        EnableTracing: true,
        AppName:      "my-app",
        AppVersion:   "1.0.0",
        TracingConfig: platform.TracingConfig{
            ServiceName:    "my-app",
            ServiceVersion: "1.0.0",
            Environment:    "production",
            OTLPEndpoint:   "http://otel-collector:4318",
            SampleRate:     0.1, // 10% sampling
        },
    },
}
```

### Advanced Observability Configuration ⭐ *Stage 2*

AuthMesh provides comprehensive observability features with minimal configuration:

```go
config := platform.DefaultConfig()

// Enable full observability stack
config.Observability.EnableMetrics = true
config.Observability.EnableTracing = true

// Configure tracing with OTLP (recommended)
config.Observability.TracingConfig = platform.TracingConfig{
    ServiceName:    "my-service",
    ServiceVersion: "1.2.3",
    Environment:    "production",
    OTLPEndpoint:   "http://otel-collector:4318", // OpenTelemetry Collector
    SampleRate:     0.1, // 10% sampling for production
}

// Or configure with Jaeger directly
config.Observability.TracingConfig.JaegerURL = "http://jaeger:14268/api/traces"

authMesh, _ := platform.New(config)

// Tracing is automatically added to all routes
router.Use(authMesh.TracingMiddleware())

// Manual span creation example
func myHandler(c *gin.Context) {
    ctx := c.Request.Context()
    
    // Create custom spans for detailed tracing
    tracingProvider := authMesh.GetTracingProvider()
    ctx, span := tracingProvider.StartSpan(ctx, "business-logic")
    defer span.End()
    
    // Add attributes and events
    observability.SetAttribute(ctx, "user.id", "123")
    observability.AddEvent(ctx, "processing-started")
    
    // Your business logic here
    
    c.JSON(200, gin.H{"message": "success"})
}
}
```

## Quick Start with Full Observability Stack ⭐ *Stage 2*

Run the complete example with Keycloak, Redis, Prometheus, Grafana, and Jaeger:

```bash
# Clone and navigate to example
git clone https://github.com/AuthMesh/authmesh
cd authmesh/examples/basic-app

# Start the full stack
docker-compose up -d

# Wait for services to be ready (about 30 seconds)
docker-compose ps

# Test the application
curl http://localhost:8080/health  # Health check
curl http://localhost:8080/        # Public endpoint
```

### 🌐 **Access Points**

| Service | URL | Description |
|---------|-----|-------------|
| **Application** | http://localhost:8080 | Main AuthMesh application |
| **Keycloak** | http://localhost:9443 | Authentication server (admin/admin) |
| **Prometheus** | http://localhost:9090 | Metrics collection |
| **Grafana** | http://localhost:3000 | Dashboards (admin/admin) |
| **Jaeger** | http://localhost:16686 | Distributed tracing |

### 🔍 **Exploring Features**

```bash
# Test tracing demo with custom spans
curl http://localhost:8080/api/trace-demo

# View traces in Jaeger UI
open http://localhost:16686

# Check metrics in Prometheus
open http://localhost:9090/targets

# View dashboards in Grafana
open http://localhost:3000
```

### 🧪 **Testing Authentication**

```bash
# 1. Get a JWT token from Keycloak (configure realm first)
# 2. Use token in Authorization header
curl -H "Authorization: Bearer YOUR_JWT_TOKEN" \
     http://localhost:8080/protected

# 3. Test tenant isolation
curl -H "Authorization: Bearer YOUR_JWT_TOKEN" \
     http://localhost:8080/t/tenant1/dashboard
```

## Architecture

### Core Components

- **`pkg/auth`**: JWT validation, user management, RBAC
- **`pkg/middleware`**: Security, CORS, logging, recovery
- **`pkg/observability`**: Metrics, tracing, health checks ⭐ *Enhanced*
- **`pkg/platform`**: Unified API that combines all components
- **`pkg/config`**: Configuration management
- **`pkg/keycloak`**: Keycloak integration
- **`pkg/ratelimit`**: Distributed Redis-based rate limiting ⭐ *Enhanced*
- **`pkg/tlsutil`**: TLS configuration utilities

### Multi-Tenant Support

AuthMesh provides built-in multi-tenant isolation:

```go
// Tenant-specific routes automatically enforce isolation
router.Group("/t/:tenant_id").Use(
    authMesh.AuthMiddleware(),
    authMesh.TenantMiddleware(), // Enforces tenant boundary
)

// Super admin routes for cross-tenant operations
router.Group("/admin").Use(authMesh.SuperAdminMiddleware())
```

### Enhanced Observability ⭐ *Stage 2*

Comprehensive monitoring and tracing:

**Metrics Available:**
- HTTP request metrics (duration, count, status codes)
- Authentication attempt tracking
- Rate limiting hit counts (per tenant/user)
- Application health and uptime
- Tenant-specific metrics

**Distributed Tracing:**
- Automatic request tracing
- Manual span creation for business logic
- Integration with Jaeger and OTLP collectors
- Performance bottleneck identification

**Access Points:**
- Metrics: `/metrics` endpoint for Prometheus scraping
- Health: `/health` and `/ready` endpoints
- Tracing: Automatic headers injection and span propagation

```go
// Example: Adding custom tracing to business logic
func businessHandler(c *gin.Context) {
    ctx := c.Request.Context()
    
    // Create custom spans for detailed tracing
    tracingProvider := authMesh.GetTracingProvider()
    ctx, span := tracingProvider.StartSpan(ctx, "business-operation")
    defer span.End()
    
    // Add custom attributes
    observability.SetAttribute(ctx, "operation.type", "data-processing")
    observability.AddEvent(ctx, "processing-started")
    
    // Your business logic with automatic tracing
    result := processData()
    
    observability.AddEvent(ctx, "processing-completed")
    c.JSON(200, result)
}

## Examples

Check the `examples/` directory for complete working examples:

- **`basic-app/`**: Simple authentication setup
- **`advanced-app/`**: Advanced features and custom middleware
- **`migration-examples/`**: Migration guides from other auth libraries

## Testing

Run the test suite:

```bash
go test ./...
```

Run with coverage:

```bash
go test -cover ./...
```

## Production Readiness

AuthMesh is built for production with:

- ✅ Comprehensive test coverage
- ✅ Security best practices
- ✅ Performance optimizations
- ✅ Graceful error handling
- ✅ Observability and monitoring
- ✅ Docker and Kubernetes ready

### Performance Targets

- JWT validation: <1ms per token
- Rate limiting: <0.5ms per request
- Memory usage: <50MB for typical workloads
- 99.9% uptime SLA compliance

## Security

- SSRF protection against malicious URL parameters
- Comprehensive security headers
- Input validation and sanitization
- Rate limiting to prevent abuse
- Secure defaults and configuration validation

## Contributing

1. Fork the repository
2. Create a feature branch
3. Add tests for new functionality
4. Run the test suite
5. Submit a pull request

## License

Apache 2.0 License - see [LICENSE](LICENSE) for details.

## Support

- 📖 [Documentation](docs/)
- 🐛 [Issue Tracker](https://github.com/AuthMesh/authmesh/issues)
- 💬 [Discussions](https://github.com/AuthMesh/authmesh/discussions)

---

Built with ❤️ for the Go community
