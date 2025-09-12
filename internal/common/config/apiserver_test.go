package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDatabaseConfig_GetDSN_Postgres(t *testing.T) {
	c := &DatabaseConfig{Type: "postgres", Host: "h", Port: 5432, User: "u", Password: "p", DBName: "d", SSLMode: "disable"}
	got := c.GetDSN()
	assert.Equal(t, "postgres://u:p@h:5432/d?sslmode=disable", got)
}

func TestDatabaseConfig_GetDSN_MySQL(t *testing.T) {
	c := &DatabaseConfig{Type: "mysql", Host: "h", Port: 3306, User: "u", Password: "p", DBName: "d"}
	got := c.GetDSN()
	assert.Equal(t, "u:p@tcp(h:3306)/d?charset=utf8mb4&parseTime=True&loc=Local", got)
}

func TestDatabaseConfig_GetDSN_SQLite(t *testing.T) {
	tmp := t.TempDir()
	dbPath := filepath.Join(tmp, "data", "app.sqlite")
	c := &DatabaseConfig{Type: "sqlite", DBName: dbPath}
	got := c.GetDSN()
	assert.Equal(t, dbPath, got)
	// Directory for sqlite DB should be created
	_, err := os.Stat(filepath.Dir(dbPath))
	assert.NoError(t, err)
}

func TestDatabaseConfig_GetDSN_Unknown(t *testing.T) {
	c := &DatabaseConfig{Type: "unknown"}
	assert.Equal(t, "", c.GetDSN())
}
