package session

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestMessage(t *testing.T) {
	t.Run("creates message with event and data", func(t *testing.T) {
		msg := &Message{
			Event: "test_event",
			Data:  []byte("test data"),
		}

		assert.Equal(t, "test_event", msg.Event)
		assert.Equal(t, []byte("test data"), msg.Data)
	})

	t.Run("handles empty event and data", func(t *testing.T) {
		msg := &Message{
			Event: "",
			Data:  nil,
		}

		assert.Equal(t, "", msg.Event)
		assert.Nil(t, msg.Data)
	})
}

func TestRequestInfo(t *testing.T) {
	t.Run("creates request info with maps", func(t *testing.T) {
		req := &RequestInfo{
			Headers: map[string]string{"Content-Type": "application/json"},
			Query:   map[string]string{"param": "value"},
			Cookies: map[string]string{"session": "abc123"},
		}

		assert.Equal(t, "application/json", req.Headers["Content-Type"])
		assert.Equal(t, "value", req.Query["param"])
		assert.Equal(t, "abc123", req.Cookies["session"])
	})

	t.Run("handles nil maps", func(t *testing.T) {
		req := &RequestInfo{
			Headers: nil,
			Query:   nil,
			Cookies: nil,
		}

		assert.Nil(t, req.Headers)
		assert.Nil(t, req.Query)
		assert.Nil(t, req.Cookies)
	})
}

func TestMeta(t *testing.T) {
	now := time.Now()
	reqInfo := &RequestInfo{
		Headers: map[string]string{"Authorization": "Bearer token"},
		Query:   map[string]string{"v": "1"},
		Cookies: map[string]string{"user": "test"},
	}

	t.Run("creates meta with all fields", func(t *testing.T) {
		meta := &Meta{
			ID:        "session-123",
			CreatedAt: now,
			Prefix:    "app",
			Type:      "sse",
			Request:   reqInfo,
			Extra:     []byte("extra data"),
		}

		assert.Equal(t, "session-123", meta.ID)
		assert.Equal(t, now, meta.CreatedAt)
		assert.Equal(t, "app", meta.Prefix)
		assert.Equal(t, "sse", meta.Type)
		assert.Equal(t, reqInfo, meta.Request)
		assert.Equal(t, []byte("extra data"), meta.Extra)
	})

	t.Run("handles minimal meta", func(t *testing.T) {
		meta := &Meta{
			ID:        "minimal",
			CreatedAt: now,
		}

		assert.Equal(t, "minimal", meta.ID)
		assert.Equal(t, now, meta.CreatedAt)
		assert.Equal(t, "", meta.Prefix)
		assert.Equal(t, "", meta.Type)
		assert.Nil(t, meta.Request)
		assert.Nil(t, meta.Extra)
	})

	t.Run("handles request info in meta", func(t *testing.T) {
		meta := &Meta{
			ID:      "with-request",
			Request: reqInfo,
		}

		assert.NotNil(t, meta.Request)
		assert.Equal(t, "Bearer token", meta.Request.Headers["Authorization"])
		assert.Equal(t, "1", meta.Request.Query["v"])
		assert.Equal(t, "test", meta.Request.Cookies["user"])
	})
}
