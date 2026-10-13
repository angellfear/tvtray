// Package samsung talks to Samsung Tizen TVs over the local network:
// the WebSocket remote-control channel, UPnP RenderingControl and Wake-on-LAN.
package samsung

import (
	"context"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/coder/websocket"

	"tvtray/internal/i18n"
)

// AppName is shown on the TV in the list of connected remote devices.
const AppName = "TvTray"

// Remote is a persistent connection to the TV's remote-control channel.
// It reconnects lazily: the first SendKeys after a failure opens a new socket.
// All methods are safe for concurrent use; key sequences never interleave.
//
// 2016 TVs accept ws://:8001 without pairing. 2017+ TVs close that channel
// and need wss://:8002: the first connection shows "Allow" on the TV and
// returns a token for later connections.
type Remote struct {
	IP     string
	Secure bool
	Token  string
	// OnAuth is called when the TV switches to the secure channel or issues
	// a new token, so the caller can persist them.
	OnAuth func(secure bool, token string)

	mu   sync.Mutex
	conn *websocket.Conn
}

// tlsClient accepts the TV's self-signed certificate.
var tlsClient = &http.Client{Transport: &http.Transport{
	TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
}}

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
	if r.Secure {
		return r.dialLocked(ctx, true)
	}
	err := r.dialLocked(ctx, false)
	if err == nil {
		return nil
	}
	// 2017+ TVs close the plain channel right away; try the secure one.
	if serr := r.dialLocked(ctx, true); serr == nil {
		return nil
	} else if !isRefused(serr) {
		return serr
	}
	return err
}

// isRefused: nothing listens on the port (TV off, or an old model without 8002).
func isRefused(err error) bool {
	s := strings.ToLower(err.Error())
	return strings.Contains(s, "refused") || strings.Contains(s, "actively")
}

func (r *Remote) dialLocked(ctx context.Context, secure bool) error {
	name := base64.StdEncoding.EncodeToString([]byte(AppName))
	u := fmt.Sprintf("ws://%s:8001/api/v2/channels/samsung.remote.control?name=%s", r.IP, url.QueryEscape(name))
	opts := &websocket.DialOptions{}
	if secure {
		u = fmt.Sprintf("wss://%s:8002/api/v2/channels/samsung.remote.control?name=%s", r.IP, url.QueryEscape(name))
		if r.Token != "" {
			u += "&token=" + url.QueryEscape(r.Token)
		}
		opts.HTTPClient = tlsClient
	}

	cctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	c, _, err := websocket.Dial(cctx, u, opts)
	if err != nil {
		return fmt.Errorf("connect %s: %w", r.IP, err)
	}
	// The TV answers with ms.channel.connect once the client is accepted;
	// a new client on a secure TV waits for "Allow" on screen (up to 30 s).
	rctx, rcancel := context.WithTimeout(ctx, 35*time.Second)
	defer rcancel()
	for {
		_, msg, err := c.Read(rctx)
		if err != nil {
			c.Close(websocket.StatusNormalClosure, "")
			if secure {
				// The pairing prompt was ignored or denied: the TV just closes.
				return i18n.Errorf("err.unauthorized")
			}
			return fmt.Errorf("handshake %s: %w", r.IP, err)
		}
		var ev struct {
			Event string `json:"event"`
			Data  struct {
				Token string `json:"token"`
			} `json:"data"`
		}
		_ = json.Unmarshal(msg, &ev)
		switch ev.Event {
		case "ms.channel.connect":
			changed := secure != r.Secure || (ev.Data.Token != "" && ev.Data.Token != r.Token)
			r.Secure = secure
			if ev.Data.Token != "" {
				r.Token = ev.Data.Token
			}
			if changed && r.OnAuth != nil {
				r.OnAuth(r.Secure, r.Token)
			}
			// We never need incoming messages; CloseRead keeps control frames flowing.
			c.CloseRead(context.Background())
			r.conn = c
			return nil
		case "ms.channel.unauthorized", "ms.channel.timeOut":
			c.Close(websocket.StatusNormalClosure, "")
			r.Token = ""
			return i18n.Errorf("err.unauthorized")
		case "ms.channel.clientConnect", "ms.channel.clientDisconnect", "ms.channel.ready":
			continue // other clients (e.g. Home Assistant) coming and going
		default:
			c.Close(websocket.StatusNormalClosure, "")
			return fmt.Errorf("handshake %s: unexpected %q", r.IP, strings.TrimSpace(string(msg)))
		}
	}
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
