//go:build !windows

package desktop

func SetAutoStart(enabled bool) error { return nil }
func IsAutoStartEnabled() bool        { return false }
