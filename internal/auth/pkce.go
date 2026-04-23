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

	"github.com/timestripe/timestripe-cli/internal/config"
)

// ClientID is the OAuth client identifier for the CLI.
const ClientID = "timestripe-cli"

// LoginPKCE runs the OAuth2 authorization-code flow with PKCE against a
// loopback redirect on an OS-assigned random port. The backend accepts any
// 127.0.0.1 loopback redirect URI, so no static port registration is needed.
//
// userAgent, if non-empty, is sent on the token-exchange request so the
// OAuth server can identify the CLI client.
//
// Flow:
//  1. Start an HTTP server on 127.0.0.1:0 (OS picks a free port).
//  2. Open the user's browser to the authorization URL.
//  3. Wait for the browser to hit /callback with ?code=...&state=....
//  4. Exchange code + PKCE verifier for tokens.
func LoginPKCE(ctx context.Context, scopes []string, userAgent string) (*Credentials, error) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("bind loopback listener: %w", err)
	}
	redirectURL := fmt.Sprintf("http://127.0.0.1:%d/callback", ln.Addr().(*net.TCPAddr).Port)

	state, err := randomString(24)
	if err != nil {
		_ = ln.Close()
		return nil, err
	}
	verifier := oauth2.GenerateVerifier()

	conf := &oauth2.Config{
		ClientID:    ClientID,
		RedirectURL: redirectURL,
		Scopes:       scopes,
		Endpoint: oauth2.Endpoint{
			AuthURL:  config.OAuthAuthorizeURL(),
			TokenURL: config.OAuthTokenURL(),
		},
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

	exchangeCtx := ctx
	if userAgent != "" {
		exchangeCtx = context.WithValue(ctx, oauth2.HTTPClient, &http.Client{
			Transport: &userAgentTransport{ua: userAgent, base: http.DefaultTransport},
		})
	}

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case r := <-done:
		if r.err != nil {
			return nil, r.err
		}
		tok, err := conf.Exchange(exchangeCtx, r.code, oauth2.VerifierOption(verifier))
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

// userAgentTransport sets User-Agent on each request before delegating.
type userAgentTransport struct {
	ua   string
	base http.RoundTripper
}

func (t *userAgentTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	clone := req.Clone(req.Context())
	clone.Header.Set("User-Agent", t.ua)
	return t.base.RoundTrip(clone)
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
