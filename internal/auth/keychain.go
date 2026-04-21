package auth

import (
	"errors"

	"github.com/zalando/go-keyring"
)

const (
	keychainService = "timestripe-cli"
	keychainUser    = "default"
)

type keychainStore struct{}

func newKeychainStore() *keychainStore { return &keychainStore{} }

// available probes the keychain with a cheap lookup. If the OS keyring is not
// accessible (headless CI, locked login keychain, etc.), we fall back to file.
func (k *keychainStore) available() bool {
	_, err := keyring.Get(keychainService, "__probe__")
	return err == nil || errors.Is(err, keyring.ErrNotFound)
}

func (k *keychainStore) Load() (*Credentials, error) {
	raw, err := keyring.Get(keychainService, keychainUser)
	if err != nil {
		if errors.Is(err, keyring.ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return decode([]byte(raw))
}

func (k *keychainStore) Save(c *Credentials) error {
	b, err := encode(c)
	if err != nil {
		return err
	}
	return keyring.Set(keychainService, keychainUser, string(b))
}

func (k *keychainStore) Delete() error {
	err := keyring.Delete(keychainService, keychainUser)
	if errors.Is(err, keyring.ErrNotFound) {
		return ErrNotFound
	}
	return err
}
