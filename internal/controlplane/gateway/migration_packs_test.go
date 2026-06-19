package gateway

// Test support for the pack-route migration (docs/spec/pack-route-migration-plan.md).
//
// The bridge migration moved /v1/{knowledge,memory,skills,tasks,projects} from
// direct gateway routes into Pack Runtime backend modules. Production registers
// those packs in cmd/agent/init_task_engine.go, so the routes still exist and
// are auth-gated. The shared test gateways must do the same; otherwise migrated
// paths fall through to the SPA catch-all and return 200, breaking auth/route
// assertions. This helper makes the test gateways production-faithful.

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"yunque-agent/internal/agentcore/costtrack"
	"yunque-agent/internal/controlplane/tenant"
	channelspack "yunque-agent/internal/packs/channels"
	cognitivelayerpack "yunque-agent/internal/packs/cognitivelayer"
	connectorspack "yunque-agent/internal/packs/connectors"
	controlplanepack "yunque-agent/internal/packs/controlplane"
	costpack "yunque-agent/internal/packs/cost"
	cronpack "yunque-agent/internal/packs/cron"
	desktoppack "yunque-agent/internal/packs/desktop"
	documentspack "yunque-agent/internal/packs/documents"
	emotionpack "yunque-agent/internal/packs/emotion"
	federationpack "yunque-agent/internal/packs/federation"
	filespack "yunque-agent/internal/packs/files"
	forkspack "yunque-agent/internal/packs/forks"
	graphpack "yunque-agent/internal/packs/graph"
	heartbeatpack "yunque-agent/internal/packs/heartbeat"
	idepack "yunque-agent/internal/packs/ide"
	identitypack "yunque-agent/internal/packs/identity"
	instructionspack "yunque-agent/internal/packs/instructions"
	knowledgepack "yunque-agent/internal/packs/knowledge"
	marketpack "yunque-agent/internal/packs/market"
	mcpdispatchpack "yunque-agent/internal/packs/mcpdispatch"
	memorypack "yunque-agent/internal/packs/memory"
	missionspack "yunque-agent/internal/packs/missions"
	modespack "yunque-agent/internal/packs/modes"
	modulespack "yunque-agent/internal/packs/modules"
	notificationspack "yunque-agent/internal/packs/notifications"
	orchestratorpack "yunque-agent/internal/packs/orchestrator"
	personapack "yunque-agent/internal/packs/persona"
	plannerrecoverypack "yunque-agent/internal/packs/plannerrecovery"
	pluginapipack "yunque-agent/internal/packs/pluginapi"
	rbacpack "yunque-agent/internal/packs/rbac"
	reflectionpack "yunque-agent/internal/packs/reflection"
	retrievalpack "yunque-agent/internal/packs/retrieval"
	reveriepack "yunque-agent/internal/packs/reverie"
	sandboxpack "yunque-agent/internal/packs/sandbox"
	schedulerpack "yunque-agent/internal/packs/scheduler"
	sessionqueuepack "yunque-agent/internal/packs/sessionqueue"
	skillhubpack "yunque-agent/internal/packs/skillhub"
	skillspack "yunque-agent/internal/packs/skills"
	speechpack "yunque-agent/internal/packs/speech"
	statepack "yunque-agent/internal/packs/state"
	subagentspack "yunque-agent/internal/packs/subagents"
	toripack "yunque-agent/internal/packs/tori"
	tracepack "yunque-agent/internal/packs/trace"
	triggerspack "yunque-agent/internal/packs/triggers"
	workpack "yunque-agent/internal/packs/work"
	"yunque-agent/pkg/packruntime"
)

// migrationPackPaths mirrors each bridge pack's Routes() so the test registry
// can enable them. Keep in sync with internal/packs/{knowledge,memory,skills,work}.
var migrationPackPaths = map[string][]string{
	knowledgepack.PackID: {
		"/v1/knowledge/search", "/v1/knowledge/sources", "/v1/knowledge/stats",
		"/v1/knowledge/upload", "/v1/knowledge/ingest", "/v1/knowledge/import-url",
		"/v1/knowledge/import-repo", "/v1/knowledge/source", "/v1/knowledge/source/update",
	},
	memorypack.PackID: {
		"/v1/memory/stats", "/v1/memory/search", "/v1/memory/recall/debug",
		"/v1/memory/add", "/v1/memory/compact", "/v1/memory/persona", "/v1/memory/update",
	},
	skillspack.PackID: {
		"/v1/skills", "/v1/skills/scan", "/v1/skills/dynamic",
		"/v1/skills/approve", "/v1/skills/reject",
	},
	workpack.PackID: {
		"/v1/tasks", "/v1/tasks/run", "/v1/tasks/cancel", "/v1/tasks/pause",
		"/v1/tasks/resume", "/v1/tasks/restart", "/v1/tasks/gaps", "/v1/tasks/gaps/resolve",
		"/v1/tasks/memory", "/v1/tasks/threads", "/v1/tasks/templates",
		"/v1/tasks/templates/instantiate", "/v1/projects", "/v1/projects/detail",
		"/v1/projects/remove",
		// Workflow surface merged into the work pack (task platform).
		"/v1/workflows", "/v1/workflows/generate", "/v1/workflows/run",
		"/v1/workflows/instances", "/v1/workflows/cancel",
	},
	// control-plane is an always-on core pack; its owned route set grows per
	// migration slice, so derive it from the package to avoid drift.
	channelspack.PackID:        channelspack.Paths(),
	connectorspack.PackID:      connectorspack.Paths(),
	controlplanepack.PackID:    controlplanepack.Paths,
	costpack.PackID:            costpack.Paths(),
	cognitivelayerpack.PackID:  cognitivelayerpack.Paths(),
	desktoppack.PackID:         desktoppack.Paths(),
	federationpack.PackID:      federationpack.Paths(),
	forkspack.PackID:           forkspack.Paths(),
	heartbeatpack.PackID:       heartbeatpack.Paths(),
	identitypack.PackID:        identitypack.Paths(),
	marketpack.PackID:          marketpack.Paths(),
	mcpdispatchpack.PackID:     mcpdispatchpack.Paths(),
	modulespack.PackID:         modulespack.Paths(),
	notificationspack.PackID:   notificationspack.Paths(),
	orchestratorpack.PackID:    orchestratorpack.Paths(),
	personapack.PackID:         personapack.Paths(),
	plannerrecoverypack.PackID: plannerrecoverypack.Paths(),
	pluginapipack.PackID:       pluginapipack.Paths(),
	rbacpack.PackID:            rbacpack.Paths(),
	reflectionpack.PackID:      reflectionpack.Paths(),
	retrievalpack.PackID:       retrievalpack.Paths(),
	schedulerpack.PackID:       schedulerpack.Paths(),
	sandboxpack.PackID:         sandboxpack.Paths(),
	sessionqueuepack.PackID:    sessionqueuepack.Paths(),
	skillhubpack.PackID:        skillhubpack.Paths(),
	speechpack.PackID:          speechpack.Paths(),
	subagentspack.PackID:       subagentspack.Paths(),
	toripack.PackID:            toripack.Paths(),
	tracepack.PackID:           tracepack.Paths(),
	// Monolith route groups extracted into native packs (Tier 0 microkernel).
	modespack.PackID:     {"/v1/persona/modes", "/v1/persona/mode", "/v1/persona/mode/current"},
	reveriepack.PackID:   reveriepack.Paths(),
	idepack.PackID:       {"/v1/ide/review", "/v1/ide/status"},
	cronpack.PackID:      {"/v1/cron/list", "/v1/cron/add", "/v1/cron/remove", "/v1/cron/run"},
	documentspack.PackID: {"/v1/documents/generate", "/v1/documents/templates"},
	emotionpack.PackID:   {"/v1/emotion/stickers", "/v1/emotion/history"},
	filespack.PackID:     {"/api/files", "/api/files/preview", "/api/files/download"},
	graphpack.PackID: {
		"/v1/graph/entities", "/v1/graph/relations", "/v1/graph/context", "/v1/graph/stats",
	},
	instructionspack.PackID: {"/v1/instructions", "/v1/instructions/reorder"},
	missionspack.PackID:     {"/v1/missions/parse"},
	triggerspack.PackID: {
		"/v1/triggers", "/v1/triggers/emit", "/v1/triggers/v2",
		"/v1/triggers/v2/emit", "/v1/triggers/v2/runs", "/v1/triggers/v2/events",
	},
	statepack.PackID: {
		"/v1/state", "/v1/state/goals", "/v1/state/focus", "/v1/state/resources",
	},
}

var migrationPackNames = map[string]string{
	knowledgepack.PackID:       "Knowledge",
	memorypack.PackID:          "Memory",
	skillspack.PackID:          "Skills",
	workpack.PackID:            "Work",
	channelspack.PackID:        "Channels",
	connectorspack.PackID:      "Connectors",
	controlplanepack.PackID:    "Control Plane",
	costpack.PackID:            "Cost",
	cognitivelayerpack.PackID:  "Cognitive Layer",
	desktoppack.PackID:         "Desktop Shell",
	federationpack.PackID:      "Federation",
	forkspack.PackID:           "Forks",
	heartbeatpack.PackID:       "Heartbeat",
	identitypack.PackID:        "Identity",
	marketpack.PackID:          "Skill Market",
	mcpdispatchpack.PackID:     "MCP Dispatch",
	modulespack.PackID:         "Runtime Modules",
	notificationspack.PackID:   "Notifications",
	orchestratorpack.PackID:    "IDE Work Orchestrator",
	personapack.PackID:         "Persona",
	plannerrecoverypack.PackID: "Planner Recovery",
	pluginapipack.PackID:       "Plugin API Bridge",
	rbacpack.PackID:            "RBAC",
	reflectionpack.PackID:      "Reflection",
	retrievalpack.PackID:       "Retrieval",
	sandboxpack.PackID:         "Sandbox",
	schedulerpack.PackID:       "Scheduler",
	sessionqueuepack.PackID:    "Session Queue",
	skillhubpack.PackID:        "SkillHub",
	speechpack.PackID:          "Speech",
	subagentspack.PackID:       "Subagents",
	toripack.PackID:            "Tori",
	tracepack.PackID:           "Trace",
	modespack.PackID:           "Persona Modes",
	reveriepack.PackID:         "Reverie",
	idepack.PackID:             "IDE",
	cronpack.PackID:            "Cron",
	triggerspack.PackID:        "Triggers",
	documentspack.PackID:       "Documents",
	emotionpack.PackID:         "Emotion",
	filespack.PackID:           "Files",
	graphpack.PackID:           "Graph",
	instructionspack.PackID:    "Instructions",
	missionspack.PackID:        "Missions",
	statepack.PackID:           "State Kernel",
}

// newMigrationPackRegistry returns a registry with the migrated core packs
// installed and enabled, mirroring production so test gateways behave like the
// real one after the route migration.
func newMigrationPackRegistry() *packruntime.Registry {
	dir, err := os.MkdirTemp("", "yunque-migpacks-")
	if err != nil {
		panic("migration pack registry tempdir: " + err.Error())
	}
	reg, err := packruntime.NewRegistry(dir)
	if err != nil {
		panic("migration pack registry: " + err.Error())
	}
	for id, paths := range migrationPackPaths {
		if _, err := reg.Install(packruntime.Manifest{
			ID:           id,
			Name:         migrationPackNames[id],
			Version:      "0.1.0",
			Optional:     true,
			DefaultState: "enabled",
			Backend:      packruntime.BackendManifest{Routes: paths},
		}, "test"); err != nil {
			panic("install migration pack " + id + ": " + err.Error())
		}
	}
	return reg
}

// registerMigrationPacks mounts the migrated core packs on gw (matches
// cmd/agent/init_task_engine.go) so migrated /v1/{knowledge,memory,skills,
// tasks,projects,state} routes exist and are auth-gated in tests.
func registerMigrationPacks(gw *Gateway) {
	gw.RegisterBackendPack(knowledgepack.NewHandlerWithStore(gw, gw.KnowledgeStore()))
	gw.RegisterBackendPack(memorypack.NewWired(gw.MemoryManager(), gw.MemoryPipeline(), gw.MemoryOrchestrator, gw.TenantOf))
	gw.RegisterBackendPack(skillspack.NewHandlerWithService(gw.SkillsRegistry(), gw.Metrics()))
	_ = gw.RegisterModule(channelspack.New(gw))
	gw.RegisterBackendPack(workpack.NewHandler(gw))
	_ = gw.RegisterModule(connectorspack.NewProvider(gw.ConnectorRegistry))
	gw.RegisterBackendPack(controlplanepack.NewHandler(gw))
	_ = gw.RegisterModule(costpack.NewProvider(func() *costtrack.Tracker { return gw.costTracker }))
	_ = gw.RegisterModule(cognitivelayerpack.New(gw))
	_ = gw.RegisterModule(desktoppack.New())
	_ = gw.RegisterModule(federationpack.New(gw))
	_ = gw.RegisterModule(forkspack.NewProvider(gw.ForkTree, gw.ForkPersister))
	_ = gw.RegisterModule(heartbeatpack.New(gw))
	_ = gw.RegisterModule(identitypack.New(gw))
	_ = gw.RegisterModule(marketpack.New(gw))
	_ = gw.RegisterModule(mcpdispatchpack.New(gw))
	_ = gw.RegisterModule(modulespack.New(gw))
	_ = gw.RegisterModule(notificationspack.NewProvider(gw.Notifier))
	_ = gw.RegisterModule(orchestratorpack.New(gw))
	_ = gw.RegisterModule(personapack.New(gw))
	_ = gw.RegisterModule(plannerrecoverypack.New(gw))
	_ = gw.RegisterModule(pluginapipack.New(pluginapipack.Config{}, pluginapipack.NewPluginTokenManager()))
	_ = gw.RegisterModule(rbacpack.New(gw))
	_ = gw.RegisterModule(reflectionpack.New(gw))
	_ = gw.RegisterModule(retrievalpack.New(gw))
	sandboxPack := sandboxpack.New(gw)
	gw.SetDesktopSandboxStatusProvider(sandboxPack.StatusMap)
	_ = gw.RegisterModule(sandboxPack)
	_ = gw.RegisterModule(schedulerpack.NewProvider(gw.Scheduler))
	_ = gw.RegisterModule(sessionqueuepack.New(gw))
	_ = gw.RegisterModule(skillhubpack.New(gw))
	_ = gw.RegisterModule(speechpack.New(gw))
	_ = gw.RegisterModule(subagentspack.New(gw))
	_ = gw.RegisterModule(toripack.New(gw))
	_ = gw.RegisterModule(tracepack.New(gw))
	// Native monolith-extracted packs (mirror cmd/agent/init_task_engine.go).
	_ = gw.RegisterModule(modespack.New(gw))
	_ = gw.RegisterModule(reveriepack.New(gw))
	_ = gw.RegisterModule(idepack.New(gw))
	_ = gw.RegisterModule(cronpack.New(gw))
	_ = gw.RegisterModule(triggerspack.New(gw))
	_ = gw.RegisterModule(documentspack.New(gw))
	_ = gw.RegisterModule(emotionpack.New(gw))
	_ = gw.RegisterModule(filespack.New(gw))
	_ = gw.RegisterModule(graphpack.New(gw))
	_ = gw.RegisterModule(instructionspack.New(gw))
	_ = gw.RegisterModule(missionspack.New(gw))
	_ = gw.RegisterModule(statepack.New(gw))
}

// newTestGatewayMigrationEnabled returns a default test gateway with all four
// migration packs registered and enabled — i.e. production-faithful for the
// migrated /v1/{knowledge,memory,skills,tasks,projects} surfaces.
func newTestGatewayMigrationEnabled() (*Gateway, *tenant.Manager) {
	gw, tm := newTestGateway()
	gw.SetPackRegistry(newMigrationPackRegistry())
	registerMigrationPacks(gw)
	return gw, tm
}

// newTestGatewayWithMigrationPack builds a gateway hosting exactly one migration
// pack at the requested status, so a single group's route gating can be tested
// in isolation (mirrors the per-pack helpers like newTestGatewayWithBrowserIntentPack).
func newTestGatewayWithMigrationPack(t *testing.T, packID string, status packruntime.PackStatus) (*Gateway, *tenant.Manager) {
	t.Helper()
	registry, err := packruntime.NewRegistry(t.TempDir())
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}
	if _, err := registry.Install(packruntime.Manifest{
		ID:           packID,
		Name:         migrationPackNames[packID],
		Version:      "0.1.0",
		Optional:     true,
		DefaultState: "enabled",
		Backend:      packruntime.BackendManifest{Routes: migrationPackPaths[packID]},
	}, "test"); err != nil {
		t.Fatalf("Install %s: %v", packID, err)
	}
	if status == packruntime.PackStatusDisabled {
		if _, err := registry.Disable(packID); err != nil {
			t.Fatalf("Disable %s: %v", packID, err)
		}
	}
	gw, tm := newTestGatewayWithConfig(GatewayConfig{Packs: registry})
	switch packID {
	case knowledgepack.PackID:
		gw.RegisterBackendPack(knowledgepack.NewHandlerWithStore(gw, gw.KnowledgeStore()))
	case memorypack.PackID:
		gw.RegisterBackendPack(memorypack.NewWired(gw.MemoryManager(), gw.MemoryPipeline(), gw.MemoryOrchestrator, gw.TenantOf))
	case skillspack.PackID:
		gw.RegisterBackendPack(skillspack.NewHandlerWithService(gw.SkillsRegistry(), gw.Metrics()))
	case channelspack.PackID:
		_ = gw.RegisterModule(channelspack.New(nil))
	case workpack.PackID:
		gw.RegisterBackendPack(workpack.NewHandler(gw))
	case connectorspack.PackID:
		_ = gw.RegisterModule(connectorspack.New(nil))
	case controlplanepack.PackID:
		gw.RegisterBackendPack(controlplanepack.NewHandler(gw))
	case costpack.PackID:
		_ = gw.RegisterModule(costpack.New(nil))
	case cognitivelayerpack.PackID:
		_ = gw.RegisterModule(cognitivelayerpack.New(gw))
	case desktoppack.PackID:
		_ = gw.RegisterModule(desktoppack.NewWithController(nil))
	case federationpack.PackID:
		_ = gw.RegisterModule(federationpack.New(nil))
	case forkspack.PackID:
		_ = gw.RegisterModule(forkspack.New(nil, nil))
	case heartbeatpack.PackID:
		_ = gw.RegisterModule(heartbeatpack.New(nil))
	case identitypack.PackID:
		_ = gw.RegisterModule(identitypack.New(gw))
	case marketpack.PackID:
		_ = gw.RegisterModule(marketpack.New(nil))
	case mcpdispatchpack.PackID:
		_ = gw.RegisterModule(mcpdispatchpack.New(gw))
	case modulespack.PackID:
		_ = gw.RegisterModule(modulespack.New(gw))
	case notificationspack.PackID:
		_ = gw.RegisterModule(notificationspack.New(nil))
	case orchestratorpack.PackID:
		_ = gw.RegisterModule(orchestratorpack.New(gw))
	case personapack.PackID:
		_ = gw.RegisterModule(personapack.New(gw))
	case plannerrecoverypack.PackID:
		_ = gw.RegisterModule(plannerrecoverypack.New(gw))
	case pluginapipack.PackID:
		_ = gw.RegisterModule(pluginapipack.New(pluginapipack.Config{}, pluginapipack.NewPluginTokenManager()))
	case rbacpack.PackID:
		_ = gw.RegisterModule(rbacpack.New(gw))
	case reflectionpack.PackID:
		_ = gw.RegisterModule(reflectionpack.New(gw))
	case retrievalpack.PackID:
		_ = gw.RegisterModule(retrievalpack.New(gw))
	case sandboxpack.PackID:
		sandboxPack := sandboxpack.New(gw)
		gw.SetDesktopSandboxStatusProvider(sandboxPack.StatusMap)
		_ = gw.RegisterModule(sandboxPack)
	case reveriepack.PackID:
		_ = gw.RegisterModule(reveriepack.New(gw))
	case schedulerpack.PackID:
		_ = gw.RegisterModule(schedulerpack.New(nil))
	case sessionqueuepack.PackID:
		_ = gw.RegisterModule(sessionqueuepack.New(gw))
	case skillhubpack.PackID:
		_ = gw.RegisterModule(skillhubpack.New(nil))
	case speechpack.PackID:
		_ = gw.RegisterModule(speechpack.New(nil))
	case subagentspack.PackID:
		_ = gw.RegisterModule(subagentspack.New(gw))
	case toripack.PackID:
		_ = gw.RegisterModule(toripack.New(gw))
	case tracepack.PackID:
		_ = gw.RegisterModule(tracepack.New(gw))
	case modespack.PackID:
		_ = gw.RegisterModule(modespack.New(gw))
	case idepack.PackID:
		_ = gw.RegisterModule(idepack.New(gw))
	case cronpack.PackID:
		_ = gw.RegisterModule(cronpack.New(gw))
	case triggerspack.PackID:
		_ = gw.RegisterModule(triggerspack.New(gw))
	case documentspack.PackID:
		_ = gw.RegisterModule(documentspack.New(gw))
	case emotionpack.PackID:
		_ = gw.RegisterModule(emotionpack.New(gw))
	case filespack.PackID:
		_ = gw.RegisterModule(filespack.New(gw))
	case graphpack.PackID:
		_ = gw.RegisterModule(graphpack.New(gw))
	case instructionspack.PackID:
		_ = gw.RegisterModule(instructionspack.New(gw))
	case missionspack.PackID:
		_ = gw.RegisterModule(missionspack.New(gw))
	case statepack.PackID:
		_ = gw.RegisterModule(statepack.New(gw))
	}
	return gw, tm
}

// TestMigrationPackRouteGating verifies each migrated bridge pack owns its
// routes through the Pack Runtime gates: the auth gate fires before the enable
// gate (no auth → 401 even when enabled), and disabling the pack removes the
// surface (authed → 404). Both checks are gate-level and never invoke the real
// business handler, so they need no extra per-handler setup.
func TestMigrationPackRouteGating(t *testing.T) {
	cases := []struct {
		name   string
		packID string
		method string
		probe  string
	}{
		{"knowledge", knowledgepack.PackID, http.MethodGet, "/v1/knowledge/stats"},
		{"memory", memorypack.PackID, http.MethodGet, "/v1/memory/stats"},
		{"skills", skillspack.PackID, http.MethodGet, "/v1/skills"},
		{"channels", channelspack.PackID, http.MethodGet, "/v1/channels/groups"},
		{"work", workpack.PackID, http.MethodGet, "/v1/tasks"},
		{"connectors", connectorspack.PackID, http.MethodGet, "/api/connectors"},
		{"cost", costpack.PackID, http.MethodGet, "/v1/cost/summary"},
		{"cognitive-layer", cognitivelayerpack.PackID, http.MethodGet, "/v1/cognitive-layer"},
		{"desktop", desktoppack.PackID, http.MethodGet, "/v1/desktop/console"},
		{"federation", federationpack.PackID, http.MethodGet, "/v1/federation/peers"},
		{"forks", forkspack.PackID, http.MethodGet, "/v1/fork/list"},
		{"heartbeat", heartbeatpack.PackID, http.MethodGet, "/v1/heartbeat"},
		{"identity", identitypack.PackID, http.MethodGet, "/v1/identity/profiles"},
		{"market", marketpack.PackID, http.MethodGet, "/v1/market/search"},
		{"mcp-dispatch", mcpdispatchpack.PackID, http.MethodGet, "/v1/workers"},
		{"modules", modulespack.PackID, http.MethodGet, "/v1/modules"},
		{"notifications", notificationspack.PackID, http.MethodGet, "/api/notify/channels"},
		{"orchestrator", orchestratorpack.PackID, http.MethodGet, "/v1/orchestrator/status"},
		{"persona", personapack.PackID, http.MethodGet, "/v1/persona"},
		{"planner-recovery", plannerrecoverypack.PackID, http.MethodGet, "/v1/planner/checkpoints"},
		{"plugin-api", pluginapipack.PackID, http.MethodPost, "/v1/plugin-api/llm"},
		{"rbac", rbacpack.PackID, http.MethodGet, "/v1/rbac/my-roles"},
		{"reflection", reflectionpack.PackID, http.MethodGet, "/v1/reflect/experiences"},
		{"retrieval", retrievalpack.PackID, http.MethodGet, "/v1/search/providers"},
		{"sandbox", sandboxpack.PackID, http.MethodGet, "/v1/sandbox/desktop/status"},
		{"reverie", reveriepack.PackID, http.MethodGet, "/v1/reverie/dream/status"},
		{"scheduler", schedulerpack.PackID, http.MethodGet, "/v1/scheduler/jobs"},
		{"session-queue", sessionqueuepack.PackID, http.MethodGet, "/v1/sessions/queue"},
		{"skillhub", skillhubpack.PackID, http.MethodGet, "/api/skillhub/search"},
		{"speech", speechpack.PackID, http.MethodGet, "/v1/speech/voices"},
		{"subagents", subagentspack.PackID, http.MethodGet, "/v1/subagent"},
		{"tori", toripack.PackID, http.MethodGet, "/v1/tori/status"},
		{"trace", tracepack.PackID, http.MethodGet, "/v1/trace/recent"},
		{"persona-modes", modespack.PackID, http.MethodGet, "/v1/persona/modes"},
		{"ide", idepack.PackID, http.MethodGet, "/v1/ide/status"},
		{"cron", cronpack.PackID, http.MethodGet, "/v1/cron/list"},
		{"triggers", triggerspack.PackID, http.MethodGet, "/v1/triggers"},
		{"documents", documentspack.PackID, http.MethodGet, "/v1/documents/templates"},
		{"emotion", emotionpack.PackID, http.MethodGet, "/v1/emotion/history"},
		{"files", filespack.PackID, http.MethodGet, "/api/files"},
		{"graph", graphpack.PackID, http.MethodGet, "/v1/graph/stats"},
		{"instructions", instructionspack.PackID, http.MethodGet, "/v1/instructions"},
		{"missions", missionspack.PackID, http.MethodPost, "/v1/missions/parse"},
		{"state", statepack.PackID, http.MethodGet, "/v1/state"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// Enabled but unauthenticated → 401 (auth gate precedes enable gate).
			gw, _ := newTestGatewayWithMigrationPack(t, tc.packID, packruntime.PackStatusEnabled)
			req := httptest.NewRequest(tc.method, tc.probe, nil)
			w := httptest.NewRecorder()
			gw.ServeHTTP(w, req)
			if w.Code != http.StatusUnauthorized {
				t.Fatalf("%s enabled+no-auth: expected 401, got %d", tc.name, w.Code)
			}

			// Disabled but authenticated → 404 (enable gate removes the surface).
			gwD, tmD := newTestGatewayWithMigrationPack(t, tc.packID, packruntime.PackStatusDisabled)
			key := tmD.Register(tc.name + "-org").APIKey
			reqD := httptest.NewRequest(tc.method, tc.probe, nil)
			reqD.Header.Set("X-API-Key", key)
			wD := httptest.NewRecorder()
			gwD.ServeHTTP(wD, reqD)
			if wD.Code != http.StatusNotFound {
				t.Fatalf("%s disabled+authed: expected 404, got %d", tc.name, wD.Code)
			}
		})
	}
}

func TestMCPDispatchPackKeepsMethodSensitiveAuth(t *testing.T) {
	gw, _ := newTestGatewayWithMigrationPack(t, mcpdispatchpack.PackID, packruntime.PackStatusEnabled)

	getReq := httptest.NewRequest(http.MethodGet, "/mcp/v1", nil)
	getRec := httptest.NewRecorder()
	gw.ServeHTTP(getRec, getReq)
	if getRec.Code == http.StatusUnauthorized {
		t.Fatal("MCP dispatch GET probe must stay unauthenticated")
	}

	postReq := httptest.NewRequest(http.MethodPost, "/mcp/v1", nil)
	postRec := httptest.NewRecorder()
	gw.ServeHTTP(postRec, postReq)
	if postRec.Code != http.StatusUnauthorized {
		t.Fatalf("MCP dispatch POST must require auth, got %d", postRec.Code)
	}
}
