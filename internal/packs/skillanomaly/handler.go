// Package skillanomaly contains the backend implementation for the built-in
// skill behavior anomaly capability pack. The first delivery is intentionally a
// pack shell: it owns manifest-gated HTTP routes, behavior-profile metadata,
// sliding-window anomaly scoring, dry-run detection, audit-hook/trust mutation
// plans, and evidence export while direct audit-chain hooks and Trust/Approval
// mutations are wired later.
package skillanomaly

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"yunque-agent/pkg/packruntime"
)

const PackID = "yunque.pack.skill-anomaly"

// Config describes runtime dependencies for the skill anomaly pack shell.
type Config struct {
	DataDir string
	Now     func() time.Time
	Policy  DetectionPolicy
}

// Handler serves skill behavior anomaly pack routes.
type Handler struct {
	dataDir string
	now     func() time.Time
	policy  DetectionPolicy
}

// DetectionPolicy contains conservative behavior-baseline thresholds.
type DetectionPolicy struct {
	WindowSize          int     `json:"window_size"`
	MinObservations     int     `json:"min_observations"`
	NewActionScore      float64 `json:"new_action_score"`
	NewParamScore       float64 `json:"new_param_score"`
	FailureBurstScore   float64 `json:"failure_burst_score"`
	DurationSpikeScore  float64 `json:"duration_spike_score"`
	NeedsApprovalScore  float64 `json:"needs_approval_score"`
	BlockScore          float64 `json:"block_score"`
	DurationSpikeFactor float64 `json:"duration_spike_factor"`
}

// Event is a normalized skill behavior observation.
type Event struct {
	ID         string    `json:"id"`
	SkillSlug  string    `json:"skill_slug"`
	Actor      string    `json:"actor,omitempty"`
	Action     string    `json:"action"`
	ParamKeys  []string  `json:"param_keys,omitempty"`
	Success    bool      `json:"success"`
	DurationMS int64     `json:"duration_ms,omitempty"`
	Timestamp  time.Time `json:"timestamp"`
}

type ObserveRequest struct {
	SkillSlug  string         `json:"skill_slug"`
	Actor      string         `json:"actor"`
	Action     string         `json:"action"`
	Params     map[string]any `json:"params"`
	ParamKeys  []string       `json:"param_keys"`
	Success    *bool          `json:"success"`
	DurationMS int64          `json:"duration_ms"`
	Timestamp  time.Time      `json:"timestamp"`
	DryRun     bool           `json:"dry_run"`
}

type DetectionRequest struct {
	SkillSlug  string         `json:"skill_slug"`
	Actor      string         `json:"actor"`
	Action     string         `json:"action"`
	Params     map[string]any `json:"params"`
	ParamKeys  []string       `json:"param_keys"`
	Success    *bool          `json:"success"`
	DurationMS int64          `json:"duration_ms"`
	DryRun     bool           `json:"dry_run"`
}

type AuditHookPlanRequest struct {
	SkillSlug   string         `json:"skill_slug"`
	Actor       string         `json:"actor"`
	Action      string         `json:"action"`
	Params      map[string]any `json:"params"`
	ParamKeys   []string       `json:"param_keys"`
	Success     *bool          `json:"success"`
	DurationMS  int64          `json:"duration_ms"`
	Reason      string         `json:"reason"`
	RequestedBy string         `json:"requested_by"`
	RequestID   string         `json:"request_id"`
	RequestKey  string         `json:"request_key"`
	DryRun      bool           `json:"dry_run"`
}

type Profile struct {
	SkillSlug      string             `json:"skill_slug"`
	WindowSize     int                `json:"window_size"`
	Observed       int                `json:"observed"`
	CallsPerMinute float64            `json:"calls_per_minute"`
	ActionDistrib  map[string]float64 `json:"action_distrib"`
	ParamKeySet    map[string]int     `json:"param_key_set"`
	SuccessRate    float64            `json:"success_rate"`
	AvgDurationMS  float64            `json:"avg_duration_ms"`
	LastAnomalyAt  time.Time          `json:"last_anomaly_at,omitempty"`
	AnomalyCount   int                `json:"anomaly_count"`
	UpdatedAt      time.Time          `json:"updated_at"`
	Recent         []Event            `json:"recent"`
}

type ProfileSummary struct {
	SkillSlug      string             `json:"skill_slug"`
	Observed       int                `json:"observed"`
	CallsPerMinute float64            `json:"calls_per_minute"`
	ActionDistrib  map[string]float64 `json:"action_distrib"`
	ParamKeySet    map[string]int     `json:"param_key_set"`
	SuccessRate    float64            `json:"success_rate"`
	AvgDurationMS  float64            `json:"avg_duration_ms"`
	LastAnomalyAt  time.Time          `json:"last_anomaly_at,omitempty"`
	AnomalyCount   int                `json:"anomaly_count"`
	UpdatedAt      time.Time          `json:"updated_at"`
}

type DetectionReason struct {
	Name     string  `json:"name"`
	Score    float64 `json:"score"`
	Severity string  `json:"severity"`
	Detail   string  `json:"detail,omitempty"`
}

type DetectionResult struct {
	SkillSlug     string            `json:"skill_slug"`
	Score         float64           `json:"score"`
	Severity      string            `json:"severity"`
	NeedsApproval bool              `json:"needs_approval"`
	Block         bool              `json:"block"`
	Reasons       []DetectionReason `json:"reasons,omitempty"`
	Profile       ProfileSummary    `json:"profile"`
	Event         Event             `json:"event"`
	Notes         []string          `json:"notes,omitempty"`
}

type AuditHookRecordPlan struct {
	EventType         string         `json:"event_type"`
	Action            string         `json:"action"`
	Subject           string         `json:"subject"`
	Severity          string         `json:"severity"`
	MerkleAppendReady bool           `json:"merkle_append_ready"`
	Payload           map[string]any `json:"payload"`
}

type TrustMutationPlan struct {
	TargetSkill        string `json:"target_skill"`
	Mutation           string `json:"mutation"`
	Delta              int    `json:"delta"`
	RecordFailureReady bool   `json:"record_failure_ready"`
	Reason             string `json:"reason"`
}

type ApprovalQueuePlan struct {
	Required            bool   `json:"required"`
	QueueName           string `json:"queue_name"`
	QueueWritebackReady bool   `json:"queue_writeback_ready"`
	WritesApprovalQueue bool   `json:"writes_approval_queue"`
	WritesQueueStore    bool   `json:"writes_queue_store"`
	RequestID           string `json:"request_id"`
	RequestKey          string `json:"request_key"`
	Status              string `json:"status"`
	RequestedBy         string `json:"requested_by,omitempty"`
	Reason              string `json:"reason,omitempty"`
	StoreArtifact       string `json:"store_artifact"`
}

type AuditHookPlanReport struct {
	PackID                 string              `json:"pack_id"`
	SkillSlug              string              `json:"skill_slug"`
	GeneratedAt            time.Time           `json:"generated_at"`
	DryRun                 bool                `json:"dry_run"`
	Status                 string              `json:"status"`
	ApprovalRequired       bool                `json:"approval_required"`
	AuditHookPlanReady     bool                `json:"audit_hook_plan_ready"`
	AuditHookReady         bool                `json:"audit_hook_ready"`
	TrustMutationPlanReady bool                `json:"trust_mutation_plan_ready"`
	TrustMutationReady     bool                `json:"trust_mutation_ready"`
	ApprovalWritebackReady bool                `json:"approval_writeback_ready"`
	Detection              DetectionResult     `json:"detection"`
	AuditRecord            AuditHookRecordPlan `json:"audit_record"`
	TrustMutation          TrustMutationPlan   `json:"trust_mutation"`
	ApprovalQueue          ApprovalQueuePlan   `json:"approval_queue"`
	Actions                []string            `json:"actions"`
	Notes                  []string            `json:"notes,omitempty"`
}

type ApprovalQueueStoreSummary struct {
	PackID                  string   `json:"pack_id"`
	QueueName               string   `json:"queue_name"`
	Store                   string   `json:"store"`
	StoreReady              bool     `json:"store_ready"`
	RecordCount             int      `json:"record_count"`
	Artifact                string   `json:"artifact"`
	WritesApprovalQueue     bool     `json:"writes_approval_queue"`
	WritesApprovalQueueFile bool     `json:"writes_approval_queue_file"`
	MerkleAppendReady       bool     `json:"merkle_append_ready"`
	TrustMutationReady      bool     `json:"trust_mutation_ready"`
	Notes                   []string `json:"notes,omitempty"`
}

type ApprovalQueueRecord struct {
	PackID                  string              `json:"pack_id"`
	QueueName               string              `json:"queue_name"`
	RequestID               string              `json:"request_id"`
	RequestKey              string              `json:"request_key"`
	SkillSlug               string              `json:"skill_slug"`
	Status                  string              `json:"status"`
	Severity                string              `json:"severity"`
	Score                   float64             `json:"score"`
	ApprovalRequired        bool                `json:"approval_required"`
	RequestedBy             string              `json:"requested_by,omitempty"`
	Reason                  string              `json:"reason,omitempty"`
	CreatedAt               time.Time           `json:"created_at"`
	UpdatedAt               time.Time           `json:"updated_at"`
	AuditHookPlanReady      bool                `json:"audit_hook_plan_ready"`
	AuditHookReady          bool                `json:"audit_hook_ready"`
	MerkleAppendReady       bool                `json:"merkle_append_ready"`
	TrustMutationPlanReady  bool                `json:"trust_mutation_plan_ready"`
	TrustMutationReady      bool                `json:"trust_mutation_ready"`
	ApprovalWritebackReady  bool                `json:"approval_writeback_ready"`
	WritesApprovalQueue     bool                `json:"writes_approval_queue"`
	WritesApprovalQueueFile bool                `json:"writes_approval_queue_file"`
	ActionAllowed           bool                `json:"action_allowed"`
	ExecutionBlocked        bool                `json:"execution_blocked"`
	Detection               DetectionResult     `json:"detection"`
	AuditRecord             AuditHookRecordPlan `json:"audit_record"`
	TrustMutation           TrustMutationPlan   `json:"trust_mutation"`
	ApprovalQueue           ApprovalQueuePlan   `json:"approval_queue"`
	StoreArtifact           string              `json:"store_artifact"`
	Artifacts               []string            `json:"artifacts"`
	Labels                  []string            `json:"labels"`
	Notes                   []string            `json:"notes,omitempty"`
}

type ApprovalQueueWritebackReport struct {
	PackID                  string                    `json:"pack_id"`
	GeneratedAt             time.Time                 `json:"generated_at"`
	Status                  string                    `json:"status"`
	ApprovalRequired        bool                      `json:"approval_required"`
	ApprovalWritebackReady  bool                      `json:"approval_writeback_ready"`
	WritesApprovalQueue     bool                      `json:"writes_approval_queue"`
	WritesApprovalQueueFile bool                      `json:"writes_approval_queue_file"`
	AuditHookPlanReady      bool                      `json:"audit_hook_plan_ready"`
	AuditHookReady          bool                      `json:"audit_hook_ready"`
	MerkleAppendReady       bool                      `json:"merkle_append_ready"`
	TrustMutationPlanReady  bool                      `json:"trust_mutation_plan_ready"`
	TrustMutationReady      bool                      `json:"trust_mutation_ready"`
	ActionAllowed           bool                      `json:"action_allowed"`
	ExecutionBlocked        bool                      `json:"execution_blocked"`
	RequestID               string                    `json:"request_id"`
	RequestKey              string                    `json:"request_key"`
	ApprovalQueueRecord     ApprovalQueueRecord       `json:"approval_queue_record"`
	ApprovalQueueStore      ApprovalQueueStoreSummary `json:"approval_queue_store"`
	PlanSummary             AuditHookPlanReport       `json:"plan_summary"`
	Artifacts               []string                  `json:"artifacts"`
	Actions                 []string                  `json:"actions"`
	Notes                   []string                  `json:"notes,omitempty"`
}

var safeSlugRe = regexp.MustCompile(`^[a-z0-9][a-z0-9_.-]{0,79}$`)

// New creates a skill anomaly pack handler.
func New(cfg Config) *Handler {
	dataDir := strings.TrimSpace(cfg.DataDir)
	if dataDir == "" {
		dataDir = filepath.Join(".", "data", "skill-anomaly")
	}
	now := cfg.Now
	if now == nil {
		now = time.Now
	}
	return &Handler{dataDir: dataDir, now: now, policy: normalizePolicy(cfg.Policy)}
}

// DefaultHandler returns a handler bound to the default local data directory.
func DefaultHandler() *Handler { return New(Config{}) }

// PackID returns the stable manifest id for the built-in skill anomaly pack.
func (h *Handler) PackID() string { return PackID }

// Routes exposes the skill anomaly HTTP API to the Pack Runtime host.
func (h *Handler) Routes() []packruntime.BackendRoute {
	return []packruntime.BackendRoute{
		{Method: http.MethodGet, Path: "/v1/skill-anomaly/status", Handler: h.Status},
		{Methods: []string{http.MethodGet, http.MethodPost}, Path: "/v1/skill-anomaly/events", Handler: h.Events},
		{Method: http.MethodGet, Path: "/v1/skill-anomaly/profiles", Handler: h.Profiles},
		{Method: http.MethodGet, Path: "/v1/skill-anomaly/profiles/", Handler: h.ProfileDetail},
		{Method: http.MethodPost, Path: "/v1/skill-anomaly/detect", Handler: h.Detect},
		{Method: http.MethodPost, Path: "/v1/skill-anomaly/audit-hook/plan", Handler: h.AuditHookPlan},
		{Method: http.MethodPost, Path: "/v1/skill-anomaly/approval-queue/writeback", Handler: h.ApprovalQueueWriteback},
		{Method: http.MethodGet, Path: "/v1/skill-anomaly/evidence/", Handler: h.Evidence},
	}
}

func (h *Handler) Status(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	profiles, err := h.listProfiles()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	active := 0
	anomalies := 0
	for _, profile := range profiles {
		if profile.Observed > 0 {
			active++
		}
		anomalies += profile.AnomalyCount
	}
	approvalQueueSummary := h.approvalQueueStoreSummary()
	writeJSON(w, http.StatusOK, map[string]any{
		"pack_id":                    PackID,
		"stage":                      "pack-shell-before-audit-hook",
		"detector_ready":             true,
		"audit_hook_plan_ready":      true,
		"audit_hook_ready":           false,
		"trust_mutation_plan_ready":  true,
		"trust_mutation_ready":       false,
		"approval_writeback_ready":   true,
		"approval_queue_store_ready": true,
		"profile_count":              len(profiles),
		"active_profiles":            active,
		"anomaly_count":              anomalies,
		"store_dir":                  h.dataDir,
		"policy":                     h.policy,
		"approval_queue_store":       approvalQueueSummary,
		"capabilities": []string{
			"skill.behavior.profile",
			"skill.anomaly.detect",
			"skill.needs_approval.plan",
			"skill.audit_hook.plan",
			"skill.trust_mutation.plan",
			"skill.approval_queue.writeback",
			"skill.evidence.export",
		},
		"notes": []string{"Audit-hook and Trust mutation plans are non-destructive; pack-local Approval queue write-back is available, while direct Merkle Chain append, Trust Score mutation, and runtime action release remain follow-up wiring."},
	})
}

func (h *Handler) Events(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		slug := strings.TrimSpace(r.URL.Query().Get("skill_slug"))
		limit := parseLimit(r.URL.Query().Get("limit"), 50)
		events, err := h.listEvents(slug, limit)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"events": events, "count": len(events)})
	case http.MethodPost:
		var req ObserveRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid skill anomaly event payload")
			return
		}
		event, err := h.eventFromObserve(req)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		profile, err := h.loadOrNewProfile(event.SkillSlug)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		result := h.score(profile, event)
		if !req.DryRun {
			profile = h.appendEvent(profile, event, result)
			if err := h.saveProfile(profile); err != nil {
				writeError(w, http.StatusInternalServerError, err.Error())
				return
			}
			if err := h.appendEventLog(event); err != nil {
				writeError(w, http.StatusInternalServerError, err.Error())
				return
			}
			result.Profile = summarize(profile)
		}
		writeJSON(w, http.StatusCreated, map[string]any{"event": event, "result": result, "status": observedStatus(req.DryRun)})
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (h *Handler) Profiles(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	profiles, err := h.listProfiles()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"profiles": profiles, "count": len(profiles)})
}

func (h *Handler) ProfileDetail(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	slug := strings.TrimPrefix(r.URL.Path, "/v1/skill-anomaly/profiles/")
	profile, err := h.loadProfile(slug)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"profile": profile})
}

func (h *Handler) Detect(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var req DetectionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid skill anomaly detection payload")
		return
	}
	event, err := h.eventFromDetection(req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	profile, err := h.loadOrNewProfile(event.SkillSlug)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	result := h.score(profile, event)
	if !req.DryRun {
		profile = h.appendEvent(profile, event, result)
		if err := h.saveProfile(profile); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if err := h.appendEventLog(event); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		result.Profile = summarize(profile)
	}
	writeJSON(w, http.StatusOK, map[string]any{"result": result})
}

func (h *Handler) AuditHookPlan(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var req AuditHookPlanRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid skill anomaly audit-hook plan payload")
		return
	}
	event, err := h.eventFromAuditHookPlan(req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	profile, err := h.loadOrNewProfile(event.SkillSlug)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	result := h.score(profile, event)
	plan := h.buildAuditHookPlan(result, req.RequestedBy, req.Reason, req.RequestID, req.RequestKey)
	writeJSON(w, http.StatusOK, map[string]any{"plan": plan})
}

func (h *Handler) ApprovalQueueWriteback(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var req AuditHookPlanRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid skill anomaly approval queue writeback payload")
		return
	}
	report, err := h.writeApprovalQueueRecord(req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]any{"writeback": report})
}

func (h *Handler) Evidence(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	slug := strings.TrimPrefix(r.URL.Path, "/v1/skill-anomaly/evidence/")
	profile, err := h.loadProfile(slug)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	events, _ := h.listEvents(profile.SkillSlug, h.policy.WindowSize)
	auditHookPlan := h.buildEvidenceAuditHookPlan(profile)
	queueRecord := h.approvalQueueRecordPreview(auditHookPlan)
	writeJSON(w, http.StatusOK, map[string]any{
		"pack_id":               PackID,
		"exported_at":           h.now().UTC(),
		"format":                "json-skill-anomaly-evidence",
		"files":                 []string{"profile.json", "recent-events.json", "detection-policy.json", "audit-hook-plan.json", "trust-mutation-plan.json", "approval-queue-plan.json", "approval-queue-store.json", "approval-queue-record.json"},
		"profile":               profile,
		"events":                events,
		"policy":                h.policy,
		"audit_hook_plan":       auditHookPlan,
		"trust_mutation_plan":   auditHookPlan.TrustMutation,
		"approval_queue_plan":   auditHookPlan.ApprovalQueue,
		"approval_queue_store":  h.approvalQueueStoreSummary(),
		"approval_queue_record": queueRecord,
	})
}

func normalizePolicy(policy DetectionPolicy) DetectionPolicy {
	if policy.WindowSize <= 0 {
		policy.WindowSize = 100
	}
	if policy.MinObservations <= 0 {
		policy.MinObservations = 20
	}
	if policy.NewActionScore <= 0 {
		policy.NewActionScore = 3
	}
	if policy.NewParamScore <= 0 {
		policy.NewParamScore = 4
	}
	if policy.FailureBurstScore <= 0 {
		policy.FailureBurstScore = 4
	}
	if policy.DurationSpikeScore <= 0 {
		policy.DurationSpikeScore = 1
	}
	if policy.NeedsApprovalScore <= 0 {
		policy.NeedsApprovalScore = 3
	}
	if policy.BlockScore <= 0 {
		policy.BlockScore = 7
	}
	if policy.DurationSpikeFactor <= 0 {
		policy.DurationSpikeFactor = 3
	}
	return policy
}

func (h *Handler) eventFromObserve(req ObserveRequest) (Event, error) {
	return h.normalizeEvent(req.SkillSlug, req.Actor, req.Action, req.Params, req.ParamKeys, req.Success, req.DurationMS, req.Timestamp)
}

func (h *Handler) eventFromDetection(req DetectionRequest) (Event, error) {
	return h.normalizeEvent(req.SkillSlug, req.Actor, req.Action, req.Params, req.ParamKeys, req.Success, req.DurationMS, time.Time{})
}

func (h *Handler) eventFromAuditHookPlan(req AuditHookPlanRequest) (Event, error) {
	return h.normalizeEvent(req.SkillSlug, req.Actor, req.Action, req.Params, req.ParamKeys, req.Success, req.DurationMS, time.Time{})
}

func (h *Handler) normalizeEvent(skillSlug, actor, action string, params map[string]any, paramKeys []string, success *bool, durationMS int64, timestamp time.Time) (Event, error) {
	slug := normalizeSlug(skillSlug)
	if !safeSlugRe.MatchString(slug) {
		return Event{}, fmt.Errorf("skill_slug must match ^[a-z0-9][a-z0-9_.-]{0,79}$")
	}
	action = strings.TrimSpace(action)
	if action == "" {
		return Event{}, fmt.Errorf("action is required")
	}
	ok := true
	if success != nil {
		ok = *success
	}
	if timestamp.IsZero() {
		timestamp = h.now().UTC()
	}
	keys := normalizeParamKeys(params, paramKeys)
	return Event{
		ID:         fmt.Sprintf("%s-%d", slug, timestamp.UnixNano()),
		SkillSlug:  slug,
		Actor:      strings.TrimSpace(actor),
		Action:     action,
		ParamKeys:  keys,
		Success:    ok,
		DurationMS: durationMS,
		Timestamp:  timestamp.UTC(),
	}, nil
}

func normalizeSlug(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.ReplaceAll(value, " ", "-")
	return value
}

func normalizeParamKeys(params map[string]any, explicit []string) []string {
	keys := make([]string, 0, len(params)+len(explicit))
	seen := map[string]bool{}
	for _, key := range explicit {
		key = strings.TrimSpace(key)
		if key != "" && !seen[key] {
			seen[key] = true
			keys = append(keys, key)
		}
	}
	for key := range params {
		key = strings.TrimSpace(key)
		if key != "" && !seen[key] {
			seen[key] = true
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	return keys
}

func (h *Handler) score(profile Profile, event Event) DetectionResult {
	result := DetectionResult{SkillSlug: event.SkillSlug, Event: event, Profile: summarize(profile)}
	if profile.Observed < h.policy.MinObservations {
		result.Severity = "learning"
		result.Notes = []string{fmt.Sprintf("collecting baseline: %d/%d observations", profile.Observed, h.policy.MinObservations)}
		return result
	}

	if profile.ActionDistrib[event.Action] == 0 {
		result.Score += h.policy.NewActionScore
		result.Reasons = append(result.Reasons, DetectionReason{Name: "new_action", Score: h.policy.NewActionScore, Severity: "medium", Detail: fmt.Sprintf("action %q has not appeared in the baseline", event.Action)})
	}
	var newKeys []string
	for _, key := range event.ParamKeys {
		if profile.ParamKeySet[key] == 0 {
			newKeys = append(newKeys, key)
		}
	}
	if len(newKeys) > 0 {
		result.Score += h.policy.NewParamScore
		result.Reasons = append(result.Reasons, DetectionReason{Name: "new_param_keys", Score: h.policy.NewParamScore, Severity: "high", Detail: strings.Join(newKeys, ",")})
	}
	if !event.Success && consecutiveFailures(profile.Recent) >= 2 {
		result.Score += h.policy.FailureBurstScore
		result.Reasons = append(result.Reasons, DetectionReason{Name: "failure_burst", Score: h.policy.FailureBurstScore, Severity: "high", Detail: "candidate event extends a recent failure burst"})
	}
	if event.DurationMS > 0 && profile.AvgDurationMS > 0 && float64(event.DurationMS) > profile.AvgDurationMS*h.policy.DurationSpikeFactor {
		result.Score += h.policy.DurationSpikeScore
		result.Reasons = append(result.Reasons, DetectionReason{Name: "duration_spike", Score: h.policy.DurationSpikeScore, Severity: "low", Detail: fmt.Sprintf("duration %dms > %.1fx avg %.1fms", event.DurationMS, h.policy.DurationSpikeFactor, profile.AvgDurationMS)})
	}

	result.Severity = severityForScore(result.Score, h.policy)
	result.NeedsApproval = result.Score >= h.policy.NeedsApprovalScore
	result.Block = result.Score >= h.policy.BlockScore
	if result.Score == 0 {
		result.Notes = []string{"behavior matches current baseline"}
	}
	return result
}

func (h *Handler) buildEvidenceAuditHookPlan(profile Profile) AuditHookPlanReport {
	if len(profile.Recent) == 0 {
		event := Event{SkillSlug: profile.SkillSlug, Action: "no_recent_event", Success: true, Timestamp: h.now().UTC()}
		return h.buildAuditHookPlan(DetectionResult{
			SkillSlug: profile.SkillSlug,
			Severity:  "normal",
			Profile:   summarize(profile),
			Event:     event,
			Notes:     []string{"No recent skill behavior event is available; evidence carries the non-destructive audit hook plan contract only."},
		}, "evidence-export", "evidence export schema snapshot", "", "")
	}
	event := profile.Recent[len(profile.Recent)-1]
	return h.buildAuditHookPlan(h.score(profile, event), "evidence-export", "latest profile event schema snapshot", "", "")
}

func (h *Handler) buildAuditHookPlan(result DetectionResult, requestedBy, reason, requestID, requestKey string) AuditHookPlanReport {
	approvalRequired := result.NeedsApproval || result.Block
	status := "no_op"
	if result.Severity == "learning" {
		status = "learning"
	}
	if approvalRequired {
		status = "approval_plan"
	}
	delta := trustDelta(result)
	requestedBy = strings.TrimSpace(requestedBy)
	if requestedBy == "" {
		requestedBy = "operator"
	}
	queueReason := approvalReason(result, reason)
	requestID, requestKey = approvalQueueIdentifiers(result, requestedBy, queueReason, requestID, requestKey)
	actions := []string{}
	if approvalRequired {
		actions = append(actions,
			fmt.Sprintf("would append Merkle audit record skill_anomaly_%s for %s", result.Severity, result.SkillSlug),
			fmt.Sprintf("would request Trust Score mutation %+d for %s", delta, result.SkillSlug),
			fmt.Sprintf("would enqueue Approval queue item for %s before allowing the anomalous action", result.SkillSlug),
		)
	} else {
		actions = append(actions,
			"no Merkle audit append required for the current detection result",
			"no Trust Score mutation required for the current detection result",
			"no Approval queue write-back required for the current detection result",
		)
	}
	payload := map[string]any{
		"skill_slug":     result.SkillSlug,
		"score":          result.Score,
		"severity":       result.Severity,
		"needs_approval": result.NeedsApproval,
		"block":          result.Block,
		"reason_count":   len(result.Reasons),
	}
	if result.Event.ID != "" {
		payload["event_id"] = result.Event.ID
	}
	return AuditHookPlanReport{
		PackID:                 PackID,
		SkillSlug:              result.SkillSlug,
		GeneratedAt:            h.now().UTC(),
		DryRun:                 true,
		Status:                 status,
		ApprovalRequired:       approvalRequired,
		AuditHookPlanReady:     true,
		AuditHookReady:         false,
		TrustMutationPlanReady: true,
		TrustMutationReady:     false,
		ApprovalWritebackReady: false,
		Detection:              result,
		AuditRecord: AuditHookRecordPlan{
			EventType:         "system",
			Action:            auditActionFor(result),
			Subject:           result.SkillSlug,
			Severity:          result.Severity,
			MerkleAppendReady: false,
			Payload:           payload,
		},
		TrustMutation: TrustMutationPlan{
			TargetSkill:        result.SkillSlug,
			Mutation:           trustMutationName(result),
			Delta:              delta,
			RecordFailureReady: false,
			Reason:             trustMutationReason(result),
		},
		ApprovalQueue: ApprovalQueuePlan{
			Required:            approvalRequired,
			QueueName:           "skill_anomaly_approval",
			QueueWritebackReady: false,
			WritesApprovalQueue: false,
			WritesQueueStore:    false,
			RequestID:           requestID,
			RequestKey:          requestKey,
			Status:              boolString(approvalRequired, "blocked_until_approval_queue_writeback", "not_required"),
			RequestedBy:         requestedBy,
			Reason:              queueReason,
			StoreArtifact:       "approval-queue-store.json",
		},
		Actions: actions,
		Notes: []string{
			"This route is non-destructive: it does not append to the Merkle audit chain, mutate Trust Score, or write Approval queue items.",
			"Use the plan shape as the contract for the later audit hook / Trust / Approval write-back slice.",
		},
	}
}

func (h *Handler) writeApprovalQueueRecord(req AuditHookPlanRequest) (ApprovalQueueWritebackReport, error) {
	event, err := h.eventFromAuditHookPlan(req)
	if err != nil {
		return ApprovalQueueWritebackReport{}, err
	}
	profile, err := h.loadOrNewProfile(event.SkillSlug)
	if err != nil {
		return ApprovalQueueWritebackReport{}, err
	}
	result := h.score(profile, event)
	plan := h.buildAuditHookPlan(result, req.RequestedBy, req.Reason, req.RequestID, req.RequestKey)
	record := h.approvalQueueRecordFromPlan(plan, true)
	if err := h.saveApprovalQueueRecord(record); err != nil {
		return ApprovalQueueWritebackReport{}, err
	}
	store := h.approvalQueueStoreSummary()
	return ApprovalQueueWritebackReport{
		PackID:                  PackID,
		GeneratedAt:             h.now().UTC(),
		Status:                  "approval_queue_written_pending_audit_trust_wiring",
		ApprovalRequired:        plan.ApprovalRequired,
		ApprovalWritebackReady:  true,
		WritesApprovalQueue:     true,
		WritesApprovalQueueFile: true,
		AuditHookPlanReady:      plan.AuditHookPlanReady,
		AuditHookReady:          false,
		MerkleAppendReady:       false,
		TrustMutationPlanReady:  plan.TrustMutationPlanReady,
		TrustMutationReady:      false,
		ActionAllowed:           false,
		ExecutionBlocked:        true,
		RequestID:               record.RequestID,
		RequestKey:              record.RequestKey,
		ApprovalQueueRecord:     record,
		ApprovalQueueStore:      store,
		PlanSummary:             plan,
		Artifacts:               []string{"approval-queue-store.json", "approval-queue-record.json", "audit-hook-plan.json", "trust-mutation-plan.json", "approval-queue-plan.json"},
		Actions: []string{
			"persisted the anomalous skill behavior approval request into the pack-local approval queue store",
			"kept Merkle audit append, Trust Score mutation, and runtime action release blocked until explicit wiring consumes the queue record",
		},
		Notes: []string{
			"This route writes only the pack-local Skill Anomaly approval queue store.",
			"It does not append Merkle audit records, mutate Trust Score, approve the action, or release runtime execution.",
		},
	}, nil
}

func (h *Handler) approvalQueueRecordPreview(plan AuditHookPlanReport) ApprovalQueueRecord {
	return h.approvalQueueRecordFromPlan(plan, false)
}

func (h *Handler) approvalQueueRecordFromPlan(plan AuditHookPlanReport, persisted bool) ApprovalQueueRecord {
	now := h.now().UTC()
	status := "preview_not_persisted"
	if persisted {
		status = "written_pending_audit_trust_wiring"
	}
	queue := plan.ApprovalQueue
	return ApprovalQueueRecord{
		PackID:                  PackID,
		QueueName:               "skill_anomaly_approval",
		RequestID:               queue.RequestID,
		RequestKey:              queue.RequestKey,
		SkillSlug:               plan.SkillSlug,
		Status:                  status,
		Severity:                plan.Detection.Severity,
		Score:                   plan.Detection.Score,
		ApprovalRequired:        plan.ApprovalRequired,
		RequestedBy:             queue.RequestedBy,
		Reason:                  queue.Reason,
		CreatedAt:               now,
		UpdatedAt:               now,
		AuditHookPlanReady:      plan.AuditHookPlanReady,
		AuditHookReady:          false,
		MerkleAppendReady:       false,
		TrustMutationPlanReady:  plan.TrustMutationPlanReady,
		TrustMutationReady:      false,
		ApprovalWritebackReady:  persisted,
		WritesApprovalQueue:     persisted,
		WritesApprovalQueueFile: persisted,
		ActionAllowed:           false,
		ExecutionBlocked:        true,
		Detection:               plan.Detection,
		AuditRecord:             plan.AuditRecord,
		TrustMutation:           plan.TrustMutation,
		ApprovalQueue: ApprovalQueuePlan{
			Required:            queue.Required,
			QueueName:           "skill_anomaly_approval",
			QueueWritebackReady: persisted,
			WritesApprovalQueue: persisted,
			WritesQueueStore:    persisted,
			RequestID:           queue.RequestID,
			RequestKey:          queue.RequestKey,
			Status:              boolString(persisted, "written_pending_audit_trust_wiring", queue.Status),
			RequestedBy:         queue.RequestedBy,
			Reason:              queue.Reason,
			StoreArtifact:       "approval-queue-store.json",
		},
		StoreArtifact: "approval-queue-store.json",
		Artifacts:     []string{"approval-queue-store.json", "approval-queue-record.json", "audit-hook-plan.json", "trust-mutation-plan.json", "approval-queue-plan.json"},
		Labels:        []string{"skill-anomaly", "approval-queue", "pack-local-store", "audit-trust-blocked", "severity-" + plan.Detection.Severity},
		Notes: []string{
			"Pack-local Skill Anomaly approval queue record.",
			"Merkle audit append, Trust mutation, and action release remain blocked until a later route consumes this record.",
		},
	}
}

func (h *Handler) approvalQueueStoreSummary() ApprovalQueueStoreSummary {
	records, _ := h.loadApprovalQueueRecords()
	return ApprovalQueueStoreSummary{
		PackID:                  PackID,
		QueueName:               "skill_anomaly_approval",
		Store:                   "pack-local-json",
		StoreReady:              true,
		RecordCount:             len(records),
		Artifact:                "approval-queue-store.json",
		WritesApprovalQueue:     false,
		WritesApprovalQueueFile: false,
		MerkleAppendReady:       false,
		TrustMutationReady:      false,
		Notes: []string{
			"Store readiness covers only the pack-local JSON approval queue bridge.",
			"Merkle append, Trust Score mutation, and runtime action release remain disabled until later explicit wiring.",
		},
	}
}

func trustDelta(result DetectionResult) int {
	if result.Block {
		return -10
	}
	if result.NeedsApproval {
		return -3
	}
	return 0
}

func auditActionFor(result DetectionResult) string {
	if result.Block {
		return "skill_anomaly_block_plan"
	}
	if result.NeedsApproval {
		return "skill_anomaly_needs_approval_plan"
	}
	return "skill_anomaly_noop_plan"
}

func trustMutationName(result DetectionResult) string {
	if result.Block {
		return "record_failure_high"
	}
	if result.NeedsApproval {
		return "record_failure_medium"
	}
	return "none"
}

func trustMutationReason(result DetectionResult) string {
	if len(result.Reasons) == 0 {
		return "behavior matches current baseline or is still collecting baseline"
	}
	names := make([]string, 0, len(result.Reasons))
	for _, reason := range result.Reasons {
		names = append(names, reason.Name)
	}
	return strings.Join(names, ",")
}

func approvalReason(result DetectionResult, explicit string) string {
	explicit = strings.TrimSpace(explicit)
	if explicit != "" {
		return explicit
	}
	if result.Block {
		return "candidate skill behavior reaches the block threshold"
	}
	if result.NeedsApproval {
		return "candidate skill behavior deviates from the learned baseline"
	}
	if result.Severity == "learning" {
		return "profile is still collecting the minimum baseline observations"
	}
	return "no approval required"
}

func approvalQueueIdentifiers(result DetectionResult, requestedBy, reason, requestID, requestKey string) (string, string) {
	requestKey = strings.TrimSpace(requestKey)
	if requestKey == "" {
		payload := strings.Join([]string{
			"pack_id=" + PackID,
			"skill_slug=" + result.SkillSlug,
			"event_id=" + result.Event.ID,
			"action=" + result.Event.Action,
			"severity=" + result.Severity,
			fmt.Sprintf("score=%.3f", result.Score),
			"requested_by=" + requestedBy,
			"reason=" + reason,
		}, "\n")
		requestKey = sha256Hex(payload)
	}
	requestID = strings.TrimSpace(requestID)
	if requestID == "" {
		requestID = "skill-anomaly-" + requestKey[:16]
	}
	return requestID, requestKey
}

func boolString(ok bool, yes, no string) string {
	if ok {
		return yes
	}
	return no
}

func consecutiveFailures(events []Event) int {
	count := 0
	for i := len(events) - 1; i >= 0; i-- {
		if events[i].Success {
			break
		}
		count++
	}
	return count
}

func severityForScore(score float64, policy DetectionPolicy) string {
	switch {
	case score >= policy.BlockScore:
		return "block"
	case score >= policy.NeedsApprovalScore:
		return "needs_approval"
	default:
		return "normal"
	}
}

func (h *Handler) appendEvent(profile Profile, event Event, result DetectionResult) Profile {
	profile.Recent = append(profile.Recent, event)
	if len(profile.Recent) > h.policy.WindowSize {
		profile.Recent = profile.Recent[len(profile.Recent)-h.policy.WindowSize:]
	}
	if result.NeedsApproval || result.Block {
		profile.AnomalyCount++
		profile.LastAnomalyAt = h.now().UTC()
	}
	return h.recalculate(profile)
}

func (h *Handler) recalculate(profile Profile) Profile {
	profile.WindowSize = h.policy.WindowSize
	profile.Observed = len(profile.Recent)
	profile.ActionDistrib = map[string]float64{}
	profile.ParamKeySet = map[string]int{}
	if len(profile.Recent) == 0 {
		profile.UpdatedAt = h.now().UTC()
		return profile
	}
	successes := 0
	durationTotal := int64(0)
	durationCount := 0
	actionCounts := map[string]int{}
	for _, event := range profile.Recent {
		actionCounts[event.Action]++
		if event.Success {
			successes++
		}
		if event.DurationMS > 0 {
			durationTotal += event.DurationMS
			durationCount++
		}
		for _, key := range event.ParamKeys {
			profile.ParamKeySet[key]++
		}
	}
	for action, count := range actionCounts {
		profile.ActionDistrib[action] = round4(float64(count) / float64(len(profile.Recent)))
	}
	profile.SuccessRate = round4(float64(successes) / float64(len(profile.Recent)))
	if durationCount > 0 {
		profile.AvgDurationMS = round4(float64(durationTotal) / float64(durationCount))
	}
	profile.CallsPerMinute = round4(callsPerMinute(profile.Recent))
	profile.UpdatedAt = h.now().UTC()
	return profile
}

func callsPerMinute(events []Event) float64 {
	if len(events) < 2 {
		return float64(len(events))
	}
	first := events[0].Timestamp
	last := events[len(events)-1].Timestamp
	minutes := last.Sub(first).Minutes()
	if minutes <= 0 {
		return float64(len(events))
	}
	return float64(len(events)) / minutes
}

func round4(v float64) float64 {
	return math.Round(v*10000) / 10000
}

func (h *Handler) loadOrNewProfile(slug string) (Profile, error) {
	profile, err := h.loadProfile(slug)
	if err == nil {
		return profile, nil
	}
	if os.IsNotExist(err) {
		return Profile{SkillSlug: normalizeSlug(slug), WindowSize: h.policy.WindowSize, ActionDistrib: map[string]float64{}, ParamKeySet: map[string]int{}, Recent: []Event{}}, nil
	}
	return Profile{}, err
}

func (h *Handler) loadProfile(slug string) (Profile, error) {
	slug = normalizeSlug(slug)
	if !safeSlugRe.MatchString(slug) {
		return Profile{}, fmt.Errorf("invalid skill_slug")
	}
	data, err := os.ReadFile(h.profilePath(slug))
	if err != nil {
		return Profile{}, err
	}
	var profile Profile
	if err := json.Unmarshal(data, &profile); err != nil {
		return Profile{}, err
	}
	profile = h.recalculate(profile)
	return profile, nil
}

func (h *Handler) saveProfile(profile Profile) error {
	if !safeSlugRe.MatchString(profile.SkillSlug) {
		return fmt.Errorf("invalid skill_slug")
	}
	if err := os.MkdirAll(filepath.Dir(h.profilePath(profile.SkillSlug)), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(profile, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(h.profilePath(profile.SkillSlug), data, 0o644)
}

func (h *Handler) listProfiles() ([]ProfileSummary, error) {
	files, err := filepath.Glob(filepath.Join(h.dataDir, "profiles", "*.json"))
	if err != nil {
		return nil, err
	}
	profiles := make([]ProfileSummary, 0, len(files))
	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			continue
		}
		var profile Profile
		if err := json.Unmarshal(data, &profile); err != nil {
			continue
		}
		profile = h.recalculate(profile)
		profiles = append(profiles, summarize(profile))
	}
	sort.Slice(profiles, func(i, j int) bool { return profiles[i].SkillSlug < profiles[j].SkillSlug })
	return profiles, nil
}

func (h *Handler) appendEventLog(event Event) error {
	if err := os.MkdirAll(filepath.Dir(h.eventLogPath(event.SkillSlug)), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(h.eventLogPath(event.SkillSlug), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	data, err := json.Marshal(event)
	if err != nil {
		return err
	}
	_, err = f.Write(append(data, '\n'))
	return err
}

func (h *Handler) saveApprovalQueueRecord(record ApprovalQueueRecord) error {
	records, _ := h.loadApprovalQueueRecords()
	replaced := false
	for idx := range records {
		if records[idx].RequestKey == record.RequestKey || records[idx].RequestID == record.RequestID {
			if !records[idx].CreatedAt.IsZero() {
				record.CreatedAt = records[idx].CreatedAt
			}
			records[idx] = record
			replaced = true
			break
		}
	}
	if !replaced {
		records = append(records, record)
	}
	sort.Slice(records, func(i, j int) bool { return records[i].UpdatedAt.After(records[j].UpdatedAt) })
	store := map[string]any{
		"pack_id":      PackID,
		"queue_name":   "skill_anomaly_approval",
		"format":       "json-skill-anomaly-approval-queue-store",
		"record_count": len(records),
		"updated_at":   h.now().UTC(),
		"records":      records,
	}
	if err := os.MkdirAll(filepath.Dir(h.approvalQueueStorePath()), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(store, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(h.approvalQueueStorePath(), data, 0o644)
}

func (h *Handler) loadApprovalQueueRecords() ([]ApprovalQueueRecord, error) {
	data, err := os.ReadFile(h.approvalQueueStorePath())
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var store struct {
		Records []ApprovalQueueRecord `json:"records"`
	}
	if err := json.Unmarshal(data, &store); err != nil {
		return nil, err
	}
	return store.Records, nil
}

func (h *Handler) listEvents(slug string, limit int) ([]Event, error) {
	if strings.TrimSpace(slug) == "" {
		return h.listAllEvents(limit)
	}
	slug = normalizeSlug(slug)
	if !safeSlugRe.MatchString(slug) {
		return nil, fmt.Errorf("invalid skill_slug")
	}
	return readEventLog(h.eventLogPath(slug), limit), nil
}

func (h *Handler) listAllEvents(limit int) ([]Event, error) {
	files, err := filepath.Glob(filepath.Join(h.dataDir, "events", "*.jsonl"))
	if err != nil {
		return nil, err
	}
	var out []Event
	for _, file := range files {
		out = append(out, readEventLog(file, limit)...)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Timestamp.After(out[j].Timestamp) })
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func readEventLog(path string, limit int) []Event {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()
	var events []Event
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		var event Event
		if err := json.Unmarshal(scanner.Bytes(), &event); err == nil {
			events = append(events, event)
		}
	}
	sort.Slice(events, func(i, j int) bool { return events[i].Timestamp.After(events[j].Timestamp) })
	if limit > 0 && len(events) > limit {
		events = events[:limit]
	}
	return events
}

func parseLimit(raw string, fallback int) int {
	if strings.TrimSpace(raw) == "" {
		return fallback
	}
	var n int
	_, _ = fmt.Sscanf(raw, "%d", &n)
	if n <= 0 || n > 500 {
		return fallback
	}
	return n
}

func (h *Handler) profilePath(slug string) string {
	return filepath.Join(h.dataDir, "profiles", slug+".json")
}

func (h *Handler) eventLogPath(slug string) string {
	return filepath.Join(h.dataDir, "events", slug+".jsonl")
}

func (h *Handler) approvalQueueStorePath() string {
	return filepath.Join(h.dataDir, "approval-queue-store.json")
}

func summarize(profile Profile) ProfileSummary {
	return ProfileSummary{
		SkillSlug:      profile.SkillSlug,
		Observed:       profile.Observed,
		CallsPerMinute: profile.CallsPerMinute,
		ActionDistrib:  profile.ActionDistrib,
		ParamKeySet:    profile.ParamKeySet,
		SuccessRate:    profile.SuccessRate,
		AvgDurationMS:  profile.AvgDurationMS,
		LastAnomalyAt:  profile.LastAnomalyAt,
		AnomalyCount:   profile.AnomalyCount,
		UpdatedAt:      profile.UpdatedAt,
	}
}

func observedStatus(dryRun bool) string {
	if dryRun {
		return "validated"
	}
	return "observed"
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]any{"error": message})
}

func sha256Hex(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}
