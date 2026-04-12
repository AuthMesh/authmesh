# AuthMesh API Testing Guide

Complete curl command examples for testing AuthMesh applications across all complexity levels.

## 🎯 Quick Reference

| Endpoint Category | Auth Required | Purpose |
|------------------|---------------|---------|
| **Health & Info** | ❌ No | Service health, API discovery |
| **Protected API** | ✅ User JWT | Application functionality |
| **Admin API** | ✅ Admin JWT | Administrative operations |
| **Tenant API** | ✅ Tenant-aware JWT | Multi-tenant operations |

---

## 📋 Universal Endpoints (All Examples)

### Health & Status Checks
```bash
# Welcome / API Discovery
curl http://localhost:8080/

# Health checks (Kubernetes compatible)
curl http://localhost:8080/health
curl http://localhost:8080/healthz
curl http://localhost:8080/ready
curl http://localhost:8080/readyz

# Comprehensive health check
curl http://localhost:8080/api/v1/health

# Prometheus metrics
curl http://localhost:8080/metrics
```

---

## 🔑 Authentication & Token Management

### Get JWT Token from Keycloak
```bash
# User token
curl -X POST http://localhost:9443/realms/master/protocol/openid-connect/token \
  -H "Content-Type: application/x-www-form-urlencoded" \
  -d "grant_type=password" \
  -d "client_id=account" \
  -d "username=testuser" \
  -d "password=testpass" | jq -r '.access_token'

# Store token for reuse
export TOKEN=$(curl -s -X POST http://localhost:9443/realms/master/protocol/openid-connect/token \
  -H "Content-Type: application/x-www-form-urlencoded" \
  -d "grant_type=password" \
  -d "client_id=account" \
  -d "username=testuser" \
  -d "password=testpass" | jq -r '.access_token')

# Admin token (user with admin role)
export ADMIN_TOKEN=$(curl -s -X POST http://localhost:9443/realms/master/protocol/openid-connect/token \
  -H "Content-Type: application/x-www-form-urlencoded" \
  -d "grant_type=password" \
  -d "client_id=account" \
  -d "username=admin" \
  -d "password=admin" | jq -r '.access_token')
```

### Validate Token / User Info
```bash
# Get user information
curl -H "Authorization: Bearer $TOKEN" \
     http://localhost:8080/whoami
```

---

## 🏃‍♂️ Minimal App (Quick Start)

### Available Endpoints
```bash
# Custom hello endpoint
curl http://localhost:8080/hello

# Protected endpoint
curl -H "Authorization: Bearer $TOKEN" \
     http://localhost:8080/api/v1/protected
```

---

## ✅ Todo API Example

### Public Endpoints
```bash
# API information
curl http://localhost:8080/api
```

### User Operations (Requires JWT)
```bash
# List user's todos
curl -H "Authorization: Bearer $TOKEN" \
     http://localhost:8080/api/v1/todos

# Create a new todo
curl -X POST \
     -H "Authorization: Bearer $TOKEN" \
     -H "Content-Type: application/json" \
     -d '{"title": "Learn AuthMesh APIs"}' \
     http://localhost:8080/api/v1/todos

# Create multiple todos
curl -X POST \
     -H "Authorization: Bearer $TOKEN" \
     -H "Content-Type: application/json" \
     -d '{"title": "Build awesome API"}' \
     http://localhost:8080/api/v1/todos

curl -X POST \
     -H "Authorization: Bearer $TOKEN" \
     -H "Content-Type: application/json" \
     -d '{"title": "Deploy to production"}' \
     http://localhost:8080/api/v1/todos

# Toggle todo completion
curl -X PUT \
     -H "Authorization: Bearer $TOKEN" \
     http://localhost:8080/api/v1/todos/1
```

### Admin Operations (Requires Admin Role)
```bash
# List all todos (admin view)
curl -H "Authorization: Bearer $ADMIN_TOKEN" \
     http://localhost:8080/api/v1/admin/todos

# List all users with todo counts
curl -H "Authorization: Bearer $ADMIN_TOKEN" \
     http://localhost:8080/api/v1/admin/users

# Standard admin info
curl -H "Authorization: Bearer $ADMIN_TOKEN" \
     http://localhost:8080/api/v1/admin/info

# System information
curl -H "Authorization: Bearer $ADMIN_TOKEN" \
     http://localhost:8080/api/v1/admin/system
```

---

## 🔒 Security Testing

### Rate Limiting Tests
```bash
# Test rate limits (adjust port for your app)
for i in {1..25}; do
  echo "Request $i:"
  curl -w "Status: %{http_code}, Time: %{time_total}s\n" \
       -H "Authorization: Bearer $TOKEN" \
       http://localhost:8080/api/v1/todos
  sleep 0.1
done

# You should see 429 Too Many Requests after hitting the limit
```

### CORS Testing
```bash
# Test CORS headers
curl -H "Origin: http://localhost:3000" \
     -H "Access-Control-Request-Method: GET" \
     -H "Access-Control-Request-Headers: authorization" \
     -X OPTIONS \
     http://localhost:8080/api/v1/todos

# Check for proper CORS headers in response
```

### Security Headers
```bash
# Check security headers
curl -I http://localhost:8080/

# Look for:
# X-Content-Type-Options: nosniff
# X-Frame-Options: DENY
# X-XSS-Protection: 1; mode=block
```

---

## 📊 Monitoring & Observability

### Metrics Collection
```bash
# Get all metrics
curl http://localhost:8080/metrics

# Filter specific metrics
curl -s http://localhost:8080/metrics | grep "http_requests_total"
curl -s http://localhost:8080/metrics | grep "authmesh"

# Check rate limiting metrics
curl -s http://localhost:8080/metrics | grep "rate_limit"
```

### Health Monitoring Script
```bash
#!/bin/bash
# health-check.sh - Monitor application health

ENDPOINTS=(
  "http://localhost:8080/"
  "http://localhost:8080/health"
  "http://localhost:8080/ready"
  "http://localhost:8080/metrics"
)

for endpoint in "${ENDPOINTS[@]}"; do
  status=$(curl -s -o /dev/null -w "%{http_code}" "$endpoint")
  if [ "$status" = "200" ]; then
    echo "✅ $endpoint - OK"
  else
    echo "❌ $endpoint - FAILED ($status)"
  fi
done
```

---

## 🧪 Load Testing

### Simple Load Test
```bash
# Install apache bench (if not available)
# brew install httpie  # for mac
# apt-get install apache2-utils  # for ubuntu

# Load test public endpoint
ab -n 100 -c 10 http://localhost:8080/health

# Load test authenticated endpoint (requires valid token)
ab -n 100 -c 10 -H "Authorization: Bearer $TOKEN" \
   http://localhost:8080/api/v1/todos
```

### Wrk Load Testing
```bash
# Install wrk: https://github.com/wg/wrk

# Load test with authentication
echo 'wrk.headers["Authorization"] = "Bearer '$TOKEN'"' > auth.lua

wrk -t12 -c400 -d30s -s auth.lua \
    http://localhost:8080/api/v1/todos
```

---

## 🔧 Troubleshooting Commands

### Debug Authentication Issues
```bash
# Check token validity
echo $TOKEN | cut -d. -f2 | base64 -d 2>/dev/null | jq .

# Test with invalid token
curl -H "Authorization: Bearer invalid-token" \
     http://localhost:8080/api/v1/protected

# Test without token
curl http://localhost:8080/api/v1/protected
```

### Debug Configuration
```bash
# Check application logs
docker-compose logs backend

# Check Keycloak logs
docker-compose logs keycloak

# Check Redis connectivity
redis-cli ping

# Check if services are running
docker-compose ps
```

### Network Debugging
```bash
# Test connectivity between services
docker-compose exec backend ping keycloak
docker-compose exec backend ping redis

# Check DNS resolution
nslookup keycloak
nslookup localhost
```

---

## 📱 Mobile/Frontend Integration

### JavaScript Fetch Examples
```javascript
// Get user todos
const response = await fetch('http://localhost:8080/api/v1/todos', {
  headers: {
    'Authorization': `Bearer ${token}`,
    'Content-Type': 'application/json'
  }
});

// Create todo
const newTodo = await fetch('http://localhost:8080/api/v1/todos', {
  method: 'POST',
  headers: {
    'Authorization': `Bearer ${token}`,
    'Content-Type': 'application/json'
  },
  body: JSON.stringify({ title: 'New todo' })
});
```

### React/Vue/Angular Integration
```typescript
// axios example (works with React/Vue/Angular)
import axios from 'axios';

const api = axios.create({
  baseURL: 'http://localhost:8080',
  headers: {
    'Authorization': `Bearer ${localStorage.getItem('token')}`
  }
});

// Use the API
const todos = await api.get('/api/v1/todos');
const newTodo = await api.post('/api/v1/todos', { title: 'Learn AuthMesh' });
```

---

## 🎯 Example Scenarios

### Complete User Workflow
```bash
# 1. Get token
export TOKEN=$(curl -s -X POST http://localhost:9443/realms/master/protocol/openid-connect/token \
  -H "Content-Type: application/x-www-form-urlencoded" \
  -d "grant_type=password&client_id=account&username=testuser&password=testpass" | jq -r '.access_token')

# 2. Check user info
curl -H "Authorization: Bearer $TOKEN" http://localhost:8080/whoami

# 3. Use application features
curl -H "Authorization: Bearer $TOKEN" http://localhost:8080/api/v1/todos
curl -X POST -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" \
     -d '{"title": "Complete tutorial"}' http://localhost:8080/api/v1/todos

# 4. Check metrics
curl -s http://localhost:8080/metrics | grep "http_requests_total"
```

### Multi-tenant Workflow
```bash
# Work with tenant1
curl -H "Authorization: Bearer $TOKEN" \
     http://localhost:8080/t/tenant1/api/v1/info

curl -H "Authorization: Bearer $TOKEN" \
     http://localhost:8080/t/tenant1/api/v1/data

# Switch to tenant2
curl -H "Authorization: Bearer $TOKEN" \
     http://localhost:8080/t/tenant2/api/v1/info

curl -H "Authorization: Bearer $TOKEN" \
     http://localhost:8080/t/tenant2/api/v1/data
```

This guide provides comprehensive testing capabilities for any AuthMesh application! 🚀
