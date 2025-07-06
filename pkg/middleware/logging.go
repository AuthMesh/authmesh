package middleware

import (
	"context" 
	"fmt"
	"net/http"
	"runtime/debug"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// LoggingConfig represents logging middleware configuration
type LoggingConfig struct {
	Logger         *zap.Logger
	SkipPaths      []string
	EnableBody     bool
	EnableHeaders  bool
	MaxBodySize    int64
}

// DefaultLoggingConfig returns a default logging configuration
func DefaultLoggingConfig() LoggingConfig {
	logger, _ := zap.NewProduction()
	return LoggingConfig{
		Logger:        logger,
		SkipPaths:     []string{"/health", "/metrics", "/ping"},
		EnableBody:    false,
		EnableHeaders: false,
		MaxBodySize:   1024, // 1KB
	}
}

// StructuredLoggingMiddleware provides structured request/response logging
func StructuredLoggingMiddleware(config LoggingConfig) gin.HandlerFunc {
	return gin.HandlerFunc(func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path

		// Skip logging for certain paths
		for _, skipPath := range config.SkipPaths {
			if path == skipPath {
				c.Next()
				return
			}
		}

		// Process request
		c.Next()

		// Calculate latency
		latency := time.Since(start)

		// Get client IP
		clientIP := c.ClientIP()

		// Build log fields
		fields := []zapcore.Field{
			zap.String("method", c.Request.Method),
			zap.String("path", path),
			zap.String("query", c.Request.URL.RawQuery),
			zap.Int("status", c.Writer.Status()),
			zap.Duration("latency", latency),
			zap.String("client_ip", clientIP),
			zap.String("user_agent", c.Request.UserAgent()),
			zap.Int("body_size", c.Writer.Size()),
		}

		// Add request ID if available
		if requestID, exists := c.Get("request_id"); exists {
			fields = append(fields, zap.String("request_id", requestID.(string)))
		}

		// Add tenant ID if available
		if tenantID, exists := c.Get("tenant_id"); exists {
			fields = append(fields, zap.String("tenant_id", tenantID.(string)))
		}

		// Add user ID if available
		if userID, exists := c.Get("user_id"); exists {
			fields = append(fields, zap.String("user_id", userID.(string)))
		}

		// Add error information if request failed
		if len(c.Errors) > 0 {
			fields = append(fields, zap.String("errors", c.Errors.String()))
		}

		// Add headers if enabled (be careful with sensitive data)
		if config.EnableHeaders {
			headers := make(map[string]string)
			for name, values := range c.Request.Header {
				if len(values) > 0 && !isSensitiveHeader(name) {
					headers[name] = values[0]
				}
			}
			if len(headers) > 0 {
				fields = append(fields, zap.Any("headers", headers))
			}
		}

		// Log with appropriate level based on status code
		switch {
		case c.Writer.Status() >= 500:
			config.Logger.Error("HTTP request completed", fields...)
		case c.Writer.Status() >= 400:
			config.Logger.Warn("HTTP request completed", fields...)
		default:
			config.Logger.Info("HTTP request completed", fields...)
		}
	})
}

// AuditLoggingMiddleware provides detailed audit logging for sensitive operations
func AuditLoggingMiddleware(logger *zap.Logger) gin.HandlerFunc {
	return gin.HandlerFunc(func(c *gin.Context) {
		start := time.Now()

		// Only log for non-GET requests or admin paths
		if c.Request.Method != "GET" || isAdminPath(c.Request.URL.Path) {
			// Log request details
			logger.Info("Audit: Request initiated",
				zap.String("method", c.Request.Method),
				zap.String("path", c.Request.URL.Path),
				zap.String("client_ip", c.ClientIP()),
				zap.String("user_agent", c.Request.UserAgent()),
				zap.Time("timestamp", start),
			)
		}

		c.Next()

		// Log completion for audited requests
		if c.Request.Method != "GET" || isAdminPath(c.Request.URL.Path) {
			latency := time.Since(start)
			
			fields := []zapcore.Field{
				zap.String("method", c.Request.Method),
				zap.String("path", c.Request.URL.Path),
				zap.Int("status", c.Writer.Status()),
				zap.Duration("latency", latency),
				zap.String("client_ip", c.ClientIP()),
			}

			// Add context information
			if requestID, exists := c.Get("request_id"); exists {
				fields = append(fields, zap.String("request_id", requestID.(string)))
			}
			if tenantID, exists := c.Get("tenant_id"); exists {
				fields = append(fields, zap.String("tenant_id", tenantID.(string)))
			}
			if userID, exists := c.Get("user_id"); exists {
				fields = append(fields, zap.String("user_id", userID.(string)))
			}

			logger.Info("Audit: Request completed", fields...)
		}
	})
}

// RequestLoggingMiddleware provides simple request logging (deprecated - use StructuredLoggingMiddleware)
func RequestLoggingMiddleware() gin.HandlerFunc {
	return gin.Logger()
}

// RecoveryWithLoggingMiddleware handles panics and returns a proper error response
func RecoveryWithLoggingMiddleware(logger *zap.Logger) gin.HandlerFunc {
	return gin.CustomRecovery(func(c *gin.Context, recovered interface{}) {
		if requestID, exists := c.Get("request_id"); exists {
			c.Header("X-Request-ID", requestID.(string))
		}
		
		// Log the panic
		logger.Error("Panic recovered",
			zap.Any("panic", recovered),
			zap.String("path", c.Request.URL.Path),
			zap.String("method", c.Request.Method),
			zap.String("client_ip", c.ClientIP()),
		)
		
		c.JSON(500, gin.H{
			"error":      "Internal server error",
			"request_id": c.GetHeader("X-Request-ID"),
		})
	})
}

// isSensitiveHeader checks if a header contains sensitive information
func isSensitiveHeader(name string) bool {
	sensitive := []string{
		"authorization",
		"cookie",
		"x-api-key",
		"x-auth-token",
		"x-session-id",
	}
	
	lowerName := strings.ToLower(name)
	for _, s := range sensitive {
		if lowerName == s {
			return true
		}
	}
	return false
}

// isAdminPath checks if the path is an administrative endpoint
func isAdminPath(path string) bool {
	adminPaths := []string{
		"/admin",
		"/api/admin",
		"/api/v1/admin",
		"/superadmin",
		"/api/superadmin",
		"/api/v1/superadmin",
	}
	
	for _, adminPath := range adminPaths {
		if strings.HasPrefix(path, adminPath) {
			return true
		}
	}
	return false
}
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
