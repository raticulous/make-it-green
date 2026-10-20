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
