package mcpproxy

import (
	"testing"

	"github.com/amoylab/unla/internal/common/config"
	"github.com/stretchr/testify/assert"
)

func TestNewTransportVariantsAndUnknown(t *testing.T) {
	sse, err := NewTransport(config.MCPServerConfig{Type: string(TypeSSE), URL: "http://localhost"})
	assert.NoError(t, err)
	if assert.NotNil(t, sse) {
		_, ok := sse.(*SSETransport)
		assert.True(t, ok)
	}

	stdio, err := NewTransport(config.MCPServerConfig{Type: string(TypeStdio), Command: "echo"})
	assert.NoError(t, err)
	if assert.NotNil(t, stdio) {
		_, ok := stdio.(*StdioTransport)
		assert.True(t, ok)
	}

	stream, err := NewTransport(config.MCPServerConfig{Type: string(TypeStreamable), URL: "http://localhost"})
	assert.NoError(t, err)
	if assert.NotNil(t, stream) {
		_, ok := stream.(*StreamableTransport)
		assert.True(t, ok)
	}

	_, err = NewTransport(config.MCPServerConfig{Type: "unknown"})
	assert.Error(t, err)
}
