package gateway

// registerChatRoutes registers chat, conversation, session, and real-time routes.
func (g *Gateway) registerChatRoutes() {
	g.mux.HandleFunc("/v1/chat", g.requireAuth(g.limiter.Middleware(g.handleChat)))
	g.mux.HandleFunc("/v1/chat/stream", g.requireAuth(g.limiter.Middleware(g.handleStreamChat)))
	g.mux.HandleFunc("/v1/chat/agentic", g.requireAuth(g.limiter.Middleware(g.handleAgenticChat)))
	g.mux.HandleFunc("/v1/ws", g.requireAuth(g.handleWebSocket))
	g.mux.HandleFunc("/v1/token", g.handleTokenGenerate)

	// Conversations
	g.mux.HandleFunc("/v1/conversations", g.requireAuth(g.handleConversations))
	g.mux.HandleFunc("/v1/conversations/messages", g.requireAuth(g.handleConversationMessages))
	g.mux.HandleFunc("/v1/conversations/manage", g.requireAuth(g.handleConversationManage))

	// Fork
	g.mux.HandleFunc("/v1/fork", g.requireAuth(g.handleFork))
	g.mux.HandleFunc("/v1/fork/branch", g.requireAuth(g.handleForkBranch))
	g.mux.HandleFunc("/v1/fork/list", g.requireAuth(g.handleForkList))

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
