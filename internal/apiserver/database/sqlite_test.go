package database

import (
	"context"
	"testing"
	"time"

	"github.com/amoylab/unla/internal/common/config"
	"github.com/stretchr/testify/assert"
)

func newTestSQLite(t *testing.T) *SQLite {
	t.Helper()
	cfg := &config.DatabaseConfig{Type: "sqlite", DBName: ":memory:"}
	dbi, err := NewSQLite(cfg)
	if err != nil {
		t.Fatalf("NewSQLite: %v", err)
	}
	return dbi.(*SQLite)
}

func TestSQLite_SessionsAndMessages(t *testing.T) {
	db := newTestSQLite(t)
	ctx := context.Background()

	// sessions
	assert.NoError(t, db.CreateSession(ctx, "s1"))
	assert.NoError(t, db.CreateSessionWithTitle(ctx, "s2", "Title"))
	exists, err := db.SessionExists(ctx, "s1")
	assert.NoError(t, err)
	assert.True(t, exists)
	assert.NoError(t, db.UpdateSessionTitle(ctx, "s2", "New"))

	sessions, err := db.GetSessions(ctx)
	assert.NoError(t, err)
	assert.GreaterOrEqual(t, len(sessions), 2)

	// messages
	m1 := &Message{ID: "m1", SessionID: "s1", Content: "hi", Sender: "u", Timestamp: time.Now()}
	m2 := &Message{ID: "m2", SessionID: "s1", Content: "there", Sender: "u", Timestamp: time.Now().Add(time.Millisecond)}
	assert.NoError(t, db.SaveMessage(ctx, m1))
	assert.NoError(t, db.SaveMessage(ctx, m2))

	got, err := db.GetMessages(ctx, "s1")
	assert.NoError(t, err)
	assert.Len(t, got, 2)

	got2, err := db.GetMessagesWithPagination(ctx, "s1", 1, 1)
	assert.NoError(t, err)
	assert.Len(t, got2, 1)

	assert.NoError(t, db.DeleteSession(ctx, "s2"))
}

func TestSQLite_UsersTenantsAndSystemPrompt(t *testing.T) {
	db := newTestSQLite(t)
	ctx := context.Background()

	// users
	u1 := &User{Username: "u1", Password: "p", Role: RoleAdmin}
	u2 := &User{Username: "u2", Password: "p", Role: RoleNormal}
	assert.NoError(t, db.CreateUser(ctx, u1))
	assert.NoError(t, db.CreateUser(ctx, u2))
	gotU, err := db.GetUserByUsername(ctx, "u1")
	assert.NoError(t, err)
	assert.Equal(t, u1.Username, gotU.Username)
	gotU.IsActive = false
	assert.NoError(t, db.UpdateUser(ctx, gotU))
	users, err := db.ListUsers(ctx)
	assert.NoError(t, err)
	assert.GreaterOrEqual(t, len(users), 2)

	// tenants
	t1 := &Tenant{Name: "t1", Prefix: "/t1", Description: "d1", IsActive: true, CreatedAt: time.Now(), UpdatedAt: time.Now()}
	t2 := &Tenant{Name: "t2", Prefix: "/t2", Description: "d2", IsActive: true, CreatedAt: time.Now(), UpdatedAt: time.Now()}
	assert.NoError(t, db.CreateTenant(ctx, t1))
	assert.NoError(t, db.CreateTenant(ctx, t2))
	gotT, err := db.GetTenantByName(ctx, "t1")
	assert.NoError(t, err)
	assert.Equal(t, t1.Name, gotT.Name)
	// cover UpdateTenant for sqlite implementation
	gotT.Description = "updated"
	assert.NoError(t, db.UpdateTenant(ctx, gotT))
	gotT2, err := db.GetTenantByID(ctx, t2.ID)
	assert.NoError(t, err)
	assert.Equal(t, t2.ID, gotT2.ID)
	allT, err := db.ListTenants(ctx)
	assert.NoError(t, err)
	assert.GreaterOrEqual(t, len(allT), 2)

	// relations
	assert.NoError(t, db.AddUserToTenant(ctx, u1.ID, t1.ID))
	assert.NoError(t, db.AddUserToTenant(ctx, u1.ID, t2.ID))
	tus, err := db.GetUserTenants(ctx, u1.ID)
	assert.NoError(t, err)
	assert.Len(t, tus, 2)
	us, err := db.GetTenantUsers(ctx, t1.ID)
	assert.NoError(t, err)
	assert.GreaterOrEqual(t, len(us), 1)
	assert.NoError(t, db.RemoveUserFromTenant(ctx, u1.ID, t2.ID))
	assert.NoError(t, db.DeleteUserTenants(ctx, u1.ID))

	// system prompt
	prompt, err := db.GetSystemPrompt(ctx, u1.ID)
	assert.NoError(t, err)
	assert.Equal(t, "", prompt)
	assert.NoError(t, db.SaveSystemPrompt(ctx, u1.ID, "hello"))
	prompt2, err := db.GetSystemPrompt(ctx, u1.ID)
	assert.NoError(t, err)
	assert.Equal(t, "hello", prompt2)
	assert.NoError(t, db.SaveSystemPrompt(ctx, u1.ID, "world"))
	prompt3, _ := db.GetSystemPrompt(ctx, u1.ID)
	assert.Equal(t, "world", prompt3)

	// cleanup
	assert.NoError(t, db.DeleteTenant(ctx, t1.ID))
	assert.NoError(t, db.DeleteUser(ctx, u2.ID))
}

func TestSQLite_Transaction(t *testing.T) {
	db := newTestSQLite(t)
	base := context.Background()

	// case 1: no tx on context, should start a new transaction
	err := db.Transaction(base, func(ctx context.Context) error { return nil })
	assert.NoError(t, err)

	// case 2: tx already on context, should reuse it (early branch)
	sqlTx := db.db.Begin()
	defer sqlTx.Rollback()
	withTx := ContextWithTransaction(base, sqlTx)
	err = db.Transaction(withTx, func(ctx context.Context) error { return nil })
	assert.NoError(t, err)
}
