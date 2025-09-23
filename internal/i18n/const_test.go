package i18n

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCommonErrors(t *testing.T) {
	t.Run("common errors are not nil", func(t *testing.T) {
		assert.NotNil(t, ErrNotFound)
		assert.NotNil(t, ErrUnauthorized)
		assert.NotNil(t, ErrForbidden)
		assert.NotNil(t, ErrBadRequest)
		assert.NotNil(t, ErrInternalServer)
	})

	t.Run("common errors have correct message IDs", func(t *testing.T) {
		assert.Equal(t, "ErrorResourceNotFound", ErrNotFound.MessageID)
		assert.Equal(t, "ErrorUnauthorized", ErrUnauthorized.MessageID)
		assert.Equal(t, "ErrorForbidden", ErrForbidden.MessageID)
		assert.Equal(t, "ErrorBadRequest", ErrBadRequest.MessageID)
		assert.Equal(t, "ErrorInternalServer", ErrInternalServer.MessageID)
	})
}

func TestTenantErrors(t *testing.T) {
	tenantErrors := []error{
		ErrorTenantNotFound,
		ErrorTenantNameRequired,
		ErrorTenantPrefixExists,
		ErrorTenantNameExists,
		ErrorTenantRequiredFields,
		ErrorTenantPermissionError,
		ErrorTenantDisabled,
	}

	for _, err := range tenantErrors {
		assert.NotNil(t, err)
		assert.IsType(t, &ErrorWithCode{}, err)
	}
}

func TestUserErrors(t *testing.T) {
	userErrors := []error{
		ErrorUserNotFound,
		ErrorInvalidCredentials,
		ErrorUserDisabled,
		ErrorUserNamePasswordRequired,
		ErrorInvalidOldPassword,
		ErrorUsernameExists,
		ErrorInvalidUsername,
		ErrorEmailExists,
		ErrorInvalidEmail,
	}

	for _, err := range userErrors {
		assert.NotNil(t, err)
		assert.IsType(t, &ErrorWithCode{}, err)
	}
}

func TestMCPErrors(t *testing.T) {
	mcpErrors := []error{
		ErrorMCPServerNotFound,
		ErrorMCPServerExists,
		ErrorMCPServerValidation,
		ErrorTenantRequired,
		ErrorMCPServerNameRequired,
		ErrorVersionRequired,
		ErrorRouterPrefixError,
		ErrorMCPConfigInvalid,
		ErrorMCPRequestFailed,
	}

	for _, err := range mcpErrors {
		assert.NotNil(t, err)
		assert.IsType(t, &ErrorWithCode{}, err)
	}
}

func TestAPIErrors(t *testing.T) {
	apiErrors := []error{
		ErrorAPINotFound,
		ErrorAPIMethodNotAllowed,
		ErrorAPIRateLimitExceeded,
		ErrorAPIUnavailable,
		ErrorAPITimeout,
		ErrorAPIValidationFailed,
		ErrorAPIMalformedRequest,
		ErrorAPIResponseInvalid,
		ErrorAPIInvalidCredentials,
		ErrorAPIPermissionDenied,
		ErrorAPIUnsupportedMediaType,
	}

	for _, err := range apiErrors {
		assert.NotNil(t, err)
		assert.IsType(t, &ErrorWithCode{}, err)
	}
}

func TestValidationErrors(t *testing.T) {
	validationErrors := []error{
		ErrorRequiredField,
		ErrorInvalidFormat,
		ErrorInvalidValue,
		ErrorDuplicateEntity,
		ErrorDataIntegrityViolation,
	}

	for _, err := range validationErrors {
		assert.NotNil(t, err)
		assert.IsType(t, &ErrorWithCode{}, err)
	}
}

func TestSuccessConstants(t *testing.T) {
	t.Run("tenant success constants", func(t *testing.T) {
		assert.Equal(t, "SuccessTenantCreated", SuccessTenantCreated)
		assert.Equal(t, "SuccessTenantUpdated", SuccessTenantUpdated)
		assert.Equal(t, "SuccessTenantDeleted", SuccessTenantDeleted)
		assert.Equal(t, "SuccessTenantInfo", SuccessTenantInfo)
		assert.Equal(t, "SuccessTenantList", SuccessTenantList)
		assert.Equal(t, "SuccessTenantStatus", SuccessTenantStatus)
	})

	t.Run("user success constants", func(t *testing.T) {
		assert.Equal(t, "SuccessLogin", SuccessLogin)
		assert.Equal(t, "SuccessLogout", SuccessLogout)
		assert.Equal(t, "SuccessPasswordChanged", SuccessPasswordChanged)
		assert.Equal(t, "SuccessUserCreated", SuccessUserCreated)
		assert.Equal(t, "SuccessUserUpdated", SuccessUserUpdated)
		assert.Equal(t, "SuccessUserDeleted", SuccessUserDeleted)
	})

	t.Run("MCP success constants", func(t *testing.T) {
		assert.Equal(t, "SuccessMCPServerCreated", SuccessMCPServerCreated)
		assert.Equal(t, "SuccessMCPServerUpdated", SuccessMCPServerUpdated)
		assert.Equal(t, "SuccessMCPServerDeleted", SuccessMCPServerDeleted)
		assert.Equal(t, "SuccessMCPServerSynced", SuccessMCPServerSynced)
	})

	t.Run("general success constants", func(t *testing.T) {
		assert.Equal(t, "SuccessOperationCompleted", SuccessOperationCompleted)
		assert.Equal(t, "SuccessItemCreated", SuccessItemCreated)
		assert.Equal(t, "SuccessItemUpdated", SuccessItemUpdated)
		assert.Equal(t, "SuccessItemDeleted", SuccessItemDeleted)
	})
}
