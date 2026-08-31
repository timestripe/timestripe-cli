package cli

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

// specEnums loads api/openapi.yaml and returns every enum, keyed by
// "<path> <param>" for query params and "<Schema>.<property>" for body fields.
//
// This is the guard that makes hand-written tables acceptable: `make gen`
// changing an enum now fails `make test` instead of silently diverging.
func specEnums(t *testing.T) map[string][]string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "..", "api", "openapi.yaml"))
	if err != nil {
		t.Fatalf("read spec: %v", err)
	}
	var doc struct {
		Paths map[string]map[string]struct {
			Parameters []struct {
				Name   string `yaml:"name"`
				Schema struct {
					Enum  []any `yaml:"enum"`
					Items struct {
						Enum []any `yaml:"enum"`
					} `yaml:"items"`
				} `yaml:"schema"`
			} `yaml:"parameters"`
		} `yaml:"paths"`
		Components struct {
			Schemas map[string]struct {
				Properties map[string]struct {
					Enum []any `yaml:"enum"`
				} `yaml:"properties"`
			} `yaml:"schemas"`
		} `yaml:"components"`
	}
	if err := yaml.Unmarshal(b, &doc); err != nil {
		t.Fatalf("parse spec: %v", err)
	}

	out := map[string][]string{}
	add := func(key string, raw []any) {
		var vals []string
		for _, v := range raw {
			s, ok := v.(string)
			// A nil member is the nullable marker; the CLI spells that "none".
			if !ok || s == "" {
				continue
			}
			vals = append(vals, s)
		}
		if len(vals) > 0 {
			out[key] = vals
		}
	}
	for path, ops := range doc.Paths {
		for _, op := range ops {
			for _, p := range op.Parameters {
				if len(p.Schema.Enum) > 0 {
					add(path+" "+p.Name, p.Schema.Enum)
				} else if len(p.Schema.Items.Enum) > 0 {
					add(path+" "+p.Name, p.Schema.Items.Enum)
				}
			}
		}
	}
	for name, sc := range doc.Components.Schemas {
		for prop, pm := range sc.Properties {
			if len(pm.Enum) > 0 {
				add(name+"."+prop, pm.Enum)
			}
		}
	}
	return out
}

func TestEnumTablesMatchSpec(t *testing.T) {
	spec := specEnums(t)
	cases := []struct {
		key   string
		table []string
	}{
		{"/goals/ horizon", enumHorizon},
		{"/goals/ color", enumColor},
		{"Goal.horizon", enumHorizon},
		{"Goal.color", enumColor},
		{"Board.layout", enumLayout},
		{"/goals/ sort", enumSortGoals},
		{"/boards/ sort", enumSortBoards},
		{"/buckets/ sort", enumSortBuckets},
		{"/comments/ sort", enumSortComments},
		{"/events/ sort", enumSortEvents},
		{"/folders/ sort", enumSortFolders},
		{"/folder-goals/ sort", enumSortFolderGoals},
		{"/events/ type", enumEventType},
	}
	for _, tc := range cases {
		t.Run(tc.key, func(t *testing.T) {
			want, ok := spec[tc.key]
			if !ok {
				t.Fatalf("%q is no longer in the spec; the table is stale", tc.key)
			}
			gotSorted, wantSorted := sortedCopy(tc.table), sortedCopy(want)
			if !reflect.DeepEqual(gotSorted, wantSorted) {
				t.Errorf("table drifted from the spec\n have: %v\n want: %v", gotSorted, wantSorted)
			}
		})
	}
}

// Anything the spec calls an enum should be covered by a table, so a new one
// cannot appear unvalidated.
func TestNoUncoveredSpecEnums(t *testing.T) {
	covered := map[string]bool{
		"/goals/ horizon": true, "/goals/ color": true, "/goals/ sort": true,
		"/boards/ sort": true, "/buckets/ sort": true, "/comments/ sort": true,
		"/events/ sort": true, "/events/ type": true, "/folders/ sort": true,
		"/folder-goals/ sort": true,
		"Goal.horizon":        true, "Goal.color": true, "Board.layout": true,
		"PatchedGoal.horizon": true, "PatchedGoal.color": true, "PatchedBoard.layout": true,
		"Event.type": true,
		// Roles are not exposed as a flag: memberships is read-only and
		// magic links are not implemented.
		"Membership.role": true, "MagicLink.role": true,
	}
	for key := range specEnums(t) {
		if !covered[key] {
			t.Errorf("spec enum %q has no table in enums.go and no exemption", key)
		}
	}
}

func TestValidateEnumSuggests(t *testing.T) {
	tests := []struct {
		in       string
		contains []string
	}{
		{"weekly", []string{`"weekly" is not valid`, `did you mean "week"`, "day, week, month"}},
		{"dai", []string{`did you mean "day"`}},
		{"zzzzzzzz", []string{"is not valid", "valid: day, week"}},
	}
	for _, tc := range tests {
		t.Run(tc.in, func(t *testing.T) {
			var seen []string
			srv := goalsQueryStub(t, &seen)
			_, err := runAgainst(t, srv.URL, "goals", "list", "--horizon", tc.in)
			if err == nil {
				t.Fatal("expected a validation error")
			}
			for _, want := range tc.contains {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("error missing %q:\n%s", want, err)
				}
			}
			if len(seen) > 0 {
				t.Errorf("a rejected enum still hit the API: %v", seen)
			}
		})
	}
}

// Validation must run before authentication, so a typo does not first demand
// credentials the user may not have.
func TestEnumValidationPrecedesAuth(t *testing.T) {
	t.Setenv("TIMESTRIPE_TOKEN", "")
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	root := newRootCmd()
	root.SetArgs([]string{"goals", "list", "--horizon", "weekly"})
	root.SetOut(new(strings.Builder))
	root.SetErr(new(strings.Builder))
	err := root.Execute()
	if err == nil {
		t.Fatal("expected an error")
	}
	if strings.Contains(err.Error(), "auth login") {
		t.Errorf("auth ran before enum validation:\n%s", err)
	}
}

func TestColorNormalisation(t *testing.T) {
	if got := normalizeColor("ecce32"); got != "#ecce32" {
		t.Errorf("normalizeColor(ecce32) = %q", got)
	}
	if got := normalizeColor("#ecce32"); got != "#ecce32" {
		t.Errorf("normalizeColor(#ecce32) = %q", got)
	}
}

// Every enum flag must have completion registered. These registrations are
// easy to drop silently — RegisterFlagCompletionFunc's error is discarded, and
// a missing one is invisible until someone presses TAB.
func TestEnumFlagsHaveCompletion(t *testing.T) {
	cases := []struct{ path, flag string }{
		{"timestripe goals list", "horizon"},
		{"timestripe goals list", "color"},
		{"timestripe goals list", "sort"},
		{"timestripe goals create", "horizon"},
		{"timestripe goals create", "color"},
		{"timestripe goals update", "horizon"},
		{"timestripe boards list", "sort"},
		{"timestripe boards create", "layout"},
		{"timestripe buckets list", "sort"},
		{"timestripe comments list", "sort"},
		{"timestripe events list", "sort"},
		{"timestripe events list", "type"},
		{"timestripe folders list", "sort"},
		{"timestripe folders goals list", "sort"},
	}
	for _, tc := range cases {
		t.Run(tc.path+" --"+tc.flag, func(t *testing.T) {
			cmd := findCommand(t, tc.path)
			if cmd.Flags().Lookup(tc.flag) == nil {
				t.Fatalf("%s has no --%s", tc.path, tc.flag)
			}
			// Registering succeeds only if nothing is registered yet, so a nil
			// error means the production registration is missing.
			err := cmd.RegisterFlagCompletionFunc(tc.flag,
				cobra.FixedCompletions(nil, cobra.ShellCompDirectiveNoFileComp))
			if err == nil {
				t.Errorf("%s --%s has no completion registered", tc.path, tc.flag)
			}
		})
	}
}

func findCommand(t *testing.T, path string) *cobra.Command {
	t.Helper()
	var found *cobra.Command
	walkCommands(t, func(p string, c *cobra.Command) {
		if p == path {
			found = c
		}
	})
	if found == nil {
		t.Fatalf("command %q not found", path)
	}
	return found
}
