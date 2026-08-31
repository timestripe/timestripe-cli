// Package dates parses the human date and time forms the CLI accepts.
//
// Every entry point takes an explicit "now" so callers are testable without
// mocking the clock.
package dates

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// NoneLiteral clears a nullable field. The API marks date, horizon, color and
// the foreign keys nullable, but there was previously no way to clear one from
// a flag: --date "" sends an empty string and is rejected.
const NoneLiteral = "none"

// IsNone reports whether s asks to clear the field.
func IsNone(s string) bool {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case NoneLiteral, "null", "clear":
		return true
	}
	return false
}

// Accepted describes the grammar, for help text and error messages.
func Accepted() string {
	return `accepted: YYYY-MM-DD; today|tomorrow|yesterday; a weekday ` +
		`(mon..sun, optionally "next"/"last"); an offset (+3d, -1w, +2m); ` +
		`or "none" to clear the field`
}

var weekdays = map[string]time.Weekday{
	"sunday": time.Sunday, "sun": time.Sunday,
	"monday": time.Monday, "mon": time.Monday,
	"tuesday": time.Tuesday, "tue": time.Tuesday, "tues": time.Tuesday,
	"wednesday": time.Wednesday, "wed": time.Wednesday,
	"thursday": time.Thursday, "thu": time.Thursday, "thur": time.Thursday, "thurs": time.Thursday,
	"friday": time.Friday, "fri": time.Friday,
	"saturday": time.Saturday, "sat": time.Saturday,
}

// ParseDate resolves s to a local midnight.
//
// Weekday rule: a bare "friday" means the next Friday *including today*, so
// typing it on a Friday means today rather than a week out. "next friday" is
// strictly after today and "last friday" strictly before.
func ParseDate(s string, now time.Time) (time.Time, error) {
	v := strings.ToLower(strings.TrimSpace(s))
	if v == "" {
		return time.Time{}, fmt.Errorf("empty date")
	}
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, now.Location()), nil
	}
	if t, err := time.ParseInLocation("2006-01-02", v, now.Location()); err == nil {
		return t, nil
	}

	switch v {
	case "today", "tod", "now":
		return today, nil
	case "tomorrow", "tmr", "tom":
		return today.AddDate(0, 0, 1), nil
	case "yesterday", "yd":
		return today.AddDate(0, 0, -1), nil
	}

	if t, ok := parseOffset(v, today); ok {
		return t, nil
	}
	if t, ok := parseWeekday(v, today); ok {
		return t, nil
	}
	return time.Time{}, fmt.Errorf("cannot parse %q\n%s", s, Accepted())
}

// parseOffset handles +3d, -1w, "+2 months", "3 days".
func parseOffset(v string, today time.Time) (time.Time, bool) {
	v = strings.ReplaceAll(v, " ", "")
	if v == "" {
		return time.Time{}, false
	}
	sign := 1
	switch v[0] {
	case '+':
		v = v[1:]
	case '-':
		sign = -1
		v = v[1:]
	}
	i := 0
	for i < len(v) && v[i] >= '0' && v[i] <= '9' {
		i++
	}
	if i == 0 {
		return time.Time{}, false
	}
	n, err := strconv.Atoi(v[:i])
	if err != nil {
		return time.Time{}, false
	}
	n *= sign
	switch strings.TrimSuffix(v[i:], "s") {
	case "d", "day":
		return today.AddDate(0, 0, n), true
	case "w", "week", "wk":
		return today.AddDate(0, 0, 7*n), true
	case "m", "month", "mo":
		return addMonths(today, n), true
	case "y", "year", "yr":
		return today.AddDate(n, 0, 0), true
	}
	return time.Time{}, false
}

// addMonths clamps to the last day of the target month, so +1m from Jan 31
// lands on Feb 28 rather than overflowing into March as AddDate would.
func addMonths(t time.Time, n int) time.Time {
	year, month, day := t.Date()
	first := time.Date(year, month, 1, 0, 0, 0, 0, t.Location()).AddDate(0, n, 0)
	last := time.Date(first.Year(), first.Month()+1, 0, 0, 0, 0, 0, t.Location()).Day()
	if day > last {
		day = last
	}
	return time.Date(first.Year(), first.Month(), day, 0, 0, 0, 0, t.Location())
}

func parseWeekday(v string, today time.Time) (time.Time, bool) {
	strictlyAfter, strictlyBefore := false, false
	switch {
	case strings.HasPrefix(v, "next"):
		strictlyAfter = true
		v = strings.TrimSpace(strings.TrimPrefix(v, "next"))
	case strings.HasPrefix(v, "last"):
		strictlyBefore = true
		v = strings.TrimSpace(strings.TrimPrefix(v, "last"))
	case strings.HasPrefix(v, "this"):
		v = strings.TrimSpace(strings.TrimPrefix(v, "this"))
	}
	wd, ok := weekdays[v]
	if !ok {
		return time.Time{}, false
	}
	cur := int(today.Weekday())
	want := int(wd)

	if strictlyBefore {
		delta := cur - want
		if delta <= 0 {
			delta += 7
		}
		return today.AddDate(0, 0, -delta), true
	}
	delta := (want - cur + 7) % 7
	if strictlyAfter && delta == 0 {
		delta = 7
	}
	return today.AddDate(0, 0, delta), true
}

// ParseClock normalises a time of day to the "HH:MM" the API expects for
// start_time and end_time, which are format:time, not datetimes.
func ParseClock(s string) (string, error) {
	v := strings.ToLower(strings.TrimSpace(s))
	if v == "" {
		return "", fmt.Errorf("empty time")
	}
	for _, layout := range []string{"15:04", "15:04:05", "3:04pm", "3pm", "3:04 pm", "3 pm", "1504"} {
		if t, err := time.Parse(layout, v); err == nil {
			return t.Format("15:04"), nil
		}
	}
	// A bare hour: "9" or "21".
	if n, err := strconv.Atoi(v); err == nil && n >= 0 && n <= 23 {
		return fmt.Sprintf("%02d:00", n), nil
	}
	return "", fmt.Errorf("cannot parse %q as a time of day (expected HH:MM, e.g. 09:00 or 21:30; 9am and 9:30pm also work)", s)
}
