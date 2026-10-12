package platform

import "golang.org/x/sys/windows"

// SystemLanguage returns the Windows UI language as a BCP-47 tag (e.g. "de-DE").
func SystemLanguage() string {
	langs, err := windows.GetUserPreferredUILanguages(windows.MUI_LANGUAGE_NAME)
	if err != nil || len(langs) == 0 {
		return "en"
	}
	return langs[0]
}
