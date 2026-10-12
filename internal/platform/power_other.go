//go:build !windows

package platform

import "errors"

// WatchPower is not implemented yet on this OS. On Linux it can be built on
// logind PrepareForSleep (D-Bus) and the desktop's screensaver/DPMS signals.
func WatchPower(h PowerHandler) error {
	return errors.New("display/sleep notifications are not implemented on this OS yet")
}
