package middleware

import (
	"context"
	"fmt"
	"net/http"
	"runtime/debug"
	"time"

	"github.com/gin-gonic/gin"
)

// LoggingConfig represents logging middleware configuration
type LoggingConfig struct {
	EnableRequestLogging  bool `json:"enable_request_logging"`
	EnableResponseLogging bool `json:"enable_response_logging"`
	SkipPaths            []string `json:"skip_paths"`
}

// DefaultLoggingConfig returns a default logging configuration
func DefaultLoggingConfig() LoggingConfig {
	return LoggingConfig{
		EnableRequestLogging:  true,
		EnableResponseLogging: true,
		SkipPaths: []string{
			"/health",
			"/ready",
			"/metrics",
		},
	}
}

// RequestLoggingMiddleware logs HTTP requests
func RequestLoggingMiddleware(config LoggingConfig) gin.HandlerFunc {
	return gin.LoggerWithConfig(gin.LoggerConfig{
		Formatter: func(param gin.LogFormatterParams) string {
			return fmt.Sprintf("[%s] %s %s %d %s %s\n",
				param.TimeStamp.Format("2006-01-02 15:04:05"),
				param.Method,
				param.Path,
				param.StatusCode,
				param.Latency,
				param.ClientIP,
			)
		},
		SkipPaths: config.SkipPaths,
	})
}

// RecoveryMiddleware handles panics and returns a proper error response
func RecoveryMiddleware() gin.HandlerFunc {
	return gin.CustomRecovery(func(c *gin.Context, recovered interface{}) {
		// Log the panic
		fmt.Printf("[PANIC RECOVERED] %v\n", recovered)
		fmt.Printf("Stack trace:\n%s\n", debug.Stack())

		// Return a proper error response
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Internal server error",
			"message": "An unexpected error occurred",
		})
	})
}

// RequestIDMiddleware adds a unique request ID to each request
func RequestIDMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Check if request ID already exists (from proxy or load balancer)
		requestID := c.GetHeader("X-Request-ID")
		if requestID == "" {
			// Generate a simple request ID (in production, use UUID)
			requestID = fmt.Sprintf("%d", time.Now().UnixNano())
		}

		// Set request ID in context and response header
		c.Set("request_id", requestID)
		c.Header("X-Request-ID", requestID)

		c.Next()
	}
}

// ResponseTimeMiddleware adds response time headers
func ResponseTimeMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()

		c.Next()

		// Calculate response time
		duration := time.Since(start)
		c.Header("X-Response-Time", duration.String())
	}
}

// TimeoutMiddleware adds request timeout handling
func TimeoutMiddleware(timeout time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Create a context with timeout
		ctx, cancel := c.Request.Context(), func() {}
		if timeout > 0 {
			ctx, cancel = context.WithTimeout(c.Request.Context(), timeout)
		}
		defer cancel()

		// Update request context
		c.Request = c.Request.WithContext(ctx)

		// Channel to signal completion
		finished := make(chan struct{})
		panicChan := make(chan interface{}, 1)

		go func() {
			defer func() {
				if p := recover(); p != nil {
					panicChan <- p
				}
			}()
			c.Next()
			finished <- struct{}{}
		}()

		select {
		case <-finished:
			// Request completed normally
		case p := <-panicChan:
			// Panic occurred
			panic(p)
		case <-ctx.Done():
			// Timeout occurred
			c.JSON(http.StatusRequestTimeout, gin.H{
				"error":   "Request timeout",
				"message": "Request took too long to process",
			})
			c.Abort()
		}
	}
}

// HealthCheckMiddleware provides health check endpoints
func HealthCheckMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.URL.Path == "/health" {
			c.JSON(http.StatusOK, gin.H{
				"status":    "healthy",
				"timestamp": time.Now().UTC().Format(time.RFC3339),
			})
			c.Abort()
			return
		}

		if c.Request.URL.Path == "/ready" {
			// Add readiness checks here (database, external services, etc.)
			c.JSON(http.StatusOK, gin.H{
				"status":    "ready",
				"timestamp": time.Now().UTC().Format(time.RFC3339),
			})
			c.Abort()
			return
		}

		c.Next()
	}
}
