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
