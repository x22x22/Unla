package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func TestHandleDefaultLLMProviders_EmptyAndFromEnv(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewChat(nil, zap.NewNop())

	r := gin.New()
	r.GET("/llm", h.HandleDefaultLLMProviders)

	// No env -> empty configs
	w1 := httptest.NewRecorder()
	req1, _ := http.NewRequest("GET", "/llm", nil)
	r.ServeHTTP(w1, req1)
	if w1.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w1.Code)
	}
	var resp1 struct {
		Configs []any `json:"configs"`
	}
	_ = json.Unmarshal(w1.Body.Bytes(), &resp1)
	if len(resp1.Configs) != 0 {
		t.Fatalf("expected empty configs, got %v", resp1.Configs)
	}

	// With env -> single config
	t.Setenv("OPENAI_API_KEY", "k")
	t.Setenv("OPENAI_BASE_URL", "http://x")
	t.Setenv("OPENAI_MODEL", "gpt-x")
	w2 := httptest.NewRecorder()
	req2, _ := http.NewRequest("GET", "/llm", nil)
	r.ServeHTTP(w2, req2)
	if w2.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w2.Code)
	}
	var resp2 map[string][]map[string]any
	if err := json.Unmarshal(w2.Body.Bytes(), &resp2); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(resp2["configs"]) != 1 {
		t.Fatalf("expected 1 config, got %d", len(resp2["configs"]))
	}
	cfg := resp2["configs"][0]
	if cfg["enabled"] != true || cfg["name"] != "Default" {
		t.Fatalf("unexpected cfg: %v", cfg)
	}
}

func Test_getEnv(t *testing.T) {
	if got := getEnv("NO_SUCH_ENV", "fallback"); got != "fallback" {
		t.Fatalf("expected fallback, got %s", got)
	}
	t.Setenv("FOO", "bar")
	if got := getEnv("FOO", "fallback"); got != "bar" {
		t.Fatalf("expected bar, got %s", got)
	}
}
