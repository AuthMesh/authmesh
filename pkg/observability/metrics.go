package observability

import (
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Metrics holds all the Prometheus metrics
type Metrics struct {
	RequestsTotal    *prometheus.CounterVec
	RequestDuration  *prometheus.HistogramVec
	ActiveRequests   *prometheus.GaugeVec
	ResponseSize     *prometheus.HistogramVec
	AuthAttempts     *prometheus.CounterVec
	TokenValidations *prometheus.CounterVec
	RateLimitHits    *prometheus.CounterVec
}

// NewMetrics creates and registers Prometheus metrics
func NewMetrics() *Metrics {
	m := &Metrics{
		RequestsTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "http_requests_total",
				Help: "Total number of HTTP requests",
			},
			[]string{"method", "path", "status", "tenant_id"},
		),
		RequestDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "http_request_duration_seconds",
				Help:    "HTTP request duration in seconds",
				Buckets: prometheus.DefBuckets,
			},
			[]string{"method", "path", "status", "tenant_id"},
		),
		ActiveRequests: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "http_requests_active",
				Help: "Number of active HTTP requests",
			},
			[]string{"method", "path", "tenant_id"},
		),
		ResponseSize: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "http_response_size_bytes",
				Help:    "HTTP response size in bytes",
				Buckets: []float64{100, 1000, 10000, 100000, 1000000},
			},
			[]string{"method", "path", "status", "tenant_id"},
		),
		AuthAttempts: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "auth_attempts_total",
				Help: "Total number of authentication attempts",
			},
			[]string{"result", "tenant_id", "realm"},
		),
		TokenValidations: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "token_validations_total",
				Help: "Total number of token validations",
			},
			[]string{"result", "tenant_id", "realm"},
		),
		RateLimitHits: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "rate_limit_hits_total",
				Help: "Total number of rate limit hits",
			},
			[]string{"type", "tenant_id"},
		),
	}

	// Register metrics safely (ignore errors if already registered)
	prometheus.Register(m.RequestsTotal)
	prometheus.Register(m.RequestDuration)
	prometheus.Register(m.ActiveRequests)
	prometheus.Register(m.ResponseSize)
	prometheus.Register(m.AuthAttempts)
	prometheus.Register(m.TokenValidations)
	prometheus.Register(m.RateLimitHits)

	return m
}

// Global metrics instance
var defaultMetrics = NewMetrics()

// GetDefaultMetrics returns the default metrics instance
func GetDefaultMetrics() *Metrics {
	return defaultMetrics
}

// MetricsMiddleware creates a middleware that records HTTP metrics
func MetricsMiddleware(metrics *Metrics) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		
		// Get tenant ID for metrics labeling
		tenantID := c.GetString("tenant_id")
		if tenantID == "" {
			tenantID = "unknown"
		}

		// Track active requests
		metrics.ActiveRequests.WithLabelValues(
			c.Request.Method,
			c.FullPath(),
			tenantID,
		).Inc()

		c.Next()

		// Record metrics after request completion
		duration := time.Since(start).Seconds()
		status := strconv.Itoa(c.Writer.Status())
		
		labels := []string{c.Request.Method, c.FullPath(), status, tenantID}
		
		metrics.RequestsTotal.WithLabelValues(labels...).Inc()
		metrics.RequestDuration.WithLabelValues(labels...).Observe(duration)
		metrics.ResponseSize.WithLabelValues(labels...).Observe(float64(c.Writer.Size()))
		
		// Decrement active requests
		metrics.ActiveRequests.WithLabelValues(
			c.Request.Method,
			c.FullPath(),
			tenantID,
		).Dec()
	}
}

// DefaultMetricsMiddleware creates a middleware using the default metrics instance
func DefaultMetricsMiddleware() gin.HandlerFunc {
	return MetricsMiddleware(defaultMetrics)
}

// RecordAuthAttempt records an authentication attempt
func (m *Metrics) RecordAuthAttempt(result, tenantID, realm string) {
	m.AuthAttempts.WithLabelValues(result, tenantID, realm).Inc()
}

// RecordTokenValidation records a token validation
func (m *Metrics) RecordTokenValidation(result, tenantID, realm string) {
	m.TokenValidations.WithLabelValues(result, tenantID, realm).Inc()
}

// RecordRateLimitHit records a rate limit hit
func (m *Metrics) RecordRateLimitHit(limitType, tenantID string) {
	m.RateLimitHits.WithLabelValues(limitType, tenantID).Inc()
}

// MetricsHandler returns the Prometheus metrics handler
func MetricsHandler() gin.HandlerFunc {
	h := promhttp.Handler()
	return gin.WrapH(h)
}

// SetupMetricsEndpoint adds the /metrics endpoint to a Gin router
func SetupMetricsEndpoint(router *gin.Engine) {
	router.GET("/metrics", MetricsHandler())
}

// HealthMetrics provides health check metrics
type HealthMetrics struct {
	AppInfo *prometheus.GaugeVec
	Uptime  prometheus.Gauge
}

// NewHealthMetrics creates health-related metrics
func NewHealthMetrics(appName, version string) *HealthMetrics {
	hm := &HealthMetrics{
		AppInfo: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "app_info",
				Help: "Application information",
			},
			[]string{"name", "version"},
		),
		Uptime: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "app_uptime_seconds",
				Help: "Application uptime in seconds",
			},
		),
	}

	// Set app info
	hm.AppInfo.WithLabelValues(appName, version).Set(1)

	// Register metrics safely
	prometheus.Register(hm.AppInfo)
	prometheus.Register(hm.Uptime)

	// Start uptime tracking
	startTime := time.Now()
	go func() {
		for {
			hm.Uptime.Set(time.Since(startTime).Seconds())
			time.Sleep(time.Second)
		}
	}()

	return hm
}
