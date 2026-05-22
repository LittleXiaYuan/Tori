package selfheal

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"yunque-agent/pkg/jsonutil"
	"yunque-agent/pkg/plugin"
	"yunque-agent/pkg/skills"
)

// LLMFunc calls the LLM with system+user prompts.
type LLMFunc func(ctx context.Context, system, user string) (string, error)

// PromptInvalidator is called after hot-loading a plugin to refresh the
// planner's cached system prompt (e.g. planner.InvalidatePromptCache).
type PromptInvalidator func()

// Healer generates plugins automatically when the agent encounters tasks it cannot handle.
type Healer struct {
	pluginsDir string
	llm        LLMFunc
	pluginReg  *plugin.Registry
	skillReg   *skills.Registry
	invalidate PromptInvalidator
}

// New creates a self-healing plugin generator.
func New(pluginsDir string, llm LLMFunc) *Healer {
	if pluginsDir == "" {
		pluginsDir = "data/plugins"
	}
	return &Healer{pluginsDir: pluginsDir, llm: llm}
}

// SetRegistries wires the plugin and skill registries for hot-loading.
func (h *Healer) SetRegistries(pr *plugin.Registry, sr *skills.Registry, inv PromptInvalidator) {
	h.pluginReg = pr
	h.skillReg = sr
	h.invalidate = inv
}

// GeneratedPlugin contains the output of auto-generation.
type GeneratedPlugin struct {
	Name        string              `json:"name"`
	Description string              `json:"description"`
	Language    string              `json:"language"`
	HandlerCode string              `json:"handler_code"`
	SkillName   string              `json:"skill_name"`
	SkillDesc   string              `json:"skill_desc"`
	Params      map[string]ParamDef `json:"params"`
}

// ParamDef describes a parameter for the generated skill.
type ParamDef struct {
	Type        string `json:"type"`
	Description string `json:"description"`
}

const generatePrompt = `You are a plugin generator for an AI Agent platform.
The agent encountered a task it cannot handle. Generate a Python plugin to solve it.

Respond in this EXACT JSON format (no markdown, no extra text):
{
  "name": "plugin-name",
  "description": "What this plugin does",
  "language": "python",
  "skill_name": "skill_name",
  "skill_desc": "What the skill does",
  "params": {"param_name": {"type": "string", "description": "What this param is"}},
  "handler_code": "#!/usr/bin/env python3\nimport json, os\nargs = json.loads(os.environ.get('PLUGIN_ARGS', '{}'))\n# your code here\nprint(result)"
}

Rules:
- Plugin name must be lowercase with hyphens
- handler_code must be a complete, runnable Python script
- Read args from PLUGIN_ARGS env var (JSON)
- Print the result to stdout
- Keep it simple and focused
- Use only Python stdlib (no pip install)`

// Analyze determines if a failed task could be solved by a generated plugin.
// Returns a description of the capability gap, or empty string if not applicable.
func (h *Healer) Analyze(taskDescription string, errorMsg string) string {
	// Heuristics for tasks that could benefit from a plugin
	indicators := []string{
		"无法", "cannot", "unsupported", "not implemented",
		"no skill", "no tool", "不支持", "没有找到",
		"计算", "calculate", "convert", "转换",
		"格式", "format", "parse", "解析",
		"查询", "query", "fetch", "获取",
	}

	combined := strings.ToLower(taskDescription + " " + errorMsg)
	for _, ind := range indicators {
		if strings.Contains(combined, ind) {
			return fmt.Sprintf("Task '%s' failed with: %s", taskDescription, errorMsg)
		}
	}
	return ""
}

// ShouldHeal checks if a failed task could benefit from auto-generated plugin.
func (h *Healer) ShouldHeal(taskDescription, errorMsg string) bool {
	return h.Analyze(taskDescription, errorMsg) != ""
}

// Generate creates a plugin using LLM to handle the described task.
func (h *Healer) Generate(ctx context.Context, taskDescription string) (*GeneratedPlugin, error) {
	if h.llm == nil {
		return nil, fmt.Errorf("LLM function not configured")
	}

	userPrompt := fmt.Sprintf("The agent needs a plugin for this task:\n\n%s\n\nGenerate a Python plugin to handle this.", taskDescription)

	resp, err := h.llm(ctx, generatePrompt, userPrompt)
	if err != nil {
		return nil, fmt.Errorf("LLM generation failed: %w", err)
	}

	// Extract JSON from response (handle markdown code blocks)
	resp = jsonutil.Extract(resp)

	var plugin GeneratedPlugin
	if err := json.Unmarshal([]byte(resp), &plugin); err != nil {
		return nil, fmt.Errorf("parse LLM response: %w (response: %s)", err, resp[:min(len(resp), 200)])
	}

	if plugin.Name == "" || plugin.HandlerCode == "" {
		return nil, fmt.Errorf("incomplete plugin: name=%q, has_code=%v", plugin.Name, plugin.HandlerCode != "")
	}

	return &plugin, nil
}

// Install writes the generated plugin to disk so the Loader can pick it up.
func (h *Healer) Install(plugin *GeneratedPlugin) error {
	dir := filepath.Join(h.pluginsDir, plugin.Name)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	// Write manifest
	params := make(map[string]any, len(plugin.Params))
	for k, v := range plugin.Params {
		params[k] = map[string]any{"type": v.Type, "description": v.Description}
	}

	handlerFile := "handler.py"
	if plugin.Language == "node" {
		handlerFile = "handler.js"
	} else if plugin.Language == "shell" {
		handlerFile = "handler.sh"
	}

	manifest := map[string]any{
		"name":           plugin.Name,
		"description":    plugin.Description,
		"language":       orDefault(plugin.Language, "python"),
		"system_prompt":  fmt.Sprintf("You can use the '%s' tool: %s", plugin.SkillName, plugin.SkillDesc),
		"auto_generated": true,
		"generated_at":   time.Now().Format(time.RFC3339),
		"skills": []map[string]any{{
			"name":        plugin.SkillName,
			"description": plugin.SkillDesc,
			"handler":     handlerFile,
			"parameters":  params,
		}},
	}

	manifestJSON, _ := json.MarshalIndent(manifest, "", "  ")
	if err := os.WriteFile(filepath.Join(dir, "plugin.json"), manifestJSON, 0644); err != nil {
		return err
	}

	// Write handler
	if err := os.WriteFile(filepath.Join(dir, handlerFile), []byte(plugin.HandlerCode), 0644); err != nil {
		return err
	}

	slog.Info("self-heal: plugin installed", "name", plugin.Name, "skill", plugin.SkillName)
	return nil
}

// GenerateAndInstall is the one-call convenience method.
func (h *Healer) GenerateAndInstall(ctx context.Context, taskDescription string) (*GeneratedPlugin, error) {
	plugin, err := h.Generate(ctx, taskDescription)
	if err != nil {
		return nil, err
	}
	if err := h.Install(plugin); err != nil {
		return nil, fmt.Errorf("install: %w", err)
	}
	return plugin, nil
}

// ──────────────────────────────────────────────
// ValidatePlugin — static safety checks
// ──────────────────────────────────────────────

// ValidationError describes a specific safety violation.
type ValidationError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("%s: %s", e.Field, e.Message)
}

// dangerousPatterns lists code patterns that auto-generated plugins must not contain.
var dangerousPatterns = []struct {
	pattern string
	reason  string
}{
	{"os.Remove", "filesystem deletion"},
	{"os.RemoveAll", "recursive filesystem deletion"},
	{"shutil.rmtree", "recursive filesystem deletion"},
	{"subprocess.call", "arbitrary command execution"},
	{"subprocess.Popen", "arbitrary process spawn"},
	{"os.system", "arbitrary shell command"},
	{"exec(", "dynamic code execution"},
	{"eval(", "dynamic code evaluation"},
	{"__import__", "dynamic module import"},
	{"open(", "file I/O (use PLUGIN_ARGS instead)"},
	{"socket.", "raw network access"},
	{"http.server", "starts a server"},
}

// ValidatePlugin performs static safety checks on a generated plugin.
// Returns nil if safe, or a list of ValidationErrors.
func (h *Healer) ValidatePlugin(gp *GeneratedPlugin) []ValidationError {
	var errs []ValidationError

	if gp.Name == "" {
		errs = append(errs, ValidationError{Field: "name", Message: "plugin name is empty"})
	}
	if gp.HandlerCode == "" {
		errs = append(errs, ValidationError{Field: "handler_code", Message: "handler code is empty"})
	}
	if gp.SkillName == "" {
		errs = append(errs, ValidationError{Field: "skill_name", Message: "skill name is empty"})
	}

	// Check for dangerous patterns in generated code
	codeLower := strings.ToLower(gp.HandlerCode)
	for _, dp := range dangerousPatterns {
		if strings.Contains(codeLower, strings.ToLower(dp.pattern)) {
			errs = append(errs, ValidationError{
				Field:   "handler_code",
				Message: fmt.Sprintf("contains dangerous pattern %q (%s)", dp.pattern, dp.reason),
			})
		}
	}

	// Name format check
	for _, c := range gp.Name {
		if !((c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '-' || c == '_') {
			errs = append(errs, ValidationError{
				Field:   "name",
				Message: "plugin name must be lowercase alphanumeric with hyphens/underscores",
			})
			break
		}
	}

	return errs
}

// ──────────────────────────────────────────────
// HotLoad — register plugin + skills + refresh prompt cache
// ──────────────────────────────────────────────

// hotLoadedSkill wraps a GeneratedPlugin as a skills.Skill for registry injection.
type hotLoadedSkill struct {
	name   string
	desc   string
	params map[string]any
}

func (s *hotLoadedSkill) Name() string               { return s.name }
func (s *hotLoadedSkill) Description() string        { return s.desc }
func (s *hotLoadedSkill) Parameters() map[string]any { return s.params }
func (s *hotLoadedSkill) Execute(_ context.Context, _ map[string]any, _ *skills.Environment) (string, error) {
	return "", fmt.Errorf("hot-loaded skill %q requires external executor", s.name)
}

// HotLoad validates, installs, and live-registers a generated plugin into
// the running agent. On failure it rolls back (unregisters + removes files).
func (h *Healer) HotLoad(ctx context.Context, gp *GeneratedPlugin) error {
	// 1. Validate
	if errs := h.ValidatePlugin(gp); len(errs) > 0 {
		msgs := make([]string, len(errs))
		for i, e := range errs {
			msgs[i] = e.Error()
		}
		return fmt.Errorf("validation failed: %s", strings.Join(msgs, "; "))
	}

	// 2. Install to disk
	if err := h.Install(gp); err != nil {
		return fmt.Errorf("install: %w", err)
	}

	// 3. Register skill into live registries
	if h.skillReg != nil {
		params := make(map[string]any, len(gp.Params))
		for k, v := range gp.Params {
			params[k] = map[string]any{"type": v.Type, "description": v.Description}
		}
		h.skillReg.Register(&hotLoadedSkill{
			name:   gp.SkillName,
			desc:   gp.SkillDesc,
			params: params,
		})
	}

	// 4. Invalidate planner prompt cache so new skill appears in next turn
	if h.invalidate != nil {
		h.invalidate()
	}

	slog.Info("self-heal: hot-loaded plugin", "name", gp.Name, "skill", gp.SkillName)
	return nil
}

// Rollback removes a previously hot-loaded plugin from disk and live registries.
func (h *Healer) Rollback(pluginName, skillName string) {
	// Remove from registries
	if h.pluginReg != nil {
		h.pluginReg.Unregister(pluginName)
	}
	if h.skillReg != nil {
		// skills.Registry doesn't have Unregister, but we can re-register
		// a no-op placeholder. For now, log the intent.
		slog.Warn("self-heal: rollback skill (requires restart to fully remove)", "skill", skillName)
	}

	// Remove files
	dir := filepath.Join(h.pluginsDir, pluginName)
	if err := os.RemoveAll(dir); err != nil {
		slog.Error("self-heal: rollback file cleanup", "dir", dir, "err", err)
	} else {
		slog.Info("self-heal: rolled back plugin", "name", pluginName)
	}

	if h.invalidate != nil {
		h.invalidate()
	}
}

// ──────────────────────────────────────────────
// Extended trigger: capability-gap signals
// ──────────────────────────────────────────────

// CapabilityGap describes a missing capability detected during planning.
type CapabilityGap struct {
	RequestedSkill string `json:"requested_skill"`
	UserIntent     string `json:"user_intent"`
	Context        string `json:"context,omitempty"`
}

// ShouldHealGap checks if a capability gap should trigger auto-generation.
// This extends ShouldHeal from error-only to proactive gap detection.
func (h *Healer) ShouldHealGap(gap CapabilityGap) bool {
	if gap.RequestedSkill == "" && gap.UserIntent == "" {
		return false
	}
	return h.ShouldHeal(gap.UserIntent, "no skill: "+gap.RequestedSkill)
}

// HealGap generates and hot-loads a plugin to fill a capability gap.
func (h *Healer) HealGap(ctx context.Context, gap CapabilityGap) (*GeneratedPlugin, error) {
	desc := fmt.Sprintf("User intent: %s\nMissing skill: %s", gap.UserIntent, gap.RequestedSkill)
	if gap.Context != "" {
		desc += "\nContext: " + gap.Context
	}

	gp, err := h.Generate(ctx, desc)
	if err != nil {
		return nil, err
	}

	if err := h.HotLoad(ctx, gp); err != nil {
		// Rollback on failure
		h.Rollback(gp.Name, gp.SkillName)
		return nil, fmt.Errorf("hot-load failed, rolled back: %w", err)
	}

	return gp, nil
}

func orDefault(s, def string) string {
	if s == "" {
		return def
	}
	return s
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
