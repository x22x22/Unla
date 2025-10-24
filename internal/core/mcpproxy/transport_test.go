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

func TestTransportTypes(t *testing.T) {
	// Test that transport type constants have expected values
	assert.Equal(t, TransportType("sse"), TypeSSE)
	assert.Equal(t, TransportType("stdio"), TypeStdio)
	assert.Equal(t, TransportType("streamable-http"), TypeStreamable)

	// Test type conversion
	assert.Equal(t, string(TypeSSE), "sse")
	assert.Equal(t, string(TypeStdio), "stdio")
	assert.Equal(t, string(TypeStreamable), "streamable-http")
}

func TestNewTransport_ErrorMessage(t *testing.T) {
	_, err := NewTransport(config.MCPServerConfig{Type: "invalid-type"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown transport type: invalid-type")
}
