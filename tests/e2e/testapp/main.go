package main

import (
	"log"
	"net/http"
	"os"

	"github.com/AuthMesh/authmesh/pkg/platform"
	"github.com/gin-gonic/gin"
)

func main() {
	gin.SetMode(gin.ReleaseMode)

	config := platform.DefaultConfig()
	config.Keycloak.URL = ""
	config.Keycloak.SkipJWKSInit = true

	// Use Redis if REDIS_URL is set
	if redisURL := os.Getenv("REDIS_URL"); redisURL != "" {
		config.Redis.URL = redisURL
	} else {
		config.Redis.URL = ""
	}

	config.RateLimit.Enabled = true
	config.RateLimit.RequestsPerSecond = 10.0
	config.RateLimit.BurstSize = 20
	config.Observability.EnableMetrics = true
	config.Observability.EnableTracing = false

	config.Security.EnableSecurityHeaders = true
	config.Security.EnableSSRFProtection = true
	config.Security.AllowedHosts = []string{"localhost", "127.0.0.1"}

	authMesh, err := platform.New(config)
	if err != nil {
		log.Fatalf("Failed to initialize: %v", err)
	}

	router := gin.New()
	router.Use(gin.Recovery())
	authMesh.SetupMiddleware(router)
	authMesh.SetupRoutes(router)

	router.GET("/", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"message": "E2E Test App",
		})
	})

	log.Println("E2E test app starting on :8080")
	router.Run(":8080")
}
