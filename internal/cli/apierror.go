package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// maxRawBody bounds an unparseable error body so a Django debug page cannot
// flood the terminal.
const maxRawBody = 500

// APIError is a non-2xx API response rendered for humans.
//
// The API is Django REST Framework, which returns a small number of predictable
// error shapes. Parsing them into fields beats dumping raw JSON at the user,
// which is what this replaced.
type APIError struct {
	Status int
	Detail string
	Fields []FieldError
	Raw    []byte
}

// FieldError is one field's validation messages.
type FieldError struct {
	Field    string
	Messages []string
}

// apiError pulls a readable message out of a non-2xx API response body.
//
// The signature is unchanged from the original so every call site keeps
// compiling; the returned error is now an *APIError.
func apiError(status int, body []byte) error {
	e := &APIError{Status: status, Raw: body}
	e.parse(body)
	return e
}

func (e *APIError) parse(body []byte) {
	trimmed := strings.TrimSpace(string(body))
	if trimmed == "" {
		return
	}
	// An HTML body means we did not reach the API at all.
	if strings.HasPrefix(trimmed, "<") {
		e.Detail = "(HTML response — likely a proxy, or a misconfigured TIMESTRIPE_BACKEND)"
		return
	}

	// {"detail": "..."} or a per-field map.
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(body, &obj); err == nil {
		for _, key := range []string{"detail", "error", "message"} {
			if raw, ok := obj[key]; ok {
				if s, ok := decodeMessage(raw); ok {
					e.Detail = s
					delete(obj, key)
				}
			}
		}
		for key, raw := range obj {
			msgs := decodeMessages(raw, key)
			if key == "non_field_errors" {
				if e.Detail == "" {
					e.Detail = strings.Join(msgs, " ")
				}
				continue
			}
			e.Fields = append(e.Fields, FieldError{Field: key, Messages: msgs})
		}
		// Deterministic output regardless of Go's map iteration order.
		sort.Slice(e.Fields, func(i, j int) bool { return e.Fields[i].Field < e.Fields[j].Field })
		if e.Detail != "" || len(e.Fields) > 0 {
			return
		}
	}

	// A bare array of strings.
	var arr []string
	if err := json.Unmarshal(body, &arr); err == nil && len(arr) > 0 {
		e.Detail = strings.Join(arr, " ")
		return
	}
}

// decodeMessage pulls a single string out of a JSON value.
func decodeMessage(raw json.RawMessage) (string, bool) {
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s, true
	}
	return "", false
}

// decodeMessages flattens a field's value into messages. DRF nests one level
// for list serializers: {"magic_links": [{"role": ["..."]}]}.
func decodeMessages(raw json.RawMessage, field string) []string {
	if s, ok := decodeMessage(raw); ok {
		return []string{s}
	}
	var list []string
	if err := json.Unmarshal(raw, &list); err == nil {
		return list
	}
	var nested []map[string][]string
	if err := json.Unmarshal(raw, &nested); err == nil {
		var out []string
		for i, item := range nested {
			keys := make([]string, 0, len(item))
			for k := range item {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			for _, k := range keys {
				for _, m := range item[k] {
					out = append(out, fmt.Sprintf("%s.%d.%s: %s", field, i, k, m))
				}
			}
		}
		return out
	}
	return []string{strings.TrimSpace(string(raw))}
}

func (e *APIError) Error() string {
	var b strings.Builder

	switch e.Status {
	case 401:
		b.WriteString("not authenticated")
	case 403:
		b.WriteString("forbidden")
	case 404:
		b.WriteString("not found")
	case 429:
		b.WriteString("rate limited")
	default:
		if len(e.Fields) > 0 {
			b.WriteString("request rejected")
		} else {
			b.WriteString(fmt.Sprintf("api returned status %d", e.Status))
		}
	}
	if e.Status >= 400 && e.Status != 429 && (len(e.Fields) > 0 || e.Detail != "") {
		b.WriteString(fmt.Sprintf(" (%d)", e.Status))
	}

	if e.Detail != "" {
		b.WriteString(": " + e.Detail)
	} else if len(e.Fields) == 0 {
		if raw := truncateBody(strings.TrimSpace(string(e.Raw))); raw != "" {
			b.WriteString(": " + raw)
		}
	}

	if len(e.Fields) > 0 {
		width := 0
		for _, f := range e.Fields {
			if len(f.Field) > width {
				width = len(f.Field)
			}
		}
		for _, f := range e.Fields {
			for _, m := range f.Messages {
				b.WriteString(fmt.Sprintf("\n  %-*s  %s", width+1, f.Field+":", m))
			}
		}
	}

	if hint := e.hint(); hint != "" {
		b.WriteString("\n" + hint)
	}
	return b.String()
}

// hint appends the action most likely to resolve this error.
func (e *APIError) hint() string {
	switch e.Status {
	case 401:
		return "Run `timestripe auth login` to sign in."
	case 403:
		return "If this is a read-only session, re-run `timestripe auth login` (grants read_write)."
	}
	// A bad foreign key is usually someone passing a name where the flag wants
	// an ID — except the ref flags accept both, so point at them.
	for _, f := range e.Fields {
		if !strings.HasSuffix(f.Field, "_id") {
			continue
		}
		for _, m := range f.Messages {
			if strings.Contains(strings.ToLower(m), "does not exist") || strings.Contains(strings.ToLower(m), "invalid pk") {
				return "--space, --bucket, --assignee and --parent accept a name as well as an ID."
			}
		}
	}
	return ""
}

func truncateBody(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	if len(s) <= maxRawBody {
		return s
	}
	return s[:maxRawBody] + "… (truncated; use --verbose for the full body)"
}
