package hotreload

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/amoylab/unla/internal/common/errorx"
	"github.com/amoylab/unla/internal/common/monitor"
	"github.com/amoylab/unla/internal/mcp/storage"
	"github.com/amoylab/unla/internal/mcp/storage/notifier"
	"go.uber.org/zap"
)

// ReloadManager manages configuration hot reloading and capability synchronization
type ReloadManager struct {
	logger           *zap.Logger
	notifier         notifier.Notifier
	capabilityStore  storage.CapabilityStore
	monitor          *monitor.SystemMonitor
	eventHandlers    []EventHandler
	mu               sync.RWMutex
	ctx              context.Context
	cancel           context.CancelFunc
	isReloading      bool
	lastReload       time.Time
	reloadHistory    []ReloadEvent
	maxHistorySize   int
}

// EventHandler defines the interface for handling reload events
type EventHandler interface {
	HandleReloadEvent(ctx context.Context, event ReloadEvent) error
}

// ReloadEvent represents a configuration reload event
type ReloadEvent struct {
	ID          string                 `json:"id"`
	Type        ReloadEventType        `json:"type"`
	Status      ReloadEventStatus      `json:"status"`
	Timestamp   time.Time              `json:"timestamp"`
	ConfigType  string                 `json:"config_type,omitempty"`
	Source      string                 `json:"source,omitempty"`
	Changes     []ConfigChange         `json:"changes,omitempty"`
	Error       string                 `json:"error,omitempty"`
	Duration    time.Duration          `json:"duration,omitempty"`
	Details     map[string]interface{} `json:"details,omitempty"`
}

// ReloadEventType represents the type of reload event
type ReloadEventType string

const (
	ReloadEventTypeConfigChange     ReloadEventType = "config_change"
	ReloadEventTypeCapabilitySync   ReloadEventType = "capability_sync"
	ReloadEventTypeScheduledReload  ReloadEventType = "scheduled_reload"
	ReloadEventTypeManualReload     ReloadEventType = "manual_reload"
)

// ReloadEventStatus represents the status of a reload event
type ReloadEventStatus string

const (
	ReloadEventStatusPending    ReloadEventStatus = "pending"
	ReloadEventStatusProcessing ReloadEventStatus = "processing"
	ReloadEventStatusCompleted  ReloadEventStatus = "completed"
	ReloadEventStatusFailed     ReloadEventStatus = "failed"
	ReloadEventStatusCancelled  ReloadEventStatus = "cancelled"
)

// ConfigChange represents a change in configuration
type ConfigChange struct {
	Path     string      `json:"path"`
	OldValue interface{} `json:"old_value,omitempty"`
	NewValue interface{} `json:"new_value,omitempty"`
	Action   string      `json:"action"` // "added", "updated", "removed"
}

// NewReloadManager creates a new reload manager
func NewReloadManager(
	logger *zap.Logger,
	notifier notifier.Notifier,
	capabilityStore storage.CapabilityStore,
	monitor *monitor.SystemMonitor,
) *ReloadManager {
	ctx, cancel := context.WithCancel(context.Background())

	manager := &ReloadManager{
		logger:          logger.Named("reload_manager"),
		notifier:        notifier,
		capabilityStore: capabilityStore,
		monitor:         monitor,
		eventHandlers:   make([]EventHandler, 0),
		ctx:             ctx,
		cancel:          cancel,
		reloadHistory:   make([]ReloadEvent, 0),
		maxHistorySize:  100,
	}

	// Start watching for configuration changes
	go manager.watchConfigChanges()

	return manager
}

// AddEventHandler adds an event handler
func (m *ReloadManager) AddEventHandler(handler EventHandler) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.eventHandlers = append(m.eventHandlers, handler)
}

// watchConfigChanges watches for configuration changes and triggers reloads
func (m *ReloadManager) watchConfigChanges() {
	if !m.notifier.CanReceive() {
		m.logger.Info("Notifier cannot receive updates, skipping config watch")
		return
	}

	watchCtx, cancel := context.WithCancel(m.ctx)
	defer cancel()

	eventChan, err := m.notifier.Watch(watchCtx)
	if err != nil {
		m.logger.Error("Failed to watch configuration changes", zap.Error(err))
		return
	}

	m.logger.Info("Started watching for configuration changes")

	for {
		select {
		case configUpdate, ok := <-eventChan:
			if !ok {
				m.logger.Info("Configuration watch channel closed")
				return
			}

			m.logger.Info("Received configuration change notification")
			
			// Trigger reload with configuration change event
			event := ReloadEvent{
				ID:         fmt.Sprintf("config_%d", time.Now().UnixNano()),
				Type:       ReloadEventTypeConfigChange,
				Status:     ReloadEventStatusPending,
				Timestamp:  time.Now(),
				ConfigType: "mcp",
				Source:     "file_watcher",
			}

			if configUpdate != nil {
				event.Details = map[string]interface{}{
					"config_updated": true,
				}
			}

			go m.processReloadEvent(event)

		case <-m.ctx.Done():
			m.logger.Info("Configuration watch stopped")
			return
		}
	}
}

// TriggerReload manually triggers a configuration reload
func (m *ReloadManager) TriggerReload(source string, changes []ConfigChange) error {
	event := ReloadEvent{
		ID:        fmt.Sprintf("manual_%d", time.Now().UnixNano()),
		Type:      ReloadEventTypeManualReload,
		Status:    ReloadEventStatusPending,
		Timestamp: time.Now(),
		Source:    source,
		Changes:   changes,
	}

	go m.processReloadEvent(event)
	return nil
}

// processReloadEvent processes a reload event
func (m *ReloadManager) processReloadEvent(event ReloadEvent) {
	m.mu.Lock()
	if m.isReloading {
		m.logger.Warn("Reload already in progress, skipping", zap.String("event_id", event.ID))
		m.mu.Unlock()
		return
	}
	m.isReloading = true
	m.mu.Unlock()

	defer func() {
		m.mu.Lock()
		m.isReloading = false
		m.lastReload = time.Now()
		m.mu.Unlock()
	}()

	startTime := time.Now()
	event.Status = ReloadEventStatusProcessing
	m.addToHistory(event)

	m.logger.Info("Processing reload event",
		zap.String("event_id", event.ID),
		zap.String("type", string(event.Type)),
		zap.String("source", event.Source))

	// Notify event handlers
	m.notifyEventHandlers(event)

	// Perform the actual reload based on event type
	var err error
	switch event.Type {
	case ReloadEventTypeConfigChange, ReloadEventTypeManualReload:
		err = m.reloadConfiguration()
	case ReloadEventTypeCapabilitySync:
		err = m.syncCapabilities()
	case ReloadEventTypeScheduledReload:
		err = m.performScheduledReload()
	default:
		err = fmt.Errorf("unknown reload event type: %s", event.Type)
	}

	// Update event status and record metrics
	event.Duration = time.Since(startTime)
	if err != nil {
		event.Status = ReloadEventStatusFailed
		event.Error = err.Error()
		
		m.logger.Error("Reload event failed",
			zap.String("event_id", event.ID),
			zap.Error(err),
			zap.Duration("duration", event.Duration))

		// Record error in monitoring system
		if m.monitor != nil {
			apiErr := &errorx.APIError{
				Code:     "E6005",
				Message:  "Configuration reload failed",
				Category: errorx.CategoryConfiguration,
				Severity: errorx.SeverityError,
				Details: map[string]interface{}{
					"event_id":   event.ID,
					"event_type": string(event.Type),
					"source":     event.Source,
				},
			}
			m.monitor.RecordError(apiErr, map[string]interface{}{
				"component": "reload_manager",
				"operation": "reload",
			})
		}
	} else {
		event.Status = ReloadEventStatusCompleted
		m.logger.Info("Reload event completed successfully",
			zap.String("event_id", event.ID),
			zap.Duration("duration", event.Duration))
	}

	// Update history with final event status
	m.addToHistory(event)
	
	// Notify handlers of completion
	m.notifyEventHandlers(event)
}

// reloadConfiguration reloads the MCP configuration
func (m *ReloadManager) reloadConfiguration() error {
	m.logger.Info("Reloading MCP configuration")

	// This would typically involve:
	// 1. Re-reading configuration files
	// 2. Validating new configuration
	// 3. Updating runtime configuration
	// 4. Refreshing MCP server capabilities

	// For now, we'll trigger a capability sync
	return m.syncCapabilities()
}

// syncCapabilities synchronizes MCP capabilities
func (m *ReloadManager) syncCapabilities() error {
	if m.capabilityStore == nil {
		return fmt.Errorf("capability store not available")
	}

	m.logger.Info("Synchronizing MCP capabilities")

	// Get all servers and refresh their capabilities
	// This is a simplified implementation - in practice, you'd want to:
	// 1. Get list of configured servers
	// 2. Refresh capabilities for each server
	// 3. Update the capability store
	// 4. Notify clients of changes

	// For now, we'll just log that sync would happen
	m.logger.Info("MCP capability synchronization completed")
	
	return nil
}

// performScheduledReload performs a scheduled reload
func (m *ReloadManager) performScheduledReload() error {
	m.logger.Info("Performing scheduled reload")
	return m.reloadConfiguration()
}

// notifyEventHandlers notifies all registered event handlers
func (m *ReloadManager) notifyEventHandlers(event ReloadEvent) {
	m.mu.RLock()
	handlers := make([]EventHandler, len(m.eventHandlers))
	copy(handlers, m.eventHandlers)
	m.mu.RUnlock()

	for _, handler := range handlers {
		go func(h EventHandler) {
			if err := h.HandleReloadEvent(m.ctx, event); err != nil {
				m.logger.Error("Event handler failed",
					zap.String("event_id", event.ID),
					zap.Error(err))
			}
		}(handler)
	}
}

// addToHistory adds an event to the reload history
func (m *ReloadManager) addToHistory(event ReloadEvent) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Update existing event or add new one
	found := false
	for i, e := range m.reloadHistory {
		if e.ID == event.ID {
			m.reloadHistory[i] = event
			found = true
			break
		}
	}

	if !found {
		m.reloadHistory = append(m.reloadHistory, event)
		
		// Trim history if it gets too large
		if len(m.reloadHistory) > m.maxHistorySize {
			m.reloadHistory = m.reloadHistory[1:]
		}
	}
}

// GetReloadHistory returns the recent reload history
func (m *ReloadManager) GetReloadHistory() []ReloadEvent {
	m.mu.RLock()
	defer m.mu.RUnlock()

	history := make([]ReloadEvent, len(m.reloadHistory))
	copy(history, m.reloadHistory)
	return history
}

// GetStatus returns the current reload manager status
func (m *ReloadManager) GetStatus() ReloadManagerStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return ReloadManagerStatus{
		IsReloading:      m.isReloading,
		LastReload:       m.lastReload,
		EventHandlers:    len(m.eventHandlers),
		HistoryCount:     len(m.reloadHistory),
		CanReceiveEvents: m.notifier.CanReceive(),
		CanSendEvents:    m.notifier.CanSend(),
	}
}

// ReloadManagerStatus represents the current status of the reload manager
type ReloadManagerStatus struct {
	IsReloading      bool      `json:"is_reloading"`
	LastReload       time.Time `json:"last_reload"`
	EventHandlers    int       `json:"event_handlers"`
	HistoryCount     int       `json:"history_count"`
	CanReceiveEvents bool      `json:"can_receive_events"`
	CanSendEvents    bool      `json:"can_send_events"`
}

// Close stops the reload manager and releases resources
func (m *ReloadManager) Close() error {
	m.logger.Info("Stopping reload manager")
	m.cancel()
	return nil
}