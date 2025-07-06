package ratelimit

import (
	"net/http"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

// Config represents rate limiting configuration
type Config struct {
	RedisClient       *redis.Client
	RequestsPerSecond float64
	BurstSize         int
	TenantEnabled     bool
	TenantRequestsPerSecond float64
	TenantBurstSize   int
}

// Middleware creates a rate limiting middleware
func Middleware(config Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		// For now, implement a simple in-memory rate limiter
		// In production, this would use Redis for distributed rate limiting
		
		// Get client identifier (IP address or user ID)
		clientID := getClientID(c)
		
		// Create or get token bucket for this client
		bucket := getOrCreateBucket(clientID, config)
		
		// Check if request is allowed
		if !bucket.Allow() {
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error":   "Rate limit exceeded",
				"message": "Too many requests",
			})
			c.Abort()
			return
		}
		
		c.Next()
	}
}

// Simple in-memory store for buckets (in production, use Redis)
var bucketStore = make(map[string]*TokenBucket)
var bucketMutex sync.RWMutex

func getClientID(c *gin.Context) string {
	// Try to get user ID first, then fall back to IP
	if userID, exists := c.Get("user_id"); exists {
		return userID.(string)
	}
	return c.ClientIP()
}

func getOrCreateBucket(clientID string, config Config) *TokenBucket {
	bucketMutex.RLock()
	bucket, exists := bucketStore[clientID]
	bucketMutex.RUnlock()
	
	if exists {
		return bucket
	}
	
	// Create new bucket
	bucketMutex.Lock()
	defer bucketMutex.Unlock()
	
	// Double-check after acquiring write lock
	if bucket, exists := bucketStore[clientID]; exists {
		return bucket
	}
	
	// Create new token bucket
	bucket = NewTokenBucket(int(config.RequestsPerSecond), config.BurstSize)
	bucketStore[clientID] = bucket
	
	return bucket
}
