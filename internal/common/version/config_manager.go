package version

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/amoylab/unla/internal/common/hotreload"
	"go.uber.org/zap"
)

// ConfigVersionManager manages configuration versions and rollbacks
type ConfigVersionManager struct {
	logger         *zap.Logger
	versionsDir    string
	currentVersion string
	versions       map[string]ConfigVersion
	maxVersions    int
	mu             sync.RWMutex
	history        []VersionEvent
	maxHistory     int
}

// ConfigVersion represents a configuration version
type ConfigVersion struct {
	ID          string                 `json:"id"`
	Timestamp   time.Time              `json:"timestamp"`
	Hash        string                 `json:"hash"`
	Size        int64                  `json:"size"`
	Source      string                 `json:"source"`
	Description string                 `json:"description"`
	Tags        []string               `json:"tags"`
	Config      interface{}            `json:"config,omitempty"`
	FilePath    string                 `json:"file_path"`
	Metadata    map[string]interface{} `json:"metadata"`
	IsActive    bool                   `json:"is_active"`
	Parent      string                 `json:"parent,omitempty"`
	Changes     []ConfigChange         `json:"changes,omitempty"`
}

// ConfigChange represents a configuration change
type ConfigChange struct {
	Path      string      `json:"path"`
	Operation string      `json:"operation"` // "add", "update", "delete"
	OldValue  interface{} `json:"old_value,omitempty"`
	NewValue  interface{} `json:"new_value,omitempty"`
}

// VersionEvent represents a version management event
type VersionEvent struct {
	ID          string                 `json:"id"`
	Type        VersionEventType       `json:"type"`
	Status      VersionEventStatus     `json:"status"`
	Timestamp   time.Time              `json:"timestamp"`
	VersionID   string                 `json:"version_id"`
	Source      string                 `json:"source"`
	Duration    time.Duration          `json:"duration"`
	Error       string                 `json:"error,omitempty"`
	Details     map[string]interface{} `json:"details,omitempty"`
}

// VersionEventType represents the type of version event
type VersionEventType string

const (
	VersionEventTypeCreate   VersionEventType = "create"
	VersionEventTypeRollback VersionEventType = "rollback"
	VersionEventTypeDelete   VersionEventType = "delete"
	VersionEventTypeValidate VersionEventType = "validate"
	VersionEventTypeApply    VersionEventType = "apply"
)

// VersionEventStatus represents the status of a version event
type VersionEventStatus string

const (
	VersionEventStatusPending    VersionEventStatus = "pending"
	VersionEventStatusProcessing VersionEventStatus = "processing"
	VersionEventStatusCompleted  VersionEventStatus = "completed"
	VersionEventStatusFailed     VersionEventStatus = "failed"
)

// NewConfigVersionManager creates a new configuration version manager
func NewConfigVersionManager(logger *zap.Logger, versionsDir string) (*ConfigVersionManager, error) {
	// Ensure versions directory exists
	if err := os.MkdirAll(versionsDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create versions directory: %w", err)
	}

	manager := &ConfigVersionManager{
		logger:      logger.Named("config_version"),
		versionsDir: versionsDir,
		versions:    make(map[string]ConfigVersion),
		maxVersions: 100,
		history:     make([]VersionEvent, 0),
		maxHistory:  500,
	}

	// Load existing versions
	if err := manager.loadExistingVersions(); err != nil {
		manager.logger.Warn("Failed to load existing versions", zap.Error(err))
	}

	return manager, nil
}

// CreateVersion creates a new configuration version
func (m *ConfigVersionManager) CreateVersion(ctx context.Context, config interface{}, source string, description string, tags []string) (*ConfigVersion, error) {
	startTime := time.Now()
	eventID := fmt.Sprintf("create_%d", startTime.UnixNano())
	
	m.logger.Info("Creating new configuration version",
		zap.String("event_id", eventID),
		zap.String("source", source),
		zap.String("description", description))

	event := VersionEvent{
		ID:        eventID,
		Type:      VersionEventTypeCreate,
		Status:    VersionEventStatusProcessing,
		Timestamp: startTime,
		Source:    source,
		Details: map[string]interface{}{
			"description": description,
			"tags":        tags,
		},
	}

	// Serialize configuration
	configData, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		event.Status = VersionEventStatusFailed
		event.Error = fmt.Sprintf("Failed to serialize config: %v", err)
		event.Duration = time.Since(startTime)
		m.addVersionEvent(event)
		return nil, err
	}

	// Calculate hash
	hash := sha256.Sum256(configData)
	hashStr := hex.EncodeToString(hash[:])

	// Check if this version already exists
	m.mu.RLock()
	for _, version := range m.versions {
		if version.Hash == hashStr {
			m.mu.RUnlock()
			m.logger.Info("Configuration version already exists",
				zap.String("version_id", version.ID),
				zap.String("hash", hashStr))
			return &version, nil
		}
	}
	m.mu.RUnlock()

	// Create version ID
	versionID := fmt.Sprintf("v%d_%s", startTime.Unix(), hashStr[:8])

	// Save configuration to file
	filePath := filepath.Join(m.versionsDir, fmt.Sprintf("%s.json", versionID))
	if err := ioutil.WriteFile(filePath, configData, 0644); err != nil {
		event.Status = VersionEventStatusFailed
		event.Error = fmt.Sprintf("Failed to write config file: %v", err)
		event.Duration = time.Since(startTime)
		m.addVersionEvent(event)
		return nil, fmt.Errorf("failed to write config file: %w", err)
	}

	// Get current version for comparison
	var changes []ConfigChange
	if m.currentVersion != "" {
		if currentConfig, exists := m.versions[m.currentVersion]; exists {
			changes = m.calculateChanges(currentConfig.Config, config)
		}
	}

	// Create version record
	version := ConfigVersion{
		ID:          versionID,
		Timestamp:   startTime,
		Hash:        hashStr,
		Size:        int64(len(configData)),
		Source:      source,
		Description: description,
		Tags:        tags,
		Config:      config,
		FilePath:    filePath,
		Metadata: map[string]interface{}{
			"created_by": source,
			"size_bytes": len(configData),
		},
		IsActive: false,
		Parent:   m.currentVersion,
		Changes:  changes,
	}

	// Store version
	m.mu.Lock()
	m.versions[versionID] = version
	m.mu.Unlock()

	// Clean up old versions if needed
	if err := m.cleanupOldVersions(); err != nil {
		m.logger.Warn("Failed to cleanup old versions", zap.Error(err))
	}

	event.VersionID = versionID
	event.Status = VersionEventStatusCompleted
	event.Duration = time.Since(startTime)
	m.addVersionEvent(event)

	m.logger.Info("Successfully created configuration version",
		zap.String("version_id", versionID),
		zap.String("hash", hashStr),
		zap.Duration("duration", event.Duration))

	return &version, nil
}

// ApplyVersion applies a specific version as the current configuration
func (m *ConfigVersionManager) ApplyVersion(ctx context.Context, versionID string, source string) error {
	startTime := time.Now()
	eventID := fmt.Sprintf("apply_%d", startTime.UnixNano())
	
	m.logger.Info("Applying configuration version",
		zap.String("event_id", eventID),
		zap.String("version_id", versionID),
		zap.String("source", source))

	event := VersionEvent{
		ID:        eventID,
		Type:      VersionEventTypeApply,
		Status:    VersionEventStatusProcessing,
		Timestamp: startTime,
		VersionID: versionID,
		Source:    source,
	}

	// Get version
	m.mu.RLock()
	version, exists := m.versions[versionID]
	m.mu.RUnlock()

	if !exists {
		event.Status = VersionEventStatusFailed
		event.Error = "Version not found"
		event.Duration = time.Since(startTime)
		m.addVersionEvent(event)
		return fmt.Errorf("version %s not found", versionID)
	}

	// Load configuration if not already loaded
	if version.Config == nil {
		config, err := m.loadVersionConfig(version.FilePath)
		if err != nil {
			event.Status = VersionEventStatusFailed
			event.Error = fmt.Sprintf("Failed to load config: %v", err)
			event.Duration = time.Since(startTime)
			m.addVersionEvent(event)
			return fmt.Errorf("failed to load version config: %w", err)
		}
		version.Config = config
	}

	// Update version status
	m.mu.Lock()
	// Mark previous current version as inactive
	if m.currentVersion != "" {
		if prevVersion, exists := m.versions[m.currentVersion]; exists {
			prevVersion.IsActive = false
			m.versions[m.currentVersion] = prevVersion
		}
	}

	// Mark new version as active
	version.IsActive = true
	m.versions[versionID] = version
	m.currentVersion = versionID
	m.mu.Unlock()

	event.Status = VersionEventStatusCompleted
	event.Duration = time.Since(startTime)
	m.addVersionEvent(event)

	m.logger.Info("Successfully applied configuration version",
		zap.String("version_id", versionID),
		zap.Duration("duration", event.Duration))

	return nil
}

// RollbackToVersion rolls back to a specific configuration version
func (m *ConfigVersionManager) RollbackToVersion(ctx context.Context, versionID string, source string) error {
	startTime := time.Now()
	eventID := fmt.Sprintf("rollback_%d", startTime.UnixNano())
	
	m.logger.Info("Rolling back to configuration version",
		zap.String("event_id", eventID),
		zap.String("version_id", versionID),
		zap.String("source", source))

	event := VersionEvent{
		ID:        eventID,
		Type:      VersionEventTypeRollback,
		Status:    VersionEventStatusProcessing,
		Timestamp: startTime,
		VersionID: versionID,
		Source:    source,
	}

	// Validate version exists
	m.mu.RLock()
	_, exists := m.versions[versionID]
	currentVersionID := m.currentVersion
	m.mu.RUnlock()

	if !exists {
		event.Status = VersionEventStatusFailed
		event.Error = "Version not found"
		event.Duration = time.Since(startTime)
		m.addVersionEvent(event)
		return fmt.Errorf("version %s not found", versionID)
	}

	if versionID == currentVersionID {
		event.Status = VersionEventStatusFailed
		event.Error = "Cannot rollback to current version"
		event.Duration = time.Since(startTime)
		m.addVersionEvent(event)
		return fmt.Errorf("version %s is already current", versionID)
	}

	// Apply the version
	if err := m.ApplyVersion(ctx, versionID, fmt.Sprintf("rollback_%s", source)); err != nil {
		event.Status = VersionEventStatusFailed
		event.Error = fmt.Sprintf("Failed to apply version: %v", err)
		event.Duration = time.Since(startTime)
		m.addVersionEvent(event)
		return err
	}

	event.Status = VersionEventStatusCompleted
	event.Duration = time.Since(startTime)
	event.Details = map[string]interface{}{
		"previous_version": currentVersionID,
		"rollback_version": versionID,
	}
	m.addVersionEvent(event)

	m.logger.Info("Successfully rolled back to configuration version",
		zap.String("version_id", versionID),
		zap.String("previous_version", currentVersionID),
		zap.Duration("duration", event.Duration))

	return nil
}

// GetCurrentVersion returns the current active version
func (m *ConfigVersionManager) GetCurrentVersion() *ConfigVersion {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.currentVersion == "" {
		return nil
	}

	if version, exists := m.versions[m.currentVersion]; exists {
		return &version
	}

	return nil
}

// ListVersions returns all configuration versions
func (m *ConfigVersionManager) ListVersions() []ConfigVersion {
	m.mu.RLock()
	defer m.mu.RUnlock()

	versions := make([]ConfigVersion, 0, len(m.versions))
	for _, version := range m.versions {
		// Don't include the full config in the list for memory efficiency
		v := version
		v.Config = nil
		versions = append(versions, v)
	}

	// Sort by timestamp (newest first)
	sort.Slice(versions, func(i, j int) bool {
		return versions[i].Timestamp.After(versions[j].Timestamp)
	})

	return versions
}

// GetVersion returns a specific version by ID
func (m *ConfigVersionManager) GetVersion(versionID string) (*ConfigVersion, error) {
	m.mu.RLock()
	version, exists := m.versions[versionID]
	m.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("version %s not found", versionID)
	}

	// Load configuration if not already loaded
	if version.Config == nil {
		config, err := m.loadVersionConfig(version.FilePath)
		if err != nil {
			return nil, fmt.Errorf("failed to load version config: %w", err)
		}
		version.Config = config
	}

	return &version, nil
}

// DeleteVersion deletes a specific version
func (m *ConfigVersionManager) DeleteVersion(ctx context.Context, versionID string, source string) error {
	startTime := time.Now()
	eventID := fmt.Sprintf("delete_%d", startTime.UnixNano())
	
	event := VersionEvent{
		ID:        eventID,
		Type:      VersionEventTypeDelete,
		Status:    VersionEventStatusProcessing,
		Timestamp: startTime,
		VersionID: versionID,
		Source:    source,
	}

	m.mu.Lock()
	version, exists := m.versions[versionID]
	if !exists {
		m.mu.Unlock()
		event.Status = VersionEventStatusFailed
		event.Error = "Version not found"
		event.Duration = time.Since(startTime)
		m.addVersionEvent(event)
		return fmt.Errorf("version %s not found", versionID)
	}

	if versionID == m.currentVersion {
		m.mu.Unlock()
		event.Status = VersionEventStatusFailed
		event.Error = "Cannot delete current version"
		event.Duration = time.Since(startTime)
		m.addVersionEvent(event)
		return fmt.Errorf("cannot delete current version %s", versionID)
	}

	delete(m.versions, versionID)
	m.mu.Unlock()

	// Delete file
	if err := os.Remove(version.FilePath); err != nil {
		m.logger.Warn("Failed to delete version file",
			zap.String("version_id", versionID),
			zap.String("file_path", version.FilePath),
			zap.Error(err))
	}

	event.Status = VersionEventStatusCompleted
	event.Duration = time.Since(startTime)
	m.addVersionEvent(event)

	m.logger.Info("Successfully deleted configuration version",
		zap.String("version_id", versionID),
		zap.Duration("duration", event.Duration))

	return nil
}

// HandleReloadEvent handles reload events by creating version snapshots
func (m *ConfigVersionManager) HandleReloadEvent(ctx context.Context, event hotreload.ReloadEvent) error {
	// Only create versions for certain reload events
	switch event.Type {
	case hotreload.ReloadEventTypeConfigChange, hotreload.ReloadEventTypeManualReload:
		// This would require access to the current configuration
		// In a real implementation, you'd inject the configuration provider
		m.logger.Debug("Would create version snapshot for reload event",
			zap.String("event_id", event.ID),
			zap.String("type", string(event.Type)))
	}

	return nil
}

// loadExistingVersions loads versions from the versions directory
func (m *ConfigVersionManager) loadExistingVersions() error {
	files, err := ioutil.ReadDir(m.versionsDir)
	if err != nil {
		return err
	}

	for _, file := range files {
		if filepath.Ext(file.Name()) != ".json" {
			continue
		}

		filePath := filepath.Join(m.versionsDir, file.Name())
		versionID := strings.TrimSuffix(file.Name(), ".json")

		// Create version record (without loading the full config)
		version := ConfigVersion{
			ID:        versionID,
			Timestamp: file.ModTime(),
			FilePath:  filePath,
			Size:      file.Size(),
		}

		m.versions[versionID] = version
	}

	m.logger.Info("Loaded existing versions",
		zap.Int("count", len(m.versions)))

	return nil
}

// loadVersionConfig loads configuration from a version file
func (m *ConfigVersionManager) loadVersionConfig(filePath string) (interface{}, error) {
	data, err := ioutil.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	var config interface{}
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, err
	}

	return config, nil
}

// calculateChanges calculates differences between two configurations
func (m *ConfigVersionManager) calculateChanges(oldConfig, newConfig interface{}) []ConfigChange {
	// This is a simplified implementation
	// In practice, you'd want a more sophisticated diff algorithm
	
	var changes []ConfigChange
	
	// For now, just record that a change occurred
	changes = append(changes, ConfigChange{
		Path:      "root",
		Operation: "update",
		OldValue:  "previous_config",
		NewValue:  "new_config",
	})
	
	return changes
}

// cleanupOldVersions removes old versions if the limit is exceeded
func (m *ConfigVersionManager) cleanupOldVersions() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if len(m.versions) <= m.maxVersions {
		return nil
	}

	// Sort versions by timestamp
	versions := make([]ConfigVersion, 0, len(m.versions))
	for _, version := range m.versions {
		versions = append(versions, version)
	}

	sort.Slice(versions, func(i, j int) bool {
		return versions[i].Timestamp.Before(versions[j].Timestamp)
	})

	// Delete oldest versions (except current)
	toDelete := len(versions) - m.maxVersions
	for i := 0; i < toDelete; i++ {
		version := versions[i]
		if version.ID == m.currentVersion {
			continue // Never delete current version
		}

		delete(m.versions, version.ID)
		if err := os.Remove(version.FilePath); err != nil {
			m.logger.Warn("Failed to delete old version file",
				zap.String("version_id", version.ID),
				zap.Error(err))
		}
	}

	return nil
}

// addVersionEvent adds a version event to the history
func (m *ConfigVersionManager) addVersionEvent(event VersionEvent) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.history = append(m.history, event)
	
	// Trim history if needed
	if len(m.history) > m.maxHistory {
		m.history = m.history[len(m.history)-m.maxHistory:]
	}
}

// GetVersionHistory returns the version management history
func (m *ConfigVersionManager) GetVersionHistory() []VersionEvent {
	m.mu.RLock()
	defer m.mu.RUnlock()

	history := make([]VersionEvent, len(m.history))
	copy(history, m.history)
	return history
}

// GetVersionStats returns statistics about configuration versions
func (m *ConfigVersionManager) GetVersionStats() VersionStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := VersionStats{
		TotalVersions:  len(m.versions),
		CurrentVersion: m.currentVersion,
		MaxVersions:    m.maxVersions,
		StoragePath:    m.versionsDir,
	}

	var totalSize int64
	for _, version := range m.versions {
		totalSize += version.Size
		if version.Timestamp.After(stats.LatestTimestamp) {
			stats.LatestTimestamp = version.Timestamp
		}
		if stats.OldestTimestamp.IsZero() || version.Timestamp.Before(stats.OldestTimestamp) {
			stats.OldestTimestamp = version.Timestamp
		}
	}

	stats.TotalSize = totalSize
	return stats
}

// VersionStats represents statistics about configuration versions
type VersionStats struct {
	TotalVersions     int       `json:"total_versions"`
	CurrentVersion    string    `json:"current_version"`
	MaxVersions       int       `json:"max_versions"`
	TotalSize         int64     `json:"total_size"`
	StoragePath       string    `json:"storage_path"`
	LatestTimestamp   time.Time `json:"latest_timestamp"`
	OldestTimestamp   time.Time `json:"oldest_timestamp"`
}