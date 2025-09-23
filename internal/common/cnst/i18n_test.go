package cnst

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestI18nConstants(t *testing.T) {
	t.Run("language constants", func(t *testing.T) {
		assert.Equal(t, "en", LangEN)
		assert.Equal(t, "zh", LangZH)
		assert.Equal(t, LangEN, LangDefault)
	})

	t.Run("header and context key constants", func(t *testing.T) {
		assert.Equal(t, "X-Lang", XLang)
		assert.Equal(t, "translator", CtxKeyTranslator)
	})
}
