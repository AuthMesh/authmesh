package keycloak

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAPIError(t *testing.T) {
	err := &APIError{
		StatusCode: 401,
		Message:    "unauthorized",
	}
	assert.Equal(t, "unauthorized", err.Error())
	assert.Equal(t, 401, err.StatusCode)
}

func TestErrorResponse(t *testing.T) {
	resp := ErrorResponse{
		Error:            "invalid_grant",
		ErrorDescription: "Invalid user credentials",
	}
	assert.Equal(t, "invalid_grant", resp.Error)
	assert.Equal(t, "Invalid user credentials", resp.ErrorDescription)
}

func TestTokenEntry(t *testing.T) {
	entry := tokenEntry{
		token:     "test-token",
		expiresAt: 1234567890,
	}
	assert.Equal(t, "test-token", entry.token)
	assert.Equal(t, int64(1234567890), entry.expiresAt)
}

func TestRealmConfig(t *testing.T) {
	cfg := RealmConfig{
		DisplayName: "Test Realm",
		Enabled:     true,
		Attributes:  map[string]string{"key": "val"},
	}
	assert.Equal(t, "Test Realm", cfg.DisplayName)
	assert.True(t, cfg.Enabled)
}

func TestTokenResponse(t *testing.T) {
	resp := TokenResponse{
		AccessToken: "abc123",
		ExpiresIn:   300,
		TokenType:   "Bearer",
	}
	assert.Equal(t, "abc123", resp.AccessToken)
	assert.Equal(t, 300, resp.ExpiresIn)
}
