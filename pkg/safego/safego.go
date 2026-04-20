package safego

import (
	"fmt"
	"log/slog"
	"os"
	rdebug "runtime/debug"
	"sync/atomic"
	"time"
)

var (
	panicCount   atomic.Int64
	panicLogPath atomic.Pointer[string]
)

// SetPanicLogPath configures a file path where recovered panics are appended
// for post-mortem analysis. Typically called once from the application's main
// after the data directory is resolved, for example:
//
//	safego.SetPanicLogPath(appdir.File("panic.log"))
//
// If the path is empty or SetPanicLogPath is never called, panics are still
// logged via slog but no file is written. This keeps the safego package free
// of any dependency on the caller's directory-layout module and preserves a
// clean `pkg -> internal` layering boundary.
func SetPanicLogPath(path string) {
	p := path
	panicLogPath.Store(&p)
}

// Go launches a goroutine with panic recovery.
// If the goroutine panics, the panic is logged with full stack trace
// and appended to the configured panic log file (if any), but the process
// continues running.
func Go(name string, fn func()) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				panicCount.Add(1)
				stack := string(rdebug.Stack())
				slog.Error("goroutine panic recovered",
					"goroutine", name,
					"panic", r,
					"stack", stack)
				writePanicLog(name, r, stack)
			}
		}()
		fn()
	}()
}

// PanicCount returns the total number of recovered panics since process start.
func PanicCount() int64 {
	return panicCount.Load()
}

func writePanicLog(name string, r any, stack string) {
	p := panicLogPath.Load()
	if p == nil || *p == "" {
		return
	}
	f, err := os.OpenFile(*p, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return
	}
	defer f.Close()
	entry := fmt.Sprintf(
		"=== RECOVERED PANIC at %s ===\ngoroutine: %s\npanic: %v\n%s\n\n",
		time.Now().Format(time.RFC3339), name, r, stack,
	)
	f.WriteString(entry)
}
