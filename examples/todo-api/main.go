package main

import (
	"log"
	"net/http"
	"os"

	"github.com/AuthMesh/authmesh/pkg/platform"
	"github.com/AuthMesh/authmesh/pkg/auth"
	"github.com/gin-gonic/gin"
)

func main() {
	// 🔧 AuthMesh Custom Configuration Example
	// Shows how to customize the platform for your specific needs
	
	config := platform.Config{
		Keycloak: platform.KeycloakConfig{
			URL:   getEnv("KEYCLOAK_URL", "http://localhost:9443"),
			Realm: getEnv("KEYCLOAK_REALM", "master"),
			TrustedIssuers: []string{
				"http://localhost:9443",
				"http://localhost:9443/realms/master",
				"http://localhost:9443/realms/myapp",
			},
		},
		Redis: platform.RedisConfig{
			URL: getEnv("REDIS_URL", "redis://localhost:6379"),
			DB:  0,
		},
		Security: platform.SecurityConfig{
			EnableSSRFProtection:  true,
			BlockPrivateIPs:       true,
			EnableSecurityHeaders: true,
		},
		CORS: platform.CORSConfig{
			AllowOrigins: []string{
				"http://localhost:3000",  // React app
				"http://localhost:8080",  // Vue app
				"http://localhost:4200",  // Angular app
			},
			AllowMethods: []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
			AllowHeaders: []string{"Origin", "Content-Type", "Authorization"},
		},
		RateLimit: platform.RateLimitConfig{
			Enabled:           true,
			RequestsPerSecond: 10,   // Lower limit for demo
			BurstSize:         20,
		},
		Observability: platform.ObservabilityConfig{
			EnableMetrics:    true,
			EnableTracing:    true,  // Enable tracing for this example
			AppName:          "todo-api",
			AppVersion:       "1.0.0",
			PrometheusURL:    "http://localhost:9090",
			OTelCollectorURL: "http://localhost:4318",
		},
		Routes: platform.RouteConfig{
			EnableStandardHealthRoutes: true,
			EnableRootWelcomeRoute:     true,
			EnableStandardAdminRoutes:  true,
			CustomWelcomeMessage:       "Welcome to Todo API - Powered by AuthMesh",
		},
	}

	// Initialize the platform with custom config
	authMesh, err := platform.New(config)
	if err != nil {
		log.Fatalf("Failed to initialize AuthMesh: %v", err)
	}

	// Create router and setup middleware
	router := gin.Default()
	authMesh.SetupMiddleware(router)
	authMesh.SetupRoutes(router)

	// Add application-specific routes
	setupTodoAPI(router, authMesh)

	port := getEnv("PORT", "8080")
	log.Printf("🚀 Todo API running on http://localhost:%s", port)
	log.Println("📋 Try these endpoints:")
	log.Println("   GET  /              - API info")
	log.Println("   GET  /health        - Health check")
	log.Println("   GET  /metrics       - Prometheus metrics")
	log.Println("   GET  /api/v1/todos  - List todos (auth required)")
	log.Println("   POST /api/v1/todos  - Create todo (auth required)")
	log.Println("   GET  /api/v1/admin/users - Admin only")
	
	if err := router.Run(":" + port); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}

// Simple Todo structure
type Todo struct {
	ID     int    `json:"id"`
	Title  string `json:"title"`
	Done   bool   `json:"done"`
	UserID string `json:"user_id"`
}

// In-memory storage for demo
var todos = []Todo{
	{ID: 1, Title: "Learn AuthMesh", Done: false, UserID: "demo-user"},
	{ID: 2, Title: "Build awesome API", Done: false, UserID: "demo-user"},
}
var nextID = 3

func setupTodoAPI(router *gin.Engine, authMesh *platform.Platform) {
	// Public API info
	router.GET("/api", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"name":    "Todo API",
			"version": "1.0.0",
			"powered_by": "AuthMesh",
			"endpoints": gin.H{
				"todos": "/api/v1/todos",
				"admin": "/api/v1/admin",
				"auth":  "/whoami",
			},
		})
	})

	// Protected API routes
	api := router.Group("/api/v1")
	api.Use(authMesh.AuthMiddleware()) // Require authentication
	{
		// List todos for authenticated user
		api.GET("/todos", func(c *gin.Context) {
			userSub, _ := auth.GetUserSubjectFromContext(c)
			
			// Filter todos by user (in real app, query database)
			userTodos := []Todo{}
			for _, todo := range todos {
				if todo.UserID == userSub {
					userTodos = append(userTodos, todo)
				}
			}
			
			c.JSON(http.StatusOK, gin.H{
				"todos": userTodos,
				"user":  userSub,
			})
		})

		// Create new todo
		api.POST("/todos", func(c *gin.Context) {
			userSub, _ := auth.GetUserSubjectFromContext(c)
			
			var newTodo struct {
				Title string `json:"title" binding:"required"`
			}
			
			if err := c.ShouldBindJSON(&newTodo); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}
			
			todo := Todo{
				ID:     nextID,
				Title:  newTodo.Title,
				Done:   false,
				UserID: userSub,
			}
			nextID++
			todos = append(todos, todo)
			
			c.JSON(http.StatusCreated, gin.H{
				"todo":    todo,
				"message": "Todo created successfully",
			})
		})

		// Toggle todo status
		api.PUT("/todos/:id", func(c *gin.Context) {
			userSub, _ := auth.GetUserSubjectFromContext(c)
			id := c.Param("id")
			
			// Find and update todo (simplified for demo)
			for i, todo := range todos {
				if todo.ID == parseInt(id) && todo.UserID == userSub {
					todos[i].Done = !todos[i].Done
					c.JSON(http.StatusOK, gin.H{
						"todo":    todos[i],
						"message": "Todo updated successfully",
					})
					return
				}
			}
			
			c.JSON(http.StatusNotFound, gin.H{"error": "Todo not found"})
		})
	}

	// Admin routes (uses AuthMesh's standard admin routes)
	admin := router.Group("/api/v1/admin")
	admin.Use(authMesh.AuthMiddleware("admin"))
	{
		// List all todos (admin only)
		admin.GET("/todos", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{
				"todos": todos,
				"total": len(todos),
			})
		})

		// User management
		admin.GET("/users", func(c *gin.Context) {
			// Get unique users from todos
			users := make(map[string]int)
			for _, todo := range todos {
				users[todo.UserID]++
			}
			
			c.JSON(http.StatusOK, gin.H{
				"users": users,
				"message": "User list with todo counts",
			})
		})
	}
}

// Helper functions
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func parseInt(s string) int {
	// Simplified for demo - in real app use strconv.Atoi with error handling
	if s == "1" { return 1 }
	if s == "2" { return 2 }
	if s == "3" { return 3 }
	return 0
}
