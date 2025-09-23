package cnst

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestErrorConstants(t *testing.T) {
	t.Run("duplicate errors", func(t *testing.T) {
		assert.Equal(t, "duplicate tool name", ErrDuplicateToolName.Error())
		assert.Equal(t, "duplicate server name", ErrDuplicateServerName.Error())
		assert.Equal(t, "duplicate router prefix", ErrDuplicateRouterPrefix.Error())
	})

	t.Run("notifier errors", func(t *testing.T) {
		assert.Equal(t, "notifier cannot receive updates", ErrNotReceiver.Error())
		assert.Equal(t, "notifier cannot send updates", ErrNotSender.Error())
	})

	t.Run("errors are not nil", func(t *testing.T) {
		assert.NotNil(t, ErrDuplicateToolName)
		assert.NotNil(t, ErrDuplicateServerName)
		assert.NotNil(t, ErrDuplicateRouterPrefix)
		assert.NotNil(t, ErrNotReceiver)
		assert.NotNil(t, ErrNotSender)
	})
}
