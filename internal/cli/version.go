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

// userAgent is the HTTP User-Agent sent on requests to Timestripe services.
func userAgent() string { return "timestripe-cli/" + version }

// versionString is the human-readable version line shared by `version` and `--version`.
func versionString() string {
	return fmt.Sprintf("timestripe %s (%s, built %s)\n", version, commit, date)
}

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the CLI version",
		RunE: func(cmd *cobra.Command, args []string) error {
			_, err := fmt.Fprint(cmd.OutOrStdout(), versionString())
			return err
		},
	}
}
