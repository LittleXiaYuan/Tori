package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"yunque-agent/internal/version"
)

func main() {
	fmt.Printf("Yunque Agent Release Checklist — %s\n\n", version.String())

	passed := 0
	failed := 0
	warned := 0

	check := func(name string, fn func() (bool, string)) {
		ok, msg := fn()
		if ok {
			fmt.Printf("  ✓ %s: %s\n", name, msg)
			passed++
		} else if msg != "" && strings.HasPrefix(msg, "⚠") {
			fmt.Printf("  ⚠ %s: %s\n", name, msg)
			warned++
		} else {
			fmt.Printf("  ✗ %s: %s\n", name, msg)
			failed++
		}
	}

	// 1. Version is not -dev
	check("Version", func() (bool, string) {
		v := version.Version
		if strings.Contains(v, "dev") {
			return false, fmt.Sprintf("%s (contains 'dev', set via -ldflags)", v)
		}
		return true, v
	})

	// 2. Git commit is set
	check("Git Commit", func() (bool, string) {
		if version.GitCommit == "unknown" || version.GitCommit == "" {
			return false, "not set (build with -ldflags)"
		}
		return true, version.GitCommit
	})

	// 3. Build date is set
	check("Build Date", func() (bool, string) {
		if version.BuildDate == "" {
			return false, "not set (build with -ldflags)"
		}
		return true, version.BuildDate
	})

	// 4. All tests pass
	check("Tests", func() (bool, string) {
		cmd := exec.Command("go", "test", "./...", "-count=1", "-timeout", "60s")
		out, err := cmd.CombinedOutput()
		if err != nil {
			lines := strings.Split(string(out), "\n")
			for _, l := range lines {
				if strings.Contains(l, "FAIL") {
					return false, l
				}
			}
			return false, err.Error()
		}
		// Count passed packages
		count := 0
		for _, l := range strings.Split(string(out), "\n") {
			if strings.HasPrefix(l, "ok") {
				count++
			}
		}
		return true, fmt.Sprintf("%d packages passed", count)
	})

	// 5. Build succeeds
	check("Build", func() (bool, string) {
		cmd := exec.Command("go", "build", "./...")
		out, err := cmd.CombinedOutput()
		if err != nil {
			return false, string(out)
		}
		return true, "compiled successfully"
	})

	// 6. Go vet passes
	check("Go Vet", func() (bool, string) {
		cmd := exec.Command("go", "vet", "./...")
		out, err := cmd.CombinedOutput()
		if err != nil {
			return false, strings.TrimSpace(string(out))
		}
		return true, "no issues"
	})

	// 7. No TODO/FIXME in critical paths
	check("TODOs", func() (bool, string) {
		cmd := exec.Command("grep", "-r", "--include=*.go", "-c", "TODO\\|FIXME\\|HACK", ".")
		out, _ := cmd.CombinedOutput()
		lines := strings.Split(strings.TrimSpace(string(out)), "\n")
		count := 0
		for _, l := range lines {
			if l != "" && !strings.HasSuffix(l, ":0") {
				count++
			}
		}
		if count > 0 {
			return false, fmt.Sprintf("⚠ %d files with TODO/FIXME markers", count)
		}
		return true, "clean"
	})

	// 8. Check .env not committed
	check("Secrets", func() (bool, string) {
		cmd := exec.Command("git", "ls-files", ".env")
		out, _ := cmd.CombinedOutput()
		if strings.TrimSpace(string(out)) != "" {
			return false, ".env is tracked by git!"
		}
		return true, ".env not in git"
	})

	// 9. Go mod tidy
	check("Go Mod", func() (bool, string) {
		cmd := exec.Command("go", "mod", "tidy")
		_, err := cmd.CombinedOutput()
		if err != nil {
			return false, err.Error()
		}
		// Check if go.sum changed
		cmd2 := exec.Command("git", "diff", "--name-only", "go.sum")
		out, _ := cmd2.CombinedOutput()
		if strings.TrimSpace(string(out)) != "" {
			return false, "go.sum changed after tidy — commit it"
		}
		return true, "dependencies clean"
	})

	fmt.Printf("\n─────────────────────────────────\n")
	fmt.Printf("Results: %d passed, %d warnings, %d failed\n", passed, warned, failed)
	if failed > 0 {
		fmt.Println("\n✗ Release NOT ready — fix failures above")
		os.Exit(1)
	} else if warned > 0 {
		fmt.Println("\n⚠ Release ready with warnings")
	} else {
		fmt.Println("\n✓ Release ready!")
	}
}
