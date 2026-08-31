package cli

import (
	"strings"
	"testing"
)

func TestAPIErrorRendering(t *testing.T) {
	tests := []struct {
		name     string
		status   int
		body     string
		contains []string
		absent   []string
	}{
		{
			name:     "detail",
			status:   404,
			body:     `{"detail":"Not found."}`,
			contains: []string{"not found (404)", "Not found."},
		},
		{
			name:     "unauthenticated gets a login hint",
			status:   401,
			body:     `{"detail":"Authentication credentials were not provided."}`,
			contains: []string{"not authenticated", "Run `timestripe auth login`"},
		},
		{
			name:     "forbidden mentions scope",
			status:   403,
			body:     `{"detail":"You do not have permission to perform this action."}`,
			contains: []string{"forbidden", "read_write"},
		},
		{
			name:     "field errors are listed and sorted",
			status:   400,
			body:     `{"space_id":["This field is required."],"name":["May not be blank."]}`,
			contains: []string{"request rejected (400)", "name:", "May not be blank.", "space_id:"},
		},
		{
			name:     "bad foreign key points at ref flags",
			status:   400,
			body:     `{"bucket_id":["Invalid pk \"nope\" - object does not exist."]}`,
			contains: []string{"--bucket", "accept a name as well as an ID"},
		},
		{
			name:     "non_field_errors become the detail",
			status:   400,
			body:     `{"non_field_errors":["Start must precede end."]}`,
			contains: []string{"Start must precede end."},
		},
		{
			name:     "bare array",
			status:   400,
			body:     `["Something broke."]`,
			contains: []string{"Something broke."},
		},
		{
			name:     "nested list serializer is flattened",
			status:   400,
			body:     `{"magic_links":[{"role":["Invalid role."]}]}`,
			contains: []string{"magic_links.0.role: Invalid role."},
		},
		{
			name:     "html body is diagnosed, not dumped",
			status:   502,
			body:     "<!doctype html><html><body>" + strings.Repeat("x", 5000) + "</body></html>",
			contains: []string{"HTML response", "TIMESTRIPE_BACKEND"},
			absent:   []string{strings.Repeat("x", 100)},
		},
		{
			name:     "unparseable body is truncated",
			status:   500,
			body:     strings.Repeat("garbage ", 500),
			contains: []string{"api returned status 500", "truncated"},
		},
		{
			name:     "empty body",
			status:   500,
			body:     "",
			contains: []string{"api returned status 500"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := apiError(tc.status, []byte(tc.body)).Error()
			for _, want := range tc.contains {
				if !strings.Contains(got, want) {
					t.Errorf("missing %q in:\n%s", want, got)
				}
			}
			for _, bad := range tc.absent {
				if strings.Contains(got, bad) {
					t.Errorf("unexpected %q in:\n%s", bad, got)
				}
			}
			if len(got) > 2000 {
				t.Errorf("rendered error is %d bytes; should stay bounded", len(got))
			}
		})
	}
}

// Field order must not depend on Go's map iteration order.
func TestAPIErrorFieldOrderIsStable(t *testing.T) {
	body := []byte(`{"z":["z"],"a":["a"],"m":["m"]}`)
	first := apiError(400, body).Error()
	for i := 0; i < 20; i++ {
		if got := apiError(400, body).Error(); got != first {
			t.Fatalf("unstable output:\n%s\nvs\n%s", first, got)
		}
	}
	if !strings.Contains(first, "a:") || strings.Index(first, "a:") > strings.Index(first, "z:") {
		t.Errorf("fields not sorted:\n%s", first)
	}
}
