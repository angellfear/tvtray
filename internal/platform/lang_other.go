//go:build !windows

package platform

import "os"

// SystemLanguage returns the locale from the environment (e.g. "de_DE.UTF-8").
func SystemLanguage() string {
	for _, k := range []string{"LC_ALL", "LC_MESSAGES", "LANG"} {
		if v := os.Getenv(k); v != "" {
			return v
		}
	}
	return "en"
}
