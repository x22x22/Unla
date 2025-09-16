package handler

import (
	"context"
	"errors"
	"testing"

	"net/http/httptest"

	"github.com/amoylab/unla/internal/apiserver/database"
	jsvc "github.com/amoylab/unla/internal/auth/jwt"
	"github.com/amoylab/unla/internal/common/config"
	"github.com/amoylab/unla/internal/i18n"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

type permissionDBMock struct {
	user           *database.User
	userErr        error
	tenant         *database.Tenant
	tenantErr      error
	userTenants    []*database.Tenant
	userTenantsErr error
}

func (m *permissionDBMock) Close() error                                         { return nil }
func (m *permissionDBMock) SaveMessage(context.Context, *database.Message) error { return nil }
func (m *permissionDBMock) GetMessages(context.Context, string) ([]*database.Message, error) {
	return nil, nil
}
func (m *permissionDBMock) GetMessagesWithPagination(context.Context, string, int, int) ([]*database.Message, error) {
	return nil, nil
}
func (m *permissionDBMock) CreateSession(context.Context, string) error                  { return nil }
func (m *permissionDBMock) CreateSessionWithTitle(context.Context, string, string) error { return nil }
func (m *permissionDBMock) SessionExists(context.Context, string) (bool, error)          { return true, nil }
func (m *permissionDBMock) GetSessions(context.Context) ([]*database.Session, error)     { return nil, nil }
func (m *permissionDBMock) UpdateSessionTitle(context.Context, string, string) error     { return nil }
func (m *permissionDBMock) DeleteSession(context.Context, string) error                  { return nil }
func (m *permissionDBMock) CreateUser(context.Context, *database.User) error             { return nil }
func (m *permissionDBMock) GetUserByUsername(context.Context, string) (*database.User, error) {
	return m.user, m.userErr
}
func (m *permissionDBMock) UpdateUser(context.Context, *database.User) error     { return nil }
func (m *permissionDBMock) DeleteUser(context.Context, uint) error               { return nil }
func (m *permissionDBMock) ListUsers(context.Context) ([]*database.User, error)  { return nil, nil }
func (m *permissionDBMock) CreateTenant(context.Context, *database.Tenant) error { return nil }
func (m *permissionDBMock) GetTenantByName(context.Context, string) (*database.Tenant, error) {
	return m.tenant, m.tenantErr
}
func (m *permissionDBMock) GetTenantByID(context.Context, uint) (*database.Tenant, error) {
	return m.tenant, m.tenantErr
}
func (m *permissionDBMock) UpdateTenant(context.Context, *database.Tenant) error    { return nil }
func (m *permissionDBMock) DeleteTenant(context.Context, uint) error                { return nil }
func (m *permissionDBMock) ListTenants(context.Context) ([]*database.Tenant, error) { return nil, nil }
func (m *permissionDBMock) AddUserToTenant(context.Context, uint, uint) error       { return nil }
func (m *permissionDBMock) RemoveUserFromTenant(context.Context, uint, uint) error  { return nil }
func (m *permissionDBMock) GetUserTenants(context.Context, uint) ([]*database.Tenant, error) {
	return m.userTenants, m.userTenantsErr
}
func (m *permissionDBMock) GetTenantUsers(context.Context, uint) ([]*database.User, error) {
	return nil, nil
}
func (m *permissionDBMock) DeleteUserTenants(context.Context, uint) error { return nil }
func (m *permissionDBMock) Transaction(ctx context.Context, fn func(ctx context.Context) error) error {
	return fn(ctx)
}
func (m *permissionDBMock) GetSystemPrompt(context.Context, uint) (string, error) { return "", nil }
func (m *permissionDBMock) SaveSystemPrompt(context.Context, uint, string) error  { return nil }

// NOTE: we avoid using router; checkTenantPermission does not write to response

func TestMCP_checkTenantPermission_EmptyTenant(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := &permissionDBMock{}
	h := &MCP{db: db, logger: zap.NewNop()}
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/", nil)
	tenant, err := h.checkTenantPermission(c, "", &config.MCPConfig{})
	assert.Nil(t, tenant)
	var ew *i18n.ErrorWithCode
	assert.True(t, errors.As(err, &ew))
	assert.Equal(t, i18n.ErrorBadRequest, ew.GetCode())
}

func TestMCP_checkTenantPermission_MissingClaims(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := &permissionDBMock{}
	h := &MCP{db: db, logger: zap.NewNop()}
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/", nil)
	tenant, err := h.checkTenantPermission(c, "t", &config.MCPConfig{})
	assert.Nil(t, tenant)
	var ew *i18n.ErrorWithCode
	assert.True(t, errors.As(err, &ew))
	assert.Equal(t, i18n.ErrorUnauthorized, ew.GetCode())
}

func TestMCP_checkTenantPermission_UserLookupError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := &permissionDBMock{userErr: assert.AnError}
	h := &MCP{db: db, logger: zap.NewNop()}
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/", nil)
	c.Set("claims", &jsvc.Claims{Username: "u"})
	_, err := h.checkTenantPermission(c, "t", &config.MCPConfig{})
	var ew *i18n.ErrorWithCode
	assert.True(t, errors.As(err, &ew))
	assert.Equal(t, i18n.ErrorInternalServer, ew.GetCode())
}

func TestMCP_checkTenantPermission_TenantNotFound(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := &permissionDBMock{user: &database.User{ID: 1, Username: "u"}, tenantErr: assert.AnError}
	h := &MCP{db: db, logger: zap.NewNop()}
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/", nil)
	c.Set("claims", &jsvc.Claims{Username: "u"})
	_, err := h.checkTenantPermission(c, "t", &config.MCPConfig{})
	var ew *i18n.ErrorWithCode
	assert.True(t, errors.As(err, &ew))
	assert.Equal(t, i18n.ErrorNotFound, ew.GetCode())
}

func TestMCP_checkTenantPermission_PrefixMismatch(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := &permissionDBMock{user: &database.User{ID: 1, Username: "u", Role: database.RoleAdmin}, tenant: &database.Tenant{ID: 2, Name: "t", Prefix: "/tp"}}
	h := &MCP{db: db, logger: zap.NewNop()}
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/", nil)
	c.Set("claims", &jsvc.Claims{Username: "u"})
	cfg := &config.MCPConfig{Routers: []config.RouterConfig{{Prefix: "/wrong"}}}
	_, err := h.checkTenantPermission(c, "t", cfg)
	var ew *i18n.ErrorWithCode
	assert.True(t, errors.As(err, &ew))
	assert.Equal(t, i18n.ErrorBadRequest, ew.GetCode())
}

func TestMCP_checkTenantPermission_Success_Admin(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := &permissionDBMock{user: &database.User{ID: 1, Username: "u", Role: database.RoleAdmin}, tenant: &database.Tenant{ID: 2, Name: "t", Prefix: "tp"}}
	h := &MCP{db: db, logger: zap.NewNop()}
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/", nil)
	c.Set("claims", &jsvc.Claims{Username: "u"})
	cfg := &config.MCPConfig{Routers: []config.RouterConfig{{Prefix: "/tp/api"}}}
	tenant, err := h.checkTenantPermission(c, "t", cfg)
	assert.NoError(t, err)
	assert.NotNil(t, tenant)
}

func TestMCP_checkTenantPermission_NonAdmin_NoPermission(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := &permissionDBMock{
		user:        &database.User{ID: 1, Username: "u", Role: database.RoleNormal},
		tenant:      &database.Tenant{ID: 2, Name: "t", Prefix: "/t"},
		userTenants: []*database.Tenant{{ID: 3, Name: "other"}},
	}
	h := &MCP{db: db, logger: zap.NewNop()}
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/", nil)
	c.Set("claims", &jsvc.Claims{Username: "u"})
	cfg := &config.MCPConfig{Routers: []config.RouterConfig{{Prefix: "/t"}}}
	_, err := h.checkTenantPermission(c, "t", cfg)
	var ew *i18n.ErrorWithCode
	assert.True(t, errors.As(err, &ew))
	assert.Equal(t, i18n.ErrorForbidden, ew.GetCode())
}
