package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// CacheLayer represents the cache layer type
type CacheLayer string

const (
	L1Memory CacheLayer = "L1_MEMORY"
	L2Redis  CacheLayer = "L2_REDIS"
)

// CacheEntry represents a cached item with metadata
type CacheEntry struct {
	Data      interface{} `json:"data"`
	ExpiresAt time.Time   `json:"expiresAt"`
	CreatedAt time.Time   `json:"createdAt"`
	AccessCount int64     `json:"accessCount"`
	CacheLayer CacheLayer `json:"cacheLayer"`
	Size      int64       `json:"size,omitempty"`
}

// CacheStats represents cache statistics
type CacheStats struct {
	L1Memory MemoryStats `json:"l1Memory"`
	L2Redis  RedisStats  `json:"l2Redis"`
	Total    TotalStats  `json:"total"`
}

type MemoryStats struct {
	Entries     int64 `json:"entries"`
	Hits        int64 `json:"hits"`
	Misses      int64 `json:"misses"`
	Evictions   int64 `json:"evictions"`
	TotalSize   int64 `json:"totalSize"`
}

type RedisStats struct {
	Entries   int64 `json:"entries"`
	Hits      int64 `json:"hits"`
	Misses    int64 `json:"misses"`
	TotalSize int64 `json:"totalSize"`
}

type TotalStats struct {
	Hits       int64   `json:"hits"`
	Misses     int64   `json:"misses"`
	HitRate    float64 `json:"hitRate"`
	Operations int64   `json:"operations"`
}

// MultiLayerCache provides L1 (memory) + L2 (Redis) caching
type MultiLayerCache struct {
	logger    *zap.Logger
	l1Cache   *MemoryCache
	l2Cache   redis.Cmdable
	keyPrefix string
	
	// Statistics
	stats    CacheStats
	statsMux sync.RWMutex
	
	// Configuration
	l1TTL    time.Duration
	l2TTL    time.Duration
	maxL1Size int64
}

// MemoryCache represents the L1 memory cache
type MemoryCache struct {
	items    map[string]*CacheEntry
	mutex    sync.RWMutex
	maxSize  int64
	currentSize int64
	
	// LRU tracking
	accessOrder []string
	accessMux   sync.Mutex
}

// MultiLayerCacheConfig holds configuration for the cache
type MultiLayerCacheConfig struct {
	RedisClient redis.Cmdable
	KeyPrefix   string
	L1TTL       time.Duration
	L2TTL       time.Duration
	MaxL1Size   int64 // Maximum L1 cache size in bytes
}

// NewMultiLayerCache creates a new multi-layer cache instance
func NewMultiLayerCache(config MultiLayerCacheConfig, logger *zap.Logger) *MultiLayerCache {
	if config.L1TTL == 0 {
		config.L1TTL = 5 * time.Minute
	}
	if config.L2TTL == 0 {
		config.L2TTL = 30 * time.Minute
	}
	if config.MaxL1Size == 0 {
		config.MaxL1Size = 100 * 1024 * 1024 // 100MB
	}

	l1Cache := &MemoryCache{
		items:       make(map[string]*CacheEntry),
		maxSize:     config.MaxL1Size,
		accessOrder: make([]string, 0),
	}

	cache := &MultiLayerCache{
		logger:    logger.Named("cache.multilayer"),
		l1Cache:   l1Cache,
		l2Cache:   config.RedisClient,
		keyPrefix: config.KeyPrefix,
		l1TTL:     config.L1TTL,
		l2TTL:     config.L2TTL,
		maxL1Size: config.MaxL1Size,
		stats:     CacheStats{},
	}

	// Start background cleanup goroutine
	go cache.startCleanupRoutine(context.Background())

	return cache
}

// Get retrieves an item from cache, checking L1 first, then L2
func (mlc *MultiLayerCache) Get(ctx context.Context, key string) (interface{}, bool) {
	// Try L1 cache first
	if entry, found := mlc.getFromL1(key); found {
		mlc.updateStats(func(stats *CacheStats) {
			stats.L1Memory.Hits++
			stats.Total.Hits++
			stats.Total.Operations++
		})
		return entry.Data, true
	}

	// Try L2 cache
	if entry, found := mlc.getFromL2(ctx, key); found {
		// Promote to L1 cache
		mlc.setToL1(key, entry)
		
		mlc.updateStats(func(stats *CacheStats) {
			stats.L1Memory.Misses++
			stats.L2Redis.Hits++
			stats.Total.Hits++
			stats.Total.Operations++
		})
		return entry.Data, true
	}

	// Cache miss
	mlc.updateStats(func(stats *CacheStats) {
		stats.L1Memory.Misses++
		stats.L2Redis.Misses++
		stats.Total.Misses++
		stats.Total.Operations++
	})
	return nil, false
}

// Set stores an item in both L1 and L2 cache
func (mlc *MultiLayerCache) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	now := time.Now()
	
	// Use provided TTL or default L1TTL
	if ttl == 0 {
		ttl = mlc.l1TTL
	}

	entry := &CacheEntry{
		Data:        value,
		ExpiresAt:   now.Add(ttl),
		CreatedAt:   now,
		AccessCount: 0,
		CacheLayer:  L1Memory,
		Size:        mlc.estimateSize(value),
	}

	// Set in L1 cache
	mlc.setToL1(key, entry)

	// Set in L2 cache with longer TTL
	l2Entry := *entry
	l2Entry.ExpiresAt = now.Add(mlc.l2TTL)
	l2Entry.CacheLayer = L2Redis
	
	return mlc.setToL2(ctx, key, &l2Entry)
}

// Delete removes an item from both cache layers
func (mlc *MultiLayerCache) Delete(ctx context.Context, key string) error {
	// Remove from L1
	mlc.deleteFromL1(key)

	// Remove from L2
	redisKey := mlc.redisKey(key)
	return mlc.l2Cache.Del(ctx, redisKey).Err()
}

// Clear removes all items from both cache layers
func (mlc *MultiLayerCache) Clear(ctx context.Context) error {
	// Clear L1
	mlc.l1Cache.mutex.Lock()
	mlc.l1Cache.items = make(map[string]*CacheEntry)
	mlc.l1Cache.accessOrder = make([]string, 0)
	mlc.l1Cache.currentSize = 0
	mlc.l1Cache.mutex.Unlock()

	// Clear L2 (all keys with prefix)
	pattern := mlc.keyPrefix + "*"
	keys, err := mlc.l2Cache.Keys(ctx, pattern).Result()
	if err != nil {
		return err
	}

	if len(keys) > 0 {
		return mlc.l2Cache.Del(ctx, keys...).Err()
	}

	mlc.resetStats()
	return nil
}

// Warmup preloads cache with data using the provided function
func (mlc *MultiLayerCache) Warmup(ctx context.Context, keyDataLoader func(ctx context.Context) (map[string]interface{}, error)) error {
	mlc.logger.Info("starting cache warmup")
	
	data, err := keyDataLoader(ctx)
	if err != nil {
		return fmt.Errorf("failed to load warmup data: %w", err)
	}

	warmed := 0
	for key, value := range data {
		if err := mlc.Set(ctx, key, value, 0); err != nil {
			mlc.logger.Warn("failed to warmup cache entry", 
				zap.String("key", key), zap.Error(err))
		} else {
			warmed++
		}
	}

	mlc.logger.Info("cache warmup completed", 
		zap.Int("warmed_entries", warmed), 
		zap.Int("total_entries", len(data)))
	
	return nil
}

// GetStats returns current cache statistics
func (mlc *MultiLayerCache) GetStats() CacheStats {
	mlc.statsMux.RLock()
	defer mlc.statsMux.RUnlock()

	stats := mlc.stats
	
	// Calculate hit rate
	if stats.Total.Operations > 0 {
		stats.Total.HitRate = float64(stats.Total.Hits) / float64(stats.Total.Operations)
	}

	// Update current entries count
	mlc.l1Cache.mutex.RLock()
	stats.L1Memory.Entries = int64(len(mlc.l1Cache.items))
	stats.L1Memory.TotalSize = mlc.l1Cache.currentSize
	mlc.l1Cache.mutex.RUnlock()

	return stats
}

// L1 cache operations

func (mlc *MultiLayerCache) getFromL1(key string) (*CacheEntry, bool) {
	mlc.l1Cache.mutex.RLock()
	defer mlc.l1Cache.mutex.RUnlock()

	entry, exists := mlc.l1Cache.items[key]
	if !exists || entry.ExpiresAt.Before(time.Now()) {
		return nil, false
	}

	// Update access tracking
	entry.AccessCount++
	mlc.updateAccessOrder(key)

	return entry, true
}

func (mlc *MultiLayerCache) setToL1(key string, entry *CacheEntry) {
	mlc.l1Cache.mutex.Lock()
	defer mlc.l1Cache.mutex.Unlock()

	// Check if we need to evict items to make space
	if mlc.l1Cache.currentSize+entry.Size > mlc.l1Cache.maxSize {
		mlc.evictLRU()
	}

	// Store the entry
	mlc.l1Cache.items[key] = entry
	mlc.l1Cache.currentSize += entry.Size
	mlc.updateAccessOrder(key)

	mlc.updateStats(func(stats *CacheStats) {
		stats.L1Memory.Entries = int64(len(mlc.l1Cache.items))
		stats.L1Memory.TotalSize = mlc.l1Cache.currentSize
	})
}

func (mlc *MultiLayerCache) deleteFromL1(key string) {
	mlc.l1Cache.mutex.Lock()
	defer mlc.l1Cache.mutex.Unlock()

	if entry, exists := mlc.l1Cache.items[key]; exists {
		mlc.l1Cache.currentSize -= entry.Size
		delete(mlc.l1Cache.items, key)
		mlc.removeFromAccessOrder(key)
	}
}

// L2 cache operations

func (mlc *MultiLayerCache) getFromL2(ctx context.Context, key string) (*CacheEntry, bool) {
	redisKey := mlc.redisKey(key)
	data, err := mlc.l2Cache.Get(ctx, redisKey).Result()
	if err == redis.Nil {
		return nil, false
	}
	if err != nil {
		mlc.logger.Error("failed to get from L2 cache", 
			zap.String("key", key), zap.Error(err))
		return nil, false
	}

	var entry CacheEntry
	if err := json.Unmarshal([]byte(data), &entry); err != nil {
		mlc.logger.Error("failed to unmarshal L2 cache entry", 
			zap.String("key", key), zap.Error(err))
		return nil, false
	}

	if entry.ExpiresAt.Before(time.Now()) {
		// Entry expired, delete it
		mlc.l2Cache.Del(ctx, redisKey)
		return nil, false
	}

	return &entry, true
}

func (mlc *MultiLayerCache) setToL2(ctx context.Context, key string, entry *CacheEntry) error {
	redisKey := mlc.redisKey(key)
	data, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("failed to marshal cache entry: %w", err)
	}

	ttl := time.Until(entry.ExpiresAt)
	if ttl <= 0 {
		return nil // Don't store expired entries
	}

	return mlc.l2Cache.Set(ctx, redisKey, data, ttl).Err()
}

// Utility methods

func (mlc *MultiLayerCache) redisKey(key string) string {
	return mlc.keyPrefix + key
}

func (mlc *MultiLayerCache) estimateSize(value interface{}) int64 {
	// Simple size estimation based on JSON marshaling
	data, err := json.Marshal(value)
	if err != nil {
		return 1024 // Default size estimate
	}
	return int64(len(data))
}

func (mlc *MultiLayerCache) updateAccessOrder(key string) {
	mlc.l1Cache.accessMux.Lock()
	defer mlc.l1Cache.accessMux.Unlock()

	// Remove key from current position
	for i, k := range mlc.l1Cache.accessOrder {
		if k == key {
			mlc.l1Cache.accessOrder = append(mlc.l1Cache.accessOrder[:i], mlc.l1Cache.accessOrder[i+1:]...)
			break
		}
	}

	// Add to end (most recently used)
	mlc.l1Cache.accessOrder = append(mlc.l1Cache.accessOrder, key)
}

func (mlc *MultiLayerCache) removeFromAccessOrder(key string) {
	for i, k := range mlc.l1Cache.accessOrder {
		if k == key {
			mlc.l1Cache.accessOrder = append(mlc.l1Cache.accessOrder[:i], mlc.l1Cache.accessOrder[i+1:]...)
			break
		}
	}
}

func (mlc *MultiLayerCache) evictLRU() {
	if len(mlc.l1Cache.accessOrder) == 0 {
		return
	}

	// Evict least recently used items until we have enough space
	for mlc.l1Cache.currentSize > mlc.l1Cache.maxSize/2 && len(mlc.l1Cache.accessOrder) > 0 {
		lruKey := mlc.l1Cache.accessOrder[0]
		if entry, exists := mlc.l1Cache.items[lruKey]; exists {
			mlc.l1Cache.currentSize -= entry.Size
			delete(mlc.l1Cache.items, lruKey)
		}
		mlc.l1Cache.accessOrder = mlc.l1Cache.accessOrder[1:]

		mlc.updateStats(func(stats *CacheStats) {
			stats.L1Memory.Evictions++
		})
	}
}

func (mlc *MultiLayerCache) updateStats(updateFunc func(*CacheStats)) {
	mlc.statsMux.Lock()
	defer mlc.statsMux.Unlock()
	updateFunc(&mlc.stats)
}

func (mlc *MultiLayerCache) resetStats() {
	mlc.statsMux.Lock()
	defer mlc.statsMux.Unlock()
	mlc.stats = CacheStats{}
}

func (mlc *MultiLayerCache) startCleanupRoutine(ctx context.Context) {
	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			mlc.cleanupExpiredEntries()
		}
	}
}

func (mlc *MultiLayerCache) cleanupExpiredEntries() {
	now := time.Now()
	mlc.l1Cache.mutex.Lock()
	defer mlc.l1Cache.mutex.Unlock()

	toDelete := make([]string, 0)
	for key, entry := range mlc.l1Cache.items {
		if entry.ExpiresAt.Before(now) {
			toDelete = append(toDelete, key)
		}
	}

	for _, key := range toDelete {
		if entry, exists := mlc.l1Cache.items[key]; exists {
			mlc.l1Cache.currentSize -= entry.Size
			delete(mlc.l1Cache.items, key)
			mlc.removeFromAccessOrder(key)
		}
	}

	if len(toDelete) > 0 {
		mlc.logger.Debug("cleaned up expired cache entries", 
			zap.Int("count", len(toDelete)))
	}
}