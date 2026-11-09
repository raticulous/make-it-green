package engine

import (
	"testing"
	"time"
)

func TestInHours(t *testing.T) {
	c := DefaultConfig()                                                                                // 9-18, weekdays only
	at := func(day, hour int) time.Time { return time.Date(2026, 10, day, hour, 30, 0, 0, time.Local) } // 2026-10-19 is a Monday
	cases := []struct {
		t    time.Time
		want bool
	}{
		{at(19, 9), true},
		{at(19, 17), true},
		{at(19, 18), false},
		{at(19, 8), false},
		{at(24, 12), false}, // Saturday
	}
	for _, tc := range cases {
		if got := c.InHours(tc.t); got != tc.want {
			t.Errorf("InHours(%v) = %v, want %v", tc.t, got, tc.want)
		}
	}
	c.WeekdaysOnly = false
	if !c.InHours(at(24, 12)) {
		t.Error("weekend should count when WeekdaysOnly is false")
	}
}

func TestResumeAt(t *testing.T) {
	c := DefaultConfig()                                                                                // 9-18, weekdays only, next_day
	at := func(day, hour int) time.Time { return time.Date(2026, 10, day, hour, 30, 0, 0, time.Local) } // 2026-10-19 is a Monday
	start := func(day int) time.Time { return time.Date(2026, 10, day, 9, 0, 0, 0, time.Local) }
	cases := []struct{ now, want time.Time }{
		{at(19, 7), start(19)},  // before work: today
		{at(19, 12), start(20)}, // during work: tomorrow
		{at(23, 12), start(26)}, // Friday: Monday
		{at(24, 12), start(26)}, // Saturday: Monday
	}
	for _, tc := range cases {
		if got := c.ResumeAt(tc.now); !got.Equal(tc.want) {
			t.Errorf("ResumeAt(%v) = %v, want %v", tc.now, got, tc.want)
		}
	}
	c.PauseMode, c.PauseMinutes = "minutes", 15
	if got := c.ResumeAt(at(19, 12)); !got.Equal(at(19, 12).Add(15 * time.Minute)) {
		t.Errorf("minutes mode: got %v", got)
	}
}
