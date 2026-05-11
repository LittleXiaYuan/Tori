package gateway

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	reflectpkg "yunque-agent/internal/experimental/reflect"
)

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
