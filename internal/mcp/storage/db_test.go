package storage

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/amoylab/unla/internal/common/config"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func newSQLiteStore(t *testing.T) *DBStore {
	t.Helper()
	tmp := t.TempDir()
	dbPath := filepath.Join(tmp, "store.db")
	cfg := &config.StorageConfig{
		RevisionHistoryLimit: 3,
		Database:             config.DatabaseConfig{Type: "sqlite", DBName: dbPath},
	}
	s, err := NewDBStore(zap.NewNop(), cfg)
	assert.NoError(t, err)
	return s
}

func TestDBStore_CreateUpdateVersioningAndDelete(t *testing.T) {
	s := newSQLiteStore(t)
	ctx := context.Background()
	cfg := sampleConfig()

	// Create v1
	assert.NoError(t, s.Create(ctx, cfg))

	got, err := s.Get(ctx, cfg.Tenant, cfg.Name)
	assert.NoError(t, err)
	assert.Equal(t, cfg.Name, got.Name)

	// List versions: only v1 active
	vers, err := s.ListVersions(ctx, cfg.Tenant, cfg.Name)
	assert.NoError(t, err)
	if assert.Len(t, vers, 1) {
		assert.Equal(t, 1, vers[0].Version)
		assert.True(t, vers[0].IsActive)
	}

	// Update with same content -> no new version
	assert.NoError(t, s.Update(ctx, cfg))
	vers, err = s.ListVersions(ctx, cfg.Tenant, cfg.Name)
	assert.NoError(t, err)
	assert.Len(t, vers, 1)

	// Update with changed content -> new version v2
	cfg.Tools = append(cfg.Tools, config.ToolConfig{Name: "tool2", Method: "GET", Endpoint: "http://e"})
	assert.NoError(t, s.Update(ctx, cfg))
	vers, err = s.ListVersions(ctx, cfg.Tenant, cfg.Name)
	assert.NoError(t, err)
	if assert.Len(t, vers, 2) {
		assert.Equal(t, 2, vers[0].Version) // ordered desc
		assert.True(t, vers[0].IsActive)
	}

	// GetVersion(1)
	v1, err := s.GetVersion(ctx, cfg.Tenant, cfg.Name, 1)
	assert.NoError(t, err)
	assert.Equal(t, 1, v1.Version)

	// Revert active to version 1 -> creates new version 3
	assert.NoError(t, s.SetActiveVersion(ctx, cfg.Tenant, cfg.Name, 1))
	vers, err = s.ListVersions(ctx, cfg.Tenant, cfg.Name)
	assert.NoError(t, err)
	// versions include 3,2,1 with 3 active
	if assert.GreaterOrEqual(t, len(vers), 3) {
		assert.Equal(t, 3, vers[0].Version)
		assert.True(t, vers[0].IsActive)
	}

	// Delete should soft delete and create delete version
	assert.NoError(t, s.Delete(ctx, cfg.Tenant, cfg.Name))
	// Get without includeDeleted now fails
	_, err = s.Get(ctx, cfg.Tenant, cfg.Name)
	assert.Error(t, err)

	// ListUpdated should include deletion with DeletedAt set
	since := time.Now().Add(-time.Hour)
	ups, err := s.ListUpdated(ctx, since)
	assert.NoError(t, err)
	assert.NotEmpty(t, ups)

	// Active version cannot be deleted; find latest active version number
	vers, _ = s.ListVersions(ctx, cfg.Tenant, cfg.Name)
	activeVer := -1
	for _, v := range vers {
		if v.IsActive {
			activeVer = v.Version
			break
		}
	}
	if activeVer > 0 {
		assert.Error(t, s.DeleteVersion(ctx, cfg.Tenant, cfg.Name, activeVer))
	}
}
