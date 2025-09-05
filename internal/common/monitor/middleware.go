package monitor

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/amoylab/unla/internal/common/errorx"
)

// MonitoringMiddleware returns a gin middleware that integrates with SystemMonitor
func (m *SystemMonitor) MonitoringMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		
		// Increment active requests
		m.IncrementActiveRequests()
		defer m.DecrementActiveRequests()

		// Add trace ID if not present
		traceID := errorx.ExtractTraceID(c)
		c.Set("trace_id", traceID)

		// Process request
		c.Next()

		// Calculate duration
		duration := time.Since(start)

		// Record request metrics
		m.RecordRequest(c.Request.URL.Path, c.Request.Method, duration)

		// Record errors if any occurred
		if len(c.Errors) > 0 {
			for _, ginErr := range c.Errors {
				// Convert to APIError if not already
				var apiErr *errorx.APIError
				if err, ok := ginErr.Err.(*errorx.APIError); ok {
					apiErr = err
				} else {
					// Create APIError from gin error
					apiErr = &errorx.APIError{
						Code:       "E5001",
						Message:    ginErr.Error(),
						Category:   errorx.CategoryInternal,
						Severity:   errorx.SeverityError,
						HTTPStatus: c.Writer.Status(),
						TraceID:    traceID,
						Timestamp:  time.Now().UTC().Format(time.RFC3339),
					}
				}

				// Create context for error recording
				context := map[string]interface{}{
					"path":        c.Request.URL.Path,
					"method":      c.Request.Method,
					"status_code": c.Writer.Status(),
					"user_agent":  c.GetHeader("User-Agent"),
					"client_ip":   c.ClientIP(),
					"duration_ms": duration.Milliseconds(),
				}

				// Add user context if available
				if userID, exists := c.Get("user_id"); exists {
					context["user_id"] = userID
				}
				if tenant, exists := c.Get("tenant"); exists {
					context["tenant"] = tenant
				}

				// Record the error
				m.RecordError(apiErr, context)
			}
		}

		// Check for slow requests (configurable threshold)
		slowThreshold := 2 * time.Second
		if duration > slowThreshold {
			slowRequestErr := &errorx.APIError{
				Code:       "W5001", // Warning code for slow requests
				Message:    "Slow request detected",
				Category:   errorx.CategoryInternal,
				Severity:   errorx.SeverityWarning,
				HTTPStatus: c.Writer.Status(),
				TraceID:    traceID,
				Timestamp:  time.Now().UTC().Format(time.RFC3339),
				Details: map[string]interface{}{
					"duration_ms": duration.Milliseconds(),
					"threshold_ms": slowThreshold.Milliseconds(),
				},
			}

			context := map[string]interface{}{
				"path":        c.Request.URL.Path,
				"method":      c.Request.Method,
				"duration_ms": duration.Milliseconds(),
				"threshold_ms": slowThreshold.Milliseconds(),
			}

			m.RecordError(slowRequestErr, context)
		}
	}
}

// HealthCheckHandler returns a handler for health check that includes monitoring data
func (m *SystemMonitor) HealthCheckHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		errorStats := m.GetErrorStatistics()
		perfStats := m.GetPerformanceStatistics()

		health := gin.H{
			"status": "healthy",
			"timestamp": time.Now().UTC().Format(time.RFC3339),
			"monitoring": gin.H{
				"errors": gin.H{
					"total_errors":        getTotalErrors(errorStats.ErrorCounts),
					"errors_by_category":  errorStats.ErrorsByCategory,
					"errors_by_severity":  errorStats.ErrorsBySeverity,
					"recent_error_count":  len(errorStats.RecentErrors),
				},
				"performance": gin.H{
					"active_requests":    perfStats.ActiveRequests,
					"total_requests":     getTotalRequests(perfStats.RequestCounts),
					"memory_usage_mb":    float64(perfStats.SystemMetrics.MemoryUsed) / 1024 / 1024,
					"memory_percent":     perfStats.SystemMetrics.MemoryPercent,
					"goroutines":         perfStats.SystemMetrics.Goroutines,
				},
			},
		}

		// Check if system is actually healthy
		status := "healthy"
		httpStatus := 200

		// Check error rates
		if errorStats.ErrorsBySeverity[errorx.SeverityCritical] > 0 {
			status = "critical"
			httpStatus = 503
		} else if errorStats.ErrorsBySeverity[errorx.SeverityError] > 10 {
			status = "degraded"
			httpStatus = 200
		}

		// Check memory usage
		if perfStats.SystemMetrics.MemoryPercent > 90 {
			if status == "healthy" {
				status = "degraded"
			}
		}

		health["status"] = status
		c.JSON(httpStatus, health)
	}
}

// MetricsHandler returns detailed metrics for monitoring systems
func (m *SystemMonitor) MetricsHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		errorStats := m.GetErrorStatistics()
		perfStats := m.GetPerformanceStatistics()

		metrics := gin.H{
			"timestamp": time.Now().UTC().Format(time.RFC3339),
			"errors": gin.H{
				"counts":        errorStats.ErrorCounts,
				"by_category":   errorStats.ErrorsByCategory,
				"by_severity":   errorStats.ErrorsBySeverity,
				"recent_errors": errorStats.RecentErrors,
				"rates":         errorStats.ErrorRates,
				"last_reset":    errorStats.LastReset.Format(time.RFC3339),
			},
			"performance": gin.H{
				"request_counts":   perfStats.RequestCounts,
				"response_times":   perfStats.ResponseTimes,
				"active_requests":  perfStats.ActiveRequests,
				"system_metrics":   perfStats.SystemMetrics,
				"last_update":      perfStats.LastUpdate.Format(time.RFC3339),
			},
		}

		c.JSON(200, metrics)
	}
}

// AlertsHandler returns current alerts (this would integrate with alert storage)
func (m *SystemMonitor) AlertsHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		// In a real implementation, this would fetch alerts from storage
		// For now, return a sample structure
		alerts := gin.H{
			"timestamp": time.Now().UTC().Format(time.RFC3339),
			"alerts": []gin.H{
				// Sample alert structure
			},
			"count": 0,
		}

		c.JSON(200, alerts)
	}
}

// Helper functions

func getTotalErrors(errorCounts map[string]int64) int64 {
	var total int64
	for _, count := range errorCounts {
		total += count
	}
	return total
}

func getTotalRequests(requestCounts map[string]int64) int64 {
	var total int64
	for _, count := range requestCounts {
		total += count
	}
	return total
}