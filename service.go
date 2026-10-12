package main

import (
	"context"
	"errors"
	"log/slog"
	"net/url"
	"os"
	"strings"
	"sync"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"

	"tvtray/internal/config"
	"tvtray/internal/controller"
	"tvtray/internal/install"
	"tvtray/internal/platform"
	"tvtray/internal/samsung"
)

const panelWidth = 340

// TvService is called from the frontend as main.TvService.<Method>.
type TvService struct {
	app   *application.App
	store *config.Store
	ctrl  *controller.Controller
	log   *slog.Logger
	tray  *application.SystemTray
	panel *application.WebviewWindow

	mu       sync.Mutex
	settings *application.WebviewWindow
	remotes  map[string]*application.WebviewWindow
}

// PanelTV is what the panel needs to draw one TV group.
type PanelTV struct {
	MAC          string `json:"mac"`
	Name         string `json:"name"`
	Model        string `json:"model"`
	Online       bool   `json:"online"`
	LinkPower    bool   `json:"linkPower"`
	VolumeMode   string `json:"volumeMode"`
	Volume       int    `json:"volume"` // keys mode only; upnp is fetched via Volume()
	BacklightOn  bool   `json:"backlightOn"`
	Backlight    int    `json:"backlight"`
	BacklightMax int    `json:"backlightMax"`
}

func (s *TvService) list(onlyPanel bool) []PanelTV {
	out := []PanelTV{}
	for _, tv := range s.store.Get().TVs {
		if onlyPanel && !tv.ShowInPanel {
			continue
		}
		out = append(out, PanelTV{
			MAC: tv.MAC, Name: tv.Name, Model: tv.Model,
			Online:       s.ctrl.Online(tv.MAC),
			LinkPower:    tv.LinkPower,
			VolumeMode:   tv.VolumeMode,
			Volume:       tv.Volume,
			BacklightOn:  tv.Backlight.Enabled,
			Backlight:    tv.Backlight.Current,
			BacklightMax: tv.Backlight.Max,
		})
	}
	return out
}

func (s *TvService) emitState() {
	if s.app == nil {
		return
	}
	s.app.Event.Emit("state", s.list(false))
}

// ---- panel -----------------------------------------------------------------

func (s *TvService) PanelState() []PanelTV {
	s.ctrl.Refresh()
	return s.list(true)
}

// ResizePanel fits the panel to its content and re-anchors it at the tray.
func (s *TvService) ResizePanel(height int) {
	if height < 60 {
		height = 60
	}
	s.panel.SetSize(panelWidth, height)
	if s.panel.IsVisible() {
		s.tray.PositionWindow(s.panel, 8)
	}
}

func (s *TvService) Power(mac string) error            { return s.ctrl.TogglePower(mac) }
func (s *TvService) Key(mac, key string) error         { return s.ctrl.Key(mac, key) }
func (s *TvService) Volume(mac string) (int, error)    { return s.ctrl.Volume(mac) }
func (s *TvService) SetVolume(mac string, v int) error { return s.ctrl.SetVolume(mac, v) }
func (s *TvService) SetBacklight(mac string, v int) error {
	return s.ctrl.SetBacklight(mac, v, false)
}
func (s *TvService) SyncBacklight(mac string, v int) error {
	return s.ctrl.SetBacklight(mac, v, true)
}

func (s *TvService) OpenRemote(mac string) {
	tv, ok := s.store.TV(mac)
	if !ok {
		return
	}
	s.panel.Hide()
	s.mu.Lock()
	w := s.remotes[mac]
	if w == nil {
		w = s.app.Window.NewWithOptions(application.WebviewWindowOptions{
			Name:                "remote-" + mac,
			Title:               "Пульт — " + tv.Name,
			Width:               320,
			Height:              640,
			DisableResize:       true,
			URL:                 "/remote.html?mac=" + url.QueryEscape(mac),
			MaximiseButtonState: application.ButtonHidden,
		})
		w.RegisterHook(events.Common.WindowClosing, func(e *application.WindowEvent) {
			w.Hide()
			e.Cancel()
		})
		s.remotes[mac] = w
	}
	s.mu.Unlock()
	w.Show()
	w.Focus()
}

func (s *TvService) OpenSettings() {
	s.panel.Hide()
	s.mu.Lock()
	if s.settings == nil {
		s.settings = s.app.Window.NewWithOptions(application.WebviewWindowOptions{
			Name:      "settings",
			Title:     "TvTray — настройки",
			Width:     760,
			Height:    700,
			MinWidth:  600,
			MinHeight: 400,
			URL:       "/settings.html",
		})
		w := s.settings
		w.RegisterHook(events.Common.WindowClosing, func(e *application.WindowEvent) {
			w.Hide()
			e.Cancel()
		})
	}
	w := s.settings
	s.mu.Unlock()
	w.Show()
	w.Restore()
	w.Focus()
}

// ---- settings --------------------------------------------------------------

type SettingsInfo struct {
	Config  config.Config   `json:"config"`
	Online  map[string]bool `json:"online"`
	IniPath string          `json:"iniPath"`
}

func (s *TvService) Settings() SettingsInfo {
	cfg := s.store.Get()
	on := map[string]bool{}
	for _, tv := range cfg.TVs {
		on[tv.MAC] = s.ctrl.Online(tv.MAC)
	}
	return SettingsInfo{Config: cfg, Online: on, IniPath: s.store.Path()}
}

type ScanItem struct {
	MAC   string `json:"mac"`
	IP    string `json:"ip"`
	Name  string `json:"name"`
	Model string `json:"model"`
	Net   string `json:"net"`
	Added bool   `json:"added"`
}

func (s *TvService) Scan() []ScanItem {
	known := map[string]bool{}
	for _, tv := range s.store.Get().TVs {
		known[tv.MAC] = true
	}
	out := []ScanItem{}
	for _, f := range samsung.Scan(context.Background()) {
		if f.MAC == "" {
			continue
		}
		out = append(out, ScanItem{MAC: f.MAC, IP: f.IP, Name: cleanName(f.Name), Model: f.Model, Net: f.Net, Added: known[f.MAC]})
		// Keep the IP of known TVs up to date.
		if known[f.MAC] {
			ip := f.IP
			s.store.UpdateTV(f.MAC, func(t *config.TV) { t.IP = ip })
		}
	}
	return out
}

func cleanName(n string) string { return strings.TrimSpace(strings.TrimPrefix(n, "[TV]")) }

func (s *TvService) AddTV(item ScanItem) error {
	mac := samsung.NormalizeMAC(item.MAC)
	if mac == "" {
		return errors.New("неверный MAC-адрес")
	}
	err := s.store.Update(func(c *config.Config) {
		for _, tv := range c.TVs {
			if tv.MAC == mac {
				tv.IP = item.IP
				return
			}
		}
		c.TVs = append(c.TVs, config.NewTV(mac, item.IP, item.Name, item.Model))
	})
	s.ctrl.Refresh()
	s.emitState()
	return err
}

func (s *TvService) RemoveTV(mac string) error {
	err := s.store.Update(func(c *config.Config) {
		out := c.TVs[:0]
		for _, tv := range c.TVs {
			if tv.MAC != mac {
				out = append(out, tv)
			}
		}
		c.TVs = out
	})
	s.emitState()
	return err
}

func (s *TvService) SaveTV(tv config.TV) error {
	if tv.Backlight.Max < 1 {
		tv.Backlight.Max = 1
	}
	if tv.VolumeMode != config.VolumeKeys {
		tv.VolumeMode = config.VolumeUPnP
	}
	err := s.store.UpdateTV(tv.MAC, func(t *config.TV) { *t = tv })
	s.emitState()
	return err
}

func (s *TvService) SaveGeneral(offDelaySec, wakeTimeoutSec int, autostart bool) error {
	err := s.store.Update(func(c *config.Config) {
		c.OffDelaySec = max(0, offDelaySec)
		c.WakeTimeoutSec = max(5, wakeTimeoutSec)
		c.Autostart = autostart
	})
	s.applyAutostart(autostart)
	return err
}

func (s *TvService) applyAutostart(on bool) {
	var err error
	if on {
		err = s.app.Autostart.Enable()
	} else {
		err = s.app.Autostart.Disable()
	}
	if err != nil {
		s.log.Warn("autostart", "enable", on, "err", err)
	}
}

// TestKeys sends a comma-separated key list (backlight calibration).
func (s *TvService) TestKeys(mac, keys string) error {
	var list []string
	for _, k := range strings.FieldsFunc(keys, func(r rune) bool { return r == ',' || r == ' ' || r == ';' }) {
		list = append(list, strings.ToUpper(k))
	}
	return s.ctrl.TestKeys(mac, list)
}

func (s *TvService) Uninstall() error {
	s.app.Autostart.Disable()
	if err := install.Uninstall(); err != nil {
		return err
	}
	s.app.Quit()
	return nil
}

func (s *TvService) Quit() { s.app.Quit() }

// ---- installer -------------------------------------------------------------

// InstallerService backs install.html (shown when no ini is next to the exe).
type InstallerService struct {
	app *application.App
}

type InstallDefaults struct {
	Dir     string `json:"dir"`
	Current string `json:"current"`
}

func (s *InstallerService) Defaults() InstallDefaults {
	return InstallDefaults{Dir: platform.DefaultInstallDir(), Current: config.ExeDir()}
}

func (s *InstallerService) Browse(current string) (string, error) {
	return s.app.Dialog.OpenFile().
		CanChooseDirectories(true).
		CanChooseFiles(false).
		SetTitle("Папка установки").
		SetDirectory(current).
		PromptForSingleSelection()
}

// Install copies the app into dir, then starts the installed copy and quits.
func (s *InstallerService) Install(dir string, autostart bool) error {
	dir = strings.TrimSpace(dir)
	if dir == "" {
		return errors.New("укажите папку")
	}
	if err := install.Run(dir, autostart); err != nil {
		return err
	}
	if err := platform.StartDetached(install.Target(dir)); err != nil {
		return err
	}
	s.app.Quit()
	return nil
}

// Portable creates the ini next to the current exe and restarts in place.
func (s *InstallerService) Portable(autostart bool) error {
	st, _ := config.Load(config.PathIn(config.ExeDir()))
	if err := st.Update(func(c *config.Config) { c.Autostart = autostart }); err != nil {
		return err
	}
	self, _ := os.Executable()
	if err := platform.StartDetached(self); err != nil {
		return err
	}
	s.app.Quit()
	return nil
}

func (s *InstallerService) Quit() { s.app.Quit() }
