package storage

import (
	"testing"
	"time"

	"github.com/amoylab/unla/internal/common/cnst"
	"github.com/amoylab/unla/internal/common/config"
	"github.com/stretchr/testify/assert"
)

func sampleConfig() *config.MCPConfig {
	return &config.MCPConfig{
		Name:       "demo",
		Tenant:     "t1",
		Routers:    []config.RouterConfig{{Server: "s1", Prefix: "/p"}},
		Servers:    []config.ServerConfig{{Name: "s1"}},
		Tools:      []config.ToolConfig{{Name: "tool1", Method: "GET", Endpoint: "http://example.com"}},
		Prompts:    []config.PromptConfig{{Name: "pr1", Description: "d", Arguments: []config.PromptArgument{{Name: "a", Required: true}}}},
		McpServers: []config.MCPServerConfig{{Type: "sse", Name: "ms1"}},
	}
}

func TestMCPConfig_FromToModelAndHooks(t *testing.T) {
	cfg := sampleConfig()
	m, err := FromMCPConfig(cfg)
	assert.NoError(t, err)
	assert.Equal(t, cfg.Name, m.Name)
	assert.NotEmpty(t, m.Routers)
	assert.NotEmpty(t, m.McpServers)

	// hooks
	m.CreatedAt = time.Time{}
	m.UpdatedAt = time.Time{}
	assert.NoError(t, m.BeforeCreate(nil))
	assert.False(t, m.CreatedAt.IsZero())
	assert.False(t, m.UpdatedAt.IsZero())

	old := m.UpdatedAt
	time.Sleep(10 * time.Millisecond)
	assert.NoError(t, m.BeforeUpdate(nil))
	assert.True(t, m.UpdatedAt.After(old))

	back, err := m.ToMCPConfig()
	assert.NoError(t, err)
	assert.Equal(t, cfg.Name, back.Name)
	assert.Equal(t, cfg.Tenant, back.Tenant)
	assert.Len(t, back.Routers, 1)
	assert.Len(t, back.McpServers, 1)
}

func TestMCPConfigVersion_FromTo(t *testing.T) {
	cfg := sampleConfig()
	v, err := FromMCPConfigVersion(cfg, 1, "me", cnst.ActionCreate)
	assert.NoError(t, err)
	assert.Equal(t, 1, v.Version)
	assert.Equal(t, "me", v.CreatedBy)
	assert.Equal(t, cnst.ActionCreate, v.ActionType)
	assert.NotEmpty(t, v.Hash)

	// Map to config version DTO
	dto := v.ToConfigVersion()
	assert.Equal(t, v.Version, dto.Version)
	assert.Equal(t, v.Hash, dto.Hash)

	// Back to config content
	cfg2, err := v.ToMCPConfig()
	assert.NoError(t, err)
	assert.Equal(t, cfg.Name, cfg2.Name)
	assert.Len(t, cfg2.Tools, 1)
}
