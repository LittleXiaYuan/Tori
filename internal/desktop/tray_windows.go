package desktop

import (
	"log/slog"
	"os/exec"
	"sync"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	shell32           = windows.NewLazySystemDLL("shell32.dll")
	shellNotifyIcon   = shell32.NewProc("Shell_NotifyIconW")
	createPopupMenu   = user32.NewProc("CreatePopupMenu")
	appendMenuW       = user32.NewProc("AppendMenuW")
	trackPopupMenu    = user32.NewProc("TrackPopupMenu")
	destroyMenu       = user32.NewProc("DestroyMenu")
	postMessage       = user32.NewProc("PostMessageW")
	getCursorPos      = user32.NewProc("GetCursorPos")
	setForegroundWin  = user32.NewProc("SetForegroundWindow")
	registerClassExW  = user32.NewProc("RegisterClassExW")
	createWindowExW   = user32.NewProc("CreateWindowExW")
	defWindowProcW    = user32.NewProc("DefWindowProcW")
	getMessageW       = user32.NewProc("GetMessageW")
	translateMessage  = user32.NewProc("TranslateMessage")
	dispatchMessageW  = user32.NewProc("DispatchMessageW")
	loadIcon          = user32.NewProc("LoadIconW")
	postQuitMessage   = user32.NewProc("PostQuitMessage")
)

const (
	nimAdd    = 0x00000000
	nimDelete = 0x00000002
	nimModify = 0x00000001

	nifMessage = 0x00000001
	nifIcon    = 0x00000002
	nifTip     = 0x00000004

	wmApp          = 0x8000
	wmTrayCallback = wmApp + 1
	wmLButtonUp    = 0x0202
	wmRButtonUp    = 0x0205
	wmCommand      = 0x0111
	wmDestroy      = 0x0002

	mfString    = 0x00000000
	mfSeparator = 0x00000800

	tpmBottomAlign = 0x0020
	tpmLeftAlign   = 0x0000

	idiApplication = 32512

	menuOpenBrowser  = 1001
	menuToggleShell  = 1002
	menuQuit         = 1003
)

type notifyIconData struct {
	cbSize           uint32
	hWnd             uintptr
	uID              uint32
	uFlags           uint32
	uCallbackMessage uint32
	hIcon            uintptr
	szTip            [128]uint16
}

type point struct {
	x, y int32
}

type msg struct {
	hwnd    uintptr
	message uint32
	wParam  uintptr
	lParam  uintptr
	time    uint32
	pt      point
}

type wndClassEx struct {
	cbSize        uint32
	style         uint32
	lpfnWndProc   uintptr
	cbClsExtra    int32
	cbWndExtra    int32
	hInstance     uintptr
	hIcon         uintptr
	hCursor       uintptr
	hbrBackground uintptr
	lpszMenuName  *uint16
	lpszClassName *uint16
	hIconSm       uintptr
}

// TrayConfig configures the system tray behavior.
type TrayConfig struct {
	BrowserURL string
	OnQuit     func()
}

var (
	trayOnce   sync.Once
	trayHWND   uintptr
	trayCfg    TrayConfig
	trayNID    notifyIconData
)

// StartTray creates a system tray icon with context menu.
// Must be called from a dedicated goroutine (it runs a message pump).
func StartTray(cfg TrayConfig) {
	trayCfg = cfg

	className, _ := windows.UTF16PtrFromString("YunqueTrayClass")
	hInst, _, _ := kernel32.NewProc("GetModuleHandleW").Call(0)

	hIcon, _, _ := loadIcon.Call(0, uintptr(idiApplication))

	wc := wndClassEx{
		cbSize:        uint32(unsafe.Sizeof(wndClassEx{})),
		lpfnWndProc:   windows.NewCallback(trayWndProc),
		hInstance:     hInst,
		lpszClassName: className,
		hIcon:         hIcon,
		hIconSm:       hIcon,
	}
	registerClassExW.Call(uintptr(unsafe.Pointer(&wc)))

	windowName, _ := windows.UTF16PtrFromString("Yunque Agent Tray")
	hwnd, _, _ := createWindowExW.Call(
		0, uintptr(unsafe.Pointer(className)), uintptr(unsafe.Pointer(windowName)),
		0, 0, 0, 0, 0, 0, 0, hInst, 0,
	)
	trayHWND = hwnd

	tip := [128]uint16{}
	tipStr, _ := windows.UTF16FromString("云雀 Agent")
	copy(tip[:], tipStr)

	trayNID = notifyIconData{
		cbSize:           uint32(unsafe.Sizeof(notifyIconData{})),
		hWnd:             hwnd,
		uID:              1,
		uFlags:           nifMessage | nifIcon | nifTip,
		uCallbackMessage: wmTrayCallback,
		hIcon:            hIcon,
		szTip:            tip,
	}
	shellNotifyIcon.Call(nimAdd, uintptr(unsafe.Pointer(&trayNID)))

	slog.Info("system tray: started")

	var m msg
	for {
		ret, _, _ := getMessageW.Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0)
		if ret == 0 {
			break
		}
		translateMessage.Call(uintptr(unsafe.Pointer(&m)))
		dispatchMessageW.Call(uintptr(unsafe.Pointer(&m)))
	}

	shellNotifyIcon.Call(nimDelete, uintptr(unsafe.Pointer(&trayNID)))
	slog.Info("system tray: stopped")
}

// StopTray removes the tray icon and stops the message pump.
func StopTray() {
	if trayHWND != 0 {
		postMessage.Call(trayHWND, wmDestroy, 0, 0)
	}
}

func trayWndProc(hwnd uintptr, msg uint32, wParam, lParam uintptr) uintptr {
	switch msg {
	case wmTrayCallback:
		switch lParam {
		case wmLButtonUp:
			openBrowser()
		case wmRButtonUp:
			showTrayMenu(hwnd)
		}
		return 0

	case wmCommand:
		switch wParam {
		case menuOpenBrowser:
			openBrowser()
		case menuToggleShell:
			ToggleConsole()
		case menuQuit:
			if trayCfg.OnQuit != nil {
				trayCfg.OnQuit()
			}
			postQuitMessage.Call(0)
		}
		return 0

	case wmDestroy:
		postQuitMessage.Call(0)
		return 0
	}

	ret, _, _ := defWindowProcW.Call(hwnd, uintptr(msg), wParam, lParam)
	return ret
}

func showTrayMenu(hwnd uintptr) {
	hMenu, _, _ := createPopupMenu.Call()
	if hMenu == 0 {
		return
	}

	openStr, _ := windows.UTF16PtrFromString("打开云雀")
	shellLabel := "隐藏控制台"
	if IsConsoleHidden() {
		shellLabel = "显示控制台"
	}
	shellStr, _ := windows.UTF16PtrFromString(shellLabel)
	quitStr, _ := windows.UTF16PtrFromString("退出")

	appendMenuW.Call(hMenu, mfString, menuOpenBrowser, uintptr(unsafe.Pointer(openStr)))
	appendMenuW.Call(hMenu, mfString, menuToggleShell, uintptr(unsafe.Pointer(shellStr)))
	appendMenuW.Call(hMenu, mfSeparator, 0, 0)
	appendMenuW.Call(hMenu, mfString, menuQuit, uintptr(unsafe.Pointer(quitStr)))

	var pt point
	getCursorPos.Call(uintptr(unsafe.Pointer(&pt)))
	setForegroundWin.Call(hwnd)
	trackPopupMenu.Call(hMenu, tpmBottomAlign|tpmLeftAlign, uintptr(pt.x), uintptr(pt.y), 0, hwnd, 0)
	destroyMenu.Call(hMenu)
}

func openBrowser() {
	url := trayCfg.BrowserURL
	if url == "" {
		url = "http://localhost:9090"
	}
	exec.Command("cmd", "/c", "start", url).Start()
}
