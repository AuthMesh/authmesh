package middleware

import (
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

// CORSConfig represents CORS configuration
type CORSConfig struct {
	AllowOrigins     []string      `json:"allow_origins"`
	AllowMethods     []string      `json:"allow_methods"`
	AllowHeaders     []string      `json:"allow_headers"`
	ExposeHeaders    []string      `json:"expose_headers"`
	AllowCredentials bool          `json:"allow_credentials"`
	MaxAge           time.Duration `json:"max_age"`
}

// DefaultCORSConfig returns a secure default CORS configuration
func DefaultCORSConfig() CORSConfig {
	return CORSConfig{
		AllowOrigins: []string{
			"http://localhost:3000",
			"http://localhost:8080",
			"https://localhost:3000",
			"https://localhost:8080",
		},
		AllowMethods: []string{
			"GET",
			"POST", 
			"PUT",
			"PATCH",
			"DELETE",
			"OPTIONS",
		},
		AllowHeaders: []string{
			"Origin",
			"Content-Type",
			"Content-Length",
			"Accept-Encoding",
			"X-CSRF-Token",
			"Authorization",
			"Accept",
			"Cache-Control",
			"X-Requested-With",
			"X-Tenant-ID",
		},
		ExposeHeaders: []string{
			"Content-Length",
			"X-Tenant-ID",
		},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}
}

// SetupCORS configures CORS middleware with the given configuration
func SetupCORS(config CORSConfig) gin.HandlerFunc {
	corsConfig := cors.Config{
		AllowOrigins:     config.AllowOrigins,
		AllowMethods:     config.AllowMethods,
		AllowHeaders:     config.AllowHeaders,
		ExposeHeaders:    config.ExposeHeaders,
		AllowCredentials: config.AllowCredentials,
		MaxAge:           config.MaxAge,
	}

	return cors.New(corsConfig)
}

// SetupDefaultCORS configures CORS middleware with secure defaults
func SetupDefaultCORS() gin.HandlerFunc {
	return SetupCORS(DefaultCORSConfig())
}

// ProductionCORSConfig returns a production-ready CORS configuration
func ProductionCORSConfig(allowedOrigins []string) CORSConfig {
	config := DefaultCORSConfig()
	
	// In production, use explicit allowed origins
	if len(allowedOrigins) > 0 {
		config.AllowOrigins = allowedOrigins
	} else {
		// Default to no origins in production (must be explicitly configured)
		config.AllowOrigins = []string{}
	}
	
	// Reduce max age in production
	config.MaxAge = 1 * time.Hour
	
	return config
}

// DevelopmentCORSConfig returns a development-friendly CORS configuration
func DevelopmentCORSConfig() CORSConfig {
	config := DefaultCORSConfig()
	
	// In development, allow common localhost ports
	config.AllowOrigins = append(config.AllowOrigins,
		"http://localhost:3001",
		"http://localhost:3002",
		"http://localhost:4200", // Angular default
		"http://localhost:5173", // Vite default
		"http://localhost:8000", // Common dev server
	)
	
	return config
}
