package samsung

import (
	"bufio"
	"os"
	"strings"
)

// ARPTable reads the OS neighbour cache.
func ARPTable() ARP {
	t := ARP{}
	f, err := os.Open("/proc/net/arp")
	if err != nil {
		return t
	}
	defer f.Close()
	s := bufio.NewScanner(f)
	s.Scan() // header
	for s.Scan() {
		fields := strings.Fields(s.Text())
		if len(fields) >= 4 {
			if mac := NormalizeMAC(fields[3]); mac != "" && mac != "00:00:00:00:00:00" {
				t[fields[0]] = mac
			}
		}
	}
	return t
}
