// Package config resolves user configuration and environment overrides.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	// DefaultBackend is the production Timestripe site root.
	DefaultBackend = "https://timestripe.com"

	// APIPath is the path suffix for v3 of the REST API.
	APIPath = "/api/v3"

	// OAuthAuthorizePath is the path suffix for the OAuth authorization endpoint.
	OAuthAuthorizePath = "/oauth/authorize"

	// OAuthTokenPath is the path suffix for the OAuth token endpoint.
	OAuthTokenPath = "/oauth/token"

	// EnvBackend overrides the Timestripe site root when set.
	EnvBackend = "TIMESTRIPE_BACKEND"

	// EnvToken allows passing a bearer token via environment (bypasses stored credentials).
	EnvToken = "TIMESTRIPE_TOKEN"

	appDir = "timestripe"
)

// Backend returns the Timestripe site root (no path). Precedence: TIMESTRIPE_BACKEND env > default.
// Any trailing slash is stripped so callers can safely concatenate paths.
func Backend() string {
	v := os.Getenv(EnvBackend)
	if v == "" {
		v = DefaultBackend
	}
	return strings.TrimRight(v, "/")
}

// APIBase returns the full base URL for the REST API.
func APIBase() string { return Backend() + APIPath }

// OAuthAuthorizeURL returns the OAuth2 authorization endpoint URL.
func OAuthAuthorizeURL() string { return Backend() + OAuthAuthorizePath }

// OAuthTokenURL returns the OAuth2 token endpoint URL.
func OAuthTokenURL() string { return Backend() + OAuthTokenPath }

// Dir returns the config directory, creating it if missing.
// Honors XDG_CONFIG_HOME; falls back to os.UserConfigDir.
func Dir() (string, error) {
	base := os.Getenv("XDG_CONFIG_HOME")
	if base == "" {
		d, err := os.UserConfigDir()
		if err != nil {
			return "", fmt.Errorf("resolve config dir: %w", err)
		}
		base = d
	}
	dir := filepath.Join(base, appDir)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("create config dir %s: %w", dir, err)
	}
	return dir, nil
}

// Path returns the absolute path of a file inside the config directory.
func Path(name string) (string, error) {
	d, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(d, name), nil
}
