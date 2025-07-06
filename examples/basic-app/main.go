package main

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/AuthMesh/authmesh/pkg/observability"
	"github.com/AuthMesh/authmesh/pkg/platform"
	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

func main() {
	// Create platform configuration
	config := platform.DefaultConfig()
	
	// Customize configuration for your environment
	config.Keycloak.URL = "http://localhost:9443"
	config.Keycloak.Realm = "master"
	config.Redis.URL = "redis://localhost:6379"
	
	// Enhanced observability configuration (Stage 2)
	config.Observability.AppName = "basic-example"
	config.Observability.AppVersion = "1.0.0"
	config.Observability.EnableMetrics = true
	config.Observability.EnableTracing = true // Enable tracing for demo
	config.Observability.TracingConfig.ServiceName = "basic-example"
	config.Observability.TracingConfig.ServiceVersion = "1.0.0"
	config.Observability.TracingConfig.Environment = "development"
	config.Observability.TracingConfig.OTLPEndpoint = "http://localhost:4318"
	config.Observability.TracingConfig.SampleRate = 1.0 // 100% sampling for demo
	
	// Enhanced security configuration (Stage 2)
	config.Security.EnableSSRFProtection = true
	config.Security.BlockPrivateIPs = true
	config.Security.EnableSecurityHeaders = true
	config.Security.AllowedHosts = []string{"localhost", "127.0.0.1"}
	
	// Enhanced rate limiting configuration (Stage 2)
	config.RateLimit.Enabled = true
	config.RateLimit.RequestsPerSecond = 10.0
	config.RateLimit.BurstSize = 20
	config.RateLimit.TenantEnabled = true
	config.RateLimit.TenantRequestsPerSecond = 50.0
	config.RateLimit.TenantBurstSize = 100

	// Initialize the platform
	authMesh, err := platform.New(config)
	if err != nil {
		log.Fatalf("Failed to initialize AuthMesh: %v", err)
	}

	// Create Gin router
	router := gin.Default()

	// Setup comprehensive middleware stack (Stage 2 enhancements)
	// Includes: Recovery, RequestID, Security Headers, SSRF Protection,
	// CORS, Redis Rate Limiting, Structured Logging, Audit Logging, Metrics
	authMesh.SetupMiddleware(router)

	// Setup common routes (health, metrics, etc.)
	authMesh.SetupRoutes(router)

	// Example protected routes
	setupProtectedRoutes(router, authMesh)

	// Example tenant-specific routes (demonstrates rate limiting per tenant)
	setupTenantRoutes(router, authMesh)

	log.Println("🚀 Starting AuthMesh Basic Example on :8080")
	log.Printf("📊 Health check: http://localhost:8080/health")
	log.Printf("📈 Metrics: http://localhost:8080/metrics") 
	log.Printf("👤 User info: http://localhost:8080/whoami")
	log.Printf("🏠 Example: http://localhost:8080/")
	log.Printf("🔒 Protected: http://localhost:8080/protected (requires JWT)")
	log.Printf("👑 Admin: http://localhost:8080/admin (requires admin role)")
	log.Printf("🏢 Tenant: http://localhost:8080/t/tenant1/data (tenant-specific)")
	log.Printf("🔍 Tracing Demo: http://localhost:8080/api/trace-demo (demonstrates tracing)")
	
	// Graceful shutdown setup
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	
	// Start server in goroutine
	go func() {
		if err := router.Run(":8080"); err != nil {
			log.Printf("Server failed: %v", err)
		}
	}()
	
	// Wait for interrupt signal for graceful shutdown
	// In a real application, you'd use os/signal to handle SIGINT/SIGTERM
	select {
	case <-ctx.Done():
		log.Println("Shutting down gracefully...")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		
		if err := authMesh.Shutdown(shutdownCtx); err != nil {
			log.Printf("Error during shutdown: %v", err)
		}
		log.Println("Shutdown completed")
	}
}

func setupProtectedRoutes(router *gin.Engine, authMesh *platform.Platform) {
	// Public route (no authentication required)
	router.GET("/", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"message": "Welcome to AuthMesh Basic Example",
			"version": "1.0.0",
		})
	})

	// Protected route (authentication required)
	router.GET("/protected", authMesh.AuthMiddleware(), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"message": "This is a protected endpoint",
			"user":    getUserInfo(c),
		})
	})

	// Admin-only route
	router.GET("/admin", authMesh.AuthMiddleware("admin"), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"message": "Admin access granted",
			"user":    getUserInfo(c),
		})
	})

	// Editor or above route
	api := router.Group("/api")
	api.Use(authMesh.AuthMiddleware())
	{
		// Read operations (any authenticated user)
		api.GET("/data", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{
				"data": []string{"item1", "item2", "item3"},
				"user": getUserInfo(c),
			})
		})

		// Write operations (editor role required)
		api.POST("/data", authMesh.RequireRole("editor"), func(c *gin.Context) {
			c.JSON(http.StatusCreated, gin.H{
				"message": "Data created successfully",
				"user":    getUserInfo(c),
			})
		})

		// Tracing demo route with manual spans
		api.GET("/trace-demo", func(c *gin.Context) {
			tracingDemo(c, authMesh)
		})

		// Admin operations
		api.DELETE("/data/:id", authMesh.RequireRole("admin"), func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{
				"message": "Data deleted successfully",
				"id":      c.Param("id"),
				"user":    getUserInfo(c),
			})
		})
	}
}

func setupTenantRoutes(router *gin.Engine, authMesh *platform.Platform) {
	// Tenant-specific routes
	tenantGroup := router.Group("/t/:tenant_id")
	tenantGroup.Use(authMesh.AuthMiddleware())
	tenantGroup.Use(authMesh.TenantMiddleware())
	{
		tenantGroup.GET("/dashboard", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{
				"message":   "Tenant dashboard",
				"tenant_id": c.Param("tenant_id"),
				"user":      getUserInfo(c),
			})
		})

		tenantGroup.GET("/users", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{
				"message":   "Tenant users",
				"tenant_id": c.Param("tenant_id"),
				"users":     []string{"user1", "user2"},
				"user":      getUserInfo(c),
			})
		})

		tenantGroup.POST("/users", authMesh.RequireRole("admin"), func(c *gin.Context) {
			c.JSON(http.StatusCreated, gin.H{
				"message":   "User created in tenant",
				"tenant_id": c.Param("tenant_id"),
				"user":      getUserInfo(c),
			})
		})
	}

	// Super admin routes (cross-tenant)
	superAdminGroup := router.Group("/admin")
	superAdminGroup.Use(authMesh.SuperAdminMiddleware())
	{
		superAdminGroup.GET("/tenants", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{
				"message": "All tenants",
				"tenants": []string{"tenant1", "tenant2", "tenant3"},
				"user":    getUserInfo(c),
			})
		})

		superAdminGroup.POST("/tenants", func(c *gin.Context) {
			c.JSON(http.StatusCreated, gin.H{
				"message": "Tenant created",
				"user":    getUserInfo(c),
			})
		})
	}
}

func getUserInfo(c *gin.Context) gin.H {
	userID, _ := c.Get("user_id")
	username, _ := c.Get("username")
	tenantID, _ := c.Get("tenant_id")

	return gin.H{
		"user_id":   userID,
		"username":  username,
		"tenant_id": tenantID,
	}
}

// tracingDemo demonstrates manual span creation and tracing features
func tracingDemo(c *gin.Context, authMesh *platform.Platform) {
	ctx := c.Request.Context()
	
	// Get the tracing provider from the platform
	tracingProvider := authMesh.GetTracingProvider()
	if tracingProvider == nil {
		c.JSON(http.StatusOK, gin.H{
			"message": "Tracing is not enabled",
			"traces": []string{},
		})
		return
	}

	// Create a new span for the main operation
	ctx, mainSpan := tracingProvider.StartSpan(ctx, "trace-demo-operation",
		trace.WithAttributes(
			attribute.String("demo.type", "manual_tracing"),
			attribute.String("demo.version", "1.0"),
		),
	)
	defer mainSpan.End()

	// Add some attributes to the main span
	observability.SetAttribute(ctx, "user.demo", "true")
	if userID, exists := c.Get("user_id"); exists {
		observability.SetAttribute(ctx, "user.id", userID)
	}

	// Simulate some work with child spans
	traces := []string{}

	// First operation: database lookup simulation
	ctx, dbSpan := tracingProvider.StartSpan(ctx, "database-lookup",
		trace.WithAttributes(
			attribute.String("db.operation", "SELECT"),
			attribute.String("db.table", "users"),
		),
	)
	time.Sleep(50 * time.Millisecond) // Simulate DB latency
	observability.AddEvent(ctx, "database-query-completed",
		attribute.Int("rows.found", 5),
	)
	dbSpan.End()
	traces = append(traces, "database-lookup")

	// Second operation: external API call simulation
	ctx, apiSpan := tracingProvider.StartSpan(ctx, "external-api-call",
		trace.WithAttributes(
			attribute.String("http.method", "GET"),
			attribute.String("http.url", "https://api.example.com/data"),
		),
	)
	time.Sleep(100 * time.Millisecond) // Simulate API latency
	observability.AddEvent(ctx, "api-response-received",
		attribute.Int("http.status_code", 200),
		attribute.String("response.size", "1.2KB"),
	)
	apiSpan.End()
	traces = append(traces, "external-api-call")

	// Third operation: business logic processing
	ctx, processSpan := tracingProvider.StartSpan(ctx, "business-logic-processing",
		trace.WithAttributes(
			attribute.String("processor.type", "data-transformer"),
			attribute.Int("items.count", 10),
		),
	)
	time.Sleep(30 * time.Millisecond) // Simulate processing time
	observability.AddEvent(ctx, "processing-completed",
		attribute.Int("items.processed", 10),
		attribute.Bool("validation.passed", true),
	)
	processSpan.End()
	traces = append(traces, "business-logic-processing")

	// Add final event to main span
	observability.AddEvent(ctx, "demo-completed",
		attribute.Int("total.operations", len(traces)),
		attribute.String("demo.status", "success"),
	)

	c.JSON(http.StatusOK, gin.H{
		"message": "Tracing demo completed successfully",
		"traces":  traces,
		"user":    getUserInfo(c),
		"metadata": gin.H{
			"total_operations": len(traces),
			"trace_id": mainSpan.SpanContext().TraceID().String(),
			"span_id":  mainSpan.SpanContext().SpanID().String(),
		},
	})
}
