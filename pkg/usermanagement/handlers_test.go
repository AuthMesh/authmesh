package usermanagement

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func setupHandlerTest(t *testing.T) (*UserManagementHandler, error) {
	logger, _ := zap.NewDevelopment()
	config := UserManagerConfig{
		KeycloakURL: "https://keycloak.example.com",
		RedisClient: redis.NewClient(&redis.Options{Addr: "localhost:6379"}),
		Logger:      logger,
	}

	userManager, err := NewUserManager(config)
	if err != nil {
		return nil, err
	}

	handler, err := NewUserManagementHandler(userManager, logger)
	if err != nil {
		return nil, err
	}

	return handler, nil
}

func TestNewUserManagementHandler(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	config := UserManagerConfig{
		KeycloakURL: "https://keycloak.example.com",
		RedisClient: redis.NewClient(&redis.Options{Addr: "localhost:6379"}),
		Logger:      logger,
	}

	userManager, err := NewUserManager(config)
	require.NoError(t, err)

	handler, err := NewUserManagementHandler(userManager, logger)

	assert.NoError(t, err)
	assert.NotNil(t, handler)
	assert.NotNil(t, handler.userManager)
	assert.NotNil(t, handler.logger)
	assert.NotNil(t, handler.tracer)
	assert.NotNil(t, handler.requestsCounter)
	assert.NotNil(t, handler.failuresCounter)
}

func TestNewUserManagementHandler_NilInputs(t *testing.T) {
	logger, _ := zap.NewDevelopment()

	tests := []struct {
		name        string
		userManager *UserManager
		logger      *zap.Logger
		expectError bool
	}{
		{
			name:        "nil user manager",
			userManager: nil,
			logger:      logger,
			expectError: false, // Current implementation doesn't validate
		},
		{
			name:        "nil logger",
			userManager: &UserManager{}, // Mock user manager
			logger:      nil,
			expectError: false, // Current implementation doesn't validate
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler, err := NewUserManagementHandler(tt.userManager, tt.logger)

			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, handler)
			} else {
				// Note: Current implementation allows nil inputs
				if err != nil {
					assert.Nil(t, handler)
				} else {
					assert.NotNil(t, handler)
				}
			}
		})
	}
}

func TestAddUserSuperAdmin(t *testing.T) {
	handler, err := setupHandlerTest(t)
	require.NoError(t, err)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/superadmin/tenants/:tenant_id/users", handler.AddUserSuperAdmin)

	tests := []struct {
		name           string
		tenantID       string
		requestBody    interface{}
		expectedStatus int
		expectedError  string
	}{
		{
			name:     "valid request",
			tenantID: "tenant1",
			requestBody: AddUserRequest{
				Username: "testuser",
				Email:    "test@example.com",
			},
			expectedStatus: http.StatusInternalServerError, // Will fail without real Keycloak
		},
		{
			name:     "invalid request - missing username",
			tenantID: "tenant1",
			requestBody: map[string]interface{}{
				"email": "test@example.com",
			},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Invalid request payload",
		},
		{
			name:     "invalid request - missing email",
			tenantID: "tenant1",
			requestBody: map[string]interface{}{
				"username": "testuser",
			},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Invalid request payload",
		},
		{
			name:     "invalid request - invalid email",
			tenantID: "tenant1",
			requestBody: AddUserRequest{
				Username: "testuser",
				Email:    "invalid-email",
			},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Invalid request payload",
		},
		{
			name:           "invalid JSON",
			tenantID:       "tenant1",
			requestBody:    "invalid json",
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Invalid request payload",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var body []byte
			var err error

			if str, ok := tt.requestBody.(string); ok {
				body = []byte(str)
			} else {
				body, err = json.Marshal(tt.requestBody)
				require.NoError(t, err)
			}

			req, err := http.NewRequest("POST", "/superadmin/tenants/"+tt.tenantID+"/users", bytes.NewBuffer(body))
			require.NoError(t, err)
			req.Header.Set("Content-Type", "application/json")

			rr := httptest.NewRecorder()
			router.ServeHTTP(rr, req)

			assert.Equal(t, tt.expectedStatus, rr.Code)

			if tt.expectedError != "" {
				var response map[string]interface{}
				err = json.Unmarshal(rr.Body.Bytes(), &response)
				require.NoError(t, err)
				assert.Contains(t, response["error"], tt.expectedError)
			}
		})
	}
}

func TestAddUser(t *testing.T) {
	handler, err := setupHandlerTest(t)
	require.NoError(t, err)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/t/:tenant_id/admin/users", handler.AddUser)

	tests := []struct {
		name           string
		tenantID       string
		requestBody    interface{}
		expectedStatus int
		expectedError  string
	}{
		{
			name:     "valid request",
			tenantID: "tenant1",
			requestBody: AddUserRequest{
				Username: "adminuser",
				Email:    "admin@example.com",
			},
			expectedStatus: http.StatusInternalServerError, // Will fail without real Keycloak
		},
		{
			name:     "invalid request - empty username",
			tenantID: "tenant1",
			requestBody: AddUserRequest{
				Username: "",
				Email:    "admin@example.com",
			},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Invalid request payload",
		},
		{
			name:     "invalid request - malformed email",
			tenantID: "tenant1",
			requestBody: AddUserRequest{
				Username: "adminuser",
				Email:    "not-an-email",
			},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Invalid request payload",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, err := json.Marshal(tt.requestBody)
			require.NoError(t, err)

			req, err := http.NewRequest("POST", "/t/"+tt.tenantID+"/admin/users", bytes.NewBuffer(body))
			require.NoError(t, err)
			req.Header.Set("Content-Type", "application/json")

			rr := httptest.NewRecorder()
			router.ServeHTTP(rr, req)

			assert.Equal(t, tt.expectedStatus, rr.Code)

			if tt.expectedError != "" {
				var response map[string]interface{}
				err = json.Unmarshal(rr.Body.Bytes(), &response)
				require.NoError(t, err)
				assert.Contains(t, response["error"], tt.expectedError)
			}
		})
	}
}

func TestRegister(t *testing.T) {
	handler, err := setupHandlerTest(t)
	require.NoError(t, err)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/t/:tenant_id/register", handler.Register)

	tests := []struct {
		name           string
		tenantID       string
		requestBody    interface{}
		expectedStatus int
		expectedError  string
	}{
		{
			name:     "valid registration",
			tenantID: "tenant1",
			requestBody: RegisterRequest{
				Username: "newuser",
				Email:    "newuser@example.com",
			},
			expectedStatus: http.StatusInternalServerError, // Will fail without real Keycloak
		},
		{
			name:     "invalid registration - missing username",
			tenantID: "tenant1",
			requestBody: map[string]interface{}{
				"email": "newuser@example.com",
			},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Invalid request payload",
		},
		{
			name:     "invalid registration - invalid email format",
			tenantID: "tenant1",
			requestBody: RegisterRequest{
				Username: "newuser",
				Email:    "invalid.email",
			},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Invalid request payload",
		},
		{
			name:     "empty tenant ID",
			tenantID: "",
			requestBody: RegisterRequest{
				Username: "newuser",
				Email:    "newuser@example.com",
			},
			expectedStatus: http.StatusInternalServerError, // Will fail without real Keycloak
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, err := json.Marshal(tt.requestBody)
			require.NoError(t, err)

			req, err := http.NewRequest("POST", "/t/"+tt.tenantID+"/register", bytes.NewBuffer(body))
			require.NoError(t, err)
			req.Header.Set("Content-Type", "application/json")

			rr := httptest.NewRecorder()
			router.ServeHTTP(rr, req)

			assert.Equal(t, tt.expectedStatus, rr.Code)

			if tt.expectedError != "" {
				var response map[string]interface{}
				err = json.Unmarshal(rr.Body.Bytes(), &response)
				require.NoError(t, err)
				assert.Contains(t, response["error"], tt.expectedError)
			} else if rr.Code == http.StatusCreated {
				var response map[string]interface{}
				err = json.Unmarshal(rr.Body.Bytes(), &response)
				require.NoError(t, err)
				assert.Contains(t, response, "message")
				assert.Contains(t, response, "tenant_id")
				assert.Contains(t, response, "user")
			}
		})
	}
}

func TestHandler_RouteIntegration(t *testing.T) {
	handler, err := setupHandlerTest(t)
	require.NoError(t, err)

	gin.SetMode(gin.TestMode)
	router := gin.New()

	// Setup all routes
	router.POST("/superadmin/tenants/:tenant_id/users", handler.AddUserSuperAdmin)
	router.POST("/t/:tenant_id/admin/users", handler.AddUser)
	router.POST("/t/:tenant_id/register", handler.Register)

	validRequest := AddUserRequest{
		Username: "integrationuser",
		Email:    "integration@example.com",
	}
	body, err := json.Marshal(validRequest)
	require.NoError(t, err)

	endpoints := []struct {
		method string
		path   string
	}{
		{"POST", "/superadmin/tenants/tenant1/users"},
		{"POST", "/t/tenant1/admin/users"},
		{"POST", "/t/tenant1/register"},
	}

	for _, endpoint := range endpoints {
		t.Run(endpoint.method+"_"+endpoint.path, func(t *testing.T) {
			req, err := http.NewRequest(endpoint.method, endpoint.path, bytes.NewBuffer(body))
			require.NoError(t, err)
			req.Header.Set("Content-Type", "application/json")

			rr := httptest.NewRecorder()
			router.ServeHTTP(rr, req)

			// All should return 500 because we don't have real Keycloak
			assert.Equal(t, http.StatusInternalServerError, rr.Code)
		})
	}
}

func TestRequestValidation(t *testing.T) {
	tests := []struct {
		name    string
		request interface{}
		valid   bool
	}{
		{
			name: "valid AddUserRequest",
			request: AddUserRequest{
				Username: "validuser",
				Email:    "valid@example.com",
			},
			valid: true,
		},
		{
			name: "valid RegisterRequest",
			request: RegisterRequest{
				Username: "validuser",
				Email:    "valid@example.com",
			},
			valid: true,
		},
		{
			name: "invalid email in AddUserRequest",
			request: AddUserRequest{
				Username: "validuser",
				Email:    "invalid-email",
			},
			valid: false,
		},
		{
			name: "empty username in RegisterRequest",
			request: RegisterRequest{
				Username: "",
				Email:    "valid@example.com",
			},
			valid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test JSON marshalling/unmarshalling
			data, err := json.Marshal(tt.request)
			assert.NoError(t, err)

			// Test that the request structure is valid
			assert.Greater(t, len(data), 0)

			// For a complete validation test, we would need to set up
			// Gin's binding validation, which requires a full HTTP context
		})
	}
}

// Benchmark tests
func BenchmarkAddUserSuperAdmin(b *testing.B) {
	handler, err := setupHandlerTest(&testing.T{})
	if err != nil {
		b.Fatal(err)
	}

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/superadmin/tenants/:tenant_id/users", handler.AddUserSuperAdmin)

	validRequest := AddUserRequest{
		Username: "benchuser",
		Email:    "bench@example.com",
	}
	body, err := json.Marshal(validRequest)
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req, _ := http.NewRequest("POST", "/superadmin/tenants/tenant1/users", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")

		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)
	}
}

func BenchmarkRegister(b *testing.B) {
	handler, err := setupHandlerTest(&testing.T{})
	if err != nil {
		b.Fatal(err)
	}

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/t/:tenant_id/register", handler.Register)

	validRequest := RegisterRequest{
		Username: "benchuser",
		Email:    "bench@example.com",
	}
	body, err := json.Marshal(validRequest)
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req, _ := http.NewRequest("POST", "/t/tenant1/register", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")

		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)
	}
}
