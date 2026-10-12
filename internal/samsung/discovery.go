package samsung

import (
	"context"
	"net"
	"sort"
	"sync"
	"time"
)

// Found is a TV discovered on the LAN.
type Found struct {
	Info
	MAC string // MAC to use for Wake-on-LAN (from ARP, falls back to wifiMac)
}

// Scan probes port 8001 on every host of each local IPv4 network (up to /22)
// and queries /api/v2/ on the hosts that answer.
func Scan(ctx context.Context) []Found {
	var hosts []net.IP
	for _, n := range localNets() {
		ones, bits := n.Mask.Size()
		if bits != 32 {
			continue
		}
		if ones < 22 {
			ones = 22 // don't sweep huge networks
		}
		base := n.IP.Mask(net.CIDRMask(ones, 32)).To4()
		count := 1 << (32 - ones)
		for i := 1; i < count-1; i++ {
			ip := make(net.IP, 4)
			v := uint32(base[0])<<24 | uint32(base[1])<<16 | uint32(base[2])<<8 | uint32(base[3])
			v += uint32(i)
			ip[0], ip[1], ip[2], ip[3] = byte(v>>24), byte(v>>16), byte(v>>8), byte(v)
			hosts = append(hosts, ip)
		}
	}

	var (
		mu    sync.Mutex
		found []Found
		wg    sync.WaitGroup
		sem   = make(chan struct{}, 128)
	)
	for _, ip := range hosts {
		wg.Add(1)
		sem <- struct{}{}
		go func(ip string) {
			defer wg.Done()
			defer func() { <-sem }()
			d := net.Dialer{Timeout: 700 * time.Millisecond}
			c, err := d.DialContext(ctx, "tcp", net.JoinHostPort(ip, "8001"))
			if err != nil {
				return
			}
			c.Close()
			info, err := GetInfo(ctx, ip)
			if err != nil {
				return
			}
			info.IP = ip
			mu.Lock()
			found = append(found, Found{Info: *info})
			mu.Unlock()
		}(ip.String())
	}
	wg.Wait()

	arp := ARPTable()
	for i := range found {
		found[i].MAC = arp[found[i].IP]
		if found[i].MAC == "" {
			found[i].MAC = found[i].WifiMAC
		}
	}
	sort.Slice(found, func(i, j int) bool { return found[i].IP < found[j].IP })
	return found
}

// FindByMAC returns the current IP of a TV by its MAC, or "" if it is not
// found (e.g. powered off).
func FindByMAC(ctx context.Context, mac string) string {
	if ip := ARPTable().lookupMAC(mac); ip != "" {
		if info, err := GetInfo(ctx, ip); err == nil && info != nil {
			return ip
		}
	}
	for _, f := range Scan(ctx) {
		if f.MAC == mac || f.WifiMAC == mac {
			return f.IP
		}
	}
	return ""
}

// ARP maps IP -> MAC (normalized).
type ARP map[string]string

func (a ARP) lookupMAC(mac string) string {
	for ip, m := range a {
		if m == mac {
			return ip
		}
	}
	return ""
}
