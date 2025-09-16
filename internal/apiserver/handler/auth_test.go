package handler

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"fmt"

	"github.com/amoylab/unla/internal/apiserver/database"
	"github.com/amoylab/unla/internal/auth/jwt"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

// fakeDB is a minimal stub to satisfy database.Database for handler tests.
type fakeDB struct {
	users []*database.User
}

func (f *fakeDB) Close() error                                               { return nil }
func (f *fakeDB) SaveMessage(ctx context.Context, m *database.Message) error { return nil }
func (f *fakeDB) GetMessages(ctx context.Context, sessionID string) ([]*database.Message, error) {
	return nil, nil
}
func (f *fakeDB) GetMessagesWithPagination(ctx context.Context, sessionID string, page, pageSize int) ([]*database.Message, error) {
	return nil, nil
}
func (f *fakeDB) CreateSession(ctx context.Context, sessionId string) error { return nil }
func (f *fakeDB) CreateSessionWithTitle(ctx context.Context, sessionId string, title string) error {
	return nil
}
func (f *fakeDB) SessionExists(ctx context.Context, sessionID string) (bool, error) {
	return false, nil
}
func (f *fakeDB) GetSessions(ctx context.Context) ([]*database.Session, error) { return nil, nil }
func (f *fakeDB) UpdateSessionTitle(ctx context.Context, sessionID string, title string) error {
	return nil
}
func (f *fakeDB) DeleteSession(ctx context.Context, sessionID string) error { return nil }
func (f *fakeDB) CreateUser(ctx context.Context, user *database.User) error { return nil }
func (f *fakeDB) GetUserByUsername(ctx context.Context, username string) (*database.User, error) {
	// return a simple active normal user by default for GetUserInfo
	return &database.User{ID: 10, Username: username, Role: database.RoleNormal, IsActive: true}, nil
}
func (f *fakeDB) UpdateUser(ctx context.Context, user *database.User) error       { return nil }
func (f *fakeDB) DeleteUser(ctx context.Context, id uint) error                   { return nil }
func (f *fakeDB) ListUsers(ctx context.Context) ([]*database.User, error)         { return f.users, nil }
func (f *fakeDB) CreateTenant(ctx context.Context, tenant *database.Tenant) error { return nil }
func (f *fakeDB) GetTenantByName(ctx context.Context, name string) (*database.Tenant, error) {
	return nil, nil
}
func (f *fakeDB) GetTenantByID(ctx context.Context, id uint) (*database.Tenant, error) {
	return nil, nil
}
func (f *fakeDB) UpdateTenant(ctx context.Context, tenant *database.Tenant) error       { return nil }
func (f *fakeDB) DeleteTenant(ctx context.Context, id uint) error                       { return nil }
func (f *fakeDB) ListTenants(ctx context.Context) ([]*database.Tenant, error)           { return nil, nil }
func (f *fakeDB) AddUserToTenant(ctx context.Context, userID, tenantID uint) error      { return nil }
func (f *fakeDB) RemoveUserFromTenant(ctx context.Context, userID, tenantID uint) error { return nil }
func (f *fakeDB) GetUserTenants(ctx context.Context, userID uint) ([]*database.Tenant, error) {
	return nil, nil
}
func (f *fakeDB) GetTenantUsers(ctx context.Context, tenantID uint) ([]*database.User, error) {
	return nil, nil
}
func (f *fakeDB) DeleteUserTenants(ctx context.Context, userID uint) error { return nil }
func (f *fakeDB) Transaction(ctx context.Context, fn func(ctx context.Context) error) error {
	return fn(ctx)
}
func (f *fakeDB) GetSystemPrompt(ctx context.Context, userID uint) (string, error)       { return "", nil }
func (f *fakeDB) SaveSystemPrompt(ctx context.Context, userID uint, prompt string) error { return nil }

func TestHandler_ListUsers(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	users := []*database.User{{ID: 1, Username: "a"}, {ID: 2, Username: "b"}}
	h := NewHandler(&fakeDB{users: users}, nil, nil, zap.NewNop())

	r.GET("/users", func(c *gin.Context) {
		// inject admin claims
		c.Set("claims", &jwt.Claims{Username: "admin", Role: "admin"})
		h.ListUsers(c)
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/users", nil)
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestHandler_GetUserInfo(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := NewHandler(&fakeDB{}, nil, nil, zap.NewNop())

	r.GET("/me", func(c *gin.Context) {
		claims := &jwt.Claims{UserID: 10, Username: "u", Role: "normal"}
		c.Set("claims", claims)
		h.GetUserInfo(c)
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/me", nil)
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

// fakeDBUpdate extends fakeDB to support UpdateUser flow with tenant ops tracking
type fakeDBUpdate struct {
	fakeDB
	updatedUser     *database.User
	existingTenants []*database.Tenant
	added           []uint
	removed         []uint
}

func (f *fakeDBUpdate) GetUserByUsername(ctx context.Context, username string) (*database.User, error) {
	return &database.User{ID: 1, Username: username, Role: database.RoleNormal, IsActive: true, Password: "x"}, nil
}
func (f *fakeDBUpdate) UpdateUser(ctx context.Context, user *database.User) error {
	f.updatedUser = user
	return nil
}
func (f *fakeDBUpdate) GetUserTenants(ctx context.Context, userID uint) ([]*database.Tenant, error) {
	return f.existingTenants, nil
}
func (f *fakeDBUpdate) RemoveUserFromTenant(ctx context.Context, userID, tenantID uint) error {
	f.removed = append(f.removed, tenantID)
	return nil
}
func (f *fakeDBUpdate) AddUserToTenant(ctx context.Context, userID, tenantID uint) error {
	f.added = append(f.added, tenantID)
	return nil
}

func TestHandler_UpdateUser(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	f := &fakeDBUpdate{existingTenants: []*database.Tenant{{ID: 1}}}
	h := NewHandler(f, nil, nil, zap.NewNop())

	r.PUT("/user", func(c *gin.Context) {
		body := `{"username":"u","role":"admin","isActive":true,"password":"new","tenantIds":[1,2]}`
		c.Request = c.Request.WithContext(context.Background())
		c.Request.Method = http.MethodPut
		c.Request.Body = io.NopCloser(strings.NewReader(body))
		c.Request.Header.Set("Content-Type", "application/json")
		h.UpdateUser(c)
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPut, "/user", nil)
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	// verify role updated and tenant 2 added
	assert.NotNil(t, f.updatedUser)
	assert.Equal(t, database.RoleAdmin, f.updatedUser.Role)
	assert.Contains(t, f.added, uint(2))
}

type fakeDBDelete struct {
	fakeDB
	deletedUserID uint
	delTenants    bool
}

func (f *fakeDBDelete) GetUserByUsername(ctx context.Context, username string) (*database.User, error) {
	return &database.User{ID: 2, Username: username}, nil
}
func (f *fakeDBDelete) DeleteUserTenants(ctx context.Context, userID uint) error {
	f.delTenants = true
	return nil
}
func (f *fakeDBDelete) DeleteUser(ctx context.Context, id uint) error {
	f.deletedUserID = id
	return nil
}

func TestHandler_DeleteUser(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	f := &fakeDBDelete{}
	h := NewHandler(f, nil, nil, zap.NewNop())

	r.DELETE("/users/:username", func(c *gin.Context) { h.DeleteUser(c) })

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodDelete, "/users/u", nil)
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, uint(2), f.deletedUserID)
	assert.True(t, f.delTenants)
}

type fakeDBCreate struct {
	fakeDB
	created *database.User
	added   []uint
}

func (f *fakeDBCreate) GetUserByUsername(ctx context.Context, username string) (*database.User, error) {
	return nil, fmt.Errorf("not found")
}
func (f *fakeDBCreate) CreateUser(ctx context.Context, user *database.User) error {
	user.ID = 9
	f.created = user
	return nil
}
func (f *fakeDBCreate) AddUserToTenant(ctx context.Context, userID, tenantID uint) error {
	f.added = append(f.added, tenantID)
	return nil
}

func TestHandler_CreateUser(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	f := &fakeDBCreate{}
	h := NewHandler(f, nil, nil, zap.NewNop())
	r.POST("/users", func(c *gin.Context) {
		c.Set("claims", &jwt.Claims{Username: "admin", Role: "admin"})
		body := `{"username":"nu","password":"pw","role":"normal","tenantIds":[7]}`
		c.Request = c.Request.WithContext(context.Background())
		c.Request.Method = http.MethodPost
		c.Request.Body = io.NopCloser(strings.NewReader(body))
		c.Request.Header.Set("Content-Type", "application/json")
		h.CreateUser(c)
	})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/users", nil)
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusCreated, w.Code)
	assert.NotNil(t, f.created)
	assert.Contains(t, f.added, uint(7))
}

type fakeDBTenants struct{ fakeDB }

func (f *fakeDBTenants) GetUserByUsername(ctx context.Context, username string) (*database.User, error) {
	return &database.User{ID: 3, Username: username, Role: database.RoleAdmin}, nil
}
func (f *fakeDBTenants) ListTenants(ctx context.Context) ([]*database.Tenant, error) {
	return []*database.Tenant{{ID: 1, Name: "t"}}, nil
}

func TestHandler_ListTenants(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := NewHandler(&fakeDBTenants{}, nil, nil, zap.NewNop())
	r.GET("/tenants", func(c *gin.Context) {
		c.Set("claims", &jwt.Claims{Username: "admin", Role: "admin"})
		h.ListTenants(c)
	})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/tenants", nil)
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}
