package monitor

import (
	"context"
	"encoding/json"
	"fmt"
	"runtime"
	"sync"
	"time"

	"github.com/amoylab/unla/internal/common/errorx"
	"go.uber.org/zap"
)

// SystemMonitor provides global system monitoring and error tracking
type SystemMonitor struct {
	logger           *zap.Logger
	errorStats       *ErrorStatistics
	performanceStats *PerformanceStatistics
	alertHandlers    []AlertHandler
	mu               sync.RWMutex
	ctx              context.Context
	cancel           context.CancelFunc
}

// ErrorStatistics tracks error occurrences and patterns
type ErrorStatistics struct {
	mu                sync.RWMutex
	ErrorCounts       map[string]int64                    `json:"error_counts"`
	ErrorsByCategory  map[errorx.ErrorCategory]int64      `json:"errors_by_category"`
	ErrorsBySeverity  map[errorx.Severity]int64           `json:"errors_by_severity"`
	RecentErrors      []ErrorEvent                        `json:"recent_errors"`
	ErrorRates        map[string]*RateCounter             `json:"-"`
	LastReset         time.Time                           `json:"last_reset"`
	AlertThresholds   map[string]int64                    `json:"alert_thresholds"`
}

// PerformanceStatistics tracks system performance metrics
type PerformanceStatistics struct {
	mu              sync.RWMutex
	RequestCounts   map[string]int64        `json:"request_counts"`
	ResponseTimes   map[string]*TimeMetrics `json:"response_times"`
	ActiveRequests  int64                   `json:"active_requests"`
	SystemMetrics   *SystemMetrics          `json:"system_metrics"`
	LastUpdate      time.Time               `json:"last_update"`
}

// ErrorEvent represents a single error occurrence
type ErrorEvent struct {
	Timestamp   time.Time              `json:"timestamp"`
	Code        string                 `json:"code"`
	Message     string                 `json:"message"`
	Category    errorx.ErrorCategory   `json:"category"`
	Severity    errorx.Severity        `json:"severity"`
	TraceID     string                 `json:"trace_id"`
	Path        string                 `json:"path,omitempty"`
	Method      string                 `json:"method,omitempty"`
	UserAgent   string                 `json:"user_agent,omitempty"`
	ClientIP    string                 `json:"client_ip,omitempty"`
	Details     map[string]interface{} `json:"details,omitempty"`
	StackTrace  string                 `json:"stack_trace,omitempty"`
}

// RateCounter tracks rate of occurrences over time
type RateCounter struct {
	Count       int64     `json:"count"`
	WindowStart time.Time `json:"window_start"`
	WindowSize  time.Duration `json:"window_size"`
}

// TimeMetrics tracks timing statistics
type TimeMetrics struct {
	Count    int64         `json:"count"`
	Total    time.Duration `json:"total"`
	Min      time.Duration `json:"min"`
	Max      time.Duration `json:"max"`
	Average  time.Duration `json:"average"`
	LastUpdated time.Time  `json:"last_updated"`
}

// SystemMetrics tracks system resource usage
type SystemMetrics struct {
	CPUPercent    float64   `json:"cpu_percent"`
	MemoryUsed    uint64    `json:"memory_used"`
	MemoryTotal   uint64    `json:"memory_total"`
	MemoryPercent float64   `json:"memory_percent"`
	Goroutines    int       `json:"goroutines"`
	Timestamp     time.Time `json:"timestamp"`
}

// AlertHandler defines the interface for alert handlers
type AlertHandler interface {
	HandleAlert(ctx context.Context, alert Alert) error
}

// Alert represents a system alert
type Alert struct {
	ID          string                 `json:"id"`
	Type        AlertType              `json:"type"`
	Severity    AlertSeverity          `json:"severity"`
	Title       string                 `json:"title"`
	Message     string                 `json:"message"`
	Timestamp   time.Time              `json:"timestamp"`
	Details     map[string]interface{} `json:"details,omitempty"`
	Actions     []string               `json:"actions,omitempty"`
}

// AlertType defines the type of alert
type AlertType string

const (
	AlertTypeError       AlertType = "error"
	AlertTypePerformance AlertType = "performance"
	AlertTypeSystem      AlertType = "system"
	AlertTypeSecurity    AlertType = "security"
)

// AlertSeverity defines the severity of alert
type AlertSeverity string

const (
	AlertSeverityLow      AlertSeverity = "low"
	AlertSeverityMedium   AlertSeverity = "medium"
	AlertSeverityHigh     AlertSeverity = "high"
	AlertSeverityCritical AlertSeverity = "critical"
)

// NewSystemMonitor creates a new system monitor
func NewSystemMonitor(logger *zap.Logger) *SystemMonitor {
	ctx, cancel := context.WithCancel(context.Background())
	
	monitor := &SystemMonitor{
		logger:           logger.Named("monitor"),
		ctx:              ctx,
		cancel:           cancel,
		errorStats:       newErrorStatistics(),
		performanceStats: newPerformanceStatistics(),
		alertHandlers:    make([]AlertHandler, 0),
	}

	// Start background monitoring
	go monitor.startBackgroundMonitoring()

	return monitor
}

// newErrorStatistics creates new error statistics
func newErrorStatistics() *ErrorStatistics {
	return &ErrorStatistics{
		ErrorCounts:       make(map[string]int64),
		ErrorsByCategory:  make(map[errorx.ErrorCategory]int64),
		ErrorsBySeverity:  make(map[errorx.Severity]int64),
		RecentErrors:      make([]ErrorEvent, 0),
		ErrorRates:        make(map[string]*RateCounter),
		LastReset:         time.Now(),
		AlertThresholds:   getDefaultAlertThresholds(),
	}
}

// newPerformanceStatistics creates new performance statistics
func newPerformanceStatistics() *PerformanceStatistics {
	return &PerformanceStatistics{
		RequestCounts:  make(map[string]int64),
		ResponseTimes:  make(map[string]*TimeMetrics),
		SystemMetrics:  &SystemMetrics{},
		LastUpdate:     time.Now(),
	}
}

// getDefaultAlertThresholds returns default alert thresholds
func getDefaultAlertThresholds() map[string]int64 {
	return map[string]int64{
		"error_rate_per_minute":    100,
		"critical_errors_per_hour": 10,
		"database_errors_per_hour": 50,
		"network_errors_per_hour":  30,
	}
}

// RecordError records an error event
func (m *SystemMonitor) RecordError(err *errorx.APIError, context map[string]interface{}) {
	if err == nil {
		return
	}

	event := ErrorEvent{
		Timestamp: time.Now(),
		Code:      err.Code,
		Message:   err.Message,
		Category:  err.Category,
		Severity:  err.Severity,
		TraceID:   err.TraceID,
		Details:   err.Details,
	}

	// Add context information
	if context != nil {
		if event.Details == nil {
			event.Details = make(map[string]interface{})
		}
		for k, v := range context {
			event.Details[k] = v
		}

		// Extract common fields
		if path, ok := context["path"].(string); ok {
			event.Path = path
		}
		if method, ok := context["method"].(string); ok {
			event.Method = method
		}
		if userAgent, ok := context["user_agent"].(string); ok {
			event.UserAgent = userAgent
		}
		if clientIP, ok := context["client_ip"].(string); ok {
			event.ClientIP = clientIP
		}
	}

	// Add stack trace for critical errors
	if err.Severity == errorx.SeverityCritical {
		buf := make([]byte, 1024*4)
		n := runtime.Stack(buf, false)
		event.StackTrace = string(buf[:n])
	}

	// Update statistics
	m.updateErrorStats(event)

	// Check for alerts
	m.checkErrorAlerts(event)

	// Log the error
	m.logErrorEvent(event)
}

// updateErrorStats updates error statistics
func (m *SystemMonitor) updateErrorStats(event ErrorEvent) {
	m.errorStats.mu.Lock()
	defer m.errorStats.mu.Unlock()

	// Update counters
	m.errorStats.ErrorCounts[event.Code]++
	m.errorStats.ErrorsByCategory[event.Category]++
	m.errorStats.ErrorsBySeverity[event.Severity]++

	// Update rate counters
	m.updateRateCounter("error_rate_per_minute", time.Minute)
	m.updateRateCounter("critical_errors_per_hour", time.Hour)

	// Add to recent errors (keep last 100)
	m.errorStats.RecentErrors = append(m.errorStats.RecentErrors, event)
	if len(m.errorStats.RecentErrors) > 100 {
		m.errorStats.RecentErrors = m.errorStats.RecentErrors[1:]
	}
}

// updateRateCounter updates a rate counter
func (m *SystemMonitor) updateRateCounter(key string, window time.Duration) {
	now := time.Now()
	
	if counter, exists := m.errorStats.ErrorRates[key]; exists {
		if now.Sub(counter.WindowStart) > window {
			// Reset window
			counter.Count = 1
			counter.WindowStart = now
		} else {
			counter.Count++
		}
	} else {
		m.errorStats.ErrorRates[key] = &RateCounter{
			Count:       1,
			WindowStart: now,
			WindowSize:  window,
		}
	}
}

// RecordRequest records a request for performance monitoring
func (m *SystemMonitor) RecordRequest(path string, method string, duration time.Duration) {
	key := fmt.Sprintf("%s:%s", method, path)
	
	m.performanceStats.mu.Lock()
	defer m.performanceStats.mu.Unlock()

	// Update request count
	m.performanceStats.RequestCounts[key]++

	// Update timing metrics
	if metrics, exists := m.performanceStats.ResponseTimes[key]; exists {
		metrics.Count++
		metrics.Total += duration
		if duration < metrics.Min || metrics.Min == 0 {
			metrics.Min = duration
		}
		if duration > metrics.Max {
			metrics.Max = duration
		}
		metrics.Average = time.Duration(int64(metrics.Total) / metrics.Count)
		metrics.LastUpdated = time.Now()
	} else {
		m.performanceStats.ResponseTimes[key] = &TimeMetrics{
			Count:       1,
			Total:       duration,
			Min:         duration,
			Max:         duration,
			Average:     duration,
			LastUpdated: time.Now(),
		}
	}

	m.performanceStats.LastUpdate = time.Now()
}

// IncrementActiveRequests increments active request counter
func (m *SystemMonitor) IncrementActiveRequests() {
	m.performanceStats.mu.Lock()
	defer m.performanceStats.mu.Unlock()
	m.performanceStats.ActiveRequests++
}

// DecrementActiveRequests decrements active request counter
func (m *SystemMonitor) DecrementActiveRequests() {
	m.performanceStats.mu.Lock()
	defer m.performanceStats.mu.Unlock()
	if m.performanceStats.ActiveRequests > 0 {
		m.performanceStats.ActiveRequests--
	}
}

// GetErrorStatistics returns current error statistics
func (m *SystemMonitor) GetErrorStatistics() *ErrorStatistics {
	m.errorStats.mu.RLock()
	defer m.errorStats.mu.RUnlock()

	// Create a deep copy
	stats := &ErrorStatistics{
		ErrorCounts:      make(map[string]int64),
		ErrorsByCategory: make(map[errorx.ErrorCategory]int64),
		ErrorsBySeverity: make(map[errorx.Severity]int64),
		RecentErrors:     make([]ErrorEvent, len(m.errorStats.RecentErrors)),
		LastReset:        m.errorStats.LastReset,
		AlertThresholds:  make(map[string]int64),
	}

	for k, v := range m.errorStats.ErrorCounts {
		stats.ErrorCounts[k] = v
	}
	for k, v := range m.errorStats.ErrorsByCategory {
		stats.ErrorsByCategory[k] = v
	}
	for k, v := range m.errorStats.ErrorsBySeverity {
		stats.ErrorsBySeverity[k] = v
	}
	for k, v := range m.errorStats.AlertThresholds {
		stats.AlertThresholds[k] = v
	}
	copy(stats.RecentErrors, m.errorStats.RecentErrors)

	return stats
}

// GetPerformanceStatistics returns current performance statistics
func (m *SystemMonitor) GetPerformanceStatistics() *PerformanceStatistics {
	m.performanceStats.mu.RLock()
	defer m.performanceStats.mu.RUnlock()

	// Update system metrics before returning
	m.updateSystemMetrics()

	// Create a deep copy
	stats := &PerformanceStatistics{
		RequestCounts:  make(map[string]int64),
		ResponseTimes:  make(map[string]*TimeMetrics),
		ActiveRequests: m.performanceStats.ActiveRequests,
		SystemMetrics:  m.performanceStats.SystemMetrics,
		LastUpdate:     m.performanceStats.LastUpdate,
	}

	for k, v := range m.performanceStats.RequestCounts {
		stats.RequestCounts[k] = v
	}
	for k, v := range m.performanceStats.ResponseTimes {
		stats.ResponseTimes[k] = &TimeMetrics{
			Count:       v.Count,
			Total:       v.Total,
			Min:         v.Min,
			Max:         v.Max,
			Average:     v.Average,
			LastUpdated: v.LastUpdated,
		}
	}

	return stats
}

// updateSystemMetrics updates system resource metrics
func (m *SystemMonitor) updateSystemMetrics() {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	m.performanceStats.SystemMetrics = &SystemMetrics{
		MemoryUsed:    memStats.Alloc,
		MemoryTotal:   memStats.Sys,
		MemoryPercent: float64(memStats.Alloc) / float64(memStats.Sys) * 100,
		Goroutines:    runtime.NumGoroutine(),
		Timestamp:     time.Now(),
	}
}

// AddAlertHandler adds an alert handler
func (m *SystemMonitor) AddAlertHandler(handler AlertHandler) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.alertHandlers = append(m.alertHandlers, handler)
}

// checkErrorAlerts checks if error thresholds are exceeded and triggers alerts
func (m *SystemMonitor) checkErrorAlerts(event ErrorEvent) {
	// Check error rate alerts
	if counter, exists := m.errorStats.ErrorRates["error_rate_per_minute"]; exists {
		threshold := m.errorStats.AlertThresholds["error_rate_per_minute"]
		if counter.Count >= threshold {
			alert := Alert{
				ID:        fmt.Sprintf("error_rate_%d", time.Now().Unix()),
				Type:      AlertTypeError,
				Severity:  AlertSeverityHigh,
				Title:     "High Error Rate Detected",
				Message:   fmt.Sprintf("Error rate of %d errors per minute exceeds threshold of %d", counter.Count, threshold),
				Timestamp: time.Now(),
				Details: map[string]interface{}{
					"error_count": counter.Count,
					"threshold":   threshold,
					"window":      "1 minute",
				},
				Actions: []string{"check_logs", "scale_up", "investigate"},
			}
			m.triggerAlert(alert)
		}
	}

	// Check critical error alerts
	if event.Severity == errorx.SeverityCritical {
		alert := Alert{
			ID:        fmt.Sprintf("critical_error_%s_%d", event.Code, time.Now().Unix()),
			Type:      AlertTypeError,
			Severity:  AlertSeverityCritical,
			Title:     "Critical Error Occurred",
			Message:   fmt.Sprintf("Critical error %s: %s", event.Code, event.Message),
			Timestamp: time.Now(),
			Details: map[string]interface{}{
				"error_code": event.Code,
				"trace_id":   event.TraceID,
				"path":       event.Path,
				"method":     event.Method,
			},
			Actions: []string{"immediate_investigation", "contact_on_call", "check_dependencies"},
		}
		m.triggerAlert(alert)
	}
}

// triggerAlert triggers an alert through all registered handlers
func (m *SystemMonitor) triggerAlert(alert Alert) {
	m.mu.RLock()
	handlers := make([]AlertHandler, len(m.alertHandlers))
	copy(handlers, m.alertHandlers)
	m.mu.RUnlock()

	for _, handler := range handlers {
		go func(h AlertHandler) {
			if err := h.HandleAlert(m.ctx, alert); err != nil {
				m.logger.Error("Failed to handle alert",
					zap.String("alert_id", alert.ID),
					zap.Error(err))
			}
		}(handler)
	}

	// Log the alert
	m.logger.Warn("Alert triggered",
		zap.String("alert_id", alert.ID),
		zap.String("type", string(alert.Type)),
		zap.String("severity", string(alert.Severity)),
		zap.String("title", alert.Title))
}

// logErrorEvent logs an error event
func (m *SystemMonitor) logErrorEvent(event ErrorEvent) {
	fields := []zap.Field{
		zap.String("error_code", event.Code),
		zap.String("category", string(event.Category)),
		zap.String("severity", string(event.Severity)),
		zap.String("trace_id", event.TraceID),
	}

	if event.Path != "" {
		fields = append(fields, zap.String("path", event.Path))
	}
	if event.Method != "" {
		fields = append(fields, zap.String("method", event.Method))
	}
	if event.ClientIP != "" {
		fields = append(fields, zap.String("client_ip", event.ClientIP))
	}
	if len(event.Details) > 0 {
		if detailsJSON, err := json.Marshal(event.Details); err == nil {
			fields = append(fields, zap.String("details", string(detailsJSON)))
		}
	}
	if event.StackTrace != "" {
		fields = append(fields, zap.String("stack_trace", event.StackTrace))
	}

	switch event.Severity {
	case errorx.SeverityInfo:
		m.logger.Info(event.Message, fields...)
	case errorx.SeverityWarning:
		m.logger.Warn(event.Message, fields...)
	case errorx.SeverityError:
		m.logger.Error(event.Message, fields...)
	case errorx.SeverityCritical:
		m.logger.Error(event.Message, fields...)
	default:
		m.logger.Error(event.Message, fields...)
	}
}

// startBackgroundMonitoring starts background monitoring tasks
func (m *SystemMonitor) startBackgroundMonitoring() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			m.performanceStats.mu.Lock()
			m.updateSystemMetrics()
			m.performanceStats.mu.Unlock()

		case <-m.ctx.Done():
			return
		}
	}
}

// ResetStatistics resets all statistics
func (m *SystemMonitor) ResetStatistics() {
	m.errorStats.mu.Lock()
	m.performanceStats.mu.Lock()
	defer m.errorStats.mu.Unlock()
	defer m.performanceStats.mu.Unlock()

	m.errorStats = newErrorStatistics()
	m.performanceStats = newPerformanceStatistics()
}

// Close stops the monitor and releases resources
func (m *SystemMonitor) Close() error {
	m.cancel()
	return nil
}