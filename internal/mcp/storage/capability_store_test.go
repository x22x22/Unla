package storage

import (
	"context"
	"testing"
	"time"

	"github.com/amoylab/unla/pkg/mcp"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

func setupTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	err = db.AutoMigrate(&MCPToolModel{}, &MCPPromptModel{}, &MCPResourceModel{}, &MCPResourceTemplateModel{})
	require.NoError(t, err)

	return db
}

func TestMCPToolOperations(t *testing.T) {
	db := setupTestDB(t)
	logger := zap.NewNop()
	store := NewDBCapabilityStore(logger, db)

	ctx := context.Background()
	tenant := "test-tenant"
	serverName := "test-server"

	// Create test tool
	tool := &mcp.MCPTool{
		Name:        "test-tool",
		Description: "A test tool",
		InputSchema: mcp.ToolInputSchema{
			Type:  "object",
			Title: "Test Tool Schema",
			Properties: map[string]any{
				"param1": map[string]any{
					"type": "string",
				},
			},
			Required: []string{"param1"},
		},
		Annotations: &mcp.ToolAnnotations{
			Title:          "Test Tool",
			ReadOnlyHint:   true,
			IdempotentHint: true,
		},
		Enabled:    true,
		LastSynced: time.Now().Format(time.RFC3339),
	}

	// Test SaveTool
	err := store.SaveTool(ctx, tool, tenant, serverName)
	assert.NoError(t, err)

	// Test GetTool
	retrievedTool, err := store.GetTool(ctx, tenant, serverName, tool.Name)
	assert.NoError(t, err)
	assert.Equal(t, tool.Name, retrievedTool.Name)
	assert.Equal(t, tool.Description, retrievedTool.Description)
	assert.Equal(t, tool.Enabled, retrievedTool.Enabled)
	assert.Equal(t, tool.InputSchema.Type, retrievedTool.InputSchema.Type)
	assert.Equal(t, tool.Annotations.Title, retrievedTool.Annotations.Title)

	// Test ListTools
	tools, err := store.ListTools(ctx, tenant, serverName)
	assert.NoError(t, err)
	assert.Len(t, tools, 1)
	assert.Equal(t, tool.Name, tools[0].Name)

	// Test UpdateTool
	tool.Description = "Updated description"
	err = store.SaveTool(ctx, tool, tenant, serverName)
	assert.NoError(t, err)

	updatedTool, err := store.GetTool(ctx, tenant, serverName, tool.Name)
	assert.NoError(t, err)
	assert.Equal(t, "Updated description", updatedTool.Description)

	// Test DeleteTool
	err = store.DeleteTool(ctx, tenant, serverName, tool.Name)
	assert.NoError(t, err)

	_, err = store.GetTool(ctx, tenant, serverName, tool.Name)
	assert.Error(t, err)
	assert.Equal(t, gorm.ErrRecordNotFound, err)
}

func TestMCPPromptOperations(t *testing.T) {
	db := setupTestDB(t)
	logger := zap.NewNop()
	store := NewDBCapabilityStore(logger, db)

	ctx := context.Background()
	tenant := "test-tenant"
	serverName := "test-server"

	// Create test prompt
	prompt := &mcp.MCPPrompt{
		Name:        "test-prompt",
		Description: "A test prompt",
		Arguments: []mcp.PromptArgumentSchema{
			{
				Name:        "arg1",
				Description: "First argument",
				Required:    true,
			},
		},
		PromptResponse: []mcp.PromptResponseSchema{
			{
				Role: "user",
				Content: mcp.PromptResponseContentSchema{
					Type: "text",
					Text: "Test response",
				},
			},
		},
		LastSynced: time.Now().Format(time.RFC3339),
	}

	// Test SavePrompt
	err := store.SavePrompt(ctx, prompt, tenant, serverName)
	assert.NoError(t, err)

	// Test GetPrompt
	retrievedPrompt, err := store.GetPrompt(ctx, tenant, serverName, prompt.Name)
	assert.NoError(t, err)
	assert.Equal(t, prompt.Name, retrievedPrompt.Name)
	assert.Equal(t, prompt.Description, retrievedPrompt.Description)
	assert.Len(t, retrievedPrompt.Arguments, 1)
	assert.Equal(t, "arg1", retrievedPrompt.Arguments[0].Name)
	assert.Len(t, retrievedPrompt.PromptResponse, 1)
	assert.Equal(t, "user", retrievedPrompt.PromptResponse[0].Role)

	// Test ListPrompts
	prompts, err := store.ListPrompts(ctx, tenant, serverName)
	assert.NoError(t, err)
	assert.Len(t, prompts, 1)
	assert.Equal(t, prompt.Name, prompts[0].Name)

	// Test DeletePrompt
	err = store.DeletePrompt(ctx, tenant, serverName, prompt.Name)
	assert.NoError(t, err)

	_, err = store.GetPrompt(ctx, tenant, serverName, prompt.Name)
	assert.Error(t, err)
	assert.Equal(t, gorm.ErrRecordNotFound, err)
}

func TestMCPResourceOperations(t *testing.T) {
	db := setupTestDB(t)
	logger := zap.NewNop()
	store := NewDBCapabilityStore(logger, db)

	ctx := context.Background()
	tenant := "test-tenant"
	serverName := "test-server"

	// Create test resource
	resource := &mcp.MCPResource{
		URI:         "file:///test/resource.txt",
		Name:        "test-resource",
		Description: "A test resource",
		MIMEType:    "text/plain",
		LastSynced:  time.Now().Format(time.RFC3339),
	}

	// Test SaveResource
	err := store.SaveResource(ctx, resource, tenant, serverName)
	assert.NoError(t, err)

	// Test GetResource
	retrievedResource, err := store.GetResource(ctx, tenant, serverName, resource.URI)
	assert.NoError(t, err)
	assert.Equal(t, resource.URI, retrievedResource.URI)
	assert.Equal(t, resource.Name, retrievedResource.Name)
	assert.Equal(t, resource.Description, retrievedResource.Description)
	assert.Equal(t, resource.MIMEType, retrievedResource.MIMEType)

	// Test ListResources
	resources, err := store.ListResources(ctx, tenant, serverName)
	assert.NoError(t, err)
	assert.Len(t, resources, 1)
	assert.Equal(t, resource.URI, resources[0].URI)

	// Test DeleteResource
	err = store.DeleteResource(ctx, tenant, serverName, resource.URI)
	assert.NoError(t, err)

	_, err = store.GetResource(ctx, tenant, serverName, resource.URI)
	assert.Error(t, err)
	assert.Equal(t, gorm.ErrRecordNotFound, err)
}

func TestMCPResourceTemplateOperations(t *testing.T) {
	db := setupTestDB(t)
	logger := zap.NewNop()
	store := NewDBCapabilityStore(logger, db)

	ctx := context.Background()
	tenant := "test-tenant"
	serverName := "test-server"

	// Create test resource template
	template := &mcp.MCPResourceTemplate{
		URITemplate: "file:///test/{filename}",
		Name:        "test-template",
		Description: "A test resource template",
		MIMEType:    "text/plain",
		Parameters: []mcp.ResourceTemplateParameterSchema{
			{
				Name:        "filename",
				Description: "Name of the file",
				Required:    true,
				Type:        "string",
			},
		},
		LastSynced: time.Now().Format(time.RFC3339),
	}

	// Test SaveResourceTemplate
	err := store.SaveResourceTemplate(ctx, template, tenant, serverName)
	assert.NoError(t, err)

	// Test GetResourceTemplate
	retrievedTemplate, err := store.GetResourceTemplate(ctx, tenant, serverName, template.URITemplate)
	assert.NoError(t, err)
	assert.Equal(t, template.URITemplate, retrievedTemplate.URITemplate)
	assert.Equal(t, template.Name, retrievedTemplate.Name)
	assert.Equal(t, template.Description, retrievedTemplate.Description)
	assert.Equal(t, template.MIMEType, retrievedTemplate.MIMEType)
	assert.Len(t, retrievedTemplate.Parameters, 1)
	assert.Equal(t, "filename", retrievedTemplate.Parameters[0].Name)

	// Test ListResourceTemplates
	templates, err := store.ListResourceTemplates(ctx, tenant, serverName)
	assert.NoError(t, err)
	assert.Len(t, templates, 1)
	assert.Equal(t, template.URITemplate, templates[0].URITemplate)

	// Test DeleteResourceTemplate
	err = store.DeleteResourceTemplate(ctx, tenant, serverName, template.URITemplate)
	assert.NoError(t, err)

	_, err = store.GetResourceTemplate(ctx, tenant, serverName, template.URITemplate)
	assert.Error(t, err)
	assert.Equal(t, gorm.ErrRecordNotFound, err)
}

func TestSyncCapabilities(t *testing.T) {
	db := setupTestDB(t)
	logger := zap.NewNop()
	store := NewDBCapabilityStore(logger, db)

	ctx := context.Background()
	tenant := "test-tenant"
	serverName := "test-server"

	// Create test capabilities
	info := &mcp.CapabilitiesInfo{
		Tools: []mcp.MCPTool{
			{
				Name:        "tool1",
				Description: "Tool 1",
				InputSchema: mcp.ToolInputSchema{Type: "object", Title: "Tool 1 Schema"},
				Enabled:     true,
			},
			{
				Name:        "tool2",
				Description: "Tool 2",
				InputSchema: mcp.ToolInputSchema{Type: "object", Title: "Tool 2 Schema"},
				Enabled:     false,
			},
		},
		Prompts: []mcp.MCPPrompt{
			{
				Name:        "prompt1",
				Description: "Prompt 1",
				Arguments:   []mcp.PromptArgumentSchema{},
			},
		},
		Resources: []mcp.MCPResource{
			{
				URI:         "file:///resource1.txt",
				Name:        "Resource 1",
				Description: "Resource 1",
				MIMEType:    "text/plain",
			},
		},
		ResourceTemplates: []mcp.MCPResourceTemplate{
			{
				URITemplate: "file:///template/{id}",
				Name:        "Template 1",
				Description: "Template 1",
				MIMEType:    "text/plain",
				Parameters:  []mcp.ResourceTemplateParameterSchema{},
			},
		},
		LastSynced: time.Now().Format(time.RFC3339),
	}

	// Test SyncCapabilities
	err := store.SyncCapabilities(ctx, info, tenant, serverName)
	assert.NoError(t, err)

	// Verify all capabilities were synced
	tools, err := store.ListTools(ctx, tenant, serverName)
	assert.NoError(t, err)
	assert.Len(t, tools, 2)

	prompts, err := store.ListPrompts(ctx, tenant, serverName)
	assert.NoError(t, err)
	assert.Len(t, prompts, 1)

	resources, err := store.ListResources(ctx, tenant, serverName)
	assert.NoError(t, err)
	assert.Len(t, resources, 1)

	templates, err := store.ListResourceTemplates(ctx, tenant, serverName)
	assert.NoError(t, err)
	assert.Len(t, templates, 1)

	// Test GetCapabilitiesInfo
	retrievedInfo, err := store.GetCapabilitiesInfo(ctx, tenant, serverName)
	assert.NoError(t, err)
	assert.Len(t, retrievedInfo.Tools, 2)
	assert.Len(t, retrievedInfo.Prompts, 1)
	assert.Len(t, retrievedInfo.Resources, 1)
	assert.Len(t, retrievedInfo.ResourceTemplates, 1)

	// Test CleanupServerCapabilities
	err = store.CleanupServerCapabilities(ctx, tenant, serverName)
	assert.NoError(t, err)

	// Verify all capabilities were cleaned up
	emptyInfo, err := store.GetCapabilitiesInfo(ctx, tenant, serverName)
	assert.NoError(t, err)
	assert.Len(t, emptyInfo.Tools, 0)
	assert.Len(t, emptyInfo.Prompts, 0)
	assert.Len(t, emptyInfo.Resources, 0)
	assert.Len(t, emptyInfo.ResourceTemplates, 0)
}

func TestMultiTenantSupport(t *testing.T) {
	db := setupTestDB(t)
	logger := zap.NewNop()
	store := NewDBCapabilityStore(logger, db)

	ctx := context.Background()

	// Create tools for different tenants
	tool1 := &mcp.MCPTool{
		Name:        "shared-tool",
		Description: "Tool for tenant 1",
		InputSchema: mcp.ToolInputSchema{Type: "object", Title: "Tool Schema"},
		Enabled:     true,
	}

	tool2 := &mcp.MCPTool{
		Name:        "shared-tool",
		Description: "Tool for tenant 2",
		InputSchema: mcp.ToolInputSchema{Type: "object", Title: "Tool Schema"},
		Enabled:     false,
	}

	// Save tools for different tenants
	err := store.SaveTool(ctx, tool1, "tenant1", "server1")
	assert.NoError(t, err)

	err = store.SaveTool(ctx, tool2, "tenant2", "server1")
	assert.NoError(t, err)

	// Verify tenant isolation
	tools1, err := store.ListTools(ctx, "tenant1", "server1")
	assert.NoError(t, err)
	assert.Len(t, tools1, 1)
	assert.Equal(t, "Tool for tenant 1", tools1[0].Description)
	assert.True(t, tools1[0].Enabled)

	tools2, err := store.ListTools(ctx, "tenant2", "server1")
	assert.NoError(t, err)
	assert.Len(t, tools2, 1)
	assert.Equal(t, "Tool for tenant 2", tools2[0].Description)
	assert.False(t, tools2[0].Enabled)

	// Verify cross-tenant operations don't interfere
	err = store.DeleteTool(ctx, "tenant1", "server1", "shared-tool")
	assert.NoError(t, err)

	// Tenant1 tool should be deleted
	_, err = store.GetTool(ctx, "tenant1", "server1", "shared-tool")
	assert.Error(t, err)

	// Tenant2 tool should still exist
	tool, err := store.GetTool(ctx, "tenant2", "server1", "shared-tool")
	assert.NoError(t, err)
	assert.Equal(t, "Tool for tenant 2", tool.Description)
}