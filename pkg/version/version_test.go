package version

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetReturnsVersion(t *testing.T) {
	// Get should return the embedded VERSION content.
	got := Get()
	assert.Equal(t, Version, got)
}

func TestVersionNotEmptyAndPrefixed(t *testing.T) {
	s := Get()
	assert.NotEmpty(t, s)
	// Common convention in this repo: version strings are prefixed with 'v'
	if s != "" {
		assert.Equal(t, byte('v'), s[0])
	}
}
