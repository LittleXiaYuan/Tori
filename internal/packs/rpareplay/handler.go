// Package rpareplay contains the backend implementation for the built-in RPA
// replay capability pack. The first delivery is intentionally a pack shell: it
// owns manifest-gated HTTP routes, trace metadata storage, dry-run replay plans,
// and evidence export while the browser/computer-use executor is wired later.
package rpareplay

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"yunque-agent/pkg/packruntime"
)

const PackID = "yunque.pack.rpa-replay"

// Config describes runtime dependencies for the RPA replay pack shell.
type Config struct {
	DataDir string
	Now     func() time.Time
}

// Handler serves the RPA record/replay pack API surface.
type Handler struct {
	dataDir    string
	now        func() time.Time
	sessionsMu sync.Mutex
	sessions   map[string]RecordingSession
}

// New creates an RPA replay pack handler.
func New(cfg Config) *Handler {
	dataDir := cfg.DataDir
	if strings.TrimSpace(dataDir) == "" {
		dataDir = filepath.Join(".", "data", "rpa-replay")
	}
	now := cfg.Now
	if now == nil {
		now = time.Now
	}
	return &Handler{dataDir: dataDir, now: now, sessions: make(map[string]RecordingSession)}
}

// DefaultHandler returns a handler bound to the default local data directory.
func DefaultHandler() *Handler { return New(Config{}) }

// PackID returns the stable manifest id for the built-in RPA replay pack.
func (h *Handler) PackID() string { return PackID }

// Routes exposes the RPA replay shell HTTP API to the Pack Runtime host.
func (h *Handler) Routes() []packruntime.BackendRoute {
	return []packruntime.BackendRoute{
		{Method: http.MethodGet, Path: "/v1/rpa-replay/status", Handler: h.Status},
		{Methods: []string{http.MethodGet, http.MethodPost}, Path: "/v1/rpa-replay/traces", Handler: h.Traces},
		{Method: http.MethodGet, Path: "/v1/rpa-replay/traces/", Handler: h.TraceDetail},
		{Method: http.MethodPost, Path: "/v1/rpa-replay/recordings/start", Handler: h.StartRecording},
		{Method: http.MethodPost, Path: "/v1/rpa-replay/recordings/stop", Handler: h.StopRecording},
		{Method: http.MethodPost, Path: "/v1/rpa-replay/replay", Handler: h.Replay},
		{Method: http.MethodGet, Path: "/v1/rpa-replay/evidence/", Handler: h.Evidence},
	}
}

type ParamDef struct {
	Type        string `json:"type,omitempty"`
	Description string `json:"description,omitempty"`
	Required    bool   `json:"required,omitempty"`
	Default     string `json:"default,omitempty"`
}

type StepAssertion struct {
	Type     string `json:"type"`
	Selector string `json:"selector,omitempty"`
	Expected string `json:"expected,omitempty"`
}

type TraceStep struct {
	Index       int            `json:"index"`
	Action      string         `json:"action"`
	Selector    string         `json:"selector,omitempty"`
	Value       string         `json:"value,omitempty"`
	ParamRef    string         `json:"param_ref,omitempty"`
	Screenshot  string         `json:"screenshot,omitempty"`
	Assertion   *StepAssertion `json:"assertion,omitempty"`
	TimestampMS int64          `json:"timestamp_ms,omitempty"`
}

type Trace struct {
	Slug          string              `json:"slug"`
	Name          string              `json:"name"`
	Description   string              `json:"description,omitempty"`
	Type          string              `json:"type"`
	Parameters    map[string]ParamDef `json:"parameters,omitempty"`
	TargetURL     string              `json:"target_url,omitempty"`
	RecordedAt    time.Time           `json:"recorded_at"`
	SuccessRate   float64             `json:"success_rate,omitempty"`
	AvgDurationMS int64               `json:"avg_duration_ms,omitempty"`
	Steps         []TraceStep         `json:"steps"`
}

type TraceSummary struct {
	Slug          string    `json:"slug"`
	Name          string    `json:"name"`
	Description   string    `json:"description,omitempty"`
	TargetURL     string    `json:"target_url,omitempty"`
	RecordedAt    time.Time `json:"recorded_at"`
	StepCount     int       `json:"step_count"`
	SuccessRate   float64   `json:"success_rate,omitempty"`
	AvgDurationMS int64     `json:"avg_duration_ms,omitempty"`
}

type RecordingSession struct {
	ID          string              `json:"id"`
	Slug        string              `json:"slug,omitempty"`
	Name        string              `json:"name,omitempty"`
	Description string              `json:"description,omitempty"`
	TargetURL   string              `json:"target_url,omitempty"`
	Parameters  map[string]ParamDef `json:"parameters,omitempty"`
	StartedAt   time.Time           `json:"started_at"`
	Status      string              `json:"status"`
}

type ReplayResult struct {
	Success      bool        `json:"success"`
	DryRun       bool        `json:"dry_run"`
	Output       string      `json:"output,omitempty"`
	StepsRun     int         `json:"steps_run"`
	FailedStep   int         `json:"failed_step"`
	FailReason   string      `json:"fail_reason,omitempty"`
	DurationMS   int64       `json:"duration_ms"`
	PlannedSteps []TraceStep `json:"planned_steps,omitempty"`
}

var safeSlugRe = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,79}$`)

func (h *Handler) Status(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	traces, err := h.listTraces()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	h.sessionsMu.Lock()
	active := len(h.sessions)
	h.sessionsMu.Unlock()
	writeJSON(w, http.StatusOK, map[string]any{
		"pack_id":           PackID,
		"stage":             "pack-shell",
		"executor_ready":    false,
		"trace_count":       len(traces),
		"active_recordings": active,
		"store_dir":         h.dataDir,
		"capabilities": []string{
			"rpa.trace.store",
			"rpa.recording.session",
			"rpa.replay.dry_run",
			"rpa.evidence.export",
		},
	})
}

func (h *Handler) Traces(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		traces, err := h.listTraces()
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"traces": traces, "count": len(traces)})
	case http.MethodPost:
		var req Trace
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid trace payload")
			return
		}
		trace, err := h.normalizeTrace(req)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		if err := h.saveTrace(trace); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, map[string]any{"trace": trace, "status": "created"})
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (h *Handler) TraceDetail(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	slug := strings.TrimPrefix(r.URL.Path, "/v1/rpa-replay/traces/")
	trace, err := h.loadTrace(slug)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"trace": trace})
}

func (h *Handler) StartRecording(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var req RecordingSession
	_ = json.NewDecoder(r.Body).Decode(&req)
	now := h.now().UTC()
	session := RecordingSession{
		ID:          fmt.Sprintf("rec-%d", now.UnixNano()),
		Slug:        strings.TrimSpace(req.Slug),
		Name:        strings.TrimSpace(req.Name),
		Description: strings.TrimSpace(req.Description),
		TargetURL:   strings.TrimSpace(req.TargetURL),
		Parameters:  req.Parameters,
		StartedAt:   now,
		Status:      "recording",
	}
	if session.Slug != "" && !safeSlug(session.Slug) {
		writeError(w, http.StatusBadRequest, "slug must match ^[a-z0-9][a-z0-9_-]{0,79}$")
		return
	}
	h.sessionsMu.Lock()
	h.sessions[session.ID] = session
	h.sessionsMu.Unlock()
	writeJSON(w, http.StatusAccepted, map[string]any{
		"session": session,
		"status":  "recording",
		"note":    "ActionTracer hooks are not connected yet; stop with explicit steps or create traces directly.",
	})
}

func (h *Handler) StopRecording(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var req struct {
		SessionID string      `json:"session_id"`
		Slug      string      `json:"slug,omitempty"`
		Name      string      `json:"name,omitempty"`
		Steps     []TraceStep `json:"steps,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || strings.TrimSpace(req.SessionID) == "" {
		writeError(w, http.StatusBadRequest, "session_id is required")
		return
	}
	h.sessionsMu.Lock()
	session, ok := h.sessions[req.SessionID]
	if ok {
		delete(h.sessions, req.SessionID)
	}
	h.sessionsMu.Unlock()
	if !ok {
		writeError(w, http.StatusNotFound, "recording session not found")
		return
	}
	trace := Trace{
		Slug:        firstNonEmpty(req.Slug, session.Slug, slugify(firstNonEmpty(req.Name, session.Name, session.ID))),
		Name:        firstNonEmpty(req.Name, session.Name, session.ID),
		Description: session.Description,
		Type:        "rpa-replay",
		Parameters:  session.Parameters,
		TargetURL:   session.TargetURL,
		RecordedAt:  h.now().UTC(),
		Steps:       req.Steps,
	}
	trace, err := h.normalizeTrace(trace)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := h.saveTrace(trace); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"trace": trace, "status": "recorded"})
}

func (h *Handler) Replay(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	started := h.now()
	var req struct {
		Slug   string            `json:"slug"`
		Params map[string]string `json:"params,omitempty"`
		DryRun *bool             `json:"dry_run,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || strings.TrimSpace(req.Slug) == "" {
		writeError(w, http.StatusBadRequest, "slug is required")
		return
	}
	trace, err := h.loadTrace(req.Slug)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	planned := substituteSteps(trace.Steps, req.Params)
	dryRun := req.DryRun == nil || *req.DryRun
	result := ReplayResult{
		Success:      dryRun,
		DryRun:       dryRun,
		Output:       "dry-run replay plan generated",
		StepsRun:     len(planned),
		FailedStep:   -1,
		DurationMS:   int64(h.now().Sub(started) / time.Millisecond),
		PlannedSteps: planned,
	}
	if !dryRun {
		result.Output = ""
		result.FailReason = "replay executor is not connected yet; dry-run plan returned for Pack Runtime shell"
		result.FailedStep = 0
	}
	writeJSON(w, http.StatusOK, map[string]any{"result": result, "trace": trace.Slug})
}

func (h *Handler) Evidence(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	slug := strings.TrimPrefix(r.URL.Path, "/v1/rpa-replay/evidence/")
	trace, err := h.loadTrace(slug)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"pack_id":     PackID,
		"exported_at": h.now().UTC(),
		"format":      "json-evidence-pack",
		"files":       []string{"meta.json", "trace.json"},
		"trace":       trace,
	})
}

func (h *Handler) normalizeTrace(trace Trace) (Trace, error) {
	trace.Slug = strings.TrimSpace(trace.Slug)
	if trace.Slug == "" {
		trace.Slug = slugify(trace.Name)
	}
	if !safeSlug(trace.Slug) {
		return Trace{}, fmt.Errorf("slug must match ^[a-z0-9][a-z0-9_-]{0,79}$")
	}
	trace.Name = strings.TrimSpace(firstNonEmpty(trace.Name, trace.Slug))
	trace.Type = "rpa-replay"
	if trace.RecordedAt.IsZero() {
		trace.RecordedAt = h.now().UTC()
	}
	for i := range trace.Steps {
		if trace.Steps[i].Index == 0 {
			trace.Steps[i].Index = i + 1
		}
		trace.Steps[i].Action = strings.TrimSpace(trace.Steps[i].Action)
	}
	return trace, nil
}

func (h *Handler) traceRoot() string { return filepath.Join(h.dataDir, "traces") }

func (h *Handler) traceDir(slug string) (string, error) {
	if !safeSlug(slug) {
		return "", fmt.Errorf("invalid trace slug")
	}
	return filepath.Join(h.traceRoot(), slug), nil
}

func (h *Handler) saveTrace(trace Trace) error {
	dir, err := h.traceDir(trace.Slug)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create trace dir: %w", err)
	}
	data, err := json.MarshalIndent(trace, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal trace: %w", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "trace.json"), append(data, '\n'), 0o644); err != nil {
		return fmt.Errorf("write trace: %w", err)
	}
	meta, err := json.MarshalIndent(toSummary(trace), "", "  ")
	if err != nil {
		return fmt.Errorf("marshal trace meta: %w", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "meta.json"), append(meta, '\n'), 0o644); err != nil {
		return fmt.Errorf("write trace meta: %w", err)
	}
	return nil
}

func (h *Handler) loadTrace(slug string) (Trace, error) {
	slug = strings.Trim(strings.TrimSpace(slug), "/")
	dir, err := h.traceDir(slug)
	if err != nil {
		return Trace{}, err
	}
	data, err := os.ReadFile(filepath.Join(dir, "trace.json"))
	if err != nil {
		return Trace{}, fmt.Errorf("trace not found")
	}
	var trace Trace
	if err := json.Unmarshal(data, &trace); err != nil {
		return Trace{}, fmt.Errorf("invalid trace file")
	}
	return trace, nil
}

func (h *Handler) listTraces() ([]TraceSummary, error) {
	entries, err := os.ReadDir(h.traceRoot())
	if os.IsNotExist(err) {
		return []TraceSummary{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("list traces: %w", err)
	}
	summaries := make([]TraceSummary, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() || !safeSlug(entry.Name()) {
			continue
		}
		trace, err := h.loadTrace(entry.Name())
		if err != nil {
			continue
		}
		summaries = append(summaries, toSummary(trace))
	}
	sort.Slice(summaries, func(i, j int) bool {
		return summaries[i].RecordedAt.After(summaries[j].RecordedAt)
	})
	return summaries, nil
}

func toSummary(trace Trace) TraceSummary {
	return TraceSummary{
		Slug:          trace.Slug,
		Name:          trace.Name,
		Description:   trace.Description,
		TargetURL:     trace.TargetURL,
		RecordedAt:    trace.RecordedAt,
		StepCount:     len(trace.Steps),
		SuccessRate:   trace.SuccessRate,
		AvgDurationMS: trace.AvgDurationMS,
	}
}

func substituteSteps(steps []TraceStep, params map[string]string) []TraceStep {
	out := make([]TraceStep, len(steps))
	copy(out, steps)
	for i := range out {
		out[i].Selector = substitute(out[i].Selector, params)
		out[i].Value = substitute(out[i].Value, params)
		out[i].ParamRef = substitute(out[i].ParamRef, params)
		if out[i].Assertion != nil {
			assertion := *out[i].Assertion
			assertion.Selector = substitute(assertion.Selector, params)
			assertion.Expected = substitute(assertion.Expected, params)
			out[i].Assertion = &assertion
		}
	}
	return out
}

func substitute(text string, params map[string]string) string {
	if text == "" || len(params) == 0 {
		return text
	}
	for key, value := range params {
		text = strings.ReplaceAll(text, "{{"+key+"}}", value)
	}
	return text
}

func safeSlug(slug string) bool { return safeSlugRe.MatchString(slug) }

func slugify(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	var b strings.Builder
	lastDash := false
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			lastDash = false
		case r == '-' || r == '_' || r == ' ' || r == '.' || r == '/':
			if !lastDash && b.Len() > 0 {
				b.WriteByte('-')
				lastDash = true
			}
		}
	}
	out := strings.Trim(b.String(), "-")
	if out == "" {
		out = "trace"
	}
	if len(out) > 80 {
		out = out[:80]
		out = strings.Trim(out, "-")
	}
	return out
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]any{"error": message})
}
