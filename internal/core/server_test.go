package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseHeaderList(t *testing.T) {
	// empty
	assert.Equal(t, []string{}, parseHeaderList("", false))

	// normal
	got := parseHeaderList("X-A, Y-B , Z-C", false)
	assert.Equal(t, []string{"X-A", "Y-B", "Z-C"}, got)

	// case-insensitive lowercasing
	got2 := parseHeaderList("X-A, y-b", true)
	assert.Equal(t, []string{"x-a", "y-b"}, got2)

	// extras commas/spaces handled
	got3 := parseHeaderList(" X , , Y ", false)
	assert.Equal(t, []string{"X", "Y"}, got3)
}
