package main

import (
	"log"
	"net/http"

	"github.com/AuthMesh/authmesh/pkg/platform"
	"github.com/gin-gonic/gin"
)

func main() {
	// 🚀 AuthMesh Minimal Example - Just 3 lines to get started!
	authMesh, err := platform.QuickStart("minimal-app")
	if err != nil {
		log.Fatalf("Failed to initialize AuthMesh: %v", err)
	}

	// Create router and setup everything
	router := gin.Default()
	authMesh.SetupAll(router) // One call: middleware + standard routes

	// Add your custom routes
	router.GET("/hello", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"message": "Hello from AuthMesh!",
			"powered_by": "AuthMesh Platform",
		})
	})

	// Protected route example
	protected := router.Group("/api/v1")
	protected.Use(authMesh.AuthMiddleware())
	{
		protected.GET("/protected", func(c *gin.Context) {
			userSub := c.GetString("user_sub")
			c.JSON(http.StatusOK, gin.H{
				"message": "This endpoint requires authentication",
				"user": userSub,
			})
		})
	}

	log.Println("🚀 Minimal AuthMesh app running on http://localhost:8080")
	log.Println("📋 Try these endpoints:")
	log.Println("   GET  /              - Welcome message")
	log.Println("   GET  /health        - Health check")
	log.Println("   GET  /hello         - Custom hello endpoint")
	log.Println("   GET  /api/v1/protected - Protected endpoint (requires auth)")
	
	router.Run(":8080")
}
