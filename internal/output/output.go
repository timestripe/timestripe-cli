// Package output renders command results in user-selected formats.
//
// Supported formats: json, yaml, markdown, table, csv. The JSON and YAML
// renderers operate directly on the provided value (via encoding/json and
// gopkg.in/yaml.v3). The markdown/table/csv renderers operate on the tabular
// representation supplied by the caller, so each command is responsible for
// deciding which fields are columns.
package output

import (
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/jedib0t/go-pretty/v6/table"
	"golang.org/x/term"
	yaml "gopkg.in/yaml.v3"
)

// Format identifies the output format.
type Format string

const (
	FormatJSON     Format = "json"
	FormatYAML     Format = "yaml"
	FormatMarkdown Format = "markdown"
	FormatTable    Format = "table"
	FormatCSV      Format = "csv"
)

// Flags captures the mutually-exclusive format flags from Cobra. Exactly zero
// or one field may be true.
type Flags struct {
	JSON     bool
	YAML     bool
	Markdown bool
	Table    bool
	CSV      bool
}

// Resolve picks a Format from the flags. If no flag is set, defaults to
// FormatTable on a TTY and FormatJSON otherwise.
func Resolve(w io.Writer, f Flags) (Format, error) {
	set := []Format{}
	if f.JSON {
		set = append(set, FormatJSON)
	}
	if f.YAML {
		set = append(set, FormatYAML)
	}
	if f.Markdown {
		set = append(set, FormatMarkdown)
	}
	if f.Table {
		set = append(set, FormatTable)
	}
	if f.CSV {
		set = append(set, FormatCSV)
	}
	if len(set) > 1 {
		return "", fmt.Errorf("--json, --yaml, --markdown, --table, --csv are mutually exclusive")
	}
	if len(set) == 1 {
		return set[0], nil
	}
	if isTerminal(w) {
		return FormatTable, nil
	}
	return FormatJSON, nil
}

func isTerminal(w io.Writer) bool {
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	return term.IsTerminal(int(f.Fd()))
}

// Tabular is a row-oriented representation of data for markdown/table/csv
// formats. Values are stringified by the caller.
type Tabular struct {
	Headers []string
	Rows    [][]string
}

// Render writes v in the given format. For markdown/table/csv, t must be
// supplied (JSON/YAML ignore it).
func Render(w io.Writer, format Format, v any, t *Tabular) error {
	switch format {
	case FormatJSON:
		return renderJSON(w, v)
	case FormatYAML:
		return renderYAML(w, v)
	case FormatMarkdown:
		if t == nil {
			return errRequiresTabular(format)
		}
		return renderMarkdown(w, t)
	case FormatTable:
		if t == nil {
			return errRequiresTabular(format)
		}
		return renderTable(w, t)
	case FormatCSV:
		if t == nil {
			return errRequiresTabular(format)
		}
		return renderCSV(w, t)
	default:
		return fmt.Errorf("unknown output format %q", format)
	}
}

func errRequiresTabular(f Format) error {
	return fmt.Errorf("--%s requires a command that produces tabular output", f)
}

func renderJSON(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	return enc.Encode(v)
}

func renderYAML(w io.Writer, v any) error {
	enc := yaml.NewEncoder(w)
	enc.SetIndent(2)
	defer enc.Close()
	return enc.Encode(v)
}

func renderMarkdown(w io.Writer, t *Tabular) error {
	if len(t.Headers) == 0 {
		return errors.New("markdown renderer: no headers")
	}
	var b strings.Builder
	b.WriteString("| ")
	b.WriteString(strings.Join(t.Headers, " | "))
	b.WriteString(" |\n|")
	for range t.Headers {
		b.WriteString(" --- |")
	}
	b.WriteString("\n")
	for _, row := range t.Rows {
		b.WriteString("| ")
		b.WriteString(strings.Join(escapeMarkdownRow(row), " | "))
		b.WriteString(" |\n")
	}
	_, err := io.WriteString(w, b.String())
	return err
}

func escapeMarkdownRow(row []string) []string {
	out := make([]string, len(row))
	for i, c := range row {
		c = strings.ReplaceAll(c, "|", `\|`)
		c = strings.ReplaceAll(c, "\n", " ")
		out[i] = c
	}
	return out
}

func renderTable(w io.Writer, t *Tabular) error {
	tw := table.NewWriter()
	tw.SetOutputMirror(w)
	tw.Style().Format.Header = 0 // keep casing as provided
	hdr := make(table.Row, len(t.Headers))
	for i, h := range t.Headers {
		hdr[i] = h
	}
	tw.AppendHeader(hdr)
	for _, row := range t.Rows {
		r := make(table.Row, len(row))
		for i, c := range row {
			r[i] = c
		}
		tw.AppendRow(r)
	}
	tw.Render()
	return nil
}

func renderCSV(w io.Writer, t *Tabular) error {
	cw := csv.NewWriter(w)
	if err := cw.Write(t.Headers); err != nil {
		return err
	}
	for _, row := range t.Rows {
		if err := cw.Write(row); err != nil {
			return err
		}
	}
	cw.Flush()
	return cw.Error()
}
