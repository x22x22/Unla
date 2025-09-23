package template

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewContext(t *testing.T) {
	ctx := NewContext()

	t.Run("initializes basic maps", func(t *testing.T) {
		assert.NotNil(t, ctx.Args)
		assert.NotNil(t, ctx.Config)
		assert.Equal(t, 0, len(ctx.Args))
		assert.Equal(t, 0, len(ctx.Config))
	})

	t.Run("initializes request wrapper", func(t *testing.T) {
		assert.NotNil(t, ctx.Request.Headers)
		assert.NotNil(t, ctx.Request.Query)
		assert.NotNil(t, ctx.Request.Cookies)
		assert.NotNil(t, ctx.Request.Path)
		assert.NotNil(t, ctx.Request.Body)
		assert.Equal(t, 0, len(ctx.Request.Headers))
		assert.Equal(t, 0, len(ctx.Request.Query))
		assert.Equal(t, 0, len(ctx.Request.Cookies))
		assert.Equal(t, 0, len(ctx.Request.Path))
		assert.Equal(t, 0, len(ctx.Request.Body))
	})

	t.Run("initializes response wrapper", func(t *testing.T) {
		assert.Nil(t, ctx.Response.Data)
		assert.Nil(t, ctx.Response.Body)
	})

	t.Run("env function works", func(t *testing.T) {
		assert.NotNil(t, ctx.Env)
		// Test with a known environment variable
		os.Setenv("TEST_TEMPLATE_VAR", "test_value")
		defer os.Unsetenv("TEST_TEMPLATE_VAR")

		assert.Equal(t, "test_value", ctx.Env("TEST_TEMPLATE_VAR"))
		assert.Equal(t, "", ctx.Env("NON_EXISTENT_VAR"))
	})
}

func TestContextFields(t *testing.T) {
	ctx := NewContext()

	t.Run("can set and get args", func(t *testing.T) {
		ctx.Args["key"] = "value"
		assert.Equal(t, "value", ctx.Args["key"])
	})

	t.Run("can set and get config", func(t *testing.T) {
		ctx.Config["setting"] = "config_value"
		assert.Equal(t, "config_value", ctx.Config["setting"])
	})

	t.Run("can modify request fields", func(t *testing.T) {
		ctx.Request.Headers["Content-Type"] = "application/json"
		ctx.Request.Query["param"] = "value"
		ctx.Request.Cookies["session"] = "abc123"
		ctx.Request.Path["id"] = "123"
		ctx.Request.Body["field"] = "data"

		assert.Equal(t, "application/json", ctx.Request.Headers["Content-Type"])
		assert.Equal(t, "value", ctx.Request.Query["param"])
		assert.Equal(t, "abc123", ctx.Request.Cookies["session"])
		assert.Equal(t, "123", ctx.Request.Path["id"])
		assert.Equal(t, "data", ctx.Request.Body["field"])
	})

	t.Run("can modify response fields", func(t *testing.T) {
		ctx.Response.Data = map[string]string{"result": "success"}
		ctx.Response.Body = "response body"

		assert.Equal(t, map[string]string{"result": "success"}, ctx.Response.Data)
		assert.Equal(t, "response body", ctx.Response.Body)
	})
}
