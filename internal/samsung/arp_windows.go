package samsung

import (
	"os/exec"
	"regexp"
	"syscall"
)

var arpLine = regexp.MustCompile(`(?m)^\s*(\d+\.\d+\.\d+\.\d+)\s+([0-9a-fA-F]{2}(?:-[0-9a-fA-F]{2}){5})\s`)

// ARPTable reads the OS neighbour cache.
func ARPTable() ARP {
	cmd := exec.Command("arp", "-a")
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000} // CREATE_NO_WINDOW
	out, err := cmd.Output()
	t := ARP{}
	if err != nil {
		return t
	}
	for _, m := range arpLine.FindAllStringSubmatch(string(out), -1) {
		if mac := NormalizeMAC(m[2]); mac != "" && mac != "FF:FF:FF:FF:FF:FF" {
			t[m[1]] = mac
		}
	}
	return t
}
