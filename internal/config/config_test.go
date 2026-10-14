package config

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestBacklightRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), FileName)
	st, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	tv := NewTV("AA:BB:CC", "192.168.0.24", "TV", "UE49KU6470")
	tv.Backlight.Enabled = true
	tv.Backlight.ByTime = true
	tv.Backlight.Schedule = []ScheduleItem{{Time: "08:00", Value: 15}, {Time: "22:00", Value: 4}}
	tv.Backlight.OpenKeys = []Step{{Key: "KEY_MENU"}, {Key: "KEY_ENTER", Pause: 1500}}
	tv.Backlight.CloseKeys = []Step{{Key: "KEY_EXIT"}}
	tv.Backlight.PauseMs = 450
	st.Update(func(c *Config) { c.TVs = append(c.TVs, tv) })

	st2, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	got, ok := st2.TV("AA:BB:CC")
	if !ok {
		t.Fatal("tv lost after reload")
	}
	b := got.Backlight
	if !b.Enabled || !b.ByTime || b.PauseMs != 450 {
		t.Fatalf("flags lost: %+v", b)
	}
	if len(b.Schedule) != 2 || b.Schedule[1].Time != "22:00" || b.Schedule[1].Value != 4 {
		t.Fatalf("schedule: %+v", b.Schedule)
	}
	if len(b.OpenKeys) != 2 || b.OpenKeys[1].Key != "KEY_ENTER" || b.OpenKeys[1].Pause != 1500 {
		t.Fatalf("openKeys: %+v", b.OpenKeys)
	}
	if len(b.CloseKeys) != 1 || b.CloseKeys[0].Pause != 0 {
		t.Fatalf("closeKeys: %+v", b.CloseKeys)
	}
}

func TestParseStepsOldList(t *testing.T) {
	// An ini written before per-key pauses existed still loads.
	steps := parseSteps("key_menu, KEY_ENTER ,key_down")
	if len(steps) != 3 || steps[0].Key != "KEY_MENU" || steps[2].Key != "KEY_DOWN" {
		t.Fatalf("got %+v", steps)
	}
	for _, s := range steps {
		if s.Pause != 0 {
			t.Fatalf("pause should be 0, got %+v", steps)
		}
	}
	if got := formatSteps([]Step{{Key: "KEY_A"}, {Key: "KEY_B", Pause: 1200}}); got != "KEY_A,KEY_B:1200" {
		t.Fatalf("formatSteps: %q", got)
	}
}

func TestParseSchedule(t *testing.T) {
	// "7:00" is leniently accepted (Go clock parsing), the rest is dropped.
	got := parseSchedule(" 08:00 = 15 ; 22:00=4 ; garbage ; 25:00=1 ; 7:00=2")
	if len(got) != 3 || got[0].Time != "08:00" || got[1].Value != 4 || got[2].Time != "7:00" {
		t.Fatalf("got %+v", got)
	}
	if s := formatSchedule([]ScheduleItem{{Time: "22:00", Value: 4}}); s != "22:00=4" {
		t.Fatalf("formatSchedule: %q", s)
	}
}

func TestSanitize(t *testing.T) {
	b := DefaultBacklight()
	b.Max = 20
	b.PauseMs = -5
	b.OpenKeys = []Step{{Key: " key_down ", Pause: -3}, {Key: ""}}
	b.Schedule = []ScheduleItem{{Time: "22:00", Value: 99}, {Time: "nope", Value: 1}, {Time: "08:00", Value: -4}}
	b.Sanitize()
	if b.PauseMs != DefaultPauseMs {
		t.Fatalf("pause %d", b.PauseMs)
	}
	if len(b.OpenKeys) != 1 || b.OpenKeys[0].Key != "KEY_DOWN" || b.OpenKeys[0].Pause != 0 {
		t.Fatalf("openKeys: %+v", b.OpenKeys)
	}
	if len(b.Schedule) != 2 || b.Schedule[0].Value != 20 || b.Schedule[1].Value != 0 {
		t.Fatalf("schedule: %+v", b.Schedule)
	}
	if !ValidTime("") || !ValidTime("08:00") || ValidTime("25:00") {
		t.Fatal("ValidTime is wrong")
	}
	if strings.Contains(formatSteps(parseSteps("KEY_A,KEY_B:60")), " ") {
		t.Fatal("formatSteps produced spaces")
	}
}
