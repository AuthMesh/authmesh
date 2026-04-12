package keycloak

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	"go.uber.org/zap"
)

func TestNewClient(t *testing.T) {
	logger := zap.NewNop()
	tracer := otel.Tracer("test")

	tests := []struct {
		name      string
		baseURL   string
		wantError bool
	}{
		{
			name:      "valid HTTPS URL",
			baseURL:   "https://localhost:8080",
			wantError: false,
		},
		{
			name:      "valid HTTP URL",
			baseURL:   "http://localhost:8080",
			wantError: false,
		},
		{
			name:      "URL with trailing slash",
			baseURL:   "https://localhost:8080/",
			wantError: false,
		},
		{
			name:      "empty URL",
			baseURL:   "",
			wantError: true,
		},
		{
			name:      "invalid URL",
			baseURL:   "not-a-url",
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := NewClient(tt.baseURL, logger, tracer)

			if tt.wantError {
				assert.Error(t, err)
				assert.Nil(t, client)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, client)
				assert.NotNil(t, client.httpClient)
				assert.NotNil(t, client.logger)
				assert.NotNil(t, client.tracer)
				assert.NotNil(t, client.tokenCache)

				// Check that trailing slash is removed
				expectedURL := tt.baseURL
				if expectedURL == "https://localhost:8080/" {
					expectedURL = "https://localhost:8080"
				}
				assert.Equal(t, expectedURL, client.baseURL)
			}
		})
	}
}

func TestClient_GetAdminToken(t *testing.T) {
	logger := zap.NewNop()
	tracer := otel.Tracer("test")

	t.Run("successful token retrieval", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "POST", r.Method)
			assert.Contains(t, r.URL.Path, "/realms/master/protocol/openid-connect/token")
			assert.Contains(t, r.URL.RawQuery, "version=26.0.0")

			response := TokenResponse{
				AccessToken: "test-token",
				ExpiresIn:   3600,
				TokenType:   "bearer",
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(response)
		}))
		defer server.Close()

		client, err := NewClient(server.URL, logger, tracer)
		require.NoError(t, err)

		token, err := client.GetAdminToken("test-client", "test-secret")
		assert.NoError(t, err)
		assert.Equal(t, "test-token", token)
	})

	t.Run("token caching", func(t *testing.T) {
		callCount := 0
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			callCount++
			response := TokenResponse{
				AccessToken: "cached-token",
				ExpiresIn:   3600,
				TokenType:   "bearer",
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(response)
		}))
		defer server.Close()

		client, err := NewClient(server.URL, logger, tracer)
		require.NoError(t, err)

		// First call should hit the server
		token1, err := client.GetAdminToken("test-client", "test-secret")
		assert.NoError(t, err)
		assert.Equal(t, "cached-token", token1)
		assert.Equal(t, 1, callCount)

		// Second call should use cache
		token2, err := client.GetAdminToken("test-client", "test-secret")
		assert.NoError(t, err)
		assert.Equal(t, "cached-token", token2)
		assert.Equal(t, 1, callCount) // Should not increment
	})

	t.Run("token expiration", func(t *testing.T) {
		callCount := 0
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			callCount++
			response := TokenResponse{
				AccessToken: "short-lived-token",
				ExpiresIn:   1, // 1 second expiry
				TokenType:   "bearer",
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(response)
		}))
		defer server.Close()

		client, err := NewClient(server.URL, logger, tracer)
		require.NoError(t, err)

		// First call
		token1, err := client.GetAdminToken("test-client", "test-secret")
		assert.NoError(t, err)
		assert.Equal(t, "short-lived-token", token1)
		assert.Equal(t, 1, callCount)

		// Wait for token to expire (1 second + 30 second buffer is already passed due to immediate check)
		time.Sleep(100 * time.Millisecond)

		// Second call should get a new token due to 30-second buffer
		token2, err := client.GetAdminToken("test-client", "test-secret")
		assert.NoError(t, err)
		assert.Equal(t, "short-lived-token", token2)
		assert.Equal(t, 2, callCount) // Should increment
	})

	t.Run("authentication error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte(`{"error": "invalid_client"}`))
		}))
		defer server.Close()

		client, err := NewClient(server.URL, logger, tracer)
		require.NoError(t, err)

		token, err := client.GetAdminToken("invalid-client", "wrong-secret")
		assert.Error(t, err)
		assert.Empty(t, token)
		assert.Contains(t, err.Error(), "401")
	})
}

func TestClient_GetRealmAttributes(t *testing.T) {
	logger := zap.NewNop()
	tracer := otel.Tracer("test")

	t.Run("successful retrieval", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "GET", r.Method)
			assert.Contains(t, r.URL.Path, "/admin/realms/test-realm")
			assert.Contains(t, r.URL.RawQuery, "version=26.0.0")
			assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))

			realm := Realm{
				ID:    "test-realm",
				Realm: "test-realm",
				Attributes: map[string]string{
					"max_rate_limit_requests_per_hour": "2000",
					"custom_attribute":                 "value",
				},
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(realm)
		}))
		defer server.Close()

		client, err := NewClient(server.URL, logger, tracer)
		require.NoError(t, err)

		attrs, err := client.GetRealmAttributes("test-realm", "test-token")
		assert.NoError(t, err)
		assert.Equal(t, "2000", attrs["max_rate_limit_requests_per_hour"])
		assert.Equal(t, "value", attrs["custom_attribute"])
	})

	t.Run("realm not found", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte(`{"errorMessage": "Realm not found"}`))
		}))
		defer server.Close()

		client, err := NewClient(server.URL, logger, tracer)
		require.NoError(t, err)

		attrs, err := client.GetRealmAttributes("nonexistent-realm", "test-token")
		assert.Error(t, err)
		assert.Nil(t, attrs)
		assert.Contains(t, err.Error(), "404")
	})
}

func TestClient_SetRealmAttributes(t *testing.T) {
	logger := zap.NewNop()
	tracer := otel.Tracer("test")

	t.Run("successful update", func(t *testing.T) {
		var receivedRealm Realm
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == "GET" {
				// Return current realm
				realm := Realm{
					ID:    "test-realm",
					Realm: "test-realm",
					Attributes: map[string]string{
						"existing_attr": "existing_value",
					},
				}
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(realm)
			} else if r.Method == "PUT" {
				// Capture the updated realm
				assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))
				assert.Contains(t, r.URL.RawQuery, "version=26.0.0")

				err := json.NewDecoder(r.Body).Decode(&receivedRealm)
				assert.NoError(t, err)
				w.WriteHeader(http.StatusOK)
			}
		}))
		defer server.Close()

		client, err := NewClient(server.URL, logger, tracer)
		require.NoError(t, err)

		attrs := map[string]string{
			"max_rate_limit_requests_per_hour": "3000",
			"new_attribute":                    "new_value",
		}

		err = client.SetRealmAttributes("test-realm", "test-token", attrs)
		assert.NoError(t, err)

		// Verify the attributes were merged correctly
		assert.Equal(t, "existing_value", receivedRealm.Attributes["existing_attr"])
		assert.Equal(t, "3000", receivedRealm.Attributes["max_rate_limit_requests_per_hour"])
		assert.Equal(t, "new_value", receivedRealm.Attributes["new_attribute"])
	})
}

func TestClient_RetryLogic(t *testing.T) {
	logger := zap.NewNop()
	tracer := otel.Tracer("test")

	t.Run("retry on 429", func(t *testing.T) {
		callCount := 0
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			callCount++
			if callCount < 3 {
				w.WriteHeader(http.StatusTooManyRequests)
				return
			}
			// Success on third try
			response := TokenResponse{
				AccessToken: "retry-success-token",
				ExpiresIn:   3600,
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(response)
		}))
		defer server.Close()

		client, err := NewClient(server.URL, logger, tracer)
		require.NoError(t, err)

		token, err := client.GetAdminToken("test-client", "test-secret")
		assert.NoError(t, err)
		assert.Equal(t, "retry-success-token", token)
		assert.Equal(t, 3, callCount) // Should have retried twice
	})

	t.Run("retry on 503", func(t *testing.T) {
		callCount := 0
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			callCount++
			if callCount < 2 {
				w.WriteHeader(http.StatusServiceUnavailable)
				return
			}
			// Success on second try
			response := TokenResponse{
				AccessToken: "retry-503-token",
				ExpiresIn:   3600,
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(response)
		}))
		defer server.Close()

		client, err := NewClient(server.URL, logger, tracer)
		require.NoError(t, err)

		token, err := client.GetAdminToken("test-client", "test-secret")
		assert.NoError(t, err)
		assert.Equal(t, "retry-503-token", token)
		assert.Equal(t, 2, callCount) // Should have retried once
	})

	t.Run("exhaust retries", func(t *testing.T) {
		callCount := 0
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			callCount++
			w.WriteHeader(http.StatusTooManyRequests)
		}))
		defer server.Close()

		client, err := NewClient(server.URL, logger, tracer)
		require.NoError(t, err)

		token, err := client.GetAdminToken("test-client", "test-secret")
		assert.Error(t, err)
		assert.Empty(t, token)
		assert.Contains(t, err.Error(), "failed to get token after 3 attempts")
		assert.Equal(t, 3, callCount) // Should have tried exactly 3 times
	})
}

func TestClient_ThreadSafety(t *testing.T) {
	logger := zap.NewNop()
	tracer := otel.Tracer("test")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := TokenResponse{
			AccessToken: "thread-safe-token",
			ExpiresIn:   3600,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	client, err := NewClient(server.URL, logger, tracer)
	require.NoError(t, err)

	// Test concurrent access to token cache
	const numGoroutines = 10
	results := make(chan string, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func() {
			token, err := client.GetAdminToken("test-client", "test-secret")
			if err != nil {
				results <- ""
			} else {
				results <- token
			}
		}()
	}

	// Collect results
	for i := 0; i < numGoroutines; i++ {
		token := <-results
		assert.Equal(t, "thread-safe-token", token)
	}
}

func TestClient_ErrorHandling(t *testing.T) {
	logger := zap.NewNop()
	tracer := otel.Tracer("test")

	t.Run("invalid JSON response", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte("invalid json"))
		}))
		defer server.Close()

		client, err := NewClient(server.URL, logger, tracer)
		require.NoError(t, err)

		token, err := client.GetAdminToken("test-client", "test-secret")
		assert.Error(t, err)
		assert.Empty(t, token)
		assert.Contains(t, err.Error(), "parse")
	})

	t.Run("network error", func(t *testing.T) {
		// Use an invalid port to trigger network error
		client, err := NewClient("http://localhost:99999", logger, tracer)
		require.NoError(t, err)

		token, err := client.GetAdminToken("test-client", "test-secret")
		assert.Error(t, err)
		assert.Empty(t, token)
		assert.Contains(t, err.Error(), "failed to get token after 3 attempts")
	})
}
