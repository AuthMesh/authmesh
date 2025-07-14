# AuthMesh Todo API Example

A practical Todo API that demonstrates AuthMesh's custom configuration capabilities and real-world patterns.

## ✨ Features

- **Custom Configuration**: Shows how to configure AuthMesh for specific needs
- **Role-Based Access**: User and admin endpoints with proper authorization
- **Rate Limiting**: Demonstrates custom rate limiting configuration
- **CORS Setup**: Multi-frontend support (React, Vue, Angular)
- **Observability**: Metrics and tracing enabled
- **RESTful API**: Complete CRUD operations for todos

## 🚀 Quick Start

### Prerequisites
- Go 1.21+
- Docker and Docker Compose

### 1. Start Infrastructure
```bash
# Start Keycloak and Redis
docker-compose up -d

# Wait for services to be ready
sleep 10
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

# List user's todos
curl -H "Authorization: Bearer $TOKEN" \
     http://localhost:8080/api/v1/todos

# Create a new todo
curl -X POST \
     -H "Authorization: Bearer $TOKEN" \
     -H "Content-Type: application/json" \
     -d '{"title": "Learn AuthMesh"}' \
     http://localhost:8080/api/v1/todos

# Toggle todo completion
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

# Standard admin info
curl -H "Authorization: Bearer $ADMIN_TOKEN" \
     http://localhost:8080/api/v1/admin/info
```

## 🔑 Getting JWT Tokens

### Option 1: Keycloak UI
1. Open http://localhost:9443
2. Login with admin/admin
3. Go to your realm → Users
4. Create test users with different roles
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

This example shows how to customize AuthMesh:

```go
config := platform.Config{
    Keycloak: platform.KeycloakConfig{
        URL:   "http://localhost:9443",
        Realm: "master",
        TrustedIssuers: []string{
            "http://localhost:9443/realms/master",
            "http://localhost:9443/realms/myapp",
        },
    },
    RateLimit: platform.RateLimitConfig{
        Enabled:           true,
        RequestsPerSecond: 10,  // Conservative limit
        BurstSize:         20,
    },
    CORS: platform.CORSConfig{
        AllowOrigins: []string{
            "http://localhost:3000", // React
            "http://localhost:8080", // Vue  
            "http://localhost:4200", // Angular
        },
    },
    // ... more configuration
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

## 🔄 Code Structure

```
todo-api/
├── main.go              # Application entry point
├── docker-compose.yml   # Infrastructure services
├── README.md           # This file
└── .env.example        # Environment variables
```

## 📚 Next Steps

- Try the [basic-app](../basic-app) for advanced features and production patterns
- See the [AuthMesh documentation](../../../docs/) for production deployment patterns
- Check [AuthMesh docs](../../../docs) for advanced features

## 🆚 Comparison with Other Examples

| Feature | Minimal | Todo API | Multi-Tenant | Advanced |
|---------|---------|----------|--------------|-----------|
| Setup Lines | 3 | ~15 | ~25 | ~50 |
| Configuration | None | Custom | Advanced | Production |
| Auth | Basic | JWT + Roles | Tenant-aware | Multi-realm |
| Database | None | In-memory | PostgreSQL | PostgreSQL |
| Complexity | Beginner | Intermediate | Advanced | Production |
