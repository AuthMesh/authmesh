package middleware

import (
	"crypto/rand"
	"encoding/hex"

	"github.com/gin-gonic/gin"
)

// RequestIDMiddleware adds a unique request ID to each request
func RequestIDMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Check if request ID already exists in headers
		requestID := c.GetHeader("X-Request-ID")
		if requestID == "" {
			// Generate a new request ID
			bytes := make([]byte, 16)
			if _, err := rand.Read(bytes); err != nil {
				c.AbortWithStatus(500)
				return
			}
			requestID = hex.EncodeToString(bytes)
		}

		// Set the request ID in context and response header
		c.Set("request_id", requestID)
		c.Header("X-Request-ID", requestID)

		c.Next()
	}
}
