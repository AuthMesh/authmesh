package main

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/AuthMesh/authmesh/pkg/platform"
	"github.com/gin-gonic/gin"
)

func main() {
	// Stage 3: Unified API - One-line setup for common scenarios
	authMesh, err := platform.QuickStart("basic-example")
	if err != nil {
		log.Fatalf("Failed to initialize AuthMesh: %v", err)
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		authMesh.Shutdown(ctx)
	}()

	// Create router and setup everything in one call
	router := gin.Default()
	authMesh.SetupAll(router) // Unified setup: middleware + routes

	// Add your application routes
	setupExampleRoutes(router, authMesh)

	log.Println("🚀 AuthMesh QuickStart Example on :8080")
	log.Println("📊 Health: http://localhost:8080/health")
	log.Println("📈 Metrics: http://localhost:8080/metrics")
	log.Println("🔍 Tracing: http://localhost:16686 (Jaeger)")

	router.Run(":8080")
}

func setupExampleRoutes(router *gin.Engine, authMesh *platform.Platform) {
	// Public route
	router.GET("/", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"message": "AuthMesh QuickStart Example",
			"version": "Stage 3 - Unified API",
		})
	})

	// Protected routes
	protected := router.Group("/api")
	protected.Use(authMesh.AuthMiddleware())
	{
		protected.GET("/profile", func(c *gin.Context) {
			userID, _ := c.Get("user_id")
			c.JSON(http.StatusOK, gin.H{
				"user_id": userID,
				"message": "User profile data",
			})
		})

		// Admin route
		protected.GET("/admin", authMesh.RequireRole("admin"), func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{
				"message": "Admin dashboard",
			})
		})
	}

	// Tenant routes
	tenant := router.Group("/t/:tenant_id")
	tenant.Use(authMesh.AuthMiddleware())
	tenant.Use(authMesh.TenantMiddleware())
	{
		tenant.GET("/dashboard", func(c *gin.Context) {
			tenantID := c.Param("tenant_id")
			c.JSON(http.StatusOK, gin.H{
				"tenant_id": tenantID,
				"message":   "Tenant dashboard",
			})
		})
	}
}
