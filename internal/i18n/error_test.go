package i18n

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/amoylab/unla/internal/common/cnst"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"golang.org/x/text/language"
)

func TestI18nError_Basics(t *testing.T) {
	e := NewWithMessage("MsgID", "Hello, {{.Name}}!")
	e = e.WithParam("Name", "World")
	if got := e.Error(); got != "Hello, World!" {
		t.Fatalf("unexpected error message: %s", got)
	}

	// WithData merge
	e.WithData(map[string]any{"Name": "Alice"})
	if got := e.Error(); got != "Hello, Alice!" {
		t.Fatalf("unexpected merged message: %s", got)
	}

	// TranslateByRequest falls back to default when translator not initialized
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	if got := e.TranslateByRequest(r); got == "" {
		t.Fatalf("expected fallback translation, got empty")
	}
}

func TestErrorWithCode(t *testing.T) {
	ew := NewErrorWithCode("ErrID", ErrorBadRequest)
	if ew.GetCode() != ErrorBadRequest {
		t.Fatalf("unexpected code: %v", ew.GetCode())
	}
	// Change status code
	ew2 := ew.WithHttpCode(ErrorForbidden)
	if ew2.GetCode() != ErrorForbidden {
		t.Fatalf("unexpected changed code: %v", ew2.GetCode())
	}
}

func TestIsAndAsI18nError(t *testing.T) {
	e := New("X")
	if !IsI18nError(e) {
		t.Fatalf("expected true for I18nError")
	}
	if AsI18nError(e) == nil {
		t.Fatalf("expected non-nil from AsI18nError")
	}
	var err error
	if IsI18nError(err) {
		t.Fatalf("nil should not be I18nError")
	}
}

func TestI18nError_GetMessageID(t *testing.T) {
	e := New("TestMessageID")
	assert.Equal(t, "TestMessageID", e.GetMessageID())
}

func TestErrorWithCode_WithData(t *testing.T) {
	ew := NewErrorWithCode("TestError", ErrorBadRequest)
	data := map[string]interface{}{"key": "value"}

	result := ew.WithData(data)
	assert.Equal(t, ew, result) // Should return same instance
	assert.Equal(t, data, ew.GetData())
}

func TestI18nError_TranslateByContext(t *testing.T) {
	gin.SetMode(gin.TestMode)

	e := NewWithMessage("TestMessage", "Hello {{.Name}}")
	e.WithParam("Name", "World")

	t.Run("with language in context", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Set(cnst.XLang, "en")

		result := e.TranslateByContext(c)
		assert.Contains(t, result, "Hello World")
	})

	t.Run("without language in context", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)

		result := e.TranslateByContext(c)
		assert.Contains(t, result, "Hello World")
	})

	t.Run("with invalid language type in context", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Set(cnst.XLang, 123) // Invalid type

		result := e.TranslateByContext(c)
		assert.Contains(t, result, "Hello World")
	})
}

func TestI18nError_TranslateByRequest(t *testing.T) {
	e := NewWithMessage("TestMessage", "Hello {{.Name}}")
	e.WithParam("Name", "World")

	t.Run("with X-Lang header", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set(cnst.XLang, "en")

		result := e.TranslateByRequest(req)
		assert.Contains(t, result, "Hello World")
	})

	t.Run("with Accept-Language header", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("Accept-Language", "zh-CN")

		result := e.TranslateByRequest(req)
		assert.Contains(t, result, "Hello World")
	})

	t.Run("with no headers", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)

		result := e.TranslateByRequest(req)
		assert.Contains(t, result, "Hello World")
	})
}

func TestAsI18nError_WithNonI18nError(t *testing.T) {
	regularError := errors.New("regular error")
	result := AsI18nError(regularError)
	assert.Nil(t, result)
}

func TestIsI18nError_WithNonI18nError(t *testing.T) {
	regularError := errors.New("regular error")
	result := IsI18nError(regularError)
	assert.False(t, result)
}

func TestTranslateError_WithI18nError(t *testing.T) {
	gin.SetMode(gin.TestMode)

	e := NewWithMessage("TestMessage", "Test error message")
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Set(cnst.XLang, "en")

	result := TranslateError(c, e)
	assert.Equal(t, "Test error message", result)
}

func TestTranslateError_WithRegularError(t *testing.T) {
	gin.SetMode(gin.TestMode)

	regularError := errors.New("regular error")
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	result := TranslateError(c, regularError)
	assert.Equal(t, "regular error", result)
}

func TestTranslateError_WithNilError(t *testing.T) {
	gin.SetMode(gin.TestMode)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	result := TranslateError(c, nil)
	assert.Equal(t, "", result)
}

func TestI18nError_ErrorWithTranslator(t *testing.T) {
	// Test with a translator available
	translatorOnce = sync.Once{}
	translator = NewI18n(language.English)

	e := New("NonExistentMessage")
	result := e.Error()
	assert.Equal(t, "NonExistentMessage", result) // Falls back to message ID when translation fails

	// Test with template data but no translator
	translator = nil
	e2 := NewWithMessage("TestMessage", "Hello {{.Name}}")
	e2.WithParam("Name", "World")
	result2 := e2.Error()
	assert.Equal(t, "Hello World", result2)
}

func TestI18nError_ErrorWithNoData(t *testing.T) {
	// Test error without template data
	translator = nil
	e := NewWithMessage("TestMessage", "Simple message")

	result := e.Error()
	assert.Equal(t, "Simple message", result)
}

func TestTranslateError_MoreCases(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("with ErrorWithCode", func(t *testing.T) {
		errWithCode := NewErrorWithCode("TestError", ErrorBadRequest)
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Set(cnst.XLang, "en")

		result := TranslateError(c, errWithCode)
		assert.Equal(t, "TestError", result)
	})

	t.Run("with nested I18nError", func(t *testing.T) {
		nestedErr := NewWithMessage("NestedError", "Nested error message")
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)

		result := TranslateError(c, nestedErr)
		assert.Equal(t, "Nested error message", result)
	})
}

func TestI18nError_TranslateByContext_EdgeCases(t *testing.T) {
	gin.SetMode(gin.TestMode)

	e := NewWithMessage("TestMessage", "Test message {{.param}}")
	e.WithParam("param", "value")

	t.Run("with empty string language", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Set(cnst.XLang, "")

		result := e.TranslateByContext(c)
		assert.Contains(t, result, "Test message value")
	})
}

func TestI18nError_TranslateByRequest_EdgeCases(t *testing.T) {
	e := NewWithMessage("TestMessage", "Test message {{.param}}")
	e.WithParam("param", "value")

	t.Run("with empty Accept-Language", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("Accept-Language", "")

		result := e.TranslateByRequest(req)
		assert.Contains(t, result, "Test message value")
	})

	t.Run("with malformed Accept-Language", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("Accept-Language", "invalid-format")

		result := e.TranslateByRequest(req)
		assert.Contains(t, result, "Test message value")
	})
}
