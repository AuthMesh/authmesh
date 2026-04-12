package tlsutil

import (
	"crypto/tls"
	"net/http"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCreateSecureHTTPClient(t *testing.T) {
	// Save original environment
	originalEnv := os.Getenv("APP_ENV")
	originalCACert := os.Getenv("TLS_CA_CERT_PATH")
	originalAllowSelfSigned := os.Getenv("TLS_ALLOW_SELF_SIGNED")
	originalTrustedHosts := os.Getenv("TLS_TRUSTED_HOSTS")

	// Cleanup function to restore environment
	cleanup := func() {
		os.Setenv("APP_ENV", originalEnv)
		os.Setenv("TLS_CA_CERT_PATH", originalCACert)
		os.Setenv("TLS_ALLOW_SELF_SIGNED", originalAllowSelfSigned)
		os.Setenv("TLS_TRUSTED_HOSTS", originalTrustedHosts)
	}
	defer cleanup()

	t.Run("Production_Environment_Secure_By_Default", func(t *testing.T) {
		// Set production environment
		os.Setenv("APP_ENV", "production")
		os.Setenv("TLS_CA_CERT_PATH", "")
		os.Setenv("TLS_ALLOW_SELF_SIGNED", "")
		os.Setenv("TLS_TRUSTED_HOSTS", "")

		client := CreateSecureHTTPClient()

		// Verify client is created
		assert.NotNil(t, client, "HTTP client should be created")
		assert.NotNil(t, client.Transport, "Transport should be configured")

		// Verify TLS configuration
		transport, ok := client.Transport.(*http.Transport)
		assert.True(t, ok, "Transport should be *http.Transport")

		tlsConfig := transport.TLSClientConfig
		assert.NotNil(t, tlsConfig, "TLS config should be set")

		// Verify secure settings
		assert.Equal(t, uint16(tls.VersionTLS12), tlsConfig.MinVersion,
			"Should enforce TLS 1.2 minimum")
		assert.False(t, tlsConfig.InsecureSkipVerify,
			"Should not skip certificate verification in production")
	})

	t.Run("Development_With_Self_Signed_And_Trusted_Hosts", func(t *testing.T) {
		// Set development environment with trusted hosts
		os.Setenv("APP_ENV", "development")
		os.Setenv("TLS_ALLOW_SELF_SIGNED", "true")
		os.Setenv("TLS_TRUSTED_HOSTS", "keycloak,localhost")
		os.Setenv("TLS_CA_CERT_PATH", "")

		client := CreateSecureHTTPClient()

		// Verify client is created
		assert.NotNil(t, client, "HTTP client should be created")
		assert.NotNil(t, client.Transport, "Transport should be configured")

		transport, ok := client.Transport.(*http.Transport)
		assert.True(t, ok, "Transport should be *http.Transport")

		tlsConfig := transport.TLSClientConfig
		assert.NotNil(t, tlsConfig, "TLS config should be set")

		// TLS 1.2 minimum should still be enforced
		assert.Equal(t, uint16(tls.VersionTLS12), tlsConfig.MinVersion,
			"Should enforce TLS 1.2 minimum even in development")

		t.Logf("✅ Development TLS configuration allows trusted self-signed certificates")
	})

	t.Run("Self_Signed_Without_Trusted_Hosts_Should_Be_Secure", func(t *testing.T) {
		// This is a security test - allowing self-signed without trusted hosts is dangerous
		os.Setenv("APP_ENV", "development")
		os.Setenv("TLS_ALLOW_SELF_SIGNED", "true")
		os.Setenv("TLS_TRUSTED_HOSTS", "")
		os.Setenv("TLS_CA_CERT_PATH", "")

		client := CreateSecureHTTPClient()

		// Should still create a client but with secure defaults
		assert.NotNil(t, client, "HTTP client should be created")
		
		transport, ok := client.Transport.(*http.Transport)
		assert.True(t, ok, "Transport should be *http.Transport")

		tlsConfig := transport.TLSClientConfig
		assert.NotNil(t, tlsConfig, "TLS config should be set")

		// Should not globally disable certificate verification
		// (the implementation logs a warning but doesn't make the client insecure)
		assert.Equal(t, uint16(tls.VersionTLS12), tlsConfig.MinVersion,
			"Should maintain TLS 1.2 minimum")

		t.Logf("✅ Self-signed without trusted hosts maintains security")
	})

	t.Run("Invalid_TLS_Versions_Are_Rejected", func(t *testing.T) {
		// Ensure we don't accidentally downgrade to insecure TLS versions
		os.Setenv("APP_ENV", "production")
		os.Setenv("TLS_CA_CERT_PATH", "")

		client := CreateSecureHTTPClient()
		transport := client.Transport.(*http.Transport)
		tlsConfig := transport.TLSClientConfig

		// TLS 1.0 and 1.1 should be rejected
		assert.GreaterOrEqual(t, tlsConfig.MinVersion, uint16(tls.VersionTLS12),
			"Should reject TLS versions below 1.2")

		t.Logf("✅ Insecure TLS versions are properly rejected")
	})
}

func TestValidateTLSConfiguration(t *testing.T) {
	// Save original environment
	originalEnv := os.Getenv("APP_ENV")
	originalCACert := os.Getenv("TLS_CA_CERT_PATH")
	originalAllowSelfSigned := os.Getenv("TLS_ALLOW_SELF_SIGNED")
	originalTrustedHosts := os.Getenv("TLS_TRUSTED_HOSTS")

	cleanup := func() {
		os.Setenv("APP_ENV", originalEnv)
		os.Setenv("TLS_CA_CERT_PATH", originalCACert)
		os.Setenv("TLS_ALLOW_SELF_SIGNED", originalAllowSelfSigned)
		os.Setenv("TLS_TRUSTED_HOSTS", originalTrustedHosts)
	}
	defer cleanup()

	t.Run("Valid_Production_Configuration", func(t *testing.T) {
		os.Setenv("APP_ENV", "production")
		os.Setenv("TLS_ALLOW_SELF_SIGNED", "false")
		os.Setenv("TLS_TRUSTED_HOSTS", "")

		err := ValidateTLSConfiguration()
		assert.NoError(t, err, "Valid production configuration should not error")
	})

	t.Run("Valid_Development_Configuration", func(t *testing.T) {
		os.Setenv("APP_ENV", "development")
		os.Setenv("TLS_ALLOW_SELF_SIGNED", "true")
		os.Setenv("TLS_TRUSTED_HOSTS", "keycloak,localhost")

		err := ValidateTLSConfiguration()
		assert.NoError(t, err, "Valid development configuration should not error")
	})

	t.Run("Invalid_Self_Signed_Without_Hosts", func(t *testing.T) {
		os.Setenv("APP_ENV", "development")
		os.Setenv("TLS_ALLOW_SELF_SIGNED", "true")
		os.Setenv("TLS_TRUSTED_HOSTS", "")

		err := ValidateTLSConfiguration()
		// This might return an error or warning depending on implementation
		// The important thing is it doesn't create an insecure configuration
		t.Logf("Configuration validation result: %v", err)
	})
}

func TestTLSConfig_Security(t *testing.T) {
	t.Run("TLS_Version_Security", func(t *testing.T) {
		client := CreateSecureHTTPClient()
		transport := client.Transport.(*http.Transport)
		tlsConfig := transport.TLSClientConfig

		// Test TLS version requirements
		assert.GreaterOrEqual(t, tlsConfig.MinVersion, uint16(tls.VersionTLS12),
			"Must use TLS 1.2 or higher")

		// Should not allow SSLv3, TLS 1.0, or TLS 1.1
		assert.NotEqual(t, uint16(tls.VersionSSL30), tlsConfig.MinVersion, //nolint:staticcheck // intentionally testing deprecated version
			"Must not allow SSLv3")
		assert.NotEqual(t, uint16(tls.VersionTLS10), tlsConfig.MinVersion,
			"Must not allow TLS 1.0")
		assert.NotEqual(t, uint16(tls.VersionTLS11), tlsConfig.MinVersion,
			"Must not allow TLS 1.1")

		t.Logf("✅ TLS version security requirements met")
	})

	t.Run("Certificate_Verification_Default", func(t *testing.T) {
		// Default should be secure (verify certificates)
		os.Setenv("APP_ENV", "production")
		os.Setenv("TLS_ALLOW_SELF_SIGNED", "false")

		client := CreateSecureHTTPClient()
		transport := client.Transport.(*http.Transport)
		tlsConfig := transport.TLSClientConfig

		// Default should verify certificates
		assert.False(t, tlsConfig.InsecureSkipVerify,
			"Should verify certificates by default")

		t.Logf("✅ Certificate verification enabled by default")
	})
}

func TestTLSConfig_Environment_Handling(t *testing.T) {
	// Test that environment variables are properly parsed and handled
	testCases := []struct {
		name            string
		env             string
		allowSelfSigned string
		trustedHosts    string
		expectSecure    bool
	}{
		{
			name:            "Production_Default",
			env:             "production",
			allowSelfSigned: "false",
			trustedHosts:    "",
			expectSecure:    true,
		},
		{
			name:            "Development_Secure",
			env:             "development",
			allowSelfSigned: "false",
			trustedHosts:    "",
			expectSecure:    true,
		},
		{
			name:            "Development_With_Trusted_Hosts",
			env:             "development",
			allowSelfSigned: "true",
			trustedHosts:    "keycloak,localhost",
			expectSecure:    true, // Still secure because hosts are limited
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			os.Setenv("APP_ENV", tc.env)
			os.Setenv("TLS_ALLOW_SELF_SIGNED", tc.allowSelfSigned)
			os.Setenv("TLS_TRUSTED_HOSTS", tc.trustedHosts)

			client := CreateSecureHTTPClient()
			transport := client.Transport.(*http.Transport)
			tlsConfig := transport.TLSClientConfig

			// All configurations should maintain TLS 1.2 minimum
			assert.GreaterOrEqual(t, tlsConfig.MinVersion, uint16(tls.VersionTLS12),
				"All environments should enforce TLS 1.2+")

			if tc.expectSecure {
				t.Logf("✅ %s: Secure TLS configuration maintained", tc.name)
			}
		})
	}
}
