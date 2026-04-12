package ratelimit

import (
	"sync"
)

// RateLimitCache provides in-memory caching for per-tenant rate limits (per-second)
type RateLimitCache struct {
	   tenantLimits map[string]float64 // e.g., "tenant1" -> 5000.0
	   mu sync.RWMutex
}

// NewRateLimitCache creates a new rate limit cache instance
func NewRateLimitCache() *RateLimitCache {
	   return &RateLimitCache{
			   tenantLimits: make(map[string]float64),
	   }
}

// GetTenantRateLimitCached retrieves the cached per-tenant rate limit (per-second)
func (c *RateLimitCache) GetTenantRateLimitCached(tenantID string) (float64, bool) {
	   c.mu.RLock()
	   defer c.mu.RUnlock()
	   limit, exists := c.tenantLimits[tenantID]
	   return limit, exists
}

// UpdateTenantRateLimitCache updates the cached per-tenant rate limit (per-second)
func (c *RateLimitCache) UpdateTenantRateLimitCache(tenantID string, limit float64) {
	   c.mu.Lock()
	   defer c.mu.Unlock()
	   c.tenantLimits[tenantID] = limit
}

// ClearCache clears all cached tenant rate limits (useful for testing)
func (c *RateLimitCache) ClearCache() {
	   c.mu.Lock()
	   defer c.mu.Unlock()
	   c.tenantLimits = make(map[string]float64)
}

// GetAllTenantLimits returns a copy of all cached tenant rate limits (for debugging/monitoring)
func (c *RateLimitCache) GetAllTenantLimits() map[string]float64 {
	   c.mu.RLock()
	   defer c.mu.RUnlock()
	   copy := make(map[string]float64)
	   for k, v := range c.tenantLimits {
			   copy[k] = v
	   }
	   return copy
}
