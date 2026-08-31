package cli

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/spf13/pflag"
)

type pflagFlag = pflag.Flag

// goalStub serves one goal, supports search, and records PATCH bodies.
func goalStub(t *testing.T, patched *[]map[string]any) *httptest.Server {
	t.Helper()
	goal := map[string]any{"id": "gl_1", "name": "Buy milk", "space_id": "spc_1", "checked": false}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v3/goals/", func(w http.ResponseWriter, r *http.Request) {
		id := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/v3/goals/"), "/")
		switch {
		case r.Method == http.MethodPatch:
			b, _ := io.ReadAll(r.Body)
			var body map[string]any
			_ = json.Unmarshal(b, &body)
			*patched = append(*patched, body)
			out := map[string]any{}
			for k, v := range goal {
				out[k] = v
			}
			for k, v := range body {
				out[k] = v
			}
			writeJSON(w, 200, out)
		case id != "":
			if id == "gl_1" {
				writeJSON(w, 200, goal)
				return
			}
			writeJSON(w, 404, map[string]string{"detail": "Not found."})
		default:
			results := []any{goal}
			if q := r.URL.Query().Get("search"); q != "" && !strings.Contains("buy milk", strings.ToLower(q)) {
				results = nil
			}
			writeJSON(w, 200, map[string]any{"count": len(results), "next": nil, "previous": nil, "results": results})
		}
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func TestDoneAndReopen(t *testing.T) {
	for _, tc := range []struct {
		cmd  string
		want bool
	}{{"done", true}, {"reopen", false}} {
		t.Run(tc.cmd, func(t *testing.T) {
			var patched []map[string]any
			srv := goalStub(t, &patched)
			if _, err := runAgainst(t, srv.URL, tc.cmd, "gl_1", "--json"); err != nil {
				t.Fatal(err)
			}
			if len(patched) != 1 {
				t.Fatalf("expected 1 PATCH, got %d", len(patched))
			}
			if got, ok := patched[0]["checked"].(bool); !ok || got != tc.want {
				t.Errorf("PATCH body = %v, want checked=%v", patched[0], tc.want)
			}
		})
	}
}

// The payoff for name resolution: no ID lookup step first.
func TestDoneAcceptsAName(t *testing.T) {
	var patched []map[string]any
	srv := goalStub(t, &patched)
	if _, err := runAgainst(t, srv.URL, "done", "Buy milk", "--json"); err != nil {
		t.Fatal(err)
	}
	if len(patched) != 1 {
		t.Fatalf("expected 1 PATCH, got %d", len(patched))
	}
}

func TestQuickVerbsAreAliasesNotForks(t *testing.T) {
	root := newRootCmd()
	var add, create *cobraCmd
	for _, c := range root.Commands() {
		if strings.Fields(c.Use)[0] == "add" {
			add = c
		}
		if strings.Fields(c.Use)[0] == "goals" {
			for _, sub := range c.Commands() {
				if strings.Fields(sub.Use)[0] == "create" {
					create = sub
				}
			}
		}
	}
	if add == nil || create == nil {
		t.Fatal("add or goals create is missing")
	}
	// Same flag surface: add is goals create under another name, so the two
	// cannot drift as fields are added.
	addFlags, createFlags := flagNames(add), flagNames(create)
	if len(addFlags) != len(createFlags) {
		t.Fatalf("add has %d flags, goals create has %d", len(addFlags), len(createFlags))
	}
	for name := range createFlags {
		if _, ok := addFlags[name]; !ok {
			t.Errorf("add is missing --%s", name)
		}
	}
}

func flagNames(c *cobraCmd) map[string]string {
	out := map[string]string{}
	c.Flags().VisitAll(func(f *pflagFlag) { out[f.Name] = f.Shorthand })
	return out
}

func TestGoalsAlsoExposesDoneAndReopen(t *testing.T) {
	root := newRootCmd()
	for _, c := range root.Commands() {
		if strings.Fields(c.Use)[0] != "goals" {
			continue
		}
		found := map[string]bool{}
		for _, sub := range c.Commands() {
			found[strings.Fields(sub.Use)[0]] = true
		}
		for _, want := range []string{"done", "reopen"} {
			if !found[want] {
				t.Errorf("goals is missing the %q subcommand", want)
			}
		}
		return
	}
	t.Fatal("goals command not found")
}
