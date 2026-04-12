package observability

import (
	"context"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/otel/attribute"
	oteltrace "go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"
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

	// Register metrics - use Register instead of MustRegister to handle duplicates
	if err := prometheus.Register(&m.RequestDuration); err != nil {
		// If already registered, that's fine - reuse the existing one
		if _, ok := err.(prometheus.AlreadyRegisteredError); !ok {
			panic(err)
		}
	}
	if err := prometheus.Register(&m.RequestsTotal); err != nil {
		if _, ok := err.(prometheus.AlreadyRegisteredError); !ok {
			panic(err)
		}
	}
	if err := prometheus.Register(&m.ActiveRequests); err != nil {
		if _, ok := err.(prometheus.AlreadyRegisteredError); !ok {
			panic(err)
		}
	}

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
	// Use Register instead of MustRegister to handle duplicates
	if err := prometheus.Register(info); err != nil {
		// If already registered, that's fine - reuse the existing one
		if _, ok := err.(prometheus.AlreadyRegisteredError); !ok {
			panic(err)
		}
	}
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
		statusCode := strconv.Itoa(c.Writer.Status())

		m.RequestDuration.WithLabelValues(c.Request.Method, path, statusCode).Observe(duration)
		m.RequestsTotal.WithLabelValues(c.Request.Method, path, statusCode).Inc()
		m.ActiveRequests.WithLabelValues(c.Request.Method, path).Dec()
	}
}

// SetupMetricsEndpoint sets up the /metrics endpoint for Prometheus
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

// StartSpan creates a new span using the tracing provider's tracer.
// Returns the child context and span. The caller must call span.End().
func (tp *TracingProvider) StartSpan(ctx context.Context, name string, opts ...oteltrace.SpanStartOption) (context.Context, oteltrace.Span) {
	tracer := noop.NewTracerProvider().Tracer(tp.config.ServiceName)
	return tracer.Start(ctx, name, opts...)
}

// SetAttribute is a helper to set an attribute on the current span in context.
func SetAttribute(ctx context.Context, key string, value interface{}) {
	span := oteltrace.SpanFromContext(ctx)
	if span == nil {
		return
	}
	switch v := value.(type) {
	case string:
		span.SetAttributes(attribute.String(key, v))
	case int:
		span.SetAttributes(attribute.Int(key, v))
	case bool:
		span.SetAttributes(attribute.Bool(key, v))
	case float64:
		span.SetAttributes(attribute.Float64(key, v))
	default:
		// Fallback: convert to string representation
	}
}

// AddEvent adds an event to the current span in context.
func AddEvent(ctx context.Context, name string, attrs ...attribute.KeyValue) {
	span := oteltrace.SpanFromContext(ctx)
	if span == nil {
		return
	}
	span.AddEvent(name, oteltrace.WithAttributes(attrs...))
}
