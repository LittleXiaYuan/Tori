//go:build !windows

package desktop

func HideConsole()         {}
func ShowConsole()         {}
func ToggleConsole() bool  { return false }
func IsConsoleHidden() bool { return false }
