package main

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/joho/godotenv"
)

const envFileName = ".env"

// loadBestEnvFile loads the most useful .env file it can find.
//
// Precedence rules:
//  1. YUNQUE_ENV_FILE, if explicitly set.
//  2. The best candidate discovered by walking up from the working directory
//     and executable directory.
//
// "Best" means the file with the strongest non-placeholder LLM settings.
// This avoids dev-only placeholder files like apps/desktop/.env with values such
// as LLM_API_KEY=1 from overriding the real repo-root config.
func loadBestEnvFile() (string, error) {
	if override := strings.TrimSpace(os.Getenv("YUNQUE_ENV_FILE")); override != "" {
		if err := godotenv.Load(override); err != nil {
			return "", err
		}
		return override, nil
	}

	candidates := discoverEnvCandidates()
	best := pickBestEnvFile(candidates)
	if best == "" {
		return "", nil
	}
	if err := godotenv.Load(best); err != nil {
		return "", err
	}
	return best, nil
}

func discoverEnvCandidates() []string {
	seen := map[string]struct{}{}
	candidates := make([]string, 0, 8)
	addChain := func(start string) {
		if start == "" {
			return
		}
		dir := start
		for {
			candidate := filepath.Join(dir, envFileName)
			candidate = filepath.Clean(candidate)
			if _, ok := seen[candidate]; !ok {
				seen[candidate] = struct{}{}
				candidates = append(candidates, candidate)
			}
			parent := filepath.Dir(dir)
			if parent == dir {
				break
			}
			dir = parent
		}
	}

	if wd, err := os.Getwd(); err == nil {
		addChain(wd)
	}
	if exe, err := os.Executable(); err == nil {
		addChain(filepath.Dir(exe))
	}
	return candidates
}

func pickBestEnvFile(candidates []string) string {
	type scoredEnv struct {
		path  string
		score int
	}

	scored := make([]scoredEnv, 0, len(candidates))
	for _, path := range candidates {
		if info, err := os.Stat(path); err != nil || info.IsDir() {
			continue
		}
		values, err := godotenv.Read(path)
		if err != nil {
			continue
		}
		score := scoreEnv(values)
		if score <= 0 {
			continue
		}
		scored = append(scored, scoredEnv{path: path, score: score})
	}

	if len(scored) == 0 {
		return ""
	}
	sort.SliceStable(scored, func(i, j int) bool {
		if scored[i].score != scored[j].score {
			return scored[i].score > scored[j].score
		}
		return len(scored[i].path) > len(scored[j].path)
	})
	return scored[0].path
}

func scoreEnv(values map[string]string) int {
	score := 0
	if isUsefulEnvValue(values["LLM_BASE_URL"]) {
		score += 4
	}
	if isUsefulEnvValue(values["LLM_MODEL"]) {
		score += 4
	}
	if isUsefulEnvValue(values["LLM_API_KEY"]) {
		score += 2
	}
	if isUsefulEnvValue(values["JWT_SECRET"]) {
		score++
	}
	if isUsefulEnvValue(values["AGENT_ADDR"]) {
		score++
	}
	return score
}

func isUsefulEnvValue(v string) bool {
	v = strings.TrimSpace(v)
	if v == "" || v == "1" {
		return false
	}
	lower := strings.ToLower(v)
	switch {
	case strings.HasPrefix(lower, "your-"),
		strings.HasPrefix(lower, "change-me"),
		strings.HasPrefix(lower, "changeme"),
		strings.Contains(lower, "placeholder"),
		strings.Contains(lower, "todo"):
		return false
	default:
		return true
	}
}
