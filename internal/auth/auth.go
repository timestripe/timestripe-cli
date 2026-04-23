// Package auth manages credentials for the Timestripe CLI.
//
// Two credential types are supported:
//   - "bearer": personal API token
//   - "oauth":  OAuth2 authorization-code + PKCE access/refresh token pair
//
// Credentials are persisted to $XDG_CONFIG_HOME/timestripe/credentials.json.
package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/timestripe/timestripe-cli/internal/config"
)

// Type enumerates supported credential kinds.
type Type string

const (
	TypeBearer Type = "bearer"
	TypeOAuth  Type = "oauth"
)

// Credentials is the persisted auth state for a user.
type Credentials struct {
	Type         Type      `json:"type"`
	AccessToken  string    `json:"accessToken"`
	RefreshToken string    `json:"refreshToken,omitempty"`
	ExpiresAt    time.Time `json:"expiresAt,omitempty"`
	// Backend is the Timestripe site root the user signed into.
	// Used to pin subsequent requests to the same environment.
	Backend string `json:"backend,omitempty"`
}

// Expired reports whether the access token has (or is about to) expire.
// Always false for bearer tokens (personal API keys do not expire client-side).
func (c *Credentials) Expired() bool {
	if c.Type != TypeOAuth || c.ExpiresAt.IsZero() {
		return false
	}
	return time.Now().Add(30 * time.Second).After(c.ExpiresAt)
}

// Store persists credentials to some backend.
type Store interface {
	Load() (*Credentials, error)
	Save(*Credentials) error
	Delete() error
}

// ErrNotFound is returned when no credentials are stored.
var ErrNotFound = errors.New("no credentials stored; run `timestripe auth login`")

// DefaultStore returns the file-backed credentials store.
func DefaultStore() Store { return &fileStore{} }

// Resolve returns the caller's current credentials, respecting the
// TIMESTRIPE_TOKEN environment override (which wins over any stored creds).
// Callers should pass ctx through to token-refresh flows when Expired().
func Resolve(ctx context.Context) (*Credentials, error) {
	if tok := os.Getenv(config.EnvToken); tok != "" {
		return &Credentials{Type: TypeBearer, AccessToken: tok}, nil
	}
	c, err := DefaultStore().Load()
	if err != nil {
		return nil, err
	}
	if c.Expired() {
		return nil, fmt.Errorf("access token expired at %s; run `timestripe auth login` to refresh", c.ExpiresAt.Format(time.RFC3339))
	}
	return c, nil
}

// encode/decode are shared between concrete stores.
func encode(c *Credentials) ([]byte, error) { return json.Marshal(c) }
func decode(b []byte) (*Credentials, error) {
	var c Credentials
	if err := json.Unmarshal(b, &c); err != nil {
		return nil, err
	}
	return &c, nil
}
