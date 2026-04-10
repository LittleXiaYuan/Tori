//go:build !windows

package desktop

type TrayConfig struct {
	BrowserURL string
	OnQuit     func()
}

func StartTray(cfg TrayConfig) {}
func StopTray()                {}
