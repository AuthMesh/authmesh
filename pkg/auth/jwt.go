package auth

import (
	"context"
	"crypto/tls"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/MicahParks/keyfunc"
	"github.com/golang-jwt/jwt/v4"
)

// Constants for error messages and security
const (
	ErrMissingAuth         = "Missing or invalid Authorization header"
	ErrInvalidToken        = "Invalid or expired token"
	ErrInvalidClaims       = "Invalid token claims"
	ErrNoUserContext       = "No user context found"
	ErrInsufficientRole    = "Insufficient role permissions"
	ErrInvalidClaimsFormat = "Invalid claims format"

	// Security headers
	XContentTypeOptions = "nosniff"
	XFrameOptions       = "DENY"
	XSSProtection       = "1; mode=block"
)

// JWKSRegistry holds JWKS for different realms and provides multi-tenant JWT validation
type JWKSRegistry struct {
	Mu             sync.RWMutex
	KeycloakURL    string
	DefaultRealm   string
	JwksStore      map[string]*keyfunc.JWKS
	TrustedIssuers []string // Whitelist of trusted issuer base URLs
}

// NewJWKSRegistry creates a new JWKS registry with the given configuration
func NewJWKSRegistry(keycloakURL, defaultRealm string, trustedIssuers []string) *JWKSRegistry {
	return &JWKSRegistry{
		KeycloakURL:    keycloakURL,
		DefaultRealm:   defaultRealm,
		JwksStore:      make(map[string]*keyfunc.JWKS),
		TrustedIssuers: trustedIssuers,
	}
}

// Global registry instance for backward compatibility
var Registry = &JWKSRegistry{
	JwksStore: make(map[string]*keyfunc.JWKS),
}

// JWKS is kept for backward compatibility
var JWKS *keyfunc.JWKS

// InitJWKS initializes the JWKS registry with a default realm
func InitJWKS(jwksURL string) error {
	parts := strings.Split(jwksURL, "/realms/")
	if len(parts) != 2 {
		return fmt.Errorf("invalid JWKS URL format: %s", jwksURL)
	}

	keycloakURL := parts[0]
	realmWithPath := parts[1]
	realmParts := strings.Split(realmWithPath, "/")
	if len(realmParts) < 1 {
		return fmt.Errorf("could not extract realm from JWKS URL: %s", jwksURL)
	}

	realm := realmParts[0]

	Registry.KeycloakURL = keycloakURL
	Registry.DefaultRealm = realm

	// Initialize default realm JWKS
	jwks, err := loadJWKS(jwksURL)
	if err != nil {
		return fmt.Errorf("failed to initialize JWKS from %s: %v", jwksURL, err)
	}

	Registry.Mu.Lock()
	Registry.JwksStore[realm] = jwks
	Registry.Mu.Unlock()

	// Set the global JWKS for backward compatibility
	JWKS = jwks

	log.Printf("[INFO] JWKS Registry initialized with default realm: %s", realm)
	return nil
}

// loadJWKS creates a new JWKS instance for a specific URL
func loadJWKS(jwksURL string) (*keyfunc.JWKS, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Create secure HTTP client with proper TLS validation
	client := createSecureHTTPClient()
	client.Timeout = 30 * time.Second

	jwks, err := keyfunc.Get(jwksURL, keyfunc.Options{
		Ctx:             ctx,
		RefreshInterval: 5 * time.Minute, // Refresh every 5 minutes instead of 1 hour
		RefreshTimeout:  10 * time.Second,
		Client:          client,
		RefreshErrorHandler: func(err error) {
			log.Printf("[ERROR] JWKS refresh failed: %v", err)
		},
	})

	if err != nil {
		return nil, err
	}

	log.Printf("[INFO] JWKS loaded successfully from %s", jwksURL)
	return jwks, nil
}

// createSecureHTTPClient creates a secure HTTP client with proper TLS configuration
func createSecureHTTPClient() *http.Client {
	return &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				MinVersion: tls.VersionTLS12,
				// Add other TLS security configurations
			},
		},
	}
}

// GetJWKS gets or loads JWKS for a specific realm
func (r *JWKSRegistry) GetJWKS(realm string) (*keyfunc.JWKS, error) {
	r.Mu.RLock()
	jwks, exists := r.JwksStore[realm]
	r.Mu.RUnlock()

	if exists {
		return jwks, nil
	}

	// Need to load this realm's JWKS
	jwksURL := fmt.Sprintf("%s/realms/%s/protocol/openid-connect/certs", r.KeycloakURL, realm)

	jwks, err := loadJWKS(jwksURL)
	if err != nil {
		return nil, err
	}

	r.Mu.Lock()
	r.JwksStore[realm] = jwks
	r.Mu.Unlock()

	return jwks, nil
}

// GetJWKSForIssuer gets or loads JWKS for a specific realm using the issuer base URL
func (r *JWKSRegistry) GetJWKSForIssuer(realm, issuerBaseURL string) (*keyfunc.JWKS, error) {
	// SECURITY: Validate issuer against trusted issuers whitelist to prevent SSRF
	if !r.validateTrustedIssuer(issuerBaseURL) {
		return nil, fmt.Errorf("untrusted issuer: %s", issuerBaseURL)
	}

	// Create a cache key that includes both realm and issuer base URL
	cacheKey := fmt.Sprintf("%s:%s", realm, issuerBaseURL)

	r.Mu.RLock()
	jwks, exists := r.JwksStore[cacheKey]
	r.Mu.RUnlock()

	if exists {
		return jwks, nil
	}

	// Load JWKS for this issuer
	jwksURL := fmt.Sprintf("%s/realms/%s/protocol/openid-connect/certs", issuerBaseURL, realm)

	jwks, err := loadJWKS(jwksURL)
	if err != nil {
		return nil, err
	}

	r.Mu.Lock()
	r.JwksStore[cacheKey] = jwks
	r.Mu.Unlock()

	return jwks, nil
}

// validateTrustedIssuer checks if the issuer base URL is in the trusted issuers list
func (r *JWKSRegistry) validateTrustedIssuer(issuerBaseURL string) bool {
	if len(r.TrustedIssuers) == 0 {
		// If no trusted issuers list is provided, default to allowing the configured Keycloak URL
		return strings.HasPrefix(issuerBaseURL, r.KeycloakURL)
	}

	for _, trustedIssuer := range r.TrustedIssuers {
		if strings.HasPrefix(issuerBaseURL, trustedIssuer) {
			return true
		}
	}
	return false
}

// TokenClaims represents the structure of JWT claims with multi-tenant support
type TokenClaims struct {
	jwt.RegisteredClaims
	PreferredUsername string                 `json:"preferred_username"`
	RealmAccess       map[string]interface{} `json:"realm_access"`
	ResourceAccess    map[string]interface{} `json:"resource_access"`
	TenantID          string                 `json:"tenant_id,omitempty"`
	Realm             string                 `json:"iss_realm,omitempty"`
}

// ValidateToken validates a JWT token and returns the claims
func (r *JWKSRegistry) ValidateToken(tokenString, realm string) (*TokenClaims, error) {
	jwks, err := r.GetJWKS(realm)
	if err != nil {
		return nil, fmt.Errorf("failed to get JWKS for realm %s: %v", realm, err)
	}

	// Parse and validate the token
	token, err := jwt.ParseWithClaims(tokenString, &TokenClaims{}, jwks.Keyfunc)
	if err != nil {
		return nil, fmt.Errorf("failed to parse token: %v", err)
	}

	if !token.Valid {
		return nil, fmt.Errorf("invalid token")
	}

	claims, ok := token.Claims.(*TokenClaims)
	if !ok {
		return nil, fmt.Errorf("invalid claims format")
	}

	return claims, nil
}

// ValidateTokenForIssuer validates a JWT token against a specific issuer
func (r *JWKSRegistry) ValidateTokenForIssuer(tokenString, realm, issuerBaseURL string) (*TokenClaims, error) {
	jwks, err := r.GetJWKSForIssuer(realm, issuerBaseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to get JWKS for issuer %s: %v", issuerBaseURL, err)
	}

	// Parse and validate the token
	token, err := jwt.ParseWithClaims(tokenString, &TokenClaims{}, jwks.Keyfunc)
	if err != nil {
		return nil, fmt.Errorf("failed to parse token: %v", err)
	}

	if !token.Valid {
		return nil, fmt.Errorf("invalid token")
	}

	claims, ok := token.Claims.(*TokenClaims)
	if !ok {
		return nil, fmt.Errorf("invalid claims format")
	}

	// Validate that the issuer matches what we expect
	if !strings.HasPrefix(claims.Issuer, issuerBaseURL) {
		return nil, fmt.Errorf("token issuer %s does not match expected issuer base %s", claims.Issuer, issuerBaseURL)
	}

	return claims, nil
}

// HasRole checks if the token claims contain a specific role
func (c *TokenClaims) HasRole(role string) bool {
	if c.RealmAccess == nil {
		return false
	}

	roles, ok := c.RealmAccess["roles"].([]interface{})
	if !ok {
		return false
	}

	for _, r := range roles {
		if roleStr, ok := r.(string); ok && roleStr == role {
			return true
		}
	}

	return false
}

// GetTenantID extracts the tenant ID from the token claims
func (c *TokenClaims) GetTenantID() string {
	if c.TenantID != "" {
		return c.TenantID
	}

	// Fallback: try to extract from issuer or other claim
	if c.Issuer != "" {
		parts := strings.Split(c.Issuer, "/realms/")
		if len(parts) == 2 {
			return parts[1]
		}
	}

	return ""
}
