package credential

import (
	"errors"

	keyring "github.com/zalando/go-keyring"
)

const service = "sateia-cli"

var ErrNotFound = errors.New("credential not found")

type Store interface {
	Get(account string) (string, error)
	Set(account, token string) error
	Delete(account string) error
}

type KeyringStore struct{}

func (KeyringStore) Get(account string) (string, error) {
	value, err := keyring.Get(service, account)
	if errors.Is(err, keyring.ErrNotFound) {
		return "", ErrNotFound
	}
	return value, err
}

func (KeyringStore) Set(account, token string) error {
	return keyring.Set(service, account, token)
}

func (KeyringStore) Delete(account string) error {
	err := keyring.Delete(service, account)
	if errors.Is(err, keyring.ErrNotFound) {
		return ErrNotFound
	}
	return err
}
