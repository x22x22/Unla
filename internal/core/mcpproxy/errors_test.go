package mcpproxy

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHTTPError(t *testing.T) {
	t.Run("creates error with status code and message", func(t *testing.T) {
		err := &HTTPError{
			StatusCode: 404,
			Message:    "Not Found",
		}

		assert.Equal(t, 404, err.StatusCode)
		assert.Equal(t, "Not Found", err.Message)
		assert.Equal(t, "Not Found", err.Error())
	})

	t.Run("implements error interface", func(t *testing.T) {
		var err error = &HTTPError{
			StatusCode: 500,
			Message:    "Internal Server Error",
		}

		assert.Equal(t, "Internal Server Error", err.Error())
	})

	t.Run("handles empty message", func(t *testing.T) {
		err := &HTTPError{
			StatusCode: 200,
			Message:    "",
		}

		assert.Equal(t, "", err.Error())
	})
}
