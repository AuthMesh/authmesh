# AuthMesh Minimal Example

A minimal AuthMesh application that shows how to get started with just a few lines of code.

## ✨ Features

- **3-line setup**: QuickStart API for instant platform initialization
- **Zero configuration**: Works out of the box with defaults
- **Standard endpoints**: Health checks, metrics, and auth endpoints included
- **Authentication ready**: JWT-based auth with role-based access control

## 🚀 Quick Start

### Prerequisites
- Go 1.21+
- Docker and Docker Compose (for Keycloak)

### 1. Start Keycloak (Optional)
```bash
# Start Keycloak for authentication (runs on port 9443)
docker-compose up -d keycloak

# Or run without external dependencies (public endpoints only)
go run main.go
```

### 2. Run the Application
```bash
go run main.go
```

The app starts on http://localhost:8080

## 📋 API Endpoints

### Public Endpoints (No Auth Required)
```bash
# Welcome message with API discovery
curl http://localhost:8080/

# Health checks (Kubernetes-style)
curl http://localhost:8080/health
curl http://localhost:8080/healthz
curl http://localhost:8080/ready
curl http://localhost:8080/readyz

# Custom hello endpoint
curl http://localhost:8080/hello

# Prometheus metrics
curl http://localhost:8080/metrics
```

### Protected Endpoints (Requires JWT Token)
```bash
# Get JWT token from Keycloak first
export TOKEN="your-jwt-token-here"

# Protected endpoint
curl -H "Authorization: Bearer $TOKEN" \
     http://localhost:8080/api/v1/protected

# User info endpoint
curl -H "Authorization: Bearer $TOKEN" \
     http://localhost:8080/whoami
```

## 🔑 Getting a JWT Token

### Option 1: Use Keycloak (Recommended)
1. Start Keycloak: `docker-compose up -d keycloak`
2. Open http://localhost:9443
3. Login with admin/admin
4. Create a user and get a token

### Option 2: Development Token
```bash
# For development/testing only - create a simple JWT
# (This would be provided by your identity provider in production)
```

## 🎯 Key Learning Points

1. **Minimal Setup**: See how AuthMesh reduces boilerplate to just 3 lines
2. **Standard Patterns**: Health checks, metrics, and auth follow industry standards
3. **Security by Default**: All best practices are enabled automatically
4. **Extensible**: Add your own routes while keeping all AuthMesh benefits

## 📚 Next Steps

- Try the [basic-app](../basic-app) example for custom configuration
- See [todo-api](../todo-api) for practical API examples with custom configuration
- Check out the [AuthMesh documentation](../../../docs/) for advanced features

## 🔄 Code Comparison

**Without AuthMesh** (40+ lines):
```go
router := gin.Default()

// Manual middleware setup
router.Use(cors.Default())
router.Use(recovery.Recovery())
router.Use(logging.Middleware())

// Manual health checks
router.GET("/health", healthHandler)
router.GET("/ready", readyHandler)

// Manual auth setup
authMiddleware := jwt.New(jwt.Config{...})
router.Use(authMiddleware)

// Manual metrics setup
prometheus.Setup(router)

// And much more...
```

**With AuthMesh** (3 lines):
```go
authMesh, _ := platform.QuickStart("minimal-app")
router := gin.Default()
authMesh.SetupAll(router) // Everything configured!
```

## 🛠️ What's Included Automatically

- ✅ **CORS** - Cross-origin resource sharing
- ✅ **Security Headers** - CSRF, XSS, content type protection
- ✅ **Rate Limiting** - Request throttling with Redis backend
- ✅ **JWT Authentication** - Keycloak integration with role-based access
- ✅ **Metrics** - Prometheus metrics collection
- ✅ **Health Checks** - Kubernetes-compatible health endpoints
- ✅ **Request Logging** - Structured logging with request IDs
- ✅ **Recovery** - Panic recovery with graceful error responses
- ✅ **Observability** - OpenTelemetry tracing ready
