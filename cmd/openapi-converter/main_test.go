package main

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRootCmd_JSONInput_ToYAMLFile(t *testing.T) {
	outputFile = ""
	outputFormat = "yaml"
	// Minimal OpenAPI 3.0 spec
	input := `{"openapi":"3.0.0","info":{"title":"T","version":"1"},"paths":{"/ping":{"get":{"responses":{"200":{"description":"ok"}}}}}}`
	tmpDir := t.TempDir()
	in := filepath.Join(tmpDir, "in.json")
	out := filepath.Join(tmpDir, "out.yaml")
	if err := os.WriteFile(in, []byte(input), 0644); err != nil {
		t.Fatal(err)
	}

	rootCmd.SetArgs([]string{"-f", "yaml", "-o", out, in})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute error: %v", err)
	}

	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	s := string(data)
	if !strings.Contains(s, "routers:") || !strings.Contains(s, "servers:") {
		t.Fatalf("unexpected output: %s", s)
	}
}

func TestRootCmd_YAMLInput_ToJSONStdout(t *testing.T) {
	outputFile = ""
	outputFormat = "json"
	input := "openapi: 3.0.0\ninfo:\n  title: T\n  version: '1'\npaths:\n  /ping:\n    get:\n      responses:\n        '200':\n          description: ok\n"
	tmpDir := t.TempDir()
	in := filepath.Join(tmpDir, "in.yaml")
	if err := os.WriteFile(in, []byte(input), 0644); err != nil {
		t.Fatal(err)
	}

	// capture stdout
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	rootCmd.SetArgs([]string{"-f", "json", in})
	err := rootCmd.Execute()

	// restore stdout
	_ = w.Close()
	os.Stdout = old
	out, _ := io.ReadAll(r)

	if err != nil {
		t.Fatalf("execute error: %v", err)
	}
	s := string(out)
	if !strings.Contains(s, "\"routers\"") || !strings.Contains(s, "\"servers\"") {
		t.Fatalf("unexpected output: %s", s)
	}
}

func TestRootCmd_UnsupportedFormat(t *testing.T) {
	input := `{"openapi":"3.0.0","info":{"title":"T","version":"1"},"paths":{}}`
	tmp := filepath.Join(t.TempDir(), "in.json")
	if err := os.WriteFile(tmp, []byte(input), 0644); err != nil {
		t.Fatal(err)
	}
	rootCmd.SetArgs([]string{"-f", "xml", tmp})
	if err := rootCmd.Execute(); err == nil {
		t.Fatalf("expected error for unsupported format")
	}
}
