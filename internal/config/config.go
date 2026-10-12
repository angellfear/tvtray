// Package config stores TvTray settings in TvTray.ini next to the executable.
// The presence of that file is also how the app knows it is installed.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"gopkg.in/ini.v1"
)

const FileName = "TvTray.ini"

const (
	VolumeUPnP = "upnp" // absolute volume via UPnP RenderingControl
	VolumeKeys = "keys" // KEY_VOLUP/KEY_VOLDOWN, value tracked by the app
)

// Backlight describes the remote-key macro that reaches the backlight slider
// in the TV menu. The app tracks the current value itself and sends only the
// difference as LEFT/RIGHT presses.
type Backlight struct {
	Enabled     bool     `json:"enabled"`
	OpenKeys    []string `json:"openKeys"`  // keys that open the backlight slider
	CloseKeys   []string `json:"closeKeys"` // keys that close the menu again
	Max         int      `json:"max"`
	Current     int      `json:"current"`     // -1 = unknown, needs sync
	NavDelayMs  int      `json:"navDelayMs"`  // pause after each Open/Close key (menu animations)
	StepDelayMs int      `json:"stepDelayMs"` // pause between LEFT/RIGHT presses
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
}

func DefaultBacklight() Backlight {
	return Backlight{
		OpenKeys:    []string{"KEY_MENU", "KEY_ENTER", "KEY_ENTER"},
		CloseKeys:   []string{"KEY_EXIT"},
		Max:         20,
		Current:     -1,
		NavDelayMs:  700,
		StepDelayMs: 150,
	}
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
			Backlight: Backlight{
				Enabled:     sec.Key("backlight_enabled").MustBool(false),
				OpenKeys:    splitKeys(sec.Key("backlight_open_keys").MustString(strings.Join(d.OpenKeys, ","))),
				CloseKeys:   splitKeys(sec.Key("backlight_close_keys").MustString(strings.Join(d.CloseKeys, ","))),
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

func splitKeys(s string) []string {
	var out []string
	for _, k := range strings.FieldsFunc(s, func(r rune) bool { return r == ',' || r == ' ' || r == ';' }) {
		out = append(out, strings.ToUpper(k))
	}
	return out
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
	c.Backlight.OpenKeys = append([]string(nil), tv.Backlight.OpenKeys...)
	c.Backlight.CloseKeys = append([]string(nil), tv.Backlight.CloseKeys...)
	return c
}

func (s *Store) saveLocked() error {
	f := ini.Empty()
	g := f.Section("general")
	g.Key("off_delay_sec").SetValue(fmt.Sprint(s.cfg.OffDelaySec))
	g.Key("wake_timeout_sec").SetValue(fmt.Sprint(s.cfg.WakeTimeoutSec))
	g.Key("autostart").SetValue(fmt.Sprint(s.cfg.Autostart))

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
		set("backlight_enabled", tv.Backlight.Enabled)
		set("backlight_open_keys", strings.Join(tv.Backlight.OpenKeys, ","))
		set("backlight_close_keys", strings.Join(tv.Backlight.CloseKeys, ","))
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
