package notifier

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/amoylab/unla/internal/common/config"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

type fakeNotifier struct {
	recv bool
	send bool
	ch   chan *config.MCPConfig
	err  error
}

func (f *fakeNotifier) Watch(ctx context.Context) (<-chan *config.MCPConfig, error) {
	if !f.recv {
		return nil, errors.New("not receiver")
	}
	if f.ch == nil {
		f.ch = make(chan *config.MCPConfig, 1)
	}
	out := f.ch
	go func() {
		<-ctx.Done()
		close(out)
	}()
	return out, nil
}

func (f *fakeNotifier) NotifyUpdate(ctx context.Context, updated *config.MCPConfig) error {
	if !f.send {
		return errors.New("not sender")
	}
	return f.err
}

func (f *fakeNotifier) CanReceive() bool { return f.recv }
func (f *fakeNotifier) CanSend() bool    { return f.send }

func TestCompositeNotifier_CanSendReceive(t *testing.T) {
	n1 := &fakeNotifier{recv: true}
	n2 := &fakeNotifier{send: true}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	comp := NewCompositeNotifier(ctx, zap.NewNop(), n1, n2)

	assert.True(t, comp.CanReceive())
	assert.True(t, comp.CanSend())
}

func TestCompositeNotifier_WatchForwards(t *testing.T) {
	n1 := &fakeNotifier{recv: true, ch: make(chan *config.MCPConfig, 1)}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	comp := NewCompositeNotifier(ctx, zap.NewNop(), n1)

	ch, err := comp.Watch(context.Background())
	assert.NoError(t, err)

	// Send from underlying and expect to receive on composite channel
	want := &config.MCPConfig{Name: "x"}
	n1.ch <- want

	select {
	case got := <-ch:
		assert.Equal(t, want.Name, got.Name)
	case <-time.After(200 * time.Millisecond):
		t.Fatal("did not receive forwarded notification")
	}
}

func TestCompositeNotifier_NotifyUpdate_AggregatesLastError(t *testing.T) {
	n1 := &fakeNotifier{send: true, err: errors.New("e1")}
	n2 := &fakeNotifier{send: true, err: nil}
	n3 := &fakeNotifier{send: true, err: errors.New("e3")}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	comp := NewCompositeNotifier(ctx, zap.NewNop(), n1, n2, n3)

	err := comp.NotifyUpdate(context.Background(), &config.MCPConfig{Name: "x"})
	if assert.Error(t, err) {
		assert.Contains(t, err.Error(), "e3")
	}
}
