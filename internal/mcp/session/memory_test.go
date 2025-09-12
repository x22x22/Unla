package session

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func TestMemoryStore_RegisterGetListUnregister(t *testing.T) {
	s := NewMemoryStore(zap.NewNop())
	meta := &Meta{ID: "sid"}

	// register
	conn, err := s.Register(context.Background(), meta)
	assert.NoError(t, err)
	assert.NotNil(t, conn)

	// duplicate register should fail
	_, err = s.Register(context.Background(), meta)
	assert.Error(t, err)

	// get
	got, err := s.Get(context.Background(), "sid")
	assert.NoError(t, err)
	assert.Equal(t, "sid", got.Meta().ID)

	// list
	list, err := s.List(context.Background())
	assert.NoError(t, err)
	assert.Len(t, list, 1)

	// unregister
	err = s.Unregister(context.Background(), "sid")
	assert.NoError(t, err)
	// get after unregister
	_, err = s.Get(context.Background(), "sid")
	assert.ErrorIs(t, err, ErrSessionNotFound)

	// unregister unknown id
	assert.ErrorIs(t, s.Unregister(context.Background(), "nope"), ErrSessionNotFound)
}

func TestMemoryConnection_SendQueueFull(t *testing.T) {
	c := &MemoryConnection{meta: &Meta{ID: "x"}, queue: make(chan *Message, 2)}
	assert.NoError(t, c.Send(context.Background(), &Message{Event: "e"}))
	assert.NoError(t, c.Send(context.Background(), &Message{Event: "e2"}))
	// now should be full
	assert.Error(t, c.Send(context.Background(), &Message{Event: "e3"}))
}
