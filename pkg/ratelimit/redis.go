package ratelimit

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

// AlgorithmType represents different rate limiting algorithms
type AlgorithmType string

const (
	TokenBucketAlgorithm    AlgorithmType = "token_bucket"
	SlidingWindow           AlgorithmType = "sliding_window"
	FixedWindow             AlgorithmType = "fixed_window"
)

// RedisLimiter implements distributed rate limiting using Redis
type RedisLimiter struct {
	client *redis.Client
}

// NewRedisLimiter creates a new Redis-based rate limiter
func NewRedisLimiter(client *redis.Client) *RedisLimiter {
	return &RedisLimiter{
		client: client,
	}
}

// Allow checks if a request is allowed for the given key and rate limit
func (r *RedisLimiter) Allow(ctx context.Context, key string, limit float64, burst int, window time.Duration) (bool, error) {
	// Use Redis with a sliding window rate limiter
	return r.slidingWindowRateLimit(ctx, key, limit, burst, window)
}

// slidingWindowRateLimit implements a sliding window rate limiter using Redis
func (r *RedisLimiter) slidingWindowRateLimit(ctx context.Context, key string, limit float64, burst int, window time.Duration) (bool, error) {
	now := time.Now()
	pipeline := r.client.Pipeline()

	// Remove old entries outside the window
	windowStart := now.Add(-window)
	pipeline.ZRemRangeByScore(ctx, key, "0", strconv.FormatInt(windowStart.UnixNano(), 10))

	// Count current requests in the window
	pipeline.ZCard(ctx, key)

	// Add current request
	pipeline.ZAdd(ctx, key, redis.Z{
		Score:  float64(now.UnixNano()),
		Member: fmt.Sprintf("%d-%d", now.UnixNano(), now.Unix()),
	})

	// Set expiration
	pipeline.Expire(ctx, key, window*2)

	// Execute pipeline
	results, err := pipeline.Exec(ctx)
	if err != nil {
		return false, err
	}

	// Get the count result (second command)
	countCmd := results[1].(*redis.IntCmd)
	currentCount, err := countCmd.Result()
	if err != nil {
		return false, err
	}

	// Check if request is allowed
	return currentCount < int64(burst), nil
}

// RedisTokenBucket implements a Redis-based token bucket algorithm
type RedisTokenBucket struct {
	client *redis.Client
}

// NewTokenBucketRedis creates a new Redis-based token bucket
func NewTokenBucketRedis(client *redis.Client) *RedisTokenBucket {
	return &RedisTokenBucket{
		client: client,
	}
}

// Allow checks if a token is available using Redis-based token bucket
func (tb *RedisTokenBucket) Allow(ctx context.Context, key string, rate float64, capacity int) (bool, error) {
	script := `
		local key = KEYS[1]
		local capacity = tonumber(ARGV[1])
		local rate = tonumber(ARGV[2])
		local now = tonumber(ARGV[3])
		
		-- Get current bucket state
		local bucket = redis.call('hmget', key, 'tokens', 'last_refill')
		local tokens = tonumber(bucket[1]) or capacity
		local last_refill = tonumber(bucket[2]) or now
		
		-- Calculate tokens to add based on time passed
		local time_passed = math.max(0, now - last_refill)
		local tokens_to_add = time_passed * rate / 1000000000  -- rate per nanosecond
		tokens = math.min(capacity, tokens + tokens_to_add)
		
		-- Check if we can consume a token
		if tokens >= 1 then
			tokens = tokens - 1
			-- Update bucket state
			redis.call('hmset', key, 'tokens', tokens, 'last_refill', now)
			redis.call('expire', key, 3600)  -- 1 hour expiry
			return 1
		else
			-- Update last_refill time even if no token consumed
			redis.call('hmset', key, 'tokens', tokens, 'last_refill', now)
			redis.call('expire', key, 3600)
			return 0
		end
	`

	result, err := tb.client.Eval(ctx, script, []string{key}, capacity, rate, time.Now().UnixNano()).Result()
	if err != nil {
		return false, err
	}

	return result.(int64) == 1, nil
}

// GetBucketInfo returns current bucket information for debugging
func (tb *RedisTokenBucket) GetBucketInfo(ctx context.Context, key string) (tokens float64, lastRefill time.Time, err error) {
	result, err := tb.client.HMGet(ctx, key, "tokens", "last_refill").Result()
	if err != nil {
		return 0, time.Time{}, err
	}

	if result[0] != nil {
		if t, parseErr := strconv.ParseFloat(result[0].(string), 64); parseErr == nil {
			tokens = t
		}
	}

	if result[1] != nil {
		if lr, parseErr := strconv.ParseInt(result[1].(string), 10, 64); parseErr == nil {
			lastRefill = time.Unix(0, lr)
		}
	}

	return tokens, lastRefill, nil
}

// Algorithm interface for different rate limiting strategies
type Algorithm interface {
	Allow(ctx context.Context, key string, rate float64, capacity int) (bool, error)
}

// RedisAlgorithm is a Redis-based implementation of the Algorithm interface
type RedisAlgorithm struct {
	client *redis.Client
	algType AlgorithmType
}

// NewAlgorithm creates a rate limiting algorithm based on type
func NewAlgorithm(algType AlgorithmType, client *redis.Client) Algorithm {
	return &RedisAlgorithm{
		client: client,
		algType: algType,
	}
}

// Allow implements the Algorithm interface for Redis-based rate limiting
func (r *RedisAlgorithm) Allow(ctx context.Context, key string, rate float64, capacity int) (bool, error) {
	switch r.algType {
	case TokenBucketAlgorithm:
		tb := NewTokenBucketRedis(r.client)
		return tb.Allow(ctx, key, rate, capacity)
	case SlidingWindow:
		limiter := NewRedisLimiter(r.client)
		return limiter.Allow(ctx, key, rate, capacity, time.Minute)
	default:
		tb := NewTokenBucketRedis(r.client)
		return tb.Allow(ctx, key, rate, capacity)
	}
}
