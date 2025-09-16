package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/amoylab/unla/internal/apiserver/database"
	jsvc "github.com/amoylab/unla/internal/auth/jwt"
	"github.com/amoylab/unla/internal/common/dto"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

type tenantDBMock struct {
	tenants []*database.Tenant
	users   map[string]*database.User
}

// Implement database.Database (only used methods meaningfully)
func (m *tenantDBMock) Close() error                                         { return nil }
func (m *tenantDBMock) SaveMessage(context.Context, *database.Message) error { return nil }
func (m *tenantDBMock) GetMessages(context.Context, string) ([]*database.Message, error) {
	return nil, nil
}
func (m *tenantDBMock) GetMessagesWithPagination(context.Context, string, int, int) ([]*database.Message, error) {
	return nil, nil
}
func (m *tenantDBMock) CreateSession(context.Context, string) error                  { return nil }
func (m *tenantDBMock) CreateSessionWithTitle(context.Context, string, string) error { return nil }
func (m *tenantDBMock) SessionExists(context.Context, string) (bool, error)          { return true, nil }
func (m *tenantDBMock) GetSessions(context.Context) ([]*database.Session, error)     { return nil, nil }
func (m *tenantDBMock) UpdateSessionTitle(context.Context, string, string) error     { return nil }
func (m *tenantDBMock) DeleteSession(context.Context, string) error                  { return nil }
func (m *tenantDBMock) CreateUser(ctx context.Context, u *database.User) error {
	if m.users == nil {
		m.users = map[string]*database.User{}
	}
	u.ID = uint(len(m.users) + 1)
	u.CreatedAt = time.Now()
	u.UpdatedAt = u.CreatedAt
	m.users[u.Username] = u
	return nil
}
func (m *tenantDBMock) GetUserByUsername(ctx context.Context, username string) (*database.User, error) {
	if m.users == nil {
		m.users = map[string]*database.User{}
	}
	if u, ok := m.users[username]; ok {
		return u, nil
	}
	return &database.User{Username: username, Role: database.RoleAdmin, IsActive: true}, nil
}
func (m *tenantDBMock) UpdateUser(context.Context, *database.User) error    { return nil }
func (m *tenantDBMock) DeleteUser(context.Context, uint) error              { return nil }
func (m *tenantDBMock) ListUsers(context.Context) ([]*database.User, error) { return nil, nil }
func (m *tenantDBMock) CreateTenant(ctx context.Context, t *database.Tenant) error {
	t.ID = uint(len(m.tenants) + 1)
	t.CreatedAt = time.Now()
	t.UpdatedAt = t.CreatedAt
	m.tenants = append(m.tenants, t)
	return nil
}
func (m *tenantDBMock) GetTenantByName(ctx context.Context, name string) (*database.Tenant, error) {
	for _, t := range m.tenants {
		if t.Name == name {
			return t, nil
		}
	}
	return nil, assert.AnError
}
func (m *tenantDBMock) GetTenantByID(ctx context.Context, id uint) (*database.Tenant, error) {
	for _, t := range m.tenants {
		if t.ID == id {
			return t, nil
		}
	}
	return nil, assert.AnError
}
func (m *tenantDBMock) UpdateTenant(ctx context.Context, t *database.Tenant) error {
	for i, x := range m.tenants {
		if x.ID == t.ID {
			m.tenants[i] = t
			return nil
		}
	}
	return nil
}
func (m *tenantDBMock) DeleteTenant(ctx context.Context, id uint) error {
	out := make([]*database.Tenant, 0, len(m.tenants))
	for _, t := range m.tenants {
		if t.ID != id {
			out = append(out, t)
		}
	}
	m.tenants = out
	return nil
}
func (m *tenantDBMock) ListTenants(context.Context) ([]*database.Tenant, error) {
	return m.tenants, nil
}
func (m *tenantDBMock) AddUserToTenant(context.Context, uint, uint) error      { return nil }
func (m *tenantDBMock) RemoveUserFromTenant(context.Context, uint, uint) error { return nil }
func (m *tenantDBMock) GetUserTenants(context.Context, uint) ([]*database.Tenant, error) {
	if len(m.tenants) == 0 {
		return []*database.Tenant{}, nil
	}
	return []*database.Tenant{m.tenants[0]}, nil
}
func (m *tenantDBMock) GetTenantUsers(context.Context, uint) ([]*database.User, error) {
	return nil, nil
}
func (m *tenantDBMock) DeleteUserTenants(context.Context, uint) error { return nil }
func (m *tenantDBMock) Transaction(ctx context.Context, fn func(ctx context.Context) error) error {
	return fn(ctx)
}
func (m *tenantDBMock) GetSystemPrompt(context.Context, uint) (string, error) { return "", nil }
func (m *tenantDBMock) SaveSystemPrompt(context.Context, uint, string) error  { return nil }

func withClaims(r *gin.Engine, route string, handler gin.HandlerFunc, c *jsvc.Claims) (*httptest.ResponseRecorder, *http.Request) {
	r.POST(route, func(ctx *gin.Context) {
		ctx.Set("claims", c)
		handler(ctx)
	})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", route, nil)
	return w, req
}

func TestListTenants_AdminAndUser(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := &tenantDBMock{tenants: []*database.Tenant{{ID: 1, Name: "t1", Prefix: "/t1"}, {ID: 2, Name: "t2", Prefix: "/t2"}}}
	h := &Handler{db: db, logger: zap.NewNop()}
	r := gin.New()

	// admin sees all
	r.GET("/tenants", func(c *gin.Context) {
		c.Set("claims", &jsvc.Claims{Username: "admin", Role: "admin"})
		h.ListTenants(c)
	})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/tenants", nil)
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	// normal user sees assigned
	r2 := gin.New()
	r2.GET("/tenants", func(c *gin.Context) {
		c.Set("claims", &jsvc.Claims{Username: "user", Role: "normal"})
		h.ListTenants(c)
	})
	w2 := httptest.NewRecorder()
	req2, _ := http.NewRequest("GET", "/tenants", nil)
	r2.ServeHTTP(w2, req2)
	assert.Equal(t, http.StatusOK, w2.Code)
}

func TestCreateUpdateDeleteTenant(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := &tenantDBMock{}
	h := &Handler{db: db, logger: zap.NewNop()}

	// create
	r := gin.New()
	r.POST("/tenant", func(c *gin.Context) { h.CreateTenant(c) })
	body, _ := json.Marshal(&dto.CreateTenantRequest{Name: "t1", Prefix: "/t1"})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/tenant", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusCreated, w.Code)

	// update
	r2 := gin.New()
	r2.PUT("/tenant", func(c *gin.Context) { h.UpdateTenant(c) })
	updBody, _ := json.Marshal(&dto.UpdateTenantRequest{Name: "t1", Prefix: "/t1x"})
	w2 := httptest.NewRecorder()
	req2, _ := http.NewRequest("PUT", "/tenant", bytes.NewReader(updBody))
	req2.Header.Set("Content-Type", "application/json")
	r2.ServeHTTP(w2, req2)
	assert.Equal(t, http.StatusOK, w2.Code)

	// delete
	r3 := gin.New()
	r3.DELETE("/tenant/:name", func(c *gin.Context) { h.DeleteTenant(c) })
	w3 := httptest.NewRecorder()
	req3, _ := http.NewRequest("DELETE", "/tenant/t1", nil)
	r3.ServeHTTP(w3, req3)
	assert.Equal(t, http.StatusOK, w3.Code)
}
