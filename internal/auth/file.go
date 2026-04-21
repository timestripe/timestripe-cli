package auth

import (
	"errors"
	"fmt"
	"os"

	"github.com/timestripe/timestripe-cli/internal/config"
)

const credentialsFile = "credentials.json"

type fileStore struct{}

func (f *fileStore) path() (string, error) { return config.Path(credentialsFile) }

func (f *fileStore) Load() (*Credentials, error) {
	p, err := f.path()
	if err != nil {
		return nil, err
	}
	b, err := os.ReadFile(p)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return decode(b)
}

func (f *fileStore) Save(c *Credentials) error {
	p, err := f.path()
	if err != nil {
		return err
	}
	b, err := encode(c)
	if err != nil {
		return err
	}
	// WriteFile with 0600 for secrets-at-rest.
	if err := os.WriteFile(p, b, 0o600); err != nil {
		return fmt.Errorf("write credentials: %w", err)
	}
	return nil
}

func (f *fileStore) Delete() error {
	p, err := f.path()
	if err != nil {
		return err
	}
	if err := os.Remove(p); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return ErrNotFound
		}
		return err
	}
	return nil
}
