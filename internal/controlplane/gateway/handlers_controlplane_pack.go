package gateway

import "net/http"

// HandleControlPlanePack is the compatibility bridge entrypoint for the
// control-plane pack (internal/packs/controlplane). The pack owns route
// registration + the enablement gate. Native slices live in the pack itself;
// remaining governance/ops handlers dispatch here by path, preserving each
// handler's original method behavior.
func (g *Gateway) HandleControlPlanePack(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case "/v1/audit/tail":
		g.handleAuditTail(w, r)
	case "/v1/audit/verify":
		g.handleAuditVerify(w, r)
	case "/v1/audit/stats":
		g.handleAuditStats(w, r)
	case "/api/audit/trail":
		g.handleAuditTrail(w, r)
	case "/api/trust/scores":
		g.handleTrustScores(w, r)
	case "/api/trust/reset":
		g.handleTrustReset(w, r)
	case "/api/trust/grant":
		g.handleTrustGrant(w, r)
	case "/api/iterate/proposals":
		g.handleIterateProposals(w, r)
	case "/api/iterate/approve":
		g.handleIterateApprove(w, r)
	case "/api/iterate/reject":
		g.handleIterateReject(w, r)
	case "/api/iterate/trigger":
		g.handleIterateTrigger(w, r)
	case "/api/iterate/status":
		g.handleIterateStatus(w, r)
	case "/api/review/status":
		g.handleReviewStatus(w, r)
	case "/api/skillgrow/patterns":
		g.handleSkillGrowPatterns(w, r)
	case "/v1/plugins":
		g.handlePlugins(w, r)
	case "/v1/plugins/toggle":
		g.handlePluginToggle(w, r)
	case "/v1/plugins/create":
		g.handlePluginCreate(w, r)
	case "/v1/plugins/delete":
		g.handlePluginDelete(w, r)
	case "/v1/plugins/files":
		g.handlePluginFiles(w, r)
	case "/v1/plugins/ui":
		g.handlePluginUI(w, r)
	case "/v1/plugins/reload":
		g.handlePluginReload(w, r)
	case "/v1/plugins/open-folder":
		g.handlePluginOpenFolder(w, r)
	case "/api/providers":
		g.handleProviderList(w, r)
	case "/api/providers/test":
		g.handleProviderTest(w, r)
	case "/api/providers/enable":
		g.handleProviderEnable(w, r)
	case "/api/providers/disable":
		g.handleProviderDisable(w, r)
	case "/api/providers/switch-model":
		g.handleProviderSwitchModel(w, r)
	case "/api/providers/session":
		g.handleProviderSessionOverride(w, r)
	case "/api/providers/local/discover":
		g.handleLocalDiscover(w, r)
	case "/api/providers/local/register":
		g.handleLocalRegister(w, r)
	case "/api/providers/delete":
		g.handleProviderDelete(w, r)
	case "/api/providers/tori/discover":
		g.handleToriDiscover(w, r)
	case "/v1/router/stats":
		g.handleRouterStats(w, r)
	case "/api/breaker/reset":
		g.handleBreakerReset(w, r)
	case "/api/providers/exec":
		g.handleExecProvider(w, r)
	default:
		http.NotFound(w, r)
	}
}
