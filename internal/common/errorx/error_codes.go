package errorx

import (
	"encoding/json"
	"fmt"
	"net/http"
)

// ErrorCategory represents different categories of errors
type ErrorCategory string

const (
	CategoryValidation    ErrorCategory = "validation"
	CategoryAuthentication ErrorCategory = "authentication" 
	CategoryAuthorization ErrorCategory = "authorization"
	CategoryNotFound      ErrorCategory = "not_found"
	CategoryConflict      ErrorCategory = "conflict"
	CategoryInternal      ErrorCategory = "internal"
	CategoryExternal      ErrorCategory = "external"
	CategoryConfiguration ErrorCategory = "configuration"
	CategoryNetwork       ErrorCategory = "network"
	CategoryTimeout       ErrorCategory = "timeout"
	CategoryRateLimit     ErrorCategory = "rate_limit"
)

// Severity represents the severity level of an error
type Severity string

const (
	SeverityInfo    Severity = "info"
	SeverityWarning Severity = "warning"
	SeverityError   Severity = "error"
	SeverityCritical Severity = "critical"
)

// APIError represents a structured API error with comprehensive information
type APIError struct {
	Code        string            `json:"code"`
	Message     string            `json:"message"`
	Category    ErrorCategory     `json:"category"`
	Severity    Severity          `json:"severity"`
	HTTPStatus  int               `json:"-"`
	Details     map[string]any    `json:"details,omitempty"`
	Suggestions []string          `json:"suggestions,omitempty"`
	TraceID     string            `json:"trace_id,omitempty"`
	Timestamp   string            `json:"timestamp,omitempty"`
}

// Error implements the error interface
func (e *APIError) Error() string {
	return fmt.Sprintf("[%s] %s: %s", e.Code, e.Category, e.Message)
}

// JSON returns the error as a JSON string
func (e *APIError) JSON() string {
	out, _ := json.Marshal(e)
	return string(out)
}

// WithDetail adds a detail to the error
func (e *APIError) WithDetail(key string, value any) *APIError {
	if e.Details == nil {
		e.Details = make(map[string]any)
	}
	e.Details[key] = value
	return e
}

// WithSuggestion adds a suggestion to the error
func (e *APIError) WithSuggestion(suggestion string) *APIError {
	e.Suggestions = append(e.Suggestions, suggestion)
	return e
}

// WithTraceID adds a trace ID to the error
func (e *APIError) WithTraceID(traceID string) *APIError {
	e.TraceID = traceID
	return e
}

// Common error codes and messages
var (
	// Validation Errors (E1000-E1999)
	ErrInvalidInput = &APIError{
		Code:       "E1001",
		Message:    "Invalid input provided",
		Category:   CategoryValidation,
		Severity:   SeverityError,
		HTTPStatus: http.StatusBadRequest,
		Suggestions: []string{
			"Check the request format and try again",
			"Ensure all required fields are provided",
		},
	}

	ErrMissingField = &APIError{
		Code:       "E1002",
		Message:    "Required field is missing",
		Category:   CategoryValidation,
		Severity:   SeverityError,
		HTTPStatus: http.StatusBadRequest,
		Suggestions: []string{
			"Check the API documentation for required fields",
		},
	}

	ErrInvalidFormat = &APIError{
		Code:       "E1003",
		Message:    "Invalid data format",
		Category:   CategoryValidation,
		Severity:   SeverityError,
		HTTPStatus: http.StatusBadRequest,
		Suggestions: []string{
			"Check the data format according to the API specification",
		},
	}

	// Authentication Errors (E2000-E2999)
	ErrUnauthorized = &APIError{
		Code:       "E2001",
		Message:    "Authentication required",
		Category:   CategoryAuthentication,
		Severity:   SeverityError,
		HTTPStatus: http.StatusUnauthorized,
		Suggestions: []string{
			"Please login and try again",
			"Check if your authentication token is valid",
		},
	}

	ErrInvalidCredentials = &APIError{
		Code:       "E2002",
		Message:    "Invalid credentials provided",
		Category:   CategoryAuthentication,
		Severity:   SeverityError,
		HTTPStatus: http.StatusUnauthorized,
		Suggestions: []string{
			"Check your username and password",
			"Reset your password if needed",
		},
	}

	ErrTokenExpired = &APIError{
		Code:       "E2003",
		Message:    "Authentication token has expired",
		Category:   CategoryAuthentication,
		Severity:   SeverityWarning,
		HTTPStatus: http.StatusUnauthorized,
		Suggestions: []string{
			"Please login again to get a new token",
		},
	}

	// Authorization Errors (E3000-E3999)
	ErrForbidden = &APIError{
		Code:       "E3001",
		Message:    "Access denied",
		Category:   CategoryAuthorization,
		Severity:   SeverityError,
		HTTPStatus: http.StatusForbidden,
		Suggestions: []string{
			"Contact your administrator for permission",
		},
	}

	ErrInsufficientPermissions = &APIError{
		Code:       "E3002",
		Message:    "Insufficient permissions to perform this action",
		Category:   CategoryAuthorization,
		Severity:   SeverityError,
		HTTPStatus: http.StatusForbidden,
		Suggestions: []string{
			"Request additional permissions from your administrator",
		},
	}

	// Not Found Errors (E4000-E4999)
	ErrResourceNotFound = &APIError{
		Code:       "E4001",
		Message:    "Requested resource not found",
		Category:   CategoryNotFound,
		Severity:   SeverityError,
		HTTPStatus: http.StatusNotFound,
		Suggestions: []string{
			"Check if the resource ID is correct",
			"The resource might have been deleted",
		},
	}

	ErrEndpointNotFound = &APIError{
		Code:       "E4002",
		Message:    "API endpoint not found",
		Category:   CategoryNotFound,
		Severity:   SeverityError,
		HTTPStatus: http.StatusNotFound,
		Suggestions: []string{
			"Check the API documentation for correct endpoints",
		},
	}

	// Conflict Errors (E4090-E4099)
	ErrResourceExists = &APIError{
		Code:       "E4091",
		Message:    "Resource already exists",
		Category:   CategoryConflict,
		Severity:   SeverityError,
		HTTPStatus: http.StatusConflict,
		Suggestions: []string{
			"Use a different name or ID",
			"Update the existing resource instead",
		},
	}

	ErrConcurrentModification = &APIError{
		Code:       "E4092",
		Message:    "Resource was modified by another request",
		Category:   CategoryConflict,
		Severity:   SeverityWarning,
		HTTPStatus: http.StatusConflict,
		Suggestions: []string{
			"Refresh the resource and try again",
		},
	}

	// Rate Limiting Errors (E4290-E4299)
	ErrRateLimitExceeded = &APIError{
		Code:       "E4291",
		Message:    "Rate limit exceeded",
		Category:   CategoryRateLimit,
		Severity:   SeverityWarning,
		HTTPStatus: http.StatusTooManyRequests,
		Suggestions: []string{
			"Please wait before making more requests",
			"Consider upgrading your plan for higher limits",
		},
	}

	// Internal Server Errors (E5000-E5999)
	ErrInternalServer = &APIError{
		Code:       "E5001",
		Message:    "Internal server error occurred",
		Category:   CategoryInternal,
		Severity:   SeverityCritical,
		HTTPStatus: http.StatusInternalServerError,
		Suggestions: []string{
			"Please try again later",
			"Contact support if the issue persists",
		},
	}

	ErrDatabaseError = &APIError{
		Code:       "E5002",
		Message:    "Database operation failed",
		Category:   CategoryInternal,
		Severity:   SeverityError,
		HTTPStatus: http.StatusInternalServerError,
		Suggestions: []string{
			"Please try again later",
			"Contact support if the issue persists",
		},
	}

	ErrConfigurationError = &APIError{
		Code:       "E5003",
		Message:    "Configuration error",
		Category:   CategoryConfiguration,
		Severity:   SeverityError,
		HTTPStatus: http.StatusInternalServerError,
		Suggestions: []string{
			"Check the server configuration",
			"Contact your system administrator",
		},
	}

	// Network/External Errors (E5030-E5099)
	ErrNetworkTimeout = &APIError{
		Code:       "E5031",
		Message:    "Network timeout occurred",
		Category:   CategoryTimeout,
		Severity:   SeverityWarning,
		HTTPStatus: http.StatusGatewayTimeout,
		Suggestions: []string{
			"Please try again",
			"Check your network connection",
		},
	}

	ErrExternalServiceUnavailable = &APIError{
		Code:       "E5032",
		Message:    "External service unavailable",
		Category:   CategoryExternal,
		Severity:   SeverityError,
		HTTPStatus: http.StatusBadGateway,
		Suggestions: []string{
			"The service is temporarily unavailable",
			"Please try again later",
		},
	}

	// MCP Specific Errors (E6000-E6999)
	ErrMCPServerNotFound = &APIError{
		Code:       "E6001",
		Message:    "MCP server not found",
		Category:   CategoryNotFound,
		Severity:   SeverityError,
		HTTPStatus: http.StatusNotFound,
		Suggestions: []string{
			"Check the server name and configuration",
			"Ensure the server is properly registered",
		},
	}

	ErrMCPConnectionFailed = &APIError{
		Code:       "E6002",
		Message:    "Failed to connect to MCP server",
		Category:   CategoryNetwork,
		Severity:   SeverityError,
		HTTPStatus: http.StatusBadGateway,
		Suggestions: []string{
			"Check if the MCP server is running",
			"Verify the connection configuration",
		},
	}

	ErrMCPCapabilityNotFound = &APIError{
		Code:       "E6003",
		Message:    "MCP capability not found",
		Category:   CategoryNotFound,
		Severity:   SeverityError,
		HTTPStatus: http.StatusNotFound,
		Suggestions: []string{
			"Check the capability name",
			"Refresh the server capabilities",
		},
	}

	ErrMCPToolExecutionFailed = &APIError{
		Code:       "E6004",
		Message:    "MCP tool execution failed",
		Category:   CategoryExternal,
		Severity:   SeverityError,
		HTTPStatus: http.StatusBadGateway,
		Suggestions: []string{
			"Check the tool parameters",
			"Verify the MCP server is functioning correctly",
		},
	}

	ErrMCPConfigInvalid = &APIError{
		Code:       "E6005",
		Message:    "Invalid MCP configuration",
		Category:   CategoryConfiguration,
		Severity:   SeverityError,
		HTTPStatus: http.StatusBadRequest,
		Suggestions: []string{
			"Check the configuration format",
			"Validate against the MCP configuration schema",
		},
	}
)