package auth

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"
	oteltrace "go.opentelemetry.io/otel/trace"
)

// Telemetry holds OpenTelemetry instruments
type Telemetry struct {
	AuthRequestsTotal metric.Int64Counter
	AuthFailuresTotal metric.Int64Counter
	Tracer            oteltrace.Tracer
}

// GlobalTelemetry is the global telemetry instance
var GlobalTelemetry *Telemetry

// TelemetryEvent represents a telemetry event for JSON export
type TelemetryEvent struct {
	Timestamp  time.Time         `json:"timestamp"`
	Event      string            `json:"event"`
	Attributes map[string]string `json:"attributes"`
}

// telemetryEvents stores events for JSON export
var telemetryEvents []TelemetryEvent

// InitTelemetry initializes OpenTelemetry
func InitTelemetry(serviceName, otelCollectorURL string) error {
	// Create resource
	res, err := resource.New(context.Background(),
		resource.WithAttributes(
			semconv.ServiceNameKey.String(serviceName),
			semconv.ServiceVersionKey.String("1.0.0"),
		),
	)
	if err != nil {
		return err
	}

	// Create OTLP HTTP exporter
	exporter, err := otlptracehttp.New(context.Background(),
		otlptracehttp.WithEndpoint(otelCollectorURL),
		otlptracehttp.WithInsecure(),
	)
	if err != nil {
		log.Printf("Warning: Failed to create OTLP exporter: %v", err)
		// Continue without exporter for development
	}

	// Create trace provider
	var tp *trace.TracerProvider
	if exporter != nil {
		tp = trace.NewTracerProvider(
			trace.WithBatcher(exporter),
			trace.WithResource(res),
		)
	} else {
		tp = trace.NewTracerProvider(
			trace.WithResource(res),
		)
	}
	otel.SetTracerProvider(tp)

	// Create meter provider
	mp := sdkmetric.NewMeterProvider(
		sdkmetric.WithResource(res),
	)
	otel.SetMeterProvider(mp)

	// Create meter and instruments
	meter := otel.Meter("auth")

	authRequestsTotal, err := meter.Int64Counter(
		"auth_requests_total",
		metric.WithDescription("Total number of authentication requests"),
		metric.WithUnit("count"),
	)
	if err != nil {
		return err
	}

	authFailuresTotal, err := meter.Int64Counter(
		"auth_failures_total",
		metric.WithDescription("Total number of authentication failures"),
		metric.WithUnit("count"),
	)
	if err != nil {
		return err
	}

	GlobalTelemetry = &Telemetry{
		AuthRequestsTotal: authRequestsTotal,
		AuthFailuresTotal: authFailuresTotal,
		Tracer:            otel.Tracer("auth"),
	}

	log.Println("OpenTelemetry initialized")
	return nil
}

// RecordAuthRequest records an authentication request
func (t *Telemetry) RecordAuthRequest(ctx context.Context, tenantID, userID, role, outcome string) {
	if t == nil {
		return
	}

	attrs := []attribute.KeyValue{
		attribute.String("auth.tenant_id", tenantID),
		attribute.String("auth.user_id", userID),
		attribute.String("auth.role", role),
		attribute.String("auth.outcome", outcome),
	}

	// Record metric
	t.AuthRequestsTotal.Add(ctx, 1, metric.WithAttributes(attrs...))

	// Record failure if applicable
	if outcome != "success" {
		t.AuthFailuresTotal.Add(ctx, 1, metric.WithAttributes(attrs...))
	}

	// Record event for JSON export
	event := TelemetryEvent{
		Timestamp: time.Now(),
		Event:     "auth_request",
		Attributes: map[string]string{
			"auth.tenant_id": tenantID,
			"auth.user_id":   userID,
			"auth.role":      role,
			"auth.outcome":   outcome,
		},
	}

	telemetryEvents = append(telemetryEvents, event)

	// Export to JSON file
	go exportTelemetryToJSON()
}

// exportTelemetryToJSON exports telemetry events to a telemetry file
func exportTelemetryToJSON() {
	if len(telemetryEvents) == 0 {
		return
	}

	telemetryPath := os.Getenv("TELEMETRY_FILE_PATH")
	if telemetryPath == "" {
		telemetryPath = filepath.Join(os.TempDir(), "telemetry.json")
	}

	file, err := os.OpenFile(telemetryPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		log.Printf("Error opening telemetry file: %v", err)
		return
	}
	defer func() { _ = file.Close() }()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")

	if err := encoder.Encode(map[string]interface{}{
		"service":   "platform-backend",
		"timestamp": time.Now().Format(time.RFC3339),
		"events":    telemetryEvents,
	}); err != nil {
		log.Printf("Error encoding telemetry: %v", err)
	}
}
