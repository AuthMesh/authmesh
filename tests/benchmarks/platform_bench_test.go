package benchmarks

import (
	"context"
	"net/http/httptest"
	"testing"
	

	"github.com/AuthMesh/authmesh/pkg/platform"
	"github.com/AuthMesh/authmesh/pkg/auth"
	"github.com/AuthMesh/authmesh/pkg/ratelimit"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v4"
)

// BenchmarkPlatformSetup measures the time to initialize the platform
func BenchmarkPlatformSetup(b *testing.B) {
	gin.SetMode(gin.ReleaseMode)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := platform.NewForTesting("benchmark")
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkMiddlewareStack measures full middleware stack performance
func BenchmarkMiddlewareStack(b *testing.B) {
	gin.SetMode(gin.ReleaseMode)
	
	authMesh, err := platform.NewForTesting("middleware-bench")
	if err != nil {
		b.Fatal(err)
	}
	
	router := gin.New()
	authMesh.SetupMiddleware(router)
	
	// Simple test endpoint
	router.GET("/bench", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})
	
	// Create request
	req := httptest.NewRequest("GET", "/bench", nil)
	req.Header.Set("User-Agent", "benchmark-test")
	req.Header.Set("X-Real-IP", "127.0.0.1")
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
	}
}

// BenchmarkRateLimiting measures rate limiting performance
func BenchmarkRateLimiting(b *testing.B) {
	limiter := ratelimit.NewTokenBucket(1000, 1000)
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		allowed := limiter.Allow()
		if !allowed && i < 1000 {
			b.Fatal("Expected request to be allowed")
		}
	}
}

// BenchmarkJWTValidation measures JWT token validation performance
func BenchmarkJWTValidation(b *testing.B) {
	// Create JWKS registry for testing
	registry := auth.NewJWKSRegistry("http://localhost:8080", "master", []string{"http://localhost:8080"})
	
	// Create a test JWT token (this measures parsing overhead)
	tokenString := "eyJhbGciOiJSUzI1NiIsInR5cCIgOiAiSldUIiwia2lkIiA6ICJyS2dCbGJUV3o0N3B0bmlFNnQyZFRkV2pUbGxmdjN2VkpkcGtWdjdKR3c4In0"
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// This benchmarks the token parsing overhead
		_, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
			return []byte("test-secret"), nil
		})
		_ = err // Ignore errors for benchmark (token will be invalid without proper key)
		_ = registry // Use registry to avoid unused variable
	}
}

// BenchmarkConcurrentRequests measures performance under concurrent load
func BenchmarkConcurrentRequests(b *testing.B) {
	gin.SetMode(gin.ReleaseMode)
	
	authMesh, err := platform.NewForTesting("concurrent-bench")
	if err != nil {
		b.Fatal(err)
	}
	
	router := gin.New()
	authMesh.SetupMiddleware(router)
	
	router.GET("/concurrent", func(c *gin.Context) {
		c.JSON(200, gin.H{"message": "success"})
	})
	
	req := httptest.NewRequest("GET", "/concurrent", nil)
	
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)
		}
	})
}

// BenchmarkPlatformShutdown measures graceful shutdown performance
func BenchmarkPlatformShutdown(b *testing.B) {
	gin.SetMode(gin.ReleaseMode)
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		authMesh, err := platform.NewForTesting("shutdown-bench")
		if err != nil {
			b.Fatal(err)
		}
		
		err = authMesh.Shutdown(context.Background())
		if err != nil {
			b.Fatal(err)
		}
	}
}
