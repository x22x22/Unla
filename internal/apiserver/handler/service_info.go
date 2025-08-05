package handler

import (
	"net/http"

	"github.com/amoylab/unla/pkg/version"
	"github.com/gin-gonic/gin"
)

// ServiceInfoHandler represents the service information handler
type ServiceInfoHandler struct{}

// NewServiceInfoHandler creates a new service information handler
func NewServiceInfoHandler() *ServiceInfoHandler {
	return &ServiceInfoHandler{}
}

// ServiceInfo represents the service identity information
type ServiceInfo struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Version     string `json:"version"`
	Type        string `json:"type"`
	Capabilities []string `json:"capabilities"`
}

// HandleServiceInfo serves service identity information as JSON
func (h *ServiceInfoHandler) HandleServiceInfo(c *gin.Context) {
	info := ServiceInfo{
		Name:        "Unla",
		Description: "MCP Gateway - A lightweight and highly available gateway service that converts existing MCP Servers and APIs into MCP Protocol compliant services",
		Version:     version.Get(),
		Type:        "mcp-gateway",
		Capabilities: []string{
			"mcp-proxy",
			"api-conversion",
			"sse-transport",
			"http-transport",
			"multi-tenant",
			"session-management",
			"authentication",
			"configuration-management",
		},
	}

	c.JSON(http.StatusOK, info)
}