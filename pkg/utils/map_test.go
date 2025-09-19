package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetBool(t *testing.T) {
	m := map[string]any{
		"a": true,
	}
	assert.Equal(t, GetBool(m, "a", false), true)
	// missing key -> default
	assert.Equal(t, GetBool(m, "missing", true), true)
	// wrong type -> default
	assert.Equal(t, GetBool(map[string]any{"x": "not-bool"}, "x", false), false)
	// nil map -> default
	assert.Equal(t, GetBool(nil, "x", true), true)
}

func TestGetString(t *testing.T) {
	m := map[string]any{
		"a": "123",
	}
	assert.Equal(t, GetString(m, "a", ""), "123")
	// missing key -> default
	assert.Equal(t, GetString(m, "missing", "d"), "d")
	// wrong type -> default
	assert.Equal(t, GetString(map[string]any{"x": 1}, "x", "d"), "d")
	// nil map -> default
	assert.Equal(t, GetString(nil, "x", "d"), "d")
}
