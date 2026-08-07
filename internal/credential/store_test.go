package credential

import (
	"errors"
	"testing"

	keyring "github.com/zalando/go-keyring"
)

func TestNormalizeKeyringErrorDistinguishesUnavailableBackend(t *testing.T) {
	t.Parallel()
	err := normalizeKeyringError(errors.New("The name org.freedesktop.secrets was not provided by any .service files"))
	if !errors.Is(err, ErrUnavailable) {
		t.Fatalf("expected unavailable error, got %v", err)
	}
	if errors.Is(err, ErrNotFound) {
		t.Fatalf("unavailable backend must not be reported as a missing credential: %v", err)
	}
}

func TestNormalizeKeyringErrorPreservesMissingCredential(t *testing.T) {
	t.Parallel()
	err := normalizeKeyringError(keyring.ErrNotFound)
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected not-found error, got %v", err)
	}
}
