package database

import (
	"context"
)

// Database defines the methods for database operations.
type Database interface {
	// Close closes the database connection.
	Close() error

	CreateUser(ctx context.Context, user *User) error
	GetUserByUsername(ctx context.Context, username string) (*User, error)
	UpdateUser(ctx context.Context, user *User) error
	DeleteUser(ctx context.Context, id uint) error
	ListUsers(ctx context.Context) ([]*User, error)

	CreateTenant(ctx context.Context, tenant *Tenant) error
	GetTenantByName(ctx context.Context, name string) (*Tenant, error)
	GetTenantByID(ctx context.Context, id uint) (*Tenant, error)
	UpdateTenant(ctx context.Context, tenant *Tenant) error
	DeleteTenant(ctx context.Context, id uint) error
	ListTenants(ctx context.Context) ([]*Tenant, error)

	AddUserToTenant(ctx context.Context, userID, tenantID uint) error
	RemoveUserFromTenant(ctx context.Context, userID, tenantID uint) error
	GetUserTenants(ctx context.Context, userID uint) ([]*Tenant, error)
	GetTenantUsers(ctx context.Context, tenantID uint) ([]*User, error)
	DeleteUserTenants(ctx context.Context, userID uint) error

	Transaction(ctx context.Context, fn func(ctx context.Context) error) error
}
