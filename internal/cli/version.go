package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

// Populated via -ldflags at build time (see Makefile).
var (
	version = "dev"
	commit  = "unknown"
	date    = "unknown"
)

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the CLI version",
		RunE: func(cmd *cobra.Command, args []string) error {
			_, err := fmt.Fprintf(cmd.OutOrStdout(), "timestripe %s (%s, built %s)\n", version, commit, date)
			return err
		},
	}
}
