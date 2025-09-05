package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/amoylab/unla/internal/apiserver/cache"
	"go.uber.org/zap"
)

// Cache handles cache-related API endpoints
type Cache struct {
	logger  *zap.Logger
	manager *cache.CacheManager
}

// NewCache creates a new cache handler
func NewCache(manager *cache.CacheManager, logger *zap.Logger) *Cache {
	return &Cache{
		logger:  logger.Named("handler.cache"),
		manager: manager,
	}
}

// CacheStatsResponse represents the response for cache statistics
type CacheStatsResponse struct {
	Stats        cache.CacheStats `json:"stats"`
	HealthCheck  HealthStatus     `json:"healthCheck"`
	Recommendations []string     `json:"recommendations,omitempty"`
}

type HealthStatus struct {
	L1Status    string  `json:"l1Status"`
	L2Status    string  `json:"l2Status"`
	OverallHitRate float64 `json:"overallHitRate"`
	Performance string   `json:"performance"`
}

// HandleGetCacheStats handles GET /api/cache/stats
func (h *Cache) HandleGetCacheStats(c *gin.Context) {
	stats := h.manager.GetStats()
	
	// Determine health status
	health := h.calculateHealthStatus(stats)
	recommendations := h.generateRecommendations(stats)

	response := CacheStatsResponse{
		Stats:        stats,
		HealthCheck:  health,
		Recommendations: recommendations,
	}

	c.JSON(http.StatusOK, gin.H{"data": response, "status": "success"})
}

// HandleCacheClear handles POST /api/cache/clear
func (h *Cache) HandleCacheClear(c *gin.Context) {
	if err := h.manager.Clear(c.Request.Context()); err != nil {
		h.logger.Error("failed to clear cache", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		return
	}

	h.logger.Info("cache cleared successfully")
	c.JSON(http.StatusOK, gin.H{"status": "success", "message": "Cache cleared successfully"})
}

// HandleInvalidateCapabilities handles DELETE /api/cache/capabilities/{tenant}/{name}
func (h *Cache) HandleInvalidateCapabilities(c *gin.Context) {
	tenant := c.Param("tenant")
	if tenant == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Tenant is required"})
		return
	}
	
	name := c.Param("name")
	if name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Server name is required"})
		return
	}

	if err := h.manager.InvalidateAll(c.Request.Context(), tenant, name); err != nil {
		h.logger.Error("failed to invalidate cache",
			zap.String("tenant", tenant),
			zap.String("name", name),
			zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		return
	}

	h.logger.Info("cache invalidated successfully",
		zap.String("tenant", tenant),
		zap.String("name", name))

	data := gin.H{
		"tenant": tenant,
		"name":   name,
		"invalidated": []string{"capabilities", "tools", "prompts", "resources", "resource_templates"},
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "data": data})
}

// HandleWarmupCache handles POST /api/cache/warmup/{tenant}
func (h *Cache) HandleWarmupCache(c *gin.Context) {
	tenant := c.Param("tenant")
	if tenant == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Tenant is required"})
		return
	}

	if err := h.manager.WarmupCapabilities(c.Request.Context(), tenant); err != nil {
		h.logger.Error("failed to warmup cache",
			zap.String("tenant", tenant),
			zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		return
	}

	h.logger.Info("cache warmup completed", zap.String("tenant", tenant))

	data := gin.H{
		"tenant": tenant,
		"status": "completed",
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "data": data})
}

// HandleCacheMonitoring handles GET /api/cache/monitoring
func (h *Cache) HandleCacheMonitoring(c *gin.Context) {
	stats := h.manager.GetStats()
	
	// Get query parameters for filtering
	formatParam := c.DefaultQuery("format", "detailed")
	intervalParam := c.DefaultQuery("interval", "5m")

	// Parse interval
	var intervalSeconds int64 = 300 // Default 5 minutes
	if interval, err := strconv.ParseInt(intervalParam[:len(intervalParam)-1], 10, 64); err == nil {
		switch intervalParam[len(intervalParam)-1:] {
		case "s":
			intervalSeconds = interval
		case "m":
			intervalSeconds = interval * 60
		case "h":
			intervalSeconds = interval * 3600
		}
	}

	monitoring := gin.H{
		"timestamp":    c.Request.Context().Value("timestamp"),
		"interval":     intervalParam,
		"stats":        stats,
		"alerts":       h.generateAlerts(stats),
		"trends":       h.calculateTrends(stats, intervalSeconds),
		"health":       h.calculateHealthStatus(stats),
	}

	if formatParam == "summary" {
		monitoring = gin.H{
			"hitRate":     stats.Total.HitRate,
			"operations":  stats.Total.Operations,
			"l1Entries":   stats.L1Memory.Entries,
			"performance": h.calculateHealthStatus(stats).Performance,
		}
	}

	c.JSON(http.StatusOK, monitoring)
}

// Helper methods

func (h *Cache) calculateHealthStatus(stats cache.CacheStats) HealthStatus {
	l1Status := "healthy"
	l2Status := "healthy"
	performance := "good"

	// Check L1 status
	if stats.L1Memory.Entries > 1000 {
		l1Status = "high_load"
	}
	if stats.L1Memory.Evictions > 100 {
		l1Status = "overloaded"
	}

	// Check L2 status
	if stats.L2Redis.Entries > 10000 {
		l2Status = "high_load"
	}

	// Overall performance
	if stats.Total.HitRate < 0.5 {
		performance = "poor"
	} else if stats.Total.HitRate < 0.8 {
		performance = "fair"
	} else {
		performance = "excellent"
	}

	return HealthStatus{
		L1Status:       l1Status,
		L2Status:       l2Status,
		OverallHitRate: stats.Total.HitRate,
		Performance:    performance,
	}
}

func (h *Cache) generateRecommendations(stats cache.CacheStats) []string {
	var recommendations []string

	if stats.Total.HitRate < 0.5 {
		recommendations = append(recommendations, "Consider increasing cache TTL or reviewing cache key strategies")
	}

	if stats.L1Memory.Evictions > 50 {
		recommendations = append(recommendations, "Consider increasing L1 cache size to reduce evictions")
	}

	if stats.Total.Operations > 1000 && stats.Total.HitRate > 0.9 {
		recommendations = append(recommendations, "Excellent cache performance - consider similar patterns for other data")
	}

	if stats.L1Memory.TotalSize > 80*1024*1024 { // 80MB
		recommendations = append(recommendations, "L1 cache size is approaching limit - monitor for performance impact")
	}

	return recommendations
}

func (h *Cache) generateAlerts(stats cache.CacheStats) []string {
	var alerts []string

	if stats.Total.HitRate < 0.3 {
		alerts = append(alerts, "Critical: Cache hit rate below 30%")
	}

	if stats.L1Memory.Evictions > 100 {
		alerts = append(alerts, "Warning: High number of L1 cache evictions")
	}

	if stats.L1Memory.TotalSize > 90*1024*1024 { // 90MB
		alerts = append(alerts, "Warning: L1 cache approaching size limit")
	}

	return alerts
}

func (h *Cache) calculateTrends(stats cache.CacheStats, intervalSeconds int64) gin.H {
	// This is a simplified trend calculation
	// In a real implementation, you would store historical data
	
	return gin.H{
		"hitRateTrend":    "stable", // Would be calculated from historical data
		"operationsTrend": "stable",
		"sizeTrend":       "stable",
		"interval":        intervalSeconds,
	}
}