# Keycloak REST API Client

A comprehensive Go client library for interacting with Keycloak REST API v26.0.0.

## Features

- **Token Caching**: Automatic token caching with 30-second buffer for renewal
- **Retry Logic**: Exponential backoff retry for 429/503 status codes (3 attempts max)
- **OpenTelemetry Integration**: Built-in metrics and tracing
- **Thread-Safe**: Concurrent access to token cache is protected
- **Error Handling**: Detailed error messages with context
- **API Versioning**: Uses Keycloak API v26.0.0 for stability

## Usage

### Basic Setup

```go
import (
    "golang-multi-tenant/pkg/keycloak"
    "go.uber.org/zap"
    "go.opentelemetry.io/otel"
)

// Initialize logger and tracer
logger, _ := zap.NewProduction()
tracer := otel.Tracer("my-app")

// Create client
client, err := keycloak.NewClient("https://localhost:8080", logger, tracer)
if err != nil {
    log.Fatal(err)
}
```

### Authentication

```go
// Get admin token (cached automatically)
token, err := client.GetAdminToken("management-client", "your-secret")
if err != nil {
    log.Fatal(err)
}
```

### Realm Management

```go
// Get realm attributes
attrs, err := client.GetRealmAttributes("tenant1", token)
if err != nil {
    log.Fatal(err)
}

// Set realm attributes
newAttrs := map[string]string{
    "max_rate_limit_requests_per_hour": "2000",
}
err = client.SetRealmAttributes("tenant1", token, newAttrs)
if err != nil {
    log.Fatal(err)
}

// Create new realm
config := map[string]interface{}{
    "id": "tenant3",
    "realm": "tenant3",
    "enabled": true,
    "displayName": "Tenant 3",
}
err = client.CreateRealm("tenant3", token, config)
if err != nil {
    log.Fatal(err)
}
```

### Group Management

```go
// Get group attributes
attrs, err := client.GetGroupAttributes("tenant1", "team-a-id", token)
if err != nil {
    log.Fatal(err)
}

// Set group attributes
groupAttrs := map[string]string{
    "rate_limit_requests_per_hour": "1000",
}
err = client.SetGroupAttributes("tenant1", "team-a-id", token, groupAttrs)
if err != nil {
    log.Fatal(err)
}
```

### User Management

```go
// Add new user
user := map[string]interface{}{
    "username": "testuser",
    "email": "test@example.com",
    "enabled": true,
    "emailVerified": false,
}
err = client.AddUser("tenant1", token, user)
if err != nil {
    log.Fatal(err)
}

// Get user by ID
userData, err := client.GetUser("tenant1", "user-id", token)
if err != nil {
    log.Fatal(err)
}
```

## Configuration

### Environment Variables

- `KEYCLOAK_URL`: Base URL for Keycloak (default: https://localhost:8080)

### Client Configuration

The client is configured with:
- **Timeout**: 10 seconds for HTTP requests
- **Retry Attempts**: 3 attempts with exponential backoff
- **Token Buffer**: 30 seconds before token expiry for refresh
- **Connection Pool**: 10 idle connections per host

## Error Handling

The client implements comprehensive error handling:

```go
token, err := client.GetAdminToken("client-id", "secret")
if err != nil {
    // Handle authentication errors
    log.Printf("Failed to get token: %v", err)
}

attrs, err := client.GetRealmAttributes("realm", token)
if err != nil {
    // Handle API errors with detailed context
    log.Printf("Failed to get realm attributes: %v", err)
}
```

## OpenTelemetry Metrics

The client exports the following metrics:

- `keycloak_requests_total{endpoint}`: Total number of requests
- `keycloak_errors_total{endpoint}`: Total number of errors

## OpenTelemetry Tracing

All API calls are traced with the following attributes:

- `realm`: Target realm name
- `operation`: Operation type (e.g., get_token, set_realm_attributes)
- `http.method`: HTTP method
- `http.status_code`: Response status code

## Rate Limiting Integration

This client is designed to work with the rate limiting system:

```go
// Example: Set group rate limit
groupAttrs := map[string]string{
    "rate_limit_requests_per_hour": "1000",
}
err = client.SetGroupAttributes("tenant1", "team-a-id", token, groupAttrs)

// Example: Set realm max rate limit
realmAttrs := map[string]string{
    "max_rate_limit_requests_per_hour": "2000",
}
err = client.SetRealmAttributes("tenant1", token, realmAttrs)
```

## Thread Safety

The client is thread-safe and can be used concurrently from multiple goroutines. Token caching is protected by mutexes.

## API Compatibility

This client targets Keycloak API version 26.0.0. All requests include the `version=26.0.0` parameter for API stability.
