// Package cli wires Cobra commands, global flags, and command-level helpers.
package cli

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/timestripe/timestripe-cli/internal/output"
	"github.com/timestripe/timestripe-cli/internal/pagination"
)

// outputFlags holds the mutually-exclusive format selectors. Populated by the
// persistent flags registered on the root command.
var outputFlags output.Flags

// listFlags holds the pagination selectors shared by every list subcommand.
type listFlags struct {
	Limit  int
	Offset int
	All    bool
}

// Execute runs the CLI with the given context. The context carries signal
// cancellation from main, so Ctrl-C aborts an in-flight paginated walk.
func Execute(ctx context.Context) error {
	return newRootCmd().ExecuteContext(ctx)
}

// newRootCmd builds the command tree. Split out from Execute so tests can
// drive it with SetArgs/SetOut/SetErr.
func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:           "timestripe",
		Short:         "Timestripe command-line interface",
		Long:          "timestripe is the official command-line client for the Timestripe API.",
		Version:       versionString(),
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.SetVersionTemplate("{{.Version}}")
	// Pre-register --version without a shorthand so Cobra does not auto-bind -v,
	// which we want to reserve for a future --verbose flag.
	root.Flags().Bool("version", false, "version for timestripe")

	pf := root.PersistentFlags()
	pf.BoolVar(&outputFlags.JSON, "json", false, "output JSON")
	pf.BoolVar(&outputFlags.YAML, "yaml", false, "output YAML")
	pf.BoolVar(&outputFlags.Markdown, "markdown", false, "output a Markdown table")
	pf.BoolVar(&outputFlags.Table, "table", false, "output a pretty table (default on a TTY)")
	pf.BoolVar(&outputFlags.CSV, "csv", false, "output CSV")
	root.MarkFlagsMutuallyExclusive("json", "yaml", "markdown", "table", "csv")

	root.AddCommand(
		newAuthCmd(),
		newSpacesCmd(),
		newBoardsCmd(),
		newBucketsCmd(),
		newGoalsCmd(),
		newCommentsCmd(),
		newEventsCmd(),
		newFoldersCmd(),
		newMembershipsCmd(),
		newUsersCmd(),
		newConfigCmd(),
		newCompletionCmd(),
		newVersionCmd(),
	)

	for _, c := range []*cobra.Command{
		newAuthLoginCmd(),
		newAuthLogoutCmd(),
		newAuthWhoamiCmd(),
		newAuthStatusCmd(),
	} {
		c.Hidden = true
		root.AddCommand(c)
	}

	return root
}

// addListFlags registers --limit, --offset, and --all on a list command, and
// rejects positional arguments. Every list command routes through here, so this
// is the single place that closes the `--checked false` hole.
func addListFlags(cmd *cobra.Command, f *listFlags) {
	cmd.Args = noArgsWithBoolHint
	cmd.Flags().IntVar(&f.Limit, "limit", pagination.DefaultLimit, "maximum number of items to return across all pages")
	cmd.Flags().IntVar(&f.Offset, "offset", 0, "starting offset into the result set")
	cmd.Flags().BoolVar(&f.All, "all", false, "fetch every page; ignores --limit")
}

func (f *listFlags) options() pagination.Options {
	return pagination.Options{Limit: f.Limit, Offset: f.Offset, All: f.All}
}
