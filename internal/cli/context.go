package cli

import (
	"context"
	"fmt"
	"net/http"

	"github.com/spf13/cobra"

	"github.com/timestripe/timestripe-cli/internal/api"
	"github.com/timestripe/timestripe-cli/internal/auth"
	"github.com/timestripe/timestripe-cli/internal/config"
	"github.com/timestripe/timestripe-cli/internal/output"
	"github.com/timestripe/timestripe-cli/internal/pagination"
)

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
	return api.NewClientWithResponses(config.APIBase(), api.WithRequestEditorFn(editor))
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

// apiError pulls a readable message out of a non-2xx API response body.
func apiError(status int, body []byte) error {
	if len(body) == 0 {
		return fmt.Errorf("api returned status %d", status)
	}
	return fmt.Errorf("api returned status %d: %s", status, string(body))
}

