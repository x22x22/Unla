package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/amoylab/unla/internal/auth/jwt"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestAdminAuthMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// helper to perform a request against a router configured with optional claims
	perform := func(claims *jwt.Claims) *httptest.ResponseRecorder {
		r := gin.New()
		// inject claims (if any) before the admin middleware
		r.Use(func(c *gin.Context) {
			if claims != nil {
				c.Set("claims", claims)
			}
			c.Next()
		})
		r.GET("/admin", AdminAuthMiddleware(), func(c *gin.Context) {
			c.String(http.StatusOK, "ok")
		})
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/admin", nil)
		r.ServeHTTP(w, req)
		return w
	}

	// no claims => unauthorized
	resp := perform(nil)
	assert.Equal(t, http.StatusUnauthorized, resp.Code)

	// non-admin => forbidden
	resp = perform(&jwt.Claims{Role: "normal"})
	assert.Equal(t, http.StatusForbidden, resp.Code)

	// admin => ok
	resp = perform(&jwt.Claims{Role: "admin"})
	assert.Equal(t, http.StatusOK, resp.Code)
	assert.Equal(t, "ok", resp.Body.String())
}
