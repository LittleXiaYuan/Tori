package yunque

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestBreakerHelpers(t *testing.T) {
	withTestAPI(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/breaker/reset" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true,"reset_count":2}`))
	})

	reset, err := Breaker.Reset(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if reset["reset_count"].(float64) != 2 || NewAgentKit().Breaker != Breaker {
		t.Fatalf("unexpected breaker reset: %+v", reset)
	}
}

func TestAiriHelpers(t *testing.T) {
	var seen []string
	withTestAPI(t, func(w http.ResponseWriter, r *http.Request) {
		seen = append(seen, r.Method+" "+r.URL.String())
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/v1/ext/airi/status":
			_, _ = w.Write([]byte(`{"plugin":"airi","connected":false}`))
		case "/v1/ext/airi/models":
			_, _ = w.Write([]byte(`{"object":"list","data":[{"id":"yunque-airi"}]}`))
		case "/v1/ext/airi/chat/completions":
			var body AiriChatCompletionRequest
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			if body.Model != "yunque-airi" || body.Stream {
				t.Fatalf("unexpected Airi body: %+v", body)
			}
			_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"hi"}}]}`))
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
		}
	})

	status, err := Airi.Status(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	models, err := Airi.Models(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	reply, err := Airi.ChatCompletions(context.Background(), AiriChatCompletionRequest{Model: "yunque-airi", Stream: true, Messages: []AiriChatMessage{{Role: "user", Content: "hi"}}})
	if err != nil {
		t.Fatal(err)
	}
	streamReq := Airi.StreamRequest(AiriChatCompletionRequest{Messages: []AiriChatMessage{{Role: "user", Content: "hi"}}})
	items := Airi.ParseStream("data: {\"choices\":[{\"delta\":{\"content\":\"hi\"}}]}\n\ndata: [DONE]\n\n")
	if status["plugin"] != "airi" || models.Data[0].ID != "yunque-airi" || reply["choices"] == nil || !streamReq.Stream || len(items) != 2 || items[1].Kind != "done" || NewAgentKit().Airi != Airi {
		t.Fatalf("unexpected Airi results: status=%+v models=%+v reply=%+v items=%+v", status, models, reply, items)
	}
	if len(seen) != 3 {
		t.Fatalf("expected 3 requests, got %d: %v", len(seen), seen)
	}
}

func TestBackupHelpers(t *testing.T) {
	var seen []string
	withTestAPI(t, func(w http.ResponseWriter, r *http.Request) {
		seen = append(seen, r.Method+" "+r.URL.String())
		switch r.URL.Path {
		case "/v1/backup/info":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"file_count":1,"total_bytes":12}`))
		case "/v1/backup/export":
			w.Header().Set("Content-Type", "application/zip")
			w.Header().Set("Content-Disposition", `attachment; filename="backup.zip"`)
			_, _ = w.Write([]byte("zipdata"))
		case "/v1/backup/import":
			if err := r.ParseMultipartForm(1 << 20); err != nil {
				t.Fatal(err)
			}
			if _, _, err := r.FormFile("backup"); err != nil {
				t.Fatal(err)
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"success":true,"restored":2}`))
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	})

	ctx := context.Background()
	info, _ := Backup.Info(ctx)
	exported, _ := Backup.Export(ctx)
	imported, _ := Backup.Import(ctx, []byte("zipdata"), "restore.zip")
	if info["file_count"].(float64) != 1 || string(exported.Data) != "zipdata" || exported.Filename != "backup.zip" || imported["restored"].(float64) != 2 || NewAgentKit().Backup != Backup {
		t.Fatalf("unexpected backup results")
	}
	if len(seen) != 3 {
		t.Fatalf("expected 3 backup API calls, got %d: %v", len(seen), seen)
	}
}

func TestToriHelpers(t *testing.T) {
	var seen []string
	withTestAPI(t, func(w http.ResponseWriter, r *http.Request) {
		seen = append(seen, r.Method+" "+r.URL.String())
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/v1/tori/bind":
			var body ToriBindRequest
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			if body.ToriURL != "https://tori.example" {
				t.Fatalf("unexpected bind body: %+v", body)
			}
			_, _ = w.Write([]byte(`{"status":"pending","authorize_url":"https://tori.example/oauth"}`))
		case "/v1/tori/status":
			_, _ = w.Write([]byte(`{"bound":true,"username":"alice"}`))
		case "/v1/tori/unbind":
			_, _ = w.Write([]byte(`{"status":"unbound"}`))
		case "/v1/tori/health":
			_, _ = w.Write([]byte(`{"status":"ok"}`))
		case "/v1/tori/usage":
			_, _ = w.Write([]byte(`{"total_tokens":12}`))
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
		}
	})

	ctx := context.Background()
	bind, _ := Tori.Bind(ctx, ToriBindRequest{ToriURL: "https://tori.example"})
	status, _ := Tori.Status(ctx)
	unbind, _ := Tori.Unbind(ctx)
	health, _ := Tori.Health(ctx)
	usage, _ := Tori.Usage(ctx)
	if bind["status"] != "pending" || status["bound"] != true || unbind["status"] != "unbound" || health["status"] != "ok" || usage["total_tokens"].(float64) != 12 || NewAgentKit().Tori != Tori {
		t.Fatalf("unexpected tori results")
	}
	if len(seen) != 5 {
		t.Fatalf("expected 5 tori API calls, got %d: %v", len(seen), seen)
	}
}

func TestSpeechHelpers(t *testing.T) {
	var seen []string
	withTestAPI(t, func(w http.ResponseWriter, r *http.Request) {
		seen = append(seen, r.Method+" "+r.URL.String())
		switch r.URL.Path {
		case "/v1/speech/tts":
			var body SpeechTTSRequest
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			if body.Text != "你好" || body.Voice != "v1" || body.Format != "wav" || body.Emotion != "happy" {
				t.Fatalf("unexpected tts body: %+v", body)
			}
			w.Header().Set("Content-Type", "audio/wav")
			_, _ = w.Write([]byte("audio"))
		case "/v1/speech/stt":
			if r.URL.Query().Get("language") != "en" || r.URL.Query().Get("detect_emotion") != "true" {
				t.Fatalf("unexpected stt query: %s", r.URL.RawQuery)
			}
			audio, _ := io.ReadAll(r.Body)
			if string(audio) != "audio" {
				t.Fatalf("unexpected stt body: %q", string(audio))
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"text":"hello","emotion":{"label":"calm"}}`))
		case "/v1/speech/voices":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"voices":[{"id":"v1"}],"providers":["mock"]}`))
		case "/v1/upload":
			if err := r.ParseMultipartForm(1 << 20); err != nil {
				t.Fatal(err)
			}
			if _, _, err := r.FormFile("file"); err != nil {
				t.Fatal(err)
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"filename":"note.txt","size":4}`))
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
		}
	})

	ctx := context.Background()
	tts, _ := Speech.TTS(ctx, SpeechTTSRequest{Text: "你好", Voice: "v1", Format: "wav", Emotion: "happy"})
	stt, _ := Speech.STT(ctx, []byte("audio"), SpeechSTTOptions{Language: "en", DetectEmotion: true})
	voices, _ := Speech.Voices(ctx)
	upload, _ := Speech.Upload(ctx, []byte("demo"), "note.txt")
	streamURL := Speech.STTStreamURL("zh", true)
	if string(tts.Data) != "audio" || tts.ContentType != "audio/wav" || stt["text"] != "hello" || voices["providers"].([]any)[0] != "mock" || upload["filename"] != "note.txt" || !strings.Contains(streamURL, "/v1/speech/stt/stream?") || NewAgentKit().Speech != Speech {
		t.Fatalf("unexpected speech results")
	}
	if len(seen) != 4 {
		t.Fatalf("expected 4 speech API calls, got %d: %v", len(seen), seen)
	}
}

func TestUploadHelpers(t *testing.T) {
	withTestAPI(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/upload" || r.Method != http.MethodPost {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
		}
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			t.Fatal(err)
		}
		file, header, err := r.FormFile("file")
		if err != nil {
			t.Fatal(err)
		}
		defer file.Close()
		body, _ := io.ReadAll(file)
		if header.Filename != "apply.docx" || string(body) != "doc" {
			t.Fatalf("unexpected upload file: %s %q", header.Filename, string(body))
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"filename":"apply.docx","parse":{"status":"parsed"},"actions":[]}`))
	})
	out, err := Upload.File(context.Background(), []byte("doc"), "apply.docx")
	if err != nil {
		t.Fatal(err)
	}
	if out["filename"] != "apply.docx" || NewAgentKit().Upload != Upload {
		t.Fatalf("unexpected upload result")
	}
}

func TestSetupHelpers(t *testing.T) {
	var seen []string
	withTestAPI(t, func(w http.ResponseWriter, r *http.Request) {
		seen = append(seen, r.Method+" "+r.URL.String())
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/v1/setup/detect":
			_, _ = w.Write([]byte(`{"has_docker":true,"has_ollama":true}`))
		case "/v1/setup/health":
			_, _ = w.Write([]byte(`{"providers":[{"id":"ollama","available":true}]}`))
		case "/v1/setup/templates":
			_, _ = w.Write([]byte(`{"templates":[{"id":"local"}],"count":1}`))
		case "/v1/setup/test-provider":
			var body SetupTestProviderRequest
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			if body.BaseURL != "http://127.0.0.1:11434" || body.Model != "qwen" {
				t.Fatalf("unexpected setup provider body: %+v", body)
			}
			_, _ = w.Write([]byte(`{"ok":true}`))
		case "/v1/setup/apply":
			var body SetupApplyRequest
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			if body.TemplateID != "local" || body.BaseURL == "" || body.Model == "" {
				t.Fatalf("unexpected setup apply body: %+v", body)
			}
			_, _ = w.Write([]byte(`{"status":"applied","restart_required":true}`))
		case "/v1/setup/install-component":
			_, _ = w.Write([]byte(`{"success":true,"message":"installed"}`))
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
		}
	})

	ctx := context.Background()
	detect, _ := Setup.Detect(ctx)
	health, _ := Setup.Health(ctx)
	templates, _ := Setup.Templates(ctx)
	tested, _ := Setup.TestProvider(ctx, SetupTestProviderRequest{BaseURL: "http://127.0.0.1:11434", Model: "qwen"})
	applied, _ := Setup.Apply(ctx, SetupApplyRequest{TemplateID: "local", BaseURL: "http://127.0.0.1:11434", Model: "qwen", Overrides: map[string]any{"sandbox_tier": "local"}})
	installed, _ := Setup.InstallComponent(ctx, "python_office")
	if detect["has_docker"] != true || len(health["providers"].([]any)) != 1 || templates["count"].(float64) != 1 || tested["ok"] != true || applied["status"] != "applied" || installed["success"] != true || NewAgentKit().Setup != Setup {
		t.Fatalf("unexpected setup results")
	}
	if len(seen) != 6 {
		t.Fatalf("expected 6 setup API calls, got %d: %v", len(seen), seen)
	}
}

func TestDiscoveryHelpers(t *testing.T) {
	var seen []string
	withTestAPI(t, func(w http.ResponseWriter, r *http.Request) {
		seen = append(seen, r.Method+" "+r.URL.String())
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/v1/identity/resolve":
			var body DiscoveryResolveIdentityRequest
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			if body.Channel != "feishu" || body.UserID != "42" {
				t.Fatalf("unexpected identity body: %+v", body)
			}
			_, _ = w.Write([]byte(`{"unified_id":"u1","display_name":"小羽"}`))
		case "/v1/identity/profiles":
			_, _ = w.Write([]byte(`{"profiles":[{"unified_id":"u1"}]}`))
		case "/v1/embeddings":
			if r.Method == http.MethodGet {
				_, _ = w.Write([]byte(`{"providers":["mock"]}`))
				return
			}
			var body map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			if body["text"] != "云雀" || body["provider"] != "mock" {
				t.Fatalf("unexpected embed body: %+v", body)
			}
			_, _ = w.Write([]byte(`{"embedding":[0.1,0.2],"dimensions":2}`))
		case "/v1/search":
			if r.URL.Query().Get("q") != "planner" || r.URL.Query().Get("limit") != "3" || r.URL.Query().Get("provider") != "bing" {
				t.Fatalf("unexpected search query: %s", r.URL.RawQuery)
			}
			_, _ = w.Write([]byte(`{"results":[{"title":"云雀"}]}`))
		case "/v1/search/providers":
			_, _ = w.Write([]byte(`{"enabled":true,"providers":["bing"]}`))
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
		}
	})

	ctx := context.Background()
	profile, err := Discovery.ResolveIdentity(ctx, DiscoveryResolveIdentityRequest{Channel: "feishu", UserID: "42", DisplayName: "小羽"})
	if err != nil {
		t.Fatal(err)
	}
	profiles, err := Discovery.IdentityProfiles(ctx)
	if err != nil {
		t.Fatal(err)
	}
	embedProviders, err := Discovery.EmbeddingProviders(ctx)
	if err != nil {
		t.Fatal(err)
	}
	embedded, err := Discovery.Embed(ctx, "云雀", "mock")
	if err != nil {
		t.Fatal(err)
	}
	searched, err := Discovery.Search(ctx, "planner", 3, "bing")
	if err != nil {
		t.Fatal(err)
	}
	searchProviders, err := Discovery.SearchProviders(ctx)
	if err != nil {
		t.Fatal(err)
	}
	kit := NewAgentKit()

	if profile["unified_id"] != "u1" || profiles["profiles"] == nil || embedProviders["providers"] == nil || embedded["dimensions"].(float64) != 2 || searched["results"] == nil || searchProviders["enabled"] != true || kit.Discovery != Discovery {
		t.Fatalf("unexpected discovery results")
	}
	if len(seen) != 6 || seen[0] != "POST /v1/identity/resolve" || seen[4] != "GET /v1/search?limit=3&provider=bing&q=planner" || seen[5] != "GET /v1/search/providers" {
		t.Fatalf("unexpected discovery requests: %v", seen)
	}
}

func TestIDEHelpers(t *testing.T) {
	var seen []string
	withTestAPI(t, func(w http.ResponseWriter, r *http.Request) {
		seen = append(seen, r.Method+" "+r.URL.String())
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/v1/ide/status":
			_, _ = w.Write([]byte(`{"connected":true,"capabilities":["review"],"skills_count":3}`))
		case "/v1/ide/review":
			var body IDEReviewRequest
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			if body.Mode == "" {
				t.Fatalf("missing review mode: %+v", body)
			}
			_, _ = w.Write([]byte(`{"summary":"ok","issues":[],"score":9}`))
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
		}
	})
	ctx := context.Background()
	status, _ := IDE.Status(ctx)
	review, _ := IDE.Review(ctx, IDEReviewRequest{FilePath: "main.go", Content: "package main", Language: "go", Mode: "full"})
	diff, _ := IDE.ReviewDiff(ctx, "+fmt.Println(1)", "main.go", "go")
	quick, _ := IDE.ReviewQuick(ctx, "console.log(1)", "main.ts", "ts")
	full, _ := IDE.ReviewFull(ctx, "print(1)", "main.py", "py")
	if status["connected"] != true || review["score"].(float64) != 9 || diff["summary"] != "ok" || quick["summary"] != "ok" || full["summary"] != "ok" || NewAgentKit().IDE != IDE {
		t.Fatalf("unexpected ide results")
	}
	if len(seen) != 5 {
		t.Fatalf("expected 5 ide API calls, got %d: %v", len(seen), seen)
	}
}

func TestPlannerHelpers(t *testing.T) {
	var seen []string
	withTestAPI(t, func(w http.ResponseWriter, r *http.Request) {
		seen = append(seen, r.Method+" "+r.URL.String())
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/v1/planner/checkpoints":
			if r.URL.Query().Get("plan_id") != "plan-1" || r.URL.Query().Get("include_snapshot") != "true" {
				t.Fatalf("unexpected checkpoints query: %s", r.URL.RawQuery)
			}
			_, _ = w.Write([]byte(`{"checkpoints":[{"plan_id":"plan-1"}],"count":1}`))
		case "/v1/planner/checkpoints/recover":
			_, _ = w.Write([]byte(`{"action":"retry_failed","plan_id":"plan-1"}`))
		case "/v1/planner/checkpoints/resume":
			_, _ = w.Write([]byte(`{"status":"accepted","task_id":"task-1"}`))
		case "/v1/planner/checkpoints/resume-plan":
			_, _ = w.Write([]byte(`{"status":"accepted","plan_id":"plan-1","job_id":"job-1"}`))
		case "/v1/planner/checkpoints/resume-plan/jobs":
			if r.URL.Query().Get("job_id") != "job-1" {
				t.Fatalf("unexpected job query: %s", r.URL.RawQuery)
			}
			_, _ = w.Write([]byte(`{"job":{"id":"job-1","status":"running"}}`))
		case "/v1/planner/execution-state":
			if r.URL.Query().Get("action") != "retry_failed" {
				t.Fatalf("unexpected state query: %s", r.URL.RawQuery)
			}
			_, _ = w.Write([]byte(`{"plan_id":"plan-1","next_action":"retry_failed"}`))
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
		}
	})
	ctx := context.Background()
	checkpoints, _ := Planner.ListCheckpoints(ctx, PlannerCheckpointQuery{Limit: 5, PlanID: "plan-1", IncludeSnapshot: true})
	recovered, _ := Planner.RecoverCheckpoint(ctx, PlannerRecoveryRequest{PlanID: "plan-1", Action: "retry_failed"})
	resumedTask, _ := Planner.ResumeCheckpointTask(ctx, PlannerResumeTaskRequest{PlanID: "plan-1", Action: "continue", Run: false})
	resumedPlan, _ := Planner.ResumeCheckpointPlan(ctx, PlannerResumePlanRequest{PlanID: "plan-1", Action: "continue", Async: true})
	job, _ := Planner.GetResumePlanJob(ctx, PlannerResumePlanJobQuery{JobID: "job-1"})
	state, _ := Planner.ExecutionState(ctx, PlannerExecutionStateQuery{PlanID: "plan-1", Action: "retry_failed"})
	if checkpoints["count"].(float64) != 1 || recovered["action"] != "retry_failed" || resumedTask["status"] != "accepted" || resumedPlan["job_id"] != "job-1" || job["job"] == nil || state["next_action"] != "retry_failed" || NewAgentKit().Planner != Planner {
		t.Fatalf("unexpected planner results")
	}
	if len(seen) != 6 {
		t.Fatalf("expected 6 planner API calls, got %d: %v", len(seen), seen)
	}
}

func TestFederationHelpers(t *testing.T) {
	var seen []string
	withTestAPI(t, func(w http.ResponseWriter, r *http.Request) {
		seen = append(seen, r.Method+" "+r.URL.String())
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/v1/federation/peers":
			_, _ = w.Write([]byte(`{"local_id":"agent-local","peers":[{"id":"peer-a"}]}`))
		case "/v1/federation/stats":
			_, _ = w.Write([]byte(`{"peers":1,"messages":2}`))
		case "/v1/federation/capabilities":
			if r.Method == http.MethodPost {
				_, _ = w.Write([]byte(`{"status":"updated"}`))
				return
			}
			_, _ = w.Write([]byte(`{"local":{"agent_id":"agent-a"},"peers":[]}`))
		case "/v1/federation/discover":
			var body FederationDiscoverRequest
			_ = json.NewDecoder(r.Body).Decode(&body)
			if body.Feature != "browser" {
				t.Fatalf("unexpected discover body: %+v", body)
			}
			_, _ = w.Write([]byte(`{"results":[{"peer_id":"p1"}],"count":1}`))
		case "/v1/federation/delegate":
			_, _ = w.Write([]byte(`{"status":"delegated","result":{"task_id":"t1"}}`))
		case "/v1/federation/bridge/stats":
			_, _ = w.Write([]byte(`{"configured":true,"peers":2}`))
		case "/v1/federation/broadcast":
			_, _ = w.Write([]byte(`{"status":"broadcasted"}`))
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
		}
	})
	ctx := context.Background()
	peers, _ := Federation.Peers(ctx)
	stats, _ := Federation.Stats(ctx)
	caps, _ := Federation.Capabilities(ctx)
	updated, _ := Federation.UpdateCapabilities(ctx, FederationCapabilityPayload{"agent_id": "agent-a"})
	found, _ := Federation.Discover(ctx, FederationDiscoverRequest{Feature: "browser"})
	delegated, _ := Federation.Delegate(ctx, FederationDelegatePayload{"peer_id": "p1"})
	bridge, _ := Federation.BridgeStats(ctx)
	broadcast, _ := Federation.Broadcast(ctx)
	if peers["local_id"] != "agent-local" || stats["messages"].(float64) != 2 || caps["local"] == nil || updated["status"] != "updated" || found["count"].(float64) != 1 || delegated["status"] != "delegated" || bridge["configured"] != true || broadcast["status"] != "broadcasted" || NewAgentKit().Federation != Federation {
		t.Fatalf("unexpected federation results")
	}
	if len(seen) != 8 {
		t.Fatalf("expected 8 federation API calls, got %d: %v", len(seen), seen)
	}
}

func TestAdminHelpers(t *testing.T) {
	var seen []string
	withTestAPI(t, func(w http.ResponseWriter, r *http.Request) {
		seen = append(seen, r.Method+" "+r.URL.String())
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/v1/desktop/console":
			_, _ = w.Write([]byte(`{"console_hidden":` + fmt.Sprint(r.Method == http.MethodPost) + `}`))
		case "/v1/desktop/autostart":
			_, _ = w.Write([]byte(`{"autostart_enabled":` + fmt.Sprint(r.Method == http.MethodPost) + `}`))
		case "/v1/tenants":
			if r.Method == http.MethodPost {
				var body AdminCreateTenantRequest
				_ = json.NewDecoder(r.Body).Decode(&body)
				if body.Name != "team" {
					t.Fatalf("unexpected tenant body: %+v", body)
				}
				_, _ = w.Write([]byte(`{"id":"t2","name":"team"}`))
				return
			}
			_, _ = w.Write([]byte(`{"tenants":[{"id":"t1","name":"default"}],"count":1}`))
		case "/v1/nl-config", "/v1/nl-config/translate":
			var body AdminNLConfigRequest
			_ = json.NewDecoder(r.Body).Decode(&body)
			if body.Text == "" {
				t.Fatalf("missing nl config text")
			}
			_, _ = w.Write([]byte(`{"status":"ok","executed":` + fmt.Sprint(body.Execute) + `}`))
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
		}
	})
	ctx := context.Background()
	consoleStatus, _ := Admin.ConsoleStatus(ctx)
	consoleToggle, _ := Admin.ToggleConsole(ctx)
	autostartStatus, _ := Admin.AutostartStatus(ctx)
	autostartToggle, _ := Admin.ToggleAutostart(ctx)
	tenants, _ := Admin.ListTenants(ctx)
	created, _ := Admin.CreateTenant(ctx, "team")
	translated, _ := Admin.NLConfigTranslate(ctx, "切换到 qwen")
	executed, _ := Admin.NLConfig(ctx, "切换到 qwen", true)
	if consoleStatus["console_hidden"] != false || consoleToggle["console_hidden"] != true || autostartStatus["autostart_enabled"] != false || autostartToggle["autostart_enabled"] != true || tenants["count"].(float64) != 1 || created["name"] != "team" || translated["executed"] != false || executed["executed"] != true || NewAgentKit().Admin != Admin {
		t.Fatalf("unexpected admin results")
	}
	if len(seen) != 8 {
		t.Fatalf("expected 8 admin API calls, got %d: %v", len(seen), seen)
	}
}

func TestSettingsHelpers(t *testing.T) {
	var seen []string
	withTestAPI(t, func(w http.ResponseWriter, r *http.Request) {
		seen = append(seen, r.Method+" "+r.URL.String())
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/settings/schema":
			_, _ = w.Write([]byte(`{"groups":[{"id":"llm"}]}`))
		case "/api/settings/config":
			if r.Method == http.MethodPut {
				var body map[string]map[string]string
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Fatal(err)
				}
				if body["values"]["LLM_MODEL"] != "deepseek" {
					t.Fatalf("unexpected settings body: %+v", body)
				}
				_, _ = w.Write([]byte(`{"success":true,"restart_required":true}`))
				return
			}
			_, _ = w.Write([]byte(`{"values":{"LLM_MODEL":"qwen"}}`))
		case "/v1/backup/info":
			_, _ = w.Write([]byte(`{"file_count":1}`))
		case "/api/settings/check":
			_, _ = w.Write([]byte(`{"setup_needed":false}`))
		case "/v1/config/reload":
			_, _ = w.Write([]byte(`{"success":true,"reloaded":["smart"]}`))
		case "/api/settings/detect-dirs":
			_, _ = w.Write([]byte(`{"dirs":{"documents":"C:/Users/A/Documents"}}`))
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	})

	ctx := context.Background()
	schema, _ := Settings.Schema(ctx)
	config, _ := Settings.Config(ctx)
	updated, _ := Settings.UpdateConfig(ctx, map[string]string{"LLM_MODEL": "deepseek"})
	check, _ := Settings.Check(ctx)
	reloaded, _ := Settings.Reload(ctx)
	dirs, _ := Settings.DetectDirs(ctx)
	if schema["groups"] == nil || config["values"] == nil || updated["success"] != true || check["setup_needed"] != false || reloaded["success"] != true || dirs["dirs"] == nil || NewAgentKit().Settings != Settings {
		t.Fatalf("unexpected settings results")
	}
	if len(seen) != 6 {
		t.Fatalf("expected 6 settings API calls, got %d: %v", len(seen), seen)
	}
}

func TestSystemHelpers(t *testing.T) {
	var seen []string
	withTestAPI(t, func(w http.ResponseWriter, r *http.Request) {
		seen = append(seen, r.Method+" "+r.URL.String())
		if r.URL.Path == "/v1/metrics/prometheus" {
			w.Header().Set("Content-Type", "text/plain")
			_, _ = w.Write([]byte("yunque_requests_total 12\n"))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/healthz", "/livez", "/readyz", "/healthz/cognitive", "/v1/version", "/v1/system/info", "/v1/system/stats", "/v1/metrics", "/v1/cache/stats", "/v1/modules", "/sbom":
			_, _ = w.Write([]byte(fmt.Sprintf(`{"status":"ok","path":%q,"modules":[]}`, r.URL.Path)))
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	})

	ctx := context.Background()
	health, err := System.Health(ctx)
	if err != nil {
		t.Fatal(err)
	}
	livez, _ := System.Livez(ctx)
	readyz, _ := System.Readyz(ctx)
	cognitive, _ := System.CognitiveHealth(ctx)
	version, _ := System.Version(ctx)
	info, _ := System.Info(ctx)
	stats, _ := System.Stats(ctx)
	metrics, _ := System.Metrics(ctx)
	prometheus, _ := System.MetricsPrometheus(ctx)
	cache, _ := System.CacheStats(ctx)
	modules, _ := System.Modules(ctx)
	sbom, _ := System.SBOM(ctx)

	if health["path"] != "/healthz" || livez["path"] != "/livez" || readyz["path"] != "/readyz" || cognitive["path"] != "/healthz/cognitive" || version["path"] != "/v1/version" || info["path"] != "/v1/system/info" || stats["path"] != "/v1/system/stats" || metrics["path"] != "/v1/metrics" || cache["path"] != "/v1/cache/stats" || modules["path"] != "/v1/modules" || sbom["path"] != "/sbom" || !strings.Contains(prometheus, "yunque_requests_total") {
		t.Fatalf("unexpected system responses")
	}
	if NewAgentKit().System != System {
		t.Fatalf("agent kit should reuse System namespace")
	}
	if len(seen) != 12 {
		t.Fatalf("expected 12 system API calls, got %d: %v", len(seen), seen)
	}
}

func TestAuthHelpers(t *testing.T) {
	var seen []string
	withTestAPI(t, func(w http.ResponseWriter, r *http.Request) {
		seen = append(seen, r.Method+" "+r.URL.String())
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/v1/auth/status":
			if r.Method != http.MethodGet {
				t.Fatalf("method = %s", r.Method)
			}
			_, _ = w.Write([]byte(`{"password_set":true,"authenticated":true}`))
		case "/v1/auth/login":
			var body AuthLoginRequest
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			if body.Password != "secret" || !body.Remember {
				t.Fatalf("unexpected login body: %+v", body)
			}
			_, _ = w.Write([]byte(`{"token":"jwt-admin","expires_in":604800}`))
		case "/v1/auth/set-password":
			var body AuthSetPasswordRequest
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			if body.Password != "new" || body.Current != "old" {
				t.Fatalf("unexpected password body: %+v", body)
			}
			_, _ = w.Write([]byte(`{"status":"ok"}`))
		case "/v1/token":
			var body GenerateTokenRequest
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			if body.Role != "viewer" {
				t.Fatalf("unexpected token body: %+v", body)
			}
			_, _ = w.Write([]byte(`{"token":"jwt-viewer","type":"Bearer"}`))
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	})

	status, err := Auth.Status(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	login, err := Auth.Login(context.Background(), AuthLoginRequest{Password: "secret", Remember: true})
	if err != nil {
		t.Fatal(err)
	}
	changed, err := Auth.SetPassword(context.Background(), AuthSetPasswordRequest{Password: "new", Current: "old"})
	if err != nil {
		t.Fatal(err)
	}
	token, err := Auth.GenerateToken(context.Background(), GenerateTokenRequest{Role: "viewer"})
	if err != nil {
		t.Fatal(err)
	}
	if status["authenticated"] != true || login["token"] != "jwt-admin" || changed["status"] != "ok" || token["token"] != "jwt-viewer" {
		t.Fatalf("unexpected auth responses: %+v %+v %+v %+v", status, login, changed, token)
	}
	if Auth.ToriOAuthURL("") != strings.TrimRight(apiBase, "/")+"/v1/auth/oauth/tori" {
		t.Fatalf("unexpected tori URL: %s", Auth.ToriOAuthURL(""))
	}
	if !strings.Contains(Auth.ToriOAuthURL("https://tori.example"), "tori_url=https%3A%2F%2Ftori.example") {
		t.Fatalf("unexpected encoded tori URL: %s", Auth.ToriOAuthURL("https://tori.example"))
	}
	if NewAgentKit().Auth != Auth {
		t.Fatalf("agent kit should reuse Auth namespace")
	}
	if len(seen) != 4 {
		t.Fatalf("expected 4 auth API calls, got %d: %v", len(seen), seen)
	}
}

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

func TestEventsHelpers(t *testing.T) {
	messages := Events.Parse("event: connected\nid: evt-1\ndata: {\"client_id\":\"sse-1\"}\n\n: ignored\n\ndata: plain\nretry: 1500\n\n")
	if Events.StreamURL() != strings.TrimRight(apiBase, "/")+"/v1/events/stream" {
		t.Fatalf("unexpected stream URL: %s", Events.StreamURL())
	}
	if len(messages) != 2 || messages[0].Event != "connected" || messages[0].ID != "evt-1" {
		t.Fatalf("unexpected messages: %+v", messages)
	}
	payload, ok := messages[0].Data.(map[string]any)
	if !ok || payload["client_id"] != "sse-1" || messages[1].Data != "plain" || messages[1].Retry != 1500 {
		t.Fatalf("unexpected event payloads: %+v", messages)
	}
	if NewAgentKit().Events != Events {
		t.Fatalf("agent kit should reuse Events namespace")
	}
}

func TestTasksHelpers(t *testing.T) {
	var seen []string
	withTestAPI(t, func(w http.ResponseWriter, r *http.Request) {
		seen = append(seen, r.Method+" "+r.URL.String())
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/v1/tasks":
			switch r.Method {
			case http.MethodGet:
				if r.URL.Query().Get("id") == "task-1" {
					_, _ = w.Write([]byte(`{"id":"task-1","status":"running"}`))
					return
				}
				_, _ = w.Write([]byte(`[{"id":"task-1"}]`))
			case http.MethodPost:
				var body CreateTaskRequest
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Fatal(err)
				}
				if body.Description != "ship SDK" || body.Constraints.MaxSteps != 3 {
					t.Fatalf("unexpected task body: %+v", body)
				}
				_, _ = w.Write([]byte(`{"id":"task-2","description":"ship SDK"}`))
			case http.MethodDelete:
				_, _ = w.Write([]byte(`{"deleted":"task-1"}`))
			}
		case "/v1/tasks/run":
			_, _ = w.Write([]byte(`{"status":"accepted","task_id":"task-1"}`))
		case "/v1/tasks/templates":
			switch r.Method {
			case http.MethodGet:
				if r.URL.Query().Get("id") == "tpl-1" {
					_, _ = w.Write([]byte(`{"id":"tpl-1","name":"Review"}`))
					return
				}
				_, _ = w.Write([]byte(`{"templates":[{"id":"tpl-1"}],"total":1}`))
			case http.MethodPost:
				var body CreateTaskTemplateRequest
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Fatal(err)
				}
				if body.ID != "tpl-1" || body.Steps[0].Action != "review" {
					t.Fatalf("unexpected template body: %+v", body)
				}
				_, _ = w.Write([]byte(`{"id":"tpl-1","name":"Review"}`))
			case http.MethodDelete:
				_, _ = w.Write([]byte(`{"deleted":"tpl-1"}`))
			}
		case "/v1/tasks/templates/instantiate":
			var body InstantiateTaskTemplateRequest
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			if body.TemplateID != "tpl-1" || body.Variables["repo"] != "yunque" {
				t.Fatalf("unexpected instantiate body: %+v", body)
			}
			_, _ = w.Write([]byte(`{"id":"task-3","title":"Review"}`))
		case "/v1/tasks/gaps":
			if r.URL.Query().Get("stats") == "true" {
				_, _ = w.Write([]byte(`{"total":2,"unresolved":1}`))
				return
			}
			if r.URL.Query().Get("type") != "skill_missing" {
				t.Fatalf("unexpected gap query: %s", r.URL.RawQuery)
			}
			_, _ = w.Write([]byte(`[{"id":"gap-1","gap_type":"skill_missing"}]`))
		case "/v1/tasks/gaps/resolve":
			var body map[string]string
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			if body["id"] != "gap-1" {
				t.Fatalf("unexpected gap resolve body: %+v", body)
			}
			_, _ = w.Write([]byte(`{"resolved":"gap-1"}`))
		case "/v1/tasks/memory":
			if r.URL.Query().Get("id") != "task-1" {
				t.Fatalf("unexpected memory query: %s", r.URL.RawQuery)
			}
			_, _ = w.Write([]byte(`{"task_id":"task-1","goal":"ship SDK","next_action":"resume"}`))
		case "/v1/tasks/threads":
			switch r.Method {
			case http.MethodGet:
				if r.URL.Query().Get("id") == "task-1" {
					_, _ = w.Write([]byte(`{"task_id":"task-1","info":{"state":"open"},"messages":[{"role":"user","content":"hi"}]}`))
					return
				}
				if r.URL.Query().Get("state") != "open" {
					t.Fatalf("unexpected threads query: %s", r.URL.RawQuery)
				}
				_, _ = w.Write([]byte(`{"threads":[{"task_id":"task-1","state":"open"}],"total":1}`))
			case http.MethodPost:
				var body PostTaskThreadMessageRequest
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Fatal(err)
				}
				if body.TaskID != "task-1" || body.Content != "hi" || body.Channel.ChannelID != "chat-1" {
					t.Fatalf("unexpected thread post body: %+v", body)
				}
				_, _ = w.Write([]byte(`{"status":"posted","task_id":"task-1"}`))
			case http.MethodPut:
				var body UpdateTaskThreadStateRequest
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Fatal(err)
				}
				if body.TaskID != "task-1" || body.State != "paused" {
					t.Fatalf("unexpected thread state body: %+v", body)
				}
				_, _ = w.Write([]byte(`{"status":"updated","task_id":"task-1","state":"paused"}`))
			}
		case "/v1/trace/task/task-1":
			if r.URL.Query().Get("raw") != "true" {
				t.Fatalf("unexpected trace query: %s", r.URL.RawQuery)
			}
			_, _ = w.Write([]byte(`{"task_id":"task-1","count":1,"raw":true,"events":[{"id":"evt-1"}]}`))
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	})

	ctx := context.Background()
	list, err := Tasks.List(ctx)
	if err != nil || len(list) != 1 {
		t.Fatalf("list = %+v, %v", list, err)
	}
	got, err := Tasks.Get(ctx, "task-1")
	if err != nil || got["status"] != "running" {
		t.Fatalf("get = %+v, %v", got, err)
	}
	created, err := Tasks.Create(ctx, CreateTaskRequest{Description: "ship SDK", Constraints: TaskConstraints{MaxSteps: 3}})
	if err != nil || created["id"] != "task-2" {
		t.Fatalf("create = %+v, %v", created, err)
	}
	run, err := Tasks.Run(ctx, "task-1")
	if err != nil || run["status"] != "accepted" {
		t.Fatalf("run = %+v, %v", run, err)
	}
	deleted, err := Tasks.Delete(ctx, "task-1")
	if err != nil || deleted["deleted"] != "task-1" {
		t.Fatalf("delete = %+v, %v", deleted, err)
	}
	templates, err := Tasks.Templates(ctx)
	if err != nil || templates["total"].(float64) != 1 {
		t.Fatalf("templates = %+v, %v", templates, err)
	}
	tpl, err := Tasks.Template(ctx, "tpl-1")
	if err != nil || tpl["name"] != "Review" {
		t.Fatalf("template = %+v, %v", tpl, err)
	}
	createdTpl, err := Tasks.CreateTemplate(ctx, CreateTaskTemplateRequest{ID: "tpl-1", Steps: []TaskTemplateStep{{Action: "review"}}})
	if err != nil || createdTpl["id"] != "tpl-1" {
		t.Fatalf("create template = %+v, %v", createdTpl, err)
	}
	instantiated, err := Tasks.InstantiateTemplate(ctx, "tpl-1", map[string]string{"repo": "yunque"})
	if err != nil || instantiated["id"] != "task-3" {
		t.Fatalf("instantiate = %+v, %v", instantiated, err)
	}
	deletedTpl, err := Tasks.DeleteTemplate(ctx, "tpl-1")
	if err != nil || deletedTpl["deleted"] != "tpl-1" {
		t.Fatalf("delete template = %+v, %v", deletedTpl, err)
	}
	gaps, err := Tasks.Gaps(ctx, "skill_missing")
	if err != nil || gaps[0]["id"] != "gap-1" {
		t.Fatalf("gaps = %+v, %v", gaps, err)
	}
	stats, err := Tasks.GapStats(ctx)
	if err != nil || stats["unresolved"].(float64) != 1 {
		t.Fatalf("gap stats = %+v, %v", stats, err)
	}
	resolved, err := Tasks.ResolveGap(ctx, "gap-1")
	if err != nil || resolved["resolved"] != "gap-1" {
		t.Fatalf("resolve gap = %+v, %v", resolved, err)
	}
	memory, err := Tasks.WorkingMemory(ctx, "task-1")
	if err != nil || memory["next_action"] != "resume" {
		t.Fatalf("working memory = %+v, %v", memory, err)
	}
	threads, err := Tasks.Threads(ctx, "open")
	if err != nil || threads["total"].(float64) != 1 {
		t.Fatalf("threads = %+v, %v", threads, err)
	}
	thread, err := Tasks.Thread(ctx, "task-1")
	if err != nil || thread["task_id"] != "task-1" {
		t.Fatalf("thread = %+v, %v", thread, err)
	}
	posted, err := Tasks.PostThreadMessage(ctx, PostTaskThreadMessageRequest{TaskID: "task-1", Content: "hi", Channel: &TaskChannelBinding{ChannelType: "feishu", ChannelID: "chat-1"}})
	if err != nil || posted["status"] != "posted" {
		t.Fatalf("post thread = %+v, %v", posted, err)
	}
	updated, err := Tasks.UpdateThreadState(ctx, UpdateTaskThreadStateRequest{TaskID: "task-1", State: "paused"})
	if err != nil || updated["state"] != "paused" {
		t.Fatalf("update thread = %+v, %v", updated, err)
	}
	trace, err := Tasks.Trace(ctx, "task-1", true)
	if err != nil || trace.TaskID != "task-1" || !trace.Raw || len(trace.Events) != 1 {
		t.Fatalf("task trace = %+v, %v", trace, err)
	}
	if NewAgentKit().Tasks != Tasks {
		t.Fatalf("agent kit should reuse Tasks namespace")
	}
	if len(seen) != 19 {
		t.Fatalf("unexpected calls: %v", seen)
	}
}

func TestInteractionsNamespaceBundlesRuntimeInteractions(t *testing.T) {
	var seen []string
	withTestAPI(t, func(w http.ResponseWriter, r *http.Request) {
		seen = append(seen, r.Method+" "+r.URL.String())
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/v1/emotion/history":
			_, _ = w.Write([]byte(`{"entries":[{"emotion":"happy"}],"total":1}`))
		case "/v1/emotion/stickers":
			_, _ = w.Write([]byte(`{"status":"ok"}`))
		case "/v1/instructions":
			if r.Method == http.MethodGet {
				_, _ = w.Write([]byte(`{"instructions":[{"instruction_id":"i1"}],"total":1}`))
			} else {
				_, _ = w.Write([]byte(`{"instruction_id":"i1","content":"keep concise"}`))
			}
		case "/v1/react", "/v1/sticker/send":
			_, _ = w.Write([]byte(`{"status":"ok"}`))
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
		}
	})

	ctx := context.Background()
	history, _ := Interactions.EmotionHistory(ctx, EmotionHistoryOptions{SessionID: "s1", Limit: 1})
	stickers, _ := Interactions.Stickers(ctx)
	listed, _ := Interactions.Instructions(ctx, "style")
	created, _ := Interactions.CreateInstruction(ctx, UserInstruction{"content": "keep concise"})
	reacted, _ := Interactions.React(ctx, ReactRequest{ChannelType: "telegram", Target: "chat1", MessageID: "m1", Emoji: "👍"})
	sent, _ := Interactions.SendSticker(ctx, SendStickerRequest{ChannelType: "telegram", Target: "chat1", PackageID: "pkg", StickerID: "stk"})
	if history["total"].(float64) != 1 || stickers["status"] != "ok" || listed["total"].(float64) != 1 || created["instruction_id"] != "i1" || reacted["status"] != "ok" || sent["status"] != "ok" {
		t.Fatalf("unexpected interactions results")
	}
	if NewAgentKit().Interactions != Interactions {
		t.Fatalf("agent kit should expose Interactions namespace")
	}
	if len(seen) != 6 {
		t.Fatalf("expected 6 interaction requests, got %d: %v", len(seen), seen)
	}
}

func TestPermissionsHelpers(t *testing.T) {
	var seen []string
	withTestAPI(t, func(w http.ResponseWriter, r *http.Request) {
		seen = append(seen, r.Method+" "+r.URL.String())
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/v1/rbac/check":
			var body RBACCheckRequest
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			if body.Resource != "knowledge" || body.Action != "read" {
				t.Fatalf("unexpected permission body: %+v", body)
			}
			_, _ = w.Write([]byte(`{"allowed":true,"subject_id":"u1"}`))
		case "/v1/rbac/my-roles":
			_, _ = w.Write([]byte(`{"subject_id":"u1","roles":[{"id":"viewer"}],"total":1}`))
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
		}
	})

	ctx := context.Background()
	checked, err := Permissions.Check(ctx, RBACCheckRequest{SubjectID: "u1", Resource: "knowledge", Action: "read"})
	if err != nil {
		t.Fatal(err)
	}
	roles, err := Permissions.MyRoles(ctx)
	if err != nil {
		t.Fatal(err)
	}

	if checked["allowed"] != true || roles["total"].(float64) != 1 || NewAgentKit().Permissions != Permissions {
		t.Fatalf("unexpected permissions results")
	}
	if len(seen) != 2 || seen[0] != "POST /v1/rbac/check" || seen[1] != "GET /v1/rbac/my-roles" {
		t.Fatalf("unexpected permissions requests: %v", seen)
	}
}

func TestReactionsHelpers(t *testing.T) {
	var seen []string
	withTestAPI(t, func(w http.ResponseWriter, r *http.Request) {
		seen = append(seen, r.Method+" "+r.URL.String())
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/v1/react":
			var body ReactRequest
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			if body.MessageID != "m1" {
				t.Fatalf("unexpected react body: %+v", body)
			}
			_, _ = w.Write([]byte(`{"status":"ok"}`))
		case "/v1/sticker/send":
			_, _ = w.Write([]byte(`{"status":"sent"}`))
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
		}
	})

	ctx := context.Background()
	reacted, err := Reactions.React(ctx, ReactRequest{ChannelType: "wechat", Target: "u1", MessageID: "m1", Emoji: "👍"})
	if err != nil {
		t.Fatal(err)
	}
	sent, err := Reactions.SendSticker(ctx, SendStickerRequest{ChannelType: "wechat", Target: "u1", Emoji: "🌟"})
	if err != nil {
		t.Fatal(err)
	}

	if reacted["status"] != "ok" || sent["status"] != "sent" || NewAgentKit().Reactions != Reactions {
		t.Fatalf("unexpected reactions results")
	}
	if len(seen) != 2 || seen[0] != "POST /v1/react" || seen[1] != "POST /v1/sticker/send" {
		t.Fatalf("unexpected reactions requests: %v", seen)
	}
}

func TestInstructionsHelpers(t *testing.T) {
	var seen []string
	withTestAPI(t, func(w http.ResponseWriter, r *http.Request) {
		seen = append(seen, r.Method+" "+r.URL.String())
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/v1/instructions":
			if r.Method == http.MethodGet {
				if r.URL.Query().Get("category") != "style" {
					t.Fatalf("unexpected instructions query: %s", r.URL.RawQuery)
				}
				_, _ = w.Write([]byte(`{"instructions":[{"instruction_id":"ins-1","content":"保持简洁"}],"total":1}`))
				return
			}
			if r.Method == http.MethodPost {
				_, _ = w.Write([]byte(`{"instruction_id":"ins-1","content":"保持简洁"}`))
				return
			}
			_, _ = w.Write([]byte(`{"status":"updated"}`))
		case "/v1/instructions/reorder":
			_, _ = w.Write([]byte(`{"status":"reordered"}`))
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
		}
	})

	ctx := context.Background()
	list, err := Instructions.List(ctx, "style")
	if err != nil {
		t.Fatal(err)
	}
	created, err := Instructions.Create(ctx, UserInstruction{"category": "style", "content": "保持简洁"})
	if err != nil {
		t.Fatal(err)
	}
	updated, err := Instructions.Update(ctx, UserInstruction{"instruction_id": "ins-1", "content": "更新"})
	if err != nil {
		t.Fatal(err)
	}
	deleted, err := Instructions.Delete(ctx, "ins-1")
	if err != nil {
		t.Fatal(err)
	}
	reordered, err := Instructions.Reorder(ctx, []string{"ins-2", "ins-1"})
	if err != nil {
		t.Fatal(err)
	}

	if list["total"].(float64) != 1 || created["instruction_id"] != "ins-1" || updated["status"] != "updated" || deleted["status"] != "updated" || reordered["status"] != "reordered" || NewAgentKit().Instructions != Instructions {
		t.Fatalf("unexpected instructions results")
	}
	if len(seen) != 5 || seen[0] != "GET /v1/instructions?category=style" || seen[4] != "POST /v1/instructions/reorder" {
		t.Fatalf("unexpected instructions requests: %v", seen)
	}
}

func TestEmotionHelpers(t *testing.T) {
	var seen []string
	withTestAPI(t, func(w http.ResponseWriter, r *http.Request) {
		seen = append(seen, r.Method+" "+r.URL.String())
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/v1/emotion/history":
			if r.URL.Query().Get("session_id") != "s1" || r.URL.Query().Get("limit") != "5" {
				t.Fatalf("unexpected emotion query: %s", r.URL.RawQuery)
			}
			_, _ = w.Write([]byte(`{"entries":[{"emotion":"happy"}],"total":1}`))
		case "/v1/emotion/stickers":
			if r.Method == http.MethodGet {
				_, _ = w.Write([]byte(`{"happy":{"wechat":[{"package_id":"p1","sticker_id":"s1"}]}}`))
				return
			}
			_, _ = w.Write([]byte(`{"status":"ok"}`))
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
		}
	})

	ctx := context.Background()
	history, err := Emotion.History(ctx, EmotionHistoryOptions{SessionID: "s1", Limit: 5})
	if err != nil {
		t.Fatal(err)
	}
	stickers, err := Emotion.Stickers(ctx)
	if err != nil {
		t.Fatal(err)
	}
	registered, err := Emotion.RegisterStickers(ctx, RegisterStickersRequest{Platform: "wechat", Emotion: "happy", Stickers: []StickerSuggestion{{PackageID: "p1", StickerID: "s1"}}})
	if err != nil {
		t.Fatal(err)
	}
	cleared, err := Emotion.ClearStickers(ctx, ClearStickersRequest{Platform: "wechat", Emotion: "happy"})
	if err != nil {
		t.Fatal(err)
	}

	if history["total"].(float64) != 1 || stickers["happy"] == nil || registered["status"] != "ok" || cleared["status"] != "ok" || NewAgentKit().Emotion != Emotion {
		t.Fatalf("unexpected emotion results")
	}
	if len(seen) != 4 || seen[0] != "GET /v1/emotion/history?limit=5&session_id=s1" || seen[1] != "GET /v1/emotion/stickers" || seen[2] != "PUT /v1/emotion/stickers" || seen[3] != "DELETE /v1/emotion/stickers" {
		t.Fatalf("unexpected emotion requests: %v", seen)
	}
}

func TestPersonaModesHelpers(t *testing.T) {
	var seen []string
	withTestAPI(t, func(w http.ResponseWriter, r *http.Request) {
		seen = append(seen, r.Method+" "+r.URL.String())
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/v1/persona/modes":
			if r.URL.Query().Get("tenant_id") != "tenant-1" || r.URL.Query().Get("session_id") != "session-1" {
				t.Fatalf("unexpected persona modes query: %s", r.URL.RawQuery)
			}
			_, _ = w.Write([]byte(`{"modes":[{"mode":"study"}],"total":1}`))
		case "/v1/persona/mode":
			var body SetPersonaModeRequest
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			if body.TenantID != "tenant-1" || body.Mode != "focus" || body.SessionID != "session-1" {
				t.Fatalf("unexpected persona mode body: %+v", body)
			}
			_, _ = w.Write([]byte(`{"success":true,"current_mode":"focus"}`))
		case "/v1/persona/mode/current":
			if r.URL.Query().Get("tenant_id") != "tenant-1" {
				t.Fatalf("unexpected current mode query: %s", r.URL.RawQuery)
			}
			_, _ = w.Write([]byte(`{"mode":"study","name":"Study","description":"Study mode"}`))
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
		}
	})

	ctx := context.Background()
	modes, err := Persona.Modes(ctx, "tenant-1", "session-1")
	if err != nil {
		t.Fatal(err)
	}
	set, err := Persona.SetMode(ctx, SetPersonaModeRequest{TenantID: "tenant-1", Mode: "focus", SessionID: "session-1"})
	if err != nil {
		t.Fatal(err)
	}
	current, err := Persona.CurrentMode(ctx, "tenant-1", "")
	if err != nil {
		t.Fatal(err)
	}
	if modes["total"].(float64) != 1 || set["current_mode"] != "focus" || current["mode"] != "study" || NewAgentKit().Persona != Persona {
		t.Fatalf("unexpected persona mode results")
	}
	if len(seen) != 3 || seen[0] != "GET /v1/persona/modes?session_id=session-1&tenant_id=tenant-1" && seen[0] != "GET /v1/persona/modes?tenant_id=tenant-1&session_id=session-1" || seen[1] != "POST /v1/persona/mode" || seen[2] != "GET /v1/persona/mode/current?tenant_id=tenant-1" {
		t.Fatalf("unexpected persona mode requests: %v", seen)
	}
}

func TestModesNamespaceWrapsPersonaModes(t *testing.T) {
	var seen []string
	withTestAPI(t, func(w http.ResponseWriter, r *http.Request) {
		seen = append(seen, r.Method+" "+r.URL.String())
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/v1/persona/modes":
			_, _ = w.Write([]byte(`{"modes":[{"mode":"coder","active":true}],"total":1}`))
		case "/v1/persona/mode/current":
			_, _ = w.Write([]byte(`{"mode":"coder","name":"Coder"}`))
		case "/v1/persona/mode":
			_, _ = w.Write([]byte(`{"success":true,"current_mode":"operator"}`))
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
		}
	})

	ctx := context.Background()
	listed, err := Modes.List(ctx, "tenant-1", "session-1")
	if err != nil {
		t.Fatal(err)
	}
	current, err := Modes.Current(ctx, "tenant-1", "session-1")
	if err != nil {
		t.Fatal(err)
	}
	set, err := Modes.Set(ctx, SetPersonaModeRequest{TenantID: "tenant-1", SessionID: "session-1", Mode: "operator"})
	if err != nil {
		t.Fatal(err)
	}
	if listed["modes"] == nil || current["mode"] != "coder" || set["success"] != true {
		t.Fatalf("unexpected modes results")
	}
	if NewAgentKit().Modes != Modes {
		t.Fatalf("agent kit should expose Modes namespace")
	}
	if len(seen) != 3 || seen[0] != "GET /v1/persona/modes?session_id=session-1&tenant_id=tenant-1" || seen[1] != "GET /v1/persona/mode/current?session_id=session-1&tenant_id=tenant-1" || seen[2] != "POST /v1/persona/mode" {
		t.Fatalf("unexpected modes requests: %v", seen)
	}
}

func TestPersonaHelpers(t *testing.T) {
	var seen []string
	withTestAPI(t, func(w http.ResponseWriter, r *http.Request) {
		seen = append(seen, r.Method+" "+r.URL.String())
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/v1/persona":
			if r.Method == http.MethodGet {
				_, _ = w.Write([]byte(`{"identity":"Tori","soul":"careful","skills":[{"name":"review"}]}`))
				return
			}
			_, _ = w.Write([]byte(`{"status":"ok"}`))
		case "/v1/persona/skills":
			if r.Method == http.MethodGet {
				_, _ = w.Write([]byte(`{"skills":[{"name":"review"}]}`))
				return
			}
			_, _ = w.Write([]byte(`{"status":"ok"}`))
		case "/v1/persona/presets":
			if r.Method == http.MethodGet {
				_, _ = w.Write([]byte(`{"presets":[{"id":"default","name":"Default"}],"active":"default"}`))
				return
			}
			_, _ = w.Write([]byte(`{"status":"ok","active":"studio"}`))
		case "/v1/persona/presets/custom", "/v1/persona/presets/features":
			_, _ = w.Write([]byte(`{"status":"ok","id":"studio"}`))
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
		}
	})

	ctx := context.Background()
	state, err := Persona.Get(ctx)
	if err != nil {
		t.Fatal(err)
	}
	updated, err := Persona.Update(ctx, UpdatePersonaRequest{Identity: "Tori", Soul: "careful"})
	if err != nil {
		t.Fatal(err)
	}
	skills, err := Persona.Skills(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Persona.AddSkill(ctx, AddPersonaSkillRequest{Name: "review", Description: "Review", Content: "review code"}); err != nil {
		t.Fatal(err)
	}
	if _, err := Persona.DeleteSkill(ctx, "review"); err != nil {
		t.Fatal(err)
	}
	presets, err := Persona.Presets(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Persona.SwitchPreset(ctx, "studio"); err != nil {
		t.Fatal(err)
	}
	if _, err := Persona.AddCustomPreset(ctx, AddCustomPersonaPresetRequest{"id": "studio", "name": "Studio"}); err != nil {
		t.Fatal(err)
	}
	if _, err := Persona.UpdatePresetFeatures(ctx, UpdatePersonaPresetFeaturesRequest{ID: "studio", Features: map[string]bool{"emotion": true}}); err != nil {
		t.Fatal(err)
	}
	if _, err := Persona.DeleteCustomPreset(ctx, "studio"); err != nil {
		t.Fatal(err)
	}
	if state["identity"] != "Tori" || updated["status"] != "ok" || skills["skills"] == nil || presets["active"] != "default" || NewAgentKit().Persona != Persona {
		t.Fatalf("unexpected persona results: state=%+v updated=%+v skills=%+v presets=%+v", state, updated, skills, presets)
	}
	if len(seen) != 10 || seen[0] != "GET /v1/persona" || seen[9] != "DELETE /v1/persona/presets/custom" {
		t.Fatalf("unexpected persona requests: %v", seen)
	}
}

func TestIterateHelpers(t *testing.T) {
	var seen []string
	withTestAPI(t, func(w http.ResponseWriter, r *http.Request) {
		seen = append(seen, r.Method+" "+r.URL.String())
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/iterate/proposals":
			_, _ = w.Write([]byte(`{"proposals":[{"id":"it-1","status":"pending"}],"count":1}`))
		case "/api/iterate/approve", "/api/iterate/reject":
			var body IterateDecisionRequest
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			status := "approved"
			if r.URL.Path == "/api/iterate/reject" {
				status = "rejected"
			}
			_, _ = w.Write([]byte(`{"status":"` + status + `","id":"` + body.ID + `"}`))
		case "/api/iterate/trigger":
			_, _ = w.Write([]byte(`{"status":"ok","cycle":{"id":"cycle-1"}}`))
		case "/api/iterate/status":
			_, _ = w.Write([]byte(`{"enabled":true,"pending_proposals":1}`))
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
		}
	})

	ctx := context.Background()
	pending, err := Iterate.PendingProposals(ctx)
	if err != nil {
		t.Fatal(err)
	}
	approved, err := Iterate.Approve(ctx, "it-1")
	if err != nil {
		t.Fatal(err)
	}
	rejected, err := Iterate.Reject(ctx, "it-2")
	if err != nil {
		t.Fatal(err)
	}
	trigger, err := Iterate.Trigger(ctx)
	if err != nil {
		t.Fatal(err)
	}
	status, err := Iterate.Status(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if pending["count"].(float64) != 1 || approved["status"] != "approved" || rejected["id"] != "it-2" || trigger["status"] != "ok" || status["enabled"] != true || NewAgentKit().Iterate != Iterate {
		t.Fatalf("unexpected iterate results: pending=%+v approved=%+v rejected=%+v trigger=%+v status=%+v", pending, approved, rejected, trigger, status)
	}
	if len(seen) != 5 || seen[0] != "GET /api/iterate/proposals?status=pending" || seen[4] != "GET /api/iterate/status" {
		t.Fatalf("unexpected iterate requests: %v", seen)
	}
}

func TestTrustHelpers(t *testing.T) {
	var seen []string
	withTestAPI(t, func(w http.ResponseWriter, r *http.Request) {
		seen = append(seen, r.Method+" "+r.URL.String())
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/trust/scores":
			_, _ = w.Write([]byte(`{"scores":{"shell":{"score":80,"level":"review"}},"count":1}`))
		case "/api/trust/reset", "/api/trust/grant":
			var body TrustSlugRequest
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			_, _ = w.Write([]byte(`{"status":"ok","slug":"` + body.Slug + `"}`))
		case "/api/review/status":
			_, _ = w.Write([]byte(`{"enabled":true,"trust_enabled":true,"distill_enabled":false}`))
		case "/api/skillgrow/patterns":
			_, _ = w.Write([]byte(`{"patterns":[{"pattern":"repeat-review","count":4}],"count":1}`))
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
		}
	})

	ctx := context.Background()
	scores, err := Trust.Scores(ctx)
	if err != nil {
		t.Fatal(err)
	}
	reset, err := Trust.Reset(ctx, "shell")
	if err != nil {
		t.Fatal(err)
	}
	grantAll, err := Trust.GrantAll(ctx)
	if err != nil {
		t.Fatal(err)
	}
	status, err := Trust.ReviewStatus(ctx)
	if err != nil {
		t.Fatal(err)
	}
	patterns, err := Trust.SkillGrowPatterns(ctx)
	if err != nil {
		t.Fatal(err)
	}
	skillGrowPatterns, err := SkillGrow.Patterns(ctx)
	if err != nil {
		t.Fatal(err)
	}
	reviewStatus, err := Review.Status(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if scores["count"].(float64) != 1 || reset["slug"] != "shell" || grantAll["slug"] != "*" || status["enabled"] != true || patterns["count"].(float64) != 1 || skillGrowPatterns["count"].(float64) != 1 || reviewStatus["enabled"] != true || NewAgentKit().Trust != Trust || NewAgentKit().SkillGrow != SkillGrow || NewAgentKit().Review != Review {
		t.Fatalf("unexpected trust results: scores=%+v reset=%+v grantAll=%+v status=%+v patterns=%+v", scores, reset, grantAll, status, patterns)
	}
	if len(seen) != 7 || seen[0] != "GET /api/trust/scores" || seen[4] != "GET /api/skillgrow/patterns" || seen[5] != "GET /api/skillgrow/patterns" || seen[6] != "GET /api/review/status" {
		t.Fatalf("unexpected trust requests: %v", seen)
	}
}

func TestAuditHelpers(t *testing.T) {
	var seen []string
	withTestAPI(t, func(w http.ResponseWriter, r *http.Request) {
		seen = append(seen, r.Method+" "+r.URL.String())
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/v1/audit/tail":
			_, _ = w.Write([]byte(`{"records":[{"id":"r1","type":"system"}],"count":1}`))
		case "/v1/audit/verify":
			_, _ = w.Write([]byte(`{"valid":true,"checked":1}`))
		case "/api/trust/scores":
			_, _ = w.Write([]byte(`{"scores":{"shell":{"score":80}},"count":1}`))
		case "/v1/audit/stats":
			_, _ = w.Write([]byte(`{"total":12}`))
		case "/api/audit/trail":
			_, _ = w.Write([]byte(`{"entries":[{"operation":"nl_config","result":"ok"}],"count":1}`))
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
		}
	})

	ctx := context.Background()
	tail, err := Audit.Tail(ctx, AuditTailOptions{N: 10, Type: "system event", Actor: "tenant"})
	if err != nil {
		t.Fatal(err)
	}
	verify, err := Audit.Verify(ctx)
	if err != nil {
		t.Fatal(err)
	}
	stats, err := Audit.Stats(ctx)
	if err != nil {
		t.Fatal(err)
	}
	trail, err := Audit.Trail(ctx, AuditTrailOptions{Date: "2026-05-11", Type: "nl_config"})
	if err != nil {
		t.Fatal(err)
	}
	if tail.Count != 1 || verify["valid"] != true || stats["total"].(float64) != 12 || trail.Entries[0]["operation"] != "nl_config" || NewAgentKit().Audit != Audit {
		t.Fatalf("unexpected audit results: tail=%+v verify=%+v stats=%+v trail=%+v", tail, verify, stats, trail)
	}
	if len(seen) != 4 || seen[0] != "GET /v1/audit/tail?actor=tenant&n=10&type=system+event" || seen[3] != "GET /api/audit/trail?date=2026-05-11&type=nl_config" {
		t.Fatalf("unexpected audit requests: %v", seen)
	}
}

func TestToolsHelpers(t *testing.T) {
	var seen []string
	withTestAPI(t, func(w http.ResponseWriter, r *http.Request) {
		seen = append(seen, r.Method+" "+r.URL.String())
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/v1/tools/exec":
			var body ToolExecOptions
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			if body.Command != "echo ok" || body.Cwd != "work" || body.Env[0] != "A=B" {
				t.Fatalf("unexpected exec body: %+v", body)
			}
			_, _ = w.Write([]byte(`{"output":"ok","exit_code":0,"state":"exited"}`))
		case "/v1/tools/list":
			_, _ = w.Write([]byte(`{"sessions":[{"id":"tool-1","command":"npm test","state":"running"}]}`))
		case "/v1/audit/verify":
			_, _ = w.Write([]byte(`{"valid":true,"checked":1}`))
		case "/api/trust/scores":
			_, _ = w.Write([]byte(`{"scores":{"shell":{"score":80}},"count":1}`))
		case "/v1/tools/poll":
			_, _ = w.Write([]byte(`{"lines":["hello"],"state":"running"}`))
		case "/v1/tools/kill":
			_, _ = w.Write([]byte(`{"killed":"tool-1"}`))
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
		}
	})

	ctx := context.Background()
	execResult, err := Tools.Exec(ctx, ToolExecOptions{Command: "echo ok", Cwd: "work", TimeoutMs: 1000, Env: []string{"A=B"}})
	if err != nil {
		t.Fatal(err)
	}
	list, err := Tools.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	poll, err := Tools.Poll(ctx, "tool-1")
	if err != nil {
		t.Fatal(err)
	}
	killed, err := Tools.Kill(ctx, "tool-1")
	if err != nil {
		t.Fatal(err)
	}
	if execResult["output"] != "ok" || list.Sessions[0].ID != "tool-1" || poll.Lines[0] != "hello" || killed["killed"] != "tool-1" || NewAgentKit().Tools != Tools {
		t.Fatalf("unexpected tools results: exec=%+v list=%+v poll=%+v killed=%+v", execResult, list, poll, killed)
	}
	if len(seen) != 4 || seen[0] != "POST /v1/tools/exec" || seen[2] != "GET /v1/tools/poll?id=tool-1" || seen[3] != "POST /v1/tools/kill?id=tool-1" {
		t.Fatalf("unexpected tools requests: %v", seen)
	}
}

func TestSubagentsHelpers(t *testing.T) {
	var seen []string
	withTestAPI(t, func(w http.ResponseWriter, r *http.Request) {
		seen = append(seen, r.Method+" "+r.URL.String())
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/v1/subagent":
			switch r.Method {
			case http.MethodGet:
				if r.URL.Query().Get("id") == "sa/1" {
					_, _ = w.Write([]byte(`{"id":"sa/1","name":"reviewer"}`))
					return
				}
				_, _ = w.Write([]byte(`{"subagents":[{"id":"sa/1","name":"reviewer","parent_id":"task/1"}]}`))
			case http.MethodPost:
				var body map[string]any
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Fatal(err)
				}
				if body["name"] != "planner" || body["parent_id"] != "task/1" {
					t.Fatalf("unexpected spawn body: %+v", body)
				}
				_, _ = w.Write([]byte(`{"id":"sa/2","name":"planner","skills":["plan"]}`))
			case http.MethodDelete:
				_, _ = w.Write([]byte(`{"deleted":true}`))
			default:
				t.Fatalf("unexpected subagent method: %s", r.Method)
			}
		case "/v1/subagent/message":
			var body map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			if body["id"] != "sa/1" {
				t.Fatalf("unexpected append body: %+v", body)
			}
			_, _ = w.Write([]byte(`{"ok":true}`))
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
		}
	})

	ctx := context.Background()
	listed, err := Subagents.List(ctx, "task/1")
	if err != nil {
		t.Fatal(err)
	}
	one, err := Subagents.Get(ctx, "sa/1")
	if err != nil {
		t.Fatal(err)
	}
	spawned, err := Subagents.Spawn(ctx, SpawnSubagentRequest{ParentID: "task/1", Name: "planner", Description: "计划拆解", Skills: []string{"plan"}})
	if err != nil {
		t.Fatal(err)
	}
	appended, err := Subagents.AppendMessages(ctx, "sa/1", []SubagentMessage{{"role": "user", "content": "继续"}})
	if err != nil {
		t.Fatal(err)
	}
	deleted, err := Subagents.Destroy(ctx, "sa/1")
	if err != nil {
		t.Fatal(err)
	}
	if listed.Subagents[0].ID != "sa/1" || one.Name != "reviewer" || spawned.Skills[0] != "plan" || appended["ok"] != true || deleted["deleted"] != true || NewAgentKit().Subagents != Subagents {
		t.Fatalf("unexpected subagent results: listed=%+v one=%+v spawned=%+v appended=%+v deleted=%+v", listed, one, spawned, appended, deleted)
	}
	if len(seen) != 5 || seen[0] != "GET /v1/subagent?parent_id=task%2F1" || seen[4] != "DELETE /v1/subagent?id=sa%2F1" {
		t.Fatalf("unexpected subagent requests: %v", seen)
	}
}

func TestRuntimeHelpers(t *testing.T) {
	var seen []string
	withTestAPI(t, func(w http.ResponseWriter, r *http.Request) {
		seen = append(seen, r.Method+" "+r.URL.String())
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/v1/sessions/queue":
			if r.Method != http.MethodGet {
				t.Fatalf("unexpected queue method: %s", r.Method)
			}
			if r.URL.Query().Get("id") == "session 1" {
				_, _ = w.Write([]byte(`{"session_id":"session 1","tasks":[{"id":"task/1"}]}`))
				return
			}
			_, _ = w.Write([]byte(`{"queues":{"session 1":1}}`))
		case "/v1/sessions/queue/cancel":
			if r.Method != http.MethodPost {
				t.Fatalf("unexpected cancel method: %s", r.Method)
			}
			var body map[string]string
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			if body["session_id"] != "session 1" || body["task_id"] != "task/1" {
				t.Fatalf("unexpected cancel body: %+v", body)
			}
			_, _ = w.Write([]byte(`{"status":"cancelled"}`))
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
		}
	})

	ctx := context.Background()
	overview, err := Runtime.Queues(ctx)
	if err != nil {
		t.Fatal(err)
	}
	session, err := Runtime.SessionQueue(ctx, "session 1")
	if err != nil {
		t.Fatal(err)
	}
	cancelled, err := Runtime.CancelQueuedTask(ctx, "session 1", "task/1")
	if err != nil {
		t.Fatal(err)
	}
	queueOverview, err := RuntimeQueue.Overview(ctx)
	if err != nil {
		t.Fatal(err)
	}
	queueSession, err := RuntimeQueue.Session(ctx, "session 1")
	if err != nil {
		t.Fatal(err)
	}
	queueCancelled, err := RuntimeQueue.Cancel(ctx, "session 1", "task/1")
	if err != nil {
		t.Fatal(err)
	}
	if overview["queues"] == nil || session["session_id"] != "session 1" || cancelled["status"] != "cancelled" || queueOverview["queues"] == nil || queueSession["session_id"] != "session 1" || queueCancelled["status"] != "cancelled" || Runtime.EventsStreamURL() != strings.TrimRight(apiBase, "/")+"/v1/events/stream" || NewAgentKit().Runtime != Runtime || NewAgentKit().RuntimeQueue != RuntimeQueue {
		t.Fatalf("unexpected runtime results: overview=%+v session=%+v cancelled=%+v", overview, session, cancelled)
	}
	if len(seen) != 6 || seen[0] != "GET /v1/sessions/queue" || seen[1] != "GET /v1/sessions/queue?id=session+1" || seen[2] != "POST /v1/sessions/queue/cancel" || seen[3] != "GET /v1/sessions/queue" || seen[4] != "GET /v1/sessions/queue?id=session+1" || seen[5] != "POST /v1/sessions/queue/cancel" {
		t.Fatalf("unexpected runtime requests: %v", seen)
	}
}

func TestBrowserHelpers(t *testing.T) {
	var seen []string
	withTestAPI(t, func(w http.ResponseWriter, r *http.Request) {
		seen = append(seen, r.Method+" "+r.URL.String())
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/v1/browser/status":
			fmt.Fprint(w, `{"connected":true}`)
		case "/v1/browser/config":
			fmt.Fprint(w, `{"mode":"extension"}`)
		case "/v1/browser/navigate":
			fmt.Fprint(w, `{"title":"Example"}`)
		case "/v1/browser/screenshot", "/v1/browser/screenshot/latest":
			fmt.Fprint(w, `{"screenshot":"abc"}`)
		case "/v1/browser/ocr":
			fmt.Fprint(w, `{"text":"page"}`)
		case "/v1/browser/opp/pending":
			fmt.Fprint(w, `{"items":[],"total":0}`)
		case "/v1/browser/opp/decide":
			fmt.Fprint(w, `{"status":"resolved"}`)
		case "/api/browser/ext/status":
			fmt.Fprint(w, `{"connected":true}`)
		case "/api/browser/ext/session":
			fmt.Fprint(w, `{"ok":true,"ticket":"t1"}`)
		case "/api/browser/ext/action":
			fmt.Fprint(w, `{"ok":true}`)
		case "/api/browser/ext/scenarios":
			fmt.Fprint(w, `{"scenarios":[{"id":"open-page"}]}`)
		case "/api/browser/ext/scenarios/run":
			fmt.Fprint(w, `{"ok":true,"scenario":"open-page"}`)
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
		}
	})
	ctx := context.Background()
	status, _ := Browser.Status(ctx)
	_, _ = Browser.Config(ctx)
	_, _ = Browser.Navigate(ctx, "https://example.test")
	_, _ = Browser.Screenshot(ctx)
	_, _ = Browser.LatestScreenshot(ctx)
	_, _ = Browser.OCR(ctx)
	_, _ = Browser.OPPPending(ctx)
	_, _ = Browser.OPPDecide(ctx, map[string]any{"problem_id": "opp1", "decision": "allow_once"})
	_, _ = Browser.ExtensionStatus(ctx)
	session, _ := Browser.ExtensionSession(ctx)
	_, _ = Browser.ExtensionAction(ctx, BrowserAction{"type": "browser_screenshot"})
	_, _ = Browser.Scenarios(ctx)
	run, _ := Browser.RunScenario(ctx, "open-page")
	if !status["connected"].(bool) || session["ticket"] != "t1" || run["scenario"] != "open-page" || NewAgentKit().Browser != Browser {
		t.Fatalf("unexpected browser results")
	}
	if len(seen) != 13 || seen[0] != "GET /v1/browser/status" || seen[12] != "POST /api/browser/ext/scenarios/run" {
		t.Fatalf("unexpected browser requests: %v", seen)
	}
}

func TestFilesHelpers(t *testing.T) {
	var seen []string
	withTestAPI(t, func(w http.ResponseWriter, r *http.Request) {
		seen = append(seen, r.Method+" "+r.URL.String())
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/files":
			fmt.Fprint(w, `{"files":[{"name":"report.md","path":"artifacts/report.md","size":12,"is_dir":false}]}`)
		case "/api/files/preview":
			fmt.Fprint(w, `{"name":"report.md","path":"artifacts/report.md","preview":"hello"}`)
		case "/api/files/download":
			w.Header().Set("Content-Disposition", `attachment; filename="report.md"`)
			fmt.Fprint(w, `hello`)
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
		}
	})
	ctx := context.Background()
	list, _ := Files.List(ctx, "artifacts")
	preview, _ := Files.Preview(ctx, "artifacts/report.md")
	down, _ := Files.Download(ctx, "artifacts/report.md")
	if list.Files[0].Name != "report.md" || preview["preview"] != "hello" || string(down.Content) != "hello" || down.Filename != "report.md" || NewAgentKit().Files != Files {
		t.Fatalf("unexpected files results")
	}
	if len(seen) != 3 || seen[0] != "GET /api/files?path=artifacts" || seen[1] != "GET /api/files/preview?path=artifacts%2Freport.md" || seen[2] != "GET /api/files/download?path=artifacts%2Freport.md" {
		t.Fatalf("unexpected files requests: %v", seen)
	}
}

func TestRBACHelpers(t *testing.T) {
	var seen []string
	withTestAPI(t, func(w http.ResponseWriter, r *http.Request) {
		seen = append(seen, r.Method+" "+r.URL.String())
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/v1/rbac/roles":
			if r.Method == http.MethodGet {
				fmt.Fprint(w, `{"roles":[{"id":"operator","name":"Operator","permissions":[]}],"total":1}`)
			} else if r.Method == http.MethodPost {
				fmt.Fprint(w, `{"id":"operator","name":"Operator","permissions":[]}`)
			} else {
				fmt.Fprint(w, `{"deleted":"operator"}`)
			}
		case "/v1/rbac/assign":
			fmt.Fprint(w, `{"status":"assigned","subject_id":"u1","role_id":"operator"}`)
		case "/v1/rbac/revoke":
			fmt.Fprint(w, `{"status":"revoked","subject_id":"u1","role_id":"operator"}`)
		case "/v1/rbac/check":
			fmt.Fprint(w, `{"allowed":true,"subject_id":"u1","resource":"tasks","action":"write"}`)
		case "/v1/rbac/my-roles":
			fmt.Fprint(w, `{"subject_id":"u1","roles":[{"id":"operator"}],"total":1}`)
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
		}
	})
	ctx := context.Background()
	roles, _ := RBAC.Roles(ctx)
	created, _ := RBAC.CreateRole(ctx, RBACRole{"id": "operator", "name": "Operator", "permissions": []any{}})
	deleted, _ := RBAC.DeleteRole(ctx, "operator")
	assigned, _ := RBAC.AssignRole(ctx, RBACRoleBindingRequest{SubjectID: "u1", RoleID: "operator", TenantID: "t1"})
	revoked, _ := RBAC.RevokeRole(ctx, RBACRoleBindingRequest{SubjectID: "u1", RoleID: "operator"})
	checked, _ := RBAC.Check(ctx, RBACCheckRequest{SubjectID: "u1", Resource: "tasks", Action: "write"})
	mine, _ := RBAC.MyRoles(ctx)
	if roles["total"].(float64) != 1 || created["id"] != "operator" || deleted["deleted"] != "operator" || assigned["status"] != "assigned" || revoked["status"] != "revoked" || !checked["allowed"].(bool) || mine["total"].(float64) != 1 || NewAgentKit().RBAC != RBAC {
		t.Fatalf("unexpected RBAC results")
	}
	if len(seen) != 7 || seen[0] != "GET /v1/rbac/roles" || seen[2] != "DELETE /v1/rbac/roles?id=operator" || seen[6] != "GET /v1/rbac/my-roles" {
		t.Fatalf("unexpected RBAC requests: %v", seen)
	}
}

func TestApprovalsHelpers(t *testing.T) {
	var seen []string
	withTestAPI(t, func(w http.ResponseWriter, r *http.Request) {
		seen = append(seen, r.Method+" "+r.URL.String())
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/v1/approvals":
			fmt.Fprint(w, `{"approvals":[{"id":"ap1","status":"pending"}],"total":1}`)
		case "/v1/approvals/approve":
			fmt.Fprint(w, `{"status":"approved","id":"ap1"}`)
		case "/v1/approvals/deny":
			fmt.Fprint(w, `{"status":"denied","id":"ap2"}`)
		case "/v1/approvals/decide":
			fmt.Fprint(w, `{"status":"ok","decision":"allow_once"}`)
		case "/v1/approvals/rules":
			if r.Method == http.MethodGet {
				fmt.Fprint(w, `{"rules":[{"id":"r1"}],"total":1}`)
			} else if r.Method == http.MethodPost {
				fmt.Fprint(w, `{"status":"ok","id":"r1"}`)
			} else {
				fmt.Fprint(w, `{"deleted":true}`)
			}
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
		}
	})
	ctx := context.Background()
	pending, _ := Approvals.Pending(ctx)
	history, _ := Approvals.History(ctx, "approved")
	approved, _ := Approvals.Approve(ctx, "ap1")
	denied, _ := Approvals.Deny(ctx, "ap2", "too risky")
	decided, _ := Approvals.Decide(ctx, "ap3", "allow_once")
	rules, _ := Approvals.Rules(ctx)
	added, _ := Approvals.AddRule(ctx, ApprovalRule{"id": "r1", "decision": "allow_always"})
	deleted, _ := Approvals.DeleteRule(ctx, "r1")
	if pending["total"].(float64) != 1 || history["total"].(float64) != 1 || approved["status"] != "approved" || denied["status"] != "denied" || decided["decision"] != "allow_once" || rules["total"].(float64) != 1 || added["id"] != "r1" || !deleted["deleted"].(bool) || NewAgentKit().Approvals != Approvals {
		t.Fatalf("unexpected approval results")
	}
	if len(seen) != 8 || seen[0] != "GET /v1/approvals?status=pending" || seen[1] != "GET /v1/approvals?history=true&status=approved" || seen[7] != "DELETE /v1/approvals/rules?id=r1" {
		t.Fatalf("unexpected approval requests: %v", seen)
	}
}

func TestConversationsHelpers(t *testing.T) {
	var seen []string
	withTestAPI(t, func(w http.ResponseWriter, r *http.Request) {
		seen = append(seen, r.Method+" "+r.URL.String())
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/v1/conversations":
			if r.URL.Query().Get("archived") != "true" {
				t.Fatalf("unexpected conversations query: %s", r.URL.RawQuery)
			}
			_, _ = w.Write([]byte(`{"sessions":[{"id":"s1"}],"count":1}`))
		case "/v1/conversations/messages":
			if r.URL.Query().Get("session_id") != "s1" {
				t.Fatalf("unexpected messages query: %s", r.URL.RawQuery)
			}
			if r.Method == http.MethodDelete {
				_, _ = w.Write([]byte(`{"status":"deleted"}`))
			} else {
				_, _ = w.Write([]byte(`{"messages":[{"role":"user","content":"hi"}],"count":1}`))
			}
		case "/v1/conversations/manage":
			_, _ = w.Write([]byte(`{"status":"updated","session":{"id":"s1"}}`))
		case "/v1/conversations/replay":
			if r.URL.Query().Get("raw") != "true" || r.URL.Query().Get("limit") != "10" {
				t.Fatalf("unexpected replay query: %s", r.URL.RawQuery)
			}
			_, _ = w.Write([]byte(`{"session_id":"s1","turns":[],"total_turns":0}`))
		default:
			t.Fatalf("unexpected request: %s", r.URL.Path)
		}
	})

	if _, err := Conversations.List(context.Background(), true); err != nil {
		t.Fatal(err)
	}
	if _, err := Conversations.Messages(context.Background(), "s1"); err != nil {
		t.Fatal(err)
	}
	if _, err := Conversations.DeleteMessages(context.Background(), "s1"); err != nil {
		t.Fatal(err)
	}
	if _, err := Conversations.Rename(context.Background(), "s1", "新的会话"); err != nil {
		t.Fatal(err)
	}
	if _, err := Conversations.Pin(context.Background(), "s1", true); err != nil {
		t.Fatal(err)
	}
	if _, err := Conversations.Archive(context.Background(), "s1", false); err != nil {
		t.Fatal(err)
	}
	if _, err := Conversations.Replay(context.Background(), "s1", ConversationReplayOptions{Raw: true, Limit: 10, Offset: 2}); err != nil {
		t.Fatal(err)
	}
	if NewAgentKit().Conversations != Conversations {
		t.Fatalf("agent kit should reuse Conversations namespace")
	}
	if len(seen) != 7 {
		t.Fatalf("expected 7 requests, got %d: %v", len(seen), seen)
	}
}

func TestChatHelpers(t *testing.T) {
	var seen []string
	withTestAPI(t, func(w http.ResponseWriter, r *http.Request) {
		seen = append(seen, r.Method+" "+r.URL.String())
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/v1/chat":
			_, _ = w.Write([]byte(`{"reply":"basic"}`))
		case "/v1/chat/agentic":
			_, _ = w.Write([]byte(`{"reply":"agentic"}`))
		default:
			t.Fatalf("unexpected request: %s", r.URL.Path)
		}
	})

	request := ChatRequest{Messages: []ChatMessage{{Role: "user", Content: "hi"}}, SessionID: "s1"}
	basic, err := ChatSDK.Send(context.Background(), request)
	if err != nil || basic["reply"] != "basic" {
		t.Fatalf("unexpected basic chat: %+v err=%v", basic, err)
	}
	agentic, err := ChatSDK.Agentic(context.Background(), request)
	if err != nil || agentic["reply"] != "agentic" {
		t.Fatalf("unexpected agentic chat: %+v err=%v", agentic, err)
	}
	if ChatSDK.StreamURL() != strings.TrimRight(apiBase, "/")+"/v1/chat/stream" {
		t.Fatalf("unexpected stream URL: %s", ChatSDK.StreamURL())
	}
	streamReq := ChatSDK.StreamRequest(request)
	if !streamReq.Stream {
		t.Fatalf("stream request should set stream=true")
	}
	items := ChatSDK.ParseStream("event: message\ndata: {\"type\":\"delta\",\"content\":\"你\"}\n\nevent: error\ndata: {\"error\":\"bad\"}\n\n")
	if len(items) != 2 || items[0].Kind != "delta" || items[0].Content != "你" || items[1].Kind != "error" {
		t.Fatalf("unexpected stream items: %+v", items)
	}
	if NewAgentKit().Chat != ChatSDK {
		t.Fatalf("agent kit should reuse Chat namespace")
	}
	if len(seen) != 2 {
		t.Fatalf("expected 2 requests, got %d", len(seen))
	}
}

func TestRealtimeHelpers(t *testing.T) {
	oldBase, oldToken := apiBase, pluginToken
	apiBase, pluginToken = "https://agent.example/", "plugin-token"
	defer func() { apiBase, pluginToken = oldBase, oldToken }()

	url := Realtime.WSURL(map[string]string{"tenant": "t1", "empty": ""})
	if url != "wss://agent.example/v1/ws?access_token=plugin-token&tenant=t1" && url != "wss://agent.example/v1/ws?tenant=t1&access_token=plugin-token" {
		t.Fatalf("unexpected websocket URL: %s", url)
	}
	ping := Realtime.Ping(map[string]any{"trace_id": "tr-1"})
	if ping["type"] != "ping" || ping["trace_id"] != "tr-1" {
		t.Fatalf("unexpected ping: %+v", ping)
	}
	chat := Realtime.Chat("你好", "s1", map[string]any{"locale": "zh-CN"})
	encoded, err := Realtime.Serialize(chat)
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := Realtime.Parse(encoded)
	if err != nil {
		t.Fatal(err)
	}
	if parsed["type"] != "chat" || parsed["content"] != "你好" || parsed["session"] != "s1" {
		t.Fatalf("unexpected parsed chat: %+v", parsed)
	}
	if _, err := Realtime.Parse("[]"); err == nil {
		t.Fatalf("expected arrays to be rejected")
	}
	if NewAgentKit().Realtime != Realtime {
		t.Fatalf("agent kit should reuse Realtime namespace")
	}
}

func TestReverieHelpers(t *testing.T) {
	delivered := false
	var seen []string
	withTestAPI(t, func(w http.ResponseWriter, r *http.Request) {
		seen = append(seen, r.Method+" "+r.URL.String())
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/v1/reverie/journal":
			if r.URL.Query().Get("category") != "task" || r.URL.Query().Get("delivered") != "false" {
				t.Fatalf("unexpected journal query: %s", r.URL.RawQuery)
			}
			_, _ = w.Write([]byte(`{"thoughts":[{"id":"t1"}],"total":1,"limit":10,"offset":0}`))
		case "/v1/reverie/stats":
			_, _ = w.Write([]byte(`{"total":2}`))
		case "/v1/reverie/config":
			if r.Method == http.MethodPut {
				_, _ = w.Write([]byte(`{"config":{"enabled":false}}`))
			} else {
				_, _ = w.Write([]byte(`{"config":{"enabled":true}}`))
			}
		case "/v1/reverie/think":
			_, _ = w.Write([]byte(`{"thought":{"id":"t2"}}`))
		case "/v1/reverie/thought":
			_, _ = w.Write([]byte(`{"deleted":true,"id":"t1"}`))
		case "/v1/reverie/actions":
			_, _ = w.Write([]byte(`{"actions":[{"id":"a1"}],"total":1}`))
		case "/v1/reverie/targets":
			_, _ = w.Write([]byte(`{"targets":[{"channel":"feishu"}],"count":1}`))
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
		}
	})

	journal, err := Reverie.Journal(context.Background(), ReverieJournalOptions{Category: "task", Delivered: &delivered, Limit: 10})
	if err != nil || journal.Total != 1 || journal.Thoughts[0]["id"] != "t1" {
		t.Fatalf("unexpected journal: %+v err=%v", journal, err)
	}
	stats, err := Reverie.Stats(context.Background())
	if err != nil || stats["total"].(float64) != 2 {
		t.Fatalf("unexpected stats: %+v err=%v", stats, err)
	}
	if _, err := Reverie.Config(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := Reverie.UpdateConfig(context.Background(), ReverieConfig{"enabled": false}); err != nil {
		t.Fatal(err)
	}
	if _, err := Reverie.Think(context.Background(), ReverieThinkRequest{EventType: "task_completed", Trigger: "sdk"}); err != nil {
		t.Fatal(err)
	}
	if _, err := Reverie.DeleteThought(context.Background(), "t1"); err != nil {
		t.Fatal(err)
	}
	if _, err := Reverie.Actions(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := Reverie.Targets(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(seen) != 8 {
		t.Fatalf("expected 8 requests, got %d: %v", len(seen), seen)
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
		case r.Method == http.MethodDelete && r.URL.Path == "/v1/state/goals":
			if r.URL.Query().Get("id") != "g1" {
				t.Fatalf("unexpected delete goal query: %s", r.URL.RawQuery)
			}
			_, _ = w.Write([]byte(`{"id":"g1","status":"deleted"}`))
		case r.Method == http.MethodGet && r.URL.Path == "/v1/state/focus":
			_, _ = w.Write([]byte(`{"focus":"SDK boundary"}`))
		case r.Method == http.MethodPost && r.URL.Path == "/v1/state/focus":
			var body StateFocusUpdateRequest
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			if body.Focus != "Next SDK" || len(body.Topics) != 1 || body.Topics[0] != "state" {
				t.Fatalf("unexpected focus body: %+v", body)
			}
			_, _ = w.Write([]byte(`{"status":"updated"}`))
		case r.Method == http.MethodGet && r.URL.Path == "/v1/state/resources":
			_, _ = w.Write([]byte(`[{"id":"r1","type":"file","path":"sdk/go/yunque/yunque.go","status":"active"}]`))
		case r.Method == http.MethodPost && r.URL.Path == "/v1/state/resources":
			var body StateResource
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			if body.Path != "sdk/go/yunque/yunque.go" || body.Type != "file" {
				t.Fatalf("unexpected resource body: %+v", body)
			}
			_, _ = w.Write([]byte(`{"status":"tracked"}`))
		case r.Method == http.MethodDelete && r.URL.Path == "/v1/state/resources":
			if r.URL.Query().Get("id") != "r1" {
				t.Fatalf("unexpected release resource query: %s", r.URL.RawQuery)
			}
			_, _ = w.Write([]byte(`{"status":"released"}`))
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

	deleted, err := State.DeleteGoal(context.Background(), "g1")
	if err != nil {
		t.Fatal(err)
	}
	if deleted.ID != "g1" || deleted.Status != "deleted" {
		t.Fatalf("unexpected delete response: %+v", deleted)
	}

	focus, err := State.Focus(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if focus != "SDK boundary" {
		t.Fatalf("unexpected focus: %q", focus)
	}

	updatedFocus, err := State.UpdateFocus(context.Background(), "Next SDK", []string{"state"})
	if err != nil {
		t.Fatal(err)
	}
	if updatedFocus.Status != "updated" {
		t.Fatalf("unexpected focus update response: %+v", updatedFocus)
	}

	resources, err := State.Resources(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(resources) != 1 || resources[0].Path != "sdk/go/yunque/yunque.go" {
		t.Fatalf("unexpected resources: %+v", resources)
	}

	tracked, err := State.TrackResource(context.Background(), StateResource{Path: "sdk/go/yunque/yunque.go", Type: "file"})
	if err != nil {
		t.Fatal(err)
	}
	if tracked.Status != "tracked" {
		t.Fatalf("unexpected track response: %+v", tracked)
	}

	released, err := State.ReleaseResource(context.Background(), "r1")
	if err != nil {
		t.Fatal(err)
	}
	if released.Status != "released" {
		t.Fatalf("unexpected release response: %+v", released)
	}

	if len(seen) != 8 {
		t.Fatalf("expected 8 requests, got %d: %v", len(seen), seen)
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
		case "/v1/sessions/queue":
			_, _ = w.Write([]byte(`{"queues":{"s1":1}}`))
		case "/v1/subagent":
			_, _ = w.Write([]byte(`{"subagents":[{"id":"sa-1","name":"reviewer"}]}`))
		case "/v1/tools/list":
			_, _ = w.Write([]byte(`{"sessions":[{"id":"tool-1","command":"npm test","state":"running"}]}`))
		case "/v1/audit/verify":
			_, _ = w.Write([]byte(`{"valid":true,"checked":1}`))
		case "/api/trust/scores":
			_, _ = w.Write([]byte(`{"scores":{"shell":{"score":80}},"count":1}`))
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
		case "/api/notify/channels":
			_, _ = w.Write([]byte(`{"channels":[{"id":"feishu-main","type":"feishu","name":"Feishu","enabled":true}]}`))
		case "/v1/orchestrator/status":
			_, _ = w.Write([]byte(`{"running":true,"adapters":["cursor"],"active_sessions":1}`))
		case "/v1/fork/list":
			_, _ = w.Write([]byte(`{"forks":[{"id":"fork_1","session_id":"s1","messages":[],"created_at":"2026-05-12T00:00:00Z"}]}`))
		case "/v1/cost/summary":
			_, _ = w.Write([]byte(`{"today_cost":0.12,"month_cost":1.5}`))
		case "/api/providers":
			_, _ = w.Write([]byte(`{"providers":[{"id":"deepseek","model":"deepseek-chat"}],"mode":"hybrid"}`))
		case "/v1/cognis":
			_, _ = w.Write([]byte(`{"cognis":[{"id":"reviewer","name":"Code Reviewer"}],"count":1}`))
		case "/v1/trace/recent":
			_, _ = w.Write([]byte(`{"events":[{"trace_id":"tr-1"}],"count":1}`))
		case "/v1/heartbeat":
			_, _ = w.Write([]byte(`{"running":true}`))
		case "/v1/reverie/stats":
			_, _ = w.Write([]byte(`{"total":2}`))
		case "/v1/approvals":
			_, _ = w.Write([]byte(`{"approvals":[{"id":"ap1","status":"pending"}],"total":1}`))
		case "/v1/rbac/my-roles":
			_, _ = w.Write([]byte(`{"subject_id":"u1","roles":[{"id":"operator","name":"Operator","permissions":[]}],"total":1}`))
		case "/api/files":
			_, _ = w.Write([]byte(`{"files":[{"name":"report.md","path":"artifacts/report.md","size":12,"is_dir":false}]}`))
		case "/v1/browser/status":
			_, _ = w.Write([]byte(`{"connected":true,"state":"extension"}`))
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
	runtimeQueues, err := kit.Runtime.Queues(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	subagentsList, err := kit.Subagents.List(context.Background(), "task-1")
	if err != nil {
		t.Fatal(err)
	}
	toolList, err := kit.Tools.List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	auditVerify, err := kit.Audit.Verify(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	trustScores, err := kit.Trust.Scores(context.Background())
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
	notifyChannels, err := kit.Notify.Channels(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	orchStatus, err := kit.Orchestrator.Status(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	forkList, err := kit.Fork.List(context.Background(), "s1")
	if err != nil {
		t.Fatal(err)
	}
	costSummary, err := kit.Cost.Summary(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	providerList, err := kit.Providers.List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	cogniList, err := kit.Cognis.List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	traceRecent, err := kit.Trace.Recent(context.Background(), TraceRecentOptions{Limit: 1})
	if err != nil {
		t.Fatal(err)
	}
	heartbeatStatus, err := kit.Heartbeat.Status(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	reverieStats, err := kit.Reverie.Stats(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	approvalList, err := kit.Approvals.Pending(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	rbacRoles, err := kit.RBAC.MyRoles(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	fileList, err := kit.Files.List(context.Background(), "artifacts")
	if err != nil {
		t.Fatal(err)
	}
	browserStatus, err := kit.Browser.Status(context.Background())
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

	if focus != "sdk" || runtimeQueues["queues"] == nil || subagentsList.Subagents[0].ID != "sa-1" || toolList.Sessions[0].ID != "tool-1" || auditVerify["valid"] != true || trustScores["count"].(float64) != 1 || !strings.Contains(strategies, "SDK slices") || mission.Type != "cron" || jobs.Count != 1 || len(cronJobs.Jobs) != 1 || triggerDefs.Total != 1 || memoryResults.Count != 1 || graphStats.Entities != 2 || kbStats["sources"].(float64) != 2 || loraStatus["active_model"] != "adapter-a" || workflowList.Total != 1 || len(connectorList.Connectors) != 1 || connectorList.Connectors[0].ID != "github" || len(notifyChannels.Channels) != 1 || notifyChannels.Channels[0].ID != "feishu-main" || !orchStatus.Running || len(forkList.Forks) != 1 || costSummary["today_cost"].(float64) != 0.12 || providerList.Providers[0]["id"] != "deepseek" || cogniList["count"].(float64) != 1 || traceRecent.Events[0]["trace_id"] != "tr-1" || !heartbeatStatus["running"].(bool) || reverieStats["total"].(float64) != 2 || approvalList["total"].(float64) != 1 || rbacRoles["total"].(float64) != 1 || fileList.Files[0].Name != "report.md" || !browserStatus["connected"].(bool) || len(results) != 1 || results[0].Title != "Agent Kit" {
		t.Fatalf("unexpected kit results: focus=%q strategies=%q mission=%+v jobs=%+v results=%+v", focus, strategies, mission, jobs, results)
	}
	if kit.State != State || kit.Reflect != Reflect || kit.Missions != Missions || kit.Scheduler != Scheduler || kit.CronSystem != CronSystem || kit.Triggers != Triggers || kit.MemoryCore != MemoryCore || kit.Graph != Graph || kit.KnowledgeKB != KnowledgeKB || kit.LoRA != LoRA || kit.Workflows != Workflows || kit.Connectors != Connectors || kit.Notify != Notify || kit.Orchestrator != Orchestrator || kit.Fork != Fork || kit.Cost != Cost || kit.Providers != Providers || kit.Cognis != Cognis || kit.Trace != Trace || kit.Heartbeat != Heartbeat || kit.Events != Events || kit.Runtime != Runtime || kit.Subagents != Subagents || kit.Tools != Tools || kit.Audit != Audit || kit.Trust != Trust || kit.Reverie != Reverie || kit.Approvals != Approvals || kit.RBAC != RBAC || kit.Files != Files || kit.Browser != Browser || kit.Plugin != Plugin || kit.Memory != Memory || kit.AgentMemory != AgentMemory || kit.Knowledge != Knowledge || kit.Cron != Cron {
		t.Fatalf("agent kit should reuse lightweight singleton namespaces")
	}
	if len(seen) != 32 {
		t.Fatalf("expected 32 requests, got %d: %v", len(seen), seen)
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

func TestDispatchNamespaceManagesWorkersQueueAndConfig(t *testing.T) {
	var seen []string
	withTestAPI(t, func(w http.ResponseWriter, r *http.Request) {
		seen = append(seen, r.Method+" "+r.URL.String())
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/v1/workers":
			_, _ = w.Write([]byte(`{"workers":[{"id":"w1","type":"cursor","capabilities":["coding"]}],"count":1}`))
		case "/v1/workers/detail":
			if r.URL.Query().Get("id") != "w1" {
				t.Fatalf("unexpected worker detail query: %s", r.URL.RawQuery)
			}
			_, _ = w.Write([]byte(`{"id":"w1","type":"cursor","capabilities":["coding"]}`))
		case "/v1/workers/remove":
			var body map[string]string
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			if body["id"] != "w1" {
				t.Fatalf("unexpected remove body: %+v", body)
			}
			_, _ = w.Write([]byte(`{"status":"removed"}`))
		case "/v1/dispatch/queue":
			_, _ = w.Write([]byte(`{"message":"dispatch queue (use task system for now)"}`))
		case "/v1/dispatch/enqueue":
			var body DispatchEnqueueRequest
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			if body.TaskID != "t1" || body.Priority != 10 || body.Capabilities[0] != "coding" {
				t.Fatalf("unexpected enqueue body: %+v", body)
			}
			_, _ = w.Write([]byte(`{"task_id":"t1","status":"enqueued"}`))
		case "/v1/workers/config":
			if r.URL.Query().Get("type") != "cursor" {
				t.Fatalf("unexpected worker config query: %s", r.URL.RawQuery)
			}
			_, _ = w.Write([]byte(`{"type":"cursor","mcp_config":"{}","instructions":"Register worker","server_url":"http://localhost:9090/mcp/v1"}`))
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
		}
	})

	workers, err := Dispatch.Workers(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	worker, err := Dispatch.Worker(context.Background(), "w1")
	if err != nil {
		t.Fatal(err)
	}
	removed, err := Dispatch.RemoveWorker(context.Background(), "w1")
	if err != nil {
		t.Fatal(err)
	}
	queue, err := Dispatch.Queue(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	enqueued, err := Dispatch.Enqueue(context.Background(), DispatchEnqueueRequest{TaskID: "t1", Capabilities: []string{"coding"}, Priority: 10})
	if err != nil {
		t.Fatal(err)
	}
	config, err := Dispatch.WorkerConfig(context.Background(), "cursor")
	if err != nil {
		t.Fatal(err)
	}
	kit := NewAgentKit()

	if workers.Count != 1 || worker.Type != "cursor" || removed.Status != "removed" || queue["message"] == "" || enqueued.Status != "enqueued" || config.Type != "cursor" || kit.Dispatch != Dispatch {
		t.Fatalf("unexpected dispatch results")
	}
	if len(seen) != 6 {
		t.Fatalf("expected 6 requests, got %d: %v", len(seen), seen)
	}
}

func TestSkillMarketNamespaceSearchTopAndStats(t *testing.T) {
	var seen []string
	withTestAPI(t, func(w http.ResponseWriter, r *http.Request) {
		seen = append(seen, r.Method+" "+r.URL.String())
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/v1/market/search":
			if r.URL.Query().Get("q") == "docx" {
				_, _ = w.Write([]byte(`{"skills":[{"name":"doc_parse","version":"1.0.0","category":"data"}],"count":1}`))
				return
			}
			_, _ = w.Write([]byte(`{"skills":[{"name":"web_search","version":"1.0.0"}]}`))
		case "/v1/market/top":
			if r.URL.Query().Get("n") != "3" || r.URL.Query().Get("by") != "rating" {
				t.Fatalf("unexpected top query: %s", r.URL.RawQuery)
			}
			_, _ = w.Write([]byte(`{"skills":[{"name":"code_gen","version":"2.1.0","rating":4.8}]}`))
		case "/v1/market/stats":
			_, _ = w.Write([]byte(`{"total":3,"categories":{"coding":1}}`))
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
		}
	})

	found, err := SkillMarket.Search(context.Background(), "docx")
	if err != nil {
		t.Fatal(err)
	}
	all, err := SkillMarket.Search(context.Background(), "")
	if err != nil {
		t.Fatal(err)
	}
	top, err := SkillMarket.Top(context.Background(), SkillMarketTopOptions{N: 3, By: "rating"})
	if err != nil {
		t.Fatal(err)
	}
	stats, err := SkillMarket.Stats(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	kit := NewAgentKit()

	if found.Skills[0].Name != "doc_parse" || all.Skills[0].Name != "web_search" || top.Skills[0].Name != "code_gen" || stats["total"].(float64) != 3 || kit.Market != SkillMarket {
		t.Fatalf("unexpected market results")
	}
	if len(seen) != 4 {
		t.Fatalf("expected 4 requests, got %d: %v", len(seen), seen)
	}
}

func TestProjectsNamespaceManagesProjectWorkspaces(t *testing.T) {
	var seen []string
	withTestAPI(t, func(w http.ResponseWriter, r *http.Request) {
		seen = append(seen, r.Method+" "+r.URL.String())
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/v1/projects":
			switch r.Method {
			case http.MethodGet:
				_, _ = w.Write([]byte(`{"projects":[{"id":"p1","name":"云雀","repo_path":"C:/repo","default_caps":["read"]}]}`))
			case http.MethodPost:
				var body CreateProjectRequest
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Fatal(err)
				}
				if body.Name != "云雀" || body.RepoPath != "C:/repo" || body.DefaultCaps[0] != "read" {
					t.Fatalf("unexpected create body: %+v", body)
				}
				_, _ = w.Write([]byte(`{"id":"p1","name":"云雀","repo_path":"C:/repo"}`))
			default:
				t.Fatalf("unexpected method: %s", r.Method)
			}
		case "/v1/projects/detail":
			if r.URL.Query().Get("id") != "p1" {
				t.Fatalf("unexpected project detail query: %s", r.URL.RawQuery)
			}
			if r.Method == http.MethodPut {
				var body UpdateProjectRequest
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Fatal(err)
				}
				if body.Description != "Agent" {
					t.Fatalf("unexpected update body: %+v", body)
				}
			}
			_, _ = w.Write([]byte(`{"id":"p1","name":"云雀+","repo_path":"C:/repo","description":"Agent"}`))
		case "/v1/projects/remove":
			var body map[string]string
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			if body["id"] != "p1" {
				t.Fatalf("unexpected remove body: %+v", body)
			}
			_, _ = w.Write([]byte(`{"status":"deleted"}`))
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
		}
	})

	list, err := Projects.List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	created, err := Projects.Create(context.Background(), CreateProjectRequest{Name: "云雀", RepoPath: "C:/repo", DefaultCaps: []string{"read"}})
	if err != nil {
		t.Fatal(err)
	}
	detail, err := Projects.Detail(context.Background(), "p1")
	if err != nil {
		t.Fatal(err)
	}
	updated, err := Projects.Update(context.Background(), "p1", UpdateProjectRequest{Description: "Agent"})
	if err != nil {
		t.Fatal(err)
	}
	removed, err := Projects.Remove(context.Background(), "p1")
	if err != nil {
		t.Fatal(err)
	}
	kit := NewAgentKit()

	if list.Projects[0].ID != "p1" || created.ID != "p1" || detail.Name != "云雀+" || updated.Description != "Agent" || removed.Status != "deleted" || kit.Projects != Projects {
		t.Fatalf("unexpected projects results")
	}
	if len(seen) != 5 {
		t.Fatalf("expected 5 requests, got %d: %v", len(seen), seen)
	}
}

func TestNotifyNamespaceManagesChannelsAndShare(t *testing.T) {
	var seen []string
	withTestAPI(t, func(w http.ResponseWriter, r *http.Request) {
		seen = append(seen, r.Method+" "+r.URL.String())
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/notify/channels":
			_, _ = w.Write([]byte(`{"channels":[{"id":"feishu-main","type":"feishu","name":"Feishu","enabled":true}]}`))
		case "/api/notify/add":
			var body NotifyChannel
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			if body.ID != "feishu-main" || body.Type != "feishu" {
				t.Fatalf("unexpected add body: %+v", body)
			}
			_, _ = w.Write([]byte(`{"ok":true}`))
		case "/api/notify/remove":
			if r.URL.Query().Get("id") != "feishu-main" {
				t.Fatalf("unexpected remove query: %s", r.URL.RawQuery)
			}
			_, _ = w.Write([]byte(`{"ok":true}`))
		case "/api/notify/toggle":
			var body NotifyToggleRequest
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			if body.ID != "feishu-main" || body.Enabled {
				t.Fatalf("unexpected toggle body: %+v", body)
			}
			_, _ = w.Write([]byte(`{"ok":true}`))
		case "/api/notify/test":
			_, _ = w.Write([]byte(`{"ok":true}`))
		case "/api/notify/share":
			var body NotifyShareRequest
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			if body.ChannelID != "feishu-main" || body.Message != "done" {
				t.Fatalf("unexpected share body: %+v", body)
			}
			_, _ = w.Write([]byte(`{"ok":true,"sent_at":"2026-05-12T00:00:00Z","share":{"code":"yq_abc"},"channel":{"id":"feishu-main"}}`))
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
		}
	})

	channels, err := Notify.Channels(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	added, err := Notify.AddChannel(context.Background(), NotifyChannel{ID: "feishu-main", Type: "feishu", Name: "Feishu"})
	if err != nil {
		t.Fatal(err)
	}
	removed, err := Notify.RemoveChannel(context.Background(), "feishu-main")
	if err != nil {
		t.Fatal(err)
	}
	toggled, err := Notify.ToggleChannel(context.Background(), NotifyToggleRequest{ID: "feishu-main", Enabled: false})
	if err != nil {
		t.Fatal(err)
	}
	tested, err := Notify.TestChannel(context.Background(), "feishu-main")
	if err != nil {
		t.Fatal(err)
	}
	shared, err := Notify.Share(context.Background(), NotifyShareRequest{ChannelID: "feishu-main", Message: "done"})
	if err != nil {
		t.Fatal(err)
	}
	if channels.Channels[0].ID != "feishu-main" || !added.OK || !removed.OK || !toggled.OK || !tested.OK || shared.Share["code"] != "yq_abc" {
		t.Fatalf("unexpected notify results")
	}
	if len(seen) != 6 {
		t.Fatalf("expected 6 requests, got %d: %v", len(seen), seen)
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

func TestOrchestratorHelpers(t *testing.T) {
	var seen []string
	withTestAPI(t, func(w http.ResponseWriter, r *http.Request) {
		seen = append(seen, r.Method+" "+r.URL.String())
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/v1/orchestrator/status":
			_, _ = w.Write([]byte(`{"running":true,"adapters":["cursor"],"active_sessions":1,"event_count":2}`))
		case "/v1/orchestrator/toggle":
			var body map[string]string
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			if body["action"] != "start" {
				t.Fatalf("unexpected action: %+v", body)
			}
			_, _ = w.Write([]byte(`{"status":"started"}`))
		case "/v1/orchestrator/sessions":
			_, _ = w.Write([]byte(`{"sessions":[{"session_id":"s1","adapter":"cursor","task_id":"t1"}]}`))
		case "/v1/orchestrator/detect":
			_, _ = w.Write([]byte(`{"ides":[{"name":"Cursor","available":true}]}`))
		case "/v1/orchestrator/events":
			if r.URL.Query().Get("limit") != "2" {
				t.Fatalf("unexpected events query: %s", r.URL.RawQuery)
			}
			_, _ = w.Write([]byte(`{"events":[{"id":"e1","type":"task_assigned","task_id":"t1","message":"assigned"}],"total":1}`))
		case "/v1/orchestrator/events/task":
			if r.URL.Query().Get("task_id") != "t1" {
				t.Fatalf("unexpected timeline query: %s", r.URL.RawQuery)
			}
			_, _ = w.Write([]byte(`{"task_id":"t1","events":[{"id":"e1","type":"task_assigned"}]}`))
		case "/v1/orchestrator/policy":
			if r.Method == http.MethodPut {
				var body map[string]any
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Fatal(err)
				}
				_, _ = w.Write([]byte(`{"status":"updated","policy":{"allow_auto_launch":true}}`))
				return
			}
			_, _ = w.Write([]byte(`{"allow_auto_launch":false}`))
		case "/v1/orchestrator/adapters/add":
			var body OrchestratorAdapterConfig
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			if body.AdapterName != "custom" {
				t.Fatalf("unexpected adapter body: %+v", body)
			}
			_, _ = w.Write([]byte(`{"status":"registered","name":"custom","available":true}`))
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
		}
	})

	status, err := Orchestrator.Status(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	toggled, err := Orchestrator.Toggle(context.Background(), "start")
	if err != nil {
		t.Fatal(err)
	}
	sessions, err := Orchestrator.Sessions(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	ides, err := Orchestrator.DetectIDEs(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	events, err := Orchestrator.Events(context.Background(), 2)
	if err != nil {
		t.Fatal(err)
	}
	timeline, err := Orchestrator.TaskTimeline(context.Background(), "t1")
	if err != nil {
		t.Fatal(err)
	}
	policy, err := Orchestrator.Policy(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	updated, err := Orchestrator.UpdatePolicy(context.Background(), OrchestratorPolicy{"allow_auto_launch": true})
	if err != nil {
		t.Fatal(err)
	}
	adapter, err := Orchestrator.AddAdapter(context.Background(), OrchestratorAdapterConfig{AdapterName: "custom", Binary: "worker.exe", MCPConfigPath: "mcp.json"})
	if err != nil {
		t.Fatal(err)
	}

	if !status.Running || toggled.Status != "started" || sessions.Sessions[0].Adapter != "cursor" || ides.IDEs[0].Name != "Cursor" || events.Total != 1 || timeline.TaskID != "t1" || policy["allow_auto_launch"] != false || updated.Policy["allow_auto_launch"] != true || adapter.Name != "custom" {
		t.Fatalf("unexpected orchestrator results")
	}
	if len(seen) != 9 {
		t.Fatalf("expected 9 requests, got %d: %v", len(seen), seen)
	}
}

func TestForkHelpers(t *testing.T) {
	var seen []string
	withTestAPI(t, func(w http.ResponseWriter, r *http.Request) {
		seen = append(seen, r.Method+" "+r.URL.String())
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/v1/fork":
			switch r.Method {
			case http.MethodGet:
				if r.URL.Query().Get("session_id") == "s1" || r.URL.Query().Get("id") == "fork_1" {
					_, _ = w.Write([]byte(`{"id":"fork_1","session_id":"s1","messages":[],"created_at":"2026-05-12T00:00:00Z"}`))
					return
				}
			case http.MethodPost:
				var body ForkCreateRequest
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Fatal(err)
				}
				_, _ = w.Write([]byte(`{"id":"fork_1","session_id":"s1","messages":[{"role":"user","content":"hi"}],"created_at":"2026-05-12T00:00:00Z"}`))
				return
			case http.MethodDelete:
				_, _ = w.Write([]byte(`{"deleted":true}`))
				return
			}
		case "/v1/fork/branch":
			var body ForkBranchRequest
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			_, _ = w.Write([]byte(`{"id":"fork_2","parent_id":"fork_1","session_id":"s1","label":"alt","messages":[],"created_at":"2026-05-12T00:00:00Z"}`))
		case "/v1/fork/list":
			_, _ = w.Write([]byte(`{"forks":[{"id":"fork_1","session_id":"s1","messages":[],"created_at":"2026-05-12T00:00:00Z"}]}`))
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
		}
	})

	root, err := Fork.Root(context.Background(), "s1")
	if err != nil {
		t.Fatal(err)
	}
	got, err := Fork.Get(context.Background(), "fork_1")
	if err != nil {
		t.Fatal(err)
	}
	created, err := Fork.Create(context.Background(), ForkCreateRequest{SessionID: "s1", Messages: []ForkMessage{{Role: "user", Content: "hi"}}})
	if err != nil {
		t.Fatal(err)
	}
	removed, err := Fork.Remove(context.Background(), "fork_1")
	if err != nil {
		t.Fatal(err)
	}
	branched, err := Fork.Branch(context.Background(), ForkBranchRequest{ForkID: "fork_1", AtIndex: 0, Label: "alt"})
	if err != nil {
		t.Fatal(err)
	}
	list, err := Fork.List(context.Background(), "s1")
	if err != nil {
		t.Fatal(err)
	}

	if root["id"] != "fork_1" || got.ID != "fork_1" || len(created.Messages) != 1 || !removed.Deleted || branched.ParentID != "fork_1" || len(list.Forks) != 1 {
		t.Fatalf("unexpected fork results")
	}
	if len(seen) != 6 {
		t.Fatalf("expected 6 requests, got %d: %v", len(seen), seen)
	}
}

func TestCostHelpers(t *testing.T) {
	var seen []string
	withTestAPI(t, func(w http.ResponseWriter, r *http.Request) {
		seen = append(seen, r.Method+" "+r.URL.String())
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/v1/cost/summary":
			_, _ = w.Write([]byte(`{"today_cost":0.12,"month_cost":1.5}`))
		case "/v1/cost/budget":
			var body CostBudget
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			if body["daily_limit_usd"].(float64) != 1 {
				t.Fatalf("unexpected budget body: %+v", body)
			}
			_, _ = w.Write([]byte(`{"ok":true}`))
		case "/v1/cost/task":
			if r.URL.Query().Get("id") != "task/1" {
				t.Fatalf("unexpected task query: %s", r.URL.RawQuery)
			}
			_, _ = w.Write([]byte(`{"total_cost_usd":0.2}`))
		case "/v1/cost/task/timeline":
			_, _ = w.Write([]byte(`{"records":[]}`))
		case "/v1/cost/breakdown":
			_, _ = w.Write([]byte(`{"by_provider":[]}`))
		case "/v1/cost/history":
			if r.URL.Query().Get("page") != "2" || r.URL.Query().Get("model") != "gpt-test" {
				t.Fatalf("unexpected history query: %s", r.URL.RawQuery)
			}
			_, _ = w.Write([]byte(`{"records":[],"page":2}`))
		case "/v1/cost/alerts":
			_, _ = w.Write([]byte(`{"alerts":[]}`))
		case "/v1/usage":
			_, _ = w.Write([]byte(`{"tenant_id":"tenant-1"}`))
		case "/v1/quota":
			var body SetQuotaRequest
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			if body.TenantID != "tenant-1" || body.Quota["max_chat_calls"].(float64) != 10 {
				t.Fatalf("unexpected quota body: %+v", body)
			}
			_, _ = w.Write([]byte(`{"status":"ok"}`))
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
		}
	})

	summary, err := Cost.Summary(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	budget, err := Cost.SetBudget(context.Background(), CostBudget{"daily_limit_usd": 1})
	if err != nil {
		t.Fatal(err)
	}
	task, err := Cost.Task(context.Background(), "task/1")
	if err != nil {
		t.Fatal(err)
	}
	timeline, err := Cost.TaskTimeline(context.Background(), "task/1")
	if err != nil {
		t.Fatal(err)
	}
	breakdown, err := Cost.Breakdown(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	history, err := Cost.History(context.Background(), CostHistoryOptions{Page: 2, Limit: 25, Model: "gpt-test"})
	if err != nil {
		t.Fatal(err)
	}
	alerts, err := Cost.Alerts(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	usage, err := Cost.Usage(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	quota, err := Cost.SetQuota(context.Background(), SetQuotaRequest{TenantID: "tenant-1", Quota: map[string]any{"max_chat_calls": 10}})
	if err != nil {
		t.Fatal(err)
	}
	kit := NewAgentKit()

	if summary["today_cost"].(float64) != 0.12 || !budget["ok"].(bool) || task["total_cost_usd"].(float64) != 0.2 || timeline["records"] == nil || breakdown["by_provider"] == nil || history["page"].(float64) != 2 || alerts["alerts"] == nil || usage["tenant_id"] != "tenant-1" || quota["status"] != "ok" || kit.Cost != Cost {
		t.Fatalf("unexpected cost results")
	}
	if len(seen) != 9 {
		t.Fatalf("expected 9 requests, got %d: %v", len(seen), seen)
	}
}

func TestProvidersHelpers(t *testing.T) {
	var seen []string
	withTestAPI(t, func(w http.ResponseWriter, r *http.Request) {
		seen = append(seen, r.Method+" "+r.URL.String())
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/v1/models":
			if r.Method == http.MethodGet {
				_, _ = w.Write([]byte(`{"models":[{"id":"m1","model_id":"deepseek-chat"}]}`))
				return
			}
			if r.Method == http.MethodDelete {
				_, _ = w.Write([]byte(`{"status":"ok"}`))
				return
			}
			_, _ = w.Write([]byte(`{"id":"m1","model_id":"deepseek-chat"}`))
		case "/api/providers":
			_, _ = w.Write([]byte(`{"providers":[{"id":"deepseek","model":"deepseek-chat"}],"mode":"hybrid"}`))
		case "/api/providers/test", "/api/providers/enable", "/api/providers/disable", "/api/providers/switch-model", "/api/providers/session", "/api/providers/register", "/api/providers/delete", "/api/providers/local/discover", "/api/providers/local/register", "/api/providers/exec", "/api/breaker/reset":
			_, _ = w.Write([]byte(`{"ok":true,"provider_id":"deepseek","exec_provider":"deepseek","reset_count":1}`))
		case "/api/providers/mode":
			_, _ = w.Write([]byte(`{"ok":true,"mode":"hybrid"}`))
		case "/api/providers/presets":
			_, _ = w.Write([]byte(`{"presets":[{"id":"deepseek"}]}`))
		case "/api/providers/tori/discover":
			_, _ = w.Write([]byte(`{"ok":true,"registered":1}`))
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
		}
	})

	models, err := Providers.Models(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	added, err := Providers.AddModel(context.Background(), ModelEntry{"id": "m1", "model_id": "deepseek-chat"})
	if err != nil {
		t.Fatal(err)
	}
	deletedModel, err := Providers.DeleteModel(context.Background(), "m1")
	if err != nil {
		t.Fatal(err)
	}
	list, err := Providers.List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	tested, err := Providers.Test(context.Background(), "deepseek")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = Providers.Enable(context.Background(), "deepseek")
	_, _ = Providers.Disable(context.Background(), "deepseek")
	_, _ = Providers.SwitchModel(context.Background(), "deepseek", "deepseek-chat")
	_, _ = Providers.SetSession(context.Background(), ProviderSessionOverrideRequest{SessionID: "s1", ProviderID: "deepseek"})
	mode, err := Providers.Mode(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	_, _ = Providers.SetMode(context.Background(), "hybrid")
	presets, err := Providers.Presets(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	registered, err := Providers.Register(context.Background(), ProviderConfig{"preset_id": "deepseek"})
	if err != nil {
		t.Fatal(err)
	}
	_, _ = Providers.Delete(context.Background(), "deepseek")
	_, _ = Providers.DiscoverLocal(context.Background(), LocalDiscoverRequest{BaseURL: "http://127.0.0.1:11434"})
	_, _ = Providers.RegisterLocal(context.Background(), LocalRegisterRequest{BaseURL: "http://127.0.0.1:11434", Model: "qwen", Backend: "ollama"})
	tori, err := Providers.DiscoverTori(context.Background(), true)
	if err != nil {
		t.Fatal(err)
	}
	exec, err := Providers.Exec(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	_, _ = Providers.SetExec(context.Background(), "deepseek")
	reset, err := Providers.ResetBreakers(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	kit := NewAgentKit()

	if models.Models[0]["id"] != "m1" || added["id"] != "m1" || deletedModel["status"] != "ok" || list.Providers[0]["id"] != "deepseek" || !tested["ok"].(bool) || mode["mode"] != "hybrid" || presets["presets"] == nil || registered["provider_id"] != "deepseek" || tori["registered"].(float64) != 1 || exec["exec_provider"] != "deepseek" || reset["reset_count"].(float64) != 1 || kit.Providers != Providers {
		t.Fatalf("unexpected providers results")
	}
	if len(seen) != 20 {
		t.Fatalf("expected 20 requests, got %d: %v", len(seen), seen)
	}
}

func TestCognisHelpers(t *testing.T) {
	var seen []string
	withTestAPI(t, func(w http.ResponseWriter, r *http.Request) {
		seen = append(seen, r.Method+" "+r.URL.String())
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/v1/cognis":
			if r.Method == http.MethodPost {
				_, _ = w.Write([]byte(`{"id":"reviewer","name":"Code Reviewer"}`))
				return
			}
			_, _ = w.Write([]byte(`{"cognis":[{"id":"reviewer"}],"count":1}`))
		case "/v1/cognis/reviewer":
			_, _ = w.Write([]byte(`{"id":"reviewer","enabled":true}`))
		case "/v1/cognis/reviewer/enable", "/v1/cognis/reviewer/disable", "/v1/cognis/reload", "/v1/cognis/alerts/scan", "/v1/cognis/generate", "/v1/cognis/import", "/v1/cognis/reviewer/workflow/summarize", "/v1/cognis/reviewer/experience/record", "/v1/cognis/reviewer/experience/patterns/pat-1/confirm", "/v1/cognis/reviewer/evolve", "/v1/cognis/federation/discover", "/v1/cognis/reviewer/expose", "/v1/cognis/reviewer/unexpose":
			_, _ = w.Write([]byte(`{"status":"ok"}`))
		case "/v1/cognis/traces", "/v1/cognis/reviewer/trace":
			_, _ = w.Write([]byte(`{"traces":[{"id":"t1"}],"count":1}`))
		case "/v1/cognis/stats":
			_, _ = w.Write([]byte(`{"activations":2}`))
		case "/v1/cognis/health", "/v1/cognis/reviewer/health":
			_, _ = w.Write([]byte(`{"healthy":true}`))
		case "/v1/cognis/verify", "/v1/cognis/reviewer/verify":
			_, _ = w.Write([]byte(`{"ok":true}`))
		case "/v1/cognis/alerts":
			_, _ = w.Write([]byte(`{"alerts":[],"count":0}`))
		case "/v1/cognis/export":
			_, _ = w.Write([]byte(`{"bundle":{"version":1}}`))
		case "/v1/cognis/reviewer/workflows":
			_, _ = w.Write([]byte(`{"workflows":["summarize"]}`))
		case "/v1/cognis/reviewer/experience":
			_, _ = w.Write([]byte(`{"enabled":true}`))
		case "/v1/cognis/evolution", "/v1/cognis/reviewer/evolution":
			_, _ = w.Write([]byte(`{"generation":2}`))
		case "/v1/cognis/federation":
			_, _ = w.Write([]byte(`{"enabled":true}`))
		case "/v1/cognis/federation/peers":
			_, _ = w.Write([]byte(`{"peers":[]}`))
		case "/v1/cognis/economics":
			_, _ = w.Write([]byte(`{"cost":0}`))
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
		}
	})

	ctx := context.Background()
	list, err := Cognis.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	created, _ := Cognis.Create(ctx, CogniDeclaration{"id": "reviewer"})
	detail, _ := Cognis.Get(ctx, "reviewer")
	removed, err := Cognis.Remove(ctx, "reviewer")
	if err != nil {
		t.Fatal(err)
	}
	enabled, _ := Cognis.Enable(ctx, "reviewer")
	disabled, _ := Cognis.Disable(ctx, "reviewer")
	reloaded, _ := Cognis.Reload(ctx)
	traces, _ := Cognis.Traces(ctx, 5)
	trace, _ := Cognis.Trace(ctx, "reviewer", 2)
	stats, _ := Cognis.Stats(ctx)
	health, _ := Cognis.Health(ctx, "reviewer")
	verify, _ := Cognis.Verify(ctx, "")
	alerts, _ := Cognis.Alerts(ctx)
	scanned, _ := Cognis.ScanAlerts(ctx)
	generated, _ := Cognis.Generate(ctx, map[string]any{"prompt": "make cogni"})
	exported, _ := Cognis.ExportBundle(ctx)
	imported, _ := Cognis.ImportBundle(ctx, map[string]any{"bundle": map[string]any{}})
	workflows, _ := Cognis.Workflows(ctx, "reviewer")
	ran, _ := Cognis.RunWorkflow(ctx, "reviewer", "summarize", CogniWorkflowRunRequest{"input": "x"})
	experience, _ := Cognis.Experience(ctx, "reviewer")
	recorded, _ := Cognis.RecordExperience(ctx, "reviewer", CogniExperienceRecordRequest{"type": "fact", "data": map[string]any{"fact": "x"}})
	confirmed, _ := Cognis.ConfirmExperiencePattern(ctx, "reviewer", "pat-1")
	evolved, _ := Cognis.Evolve(ctx, "reviewer", map[string]any{})
	evolution, _ := Cognis.Evolution(ctx, "reviewer")
	federation, _ := Cognis.Federation(ctx)
	peers, _ := Cognis.FederationPeers(ctx)
	discovered, _ := Cognis.DiscoverFederation(ctx, map[string]any{"query": "reviewer"})
	exposed, _ := Cognis.Expose(ctx, "reviewer")
	unexposed, _ := Cognis.Unexpose(ctx, "reviewer")
	economics, _ := Cognis.Economics(ctx)
	kit := NewAgentKit()

	if list["count"].(float64) != 1 || created["id"] != "reviewer" || !detail["enabled"].(bool) || removed["id"] != "reviewer" || enabled["status"] != "ok" || disabled["status"] != "ok" || reloaded["status"] != "ok" || traces["count"].(float64) != 1 || trace["count"].(float64) != 1 || stats["activations"].(float64) != 2 || !health["healthy"].(bool) || !verify["ok"].(bool) || alerts["count"].(float64) != 0 || scanned["status"] != "ok" || generated["status"] != "ok" || exported["bundle"] == nil || imported["status"] != "ok" || workflows["workflows"] == nil || ran["status"] != "ok" || !experience["enabled"].(bool) || recorded["status"] != "ok" || confirmed["status"] != "ok" || evolved["status"] != "ok" || evolution["generation"].(float64) != 2 || !federation["enabled"].(bool) || peers["peers"] == nil || discovered["status"] != "ok" || exposed["status"] != "ok" || unexposed["status"] != "ok" || economics["cost"].(float64) != 0 || kit.Cognis != Cognis {
		t.Fatalf("unexpected cognis results")
	}
	if len(seen) != 32 {
		t.Fatalf("expected 32 requests, got %d: %v", len(seen), seen)
	}
}

func TestTraceHelpers(t *testing.T) {
	var seen []string
	withTestAPI(t, func(w http.ResponseWriter, r *http.Request) {
		seen = append(seen, r.Method+" "+r.URL.String())
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/v1/trace/recent":
			_, _ = w.Write([]byte(`{"events":[{"trace_id":"tr/1"}],"count":1}`))
		case "/v1/trace/tr/1":
			_, _ = w.Write([]byte(`{"trace_id":"tr/1","events":[{"trace_id":"tr/1"}],"count":1,"raw":true}`))
		case "/v1/trace/task/task/1":
			_, _ = w.Write([]byte(`{"task_id":"task/1","events":[{"task_id":"task/1"}],"count":1}`))
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
		}
	})

	ctx := context.Background()
	recent, err := Trace.Recent(ctx, TraceRecentOptions{Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	byTrace, err := Trace.ByTraceID(ctx, "tr/1", true)
	if err != nil {
		t.Fatal(err)
	}
	byTask, err := Trace.ByTaskID(ctx, "task/1", false)
	if err != nil {
		t.Fatal(err)
	}
	kit := NewAgentKit()

	if recent.Count != 1 || byTrace.TraceID != "tr/1" || !byTrace.Raw || byTask.TaskID != "task/1" || kit.Trace != Trace {
		t.Fatalf("unexpected trace results")
	}
	if len(seen) != 3 || seen[0] != "GET /v1/trace/recent?limit=10" || seen[1] != "GET /v1/trace/tr%2F1?raw=true" || seen[2] != "GET /v1/trace/task/task%2F1" {
		t.Fatalf("unexpected trace requests: %v", seen)
	}
}

func TestHeartbeatHelpers(t *testing.T) {
	var seen []string
	withTestAPI(t, func(w http.ResponseWriter, r *http.Request) {
		seen = append(seen, r.Method+" "+r.URL.String())
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/v1/heartbeat":
			if r.Method == http.MethodPut {
				_, _ = w.Write([]byte(`{"status":"ok"}`))
				return
			}
			_, _ = w.Write([]byte(`{"running":true}`))
		case "/v1/heartbeat/trigger":
			_, _ = w.Write([]byte(`{"id":"hb1","summary":"checked"}`))
		case "/v1/heartbeat/logs":
			_, _ = w.Write([]byte(`[{"id":"hb1"},{"id":"hb2"}]`))
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
		}
	})

	ctx := context.Background()
	status, err := Heartbeat.Status(ctx)
	if err != nil {
		t.Fatal(err)
	}
	enabled := true
	interval := 30
	updated, err := Heartbeat.Update(ctx, HeartbeatUpdateRequest{Enabled: &enabled, IntervalMinutes: &interval})
	if err != nil {
		t.Fatal(err)
	}
	triggered, err := Heartbeat.Trigger(ctx)
	if err != nil {
		t.Fatal(err)
	}
	logs, err := Heartbeat.Logs(ctx, 2)
	if err != nil {
		t.Fatal(err)
	}
	kit := NewAgentKit()

	if !status["running"].(bool) || updated["status"] != "ok" || triggered["id"] != "hb1" || len(logs) != 2 || kit.Heartbeat != Heartbeat {
		t.Fatalf("unexpected heartbeat results")
	}
	if len(seen) != 4 || seen[0] != "GET /v1/heartbeat" || seen[1] != "PUT /v1/heartbeat" || seen[2] != "POST /v1/heartbeat/trigger" || seen[3] != "GET /v1/heartbeat/logs?limit=2" {
		t.Fatalf("unexpected heartbeat requests: %v", seen)
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

func TestBotsNamespaceManagesBotsInboxAndChannels(t *testing.T) {
	var seen []string
	withTestAPI(t, func(w http.ResponseWriter, r *http.Request) {
		seen = append(seen, r.Method+" "+r.URL.String())
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/v1/bots":
			if r.Method == http.MethodPost {
				var body CreateBotRequest
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Fatal(err)
				}
				if body.Name != "planner" || body.Config["model"] != "deepseek" {
					t.Fatalf("unexpected bot create body: %+v", body)
				}
				_, _ = w.Write([]byte(`{"id":"bot-2","name":"planner"}`))
				return
			}
			_, _ = w.Write([]byte(`{"bots":[{"id":"bot-1","name":"default"}],"total":1,"active":1}`))
		case "/v1/bots/detail":
			if r.URL.Query().Get("id") != "bot/1" {
				t.Fatalf("unexpected bot id: %s", r.URL.RawQuery)
			}
			switch r.Method {
			case http.MethodGet:
				_, _ = w.Write([]byte(`{"id":"bot-1","name":"default"}`))
			case http.MethodPut:
				var body UpdateBotRequest
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Fatal(err)
				}
				if body.Active == nil || *body.Active {
					t.Fatalf("unexpected bot update body: %+v", body)
				}
				_, _ = w.Write([]byte(`{"id":"bot-1","active":false}`))
			case http.MethodDelete:
				_, _ = w.Write([]byte(`{"status":"ok"}`))
			}
		case "/v1/inbox":
			switch r.Method {
			case http.MethodGet:
				if r.URL.Query().Get("unread") != "true" {
					t.Fatalf("expected unread query")
				}
				_, _ = w.Write([]byte(`{"items":[{"id":"in-1","content":"ping"}],"count":{"unread":1,"total":1}}`))
			case http.MethodPost:
				var body PushInboxRequest
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Fatal(err)
				}
				if body.Content != "ping" || body.Action != "trigger" {
					t.Fatalf("unexpected inbox body: %+v", body)
				}
				_, _ = w.Write([]byte(`{"id":"in-2","content":"ping","action":"trigger"}`))
			case http.MethodDelete:
				_, _ = w.Write([]byte(`{"status":"ok"}`))
			}
		case "/v1/inbox/read":
			_, _ = w.Write([]byte(`{"marked":2}`))
		case "/v1/channels/groups":
			if r.URL.Query().Get("type") != "telegram" {
				t.Fatalf("expected telegram type")
			}
			_, _ = w.Write([]byte(`{"groups":[{"id":"g1","channel_type":"telegram"}],"count":1}`))
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
		}
	})

	ctx := context.Background()
	list, err := Bots.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	created, err := Bots.Create(ctx, CreateBotRequest{Name: "planner", Config: BotConfig{"model": "deepseek"}})
	if err != nil {
		t.Fatal(err)
	}
	got, err := Bots.Get(ctx, "bot/1")
	if err != nil {
		t.Fatal(err)
	}
	updated, err := Bots.SetActive(ctx, "bot/1", false)
	if err != nil {
		t.Fatal(err)
	}
	deleted, err := Bots.Delete(ctx, "bot/1")
	if err != nil {
		t.Fatal(err)
	}
	inbox, err := Bots.Inbox(ctx, true)
	if err != nil {
		t.Fatal(err)
	}
	pushed, err := Bots.PushInbox(ctx, PushInboxRequest{Source: "webhook", Content: "ping", Action: "trigger"})
	if err != nil {
		t.Fatal(err)
	}
	removed, err := Bots.DeleteInbox(ctx, "in-1")
	if err != nil {
		t.Fatal(err)
	}
	marked, err := Bots.MarkInboxRead(ctx, []string{"in-1", "in-2"})
	if err != nil {
		t.Fatal(err)
	}
	markedAll, err := Bots.MarkAllInboxRead(ctx)
	if err != nil {
		t.Fatal(err)
	}
	groups, err := Bots.ChannelGroups(ctx, "telegram")
	if err != nil {
		t.Fatal(err)
	}

	if list.Total != 1 || created["id"] != "bot-2" || got["id"] != "bot-1" || updated["active"] != false || deleted["status"] != "ok" || inbox.Count.Unread != 1 || pushed["action"] != "trigger" || removed["status"] != "ok" || marked.Marked != 2 || markedAll.Marked != 2 || groups.Groups[0]["id"] != "g1" {
		t.Fatalf("unexpected bots results")
	}
	if NewAgentKit().Bots != Bots {
		t.Fatalf("agent kit should expose Bots namespace")
	}
	if len(seen) != 11 {
		t.Fatalf("expected 11 requests, got %d: %v", len(seen), seen)
	}
}

func TestDocumentsNamespaceGeneratesArtifacts(t *testing.T) {
	var seen []string
	withTestAPI(t, func(w http.ResponseWriter, r *http.Request) {
		seen = append(seen, r.Method+" "+r.URL.String())
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/v1/documents/templates":
			_, _ = w.Write([]byte(`{"templates":[{"id":"brief","format":"docx"}]}`))
		case "/v1/documents/generate":
			var body DocumentGenerateRequest
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			if body.Content == "" || body.Format == "" {
				t.Fatalf("unexpected document body: %+v", body)
			}
			_, _ = w.Write([]byte(`{"result":"ok","path":"out.` + body.Format + `","format":"` + body.Format + `"}`))
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
		}
	})
	ctx := context.Background()
	templates, err := Documents.Templates(ctx)
	if err != nil {
		t.Fatal(err)
	}
	docx, err := Documents.GenerateDocx(ctx, "hello", "out.docx", "Report")
	if err != nil {
		t.Fatal(err)
	}
	xlsx, err := Documents.GenerateXlsx(ctx, "a,b", "out.xlsx", "Sheet", "Data")
	if err != nil {
		t.Fatal(err)
	}
	pptx, err := Documents.GeneratePptx(ctx, "slides", "out.pptx", "Deck")
	if err != nil {
		t.Fatal(err)
	}
	html, err := Documents.GenerateHtml(ctx, "<p>hi</p>", "out.html", "HTML")
	if err != nil {
		t.Fatal(err)
	}
	if templates.Templates[0]["id"] != "brief" || docx.Format != "docx" || xlsx.Format != "xlsx" || pptx.Format != "pptx" || html.Format != "html" {
		t.Fatalf("unexpected document results")
	}
	if NewAgentKit().Documents != Documents {
		t.Fatalf("agent kit should expose Documents namespace")
	}
	if len(seen) != 5 {
		t.Fatalf("expected 5 requests, got %d: %v", len(seen), seen)
	}
}

func TestWebChatNamespaceBuildsWidgetHelpers(t *testing.T) {
	withTestAPI(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/webchat/widget.js" {
			t.Fatalf("unexpected request: %s", r.URL.String())
		}
		w.Header().Set("Content-Type", "application/javascript")
		_, _ = w.Write([]byte("console.log('ok')"))
	})
	url := WebChat.WidgetURL()
	snippet, err := WebChat.EmbedSnippet(WebChatEmbedOptions{APIKey: "key&1", Title: `Tori "Chat"`, Theme: "dark"})
	if err != nil {
		t.Fatal(err)
	}
	script, err := WebChat.WidgetScript(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(url, "/v1/webchat/widget.js") || !strings.Contains(snippet, `data-api-key="key&amp;1"`) || !strings.Contains(snippet, `data-title="Tori &quot;Chat&quot;"`) || script != "console.log('ok')" {
		t.Fatalf("unexpected webchat helpers: %s %s %s", url, snippet, script)
	}
	if _, err := WebChat.EmbedSnippet(WebChatEmbedOptions{}); err == nil {
		t.Fatalf("expected missing APIKey error")
	}
	if NewAgentKit().WebChat != WebChat {
		t.Fatalf("agent kit should expose WebChat namespace")
	}
}

func TestSandboxNamespaceManagesRuntime(t *testing.T) {
	var seen []string
	withTestAPI(t, func(w http.ResponseWriter, r *http.Request) {
		seen = append(seen, r.Method+" "+r.URL.String())
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/v1/sandbox/exec":
			var body SandboxExecRequest
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			if body.Command != "python" || body.Args[0] != "-V" {
				t.Fatalf("unexpected sandbox exec body: %+v", body)
			}
			_, _ = w.Write([]byte(`{"stdout":"ok","exit_code":0}`))
		case "/v1/sandbox/probe":
			_, _ = w.Write([]byte(`{"cloud_runner_ready":true}`))
		case "/v1/sandbox/desktop":
			_, _ = w.Write([]byte(`{"ok":true,"sandbox":{"id":"desk-1"}}`))
		case "/v1/sandbox/desktop/status":
			_, _ = w.Write([]byte(`{"ok":true,"running":true}`))
		case "/v1/sandbox/desktop/destroy":
			_, _ = w.Write([]byte(`{"ok":true,"message":"destroyed"}`))
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
		}
	})
	ctx := context.Background()
	exec, err := Sandbox.Exec(ctx, SandboxExecRequest{Command: "python", Args: []string{"-V"}})
	if err != nil {
		t.Fatal(err)
	}
	probe, err := Sandbox.Probe(ctx)
	if err != nil {
		t.Fatal(err)
	}
	created, err := Sandbox.CreateDesktop(ctx)
	if err != nil {
		t.Fatal(err)
	}
	status, err := Sandbox.DesktopStatus(ctx)
	if err != nil {
		t.Fatal(err)
	}
	destroyed, err := Sandbox.DestroyDesktop(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if exec["stdout"] != "ok" || probe["cloud_runner_ready"] != true || created["ok"] != true || status["running"] != true || destroyed["message"] != "destroyed" {
		t.Fatalf("unexpected sandbox results")
	}
	if NewAgentKit().Sandbox != Sandbox {
		t.Fatalf("agent kit should expose Sandbox namespace")
	}
	if len(seen) != 5 {
		t.Fatalf("expected 5 requests, got %d: %v", len(seen), seen)
	}
}

func TestRouterNamespaceReadsStats(t *testing.T) {
	var seen []string
	withTestAPI(t, func(w http.ResponseWriter, r *http.Request) {
		seen = append(seen, r.Method+" "+r.URL.String())
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path != "/v1/router/stats" || r.Method != http.MethodGet {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
		}
		_, _ = w.Write([]byte(`{"slots":{"coding":{"provider":"deepseek"}},"stats":{"routed":7,"fallback":1}}`))
	})

	stats, err := Router.Stats(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if stats["slots"] == nil || stats["stats"].(map[string]any)["routed"].(float64) != 7 {
		t.Fatalf("unexpected router stats: %+v", stats)
	}
	if NewAgentKit().Router != Router {
		t.Fatalf("agent kit should expose Router namespace")
	}
	if len(seen) != 1 || seen[0] != "GET /v1/router/stats" {
		t.Fatalf("unexpected router requests: %v", seen)
	}
}

func TestSkillHubNamespaceManagesIncrementalPackages(t *testing.T) {
	var seen []string
	withTestAPI(t, func(w http.ResponseWriter, r *http.Request) {
		seen = append(seen, r.Method+" "+r.URL.String())
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/skillhub/search":
			if r.URL.Query().Get("q") != "browser" || r.URL.Query().Get("limit") != "5" {
				t.Fatalf("unexpected search query: %s", r.URL.RawQuery)
			}
			_, _ = w.Write([]byte(`{"results":[{"slug":"browser"}],"count":1}`))
		case "/api/skillhub/installed":
			_, _ = w.Write([]byte(`{"skills":[{"slug":"browser"}],"count":1}`))
		case "/api/skillhub/install", "/api/skillhub/uninstall", "/api/skillhub/update", "/api/skillhub/rollback", "/api/skillhub/policy":
			_, _ = w.Write([]byte(`{"ok":true,"slug":"browser"}`))
		case "/api/skillhub/trending":
			_, _ = w.Write([]byte(`{"skills":[{"slug":"browser"}],"count":1}`))
		case "/api/skillhub/detail":
			_, _ = w.Write([]byte(`{"slug":"browser","name":"Browser"}`))
		case "/api/skillhub/check-updates":
			_, _ = w.Write([]byte(`{"updates":[{"slug":"browser"}]}`))
		case "/api/skillhub/versions":
			_, _ = w.Write([]byte(`{"versions":["1.0.0"]}`))
		case "/api/skillhub/policy/check":
			_, _ = w.Write([]byte(`{"allowed":true}`))
		case "/api/skillhub/analytics":
			_, _ = w.Write([]byte(`{"installed_count":1}`))
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
		}
	})

	ctx := context.Background()
	search, err := SkillHub.Search(ctx, SkillHubQuery{Q: "browser", Limit: 5, Source: "clawhub"})
	if err != nil {
		t.Fatal(err)
	}
	installed, _ := SkillHub.Installed(ctx)
	installedOne, _ := SkillHub.Install(ctx, "browser")
	_, _ = SkillHub.Uninstall(ctx, "browser")
	trending, _ := SkillHub.Trending(ctx, SkillHubQuery{Limit: 3, Cursor: "n1"})
	detail, _ := SkillHub.Detail(ctx, "browser")
	updates, _ := SkillHub.CheckUpdates(ctx)
	_, _ = SkillHub.Update(ctx, "browser")
	_, _ = SkillHub.Rollback(ctx, "browser", "1.0.0")
	versions, _ := SkillHub.Versions(ctx, "browser")
	policy, _ := SkillHub.Policy(ctx)
	_, _ = SkillHub.UpdatePolicy(ctx, map[string]any{"min_security_score": 80})
	check, _ := SkillHub.PolicyCheck(ctx, "browser")
	analytics, _ := SkillHub.Analytics(ctx)

	if search["count"].(float64) != 1 || installed["count"].(float64) != 1 || installedOne["slug"] != "browser" || trending["count"].(float64) != 1 || detail["name"] != "Browser" || updates["updates"] == nil || versions["versions"] == nil || policy["ok"] != true || check["allowed"] != true || analytics["installed_count"].(float64) != 1 {
		t.Fatalf("unexpected SkillHub results")
	}
	if NewAgentKit().SkillHub != SkillHub {
		t.Fatalf("agent kit should expose SkillHub namespace")
	}
	if len(seen) != 14 {
		t.Fatalf("expected 14 requests, got %d: %v", len(seen), seen)
	}
}

func TestPluginsNamespaceManagesPluginLifecycle(t *testing.T) {
	var seen []string
	withTestAPI(t, func(w http.ResponseWriter, r *http.Request) {
		seen = append(seen, r.Method+" "+r.URL.String())
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/v1/plugins":
			_, _ = w.Write([]byte(`{"plugins":[{"name":"demo","enabled":true}]}`))
		case "/v1/plugins/toggle":
			_, _ = w.Write([]byte(`{"name":"demo","enabled":true,"skills_count":1}`))
		case "/v1/plugins/create":
			_, _ = w.Write([]byte(`{"status":"created","name":"demo","dir":"plugins/demo"}`))
		case "/v1/plugins/delete":
			_, _ = w.Write([]byte(`{"status":"deleted","name":"demo"}`))
		case "/v1/plugins/files":
			if r.Method == http.MethodPut {
				_, _ = w.Write([]byte(`{"status":"saved"}`))
			} else {
				_, _ = w.Write([]byte(`{"files":[{"name":"handler.py","content":"print('ok')","size":11}]}`))
			}
		case "/v1/plugins/ui":
			_, _ = w.Write([]byte(`{"tabs":[]}`))
		case "/v1/plugins/reload":
			_, _ = w.Write([]byte(`{"status":"reloaded","skills":1}`))
		case "/v1/plugins/open-folder":
			_, _ = w.Write([]byte(`{"ok":true,"path":"plugins/demo"}`))
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
		}
	})

	ctx := context.Background()
	list, err := Plugins.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	toggled, _ := Plugins.Toggle(ctx, "demo", true)
	created, _ := Plugins.Create(ctx, PluginCreateRequest{Name: "demo", Description: "Demo", Language: "python"})
	deleted, _ := Plugins.Delete(ctx, "demo")
	files, _ := Plugins.Files(ctx, "demo")
	saved, _ := Plugins.SaveFile(ctx, "demo", "handler.py", "print('ok')", "demo")
	ui, _ := Plugins.UI(ctx)
	reloaded, _ := Plugins.Reload(ctx)
	opened, _ := Plugins.OpenFolder(ctx, "demo")

	if list["plugins"] == nil || toggled["enabled"] != true || created["name"] != "demo" || deleted["status"] != "deleted" || files["files"] == nil || saved["status"] != "saved" || ui["tabs"] == nil || reloaded["skills"].(float64) != 1 || opened["ok"] != true {
		t.Fatalf("unexpected Plugins results")
	}
	if NewAgentKit().Plugins != Plugins {
		t.Fatalf("agent kit should expose Plugins namespace")
	}
	if len(seen) != 9 {
		t.Fatalf("expected 9 requests, got %d: %v", len(seen), seen)
	}
}

func TestSkillsNamespaceManagesRuntimeSkills(t *testing.T) {
	var seen []string
	withTestAPI(t, func(w http.ResponseWriter, r *http.Request) {
		seen = append(seen, r.Method+" "+r.URL.String())
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/v1/skills":
			_, _ = w.Write([]byte(`{"skills":[{"name":"web.search","description":"search"}],"count":1}`))
		case "/v1/skills/scan":
			_, _ = w.Write([]byte(`{"status":"scanned","skills_loaded":2}`))
		case "/v1/skills/dynamic":
			_, _ = w.Write([]byte(`{"skills":[{"name":"draft_doc","approval_status":"pending"}]}`))
		case "/v1/skills/approve":
			_, _ = w.Write([]byte(`{"status":"ok","name":"draft_doc"}`))
		case "/v1/skills/reject":
			_, _ = w.Write([]byte(`{"status":"ok","name":"old_skill"}`))
		case "/v1/skill-suggestions":
			_, _ = w.Write([]byte(`{"suggestions":[{"name":"summarize"}]}`))
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
		}
	})

	ctx := context.Background()
	list, err := Skills.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	scan, _ := Skills.Scan(ctx)
	dynamic, _ := Skills.Dynamic(ctx)
	approved, _ := Skills.Approve(ctx, "draft_doc", "use safely")
	rejected, _ := Skills.Reject(ctx, "old_skill")
	suggestions, _ := Skills.Suggestions(ctx, "sess-1")

	if list["count"].(float64) != 1 || scan["status"] != "scanned" || dynamic["skills"] == nil || approved["status"] != "ok" || rejected["name"] != "old_skill" || suggestions["suggestions"] == nil {
		t.Fatalf("unexpected Skills results")
	}
	if NewAgentKit().Skills != Skills {
		t.Fatalf("agent kit should expose Skills namespace")
	}
	if len(seen) != 6 {
		t.Fatalf("expected 6 requests, got %d: %v", len(seen), seen)
	}
}

func TestModelsNamespaceWrapsModelRegistry(t *testing.T) {
	var seen []string
	withTestAPI(t, func(w http.ResponseWriter, r *http.Request) {
		seen = append(seen, r.Method+" "+r.URL.String())
		w.Header().Set("Content-Type", "application/json")
		switch r.Method {
		case http.MethodGet:
			_, _ = w.Write([]byte(`{"models":[{"id":"kimi","model_id":"moonshot-v1-8k"}]}`))
		case http.MethodPost:
			_, _ = w.Write([]byte(`{"id":"custom","model_id":"custom-model"}`))
		case http.MethodDelete:
			_, _ = w.Write([]byte(`{"status":"ok"}`))
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
		}
	})

	ctx := context.Background()
	list, err := Models.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	added, _ := Models.Add(ctx, ModelEntry{"id": "custom", "model_id": "custom-model"})
	deleted, _ := Models.Delete(ctx, "custom")

	if list.Models[0]["model_id"] != "moonshot-v1-8k" || added["model_id"] != "custom-model" || deleted["status"] != "ok" {
		t.Fatalf("unexpected Models results")
	}
	if NewAgentKit().Models != Models {
		t.Fatalf("agent kit should expose Models namespace")
	}
	if len(seen) != 3 || seen[0] != "GET /v1/models" || seen[2] != "DELETE /v1/models?id=custom" {
		t.Fatalf("unexpected model requests: %v", seen)
	}
}

func TestWebSearchNamespaceWrapsDiscoverySearch(t *testing.T) {
	var seen []string
	withTestAPI(t, func(w http.ResponseWriter, r *http.Request) {
		seen = append(seen, r.Method+" "+r.URL.String())
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/v1/search":
			_, _ = w.Write([]byte(`{"results":[{"title":"云雀"}],"total":1}`))
		case "/v1/search/providers":
			_, _ = w.Write([]byte(`{"enabled":true,"providers":["local"]}`))
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
		}
	})

	ctx := context.Background()
	results, err := WebSearch.Query(ctx, "agent", 3, "local")
	if err != nil {
		t.Fatal(err)
	}
	providers, err := WebSearch.Providers(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if results["results"] == nil || providers["enabled"] != true {
		t.Fatalf("unexpected web search results")
	}
	if NewAgentKit().WebSearch != WebSearch {
		t.Fatalf("agent kit should expose WebSearch namespace")
	}
	if len(seen) != 2 || seen[0] != "GET /v1/search?limit=3&provider=local&q=agent" || seen[1] != "GET /v1/search/providers" {
		t.Fatalf("unexpected web search requests: %v", seen)
	}
}

func TestEmbeddingsNamespaceWrapsDiscoveryEmbeddings(t *testing.T) {
	var seen []string
	withTestAPI(t, func(w http.ResponseWriter, r *http.Request) {
		seen = append(seen, r.Method+" "+r.URL.String())
		w.Header().Set("Content-Type", "application/json")
		switch r.Method {
		case http.MethodGet:
			_, _ = w.Write([]byte(`{"providers":["local"]}`))
		case http.MethodPost:
			var body map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			if body["text"] != "hello" || body["provider"] != "local" {
				t.Fatalf("unexpected embeddings body: %+v", body)
			}
			_, _ = w.Write([]byte(`{"embedding":[0.1,0.2],"dimensions":2}`))
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
		}
	})

	ctx := context.Background()
	providers, err := Embeddings.Providers(ctx)
	if err != nil {
		t.Fatal(err)
	}
	embedded, err := Embeddings.Embed(ctx, "hello", "local")
	if err != nil {
		t.Fatal(err)
	}
	if providers["providers"] == nil || embedded["dimensions"].(float64) != 2 {
		t.Fatalf("unexpected embeddings results")
	}
	if NewAgentKit().Embeddings != Embeddings {
		t.Fatalf("agent kit should expose Embeddings namespace")
	}
	if len(seen) != 2 || seen[0] != "GET /v1/embeddings" || seen[1] != "POST /v1/embeddings" {
		t.Fatalf("unexpected embeddings requests: %v", seen)
	}
}

func TestIdentityNamespaceWrapsDiscoveryIdentity(t *testing.T) {
	var seen []string
	withTestAPI(t, func(w http.ResponseWriter, r *http.Request) {
		seen = append(seen, r.Method+" "+r.URL.String())
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/v1/identity/resolve":
			_, _ = w.Write([]byte(`{"unified_id":"wechat:u1","display_name":"小云"}`))
		case "/v1/identity/profiles":
			_, _ = w.Write([]byte(`{"profiles":[{"unified_id":"wechat:u1"}]}`))
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
		}
	})

	ctx := context.Background()
	resolved, err := Identity.Resolve(ctx, DiscoveryResolveIdentityRequest{Channel: "wechat", UserID: "u1", DisplayName: "小云"})
	if err != nil {
		t.Fatal(err)
	}
	profiles, err := Identity.Profiles(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if resolved["unified_id"] != "wechat:u1" || profiles["profiles"] == nil {
		t.Fatalf("unexpected Identity results")
	}
	if NewAgentKit().Identity != Identity {
		t.Fatalf("agent kit should expose Identity namespace")
	}
	if len(seen) != 2 || seen[0] != "POST /v1/identity/resolve" || seen[1] != "GET /v1/identity/profiles" {
		t.Fatalf("unexpected identity requests: %v", seen)
	}
}
