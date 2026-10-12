// Package samsung talks to Samsung Tizen TVs over the local network:
// the WebSocket remote-control channel, UPnP RenderingControl and Wake-on-LAN.
package samsung

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/coder/websocket"
)

// AppName is shown on the TV in the list of connected remote devices.
const AppName = "TvTray"

// Remote is a persistent connection to the TV's remote-control channel.
// It reconnects lazily: the first SendKeys after a failure opens a new socket.
// All methods are safe for concurrent use; key sequences never interleave.
type Remote struct {
	IP string

	mu   sync.Mutex
	conn *websocket.Conn
}

func NewRemote(ip string) *Remote { return &Remote{IP: ip} }

// SetIP switches the remote to a new address, dropping any open connection.
func (r *Remote) SetIP(ip string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.IP != ip {
		r.closeLocked()
		r.IP = ip
	}
}

func (r *Remote) Close() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.closeLocked()
}

func (r *Remote) closeLocked() {
	if r.conn != nil {
		r.conn.Close(websocket.StatusNormalClosure, "")
		r.conn = nil
	}
}

func (r *Remote) connectLocked(ctx context.Context) error {
	if r.conn != nil {
		return nil
	}
	name := base64.StdEncoding.EncodeToString([]byte(AppName))
	u := fmt.Sprintf("ws://%s:8001/api/v2/channels/samsung.remote.control?name=%s", r.IP, url.QueryEscape(name))

	cctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	c, _, err := websocket.Dial(cctx, u, nil)
	if err != nil {
		return fmt.Errorf("connect %s: %w", r.IP, err)
	}
	// The TV answers with ms.channel.connect once the client is accepted.
	// Older firmware may ask the user to allow the device first.
	rctx, rcancel := context.WithTimeout(ctx, 30*time.Second)
	defer rcancel()
	_, msg, err := c.Read(rctx)
	if err != nil {
		c.Close(websocket.StatusNormalClosure, "")
		return fmt.Errorf("handshake %s: %w", r.IP, err)
	}
	var ev struct {
		Event string `json:"event"`
	}
	_ = json.Unmarshal(msg, &ev)
	if ev.Event != "ms.channel.connect" {
		c.Close(websocket.StatusNormalClosure, "")
		return fmt.Errorf("handshake %s: unexpected %q", r.IP, strings.TrimSpace(string(msg)))
	}
	// We never need incoming messages; CloseRead keeps control frames flowing.
	c.CloseRead(context.Background())
	r.conn = c
	return nil
}

func keyMessage(key string) []byte {
	b, _ := json.Marshal(map[string]any{
		"method": "ms.remote.control",
		"params": map[string]string{
			"Cmd":          "Click",
			"DataOfCmd":    key,
			"Option":       "false",
			"TypeOfRemote": "SendRemoteKey",
		},
	})
	return b
}

// Key is one step of a key sequence: press Key, then wait Delay.
type Key struct {
	Key   string
	Delay time.Duration
}

// SendKeys sends the sequence over one connection, retrying the connection
// once if the cached socket turned out to be dead.
func (r *Remote) SendKeys(ctx context.Context, keys ...Key) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for i, k := range keys {
		if err := r.sendLocked(ctx, k.Key, i == 0); err != nil {
			return err
		}
		if k.Delay > 0 {
			select {
			case <-time.After(k.Delay):
			case <-ctx.Done():
				return ctx.Err()
			}
		}
	}
	return nil
}

func (r *Remote) sendLocked(ctx context.Context, key string, allowRetry bool) error {
	for attempt := 0; ; attempt++ {
		if err := r.connectLocked(ctx); err != nil {
			return err
		}
		wctx, cancel := context.WithTimeout(ctx, 3*time.Second)
		err := r.conn.Write(wctx, websocket.MessageText, keyMessage(key))
		cancel()
		if err == nil {
			return nil
		}
		r.closeLocked()
		if !allowRetry || attempt > 0 {
			return fmt.Errorf("send %s: %w", key, err)
		}
	}
}

// Press sends single keys with a short gap between them.
func (r *Remote) Press(ctx context.Context, keys ...string) error {
	seq := make([]Key, len(keys))
	for i, k := range keys {
		seq[i] = Key{Key: k, Delay: 80 * time.Millisecond}
	}
	return r.SendKeys(ctx, seq...)
}
