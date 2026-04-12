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
	// 🚀 AuthMesh QuickStart - Get a full platform in 3 lines!
	authMesh, err := platform.QuickStart("quickstart-example")
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
	authMesh.SetupAll(router) // Everything configured: middleware + routes + auth + metrics!

	// Add your custom application routes
	setupApplicationRoutes(router, authMesh)

	log.Println("🚀 AuthMesh QuickStart Example running on http://localhost:8080")
	log.Println("� Try these endpoints:")
	log.Println("   GET  /              - Welcome message")
	log.Println("   GET  /health        - Health check") 
	log.Println("   GET  /api/profile   - User profile (auth required)")
	log.Println("   GET  /api/admin     - Admin dashboard (admin role required)")
	log.Println("   GET  /whoami        - Current user info (auth required)")

	router.Run(":8080")
}

func setupApplicationRoutes(router *gin.Engine, authMesh *platform.Platform) {
	// Your custom public routes
	router.GET("/api", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"message": "AuthMesh QuickStart API",
			"version": "1.0.0",
			"powered_by": "AuthMesh Platform",
			"endpoints": gin.H{
				"profile": "/api/profile",
				"admin":   "/api/admin", 
				"tenant":  "/t/{tenant_id}/dashboard",
			},
		})
	})

	// Protected user routes
	protected := router.Group("/api")
	protected.Use(authMesh.AuthMiddleware())
	{
		protected.GET("/profile", func(c *gin.Context) {
			userSub := c.GetString("user_sub")
			c.JSON(http.StatusOK, gin.H{
				"message": "User profile data",
				"user_id": userSub,
				"features": []string{"dashboard", "settings", "billing"},
			})
		})

		protected.GET("/dashboard", func(c *gin.Context) {
			userSub := c.GetString("user_sub")
			c.JSON(http.StatusOK, gin.H{
				"message": "User dashboard",
				"user": userSub,
				"stats": gin.H{
					"login_count": 42,
					"last_login": "2024-01-15T10:30:00Z",
				},
			})
		})
	}

	// Admin routes (requires admin role)
	admin := router.Group("/api") 
	admin.Use(authMesh.AuthMiddleware())
	admin.Use(authMesh.RequireRole("admin"))
	{
		admin.GET("/admin", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{
				"message": "Admin dashboard",
				"admin_features": []string{"user_management", "system_config", "analytics"},
			})
		})
	}

	// Multi-tenant routes
	tenant := router.Group("/t/:tenant_id")
	tenant.Use(authMesh.AuthMiddleware())
	tenant.Use(authMesh.TenantMiddleware())
	{
		tenant.GET("/dashboard", func(c *gin.Context) {
			tenantID := c.Param("tenant_id")
			userSub := c.GetString("user_sub")
			c.JSON(http.StatusOK, gin.H{
				"message":   "Tenant dashboard",
				"tenant_id": tenantID,
				"user":      userSub,
				"tenant_data": gin.H{
					"name": "Acme Corp",
					"plan": "enterprise",
				},
			})
		})
	}
}
