//go:build !windows

package platform

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"

	"tvtray/internal/icon"
)

// DefaultInstallDir is per-user, so no root is needed.
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

func desktopFile() string {
	dir := os.Getenv("XDG_DATA_HOME")
	if dir == "" {
		home, _ := os.UserHomeDir()
		dir = filepath.Join(home, ".local", "share")
	}
	return filepath.Join(dir, "applications", "tvtray.desktop")
}

// CreateShortcut adds the app to the desktop's application menu.
func CreateShortcut(target string) error {
	iconPath := filepath.Join(filepath.Dir(target), "tvtray.png")
	if err := os.WriteFile(iconPath, icon.PNG(256, true), 0o644); err != nil {
		return err
	}
	entry := fmt.Sprintf(`[Desktop Entry]
Type=Application
Name=TvTray
Comment=Samsung TV control from the tray
Exec="%s"
Icon=%s
Terminal=false
Categories=Utility;
`, target, iconPath)
	if err := os.MkdirAll(filepath.Dir(desktopFile()), 0o755); err != nil {
		return err
	}
	return os.WriteFile(desktopFile(), []byte(entry), 0o644)
}

func RemoveShortcut() { os.Remove(desktopFile()) }

func StartDetached(exe string, args ...string) error {
	cmd := exec.Command(exe, args...)
	cmd.Dir = filepath.Dir(exe)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	return cmd.Start()
}

func RemoveDirAfterExit(dir string) error {
	return exec.Command("sh", "-c", "sleep 3; rm -rf \"$0\"", dir).Start()
}

func KillOthers(path string) {
	exec.Command("pkill", "-f", "^"+path+"( |$)").Run()
}
