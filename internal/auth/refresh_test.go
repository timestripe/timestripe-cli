package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// tokenServer stands in for the OAuth token endpoint, counting exchanges and
// asserting the client authenticates in the request body (AuthStyleInParams).
func tokenServer(t *testing.T, calls *int32) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(calls, 1)
		if err := r.ParseForm(); err != nil {
			t.Errorf("parse form: %v", err)
		}
		if got := r.PostForm.Get("client_id"); got != ClientID {
			t.Errorf("client_id in body = %q, want %q (AuthStyleInParams)", got, ClientID)
		}
		if got := r.PostForm.Get("grant_type"); got != "refresh_token" {
			t.Errorf("grant_type = %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token":  fmt.Sprintf("new-access-%d", n),
			"refresh_token": fmt.Sprintf("new-refresh-%d", n),
			"token_type":    "Bearer",
			"expires_in":    3600,
		})
	}))
}

// seed writes expired OAuth credentials into an isolated config dir.
func seed(t *testing.T, srvURL string) {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("TIMESTRIPE_BACKEND", srvURL)
	t.Setenv("TIMESTRIPE_TOKEN", "")
	c := &Credentials{
		Type:         TypeOAuth,
		AccessToken:  "stale",
		RefreshToken: "refresh-me",
		ExpiresAt:    time.Now().Add(-time.Hour),
		Backend:      srvURL,
	}
	if err := DefaultStore().Save(c); err != nil {
		t.Fatal(err)
	}
}

func TestResolveRefreshesExpiredToken(t *testing.T) {
	var calls int32
	srv := tokenServer(t, &calls)
	defer srv.Close()
	seed(t, srv.URL)

	got, err := Resolve(context.Background(), "timestripe-cli/test")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if got.AccessToken != "new-access-1" {
		t.Errorf("access token = %q, want the refreshed one", got.AccessToken)
	}
	if got.RefreshToken != "new-refresh-1" {
		t.Errorf("rotated refresh token not stored: %q", got.RefreshToken)
	}
	if got.Backend != srv.URL {
		t.Errorf("Backend was dropped by refresh: %q", got.Backend)
	}

	// The new token must be persisted, not just returned.
	reloaded, err := DefaultStore().Load()
	if err != nil {
		t.Fatal(err)
	}
	if reloaded.AccessToken != "new-access-1" {
		t.Errorf("refreshed token not saved: %q", reloaded.AccessToken)
	}

	// A second Resolve is now a cache hit, not another exchange.
	if _, err := Resolve(context.Background(), ""); err != nil {
		t.Fatal(err)
	}
	if n := atomic.LoadInt32(&calls); n != 1 {
		t.Errorf("token endpoint called %d times, want 1", n)
	}
}

// Concurrent invocations must not each burn the refresh token.
func TestConcurrentResolveRefreshesOnce(t *testing.T) {
	var calls int32
	srv := tokenServer(t, &calls)
	defer srv.Close()
	seed(t, srv.URL)

	var wg sync.WaitGroup
	errs := make(chan error, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := Resolve(context.Background(), ""); err != nil {
				errs <- err
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Errorf("concurrent Resolve failed: %v", err)
	}
	if n := atomic.LoadInt32(&calls); n != 1 {
		t.Errorf("token endpoint called %d times under concurrency, want 1", n)
	}
}

func TestResolveWithoutRefreshTokenStillErrors(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("TIMESTRIPE_TOKEN", "")
	c := &Credentials{Type: TypeOAuth, AccessToken: "stale", ExpiresAt: time.Now().Add(-time.Hour)}
	if err := DefaultStore().Save(c); err != nil {
		t.Fatal(err)
	}
	if _, err := Resolve(context.Background(), ""); err == nil {
		t.Fatal("expected an error with no refresh token")
	}
}

func TestBearerTokenNeverExpires(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("TIMESTRIPE_TOKEN", "")
	c := &Credentials{Type: TypeBearer, AccessToken: "pat", ExpiresAt: time.Now().Add(-time.Hour)}
	if err := DefaultStore().Save(c); err != nil {
		t.Fatal(err)
	}
	got, err := Resolve(context.Background(), "")
	if err != nil {
		t.Fatalf("bearer credentials should not expire: %v", err)
	}
	if got.AccessToken != "pat" {
		t.Errorf("access token = %q", got.AccessToken)
	}
}

// An abandoned lock must not wedge the CLI forever.
func TestStaleLockIsBroken(t *testing.T) {
	var calls int32
	srv := tokenServer(t, &calls)
	defer srv.Close()
	seed(t, srv.URL)

	dir := os.Getenv("XDG_CONFIG_HOME")
	lock := filepath.Join(dir, "timestripe", lockFile)
	if err := os.WriteFile(lock, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-2 * lockStale)
	if err := os.Chtimes(lock, old, old); err != nil {
		t.Fatal(err)
	}

	done := make(chan error, 1)
	go func() { _, err := Resolve(context.Background(), ""); done <- err }()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Resolve: %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("Resolve hung on a stale lock")
	}
}
