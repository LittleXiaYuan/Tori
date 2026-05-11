package config

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/joho/godotenv"
)

const envFileName = ".env"

// ResolveEnvFilePath returns the most useful .env file discovered from the
// current working directory. If YUNQUE_ENV_FILE is set, that file wins.
func ResolveEnvFilePath() string {
	if explicit := strings.TrimSpace(os.Getenv("YUNQUE_ENV_FILE")); explicit != "" {
		if info, err := os.Stat(explicit); err == nil && !info.IsDir() {
			return explicit
		}
	}
	if path := nearestEnvInWorkingTree(); path != "" {
		return path
	}
	return pickBestEnvFile(discoverEnvCandidates())
}

// LoadBestEnvFile loads the resolved .env file into the process environment.
func LoadBestEnvFile() (string, error) {
	if explicit := strings.TrimSpace(os.Getenv("YUNQUE_ENV_FILE")); explicit != "" {
		if err := godotenv.Load(explicit); err != nil {
			return "", err
		}
		_ = os.Setenv("YUNQUE_ENV_FILE", explicit)
		return explicit, nil
	}
	path := pickBestEnvFile(discoverEnvCandidates())
	if path == "" {
		return "", nil
	}
	if err := godotenv.Load(path); err != nil {
		return "", err
	}
	_ = os.Setenv("YUNQUE_ENV_FILE", path)
	return path, nil
}

// ReadEnvFile reads the resolved .env file, or returns an empty map if none
// is found.
func ReadEnvFile() map[string]string {
	path := ResolveEnvFilePath()
	if path == "" {
		return map[string]string{}
	}
	values, err := godotenv.Read(path)
	if err != nil {
		return map[string]string{}
	}
	return values
}

// WriteEnvFile writes the given values to the resolved .env file path.
// If no file exists yet, it falls back to the current working directory.
func WriteEnvFile(values map[string]string) error {
	path := ResolveEnvFilePath()
	if path == "" {
		path = envFileName
	}
	var lines []string
	lines = append(lines, "# ╔════════════════════════════════════╗")
	lines = append(lines, "# ║  云雀 Agent 配置文件               ║")
	lines = append(lines, "# ║  由 Settings 页面管理              ║")
	lines = append(lines, "# ╚════════════════════════════════════╝")
	lines = append(lines, "")

	written := map[string]bool{}
	for _, key := range orderedEnvKeys {
		v := values[key]
		if v == "" {
			lines = append(lines, "# "+key+"=")
		} else {
			lines = append(lines, key+"="+v)
		}
		written[key] = true
	}

	var extras []string
	for k := range values {
		if !written[k] && values[k] != "" {
			extras = append(extras, k)
		}
	}
	if len(extras) > 0 {
		sort.Strings(extras)
		lines = append(lines, "# ── Extra ──")
		for _, k := range extras {
			lines = append(lines, k+"="+values[k])
		}
		lines = append(lines, "")
	}

	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, []byte(strings.Join(lines, "\n")+"\n"), 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

var orderedEnvKeys = []string{
	"LLM_API_KEY",
	"LLM_BASE_URL",
	"LLM_MODEL",
	"AGENT_ADDR",
	"JWT_SECRET",
	"DATABASE_URL",
	"LLM_FAST_URL",
	"LLM_FAST_KEY",
	"LLM_FAST_MODEL",
	"LLM_EXPERT_URL",
	"LLM_EXPERT_KEY",
	"LLM_EXPERT_MODEL",
	"HEARTBEAT_ENABLED",
	"HEARTBEAT_INTERVAL",
	"SELF_ITERATE_ENABLED",
	"TELEGRAM_BOT_TOKEN",
	"FEISHU_APP_ID",
	"FEISHU_APP_SECRET",
	"DISCORD_BOT_TOKEN",
	"SLACK_BOT_TOKEN",
	"SLACK_SIGNING_SECRET",
	"EMBED_BASE_URL",
	"EMBED_MODEL",
	"EMBED_DIMS",
	"VECTOR_ANN_BACKEND",
	"VECTOR_HNSW_M",
	"VECTOR_HNSW_EF_CONSTRUCTION",
	"VECTOR_HNSW_EF_SEARCH",
	"VECTOR_IVF_CLUSTERS",
	"VECTOR_IVF_PROBE",
	"VECTOR_IVF_MIN_TRAIN",
	"RATE_LIMIT",
	"ALLOWED_ORIGINS",
	"PERSONA_DIR",
	"SEARXNG_URL",
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

func nearestEnvInWorkingTree() string {
	wd, err := os.Getwd()
	if err != nil || wd == "" {
		return ""
	}
	dir := wd
	for {
		path := filepath.Join(dir, envFileName)
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			return path
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return ""
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
