package keycloak

// TokenResponse represents the response from Keycloak token endpoint
type TokenResponse struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   int    `json:"expires_in"`
	TokenType   string `json:"token_type,omitempty"`
	Scope       string `json:"scope,omitempty"`
}

// Realm represents a Keycloak realm with its attributes
type Realm struct {
	ID          string            `json:"id"`
	Realm       string            `json:"realm"`
	DisplayName string            `json:"displayName,omitempty"`
	Enabled     bool              `json:"enabled,omitempty"`
	Attributes  map[string]string `json:"attributes,omitempty"`
}
// User represents a Keycloak user (cleaned up for Milestone 3)
// User represents a Keycloak user (cleaned up for Milestone 3)
type User struct {
	   ID       string `json:"id,omitempty"`
	   Username string `json:"username"`
	   Email    string `json:"email,omitempty"`
	RequiredActions  []string            `json:"requiredActions,omitempty"`
	CreatedTimestamp int64               `json:"createdTimestamp,omitempty"`
}

// ErrorResponse represents a Keycloak API error response
type ErrorResponse struct {
	Error            string `json:"error,omitempty"`
	ErrorDescription string `json:"error_description,omitempty"`
	ErrorMessage     string `json:"errorMessage,omitempty"`
}

// RealmConfig represents configuration for creating a new realm
type RealmConfig struct {
	DisplayName string            `json:"displayName,omitempty"`
	Enabled     bool              `json:"enabled"`
	Attributes  map[string]string `json:"attributes,omitempty"`
	// Add other realm configuration options as needed
}

// tokenEntry represents a cached token with its expiration time
type tokenEntry struct {
	token     string
	expiresAt int64 // Unix timestamp
}
