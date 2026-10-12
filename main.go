// TvTray: turns Samsung TVs off and on together with the PC display and
// controls volume/backlight from the system tray.
package main

import (
	"context"
	"embed"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"

	"tvtray/internal/config"
	"tvtray/internal/controller"
	"tvtray/internal/icon"
	"tvtray/internal/install"
	"tvtray/internal/platform"
	"tvtray/internal/samsung"
)

//go:embed frontend
var embedded embed.FS

func assets() application.AssetOptions {
	sub, _ := fs.Sub(embedded, "frontend")
	return application.AssetOptions{Handler: application.BundledAssetFileServer(sub)}
}

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case install.ElevatedFlag:
			os.Exit(install.RunElevatedPart(os.Args[2:]))
		case "--scan": // diagnostics: go run . --scan
			for _, f := range samsung.Scan(context.Background()) {
				fmt.Printf("%-15s %s  %s  %s (%s)\n", f.IP, f.MAC, f.Model, f.Name, f.Net)
			}
			return
		}
	}
	if !config.Installed() {
		runInstaller()
		return
	}
	runApp()
}

// ---- installer mode --------------------------------------------------------

func runInstaller() {
	svc := &InstallerService{}
	app := application.New(application.Options{
		Name:     "TvTray",
		Icon:     icon.PNG(256, true),
		Services: []application.Service{application.NewService(svc)},
		Assets:   assets(),
	})
	svc.app = app
	app.Window.NewWithOptions(application.WebviewWindowOptions{
		Name:                "installer",
		Title:               "Установка TvTray",
		Width:               560,
		Height:              360,
		DisableResize:       true,
		URL:                 "/install.html",
		MaximiseButtonState: application.ButtonHidden,
		MinimiseButtonState: application.ButtonHidden,
	})
	if err := app.Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// ---- tray app mode ---------------------------------------------------------

func openLog() io.Writer {
	path := filepath.Join(config.ExeDir(), "TvTray.log")
	if st, err := os.Stat(path); err == nil && st.Size() > 1<<20 {
		os.Rename(path, path+".old")
	}
	var w io.Writer = os.Stderr
	if f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644); err == nil {
		w = f
	}
	return w
}

// devtoolsArgs exposes WebView2 over CDP for debugging: TVTRAY_DEVTOOLS=9333.
func devtoolsArgs() []string {
	if port := os.Getenv("TVTRAY_DEVTOOLS"); port != "" {
		return []string{"--remote-debugging-port=" + port}
	}
	return nil
}

func runApp() {
	logw := openLog()
	log := slog.New(slog.NewTextHandler(logw, &slog.HandlerOptions{Level: slog.LevelInfo}))
	// Wails logs every asset request at Info; keep only its warnings.
	wailsLog := slog.New(slog.NewTextHandler(logw, &slog.HandlerOptions{Level: slog.LevelWarn}))
	store, err := config.Load(config.PathIn(config.ExeDir()))
	if err != nil {
		log.Error("config", "err", err)
	}
	ctrl := controller.New(store, log)
	svc := &TvService{store: store, ctrl: ctrl, log: log, remotes: map[string]*application.WebviewWindow{}}

	app := application.New(application.Options{
		Name:     "TvTray",
		Icon:     icon.PNG(256, true),
		Services: []application.Service{application.NewService(svc)},
		Assets:   assets(),
		Logger:   wailsLog,
		LogLevel: slog.LevelWarn,
		SingleInstance: &application.SingleInstanceOptions{
			UniqueID:               "tvtray.samsung.tray",
			OnSecondInstanceLaunch: func(application.SecondInstanceData) { svc.OpenSettings() },
		},
		Windows: application.WindowsOptions{DisableQuitOnLastWindowClosed: true, AdditionalBrowserArgs: devtoolsArgs()},
		Linux:   application.LinuxOptions{DisableQuitOnLastWindowClosed: true},
	})
	svc.app = app

	svc.panel = app.Window.NewWithOptions(application.WebviewWindowOptions{
		Name:            "panel",
		Width:           panelWidth,
		Height:          200,
		Frameless:       true,
		AlwaysOnTop:     true,
		Hidden:          true,
		DisableResize:   true,
		HideOnEscape:    true,
		HideOnFocusLost: true,
		URL:             "/panel.html",
		Windows:         application.WindowsWindow{HiddenOnTaskbar: true},
	})
	svc.panel.RegisterHook(events.Common.WindowClosing, func(e *application.WindowEvent) {
		svc.panel.Hide()
		e.Cancel()
	})

	tray := app.SystemTray.New()
	svc.tray = tray
	tray.SetIcon(icon.PNG(32, true))
	tray.SetTooltip("TvTray")
	menu := app.NewMenu()
	menu.Add("Настройки…").OnClick(func(*application.Context) { svc.OpenSettings() })
	menu.Add("Обновить состояние").OnClick(func(*application.Context) { ctrl.Refresh() })
	menu.AddSeparator()
	menu.Add("Выход").OnClick(func(*application.Context) { app.Quit() })
	tray.SetMenu(menu)
	tray.AttachWindow(svc.panel).WindowOffset(8)

	ctrl.OnChange = svc.emitState

	cfg := store.Get()
	svc.applyAutostart(cfg.Autostart)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go ctrl.Run(ctx)
	if err := platform.WatchPower(ctrl.HandlePower); err != nil {
		log.Error("power notifications unavailable", "err", err)
	}
	if len(cfg.TVs) == 0 {
		go func() {
			time.Sleep(500 * time.Millisecond)
			svc.OpenSettings()
		}()
	}
	log.Info("started", "ini", store.Path())
	if err := app.Run(); err != nil {
		log.Error("run", "err", err)
		os.Exit(1)
	}
}
