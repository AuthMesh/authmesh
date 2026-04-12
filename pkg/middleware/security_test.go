package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestCORSSecurityCompliance(t *testing.T) {
	// Set up test environment with known origins
	t.Setenv("FRONTEND_URL", "https://trusted-app.com")
	t.Setenv("CORS_ALLOW_ORIGINS", "https://trusted-app.com,https://admin.trusted-app.com,http://localhost:3000")

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(SetupCORS())

	router.GET("/api/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})

	testCases := []struct {
		name                     string
		origin                   string
		expectedAllowOrigin      string
		expectedAllowCredentials string
		shouldAllowOrigin        bool
		description              string
	}{
		{
			name:                     "Trusted_Production_Origin",
			origin:                   "https://trusted-app.com",
			expectedAllowOrigin:      "https://trusted-app.com",
			expectedAllowCredentials: "true",
			shouldAllowOrigin:        true,
			description:              "Should allow trusted production origin",
		},
		{
			name:                     "Trusted_Admin_Origin",
			origin:                   "https://admin.trusted-app.com",
			expectedAllowOrigin:      "https://admin.trusted-app.com",
			expectedAllowCredentials: "true",
			shouldAllowOrigin:        true,
			description:              "Should allow trusted admin origin",
		},
		{
			name:                     "Trusted_Development_Origin",
			origin:                   "http://localhost:3000",
			expectedAllowOrigin:      "http://localhost:3000",
			expectedAllowCredentials: "true",
			shouldAllowOrigin:        true,
			description:              "Should allow trusted development origin",
		},
		{
			name:                     "Malicious_Origin_Attack",
			origin:                   "https://malicious-site.com",
			expectedAllowOrigin:      "",
			expectedAllowCredentials: "",
			shouldAllowOrigin:        false,
			description:              "Should block malicious origin",
		},
		{
			name:                     "Subdomain_Attack",
			origin:                   "https://evil.trusted-app.com",
			expectedAllowOrigin:      "",
			expectedAllowCredentials: "",
			shouldAllowOrigin:        false,
			description:              "Should block subdomain attacks",
		},
		{
			name:                     "Similar_Domain_Attack",
			origin:                   "https://trusted-app.com.evil.com",
			expectedAllowOrigin:      "",
			expectedAllowCredentials: "",
			shouldAllowOrigin:        false,
			description:              "Should block similar domain attacks",
		},
		{
			name:                     "HTTP_vs_HTTPS_Attack",
			origin:                   "http://trusted-app.com",
			expectedAllowOrigin:      "",
			expectedAllowCredentials: "",
			shouldAllowOrigin:        false,
			description:              "Should block HTTP version of HTTPS-only domain",
		},
		{
			name:                     "No_Origin_Header",
			origin:                   "",
			expectedAllowOrigin:      "",
			expectedAllowCredentials: "",
			shouldAllowOrigin:        false,
			description:              "Should handle missing origin header",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req, _ := http.NewRequest("GET", "/api/test", nil)
			if tc.origin != "" {
				req.Header.Set("Origin", tc.origin)
			}

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			// Check response status - gin-contrib/cors blocks with 403 for untrusted origins
			if tc.shouldAllowOrigin {
				assert.Equal(t, http.StatusOK, w.Code, "API should respond successfully for trusted origin")
			} else {
				// CORS middleware blocks untrusted origins with 403 - this is correct security behavior
				if w.Code == http.StatusForbidden {
					t.Logf("🛡️ %s: Correctly blocked with 403 status", tc.description)
				} else {
					assert.Equal(t, http.StatusOK, w.Code, "API should respond (CORS headers control access)")
				}
			}

			// Check CORS headers
			actualAllowOrigin := w.Header().Get("Access-Control-Allow-Origin")
			actualAllowCredentials := w.Header().Get("Access-Control-Allow-Credentials")

			if tc.shouldAllowOrigin {
				assert.Equal(t, tc.expectedAllowOrigin, actualAllowOrigin,
					"Should set correct Allow-Origin for trusted origin")
				assert.Equal(t, tc.expectedAllowCredentials, actualAllowCredentials,
					"Should allow credentials for trusted origin")
				t.Logf("✅ %s: Allowed origin %s", tc.description, tc.origin)
			} else {
				assert.Empty(t, actualAllowOrigin,
					"Should not set Allow-Origin for untrusted origin")
				assert.Empty(t, actualAllowCredentials,
					"Should not allow credentials for untrusted origin")
				t.Logf("🛡️ %s: Blocked origin %s", tc.description, tc.origin)
			}
		})
	}
}

func TestCORSPreflightSecurity(t *testing.T) {
	// Test preflight (OPTIONS) requests security
	t.Setenv("CORS_ALLOW_ORIGINS", "https://trusted.com")

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(SetupCORS())

	router.POST("/api/sensitive", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"data": "sensitive"})
	})

	t.Run("Preflight_From_Trusted_Origin", func(t *testing.T) {
		req, _ := http.NewRequest("OPTIONS", "/api/sensitive", nil)
		req.Header.Set("Origin", "https://trusted.com")
		req.Header.Set("Access-Control-Request-Method", "POST")
		req.Header.Set("Access-Control-Request-Headers", "Content-Type")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		// Should allow preflight
		assert.Equal(t, http.StatusNoContent, w.Code, "Should allow preflight from trusted origin")
		assert.Equal(t, "https://trusted.com", w.Header().Get("Access-Control-Allow-Origin"))
		assert.Equal(t, "true", w.Header().Get("Access-Control-Allow-Credentials"))
	})

	t.Run("Preflight_From_Malicious_Origin", func(t *testing.T) {
		req, _ := http.NewRequest("OPTIONS", "/api/sensitive", nil)
		req.Header.Set("Origin", "https://malicious.com")
		req.Header.Set("Access-Control-Request-Method", "POST")
		req.Header.Set("Access-Control-Request-Headers", "Content-Type")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		// Should block preflight
		assert.Empty(t, w.Header().Get("Access-Control-Allow-Origin"),
			"Should not allow preflight from malicious origin")
		assert.Empty(t, w.Header().Get("Access-Control-Allow-Credentials"),
			"Should not allow credentials from malicious origin")
	})
}

func TestCORSConfiguration_EdgeCases(t *testing.T) {
	// Test various configuration edge cases
	t.Run("Empty_Origins_Configuration", func(t *testing.T) {
		t.Setenv("CORS_ALLOW_ORIGINS", "")
		t.Setenv("FRONTEND_URL", "https://default.com")

		gin.SetMode(gin.TestMode)
		router := gin.New()
		router.Use(SetupCORS())

		router.GET("/test", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"status": "ok"})
		})

		// Should fall back to default origins (frontend URL + localhost)
		req, _ := http.NewRequest("GET", "/test", nil)
		req.Header.Set("Origin", "https://default.com")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, "https://default.com", w.Header().Get("Access-Control-Allow-Origin"),
			"Should allow default frontend URL")
	})

	t.Run("Wildcard_Origin_Security", func(t *testing.T) {
		// Ensure wildcard origins are handled securely
		t.Setenv("CORS_ALLOW_ORIGINS", "https://safe.com")

		gin.SetMode(gin.TestMode)
		router := gin.New()
		router.Use(SetupCORS())

		router.GET("/test", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"status": "ok"})
		})

		// Try to exploit with wildcard
		req, _ := http.NewRequest("GET", "/test", nil)
		req.Header.Set("Origin", "*")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		// Should not allow wildcard
		assert.Empty(t, w.Header().Get("Access-Control-Allow-Origin"),
			"Should not allow wildcard origin")
		assert.Empty(t, w.Header().Get("Access-Control-Allow-Credentials"),
			"Should not allow credentials with wildcard")
	})
}
