//go:build !windows && !linux

package platform

import "errors"

// WatchPower is not implemented on this OS.
func WatchPower(h PowerHandler) error {
	return errors.New("display/sleep notifications are not implemented on this OS yet")
}
