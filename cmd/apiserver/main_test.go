package main

import (
	"bytes"
	"io"
	"os"
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
