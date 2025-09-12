package cnst

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestProtoTypeString(t *testing.T) {
	assert.Equal(t, "stdio", BackendProtoStdio.String())
	assert.Equal(t, "sse", BackendProtoSSE.String())
	assert.Equal(t, "streamable-http", BackendProtoStreamable.String())
	assert.Equal(t, "http", BackendProtoHttp.String())
	assert.Equal(t, "grpc", BackendProtoGrpc.String())
	assert.Equal(t, "sse", FrontendProtoSSE.String())
}
