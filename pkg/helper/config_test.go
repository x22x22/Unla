package helper

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetCfgPath(t *testing.T) {
	// panic on empty
	assert.Panics(t, func() { GetCfgPath("") })

	// absolute path returns as-is
	abs := "/tmp/test.yaml"
	assert.Equal(t, abs, GetCfgPath(abs))

	// use temp dir
	old, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(old) })

	tmp := t.TempDir()
	_ = os.Chdir(tmp)

	// file in current directory
	f1 := "a.yaml"
	assert.NoError(t, os.WriteFile(f1, []byte("x"), 0o644))
	got := GetCfgPath(f1)
	exp, _ := filepath.EvalSymlinks(filepath.Join(tmp, f1))
	realGot, _ := filepath.EvalSymlinks(got)
	assert.Equal(t, exp, realGot)

	// prefer ./configs second
	_ = os.Remove(filepath.Join(tmp, f1))
	_ = os.MkdirAll("configs", 0o755)
	assert.NoError(t, os.WriteFile(filepath.Join("configs", f1), []byte("x"), 0o644))
	got = GetCfgPath(f1)
	exp, _ = filepath.EvalSymlinks(filepath.Join(tmp, "configs", f1))
	realGot, _ = filepath.EvalSymlinks(got)
	assert.Equal(t, exp, realGot)

	// fallback when not found
	_ = os.Remove(filepath.Join(tmp, "configs", f1))
	got = GetCfgPath(f1)
	assert.Equal(t, filepath.Join("/etc/unla", f1), got)
}
