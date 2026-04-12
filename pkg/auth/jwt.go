package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/AuthMesh/authmesh/pkg/tlsutil"
	"github.com/MicahParks/keyfunc"
	"github.com/gin-gonic/gin"
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

// JWKSRegistry holds JWKS for different realms
type JWKSRegistry struct {
	Mu             sync.RWMutex
	KeycloakURL    string
	DefaultRealm   string
	JwksStore      map[string]*keyfunc.JWKS
	TrustedIssuers []string // Whitelist of trusted issuer base URLs
}

var Registry = &JWKSRegistry{
	JwksStore: make(map[string]*keyfunc.JWKS),
}

// JWKS is kept for backward compatibility
var JWKS *keyfunc.JWKS

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

// loadJWKS creates a new JWKS instance for a specific URL with retry logic
func loadJWKS(jwksURL string) (*keyfunc.JWKS, error) {
	// Create secure HTTP client with proper TLS validation
	client := tlsutil.CreateSecureHTTPClient()
	client.Timeout = 10 * time.Second

	// Retry configuration
	maxRetries := 10
	baseDelay := 2 * time.Second
	maxDelay := 30 * time.Second

	var lastErr error
	for attempt := 0; attempt < maxRetries; attempt++ {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)

		log.Printf("[INFO] Attempting to load JWKS from %s (attempt %d/%d)", jwksURL, attempt+1, maxRetries)

		jwks, err := keyfunc.Get(jwksURL, keyfunc.Options{
			Ctx:             ctx,
			RefreshInterval: 5 * time.Minute, // Refresh every 5 minutes instead of 1 hour
			RefreshTimeout:  10 * time.Second,
			Client:          client,
			RefreshErrorHandler: func(err error) {
				log.Printf("[ERROR] JWKS refresh failed: %v", err)
			},
		})

		cancel()

		if err == nil {
			log.Printf("[INFO] JWKS loaded successfully from %s after %d attempts", jwksURL, attempt+1)
			return jwks, nil
		}

		lastErr = err

		// If this is the last attempt, don't wait
		if attempt == maxRetries-1 {
			break
		}

		// Calculate delay with exponential backoff and jitter
		backoffMultiplier := 1 << uint(attempt) // 2^attempt
		delay := time.Duration(float64(baseDelay) * float64(backoffMultiplier))
		if delay > maxDelay {
			delay = maxDelay
		}

		log.Printf("[WARN] JWKS load failed (attempt %d/%d): %v. Retrying in %v...", attempt+1, maxRetries, err, delay)
		time.Sleep(delay)
	}

	return nil, fmt.Errorf("failed to load JWKS after %d attempts: %w", maxRetries, lastErr)
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

	// Map external issuer URLs to internal URLs for Docker environments
	jwksBaseURL := issuerBaseURL
	if issuerBaseURL == "https://localhost:9443" || issuerBaseURL == "https://localhost:9444" {
		// Map localhost URLs to the internal keycloak service URL
		jwksBaseURL = "https://keycloak:9443"
	}

	// Need to load this realm's JWKS using the mapped base URL
	jwksURL := fmt.Sprintf("%s/realms/%s/protocol/openid-connect/certs", jwksBaseURL, realm)

	jwks, err := loadJWKS(jwksURL)
	if err != nil {
		return nil, err
	}

	r.Mu.Lock()
	r.JwksStore[cacheKey] = jwks
	r.Mu.Unlock()

	return jwks, nil
}

// SecurityMiddleware adds security headers to all responses
func SecurityMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("X-Content-Type-Options", XContentTypeOptions)
		c.Header("X-Frame-Options", XFrameOptions)
		c.Header("X-XSS-Protection", XSSProtection)
		c.Header("Referrer-Policy", "strict-origin-when-cross-origin")
		c.Next()
	}
}

// sanitizeLogValue removes newlines and control characters from user input to prevent log injection
func sanitizeLogValue(s string) string {
	return strings.NewReplacer(
		"\n", "",
		"\r", "",
		"\t", " ",
	).Replace(s)
}

// logAuthEvent logs authentication events with structured information
func logAuthEvent(level, event string, c *gin.Context, details map[string]interface{}) {
	// Sanitize user-controlled inputs to prevent log injection
	sanitizedIP := sanitizeLogValue(c.ClientIP())
	sanitizedUA := sanitizeLogValue(c.GetHeader("User-Agent"))
	sanitizedPath := sanitizeLogValue(c.Request.URL.Path)

	logEntry := fmt.Sprintf("[%s] %s - IP: %s, UserAgent: %s, Path: %s",
		level, event, sanitizedIP, sanitizedUA, sanitizedPath)

	for k, v := range details {
		logEntry += fmt.Sprintf(", %s: %v", sanitizeLogValue(k), v)
	}

	log.Println(logEntry)
}

func RequireAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Extract and validate Authorization header
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
			logAuthEvent("WARN", "Authentication failed - missing/invalid header", c, nil)
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": ErrMissingAuth,
				"code":  "AUTH_HEADER_MISSING",
			})
			return
		}

		tokenStr := strings.TrimPrefix(authHeader, "Bearer ")
		if len(tokenStr) == 0 {
			logAuthEvent("WARN", "Authentication failed - empty token", c, nil)
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": ErrMissingAuth,
				"code":  "TOKEN_EMPTY",
			})
			return
		}

		// Parse the token without verification first to extract the issuer/realm
		parser := &jwt.Parser{
			SkipClaimsValidation: true,
		}
		unverifiedToken, _, err := parser.ParseUnverified(tokenStr, jwt.MapClaims{})
		if err != nil {
			logAuthEvent("WARN", "Authentication failed - token parse error", c, map[string]interface{}{
				"error": err.Error(),
			})
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": ErrInvalidToken,
				"code":  "TOKEN_PARSE_ERROR",
			})
			return
		}

		// Extract claims to get the issuer
		claims, ok := unverifiedToken.Claims.(jwt.MapClaims)
		if !ok {
			logAuthEvent("ERROR", "Authentication failed - invalid claims format", c, nil)
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": ErrInvalidClaims,
				"code":  "CLAIMS_FORMAT_ERROR",
			})
			return
		}

		// Extract realm from issuer
		realm := Registry.DefaultRealm
		var issuerBaseURL string
		if iss, ok := claims["iss"].(string); ok {
			parts := strings.Split(iss, "/realms/")
			if len(parts) == 2 {
				issuerBaseURL = parts[0]
				realmParts := strings.Split(parts[1], "/")
				if len(realmParts) > 0 {
					realm = realmParts[0]
				}
			}
		}

		// Get the appropriate JWKS for this realm
		// Use the issuer base URL if available, otherwise fall back to configured URL
		var jwksErr error

		if issuerBaseURL != "" {
			_, jwksErr = Registry.GetJWKSForIssuer(realm, issuerBaseURL)
		} else {
			_, jwksErr = Registry.GetJWKS(realm)
		}
		if jwksErr != nil {
			logAuthEvent("ERROR", "Authentication failed - JWKS not available for realm", c, map[string]interface{}{
				"realm": realm,
				"error": jwksErr.Error(),
			})
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "Invalid token issuer",
				"code":  "UNKNOWN_REALM",
			})
			return
		}

		// Now fully verify the token with revocation checking
		verifiedClaims, err := ValidateJWTWithRevocationCheck(tokenStr)
		if err != nil {
			logAuthEvent("WARN", "Authentication failed - token verification error", c, map[string]interface{}{
				"realm": realm,
				"error": err.Error(),
			})
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": ErrInvalidToken,
				"code":  "TOKEN_VERIFICATION_ERROR",
			})
			return
		}

		// Use verified claims
		claims = verifiedClaims

		// Validate token expiration (extra check)
		if exp, ok := claims["exp"].(float64); ok {
			if time.Now().Unix() > int64(exp) {
				logAuthEvent("WARN", "Authentication failed - token expired", c, map[string]interface{}{
					"exp": int64(exp),
					"now": time.Now().Unix(),
				})
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
					"error": ErrInvalidToken,
					"code":  "TOKEN_EXPIRED",
				})
				return
			}
		}

		// Extract user information for logging
		userInfo := extractUserInfo(claims)
		logAuthEvent("INFO", "Authentication successful", c, userInfo)

		// Create a token object with verified claims for middleware access
		token := &jwt.Token{
			Claims: claims,
			Valid:  true,
		}
		c.Set("token", token)

		// Attach claims, user info, and realm to context
		c.Set("claims", claims)
		c.Set("user_info", userInfo)
		c.Set("realm", realm)

		c.Next()
	}
}

// extractUserInfo extracts user information from JWT claims for logging
func extractUserInfo(claims jwt.MapClaims) map[string]interface{} {
	userInfo := make(map[string]interface{})

	if sub, ok := claims["sub"].(string); ok {
		userInfo["sub"] = sub
	} else if preferredUsername, ok := claims["preferred_username"].(string); ok {
		// Use preferred_username as sub if sub is not available
		userInfo["sub"] = preferredUsername
	}
	if email, ok := claims["email"].(string); ok {
		userInfo["email"] = email
	}
	if preferredUsername, ok := claims["preferred_username"].(string); ok {
		userInfo["username"] = preferredUsername
	}
	if name, ok := claims["name"].(string); ok {
		userInfo["name"] = name
	}
	// Add tenant_id to userInfo if available
	if tenantID, ok := claims["tenant_id"].(string); ok {
		userInfo["tenant_id"] = tenantID
	}

	roles := extractRoles(claims)
	if len(roles) > 0 {
		userInfo["roles"] = roles
	}

	return userInfo
}

func RequireRole(role string) gin.HandlerFunc {
	return func(c *gin.Context) {
		claims, exists := c.Get("claims")
		if !exists {
			logAuthEvent("WARN", "Authorization failed - no user context", c, nil)
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": ErrNoUserContext,
				"code":  "NO_USER_CONTEXT",
			})
			return
		}

		mapClaims, ok := claims.(jwt.MapClaims)
		if !ok {
			logAuthEvent("ERROR", "Authorization failed - invalid claims format", c, nil)
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": ErrInvalidClaimsFormat,
				"code":  "CLAIMS_FORMAT_ERROR",
			})
			return
		}

		userRoles := extractRoles(mapClaims)

		// Check if user is suspended first
		for _, r := range userRoles {
			if r == "suspended" {
				userInfo := extractUserInfo(mapClaims)
				logAuthEvent("WARN", "Authorization failed - user suspended", c, map[string]interface{}{
					"required_role": role,
					"user_roles":    userRoles,
					"user":          userInfo["username"],
				})
				c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
					"error": "Account suspended",
					"code":  "ACCOUNT_SUSPENDED",
				})
				return
			}
		}

		hasRole := false
		for _, r := range userRoles {
			if r == role {
				hasRole = true
				break
			}
		}

		if !hasRole {
			userInfo := extractUserInfo(mapClaims)
			logAuthEvent("WARN", "Authorization failed - insufficient role", c, map[string]interface{}{
				"required_role": role,
				"user_roles":    userRoles,
				"user":          userInfo["username"],
			})
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"error":         ErrInsufficientRole,
				"code":          "INSUFFICIENT_ROLE",
				"required_role": role,
			})
			return
		}

		logAuthEvent("DEBUG", "Authorization successful", c, map[string]interface{}{
			"required_role": role,
			"user_roles":    userRoles,
		})

		c.Next()
	}
}

func extractRoles(claims jwt.MapClaims) []string {
	// Try to extract roles from realm_access first (Keycloak default)
	if realmAccess, ok := claims["realm_access"].(map[string]interface{}); ok {
		if rolesInterface, ok := realmAccess["roles"].([]interface{}); ok {
			roles := make([]string, 0, len(rolesInterface))
			for _, r := range rolesInterface {
				if roleStr, ok := r.(string); ok {
					roles = append(roles, roleStr)
				}
			}
			return roles
		}
	}

	// Fallback: try to extract from top-level "roles" claim
	if rolesInterface, ok := claims["roles"].([]interface{}); ok {
		roles := make([]string, 0, len(rolesInterface))
		for _, r := range rolesInterface {
			if roleStr, ok := r.(string); ok {
				roles = append(roles, roleStr)
			}
		}
		return roles
	}

	// Fallback: try string array at top level
	if rolesStringArray, ok := claims["roles"].([]string); ok {
		return rolesStringArray
	}

	return []string{}
}

func RequireAnyRole(allowedRoles ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		claims, exists := c.Get("claims")
		if !exists {
			logAuthEvent("WARN", "Authorization failed - no user context", c, nil)
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": ErrNoUserContext,
				"code":  "NO_USER_CONTEXT",
			})
			return
		}

		mapClaims, ok := claims.(jwt.MapClaims)
		if !ok {
			logAuthEvent("ERROR", "Authorization failed - invalid claims format", c, nil)
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": ErrInvalidClaimsFormat,
				"code":  "CLAIMS_FORMAT_ERROR",
			})
			return
		}

		userRoles := extractRoles(mapClaims)

		// Use a map for O(1) lookup instead of nested loops
		userRoleSet := make(map[string]struct{}, len(userRoles))
		for _, r := range userRoles {
			userRoleSet[r] = struct{}{}
		}

		// Check if user has any of the allowed roles
		for _, allowedRole := range allowedRoles {
			if _, found := userRoleSet[allowedRole]; found {
				logAuthEvent("DEBUG", "Authorization successful", c, map[string]interface{}{
					"allowed_roles": allowedRoles,
					"user_roles":    userRoles,
					"matched_role":  allowedRole,
				})
				c.Next()
				return
			}
		}

		// No matching role found
		userInfo := extractUserInfo(mapClaims)
		logAuthEvent("WARN", "Authorization failed - insufficient role", c, map[string]interface{}{
			"allowed_roles": allowedRoles,
			"user_roles":    userRoles,
			"user":          userInfo["username"],
		})
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
			"error":         ErrInsufficientRole,
			"code":          "INSUFFICIENT_ROLE",
			"allowed_roles": allowedRoles,
		})
	}
}

// GetUserFromContext extracts user information from the Gin context
func GetUserFromContext(c *gin.Context) (map[string]interface{}, bool) {
	userInfo, exists := c.Get("user_info")
	if !exists {
		return nil, false
	}

	if info, ok := userInfo.(map[string]interface{}); ok {
		return info, true
	}

	return nil, false
}

// GetUserRoles extracts user roles from the Gin context
func GetUserRoles(c *gin.Context) ([]string, bool) {
	claims, exists := c.Get("claims")
	if !exists {
		return nil, false
	}

	mapClaims, ok := claims.(jwt.MapClaims)
	if !ok {
		return nil, false
	}

	return extractRoles(mapClaims), true
}

// HasRole checks if the current user has a specific role
func HasRole(c *gin.Context, role string) bool {
	roles, exists := GetUserRoles(c)
	if !exists {
		return false
	}

	for _, r := range roles {
		if r == role {
			return true
		}
	}

	return false
}

// HasAnyRole checks if the current user has any of the specified roles
func HasAnyRole(c *gin.Context, roles ...string) bool {
	userRoles, exists := GetUserRoles(c)
	if !exists {
		return false
	}

	userRoleSet := make(map[string]struct{}, len(userRoles))
	for _, r := range userRoles {
		userRoleSet[r] = struct{}{}
	}

	for _, role := range roles {
		if _, found := userRoleSet[role]; found {
			return true
		}
	}

	return false
}

// GetRolesFromContext retrieves the user roles from the request context
func GetRolesFromContext(c *gin.Context) ([]string, bool) {
	// First try to get roles from user_info (extracted during JWT validation)
	if userInfo, exists := c.Get("user_info"); exists {
		if info, ok := userInfo.(map[string]interface{}); ok {
			if roles, roleExists := info["roles"]; roleExists {
				if roleStrings, ok := roles.([]string); ok {
					return roleStrings, true
				}
			}
		}
	}

	// Fallback: extract roles directly from JWT claims
	if claims, exists := c.Get("claims"); exists {
		if mapClaims, ok := claims.(jwt.MapClaims); ok {
			roles := extractRoles(mapClaims)
			return roles, len(roles) > 0
		}
	}

	return []string{}, false
}

// GetUserSubjectFromContext retrieves the user subject (sub claim) from context
func GetUserSubjectFromContext(c *gin.Context) (string, bool) {
	if claims, exists := c.Get("claims"); exists {
		if mapClaims, ok := claims.(jwt.MapClaims); ok {
			if sub, ok := mapClaims["sub"].(string); ok {
				return sub, true
			}
		}
	}
	return "", false
}

// GetTenantIDFromJWTClaims retrieves tenant ID from JWT claims
func GetTenantIDFromJWTClaims(c *gin.Context) (string, bool) {
	if claims, exists := c.Get("claims"); exists {
		if mapClaims, ok := claims.(jwt.MapClaims); ok {
			if tenantID, ok := mapClaims["tenant_id"].(string); ok {
				return tenantID, true
			}
		}
	}
	return "", false
}

// GetRealmFromContext retrieves the tenant_id (realm) from the Gin context
// Returns the tenant_id and a boolean indicating if it was found
func GetRealmFromContext(c *gin.Context) (string, bool) {
	if tenantID, exists := c.Get("tenant_id"); exists {
		if tid, ok := tenantID.(string); ok {
			return tid, true
		}
	}
	return "", false
}

// RefreshTokenRequest represents a request to refresh a token
type RefreshTokenRequest struct {
	RefreshToken string `json:"refresh_token"`
	GrantType    string `json:"grant_type"`
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
}

// RefreshTokenResponse represents the response from Keycloak token endpoint
type RefreshTokenResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	RefreshToken string `json:"refresh_token"`
	Scope        string `json:"scope"`
}

// ValidateRefreshToken validates a refresh token against Keycloak
func ValidateRefreshToken(realm, refreshToken, clientID, clientSecret string) (*RefreshTokenResponse, error) {
	if Registry.KeycloakURL == "" {
		return nil, fmt.Errorf("keycloak URL not configured")
	}

	tokenURL := fmt.Sprintf("%s/realms/%s/protocol/openid-connect/token", Registry.KeycloakURL, realm)

	// Create form data for token refresh
	data := fmt.Sprintf("grant_type=refresh_token&refresh_token=%s&client_id=%s&client_secret=%s",
		refreshToken, clientID, clientSecret)

	// Create secure HTTP client with proper TLS validation
	client := tlsutil.CreateSecureHTTPClient()
	client.Timeout = 10 * time.Second

	req, err := http.NewRequest("POST", tokenURL, strings.NewReader(data))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("token refresh failed with status: %d", resp.StatusCode)
	}

	var tokenResp RefreshTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return nil, err
	}

	return &tokenResp, nil
}

// ValidateJWTWithRevocationCheck validates a JWT token and checks for revocation
func ValidateJWTWithRevocationCheck(tokenString string) (jwt.MapClaims, error) {
	// Parse the token
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}

		// Get issuer from token to determine which JWKS to use
		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			return nil, fmt.Errorf("invalid claims format")
		}

		issuer, ok := claims["iss"].(string)
		if !ok {
			return nil, fmt.Errorf("missing issuer in token")
		}

		// Extract realm from issuer (e.g., "https://localhost:9443/realms/tenant1" -> "tenant1")
		parts := strings.Split(issuer, "/realms/")
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid issuer format: %s", issuer)
		}
		realm := parts[1]
		issuerBaseURL := parts[0]

		// Get the appropriate JWKS for this realm
		jwks, err := Registry.GetJWKSForIssuer(realm, issuerBaseURL)
		if err != nil {
			return nil, fmt.Errorf("failed to get JWKS for realm %s: %v", realm, err)
		}

		key, err := jwks.Keyfunc(token)
		if err != nil {
			// If key not found, try to refresh JWKS and retry once
			if strings.Contains(err.Error(), "key ID") || strings.Contains(err.Error(), "not found") {
				// Force refresh by removing from cache and reloading
				Registry.Mu.Lock()
				cacheKey := fmt.Sprintf("%s:%s", realm, issuerBaseURL)
				delete(Registry.JwksStore, cacheKey)
				Registry.Mu.Unlock()

				// Reload JWKS
				jwks, refreshErr := Registry.GetJWKSForIssuer(realm, issuerBaseURL)
				if refreshErr != nil {
					return nil, fmt.Errorf("failed to refresh JWKS for realm %s: %v", realm, refreshErr)
				}

				// Retry with refreshed JWKS
				key, err = jwks.Keyfunc(token)
				if err != nil {
					return nil, err
				}
			} else {
				return nil, err
			}
		}

		return key, nil
	})

	if err != nil {
		return nil, err
	}

	if !token.Valid {
		return nil, fmt.Errorf("token is not valid")
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, fmt.Errorf("invalid claims format")
	}

	// Extract tenant_id, user_id, and jti for revocation check
	tenantID, _ := claims["tenant_id"].(string)
	userID, _ := claims["sub"].(string)
	jti, _ := claims["jti"].(string)

	// Check if token is revoked
	if tenantID != "" && userID != "" && jti != "" {
		if IsTokenRevoked(tenantID, userID, jti) {
			return nil, fmt.Errorf("token has been revoked")
		}
	}

	return claims, nil
}

// IsJWKSReady checks if JWKS registry has at least one loaded realm
func IsJWKSReady() bool {
	if Registry == nil {
		return false
	}

	Registry.Mu.RLock()
	defer Registry.Mu.RUnlock()

	return len(Registry.JwksStore) > 0 && Registry.DefaultRealm != ""
}

// GetJWKSStatus returns detailed status of JWKS registry
func GetJWKSStatus() map[string]interface{} {
	if Registry == nil {
		return map[string]interface{}{
			"status": "not_initialized",
			"realms": 0,
		}
	}

	Registry.Mu.RLock()
	defer Registry.Mu.RUnlock()

	realms := make([]string, 0, len(Registry.JwksStore))
	for realm := range Registry.JwksStore {
		realms = append(realms, realm)
	}

	return map[string]interface{}{
		"status":        "ready",
		"realms":        len(Registry.JwksStore),
		"realm_list":    realms,
		"default_realm": Registry.DefaultRealm,
		"keycloak_url":  Registry.KeycloakURL,
	}
}

// validateTrustedIssuer checks if the issuer is in the trusted issuers whitelist
func (r *JWKSRegistry) validateTrustedIssuer(issuerBaseURL string) bool {
	if len(r.TrustedIssuers) == 0 {
		// If no trusted issuers configured, fall back to default behavior
		log.Println("[WARN] No trusted issuers configured - potential SSRF risk")
		return true
	}

	// Check against trusted issuers whitelist
	for _, trustedIssuer := range r.TrustedIssuers {
		if issuerBaseURL == trustedIssuer {
			return true
		}
	}

	log.Printf("[SECURITY] Untrusted issuer blocked: %s", issuerBaseURL)
	return false
}

// SetTrustedIssuers sets the whitelist of trusted issuer base URLs
func (r *JWKSRegistry) SetTrustedIssuers(trustedIssuers []string) {
	r.Mu.Lock()
	defer r.Mu.Unlock()
	r.TrustedIssuers = trustedIssuers
	log.Printf("[INFO] Trusted issuers configured: %v", trustedIssuers)
}
