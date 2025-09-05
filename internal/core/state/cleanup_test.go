package state

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/amoylab/unla/pkg/mcp"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func TestNewCapabilitiesCleanupManager(t *testing.T) {
	state := NewState()
	logger := zap.NewNop()
	
	manager := NewCapabilitiesCleanupManager(state, logger)
	
	assert.NotNil(t, manager)
	assert.Equal(t, state, manager.state)
	assert.Equal(t, DefaultCleanupInterval, manager.cleanupInterval)
	assert.Equal(t, DefaultCleanupThreshold, manager.threshold)
	assert.False(t, manager.IsRunning())
}

func TestCapabilitiesCleanupManagerStartStop(t *testing.T) {
	state := NewState()
	logger := zap.NewNop()
	manager := NewCapabilitiesCleanupManager(state, logger)
	
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	
	// Start manager
	manager.Start(ctx)
	assert.True(t, manager.IsRunning())
	
	// Stop manager
	manager.Stop()
	assert.False(t, manager.IsRunning())
	
	manager.Stop()
}

func TestCapabilitiesCleanupManagerSettings(t *testing.T) {
	state := NewState()
	logger := zap.NewNop()
	manager := NewCapabilitiesCleanupManager(state, logger)
	
	// Test setting cleanup interval
	newInterval := 30 * time.Second
	manager.SetCleanupInterval(newInterval)
	assert.Equal(t, newInterval, manager.cleanupInterval)
	
	// Test setting threshold
	manager.SetThreshold(0.5)
	assert.Equal(t, 0.5, manager.threshold)
	
	// Test invalid threshold (should be ignored)
	manager.SetThreshold(0)
	assert.Equal(t, 0.5, manager.threshold)
	
	manager.SetThreshold(1.5)
	assert.Equal(t, 0.5, manager.threshold)
}

func TestCapabilitiesCleanupManagerForceCleanup(t *testing.T) {
	state := NewState()
	logger := zap.NewNop()
	manager := NewCapabilitiesCleanupManager(state, logger)
	
	// Add some expired capabilities
	state.SetCapabilitiesWithTTL("tenant1", "server1", 
		&mcp.CapabilitiesInfo{Tools: []mcp.MCPTool{{Name: "tool1"}}}, 
		1*time.Millisecond)
	
	state.SetCapabilitiesWithTTL("tenant1", "server2", 
		&mcp.CapabilitiesInfo{Tools: []mcp.MCPTool{{Name: "tool2"}}}, 
		1*time.Hour)
	
	time.Sleep(10 * time.Millisecond) // Wait for expiration
	
	// Force cleanup
	removedCount := manager.ForceCleanup()
	assert.Equal(t, 1, removedCount)
	assert.Equal(t, 1, state.GetCapabilitiesCount())
}

func TestCapabilitiesCleanupManagerStats(t *testing.T) {
	state := NewState()
	logger := zap.NewNop()
	manager := NewCapabilitiesCleanupManager(state, logger)
	
	// Add test capabilities
	state.SetCapabilities("tenant1", "server1", &mcp.CapabilitiesInfo{
		Tools: []mcp.MCPTool{{Name: "tool1"}},
	})
	
	stats := manager.GetCleanupStats()
	
	assert.Contains(t, stats, "cleanupInterval")
	assert.Contains(t, stats, "cleanupThreshold")
	assert.Contains(t, stats, "cleanupRunning")
	assert.Contains(t, stats, "totalEntries")
	assert.Equal(t, 1, stats["totalEntries"])
	assert.Equal(t, false, stats["cleanupRunning"])
}

func TestPerformCleanup(t *testing.T) {
	state := NewState()
	logger := zap.NewNop()
	manager := NewCapabilitiesCleanupManager(state, logger)
	
	// Set low threshold for testing
	manager.SetThreshold(0.1)
	
	// Add expired capabilities to trigger cleanup
	for i := 0; i < 10; i++ {
		state.SetCapabilitiesWithTTL(
			"tenant1", 
			fmt.Sprintf("server%d", i),
			&mcp.CapabilitiesInfo{Tools: []mcp.MCPTool{{Name: fmt.Sprintf("tool%d", i)}}},
			1*time.Millisecond,
		)
	}
	
	// Add one valid capability
	state.SetCapabilities("tenant1", "server_valid", &mcp.CapabilitiesInfo{
		Tools: []mcp.MCPTool{{Name: "valid_tool"}},
	})
	
	initialCount := state.GetCapabilitiesCount()
	assert.Equal(t, 11, initialCount)
	
	// Wait for expiration
	time.Sleep(10 * time.Millisecond)
	
	// Perform cleanup
	manager.performCleanup()
	
	// Should have cleaned up expired entries
	finalCount := state.GetCapabilitiesCount()
	assert.Equal(t, 1, finalCount) // Only the valid one should remain
}

func TestScheduledCleanupTask(t *testing.T) {
	state := NewState()
	logger := zap.NewNop()
	
	// Add expired capabilities
	state.SetCapabilitiesWithTTL("tenant1", "server1", 
		&mcp.CapabilitiesInfo{Tools: []mcp.MCPTool{{Name: "tool1"}}}, 
		1*time.Millisecond)
	
	time.Sleep(10 * time.Millisecond) // Wait for expiration
	
	initialCount := state.GetCapabilitiesCount()
	assert.Equal(t, 1, initialCount)
	
	// Run scheduled cleanup with very short interval for testing
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	
	go state.ScheduledCleanupTask(ctx, logger, 10*time.Millisecond)
	
	// Wait for cleanup to run
	time.Sleep(50 * time.Millisecond)
	
	// Should have cleaned up expired entry
	finalCount := state.GetCapabilitiesCount()
	assert.Equal(t, 0, finalCount)
}

func TestHealthCheckCapabilities(t *testing.T) {
	state := NewState()
	
	// Test healthy state
	health := state.HealthCheckCapabilities()
	assert.Equal(t, "healthy", health["status"])
	assert.Equal(t, 0, health["totalEntries"])
	assert.Equal(t, 0, health["validEntries"])
	assert.Equal(t, 0, health["expiredEntries"])
	
	// Add some capabilities to test different states
	for i := 0; i < 10; i++ {
		state.SetCapabilities("tenant1", fmt.Sprintf("server%d", i), &mcp.CapabilitiesInfo{
			Tools: []mcp.MCPTool{{Name: fmt.Sprintf("tool%d", i)}},
		})
	}
	
	health = state.HealthCheckCapabilities()
	assert.Equal(t, "healthy", health["status"])
	assert.Equal(t, 10, health["totalEntries"])
	assert.Equal(t, 10, health["validEntries"])
	
	// Test high utilization warning
	for i := 10; i < int(float64(MaxCapabilitiesEntries)*0.85); i++ {
		state.SetCapabilities("tenant1", fmt.Sprintf("server%d", i), &mcp.CapabilitiesInfo{
			Tools: []mcp.MCPTool{{Name: fmt.Sprintf("tool%d", i)}},
		})
	}
	
	health = state.HealthCheckCapabilities()
	assert.Equal(t, "warning", health["status"])
	assert.Contains(t, health["message"], "Cache utilization high")
	
	// Test critical state by filling to max capacity
	remainingCapacity := MaxCapabilitiesEntries - state.GetCapabilitiesCount()
	for i := 0; i < remainingCapacity; i++ {
		state.SetCapabilities("tenant2", fmt.Sprintf("server%d", i), &mcp.CapabilitiesInfo{
			Tools: []mcp.MCPTool{{Name: fmt.Sprintf("tool%d", i)}},
		})
	}
	
	health = state.HealthCheckCapabilities()
	assert.Equal(t, "critical", health["status"])
	assert.Contains(t, health["message"], "Cache at maximum capacity")
}

func TestCleanupWithHighExpiredRatio(t *testing.T) {
	state := NewState()
	logger := zap.NewNop()
	manager := NewCapabilitiesCleanupManager(state, logger)
	
	// Add mostly expired capabilities
	for i := 0; i < 8; i++ {
		state.SetCapabilitiesWithTTL("tenant1", fmt.Sprintf("server%d", i), 
			&mcp.CapabilitiesInfo{Tools: []mcp.MCPTool{{Name: fmt.Sprintf("tool%d", i)}}}, 
			1*time.Millisecond)
	}
	
	// Add few valid capabilities
	for i := 8; i < 10; i++ {
		state.SetCapabilities("tenant1", fmt.Sprintf("server%d", i), &mcp.CapabilitiesInfo{
			Tools: []mcp.MCPTool{{Name: fmt.Sprintf("tool%d", i)}},
		})
	}
	
	time.Sleep(10 * time.Millisecond) // Wait for expiration
	
	initialCount := state.GetCapabilitiesCount()
	assert.Equal(t, 10, initialCount)
	
	// Perform cleanup (should trigger due to high expired ratio)
	manager.performCleanup()
	
	// Should have cleaned up expired entries
	finalCount := state.GetCapabilitiesCount()
	assert.Equal(t, 2, finalCount) // Only valid ones should remain
}

func TestCleanupManagerContextCancellation(t *testing.T) {
	state := NewState()
	logger := zap.NewNop()
	manager := NewCapabilitiesCleanupManager(state, logger)
	
	ctx, cancel := context.WithCancel(context.Background())
	
	// Start manager
	manager.Start(ctx)
	assert.True(t, manager.IsRunning())
	
	// Cancel context
	cancel()
	
	// Give some time for the cleanup loop to detect cancellation
	time.Sleep(10 * time.Millisecond)
	
	// Manager should still be marked as running (context cancellation is handled internally)
	assert.True(t, manager.IsRunning())
	
	// But we can still stop it manually
	manager.Stop()
	assert.False(t, manager.IsRunning())
}