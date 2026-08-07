package credential

import (
	"errors"
	"fmt"

	keyring "github.com/zalando/go-keyring"
)

const service = "sateia-cli"

var (
	ErrNotFound    = errors.New("credential not found")
	ErrUnavailable = errors.New("system credential store is unavailable")
)

type Store interface {
	Get(account string) (string, error)
	Set(account, token string) error
	Delete(account string) error
}

type KeyringStore struct{}

func (KeyringStore) Get(account string) (string, error) {
	value, err := keyring.Get(service, account)
	return value, normalizeKeyringError(err)
}

func (KeyringStore) Set(account, token string) error {
	return normalizeKeyringError(keyring.Set(service, account, token))
}

func (KeyringStore) Delete(account string) error {
	return normalizeKeyringError(keyring.Delete(service, account))
}

func normalizeKeyringError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, keyring.ErrNotFound) {
		return ErrNotFound
	}
	return fmt.Errorf("%w: %v", ErrUnavailable, err)
}
