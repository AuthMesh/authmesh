# Migration Guide

## Overview

This guide helps you migrate from various authentication systems to AuthMesh, ensuring a smooth transition with minimal downtime.

## Migration Strategies

### 1. From Custom JWT Solutions

#### Before: Manual JWT Handling
```go
// Old approach - manual JWT validation
func validateJWT(tokenString string) (*jwt.Token, error) {
    token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
        if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
            return nil, fmt.Errorf("unexpected signing method")
        }
        return publicKey, nil
    })
    
    if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
        // Manual claim validation
        if exp, ok := claims["exp"].(float64); ok {
            if time.Unix(int64(exp), 0).Before(time.Now()) {
                return nil, fmt.Errorf("token expired")
            }
        }
        return token, nil
    }
    return nil, err
}
```

#### After: AuthMesh Integration
```go
// New approach - AuthMesh handles everything
func main() {
    authMesh, err := platform.QuickStart("myapp")
    if err != nil {
        log.Fatal(err)
    }
    
    router := gin.New()
    authMesh.SetupAll(router) // JWT validation, rate limiting, etc.
    
    // Protected routes automatically validate JWTs
    api := router.Group("/api")
    api.Use(authMesh.RequireAuth())
    api.GET("/users", getUsersHandler)
}
```

### 2. From Auth0

#### Migration Steps

1. **Extract Configuration**
```bash
# Current Auth0 settings
AUTH0_DOMAIN="your-tenant.auth0.com"
AUTH0_CLIENT_ID="your-client-id"
AUTH0_CLIENT_SECRET="your-client-secret"
```

2. **Update to AuthMesh**
```go
// AuthMesh equivalent configuration
authMesh := platform.NewProduction(platform.Config{
    KeycloakURL: "https://your-keycloak.com", // Replace Auth0 with Keycloak
    TrustedIssuers: []string{
        "https://your-keycloak.com/realms/master",
    },
    DefaultRealm: "master",
})
```

3. **Token Format Migration**
```go
// Auth0 tokens -> Keycloak tokens
// Update your token generation to use Keycloak format
// AuthMesh handles the validation automatically
```

### 3. From Firebase Auth

#### Before: Firebase SDK
```go
// Firebase Auth validation
client, err := app.Auth(ctx)
if err != nil {
    log.Fatalf("error getting Auth client: %v\n", err)
}

token, err := client.VerifyIDToken(ctx, idToken)
if err != nil {
    log.Fatalf("error verifying ID token: %v\n", err)
}
```

#### After: AuthMesh
```go
// AuthMesh with Keycloak backend
authMesh, err := platform.NewWithDefaults("myapp")
if err != nil {
    log.Fatal(err)
}

// Use standard middleware approach
router.Use(authMesh.RequireAuth())
```

### 4. From AWS Cognito

#### Migration Process

1. **Export User Pool**
```bash
# Export users from Cognito
aws cognito-idp list-users --user-pool-id us-west-2_abcdef123
```

2. **Import to Keycloak**
```bash
# Use Keycloak user import
curl -X POST "https://keycloak.com/admin/realms/master/users" \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d @users.json
```

3. **Update Application**
```go
// Replace Cognito client
authMesh := platform.NewProduction(platform.Config{
    KeycloakURL: "https://your-keycloak.com",
    DefaultRealm: "master", // Map from Cognito User Pool
})
```

## Database Migration

### User Data Migration

#### Script Template
```go
package main

import (
    "context"
    "database/sql"
    "log"
    
    "github.com/AuthMesh/authmesh/pkg/keycloak"
)

func migrateUsers(oldDB *sql.DB, keycloakClient *keycloak.Client) error {
    rows, err := oldDB.Query("SELECT id, email, created_at FROM users")
    if err != nil {
        return err
    }
    defer rows.Close()
    
    for rows.Next() {
        var id, email string
        var createdAt time.Time
        
        if err := rows.Scan(&id, &email, &createdAt); err != nil {
            log.Printf("Error scanning row: %v", err)
            continue
        }
        
        // Create user in Keycloak
        user := keycloak.User{
            Username: email,
            Email:    email,
            Enabled:  true,
            Attributes: map[string][]string{
                "legacy_id": {id},
                "migrated_at": {time.Now().Format(time.RFC3339)},
            },
        }
        
        if err := keycloakClient.CreateUser(context.Background(), "master", user); err != nil {
            log.Printf("Error creating user %s: %v", email, err)
            continue
        }
        
        log.Printf("Migrated user: %s", email)
    }
    
    return nil
}
```

### Role Migration

```go
// Migrate roles and permissions
func migrateRoles(oldDB *sql.DB, keycloakClient *keycloak.Client) error {
    // Extract roles from old system
    roles, err := oldDB.Query("SELECT name, description FROM roles")
    if err != nil {
        return err
    }
    defer roles.Close()
    
    for roles.Next() {
        var name, description string
        roles.Scan(&name, &description)
        
        // Create role in Keycloak
        role := keycloak.Role{
            Name:        name,
            Description: description,
        }
        
        keycloakClient.CreateRole(context.Background(), "master", role)
    }
    
    return nil
}
```

## Configuration Migration

### Environment Variables

#### Old System
```bash
# Example legacy configuration
JWT_SECRET="your-secret-key"
JWT_EXPIRATION="24h"
AUTH_PROVIDER="custom"
USER_DB_URL="postgres://..."
```

#### AuthMesh Configuration
```bash
# AuthMesh environment variables
KEYCLOAK_URL="https://auth.example.com"
TRUSTED_ISSUERS="https://auth.example.com"
DEFAULT_REALM="master"
HTTP_TIMEOUT="5s"
RATE_LIMIT_ENABLED="true"
OBSERVABILITY_ENABLED="true"
```

### Configuration File Migration

#### Legacy config.yml
```yaml
auth:
  jwt_secret: "secret"
  expiration: "24h"
  issuer: "myapp"
database:
  url: "postgres://..."
  max_connections: 10
```

#### AuthMesh config
```go
// Configuration through code
authMesh := platform.NewProduction(platform.Config{
    KeycloakURL: os.Getenv("KEYCLOAK_URL"),
    TrustedIssuers: strings.Split(os.Getenv("TRUSTED_ISSUERS"), ","),
    HTTPTimeout: 5 * time.Second,
    RateLimiting: platform.RateLimitConfig{
        Enabled: true,
        Global:  1000,
        PerUser: 100,
    },
})
```

## Testing Migration

### Parallel Testing

```go
// Run both systems in parallel during migration
func TestMigration(t *testing.T) {
    // Old system
    oldAuth := NewOldAuthSystem()
    
    // New AuthMesh system
    authMesh, err := platform.NewForTesting("migration-test")
    require.NoError(t, err)
    
    // Test same tokens against both systems
    token := generateTestToken()
    
    // Validate with old system
    oldResult, oldErr := oldAuth.ValidateToken(token)
    
    // Validate with AuthMesh
    newResult, newErr := authMesh.ValidateToken(token)
    
    // Compare results
    assert.Equal(t, oldErr == nil, newErr == nil)
    if oldErr == nil && newErr == nil {
        assert.Equal(t, oldResult.Subject, newResult.Subject)
    }
}
```

### Load Testing

```bash
# Test both systems under load
hey -n 10000 -c 100 \
  -H "Authorization: Bearer $OLD_TOKEN" \
  http://old-system:8080/api/test

hey -n 10000 -c 100 \
  -H "Authorization: Bearer $NEW_TOKEN" \
  http://new-system:8080/api/test
```

## Deployment Strategies

### Blue-Green Deployment

```yaml
# Deploy AuthMesh alongside existing system
apiVersion: apps/v1
kind: Deployment
metadata:
  name: app-authmesh
spec:
  replicas: 3
  selector:
    matchLabels:
      app: myapp
      version: authmesh
  template:
    metadata:
      labels:
        app: myapp
        version: authmesh
    spec:
      containers:
      - name: app
        image: myapp:authmesh
        env:
        - name: KEYCLOAK_URL
          value: "https://auth.example.com"
---
# Gradually shift traffic
apiVersion: networking.istio.io/v1alpha3
kind: VirtualService
metadata:
  name: myapp
spec:
  http:
  - match:
    - headers:
        x-version:
          exact: authmesh
    route:
    - destination:
        host: myapp
        subset: authmesh
      weight: 100
  - route:
    - destination:
        host: myapp
        subset: legacy
      weight: 90
    - destination:
        host: myapp
        subset: authmesh
      weight: 10
```

### Canary Deployment

```bash
# Start with 5% traffic to AuthMesh
kubectl patch virtualservice myapp --type merge -p '
{
  "spec": {
    "http": [{
      "route": [
        {"destination": {"host": "myapp", "subset": "legacy"}, "weight": 95},
        {"destination": {"host": "myapp", "subset": "authmesh"}, "weight": 5}
      ]
    }]
  }
}'

# Gradually increase if no errors
# 5% -> 10% -> 25% -> 50% -> 100%
```

## Rollback Procedures

### Quick Rollback

```bash
# Revert traffic to old system
kubectl patch virtualservice myapp --type merge -p '
{
  "spec": {
    "http": [{
      "route": [
        {"destination": {"host": "myapp", "subset": "legacy"}, "weight": 100}
      ]
    }]
  }
}'
```

### Data Rollback

```go
// If user data needs to be rolled back
func rollbackUsers(keycloakClient *keycloak.Client, backupDB *sql.DB) error {
    // Identify migrated users
    users, err := keycloakClient.GetUsers(context.Background(), "master", keycloak.GetUsersOptions{
        Max: 1000,
    })
    if err != nil {
        return err
    }
    
    for _, user := range users {
        // Check if user was migrated
        if migratedAt, exists := user.Attributes["migrated_at"]; exists {
            // Delete from Keycloak
            err := keycloakClient.DeleteUser(context.Background(), "master", user.ID)
            if err != nil {
                log.Printf("Error deleting user %s: %v", user.Username, err)
            }
            
            // Restore to old system if needed
            // ... restore logic ...
        }
    }
    
    return nil
}
```

## Common Issues & Solutions

### Issue 1: Token Format Incompatibility

**Problem**: Existing tokens don't work with AuthMesh

**Solution**: 
```go
// Create token adapter
func adaptLegacyToken(legacyToken string) (string, error) {
    // Parse legacy token
    claims, err := parseLegacyToken(legacyToken)
    if err != nil {
        return "", err
    }
    
    // Convert to Keycloak format
    keycloakClaims := convertToKeycloakClaims(claims)
    
    // Generate new token
    return generateKeycloakToken(keycloakClaims)
}
```

### Issue 2: Performance Degradation

**Problem**: AuthMesh is slower than old system

**Solution**:
```go
// Enable performance optimizations
authMesh := platform.NewProduction(platform.Config{
    HTTPTimeout: 1 * time.Second, // Reduce timeout
    JWKSCacheTTL: 1 * time.Hour,  // Cache JWKS longer
    SkipJWKSInit: false,          // Pre-load JWKS
})

// Use Redis for rate limiting
redisClient := redis.NewClient(&redis.Options{
    Addr: "localhost:6379",
    PoolSize: 100, // Increase pool size
})
authMesh.SetRateLimiter(ratelimit.NewRedisLimiter(redisClient))
```

### Issue 3: User Data Loss

**Problem**: User attributes not migrated correctly

**Solution**:
```go
// Comprehensive user migration with validation
func migrateUserWithValidation(oldUser OldUser, keycloak *keycloak.Client) error {
    newUser := keycloak.User{
        Username: oldUser.Username,
        Email:    oldUser.Email,
        Enabled:  oldUser.Active,
        Attributes: map[string][]string{
            "legacy_id":     {oldUser.ID},
            "created_at":    {oldUser.CreatedAt.Format(time.RFC3339)},
            "last_login":    {oldUser.LastLogin.Format(time.RFC3339)},
            "user_type":     {oldUser.Type},
            "preferences":   {oldUser.Preferences},
        },
    }
    
    // Create user
    userID, err := keycloak.CreateUser(context.Background(), "master", newUser)
    if err != nil {
        return fmt.Errorf("failed to create user: %w", err)
    }
    
    // Migrate roles
    for _, role := range oldUser.Roles {
        err := keycloak.AssignRole(context.Background(), "master", userID, role)
        if err != nil {
            log.Printf("Warning: failed to assign role %s to user %s: %v", role, userID, err)
        }
    }
    
    // Verify migration
    migratedUser, err := keycloak.GetUser(context.Background(), "master", userID)
    if err != nil {
        return fmt.Errorf("failed to verify migrated user: %w", err)
    }
    
    if migratedUser.Email != oldUser.Email {
        return fmt.Errorf("migration verification failed: email mismatch")
    }
    
    return nil
}
```

## Post-Migration Checklist

- [ ] All users successfully migrated
- [ ] Roles and permissions properly mapped
- [ ] Authentication flow working end-to-end
- [ ] Authorization rules functioning correctly
- [ ] Rate limiting configured appropriately
- [ ] Monitoring and alerting set up
- [ ] Load testing completed successfully
- [ ] Rollback procedures tested
- [ ] Documentation updated
- [ ] Team trained on new system
- [ ] Legacy system decommissioning planned

## Support & Troubleshooting

### Getting Help

1. **Documentation**: Check the comprehensive docs in `/docs`
2. **Examples**: Review working examples in `/examples`
3. **Issues**: Open GitHub issues for bugs or questions
4. **Community**: Join the AuthMesh community discussions

### Migration Support

For complex migrations, consider:
- Professional services consultation
- Dedicated migration tooling
- Extended parallel testing periods
- Gradual feature migration approach
