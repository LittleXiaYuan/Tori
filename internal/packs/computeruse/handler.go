// Package computeruse exposes a gated, Pack-first computer-use capability.
//
// The first slice is deliberately non-destructive: it reports available
// surfaces, shapes an intent plan and can proxy a browser screenshot, but it
// does not execute local desktop mouse/keyboard actions or sandbox commands.
package computeruse

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync/atomic"
	"time"

	"yunque-agent/pkg/packruntime"
	"yunque-agent/pkg/skills"
)

const PackID = "yunque.pack.computer-use"

// Gateway is the narrow host surface required by the Computer Use pack.
type Gateway interface {
	TenantOf(ctx context.Context) string
	BrowserConnectedForTenant(tenantID string) bool
	BrowserHealth() map[string]any
	SendBrowserActionRaw(ctx context.Context, action json.RawMessage) (json.RawMessage, error)
	DesktopSandboxStatus(ctx context.Context) map[string]any
}

// Handler is the Computer Use backend module.
type Handler struct {
	gateway Gateway
	host    packruntime.Host
	started atomic.Bool
}

func New(gateway Gateway) *Handler {
	return &Handler{gateway: gateway}
}

var _ packruntime.Module = (*Handler)(nil)
var _ packruntime.ContextProvider = (*Handler)(nil)
var _ packruntime.SkillProvider = (*Handler)(nil)

func (h *Handler) PackID() string { return PackID }

func (h *Handler) Init(host packruntime.Host) error {
	h.host = host
	return nil
}

func (h *Handler) Start(ctx context.Context) error {
	h.started.Store(true)
	if h.host != nil {
		h.host.Logger().Info("computer-use pack started", "pack", PackID)
	}
	return nil
}

func (h *Handler) Stop(ctx context.Context) error {
	h.started.Store(false)
	return nil
}

func (h *Handler) Routes() []packruntime.BackendRoute {
	return []packruntime.BackendRoute{
		{Method: http.MethodGet, Path: "/v1/computer/status", Handler: h.Status},
		{Method: http.MethodPost, Path: "/v1/computer/intent/plan", Handler: h.IntentPlan},
		{Method: http.MethodGet, Path: "/v1/computer/screenshot", Handler: h.Screenshot},
	}
}

// Skills exposes a plan-only tool. RegisterModule only publishes it while this
// pack is enabled, so Pack state controls whether Cogni/Planner can select it.
func (h *Handler) Skills() []skills.Skill {
	return []skills.Skill{computerUsePlanSkill{handler: h}}
}

// BuildContext lets the enabled pack show up in the planner's reasoning flow
// only when the user asks for screen/browser/computer operation.
func (h *Handler) BuildContext(ctx context.Context, message, tenant string) string {
	if !computerUseRelevant(message) {
		return ""
	}
	status := h.statusPayloadFor(ctx, tenant)
	executionReady, _ := status["execution_ready"].(bool)
	return fmt.Sprintf(
		"## Computer Use Pack\n"+
			"- Pack: %s is enabled and available as a plan-first capability.\n"+
			"- Execution ready: %v. Dangerous desktop actions are still blocked by approval/runtime gates.\n"+
			"- Use skill `computer_use_plan` to produce a non-destructive plan before any browser, cloud desktop, or local desktop action.\n"+
			"- Read-only browser screenshots may be available when the browser connector is connected; local desktop control is not enabled in beta.",
		PackID, executionReady,
	)
}

func (h *Handler) tenantOf(r *http.Request) string {
	if h.gateway == nil {
		return "default"
	}
	return h.gateway.TenantOf(r.Context())
}

func (h *Handler) browserConnected(r *http.Request) bool {
	return h.gateway != nil && h.gateway.BrowserConnectedForTenant(h.tenantOf(r))
}

func (h *Handler) Status(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "GET only")
		return
	}
	writeJSON(w, http.StatusOK, h.statusPayload(r))
}

func (h *Handler) IntentPlan(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "POST only")
		return
	}
	var req intentPlanRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid computer-use plan payload")
		return
	}
	req.Goal = strings.TrimSpace(req.Goal)
	if req.Goal == "" {
		writeError(w, http.StatusBadRequest, "goal is required")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"plan": h.buildIntentPlanFor(r.Context(), h.tenantOf(r), req)})
}

func (h *Handler) Screenshot(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "GET only")
		return
	}
	surface := normalizeSurface(r.URL.Query().Get("surface"))
	if surface != "auto" && surface != "browser" {
		writeError(w, http.StatusConflict, "only browser screenshots are wired in this computer-use pack slice")
		return
	}
	if !h.browserConnected(r) {
		writeError(w, http.StatusConflict, "browser connector is not connected for current tenant")
		return
	}
	result, err := h.sendBrowserAction(r.Context(), map[string]any{"type": "browser_screenshot"})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "browser screenshot failed: "+err.Error())
		return
	}
	if !resultOK(result) {
		writeError(w, http.StatusInternalServerError, "browser screenshot failed: "+resultError(result))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"surface":    "browser",
		"screenshot": stripDataPrefix(stringValue(result["screenshot"])),
		"timestamp":  time.Now().UTC().Format(time.RFC3339),
		"safety": map[string]any{
			"read_only":            true,
			"local_desktop_action": false,
			"requires_pack_enable": true,
		},
	})
}

func (h *Handler) statusPayload(r *http.Request) map[string]any {
	return h.statusPayloadFor(r.Context(), h.tenantOf(r))
}

func (h *Handler) statusPayloadFor(ctx context.Context, tenantID string) map[string]any {
	connected := h.gateway != nil && h.gateway.BrowserConnectedForTenant(tenantID)
	health := map[string]any{"connected": false}
	if h.gateway != nil {
		health = h.gateway.BrowserHealth()
	}
	desktop := map[string]any{"available": false, "running": false, "status": "not_wired"}
	if h.gateway != nil {
		desktop = h.gateway.DesktopSandboxStatus(ctx)
	}
	return map[string]any{
		"pack_id":         PackID,
		"enabled":         true,
		"execution_ready": false,
		"surfaces": map[string]any{
			"browser": map[string]any{
				"available": true,
				"connected": connected,
				"health":    health,
			},
			"desktop_sandbox": desktop,
			"local_desktop": map[string]any{
				"available": false,
				"status":    "not_supported_in_beta",
			},
		},
		"safety": map[string]any{
			"requires_human_approval": true,
			"direct_local_control":    false,
			"executes_local_commands": false,
			"destructive_actions":     false,
		},
		"capabilities": []string{
			"computer.status",
			"computer.intent.plan",
			"computer.screenshot.browser",
		},
	}
}

type intentPlanRequest struct {
	Goal         string `json:"goal"`
	Surface      string `json:"surface,omitempty"`
	AllowExecute bool   `json:"allow_execute,omitempty"`
	RequestedBy  string `json:"requested_by,omitempty"`
	Reason       string `json:"reason,omitempty"`
}

type intentPlanReport struct {
	PackID                string         `json:"pack_id"`
	GeneratedAt           time.Time      `json:"generated_at"`
	Goal                  string         `json:"goal"`
	Surface               string         `json:"surface"`
	Status                string         `json:"status"`
	PlanReady             bool           `json:"plan_ready"`
	ExecutionReady        bool           `json:"execution_ready"`
	AllowExecuteRequested bool           `json:"allow_execute_requested"`
	RequiresApproval      bool           `json:"requires_approval"`
	ConsumesBrowser       bool           `json:"consumes_browser"`
	ControlsLocalDesktop  bool           `json:"controls_local_desktop"`
	ExecutesCommands      bool           `json:"executes_commands"`
	WritesFiles           bool           `json:"writes_files"`
	NetworkAccess         bool           `json:"network_access"`
	RequiredPermissions   []string       `json:"required_permissions"`
	Steps                 []plannedStep  `json:"steps"`
	Gates                 []gatePlan     `json:"gates"`
	BlockedBy             []string       `json:"blocked_by"`
	Surfaces              map[string]any `json:"surfaces"`
	Artifacts             []string       `json:"artifacts"`
	Notes                 []string       `json:"notes,omitempty"`
}

type plannedStep struct {
	Index       int      `json:"index"`
	Action      string   `json:"action"`
	Surface     string   `json:"surface"`
	ReadOnly    bool     `json:"read_only"`
	Permission  string   `json:"permission"`
	Executor    string   `json:"executor"`
	BlockedBy   []string `json:"blocked_by,omitempty"`
	Description string   `json:"description"`
}

type gatePlan struct {
	Gate           string   `json:"gate"`
	Ready          bool     `json:"ready"`
	AllowsExecute  bool     `json:"allows_execute"`
	RequiredBy     []string `json:"required_by,omitempty"`
	BlockedBy      []string `json:"blocked_by,omitempty"`
	HumanApproval  bool     `json:"human_approval"`
	PolicyEnforced bool     `json:"policy_enforced"`
}

func (h *Handler) buildIntentPlanFor(ctx context.Context, tenantID string, req intentPlanRequest) intentPlanReport {
	surface := normalizeSurface(req.Surface)
	if surface == "auto" {
		if h.gateway != nil && h.gateway.BrowserConnectedForTenant(tenantID) {
			surface = "browser"
		} else {
			surface = "desktop_sandbox"
		}
	}
	permissions := permissionsForSurface(surface)
	blockedBy := []string{
		"computer-use-executor-not-wired",
		"permission-policy-not-enforced",
		"human-approval-runtime-not-wired",
	}
	if req.AllowExecute {
		blockedBy = append(blockedBy, "allow_execute-requested-but-execution-disabled")
	}
	steps := []plannedStep{
		{
			Index:       1,
			Action:      "inspect_surface",
			Surface:     surface,
			ReadOnly:    true,
			Permission:  permissions[0],
			Executor:    executorForSurface(surface, "inspect"),
			Description: "Inspect current surface readiness and collect non-destructive context.",
			BlockedBy:   []string{"computer-use-executor-not-wired"},
		},
		{
			Index:       2,
			Action:      "propose_actions",
			Surface:     surface,
			ReadOnly:    true,
			Permission:  permissions[0],
			Executor:    "computer.plan_only",
			Description: "Return a human-reviewable action plan before any cursor, keyboard, browser or sandbox mutation.",
			BlockedBy:   []string{"human-approval-runtime-not-wired"},
		},
	}
	return intentPlanReport{
		PackID:                PackID,
		GeneratedAt:           time.Now().UTC(),
		Goal:                  req.Goal,
		Surface:               surface,
		Status:                "plan_ready_pending_policy_runtime",
		PlanReady:             true,
		ExecutionReady:        false,
		AllowExecuteRequested: req.AllowExecute,
		RequiresApproval:      true,
		ConsumesBrowser:       false,
		ControlsLocalDesktop:  false,
		ExecutesCommands:      false,
		WritesFiles:           false,
		NetworkAccess:         false,
		RequiredPermissions:   permissions,
		Steps:                 steps,
		Gates: []gatePlan{
			{
				Gate:           "computer.permission.policy_gate",
				Ready:          true,
				AllowsExecute:  false,
				RequiredBy:     permissions,
				BlockedBy:      []string{"permission-policy-not-enforced", "computer-use-executor-not-wired"},
				HumanApproval:  true,
				PolicyEnforced: false,
			},
			{
				Gate:           "computer.human_approval_gate",
				Ready:          true,
				AllowsExecute:  false,
				BlockedBy:      []string{"human-approval-runtime-not-wired", "computer-use-executor-not-wired"},
				HumanApproval:  true,
				PolicyEnforced: false,
			},
		},
		BlockedBy: blockedBy,
		Surfaces:  h.statusPayloadFor(ctx, tenantID)["surfaces"].(map[string]any),
		Artifacts: []string{"computer-use-plan.json", "computer-permission-gate.json", "computer-approval-gate.json"},
		Notes: []string{
			"This route is plan-only. It never moves the mouse, types keys, runs commands, writes files or opens network targets.",
			"Browser screenshots are the only wired read action in this slice, and only after the pack is enabled and the browser connector is connected.",
		},
	}
}

type computerUsePlanSkill struct {
	handler *Handler
}

func (s computerUsePlanSkill) Name() string { return "computer_use_plan" }

func (s computerUsePlanSkill) Description() string {
	return "Create a non-destructive computer-use plan for browser, cloud desktop, or local desktop work. It never clicks, types, runs commands, writes files, or controls the desktop."
}

func (s computerUsePlanSkill) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"goal": map[string]any{
				"type":        "string",
				"description": "The user's goal, e.g. inspect a page, prepare a browser workflow, or plan desktop steps.",
			},
			"surface": map[string]any{
				"type":        "string",
				"enum":        []string{"auto", "browser", "desktop_sandbox", "local_desktop"},
				"description": "Target surface. Defaults to auto.",
			},
			"allow_execute": map[string]any{
				"type":        "boolean",
				"description": "Whether the caller asks for execution. The skill still returns plan-only output in this pack slice.",
			},
		},
		"required": []string{"goal"},
	}
}

func (s computerUsePlanSkill) Execute(ctx context.Context, args map[string]any, env *skills.Environment) (string, error) {
	goal, _ := args["goal"].(string)
	if strings.TrimSpace(goal) == "" {
		return "", fmt.Errorf("goal is required")
	}
	surface, _ := args["surface"].(string)
	allowExecute, _ := args["allow_execute"].(bool)
	tenantID := "default"
	if env != nil && strings.TrimSpace(env.TenantID) != "" {
		tenantID = env.TenantID
	}
	req := intentPlanRequest{Goal: goal, Surface: surface, AllowExecute: allowExecute, RequestedBy: "planner"}
	plan := s.handler.buildIntentPlanFor(ctx, tenantID, req)
	data, err := json.MarshalIndent(plan, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func computerUseRelevant(message string) bool {
	msg := strings.ToLower(message)
	keywords := []string{
		"computer", "desktop", "screen", "screenshot", "browser", "click", "type", "gui", "rpa",
		"电脑", "桌面", "屏幕", "截图", "浏览器", "点击", "输入", "操作", "自动化",
	}
	for _, kw := range keywords {
		if strings.Contains(msg, kw) {
			return true
		}
	}
	return false
}

func normalizeSurface(surface string) string {
	surface = strings.ToLower(strings.TrimSpace(surface))
	surface = strings.ReplaceAll(surface, "-", "_")
	switch surface {
	case "", "auto":
		return "auto"
	case "browser", "web":
		return "browser"
	case "desktop", "desktop_sandbox", "cloud_desktop", "sandbox":
		return "desktop_sandbox"
	case "local", "local_desktop":
		return "local_desktop"
	default:
		return surface
	}
}

func permissionsForSurface(surface string) []string {
	switch surface {
	case "browser":
		return []string{"computer:read", "browser:read", "browser:connect", "computer:control"}
	case "desktop_sandbox":
		return []string{"computer:read", "sandbox:desktop", "computer:control"}
	case "local_desktop":
		return []string{"computer:read", "computer:local-desktop", "computer:control"}
	default:
		return []string{"computer:read", "computer:control"}
	}
}

func executorForSurface(surface, action string) string {
	switch surface {
	case "browser":
		return "browser." + action
	case "desktop_sandbox":
		return "sandbox.desktop." + action
	case "local_desktop":
		return "local_desktop." + action
	default:
		return "computer." + action
	}
}

func (h *Handler) sendBrowserAction(ctx context.Context, action map[string]any) (map[string]any, error) {
	if h.gateway == nil {
		return nil, fmt.Errorf("computer-use gateway not configured")
	}
	raw, err := json.Marshal(action)
	if err != nil {
		return nil, err
	}
	resultRaw, err := h.gateway.SendBrowserActionRaw(ctx, raw)
	if err != nil {
		return nil, err
	}
	var result map[string]any
	if err := json.Unmarshal(resultRaw, &result); err != nil {
		return nil, err
	}
	return result, nil
}

func resultOK(result map[string]any) bool {
	if ok, exists := result["ok"].(bool); exists {
		return ok
	}
	if success, exists := result["success"].(bool); exists {
		return success
	}
	return resultError(result) == ""
}

func resultError(result map[string]any) string {
	for _, key := range []string{"error", "message"} {
		if s := stringValue(result[key]); s != "" {
			return s
		}
	}
	return "unknown error"
}

func stringValue(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

func stripDataPrefix(s string) string {
	if idx := strings.Index(s, ","); idx >= 0 && strings.HasPrefix(s[:idx], "data:") {
		return s[idx+1:]
	}
	return s
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]any{"error": message})
}
