// Package controlplanepack mounts the governance/ops control-plane HTTP surface
// as a Pack Runtime backend module. The pack owns route registration + the
// enable gate; migrated slices host their handlers here and obtain runtime state
// through the narrow ControlPlaneGateway interface.
//
// Migrated slices so far:
//   - governance (registerGovernanceRoutes): audit/trust/iterate/review/
//     skillgrow + usage/quota (native).
//   - approvals (registerApprovalRoutes): human-in-the-loop approval surface
//     (native).
//   - observability: system info/stats, metrics/prometheus and cache stats
//     (native).
//   - tenants: list/create tenant collection route (native).
//   - inbox: message collection and mark-read route (native).
//   - bots: bot collection/detail CRUD routes (native).
//   - tools: process execution/session routes (native).
//   - models: explicit model catalog plus provider-derived models (native).
//   - usage/quota: tenant usage read and quota write routes (native).
//   - plugins: CRUD/files/UI/reload/open-folder routes (native).
//   - providers/router: requireAuth provider management, router stats, breaker
//     reset and exec-provider routes (native).
//
// It ships default-enabled (an always-on core surface) so audit/trust and the
// other governance APIs stay available out of the box; operators can still
// disable it from the pack center for the leanest surface.
//
// Only surfaces whose gateway routes are uniformly requireAuth are migrated per
// slice: the pack route gate wraps handlers with requireAuth, so surfaces that
// need requireAdmin or requireSetupOrAuth (e.g. sandbox, rbac, setup, some
// provider setup routes) must wait until the pack auth modes are extended.
package controlplanepack

import (
	"context"
	"net/http"
	"sync/atomic"

	"yunque-agent/internal/agentcore/approval"
	"yunque-agent/internal/agentcore/audit"
	"yunque-agent/internal/agentcore/bots"
	"yunque-agent/internal/agentcore/inbox"
	"yunque-agent/internal/agentcore/llm"
	"yunque-agent/internal/agentcore/llm/distill"
	"yunque-agent/internal/agentcore/planner"
	"yunque-agent/internal/agentcore/review"
	"yunque-agent/internal/agentcore/router"
	"yunque-agent/internal/agentcore/selfheal/iterate"
	"yunque-agent/internal/agentcore/skillgrowth/adapter"
	"yunque-agent/internal/agentcore/tools"
	"yunque-agent/internal/agentcore/trust"
	"yunque-agent/internal/controlplane/models"
	"yunque-agent/internal/controlplane/tenant"
	"yunque-agent/internal/observe"
	"yunque-agent/internal/tori"
	"yunque-agent/pkg/packruntime"
	"yunque-agent/pkg/plugin"
)

const PackID = "yunque.pack.control-plane"

// Paths is the governance subset this slice owns. Exported so tests and the
// manifest helper can stay in sync with the handler without duplicating strings.
var Paths = []string{
	"/v1/audit/tail",
	"/v1/audit/verify",
	"/v1/audit/stats",
	"/api/audit/trail",
	"/api/trust/scores",
	"/api/trust/reset",
	"/api/trust/grant",
	"/api/iterate/proposals",
	"/api/iterate/approve",
	"/api/iterate/reject",
	"/api/iterate/trigger",
	"/api/iterate/status",
	"/api/review/status",
	"/api/skillgrow/patterns",
	"/v1/usage",
	"/v1/quota",
	// approvals (registerApprovalRoutes)
	"/v1/approvals",
	"/v1/approvals/approve",
	"/v1/approvals/deny",
	"/v1/approvals/decide",
	"/v1/approvals/rules",
	// inbox (registerChatRoutes inbox subset)
	"/v1/inbox",
	"/v1/inbox/read",
	// tools process execution (registerTriggerRoutes tools subset; sandbox stays admin)
	"/v1/tools/exec",
	"/v1/tools/list",
	"/v1/tools/poll",
	"/v1/tools/kill",
	// bots (registerChatRoutes bots subset)
	"/v1/bots",
	"/v1/bots/detail",
	// plugins (registerPluginRoutes plugin subset; skillhub/market stay separate)
	"/v1/plugins",
	"/v1/plugins/toggle",
	"/v1/plugins/create",
	"/v1/plugins/delete",
	"/v1/plugins/files",
	"/v1/plugins/ui",
	"/v1/plugins/reload",
	"/v1/plugins/open-folder",
	// metrics / system observability (registerSystemRoutes subset)
	"/v1/system/info",
	"/v1/system/stats",
	"/v1/metrics",
	"/v1/metrics/prometheus",
	"/v1/cache/stats",
	// tenants (registerSystemRoutes subset)
	"/v1/tenants",
	// providers / models (registerProviderRoutes requireAuth subset; the 3
	// setup-flow routes /api/providers/{mode,presets,register} stay direct because
	// they are requireSetupOrAuth and must not depend on the pack-enabled gate
	// during onboarding).
	"/v1/models",
	"/api/providers",
	"/api/providers/test",
	"/api/providers/enable",
	"/api/providers/disable",
	"/api/providers/switch-model",
	"/api/providers/session",
	"/api/providers/local/discover",
	"/api/providers/local/register",
	"/api/providers/delete",
	"/api/providers/tori/discover",
	"/v1/router/stats",
	"/api/breaker/reset",
	"/api/providers/exec",
}

// ControlPlaneGateway is the narrow gateway surface the control-plane pack needs.
type ControlPlaneGateway interface {
	ApprovalManager() *approval.Manager
	AuditChain() *audit.Chain
	AuditTrail() *audit.Trail
	BotManager() *bots.Manager
	InboxStore() *inbox.Store
	ShellPolicy() *tools.ShellExecPolicy
	TenantManager() *tenant.Manager
	TenantOf(ctx context.Context) string
	ToolsManager() *tools.ProcessManager
	TrustTracker() *trust.Tracker
	RoleOf(ctx context.Context) string
	ReviewGate() *review.Gate
	Distiller() *distill.Distiller
	SkillGrowDetector() *adapter.Detector
	IterateEngine() *iterate.Engine
	OutputDir() string
	MetricsSnapshot() observe.MetricsSnapshot
	MetricsPrometheus() string
	ModelRuntimeHealth() planner.ModelRuntimeHealth
	LLMResponseCacheStats() map[string]any
	SystemStats(ctx context.Context) map[string]any
	ModelManager() *models.Manager
	ProviderModels() []models.ProviderModel
	DeleteProviderModel(id string) bool
	UsageSnapshot(ctx context.Context) any
	SetUsageQuota(ctx context.Context, tenantID string, maxChatCalls, maxTokensPerDay int64)
}

type pluginGateway interface {
	PluginRegistry() *plugin.Registry
	PluginLoader() *plugin.Loader
	RebuildSkillsFromPlugins() int
}

type providerGateway interface {
	ProviderRegistry() *llm.ProviderRegistry
	ToriTokenStore() *tori.TokenStore
	SmartRouter() *router.Router
	ExecProvider() string
	SetExecProvider(id string)
}

// Handler is the control-plane pack's backend module.
type Handler struct {
	gateway ControlPlaneGateway
	host    packruntime.Host
	started atomic.Bool
}

// NewHandler builds the control-plane pack backed by narrow gateway accessors.
func NewHandler(gateway ControlPlaneGateway) *Handler { return &Handler{gateway: gateway} }

// PackID returns the stable manifest id.
func (h *Handler) PackID() string { return PackID }

// compile-time assertion: Control Plane is a v2 capability Module (Tier 0 microkernel).
var _ packruntime.Module = (*Handler)(nil)

// Init wires the pack against the kernel Host. The pack already depends on the
// narrow ControlPlaneGateway interface, not the concrete Gateway.
func (h *Handler) Init(host packruntime.Host) error {
	h.host = host
	return nil
}

// Start marks the pack live on enable.
func (h *Handler) Start(ctx context.Context) error {
	h.started.Store(true)
	if h.host != nil {
		h.host.Logger().Info("control-plane pack started", "pack", PackID)
	}
	return nil
}

// Stop marks the pack stopped on disable.
func (h *Handler) Stop(ctx context.Context) error {
	h.started.Store(false)
	return nil
}

// Routes mounts the governance/ops surface. Methods are declared broadly to
// preserve the original handler-decides method behavior; the manifest lists
// these as path-only routes so the pack gate allows any method. Tightening to
// exact methods is a follow-up.
func (h *Handler) Routes() []packruntime.BackendRoute {
	methods := []string{
		http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodPatch,
	}
	routes := make([]packruntime.BackendRoute, 0, len(Paths))
	for _, p := range Paths {
		handler := http.NotFound
		switch p {
		case "/v1/audit/tail":
			handler = h.handleAuditTail
		case "/v1/audit/verify":
			handler = h.handleAuditVerify
		case "/v1/audit/stats":
			handler = h.handleAuditStats
		case "/api/audit/trail":
			handler = h.handleAuditTrail
		case "/api/trust/scores":
			handler = h.handleTrustScores
		case "/api/trust/reset":
			handler = h.handleTrustReset
		case "/api/trust/grant":
			handler = h.handleTrustGrant
		case "/api/review/status":
			handler = h.handleReviewStatus
		case "/api/skillgrow/patterns":
			handler = h.handleSkillGrowPatterns
		case "/api/iterate/proposals":
			handler = h.handleIterateProposals
		case "/api/iterate/approve":
			handler = h.handleIterateApprove
		case "/api/iterate/reject":
			handler = h.handleIterateReject
		case "/api/iterate/trigger":
			handler = h.handleIterateTrigger
		case "/api/iterate/status":
			handler = h.handleIterateStatus
		case "/v1/approvals":
			handler = h.handleApprovalRouteSwitch
		case "/v1/approvals/approve":
			handler = h.handleApprovalApprove
		case "/v1/approvals/deny":
			handler = h.handleApprovalDeny
		case "/v1/approvals/decide":
			handler = h.handleApprovalDecide
		case "/v1/approvals/rules":
			handler = h.handleApprovalRules
		case "/v1/tenants":
			handler = h.handleTenants
		case "/v1/inbox":
			handler = h.handleInbox
		case "/v1/inbox/read":
			handler = h.handleInboxRead
		case "/v1/bots":
			handler = h.handleBots
		case "/v1/bots/detail":
			handler = h.handleBotDetail
		case "/v1/tools/exec":
			handler = h.handleToolExec
		case "/v1/tools/list":
			handler = h.handleToolList
		case "/v1/tools/poll":
			handler = h.handleToolPoll
		case "/v1/tools/kill":
			handler = h.handleToolKill
		case "/v1/plugins":
			handler = h.handlePlugins
		case "/v1/plugins/toggle":
			handler = h.handlePluginToggle
		case "/v1/plugins/create":
			handler = h.handlePluginCreate
		case "/v1/plugins/delete":
			handler = h.handlePluginDelete
		case "/v1/plugins/files":
			handler = h.handlePluginFiles
		case "/v1/plugins/ui":
			handler = h.handlePluginUI
		case "/v1/plugins/reload":
			handler = h.handlePluginReload
		case "/v1/plugins/open-folder":
			handler = h.handlePluginOpenFolder
		case "/v1/system/info":
			handler = h.handleSystemInfo
		case "/v1/system/stats":
			handler = h.handleSystemStats
		case "/v1/metrics":
			handler = h.handleMetrics
		case "/v1/metrics/prometheus":
			handler = h.handleMetricsPrometheus
		case "/v1/cache/stats":
			handler = h.handleCacheStats
		case "/v1/models":
			handler = h.handleModels
		case "/api/providers":
			handler = h.handleProviderList
		case "/api/providers/test":
			handler = h.handleProviderTest
		case "/api/providers/enable":
			handler = h.handleProviderEnable
		case "/api/providers/disable":
			handler = h.handleProviderDisable
		case "/api/providers/switch-model":
			handler = h.handleProviderSwitchModel
		case "/api/providers/session":
			handler = h.handleProviderSessionOverride
		case "/api/providers/local/discover":
			handler = h.handleLocalDiscover
		case "/api/providers/local/register":
			handler = h.handleLocalRegister
		case "/api/providers/delete":
			handler = h.handleProviderDelete
		case "/api/providers/tori/discover":
			handler = h.handleToriDiscover
		case "/v1/router/stats":
			handler = h.handleRouterStats
		case "/api/breaker/reset":
			handler = h.handleBreakerReset
		case "/api/providers/exec":
			handler = h.handleExecProvider
		case "/v1/usage":
			handler = h.handleUsage
		case "/v1/quota":
			handler = h.handleQuota
		}
		routes = append(routes, packruntime.BackendRoute{Methods: methods, Path: p, Handler: handler})
	}
	return routes
}
