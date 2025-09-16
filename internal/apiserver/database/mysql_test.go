package database

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// newTestMySQL creates a MySQL instance backed by an in-memory SQLite database.
// This allows us to exercise MySQL methods without requiring a real MySQL server,
// because the GORM operations used are dialect-agnostic for these paths.
func newTestMySQL(t *testing.T) *MySQL {
	t.Helper()
	gdb, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite memory: %v", err)
	}
	// Migrate required models.
	if err := gdb.AutoMigrate(&Message{}, &Session{}, &User{}, &Tenant{}, &UserTenant{}, &SystemPrompt{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	return &MySQL{db: gdb}
}

func TestMySQL_SessionsAndMessages(t *testing.T) {
	db := newTestMySQL(t)
	ctx := context.Background()

	// sessions
	assert.NoError(t, db.CreateSession(ctx, "ms1"))
	assert.NoError(t, db.CreateSessionWithTitle(ctx, "ms2", "Title"))
	exists, err := db.SessionExists(ctx, "ms1")
	assert.NoError(t, err)
	assert.True(t, exists)
	assert.NoError(t, db.UpdateSessionTitle(ctx, "ms2", "New"))

	sessions, err := db.GetSessions(ctx)
	assert.NoError(t, err)
	assert.GreaterOrEqual(t, len(sessions), 2)

	// messages
	m1 := &Message{ID: "mm1", SessionID: "ms1", Content: "hi", Sender: "u", Timestamp: time.Now()}
	m2 := &Message{ID: "mm2", SessionID: "ms1", Content: "there", Sender: "u", Timestamp: time.Now().Add(time.Millisecond)}
	assert.NoError(t, db.SaveMessage(ctx, m1))
	assert.NoError(t, db.SaveMessage(ctx, m2))

	got, err := db.GetMessages(ctx, "ms1")
	assert.NoError(t, err)
	assert.Len(t, got, 2)

	got2, err := db.GetMessagesWithPagination(ctx, "ms1", 1, 1)
	assert.NoError(t, err)
	assert.Len(t, got2, 1)

	assert.NoError(t, db.DeleteSession(ctx, "ms2"))
}

func TestMySQL_UsersAndTenants(t *testing.T) {
	db := newTestMySQL(t)
	ctx := context.Background()

	// users
	u1 := &User{Username: "mu1", Password: "p", Role: RoleAdmin}
	u2 := &User{Username: "mu2", Password: "p", Role: RoleNormal}
	assert.NoError(t, db.CreateUser(ctx, u1))
	assert.NoError(t, db.CreateUser(ctx, u2))
	gotU, err := db.GetUserByUsername(ctx, "mu1")
	assert.NoError(t, err)
	assert.Equal(t, u1.Username, gotU.Username)
	gotU.IsActive = false
	assert.NoError(t, db.UpdateUser(ctx, gotU))
	users, err := db.ListUsers(ctx)
	assert.NoError(t, err)
	assert.GreaterOrEqual(t, len(users), 2)

	// tenants
	t1 := &Tenant{Name: "mt1", Prefix: "/mt1", Description: "d1", IsActive: true, CreatedAt: time.Now(), UpdatedAt: time.Now()}
	t2 := &Tenant{Name: "mt2", Prefix: "/mt2", Description: "d2", IsActive: true, CreatedAt: time.Now(), UpdatedAt: time.Now()}
	assert.NoError(t, db.CreateTenant(ctx, t1))
	assert.NoError(t, db.CreateTenant(ctx, t2))
	gotT, err := db.GetTenantByName(ctx, "mt1")
	assert.NoError(t, err)
	assert.Equal(t, t1.Name, gotT.Name)
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

	// update + delete
	gotT.Description = "updated"
	assert.NoError(t, db.UpdateTenant(ctx, gotT))
	assert.NoError(t, db.DeleteTenant(ctx, t1.ID))
	assert.NoError(t, db.DeleteUser(ctx, u2.ID))
}

func TestMySQL_TransactionAndClose(t *testing.T) {
	db := newTestMySQL(t)
	base := context.Background()
	// cover transaction paths with no-op
	assert.NoError(t, db.Transaction(base, func(ctx context.Context) error { return nil }))
	tx := db.db.Begin()
	defer tx.Rollback()
	withTx := ContextWithTransaction(base, tx)
	assert.NoError(t, db.Transaction(withTx, func(ctx context.Context) error { return nil }))
	// cover Close
	assert.NoError(t, db.Close())
}
