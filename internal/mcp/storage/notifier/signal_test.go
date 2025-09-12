package notifier

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/amoylab/unla/internal/common/cnst"
	"github.com/amoylab/unla/internal/common/config"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func TestNewSignalNotifier_PanicsOnNilLogger(t *testing.T) {
	assert.Panics(t, func() {
		NewSignalNotifier(context.Background(), nil, "pidfile", config.RoleBoth)
	})
}

func TestNewSignalNotifier_PanicsOnEmptyPID(t *testing.T) {
	assert.Panics(t, func() {
		NewSignalNotifier(context.Background(), zap.NewNop(), "", config.RoleBoth)
	})
}

func TestSignalNotifier_CanSendReceiveByRole(t *testing.T) {
	nRecv := NewSignalNotifier(context.Background(), zap.NewNop(), "pidfile", config.RoleReceiver)
	assert.True(t, nRecv.CanReceive())
	assert.False(t, nRecv.CanSend())

	nSend := NewSignalNotifier(context.Background(), zap.NewNop(), "pidfile", config.RoleSender)
	assert.False(t, nSend.CanReceive())
	assert.True(t, nSend.CanSend())

	nBoth := NewSignalNotifier(context.Background(), zap.NewNop(), "pidfile", config.RoleBoth)
	assert.True(t, nBoth.CanReceive())
	assert.True(t, nBoth.CanSend())
}

func TestSignalNotifier_WatchWhenNotReceiver(t *testing.T) {
	n := NewSignalNotifier(context.Background(), zap.NewNop(), "pidfile", config.RoleSender)
	ch, err := n.Watch(context.Background())
	assert.Nil(t, ch)
	assert.ErrorIs(t, err, cnst.ErrNotReceiver)
}

func TestSignalNotifier_WatchAndNotify(t *testing.T) {
	n := NewSignalNotifier(context.Background(), zap.NewNop(), "pidfile", config.RoleReceiver)

	watchCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ch, err := n.Watch(watchCtx)
	assert.NoError(t, err)
	assert.NotNil(t, ch)

	// Simulate a signal by directly invoking notifyWatchers
	n.notifyWatchers()

	select {
	case v := <-ch:
		assert.Nil(t, v)
	case <-time.After(1 * time.Second):
		t.Fatal("timed out waiting for watcher notification")
	}

	// After cancel, channel should be closed
	cancel()
	// Allow cleanup goroutine to run
	select {
	case _, ok := <-ch:
		if ok {
			// If something is still buffered, wait a moment and try again
			time.Sleep(50 * time.Millisecond)
			_, ok2 := <-ch
			assert.False(t, ok2, "channel should be closed after context cancel")
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timed out waiting for channel close")
	}
}

func TestSignalNotifier_NotifyUpdate_NotSender(t *testing.T) {
	n := NewSignalNotifier(context.Background(), zap.NewNop(), "pidfile", config.RoleReceiver)
	err := n.NotifyUpdate(context.Background(), nil)
	assert.ErrorIs(t, err, cnst.ErrNotSender)
}

func TestSignalNotifier_NotifyUpdate_SendError(t *testing.T) {
	// Create a temp directory and a pid file with invalid content to force an error
	dir := t.TempDir()
	pidPath := filepath.Join(dir, "test.pid")
	assert.NoError(t, os.WriteFile(pidPath, []byte("not-a-pid"), 0o644))

	n := NewSignalNotifier(context.Background(), zap.NewNop(), pidPath, config.RoleSender)
	err := n.NotifyUpdate(context.Background(), nil)
	assert.Error(t, err)
}
