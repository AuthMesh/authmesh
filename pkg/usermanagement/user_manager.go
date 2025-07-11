package usermanagement

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"

	"github.com/AuthMesh/authmesh/pkg/keycloak"
)

// Error definitions for rate limiting operations
var (
	ErrRateLimitExceedsMax = errors.New("rate limit exceeds maximum allowed for tenant")
	ErrGroupNotFound       = errors.New("group not found")
	ErrTenantNotFound      = errors.New("tenant not found")
)

// UserManager handles user management operations and rate limit configuration syncing
type UserManager struct {
	keycloakClient *keycloak.Client
	redisClient    *redis.Client
	rateLimitCache *RateLimitCache
	logger         *zap.Logger
	tracer         trace.Tracer
	syncInterval   time.Duration
	syncRunning    bool
	syncMutex      sync.Mutex

	// OpenTelemetry metrics
	syncTotalCounter  metric.Int64Counter
	syncErrorsCounter metric.Int64Counter
}

// UserManagerConfig contains configuration for the UserManager
type UserManagerConfig struct {
	KeycloakURL  string
	RedisClient  *redis.Client
	Logger       *zap.Logger
	SyncInterval time.Duration
}

// NewUserManager creates a new UserManager instance
func NewUserManager(config UserManagerConfig) (*UserManager, error) {
	// Initialize OpenTelemetry
	tracer := otel.Tracer("user-manager")
	meter := otel.Meter("user-manager")

	// Create metrics
	syncTotalCounter, err := meter.Int64Counter(
		"rate_limit_sync_total",
		metric.WithDescription("Total number of rate limit sync operations"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create sync total counter: %w", err)
	}

	syncErrorsCounter, err := meter.Int64Counter(
		"rate_limit_sync_errors_total",
		metric.WithDescription("Total number of rate limit sync errors"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create sync errors counter: %w", err)
	}

	// Initialize Keycloak client
	keycloakClient, err := keycloak.NewClient(config.KeycloakURL, config.Logger, tracer)
	if err != nil {
		return nil, fmt.Errorf("failed to create Keycloak client: %w", err)
	}

	// Set default sync interval if not provided
	syncInterval := config.SyncInterval
	if syncInterval == 0 {
		syncInterval = 3600 * time.Second // Default 1 hour
	}

	return &UserManager{
		keycloakClient:    keycloakClient,
		redisClient:       config.RedisClient,
		rateLimitCache:    NewRateLimitCache(),
		logger:            config.Logger,
		tracer:            tracer,
		syncInterval:      syncInterval,
		syncTotalCounter:  syncTotalCounter,
		syncErrorsCounter: syncErrorsCounter,
	}, nil
}

// GetAdminTokenForTenant retrieves an admin token for the specified tenant's management client
func (um *UserManager) GetAdminTokenForTenant(tenantID string) (string, error) {
	_, span := um.tracer.Start(context.Background(), "get_admin_token_for_tenant")
	defer span.End()

	span.SetAttributes(
		attribute.String("tenant_id", tenantID),
		attribute.String("client_id", "management-client"),
	)

	// Get tenant-specific admin secret from environment
	// Convert tenant ID to valid env var name (replace hyphens with underscores)
	envVarName := strings.ToUpper(strings.ReplaceAll(tenantID, "-", "_"))
	envVar := fmt.Sprintf("KEYCLOAK_%s_ADMIN_SECRET", envVarName)
	clientSecret := os.Getenv(envVar)

	// Debug logging to see what's happening
	um.logger.Debug("Looking up admin secret for tenant",
		zap.String("tenant_id", tenantID),
		zap.String("env_var", envVar),
		zap.Bool("secret_found", clientSecret != ""))

	if clientSecret == "" {
		err := fmt.Errorf("admin secret not found for tenant %s (env var: %s)", tenantID, envVar)
		span.RecordError(err)
		um.logger.Error("Admin secret not configured for tenant",
			zap.String("tenant_id", tenantID),
			zap.String("env_var", envVar))
		return "", err
	}

	// Get admin token using the tenant's management client
	// This authenticates against the tenant realm, not master realm
	token, err := um.keycloakClient.GetClientCredentialsToken(tenantID, "management-client", clientSecret)
	if err != nil {
		span.RecordError(err)
		um.logger.Error("Failed to get admin token for tenant",
			zap.String("tenant_id", tenantID),
			zap.Error(err))
		return "", fmt.Errorf("failed to get admin token for tenant %s: %w", tenantID, err)
	}

	um.logger.Debug("Successfully retrieved admin token for tenant",
		zap.String("tenant_id", tenantID))
	return token, nil
}

// LoadRateLimitConfigs syncs rate limit configurations from Keycloak to Redis and cache
func (um *UserManager) LoadRateLimitConfigs() error {
	ctx, span := um.tracer.Start(context.Background(), "load_rate_limit_configs")
	defer span.End()

	um.logger.Info("Starting rate limit configuration sync")

	// Increment sync counter
	um.syncTotalCounter.Add(ctx, 1)

	// Sync configurations for known tenants (excluding administrative realms)
	tenants := []string{"tenant1", "tenant2"} // TODO: Make this configurable

	for _, tenantID := range tenants {
		// Get tenant-specific admin token
		token, err := um.GetAdminTokenForTenant(tenantID)
		if err != nil {
			um.logger.Error("Failed to get admin token for tenant",
				zap.String("tenant_id", tenantID),
				zap.Error(err))
			um.syncErrorsCounter.Add(ctx, 1)
			// Continue with other tenants even if one fails
			continue
		}

		if err := um.syncTenantConfigs(ctx, tenantID, token); err != nil {
			um.logger.Error("Failed to sync tenant configs",
				zap.String("tenant_id", tenantID),
				zap.Error(err))
			um.syncErrorsCounter.Add(ctx, 1)
			// Continue with other tenants even if one fails
		}
	}

	um.logger.Info("Rate limit configuration sync completed")
	return nil
}

// syncTenantConfigs syncs rate limit configurations for a specific tenant
func (um *UserManager) syncTenantConfigs(ctx context.Context, tenantID, token string) error {
	span := trace.SpanFromContext(ctx)
	span.SetAttributes(
		attribute.String("tenant_id", tenantID),
		attribute.String("operation", "sync_tenant_configs"),
	)

	// Get realm attributes for per-tenant rate limit
	realmAttrs, err := um.keycloakClient.GetRealmAttributes(tenantID, token)
	if err != nil {
		return fmt.Errorf("failed to get realm attributes for %s: %w", tenantID, err)
	}

	// Parse and cache per-tenant rate limit (requests per second)
	if rateLimitStr, exists := realmAttrs["max_rate_limit_requests_per_second"]; exists {
		rateLimit, err := strconv.ParseFloat(rateLimitStr, 64)
		if err != nil {
			um.logger.Warn("Invalid tenant rate limit value",
				zap.String("tenant_id", tenantID),
				zap.String("value", rateLimitStr))
			rateLimit = 5000.0 // Default tenant rate limit per second
		}

		// Store in Redis
		redisKey := fmt.Sprintf("ratelimit_tenant:golang-mt:%s", tenantID)
		err = um.redisClient.SetEx(ctx, redisKey, rateLimit, 7*24*time.Hour).Err() // 7 days
		if err != nil {
			um.logger.Error("Failed to store tenant rate limit in Redis",
				zap.String("tenant_id", tenantID),
				zap.Error(err))
		}

		// Update in-memory cache
		um.rateLimitCache.UpdateTenantRateLimitCache(tenantID, rateLimit)

		um.logger.Debug("Updated tenant rate limit",
			zap.String("tenant_id", tenantID),
			zap.Float64("rate_limit_per_second", rateLimit))
	}

	return nil
}

// StartSyncLoop starts the periodic rate limit configuration sync
func (um *UserManager) StartSyncLoop(ctx context.Context) {
	um.syncMutex.Lock()
	if um.syncRunning {
		um.syncMutex.Unlock()
		return
	}
	um.syncRunning = true
	um.syncMutex.Unlock()

	um.logger.Info("Starting rate limit sync loop",
		zap.Duration("interval", um.syncInterval))

	// Initial sync
	if err := um.LoadRateLimitConfigs(); err != nil {
		um.logger.Error("Initial rate limit sync failed", zap.Error(err))
	}

	// Start periodic sync
	ticker := time.NewTicker(um.syncInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			um.logger.Info("Rate limit sync loop stopped")
			um.syncMutex.Lock()
			um.syncRunning = false
			um.syncMutex.Unlock()
			return
		case <-ticker.C:
			if err := um.LoadRateLimitConfigs(); err != nil {
				um.logger.Error("Periodic rate limit sync failed", zap.Error(err))
			}
		}
	}
}

// StopSyncLoop stops the periodic sync (for testing/cleanup)
func (um *UserManager) StopSyncLoop() {
	um.syncMutex.Lock()
	defer um.syncMutex.Unlock()
	um.syncRunning = false
}

// GetRateLimitCache returns the rate limit cache for direct access (mainly for testing)
func (um *UserManager) GetRateLimitCache() *RateLimitCache {
	return um.rateLimitCache
}

// GetKeycloakClient returns the Keycloak client for direct access (mainly for superadmin operations)
func (um *UserManager) GetKeycloakClient() *keycloak.Client {
	return um.keycloakClient
}

// GetMasterAdminToken retrieves a token for administrative operations using master realm admin
func (um *UserManager) GetMasterAdminToken() (string, error) {
	ctx, span := um.tracer.Start(context.Background(), "get_master_admin_token")
	defer span.End()

	_ = ctx // Use context variable

	span.SetAttributes(
		attribute.String("realm", "master"),
		attribute.String("client_id", "admin-cli"),
	)

	adminUsername := os.Getenv("KEYCLOAK_ADMIN")
	adminPassword := os.Getenv("KEYCLOAK_ADMIN_PASSWORD")
	
	if adminUsername == "" || adminPassword == "" {
		err := fmt.Errorf("KEYCLOAK_ADMIN or KEYCLOAK_ADMIN_PASSWORD not set")
		span.RecordError(err)
		return "", err
	}

	token, err := um.keycloakClient.GetPasswordToken("master", "admin-cli", adminUsername, adminPassword)
	if err != nil {
		span.RecordError(err)
		um.logger.Error("Failed to get master realm admin token", zap.Error(err))
		return "", fmt.Errorf("failed to get master realm admin token: %w", err)
	}

	um.logger.Debug("Successfully retrieved master realm admin token")
	return token, nil
}

// CreateRealm creates a new realm with the specified configuration
func (um *UserManager) CreateRealm(realmID string, config keycloak.RealmConfig) error {
	ctx, span := um.tracer.Start(context.Background(), "create_realm")
	defer span.End()

	_ = ctx // Use context variable

	span.SetAttributes(
		attribute.String("realm_id", realmID),
		attribute.String("operation", "create_realm"),
	)

	// Get master admin token
	token, err := um.GetMasterAdminToken()
	if err != nil {
		span.RecordError(err)
		return fmt.Errorf("failed to get master admin token: %w", err)
	}

	// Convert RealmConfig to map[string]interface{} for Keycloak client
	configMap := map[string]interface{}{
		"displayName": config.DisplayName,
		"enabled":     config.Enabled,
	}
	if config.Attributes != nil {
		configMap["attributes"] = config.Attributes
	}

	// Create the realm using Keycloak client
	err = um.keycloakClient.CreateRealm(realmID, token, configMap)
	if err != nil {
		span.RecordError(err)
		um.logger.Error("Failed to create realm",
			zap.String("realm_id", realmID),
			zap.Error(err))
		return fmt.Errorf("failed to create realm %s: %w", realmID, err)
	}

	um.logger.Info("Successfully created realm",
		zap.String("realm_id", realmID))

	return nil
}

// AddUser adds a new user to the specified realm
func (um *UserManager) AddUser(realm, username, email string) error {
	_, span := um.tracer.Start(context.Background(), "add_user")
	defer span.End()

	span.SetAttributes(
		attribute.String("realm", realm),
		attribute.String("username", username),
		attribute.String("operation", "add_user"),
	)

	// Get admin token for this tenant (realm)
	token, err := um.GetAdminTokenForTenant(realm)
	if err != nil {
		span.RecordError(err)
		return fmt.Errorf("failed to get admin token for realm %s: %w", realm, err)
	}

	// Create user data
	userData := map[string]interface{}{
		"username": username,
		"email":    email,
		"enabled":  true,
	}

	// Add user via Keycloak
	err = um.keycloakClient.AddUser(realm, token, userData)
	if err != nil {
		span.RecordError(err)
		um.logger.Error("Failed to add user",
			zap.String("realm", realm),
			zap.String("username", username),
			zap.Error(err))
		return fmt.Errorf("failed to add user to realm %s: %w", realm, err)
	}

	um.logger.Info("Successfully added user",
		zap.String("realm", realm),
		zap.String("username", username),
		zap.String("email", email))

	return nil
}

// GetUser retrieves user information from the specified realm
func (um *UserManager) GetUser(realm, userID string) (map[string]interface{}, error) {
	_, span := um.tracer.Start(context.Background(), "get_user")
	defer span.End()

	span.SetAttributes(
		attribute.String("realm", realm),
		attribute.String("user_id", userID),
		attribute.String("operation", "get_user"),
	)

	// Get admin token for this tenant (realm)
	token, err := um.GetAdminTokenForTenant(realm)
	if err != nil {
		span.RecordError(err)
		return nil, fmt.Errorf("failed to get admin token for realm %s: %w", realm, err)
	}

	// Get user from Keycloak
	user, err := um.keycloakClient.GetUser(realm, userID, token)
	if err != nil {
		span.RecordError(err)
		um.logger.Error("Failed to get user",
			zap.String("realm", realm),
			zap.String("user_id", userID),
			zap.Error(err))
		return nil, fmt.Errorf("failed to get user from realm %s: %w", realm, err)
	}

	return user, nil
}

// GetTenantRateLimit gets the rate limit for a tenant from cache
func (um *UserManager) GetTenantRateLimit(tenantID string) (float64, error) {
	if limit, exists := um.rateLimitCache.GetTenantRateLimitCached(tenantID); exists {
		return limit, nil
	}
	// Return default if not configured
	return 1000.0, nil // Default fallback
}

// Helper functions to get default values from environment or use fallbacks
func getDefaultRateLimit() int {
	if val := os.Getenv("RATE_LIMIT_DEFAULT"); val != "" {
		if limit, err := strconv.Atoi(val); err == nil {
			return limit
		}
	}
	return 1000 // Default fallback
}

func getDefaultMaxRateLimit() int {
	if val := os.Getenv("RATE_LIMIT_MAX_DEFAULT"); val != "" {
		if limit, err := strconv.Atoi(val); err == nil {
			return limit
		}
	}
	return 2000 // Default fallback
}
