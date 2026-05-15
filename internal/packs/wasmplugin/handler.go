package wasmplugin

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"yunque-agent/internal/execution/sandbox"
	"yunque-agent/pkg/packruntime"
)

const PackID = "yunque.pack.wasm-plugin"

// Config describes runtime dependencies for the WASM plugin capability pack.
type Config struct {
	PluginDir string
	DataDir   string
	Sandbox   WasmExecutor
	Now       func() time.Time
}

// WasmExecutor is the narrow sandbox contract used by the pack shell.
type WasmExecutor interface {
	Execute(ctx context.Context, wasmBytes []byte, stdin string, entryPoint string) (*sandbox.WasmResult, error)
	Stats() map[string]any
}

// Handler owns WASM plugin pack routes and local metadata storage.
type Handler struct {
	pluginDir string
	dataDir   string
	sandbox   WasmExecutor
	now       func() time.Time
}

type PluginPermissionPolicy struct {
	LedgerKV       bool     `json:"ledger_kv"`
	MemorySearch   bool     `json:"memory_search"`
	HTTPFetch      bool     `json:"http_fetch"`
	AllowedHosts   []string `json:"allowed_hosts,omitempty"`
	EnvAllowlist   []string `json:"env_allowlist,omitempty"`
	MaxMemoryMB    int      `json:"max_memory_mb"`
	TimeoutSeconds int      `json:"timeout_seconds"`
}

type Plugin struct {
	Slug         string                 `json:"slug"`
	Name         string                 `json:"name"`
	Version      string                 `json:"version"`
	Description  string                 `json:"description,omitempty"`
	Runtime      string                 `json:"runtime"`
	Entrypoint   string                 `json:"entrypoint"`
	ModulePath   string                 `json:"module_path"`
	SHA256       string                 `json:"sha256,omitempty"`
	Status       string                 `json:"status"`
	LoadedAt     time.Time              `json:"loaded_at,omitempty"`
	ExecCount    int64                  `json:"exec_count"`
	LastExecAt   time.Time              `json:"last_exec_at,omitempty"`
	Permissions  PluginPermissionPolicy `json:"permissions"`
	Capabilities []string               `json:"capabilities,omitempty"`
	Tags         []string               `json:"tags,omitempty"`
}

type PluginSummary struct {
	Slug         string                 `json:"slug"`
	Name         string                 `json:"name"`
	Version      string                 `json:"version"`
	Description  string                 `json:"description,omitempty"`
	Runtime      string                 `json:"runtime"`
	Entrypoint   string                 `json:"entrypoint"`
	ModulePath   string                 `json:"module_path"`
	SHA256       string                 `json:"sha256,omitempty"`
	Status       string                 `json:"status"`
	LoadedAt     time.Time              `json:"loaded_at,omitempty"`
	ExecCount    int64                  `json:"exec_count"`
	LastExecAt   time.Time              `json:"last_exec_at,omitempty"`
	Permissions  PluginPermissionPolicy `json:"permissions"`
	Capabilities []string               `json:"capabilities,omitempty"`
}

type InstallRequest struct {
	Slug         string                 `json:"slug"`
	Name         string                 `json:"name"`
	Version      string                 `json:"version"`
	Description  string                 `json:"description"`
	ModulePath   string                 `json:"module_path"`
	Entrypoint   string                 `json:"entrypoint"`
	Permissions  PluginPermissionPolicy `json:"permissions"`
	Capabilities []string               `json:"capabilities"`
	Tags         []string               `json:"tags"`
	DryRun       bool                   `json:"dry_run"`
}

type ExecuteRequest struct {
	Slug       string `json:"slug"`
	Input      string `json:"input"`
	Entrypoint string `json:"entrypoint,omitempty"`
	DryRun     bool   `json:"dry_run"`
}

type ExecuteResult struct {
	Slug        string            `json:"slug"`
	DryRun      bool              `json:"dry_run"`
	Entrypoint  string            `json:"entrypoint"`
	Success     bool              `json:"success"`
	ExitCode    int               `json:"exit_code"`
	Stdout      string            `json:"stdout,omitempty"`
	Stderr      string            `json:"stderr,omitempty"`
	Duration    string            `json:"duration,omitempty"`
	MemUsed     uint32            `json:"mem_used_bytes,omitempty"`
	Exports     []string          `json:"exports,omitempty"`
	KVWrites    map[string]string `json:"kv_writes,omitempty"`
	Plan        []PermissionCheck `json:"plan,omitempty"`
	HostABIPlan HostABIPlan       `json:"host_abi_plan"`
	Notes       []string          `json:"notes,omitempty"`
}

type PermissionCheck struct {
	Name    string `json:"name"`
	Allowed bool   `json:"allowed"`
	Reason  string `json:"reason,omitempty"`
}

type HostABIPlan struct {
	PlanReady        bool                  `json:"plan_ready"`
	Ready            bool                  `json:"ready"`
	Status           string                `json:"status"`
	EnforcementReady bool                  `json:"enforcement_ready"`
	WritesFiles      bool                  `json:"writes_files"`
	NetworkAccess    bool                  `json:"network_access"`
	Functions        []HostABIFunctionPlan `json:"functions"`
	Summary          HostABISummary        `json:"summary"`
	ResourceLimits   HostABIResourceLimits `json:"resource_limits"`
	Labels           []string              `json:"labels"`
	Notes            []string              `json:"notes,omitempty"`
}

type HostABIFunctionPlan struct {
	Name             string   `json:"name"`
	Category         string   `json:"category"`
	Permission       string   `json:"permission"`
	Enabled          bool     `json:"enabled"`
	EnforcementReady bool     `json:"enforcement_ready"`
	WritesFiles      bool     `json:"writes_files"`
	NetworkAccess    bool     `json:"network_access"`
	Constraints      []string `json:"constraints,omitempty"`
	Reason           string   `json:"reason,omitempty"`
}

type HostABISummary struct {
	FunctionCount     int  `json:"function_count"`
	EnabledCount      int  `json:"enabled_count"`
	LedgerKV          bool `json:"ledger_kv"`
	MemorySearch      bool `json:"memory_search"`
	HTTPFetch         bool `json:"http_fetch"`
	EnvGet            bool `json:"env_get"`
	AllowedHostCount  int  `json:"allowed_host_count"`
	EnvAllowlistCount int  `json:"env_allowlist_count"`
}

type HostABIResourceLimits struct {
	MaxMemoryMB    int      `json:"max_memory_mb"`
	TimeoutSeconds int      `json:"timeout_seconds"`
	AllowedHosts   []string `json:"allowed_hosts"`
	EnvAllowlist   []string `json:"env_allowlist"`
}

var safeSlugRe = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,79}$`)
var windowsVolumeRe = regexp.MustCompile(`^[A-Za-z]:`)

// New creates a WASM plugin pack handler.
func New(cfg Config) *Handler {
	pluginDir := strings.TrimSpace(cfg.PluginDir)
	if pluginDir == "" {
		pluginDir = filepath.Join(".", "data", "plugins")
	}
	dataDir := strings.TrimSpace(cfg.DataDir)
	if dataDir == "" {
		dataDir = filepath.Join(".", "data", "wasm-plugin")
	}
	now := cfg.Now
	if now == nil {
		now = time.Now
	}
	exec := cfg.Sandbox
	if exec == nil {
		exec = sandbox.NewWasmSandbox(sandbox.WasmConfig{MemoryLimitPages: 1024, MaxDuration: 30 * time.Second, MaxOutputBytes: 128 * 1024})
	}
	return &Handler{pluginDir: pluginDir, dataDir: dataDir, sandbox: exec, now: now}
}

func DefaultHandler() *Handler { return New(Config{}) }

func (h *Handler) PackID() string { return PackID }

func (h *Handler) Routes() []packruntime.BackendRoute {
	return []packruntime.BackendRoute{
		{Method: http.MethodGet, Path: "/v1/wasm-plugin/status", Handler: h.Status},
		{Methods: []string{http.MethodGet, http.MethodPost}, Path: "/v1/wasm-plugin/plugins", Handler: h.Plugins},
		{Method: http.MethodGet, Path: "/v1/wasm-plugin/plugins/", Handler: h.PluginDetail},
		{Method: http.MethodPost, Path: "/v1/wasm-plugin/plugins/load", Handler: h.Load},
		{Method: http.MethodPost, Path: "/v1/wasm-plugin/plugins/unload", Handler: h.Unload},
		{Method: http.MethodPost, Path: "/v1/wasm-plugin/execute", Handler: h.Execute},
		{Method: http.MethodGet, Path: "/v1/wasm-plugin/evidence/", Handler: h.Evidence},
	}
}

func (h *Handler) Status(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	plugins, err := h.listPlugins()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	loaded := 0
	for _, plugin := range plugins {
		if plugin.Status == "loaded" {
			loaded++
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"pack_id":        PackID,
		"stage":          "pack-shell-before-runtime-hosts",
		"runtime_ready":  true,
		"abi_plan_ready": true,
		"abi_ready":      false,
		"plugin_count":   len(plugins),
		"loaded_count":   loaded,
		"plugin_dir":     h.pluginDir,
		"store_dir":      h.dataDir,
		"sandbox":        h.sandbox.Stats(),
		"capabilities": []string{
			"wasm.plugin.registry",
			"wasm.plugin.lifecycle",
			"wasm.sandbox.execute",
			"wasm.permission.plan",
			"wasm.host_abi.plan",
			"wasm.evidence.export",
		},
		"notes": []string{"Host ABI permission plan preview is available as a non-destructive contract; runtime host function binding/enforcement and remote signed package install remain follow-up wiring."},
	})
}

func (h *Handler) Plugins(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		plugins, err := h.listPlugins()
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"plugins": plugins, "count": len(plugins)})
	case http.MethodPost:
		var req InstallRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid plugin payload")
			return
		}
		plugin, err := h.normalizePlugin(req)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		if req.DryRun {
			writeJSON(w, http.StatusOK, map[string]any{"plugin": plugin, "status": "validated", "plan": permissionPlan(plugin.Permissions), "host_abi_plan": hostABIPlan(plugin.Permissions)})
			return
		}
		if err := h.savePlugin(plugin); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, map[string]any{"plugin": plugin, "status": "installed"})
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (h *Handler) PluginDetail(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	slug := strings.TrimPrefix(r.URL.Path, "/v1/wasm-plugin/plugins/")
	plugin, err := h.loadPlugin(slug)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"plugin": plugin})
}

func (h *Handler) Load(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	plugin, err := h.pluginFromActionBody(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	plugin.Status = "loaded"
	plugin.LoadedAt = h.now().UTC()
	if plugin.SHA256 == "" {
		plugin.SHA256 = h.computeSHA256(plugin.ModulePath)
	}
	if err := h.savePlugin(plugin); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]any{"plugin": plugin, "status": "loaded"})
}

func (h *Handler) Unload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	plugin, err := h.pluginFromActionBody(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	plugin.Status = "installed"
	plugin.LoadedAt = time.Time{}
	if err := h.savePlugin(plugin); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]any{"plugin": plugin, "status": "unloaded"})
}

func (h *Handler) Execute(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var req ExecuteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || strings.TrimSpace(req.Slug) == "" {
		writeError(w, http.StatusBadRequest, "slug is required")
		return
	}
	plugin, err := h.loadPlugin(req.Slug)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	entrypoint := strings.TrimSpace(req.Entrypoint)
	if entrypoint == "" {
		entrypoint = plugin.Entrypoint
	}
	if entrypoint == "" {
		entrypoint = "_start"
	}
	if plugin.Status != "loaded" {
		writeJSON(w, http.StatusConflict, map[string]any{"error": "plugin is not loaded", "plugin": plugin.Slug})
		return
	}
	result := ExecuteResult{Slug: plugin.Slug, DryRun: req.DryRun, Entrypoint: entrypoint, Success: true, ExitCode: 0, Plan: permissionPlan(plugin.Permissions), HostABIPlan: hostABIPlan(plugin.Permissions)}
	if req.DryRun {
		result.Notes = []string{"dry-run only validates manifest metadata, permission plan, Host ABI plan, and entrypoint selection."}
		writeJSON(w, http.StatusOK, map[string]any{"result": result})
		return
	}
	modulePath, err := h.resolveModulePath(plugin.ModulePath)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	wasmBytes, err := os.ReadFile(modulePath)
	if err != nil {
		writeError(w, http.StatusNotFound, fmt.Sprintf("wasm module not found: %s", plugin.ModulePath))
		return
	}
	sandboxResult, err := h.sandbox.Execute(r.Context(), wasmBytes, req.Input, entrypoint)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	result.Success = sandboxResult.ExitCode == 0
	result.ExitCode = sandboxResult.ExitCode
	result.Stdout = sandboxResult.Stdout
	result.Stderr = sandboxResult.Stderr
	result.Duration = sandboxResult.Duration
	result.MemUsed = sandboxResult.MemUsed
	result.Exports = sandboxResult.Exports
	result.KVWrites = sandboxResult.KVWrites
	plugin.ExecCount++
	plugin.LastExecAt = h.now().UTC()
	_ = h.savePlugin(plugin)
	writeJSON(w, http.StatusOK, map[string]any{"result": result})
}

func (h *Handler) Evidence(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	slug := strings.TrimPrefix(r.URL.Path, "/v1/wasm-plugin/evidence/")
	plugin, err := h.loadPlugin(slug)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"pack_id":       PackID,
		"exported_at":   h.now().UTC(),
		"format":        "json-wasm-plugin-evidence",
		"files":         []string{"plugin.json", "permission-plan.json", "host-abi-plan.json", "sandbox-stats.json"},
		"plugin":        plugin,
		"plan":          permissionPlan(plugin.Permissions),
		"host_abi_plan": hostABIPlan(plugin.Permissions),
		"sandbox":       h.sandbox.Stats(),
	})
}

func (h *Handler) pluginFromActionBody(r *http.Request) (Plugin, error) {
	var req struct {
		Slug string `json:"slug"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || strings.TrimSpace(req.Slug) == "" {
		return Plugin{}, fmt.Errorf("slug is required")
	}
	return h.loadPlugin(req.Slug)
}

func (h *Handler) normalizePlugin(req InstallRequest) (Plugin, error) {
	slug := strings.ToLower(strings.TrimSpace(req.Slug))
	if slug == "" {
		slug = slugify(req.Name)
	}
	if !safeSlugRe.MatchString(slug) {
		return Plugin{}, fmt.Errorf("plugin slug must match ^[a-z0-9][a-z0-9_-]{0,79}$")
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		name = slug
	}
	version := strings.TrimSpace(req.Version)
	if version == "" {
		version = "0.1.0"
	}
	entrypoint := strings.TrimSpace(req.Entrypoint)
	if entrypoint == "" {
		entrypoint = "_start"
	}
	modulePath, err := normalizeModulePath(req.ModulePath, slug)
	if err != nil {
		return Plugin{}, err
	}
	policy := normalizePolicy(req.Permissions)
	plugin := Plugin{
		Slug:         slug,
		Name:         name,
		Version:      version,
		Description:  strings.TrimSpace(req.Description),
		Runtime:      "wazero",
		Entrypoint:   entrypoint,
		ModulePath:   modulePath,
		SHA256:       h.computeSHA256(modulePath),
		Status:       "installed",
		Permissions:  policy,
		Capabilities: cleanList(req.Capabilities),
		Tags:         cleanList(req.Tags),
	}
	return plugin, nil
}

func normalizePolicy(policy PluginPermissionPolicy) PluginPermissionPolicy {
	if policy.MaxMemoryMB <= 0 {
		policy.MaxMemoryMB = 64
	}
	if policy.TimeoutSeconds <= 0 {
		policy.TimeoutSeconds = 30
	}
	policy.AllowedHosts = cleanList(policy.AllowedHosts)
	policy.EnvAllowlist = cleanList(policy.EnvAllowlist)
	return policy
}

func permissionPlan(policy PluginPermissionPolicy) []PermissionCheck {
	policy = normalizePolicy(policy)
	checks := []PermissionCheck{
		{Name: "ledger_kv", Allowed: policy.LedgerKV, Reason: boolReason(policy.LedgerKV, "KV ABI enabled", "KV ABI disabled")},
		{Name: "memory_search", Allowed: policy.MemorySearch, Reason: boolReason(policy.MemorySearch, "memory search ABI enabled", "memory search ABI disabled")},
		{Name: "http_fetch", Allowed: policy.HTTPFetch, Reason: boolReason(policy.HTTPFetch, fmt.Sprintf("allowed hosts: %s", strings.Join(policy.AllowedHosts, ",")), "network ABI disabled")},
		{Name: "env_get", Allowed: len(policy.EnvAllowlist) > 0, Reason: fmt.Sprintf("allowlist entries: %d", len(policy.EnvAllowlist))},
	}
	return checks
}

func hostABIPlan(policy PluginPermissionPolicy) HostABIPlan {
	policy = normalizePolicy(policy)
	envEnabled := len(policy.EnvAllowlist) > 0
	functions := []HostABIFunctionPlan{
		{
			Name:             "ledger_kv_get",
			Category:         "ledger_kv",
			Permission:       "ledger_kv",
			Enabled:          policy.LedgerKV,
			EnforcementReady: false,
			WritesFiles:      false,
			NetworkAccess:    false,
			Constraints:      []string{"namespace/key scope must be enforced by the future host binding"},
			Reason:           boolReason(policy.LedgerKV, "ledger KV read ABI requested by plugin policy", "ledger KV ABI disabled by plugin policy"),
		},
		{
			Name:             "ledger_kv_put",
			Category:         "ledger_kv",
			Permission:       "ledger_kv",
			Enabled:          policy.LedgerKV,
			EnforcementReady: false,
			WritesFiles:      false,
			NetworkAccess:    false,
			Constraints:      []string{"writes must stay inside Ledger KV namespaces after enforcement is wired"},
			Reason:           boolReason(policy.LedgerKV, "ledger KV write ABI requested by plugin policy", "ledger KV ABI disabled by plugin policy"),
		},
		{
			Name:             "ledger_memory_search",
			Category:         "memory_search",
			Permission:       "memory_search",
			Enabled:          policy.MemorySearch,
			EnforcementReady: false,
			WritesFiles:      false,
			NetworkAccess:    false,
			Constraints:      []string{"read-only search must apply tenant and memory-scope filters in the future host binding"},
			Reason:           boolReason(policy.MemorySearch, "memory search ABI requested by plugin policy", "memory search ABI disabled by plugin policy"),
		},
		{
			Name:             "http_fetch",
			Category:         "http_fetch",
			Permission:       "http_fetch",
			Enabled:          policy.HTTPFetch,
			EnforcementReady: false,
			WritesFiles:      false,
			NetworkAccess:    policy.HTTPFetch,
			Constraints:      []string{fmt.Sprintf("allowed_hosts=%s", strings.Join(policy.AllowedHosts, ","))},
			Reason:           boolReason(policy.HTTPFetch, fmt.Sprintf("HTTP fetch ABI requested with %d allowed host(s)", len(policy.AllowedHosts)), "network ABI disabled by plugin policy"),
		},
		{
			Name:             "log_write",
			Category:         "telemetry",
			Permission:       "telemetry",
			Enabled:          true,
			EnforcementReady: false,
			WritesFiles:      false,
			NetworkAccess:    false,
			Constraints:      []string{"structured logs must be redacted and rate-limited by the future host binding"},
			Reason:           "telemetry ABI is planned for diagnostics only; host binding is not wired yet",
		},
		{
			Name:             "env_get",
			Category:         "env_get",
			Permission:       "env_get",
			Enabled:          envEnabled,
			EnforcementReady: false,
			WritesFiles:      false,
			NetworkAccess:    false,
			Constraints:      []string{fmt.Sprintf("env_allowlist=%s", strings.Join(policy.EnvAllowlist, ","))},
			Reason:           fmt.Sprintf("environment allowlist entries: %d", len(policy.EnvAllowlist)),
		},
	}
	enabled := 0
	for _, fn := range functions {
		if fn.Enabled {
			enabled++
		}
	}
	return HostABIPlan{
		PlanReady:        true,
		Ready:            false,
		Status:           "plan_only",
		EnforcementReady: false,
		WritesFiles:      false,
		NetworkAccess:    policy.HTTPFetch,
		Functions:        functions,
		Summary: HostABISummary{
			FunctionCount:     len(functions),
			EnabledCount:      enabled,
			LedgerKV:          policy.LedgerKV,
			MemorySearch:      policy.MemorySearch,
			HTTPFetch:         policy.HTTPFetch,
			EnvGet:            envEnabled,
			AllowedHostCount:  len(policy.AllowedHosts),
			EnvAllowlistCount: len(policy.EnvAllowlist),
		},
		ResourceLimits: HostABIResourceLimits{
			MaxMemoryMB:    policy.MaxMemoryMB,
			TimeoutSeconds: policy.TimeoutSeconds,
			AllowedHosts:   append([]string{}, policy.AllowedHosts...),
			EnvAllowlist:   append([]string{}, policy.EnvAllowlist...),
		},
		Labels: []string{"host-abi", "plan-only", "no-enforcement", "no-file-write"},
		Notes: []string{
			"Preview only: this pack does not bind wazero host functions or enforce Host ABI permissions in this slice.",
			"Use this deterministic plan as the contract for the later Host ABI permission enforcement slice.",
		},
	}
}

func boolReason(ok bool, yes string, no string) string {
	if ok {
		return yes
	}
	return no
}

func (h *Handler) listPlugins() ([]PluginSummary, error) {
	files, err := filepath.Glob(filepath.Join(h.dataDir, "plugins", "*.json"))
	if err != nil {
		return nil, err
	}
	summaries := make([]PluginSummary, 0, len(files))
	for _, file := range files {
		plugin, err := h.loadPluginFromPath(file)
		if err != nil {
			continue
		}
		summaries = append(summaries, summarize(plugin))
	}
	sort.Slice(summaries, func(i, j int) bool { return summaries[i].Slug < summaries[j].Slug })
	return summaries, nil
}

func (h *Handler) loadPlugin(slug string) (Plugin, error) {
	slug = strings.ToLower(strings.TrimSpace(slug))
	if !safeSlugRe.MatchString(slug) {
		return Plugin{}, fmt.Errorf("invalid plugin slug")
	}
	return h.loadPluginFromPath(h.pluginMetaPath(slug))
}

func (h *Handler) loadPluginFromPath(path string) (Plugin, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Plugin{}, err
	}
	var plugin Plugin
	if err := json.Unmarshal(data, &plugin); err != nil {
		return Plugin{}, err
	}
	plugin.Permissions = normalizePolicy(plugin.Permissions)
	return plugin, nil
}

func (h *Handler) savePlugin(plugin Plugin) error {
	if !safeSlugRe.MatchString(plugin.Slug) {
		return fmt.Errorf("invalid plugin slug")
	}
	if err := os.MkdirAll(filepath.Dir(h.pluginMetaPath(plugin.Slug)), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(plugin, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(h.pluginMetaPath(plugin.Slug), data, 0o644)
}

func (h *Handler) pluginMetaPath(slug string) string {
	return filepath.Join(h.dataDir, "plugins", slug+".json")
}

func normalizeModulePath(raw string, slug string) (string, error) {
	modulePath := strings.TrimSpace(raw)
	if modulePath == "" || modulePath == "." {
		modulePath = slug + ".wasm"
	}
	return validateModulePath(modulePath)
}

func validateModulePath(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	slashNormalized := strings.ReplaceAll(trimmed, "\\", "/")
	if strings.HasPrefix(slashNormalized, "/") || strings.HasPrefix(slashNormalized, "//") || windowsVolumeRe.MatchString(slashNormalized) {
		return "", fmt.Errorf("module_path must be relative to plugin_dir")
	}
	modulePath := filepath.FromSlash(slashNormalized)
	if modulePath == "" || modulePath == "." {
		return "", fmt.Errorf("module_path is required")
	}
	if filepath.IsAbs(modulePath) || filepath.VolumeName(modulePath) != "" {
		return "", fmt.Errorf("module_path must be relative to plugin_dir")
	}
	for _, part := range strings.FieldsFunc(modulePath, func(r rune) bool { return r == '/' || r == '\\' }) {
		if part == ".." {
			return "", fmt.Errorf("module_path must not contain traversal segments")
		}
	}
	clean := filepath.Clean(modulePath)
	if clean == "." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) || clean == ".." {
		return "", fmt.Errorf("module_path must stay inside plugin_dir")
	}
	return filepath.ToSlash(clean), nil
}

func (h *Handler) resolveModulePath(modulePath string) (string, error) {
	clean, err := validateModulePath(modulePath)
	if err != nil {
		return "", err
	}
	return filepath.Join(h.pluginDir, filepath.FromSlash(clean)), nil
}

func (h *Handler) computeSHA256(modulePath string) string {
	resolved, err := h.resolveModulePath(modulePath)
	if err != nil {
		return ""
	}
	data, err := os.ReadFile(resolved)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func summarize(plugin Plugin) PluginSummary {
	return PluginSummary{
		Slug:         plugin.Slug,
		Name:         plugin.Name,
		Version:      plugin.Version,
		Description:  plugin.Description,
		Runtime:      plugin.Runtime,
		Entrypoint:   plugin.Entrypoint,
		ModulePath:   plugin.ModulePath,
		SHA256:       plugin.SHA256,
		Status:       plugin.Status,
		LoadedAt:     plugin.LoadedAt,
		ExecCount:    plugin.ExecCount,
		LastExecAt:   plugin.LastExecAt,
		Permissions:  plugin.Permissions,
		Capabilities: plugin.Capabilities,
	}
}

func slugify(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	var b strings.Builder
	lastDash := false
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_' {
			b.WriteRune(r)
			lastDash = false
			continue
		}
		if !lastDash {
			b.WriteByte('-')
			lastDash = true
		}
	}
	out := strings.Trim(b.String(), "-")
	if out == "" {
		out = "wasm-plugin"
	}
	if len(out) > 80 {
		out = strings.Trim(out[:80], "-")
	}
	return out
}

func cleanList(values []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]any{"error": message})
}
