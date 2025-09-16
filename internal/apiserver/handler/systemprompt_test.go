package handler

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/amoylab/unla/internal/apiserver/database"
	jsvc "github.com/amoylab/unla/internal/auth/jwt"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

type spDBMock struct {
	user   *database.User
	prompt string
	saved  string
}

func (m *spDBMock) Close() error                                                     { return nil }
func (m *spDBMock) SaveMessage(context.Context, *database.Message) error             { return nil }
func (m *spDBMock) GetMessages(context.Context, string) ([]*database.Message, error) { return nil, nil }
func (m *spDBMock) GetMessagesWithPagination(context.Context, string, int, int) ([]*database.Message, error) {
	return nil, nil
}
func (m *spDBMock) CreateSession(context.Context, string) error                  { return nil }
func (m *spDBMock) CreateSessionWithTitle(context.Context, string, string) error { return nil }
func (m *spDBMock) SessionExists(context.Context, string) (bool, error)          { return true, nil }
func (m *spDBMock) GetSessions(context.Context) ([]*database.Session, error)     { return nil, nil }
func (m *spDBMock) UpdateSessionTitle(context.Context, string, string) error     { return nil }
func (m *spDBMock) DeleteSession(context.Context, string) error                  { return nil }
func (m *spDBMock) CreateUser(context.Context, *database.User) error             { return nil }
func (m *spDBMock) GetUserByUsername(context.Context, string) (*database.User, error) {
	return m.user, nil
}
func (m *spDBMock) UpdateUser(context.Context, *database.User) error     { return nil }
func (m *spDBMock) DeleteUser(context.Context, uint) error               { return nil }
func (m *spDBMock) ListUsers(context.Context) ([]*database.User, error)  { return nil, nil }
func (m *spDBMock) CreateTenant(context.Context, *database.Tenant) error { return nil }
func (m *spDBMock) GetTenantByName(context.Context, string) (*database.Tenant, error) {
	return nil, nil
}
func (m *spDBMock) GetTenantByID(context.Context, uint) (*database.Tenant, error)    { return nil, nil }
func (m *spDBMock) UpdateTenant(context.Context, *database.Tenant) error             { return nil }
func (m *spDBMock) DeleteTenant(context.Context, uint) error                         { return nil }
func (m *spDBMock) ListTenants(context.Context) ([]*database.Tenant, error)          { return nil, nil }
func (m *spDBMock) AddUserToTenant(context.Context, uint, uint) error                { return nil }
func (m *spDBMock) RemoveUserFromTenant(context.Context, uint, uint) error           { return nil }
func (m *spDBMock) GetUserTenants(context.Context, uint) ([]*database.Tenant, error) { return nil, nil }
func (m *spDBMock) GetTenantUsers(context.Context, uint) ([]*database.User, error)   { return nil, nil }
func (m *spDBMock) DeleteUserTenants(context.Context, uint) error                    { return nil }
func (m *spDBMock) Transaction(ctx context.Context, fn func(ctx context.Context) error) error {
	return fn(ctx)
}
func (m *spDBMock) GetSystemPrompt(context.Context, uint) (string, error) { return m.prompt, nil }
func (m *spDBMock) SaveSystemPrompt(context.Context, uint, string) error  { return nil }

func TestSystemPrompt_GetSystemPrompt_MissingClaims(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := &spDBMock{}
	h := NewSystemPrompt(db, zap.NewNop())

	r := gin.New()
	r.GET("/sp", func(c *gin.Context) { h.GetSystemPrompt(c) })
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/sp", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestSystemPrompt_GetSystemPrompt_Success(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := &spDBMock{user: &database.User{ID: 1, Username: "u"}, prompt: "hello"}
	h := NewSystemPrompt(db, zap.NewNop())

	r := gin.New()
	r.GET("/sp", func(c *gin.Context) {
		c.Set("claims", &jsvc.Claims{Username: "u"})
		h.GetSystemPrompt(c)
	})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/sp", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "\"prompt\":\"hello\"")
}

func TestSystemPrompt_SaveSystemPrompt_InvalidJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := &spDBMock{user: &database.User{ID: 1, Username: "u"}}
	h := NewSystemPrompt(db, zap.NewNop())

	r := gin.New()
	r.POST("/sp", func(c *gin.Context) {
		c.Set("claims", &jsvc.Claims{Username: "u"})
		h.SaveSystemPrompt(c)
	})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/sp", bytes.NewBufferString("{"))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestSystemPrompt_SaveSystemPrompt_Success(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := &spDBMock{user: &database.User{ID: 1, Username: "u"}}
	h := NewSystemPrompt(db, zap.NewNop())

	r := gin.New()
	r.POST("/sp", func(c *gin.Context) {
		c.Set("claims", &jsvc.Claims{Username: "u"})
		h.SaveSystemPrompt(c)
	})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/sp", bytes.NewBufferString(`{"prompt":"hi"}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "\"success\":true")
}
