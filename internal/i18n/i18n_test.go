package i18n

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/amoylab/unla/internal/common/cnst"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"golang.org/x/text/language"
)

func TestNewI18nAndLoadTranslations(t *testing.T) {
	dir := t.TempDir()
	// write a minimal TOML translation file
	content := []byte(`[Hello]
other = "Hello"
`)
	if err := os.WriteFile(filepath.Join(dir, "en.toml"), content, 0644); err != nil {
		t.Fatalf("write toml: %v", err)
	}

	i := NewI18n(language.English)
	if err := i.LoadTranslations(dir); err != nil {
		t.Fatalf("load translations: %v", err)
	}
}

func TestSetDefaultLanguage(t *testing.T) {
	originalLang := defaultLang
	defer func() { defaultLang = originalLang }()

	SetDefaultLanguage("zh")
	assert.Equal(t, "zh", defaultLang)

	SetDefaultLanguage("en")
	assert.Equal(t, "en", defaultLang)
}

func TestInitTranslator(t *testing.T) {
	// Reset translator for test
	translatorOnce = sync.Once{}
	translator = nil

	t.Run("successful initialization", func(t *testing.T) {
		dir := t.TempDir()
		content := []byte(`[TestMessage]
other = "Test message"
`)
		if err := os.WriteFile(filepath.Join(dir, "en.toml"), content, 0644); err != nil {
			t.Fatalf("write toml: %v", err)
		}

		err := InitTranslator(dir)
		assert.NoError(t, err)
		assert.NotNil(t, translator)
	})

	t.Run("failure with invalid path", func(t *testing.T) {
		// Reset for this test
		translatorOnce = sync.Once{}
		translator = nil

		err := InitTranslator("/non/existent/path")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to read translations directory")
	})
}

func TestGetTranslator(t *testing.T) {
	// Reset translator
	translatorOnce = sync.Once{}
	translator = nil

	// First call should initialize translator
	tr := GetTranslator()
	assert.NotNil(t, tr)

	// Second call should return same instance
	tr2 := GetTranslator()
	assert.Equal(t, tr, tr2)
}

func TestI18n_TranslateContext(t *testing.T) {
	gin.SetMode(gin.TestMode)

	dir := t.TempDir()
	content := []byte(`[TestMessage]
other = "Test message in English"
`)
	if err := os.WriteFile(filepath.Join(dir, "en.toml"), content, 0644); err != nil {
		t.Fatalf("write toml: %v", err)
	}

	i18nInstance := NewI18n(language.English)
	if err := i18nInstance.LoadTranslations(dir); err != nil {
		t.Fatalf("load translations: %v", err)
	}

	t.Run("with language in context", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Set(cnst.XLang, "en")

		result := i18nInstance.TranslateContext(c, "TestMessage", nil)
		assert.Contains(t, result, "Test message")
	})

	t.Run("without language in context", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)

		result := i18nInstance.TranslateContext(c, "TestMessage", nil)
		// Should use default language "zh", but translation exists for "en"
		assert.Contains(t, result, "Test message")
	})

	t.Run("with invalid language type in context", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Set(cnst.XLang, 123) // Invalid type

		result := i18nInstance.TranslateContext(c, "TestMessage", nil)
		assert.Contains(t, result, "Test message")
	})
}

func TestGetLanguageFromRequest(t *testing.T) {
	t.Run("with X-Lang header", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set(cnst.XLang, "en")

		lang := getLanguageFromRequest(req)
		assert.Equal(t, "en", lang)
	})

	t.Run("with Accept-Language header", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8")

		lang := getLanguageFromRequest(req)
		assert.Equal(t, "zh", lang)
	})

	t.Run("with no language headers", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)

		lang := getLanguageFromRequest(req)
		assert.Equal(t, defaultLang, lang)
	})
}

func TestNormalizeLang(t *testing.T) {
	t.Run("supported languages", func(t *testing.T) {
		assert.Equal(t, "en", normalizeLang("en"))
		assert.Equal(t, "zh", normalizeLang("zh"))
		assert.Equal(t, "en", normalizeLang("EN"))
		assert.Equal(t, "zh", normalizeLang("zh-CN"))
		assert.Equal(t, "en", normalizeLang("en-US"))
	})

	t.Run("unsupported languages", func(t *testing.T) {
		assert.Equal(t, defaultLang, normalizeLang("fr"))
		assert.Equal(t, defaultLang, normalizeLang("de"))
		assert.Equal(t, defaultLang, normalizeLang("ja"))
	})
}

func TestTranslateMessage(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Setup translator
	dir := t.TempDir()
	content := []byte(`[TestMessage]
other = "Test message"
`)
	if err := os.WriteFile(filepath.Join(dir, "en.toml"), content, 0644); err != nil {
		t.Fatalf("write toml: %v", err)
	}

	translatorOnce = sync.Once{}
	translator = NewI18n(language.English)
	if err := translator.LoadTranslations(dir); err != nil {
		t.Fatalf("load translations: %v", err)
	}

	t.Run("with language in context", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Set(cnst.XLang, "en")

		result := TranslateMessage(c, "TestMessage", nil)
		assert.Contains(t, result, "Test message")
	})

	t.Run("without language in context", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)

		result := TranslateMessage(c, "NonExistentMessage", nil)
		assert.Equal(t, "NonExistentMessage", result)
	})

	t.Run("with invalid language type", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Set(cnst.XLang, 123)

		result := TranslateMessage(c, "TestMessage", nil)
		assert.Contains(t, result, "Test message")
	})

	t.Run("with nil translator", func(t *testing.T) {
		// Temporarily set translator to nil
		originalTranslator := translator
		translator = nil
		defer func() { translator = originalTranslator }()

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Set(cnst.XLang, "en")

		result := TranslateMessage(c, "TestMessage", nil)
		assert.Equal(t, "TestMessage", result) // Should return message ID when translator is nil
	})
}

func TestTranslateMessageGin(t *testing.T) {
	gin.SetMode(gin.TestMode)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Set(cnst.XLang, "en")

	// Should be identical to TranslateMessage
	result1 := TranslateMessage(c, "TestMessage", nil)
	result2 := TranslateMessageGin(c, "TestMessage", nil)
	assert.Equal(t, result1, result2)
}

func TestI18n_DebugLoadedMessages(t *testing.T) {
	i := NewI18n(language.English)

	// Should not panic
	assert.NotPanics(t, func() {
		i.DebugLoadedMessages()
	})
}

func TestLoadTranslations_EdgeCases(t *testing.T) {
	i := NewI18n(language.English)

	t.Run("directory with non-toml files", func(t *testing.T) {
		dir := t.TempDir()

		// Create non-toml files
		if err := os.WriteFile(filepath.Join(dir, "file.txt"), []byte("text"), 0644); err != nil {
			t.Fatalf("write txt: %v", err)
		}
		if err := os.WriteFile(filepath.Join(dir, "file.json"), []byte("{}"), 0644); err != nil {
			t.Fatalf("write json: %v", err)
		}

		// Should not error, just skip non-toml files
		err := i.LoadTranslations(dir)
		assert.NoError(t, err)
	})

	t.Run("directory with subdirectories", func(t *testing.T) {
		dir := t.TempDir()
		subDir := filepath.Join(dir, "subdir")
		if err := os.Mkdir(subDir, 0755); err != nil {
			t.Fatalf("create subdir: %v", err)
		}

		// Should skip directories
		err := i.LoadTranslations(dir)
		assert.NoError(t, err)
	})
}
