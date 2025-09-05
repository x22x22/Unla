package mcp

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/amoylab/unla/internal/common/config"
	"github.com/amoylab/unla/internal/common/errorx"
	"github.com/amoylab/unla/internal/common/hotreload"
	"github.com/amoylab/unla/internal/common/monitor"
	"github.com/amoylab/unla/internal/mcp/storage"
	"github.com/amoylab/unla/pkg/mcp"
	"go.uber.org/zap"
)

// CapabilitySyncManager manages automatic MCP capability synchronization
type CapabilitySyncManager struct {
	logger          *zap.Logger
	capabilityStore storage.CapabilityStore
	monitor         *monitor.SystemMonitor
	reloadManager   *hotreload.ReloadManager
	serverFactory   ServerFactory
	mu              sync.RWMutex
	servers         map[string]MCPServerInfo
	syncInterval    time.Duration
	ctx             context.Context
	cancel          context.CancelFunc
	lastSync        time.Time
	syncHistory     []SyncEvent
	maxHistorySize  int
}

// ServerFactory creates MCP server connections
type ServerFactory interface {
	CreateServer(config ServerConfig) (MCPServer, error)
}

// MCPServer represents an MCP server connection
type MCPServer interface {
	GetCapabilities(ctx context.Context) (*mcp.CapabilitiesInfo, error)
	Close() error
	IsHealthy() bool
}

// MCPServerInfo holds information about an MCP server
type MCPServerInfo struct {
	Config       ServerConfig           `json:"config"`
	Server       MCPServer              `json:"-"`
	Capabilities *mcp.CapabilitiesInfo  `json:"capabilities,omitempty"`
	Status       ServerStatus           `json:"status"`
	LastSync     time.Time              `json:"last_sync"`
	Error        string                 `json:"error,omitempty"`
}

// ServerConfig represents MCP server configuration
type ServerConfig struct {
	Name        string            `json:"name"`
	Type        string            `json:"type"`
	Command     []string          `json:"command,omitempty"`
	Args        []string          `json:"args,omitempty"`
	Env         map[string]string `json:"env,omitempty"`
	WorkingDir  string            `json:"working_dir,omitempty"`
	Timeout     time.Duration     `json:"timeout"`
	Enabled     bool              `json:"enabled"`
	TenantName  string            `json:"tenant_name,omitempty"`
}

// ServerStatus represents the status of an MCP server
type ServerStatus string

const (
	ServerStatusUnknown     ServerStatus = "unknown"
	ServerStatusStarting    ServerStatus = "starting"
	ServerStatusReady       ServerStatus = "ready"
	ServerStatusError       ServerStatus = "error"
	ServerStatusDisconnected ServerStatus = "disconnected"
)

// SyncEvent represents a capability synchronization event
type SyncEvent struct {
	ID          string                 `json:"id"`
	Type        SyncEventType          `json:"type"`
	Status      SyncEventStatus        `json:"status"`
	Timestamp   time.Time              `json:"timestamp"`
	ServerName  string                 `json:"server_name,omitempty"`
	Duration    time.Duration          `json:"duration,omitempty"`
	Error       string                 `json:"error,omitempty"`
	Changes     []CapabilityChange     `json:"changes,omitempty"`
	Details     map[string]interface{} `json:"details,omitempty"`
}

// SyncEventType represents the type of sync event
type SyncEventType string

const (
	SyncEventTypeConfigReload   SyncEventType = "config_reload"
	SyncEventTypeScheduledSync  SyncEventType = "scheduled_sync"
	SyncEventTypeManualSync     SyncEventType = "manual_sync"
	SyncEventTypeServerAdded    SyncEventType = "server_added"
	SyncEventTypeServerRemoved  SyncEventType = "server_removed"
	SyncEventTypeServerUpdated  SyncEventType = "server_updated"
)

// SyncEventStatus represents the status of a sync event
type SyncEventStatus string

const (
	SyncEventStatusPending    SyncEventStatus = "pending"
	SyncEventStatusProcessing SyncEventStatus = "processing"
	SyncEventStatusCompleted  SyncEventStatus = "completed"
	SyncEventStatusFailed     SyncEventStatus = "failed"
)

// CapabilityChange represents a change in server capabilities
type CapabilityChange struct {
	Type        string      `json:"type"` // "added", "updated", "removed"
	Category    string      `json:"category"` // "tool", "resource", "prompt"
	Name        string      `json:"name"`
	OldValue    interface{} `json:"old_value,omitempty"`
	NewValue    interface{} `json:"new_value,omitempty"`
}

// NewCapabilitySyncManager creates a new capability sync manager
func NewCapabilitySyncManager(
	logger *zap.Logger,
	capabilityStore storage.CapabilityStore,
	monitor *monitor.SystemMonitor,
	reloadManager *hotreload.ReloadManager,
	serverFactory ServerFactory,
) *CapabilitySyncManager {
	ctx, cancel := context.WithCancel(context.Background())

	manager := &CapabilitySyncManager{
		logger:          logger.Named("capability_sync"),
		capabilityStore: capabilityStore,
		monitor:         monitor,
		reloadManager:   reloadManager,
		serverFactory:   serverFactory,
		servers:         make(map[string]MCPServerInfo),
		syncInterval:    5 * time.Minute,
		ctx:             ctx,
		cancel:          cancel,
		syncHistory:     make([]SyncEvent, 0),
		maxHistorySize:  100,
	}

	// Register as reload event handler
	if reloadManager != nil {
		reloadManager.AddEventHandler(manager)
	}

	// Start background sync process
	go manager.runBackgroundSync()

	return manager
}

// HandleReloadEvent handles configuration reload events
func (m *CapabilitySyncManager) HandleReloadEvent(ctx context.Context, event hotreload.ReloadEvent) error {
	m.logger.Info("Handling reload event for capability sync",
		zap.String("event_id", event.ID),
		zap.String("type", string(event.Type)))

	// Trigger capability synchronization based on reload event type
	switch event.Type {
	case hotreload.ReloadEventTypeConfigChange, hotreload.ReloadEventTypeManualReload:
		return m.SyncAllServers(ctx, SyncEventTypeConfigReload)
	case hotreload.ReloadEventTypeCapabilitySync:
		return m.SyncAllServers(ctx, SyncEventTypeScheduledSync)
	}

	return nil
}

// LoadServerConfigs loads server configurations from the provided config
func (m *CapabilitySyncManager) LoadServerConfigs(mcpConfig *config.MCPConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.logger.Info("Loading MCP server configurations", zap.Int("server_count", len(mcpConfig.Servers)))

	// Track changes for reload event
	var changes []CapabilityChange

	// Process new/updated servers
	newServers := make(map[string]MCPServerInfo)
	for _, serverConfig := range mcpConfig.McpServers {
		var command []string
		if serverConfig.Command != "" {
			command = []string{serverConfig.Command}
		}
		
		config := ServerConfig{
			Name:       serverConfig.Name,
			Type:       serverConfig.Type,
			Command:    command,
			Args:       serverConfig.Args,
			Env:        serverConfig.Env,
			WorkingDir: "", // Not available in MCPServerConfig
			Timeout:    30 * time.Second, // Default timeout
			Enabled:    true, // MCPServerConfig doesn't have Enabled field
			TenantName: "default", // Default tenant name
		}

		// Check if this is a new or updated server
		if existingServer, exists := m.servers[serverConfig.Name]; exists {
			// Server exists, check for changes
			if !m.configsEqual(existingServer.Config, config) {
				changes = append(changes, CapabilityChange{
					Type:     "updated",
					Category: "server",
					Name:     serverConfig.Name,
					OldValue: existingServer.Config,
					NewValue: config,
				})
				
				// Close existing server connection
				if existingServer.Server != nil {
					existingServer.Server.Close()
				}
			} else {
				// No changes, keep existing server
				newServers[serverConfig.Name] = existingServer
				continue
			}
		} else {
			// New server
			changes = append(changes, CapabilityChange{
				Type:     "added",
				Category: "server",
				Name:     serverConfig.Name,
				NewValue: config,
			})
		}

		// Create new server info
		newServers[serverConfig.Name] = MCPServerInfo{
			Config:   config,
			Status:   ServerStatusUnknown,
			LastSync: time.Time{},
		}
	}

	// Check for removed servers
	for serverName := range m.servers {
		if _, exists := newServers[serverName]; !exists {
			changes = append(changes, CapabilityChange{
				Type:     "removed",
				Category: "server",
				Name:     serverName,
				OldValue: m.servers[serverName].Config,
			})
			
			// Close removed server
			if server := m.servers[serverName].Server; server != nil {
				server.Close()
			}
		}
	}

	m.servers = newServers

	// Log configuration changes
	if len(changes) > 0 {
		m.logger.Info("Server configuration changes detected",
			zap.Int("changes", len(changes)))
		
		// Record sync event
		event := SyncEvent{
			ID:        fmt.Sprintf("config_%d", time.Now().UnixNano()),
			Type:      SyncEventTypeConfigReload,
			Status:    SyncEventStatusCompleted,
			Timestamp: time.Now(),
			Changes:   changes,
			Details: map[string]interface{}{
				"total_servers": len(newServers),
				"changes_count": len(changes),
			},
		}
		m.addSyncEvent(event)
	}

	return nil
}

// SyncAllServers synchronizes capabilities for all servers
func (m *CapabilitySyncManager) SyncAllServers(ctx context.Context, syncType SyncEventType) error {
	m.mu.RLock()
	serverNames := make([]string, 0, len(m.servers))
	for name := range m.servers {
		serverNames = append(serverNames, name)
	}
	m.mu.RUnlock()

	if len(serverNames) == 0 {
		m.logger.Info("No servers configured, skipping capability sync")
		return nil
	}

	m.logger.Info("Starting capability sync for all servers",
		zap.Int("server_count", len(serverNames)),
		zap.String("sync_type", string(syncType)))

	startTime := time.Now()
	event := SyncEvent{
		ID:        fmt.Sprintf("sync_all_%d", time.Now().UnixNano()),
		Type:      syncType,
		Status:    SyncEventStatusProcessing,
		Timestamp: startTime,
		Details: map[string]interface{}{
			"server_count": len(serverNames),
		},
	}
	m.addSyncEvent(event)

	var wg sync.WaitGroup
	var syncErrors []string
	var errorsMu sync.Mutex

	// Sync all servers in parallel
	for _, serverName := range serverNames {
		wg.Add(1)
		go func(name string) {
			defer wg.Done()
			if err := m.SyncServer(ctx, name); err != nil {
				errorsMu.Lock()
				syncErrors = append(syncErrors, fmt.Sprintf("%s: %v", name, err))
				errorsMu.Unlock()
			}
		}(serverName)
	}

	wg.Wait()

	// Update sync event with results
	event.Duration = time.Since(startTime)
	if len(syncErrors) > 0 {
		event.Status = SyncEventStatusFailed
		event.Error = fmt.Sprintf("Failed to sync %d servers: %v", len(syncErrors), syncErrors)
	} else {
		event.Status = SyncEventStatusCompleted
	}

	m.addSyncEvent(event)
	m.lastSync = time.Now()

	if len(syncErrors) > 0 {
		return fmt.Errorf("capability sync completed with errors: %v", syncErrors)
	}

	m.logger.Info("Capability sync completed successfully",
		zap.Duration("duration", event.Duration),
		zap.Int("server_count", len(serverNames)))

	return nil
}

// SyncServer synchronizes capabilities for a specific server
func (m *CapabilitySyncManager) SyncServer(ctx context.Context, serverName string) error {
	m.mu.Lock()
	serverInfo, exists := m.servers[serverName]
	if !exists {
		m.mu.Unlock()
		return fmt.Errorf("server %s not found", serverName)
	}

	if !serverInfo.Config.Enabled {
		m.mu.Unlock()
		m.logger.Debug("Skipping disabled server", zap.String("server", serverName))
		return nil
	}
	m.mu.Unlock()

	m.logger.Debug("Syncing capabilities for server", zap.String("server", serverName))

	startTime := time.Now()
	
	// Create or get server connection
	server, err := m.getOrCreateServer(serverInfo)
	if err != nil {
		m.updateServerStatus(serverName, ServerStatusError, err.Error())
		return fmt.Errorf("failed to create server connection: %w", err)
	}

	// Update server status
	m.updateServerStatus(serverName, ServerStatusStarting, "")

	// Get capabilities with timeout
	syncCtx, cancel := context.WithTimeout(ctx, serverInfo.Config.Timeout)
	defer cancel()

	capabilities, err := server.GetCapabilities(syncCtx)
	if err != nil {
		m.updateServerStatus(serverName, ServerStatusError, err.Error())
		return fmt.Errorf("failed to get capabilities: %w", err)
	}

	// Update server info
	m.mu.Lock()
	serverInfo.Capabilities = capabilities
	serverInfo.Status = ServerStatusReady
	serverInfo.LastSync = time.Now()
	serverInfo.Error = ""
	serverInfo.Server = server
	m.servers[serverName] = serverInfo
	m.mu.Unlock()

	// Store capabilities
	if m.capabilityStore != nil {
		if err := m.capabilityStore.SyncCapabilities(ctx, capabilities, serverInfo.Config.TenantName, serverName); err != nil {
			m.logger.Error("Failed to store capabilities",
				zap.String("server", serverName),
				zap.Error(err))
			
			// Don't fail the sync for storage errors
		}
	}

	duration := time.Since(startTime)
	m.logger.Debug("Successfully synced server capabilities",
		zap.String("server", serverName),
		zap.Duration("duration", duration))

	// Record successful sync event
	event := SyncEvent{
		ID:         fmt.Sprintf("sync_%s_%d", serverName, time.Now().UnixNano()),
		Type:       SyncEventTypeManualSync,
		Status:     SyncEventStatusCompleted,
		Timestamp:  startTime,
		ServerName: serverName,
		Duration:   duration,
		Details: map[string]interface{}{
			"tools_count":     len(capabilities.Tools),
			"resources_count": len(capabilities.Resources),
			"prompts_count":   len(capabilities.Prompts),
		},
	}
	m.addSyncEvent(event)

	return nil
}

// getOrCreateServer gets an existing server connection or creates a new one
func (m *CapabilitySyncManager) getOrCreateServer(serverInfo MCPServerInfo) (MCPServer, error) {
	// Check if existing server is healthy
	if serverInfo.Server != nil && serverInfo.Server.IsHealthy() {
		return serverInfo.Server, nil
	}

	// Create new server connection
	if m.serverFactory == nil {
		return nil, fmt.Errorf("server factory not available")
	}

	server, err := m.serverFactory.CreateServer(serverInfo.Config)
	if err != nil {
		return nil, err
	}

	return server, nil
}

// updateServerStatus updates the status of a server
func (m *CapabilitySyncManager) updateServerStatus(serverName string, status ServerStatus, errorMsg string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if serverInfo, exists := m.servers[serverName]; exists {
		serverInfo.Status = status
		serverInfo.Error = errorMsg
		m.servers[serverName] = serverInfo
	}
}

// configsEqual compares two server configurations
func (m *CapabilitySyncManager) configsEqual(a, b ServerConfig) bool {
	return a.Name == b.Name &&
		a.Type == b.Type &&
		len(a.Command) == len(b.Command) &&
		len(a.Args) == len(b.Args) &&
		a.WorkingDir == b.WorkingDir &&
		a.Timeout == b.Timeout &&
		a.Enabled == b.Enabled &&
		a.TenantName == b.TenantName
}

// addSyncEvent adds an event to the sync history
func (m *CapabilitySyncManager) addSyncEvent(event SyncEvent) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.syncHistory = append(m.syncHistory, event)
	
	// Trim history if it gets too large
	if len(m.syncHistory) > m.maxHistorySize {
		m.syncHistory = m.syncHistory[1:]
	}
}

// runBackgroundSync runs periodic capability synchronization
func (m *CapabilitySyncManager) runBackgroundSync() {
	ticker := time.NewTicker(m.syncInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if err := m.SyncAllServers(m.ctx, SyncEventTypeScheduledSync); err != nil {
				m.logger.Error("Background capability sync failed", zap.Error(err))
				
				// Record error in monitoring
				if m.monitor != nil {
					apiErr := &errorx.APIError{
						Code:     "E6003",
						Message:  "Background capability sync failed",
						Category: errorx.CategoryInternal,
						Severity: errorx.SeverityError,
						Details: map[string]interface{}{
							"error": err.Error(),
						},
					}
					m.monitor.RecordError(apiErr, map[string]interface{}{
						"component": "capability_sync",
						"operation": "background_sync",
					})
				}
			}

		case <-m.ctx.Done():
			m.logger.Info("Background capability sync stopped")
			return
		}
	}
}

// GetServerInfo returns information about all servers
func (m *CapabilitySyncManager) GetServerInfo() map[string]MCPServerInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()

	info := make(map[string]MCPServerInfo)
	for name, serverInfo := range m.servers {
		// Don't expose the actual server connection
		info[name] = MCPServerInfo{
			Config:       serverInfo.Config,
			Capabilities: serverInfo.Capabilities,
			Status:       serverInfo.Status,
			LastSync:     serverInfo.LastSync,
			Error:        serverInfo.Error,
		}
	}

	return info
}

// GetSyncHistory returns the synchronization history
func (m *CapabilitySyncManager) GetSyncHistory() []SyncEvent {
	m.mu.RLock()
	defer m.mu.RUnlock()

	history := make([]SyncEvent, len(m.syncHistory))
	copy(history, m.syncHistory)
	return history
}

// Close stops the capability sync manager and releases resources
func (m *CapabilitySyncManager) Close() error {
	m.logger.Info("Stopping capability sync manager")
	m.cancel()

	// Close all server connections
	m.mu.Lock()
	defer m.mu.Unlock()
	
	for _, serverInfo := range m.servers {
		if serverInfo.Server != nil {
			serverInfo.Server.Close()
		}
	}

	return nil
}