package appdir

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
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
//  2. ./data next to the executable
//  3. ./data under the current working directory when the executable lives in
//     a `go run` build cache (Temp go-buildNNN dir) — anchoring data next to a
//     throwaway binary would scatter state into a different Temp dir on every
//     run and silently "lose" it
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
	if std := osStandardDataDir(); std != "" {
		return std
	}
	return filepath.Join(".", "data")
}

// osStandardDataDir returns the OS-standard per-user data directory for
// Yunque Agent. This is the same root Tauri uses for the desktop app, so all
// launch modes (desktop, go run, compiled binary, Docker with explicit
// YUNQUE_DATA_DIR) share one data store without manual configuration.
//
//   - Windows: %APPDATA%\com.yunque.agent\data
//   - macOS:   ~/Library/Application Support/com.yunque.agent/data
//   - Linux:   $XDG_DATA_HOME/com.yunque.agent/data  (fallback ~/.local/share)
func osStandardDataDir() string {
	if runtime.GOOS == "linux" {
		base := os.Getenv("XDG_DATA_HOME")
		if base == "" {
			if home, err := os.UserHomeDir(); err == nil {
				base = filepath.Join(home, ".local", "share")
			}
		}
		if base != "" {
			return filepath.Join(base, "com.yunque.agent", "data")
		}
	}
	if cfgDir, err := os.UserConfigDir(); err == nil {
		return filepath.Join(cfgDir, "com.yunque.agent", "data")
	}
	return ""
}

// isGoRunBinary reports whether the executable was produced by `go run`, whose
// binaries land in a per-build cache dir like
// %TEMP%\go-build123456789\b001\exe\agent.exe (or $GOTMPDIR/go-build…). Data
// must not be anchored there: the path changes on every build and the OS may
// purge it at any time.
func isGoRunBinary(exe string) bool {
	for _, seg := range strings.Split(filepath.ToSlash(exe), "/") {
		if strings.HasPrefix(seg, "go-build") {
			return true
		}
	}
	return false
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
