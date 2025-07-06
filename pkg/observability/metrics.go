package observability

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Metrics holds all Prometheus metrics
type Metrics struct {
	RequestDuration prometheus.HistogramVec
	RequestsTotal   prometheus.CounterVec
	ActiveRequests  prometheus.GaugeVec
}

// NewMetrics creates a new Metrics instance
func NewMetrics() *Metrics {
	m := &Metrics{
		RequestDuration: *prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name: "http_request_duration_seconds",
				Help: "Duration of HTTP requests in seconds",
			},
			[]string{"method", "path", "status_code"},
		),
		RequestsTotal: *prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "http_requests_total",
				Help: "Total number of HTTP requests",
			},
			[]string{"method", "path", "status_code"},
		),
		ActiveRequests: *prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "http_active_requests",
				Help: "Number of active HTTP requests",
			},
			[]string{"method", "path"},
		),
	}

	// Register metrics
	prometheus.MustRegister(&m.RequestDuration)
	prometheus.MustRegister(&m.RequestsTotal)
	prometheus.MustRegister(&m.ActiveRequests)

	return m
}

// NewHealthMetrics creates health-related metrics
func NewHealthMetrics(appName, appVersion string) {
	info := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "app_info",
			Help: "Application information",
		},
		[]string{"app_name", "version"},
	)
	
	info.WithLabelValues(appName, appVersion).Set(1)
	prometheus.MustRegister(info)
}

// MetricsMiddleware returns a middleware that records metrics
func MetricsMiddleware(m *Metrics) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.FullPath()
		if path == "" {
			path = c.Request.URL.Path
		}

		// Increment active requests
		m.ActiveRequests.WithLabelValues(c.Request.Method, path).Inc()

		// Process request
		c.Next()

		// Record metrics
		duration := time.Since(start).Seconds()
		statusCode := string(rune(c.Writer.Status()))

		m.RequestDuration.WithLabelValues(c.Request.Method, path, statusCode).Observe(duration)
		m.RequestsTotal.WithLabelValues(c.Request.Method, path, statusCode).Inc()
		m.ActiveRequests.WithLabelValues(c.Request.Method, path).Dec()
	}
}

// SetupMetricsEndpoint sets up the /metrics endpoint
func SetupMetricsEndpoint(router *gin.Engine) {
	router.GET("/metrics", gin.WrapH(promhttp.Handler()))
}

// TracingConfig represents tracing configuration
type TracingConfig struct {
	ServiceName    string  `json:"service_name"`
	ServiceVersion string  `json:"service_version"`
	Environment    string  `json:"environment"`
	JaegerURL      string  `json:"jaeger_url"`
	OTLPEndpoint   string  `json:"otlp_endpoint"`
	SampleRate     float64 `json:"sample_rate"`
	Enabled        bool    `json:"enabled"`
}

// TracingProvider represents a tracing provider
type TracingProvider struct {
	config TracingConfig
}

// NewTracingProvider creates a new tracing provider
func NewTracingProvider(config TracingConfig) (*TracingProvider, error) {
	return &TracingProvider{
		config: config,
	}, nil
}

// TracingMiddleware returns a tracing middleware
func (tp *TracingProvider) TracingMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// For now, just pass through - can be enhanced later
		c.Next()
	}
}

// Shutdown shuts down the tracing provider
func (tp *TracingProvider) Shutdown(ctx interface{}) error {
	// No-op for now
	return nil
}
