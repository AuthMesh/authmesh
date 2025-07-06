package platform

import (
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()
	
	if config.Keycloak.URL == "" {
		t.Error("Keycloak URL should not be empty")
	}
	
	if config.Keycloak.Realm == "" {
		t.Error("Keycloak realm should not be empty")
	}
	
	if config.Redis.URL == "" {
		t.Error("Redis URL should not be empty")
	}
	
	if !config.Security.EnableSecurityHeaders {
		t.Error("Security headers should be enabled by default")
	}
	
	if !config.RateLimit.Enabled {
		t.Error("Rate limiting should be enabled by default")
	}
	
	if !config.Observability.EnableMetrics {
		t.Error("Metrics should be enabled by default")
	}
}

func TestNewPlatform(t *testing.T) {
	config := DefaultConfig()
	
	// Test with invalid Keycloak URL (should fail gracefully)
	config.Keycloak.URL = "invalid-url"
	
	_, err := New(config)
	if err == nil {
		t.Error("Expected error for invalid Keycloak URL")
	}
}

func TestPlatformConfiguration(t *testing.T) {
	config := DefaultConfig()
	config.Keycloak.URL = "http://localhost:9443"
	config.Keycloak.Realm = "test"
	
	platform, err := New(config)
	if err != nil {
		// Skip if we can't connect to Keycloak (not running)
		t.Skipf("Skipping test due to Keycloak connection error: %v", err)
	}
	
	if platform == nil {
		t.Fatal("Platform should not be nil")
	}
	
	retrievedConfig := platform.GetConfig()
	if retrievedConfig.Keycloak.Realm != "test" {
		t.Error("Configuration should be preserved")
	}
}
