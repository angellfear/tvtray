// Package config stores TvTray settings in TvTray.ini next to the executable.
// The presence of that file is also how the app knows it is installed.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"gopkg.in/ini.v1"
)

const FileName = "TvTray.ini"

const (
	VolumeUPnP = "upnp" // absolute volume via UPnP RenderingControl
	VolumeKeys = "keys" // KEY_VOLUP/KEY_VOLDOWN, value tracked by the app
)

// Step is one key of a macro. Pause (ms) overrides the default pause between
// presses for this key; 0 means "use the default".
type Step struct {
	Key   string `json:"key"`
	Pause int    `json:"pause"`
}

// ScheduleItem switches the backlight to Value when the clock reaches Time.
type ScheduleItem struct {
	Time  string `json:"time"`  // "HH:MM"
	Value int    `json:"value"` // backlight level
}

// Backlight describes the remote-key macro that reaches the backlight slider
// in the TV menu. The app tracks the current value itself and sends only the
// difference as LEFT/RIGHT presses. Schedule switches the level by the clock;
// a TV that was off at the switch time catches up when it turns on.
type Backlight struct {
	Enabled   bool           `json:"enabled"`
	ByTime    bool           `json:"byTime"` // follow the schedule
	Schedule  []ScheduleItem `json:"schedule"`
	OpenKeys  []Step         `json:"openKeys"`
	CloseKeys []Step         `json:"closeKeys"`
	Max       int            `json:"max"`
	Current   int            `json:"current"` // -1 = unknown, needs sync
	// NavDelayMs: pause after each Open/Close key (menu animations), unless
	// the step sets its own. StepDelayMs: pause between LEFT/RIGHT presses.
	NavDelayMs  int `json:"navDelayMs"`
	StepDelayMs int `json:"stepDelayMs"`
}

func DefaultBacklight() Backlight {
	return Backlight{
		OpenKeys:    []Step{{Key: "KEY_MENU"}, {Key: "KEY_ENTER"}, {Key: "KEY_ENTER"}},
		CloseKeys:   []Step{{Key: "KEY_EXIT"}},
		Max:         20,
		Current:     -1,
		NavDelayMs:  700,
		StepDelayMs: 150,
	}
}

// ParseClock parses "HH:MM" (also "H:MM") into minutes since midnight.
func ParseClock(s string) (int, bool) {
	t, err := time.Parse("15:04", strings.TrimSpace(s))
	if err != nil {
		return 0, false
	}
	return t.Hour()*60 + t.Minute(), true
}

// Sanitize drops broken steps and schedule points (a hand-edited ini or a
// stale page) and clamps the values.
func (b *Backlight) Sanitize() {
	cleanSteps := func(in []Step) []Step {
		out := make([]Step, 0, len(in))
		for _, s := range in {
			s.Key = strings.ToUpper(strings.TrimSpace(s.Key))
			if s.Key == "" {
				continue
			}
			if s.Pause < 0 {
				s.Pause = 0
			}
			out = append(out, s)
		}
		return out
	}
	b.OpenKeys = cleanSteps(b.OpenKeys)
	b.CloseKeys = cleanSteps(b.CloseKeys)
	if b.Max < 1 {
		b.Max = 1
	}
	sched := make([]ScheduleItem, 0, len(b.Schedule))
	for _, it := range b.Schedule {
		m, ok := ParseClock(it.Time)
		if !ok {
			continue
		}
		it.Time = fmt.Sprintf("%02d:%02d", m/60, m%60)
		it.Value = clampInt(it.Value, 0, b.Max)
		sched = append(sched, it)
	}
	sort.SliceStable(sched, func(i, j int) bool { return sched[i].Time < sched[j].Time })
	b.Schedule = sched
	d := DefaultBacklight()
	if b.NavDelayMs <= 0 {
		b.NavDelayMs = d.NavDelayMs
	}
	if b.StepDelayMs <= 0 {
		b.StepDelayMs = d.StepDelayMs
	}
}

func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// ParseSteps reads "KEY,KEY:1200,..." (old plain lists load with pause 0).
func ParseSteps(s string) []Step {
	var out []Step
	for _, part := range strings.FieldsFunc(s, func(r rune) bool { return r == ',' || r == ';' }) {
		key, pause, _ := strings.Cut(strings.TrimSpace(part), ":")
		key = strings.ToUpper(strings.TrimSpace(key))
		if key == "" {
			continue
		}
		ms, _ := strconv.Atoi(strings.TrimSpace(pause))
		out = append(out, Step{Key: key, Pause: max(0, ms)})
	}
	return out
}

func FormatSteps(steps []Step) string {
	parts := make([]string, len(steps))
	for i, s := range steps {
		if s.Pause > 0 {
			parts[i] = s.Key + ":" + strconv.Itoa(s.Pause)
		} else {
			parts[i] = s.Key
		}
	}
	return strings.Join(parts, ",")
}

// parseSchedule reads "HH:MM=V;HH:MM=V".
func parseSchedule(s string) []ScheduleItem {
	var out []ScheduleItem
	for _, part := range strings.Split(s, ";") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		timeStr, val, ok := strings.Cut(part, "=")
		m, valid := ParseClock(timeStr)
		v, err := strconv.Atoi(strings.TrimSpace(val))
		if !ok || err != nil || !valid {
			continue
		}
		out = append(out, ScheduleItem{Time: fmt.Sprintf("%02d:%02d", m/60, m%60), Value: v})
	}
	return out
}

func formatSchedule(items []ScheduleItem) string {
	parts := make([]string, len(items))
	for i, it := range items {
		parts[i] = it.Time + "=" + strconv.Itoa(it.Value)
	}
	return strings.Join(parts, ";")
}

type TV struct {
	MAC         string    `json:"mac"` // identity; IP may change
	IP          string    `json:"ip"`
	Name        string    `json:"name"`
	Model       string    `json:"model"`
	LinkPower   bool      `json:"linkPower"`   // turn off/on together with the PC display
	ShowInPanel bool      `json:"showInPanel"` // show in the left-click panel
	VolumeMode  string    `json:"volumeMode"`
	Volume      int       `json:"volume"` // last known value, used in keys mode
	Backlight   Backlight `json:"backlight"`
	// Secure: the TV needs wss://:8002 with a token (2017+ models).
	Secure bool   `json:"secure"`
	Token  string `json:"token"`
}

type Config struct {
	TVs []*TV `json:"tvs"`
	// Delay between the display turning off and the TV power-off, so short
	// blanks (e.g. a resolution change) don't cycle the TV.
	OffDelaySec int `json:"offDelaySec"`
	// How long to keep sending Wake-on-LAN after the display comes back.
	WakeTimeoutSec int `json:"wakeTimeoutSec"`
	// Start with Windows (HKCU Run); applied by the app on every start.
	Autostart bool `json:"autostart"`
	// UI language: ru, en, de ("" = system language).
	Language string `json:"language"`
}

func NewTV(mac, ip, name, model string) *TV {
	return &TV{
		MAC:         mac,
		IP:          ip,
		Name:        name,
		Model:       model,
		ShowInPanel: true,
		VolumeMode:  VolumeUPnP,
		Volume:      -1,
		Backlight:   DefaultBacklight(),
	}
}

func Defaults() *Config {
	return &Config{OffDelaySec: 5, WakeTimeoutSec: 40, Autostart: true}
}

// ExeDir is the directory of the running executable.
func ExeDir() string {
	exe, err := os.Executable()
	if err != nil {
		return "."
	}
	if p, err := filepath.EvalSymlinks(exe); err == nil {
		exe = p
	}
	return filepath.Dir(exe)
}

// PathIn returns the ini path inside dir.
func PathIn(dir string) string { return filepath.Join(dir, FileName) }

// Installed reports whether an ini file sits next to the executable.
func Installed() bool {
	_, err := os.Stat(PathIn(ExeDir()))
	return err == nil
}

// Store guards the config and persists it on every change.
type Store struct {
	mu   sync.Mutex
	path string
	cfg  *Config
}

// Load reads the ini at path. A missing file yields defaults.
func Load(path string) (*Store, error) {
	s := &Store{path: path, cfg: Defaults()}
	if _, err := os.Stat(path); err != nil {
		return s, nil
	}
	f, err := ini.Load(path)
	if err != nil {
		return s, err
	}
	g := f.Section("general")
	s.cfg.OffDelaySec = g.Key("off_delay_sec").MustInt(5)
	s.cfg.WakeTimeoutSec = g.Key("wake_timeout_sec").MustInt(40)
	s.cfg.Autostart = g.Key("autostart").MustBool(true)
	s.cfg.Language = g.Key("language").String()

	for _, sec := range f.Sections() {
		if !strings.HasPrefix(sec.Name(), "tv ") {
			continue
		}
		d := DefaultBacklight()
		tv := &TV{
			MAC:         sec.Key("mac").String(),
			IP:          sec.Key("ip").String(),
			Name:        sec.Key("name").String(),
			Model:       sec.Key("model").String(),
			LinkPower:   sec.Key("link_power").MustBool(false),
			ShowInPanel: sec.Key("show_in_panel").MustBool(true),
			VolumeMode:  sec.Key("volume_mode").In(VolumeUPnP, []string{VolumeUPnP, VolumeKeys}),
			Volume:      sec.Key("volume").MustInt(-1),
			Secure:      sec.Key("secure").MustBool(false),
			Token:       sec.Key("token").String(),
			Backlight: Backlight{
				Enabled:     sec.Key("backlight_enabled").MustBool(false),
				ByTime:      sec.Key("backlight_by_time").MustBool(false),
				Schedule:    parseSchedule(sec.Key("backlight_schedule").String()),
				OpenKeys:    ParseSteps(sec.Key("backlight_open_keys").MustString(FormatSteps(d.OpenKeys))),
				CloseKeys:   ParseSteps(sec.Key("backlight_close_keys").MustString(FormatSteps(d.CloseKeys))),
				Max:         sec.Key("backlight_max").MustInt(d.Max),
				Current:     sec.Key("backlight_current").MustInt(-1),
				NavDelayMs:  sec.Key("backlight_nav_delay_ms").MustInt(d.NavDelayMs),
				StepDelayMs: sec.Key("backlight_step_delay_ms").MustInt(d.StepDelayMs),
			},
		}
		if tv.MAC != "" {
			s.cfg.TVs = append(s.cfg.TVs, tv)
		}
	}
	return s, nil
}

// Path is the ini file location.
func (s *Store) Path() string { return s.path }

// Get returns a deep copy, safe to read without holding the lock.
func (s *Store) Get() Config {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := *s.cfg
	out.TVs = make([]*TV, len(s.cfg.TVs))
	for i, tv := range s.cfg.TVs {
		c := cloneTV(tv)
		out.TVs[i] = &c
	}
	return out
}

// Update applies fn to the live config and saves it.
func (s *Store) Update(fn func(c *Config)) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	fn(s.cfg)
	return s.saveLocked()
}

// TV returns a copy of the TV with the given MAC.
func (s *Store) TV(mac string) (TV, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, tv := range s.cfg.TVs {
		if tv.MAC == mac {
			return cloneTV(tv), true
		}
	}
	return TV{}, false
}

// UpdateTV applies fn to the TV with the given MAC and saves.
func (s *Store) UpdateTV(mac string, fn func(tv *TV)) error {
	return s.Update(func(c *Config) {
		for _, tv := range c.TVs {
			if tv.MAC == mac {
				fn(tv)
			}
		}
	})
}

// Save writes the current config (used to create the ini on install).
func (s *Store) Save() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.saveLocked()
}

func cloneTV(tv *TV) TV {
	c := *tv
	c.Backlight.OpenKeys = append([]Step(nil), tv.Backlight.OpenKeys...)
	c.Backlight.CloseKeys = append([]Step(nil), tv.Backlight.CloseKeys...)
	c.Backlight.Schedule = append([]ScheduleItem(nil), tv.Backlight.Schedule...)
	return c
}

func (s *Store) saveLocked() error {
	f := ini.Empty()
	g := f.Section("general")
	g.Key("off_delay_sec").SetValue(fmt.Sprint(s.cfg.OffDelaySec))
	g.Key("wake_timeout_sec").SetValue(fmt.Sprint(s.cfg.WakeTimeoutSec))
	g.Key("autostart").SetValue(fmt.Sprint(s.cfg.Autostart))
	g.Key("language").SetValue(s.cfg.Language)

	tvs := append([]*TV(nil), s.cfg.TVs...)
	sort.SliceStable(tvs, func(i, j int) bool { return tvs[i].Name < tvs[j].Name })
	for _, tv := range tvs {
		sec := f.Section("tv " + strings.ReplaceAll(tv.MAC, ":", ""))
		set := func(k string, v any) { sec.Key(k).SetValue(fmt.Sprint(v)) }
		set("mac", tv.MAC)
		set("ip", tv.IP)
		set("name", tv.Name)
		set("model", tv.Model)
		set("link_power", tv.LinkPower)
		set("show_in_panel", tv.ShowInPanel)
		set("volume_mode", tv.VolumeMode)
		set("volume", tv.Volume)
		set("secure", tv.Secure)
		set("token", tv.Token)
		set("backlight_enabled", tv.Backlight.Enabled)
		set("backlight_by_time", tv.Backlight.ByTime)
		set("backlight_schedule", formatSchedule(tv.Backlight.Schedule))
		set("backlight_open_keys", FormatSteps(tv.Backlight.OpenKeys))
		set("backlight_close_keys", FormatSteps(tv.Backlight.CloseKeys))
		set("backlight_max", tv.Backlight.Max)
		set("backlight_current", tv.Backlight.Current)
		set("backlight_nav_delay_ms", tv.Backlight.NavDelayMs)
		set("backlight_step_delay_ms", tv.Backlight.StepDelayMs)
	}

	tmp := s.path + ".tmp"
	if err := f.SaveTo(tmp); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}
