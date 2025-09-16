package backend

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
)

func TestHandleReadAndTemplates(t *testing.T) {
	// static read
	res, err := handleReadResource(context.Background(), mcp.ReadResourceRequest{})
	assert.NoError(t, err)
	assert.Len(t, res, 1)
	if len(res) > 0 {
		tr, ok := res[0].(mcp.TextResourceContents)
		assert.True(t, ok)
		assert.Equal(t, "test://static/resource", tr.URI)
		assert.Equal(t, "text/plain", tr.MIMEType)
	}

	// template with param
	uri := "test://dynamic/resource/42"
	res, err = handleResourceTemplate(context.Background(), mcp.ReadResourceRequest{Params: mcp.ReadResourceParams{URI: uri}})
	assert.NoError(t, err)
	assert.Len(t, res, 1)
	if len(res) > 0 {
		tr, ok := res[0].(mcp.TextResourceContents)
		assert.True(t, ok)
		assert.Equal(t, uri, tr.URI)
	}
}

func TestHandleGeneratedResource(t *testing.T) {
	// even => text
	even := "test://static/resource/2"
	res, err := handleGeneratedResource(context.Background(), mcp.ReadResourceRequest{Params: mcp.ReadResourceParams{URI: even}})
	assert.NoError(t, err)
	assert.Len(t, res, 1)
	// just ensure content returned
	assert.NotNil(t, res[0])

	// odd => blob
	odd := "test://static/resource/3"
	res, err = handleGeneratedResource(context.Background(), mcp.ReadResourceRequest{Params: mcp.ReadResourceParams{URI: odd}})
	assert.NoError(t, err)
	assert.Len(t, res, 1)
	assert.NotNil(t, res[0])
}

func TestHandleEchoAndAddTools(t *testing.T) {
	// echo
	args, _ := json.Marshal(map[string]any{"message": "hello"})
	req := mcp.CallToolRequest{Params: mcp.CallToolParams{Arguments: json.RawMessage(args)}}
	out, err := handleEchoTool(context.Background(), req)
	assert.NoError(t, err)
	assert.NotEmpty(t, out.Content)

	// add
	args2, _ := json.Marshal(map[string]any{"a": 1, "b": 2})
	req2 := mcp.CallToolRequest{Params: mcp.CallToolParams{Arguments: json.RawMessage(args2)}}
	out2, err := handleAddTool(context.Background(), req2)
	assert.NoError(t, err)
	assert.NotEmpty(t, out2.Content)
}

func TestHandleGetTinyImageTool(t *testing.T) {
	out, err := handleGetTinyImageTool(context.Background(), mcp.CallToolRequest{})
	assert.NoError(t, err)
	assert.GreaterOrEqual(t, len(out.Content), 3)
}
