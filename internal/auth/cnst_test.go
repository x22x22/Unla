package auth

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestStorageType_Constants(t *testing.T) {
	assert.Equal(t, StorageType("memory"), StorageTypeMemory)
	assert.Equal(t, StorageType("redis"), StorageTypeRedis)
}

func TestStorageType_String(t *testing.T) {
	assert.Equal(t, "memory", string(StorageTypeMemory))
	assert.Equal(t, "redis", string(StorageTypeRedis))
}
