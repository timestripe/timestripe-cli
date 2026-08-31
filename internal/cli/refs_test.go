package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

// apiStub serves a minimal spaces endpoint and records the requests made, so
// tests can assert on round-trip counts as well as behaviour.
type apiStub struct {
	*httptest.Server
	requests atomic.Int32
	paths    []string
}

type stubSpace struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// newSpacesStub serves GET /spaces/<id>/ and GET /spaces/?search=...
func newSpacesStub(t *testing.T, spaces []stubSpace) *apiStub {
	t.Helper()
	s := &apiStub{}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v3/spaces/", func(w http.ResponseWriter, r *http.Request) {
		s.requests.Add(1)
		s.paths = append(s.paths, r.URL.String())
		id := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/v3/spaces/"), "/")

		if id != "" { // retrieve by ID
			for _, sp := range spaces {
				if sp.ID == id {
					writeJSON(w, 200, sp)
					return
				}
			}
			writeJSON(w, 404, map[string]string{"detail": "Not found."})
			return
		}

		results := spaces
		if q := r.URL.Query().Get("search"); q != "" {
			results = nil
			for _, sp := range spaces {
				if strings.Contains(strings.ToLower(sp.Name), strings.ToLower(q)) {
					results = append(results, sp)
				}
			}
		}
		writeJSON(w, 200, map[string]any{
			"count": len(results), "next": nil, "previous": nil, "results": results,
		})
	})
	s.Server = httptest.NewServer(mux)
	t.Cleanup(s.Close)
	return s
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// runAgainst points the CLI at srv with a bearer token and runs args.
func runAgainst(t *testing.T, srvURL string, args ...string) (string, error) {
	t.Helper()
	t.Setenv("TIMESTRIPE_BACKEND", srvURL)
	t.Setenv("TIMESTRIPE_TOKEN", "test-token")
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	root := newRootCmd()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	// A bytes.Reader is not an *os.File, so interactive() is false: these
	// exercise the non-TTY path that scripts and agents get.
	root.SetIn(bytes.NewReader(nil))
	root.SetArgs(args)
	err := root.Execute()
	return out.String(), err
}

// A valid ID must cost exactly one lookup: the GET probe, with no name scan.
func TestResolveByIDTakesOneRequest(t *testing.T) {
	stub := newSpacesStub(t, []stubSpace{{ID: "spc_abc", Name: "Work"}})
	// The folder create itself 404s (unstubbed), which is fine — resolution
	// happens first and is all this asserts on.
	_, _ = runAgainst(t, stub.URL, "folders", "create", "F", "--space", "spc_abc")

	if n := stub.requests.Load(); n != 1 {
		t.Errorf("resolving a valid ID made %d space requests, want 1: %v", n, stub.paths)
	}
	for _, p := range stub.paths {
		if strings.Contains(p, "search=") {
			t.Errorf("a valid ID should not trigger a name scan: %s", p)
		}
	}
}

func TestResolveByNameUsesServerSearch(t *testing.T) {
	stub := newSpacesStub(t, []stubSpace{
		{ID: "spc_abc", Name: "Work"},
		{ID: "spc_def", Name: "Personal"},
	})
	_, _ = runAgainst(t, stub.URL, "folders", "create", "F", "--space", "Work")

	var searched bool
	for _, p := range stub.paths {
		if strings.Contains(p, "search=Work") {
			searched = true
		}
	}
	if !searched {
		t.Errorf("name lookup did not use the server-side search filter; paths: %v", stub.paths)
	}
}

func TestResolveAmbiguousNameListsCandidates(t *testing.T) {
	stub := newSpacesStub(t, []stubSpace{
		{ID: "spc_1", Name: "Inbox"},
		{ID: "spc_2", Name: "Inbox"},
	})
	_, err := runAgainst(t, stub.URL, "folders", "create", "F", "--space", "Inbox")
	if err == nil {
		t.Fatal("expected an ambiguity error")
	}
	for _, want := range []string{"matches 2 spaces", "spc_1", "spc_2"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error missing %q:\n%s", want, err)
		}
	}
}

func TestResolveTypoSuggests(t *testing.T) {
	stub := newSpacesStub(t, []stubSpace{{ID: "spc_abc", Name: "Work"}})
	_, err := runAgainst(t, stub.URL, "folders", "create", "F", "--space", "Wor")
	if err == nil {
		t.Fatal("expected a no-match error")
	}
	if !strings.Contains(err.Error(), "did you mean") || !strings.Contains(err.Error(), "Work") {
		t.Errorf("expected a suggestion, got:\n%s", err)
	}
}

// Regression: a 401 on the get-by-ID probe used to be swallowed, degrading into
// a name scan that reported "no space matches" — hiding an expired token.
func TestResolveSurfacesAuthErrors(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 401, map[string]string{"detail": "Authentication credentials were not provided."})
	}))
	defer srv.Close()

	_, err := runAgainst(t, srv.URL, "folders", "create", "F", "--space", "spc_abc")
	if err == nil {
		t.Fatal("expected an error")
	}
	if !strings.Contains(err.Error(), "not authenticated") {
		t.Errorf("401 was masked as a name-match failure:\n%s", err)
	}
	if strings.Contains(err.Error(), "no space matches") {
		t.Errorf("auth failure still degrades into a name scan:\n%s", err)
	}
}

// A value with a space cannot be an ID, so we must not spend a request probing.
func TestResolveSkipsIDProbeForMultiWordNames(t *testing.T) {
	stub := newSpacesStub(t, []stubSpace{{ID: "spc_abc", Name: "My Space"}})
	_, _ = runAgainst(t, stub.URL, "folders", "create", "F", "--space", "My Space")
	for _, p := range stub.paths {
		if strings.Contains(p, "My%20Space") && !strings.Contains(p, "search=") {
			t.Errorf("probed a multi-word value as an ID: %s", p)
		}
	}
}

func TestResolveNoCandidatesAtAll(t *testing.T) {
	stub := newSpacesStub(t, nil)
	_, err := runAgainst(t, stub.URL, "folders", "create", "F", "--space", "Nope")
	if err == nil || !strings.Contains(err.Error(), fmt.Sprintf("no space matches %q", "Nope")) {
		t.Fatalf("unexpected error: %v", err)
	}
}
