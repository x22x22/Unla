package errorx

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"runtime"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// ErrorHandler provides unified error handling capabilities
type ErrorHandler struct {
	logger *zap.Logger
}

// NewErrorHandler creates a new error handler
func NewErrorHandler(logger *zap.Logger) *ErrorHandler {
	return &ErrorHandler{
		logger: logger,
	}
}

// HandleError converts any error to APIError and returns appropriate HTTP response
func (h *ErrorHandler) HandleError(c *gin.Context, err error) {
	if err == nil {
		return
	}

	// Generate trace ID
	traceID := uuid.New().String()
	
	// Convert to APIError
	apiErr := h.ConvertToAPIError(err)
	apiErr.TraceID = traceID
	apiErr.Timestamp = time.Now().UTC().Format(time.RFC3339)

	// Log the error with context
	h.logError(c, apiErr, err)

	// Return JSON error response
	c.JSON(apiErr.HTTPStatus, gin.H{
		"error": apiErr,
	})
}

// ConvertToAPIError converts any error to APIError
func (h *ErrorHandler) ConvertToAPIError(err error) *APIError {
	// If already an APIError, return it
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		return apiErr
	}

	// If it's an OAuth2Error, convert it
	var oauthErr *OAuth2Error
	if errors.As(err, &oauthErr) {
		return &APIError{
			Code:       fmt.Sprintf("E2%03d", oauthErr.HTTPStatus-200), // Generate code from HTTP status
			Message:    oauthErr.ErrorDescription,
			Category:   CategoryAuthentication,
			Severity:   SeverityError,
			HTTPStatus: oauthErr.HTTPStatus,
			Details: map[string]any{
				"oauth2_error": oauthErr.ErrorType,
				"oauth2_code":  oauthErr.ErrorCode,
			},
		}
	}

	// For other errors, create a generic internal server error
	return &APIError{
		Code:       "E5001",
		Message:    "Internal server error occurred",
		Category:   CategoryInternal,
		Severity:   SeverityCritical,
		HTTPStatus: http.StatusInternalServerError,
		Details: map[string]any{
			"original_error": err.Error(),
		},
		Suggestions: []string{
			"Please try again later",
			"Contact support if the issue persists",
		},
	}
}

// logError logs the error with appropriate context and stack trace
func (h *ErrorHandler) logError(c *gin.Context, apiErr *APIError, originalErr error) {
	// Get stack trace for critical errors
	var stackTrace string
	if apiErr.Severity == SeverityCritical {
		buf := make([]byte, 1024*4)
		n := runtime.Stack(buf, false)
		stackTrace = string(buf[:n])
	}

	// Create log fields
	fields := []zap.Field{
		zap.String("trace_id", apiErr.TraceID),
		zap.String("error_code", apiErr.Code),
		zap.String("category", string(apiErr.Category)),
		zap.String("severity", string(apiErr.Severity)),
		zap.Int("http_status", apiErr.HTTPStatus),
		zap.String("path", c.Request.URL.Path),
		zap.String("method", c.Request.Method),
		zap.String("user_agent", c.GetHeader("User-Agent")),
		zap.String("client_ip", c.ClientIP()),
	}

	// Add original error if different from APIError message
	if originalErr != nil && originalErr.Error() != apiErr.Message {
		fields = append(fields, zap.Error(originalErr))
	}

	// Add details if present
	if len(apiErr.Details) > 0 {
		detailsJSON, _ := json.Marshal(apiErr.Details)
		fields = append(fields, zap.String("details", string(detailsJSON)))
	}

	// Add stack trace for critical errors
	if stackTrace != "" {
		fields = append(fields, zap.String("stack_trace", stackTrace))
	}

	// Log with appropriate level based on severity
	switch apiErr.Severity {
	case SeverityInfo:
		h.logger.Info(apiErr.Message, fields...)
	case SeverityWarning:
		h.logger.Warn(apiErr.Message, fields...)
	case SeverityError:
		h.logger.Error(apiErr.Message, fields...)
	case SeverityCritical:
		h.logger.Error(apiErr.Message, fields...)
	default:
		h.logger.Error(apiErr.Message, fields...)
	}
}

// ErrorMiddleware returns a gin middleware for error handling
func (h *ErrorHandler) ErrorMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Execute the request
		c.Next()

		// Check for errors
		if len(c.Errors) > 0 {
			// Handle the last error
			lastErr := c.Errors.Last()
			h.HandleError(c, lastErr.Err)
		}
	}
}

// RecoveryMiddleware returns a gin middleware for panic recovery
func (h *ErrorHandler) RecoveryMiddleware() gin.HandlerFunc {
	return gin.CustomRecoveryWithWriter(nil, func(c *gin.Context, err interface{}) {
		// Create a critical error from the panic
		panicErr := &APIError{
			Code:       "E5000",
			Message:    "Server panic occurred",
			Category:   CategoryInternal,
			Severity:   SeverityCritical,
			HTTPStatus: http.StatusInternalServerError,
			Details: map[string]any{
				"panic": fmt.Sprintf("%v", err),
			},
			Suggestions: []string{
				"This is a critical server error",
				"Please contact support immediately",
			},
		}

		h.HandleError(c, panicErr)
	})
}

// Helper functions for creating specific errors

// ValidationError creates a validation error with details
func ValidationError(field string, value interface{}, reason string) *APIError {
	return ErrInvalidInput.WithDetail("field", field).
		WithDetail("value", value).
		WithDetail("reason", reason).
		WithSuggestion(fmt.Sprintf("Fix the '%s' field and try again", field))
}

// NotFoundError creates a not found error for a specific resource
func NotFoundError(resourceType string, identifier string) *APIError {
	return ErrResourceNotFound.WithDetail("resource_type", resourceType).
		WithDetail("identifier", identifier).
		WithSuggestion(fmt.Sprintf("Check if the %s with ID '%s' exists", resourceType, identifier))
}

// ConflictError creates a conflict error for a specific resource
func ConflictError(resourceType string, field string, value interface{}) *APIError {
	return ErrResourceExists.WithDetail("resource_type", resourceType).
		WithDetail("field", field).
		WithDetail("value", value).
		WithSuggestion(fmt.Sprintf("Use a different %s value", field))
}

// NetworkError creates a network error with connection details
func NetworkError(service string, endpoint string, originalErr error) *APIError {
	return ErrExternalServiceUnavailable.WithDetail("service", service).
		WithDetail("endpoint", endpoint).
		WithDetail("original_error", originalErr.Error()).
		WithSuggestion(fmt.Sprintf("The %s service is currently unavailable", service))
}

// ConfigurationError creates a configuration error with details
func ConfigurationError(component string, key string, reason string) *APIError {
	return ErrConfigurationError.WithDetail("component", component).
		WithDetail("key", key).
		WithDetail("reason", reason).
		WithSuggestion(fmt.Sprintf("Check the %s configuration for '%s'", component, key))
}

// ExtractTraceID extracts trace ID from context or request
func ExtractTraceID(c *gin.Context) string {
	// Try to get from context first
	if traceID := c.GetString("trace_id"); traceID != "" {
		return traceID
	}

	// Try to get from headers
	if traceID := c.GetHeader("X-Trace-Id"); traceID != "" {
		return traceID
	}

	// Generate new trace ID
	traceID := uuid.New().String()
	c.Set("trace_id", traceID)
	return traceID
}

// SetErrorContext adds error context information to the gin context
func SetErrorContext(c *gin.Context, key string, value interface{}) {
	if c == nil {
		return
	}
	
	contextKey := fmt.Sprintf("error_context_%s", key)
	c.Set(contextKey, value)
}

// GetErrorContext retrieves error context information from gin context
func GetErrorContext(c *gin.Context, key string) (interface{}, bool) {
	if c == nil {
		return nil, false
	}
	
	contextKey := fmt.Sprintf("error_context_%s", key)
	return c.Get(contextKey)
}

// WrapWithContext wraps an error with additional context from gin.Context
func WrapWithContext(c *gin.Context, err error) error {
	if err == nil {
		return nil
	}

	// If already an APIError, enhance with context
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		// Add request context
		if apiErr.Details == nil {
			apiErr.Details = make(map[string]any)
		}
		
		apiErr.Details["request_path"] = c.Request.URL.Path
		apiErr.Details["request_method"] = c.Request.Method
		
		// Add any error context set in the request
		for keyStr, value := range c.Keys {
			if len(keyStr) > 14 && keyStr[:14] == "error_context_" {
				contextKey := keyStr[14:] // Remove "error_context_" prefix
				apiErr.Details[contextKey] = value
			}
		}
		
		return apiErr
	}

	return err
}