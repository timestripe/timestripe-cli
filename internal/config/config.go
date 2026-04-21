// Package config resolves user configuration and environment overrides.
package config

import (
	"fmt"
	"os"
	"path/filepath"
)

const (
	// DefaultBackend is the production Timestripe API base URL.
	DefaultBackend = "https://timestripe.com/api/v3"

	// EnvBackend overrides the API base URL when set.
	EnvBackend = "TIMESTRIPE_BACKEND"

	// EnvToken allows passing a bearer token via environment (bypasses stored credentials).
	EnvToken = "TIMESTRIPE_TOKEN"

	appDir = "timestripe"
)

// Backend returns the API base URL. Precedence: TIMESTRIPE_BACKEND env > default.
func Backend() string {
	if v := os.Getenv(EnvBackend); v != "" {
		return v
	}
	return DefaultBackend
}

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
