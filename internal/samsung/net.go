package samsung

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strings"
	"syscall"
	"time"
)

// Alive reports whether the TV is powered on. Tested on a 2016 KU-series TV:
// port 8001 stops answering ~18 s after power-off and is back ~2 s after WoL.
func Alive(ip string) bool {
	c, err := net.DialTimeout("tcp", net.JoinHostPort(ip, "8001"), 800*time.Millisecond)
	if err != nil {
		return false
	}
	c.Close()
	return true
}

// Info is the subset of /api/v2/ we use.
type Info struct {
	ID      string
	Name    string
	Model   string
	IP      string
	Net     string // "wireless" or "wired"
	WifiMAC string
	// TokenAuth: the remote channel needs wss://:8002 and a pairing token.
	TokenAuth bool
}

func GetInfo(ctx context.Context, ip string) (*Info, error) {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, "http://"+ip+":8001/api/v2/", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var v struct {
		ID     string `json:"id"`
		Name   string `json:"name"`
		Device struct {
			ModelName   string `json:"modelName"`
			IP          string `json:"ip"`
			NetworkType string `json:"networkType"`
			WifiMac     string `json:"wifiMac"`
			TokenAuth   string `json:"TokenAuthSupport"`
		} `json:"device"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&v); err != nil {
		return nil, fmt.Errorf("%s: not a Samsung TV API: %w", ip, err)
	}
	if v.Device.ModelName == "" {
		return nil, fmt.Errorf("%s: not a Samsung TV", ip)
	}
	if v.Device.IP == "" {
		v.Device.IP = ip
	}
	return &Info{
		ID:        v.ID,
		Name:      v.Name,
		Model:     v.Device.ModelName,
		IP:        v.Device.IP,
		Net:       v.Device.NetworkType,
		WifiMAC:   NormalizeMAC(v.Device.WifiMac),
		TokenAuth: v.Device.TokenAuth == "true",
	}, nil
}

// NormalizeMAC returns AA:BB:CC:DD:EE:FF or "" if s is not a MAC address.
func NormalizeMAC(s string) string {
	hw, err := net.ParseMAC(strings.ReplaceAll(strings.TrimSpace(s), "-", ":"))
	if err != nil || len(hw) != 6 {
		return ""
	}
	return strings.ToUpper(hw.String())
}

// WakeOnLAN broadcasts a magic packet on every IPv4 interface.
func WakeOnLAN(mac string) error {
	hw, err := net.ParseMAC(mac)
	if err != nil {
		return err
	}
	pkt := make([]byte, 0, 102)
	for i := 0; i < 6; i++ {
		pkt = append(pkt, 0xFF)
	}
	for i := 0; i < 16; i++ {
		pkt = append(pkt, hw...)
	}

	targets := []string{"255.255.255.255"}
	for _, n := range localNets() {
		targets = append(targets, broadcastAddr(n).String())
	}
	// Linux refuses broadcast destinations unless SO_BROADCAST is set.
	lc := net.ListenConfig{Control: func(_, _ string, c syscall.RawConn) error {
		var serr error
		if err := c.Control(func(fd uintptr) { serr = setBroadcast(fd) }); err != nil {
			return err
		}
		return serr
	}}
	pc, err := lc.ListenPacket(context.Background(), "udp4", ":0")
	if err != nil {
		return err
	}
	defer pc.Close()

	var lastErr error
	sent := false
	for _, t := range targets {
		for _, port := range []int{9, 7} {
			if _, err := pc.WriteTo(pkt, &net.UDPAddr{IP: net.ParseIP(t), Port: port}); err != nil {
				lastErr = err
			} else {
				sent = true
			}
		}
	}
	if !sent {
		return lastErr
	}
	return nil
}

func localNets() []*net.IPNet {
	var out []*net.IPNet
	ifs, _ := net.Interfaces()
	for _, ifc := range ifs {
		if ifc.Flags&net.FlagUp == 0 || ifc.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, _ := ifc.Addrs()
		for _, a := range addrs {
			if n, ok := a.(*net.IPNet); ok && n.IP.To4() != nil && !n.IP.IsLinkLocalUnicast() {
				out = append(out, n)
			}
		}
	}
	return out
}

func broadcastAddr(n *net.IPNet) net.IP {
	ip := n.IP.To4()
	mask := net.IP(n.Mask).To4()
	if mask == nil {
		mask = net.IP(n.Mask[len(n.Mask)-4:])
	}
	b := make(net.IP, 4)
	for i := range b {
		b[i] = ip[i] | ^mask[i]
	}
	return b
}
