package platform

import (
	"bufio"
	"bytes"
	"errors"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/godbus/dbus/v5"
)

// WatchPower reports sleep via logind and display power via the first
// source that works on this desktop:
//
//  1. GNOME/Mutter: org.gnome.Mutter.DisplayConfig.PowerSaveMode (X11 and Wayland)
//  2. X11: DPMS state polled from `xset q`
//  3. Fallback: org.freedesktop/org.gnome ScreenSaver ActiveChanged (KDE etc.)
func WatchPower(h PowerHandler) error {
	var errs []error
	if err := watchSleep(h); err != nil {
		errs = append(errs, err)
	}
	if err := watchDisplay(h); err != nil {
		errs = append(errs, err)
	}
	return errors.Join(errs...)
}

// ---- sleep (logind) ---------------------------------------------------------

func watchSleep(h PowerHandler) error {
	conn, err := dbus.ConnectSystemBus()
	if err != nil {
		return err
	}
	login := conn.Object("org.freedesktop.login1", "/org/freedesktop/login1")

	// A "delay" inhibitor makes logind wait (up to InhibitDelayMaxSec, 5 s by
	// default) until we close the fd, giving us time to switch the TVs off.
	var lock dbus.UnixFD = -1
	takeLock := func() {
		var fd dbus.UnixFD
		if err := login.Call("org.freedesktop.login1.Manager.Inhibit", 0,
			"sleep", "TvTray", "Turn TVs off before sleep", "delay").Store(&fd); err != nil {
			Logger.Warn("logind inhibit", "err", err)
			return
		}
		lock = fd
	}
	releaseLock := func() {
		if lock >= 0 {
			os.NewFile(uintptr(lock), "inhibit").Close()
			lock = -1
		}
	}

	if err := conn.AddMatchSignal(
		dbus.WithMatchInterface("org.freedesktop.login1.Manager"),
		dbus.WithMatchMember("PrepareForSleep"),
	); err != nil {
		return err
	}
	takeLock()
	ch := make(chan *dbus.Signal, 8)
	conn.Signal(ch)
	go func() {
		for s := range ch {
			if s.Name != "org.freedesktop.login1.Manager.PrepareForSleep" || len(s.Body) == 0 {
				continue
			}
			if going, _ := s.Body[0].(bool); going {
				h(Suspend)
				releaseLock()
			} else {
				h(Resume)
				takeLock()
			}
		}
	}()
	return nil
}

// ---- display ---------------------------------------------------------------

// displayState dedups transitions and drops the initial state.
type displayState struct {
	mu    sync.Mutex
	known bool
	on    bool
	h     PowerHandler
}

func (d *displayState) set(on bool) {
	d.mu.Lock()
	changed := d.known && d.on != on
	d.known, d.on = true, on
	d.mu.Unlock()
	if !changed {
		return
	}
	if on {
		d.h(DisplayOn)
	} else {
		d.h(DisplayOff)
	}
}

func watchDisplay(h PowerHandler) error {
	st := &displayState{h: h}
	sess, err := dbus.ConnectSessionBus()
	if err == nil {
		if watchMutter(sess, st) {
			Logger.Info("display source", "source", "mutter")
			return nil
		}
	}
	if os.Getenv("DISPLAY") != "" && os.Getenv("WAYLAND_DISPLAY") == "" {
		if _, err := exec.LookPath("xset"); err == nil {
			Logger.Info("display source", "source", "xset dpms")
			go pollXset(st)
			return nil
		}
	}
	if sess != nil {
		if err := watchScreenSaver(sess, st); err == nil {
			Logger.Info("display source", "source", "screensaver")
			return nil
		}
	}
	return errors.New("no display power source found (need GNOME, X11 with xset, or a ScreenSaver D-Bus service)")
}

const (
	mutterName  = "org.gnome.Mutter.DisplayConfig"
	mutterPath  = "/org/gnome/Mutter/DisplayConfig"
	propsIface  = "org.freedesktop.DBus.Properties"
	mutterPower = "PowerSaveMode"
)

// watchMutter follows PowerSaveMode: 0 = on, 1/2/3 = standby/suspend/off.
func watchMutter(conn *dbus.Conn, st *displayState) bool {
	obj := conn.Object(mutterName, mutterPath)
	v, err := obj.GetProperty(mutterName + "." + mutterPower)
	if err != nil {
		return false
	}
	mode, _ := v.Value().(int32)
	st.set(mode == 0)

	if err := conn.AddMatchSignal(
		dbus.WithMatchObjectPath(mutterPath),
		dbus.WithMatchInterface(propsIface),
		dbus.WithMatchMember("PropertiesChanged"),
	); err != nil {
		return false
	}
	ch := make(chan *dbus.Signal, 8)
	conn.Signal(ch)
	go func() {
		for s := range ch {
			if s.Path != mutterPath || s.Name != propsIface+".PropertiesChanged" || len(s.Body) < 2 {
				continue
			}
			changed, _ := s.Body[1].(map[string]dbus.Variant)
			if v, ok := changed[mutterPower]; ok {
				mode, _ := v.Value().(int32)
				st.set(mode == 0)
			}
		}
	}()
	return true
}

func pollXset(st *displayState) {
	for {
		out, err := exec.Command("xset", "q").Output()
		if err == nil {
			if on, ok := parseXset(out); ok {
				st.set(on)
			}
		}
		time.Sleep(2 * time.Second)
	}
}

// parseXset reads "Monitor is On/Off/In Standby/In Suspend" from `xset q`.
func parseXset(out []byte) (on, ok bool) {
	sc := bufio.NewScanner(bytes.NewReader(out))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if strings.HasPrefix(line, "Monitor is ") {
			return strings.TrimPrefix(line, "Monitor is ") == "On", true
		}
	}
	return false, false
}

// watchScreenSaver treats an active screensaver as "display off".
func watchScreenSaver(conn *dbus.Conn, st *displayState) error {
	found := false
	for _, iface := range []string{"org.freedesktop.ScreenSaver", "org.gnome.ScreenSaver"} {
		if err := conn.AddMatchSignal(dbus.WithMatchInterface(iface), dbus.WithMatchMember("ActiveChanged")); err == nil {
			found = true
		}
	}
	if !found {
		return errors.New("no screensaver service")
	}
	st.set(true)
	ch := make(chan *dbus.Signal, 8)
	conn.Signal(ch)
	go func() {
		for s := range ch {
			if !strings.HasSuffix(s.Name, ".ActiveChanged") || len(s.Body) == 0 {
				continue
			}
			if active, ok := s.Body[0].(bool); ok {
				st.set(!active)
			}
		}
	}()
	return nil
}
