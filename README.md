# TvTray

Иконка в трее Windows для телевизоров Samsung (Tizen), подключённых к ПК как монитор.

**Языки:** русский, English, Deutsch.

<p>
  <img src="docs/screenshots/panel.png" alt="Панель в трее" width="300">
  <img src="docs/screenshots/remote.png" alt="Пульт" width="190">
</p>

- **Питание вместе с ПК.** ТВ выключается, когда Windows гасит дисплей или уходит в сон, и включается через Wake-on-LAN, когда дисплей просыпается.
- **Панель по левому клику.** Для каждого ТВ: питание, громкость (UPnP — точное значение, или кнопками пульта), подсветка и кнопка полного пульта.
- **Подсветка.** У Samsung нет API для подсветки, поэтому TvTray проходит меню кнопками пульта. Последовательность для своей модели берётся из [пресетов](presets/) или набирается на мини-пульте в настройках.
- **Автоскан сети.** Находит все Samsung Tizen ТВ в локальной сети; можно управлять несколькими.
- **Один exe.** При первом запуске предлагает установиться в `Program Files\TvTray` (или работать без установки). Настройки хранятся в `TvTray.ini` рядом с exe.

## Установка

Скачайте `TvTray.exe` из [последнего релиза](https://github.com/angellfear/tvtray/releases/latest) и запустите. Установщик предложит язык, папку (по умолчанию `Program Files\TvTray`) и автозапуск.

<img src="docs/screenshots/installer.png" alt="Установщик" width="480">

## Настройки

Автоскан сети, флажки «вместе с ПК» и «в панели» для каждого ТВ, режим громкости, макрос подсветки с пресетами и мини-пультом для записи кнопок.

<img src="docs/screenshots/settings-backlight.png" alt="Настройки подсветки" width="560">

<details>
<summary>English / Deutsch</summary>

<img src="docs/screenshots/settings-en.png" alt="Settings (English)" width="560">
<img src="docs/screenshots/installer-de.png" alt="Installer (Deutsch)" width="480">
</details>

## Требования к ТВ

- Samsung на Tizen (2016+), в той же сети, что и ПК.
- Для включения по сети: «Общие → Сеть → Экспертные настройки → Включение с мобильного» = вкл. Лучше закрепить за ТВ IP в роутере.

Проверено на UE49KU6470 (2016) по Wi-Fi.

## Сборка

Нужны Go 1.26+ и GCC (MinGW) для CGO.

```powershell
.\build.ps1
```

Результат: `build\TvTray.exe`. Диагностика из консоли: `go run . --scan`.

Код Wails v3 (WebView2), всё ОС-зависимое — в `internal/platform`. Для Linux остаётся реализовать уведомления о дисплее/сне.

## Пресеты

См. [presets/README.md](presets/README.md). Новые модели приветствуются через pull request.
