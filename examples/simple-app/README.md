# AuthMesh Simple Example

This example demonstrates the **Stage 3 Unified API** with minimal setup code.

## Features Demonstrated

- **One-line setup**: `platform.QuickStart("service-name")`
- **Unified configuration**: All middleware and routes in one call
- **Automatic observability**: Metrics and tracing enabled by default
- **Production patterns**: Graceful shutdown and error handling

## Quick Start

```bash
# Start the services
docker-compose up -d

# Test the application
curl http://localhost:8080/                    # Public endpoint
curl http://localhost:8080/health             # Health check
curl http://localhost:8080/metrics           # Prometheus metrics

# View traces (if enabled)
open http://localhost:16686                   # Jaeger UI
```

## Code Comparison

**Before (Stage 2)**: ~40 lines of setup
```go
config := platform.DefaultConfig()
config.Keycloak.URL = "http://localhost:9443"
config.Redis.URL = "redis://localhost:6379"
config.Observability.AppName = "basic-example"
config.Observability.EnableMetrics = true
config.Observability.EnableTracing = true
// ... more configuration

authMesh, err := platform.New(config)
router := gin.Default()
authMesh.SetupMiddleware(router)
authMesh.SetupRoutes(router)
```

**After (Stage 3)**: ~5 lines of setup
```go
authMesh, err := platform.QuickStart("basic-example")
router := gin.Default()
authMesh.SetupAll(router) // One call for everything
```

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
