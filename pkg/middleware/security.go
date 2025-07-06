package middleware

import (
	"net"
	"net/http"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"
)

// SecurityConfig represents security middleware configuration
type SecurityConfig struct {
	EnableSSRFProtection    bool     `json:"enable_ssrf_protection"`
	AllowedHosts           []string `json:"allowed_hosts"`
	BlockPrivateIPs        bool     `json:"block_private_ips"`
	EnableSecurityHeaders  bool     `json:"enable_security_headers"`
}

// DefaultSecurityConfig returns a secure default configuration
func DefaultSecurityConfig() SecurityConfig {
	return SecurityConfig{
		EnableSSRFProtection:   true,
		AllowedHosts:          []string{},
		BlockPrivateIPs:       true,
		EnableSecurityHeaders: true,
	}
}

// SecurityHeadersMiddleware adds security headers to responses
func SecurityHeadersMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Prevent MIME type sniffing
		c.Header("X-Content-Type-Options", "nosniff")
		
		// Prevent clickjacking attacks
		c.Header("X-Frame-Options", "DENY")
		
		// Enable XSS protection
		c.Header("X-XSS-Protection", "1; mode=block")
		
		// Referrer policy
		c.Header("Referrer-Policy", "strict-origin-when-cross-origin")
		
		// Content Security Policy
		c.Header("Content-Security-Policy", "default-src 'self'; script-src 'self' 'unsafe-inline'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; font-src 'self'; connect-src 'self'; frame-ancestors 'none';")
		
		// Permissions policy
		c.Header("Permissions-Policy", "geolocation=(), microphone=(), camera=()")
		
		// Strict Transport Security (only for HTTPS)
		if c.Request.TLS != nil {
			c.Header("Strict-Transport-Security", "max-age=31536000; includeSubDomains; preload")
		}

		c.Next()
	}
}

// SSRFProtectionMiddleware protects against Server-Side Request Forgery attacks
func SSRFProtectionMiddleware(config SecurityConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !config.EnableSSRFProtection {
			c.Next()
			return
		}

		// Check various parameters that might contain URLs
		urlParams := []string{"url", "redirect", "callback", "next", "return_url", "target"}
		
		for _, param := range urlParams {
			if urlValue := c.Query(param); urlValue != "" {
				if !isURLSafe(urlValue, config) {
					c.JSON(http.StatusBadRequest, gin.H{
						"error": "Invalid URL parameter",
						"param": param,
					})
					c.Abort()
					return
				}
			}
			
			// Also check form parameters
			if urlValue := c.PostForm(param); urlValue != "" {
				if !isURLSafe(urlValue, config) {
					c.JSON(http.StatusBadRequest, gin.H{
						"error": "Invalid URL parameter",
						"param": param,
					})
					c.Abort()
					return
				}
			}
		}

		c.Next()
	}
}

// isURLSafe checks if a URL is safe to access (prevents SSRF)
func isURLSafe(urlStr string, config SecurityConfig) bool {
	// Parse the URL
	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return false
	}

	// Block non-HTTP/HTTPS schemes
	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return false
	}

	// Extract hostname/IP
	host := parsedURL.Hostname()
	if host == "" {
		return false
	}

	// Check against allowed hosts if specified
	if len(config.AllowedHosts) > 0 {
		allowed := false
		for _, allowedHost := range config.AllowedHosts {
			if strings.EqualFold(host, allowedHost) {
				allowed = true
				break
			}
		}
		if !allowed {
			return false
		}
	}

	// Block private IPs if enabled
	if config.BlockPrivateIPs {
		if ip := net.ParseIP(host); ip != nil {
			if isPrivateIP(ip) {
				return false
			}
		}
	}

	return true
}

// isPrivateIP checks if an IP address is in a private range
func isPrivateIP(ip net.IP) bool {
	// IPv4 private ranges
	private4 := []net.IPNet{
		{IP: net.IPv4(10, 0, 0, 0), Mask: net.CIDRMask(8, 32)},     // 10.0.0.0/8
		{IP: net.IPv4(172, 16, 0, 0), Mask: net.CIDRMask(12, 32)},  // 172.16.0.0/12
		{IP: net.IPv4(192, 168, 0, 0), Mask: net.CIDRMask(16, 32)}, // 192.168.0.0/16
		{IP: net.IPv4(127, 0, 0, 0), Mask: net.CIDRMask(8, 32)},    // 127.0.0.0/8 (loopback)
		{IP: net.IPv4(169, 254, 0, 0), Mask: net.CIDRMask(16, 32)}, // 169.254.0.0/16 (link-local)
	}

	// IPv6 private ranges
	private6 := []net.IPNet{
		{IP: net.ParseIP("::1"), Mask: net.CIDRMask(128, 128)},    // ::1/128 (loopback)
		{IP: net.ParseIP("fc00::"), Mask: net.CIDRMask(7, 128)},   // fc00::/7 (unique local)
		{IP: net.ParseIP("fe80::"), Mask: net.CIDRMask(10, 128)},  // fe80::/10 (link-local)
	}

	// Check IPv4
	if ip.To4() != nil {
		for _, network := range private4 {
			if network.Contains(ip) {
				return true
			}
		}
	} else {
		// Check IPv6
		for _, network := range private6 {
			if network.Contains(ip) {
				return true
			}
		}
	}

	return false
}

// SecurityMiddleware combines all security middleware
func SecurityMiddleware(config SecurityConfig) gin.HandlerFunc {
	return gin.HandlerFunc(func(c *gin.Context) {
		// Apply security headers
		if config.EnableSecurityHeaders {
			SecurityHeadersMiddleware()(c)
		}
		
		// Apply SSRF protection
		if config.EnableSSRFProtection {
			SSRFProtectionMiddleware(config)(c)
		}
		
		if c.IsAborted() {
			return
		}
		
		c.Next()
	})
}

// TrustedProxyMiddleware ensures requests come from trusted proxies
func TrustedProxyMiddleware(trustedProxies []string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if len(trustedProxies) == 0 {
			c.Next()
			return
		}

		clientIP := c.ClientIP()
		trusted := false

		for _, proxy := range trustedProxies {
			if clientIP == proxy {
				trusted = true
				break
			}
			
			// Support CIDR notation
			if _, network, err := net.ParseCIDR(proxy); err == nil {
				if ip := net.ParseIP(clientIP); ip != nil && network.Contains(ip) {
					trusted = true
					break
				}
			}
		}

		if !trusted {
			c.JSON(http.StatusForbidden, gin.H{
				"error": "Access denied: untrusted proxy",
			})
			c.Abort()
			return
		}

		c.Next()
	}
}
