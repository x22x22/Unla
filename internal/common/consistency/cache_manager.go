package consistency

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/amoylab/unla/internal/common/hotreload"
	"go.uber.org/zap"
)

// CacheConsistencyManager manages cache consistency across different layers
type CacheConsistencyManager struct {
	logger         *zap.Logger
	cacheProviders map[string]CacheProvider
	dependencies   map[string][]string // key -> dependent keys
	mu             sync.RWMutex
	invalidations  []InvalidationEvent
	maxHistory     int
}

// CacheProvider defines the interface for cache providers
type CacheProvider interface {
	Invalidate(ctx context.Context, keys []string) error
	InvalidatePattern(ctx context.Context, pattern string) error
	GetStats() CacheStats
	Name() string
}

// CacheStats represents cache statistics
type CacheStats struct {
	HitCount    int64 `json:"hit_count"`
	MissCount   int64 `json:"miss_count"`
	ItemCount   int64 `json:"item_count"`
	Size        int64 `json:"size"`
	LastUpdated time.Time `json:"last_updated"`
}

// InvalidationEvent represents a cache invalidation event
type InvalidationEvent struct {
	ID          string    `json:"id"`
	Timestamp   time.Time `json:"timestamp"`
	Keys        []string  `json:"keys"`
	Pattern     string    `json:"pattern,omitempty"`
	Source      string    `json:"source"`
	Reason      string    `json:"reason"`
	Providers   []string  `json:"providers"`
	Success     bool      `json:"success"`
	Error       string    `json:"error,omitempty"`
	Duration    time.Duration `json:"duration"`
}

// NewCacheConsistencyManager creates a new cache consistency manager
func NewCacheConsistencyManager(logger *zap.Logger) *CacheConsistencyManager {
	return &CacheConsistencyManager{
		logger:         logger.Named("cache_consistency"),
		cacheProviders: make(map[string]CacheProvider),
		dependencies:   make(map[string][]string),
		invalidations:  make([]InvalidationEvent, 0),
		maxHistory:     1000,
	}
}

// RegisterCacheProvider registers a cache provider
func (m *CacheConsistencyManager) RegisterCacheProvider(provider CacheProvider) {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	m.cacheProviders[provider.Name()] = provider
	m.logger.Info("Registered cache provider", zap.String("provider", provider.Name()))
}

// UnregisterCacheProvider unregisters a cache provider
func (m *CacheConsistencyManager) UnregisterCacheProvider(name string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	delete(m.cacheProviders, name)
	m.logger.Info("Unregistered cache provider", zap.String("provider", name))
}

// SetDependency sets a dependency relationship between cache keys
func (m *CacheConsistencyManager) SetDependency(key string, dependentKeys ...string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	m.dependencies[key] = dependentKeys
	m.logger.Debug("Set cache dependency",
		zap.String("key", key),
		zap.Strings("dependent_keys", dependentKeys))
}

// InvalidateKeys invalidates specific cache keys across all providers
func (m *CacheConsistencyManager) InvalidateKeys(ctx context.Context, keys []string, source string, reason string) error {
	if len(keys) == 0 {
		return nil
	}

	eventID := fmt.Sprintf("invalidate_%d", time.Now().UnixNano())
	startTime := time.Now()
	
	m.logger.Info("Starting cache key invalidation",
		zap.String("event_id", eventID),
		zap.Strings("keys", keys),
		zap.String("source", source),
		zap.String("reason", reason))

	// Get all dependent keys
	allKeys := m.getAllDependentKeys(keys)
	
	// Get providers to invalidate
	m.mu.RLock()
	providerNames := make([]string, 0, len(m.cacheProviders))
	providers := make([]CacheProvider, 0, len(m.cacheProviders))
	for name, provider := range m.cacheProviders {
		providerNames = append(providerNames, name)
		providers = append(providers, provider)
	}
	m.mu.RUnlock()

	// Invalidate across all providers
	var errors []string
	for _, provider := range providers {
		if err := provider.Invalidate(ctx, allKeys); err != nil {
			errorMsg := fmt.Sprintf("%s: %v", provider.Name(), err)
			errors = append(errors, errorMsg)
			m.logger.Error("Cache invalidation failed",
				zap.String("provider", provider.Name()),
				zap.Error(err))
		}
	}

	// Record invalidation event
	event := InvalidationEvent{
		ID:        eventID,
		Timestamp: startTime,
		Keys:      allKeys,
		Source:    source,
		Reason:    reason,
		Providers: providerNames,
		Success:   len(errors) == 0,
		Duration:  time.Since(startTime),
	}

	if len(errors) > 0 {
		event.Error = fmt.Sprintf("Errors: %v", errors)
	}

	m.addInvalidationEvent(event)

	if len(errors) > 0 {
		return fmt.Errorf("cache invalidation completed with errors: %v", errors)
	}

	m.logger.Info("Cache key invalidation completed successfully",
		zap.String("event_id", eventID),
		zap.Duration("duration", event.Duration),
		zap.Int("total_keys", len(allKeys)),
		zap.Int("providers", len(providers)))

	return nil
}

// InvalidatePattern invalidates cache keys matching a pattern across all providers
func (m *CacheConsistencyManager) InvalidatePattern(ctx context.Context, pattern string, source string, reason string) error {
	if pattern == "" {
		return nil
	}

	eventID := fmt.Sprintf("invalidate_pattern_%d", time.Now().UnixNano())
	startTime := time.Now()
	
	m.logger.Info("Starting cache pattern invalidation",
		zap.String("event_id", eventID),
		zap.String("pattern", pattern),
		zap.String("source", source),
		zap.String("reason", reason))

	// Get providers to invalidate
	m.mu.RLock()
	providerNames := make([]string, 0, len(m.cacheProviders))
	providers := make([]CacheProvider, 0, len(m.cacheProviders))
	for name, provider := range m.cacheProviders {
		providerNames = append(providerNames, name)
		providers = append(providers, provider)
	}
	m.mu.RUnlock()

	// Invalidate pattern across all providers
	var errors []string
	for _, provider := range providers {
		if err := provider.InvalidatePattern(ctx, pattern); err != nil {
			errorMsg := fmt.Sprintf("%s: %v", provider.Name(), err)
			errors = append(errors, errorMsg)
			m.logger.Error("Cache pattern invalidation failed",
				zap.String("provider", provider.Name()),
				zap.String("pattern", pattern),
				zap.Error(err))
		}
	}

	// Record invalidation event
	event := InvalidationEvent{
		ID:        eventID,
		Timestamp: startTime,
		Pattern:   pattern,
		Source:    source,
		Reason:    reason,
		Providers: providerNames,
		Success:   len(errors) == 0,
		Duration:  time.Since(startTime),
	}

	if len(errors) > 0 {
		event.Error = fmt.Sprintf("Errors: %v", errors)
	}

	m.addInvalidationEvent(event)

	if len(errors) > 0 {
		return fmt.Errorf("cache pattern invalidation completed with errors: %v", errors)
	}

	m.logger.Info("Cache pattern invalidation completed successfully",
		zap.String("event_id", eventID),
		zap.Duration("duration", event.Duration),
		zap.String("pattern", pattern),
		zap.Int("providers", len(providers)))

	return nil
}

// getAllDependentKeys returns all keys including their dependencies
func (m *CacheConsistencyManager) getAllDependentKeys(keys []string) []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	keySet := make(map[string]struct{})
	
	// Add original keys
	for _, key := range keys {
		keySet[key] = struct{}{}
	}
	
	// Add dependent keys recursively
	for _, key := range keys {
		m.addDependentKeysRecursive(key, keySet, make(map[string]struct{}))
	}
	
	// Convert set to slice
	allKeys := make([]string, 0, len(keySet))
	for key := range keySet {
		allKeys = append(allKeys, key)
	}
	
	return allKeys
}

// addDependentKeysRecursive recursively adds dependent keys to avoid cycles
func (m *CacheConsistencyManager) addDependentKeysRecursive(key string, keySet map[string]struct{}, visited map[string]struct{}) {
	if _, seen := visited[key]; seen {
		return // Cycle detected, skip
	}
	
	visited[key] = struct{}{}
	
	if dependentKeys, exists := m.dependencies[key]; exists {
		for _, depKey := range dependentKeys {
			keySet[depKey] = struct{}{}
			m.addDependentKeysRecursive(depKey, keySet, visited)
		}
	}
}

// addInvalidationEvent adds an invalidation event to the history
func (m *CacheConsistencyManager) addInvalidationEvent(event InvalidationEvent) {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	m.invalidations = append(m.invalidations, event)
	
	// Trim history if needed
	if len(m.invalidations) > m.maxHistory {
		m.invalidations = m.invalidations[len(m.invalidations)-m.maxHistory:]
	}
}

// GetInvalidationHistory returns the invalidation history
func (m *CacheConsistencyManager) GetInvalidationHistory() []InvalidationEvent {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	history := make([]InvalidationEvent, len(m.invalidations))
	copy(history, m.invalidations)
	return history
}

// GetCacheStats returns statistics for all cache providers
func (m *CacheConsistencyManager) GetCacheStats() map[string]CacheStats {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	stats := make(map[string]CacheStats)
	for name, provider := range m.cacheProviders {
		stats[name] = provider.GetStats()
	}
	
	return stats
}

// GetDependencies returns all cache dependencies
func (m *CacheConsistencyManager) GetDependencies() map[string][]string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	deps := make(map[string][]string)
	for key, values := range m.dependencies {
		deps[key] = make([]string, len(values))
		copy(deps[key], values)
	}
	
	return deps
}

// HandleReloadEvent handles reload events by invalidating relevant caches
func (m *CacheConsistencyManager) HandleReloadEvent(ctx context.Context, event hotreload.ReloadEvent) error {
	m.logger.Info("Handling reload event for cache consistency",
		zap.String("event_id", event.ID),
		zap.String("type", string(event.Type)))

	// Determine cache keys to invalidate based on reload event type
	var keysToInvalidate []string
	var patternsToInvalidate []string

	switch event.Type {
	case hotreload.ReloadEventTypeConfigChange:
		keysToInvalidate = []string{
			"mcp_servers",
			"gateway_config", 
			"server_capabilities",
		}
		patternsToInvalidate = []string{
			"config:*",
			"tenant:*:config",
		}
		
	case hotreload.ReloadEventTypeCapabilitySync:
		keysToInvalidate = []string{
			"server_capabilities",
			"tool_definitions",
			"resource_templates",
		}
		patternsToInvalidate = []string{
			"capability:*",
			"tenant:*:capabilities",
		}
		
	case hotreload.ReloadEventTypeManualReload, hotreload.ReloadEventTypeScheduledReload:
		// Invalidate everything
		patternsToInvalidate = []string{
			"*",
		}
	}

	// Invalidate specific keys
	if len(keysToInvalidate) > 0 {
		if err := m.InvalidateKeys(ctx, keysToInvalidate, "reload_event", event.ID); err != nil {
			m.logger.Error("Failed to invalidate cache keys during reload",
				zap.String("event_id", event.ID),
				zap.Error(err))
		}
	}

	// Invalidate patterns
	for _, pattern := range patternsToInvalidate {
		if err := m.InvalidatePattern(ctx, pattern, "reload_event", event.ID); err != nil {
			m.logger.Error("Failed to invalidate cache pattern during reload",
				zap.String("event_id", event.ID),
				zap.String("pattern", pattern),
				zap.Error(err))
		}
	}

	return nil
}

// ClearAll clears all caches across all providers
func (m *CacheConsistencyManager) ClearAll(ctx context.Context, source string, reason string) error {
	return m.InvalidatePattern(ctx, "*", source, reason)
}

// GetConsistencyReport generates a consistency report
func (m *CacheConsistencyManager) GetConsistencyReport() ConsistencyReport {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	stats := make(map[string]CacheStats)
	for name, provider := range m.cacheProviders {
		stats[name] = provider.GetStats()
	}
	
	return ConsistencyReport{
		Timestamp:     time.Now(),
		Providers:     len(m.cacheProviders),
		Dependencies:  len(m.dependencies),
		Stats:         stats,
		RecentEvents:  m.getRecentInvalidations(10),
		HealthStatus:  m.assessHealth(stats),
	}
}

// ConsistencyReport represents the cache consistency status
type ConsistencyReport struct {
	Timestamp     time.Time                  `json:"timestamp"`
	Providers     int                        `json:"providers"`
	Dependencies  int                        `json:"dependencies"`
	Stats         map[string]CacheStats      `json:"stats"`
	RecentEvents  []InvalidationEvent        `json:"recent_events"`
	HealthStatus  string                     `json:"health_status"`
}

// getRecentInvalidations returns the most recent invalidation events
func (m *CacheConsistencyManager) getRecentInvalidations(count int) []InvalidationEvent {
	if len(m.invalidations) == 0 {
		return []InvalidationEvent{}
	}
	
	start := len(m.invalidations) - count
	if start < 0 {
		start = 0
	}
	
	events := make([]InvalidationEvent, len(m.invalidations)-start)
	copy(events, m.invalidations[start:])
	return events
}

// assessHealth assesses the health of cache providers
func (m *CacheConsistencyManager) assessHealth(stats map[string]CacheStats) string {
	if len(stats) == 0 {
		return "no_providers"
	}
	
	// Simple health assessment based on hit rates and freshness
	healthyProviders := 0
	for _, stat := range stats {
		if stat.HitCount > 0 || stat.MissCount > 0 {
			hitRate := float64(stat.HitCount) / float64(stat.HitCount + stat.MissCount)
			if hitRate > 0.7 && time.Since(stat.LastUpdated) < time.Hour {
				healthyProviders++
			}
		}
	}
	
	healthRatio := float64(healthyProviders) / float64(len(stats))
	if healthRatio >= 0.8 {
		return "healthy"
	} else if healthRatio >= 0.5 {
		return "degraded"
	} else {
		return "unhealthy"
	}
}