package observability

import (
	"context"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/jaeger"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.20.0"
	oteltrace "go.opentelemetry.io/otel/trace"
)

// TracingConfig holds the configuration for tracing
type TracingConfig struct {
	ServiceName    string
	ServiceVersion string
	Environment    string
	JaegerURL      string
	OTLPEndpoint   string
	SampleRate     float64
	Enabled        bool
}

// TracingProvider manages OpenTelemetry tracing
type TracingProvider struct {
	tracer   oteltrace.Tracer
	provider *trace.TracerProvider
	config   TracingConfig
}

// NewTracingProvider creates a new tracing provider
func NewTracingProvider(config TracingConfig) (*TracingProvider, error) {
	if !config.Enabled {
		return &TracingProvider{
			tracer: otel.Tracer("noop"),
			config: config,
		}, nil
	}

	// Create resource
	res, err := resource.New(context.Background(),
		resource.WithAttributes(
			semconv.ServiceName(config.ServiceName),
			semconv.ServiceVersion(config.ServiceVersion),
			semconv.DeploymentEnvironment(config.Environment),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource: %w", err)
	}

	// Create exporter
	var exporter trace.SpanExporter
	if config.OTLPEndpoint != "" {
		exporter, err = otlptracehttp.New(context.Background(),
			otlptracehttp.WithEndpoint(config.OTLPEndpoint),
		)
		if err != nil {
			return nil, fmt.Errorf("failed to create OTLP exporter: %w", err)
		}
	} else if config.JaegerURL != "" {
		exporter, err = jaeger.New(jaeger.WithCollectorEndpoint(jaeger.WithEndpoint(config.JaegerURL)))
		if err != nil {
			return nil, fmt.Errorf("failed to create Jaeger exporter: %w", err)
		}
	} else {
		return nil, fmt.Errorf("no tracing endpoint configured")
	}

	// Create tracer provider
	tp := trace.NewTracerProvider(
		trace.WithBatcher(exporter),
		trace.WithResource(res),
		trace.WithSampler(trace.TraceIDRatioBased(config.SampleRate)),
	)

	// Set global tracer provider
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	tracer := tp.Tracer(config.ServiceName)

	return &TracingProvider{
		tracer:   tracer,
		provider: tp,
		config:   config,
	}, nil
}

// Shutdown gracefully shuts down the tracing provider
func (tp *TracingProvider) Shutdown(ctx context.Context) error {
	if tp.provider != nil {
		return tp.provider.Shutdown(ctx)
	}
	return nil
}

// TracingMiddleware creates a Gin middleware for tracing HTTP requests
func (tp *TracingProvider) TracingMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !tp.config.Enabled {
			c.Next()
			return
		}

		// Extract context from headers
		ctx := otel.GetTextMapPropagator().Extract(c.Request.Context(), propagation.HeaderCarrier(c.Request.Header))

		// Start span
		spanName := fmt.Sprintf("%s %s", c.Request.Method, c.FullPath())
		if spanName == " " {
			spanName = fmt.Sprintf("%s %s", c.Request.Method, c.Request.URL.Path)
		}

		ctx, span := tp.tracer.Start(ctx, spanName,
			oteltrace.WithAttributes(
				semconv.HTTPMethod(c.Request.Method),
				semconv.HTTPTarget(c.Request.URL.Path),
				semconv.HTTPRoute(c.FullPath()),
				semconv.HTTPScheme(c.Request.URL.Scheme),
				attribute.String("http.host", c.Request.Host),
				attribute.String("http.user_agent", c.Request.UserAgent()),
				semconv.HTTPRequestContentLength(int(c.Request.ContentLength)),
			),
			oteltrace.WithSpanKind(oteltrace.SpanKindServer),
		)
		defer span.End()

		// Add span to context
		c.Request = c.Request.WithContext(ctx)

		// Add tenant information if available
		if tenantID := c.GetString("tenant_id"); tenantID != "" {
			span.SetAttributes(attribute.String("tenant.id", tenantID))
		}

		// Add user information if available
		if userID := c.GetString("user_id"); userID != "" {
			span.SetAttributes(attribute.String("user.id", userID))
		}

		// Process request
		c.Next()

		// Set response attributes
		span.SetAttributes(
			semconv.HTTPStatusCode(c.Writer.Status()),
			semconv.HTTPResponseContentLength(c.Writer.Size()),
		)

		// Set span status based on HTTP status
		if c.Writer.Status() >= 400 {
			span.SetAttributes(attribute.Bool("error", true))
			if len(c.Errors) > 0 {
				span.SetAttributes(attribute.String("error.message", c.Errors.Last().Error()))
			}
		}
	}
}

// StartSpan starts a new span with the given name
func (tp *TracingProvider) StartSpan(ctx context.Context, name string, opts ...oteltrace.SpanStartOption) (context.Context, oteltrace.Span) {
	if !tp.config.Enabled {
		return ctx, oteltrace.SpanFromContext(ctx)
	}
	return tp.tracer.Start(ctx, name, opts...)
}

// SpanFromContext returns the current span from context
func SpanFromContext(ctx context.Context) oteltrace.Span {
	return oteltrace.SpanFromContext(ctx)
}

// AddEvent adds an event to the current span
func AddEvent(ctx context.Context, name string, attrs ...attribute.KeyValue) {
	span := oteltrace.SpanFromContext(ctx)
	span.AddEvent(name, oteltrace.WithAttributes(attrs...))
}

// SetAttribute sets an attribute on the current span
func SetAttribute(ctx context.Context, key string, value interface{}) {
	span := oteltrace.SpanFromContext(ctx)
	span.SetAttributes(attribute.String(key, fmt.Sprintf("%v", value)))
}

// RecordError records an error on the current span
func RecordError(ctx context.Context, err error, description string) {
	span := oteltrace.SpanFromContext(ctx)
	span.RecordError(err, oteltrace.WithAttributes(attribute.String("error.description", description)))
	span.SetAttributes(attribute.Bool("error", true))
}

// InjectHeaders injects tracing headers into an HTTP request
func InjectHeaders(ctx context.Context, req *http.Request) {
	otel.GetTextMapPropagator().Inject(ctx, propagation.HeaderCarrier(req.Header))
}
