package main

import (
	"log"
	"net/http"

	"github.com/AuthMesh/authmesh/pkg/platform"
	"github.com/gin-gonic/gin"
)

func main() {
	// Create platform configuration
	config := platform.DefaultConfig()
	
	// Customize configuration for your environment
	config.Keycloak.URL = "http://localhost:9443"
	config.Keycloak.Realm = "master"
	config.Redis.URL = "redis://localhost:6379"
	config.Observability.AppName = "basic-example"
	config.Observability.AppVersion = "1.0.0"

	// Initialize the platform
	authMesh, err := platform.New(config)
	if err != nil {
		log.Fatalf("Failed to initialize AuthMesh: %v", err)
	}

	// Create Gin router
	router := gin.Default()

	// Setup middleware
	authMesh.SetupMiddleware(router)

	// Setup common routes (health, metrics, etc.)
	authMesh.SetupRoutes(router)

	// Example protected routes
	setupProtectedRoutes(router, authMesh)

	// Example tenant-specific routes
	setupTenantRoutes(router, authMesh)

	log.Println("Starting server on :8080")
	log.Printf("Health check: http://localhost:8080/health")
	log.Printf("Metrics: http://localhost:8080/metrics")
	log.Printf("User info: http://localhost:8080/whoami")
	
	if err := router.Run(":8080"); err != nil {
		log.Fatalf("Failed to start server: %v", err)
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
