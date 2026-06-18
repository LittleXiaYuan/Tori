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
	connectorspack "yunque-agent/internal/packs/connectors"
	controlplanepack "yunque-agent/internal/packs/controlplane"
	costpack "yunque-agent/internal/packs/cost"
	cronpack "yunque-agent/internal/packs/cron"
	documentspack "yunque-agent/internal/packs/documents"
	idepack "yunque-agent/internal/packs/ide"
	knowledgepack "yunque-agent/internal/packs/knowledge"
	memorypack "yunque-agent/internal/packs/memory"
	missionspack "yunque-agent/internal/packs/missions"
	modespack "yunque-agent/internal/packs/modes"
	reveriepack "yunque-agent/internal/packs/reverie"
	skillspack "yunque-agent/internal/packs/skills"
	statepack "yunque-agent/internal/packs/state"
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
	connectorspack.PackID:   connectorspack.Paths(),
	controlplanepack.PackID: controlplanepack.Paths,
	costpack.PackID:         costpack.Paths(),
	// Monolith route groups extracted into native packs (Tier 0 microkernel).
	modespack.PackID: {"/v1/persona/modes", "/v1/persona/mode", "/v1/persona/mode/current"},
	reveriepack.PackID: {
		"/v1/reverie/journal", "/v1/reverie/stats", "/v1/reverie/config",
		"/v1/reverie/think", "/v1/reverie/thought", "/v1/reverie/targets", "/v1/reverie/actions",
	},
	idepack.PackID:       {"/v1/ide/review", "/v1/ide/status"},
	cronpack.PackID:      {"/v1/cron/list", "/v1/cron/add", "/v1/cron/remove", "/v1/cron/run"},
	documentspack.PackID: {"/v1/documents/generate", "/v1/documents/templates"},
	missionspack.PackID:  {"/v1/missions/parse"},
	triggerspack.PackID: {
		"/v1/triggers", "/v1/triggers/emit", "/v1/triggers/v2",
		"/v1/triggers/v2/emit", "/v1/triggers/v2/runs", "/v1/triggers/v2/events",
	},
	statepack.PackID: {
		"/v1/state", "/v1/state/goals", "/v1/state/focus", "/v1/state/resources",
	},
}

var migrationPackNames = map[string]string{
	knowledgepack.PackID:    "Knowledge",
	memorypack.PackID:       "Memory",
	skillspack.PackID:       "Skills",
	workpack.PackID:         "Work",
	connectorspack.PackID:   "Connectors",
	controlplanepack.PackID: "Control Plane",
	costpack.PackID:         "Cost",
	modespack.PackID:        "Persona Modes",
	reveriepack.PackID:      "Reverie",
	idepack.PackID:          "IDE",
	cronpack.PackID:         "Cron",
	triggerspack.PackID:     "Triggers",
	documentspack.PackID:    "Documents",
	missionspack.PackID:     "Missions",
	statepack.PackID:        "State Kernel",
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
	gw.RegisterBackendPack(workpack.NewHandler(gw))
	_ = gw.RegisterModule(connectorspack.NewProvider(gw.ConnectorRegistry))
	gw.RegisterBackendPack(controlplanepack.NewHandler(gw))
	_ = gw.RegisterModule(costpack.NewProvider(func() *costtrack.Tracker { return gw.costTracker }))
	// Native monolith-extracted packs (mirror cmd/agent/init_task_engine.go).
	_ = gw.RegisterModule(modespack.New(gw))
	_ = gw.RegisterModule(reveriepack.New(gw))
	_ = gw.RegisterModule(idepack.New(gw))
	_ = gw.RegisterModule(cronpack.New(gw))
	_ = gw.RegisterModule(triggerspack.New(gw))
	_ = gw.RegisterModule(documentspack.New(gw))
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
	case workpack.PackID:
		gw.RegisterBackendPack(workpack.NewHandler(gw))
	case connectorspack.PackID:
		_ = gw.RegisterModule(connectorspack.New(nil))
	case controlplanepack.PackID:
		gw.RegisterBackendPack(controlplanepack.NewHandler(gw))
	case costpack.PackID:
		_ = gw.RegisterModule(costpack.New(nil))
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
		probe  string
	}{
		{"knowledge", knowledgepack.PackID, "/v1/knowledge/stats"},
		{"memory", memorypack.PackID, "/v1/memory/stats"},
		{"skills", skillspack.PackID, "/v1/skills"},
		{"work", workpack.PackID, "/v1/tasks"},
		{"connectors", connectorspack.PackID, "/api/connectors"},
		{"cost", costpack.PackID, "/v1/cost/summary"},
		{"state", statepack.PackID, "/v1/state"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// Enabled but unauthenticated → 401 (auth gate precedes enable gate).
			gw, _ := newTestGatewayWithMigrationPack(t, tc.packID, packruntime.PackStatusEnabled)
			req := httptest.NewRequest(http.MethodGet, tc.probe, nil)
			w := httptest.NewRecorder()
			gw.ServeHTTP(w, req)
			if w.Code != http.StatusUnauthorized {
				t.Fatalf("%s enabled+no-auth: expected 401, got %d", tc.name, w.Code)
			}

			// Disabled but authenticated → 404 (enable gate removes the surface).
			gwD, tmD := newTestGatewayWithMigrationPack(t, tc.packID, packruntime.PackStatusDisabled)
			key := tmD.Register(tc.name + "-org").APIKey
			reqD := httptest.NewRequest(http.MethodGet, tc.probe, nil)
			reqD.Header.Set("X-API-Key", key)
			wD := httptest.NewRecorder()
			gwD.ServeHTTP(wD, reqD)
			if wD.Code != http.StatusNotFound {
				t.Fatalf("%s disabled+authed: expected 404, got %d", tc.name, wD.Code)
			}
		})
	}
}
