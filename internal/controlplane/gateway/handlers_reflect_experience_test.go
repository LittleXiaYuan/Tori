package gateway

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
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
