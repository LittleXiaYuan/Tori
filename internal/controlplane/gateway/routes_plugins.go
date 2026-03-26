package gateway

// registerPluginRoutes registers plugin management, skill hub, and skill market routes.
func (g *Gateway) registerPluginRoutes() {
	// Plugin CRUD
	g.mux.HandleFunc("/v1/plugins", g.requireAuth(g.handlePlugins))
	g.mux.HandleFunc("/v1/plugins/toggle", g.requireAuth(g.handlePluginToggle))
	g.mux.HandleFunc("/v1/plugins/create", g.requireAuth(g.handlePluginCreate))
	g.mux.HandleFunc("/v1/plugins/delete", g.requireAuth(g.handlePluginDelete))
	g.mux.HandleFunc("/v1/plugins/files", g.requireAuth(g.handlePluginFiles))
	g.mux.HandleFunc("/v1/plugins/ui", g.requireAuth(g.handlePluginUI))

	// Skills
	g.mux.HandleFunc("/v1/skills", g.requireAuth(g.handleSkills))

	// Skill Market
	g.mux.HandleFunc("/v1/market/search", g.requireAuth(g.handleMarketSearch))
	g.mux.HandleFunc("/v1/market/top", g.requireAuth(g.handleMarketTop))
	g.mux.HandleFunc("/v1/market/stats", g.requireAuth(g.handleMarketStats))

	// SkillHub API
	g.mux.HandleFunc("/api/skillhub/search", g.requireAuth(g.handleSkillHubSearch))
	g.mux.HandleFunc("/api/skillhub/install", g.requireAuth(g.handleSkillHubInstall))
	g.mux.HandleFunc("/api/skillhub/installed", g.requireAuth(g.handleSkillHubInstalled))
	g.mux.HandleFunc("/api/skillhub/uninstall", g.requireAuth(g.handleSkillHubUninstall))
	g.mux.HandleFunc("/api/skillhub/trending", g.requireAuth(g.handleSkillHubTrending))
	g.mux.HandleFunc("/api/skillhub/detail", g.requireAuth(g.handleSkillHubDetail))
	g.mux.HandleFunc("/api/skillhub/check-updates", g.requireAuth(g.handleSkillHubCheckUpdates))
	g.mux.HandleFunc("/api/skillhub/update", g.requireAuth(g.handleSkillHubUpdate))
	g.mux.HandleFunc("/api/skillhub/rollback", g.requireAuth(g.handleSkillHubRollback))
	g.mux.HandleFunc("/api/skillhub/versions", g.requireAuth(g.handleSkillHubVersions))
	g.mux.HandleFunc("/api/skillhub/policy", g.requireAuth(g.handleSkillHubPolicy))
	g.mux.HandleFunc("/api/skillhub/policy/check", g.requireAuth(g.handleSkillHubPolicyCheck))
	g.mux.HandleFunc("/api/skillhub/analytics", g.requireAuth(g.handleSkillHubAnalytics))
}
