package controller

import (
	"testing"
	"time"

	"tvtray/internal/config"
)

func TestDesiredBacklight(t *testing.T) {
	b := config.Backlight{Schedule: []config.ScheduleItem{
		{Time: "08:00", Value: 15},
		{Time: "22:00", Value: 4},
	}}
	cases := []struct {
		hh, mm int
		want   int
		ok     bool
	}{
		{7, 59, 0, false},  // nothing scheduled yet
		{8, 0, 15, true},   // day level
		{12, 0, 15, true},  // still day
		{21, 59, 15, true}, // still day
		{22, 0, 4, true},   // night level
		{23, 30, 4, true},  // still night
	}
	for _, c := range cases {
		now := time.Date(2026, 10, 14, c.hh, c.mm, 0, 0, time.Local)
		got, ok := desiredBacklight(b, now)
		if got != c.want || ok != c.ok {
			t.Errorf("at %02d:%02d got (%d, %v), want (%d, %v)", c.hh, c.mm, got, ok, c.want, c.ok)
		}
	}
}

func TestDesiredBacklightIgnoresBroken(t *testing.T) {
	b := config.Backlight{Schedule: []config.ScheduleItem{
		{Time: "garbage", Value: 5},
		{Time: "08:00", Value: 9},
	}}
	now := time.Date(2026, 10, 14, 12, 0, 0, 0, time.Local)
	if got, ok := desiredBacklight(b, now); !ok || got != 9 {
		t.Errorf("got (%d, %v), want (9, true)", got, ok)
	}
	if _, ok := desiredBacklight(config.Backlight{Schedule: []config.ScheduleItem{{Time: "bad", Value: 1}}}, now); ok {
		t.Error("broken-only schedule should not apply")
	}
}
