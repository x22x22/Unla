package cnst

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestActionType_Constants(t *testing.T) {
	assert.Equal(t, ActionType("Create"), ActionCreate)
	assert.Equal(t, ActionType("Update"), ActionUpdate)
	assert.Equal(t, ActionType("Delete"), ActionDelete)
	assert.Equal(t, ActionType("Revert"), ActionRevert)
}

func TestActionType_String(t *testing.T) {
	assert.Equal(t, "Create", string(ActionCreate))
	assert.Equal(t, "Update", string(ActionUpdate))
	assert.Equal(t, "Delete", string(ActionDelete))
	assert.Equal(t, "Revert", string(ActionRevert))
}

func TestAuthMode_Constants(t *testing.T) {
	assert.Equal(t, AuthMode("oauth2"), AuthModeOAuth2)
}

func TestAuthMode_String(t *testing.T) {
	assert.Equal(t, "oauth2", string(AuthModeOAuth2))
}
