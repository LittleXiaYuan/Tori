package gateway

// registerGovernanceRoutes registers audit, trust, iterate, review, cost, and usage routes.
func (g *Gateway) registerGovernanceRoutes() {
	// Audit
	g.mux.HandleFunc("/v1/audit/tail", g.requireAuth(g.handleAuditTail))
	g.mux.HandleFunc("/v1/audit/verify", g.requireAuth(g.handleAuditVerify))
	g.mux.HandleFunc("/v1/audit/stats", g.requireAuth(g.handleAuditStats))
	g.mux.HandleFunc("/api/audit/trail", g.requireAuth(g.handleAuditTrail))

	// Trust
	g.mux.HandleFunc("/api/trust/scores", g.requireAuth(g.handleTrustScores))
	g.mux.HandleFunc("/api/trust/reset", g.requireAuth(g.handleTrustReset))

	// Iterate (self-improvement)
	g.mux.HandleFunc("/api/iterate/proposals", g.requireAuth(g.handleIterateProposals))
	g.mux.HandleFunc("/api/iterate/approve", g.requireAuth(g.handleIterateApprove))
	g.mux.HandleFunc("/api/iterate/reject", g.requireAuth(g.handleIterateReject))
	g.mux.HandleFunc("/api/iterate/trigger", g.requireAuth(g.handleIterateTrigger))
	g.mux.HandleFunc("/api/iterate/status", g.requireAuth(g.handleIterateStatus))

	// Review
	g.mux.HandleFunc("/api/review/status", g.requireAuth(g.handleReviewStatus))

	// Skill Grow
	g.mux.HandleFunc("/api/skillgrow/patterns", g.requireAuth(g.handleSkillGrowPatterns))

	// Cost tracking
	g.mux.HandleFunc("/v1/cost/summary", g.requireAuth(g.handleCostSummary))
	g.mux.HandleFunc("/v1/cost/budget", g.requireAuth(g.handleCostBudget))
	g.mux.HandleFunc("/v1/cost/task", g.requireAuth(g.handleCostByTask))
	g.mux.HandleFunc("/v1/cost/task/timeline", g.requireAuth(g.handleCostTaskTimeline))
	g.mux.HandleFunc("/v1/cost/breakdown", g.requireAuth(g.handleCostBreakdown))
	g.mux.HandleFunc("/v1/cost/history", g.requireAuth(g.handleCostHistory))
	g.mux.HandleFunc("/v1/cost/alerts", g.requireAuth(g.handleCostAlerts))

	// Usage / Quota
	g.mux.HandleFunc("/v1/usage", g.requireAuth(g.handleUsage))
	g.mux.HandleFunc("/v1/quota", g.requireAuth(g.handleSetQuota))
}
