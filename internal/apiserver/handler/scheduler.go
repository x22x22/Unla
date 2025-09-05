package handler

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/amoylab/unla/internal/apiserver/scheduler"
	"go.uber.org/zap"
)

// Scheduler handles scheduler-related API endpoints
type Scheduler struct {
	logger        *zap.Logger
	syncScheduler *scheduler.SyncScheduler
}

// NewScheduler creates a new scheduler handler
func NewScheduler(syncScheduler *scheduler.SyncScheduler, logger *zap.Logger) *Scheduler {
	return &Scheduler{
		logger:        logger.Named("handler.scheduler"),
		syncScheduler: syncScheduler,
	}
}

// SyncTaskRequest represents the request to create/update a sync task
type SyncTaskRequest struct {
	Tenant      string                 `json:"tenant" binding:"required"`
	ServerName  string                 `json:"serverName" binding:"required"`
	SyncTypes   []string               `json:"syncTypes" binding:"required"`
	Interval    string                 `json:"interval" binding:"required"` // e.g., "30m", "1h", "24h"
	Enabled     *bool                  `json:"enabled,omitempty"`
	RetryPolicy *scheduler.RetryPolicy `json:"retryPolicy,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// HandleGetSchedulerStatus handles GET /api/scheduler/status
func (h *Scheduler) HandleGetSchedulerStatus(c *gin.Context) {
	status := h.syncScheduler.GetStatus()
	
	response := gin.H{
		"scheduler": status,
		"uptime":    time.Since(time.Now().Add(-1 * time.Hour)).String(), // Placeholder uptime
		"version":   "1.0.0", // Placeholder version
	}
	
	c.JSON(http.StatusOK, gin.H{"data": response, "status": "success"})
}

// HandleListSyncTasks handles GET /api/scheduler/tasks
func (h *Scheduler) HandleListSyncTasks(c *gin.Context) {
	// Parse query parameters
	tenantFilter := c.Query("tenant")
	serverFilter := c.Query("server")
	statusFilter := c.Query("status")
	
	tasks := h.syncScheduler.ListTasks()
	
	// Apply filters
	var filteredTasks []*scheduler.SyncTask
	for _, task := range tasks {
		if tenantFilter != "" && task.Tenant != tenantFilter {
			continue
		}
		if serverFilter != "" && task.ServerName != serverFilter {
			continue
		}
		if statusFilter != "" && task.LastResult != nil && task.LastResult.Status != statusFilter {
			continue
		}
		
		filteredTasks = append(filteredTasks, task)
	}
	
	response := gin.H{
		"tasks":       filteredTasks,
		"totalCount":  len(tasks),
		"filteredCount": len(filteredTasks),
		"filters": gin.H{
			"tenant": tenantFilter,
			"server": serverFilter,
			"status": statusFilter,
		},
	}
	
	c.JSON(http.StatusOK, gin.H{"data": response, "status": "success"})
}

// HandleGetSyncTask handles GET /api/scheduler/tasks/{taskId}
func (h *Scheduler) HandleGetSyncTask(c *gin.Context) {
	taskID := c.Param("taskId")
	if taskID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Task ID is required"})
		return
	}
	
	task, err := h.syncScheduler.GetTask(taskID)
	if err != nil {
		h.logger.Error("failed to get sync task",
			zap.String("task_id", taskID),
			zap.Error(err))
		c.JSON(http.StatusNotFound, gin.H{"error": "Sync task not found"})
		return
	}
	
	c.JSON(http.StatusOK, gin.H{"data": task, "status": "success"})
}

// HandleCreateSyncTask handles POST /api/scheduler/tasks
func (h *Scheduler) HandleCreateSyncTask(c *gin.Context) {
	var req SyncTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	
	// Parse interval
	interval, err := time.ParseDuration(req.Interval)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid interval format"})
		return
	}
	
	if interval < time.Minute {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Minimum interval is 1 minute"})
		return
	}
	
	task := &scheduler.SyncTask{
		Tenant:     req.Tenant,
		ServerName: req.ServerName,
		SyncTypes:  req.SyncTypes,
		Interval:   interval,
		Enabled:    true,
		Metadata:   req.Metadata,
	}
	
	if req.Enabled != nil {
		task.Enabled = *req.Enabled
	}
	
	if req.RetryPolicy != nil {
		task.RetryPolicy = *req.RetryPolicy
	}
	
	if err := h.syncScheduler.AddTask(task); err != nil {
		h.logger.Error("failed to create sync task", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		return
	}
	
	h.logger.Info("sync task created",
		zap.String("task_id", task.ID),
		zap.String("tenant", req.Tenant),
		zap.String("server", req.ServerName))
	
	c.JSON(http.StatusOK, gin.H{"data": task, "status": "success"})
}

// HandleUpdateSyncTask handles PUT /api/scheduler/tasks/{taskId}
func (h *Scheduler) HandleUpdateSyncTask(c *gin.Context) {
	taskID := c.Param("taskId")
	if taskID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Task ID is required"})
		return
	}
	
	var req SyncTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	
	// Get existing task
	task, err := h.syncScheduler.GetTask(taskID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Sync task not found"})
		return
	}
	
	// Parse interval
	interval, err := time.ParseDuration(req.Interval)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid interval format"})
		return
	}
	
	if interval < time.Minute {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Minimum interval is 1 minute"})
		return
	}
	
	// Update task fields
	task.Tenant = req.Tenant
	task.ServerName = req.ServerName
	task.SyncTypes = req.SyncTypes
	task.Interval = interval
	task.Metadata = req.Metadata
	
	if req.Enabled != nil {
		task.Enabled = *req.Enabled
	}
	
	if req.RetryPolicy != nil {
		task.RetryPolicy = *req.RetryPolicy
	}
	
	// Update next run time if interval changed
	if task.NextRun.Before(time.Now()) {
		task.NextRun = time.Now().Add(interval)
	}
	
	if err := h.syncScheduler.UpdateTask(task); err != nil {
		h.logger.Error("failed to update sync task",
			zap.String("task_id", taskID),
			zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		return
	}
	
	h.logger.Info("sync task updated", zap.String("task_id", taskID))
	c.JSON(http.StatusOK, gin.H{"data": task, "status": "success"})
}

// HandleDeleteSyncTask handles DELETE /api/scheduler/tasks/{taskId}
func (h *Scheduler) HandleDeleteSyncTask(c *gin.Context) {
	taskID := c.Param("taskId")
	if taskID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Task ID is required"})
		return
	}
	
	if err := h.syncScheduler.RemoveTask(taskID); err != nil {
		h.logger.Error("failed to delete sync task",
			zap.String("task_id", taskID),
			zap.Error(err))
		c.JSON(http.StatusNotFound, gin.H{"error": "Sync task not found"})
		return
	}
	
	h.logger.Info("sync task deleted", zap.String("task_id", taskID))
	c.JSON(http.StatusOK, gin.H{"data": gin.H{"taskId": taskID}, "status": "success"})
}

// HandleToggleSyncTask handles POST /api/scheduler/tasks/{taskId}/toggle
func (h *Scheduler) HandleToggleSyncTask(c *gin.Context) {
	taskID := c.Param("taskId")
	if taskID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Task ID is required"})
		return
	}
	
	task, err := h.syncScheduler.GetTask(taskID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Sync task not found"})
		return
	}
	
	// Toggle enabled state
	task.Enabled = !task.Enabled
	
	if err := h.syncScheduler.UpdateTask(task); err != nil {
		h.logger.Error("failed to toggle sync task",
			zap.String("task_id", taskID),
			zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		return
	}
	
	action := "enabled"
	if !task.Enabled {
		action = "disabled"
	}
	
	h.logger.Info("sync task toggled",
		zap.String("task_id", taskID),
		zap.String("action", action))
	
	data := gin.H{
		"taskId":  taskID,
		"enabled": task.Enabled,
		"action":  action,
	}
	
	c.JSON(http.StatusOK, gin.H{"data": data, "status": "success"})
}

// HandleTriggerSyncTask handles POST /api/scheduler/tasks/{taskId}/trigger
func (h *Scheduler) HandleTriggerSyncTask(c *gin.Context) {
	taskID := c.Param("taskId")
	if taskID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Task ID is required"})
		return
	}
	
	task, err := h.syncScheduler.GetTask(taskID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Sync task not found"})
		return
	}
	
	// Update next run time to now to trigger immediate execution
	task.NextRun = time.Now()
	
	if err := h.syncScheduler.UpdateTask(task); err != nil {
		h.logger.Error("failed to trigger sync task",
			zap.String("task_id", taskID),
			zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		return
	}
	
	h.logger.Info("sync task manually triggered", zap.String("task_id", taskID))
	
	data := gin.H{
		"taskId":    taskID,
		"triggered": true,
		"nextRun":   task.NextRun.Format(time.RFC3339),
	}
	
	c.JSON(http.StatusOK, gin.H{"data": data, "status": "success"})
}

// HandleGetTaskHistory handles GET /api/scheduler/tasks/{taskId}/history
func (h *Scheduler) HandleGetTaskHistory(c *gin.Context) {
	taskID := c.Param("taskId")
	if taskID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Task ID is required"})
		return
	}
	
	// Parse query parameters
	limitStr := c.DefaultQuery("limit", "20")
	offsetStr := c.DefaultQuery("offset", "0")
	
	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit < 1 || limit > 100 {
		limit = 20
	}
	
	offset, err := strconv.Atoi(offsetStr)
	if err != nil || offset < 0 {
		offset = 0
	}
	
	task, err := h.syncScheduler.GetTask(taskID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Sync task not found"})
		return
	}
	
	// This is a simplified history response - in a real implementation,
	// you would query the database for historical sync records
	history := []gin.H{}
	if task.LastResult != nil {
		history = append(history, gin.H{
			"syncId":     task.LastResult.SyncID,
			"status":     task.LastResult.Status,
			"startTime":  task.LastResult.StartTime.Format(time.RFC3339),
			"endTime":    task.LastResult.EndTime.Format(time.RFC3339),
			"duration":   task.LastResult.Duration.String(),
			"error":      task.LastResult.Error,
			"retryCount": task.LastResult.RetryCount,
			"summary":    task.LastResult.Summary,
		})
	}
	
	response := gin.H{
		"taskId":   taskID,
		"history":  history,
		"count":    len(history),
		"limit":    limit,
		"offset":   offset,
	}
	
	c.JSON(http.StatusOK, gin.H{"data": response, "status": "success"})
}

// HandleGetSchedulerMetrics handles GET /api/scheduler/metrics
func (h *Scheduler) HandleGetSchedulerMetrics(c *gin.Context) {
	status := h.syncScheduler.GetStatus()
	tasks := h.syncScheduler.ListTasks()
	
	// Calculate metrics
	var successRate float64
	var avgInterval time.Duration
	recentFailures := 0
	
	if len(tasks) > 0 {
		successCount := 0
		totalInterval := time.Duration(0)
		
		for _, task := range tasks {
			if task.LastResult != nil {
				if task.LastResult.Status == "success" {
					successCount++
				} else if task.LastResult.Status == "failed" {
					// Count failures in the last 24 hours
					if time.Since(task.LastResult.EndTime) < 24*time.Hour {
						recentFailures++
					}
				}
			}
			totalInterval += task.Interval
		}
		
		if len(tasks) > 0 {
			successRate = float64(successCount) / float64(len(tasks))
			avgInterval = totalInterval / time.Duration(len(tasks))
		}
	}
	
	metrics := gin.H{
		"totalTasks":      len(tasks),
		"enabledTasks":    status.EnabledTasks,
		"runningTasks":    status.RunningTasks,
		"successRate":     successRate,
		"avgInterval":     avgInterval.String(),
		"recentFailures":  recentFailures,
		"lastUpdate":      time.Now().Format(time.RFC3339),
	}
	
	c.JSON(http.StatusOK, gin.H{
		"metrics": metrics,
		"status":  "ok",
	})
}