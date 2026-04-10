package safego

import (
	"fmt"
	"log/slog"
	"os"
	rdebug "runtime/debug"
	"sync/atomic"
	"time"

	"yunque-agent/internal/appdir"
)

var panicCount atomic.Int64

// Go launches a goroutine with panic recovery.
// If the goroutine panics, the panic is logged with full stack trace
// and written to data/panic.log, but the process continues running.
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
	f, err := os.OpenFile(appdir.File("panic.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
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
