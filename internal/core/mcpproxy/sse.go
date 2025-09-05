package mcpproxy

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"

	"github.com/amoylab/unla/internal/common/cnst"

	"github.com/amoylab/unla/internal/common/config"
	"github.com/amoylab/unla/internal/template"
	"github.com/amoylab/unla/pkg/mcp"
	"github.com/amoylab/unla/pkg/version"
	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/client/transport"
	mcpgo "github.com/mark3labs/mcp-go/mcp"
)

// SSETransport implements Transport using Server-Sent Events
type SSETransport struct {
	client *client.Client
	cfg    config.MCPServerConfig
	mu     sync.Mutex // Protects client operations in on-demand mode
}

var _ Transport = (*SSETransport)(nil)

func (t *SSETransport) Start(ctx context.Context, tmplCtx *template.Context) error {
	if t.IsRunning() {
		return nil
	}

	// Create SSE transport
	sseTransport, err := transport.NewSSE(t.cfg.URL)
	if err != nil {
		return fmt.Errorf("failed to create SSE transport: %w", err)
	}

	// Start the transport
	if err := sseTransport.Start(ctx); err != nil {
		return fmt.Errorf("failed to start SSE transport: %w", err)
	}

	// Create client with the transport
	c := client.NewClient(sseTransport)

	// Initialize the client
	initRequest := mcpgo.InitializeRequest{}
	initRequest.Params.ProtocolVersion = mcpgo.LATEST_PROTOCOL_VERSION
	initRequest.Params.ClientInfo = mcpgo.Implementation{
		Name:    cnst.AppName,
		Version: version.Get(),
	}

	_, err = c.Initialize(ctx, initRequest)
	if err != nil {
		_ = sseTransport.Close()
		return fmt.Errorf("failed to initialize SSE client: %w", err)
	}

	t.client = c
	return nil
}

func (t *SSETransport) Stop(_ context.Context) error {
	if !t.IsRunning() {
		return nil
	}

	if t.client != nil {
		err := t.client.Close()
		if err != nil {
			return err
		}
		t.client = nil
	}

	return nil
}

func (t *SSETransport) IsRunning() bool {
	return t.client != nil
}

func (t *SSETransport) FetchTools(ctx context.Context) ([]mcp.ToolSchema, error) {
	if t.cfg.Policy == cnst.PolicyOnDemand {
		t.mu.Lock()
		defer t.mu.Unlock()
	}
	
	if !t.IsRunning() {
		if err := t.Start(ctx, nil); err != nil {
			return nil, err
		}
	}

	// List available tools
	toolsResult, err := t.client.ListTools(ctx, mcpgo.ListToolsRequest{})
	if err != nil {
		return nil, fmt.Errorf("failed to list tools: %w", err)
	}

	// Convert from mcpgo.Tool to mcp.ToolSchema
	tools := make([]mcp.ToolSchema, len(toolsResult.Tools))
	for i, schema := range toolsResult.Tools {
		// Create local mcp package ToolInputSchema
		inputSchema := mcp.ToolInputSchema{
			Type:       "object",
			Properties: make(map[string]any),
		}

		// Convert mcpgo InputSchema to local mcp format
		rawSchema, err := json.Marshal(schema.InputSchema)
		if err == nil {
			// Parse schema properties
			var schemaMap map[string]interface{}
			if err := json.Unmarshal(rawSchema, &schemaMap); err == nil {
				if properties, ok := schemaMap["properties"].(map[string]interface{}); ok {
					inputSchema.Properties = properties
				}
				if typ, ok := schemaMap["type"].(string); ok {
					inputSchema.Type = typ
				}
				if required, ok := schemaMap["required"].([]interface{}); ok {
					reqStrings := make([]string, len(required))
					for j, r := range required {
						if rStr, ok := r.(string); ok {
							reqStrings[j] = rStr
						}
					}
					inputSchema.Required = reqStrings
				}
			}
		}

		tools[i] = mcp.ToolSchema{
			Name:        schema.Name,
			Description: schema.Description,
			InputSchema: inputSchema,
		}
	}

	if t.cfg.Policy == cnst.PolicyOnDemand {
		t.Stop(ctx)
	}
	return tools, nil
}

func (t *SSETransport) CallTool(ctx context.Context, params mcp.CallToolParams, req *template.RequestWrapper) (*mcp.CallToolResult, error) {
	if !t.IsRunning() {
		if err := t.Start(ctx, nil); err != nil {
			return nil, err
		}
	}

	// Convert arguments to map[string]any
	var args map[string]any
	if err := json.Unmarshal(params.Arguments, &args); err != nil {
		return nil, fmt.Errorf("invalid tool arguments: %w", err)
	}

	// Prepare template context for environment variables
	tmplCtx, err := template.AssembleTemplateContext(req, args, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare template context: %w", err)
	}

	// Process environment variables with templates
	renderedClientEnv := make(map[string]string)
	for k, v := range t.cfg.Env {
		rendered, err := template.RenderTemplate(v, tmplCtx)
		if err != nil {
			return nil, fmt.Errorf("failed to render env template: %w", err)
		}
		renderedClientEnv[k] = rendered
	}

	// Prepare tool call request parameters
	toolCallRequestParams := make(map[string]interface{})
	if err := json.Unmarshal(params.Arguments, &toolCallRequestParams); err != nil {
		return nil, fmt.Errorf("failed to unmarshal tool arguments: %w", err)
	}

	// Call tool
	callRequest := mcpgo.CallToolRequest{}
	callRequest.Params.Name = params.Name
	callRequest.Params.Arguments = toolCallRequestParams

	mcpResult, err := t.client.CallTool(ctx, callRequest)
	if err != nil {
		return nil, fmt.Errorf("failed to call tool: %w", err)
	}

	if t.cfg.Policy == cnst.PolicyOnDemand {
		t.Stop(ctx)
	}
	return convertMCPGoResult(mcpResult), nil
}

// FetchPrompts returns all prompts
func (t *SSETransport) FetchPrompts(ctx context.Context) ([]mcp.PromptSchema, error) {
	if t.cfg.Policy == cnst.PolicyOnDemand {
		t.mu.Lock()
		defer t.mu.Unlock()
	}
	
	if !t.IsRunning() {
		if err := t.Start(ctx, nil); err != nil {
			return nil, err
		}
	}

	// List available prompts
	promptsResult, err := t.client.ListPrompts(ctx, mcpgo.ListPromptsRequest{})
	if err != nil {
		return nil, fmt.Errorf("failed to list prompts: %w", err)
	}

	// Convert from mcpgo.Prompt to mcp.PromptSchema
	prompts := make([]mcp.PromptSchema, len(promptsResult.Prompts))
	for i, prompt := range promptsResult.Prompts {
		// Convert arguments
		arguments := make([]mcp.PromptArgumentSchema, len(prompt.Arguments))
		for j, arg := range prompt.Arguments {
			arguments[j] = mcp.PromptArgumentSchema{
				Name:        arg.Name,
				Description: arg.Description,
				Required:    arg.Required,
			}
		}

		prompts[i] = mcp.PromptSchema{
			Name:        prompt.Name,
			Description: prompt.Description,
			Arguments:   arguments,
		}
	}

	if t.cfg.Policy == cnst.PolicyOnDemand {
		t.Stop(ctx)
	}
	return prompts, nil
}

// FetchPrompt returns a specific prompt by name
func (t *SSETransport) FetchPrompt(ctx context.Context, name string) (*mcp.PromptSchema, error) {
	return nil, &HTTPError{
		StatusCode: http.StatusNotFound,
		Message:    "Prompt not found",
	}
}

// FetchResources fetches the list of available resources
func (t *SSETransport) FetchResources(ctx context.Context) ([]mcp.ResourceSchema, error) {
	if t.cfg.Policy == cnst.PolicyOnDemand {
		t.mu.Lock()
		defer t.mu.Unlock()
	}
	
	if !t.IsRunning() {
		if err := t.Start(ctx, nil); err != nil {
			return nil, err
		}
	}

	// List available resources
	resourcesResult, err := t.client.ListResources(ctx, mcpgo.ListResourcesRequest{})
	if err != nil {
		return nil, fmt.Errorf("failed to list resources: %w", err)
	}

	// Convert from mcpgo.Resource to mcp.ResourceSchema
	resources := make([]mcp.ResourceSchema, len(resourcesResult.Resources))
	for i, resource := range resourcesResult.Resources {
		resources[i] = mcp.ResourceSchema{
			Name:        resource.Name,
			Description: resource.Description,
			URI:         resource.URI,
			MIMEType:    resource.MIMEType,
		}
	}

	if t.cfg.Policy == cnst.PolicyOnDemand {
		t.Stop(ctx)
	}
	return resources, nil
}

// FetchResourceTemplates fetches the list of available resource templates
func (t *SSETransport) FetchResourceTemplates(ctx context.Context) ([]mcp.ResourceTemplateSchema, error) {
	if t.cfg.Policy == cnst.PolicyOnDemand {
		t.mu.Lock()
		defer t.mu.Unlock()
	}
	
	if !t.IsRunning() {
		if err := t.Start(ctx, nil); err != nil {
			return nil, err
		}
	}

	// List available resource templates
	resourceTemplatesResult, err := t.client.ListResourceTemplates(ctx, mcpgo.ListResourceTemplatesRequest{})
	if err != nil {
		return nil, fmt.Errorf("failed to list resource templates: %w", err)
	}

	// Convert from mcpgo.ResourceTemplate to mcp.ResourceTemplateSchema
	templates := make([]mcp.ResourceTemplateSchema, len(resourceTemplatesResult.ResourceTemplates))
	for i, template := range resourceTemplatesResult.ResourceTemplates {
		// Extract URI template string from the mcpgo.ResourceTemplate
		uriTemplateString := ""
		if template.URITemplate != nil {
			uriTemplateString = template.URITemplate.Raw()
		}

		templates[i] = mcp.ResourceTemplateSchema{
			URITemplate: uriTemplateString,
			Name:        template.Name,
			Description: template.Description,
			MIMEType:    template.MIMEType,
			// Parameters will be parsed from URI template if needed in the future
		}
	}

	if t.cfg.Policy == cnst.PolicyOnDemand {
		t.Stop(ctx)
	}
	return templates, nil
}