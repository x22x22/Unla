package state

import (
	"testing"

	"github.com/amoylab/unla/internal/common/cnst"
	"github.com/amoylab/unla/internal/common/config"
	"github.com/amoylab/unla/pkg/mcp"
	"github.com/stretchr/testify/assert"
)

func TestStateGetters(t *testing.T) {
	s := NewState()
	// Prepare runtime for prefix
	prefix := "/p"
	rt := runtimeUnit{
		protoType: cnst.BackendProtoHttp,
		router: &config.RouterConfig{
			Prefix:    prefix,
			SSEPrefix: "/sse",
			CORS:      &config.CORSConfig{AllowOrigins: []string{"*"}},
			Auth:      &config.Auth{},
		},
		server: &config.ServerConfig{Name: "srv", Config: map[string]string{"k": "v"}},
		tools: map[toolName]*config.ToolConfig{
			toolName("t1"): {Name: "t1"},
		},
		toolSchemas: []mcp.ToolSchema{{Name: "t1"}},
		prompts: map[promptName]*config.PromptConfig{
			promptName("p1"): {Name: "p1"},
		},
		promptSchemas: []mcp.PromptSchema{{Name: "p1"}},
	}
	s.runtime[uriPrefix(prefix)] = rt
	s.metrics.totalTools = 2
	s.metrics.missingTools = 1
	s.rawConfigs = []*config.MCPConfig{{Name: "cfg"}}

	assert.NotNil(t, s.GetCORS(prefix))
	assert.Equal(t, 1, s.GetRouterCount())
	assert.Equal(t, 2, s.GetToolCount())
	assert.Equal(t, 1, s.GetMissingToolCount())
	assert.Equal(t, 1, s.GetServerCount())
	assert.Equal(t, "t1", s.GetTool(prefix, "t1").Name)
	assert.Len(t, s.GetToolSchemas(prefix), 1)
	assert.Equal(t, "srv", s.GetServerConfig(prefix).Name)
	assert.Equal(t, cnst.BackendProtoHttp, s.GetProtoType(prefix))
	assert.Nil(t, s.GetTransport(prefix))
	assert.Empty(t, s.GetTransports())
	assert.Len(t, s.GetRawConfigs(), 1)
	assert.NotNil(t, s.GetAuth(prefix))
	assert.Equal(t, "/sse", s.GetSSEPrefix(prefix))
	assert.Equal(t, "p1", s.GetPrompt(prefix, "p1").Name)
	assert.Len(t, s.GetPromptSchemas(prefix), 1)
}

func TestNewState(t *testing.T) {
	s := NewState()
	assert.NotNil(t, s)
	assert.NotNil(t, s.rawConfigs)
	assert.NotNil(t, s.runtime)
	assert.Equal(t, 0, len(s.rawConfigs))
	assert.Equal(t, 0, len(s.runtime))
	assert.Equal(t, 0, s.metrics.totalTools)
	assert.Equal(t, 0, s.metrics.missingTools)
	assert.Equal(t, 0, s.metrics.httpServers)
	assert.Equal(t, 0, s.metrics.mcpServers)
}

func TestStateGetters_NonExistentPrefix(t *testing.T) {
	s := NewState()

	// Test all getters with non-existent prefix
	assert.Nil(t, s.GetCORS("nonexistent"))
	assert.Nil(t, s.GetTool("nonexistent", "tool"))
	assert.Nil(t, s.GetToolSchemas("nonexistent"))
	assert.Nil(t, s.GetServerConfig("nonexistent"))
	assert.Equal(t, cnst.ProtoType(""), s.GetProtoType("nonexistent"))
	assert.Nil(t, s.GetTransport("nonexistent"))
	assert.Nil(t, s.GetAuth("nonexistent"))
	assert.Equal(t, "", s.GetSSEPrefix("nonexistent"))
	assert.Nil(t, s.GetPrompt("nonexistent", "prompt"))
	assert.Nil(t, s.GetPromptSchemas("nonexistent"))
}

func TestStateCounters(t *testing.T) {
	s := NewState()

	// Test initial counters
	assert.Equal(t, 0, s.GetRouterCount())
	assert.Equal(t, 0, s.GetToolCount())
	assert.Equal(t, 0, s.GetMissingToolCount())
	assert.Equal(t, 0, s.GetServerCount())

	// Add some runtime units
	s.runtime[uriPrefix("/p1")] = runtimeUnit{
		router: &config.RouterConfig{Prefix: "/p1"},
		server: &config.ServerConfig{Name: "srv1"},
	}
	s.runtime[uriPrefix("/p2")] = runtimeUnit{
		router: &config.RouterConfig{Prefix: "/p2"},
		// No server
	}
	s.runtime[uriPrefix("/p3")] = runtimeUnit{
		// No router
		server: &config.ServerConfig{Name: "srv2"},
	}

	s.metrics.totalTools = 5
	s.metrics.missingTools = 2

	assert.Equal(t, 2, s.GetRouterCount()) // Only p1 and p2 have routers
	assert.Equal(t, 5, s.GetToolCount())
	assert.Equal(t, 2, s.GetMissingToolCount())
	assert.Equal(t, 2, s.GetServerCount()) // Only p1 and p3 have servers
}

func TestGetRuntimeMethod(t *testing.T) {
	s := NewState()

	// Test non-existent prefix
	runtime := s.getRuntime("nonexistent")
	assert.NotNil(t, runtime.tools)
	assert.NotNil(t, runtime.toolSchemas)
	assert.NotNil(t, runtime.prompts)
	assert.NotNil(t, runtime.promptSchemas)
	assert.Equal(t, 0, len(runtime.tools))
	assert.Equal(t, 0, len(runtime.toolSchemas))
	assert.Equal(t, 0, len(runtime.prompts))
	assert.Equal(t, 0, len(runtime.promptSchemas))

	// Test existing prefix
	existingRuntime := runtimeUnit{protoType: cnst.BackendProtoHttp}
	s.runtime[uriPrefix("/existing")] = existingRuntime
	runtime = s.getRuntime("/existing")
	assert.Equal(t, cnst.BackendProtoHttp, runtime.protoType)
}

func TestSetRouter(t *testing.T) {
	s := NewState()
	router := &config.RouterConfig{Prefix: "/test", Server: "srv"}

	s.setRouter("/test", router)

	// Verify router was set
	runtime, ok := s.runtime[uriPrefix("/test")]
	assert.True(t, ok)
	assert.Equal(t, router, runtime.router)
}
