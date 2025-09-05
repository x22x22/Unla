package storage

import (
	"encoding/json"
	"time"

	"github.com/amoylab/unla/pkg/mcp"
	"gorm.io/gorm"
)

// MCPToolModel represents the database model for MCP tools
type MCPToolModel struct {
	ID          uint           `gorm:"primaryKey;autoIncrement"`
	Name        string         `gorm:"column:name;type:varchar(100);not null;index:idx_tool_name_tenant_server,priority:2"`
	Tenant      string         `gorm:"column:tenant;type:varchar(50);not null;index:idx_tool_name_tenant_server,priority:1"`
	ServerName  string         `gorm:"column:server_name;type:varchar(100);not null;index:idx_tool_name_tenant_server,priority:3"`
	Description string         `gorm:"column:description;type:text"`
	InputSchema string         `gorm:"column:input_schema;type:text"` // JSON string
	Annotations string         `gorm:"column:annotations;type:text"`  // JSON string, nullable
	Enabled     bool           `gorm:"column:enabled;not null"`
	LastSynced  time.Time      `gorm:"column:last_synced"`
	CreatedAt   time.Time      `gorm:"column:created_at"`
	UpdatedAt   time.Time      `gorm:"column:updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index"`
}

// TableName returns the table name for MCPToolModel
func (MCPToolModel) TableName() string {
	return "mcp_tools"
}

// ToMCPTool converts database model to MCP tool
func (m *MCPToolModel) ToMCPTool() (*mcp.MCPTool, error) {
	tool := &mcp.MCPTool{
		Name:        m.Name,
		Description: m.Description,
		Enabled:     m.Enabled,
		LastSynced:  m.LastSynced.Format(time.RFC3339),
	}

	// Parse InputSchema JSON
	if m.InputSchema != "" {
		var schema mcp.ToolInputSchema
		if err := json.Unmarshal([]byte(m.InputSchema), &schema); err != nil {
			return nil, err
		}
		tool.InputSchema = schema
	}

	// Parse Annotations JSON if present
	if m.Annotations != "" {
		var annotations mcp.ToolAnnotations
		if err := json.Unmarshal([]byte(m.Annotations), &annotations); err != nil {
			return nil, err
		}
		tool.Annotations = &annotations
	}

	return tool, nil
}

// FromMCPTool converts MCP tool to database model
func FromMCPTool(tool *mcp.MCPTool, tenant, serverName string) (*MCPToolModel, error) {
	// Marshal InputSchema to JSON
	schemaJSON, err := json.Marshal(tool.InputSchema)
	if err != nil {
		return nil, err
	}

	var annotationsJSON string
	if tool.Annotations != nil {
		annotationsBytes, err := json.Marshal(tool.Annotations)
		if err != nil {
			return nil, err
		}
		annotationsJSON = string(annotationsBytes)
	}

	lastSynced := time.Now()
	if tool.LastSynced != "" {
		if parsed, err := time.Parse(time.RFC3339, tool.LastSynced); err == nil {
			lastSynced = parsed
		}
	}

	return &MCPToolModel{
		Name:        tool.Name,
		Tenant:      tenant,
		ServerName:  serverName,
		Description: tool.Description,
		InputSchema: string(schemaJSON),
		Annotations: annotationsJSON,
		Enabled:     tool.Enabled,
		LastSynced:  lastSynced,
	}, nil
}

// MCPPromptModel represents the database model for MCP prompts
type MCPPromptModel struct {
	ID             uint           `gorm:"primaryKey;autoIncrement"`
	Name           string         `gorm:"column:name;type:varchar(100);not null;index:idx_prompt_name_tenant_server,priority:2"`
	Tenant         string         `gorm:"column:tenant;type:varchar(50);not null;index:idx_prompt_name_tenant_server,priority:1"`
	ServerName     string         `gorm:"column:server_name;type:varchar(100);not null;index:idx_prompt_name_tenant_server,priority:3"`
	Description    string         `gorm:"column:description;type:text"`
	Arguments      string         `gorm:"column:arguments;type:text"`      // JSON array of PromptArgumentSchema
	PromptResponse string         `gorm:"column:prompt_response;type:text"` // JSON array of PromptResponseSchema, nullable
	LastSynced     time.Time      `gorm:"column:last_synced"`
	CreatedAt      time.Time      `gorm:"column:created_at"`
	UpdatedAt      time.Time      `gorm:"column:updated_at"`
	DeletedAt      gorm.DeletedAt `gorm:"index"`
}

// TableName returns the table name for MCPPromptModel
func (MCPPromptModel) TableName() string {
	return "mcp_prompts"
}

// ToMCPPrompt converts database model to MCP prompt
func (m *MCPPromptModel) ToMCPPrompt() (*mcp.MCPPrompt, error) {
	prompt := &mcp.MCPPrompt{
		Name:        m.Name,
		Description: m.Description,
		LastSynced:  m.LastSynced.Format(time.RFC3339),
	}

	// Parse Arguments JSON
	if m.Arguments != "" {
		var args []mcp.PromptArgumentSchema
		if err := json.Unmarshal([]byte(m.Arguments), &args); err != nil {
			return nil, err
		}
		prompt.Arguments = args
	}

	// Parse PromptResponse JSON if present
	if m.PromptResponse != "" {
		var responses []mcp.PromptResponseSchema
		if err := json.Unmarshal([]byte(m.PromptResponse), &responses); err != nil {
			return nil, err
		}
		prompt.PromptResponse = responses
	}

	return prompt, nil
}

// FromMCPPrompt converts MCP prompt to database model
func FromMCPPrompt(prompt *mcp.MCPPrompt, tenant, serverName string) (*MCPPromptModel, error) {
	// Marshal Arguments to JSON
	argsJSON, err := json.Marshal(prompt.Arguments)
	if err != nil {
		return nil, err
	}

	var responseJSON string
	if len(prompt.PromptResponse) > 0 {
		responseBytes, err := json.Marshal(prompt.PromptResponse)
		if err != nil {
			return nil, err
		}
		responseJSON = string(responseBytes)
	}

	lastSynced := time.Now()
	if prompt.LastSynced != "" {
		if parsed, err := time.Parse(time.RFC3339, prompt.LastSynced); err == nil {
			lastSynced = parsed
		}
	}

	return &MCPPromptModel{
		Name:           prompt.Name,
		Tenant:         tenant,
		ServerName:     serverName,
		Description:    prompt.Description,
		Arguments:      string(argsJSON),
		PromptResponse: responseJSON,
		LastSynced:     lastSynced,
	}, nil
}

// MCPResourceModel represents the database model for MCP resources
type MCPResourceModel struct {
	ID          uint           `gorm:"primaryKey;autoIncrement"`
	URI         string         `gorm:"column:uri;type:varchar(500);not null;index:idx_resource_uri_tenant_server,priority:2"`
	Tenant      string         `gorm:"column:tenant;type:varchar(50);not null;index:idx_resource_uri_tenant_server,priority:1"`
	ServerName  string         `gorm:"column:server_name;type:varchar(100);not null;index:idx_resource_uri_tenant_server,priority:3"`
	Name        string         `gorm:"column:name;type:varchar(255);not null"`
	Description string         `gorm:"column:description;type:text"`
	MIMEType    string         `gorm:"column:mime_type;type:varchar(100)"`
	LastSynced  time.Time      `gorm:"column:last_synced"`
	CreatedAt   time.Time      `gorm:"column:created_at"`
	UpdatedAt   time.Time      `gorm:"column:updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index"`
}

// TableName returns the table name for MCPResourceModel
func (MCPResourceModel) TableName() string {
	return "mcp_resources"
}

// ToMCPResource converts database model to MCP resource
func (m *MCPResourceModel) ToMCPResource() *mcp.MCPResource {
	return &mcp.MCPResource{
		URI:         m.URI,
		Name:        m.Name,
		Description: m.Description,
		MIMEType:    m.MIMEType,
		LastSynced:  m.LastSynced.Format(time.RFC3339),
	}
}

// FromMCPResource converts MCP resource to database model
func FromMCPResource(resource *mcp.MCPResource, tenant, serverName string) *MCPResourceModel {
	lastSynced := time.Now()
	if resource.LastSynced != "" {
		if parsed, err := time.Parse(time.RFC3339, resource.LastSynced); err == nil {
			lastSynced = parsed
		}
	}

	return &MCPResourceModel{
		URI:         resource.URI,
		Tenant:      tenant,
		ServerName:  serverName,
		Name:        resource.Name,
		Description: resource.Description,
		MIMEType:    resource.MIMEType,
		LastSynced:  lastSynced,
	}
}

// MCPResourceTemplateModel represents the database model for MCP resource templates
type MCPResourceTemplateModel struct {
	ID          uint           `gorm:"primaryKey;autoIncrement"`
	URITemplate string         `gorm:"column:uri_template;type:varchar(500);not null;index:idx_template_uri_tenant_server,priority:2"`
	Tenant      string         `gorm:"column:tenant;type:varchar(50);not null;index:idx_template_uri_tenant_server,priority:1"`
	ServerName  string         `gorm:"column:server_name;type:varchar(100);not null;index:idx_template_uri_tenant_server,priority:3"`
	Name        string         `gorm:"column:name;type:varchar(255);not null"`
	Description string         `gorm:"column:description;type:text"`
	MIMEType    string         `gorm:"column:mime_type;type:varchar(100)"`
	Parameters  string         `gorm:"column:parameters;type:text"` // JSON array of ResourceTemplateParameterSchema
	LastSynced  time.Time      `gorm:"column:last_synced"`
	CreatedAt   time.Time      `gorm:"column:created_at"`
	UpdatedAt   time.Time      `gorm:"column:updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index"`
}

// TableName returns the table name for MCPResourceTemplateModel
func (MCPResourceTemplateModel) TableName() string {
	return "mcp_resource_templates"
}

// ToMCPResourceTemplate converts database model to MCP resource template
func (m *MCPResourceTemplateModel) ToMCPResourceTemplate() (*mcp.MCPResourceTemplate, error) {
	template := &mcp.MCPResourceTemplate{
		URITemplate: m.URITemplate,
		Name:        m.Name,
		Description: m.Description,
		MIMEType:    m.MIMEType,
		LastSynced:  m.LastSynced.Format(time.RFC3339),
	}

	// Parse Parameters JSON
	if m.Parameters != "" {
		var params []mcp.ResourceTemplateParameterSchema
		if err := json.Unmarshal([]byte(m.Parameters), &params); err != nil {
			return nil, err
		}
		template.Parameters = params
	}

	return template, nil
}

// FromMCPResourceTemplate converts MCP resource template to database model
func FromMCPResourceTemplate(template *mcp.MCPResourceTemplate, tenant, serverName string) (*MCPResourceTemplateModel, error) {
	var paramsJSON string
	if len(template.Parameters) > 0 {
		paramsBytes, err := json.Marshal(template.Parameters)
		if err != nil {
			return nil, err
		}
		paramsJSON = string(paramsBytes)
	}

	lastSynced := time.Now()
	if template.LastSynced != "" {
		if parsed, err := time.Parse(time.RFC3339, template.LastSynced); err == nil {
			lastSynced = parsed
		}
	}

	return &MCPResourceTemplateModel{
		URITemplate: template.URITemplate,
		Tenant:      tenant,
		ServerName:  serverName,
		Name:        template.Name,
		Description: template.Description,
		MIMEType:    template.MIMEType,
		Parameters:  paramsJSON,
		LastSynced:  lastSynced,
	}, nil
}

// BeforeCreate is a GORM hook that sets timestamps for all models
func (m *MCPToolModel) BeforeCreate(tx *gorm.DB) error {
	now := time.Now()
	if m.CreatedAt.IsZero() {
		m.CreatedAt = now
	}
	if m.UpdatedAt.IsZero() {
		m.UpdatedAt = now
	}
	return nil
}

func (m *MCPToolModel) BeforeUpdate(tx *gorm.DB) error {
	m.UpdatedAt = time.Now()
	return nil
}

func (m *MCPPromptModel) BeforeCreate(tx *gorm.DB) error {
	now := time.Now()
	if m.CreatedAt.IsZero() {
		m.CreatedAt = now
	}
	if m.UpdatedAt.IsZero() {
		m.UpdatedAt = now
	}
	return nil
}

func (m *MCPPromptModel) BeforeUpdate(tx *gorm.DB) error {
	m.UpdatedAt = time.Now()
	return nil
}

func (m *MCPResourceModel) BeforeCreate(tx *gorm.DB) error {
	now := time.Now()
	if m.CreatedAt.IsZero() {
		m.CreatedAt = now
	}
	if m.UpdatedAt.IsZero() {
		m.UpdatedAt = now
	}
	return nil
}

func (m *MCPResourceModel) BeforeUpdate(tx *gorm.DB) error {
	m.UpdatedAt = time.Now()
	return nil
}

func (m *MCPResourceTemplateModel) BeforeCreate(tx *gorm.DB) error {
	now := time.Now()
	if m.CreatedAt.IsZero() {
		m.CreatedAt = now
	}
	if m.UpdatedAt.IsZero() {
		m.UpdatedAt = now
	}
	return nil
}

func (m *MCPResourceTemplateModel) BeforeUpdate(tx *gorm.DB) error {
	m.UpdatedAt = time.Now()
	return nil
}

// SyncStatus represents synchronization status
type SyncStatus string

const (
	SyncStatusPending    SyncStatus = "pending"
	SyncStatusRunning    SyncStatus = "running"
	SyncStatusSuccess    SyncStatus = "success"
	SyncStatusFailed     SyncStatus = "failed"
	SyncStatusPartial    SyncStatus = "partial"
)

// SyncHistory represents the synchronization history
type SyncHistoryModel struct {
	ID           uint           `gorm:"primaryKey;autoIncrement"`
	Tenant       string         `gorm:"column:tenant;type:varchar(50);not null;index:idx_sync_tenant_server,priority:1"`
	ServerName   string         `gorm:"column:server_name;type:varchar(100);not null;index:idx_sync_tenant_server,priority:2"`
	SyncID       string         `gorm:"column:sync_id;type:varchar(36);not null;uniqueIndex"`
	Status       SyncStatus     `gorm:"column:status;type:varchar(20);not null"`
	SyncTypes    string         `gorm:"column:sync_types;type:text"`        // JSON array of capability types synced
	StartedAt    time.Time      `gorm:"column:started_at"`
	CompletedAt  *time.Time     `gorm:"column:completed_at"`
	ErrorMessage string         `gorm:"column:error_message;type:text"`
	Summary      string         `gorm:"column:summary;type:text"`           // JSON summary of sync results
	Progress     int            `gorm:"column:progress;default:0"`          // Percentage 0-100
	UserID       uint           `gorm:"column:user_id;index"`               // User who initiated the sync
	CreatedAt    time.Time      `gorm:"column:created_at"`
	UpdatedAt    time.Time      `gorm:"column:updated_at"`
	DeletedAt    gorm.DeletedAt `gorm:"index"`
}

// TableName returns the table name for SyncHistoryModel
func (SyncHistoryModel) TableName() string {
	return "mcp_sync_history"
}

// BeforeCreate hook for SyncHistoryModel
func (m *SyncHistoryModel) BeforeCreate(tx *gorm.DB) error {
	now := time.Now()
	if m.CreatedAt.IsZero() {
		m.CreatedAt = now
	}
	if m.UpdatedAt.IsZero() {
		m.UpdatedAt = now
	}
	return nil
}

// BeforeUpdate hook for SyncHistoryModel
func (m *SyncHistoryModel) BeforeUpdate(tx *gorm.DB) error {
	m.UpdatedAt = time.Now()
	return nil
}

// ToolStatusHistory represents the tool status change history
type ToolStatusHistoryModel struct {
	ID         uint           `gorm:"primaryKey;autoIncrement"`
	Tenant     string         `gorm:"column:tenant;type:varchar(50);not null;index:idx_status_history_tenant_server,priority:1"`
	ServerName string         `gorm:"column:server_name;type:varchar(100);not null;index:idx_status_history_tenant_server,priority:2"`
	ToolName   string         `gorm:"column:tool_name;type:varchar(100);not null;index:idx_status_history_tool,priority:1"`
	OldStatus  bool           `gorm:"column:old_status"`
	NewStatus  bool           `gorm:"column:new_status"`
	UserID     uint           `gorm:"column:user_id;index"`              // User who made the change
	Reason     string         `gorm:"column:reason;type:text"`           // Optional reason for the change
	CreatedAt  time.Time      `gorm:"column:created_at"`
	DeletedAt  gorm.DeletedAt `gorm:"index"`
}

// TableName returns the table name for ToolStatusHistoryModel
func (ToolStatusHistoryModel) TableName() string {
	return "mcp_tool_status_history"
}

// BeforeCreate hook for ToolStatusHistoryModel
func (m *ToolStatusHistoryModel) BeforeCreate(tx *gorm.DB) error {
	if m.CreatedAt.IsZero() {
		m.CreatedAt = time.Now()
	}
	return nil
}