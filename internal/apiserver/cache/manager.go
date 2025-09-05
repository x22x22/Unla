package cache

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/amoylab/unla/internal/mcp/storage"
	"github.com/amoylab/unla/pkg/mcp"
	"go.uber.org/zap"
)

// CacheManager provides a unified caching interface for MCP capabilities
type CacheManager struct {
	logger        *zap.Logger
	multilayer    *MultiLayerCache
	capabilityStore storage.CapabilityStore
}

// CacheManagerConfig holds configuration for cache manager
type CacheManagerConfig struct {
	RedisClient     redis.Cmdable
	CapabilityStore storage.CapabilityStore
	Logger          *zap.Logger
}

// NewCacheManager creates a new cache manager instance
func NewCacheManager(config CacheManagerConfig) *CacheManager {
	cacheConfig := MultiLayerCacheConfig{
		RedisClient: config.RedisClient,
		KeyPrefix:   "unla:mcp:cache:",
		L1TTL:       5 * time.Minute,   // 5 minutes for L1 cache
		L2TTL:       30 * time.Minute,  // 30 minutes for L2 cache
		MaxL1Size:   50 * 1024 * 1024,  // 50MB for L1 cache
	}

	return &CacheManager{
		logger:          config.Logger.Named("cache.manager"),
		multilayer:      NewMultiLayerCache(cacheConfig, config.Logger),
		capabilityStore: config.CapabilityStore,
	}
}

// GetCapabilities retrieves capabilities from cache or database
func (cm *CacheManager) GetCapabilities(ctx context.Context, tenant, serverName string) (*mcp.CapabilitiesInfo, error) {
	key := fmt.Sprintf("capabilities:%s:%s", tenant, serverName)

	// Try cache first
	if cached, found := cm.multilayer.Get(ctx, key); found {
		if capabilities, ok := cached.(*mcp.CapabilitiesInfo); ok {
			cm.logger.Debug("capabilities found in cache",
				zap.String("tenant", tenant),
				zap.String("server", serverName))
			return capabilities, nil
		}
	}

	// Cache miss, get from database
	capabilities, err := cm.capabilityStore.GetCapabilitiesInfo(ctx, tenant, serverName)
	if err != nil {
		return nil, fmt.Errorf("failed to get capabilities from store: %w", err)
	}

	// Cache for future use
	if err := cm.multilayer.Set(ctx, key, capabilities, 0); err != nil {
		cm.logger.Warn("failed to cache capabilities",
			zap.String("tenant", tenant),
			zap.String("server", serverName),
			zap.Error(err))
	}

	cm.logger.Debug("capabilities loaded from database and cached",
		zap.String("tenant", tenant),
		zap.String("server", serverName))

	return capabilities, nil
}

// InvalidateCapabilities removes capabilities from cache
func (cm *CacheManager) InvalidateCapabilities(ctx context.Context, tenant, serverName string) error {
	key := fmt.Sprintf("capabilities:%s:%s", tenant, serverName)
	return cm.multilayer.Delete(ctx, key)
}

// GetSyncStatus retrieves sync status from cache or database
func (cm *CacheManager) GetSyncStatus(ctx context.Context, syncID string) (*storage.SyncHistoryModel, error) {
	key := fmt.Sprintf("sync_status:%s", syncID)

	// Try cache first
	if cached, found := cm.multilayer.Get(ctx, key); found {
		if status, ok := cached.(*storage.SyncHistoryModel); ok {
			cm.logger.Debug("sync status found in cache", zap.String("syncId", syncID))
			return status, nil
		}
	}

	// Cache miss, get from database
	status, err := cm.capabilityStore.GetSyncRecord(ctx, syncID)
	if err != nil {
		return nil, fmt.Errorf("failed to get sync status from store: %w", err)
	}

	// Cache for future use (shorter TTL for sync status as it changes frequently)
	if err := cm.multilayer.Set(ctx, key, status, 2*time.Minute); err != nil {
		cm.logger.Warn("failed to cache sync status",
			zap.String("syncId", syncID),
			zap.Error(err))
	}

	return status, nil
}

// InvalidateSyncStatus removes sync status from cache
func (cm *CacheManager) InvalidateSyncStatus(ctx context.Context, syncID string) error {
	key := fmt.Sprintf("sync_status:%s", syncID)
	return cm.multilayer.Delete(ctx, key)
}

// GetTools retrieves tools from cache or database
func (cm *CacheManager) GetTools(ctx context.Context, tenant, serverName string) ([]mcp.MCPTool, error) {
	key := fmt.Sprintf("tools:%s:%s", tenant, serverName)

	// Try cache first
	if cached, found := cm.multilayer.Get(ctx, key); found {
		if tools, ok := cached.([]mcp.MCPTool); ok {
			cm.logger.Debug("tools found in cache",
				zap.String("tenant", tenant),
				zap.String("server", serverName))
			return tools, nil
		}
	}

	// Cache miss, get from database
	tools, err := cm.capabilityStore.ListTools(ctx, tenant, serverName)
	if err != nil {
		return nil, fmt.Errorf("failed to get tools from store: %w", err)
	}

	// Cache for future use
	if err := cm.multilayer.Set(ctx, key, tools, 0); err != nil {
		cm.logger.Warn("failed to cache tools",
			zap.String("tenant", tenant),
			zap.String("server", serverName),
			zap.Error(err))
	}

	return tools, nil
}

// InvalidateTools removes tools from cache
func (cm *CacheManager) InvalidateTools(ctx context.Context, tenant, serverName string) error {
	key := fmt.Sprintf("tools:%s:%s", tenant, serverName)
	return cm.multilayer.Delete(ctx, key)
}

// InvalidateAll removes all cached data for a specific server
func (cm *CacheManager) InvalidateAll(ctx context.Context, tenant, serverName string) error {
	keys := []string{
		fmt.Sprintf("capabilities:%s:%s", tenant, serverName),
		fmt.Sprintf("tools:%s:%s", tenant, serverName),
		fmt.Sprintf("prompts:%s:%s", tenant, serverName),
		fmt.Sprintf("resources:%s:%s", tenant, serverName),
		fmt.Sprintf("resource_templates:%s:%s", tenant, serverName),
	}

	for _, key := range keys {
		if err := cm.multilayer.Delete(ctx, key); err != nil {
			cm.logger.Warn("failed to invalidate cache key",
				zap.String("key", key),
				zap.Error(err))
		}
	}

	return nil
}

// WarmupCapabilities preloads capability data into cache
func (cm *CacheManager) WarmupCapabilities(ctx context.Context, tenant string) error {
	cm.logger.Info("starting capabilities cache warmup", zap.String("tenant", tenant))

	// Load all capabilities for the tenant from database
	loader := func(ctx context.Context) (map[string]interface{}, error) {
		// This would need to be implemented to query all servers for a tenant
		// For now, we return empty map as placeholder
		return make(map[string]interface{}), nil
	}

	return cm.multilayer.Warmup(ctx, loader)
}

// GetStats returns cache statistics
func (cm *CacheManager) GetStats() CacheStats {
	return cm.multilayer.GetStats()
}

// Clear removes all cached data
func (cm *CacheManager) Clear(ctx context.Context) error {
	return cm.multilayer.Clear(ctx)
}

// CacheKey generates a standardized cache key
func (cm *CacheManager) CacheKey(keyType, tenant, serverName string, additional ...string) string {
	baseKey := fmt.Sprintf("%s:%s:%s", keyType, tenant, serverName)
	for _, add := range additional {
		baseKey += ":" + add
	}
	return baseKey
}

// SetCustom allows setting custom cache entries
func (cm *CacheManager) SetCustom(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	return cm.multilayer.Set(ctx, key, value, ttl)
}

// GetCustom allows getting custom cache entries
func (cm *CacheManager) GetCustom(ctx context.Context, key string) (interface{}, bool) {
	return cm.multilayer.Get(ctx, key)
}

// DeleteCustom allows deleting custom cache entries
func (cm *CacheManager) DeleteCustom(ctx context.Context, key string) error {
	return cm.multilayer.Delete(ctx, key)
}