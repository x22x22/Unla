package helper

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetPIDPath(t *testing.T) {
	// absolute path
	abs := "/tmp/xx.pid"
	assert.Equal(t, abs, GetPIDPath(abs))

	// empty filename => fallback constant path
	assert.Equal(t, filepath.Join("/var/run/mcp-gateway.pid"), GetPIDPath(""))

	// relative filename returns absolute path under cwd if parent exists
	old, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(old) })
	tmp := t.TempDir()
	_ = os.Chdir(tmp)
	got := GetPIDPath("proc.pid")
	exp, _ := filepath.EvalSymlinks(filepath.Join(tmp, "proc.pid"))
	realGot, _ := filepath.EvalSymlinks(got)
	assert.Equal(t, exp, realGot)
}
