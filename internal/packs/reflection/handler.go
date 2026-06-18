package reflectionpack

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"yunque-agent/internal/apperror"
	"yunque-agent/internal/cognikernel"
	reflectpkg "yunque-agent/internal/experimental/reflect"
	"yunque-agent/pkg/packruntime"
)

const PackID = "yunque.pack.reflection"

type Gateway interface {
	ExperienceStore() *reflectpkg.ExperienceStore
	ReflectiveLoop() *cognikernel.ReflectiveLoop
}

type Handler struct {
	storeOf func() *reflectpkg.ExperienceStore
	loopOf  func() *cognikernel.ReflectiveLoop
	host    packruntime.Host
	started atomic.Bool
}

func New(gateway Gateway) *Handler {
	if gateway == nil {
		return NewProvider(nil, nil)
	}
	return NewProvider(gateway.ExperienceStore, gateway.ReflectiveLoop)
}

func NewProvider(store func() *reflectpkg.ExperienceStore, loop func() *cognikernel.ReflectiveLoop) *Handler {
	return &Handler{storeOf: store, loopOf: loop}
}

func (h *Handler) PackID() string { return PackID }

var _ packruntime.Module = (*Handler)(nil)

func (h *Handler) Init(host packruntime.Host) error {
	h.host = host
	return nil
}

func (h *Handler) Start(ctx context.Context) error {
	h.started.Store(true)
	if h.host != nil {
		h.host.Logger().Info("reflection pack started", "pack", PackID)
	}
	return nil
}

func (h *Handler) Stop(ctx context.Context) error {
	h.started.Store(false)
	return nil
}

func (h *Handler) Routes() []packruntime.BackendRoute {
	return []packruntime.BackendRoute{
		{Methods: []string{http.MethodGet, http.MethodPost}, Path: "/v1/reflect/experiences", Handler: h.Experiences},
		{Method: http.MethodGet, Path: "/v1/reflect/strategies", Handler: h.Strategies},
	}
}

func RouteSpecs() []packruntime.BackendRouteSpec {
	return []packruntime.BackendRouteSpec{
		{Method: http.MethodGet, Path: "/v1/reflect/experiences", Description: "List, search, or summarize learned experiences."},
		{Method: http.MethodPost, Path: "/v1/reflect/experiences", Description: "Record one learned experience or workload feedback item."},
		{Method: http.MethodGet, Path: "/v1/reflect/strategies", Description: "Compile learned experiences into strategy hints."},
	}
}

func Paths() []string {
	return []string{"/v1/reflect/experiences", "/v1/reflect/strategies"}
}

func (h *Handler) store() *reflectpkg.ExperienceStore {
	if h.storeOf == nil {
		return nil
	}
	return h.storeOf()
}

func (h *Handler) loop() *cognikernel.ReflectiveLoop {
	if h.loopOf == nil {
		return nil
	}
	return h.loopOf()
}

func (h *Handler) Experiences(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "GET or POST only")
		return
	}
	store := h.store()
	if store == nil {
		apperror.WriteCode(w, apperror.CodeNotFound, "experience store not initialized")
		return
	}

	if r.Method == http.MethodPost {
		h.createExperience(w, r, store)
		return
	}

	source := r.URL.Query().Get("source")
	category := r.URL.Query().Get("category")
	outcome := r.URL.Query().Get("outcome")
	tag := r.URL.Query().Get("tag")
	limit := experienceLimit(r, 0)

	if r.URL.Query().Get("stats") == "true" {
		filtered := filterExperiences(store.All(), source, category, outcome, tag)
		if r.URL.Query().Get("kind") == "workload_feedback" {
			writeJSON(w, reflectpkg.SummarizeWorkloadFeedback(filtered, workloadIDsFromQuery(r)))
			return
		}
		writeJSON(w, summarizeExperiences(filtered))
		return
	}

	query := r.URL.Query().Get("q")
	if query != "" {
		results := limitExperiences(filterExperiences(queryExperiences(store.All(), query), source, category, outcome, tag), limit)
		writeJSON(w, map[string]any{"experiences": results, "total": len(results)})
		return
	}

	all := store.All()
	if source != "" || category != "" || outcome != "" || tag != "" {
		filtered := limitExperiences(filterExperiences(all, source, category, outcome, tag), limit)
		writeJSON(w, map[string]any{"experiences": filtered, "total": len(filtered)})
		return
	}

	all = limitExperiences(all, limit)
	writeJSON(w, map[string]any{"experiences": all, "total": len(all)})
}

func (h *Handler) createExperience(w http.ResponseWriter, r *http.Request, store *reflectpkg.ExperienceStore) {
	var req struct {
		Experience *reflectpkg.Experience `json:"experience"`
		ID         string                 `json:"id"`
		Source     string                 `json:"source"`
		SourceID   string                 `json:"source_id"`
		Category   string                 `json:"category"`
		Outcome    string                 `json:"outcome"`
		Lesson     string                 `json:"lesson"`
		Context    string                 `json:"context"`
		Tags       []string               `json:"tags"`
		CreatedAt  time.Time              `json:"created_at"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apperror.WriteCode(w, apperror.CodeBadRequest, "invalid JSON body")
		return
	}
	exp := reflectpkg.Experience{
		ID:        req.ID,
		Source:    req.Source,
		SourceID:  req.SourceID,
		Category:  req.Category,
		Outcome:   req.Outcome,
		Lesson:    req.Lesson,
		Context:   req.Context,
		Tags:      req.Tags,
		CreatedAt: req.CreatedAt,
	}
	if req.Experience != nil {
		exp = *req.Experience
	}
	exp.Source = strings.TrimSpace(exp.Source)
	exp.Category = strings.TrimSpace(exp.Category)
	exp.Outcome = strings.TrimSpace(exp.Outcome)
	exp.Lesson = strings.TrimSpace(exp.Lesson)
	exp.Context = strings.TrimSpace(exp.Context)
	exp.SourceID = strings.TrimSpace(exp.SourceID)
	if exp.Source == "" {
		exp.Source = "interaction"
	}
	if exp.Category == "" {
		exp.Category = "workload_feedback"
	}
	if exp.Outcome == "" {
		exp.Outcome = "partial"
	}
	if exp.Lesson == "" {
		apperror.WriteCode(w, apperror.CodeBadRequest, "experience lesson is required")
		return
	}
	if isReflectiveLoopFeedback(exp) {
		if loop := h.loop(); loop != nil {
			if result, err := loop.IngestFeedback(r.Context(), cognikernel.FeedbackData{
				Source:    exp.Source,
				SourceID:  exp.SourceID,
				Category:  exp.Category,
				Outcome:   exp.Outcome,
				Lesson:    exp.Lesson,
				Context:   exp.Context,
				Tags:      exp.Tags,
				CreatedAt: exp.CreatedAt,
			}); err == nil && result != nil && result.ExperiencesAdded > 0 {
				writeJSON(w, map[string]any{"experience": exp, "status": "stored", "ingested_by": "reflective_loop"})
				return
			} else if err != nil {
				slog.Warn("reflective loop feedback ingestion failed; storing directly", "err", err)
			}
		}
	}

	store.Add(exp)
	writeJSON(w, map[string]any{"experience": exp, "status": "stored"})
}

func (h *Handler) Strategies(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		apperror.WriteCode(w, apperror.CodeMethodNotAllow, "GET only")
		return
	}
	store := h.store()
	if store == nil {
		apperror.WriteCode(w, apperror.CodeNotFound, "experience store not initialized")
		return
	}

	limit := experienceLimit(r, 20)
	source := r.URL.Query().Get("source")
	category := r.URL.Query().Get("category")
	outcome := r.URL.Query().Get("outcome")
	tag := r.URL.Query().Get("tag")
	query := r.URL.Query().Get("q")

	strategies := ""
	if source != "" || category != "" || outcome != "" || tag != "" || query != "" {
		experiences := store.All()
		if query != "" {
			experiences = queryExperiences(experiences, query)
		}
		strategies = reflectpkg.CompileStrategiesFrom(filterExperiences(experiences, source, category, outcome, tag), limit)
	} else {
		strategies = store.CompileStrategies(limit)
	}
	writeJSON(w, map[string]any{"strategies": strategies})
}

func isReflectiveLoopFeedback(exp reflectpkg.Experience) bool {
	return exp.Source == "workload_feedback" || exp.Category == "workload_feedback"
}

func experienceLimit(r *http.Request, fallback int) int {
	raw := r.URL.Query().Get("limit")
	if raw == "" {
		return fallback
	}
	limit, err := strconv.Atoi(raw)
	if err != nil || limit <= 0 {
		return fallback
	}
	if limit > 200 {
		return 200
	}
	return limit
}

func workloadIDsFromQuery(r *http.Request) []string {
	raw := r.URL.Query().Get("workloads")
	if raw == "" {
		return nil
	}
	parts := strings.FieldsFunc(raw, func(r rune) bool {
		return r == ',' || r == '\n' || r == ' '
	})
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func filterExperiences(experiences []reflectpkg.Experience, source, category, outcome, tag string) []reflectpkg.Experience {
	if source == "" && category == "" && outcome == "" && tag == "" {
		return experiences
	}
	filtered := make([]reflectpkg.Experience, 0, len(experiences))
	for _, e := range experiences {
		if source != "" && e.Source != source {
			continue
		}
		if category != "" && e.Category != category {
			continue
		}
		if outcome != "" && e.Outcome != outcome {
			continue
		}
		if tag != "" && !experienceHasTag(e, tag) {
			continue
		}
		filtered = append(filtered, e)
	}
	return filtered
}

func experienceHasTag(e reflectpkg.Experience, tag string) bool {
	for _, t := range e.Tags {
		if t == tag {
			return true
		}
	}
	return false
}

func queryExperiences(experiences []reflectpkg.Experience, query string) []reflectpkg.Experience {
	filtered := make([]reflectpkg.Experience, 0, len(experiences))
	for _, e := range experiences {
		if reflectpkg.MatchesQuery(e, query) {
			filtered = append(filtered, e)
		}
	}
	return filtered
}

func limitExperiences(experiences []reflectpkg.Experience, limit int) []reflectpkg.Experience {
	if limit <= 0 || len(experiences) <= limit {
		return experiences
	}
	return experiences[:limit]
}

func summarizeExperiences(experiences []reflectpkg.Experience) reflectpkg.ExperienceStats {
	st := reflectpkg.ExperienceStats{
		Total:      len(experiences),
		BySource:   make(map[string]int),
		ByCategory: make(map[string]int),
		ByOutcome:  make(map[string]int),
	}
	week := time.Now().Add(-7 * 24 * time.Hour)
	for _, e := range experiences {
		st.BySource[e.Source]++
		st.ByCategory[e.Category]++
		st.ByOutcome[e.Outcome]++
		if e.CreatedAt.After(week) {
			st.Recent++
		}
	}
	return st
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}
