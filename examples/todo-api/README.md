
# AuthMesh Todo API Example

A practical Todo API that demonstrates AuthMesh's custom configuration, RBAC, rate limiting, CORS, and observability features in a real-world pattern.

## ✨ Features

- **Custom Configuration**: Shows how to configure AuthMesh for your needs (Keycloak, Redis, CORS, rate limiting, observability)
- **Role-Based Access**: User and admin endpoints with proper authorization (see `/api/v1/todos` and `/api/v1/admin`)
- **Rate Limiting**: Custom rate limiting config, demo-friendly limits
- **CORS Setup**: Multi-frontend support (React, Vue, Angular)
- **Observability**: Prometheus metrics and OpenTelemetry tracing enabled
- **RESTful API**: Complete CRUD operations for todos (in-memory for demo)

## 🚀 Quick Start

### Prerequisites
- Go 1.21+
- Docker and Docker Compose

docker-compose up -d

### 1. Start Infrastructure
```bash
# Start Keycloak, Redis, and Prometheus
docker-compose up -d

# Wait for services to be ready
sleep 20
```


### 2. Run the Application
```bash
go run main.go
```

The API starts on http://localhost:8080

## 📋 API Endpoints

### Public Endpoints
```bash
# API information
curl http://localhost:8080/api

# Health checks
curl http://localhost:8080/health
curl http://localhost:8080/healthz

# Welcome message
curl http://localhost:8080/
```


### User Endpoints (Requires JWT Token)
```bash
export TOKEN="your-jwt-token-here"

# List your todos
curl -H "Authorization: Bearer $TOKEN" \
     http://localhost:8080/api/v1/todos

# Create a new todo
curl -X POST \
     -H "Authorization: Bearer $TOKEN" \
     -H "Content-Type: application/json" \
     -d '{"title": "Learn AuthMesh"}' \
     http://localhost:8080/api/v1/todos

# Toggle todo completion (replace 1 with your todo ID)
curl -X PUT \
     -H "Authorization: Bearer $TOKEN" \
     http://localhost:8080/api/v1/todos/1

# Get user info
curl -H "Authorization: Bearer $TOKEN" \
     http://localhost:8080/whoami
```


### Admin Endpoints (Requires Admin Role)
```bash
export ADMIN_TOKEN="your-admin-jwt-token-here"

# List all todos (admin only)
curl -H "Authorization: Bearer $ADMIN_TOKEN" \
     http://localhost:8080/api/v1/admin/todos

# List all users (admin only)
curl -H "Authorization: Bearer $ADMIN_TOKEN" \
     http://localhost:8080/api/v1/admin/users
```

## 🔑 Getting JWT Tokens


### Option 1: Keycloak UI
1. Open http://localhost:9443
2. Login with admin/admin
3. Go to your realm → Users
4. Create test users with different roles (e.g., user, admin)
5. Use Keycloak's token endpoint to get JWT tokens

### Option 2: Direct Token Request
```bash
# Get user token
curl -X POST http://localhost:9443/realms/master/protocol/openid-connect/token \
  -H "Content-Type: application/x-www-form-urlencoded" \
  -d "grant_type=password" \
  -d "client_id=account" \
  -d "username=testuser" \
  -d "password=testpass"

# Extract access_token from response
export TOKEN="extracted-access-token"
```

## 🔧 Configuration Highlights


This example shows how to customize AuthMesh (see `main.go` for full details):

```go
config := platform.Config{
    Keycloak: platform.KeycloakConfig{
        URL:   getEnv("KEYCLOAK_URL", "http://localhost:9443"),
        Realm: getEnv("KEYCLOAK_REALM", "master"),
        TrustedIssuers: []string{
            "http://localhost:9443",
            "http://localhost:9443/realms/master",
            "http://localhost:9443/realms/myapp",
        },
    },
    Redis: platform.RedisConfig{
        URL: getEnv("REDIS_URL", "redis://localhost:6379"),
        DB:  0,
    },
    Security: platform.SecurityConfig{
        EnableSSRFProtection:  true,
        BlockPrivateIPs:       true,
        EnableSecurityHeaders: true,
    },
    CORS: platform.CORSConfig{
        AllowOrigins: []string{
            "http://localhost:3000", // React
            "http://localhost:8080", // Vue
            "http://localhost:4200", // Angular
        },
        AllowMethods: []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
        AllowHeaders: []string{"Origin", "Content-Type", "Authorization"},
    },
    RateLimit: platform.RateLimitConfig{
        Enabled:           true,
        RequestsPerSecond: 10,  // Demo limit
        BurstSize:         20,
    },
    Observability: platform.ObservabilityConfig{
        EnableMetrics:    true,
        EnableTracing:    true,
        AppName:          "todo-api",
        AppVersion:       "1.0.0",
        PrometheusURL:    "http://localhost:9090",
        OTelCollectorURL: "http://localhost:4318",
    },
    Routes: platform.RouteConfig{
        EnableStandardHealthRoutes: true,
        EnableRootWelcomeRoute:     true,
        EnableStandardAdminRoutes:  true,
        CustomWelcomeMessage:       "Welcome to Todo API - Powered by AuthMesh",
    },
}
```

## 🎯 Key Learning Points

1. **Custom Configuration**: How to override defaults for your use case
2. **Authentication Integration**: JWT token validation and user extraction
3. **Role-Based Authorization**: Different access levels for users vs admins
4. **Rate Limiting**: Protecting your API from abuse
5. **CORS Configuration**: Supporting multiple frontend frameworks
6. **Observability**: Built-in metrics and tracing

## 📊 Monitoring

### Prometheus Metrics
```bash
# View metrics
curl http://localhost:8080/metrics

# Example metrics:
# - http_requests_total
# - http_request_duration_seconds
# - authmesh_auth_attempts_total
# - authmesh_rate_limit_hits_total
```

### Health Checks
```bash
# Kubernetes-style health checks
curl http://localhost:8080/health    # Returns 200 if healthy
curl http://localhost:8080/ready     # Returns 200 if ready to serve
```

## 🧪 Testing Rate Limits

```bash
# Make rapid requests to trigger rate limiting
for i in {1..25}; do
  curl -H "Authorization: Bearer $TOKEN" \
       http://localhost:8080/api/v1/todos
  sleep 0.1
done

# You should see 429 Too Many Requests after 10 requests
```

todo-api/

## 🔄 Code Structure

```
todo-api/
├── main.go              # Application entry point
├── docker-compose.yml   # Infrastructure services
├── README.md            # This file
├── prometheus.yml       # Prometheus config
└── .env.example         # Example environment variables (add if needed)
```

## 📚 Next Steps

- Try the [basic-app](../basic-app) for advanced features and production patterns
- See the [AuthMesh documentation](../../docs/) for production deployment patterns
- Check [AuthMesh docs](../../docs/) for advanced features


## 🆚 Comparison with Other Examples

| Feature        | Minimal      | QuickStart    | Todo API     | Basic/Advanced |
|--------------- |-------------|--------------|--------------|----------------|
| Setup Lines    | 3           | ~7           | ~15          | ~30-50         |
| Configuration  | None        | Defaults     | Custom       | Advanced       |
| Auth           | Basic       | JWT + Roles  | JWT + Roles  | Multi-tenant   |
| Database       | None        | None         | In-memory    | PostgreSQL     |
| Complexity     | Beginner    | Beginner     | Intermediate | Advanced       |
