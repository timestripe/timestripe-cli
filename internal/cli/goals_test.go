package cli

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// goalsQueryStub records the query string of each /goals/ list request.
func goalsQueryStub(t *testing.T, seen *[]string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*seen = append(*seen, r.URL.RawQuery)
		writeJSON(w, 200, map[string]any{"count": 0, "next": nil, "previous": nil, "results": []any{}})
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestCheckedFilterFlags(t *testing.T) {
	tests := []struct {
		name   string
		args   []string
		want   string
		absent string
	}{
		{name: "--open sends checked=false", args: []string{"--open"}, want: "checked=false"},
		{name: "--done sends checked=true", args: []string{"--done"}, want: "checked=true"},
		{name: "--checked=false still works", args: []string{"--checked=false"}, want: "checked=false"},
		{name: "--checked still works", args: []string{"--checked"}, want: "checked=true"},
		{name: "unset sends no filter", args: nil, absent: "checked"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var seen []string
			srv := goalsQueryStub(t, &seen)
			args := append([]string{"goals", "list", "--json"}, tc.args...)
			if _, err := runAgainst(t, srv.URL, args...); err != nil {
				t.Fatal(err)
			}
			if len(seen) == 0 {
				t.Fatal("no request recorded")
			}
			if tc.want != "" && !strings.Contains(seen[0], tc.want) {
				t.Errorf("query %q missing %q", seen[0], tc.want)
			}
			if tc.absent != "" && strings.Contains(seen[0], tc.absent) {
				t.Errorf("query %q should not mention %q", seen[0], tc.absent)
			}
		})
	}
}

func TestCheckedFlagsAreMutuallyExclusive(t *testing.T) {
	for _, pair := range [][]string{
		{"--open", "--done"},
		{"--open", "--checked"},
		{"--done", "--checked"},
	} {
		var seen []string
		srv := goalsQueryStub(t, &seen)
		args := append([]string{"goals", "list"}, pair...)
		if _, err := runAgainst(t, srv.URL, args...); err == nil {
			t.Errorf("%v were accepted together", pair)
		}
	}
}
