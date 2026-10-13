// Package i18n holds the Go-side UI strings (tray menu, window titles,
// errors) in Russian, English and German. Frontend strings live in
// frontend/i18n.js.
package i18n

import (
	"fmt"
	"strings"
	"sync/atomic"
)

var Languages = []string{"ru", "en", "de"}

var current atomic.Value // string

func init() { current.Store("en") }

// Normalize maps "de-DE", "ru_RU.UTF-8" etc. to a supported code ("en" fallback).
func Normalize(lang string) string {
	l := strings.ToLower(lang)
	for _, s := range Languages {
		if strings.HasPrefix(l, s) {
			return s
		}
	}
	return "en"
}

func Set(lang string) { current.Store(Normalize(lang)) }
func Get() string     { return current.Load().(string) }

// T returns the translation of key in the current language, formatted with args.
func T(key string, args ...any) string {
	m := dict[key]
	s := m[Get()]
	if s == "" {
		s = m["en"]
	}
	if s == "" {
		s = key
	}
	if len(args) > 0 {
		return fmt.Sprintf(s, args...)
	}
	return s
}

// Errorf builds an error from a translated format (supports %w).
func Errorf(key string, args ...any) error {
	m := dict[key]
	s := m[Get()]
	if s == "" {
		s = m["en"]
	}
	return fmt.Errorf(s, args...)
}

type tr map[string]string

var dict = map[string]tr{
	"menu.settings": {"ru": "Настройки…", "en": "Settings…", "de": "Einstellungen…"},
	"menu.refresh":  {"ru": "Обновить состояние", "en": "Refresh status", "de": "Status aktualisieren"},
	"menu.quit":     {"ru": "Выход", "en": "Quit", "de": "Beenden"},

	"title.installer": {"ru": "Установка TvTray", "en": "Install TvTray", "de": "TvTray installieren"},
	"title.settings":  {"ru": "TvTray — настройки", "en": "TvTray — settings", "de": "TvTray — Einstellungen"},
	"title.remote":    {"ru": "Пульт — %s", "en": "Remote — %s", "de": "Fernbedienung — %s"},
	"title.folder":    {"ru": "Папка установки", "en": "Install folder", "de": "Installationsordner"},

	"err.tvNotFound":    {"ru": "ТВ %s не найден в настройках", "en": "TV %s is not in the settings", "de": "TV %s ist nicht in den Einstellungen"},
	"err.noWake":        {"ru": "%s не ответил на Wake-on-LAN", "en": "%s did not respond to Wake-on-LAN", "de": "%s hat nicht auf Wake-on-LAN reagiert"},
	"err.badMAC":        {"ru": "неверный MAC-адрес", "en": "invalid MAC address", "de": "ungültige MAC-Adresse"},
	"err.noPreset":      {"ru": "пресет не найден", "en": "preset not found", "de": "Preset nicht gefunden"},
	"err.presetsGitHub": {"ru": "не удалось загрузить пресеты с GitHub: %w", "en": "could not download presets from GitHub: %w", "de": "Presets konnten nicht von GitHub geladen werden: %w"},
	"err.noFolder":      {"ru": "укажите папку", "en": "choose a folder", "de": "Ordner auswählen"},
	"err.uacDenied":     {"ru": "запрос прав администратора отклонён", "en": "administrator rights request was declined", "de": "Administratorrechte wurden verweigert"},
	"err.elevatedCode":  {"ru": "установка с правами администратора завершилась с кодом %d", "en": "elevated install exited with code %d", "de": "Installation mit Administratorrechten endete mit Code %d"},
	"err.unauthorized":  {"ru": "ТВ не разрешил подключение — нажмите «Разрешить» на экране ТВ и повторите", "en": "the TV did not allow the connection — choose “Allow” on the TV screen and try again", "de": "der Fernseher hat die Verbindung nicht erlaubt — „Zulassen“ am TV wählen und erneut versuchen"},
	"err.shortcut":      {"ru": "программа установлена, но ярлык в «Пуске» не создан: %w", "en": "installed, but the Start menu shortcut could not be created: %w", "de": "installiert, aber die Startmenü-Verknüpfung konnte nicht erstellt werden: %w"},
}
