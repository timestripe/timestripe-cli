package cli

import (
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// boolLiterals are the values pflag accepts for a boolean flag. A stray
// positional matching one of these almost always means the user wrote
// `--flag value` instead of the required `--flag=value`.
var boolLiterals = map[string]bool{
	"true": true, "false": true,
	"t": true, "f": true,
	"1": true, "0": true,
	"yes": true, "no": true,
}

// noArgsWithBoolHint rejects positional arguments on commands that take none.
//
// It exists because Cobra treats a nil Args as ArbitraryArgs, which made
// `timestripe goals list --checked false` parse as --checked=true with "false"
// silently discarded — a wrong answer with exit code 0. The bool-literal case
// gets a targeted hint since that is by far the most common way to trip it.
func noArgsWithBoolHint(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		return nil
	}
	if boolLiterals[strings.ToLower(args[0])] {
		if names := boolFlagNames(cmd); len(names) > 0 {
			example := exampleBoolFlag(names)
			return fmt.Errorf(
				"unexpected argument %q\nboolean flags need an \"=\": --%s=%s, not --%s %s\nboolean flags here: --%s",
				args[0],
				example, strings.ToLower(args[0]),
				example, strings.ToLower(args[0]),
				strings.Join(names, ", --"),
			)
		}
	}
	return fmt.Errorf("unexpected argument %q for %q", args[0], cmd.CommandPath())
}

// boolFlagNames lists the command's boolean flags, sorted, excluding the
// persistent output-format selectors (which are never what the user meant).
func boolFlagNames(cmd *cobra.Command) []string {
	skip := map[string]bool{"json": true, "yaml": true, "markdown": true, "table": true, "csv": true, "help": true}
	var names []string
	cmd.Flags().VisitAll(func(f *pflag.Flag) {
		if f.Value.Type() == "bool" && !skip[f.Name] && !f.Hidden {
			names = append(names, f.Name)
		}
	})
	sort.Strings(names)
	return names
}

// exampleBoolFlag picks the flag to show in the hint. "all" sorts first on
// every list command but is rarely the one the user fumbled, so prefer any
// other flag when one exists.
func exampleBoolFlag(names []string) string {
	for _, n := range names {
		if n != "all" {
			return n
		}
	}
	return names[0]
}

// optionalPositional validates `create [x]` forms: at most one positional, and
// never alongside the flag that sets the same field. On `goals create` the
// positional is applied after the flag and silently wins; on `comments create`
// the flag wins. Rather than document which, reject the ambiguity.
func optionalPositional(flag string) cobra.PositionalArgs {
	return func(cmd *cobra.Command, args []string) error {
		if len(args) > 1 {
			return fmt.Errorf("accepts at most 1 positional %s, received %d; quote values containing spaces", flag, len(args))
		}
		if len(args) == 1 && cmd.Flags().Changed(flag) {
			return fmt.Errorf("cannot set the %s twice: positional %q and --%s %q; pick one",
				flag, args[0], flag, cmd.Flags().Lookup(flag).Value.String())
		}
		return nil
	}
}
