// UI strings in Russian, English and German. The language comes from Go
// (ini setting or system language); a "lang" event reloads every page.
import { Events } from "/wails/runtime.js";

export const LANGUAGES = [
  { code: "ru", name: "Русский" },
  { code: "en", name: "English" },
  { code: "de", name: "Deutsch" },
];

const D = {
  // status
  "status.on": ["Включён", "On", "Eingeschaltet"],
  "status.off": ["Выключен", "Off", "Ausgeschaltet"],
  "status.offline": ["Выключен / не в сети", "Off / not on the network", "Aus / nicht im Netzwerk"],
  "state.on": ["включён", "on", "an"],
  "state.off": ["выключен", "off", "aus"],

  // panel
  "panel.turnOff": ["Выключить", "Turn off", "Ausschalten"],
  "panel.turnOn": ["Включить", "Turn on", "Einschalten"],
  "panel.mute": ["Без звука", "Mute", "Stumm"],
  "panel.backlight": ["Подсветка", "Backlight", "Hintergrundbeleuchtung"],
  "panel.backlightSetup": ["Подсветка: ", "Backlight: ", "Hintergrundbeleuchtung: "],
  "panel.setup": ["настроить", "set up", "einrichten"],
  "panel.remote": ["Пульт", "Remote", "Fernbedienung"],
  "panel.empty": ["Нет телевизоров для панели.", "No TVs to show.", "Keine Fernseher zum Anzeigen."],
  "panel.openSettings": ["Открыть настройки", "Open settings", "Einstellungen öffnen"],
  "panel.settings": ["Настройки", "Settings", "Einstellungen"],

  // remote
  "remote.title": ["Пульт", "Remote", "Fernbedienung"],
  "remote.power": ["Питание", "Power", "Ein/Aus"],
  "remote.hint": [
    "Клавиатура: стрелки, Enter = OK, Backspace = Назад, Esc = Exit, +/− громкость, M = mute, H = Home, цифры",
    "Keyboard: arrows, Enter = OK, Backspace = Back, Esc = Exit, +/− volume, M = mute, H = Home, digits",
    "Tastatur: Pfeile, Enter = OK, Rücktaste = Zurück, Esc = Exit, +/− Lautstärke, M = Stumm, H = Home, Ziffern",
  ],
  "key.back": ["↩ Назад", "↩ Back", "↩ Zurück"],

  // key sequence editor
  "ks.add": ["+ добавить кнопки", "+ add keys", "+ Tasten hinzufügen"],
  "ks.field": ["Нажмите, чтобы добавить кнопки", "Click to add keys", "Klicken, um Tasten hinzuzufügen"],
  "ks.remove": ["Удалить", "Remove", "Entfernen"],
  "ks.live": ["Сразу нажимать на ТВ", "Press on the TV right away", "Sofort am TV drücken"],
  "ks.clear": ["Очистить", "Clear", "Leeren"],
  "ks.done": ["Готово", "Done", "Fertig"],

  // settings: TVs
  "set.title": ["TvTray — настройки", "TvTray — settings", "TvTray — Einstellungen"],
  "set.tvs": ["Телевизоры", "TVs", "Fernseher"],
  "set.scan": ["Сканировать сеть", "Scan network", "Netzwerk durchsuchen"],
  "set.scanHint": ["Найдёт все Samsung Tizen ТВ в локальной сети (они должны быть включены).", "Finds all Samsung Tizen TVs on the local network (they must be on).", "Findet alle Samsung-Tizen-Fernseher im lokalen Netzwerk (sie müssen eingeschaltet sein)."],
  "set.scanning": ["Сканирую…", "Scanning…", "Suche läuft…"],
  "set.found": ["Найдено: {n}", "Found: {n}", "Gefunden: {n}"],
  "set.notFound": ["Ничего не найдено. ТВ должен быть включён и в той же сети.", "Nothing found. The TV must be on and on the same network.", "Nichts gefunden. Der Fernseher muss eingeschaltet und im selben Netzwerk sein."],
  "set.col.name": ["Имя", "Name", "Name"],
  "set.col.model": ["Модель", "Model", "Modell"],
  "set.col.net": ["Сеть", "Network", "Netzwerk"],
  "set.cable": ["кабель", "cable", "Kabel"],
  "set.added": ["добавлен", "added", "hinzugefügt"],
  "set.add": ["Добавить", "Add", "Hinzufügen"],
  "set.noTVs": ["Телевизоры не добавлены. Нажмите «Сканировать сеть».", "No TVs added yet. Click “Scan network”.", "Noch keine Fernseher. Klicken Sie auf „Netzwerk durchsuchen“."],
  "set.remove": ["Удалить", "Remove", "Entfernen"],
  "set.removeConfirm": ["Удалить «{name}» из списка?", "Remove “{name}” from the list?", "„{name}“ aus der Liste entfernen?"],
  "set.power": ["Питание", "Power", "Strom"],
  "set.linkPower": ["Выключать и включать вместе с дисплеем ПК", "Turn off and on together with the PC display", "Zusammen mit dem PC-Bildschirm aus- und einschalten"],
  "set.panel": ["Панель в трее", "Tray panel", "Tray-Panel"],
  "set.showInPanel": ["Показывать по левому клику", "Show on left click", "Bei Linksklick anzeigen"],
  "set.volume": ["Громкость", "Volume", "Lautstärke"],
  "set.volUpnp": ["UPnP — точное значение (рекомендуется)", "UPnP — exact value (recommended)", "UPnP — exakter Wert (empfohlen)"],
  "set.volKeys": ["Кнопки пульта VOL+/VOL−", "Remote keys VOL+/VOL−", "Fernbedienungstasten VOL+/VOL−"],

  // settings: backlight
  "bl.title": ["Подсветка (макрос пульта)", "Backlight (remote macro)", "Hintergrundbeleuchtung (Tastenmakro)"],
  "bl.help": [
    "Приложение открывает меню ТВ кнопками «Открыть», затем жмёт ◀/▶ на разницу между текущим и нужным значением и закрывает меню. Подберите «Открыть» так, чтобы после неё на экране был активен ползунок подсветки — проверьте кнопкой «Тест».",
    "The app opens the TV menu with the “Open” keys, presses ◀/▶ by the difference between the current and the wanted value, then closes the menu. Pick “Open” keys that leave the backlight slider active on screen — check with “Test”.",
    "Die App öffnet das TV-Menü mit den „Öffnen“-Tasten, drückt ◀/▶ um die Differenz zwischen aktuellem und gewünschtem Wert und schließt das Menü. Wählen Sie die „Öffnen“-Tasten so, dass danach der Regler der Hintergrundbeleuchtung aktiv ist — prüfen Sie es mit „Test“.",
  ],
  "bl.preset": ["Пресет", "Preset", "Preset"],
  "bl.choose": ["— выберите пресет —", "— choose a preset —", "— Preset wählen —"],
  "bl.none": ["— пресетов нет, обновите с GitHub —", "— no presets, update from GitHub —", "— keine Presets, von GitHub laden —"],
  "bl.forModel": ["✓ для {model}", "✓ for {model}", "✓ für {model}"],
  "bl.noMatch": ["для {model} пресета нет", "no preset for {model}", "kein Preset für {model}"],
  "bl.apply": ["Применить", "Apply", "Anwenden"],
  "bl.applied": ["Пресет применён", "Preset applied", "Preset angewendet"],
  "bl.update": ["Обновить с GitHub", "Update from GitHub", "Von GitHub aktualisieren"],
  "bl.updateTitle": ["Загрузить свежие пресеты из репозитория на GitHub", "Download the latest presets from the GitHub repository", "Neueste Presets aus dem GitHub-Repository laden"],
  "bl.downloaded": ["Загружено пресетов: {n}", "Presets downloaded: {n}", "Presets geladen: {n}"],
  "bl.enable": ["Включить", "Enable", "Aktivieren"],
  "bl.showSlider": ["Показывать ползунок подсветки в панели", "Show the backlight slider in the panel", "Regler im Panel anzeigen"],
  "bl.open": ["Кнопки «Открыть»", "“Open” keys", "„Öffnen“-Tasten"],
  "bl.close": ["Кнопки «Закрыть»", "“Close” keys", "„Schließen“-Tasten"],
  "bl.max": ["Максимум шкалы", "Scale maximum", "Skalenmaximum"],
  "bl.navDelay": ["Пауза после пунктов меню, мс", "Pause after menu keys, ms", "Pause nach Menütasten, ms"],
  "bl.stepDelay": ["Пауза между ◀/▶, мс", "Pause between ◀/▶, ms", "Pause zwischen ◀/▶, ms"],
  "bl.current": ["Текущее значение на ТВ", "Current value on the TV", "Aktueller Wert am TV"],
  "bl.currentHelp": ["−1 = неизвестно (при следующей установке сначала уйдёт в 0)", "−1 = unknown (the next change goes to 0 first)", "−1 = unbekannt (die nächste Änderung geht zuerst auf 0)"],
  "bl.check": ["Проверка", "Check", "Prüfen"],
  "bl.testOpen": ["Тест: открыть", "Test: open", "Test: öffnen"],
  "bl.testClose": ["Тест: закрыть", "Test: close", "Test: schließen"],
  "bl.sync": ["Синхронизировать…", "Sync…", "Synchronisieren…"],
  "bl.syncPrompt": ["Выставить подсветку (0–{max}) с синхронизацией через ноль:", "Set the backlight (0–{max}), syncing via zero:", "Hintergrundbeleuchtung setzen (0–{max}), Synchronisierung über null:"],
  "bl.sent": ["Макрос отправлен", "Macro sent", "Makro gesendet"],

  // settings: general
  "gen.title": ["Общие", "General", "Allgemein"],
  "gen.language": ["Язык", "Language", "Sprache"],
  "gen.system": ["Как в системе", "System default", "Systemsprache"],
  "gen.offDelay": ["Задержка выключения ТВ, с", "TV power-off delay, s", "Ausschaltverzögerung, s"],
  "gen.offDelayHelp": ["после того как система погасит дисплей", "after the system turns the display off", "nachdem das System den Bildschirm ausschaltet"],
  "gen.wake": ["Ждать включения ТВ, с", "Wait for the TV to wake, s", "Auf Einschalten warten, s"],
  "gen.wakeHelp": ["сколько слать Wake-on-LAN при пробуждении", "how long to send Wake-on-LAN on wake-up", "wie lange Wake-on-LAN beim Aufwachen gesendet wird"],
  "gen.autostart": ["Автозапуск", "Autostart", "Autostart"],
  "gen.autostartLabel": ["Запускать при входе в систему", "Start at login", "Bei der Anmeldung starten"],
  "gen.version": ["Версия", "Version", "Version"],
  "gen.ini": ["Файл настроек", "Settings file", "Einstellungsdatei"],
  "gen.uninstall": ["Удалить TvTray…", "Uninstall TvTray…", "TvTray deinstallieren…"],
  "gen.uninstallConfirm": ["Удалить TvTray? Будут удалены автозапуск, ярлык и папка программы вместе с настройками.", "Uninstall TvTray? Autostart, the shortcut and the program folder with its settings will be removed.", "TvTray deinstallieren? Autostart, Verknüpfung und Programmordner samt Einstellungen werden entfernt."],
  "gen.quit": ["Выйти из TvTray", "Quit TvTray", "TvTray beenden"],

  // installer
  "inst.title": ["Установка TvTray", "Install TvTray", "TvTray installieren"],
  "inst.heading": ["Установить TvTray?", "Install TvTray?", "TvTray installieren?"],
  "inst.text": [
    "Программа будет скопирована в выбранную папку, настройки будут храниться там же в TvTray.ini. Для системной папки понадобятся права администратора (один раз).",
    "The program will be copied to the chosen folder; settings are kept there in TvTray.ini. A system folder needs administrator rights (once).",
    "Das Programm wird in den gewählten Ordner kopiert; die Einstellungen liegen dort in TvTray.ini. Für einen Systemordner sind einmalig Administratorrechte nötig.",
  ],
  "inst.browse": ["Обзор…", "Browse…", "Durchsuchen…"],
  "inst.autostart": ["Запускать при входе в систему", "Start at login", "Bei der Anmeldung starten"],
  "inst.portable": ["Без установки", "Run without installing", "Ohne Installation"],
  "inst.portableTitle": ["Создать TvTray.ini рядом с этим файлом и работать отсюда", "Create TvTray.ini next to this file and run from here", "TvTray.ini neben dieser Datei anlegen und von hier ausführen"],
  "inst.cancel": ["Отмена", "Cancel", "Abbrechen"],
  "inst.install": ["Установить", "Install", "Installieren"],
};

const IDX = { ru: 0, en: 1, de: 2 };
let lang = "en";

export const getLang = () => lang;

/** Translates key, filling {placeholders} from vars. */
export function t(key, vars = {}) {
  const row = D[key];
  let s = row ? (row[IDX[lang]] ?? row[1]) : key;
  for (const [k, v] of Object.entries(vars)) s = s.replaceAll(`{${k}}`, v);
  return s;
}

/** Fills elements marked with data-i18n / data-i18n-title. */
export function applyStatic(root = document) {
  root.querySelectorAll("[data-i18n]").forEach(e => { e.textContent = t(e.dataset.i18n); });
  root.querySelectorAll("[data-i18n-title]").forEach(e => { e.title = t(e.dataset.i18nTitle); });
  document.documentElement.lang = lang;
}

/** Loads the language via getter (a Go call) and translates static text. */
export async function initI18n(getter, { reloadOnChange = true } = {}) {
  setLang(await getter().catch(() => "en"));
  if (reloadOnChange) Events.On("lang", () => location.reload());
}

export function setLang(code) {
  lang = IDX[code] !== undefined ? code : "en";
  applyStatic();
}
