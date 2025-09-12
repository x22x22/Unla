package template

import (
	"net/http"
	"net/url"
	"os"
	"testing"

	"github.com/amoylab/unla/internal/mcp/session"
	"github.com/stretchr/testify/assert"
)

func TestGenerateTemplateNameDeterministic(t *testing.T) {
	n1 := generateTemplateName("hello {{ . }}")
	n2 := generateTemplateName("hello {{ . }}")
	n3 := generateTemplateName("other")
	assert.Equal(t, n1, n2)
	assert.NotEqual(t, n1, n3)
}

func TestRenderTemplateWithFuncs(t *testing.T) {
	_ = os.Setenv("X_ENV_TEST", "works")
	ctx := NewContext()
	ctx.Args["A"] = 2
	ctx.Args["B"] = 3
	tmpl := `sum={{ add .Args.A .Args.B }} env={{ env "X_ENV_TEST" }}`
	out, err := RenderTemplate(tmpl, ctx)
	assert.NoError(t, err)
	assert.Contains(t, out, "sum=5")
	assert.Contains(t, out, "env=works")
}

func TestPreprocessArgsAndNormalizeJSONStringValues(t *testing.T) {
	args := map[string]any{
		"arr": []any{"x", 1},
		"f":   float64(3),
		"g":   float64(3.14),
	}
	processed := preprocessArgs(args)
	// array becomes JSON string
	assert.Equal(t, `[`+"\"x\""+`,1]`, processed["arr"])
	// integer-like float becomes int64
	assert.Equal(t, int64(3), processed["f"])
	// non-integer float stays float64
	assert.Equal(t, 3.14, processed["g"])

	// NormalizeJSONStringValues converts JSON-looking strings to objects/arrays
	m := map[string]any{
		"obj": `{"k":"v"}`,
		"arr": `[1,2]`,
		"str": "plain",
	}
	NormalizeJSONStringValues(m)
	assert.IsType(t, map[string]any{}, m["obj"])
	assert.IsType(t, []any{}, m["arr"])
	assert.Equal(t, "plain", m["str"])
}

func TestPrepareTemplateContextMerges(t *testing.T) {
	// Build request
	u, _ := url.Parse("http://example.com/path?q=1")
	req := &http.Request{Header: http.Header{"H": {"v"}}, URL: u}
	req.AddCookie(&http.Cookie{Name: "c", Value: "cv"})

	meta := &session.RequestInfo{
		Headers: map[string]string{"HX": "vx"},
		Query:   map[string]string{"qq": "vv"},
		Cookies: map[string]string{"cx": "vx"},
	}

	cfg := map[string]string{"x": "{{ .Request.Headers.H }}-{{ .Request.Cookies.c }}"}
	ctx, err := PrepareTemplateContext(meta, map[string]any{"k": "v"}, req, cfg)
	assert.NoError(t, err)
	assert.Equal(t, "v", ctx.Args["k"])
	// Merged headers (request overrides meta on conflict)
	assert.Equal(t, "v", ctx.Request.Headers["H"])
	assert.Equal(t, "vx", ctx.Request.Headers["HX"])
	// Merged query
	assert.Equal(t, "1", ctx.Request.Query["q"])
	assert.Equal(t, "vv", ctx.Request.Query["qq"])
	// Merged cookies
	assert.Equal(t, "cv", ctx.Request.Cookies["c"])
	assert.Equal(t, "vx", ctx.Request.Cookies["cx"])
	// Rendered server config
	assert.Equal(t, "v-cv", ctx.Config["x"])
}

func TestAssembleTemplateContext(t *testing.T) {
	req := &RequestWrapper{Headers: map[string]string{"H": "v"}, Cookies: map[string]string{"c": "cv"}}
	args := map[string]any{"a": 1}
	cfg := map[string]string{"x": "{{ .Request.Headers.H }}-{{ .Request.Cookies.c }}-{{ .Args.a }}"}

	ctx, err := AssembleTemplateContext(req, args, cfg)
	assert.NoError(t, err)
	assert.Equal(t, 1, ctx.Args["a"]) // AssembleTemplateContext preserves int as-is
	assert.Equal(t, "v-cv-1", ctx.Config["x"])
}
