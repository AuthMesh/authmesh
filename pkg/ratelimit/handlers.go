package ratelimit

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// TenantRateLimitCache manages rate limit configurations for tenants
type TenantRateLimitCache struct {
	redisClient *redis.Client
}

// NewTenantRateLimitCache creates a new tenant rate limit cache
func NewTenantRateLimitCache(redisClient *redis.Client) *TenantRateLimitCache {
	return &TenantRateLimitCache{
		redisClient: redisClient,
	}
}

// UpdateTenantRateLimitCache updates the cached rate limit for a tenant
func (cache *TenantRateLimitCache) UpdateTenantRateLimitCache(tenantID string, maxRequestsPerSecond float64) error {
	ctx := context.Background()
	key := fmt.Sprintf("tenant_rate_limit:%s", tenantID)
	return cache.redisClient.Set(ctx, key, maxRequestsPerSecond, time.Hour).Err()
}

// GetTenantRateLimitCached retrieves the cached rate limit for a tenant
func (cache *TenantRateLimitCache) GetTenantRateLimitCached(tenantID string) (float64, bool) {
	ctx := context.Background()
	key := fmt.Sprintf("tenant_rate_limit:%s", tenantID)
	val, err := cache.redisClient.Get(ctx, key).Float64()
	if err != nil {
		return 0, false
	}
	return val, true
}

// GetTenantRateLimit retrieves rate limit settings from Redis
func (cache *TenantRateLimitCache) GetTenantRateLimit(tenantID string) (map[string]interface{}, error) {
	ctx := context.Background()
	key := fmt.Sprintf("realm_rate_limits:%s", tenantID)
	
	data, err := cache.redisClient.Get(ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			return nil, nil // Not found
		}
		return nil, err
	}
	
	var rateLimits map[string]interface{}
	if err := json.Unmarshal([]byte(data), &rateLimits); err != nil {
		return nil, err
	}
	
	return rateLimits, nil
}

// SetTenantRateLimit stores rate limit settings in Redis
func (cache *TenantRateLimitCache) SetTenantRateLimit(tenantID string, rateLimits map[string]interface{}) error {
	ctx := context.Background()
	key := fmt.Sprintf("realm_rate_limits:%s", tenantID)
	
	data, err := json.Marshal(rateLimits)
	if err != nil {
		return err
	}
	
	return cache.redisClient.Set(ctx, key, data, 24*time.Hour).Err()
}

// SuperAdminHandler handles superadmin operations for realm rate limit management
type SuperAdminHandler struct {
	logger      *zap.Logger
	redisClient *redis.Client
	cache       *TenantRateLimitCache
}

// NewSuperAdminHandler creates a new superadmin handler for rate limits
func NewSuperAdminHandler(redisClient *redis.Client, logger *zap.Logger) (*SuperAdminHandler, error) {
	if redisClient == nil {
		return nil, fmt.Errorf("redis client is required")
	}
	if logger == nil {
		return nil, fmt.Errorf("logger is required")
	}
	
	cache := NewTenantRateLimitCache(redisClient)
	
	return &SuperAdminHandler{
		logger:      logger,
		redisClient: redisClient,
		cache:       cache,
	}, nil
}

// SetRealmRateLimitsRequest represents the request body for setting realm rate limits
type SetRealmRateLimitsRequest struct {
	MaxRequestsPerSecond     float64 `json:"max_requests_per_second" binding:"required,min=0"`
	PerUserRequestsPerSecond float64 `json:"per_user_requests_per_second,omitempty"`
	PerUserBurst             int     `json:"per_user_burst,omitempty"`
}

// RealmRateLimitsResponse represents the response for realm rate limits
type RealmRateLimitsResponse struct {
	RealmID                  string  `json:"realm_id"`
	MaxRequestsPerSecond     float64 `json:"max_requests_per_second"`
	PerUserRequestsPerSecond float64 `json:"per_user_requests_per_second,omitempty"`
	PerUserBurst             int     `json:"per_user_burst,omitempty"`
	Message                  string  `json:"message,omitempty"`
}

// SetRealmRateLimits handles PUT /api/v1/superadmin/realms/{realm_id}/rate-limits
func (h *SuperAdminHandler) SetRealmRateLimits(c *gin.Context) {
	realmID := c.Param("realm_id")

	var req SetRealmRateLimitsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Error("Invalid request body for set realm rate limits",
			zap.String("realm_id", realmID),
			zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body", "details": err.Error()})
		return
	}

	// Store complete rate limit settings
	rateLimits := map[string]interface{}{
		"max_requests_per_second":      req.MaxRequestsPerSecond,
		"per_user_requests_per_second": req.PerUserRequestsPerSecond,
		"per_user_burst":               req.PerUserBurst,
	}
	
	if err := h.cache.SetTenantRateLimit(realmID, rateLimits); err != nil {
		h.logger.Error("Failed to store rate limit settings",
			zap.String("realm_id", realmID),
			zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to store rate limit settings"})
		return
	}

	// Update the simple cache for backwards compatibility
	h.cache.UpdateTenantRateLimitCache(realmID, req.MaxRequestsPerSecond)

	// Also update Redis so middleware can pick up the new rate limit
	redisKey := fmt.Sprintf("ratelimit_tenant:golang-mt:%s", realmID)
	if err := h.redisClient.SetEx(context.Background(), redisKey, req.MaxRequestsPerSecond, 7*24*time.Hour).Err(); err != nil {
		h.logger.Error("Failed to update rate limit in Redis",
			zap.String("realm_id", realmID),
			zap.Float64("rate_limit", req.MaxRequestsPerSecond),
			zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update rate limit in Redis"})
		return
	}

	h.logger.Info("Successfully set realm rate limits",
		zap.String("realm_id", realmID),
		zap.Float64("limit", req.MaxRequestsPerSecond))

	c.JSON(http.StatusOK, RealmRateLimitsResponse{
		RealmID:                  realmID,
		MaxRequestsPerSecond:     req.MaxRequestsPerSecond,
		PerUserRequestsPerSecond: req.PerUserRequestsPerSecond,
		PerUserBurst:             req.PerUserBurst,
		Message:                  "Rate limits updated successfully",
	})
}

// GetRealmRateLimits handles GET /api/v1/superadmin/realms/{realm_id}/rate-limits
func (h *SuperAdminHandler) GetRealmRateLimits(c *gin.Context) {
	realmID := c.Param("realm_id")

	// Get the stored rate limit settings
	rateLimits, err := h.cache.GetTenantRateLimit(realmID)
	if err != nil {
		h.logger.Error("Failed to get rate limit settings",
			zap.String("realm_id", realmID),
			zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get rate limit settings"})
		return
	}

	var response RealmRateLimitsResponse
	response.RealmID = realmID

	if rateLimits != nil {
		// Parse stored settings
		if maxRate, ok := rateLimits["max_requests_per_second"].(float64); ok {
			response.MaxRequestsPerSecond = maxRate
		}
		if perUserRate, ok := rateLimits["per_user_requests_per_second"].(float64); ok {
			response.PerUserRequestsPerSecond = perUserRate
		}
		if perUserBurst, ok := rateLimits["per_user_burst"].(float64); ok {
			response.PerUserBurst = int(perUserBurst)
		}
	} else {
		// Use defaults if no settings stored
		response.MaxRequestsPerSecond = 1000.0   // Default fallback
		response.PerUserRequestsPerSecond = 10.0 // Default from milestone3 requirements
		response.PerUserBurst = 20               // Default from milestone3 requirements
	}

	c.JSON(http.StatusOK, response)
}
