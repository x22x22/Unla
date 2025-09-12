package config

import (
	"testing"

	"github.com/amoylab/unla/pkg/mcp"
	"github.com/stretchr/testify/assert"
)

func TestToolConfig_ToToolSchema(t *testing.T) {
	tcfg := &ToolConfig{
		Name:        "t1",
		Description: "desc",
		Args: []ArgConfig{
			{Name: "a1", Type: "string", Description: "d1", Required: true},
			{Name: "arr", Type: "array", Items: ItemsConfig{Enum: []string{"x", "x", "y"}}},
		},
		InputSchema: map[string]any{"extra": map[string]any{"type": "string"}},
		Annotations: map[string]any{"title": "T"},
	}

	schema := tcfg.ToToolSchema()
	assert.Equal(t, "t1", schema.Name)
	assert.Equal(t, "object", schema.InputSchema.Type)
	// Required contains a1
	if assert.Contains(t, schema.InputSchema.Required, "a1") {
		// ok
	}
	// Properties merged
	props := schema.InputSchema.Properties
	assert.Contains(t, props, "a1")
	assert.Contains(t, props, "arr")
	assert.Contains(t, props, "extra")
	// Array items enum present
	arr := props["arr"].(map[string]any)
	items := arr["items"].(map[string]any)
	assert.Contains(t, items["enum"], "x")
	assert.Contains(t, items["enum"], "y")
	// Annotations present with title
	if assert.NotNil(t, schema.Annotations) {
		assert.Equal(t, "T", schema.Annotations.Title)
	}
}

func TestPromptConfig_ToPromptSchema(t *testing.T) {
	pc := &PromptConfig{
		Name:           "p1",
		Description:    "d",
		Arguments:      []PromptArgument{{Name: "u", Description: "user", Required: true}},
		PromptResponse: []PromptResponse{{Role: "system", Content: PromptResponseContent{Type: "text", Text: "hello"}}},
	}
	ps := pc.ToPromptSchema()
	assert.Equal(t, "p1", ps.Name)
	assert.Equal(t, "d", ps.Description)
	if assert.Len(t, ps.Arguments, 1) {
		assert.Equal(t, mcp.PromptArgumentSchema{Name: "u", Description: "user", Required: true}, ps.Arguments[0])
	}
	if assert.Len(t, ps.PromptResponse, 1) {
		assert.Equal(t, "system", ps.PromptResponse[0].Role)
		assert.Equal(t, "text", ps.PromptResponse[0].Content.Type)
		assert.Equal(t, "hello", ps.PromptResponse[0].Content.Text)
	}
}
