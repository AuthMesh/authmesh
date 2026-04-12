package middleware

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"go.uber.org/zap"
)

// AuditLogMiddleware logs request details for audit purposes
func AuditLogMiddleware() gin.HandlerFunc {
	return gin.LoggerWithFormatter(func(param gin.LogFormatterParams) string {
		logrus.WithFields(logrus.Fields{
			"timestamp":  param.TimeStamp.Format(time.RFC3339),
			"status":     param.StatusCode,
			"latency":    param.Latency,
			"client_ip":  param.ClientIP,
			"method":     param.Method,
			"path":       param.Path,
			"user_agent": param.Request.UserAgent(),
			"request_id": param.Keys["request_id"],
		}).Info("Request processed")

		return ""
	})
}

// StructuredLogMiddleware provides structured logging with zap logger
func StructuredLogMiddleware(logger *zap.Logger) gin.HandlerFunc {
	return gin.LoggerWithConfig(gin.LoggerConfig{
		SkipPaths: []string{"/health", "/ready", "/metrics"}, // Skip health check endpoints
		Formatter: func(param gin.LogFormatterParams) string {
			if logger != nil {
				logger.Info("Request processed",
					zap.String("timestamp", param.TimeStamp.Format(time.RFC3339)),
					zap.Int("status", param.StatusCode),
					zap.Duration("latency", param.Latency),
					zap.String("client_ip", param.ClientIP),
					zap.String("method", param.Method),
					zap.String("path", param.Path),
					zap.String("user_agent", param.Request.UserAgent()),
					zap.Any("request_id", param.Keys["request_id"]),
				)
			}
			return ""
		},
	})
}

// AuthEventLogger logs authentication events with context
func AuthEventLogger(logger *zap.Logger, level, event string, c *gin.Context, details map[string]interface{}) {
	fields := []zap.Field{
		zap.String("event", event),
		zap.String("client_ip", c.ClientIP()),
		zap.String("user_agent", c.GetHeader("User-Agent")),
		zap.String("path", c.Request.URL.Path),
		zap.String("method", c.Request.Method),
	}

	// Add request ID if available
	if requestID, exists := c.Get("request_id"); exists {
		fields = append(fields, zap.Any("request_id", requestID))
	}

	// Add additional details
	for k, v := range details {
		fields = append(fields, zap.Any(k, v))
	}

	switch level {
	case "INFO":
		logger.Info("Authentication event", fields...)
	case "WARN":
		logger.Warn("Authentication event", fields...)
	case "ERROR":
		logger.Error("Authentication event", fields...)
	case "DEBUG":
		logger.Debug("Authentication event", fields...)
	default:
		logger.Info("Authentication event", fields...)
	}
}
