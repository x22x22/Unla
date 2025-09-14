package notifier

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/amoylab/unla/internal/common/config"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func TestAPINotifier_NotifyUpdate_Success(t *testing.T) {
	// Handler validates path and JSON body
	var received config.MCPConfig
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Must be POST to /_reload
		if r.Method != http.MethodPost || r.URL.Path != "/_reload" {
			http.Error(w, "bad path", http.StatusNotFound)
			return
		}
		if r.Body != nil {
			defer r.Body.Close()
			b, _ := io.ReadAll(r.Body)
			if len(b) > 0 {
				_ = json.Unmarshal(b, &received)
			}
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}))
	defer srv.Close()

	// targetURL without trailing '/_reload' gets normalized inside NotifyUpdate
	n := NewAPINotifier(zap.NewNop(), 0, config.RoleSender, srv.URL)

	err := n.NotifyUpdate(context.Background(), &config.MCPConfig{Name: "cfg1"})
	assert.NoError(t, err)
	assert.Equal(t, "cfg1", received.Name)

	// Already suffixed URL should still work
	n2 := NewAPINotifier(zap.NewNop(), 0, config.RoleSender, srv.URL+"/_reload")
	err = n2.NotifyUpdate(context.Background(), nil)
	assert.NoError(t, err)
}

func TestAPINotifier_NotifyUpdate_Errors(t *testing.T) {
	// Not sender role
	n := NewAPINotifier(zap.NewNop(), 0, config.RoleReceiver, "http://example")
	assert.Error(t, n.NotifyUpdate(context.Background(), nil))

	// Empty target url
	n2 := NewAPINotifier(zap.NewNop(), 0, config.RoleSender, "")
	assert.Error(t, n2.NotifyUpdate(context.Background(), nil))

	// Remote returns non-200
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusBadGateway)
	}))
	defer srv.Close()

	n3 := NewAPINotifier(zap.NewNop(), 0, config.RoleSender, srv.URL)
	assert.Error(t, n3.NotifyUpdate(context.Background(), nil))
}

func TestAPINotifier_Watch_BasicAndShutdown(t *testing.T) {
	// Receiver role creates server; use port 0 and ensure Shutdown works
	n := NewAPINotifier(zap.NewNop(), 0, config.RoleReceiver, "")

	ctx, cancel := context.WithCancel(context.Background())
	ch, err := n.Watch(ctx)
	assert.NoError(t, err)
	assert.NotNil(t, ch)

	// cancel and ensure channel closes soon
	cancel()
	select {
	case _, ok := <-ch:
		// channel should be closed or a value; both are fine, but must not block
		_ = ok
	case <-time.After(200 * time.Millisecond):
		t.Fatal("watch channel did not close in time")
	}

	// Shutdown should not error
	assert.NoError(t, n.Shutdown(context.Background()))

	// When not receiver, Watch returns error
	n2 := NewAPINotifier(zap.NewNop(), 0, config.RoleSender, "")
	ch2, err := n2.Watch(context.Background())
	assert.Error(t, err)
	assert.Nil(t, ch2)
}
