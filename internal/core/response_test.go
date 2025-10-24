package core

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/amoylab/unla/internal/mcp/session"
	"github.com/amoylab/unla/pkg/mcp"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

type fakeConn struct {
	meta    *session.Meta
	sent    []*session.Message
	sendErr error
}

func (f *fakeConn) EventQueue() <-chan *session.Message { return nil }
func (f *fakeConn) Send(ctx context.Context, msg *session.Message) error {
	f.sent = append(f.sent, msg)
	return f.sendErr
}
func (f *fakeConn) Close(ctx context.Context) error { return nil }
func (f *fakeConn) Meta() *session.Meta             { return f.meta }

func newGin() (*gin.Context, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	c.Request = req
	return c, w
}

func TestSendProtocolError(t *testing.T) {
	s := &Server{logger: zap.NewNop()}
	c, w := newGin()
	s.sendProtocolError(c, 1, "Bad", http.StatusBadRequest, mcp.ErrorCodeInvalidRequest)
	assert.Equal(t, http.StatusBadRequest, w.Code)
	var body mcp.JSONRPCErrorSchema
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, mcp.JSPNRPCVersion, body.JSONRPC)
	assert.Equal(t, 1.0, body.ID) // gin/json encodes numeric as float64 when decoding to interface{}
	assert.Equal(t, mcp.ErrorCodeInvalidRequest, body.Error.Code)
}

func TestSendSuccessResponse_HTTP(t *testing.T) {
	s := &Server{logger: zap.NewNop()}
	c, w := newGin()
	conn := &fakeConn{meta: &session.Meta{ID: "sid"}}

	req := mcp.JSONRPCRequest{Id: 2, Method: "tools/call", JSONRPC: mcp.JSPNRPCVersion}
	s.sendSuccessResponse(c, conn, req, mcp.NewCallToolResultText("ok"), false)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "text/event-stream", w.Result().Header.Get("Content-Type"))
	assert.Equal(t, "sid", w.Result().Header.Get(mcp.HeaderMcpSessionID))
	assert.Contains(t, w.Body.String(), "event: message")
	assert.Contains(t, w.Body.String(), "\ndata: {")
}

func TestSendSuccessResponse_SSE(t *testing.T) {
	s := &Server{logger: zap.NewNop()}
	c, w := newGin()
	conn := &fakeConn{meta: &session.Meta{ID: "sid"}}
	req := mcp.JSONRPCRequest{Id: 3, Method: "tools/call", JSONRPC: mcp.JSPNRPCVersion}
	s.sendSuccessResponse(c, conn, req, mcp.NewCallToolResultText("ok"), true)
	assert.Equal(t, http.StatusAccepted, w.Code)
	if assert.Len(t, conn.sent, 1) {
		assert.Equal(t, "message", conn.sent[0].Event)
		// payload is JSON, sanity check
		var tmp any
		assert.NoError(t, json.Unmarshal(conn.sent[0].Data, &tmp))
	}
}

func TestSendResponseMarshalError_HTTP(t *testing.T) {
	s := &Server{logger: zap.NewNop()}
	c, w := newGin()
	conn := &fakeConn{meta: &session.Meta{ID: "sid"}}

	// Channel cannot be marshaled by encoding/json
	type bad struct{ C chan int }
	s.sendResponse(c, 4, conn, bad{C: make(chan int)}, false)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestSendResponseSSESendError(t *testing.T) {
	s := &Server{logger: zap.NewNop()}
	c, w := newGin()
	conn := &fakeConn{meta: &session.Meta{ID: "sid"}, sendErr: errors.New("boom")}
	s.sendResponse(c, 5, conn, mcp.NewCallToolResultText("ok"), true)
	// Should convert to protocol error
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestSendToolExecutionError_HTTP(t *testing.T) {
	s := &Server{logger: zap.NewNop()}
	c, w := newGin()
	conn := &fakeConn{meta: &session.Meta{ID: "sid"}}
	req := mcp.JSONRPCRequest{Id: 6, Method: "tools/call", JSONRPC: mcp.JSPNRPCVersion}
	s.sendToolExecutionError(c, conn, req, errors.New("x"), false)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "event: message")
}

func TestSendAcceptedResponse(t *testing.T) {
	s := &Server{logger: zap.NewNop()}
	c, w := newGin()

	// Test without logger in context
	s.sendAcceptedResponse(c)
	assert.Equal(t, http.StatusAccepted, w.Code)

	// Test with logger in context
	c2, w2 := newGin()
	c2.Set("logger", zap.NewNop())
	s.sendAcceptedResponse(c2)
	assert.Equal(t, http.StatusAccepted, w2.Code)
}
