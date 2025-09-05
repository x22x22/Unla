package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/amoylab/unla/internal/apiserver/cache"
	"github.com/amoylab/unla/internal/apiserver/database"
	"github.com/amoylab/unla/internal/auth/jwt"
	"github.com/amoylab/unla/internal/common/config"
	"github.com/amoylab/unla/internal/common/dto"
	"github.com/amoylab/unla/internal/core/mcpproxy"
	"github.com/amoylab/unla/internal/i18n"
	"github.com/amoylab/unla/internal/mcp/storage"
	"github.com/amoylab/unla/internal/mcp/storage/notifier"
	"github.com/amoylab/unla/internal/template"
	"github.com/amoylab/unla/pkg/mcp"
	"go.uber.org/zap"
	"gopkg.in/yaml.v3"
)

type MCP struct {
	db               database.Database
	store            storage.Store
	capabilityStore  storage.CapabilityStore
	notifier         notifier.Notifier
	logger           *zap.Logger
	capabilitiesCache sync.Map // key: tenant:name, value: *cachedCapabilities
	wsManager        *WebSocketManager // WebSocket manager for real-time updates
	cacheManager     *cache.CacheManager // Multi-layer cache manager
}

type cachedCapabilities struct {
	data      *mcp.CapabilitiesInfo
	timestamp time.Time
	ttl       time.Duration
}

// SyncRequest represents the sync request body
type SyncRequest struct {
	Types []string `json:"types,omitempty"` // Optional: tools, prompts, resources, resourceTemplates
	Force bool     `json:"force"`           // Force sync even if no changes detected
}

// SyncResponse represents the sync response
type SyncResponse struct {
	SyncID    string                 `json:"syncId"`
	Status    storage.SyncStatus     `json:"status"`
	StartedAt time.Time              `json:"startedAt"`
	Summary   map[string]interface{} `json:"summary,omitempty"`
}

// SyncSummary represents the sync operation summary
type SyncSummary struct {
	ToolsAdded              int                    `json:"toolsAdded"`
	ToolsUpdated            int                    `json:"toolsUpdated"`
	ToolsRemoved            int                    `json:"toolsRemoved"`
	PromptsAdded            int                    `json:"promptsAdded"`
	PromptsUpdated          int                    `json:"promptsUpdated"`
	PromptsRemoved          int                    `json:"promptsRemoved"`
	ResourcesAdded          int                    `json:"resourcesAdded"`
	ResourcesUpdated        int                    `json:"resourcesUpdated"`
	ResourcesRemoved        int                    `json:"resourcesRemoved"`
	ResourceTemplatesAdded  int                    `json:"resourceTemplatesAdded"`
	ResourceTemplatesUpdated int                   `json:"resourceTemplatesUpdated"`
	ResourceTemplatesRemoved int                   `json:"resourceTemplatesRemoved"`
	Conflicts               []CapabilityConflict   `json:"conflicts,omitempty"`
	Errors                  []string               `json:"errors,omitempty"`
}

// UpdateToolStatusRequest represents the request body for updating tool status
type UpdateToolStatusRequest struct {
	Enabled bool   `json:"enabled"`
	Reason  string `json:"reason,omitempty"` // Optional reason for the change
}

// UpdateToolStatusResponse represents the response for tool status update
type UpdateToolStatusResponse struct {
	Success   bool   `json:"success"`
	Message   string `json:"message,omitempty"`
	ToolName  string `json:"toolName"`
	Enabled   bool   `json:"enabled"`
	UpdatedAt string `json:"updatedAt"`
}

// BatchUpdateToolStatusRequest represents the request body for batch updating tool statuses
type BatchUpdateToolStatusRequest struct {
	Tools []storage.ToolStatusUpdate `json:"tools"`
}

// BatchUpdateToolStatusResponse represents the response for batch tool status update
type BatchUpdateToolStatusResponse struct {
	Success     bool                      `json:"success"`
	Message     string                    `json:"message,omitempty"`
	Results     []ToolStatusUpdateResult  `json:"results"`
	UpdatedAt   string                    `json:"updatedAt"`
	TotalTools  int                       `json:"totalTools"`
	SuccessCount int                      `json:"successCount"`
	ErrorCount   int                      `json:"errorCount"`
}

// ToolStatusUpdateResult represents the result of a single tool status update
type ToolStatusUpdateResult struct {
	ToolName  string `json:"toolName"`
	Success   bool   `json:"success"`
	Enabled   bool   `json:"enabled,omitempty"`
	Error     string `json:"error,omitempty"`
}

// ToolStatusHistoryResponse represents a tool status change history item
type ToolStatusHistoryResponse struct {
	ID         uint      `json:"id"`
	Tenant     string    `json:"tenant"`
	ServerName string    `json:"serverName"`
	ToolName   string    `json:"toolName"`
	OldStatus  bool      `json:"oldStatus"`
	NewStatus  bool      `json:"newStatus"`
	UserID     uint      `json:"userId"`
	Username   string    `json:"username,omitempty"`
	Reason     string    `json:"reason,omitempty"`
	CreatedAt  time.Time `json:"createdAt"`
}

// CapabilityConflict represents a conflict detected during sync
type CapabilityConflict struct {
	Type        string    `json:"type"`        // tool, prompt, resource, resourceTemplate
	Name        string    `json:"name"`        // name or URI
	ConflictType string   `json:"conflictType"` // schema_mismatch, description_change, etc.
	ExistingHash string   `json:"existingHash"`
	NewHash      string   `json:"newHash"`
	ResolvedBy   string   `json:"resolvedBy,omitempty"` // local, remote, manual
}

// CapabilitiesStatsResponse represents the response for capabilities statistics
type CapabilitiesStatsResponse struct {
	Server      ServerStatsInfo    `json:"server"`
	Tools       ToolsStatsInfo     `json:"tools"`
	Prompts     PromptsStatsInfo   `json:"prompts"`
	Resources   ResourcesStatsInfo `json:"resources"`
	Summary     StatsOverallInfo   `json:"summary"`
	LastUpdated time.Time          `json:"lastUpdated"`
}

// ServerStatsInfo represents server-level statistics
type ServerStatsInfo struct {
	Tenant      string    `json:"tenant"`
	ServerName  string    `json:"serverName"`
	LastSyncAt  time.Time `json:"lastSyncAt"`
	Status      string    `json:"status"`
	Version     string    `json:"version,omitempty"`
}

// ToolsStatsInfo represents tools statistics
type ToolsStatsInfo struct {
	Total       int                    `json:"total"`
	Enabled     int                    `json:"enabled"`
	Disabled    int                    `json:"disabled"`
	EnabledRate float64               `json:"enabledRate"`
	ByCategory  map[string]int        `json:"byCategory,omitempty"`
	Usage       ToolUsageStatsInfo    `json:"usage,omitempty"`
}

// PromptsStatsInfo represents prompts statistics
type PromptsStatsInfo struct {
	Total      int            `json:"total"`
	WithArgs   int            `json:"withArgs"`
	WithoutArgs int           `json:"withoutArgs"`
	ByCategory map[string]int `json:"byCategory,omitempty"`
}

// ResourcesStatsInfo represents resources statistics
type ResourcesStatsInfo struct {
	Total        int            `json:"total"`
	Templates    int            `json:"templates"`
	Static       int            `json:"static"`
	ByMimeType   map[string]int `json:"byMimeType,omitempty"`
}

// ToolUsageStatsInfo represents tool usage statistics
type ToolUsageStatsInfo struct {
	TotalCalls    int64   `json:"totalCalls"`
	SuccessRate   float64 `json:"successRate"`
	AvgExecTime   float64 `json:"avgExecTime"`
	LastUsedAt    *time.Time `json:"lastUsedAt,omitempty"`
}

// StatsOverallInfo represents overall statistics summary
type StatsOverallInfo struct {
	TotalCapabilities int            `json:"totalCapabilities"`
	ActiveCapabilities int           `json:"activeCapabilities"`
	Distribution      map[string]int `json:"distribution"`
}

func (c *cachedCapabilities) isExpired() bool {
	return time.Since(c.timestamp) > c.ttl
}

func NewMCP(db database.Database, store storage.Store, ntf notifier.Notifier, logger *zap.Logger, redisClient ...redis.Cmdable) *MCP {
	var capabilityStore storage.CapabilityStore
	
	// Try to get CapabilityStore from Store if it's a DBStore
	if dbStore, ok := store.(*storage.DBStore); ok {
		capabilityStore = dbStore.DBCapabilityStore
	}
	
	// Create cache manager if Redis client is provided
	var cacheManager *cache.CacheManager
	if len(redisClient) > 0 && redisClient[0] != nil && capabilityStore != nil {
		cacheConfig := cache.CacheManagerConfig{
			RedisClient:     redisClient[0],
			CapabilityStore: capabilityStore,
			Logger:          logger,
		}
		cacheManager = cache.NewCacheManager(cacheConfig)
	}
	
	return &MCP{
		db:                db,
		store:             store,
		capabilityStore:   capabilityStore,
		notifier:          ntf,
		logger:            logger,
		capabilitiesCache: sync.Map{},
		wsManager:         NewWebSocketManager(logger),
		cacheManager:      cacheManager,
	}
}

// checkTenantPermission checks if the user has permission to access the specified tenant and
// verifies that all router prefixes start with the tenant prefix as a complete path segment
func (h *MCP) checkTenantPermission(c *gin.Context, tenantName string, cfg *config.MCPConfig) (*database.Tenant, error) {
	h.logger.Debug("checking tenant permission",
		zap.String("tenant_name", tenantName))

	// Check if tenant name is empty
	if tenantName == "" {
		h.logger.Warn("tenant name is empty")
		return nil, i18n.ErrorTenantNameRequired
	}

	// Get user authentication information
	claims, exists := c.Get("claims")
	if !exists {
		h.logger.Warn("missing JWT claims in context")
		return nil, i18n.ErrUnauthorized
	}
	jwtClaims := claims.(*jwt.Claims)

	// Get user information
	user, err := h.db.GetUserByUsername(c.Request.Context(), jwtClaims.Username)
	if err != nil {
		h.logger.Error("failed to get user info",
			zap.String("username", jwtClaims.Username),
			zap.Error(err))
		return nil, i18n.ErrInternalServer.WithParam("Reason", "Failed to get user info: "+err.Error())
	}

	// Get tenant information
	tenant, err := h.db.GetTenantByName(c.Request.Context(), tenantName)
	if err != nil {
		h.logger.Warn("tenant not found",
			zap.String("tenant_name", tenantName),
			zap.Error(err))
		return nil, i18n.ErrorTenantNotFound.WithParam("Name", tenantName)
	}

	// Normalize tenant prefix
	tenantPrefix := tenant.Prefix
	if !strings.HasPrefix(tenantPrefix, "/") {
		tenantPrefix = "/" + tenantPrefix
	}
	tenantPrefix = strings.TrimSuffix(tenantPrefix, "/")

	// Check if all router prefixes start with tenant prefix
	for _, router := range cfg.Routers {
		// Normalize router prefix
		routerPrefix := router.Prefix
		if !strings.HasPrefix(routerPrefix, "/") {
			routerPrefix = "/" + routerPrefix
		}
		routerPrefix = strings.TrimSuffix(routerPrefix, "/")

		// Allow exact match
		if routerPrefix == tenantPrefix {
			continue
		}

		// Must start with tenant prefix followed by a path separator
		if !strings.HasPrefix(routerPrefix, tenantPrefix+"/") {
			h.logger.Warn("router prefix validation failed",
				zap.String("router_prefix", routerPrefix),
				zap.String("tenant_prefix", tenantPrefix))
			return nil, i18n.ErrorRouterPrefixError
		}
	}

	// Check user permission if not admin
	if user.Role != database.RoleAdmin {
		userTenants, err := h.db.GetUserTenants(c.Request.Context(), user.ID)
		if err != nil {
			h.logger.Error("failed to get user tenants",
				zap.Uint("user_id", user.ID),
				zap.Error(err))
			return nil, i18n.ErrInternalServer.WithParam("Reason", "Failed to get user tenants: "+err.Error())
		}

		allowed := false
		for _, userTenant := range userTenants {
			if userTenant.ID == tenant.ID {
				allowed = true
				break
			}
		}

		if !allowed {
			h.logger.Warn("user lacks permission for tenant",
				zap.Uint("user_id", user.ID),
				zap.Uint("tenant_id", tenant.ID))
			return nil, i18n.ErrorTenantPermissionError
		}
	}

	h.logger.Debug("tenant permission check passed",
		zap.String("tenant_name", tenantName),
		zap.Uint("tenant_id", tenant.ID))
	return tenant, nil
}

func (h *MCP) HandleMCPServerUpdate(c *gin.Context) {
	// Read the raw YAML content from request body
	content, err := c.GetRawData()
	if err != nil {
		h.logger.Error("failed to read request body", zap.Error(err))
		i18n.RespondWithError(c, i18n.ErrBadRequest.WithParam("Reason", "Failed to read request body: "+err.Error()))
		return
	}

	// Validate the YAML content
	var cfg config.MCPConfig
	if err := yaml.Unmarshal(content, &cfg); err != nil {
		h.logger.Error("invalid YAML content", zap.Error(err))
		i18n.RespondWithError(c, i18n.ErrorMCPServerValidation.WithParam("Reason", "Invalid YAML content: "+err.Error()))
		return
	}

	// Get existing server
	oldCfg, err := h.store.Get(c.Request.Context(), cfg.Tenant, cfg.Name)
	if err != nil {
		h.logger.Error("MCP server not found",
			zap.String("server_name", cfg.Name),
			zap.Error(err))
		i18n.RespondWithError(c, i18n.ErrorMCPServerNotFound.WithParam("Name", cfg.Name))
		return
	}

	if oldCfg.Name != cfg.Name {
		h.logger.Warn("server name mismatch",
			zap.String("config_name", cfg.Name))
		i18n.RespondWithError(c, i18n.ErrorMCPServerValidation.WithParam("Reason", "Server name in configuration must match name parameter"))
		return
	}

	_, err = h.checkTenantPermission(c, cfg.Tenant, &cfg)
	if err != nil {
		h.logger.Warn("tenant permission check failed",
			zap.String("tenant", cfg.Tenant),
			zap.Error(err))
		i18n.RespondWithError(c, err)
		return
	}

	// Get all existing configurations
	configs, err := h.store.List(c.Request.Context())
	if err != nil {
		h.logger.Error("failed to get existing configurations", zap.Error(err))
		i18n.RespondWithError(c, i18n.ErrInternalServer.WithParam("Reason", "Failed to get existing configurations: "+err.Error()))
		return
	}

	// Merge the new configuration with existing configs
	configs = config.MergeConfigs(configs, &cfg)

	// Validate all configurations
	if err := config.ValidateMCPConfigs(configs); err != nil {
		var validationErr *config.ValidationError
		if errors.As(err, &validationErr) {
			h.logger.Error("configuration validation failed",
				zap.String("validation_error", validationErr.Error()))
			i18n.RespondWithError(c, i18n.ErrorMCPServerValidation.WithParam("Reason", "Configuration validation failed: "+validationErr.Error()))
		} else {
			h.logger.Error("failed to validate configurations", zap.Error(err))
			i18n.RespondWithError(c, i18n.ErrorMCPServerValidation.WithParam("Reason", "Failed to validate configurations: "+err.Error()))
		}
		return
	}

	if err := h.store.Update(c.Request.Context(), &cfg); err != nil {
		h.logger.Error("failed to update MCP server",
			zap.String("server_name", cfg.Name),
			zap.Error(err))
		i18n.RespondWithError(c, i18n.ErrInternalServer.WithParam("Reason", "Failed to update MCP server: "+err.Error()))
		return
	}

	// Send reload signal to gateway using notifier
	if err := h.notifier.NotifyUpdate(c.Request.Context(), &cfg); err != nil {
		h.logger.Error("failed to reload gateway", zap.Error(err))
		i18n.RespondWithError(c, i18n.ErrInternalServer.WithParam("Reason", "Failed to reload gateway: "+err.Error()))
		return
	}

	// Clear cache for this server
	cacheKey := cfg.Tenant + ":" + cfg.Name
	h.clearCapabilitiesCache(cacheKey)

	h.logger.Info("MCP server updated successfully",
		zap.String("server_name", cfg.Name))
	i18n.Success(i18n.SuccessMCPServerUpdated).With("status", "success").Send(c)
}

func (h *MCP) HandleListMCPServers(c *gin.Context) {
	h.logger.Info("handling list MCP servers request")

	tenantIDStr := c.Query("tenantId")
	var tenantID uint
	if tenantIDStr != "" {
		tid, err := strconv.ParseUint(tenantIDStr, 10, 32)
		if err != nil {
			h.logger.Warn("invalid tenantId parameter",
				zap.String("tenantId", tenantIDStr))
			i18n.RespondWithError(c, i18n.ErrBadRequest.WithParam("Reason", "Invalid tenantId parameter"))
			return
		}
		tenantID = uint(tid)
		h.logger.Debug("filtering by tenant ID", zap.Uint("tenant_id", tenantID))
	}

	claims, exists := c.Get("claims")
	if !exists {
		h.logger.Warn("missing JWT claims in context")
		i18n.RespondWithError(c, i18n.ErrUnauthorized)
		return
	}
	jwtClaims := claims.(*jwt.Claims)

	user, err := h.db.GetUserByUsername(c.Request.Context(), jwtClaims.Username)
	if err != nil {
		h.logger.Error("failed to get user info",
			zap.String("username", jwtClaims.Username),
			zap.Error(err))
		i18n.RespondWithError(c, i18n.ErrInternalServer.WithParam("Reason", "Failed to get user info: "+err.Error()))
		return
	}

	servers, err := h.store.List(c.Request.Context())
	if err != nil {
		h.logger.Error("failed to get MCP servers", zap.Error(err))
		i18n.RespondWithError(c, i18n.ErrInternalServer.WithParam("Reason", "Failed to get MCP servers: "+err.Error()))
		return
	}

	if user.Role != database.RoleAdmin && tenantID > 0 {
		userTenants, err := h.db.GetUserTenants(c.Request.Context(), user.ID)
		if err != nil {
			h.logger.Error("failed to get user tenants",
				zap.Uint("user_id", user.ID),
				zap.Error(err))
			i18n.RespondWithError(c, i18n.ErrInternalServer.WithParam("Reason", "Failed to get user tenants: "+err.Error()))
			return
		}

		hasPermission := false
		for _, tenant := range userTenants {
			if tenant.ID == tenantID {
				hasPermission = true
				break
			}
		}

		if !hasPermission {
			h.logger.Warn("user lacks permission for tenant",
				zap.Uint("user_id", user.ID),
				zap.Uint("tenant_id", tenantID))
			i18n.RespondWithError(c, i18n.ErrorTenantPermissionError)
			return
		}
	}

	var filteredServers []*config.MCPConfig
	if tenantID > 0 {
		tenant, err := h.db.GetTenantByID(c.Request.Context(), tenantID)
		if err != nil {
			h.logger.Error("tenant not found",
				zap.Uint("tenant_id", tenantID),
				zap.Error(err))
			i18n.RespondWithError(c, i18n.ErrorTenantNotFound)
			return
		}

		name := tenant.Name
		for _, server := range servers {
			if server.Tenant == name {
				filteredServers = append(filteredServers, server)
			}
		}
		h.logger.Debug("filtered servers by tenant name",
			zap.String("tenant_name", name),
			zap.Int("server_count", len(filteredServers)))
	} else if user.Role != database.RoleAdmin {
		userTenants, err := h.db.GetUserTenants(c.Request.Context(), user.ID)
		if err != nil {
			h.logger.Error("failed to get user tenants",
				zap.Uint("user_id", user.ID),
				zap.Error(err))
			i18n.RespondWithError(c, i18n.ErrInternalServer.WithParam("Reason", "Failed to get user tenants: "+err.Error()))
			return
		}

		tenantNames := make([]string, len(userTenants))
		for i, tenant := range userTenants {
			tenantNames[i] = tenant.Name
		}

		for _, server := range servers {
			for _, name := range tenantNames {
				if server.Tenant == name {
					filteredServers = append(filteredServers, server)
					break
				}
			}
		}
		h.logger.Debug("filtered servers by user tenants",
			zap.Uint("user_id", user.ID),
			zap.Int("tenant_count", len(tenantNames)),
			zap.Int("server_count", len(filteredServers)))
	} else {
		filteredServers = servers
		h.logger.Debug("admin user, returning all servers",
			zap.Int("server_count", len(servers)))
	}

	// TODO: temporary
	results := make([]*dto.MCPServer, len(filteredServers))
	for i, server := range filteredServers {
		results[i] = &dto.MCPServer{
			Name:       server.Name,
			Tenant:     server.Tenant,
			McpServers: dto.FromMCPServerConfigs(server.McpServers),
			Tools:      dto.FromToolConfigs(server.Tools),
			Prompts:    dto.FromPromptConfigs(server.Prompts),
			Servers:    dto.FromServerConfigs(server.Servers),
			Routers:    dto.FromRouterConfigs(server.Routers),
			CreatedAt:  server.CreatedAt,
			UpdatedAt:  server.UpdatedAt,
		}
	}

	h.logger.Info("returning MCP server list",
		zap.Int("server_count", len(results)))
	i18n.Success(i18n.SuccessMCPServerList).With("data", results).Send(c)
}

func (h *MCP) HandleMCPServerCreate(c *gin.Context) {
	h.logger.Info("handling MCP server create request")

	// Read the raw YAML content from request body
	content, err := c.GetRawData()
	if err != nil {
		h.logger.Error("failed to read request body", zap.Error(err))
		i18n.RespondWithError(c, i18n.ErrBadRequest.WithParam("Reason", "Failed to read request body: "+err.Error()))
		return
	}

	// Validate the YAML content and get the server name
	var cfg config.MCPConfig
	if err := yaml.Unmarshal(content, &cfg); err != nil {
		h.logger.Error("invalid YAML content", zap.Error(err))
		i18n.RespondWithError(c, i18n.ErrorMCPServerValidation.WithParam("Reason", "Invalid YAML content: "+err.Error()))
		return
	}

	if cfg.Name == "" {
		h.logger.Warn("server name is required in configuration")
		i18n.RespondWithError(c, i18n.ErrorMCPServerValidation.WithParam("Reason", "Server name is required in configuration"))
		return
	}

	h.logger.Debug("validating server configuration",
		zap.String("server_name", cfg.Name),
		zap.String("tenant", cfg.Tenant))

	_, err = h.checkTenantPermission(c, cfg.Tenant, &cfg)
	if err != nil {
		h.logger.Warn("tenant permission check failed",
			zap.String("tenant", cfg.Tenant),
			zap.Error(err))

		i18n.RespondWithError(c, err)
		return
	}

	// Check if server already exists
	_, err = h.store.Get(c.Request.Context(), cfg.Tenant, cfg.Name)
	if err == nil {
		h.logger.Warn("MCP server already exists",
			zap.String("server_name", cfg.Name))
		i18n.RespondWithError(c, i18n.ErrorMCPServerExists.WithParam("Name", cfg.Name))
		return
	}

	// Get all existing configurations
	configs, err := h.store.List(c.Request.Context())
	if err != nil {
		h.logger.Error("failed to get existing configurations", zap.Error(err))
		i18n.RespondWithError(c, i18n.ErrInternalServer.WithParam("Reason", "Failed to get existing configurations: "+err.Error()))
		return
	}

	// Add the new configuration to the list
	configs = append(configs, &cfg)

	// Validate all configurations
	if err := config.ValidateMCPConfigs(configs); err != nil {
		var validationErr *config.ValidationError
		if errors.As(err, &validationErr) {
			h.logger.Error("configuration validation failed",
				zap.String("validation_error", validationErr.Error()))
			i18n.RespondWithError(c, i18n.ErrorMCPServerValidation.WithParam("Reason", "Configuration validation failed: "+validationErr.Error()))
		} else {
			h.logger.Error("failed to validate configurations", zap.Error(err))
			i18n.RespondWithError(c, i18n.ErrorMCPServerValidation.WithParam("Reason", "Failed to validate configurations: "+err.Error()))
		}
		return
	}

	if err := h.store.Create(c.Request.Context(), &cfg); err != nil {
		h.logger.Error("failed to create MCP server",
			zap.String("server_name", cfg.Name),
			zap.Error(err))
		i18n.RespondWithError(c, i18n.ErrInternalServer.WithParam("Reason", "Failed to create MCP server: "+err.Error()))
		return
	}

	// Send reload signal to gateway using notifier
	if err := h.notifier.NotifyUpdate(c.Request.Context(), &cfg); err != nil {
		h.logger.Error("failed to reload gateway", zap.Error(err))
		i18n.RespondWithError(c, i18n.ErrInternalServer.WithParam("Reason", "Failed to reload gateway: "+err.Error()))
		return
	}

	// Clear cache for this server (in case it was previously created and cached)
	cacheKey := cfg.Tenant + ":" + cfg.Name
	h.clearCapabilitiesCache(cacheKey)

	h.logger.Info("MCP server created successfully",
		zap.String("server_name", cfg.Name))
	i18n.Created(i18n.SuccessMCPServerCreated).With("status", "success").Send(c)
}

func (h *MCP) HandleMCPServerDelete(c *gin.Context) {
	// Get the server name from path parameter
	tenant := c.Param("tenant")
	if tenant == "" {
		h.logger.Warn("MCP server tenant required but missing")
		i18n.RespondWithError(c, i18n.ErrorTenantRequired)
		return
	}
	name := c.Param("name")
	if name == "" {
		h.logger.Warn("MCP server name required but missing")
		i18n.RespondWithError(c, i18n.ErrorMCPServerNameRequired)
		return
	}

	h.logger.Info("handling MCP server delete request",
		zap.String("server_name", name))

	// Check if server exists
	existingCfg, err := h.store.Get(c.Request.Context(), tenant, name)
	if err != nil {
		h.logger.Error("MCP server not found",
			zap.String("server_name", name),
			zap.Error(err))
		i18n.RespondWithError(c, i18n.ErrorMCPServerNotFound.WithParam("Name", name))
		return
	}

	// Check tenant permission
	_, err = h.checkTenantPermission(c, existingCfg.Tenant, existingCfg)
	if err != nil {
		h.logger.Warn("tenant permission check failed",
			zap.String("tenant", existingCfg.Tenant),
			zap.Error(err))
		i18n.RespondWithError(c, err)
		return
	}

	// Delete server
	if err := h.store.Delete(c.Request.Context(), existingCfg.Tenant, name); err != nil {
		h.logger.Error("failed to delete MCP server",
			zap.String("server_name", name),
			zap.Error(err))
		i18n.RespondWithError(c, i18n.ErrInternalServer.WithParam("Reason", "Failed to delete MCP server: "+err.Error()))
		return
	}

	// Send reload signal to gateway using notifier
	existingCfg.DeletedAt = time.Now()
	if err := h.notifier.NotifyUpdate(c.Request.Context(), existingCfg); err != nil {
		h.logger.Error("failed to notify gateway", zap.Error(err))
		i18n.RespondWithError(c, i18n.ErrInternalServer.WithParam("Reason", "Failed to notify gateway: "+err.Error()))
		return
	}

	// Clear cache for this server
	cacheKey := existingCfg.Tenant + ":" + name
	h.clearCapabilitiesCache(cacheKey)

	h.logger.Info("MCP server deleted successfully",
		zap.String("server_name", name))
	i18n.Success(i18n.SuccessMCPServerDeleted).With("status", "success").Send(c)
}

func (h *MCP) HandleMCPServerSync(c *gin.Context) {
	// Get user from claims
	claims, exists := c.Get("claims")
	if !exists {
		h.logger.Warn("missing JWT claims in context")
		i18n.RespondWithError(c, i18n.ErrUnauthorized)
		return
	}
	jwtClaims := claims.(*jwt.Claims)

	// Get user information
	user, err := h.db.GetUserByUsername(c.Request.Context(), jwtClaims.Username)
	if err != nil {
		h.logger.Error("failed to get user info",
			zap.String("username", jwtClaims.Username),
			zap.Error(err))
		i18n.RespondWithError(c, i18n.ErrInternalServer.WithParam("Reason", "Failed to get user info: "+err.Error()))
		return
	}

	// Only admin can sync all servers
	if user.Role != database.RoleAdmin {
		h.logger.Warn("non-admin user attempted to sync all servers",
			zap.String("username", user.Username))
		i18n.RespondWithError(c, i18n.ErrUnauthorized)
		return
	}

	h.logger.Info("handling MCP server sync request")

	// Send reload signal to gateway using notifier
	if err := h.notifier.NotifyUpdate(c.Request.Context(), nil); err != nil {
		h.logger.Error("failed to reload gateway", zap.Error(err))
		i18n.RespondWithError(c, i18n.ErrInternalServer.WithParam("Reason", "Failed to reload gateway: "+err.Error()))
		return
	}

	// Clear all cache since we're syncing all servers
	h.clearCapabilitiesCache("")

	h.logger.Info("MCP servers synced successfully")
	i18n.Success(i18n.SuccessMCPServerSynced).With("status", "success").Send(c)
}

// HandleGetConfigVersions handles the request to get configuration versions
func (h *MCP) HandleGetConfigVersions(c *gin.Context) {
	configNames := c.QueryArray("names")
	tenant := c.Query("tenant")
	var versions []*config.MCPConfigVersion
	var configs []*config.MCPConfig

	// Get user from claims
	claims, exists := c.Get("claims")
	if !exists {
		h.logger.Warn("missing JWT claims in context")
		i18n.RespondWithError(c, i18n.ErrUnauthorized)
		return
	}
	jwtClaims := claims.(*jwt.Claims)

	// Get user information
	user, err := h.db.GetUserByUsername(c.Request.Context(), jwtClaims.Username)
	if err != nil {
		h.logger.Error("failed to get user info",
			zap.String("username", jwtClaims.Username),
			zap.Error(err))
		i18n.RespondWithError(c, i18n.ErrInternalServer.WithParam("Reason", "Failed to get user info: "+err.Error()))
		return
	}

	// If no names provided, get all configs first
	configs, err = h.store.List(c.Request.Context(), true)
	if err != nil {
		h.logger.Error("failed to list configs", zap.Error(err))
		i18n.RespondWithError(c, i18n.ErrInternalServer)
		return
	}

	// Filter by tenant if specified
	if tenant != "" {
		filteredConfigs := make([]*config.MCPConfig, 0)
		for _, cfg := range configs {
			if cfg.Tenant == tenant {
				filteredConfigs = append(filteredConfigs, cfg)
			}
		}
		configs = filteredConfigs
	}

	if user.Role != database.RoleAdmin {
		// Get user's tenants
		tenants, err := h.db.GetUserTenants(c.Request.Context(), user.ID)
		if err != nil {
			h.logger.Error("failed to get user tenants",
				zap.String("username", user.Username),
				zap.Error(err))
			i18n.RespondWithError(c, i18n.ErrInternalServer)
			return
		}

		// Create a map of tenant names for faster lookup
		userTenants := make(map[string]bool)
		for _, t := range tenants {
			userTenants[t.Name] = true
		}

		filteredConfigs := make([]*config.MCPConfig, 0)
		for _, cfg := range configs {
			if userTenants[cfg.Tenant] {
				filteredConfigs = append(filteredConfigs, cfg)
			}
		}
		configs = filteredConfigs
	}

	if len(configNames) > 0 {
		// Filter configs by names
		filteredConfigs := make([]*config.MCPConfig, 0)
		for _, cfg := range configs {
			for _, name := range configNames {
				if cfg.Name == name {
					filteredConfigs = append(filteredConfigs, cfg)
					break
				}
			}
		}
		configs = filteredConfigs
	}

	if len(configs) == 0 {
		h.logger.Warn("no configs found")
		i18n.Success(i18n.SuccessMCPConfigVersions).With("data", versions).Send(c)
		return
	}

	// Get versions for each config
	for _, cfg := range configs {
		configVersions, err := h.store.ListVersions(c.Request.Context(), cfg.Tenant, cfg.Name)
		if err != nil {
			h.logger.Error("failed to list versions", zap.String("config", cfg.Name), zap.Error(err))
			continue
		}
		versions = append(versions, configVersions...)
	}

	// Sort versions by created_at in descending order
	sort.Slice(versions, func(i, j int) bool {
		return versions[i].CreatedAt.After(versions[j].CreatedAt)
	})

	i18n.Success(i18n.SuccessMCPConfigVersions).With("data", versions).Send(c)
}

// HandleSetActiveVersion handles setting a version as active
func (h *MCP) HandleSetActiveVersion(c *gin.Context) {
	tenant := c.Param("tenant")
	if tenant == "" {
		h.logger.Warn("MCP server tenant required but missing")
		i18n.RespondWithError(c, i18n.ErrorTenantRequired)
		return
	}
	name := c.Param("name")
	if name == "" {
		h.logger.Warn("config name required but missing")
		i18n.RespondWithError(c, i18n.ErrorMCPServerNameRequired)
		return
	}
	versionStr := c.Param("version")
	if versionStr == "" {
		h.logger.Warn("version required but missing")
		i18n.RespondWithError(c, i18n.ErrorVersionRequired)
		return
	}

	version, err := strconv.Atoi(versionStr)
	if err != nil {
		h.logger.Warn("invalid version number",
			zap.String("version", versionStr),
			zap.Error(err))
		i18n.RespondWithError(c, i18n.ErrBadRequest.WithParam("Reason", "Invalid version number"))
		return
	}

	h.logger.Info("handling set active version request",
		zap.String("config_name", name),
		zap.Int("version", version))

	// Get the config
	existingCfg, err := h.store.Get(c.Request.Context(), tenant, name)
	if err != nil {
		h.logger.Error("failed to get config",
			zap.String("config_name", name),
			zap.Error(err))
		i18n.RespondWithError(c, i18n.ErrInternalServer.WithParam("Reason", "Failed to get config: "+err.Error()))
		return
	}

	// Check tenant permission
	_, err = h.checkTenantPermission(c, existingCfg.Tenant, existingCfg)
	if err != nil {
		h.logger.Warn("tenant permission check failed",
			zap.String("tenant", existingCfg.Tenant),
			zap.Error(err))
		i18n.RespondWithError(c, err)
		return
	}

	// Set version as active in store
	if err := h.store.SetActiveVersion(c.Request.Context(), existingCfg.Tenant, name, version); err != nil {
		h.logger.Error("failed to set active version",
			zap.String("config_name", name),
			zap.Int("version", version),
			zap.Error(err))
		i18n.RespondWithError(c, i18n.ErrInternalServer.WithParam("Reason", "Failed to set active version: "+err.Error()))
		return
	}

	// Send reload signal to gateway using notifier
	if err := h.notifier.NotifyUpdate(c.Request.Context(), existingCfg); err != nil {
		h.logger.Error("failed to notify gateway", zap.Error(err))
		i18n.RespondWithError(c, i18n.ErrInternalServer.WithParam("Reason", "Failed to notify gateway: "+err.Error()))
		return
	}

	h.logger.Info("active version set successfully",
		zap.String("config_name", name),
		zap.Int("version", version))
	i18n.Success(i18n.SuccessMCPServerUpdated).With("status", "success").Send(c)
}

// HandleGetConfigNames handles the request to get all configuration names
func (h *MCP) HandleGetConfigNames(c *gin.Context) {
	includeDeleted := c.Query("includeDeleted") == "true"
	tenant := c.Query("tenant")

	configs, err := h.store.List(c.Request.Context(), includeDeleted)
	if err != nil {
		h.logger.Error("failed to list configs", zap.Error(err))
		i18n.RespondWithError(c, i18n.ErrInternalServer.WithParam("Reason", "Failed to list configs: "+err.Error()))
		return
	}

	// Get user from claims
	claims, exists := c.Get("claims")
	if !exists {
		h.logger.Warn("missing JWT claims in context")
		i18n.RespondWithError(c, i18n.ErrUnauthorized)
		return
	}
	jwtClaims := claims.(*jwt.Claims)

	// Get user information
	user, err := h.db.GetUserByUsername(c.Request.Context(), jwtClaims.Username)
	if err != nil {
		h.logger.Error("failed to get user info",
			zap.String("username", jwtClaims.Username),
			zap.Error(err))
		i18n.RespondWithError(c, i18n.ErrInternalServer.WithParam("Reason", "Failed to get user info: "+err.Error()))
		return
	}

	// Filter configs by tenant if user is not admin
	if user.Role != database.RoleAdmin {
		// Get user's tenants
		tenants, err := h.db.GetUserTenants(c.Request.Context(), user.ID)
		if err != nil {
			h.logger.Error("failed to get user tenants",
				zap.String("username", user.Username),
				zap.Error(err))
			i18n.RespondWithError(c, i18n.ErrInternalServer)
			return
		}

		// Create a map of tenant names for faster lookup
		userTenants := make(map[string]bool)
		for _, t := range tenants {
			userTenants[t.Name] = true
		}

		filteredConfigs := make([]*config.MCPConfig, 0)
		for _, cfg := range configs {
			if userTenants[cfg.Tenant] {
				filteredConfigs = append(filteredConfigs, cfg)
			}
		}
		configs = filteredConfigs
	}

	// Filter by tenant if specified
	if tenant != "" {
		filteredConfigs := make([]*config.MCPConfig, 0)
		for _, cfg := range configs {
			if cfg.Tenant == tenant {
				filteredConfigs = append(filteredConfigs, cfg)
			}
		}
		configs = filteredConfigs
	}

	// Extract unique config names
	names := make([]string, 0, len(configs))
	for _, cfg := range configs {
		names = append(names, cfg.Name)
	}

	c.JSON(http.StatusOK, gin.H{
		"data": names,
	})
}

// HandleGetCapabilities handles GET /api/mcp/capabilities/:tenant/:name
func (h *MCP) HandleGetCapabilities(c *gin.Context) {
	tenant := c.Param("tenant")
	if tenant == "" {
		h.logger.Warn("tenant parameter required but missing")
		i18n.RespondWithError(c, i18n.ErrorTenantRequired)
		return
	}

	name := c.Param("name")
	if name == "" {
		h.logger.Warn("MCP server name required but missing")
		i18n.RespondWithError(c, i18n.ErrorMCPServerNameRequired)
		return
	}

	h.logger.Info("handling get capabilities request",
		zap.String("tenant", tenant),
		zap.String("name", name))

	// Get MCP server configuration
	cfg, err := h.store.Get(c.Request.Context(), tenant, name)
	if err != nil {
		h.logger.Error("MCP server not found",
			zap.String("tenant", tenant),
			zap.String("name", name),
			zap.Error(err))
		i18n.RespondWithError(c, i18n.ErrorMCPServerNotFound.WithParam("Name", name))
		return
	}

	// Check tenant permission
	_, err = h.checkTenantPermission(c, cfg.Tenant, cfg)
	if err != nil {
		h.logger.Warn("tenant permission check failed",
			zap.String("tenant", cfg.Tenant),
			zap.Error(err))
		i18n.RespondWithError(c, err)
		return
	}

	// Check cache first
	cacheKey := tenant + ":" + name
	capabilities, err := h.getCapabilitiesFromCache(c.Request.Context(), cacheKey, cfg)
	if err != nil {
		h.logger.Error("failed to get capabilities",
			zap.String("tenant", tenant),
			zap.String("name", name),
			zap.Error(err))
		i18n.RespondWithError(c, i18n.ErrInternalServer.WithParam("Reason", "Failed to get capabilities: "+err.Error()))
		return
	}

	h.logger.Info("capabilities fetched successfully",
		zap.String("tenant", tenant),
		zap.String("name", name),
		zap.Int("tools_count", len(capabilities.Tools)),
		zap.Int("prompts_count", len(capabilities.Prompts)),
		zap.Int("resources_count", len(capabilities.Resources)),
		zap.Int("resource_templates_count", len(capabilities.ResourceTemplates)))

	i18n.Success(i18n.SuccessMCPCapabilities).With("data", capabilities).Send(c)
}

// fetchCapabilities fetches capabilities from all MCP servers in the configuration
func (h *MCP) fetchCapabilities(ctx context.Context, cfg *config.MCPConfig) (*mcp.CapabilitiesInfo, error) {
	capabilities := &mcp.CapabilitiesInfo{
		Tools:             make([]mcp.MCPTool, 0),
		Prompts:           make([]mcp.MCPPrompt, 0),
		Resources:         make([]mcp.MCPResource, 0),
		ResourceTemplates: make([]mcp.MCPResourceTemplate, 0),
		LastSynced:        time.Now().UTC().Format(time.RFC3339),
		ServerInfo:        make(map[string]interface{}),
	}

	var wg sync.WaitGroup
	var mu sync.Mutex
	errChan := make(chan error, len(cfg.McpServers)*4) // 4 operations per server

	// Fetch capabilities from each MCP server concurrently
	for _, mcpServerCfg := range cfg.McpServers {
		wg.Add(1)
		go func(serverCfg config.MCPServerConfig) {
			defer wg.Done()
			
			// Create transport for this MCP server
			transport, err := mcpproxy.NewTransport(serverCfg)
			if err != nil {
				h.logger.Error("failed to create transport",
					zap.String("server", serverCfg.Name),
					zap.Error(err))
				errChan <- err
				return
			}

			// Start transport if not running
			if !transport.IsRunning() {
				tmplCtx := template.NewContext()
				if err := transport.Start(ctx, tmplCtx); err != nil {
					h.logger.Error("failed to start transport",
						zap.String("server", serverCfg.Name),
						zap.Error(err))
					errChan <- err
					return
				}
				// Ensure transport is stopped after use
				defer func() {
					if stopErr := transport.Stop(ctx); stopErr != nil {
						h.logger.Warn("failed to stop transport",
							zap.String("server", serverCfg.Name),
							zap.Error(stopErr))
					}
				}()
			}

			// Fetch tools, prompts, resources, and resource templates concurrently
			var serverWg sync.WaitGroup
			serverWg.Add(4)

			// Fetch tools
			go func() {
				defer serverWg.Done()
				tools, err := transport.FetchTools(ctx)
				if err != nil {
					h.logger.Error("failed to fetch tools",
						zap.String("server", serverCfg.Name),
						zap.Error(err))
					errChan <- err
					return
				}
				
				// Convert to MCP tools
				mcpTools := make([]mcp.MCPTool, len(tools))
				for i, tool := range tools {
					mcpTools[i] = mcp.MCPTool{
						Name:        tool.Name,
						Description: tool.Description,
						InputSchema: tool.InputSchema,
						Annotations: tool.Annotations,
						Enabled:     true,
						LastSynced:  time.Now().UTC().Format(time.RFC3339),
					}
				}

				mu.Lock()
				capabilities.Tools = append(capabilities.Tools, mcpTools...)
				mu.Unlock()
			}()

			// Fetch prompts
			go func() {
				defer serverWg.Done()
				prompts, err := transport.FetchPrompts(ctx)
				if err != nil {
					h.logger.Error("failed to fetch prompts",
						zap.String("server", serverCfg.Name),
						zap.Error(err))
					errChan <- err
					return
				}
				
				// Convert to MCP prompts
				mcpPrompts := make([]mcp.MCPPrompt, len(prompts))
				for i, prompt := range prompts {
					mcpPrompts[i] = mcp.MCPPrompt{
						Name:        prompt.Name,
						Description: prompt.Description,
						Arguments:   prompt.Arguments,
						LastSynced:  time.Now().UTC().Format(time.RFC3339),
					}
				}

				mu.Lock()
				capabilities.Prompts = append(capabilities.Prompts, mcpPrompts...)
				mu.Unlock()
			}()

			// Fetch resources
			go func() {
				defer serverWg.Done()
				resources, err := transport.FetchResources(ctx)
				if err != nil {
					h.logger.Error("failed to fetch resources",
						zap.String("server", serverCfg.Name),
						zap.Error(err))
					errChan <- err
					return
				}
				
				// Convert to MCP resources
				mcpResources := make([]mcp.MCPResource, len(resources))
				for i, resource := range resources {
					mcpResources[i] = mcp.MCPResource{
						URI:         resource.URI,
						Name:        resource.Name,
						Description: resource.Description,
						MIMEType:    resource.MIMEType,
						LastSynced:  time.Now().UTC().Format(time.RFC3339),
					}
				}

				mu.Lock()
				capabilities.Resources = append(capabilities.Resources, mcpResources...)
				mu.Unlock()
			}()

			// Fetch resource templates
			go func() {
				defer serverWg.Done()
				templates, err := transport.FetchResourceTemplates(ctx)
				if err != nil {
					h.logger.Error("failed to fetch resource templates",
						zap.String("server", serverCfg.Name),
						zap.Error(err))
					errChan <- err
					return
				}
				
				// Convert to MCP resource templates
				mcpTemplates := make([]mcp.MCPResourceTemplate, len(templates))
				for i, template := range templates {
					mcpTemplates[i] = mcp.MCPResourceTemplate{
						URITemplate: template.URITemplate,
						Name:        template.Name,
						Description: template.Description,
						MIMEType:    template.MIMEType,
						Parameters:  template.Parameters,
						LastSynced:  time.Now().UTC().Format(time.RFC3339),
					}
				}

				mu.Lock()
				capabilities.ResourceTemplates = append(capabilities.ResourceTemplates, mcpTemplates...)
				mu.Unlock()
			}()

			serverWg.Wait()
		}(mcpServerCfg)
	}

	// Wait for all servers to complete
	wg.Wait()
	close(errChan)

	// Collect any errors
	var errors []error
	for err := range errChan {
		errors = append(errors, err)
	}

	// If there are errors but we got some capabilities, log warnings
	if len(errors) > 0 {
		h.logger.Warn("some capabilities could not be fetched",
			zap.Int("error_count", len(errors)),
			zap.Int("tools_fetched", len(capabilities.Tools)),
			zap.Int("prompts_fetched", len(capabilities.Prompts)))
		
		// If we didn't get any capabilities at all, return the first error
		if len(capabilities.Tools) == 0 && len(capabilities.Prompts) == 0 && 
			len(capabilities.Resources) == 0 && len(capabilities.ResourceTemplates) == 0 {
			return nil, errors[0]
		}
	}

	return capabilities, nil
}

// getCapabilitiesFromCache checks cache first, then fetches if needed
func (h *MCP) getCapabilitiesFromCache(ctx context.Context, cacheKey string, cfg *config.MCPConfig) (*mcp.CapabilitiesInfo, error) {
	// Check if we have cached data
	if cached, ok := h.capabilitiesCache.Load(cacheKey); ok {
		if cachedCaps, ok := cached.(*cachedCapabilities); ok {
			if !cachedCaps.isExpired() {
				h.logger.Debug("returning cached capabilities",
					zap.String("cache_key", cacheKey),
					zap.Time("cached_at", cachedCaps.timestamp))
				return cachedCaps.data, nil
			} else {
				h.logger.Debug("cached capabilities expired, removing from cache",
					zap.String("cache_key", cacheKey),
					zap.Time("cached_at", cachedCaps.timestamp))
				h.capabilitiesCache.Delete(cacheKey)
			}
		}
	}

	// Fetch fresh data
	h.logger.Debug("fetching fresh capabilities",
		zap.String("cache_key", cacheKey))
	
	capabilities, err := h.fetchCapabilities(ctx, cfg)
	if err != nil {
		return nil, err
	}

	// Merge with database-stored tool status configurations
	err = h.mergeToolStatusFromDB(ctx, cfg.Tenant, cfg.Name, capabilities)
	if err != nil {
		h.logger.Warn("failed to merge tool status from database",
			zap.String("tenant", cfg.Tenant),
			zap.String("server", cfg.Name),
			zap.Error(err))
		// Don't fail the entire request, just log the warning
	}

	// Cache the result with 5 minute TTL
	cached := &cachedCapabilities{
		data:      capabilities,
		timestamp: time.Now(),
		ttl:       5 * time.Minute,
	}
	h.capabilitiesCache.Store(cacheKey, cached)

	h.logger.Debug("capabilities cached successfully",
		zap.String("cache_key", cacheKey),
		zap.Time("cached_at", cached.timestamp),
		zap.Duration("ttl", cached.ttl))

	return capabilities, nil
}

// clearCapabilitiesCache clears cache for a specific server or all if key is empty
func (h *MCP) clearCapabilitiesCache(key string) {
	if key == "" {
		// Clear all cache
		h.capabilitiesCache.Range(func(k, v interface{}) bool {
			h.capabilitiesCache.Delete(k)
			return true
		})
		h.logger.Debug("cleared all capabilities cache")
	} else {
		h.capabilitiesCache.Delete(key)
		h.logger.Debug("cleared capabilities cache for key", zap.String("key", key))
	}
}

// mergeToolStatusFromDB merges tool status configurations from database into capabilities
func (h *MCP) mergeToolStatusFromDB(ctx context.Context, tenant, serverName string, capabilities *mcp.CapabilitiesInfo) error {
	if capabilities == nil || len(capabilities.Tools) == 0 {
		return nil
	}

	// Update each tool's enabled status from database
	for i := range capabilities.Tools {
		tool := &capabilities.Tools[i]
		if toolFromDB, err := h.capabilityStore.GetTool(ctx, tenant, serverName, tool.Name); err == nil {
			// Found status in database, use it
			tool.Enabled = toolFromDB.Enabled
			// Use the existing LastSynced from database if available
			if toolFromDB.LastSynced != "" {
				tool.LastSynced = toolFromDB.LastSynced
			}
		} else {
			// No status found in database, default to enabled
			tool.Enabled = true
		}
	}

	return nil
}

// HandleSyncCapabilities handles POST /api/mcp/capabilities/{tenant}/{name}/sync
func (h *MCP) HandleSyncCapabilities(c *gin.Context) {
	tenant := c.Param("tenant")
	if tenant == "" {
		h.logger.Warn("tenant parameter required but missing")
		i18n.RespondWithError(c, i18n.ErrorTenantRequired)
		return
	}

	name := c.Param("name")
	if name == "" {
		h.logger.Warn("MCP server name required but missing")
		i18n.RespondWithError(c, i18n.ErrorMCPServerNameRequired)
		return
	}

	h.logger.Info("handling sync capabilities request",
		zap.String("tenant", tenant),
		zap.String("name", name))

	// Parse request body
	var syncReq SyncRequest
	if err := c.ShouldBindJSON(&syncReq); err != nil {
		h.logger.Error("invalid sync request body", zap.Error(err))
		i18n.RespondWithError(c, i18n.ErrBadRequest.WithParam("Reason", "Invalid request body: "+err.Error()))
		return
	}

	// Get MCP server configuration
	cfg, err := h.store.Get(c.Request.Context(), tenant, name)
	if err != nil {
		h.logger.Error("MCP server not found",
			zap.String("tenant", tenant),
			zap.String("name", name),
			zap.Error(err))
		i18n.RespondWithError(c, i18n.ErrorMCPServerNotFound.WithParam("Name", name))
		return
	}

	// Check tenant permission
	_, err = h.checkTenantPermission(c, cfg.Tenant, cfg)
	if err != nil {
		h.logger.Warn("tenant permission check failed",
			zap.String("tenant", cfg.Tenant),
			zap.Error(err))
		i18n.RespondWithError(c, err)
		return
	}

	// Get user information for sync record
	claims, exists := c.Get("claims")
	if !exists {
		h.logger.Warn("missing JWT claims in context")
		i18n.RespondWithError(c, i18n.ErrUnauthorized)
		return
	}
	jwtClaims := claims.(*jwt.Claims)

	user, err := h.db.GetUserByUsername(c.Request.Context(), jwtClaims.Username)
	if err != nil {
		h.logger.Error("failed to get user info",
			zap.String("username", jwtClaims.Username),
			zap.Error(err))
		i18n.RespondWithError(c, i18n.ErrInternalServer.WithParam("Reason", "Failed to get user info: "+err.Error()))
		return
	}

	// Generate sync ID
	syncID := uuid.New().String()

	// Default to all types if none specified
	if len(syncReq.Types) == 0 {
		syncReq.Types = []string{"tools", "prompts", "resources", "resourceTemplates"}
	}

	// Create sync record
	syncTypesJSON, _ := json.Marshal(syncReq.Types)
	syncRecord := &storage.SyncHistoryModel{
		Tenant:     tenant,
		ServerName: name,
		SyncID:     syncID,
		Status:     storage.SyncStatusRunning,
		SyncTypes:  string(syncTypesJSON),
		StartedAt:  time.Now(),
		Progress:   0,
		UserID:     user.ID,
	}

	if err := h.capabilityStore.CreateSyncRecord(c.Request.Context(), syncRecord); err != nil {
		h.logger.Error("failed to create sync record",
			zap.String("sync_id", syncID),
			zap.Error(err))
		i18n.RespondWithError(c, i18n.ErrInternalServer.WithParam("Reason", "Failed to create sync record: "+err.Error()))
		return
	}

	// Start async sync operation
	go h.performSync(context.Background(), cfg, syncID, syncReq.Types, syncReq.Force)

	// Return sync response
	response := SyncResponse{
		SyncID:    syncID,
		Status:    storage.SyncStatusRunning,
		StartedAt: syncRecord.StartedAt,
	}

	h.logger.Info("sync operation started",
		zap.String("sync_id", syncID),
		zap.String("tenant", tenant),
		zap.String("name", name))

	c.JSON(http.StatusAccepted, gin.H{
		"success": true,
		"message": "Sync operation started",
		"data":    response,
	})
}

// performSync performs the actual synchronization in the background
func (h *MCP) performSync(ctx context.Context, cfg *config.MCPConfig, syncID string, types []string, force bool) {
	defer func() {
		if r := recover(); r != nil {
			h.logger.Error("sync operation panicked",
				zap.String("sync_id", syncID),
				zap.Any("panic", r))
			
			errorMsg := fmt.Sprintf("Sync operation failed with panic: %v", r)
			_ = h.capabilityStore.CompleteSyncRecord(ctx, syncID, storage.SyncStatusFailed, "")
			_ = h.capabilityStore.UpdateSyncRecord(ctx, syncID, storage.SyncStatusFailed, 100, errorMsg, "")
			// We can't get cfg here in panic recovery, so we'll use empty strings
			h.wsManager.BroadcastSyncStatus(ctx, syncID, "", "", "failed", 100, errorMsg)
		}
	}()

	h.logger.Info("starting sync operation",
		zap.String("sync_id", syncID),
		zap.String("tenant", cfg.Tenant),
		zap.String("name", cfg.Name),
		zap.Strings("types", types))

	// Broadcast sync start
	h.wsManager.BroadcastSyncStatus(ctx, syncID, cfg.Tenant, cfg.Name, "running", 0, "Sync operation started")

	// Fetch fresh capabilities from MCP servers
	_ = h.capabilityStore.UpdateSyncRecord(ctx, syncID, storage.SyncStatusRunning, 20, "", "")
	h.wsManager.BroadcastSyncStatus(ctx, syncID, cfg.Tenant, cfg.Name, "running", 20, "Fetching capabilities from MCP servers")
	
	freshCapabilities, err := h.fetchCapabilities(ctx, cfg)
	if err != nil {
		h.logger.Error("failed to fetch fresh capabilities",
			zap.String("sync_id", syncID),
			zap.Error(err))
		
		errorMsg := fmt.Sprintf("Failed to fetch capabilities: %v", err)
		_ = h.capabilityStore.CompleteSyncRecord(ctx, syncID, storage.SyncStatusFailed, "")
		_ = h.capabilityStore.UpdateSyncRecord(ctx, syncID, storage.SyncStatusFailed, 100, errorMsg, "")
		h.wsManager.BroadcastSyncStatus(ctx, syncID, cfg.Tenant, cfg.Name, "failed", 100, errorMsg)
		return
	}

	// Get existing capabilities from database
	_ = h.capabilityStore.UpdateSyncRecord(ctx, syncID, storage.SyncStatusRunning, 40, "", "")
	h.wsManager.BroadcastSyncStatus(ctx, syncID, cfg.Tenant, cfg.Name, "running", 40, "Loading existing capabilities from database")
	
	existingCapabilities, err := h.capabilityStore.GetCapabilitiesInfo(ctx, cfg.Tenant, cfg.Name)
	if err != nil {
		// If no existing capabilities, treat as empty
		existingCapabilities = &mcp.CapabilitiesInfo{
			Tools:             []mcp.MCPTool{},
			Prompts:           []mcp.MCPPrompt{},
			Resources:         []mcp.MCPResource{},
			ResourceTemplates: []mcp.MCPResourceTemplate{},
		}
		h.logger.Info("no existing capabilities found, treating as empty",
			zap.String("sync_id", syncID))
	}

	// Perform comparison and sync
	_ = h.capabilityStore.UpdateSyncRecord(ctx, syncID, storage.SyncStatusRunning, 60, "", "")
	h.wsManager.BroadcastSyncStatus(ctx, syncID, cfg.Tenant, cfg.Name, "running", 60, "Comparing and synchronizing capabilities")
	
	summary, err := h.syncCapabilityData(ctx, cfg, freshCapabilities, existingCapabilities, types, force)
	if err != nil {
		h.logger.Error("failed to sync capability data",
			zap.String("sync_id", syncID),
			zap.Error(err))
		
		errorMsg := fmt.Sprintf("Failed to sync capability data: %v", err)
		_ = h.capabilityStore.CompleteSyncRecord(ctx, syncID, storage.SyncStatusFailed, "")
		_ = h.capabilityStore.UpdateSyncRecord(ctx, syncID, storage.SyncStatusFailed, 100, errorMsg, "")
		h.wsManager.BroadcastSyncStatus(ctx, syncID, cfg.Tenant, cfg.Name, "failed", 100, errorMsg)
		return
	}

	// Complete sync
	_ = h.capabilityStore.UpdateSyncRecord(ctx, syncID, storage.SyncStatusRunning, 90, "", "")
	h.wsManager.BroadcastSyncStatus(ctx, syncID, cfg.Tenant, cfg.Name, "running", 90, "Finalizing sync operation")
	
	summaryJSON, _ := json.Marshal(summary)
	
	status := storage.SyncStatusSuccess
	if len(summary.Errors) > 0 {
		status = storage.SyncStatusPartial
	}
	
	if err := h.capabilityStore.CompleteSyncRecord(ctx, syncID, status, string(summaryJSON)); err != nil {
		h.logger.Error("failed to complete sync record",
			zap.String("sync_id", syncID),
			zap.Error(err))
	}

	// Clear cache for this server
	cacheKey := cfg.Tenant + ":" + cfg.Name
	h.clearCapabilitiesCache(cacheKey)
	
	// Clear multi-layer cache if available
	if h.cacheManager != nil {
		if err := h.cacheManager.InvalidateAll(ctx, cfg.Tenant, cfg.Name); err != nil {
			h.logger.Warn("failed to invalidate multi-layer cache",
				zap.String("tenant", cfg.Tenant),
				zap.String("server", cfg.Name),
				zap.Error(err))
		}
	}

	// Broadcast completion
	statusMsg := fmt.Sprintf("Sync completed successfully with %d total changes", summary.getTotalChanges())
	if status == storage.SyncStatusPartial {
		statusMsg = fmt.Sprintf("Sync completed with warnings (%d errors, %d changes)", len(summary.Errors), summary.getTotalChanges())
	}
	h.wsManager.BroadcastSyncStatus(ctx, syncID, cfg.Tenant, cfg.Name, string(status), 100, statusMsg)

	h.logger.Info("sync operation completed successfully",
		zap.String("sync_id", syncID),
		zap.String("status", string(status)),
		zap.Int("total_changes", summary.getTotalChanges()))
}

// syncCapabilityData performs the actual sync logic with conflict detection
func (h *MCP) syncCapabilityData(ctx context.Context, cfg *config.MCPConfig, fresh, existing *mcp.CapabilitiesInfo, types []string, force bool) (*SyncSummary, error) {
	summary := &SyncSummary{
		Conflicts: []CapabilityConflict{},
		Errors:    []string{},
	}

	// Create a map of requested sync types
	syncTypes := make(map[string]bool)
	for _, t := range types {
		syncTypes[t] = true
	}

	// Sync tools if requested
	if syncTypes["tools"] {
		if err := h.syncTools(ctx, cfg, fresh.Tools, existing.Tools, summary, force); err != nil {
			summary.Errors = append(summary.Errors, fmt.Sprintf("Failed to sync tools: %v", err))
		}
	}

	// Sync prompts if requested
	if syncTypes["prompts"] {
		if err := h.syncPrompts(ctx, cfg, fresh.Prompts, existing.Prompts, summary, force); err != nil {
			summary.Errors = append(summary.Errors, fmt.Sprintf("Failed to sync prompts: %v", err))
		}
	}

	// Sync resources if requested
	if syncTypes["resources"] {
		if err := h.syncResources(ctx, cfg, fresh.Resources, existing.Resources, summary, force); err != nil {
			summary.Errors = append(summary.Errors, fmt.Sprintf("Failed to sync resources: %v", err))
		}
	}

	// Sync resource templates if requested
	if syncTypes["resourceTemplates"] {
		if err := h.syncResourceTemplates(ctx, cfg, fresh.ResourceTemplates, existing.ResourceTemplates, summary, force); err != nil {
			summary.Errors = append(summary.Errors, fmt.Sprintf("Failed to sync resource templates: %v", err))
		}
	}

	return summary, nil
}

// getTotalChanges returns the total number of changes made during sync
func (s *SyncSummary) getTotalChanges() int {
	return s.ToolsAdded + s.ToolsUpdated + s.ToolsRemoved +
		s.PromptsAdded + s.PromptsUpdated + s.PromptsRemoved +
		s.ResourcesAdded + s.ResourcesUpdated + s.ResourcesRemoved +
		s.ResourceTemplatesAdded + s.ResourceTemplatesUpdated + s.ResourceTemplatesRemoved
}

// syncTools synchronizes tools between fresh and existing data
func (h *MCP) syncTools(ctx context.Context, cfg *config.MCPConfig, fresh, existing []mcp.MCPTool, summary *SyncSummary, force bool) error {
	// Create maps for efficient lookup
	existingMap := make(map[string]mcp.MCPTool)
	for _, tool := range existing {
		existingMap[tool.Name] = tool
	}

	freshMap := make(map[string]mcp.MCPTool)
	for _, tool := range fresh {
		freshMap[tool.Name] = tool
	}

	// Find tools to add or update
	for _, freshTool := range fresh {
		if existingTool, exists := existingMap[freshTool.Name]; exists {
			// Tool exists, check if update is needed
			if h.hasToolChanged(freshTool, existingTool) || force {
				if err := h.capabilityStore.SaveTool(ctx, &freshTool, cfg.Tenant, cfg.Name); err != nil {
					return fmt.Errorf("failed to update tool %s: %w", freshTool.Name, err)
				}
				summary.ToolsUpdated++
				h.logger.Debug("tool updated",
					zap.String("name", freshTool.Name),
					zap.String("tenant", cfg.Tenant),
					zap.String("server", cfg.Name))
			}
		} else {
			// Tool is new, add it
			if err := h.capabilityStore.SaveTool(ctx, &freshTool, cfg.Tenant, cfg.Name); err != nil {
				return fmt.Errorf("failed to add tool %s: %w", freshTool.Name, err)
			}
			summary.ToolsAdded++
			h.logger.Debug("tool added",
				zap.String("name", freshTool.Name),
				zap.String("tenant", cfg.Tenant),
				zap.String("server", cfg.Name))
		}
	}

	// Find tools to remove (exist in database but not in fresh data)
	for _, existingTool := range existing {
		if _, exists := freshMap[existingTool.Name]; !exists {
			if err := h.capabilityStore.DeleteTool(ctx, cfg.Tenant, cfg.Name, existingTool.Name); err != nil {
				return fmt.Errorf("failed to remove tool %s: %w", existingTool.Name, err)
			}
			summary.ToolsRemoved++
			h.logger.Debug("tool removed",
				zap.String("name", existingTool.Name),
				zap.String("tenant", cfg.Tenant),
				zap.String("server", cfg.Name))
		}
	}

	return nil
}

// syncPrompts synchronizes prompts between fresh and existing data
func (h *MCP) syncPrompts(ctx context.Context, cfg *config.MCPConfig, fresh, existing []mcp.MCPPrompt, summary *SyncSummary, force bool) error {
	// Create maps for efficient lookup
	existingMap := make(map[string]mcp.MCPPrompt)
	for _, prompt := range existing {
		existingMap[prompt.Name] = prompt
	}

	freshMap := make(map[string]mcp.MCPPrompt)
	for _, prompt := range fresh {
		freshMap[prompt.Name] = prompt
	}

	// Find prompts to add or update
	for _, freshPrompt := range fresh {
		if existingPrompt, exists := existingMap[freshPrompt.Name]; exists {
			// Prompt exists, check if update is needed
			if h.hasPromptChanged(freshPrompt, existingPrompt) || force {
				if err := h.capabilityStore.SavePrompt(ctx, &freshPrompt, cfg.Tenant, cfg.Name); err != nil {
					return fmt.Errorf("failed to update prompt %s: %w", freshPrompt.Name, err)
				}
				summary.PromptsUpdated++
				h.logger.Debug("prompt updated",
					zap.String("name", freshPrompt.Name),
					zap.String("tenant", cfg.Tenant),
					zap.String("server", cfg.Name))
			}
		} else {
			// Prompt is new, add it
			if err := h.capabilityStore.SavePrompt(ctx, &freshPrompt, cfg.Tenant, cfg.Name); err != nil {
				return fmt.Errorf("failed to add prompt %s: %w", freshPrompt.Name, err)
			}
			summary.PromptsAdded++
			h.logger.Debug("prompt added",
				zap.String("name", freshPrompt.Name),
				zap.String("tenant", cfg.Tenant),
				zap.String("server", cfg.Name))
		}
	}

	// Find prompts to remove
	for _, existingPrompt := range existing {
		if _, exists := freshMap[existingPrompt.Name]; !exists {
			if err := h.capabilityStore.DeletePrompt(ctx, cfg.Tenant, cfg.Name, existingPrompt.Name); err != nil {
				return fmt.Errorf("failed to remove prompt %s: %w", existingPrompt.Name, err)
			}
			summary.PromptsRemoved++
			h.logger.Debug("prompt removed",
				zap.String("name", existingPrompt.Name),
				zap.String("tenant", cfg.Tenant),
				zap.String("server", cfg.Name))
		}
	}

	return nil
}

// syncResources synchronizes resources between fresh and existing data
func (h *MCP) syncResources(ctx context.Context, cfg *config.MCPConfig, fresh, existing []mcp.MCPResource, summary *SyncSummary, force bool) error {
	// Create maps for efficient lookup
	existingMap := make(map[string]mcp.MCPResource)
	for _, resource := range existing {
		existingMap[resource.URI] = resource
	}

	freshMap := make(map[string]mcp.MCPResource)
	for _, resource := range fresh {
		freshMap[resource.URI] = resource
	}

	// Find resources to add or update
	for _, freshResource := range fresh {
		if existingResource, exists := existingMap[freshResource.URI]; exists {
			// Resource exists, check if update is needed
			if h.hasResourceChanged(freshResource, existingResource) || force {
				if err := h.capabilityStore.SaveResource(ctx, &freshResource, cfg.Tenant, cfg.Name); err != nil {
					return fmt.Errorf("failed to update resource %s: %w", freshResource.URI, err)
				}
				summary.ResourcesUpdated++
				h.logger.Debug("resource updated",
					zap.String("uri", freshResource.URI),
					zap.String("tenant", cfg.Tenant),
					zap.String("server", cfg.Name))
			}
		} else {
			// Resource is new, add it
			if err := h.capabilityStore.SaveResource(ctx, &freshResource, cfg.Tenant, cfg.Name); err != nil {
				return fmt.Errorf("failed to add resource %s: %w", freshResource.URI, err)
			}
			summary.ResourcesAdded++
			h.logger.Debug("resource added",
				zap.String("uri", freshResource.URI),
				zap.String("tenant", cfg.Tenant),
				zap.String("server", cfg.Name))
		}
	}

	// Find resources to remove
	for _, existingResource := range existing {
		if _, exists := freshMap[existingResource.URI]; !exists {
			if err := h.capabilityStore.DeleteResource(ctx, cfg.Tenant, cfg.Name, existingResource.URI); err != nil {
				return fmt.Errorf("failed to remove resource %s: %w", existingResource.URI, err)
			}
			summary.ResourcesRemoved++
			h.logger.Debug("resource removed",
				zap.String("uri", existingResource.URI),
				zap.String("tenant", cfg.Tenant),
				zap.String("server", cfg.Name))
		}
	}

	return nil
}

// syncResourceTemplates synchronizes resource templates between fresh and existing data
func (h *MCP) syncResourceTemplates(ctx context.Context, cfg *config.MCPConfig, fresh, existing []mcp.MCPResourceTemplate, summary *SyncSummary, force bool) error {
	// Create maps for efficient lookup
	existingMap := make(map[string]mcp.MCPResourceTemplate)
	for _, template := range existing {
		existingMap[template.URITemplate] = template
	}

	freshMap := make(map[string]mcp.MCPResourceTemplate)
	for _, template := range fresh {
		freshMap[template.URITemplate] = template
	}

	// Find templates to add or update
	for _, freshTemplate := range fresh {
		if existingTemplate, exists := existingMap[freshTemplate.URITemplate]; exists {
			// Template exists, check if update is needed
			if h.hasResourceTemplateChanged(freshTemplate, existingTemplate) || force {
				if err := h.capabilityStore.SaveResourceTemplate(ctx, &freshTemplate, cfg.Tenant, cfg.Name); err != nil {
					return fmt.Errorf("failed to update resource template %s: %w", freshTemplate.URITemplate, err)
				}
				summary.ResourceTemplatesUpdated++
				h.logger.Debug("resource template updated",
					zap.String("uri_template", freshTemplate.URITemplate),
					zap.String("tenant", cfg.Tenant),
					zap.String("server", cfg.Name))
			}
		} else {
			// Template is new, add it
			if err := h.capabilityStore.SaveResourceTemplate(ctx, &freshTemplate, cfg.Tenant, cfg.Name); err != nil {
				return fmt.Errorf("failed to add resource template %s: %w", freshTemplate.URITemplate, err)
			}
			summary.ResourceTemplatesAdded++
			h.logger.Debug("resource template added",
				zap.String("uri_template", freshTemplate.URITemplate),
				zap.String("tenant", cfg.Tenant),
				zap.String("server", cfg.Name))
		}
	}

	// Find templates to remove
	for _, existingTemplate := range existing {
		if _, exists := freshMap[existingTemplate.URITemplate]; !exists {
			if err := h.capabilityStore.DeleteResourceTemplate(ctx, cfg.Tenant, cfg.Name, existingTemplate.URITemplate); err != nil {
				return fmt.Errorf("failed to remove resource template %s: %w", existingTemplate.URITemplate, err)
			}
			summary.ResourceTemplatesRemoved++
			h.logger.Debug("resource template removed",
				zap.String("uri_template", existingTemplate.URITemplate),
				zap.String("tenant", cfg.Tenant),
				zap.String("server", cfg.Name))
		}
	}

	return nil
}

// hasToolChanged checks if a tool has changed between fresh and existing versions
func (h *MCP) hasToolChanged(fresh, existing mcp.MCPTool) bool {
	// Compare key fields that would indicate a change
	if fresh.Description != existing.Description ||
		fresh.Enabled != existing.Enabled {
		return true
	}

	// Compare input schema (JSON comparison)
	freshSchemaJSON, _ := json.Marshal(fresh.InputSchema)
	existingSchemaJSON, _ := json.Marshal(existing.InputSchema)
	if string(freshSchemaJSON) != string(existingSchemaJSON) {
		return true
	}

	// Compare annotations if both have them
	if fresh.Annotations != nil && existing.Annotations != nil {
		freshAnnotationsJSON, _ := json.Marshal(fresh.Annotations)
		existingAnnotationsJSON, _ := json.Marshal(existing.Annotations)
		return string(freshAnnotationsJSON) != string(existingAnnotationsJSON)
	}

	// If one has annotations and the other doesn't
	return (fresh.Annotations != nil) != (existing.Annotations != nil)
}

// hasPromptChanged checks if a prompt has changed between fresh and existing versions
func (h *MCP) hasPromptChanged(fresh, existing mcp.MCPPrompt) bool {
	if fresh.Description != existing.Description {
		return true
	}

	// Compare arguments
	freshArgsJSON, _ := json.Marshal(fresh.Arguments)
	existingArgsJSON, _ := json.Marshal(existing.Arguments)
	if string(freshArgsJSON) != string(existingArgsJSON) {
		return true
	}

	// Compare prompt responses
	freshResponseJSON, _ := json.Marshal(fresh.PromptResponse)
	existingResponseJSON, _ := json.Marshal(existing.PromptResponse)
	return string(freshResponseJSON) != string(existingResponseJSON)
}

// hasResourceChanged checks if a resource has changed between fresh and existing versions
func (h *MCP) hasResourceChanged(fresh, existing mcp.MCPResource) bool {
	return fresh.Name != existing.Name ||
		fresh.Description != existing.Description ||
		fresh.MIMEType != existing.MIMEType
}

// hasResourceTemplateChanged checks if a resource template has changed between fresh and existing versions
func (h *MCP) hasResourceTemplateChanged(fresh, existing mcp.MCPResourceTemplate) bool {
	if fresh.Name != existing.Name ||
		fresh.Description != existing.Description ||
		fresh.MIMEType != existing.MIMEType {
		return true
	}

	// Compare parameters
	freshParamsJSON, _ := json.Marshal(fresh.Parameters)
	existingParamsJSON, _ := json.Marshal(existing.Parameters)
	return string(freshParamsJSON) != string(existingParamsJSON)
}

// HandleGetSyncStatus handles GET /api/mcp/capabilities/{tenant}/{name}/sync/{syncId}
func (h *MCP) HandleGetSyncStatus(c *gin.Context) {
	tenant := c.Param("tenant")
	if tenant == "" {
		h.logger.Warn("tenant parameter required but missing")
		i18n.RespondWithError(c, i18n.ErrorTenantRequired)
		return
	}

	name := c.Param("name")
	if name == "" {
		h.logger.Warn("MCP server name required but missing")
		i18n.RespondWithError(c, i18n.ErrorMCPServerNameRequired)
		return
	}

	syncID := c.Param("syncId")
	if syncID == "" {
		h.logger.Warn("sync ID required but missing")
		i18n.RespondWithError(c, i18n.ErrBadRequest.WithParam("Reason", "Sync ID is required"))
		return
	}

	// Get MCP server configuration to check permissions
	cfg, err := h.store.Get(c.Request.Context(), tenant, name)
	if err != nil {
		h.logger.Error("MCP server not found",
			zap.String("tenant", tenant),
			zap.String("name", name),
			zap.Error(err))
		i18n.RespondWithError(c, i18n.ErrorMCPServerNotFound.WithParam("Name", name))
		return
	}

	// Check tenant permission
	_, err = h.checkTenantPermission(c, cfg.Tenant, cfg)
	if err != nil {
		h.logger.Warn("tenant permission check failed",
			zap.String("tenant", cfg.Tenant),
			zap.Error(err))
		i18n.RespondWithError(c, err)
		return
	}

	// Get sync record
	syncRecord, err := h.capabilityStore.GetSyncRecord(c.Request.Context(), syncID)
	if err != nil {
		h.logger.Error("sync record not found",
			zap.String("sync_id", syncID),
			zap.Error(err))
		i18n.RespondWithError(c, i18n.ErrBadRequest.WithParam("Reason", "Sync record not found"))
		return
	}

	// Verify sync record belongs to the specified server
	if syncRecord.Tenant != tenant || syncRecord.ServerName != name {
		h.logger.Warn("sync record mismatch",
			zap.String("sync_id", syncID),
			zap.String("record_tenant", syncRecord.Tenant),
			zap.String("record_server", syncRecord.ServerName),
			zap.String("requested_tenant", tenant),
			zap.String("requested_server", name))
		i18n.RespondWithError(c, i18n.ErrBadRequest.WithParam("Reason", "Sync record not found for this server"))
		return
	}

	// Parse summary if available
	var summary map[string]interface{}
	if syncRecord.Summary != "" {
		if err := json.Unmarshal([]byte(syncRecord.Summary), &summary); err != nil {
			h.logger.Warn("failed to parse sync summary", zap.Error(err))
		}
	}

	response := SyncResponse{
		SyncID:    syncRecord.SyncID,
		Status:    syncRecord.Status,
		StartedAt: syncRecord.StartedAt,
		Summary:   summary,
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    response,
	})
}

// HandleGetSyncHistory handles GET /api/mcp/capabilities/{tenant}/{name}/sync
func (h *MCP) HandleGetSyncHistory(c *gin.Context) {
	tenant := c.Param("tenant")
	if tenant == "" {
		h.logger.Warn("tenant parameter required but missing")
		i18n.RespondWithError(c, i18n.ErrorTenantRequired)
		return
	}

	name := c.Param("name")
	if name == "" {
		h.logger.Warn("MCP server name required but missing")
		i18n.RespondWithError(c, i18n.ErrorMCPServerNameRequired)
		return
	}

	// Parse query parameters
	limitStr := c.DefaultQuery("limit", "20")
	offsetStr := c.DefaultQuery("offset", "0")

	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit < 1 || limit > 100 {
		limit = 20
	}

	offset, err := strconv.Atoi(offsetStr)
	if err != nil || offset < 0 {
		offset = 0
	}

	// Get MCP server configuration to check permissions
	cfg, err := h.store.Get(c.Request.Context(), tenant, name)
	if err != nil {
		h.logger.Error("MCP server not found",
			zap.String("tenant", tenant),
			zap.String("name", name),
			zap.Error(err))
		i18n.RespondWithError(c, i18n.ErrorMCPServerNotFound.WithParam("Name", name))
		return
	}

	// Check tenant permission
	_, err = h.checkTenantPermission(c, cfg.Tenant, cfg)
	if err != nil {
		h.logger.Warn("tenant permission check failed",
			zap.String("tenant", cfg.Tenant),
			zap.Error(err))
		i18n.RespondWithError(c, err)
		return
	}

	// Get sync history
	records, err := h.capabilityStore.ListSyncHistory(c.Request.Context(), tenant, name, limit, offset)
	if err != nil {
		h.logger.Error("failed to get sync history",
			zap.String("tenant", tenant),
			zap.String("name", name),
			zap.Error(err))
		i18n.RespondWithError(c, i18n.ErrInternalServer.WithParam("Reason", "Failed to get sync history: "+err.Error()))
		return
	}

	// Convert records to API format
	historyItems := make([]map[string]interface{}, len(records))
	for i, record := range records {
		var summary map[string]interface{}
		if record.Summary != "" {
			json.Unmarshal([]byte(record.Summary), &summary)
		}

		var syncTypes []string
		if record.SyncTypes != "" {
			json.Unmarshal([]byte(record.SyncTypes), &syncTypes)
		}

		historyItem := map[string]interface{}{
			"syncId":      record.SyncID,
			"status":      record.Status,
			"syncTypes":   syncTypes,
			"startedAt":   record.StartedAt,
			"progress":    record.Progress,
			"summary":     summary,
		}

		if record.CompletedAt != nil {
			historyItem["completedAt"] = *record.CompletedAt
		}

		if record.ErrorMessage != "" {
			historyItem["errorMessage"] = record.ErrorMessage
		}

		historyItems[i] = historyItem
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    historyItems,
	})
}

// HandleUpdateToolStatus handles PUT /api/mcp/capabilities/{tenant}/{name}/tools/{toolName}/status
func (h *MCP) HandleUpdateToolStatus(c *gin.Context) {
	tenant := c.Param("tenant")
	if tenant == "" {
		h.logger.Warn("tenant parameter required but missing")
		i18n.RespondWithError(c, i18n.ErrorTenantRequired)
		return
	}

	name := c.Param("name")
	if name == "" {
		h.logger.Warn("MCP server name required but missing")
		i18n.RespondWithError(c, i18n.ErrorMCPServerNameRequired)
		return
	}

	toolName := c.Param("toolName")
	if toolName == "" {
		h.logger.Warn("tool name required but missing")
		i18n.RespondWithError(c, i18n.ErrBadRequest.WithParam("Reason", "Tool name is required"))
		return
	}

	h.logger.Info("handling update tool status request",
		zap.String("tenant", tenant),
		zap.String("server_name", name),
		zap.String("tool_name", toolName))

	// Parse request body
	var req UpdateToolStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Error("invalid update tool status request body", zap.Error(err))
		i18n.RespondWithError(c, i18n.ErrBadRequest.WithParam("Reason", "Invalid request body: "+err.Error()))
		return
	}

	// Get MCP server configuration to check permissions
	cfg, err := h.store.Get(c.Request.Context(), tenant, name)
	if err != nil {
		h.logger.Error("MCP server not found",
			zap.String("tenant", tenant),
			zap.String("server_name", name),
			zap.Error(err))
		i18n.RespondWithError(c, i18n.ErrorMCPServerNotFound.WithParam("Name", name))
		return
	}

	// Check tenant permission
	_, err = h.checkTenantPermission(c, cfg.Tenant, cfg)
	if err != nil {
		h.logger.Warn("tenant permission check failed",
			zap.String("tenant", cfg.Tenant),
			zap.Error(err))
		i18n.RespondWithError(c, err)
		return
	}

	// Verify that the tool exists
	existingTool, err := h.capabilityStore.GetTool(c.Request.Context(), tenant, name, toolName)
	if err != nil {
		h.logger.Error("tool not found",
			zap.String("tenant", tenant),
			zap.String("server_name", name),
			zap.String("tool_name", toolName),
			zap.Error(err))
		i18n.RespondWithError(c, i18n.ErrBadRequest.WithParam("Reason", "Tool not found: "+toolName))
		return
	}

	// Check if status is already set to the requested value
	if existingTool.Enabled == req.Enabled {
		h.logger.Debug("tool status already matches requested value",
			zap.String("tool_name", toolName),
			zap.Bool("enabled", req.Enabled))
		
		response := UpdateToolStatusResponse{
			Success:   true,
			Message:   fmt.Sprintf("Tool %s status is already %t", toolName, req.Enabled),
			ToolName:  toolName,
			Enabled:   req.Enabled,
			UpdatedAt: time.Now().Format(time.RFC3339),
		}
		
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"data":    response,
		})
		return
	}

	// Get user information for history record
	claims, exists := c.Get("claims")
	if !exists {
		h.logger.Warn("missing JWT claims in context")
		i18n.RespondWithError(c, i18n.ErrUnauthorized)
		return
	}
	jwtClaims := claims.(*jwt.Claims)

	user, err := h.db.GetUserByUsername(c.Request.Context(), jwtClaims.Username)
	if err != nil {
		h.logger.Error("failed to get user info",
			zap.String("username", jwtClaims.Username),
			zap.Error(err))
		i18n.RespondWithError(c, i18n.ErrInternalServer.WithParam("Reason", "Failed to get user info: "+err.Error()))
		return
	}

	// Update the tool status in the database
	if err := h.capabilityStore.UpdateToolStatus(c.Request.Context(), tenant, name, toolName, req.Enabled); err != nil {
		h.logger.Error("failed to update tool status",
			zap.String("tenant", tenant),
			zap.String("server_name", name),
			zap.String("tool_name", toolName),
			zap.Bool("enabled", req.Enabled),
			zap.Error(err))
		i18n.RespondWithError(c, i18n.ErrInternalServer.WithParam("Reason", "Failed to update tool status: "+err.Error()))
		return
	}

	// Record the status change in history
	if err := h.capabilityStore.RecordToolStatusChange(c.Request.Context(), tenant, name, toolName, existingTool.Enabled, req.Enabled, user.ID, req.Reason); err != nil {
		h.logger.Warn("failed to record tool status change history",
			zap.String("tenant", tenant),
			zap.String("server_name", name),
			zap.String("tool_name", toolName),
			zap.Error(err))
		// Don't fail the request if history recording fails, just log warning
	}

	// Clear cache for this server to reflect status changes
	cacheKey := tenant + ":" + name
	h.clearCapabilitiesCache(cacheKey)

	// Send reload notification to gateway so it can update its configuration
	if err := h.notifier.NotifyUpdate(c.Request.Context(), cfg); err != nil {
		h.logger.Warn("failed to notify gateway about tool status change", 
			zap.String("tenant", tenant),
			zap.String("server_name", name),
			zap.String("tool_name", toolName),
			zap.Error(err))
		// Don't fail the request if notification fails, just log warning
	}

	statusAction := "disabled"
	if req.Enabled {
		statusAction = "enabled"
	}

	h.logger.Info("tool status updated successfully",
		zap.String("tenant", tenant),
		zap.String("server_name", name),
		zap.String("tool_name", toolName),
		zap.Bool("enabled", req.Enabled))

	// Clear cache for this server to ensure fresh data on next request
	cacheKey = tenant + ":" + name
	h.clearCapabilitiesCache(cacheKey)

	response := UpdateToolStatusResponse{
		Success:   true,
		Message:   fmt.Sprintf("Tool %s %s successfully", toolName, statusAction),
		ToolName:  toolName,
		Enabled:   req.Enabled,
		UpdatedAt: time.Now().Format(time.RFC3339),
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    response,
	})
}

// HandleBatchUpdateToolStatus handles PUT /api/mcp/capabilities/{tenant}/{name}/tools/status
func (h *MCP) HandleBatchUpdateToolStatus(c *gin.Context) {
	tenant := c.Param("tenant")
	if tenant == "" {
		h.logger.Warn("tenant parameter required but missing")
		i18n.RespondWithError(c, i18n.ErrorTenantRequired)
		return
	}

	name := c.Param("name")
	if name == "" {
		h.logger.Warn("MCP server name required but missing")
		i18n.RespondWithError(c, i18n.ErrorMCPServerNameRequired)
		return
	}

	h.logger.Info("handling batch update tool status request",
		zap.String("tenant", tenant),
		zap.String("server_name", name))

	// Parse request body
	var req BatchUpdateToolStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Error("invalid batch update tool status request body", zap.Error(err))
		i18n.RespondWithError(c, i18n.ErrBadRequest.WithParam("Reason", "Invalid request body: "+err.Error()))
		return
	}

	if len(req.Tools) == 0 {
		h.logger.Warn("empty tools list in batch update request")
		i18n.RespondWithError(c, i18n.ErrBadRequest.WithParam("Reason", "Tools list cannot be empty"))
		return
	}

	// Get MCP server configuration to check permissions
	cfg, err := h.store.Get(c.Request.Context(), tenant, name)
	if err != nil {
		h.logger.Error("MCP server not found",
			zap.String("tenant", tenant),
			zap.String("server_name", name),
			zap.Error(err))
		i18n.RespondWithError(c, i18n.ErrorMCPServerNotFound.WithParam("Name", name))
		return
	}

	// Check tenant permission
	_, err = h.checkTenantPermission(c, cfg.Tenant, cfg)
	if err != nil {
		h.logger.Warn("tenant permission check failed",
			zap.String("tenant", cfg.Tenant),
			zap.Error(err))
		i18n.RespondWithError(c, err)
		return
	}

	// Process each tool update
	results := make([]ToolStatusUpdateResult, len(req.Tools))
	successCount := 0
	errorCount := 0

	for i, tool := range req.Tools {
		result := ToolStatusUpdateResult{
			ToolName: tool.Name,
		}

		// Validate tool name
		if tool.Name == "" {
			result.Success = false
			result.Error = "Tool name cannot be empty"
			errorCount++
		} else {
			// Check if tool exists and get current status
			existingTool, err := h.capabilityStore.GetTool(c.Request.Context(), tenant, name, tool.Name)
			if err != nil {
				result.Success = false
				result.Error = fmt.Sprintf("Tool not found: %s", tool.Name)
				errorCount++
			} else {
				// Check if status change is needed
				if existingTool.Enabled == tool.Enabled {
					result.Success = true
					result.Enabled = tool.Enabled
					result.Error = fmt.Sprintf("Tool %s status already %t", tool.Name, tool.Enabled)
					successCount++
				} else {
					// Update tool status
					if err := h.capabilityStore.UpdateToolStatus(c.Request.Context(), tenant, name, tool.Name, tool.Enabled); err != nil {
						result.Success = false
						result.Error = fmt.Sprintf("Failed to update tool %s: %v", tool.Name, err)
						errorCount++
					} else {
						result.Success = true
						result.Enabled = tool.Enabled
						successCount++
					}
				}
			}
		}

		results[i] = result
	}

	// Clear cache for this server to reflect status changes
	cacheKey := tenant + ":" + name
	h.clearCapabilitiesCache(cacheKey)

	// Send reload notification to gateway if any updates were successful
	if successCount > 0 {
		if err := h.notifier.NotifyUpdate(c.Request.Context(), cfg); err != nil {
			h.logger.Warn("failed to notify gateway about tool status changes", 
				zap.String("tenant", tenant),
				zap.String("server_name", name),
				zap.Int("success_count", successCount),
				zap.Error(err))
			// Don't fail the request if notification fails, just log warning
		}
	}

	overallSuccess := errorCount == 0
	var message string
	if overallSuccess {
		message = fmt.Sprintf("All %d tools updated successfully", successCount)
	} else {
		message = fmt.Sprintf("%d tools updated successfully, %d failed", successCount, errorCount)
	}

	h.logger.Info("batch tool status update completed",
		zap.String("tenant", tenant),
		zap.String("server_name", name),
		zap.Int("total_tools", len(req.Tools)),
		zap.Int("success_count", successCount),
		zap.Int("error_count", errorCount))

	response := BatchUpdateToolStatusResponse{
		Success:      overallSuccess,
		Message:      message,
		Results:      results,
		UpdatedAt:    time.Now().Format(time.RFC3339),
		TotalTools:   len(req.Tools),
		SuccessCount: successCount,
		ErrorCount:   errorCount,
	}

	statusCode := http.StatusOK
	if !overallSuccess {
		statusCode = http.StatusPartialContent
	}

	c.JSON(statusCode, gin.H{
		"success": overallSuccess,
		"data":    response,
	})
}

// HandleGetToolStatusHistory handles GET /api/mcp/capabilities/{tenant}/{name}/tools/{toolName}/status/history
func (h *MCP) HandleGetToolStatusHistory(c *gin.Context) {
	tenant := c.Param("tenant")
	if tenant == "" {
		h.logger.Warn("tenant parameter required but missing")
		i18n.RespondWithError(c, i18n.ErrorTenantRequired)
		return
	}

	name := c.Param("name")
	if name == "" {
		h.logger.Warn("MCP server name required but missing")
		i18n.RespondWithError(c, i18n.ErrorMCPServerNameRequired)
		return
	}

	toolName := c.Param("toolName")
	if toolName == "" {
		h.logger.Warn("tool name required but missing")
		i18n.RespondWithError(c, i18n.ErrBadRequest.WithParam("Reason", "Tool name is required"))
		return
	}

	// Parse query parameters
	limitStr := c.DefaultQuery("limit", "20")
	offsetStr := c.DefaultQuery("offset", "0")

	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit < 1 || limit > 100 {
		limit = 20
	}

	offset, err := strconv.Atoi(offsetStr)
	if err != nil || offset < 0 {
		offset = 0
	}

	h.logger.Info("handling get tool status history request",
		zap.String("tenant", tenant),
		zap.String("server_name", name),
		zap.String("tool_name", toolName),
		zap.Int("limit", limit),
		zap.Int("offset", offset))

	// Get MCP server configuration to check permissions
	cfg, err := h.store.Get(c.Request.Context(), tenant, name)
	if err != nil {
		h.logger.Error("MCP server not found",
			zap.String("tenant", tenant),
			zap.String("server_name", name),
			zap.Error(err))
		i18n.RespondWithError(c, i18n.ErrorMCPServerNotFound.WithParam("Name", name))
		return
	}

	// Check tenant permission
	_, err = h.checkTenantPermission(c, cfg.Tenant, cfg)
	if err != nil {
		h.logger.Warn("tenant permission check failed",
			zap.String("tenant", cfg.Tenant),
			zap.Error(err))
		i18n.RespondWithError(c, err)
		return
	}

	// Get status history records
	records, err := h.capabilityStore.GetToolStatusHistory(c.Request.Context(), tenant, name, toolName, limit, offset)
	if err != nil {
		h.logger.Error("failed to get tool status history",
			zap.String("tenant", tenant),
			zap.String("server_name", name),
			zap.String("tool_name", toolName),
			zap.Error(err))
		i18n.RespondWithError(c, i18n.ErrInternalServer.WithParam("Reason", "Failed to get tool status history: "+err.Error()))
		return
	}

	// Convert records to API response format
	historyItems := make([]ToolStatusHistoryResponse, len(records))
	for i, record := range records {
		historyItem := ToolStatusHistoryResponse{
			ID:         record.ID,
			Tenant:     record.Tenant,
			ServerName: record.ServerName,
			ToolName:   record.ToolName,
			OldStatus:  record.OldStatus,
			NewStatus:  record.NewStatus,
			UserID:     record.UserID,
			Reason:     record.Reason,
			CreatedAt:  record.CreatedAt,
		}

		// Try to get username if possible
		if record.UserID > 0 {
			if user, err := h.db.GetUserByID(c.Request.Context(), record.UserID); err == nil {
				historyItem.Username = user.Username
			}
		}

		historyItems[i] = historyItem
	}

	h.logger.Info("tool status history retrieved successfully",
		zap.String("tenant", tenant),
		zap.String("server_name", name),
		zap.String("tool_name", toolName),
		zap.Int("record_count", len(historyItems)))

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    historyItems,
	})
}

// HandleGetCapabilitiesStats handles GET /api/mcp/capabilities/{tenant}/{name}/stats
func (h *MCP) HandleGetCapabilitiesStats(c *gin.Context) {
	tenant := c.Param("tenant")
	if tenant == "" {
		h.logger.Warn("tenant parameter required but missing")
		i18n.RespondWithError(c, i18n.ErrorTenantRequired)
		return
	}

	name := c.Param("name")
	if name == "" {
		h.logger.Warn("MCP server name required but missing")
		i18n.RespondWithError(c, i18n.ErrorMCPServerNameRequired)
		return
	}

	h.logger.Info("handling get capabilities statistics request",
		zap.String("tenant", tenant),
		zap.String("server_name", name))

	// Get MCP server configuration to check permissions
	cfg, err := h.store.Get(c.Request.Context(), tenant, name)
	if err != nil {
		h.logger.Error("MCP server not found",
			zap.String("tenant", tenant),
			zap.String("server_name", name),
			zap.Error(err))
		i18n.RespondWithError(c, i18n.ErrorMCPServerNotFound.WithParam("Name", name))
		return
	}

	// Check tenant permission
	_, err = h.checkTenantPermission(c, cfg.Tenant, cfg)
	if err != nil {
		h.logger.Warn("tenant permission check failed",
			zap.String("tenant", cfg.Tenant),
			zap.Error(err))
		i18n.RespondWithError(c, err)
		return
	}

	// Get capabilities data
	capabilities, err := h.capabilityStore.GetCapabilitiesInfo(c.Request.Context(), tenant, name)
	if err != nil {
		h.logger.Error("failed to get capabilities for statistics",
			zap.String("tenant", tenant),
			zap.String("server_name", name),
			zap.Error(err))
		i18n.RespondWithError(c, i18n.ErrInternalServer.WithParam("Reason", "Failed to get capabilities: "+err.Error()))
		return
	}

	// Calculate statistics
	stats := h.calculateCapabilitiesStats(c.Request.Context(), tenant, name, capabilities, cfg)

	h.logger.Info("capabilities statistics calculated successfully",
		zap.String("tenant", tenant),
		zap.String("server_name", name),
		zap.Int("total_capabilities", stats.Summary.TotalCapabilities))

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    stats,
	})
}

// calculateCapabilitiesStats calculates comprehensive statistics from capabilities data
func (h *MCP) calculateCapabilitiesStats(ctx context.Context, tenant, serverName string, capabilities *mcp.CapabilitiesInfo, cfg *config.MCPConfig) *CapabilitiesStatsResponse {
	stats := &CapabilitiesStatsResponse{
		LastUpdated: time.Now(),
	}

	// Server information
	stats.Server = ServerStatsInfo{
		Tenant:     tenant,
		ServerName: serverName,
		Status:     "active", // Could be derived from proxy health
		Version:    "1.0", // Default version
	}

	// Get last sync time if available - use current time as placeholder
	stats.Server.LastSyncAt = time.Now()

	// Tools statistics
	stats.Tools = h.calculateToolsStats(ctx, tenant, serverName, capabilities.Tools)

	// Prompts statistics
	stats.Prompts = h.calculatePromptsStats(capabilities.Prompts)

	// Resources statistics (including templates)
	stats.Resources = h.calculateResourcesStats(capabilities.Resources, capabilities.ResourceTemplates)

	// Overall summary
	totalCapabilities := len(capabilities.Tools) + len(capabilities.Prompts) + len(capabilities.Resources) + len(capabilities.ResourceTemplates)
	activeCapabilities := stats.Tools.Enabled + stats.Prompts.Total + stats.Resources.Total

	stats.Summary = StatsOverallInfo{
		TotalCapabilities:  totalCapabilities,
		ActiveCapabilities: activeCapabilities,
		Distribution: map[string]int{
			"tools":             len(capabilities.Tools),
			"prompts":           len(capabilities.Prompts),
			"resources":         len(capabilities.Resources),
			"resourceTemplates": len(capabilities.ResourceTemplates),
		},
	}

	return stats
}

// calculateToolsStats calculates tools-specific statistics
func (h *MCP) calculateToolsStats(ctx context.Context, tenant, serverName string, tools []mcp.MCPTool) ToolsStatsInfo {
	if len(tools) == 0 {
		return ToolsStatsInfo{}
	}

	stats := ToolsStatsInfo{
		Total:      len(tools),
		ByCategory: make(map[string]int),
	}

	enabledCount := 0
	for _, tool := range tools {
		// Get tool status from storage
		if toolStatus, err := h.capabilityStore.GetTool(ctx, tenant, serverName, tool.Name); err == nil {
			if toolStatus.Enabled {
				enabledCount++
			}
		} else {
			// Default to enabled if no status record found
			enabledCount++
		}

		// Categorize tools by schema properties (simple heuristic)
		category := h.categorizeToolBySchema(tool)
		stats.ByCategory[category]++
	}

	stats.Enabled = enabledCount
	stats.Disabled = stats.Total - enabledCount
	if stats.Total > 0 {
		stats.EnabledRate = float64(enabledCount) / float64(stats.Total) * 100
	}

	// TODO: Add usage statistics from tool call logs if available
	stats.Usage = ToolUsageStatsInfo{
		TotalCalls:  0,
		SuccessRate: 0.0,
		AvgExecTime: 0.0,
	}

	return stats
}

// calculatePromptsStats calculates prompts-specific statistics
func (h *MCP) calculatePromptsStats(prompts []mcp.MCPPrompt) PromptsStatsInfo {
	if len(prompts) == 0 {
		return PromptsStatsInfo{}
	}

	stats := PromptsStatsInfo{
		Total:      len(prompts),
		ByCategory: make(map[string]int),
	}

	withArgsCount := 0
	for _, prompt := range prompts {
		if len(prompt.Arguments) > 0 {
			withArgsCount++
		}

		// Categorize prompts by argument count
		if len(prompt.Arguments) == 0 {
			stats.ByCategory["no-args"]++
		} else if len(prompt.Arguments) <= 3 {
			stats.ByCategory["few-args"]++
		} else {
			stats.ByCategory["many-args"]++
		}
	}

	stats.WithArgs = withArgsCount
	stats.WithoutArgs = stats.Total - withArgsCount

	return stats
}

// calculateResourcesStats calculates resources-specific statistics
func (h *MCP) calculateResourcesStats(resources []mcp.MCPResource, templates []mcp.MCPResourceTemplate) ResourcesStatsInfo {
	stats := ResourcesStatsInfo{
		Total:      len(resources),
		Templates:  len(templates),
		Static:     len(resources),
		ByMimeType: make(map[string]int),
	}

	// Analyze resources by MIME type
	for _, resource := range resources {
		if resource.MIMEType != "" {
			stats.ByMimeType[resource.MIMEType]++
		} else {
			stats.ByMimeType["unknown"]++
		}
	}

	// Analyze templates by MIME type
	for _, template := range templates {
		if template.MIMEType != "" {
			stats.ByMimeType[template.MIMEType]++
		} else {
			stats.ByMimeType["unknown"]++
		}
	}

	// Update total to include templates
	stats.Total += stats.Templates

	return stats
}

// categorizeToolBySchema provides a simple heuristic to categorize tools
func (h *MCP) categorizeToolBySchema(tool mcp.MCPTool) string {
	if tool.InputSchema.Properties == nil {
		return "no-params"
	}

	paramCount := len(tool.InputSchema.Properties)
	switch {
	case paramCount == 0:
		return "no-params"
	case paramCount <= 2:
		return "simple"
	case paramCount <= 5:
		return "moderate"
	default:
		return "complex"
	}
}

// Enhanced sync status response structure
type SyncStatusResponse struct {
	SyncID        string                 `json:"syncId"`
	Status        string                 `json:"status"`
	Progress      int                    `json:"progress"`
	StartedAt     string                 `json:"startedAt"`
	CompletedAt   *string                `json:"completedAt,omitempty"`
	ErrorMessage  string                 `json:"errorMessage,omitempty"`
	Summary       *SyncSummary           `json:"summary,omitempty"`
	SyncTypes     []string               `json:"syncTypes"`
	UserID        uint                   `json:"userId"`
	RealTimeURL   string                 `json:"realTimeUrl,omitempty"`
	Metrics       *SyncMetrics           `json:"metrics,omitempty"`
}

type SyncMetrics struct {
	TotalItems    int     `json:"totalItems"`
	ProcessedItems int    `json:"processedItems"`
	Duration      string  `json:"duration,omitempty"`
	ItemsPerSecond float64 `json:"itemsPerSecond,omitempty"`
}

// HandleGetEnhancedSyncStatus handles GET /api/mcp/sync/status/{tenant}/{name}/{syncId}
// This is an enhanced version that provides more detailed information
func (h *MCP) HandleGetEnhancedSyncStatus(c *gin.Context) {
	tenant := c.Param("tenant")
	if tenant == "" {
		h.logger.Warn("tenant parameter required but missing")
		i18n.RespondWithError(c, i18n.ErrorTenantRequired)
		return
	}
	name := c.Param("name")
	if name == "" {
		h.logger.Warn("MCP server name required but missing")
		i18n.RespondWithError(c, i18n.ErrorMCPServerNameRequired)
		return
	}
	syncID := c.Param("syncId")
	if syncID == "" {
		h.logger.Warn("sync ID required but missing")
		i18n.RespondWithError(c, i18n.ErrBadRequest.WithParam("Reason", "Sync ID is required"))
		return
	}

	// Get MCP server configuration to check permissions
	cfg, err := h.store.Get(c.Request.Context(), tenant, name)
	if err != nil {
		h.logger.Error("MCP server not found",
			zap.String("tenant", tenant),
			zap.String("name", name),
			zap.Error(err))
		i18n.RespondWithError(c, i18n.ErrorMCPServerNotFound.WithParam("Name", name))
		return
	}

	// Check tenant permissions
	tenantObj, err := h.checkTenantPermission(c, tenant, cfg)
	if err != nil {
		return
	}
	if tenantObj == nil {
		return
	}

	// Get sync record
	record, err := h.capabilityStore.GetSyncRecord(c.Request.Context(), syncID)
	if err != nil {
		h.logger.Error("sync record not found",
			zap.String("sync_id", syncID),
			zap.Error(err))
		i18n.RespondWithError(c, i18n.ErrNotFound.WithParam("Resource", "sync record"))
		return
	}

	// Parse sync types
	var syncTypes []string
	if record.SyncTypes != "" {
		json.Unmarshal([]byte(record.SyncTypes), &syncTypes)
	}

	// Parse summary
	var summary *SyncSummary
	if record.Summary != "" {
		var parsedSummary SyncSummary
		if err := json.Unmarshal([]byte(record.Summary), &parsedSummary); err == nil {
			summary = &parsedSummary
		}
	}

	// Calculate metrics
	var metrics *SyncMetrics
	if !record.StartedAt.IsZero() {
		totalItems := 0
		processedItems := 0
		if summary != nil {
			totalItems = summary.getTotalChanges()
			processedItems = summary.getTotalChanges()
		}

		var duration string
		var itemsPerSecond float64
		if record.CompletedAt != nil {
			durationTime := record.CompletedAt.Sub(record.StartedAt)
			duration = durationTime.String()
			if durationTime.Seconds() > 0 && processedItems > 0 {
				itemsPerSecond = float64(processedItems) / durationTime.Seconds()
			}
		}

		metrics = &SyncMetrics{
			TotalItems:     totalItems,
			ProcessedItems: processedItems,
			Duration:       duration,
			ItemsPerSecond: itemsPerSecond,
		}
	}

	// Build real-time WebSocket URL
	scheme := "ws"
	if c.Request.TLS != nil {
		scheme = "wss"
	}
	realTimeURL := fmt.Sprintf("%s://%s/api/mcp/sync/ws/%s/%s", 
		scheme, c.Request.Host, tenant, name)

	var completedAt *string
	if record.CompletedAt != nil {
		completedAtStr := record.CompletedAt.Format(time.RFC3339)
		completedAt = &completedAtStr
	}

	response := SyncStatusResponse{
		SyncID:       record.SyncID,
		Status:       string(record.Status),
		Progress:     record.Progress,
		StartedAt:    record.StartedAt.Format(time.RFC3339),
		CompletedAt:  completedAt,
		ErrorMessage: record.ErrorMessage,
		Summary:      summary,
		SyncTypes:    syncTypes,
		UserID:       record.UserID,
		RealTimeURL:  realTimeURL,
		Metrics:      metrics,
	}

	c.JSON(http.StatusOK, gin.H{"data": response, "status": "success"})
}

// HandleGetSyncStatusOverview handles GET /api/mcp/sync/status/{tenant}/{name}
// This provides an overview of recent sync operations
func (h *MCP) HandleGetSyncStatusOverview(c *gin.Context) {
	tenant := c.Param("tenant")
	if tenant == "" {
		h.logger.Warn("tenant parameter required but missing")
		i18n.RespondWithError(c, i18n.ErrorTenantRequired)
		return
	}
	name := c.Param("name")
	if name == "" {
		h.logger.Warn("MCP server name required but missing")
		i18n.RespondWithError(c, i18n.ErrorMCPServerNameRequired)
		return
	}

	// Parse query parameters
	limitStr := c.DefaultQuery("limit", "10")
	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit < 1 || limit > 50 {
		limit = 10
	}

	// Get MCP server configuration to check permissions
	cfg, err := h.store.Get(c.Request.Context(), tenant, name)
	if err != nil {
		h.logger.Error("MCP server not found",
			zap.String("tenant", tenant),
			zap.String("name", name),
			zap.Error(err))
		i18n.RespondWithError(c, i18n.ErrorMCPServerNotFound.WithParam("Name", name))
		return
	}

	// Check tenant permissions
	tenantObj, err := h.checkTenantPermission(c, tenant, cfg)
	if err != nil {
		return
	}
	if tenantObj == nil {
		return
	}

	// Get recent sync records
	records, err := h.capabilityStore.ListSyncHistory(c.Request.Context(), tenant, name, limit, 0)
	if err != nil {
		h.logger.Error("failed to get sync history",
			zap.String("tenant", tenant),
			zap.String("name", name),
			zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		return
	}

	// Build response with enhanced information
	var responses []SyncStatusResponse
	for _, record := range records {
		var syncTypes []string
		if record.SyncTypes != "" {
			json.Unmarshal([]byte(record.SyncTypes), &syncTypes)
		}

		var summary *SyncSummary
		if record.Summary != "" {
			var parsedSummary SyncSummary
			if err := json.Unmarshal([]byte(record.Summary), &parsedSummary); err == nil {
				summary = &parsedSummary
			}
		}

		var completedAt *string
		if record.CompletedAt != nil {
			completedAtStr := record.CompletedAt.Format(time.RFC3339)
			completedAt = &completedAtStr
		}

		responses = append(responses, SyncStatusResponse{
			SyncID:       record.SyncID,
			Status:       string(record.Status),
			Progress:     record.Progress,
			StartedAt:    record.StartedAt.Format(time.RFC3339),
			CompletedAt:  completedAt,
			ErrorMessage: record.ErrorMessage,
			Summary:      summary,
			SyncTypes:    syncTypes,
			UserID:       record.UserID,
		})
	}

	// Build real-time WebSocket URL
	scheme := "ws"
	if c.Request.TLS != nil {
		scheme = "wss"
	}
	realTimeURL := fmt.Sprintf("%s://%s/api/mcp/sync/ws/%s/%s", 
		scheme, c.Request.Host, tenant, name)

	data := gin.H{
		"syncHistory": responses,
		"realTimeUrl": realTimeURL,
		"wsConnections": h.wsManager.GetConnectionCount(),
	}

	c.JSON(http.StatusOK, gin.H{"data": data, "status": "success"})
}
