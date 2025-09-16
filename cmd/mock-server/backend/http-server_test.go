package backend

import "testing"

func TestNewHTTPServer(t *testing.T) {
	srv := NewHTTPServer()
	if srv == nil {
		t.Fatalf("expected server, got nil")
	}
}
