# Security Guide

## Overview

AuthMesh implements defense-in-depth security principles with multi-layered protection for multi-tenant authentication scenarios.

## Authentication & Authorization

### JWT Security

```go
// Secure JWT validation with proper verification
authMesh := platform.NewProduction(platform.Config{
    KeycloakURL: "https://auth.example.com",
    TrustedIssuers: []string{
        "https://auth.example.com", // Only trust specific issuers
    },
    RequiredClaims: []string{"tenant", "sub", "iat", "exp"},
})
```

**Security Features:**
- Signature verification using RS256/ES256
- Issuer whitelist validation
- Expiration time enforcement
- Required claims validation
- JWKS key rotation support

### Middleware Security Patterns

**Critical Security Pattern: Proper Middleware Chaining**

AuthMesh provides secure middleware patterns that prevent common authentication bypass vulnerabilities:

```go
// ✅ SECURE: Combined auth + role middleware
admin := router.Group("/api/v1/admin")
admin.Use(authMesh.AuthMiddleware("admin")) // Combines JWT validation + role check atomically

// ✅ SECURE: Separate middleware for different requirements
public := router.Group("/api/v1/public")
public.Use(authMesh.AuthMiddleware()) // JWT validation only

protected := router.Group("/api/v1/protected")
protected.Use(authMesh.AuthMiddleware()) // JWT validation
protected.Use(authMesh.RequireRole("user")) // Additional role check
```

**❌ AVOID: Manual middleware chaining that can bypass security**

```go
// NEVER do this - creates race conditions and security bypasses
func insecureMiddleware(c *gin.Context) {
    authMw := auth.RequireAuth()
    authMw(c) // This calls c.Next() immediately, bypassing role checks
    
    // Role check runs AFTER handler execution - TOO LATE!
    roleMw := auth.RequireRole("admin") 
    roleMw(c)
}
```

**Security Guarantees:**
- Atomic authentication and authorization checks
- No handler execution until ALL security checks pass
- Proper `c.AbortWithStatusJSON()` behavior prevents response tampering
- Context variables are set correctly for downstream handlers
- Audit logging captures all security decisions in order

**Implementation Details:**
The `AuthMiddleware(role)` pattern ensures:
1. JWT validation happens first (signature, expiration, issuer)
2. Claims are extracted and validated
3. Role requirements are checked atomically
4. Only on complete success does `c.Next()` continue to handlers
5. Any failure calls `c.AbortWithStatusJSON()` immediately

### Multi-Tenant Isolation

```go
// Tenant isolation at the middleware level
router.Use(authMesh.TenantIsolation()) // Enforces tenant boundaries
router.Use(authMesh.RequireRole("admin")) // Role-based access
```

**Isolation Mechanisms:**
- Tenant claim validation
- Cross-tenant request prevention
- Resource-level authorization
- Audit trail per tenant

## Network Security

### TLS Configuration

```go
// Production TLS settings
tlsConfig := &tls.Config{
    MinVersion: tls.VersionTLS12,
    CipherSuites: []uint16{
        tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
        tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,
        tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
    },
    PreferServerCipherSuites: true,
    InsecureSkipVerify: false, // Never skip in production
}
```

### CORS Protection

```go
// Secure CORS configuration
corsConfig := cors.Config{
    AllowOrigins: []string{
        "https://app.example.com",
        "https://admin.example.com",
    },
    AllowMethods: []string{"GET", "POST", "PUT", "DELETE"},
    AllowHeaders: []string{"Authorization", "Content-Type"},
    AllowCredentials: true,
    MaxAge: 12 * time.Hour,
}
```

## Attack Prevention

### Rate Limiting

```go
// Multi-layer rate limiting
authMesh.SetupRateLimiting(platform.RateLimitConfig{
    Global:    1000, // Requests per minute globally
    PerUser:   100,  // Per authenticated user
    PerIP:     50,   // Per IP address
    PerTenant: 500,  // Per tenant
})
```

**Protection Against:**
- DDoS attacks
- Brute force authentication
- API abuse
- Resource exhaustion

### Authentication Bypass Prevention

**Critical Vulnerability: Response Tampering**

AuthMesh prevents a critical class of authentication bypass vulnerabilities where middleware execution order can allow unauthorized responses:

```go
// ✅ PROTECTED: AuthMesh prevents this attack pattern
// Scenario: Attacker with "viewer" role tries to access admin endpoint
// 1. JWT validates successfully (user is authenticated)
// 2. Role check fails (viewer != admin required)
// 3. AuthMesh properly aborts with 403 BEFORE handler execution
// 4. Handler never runs, preventing data leakage
```

**Vulnerability Pattern Prevented:**
```
❌ VULNERABLE PATTERN (Fixed in AuthMesh):
1. Auth middleware validates JWT ✓
2. Auth middleware calls c.Next() → Handler executes ✓
3. Handler returns sensitive data with 200 OK ✓
4. Role middleware runs AFTER handler → Tries to abort with 403 ✗
5. Result: Response contains BOTH 200 data AND 403 error
```

**AuthMesh Protection:**
- Atomic security validation before handler execution
- Proper `c.AbortWithStatusJSON()` prevents response tampering
- No `c.Next()` until ALL security requirements are satisfied
- Comprehensive audit logging of security decisions

### SSRF Prevention

```go
// Built-in SSRF protection
authMesh := platform.NewProduction(platform.Config{
    BlockPrivateNetworks: true,
    AllowedDomains: []string{
        "auth.example.com",
        "api.example.com",
    },
})
```

**Protections:**
- Private network blocking (10.0.0.0/8, 172.16.0.0/12, 192.168.0.0/16)
- Localhost blocking (127.0.0.1, ::1)
- Domain whitelist enforcement
- URL validation

### Security Headers

```go
// Automatic security headers
router.Use(authMesh.SecurityHeaders())
```

**Headers Applied:**
```
X-Content-Type-Options: nosniff
X-Frame-Options: DENY  
X-XSS-Protection: 1; mode=block
Strict-Transport-Security: max-age=31536000; includeSubDomains
Content-Security-Policy: default-src 'self'
Referrer-Policy: strict-origin-when-cross-origin
```

## Data Protection

### Sensitive Data Handling

```go
// Secure context handling
type SecureContext struct {
    UserID   string `json:"user_id"`
    TenantID string `json:"tenant_id"`
    Roles    []string `json:"roles"`
    // Sensitive fields are automatically masked in logs
}
```

### Audit Logging

```go
// Comprehensive audit trail
authMesh.EnableAuditLogging(platform.AuditConfig{
    LogAuthenticationEvents: true,
    LogAuthorizationFailures: true,
    LogRateLimitExceeded: true,
    LogSensitiveOperations: true,
    RedactSensitiveData: true, // PII masking
})
```

**Audit Events:**
- Authentication success/failure
- Authorization decisions
- Rate limit violations
- Security policy violations
- Configuration changes

## Input Validation

### Request Validation

```go
// Built-in input sanitization
router.Use(authMesh.InputValidation())
```

**Validations:**
- Content-Type verification
- Request size limits
- Character encoding validation
- Malicious payload detection

### JWT Claims Validation

```go
// Strict claims validation
validator := auth.JWTValidator{
    RequiredClaims: []string{"sub", "iat", "exp", "tenant"},
    MaxTokenAge: 24 * time.Hour,
    ClockSkew: 5 * time.Minute,
}
```

## Secrets Management

### Environment Variables

```bash
# Use secrets management systems
KEYCLOAK_CLIENT_SECRET="$(vault kv get -field=secret secret/keycloak)"
JWT_SIGNING_KEY="$(aws ssm get-parameter --name /app/jwt-key --with-decryption)"
REDIS_PASSWORD="$(kubectl get secret redis -o jsonpath='{.data.password}' | base64 -d)"
```

### Key Rotation

```go
// Automatic key rotation support
authMesh := platform.NewProduction(platform.Config{
    JWKSRefreshInterval: 15 * time.Minute, // Refresh keys regularly
    KeyRotationGracePeriod: 1 * time.Hour, // Accept old keys briefly
})
```

## Monitoring & Alerting

### Security Metrics

```go
// Monitor security events
prometheus.NewCounterVec(prometheus.CounterOpts{
    Name: "authmesh_auth_failures_total",
    Help: "Total authentication failures",
}, []string{"tenant", "reason"})

prometheus.NewCounterVec(prometheus.CounterOpts{
    Name: "authmesh_rate_limit_exceeded_total", 
    Help: "Rate limit violations",
}, []string{"tenant", "user", "limit_type"})
```

### Alert Rules

```yaml
# Prometheus alerting rules
groups:
- name: authmesh-security
  rules:
  - alert: HighAuthFailureRate
    expr: rate(authmesh_auth_failures_total[5m]) > 10
    for: 2m
    annotations:
      summary: High authentication failure rate detected
      
  - alert: RateLimitExceeded
    expr: rate(authmesh_rate_limit_exceeded_total[1m]) > 5
    for: 1m
    annotations:
      summary: Rate limiting threshold exceeded
```

## Compliance

### GDPR Compliance

- **Data Minimization**: Only essential claims are processed
- **Right to Erasure**: Audit logs can be purged per user
- **Data Portability**: Standard JWT format for user data
- **Consent Management**: Integration with consent management platforms

### SOC 2 Type II

- **Access Controls**: Role-based authorization
- **Audit Logging**: Comprehensive security event logging
- **Encryption**: TLS 1.2+ for data in transit
- **Monitoring**: Real-time security monitoring

## Vulnerability Management

### Security Scanning

```bash
# Regular dependency scanning
go list -json -m all | nancy sleuth

# Container scanning
trivy image authmesh:latest

# Static analysis
gosec ./...
```

### Security Debugging

**Diagnosing Authentication Issues:**

```bash
# Check for middleware execution order issues
docker logs backend-container | grep -E "(Authentication|Authorization|Headers were already written)"

# Look for response tampering patterns
curl -v -H "Authorization: Bearer $INVALID_ROLE_TOKEN" http://localhost:8080/admin/endpoint
# Should return clean 403, not mixed 200+403 response
```

**Common Security Debugging Patterns:**

```go
// Add debug logging to middleware
func debugAuthMiddleware(c *gin.Context) {
    log.Printf("[DEBUG] Auth middleware: Starting, IsAborted=%v", c.IsAborted())
    
    // ... authentication logic ...
    
    if c.IsAborted() {
        log.Printf("[DEBUG] Auth middleware: Aborted early")
        return
    }
    
    log.Printf("[DEBUG] Auth middleware: Completed successfully, calling c.Next()")
    c.Next()
}
```

**Warning Signs of Security Issues:**
- "Headers were already written" errors in logs
- Mixed success/error JSON responses
- Middleware logs showing wrong execution order
- 200 status codes when expecting 401/403

### Patch Management

```go
// Keep dependencies updated
go get -u all
go mod tidy
```

**Update Schedule:**
- Critical patches: Within 24 hours
- Security patches: Within 1 week  
- Regular updates: Monthly

## Security Testing

### Penetration Testing

```bash
# JWT security testing
jwt-hack verify --token $TOKEN --key $PUBLIC_KEY

# Rate limiting testing
ab -n 10000 -c 100 http://localhost:8080/api/endpoint

# SSRF testing
curl -H "X-Forwarded-For: 127.0.0.1" http://localhost:8080/api/endpoint
```

### Fuzzing

```go
// Security fuzzing with go-fuzz
func FuzzJWTValidation(f *testing.F) {
    f.Fuzz(func(t *testing.T, token string) {
        validator := auth.NewJWTValidator(config)
        _, err := validator.ValidateToken(token)
        // Should never panic, even with malformed input
    })
}
```

## Incident Response

### Security Incident Playbook

1. **Detection**: Monitor alerts and logs
2. **Containment**: Rate limit or block suspicious IPs
3. **Analysis**: Analyze audit logs and attack patterns
4. **Recovery**: Rotate keys, patch vulnerabilities
5. **Lessons Learned**: Update security policies

### Emergency Procedures

```bash
# Emergency key rotation
kubectl create secret generic jwt-keys --from-file=new-key.pem
kubectl rollout restart deployment authmesh

# Block suspicious IPs
kubectl patch configmap rate-limits --patch '{"data": {"blocked-ips": "1.2.3.4,5.6.7.8"}}'
```

## Best Practices Summary

1. **Zero Trust**: Verify everything, trust nothing
2. **Defense in Depth**: Multiple security layers
3. **Least Privilege**: Minimal required permissions
4. **Regular Updates**: Keep dependencies current
5. **Monitor Everything**: Comprehensive logging and alerting
6. **Incident Preparation**: Have response procedures ready
7. **Security Testing**: Regular penetration testing
8. **Documentation**: Keep security procedures documented
