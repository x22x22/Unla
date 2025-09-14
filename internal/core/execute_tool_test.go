package core

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/amoylab/unla/internal/common/config"
	"github.com/amoylab/unla/internal/mcp/session"
	"github.com/amoylab/unla/pkg/mcp"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

type fakeConnExec struct{ meta *session.Meta }

func (f *fakeConnExec) EventQueue() <-chan *session.Message                  { return nil }
func (f *fakeConnExec) Send(ctx context.Context, msg *session.Message) error { return nil }
func (f *fakeConnExec) Close(ctx context.Context) error                      { return nil }
func (f *fakeConnExec) Meta() *session.Meta                                  { return f.meta }

func TestExecuteHTTPTool_Success(t *testing.T) {
	// downstream returns JSON
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"hello":"world"}`))
	}))
	defer srv.Close()

	s := &Server{logger: zap.NewNop(), toolRespHandler: CreateResponseHandlerChain()}
	tool := &config.ToolConfig{
		Name:         "t",
		Method:       http.MethodGet,
		Endpoint:     srv.URL,
		ResponseBody: "{{.Response.Body}}",
	}
	req, _ := http.NewRequest(http.MethodGet, srv.URL, nil)
	conn := &fakeConnExec{meta: &session.Meta{ID: "sid", Request: &session.RequestInfo{Headers: map[string]string{"X-Req": "v"}}}}
	res, err := s.executeHTTPTool(conn, tool, map[string]any{}, req, map[string]string{})
	assert.NoError(t, err)
	if assert.NotNil(t, res) {
		if tc, ok := res.Content[0].(*mcp.TextContent); ok {
			assert.Equal(t, `{"hello":"world"}`, tc.Text)
		} else {
			t.Fatalf("unexpected content type")
		}
	}
}

func TestExecuteHTTPTool_ForwardHeadersAndRequestError(t *testing.T) {
	s := &Server{logger: zap.NewNop(), toolRespHandler: CreateResponseHandlerChain(), forwardConfig: config.ForwardConfig{Enabled: true}}
	s.forwardConfig.McpArg.KeyForHeader = "_hdr"
	tool := &config.ToolConfig{
		Name:         "t",
		Method:       http.MethodGet,
		Endpoint:     "http://127.0.0.1:0", // invalid port triggers dial error
		ResponseBody: "{{.Response.Body}}",
	}
	req, _ := http.NewRequest(http.MethodGet, "http://example", nil)
	conn := &fakeConnExec{meta: &session.Meta{ID: "sid", Request: &session.RequestInfo{Headers: map[string]string{}}}}

	args := map[string]any{
		"_hdr": map[string]any{"X-A": "B"},
	}

	res, err := s.executeHTTPTool(conn, tool, args, req, map[string]string{})
	assert.Error(t, err)
	assert.Nil(t, res)
}
