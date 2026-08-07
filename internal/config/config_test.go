package config

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestSaveDoesNotPersistTokenAndUsesPrivatePermissions(t *testing.T) {
	directory := t.TempDir()
	t.Setenv("SATEIA_CONFIG_DIR", directory)
	if err := Save(Config{BaseURL: "https://example.com", TokenID: "token-id"}); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(directory, "config.json")
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0o600 {
		t.Fatalf("unexpected permissions %o", info.Mode().Perm())
	}
	loaded, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if loaded.TokenID != "token-id" || loaded.BaseURL != "https://example.com" {
		t.Fatalf("unexpected config %#v", loaded)
	}
}
