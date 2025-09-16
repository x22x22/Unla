package i18n

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestI18nError_Basics(t *testing.T) {
	e := NewWithMessage("MsgID", "Hello, {{.Name}}!")
	e = e.WithParam("Name", "World")
	if got := e.Error(); got != "Hello, World!" {
		t.Fatalf("unexpected error message: %s", got)
	}

	// WithData merge
	e.WithData(map[string]any{"Name": "Alice"})
	if got := e.Error(); got != "Hello, Alice!" {
		t.Fatalf("unexpected merged message: %s", got)
	}

	// TranslateByRequest falls back to default when translator not initialized
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	if got := e.TranslateByRequest(r); got == "" {
		t.Fatalf("expected fallback translation, got empty")
	}
}

func TestErrorWithCode(t *testing.T) {
	ew := NewErrorWithCode("ErrID", ErrorBadRequest)
	if ew.GetCode() != ErrorBadRequest {
		t.Fatalf("unexpected code: %v", ew.GetCode())
	}
	// Change status code
	ew2 := ew.WithHttpCode(ErrorForbidden)
	if ew2.GetCode() != ErrorForbidden {
		t.Fatalf("unexpected changed code: %v", ew2.GetCode())
	}
}

func TestIsAndAsI18nError(t *testing.T) {
	e := New("X")
	if !IsI18nError(e) {
		t.Fatalf("expected true for I18nError")
	}
	if AsI18nError(e) == nil {
		t.Fatalf("expected non-nil from AsI18nError")
	}
	var err error
	if IsI18nError(err) {
		t.Fatalf("nil should not be I18nError")
	}
}
