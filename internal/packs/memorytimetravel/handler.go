// Package memorytimetravel contains the backend implementation for the built-in
// Memory Time Travel capability pack. The first delivery is intentionally a pack
// shell: it owns manifest-gated HTTP routes, versioned memory snapshot storage,
// point-in-time reconstruction, drift diff summaries, rollback plans, JSON
// evidence export, and read-only Merkle audit-chain verification while native
// Ledger KV kv_history write-back remains a later slice.
package memorytimetravel

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"yunque-agent/pkg/packruntime"
)

const PackID = "yunque.pack.memory-time-travel"

// Config describes runtime dependencies for the Memory Time Travel pack shell.
type Config struct {
	DataDir                  string
	Now                      func() time.Time
	Policy                   RetentionPolicy
	TemporalKV               TemporalKVReader
	MemoryPersisterWriteback bool
	MerkleVerifier           MerkleVerifier
}

// Handler serves the Memory Time Travel pack API surface.
type Handler struct {
	dataDir                  string
	now                      func() time.Time
	policy                   RetentionPolicy
	temporalKV               TemporalKVReader
	memoryPersisterWriteback bool
	merkleVerifier           MerkleVerifier
}

// TemporalKVReader is the narrow Memory Time Travel dependency on Ledger KV
// history. It lets the pack read versioned memory snapshots without importing
// the concrete Ledger implementation into the pack shell.
type TemporalKVReader interface {
	SnapshotRawAt(ctx context.Context, namespace string, at time.Time) (map[string][]byte, error)
}

// MerkleVerifier is the pack-local adapter boundary for the global audit chain.
// The pack only asks for read-only verification output so it does not need to
// import the concrete agentcore/audit implementation.
type MerkleVerifier interface {
	VerifyMerkleAuditChain(ctx context.Context, limit int) (MerkleVerification, error)
}

// MerkleVerifierFunc adapts a function into a MerkleVerifier.
type MerkleVerifierFunc func(ctx context.Context, limit int) (MerkleVerification, error)

func (fn MerkleVerifierFunc) VerifyMerkleAuditChain(ctx context.Context, limit int) (MerkleVerification, error) {
	return fn(ctx, limit)
}

// RetentionPolicy models the future kv_history retention contract at pack level.
type RetentionPolicy struct {
	RetentionDays        int `json:"retention_days"`
	MaxVersionsPerKey    int `json:"max_versions_per_key"`
	MaxSnapshotBytes     int `json:"max_snapshot_bytes"`
	MaxKeysPerSnapshot   int `json:"max_keys_per_snapshot"`
	EvidenceMaxSnapshots int `json:"evidence_max_snapshots"`
}

// Snapshot contains one exported memory namespace state at a point in time.
type Snapshot struct {
	ID        string            `json:"id"`
	Namespace string            `json:"namespace"`
	CreatedAt time.Time         `json:"created_at"`
	Source    string            `json:"source,omitempty"`
	Reason    string            `json:"reason,omitempty"`
	Values    map[string]string `json:"values"`
	Hash      string            `json:"hash"`
	SizeBytes int               `json:"size_bytes"`
	KeyCount  int               `json:"key_count"`
	Version   int               `json:"version"`
}

type SnapshotSummary struct {
	ID        string    `json:"id"`
	Namespace string    `json:"namespace"`
	CreatedAt time.Time `json:"created_at"`
	Source    string    `json:"source,omitempty"`
	Reason    string    `json:"reason,omitempty"`
	Hash      string    `json:"hash"`
	SizeBytes int       `json:"size_bytes"`
	KeyCount  int       `json:"key_count"`
	Version   int       `json:"version"`
}

type SaveSnapshotRequest struct {
	ID        string            `json:"id,omitempty"`
	Namespace string            `json:"namespace,omitempty"`
	Source    string            `json:"source,omitempty"`
	Reason    string            `json:"reason,omitempty"`
	Values    map[string]string `json:"values"`
	DryRun    bool              `json:"dry_run,omitempty"`
}

type SnapshotAtRequest struct {
	Namespace string    `json:"namespace,omitempty"`
	At        time.Time `json:"at,omitempty"`
}

type SnapshotAtResponse struct {
	Namespace string            `json:"namespace"`
	At        time.Time         `json:"at"`
	Snapshot  *Snapshot         `json:"snapshot,omitempty"`
	Values    map[string]string `json:"values"`
	MatchedID string            `json:"matched_id,omitempty"`
	Status    string            `json:"status"`
	Source    string            `json:"source,omitempty"`
}

type DiffRequest struct {
	Namespace string `json:"namespace,omitempty"`
	BaseID    string `json:"base_id"`
	TargetID  string `json:"target_id"`
}

type DiffEntry struct {
	Key         string `json:"key"`
	Change      string `json:"change"`
	Before      string `json:"before,omitempty"`
	After       string `json:"after,omitempty"`
	BeforeHash  string `json:"before_hash,omitempty"`
	AfterHash   string `json:"after_hash,omitempty"`
	ImpactLevel string `json:"impact_level"`
}

type DiffReport struct {
	ID              string      `json:"id"`
	PackID          string      `json:"pack_id"`
	Namespace       string      `json:"namespace"`
	CreatedAt       time.Time   `json:"created_at"`
	Stage           string      `json:"stage"`
	BaseID          string      `json:"base_id"`
	TargetID        string      `json:"target_id"`
	AddedCount      int         `json:"added_count"`
	RemovedCount    int         `json:"removed_count"`
	ChangedCount    int         `json:"changed_count"`
	DriftScore      float64     `json:"drift_score"`
	RiskLevel       string      `json:"risk_level"`
	Entries         []DiffEntry `json:"entries"`
	RollbackPlan    []string    `json:"rollback_plan"`
	Recommendations []string    `json:"recommendations,omitempty"`
	Notes           []string    `json:"notes,omitempty"`
}

type RollbackPlanRequest struct {
	Namespace  string `json:"namespace,omitempty"`
	SnapshotID string `json:"snapshot_id"`
	DryRun     bool   `json:"dry_run,omitempty"`
}

type RollbackPlan struct {
	PackID        string            `json:"pack_id"`
	Namespace     string            `json:"namespace"`
	SnapshotID    string            `json:"snapshot_id"`
	DryRun        bool              `json:"dry_run"`
	ActionCount   int               `json:"action_count"`
	Actions       []string          `json:"actions"`
	PreviewValues map[string]string `json:"preview_values,omitempty"`
	Status        string            `json:"status"`
	Notes         []string          `json:"notes,omitempty"`
}

type MerkleAuditRecord struct {
	Seq       uint64    `json:"seq"`
	Timestamp time.Time `json:"timestamp"`
	Type      string    `json:"type"`
	Actor     string    `json:"actor,omitempty"`
	Action    string    `json:"action"`
	PrevHash  string    `json:"prev_hash,omitempty"`
	Hash      string    `json:"hash"`
}

type MerkleVerification struct {
	Ready         bool                `json:"ready"`
	Valid         bool                `json:"valid"`
	InvalidIndex  int                 `json:"invalid_index"`
	RecordCount   int                 `json:"record_count"`
	LastHash      string              `json:"last_hash,omitempty"`
	LastSeq       uint64              `json:"last_seq,omitempty"`
	CheckedAt     time.Time           `json:"checked_at"`
	RecentRecords []MerkleAuditRecord `json:"recent_records,omitempty"`
	Notes         []string            `json:"notes,omitempty"`
}

var safeIDRe = regexp.MustCompile(`^[a-z0-9][a-z0-9_.-]{0,79}$`)

// New creates a Memory Time Travel pack handler.
func New(cfg Config) *Handler {
	dataDir := strings.TrimSpace(cfg.DataDir)
	if dataDir == "" {
		dataDir = filepath.Join(".", "data", "memory-time-travel")
	}
	now := cfg.Now
	if now == nil {
		now = time.Now
	}
	return &Handler{
		dataDir:                  dataDir,
		now:                      now,
		policy:                   normalizePolicy(cfg.Policy),
		temporalKV:               cfg.TemporalKV,
		memoryPersisterWriteback: cfg.MemoryPersisterWriteback,
		merkleVerifier:           cfg.MerkleVerifier,
	}
}

// DefaultHandler returns a handler bound to the default local data directory.
func DefaultHandler() *Handler { return New(Config{}) }

// PackID returns the stable manifest id for the built-in Memory Time Travel pack.
func (h *Handler) PackID() string { return PackID }

// Routes exposes the Memory Time Travel shell HTTP API to the Pack Runtime host.
func (h *Handler) Routes() []packruntime.BackendRoute {
	return []packruntime.BackendRoute{
		{Method: http.MethodGet, Path: "/v1/memory-time-travel/status", Handler: h.Status},
		{Methods: []string{http.MethodGet, http.MethodPost}, Path: "/v1/memory-time-travel/snapshots", Handler: h.Snapshots},
		{Method: http.MethodGet, Path: "/v1/memory-time-travel/snapshots/", Handler: h.SnapshotDetail},
		{Method: http.MethodPost, Path: "/v1/memory-time-travel/snapshot-at", Handler: h.SnapshotAt},
		{Method: http.MethodPost, Path: "/v1/memory-time-travel/diff", Handler: h.Diff},
		{Method: http.MethodPost, Path: "/v1/memory-time-travel/rollback-plan", Handler: h.RollbackPlan},
		{Method: http.MethodGet, Path: "/v1/memory-time-travel/audit/verify", Handler: h.AuditVerify},
		{Method: http.MethodGet, Path: "/v1/memory-time-travel/evidence/", Handler: h.Evidence},
	}
}

func (h *Handler) Status(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	snapshots, err := h.listSnapshots("")
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	namespaces := map[string]bool{}
	for _, snapshot := range snapshots {
		namespaces[snapshot.Namespace] = true
	}
	capabilities := []string{
		"memory.snapshot.store",
		"memory.snapshot_at.reconstruct",
		"memory.drift.diff",
		"memory.rollback.plan",
		"memory.evidence.export",
	}
	if h.merkleVerifier != nil {
		capabilities = append(capabilities, "memory.audit.verify")
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"pack_id":                          PackID,
		"stage":                            "pack-shell-before-ledger-kv-history",
		"snapshot_store_ready":             true,
		"temporal_query_ready":             true,
		"ledger_history_ready":             h.temporalKV != nil,
		"merkle_verification_ready":        h.merkleVerifier != nil,
		"memory_persister_writeback_ready": h.memoryPersisterWriteback,
		"rollback_writeback_ready":         false,
		"snapshot_count":                   len(snapshots),
		"namespace_count":                  len(namespaces),
		"store_dir":                        h.dataDir,
		"policy":                           h.policy,
		"last_snapshot":                    firstSnapshot(snapshots),
		"capabilities":                     capabilities,
		"notes":                            h.statusNotes(),
	})
}

func (h *Handler) Snapshots(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		namespace := strings.TrimSpace(r.URL.Query().Get("namespace"))
		snapshots, err := h.listSnapshots(namespace)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"snapshots": snapshots, "count": len(snapshots)})
	case http.MethodPost:
		var req SaveSnapshotRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid memory snapshot payload")
			return
		}
		snapshot, err := h.normalizeSnapshot(req)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		if !req.DryRun {
			if err := h.saveSnapshot(snapshot); err != nil {
				writeError(w, http.StatusInternalServerError, err.Error())
				return
			}
		}
		status := "saved"
		if req.DryRun {
			status = "dry_run"
		}
		writeJSON(w, http.StatusCreated, map[string]any{"snapshot": snapshot, "status": status})
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (h *Handler) SnapshotDetail(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	id := strings.TrimPrefix(r.URL.Path, "/v1/memory-time-travel/snapshots/")
	snapshot, err := h.loadSnapshotByID("", id)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"snapshot": snapshot})
}

func (h *Handler) SnapshotAt(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var req SnapshotAtRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid snapshot-at payload")
		return
	}
	namespace := normalizeNamespace(req.Namespace)
	at := req.At
	if at.IsZero() {
		at = h.now().UTC()
	}
	if h.temporalKV != nil {
		values, err := h.temporalSnapshotValues(r.Context(), namespace, at)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if len(values) > 0 {
			writeJSON(w, http.StatusOK, SnapshotAtResponse{Namespace: namespace, At: at, Values: values, Status: "reconstructed", Source: "ledger-kv-history"})
			return
		}
	}
	snapshots, err := h.listSnapshots(namespace)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	var matched *Snapshot
	for _, summary := range snapshots {
		if summary.CreatedAt.After(at) {
			continue
		}
		snapshot, err := h.loadSnapshotByID(namespace, summary.ID)
		if err != nil {
			continue
		}
		matched = &snapshot
		break
	}
	if matched == nil {
		writeJSON(w, http.StatusOK, SnapshotAtResponse{Namespace: namespace, At: at, Values: map[string]string{}, Status: "not_found", Source: "pack-local-snapshots"})
		return
	}
	writeJSON(w, http.StatusOK, SnapshotAtResponse{Namespace: namespace, At: at, Snapshot: matched, Values: matched.Values, MatchedID: matched.ID, Status: "reconstructed", Source: "pack-local-snapshots"})
}

func (h *Handler) Diff(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var req DiffRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid memory diff payload")
		return
	}
	report, err := h.buildDiff(req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"diff": report})
}

func (h *Handler) RollbackPlan(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var req RollbackPlanRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid rollback-plan payload")
		return
	}
	plan, err := h.buildRollbackPlan(req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"plan": plan})
}

func (h *Handler) AuditVerify(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	limit := parseAuditLimit(r.URL.Query().Get("limit"))
	if h.merkleVerifier == nil {
		writeJSON(w, http.StatusOK, MerkleVerification{
			Ready:        false,
			Valid:        false,
			InvalidIndex: -1,
			CheckedAt:    h.now().UTC(),
			Notes:        []string{"Merkle audit-chain verifier is not attached to this Memory Time Travel pack instance."},
		})
		return
	}
	result, err := h.merkleVerifier.VerifyMerkleAuditChain(r.Context(), limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if result.CheckedAt.IsZero() {
		result.CheckedAt = h.now().UTC()
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) Evidence(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	id := strings.TrimPrefix(r.URL.Path, "/v1/memory-time-travel/evidence/")
	snapshot, err := h.loadSnapshotByID("", id)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	snapshots, _ := h.listSnapshots(snapshot.Namespace)
	payload := map[string]any{
		"pack_id":     PackID,
		"exported_at": h.now().UTC(),
		"format":      "json-memory-time-travel-evidence",
		"files":       []string{"snapshot.json", "summary.json", "rollback-plan.json", "audit-verification.json"},
		"snapshot":    snapshot,
		"history":     truncateSnapshots(snapshots, h.policy.EvidenceMaxSnapshots),
	}
	if h.merkleVerifier != nil {
		auditVerification, err := h.merkleVerifier.VerifyMerkleAuditChain(r.Context(), 10)
		if err != nil {
			payload["audit_verification_error"] = err.Error()
		} else {
			if auditVerification.CheckedAt.IsZero() {
				auditVerification.CheckedAt = h.now().UTC()
			}
			payload["audit_verification"] = auditVerification
		}
	}
	writeJSON(w, http.StatusOK, payload)
}

func (h *Handler) statusNotes() []string {
	notes := []string{"Pack-local snapshot store, point-in-time reconstruction, drift diff, dry-run rollback planning, and evidence export are available."}
	if h.temporalKV != nil {
		if h.memoryPersisterWriteback {
			notes = append(notes, "Ledger KV temporal history reader is attached and Memory Persister mirrors Mid/Long flushes into memory_snapshot; native kv_history tables, retention cron, and approved rollback execution remain follow-up wiring.")
		} else {
			notes = append(notes, "Ledger KV temporal history reader is attached for snapshot-at reconstruction; Memory Persister write-back, native kv_history tables, retention cron, and approved rollback execution remain follow-up wiring.")
		}
	} else {
		notes = append(notes, "Ledger KV kv_history reader is not attached; snapshot-at reconstruction falls back to pack-local snapshots.")
	}
	if h.merkleVerifier != nil {
		notes = append(notes, "Read-only Merkle audit-chain verification is attached through Pack Runtime; individual KV-history entries are not yet linked to audit proofs.")
	} else {
		notes = append(notes, "Merkle audit-chain verification is not attached to this pack instance yet.")
	}
	return notes
}

func (h *Handler) temporalSnapshotValues(ctx context.Context, namespace string, at time.Time) (map[string]string, error) {
	raw, err := h.temporalKV.SnapshotRawAt(ctx, namespace, at)
	if err != nil {
		return nil, err
	}
	values := make(map[string]string, len(raw))
	for key, value := range raw {
		values[key] = decodeTemporalValue(value)
	}
	return values, nil
}

func decodeTemporalValue(value []byte) string {
	var s string
	if err := json.Unmarshal(value, &s); err == nil {
		return s
	}
	var decoded any
	if err := json.Unmarshal(value, &decoded); err == nil {
		data, _ := json.Marshal(decoded)
		return string(data)
	}
	return string(value)
}

func parseAuditLimit(raw string) int {
	limit := 10
	if strings.TrimSpace(raw) != "" {
		if parsed, err := strconv.Atoi(strings.TrimSpace(raw)); err == nil {
			limit = parsed
		}
	}
	if limit <= 0 {
		return 10
	}
	if limit > 50 {
		return 50
	}
	return limit
}

func (h *Handler) normalizeSnapshot(req SaveSnapshotRequest) (Snapshot, error) {
	namespace := normalizeNamespace(req.Namespace)
	if len(req.Values) == 0 {
		return Snapshot{}, fmt.Errorf("snapshot values are required")
	}
	if len(req.Values) > h.policy.MaxKeysPerSnapshot {
		return Snapshot{}, fmt.Errorf("snapshot has too many keys: %d > %d", len(req.Values), h.policy.MaxKeysPerSnapshot)
	}
	values := make(map[string]string, len(req.Values))
	for key, value := range req.Values {
		key = strings.TrimSpace(key)
		if key == "" {
			return Snapshot{}, fmt.Errorf("snapshot key must not be empty")
		}
		values[key] = value
	}
	hash, size, err := snapshotDigest(namespace, values)
	if err != nil {
		return Snapshot{}, err
	}
	if size > h.policy.MaxSnapshotBytes {
		return Snapshot{}, fmt.Errorf("snapshot payload too large: %d > %d", size, h.policy.MaxSnapshotBytes)
	}
	createdAt := h.now().UTC()
	id := strings.TrimSpace(req.ID)
	if id == "" {
		id = h.snapshotID(namespace, hash, createdAt)
	}
	id = strings.ToLower(id)
	if !safeIDRe.MatchString(id) {
		return Snapshot{}, fmt.Errorf("snapshot id must match ^[a-z0-9][a-z0-9_.-]{0,79}$")
	}
	return Snapshot{
		ID:        id,
		Namespace: namespace,
		CreatedAt: createdAt,
		Source:    strings.TrimSpace(req.Source),
		Reason:    strings.TrimSpace(req.Reason),
		Values:    values,
		Hash:      hash,
		SizeBytes: size,
		KeyCount:  len(values),
		Version:   1,
	}, nil
}

func (h *Handler) buildDiff(req DiffRequest) (DiffReport, error) {
	namespace := normalizeNamespace(req.Namespace)
	base, err := h.loadSnapshotByID(namespace, req.BaseID)
	if err != nil {
		return DiffReport{}, fmt.Errorf("base snapshot: %w", err)
	}
	target, err := h.loadSnapshotByID(namespace, req.TargetID)
	if err != nil {
		return DiffReport{}, fmt.Errorf("target snapshot: %w", err)
	}
	entries := diffValues(base.Values, target.Values)
	added, removed, changed := 0, 0, 0
	var rollback []string
	for _, entry := range entries {
		switch entry.Change {
		case "added":
			added++
			rollback = append(rollback, fmt.Sprintf("delete %s", entry.Key))
		case "removed":
			removed++
			rollback = append(rollback, fmt.Sprintf("restore %s from %s", entry.Key, base.ID))
		case "changed":
			changed++
			rollback = append(rollback, fmt.Sprintf("set %s to base snapshot value", entry.Key))
		}
	}
	if len(rollback) == 0 {
		rollback = append(rollback, "no rollback action required; snapshots are equivalent")
	}
	driftScore := driftScore(added, removed, changed, max(len(base.Values), len(target.Values)))
	risk := riskLevel(driftScore, entries)
	return DiffReport{
		ID:              h.diffID(base.ID, target.ID, entries),
		PackID:          PackID,
		Namespace:       namespace,
		CreatedAt:       h.now().UTC(),
		Stage:           "pack-shell-before-ledger-kv-history",
		BaseID:          base.ID,
		TargetID:        target.ID,
		AddedCount:      added,
		RemovedCount:    removed,
		ChangedCount:    changed,
		DriftScore:      driftScore,
		RiskLevel:       risk,
		Entries:         entries,
		RollbackPlan:    rollback,
		Recommendations: diffRecommendations(risk, entries),
		Notes:           []string{"Diff is computed from pack-local snapshots; kv_history + Merkle audit verification remain follow-up wiring."},
	}, nil
}

func (h *Handler) buildRollbackPlan(req RollbackPlanRequest) (RollbackPlan, error) {
	namespace := normalizeNamespace(req.Namespace)
	snapshot, err := h.loadSnapshotByID(namespace, req.SnapshotID)
	if err != nil {
		return RollbackPlan{}, err
	}
	keys := make([]string, 0, len(snapshot.Values))
	for key := range snapshot.Values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	actions := make([]string, 0, len(keys)+1)
	actions = append(actions, fmt.Sprintf("load snapshot %s from namespace %s", snapshot.ID, snapshot.Namespace))
	for _, key := range keys {
		actions = append(actions, fmt.Sprintf("put %s=%s", key, snapshot.Values[key]))
	}
	status := "dry_run"
	if !req.DryRun {
		status = "plan_only"
	}
	return RollbackPlan{
		PackID:        PackID,
		Namespace:     snapshot.Namespace,
		SnapshotID:    snapshot.ID,
		DryRun:        req.DryRun,
		ActionCount:   len(actions),
		Actions:       actions,
		PreviewValues: snapshot.Values,
		Status:        status,
		Notes:         []string{"Rollback write-back is intentionally not connected in this pack shell; execute through Ledger KV after approval in a later slice."},
	}, nil
}

func (h *Handler) saveSnapshot(snapshot Snapshot) error {
	dir, err := h.snapshotDir(snapshot.Namespace, snapshot.ID)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, "snapshot.json"), append(data, '\n'), 0o644); err != nil {
		return err
	}
	summaryData, err := json.MarshalIndent(snapshotSummary(snapshot), "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "summary.json"), append(summaryData, '\n'), 0o644)
}

func (h *Handler) loadSnapshotByID(namespace, id string) (Snapshot, error) {
	id = strings.Trim(strings.TrimSpace(id), "/")
	if !safeIDRe.MatchString(id) {
		return Snapshot{}, fmt.Errorf("invalid snapshot id")
	}
	if namespace != "" {
		return h.loadSnapshot(normalizeNamespace(namespace), id)
	}
	snapshots, err := h.listSnapshots("")
	if err != nil {
		return Snapshot{}, err
	}
	for _, summary := range snapshots {
		if summary.ID == id {
			return h.loadSnapshot(summary.Namespace, summary.ID)
		}
	}
	return Snapshot{}, fmt.Errorf("memory snapshot not found")
}

func (h *Handler) loadSnapshot(namespace, id string) (Snapshot, error) {
	dir, err := h.snapshotDir(namespace, id)
	if err != nil {
		return Snapshot{}, err
	}
	data, err := os.ReadFile(filepath.Join(dir, "snapshot.json"))
	if err != nil {
		return Snapshot{}, fmt.Errorf("memory snapshot not found")
	}
	var snapshot Snapshot
	if err := json.Unmarshal(data, &snapshot); err != nil {
		return Snapshot{}, fmt.Errorf("invalid memory snapshot file")
	}
	return snapshot, nil
}

func (h *Handler) listSnapshots(namespace string) ([]SnapshotSummary, error) {
	root := h.snapshotRoot()
	var out []SnapshotSummary
	if namespace != "" {
		items, err := h.listNamespaceSnapshots(normalizeNamespace(namespace))
		if err != nil {
			return nil, err
		}
		out = append(out, items...)
	} else {
		entries, err := os.ReadDir(root)
		if os.IsNotExist(err) {
			return []SnapshotSummary{}, nil
		}
		if err != nil {
			return nil, err
		}
		for _, entry := range entries {
			if !entry.IsDir() || !safeIDRe.MatchString(entry.Name()) {
				continue
			}
			items, err := h.listNamespaceSnapshots(entry.Name())
			if err == nil {
				out = append(out, items...)
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	return out, nil
}

func (h *Handler) listNamespaceSnapshots(namespace string) ([]SnapshotSummary, error) {
	entries, err := os.ReadDir(filepath.Join(h.snapshotRoot(), namespace))
	if os.IsNotExist(err) {
		return []SnapshotSummary{}, nil
	}
	if err != nil {
		return nil, err
	}
	out := make([]SnapshotSummary, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() || !safeIDRe.MatchString(entry.Name()) {
			continue
		}
		snapshot, err := h.loadSnapshot(namespace, entry.Name())
		if err == nil {
			out = append(out, snapshotSummary(snapshot))
		}
	}
	return out, nil
}

func (h *Handler) snapshotRoot() string { return filepath.Join(h.dataDir, "snapshots") }

func (h *Handler) snapshotDir(namespace, id string) (string, error) {
	namespace = normalizeNamespace(namespace)
	id = strings.Trim(strings.TrimSpace(id), "/")
	if !safeIDRe.MatchString(id) {
		return "", fmt.Errorf("invalid snapshot id")
	}
	return filepath.Join(h.snapshotRoot(), namespace, id), nil
}

func (h *Handler) snapshotID(namespace, hash string, at time.Time) string {
	seed := fmt.Sprintf("%s:%s:%d", namespace, hash, at.UnixNano())
	hasher := fnv.New32a()
	_, _ = hasher.Write([]byte(seed))
	return fmt.Sprintf("snap-%s-%08x", at.Format("20060102-150405"), hasher.Sum32())
}

func (h *Handler) diffID(baseID, targetID string, entries []DiffEntry) string {
	seed := baseID + ":" + targetID
	for _, entry := range entries {
		seed += ":" + entry.Key + ":" + entry.Change
	}
	hash := fnv.New32a()
	_, _ = hash.Write([]byte(seed))
	return fmt.Sprintf("memory-diff-%08x", hash.Sum32())
}

func normalizeNamespace(namespace string) string {
	namespace = strings.ToLower(strings.TrimSpace(namespace))
	if namespace == "" {
		return "memory_snapshot"
	}
	var b strings.Builder
	lastDash := false
	for _, r := range namespace {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			lastDash = false
		case r == '_' || r == '-' || r == '.' || r == ' ' || r == '/':
			if !lastDash && b.Len() > 0 {
				b.WriteByte('-')
				lastDash = true
			}
		}
	}
	out := strings.Trim(b.String(), "-")
	if out == "" {
		return "memory_snapshot"
	}
	if len(out) > 80 {
		out = strings.Trim(out[:80], "-")
	}
	return out
}

func snapshotDigest(namespace string, values map[string]string) (string, int, error) {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	canonical := struct {
		Namespace string            `json:"namespace"`
		Values    map[string]string `json:"values"`
		Keys      []string          `json:"keys"`
	}{Namespace: namespace, Values: values, Keys: keys}
	data, err := json.Marshal(canonical)
	if err != nil {
		return "", 0, err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), len(data), nil
}

func snapshotSummary(snapshot Snapshot) SnapshotSummary {
	return SnapshotSummary{
		ID:        snapshot.ID,
		Namespace: snapshot.Namespace,
		CreatedAt: snapshot.CreatedAt,
		Source:    snapshot.Source,
		Reason:    snapshot.Reason,
		Hash:      snapshot.Hash,
		SizeBytes: snapshot.SizeBytes,
		KeyCount:  snapshot.KeyCount,
		Version:   snapshot.Version,
	}
}

func firstSnapshot(snapshots []SnapshotSummary) *SnapshotSummary {
	if len(snapshots) == 0 {
		return nil
	}
	return &snapshots[0]
}

func truncateSnapshots(snapshots []SnapshotSummary, limit int) []SnapshotSummary {
	if limit <= 0 || len(snapshots) <= limit {
		return snapshots
	}
	return snapshots[:limit]
}

func diffValues(base, target map[string]string) []DiffEntry {
	keys := map[string]bool{}
	for key := range base {
		keys[key] = true
	}
	for key := range target {
		keys[key] = true
	}
	sorted := make([]string, 0, len(keys))
	for key := range keys {
		sorted = append(sorted, key)
	}
	sort.Strings(sorted)
	entries := make([]DiffEntry, 0)
	for _, key := range sorted {
		before, hasBefore := base[key]
		after, hasAfter := target[key]
		switch {
		case !hasBefore && hasAfter:
			entries = append(entries, DiffEntry{Key: key, Change: "added", After: after, AfterHash: valueHash(after), ImpactLevel: impactLevel(key, after)})
		case hasBefore && !hasAfter:
			entries = append(entries, DiffEntry{Key: key, Change: "removed", Before: before, BeforeHash: valueHash(before), ImpactLevel: impactLevel(key, before)})
		case before != after:
			entries = append(entries, DiffEntry{Key: key, Change: "changed", Before: before, After: after, BeforeHash: valueHash(before), AfterHash: valueHash(after), ImpactLevel: impactLevel(key, before+after)})
		}
	}
	return entries
}

func valueHash(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

func impactLevel(key, value string) string {
	lower := strings.ToLower(key + " " + value)
	for _, token := range []string{"credential", "secret", "token", "apikey", "api_key", "password", "权限", "密钥"} {
		if strings.Contains(lower, token) {
			return "high"
		}
	}
	for _, token := range []string{"goal", "persona", "instruction", "policy", "trust", "memory"} {
		if strings.Contains(lower, token) {
			return "medium"
		}
	}
	return "low"
}

func driftScore(added, removed, changed, total int) float64 {
	if total <= 0 {
		return 0
	}
	value := (float64(added)*1.0 + float64(removed)*1.2 + float64(changed)*1.5) / float64(total) * 100
	return round2(value)
}

func riskLevel(score float64, entries []DiffEntry) string {
	for _, entry := range entries {
		if entry.ImpactLevel == "high" {
			return "high"
		}
	}
	switch {
	case score >= 60:
		return "high"
	case score >= 25:
		return "medium"
	case score > 0:
		return "low"
	default:
		return "none"
	}
}

func diffRecommendations(risk string, entries []DiffEntry) []string {
	if len(entries) == 0 {
		return []string{"No memory drift detected between the selected snapshots."}
	}
	recs := []string{"Review changed memory keys before accepting the newer state."}
	if risk == "high" {
		recs = append(recs, "High-impact memory keys changed; require approval before rollback or promotion.")
	}
	return recs
}

func normalizePolicy(policy RetentionPolicy) RetentionPolicy {
	if policy.RetentionDays <= 0 {
		policy.RetentionDays = 30
	}
	if policy.MaxVersionsPerKey <= 0 {
		policy.MaxVersionsPerKey = 100
	}
	if policy.MaxSnapshotBytes <= 0 {
		policy.MaxSnapshotBytes = 256 * 1024
	}
	if policy.MaxKeysPerSnapshot <= 0 {
		policy.MaxKeysPerSnapshot = 256
	}
	if policy.EvidenceMaxSnapshots <= 0 {
		policy.EvidenceMaxSnapshots = 20
	}
	return policy
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func round2(value float64) float64 {
	return float64(int(value*100+0.5)) / 100
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]any{"error": message})
}
