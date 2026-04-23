package cli

import (
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/timestripe/timestripe-cli/internal/api"
	"github.com/timestripe/timestripe-cli/internal/auth"
	"github.com/timestripe/timestripe-cli/internal/config"
)

func newAuthCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "auth",
		Short: "Manage authentication",
	}
	cmd.AddCommand(newAuthLoginCmd(), newAuthLogoutCmd(), newAuthWhoamiCmd(), newAuthStatusCmd())
	return cmd
}

func newAuthLoginCmd() *cobra.Command {
	var token string
	var scopes []string
	cmd := &cobra.Command{
		Use:     "login",
		Aliases: []string{"signin"},
		Short:   "Authenticate the CLI (OAuth2 + PKCE, or personal token via --token)",
		Long: strings.TrimSpace(`
Authenticate the CLI against the Timestripe API.

With no flags, the browser is opened to complete an OAuth2 authorization-code
+ PKCE flow. A loopback HTTP server on 127.0.0.1 receives the callback.

Use --token <api-key> to store a personal bearer token instead (useful for
scripting and CI).
`),
		RunE: func(cmd *cobra.Command, args []string) error {
			store := auth.DefaultStore()
			if token != "" {
				creds := &auth.Credentials{Type: auth.TypeBearer, AccessToken: token}
				if err := store.Save(creds); err != nil {
					return fmt.Errorf("save credentials: %w", err)
				}
				fmt.Fprintln(cmd.OutOrStdout(), "Saved personal token.")
				return nil
			}
			creds, err := auth.LoginPKCE(cmd.Context(), scopes, userAgent())
			if err != nil {
				return err
			}
			if err := store.Save(creds); err != nil {
				return fmt.Errorf("save credentials: %w", err)
			}
			fmt.Fprintln(cmd.OutOrStdout(), "Signed in via OAuth.")
			return nil
		},
	}
	cmd.Flags().StringVar(&token, "token", "", "use a personal API token instead of OAuth")
	cmd.Flags().StringSliceVar(&scopes, "scope", []string{"read_write"}, "OAuth scopes to request")
	return cmd
}

func newAuthLogoutCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "logout",
		Aliases: []string{"signout"},
		Short:   "Remove stored credentials",
		RunE: func(cmd *cobra.Command, args []string) error {
			err := auth.DefaultStore().Delete()
			if err != nil && !errors.Is(err, auth.ErrNotFound) {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), "Signed out.")
			return nil
		},
	}
}

func newAuthWhoamiCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "whoami",
		Short: "Print the currently authenticated user",
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := newAPIClient(cmd.Context())
			if err != nil {
				return err
			}
			resp, err := client.UsersMeRetrieveWithResponse(cmd.Context())
			if err != nil {
				return err
			}
			if resp.JSON200 == nil {
				return apiError(resp.StatusCode(), resp.Body)
			}
			u := resp.JSON200
			tab := &userTabular{[]api.User{*u}}
			return renderOrFail(cmd, u, tab.build())
		},
	}
}

func newAuthStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show authentication state without calling the API",
		RunE: func(cmd *cobra.Command, args []string) error {
			creds, err := auth.DefaultStore().Load()
			if errors.Is(err, auth.ErrNotFound) {
				fmt.Fprintln(cmd.OutOrStdout(), "Not signed in.")
				return nil
			}
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Signed in (%s) — backend: %s\n", creds.Type, config.Backend())
			if creds.Type == auth.TypeOAuth && !creds.ExpiresAt.IsZero() {
				fmt.Fprintf(cmd.OutOrStdout(), "Access token expires: %s\n", creds.ExpiresAt.Local().Format("2006-01-02 15:04:05 MST"))
			}
			return nil
		},
	}
}
