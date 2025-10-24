package core

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/amoylab/unla/internal/common/config"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func TestCORSAllowAndOptions(t *testing.T) {
	s := &Server{logger: zap.NewNop()}
	cors := &config.CORSConfig{
		AllowOrigins:     []string{"http://a.com"},
		AllowMethods:     []string{"GET", "POST"},
		AllowHeaders:     []string{"X-A"},
		ExposeHeaders:    []string{"X-B"},
		AllowCredentials: true,
	}

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(s.corsMiddleware(cors))
	r.GET("/x", func(c *gin.Context) { c.String(200, "ok") })

	// Preflight OPTIONS
	req := httptest.NewRequest(http.MethodOptions, "/x", nil)
	req.Header.Set("Origin", "http://a.com")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusNoContent, w.Code)
	assert.Equal(t, "http://a.com", w.Header().Get("Access-Control-Allow-Origin"))
	assert.Equal(t, "GET, POST", w.Header().Get("Access-Control-Allow-Methods"))
	assert.Equal(t, "X-A", w.Header().Get("Access-Control-Allow-Headers"))
	assert.Equal(t, "X-B", w.Header().Get("Access-Control-Expose-Headers"))
	assert.Equal(t, "true", w.Header().Get("Access-Control-Allow-Credentials"))

	// Simple GET with allowed origin
	req = httptest.NewRequest(http.MethodGet, "/x", nil)
	req.Header.Set("Origin", "http://a.com")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, 200, w.Code)
	assert.Equal(t, "http://a.com", w.Header().Get("Access-Control-Allow-Origin"))
}

func TestCORSNotAllowedOrigin(t *testing.T) {
	s := &Server{logger: zap.NewNop()}
	cors := &config.CORSConfig{AllowOrigins: []string{"http://b.com"}}
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(s.corsMiddleware(cors))
	r.GET("/x", func(c *gin.Context) { c.String(200, "ok") })

	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.Header.Set("Origin", "http://a.com")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, 200, w.Code)
	assert.Equal(t, "", w.Header().Get("Access-Control-Allow-Origin"))
}

func TestRecoveryMiddleware(t *testing.T) {
	s := &Server{logger: zap.NewNop()}
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(s.recoveryMiddleware())
	r.GET("/panic", func(c *gin.Context) { panic("boom") })

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/panic", nil))
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestLoggerMiddlewareSetsLogger(t *testing.T) {
	s := &Server{logger: zap.NewNop()}
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(s.loggerMiddleware())
	r.GET("/x", func(c *gin.Context) {
		// logger should be set on context
		v, ok := c.Get("logger")
		assert.True(t, ok)
		assert.NotNil(t, v)
		c.String(200, "ok")
	})

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/x", nil))
	assert.Equal(t, 200, w.Code)
}

func TestEnableTracing(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Test with valid server
	s := &Server{
		logger: zap.NewNop(),
		router: gin.New(),
	}
	s.EnableTracing("test-service")
	// Should not panic and middleware should be added
	assert.NotNil(t, s.router)

	// Test with nil server
	var nilServer *Server
	nilServer.EnableTracing("test")
	// Should not panic

	// Test with server but nil router
	s2 := &Server{logger: zap.NewNop()}
	s2.EnableTracing("test")
	// Should not panic
}
