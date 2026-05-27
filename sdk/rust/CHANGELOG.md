# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/).

## [0.1.0] - 2026-05-28

### Added
- Initial release
- Generated API client from OpenAPI specification using progenitor
- 425 async methods with ~19000 LOC of generated code
- Automatic regeneration on every build via build.rs
- OpenAPI 3.1 to 3.0.3 downgrade for progenitor compatibility
- Agent Kit bundle for one-stop automation composition
- State Kernel client for agent state management
- Reflection Experience client for lesson and strategy reuse
- Mission Parse client for natural-language intent parsing
- Scheduler client for prompt-based recurring jobs
- Cron System client for host-level scheduled tasks
- Memory Kernel client for host recall memory layer
- Knowledge Graph client for entity and relationship management
- Knowledge Base client for RAG knowledge repository
- LoRA lifecycle client for local brain training
- Workflow orchestration client
- Connectors runtime client for external service integration
- Notify runtime client for notification channels
- Triggers client for event-driven workflows
- Projects SDK for workspace CRUD operations
- Skill Market SDK for marketplace discovery
- Dispatch SDK for MCP worker orchestration
- Orchestrator SDK for IDE worker daemon management
- Providers SDK for LLM provider and model configuration
- Cognis SDK for CogniKernel registry and multi-cogni workflows
- Trace SDK for execution and audit trace inspection
- Heartbeat SDK for proactive lifecycle supervision
- Events SDK for Server-Sent Events streaming
- Runtime SDK for queue and event monitoring
- RuntimeQueue SDK for queue-only operations
- Subagents SDK for specialist agent orchestration
- Tools SDK for server-side tool process control
- Audit SDK for compliance and audit-chain integrity
- Trust SDK for trust governance operations
- Reverie SDK for proactive thought-loop management
- Chat SDK for chat and streaming endpoints
- Conversations SDK for conversation history and replay
- Realtime SDK for WebSocket connections
- Cost SDK for budget and usage governance
- Fork SDK for conversation branching
- Approvals SDK for approval workflow management
- RBAC SDK for role-based access control
- Files SDK for artifact management
- Browser SDK for browser automation
- SkillGrow SDK for skill-growth pattern inspection
- Review SDK for review-gate status
- Iterate SDK for self-iteration proposal review
- Persona SDK for persona identity and skills
- Tasks SDK for task CRUD and lifecycle management
- Permissions SDK for permission checks
- Reactions SDK for emoji reactions and stickers
- Instructions SDK for user instructions management
- Emotion SDK for emotion history and stickers
- Setup SDK for first-run setup and provider health
- Upload SDK for artifact upload
- Speech SDK for TTS and STT operations
- Tori SDK for Tori account management
- Backup SDK for backup and restore operations
- Settings SDK for runtime configuration
- System SDK for health probes and metrics
- Auth SDK for authentication and token exchange
- Admin SDK for operator controls
- Federation SDK for A2A federation
- Planner SDK for Planner Recovery
- IDE SDK for IDE supervisor and code review
- Discovery SDK for identity, embeddings, and search
- Persona Modes SDK for mode switching
- Bots SDK for bot management and inbox operations
- Documents SDK for document generation
- WebChat SDK for embeddable widget
- Sandbox SDK for sandbox runtime
- Router SDK for smart-router statistics
- SkillHub SDK for skill-package management
- Plugins SDK for plugin catalog and control
- Plugin API runtime helpers (LLM, search, memory, knowledge, cron, send, extensions)
- Skills SDK for runtime skills catalog
- Models SDK for model registry
- Identity SDK for cross-channel identity resolution
- Embeddings SDK for embedding providers
- Search SDK for web search
- Modes SDK for persona modes
- Interactions SDK for emotion, instructions, and reactions
- Airi Bridge client for Airi desktop pet integration
- Breaker client for circuit breaker reset
- reqwest runtime with rustls-tls
- Authentication support via custom reqwest client with headers

### Known Issues
- Streaming endpoints (SSE) generated as standard reqwest calls; use eventsource-stream for real SSE consumption
- OpenAPI 3.1 features not supported due to progenitor 0.10 limitation
- Body schemas mostly use serde_json::Value placeholders
