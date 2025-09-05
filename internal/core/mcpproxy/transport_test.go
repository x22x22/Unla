package mcpproxy

import (
	"context"
	"testing"

	"github.com/amoylab/unla/internal/common/config"
	"github.com/amoylab/unla/pkg/mcp"
	"github.com/stretchr/testify/assert"
)

// TestResourceTemplateSchema tests the ResourceTemplateSchema structure
func TestResourceTemplateSchema(t *testing.T) {
	schema := mcp.ResourceTemplateSchema{
		URITemplate: "test://resource/{id}",
		Name:        "Test Resource Template",
		Description: "A test resource template",
		MIMEType:    "application/json",
		Parameters: []mcp.ResourceTemplateParameterSchema{
			{
				Name:        "id",
				Description: "Resource identifier",
				Required:    true,
				Type:        "string",
			},
		},
	}

	assert.Equal(t, "test://resource/{id}", schema.URITemplate)
	assert.Equal(t, "Test Resource Template", schema.Name)
	assert.Equal(t, "A test resource template", schema.Description)
	assert.Equal(t, "application/json", schema.MIMEType)
	assert.Len(t, schema.Parameters, 1)
	assert.Equal(t, "id", schema.Parameters[0].Name)
	assert.True(t, schema.Parameters[0].Required)
}

// TestTransportInterface ensures all transport types implement the interface
func TestTransportInterface(t *testing.T) {
	cfg := config.MCPServerConfig{
		Type: "stdio",
		Command: "echo",
		Args: []string{"test"},
	}

	// Test stdio transport
	stdioTransport := &StdioTransport{cfg: cfg}
	var _ Transport = stdioTransport

	// Test SSE transport
	sseTransport := &SSETransport{cfg: cfg}
	var _ Transport = sseTransport

	// Test streamable transport  
	streamableTransport := &StreamableTransport{cfg: cfg}
	var _ Transport = streamableTransport
}

// TestFetchResourceTemplatesInterface verifies method signatures
func TestFetchResourceTemplatesInterface(t *testing.T) {
	ctx := context.Background()
	cfg := config.MCPServerConfig{
		Type: "stdio",
		Command: "echo",
		Args: []string{"test"},
	}

	// Test that method exists on stdio transport
	stdioTransport := &StdioTransport{cfg: cfg}
	_, err := stdioTransport.FetchResourceTemplates(ctx)
	// We expect an error since we're not connecting to a real MCP server
	assert.Error(t, err)

	// Test that method exists on SSE transport
	sseTransport := &SSETransport{cfg: cfg}
	_, err = sseTransport.FetchResourceTemplates(ctx)
	assert.Error(t, err)

	// Test that method exists on streamable transport
	streamableTransport := &StreamableTransport{cfg: cfg}
	_, err = streamableTransport.FetchResourceTemplates(ctx)
	assert.Error(t, err)
}

// TestFetchPromptsInterface verifies FetchPrompts method signatures
func TestFetchPromptsInterface(t *testing.T) {
	ctx := context.Background()
	cfg := config.MCPServerConfig{
		Type: "stdio",
		Command: "echo",
		Args: []string{"test"},
	}

	// Test that method exists on stdio transport
	stdioTransport := &StdioTransport{cfg: cfg}
	_, err := stdioTransport.FetchPrompts(ctx)
	// We expect an error since we're not connecting to a real MCP server
	assert.Error(t, err)

	// Test that method exists on SSE transport
	sseTransport := &SSETransport{cfg: cfg}
	_, err = sseTransport.FetchPrompts(ctx)
	assert.Error(t, err)

	// Test that method exists on streamable transport
	streamableTransport := &StreamableTransport{cfg: cfg}
	_, err = streamableTransport.FetchPrompts(ctx)
	assert.Error(t, err)
}

// TestPromptSchema tests the PromptSchema structure
func TestPromptSchema(t *testing.T) {
	schema := mcp.PromptSchema{
		Name:        "test-prompt",
		Description: "A test prompt",
		Arguments: []mcp.PromptArgumentSchema{
			{
				Name:        "param1",
				Description: "First parameter",
				Required:    true,
			},
			{
				Name:        "param2",
				Description: "Second parameter",
				Required:    false,
			},
		},
	}

	assert.Equal(t, "test-prompt", schema.Name)
	assert.Equal(t, "A test prompt", schema.Description)
	assert.Len(t, schema.Arguments, 2)
	assert.Equal(t, "param1", schema.Arguments[0].Name)
	assert.True(t, schema.Arguments[0].Required)
	assert.Equal(t, "param2", schema.Arguments[1].Name)
	assert.False(t, schema.Arguments[1].Required)
}

// TestMCPDataStructures tests the new MCP capability data structures
func TestMCPDataStructures(t *testing.T) {
	// Test MCPTool
	mcpTool := mcp.MCPTool{
		Name:        "test-tool",
		Description: "A test tool",
		InputSchema: mcp.ToolInputSchema{
			Type:       "object",
			Properties: map[string]any{"param1": "string"},
			Required:   []string{"param1"},
			Title:      "Test Tool",
		},
		Enabled:    true,
		LastSynced: "2025-09-05T10:00:00Z",
	}
	
	assert.Equal(t, "test-tool", mcpTool.Name)
	assert.True(t, mcpTool.Enabled)
	assert.Equal(t, "object", mcpTool.InputSchema.Type)

	// Test MCPResource
	mcpResource := mcp.MCPResource{
		URI:         "test://resource/1",
		Name:        "Test Resource",
		Description: "A test resource",
		MIMEType:    "application/json",
		LastSynced:  "2025-09-05T10:00:00Z",
	}
	
	assert.Equal(t, "test://resource/1", mcpResource.URI)
	assert.Equal(t, "Test Resource", mcpResource.Name)

	// Test MCPResourceTemplate
	mcpTemplate := mcp.MCPResourceTemplate{
		URITemplate: "test://resource/{id}",
		Name:        "Test Template",
		Description: "A test template",
		Parameters: []mcp.ResourceTemplateParameterSchema{
			{
				Name:        "id",
				Description: "Resource ID",
				Required:    true,
				Type:        "string",
			},
		},
		LastSynced: "2025-09-05T10:00:00Z",
	}
	
	assert.Equal(t, "test://resource/{id}", mcpTemplate.URITemplate)
	assert.Len(t, mcpTemplate.Parameters, 1)

	// Test CapabilitiesInfo
	capabilities := mcp.CapabilitiesInfo{
		Tools:             []mcp.MCPTool{mcpTool},
		Resources:         []mcp.MCPResource{mcpResource},
		ResourceTemplates: []mcp.MCPResourceTemplate{mcpTemplate},
		LastSynced:        "2025-09-05T10:00:00Z",
		ServerInfo:        map[string]interface{}{"version": "1.0.0"},
	}
	
	assert.Len(t, capabilities.Tools, 1)
	assert.Len(t, capabilities.Resources, 1)
	assert.Len(t, capabilities.ResourceTemplates, 1)
	assert.Equal(t, "1.0.0", capabilities.ServerInfo["version"])
}