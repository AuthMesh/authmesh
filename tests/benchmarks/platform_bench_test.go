package benchmarks

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/AuthMesh/authmesh/pkg/platform"
	"github.com/gin-gonic/gin"
)

// BenchmarkPlatformSetup measures the time to initialize the platform
func BenchmarkPlatformSetup(b *testing.B) {
	gin.SetMode(gin.ReleaseMode)
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := platform.NewForTesting("bench-test")
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkQuickStart measures the QuickStart convenience method
func BenchmarkQuickStart(b *testing.B) {
	gin.SetMode(gin.ReleaseMode)
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := platform.NewForTesting("quick-start-bench")
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkMiddlewareSetup measures middleware configuration time
func BenchmarkMiddlewareSetup(b *testing.B) {
	gin.SetMode(gin.ReleaseMode)
	
	// Setup platform once
	authMesh, err := platform.NewForTesting("middleware-bench")
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		router := gin.New()
		authMesh.SetupMiddleware(router)
	}
}

// BenchmarkUnifiedSetup measures the SetupAll convenience method
func BenchmarkUnifiedSetup(b *testing.B) {
	gin.SetMode(gin.ReleaseMode)
	
	// Setup platform once
	authMesh, err := platform.NewForTesting("unified-bench")
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		router := gin.New()
		authMesh.SetupAll(router)
	}
}

// BenchmarkHTTPRequest measures request processing through the middleware stack
func BenchmarkHTTPRequest(b *testing.B) {
	gin.SetMode(gin.ReleaseMode)
	
	// Setup platform and router
	authMesh, err := platform.NewForTesting("http-bench")
	if err != nil {
		b.Fatal(err)
	}

	router := gin.New()
	authMesh.SetupMiddleware(router)
	
	// Add a simple endpoint
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "test"})
	})

	// Create test server
	ts := httptest.NewServer(router)
	defer ts.Close()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		client := &http.Client{
			Timeout: 10 * time.Second,
		}
		
		for pb.Next() {
			resp, err := client.Get(ts.URL + "/test")
			if err != nil {
				b.Fatal(err)
			}
			resp.Body.Close()
		}
	})
}

// BenchmarkHealthCheck measures health check endpoint performance
func BenchmarkHealthCheck(b *testing.B) {
	gin.SetMode(gin.ReleaseMode)
	
	// Setup platform and router
	authMesh, err := platform.NewForTesting("health-bench")
	if err != nil {
		b.Fatal(err)
	}

	router := gin.New()
	authMesh.SetupAll(router)

	// Create test server
	ts := httptest.NewServer(router)
	defer ts.Close()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		client := &http.Client{
			Timeout: 10 * time.Second,
		}
		
		for pb.Next() {
			resp, err := client.Get(ts.URL + "/health")
			if err != nil {
				b.Fatal(err)
			}
			resp.Body.Close()
		}
	})
}

// BenchmarkPlatformShutdown measures graceful shutdown performance
func BenchmarkPlatformShutdown(b *testing.B) {
	gin.SetMode(gin.ReleaseMode)
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		
		authMesh, err := platform.NewForTesting("shutdown-bench")
		if err != nil {
			b.Fatal(err)
		}
		
		b.StartTimer()
		
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		err = authMesh.Shutdown(ctx)
		cancel()
		
		if err != nil {
			b.Fatal(err)
		}
	}
}
