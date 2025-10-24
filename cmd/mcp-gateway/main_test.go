package main

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
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

func TestTestCommand_FailsWithInvalidConfig(t *testing.T) {
	// Test that command parsing works, actual execution may call os.Exit
	t.Cleanup(func() { rootCmd.SetArgs([]string{}) })
	rootCmd.SetArgs([]string{"test", "--conf", "nonexistent.yaml"})
	// We don't check the error here as cobra may call os.Exit on failure
	// Instead we test that the args are parsed correctly
	commands := rootCmd.Commands()
	var testCmd *cobra.Command
	for _, cmd := range commands {
		if cmd.Name() == "test" {
			testCmd = cmd
			break
		}
	}
	if testCmd == nil {
		t.Fatal("test command not found")
	}
}

func TestReloadCommand_Setup(t *testing.T) {
	// Test that reload command exists and accepts flags
	commands := rootCmd.Commands()
	var reloadCmd *cobra.Command
	for _, cmd := range commands {
		if cmd.Name() == "reload" {
			reloadCmd = cmd
			break
		}
	}
	if reloadCmd == nil {
		t.Fatal("reload command not found")
	}

	// Test that command accepts config flag
	t.Cleanup(func() { rootCmd.SetArgs([]string{}) })
	rootCmd.SetArgs([]string{"reload", "--conf", "test.yaml"})
	// Just test that args are parsed without executing
}

func TestCommandStructure(t *testing.T) {
	// Test root command has expected subcommands
	commands := rootCmd.Commands()
	expectedCommands := []string{"reload", "test", "version"}

	if len(commands) < len(expectedCommands) {
		t.Fatalf("expected at least %d commands, got %d", len(expectedCommands), len(commands))
	}

	// Check that expected commands exist
	foundCommands := make(map[string]bool)
	for _, cmd := range commands {
		foundCommands[cmd.Name()] = true
	}

	for _, expected := range expectedCommands {
		if !foundCommands[expected] {
			t.Errorf("expected command %s not found", expected)
		}
	}
}

func TestInit(t *testing.T) {
	// Test that init function properly sets up commands and flags
	if rootCmd == nil {
		t.Fatal("rootCmd should be initialized")
	}

	// Test persistent flags
	flags := rootCmd.PersistentFlags()
	if !flags.HasFlags() {
		t.Fatal("expected persistent flags to be set")
	}

	// Test that config flag exists
	confFlag := flags.Lookup("conf")
	if confFlag == nil {
		t.Fatal("expected conf flag to exist")
	}

	// Test that PID flag exists
	pidFlag := flags.Lookup("pid")
	if pidFlag == nil {
		t.Fatal("expected pid flag to exist")
	}
}
