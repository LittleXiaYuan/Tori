package yunque

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAPIErrorMessageParsesNestedGatewayError(t *testing.T) {
	body := []byte(`{"error":{"code":"BAD_REQUEST","message":"unsupported recovery action"}}`)
	got := apiErrorMessage(body)
	want := "BAD_REQUEST: unsupported recovery action"
	if got != want {
		t.Fatalf("apiErrorMessage() = %q, want %q", got, want)
	}
}

func TestAPIErrorMessageFallsBackToText(t *testing.T) {
	got := apiErrorMessage([]byte("plain failure"))
	if got != "plain failure" {
		t.Fatalf("apiErrorMessage() = %q, want plain failure", got)
	}
}

func TestReflectExperiencesSerializesFilters(t *testing.T) {
	withTestAPI(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/reflect/experiences" {
			t.Fatalf("path = %q", r.URL.Path)
		}
		q := r.URL.Query()
		if q.Get("q") != "code review" || q.Get("source") != "task" || q.Get("outcome") != "partial" || q.Get("tag") != "quality:9" || q.Get("limit") != "5" {
			t.Fatalf("unexpected query: %s", r.URL.RawQuery)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"experiences":[{"id":"e1","source":"task","outcome":"partial","lesson":"keep slices small","tags":["quality:9"],"created_at":"2026-05-12T01:02:03Z"}],"total":1}`))
	})

	resp, err := Reflect.Experiences(context.Background(), ReflectExperienceOptions{
		Query: "code review", Source: "task", Outcome: "partial", Tag: "quality:9", Limit: 5,
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Total != 1 || len(resp.Experiences) != 1 {
		t.Fatalf("unexpected experiences response: %+v", resp)
	}
	if resp.Experiences[0].ID != "e1" || resp.Experiences[0].Tags[0] != "quality:9" {
		t.Fatalf("unexpected experience: %+v", resp.Experiences[0])
	}
}

func TestReflectStatsAndStrategies(t *testing.T) {
	var paths []string
	withTestAPI(t, func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.String())
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/v1/reflect/experiences":
			if r.URL.Query().Get("stats") != "true" || r.URL.Query().Get("tag") != "quality:9" {
				t.Fatalf("unexpected stats query: %s", r.URL.RawQuery)
			}
			_, _ = w.Write([]byte(`{"total":2,"by_source":{"task":2},"by_category":{"strategy":2},"by_outcome":{"success":2},"recent_7d":1}`))
		case "/v1/reflect/strategies":
			if r.URL.Query().Get("limit") != "3" || r.URL.Query().Get("tag") != "quality:9" {
				t.Fatalf("unexpected strategies query: %s", r.URL.RawQuery)
			}
			_, _ = w.Write([]byte(`{"strategies":"- 推荐: keep slices small"}`))
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	})

	stats, err := Reflect.Stats(context.Background(), ReflectExperienceOptions{Tag: "quality:9"})
	if err != nil {
		t.Fatal(err)
	}
	strategies, err := Reflect.StrategiesWithOptions(context.Background(), ReflectStrategyOptions{Tag: "quality:9", Limit: 3})
	if err != nil {
		t.Fatal(err)
	}
	if stats.Total != 2 || stats.ByOutcome["success"] != 2 || stats.Recent7D != 1 {
		t.Fatalf("unexpected stats: %+v", stats)
	}
	if !strings.Contains(strategies, "keep slices small") {
		t.Fatalf("unexpected strategies: %q", strategies)
	}
	if len(paths) != 2 {
		t.Fatalf("expected 2 API calls, got %d: %v", len(paths), paths)
	}
}

func TestStateSnapshotActionsAndCapabilities(t *testing.T) {
	withTestAPI(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/state" {
			t.Fatalf("path = %q", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-token" {
			t.Fatalf("Authorization = %q", got)
		}
		if got := r.Header.Get("X-Plugin-Name"); got != "state-plugin" {
			t.Fatalf("X-Plugin-Name = %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"goals":[{"id":"g1","title":"Expose state as SDK","status":"active","progress":0.4,"created_at":"2026-05-12T01:02:03Z"}],
			"resources":[{"id":"r1","type":"repo","path":"C:/Code/AI/云雀/yunque-agent","status":"active","metadata":{"kind":"sdk"},"tracked_at":"2026-05-12T01:03:03Z"}],
			"focus":"Go SDK state slice",
			"topics":["sdk","state"],
			"recent_actions":[{"timestamp":"2026-05-12T01:04:03Z","action":"slice_added","result":"ok","success":true}],
			"capabilities":{"total_skills":7,"dynamic_skills":["state"],"unresolved_gaps":2,"recent_gaps":["go-sdk"]},
			"updated_at":"2026-05-12T01:05:03Z"
		}`))
	})

	snap, err := State.Snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if snap.Focus != "Go SDK state slice" || len(snap.Goals) != 1 || snap.Goals[0].Title != "Expose state as SDK" {
		t.Fatalf("unexpected snapshot: %+v", snap)
	}
	if snap.Resources[0].Metadata["kind"] != "sdk" || snap.Topics[1] != "state" {
		t.Fatalf("unexpected state details: %+v", snap)
	}

	actions, err := State.Actions(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(actions) != 1 || actions[0].Action != "slice_added" || !actions[0].Success {
		t.Fatalf("unexpected actions: %+v", actions)
	}

	caps, err := State.Capabilities(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if caps.TotalSkills != 7 || caps.RecentGaps[0] != "go-sdk" {
		t.Fatalf("unexpected capabilities: %+v", caps)
	}
}

func TestStateFocusedHelpers(t *testing.T) {
	var seen []string
	withTestAPI(t, func(w http.ResponseWriter, r *http.Request) {
		seen = append(seen, r.Method+" "+r.URL.String())
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/v1/state/goals":
			_, _ = w.Write([]byte(`[{"id":"g1","title":"Keep SDK incremental","status":"active"}]`))
		case r.Method == http.MethodPost && r.URL.Path == "/v1/state/goals":
			var body StateGoal
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			if body.Title != "New goal" || body.Priority != 2 {
				t.Fatalf("unexpected goal body: %+v", body)
			}
			_, _ = w.Write([]byte(`{"id":"g2","status":"created"}`))
		case r.Method == http.MethodGet && r.URL.Path == "/v1/state/focus":
			_, _ = w.Write([]byte(`{"focus":"SDK boundary"}`))
		case r.Method == http.MethodGet && r.URL.Path == "/v1/state/resources":
			_, _ = w.Write([]byte(`[{"id":"r1","type":"file","path":"sdk/go/yunque/yunque.go","status":"active"}]`))
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
		}
	})

	goals, err := State.Goals(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(goals) != 1 || goals[0].Title != "Keep SDK incremental" {
		t.Fatalf("unexpected goals: %+v", goals)
	}

	saved, err := State.SaveGoal(context.Background(), StateGoal{Title: "New goal", Priority: 2})
	if err != nil {
		t.Fatal(err)
	}
	if saved.ID != "g2" || saved.Status != "created" {
		t.Fatalf("unexpected save response: %+v", saved)
	}

	focus, err := State.Focus(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if focus != "SDK boundary" {
		t.Fatalf("unexpected focus: %q", focus)
	}

	resources, err := State.Resources(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(resources) != 1 || resources[0].Path != "sdk/go/yunque/yunque.go" {
		t.Fatalf("unexpected resources: %+v", resources)
	}

	if len(seen) != 4 {
		t.Fatalf("expected 4 requests, got %d: %v", len(seen), seen)
	}
}

func TestStateActionsFallbacksToEmptySlice(t *testing.T) {
	withTestAPI(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/state" {
			t.Fatalf("path = %q", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"goals":[],"resources":[]}`))
	})

	actions, err := State.Actions(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if actions == nil || len(actions) != 0 {
		t.Fatalf("expected empty non-nil actions, got %#v", actions)
	}

	caps, err := State.Capabilities(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if caps.TotalSkills != 0 || caps.UnresolvedGaps != 0 || caps.DynamicSkills != nil || caps.RecentGaps != nil {
		t.Fatalf("expected zero capabilities, got %+v", caps)
	}
}

func TestAgentKitGroupsStateReflectAndPluginRuntime(t *testing.T) {
	var seen []string
	withTestAPI(t, func(w http.ResponseWriter, r *http.Request) {
		seen = append(seen, r.Method+" "+r.URL.String())
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/v1/state/focus":
			_, _ = w.Write([]byte(`{"focus":"sdk"}`))
		case "/v1/reflect/strategies":
			if r.URL.Query().Get("tag") != "sdk" {
				t.Fatalf("unexpected strategies query: %s", r.URL.RawQuery)
			}
			_, _ = w.Write([]byte(`{"strategies":"- keep SDK slices small"}`))
		case "/v1/missions/parse":
			var body map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			if body["description"] != "每天八点总结昨天的任务" {
				t.Fatalf("unexpected mission parse body: %+v", body)
			}
			_, _ = w.Write([]byte(`{"type":"cron","name":"每日总结","description":"每天总结昨天的任务","config":{"cron_expr":"0 8 * * *"},"confidence":0.9,"explanation":"mentions daily schedule"}`))
		case "/v1/scheduler/jobs":
			_, _ = w.Write([]byte(`{"jobs":[{"id":"job_1","name":"daily","interval":60000000000,"prompt":"复盘"}],"count":1}`))
		case "/v1/cron/list":
			_, _ = w.Write([]byte(`{"jobs":[{"id":"cron_1","name":"daily","schedule":{"type":"every","every_ms":60000},"payload":{"kind":"agentTurn","message":"ping"},"enabled":true,"created_at":"2026-05-12T00:00:00Z","run_count":0}]}`))
		case "/v1/triggers/v2":
			_, _ = w.Write([]byte(`{"triggers":[{"id":"tr_1","name":"review done","tenant_id":"default","type":"event","status":"enabled","actions":[{"kind":"notify"}]}],"total":1}`))
		case "/v1/memory/search":
			_, _ = w.Write([]byte(`{"results":[{"key":"pref","value":"喜欢中文回复","layer":"mid"}],"count":1}`))
		case "/v1/graph/stats":
			_, _ = w.Write([]byte(`{"entities":2,"relations":1}`))
		case "/v1/knowledge/stats":
			_, _ = w.Write([]byte(`{"sources":2,"chunks":8}`))
		case "/v1/lora/status":
			_, _ = w.Write([]byte(`{"active_model":"adapter-a","rolling_success_rate":0.8}`))
		case "/v1/workflows":
			_, _ = w.Write([]byte(`{"workflows":[{"id":"wf_1","name":"SDK flow"}],"total":1}`))
		case "/api/connectors":
			_, _ = w.Write([]byte(`{"connectors":[{"id":"github","name":"GitHub","supported":true,"status":"connected"}]}`))
		case "/v1/plugin-api/search":
			_, _ = w.Write([]byte(`{"results":[{"title":"Agent Kit","url":"https://example.test","snippet":"ok"}]}`))
		case "/v1/plugin-api/memory/set":
			_, _ = w.Write([]byte(`{"ok":true}`))
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
		}
	})

	kit := NewAgentKit()
	focus, err := kit.State.Focus(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	strategies, err := kit.Reflect.StrategiesWithOptions(context.Background(), ReflectStrategyOptions{Tag: "sdk"})
	if err != nil {
		t.Fatal(err)
	}
	mission, err := kit.Missions.Parse(context.Background(), "每天八点总结昨天的任务")
	if err != nil {
		t.Fatal(err)
	}
	jobs, err := kit.Scheduler.Jobs(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	cronJobs, err := kit.CronSystem.List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	triggerDefs, err := kit.Triggers.List(context.Background(), TriggerListOptions{Status: "enabled"})
	if err != nil {
		t.Fatal(err)
	}
	memoryResults, err := kit.MemoryCore.Search(context.Background(), MemorySearchRequest{Query: "中文", Limit: 1})
	if err != nil {
		t.Fatal(err)
	}
	graphStats, err := kit.Graph.Stats(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	kbStats, err := kit.KnowledgeKB.Stats(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	loraStatus, err := kit.LoRA.Status(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	workflowList, err := kit.Workflows.List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	connectorList, err := kit.Connectors.List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	results, err := kit.Plugin.Search(context.Background(), "agent kit", 2)
	if err != nil {
		t.Fatal(err)
	}
	if err := kit.Memory.Set(context.Background(), "last", "ok"); err != nil {
		t.Fatal(err)
	}

	if focus != "sdk" || !strings.Contains(strategies, "SDK slices") || mission.Type != "cron" || jobs.Count != 1 || len(cronJobs.Jobs) != 1 || triggerDefs.Total != 1 || memoryResults.Count != 1 || graphStats.Entities != 2 || kbStats["sources"].(float64) != 2 || loraStatus["active_model"] != "adapter-a" || workflowList.Total != 1 || len(connectorList.Connectors) != 1 || connectorList.Connectors[0].ID != "github" || len(results) != 1 || results[0].Title != "Agent Kit" {
		t.Fatalf("unexpected kit results: focus=%q strategies=%q mission=%+v jobs=%+v results=%+v", focus, strategies, mission, jobs, results)
	}
	if kit.State != State || kit.Reflect != Reflect || kit.Missions != Missions || kit.Scheduler != Scheduler || kit.CronSystem != CronSystem || kit.Triggers != Triggers || kit.MemoryCore != MemoryCore || kit.Graph != Graph || kit.KnowledgeKB != KnowledgeKB || kit.LoRA != LoRA || kit.Workflows != Workflows || kit.Connectors != Connectors || kit.Plugin != Plugin || kit.Memory != Memory || kit.AgentMemory != AgentMemory || kit.Knowledge != Knowledge || kit.Cron != Cron {
		t.Fatalf("agent kit should reuse lightweight singleton namespaces")
	}
	if len(seen) != 14 {
		t.Fatalf("expected 14 requests, got %d: %v", len(seen), seen)
	}
}

func TestMissionsParseSerializesDescription(t *testing.T) {
	withTestAPI(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/missions/parse" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if body["description"] != "当代码评审完成时提醒我" {
			t.Fatalf("unexpected body: %+v", body)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"type":"trigger","name":"评审提醒","description":"当代码评审完成时提醒我","config":{"event_type":"review_done"},"confidence":0.8,"explanation":"mentions event condition"}`))
	})

	result, err := Missions.Parse(context.Background(), "当代码评审完成时提醒我")
	if err != nil {
		t.Fatal(err)
	}
	if result.Type != "trigger" || result.Config["event_type"] != "review_done" {
		t.Fatalf("unexpected mission parse result: %+v", result)
	}
}

func TestMemoryCoreStatsSearchAddAndCompact(t *testing.T) {
	var seen []string
	withTestAPI(t, func(w http.ResponseWriter, r *http.Request) {
		seen = append(seen, r.Method+" "+r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/v1/memory/stats":
			_, _ = w.Write([]byte(`{"short":1,"mid":2,"long":3}`))
		case "/v1/memory/search":
			var body MemorySearchRequest
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			if body.Query != "偏好" || body.Limit != 2 {
				t.Fatalf("unexpected search body: %+v", body)
			}
			_, _ = w.Write([]byte(`{"results":[{"key":"pref","value":"喜欢短回答","layer":"mid"}],"count":1}`))
		case "/v1/memory/add":
			var body MemoryAddRequest
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			if body.Value != "喜欢中文回复" || body.Layer != "long" {
				t.Fatalf("unexpected add body: %+v", body)
			}
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"status":"ok"}`))
		case "/v1/memory/compact":
			_, _ = w.Write([]byte(`{"status":"compacted"}`))
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
		}
	})

	stats, err := MemoryCore.Stats(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	search, err := MemoryCore.Search(context.Background(), MemorySearchRequest{Query: "偏好", Limit: 2})
	if err != nil {
		t.Fatal(err)
	}
	added, err := MemoryCore.Add(context.Background(), MemoryAddRequest{Content: "喜欢中文回复", Layer: "long", Source: "sdk-test"})
	if err != nil {
		t.Fatal(err)
	}
	compacted, err := MemoryCore.Compact(context.Background(), MemoryCompactRequest{TargetCount: 10})
	if err != nil {
		t.Fatal(err)
	}
	if stats["long"].(float64) != 3 || search.Count != 1 || added.Status != "ok" || compacted["status"] != "compacted" {
		t.Fatalf("unexpected memory results: stats=%+v search=%+v added=%+v compacted=%+v", stats, search, added, compacted)
	}
	if len(seen) != 4 {
		t.Fatalf("expected 4 requests, got %d: %v", len(seen), seen)
	}
}

func TestGraphNamespaceReadsAndWrites(t *testing.T) {
	var seen []string
	withTestAPI(t, func(w http.ResponseWriter, r *http.Request) {
		seen = append(seen, r.Method+" "+r.URL.String())
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/v1/graph/entities":
			switch r.Method {
			case http.MethodGet:
				if r.URL.Query().Get("q") != "云雀" {
					t.Fatalf("unexpected entity query: %s", r.URL.RawQuery)
				}
				_, _ = w.Write([]byte(`{"entities":[{"id":"e1","name":"云雀","type":"agent"}]}`))
			case http.MethodPost:
				var body GraphEntity
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Fatal(err)
				}
				if body.Name != "云雀" {
					t.Fatalf("unexpected entity body: %+v", body)
				}
				_, _ = w.Write([]byte(`{"id":"e1","name":"云雀","type":"agent"}`))
			case http.MethodDelete:
				if r.URL.Query().Get("id") != "e1" {
					t.Fatalf("unexpected delete query: %s", r.URL.RawQuery)
				}
				_, _ = w.Write([]byte(`{"ok":true}`))
			}
		case "/v1/graph/relations":
			if r.Method == http.MethodGet {
				_, _ = w.Write([]byte(`{"relations":[{"id":"r1","from_id":"e1","to_id":"e2","type":"uses","weight":0.8}]}`))
				return
			}
			_, _ = w.Write([]byte(`{"id":"r1","from_id":"e1","to_id":"e2","type":"uses","weight":0.8}`))
		case "/v1/graph/context":
			_, _ = w.Write([]byte(`{"context":"云雀 -> SDK","neighbors":[{"id":"e2"}]}`))
		case "/v1/graph/stats":
			_, _ = w.Write([]byte(`{"entities":2,"relations":1}`))
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
		}
	})

	entities, err := Graph.Entities(context.Background(), "云雀")
	if err != nil {
		t.Fatal(err)
	}
	entity, err := Graph.PutEntity(context.Background(), GraphEntity{Name: "云雀", Type: "agent"})
	if err != nil {
		t.Fatal(err)
	}
	relations, err := Graph.Relations(context.Background(), "e1")
	if err != nil {
		t.Fatal(err)
	}
	relation, err := Graph.PutRelation(context.Background(), GraphRelation{FromID: "e1", ToID: "e2", Type: "uses"})
	if err != nil {
		t.Fatal(err)
	}
	contextResult, err := Graph.ContextByEntityID(context.Background(), "e1")
	if err != nil {
		t.Fatal(err)
	}
	stats, err := Graph.Stats(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	deleted, err := Graph.DeleteEntity(context.Background(), "e1")
	if err != nil {
		t.Fatal(err)
	}
	if len(entities.Entities) != 1 || entity.ID != "e1" || len(relations.Relations) != 1 || relation.ID != "r1" || !strings.Contains(contextResult.Context, "SDK") || stats.Entities != 2 || !deleted.OK {
		t.Fatalf("unexpected graph results: entities=%+v entity=%+v relations=%+v relation=%+v context=%+v stats=%+v deleted=%+v", entities, entity, relations, relation, contextResult, stats, deleted)
	}
	if len(seen) != 7 {
		t.Fatalf("expected 7 requests, got %d: %v", len(seen), seen)
	}
}

func TestKnowledgeKBStatsSearchSourcesAndMutations(t *testing.T) {
	var seen []string
	withTestAPI(t, func(w http.ResponseWriter, r *http.Request) {
		seen = append(seen, r.Method+" "+r.URL.String())
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/v1/knowledge/stats":
			_, _ = w.Write([]byte(`{"sources":2,"chunks":8}`))
		case "/v1/knowledge/sources":
			_, _ = w.Write([]byte(`{"sources":[{"id":"src_1","name":"README.md","type":"file"}]}`))
		case "/v1/knowledge/search":
			if r.URL.Query().Get("q") != "SDK" || r.URL.Query().Get("n") != "3" {
				t.Fatalf("unexpected search query: %s", r.URL.RawQuery)
			}
			_, _ = w.Write([]byte(`{"chunks":[{"id":"c1","source_id":"src_1","content":"SDK slice","score":0.9}],"count":1}`))
		case "/v1/knowledge/ingest":
			var body KnowledgeIngestRequest
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			if body.Content != "hello" {
				t.Fatalf("unexpected ingest body: %+v", body)
			}
			_, _ = w.Write([]byte(`{"source":{"id":"src_2","name":"inline"},"stats":{"sources":3}}`))
		case "/v1/knowledge/source/update":
			_, _ = w.Write([]byte(`{"source":{"id":"src_2","name":"inline-updated"},"stats":{"sources":3}}`))
		case "/v1/knowledge/source":
			if r.URL.Query().Get("id") != "src_2" {
				t.Fatalf("unexpected delete query: %s", r.URL.RawQuery)
			}
			_, _ = w.Write([]byte(`{"deleted":"src_2","stats":{"sources":2}}`))
		case "/v1/knowledge/import-url":
			_, _ = w.Write([]byte(`{"sources":[{"id":"src_url","name":"site"}],"stats":{"sources":3}}`))
		case "/v1/knowledge/import-repo":
			_, _ = w.Write([]byte(`{"source":{"id":"src_repo","name":"repo"},"stats":{"sources":4}}`))
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
		}
	})

	stats, err := KnowledgeKB.Stats(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	sources, err := KnowledgeKB.Sources(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	found, err := KnowledgeKB.Search(context.Background(), KnowledgeSearchOptions{Query: "SDK", Limit: 3})
	if err != nil {
		t.Fatal(err)
	}
	ingested, err := KnowledgeKB.Ingest(context.Background(), KnowledgeIngestRequest{Name: "inline", Content: "hello"})
	if err != nil {
		t.Fatal(err)
	}
	updated, err := KnowledgeKB.UpdateSource(context.Background(), KnowledgeUpdateSourceRequest{ID: "src_2", Name: "inline-updated"})
	if err != nil {
		t.Fatal(err)
	}
	deleted, err := KnowledgeKB.DeleteSource(context.Background(), "src_2")
	if err != nil {
		t.Fatal(err)
	}
	importedURL, err := KnowledgeKB.ImportURL(context.Background(), KnowledgeImportURLRequest{URL: "https://example.test"})
	if err != nil {
		t.Fatal(err)
	}
	importedRepo, err := KnowledgeKB.ImportRepo(context.Background(), KnowledgeImportRepoRequest{Path: "."})
	if err != nil {
		t.Fatal(err)
	}
	if stats["chunks"].(float64) != 8 || len(sources.Sources) != 1 || found.Count != 1 || ingested.Source.ID != "src_2" || updated.Source.Name != "inline-updated" || deleted.Deleted != "src_2" || len(importedURL.Sources) != 1 || importedRepo.Source.ID != "src_repo" {
		t.Fatalf("unexpected knowledge results: stats=%+v sources=%+v found=%+v ingested=%+v updated=%+v deleted=%+v url=%+v repo=%+v", stats, sources, found, ingested, updated, deleted, importedURL, importedRepo)
	}
	if len(seen) != 8 {
		t.Fatalf("expected 8 requests, got %d: %v", len(seen), seen)
	}
}

func TestWorkflowsNamespaceRunsDefinitionsAndInstances(t *testing.T) {
	var seen []string
	withTestAPI(t, func(w http.ResponseWriter, r *http.Request) {
		seen = append(seen, r.Method+" "+r.URL.String())
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/v1/workflows":
			switch r.Method {
			case http.MethodGet:
				if r.URL.Query().Get("id") == "wf_1" {
					_, _ = w.Write([]byte(`{"id":"wf_1","name":"SDK flow"}`))
					return
				}
				_, _ = w.Write([]byte(`{"workflows":[{"id":"wf_1","name":"SDK flow"}],"total":1}`))
			case http.MethodPost:
				var body WorkflowDefinition
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Fatal(err)
				}
				if body.Name != "SDK flow" {
					t.Fatalf("unexpected workflow body: %+v", body)
				}
				_, _ = w.Write([]byte(`{"id":"wf_1","name":"SDK flow"}`))
			case http.MethodDelete:
				_, _ = w.Write([]byte(`{"deleted":"wf_1"}`))
			}
		case "/v1/workflows/run":
			var body WorkflowRunRequest
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			if body.DefinitionID != "wf_1" {
				t.Fatalf("unexpected run body: %+v", body)
			}
			_, _ = w.Write([]byte(`{"status":"accepted","instance_id":"inst_1","instance":{"id":"inst_1","definition_id":"wf_1","status":"pending"}}`))
		case "/v1/workflows/instances":
			if r.URL.Query().Get("id") == "inst_1" {
				_, _ = w.Write([]byte(`{"id":"inst_1","definition_id":"wf_1","status":"running"}`))
				return
			}
			_, _ = w.Write([]byte(`{"instances":[{"id":"inst_1","definition_id":"wf_1","status":"running"}],"total":1}`))
		case "/v1/workflows/cancel":
			_, _ = w.Write([]byte(`{"status":"cancelling","instance_id":"inst_1"}`))
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
		}
	})

	list, err := Workflows.List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	got, err := Workflows.Get(context.Background(), "wf_1")
	if err != nil {
		t.Fatal(err)
	}
	saved, err := Workflows.Save(context.Background(), WorkflowDefinition{Name: "SDK flow"})
	if err != nil {
		t.Fatal(err)
	}
	run, err := Workflows.Run(context.Background(), WorkflowRunRequest{DefinitionID: "wf_1", Variables: map[string]any{"topic": "sdk"}})
	if err != nil {
		t.Fatal(err)
	}
	instances, err := Workflows.Instances(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	instance, err := Workflows.GetInstance(context.Background(), "inst_1")
	if err != nil {
		t.Fatal(err)
	}
	cancelled, err := Workflows.Cancel(context.Background(), WorkflowCancelRequest{InstanceID: "inst_1"})
	if err != nil {
		t.Fatal(err)
	}
	deleted, err := Workflows.Delete(context.Background(), "wf_1")
	if err != nil {
		t.Fatal(err)
	}

	if list.Total != 1 || got.Name != "SDK flow" || saved.ID != "wf_1" || run.InstanceID != "inst_1" || instances.Total != 1 || instance.Status != "running" || cancelled.Status != "cancelling" || deleted.Deleted != "wf_1" {
		t.Fatalf("unexpected workflow results")
	}
	if len(seen) != 8 {
		t.Fatalf("expected 8 requests, got %d: %v", len(seen), seen)
	}
}

func TestConnectorsNamespaceManagesCatalogAuthAndActions(t *testing.T) {
	var seen []string
	withTestAPI(t, func(w http.ResponseWriter, r *http.Request) {
		seen = append(seen, r.Method+" "+r.URL.String())
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/connectors":
			_, _ = w.Write([]byte(`{"connectors":[{"id":"github","name":"GitHub","supported":true,"status":"disconnected","action_count":2}]}`))
		case "/api/connectors/detail":
			if r.URL.Query().Get("id") != "github" {
				t.Fatalf("unexpected detail query: %s", r.URL.RawQuery)
			}
			_, _ = w.Write([]byte(`{"connector":{"id":"github","name":"GitHub","actions":[{"id":"create_issue"}]},"supported":true,"status":"disconnected"}`))
		case "/api/connectors/connect":
			var body ConnectorConnectRequest
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			if body.ConnectorID != "github" || body.Token != "oauth" {
				t.Fatalf("unexpected connect body: %+v", body)
			}
			_, _ = w.Write([]byte(`{"ok":true,"status":"connected","user_info":"octocat"}`))
		case "/api/connectors/disconnect":
			_, _ = w.Write([]byte(`{"ok":true}`))
		case "/api/connectors/execute":
			var body ConnectorExecuteRequest
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			if body.ConnectorID != "github" || body.ActionID != "create_issue" || body.Params["title"] != "SDK" {
				t.Fatalf("unexpected execute body: %+v", body)
			}
			_, _ = w.Write([]byte(`{"ok":true,"result":{"issue":1}}`))
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
		}
	})

	list, err := Connectors.List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	detail, err := Connectors.Detail(context.Background(), "github")
	if err != nil {
		t.Fatal(err)
	}
	connected, err := Connectors.Connect(context.Background(), ConnectorConnectRequest{ConnectorID: "github", Token: "oauth"})
	if err != nil {
		t.Fatal(err)
	}
	disconnected, err := Connectors.Disconnect(context.Background(), "github")
	if err != nil {
		t.Fatal(err)
	}
	executed, err := Connectors.Execute(context.Background(), ConnectorExecuteRequest{ConnectorID: "github", ActionID: "create_issue", Params: map[string]any{"title": "SDK"}})
	if err != nil {
		t.Fatal(err)
	}
	result := executed.Result.(map[string]any)
	if list.Connectors[0].ID != "github" || detail.Connector.Actions[0].ID != "create_issue" || connected.Status != "connected" || !disconnected.OK || result["issue"].(float64) != 1 {
		t.Fatalf("unexpected connector results")
	}
	if len(seen) != 5 {
		t.Fatalf("expected 5 requests, got %d: %v", len(seen), seen)
	}
}

func TestLoRALifecycleHelpers(t *testing.T) {
	var seen []string
	withTestAPI(t, func(w http.ResponseWriter, r *http.Request) {
		seen = append(seen, r.Method+" "+r.URL.String())
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/v1/lora/status":
			_, _ = w.Write([]byte(`{"active_model":"adapter-a","rolling_success_rate":0.8}`))
		case "/v1/lora/history":
			_, _ = w.Write([]byte(`{"records":[{"adapter":"a1"}],"count":1}`))
		case "/v1/lora/summary":
			_, _ = w.Write([]byte(`{"summary":{"best_score":0.9}}`))
		case "/v1/lora/preview":
			if r.URL.Query().Get("tenant_id") != "default" {
				t.Fatalf("unexpected preview query: %s", r.URL.RawQuery)
			}
			_, _ = w.Write([]byte(`{"preview":{"ready":true,"tenant_id":"default"}}`))
		case "/v1/lora/trigger":
			var body TriggerLoRARequest
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			if body.TenantID != "default" {
				t.Fatalf("unexpected trigger body: %+v", body)
			}
			_, _ = w.Write([]byte(`{"status":"ok","tenant_id":"default"}`))
		case "/v1/lora/rollback":
			_, _ = w.Write([]byte(`{"status":"ok"}`))
		case "/v1/lora/evolution":
			_, _ = w.Write([]byte(`{"state":{"phase":"eval"}}`))
		case "/v1/lora/config":
			if r.Method == http.MethodPut {
				var body map[string]any
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Fatal(err)
				}
				if body["min_samples"].(float64) != 9 {
					t.Fatalf("unexpected config body: %+v", body)
				}
				_, _ = w.Write([]byte(`{"config":{"min_samples":9},"status":"updated"}`))
				return
			}
			_, _ = w.Write([]byte(`{"config":{"min_samples":8}}`))
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
		}
	})

	status, err := LoRA.Status(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	history, err := LoRA.History(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	summary, err := LoRA.Summary(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	preview, err := LoRA.Preview(context.Background(), LoRAPreviewOptions{TenantID: "default"})
	if err != nil {
		t.Fatal(err)
	}
	triggered, err := LoRA.Trigger(context.Background(), TriggerLoRARequest{TenantID: "default"})
	if err != nil {
		t.Fatal(err)
	}
	rolledBack, err := LoRA.Rollback(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	evolution, err := LoRA.Evolution(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	config, err := LoRA.Config(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	updated, err := LoRA.UpdateConfig(context.Background(), LoRAConfig{"min_samples": 9})
	if err != nil {
		t.Fatal(err)
	}

	if status["active_model"] != "adapter-a" || history["count"].(float64) != 1 || summary["summary"].(map[string]any)["best_score"].(float64) != 0.9 || preview["preview"].(map[string]any)["ready"] != true || triggered["status"] != "ok" || rolledBack["status"] != "ok" || evolution["state"].(map[string]any)["phase"] != "eval" || config["config"].(map[string]any)["min_samples"].(float64) != 8 || updated["status"] != "updated" {
		t.Fatalf("unexpected lora results")
	}
	if len(seen) != 9 {
		t.Fatalf("expected 9 requests, got %d: %v", len(seen), seen)
	}
}

func TestPluginRuntimeNamespaceDelegatesExtensionRegistration(t *testing.T) {
	withTestAPI(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/plugin-api/register/provider" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if body["id"] != "local" || body["base_url"] != "http://localhost:11434/v1" || body["model"] != "llama3" || body["type"] != "chat" {
			t.Fatalf("unexpected provider body: %+v", body)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true,"provider_id":"local"}`))
	})

	if err := Plugin.RegisterProvider(context.Background(), "local", "http://localhost:11434/v1", "llama3"); err != nil {
		t.Fatal(err)
	}
}

func TestSchedulerJobsAddAndRemove(t *testing.T) {
	withTestAPI(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/v1/scheduler/jobs":
			if r.Method != http.MethodGet {
				t.Fatalf("unexpected jobs method: %s", r.Method)
			}
			_, _ = w.Write([]byte(`{"jobs":[{"id":"job_1","name":"daily"}],"count":1}`))
		case "/v1/scheduler/add":
			if r.Method != http.MethodPost {
				t.Fatalf("unexpected add method: %s", r.Method)
			}
			var body map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			if body["name"] != "hourly" || body["prompt"] != "检查任务" || body["interval"] != "1h" {
				t.Fatalf("unexpected add body: %+v", body)
			}
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"id":"job_2","name":"hourly","tenant_id":"default","interval":3600000000000,"prompt":"检查任务"}`))
		case "/v1/scheduler/remove":
			if r.Method != http.MethodPost {
				t.Fatalf("unexpected remove method: %s", r.Method)
			}
			var body map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			if body["id"] != "job_1" {
				t.Fatalf("unexpected remove body: %+v", body)
			}
			_, _ = w.Write([]byte(`{"status":"removed"}`))
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
		}
	})

	jobs, err := Scheduler.Jobs(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	added, err := Scheduler.Add(context.Background(), "hourly", "检查任务", "1h")
	if err != nil {
		t.Fatal(err)
	}
	removed, err := Scheduler.Remove(context.Background(), "job_1")
	if err != nil {
		t.Fatal(err)
	}
	if jobs.Count != 1 || added.ID != "job_2" || removed.Status != "removed" {
		t.Fatalf("unexpected scheduler results: jobs=%+v added=%+v removed=%+v", jobs, added, removed)
	}
}

func TestCronSystemListAddRemoveAndRun(t *testing.T) {
	withTestAPI(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/v1/cron/list":
			_, _ = w.Write([]byte(`{"jobs":[{"id":"cron_1","name":"daily","schedule":{"type":"every","every_ms":60000},"payload":{"kind":"agentTurn","message":"ping"},"enabled":true,"created_at":"2026-05-12T00:00:00Z","run_count":0}]}`))
		case "/v1/cron/add":
			var body CronAddRequest
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			if body.Name != "nightly" || body.Schedule.CronExpr != "0 2 * * *" || body.Payload.Kind != "systemEvent" {
				t.Fatalf("unexpected cron add body: %+v", body)
			}
			_, _ = w.Write([]byte(`{"job":{"id":"cron_2","name":"nightly","schedule":{"type":"cron","cron_expr":"0 2 * * *","timezone":"Asia/Shanghai"},"payload":{"kind":"systemEvent"},"enabled":true,"created_at":"2026-05-12T00:00:00Z","run_count":0}}`))
		case "/v1/cron/remove":
			if r.URL.Query().Get("id") != "cron_1" {
				t.Fatalf("unexpected remove query: %s", r.URL.RawQuery)
			}
			_, _ = w.Write([]byte(`{"deleted":"cron_1"}`))
		case "/v1/cron/run":
			if r.URL.Query().Get("id") != "cron_1" {
				t.Fatalf("unexpected run query: %s", r.URL.RawQuery)
			}
			_, _ = w.Write([]byte(`{"run":{"job_id":"cron_1","run_id":"run_1","started_at":"2026-05-12T00:00:00Z","ended_at":"2026-05-12T00:00:01Z","status":"success","output":"ok"}}`))
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
		}
	})

	jobs, err := CronSystem.List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	added, err := CronSystem.Add(context.Background(), CronAddRequest{Name: "nightly", Schedule: CronSchedule{Type: "cron", CronExpr: "0 2 * * *", Timezone: "Asia/Shanghai"}, Payload: CronPayload{Kind: "systemEvent"}})
	if err != nil {
		t.Fatal(err)
	}
	removed, err := CronSystem.Remove(context.Background(), "cron_1")
	if err != nil {
		t.Fatal(err)
	}
	run, err := CronSystem.Run(context.Background(), "cron_1")
	if err != nil {
		t.Fatal(err)
	}

	if len(jobs.Jobs) != 1 || added.Job.ID != "cron_2" || removed.Deleted != "cron_1" || run.Run.Status != "success" {
		t.Fatalf("unexpected cron system results: jobs=%+v added=%+v removed=%+v run=%+v", jobs, added, removed, run)
	}
}

func TestTriggersListEmitHistoryAndControl(t *testing.T) {
	var seen []string
	withTestAPI(t, func(w http.ResponseWriter, r *http.Request) {
		seen = append(seen, r.Method+" "+r.URL.String())
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/v1/triggers/v2":
			switch r.Method {
			case http.MethodGet:
				if r.URL.Query().Get("tenant_id") != "default" || r.URL.Query().Get("type") != "event" || r.URL.Query().Get("status") != "enabled" {
					t.Fatalf("unexpected trigger query: %s", r.URL.RawQuery)
				}
				_, _ = w.Write([]byte(`{"triggers":[{"id":"tr_1","name":"review done","tenant_id":"default","type":"event","status":"enabled","actions":[{"kind":"notify"}]}],"total":1}`))
			case http.MethodPost, http.MethodPut:
				var body TriggerDef
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Fatal(err)
				}
				if body.Name != "review done" {
					t.Fatalf("unexpected trigger body: %+v", body)
				}
				body.ID = "tr_1"
				_ = json.NewEncoder(w).Encode(body)
			case http.MethodDelete:
				if r.URL.Query().Get("id") != "tr_1" {
					t.Fatalf("unexpected delete query: %s", r.URL.RawQuery)
				}
				_, _ = w.Write([]byte(`{"deleted":"tr_1"}`))
			default:
				t.Fatalf("unexpected method: %s", r.Method)
			}
		case "/v1/triggers/v2/emit":
			var body TriggerPayload
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			if body.Event != "review.done" || body.Data["task_id"] != "task_1" {
				t.Fatalf("unexpected emit body: %+v", body)
			}
			_, _ = w.Write([]byte(`{"status":"emitted","event":"review.done"}`))
		case "/v1/triggers/v2/runs":
			if r.URL.Query().Get("trigger_id") != "tr_1" || r.URL.Query().Get("limit") != "2" {
				t.Fatalf("unexpected runs query: %s", r.URL.RawQuery)
			}
			_, _ = w.Write([]byte(`{"runs":[{"id":"run_1"}],"total":1}`))
		case "/v1/triggers/v2/events":
			_, _ = w.Write([]byte(`{"events":[{"event":"review.done"}],"total":1}`))
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
		}
	})

	list, err := Triggers.List(context.Background(), TriggerListOptions{TenantID: "default", Type: "event", Status: "enabled"})
	if err != nil {
		t.Fatal(err)
	}
	created, err := Triggers.Create(context.Background(), TriggerDef{Name: "review done", TenantID: "default", Type: "event", Status: "enabled", Actions: []any{map[string]any{"kind": "notify"}}})
	if err != nil {
		t.Fatal(err)
	}
	updated, err := Triggers.Update(context.Background(), created)
	if err != nil {
		t.Fatal(err)
	}
	emitted, err := Triggers.Emit(context.Background(), TriggerPayload{Event: "review.done", Data: map[string]any{"task_id": "task_1"}})
	if err != nil {
		t.Fatal(err)
	}
	runs, err := Triggers.Runs(context.Background(), TriggerHistoryOptions{TriggerID: "tr_1", Limit: 2})
	if err != nil {
		t.Fatal(err)
	}
	events, err := Triggers.Events(context.Background(), TriggerHistoryOptions{})
	if err != nil {
		t.Fatal(err)
	}
	deleted, err := Triggers.Delete(context.Background(), "tr_1")
	if err != nil {
		t.Fatal(err)
	}

	if list.Total != 1 || created.ID != "tr_1" || updated.Name != "review done" || emitted.Status != "emitted" || runs.Total != 1 || events.Total != 1 || deleted.Deleted != "tr_1" {
		t.Fatalf("unexpected trigger results: list=%+v created=%+v updated=%+v emitted=%+v runs=%+v events=%+v deleted=%+v", list, created, updated, emitted, runs, events, deleted)
	}
	if len(seen) != 8 {
		t.Fatalf("expected 8 requests, got %d: %v", len(seen), seen)
	}
}

func withTestAPI(t *testing.T, handler http.HandlerFunc) {
	t.Helper()
	oldBase := apiBase
	oldClient := httpClient
	oldToken := pluginToken
	oldName := pluginName
	server := httptest.NewServer(handler)
	t.Cleanup(func() {
		server.Close()
		apiBase = oldBase
		httpClient = oldClient
		pluginToken = oldToken
		pluginName = oldName
	})
	apiBase = server.URL
	httpClient = server.Client()
	pluginToken = "test-token"
	pluginName = "state-plugin"
}
