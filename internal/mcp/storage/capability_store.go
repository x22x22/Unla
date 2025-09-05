package storage

import (
	"context"
	"time"

	"github.com/amoylab/unla/pkg/mcp"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// ToolStatusUpdate represents a single tool status update for batch operations
type ToolStatusUpdate struct {
	Name    string `json:"name"`
	Enabled bool   `json:"enabled"`
	Reason  string `json:"reason,omitempty"` // Optional reason for the change
}

// CapabilityStore defines the interface for MCP capability database operations
type CapabilityStore interface {
	// Tool operations
	SaveTool(ctx context.Context, tool *mcp.MCPTool, tenant, serverName string) error
	GetTool(ctx context.Context, tenant, serverName, name string) (*mcp.MCPTool, error)
	ListTools(ctx context.Context, tenant, serverName string) ([]mcp.MCPTool, error)
	DeleteTool(ctx context.Context, tenant, serverName, name string) error
	SyncTools(ctx context.Context, tools []mcp.MCPTool, tenant, serverName string) error
	UpdateToolStatus(ctx context.Context, tenant, serverName, name string, enabled bool) error
	BatchUpdateToolStatus(ctx context.Context, tenant, serverName string, updates []ToolStatusUpdate) error
	RecordToolStatusChange(ctx context.Context, tenant, serverName, toolName string, oldStatus, newStatus bool, userID uint, reason string) error
	GetToolStatusHistory(ctx context.Context, tenant, serverName, toolName string, limit, offset int) ([]*ToolStatusHistoryModel, error)

	// Prompt operations
	SavePrompt(ctx context.Context, prompt *mcp.MCPPrompt, tenant, serverName string) error
	GetPrompt(ctx context.Context, tenant, serverName, name string) (*mcp.MCPPrompt, error)
	ListPrompts(ctx context.Context, tenant, serverName string) ([]mcp.MCPPrompt, error)
	DeletePrompt(ctx context.Context, tenant, serverName, name string) error
	SyncPrompts(ctx context.Context, prompts []mcp.MCPPrompt, tenant, serverName string) error

	// Resource operations
	SaveResource(ctx context.Context, resource *mcp.MCPResource, tenant, serverName string) error
	GetResource(ctx context.Context, tenant, serverName, uri string) (*mcp.MCPResource, error)
	ListResources(ctx context.Context, tenant, serverName string) ([]mcp.MCPResource, error)
	DeleteResource(ctx context.Context, tenant, serverName, uri string) error
	SyncResources(ctx context.Context, resources []mcp.MCPResource, tenant, serverName string) error

	// Resource template operations
	SaveResourceTemplate(ctx context.Context, template *mcp.MCPResourceTemplate, tenant, serverName string) error
	GetResourceTemplate(ctx context.Context, tenant, serverName, uriTemplate string) (*mcp.MCPResourceTemplate, error)
	ListResourceTemplates(ctx context.Context, tenant, serverName string) ([]mcp.MCPResourceTemplate, error)
	DeleteResourceTemplate(ctx context.Context, tenant, serverName, uriTemplate string) error
	SyncResourceTemplates(ctx context.Context, templates []mcp.MCPResourceTemplate, tenant, serverName string) error

	// Capability info operations
	GetCapabilitiesInfo(ctx context.Context, tenant, serverName string) (*mcp.CapabilitiesInfo, error)
	SyncCapabilities(ctx context.Context, info *mcp.CapabilitiesInfo, tenant, serverName string) error

	// Server cleanup
	CleanupServerCapabilities(ctx context.Context, tenant, serverName string) error

	// Sync history operations
	CreateSyncRecord(ctx context.Context, record *SyncHistoryModel) error
	UpdateSyncRecord(ctx context.Context, syncID string, status SyncStatus, progress int, errorMessage, summary string) error
	CompleteSyncRecord(ctx context.Context, syncID string, status SyncStatus, summary string) error
	GetSyncRecord(ctx context.Context, syncID string) (*SyncHistoryModel, error)
	ListSyncHistory(ctx context.Context, tenant, serverName string, limit, offset int) ([]*SyncHistoryModel, error)
}

// DBCapabilityStore implements CapabilityStore using a database
type DBCapabilityStore struct {
	logger *zap.Logger
	db     *gorm.DB
}

var _ CapabilityStore = (*DBCapabilityStore)(nil)

// NewDBCapabilityStore creates a new database-based capability store
func NewDBCapabilityStore(logger *zap.Logger, db *gorm.DB) *DBCapabilityStore {
	return &DBCapabilityStore{
		logger: logger.Named("mcp.store.capability"),
		db:     db,
	}
}

// Tool operations

func (s *DBCapabilityStore) SaveTool(ctx context.Context, tool *mcp.MCPTool, tenant, serverName string) error {
	model, err := FromMCPTool(tool, tenant, serverName)
	if err != nil {
		return err
	}

	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var existing MCPToolModel
		err := tx.Where("tenant = ? AND server_name = ? AND name = ?", tenant, serverName, tool.Name).First(&existing).Error
		if err == nil {
			// Update existing tool
			model.ID = existing.ID
			model.CreatedAt = existing.CreatedAt
			return tx.Save(model).Error
		} else if err == gorm.ErrRecordNotFound {
			// Create new tool
			return tx.Create(model).Error
		}
		return err
	})
}

func (s *DBCapabilityStore) GetTool(ctx context.Context, tenant, serverName, name string) (*mcp.MCPTool, error) {
	var model MCPToolModel
	err := s.db.WithContext(ctx).Where("tenant = ? AND server_name = ? AND name = ?", tenant, serverName, name).First(&model).Error
	if err != nil {
		return nil, err
	}
	return model.ToMCPTool()
}

func (s *DBCapabilityStore) ListTools(ctx context.Context, tenant, serverName string) ([]mcp.MCPTool, error) {
	var models []MCPToolModel
	err := s.db.WithContext(ctx).Where("tenant = ? AND server_name = ?", tenant, serverName).Find(&models).Error
	if err != nil {
		return nil, err
	}

	tools := make([]mcp.MCPTool, len(models))
	for i, model := range models {
		tool, err := model.ToMCPTool()
		if err != nil {
			return nil, err
		}
		tools[i] = *tool
	}
	return tools, nil
}

func (s *DBCapabilityStore) DeleteTool(ctx context.Context, tenant, serverName, name string) error {
	return s.db.WithContext(ctx).Where("tenant = ? AND server_name = ? AND name = ?", tenant, serverName, name).Delete(&MCPToolModel{}).Error
}

func (s *DBCapabilityStore) SyncTools(ctx context.Context, tools []mcp.MCPTool, tenant, serverName string) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Delete existing tools for this server
		if err := tx.Where("tenant = ? AND server_name = ?", tenant, serverName).Delete(&MCPToolModel{}).Error; err != nil {
			return err
		}

		// Insert new tools
		for _, tool := range tools {
			model, err := FromMCPTool(&tool, tenant, serverName)
			if err != nil {
				return err
			}
			if err := tx.Create(model).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func (s *DBCapabilityStore) UpdateToolStatus(ctx context.Context, tenant, serverName, name string, enabled bool) error {
	result := s.db.WithContext(ctx).Model(&MCPToolModel{}).
		Where("tenant = ? AND server_name = ? AND name = ?", tenant, serverName, name).
		Update("enabled", enabled)
	
	if result.Error != nil {
		return result.Error
	}
	
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	
	return nil
}

func (s *DBCapabilityStore) BatchUpdateToolStatus(ctx context.Context, tenant, serverName string, updates []ToolStatusUpdate) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, update := range updates {
			result := tx.Model(&MCPToolModel{}).
				Where("tenant = ? AND server_name = ? AND name = ?", tenant, serverName, update.Name).
				Update("enabled", update.Enabled)
			
			if result.Error != nil {
				return result.Error
			}
			
			// Note: We don't check RowsAffected here for batch operations as some tools might not exist
			// The caller can handle individual tool validation if needed
		}
		return nil
	})
}

func (s *DBCapabilityStore) RecordToolStatusChange(ctx context.Context, tenant, serverName, toolName string, oldStatus, newStatus bool, userID uint, reason string) error {
	historyRecord := &ToolStatusHistoryModel{
		Tenant:     tenant,
		ServerName: serverName,
		ToolName:   toolName,
		OldStatus:  oldStatus,
		NewStatus:  newStatus,
		UserID:     userID,
		Reason:     reason,
	}
	
	return s.db.WithContext(ctx).Create(historyRecord).Error
}

func (s *DBCapabilityStore) GetToolStatusHistory(ctx context.Context, tenant, serverName, toolName string, limit, offset int) ([]*ToolStatusHistoryModel, error) {
	var records []*ToolStatusHistoryModel
	query := s.db.WithContext(ctx).Model(&ToolStatusHistoryModel{})
	
	if tenant != "" {
		query = query.Where("tenant = ?", tenant)
	}
	if serverName != "" {
		query = query.Where("server_name = ?", serverName)
	}
	if toolName != "" {
		query = query.Where("tool_name = ?", toolName)
	}
	
	err := query.Order("created_at desc").Limit(limit).Offset(offset).Find(&records).Error
	return records, err
}

// Prompt operations

func (s *DBCapabilityStore) SavePrompt(ctx context.Context, prompt *mcp.MCPPrompt, tenant, serverName string) error {
	model, err := FromMCPPrompt(prompt, tenant, serverName)
	if err != nil {
		return err
	}

	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var existing MCPPromptModel
		err := tx.Where("tenant = ? AND server_name = ? AND name = ?", tenant, serverName, prompt.Name).First(&existing).Error
		if err == nil {
			// Update existing prompt
			model.ID = existing.ID
			model.CreatedAt = existing.CreatedAt
			return tx.Save(model).Error
		} else if err == gorm.ErrRecordNotFound {
			// Create new prompt
			return tx.Create(model).Error
		}
		return err
	})
}

func (s *DBCapabilityStore) GetPrompt(ctx context.Context, tenant, serverName, name string) (*mcp.MCPPrompt, error) {
	var model MCPPromptModel
	err := s.db.WithContext(ctx).Where("tenant = ? AND server_name = ? AND name = ?", tenant, serverName, name).First(&model).Error
	if err != nil {
		return nil, err
	}
	return model.ToMCPPrompt()
}

func (s *DBCapabilityStore) ListPrompts(ctx context.Context, tenant, serverName string) ([]mcp.MCPPrompt, error) {
	var models []MCPPromptModel
	err := s.db.WithContext(ctx).Where("tenant = ? AND server_name = ?", tenant, serverName).Find(&models).Error
	if err != nil {
		return nil, err
	}

	prompts := make([]mcp.MCPPrompt, len(models))
	for i, model := range models {
		prompt, err := model.ToMCPPrompt()
		if err != nil {
			return nil, err
		}
		prompts[i] = *prompt
	}
	return prompts, nil
}

func (s *DBCapabilityStore) DeletePrompt(ctx context.Context, tenant, serverName, name string) error {
	return s.db.WithContext(ctx).Where("tenant = ? AND server_name = ? AND name = ?", tenant, serverName, name).Delete(&MCPPromptModel{}).Error
}

func (s *DBCapabilityStore) SyncPrompts(ctx context.Context, prompts []mcp.MCPPrompt, tenant, serverName string) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Delete existing prompts for this server
		if err := tx.Where("tenant = ? AND server_name = ?", tenant, serverName).Delete(&MCPPromptModel{}).Error; err != nil {
			return err
		}

		// Insert new prompts
		for _, prompt := range prompts {
			model, err := FromMCPPrompt(&prompt, tenant, serverName)
			if err != nil {
				return err
			}
			if err := tx.Create(model).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

// Resource operations

func (s *DBCapabilityStore) SaveResource(ctx context.Context, resource *mcp.MCPResource, tenant, serverName string) error {
	model := FromMCPResource(resource, tenant, serverName)

	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var existing MCPResourceModel
		err := tx.Where("tenant = ? AND server_name = ? AND uri = ?", tenant, serverName, resource.URI).First(&existing).Error
		if err == nil {
			// Update existing resource
			model.ID = existing.ID
			model.CreatedAt = existing.CreatedAt
			return tx.Save(model).Error
		} else if err == gorm.ErrRecordNotFound {
			// Create new resource
			return tx.Create(model).Error
		}
		return err
	})
}

func (s *DBCapabilityStore) GetResource(ctx context.Context, tenant, serverName, uri string) (*mcp.MCPResource, error) {
	var model MCPResourceModel
	err := s.db.WithContext(ctx).Where("tenant = ? AND server_name = ? AND uri = ?", tenant, serverName, uri).First(&model).Error
	if err != nil {
		return nil, err
	}
	return model.ToMCPResource(), nil
}

func (s *DBCapabilityStore) ListResources(ctx context.Context, tenant, serverName string) ([]mcp.MCPResource, error) {
	var models []MCPResourceModel
	err := s.db.WithContext(ctx).Where("tenant = ? AND server_name = ?", tenant, serverName).Find(&models).Error
	if err != nil {
		return nil, err
	}

	resources := make([]mcp.MCPResource, len(models))
	for i, model := range models {
		resources[i] = *model.ToMCPResource()
	}
	return resources, nil
}

func (s *DBCapabilityStore) DeleteResource(ctx context.Context, tenant, serverName, uri string) error {
	return s.db.WithContext(ctx).Where("tenant = ? AND server_name = ? AND uri = ?", tenant, serverName, uri).Delete(&MCPResourceModel{}).Error
}

func (s *DBCapabilityStore) SyncResources(ctx context.Context, resources []mcp.MCPResource, tenant, serverName string) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Delete existing resources for this server
		if err := tx.Where("tenant = ? AND server_name = ?", tenant, serverName).Delete(&MCPResourceModel{}).Error; err != nil {
			return err
		}

		// Insert new resources
		for _, resource := range resources {
			model := FromMCPResource(&resource, tenant, serverName)
			if err := tx.Create(model).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

// Resource template operations

func (s *DBCapabilityStore) SaveResourceTemplate(ctx context.Context, template *mcp.MCPResourceTemplate, tenant, serverName string) error {
	model, err := FromMCPResourceTemplate(template, tenant, serverName)
	if err != nil {
		return err
	}

	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var existing MCPResourceTemplateModel
		err := tx.Where("tenant = ? AND server_name = ? AND uri_template = ?", tenant, serverName, template.URITemplate).First(&existing).Error
		if err == nil {
			// Update existing template
			model.ID = existing.ID
			model.CreatedAt = existing.CreatedAt
			return tx.Save(model).Error
		} else if err == gorm.ErrRecordNotFound {
			// Create new template
			return tx.Create(model).Error
		}
		return err
	})
}

func (s *DBCapabilityStore) GetResourceTemplate(ctx context.Context, tenant, serverName, uriTemplate string) (*mcp.MCPResourceTemplate, error) {
	var model MCPResourceTemplateModel
	err := s.db.WithContext(ctx).Where("tenant = ? AND server_name = ? AND uri_template = ?", tenant, serverName, uriTemplate).First(&model).Error
	if err != nil {
		return nil, err
	}
	return model.ToMCPResourceTemplate()
}

func (s *DBCapabilityStore) ListResourceTemplates(ctx context.Context, tenant, serverName string) ([]mcp.MCPResourceTemplate, error) {
	var models []MCPResourceTemplateModel
	err := s.db.WithContext(ctx).Where("tenant = ? AND server_name = ?", tenant, serverName).Find(&models).Error
	if err != nil {
		return nil, err
	}

	templates := make([]mcp.MCPResourceTemplate, len(models))
	for i, model := range models {
		template, err := model.ToMCPResourceTemplate()
		if err != nil {
			return nil, err
		}
		templates[i] = *template
	}
	return templates, nil
}

func (s *DBCapabilityStore) DeleteResourceTemplate(ctx context.Context, tenant, serverName, uriTemplate string) error {
	return s.db.WithContext(ctx).Where("tenant = ? AND server_name = ? AND uri_template = ?", tenant, serverName, uriTemplate).Delete(&MCPResourceTemplateModel{}).Error
}

func (s *DBCapabilityStore) SyncResourceTemplates(ctx context.Context, templates []mcp.MCPResourceTemplate, tenant, serverName string) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Delete existing templates for this server
		if err := tx.Where("tenant = ? AND server_name = ?", tenant, serverName).Delete(&MCPResourceTemplateModel{}).Error; err != nil {
			return err
		}

		// Insert new templates
		for _, template := range templates {
			model, err := FromMCPResourceTemplate(&template, tenant, serverName)
			if err != nil {
				return err
			}
			if err := tx.Create(model).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

// Capability info operations

func (s *DBCapabilityStore) GetCapabilitiesInfo(ctx context.Context, tenant, serverName string) (*mcp.CapabilitiesInfo, error) {
	info := &mcp.CapabilitiesInfo{
		LastSynced: time.Now().Format(time.RFC3339),
	}

	// Get tools
	tools, err := s.ListTools(ctx, tenant, serverName)
	if err != nil {
		return nil, err
	}
	info.Tools = tools

	// Get prompts
	prompts, err := s.ListPrompts(ctx, tenant, serverName)
	if err != nil {
		return nil, err
	}
	info.Prompts = prompts

	// Get resources
	resources, err := s.ListResources(ctx, tenant, serverName)
	if err != nil {
		return nil, err
	}
	info.Resources = resources

	// Get resource templates
	templates, err := s.ListResourceTemplates(ctx, tenant, serverName)
	if err != nil {
		return nil, err
	}
	info.ResourceTemplates = templates

	return info, nil
}

func (s *DBCapabilityStore) SyncCapabilities(ctx context.Context, info *mcp.CapabilitiesInfo, tenant, serverName string) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		store := &DBCapabilityStore{logger: s.logger, db: tx}

		// Sync all capability types
		if err := store.SyncTools(ctx, info.Tools, tenant, serverName); err != nil {
			return err
		}
		if err := store.SyncPrompts(ctx, info.Prompts, tenant, serverName); err != nil {
			return err
		}
		if err := store.SyncResources(ctx, info.Resources, tenant, serverName); err != nil {
			return err
		}
		if err := store.SyncResourceTemplates(ctx, info.ResourceTemplates, tenant, serverName); err != nil {
			return err
		}

		return nil
	})
}

func (s *DBCapabilityStore) CleanupServerCapabilities(ctx context.Context, tenant, serverName string) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Delete all capabilities for this server
		if err := tx.Where("tenant = ? AND server_name = ?", tenant, serverName).Delete(&MCPToolModel{}).Error; err != nil {
			return err
		}
		if err := tx.Where("tenant = ? AND server_name = ?", tenant, serverName).Delete(&MCPPromptModel{}).Error; err != nil {
			return err
		}
		if err := tx.Where("tenant = ? AND server_name = ?", tenant, serverName).Delete(&MCPResourceModel{}).Error; err != nil {
			return err
		}
		if err := tx.Where("tenant = ? AND server_name = ?", tenant, serverName).Delete(&MCPResourceTemplateModel{}).Error; err != nil {
			return err
		}
		return nil
	})
}

// Sync history operations

func (s *DBCapabilityStore) CreateSyncRecord(ctx context.Context, record *SyncHistoryModel) error {
	return s.db.WithContext(ctx).Create(record).Error
}

func (s *DBCapabilityStore) UpdateSyncRecord(ctx context.Context, syncID string, status SyncStatus, progress int, errorMessage, summary string) error {
	updates := map[string]interface{}{
		"status":   status,
		"progress": progress,
	}
	if errorMessage != "" {
		updates["error_message"] = errorMessage
	}
	if summary != "" {
		updates["summary"] = summary
	}
	return s.db.WithContext(ctx).Model(&SyncHistoryModel{}).Where("sync_id = ?", syncID).Updates(updates).Error
}

func (s *DBCapabilityStore) CompleteSyncRecord(ctx context.Context, syncID string, status SyncStatus, summary string) error {
	now := time.Now()
	updates := map[string]interface{}{
		"status":       status,
		"completed_at": now,
		"progress":     100,
	}
	if summary != "" {
		updates["summary"] = summary
	}
	return s.db.WithContext(ctx).Model(&SyncHistoryModel{}).Where("sync_id = ?", syncID).Updates(updates).Error
}

func (s *DBCapabilityStore) GetSyncRecord(ctx context.Context, syncID string) (*SyncHistoryModel, error) {
	var record SyncHistoryModel
	err := s.db.WithContext(ctx).Where("sync_id = ?", syncID).First(&record).Error
	if err != nil {
		return nil, err
	}
	return &record, nil
}

func (s *DBCapabilityStore) ListSyncHistory(ctx context.Context, tenant, serverName string, limit, offset int) ([]*SyncHistoryModel, error) {
	var records []*SyncHistoryModel
	query := s.db.WithContext(ctx)
	if tenant != "" {
		query = query.Where("tenant = ?", tenant)
	}
	if serverName != "" {
		query = query.Where("server_name = ?", serverName)
	}
	
	err := query.Order("created_at desc").Limit(limit).Offset(offset).Find(&records).Error
	return records, err
}