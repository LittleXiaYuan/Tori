# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/).

## [0.1.0] - 2026-05-28

### Added
- Initial release
- Generated API client from OpenAPI specification using @hey-api/openapi-ts
- 140+ focused subpath exports for selective imports
- Zero-dependency runtime using @hey-api/client-fetch with native fetch
- Tree-shakeable package with sideEffects: false
- Agent Kit bundle for one-stop automation composition
- Workload preset metadata for selectable workload catalog
- Packs client with catalog source validation and capability preparation helpers
- Planner Recovery client with checkpoint and resume-plan helpers
- Planner client facade with read, control, checkpoints, resume, and execution state
- Chat client with basic, agentic, and streaming support
- Cognis client with registry, observe, traces, health, alerts, experience, evolution, federation, workflows, and bundles
- CogniSDK schema and package manifest normalization helpers
- Events client with streaming and parsing support
- Realtime client with WebSocket connection and message helpers
- WebChat client with widget and embed snippet generation
- Conversations client with read, control, sessions, messages, replay, and management
- Subagents client with read and control operations
- Bots client with read, control, list, detail, inbox, and channels
- Discovery client with identity, embeddings, and search
- Identity, Embeddings, and Search standalone clients
- Interactions client with emotion, reactions, and instructions
- Emotion client with history and stickers
- Reactions and Instructions standalone clients
- RBAC client with roles, bindings, and permissions
- Roles, RoleBindings, MyRoles, and Permissions standalone clients
- Memory client with search, stats, add, and compact operations
- Tasks client with context, observe, templates, threads, gaps, memory, lifecycle, read, create, and delete
- Knowledge client with search, ingest, sources, import, and upload
- Providers client with registry, control, mode, session, health, and breaker
- Breaker standalone client for circuit breaker reset
- Models client for model registry management
- Setup client with detect, templates, provider, and install operations
- Documents client with templates and generation (DOCX, XLSX, PPTX, HTML)
- Approvals client with decisions, queue, pending, history, and rules
- Trace client with events, recent, by-id, and task trace
- Browser client with status, capture, OPP, and extension support
- SBOM Drift client with CI baseline and writeback plan helpers
- Runtime client with queue and events operations
- RuntimeQueue client for queue-only operations
- Router client for smart-router statistics
- Modes client with observe and control operations
- IDE client with status and review operations
- Persona client with state, skills, and presets
- Workflow client with definitions, runs, read, write, run, and instances
- Cost client with budget, alerts, observe, task, and history
- Usage client for usage tracking
- LoRA client with observe, status, history, control, config, preview, evolution, trigger, and rollback
- Iterate client with review, pending, decisions, and cycle operations
- Trust client with control operations
- Review and SkillGrow standalone clients
- Audit client with chain, tail, verify, and trail operations
- Heartbeat client with observe and control operations
- Reverie client with observe and control operations
- Federation client with observe, peers, stats, capabilities, control, and delegate
- System client with probes and ops operations
- Settings client with config, backup, schema, and runtime operations
- Tori client with observe and bind operations
- Speech client with TTS, STT, and voices operations
- Admin client with desktop, tenants, and config operations
- Files client with read, list, preview, and download operations
- Cron client with read and control operations
- SkillHub client with catalog, install, updates, installed, versions, and policy
- Skills client with catalog, scan, dynamic, and suggestions
- Plugins client with catalog, control, toggle, UI, reload, folder, files, CRUD, create, and delete
- Connectors client with catalog, auth, actions, list, detail, connect, and disconnect
- Notify client with share and channels operations
- Projects client with read, list, detail, and write operations
- Skill Market client with search, query, top, and stats
- Dispatch client with read, workers, queue, worker config, and control
- Orchestrator client with read, status, events, and control
- Fork client with read, root, list, and control operations
- Scheduler client with read and control operations
- Upload client for artifact upload
- Graph client with read, entities, relations, context, stats, and write operations
- Plugin API client with LLM, search, memory, agent memory, knowledge, cron, send, and extensions
- Plugin subclients for LLM, search, memory (read/write), agent memory (search/write), knowledge (search/ingest), cron (read/control), send, and extensions (list/register)
- State client with snapshot, actions, capabilities, resource state, focus state, and goal state
- Triggers client with legacy, read, control, definitions, history, and emit operations
- Missions client with parse operation
- Reflect client with experiences and strategies
- Tools client for server-side tool process control
- Sandbox client for sandbox runtime operations
- Airi client for Airi desktop pet bridge
- Auth client for authentication and token exchange
- Backup client for backup and restore operations

### Features
- Focused subpath imports for minimal bundle size
- TypeScript type definitions for all API operations
- Automatic tree-shaking support
- Workload-oriented SDK architecture
- Comprehensive error handling
- Native fetch-based HTTP client
