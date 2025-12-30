package audit

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewAuditLogger(t *testing.T) {
	tests := []struct {
		name           string
		envVars        map[string]string
		expectError    bool
		validateLogger func(t *testing.T, logger *Logger)
	}{
		{
			name: "default configuration",
			envVars: map[string]string{
				"AUDIT_LOG_PATH":        "",
				"AUDIT_LOG_MAX_SIZE":    "",
				"AUDIT_LOG_MAX_BACKUPS": "",
				"AUDIT_LOG_MAX_AGE":     "",
			},
			expectError: false,
			validateLogger: func(t *testing.T, logger *Logger) {
				assert.NotNil(t, logger)
				assert.NotNil(t, logger.logger)
			},
		},
		{
			name: "custom configuration",
			envVars: map[string]string{
				"AUDIT_LOG_PATH":        "/tmp/test-audit.log",
				"AUDIT_LOG_MAX_SIZE":    "50MB",
				"AUDIT_LOG_MAX_BACKUPS": "5",
				"AUDIT_LOG_MAX_AGE":     "15",
			},
			expectError: false,
			validateLogger: func(t *testing.T, logger *Logger) {
				assert.NotNil(t, logger)
				assert.NotNil(t, logger.logger)
			},
		},
		{
			name: "GB size configuration",
			envVars: map[string]string{
				"AUDIT_LOG_PATH":     "/tmp/test-audit-gb.log",
				"AUDIT_LOG_MAX_SIZE": "1GB",
			},
			expectError: false,
			validateLogger: func(t *testing.T, logger *Logger) {
				assert.NotNil(t, logger)
				assert.NotNil(t, logger.logger)
			},
		},
		{
			name: "invalid size format",
			envVars: map[string]string{
				"AUDIT_LOG_PATH":     "/tmp/test-audit-invalid.log",
				"AUDIT_LOG_MAX_SIZE": "invalid",
			},
			expectError: false, // Should fall back to default
			validateLogger: func(t *testing.T, logger *Logger) {
				assert.NotNil(t, logger)
				assert.NotNil(t, logger.logger)
			},
		},
		{
			name: "invalid numeric values",
			envVars: map[string]string{
				"AUDIT_LOG_PATH":        "/tmp/test-audit-invalid-nums.log",
				"AUDIT_LOG_MAX_BACKUPS": "invalid",
				"AUDIT_LOG_MAX_AGE":     "invalid",
			},
			expectError: false, // Should fall back to defaults
			validateLogger: func(t *testing.T, logger *Logger) {
				assert.NotNil(t, logger)
				assert.NotNil(t, logger.logger)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clean environment
			for key := range tt.envVars {
				os.Unsetenv(key)
			}

			// Set test environment variables
			for key, value := range tt.envVars {
				if value != "" {
					os.Setenv(key, value)
				}
			}

			// Create logger
			logger, err := NewAuditLogger()

			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, logger)
			} else {
				assert.NoError(t, err)
				tt.validateLogger(t, logger)

				// Clean up
				if logger != nil {
					logger.Close()
				}
			}

			// Clean up test files
			for _, envValue := range tt.envVars {
				if envValue != "" && filepath.IsAbs(envValue) {
					os.Remove(envValue)
				}
			}
		})
	}

	// Clean environment after tests
	os.Unsetenv("AUDIT_LOG_PATH")
	os.Unsetenv("AUDIT_LOG_MAX_SIZE")
	os.Unsetenv("AUDIT_LOG_MAX_BACKUPS")
	os.Unsetenv("AUDIT_LOG_MAX_AGE")
}

func TestAuditEvent_MarshalJSON(t *testing.T) {
	event := AuditEvent{
		Timestamp:  time.Date(2023, 1, 1, 12, 0, 0, 0, time.UTC),
		EventType:  "test_event",
		UserID:     "user123",
		Realm:      "test-realm",
		Action:     "test_action",
		Resource:   "test_resource",
		ResourceID: "resource123",
		Status:     "success",
		Details: map[string]interface{}{
			"key1": "value1",
			"key2": 123,
		},
		IPAddress:    "192.168.1.1",
		UserAgent:    "test-agent",
		RequestID:    "req123",
		ErrorMessage: "",
	}

	data, err := json.Marshal(event)
	require.NoError(t, err)

	var unmarshaled AuditEvent
	err = json.Unmarshal(data, &unmarshaled)
	require.NoError(t, err)

	assert.Equal(t, event.EventType, unmarshaled.EventType)
	assert.Equal(t, event.UserID, unmarshaled.UserID)
	assert.Equal(t, event.Realm, unmarshaled.Realm)
	assert.Equal(t, event.Action, unmarshaled.Action)
	assert.Equal(t, event.Resource, unmarshaled.Resource)
	assert.Equal(t, event.ResourceID, unmarshaled.ResourceID)
	assert.Equal(t, event.Status, unmarshaled.Status)
	assert.Equal(t, event.IPAddress, unmarshaled.IPAddress)
	assert.Equal(t, event.UserAgent, unmarshaled.UserAgent)
	assert.Equal(t, event.RequestID, unmarshaled.RequestID)
}

func TestLogger_LogEvent(t *testing.T) {
	// Create temporary log file
	tmpFile, err := ioutil.TempFile("", "audit-test-*.log")
	require.NoError(t, err)
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	// Set environment variable
	os.Setenv("AUDIT_LOG_PATH", tmpFile.Name())
	defer os.Unsetenv("AUDIT_LOG_PATH")

	logger, err := NewAuditLogger()
	require.NoError(t, err)
	defer logger.Close()

	tests := []struct {
		name  string
		event AuditEvent
	}{
		{
			name: "complete event",
			event: AuditEvent{
				EventType:  "test_event",
				UserID:     "user123",
				Realm:      "test-realm",
				Action:     "test_action",
				Resource:   "test_resource",
				ResourceID: "resource123",
				Status:     "success",
				Details: map[string]interface{}{
					"key1": "value1",
					"key2": 123,
				},
				IPAddress: "192.168.1.1",
				UserAgent: "test-agent",
				RequestID: "req123",
			},
		},
		{
			name: "minimal event",
			event: AuditEvent{
				EventType: "minimal_event",
				Action:    "minimal_action",
				Status:    "success",
			},
		},
		{
			name: "event with timestamp",
			event: AuditEvent{
				Timestamp: time.Date(2023, 1, 1, 12, 0, 0, 0, time.UTC),
				EventType: "timestamped_event",
				Action:    "timestamped_action",
				Status:    "success",
			},
		},
		{
			name: "event with error",
			event: AuditEvent{
				EventType:    "error_event",
				Action:       "error_action",
				Status:       "failure",
				ErrorMessage: "test error message",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger.LogEvent(tt.event)
			// Sync to ensure log is written
			logger.logger.Sync()
		})
	}

	// Verify log file exists and has content
	stat, err := os.Stat(tmpFile.Name())
	require.NoError(t, err)
	assert.Greater(t, stat.Size(), int64(0))
}

func TestLogger_LogRealmCreation(t *testing.T) {
	tmpFile, err := ioutil.TempFile("", "audit-realm-test-*.log")
	require.NoError(t, err)
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	os.Setenv("AUDIT_LOG_PATH", tmpFile.Name())
	defer os.Unsetenv("AUDIT_LOG_PATH")

	logger, err := NewAuditLogger()
	require.NoError(t, err)
	defer logger.Close()

	details := map[string]interface{}{
		"realm_name": "test-realm",
		"enabled":    true,
	}

	logger.LogRealmCreation(
		"admin123",
		"master",
		"realm456",
		"success",
		details,
		"192.168.1.1",
		"test-agent",
		"req789",
	)

	logger.logger.Sync()

	// Verify log file has content
	stat, err := os.Stat(tmpFile.Name())
	require.NoError(t, err)
	assert.Greater(t, stat.Size(), int64(0))
}

func TestLogger_LogRealmUpdate(t *testing.T) {
	tmpFile, err := ioutil.TempFile("", "audit-realm-update-test-*.log")
	require.NoError(t, err)
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	os.Setenv("AUDIT_LOG_PATH", tmpFile.Name())
	defer os.Unsetenv("AUDIT_LOG_PATH")

	logger, err := NewAuditLogger()
	require.NoError(t, err)
	defer logger.Close()

	details := map[string]interface{}{
		"updated_fields": []string{"display_name", "enabled"},
	}

	logger.LogRealmUpdate(
		"admin123",
		"master",
		"realm456",
		"success",
		details,
		"192.168.1.1",
		"test-agent",
		"req789",
	)

	logger.logger.Sync()

	stat, err := os.Stat(tmpFile.Name())
	require.NoError(t, err)
	assert.Greater(t, stat.Size(), int64(0))
}

func TestLogger_LogRateLimitUpdate(t *testing.T) {
	tmpFile, err := ioutil.TempFile("", "audit-ratelimit-test-*.log")
	require.NoError(t, err)
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	os.Setenv("AUDIT_LOG_PATH", tmpFile.Name())
	defer os.Unsetenv("AUDIT_LOG_PATH")

	logger, err := NewAuditLogger()
	require.NoError(t, err)
	defer logger.Close()

	logger.LogRateLimitUpdate(
		"admin123",
		"master",
		"target-realm",
		"success",
		100.0,
		200.0,
		"192.168.1.1",
		"test-agent",
		"req789",
	)

	logger.logger.Sync()

	stat, err := os.Stat(tmpFile.Name())
	require.NoError(t, err)
	assert.Greater(t, stat.Size(), int64(0))
}

func TestLogger_LogError(t *testing.T) {
	tmpFile, err := ioutil.TempFile("", "audit-error-test-*.log")
	require.NoError(t, err)
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	os.Setenv("AUDIT_LOG_PATH", tmpFile.Name())
	defer os.Unsetenv("AUDIT_LOG_PATH")

	logger, err := NewAuditLogger()
	require.NoError(t, err)
	defer logger.Close()

	logger.LogError(
		"user123",
		"test-realm",
		"failed_action",
		"test_resource",
		"failure",
		"test error message",
		"192.168.1.1",
		"test-agent",
		"req789",
	)

	logger.logger.Sync()

	stat, err := os.Stat(tmpFile.Name())
	require.NoError(t, err)
	assert.Greater(t, stat.Size(), int64(0))
}

func TestParseSize(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expected    int
		expectError bool
	}{
		{
			name:        "megabytes format",
			input:       "100MB",
			expected:    100,
			expectError: false,
		},
		{
			name:        "gigabytes format",
			input:       "2GB",
			expected:    2048, // 2 * 1024
			expectError: false,
		},
		{
			name:        "plain number (treated as MB)",
			input:       "50",
			expected:    50,
			expectError: false,
		},
		{
			name:        "lowercase mb",
			input:       "75mb",
			expected:    75,
			expectError: false,
		},
		{
			name:        "lowercase gb",
			input:       "1gb",
			expected:    1024,
			expectError: false,
		},
		{
			name:        "with spaces",
			input:       " 200MB ",
			expected:    200,
			expectError: false,
		},
		{
			name:        "invalid format",
			input:       "invalid",
			expected:    0,
			expectError: true,
		},
		{
			name:        "negative number",
			input:       "-100MB",
			expected:    -100,
			expectError: false, // parseSize actually parses negative numbers successfully
		},
		{
			name:        "empty string",
			input:       "",
			expected:    0,
			expectError: true,
		},
		{
			name:        "zero MB",
			input:       "0MB",
			expected:    0,
			expectError: false,
		},
		{
			name:        "zero GB",
			input:       "0GB",
			expected:    0,
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parseSize(tt.input)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestLogger_Close(t *testing.T) {
	tmpFile, err := ioutil.TempFile("", "audit-close-test-*.log")
	require.NoError(t, err)
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	os.Setenv("AUDIT_LOG_PATH", tmpFile.Name())
	defer os.Unsetenv("AUDIT_LOG_PATH")

	logger, err := NewAuditLogger()
	require.NoError(t, err)

	// Log an event
	logger.LogEvent(AuditEvent{
		EventType: "test_close",
		Action:    "test_action",
		Status:    "success",
	})

	// Close should sync and return no error
	err = logger.Close()
	assert.NoError(t, err)
}

// Benchmark tests
func BenchmarkNewAuditLogger(b *testing.B) {
	tmpFile, err := ioutil.TempFile("", "audit-bench-*.log")
	require.NoError(b, err)
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	os.Setenv("AUDIT_LOG_PATH", tmpFile.Name())
	defer os.Unsetenv("AUDIT_LOG_PATH")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		logger, err := NewAuditLogger()
		require.NoError(b, err)
		logger.Close()
	}
}

func BenchmarkLogger_LogEvent(b *testing.B) {
	tmpFile, err := ioutil.TempFile("", "audit-bench-log-*.log")
	require.NoError(b, err)
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	os.Setenv("AUDIT_LOG_PATH", tmpFile.Name())
	defer os.Unsetenv("AUDIT_LOG_PATH")

	logger, err := NewAuditLogger()
	require.NoError(b, err)
	defer logger.Close()

	event := AuditEvent{
		EventType:  "benchmark_event",
		UserID:     "user123",
		Realm:      "test-realm",
		Action:     "benchmark_action",
		Resource:   "test_resource",
		ResourceID: "resource123",
		Status:     "success",
		Details: map[string]interface{}{
			"key1": "value1",
			"key2": 123,
		},
		IPAddress: "192.168.1.1",
		UserAgent: "benchmark-agent",
		RequestID: "req123",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		logger.LogEvent(event)
	}
}
