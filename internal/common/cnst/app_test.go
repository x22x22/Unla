package cnst

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAppConstants(t *testing.T) {
	assert.Equal(t, "mcp-gateway", AppName)
	assert.Equal(t, "mcp-gateway", CommandName)
}
