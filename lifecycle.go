package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"

	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/runtime"

	"agentsync/internal/apperr"
	"agentsync/internal/autostart"
	"agentsync/internal/config"
	"agentsync/internal/orchestrator"
)

const (
	// trayReadyTimeout is how long a hidden start waits for the tray icon before showing the window instead.
	trayReadyTimeout = 3 * time.Second
	// singleInstanceBaseID identifies AgentSync to the single-instance lock; the data folder is appended.
	singleInstanceBaseID = "7f3c9a52-1e0b-4d6a-9a47-agentsync"
	// singleInstanceDevSuffix keeps `wails dev` builds from colliding with an installed copy.
	singleInstanceDevSuffix = "-dev"
	// singleInstanceHashBytes is how many bytes of the data folder hash go into the lock id.
	singleInstanceHashBytes = 8
)

// launchOptions are the command-line switches of this run.
type launchOptions struct {
	// minimized is set by the Windows startup entry.
	minimized bool
}

func parseLaunchArgs(args []string) launchOptions {
	var launch launchOptions
	for _, arg := range args {
		if strings.EqualFold(arg, autostart.MinimizedFlag) {
			launch.minimized = true
		}
	}
	return launch
}

// windowStart decides how the window appears. A minimized start only hides the window when a tray
// icon will be there to bring it back; otherwise the window starts minimized in the taskbar.
func (l launchOptions) windowStart(closeToTray bool) (startHidden bool, state options.WindowStartState) {
	switch {
	case !l.minimized:
		return false, options.Normal
	case closeToTray:
		return true, options.Normal
	default:
		return false, options.Minimised
	}
}

// singleInstanceID is the lock identity of this run. It includes the data folder, so two copies that
// share data (installed and loose exe) are one instance, while portable copies, test sandboxes and
// development builds with their own data do not block each other.
func singleInstanceID(dataDir string, devBuild bool) string {
	sum := sha256.Sum256([]byte(strings.ToLower(filepath.Clean(dataDir))))
	id := singleInstanceBaseID + "-" + hex.EncodeToString(sum[:singleInstanceHashBytes])
	if devBuild {
		id += singleInstanceDevSuffix
	}
	return id
}

// closePolicy decides what closing the window does: hide it while a tray icon can bring it back,
// quit otherwise. Real quits (tray Quit, installing an update) raise the quitting flag first so
// Wails' OnBeforeClose, which every Quit goes through, lets them through.
type closePolicy struct {
	quitting   atomic.Bool
	trayActive func() bool
}

func (p *closePolicy) shouldHide() bool {
	return !p.quitting.Load() && p.trayActive != nil && p.trayActive()
}

// planHasWork reports whether the plan has something to apply (the same cases that enable Sync in the title bar).
func planHasWork(plan *orchestrator.SyncPlan) bool {
	if plan == nil {
		return false
	}
	switch plan.Direction {
	case orchestrator.DirectionDown, orchestrator.DirectionUp, orchestrator.DirectionBoth, orchestrator.DirectionConflict:
		return true
	}
	return plan.RulesPending > 0 || plan.RulesConflicts > 0
}

// beforeClose is Wails' OnBeforeClose: it returns true to cancel the close.
func (a *App) beforeClose(ctx context.Context) (prevent bool) {
	if !a.closing.shouldHide() {
		return false
	}
	runtime.WindowHide(ctx)
	return true
}

// closeToTrayConfigured reads the stored "close to tray" option before the window exists.
func (a *App) closeToTrayConfigured() bool {
	cfg, err := a.svc.GetConfig()
	return err == nil && cfg != nil && cfg.CloseToTray
}

// shutdown runs when the application exits.
func (a *App) shutdown(context.Context) {
	a.scheduler.Stop()
	a.tray.Stop()
}

// quit exits for real, even while closing the window would only hide it.
func (a *App) quit() {
	a.closing.quitting.Store(true)
	runtime.Quit(a.ctx)
}

// showWindow brings the window back from the tray or the taskbar.
func (a *App) showWindow() {
	if a.ctx == nil {
		return
	}
	runtime.WindowShow(a.ctx)
	runtime.WindowUnminimise(a.ctx)
}

// traySyncNow runs a remote check; when it finds something to apply the window opens on it, because
// applying may need a confirmation or a conflict decision.
func (a *App) traySyncNow() {
	a.scheduler.TriggerNow(true)
	plan, err := a.svc.GetStatus()
	if err != nil || !planHasWork(plan) {
		return
	}
	a.showWindow()
}

// applyTray shows the tray icon while "close to tray" is on, in the configured language.
func (a *App) applyTray(cfg *config.Config) {
	if cfg.CloseToTray {
		a.tray.Start(trayLabelsFor(cfg.Language))
		return
	}
	a.tray.Stop()
}

// startTrayAtLaunch starts the tray and, when the window began hidden, shows it if the icon never appears.
func (a *App) startTrayAtLaunch(cfg *config.Config) {
	a.applyTray(cfg)
	if !a.startedHidden {
		return
	}
	go func() {
		if !a.tray.WaitReady(trayReadyTimeout) {
			a.showWindow()
		}
	}()
}

// GetStartupStatus reports whether Windows really starts AgentSync at sign-in (read from the registry).
func (a *App) GetStartupStatus() autostart.Status {
	return a.startupEntry.Status()
}

// applyStartupSetting makes the startup entry match wanted and reports whether it changed anything.
func (a *App) applyStartupSetting(wanted bool) (changed bool, err error) {
	status := a.startupEntry.Status()
	if status.Enabled == wanted {
		return false, nil
	}
	if !status.Supported {
		if !wanted {
			return false, nil
		}
		return false, apperr.New(apperr.StartupUnsupported)
	}
	if err := a.startupEntry.Apply(wanted); err != nil {
		return false, apperr.Wrap(apperr.StartupFailed, err)
	}
	return true, nil
}

// reconcileStartup keeps the startup entry honest at launch: it follows the program when it moved
// and drops the stored wish when the entry was removed outside the app.
func (a *App) reconcileStartup(cfg *config.Config) {
	if !cfg.StartWithWindows || !a.startupEntry.Status().Supported {
		return
	}
	if !a.startupEntry.Status().Enabled {
		_ = a.svc.SetStartWithWindows(false)
		return
	}
	_, _ = a.startupEntry.Repair()
}

// executablePath is the running program, as written to the startup entry.
func executablePath() string {
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	if abs, err := filepath.Abs(exe); err == nil {
		return abs
	}
	return exe
}
