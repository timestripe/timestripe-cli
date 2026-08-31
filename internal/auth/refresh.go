package auth

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"time"

	"golang.org/x/oauth2"

	"github.com/timestripe/timestripe-cli/internal/config"
)

const (
	// lockFile guards the read-refresh-write cycle against concurrent CLI
	// invocations. Agents routinely run several at once, and if the server
	// rotates refresh tokens, a race loses one of them permanently.
	lockFile = "credentials.lock"

	// lockWait bounds how long we spin for the lock before giving up and
	// refreshing anyway — better a possible double-refresh than a hang.
	lockWait = 2 * time.Second

	// lockStale is how old a lock must be before we assume the holder died.
	lockStale = 30 * time.Second
)

// oauthConfig builds the config used for refreshing. AuthStyleInParams is set
// explicitly: the CLI is a public PKCE client with no secret, so client_id
// belongs in the body. Left unset, oauth2 probes the style and caches the
// result per endpoint, which makes the first refresh non-deterministic.
func oauthConfig() *oauth2.Config {
	return &oauth2.Config{
		ClientID: ClientID,
		Endpoint: oauth2.Endpoint{
			TokenURL:  config.OAuthTokenURL(),
			AuthStyle: oauth2.AuthStyleInParams,
		},
	}
}

// Refresh exchanges the stored refresh token for a new access token, mutating c
// in place. Backend is preserved: it is set at login and is not part of the
// token response.
func Refresh(ctx context.Context, c *Credentials, userAgent string) error {
	if c.Type != TypeOAuth {
		return errors.New("only OAuth credentials can be refreshed")
	}
	if c.RefreshToken == "" {
		return errors.New("no refresh token stored")
	}
	if userAgent != "" {
		ctx = context.WithValue(ctx, oauth2.HTTPClient, &http.Client{
			Transport: &userAgentTransport{ua: userAgent, base: http.DefaultTransport},
			Timeout:   30 * time.Second,
		})
	}
	src := oauthConfig().TokenSource(ctx, &oauth2.Token{
		RefreshToken: c.RefreshToken,
		Expiry:       c.ExpiresAt,
	})
	tok, err := src.Token()
	if err != nil {
		return err
	}
	c.AccessToken = tok.AccessToken
	c.ExpiresAt = tok.Expiry
	// The server may or may not rotate the refresh token; keep the old one if
	// the response omits it.
	if tok.RefreshToken != "" {
		c.RefreshToken = tok.RefreshToken
	}
	return nil
}

// refreshLocked performs the refresh under a lock, re-reading credentials after
// acquiring it. A concurrent invocation has usually already refreshed, in which
// case this returns those credentials without a second network round trip.
func refreshLocked(ctx context.Context, store Store, userAgent string) (*Credentials, error) {
	release, err := acquireLock()
	if err == nil {
		defer release()
		// Re-read: another process may have refreshed while we waited.
		if fresh, err := store.Load(); err == nil && !fresh.Expired() {
			return fresh, nil
		}
	}
	c, err := store.Load()
	if err != nil {
		return nil, err
	}
	if err := Refresh(ctx, c, userAgent); err != nil {
		return nil, err
	}
	if err := store.Save(c); err != nil {
		return nil, fmt.Errorf("save refreshed credentials: %w", err)
	}
	return c, nil
}

// acquireLock takes an exclusive lock, breaking one that looks abandoned.
// Returns a release func. A failure to lock is not fatal to the caller.
func acquireLock() (func(), error) {
	p, err := config.Path(lockFile)
	if err != nil {
		return nil, err
	}
	deadline := time.Now().Add(lockWait)
	for {
		f, err := os.OpenFile(p, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
		if err == nil {
			_ = f.Close()
			return func() { _ = os.Remove(p) }, nil
		}
		if !errors.Is(err, os.ErrExist) {
			return nil, err
		}
		if st, statErr := os.Stat(p); statErr == nil && time.Since(st.ModTime()) > lockStale {
			_ = os.Remove(p)
			continue
		}
		if time.Now().After(deadline) {
			return nil, errors.New("timed out waiting for the credentials lock")
		}
		time.Sleep(25 * time.Millisecond)
	}
}
