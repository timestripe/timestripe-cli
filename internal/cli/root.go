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

// verbose logs API traffic to stderr. Populated by the persistent --verbose flag.
var verbose bool

// Help groups, so a root listing of ~17 commands stays readable.
const (
	groupQuick     = "quick"
	groupResources = "resources"
	groupSetup     = "setup"
)

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
	// Pre-register --version without a shorthand: -v is --verbose.
	root.Flags().Bool("version", false, "version for timestripe")

	pf := root.PersistentFlags()
	pf.BoolVarP(&verbose, "verbose", "v", false, "log each API request and its raw response to stderr")
	pf.BoolVar(&outputFlags.JSON, "json", false, "output JSON")
	pf.BoolVar(&outputFlags.YAML, "yaml", false, "output YAML")
	pf.BoolVar(&outputFlags.Markdown, "markdown", false, "output a Markdown table")
	pf.BoolVar(&outputFlags.Table, "table", false, "output a pretty table (default on a TTY)")
	pf.BoolVar(&outputFlags.CSV, "csv", false, "output CSV")
	root.MarkFlagsMutuallyExclusive("json", "yaml", "markdown", "table", "csv")

	root.AddGroup(
		&cobra.Group{ID: groupQuick, Title: "Quick commands:"},
		&cobra.Group{ID: groupResources, Title: "Resources:"},
		&cobra.Group{ID: groupSetup, Title: "Setup:"},
	)

	root.AddCommand(newAddCmd(), newDoneCmd(), newReopenCmd())

	for _, c := range []*cobra.Command{
		newSpacesCmd(), newBoardsCmd(), newBucketsCmd(), newGoalsCmd(),
		newCommentsCmd(), newEventsCmd(), newFoldersCmd(), newMembershipsCmd(),
		newUsersCmd(),
	} {
		c.GroupID = groupResources
		root.AddCommand(c)
	}

	for _, c := range []*cobra.Command{
		newAuthCmd(), newConfigCmd(), newCompletionCmd(), newVersionCmd(),
	} {
		c.GroupID = groupSetup
		root.AddCommand(c)
	}

	// Muscle-memory aliases: `timestripe login` alongside `timestripe auth login`.
	// Hidden, since they duplicate what `auth` already advertises.
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
	cmd.Flags().IntVarP(&f.Limit, "limit", "l", pagination.DefaultLimit, "maximum number of items to return across all pages")
	cmd.Flags().IntVar(&f.Offset, "offset", 0, "starting offset into the result set")
	cmd.Flags().BoolVarP(&f.All, "all", "A", false, "fetch every page; ignores --limit")
}

func (f *listFlags) options() pagination.Options {
	return pagination.Options{Limit: f.Limit, Offset: f.Offset, All: f.All}
}
