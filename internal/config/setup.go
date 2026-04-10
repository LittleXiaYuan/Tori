package config

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// ──────────────────────────────────────────────
// Setup Wizard — first-run configuration assistant
//
// Provides:
//   - System environment detection (OS, GPU, Docker, etc.)
//   - LLM provider auto-detection and health check
//   - Local model recommendations
//   - Configuration file generation
//   - Quick-start templates
// ──────────────────────────────────────────────

// SetupResult contains the environment detection results.
type SetupResult struct {
	OS            string           `json:"os"`
	Arch          string           `json:"arch"`
	NumCPU        int              `json:"num_cpu"`
	HasDocker     bool             `json:"has_docker"`
	HasGPU        bool             `json:"has_gpu"`
	GPUInfo       string           `json:"gpu_info,omitempty"`
	HasOllama     bool             `json:"has_ollama"`
	OllamaModels  []string         `json:"ollama_models,omitempty"`
	HasPython     bool             `json:"has_python"`
	PythonVersion string           `json:"python_version,omitempty"`
	HasNode       bool             `json:"has_node"`
	NodeVersion   string           `json:"node_version,omitempty"`
	Providers     []ProviderStatus `json:"providers"`
	DataDir       string           `json:"data_dir"`
	ConfigExists  bool             `json:"config_exists"`
	FirstRun      bool             `json:"first_run"`
	Components    []EnvComponent   `json:"components"`
}

// EnvComponent describes an optional runtime component with install status.
type EnvComponent struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Category    string `json:"category"` // "office", "runtime", "channel", "ai"
	Installed   bool   `json:"installed"`
	Version     string `json:"version,omitempty"`
	Size        string `json:"size,omitempty"`     // estimated download size
	Installable bool   `json:"installable"`        // can we auto-install it?
	Required    bool   `json:"required,omitempty"` // is it required for core functionality?
}

// ProviderStatus describes the health of a configured LLM provider.
type ProviderStatus struct {
	Name      string `json:"name"` // "primary", "fast", "expert", "ollama", "vllm"
	BaseURL   string `json:"base_url"`
	Model     string `json:"model"`
	Available bool   `json:"available"` // connection successful
	Latency   string `json:"latency"`   // test response time
	Error     string `json:"error,omitempty"`
}

// DetectEnvironment performs a full system scan.
func DetectEnvironment(cfg Config) *SetupResult {
	result := &SetupResult{
		OS:     runtime.GOOS,
		Arch:   runtime.GOARCH,
		NumCPU: runtime.NumCPU(),
	}

	// Data directory
	result.DataDir = "data"
	if _, err := os.Stat("data"); os.IsNotExist(err) {
		result.FirstRun = true
	}

	// Config existence
	if _, err := os.Stat(filepath.Join("data", "config.json")); err == nil {
		result.ConfigExists = true
	}

	// Docker
	if out, err := exec.Command("docker", "version", "--format", "{{.Server.Version}}").Output(); err == nil {
		result.HasDocker = true
		_ = out
	}

	// GPU (NVIDIA)
	if out, err := exec.Command("nvidia-smi", "--query-gpu=name", "--format=csv,noheader").Output(); err == nil {
		result.HasGPU = true
		result.GPUInfo = strings.TrimSpace(string(out))
	}

	// Ollama
	if out, err := exec.Command("ollama", "list").Output(); err == nil {
		result.HasOllama = true
		lines := strings.Split(strings.TrimSpace(string(out)), "\n")
		for _, line := range lines[1:] { // skip header
			fields := strings.Fields(line)
			if len(fields) > 0 {
				result.OllamaModels = append(result.OllamaModels, fields[0])
			}
		}
	}

	// Python
	for _, pyCmd := range []string{"python3", "python"} {
		if out, err := exec.Command(pyCmd, "--version").Output(); err == nil {
			result.HasPython = true
			result.PythonVersion = strings.TrimSpace(string(out))
			break
		}
	}

	// Node.js
	if out, err := exec.Command("node", "--version").Output(); err == nil {
		result.HasNode = true
		result.NodeVersion = strings.TrimSpace(string(out))
	}

	// LLM Provider health checks
	result.Providers = checkProviders(cfg)

	// Optional components detection
	result.Components = detectComponents(result)

	return result
}

// detectComponents enumerates optional runtime components and their install status.
func detectComponents(env *SetupResult) []EnvComponent {
	components := []EnvComponent{
		{
			ID:          "python_office",
			Name:        "Python Office 引擎",
			Description: "高质量 Word/PPT 生成（python-pptx, python-docx）。未安装时使用 Go 基础引擎",
			Category:    "office",
			Installed:   false,
			Size:        "~30MB",
			Installable: true,
		},
		{
			ID:          "ollama",
			Name:        "Ollama 本地模型",
			Description: "运行本地 LLM 模型，离线可用，隐私安全",
			Category:    "ai",
			Installed:   env.HasOllama,
			Size:        "~1GB+",
			Installable: false,
		},
		{
			ID:          "docker",
			Name:        "Docker 沙箱",
			Description: "隔离代码执行环境，更安全地运行用户代码",
			Category:    "runtime",
			Installed:   env.HasDocker,
			Installable: false,
		},
		{
			ID:          "gpu",
			Name:        "GPU 加速",
			Description: "NVIDIA GPU 加速本地模型推理和 LoRA 微调",
			Category:    "ai",
			Installed:   env.HasGPU,
			Installable: false,
		},
	}

	// Check Python Office specifically
	if env.HasPython {
		hasPptx := checkPyImport("pptx")
		hasDocx := checkPyImport("docx")
		hasOpenpyxl := checkPyImport("openpyxl")
		if hasPptx && hasDocx && hasOpenpyxl {
			components[0].Installed = true
			components[0].Version = env.PythonVersion
		}
	}

	// Check for embedded Python
	embDir := filepath.Join("data", "python-embedded")
	if _, err := os.Stat(embDir); err == nil {
		components[0].Installed = true
		components[0].Version = "embedded"
	}

	return components
}

func checkPyImport(pkg string) bool {
	for _, pyCmd := range []string{"python3", "python"} {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		cmd := exec.CommandContext(ctx, pyCmd, "-c", fmt.Sprintf("import %s", pkg))
		err := cmd.Run()
		cancel()
		if err == nil {
			return true
		}
	}
	return false
}

// checkProviders tests each configured LLM provider.
func checkProviders(cfg Config) []ProviderStatus {
	var providers []ProviderStatus

	if cfg.LLMBaseURL != "" {
		providers = append(providers, TestProviderConnection(cfg.LLMBaseURL, cfg.LLMAPIKey, cfg.LLMModel, "primary"))
	}
	if cfg.LLMFastURL != "" {
		providers = append(providers, TestProviderConnection(cfg.LLMFastURL, cfg.LLMFastKey, cfg.LLMFastModel, "fast"))
	}
	if cfg.LLMExpertURL != "" {
		providers = append(providers, TestProviderConnection(cfg.LLMExpertURL, cfg.LLMExpertKey, cfg.LLMExpertModel, "expert"))
	}
	if cfg.OllamaBaseURL != "" {
		providers = append(providers, TestProviderConnection(cfg.OllamaBaseURL, "", cfg.OllamaModel, "ollama"))
	}
	if cfg.VLLMBaseURL != "" {
		providers = append(providers, TestProviderConnection(cfg.VLLMBaseURL, "", cfg.VLLMModel, "vllm"))
	}

	return providers
}

// TestProviderConnection tests a single LLM endpoint with GET /models.
// The optional name defaults to "custom" when omitted.
func TestProviderConnection(baseURL, apiKey, model string, name ...string) ProviderStatus {
	providerName := "custom"
	if len(name) > 0 && strings.TrimSpace(name[0]) != "" {
		providerName = name[0]
	}
	status := ProviderStatus{
		Name:    providerName,
		BaseURL: baseURL,
		Model:   model,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	url := strings.TrimRight(baseURL, "/") + "/models"
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		status.Error = err.Error()
		return status
	}
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}

	start := time.Now()
	resp, err := http.DefaultClient.Do(req)
	latency := time.Since(start)
	status.Latency = latency.Round(time.Millisecond).String()

	if err != nil {
		status.Error = err.Error()
		return status
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 400 {
		status.Available = true
	} else {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 256))
		status.Error = fmt.Sprintf("HTTP %d: %s", resp.StatusCode, string(body))
	}

	return status
}

// ──────────────────────────────────────────────
// Scenario Templates — quick-start presets
// ──────────────────────────────────────────────

// ScenarioTemplate is a pre-configured agent setup.
type ScenarioTemplate struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Category    string            `json:"category"`     // "personal" | "team" | "enterprise"
	EnvVars     map[string]string `json:"env_vars"`     // environment overrides
	Skills      []string          `json:"skills"`       // auto-enable these skills
	Channels    []string          `json:"channels"`     // auto-enable these channels
	SandboxTier string            `json:"sandbox_tier"` // "personal" | "family" | "public"
}

// BuiltinTemplates returns the list of quick-start scenarios.
func BuiltinTemplates() []ScenarioTemplate {
	return []ScenarioTemplate{
		{
			ID:          "personal-assistant",
			Name:        "个人助手",
			Description: "本机运行的全能AI助手，可执行Shell命令、读写文件、搜索知识库",
			Category:    "personal",
			EnvVars: map[string]string{
				"NATIVE_FC":     "true",
				"REACT_ENABLED": "true",
			},
			Skills:      []string{"exec_command", "file_read", "file_write", "web_search", "knowledge_query"},
			Channels:    []string{"http"},
			SandboxTier: "personal",
		},
		{
			ID:          "team-chat-bot",
			Name:        "团队聊天机器人",
			Description: "接入飞书/Telegram/Slack，支持群组对话、文件处理、定时任务",
			Category:    "team",
			EnvVars: map[string]string{
				"NATIVE_FC":    "true",
				"REFLECT_MODE": "learning",
			},
			Skills:      []string{"send_message", "file_read", "web_search", "knowledge_query", "docx_create"},
			Channels:    []string{"feishu", "telegram", "slack"},
			SandboxTier: "family",
		},
		{
			ID:          "code-review-agent",
			Name:        "代码审查Agent",
			Description: "对接VS Code扩展，自动Review代码变更、生成修改建议",
			Category:    "team",
			EnvVars: map[string]string{
				"NATIVE_FC":            "true",
				"LONG_HORIZON_ENABLED": "true",
			},
			Skills:      []string{"file_read", "file_write", "exec_command", "grep_search"},
			Channels:    []string{"http", "supervisor"},
			SandboxTier: "personal",
		},
		{
			ID:          "data-analyst",
			Name:        "数据分析助手",
			Description: "表格处理、Python执行、图表生成、报告输出",
			Category:    "personal",
			EnvVars: map[string]string{
				"NATIVE_FC":     "true",
				"REACT_ENABLED": "true",
			},
			Skills:      []string{"run_python", "file_read", "file_write", "docx_create", "chart_gen"},
			Channels:    []string{"http"},
			SandboxTier: "personal",
		},
		{
			ID:          "public-api-service",
			Name:        "公开API服务",
			Description: "面向多租户的安全Agent API，严格沙箱、审批系统、审计日志",
			Category:    "enterprise",
			EnvVars: map[string]string{
				"NATIVE_FC":    "true",
				"REFLECT_MODE": "strict",
			},
			Skills:      []string{"web_search", "knowledge_query", "http_request"},
			Channels:    []string{"http"},
			SandboxTier: "public",
		},
		{
			ID:          "workflow-automation",
			Name:        "工作流自动化",
			Description: "复杂多步任务自动编排、Cron调度、触发器驱动",
			Category:    "team",
			EnvVars: map[string]string{
				"NATIVE_FC":            "true",
				"LONG_HORIZON_ENABLED": "true",
				"REACT_ENABLED":        "true",
			},
			Skills:      []string{"exec_command", "file_read", "file_write", "http_request", "send_email", "run_workflow"},
			Channels:    []string{"http", "telegram"},
			SandboxTier: "family",
		},
	}
}

// ──────────────────────────────────────────────
// Config Generation
// ──────────────────────────────────────────────

// GenerateEnvFile creates a .env file from a template and user config.
func GenerateEnvFile(template ScenarioTemplate, apiKey, baseURL, model string) string {
	var sb strings.Builder
	sb.WriteString("# 云雀Agent配置文件 — 自动生成\n")
	sb.WriteString(fmt.Sprintf("# 场景模板: %s\n", template.Name))
	sb.WriteString(fmt.Sprintf("# 生成时间: %s\n\n", time.Now().Format("2006-01-02 15:04:05")))

	sb.WriteString("# LLM 配置\n")
	sb.WriteString(fmt.Sprintf("LLM_BASE_URL=%s\n", baseURL))
	sb.WriteString(fmt.Sprintf("LLM_API_KEY=%s\n", apiKey))
	sb.WriteString(fmt.Sprintf("LLM_MODEL=%s\n\n", model))

	sb.WriteString("# 场景特定配置\n")
	for k, v := range template.EnvVars {
		sb.WriteString(fmt.Sprintf("%s=%s\n", k, v))
	}
	sb.WriteString("\n")

	sb.WriteString("# 服务地址\n")
	sb.WriteString("AGENT_ADDR=:9090\n")

	return sb.String()
}

// SaveSetupResult persists the setup results for later reference.
func SaveSetupResult(result *SetupResult) error {
	if err := os.MkdirAll("data", 0o755); err != nil {
		return err
	}
	data, _ := json.MarshalIndent(result, "", "  ")
	return os.WriteFile(filepath.Join("data", "setup_result.json"), data, 0o644)
}
