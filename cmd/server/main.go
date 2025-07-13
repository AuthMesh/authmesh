// @title Multi-Tenant Platform API
// @version 1.0
// @description This is the backend API for the Multi-Tenant Platform system.
// @host localhost:8081
// @BasePath /
// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
// @description Enter your token with the prefix: Bearer <token>

package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"golang-multi-tenant/internal/audit"
	"golang-multi-tenant/internal/auth"
	"golang-multi-tenant/internal/db"
	"golang-multi-tenant/internal/handlers"
	"golang-multi-tenant/internal/middleware"
	"golang-multi-tenant/pkg/config"
	"golang-multi-tenant/pkg/ratelimit"
)

// --- In-memory fallback rate limiter ---
var fallbackBucket = ratelimit.NewTokenBucket(100, 200)

func simpleInMemoryRateLimitMiddleware(rate, burst int) gin.HandlerFunc {
	fallbackBucket = ratelimit.NewTokenBucket(rate, burst)
	return func(c *gin.Context) {
		if !fallbackBucket.Allow() {
			c.Header("Retry-After", "1")
			c.JSON(429, gin.H{"error": "Rate limit exceeded (in-memory fallback)"})
			c.Abort()
			return
		}
		c.Next()
	}
}

func main() {
	// ...existing code from oldCode/cmd/server/main.go...
}
