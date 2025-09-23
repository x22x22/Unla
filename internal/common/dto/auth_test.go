package dto

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLoginRequest(t *testing.T) {
	req := LoginRequest{
		Username: "testuser",
		Password: "testpass",
	}

	assert.Equal(t, "testuser", req.Username)
	assert.Equal(t, "testpass", req.Password)
}

func TestLoginResponse(t *testing.T) {
	resp := LoginResponse{
		Token: "jwt-token-here",
	}

	assert.Equal(t, "jwt-token-here", resp.Token)
}

func TestInitializeRequest(t *testing.T) {
	req := InitializeRequest{
		Username: "admin",
		Password: "admin123",
	}

	assert.Equal(t, "admin", req.Username)
	assert.Equal(t, "admin123", req.Password)
}

func TestChangePasswordRequest(t *testing.T) {
	req := ChangePasswordRequest{
		OldPassword: "oldpass",
		NewPassword: "newpass",
	}

	assert.Equal(t, "oldpass", req.OldPassword)
	assert.Equal(t, "newpass", req.NewPassword)
}

func TestChangePasswordResponse(t *testing.T) {
	t.Run("successful response", func(t *testing.T) {
		resp := ChangePasswordResponse{Success: true}
		assert.True(t, resp.Success)
	})

	t.Run("failure response", func(t *testing.T) {
		resp := ChangePasswordResponse{Success: false}
		assert.False(t, resp.Success)
	})
}

func TestCreateUserRequest(t *testing.T) {
	req := CreateUserRequest{
		Username:  "newuser",
		Password:  "password123",
		Role:      "admin",
		TenantIDs: []uint{1, 2, 3},
	}

	assert.Equal(t, "newuser", req.Username)
	assert.Equal(t, "password123", req.Password)
	assert.Equal(t, "admin", req.Role)
	assert.Equal(t, []uint{1, 2, 3}, req.TenantIDs)
}

func TestUpdateUserRequest(t *testing.T) {
	isActive := true
	req := UpdateUserRequest{
		Username:  "updateuser",
		Password:  "newpass",
		Role:      "normal",
		IsActive:  &isActive,
		TenantIDs: []uint{4, 5},
	}

	assert.Equal(t, "updateuser", req.Username)
	assert.Equal(t, "newpass", req.Password)
	assert.Equal(t, "normal", req.Role)
	assert.NotNil(t, req.IsActive)
	assert.True(t, *req.IsActive)
	assert.Equal(t, []uint{4, 5}, req.TenantIDs)
}

func TestUserInfo(t *testing.T) {
	user := UserInfo{
		ID:       1,
		Username: "testuser",
		Role:     "admin",
	}

	assert.Equal(t, uint(1), user.ID)
	assert.Equal(t, "testuser", user.Username)
	assert.Equal(t, "admin", user.Role)
}

func TestCreateTenantRequest(t *testing.T) {
	req := CreateTenantRequest{
		Name:        "Test Tenant",
		Prefix:      "test",
		Description: "A test tenant",
	}

	assert.Equal(t, "Test Tenant", req.Name)
	assert.Equal(t, "test", req.Prefix)
	assert.Equal(t, "A test tenant", req.Description)
}

func TestUpdateTenantRequest(t *testing.T) {
	isActive := false
	req := UpdateTenantRequest{
		Name:        "Updated Tenant",
		Prefix:      "updated",
		Description: "Updated description",
		IsActive:    &isActive,
	}

	assert.Equal(t, "Updated Tenant", req.Name)
	assert.Equal(t, "updated", req.Prefix)
	assert.Equal(t, "Updated description", req.Description)
	assert.NotNil(t, req.IsActive)
	assert.False(t, *req.IsActive)
}

func TestTenantResponse(t *testing.T) {
	resp := TenantResponse{
		ID:          1,
		Name:        "Tenant Name",
		Prefix:      "prefix",
		Description: "Description",
		IsActive:    true,
	}

	assert.Equal(t, uint(1), resp.ID)
	assert.Equal(t, "Tenant Name", resp.Name)
	assert.Equal(t, "prefix", resp.Prefix)
	assert.Equal(t, "Description", resp.Description)
	assert.True(t, resp.IsActive)
}

func TestUserResponse(t *testing.T) {
	tenants := []*TenantResponse{
		{ID: 1, Name: "Tenant 1"},
		{ID: 2, Name: "Tenant 2"},
	}

	resp := UserResponse{
		ID:       1,
		Username: "testuser",
		Role:     "admin",
		IsActive: true,
		Tenants:  tenants,
	}

	assert.Equal(t, uint(1), resp.ID)
	assert.Equal(t, "testuser", resp.Username)
	assert.Equal(t, "admin", resp.Role)
	assert.True(t, resp.IsActive)
	assert.Len(t, resp.Tenants, 2)
	assert.Equal(t, "Tenant 1", resp.Tenants[0].Name)
	assert.Equal(t, "Tenant 2", resp.Tenants[1].Name)
}

func TestUserTenantRequest(t *testing.T) {
	req := UserTenantRequest{
		UserID:    1,
		TenantIDs: []uint{2, 3, 4},
	}

	assert.Equal(t, uint(1), req.UserID)
	assert.Equal(t, []uint{2, 3, 4}, req.TenantIDs)
}

func TestTenantUserRequest(t *testing.T) {
	req := TenantUserRequest{
		TenantID: 1,
		UserIDs:  []uint{2, 3, 4},
	}

	assert.Equal(t, uint(1), req.TenantID)
	assert.Equal(t, []uint{2, 3, 4}, req.UserIDs)
}
