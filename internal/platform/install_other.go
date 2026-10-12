//go:build !windows

package platform

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
)

func DefaultInstallDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".local", "share", "tvtray")
}

func IsAccessDenied(err error) bool { return errors.Is(err, os.ErrPermission) }

func RunElevated(exe string, args ...string) (int, error) {
	cmd := exec.Command("pkexec", append([]string{exe}, args...)...)
	err := cmd.Run()
	if cmd.ProcessState != nil {
		return cmd.ProcessState.ExitCode(), nil
	}
	return -1, err
}

func GrantUsersWrite(dir string) error { return nil }

func CreateShortcut(target string) error { return nil } // TODO: .desktop file

func RemoveShortcut() {}

func StartDetached(exe string, args ...string) error {
	cmd := exec.Command(exe, args...)
	cmd.Dir = filepath.Dir(exe)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	return cmd.Start()
}

func RemoveDirAfterExit(dir string) error {
	return exec.Command("sh", "-c", "sleep 3; rm -rf '"+dir+"'").Start()
}

func KillOthers(path string) {
	exec.Command("pkill", "-f", "^"+path+"( |$)").Run()
}
