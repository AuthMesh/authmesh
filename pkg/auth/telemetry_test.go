package auth

import (
	"context"
	"testing"
)

func TestInitTelemetry(t *testing.T) {
	// InitTelemetry may fail without OTEL collector — that's expected
	err := InitTelemetry("test-service", "localhost:4318")
	// We don't assert NoError since the OTEL exporter may fail to connect
	// but it should not panic
	_ = err
}

func TestRecordAuthRequest_NilTelemetry(t *testing.T) {
	origTelemetry := GlobalTelemetry
	defer func() { GlobalTelemetry = origTelemetry }()

	GlobalTelemetry = nil
	// Should not panic when telemetry is nil
	var t2 *Telemetry
	t2.RecordAuthRequest(context.TODO(), "tenant", "user", "role", "success")
}

func TestExportTelemetryToJSON_Empty(t *testing.T) {
	origEvents := telemetryEvents
	defer func() { telemetryEvents = origEvents }()

	telemetryEvents = nil
	// Should return early without error for empty events
	exportTelemetryToJSON()
}
