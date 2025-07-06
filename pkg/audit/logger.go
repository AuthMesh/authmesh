package audit

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

// AuditEvent represents an auditable event in the system
type AuditEvent struct {
	Timestamp    time.Time              `json:"timestamp"`
	EventType    string                 `json:"event_type"`
	UserID       string                 `json:"user_id,omitempty"`
	Realm        string                 `json:"realm,omitempty"`
	Action       string                 `json:"action"`
	Resource     string                 `json:"resource,omitempty"`
	ResourceID   string                 `json:"resource_id,omitempty"`
	Status       string                 `json:"status"` // success, failure, error
	Details      map[string]interface{} `json:"details,omitempty"`
	IPAddress    string                 `json:"ip_address,omitempty"`
	UserAgent    string                 `json:"user_agent,omitempty"`
	RequestID    string                 `json:"request_id,omitempty"`
	ErrorMessage string                 `json:"error_message,omitempty"`
}

// Logger provides audit logging functionality
type Logger struct {
	logger *zap.Logger
}

// NewAuditLogger creates a new audit logger with file rotation
func NewAuditLogger() (*Logger, error) {
	// Get configuration from environment variables
	logPath := os.Getenv("AUDIT_LOG_PATH")
	if logPath == "" {
		logPath = "/tmp/audit.log"
	}

	// Parse max size (default 100MB)
	maxSizeStr := os.Getenv("AUDIT_LOG_MAX_SIZE")
	if maxSizeStr == "" {
		maxSizeStr = "100MB"
	}
	maxSize, err := parseSize(maxSizeStr)
	if err != nil {
		maxSize = 100 // Default to 100MB
	}

	// Parse max backups (default 10)
	maxBackupsStr := os.Getenv("AUDIT_LOG_MAX_BACKUPS")
	maxBackups := 10
	if maxBackupsStr != "" {
		if parsed, err := strconv.Atoi(maxBackupsStr); err == nil {
			maxBackups = parsed
		}
	}

	// Parse max age (default 30 days)
	maxAgeStr := os.Getenv("AUDIT_LOG_MAX_AGE")
	maxAge := 30
	if maxAgeStr != "" {
		if parsed, err := strconv.Atoi(maxAgeStr); err == nil {
			maxAge = parsed
		}
	}

	// Ensure log directory exists
	if err := os.MkdirAll(filepath.Dir(logPath), 0755); err != nil {
		return nil, fmt.Errorf("failed to create audit log directory: %w", err)
	}

	// Configure lumberjack for log rotation
	lumberjackLogger := &lumberjack.Logger{
		Filename:   logPath,
		MaxSize:    maxSize,
		MaxBackups: maxBackups,
		MaxAge:     maxAge,
		Compress:   true,
	}

	// Create a custom encoder for structured JSON logging
	encoderConfig := zapcore.EncoderConfig{
		TimeKey:        "timestamp",
		LevelKey:       "level",
		NameKey:        "logger",
		CallerKey:      "caller",
		MessageKey:     "message",
		StacktraceKey:  "stacktrace",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeLevel:    zapcore.LowercaseLevelEncoder,
		EncodeTime:     zapcore.ISO8601TimeEncoder,
		EncodeDuration: zapcore.SecondsDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
	}

	// Create JSON encoder
	encoder := zapcore.NewJSONEncoder(encoderConfig)

	// Create core with file writer
	core := zapcore.NewCore(encoder, zapcore.AddSync(lumberjackLogger), zapcore.InfoLevel)

	// Create logger
	logger := zap.New(core, zap.AddCaller())

	return &Logger{
		logger: logger,
	}, nil
}

// LogEvent logs an audit event
func (l *Logger) LogEvent(event AuditEvent) {
	// Set timestamp if not provided
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now().UTC()
	}

	// Convert event to JSON for structured logging
	eventJSON, err := json.Marshal(event)
	if err != nil {
		l.logger.Error("Failed to marshal audit event", zap.Error(err))
		return
	}

	// Log the event
	l.logger.Info("audit_event",
		zap.String("event_data", string(eventJSON)),
		zap.String("event_type", event.EventType),
		zap.String("action", event.Action),
		zap.String("status", event.Status),
		zap.String("user_id", event.UserID),
		zap.String("realm", event.Realm),
	)
}

// LogRealmCreation logs realm creation events
func (l *Logger) LogRealmCreation(userID, realm, realmID, status string, details map[string]interface{}, ipAddress, userAgent, requestID string) {
	event := AuditEvent{
		EventType:  "realm_management",
		UserID:     userID,
		Realm:      realm,
		Action:     "create_realm",
		Resource:   "realm",
		ResourceID: realmID,
		Status:     status,
		Details:    details,
		IPAddress:  ipAddress,
		UserAgent:  userAgent,
		RequestID:  requestID,
	}
	l.LogEvent(event)
}

// LogRealmUpdate logs realm update events
func (l *Logger) LogRealmUpdate(userID, realm, realmID, status string, details map[string]interface{}, ipAddress, userAgent, requestID string) {
	event := AuditEvent{
		EventType:  "realm_management",
		UserID:     userID,
		Realm:      realm,
		Action:     "update_realm",
		Resource:   "realm",
		ResourceID: realmID,
		Status:     status,
		Details:    details,
		IPAddress:  ipAddress,
		UserAgent:  userAgent,
		RequestID:  requestID,
	}
	l.LogEvent(event)
}

// LogRateLimitUpdate logs rate limit configuration changes
func (l *Logger) LogRateLimitUpdate(userID, realm, targetRealm, status string, oldLimit, newLimit float64, ipAddress, userAgent, requestID string) {
	details := map[string]interface{}{
		"target_realm": targetRealm,
		"old_limit":    oldLimit,
		"new_limit":    newLimit,
	}

	event := AuditEvent{
		EventType:  "rate_limit_management",
		UserID:     userID,
		Realm:      realm,
		Action:     "update_max_rate_limit",
		Resource:   "realm_rate_limit",
		ResourceID: targetRealm,
		Status:     status,
		Details:    details,
		IPAddress:  ipAddress,
		UserAgent:  userAgent,
		RequestID:  requestID,
	}
	l.LogEvent(event)
}

// LogError logs error events with additional context
func (l *Logger) LogError(userID, realm, action, resource, status, errorMessage, ipAddress, userAgent, requestID string) {
	event := AuditEvent{
		EventType:    "error",
		UserID:       userID,
		Realm:        realm,
		Action:       action,
		Resource:     resource,
		Status:       status,
		ErrorMessage: errorMessage,
		IPAddress:    ipAddress,
		UserAgent:    userAgent,
		RequestID:    requestID,
	}
	l.LogEvent(event)
}

// Close closes the audit logger
func (l *Logger) Close() error {
	return l.logger.Sync()
}

// parseSize parses size strings like "100MB", "1GB" into megabytes
func parseSize(sizeStr string) (int, error) {
	sizeStr = strings.ToUpper(strings.TrimSpace(sizeStr))

	if strings.HasSuffix(sizeStr, "MB") {
		sizeStr = strings.TrimSuffix(sizeStr, "MB")
		size, err := strconv.Atoi(sizeStr)
		if err != nil {
			return 0, err
		}
		return size, nil
	}

	if strings.HasSuffix(sizeStr, "GB") {
		sizeStr = strings.TrimSuffix(sizeStr, "GB")
		size, err := strconv.Atoi(sizeStr)
		if err != nil {
			return 0, err
		}
		return size * 1024, nil // Convert GB to MB
	}

	// Default to interpreting as MB
	size, err := strconv.Atoi(sizeStr)
	if err != nil {
		return 0, err
	}
	return size, nil
}
