// Package install copies the running executable into an install directory
// and creates the ini there, which marks the copy as installed.
package install

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"tvtray/internal/config"
	"tvtray/internal/platform"
)

// ElevatedFlag is passed to an elevated copy of ourselves that only performs
// the privileged part of the install and exits.
const ElevatedFlag = "--install-elevated"

func ExeName() string {
	if runtime.GOOS == "windows" {
		return "TvTray.exe"
	}
	return "tvtray"
}

// Target returns the path the executable will have after install.
func Target(dir string) string { return filepath.Join(dir, ExeName()) }

// Files does the file-system part: copy exe, create ini, open the folder for
// writing by users. Needs admin rights for Program Files.
func Files(dir string, autostart bool) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	self, err := os.Executable()
	if err != nil {
		return err
	}
	target := Target(dir)
	if !samePath(self, target) {
		platform.KillOthers(target)
		if err := copyFile(self, target); err != nil {
			return err
		}
	}
	ini := config.PathIn(dir)
	if _, err := os.Stat(ini); err != nil {
		st, _ := config.Load(ini)
		st.Update(func(c *config.Config) { c.Autostart = autostart })
	}
	return platform.GrantUsersWrite(dir)
}

// Run installs into dir, elevating through UAC when the folder isn't
// writable, then adds the Start menu shortcut for the current user.
func Run(dir string, autostart bool) error {
	dir = filepath.Clean(dir)
	err := Files(dir, autostart)
	if err != nil && platform.IsAccessDenied(err) {
		self, _ := os.Executable()
		args := []string{ElevatedFlag, dir}
		if autostart {
			args = append(args, "--autostart")
		}
		code, eerr := platform.RunElevated(self, args...)
		switch {
		case eerr != nil:
			return eerr
		case code != 0:
			msg, _ := os.ReadFile(errorFile(dir))
			if len(msg) > 0 {
				return fmt.Errorf("%s", strings.TrimSpace(string(msg)))
			}
			return fmt.Errorf("установка с правами администратора завершилась с кодом %d", code)
		}
		err = nil
	}
	if err != nil {
		return err
	}
	if err := platform.CreateShortcut(Target(dir)); err != nil {
		return fmt.Errorf("программа установлена, но ярлык в «Пуске» не создан: %w", err)
	}
	return nil
}

// RunElevatedPart is the body of the elevated process.
func RunElevatedPart(args []string) int {
	if len(args) < 1 {
		return 2
	}
	dir := args[0]
	autostart := len(args) > 1 && args[1] == "--autostart"
	os.Remove(errorFile(dir))
	if err := Files(dir, autostart); err != nil {
		// The unelevated parent can't see our stderr; leave the reason in a file.
		os.WriteFile(errorFile(dir), []byte(err.Error()), 0o644)
		return 1
	}
	return 0
}

func errorFile(dir string) string {
	return filepath.Join(os.TempDir(), "tvtray-install-"+strings.NewReplacer(`\`, "_", "/", "_", ":", "_").Replace(dir)+".err")
}

// Uninstall removes the shortcut and the install folder (after exit).
func Uninstall() error {
	platform.RemoveShortcut()
	return platform.RemoveDirAfterExit(config.ExeDir())
}

func samePath(a, b string) bool {
	if runtime.GOOS == "windows" {
		return strings.EqualFold(filepath.Clean(a), filepath.Clean(b))
	}
	return filepath.Clean(a) == filepath.Clean(b)
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	tmp := dst + ".new"
	out, err := os.Create(tmp)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		os.Remove(tmp)
		return err
	}
	if err := out.Close(); err != nil {
		return err
	}
	os.Chmod(tmp, 0o755)
	if err := os.Rename(tmp, dst); err != nil {
		os.Remove(tmp)
		return err
	}
	return nil
}
