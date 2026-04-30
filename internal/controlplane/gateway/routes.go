package gateway

// routes.go — Consolidated API route registration.
// All endpoint registrations in one place for easy auditing.
//
// Route groups:
//   registerChatRoutes       — Chat, conversations, bots, persona, emotion, webhooks
//   registerTaskRoutes       — Tasks, workflows, missions, state, reflection, documents
//   registerMemoryRoutes     — Memory, knowledge graph, identity, embeddings, search
//   registerKnowledgeRoutes  — Knowledge base (RAG) routes
//   registerPluginRoutes     — Plugins, skills, market, skillhub
//   registerTriggerRoutes    — Triggers, cron, scheduler, tools, sandbox
//   registerSystemRoutes     — System info, settings, backup, speech, heartbeat, federation (in routes_system.go)
//   registerGovernanceRoutes — Audit, trust, iterate, review, cost, usage
//   registerProviderRoutes   — LLM providers, smart router
//   registerReverieRoutes    — Reverie (inner monologue)
//   registerBrowserRoutes    — Browser engine management, OPP
//   registerApprovalRoutes   — Human-in-the-loop approval, setup, queues, SSE, trace
//   registerRBACRoutes       — Role-based access control
//   registerModesRoutes      — Persona mode management
//   registerWorkflowRoutes   — Workflow DAG engine
//   registerIDERoutes        — IDE supervisor plugin
//   registerLoRARoutes       — LoRA scheduler, training metrics, evolution

// ──────────────────────────────────────────────
// Chat & Conversation
// ──────────────────────────────────────────────

func (g *Gateway) registerChatRoutes() {
	// Core chat
	g.mux.HandleFunc("/v1/chat", g.requireAuth(g.limiter.Middleware(g.handleChat)))
	g.mux.HandleFunc("/v1/chat/stream", g.requireAuth(g.limiter.Middleware(g.handleStreamChat)))
	g.mux.HandleFunc("/v1/chat/agentic", g.requireAuth(g.limiter.Middleware(g.handleAgenticChat)))
	g.mux.HandleFunc("/v1/ws", g.requireAuth(g.handleWebSocket))
	g.mux.HandleFunc("/v1/token", g.handleTokenGenerate)

	// Conversations
	g.mux.HandleFunc("/v1/conversations", g.requireAuth(g.handleConversations))
	g.mux.HandleFunc("/v1/conversations/messages", g.requireAuth(g.handleConversationMessages))
	g.mux.HandleFunc("/v1/conversations/manage", g.requireAuth(g.handleConversationManage))

	// Fork routes moved to forkapi sub-package

	// Subagent
	g.mux.HandleFunc("/v1/subagent", g.requireAuth(g.handleSubagent))
	g.mux.HandleFunc("/v1/subagent/message", g.requireAuth(g.handleSubagentMessage))

	// Bots
	g.mux.HandleFunc("/v1/bots", g.requireAuth(g.handleBots))
	g.mux.HandleFunc("/v1/bots/detail", g.requireAuth(g.handleBotDetail))

	// Persona
	g.mux.HandleFunc("/v1/persona", g.requireAuth(g.handlePersona))
	g.mux.HandleFunc("/v1/persona/skills", g.requireAuth(g.handlePersonaSkills))
	g.mux.HandleFunc("/v1/persona/presets", g.requireAuth(g.handlePresets))
	g.mux.HandleFunc("/v1/persona/presets/custom", g.requireAuth(g.handleCustomPreset))
	g.mux.HandleFunc("/v1/persona/presets/features", g.requireAuth(g.handlePresetFeatures))

	// Emotion
	g.mux.HandleFunc("/v1/emotion/stickers", g.requireAuth(g.handleStickers))
	g.mux.HandleFunc("/v1/emotion/history", g.requireAuth(g.handleEmotionHistory))

	// React & Sticker
	g.mux.HandleFunc("/v1/react", g.requireAuth(g.handleReact))
	g.mux.HandleFunc("/v1/sticker/send", g.requireAuth(g.handleSendSticker))

	// Channel Groups
	g.mux.HandleFunc("/v1/channels/groups", g.requireAuth(g.handleChannelGroups))

	// Inbox
	g.mux.HandleFunc("/v1/inbox", g.requireAuth(g.handleInbox))
	g.mux.HandleFunc("/v1/inbox/read", g.requireAuth(g.handleInboxRead))

	// Webhooks
	g.mux.HandleFunc("/webhook/feishu", g.handleFeishuWebhook)

	// WebChat widget (public — no auth, to allow embedding)
	g.mux.HandleFunc("/v1/webchat/widget.js", g.handleWebChatWidget)
}

// ──────────────────────────────────────────────
// Memory & Knowledge
// ──────────────────────────────────────────────

func (g *Gateway) registerMemoryRoutes() {
	// Memory
	g.mux.HandleFunc("/v1/memory/stats", g.requireAuth(g.handleMemoryStats))
	g.mux.HandleFunc("/v1/memory/search", g.requireAuth(g.handleMemorySearch))
	g.mux.HandleFunc("/v1/memory/add", g.requireAuth(g.handleMemoryAdd))
	g.mux.HandleFunc("/v1/memory/compact", g.requireAuth(g.handleMemoryCompact))

	// Persona & Editable Memory
	g.mux.HandleFunc("/v1/memory/persona", g.requireAuth(g.handleMemoryPersonaGet))
	g.mux.HandleFunc("/v1/memory/update", g.requireAuth(g.handleMemoryPersonaUpdate))

	// Knowledge Graph
	g.mux.HandleFunc("/v1/graph/entities", g.requireAuth(g.handleGraphEntities))
	g.mux.HandleFunc("/v1/graph/relations", g.requireAuth(g.handleGraphRelations))
	g.mux.HandleFunc("/v1/graph/context", g.requireAuth(g.handleGraphContext))
	g.mux.HandleFunc("/v1/graph/stats", g.requireAuth(g.handleGraphStats))

	// Identity
	g.mux.HandleFunc("/v1/identity/resolve", g.requireAuth(g.handleIdentityResolve))
	g.mux.HandleFunc("/v1/identity/profiles", g.requireAuth(g.handleIdentityProfiles))

	// Embeddings
	g.mux.HandleFunc("/v1/embeddings", g.requireAuth(g.handleEmbeddings))

	// Search
	g.mux.HandleFunc("/v1/search", g.requireAuth(g.handleSearch))
	g.mux.HandleFunc("/v1/search/providers", g.requireAuth(g.handleSearchProviders))
}

func (g *Gateway) registerKnowledgeRoutes() {
	g.mux.HandleFunc("/v1/knowledge/search", g.requireAuth(g.handleKBSearch))
	g.mux.HandleFunc("/v1/knowledge/sources", g.requireAuth(g.handleKBSources))
	g.mux.HandleFunc("/v1/knowledge/stats", g.requireAuth(g.handleKBStats))
	g.mux.HandleFunc("/v1/knowledge/upload", g.requireAuth(g.handleKBUpload))
	g.mux.HandleFunc("/v1/knowledge/ingest", g.requireAuth(g.handleKBIngest))
	g.mux.HandleFunc("/v1/knowledge/import-url", g.requireAuth(g.handleKBImportURL))
	g.mux.HandleFunc("/v1/knowledge/import-repo", g.requireAuth(g.handleKBImportRepo))
	g.mux.HandleFunc("/v1/knowledge/source", g.requireAuth(g.handleKBDelete))
	g.mux.HandleFunc("/v1/knowledge/source/update", g.requireAuth(g.handleKBUpdate))
}

// ──────────────────────────────────────────────
// Plugins & Skills
// ──────────────────────────────────────────────

func (g *Gateway) registerPluginRoutes() {
	// Plugin CRUD
	g.mux.HandleFunc("/v1/plugins", g.requireAuth(g.handlePlugins))
	g.mux.HandleFunc("/v1/plugins/toggle", g.requireAuth(g.handlePluginToggle))
	g.mux.HandleFunc("/v1/plugins/create", g.requireAuth(g.handlePluginCreate))
	g.mux.HandleFunc("/v1/plugins/delete", g.requireAuth(g.handlePluginDelete))
	g.mux.HandleFunc("/v1/plugins/files", g.requireAuth(g.handlePluginFiles))
	g.mux.HandleFunc("/v1/plugins/ui", g.requireAuth(g.handlePluginUI))
	g.mux.HandleFunc("/v1/plugins/reload", g.requireAuth(g.handlePluginReload))
	g.mux.HandleFunc("/v1/plugins/open-folder", g.requireAuth(g.handlePluginOpenFolder))

	// Skills
	g.mux.HandleFunc("/v1/skills", g.requireAuth(g.handleSkills))
	g.mux.HandleFunc("/v1/skills/scan", g.requireAuth(g.handleSkillsScan))
	g.mux.HandleFunc("/v1/skills/dynamic", g.requireAuth(g.handleSkillsDynamicGet))
	g.mux.HandleFunc("/v1/skills/approve", g.requireAuth(g.handleSkillsDynamicApprove))
	g.mux.HandleFunc("/v1/skills/reject", g.requireAuth(g.handleSkillsDynamicReject))

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
	g.mux.HandleFunc("/v1/skill-suggestions", g.requireAuth(g.handleSkillSuggestions))
}

// ──────────────────────────────────────────────
// Triggers & Automation
// ──────────────────────────────────────────────

func (g *Gateway) registerTriggerRoutes() {
	// Triggers (legacy)
	g.mux.HandleFunc("/v1/triggers", g.requireAuth(g.handleTriggers))
	g.mux.HandleFunc("/v1/triggers/emit", g.requireAuth(g.handleTriggerEmit))

	// Triggers v2 (unified)
	g.mux.HandleFunc("/v1/triggers/v2", g.requireAuth(g.handleTriggersV2))
	g.mux.HandleFunc("/v1/triggers/v2/emit", g.requireAuth(g.handleTriggersV2Emit))
	g.mux.HandleFunc("/v1/triggers/v2/runs", g.requireAuth(g.handleTriggersV2Runs))
	g.mux.HandleFunc("/v1/triggers/v2/events", g.requireAuth(g.handleTriggersV2Events))

	// Cron
	g.mux.HandleFunc("/v1/cron/list", g.requireAuth(g.handleCronList))
	g.mux.HandleFunc("/v1/cron/add", g.requireAuth(g.handleCronAdd))
	g.mux.HandleFunc("/v1/cron/remove", g.requireAuth(g.handleCronRemove))
	g.mux.HandleFunc("/v1/cron/run", g.requireAuth(g.handleCronRun))

	// Scheduler routes moved to schedulerapi sub-package

	// Tools (process execution)
	g.mux.HandleFunc("/v1/tools/exec", g.requireAuth(g.handleToolExec))
	g.mux.HandleFunc("/v1/tools/list", g.requireAuth(g.handleToolList))
	g.mux.HandleFunc("/v1/tools/poll", g.requireAuth(g.handleToolPoll))
	g.mux.HandleFunc("/v1/tools/kill", g.requireAuth(g.handleToolKill))

	// Sandbox (admin-only — arbitrary command execution)
	g.mux.HandleFunc("/v1/sandbox/exec", g.requireAuth(g.requireAdmin(g.handleSandboxExec)))
	g.mux.HandleFunc("/v1/sandbox/probe", g.requireAuth(g.handleSandboxProbe))
	g.mux.HandleFunc("/v1/sandbox/desktop", g.requireAuth(g.requireAdmin(g.handleDesktopCreate)))
	g.mux.HandleFunc("/v1/sandbox/desktop/status", g.requireAuth(g.handleDesktopStatus))
	g.mux.HandleFunc("/v1/sandbox/desktop/destroy", g.requireAuth(g.handleDesktopDestroy))

	// Agent output files
	g.mux.HandleFunc("/api/files", g.requireAuth(g.handleFileList))
	g.mux.HandleFunc("/api/files/download", g.requireAuth(g.handleFileDownload))
}

// ──────────────────────────────────────────────
// Governance & Audit
// ──────────────────────────────────────────────

func (g *Gateway) registerGovernanceRoutes() {
	// Audit
	g.mux.HandleFunc("/v1/audit/tail", g.requireAuth(g.handleAuditTail))
	g.mux.HandleFunc("/v1/audit/verify", g.requireAuth(g.handleAuditVerify))
	g.mux.HandleFunc("/v1/audit/stats", g.requireAuth(g.handleAuditStats))
	g.mux.HandleFunc("/api/audit/trail", g.requireAuth(g.handleAuditTrail))

	// Trust
	g.mux.HandleFunc("/api/trust/scores", g.requireAuth(g.handleTrustScores))
	g.mux.HandleFunc("/api/trust/reset", g.requireAuth(g.handleTrustReset))
	g.mux.HandleFunc("/api/trust/grant", g.requireAuth(g.handleTrustGrant))

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

	// Cost routes moved to costapi sub-package

	// Usage / Quota
	g.mux.HandleFunc("/v1/usage", g.requireAuth(g.handleUsage))
	g.mux.HandleFunc("/v1/quota", g.requireAuth(g.handleSetQuota))
}

// ──────────────────────────────────────────────
// LLM Providers & Router
// ──────────────────────────────────────────────

func (g *Gateway) registerProviderRoutes() {
	g.mux.HandleFunc("/v1/models", g.requireAuth(g.handleModels))
	g.mux.HandleFunc("/api/providers", g.requireAuth(g.handleProviderList))
	g.mux.HandleFunc("/api/providers/test", g.requireAuth(g.handleProviderTest))
	g.mux.HandleFunc("/api/providers/enable", g.requireAuth(g.handleProviderEnable))
	g.mux.HandleFunc("/api/providers/disable", g.requireAuth(g.handleProviderDisable))
	g.mux.HandleFunc("/api/providers/switch-model", g.requireAuth(g.handleProviderSwitchModel))
	g.mux.HandleFunc("/api/providers/session", g.requireAuth(g.handleProviderSessionOverride))
	g.mux.HandleFunc("/api/providers/local/discover", g.requireAuth(g.handleLocalDiscover))
	g.mux.HandleFunc("/api/providers/local/register", g.requireAuth(g.handleLocalRegister))
	g.mux.HandleFunc("/api/providers/mode", g.requireSetupOrAuth(g.handleProviderMode))
	g.mux.HandleFunc("/api/providers/presets", g.requireSetupOrAuth(g.handleProviderPresets))
	g.mux.HandleFunc("/api/providers/register", g.requireSetupOrAuth(g.handleProviderRegister))
	g.mux.HandleFunc("/api/providers/delete", g.requireAuth(g.handleProviderDelete))
	g.mux.HandleFunc("/api/providers/tori/discover", g.requireAuth(g.handleToriDiscover))
	g.mux.HandleFunc("/v1/router/stats", g.requireAuth(g.handleRouterStats))
	g.mux.HandleFunc("/api/breaker/reset", g.requireAuth(g.handleBreakerReset))
	g.mux.HandleFunc("/api/providers/exec", g.requireAuth(g.handleExecProvider))
}

// ──────────────────────────────────────────────
// Reverie (Inner Monologue)
// ──────────────────────────────────────────────

func (g *Gateway) registerReverieRoutes() {
	g.mux.HandleFunc("/v1/reverie/journal", g.requireAuth(g.handleReverieJournal))
	g.mux.HandleFunc("/v1/reverie/stats", g.requireAuth(g.handleReverieStats))
	g.mux.HandleFunc("/v1/reverie/config", g.requireAuth(g.handleReverieConfig))
	g.mux.HandleFunc("/v1/reverie/think", g.requireAuth(g.handleReverieThink))
	g.mux.HandleFunc("/v1/reverie/thought", g.requireAuth(g.handleReverieDeleteThought))
	g.mux.HandleFunc("/v1/reverie/targets", g.requireAuth(g.handleReverieTargets))
	g.mux.HandleFunc("/v1/reverie/actions", g.requireAuth(g.handleReverieActions))
}

// ──────────────────────────────────────────────
// RBAC, Approval & Safety
// ──────────────────────────────────────────────

func (g *Gateway) registerRBACRoutes() {
	g.mux.HandleFunc("/v1/rbac/roles", g.requireAuth(g.requireAdmin(g.handleRBACRolesSwitch)))
	g.mux.HandleFunc("/v1/rbac/assign", g.requireAuth(g.requireAdmin(g.handleRBACAssign)))
	g.mux.HandleFunc("/v1/rbac/revoke", g.requireAuth(g.requireAdmin(g.handleRBACRevoke)))
	g.mux.HandleFunc("/v1/rbac/check", g.requireAuth(g.handleRBACCheck))
	g.mux.HandleFunc("/v1/rbac/my-roles", g.requireAuth(g.handleRBACMyRoles))
}

func (g *Gateway) registerApprovalRoutes() {
	g.mux.HandleFunc("/v1/approvals", g.requireAuth(g.handleApprovalRouteSwitch))
	g.mux.HandleFunc("/v1/approvals/approve", g.requireAuth(g.handleApprovalApprove))
	g.mux.HandleFunc("/v1/approvals/deny", g.requireAuth(g.handleApprovalDeny))
	g.mux.HandleFunc("/v1/approvals/decide", g.requireAuth(g.handleApprovalDecide))
	g.mux.HandleFunc("/v1/approvals/rules", g.requireAuth(g.handleApprovalRules))
}

func (g *Gateway) registerSetupRoutes() {
	g.mux.HandleFunc("/v1/setup/detect", g.requireSetupOrAuth(g.handleSetupDetect))
	g.mux.HandleFunc("/v1/setup/health", g.requireSetupOrAuth(g.handleSetupHealth))
	g.mux.HandleFunc("/v1/setup/templates", g.requireSetupOrAuth(g.handleSetupTemplates))
	g.mux.HandleFunc("/v1/setup/test-provider", g.requireSetupOrAuth(g.handleSetupTestProvider))
	g.mux.HandleFunc("/v1/setup/apply", g.requireSetupOrAuth(g.handleSetupApply))
	g.mux.HandleFunc("/v1/setup/install-component", g.requireSetupOrAuth(g.handleInstallComponent))
}

func (g *Gateway) registerQueueRoutes() {
	g.mux.HandleFunc("/v1/sessions/queue", g.requireAuth(g.handleSessionQueue))
	g.mux.HandleFunc("/v1/sessions/queue/cancel", g.requireAuth(g.handleSessionQueueCancel))
}

func (g *Gateway) registerSSERoutes() {
	g.mux.HandleFunc("/v1/events/stream", g.requireAuth(g.handleSSEStream))
}

func (g *Gateway) registerTraceRoutes() {
	g.mux.HandleFunc("/v1/trace/recent", g.requireAuth(g.handleTraceRecent))
	g.mux.HandleFunc("/v1/trace/task/", g.requireAuth(g.handleTraceByTask))
	g.mux.HandleFunc("/v1/trace/", g.requireAuth(g.handleTraceByID))
}

// ──────────────────────────────────────────────
// Browser & IDE
// ──────────────────────────────────────────────

func (g *Gateway) registerBrowserRoutes() {
	g.mux.HandleFunc("/v1/browser/status", g.requireAuth(g.handleBrowserStatus))
	g.mux.HandleFunc("/v1/browser/config", g.requireAuth(g.handleBrowserConfig))
	g.mux.HandleFunc("/v1/browser/navigate", g.requireAuth(g.handleBrowserNavigate))
	g.mux.HandleFunc("/v1/browser/screenshot", g.requireAuth(g.handleBrowserScreenshot))
	g.mux.HandleFunc("/v1/browser/ocr", g.requireAuth(g.handleBrowserOCR))
	g.mux.HandleFunc("/v1/browser/screenshot/latest", g.requireAuth(g.handleBrowserScreenshotLatest))
	g.mux.HandleFunc("/v1/browser/opp/pending", g.requireAuth(g.handleOPPPending))
	g.mux.HandleFunc("/v1/browser/opp/decide", g.requireAuth(g.handleOPPDecide))

	// Browser Extension (Connector) API
	g.mux.HandleFunc("/api/browser/ext/status", g.requireAuth(g.handleBrowserExtStatus))
	g.mux.HandleFunc("/api/browser/ext/session", g.requireBrowserSessionAuth(g.handleBrowserExtSession))
	g.mux.HandleFunc("/api/browser/ext/action", g.requireAuth(g.handleBrowserExtAction))
	g.mux.HandleFunc("/api/browser/ext/scenarios", g.requireAuth(g.handleBrowserScenarios))
	g.mux.HandleFunc("/api/browser/ext/scenarios/run", g.requireAuth(g.handleBrowserRunScenario))
}

// Connector and Notify routes are registered via sub-packages
// (connectorapi, notifyapi) in gateway.go routes().

func (g *Gateway) registerIDERoutes() {
	g.mux.HandleFunc("/v1/ide/review", g.requireAuth(g.handleIDEReviewCode))
	g.mux.HandleFunc("/v1/ide/status", g.requireAuth(g.handleIDEStatus))
}

// ──────────────────────────────────────────────
// Persona Modes
// ──────────────────────────────────────────────

func (g *Gateway) registerModesRoutes() {
	g.mux.HandleFunc("/v1/persona/modes", g.requireAuth(g.handleListModes))
	g.mux.HandleFunc("/v1/persona/mode", g.requireAuth(g.handleSetMode))
	g.mux.HandleFunc("/v1/persona/mode/current", g.requireAuth(g.handleCurrentMode))
}

// Workflow routes moved to workflowapi sub-package.

// LoRA and Cost routes are registered via sub-packages (loraapi, costapi)
// in gateway.go routes() — see the "Extracted handler groups" section.
