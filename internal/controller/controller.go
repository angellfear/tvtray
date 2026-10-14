// Package controller ties the TVs to the PC: it follows display/sleep
// events, polls TV power state and runs volume/backlight commands.
package controller

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"tvtray/internal/config"
	"tvtray/internal/i18n"
	"tvtray/internal/platform"
	"tvtray/internal/samsung"
)

type Controller struct {
	store *config.Store
	log   *slog.Logger

	// OnChange is called (from any goroutine) when TV state changes.
	OnChange func()

	mu        sync.Mutex
	remotes   map[string]*samsung.Remote
	online    map[string]bool
	waking    map[string]context.CancelFunc
	blTarget  map[string]int       // pending backlight target per TV
	blRunning map[string]bool      // macro worker active per TV
	onSince   map[string]time.Time // when the TV was last seen coming online
	autoBL    map[string]string    // schedule point last applied per TV ("HH:MM=V")
	autoRetry map[string]time.Time // no scheduled retry before this time
	offTimer  *time.Timer
}

func New(store *config.Store, log *slog.Logger) *Controller {
	return &Controller{
		store:     store,
		log:       log,
		remotes:   map[string]*samsung.Remote{},
		online:    map[string]bool{},
		waking:    map[string]context.CancelFunc{},
		blTarget:  map[string]int{},
		blRunning: map[string]bool{},
		onSince:   map[string]time.Time{},
		autoBL:    map[string]string{},
		autoRetry: map[string]time.Time{},
	}
}

func (c *Controller) changed() {
	if c.OnChange != nil {
		c.OnChange()
	}
}

func (c *Controller) remote(tv config.TV) *samsung.Remote {
	c.mu.Lock()
	defer c.mu.Unlock()
	r := c.remotes[tv.MAC]
	if r == nil {
		r = samsung.NewRemote(tv.IP)
		r.Secure, r.Token = tv.Secure, tv.Token
		mac, name := tv.MAC, tv.Name
		r.OnAuth = func(secure bool, token string) {
			c.log.Info("tv paired", "tv", name, "secure", secure)
			c.store.UpdateTV(mac, func(t *config.TV) { t.Secure, t.Token = secure, token })
		}
		c.remotes[tv.MAC] = r
	} else {
		r.SetIP(tv.IP)
	}
	return r
}

func (c *Controller) tv(mac string) (config.TV, error) {
	tv, ok := c.store.TV(mac)
	if !ok {
		return tv, i18n.Errorf("err.tvNotFound", mac)
	}
	return tv, nil
}

// Online reports the last polled power state.
func (c *Controller) Online(mac string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.online[mac]
}

func (c *Controller) setOnline(mac string, on bool) {
	c.mu.Lock()
	prev, known := c.online[mac]
	c.online[mac] = on
	if on && (!known || !prev) {
		c.onSince[mac] = time.Now()
	}
	c.mu.Unlock()
	if !known || prev != on {
		c.log.Info("tv state", "mac", mac, "online", on)
		c.changed()
	}
}

// Run polls TV state until ctx is done.
func (c *Controller) Run(ctx context.Context) {
	t := time.NewTicker(3 * time.Second)
	defer t.Stop()
	for {
		c.poll()
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}

// Poll checks all TVs right now.
func (c *Controller) poll() {
	cfg := c.store.Get()
	var arp samsung.ARP
	for _, tv := range cfg.TVs {
		on := samsung.Alive(tv.IP)
		if !on {
			// The TV may have got a new address from DHCP.
			if arp == nil {
				arp = samsung.ARPTable()
			}
			for ip, mac := range arp {
				if mac == tv.MAC && ip != tv.IP {
					if samsung.Alive(ip) {
						c.log.Info("tv moved", "mac", tv.MAC, "from", tv.IP, "to", ip)
						c.store.UpdateTV(tv.MAC, func(t *config.TV) { t.IP = ip })
						on = true
					}
					break
				}
			}
		}
		c.setOnline(tv.MAC, on)
	}
	c.applyTimeBacklight(cfg)
}

// Refresh polls immediately (e.g. when the panel opens).
func (c *Controller) Refresh() { go c.poll() }

// ---- power -----------------------------------------------------------------

// HandlePower is the platform.PowerHandler.
func (c *Controller) HandlePower(ev platform.PowerEvent) {
	cfg := c.store.Get()
	c.log.Info("power event", "event", ev.String())
	switch ev {
	case platform.DisplayOff:
		c.mu.Lock()
		if c.offTimer != nil {
			c.offTimer.Stop()
		}
		c.offTimer = time.AfterFunc(time.Duration(cfg.OffDelaySec)*time.Second, func() {
			c.forLinked(func(tv config.TV) { c.turnOff(tv, 5*time.Second) })
		})
		c.mu.Unlock()

	case platform.Suspend:
		c.cancelOffTimer()
		// Windows gives us ~2 s before sleeping.
		var wg sync.WaitGroup
		c.forLinked(func(tv config.TV) {
			wg.Add(1)
			go func() { defer wg.Done(); c.turnOff(tv, 1500*time.Millisecond) }()
		})
		wg.Wait()

	case platform.DisplayOn, platform.Resume:
		c.cancelOffTimer()
		c.forLinked(func(tv config.TV) { go c.TurnOn(tv.MAC) })
	}
}

func (c *Controller) cancelOffTimer() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.offTimer != nil {
		c.offTimer.Stop()
		c.offTimer = nil
	}
}

func (c *Controller) forLinked(fn func(tv config.TV)) {
	for _, tv := range c.store.Get().TVs {
		if tv.LinkPower {
			fn(*tv)
		}
	}
}

// turnOff sends KEY_POWER only when the TV is on: on a TV in network
// standby the same key would switch it on.
func (c *Controller) turnOff(tv config.TV, timeout time.Duration) {
	c.stopWaking(tv.MAC)
	if !samsung.Alive(tv.IP) {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	if err := c.remote(tv).Press(ctx, "KEY_POWER"); err != nil {
		c.log.Warn("power off failed", "tv", tv.Name, "err", err)
		return
	}
	c.log.Info("power off sent", "tv", tv.Name)
}

func (c *Controller) stopWaking(mac string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if cancel := c.waking[mac]; cancel != nil {
		cancel()
		delete(c.waking, mac)
	}
}

// TurnOn sends Wake-on-LAN until the TV answers or the timeout expires.
func (c *Controller) TurnOn(mac string) error {
	tv, err := c.tv(mac)
	if err != nil {
		return err
	}
	if samsung.Alive(tv.IP) {
		c.setOnline(mac, true)
		return nil
	}
	timeout := time.Duration(c.store.Get().WakeTimeoutSec) * time.Second

	c.mu.Lock()
	if c.waking[mac] != nil {
		c.mu.Unlock()
		return nil // already waking
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	c.waking[mac] = cancel
	c.mu.Unlock()
	defer c.stopWaking(mac)

	c.log.Info("waking", "tv", tv.Name, "mac", mac)
	t := time.NewTicker(time.Second)
	defer t.Stop()
	for i := 0; ; i++ {
		if i%2 == 0 {
			if err := samsung.WakeOnLAN(tv.MAC); err != nil {
				c.log.Warn("wol failed", "err", err)
			}
		}
		select {
		case <-ctx.Done():
			if ctx.Err() == context.DeadlineExceeded {
				c.log.Warn("tv did not wake", "tv", tv.Name)
				return i18n.Errorf("err.noWake", tv.Name)
			}
			return nil
		case <-t.C:
		}
		if samsung.Alive(tv.IP) {
			c.log.Info("tv is up", "tv", tv.Name)
			c.setOnline(mac, true)
			return nil
		}
	}
}

// TogglePower turns the TV off if it is on, otherwise wakes it.
func (c *Controller) TogglePower(mac string) error {
	tv, err := c.tv(mac)
	if err != nil {
		return err
	}
	if samsung.Alive(tv.IP) {
		c.turnOff(tv, 5*time.Second)
		return nil
	}
	return c.TurnOn(mac)
}

// ---- keys & volume ---------------------------------------------------------

func (c *Controller) Key(mac, key string) error {
	tv, err := c.tv(mac)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	return c.remote(tv).Press(ctx, key)
}

// Volume returns the current volume, or -1 if unknown.
func (c *Controller) Volume(mac string) (int, error) {
	tv, err := c.tv(mac)
	if err != nil {
		return -1, err
	}
	if tv.VolumeMode == config.VolumeKeys {
		return tv.Volume, nil
	}
	return samsung.GetVolume(context.Background(), tv.IP)
}

func (c *Controller) SetVolume(mac string, v int) error {
	tv, err := c.tv(mac)
	if err != nil {
		return err
	}
	v = clamp(v, 0, 100)
	cur := tv.Volume
	if tv.VolumeMode != config.VolumeKeys {
		err := samsung.SetVolume(context.Background(), tv.IP, v)
		if err == nil {
			return nil
		}
		// Some models (e.g. 2017 MU) report the volume over UPnP but reject
		// SetVolume; then step there with VOL+/VOL- from the reported value.
		got, gerr := samsung.GetVolume(context.Background(), tv.IP)
		if gerr != nil {
			return err
		}
		cur = got
	}
	if cur < 0 {
		cur = v // unknown: just adopt the slider value
	}
	key, n := "KEY_VOLUP", v-cur
	if n < 0 {
		key, n = "KEY_VOLDOWN", -n
	}
	keys := make([]string, n)
	for i := range keys {
		keys[i] = key
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := c.remote(tv).Press(ctx, keys...); err != nil {
		return err
	}
	return c.store.UpdateTV(mac, func(t *config.TV) { t.Volume = v })
}

// ---- backlight -------------------------------------------------------------

// SetBacklight queues a backlight change. Calls made while a macro is running
// are coalesced: only the latest target is applied afterwards.
func (c *Controller) SetBacklight(mac string, v int, resync bool) error {
	tv, err := c.tv(mac)
	if err != nil {
		return err
	}
	v = clamp(v, 0, tv.Backlight.Max)
	if resync {
		c.store.UpdateTV(mac, func(t *config.TV) { t.Backlight.Current = -1 })
	}
	c.mu.Lock()
	c.blTarget[mac] = v
	if c.blRunning[mac] {
		c.mu.Unlock()
		return nil
	}
	c.blRunning[mac] = true
	c.mu.Unlock()

	go c.backlightWorker(mac)
	return nil
}

func (c *Controller) backlightWorker(mac string) {
	for {
		c.mu.Lock()
		target, ok := c.blTarget[mac]
		delete(c.blTarget, mac)
		if !ok {
			c.blRunning[mac] = false
			c.mu.Unlock()
			return
		}
		c.mu.Unlock()

		tv, err := c.tv(mac)
		if err != nil {
			continue
		}
		if err := c.runBacklight(tv, target); err != nil {
			c.log.Warn("backlight failed", "tv", tv.Name, "err", err)
			// Let the schedule try again later instead of on every poll.
			c.mu.Lock()
			delete(c.autoBL, mac)
			c.autoRetry[mac] = time.Now().Add(time.Minute)
			c.mu.Unlock()
			continue
		}
		c.store.UpdateTV(mac, func(t *config.TV) { t.Backlight.Current = target })
		c.changed()
	}
}

func (c *Controller) runBacklight(tv config.TV, target int) error {
	b := tv.Backlight
	nav := time.Duration(b.NavDelayMs) * time.Millisecond
	step := time.Duration(b.StepDelayMs) * time.Millisecond
	delay := func(s config.Step) time.Duration {
		if s.Pause > 0 {
			return time.Duration(s.Pause) * time.Millisecond
		}
		return nav
	}

	var seq []samsung.Key
	// Clean start: close whatever the previous (possibly broken) macro left
	// on screen, so OpenKeys always begin from the same state.
	for _, k := range b.CloseKeys {
		seq = append(seq, samsung.Key{Key: k.Key, Delay: delay(k)})
	}
	for _, k := range b.OpenKeys {
		seq = append(seq, samsung.Key{Key: k.Key, Delay: delay(k)})
	}
	cur := b.Current
	if cur < 0 || cur > b.Max {
		// Unknown value: go to zero first, then up.
		for i := 0; i < b.Max; i++ {
			seq = append(seq, samsung.Key{Key: "KEY_LEFT", Delay: step})
		}
		cur = 0
	}
	key, n := "KEY_RIGHT", target-cur
	if n < 0 {
		key, n = "KEY_LEFT", -n
	}
	for i := 0; i < n; i++ {
		seq = append(seq, samsung.Key{Key: key, Delay: step})
	}
	for _, k := range b.CloseKeys {
		seq = append(seq, samsung.Key{Key: k.Key, Delay: delay(k)})
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	return c.remote(tv).SendKeys(ctx, seq...)
}

// TestKeys sends a raw sequence with the menu pause (calibration helper).
func (c *Controller) TestKeys(mac string, keys []config.Step) error {
	tv, err := c.tv(mac)
	if err != nil {
		return err
	}
	seq := make([]samsung.Key, len(keys))
	for i, k := range keys {
		pause := tv.Backlight.NavDelayMs
		if k.Pause > 0 {
			pause = k.Pause
		}
		seq[i] = samsung.Key{Key: k.Key, Delay: time.Duration(pause) * time.Millisecond}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	return c.remote(tv).SendKeys(ctx, seq...)
}

// ---- backlight schedule ----------------------------------------------------

// bootGrace: a TV that has just come online is still starting up and drops
// menu keys, so the schedule waits this long after it appears.
const bootGrace = 20 * time.Second

// desiredBacklight returns the schedule point in effect at now: the last one
// passed today, or — before the first point of the day — the last point of
// yesterday. id identifies the point, so an edited value applies again.
func desiredBacklight(b config.Backlight, now time.Time) (value int, id string, ok bool) {
	mins := now.Hour()*60 + now.Minute()
	bestAt, lastAt := -1, -1
	var best, last config.ScheduleItem
	for _, it := range b.Schedule {
		at, valid := config.ParseClock(it.Time)
		if !valid {
			continue
		}
		if at <= mins && at > bestAt {
			best, bestAt = it, at
		}
		if at > lastAt {
			last, lastAt = it, at
		}
	}
	if bestAt < 0 {
		best, bestAt = last, lastAt
	}
	if bestAt < 0 {
		return 0, "", false
	}
	return best.Value, fmt.Sprintf("%02d:%02d=%d", bestAt/60, bestAt%60, best.Value), true
}

// applyTimeBacklight switches the backlight by schedule. Runs from poll
// (every 3 s) and sends only when the point in effect changed. The value is
// derived from the clock, so a TV that was off at the switch time picks the
// point up once it is online again; a manual change lasts until the next point.
func (c *Controller) applyTimeBacklight(cfg config.Config) {
	now := time.Now()
	for _, tv := range cfg.TVs {
		b := tv.Backlight
		if !b.ByTime {
			continue
		}
		want, id, ok := desiredBacklight(b, now)
		if !ok {
			continue
		}
		c.mu.Lock()
		ready := c.online[tv.MAC] && now.Sub(c.onSince[tv.MAC]) >= bootGrace && now.After(c.autoRetry[tv.MAC])
		done := c.autoBL[tv.MAC] == id
		if ready && !done {
			c.autoBL[tv.MAC] = id
		}
		c.mu.Unlock()
		if !ready || done {
			continue
		}
		if b.Current == clamp(want, 0, b.Max) {
			continue // already there (e.g. after an app restart)
		}
		c.log.Info("scheduled backlight", "tv", tv.Name, "point", id)
		c.SetBacklight(tv.MAC, want, false)
	}
}

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
