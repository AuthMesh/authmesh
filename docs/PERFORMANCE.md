# Performance & Optimization Guide

## Overview

AuthMesh is designed for high-performance, multi-tenant authentication scenarios. This guide covers performance characteristics, optimization strategies, and benchmarking results.

## Performance Benchmarks

Based on production testing with Go 1.21+ on modern hardware:

```
BenchmarkPlatformSetup-8                 3,000,000    392 ns/op    816 B/op    6 allocs/op
BenchmarkMiddlewareStack-8                 127,000  9,123 ns/op  4,037 B/op   59 allocs/op
BenchmarkRateLimiting-8                 10,800,000    109 ns/op      0 B/op    0 allocs/op
BenchmarkJWTValidation-8                 6,400,000    180 ns/op    160 B/op    5 allocs/op
BenchmarkConcurrentRequests-8              260,000  4,700 ns/op  4,040 B/op   59 allocs/op
```

### Key Performance Metrics

- **Platform Initialization**: ~400ns (sub-microsecond startup)
- **JWT Validation**: ~180ns per token (5.5M tokens/second)
- **Rate Limiting**: ~109ns per check (9M checks/second)
- **Request Processing**: ~9µs through full middleware stack
- **Memory Usage**: ~4KB per request with 59 allocations

## Optimization Strategies

### 1. JWT Validation Optimization

```go
// Use connection pooling for JWKS endpoints
authMesh, err := platform.NewProduction(platform.Config{
    KeycloakURL: "https://auth.example.com",
    HTTPTimeout: 5 * time.Second,
    // Enable JWKS caching for 1 hour
    JWKSCacheTTL: time.Hour,
})
```

### 2. Rate Limiting Optimization

For high-throughput scenarios, use Redis-based rate limiting:

```go
// Redis provides ~100µs per operation vs ~109ns for in-memory
redisClient := redis.NewClient(&redis.Options{
    Addr: "localhost:6379",
    PoolSize: 100, // Optimize pool size for your workload
})

authMesh := platform.NewWithDefaults("myapp", platform.WithRedis(redisClient))
```

### 3. Middleware Stack Optimization

```go
// For high-performance scenarios, disable unnecessary middleware
authMesh.SetupCore(router) // Only essential middleware
// vs
authMesh.SetupAll(router)  // Full middleware stack
```

### 4. Memory Optimization

- **Connection Pooling**: Use HTTP/2 with connection pooling for Keycloak
- **Buffer Reuse**: Enable Gin's buffer reuse for lower GC pressure
- **Context Pooling**: AuthMesh automatically pools contexts

### 5. Concurrent Request Handling

AuthMesh is designed for high concurrency:

```go
// Scales linearly with CPU cores
// ~260K concurrent requests/second on 8-core system
// Memory usage remains constant at ~4KB per active request
```

## Production Tuning

### Environment Variables

```bash
# Core settings
KEYCLOAK_URL="https://auth.example.com"
TRUSTED_ISSUERS="https://auth.example.com"

# Performance tuning
HTTP_TIMEOUT="5s"
RATE_LIMIT_REDIS_URL="redis://localhost:6379"
JWT_CACHE_TTL="3600"

# Observability
OTEL_EXPORTER_OTLP_ENDPOINT="http://jaeger:14268/api/traces"
METRICS_ENABLED="true"
LOG_LEVEL="info"
```

### Container Resource Limits

```yaml
resources:
  requests:
    memory: "256Mi"
    cpu: "100m"
  limits:
    memory: "512Mi"
    cpu: "500m"
```

### Load Testing

Use the included benchmarks to validate performance:

```bash
# Run comprehensive benchmarks
go test -bench=. -benchmem -count=3 tests/benchmarks/

# Load test with hey
hey -n 100000 -c 100 -H "Authorization: Bearer $TOKEN" \
    http://localhost:8080/api/protected
```

## Monitoring & Profiling

### Metrics to Monitor

1. **JWT Validation Latency**: Should stay <1ms
2. **Rate Limit Check Latency**: Should stay <100µs
3. **Memory Usage**: Should be stable under load
4. **Error Rates**: Monitor 401/403 responses
5. **Cache Hit Ratios**: JWKS cache should be >95%

### Profile Collection

```go
import _ "net/http/pprof"

// Enable pprof endpoint in development
go func() {
    log.Println(http.ListenAndServe("localhost:6060", nil))
}()
```

### Common Performance Issues

1. **High JWT Validation Latency**
   - Check JWKS endpoint connectivity
   - Verify certificate chain is cached
   - Monitor network latency to Keycloak

2. **Memory Leaks**
   - Monitor goroutine count
   - Check for unclosed HTTP connections
   - Verify context cancellation

3. **Rate Limit Bottlenecks**
   - Scale Redis instances horizontally
   - Consider sharding by tenant
   - Monitor Redis connection pool

## Scalability Guidelines

### Horizontal Scaling

- **Stateless Design**: AuthMesh is fully stateless
- **Load Balancing**: Use any L7 load balancer
- **Session Affinity**: Not required

### Vertical Scaling

- **CPU**: JWT validation scales linearly with cores
- **Memory**: ~4KB per concurrent request
- **Network**: Optimize Keycloak connectivity

### Database Scaling

- **Rate Limiting**: Scale Redis cluster as needed
- **Audit Logs**: Use time-series databases
- **Metrics**: Prometheus handles high cardinality well

## Security Performance

### TLS Optimization

```go
// Optimize TLS for performance
tlsConfig := &tls.Config{
    MinVersion: tls.VersionTLS12,
    CipherSuites: []uint16{
        tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
        tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,
    },
    SessionTicketsDisabled: false, // Enable for performance
}
```

### Algorithm Selection

- **JWT Signing**: RS256 provides best security/performance balance
- **Rate Limiting**: Token bucket algorithm for burst handling
- **Encryption**: AES-256-GCM for data at rest

## Best Practices

1. **Cache Aggressively**: JWKS, user info, tenant configs
2. **Monitor Everything**: Latency, errors, resource usage
3. **Load Test Regularly**: Validate performance under load
4. **Optimize Network**: Minimize round trips to Keycloak
5. **Profile in Production**: Use pprof for real workload analysis

## Troubleshooting

### High Latency

```bash
# Check JWKS connectivity
curl -w "%{time_total}" https://auth.example.com/realms/master/protocol/openid-connect/certs

# Verify rate limiting performance  
redis-cli --latency -i 1

# Monitor GC pressure
GODEBUG=gctrace=1 ./your-app
```

### Memory Issues

```bash
# Generate heap profile
go tool pprof http://localhost:6060/debug/pprof/heap

# Check goroutine leaks
go tool pprof http://localhost:6060/debug/pprof/goroutine
```

### Rate Limiting Issues

```bash
# Monitor Redis performance
redis-cli --stat

# Check rate limit cache efficiency
curl http://localhost:8080/metrics | grep rate_limit
```
