// Package controlplanepack mounts the governance/ops control-plane HTTP surface
// as a Pack Runtime backend module. The pack owns route registration + the
// enable gate; filled-in slices host their handlers here, while remaining slices
// still dispatch through the gateway bridge via the narrow ControlPlaneGateway
// interface.
//
// Migrated slices so far:
//   - governance (registerGovernanceRoutes): audit, trust, iterate, review,
//     skillgrow, usage/quota (bridge).
//   - approvals (registerApprovalRoutes): human-in-the-loop approval surface
//     (bridge).
//   - observability: system info/stats, metrics/prometheus and cache stats
//     (native).
//
// It ships default-enabled (an always-on core surface) so audit/trust and the
// other governance APIs stay available out of the box; operators can still
// disable it from the pack center for the leanest surface.
//
// Only surfaces whose gateway routes are uniformly requireAuth are migrated per
// slice: the pack route gate wraps handlers with requireAuth, so surfaces that
// need requireAdmin or requireSetupOrAuth (e.g. sandbox, rbac, setup, some
// provider routes) must wait until the pack auth modes are extended. Remaining
// ops surfaces (tenants, metrics, plugins, models, inbox, tools, bots,
// providers) are migrated in later slices.
package controlplanepack

import (
	"context"
	"net/http"
	"sync/atomic"

	"yunque-agent/internal/agentcore/planner"
	"yunque-agent/internal/observe"
	"yunque-agent/pkg/packruntime"
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
	HandleControlPlanePack(w http.ResponseWriter, r *http.Request)
	MetricsSnapshot() observe.MetricsSnapshot
	MetricsPrometheus() string
	ModelRuntimeHealth() planner.ModelRuntimeHealth
	LLMResponseCacheStats() map[string]any
	SystemStats(ctx context.Context) map[string]any
}

// Handler is the control-plane pack's backend module.
type Handler struct {
	gateway ControlPlaneGateway
	host    packruntime.Host
	started atomic.Bool
}

// NewHandler builds the control-plane pack backed by the gateway bridge.
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

// Routes mounts the governance/ops surface. Methods are declared broadly so
// bridge-backed routes keep the original permissive (handler-decides) method
// behavior; the manifest lists these as path-only routes so the pack gate allows
// any method. Tightening to exact methods is a fill-the-flesh follow-up.
func (h *Handler) Routes() []packruntime.BackendRoute {
	d := h.gateway.HandleControlPlanePack
	methods := []string{
		http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodPatch,
	}
	routes := make([]packruntime.BackendRoute, 0, len(Paths))
	for _, p := range Paths {
		handler := d
		switch p {
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
		}
		routes = append(routes, packruntime.BackendRoute{Methods: methods, Path: p, Handler: handler})
	}
	return routes
}
