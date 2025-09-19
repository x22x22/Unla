package template

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

type user struct {
	Name string
	Age  int
}

func TestFromJSON(t *testing.T) {
	s := `[{"a":1},{"b":2}]`
	out := fromJSON(s)
	assert.Len(t, out, 2)
	assert.Equal(t, float64(1), out[0]["a"]) // numbers become float64 in generic JSON

	// invalid json -> empty slice
	out2 := fromJSON("not-json")
	assert.Nil(t, out2) // json.Unmarshal to []map returns nil on error (stays nil)
}

func TestToJSON(t *testing.T) {
	m := map[string]any{"k": "v", "n": 1}
	s, err := toJSON(m)
	assert.NoError(t, err)
	var back map[string]any
	assert.NoError(t, json.Unmarshal([]byte(s), &back))
	assert.Equal(t, "v", back["k"])
}

func TestSafeGetAndSafeGetOr(t *testing.T) {
	// Nested map, slice, struct, pointers
	nested := map[string]any{
		"User":  user{Name: "Ada", Age: 30},
		"Items": []map[string]any{{"Title": "T1"}, {"Title": "T2"}},
		"Ptr":   &user{Name: "Bob", Age: 40},
		"Iface": any(&user{Name: "C", Age: 50}),
	}

	// Map keys containing dashes should be retrievable via safeGet
	nested["a-b"] = "dash-top"
	nested["m"] = map[string]any{"a-b": "dash-nested"}

	assert.Equal(t, "Ada", safeGet("User.Name", nested))
	assert.Equal(t, "T2", safeGet("Items.1.Title", nested))
	assert.Equal(t, "Bob", safeGet("Ptr.Name", nested))
	assert.Equal(t, "C", safeGet("Iface.Name", nested))
	assert.Equal(t, "dash-top", safeGet("a-b", nested))
	assert.Equal(t, "dash-nested", safeGet("m.a-b", nested))

	// Missing paths -> nil
	assert.Nil(t, safeGet("User.Unknown", nested))
	assert.Nil(t, safeGet("Items.99.Title", nested))
	assert.Nil(t, safeGet("a-b.missing", nested))

	// Default fallback
	assert.Equal(t, "anon", safeGetOr("User.Unknown", nested, "anon"))
	assert.Equal(t, "def", safeGetOr("a-b.missing", nested, "def"))
}

func TestDefaultTemplateSyntaxFailsWithDashKeyButSafeGetWorks(t *testing.T) {
	ctx := NewContext()
	ctx.Args["a-b"] = "works"

	// Default dot syntax with dash in key is parsed as subtraction and fails
	out, err := RenderTemplate("{{ .Args.a-b }}", ctx)
	assert.Error(t, err)
	assert.Empty(t, out)

	// safeGet can access keys containing dashes
	out2, err2 := RenderTemplate("{{ safeGet \"Args.a-b\" . }}", ctx)
	assert.NoError(t, err2)
	assert.Equal(t, "works", out2)
}
