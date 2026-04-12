# AuthMesh QuickStart Example

The fastest way to get started with AuthMesh - demonstrates the **QuickStart API** for instant platform setup.

## ✨ What You Get in 3 Lines

```go
authMesh, _ := platform.QuickStart("my-app")
router := gin.Default()
authMesh.SetupAll(router) // Everything configured instantly!
```

**Automatic Setup Includes**:
- ✅ **Authentication**: JWT validation with Keycloak
- ✅ **Security**: CORS, security headers, SSRF protection  
- ✅ **Rate Limiting**: Redis-backed request throttling
- ✅ **Observability**: Prometheus metrics + health checks
- ✅ **Standard Routes**: Health, metrics, auth endpoints
- ✅ **Middleware**: Recovery, logging, request IDs

## 🚀 Quick Start

### Prerequisites
- Go 1.21+
- Docker and Docker Compose (optional, for full features)

### 1. Run with Defaults
```bash
# Works immediately with built-in defaults
go run main.go
```

### 2. Run with Full Infrastructure (Recommended)
```bash
# Start Keycloak for authentication
docker-compose up -d

# Run the application
go run main.go
```

The app starts on http://localhost:8080

## 📋 Available Endpoints

### Public Endpoints
```bash
# Welcome with API discovery
curl http://localhost:8080/

# Health checks (Kubernetes-style)
curl http://localhost:8080/health
curl http://localhost:8080/healthz
curl http://localhost:8080/ready
curl http://localhost:8080/readyz

# Prometheus metrics
curl http://localhost:8080/metrics
```

### Protected Endpoints (Requires JWT)
```bash
export TOKEN="your-jwt-token"

# User profile
curl -H "Authorization: Bearer $TOKEN" \
     http://localhost:8080/api/profile

# User info
curl -H "Authorization: Bearer $TOKEN" \
     http://localhost:8080/whoami
```

### Admin Endpoints (Requires Admin Role)
```bash
export ADMIN_TOKEN="your-admin-jwt-token"

# Admin dashboard  
curl -H "Authorization: Bearer $ADMIN_TOKEN" \
     http://localhost:8080/api/admin

# Standard admin info
curl -H "Authorization: Bearer $ADMIN_TOKEN" \
     http://localhost:8080/api/v1/admin/info
```

### Multi-Tenant Endpoints
```bash
# Tenant dashboard
curl -H "Authorization: Bearer $TOKEN" \
     http://localhost:8080/t/tenant1/dashboard

# Standard tenant info
curl -H "Authorization: Bearer $TOKEN" \
     http://localhost:8080/t/tenant1/api/v1/info
```

## 🔑 Getting JWT Tokens

### Option 1: Keycloak (Full Setup)
```bash
# Get user token
curl -X POST http://localhost:9443/realms/master/protocol/openid-connect/token \
  -H "Content-Type: application/x-www-form-urlencoded" \
  -d "grant_type=password" \
  -d "client_id=account" \
  -d "username=testuser" \
  -d "password=testpass"

# Extract the access_token from response
```

### Option 2: Development Mode
```bash
# Run without Keycloak - public endpoints only
go run main.go

# Authentication endpoints will return 503 (service unavailable)
# but all public endpoints work perfectly
```

## 🎯 Key Learning Points

### 1. **Zero Configuration Required**
```go
// This is literally all you need!
authMesh, _ := platform.QuickStart("my-app")
router := gin.Default()
authMesh.SetupAll(router)
```

### 2. **Production-Ready Defaults**
- CORS configured for common development ports
- Rate limiting: 100 req/sec with 200 burst
- Security headers enabled
- Health checks follow Kubernetes patterns

### 3. **Extensible Foundation**
```go
// Add your routes on top of the foundation
router.GET("/custom", func(c *gin.Context) {
    // Your business logic
})

// Use AuthMesh helpers
protected := router.Group("/api")
protected.Use(authMesh.AuthMiddleware())
```

## 🔄 Code Architecture

```
quickstart-app/
├── main.go                # Application entry point
├── docker-compose.yml     # Optional Keycloak setup
└── README.md             # This file
```

**main.go Structure**:
1. **QuickStart**: `platform.QuickStart()` - one line initialization
2. **Setup**: `authMesh.SetupAll()` - one line middleware + routes  
3. **Custom Routes**: Add your application logic

## 📊 What Happens Under the Hood

When you call `platform.QuickStart("my-app")`, AuthMesh:

1. **Creates Default Config**: Sensible defaults for all components
2. **Initializes Services**: Redis connection, metrics, security
3. **Sets Up Dependencies**: JWKS registry, rate limiters, observability

When you call `authMesh.SetupAll(router)`, AuthMesh:

1. **Adds Middleware**: Recovery, CORS, security, auth, metrics, logging
2. **Registers Routes**: Health checks, metrics, whoami, admin endpoints
3. **Configures Handlers**: Error handling, graceful shutdown

## 🆚 Comparison with Other Examples

| Aspect | QuickStart | Minimal | Todo API | Advanced |
|--------|------------|---------|----------|-----------|
| **Setup** | `QuickStart()` | `QuickStart()` | `New(config)` | `New(config)` |
| **Lines** | 3 | 3 | ~15 | ~50 |
| **Config** | Zero | Zero | Custom | Production |
| **Features** | All defaults | Basic | Customized | Advanced |
| **Use Case** | Learning | Prototyping | Development | Production |

## 📚 Next Steps

1. **Understand the Magic**: Look at [minimal-app](../minimal-app) for the simplest possible setup
2. **Customize Configuration**: Try [todo-api](../todo-api) for custom configuration
3. **Production Patterns**: Study [basic-app](../basic-app) for advanced features and production patterns
4. **Advanced Features**: Check the [AuthMesh documentation](../../docs/) for comprehensive guides

## 🤔 When to Use QuickStart vs Custom Config

**Use QuickStart when**:
- ✅ Learning AuthMesh
- ✅ Building prototypes
- ✅ Default settings work for you
- ✅ Want fastest time-to-value

**Use Custom Config when**:
- ✅ Production deployment
- ✅ Specific security requirements  
- ✅ Custom rate limits or CORS
- ✅ Multiple Keycloak realms

## Available Convenience Methods

- `platform.QuickStart(name)` - Full featured setup for demos
- `platform.NewWithDefaults(keycloak, redis)` - Simple production setup  
- `platform.NewWithObservability(keycloak, redis, name)` - With tracing
- `platform.NewProduction(config)` - Production hardened setup

## Services

| Service | URL | Credentials |
|---------|-----|-------------|
| Application | http://localhost:8080 | - |
| Keycloak | http://localhost:9443 | admin/admin |
| Jaeger | http://localhost:16686 | - |

## Architecture

The simple example demonstrates the unified AuthMesh API that abstracts away complexity while maintaining full configurability when needed.
