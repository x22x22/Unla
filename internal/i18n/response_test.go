package i18n

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
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
