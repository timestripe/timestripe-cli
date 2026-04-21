package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

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
