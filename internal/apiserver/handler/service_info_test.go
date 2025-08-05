package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/amoylab/unla/pkg/version"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestServiceInfoHandler_HandleServiceInfo(t *testing.T) {
	// Set gin to test mode
	gin.SetMode(gin.TestMode)

	// Create a new service info handler
	handler := NewServiceInfoHandler()

	// Create a new gin router and register the endpoint
	router := gin.New()
	router.GET("/api/info", handler.HandleServiceInfo)

	// Create a test request
	req, err := http.NewRequest("GET", "/api/info", nil)
	assert.NoError(t, err)

	// Create a response recorder
	w := httptest.NewRecorder()

	// Perform the request
	router.ServeHTTP(w, req)

	// Assert the response status code
	assert.Equal(t, http.StatusOK, w.Code)

	// Assert the response content type
	assert.Equal(t, "application/json; charset=utf-8", w.Header().Get("Content-Type"))

	// Parse the response body
	var response ServiceInfo
	err = json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)

	// Assert the response content
	assert.Equal(t, "Unla", response.Name)
	assert.Equal(t, "MCP Gateway - A lightweight and highly available gateway service that converts existing MCP Servers and APIs into MCP Protocol compliant services", response.Description)
	assert.Equal(t, version.Get(), response.Version)
	assert.Equal(t, "mcp-gateway", response.Type)
	
	// Assert capabilities are present
	expectedCapabilities := []string{
		"mcp-proxy",
		"api-conversion",
		"sse-transport",
		"http-transport",
		"multi-tenant",
		"session-management",
		"authentication",
		"configuration-management",
	}
	assert.Equal(t, expectedCapabilities, response.Capabilities)
}