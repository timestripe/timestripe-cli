package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

type cobraCmd = cobra.Command

// run drives the real command tree with args, returning combined output and the
// error Execute would have surfaced to main.
func run(t *testing.T, args ...string) (string, error) {
	t.Helper()
	root := newRootCmd()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs(args)
	err := root.Execute()
	return out.String(), err
}

// Regression: `goals list --checked false` used to parse as --checked=true and
// silently discard "false", returning completed goals with exit code 0.
func TestListRejectsBoolPositional(t *testing.T) {
	_, err := run(t, "goals", "list", "--checked", "false")
	if err == nil {
		t.Fatal("expected an error for `--checked false`, got nil")
	}
	for _, want := range []string{`unexpected argument "false"`, `--checked=false`} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error missing %q:\n%s", want, err)
		}
	}
	// "all" sorts first among the bool flags but is not what the user fumbled.
	if strings.Contains(err.Error(), "--all=false, not") {
		t.Errorf("hint should not use --all as the example:\n%s", err)
	}
}

func TestListRejectsStrayPositional(t *testing.T) {
	_, err := run(t, "spaces", "list", "junk")
	if err == nil || !strings.Contains(err.Error(), `unexpected argument "junk"`) {
		t.Fatalf("expected unexpected-argument error, got %v", err)
	}
}

// Every list command must reject positionals, not just the ones spot-checked.
func TestAllListCommandsRejectArgs(t *testing.T) {
	for _, cmd := range listCommandPaths(t) {
		args := append(strings.Split(cmd, " "), "junk")
		if _, err := run(t, args...); err == nil {
			t.Errorf("%s accepted a stray positional", cmd)
		}
	}
}

func listCommandPaths(t *testing.T) []string {
	t.Helper()
	var paths []string
	var walk func(c *cobraCmd, prefix string)
	root := newRootCmd()
	walk = func(c *cobraCmd, prefix string) {
		for _, sub := range c.Commands() {
			name := strings.Fields(sub.Use)[0]
			full := strings.TrimSpace(prefix + " " + name)
			if name == "list" {
				paths = append(paths, full)
			}
			walk(sub, full)
		}
	}
	walk(root, "")
	if len(paths) == 0 {
		t.Fatal("found no list commands")
	}
	return paths
}

func TestCreateRejectsDoubleName(t *testing.T) {
	_, err := run(t, "goals", "create", "One", "--name", "Two")
	if err == nil || !strings.Contains(err.Error(), "cannot set the name twice") {
		t.Fatalf("expected double-name error, got %v", err)
	}
}
