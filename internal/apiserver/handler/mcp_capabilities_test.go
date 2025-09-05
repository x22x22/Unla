package handler

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/amoylab/unla/pkg/mcp"
	"go.uber.org/zap"
)

func TestCachedCapabilities_IsExpired(t *testing.T) {
	// Test not expired
	cached := &cachedCapabilities{
		data: &mcp.CapabilitiesInfo{
			Tools: []mcp.MCPTool{},
		},
		timestamp: time.Now(),
		ttl:       5 * time.Minute,
	}
	assert.False(t, cached.isExpired())

	// Test expired
	cached = &cachedCapabilities{
		data: &mcp.CapabilitiesInfo{
			Tools: []mcp.MCPTool{},
		},
		timestamp: time.Now().Add(-10 * time.Minute),
		ttl:       5 * time.Minute,
	}
	assert.True(t, cached.isExpired())
}

func TestClearCapabilitiesCache(t *testing.T) {
	// Create handler with nil dependencies for this simple test
	logger := zap.NewNop()
	handler := NewMCP(nil, nil, nil, logger)

	// Add some cache entries
	cached1 := &cachedCapabilities{
		data:      &mcp.CapabilitiesInfo{Tools: []mcp.MCPTool{}},
		timestamp: time.Now(),
		ttl:       5 * time.Minute,
	}
	cached2 := &cachedCapabilities{
		data:      &mcp.CapabilitiesInfo{Tools: []mcp.MCPTool{}},
		timestamp: time.Now(),
		ttl:       5 * time.Minute,
	}

	handler.capabilitiesCache.Store("tenant1:server1", cached1)
	handler.capabilitiesCache.Store("tenant2:server2", cached2)

	// Test clearing specific key
	handler.clearCapabilitiesCache("tenant1:server1")
	_, exists1 := handler.capabilitiesCache.Load("tenant1:server1")
	_, exists2 := handler.capabilitiesCache.Load("tenant2:server2")
	assert.False(t, exists1)
	assert.True(t, exists2)

	// Test clearing all cache
	handler.clearCapabilitiesCache("")
	_, exists2 = handler.capabilitiesCache.Load("tenant2:server2")
	assert.False(t, exists2)
}

func TestCapabilitiesInfo_Structure(t *testing.T) {
	// Test that CapabilitiesInfo has the expected structure
	capabilities := &mcp.CapabilitiesInfo{
		Tools: []mcp.MCPTool{
			{
				Name:        "test-tool",
				Description: "A test tool",
				Enabled:     true,
				LastSynced:  time.Now().UTC().Format(time.RFC3339),
			},
		},
		Prompts: []mcp.MCPPrompt{
			{
				Name:        "test-prompt",
				Description: "A test prompt",
				LastSynced:  time.Now().UTC().Format(time.RFC3339),
			},
		},
		Resources: []mcp.MCPResource{
			{
				URI:         "test://resource",
				Name:        "test-resource",
				Description: "A test resource",
				LastSynced:  time.Now().UTC().Format(time.RFC3339),
			},
		},
		ResourceTemplates: []mcp.MCPResourceTemplate{
			{
				URITemplate: "test://template/{id}",
				Name:        "test-template",
				Description: "A test template",
				LastSynced:  time.Now().UTC().Format(time.RFC3339),
			},
		},
		LastSynced: time.Now().UTC().Format(time.RFC3339),
		ServerInfo: map[string]interface{}{
			"name":    "test-server",
			"version": "1.0.0",
		},
	}

	// Basic structure assertions
	assert.NotNil(t, capabilities.Tools)
	assert.NotNil(t, capabilities.Prompts)
	assert.NotNil(t, capabilities.Resources)
	assert.NotNil(t, capabilities.ResourceTemplates)
	assert.NotEmpty(t, capabilities.LastSynced)
	assert.NotNil(t, capabilities.ServerInfo)

	// Content assertions
	assert.Len(t, capabilities.Tools, 1)
	assert.Equal(t, "test-tool", capabilities.Tools[0].Name)
	assert.True(t, capabilities.Tools[0].Enabled)

	assert.Len(t, capabilities.Prompts, 1)
	assert.Equal(t, "test-prompt", capabilities.Prompts[0].Name)

	assert.Len(t, capabilities.Resources, 1)
	assert.Equal(t, "test://resource", capabilities.Resources[0].URI)

	assert.Len(t, capabilities.ResourceTemplates, 1)
	assert.Equal(t, "test://template/{id}", capabilities.ResourceTemplates[0].URITemplate)

	assert.Equal(t, "test-server", capabilities.ServerInfo["name"])
}