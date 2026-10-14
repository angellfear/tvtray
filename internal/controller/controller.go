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
	blTarget  map[string]int         // pending backlight target per TV
	blRunning map[string]bool        // macro worker active per TV
	onSince   map[string]time.Time   // when the TV was last seen coming online
	sched     map[string]*schedState // backlight schedule progress per TV
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
		sched:     map[string]*schedState{},
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
			// A scheduled switch is retried a minute later, not on every poll.
			c.mu.Lock()
			if st := c.sched[mac]; st != nil && st.running {
				st.pending, st.retryAt = true, time.Now().Add(time.Minute)
			}
			c.mu.Unlock()
			continue
		}
		c.store.UpdateTV(mac, func(t *config.TV) { t.Backlight.Current = target })
		c.mu.Lock()
		if st := c.sched[mac]; st != nil {
			st.running = false
		}
		c.mu.Unlock()
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

// schedState follows the schedule of one TV.
type schedState struct {
	sig     string    // schedule as last seen; an edit only re-baselines
	point   string    // time of the point in effect at the last poll
	pending bool      // a switch is due but not yet sent
	running bool      // the pending switch was handed to the macro worker
	retryAt time.Time // no retry before this time
}

// desiredBacklight returns the schedule point in effect at now: the last one
// passed today, or — before the first point of the day — the last point of
// yesterday. at is the point's time ("HH:MM").
func desiredBacklight(b config.Backlight, now time.Time) (value int, at string, ok bool) {
	mins := now.Hour()*60 + now.Minute()
	bestAt, lastAt := -1, -1
	var best, last config.ScheduleItem
	for _, it := range b.Schedule {
		m, valid := config.ParseClock(it.Time)
		if !valid {
			continue
		}
		if m <= mins && m > bestAt {
			best, bestAt = it, m
		}
		if m > lastAt {
			last, lastAt = it, m
		}
	}
	if bestAt < 0 {
		best, bestAt = last, lastAt
	}
	if bestAt < 0 {
		return 0, "", false
	}
	return best.Value, fmt.Sprintf("%02d:%02d", bestAt/60, bestAt%60), true
}

// applyTimeBacklight switches the backlight by schedule; runs from poll.
// A switch happens only when the clock reaches the next point. If the TV was
// off then, it is sent once the TV is up. Editing the schedule or turning it
// on never sends anything by itself, and a manual change lasts until the
// next point.
func (c *Controller) applyTimeBacklight(cfg config.Config) {
	for _, d := range c.scheduleDue(cfg, time.Now()) {
		c.log.Info("scheduled backlight", "tv", d.name, "point", d.at, "value", d.value)
		c.SetBacklight(d.mac, d.value, false)
	}
}

type dueSwitch struct {
	mac, name, at string
	value         int
}

// scheduleDue advances the schedule state to now and returns the switches to send.
func (c *Controller) scheduleDue(cfg config.Config, now time.Time) []dueSwitch {
	c.mu.Lock()
	defer c.mu.Unlock()
	var out []dueSwitch
	for _, tv := range cfg.TVs {
		b := tv.Backlight
		want, at, ok := desiredBacklight(b, now)
		sig := fmt.Sprint(b.Schedule)
		st := c.sched[tv.MAC]
		switch {
		case !b.ByTime || !ok:
			delete(c.sched, tv.MAC)
			continue
		case st == nil || st.sig != sig:
			// New, edited or just enabled schedule: remember where we are,
			// send nothing.
			st = &schedState{sig: sig, point: at}
			c.sched[tv.MAC] = st
		case st.point != at:
			st.point, st.pending, st.running = at, true, false
		}
		if st.pending && !st.running && !now.Before(st.retryAt) &&
			c.online[tv.MAC] && now.Sub(c.onSince[tv.MAC]) >= bootGrace {
			st.pending = false
			if b.Current != clamp(want, 0, b.Max) {
				st.running = true
				out = append(out, dueSwitch{tv.MAC, tv.Name, at, want})
			}
		}
	}
	return out
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
