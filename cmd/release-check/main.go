package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
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

	// 7. No TODO/FIXME in critical paths.
	//
	// Cross-platform: walk the tree in Go instead of shelling out to
	// `grep`, which is not on PATH on a default Windows install. The
	// previous implementation always failed on Windows agents.
	check("TODOs", func() (bool, string) {
		count, scanErr := countTODOFiles(".")
		if scanErr != nil {
			return false, fmt.Sprintf("scan error: %v", scanErr)
		}
		if count > 0 {
			return false, fmt.Sprintf("⚠ %d files with TODO/FIXME/HACK markers", count)
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

// countTODOFiles walks `root` and returns how many *.go files contain at
// least one of the markers TODO / FIXME / HACK. Test files and vendored
// directories are skipped to keep release-blocking signal focused on
// production code.
func countTODOFiles(root string) (int, error) {
	count := 0
	skipDirs := map[string]bool{
		".git":         true,
		"vendor":       true,
		"node_modules": true,
		"dist":         true,
		"build":        true,
		"out":          true,
	}
	err := filepath.Walk(root, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return nil
		}
		if info.IsDir() {
			if skipDirs[info.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(info.Name(), ".go") || strings.HasSuffix(info.Name(), "_test.go") {
			return nil
		}
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return nil
		}
		s := string(data)
		if strings.Contains(s, "TODO") || strings.Contains(s, "FIXME") || strings.Contains(s, "HACK") {
			count++
		}
		return nil
	})
	return count, err
}
