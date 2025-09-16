package database

import (
	"testing"

	"github.com/amoylab/unla/internal/common/config"
)

func TestNewDatabase_Factory(t *testing.T) {
	// unsupported
	if _, err := NewDatabase(&config.DatabaseConfig{Type: "unknown"}); err == nil {
		t.Fatalf("expected error for unsupported db type")
	}

	// sqlite
	db, err := NewDatabase(&config.DatabaseConfig{Type: "sqlite", DBName: ":memory:"})
	if err != nil {
		t.Fatalf("sqlite: %v", err)
	}
	if db == nil {
		t.Fatalf("expected non-nil sqlite db")
	}
	_ = db.Close()

	// mysql path should attempt to open and fail quickly (invalid dsn)
	if _, err := NewDatabase(&config.DatabaseConfig{Type: "mysql", Host: "127.0.0.1", Port: 3306, User: "u", Password: "p", DBName: "d"}); err == nil {
		t.Fatalf("expected error opening mysql")
	}
}
