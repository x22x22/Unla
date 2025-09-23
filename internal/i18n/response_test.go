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

func TestRespondWithSuccessAndHelpers(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/ok", func(c *gin.Context) {
		RespondOK(c, SuccessOperationCompleted, map[string]any{"a": 1}, map[string]any{"b": 2})
	})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/ok", nil)
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	r2 := gin.New()
	r2.GET("/created", func(c *gin.Context) {
		RespondCreated(c, SuccessItemCreated, nil, nil)
	})
	w2 := httptest.NewRecorder()
	req2, _ := http.NewRequest(http.MethodGet, "/created", nil)
	r2.ServeHTTP(w2, req2)
	assert.Equal(t, http.StatusCreated, w2.Code)
}

func TestSuccessResponseChain(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/chain", func(c *gin.Context) {
		Success(SuccessOperationCompleted).With("k1", "v1").WithData(map[string]any{"k2": 2}).WithPayload(map[string]any{"p": true}).Send(c)
	})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/chain", nil)
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestRespondWithError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/err", func(c *gin.Context) {
		RespondWithError(c, ErrForbidden)
	})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/err", nil)
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestCreated(t *testing.T) {
	resp := Created(SuccessItemCreated)

	assert.Equal(t, http.StatusCreated, resp.StatusCode)
	assert.Equal(t, SuccessItemCreated, resp.MsgID)
	assert.Nil(t, resp.Data)
	assert.Nil(t, resp.Payload)
}

func TestSuccessResponse_Methods(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("With method", func(t *testing.T) {
		resp := Success(SuccessOperationCompleted)
		result := resp.With("key", "value")

		assert.Equal(t, resp, result) // Should return same instance
		assert.Equal(t, "value", resp.Data["key"])
	})

	t.Run("WithData method with nil data", func(t *testing.T) {
		resp := Success(SuccessOperationCompleted)
		result := resp.WithData(nil)

		assert.Equal(t, resp, result) // Should return same instance
		assert.NotNil(t, resp.Data)   // WithData initializes empty map even for nil
		assert.Equal(t, 0, len(resp.Data))
	})

	t.Run("WithData method with empty data", func(t *testing.T) {
		resp := Success(SuccessOperationCompleted)
		result := resp.WithData(map[string]interface{}{})

		assert.Equal(t, resp, result) // Should return same instance
		assert.NotNil(t, resp.Data)
		assert.Equal(t, 0, len(resp.Data))
	})

	t.Run("WithPayload method", func(t *testing.T) {
		resp := Success(SuccessOperationCompleted)
		payload := map[string]interface{}{"result": "success"}
		result := resp.WithPayload(payload)

		assert.Equal(t, resp, result) // Should return same instance
		assert.Equal(t, payload, resp.Payload)
	})

	t.Run("Send method", func(t *testing.T) {
		r := gin.New()
		r.GET("/test", func(c *gin.Context) {
			Success(SuccessOperationCompleted).
				With("key", "value").
				WithPayload(map[string]interface{}{"result": "test"}).
				Send(c)
		})

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/test", nil)
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), "SuccessOperationCompleted")
	})
}

func TestRespondWithSuccess_ErrorConditions(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("with nil data", func(t *testing.T) {
		r := gin.New()
		r.GET("/test", func(c *gin.Context) {
			RespondWithSuccess(c, http.StatusOK, SuccessOperationCompleted, nil, nil)
		})

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/test", nil)
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("with nil payload", func(t *testing.T) {
		r := gin.New()
		r.GET("/test", func(c *gin.Context) {
			RespondWithSuccess(c, http.StatusCreated, SuccessItemCreated, map[string]interface{}{"key": "value"}, nil)
		})

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/test", nil)
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusCreated, w.Code)
	})

	t.Run("with translator available", func(t *testing.T) {
		// Test with a real translator
		originalTranslator := translator
		defer func() { translator = originalTranslator }()

		dir := t.TempDir()
		content := []byte(`[SuccessOperationCompleted]
other = "Operation completed successfully"
`)
		if err := os.WriteFile(filepath.Join(dir, "en.toml"), content, 0644); err != nil {
			t.Fatalf("write toml: %v", err)
		}

		translatorOnce = sync.Once{}
		translator = NewI18n(language.English)
		if err := translator.LoadTranslations(dir); err != nil {
			t.Fatalf("load translations: %v", err)
		}

		r := gin.New()
		r.GET("/test", func(c *gin.Context) {
			c.Set(cnst.XLang, "en")
			RespondWithSuccess(c, http.StatusOK, "SuccessOperationCompleted", nil, nil)
		})

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/test", nil)
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), "Operation completed successfully")
	})

	t.Run("with invalid translation", func(t *testing.T) {
		r := gin.New()
		r.GET("/test", func(c *gin.Context) {
			// Test with non-existent message ID
			RespondWithSuccess(c, http.StatusOK, "NonExistentMessage", map[string]interface{}{"test": "data"}, "payload")
		})

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/test", nil)
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), "NonExistentMessage") // Should use message ID as fallback
	})
}

func TestRespondWithError_WithDifferentErrors(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("with i18n error", func(t *testing.T) {
		r := gin.New()
		r.GET("/test", func(c *gin.Context) {
			customErr := NewErrorWithCode("CustomError", ErrorNotFound)
			RespondWithError(c, customErr)
		})

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/test", nil)
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})

	t.Run("with nil error", func(t *testing.T) {
		r := gin.New()
		r.GET("/test", func(c *gin.Context) {
			RespondWithError(c, nil)
		})

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/test", nil)
		r.ServeHTTP(w, req)

		// RespondWithError with nil doesn't actually send error response, just returns
		assert.Equal(t, http.StatusOK, w.Code)
	})
}
