package sandbox

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// ──────────────────────────────────────────────
// Sandbox Configuration
// Loads from environment variables (priority) + JSON file (fallback).
// ──────────────────────────────────────────────

// SandboxConfig is the top-level sandbox configuration.
type SandboxConfig struct {
	BaseDir string       `json:"base_dir"` // working directory base (default: os.TempDir)
	Policy  Policy       `json:"policy"`   // process sandbox policy
	Docker  DockerConfig `json:"docker"`   // Docker runtime config
	Cloud   CloudConfig  `json:"cloud"`    // Cloud sandbox config (E2B-compatible)
}

// DockerConfig configures the Docker sandbox runtime.
type DockerConfig struct {
	Enabled        bool              `json:"enabled"`          // enable Docker sandbox
	DefaultImage   string            `json:"default_image"`    // fallback image (default: "python:3.12-slim")
	LanguageImages map[string]string `json:"language_images"`  // language -> image mapping
	AllowedImages  []string          `json:"allowed_images"`   // whitelist (empty = allow LanguageImages only)
	PoolSize       int               `json:"pool_size"`        // pre-warmed containers per image (default: 2)
	MaxContainers  int               `json:"max_containers"`   // max concurrent containers (default: 10)
	IdleTimeout    time.Duration     `json:"idle_timeout"`     // idle container cleanup (default: 5min)
	MemoryLimit    string            `json:"memory_limit"`     // e.g. "256m" (default: "256m")
	CPULimit       string            `json:"cpu_limit"`        // e.g. "1" (default: "1")
	PidsLimit      int               `json:"pids_limit"`       // fork bomb protection (default: 64)
	NetworkEnabled bool              `json:"network_enabled"`  // allow network (default: false)
	ReadOnlyRootfs bool              `json:"read_only_rootfs"` // mount rootfs read-only (default: true)
	NonRootUser    bool              `json:"non_root_user"`    // run as uid 1000 (default: true)
	Timeout        time.Duration     `json:"timeout"`          // execution timeout (default: 30s)
	MaxOutputBytes int               `json:"max_output_bytes"` // output limit (default: 64KB)
	BaseDir        string            `json:"base_dir"`         // host workdir base (default: os.TempDir)
}

// DefaultDockerConfig returns a secure default Docker configuration.
func DefaultDockerConfig() DockerConfig {
	return DockerConfig{
		Enabled:      false,
		DefaultImage: "python:3.12-slim",
		LanguageImages: map[string]string{
			"python":     "python:3.12-slim",
			"python3":    "python:3.12-slim",
			"javascript": "node:20-slim",
			"js":         "node:20-slim",
			"go":         "golang:1.22-alpine",
			"shell":      "alpine:latest",
			"bash":       "alpine:latest",
		},
		PoolSize:       2,
		MaxContainers:  10,
		IdleTimeout:    5 * time.Minute,
		MemoryLimit:    "256m",
		CPULimit:       "1",
		PidsLimit:      64,
		NetworkEnabled: false,
		ReadOnlyRootfs: true,
		NonRootUser:    true,
		Timeout:        30 * time.Second,
		MaxOutputBytes: 64 * 1024,
		BaseDir:        os.TempDir(),
	}
}

// DefaultSandboxConfig returns a config with process sandbox defaults.
func DefaultSandboxConfig() SandboxConfig {
	return SandboxConfig{
		BaseDir: os.TempDir(),
		Policy:  DefaultPolicy(),
		Docker:  DefaultDockerConfig(),
	}
}

// LoadConfig loads sandbox configuration from JSON file, then overlays environment variables.
// Environment variables always take priority.
func LoadConfig(jsonPath string) SandboxConfig {
	cfg := DefaultSandboxConfig()

	// 1. Load from JSON file (fallback)
	if jsonPath != "" {
		loadConfigFromFile(jsonPath, &cfg)
	} else {
		// Try default location
		loadConfigFromFile(filepath.Join("data", "sandbox.json"), &cfg)
	}

	// 2. Overlay environment variables (priority)
	loadConfigFromEnv(&cfg)

	return cfg
}

func loadConfigFromFile(path string, cfg *SandboxConfig) {
	data, err := os.ReadFile(path)
	if err != nil {
		return // File not found is fine, use defaults
	}
	// Parse JSON — partial decode merges with defaults
	_ = json.Unmarshal(data, cfg)
}

func loadConfigFromEnv(cfg *SandboxConfig) {
	if v := os.Getenv("SANDBOX_BASE_DIR"); v != "" {
		cfg.BaseDir = v
		cfg.Docker.BaseDir = v
	}

	// Docker settings
	if v := os.Getenv("SANDBOX_DOCKER_ENABLED"); v != "" {
		cfg.Docker.Enabled = parseBool(v)
	}
	if v := os.Getenv("SANDBOX_DOCKER_IMAGE"); v != "" {
		cfg.Docker.DefaultImage = v
	}
	if v := os.Getenv("SANDBOX_DOCKER_POOL_SIZE"); v != "" {
		cfg.Docker.PoolSize = parseInt(v, cfg.Docker.PoolSize)
	}
	if v := os.Getenv("SANDBOX_DOCKER_MAX_CONTAINERS"); v != "" {
		cfg.Docker.MaxContainers = parseInt(v, cfg.Docker.MaxContainers)
	}
	if v := os.Getenv("SANDBOX_DOCKER_IDLE_TIMEOUT"); v != "" {
		cfg.Docker.IdleTimeout = parseDuration(v, cfg.Docker.IdleTimeout)
	}
	if v := os.Getenv("SANDBOX_DOCKER_MEMORY"); v != "" {
		cfg.Docker.MemoryLimit = v
	}
	if v := os.Getenv("SANDBOX_DOCKER_CPU"); v != "" {
		cfg.Docker.CPULimit = v
	}
	if v := os.Getenv("SANDBOX_DOCKER_PIDS_LIMIT"); v != "" {
		cfg.Docker.PidsLimit = parseInt(v, cfg.Docker.PidsLimit)
	}
	if v := os.Getenv("SANDBOX_DOCKER_NETWORK"); v != "" {
		cfg.Docker.NetworkEnabled = parseBool(v)
	}
	if v := os.Getenv("SANDBOX_DOCKER_READONLY_ROOTFS"); v != "" {
		cfg.Docker.ReadOnlyRootfs = parseBool(v)
	}
	if v := os.Getenv("SANDBOX_DOCKER_NON_ROOT"); v != "" {
		cfg.Docker.NonRootUser = parseBool(v)
	}
	if v := os.Getenv("SANDBOX_DOCKER_TIMEOUT"); v != "" {
		cfg.Docker.Timeout = parseDuration(v, cfg.Docker.Timeout)
	}
	if v := os.Getenv("SANDBOX_DOCKER_MAX_OUTPUT"); v != "" {
		cfg.Docker.MaxOutputBytes = parseInt(v, cfg.Docker.MaxOutputBytes)
	}

	// Language images: SANDBOX_DOCKER_IMAGE_PYTHON=my-python:latest
	for _, lang := range []string{"python", "javascript", "go", "shell"} {
		envKey := "SANDBOX_DOCKER_IMAGE_" + strings.ToUpper(lang)
		if v := os.Getenv(envKey); v != "" {
			if cfg.Docker.LanguageImages == nil {
				cfg.Docker.LanguageImages = make(map[string]string)
			}
			cfg.Docker.LanguageImages[lang] = v
		}
	}

	// Allowed images whitelist: SANDBOX_DOCKER_ALLOWED_IMAGES=python:3.12-slim,node:20-slim
	if v := os.Getenv("SANDBOX_DOCKER_ALLOWED_IMAGES"); v != "" {
		cfg.Docker.AllowedImages = strings.Split(v, ",")
		for i := range cfg.Docker.AllowedImages {
			cfg.Docker.AllowedImages[i] = strings.TrimSpace(cfg.Docker.AllowedImages[i])
		}
	}

	// Process sandbox tier override
	if v := os.Getenv("SANDBOX_TIER"); v != "" {
		cfg.Policy = PolicyForTier(TierName(v))
	}

	// Cloud sandbox settings
	if v := os.Getenv("SANDBOX_CLOUD_ENABLED"); v != "" {
		cfg.Cloud.Enabled = parseBool(v)
	}
	if v := os.Getenv("SANDBOX_CLOUD_API_KEY"); v != "" {
		cfg.Cloud.APIKey = v
		cfg.Cloud.Enabled = true
	}
	if v := os.Getenv("SANDBOX_CLOUD_BASE_URL"); v != "" {
		cfg.Cloud.BaseURL = v
	}
	if v := os.Getenv("SANDBOX_CLOUD_TEMPLATE"); v != "" {
		cfg.Cloud.Template = v
	}
	if v := os.Getenv("SANDBOX_CLOUD_TIMEOUT"); v != "" {
		cfg.Cloud.Timeout = parseDuration(v, cfg.Cloud.Timeout)
	}

	if cfg.Cloud.APIKey == "" {
		if toriBase := strings.TrimSpace(os.Getenv("TORI_API_BASE_URL")); toriBase != "" {
			if llmKey := strings.TrimSpace(os.Getenv("LLM_API_KEY")); llmKey != "" {
				cfg.Cloud.APIKey = llmKey
				if cfg.Cloud.BaseURL == "" {
					trimmed := strings.TrimRight(toriBase, "/")
					if strings.HasSuffix(trimmed, "/v1") {
						cfg.Cloud.BaseURL = trimmed
					} else {
						cfg.Cloud.BaseURL = trimmed + "/v1"
					}
				}
				cfg.Cloud.Enabled = true
			}
		}
	}
}

func parseBool(s string) bool {
	s = strings.ToLower(strings.TrimSpace(s))
	return s == "true" || s == "1" || s == "yes"
}

func parseInt(s string, fallback int) int {
	n, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil {
		return fallback
	}
	return n
}

func parseDuration(s string, fallback time.Duration) time.Duration {
	d, err := time.ParseDuration(strings.TrimSpace(s))
	if err != nil {
		return fallback
	}
	return d
}
