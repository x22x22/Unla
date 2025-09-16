package i18n

import (
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/text/language"
)

func TestNewI18nAndLoadTranslations(t *testing.T) {
	dir := t.TempDir()
	// write a minimal TOML translation file
	content := []byte(`[Hello]
other = "Hello"
`)
	if err := os.WriteFile(filepath.Join(dir, "en.toml"), content, 0644); err != nil {
		t.Fatalf("write toml: %v", err)
	}

	i := NewI18n(language.English)
	if err := i.LoadTranslations(dir); err != nil {
		t.Fatalf("load translations: %v", err)
	}
}
