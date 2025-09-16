package handler

import (
	"bytes"
	"context"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/amoylab/unla/internal/apiserver/database"
	"github.com/amoylab/unla/internal/auth/jwt"
	"github.com/amoylab/unla/internal/common/config"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

// dbMock implements database.Database with minimal behavior used by OpenAPI handler
type dbMock struct{}

func (db *dbMock) Close() error                                                     { return nil }
func (db *dbMock) SaveMessage(context.Context, *database.Message) error             { return nil }
func (db *dbMock) GetMessages(context.Context, string) ([]*database.Message, error) { return nil, nil }
func (db *dbMock) GetMessagesWithPagination(context.Context, string, int, int) ([]*database.Message, error) {
	return nil, nil
}
func (db *dbMock) CreateSession(context.Context, string) error                  { return nil }
func (db *dbMock) CreateSessionWithTitle(context.Context, string, string) error { return nil }
func (db *dbMock) SessionExists(context.Context, string) (bool, error)          { return true, nil }
func (db *dbMock) GetSessions(context.Context) ([]*database.Session, error)     { return nil, nil }
func (db *dbMock) UpdateSessionTitle(context.Context, string, string) error     { return nil }
func (db *dbMock) DeleteSession(context.Context, string) error                  { return nil }
func (db *dbMock) CreateUser(context.Context, *database.User) error             { return nil }
func (db *dbMock) GetUserByUsername(ctx context.Context, username string) (*database.User, error) {
	return &database.User{Username: username, Role: database.RoleAdmin}, nil
}
func (db *dbMock) UpdateUser(context.Context, *database.User) error     { return nil }
func (db *dbMock) DeleteUser(context.Context, uint) error               { return nil }
func (db *dbMock) ListUsers(context.Context) ([]*database.User, error)  { return nil, nil }
func (db *dbMock) CreateTenant(context.Context, *database.Tenant) error { return nil }
func (db *dbMock) GetTenantByName(ctx context.Context, name string) (*database.Tenant, error) {
	return &database.Tenant{Name: name, Prefix: "tenantp"}, nil
}
func (db *dbMock) GetTenantByID(context.Context, uint) (*database.Tenant, error)    { return nil, nil }
func (db *dbMock) UpdateTenant(context.Context, *database.Tenant) error             { return nil }
func (db *dbMock) DeleteTenant(context.Context, uint) error                         { return nil }
func (db *dbMock) ListTenants(context.Context) ([]*database.Tenant, error)          { return nil, nil }
func (db *dbMock) AddUserToTenant(context.Context, uint, uint) error                { return nil }
func (db *dbMock) RemoveUserFromTenant(context.Context, uint, uint) error           { return nil }
func (db *dbMock) GetUserTenants(context.Context, uint) ([]*database.Tenant, error) { return nil, nil }
func (db *dbMock) GetTenantUsers(context.Context, uint) ([]*database.User, error)   { return nil, nil }
func (db *dbMock) DeleteUserTenants(context.Context, uint) error                    { return nil }
func (db *dbMock) Transaction(ctx context.Context, fn func(ctx context.Context) error) error {
	return fn(ctx)
}
func (db *dbMock) GetSystemPrompt(context.Context, uint) (string, error) { return "", nil }
func (db *dbMock) SaveSystemPrompt(context.Context, uint, string) error  { return nil }

type storeMock struct{ created *config.MCPConfig }

func (s *storeMock) Create(ctx context.Context, cfg *config.MCPConfig) error {
	s.created = cfg
	return nil
}
func (s *storeMock) Get(context.Context, string, string, ...bool) (*config.MCPConfig, error) {
	return nil, nil
}
func (s *storeMock) List(context.Context, ...bool) ([]*config.MCPConfig, error) { return nil, nil }
func (s *storeMock) ListUpdated(context.Context, time.Time) ([]*config.MCPConfig, error) {
	return nil, nil
}
func (s *storeMock) Update(context.Context, *config.MCPConfig) error { return nil }
func (s *storeMock) Delete(context.Context, string, string) error    { return nil }
func (s *storeMock) GetVersion(context.Context, string, string, int) (*config.MCPConfigVersion, error) {
	return nil, nil
}
func (s *storeMock) ListVersions(context.Context, string, string) ([]*config.MCPConfigVersion, error) {
	return nil, nil
}
func (s *storeMock) DeleteVersion(context.Context, string, string, int) error    { return nil }
func (s *storeMock) SetActiveVersion(context.Context, string, string, int) error { return nil }

type openapiNotifierMock struct{ called bool }

func (n *openapiNotifierMock) Watch(context.Context) (<-chan *config.MCPConfig, error) {
	return nil, nil
}
func (n *openapiNotifierMock) NotifyUpdate(context.Context, *config.MCPConfig) error {
	n.called = true
	return nil
}
func (n *openapiNotifierMock) CanReceive() bool { return false }
func (n *openapiNotifierMock) CanSend() bool    { return true }

func newMultipart(t *testing.T, field, filename, content string) (*bytes.Buffer, string) {
	t.Helper()
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile(field, filename)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = part.Write([]byte(content))
	_ = writer.WriteField("tenantName", "tenantA")
	_ = writer.WriteField("prefix", "pref")
	_ = writer.Close()
	return body, writer.FormDataContentType()
}

func TestOpenAPI_HandleImport_Success(t *testing.T) {
	gin.SetMode(gin.TestMode)
	spec := "openapi: 3.0.0\ninfo:\n  title: T\n  version: '1'\npaths:\n  /ping:\n    get:\n      responses:\n        '200':\n          description: ok\n"
	body, ctype := newMultipart(t, "file", "spec.yaml", spec)

	r := gin.New()
	db := &dbMock{}
	st := &storeMock{}
	nt := &openapiNotifierMock{}
	h := NewOpenAPI(db, st, nt, zap.NewNop())
	r.POST("/import", func(c *gin.Context) {
		// inject claims
		c.Set("claims", &jwt.Claims{Username: "user"})
		h.HandleImport(c)
	})
	req := httptest.NewRequest("POST", "/import", body)
	req.Header.Set("Content-Type", ctype)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
	assert.True(t, nt.called)
	if assert.NotNil(t, st.created) {
		assert.Equal(t, "tenantA", st.created.Tenant)
	}
}

func TestOpenAPI_HandleImport_MissingClaims(t *testing.T) {
	gin.SetMode(gin.TestMode)
	spec := "openapi: 3.0.0\ninfo:\n  title: T\n  version: '1'\npaths:\n  /ping:\n    get:\n      responses:\n        '200':\n          description: ok\n"
	body, ctype := newMultipart(t, "file", "spec.yaml", spec)
	r := gin.New()
	h := NewOpenAPI(&dbMock{}, &storeMock{}, &openapiNotifierMock{}, zap.NewNop())
	r.POST("/import", h.HandleImport)
	req := httptest.NewRequest("POST", "/import", body)
	req.Header.Set("Content-Type", ctype)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestOpenAPI_HandleImport_MissingFile(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := NewOpenAPI(&dbMock{}, &storeMock{}, &openapiNotifierMock{}, zap.NewNop())
	r.POST("/import", h.HandleImport)
	req := httptest.NewRequest("POST", "/import", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}
