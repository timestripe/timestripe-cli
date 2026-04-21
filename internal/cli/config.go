package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/timestripe/timestripe-cli/internal/config"
)

func newConfigCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "config", Short: "Inspect effective CLI configuration"}
	cmd.AddCommand(newConfigShowCmd())
	return cmd
}

func newConfigShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show",
		Short: "Print the resolved configuration (backend URL, config path)",
		RunE: func(cmd *cobra.Command, args []string) error {
			dir, err := config.Dir()
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "backend:     %s\n", config.Backend())
			fmt.Fprintf(cmd.OutOrStdout(), "api:         %s\n", config.APIBase())
			fmt.Fprintf(cmd.OutOrStdout(), "oauth auth:  %s\n", config.OAuthAuthorizeURL())
			fmt.Fprintf(cmd.OutOrStdout(), "oauth token: %s\n", config.OAuthTokenURL())
			fmt.Fprintf(cmd.OutOrStdout(), "config dir:  %s\n", dir)
			return nil
		},
	}
}
