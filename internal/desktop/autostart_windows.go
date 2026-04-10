package desktop

import (
	"os"

	"golang.org/x/sys/windows/registry"
)

const registryKey = `Software\Microsoft\Windows\CurrentVersion\Run`
const registryValue = "YunqueAgent"

// SetAutoStart enables or disables auto-start on Windows login.
func SetAutoStart(enabled bool) error {
	key, _, err := registry.CreateKey(registry.CURRENT_USER, registryKey, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer key.Close()

	if enabled {
		exe, err := os.Executable()
		if err != nil {
			return err
		}
		return key.SetStringValue(registryValue, `"`+exe+`" --background`)
	}
	return key.DeleteValue(registryValue)
}

// IsAutoStartEnabled checks if auto-start is currently enabled.
func IsAutoStartEnabled() bool {
	key, err := registry.OpenKey(registry.CURRENT_USER, registryKey, registry.QUERY_VALUE)
	if err != nil {
		return false
	}
	defer key.Close()
	_, _, err = key.GetStringValue(registryValue)
	return err == nil
}
