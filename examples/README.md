# AuthMesh Examples

Progressive examples demonstrating AuthMesh capabilities from beginner to production-ready applications.

## 🎯 Example Progression

| Example | Complexity | Lines of Code | Best For |
|---------|------------|---------------|----------|
| [minimal-app](./minimal-app/) | Beginner | ~30 | Getting started, quick prototypes |
| [quickstart-app](./quickstart-app/) | Beginner+ | ~50 | Learning the QuickStart API |
| [todo-api](./todo-api/) | Intermediate | ~150 | Custom configuration, real APIs |
| [basic-app](./basic-app/) | Advanced | ~300 | Advanced features, staging |

## 📚 Learning Path

### 1. Start Here: [Minimal App](./minimal-app/)
```go
// Just 3 lines to get a full platform!
authMesh, _ := platform.QuickStart("minimal-app")
router := gin.Default()
authMesh.SetupAll(router)
```
**Learn**: Basic setup, standard endpoints, authentication basics

### 2. Next: [QuickStart App](./quickstart-app/)
```go
// Same QuickStart but with more example routes
authMesh, _ := platform.QuickStart("quickstart-example")
router := gin.Default()
authMesh.SetupAll(router)
// + example user, admin, and tenant routes
```
**Learn**: QuickStart API, role-based routes, tenant patterns

### 3. Then: [Todo API](./todo-api/)
```go
// Custom configuration for real applications
config := platform.Config{
    RateLimit: platform.RateLimitConfig{Enabled: true},
    CORS: platform.CORSConfig{AllowOrigins: []string{"..."}},
    // ... more configuration
}
```
**Learn**: Custom configuration, role-based auth, rate limiting, CORS

### 4. Advanced: [Basic App](./basic-app/)
**Learn**: Tracing, advanced security, production patterns

## 🚀 Quick Start Any Example

```bash
cd authmesh/examples/[example-name]
docker-compose up -d    # Start infrastructure
go run main.go         # Run the application
```

## 🌟 Why This Progression?

**Without AuthMesh** (traditional setup):
```go
// 40+ lines just for basic setup
router := gin.Default()
router.Use(cors.New(cors.Config{...}))
router.Use(recovery.Recovery())
authMiddleware := jwt.New(jwt.Config{...})
// ... much more boilerplate
```

**With AuthMesh** (any complexity level):
```go
// 3-15 lines depending on your needs
authMesh, _ := platform.QuickStart("app")  // or platform.New(config)
router := gin.Default()
authMesh.SetupAll(router)
```

## 📊 Feature Matrix

| Feature | Minimal | QuickStart | Todo API | Basic App |
|---------|---------|------------|----------|-----------|
| **Setup** | QuickStart | QuickStart | Custom Config | Advanced Config |
| **Auth** | JWT Basic | JWT + Roles | JWT + Roles | JWT + Advanced |
| **Database** | None | None | In-memory | PostgreSQL |
| **Rate Limiting** | Default | Default | Custom | Advanced |
| **Observability** | Basic | Basic | Metrics | Metrics + Tracing |
| **Security** | Standard | Standard | Enhanced | Production |
| **Testing** | Manual | Manual | Basic | Unit Tests |
| **Deployment** | Local | Local | Docker | Docker Compose |

## 🎓 Learning Outcomes

After working through these examples, you'll understand:

- ✅ **AuthMesh Basics**: How to get started in minutes
- ✅ **Configuration**: Customizing for your application needs  
- ✅ **Authentication**: JWT tokens, roles, and multi-tenant auth
- ✅ **Security**: CORS, rate limiting, SSRF protection
- ✅ **Observability**: Metrics, tracing, and health checks
- ✅ **Production Deployment**: Docker, testing, and operations

## 📖 Additional Resources

- 📚 [AuthMesh Documentation](../docs/)
- 🔧 [Configuration Reference](../docs/CONFIGURATION.md)
- 🛡️ [Security Guide](../docs/CORS_SECURITY.md)
- 📊 [Monitoring Setup](../docs/monitoring/)
- 🚀 [Production Checklist](../docs/DEPLOYMENT_CHECKLIST.md)
