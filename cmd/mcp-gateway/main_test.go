package main

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func captureOutput(f func()) string {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	defer func() { os.Stdout = old }()

	f()
	_ = w.Close()
	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	return buf.String()
}

func TestRootCmd_Version(t *testing.T) {
	t.Cleanup(func() { rootCmd.SetArgs([]string{}) })
	rootCmd.SetArgs([]string{"version"})
	out := captureOutput(func() { _ = rootCmd.Execute() })
	if out == "" {
		t.Fatalf("expected version output, got empty")
	}
}

func TestRootCmd_Help(t *testing.T) {
	t.Cleanup(func() { rootCmd.SetArgs([]string{}) })
	rootCmd.SetArgs([]string{"--help"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("help should not error: %v", err)
	}
}

func TestTestCommand_SucceedsWithTempConfig(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "mcp-gateway.yaml")
	dbPath := filepath.Join(dir, "store.db")
	yaml := []byte("logger:\n  level: info\nstorage:\n  type: db\n  database:\n    type: sqlite\n    dbname: " + strings.ReplaceAll(dbPath, "\\", "\\\\") + "\n")
	if err := os.WriteFile(cfgPath, yaml, 0644); err != nil {
		t.Fatalf("write cfg: %v", err)
	}
	t.Cleanup(func() { _ = os.Remove(cfgPath) })

	t.Cleanup(func() { rootCmd.SetArgs([]string{}) })
	rootCmd.SetArgs([]string{"test", "--conf", cfgPath})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("test command should succeed: %v", err)
	}
}
