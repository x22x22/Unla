package mcpproxy

import (
	"context"
	"errors"
	"testing"

	"github.com/amoylab/unla/internal/common/config"
	"github.com/amoylab/unla/pkg/mcp"
	"github.com/stretchr/testify/assert"
)

func TestSSETransport_Basic(t *testing.T) {
	tr := &SSETransport{cfg: config.MCPServerConfig{URL: "http://127.0.0.1:0"}}
	ctx := context.Background()

	// Not running by default
	assert.False(t, tr.IsRunning())

	// Stop no-op when not running
	assert.NoError(t, tr.Stop(ctx))

	// Prompts behavior
	ps, err := tr.FetchPrompts(ctx)
	assert.NoError(t, err)
	assert.Empty(t, ps)

	p, err := tr.FetchPrompt(ctx, "nope")
	assert.Nil(t, p)
	var httpErr *HTTPError
	assert.True(t, errors.As(err, &httpErr))
	assert.Equal(t, 404, httpErr.StatusCode)
}

func TestStdioTransport_ErrorOnInvalidArgsAndBasic(t *testing.T) {
	tr := &StdioTransport{cfg: config.MCPServerConfig{Command: "echo"}}
	ctx := context.Background()

	// Not running by default
	assert.False(t, tr.IsRunning())
	assert.NoError(t, tr.Stop(ctx))

	// invalid json should be caught before trying to start
	_, err := tr.CallTool(ctx, mcp.CallToolParams{Name: "x", Arguments: []byte("not-json")}, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid tool arguments")

	// Prompts behavior
	ps, err := tr.FetchPrompts(ctx)
	assert.NoError(t, err)
	assert.Empty(t, ps)

	p, err := tr.FetchPrompt(ctx, "nope")
	assert.Nil(t, p)
	var httpErr *HTTPError
	assert.True(t, errors.As(err, &httpErr))
	assert.Equal(t, 404, httpErr.StatusCode)
}

func TestStreamableTransport_ErrorOnInvalidArgsAndBasic(t *testing.T) {
	tr := &StreamableTransport{cfg: config.MCPServerConfig{URL: "http://127.0.0.1:0"}}
	ctx := context.Background()

	// Not running by default
	assert.False(t, tr.IsRunning())
	assert.NoError(t, tr.Stop(ctx))

	// invalid json should be caught before trying to start
	_, err := tr.CallTool(ctx, mcp.CallToolParams{Name: "x", Arguments: []byte("not-json")}, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid tool arguments")

	// Prompts behavior
	ps, err := tr.FetchPrompts(ctx)
	assert.NoError(t, err)
	assert.Empty(t, ps)

	p, err := tr.FetchPrompt(ctx, "nope")
	assert.Nil(t, p)
	var httpErr *HTTPError
	assert.True(t, errors.As(err, &httpErr))
	assert.Equal(t, 404, httpErr.StatusCode)
}
