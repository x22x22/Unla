package cnst

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestConfigConstants(t *testing.T) {
	assert.Equal(t, "apiserver.yaml", ApiServerYaml)
	assert.Equal(t, "mcp-gateway.yaml", MCPGatewayYaml)
}

func TestRedisClusterTypeConstants(t *testing.T) {
	assert.Equal(t, "sentinel", RedisClusterTypeSentinel)
	assert.Equal(t, "cluster", RedisClusterTypeCluster)
	assert.Equal(t, "single", RedisClusterTypeSingle)
}
