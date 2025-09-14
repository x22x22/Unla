package notifier

import (
	"context"
	"testing"

	"github.com/amoylab/unla/internal/common/config"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func TestNewNotifier_UnknownType(t *testing.T) {
	_, err := NewNotifier(context.Background(), zap.NewNop(), &config.NotifierConfig{Type: "unknown"})
	assert.Error(t, err)
}
