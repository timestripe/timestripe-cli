package auth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"html"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"runtime"
	"time"

	"golang.org/x/oauth2"
)

// OAuth endpoint + client configuration.
//
// These values are not yet issued by Timestripe at the time of this writing;
// once an OAuth application is provisioned for the public CLI, populate them
// either at build time via -ldflags or at runtime via env vars:
//
//	TIMESTRIPE_OAUTH_CLIENT_ID
//	TIMESTRIPE_OAUTH_AUTH_URL
//	TIMESTRIPE_OAUTH_TOKEN_URL
//
// The CLI is a *public* OAuth client and MUST NOT embed a client secret.
// PKCE (RFC 7636) protects the authorization code in transit.
var (
	DefaultClientID = "" // populated via -ldflags when published
	DefaultAuthURL  = ""
	DefaultTokenURL = ""
)

// OAuthEnvConfig returns the resolved OAuth config, honoring env overrides.
func OAuthEnvConfig() (clientID, authURL, tokenURL string) {
	clientID = firstNonEmpty(os.Getenv("TIMESTRIPE_OAUTH_CLIENT_ID"), DefaultClientID)
	authURL = firstNonEmpty(os.Getenv("TIMESTRIPE_OAUTH_AUTH_URL"), DefaultAuthURL)
	tokenURL = firstNonEmpty(os.Getenv("TIMESTRIPE_OAUTH_TOKEN_URL"), DefaultTokenURL)
	return
}

func firstNonEmpty(v ...string) string {
	for _, s := range v {
		if s != "" {
			return s
		}
	}
	return ""
}

// LoginPKCE runs the OAuth2 authorization-code flow with PKCE against a
// loopback redirect, returning persisted Credentials on success.
//
// Flow:
//  1. Start an HTTP server on 127.0.0.1:<random>.
//  2. Open the user's browser to the authorization URL.
//  3. Wait for the browser to hit /callback with ?code=...&state=....
//  4. Exchange code + PKCE verifier for tokens.
func LoginPKCE(ctx context.Context, scopes []string) (*Credentials, error) {
	clientID, authURL, tokenURL := OAuthEnvConfig()
	if clientID == "" || authURL == "" || tokenURL == "" {
		return nil, errors.New("OAuth is not configured yet; use `timestripe auth login --token <api-key>` instead " +
			"or set TIMESTRIPE_OAUTH_CLIENT_ID / TIMESTRIPE_OAUTH_AUTH_URL / TIMESTRIPE_OAUTH_TOKEN_URL")
	}

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("bind loopback listener: %w", err)
	}
	redirect := fmt.Sprintf("http://%s/callback", ln.Addr().String())

	state, err := randomString(24)
	if err != nil {
		_ = ln.Close()
		return nil, err
	}
	verifier := oauth2.GenerateVerifier()

	conf := &oauth2.Config{
		ClientID:    clientID,
		RedirectURL: redirect,
		Scopes:      scopes,
		Endpoint:    oauth2.Endpoint{AuthURL: authURL, TokenURL: tokenURL},
	}

	type result struct {
		code string
		err  error
	}
	done := make(chan result, 1)

	mux := http.NewServeMux()
	mux.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if e := q.Get("error"); e != "" {
			msg := fmt.Sprintf("%s: %s", e, q.Get("error_description"))
			http.Error(w, html.EscapeString(msg), http.StatusBadRequest)
			done <- result{err: errors.New(msg)}
			return
		}
		if q.Get("state") != state {
			http.Error(w, "state mismatch", http.StatusBadRequest)
			done <- result{err: errors.New("oauth state mismatch; possible CSRF")}
			return
		}
		code := q.Get("code")
		if code == "" {
			http.Error(w, "missing code", http.StatusBadRequest)
			done <- result{err: errors.New("authorization response missing `code`")}
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(successHTML))
		done <- result{code: code}
	})

	srv := &http.Server{Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	go func() { _ = srv.Serve(ln) }()
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
	}()

	authCodeURL := conf.AuthCodeURL(
		state,
		oauth2.AccessTypeOffline,
		oauth2.S256ChallengeOption(verifier),
	)
	if err := openBrowser(authCodeURL); err != nil {
		fmt.Fprintf(os.Stderr, "Open this URL to authorize:\n%s\n", authCodeURL)
	}

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case r := <-done:
		if r.err != nil {
			return nil, r.err
		}
		tok, err := conf.Exchange(ctx, r.code, oauth2.VerifierOption(verifier))
		if err != nil {
			return nil, fmt.Errorf("token exchange: %w", err)
		}
		return &Credentials{
			Type:         TypeOAuth,
			AccessToken:  tok.AccessToken,
			RefreshToken: tok.RefreshToken,
			ExpiresAt:    tok.Expiry,
		}, nil
	}
}

func randomString(nBytes int) (string, error) {
	b := make([]byte, nBytes)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func openBrowser(raw string) error {
	if _, err := url.Parse(raw); err != nil {
		return err
	}
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", raw)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", raw)
	default:
		cmd = exec.Command("xdg-open", raw)
	}
	return cmd.Start()
}

const successHTML = `<!doctype html>
<html><head><meta charset="utf-8"><title>Timestripe CLI</title>
<style>body{font-family:system-ui;margin:4rem auto;max-width:32rem;text-align:center;color:#222}</style>
</head><body>
<h1>Signed in</h1>
<p>You can close this tab and return to your terminal.</p>
</body></html>`
