package scheduler

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/amoylab/unla/internal/common/config"
	"github.com/amoylab/unla/internal/mcp/storage"
	"go.uber.org/zap"
)

// SyncScheduler manages background synchronization tasks
type SyncScheduler struct {
	logger             *zap.Logger
	capabilityStore    storage.CapabilityStore
	configProvider     ConfigProvider
	syncHandler        SyncHandler
	tasks              map[string]*SyncTask
	taskMutex          sync.RWMutex
	ctx                context.Context
	cancel             context.CancelFunc
	running            bool
	runningMutex       sync.RWMutex
}

// ConfigProvider interface for getting MCP configurations
type ConfigProvider interface {
	GetAll(ctx context.Context) ([]*config.MCPConfig, error)
	Get(ctx context.Context, tenant, name string) (*config.MCPConfig, error)
}

// SyncHandler interface for performing synchronization
type SyncHandler interface {
	PerformSync(ctx context.Context, cfg *config.MCPConfig, syncID string, types []string, force bool)
}

// SyncTask represents a scheduled synchronization task
type SyncTask struct {
	ID              string                 `json:"id"`
	Tenant          string                 `json:"tenant"`
	ServerName      string                 `json:"serverName"`
	SyncTypes       []string               `json:"syncTypes"`
	Interval        time.Duration          `json:"interval"`
	NextRun         time.Time              `json:"nextRun"`
	LastRun         *time.Time             `json:"lastRun,omitempty"`
	LastResult      *SyncResult            `json:"lastResult,omitempty"`
	RetryPolicy     RetryPolicy            `json:"retryPolicy"`
	Enabled         bool                   `json:"enabled"`
	CreatedAt       time.Time              `json:"createdAt"`
	UpdatedAt       time.Time              `json:"updatedAt"`
	Metadata        map[string]interface{} `json:"metadata,omitempty"`
}

// SyncResult represents the result of a sync operation
type SyncResult struct {
	Status        string                 `json:"status"`
	StartTime     time.Time              `json:"startTime"`
	EndTime       time.Time              `json:"endTime"`
	Duration      time.Duration          `json:"duration"`
	Error         string                 `json:"error,omitempty"`
	RetryCount    int                    `json:"retryCount"`
	Summary       map[string]interface{} `json:"summary,omitempty"`
	SyncID        string                 `json:"syncId"`
}

// RetryPolicy defines how to handle sync failures
type RetryPolicy struct {
	MaxRetries      int           `json:"maxRetries"`
	BaseDelay       time.Duration `json:"baseDelay"`
	MaxDelay        time.Duration `json:"maxDelay"`
	BackoffFactor   float64       `json:"backoffFactor"`
	RetryOnFailure  bool          `json:"retryOnFailure"`
}

// SyncSchedulerConfig holds configuration for the sync scheduler
type SyncSchedulerConfig struct {
	ConfigProvider  ConfigProvider
	SyncHandler     SyncHandler
	CapabilityStore storage.CapabilityStore
	Logger          *zap.Logger
	
	// Default policies
	DefaultInterval    time.Duration
	DefaultRetryPolicy RetryPolicy
}

// NewSyncScheduler creates a new sync scheduler
func NewSyncScheduler(config SyncSchedulerConfig) *SyncScheduler {
	if config.DefaultInterval == 0 {
		config.DefaultInterval = 30 * time.Minute
	}
	
	if config.DefaultRetryPolicy.MaxRetries == 0 {
		config.DefaultRetryPolicy = RetryPolicy{
			MaxRetries:     3,
			BaseDelay:      1 * time.Minute,
			MaxDelay:       10 * time.Minute,
			BackoffFactor:  2.0,
			RetryOnFailure: true,
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	
	return &SyncScheduler{
		logger:          config.Logger.Named("scheduler.sync"),
		capabilityStore: config.CapabilityStore,
		configProvider:  config.ConfigProvider,
		syncHandler:     config.SyncHandler,
		tasks:           make(map[string]*SyncTask),
		ctx:             ctx,
		cancel:          cancel,
		running:         false,
	}
}

// Start begins the sync scheduler
func (ss *SyncScheduler) Start() error {
	ss.runningMutex.Lock()
	defer ss.runningMutex.Unlock()
	
	if ss.running {
		return fmt.Errorf("sync scheduler is already running")
	}
	
	ss.running = true
	ss.logger.Info("starting sync scheduler")
	
	// Load existing tasks from database/configuration
	if err := ss.loadTasks(); err != nil {
		ss.logger.Error("failed to load sync tasks", zap.Error(err))
	}
	
	// Start the scheduler loop
	go ss.schedulerLoop()
	
	return nil
}

// Stop stops the sync scheduler
func (ss *SyncScheduler) Stop() error {
	ss.runningMutex.Lock()
	defer ss.runningMutex.Unlock()
	
	if !ss.running {
		return nil
	}
	
	ss.logger.Info("stopping sync scheduler")
	ss.cancel()
	ss.running = false
	
	return nil
}

// AddTask adds a new sync task
func (ss *SyncScheduler) AddTask(task *SyncTask) error {
	if task.ID == "" {
		task.ID = uuid.New().String()
	}
	
	if task.Interval == 0 {
		task.Interval = 30 * time.Minute
	}
	
	if task.NextRun.IsZero() {
		task.NextRun = time.Now().Add(task.Interval)
	}
	
	if task.RetryPolicy.MaxRetries == 0 {
		task.RetryPolicy = RetryPolicy{
			MaxRetries:     3,
			BaseDelay:      1 * time.Minute,
			MaxDelay:       10 * time.Minute,
			BackoffFactor:  2.0,
			RetryOnFailure: true,
		}
	}
	
	task.CreatedAt = time.Now()
	task.UpdatedAt = time.Now()
	task.Enabled = true
	
	ss.taskMutex.Lock()
	defer ss.taskMutex.Unlock()
	
	ss.tasks[task.ID] = task
	
	ss.logger.Info("added sync task",
		zap.String("task_id", task.ID),
		zap.String("tenant", task.Tenant),
		zap.String("server", task.ServerName),
		zap.Duration("interval", task.Interval))
	
	return nil
}

// RemoveTask removes a sync task
func (ss *SyncScheduler) RemoveTask(taskID string) error {
	ss.taskMutex.Lock()
	defer ss.taskMutex.Unlock()
	
	if _, exists := ss.tasks[taskID]; !exists {
		return fmt.Errorf("task not found: %s", taskID)
	}
	
	delete(ss.tasks, taskID)
	
	ss.logger.Info("removed sync task", zap.String("task_id", taskID))
	
	return nil
}

// UpdateTask updates an existing sync task
func (ss *SyncScheduler) UpdateTask(task *SyncTask) error {
	ss.taskMutex.Lock()
	defer ss.taskMutex.Unlock()
	
	if _, exists := ss.tasks[task.ID]; !exists {
		return fmt.Errorf("task not found: %s", task.ID)
	}
	
	task.UpdatedAt = time.Now()
	ss.tasks[task.ID] = task
	
	ss.logger.Info("updated sync task", zap.String("task_id", task.ID))
	
	return nil
}

// GetTask retrieves a sync task by ID
func (ss *SyncScheduler) GetTask(taskID string) (*SyncTask, error) {
	ss.taskMutex.RLock()
	defer ss.taskMutex.RUnlock()
	
	task, exists := ss.tasks[taskID]
	if !exists {
		return nil, fmt.Errorf("task not found: %s", taskID)
	}
	
	return task, nil
}

// ListTasks returns all sync tasks
func (ss *SyncScheduler) ListTasks() []*SyncTask {
	ss.taskMutex.RLock()
	defer ss.taskMutex.RUnlock()
	
	tasks := make([]*SyncTask, 0, len(ss.tasks))
	for _, task := range ss.tasks {
		tasks = append(tasks, task)
	}
	
	return tasks
}

// GetStatus returns scheduler status and statistics
func (ss *SyncScheduler) GetStatus() SchedulerStatus {
	ss.taskMutex.RLock()
	defer ss.taskMutex.RUnlock()
	
	status := SchedulerStatus{
		Running:     ss.running,
		TotalTasks:  len(ss.tasks),
		TaskCounts:  make(map[string]int),
	}
	
	for _, task := range ss.tasks {
		if task.Enabled {
			status.EnabledTasks++
		} else {
			status.DisabledTasks++
		}
		
		if task.LastResult != nil {
			switch task.LastResult.Status {
			case "success":
				status.SuccessfulTasks++
			case "failed":
				status.FailedTasks++
			case "running":
				status.RunningTasks++
			}
		}
	}
	
	return status
}

// SchedulerStatus represents the current status of the scheduler
type SchedulerStatus struct {
	Running          bool           `json:"running"`
	TotalTasks       int            `json:"totalTasks"`
	EnabledTasks     int            `json:"enabledTasks"`
	DisabledTasks    int            `json:"disabledTasks"`
	RunningTasks     int            `json:"runningTasks"`
	SuccessfulTasks  int            `json:"successfulTasks"`
	FailedTasks      int            `json:"failedTasks"`
	TaskCounts       map[string]int `json:"taskCounts"`
}

// Private methods

func (ss *SyncScheduler) loadTasks() error {
	// Load MCP configurations and create default sync tasks
	configs, err := ss.configProvider.GetAll(ss.ctx)
	if err != nil {
		return fmt.Errorf("failed to get MCP configurations: %w", err)
	}
	
	for _, cfg := range configs {
		// Check if a task already exists for this server
		taskExists := false
		for _, task := range ss.tasks {
			if task.Tenant == cfg.Tenant && task.ServerName == cfg.Name {
				taskExists = true
				break
			}
		}
		
		if !taskExists {
			// Create a default sync task
			task := &SyncTask{
				ID:         fmt.Sprintf("%s-%s", cfg.Tenant, cfg.Name),
				Tenant:     cfg.Tenant,
				ServerName: cfg.Name,
				SyncTypes:  []string{"tools", "prompts", "resources", "resource_templates"},
				Interval:   30 * time.Minute,
				NextRun:    time.Now().Add(5 * time.Minute), // Start after 5 minutes
				Enabled:    true,
				RetryPolicy: RetryPolicy{
					MaxRetries:     3,
					BaseDelay:      1 * time.Minute,
					MaxDelay:       10 * time.Minute,
					BackoffFactor:  2.0,
					RetryOnFailure: true,
				},
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
				Metadata: map[string]interface{}{
					"auto_created": true,
					"server_type":  cfg.McpServers[0].Type, // Assume first server
				},
			}
			
			ss.tasks[task.ID] = task
			ss.logger.Info("created default sync task",
				zap.String("tenant", cfg.Tenant),
				zap.String("server", cfg.Name))
		}
	}
	
	return nil
}

func (ss *SyncScheduler) schedulerLoop() {
	ticker := time.NewTicker(30 * time.Second) // Check every 30 seconds
	defer ticker.Stop()
	
	for {
		select {
		case <-ss.ctx.Done():
			ss.logger.Info("sync scheduler loop stopped")
			return
		case <-ticker.C:
			ss.checkAndExecuteTasks()
		}
	}
}

func (ss *SyncScheduler) checkAndExecuteTasks() {
	now := time.Now()
	
	ss.taskMutex.RLock()
	tasksToRun := make([]*SyncTask, 0)
	
	for _, task := range ss.tasks {
		if task.Enabled && !now.Before(task.NextRun) {
			tasksToRun = append(tasksToRun, task)
		}
	}
	ss.taskMutex.RUnlock()
	
	// Execute tasks
	for _, task := range tasksToRun {
		go ss.executeTask(task)
	}
}

func (ss *SyncScheduler) executeTask(task *SyncTask) {
	ss.logger.Info("executing sync task",
		zap.String("task_id", task.ID),
		zap.String("tenant", task.Tenant),
		zap.String("server", task.ServerName))
	
	startTime := time.Now()
	syncID := uuid.New().String()
	
	// Update task status
	ss.updateTaskExecution(task, "running", startTime, syncID, 0)
	
	// Get server configuration
	cfg, err := ss.configProvider.Get(ss.ctx, task.Tenant, task.ServerName)
	if err != nil {
		ss.handleTaskFailure(task, fmt.Sprintf("Failed to get server config: %v", err), startTime, syncID, 0)
		return
	}
	
	// Execute sync with retry logic
	ss.executeWithRetry(task, cfg, syncID, startTime, 0)
}

func (ss *SyncScheduler) executeWithRetry(task *SyncTask, cfg *config.MCPConfig, syncID string, startTime time.Time, attempt int) {
	// Perform the sync
	syncCtx, cancel := context.WithTimeout(ss.ctx, 10*time.Minute)
	defer cancel()
	
	// This is a placeholder - in real implementation, we would call the actual sync handler
	ss.syncHandler.PerformSync(syncCtx, cfg, syncID, task.SyncTypes, false)
	
	// Check sync result
	syncRecord, err := ss.capabilityStore.GetSyncRecord(syncCtx, syncID)
	if err != nil {
		ss.logger.Error("failed to get sync record",
			zap.String("sync_id", syncID),
			zap.Error(err))
		
		if attempt < task.RetryPolicy.MaxRetries && task.RetryPolicy.RetryOnFailure {
			ss.scheduleRetry(task, cfg, syncID, startTime, attempt+1, fmt.Sprintf("Failed to get sync record: %v", err))
		} else {
			ss.handleTaskFailure(task, fmt.Sprintf("Failed to get sync record: %v", err), startTime, syncID, attempt)
		}
		return
	}
	
	// Handle result based on status
	switch syncRecord.Status {
	case storage.SyncStatusSuccess:
		ss.handleTaskSuccess(task, syncRecord, startTime, syncID, attempt)
	case storage.SyncStatusPartial:
		ss.handleTaskPartialSuccess(task, syncRecord, startTime, syncID, attempt)
	case storage.SyncStatusFailed:
		if attempt < task.RetryPolicy.MaxRetries && task.RetryPolicy.RetryOnFailure {
			ss.scheduleRetry(task, cfg, syncID, startTime, attempt+1, syncRecord.ErrorMessage)
		} else {
			ss.handleTaskFailure(task, syncRecord.ErrorMessage, startTime, syncID, attempt)
		}
	default:
		// Still running, check later
		go func() {
			time.Sleep(1 * time.Minute)
			ss.executeWithRetry(task, cfg, syncID, startTime, attempt)
		}()
	}
}

func (ss *SyncScheduler) scheduleRetry(task *SyncTask, cfg *config.MCPConfig, syncID string, startTime time.Time, attempt int, errorMsg string) {
	delay := ss.calculateBackoffDelay(task.RetryPolicy, attempt)
	
	ss.logger.Warn("scheduling sync retry",
		zap.String("task_id", task.ID),
		zap.Int("attempt", attempt),
		zap.Duration("delay", delay),
		zap.String("error", errorMsg))
	
	go func() {
		time.Sleep(delay)
		newSyncID := uuid.New().String()
		ss.executeWithRetry(task, cfg, newSyncID, startTime, attempt)
	}()
}

func (ss *SyncScheduler) calculateBackoffDelay(policy RetryPolicy, attempt int) time.Duration {
	delay := time.Duration(float64(policy.BaseDelay) * math.Pow(policy.BackoffFactor, float64(attempt-1)))
	if delay > policy.MaxDelay {
		delay = policy.MaxDelay
	}
	return delay
}

func (ss *SyncScheduler) handleTaskSuccess(task *SyncTask, syncRecord *storage.SyncHistoryModel, startTime time.Time, syncID string, retryCount int) {
	endTime := time.Now()
	
	result := &SyncResult{
		Status:     "success",
		StartTime:  startTime,
		EndTime:    endTime,
		Duration:   endTime.Sub(startTime),
		RetryCount: retryCount,
		SyncID:     syncID,
	}
	
	if syncRecord.Summary != "" {
		var summary map[string]interface{}
		if err := json.Unmarshal([]byte(syncRecord.Summary), &summary); err == nil {
			result.Summary = summary
		}
	}
	
	ss.updateTaskCompletion(task, result)
	
	ss.logger.Info("sync task completed successfully",
		zap.String("task_id", task.ID),
		zap.Duration("duration", result.Duration),
		zap.Int("retry_count", retryCount))
}

func (ss *SyncScheduler) handleTaskPartialSuccess(task *SyncTask, syncRecord *storage.SyncHistoryModel, startTime time.Time, syncID string, retryCount int) {
	endTime := time.Now()
	
	result := &SyncResult{
		Status:     "partial",
		StartTime:  startTime,
		EndTime:    endTime,
		Duration:   endTime.Sub(startTime),
		Error:      syncRecord.ErrorMessage,
		RetryCount: retryCount,
		SyncID:     syncID,
	}
	
	if syncRecord.Summary != "" {
		var summary map[string]interface{}
		if err := json.Unmarshal([]byte(syncRecord.Summary), &summary); err == nil {
			result.Summary = summary
		}
	}
	
	ss.updateTaskCompletion(task, result)
	
	ss.logger.Warn("sync task completed with warnings",
		zap.String("task_id", task.ID),
		zap.Duration("duration", result.Duration),
		zap.String("error", syncRecord.ErrorMessage))
}

func (ss *SyncScheduler) handleTaskFailure(task *SyncTask, errorMsg string, startTime time.Time, syncID string, retryCount int) {
	endTime := time.Now()
	
	result := &SyncResult{
		Status:     "failed",
		StartTime:  startTime,
		EndTime:    endTime,
		Duration:   endTime.Sub(startTime),
		Error:      errorMsg,
		RetryCount: retryCount,
		SyncID:     syncID,
	}
	
	ss.updateTaskCompletion(task, result)
	
	ss.logger.Error("sync task failed",
		zap.String("task_id", task.ID),
		zap.Duration("duration", result.Duration),
		zap.String("error", errorMsg),
		zap.Int("retry_count", retryCount))
}

func (ss *SyncScheduler) updateTaskExecution(task *SyncTask, status string, startTime time.Time, syncID string, retryCount int) {
	ss.taskMutex.Lock()
	defer ss.taskMutex.Unlock()
	
	if status == "running" {
		task.LastResult = &SyncResult{
			Status:     status,
			StartTime:  startTime,
			RetryCount: retryCount,
			SyncID:     syncID,
		}
	}
}

func (ss *SyncScheduler) updateTaskCompletion(task *SyncTask, result *SyncResult) {
	ss.taskMutex.Lock()
	defer ss.taskMutex.Unlock()
	
	now := time.Now()
	task.LastRun = &now
	task.LastResult = result
	task.NextRun = now.Add(task.Interval)
	task.UpdatedAt = now
}