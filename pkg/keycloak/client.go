package keycloak

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/AuthMesh/authmesh/pkg/tlsutil"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

const (
	// API version for stability
	apiVersion = "26.0.0"
	// Retry configuration
	maxRetries      = 3
	baseRetryDelay  = 100 * time.Millisecond
	tokenBufferTime = 30 * time.Second
)

// Client represents a Keycloak REST API client
type Client struct {
	httpClient *http.Client
	baseURL    string
	logger     *zap.Logger
	tracer     trace.Tracer

	// Token caching
	tokenCache map[string]tokenEntry
	mu         sync.Mutex

	// OpenTelemetry metrics
	requestsTotal metric.Int64Counter
	errorsTotal   metric.Int64Counter
}

// NewClient creates a new Keycloak REST API client
func NewClient(baseURL string, logger *zap.Logger, tracer trace.Tracer) (*Client, error) {
	if baseURL == "" {
		return nil, fmt.Errorf("baseURL cannot be empty")
	}

	// Validate and normalize baseURL
	parsedURL, err := url.Parse(baseURL)
	if err != nil {
		return nil, fmt.Errorf("invalid baseURL: %w", err)
	}
	if parsedURL.Scheme == "" {
		return nil, fmt.Errorf("baseURL must include scheme (http:// or https://)")
	}

	// Remove trailing slash
	normalizedURL := strings.TrimSuffix(baseURL, "/")

	// Initialize OpenTelemetry meter
	meter := otel.Meter("keycloak-client")

	requestsTotal, err := meter.Int64Counter(
		"keycloak_requests_total",
		metric.WithDescription("Total number of requests to Keycloak API"),
	)
	if err != nil {
		logger.Warn("Failed to create requests counter", zap.Error(err))
	}

	errorsTotal, err := meter.Int64Counter(
		"keycloak_errors_total",
		metric.WithDescription("Total number of errors from Keycloak API"),
	)
	if err != nil {
		logger.Warn("Failed to create errors counter", zap.Error(err))
	}

	client := &Client{
		httpClient:    tlsutil.CreateSecureHTTPClient(),
		baseURL:       normalizedURL,
		logger:        logger,
		tracer:        tracer,
		tokenCache:    make(map[string]tokenEntry),
		requestsTotal: requestsTotal,
		errorsTotal:   errorsTotal,
	}

	return client, nil
}

// GetAdminToken retrieves an admin token for the specified client
func (c *Client) GetAdminToken(clientID, clientSecret string) (string, error) {
	ctx := context.Background()
	ctx, span := c.tracer.Start(ctx, "keycloak.GetAdminToken",
		trace.WithAttributes(
			attribute.String("client_id", clientID),
			attribute.String("operation", "get_token"),
		),
	)
	defer span.End()

	// Check cache first - use hash to avoid storing secret in memory as map key
	hash := sha256.Sum256([]byte(clientID + ":" + clientSecret))
	cacheKey := hex.EncodeToString(hash[:])
	c.mu.Lock()
	if entry, exists := c.tokenCache[cacheKey]; exists {
		// Check if token is still valid (with 30s buffer)
		if time.Now().Unix() < entry.expiresAt-int64(tokenBufferTime.Seconds()) {
			c.mu.Unlock()
			c.logger.Debug("Using cached token", zap.String("client_id", clientID))
			return entry.token, nil
		}
		// Token expired, remove from cache
		delete(c.tokenCache, cacheKey)
	}
	c.mu.Unlock()

	// Request new token
	tokenURL := fmt.Sprintf("%s/realms/master/protocol/openid-connect/token?version=%s", c.baseURL, apiVersion)

	data := url.Values{}
	data.Set("grant_type", "client_credentials")
	data.Set("client_id", clientID)
	data.Set("client_secret", clientSecret)

	req, err := http.NewRequestWithContext(ctx, "POST", tokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		c.recordError("get_token")
		span.RecordError(err)
		return "", fmt.Errorf("failed to create token request: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	var resp *http.Response
	var respBody []byte
	var lastErr error
	var exhaustedRetries bool

	// Retry logic with exponential backoff
	for attempt := 0; attempt < maxRetries; attempt++ {
		if attempt > 0 {
			retryDelay := time.Duration(attempt) * baseRetryDelay * time.Duration(1<<uint(attempt-1))
			c.logger.Debug("Retrying token request",
				zap.Int("attempt", attempt+1),
				zap.Duration("delay", retryDelay),
			)
			time.Sleep(retryDelay)
		}

		resp, lastErr = c.httpClient.Do(req)
		if lastErr != nil {
			c.logger.Warn("Token request failed",
				zap.Int("attempt", attempt+1),
				zap.Error(lastErr),
			)
			continue
		}

		respBody, lastErr = io.ReadAll(resp.Body)
		_ = resp.Body.Close()

		if lastErr != nil {
			c.logger.Warn("Failed to read token response",
				zap.Int("attempt", attempt+1),
				zap.Error(lastErr),
			)
			continue
		}

		// Check if we should retry (429 or 503)
		if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode == http.StatusServiceUnavailable {
			c.logger.Warn("Received retryable status code",
				zap.Int("attempt", attempt+1),
				zap.Int("status_code", resp.StatusCode),
			)

			// If this is the last attempt, mark as exhausted
			if attempt == maxRetries-1 {
				exhaustedRetries = true
				lastErr = fmt.Errorf("received status %d on final attempt", resp.StatusCode)
			}
			continue
		}

		// Success or non-retryable error
		break
	}

	c.recordRequest("get_token")

	// Check if we exhausted retries
	if exhaustedRetries || lastErr != nil {
		c.recordError("get_token")
		span.RecordError(lastErr)
		return "", fmt.Errorf("failed to get token after %d attempts: %w", maxRetries, lastErr)
	}

	if resp.StatusCode != http.StatusOK {
		c.recordError("get_token")
		span.SetAttributes(attribute.Int("http.status_code", resp.StatusCode))

		var errorResp ErrorResponse
		if jsonErr := json.Unmarshal(respBody, &errorResp); jsonErr == nil {
			return "", fmt.Errorf("token request failed (status %d): %s - %s",
				resp.StatusCode, errorResp.Error, errorResp.ErrorDescription)
		}
		return "", fmt.Errorf("token request failed with status %d: %s", resp.StatusCode, string(respBody))
	}

	var tokenResp TokenResponse
	if err := json.Unmarshal(respBody, &tokenResp); err != nil {
		c.recordError("get_token")
		span.RecordError(err)
		return "", fmt.Errorf("failed to parse token response: %w", err)
	}

	// Cache the token
	expiresAt := time.Now().Unix() + int64(tokenResp.ExpiresIn)
	c.mu.Lock()
	c.tokenCache[cacheKey] = tokenEntry{
		token:     tokenResp.AccessToken,
		expiresAt: expiresAt,
	}
	c.mu.Unlock()

	c.logger.Debug("Successfully obtained token",
		zap.String("client_id", clientID),
		zap.Int("expires_in", tokenResp.ExpiresIn),
	)

	return tokenResp.AccessToken, nil
}

// GetClientCredentialsToken retrieves a token using client credentials grant for administrative operations
func (c *Client) GetClientCredentialsToken(realm, clientID, clientSecret string) (string, error) {
	ctx := context.Background()
	ctx, span := c.tracer.Start(ctx, "keycloak.GetClientCredentialsToken",
		trace.WithAttributes(
			attribute.String("realm", realm),
			attribute.String("client_id", clientID),
			attribute.String("operation", "get_client_credentials_token"),
		),
	)
	defer span.End()

	// Check cache first - use hash to avoid storing secret in memory as map key
	hash := sha256.Sum256([]byte(realm + ":" + clientID + ":" + clientSecret))
	cacheKey := hex.EncodeToString(hash[:])
	c.mu.Lock()
	if entry, exists := c.tokenCache[cacheKey]; exists {
		// Check if token is still valid (with 30s buffer)
		if time.Now().Unix() < entry.expiresAt-int64(tokenBufferTime.Seconds()) {
			c.mu.Unlock()
			c.logger.Debug("Using cached client credentials token",
				zap.String("realm", realm),
				zap.String("client_id", clientID))
			return entry.token, nil
		}
		// Token expired, remove from cache
		delete(c.tokenCache, cacheKey)
	}
	c.mu.Unlock()

	// Request new token
	tokenURL := fmt.Sprintf("%s/realms/%s/protocol/openid-connect/token", c.baseURL, realm)

	data := url.Values{}
	data.Set("grant_type", "client_credentials")
	data.Set("client_id", clientID)
	data.Set("client_secret", clientSecret)

	c.logger.Debug("Making client credentials request",
		zap.String("url", tokenURL),
		zap.String("client_id", clientID),
		zap.String("realm", realm))

	req, err := http.NewRequestWithContext(ctx, "POST", tokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		c.recordError("get_client_credentials_token")
		span.RecordError(err)
		return "", fmt.Errorf("failed to create client credentials token request: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		c.recordError("get_client_credentials_token")
		span.RecordError(err)
		return "", fmt.Errorf("failed to get client credentials token: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		c.recordError("get_client_credentials_token")
		span.RecordError(err)
		return "", fmt.Errorf("failed to read client credentials token response: %w", err)
	}

	c.recordRequest("get_client_credentials_token")

	if resp.StatusCode != http.StatusOK {
		c.recordError("get_client_credentials_token")
		span.SetAttributes(attribute.Int("http.status_code", resp.StatusCode))

		c.logger.Error("Client credentials request failed",
			zap.Int("status_code", resp.StatusCode))

		var errorResp ErrorResponse
		if jsonErr := json.Unmarshal(respBody, &errorResp); jsonErr == nil {
			return "", fmt.Errorf("client credentials token request failed (status %d): %s - %s",
				resp.StatusCode, errorResp.Error, errorResp.ErrorDescription)
		}
		return "", fmt.Errorf("client credentials token request failed with status %d: %s", resp.StatusCode, string(respBody))
	}

	var tokenResp TokenResponse
	if err := json.Unmarshal(respBody, &tokenResp); err != nil {
		c.recordError("get_client_credentials_token")
		span.RecordError(err)
		return "", fmt.Errorf("failed to parse client credentials token response: %w", err)
	}

	// Cache the token
	expiresAt := time.Now().Unix() + int64(tokenResp.ExpiresIn)
	c.mu.Lock()
	c.tokenCache[cacheKey] = tokenEntry{
		token:     tokenResp.AccessToken,
		expiresAt: expiresAt,
	}
	c.mu.Unlock()

	c.logger.Debug("Successfully obtained client credentials token",
		zap.String("realm", realm),
		zap.String("client_id", clientID),
		zap.Int("expires_in", tokenResp.ExpiresIn),
	)

	return tokenResp.AccessToken, nil
}

// GetPasswordToken retrieves a token using password grant flow (for admin users)
func (c *Client) GetPasswordToken(realm, clientID, username, password string) (string, error) {
	ctx := context.Background()
	ctx, span := c.tracer.Start(ctx, "keycloak.GetPasswordToken",
		trace.WithAttributes(
			attribute.String("realm", realm),
			attribute.String("client_id", clientID),
			attribute.String("username", username),
			attribute.String("operation", "get_password_token"),
		),
	)
	defer span.End()

	// Check cache first
	cacheKey := fmt.Sprintf("%s:%s:%s", realm, clientID, username)
	c.mu.Lock()
	if entry, exists := c.tokenCache[cacheKey]; exists {
		// Check if token is still valid (with 30s buffer)
		if time.Now().Unix() < entry.expiresAt-int64(tokenBufferTime.Seconds()) {
			c.mu.Unlock()
			c.logger.Debug("Using cached password token",
				zap.String("realm", realm),
				zap.String("client_id", clientID),
				zap.String("username", username))
			return entry.token, nil
		}
		// Token expired, remove from cache
		delete(c.tokenCache, cacheKey)
	}
	c.mu.Unlock()

	// Request new token
	tokenURL := fmt.Sprintf("%s/realms/%s/protocol/openid-connect/token", c.baseURL, realm)

	data := url.Values{}
	data.Set("grant_type", "password")
	data.Set("client_id", clientID)
	data.Set("username", username)
	data.Set("password", password)

	c.logger.Debug("Requesting password token",
		zap.String("url", tokenURL),
		zap.String("realm", realm),
		zap.String("client_id", clientID),
		zap.String("username", username),
		zap.String("form_data", data.Encode()))

	req, err := http.NewRequestWithContext(ctx, "POST", tokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		c.recordError("get_password_token")
		span.RecordError(err)
		return "", fmt.Errorf("failed to create password token request: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		c.recordError("get_password_token")
		span.RecordError(err)
		return "", fmt.Errorf("password token request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		c.recordError("get_password_token")
		span.RecordError(err)
		return "", fmt.Errorf("failed to read password token response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		c.recordError("get_password_token")
		err := fmt.Errorf("password token request failed (status %d): %s", resp.StatusCode, string(respBody))
		span.RecordError(err)
		return "", err
	}

	var tokenResp struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}

	if err := json.Unmarshal(respBody, &tokenResp); err != nil {
		c.recordError("get_password_token")
		span.RecordError(err)
		return "", fmt.Errorf("failed to parse password token response: %w", err)
	}

	// Cache the token
	c.mu.Lock()
	if c.tokenCache == nil {
		c.tokenCache = make(map[string]tokenEntry)
	}
	c.tokenCache[cacheKey] = tokenEntry{
		token:     tokenResp.AccessToken,
		expiresAt: time.Now().Unix() + int64(tokenResp.ExpiresIn),
	}
	c.mu.Unlock()

	c.logger.Debug("Password token obtained successfully",
		zap.String("realm", realm),
		zap.String("client_id", clientID),
		zap.String("username", username),
		zap.Int("expires_in", tokenResp.ExpiresIn))

	return tokenResp.AccessToken, nil
}

// GetRealmAttributes retrieves attributes for a realm
func (c *Client) GetRealmAttributes(realm, token string) (map[string]string, error) {
	ctx := context.Background()
	ctx, span := c.tracer.Start(ctx, "keycloak.GetRealmAttributes",
		trace.WithAttributes(
			attribute.String("realm", realm),
			attribute.String("operation", "get_realm_attributes"),
		),
	)
	defer span.End()

	realmData, err := c.doRequest(ctx, "GET", fmt.Sprintf("/admin/realms/%s", realm), token, nil)
	if err != nil {
		return nil, err
	}

	var realmResp Realm
	if err := json.Unmarshal(realmData, &realmResp); err != nil {
		c.recordError("get_realm_attributes")
		span.RecordError(err)
		return nil, fmt.Errorf("failed to parse realm response: %w", err)
	}

	if realmResp.Attributes == nil {
		return make(map[string]string), nil
	}

	return realmResp.Attributes, nil
}

// SetRealmAttributes sets attributes for a realm
func (c *Client) SetRealmAttributes(realm, token string, attrs map[string]string) error {
	ctx := context.Background()
	ctx, span := c.tracer.Start(ctx, "keycloak.SetRealmAttributes",
		trace.WithAttributes(
			attribute.String("realm", realm),
			attribute.String("operation", "set_realm_attributes"),
		),
	)
	defer span.End()

	// First get the current realm to preserve other fields
	currentRealm, err := c.doRequest(ctx, "GET", fmt.Sprintf("/admin/realms/%s", realm), token, nil)
	if err != nil {
		return fmt.Errorf("failed to get current realm: %w", err)
	}

	var realmData Realm
	if err := json.Unmarshal(currentRealm, &realmData); err != nil {
		c.recordError("set_realm_attributes")
		span.RecordError(err)
		return fmt.Errorf("failed to parse current realm: %w", err)
	}

	// Update attributes
	if realmData.Attributes == nil {
		realmData.Attributes = make(map[string]string)
	}
	for key, value := range attrs {
		realmData.Attributes[key] = value
	}

	_, err = c.doRequest(ctx, "PUT", fmt.Sprintf("/admin/realms/%s", realm), token, realmData)
	return err
}

// UpdateRealmAttributes updates specific attributes for a realm
func (c *Client) UpdateRealmAttributes(realm string, attrs map[string]string, token string) error {
	ctx := context.Background()
	ctx, span := c.tracer.Start(ctx, "keycloak.UpdateRealmAttributes",
		trace.WithAttributes(
			attribute.String("realm", realm),
			attribute.String("operation", "update_realm_attributes"),
		),
	)
	defer span.End()

	// First get the current realm to preserve other fields
	currentRealm, err := c.doRequest(ctx, "GET", fmt.Sprintf("/admin/realms/%s", realm), token, nil)
	if err != nil {
		return fmt.Errorf("failed to get current realm: %w", err)
	}

	var realmData Realm
	if err := json.Unmarshal(currentRealm, &realmData); err != nil {
		c.recordError("update_realm_attributes")
		span.RecordError(err)
		return fmt.Errorf("failed to parse current realm data: %w", err)
	}

	// Update attributes
	if realmData.Attributes == nil {
		realmData.Attributes = make(map[string]string)
	}
	for key, value := range attrs {
		realmData.Attributes[key] = value
	}

	// Update the realm
	_, err = c.doRequest(ctx, "PUT", fmt.Sprintf("/admin/realms/%s", realm), token, realmData)
	if err != nil {
		return fmt.Errorf("failed to update realm attributes: %w", err)
	}

	c.logger.Debug("Successfully updated realm attributes",
		zap.String("realm", realm),
		zap.Any("attributes", attrs),
	)

	return nil
}

// CreateRealm creates a new realm
func (c *Client) CreateRealm(realmID, token string, config map[string]interface{}) error {
	ctx := context.Background()
	ctx, span := c.tracer.Start(ctx, "keycloak.CreateRealm",
		trace.WithAttributes(
			attribute.String("realm", realmID),
			attribute.String("operation", "create_realm"),
		),
	)
	defer span.End()

	// Ensure realm ID is set
	if config == nil {
		config = make(map[string]interface{})
	}
	config["id"] = realmID
	config["realm"] = realmID
	if _, exists := config["enabled"]; !exists {
		config["enabled"] = true
	}

	_, err := c.doRequest(ctx, "POST", "/admin/realms", token, config)
	return err
}

// AddUser adds a new user to a realm
func (c *Client) AddUser(realm, token string, user map[string]interface{}) error {
	ctx := context.Background()
	ctx, span := c.tracer.Start(ctx, "keycloak.AddUser",
		trace.WithAttributes(
			attribute.String("realm", realm),
			attribute.String("operation", "add_user"),
		),
	)
	defer span.End()

	_, err := c.doRequest(ctx, "POST", fmt.Sprintf("/admin/realms/%s/users", realm), token, user)
	return err
}

// GetUser retrieves a user by ID
func (c *Client) GetUser(realm, userID, token string) (map[string]interface{}, error) {
	ctx := context.Background()
	ctx, span := c.tracer.Start(ctx, "keycloak.GetUser",
		trace.WithAttributes(
			attribute.String("realm", realm),
			attribute.String("user_id", userID),
			attribute.String("operation", "get_user"),
		),
	)
	defer span.End()

	userData, err := c.doRequest(ctx, "GET", fmt.Sprintf("/admin/realms/%s/users/%s", realm, userID), token, nil)
	if err != nil {
		return nil, err
	}

	var result map[string]interface{}
	if err := json.Unmarshal(userData, &result); err != nil {
		c.recordError("get_user")
		span.RecordError(err)
		return nil, fmt.Errorf("failed to parse user response: %w", err)
	}

	return result, nil
}

// doRequest performs HTTP request with retry logic and telemetry
func (c *Client) doRequest(ctx context.Context, method, path, token string, body interface{}) ([]byte, error) {
	endpoint := strings.TrimPrefix(path, "/")
	operation := fmt.Sprintf("%s_%s", strings.ToLower(method), strings.ReplaceAll(endpoint, "/", "_"))

	ctx, span := c.tracer.Start(ctx, fmt.Sprintf("keycloak.%s", operation),
		trace.WithAttributes(
			attribute.String("http.method", method),
			attribute.String("http.url", path),
			attribute.String("operation", operation),
		),
	)
	defer span.End()

	var requestBody []byte
	var err error

	if body != nil {
		requestBody, err = json.Marshal(body)
		if err != nil {
			c.recordError(operation)
			span.RecordError(err)
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
	}

	sep := "?"
	if strings.Contains(path, "?") {
		sep = "&"
	}
	url := fmt.Sprintf("%s%s%sversion=%s", c.baseURL, path, sep, apiVersion)

	var resp *http.Response
	var respBody []byte

	// Retry logic with exponential backoff
	for attempt := 0; attempt < maxRetries; attempt++ {
		if attempt > 0 {
			retryDelay := time.Duration(attempt) * baseRetryDelay * time.Duration(1<<uint(attempt-1))
			c.logger.Debug("Retrying request",
				zap.String("method", method),
				zap.String("path", path),
				zap.Int("attempt", attempt+1),
				zap.Duration("delay", retryDelay),
			)
			time.Sleep(retryDelay)
		}

		var req *http.Request
		if body != nil {
			req, err = http.NewRequestWithContext(ctx, method, url, bytes.NewReader(requestBody))
		} else {
			req, err = http.NewRequestWithContext(ctx, method, url, nil)
		}

		if err != nil {
			c.logger.Warn("Failed to create request",
				zap.Int("attempt", attempt+1),
				zap.Error(err),
			)
			continue
		}

		// Set headers
		req.Header.Set("Authorization", "Bearer "+token)
		if body != nil {
			req.Header.Set("Content-Type", "application/json")
		}

		resp, err = c.httpClient.Do(req)
		if err != nil {
			c.logger.Warn("Request failed",
				zap.String("method", method),
				zap.String("path", path),
				zap.Int("attempt", attempt+1),
				zap.Error(err),
			)
			continue
		}

		respBody, err = io.ReadAll(resp.Body)
		_ = resp.Body.Close()

		if err != nil {
			c.logger.Warn("Failed to read response",
				zap.Int("attempt", attempt+1),
				zap.Error(err),
			)
			continue
		}

		// Check if we should retry (429 or 503)
		if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode == http.StatusServiceUnavailable {
			c.logger.Warn("Received retryable status code",
				zap.String("method", method),
				zap.String("path", path),
				zap.Int("attempt", attempt+1),
				zap.Int("status_code", resp.StatusCode),
			)
			continue
		}

		// Success or non-retryable error
		break
	}

	c.recordRequest(operation)

	if err != nil {
		c.recordError(operation)
		span.RecordError(err)
		return nil, fmt.Errorf("request failed after %d attempts: %w", maxRetries, err)
	}

	span.SetAttributes(attribute.Int("http.status_code", resp.StatusCode))

	// Check for success status codes
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		c.recordError(operation)

		var msg string
		var errorResp ErrorResponse
		if jsonErr := json.Unmarshal(respBody, &errorResp); jsonErr == nil && errorResp.ErrorMessage != "" {
			msg = fmt.Sprintf("API request failed (status %d): %s", resp.StatusCode, errorResp.ErrorMessage)
		} else {
			msg = fmt.Sprintf("API request failed with status %d: %s", resp.StatusCode, string(respBody))
		}
		return nil, &APIError{StatusCode: resp.StatusCode, Message: msg}
	}

	c.logger.Debug("Request successful",
		zap.String("method", method),
		zap.String("path", path),
		zap.Int("status_code", resp.StatusCode),
	)

	return respBody, nil
}

// recordRequest records a successful request metric
func (c *Client) recordRequest(endpoint string) {
	if c.requestsTotal != nil {
		c.requestsTotal.Add(context.Background(), 1,
			metric.WithAttributes(attribute.String("endpoint", endpoint)),
		)
	}
}

// recordError records an error metric
func (c *Client) recordError(endpoint string) {
	if c.errorsTotal != nil {
		c.errorsTotal.Add(context.Background(), 1,
			metric.WithAttributes(attribute.String("endpoint", endpoint)),
		)
	}
}
