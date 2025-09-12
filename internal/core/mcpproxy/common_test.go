package mcpproxy

import (
	"testing"

	"github.com/amoylab/unla/pkg/mcp"
	mcpgo "github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
)

func TestConvertMCPGoResult_ContentMapping(t *testing.T) {
	res := &mcpgo.CallToolResult{IsError: false}
	res.Content = []mcpgo.Content{
		&mcpgo.TextContent{Text: "hello"},
		&mcpgo.ImageContent{Data: "IMG", MIMEType: "image/png"},
		&mcpgo.AudioContent{Data: "AUD", MIMEType: "audio/wav"},
		nil,
	}

	out := convertMCPGoResult(res)
	assert.False(t, out.IsError)
	if assert.Len(t, out.Content, 3) { // nil filtered
		// Just assert types in order
		_, ok0 := out.Content[0].(*mcp.TextContent)
		_, ok1 := out.Content[1].(*mcp.ImageContent)
		_, ok2 := out.Content[2].(*mcp.AudioContent)
		assert.True(t, ok0 && ok1 && ok2)
	}
}
