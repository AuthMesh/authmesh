package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func TestSecurityHeadersMiddleware(t *testing.T) {
	router := gin.New()
	router.Use(SecurityHeadersMiddleware())
	router.GET("/test", func(c *gin.Context) { c.String(200, "ok") })

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/test", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)
	assert.Equal(t, "nosniff", w.Header().Get("X-Content-Type-Options"))
	assert.Equal(t, "DENY", w.Header().Get("X-Frame-Options"))
	assert.Equal(t, "1; mode=block", w.Header().Get("X-XSS-Protection"))
	assert.Equal(t, "strict-origin-when-cross-origin", w.Header().Get("Referrer-Policy"))
	assert.Equal(t, "default-src 'self'", w.Header().Get("Content-Security-Policy"))
}

func TestCalculateRetryAfterSeconds(t *testing.T) {
	tests := []struct {
		name      string
		rate      float64
		minExpect int
		maxExpect int
	}{
		{"zero rate returns default", 0, 60, 60},
		{"negative rate returns default", -5, 60, 60},
		{"very high rate", 1000, 1, 1},
		{"moderate rate", 10, 1, 300},
		{"very low rate", 0.1, 1, 300},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CalculateRetryAfterSeconds(tt.rate)
			assert.GreaterOrEqual(t, result, tt.minExpect)
			assert.LessOrEqual(t, result, tt.maxExpect)
		})
	}
}

func TestExtractUserIDFromContext(t *testing.T) {
	t.Run("from JWT claims", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("GET", "/", nil)
		c.Set("claims", map[string]interface{}{"sub": "user-123"})

		result := extractUserIDFromContext(c)
		assert.Equal(t, "user-123", result)
	})

	t.Run("from X-User-ID header", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("GET", "/", nil)
		c.Request.Header.Set("X-User-ID", "header-user")

		result := extractUserIDFromContext(c)
		assert.Equal(t, "header-user", result)
	})

	t.Run("empty when nothing available", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("GET", "/", nil)

		result := extractUserIDFromContext(c)
		assert.Equal(t, "", result)
	})
}

func TestExtractUserGroupsFromContext(t *testing.T) {
	t.Run("groups as string slice", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("GET", "/", nil)
		c.Set("claims", map[string]interface{}{
			"groups": []string{"group1", "group2"},
		})

		result := extractUserGroupsFromContext(c)
		assert.Equal(t, []string{"group1", "group2"}, result)
	})

	t.Run("groups as interface slice", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("GET", "/", nil)
		c.Set("claims", map[string]interface{}{
			"groups": []interface{}{"a", "b"},
		})

		result := extractUserGroupsFromContext(c)
		assert.Equal(t, []string{"a", "b"}, result)
	})

	t.Run("no groups", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("GET", "/", nil)

		result := extractUserGroupsFromContext(c)
		assert.Empty(t, result)
	})
}

func TestAuditLogMiddleware(t *testing.T) {
	router := gin.New()
	router.Use(AuditLogMiddleware())
	router.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/test", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestStructuredLogMiddleware(t *testing.T) {
	logger, _ := zap.NewDevelopment()

	t.Run("with logger", func(t *testing.T) {
		router := gin.New()
		router.Use(StructuredLogMiddleware(logger))
		router.GET("/test", func(c *gin.Context) { c.String(200, "ok") })

		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/test", nil)
		router.ServeHTTP(w, req)
		assert.Equal(t, 200, w.Code)
	})

	t.Run("with nil logger", func(t *testing.T) {
		router := gin.New()
		router.Use(StructuredLogMiddleware(nil))
		router.GET("/test", func(c *gin.Context) { c.String(200, "ok") })

		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/test", nil)
		router.ServeHTTP(w, req)
		assert.Equal(t, 200, w.Code)
	})

	t.Run("skips health endpoints", func(t *testing.T) {
		router := gin.New()
		router.Use(StructuredLogMiddleware(logger))
		router.GET("/health", func(c *gin.Context) { c.String(200, "ok") })

		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/health", nil)
		router.ServeHTTP(w, req)
		assert.Equal(t, 200, w.Code)
	})
}

func TestAuthEventLogger(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/test", nil)
	c.Request.Header.Set("User-Agent", "test-agent")
	c.Set("request_id", "req-123")

	// Should not panic for any log level
	AuthEventLogger(logger, "INFO", "test_event", c, map[string]interface{}{"key": "val"})
	AuthEventLogger(logger, "WARN", "test_warn", c, nil)
	AuthEventLogger(logger, "ERROR", "test_error", c, nil)
	AuthEventLogger(logger, "DEBUG", "test_debug", c, nil)
	AuthEventLogger(logger, "UNKNOWN", "test_unknown", c, nil)
}

func TestSetupCORS(t *testing.T) {
	mw := SetupCORS()
	assert.NotNil(t, mw)
}

func TestGetDefaultGroupRateLimit(t *testing.T) {
	limit := getDefaultGroupRateLimit()
	assert.Greater(t, limit, 0)
}
