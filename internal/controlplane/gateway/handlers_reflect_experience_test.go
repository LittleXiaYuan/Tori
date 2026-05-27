package gateway

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	reflectpkg "yunque-agent/internal/experimental/reflect"
)

func TestHandleExperiencesPostStoresWorkloadFeedback(t *testing.T) {
	store := reflectpkg.NewExperienceStore(filepath.Join(t.TempDir(), "experiences.json"))
	g := &Gateway{experienceStore: store}
	body := bytes.NewBufferString(`{
		"id":"workload-feedback-browser-rpa",
		"source":"workload_feedback",
		"source_id":"browser-rpa",
		"category":"workload_feedback",
		"outcome":"partial",
		"lesson":"最顺手：浏览器意图能快速生成计划；最不顺手：入口还需要更明显",
		"context":"工作负载：浏览器 / RPA\n能力范围：browser.intent.plan, rpa.replay.dry_run",
		"tags":["workload:browser-rpa","capability:browser.intent.plan","findability:partial"]
	}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/reflect/experiences", body)
	rec := httptest.NewRecorder()

	g.handleExperiences(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	all := store.All()
	if len(all) != 1 {
		t.Fatalf("expected one stored experience, got %d", len(all))
	}
	if all[0].Source != "workload_feedback" || all[0].Category != "workload_feedback" {
		t.Fatalf("unexpected stored workload feedback: %#v", all[0])
	}
	if !strings.Contains(all[0].Lesson, "最顺手") || !reflectExperienceHasTag(all[0], "workload:browser-rpa") {
		t.Fatalf("stored experience lost workload detail: %#v", all[0])
	}
}

func TestHandleExperiencesPostFeedsWorkloadFeedbackThroughReflectiveLoop(t *testing.T) {
	store := reflectpkg.NewExperienceStore(filepath.Join(t.TempDir(), "experiences.json"))
	g := &Gateway{experienceStore: store}
	g.WireReflectionLoop()

	body := strings.NewReader(`{
		"source":"workload_feedback",
		"source_id":"browser-rpa",
		"category":"workload_feedback",
		"outcome":"success",
		"lesson":"工作负载【浏览器 / RPA】体验反馈\n30 秒找到入口：是\n最顺手：录制回放",
		"context":"能力范围：browser.intent.plan, rpa.replay.dry_run",
		"tags":["workload:browser-rpa","findability:yes"]
	}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/reflect/experiences", body)
	rec := httptest.NewRecorder()

	g.handleExperiences(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"ingested_by":"reflective_loop"`) {
		t.Fatalf("expected reflective loop ingestion marker, body = %s", rec.Body.String())
	}
	strategies := store.CompileStrategies(5)
	if !strings.Contains(strategies, "工作负载【浏览器 / RPA】体验反馈") {
		t.Fatalf("reflective loop output did not reference workload feedback: %q", strategies)
	}
	if !strings.Contains(strategies, "推荐:") {
		t.Fatalf("success feedback should compile as recommendation, got %q", strategies)
	}
	all := store.All()
	if len(all) != 1 {
		t.Fatalf("expected one stored feedback experience, got %d", len(all))
	}
	for _, want := range []string{"source:workload_feedback", "category:workload_feedback", "outcome:success", "source_id:browser-rpa"} {
		if !reflectExperienceHasTag(all[0], want) {
			t.Fatalf("reflective loop did not annotate feedback tag %q: %#v", want, all[0])
		}
	}
}

func TestHandleExperiencesSearchRespectsFilters(t *testing.T) {
	store := reflectpkg.NewExperienceStore(filepath.Join(t.TempDir(), "experiences.json"))
	store.Add(reflectpkg.Experience{
		ID:       "success-task",
		Source:   "task",
		Category: "strategy",
		Outcome:  "success",
		Lesson:   "search strategy should be reused for code review",
	})
	store.Add(reflectpkg.Experience{
		ID:       "failed-task",
		Source:   "task",
		Category: "strategy",
		Outcome:  "failure",
		Lesson:   "search strategy timed out during code review",
	})
	store.Add(reflectpkg.Experience{
		ID:       "success-interaction",
		Source:   "interaction",
		Category: "strategy",
		Outcome:  "success",
		Lesson:   "search strategy helped answer chat context",
	})

	g := &Gateway{experienceStore: store}
	req := httptest.NewRequest(http.MethodGet, "/v1/reflect/experiences?q=search&source=task&outcome=success", nil)
	rec := httptest.NewRecorder()

	g.handleExperiences(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var body struct {
		Experiences []reflectpkg.Experience `json:"experiences"`
		Total       int                     `json:"total"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Total != 1 || len(body.Experiences) != 1 {
		t.Fatalf("expected one filtered experience, got total=%d entries=%d body=%s", body.Total, len(body.Experiences), rec.Body.String())
	}
	if body.Experiences[0].ID != "success-task" {
		t.Fatalf("unexpected experience id %q", body.Experiences[0].ID)
	}
}

func TestHandleExperiencesSearchFiltersBeforeLimit(t *testing.T) {
	store := reflectpkg.NewExperienceStore(filepath.Join(t.TempDir(), "experiences.json"))
	store.Add(reflectpkg.Experience{
		ID:       "older-task-match",
		Source:   "task",
		Category: "strategy",
		Outcome:  "success",
		Lesson:   "needle strategy should still be found after many newer chat matches",
	})
	for i := 0; i < 60; i++ {
		store.Add(reflectpkg.Experience{
			ID:       "newer-chat-match",
			Source:   "interaction",
			Category: "strategy",
			Outcome:  "success",
			Lesson:   "needle strategy from chat should not hide older task matches",
		})
	}

	g := &Gateway{experienceStore: store}
	req := httptest.NewRequest(http.MethodGet, "/v1/reflect/experiences?q=needle&source=task&limit=1", nil)
	rec := httptest.NewRecorder()

	g.handleExperiences(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var body struct {
		Experiences []reflectpkg.Experience `json:"experiences"`
		Total       int                     `json:"total"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Total != 1 || len(body.Experiences) != 1 || body.Experiences[0].ID != "older-task-match" {
		t.Fatalf("search should filter before limit and preserve matching task, body=%s", rec.Body.String())
	}
}

func TestHandleExperiencesSearchMatchesNaturalQueryTokens(t *testing.T) {
	store := reflectpkg.NewExperienceStore(filepath.Join(t.TempDir(), "experiences.json"))
	store.Add(reflectpkg.Experience{
		ID:       "code-review",
		Source:   "task",
		Category: "strategy",
		Outcome:  "success",
		Lesson:   "code review should run tests before summarizing risk",
	})
	store.Add(reflectpkg.Experience{
		ID:       "web-search",
		Source:   "task",
		Category: "strategy",
		Outcome:  "success",
		Lesson:   "web search should cite sources before summarizing news",
	})

	g := &Gateway{experienceStore: store}
	req := httptest.NewRequest(http.MethodGet, "/v1/reflect/experiences?q=%E8%AF%B7%E5%81%9A+code+review&limit=1", nil)
	rec := httptest.NewRecorder()

	g.handleExperiences(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var body struct {
		Experiences []reflectpkg.Experience `json:"experiences"`
		Total       int                     `json:"total"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Total != 1 || len(body.Experiences) != 1 || body.Experiences[0].ID != "code-review" {
		t.Fatalf("natural query should match tokenized experience, body=%s", rec.Body.String())
	}
}

func TestHandleExperiencesLimitAppliesAfterFilters(t *testing.T) {
	store := reflectpkg.NewExperienceStore(filepath.Join(t.TempDir(), "experiences.json"))
	store.Add(reflectpkg.Experience{ID: "old-task", Source: "task", Category: "strategy", Outcome: "partial", Lesson: "older task lesson"})
	store.Add(reflectpkg.Experience{ID: "chat", Source: "interaction", Category: "strategy", Outcome: "partial", Lesson: "chat lesson"})
	store.Add(reflectpkg.Experience{ID: "new-task", Source: "task", Category: "strategy", Outcome: "partial", Lesson: "newer task lesson"})

	g := &Gateway{experienceStore: store}
	req := httptest.NewRequest(http.MethodGet, "/v1/reflect/experiences?source=task&limit=1", nil)
	rec := httptest.NewRecorder()

	g.handleExperiences(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var body struct {
		Experiences []reflectpkg.Experience `json:"experiences"`
		Total       int                     `json:"total"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Total != 1 || len(body.Experiences) != 1 {
		t.Fatalf("expected one limited experience, got total=%d entries=%d body=%s", body.Total, len(body.Experiences), rec.Body.String())
	}
	if body.Experiences[0].ID != "new-task" {
		t.Fatalf("limit should apply after source filter and preserve newest order, got %q", body.Experiences[0].ID)
	}
}

func TestHandleExperiencesFiltersByTag(t *testing.T) {
	store := reflectpkg.NewExperienceStore(filepath.Join(t.TempDir(), "experiences.json"))
	store.Add(reflectpkg.Experience{ID: "low-quality", Source: "task", Category: "strategy", Outcome: "partial", Lesson: "low quality lesson", Tags: []string{"quality:4", "outcome:partial"}})
	store.Add(reflectpkg.Experience{ID: "high-quality", Source: "task", Category: "strategy", Outcome: "success", Lesson: "high quality lesson", Tags: []string{"quality:9", "outcome:success"}})
	store.Add(reflectpkg.Experience{ID: "chat-high", Source: "interaction", Category: "strategy", Outcome: "success", Lesson: "chat high lesson", Tags: []string{"quality:9", "outcome:success"}})

	g := &Gateway{experienceStore: store}
	req := httptest.NewRequest(http.MethodGet, "/v1/reflect/experiences?source=task&tag=quality:9", nil)
	rec := httptest.NewRecorder()

	g.handleExperiences(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var body struct {
		Experiences []reflectpkg.Experience `json:"experiences"`
		Total       int                     `json:"total"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Total != 1 || len(body.Experiences) != 1 {
		t.Fatalf("expected one tag-filtered experience, got total=%d entries=%d body=%s", body.Total, len(body.Experiences), rec.Body.String())
	}
	if body.Experiences[0].ID != "high-quality" {
		t.Fatalf("unexpected experience id %q", body.Experiences[0].ID)
	}
}

func TestHandleStrategiesRespectsLimit(t *testing.T) {
	store := reflectpkg.NewExperienceStore(filepath.Join(t.TempDir(), "experiences.json"))
	store.Add(reflectpkg.Experience{ID: "first", Outcome: "success", Lesson: "first strategy lesson should appear in compiled hints"})
	store.Add(reflectpkg.Experience{ID: "second", Outcome: "success", Lesson: "second strategy lesson should appear in compiled hints"})

	g := &Gateway{experienceStore: store}
	req := httptest.NewRequest(http.MethodGet, "/v1/reflect/strategies?limit=1", nil)
	rec := httptest.NewRecorder()

	g.handleStrategies(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var body struct {
		Strategies string `json:"strategies"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Strategies == "" {
		t.Fatal("expected strategies")
	}
	if count := strings.Count(body.Strategies, "推荐:"); count != 1 {
		t.Fatalf("expected one compiled strategy, got %d in %q", count, body.Strategies)
	}
}

func TestHandleStrategiesRespectsFilters(t *testing.T) {
	store := reflectpkg.NewExperienceStore(filepath.Join(t.TempDir(), "experiences.json"))
	store.Add(reflectpkg.Experience{ID: "low", Source: "task", Category: "strategy", Outcome: "success", Lesson: "low quality strategy lesson should not appear", Tags: []string{"quality:4"}})
	store.Add(reflectpkg.Experience{ID: "target", Source: "task", Category: "strategy", Outcome: "success", Lesson: "high quality strategy lesson should appear", Tags: []string{"quality:9"}})
	store.Add(reflectpkg.Experience{ID: "chat", Source: "interaction", Category: "strategy", Outcome: "success", Lesson: "chat high quality strategy lesson should not appear", Tags: []string{"quality:9"}})

	g := &Gateway{experienceStore: store}
	req := httptest.NewRequest(http.MethodGet, "/v1/reflect/strategies?source=task&tag=quality:9", nil)
	rec := httptest.NewRecorder()

	g.handleStrategies(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var body struct {
		Strategies string `json:"strategies"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(body.Strategies, "high quality strategy lesson should appear") {
		t.Fatalf("expected filtered strategy, got %q", body.Strategies)
	}
	if strings.Contains(body.Strategies, "low quality") || strings.Contains(body.Strategies, "chat high") {
		t.Fatalf("strategies leaked non-matching experiences: %q", body.Strategies)
	}
}

func TestHandleExperienceStatsRespectsFilters(t *testing.T) {
	store := reflectpkg.NewExperienceStore(filepath.Join(t.TempDir(), "experiences.json"))
	store.Add(reflectpkg.Experience{ID: "task-success", Source: "task", Category: "strategy", Outcome: "success", Lesson: "task success lesson"})
	store.Add(reflectpkg.Experience{ID: "task-failure", Source: "task", Category: "strategy", Outcome: "failure", Lesson: "task failure lesson"})
	store.Add(reflectpkg.Experience{ID: "chat-success", Source: "interaction", Category: "strategy", Outcome: "success", Lesson: "chat success lesson"})

	g := &Gateway{experienceStore: store}
	req := httptest.NewRequest(http.MethodGet, "/v1/reflect/experiences?stats=true&source=task&outcome=success", nil)
	rec := httptest.NewRecorder()

	g.handleExperiences(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var body reflectpkg.ExperienceStats
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Total != 1 {
		t.Fatalf("expected one filtered stat entry, got total=%d body=%s", body.Total, rec.Body.String())
	}
	if body.BySource["task"] != 1 || body.ByOutcome["success"] != 1 {
		t.Fatalf("unexpected filtered stats: %+v", body)
	}
	if body.ByOutcome["failure"] != 0 || body.BySource["interaction"] != 0 {
		t.Fatalf("stats leaked filtered experiences: %+v", body)
	}
}

func TestHandleExperienceStatsRespectsTagFilter(t *testing.T) {
	store := reflectpkg.NewExperienceStore(filepath.Join(t.TempDir(), "experiences.json"))
	store.Add(reflectpkg.Experience{ID: "ok", Source: "task", Category: "strategy", Outcome: "success", Lesson: "ok lesson", Tags: []string{"quality:9"}})
	store.Add(reflectpkg.Experience{ID: "partial", Source: "task", Category: "strategy", Outcome: "partial", Lesson: "partial lesson", Tags: []string{"quality:5"}})

	g := &Gateway{experienceStore: store}
	req := httptest.NewRequest(http.MethodGet, "/v1/reflect/experiences?stats=true&tag=quality:9", nil)
	rec := httptest.NewRecorder()

	g.handleExperiences(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var body reflectpkg.ExperienceStats
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Total != 1 || body.ByOutcome["success"] != 1 {
		t.Fatalf("unexpected tag-filtered stats: %+v", body)
	}
	if body.ByOutcome["partial"] != 0 {
		t.Fatalf("stats leaked non-matching tag: %+v", body)
	}
}

func TestHandleExperienceStatsReturnsWorkloadFeedbackDogfoodMetrics(t *testing.T) {
	store := reflectpkg.NewExperienceStore(filepath.Join(t.TempDir(), "experiences.json"))
	store.Add(reflectpkg.Experience{
		ID:       "browser-yes",
		Source:   "workload_feedback",
		SourceID: "browser-rpa",
		Category: "workload_feedback",
		Outcome:  "success",
		Lesson:   "工作负载【浏览器 / RPA】体验反馈\n30 秒找到入口：是\n最顺手：录制回放",
		Context:  "真实场景：网页资料收集",
		Tags:     []string{"workload:browser-rpa", "findability:yes"},
	})
	store.Add(reflectpkg.Experience{
		ID:       "memory-no",
		Source:   "workload_feedback",
		SourceID: "memory-review",
		Category: "workload_feedback",
		Outcome:  "failure",
		Lesson:   "工作负载【记忆 / 回溯】体验反馈\n30 秒找到入口：不能\n最不顺手：入口藏太深",
		Context:  "真实场景：复盘",
		Tags:     []string{"workload:memory-review", "findability:no"},
	})
	store.Add(reflectpkg.Experience{ID: "task", Source: "task", Category: "strategy", Outcome: "success"})

	g := &Gateway{experienceStore: store}
	req := httptest.NewRequest(http.MethodGet, "/v1/reflect/experiences?stats=true&kind=workload_feedback&source=workload_feedback&category=workload_feedback&workloads=browser-rpa,memory-review,wasm-workload", nil)
	rec := httptest.NewRecorder()

	g.handleExperiences(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var body reflectpkg.WorkloadFeedbackStats
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Total != 2 || body.Workloads != 3 || body.ByWorkload["wasm-workload"] != 0 {
		t.Fatalf("unexpected workload dogfood stats: %+v", body)
	}
	if body.Findability["yes"] != 1 || body.Findability["no"] != 1 {
		t.Fatalf("unexpected findability stats: %+v", body.Findability)
	}
	if body.Filled != 2 || body.FillRate != 1 {
		t.Fatalf("unexpected fill stats: %+v", body)
	}
}
