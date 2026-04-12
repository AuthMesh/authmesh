package keycloak

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

// UserAdmin provides high-level user and role management against the Keycloak
// Admin REST API. It wraps a *Client and a set of per-realm service-account
// credentials so callers do not need to manage tokens themselves.
//
// Thread-safe — token caching is handled by the underlying Client.
type UserAdmin struct {
	client       *Client
	clientID     string
	clientSecret string
	logger       *zap.Logger
}

// NewUserAdmin creates a UserAdmin that authenticates via the service account
// of the given client (client_credentials grant) in each realm.
//
// The client must have the realm-management "manage-users" role assigned to its
// service-account user in every realm it will manage.
func NewUserAdmin(client *Client, clientID, clientSecret string, logger *zap.Logger) *UserAdmin {
	return &UserAdmin{
		client:       client,
		clientID:     clientID,
		clientSecret: clientSecret,
		logger:       logger,
	}
}

// token gets a cached service-account token for the given realm.
func (ua *UserAdmin) token(realm string) (string, error) {
	return ua.client.GetClientCredentialsToken(realm, ua.clientID, ua.clientSecret)
}

// ---------- User CRUD ---------------------------------------------------------

// KCUser represents the subset of Keycloak user fields we manage.
type KCUser struct {
	ID            string `json:"id,omitempty"`
	Username      string `json:"username"`
	Email         string `json:"email,omitempty"`
	FirstName     string `json:"firstName,omitempty"`
	LastName      string `json:"lastName,omitempty"`
	Enabled       bool   `json:"enabled"`
	EmailVerified bool   `json:"emailVerified,omitempty"`
}

// FindUserByEmail returns the first user matching the email in the given realm,
// or nil if no match is found.
func (ua *UserAdmin) FindUserByEmail(realm, email string) (*KCUser, error) {
	tok, err := ua.token(realm)
	if err != nil {
		return nil, err
	}

	ctx := context.Background()
	ctx, span := ua.client.tracer.Start(ctx, "keycloak.FindUserByEmail",
		trace.WithAttributes(
			attribute.String("realm", realm),
			attribute.String("email", email),
		),
	)
	defer span.End()

	path := fmt.Sprintf("/admin/realms/%s/users?email=%s&exact=true&max=1",
		url.PathEscape(realm), url.QueryEscape(email))

	body, err := ua.client.doRequest(ctx, "GET", path, tok, nil)
	if err != nil {
		return nil, fmt.Errorf("find user by email: %w", err)
	}

	var users []KCUser
	if err := json.Unmarshal(body, &users); err != nil {
		return nil, fmt.Errorf("parse user list: %w", err)
	}
	if len(users) == 0 {
		return nil, nil
	}
	return &users[0], nil
}

// CreateUser creates a user in Keycloak. Returns the Keycloak user ID.
// If a user with the same email already exists, returns the existing ID
// (idempotent).
func (ua *UserAdmin) CreateUser(realm string, user KCUser) (string, error) {
	tok, err := ua.token(realm)
	if err != nil {
		return "", err
	}

	ctx := context.Background()
	ctx, span := ua.client.tracer.Start(ctx, "keycloak.UserAdmin.CreateUser",
		trace.WithAttributes(
			attribute.String("realm", realm),
			attribute.String("email", user.Email),
		),
	)
	defer span.End()

	payload := map[string]interface{}{
		"username":      user.Username,
		"email":         user.Email,
		"firstName":     user.FirstName,
		"lastName":      user.LastName,
		"enabled":       user.Enabled,
		"emailVerified": user.EmailVerified,
	}

	_, err = ua.client.doRequest(ctx, "POST",
		fmt.Sprintf("/admin/realms/%s/users", url.PathEscape(realm)), tok, payload)

	if err != nil {
		// 409 = user already exists — treat as success.
		var apiErr *APIError
		if errors.As(err, &apiErr) && apiErr.StatusCode == 409 {
			ua.logger.Info("Keycloak user already exists, looking up",
				zap.String("realm", realm),
				zap.String("email", user.Email))
			existing, findErr := ua.FindUserByEmail(realm, user.Email)
			if findErr != nil {
				return "", fmt.Errorf("user exists but lookup failed: %w", findErr)
			}
			if existing != nil {
				return existing.ID, nil
			}
			return "", fmt.Errorf("user exists but could not be found by email")
		}
		return "", fmt.Errorf("create user: %w", err)
	}

	// Keycloak returns 201 with Location header but no body.
	// Look up the user to get the ID.
	created, err := ua.FindUserByEmail(realm, user.Email)
	if err != nil {
		return "", fmt.Errorf("created user but lookup failed: %w", err)
	}
	if created == nil {
		return "", fmt.Errorf("created user but could not find by email")
	}
	return created.ID, nil
}

// EnsureUser finds or creates a Keycloak user by email and returns the KC user ID
// and whether the user was newly created.
func (ua *UserAdmin) EnsureUser(realm string, user KCUser) (kcID string, created bool, err error) {
	existing, err := ua.FindUserByEmail(realm, user.Email)
	if err != nil {
		return "", false, err
	}
	if existing != nil {
		return existing.ID, false, nil
	}
	id, err := ua.CreateUser(realm, user)
	return id, err == nil, err
}

// ---------- Realm Role Management ---------------------------------------------

// RoleRepresentation is the Keycloak role object.
type RoleRepresentation struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// getRealmRole fetches a single realm role by name.
func (ua *UserAdmin) getRealmRole(ctx context.Context, realm, roleName, tok string) (*RoleRepresentation, error) {
	body, err := ua.client.doRequest(ctx, "GET",
		fmt.Sprintf("/admin/realms/%s/roles/%s", url.PathEscape(realm), url.PathEscape(roleName)),
		tok, nil)
	if err != nil {
		return nil, err
	}
	var role RoleRepresentation
	if err := json.Unmarshal(body, &role); err != nil {
		return nil, fmt.Errorf("parse role: %w", err)
	}
	return &role, nil
}

// EnsureRealmRole creates a realm role if it doesn't already exist.
func (ua *UserAdmin) EnsureRealmRole(realm, roleName string) error {
	tok, err := ua.token(realm)
	if err != nil {
		return err
	}
	ctx := context.Background()
	_, err = ua.getRealmRole(ctx, realm, roleName, tok)
	if err == nil {
		return nil // already exists
	}
	// Try to create it.
	payload := map[string]interface{}{"name": roleName}
	_, createErr := ua.client.doRequest(ctx, "POST",
		fmt.Sprintf("/admin/realms/%s/roles", url.PathEscape(realm)), tok, payload)
	if createErr != nil {
		var apiErr *APIError
		if errors.As(createErr, &apiErr) && apiErr.StatusCode == 409 {
			return nil // race: created between check and create
		}
	}
	return createErr
}

// getUserRealmRoles returns all realm-level roles assigned to a user.
func (ua *UserAdmin) getUserRealmRoles(ctx context.Context, realm, kcUserID, tok string) ([]RoleRepresentation, error) {
	body, err := ua.client.doRequest(ctx, "GET",
		fmt.Sprintf("/admin/realms/%s/users/%s/role-mappings/realm",
			url.PathEscape(realm), url.PathEscape(kcUserID)),
		tok, nil)
	if err != nil {
		return nil, err
	}
	var roles []RoleRepresentation
	if err := json.Unmarshal(body, &roles); err != nil {
		return nil, fmt.Errorf("parse user realm roles: %w", err)
	}
	return roles, nil
}

// SetUserRealmRoles sets a user's realm roles to exactly the given set.
// It removes any roles not in the desired set and adds any that are missing.
// Built-in roles (e.g. "default-roles-*", "offline_access", "uma_authorization")
// are left untouched.
func (ua *UserAdmin) SetUserRealmRoles(realm, kcUserID string, desiredRoles []string) error {
	tok, err := ua.token(realm)
	if err != nil {
		return err
	}

	ctx := context.Background()
	ctx, span := ua.client.tracer.Start(ctx, "keycloak.UserAdmin.SetUserRealmRoles",
		trace.WithAttributes(
			attribute.String("realm", realm),
			attribute.String("kc_user_id", kcUserID),
			attribute.Int("desired_role_count", len(desiredRoles)),
		),
	)
	defer span.End()

	// Our managed roles — anything outside this set is left alone.
	managedRoles := map[string]bool{
		"student":      true,
		"teacher":      true,
		"school_admin": true,
		"tenant_admin": true,
		"super_admin":  true,
	}

	// Build desired set.
	desiredSet := make(map[string]bool, len(desiredRoles))
	for _, r := range desiredRoles {
		r = strings.TrimSpace(strings.ToLower(r))
		if r != "" {
			desiredSet[r] = true
		}
	}

	// Get current roles.
	current, err := ua.getUserRealmRoles(ctx, realm, kcUserID, tok)
	if err != nil {
		return fmt.Errorf("get current roles: %w", err)
	}
	currentByName := make(map[string]RoleRepresentation, len(current))
	for _, r := range current {
		currentByName[r.Name] = r
	}

	// Compute removals (managed roles that user has but shouldn't).
	var toRemove []RoleRepresentation
	for _, r := range current {
		if managedRoles[r.Name] && !desiredSet[r.Name] {
			toRemove = append(toRemove, r)
		}
	}

	// Compute additions (desired roles the user doesn't have).
	var toAdd []RoleRepresentation
	for r := range desiredSet {
		if _, has := currentByName[r]; !has {
			// Ensure the role exists in the realm first.
			if err := ua.EnsureRealmRole(realm, r); err != nil {
				return fmt.Errorf("ensure role %q: %w", r, err)
			}
			role, err := ua.getRealmRole(ctx, realm, r, tok)
			if err != nil {
				return fmt.Errorf("get role %q: %w", r, err)
			}
			toAdd = append(toAdd, *role)
		}
	}

	// Remove unwanted roles.
	if len(toRemove) > 0 {
		_, err := ua.client.doRequest(ctx, "DELETE",
			fmt.Sprintf("/admin/realms/%s/users/%s/role-mappings/realm",
				url.PathEscape(realm), url.PathEscape(kcUserID)),
			tok, toRemove)
		if err != nil {
			return fmt.Errorf("remove roles: %w", err)
		}
		ua.logger.Info("Removed realm roles",
			zap.String("realm", realm),
			zap.String("kc_user_id", kcUserID),
			zap.Any("roles", roleNames(toRemove)))
	}

	// Add new roles.
	if len(toAdd) > 0 {
		_, err := ua.client.doRequest(ctx, "POST",
			fmt.Sprintf("/admin/realms/%s/users/%s/role-mappings/realm",
				url.PathEscape(realm), url.PathEscape(kcUserID)),
			tok, toAdd)
		if err != nil {
			return fmt.Errorf("add roles: %w", err)
		}
		ua.logger.Info("Added realm roles",
			zap.String("realm", realm),
			zap.String("kc_user_id", kcUserID),
			zap.Any("roles", roleNames(toAdd)))
	}

	return nil
}

// SyncUserRoles is a convenience that ensures the KC user exists and sets their roles.
// It returns the Keycloak user ID and the temporary password (non-empty only for
// newly created users).
func (ua *UserAdmin) SyncUserRoles(realm string, user KCUser, roles []string) (kcID string, tempPassword string, err error) {
	kcID, isNew, err := ua.EnsureUser(realm, user)
	if err != nil {
		return "", "", fmt.Errorf("ensure user: %w", err)
	}
	if err := ua.SetUserRealmRoles(realm, kcID, roles); err != nil {
		return kcID, "", fmt.Errorf("set roles: %w", err)
	}
	// Set a temporary password only for newly created users.
	if isNew {
		tempPassword = GenerateTempPassword(user.FirstName, user.LastName)
		if setErr := ua.SetTemporaryPassword(realm, kcID, tempPassword); setErr != nil {
			ua.logger.Warn("Failed to set temporary password",
				zap.String("realm", realm),
				zap.String("kc_user_id", kcID),
				zap.Error(setErr))
			tempPassword = "" // don't return a password that wasn't set
		}
	}
	return kcID, tempPassword, nil
}

func roleNames(roles []RoleRepresentation) []string {
	names := make([]string, len(roles))
	for i, r := range roles {
		names[i] = r.Name
	}
	return names
}

// ---------- Password Management -----------------------------------------------

// SetTemporaryPassword sets a temporary password on a Keycloak user.
// The user will be required to change it on next login.
func (ua *UserAdmin) SetTemporaryPassword(realm, kcUserID, password string) error {
	tok, err := ua.token(realm)
	if err != nil {
		return err
	}

	ctx := context.Background()
	payload := map[string]interface{}{
		"type":      "password",
		"value":     password,
		"temporary": true,
	}

	_, err = ua.client.doRequest(ctx, "PUT",
		fmt.Sprintf("/admin/realms/%s/users/%s/reset-password",
			url.PathEscape(realm), url.PathEscape(kcUserID)),
		tok, payload)
	if err != nil {
		return fmt.Errorf("set temporary password: %w", err)
	}
	return nil
}

// GenerateTempPassword builds a predictable temporary password from the user's
// name: first 3 chars of first name + first 3 chars of last name + "123",
// with the 1st and 4th characters uppercased.
// Examples: "Dinesh","Ramasamy" → "DinRam123"; "Jo","Li" → "JoLi123"
func GenerateTempPassword(firstName, lastName string) string {
	first := strings.TrimSpace(firstName)
	last := strings.TrimSpace(lastName)

	take := func(s string, n int) string {
		s = strings.ToLower(s)
		if len(s) > n {
			return s[:n]
		}
		return s
	}

	part1 := take(first, 3)
	part2 := take(last, 3)
	combined := part1 + part2

	// Uppercase 1st and 4th characters (0-indexed: 0 and 3).
	runes := []rune(combined)
	for i := range runes {
		if i == 0 || i == 3 {
			runes[i] = []rune(strings.ToUpper(string(runes[i])))[0]
		}
	}

	return string(runes) + "123"
}
