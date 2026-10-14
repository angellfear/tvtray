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

func TestScheduleDue(t *testing.T) {
	c := New(nil, nil)
	day := func(hh, mm int) time.Time { return time.Date(2026, 10, 14, hh, mm, 0, 0, time.Local) }
	tv := &config.TV{MAC: "M", Name: "tv", Backlight: config.Backlight{
		ByTime: true, Max: 20, Current: 15,
		Schedule: []config.ScheduleItem{{Time: "08:00", Value: 15}, {Time: "22:00", Value: 4}},
	}}
	cfg := config.Config{TVs: []*config.TV{tv}}
	c.online["M"], c.onSince["M"] = true, day(0, 0)

	if d := c.scheduleDue(cfg, day(21, 0)); len(d) != 0 {
		t.Fatalf("enabling the schedule must not send: %+v", d)
	}
	// Editing the schedule (a point now in effect) must not send either.
	tv.Backlight.Schedule = append(tv.Backlight.Schedule, config.ScheduleItem{Time: "20:00", Value: 9})
	if d := c.scheduleDue(cfg, day(21, 1)); len(d) != 0 {
		t.Fatalf("an edit must not send: %+v", d)
	}
	// The clock reaches 22:00.
	d := c.scheduleDue(cfg, day(22, 0))
	if len(d) != 1 || d[0].value != 4 {
		t.Fatalf("22:00: %+v", d)
	}
	if d := c.scheduleDue(cfg, day(22, 1)); len(d) != 0 {
		t.Fatalf("sent twice: %+v", d)
	}

	// TV off at 08:00: the switch waits until it is up and has booted.
	tv.Backlight.Current = 4
	c.online["M"] = false
	if d := c.scheduleDue(cfg, day(8, 0).AddDate(0, 0, 1)); len(d) != 0 {
		t.Fatalf("sent to an offline TV: %+v", d)
	}
	on := day(9, 0).AddDate(0, 0, 1)
	c.online["M"], c.onSince["M"] = true, on
	if d := c.scheduleDue(cfg, on.Add(5*time.Second)); len(d) != 0 {
		t.Fatalf("sent while booting: %+v", d)
	}
	if d := c.scheduleDue(cfg, on.Add(bootGrace)); len(d) != 1 || d[0].value != 15 {
		t.Fatalf("catch-up: %+v", d)
	}
}
