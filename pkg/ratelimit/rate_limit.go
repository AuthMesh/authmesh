package ratelimit

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// Allow checks and updates the token bucket for a given key (per-user or per-tenant)
// Returns true if allowed, false if rate limited
func Allow(ctx context.Context, rdb *redis.Client, key string, rate float64, burst int, expiry time.Duration) (bool, error) {
	// Lua script for atomic token bucket
	tokenBucketLua := `
local key = KEYS[1]
local rate = tonumber(ARGV[1])
local burst = tonumber(ARGV[2])
local now = tonumber(ARGV[3])
local expiry = tonumber(ARGV[4])
local tokens = tonumber(redis.call('get', key) or burst)
local last_refill = tonumber(redis.call('get', key..':last_refill') or now)
local elapsed = now - last_refill
tokens = math.min(burst, tokens + elapsed * rate)
if tokens >= 1 then
  tokens = tokens - 1
  redis.call('set', key, tokens, 'EX', expiry)
  redis.call('set', key..':last_refill', now, 'EX', expiry)
  return 1
else
  redis.call('set', key, tokens, 'EX', expiry)
  redis.call('set', key..':last_refill', now, 'EX', expiry)
  return 0
end
`

	now := time.Now().Unix()
	result, err := rdb.Eval(ctx, tokenBucketLua, []string{key}, rate, burst, now, int(expiry.Seconds())).Result()
	if err != nil {
		return false, fmt.Errorf("redis token bucket error: %w", err)
	}
	allowed, ok := result.(int64)
	return ok && allowed == 1, nil
}
