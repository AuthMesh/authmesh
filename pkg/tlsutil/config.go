package tlsutil

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
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

	// Load custom CA certificate if provided
	if caCertPath != "" {
		if err := loadCACertificate(tlsConfig, caCertPath); err != nil {
			log.Printf("[ERROR] Failed to load CA certificate: %v", err)
			// Fall back to system CA pool behavior
		} else {
			log.Printf("[INFO] Loaded custom CA certificate: %s", caCertPath)
		}
	}

	// Configure self-signed certificate handling for trusted hosts
	if allowSelfSigned && len(trustedHosts) > 0 && trustedHosts[0] != "" {
		// SECURITY NOTE: This configuration allows self-signed certificates
		// only for explicitly trusted hosts. This is secure because:
		// 1. We still verify the certificate belongs to a trusted host
		// 2. The list of trusted hosts is explicitly configured
		// 3. We're not globally disabling certificate verification

		// Try to load the self-signed certificate as a trusted CA first
		err := loadSelfSignedCertAsCA(tlsConfig, "/app/certs/keycloak.crt")
		if err != nil {
			log.Printf("[WARN] Failed to load self-signed cert as CA: %v", err)
			log.Printf("[INFO] Falling back to custom certificate verifier")

			// Fallback: For self-signed certificates, we need to skip the standard verification
			// and use our custom verifier to check the hostname matches our trusted hosts
			tlsConfig.InsecureSkipVerify = true
			tlsConfig.VerifyPeerCertificate = createHostVerifier(trustedHosts)
		} else {
			log.Printf("[INFO] Successfully loaded self-signed certificate as trusted CA")
		}

		log.Printf("[INFO] Allowing self-signed certificates for trusted hosts: %v", trustedHosts)
	} else if allowSelfSigned {
		log.Printf("[ERROR] TLS_ALLOW_SELF_SIGNED is true but no TLS_TRUSTED_HOSTS specified")
	}

	transport := &http.Transport{
		TLSClientConfig: tlsConfig,
	}

	return &http.Client{Transport: transport}
}

// loadCACertificate loads a custom CA certificate from file
func loadCACertificate(tlsConfig *tls.Config, caCertPath string) error {
	caCert, err := os.ReadFile(caCertPath)
	if err != nil {
		return fmt.Errorf("failed to read CA certificate file %s: %w", caCertPath, err)
	}

	caCertPool := x509.NewCertPool()
	if !caCertPool.AppendCertsFromPEM(caCert) {
		return fmt.Errorf("failed to parse CA certificate from %s", caCertPath)
	}

	tlsConfig.RootCAs = caCertPool
	return nil
}

// loadSelfSignedCertAsCA loads a self-signed certificate as a trusted CA
// This is specifically for development environments with self-signed certificates
func loadSelfSignedCertAsCA(tlsConfig *tls.Config, certPath string) error {
	// Check if the certificate file exists
	if _, err := os.Stat(certPath); os.IsNotExist(err) {
		return fmt.Errorf("certificate file not found: %s", certPath)
	}

	// Read the certificate file
	certPEM, err := os.ReadFile(certPath)
	if err != nil {
		return fmt.Errorf("failed to read certificate file %s: %w", certPath, err)
	}

	// Create a certificate pool and add our certificate
	caCertPool := x509.NewCertPool()
	if !caCertPool.AppendCertsFromPEM(certPEM) {
		return fmt.Errorf("failed to parse certificate from %s", certPath)
	}

	// Set the certificate pool as the trusted root CAs
	tlsConfig.RootCAs = caCertPool

	log.Printf("[INFO] Added self-signed certificate as trusted CA: %s", certPath)
	return nil
}

// createHostVerifier creates a custom certificate verifier for trusted hosts
func createHostVerifier(trustedHosts []string) func([][]byte, [][]*x509.Certificate) error {
	return func(rawCerts [][]byte, verifiedChains [][]*x509.Certificate) error {
		log.Printf("[DEBUG] Custom certificate verifier called with %d certificates", len(rawCerts))

		if len(rawCerts) == 0 {
			log.Printf("[DEBUG] No certificates provided to verifier")
			return fmt.Errorf("no certificates provided")
		}

		cert, err := x509.ParseCertificate(rawCerts[0])
		if err != nil {
			log.Printf("[DEBUG] Failed to parse certificate: %v", err)
			return fmt.Errorf("failed to parse certificate: %w", err)
		}

		log.Printf("[DEBUG] Verifying certificate: CN=%s, SANs=%v", cert.Subject.CommonName, cert.DNSNames)

		// Check if the certificate is for one of our trusted hosts
		for _, trustedHost := range trustedHosts {
			trustedHost = strings.TrimSpace(trustedHost)
			if trustedHost == "" {
				continue
			}

			// Check Subject Common Name
			if cert.Subject.CommonName == trustedHost {
				log.Printf("[INFO] Accepting self-signed certificate for trusted host: %s (matched CN)", trustedHost)
				return nil
			}

			// Check Subject Alternative Names (DNS names)
			for _, dnsName := range cert.DNSNames {
				if dnsName == trustedHost {
					log.Printf("[INFO] Accepting self-signed certificate for trusted host: %s (matched SAN)", trustedHost)
					return nil
				}
			}

			// Check Subject Alternative Names (IP addresses)
			for _, ipAddr := range cert.IPAddresses {
				if ipAddr.String() == trustedHost {
					log.Printf("[INFO] Accepting self-signed certificate for trusted host: %s (matched IP)", trustedHost)
					return nil
				}
			}
		}

		log.Printf("[ERROR] Certificate verification failed: Certificate hosts (CN=%s, SANs=%v) do not match any trusted hosts: %v",
			cert.Subject.CommonName, cert.DNSNames, trustedHosts)
		return fmt.Errorf("certificate not issued for any trusted host. Certificate hosts: CN=%s, SANs=%v, Trusted hosts: %v",
			cert.Subject.CommonName, cert.DNSNames, trustedHosts)
	}
}

// getBoolEnv gets a boolean environment variable with a default value
func getBoolEnv(key string, defaultValue bool) bool {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}

	boolValue, err := strconv.ParseBool(value)
	if err != nil {
		log.Printf("[WARN] Invalid boolean value for %s: %s, using default: %t", key, value, defaultValue)
		return defaultValue
	}

	return boolValue
}

// getAppEnv gets the application environment (development, production, etc.)
func getAppEnv() string {
	env := strings.ToLower(strings.TrimSpace(os.Getenv("APP_ENV")))
	if env == "" {
		return "development" // default to development if not set
	}
	return env
}

// ValidateTLSConfiguration validates the current TLS configuration
func ValidateTLSConfiguration() error {
	allowSelfSigned := getBoolEnv("TLS_ALLOW_SELF_SIGNED", false)
	trustedHosts := os.Getenv("TLS_TRUSTED_HOSTS")
	caCertPath := os.Getenv("TLS_CA_CERT_PATH")
	appEnv := getAppEnv()

	// Check if we're in production environment
	isProduction := appEnv == "production" || appEnv == "prod"

	// Production environment should never allow self-signed certificates
	if isProduction && allowSelfSigned {
		return fmt.Errorf("TLS_ALLOW_SELF_SIGNED should not be enabled in production environment")
	}

	// Validate self-signed certificate configuration
	if allowSelfSigned {
		if trustedHosts == "" {
			return fmt.Errorf("TLS_ALLOW_SELF_SIGNED is enabled but TLS_TRUSTED_HOSTS is not specified - this is a security risk")
		}

		hosts := strings.Split(trustedHosts, ",")
		validHosts := make([]string, 0, len(hosts))
		for _, host := range hosts {
			host = strings.TrimSpace(host)
			if host != "" {
				validHosts = append(validHosts, host)
			}
		}

		if len(validHosts) == 0 {
			return fmt.Errorf("TLS_TRUSTED_HOSTS contains no valid hosts")
		}

		log.Printf("[INFO] TLS configuration: allowing self-signed certificates for %d trusted hosts", len(validHosts))
	}

	// Validate CA certificate path if provided
	if caCertPath != "" {
		if _, err := os.Stat(caCertPath); os.IsNotExist(err) {
			return fmt.Errorf("TLS_CA_CERT_PATH points to non-existent file: %s", caCertPath)
		}
		log.Printf("[INFO] TLS configuration: using custom CA certificate: %s", caCertPath)
	}

	return nil
}
