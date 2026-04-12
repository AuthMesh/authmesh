package auth

import (
	"context"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// RedisClient holds the Redis connection
var RedisClient *redis.Client

// Redis metrics for observability
var (
	redisOperationsTotal metric.Int64Counter
	redisErrorsTotal     metric.Int64Counter
	refreshTokenTTL      time.Duration = 7 * 24 * time.Hour // Default: 7 days, will be updated from config
)

// initRedisMetrics initializes Redis-specific telemetry metrics
func initRedisMetrics() {
	// Use the global otel meter provider to create a meter for Redis
	meter := otel.Meter("redis")

	var err error
	redisOperationsTotal, err = meter.Int64Counter(
		"redis_operations_total",
		metric.WithDescription("Total number of Redis operations"),
		metric.WithUnit("count"),
	)
	if err != nil {
		log.Printf("Warning: Failed to create redis_operations_total metric: %v", err)
	}

	redisErrorsTotal, err = meter.Int64Counter(
		"redis_errors_total",
		metric.WithDescription("Total number of Redis operation errors"),
		metric.WithUnit("count"),
	)
	if err != nil {
		log.Printf("Warning: Failed to create redis_errors_total metric: %v", err)
	}
}

// parseRefreshTokenTTL parses the REFRESH_TOKEN_TTL from environment
func parseRefreshTokenTTL() time.Duration {
	ttlStr := os.Getenv("REFRESH_TOKEN_TTL")
	if ttlStr == "" {
		return 7 * 24 * time.Hour // Default: 7 days
	}

	// Handle format like "7d", "24h", "3600s", etc.
	if duration, err := time.ParseDuration(ttlStr); err == nil {
		return duration
	}

	// Handle numeric values (assume seconds for backward compatibility)
	if ttlInt, err := strconv.Atoi(strings.TrimSpace(ttlStr)); err == nil {
		return time.Duration(ttlInt) * time.Second
	}

	// Fallback to default
	log.Printf("Warning: Invalid REFRESH_TOKEN_TTL format '%s', using default 7d", ttlStr)
	return 7 * 24 * time.Hour
}

// InitRedis initializes the Redis connection
func InitRedis(redisURL string) error {
	opts, err := redis.ParseURL(redisURL)
	if err != nil {
		return err
	}

	RedisClient = redis.NewClient(opts)

	// Initialize Redis metrics
	initRedisMetrics()

	// Parse refresh token TTL from environment
	refreshTokenTTL = parseRefreshTokenTTL()
	log.Printf("Redis: Using refresh token TTL: %v", refreshTokenTTL)

	// Test the connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err = RedisClient.Ping(ctx).Result()
	if err != nil {
		// Record error metric
		recordRedisError(ctx, "ping", err)
		return err
	}

	log.Println("Redis connection established")
	return nil
}

// RevokeToken stores a revoked token JTI in Redis with atomic set and expire operations
func RevokeToken(tenantID, userID, jti string, ttl time.Duration) error {
	if RedisClient == nil {
		return nil // Skip if Redis is not available
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	key := "revoked:" + tenantID + ":" + userID + ":" + jti

	// Use pipelined transaction for atomic set and expire
	pipe := RedisClient.TxPipeline()
	pipe.Set(ctx, key, "revoked", ttl)
	pipe.Expire(ctx, key, ttl)

	_, err := pipe.Exec(ctx)
	if err != nil {
		recordRedisError(ctx, "revoke_token", err)
		return err
	}

	recordRedisOperation(ctx, "revoke_token", true)
	log.Printf("Token revoked: tenant=%s, user=%s, jti=%s, ttl=%v", tenantID, userID, jti, ttl)
	return nil
}

// IsTokenRevoked checks if a token JTI is revoked and extends expiration if found
// The expiration extension aligns with the configured REFRESH_TOKEN_TTL to prevent
// premature eviction of active token revocation records
func IsTokenRevoked(tenantID, userID, jti string) bool {
	if RedisClient == nil {
		return false // Skip if Redis is not available
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	key := "revoked:" + tenantID + ":" + userID + ":" + jti

	// Use pipelined transaction for atomic get and expire extension
	pipe := RedisClient.TxPipeline()
	getResult := pipe.Get(ctx, key)
	// Extend expiration by 20% of refresh token TTL to maintain revocation during active use
	// This prevents premature eviction while avoiding indefinite storage
	extensionTTL := time.Duration(float64(refreshTokenTTL) * 0.2)
	if extensionTTL < 5*time.Minute {
		extensionTTL = 5 * time.Minute // Minimum 5 minutes
	}
	pipe.Expire(ctx, key, extensionTTL)

	_, err := pipe.Exec(ctx)
	if err != nil {
		// On Redis errors, assume token is not revoked (fail-safe approach)
		// This prevents false positives that would block valid users
		recordRedisError(ctx, "check_token_revoked", err)
		return false
	}

	recordRedisOperation(ctx, "check_token_revoked", true)

	result := getResult.Val()
	isRevoked := result == "revoked"

	if isRevoked {
		log.Printf("Token found revoked: tenant=%s, user=%s, jti=%s, extended_ttl=%v",
			tenantID, userID, jti, extensionTTL)
	}

	return isRevoked
}

// IsRedisReady checks if Redis is connected and ready
func IsRedisReady() bool {
	if RedisClient == nil {
		return false
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	_, err := RedisClient.Ping(ctx).Result()
	if err != nil {
		recordRedisError(ctx, "health_check", err)
		return false
	}

	recordRedisOperation(ctx, "health_check", true)
	return true
}

// GetRedisStatus returns detailed Redis status with metrics tracking
func GetRedisStatus() map[string]interface{} {
	if RedisClient == nil {
		return map[string]interface{}{
			"status": "not_initialized",
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	info, err := RedisClient.Info(ctx).Result()
	if err != nil {
		recordRedisError(ctx, "status_check", err)
		return map[string]interface{}{
			"status": "connection_failed",
			"error":  err.Error(),
		}
	}

	recordRedisOperation(ctx, "status_check", true)

	return map[string]interface{}{
		"status":            "ready",
		"connected":         true,
		"server_info":       len(info) > 0,
		"refresh_token_ttl": refreshTokenTTL.String(),
		"metrics_enabled":   redisOperationsTotal != nil && redisErrorsTotal != nil,
	}
}

// recordRedisOperation records a Redis operation metric
func recordRedisOperation(ctx context.Context, operation string, success bool) {
	if redisOperationsTotal == nil {
		return
	}

	attrs := []attribute.KeyValue{
		attribute.String("redis.operation", operation),
		attribute.Bool("redis.success", success),
	}

	redisOperationsTotal.Add(ctx, 1, metric.WithAttributes(attrs...))
}

// recordRedisError records a Redis error metric
func recordRedisError(ctx context.Context, operation string, err error) {
	if redisErrorsTotal == nil {
		return
	}

	attrs := []attribute.KeyValue{
		attribute.String("redis.operation", operation),
		attribute.String("redis.error", err.Error()),
	}

	redisErrorsTotal.Add(ctx, 1, metric.WithAttributes(attrs...))
	log.Printf("Redis %s error: %v", operation, err)
}
