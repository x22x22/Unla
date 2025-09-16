package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/amoylab/unla/internal/common/config"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestRuntimeConfigHandler_HandleRuntimeConfig(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cfg := &config.APIServerConfig{
		Web: config.WebConfig{
			APIBaseURL:               "http://api.example",
			WSBaseURL:                "ws://ws.example",
			MCPGatewayBaseURL:        "http://mcp.example",
			DirectMCPGatewayModifier: "auth",
			BaseURL:                  "http://web.example",
			DebugMode:                true,
			EnableExperimental:       true,
			LLMConfigAdminOnly:       true,
		},
	}

	h := NewRuntimeConfigHandler(cfg)

	r := gin.New()
	r.GET("/runtime", h.HandleRuntimeConfig)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/runtime", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	body := w.Body.String()
	// Check a few key fields exist and match
	assert.Contains(t, body, "\"VITE_API_BASE_URL\":\"http://api.example\"")
	assert.Contains(t, body, "\"VITE_WS_BASE_URL\":\"ws://ws.example\"")
	assert.Contains(t, body, "\"VITE_MCP_GATEWAY_BASE_URL\":\"http://mcp.example\"")
	assert.Contains(t, body, "\"directMcpGatewayModifier\":\"auth\"")
	assert.Contains(t, body, "\"apiBaseUrl\":\"http://api.example\"")
	assert.Contains(t, body, "\"debugMode\":true")
	assert.Contains(t, body, "\"LLM_CONFIG_ADMIN_ONLY\":true")
}
