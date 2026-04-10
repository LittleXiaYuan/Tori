package desktop

import (
	"sync"

	"golang.org/x/sys/windows"
)

var (
	kernel32        = windows.NewLazySystemDLL("kernel32.dll")
	user32          = windows.NewLazySystemDLL("user32.dll")
	getConsoleWin   = kernel32.NewProc("GetConsoleWindow")
	showWindow      = user32.NewProc("ShowWindow")
	consoleHidden   bool
	consoleMu       sync.Mutex
)

const (
	swHide    = 0
	swShow    = 5
	swRestore = 9
)

// HideConsole hides the Windows console window.
func HideConsole() {
	consoleMu.Lock()
	defer consoleMu.Unlock()
	if hwnd := consoleHWND(); hwnd != 0 {
		showWindow.Call(hwnd, swHide)
		consoleHidden = true
	}
}

// ShowConsole shows the Windows console window.
func ShowConsole() {
	consoleMu.Lock()
	defer consoleMu.Unlock()
	if hwnd := consoleHWND(); hwnd != 0 {
		showWindow.Call(hwnd, swRestore)
		consoleHidden = false
	}
}

// ToggleConsole toggles the console visibility and returns the new state.
func ToggleConsole() (hidden bool) {
	consoleMu.Lock()
	defer consoleMu.Unlock()
	hwnd := consoleHWND()
	if hwnd == 0 {
		return false
	}
	if consoleHidden {
		showWindow.Call(hwnd, swRestore)
		consoleHidden = false
	} else {
		showWindow.Call(hwnd, swHide)
		consoleHidden = true
	}
	return consoleHidden
}

// IsConsoleHidden returns whether the console is currently hidden.
func IsConsoleHidden() bool {
	consoleMu.Lock()
	defer consoleMu.Unlock()
	return consoleHidden
}

func consoleHWND() uintptr {
	hwnd, _, _ := getConsoleWin.Call()
	return hwnd
}
