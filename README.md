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
- Rate limiting with Redis backend

📊 **Observability**
- Prometheus metrics for all operations
- Request tracing and performance monitoring
- Health checks and readiness probes

🚀 **Developer Experience**
- Simple, unified API
- Gin middleware integration
- Comprehensive examples and documentation

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
        AppName:      "my-app",
        AppVersion:   "1.0.0",
    },
}
```

## Architecture

### Core Components

- **`pkg/auth`**: JWT validation, user management, RBAC
- **`pkg/middleware`**: Security, CORS, logging, recovery
- **`pkg/observability`**: Metrics, tracing, health checks
- **`pkg/platform`**: Unified API that combines all components
- **`pkg/config`**: Configuration management
- **`pkg/keycloak`**: Keycloak integration
- **`pkg/ratelimit`**: Rate limiting with Redis support
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

### Observability

Built-in metrics and monitoring:

- HTTP request metrics (duration, count, status codes)
- Authentication attempt tracking
- Rate limiting hit counts
- Application health and uptime
- Tenant-specific metrics

Access metrics at `/metrics` endpoint for Prometheus scraping.

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
