package wasmplugin

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
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

type RemoteInstallPlanRequest struct {
	Slug         string            `json:"slug,omitempty"`
	Name         string            `json:"name,omitempty"`
	Version      string            `json:"version,omitempty"`
	PackageURL   string            `json:"package_url"`
	ManifestURL  string            `json:"manifest_url,omitempty"`
	ModulePath   string            `json:"module_path,omitempty"`
	SHA256       string            `json:"sha256,omitempty"`
	Signature    string            `json:"signature,omitempty"`
	PublicKeyID  string            `json:"public_key_id,omitempty"`
	Entrypoint   string            `json:"entrypoint,omitempty"`
	RequestedBy  string            `json:"requested_by,omitempty"`
	Reason       string            `json:"reason,omitempty"`
	Metadata     map[string]string `json:"metadata,omitempty"`
	Capabilities []string          `json:"capabilities,omitempty"`
	Tags         []string          `json:"tags,omitempty"`
}

type RemoteInstallApprovalPlanRequest struct {
	Slug        string            `json:"slug,omitempty"`
	Name        string            `json:"name,omitempty"`
	Version     string            `json:"version,omitempty"`
	PackageURL  string            `json:"package_url"`
	ManifestURL string            `json:"manifest_url,omitempty"`
	ModulePath  string            `json:"module_path,omitempty"`
	SHA256      string            `json:"sha256,omitempty"`
	Signature   string            `json:"signature,omitempty"`
	PublicKeyID string            `json:"public_key_id,omitempty"`
	Entrypoint  string            `json:"entrypoint,omitempty"`
	RequestedBy string            `json:"requested_by,omitempty"`
	Reason      string            `json:"reason,omitempty"`
	RiskTier    string            `json:"risk_tier,omitempty"`
	Approvers   []string          `json:"approvers,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

type RemoteInstallPlanReport struct {
	PackID                 string                   `json:"pack_id"`
	GeneratedAt            time.Time                `json:"generated_at"`
	Status                 string                   `json:"status"`
	RemoteInstallPlanReady bool                     `json:"remote_install_plan_ready"`
	RemoteInstallReady     bool                     `json:"remote_install_ready"`
	DownloadReady          bool                     `json:"download_ready"`
	SignatureVerifyReady   bool                     `json:"signature_verify_ready"`
	Downloads              bool                     `json:"downloads"`
	InstallsPlugin         bool                     `json:"installs_plugin"`
	WritesFiles            bool                     `json:"writes_files"`
	NetworkAccess          bool                     `json:"network_access"`
	Plugin                 RemoteInstallPluginPlan  `json:"plugin"`
	Package                RemoteInstallPackagePlan `json:"package"`
	Checks                 []RemoteInstallCheck     `json:"checks"`
	Artifacts              []string                 `json:"artifacts"`
	Actions                []string                 `json:"actions"`
	Labels                 []string                 `json:"labels"`
	RequestedBy            string                   `json:"requested_by,omitempty"`
	Reason                 string                   `json:"reason,omitempty"`
	Metadata               map[string]string        `json:"metadata,omitempty"`
	Notes                  []string                 `json:"notes,omitempty"`
}

type RemoteInstallApprovalPlanReport struct {
	PackID                   string                   `json:"pack_id"`
	GeneratedAt              time.Time                `json:"generated_at"`
	Status                   string                   `json:"status"`
	ApprovalGatePlanReady    bool                     `json:"approval_gate_plan_ready"`
	ApprovalGateReady        bool                     `json:"approval_gate_ready"`
	RequiresApproval         bool                     `json:"requires_approval"`
	ApprovalQueueReady       bool                     `json:"approval_queue_ready"`
	WritesApprovalQueue      bool                     `json:"writes_approval_queue"`
	WritesFiles              bool                     `json:"writes_files"`
	Downloads                bool                     `json:"downloads"`
	NetworkAccess            bool                     `json:"network_access"`
	InstallsPlugin           bool                     `json:"installs_plugin"`
	Decision                 string                   `json:"decision"`
	RiskTier                 string                   `json:"risk_tier"`
	RequestedBy              string                   `json:"requested_by,omitempty"`
	Reason                   string                   `json:"reason,omitempty"`
	Plugin                   RemoteInstallPluginPlan  `json:"plugin"`
	Package                  RemoteInstallPackagePlan `json:"package"`
	Checks                   []RemoteInstallCheck     `json:"checks"`
	Approvers                []string                 `json:"approvers,omitempty"`
	Artifacts                []string                 `json:"artifacts"`
	Actions                  []string                 `json:"actions"`
	Labels                   []string                 `json:"labels"`
	Metadata                 map[string]string        `json:"metadata,omitempty"`
	RemoteInstallPlanSummary RemoteInstallPlanReport  `json:"remote_install_plan_summary"`
	Notes                    []string                 `json:"notes,omitempty"`
}

type RemoteInstallPluginPlan struct {
	Slug         string   `json:"slug"`
	Name         string   `json:"name"`
	Version      string   `json:"version"`
	Runtime      string   `json:"runtime"`
	Entrypoint   string   `json:"entrypoint"`
	ModulePath   string   `json:"module_path"`
	Capabilities []string `json:"capabilities,omitempty"`
	Tags         []string `json:"tags,omitempty"`
}

type RemoteInstallPackagePlan struct {
	ManifestURL      string `json:"manifest_url"`
	PackageURL       string `json:"package_url"`
	ExpectedSHA256   string `json:"expected_sha256,omitempty"`
	Signature        string `json:"signature,omitempty"`
	PublicKeyID      string `json:"public_key_id,omitempty"`
	ManifestArtifact string `json:"manifest_artifact"`
	PackageArtifact  string `json:"package_artifact"`
	CacheKey         string `json:"cache_key"`
}

type RemoteInstallCheck struct {
	Name     string `json:"name"`
	Required bool   `json:"required"`
	Ready    bool   `json:"ready"`
	Reason   string `json:"reason,omitempty"`
}

type ExecuteResult struct {
	Slug                string               `json:"slug"`
	DryRun              bool                 `json:"dry_run"`
	Entrypoint          string               `json:"entrypoint"`
	Success             bool                 `json:"success"`
	ExitCode            int                  `json:"exit_code"`
	Stdout              string               `json:"stdout,omitempty"`
	Stderr              string               `json:"stderr,omitempty"`
	Duration            string               `json:"duration,omitempty"`
	MemUsed             uint32               `json:"mem_used_bytes,omitempty"`
	Exports             []string             `json:"exports,omitempty"`
	KVWrites            map[string]string    `json:"kv_writes,omitempty"`
	Plan                []PermissionCheck    `json:"plan,omitempty"`
	HostABIPlan         HostABIPlan          `json:"host_abi_plan"`
	HostABIGate         HostABIExecutionGate `json:"host_abi_gate"`
	ModuleIntegrityGate ModuleIntegrityGate  `json:"module_integrity_gate"`
	Notes               []string             `json:"notes,omitempty"`
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

type ModuleIntegrityGate struct {
	IntegrityGateReady bool     `json:"integrity_gate_ready"`
	AllowsExecution    bool     `json:"allows_execution"`
	Blocked            bool     `json:"blocked"`
	Status             string   `json:"status"`
	ExpectedSHA256     string   `json:"expected_sha256,omitempty"`
	ActualSHA256       string   `json:"actual_sha256,omitempty"`
	ModulePath         string   `json:"module_path"`
	WritesFiles        bool     `json:"writes_files"`
	NetworkAccess      bool     `json:"network_access"`
	Reason             string   `json:"reason,omitempty"`
	Labels             []string `json:"labels"`
	Notes              []string `json:"notes,omitempty"`
}

type HostABIExecutionGate struct {
	ExecutionGateReady bool     `json:"execution_gate_ready"`
	AllowsExecution    bool     `json:"allows_execution"`
	Blocked            bool     `json:"blocked"`
	Status             string   `json:"status"`
	EnforcementReady   bool     `json:"enforcement_ready"`
	WritesFiles        bool     `json:"writes_files"`
	NetworkAccess      bool     `json:"network_access"`
	RequestedFunctions []string `json:"requested_functions,omitempty"`
	AllowedFunctions   []string `json:"allowed_functions,omitempty"`
	BlockedFunctions   []string `json:"blocked_functions,omitempty"`
	Reason             string   `json:"reason,omitempty"`
	Labels             []string `json:"labels"`
	Notes              []string `json:"notes,omitempty"`
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
		{Method: http.MethodPost, Path: "/v1/wasm-plugin/remote-install/plan", Handler: h.RemoteInstallPlan},
		{Method: http.MethodPost, Path: "/v1/wasm-plugin/remote-install/approval/plan", Handler: h.RemoteInstallApprovalPlan},
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
		"pack_id":                       PackID,
		"stage":                         "pack-shell-before-runtime-hosts",
		"runtime_ready":                 true,
		"abi_plan_ready":                true,
		"abi_ready":                     false,
		"host_abi_execution_gate_ready": true,
		"host_abi_enforcement_ready":    false,
		"module_integrity_gate_ready":   true,
		"remote_install_plan_ready":     true,
		"remote_install_ready":          false,
		"approval_gate_plan_ready":      true,
		"approval_gate_ready":           false,
		"plugin_count":                  len(plugins),
		"loaded_count":                  loaded,
		"plugin_dir":                    h.pluginDir,
		"store_dir":                     h.dataDir,
		"sandbox":                       h.sandbox.Stats(),
		"capabilities": []string{
			"wasm.plugin.registry",
			"wasm.plugin.lifecycle",
			"wasm.sandbox.execute",
			"wasm.permission.plan",
			"wasm.host_abi.plan",
			"wasm.host_abi.execution_gate",
			"wasm.module.integrity_gate",
			"wasm.remote_install.plan",
			"wasm.remote_install.approval_plan",
			"wasm.evidence.export",
		},
		"notes": []string{"Host ABI permission plan preview, conservative execution gate, module integrity gate, remote signed package install plan preview, and approval gate plan preview are available as contracts; privileged Host ABI calls are blocked during real execution while enforcement_ready=false, local module SHA-256 drift is blocked before sandbox execution, and runtime host function binding/enforcement, package download, signature verification, approval queue write-back, and install write-back remain follow-up wiring."},
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
	result := ExecuteResult{Slug: plugin.Slug, DryRun: req.DryRun, Entrypoint: entrypoint, Success: true, ExitCode: 0, Plan: permissionPlan(plugin.Permissions), HostABIPlan: hostABIPlan(plugin.Permissions), HostABIGate: hostABIExecutionGate(plugin.Permissions), ModuleIntegrityGate: moduleIntegrityGate(plugin, "")}
	if req.DryRun {
		result.Notes = []string{"dry-run only validates manifest metadata, permission plan, Host ABI plan, Host ABI execution gate, module integrity gate contract, and entrypoint selection."}
		writeJSON(w, http.StatusOK, map[string]any{"result": result})
		return
	}
	if !result.HostABIGate.AllowsExecution {
		result.Success = false
		result.ExitCode = -3
		result.Stderr = "host ABI execution blocked until permission enforcement is ready"
		result.Notes = []string{"Conservative Pack Runtime gate: privileged Host ABI requests cannot execute until host function permission enforcement is wired."}
		writeJSON(w, http.StatusConflict, map[string]any{"error": "host ABI execution blocked by pack gate", "result": result})
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
	actualSHA := sha256Bytes(wasmBytes)
	result.ModuleIntegrityGate = moduleIntegrityGate(plugin, actualSHA)
	if !result.ModuleIntegrityGate.AllowsExecution {
		result.Success = false
		result.ExitCode = -4
		result.Stderr = "wasm module integrity check failed before sandbox execution"
		result.Notes = []string{"Conservative Pack Runtime gate: local WASM module bytes must match the registered SHA-256 before sandbox execution."}
		writeJSON(w, http.StatusConflict, map[string]any{"error": "wasm module integrity blocked by pack gate", "result": result})
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

func (h *Handler) RemoteInstallPlan(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var req RemoteInstallPlanRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid remote install plan payload")
		return
	}
	plan, err := h.buildRemoteInstallPlan(req, true)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"plan": plan})
}

func (h *Handler) RemoteInstallApprovalPlan(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var req RemoteInstallApprovalPlanRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid remote install approval plan payload")
		return
	}
	plan, err := h.buildRemoteInstallApprovalPlan(req, true)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"plan": plan})
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
		"pack_id":               PackID,
		"exported_at":           h.now().UTC(),
		"format":                "json-wasm-plugin-evidence",
		"files":                 []string{"plugin.json", "permission-plan.json", "host-abi-plan.json", "module-integrity-gate.json", "remote-install-plan.json", "approval-gate-plan.json", "sandbox-stats.json"},
		"plugin":                plugin,
		"plan":                  permissionPlan(plugin.Permissions),
		"host_abi_plan":         hostABIPlan(plugin.Permissions),
		"host_abi_gate":         hostABIExecutionGate(plugin.Permissions),
		"module_integrity_gate": moduleIntegrityGate(plugin, h.computeSHA256(plugin.ModulePath)),
		"remote_install_plan":   h.remoteInstallPlanForPlugin(plugin),
		"approval_gate_plan":    h.remoteInstallApprovalPlanForPlugin(plugin),
		"sandbox":               h.sandbox.Stats(),
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

func moduleIntegrityGate(plugin Plugin, actualSHA string) ModuleIntegrityGate {
	expectedSHA := strings.ToLower(strings.TrimSpace(plugin.SHA256))
	actualSHA = strings.ToLower(strings.TrimSpace(actualSHA))
	allowsExecution := expectedSHA == "" || actualSHA == "" || expectedSHA == actualSHA
	status := "pending_runtime_sha256"
	reason := "runtime module SHA-256 will be checked after loading module bytes"
	labels := []string{"module-integrity", "execution-gate", "pending-runtime-check"}
	notes := []string{"Dry-run exposes the integrity gate contract without reading or hashing module bytes."}
	if actualSHA != "" {
		status = "verified"
		reason = "registered SHA-256 matches local module bytes"
		labels = []string{"module-integrity", "execution-gate", "verified"}
		notes = []string{"Local module bytes were hashed before sandbox execution."}
	}
	if expectedSHA == "" {
		status = "allowed_no_registered_sha256"
		reason = "plugin has no registered SHA-256; integrity gate is ready but cannot compare bytes"
		labels = []string{"module-integrity", "execution-gate", "no-registered-sha256"}
		notes = []string{"Register plugins through the pack installer to persist a SHA-256 before real execution."}
	}
	if expectedSHA != "" && actualSHA != "" && expectedSHA != actualSHA {
		allowsExecution = false
		status = "blocked_module_sha256_mismatch"
		reason = "local WASM module SHA-256 does not match the registered plugin metadata"
		labels = []string{"module-integrity", "execution-gate", "blocked", "sha256-mismatch"}
		notes = []string{
			"Real execution is blocked before sandbox execution because module bytes drifted after registration.",
			"Re-register or reinstall the plugin to update trusted metadata after reviewing the module change.",
		}
	}
	return ModuleIntegrityGate{
		IntegrityGateReady: true,
		AllowsExecution:    allowsExecution,
		Blocked:            !allowsExecution,
		Status:             status,
		ExpectedSHA256:     expectedSHA,
		ActualSHA256:       actualSHA,
		ModulePath:         plugin.ModulePath,
		WritesFiles:        false,
		NetworkAccess:      false,
		Reason:             reason,
		Labels:             labels,
		Notes:              notes,
	}
}

func hostABIExecutionGate(policy PluginPermissionPolicy) HostABIExecutionGate {
	policy = normalizePolicy(policy)
	requested := []string{}
	if policy.LedgerKV {
		requested = append(requested, "ledger_kv_get", "ledger_kv_put")
	}
	if policy.MemorySearch {
		requested = append(requested, "ledger_memory_search")
	}
	if policy.HTTPFetch {
		requested = append(requested, "http_fetch")
	}
	if len(policy.EnvAllowlist) > 0 {
		requested = append(requested, "env_get")
	}
	sort.Strings(requested)
	allowed := []string{"log_write"}
	blocked := append([]string{}, requested...)
	allowsExecution := len(blocked) == 0
	status := "allowed_no_privileged_host_abi"
	reason := "plugin does not request privileged Host ABI functions; sandbox execution may proceed without Host ABI enforcement"
	labels := []string{"host-abi", "execution-gate", "no-privileged-host-abi"}
	notes := []string{"The gate is active before real sandbox execution and keeps privileged Host ABI calls unavailable until enforcement is wired."}
	if !allowsExecution {
		status = "blocked_until_host_abi_enforcement"
		reason = "plugin requests privileged Host ABI functions while enforcement_ready=false"
		labels = []string{"host-abi", "execution-gate", "blocked", "needs-enforcement"}
		notes = []string{
			"Real execution is blocked before loading the WASM module because privileged Host ABI permission enforcement is not wired yet.",
			"Dry-run remains available for manifest, permission, and Host ABI planning.",
		}
	}
	return HostABIExecutionGate{
		ExecutionGateReady: true,
		AllowsExecution:    allowsExecution,
		Blocked:            !allowsExecution,
		Status:             status,
		EnforcementReady:   false,
		WritesFiles:        false,
		NetworkAccess:      false,
		RequestedFunctions: requested,
		AllowedFunctions:   allowed,
		BlockedFunctions:   blocked,
		Reason:             reason,
		Labels:             labels,
		Notes:              notes,
	}
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

func (h *Handler) remoteInstallPlanForPlugin(plugin Plugin) RemoteInstallPlanReport {
	plan, _ := h.buildRemoteInstallPlan(RemoteInstallPlanRequest{
		Slug:         plugin.Slug,
		Name:         plugin.Name,
		Version:      plugin.Version,
		PackageURL:   fmt.Sprintf("https://packs.yunque.local/wasm-plugin/%s-%s.tgz", plugin.Slug, plugin.Version),
		ManifestURL:  fmt.Sprintf("https://packs.yunque.local/wasm-plugin/%s/manifest.json", plugin.Slug),
		ModulePath:   plugin.ModulePath,
		SHA256:       plugin.SHA256,
		Entrypoint:   plugin.Entrypoint,
		Capabilities: plugin.Capabilities,
		Tags:         plugin.Tags,
		Reason:       "evidence export preview for remote signed package install contract",
	}, false)
	return plan
}

func (h *Handler) remoteInstallApprovalPlanForPlugin(plugin Plugin) RemoteInstallApprovalPlanReport {
	plan, _ := h.buildRemoteInstallApprovalPlan(RemoteInstallApprovalPlanRequest{
		Slug:        plugin.Slug,
		Name:        plugin.Name,
		Version:     plugin.Version,
		PackageURL:  fmt.Sprintf("https://packs.yunque.local/wasm-plugin/%s-%s.tgz", plugin.Slug, plugin.Version),
		ManifestURL: fmt.Sprintf("https://packs.yunque.local/wasm-plugin/%s/manifest.json", plugin.Slug),
		ModulePath:  plugin.ModulePath,
		SHA256:      plugin.SHA256,
		Entrypoint:  plugin.Entrypoint,
		RequestedBy: "evidence-export",
		Reason:      "evidence export preview for remote signed package approval gate contract",
		RiskTier:    "high",
	}, false)
	return plan
}

func (h *Handler) buildRemoteInstallPlan(req RemoteInstallPlanRequest, requirePackageURL bool) (RemoteInstallPlanReport, error) {
	packageURL := strings.TrimSpace(req.PackageURL)
	if packageURL == "" && requirePackageURL {
		return RemoteInstallPlanReport{}, fmt.Errorf("package_url is required")
	}
	if packageURL == "" {
		packageURL = fmt.Sprintf("https://packs.yunque.local/wasm-plugin/%s.tgz", slugify(req.Name))
	}
	normalizedPackageURL, err := validateRemotePackageURL(packageURL)
	if err != nil {
		return RemoteInstallPlanReport{}, err
	}
	manifestURL := strings.TrimSpace(req.ManifestURL)
	if manifestURL == "" {
		manifestURL = normalizedPackageURL + ".manifest.json"
	}
	normalizedManifestURL, err := validateRemotePackageURL(manifestURL)
	if err != nil {
		return RemoteInstallPlanReport{}, fmt.Errorf("manifest_url: %w", err)
	}
	slug := strings.ToLower(strings.TrimSpace(req.Slug))
	if slug == "" {
		slug = slugify(req.Name)
	}
	if !safeSlugRe.MatchString(slug) {
		return RemoteInstallPlanReport{}, fmt.Errorf("plugin slug must match ^[a-z0-9][a-z0-9_-]{0,79}$")
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
		return RemoteInstallPlanReport{}, err
	}
	expectedSHA := strings.ToLower(strings.TrimSpace(req.SHA256))
	signature := strings.TrimSpace(req.Signature)
	publicKeyID := strings.TrimSpace(req.PublicKeyID)
	packageArtifact := remotePackageArtifactName(slug, version, normalizedPackageURL)
	manifestArtifact := slug + "-remote-manifest.json"
	checks := []RemoteInstallCheck{
		{Name: "package_url_valid", Required: true, Ready: true, Reason: "package_url is a normalized http(s) URL"},
		{Name: "manifest_url_valid", Required: true, Ready: true, Reason: "manifest_url is a normalized http(s) URL"},
		{Name: "sha256_present", Required: true, Ready: expectedSHA != "", Reason: boolReason(expectedSHA != "", "expected SHA-256 is provided for later artifact verification", "expected SHA-256 is required before real install")},
		{Name: "signature_present", Required: true, Ready: signature != "", Reason: boolReason(signature != "", "signature metadata is provided for later verification", "signature is required before real install")},
		{Name: "public_key_id_present", Required: true, Ready: publicKeyID != "", Reason: boolReason(publicKeyID != "", "public key id is provided for later trust-root lookup", "public key id is required before real install")},
		{Name: "module_path_relative", Required: true, Ready: true, Reason: "module_path is validated to stay inside plugin_dir"},
	}
	return RemoteInstallPlanReport{
		PackID:                 PackID,
		GeneratedAt:            h.now().UTC(),
		Status:                 "plan_only",
		RemoteInstallPlanReady: true,
		RemoteInstallReady:     false,
		DownloadReady:          false,
		SignatureVerifyReady:   false,
		Downloads:              false,
		InstallsPlugin:         false,
		WritesFiles:            false,
		NetworkAccess:          false,
		Plugin: RemoteInstallPluginPlan{
			Slug:         slug,
			Name:         name,
			Version:      version,
			Runtime:      "wazero",
			Entrypoint:   entrypoint,
			ModulePath:   modulePath,
			Capabilities: cleanList(req.Capabilities),
			Tags:         cleanList(req.Tags),
		},
		Package: RemoteInstallPackagePlan{
			ManifestURL:      normalizedManifestURL,
			PackageURL:       normalizedPackageURL,
			ExpectedSHA256:   expectedSHA,
			Signature:        signature,
			PublicKeyID:      publicKeyID,
			ManifestArtifact: manifestArtifact,
			PackageArtifact:  packageArtifact,
			CacheKey:         sha256Hex(normalizedManifestURL + "\n" + normalizedPackageURL + "\n" + slug + "\n" + version),
		},
		Checks: checks,
		Artifacts: []string{
			"remote-install-plan.json",
			manifestArtifact,
			packageArtifact,
			"signature-verification.json",
		},
		Actions: []string{
			"would fetch the remote plugin manifest after explicit install wiring is enabled",
			"would download the package into the Pack Runtime artifact cache without touching plugin_dir in plan mode",
			"would verify SHA-256 and signature before allowing install write-back",
			"would register plugin metadata only after download and signature verification pass",
		},
		Labels:      []string{"remote-install", "signed-package", "plan-only", "no-download", "no-file-write"},
		RequestedBy: strings.TrimSpace(req.RequestedBy),
		Reason:      strings.TrimSpace(req.Reason),
		Metadata:    cleanStringMap(req.Metadata),
		Notes: []string{
			"Preview only: this route does not download packages, fetch manifests, verify signatures, or write plugin metadata.",
			"Use this deterministic plan as the contract for the later remote signed package installer slice.",
		},
	}, nil
}

func (h *Handler) buildRemoteInstallApprovalPlan(req RemoteInstallApprovalPlanRequest, requirePackageURL bool) (RemoteInstallApprovalPlanReport, error) {
	installPlan, err := h.buildRemoteInstallPlan(RemoteInstallPlanRequest{
		Slug:        req.Slug,
		Name:        req.Name,
		Version:     req.Version,
		PackageURL:  req.PackageURL,
		ManifestURL: req.ManifestURL,
		ModulePath:  req.ModulePath,
		SHA256:      req.SHA256,
		Signature:   req.Signature,
		PublicKeyID: req.PublicKeyID,
		Entrypoint:  req.Entrypoint,
		RequestedBy: req.RequestedBy,
		Reason:      req.Reason,
		Metadata:    req.Metadata,
	}, requirePackageURL)
	if err != nil {
		return RemoteInstallApprovalPlanReport{}, err
	}
	riskTier := strings.ToLower(strings.TrimSpace(req.RiskTier))
	if riskTier == "" {
		riskTier = "high"
	}
	approvers := cleanList(req.Approvers)
	checks := append([]RemoteInstallCheck{}, installPlan.Checks...)
	checks = append(checks,
		RemoteInstallCheck{Name: "approval_required", Required: true, Ready: true, Reason: "remote signed WASM package install must pass approval before any download or write-back"},
		RemoteInstallCheck{Name: "approval_queue_ready", Required: true, Ready: false, Reason: "approval queue persistence is not wired in this plan-only slice"},
		RemoteInstallCheck{Name: "approver_present", Required: false, Ready: len(approvers) > 0, Reason: boolReason(len(approvers) > 0, "approver hints are provided for later queue routing", "approver hints are optional until approval queue routing lands")},
	)
	return RemoteInstallApprovalPlanReport{
		PackID:                   PackID,
		GeneratedAt:              h.now().UTC(),
		Status:                   "plan_only",
		ApprovalGatePlanReady:    true,
		ApprovalGateReady:        false,
		RequiresApproval:         true,
		ApprovalQueueReady:       false,
		WritesApprovalQueue:      false,
		WritesFiles:              false,
		Downloads:                false,
		NetworkAccess:            false,
		InstallsPlugin:           false,
		Decision:                 "requires_approval",
		RiskTier:                 riskTier,
		RequestedBy:              strings.TrimSpace(req.RequestedBy),
		Reason:                   strings.TrimSpace(req.Reason),
		Plugin:                   installPlan.Plugin,
		Package:                  installPlan.Package,
		Checks:                   checks,
		Approvers:                approvers,
		Artifacts:                []string{"approval-gate-plan.json", "remote-install-plan.json", "signature-verification.json"},
		Actions:                  []string{"would create an approval request only after approval queue persistence is wired", "would require an explicit approval decision before remote package download starts", "would keep package download, signature verification, install write-back, and plugin registration blocked while approval_gate_ready=false"},
		Labels:                   []string{"remote-install", "approval-gate", "plan-only", "requires-approval", "no-queue-write", "no-download", "no-file-write"},
		Metadata:                 cleanStringMap(req.Metadata),
		RemoteInstallPlanSummary: installPlan,
		Notes:                    []string{"Preview only: this route does not write an approval queue entry, download packages, fetch manifests, verify signatures, or install plugins.", "Use this deterministic approval gate plan as the contract for the later remote installer approval workflow slice."},
	}, nil
}

func validateRemotePackageURL(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", fmt.Errorf("remote package URL is required")
	}
	parsed, err := url.Parse(trimmed)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", fmt.Errorf("remote package URL must be absolute")
	}
	if parsed.Scheme != "https" && parsed.Scheme != "http" {
		return "", fmt.Errorf("remote package URL must use http or https")
	}
	parsed.Fragment = ""
	return parsed.String(), nil
}

func remotePackageArtifactName(slug, version, packageURL string) string {
	parsed, err := url.Parse(packageURL)
	ext := ".tgz"
	if err == nil {
		base := filepath.Base(parsed.Path)
		if parsedExt := filepath.Ext(base); parsedExt != "" && len(parsedExt) <= 10 {
			ext = parsedExt
		}
	}
	return slug + "-" + version + ext
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
	return sha256Bytes(data)
}

func sha256Bytes(data []byte) string {
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

func cleanStringMap(values map[string]string) map[string]string {
	if len(values) == 0 {
		return nil
	}
	out := map[string]string{}
	for key, value := range values {
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if key == "" || value == "" {
			continue
		}
		out[key] = value
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func sha256Hex(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]any{"error": message})
}
