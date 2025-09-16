package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/amoylab/unla/internal/apiserver/database"
	jsvc "github.com/amoylab/unla/internal/auth/jwt"
	"github.com/amoylab/unla/internal/common/config"
	"github.com/amoylab/unla/internal/mcp/storage"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

type storeListMock struct {
	cfgs []*config.MCPConfig
	err  error
	storage.Store
}

func (s *storeListMock) List(_ context.Context, includeDeleted ...bool) ([]*config.MCPConfig, error) {
	return s.cfgs, s.err
}

type storeGetMock struct {
	cfg *config.MCPConfig
	err error
}

func (s *storeGetMock) Create(ctx context.Context, cfg *config.MCPConfig) error { return nil }
func (s *storeGetMock) Get(ctx context.Context, tenant, name string, includeDeleted ...bool) (*config.MCPConfig, error) {
	return s.cfg, s.err
}
func (s *storeGetMock) List(ctx context.Context, includeDeleted ...bool) ([]*config.MCPConfig, error) {
	return []*config.MCPConfig{s.cfg}, nil
}
func (s *storeGetMock) ListUpdated(ctx context.Context, since time.Time) ([]*config.MCPConfig, error) {
	return nil, nil
}
func (s *storeGetMock) Update(ctx context.Context, cfg *config.MCPConfig) error { return nil }
func (s *storeGetMock) Delete(ctx context.Context, tenant, name string) error   { return nil }
func (s *storeGetMock) GetVersion(ctx context.Context, tenant, name string, version int) (*config.MCPConfigVersion, error) {
	return nil, nil
}
func (s *storeGetMock) ListVersions(ctx context.Context, tenant, name string) ([]*config.MCPConfigVersion, error) {
	return nil, nil
}
func (s *storeGetMock) DeleteVersion(ctx context.Context, tenant, name string, version int) error {
	return nil
}
func (s *storeGetMock) SetActiveVersion(ctx context.Context, tenant, name string, version int) error {
	return nil
}

// db mock for GetConfigNames
type namesDBMock struct {
	user        *database.User
	userErr     error
	userTenants []*database.Tenant
	utErr       error
	tenant      *database.Tenant
	tenantErr   error
}

func (m *namesDBMock) Close() error                                         { return nil }
func (m *namesDBMock) SaveMessage(context.Context, *database.Message) error { return nil }
func (m *namesDBMock) GetMessages(context.Context, string) ([]*database.Message, error) {
	return nil, nil
}
func (m *namesDBMock) GetMessagesWithPagination(context.Context, string, int, int) ([]*database.Message, error) {
	return nil, nil
}
func (m *namesDBMock) CreateSession(context.Context, string) error                  { return nil }
func (m *namesDBMock) CreateSessionWithTitle(context.Context, string, string) error { return nil }
func (m *namesDBMock) SessionExists(context.Context, string) (bool, error)          { return true, nil }
func (m *namesDBMock) GetSessions(context.Context) ([]*database.Session, error)     { return nil, nil }
func (m *namesDBMock) UpdateSessionTitle(context.Context, string, string) error     { return nil }
func (m *namesDBMock) DeleteSession(context.Context, string) error                  { return nil }
func (m *namesDBMock) CreateUser(context.Context, *database.User) error             { return nil }
func (m *namesDBMock) GetUserByUsername(context.Context, string) (*database.User, error) {
	return m.user, m.userErr
}
func (m *namesDBMock) UpdateUser(context.Context, *database.User) error     { return nil }
func (m *namesDBMock) DeleteUser(context.Context, uint) error               { return nil }
func (m *namesDBMock) ListUsers(context.Context) ([]*database.User, error)  { return nil, nil }
func (m *namesDBMock) CreateTenant(context.Context, *database.Tenant) error { return nil }
func (m *namesDBMock) GetTenantByName(context.Context, string) (*database.Tenant, error) {
	return m.tenant, m.tenantErr
}
func (m *namesDBMock) GetTenantByID(context.Context, uint) (*database.Tenant, error) { return nil, nil }
func (m *namesDBMock) UpdateTenant(context.Context, *database.Tenant) error          { return nil }
func (m *namesDBMock) DeleteTenant(context.Context, uint) error                      { return nil }
func (m *namesDBMock) ListTenants(context.Context) ([]*database.Tenant, error)       { return nil, nil }
func (m *namesDBMock) AddUserToTenant(context.Context, uint, uint) error             { return nil }
func (m *namesDBMock) RemoveUserFromTenant(context.Context, uint, uint) error        { return nil }
func (m *namesDBMock) GetUserTenants(context.Context, uint) ([]*database.Tenant, error) {
	return m.userTenants, m.utErr
}
func (m *namesDBMock) GetTenantUsers(context.Context, uint) ([]*database.User, error) {
	return nil, nil
}
func (m *namesDBMock) DeleteUserTenants(context.Context, uint) error { return nil }
func (m *namesDBMock) Transaction(ctx context.Context, fn func(ctx context.Context) error) error {
	return fn(ctx)
}
func (m *namesDBMock) GetSystemPrompt(context.Context, uint) (string, error) { return "", nil }
func (m *namesDBMock) SaveSystemPrompt(context.Context, uint, string) error  { return nil }

func TestMCP_HandleGetConfigNames_MissingClaims(t *testing.T) {
	gin.SetMode(gin.TestMode)
	st := &storeListMock{}
	h := &MCP{db: &namesDBMock{}, store: st, logger: zap.NewNop()}
	r := gin.New()
	r.GET("/names", h.HandleGetConfigNames)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/names", nil)
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestMCP_HandleGetConfigNames_StoreError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	st := &storeListMock{err: assert.AnError}
	db := &namesDBMock{user: &database.User{ID: 1, Username: "u", Role: database.RoleAdmin}}
	h := &MCP{db: db, store: st, logger: zap.NewNop()}
	r := gin.New()
	r.GET("/names", func(c *gin.Context) { c.Set("claims", &jsvc.Claims{Username: "u"}); h.HandleGetConfigNames(c) })

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/names", nil)
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestMCP_HandleGetConfigNames_AdminAndFilter(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfgs := []*config.MCPConfig{
		{Name: "a", Tenant: "t1"},
		{Name: "b", Tenant: "t2"},
		{Name: "c", Tenant: "t1"},
	}
	st := &storeListMock{cfgs: cfgs}
	db := &mcpDBMock{user: &database.User{ID: 1, Username: "admin", Role: database.RoleAdmin}, tenant: &database.Tenant{ID: 2, Name: "t1", Prefix: "/t1"}}
	h := &MCP{db: db, store: st, logger: zap.NewNop()}
	r := gin.New()
	r.GET("/names", func(c *gin.Context) { c.Set("claims", &jsvc.Claims{Username: "admin"}); h.HandleGetConfigNames(c) })

	// all
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/names", nil)
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	body := w.Body.String()
	assert.Contains(t, body, "\"a\"")
	assert.Contains(t, body, "\"b\"")
	assert.Contains(t, body, "\"c\"")

	// filter by tenant
	w2 := httptest.NewRecorder()
	req2, _ := http.NewRequest("GET", "/names?tenant=t1", nil)
	r.ServeHTTP(w2, req2)
	assert.Equal(t, http.StatusOK, w2.Code)
	body2 := w2.Body.String()
	assert.Contains(t, body2, "\"a\"")
	assert.Contains(t, body2, "\"c\"")
	assert.NotContains(t, body2, "\"b\"")
}

func TestMCP_HandleGetConfigNames_NonAdmin_FilterByUserTenants(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfgs := []*config.MCPConfig{
		{Name: "a", Tenant: "t1"},
		{Name: "b", Tenant: "t2"},
		{Name: "c", Tenant: "t3"},
	}
	st := &storeListMock{cfgs: cfgs}
	db := &namesDBMock{user: &database.User{ID: 1, Username: "u", Role: database.RoleNormal}, userTenants: []*database.Tenant{{Name: "t1"}, {Name: "t3"}}}
	h := &MCP{db: db, store: st, logger: zap.NewNop()}
	r := gin.New()
	r.GET("/names", func(c *gin.Context) { c.Set("claims", &jsvc.Claims{Username: "u"}); h.HandleGetConfigNames(c) })

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/names", nil)
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	body := w.Body.String()
	assert.Contains(t, body, "\"a\"")
	assert.Contains(t, body, "\"c\"")
	assert.NotContains(t, body, "\"b\"")
}

func TestMCP_HandleGetCapabilities_BadParams(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := &MCP{db: &namesDBMock{}, logger: zap.NewNop()}
	r := gin.New()
	r.GET("/cap-missing-tenant", func(c *gin.Context) { h.HandleGetCapabilities(c) })
	r.GET("/cap-missing-name/:tenant", func(c *gin.Context) {
		// only tenant provided
		h.HandleGetCapabilities(c)
	})

	// missing tenant and name
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/cap-missing-tenant", nil)
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)

	// missing name
	w2 := httptest.NewRecorder()
	req2, _ := http.NewRequest("GET", "/cap-missing-name/t1", nil)
	r.ServeHTTP(w2, req2)
	assert.Equal(t, http.StatusBadRequest, w2.Code)
}

func TestMCP_HandleGetCapabilities_NotFound(t *testing.T) {
	gin.SetMode(gin.TestMode)
	st := &storeGetMock{cfg: nil, err: assert.AnError}
	h := &MCP{db: &namesDBMock{}, store: st, logger: zap.NewNop()}
	r := gin.New()
	r.GET("/cap/:tenant/:name", h.HandleGetCapabilities)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/cap/t1/srv", nil)
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestMCP_HandleMCPServerCreate_InvalidYAML(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := &MCP{db: &namesDBMock{}, store: &storeGetMock{}, logger: zap.NewNop()}
	r := gin.New()
	r.POST("/create", h.HandleMCPServerCreate)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/create", strings.NewReader("not: [valid"))
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

type notifierMock struct {
	called bool
	err    error
}

func (n *notifierMock) Watch(ctx context.Context) (<-chan *config.MCPConfig, error) { return nil, nil }
func (n *notifierMock) NotifyUpdate(ctx context.Context, cfg *config.MCPConfig) error {
	n.called = true
	return n.err
}
func (n *notifierMock) CanReceive() bool { return false }
func (n *notifierMock) CanSend() bool    { return true }

// db mock for delete/sync paths
type mcpDBMock struct {
	user   *database.User
	tenant *database.Tenant
}

func (m *mcpDBMock) Close() error                                         { return nil }
func (m *mcpDBMock) SaveMessage(context.Context, *database.Message) error { return nil }
func (m *mcpDBMock) GetMessages(context.Context, string) ([]*database.Message, error) {
	return nil, nil
}
func (m *mcpDBMock) GetMessagesWithPagination(context.Context, string, int, int) ([]*database.Message, error) {
	return nil, nil
}
func (m *mcpDBMock) CreateSession(context.Context, string) error                  { return nil }
func (m *mcpDBMock) CreateSessionWithTitle(context.Context, string, string) error { return nil }
func (m *mcpDBMock) SessionExists(context.Context, string) (bool, error)          { return true, nil }
func (m *mcpDBMock) GetSessions(context.Context) ([]*database.Session, error)     { return nil, nil }
func (m *mcpDBMock) UpdateSessionTitle(context.Context, string, string) error     { return nil }
func (m *mcpDBMock) DeleteSession(context.Context, string) error                  { return nil }
func (m *mcpDBMock) CreateUser(context.Context, *database.User) error             { return nil }
func (m *mcpDBMock) GetUserByUsername(context.Context, string) (*database.User, error) {
	return m.user, nil
}
func (m *mcpDBMock) UpdateUser(context.Context, *database.User) error     { return nil }
func (m *mcpDBMock) DeleteUser(context.Context, uint) error               { return nil }
func (m *mcpDBMock) ListUsers(context.Context) ([]*database.User, error)  { return nil, nil }
func (m *mcpDBMock) CreateTenant(context.Context, *database.Tenant) error { return nil }
func (m *mcpDBMock) GetTenantByName(context.Context, string) (*database.Tenant, error) {
	return m.tenant, nil
}
func (m *mcpDBMock) GetTenantByID(context.Context, uint) (*database.Tenant, error) { return nil, nil }
func (m *mcpDBMock) UpdateTenant(context.Context, *database.Tenant) error          { return nil }
func (m *mcpDBMock) DeleteTenant(context.Context, uint) error                      { return nil }
func (m *mcpDBMock) ListTenants(context.Context) ([]*database.Tenant, error)       { return nil, nil }
func (m *mcpDBMock) AddUserToTenant(context.Context, uint, uint) error             { return nil }
func (m *mcpDBMock) RemoveUserFromTenant(context.Context, uint, uint) error        { return nil }
func (m *mcpDBMock) GetUserTenants(context.Context, uint) ([]*database.Tenant, error) {
	return []*database.Tenant{{ID: m.tenant.ID}}, nil
}
func (m *mcpDBMock) GetTenantUsers(context.Context, uint) ([]*database.User, error) { return nil, nil }
func (m *mcpDBMock) DeleteUserTenants(context.Context, uint) error                  { return nil }
func (m *mcpDBMock) Transaction(ctx context.Context, fn func(ctx context.Context) error) error {
	return fn(ctx)
}
func (m *mcpDBMock) GetSystemPrompt(context.Context, uint) (string, error) { return "", nil }
func (m *mcpDBMock) SaveSystemPrompt(context.Context, uint, string) error  { return nil }

type storeDeleteMock struct {
	storage.Store
	cfg    *config.MCPConfig
	getErr error
	delErr error
}

func (s *storeDeleteMock) Get(ctx context.Context, tenant, name string, includeDeleted ...bool) (*config.MCPConfig, error) {
	return s.cfg, s.getErr
}
func (s *storeDeleteMock) Delete(ctx context.Context, tenant, name string) error { return s.delErr }

func TestMCP_HandleMCPServerDelete_Paths(t *testing.T) {
	gin.SetMode(gin.TestMode)
	// missing tenant
	h := &MCP{db: &mcpDBMock{}, store: &storeDeleteMock{}, logger: zap.NewNop()}
	r := gin.New()
	r.DELETE("/del", h.HandleMCPServerDelete)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("DELETE", "/del", nil)
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)

	// missing name
	r2 := gin.New()
	r2.DELETE("/del/:tenant", h.HandleMCPServerDelete)
	w2 := httptest.NewRecorder()
	req2, _ := http.NewRequest("DELETE", "/del/t1", nil)
	r2.ServeHTTP(w2, req2)
	assert.Equal(t, http.StatusBadRequest, w2.Code)

	// not found
	stNF := &storeDeleteMock{cfg: nil, getErr: assert.AnError}
	r3 := gin.New()
	h3 := &MCP{db: &mcpDBMock{}, store: stNF, logger: zap.NewNop()}
	r3.DELETE("/del/:tenant/:name", h3.HandleMCPServerDelete)
	w3 := httptest.NewRecorder()
	req3, _ := http.NewRequest("DELETE", "/del/t1/srv", nil)
	r3.ServeHTTP(w3, req3)
	assert.Equal(t, http.StatusNotFound, w3.Code)

	// success
	stOK := &storeDeleteMock{cfg: &config.MCPConfig{Name: "srv", Tenant: "t1", Routers: []config.RouterConfig{{Prefix: "/t1"}}}}
	nt := &notifierMock{}
	db := &mcpDBMock{user: &database.User{ID: 1, Username: "admin", Role: database.RoleAdmin}, tenant: &database.Tenant{ID: 1, Name: "t1", Prefix: "/t1"}}
	h4 := &MCP{db: db, store: stOK, notifier: nt, logger: zap.NewNop()}
	r4 := gin.New()
	r4.DELETE("/del/:tenant/:name", func(c *gin.Context) { c.Set("claims", &jsvc.Claims{Username: "admin"}); h4.HandleMCPServerDelete(c) })
	w4 := httptest.NewRecorder()
	req4, _ := http.NewRequest("DELETE", "/del/t1/srv", nil)
	r4.ServeHTTP(w4, req4)
	assert.Equal(t, http.StatusOK, w4.Code)
	assert.True(t, nt.called)
}

func TestMCP_HandleMCPServerSync(t *testing.T) {
	gin.SetMode(gin.TestMode)
	// missing claims
	h := &MCP{db: &mcpDBMock{}, notifier: &notifierMock{}, logger: zap.NewNop()}
	r := gin.New()
	r.POST("/sync", h.HandleMCPServerSync)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/sync", nil)
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusUnauthorized, w.Code)

	// non-admin
	db := &mcpDBMock{user: &database.User{ID: 1, Username: "u", Role: database.RoleNormal}}
	h2 := &MCP{db: db, notifier: &notifierMock{}, logger: zap.NewNop()}
	r2 := gin.New()
	r2.POST("/sync", func(c *gin.Context) { c.Set("claims", &jsvc.Claims{Username: "u"}); h2.HandleMCPServerSync(c) })
	w2 := httptest.NewRecorder()
	req2, _ := http.NewRequest("POST", "/sync", nil)
	r2.ServeHTTP(w2, req2)
	assert.Equal(t, http.StatusUnauthorized, w2.Code)

	// admin success
	db3 := &mcpDBMock{user: &database.User{ID: 1, Username: "admin", Role: database.RoleAdmin}}
	nt := &notifierMock{}
	h3 := &MCP{db: db3, notifier: nt, logger: zap.NewNop()}
	r3 := gin.New()
	r3.POST("/sync", func(c *gin.Context) { c.Set("claims", &jsvc.Claims{Username: "admin"}); h3.HandleMCPServerSync(c) })
	w3 := httptest.NewRecorder()
	req3, _ := http.NewRequest("POST", "/sync", nil)
	r3.ServeHTTP(w3, req3)
	assert.Equal(t, http.StatusOK, w3.Code)
	assert.True(t, nt.called)
}
