package supervisor

import (
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"yunque-agent/internal/appdir"
)

const (
	initialBackoff = 2 * time.Second
	maxBackoff     = 60 * time.Second
	maxRestarts    = 5
	stableRunTime  = 10 * time.Second
)

// ShouldSupervise returns true when the current process should act as a
// supervisor (i.e. it was NOT launched with --supervised and the user has
// not opted out via YUNQUE_NO_SUPERVISOR).
func ShouldSupervise() bool {
	if os.Getenv("YUNQUE_NO_SUPERVISOR") == "true" {
		return false
	}
	for _, arg := range os.Args[1:] {
		if arg == "--supervised" {
			return false
		}
	}
	return true
}

// Run launches the agent as a supervised child process and restarts it on
// unexpected exits. It only returns when the child exits cleanly (code 0)
// or the restart budget is exhausted.
func Run() int {
	exe, err := os.Executable()
	if err != nil {
		slog.Error("supervisor: cannot resolve executable path", "err", err)
		return 1
	}
	exe, _ = filepath.EvalSymlinks(exe)

	backoff := initialBackoff
	consecutiveCrashes := 0

	for {
		args := append(os.Args[1:], "--supervised")
		cmd := exec.Command(exe, args...)
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Env = os.Environ()

		startTime := time.Now()
		slog.Info("supervisor: starting agent", "pid_parent", os.Getpid())

		if err := cmd.Run(); err == nil {
			slog.Info("supervisor: agent exited cleanly")
			return 0
		}

		exitCode := cmd.ProcessState.ExitCode()
		runDuration := time.Since(startTime)

		writeRestartLog(exitCode, runDuration, consecutiveCrashes+1)

		// Exit code 2 = config/port error; restarting won't help
		if exitCode == 2 {
			slog.Error("supervisor: agent exited with config error, not restarting",
				"exit_code", exitCode)
			return exitCode
		}

		if runDuration >= stableRunTime {
			backoff = initialBackoff
			consecutiveCrashes = 0
		} else {
			consecutiveCrashes++
		}

		if consecutiveCrashes >= maxRestarts {
			slog.Error("supervisor: too many rapid crashes, giving up",
				"crashes", consecutiveCrashes)
			writeRestartLog(-1, 0, consecutiveCrashes)
			return exitCode
		}

		slog.Warn("supervisor: agent crashed, restarting",
			"exit_code", exitCode,
			"ran_for", runDuration.Round(time.Millisecond),
			"backoff", backoff,
			"crash_count", consecutiveCrashes,
		)

		time.Sleep(backoff)
		backoff *= 2
		if backoff > maxBackoff {
			backoff = maxBackoff
		}
	}
}

func writeRestartLog(exitCode int, runDuration time.Duration, count int) {
	logPath := appdir.File("restart.log")
	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer f.Close()

	if exitCode == -1 {
		fmt.Fprintf(f, "[%s] GAVE UP after %d consecutive rapid crashes\n",
			time.Now().Format(time.RFC3339), count)
		return
	}

	fmt.Fprintf(f, "[%s] exit_code=%d ran=%s crash_count=%d — restarting\n",
		time.Now().Format(time.RFC3339), exitCode,
		runDuration.Round(time.Millisecond), count)
}
