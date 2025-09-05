package state

import (
	"context"
	"sync/atomic"
	"time"

	"go.uber.org/zap"
)

const (
	// DefaultCleanupInterval defines how often the cleanup routine runs
	DefaultCleanupInterval = 2 * time.Minute
	// DefaultCleanupThreshold defines the percentage threshold to trigger cleanup
	DefaultCleanupThreshold = 0.8 // 80%
)

// CapabilitiesCleanupManager manages the automatic cleanup of expired capabilities
type CapabilitiesCleanupManager struct {
	state           *State
	logger          *zap.Logger
	cleanupInterval time.Duration
	threshold       float64
	running         *atomic.Bool
	stopChan        chan struct{}
	stopped         *atomic.Bool
}

// NewCapabilitiesCleanupManager creates a new cleanup manager
func NewCapabilitiesCleanupManager(state *State, logger *zap.Logger) *CapabilitiesCleanupManager {
	return &CapabilitiesCleanupManager{
		state:           state,
		logger:          logger,
		cleanupInterval: DefaultCleanupInterval,
		threshold:       DefaultCleanupThreshold,
		running:         &atomic.Bool{},
		stopChan:        make(chan struct{}),
		stopped:         &atomic.Bool{},
	}
}

// Start begins the automatic cleanup process
func (c *CapabilitiesCleanupManager) Start(ctx context.Context) {
	if c.running.CompareAndSwap(false, true) {
		go c.cleanupLoop(ctx)
		c.logger.Info("Started capabilities cleanup manager",
			zap.Duration("interval", c.cleanupInterval),
			zap.Float64("threshold", c.threshold))
	}
}

// Stop halts the automatic cleanup process
func (c *CapabilitiesCleanupManager) Stop() {
	if c.running.CompareAndSwap(true, false) {
		if c.stopped.CompareAndSwap(false, true) {
			close(c.stopChan)
		}
		c.logger.Info("Stopped capabilities cleanup manager")
	}
}

// SetCleanupInterval updates the cleanup interval
func (c *CapabilitiesCleanupManager) SetCleanupInterval(interval time.Duration) {
	c.cleanupInterval = interval
}

// SetThreshold updates the cleanup threshold
func (c *CapabilitiesCleanupManager) SetThreshold(threshold float64) {
	if threshold > 0 && threshold <= 1.0 {
		c.threshold = threshold
	}
}

// IsRunning returns whether the cleanup manager is running
func (c *CapabilitiesCleanupManager) IsRunning() bool {
	return c.running.Load()
}

// cleanupLoop runs the periodic cleanup process
func (c *CapabilitiesCleanupManager) cleanupLoop(ctx context.Context) {
	ticker := time.NewTicker(c.cleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			c.logger.Info("Capabilities cleanup loop stopped due to context cancellation")
			return
		case <-c.stopChan:
			c.logger.Info("Capabilities cleanup loop stopped")
			return
		case <-ticker.C:
			c.performCleanup()
		}
	}
}

// performCleanup executes the cleanup logic
func (c *CapabilitiesCleanupManager) performCleanup() {
	stats := c.state.GetCapabilitiesStats()
	totalEntries := stats["totalEntries"].(int)
	expiredEntries := stats["expiredEntries"].(int)
	
	// Check if cleanup is needed
	if totalEntries == 0 {
		return
	}
	
	expiredRatio := float64(expiredEntries) / float64(totalEntries)
	
	// Only cleanup if we have exceeded the threshold
	if expiredRatio >= c.threshold || totalEntries >= MaxCapabilitiesEntries {
		removedCount := c.state.CleanExpiredCapabilities()
		
		c.logger.Info("Performed capabilities cleanup",
			zap.Int("totalEntries", totalEntries),
			zap.Int("expiredEntries", expiredEntries),
			zap.Float64("expiredRatio", expiredRatio),
			zap.Int("removedCount", removedCount))
	}
}

// ForceCleanup immediately performs a cleanup operation
func (c *CapabilitiesCleanupManager) ForceCleanup() int {
	return c.state.CleanExpiredCapabilities()
}

// GetCleanupStats returns statistics about the cleanup manager
func (c *CapabilitiesCleanupManager) GetCleanupStats() map[string]interface{} {
	stats := c.state.GetCapabilitiesStats()
	stats["cleanupInterval"] = c.cleanupInterval.String()
	stats["cleanupThreshold"] = c.threshold
	stats["cleanupRunning"] = c.running.Load()
	return stats
}

// ScheduledCleanupTask performs cleanup based on a cron-like schedule
func (s *State) ScheduledCleanupTask(ctx context.Context, logger *zap.Logger, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	logger.Info("Started scheduled capabilities cleanup task", zap.Duration("interval", interval))

	for {
		select {
		case <-ctx.Done():
			logger.Info("Scheduled capabilities cleanup task stopped due to context cancellation")
			return
		case <-ticker.C:
			before := s.GetCapabilitiesCount()
			removed := s.CleanExpiredCapabilities()
			after := s.GetCapabilitiesCount()
			
			if removed > 0 {
				logger.Info("Scheduled capabilities cleanup completed",
					zap.Int("beforeCount", before),
					zap.Int("afterCount", after),
					zap.Int("removedCount", removed))
			}
		}
	}
}

// HealthCheck verifies the state of the capabilities cache
func (s *State) HealthCheckCapabilities() map[string]interface{} {
	stats := s.GetCapabilitiesStats()
	totalEntries := stats["totalEntries"].(int)
	expiredEntries := stats["expiredEntries"].(int)
	validEntries := stats["validEntries"].(int)
	
	health := map[string]interface{}{
		"status":        "healthy",
		"totalEntries":  totalEntries,
		"validEntries":  validEntries,
		"expiredEntries": expiredEntries,
		"cacheUtilization": float64(totalEntries) / float64(MaxCapabilitiesEntries),
	}
	
	// Determine health status
	if totalEntries >= MaxCapabilitiesEntries {
		health["status"] = "critical"
		health["message"] = "Cache at maximum capacity"
	} else if float64(expiredEntries)/float64(totalEntries) > DefaultCleanupThreshold {
		health["status"] = "warning"
		health["message"] = "High number of expired entries"
	} else if totalEntries > int(float64(MaxCapabilitiesEntries)*0.8) {
		health["status"] = "warning"
		health["message"] = "Cache utilization high"
	}
	
	return health
}