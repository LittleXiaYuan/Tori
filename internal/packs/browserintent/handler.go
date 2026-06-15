package browserintent

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"sync/atomic"
	"time"

	"yunque-agent/pkg/packruntime"
)

const PackID = "yunque.pack.browser-intent"

// BrowserGateway is the narrow Gateway surface required by the Browser Intent
// pack. The pack owns route registration and enablement gates while Gateway
// continues to host the existing browser connector implementation during this
// bridge phase.
type BrowserGateway interface {
	HandleBrowserIntentPack(w http.ResponseWriter, r *http.Request)
	HandleBrowserIntentSession(w http.ResponseWriter, r *http.Request)
}

// Handler exposes browser connection, capture, OPP preview and extension
// scenario surfaces as a Pack Runtime backend module. The bridge keeps the
// migration reversible: disabling the pack removes the HTTP surface without
// touching the browser WebSocket hub or skill implementation.
type Handler struct {
	gateway BrowserGateway
	host    packruntime.Host
	started atomic.Bool
}

func NewHandler(gateway BrowserGateway) *Handler {
	return &Handler{gateway: gateway}
}

func (h *Handler) PackID() string { return PackID }

// compile-time assertion: Browser Intent is a v2 capability Module (Tier 0 microkernel).
var _ packruntime.Module = (*Handler)(nil)

// Init wires the pack against the kernel Host. The pack already depends on the
// narrow BrowserGateway interface, not the concrete Gateway.
func (h *Handler) Init(host packruntime.Host) error {
	h.host = host
	return nil
}

// Start marks the pack live on enable.
func (h *Handler) Start(ctx context.Context) error {
	h.started.Store(true)
	if h.host != nil {
		h.host.Logger().Info("browser-intent pack started", "pack", PackID)
	}
	return nil
}

// Stop marks the pack stopped on disable.
func (h *Handler) Stop(ctx context.Context) error {
	h.started.Store(false)
	return nil
}

func (h *Handler) Routes() []packruntime.BackendRoute {
	return []packruntime.BackendRoute{
		{Method: http.MethodGet, Path: "/v1/browser/status", Handler: h.gateway.HandleBrowserIntentPack},
		{Method: http.MethodGet, Path: "/v1/browser/config", Handler: h.gateway.HandleBrowserIntentPack},
		{Method: http.MethodPost, Path: "/v1/browser/intent/plan", Handler: h.BrowserActPlan},
		{Method: http.MethodPost, Path: "/v1/browser/navigate", Handler: h.gateway.HandleBrowserIntentPack},
		{Method: http.MethodGet, Path: "/v1/browser/screenshot", Handler: h.gateway.HandleBrowserIntentPack},
		{Method: http.MethodPost, Path: "/v1/browser/ocr", Handler: h.gateway.HandleBrowserIntentPack},
		{Method: http.MethodGet, Path: "/v1/browser/screenshot/latest", Handler: h.gateway.HandleBrowserIntentPack},
		{Method: http.MethodGet, Path: "/v1/browser/opp/pending", Handler: h.gateway.HandleBrowserIntentPack},
		{Method: http.MethodPost, Path: "/v1/browser/opp/decide", Handler: h.gateway.HandleBrowserIntentPack},
		{Method: http.MethodGet, Path: "/api/browser/ext/status", Handler: h.gateway.HandleBrowserIntentPack},
		{Method: http.MethodPost, Path: "/api/browser/ext/session", Auth: packruntime.BackendRouteAuthPassthrough, Handler: h.gateway.HandleBrowserIntentSession},
		{Method: http.MethodPost, Path: "/api/browser/ext/action", Handler: h.gateway.HandleBrowserIntentPack},
		{Method: http.MethodGet, Path: "/api/browser/ext/scenarios", Handler: h.gateway.HandleBrowserIntentPack},
		{Method: http.MethodPost, Path: "/api/browser/ext/scenarios/run", Handler: h.gateway.HandleBrowserIntentPack},
	}
}

// BrowserActPlanRequest describes one semantic browser_act intent for the
// plan-only gate. The route deliberately accepts a compact action shape rather
// than the raw extension command payload so operators can review policy and
// runtime skill gates before a future executor consumes a browser session.
type BrowserActPlanRequest struct {
	Intent      string         `json:"intent,omitempty"`
	TargetURL   string         `json:"target_url,omitempty"`
	Selector    string         `json:"selector,omitempty"`
	Text        string         `json:"text,omitempty"`
	Value       string         `json:"value,omitempty"`
	RequestedBy string         `json:"requested_by,omitempty"`
	Reason      string         `json:"reason,omitempty"`
	DryRun      *bool          `json:"dry_run,omitempty"`
	Metadata    map[string]any `json:"metadata,omitempty"`
}

type BrowserActPlannedAction struct {
	Index                  int    `json:"index"`
	Intent                 string `json:"intent"`
	ExecutorAction         string `json:"executor_action"`
	TargetURL              string `json:"target_url,omitempty"`
	Selector               string `json:"selector,omitempty"`
	Text                   string `json:"text,omitempty"`
	Value                  string `json:"value,omitempty"`
	RequiresPermission     string `json:"requires_permission"`
	RequiresRuntimeSkill   string `json:"requires_runtime_skill"`
	RequiresOPPGate        bool   `json:"requires_opp_gate"`
	ConsumesBrowserSession bool   `json:"consumes_browser_session"`
	ExecutesBrowserAction  bool   `json:"executes_browser_action"`
	WritesBrowserState     bool   `json:"writes_browser_state"`
	NetworkAccess          bool   `json:"network_access"`
}

type BrowserPermissionGatePlan struct {
	Gate                     string   `json:"gate"`
	RequiredPermissions      []string `json:"required_permissions"`
	PermissionGateReady      bool     `json:"permission_gate_ready"`
	PermissionPolicyEnforced bool     `json:"permission_policy_enforced"`
	AllowsExecution          bool     `json:"allows_execution"`
	RequiresHumanApproval    bool     `json:"requires_human_approval"`
	BlockedBy                []string `json:"blocked_by"`
	Notes                    []string `json:"notes,omitempty"`
}

type BrowserRuntimeSkillGatePlan struct {
	Gate                  string   `json:"gate"`
	RequiredSkill         string   `json:"required_skill"`
	RuntimeSkillGateReady bool     `json:"runtime_skill_gate_ready"`
	RuntimeSkillReady     bool     `json:"runtime_skill_ready"`
	ConsumesSkillRuntime  bool     `json:"consumes_skill_runtime"`
	AllowsExecution       bool     `json:"allows_execution"`
	BlockedBy             []string `json:"blocked_by"`
	Notes                 []string `json:"notes,omitempty"`
}

type BrowserOPPGatePlan struct {
	Gate                  string   `json:"gate"`
	OPPGateReady          bool     `json:"opp_gate_ready"`
	OPPDecisionReady      bool     `json:"opp_decision_ready"`
	RequiresHumanApproval bool     `json:"requires_human_approval"`
	AllowsExecution       bool     `json:"allows_execution"`
	BlockedBy             []string `json:"blocked_by"`
	Notes                 []string `json:"notes,omitempty"`
}

type BrowserActPlanReport struct {
	PackID                 string                      `json:"pack_id"`
	GeneratedAt            time.Time                   `json:"generated_at"`
	Stage                  string                      `json:"stage"`
	Status                 string                      `json:"status"`
	DryRun                 bool                        `json:"dry_run"`
	Intent                 string                      `json:"intent"`
	RequestedBy            string                      `json:"requested_by,omitempty"`
	Reason                 string                      `json:"reason,omitempty"`
	BrowserActPlanReady    bool                        `json:"browser_act_plan_ready"`
	BrowserActReady        bool                        `json:"browser_act_ready"`
	PermissionGateReady    bool                        `json:"permission_gate_ready"`
	RuntimeSkillGateReady  bool                        `json:"runtime_skill_gate_ready"`
	OPPGateReady           bool                        `json:"opp_gate_ready"`
	ConsumesBrowserSession bool                        `json:"consumes_browser_session"`
	ExecutesBrowserActions bool                        `json:"executes_browser_actions"`
	WritesBrowserState     bool                        `json:"writes_browser_state"`
	WritesFiles            bool                        `json:"writes_files"`
	NetworkAccess          bool                        `json:"network_access"`
	RequiresHumanApproval  bool                        `json:"requires_human_approval"`
	ActionCount            int                         `json:"action_count"`
	PlannedActions         []BrowserActPlannedAction   `json:"planned_actions"`
	PermissionGate         BrowserPermissionGatePlan   `json:"permission_gate"`
	RuntimeSkillGate       BrowserRuntimeSkillGatePlan `json:"runtime_skill_gate"`
	OPPGate                BrowserOPPGatePlan          `json:"opp_gate"`
	Artifacts              []string                    `json:"artifacts"`
	Actions                []string                    `json:"actions"`
	BlockedBy              []string                    `json:"blocked_by"`
	Labels                 []string                    `json:"labels"`
	Notes                  []string                    `json:"notes,omitempty"`
}

// BrowserActPlan shapes a future semantic browser_act execution contract. It
// does not call the browser extension, consume a session, navigate, click, OCR,
// write browser state, write files, or touch the network.
func (h *Handler) BrowserActPlan(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var req BrowserActPlanRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid browser_act plan payload")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"plan": buildBrowserActPlan(req)})
}

func buildBrowserActPlan(req BrowserActPlanRequest) BrowserActPlanReport {
	intent := normalizeIntent(req.Intent, req.TargetURL, req.Selector, req.Text, req.Value)
	dryRun := true
	if req.DryRun != nil {
		dryRun = *req.DryRun
	}
	requestedBy := strings.TrimSpace(req.RequestedBy)
	if requestedBy == "" {
		requestedBy = "operator"
	}
	reason := strings.TrimSpace(req.Reason)
	if reason == "" {
		reason = "plan semantic browser_act intent before policy/runtime execution"
	}
	blockedBy := []string{
		"browser-act-runtime-not-wired",
		"runtime-skill-gate-not-wired",
		"permission-policy-not-enforced",
		"opp-decision-runtime-not-wired",
	}
	action := BrowserActPlannedAction{
		Index:                  1,
		Intent:                 intent,
		ExecutorAction:         executorActionForIntent(intent),
		TargetURL:              strings.TrimSpace(req.TargetURL),
		Selector:               strings.TrimSpace(req.Selector),
		Text:                   strings.TrimSpace(req.Text),
		Value:                  strings.TrimSpace(req.Value),
		RequiresPermission:     permissionForIntent(intent),
		RequiresRuntimeSkill:   "browser_act",
		RequiresOPPGate:        true,
		ConsumesBrowserSession: false,
		ExecutesBrowserAction:  false,
		WritesBrowserState:     false,
		NetworkAccess:          false,
	}
	permissionGate := BrowserPermissionGatePlan{
		Gate:                     "browser.permission.policy_gate",
		RequiredPermissions:      []string{permissionForIntent(intent), "browser:connect"},
		PermissionGateReady:      true,
		PermissionPolicyEnforced: false,
		AllowsExecution:          false,
		RequiresHumanApproval:    true,
		BlockedBy:                []string{"permission-policy-not-enforced", "browser-act-runtime-not-wired"},
		Notes: []string{
			"Permission gate is a review contract only; it does not grant browser permissions.",
			"Execution remains blocked until Pack Runtime enforces the declared permission policy.",
		},
	}
	runtimeSkillGate := BrowserRuntimeSkillGatePlan{
		Gate:                  "browser.runtime_skill.gate",
		RequiredSkill:         "browser_act",
		RuntimeSkillGateReady: true,
		RuntimeSkillReady:     false,
		ConsumesSkillRuntime:  false,
		AllowsExecution:       false,
		BlockedBy:             []string{"runtime-skill-gate-not-wired", "browser-act-runtime-not-wired"},
		Notes:                 []string{"Runtime skill gate is plan-only; no skill runtime or extension session is consumed."},
	}
	oppGate := BrowserOPPGatePlan{
		Gate:                  "browser.opp.human_approval_gate",
		OPPGateReady:          true,
		OPPDecisionReady:      false,
		RequiresHumanApproval: true,
		AllowsExecution:       false,
		BlockedBy:             []string{"opp-decision-runtime-not-wired", "browser-act-runtime-not-wired"},
		Notes:                 []string{"OPP gate is shaped for future approval flow; this route does not create or resolve OPP items."},
	}
	return BrowserActPlanReport{
		PackID:                 PackID,
		GeneratedAt:            time.Now().UTC(),
		Stage:                  "browser-act-plan-before-runtime",
		Status:                 "browser_act_intent_plan_ready_pending_policy_runtime",
		DryRun:                 dryRun,
		Intent:                 intent,
		RequestedBy:            requestedBy,
		Reason:                 reason,
		BrowserActPlanReady:    true,
		BrowserActReady:        false,
		PermissionGateReady:    true,
		RuntimeSkillGateReady:  true,
		OPPGateReady:           true,
		ConsumesBrowserSession: false,
		ExecutesBrowserActions: false,
		WritesBrowserState:     false,
		WritesFiles:            false,
		NetworkAccess:          false,
		RequiresHumanApproval:  true,
		ActionCount:            1,
		PlannedActions:         []BrowserActPlannedAction{action},
		PermissionGate:         permissionGate,
		RuntimeSkillGate:       runtimeSkillGate,
		OPPGate:                oppGate,
		Artifacts:              []string{"browser-act-plan.json", "browser-permission-gate.json", "runtime-skill-gate.json", "opp-gate-plan.json"},
		Actions: []string{
			"mapped semantic browser_act intent into a future executor input contract",
			"kept browser session consumption, action execution, browser state writes, file writes, and network access disabled",
		},
		BlockedBy: blockedBy,
		Labels:    []string{"browser-intent", "browser_act", "permission-gate", "runtime-skill-gate", "opp-gate", "plan-only"},
		Notes: []string{
			"browser_act_plan_ready=true only means the semantic intent and gate artifacts are shaped.",
			"browser_act_ready=false, consumes_browser_session=false, executes_browser_actions=false, writes_browser_state=false, writes_files=false, and network_access=false keep this route non-destructive.",
		},
	}
}

func normalizeIntent(intent, targetURL, selector, text, value string) string {
	intent = strings.ToLower(strings.TrimSpace(intent))
	intent = strings.ReplaceAll(intent, "-", "_")
	intent = strings.ReplaceAll(intent, " ", "_")
	switch intent {
	case "", "browser_act":
		switch {
		case strings.TrimSpace(targetURL) != "":
			return "open_url"
		case strings.TrimSpace(selector) != "" && strings.TrimSpace(value) != "":
			return "type_text"
		case strings.TrimSpace(selector) != "" || strings.TrimSpace(text) != "":
			return "click"
		default:
			return "noop"
		}
	case "navigate", "open", "open_url", "browser_navigate":
		return "open_url"
	case "click", "press", "tap", "browser_click":
		return "click"
	case "type", "type_text", "fill", "input", "browser_type":
		return "type_text"
	case "extract", "ocr", "extract_content", "browser_extract":
		return "extract_content"
	case "screenshot", "capture", "capture_screenshot", "browser_screenshot":
		return "capture_screenshot"
	default:
		return intent
	}
}

func executorActionForIntent(intent string) string {
	switch intent {
	case "open_url":
		return "browser.navigate"
	case "click":
		return "browser.click"
	case "type_text":
		return "browser.type_text"
	case "extract_content":
		return "browser.extract_content"
	case "capture_screenshot":
		return "browser.capture_screenshot"
	case "noop":
		return "browser.noop"
	default:
		return "browser.custom_intent"
	}
}

func permissionForIntent(intent string) string {
	switch intent {
	case "extract_content", "capture_screenshot":
		return "browser:read"
	case "noop":
		return "browser:read"
	default:
		return "browser:write"
	}
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]any{"error": message})
}
