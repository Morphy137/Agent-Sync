//go:build windows

package main

import (
	_ "embed"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"fyne.io/systray"
	"golang.org/x/sys/windows"
)

// trayIconData is the application icon (multi-size ICO) shown in the notification area.
//
//go:embed build/windows/icon.ico
var trayIconData []byte

const (
	// trayStopTimeout bounds how long Stop waits for the tray's message loop to end.
	trayStopTimeout = 2 * time.Second
	// Win32 window messages used to end the tray's message loop.
	messageClose = 0x0010
	messageQuit  = 0x0012
)

var (
	user32                = windows.NewLazySystemDLL("user32.dll")
	procEnumThreadWindows = user32.NewProc("EnumThreadWindows")
	procPostMessage       = user32.NewProc("PostMessageW")
	procPostThreadMessage = user32.NewProc("PostThreadMessageW")

	// closeWindowCallback posts WM_CLOSE to every window of a thread; it is created once because
	// Go never frees callbacks made with NewCallback.
	closeWindowCallback = windows.NewCallback(func(hwnd, _ uintptr) uintptr {
		procPostMessage.Call(hwnd, messageClose, 0, 0)
		return 1
	})
)

type trayItems struct {
	open, syncNow, quit *systray.MenuItem
}

// trayRun is one lifetime of the tray: from Start until its message loop ends.
type trayRun struct {
	threadID atomic.Uint32
	started  chan struct{} // closed once threadID is known
	readyCh  chan struct{} // closed when the icon and menu are in place
	ready    atomic.Bool
	done     chan struct{} // closed when the message loop ended
	stop     chan struct{} // closed to stop handling menu clicks
	items    atomic.Pointer[trayItems]
}

// trayIcon owns the notification-area icon. The icon exists only between Start and Stop, and can be
// started again later.
//
// fyne.io/systray creates its window and runs its message loop on the calling OS thread, so the
// whole tray runs on a goroutine locked to a thread of its own; Wails keeps the main thread.
// systray.Quit can only be used once per process, so Stop closes the tray window itself (the same
// WM_CLOSE the library handles) and the tray can start again.
type trayIcon struct {
	handlers trayHandlers

	mu     sync.Mutex
	run    *trayRun
	labels trayLabels
}

func newTrayIcon(handlers trayHandlers) *trayIcon {
	return &trayIcon{handlers: handlers}
}

// Start shows the tray icon, or only updates the menu texts when it is already shown.
func (t *trayIcon) Start(labels trayLabels) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.labels = labels
	if t.run != nil {
		t.applyLabelsLocked()
		return
	}
	run := &trayRun{
		started: make(chan struct{}),
		readyCh: make(chan struct{}),
		done:    make(chan struct{}),
		stop:    make(chan struct{}),
	}
	t.run = run
	go t.loop(run, labels)
}

func (t *trayIcon) loop(run *trayRun, labels trayLabels) {
	runtime.LockOSThread()
	defer close(run.done)
	run.threadID.Store(windows.GetCurrentThreadId())
	close(run.started)
	systray.Run(func() { t.onReady(run, labels) }, func() {})
}

func (t *trayIcon) onReady(run *trayRun, labels trayLabels) {
	systray.SetIcon(trayIconData)
	systray.SetTooltip(trayTooltip)
	systray.SetOnTapped(func() { go t.handlers.Open() })

	items := &trayItems{
		open:    systray.AddMenuItem(labels.Open, ""),
		syncNow: systray.AddMenuItem(labels.SyncNow, ""),
	}
	systray.AddSeparator()
	items.quit = systray.AddMenuItem(labels.Quit, "")
	run.items.Store(items)

	t.mu.Lock()
	t.applyLabelsLocked()
	t.mu.Unlock()

	run.ready.Store(true)
	close(run.readyCh)
	go t.dispatchClicks(run, items)
}

// dispatchClicks runs the handlers off the tray thread, so a slow action never freezes the menu.
func (t *trayIcon) dispatchClicks(run *trayRun, items *trayItems) {
	for {
		select {
		case <-items.open.ClickedCh:
			go t.handlers.Open()
		case <-items.syncNow.ClickedCh:
			go t.handlers.SyncNow()
		case <-items.quit.ClickedCh:
			go t.handlers.Quit()
		case <-run.stop:
			return
		}
	}
}

// SetLabels changes the menu texts of a running tray (and the ones used by the next Start).
func (t *trayIcon) SetLabels(labels trayLabels) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.labels = labels
	t.applyLabelsLocked()
}

func (t *trayIcon) applyLabelsLocked() {
	if t.run == nil {
		return
	}
	items := t.run.items.Load()
	if items == nil {
		return
	}
	items.open.SetTitle(t.labels.Open)
	items.syncNow.SetTitle(t.labels.SyncNow)
	items.quit.SetTitle(t.labels.Quit)
}

// Active reports whether the icon is on screen, so the window can safely hide instead of closing.
func (t *trayIcon) Active() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.run != nil && t.run.ready.Load()
}

// WaitReady waits until the icon is on screen; false when it is not started or does not appear in time.
func (t *trayIcon) WaitReady(timeout time.Duration) bool {
	t.mu.Lock()
	run := t.run
	t.mu.Unlock()
	if run == nil {
		return false
	}
	select {
	case <-run.readyCh:
		return true
	case <-time.After(timeout):
		return false
	}
}

// Stop removes the icon and waits for the tray thread to finish.
func (t *trayIcon) Stop() {
	t.mu.Lock()
	run := t.run
	t.run = nil
	t.mu.Unlock()
	if run == nil {
		return
	}
	close(run.stop)
	<-run.started
	threadID := uintptr(run.threadID.Load())

	procEnumThreadWindows.Call(threadID, closeWindowCallback, 0)
	select {
	case <-run.done:
		return
	case <-time.After(trayStopTimeout):
	}
	// The tray window never came up (so there was nothing to close): end the message loop directly.
	procPostThreadMessage.Call(threadID, messageQuit, 0, 0)
	select {
	case <-run.done:
	case <-time.After(trayStopTimeout):
	}
}

// systemUILanguage returns the preferred Windows display language, such as "es-ES".
func systemUILanguage() string {
	languages, err := windows.GetUserPreferredUILanguages(windows.MUI_LANGUAGE_NAME)
	if err != nil || len(languages) == 0 {
		return ""
	}
	return languages[0]
}
