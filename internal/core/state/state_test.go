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
