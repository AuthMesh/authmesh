package usermanagement

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

// UserManagementHandler handles user management HTTP operations
type UserManagementHandler struct {
	userManager     *UserManager
	logger          *zap.Logger
	tracer          trace.Tracer
	requestsCounter metric.Int64Counter
	failuresCounter metric.Int64Counter
}

// NewUserManagementHandler creates a new UserManagementHandler
func NewUserManagementHandler(userManager *UserManager, logger *zap.Logger) (*UserManagementHandler, error) {
	tracer := otel.Tracer("user-management-handler")
	meter := otel.Meter("user-management-handler")

	// Create metrics
	requestsCounter, err := meter.Int64Counter(
		"user_management_requests_total",
		metric.WithDescription("Total number of user management requests"),
	)
	if err != nil {
		return nil, err
	}

	failuresCounter, err := meter.Int64Counter(
		"user_management_failures_total",
		metric.WithDescription("Total number of user management failures"),
	)
	if err != nil {
		return nil, err
	}

	return &UserManagementHandler{
		userManager:     userManager,
		logger:          logger,
		tracer:          tracer,
		requestsCounter: requestsCounter,
		failuresCounter: failuresCounter,
	}, nil
}

// AddUserRequest represents the request payload for adding a user
type AddUserRequest struct {
	Username string `json:"username" binding:"required"`
	Email    string `json:"email" binding:"required,email"`
}

// RegisterRequest represents the request payload for self-registration
type RegisterRequest struct {
	Username string `json:"username" binding:"required"`
	Email    string `json:"email" binding:"required,email"`
}

// AddUserSuperAdmin handles POST /superadmin/tenants/{tenant_id}/users
func (h *UserManagementHandler) AddUserSuperAdmin(c *gin.Context) {
	start := time.Now()
	tenantID := c.Param("tenant_id")

	ctx, span := h.tracer.Start(c.Request.Context(), "add_user_superadmin")
	defer span.End()

	span.SetAttributes(
		attribute.String("tenant_id", tenantID),
		attribute.String("operation", "add_user_superadmin"),
	)

	var req AddUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		span.RecordError(err)
		h.failuresCounter.Add(ctx, 1, metric.WithAttributes(
			attribute.String("tenant_id", tenantID),
			attribute.String("error", "invalid_request"),
		))
		h.logger.Warn("Invalid superadmin add user request",
			zap.String("tenant_id", tenantID),
			zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request payload"})
		return
	}

	// Add user via UserManager
	err := h.userManager.AddUser(tenantID, req.Username, req.Email)
	if err != nil {
		span.RecordError(err)
		h.failuresCounter.Add(ctx, 1, metric.WithAttributes(
			attribute.String("tenant_id", tenantID),
			attribute.String("error", "add_user_failed"),
		))
		h.logger.Error("Failed to add user (superadmin)",
			zap.String("tenant_id", tenantID),
			zap.String("username", req.Username),
			zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to add user"})
		return
	}

	// Record success metrics
	h.requestsCounter.Add(ctx, 1, metric.WithAttributes(
		attribute.String("tenant_id", tenantID),
		attribute.String("operation", "add_user_superadmin"),
	))

	h.logger.Info("Successfully added user via superadmin endpoint",
		zap.String("tenant_id", tenantID),
		zap.String("username", req.Username),
		zap.String("email", req.Email),
		zap.Duration("duration", time.Since(start)))

	c.JSON(http.StatusCreated, gin.H{
		"message":  "User created successfully (superadmin)",
		"username": req.Username,
		"email":    req.Email,
	})
}

// AddUser handles POST /t/{tenant_id}/admin/users
func (h *UserManagementHandler) AddUser(c *gin.Context) {
	start := time.Now()
	tenantID := c.Param("tenant_id")

	ctx, span := h.tracer.Start(c.Request.Context(), "add_user_admin")
	defer span.End()

	span.SetAttributes(
		attribute.String("tenant_id", tenantID),
		attribute.String("operation", "add_user_admin"),
	)

	var req AddUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		span.RecordError(err)
		h.failuresCounter.Add(ctx, 1, metric.WithAttributes(
			attribute.String("tenant_id", tenantID),
			attribute.String("error", "invalid_request"),
		))
		h.logger.Warn("Invalid add user request",
			zap.String("tenant_id", tenantID),
			zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request payload"})
		return
	}

	// Add user via UserManager
	err := h.userManager.AddUser(tenantID, req.Username, req.Email)
	if err != nil {
		span.RecordError(err)
		h.failuresCounter.Add(ctx, 1, metric.WithAttributes(
			attribute.String("tenant_id", tenantID),
			attribute.String("error", "add_user_failed"),
		))
		h.logger.Error("Failed to add user",
			zap.String("tenant_id", tenantID),
			zap.String("username", req.Username),
			zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to add user"})
		return
	}

	// Record success metrics
	h.requestsCounter.Add(ctx, 1, metric.WithAttributes(
		attribute.String("tenant_id", tenantID),
		attribute.String("operation", "add_user_admin"),
	))

	h.logger.Info("Successfully added user via admin endpoint",
		zap.String("tenant_id", tenantID),
		zap.String("username", req.Username),
		zap.String("email", req.Email),
		zap.Duration("duration", time.Since(start)))

	c.JSON(http.StatusCreated, gin.H{
		"message":  "User created successfully",
		"username": req.Username,
		"email":    req.Email,
	})
}

// Register handles POST /t/{tenant_id}/register (self-registration)
func (h *UserManagementHandler) Register(c *gin.Context) {
	start := time.Now()
	tenantID := c.Param("tenant_id")

	ctx, span := h.tracer.Start(c.Request.Context(), "self_register")
	defer span.End()

	span.SetAttributes(
		attribute.String("tenant_id", tenantID),
		attribute.String("operation", "self_register"),
	)

	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		span.RecordError(err)
		h.failuresCounter.Add(ctx, 1, metric.WithAttributes(
			attribute.String("tenant_id", tenantID),
			attribute.String("error", "invalid_request"),
		))
		h.logger.Warn("Invalid registration request",
			zap.String("tenant_id", tenantID),
			zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request payload"})
		return
	}

	// Add user via UserManager (self-registration)
	err := h.userManager.AddUser(tenantID, req.Username, req.Email)
	if err != nil {
		span.RecordError(err)
		h.failuresCounter.Add(ctx, 1, metric.WithAttributes(
			attribute.String("tenant_id", tenantID),
			attribute.String("error", "registration_failed"),
		))
		h.logger.Error("Failed to register user",
			zap.String("tenant_id", tenantID),
			zap.String("username", req.Username),
			zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Registration failed"})
		return
	}

	// Record success metrics
	h.requestsCounter.Add(ctx, 1, metric.WithAttributes(
		attribute.String("tenant_id", tenantID),
		attribute.String("operation", "self_register"),
	))

	h.logger.Info("Successfully registered user",
		zap.String("tenant_id", tenantID),
		zap.String("username", req.Username),
		zap.String("email", req.Email),
		zap.Duration("duration", time.Since(start)))

	c.JSON(http.StatusCreated, gin.H{
		"message":   "User registration request received for tenant " + tenantID,
		"tenant_id": tenantID,
		"user": gin.H{
			"username": req.Username,
			"email":    req.Email,
		},
	})
}
