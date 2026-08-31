package cli

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/timestripe/timestripe-cli/internal/api"
	"github.com/timestripe/timestripe-cli/internal/auth"
	"github.com/timestripe/timestripe-cli/internal/config"
	"github.com/timestripe/timestripe-cli/internal/output"
	"github.com/timestripe/timestripe-cli/internal/pagination"
)

// httpTimeout bounds a single API request. The generated client otherwise uses
// http.DefaultClient, which has no timeout at all.
const httpTimeout = 60 * time.Second

// newAPIClient builds an authenticated API client from stored credentials.
// Errors here are user-facing: missing token, expired OAuth token, etc.
func newAPIClient(ctx context.Context) (*api.ClientWithResponses, error) {
	creds, err := auth.Resolve(ctx)
	if err != nil {
		return nil, err
	}
	ua := userAgent()
	editor := func(ctx context.Context, req *http.Request) error {
		req.Header.Set("Authorization", "Bearer "+creds.AccessToken)
		req.Header.Set("User-Agent", ua)
		return nil
	}
	hc := &http.Client{Timeout: httpTimeout}
	if verbose {
		hc.Transport = &verboseTransport{base: http.DefaultTransport, w: os.Stderr}
	}
	return api.NewClientWithResponses(config.APIBase(),
		api.WithRequestEditorFn(editor),
		api.WithHTTPClient(hc),
	)
}

// pickFormat resolves the output format against the command's writer.
func pickFormat(cmd *cobra.Command) (output.Format, error) {
	return output.Resolve(cmd.OutOrStdout(), outputFlags)
}

// renderOrFail writes the value in the selected format, or exits with an error.
func renderOrFail(cmd *cobra.Command, v any, t *output.Tabular) error {
	f, err := pickFormat(cmd)
	if err != nil {
		return err
	}
	return output.Render(cmd.OutOrStdout(), f, v, t)
}

// renderListOrFail renders a paginated envelope and, for tabular formats,
// writes a one-line pagination hint to stderr when more results are available.
// JSON and YAML already carry this information in the pageInfo envelope.
func renderListOrFail[T any](cmd *cobra.Command, env *pagination.Envelope[T], offset int, t *output.Tabular) error {
	f, err := pickFormat(cmd)
	if err != nil {
		return err
	}
	if err := output.Render(cmd.OutOrStdout(), f, env, t); err != nil {
		return err
	}
	switch f {
	case output.FormatTable, output.FormatMarkdown, output.FormatCSV:
		if env.PageInfo.HasMore && len(env.Items) > 0 {
			next := offset + len(env.Items)
			fmt.Fprintf(cmd.ErrOrStderr(),
				"Showing %d of %d. Use --offset %d or --all for more.\n",
				len(env.Items), env.PageInfo.Count, next)
		}
	}
	return nil
}

// verboseTransport logs each request and its response body to w. Enabled by
// --verbose; the body is what makes an unrecognized error shape debuggable.
type verboseTransport struct {
	base http.RoundTripper
	w    io.Writer
}

func (t *verboseTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	start := time.Now()
	resp, err := t.base.RoundTrip(req)
	if err != nil {
		fmt.Fprintf(t.w, "> %s %s\n< error after %s: %v\n", req.Method, req.URL, time.Since(start).Round(time.Millisecond), err)
		return resp, err
	}
	fmt.Fprintf(t.w, "> %s %s\n< %s in %s\n", req.Method, req.URL, resp.Status, time.Since(start).Round(time.Millisecond))
	if resp.Body != nil {
		body, readErr := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		resp.Body = io.NopCloser(bytes.NewReader(body))
		if readErr == nil && len(body) > 0 {
			fmt.Fprintf(t.w, "< %s\n", string(body))
		}
	}
	return resp, nil
}
