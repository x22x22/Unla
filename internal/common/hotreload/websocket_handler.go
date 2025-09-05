package hotreload

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"go.uber.org/zap"
)

// WebSocketEventHandler handles reload events by broadcasting them to WebSocket clients
type WebSocketEventHandler struct {
	logger      *zap.Logger
	broadcaster WebSocketBroadcaster
	mu          sync.RWMutex
	clients     map[string]WebSocketClient
}

// WebSocketBroadcaster defines the interface for broadcasting messages to WebSocket clients
type WebSocketBroadcaster interface {
	BroadcastToAll(message []byte) error
	BroadcastToClient(clientID string, message []byte) error
	BroadcastToTenant(tenantID string, message []byte) error
}

// WebSocketClient represents a connected WebSocket client
type WebSocketClient struct {
	ID       string `json:"id"`
	TenantID string `json:"tenant_id,omitempty"`
	UserID   string `json:"user_id,omitempty"`
}

// WebSocketMessage represents a message sent to WebSocket clients
type WebSocketMessage struct {
	Type      string      `json:"type"`
	Event     string      `json:"event"`
	Data      interface{} `json:"data"`
	Timestamp string      `json:"timestamp"`
	ClientID  string      `json:"client_id,omitempty"`
	TenantID  string      `json:"tenant_id,omitempty"`
}

// NewWebSocketEventHandler creates a new WebSocket event handler
func NewWebSocketEventHandler(logger *zap.Logger, broadcaster WebSocketBroadcaster) *WebSocketEventHandler {
	return &WebSocketEventHandler{
		logger:      logger.Named("websocket_handler"),
		broadcaster: broadcaster,
		clients:     make(map[string]WebSocketClient),
	}
}

// HandleReloadEvent handles a reload event by broadcasting it to WebSocket clients
func (h *WebSocketEventHandler) HandleReloadEvent(ctx context.Context, event ReloadEvent) error {
	h.logger.Debug("Broadcasting reload event to WebSocket clients",
		zap.String("event_id", event.ID),
		zap.String("type", string(event.Type)),
		zap.String("status", string(event.Status)))

	// Create WebSocket message
	message := WebSocketMessage{
		Type:      "hotreload",
		Event:     "config_reload",
		Data:      event,
		Timestamp: event.Timestamp.Format("2006-01-02T15:04:05.000Z"),
	}

	// Serialize message
	messageBytes, err := json.Marshal(message)
	if err != nil {
		h.logger.Error("Failed to marshal WebSocket message",
			zap.String("event_id", event.ID),
			zap.Error(err))
		return err
	}

	// Broadcast to all clients
	if err := h.broadcaster.BroadcastToAll(messageBytes); err != nil {
		h.logger.Error("Failed to broadcast reload event",
			zap.String("event_id", event.ID),
			zap.Error(err))
		return err
	}

	h.logger.Debug("Successfully broadcasted reload event",
		zap.String("event_id", event.ID))

	return nil
}

// RegisterClient registers a WebSocket client
func (h *WebSocketEventHandler) RegisterClient(client WebSocketClient) {
	h.mu.Lock()
	defer h.mu.Unlock()
	
	h.clients[client.ID] = client
	h.logger.Debug("Registered WebSocket client",
		zap.String("client_id", client.ID),
		zap.String("tenant_id", client.TenantID))
}

// UnregisterClient unregisters a WebSocket client
func (h *WebSocketEventHandler) UnregisterClient(clientID string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	
	delete(h.clients, clientID)
	h.logger.Debug("Unregistered WebSocket client",
		zap.String("client_id", clientID))
}

// GetConnectedClients returns the list of connected clients
func (h *WebSocketEventHandler) GetConnectedClients() []WebSocketClient {
	h.mu.RLock()
	defer h.mu.RUnlock()
	
	clients := make([]WebSocketClient, 0, len(h.clients))
	for _, client := range h.clients {
		clients = append(clients, client)
	}
	
	return clients
}

// BroadcastCapabilityUpdate broadcasts a capability update event
func (h *WebSocketEventHandler) BroadcastCapabilityUpdate(serverName string, capabilities interface{}) error {
	message := WebSocketMessage{
		Type:  "hotreload",
		Event: "capability_update",
		Data: map[string]interface{}{
			"server_name":  serverName,
			"capabilities": capabilities,
		},
		Timestamp: ReloadEvent{}.Timestamp.Format("2006-01-02T15:04:05.000Z"),
	}

	messageBytes, err := json.Marshal(message)
	if err != nil {
		h.logger.Error("Failed to marshal capability update message",
			zap.String("server_name", serverName),
			zap.Error(err))
		return err
	}

	return h.broadcaster.BroadcastToAll(messageBytes)
}

// BroadcastServerStatus broadcasts a server status update
func (h *WebSocketEventHandler) BroadcastServerStatus(serverName string, status string, details interface{}) error {
	message := WebSocketMessage{
		Type:  "hotreload",
		Event: "server_status",
		Data: map[string]interface{}{
			"server_name": serverName,
			"status":      status,
			"details":     details,
		},
		Timestamp: ReloadEvent{}.Timestamp.Format("2006-01-02T15:04:05.000Z"),
	}

	messageBytes, err := json.Marshal(message)
	if err != nil {
		h.logger.Error("Failed to marshal server status message",
			zap.String("server_name", serverName),
			zap.Error(err))
		return err
	}

	return h.broadcaster.BroadcastToAll(messageBytes)
}

// CacheInvalidationHandler handles cache invalidation events
type CacheInvalidationHandler struct {
	logger      *zap.Logger
	cacheKeys   []string
	broadcaster WebSocketBroadcaster
}

// NewCacheInvalidationHandler creates a new cache invalidation handler
func NewCacheInvalidationHandler(logger *zap.Logger, broadcaster WebSocketBroadcaster) *CacheInvalidationHandler {
	return &CacheInvalidationHandler{
		logger:      logger.Named("cache_invalidation"),
		broadcaster: broadcaster,
		cacheKeys:   make([]string, 0),
	}
}

// HandleReloadEvent handles reload events by invalidating relevant caches
func (h *CacheInvalidationHandler) HandleReloadEvent(ctx context.Context, event ReloadEvent) error {
	h.logger.Info("Processing cache invalidation for reload event",
		zap.String("event_id", event.ID),
		zap.String("type", string(event.Type)))

	// Determine which caches need to be invalidated based on the event
	cacheKeysToInvalidate := h.determineCacheKeys(event)
	
	if len(cacheKeysToInvalidate) == 0 {
		h.logger.Debug("No cache keys to invalidate for this event")
		return nil
	}

	// Broadcast cache invalidation message to clients
	message := WebSocketMessage{
		Type:  "cache",
		Event: "invalidate",
		Data: map[string]interface{}{
			"keys":     cacheKeysToInvalidate,
			"event_id": event.ID,
			"reason":   "config_reload",
		},
		Timestamp: event.Timestamp.Format("2006-01-02T15:04:05.000Z"),
	}

	messageBytes, err := json.Marshal(message)
	if err != nil {
		h.logger.Error("Failed to marshal cache invalidation message",
			zap.String("event_id", event.ID),
			zap.Error(err))
		return err
	}

	if err := h.broadcaster.BroadcastToAll(messageBytes); err != nil {
		h.logger.Error("Failed to broadcast cache invalidation",
			zap.String("event_id", event.ID),
			zap.Error(err))
		return err
	}

	h.logger.Info("Successfully broadcasted cache invalidation",
		zap.String("event_id", event.ID),
		zap.Strings("cache_keys", cacheKeysToInvalidate))

	return nil
}

// determineCacheKeys determines which cache keys should be invalidated based on the reload event
func (h *CacheInvalidationHandler) determineCacheKeys(event ReloadEvent) []string {
	var keys []string

	switch event.Type {
	case ReloadEventTypeConfigChange:
		// Invalidate configuration-related caches
		keys = append(keys, 
			"mcp_servers",
			"server_capabilities", 
			"gateway_config",
		)
		
	case ReloadEventTypeCapabilitySync:
		// Invalidate capability-related caches
		keys = append(keys,
			"server_capabilities",
			"tool_definitions",
			"resource_templates",
		)
		
	case ReloadEventTypeManualReload, ReloadEventTypeScheduledReload:
		// Invalidate all relevant caches
		keys = append(keys,
			"mcp_servers",
			"server_capabilities",
			"gateway_config",
			"tool_definitions",
			"resource_templates",
		)
	}

	// Add tenant-specific cache keys if applicable
	if event.Details != nil {
		if tenantID, ok := event.Details["tenant_id"].(string); ok && tenantID != "" {
			tenantKeys := make([]string, len(keys))
			for i, key := range keys {
				tenantKeys[i] = fmt.Sprintf("tenant:%s:%s", tenantID, key)
			}
			keys = append(keys, tenantKeys...)
		}
	}

	return keys
}

// VersionManagerHandler manages configuration versions during reloads
type VersionManagerHandler struct {
	logger   *zap.Logger
	versions map[string]ConfigVersion
	mu       sync.RWMutex
}

// ConfigVersion represents a configuration version
type ConfigVersion struct {
	Version   string                 `json:"version"`
	Timestamp string                 `json:"timestamp"`
	Changes   []ConfigChange         `json:"changes"`
	Status    string                 `json:"status"`
	Details   map[string]interface{} `json:"details"`
}

// NewVersionManagerHandler creates a new version manager handler
func NewVersionManagerHandler(logger *zap.Logger) *VersionManagerHandler {
	return &VersionManagerHandler{
		logger:   logger.Named("version_manager"),
		versions: make(map[string]ConfigVersion),
	}
}

// HandleReloadEvent handles reload events by managing configuration versions
func (h *VersionManagerHandler) HandleReloadEvent(ctx context.Context, event ReloadEvent) error {
	h.logger.Debug("Managing configuration version for reload event",
		zap.String("event_id", event.ID),
		zap.String("status", string(event.Status)))

	h.mu.Lock()
	defer h.mu.Unlock()

	version := ConfigVersion{
		Version:   event.ID,
		Timestamp: event.Timestamp.Format("2006-01-02T15:04:05.000Z"),
		Changes:   event.Changes,
		Status:    string(event.Status),
		Details: map[string]interface{}{
			"type":     string(event.Type),
			"source":   event.Source,
			"duration": event.Duration.String(),
		},
	}

	if event.Error != "" {
		version.Details["error"] = event.Error
	}

	h.versions[event.ID] = version

	// Keep only last 50 versions
	if len(h.versions) > 50 {
		// This is a simplified cleanup - in practice, you'd want to remove oldest versions
		h.logger.Debug("Version history limit reached, cleanup needed")
	}

	h.logger.Debug("Updated configuration version",
		zap.String("version", event.ID),
		zap.String("status", string(event.Status)))

	return nil
}

// GetVersionHistory returns the configuration version history
func (h *VersionManagerHandler) GetVersionHistory() map[string]ConfigVersion {
	h.mu.RLock()
	defer h.mu.RUnlock()

	versions := make(map[string]ConfigVersion)
	for k, v := range h.versions {
		versions[k] = v
	}

	return versions
}

// GetCurrentVersion returns the current/latest configuration version
func (h *VersionManagerHandler) GetCurrentVersion() *ConfigVersion {
	h.mu.RLock()
	defer h.mu.RUnlock()

	var latest *ConfigVersion
	var latestTime string

	for _, version := range h.versions {
		if latest == nil || version.Timestamp > latestTime {
			latest = &version
			latestTime = version.Timestamp
		}
	}

	return latest
}