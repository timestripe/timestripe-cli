package cli

import (
	"fmt"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// walkCommands visits every command in the tree, including the root.
func walkCommands(t *testing.T, fn func(path string, c *cobra.Command)) {
	t.Helper()
	var walk func(c *cobra.Command, prefix string)
	walk = func(c *cobra.Command, prefix string) {
		path := strings.TrimSpace(prefix + " " + strings.Fields(c.Use)[0])
		fn(path, c)
		for _, sub := range c.Commands() {
			walk(sub, path)
		}
	}
	walk(newRootCmd(), "")
}

// eachFlag visits every flag reachable from a command, local and inherited.
func eachFlag(c *cobra.Command, fn func(*pflag.Flag)) {
	c.Flags().VisitAll(fn)
	c.InheritedFlags().VisitAll(fn)
}

// A shorthand must mean one thing across the whole CLI. -s is space, never
// search; -b is bucket, never board. Without this test the scheme rots as
// commands are added.
func TestShorthandsAreGloballyConsistent(t *testing.T) {
	// shorthand -> long name it was first seen bound to
	owner := map[string]string{}
	// where each binding was found, for a useful failure message
	where := map[string]string{}

	walkCommands(t, func(path string, c *cobra.Command) {
		eachFlag(c, func(f *pflag.Flag) {
			if f.Shorthand == "" {
				return
			}
			key := f.Shorthand
			if prev, ok := owner[key]; ok {
				if prev != f.Name {
					t.Errorf("-%s means --%s on %q but --%s on %q; a letter must mean one thing",
						key, f.Name, path, prev, where[key])
				}
				return
			}
			owner[key] = f.Name
			where[key] = path
		})
	})

	// Lock in the intended scheme so a future change is deliberate.
	want := map[string]string{
		"f": "file", "n": "name", "s": "space", "b": "bucket", "a": "assignee",
		"H": "horizon", "d": "date", "q": "search", "l": "limit", "A": "all",
		"h": "help", "v": "verbose",
	}
	for short, long := range want {
		if got, ok := owner[short]; ok && got != long {
			t.Errorf("-%s is bound to --%s, expected --%s", short, got, long)
		}
	}
	for short, long := range owner {
		if _, ok := want[short]; !ok {
			t.Errorf("-%s (--%s) is not part of the documented scheme", short, long)
		}
	}
}

// Within a single command, two flags obviously cannot share a letter — pflag
// panics on that. This catches it at test time with a readable message.
func TestNoShorthandCollisionsWithinACommand(t *testing.T) {
	walkCommands(t, func(path string, c *cobra.Command) {
		seen := map[string]string{}
		eachFlag(c, func(f *pflag.Flag) {
			if f.Shorthand == "" {
				return
			}
			if prev, ok := seen[f.Shorthand]; ok && prev != f.Name {
				t.Errorf("%s: -%s is bound to both --%s and --%s", path, f.Shorthand, prev, f.Name)
			}
			seen[f.Shorthand] = f.Name
		})
	})
}

// -v must stay --verbose, not get auto-bound to --version by Cobra.
func TestVerboseOwnsV(t *testing.T) {
	root := newRootCmd()
	f := root.PersistentFlags().ShorthandLookup("v")
	if f == nil {
		t.Fatal("-v is not bound")
	}
	if f.Name != "verbose" {
		t.Errorf("-v is bound to --%s, want --verbose", f.Name)
	}
}

// The format selectors stay long-form: they live in scripts, where clarity
// wins, and -y would fight a future --yes.
func TestFormatFlagsHaveNoShorthand(t *testing.T) {
	root := newRootCmd()
	for _, name := range []string{"json", "yaml", "markdown", "table", "csv"} {
		f := root.PersistentFlags().Lookup(name)
		if f == nil {
			t.Fatalf("--%s is missing", name)
		}
		if f.Shorthand != "" {
			t.Errorf("--%s should not have the shorthand -%s", name, f.Shorthand)
		}
	}
}

// Spot-check that the letters actually work end to end.
func TestShorthandsParse(t *testing.T) {
	var seen []string
	srv := goalsQueryStub(t, &seen)
	out, err := runAgainst(t, srv.URL, "goals", "list", "-q", "report", "-l", "5", "-H", "week")
	if err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	if len(seen) == 0 {
		t.Fatal("no request recorded")
	}
	for _, want := range []string{"search=report", "limit=5", "horizon=week"} {
		if !strings.Contains(seen[0], want) {
			t.Errorf("query %q missing %q", seen[0], want)
		}
	}
}

// Guard against a silent regression in which a long flag loses its shorthand.
func TestExpectedShorthandsArePresent(t *testing.T) {
	cases := []struct{ path, long, short string }{
		{"timestripe goals list", "search", "q"},
		{"timestripe goals list", "limit", "l"},
		{"timestripe goals list", "all", "A"},
		{"timestripe goals list", "horizon", "H"},
		{"timestripe goals create", "name", "n"},
		{"timestripe goals create", "space", "s"},
		{"timestripe goals create", "bucket", "b"},
		{"timestripe goals create", "assignee", "a"},
		{"timestripe goals create", "date", "d"},
		{"timestripe goals create", "file", "f"},
	}
	found := map[string]*pflag.Flag{}
	walkCommands(t, func(path string, c *cobra.Command) {
		eachFlag(c, func(f *pflag.Flag) {
			found[fmt.Sprintf("%s\x00%s", path, f.Name)] = f
		})
	})
	for _, tc := range cases {
		f, ok := found[fmt.Sprintf("%s\x00%s", tc.path, tc.long)]
		if !ok {
			t.Errorf("%s has no --%s", tc.path, tc.long)
			continue
		}
		if f.Shorthand != tc.short {
			t.Errorf("%s --%s has shorthand %q, want %q", tc.path, tc.long, f.Shorthand, tc.short)
		}
	}
}
