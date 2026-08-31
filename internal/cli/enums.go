package cli

import (
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/timestripe/timestripe-cli/internal/dates"
)

// Enum tables, hand-written and kept honest by enums_test.go, which re-parses
// api/openapi.yaml and fails if any table drifts from the spec.
//
// The generated constants are not usable here: oapi-codegen emits bare
// api.Day/api.Week, api.Hash278dea for colors, api.MinusDatetime for sorts, and
// members like GoalHorizonLessThannil = "<nil>" for the nullable variants.
var (
	enumHorizon = []string{"day", "week", "month", "quarter", "year", "decade", "life"}
	enumColor   = []string{"#ecce32", "#df496d", "#92ce14", "#278dea", "#955be0", "#f2713a"}
	enumLayout  = []string{"horizontal", "vertical"}

	enumSortGoals       = sortable("assignee", "checked", "created_datetime", "date", "horizon", "modified_datetime", "name")
	enumSortBoards      = sortable("archived", "name", "sequence_no")
	enumSortBuckets     = sortable("created_datetime", "name", "sequence_no")
	enumSortComments    = sortable("created_datetime", "modified_datetime")
	enumSortEvents      = sortable("datetime")
	enumSortFolders     = sortable("created_datetime", "name", "sequence_no")
	enumSortFolderGoals = sortable("created_datetime", "sequence_no")

	enumEventType = []string{
		"BOARD_CREATED", "BOARD_DELETED", "BOARD_MODIFIED",
		"BUCKET_CREATED", "BUCKET_DELETED", "BUCKET_MODIFIED",
		"CLIMB_SUBSCRIPTION_CREATED", "CLIMB_SUBSCRIPTION_DELETED",
		"COMMENT_CREATED", "COMMENT_DELETED", "COMMENT_MODIFIED",
		"GOAL_ASSIGNED", "GOAL_CREATED", "GOAL_DELETED", "GOAL_DONE",
		"GOAL_MODIFIED", "GOAL_RESCHEDULED",
		"MEMBERSHIP_CREATED", "MEMBERSHIP_DELETED", "MENTION_CREATED",
		"SPACE_CREATED", "SPACE_CREATED_BY_CLONING", "SPACE_USED_FOR_CLONING",
		"USER_CREATED",
	}
)

// sortable expands field names into the ascending and descending forms the API
// accepts, mirroring how the spec spells them out.
func sortable(fields ...string) []string {
	out := make([]string, 0, len(fields)*2)
	out = append(out, fields...)
	for _, f := range fields {
		out = append(out, "-"+f)
	}
	return out
}

// normalizeColor accepts "ecce32" for "#ecce32".
func normalizeColor(v string) string {
	if v != "" && !strings.HasPrefix(v, "#") {
		return "#" + v
	}
	return v
}

// validateEnum checks a flag value against its table when the flag was set.
// Nullable fields also accept "none".
//
// Called at the top of RunE, before newAPIClient, so a typo costs no network
// round trip and no auth.
func validateEnum(cmd *cobra.Command, flag string, allowed []string, nullable bool) error {
	if !cmd.Flags().Changed(flag) {
		return nil
	}
	v := cmd.Flags().Lookup(flag).Value.String()
	if nullable && dates.IsNone(v) {
		return nil
	}
	if flag == "color" {
		v = normalizeColor(v)
	}
	for _, a := range allowed {
		if v == a {
			return nil
		}
	}
	msg := fmt.Sprintf("--%s: %q is not valid", flag, v)
	if s := didYouMean(v, allowed); s != "" {
		msg += fmt.Sprintf("\ndid you mean %q?", s)
	}
	msg += "\nvalid: " + strings.Join(allowed, ", ")
	if nullable {
		msg += ", none"
	}
	return fmt.Errorf("%s", msg)
}

// validateEnumSlice is the repeatable-flag form, used by --horizon on lists.
func validateEnumSlice(cmd *cobra.Command, flag string, values, allowed []string) error {
	if !cmd.Flags().Changed(flag) {
		return nil
	}
	for _, v := range values {
		found := false
		for _, a := range allowed {
			if v == a {
				found = true
				break
			}
		}
		if found {
			continue
		}
		msg := fmt.Sprintf("--%s: %q is not valid", flag, v)
		if s := didYouMean(v, allowed); s != "" {
			msg += fmt.Sprintf("\ndid you mean %q?", s)
		}
		return fmt.Errorf("%s\nvalid: %s", msg, strings.Join(allowed, ", "))
	}
	return nil
}

// didYouMean returns the closest candidate within a small edit distance, or a
// prefix match, or "".
func didYouMean(v string, allowed []string) string {
	v = strings.ToLower(v)
	best, bestDist := "", 3 // only suggest within edit distance 2
	for _, a := range allowed {
		d := editDistance(v, strings.ToLower(a))
		if d < bestDist {
			best, bestDist = a, d
		}
	}
	if best != "" {
		return best
	}
	for _, a := range allowed {
		la := strings.ToLower(a)
		if strings.HasPrefix(la, v) || strings.HasPrefix(v, la) {
			return a
		}
	}
	return ""
}

func editDistance(a, b string) int {
	ra, rb := []rune(a), []rune(b)
	prev := make([]int, len(rb)+1)
	cur := make([]int, len(rb)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(ra); i++ {
		cur[0] = i
		for j := 1; j <= len(rb); j++ {
			cost := 1
			if ra[i-1] == rb[j-1] {
				cost = 0
			}
			cur[j] = min3(cur[j-1]+1, prev[j]+1, prev[j-1]+cost)
		}
		prev, cur = cur, prev
	}
	return prev[len(rb)]
}

func min3(a, b, c int) int {
	if b < a {
		a = b
	}
	if c < a {
		a = c
	}
	return a
}

// completeEnum registers static shell completion for a flag. Free at runtime,
// and it makes the valid values discoverable without reading --help.
func completeEnum(cmd *cobra.Command, flag string, allowed []string, nullable bool) {
	values := allowed
	if nullable {
		values = append(append([]string{}, allowed...), dates.NoneLiteral)
	}
	_ = cmd.RegisterFlagCompletionFunc(flag, cobra.FixedCompletions(values, cobra.ShellCompDirectiveNoFileComp))
}

// sortedCopy is used by tests to compare against the spec order-insensitively.
func sortedCopy(in []string) []string {
	out := append([]string{}, in...)
	sort.Strings(out)
	return out
}
