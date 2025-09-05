package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/amoylab/unla/internal/apiserver/database"
	"github.com/amoylab/unla/internal/auth/jwt"
	"github.com/amoylab/unla/internal/common/config"
	"github.com/amoylab/unla/internal/mcp/storage"
	"github.com/amoylab/unla/pkg/mcp"
)

// MockDatabase is a mock implementation of database.Database interface
type MockDatabase struct {
	mock.Mock
}

func (m *MockDatabase) Close() error {
	args := m.Called()
	return args.Error(0)
}

func (m *MockDatabase) SaveMessage(ctx context.Context, message *database.Message) error {
	args := m.Called(ctx, message)
	return args.Error(0)
}

func (m *MockDatabase) GetMessages(ctx context.Context, sessionID string) ([]*database.Message, error) {
	args := m.Called(ctx, sessionID)
	return args.Get(0).([]*database.Message), args.Error(1)
}

func (m *MockDatabase) GetMessagesWithPagination(ctx context.Context, sessionID string, page, pageSize int) ([]*database.Message, error) {
	args := m.Called(ctx, sessionID, page, pageSize)
	return args.Get(0).([]*database.Message), args.Error(1)
}

func (m *MockDatabase) CreateSession(ctx context.Context, sessionId string) error {
	args := m.Called(ctx, sessionId)
	return args.Error(0)
}

func (m *MockDatabase) CreateSessionWithTitle(ctx context.Context, sessionId string, title string) error {
	args := m.Called(ctx, sessionId, title)
	return args.Error(0)
}

func (m *MockDatabase) SessionExists(ctx context.Context, sessionID string) (bool, error) {
	args := m.Called(ctx, sessionID)
	return args.Bool(0), args.Error(1)
}

func (m *MockDatabase) GetSessions(ctx context.Context) ([]*database.Session, error) {
	args := m.Called(ctx)
	return args.Get(0).([]*database.Session), args.Error(1)
}

func (m *MockDatabase) UpdateSessionTitle(ctx context.Context, sessionID string, title string) error {
	args := m.Called(ctx, sessionID, title)
	return args.Error(0)
}

func (m *MockDatabase) DeleteSession(ctx context.Context, sessionID string) error {
	args := m.Called(ctx, sessionID)
	return args.Error(0)
}

func (m *MockDatabase) CreateUser(ctx context.Context, user *database.User) error {
	args := m.Called(ctx, user)
	return args.Error(0)
}

func (m *MockDatabase) GetUserByUsername(ctx context.Context, username string) (*database.User, error) {
	args := m.Called(ctx, username)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*database.User), args.Error(1)
}

func (m *MockDatabase) GetUserByID(ctx context.Context, id uint) (*database.User, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*database.User), args.Error(1)
}

func (m *MockDatabase) UpdateUser(ctx context.Context, user *database.User) error {
	args := m.Called(ctx, user)
	return args.Error(0)
}

func (m *MockDatabase) DeleteUser(ctx context.Context, id uint) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockDatabase) ListUsers(ctx context.Context) ([]*database.User, error) {
	args := m.Called(ctx)
	return args.Get(0).([]*database.User), args.Error(1)
}

func (m *MockDatabase) CreateTenant(ctx context.Context, tenant *database.Tenant) error {
	args := m.Called(ctx, tenant)
	return args.Error(0)
}

func (m *MockDatabase) GetTenantByName(ctx context.Context, name string) (*database.Tenant, error) {
	args := m.Called(ctx, name)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*database.Tenant), args.Error(1)
}

func (m *MockDatabase) GetTenantByID(ctx context.Context, id uint) (*database.Tenant, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*database.Tenant), args.Error(1)
}

func (m *MockDatabase) UpdateTenant(ctx context.Context, tenant *database.Tenant) error {
	args := m.Called(ctx, tenant)
	return args.Error(0)
}

func (m *MockDatabase) DeleteTenant(ctx context.Context, id uint) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockDatabase) ListTenants(ctx context.Context) ([]*database.Tenant, error) {
	args := m.Called(ctx)
	return args.Get(0).([]*database.Tenant), args.Error(1)
}

func (m *MockDatabase) AddUserToTenant(ctx context.Context, userID, tenantID uint) error {
	args := m.Called(ctx, userID, tenantID)
	return args.Error(0)
}

func (m *MockDatabase) RemoveUserFromTenant(ctx context.Context, userID, tenantID uint) error {
	args := m.Called(ctx, userID, tenantID)
	return args.Error(0)
}

func (m *MockDatabase) GetUserTenants(ctx context.Context, userID uint) ([]*database.Tenant, error) {
	args := m.Called(ctx, userID)
	return args.Get(0).([]*database.Tenant), args.Error(1)
}

func (m *MockDatabase) GetTenantUsers(ctx context.Context, tenantID uint) ([]*database.User, error) {
	args := m.Called(ctx, tenantID)
	return args.Get(0).([]*database.User), args.Error(1)
}

func (m *MockDatabase) DeleteUserTenants(ctx context.Context, userID uint) error {
	args := m.Called(ctx, userID)
	return args.Error(0)
}

func (m *MockDatabase) Transaction(ctx context.Context, fn func(ctx context.Context) error) error {
	args := m.Called(ctx, fn)
	return args.Error(0)
}

func (m *MockDatabase) GetSystemPrompt(ctx context.Context, userID uint) (string, error) {
	args := m.Called(ctx, userID)
	return args.String(0), args.Error(1)
}

func (m *MockDatabase) SaveSystemPrompt(ctx context.Context, userID uint, prompt string) error {
	args := m.Called(ctx, userID, prompt)
	return args.Error(0)
}

// MockStore is a mock implementation of storage.Store interface
type MockStore struct {
	mock.Mock
}

func (m *MockStore) Get(ctx context.Context, tenant, name string) (*config.MCPConfig, error) {
	args := m.Called(ctx, tenant, name)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*config.MCPConfig), args.Error(1)
}

func (m *MockStore) Create(ctx context.Context, cfg *config.MCPConfig) error {
	args := m.Called(ctx, cfg)
	return args.Error(0)
}

func (m *MockStore) Update(ctx context.Context, cfg *config.MCPConfig) error {
	args := m.Called(ctx, cfg)
	return args.Error(0)
}

func (m *MockStore) Delete(ctx context.Context, tenant, name string) error {
	args := m.Called(ctx, tenant, name)
	return args.Error(0)
}

func (m *MockStore) List(ctx context.Context, includeDeleted ...bool) ([]*config.MCPConfig, error) {
	args := m.Called(ctx, includeDeleted)
	return args.Get(0).([]*config.MCPConfig), args.Error(1)
}

func (m *MockStore) ListVersions(ctx context.Context, tenant, name string) ([]*config.MCPConfigVersion, error) {
	args := m.Called(ctx, tenant, name)
	return args.Get(0).([]*config.MCPConfigVersion), args.Error(1)
}

func (m *MockStore) SetActiveVersion(ctx context.Context, tenant, name string, version int) error {
	args := m.Called(ctx, tenant, name, version)
	return args.Error(0)
}

// MockCapabilityStore is a mock implementation of storage.CapabilityStore interface
type MockCapabilityStore struct {
	mock.Mock
}

func (m *MockCapabilityStore) SaveTool(ctx context.Context, tool *mcp.MCPTool, tenant, serverName string) error {
	args := m.Called(ctx, tool, tenant, serverName)
	return args.Error(0)
}

func (m *MockCapabilityStore) GetTool(ctx context.Context, tenant, serverName, name string) (*mcp.MCPTool, error) {
	args := m.Called(ctx, tenant, serverName, name)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*mcp.MCPTool), args.Error(1)
}

func (m *MockCapabilityStore) UpdateToolStatus(ctx context.Context, tenant, serverName, name string, enabled bool) error {
	args := m.Called(ctx, tenant, serverName, name, enabled)
	return args.Error(0)
}

func (m *MockCapabilityStore) RecordToolStatusChange(ctx context.Context, tenant, serverName, toolName string, oldStatus, newStatus bool, userID uint, reason string) error {
	args := m.Called(ctx, tenant, serverName, toolName, oldStatus, newStatus, userID, reason)
	return args.Error(0)
}

func (m *MockCapabilityStore) ListTools(ctx context.Context, tenant, serverName string) ([]mcp.MCPTool, error) {
	args := m.Called(ctx, tenant, serverName)
	return args.Get(0).([]mcp.MCPTool), args.Error(1)
}

func (m *MockCapabilityStore) DeleteTool(ctx context.Context, tenant, serverName, name string) error {
	args := m.Called(ctx, tenant, serverName, name)
	return args.Error(0)
}

func (m *MockCapabilityStore) SyncTools(ctx context.Context, tools []mcp.MCPTool, tenant, serverName string) error {
	args := m.Called(ctx, tools, tenant, serverName)
	return args.Error(0)
}

func (m *MockCapabilityStore) BatchUpdateToolStatus(ctx context.Context, tenant, serverName string, updates []storage.ToolStatusUpdate) error {
	args := m.Called(ctx, tenant, serverName, updates)
	return args.Error(0)
}

func (m *MockCapabilityStore) GetToolStatusHistory(ctx context.Context, tenant, serverName, toolName string, limit, offset int) ([]*storage.ToolStatusHistoryModel, error) {
	args := m.Called(ctx, tenant, serverName, toolName, limit, offset)
	return args.Get(0).([]*storage.ToolStatusHistoryModel), args.Error(1)
}

// Add other methods to satisfy the interface
func (m *MockCapabilityStore) SavePrompt(ctx context.Context, prompt *mcp.MCPPrompt, tenant, serverName string) error {
	args := m.Called(ctx, prompt, tenant, serverName)
	return args.Error(0)
}

func (m *MockCapabilityStore) GetPrompt(ctx context.Context, tenant, serverName, name string) (*mcp.MCPPrompt, error) {
	args := m.Called(ctx, tenant, serverName, name)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*mcp.MCPPrompt), args.Error(1)
}

func (m *MockCapabilityStore) ListPrompts(ctx context.Context, tenant, serverName string) ([]mcp.MCPPrompt, error) {
	args := m.Called(ctx, tenant, serverName)
	return args.Get(0).([]mcp.MCPPrompt), args.Error(1)
}

func (m *MockCapabilityStore) DeletePrompt(ctx context.Context, tenant, serverName, name string) error {
	args := m.Called(ctx, tenant, serverName, name)
	return args.Error(0)
}

func (m *MockCapabilityStore) SyncPrompts(ctx context.Context, prompts []mcp.MCPPrompt, tenant, serverName string) error {
	args := m.Called(ctx, prompts, tenant, serverName)
	return args.Error(0)
}

func (m *MockCapabilityStore) SaveResource(ctx context.Context, resource *mcp.MCPResource, tenant, serverName string) error {
	args := m.Called(ctx, resource, tenant, serverName)
	return args.Error(0)
}

func (m *MockCapabilityStore) GetResource(ctx context.Context, tenant, serverName, uri string) (*mcp.MCPResource, error) {
	args := m.Called(ctx, tenant, serverName, uri)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*mcp.MCPResource), args.Error(1)
}

func (m *MockCapabilityStore) ListResources(ctx context.Context, tenant, serverName string) ([]mcp.MCPResource, error) {
	args := m.Called(ctx, tenant, serverName)
	return args.Get(0).([]mcp.MCPResource), args.Error(1)
}

func (m *MockCapabilityStore) DeleteResource(ctx context.Context, tenant, serverName, uri string) error {
	args := m.Called(ctx, tenant, serverName, uri)
	return args.Error(0)
}

func (m *MockCapabilityStore) SyncResources(ctx context.Context, resources []mcp.MCPResource, tenant, serverName string) error {
	args := m.Called(ctx, resources, tenant, serverName)
	return args.Error(0)
}

func (m *MockCapabilityStore) SaveResourceTemplate(ctx context.Context, template *mcp.MCPResourceTemplate, tenant, serverName string) error {
	args := m.Called(ctx, template, tenant, serverName)
	return args.Error(0)
}

func (m *MockCapabilityStore) GetResourceTemplate(ctx context.Context, tenant, serverName, uriTemplate string) (*mcp.MCPResourceTemplate, error) {
	args := m.Called(ctx, tenant, serverName, uriTemplate)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*mcp.MCPResourceTemplate), args.Error(1)
}

func (m *MockCapabilityStore) ListResourceTemplates(ctx context.Context, tenant, serverName string) ([]mcp.MCPResourceTemplate, error) {
	args := m.Called(ctx, tenant, serverName)
	return args.Get(0).([]mcp.MCPResourceTemplate), args.Error(1)
}

func (m *MockCapabilityStore) DeleteResourceTemplate(ctx context.Context, tenant, serverName, uriTemplate string) error {
	args := m.Called(ctx, tenant, serverName, uriTemplate)
	return args.Error(0)
}

func (m *MockCapabilityStore) SyncResourceTemplates(ctx context.Context, templates []mcp.MCPResourceTemplate, tenant, serverName string) error {
	args := m.Called(ctx, templates, tenant, serverName)
	return args.Error(0)
}

func (m *MockCapabilityStore) GetCapabilitiesInfo(ctx context.Context, tenant, serverName string) (*mcp.CapabilitiesInfo, error) {
	args := m.Called(ctx, tenant, serverName)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*mcp.CapabilitiesInfo), args.Error(1)
}

func (m *MockCapabilityStore) SyncCapabilities(ctx context.Context, info *mcp.CapabilitiesInfo, tenant, serverName string) error {
	args := m.Called(ctx, info, tenant, serverName)
	return args.Error(0)
}

func (m *MockCapabilityStore) CleanupServerCapabilities(ctx context.Context, tenant, serverName string) error {
	args := m.Called(ctx, tenant, serverName)
	return args.Error(0)
}

func (m *MockCapabilityStore) CreateSyncRecord(ctx context.Context, record *storage.SyncHistoryModel) error {
	args := m.Called(ctx, record)
	return args.Error(0)
}

func (m *MockCapabilityStore) UpdateSyncRecord(ctx context.Context, syncID string, status storage.SyncStatus, progress int, errorMessage, summary string) error {
	args := m.Called(ctx, syncID, status, progress, errorMessage, summary)
	return args.Error(0)
}

func (m *MockCapabilityStore) CompleteSyncRecord(ctx context.Context, syncID string, status storage.SyncStatus, summary string) error {
	args := m.Called(ctx, syncID, status, summary)
	return args.Error(0)
}

func (m *MockCapabilityStore) GetSyncRecord(ctx context.Context, syncID string) (*storage.SyncHistoryModel, error) {
	args := m.Called(ctx, syncID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*storage.SyncHistoryModel), args.Error(1)
}

func (m *MockCapabilityStore) ListSyncHistory(ctx context.Context, tenant, serverName string, limit, offset int) ([]*storage.SyncHistoryModel, error) {
	args := m.Called(ctx, tenant, serverName, limit, offset)
	return args.Get(0).([]*storage.SyncHistoryModel), args.Error(1)
}

// MockNotifier is a mock implementation of notifier.Notifier interface
type MockNotifier struct {
	mock.Mock
}

func (m *MockNotifier) NotifyUpdate(ctx context.Context, cfg *config.MCPConfig) error {
	args := m.Called(ctx, cfg)
	return args.Error(0)
}

func (m *MockNotifier) Start() error {
	args := m.Called()
	return args.Error(0)
}

func (m *MockNotifier) Stop() error {
	args := m.Called()
	return args.Error(0)
}

func TestHandleUpdateToolStatus(t *testing.T) {
	gin.SetMode(gin.TestMode)
	
	// Create mocks
	mockDB := &MockDatabase{}
	mockStore := &MockStore{}
	mockCapabilityStore := &MockCapabilityStore{}
	mockNotifier := &MockNotifier{}
	
	// Create MCP handler
	handler := &MCP{
		db:              mockDB,
		store:           mockStore,
		capabilityStore: mockCapabilityStore,
		notifier:        mockNotifier,
	}
	
	// Setup test data
	tenant := "test-tenant"
	serverName := "test-server"
	toolName := "test-tool"
	
	testUser := &database.User{
		ID:       1,
		Username: "testuser",
		Role:     database.RoleUser,
	}
	
	testTenant := &database.Tenant{
		ID:     1,
		Name:   tenant,
		Prefix: "/test",
	}
	
	testConfig := &config.MCPConfig{
		Name:   serverName,
		Tenant: tenant,
		Routers: []config.RouterConfig{
			{Prefix: "/test/router"},
		},
	}
	
	existingTool := &mcp.MCPTool{
		Name:    toolName,
		Enabled: false,
	}
	
	// Setup expectations
	mockStore.On("Get", mock.Anything, tenant, serverName).Return(testConfig, nil)
	mockDB.On("GetUserByUsername", mock.Anything, "testuser").Return(testUser, nil)
	mockDB.On("GetTenantByName", mock.Anything, tenant).Return(testTenant, nil)
	mockDB.On("GetUserTenants", mock.Anything, testUser.ID).Return([]*database.Tenant{testTenant}, nil)
	mockCapabilityStore.On("GetTool", mock.Anything, tenant, serverName, toolName).Return(existingTool, nil)
	mockCapabilityStore.On("UpdateToolStatus", mock.Anything, tenant, serverName, toolName, true).Return(nil)
	mockCapabilityStore.On("RecordToolStatusChange", mock.Anything, tenant, serverName, toolName, false, true, testUser.ID, "").Return(nil)
	mockNotifier.On("NotifyUpdate", mock.Anything, testConfig).Return(nil)
	
	// Create request
	requestBody := UpdateToolStatusRequest{
		Enabled: true,
	}
	
	body, _ := json.Marshal(requestBody)
	req := httptest.NewRequest(http.MethodPut, "/api/mcp/capabilities/test-tenant/test-server/tools/test-tool/status", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	
	// Create response recorder
	w := httptest.NewRecorder()
	
	// Create Gin context
	c, _ := gin.CreateTestContext(w)
	c.Request = req
	c.Params = gin.Params{
		{Key: "tenant", Value: tenant},
		{Key: "name", Value: serverName},
		{Key: "toolName", Value: toolName},
	}
	
	// Set JWT claims
	claims := &jwt.Claims{
		Username: "testuser",
	}
	c.Set("claims", claims)
	
	// Call the handler
	handler.HandleUpdateToolStatus(c)
	
	// Assert response
	assert.Equal(t, http.StatusOK, w.Code)
	
	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	
	assert.True(t, response["success"].(bool))
	
	data := response["data"].(map[string]interface{})
	assert.Equal(t, toolName, data["toolName"])
	assert.True(t, data["enabled"].(bool))
	
	// Verify all mocks were called
	mockDB.AssertExpectations(t)
	mockStore.AssertExpectations(t)
	mockCapabilityStore.AssertExpectations(t)
	mockNotifier.AssertExpectations(t)
}