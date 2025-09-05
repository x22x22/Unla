package errorx

import (
	"fmt"

	"github.com/amoylab/unla/internal/i18n"
)

// ErrorTranslator provides internationalized error messages
type ErrorTranslator struct {
	translator *i18n.I18n
}

// NewErrorTranslator creates a new error translator
func NewErrorTranslator(translator *i18n.I18n) *ErrorTranslator {
	return &ErrorTranslator{
		translator: translator,
	}
}

// TranslateError translates an APIError to the specified language
func (t *ErrorTranslator) TranslateError(err *APIError, lang string) *APIError {
	if t.translator == nil {
		return err
	}

	// Create a copy to avoid modifying the original
	translatedErr := &APIError{
		Code:        err.Code,
		Message:     t.translateMessage(err.Code, lang),
		Category:    err.Category,
		Severity:    err.Severity,
		HTTPStatus:  err.HTTPStatus,
		Details:     err.Details,
		Suggestions: t.translateSuggestions(err.Code, lang),
		TraceID:     err.TraceID,
		Timestamp:   err.Timestamp,
	}

	return translatedErr
}

// translateMessage translates the error message based on error code
func (t *ErrorTranslator) translateMessage(code string, lang string) string {
	key := fmt.Sprintf("error.%s.message", code)
	if translated := t.translator.Translate(key, lang, nil); translated != key {
		return translated
	}

	// Fallback to category-based translation
	switch code[:2] {
	case "E1":
		return t.translator.Translate("error.category.validation", lang, nil)
	case "E2":
		return t.translator.Translate("error.category.authentication", lang, nil)
	case "E3":
		return t.translator.Translate("error.category.authorization", lang, nil)
	case "E4":
		return t.translator.Translate("error.category.not_found", lang, nil)
	case "E5":
		return t.translator.Translate("error.category.internal", lang, nil)
	case "E6":
		return t.translator.Translate("error.category.mcp", lang, nil)
	default:
		return t.translator.Translate("error.category.unknown", lang, nil)
	}
}

// translateSuggestions translates error suggestions
func (t *ErrorTranslator) translateSuggestions(code string, lang string) []string {
	suggestionsKey := fmt.Sprintf("error.%s.suggestions", code)
	
	// Try to get translated suggestions as a slice
	// Note: TranslateSlice method doesn't exist in I18n, using generic approach
	if translated := t.translator.Translate(suggestionsKey, lang, nil); translated != suggestionsKey {
		// For now, return as single suggestion - would need custom implementation for slice support
		return []string{translated}
	}

	// Fallback to generic suggestions based on category
	return t.getGenericSuggestions(code, lang)
}

// getGenericSuggestions provides generic suggestions based on error code category
func (t *ErrorTranslator) getGenericSuggestions(code string, lang string) []string {
	switch code[:2] {
	case "E1": // Validation
		return []string{
			t.translator.Translate("error.suggestion.check_input", lang, nil),
			t.translator.Translate("error.suggestion.check_documentation", lang, nil),
		}
	case "E2": // Authentication
		return []string{
			t.translator.Translate("error.suggestion.check_credentials", lang, nil),
			t.translator.Translate("error.suggestion.login_again", lang, nil),
		}
	case "E3": // Authorization
		return []string{
			t.translator.Translate("error.suggestion.contact_admin", lang, nil),
			t.translator.Translate("error.suggestion.check_permissions", lang, nil),
		}
	case "E4": // Not Found
		return []string{
			t.translator.Translate("error.suggestion.check_resource_id", lang, nil),
			t.translator.Translate("error.suggestion.check_existence", lang, nil),
		}
	case "E5": // Internal
		return []string{
			t.translator.Translate("error.suggestion.try_later", lang, nil),
			t.translator.Translate("error.suggestion.contact_support", lang, nil),
		}
	case "E6": // MCP
		return []string{
			t.translator.Translate("error.suggestion.check_mcp_config", lang, nil),
			t.translator.Translate("error.suggestion.check_server_status", lang, nil),
		}
	default:
		return []string{
			t.translator.Translate("error.suggestion.generic", lang, nil),
		}
	}
}

// GetUserFriendlyMessage returns a user-friendly error message with context
func (t *ErrorTranslator) GetUserFriendlyMessage(err *APIError, lang string, context map[string]interface{}) string {
	baseMessage := t.translateMessage(err.Code, lang)
	
	// Add context if available
	if context != nil {
		if resource, ok := context["resource"]; ok {
			contextKey := fmt.Sprintf("error.%s.with_resource", err.Code)
			if translated := t.translator.Translate(contextKey, lang, map[string]interface{}{
				"resource": resource,
			}); translated != contextKey {
				return translated
			}
		}
	}

	return baseMessage
}

// GetActionableSteps returns actionable steps the user can take
func (t *ErrorTranslator) GetActionableSteps(err *APIError, lang string) []ActionableStep {
	stepsKey := fmt.Sprintf("error.%s.steps", err.Code)
	
	// Try to get specific steps for this error code
	if steps := t.getTranslatedSteps(stepsKey, lang); len(steps) > 0 {
		return steps
	}

	// Fallback to category-based steps
	return t.getCategorySteps(err.Code, lang)
}

// ActionableStep represents a step the user can take to resolve the error
type ActionableStep struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	Action      string `json:"action,omitempty"` // Optional action identifier
	URL         string `json:"url,omitempty"`    // Optional URL for more information
}

// getTranslatedSteps retrieves translated actionable steps
func (t *ErrorTranslator) getTranslatedSteps(key string, lang string) []ActionableStep {
	// This would need to be implemented based on your i18n system's capabilities
	// For now, return empty to use fallback
	return nil
}

// getCategorySteps provides generic steps based on error category
func (t *ErrorTranslator) getCategorySteps(code string, lang string) []ActionableStep {
	switch code[:2] {
	case "E1": // Validation
		return []ActionableStep{
			{
				Title:       t.translator.Translate("error.step.validate_input.title", lang, nil),
				Description: t.translator.Translate("error.step.validate_input.description", lang, nil),
				Action:      "validate_input",
			},
			{
				Title:       t.translator.Translate("error.step.check_format.title", lang, nil),
				Description: t.translator.Translate("error.step.check_format.description", lang, nil),
				Action:      "check_format",
			},
		}
	case "E2": // Authentication
		return []ActionableStep{
			{
				Title:       t.translator.Translate("error.step.login.title", lang, nil),
				Description: t.translator.Translate("error.step.login.description", lang, nil),
				Action:      "login",
			},
			{
				Title:       t.translator.Translate("error.step.refresh_token.title", lang, nil),
				Description: t.translator.Translate("error.step.refresh_token.description", lang, nil),
				Action:      "refresh_token",
			},
		}
	case "E3": // Authorization
		return []ActionableStep{
			{
				Title:       t.translator.Translate("error.step.contact_admin.title", lang, nil),
				Description: t.translator.Translate("error.step.contact_admin.description", lang, nil),
				Action:      "contact_admin",
			},
		}
	case "E4": // Not Found
		return []ActionableStep{
			{
				Title:       t.translator.Translate("error.step.verify_resource.title", lang, nil),
				Description: t.translator.Translate("error.step.verify_resource.description", lang, nil),
				Action:      "verify_resource",
			},
			{
				Title:       t.translator.Translate("error.step.check_spelling.title", lang, nil),
				Description: t.translator.Translate("error.step.check_spelling.description", lang, nil),
				Action:      "check_spelling",
			},
		}
	case "E5": // Internal
		return []ActionableStep{
			{
				Title:       t.translator.Translate("error.step.retry.title", lang, nil),
				Description: t.translator.Translate("error.step.retry.description", lang, nil),
				Action:      "retry",
			},
			{
				Title:       t.translator.Translate("error.step.wait.title", lang, nil),
				Description: t.translator.Translate("error.step.wait.description", lang, nil),
				Action:      "wait",
			},
		}
	case "E6": // MCP
		return []ActionableStep{
			{
				Title:       t.translator.Translate("error.step.check_mcp_server.title", lang, nil),
				Description: t.translator.Translate("error.step.check_mcp_server.description", lang, nil),
				Action:      "check_mcp_server",
			},
			{
				Title:       t.translator.Translate("error.step.reload_config.title", lang, nil),
				Description: t.translator.Translate("error.step.reload_config.description", lang, nil),
				Action:      "reload_config",
			},
		}
	default:
		return []ActionableStep{
			{
				Title:       t.translator.Translate("error.step.generic.title", lang, nil),
				Description: t.translator.Translate("error.step.generic.description", lang, nil),
				Action:      "contact_support",
			},
		}
	}
}

// Enhanced error response structure for API
type ErrorResponse struct {
	Error           *APIError        `json:"error"`
	Message         string           `json:"message"`
	Suggestions     []string         `json:"suggestions,omitempty"`
	ActionableSteps []ActionableStep `json:"actionable_steps,omitempty"`
	TraceID         string           `json:"trace_id,omitempty"`
	Timestamp       string           `json:"timestamp,omitempty"`
	HelpURL         string           `json:"help_url,omitempty"`
}

// CreateUserFriendlyResponse creates a comprehensive error response with translations
func (t *ErrorTranslator) CreateUserFriendlyResponse(err *APIError, lang string, context map[string]interface{}) *ErrorResponse {
	translatedErr := t.TranslateError(err, lang)
	
	response := &ErrorResponse{
		Error:           translatedErr,
		Message:         t.GetUserFriendlyMessage(err, lang, context),
		Suggestions:     translatedErr.Suggestions,
		ActionableSteps: t.GetActionableSteps(err, lang),
		TraceID:         err.TraceID,
		Timestamp:       err.Timestamp,
		HelpURL:         t.getHelpURL(err.Code),
	}

	return response
}

// getHelpURL returns a help URL for the specific error code
func (t *ErrorTranslator) getHelpURL(code string) string {
	// This would typically be configured or come from a database
	baseURL := "https://docs.unla.amoylab.com/errors"
	return fmt.Sprintf("%s/%s", baseURL, code)
}