package credential

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPreparedTokenFileCommitsPrivateToken(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "sateia-token")
	prepared, err := PrepareTokenFile(path)
	if err != nil {
		t.Fatal(err)
	}
	defer prepared.Abort()
	if err := prepared.Commit("headless-secret"); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("unexpected permissions %o", info.Mode().Perm())
	}
	token, err := ReadTokenFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if token != "headless-secret" {
		t.Fatalf("unexpected token %q", token)
	}
}

func TestPreparedTokenFileAbortRemovesPlaceholder(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "sateia-token")
	prepared, err := PrepareTokenFile(path)
	if err != nil {
		t.Fatal(err)
	}
	prepared.Abort()
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("expected placeholder removal, got %v", err)
	}
}

func TestPrepareTokenFileRefusesExistingPath(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "sateia-token")
	if err := os.WriteFile(path, []byte("existing-secret"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := PrepareTokenFile(path); err == nil {
		t.Fatal("expected existing token file to be rejected")
	}
}

func TestPrepareTokenFileRejectsLineBreaks(t *testing.T) {
	t.Parallel()
	if _, err := PrepareTokenFile(filepath.Join(t.TempDir(), "token\nforged")); err == nil {
		t.Fatal("expected path with line break to be rejected")
	}
}
