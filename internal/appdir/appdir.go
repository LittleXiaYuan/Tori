package appdir

import (
	"os"
	"path/filepath"
	"runtime"
	"sync"
)

var (
	once    sync.Once
	dataDir string
)

// DataDir returns the root data directory for Yunque Agent.
//
// Resolution order:
//  1. YUNQUE_DATA_DIR environment variable (explicit override)
//  2. Platform-specific user data directory:
//     - Windows: %AppData%\YunqueAgent
//     - macOS:   ~/Library/Application Support/YunqueAgent
//     - Linux:   ~/.local/share/yunque-agent
//  3. Fallback: ./data (relative to working directory)
//
// The directory is created if it doesn't exist.
func DataDir() string {
	once.Do(func() {
		dataDir = resolve()
		os.MkdirAll(dataDir, 0755)
	})
	return dataDir
}

// Sub returns a subdirectory under DataDir, creating it if needed.
// Example: appdir.Sub("sessions") → "%AppData%/YunqueAgent/sessions"
func Sub(parts ...string) string {
	p := filepath.Join(append([]string{DataDir()}, parts...)...)
	os.MkdirAll(p, 0755)
	return p
}

// File returns a file path under DataDir (parent dir is created).
// Example: appdir.File("panic.log") → "%AppData%/YunqueAgent/panic.log"
func File(parts ...string) string {
	p := filepath.Join(append([]string{DataDir()}, parts...)...)
	os.MkdirAll(filepath.Dir(p), 0755)
	return p
}

func resolve() string {
	if env := os.Getenv("YUNQUE_DATA_DIR"); env != "" {
		return filepath.Clean(env)
	}

	switch runtime.GOOS {
	case "windows":
		if appData := os.Getenv("APPDATA"); appData != "" {
			return filepath.Join(appData, "YunqueAgent")
		}
	case "darwin":
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, "Library", "Application Support", "YunqueAgent")
		}
	default: // linux, freebsd, etc.
		if xdg := os.Getenv("XDG_DATA_HOME"); xdg != "" {
			return filepath.Join(xdg, "yunque-agent")
		}
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, ".local", "share", "yunque-agent")
		}
	}

	return filepath.Join(".", "data")
}

// LegacyDataDir returns the old data directory path (./data relative to exe).
// Used for migration detection.
func LegacyDataDir() string {
	exe, err := os.Executable()
	if err != nil {
		return filepath.Join(".", "data")
	}
	return filepath.Join(filepath.Dir(exe), "data")
}

// IsUsingLegacy returns true if YUNQUE_DATA_DIR points to the legacy location,
// or if the resolved path equals the legacy path.
func IsUsingLegacy() bool {
	return filepath.Clean(DataDir()) == filepath.Clean(LegacyDataDir())
}
