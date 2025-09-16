package database

import (
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"gorm.io/gorm"
)

func TestInitDefaultTenant_IdempotentAndGrantsAdmins(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&User{}, &Tenant{}, &UserTenant{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	// create an admin and a normal user
	admin := &User{Username: "admin", Role: RoleAdmin}
	normal := &User{Username: "normal", Role: RoleNormal}
	assert.NoError(t, db.Create(admin).Error)
	assert.NoError(t, db.Create(normal).Error)

	// first init
	assert.NoError(t, InitDefaultTenant(db))

	var tenant Tenant
	assert.NoError(t, db.Where("name = ?", "default").First(&tenant).Error)
	assert.NotZero(t, tenant.ID)

	// admin should be granted
	var rel UserTenant
	assert.NoError(t, db.Where("user_id = ? AND tenant_id = ?", admin.ID, tenant.ID).First(&rel).Error)

	// running again should be idempotent
	assert.NoError(t, InitDefaultTenant(db))
}
