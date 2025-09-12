package dto

import (
	"testing"
	"time"

	"github.com/amoylab/unla/internal/common/config"
	"github.com/stretchr/testify/assert"
)

func TestFromConfigAndConverters(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	// Build a config with nested structures
	c := &config.MCPConfig{
		Name:      "cfg",
		Tenant:    "t",
		CreatedAt: now,
		UpdatedAt: now,
		Routers: []config.RouterConfig{{
			Server: "s1", Prefix: "/p", SSEPrefix: "/sse",
			CORS: &config.CORSConfig{AllowOrigins: []string{"*"}, AllowCredentials: true},
			Auth: &config.Auth{Mode: "none"},
		}},
		Servers: []config.ServerConfig{{
			Name: "s1", Description: "d1", AllowedTools: []string{"t1"},
			Config: map[string]string{"k": "v"},
		}},
		Tools: []config.ToolConfig{{
			Name: "t1", Description: "td", Method: "GET", Endpoint: "/e",
			Proxy:       &config.ProxyConfig{Host: "h", Port: 8080, Type: "http"},
			Headers:     map[string]string{"H": "V"},
			Args:        []config.ArgConfig{{Name: "q", Position: "query", Required: true, Type: "string", Description: "qd"}},
			RequestBody: "{}", ResponseBody: "{}",
			InputSchema: map[string]any{"x": "y"},
		}},
	}

	dtoSrv := FromConfig(c)
	assert.Equal(t, "cfg", dtoSrv.Name)
	assert.Equal(t, "t", dtoSrv.Tenant)
	assert.Equal(t, now, dtoSrv.CreatedAt)
	assert.Equal(t, now, dtoSrv.UpdatedAt)
	if assert.Len(t, dtoSrv.Routers, 1) {
		r := dtoSrv.Routers[0]
		assert.Equal(t, "s1", r.Server)
		assert.Equal(t, "/p", r.Prefix)
		assert.Equal(t, "/sse", r.SSEPrefix)
		if assert.NotNil(t, r.CORS) {
			assert.Equal(t, []string{"*"}, r.CORS.AllowOrigins)
			assert.True(t, r.CORS.AllowCredentials)
		}
	}
	if assert.Len(t, dtoSrv.Servers, 1) {
		s := dtoSrv.Servers[0]
		assert.Equal(t, "s1", s.Name)
		assert.Equal(t, "d1", s.Description)
		assert.Equal(t, []string{"t1"}, s.AllowedTools)
		assert.Equal(t, "v", s.Config["k"])
	}
	if assert.Len(t, dtoSrv.Tools, 1) {
		t0 := dtoSrv.Tools[0]
		assert.Equal(t, "t1", t0.Name)
		assert.Equal(t, "GET", t0.Method)
		assert.Equal(t, "/e", t0.Endpoint)
		if assert.NotNil(t, t0.Proxy) {
			assert.Equal(t, "h", t0.Proxy.Host)
			assert.Equal(t, 8080, t0.Proxy.Port)
			assert.Equal(t, "http", t0.Proxy.Type)
		}
		assert.Equal(t, map[string]string{"H": "V"}, t0.Headers)
		if assert.Len(t, t0.Args, 1) {
			assert.Equal(t, "q", t0.Args[0].Name)
		}
		assert.Equal(t, map[string]any{"x": "y"}, t0.InputSchema)
	}
}

func TestNilConverters(t *testing.T) {
	assert.Nil(t, FromRouterConfigs(nil))
	assert.Nil(t, FromCORSConfig(nil))
	assert.Nil(t, FromServerConfigs(nil))
	assert.Nil(t, FromToolConfigs(nil))
	assert.Nil(t, FromProxyConfig(nil))
}
