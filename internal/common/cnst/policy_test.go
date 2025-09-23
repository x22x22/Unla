package cnst

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMCPStartupPolicy_Constants(t *testing.T) {
	assert.Equal(t, MCPStartupPolicy("onStart"), PolicyOnStart)
	assert.Equal(t, MCPStartupPolicy("onDemand"), PolicyOnDemand)
}

func TestMCPStartupPolicy_String(t *testing.T) {
	assert.Equal(t, "onStart", string(PolicyOnStart))
	assert.Equal(t, "onDemand", string(PolicyOnDemand))
}
