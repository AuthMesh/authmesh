package auth

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestParseRefreshTokenTTL(t *testing.T) {
	tests := []struct {
		name     string
		envVal   string
		expected time.Duration
	}{
		{"default value", "", 7 * 24 * time.Hour},
		{"custom hours", "48h", 48 * time.Hour},
		{"invalid falls back to default", "invalid", 7 * 24 * time.Hour},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.envVal != "" {
				t.Setenv("REFRESH_TOKEN_TTL", tt.envVal)
			}
			result := parseRefreshTokenTTL()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestIsRedisReady(t *testing.T) {
	// Without Redis configured, should return false
	origClient := RedisClient
	defer func() { RedisClient = origClient }()

	RedisClient = nil
	assert.False(t, IsRedisReady())
}

func TestGetRedisStatus(t *testing.T) {
	origClient := RedisClient
	defer func() { RedisClient = origClient }()

	RedisClient = nil
	status := GetRedisStatus()
	assert.Equal(t, "not_initialized", status["status"])
}

func TestIsTokenRevoked_NoRedis(t *testing.T) {
	origClient := RedisClient
	defer func() { RedisClient = origClient }()

	RedisClient = nil
	// Without Redis, tokens cannot be checked - should return false (allow)
	assert.False(t, IsTokenRevoked("tenant", "user", "jti"))
}

func TestInitRedisMetrics(t *testing.T) {
	// Should not panic
	initRedisMetrics()
}
