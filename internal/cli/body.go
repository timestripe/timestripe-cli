package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

// loadBodyFromFile returns a JSON object read from --file (path or "-" for
// stdin). Returns an empty map when file is unset, so callers can still layer
// flag-driven fields on top.
func loadBodyFromFile(cmd *cobra.Command, file string) (map[string]any, error) {
	if file == "" {
		return map[string]any{}, nil
	}
	var r io.Reader
	if file == "-" {
		r = cmd.InOrStdin()
	} else {
		f, err := os.Open(file)
		if err != nil {
			return nil, fmt.Errorf("open %s: %w", file, err)
		}
		defer f.Close()
		r = f
	}
	var body map[string]any
	dec := json.NewDecoder(r)
	if err := dec.Decode(&body); err != nil {
		return nil, fmt.Errorf("decode JSON body from %s: %w", file, err)
	}
	if body == nil {
		body = map[string]any{}
	}
	return body, nil
}

// ifChanged assigns body[key] = value when the named flag was set on cmd.
func ifChanged(cmd *cobra.Command, flag, key string, value any, body map[string]any) {
	if cmd.Flags().Changed(flag) {
		body[key] = value
	}
}

// encodeJSONBody is a convenience that serializes body to an io.Reader and
// returns the canonical JSON content-type for the generated client's WithBody
// variants.
func encodeJSONBody(body map[string]any) (string, io.Reader, error) {
	b, err := json.Marshal(body)
	if err != nil {
		return "", nil, fmt.Errorf("encode body: %w", err)
	}
	return "application/json", bytes.NewReader(b), nil
}

// encodeMultipartFile builds a multipart/form-data body with a single file
// field. path may be "-" to read from stdin (filename defaults to "stdin").
func encodeMultipartFile(cmd *cobra.Command, field, path string) (string, io.Reader, error) {
	var src io.Reader
	name := filepath.Base(path)
	if path == "-" {
		src = cmd.InOrStdin()
		name = "stdin"
	} else {
		f, err := os.Open(path)
		if err != nil {
			return "", nil, fmt.Errorf("open %s: %w", path, err)
		}
		defer f.Close()
		src = f
	}
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	fw, err := w.CreateFormFile(field, name)
	if err != nil {
		return "", nil, err
	}
	if _, err := io.Copy(fw, src); err != nil {
		return "", nil, err
	}
	if err := w.Close(); err != nil {
		return "", nil, err
	}
	return w.FormDataContentType(), &buf, nil
}
