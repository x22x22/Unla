package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFirstNonEmpty(t *testing.T) {
	t.Run("returns first when first is non-empty", func(t *testing.T) {
		assert.Equal(t, "a", FirstNonEmpty("a", "b"))
	})

	t.Run("returns second when first is empty", func(t *testing.T) {
		assert.Equal(t, "b", FirstNonEmpty("", "b"))
	})

	t.Run("returns empty when both are empty", func(t *testing.T) {
		assert.Equal(t, "", FirstNonEmpty("", ""))
	})

	t.Run("treats whitespace as non-empty", func(t *testing.T) {
		assert.Equal(t, " ", FirstNonEmpty(" ", "b"))
	})
}

func TestSplitByMultipleDelimiters(t *testing.T) {
	// no delimiters -> whole string
	got := SplitByMultipleDelimiters("a,b;c")
	assert.Equal(t, []string{"a,b;c"}, got)

	got = SplitByMultipleDelimiters("a,b;c|d", ",", ";", "|")
	assert.Equal(t, []string{"a", "b", "c", "d"}, got)
}
