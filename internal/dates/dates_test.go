package dates

import (
	"testing"
	"time"
)

// Wednesday 2026-08-26, so weekday arithmetic has a known anchor.
var now = time.Date(2026, 8, 26, 14, 30, 0, 0, time.UTC)

func TestParseDate(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"2026-01-15", "2026-01-15"},
		{"2026-01-15T09:00:00Z", "2026-01-15"},
		{"today", "2026-08-26"},
		{"TODAY", "2026-08-26"},
		{" tomorrow ", "2026-08-27"},
		{"yesterday", "2026-08-25"},
		{"+1d", "2026-08-27"},
		{"-1d", "2026-08-25"},
		{"+3d", "2026-08-29"},
		{"+1w", "2026-09-02"},
		{"-2w", "2026-08-12"},
		{"+1m", "2026-09-26"},
		{"+1y", "2027-08-26"},
		{"3 days", "2026-08-29"},
		{"+2 weeks", "2026-09-09"},

		// Wednesday anchor: Friday is 2 days out.
		{"friday", "2026-08-28"},
		{"fri", "2026-08-28"},
		{"next friday", "2026-08-28"},
		{"last friday", "2026-08-21"},
		// Bare weekday including today: Wednesday resolves to today...
		{"wednesday", "2026-08-26"},
		// ...but "next wednesday" is strictly after.
		{"next wednesday", "2026-09-02"},
		{"last wednesday", "2026-08-19"},
		// Monday already passed this week; the bare form looks forward.
		{"monday", "2026-08-31"},
	}
	for _, tc := range tests {
		t.Run(tc.in, func(t *testing.T) {
			got, err := ParseDate(tc.in, now)
			if err != nil {
				t.Fatalf("ParseDate(%q): %v", tc.in, err)
			}
			if s := got.Format("2006-01-02"); s != tc.want {
				t.Errorf("ParseDate(%q) = %s, want %s", tc.in, s, tc.want)
			}
			if h, m, sec := got.Clock(); h != 0 || m != 0 || sec != 0 {
				t.Errorf("ParseDate(%q) is not midnight: %v", tc.in, got)
			}
		})
	}
}

// +1m from Jan 31 must clamp to Feb 28, not overflow into March as
// time.AddDate does.
func TestMonthOffsetClampsToMonthEnd(t *testing.T) {
	jan31 := time.Date(2026, 1, 31, 0, 0, 0, 0, time.UTC)
	got, err := ParseDate("+1m", jan31)
	if err != nil {
		t.Fatal(err)
	}
	if s := got.Format("2006-01-02"); s != "2026-02-28" {
		t.Errorf("+1m from Jan 31 = %s, want 2026-02-28", s)
	}

	// Leap year.
	jan31Leap := time.Date(2028, 1, 31, 0, 0, 0, 0, time.UTC)
	got, _ = ParseDate("+1m", jan31Leap)
	if s := got.Format("2006-01-02"); s != "2028-02-29" {
		t.Errorf("+1m from Jan 31 2028 = %s, want 2028-02-29", s)
	}
}

// Every bare weekday must resolve to today or the next 6 days, never the past.
func TestBareWeekdayNeverGoesBackwards(t *testing.T) {
	for day := 0; day < 7; day++ {
		anchor := now.AddDate(0, 0, day)
		for name := range weekdays {
			got, err := ParseDate(name, anchor)
			if err != nil {
				t.Fatalf("%s from %s: %v", name, anchor.Format("Mon"), err)
			}
			delta := int(got.Sub(time.Date(anchor.Year(), anchor.Month(), anchor.Day(), 0, 0, 0, 0, anchor.Location())).Hours() / 24)
			if delta < 0 || delta > 6 {
				t.Errorf("%q from %s resolved %d days away", name, anchor.Format("Mon"), delta)
			}
		}
	}
}

func TestParseDateRejectsGarbage(t *testing.T) {
	for _, in := range []string{"", "nxt fri", "2026-13-01", "someday", "+", "+d", "5x"} {
		if _, err := ParseDate(in, now); err == nil {
			t.Errorf("ParseDate(%q) should have failed", in)
		}
	}
}

func TestParseDateErrorNamesTheGrammar(t *testing.T) {
	_, err := ParseDate("nxt fri", now)
	if err == nil {
		t.Fatal("expected an error")
	}
	for _, want := range []string{"YYYY-MM-DD", "today", "none"} {
		if !contains(err.Error(), want) {
			t.Errorf("error does not mention %q:\n%s", want, err)
		}
	}
}

func TestParseClock(t *testing.T) {
	tests := map[string]string{
		"09:00": "09:00", "9:00": "09:00", "21:30": "21:30",
		"9": "09:00", "21": "21:00",
		"9am": "09:00", "9pm": "21:00", "9:30pm": "21:30",
		"12am": "00:00", "12pm": "12:00",
		"09:00:00": "09:00",
	}
	for in, want := range tests {
		got, err := ParseClock(in)
		if err != nil {
			t.Errorf("ParseClock(%q): %v", in, err)
			continue
		}
		if got != want {
			t.Errorf("ParseClock(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestParseClockRejectsGarbage(t *testing.T) {
	for _, in := range []string{"", "25:00", "noon", "abc", "99"} {
		if got, err := ParseClock(in); err == nil {
			t.Errorf("ParseClock(%q) = %q, should have failed", in, got)
		}
	}
}

func TestIsNone(t *testing.T) {
	for _, in := range []string{"none", "NONE", " null ", "clear"} {
		if !IsNone(in) {
			t.Errorf("IsNone(%q) = false", in)
		}
	}
	for _, in := range []string{"", "today", "nonexistent"} {
		if IsNone(in) {
			t.Errorf("IsNone(%q) = true", in)
		}
	}
}

func contains(haystack, needle string) bool {
	return len(haystack) >= len(needle) && (func() bool {
		for i := 0; i+len(needle) <= len(haystack); i++ {
			if haystack[i:i+len(needle)] == needle {
				return true
			}
		}
		return false
	})()
}
