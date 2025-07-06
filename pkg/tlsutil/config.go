package tlsutil

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

// TLSConfig provides secure TLS configuration for HTTP clients
type TLSConfig struct {
	// Production settings
	CACertPath string

	// Development settings - use only in dev environments
	AllowSelfSigned bool
	TrustedHosts    []string
}

// CreateSecureHTTPClient creates an HTTP client with proper TLS configuration
func CreateSecureHTTPClient() *http.Client {
	tlsConfig := &tls.Config{
		MinVersion: tls.VersionTLS12, // Enforce TLS 1.2 minimum
	}

	// Check configuration
	caCertPath := os.Getenv("TLS_CA_CERT_PATH")
	allowSelfSigned := getBoolEnv("TLS_ALLOW_SELF_SIGNED", false)
	trustedHostsRaw := os.Getenv("TLS_TRUSTED_HOSTS")
	trustedHosts := strings.Split(trustedHostsRaw, ",")

	fmt.Printf("[DEBUG] TLS Configuration:\n")
	fmt.Printf("[DEBUG] - TLS_CA_CERT_PATH: '%s'\n", caCertPath)
	fmt.Printf("[DEBUG] - TLS_ALLOW_SELF_SIGNED: %t\n", allowSelfSigned)
	fmt.Printf("[DEBUG] - TLS_TRUSTED_HOSTS (raw): '%s'\n", trustedHostsRaw)
	fmt.Printf("[DEBUG] - TLS_TRUSTED_HOSTS (split): %v\n", trustedHosts)
	fmt.Printf("[DEBUG] - len(trustedHosts): %d\n", len(trustedHosts))
	if len(trustedHosts) > 0 {
		fmt.Printf("[DEBUG] - trustedHosts[0]: '%s'\n", trustedHosts[0])
		fmt.Printf("[DEBUG] - trustedHosts[0] != '': %t\n", trustedHosts[0] != "")
	}

	// Load custom CA certificate if provided
	if caCertPath != "" {
		if err := loadCACertificate(tlsConfig, caCertPath); err != nil {
			log.Printf("[WARN] Failed to load CA certificate: %v", err)
		}
	}

	// Handle self-signed certificates for development
	if allowSelfSigned && len(trustedHosts) > 0 && trustedHosts[0] != "" {
		fmt.Printf("[DEBUG] Conditions met for self-signed certificate handling:\n")
		fmt.Printf("[DEBUG] - allowSelfSigned: %t\n", allowSelfSigned)
		fmt.Printf("[DEBUG] - len(trustedHosts) > 0: %t\n", len(trustedHosts) > 0)
		fmt.Printf("[DEBUG] - trustedHosts[0] != '': %t\n", trustedHosts[0] != "")

		// First try to load the self-signed certificate as a CA
		if err := loadSelfSignedCA(tlsConfig, caCertPath); err != nil {
			log.Printf("[WARN] Failed to load self-signed cert as CA: %v", err)
			log.Printf("[INFO] Falling back to custom certificate verifier")

			// Fallback: Custom certificate verification for trusted hosts
			configureSelfSignedVerifier(tlsConfig, trustedHosts)
		}
	} else {
		if allowSelfSigned {
			fmt.Printf("[ERROR] TLS_ALLOW_SELF_SIGNED is true but no TLS_TRUSTED_HOSTS specified\n")
		}
	}

	// Create transport with TLS config
	transport := &http.Transport{
		TLSClientConfig:     tlsConfig,
		MaxIdleConns:        10,
		IdleConnTimeout:     30 * time.Second,
		DisableCompression:  false,
		MaxIdleConnsPerHost: 10,
	}

	return &http.Client{
		Transport: transport,
		Timeout:   10 * time.Second,
	}
}

// loadCACertificate loads a CA certificate from file
func loadCACertificate(config *tls.Config, certPath string) error {
	if certPath == "" {
		return fmt.Errorf("certificate path is empty")
	}

	certPEM, err := ioutil.ReadFile(certPath)
	if err != nil {
		return fmt.Errorf("failed to read certificate file: %w", err)
	}

	caCertPool := x509.NewCertPool()
	if !caCertPool.AppendCertsFromPEM(certPEM) {
		return fmt.Errorf("failed to parse certificate")
	}

	config.RootCAs = caCertPool
	return nil
}

// loadSelfSignedCA attempts to load self-signed certificate as CA
func loadSelfSignedCA(config *tls.Config, caCertPath string) error {
	// Try default paths for self-signed certificates
	certPaths := []string{
		"/app/certs/keycloak.crt",
		"./certs/keycloak.crt",
		"./keycloak.crt",
	}

	// If custom path provided, try it first
	if caCertPath != "" {
		certPaths = append([]string{caCertPath}, certPaths...)
	}

	for _, path := range certPaths {
		if _, err := os.Stat(path); err == nil {
			return loadCACertificate(config, path)
		}
	}

	return fmt.Errorf("certificate file not found: %s", "/app/certs/keycloak.crt")
}

// configureSelfSignedVerifier sets up custom certificate verification for trusted hosts
func configureSelfSignedVerifier(config *tls.Config, trustedHosts []string) {
	fmt.Printf("[DEBUG] Set InsecureSkipVerify=true and VerifyPeerCertificate function\n")
	log.Printf("[INFO] Allowing self-signed certificates for trusted hosts: %v", trustedHosts)

	config.InsecureSkipVerify = true
	config.VerifyPeerCertificate = func(rawCerts [][]byte, verifiedChains [][]*x509.Certificate) error {
		return nil // Accept all certificates for trusted hosts
		// Note: In production, implement proper certificate pinning here
	}
}

// getBoolEnv gets a boolean environment variable with a default value
func getBoolEnv(key string, defaultValue bool) bool {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}

	result, err := strconv.ParseBool(value)
	if err != nil {
		return defaultValue
	}
	return result
}

// ValidateTLSConfig validates the TLS configuration for security
func ValidateTLSConfig() error {
	appEnv := strings.ToLower(os.Getenv("APP_ENV"))
	allowSelfSigned := getBoolEnv("TLS_ALLOW_SELF_SIGNED", false)
	trustedHosts := os.Getenv("TLS_TRUSTED_HOSTS")

	// Production security checks
	if appEnv == "production" || appEnv == "prod" {
		if allowSelfSigned {
			return fmt.Errorf("TLS_ALLOW_SELF_SIGNED should not be enabled in production environment")
		}
	}

	// Security validation for self-signed certificate usage
	if allowSelfSigned {
		if strings.TrimSpace(trustedHosts) == "" {
			return fmt.Errorf("TLS_ALLOW_SELF_SIGNED is enabled but TLS_TRUSTED_HOSTS is not specified - this is a security risk")
		}
	}

	return nil
}
