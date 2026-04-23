package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"

	openapi_types "github.com/oapi-codegen/runtime/types"
	"github.com/spf13/cobra"
)

// readJSONBody loads a JSON body from a path (or "-" for stdin) and unmarshals it.
// If path is empty and stdin is a TTY, returns an error prompting the user to
// supply --file or pipe in JSON.
func readJSONBody[T any](cmd *cobra.Command, path string) (T, error) {
	var zero T
	var r io.Reader
	switch {
	case path == "-":
		r = cmd.InOrStdin()
	case path != "":
		f, err := os.Open(path)
		if err != nil {
			return zero, fmt.Errorf("open %s: %w", path, err)
		}
		defer f.Close()
		r = f
	default:
		return zero, fmt.Errorf("no body provided; use --file <path> or --file - to read from stdin")
	}
	dec := json.NewDecoder(r)
	dec.DisallowUnknownFields()
	var v T
	if err := dec.Decode(&v); err != nil {
		return zero, fmt.Errorf("decode JSON body: %w", err)
	}
	return v, nil
}

// ptrStr returns the string at p, or "" if p is nil.
func ptrStr(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

// ptrBool formats *bool as "true"/"false"/"" (for nil).
func ptrBool(p *bool) string {
	if p == nil {
		return ""
	}
	if *p {
		return "true"
	}
	return "false"
}

// ptrInt formats *int, returning "" for nil.
func ptrInt(p *int) string {
	if p == nil {
		return ""
	}
	return fmt.Sprintf("%d", *p)
}

// intPtr is the inverse — useful for setting optional request params.
func intPtr(v int) *int { return &v }

// strFlag returns a pointer to v when the named flag was set, else nil. Used to
// populate optional list-filter params only when the user asked for them.
func strFlag(cmd *cobra.Command, name, v string) *string {
	if !cmd.Flags().Changed(name) {
		return nil
	}
	return &v
}

// boolFlag mirrors strFlag for *bool filters.
func boolFlag(cmd *cobra.Command, name string, v bool) *bool {
	if !cmd.Flags().Changed(name) {
		return nil
	}
	return &v
}

// dateFlag parses v as YYYY-MM-DD when the named flag was set. Returns nil when
// the flag was not provided.
func dateFlag(cmd *cobra.Command, name, v string) (*openapi_types.Date, error) {
	if !cmd.Flags().Changed(name) {
		return nil, nil
	}
	t, err := time.Parse("2006-01-02", v)
	if err != nil {
		return nil, fmt.Errorf("--%s: %w (expected YYYY-MM-DD)", name, err)
	}
	return &openapi_types.Date{Time: t}, nil
}

// timeFlag parses v as RFC3339 when the named flag was set.
func timeFlag(cmd *cobra.Command, name, v string) (*time.Time, error) {
	if !cmd.Flags().Changed(name) {
		return nil, nil
	}
	t, err := time.Parse(time.RFC3339, v)
	if err != nil {
		return nil, fmt.Errorf("--%s: %w (expected RFC3339, e.g. 2006-01-02T15:04:05Z)", name, err)
	}
	return &t, nil
}
