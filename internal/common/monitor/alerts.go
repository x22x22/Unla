package monitor

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"go.uber.org/zap"
)

// LogAlertHandler logs alerts to the application logger
type LogAlertHandler struct {
	logger *zap.Logger
}

// NewLogAlertHandler creates a new log-based alert handler
func NewLogAlertHandler(logger *zap.Logger) *LogAlertHandler {
	return &LogAlertHandler{
		logger: logger.Named("alerts"),
	}
}

// HandleAlert handles an alert by logging it
func (h *LogAlertHandler) HandleAlert(ctx context.Context, alert Alert) error {
	fields := []zap.Field{
		zap.String("alert_id", alert.ID),
		zap.String("type", string(alert.Type)),
		zap.String("severity", string(alert.Severity)),
		zap.String("title", alert.Title),
		zap.Time("timestamp", alert.Timestamp),
	}

	if len(alert.Details) > 0 {
		if detailsJSON, err := json.Marshal(alert.Details); err == nil {
			fields = append(fields, zap.String("details", string(detailsJSON)))
		}
	}

	if len(alert.Actions) > 0 {
		fields = append(fields, zap.Strings("actions", alert.Actions))
	}

	switch alert.Severity {
	case AlertSeverityLow:
		h.logger.Info(alert.Message, fields...)
	case AlertSeverityMedium:
		h.logger.Warn(alert.Message, fields...)
	case AlertSeverityHigh, AlertSeverityCritical:
		h.logger.Error(alert.Message, fields...)
	}

	return nil
}

// WebhookAlertHandler sends alerts to webhook endpoints
type WebhookAlertHandler struct {
	webhookURL string
	client     *http.Client
	logger     *zap.Logger
}

// NewWebhookAlertHandler creates a new webhook alert handler
func NewWebhookAlertHandler(webhookURL string, logger *zap.Logger) *WebhookAlertHandler {
	return &WebhookAlertHandler{
		webhookURL: webhookURL,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
		logger: logger.Named("webhook_alerts"),
	}
}

// HandleAlert handles an alert by sending it to a webhook
func (h *WebhookAlertHandler) HandleAlert(ctx context.Context, alert Alert) error {
	payload := map[string]interface{}{
		"alert":     alert,
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"source":    "unla-monitoring",
	}

	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		h.logger.Error("Failed to marshal alert payload", zap.Error(err))
		return err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", h.webhookURL, strings.NewReader(string(jsonPayload)))
	if err != nil {
		h.logger.Error("Failed to create webhook request", zap.Error(err))
		return err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "Unla-Monitor/1.0")

	resp, err := h.client.Do(req)
	if err != nil {
		h.logger.Error("Failed to send webhook alert", zap.Error(err))
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		h.logger.Error("Webhook returned error status",
			zap.Int("status_code", resp.StatusCode),
			zap.String("alert_id", alert.ID))
		return fmt.Errorf("webhook returned status %d", resp.StatusCode)
	}

	h.logger.Info("Alert sent to webhook successfully",
		zap.String("alert_id", alert.ID),
		zap.String("webhook_url", h.webhookURL))

	return nil
}

// EmailAlertHandler sends alerts via email (placeholder implementation)
type EmailAlertHandler struct {
	smtpConfig SMTPConfig
	logger     *zap.Logger
}

// SMTPConfig represents SMTP configuration
type SMTPConfig struct {
	Host     string
	Port     int
	Username string
	Password string
	From     string
	To       []string
}

// NewEmailAlertHandler creates a new email alert handler
func NewEmailAlertHandler(config SMTPConfig, logger *zap.Logger) *EmailAlertHandler {
	return &EmailAlertHandler{
		smtpConfig: config,
		logger:     logger.Named("email_alerts"),
	}
}

// HandleAlert handles an alert by sending an email
func (h *EmailAlertHandler) HandleAlert(ctx context.Context, alert Alert) error {
	// This is a placeholder implementation
	// In a real system, you would use an SMTP client to send emails
	
	subject := fmt.Sprintf("[%s] %s - %s", 
		strings.ToUpper(string(alert.Severity)), 
		alert.Title, 
		alert.ID)
	
	body := h.formatEmailBody(alert)
	
	h.logger.Info("Email alert would be sent",
		zap.String("alert_id", alert.ID),
		zap.String("subject", subject),
		zap.String("body", body),
		zap.Strings("recipients", h.smtpConfig.To))
	
	// TODO: Implement actual email sending using SMTP
	// Example with net/smtp:
	// auth := smtp.PlainAuth("", h.smtpConfig.Username, h.smtpConfig.Password, h.smtpConfig.Host)
	// err := smtp.SendMail(fmt.Sprintf("%s:%d", h.smtpConfig.Host, h.smtpConfig.Port),
	//     auth, h.smtpConfig.From, h.smtpConfig.To, []byte(message))
	
	return nil
}

// formatEmailBody formats the alert as an email body
func (h *EmailAlertHandler) formatEmailBody(alert Alert) string {
	var body strings.Builder
	
	body.WriteString(fmt.Sprintf("Alert ID: %s\n", alert.ID))
	body.WriteString(fmt.Sprintf("Type: %s\n", alert.Type))
	body.WriteString(fmt.Sprintf("Severity: %s\n", alert.Severity))
	body.WriteString(fmt.Sprintf("Timestamp: %s\n", alert.Timestamp.Format(time.RFC3339)))
	body.WriteString(fmt.Sprintf("Title: %s\n", alert.Title))
	body.WriteString(fmt.Sprintf("Message: %s\n\n", alert.Message))
	
	if len(alert.Details) > 0 {
		body.WriteString("Details:\n")
		if detailsJSON, err := json.MarshalIndent(alert.Details, "", "  "); err == nil {
			body.WriteString(string(detailsJSON))
		}
		body.WriteString("\n\n")
	}
	
	if len(alert.Actions) > 0 {
		body.WriteString("Recommended Actions:\n")
		for _, action := range alert.Actions {
			body.WriteString(fmt.Sprintf("- %s\n", action))
		}
	}
	
	return body.String()
}

// SlackAlertHandler sends alerts to Slack (placeholder implementation)
type SlackAlertHandler struct {
	webhookURL string
	channel    string
	client     *http.Client
	logger     *zap.Logger
}

// NewSlackAlertHandler creates a new Slack alert handler
func NewSlackAlertHandler(webhookURL string, channel string, logger *zap.Logger) *SlackAlertHandler {
	return &SlackAlertHandler{
		webhookURL: webhookURL,
		channel:    channel,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
		logger: logger.Named("slack_alerts"),
	}
}

// HandleAlert handles an alert by sending it to Slack
func (h *SlackAlertHandler) HandleAlert(ctx context.Context, alert Alert) error {
	color := h.getSeverityColor(alert.Severity)
	
	payload := map[string]interface{}{
		"channel": h.channel,
		"attachments": []map[string]interface{}{
			{
				"color":     color,
				"title":     fmt.Sprintf("%s Alert: %s", strings.ToTitle(string(alert.Severity)), alert.Title),
				"text":      alert.Message,
				"timestamp": alert.Timestamp.Unix(),
				"fields": []map[string]interface{}{
					{
						"title": "Alert ID",
						"value": alert.ID,
						"short": true,
					},
					{
						"title": "Type",
						"value": string(alert.Type),
						"short": true,
					},
				},
			},
		},
	}

	// Add details as fields
	if len(alert.Details) > 0 {
		attachment := payload["attachments"].([]map[string]interface{})[0]
		fields := attachment["fields"].([]map[string]interface{})
		
		for key, value := range alert.Details {
			fields = append(fields, map[string]interface{}{
				"title": strings.Title(strings.ReplaceAll(key, "_", " ")),
				"value": fmt.Sprintf("%v", value),
				"short": true,
			})
		}
		
		attachment["fields"] = fields
	}

	// Add actions
	if len(alert.Actions) > 0 {
		attachment := payload["attachments"].([]map[string]interface{})[0]
		attachment["footer"] = fmt.Sprintf("Recommended actions: %s", strings.Join(alert.Actions, ", "))
	}

	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		h.logger.Error("Failed to marshal Slack payload", zap.Error(err))
		return err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", h.webhookURL, strings.NewReader(string(jsonPayload)))
	if err != nil {
		h.logger.Error("Failed to create Slack request", zap.Error(err))
		return err
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := h.client.Do(req)
	if err != nil {
		h.logger.Error("Failed to send Slack alert", zap.Error(err))
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		h.logger.Error("Slack webhook returned error status",
			zap.Int("status_code", resp.StatusCode),
			zap.String("alert_id", alert.ID))
		return fmt.Errorf("Slack webhook returned status %d", resp.StatusCode)
	}

	h.logger.Info("Alert sent to Slack successfully",
		zap.String("alert_id", alert.ID),
		zap.String("channel", h.channel))

	return nil
}

// getSeverityColor returns a color code for Slack based on alert severity
func (h *SlackAlertHandler) getSeverityColor(severity AlertSeverity) string {
	switch severity {
	case AlertSeverityLow:
		return "good" // Green
	case AlertSeverityMedium:
		return "warning" // Yellow
	case AlertSeverityHigh:
		return "danger" // Red
	case AlertSeverityCritical:
		return "#ff0000" // Bright red
	default:
		return "#808080" // Gray
	}
}

// CompositeAlertHandler combines multiple alert handlers
type CompositeAlertHandler struct {
	handlers []AlertHandler
	logger   *zap.Logger
}

// NewCompositeAlertHandler creates a new composite alert handler
func NewCompositeAlertHandler(handlers []AlertHandler, logger *zap.Logger) *CompositeAlertHandler {
	return &CompositeAlertHandler{
		handlers: handlers,
		logger:   logger.Named("composite_alerts"),
	}
}

// HandleAlert handles an alert by delegating to all configured handlers
func (h *CompositeAlertHandler) HandleAlert(ctx context.Context, alert Alert) error {
	var errors []string

	for i, handler := range h.handlers {
		if err := handler.HandleAlert(ctx, alert); err != nil {
			errorMsg := fmt.Sprintf("handler[%d]: %v", i, err)
			errors = append(errors, errorMsg)
			h.logger.Error("Alert handler failed",
				zap.Int("handler_index", i),
				zap.String("alert_id", alert.ID),
				zap.Error(err))
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("some alert handlers failed: %s", strings.Join(errors, "; "))
	}

	return nil
}