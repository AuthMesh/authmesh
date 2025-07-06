# AuthMesh Basic Example

This example demonstrates the basic usage of AuthMesh with Keycloak authentication, Redis rate limiting, and Prometheus metrics.

## Prerequisites

- Docker and Docker Compose
- Go 1.21+

## Quick Start

### 1. Start Infrastructure Services

```bash
docker-compose up -d
```

This starts:
- Keycloak (port 9443) - Authentication server
- Redis (port 6379) - Rate limiting and caching
- Prometheus (port 9090) - Metrics collection
- Grafana (port 3000) - Metrics visualization

### 2. Configure Keycloak

1. Open Keycloak admin console: http://localhost:9443
2. Login with admin/admin
3. Create a new realm or use the master realm
4. Create a client for your application
5. Create users and assign roles (admin, editor, viewer)

### 3. Run the Example Application

```bash
go run main.go
```

The application will start on http://localhost:8080

## API Endpoints

### Public Endpoints
- `GET /` - Welcome message
- `GET /health` - Health check
- `GET /ready` - Readiness check
- `GET /metrics` - Prometheus metrics

### Protected Endpoints
- `GET /protected` - Requires valid JWT token
- `GET /admin` - Requires admin role
- `GET /whoami` - Returns user information
- `GET /session` - Returns session information

### API Endpoints
- `GET /api/data` - Read data (any authenticated user)
- `POST /api/data` - Create data (editor role required)
- `DELETE /api/data/:id` - Delete data (admin role required)

### Tenant-Specific Endpoints
- `GET /t/:tenant_id/dashboard` - Tenant dashboard
- `GET /t/:tenant_id/users` - List tenant users
- `POST /t/:tenant_id/users` - Create user in tenant (admin only)

### Super Admin Endpoints
- `GET /admin/tenants` - List all tenants (super-admin only)
- `POST /admin/tenants` - Create tenant (super-admin only)

## Testing Authentication

### 1. Get a JWT Token from Keycloak

```bash
# Replace with your Keycloak configuration
curl -X POST "http://localhost:9443/realms/master/protocol/openid-connect/token" \
  -H "Content-Type: application/x-www-form-urlencoded" \
  -d "username=your-username" \
  -d "password=your-password" \
  -d "grant_type=password" \
  -d "client_id=your-client-id"
```

### 2. Use the Token

```bash
# Set the token
TOKEN="your-jwt-token-here"

# Access protected endpoint
curl -H "Authorization: Bearer $TOKEN" http://localhost:8080/protected

# Access user info
curl -H "Authorization: Bearer $TOKEN" http://localhost:8080/whoami

# Access tenant-specific endpoint
curl -H "Authorization: Bearer $TOKEN" http://localhost:8080/t/tenant1/dashboard
```

## Monitoring

### Prometheus Metrics
- Open http://localhost:9090
- Query metrics like `http_requests_total`, `auth_attempts_total`

### Grafana Dashboards
- Open http://localhost:3000
- Login with admin/admin
- Add Prometheus as data source: http://prometheus:9090
- Create dashboards for application metrics

## Configuration

The example uses default configuration. For production, customize the config:

```go
config := platform.Config{
    Keycloak: platform.KeycloakConfig{
        URL:   "https://your-keycloak.com",
        Realm: "your-realm",
    },
    Redis: platform.RedisConfig{
        URL: "redis://your-redis:6379",
    },
    // ... other configuration
}
```

## Environment Variables

You can override configuration with environment variables:

```bash
export KEYCLOAK_URL=http://localhost:9443
export KEYCLOAK_REALM=master
export REDIS_URL=redis://localhost:6379
```

## Cleanup

```bash
docker-compose down -v
```

This removes all containers and volumes.

## Next Steps

- Check the [advanced example](../advanced-app/) for more features
- Read the [full documentation](../../docs/)
- Explore [migration examples](../migration-examples/) from other auth libraries
