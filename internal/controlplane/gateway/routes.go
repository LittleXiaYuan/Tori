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
	// Core chat. guardNoOfflineRole hard-blocks (403) any front-stage request
	// that tries to target the offline background engine (小羽 / RWKV-7), so its
	// latency never leaks into the user-facing path.
	g.mux.HandleFunc("/v1/chat", g.requireAuth(g.limiter.Middleware(g.guardNoOfflineRole(g.handleChat))))
	g.mux.HandleFunc("/v1/chat/stream", g.requireAuth(g.limiter.Middleware(g.guardNoOfflineRole(g.handleStreamChat))))
	g.mux.HandleFunc("/v1/chat/agentic", g.requireAuth(g.limiter.Middleware(g.guardNoOfflineRole(g.handleAgenticChat))))
	g.mux.HandleFunc("/v1/chat/starter-suggestions", g.requireAuth(g.handleStarterSuggestions))
	g.mux.HandleFunc("/v1/ws", g.requireAuth(g.handleWebSocket))
	g.mux.HandleFunc("/v1/token", g.handleTokenGenerate)

	// Conversations
	g.mux.HandleFunc("/v1/conversations", g.requireAuth(g.handleConversations))
	g.mux.HandleFunc("/v1/conversations/messages", g.requireAuth(g.handleConversationMessages))
	g.mux.HandleFunc("/v1/conversations/manage", g.requireAuth(g.handleConversationManage))
	g.mux.HandleFunc("/v1/conversations/replay", g.requireAuth(g.handleConversationReplay))

	// Fork routes moved to forkapi sub-package

	// Subagent
	g.mux.HandleFunc("/v1/subagent", g.requireAuth(g.handleSubagent))
	g.mux.HandleFunc("/v1/subagent/message", g.requireAuth(g.handleSubagentMessage))

	// Bots routes migrated to the control-plane pack (internal/packs/controlplane).

	// Persona
	g.mux.HandleFunc("/v1/persona", g.requireAuth(g.handlePersona))
	g.mux.HandleFunc("/v1/persona/skills", g.requireAuth(g.handlePersonaSkills))
	g.mux.HandleFunc("/v1/persona/presets", g.requireAuth(g.handlePresets))
	g.mux.HandleFunc("/v1/persona/presets/custom", g.requireAuth(g.handleCustomPreset))
	g.mux.HandleFunc("/v1/persona/presets/features", g.requireAuth(g.handlePresetFeatures))

	// Emotion (/v1/emotion/stickers, /v1/emotion/history) are owned by the emotion
	// pack (internal/packs/emotion), mounted via gw.RegisterModule in
	// cmd/agent/init_task_engine.go. The sticker-map + history logic lives in that
	// pack natively; the gateway no longer hosts these routes.

	// User Instructions (/v1/instructions, /v1/instructions/reorder) are owned by
	// the instructions pack (internal/packs/instructions), mounted via
	// gw.RegisterModule in cmd/agent/init_task_engine.go. The CRUD + reorder
	// logic lives in that pack natively; the gateway no longer hosts these routes.

	// React & Sticker
	g.mux.HandleFunc("/v1/react", g.requireAuth(g.handleReact))
	g.mux.HandleFunc("/v1/sticker/send", g.requireAuth(g.handleSendSticker))

	// Channel Groups
	g.mux.HandleFunc("/v1/channels/groups", g.requireAuth(g.handleChannelGroups))

	// Inbox routes migrated to the control-plane pack (internal/packs/controlplane).

	// Webhooks
	g.mux.HandleFunc("/webhook/feishu", g.handleFeishuWebhook)

	// WebChat widget (public — no auth, to allow embedding)
	g.mux.HandleFunc("/v1/webchat/widget.js", g.handleWebChatWidget)
}

// ──────────────────────────────────────────────
// Memory & Knowledge
// ──────────────────────────────────────────────

func (g *Gateway) registerMemoryRoutes() {
	// Memory (/v1/memory/*) is now mounted as a Pack Runtime backend module
	// (internal/packs/memory), so toggling yunque.pack.memory enables/disables it
	// at runtime. Registering them here too would panic the mux on duplicate
	// patterns. Graph / identity / embeddings / search keep their own gateway
	// routes below.

	// Knowledge Graph (/v1/graph/{entities,relations,context,stats}) is owned by
	// the graph pack (internal/packs/graph), mounted via gw.RegisterModule in
	// cmd/agent/init_task_engine.go. The graph CRUD/context/stats logic lives in
	// that pack natively (reading the memory pipeline's graph); the gateway no
	// longer hosts these routes.

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
	// Knowledge (RAG) routes are now mounted as a Pack Runtime backend module
	// (internal/packs/knowledge), so toggling yunque.pack.knowledge enables or
	// disables the surface at runtime. Registering them here too would panic the
	// mux on duplicate patterns.
}

// ──────────────────────────────────────────────
// Plugins & Skills
// ──────────────────────────────────────────────

func (g *Gateway) registerPluginRoutes() {
	// Plugin CRUD routes migrated to the control-plane pack
	// (internal/packs/controlplane). SkillHub / market keep their own routes below.

	// Skills (/v1/skills/*) are now mounted as a Pack Runtime backend module
	// (internal/packs/skills), so toggling yunque.pack.skills enables/disables
	// the skill-management surface at runtime. Registering them here too would
	// panic the mux on duplicate patterns. SkillHub and the skill market below
	// keep their own gateway routes for now.

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
	// Trigger routes (/v1/triggers and /v1/triggers/v2*) are owned by the triggers
	// pack (internal/packs/triggers), mounted via gw.RegisterModule in
	// cmd/agent/init_task_engine.go.

	// Cron routes (/v1/cron/*) are owned by the cron pack (internal/packs/cron),
	// mounted via gw.RegisterModule in cmd/agent/init_task_engine.go.

	// Scheduler routes moved to schedulerapi sub-package

	// Tools (process execution) migrated to the control-plane pack
	// (internal/packs/controlplane). Sandbox routes below stay admin-gated here.

	// Sandbox (admin-only — arbitrary command execution)
	g.mux.HandleFunc("/v1/sandbox/exec", g.requireAuth(g.requireAdmin(g.handleSandboxExec)))
	g.mux.HandleFunc("/v1/sandbox/probe", g.requireAuth(g.requireAdmin(g.handleSandboxProbe)))
	g.mux.HandleFunc("/v1/sandbox/desktop", g.requireAuth(g.requireAdmin(g.handleDesktopCreate)))
	g.mux.HandleFunc("/v1/sandbox/desktop/status", g.requireAuth(g.handleDesktopStatus))
	g.mux.HandleFunc("/v1/sandbox/desktop/destroy", g.requireAuth(g.handleDesktopDestroy))

	// Agent output files (/api/files, /api/files/preview, /api/files/download)
	// are owned by the files pack (internal/packs/files), mounted via
	// gw.RegisterModule in cmd/agent/init_task_engine.go. The handler logic lives
	// in that pack natively; the gateway no longer hosts these routes.
}

// ──────────────────────────────────────────────
// Governance & Audit
// ──────────────────────────────────────────────

func (g *Gateway) registerGovernanceRoutes() {
	// Governance routes (audit / trust / iterate / review / skillgrow / usage)
	// migrated to the control-plane pack (internal/packs/controlplane), mounted
	// via gw.RegisterModule(controlplanepack.NewHandler(gw)) in
	// cmd/agent/init_task_engine.go. Cost routes are owned by the cost pack.
}

// ──────────────────────────────────────────────
// LLM Providers & Router
// ──────────────────────────────────────────────

func (g *Gateway) registerProviderRoutes() {
	// Most provider/model routes (requireAuth) migrated to the control-plane pack
	// (internal/packs/controlplane). The setup-flow routes below stay direct
	// because they are requireSetupOrAuth and must remain reachable during
	// onboarding without depending on the pack-enabled gate.
	g.mux.HandleFunc("/api/providers/mode", g.requireSetupOrAuth(g.handleProviderMode))
	g.mux.HandleFunc("/api/providers/presets", g.requireSetupOrAuth(g.handleProviderPresets))
	g.mux.HandleFunc("/api/providers/register", g.requireSetupOrAuth(g.handleProviderRegister))
}

// ──────────────────────────────────────────────
// Reverie (Inner Monologue)
// ──────────────────────────────────────────────

func (g *Gateway) registerReverieRoutes() {
	// The reverie inner-monologue surface (/v1/reverie/{journal,stats,config,
	// think,thought,targets,actions}) is owned by the reverie pack
	// (internal/packs/reverie), mounted via gw.RegisterModule in
	// cmd/agent/init_task_engine.go. Only the two routes below stay on the
	// gateway: dream/status is observability, and the cognitive-layer switch is
	// admin-gated (pack routes only run through requireAuth, not requireAdmin).

	// Offline dream loop + self-evolution status (read-only observability).
	g.mux.HandleFunc("/v1/reverie/dream/status", g.requireAuth(g.handleDreamStatus))
	// Cognitive-layer master switch — runtime hot-toggle (admin-only). GET reads
	// state; POST {"enabled":bool} flips the whole cognitive stack without restart.
	g.mux.HandleFunc("/v1/cognitive-layer", g.requireAuth(g.requireAdmin(g.handleCognitiveLayer)))
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
	// Approval (human-in-the-loop) routes migrated to the control-plane pack
	// (internal/packs/controlplane).
}

func (g *Gateway) registerSetupRoutes() {
	g.mux.HandleFunc("/v1/setup/detect", g.requireSetupOrAuth(g.handleSetupDetect))
	g.mux.HandleFunc("/v1/setup/health", g.requireSetupOrAuth(g.handleSetupHealth))
	g.mux.HandleFunc("/v1/setup/templates", g.requireSetupOrAuth(g.handleSetupTemplates))
	g.mux.HandleFunc("/v1/setup/test-provider", g.requireSetupOrAuth(g.handleSetupTestProvider))
	g.mux.HandleFunc("/v1/setup/apply", g.requireSetupOrAuth(g.handleSetupApply))
	g.mux.HandleFunc("/v1/setup/install-component", g.requireSetupOrAuth(g.handleInstallComponent))
	g.mux.HandleFunc("/v1/onboarding/state", g.requireSetupOrAuth(g.handleOnboardingState))
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
	// Browser Intent HTTP surfaces are mounted as an optional Pack Runtime backend
	// module (internal/packs/browserintent). The WebSocket endpoint remains on
	// Gateway because it is attached when SetBrowserHub wires a concrete hub.
}

// Notification routes are owned by the notifications pack
// (internal/packs/notifications). Connector routes are owned by the connectors
// pack (internal/packs/connectors).

// IDE routes (/v1/ide/*) are owned by the IDE pack (internal/packs/ide),
// mounted via gw.RegisterModule in cmd/agent/init_task_engine.go.

// Persona-mode routes (/v1/persona/mode*) are now owned by the persona-modes
// pack (internal/packs/modes), mounted via gw.RegisterModule in
// cmd/agent/init_task_engine.go. The gateway only exposes ModeManager().

// Workflow routes moved to workflowapi sub-package.

// LoRA and cost routes are mounted as Pack Runtime backend modules
// (internal/packs/lora, internal/packs/cost).
