//go:build windows

package main

import (
	"sync/atomic"
	"syscall"
	"testing"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	// trayWindowClass is the window class fyne.io/systray registers for its message window.
	trayWindowClass = "SystrayClass"
	// trayTestTimeout bounds how long the test waits for the icon to appear or disappear.
	trayTestTimeout = 5 * time.Second
	trayTestPoll    = 20 * time.Millisecond
)

var (
	procEnumWindows              = user32.NewProc("EnumWindows")
	procGetClassName             = user32.NewProc("GetClassNameW")
	procGetWindowThreadProcessID = user32.NewProc("GetWindowThreadProcessId")
)

// countTrayWindows counts the tray message windows owned by this process.
func countTrayWindows() int {
	var count int
	callback := syscall.NewCallback(func(hwnd, _ uintptr) uintptr {
		var pid uint32
		procGetWindowThreadProcessID.Call(hwnd, uintptr(unsafe.Pointer(&pid)))
		if pid != windows.GetCurrentProcessId() {
			return 1
		}
		name := make([]uint16, len(trayWindowClass)+2)
		procGetClassName.Call(hwnd, uintptr(unsafe.Pointer(&name[0])), uintptr(len(name)))
		if windows.UTF16ToString(name) == trayWindowClass {
			count++
		}
		return 1
	})
	procEnumWindows.Call(callback, 0)
	return count
}

func waitForTrayWindows(want int) bool {
	deadline := time.Now().Add(trayTestTimeout)
	for time.Now().Before(deadline) {
		if countTrayWindows() == want {
			return true
		}
		time.Sleep(trayTestPoll)
	}
	return false
}

// TestTrayStartsStopsAndStartsAgain covers the live toggle of "close to tray": the icon must go away
// when stopped and come back later (fyne.io/systray alone can only quit once per process).
func TestTrayStartsStopsAndStartsAgain(t *testing.T) {
	var opened atomic.Int32
	tray := newTrayIcon(trayHandlers{Open: func() { opened.Add(1) }, SyncNow: func() {}, Quit: func() {}})
	labels := trayLabelsFor("en")

	if tray.Active() {
		t.Fatal("a new tray must not be active")
	}
	for cycle := 1; cycle <= 2; cycle++ {
		tray.Start(labels)
		if !tray.WaitReady(trayTestTimeout) {
			tray.Stop()
			t.Skip("the notification area is not available in this session")
		}
		if !tray.Active() || countTrayWindows() != 1 {
			t.Fatalf("cycle %d: tray not shown (active=%v windows=%d)", cycle, tray.Active(), countTrayWindows())
		}
		tray.SetLabels(trayLabelsFor("es"))
		tray.Stop()
		if tray.Active() {
			t.Fatalf("cycle %d: tray still active after Stop", cycle)
		}
		if !waitForTrayWindows(0) {
			t.Fatalf("cycle %d: the tray window was not closed by Stop", cycle)
		}
	}
	tray.Stop()
}
