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
	}{
		{0, 30, 4},   // after midnight: yesterday's night point
		{7, 59, 4},   // still night
		{8, 0, 15},   // day level
		{12, 0, 15},  // still day
		{21, 59, 15}, // still day
		{22, 0, 4},   // night level
		{23, 30, 4},  // still night
	}
	for _, c := range cases {
		now := time.Date(2026, 10, 14, c.hh, c.mm, 0, 0, time.Local)
		got, _, ok := desiredBacklight(b, now)
		if got != c.want || !ok {
			t.Errorf("at %02d:%02d got (%d, %v), want %d", c.hh, c.mm, got, ok, c.want)
		}
	}
}

func TestDesiredBacklightIgnoresBroken(t *testing.T) {
	b := config.Backlight{Schedule: []config.ScheduleItem{
		{Time: "garbage", Value: 5},
		{Time: "08:00", Value: 9},
	}}
	now := time.Date(2026, 10, 14, 12, 0, 0, 0, time.Local)
	if got, _, ok := desiredBacklight(b, now); !ok || got != 9 {
		t.Errorf("got (%d, %v), want (9, true)", got, ok)
	}
	if _, _, ok := desiredBacklight(config.Backlight{Schedule: []config.ScheduleItem{{Time: "bad", Value: 1}}}, now); ok {
		t.Error("broken-only schedule should not apply")
	}
}
