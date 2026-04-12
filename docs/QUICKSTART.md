# Quick Start Guide

Get AuthMesh running in under 5 minutes with this step-by-step guide.

## Prerequisites

- Go 1.21 or higher
- Docker and Docker Compose (for full stack)
- Git

## Option 1: Simple Setup (5 lines of code)

### 1. Install AuthMesh

```bash
go mod init myapp
go get github.com/AuthMesh/authmesh
```

### 2. Create main.go

```go
package main

import (
    "log"
    "github.com/AuthMesh/authmesh/pkg/platform"
    "github.com/gin-gonic/gin"
)

func main() {
    // 1. Initialize AuthMesh
    authMesh, err := platform.NewWithDefaults(
        "http://localhost:9443",  // Keycloak URL
        "redis://localhost:6379", // Redis URL  
    )
    if err != nil {
        log.Fatal(err)
    }
    
    // 2. Setup router
    router := gin.Default()
    authMesh.SetupAll(router)
    
    // 3. Add protected route
    router.GET("/protected", authMesh.AuthMiddleware(), func(c *gin.Context) {
        c.JSON(200, gin.H{"message": "Hello authenticated user!"})
    })
    
    // 4. Start server
    router.Run(":8080")
}
```

### 3. Run

```bash
go run main.go
```

Your API is now running with:
- ✅ JWT Authentication
- ✅ Rate Limiting  
- ✅ Security Headers
- ✅ Health Checks
- ✅ Metrics

## Option 2: Full Stack with Keycloak (Docker)

### 1. Clone Example

```bash
git clone https://github.com/AuthMesh/authmesh
cd authmesh/examples/basic-app
```

### 2. Start Services

```bash
docker-compose up -d
```

### 3. Wait for Services (30 seconds)

```bash
# Check service status
docker-compose ps

# Verify health
curl http://localhost:8080/health
```

### 4. Test Authentication

```bash
# Public endpoint
curl http://localhost:8080/

# Protected endpoint (requires JWT token)
curl -H "Authorization: Bearer YOUR_JWT_TOKEN" \
     http://localhost:8080/protected
```

## Access Points

| Service | URL | Credentials |
|---------|-----|-------------|
| **Application** | http://localhost:8080 | - |
| **Keycloak Admin** | http://localhost:9443 | admin/admin |
| **Prometheus** | http://localhost:9090 | - |
| **Grafana** | http://localhost:3000 | admin/admin |
| **Jaeger** | http://localhost:16686 | - |

## Getting JWT Tokens

### Method 1: Direct Keycloak API

```bash
# Get token from Keycloak
curl -X POST "http://localhost:9443/realms/master/protocol/openid-connect/token" \
  -H "Content-Type: application/x-www-form-urlencoded" \
  -d "grant_type=password" \
  -d "client_id=admin-cli" \
  -d "username=admin" \
  -d "password=admin"
```

### Method 2: Using Keycloak Admin UI

1. Visit http://localhost:9443
2. Login with admin/admin
3. Go to Clients → Create Client
4. Configure your application
5. Generate tokens in the admin interface

## Configuration Options

### Environment Variables

```bash
# Core settings
export KEYCLOAK_URL="http://localhost:9443"
export REDIS_URL="redis://localhost:6379"

# Security
export TRUSTED_ISSUERS="http://localhost:9443"
export CORS_ALLOWED_ORIGINS="http://localhost:3000"

# Performance
export RATE_LIMIT_REQUESTS_PER_SECOND="100"
export JWT_CACHE_TTL="3600"

# Observability
export METRICS_ENABLED="true"
export TRACING_ENABLED="true"
```

### Programmatic Configuration

```go
// Custom configuration
config := platform.Config{
    Keycloak: platform.KeycloakConfig{
        URL:            "https://your-keycloak.com",
        Realm:          "your-realm",
        TrustedIssuers: []string{"https://your-keycloak.com"},
    },
    Redis: platform.RedisConfig{
        URL: "redis://your-redis:6379",
    },
    Security: platform.SecurityConfig{
        EnableSSRFProtection:   true,
        BlockPrivateIPs:       true,
        EnableSecurityHeaders: true,
    },
    RateLimit: platform.RateLimitConfig{
        Enabled:           true,
        RequestsPerSecond: 100,
        BurstSize:        200,
    },
}

authMesh, err := platform.New(config)
```

## Next Steps

1. **Security**: Review [SECURITY.md](SECURITY.md) for production hardening
2. **Performance**: Check [PERFORMANCE.md](PERFORMANCE.md) for optimization tips
3. **Migration**: See [MIGRATION.md](MIGRATION.md) if migrating from another auth system
4. **API Reference**: Browse [API_REFERENCE.md](API_REFERENCE.md) for detailed API docs

## Common Issues

### Issue: Connection Refused
**Solution**: Ensure Keycloak is running and accessible
```bash
curl http://localhost:9443/health  # Should return 200
```

### Issue: JWT Validation Fails
**Solution**: Check trusted issuers configuration
```go
config.Keycloak.TrustedIssuers = []string{
    "http://localhost:9443",  // Must match token issuer
}
```

### Issue: Rate Limiting Too Strict
**Solution**: Adjust rate limits
```go
config.RateLimit.RequestsPerSecond = 1000  // Increase limit
config.RateLimit.BurstSize = 2000          // Allow bursts
```

## Support

- 📖 [Full Documentation](../README.md)
- 🐛 [Report Issues](https://github.com/AuthMesh/authmesh/issues)
- 💬 [Community Discussion](https://github.com/AuthMesh/authmesh/issues)
