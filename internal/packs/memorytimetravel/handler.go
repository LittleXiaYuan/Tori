// Package memorytimetravel contains the backend implementation for the built-in
// Memory Time Travel capability pack. The first delivery is intentionally a pack
// shell: it owns manifest-gated HTTP routes, versioned memory snapshot storage,
// point-in-time reconstruction, drift diff summaries, rollback plans, approved
// rollback write-back planning, retention dry-run planning, JSON evidence
// export, read-only Merkle audit-chain verification, and a conservative KV
// audit proof-link schema placeholder plus native kv_history table/index/
// migration planning, read-only migration row previews, dual-read parity
// validation against the reserved adapter, cutover readiness gate reporting,
// and non-destructive dual-read/dual-write cutover planning while native Ledger
// KV kv_history write-back remains a later slice.
package memorytimetravel

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"io"
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
	NativeKVHistoryPreviewer NativeKVHistoryPreviewer
	MemoryPersisterWriteback bool
	MerkleVerifier           MerkleVerifier
}

// Handler serves the Memory Time Travel pack API surface.
type Handler struct {
	dataDir                  string
	now                      func() time.Time
	policy                   RetentionPolicy
	temporalKV               TemporalKVReader
	nativeKVHistoryPreviewer NativeKVHistoryPreviewer
	memoryPersisterWriteback bool
	merkleVerifier           MerkleVerifier
}

// TemporalKVReader is the narrow Memory Time Travel dependency on Ledger KV
// history. It lets the pack read versioned memory snapshots without importing
// the concrete Ledger implementation into the pack shell.
type TemporalKVReader interface {
	SnapshotRawAt(ctx context.Context, namespace string, at time.Time) (map[string][]byte, error)
}

// NativeKVHistoryPreviewer is the narrow pack-local dependency for expanding
// reserved TemporalKV history documents into future native kv_history row
// previews without importing the concrete Ledger implementation here.
type NativeKVHistoryPreviewer interface {
	PreviewNativeKVHistoryRows(ctx context.Context, namespace string, limit int) (NativeKVHistoryMigrationPreview, error)
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
	RetentionDays            int `json:"retention_days"`
	MaxVersionsPerKey        int `json:"max_versions_per_key"`
	MaxSnapshotsPerNamespace int `json:"max_snapshots_per_namespace"`
	MaxSnapshotBytes         int `json:"max_snapshot_bytes"`
	MaxKeysPerSnapshot       int `json:"max_keys_per_snapshot"`
	EvidenceMaxSnapshots     int `json:"evidence_max_snapshots"`
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

type RollbackExecutionPlanRequest struct {
	Namespace   string `json:"namespace,omitempty"`
	SnapshotID  string `json:"snapshot_id"`
	RequestedBy string `json:"requested_by,omitempty"`
	Reason      string `json:"reason,omitempty"`
	ApprovalID  string `json:"approval_id,omitempty"`
	DryRun      bool   `json:"dry_run,omitempty"`
}

type RollbackWritebackActionPlan struct {
	Operation        string    `json:"operation"`
	Namespace        string    `json:"namespace"`
	Key              string    `json:"key"`
	ValueHash        string    `json:"value_hash,omitempty"`
	ValueBytes       int       `json:"value_bytes,omitempty"`
	TargetSnapshotID string    `json:"target_snapshot_id"`
	TemporalVersion  int       `json:"temporal_version"`
	AuditAction      string    `json:"audit_action"`
	RequiresApproval bool      `json:"requires_approval"`
	ApprovalID       string    `json:"approval_id,omitempty"`
	GeneratedAt      time.Time `json:"generated_at"`
}

type GlobalApprovalRequestPlan struct {
	RequestID                   string         `json:"request_id"`
	RequestKey                  string         `json:"request_key"`
	TaskID                      string         `json:"task_id,omitempty"`
	WorkflowID                  string         `json:"workflow_id,omitempty"`
	StepIndex                   int            `json:"step_index,omitempty"`
	QueueName                   string         `json:"queue_name"`
	Category                    string         `json:"category"`
	RiskLevel                   string         `json:"risk_level"`
	Summary                     string         `json:"summary"`
	Details                     map[string]any `json:"details"`
	Requester                   string         `json:"requester"`
	TenantID                    string         `json:"tenant_id,omitempty"`
	Reason                      string         `json:"reason"`
	RequiredFields              []string       `json:"required_fields"`
	DecisionStates              []string       `json:"decision_states"`
	ApprovalManagerEnqueueReady bool           `json:"approval_manager_enqueue_ready"`
	GlobalApprovalEnqueueReady  bool           `json:"global_approval_enqueue_ready"`
	ActionReleaseReady          bool           `json:"action_release_ready"`
	SourceStore                 string         `json:"source_store"`
	SourceArtifact              string         `json:"source_artifact"`
	Payload                     map[string]any `json:"payload"`
	Notes                       []string       `json:"notes,omitempty"`
}

type ApprovedRollbackExecutionPlanReport struct {
	PackID                         string                        `json:"pack_id"`
	GeneratedAt                    time.Time                     `json:"generated_at"`
	Stage                          string                        `json:"stage"`
	Status                         string                        `json:"status"`
	Namespace                      string                        `json:"namespace"`
	SnapshotID                     string                        `json:"snapshot_id"`
	RequestedBy                    string                        `json:"requested_by,omitempty"`
	Reason                         string                        `json:"reason,omitempty"`
	ApprovalID                     string                        `json:"approval_id,omitempty"`
	DryRun                         bool                          `json:"dry_run"`
	ApprovalRequired               bool                          `json:"approval_required"`
	ApprovalRequestPlanReady       bool                          `json:"approval_request_plan_ready"`
	ApprovalManagerBridgePlanReady bool                          `json:"approval_manager_bridge_plan_ready"`
	GlobalApprovalEnqueueReady     bool                          `json:"global_approval_enqueue_ready"`
	ApprovedRollbackPlanReady      bool                          `json:"approved_rollback_plan_ready"`
	RollbackWritebackPlanReady     bool                          `json:"rollback_writeback_plan_ready"`
	RollbackWritebackReady         bool                          `json:"rollback_writeback_ready"`
	WritesLedgerKV                 bool                          `json:"writes_ledger_kv"`
	WritesTemporalKV               bool                          `json:"writes_temporal_kv"`
	MerkleAppendReady              bool                          `json:"merkle_append_ready"`
	AuditProofLinkReady            bool                          `json:"audit_proof_link_ready"`
	ActionCount                    int                           `json:"action_count"`
	PreviewValues                  map[string]string             `json:"preview_values,omitempty"`
	RollbackPlan                   RollbackPlan                  `json:"rollback_plan"`
	ProposedApprovalRequest        GlobalApprovalRequestPlan     `json:"proposed_approval_request"`
	WritebackActions               []RollbackWritebackActionPlan `json:"writeback_actions"`
	Artifacts                      []string                      `json:"artifacts"`
	Actions                        []string                      `json:"actions"`
	Labels                         []string                      `json:"labels"`
	Notes                          []string                      `json:"notes,omitempty"`
}

type RetentionCandidate struct {
	ID        string    `json:"id"`
	Namespace string    `json:"namespace"`
	CreatedAt time.Time `json:"created_at"`
	Hash      string    `json:"hash"`
	SizeBytes int       `json:"size_bytes"`
	KeyCount  int       `json:"key_count"`
	Reasons   []string  `json:"reasons"`
	Action    string    `json:"action"`
}

type RetentionPlanReport struct {
	PackID                   string               `json:"pack_id"`
	Namespace                string               `json:"namespace"`
	GeneratedAt              time.Time            `json:"generated_at"`
	DryRun                   bool                 `json:"dry_run"`
	Status                   string               `json:"status"`
	Policy                   RetentionPolicy      `json:"policy"`
	CutoffAt                 time.Time            `json:"cutoff_at"`
	Scopes                   []string             `json:"scopes"`
	MaxSnapshotsPerNamespace int                  `json:"max_snapshots_per_namespace"`
	SnapshotCount            int                  `json:"snapshot_count"`
	KeepCount                int                  `json:"keep_count"`
	CandidateCount           int                  `json:"candidate_count"`
	ReclaimableBytes         int                  `json:"reclaimable_bytes"`
	TemporalHistoryReady     bool                 `json:"temporal_history_ready"`
	TemporalPruneReady       bool                 `json:"temporal_prune_ready"`
	Candidates               []RetentionCandidate `json:"candidates"`
	Actions                  []string             `json:"actions"`
	Notes                    []string             `json:"notes,omitempty"`
}

type RetentionPrunePlanRequest struct {
	Namespace    string   `json:"namespace,omitempty"`
	CandidateIDs []string `json:"candidate_ids,omitempty"`
	Reason       string   `json:"reason,omitempty"`
	RequestedBy  string   `json:"requested_by,omitempty"`
	DryRun       bool     `json:"dry_run,omitempty"`
}

type RetentionPrunePlanReport struct {
	PackID                   string               `json:"pack_id"`
	Namespace                string               `json:"namespace"`
	GeneratedAt              time.Time            `json:"generated_at"`
	DryRun                   bool                 `json:"dry_run"`
	Status                   string               `json:"status"`
	ApprovalRequired         bool                 `json:"approval_required"`
	PruneReady               bool                 `json:"prune_ready"`
	TemporalPruneReady       bool                 `json:"temporal_prune_ready"`
	CandidateCount           int                  `json:"candidate_count"`
	SelectedCandidateCount   int                  `json:"selected_candidate_count"`
	ReclaimableBytes         int                  `json:"reclaimable_bytes"`
	ActionCount              int                  `json:"action_count"`
	RequestedBy              string               `json:"requested_by,omitempty"`
	Reason                   string               `json:"reason,omitempty"`
	RetentionPlanGeneratedAt time.Time            `json:"retention_plan_generated_at"`
	Candidates               []RetentionCandidate `json:"candidates"`
	Actions                  []string             `json:"actions"`
	Notes                    []string             `json:"notes,omitempty"`
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

type KVAuditProofLink struct {
	Namespace   string    `json:"namespace"`
	Key         string    `json:"key"`
	SnapshotID  string    `json:"snapshot_id,omitempty"`
	KVVersionAt time.Time `json:"kv_version_at,omitempty"`
	ValueHash   string    `json:"value_hash,omitempty"`
	AuditSeq    uint64    `json:"audit_seq,omitempty"`
	AuditHash   string    `json:"audit_hash,omitempty"`
	ProofStatus string    `json:"proof_status"`
	Notes       []string  `json:"notes,omitempty"`
}

type KVAuditLinksReport struct {
	PackID                  string             `json:"pack_id"`
	Namespace               string             `json:"namespace"`
	GeneratedAt             time.Time          `json:"generated_at"`
	SchemaReady             bool               `json:"schema_ready"`
	LinkageReady            bool               `json:"linkage_ready"`
	NativeKVHistoryReady    bool               `json:"native_kv_history_ready"`
	MerkleVerificationReady bool               `json:"merkle_verification_ready"`
	Source                  string             `json:"source"`
	KVAuditLinks            []KVAuditProofLink `json:"kv_audit_links"`
	RequiredFields          []string           `json:"required_fields"`
	Notes                   []string           `json:"notes,omitempty"`
}

type NativeKVHistoryColumnPlan struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	Nullable bool   `json:"nullable"`
	Purpose  string `json:"purpose"`
}

type NativeKVHistoryIndexPlan struct {
	Name    string   `json:"name"`
	Columns []string `json:"columns"`
	Unique  bool     `json:"unique"`
	Purpose string   `json:"purpose"`
}

type KVHistoryMigrationStepPlan struct {
	Step        int    `json:"step"`
	Name        string `json:"name"`
	From        string `json:"from"`
	To          string `json:"to"`
	DryRun      bool   `json:"dry_run"`
	Writes      bool   `json:"writes"`
	Status      string `json:"status"`
	Description string `json:"description"`
}

type NativeKVHistoryRowPreview struct {
	ID            string    `json:"id"`
	Namespace     string    `json:"namespace"`
	Key           string    `json:"key"`
	Version       int       `json:"version"`
	Value         []byte    `json:"value_base64"`
	ValueSHA256   string    `json:"value_sha256"`
	UpdatedAt     time.Time `json:"updated_at"`
	ArchivedAt    time.Time `json:"archived_at,omitempty"`
	Current       bool      `json:"current"`
	AuditSeq      uint64    `json:"audit_seq,omitempty"`
	AuditHash     string    `json:"audit_hash,omitempty"`
	SourceAdapter string    `json:"source_adapter"`
}

type NativeKVHistoryMigrationPreview struct {
	PackID                      string                      `json:"pack_id,omitempty"`
	Namespace                   string                      `json:"namespace"`
	GeneratedAt                 time.Time                   `json:"generated_at"`
	Stage                       string                      `json:"stage,omitempty"`
	Status                      string                      `json:"status,omitempty"`
	SourceNamespace             string                      `json:"source_namespace"`
	NativeTable                 string                      `json:"native_table"`
	ScannedDocumentCount        int                         `json:"scanned_document_count"`
	PreviewRowCount             int                         `json:"preview_row_count"`
	ReturnedRowCount            int                         `json:"returned_row_count"`
	Limit                       int                         `json:"limit,omitempty"`
	NativeKVHistoryPreviewReady bool                        `json:"native_kv_history_preview_ready"`
	WritesNativeKVHistory       bool                        `json:"writes_native_kv_history"`
	MigratesKVHistory           bool                        `json:"migrates_kv_history"`
	UsesReservedKVNamespace     bool                        `json:"uses_reserved_kv_namespace"`
	Rows                        []NativeKVHistoryRowPreview `json:"rows"`
	Artifacts                   []string                    `json:"artifacts,omitempty"`
	Actions                     []string                    `json:"actions,omitempty"`
	Labels                      []string                    `json:"labels,omitempty"`
	Notes                       []string                    `json:"notes,omitempty"`
}

type NativeKVHistoryPlanReport struct {
	PackID                      string                       `json:"pack_id"`
	Namespace                   string                       `json:"namespace"`
	GeneratedAt                 time.Time                    `json:"generated_at"`
	Stage                       string                       `json:"stage"`
	Status                      string                       `json:"status"`
	Source                      string                       `json:"source"`
	CurrentAdapter              string                       `json:"current_adapter"`
	CurrentHistoryNamespace     string                       `json:"current_history_namespace"`
	NativeTable                 string                       `json:"native_table"`
	TemporalKVAdapterReady      bool                         `json:"temporal_kv_adapter_ready"`
	NativeKVHistoryPlanReady    bool                         `json:"native_kv_history_plan_ready"`
	KVHistoryMigrationPlanReady bool                         `json:"kv_history_migration_plan_ready"`
	KVHistoryIndexPlanReady     bool                         `json:"kv_history_index_plan_ready"`
	NativeKVHistoryReady        bool                         `json:"native_kv_history_ready"`
	WritesNativeKVHistory       bool                         `json:"writes_native_kv_history"`
	MigratesKVHistory           bool                         `json:"migrates_kv_history"`
	UsesReservedKVNamespace     bool                         `json:"uses_reserved_kv_namespace"`
	SnapshotStoreReady          bool                         `json:"snapshot_store_ready"`
	RetentionPlanReady          bool                         `json:"retention_plan_ready"`
	AuditProofLinkSchemaReady   bool                         `json:"audit_proof_link_schema_ready"`
	SchemaPlan                  []NativeKVHistoryColumnPlan  `json:"schema_plan"`
	KVHistoryIndexPlan          []NativeKVHistoryIndexPlan   `json:"kv_history_index_plan"`
	KVHistoryMigrationPlan      []KVHistoryMigrationStepPlan `json:"kv_history_migration_plan"`
	Artifacts                   []string                     `json:"artifacts"`
	Actions                     []string                     `json:"actions"`
	BlockedBy                   []string                     `json:"blocked_by"`
	Labels                      []string                     `json:"labels"`
	Notes                       []string                     `json:"notes,omitempty"`
}

type KVHistoryCutoverPlanRequest struct {
	Namespace   string `json:"namespace,omitempty"`
	RequestedBy string `json:"requested_by,omitempty"`
	Reason      string `json:"reason,omitempty"`
	Limit       int    `json:"limit,omitempty"`
	DryRun      bool   `json:"dry_run,omitempty"`
}

type KVHistoryCutoverPhasePlan struct {
	Step        int      `json:"step"`
	Name        string   `json:"name"`
	From        string   `json:"from"`
	To          string   `json:"to"`
	Gate        string   `json:"gate"`
	Ready       bool     `json:"ready"`
	Writes      bool     `json:"writes"`
	Status      string   `json:"status"`
	Description string   `json:"description"`
	BlockedBy   []string `json:"blocked_by,omitempty"`
}

type KVHistoryDualReadPlan struct {
	PlanReady                bool     `json:"plan_ready"`
	Ready                    bool     `json:"ready"`
	PreferredSource          string   `json:"preferred_source"`
	FallbackSource           string   `json:"fallback_source"`
	ReadsNativeKVHistory     bool     `json:"reads_native_kv_history"`
	ReadsReservedKVNamespace bool     `json:"reads_reserved_kv_namespace"`
	SwitchesAdapter          bool     `json:"switches_adapter"`
	Validation               []string `json:"validation"`
	BlockedBy                []string `json:"blocked_by"`
	Notes                    []string `json:"notes,omitempty"`
}

type KVHistoryDualWritePlan struct {
	PlanReady                 bool     `json:"plan_ready"`
	Ready                     bool     `json:"ready"`
	PrimaryTarget             string   `json:"primary_target"`
	MirrorTarget              string   `json:"mirror_target"`
	WritesNativeKVHistory     bool     `json:"writes_native_kv_history"`
	WritesReservedKVNamespace bool     `json:"writes_reserved_kv_namespace"`
	WritesLedgerKV            bool     `json:"writes_ledger_kv"`
	MigrationExecutorReady    bool     `json:"migration_executor_ready"`
	Guardrails                []string `json:"guardrails"`
	BlockedBy                 []string `json:"blocked_by"`
	Notes                     []string `json:"notes,omitempty"`
}

type KVHistoryCutoverRollbackPlan struct {
	PlanReady                  bool     `json:"plan_ready"`
	Ready                      bool     `json:"ready"`
	RequiresApproval           bool     `json:"requires_approval"`
	RestoresReservedAdapter    bool     `json:"restores_reserved_adapter"`
	DropsNativeRows            bool     `json:"drops_native_rows"`
	DeletesReservedKVNamespace bool     `json:"deletes_reserved_kv_namespace"`
	Actions                    []string `json:"actions"`
	BlockedBy                  []string `json:"blocked_by"`
	Notes                      []string `json:"notes,omitempty"`
}

type KVHistoryCutoverPlanReport struct {
	PackID                      string                          `json:"pack_id"`
	Namespace                   string                          `json:"namespace"`
	GeneratedAt                 time.Time                       `json:"generated_at"`
	Stage                       string                          `json:"stage"`
	Status                      string                          `json:"status"`
	DryRun                      bool                            `json:"dry_run"`
	RequestedBy                 string                          `json:"requested_by,omitempty"`
	Reason                      string                          `json:"reason,omitempty"`
	Source                      string                          `json:"source"`
	NativeTable                 string                          `json:"native_table"`
	CurrentHistoryNamespace     string                          `json:"current_history_namespace"`
	ConsumesNativeKVHistoryPlan bool                            `json:"consumes_native_kv_history_plan"`
	ConsumesMigrationPreview    bool                            `json:"consumes_migration_preview"`
	NativeKVHistoryPlanReady    bool                            `json:"native_kv_history_plan_ready"`
	NativeKVHistoryPreviewReady bool                            `json:"native_kv_history_preview_ready"`
	KVHistoryCutoverPlanReady   bool                            `json:"kv_history_cutover_plan_ready"`
	DualReadPlanReady           bool                            `json:"dual_read_plan_ready"`
	DualWritePlanReady          bool                            `json:"dual_write_plan_ready"`
	NativeKVHistoryReady        bool                            `json:"native_kv_history_ready"`
	WritesNativeKVHistory       bool                            `json:"writes_native_kv_history"`
	MigratesKVHistory           bool                            `json:"migrates_kv_history"`
	DualReadReady               bool                            `json:"dual_read_ready"`
	DualWriteReady              bool                            `json:"dual_write_ready"`
	CutoverReady                bool                            `json:"cutover_ready"`
	RollbackReady               bool                            `json:"rollback_ready"`
	CreatesNativeTable          bool                            `json:"creates_native_table"`
	DeletesReservedKVNamespace  bool                            `json:"deletes_reserved_kv_namespace"`
	SwitchesTemporalAdapter     bool                            `json:"switches_temporal_adapter"`
	PreviewRowCount             int                             `json:"preview_row_count"`
	ReturnedPreviewRowCount     int                             `json:"returned_preview_row_count"`
	NativeKVHistoryPlan         NativeKVHistoryPlanReport       `json:"native_kv_history_plan"`
	KVHistoryMigrationPreview   NativeKVHistoryMigrationPreview `json:"kv_history_migration_preview"`
	Phases                      []KVHistoryCutoverPhasePlan     `json:"phases"`
	DualReadPlan                KVHistoryDualReadPlan           `json:"dual_read_plan"`
	DualWritePlan               KVHistoryDualWritePlan          `json:"dual_write_plan"`
	CutoverRollbackPlan         KVHistoryCutoverRollbackPlan    `json:"cutover_rollback_plan"`
	Artifacts                   []string                        `json:"artifacts"`
	Actions                     []string                        `json:"actions"`
	BlockedBy                   []string                        `json:"blocked_by"`
	Labels                      []string                        `json:"labels"`
	Notes                       []string                        `json:"notes,omitempty"`
}

type KVHistoryCutoverReadinessRequest struct {
	Namespace   string    `json:"namespace,omitempty"`
	At          time.Time `json:"at,omitempty"`
	RequestedBy string    `json:"requested_by,omitempty"`
	Reason      string    `json:"reason,omitempty"`
	Limit       int       `json:"limit,omitempty"`
	DryRun      bool      `json:"dry_run,omitempty"`
}

type KVHistoryCutoverReadinessGate struct {
	Name        string   `json:"name"`
	Ready       bool     `json:"ready"`
	Required    bool     `json:"required"`
	Status      string   `json:"status"`
	Evidence    []string `json:"evidence"`
	BlockedBy   []string `json:"blocked_by,omitempty"`
	Description string   `json:"description"`
}

type KVHistoryCutoverReadinessReport struct {
	PackID                      string                          `json:"pack_id"`
	Namespace                   string                          `json:"namespace"`
	TemporalNamespace           string                          `json:"temporal_namespace"`
	GeneratedAt                 time.Time                       `json:"generated_at"`
	At                          time.Time                       `json:"at"`
	Stage                       string                          `json:"stage"`
	Status                      string                          `json:"status"`
	DryRun                      bool                            `json:"dry_run"`
	RequestedBy                 string                          `json:"requested_by,omitempty"`
	Reason                      string                          `json:"reason,omitempty"`
	Source                      string                          `json:"source"`
	NativeTable                 string                          `json:"native_table"`
	CurrentHistoryNamespace     string                          `json:"current_history_namespace"`
	CutoverReadinessCheckReady  bool                            `json:"cutover_readiness_check_ready"`
	CutoverReady                bool                            `json:"cutover_ready"`
	NativeKVHistoryPlanReady    bool                            `json:"native_kv_history_plan_ready"`
	NativeKVHistoryPreviewReady bool                            `json:"native_kv_history_preview_ready"`
	DualReadParityCheckReady    bool                            `json:"dual_read_parity_check_ready"`
	DualReadParityReady         bool                            `json:"dual_read_parity_ready"`
	ParityPassed                bool                            `json:"parity_passed"`
	PreviewComplete             bool                            `json:"preview_complete"`
	MigrationExecutorReady      bool                            `json:"migration_executor_ready"`
	NativeReadAdapterReady      bool                            `json:"native_read_adapter_ready"`
	NativeWritePathReady        bool                            `json:"native_write_path_ready"`
	ApprovalManagerReady        bool                            `json:"approval_manager_ready"`
	RollbackExecutorReady       bool                            `json:"rollback_executor_ready"`
	AuditProofLinkReady         bool                            `json:"audit_proof_link_ready"`
	SwitchesTemporalAdapter     bool                            `json:"switches_temporal_adapter"`
	WritesLedgerKV              bool                            `json:"writes_ledger_kv"`
	WritesNativeKVHistory       bool                            `json:"writes_native_kv_history"`
	ConsumesCutoverPlan         bool                            `json:"consumes_cutover_plan"`
	ConsumesDualReadParity      bool                            `json:"consumes_dual_read_parity"`
	RequiredGateCount           int                             `json:"required_gate_count"`
	PassedGateCount             int                             `json:"passed_gate_count"`
	BlockedGateCount            int                             `json:"blocked_gate_count"`
	Gates                       []KVHistoryCutoverReadinessGate `json:"gates"`
	CutoverPlan                 KVHistoryCutoverPlanReport      `json:"cutover_plan"`
	DualReadParity              KVHistoryDualReadParityReport   `json:"dual_read_parity"`
	Artifacts                   []string                        `json:"artifacts"`
	Actions                     []string                        `json:"actions"`
	BlockedBy                   []string                        `json:"blocked_by"`
	Labels                      []string                        `json:"labels"`
	Notes                       []string                        `json:"notes,omitempty"`
}

type KVHistoryDualReadParityRequest struct {
	Namespace string    `json:"namespace,omitempty"`
	At        time.Time `json:"at,omitempty"`
	Limit     int       `json:"limit,omitempty"`
}

type KVHistoryDualReadParityMismatch struct {
	Key                string `json:"key"`
	Kind               string `json:"kind"`
	ReservedValue      string `json:"reserved_value,omitempty"`
	NativePreviewValue string `json:"native_preview_value,omitempty"`
	ReservedHash       string `json:"reserved_hash,omitempty"`
	NativePreviewHash  string `json:"native_preview_hash,omitempty"`
}

type KVHistoryDualReadParityReport struct {
	PackID                      string                            `json:"pack_id"`
	Namespace                   string                            `json:"namespace"`
	TemporalNamespace           string                            `json:"temporal_namespace"`
	GeneratedAt                 time.Time                         `json:"generated_at"`
	At                          time.Time                         `json:"at"`
	Stage                       string                            `json:"stage"`
	Status                      string                            `json:"status"`
	Source                      string                            `json:"source"`
	NativeTable                 string                            `json:"native_table"`
	CurrentHistoryNamespace     string                            `json:"current_history_namespace"`
	Limit                       int                               `json:"limit"`
	PreviewRowCount             int                               `json:"preview_row_count"`
	ReturnedPreviewRowCount     int                               `json:"returned_preview_row_count"`
	TemporalKeyCount            int                               `json:"temporal_key_count"`
	NativePreviewKeyCount       int                               `json:"native_preview_key_count"`
	MatchedKeyCount             int                               `json:"matched_key_count"`
	MismatchCount               int                               `json:"mismatch_count"`
	MissingFromNativeCount      int                               `json:"missing_from_native_count"`
	ExtraInNativeCount          int                               `json:"extra_in_native_count"`
	ValueMismatchCount          int                               `json:"value_mismatch_count"`
	DualReadParityCheckReady    bool                              `json:"dual_read_parity_check_ready"`
	DualReadParityReady         bool                              `json:"dual_read_parity_ready"`
	ParityPassed                bool                              `json:"parity_passed"`
	PreviewComplete             bool                              `json:"preview_complete"`
	ReadsTemporalKV             bool                              `json:"reads_temporal_kv"`
	ReadsNativeKVHistory        bool                              `json:"reads_native_kv_history"`
	ReadsNativeKVHistoryPreview bool                              `json:"reads_native_kv_history_preview"`
	SwitchesTemporalAdapter     bool                              `json:"switches_temporal_adapter"`
	WritesLedgerKV              bool                              `json:"writes_ledger_kv"`
	WritesNativeKVHistory       bool                              `json:"writes_native_kv_history"`
	KVHistoryMigrationPreview   NativeKVHistoryMigrationPreview   `json:"kv_history_migration_preview"`
	Mismatches                  []KVHistoryDualReadParityMismatch `json:"mismatches"`
	Artifacts                   []string                          `json:"artifacts"`
	Actions                     []string                          `json:"actions"`
	BlockedBy                   []string                          `json:"blocked_by"`
	Labels                      []string                          `json:"labels"`
	Notes                       []string                          `json:"notes,omitempty"`
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
		nativeKVHistoryPreviewer: cfg.NativeKVHistoryPreviewer,
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
		{Method: http.MethodPost, Path: "/v1/memory-time-travel/rollback/approved-plan", Handler: h.ApprovedRollbackPlan},
		{Method: http.MethodGet, Path: "/v1/memory-time-travel/retention/plan", Handler: h.RetentionPlan},
		{Method: http.MethodPost, Path: "/v1/memory-time-travel/retention/prune-plan", Handler: h.RetentionPrunePlan},
		{Method: http.MethodGet, Path: "/v1/memory-time-travel/kv-history/native-plan", Handler: h.NativeKVHistoryPlan},
		{Method: http.MethodGet, Path: "/v1/memory-time-travel/kv-history/migration-preview", Handler: h.NativeKVHistoryMigrationPreview},
		{Method: http.MethodPost, Path: "/v1/memory-time-travel/kv-history/dual-read/parity", Handler: h.KVHistoryDualReadParity},
		{Method: http.MethodPost, Path: "/v1/memory-time-travel/kv-history/cutover/plan", Handler: h.KVHistoryCutoverPlan},
		{Method: http.MethodPost, Path: "/v1/memory-time-travel/kv-history/cutover/readiness", Handler: h.KVHistoryCutoverReadiness},
		{Method: http.MethodGet, Path: "/v1/memory-time-travel/audit/links", Handler: h.AuditLinks},
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
		"memory.rollback.approved_plan",
		"memory.rollback.writeback.plan",
		"memory.retention.plan",
		"memory.retention.prune_plan",
		"memory.kv_history.native_plan",
		"memory.kv_history.migration_preview",
		"memory.kv_history.dual_read.parity",
		"memory.kv_history.cutover.plan",
		"memory.kv_history.cutover.readiness",
		"memory.audit.links.schema",
		"memory.evidence.export",
	}
	if h.merkleVerifier != nil {
		capabilities = append(capabilities, "memory.audit.verify")
	}
	writeJSON(w, http.StatusOK, statusResponse{
		PackID:                         PackID,
		Stage:                          "pack-shell-before-ledger-kv-history",
		SnapshotStoreReady:             true,
		TemporalQueryReady:             true,
		LedgerHistoryReady:             h.temporalKV != nil,
		TemporalKVAdapterReady:         true,
		NativeKVHistoryPlanReady:       true,
		KVHistoryMigrationPlanReady:    true,
		KVHistoryIndexPlanReady:        true,
		NativeKVHistoryPreviewReady:    h.nativeKVHistoryPreviewer != nil,
		KVHistoryCutoverPlanReady:      true,
		KVHistoryCutoverReadinessReady: true,
		DualReadPlanReady:              true,
		DualReadParityCheckReady:       h.temporalKV != nil && h.nativeKVHistoryPreviewer != nil,
		DualWritePlanReady:             true,
		NativeKVHistoryReady:           false,
		WritesNativeKVHistory:          false,
		MigratesKVHistory:              false,
		DualReadReady:                  false,
		DualWriteReady:                 false,
		CutoverReady:                   false,
		RollbackReady:                  false,
		MerkleVerificationReady:        h.merkleVerifier != nil,
		MemoryPersisterWritebackReady:  h.memoryPersisterWriteback,
		ApprovedRollbackPlanReady:      true,
		ApprovalRequestPlanReady:       true,
		ApprovalManagerBridgePlanReady: true,
		GlobalApprovalEnqueueReady:     false,
		RollbackWritebackPlanReady:     true,
		RollbackWritebackReady:         false,
		WritesLedgerKV:                 false,
		WritesTemporalKV:               false,
		RetentionPlanReady:             true,
		RetentionPrunePlanReady:        true,
		RetentionPruneReady:            false,
		KVAuditLinkSchemaReady:         true,
		KVAuditLinkageReady:            false,
		SnapshotCount:                  len(snapshots),
		NamespaceCount:                 len(namespaces),
		StoreDir:                       h.dataDir,
		Policy:                         h.policy,
		LastSnapshot:                   firstSnapshot(snapshots),
		Capabilities:                   capabilities,
		Notes:                          h.statusNotes(),
	})
}

type statusResponse struct {
	PackID                         string           `json:"pack_id"`
	Stage                          string           `json:"stage"`
	SnapshotStoreReady             bool             `json:"snapshot_store_ready"`
	TemporalQueryReady             bool             `json:"temporal_query_ready"`
	LedgerHistoryReady             bool             `json:"ledger_history_ready"`
	TemporalKVAdapterReady         bool             `json:"temporal_kv_adapter_ready"`
	NativeKVHistoryPlanReady       bool             `json:"native_kv_history_plan_ready"`
	KVHistoryMigrationPlanReady    bool             `json:"kv_history_migration_plan_ready"`
	KVHistoryIndexPlanReady        bool             `json:"kv_history_index_plan_ready"`
	NativeKVHistoryPreviewReady    bool             `json:"native_kv_history_preview_ready"`
	KVHistoryCutoverPlanReady      bool             `json:"kv_history_cutover_plan_ready"`
	KVHistoryCutoverReadinessReady bool             `json:"kv_history_cutover_readiness_ready"`
	DualReadPlanReady              bool             `json:"dual_read_plan_ready"`
	DualReadParityCheckReady       bool             `json:"dual_read_parity_check_ready"`
	DualWritePlanReady             bool             `json:"dual_write_plan_ready"`
	NativeKVHistoryReady           bool             `json:"native_kv_history_ready"`
	WritesNativeKVHistory          bool             `json:"writes_native_kv_history"`
	MigratesKVHistory              bool             `json:"migrates_kv_history"`
	DualReadReady                  bool             `json:"dual_read_ready"`
	DualWriteReady                 bool             `json:"dual_write_ready"`
	CutoverReady                   bool             `json:"cutover_ready"`
	RollbackReady                  bool             `json:"rollback_ready"`
	MerkleVerificationReady        bool             `json:"merkle_verification_ready"`
	MemoryPersisterWritebackReady  bool             `json:"memory_persister_writeback_ready"`
	ApprovedRollbackPlanReady      bool             `json:"approved_rollback_plan_ready"`
	ApprovalRequestPlanReady       bool             `json:"approval_request_plan_ready"`
	ApprovalManagerBridgePlanReady bool             `json:"approval_manager_bridge_plan_ready"`
	GlobalApprovalEnqueueReady     bool             `json:"global_approval_enqueue_ready"`
	RollbackWritebackPlanReady     bool             `json:"rollback_writeback_plan_ready"`
	RollbackWritebackReady         bool             `json:"rollback_writeback_ready"`
	WritesLedgerKV                 bool             `json:"writes_ledger_kv"`
	WritesTemporalKV               bool             `json:"writes_temporal_kv"`
	RetentionPlanReady             bool             `json:"retention_plan_ready"`
	RetentionPrunePlanReady        bool             `json:"retention_prune_plan_ready"`
	RetentionPruneReady            bool             `json:"retention_prune_ready"`
	KVAuditLinkSchemaReady         bool             `json:"kv_audit_link_schema_ready"`
	KVAuditLinkageReady            bool             `json:"kv_audit_linkage_ready"`
	SnapshotCount                  int              `json:"snapshot_count"`
	NamespaceCount                 int              `json:"namespace_count"`
	StoreDir                       string           `json:"store_dir"`
	Policy                         RetentionPolicy  `json:"policy"`
	LastSnapshot                   *SnapshotSummary `json:"last_snapshot"`
	Capabilities                   []string         `json:"capabilities"`
	Notes                          []string         `json:"notes"`
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
		values, err := h.temporalSnapshotValues(r.Context(), temporalNamespaceFor(namespace), at)
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

func (h *Handler) ApprovedRollbackPlan(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var req RollbackExecutionPlanRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid approved rollback plan payload")
		return
	}
	plan, err := h.buildApprovedRollbackPlan(req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"plan": plan})
}

func (h *Handler) RetentionPlan(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	plan, err := h.buildRetentionPlan(r.URL.Query().Get("namespace"))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"plan": plan})
}

func (h *Handler) RetentionPrunePlan(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var req RetentionPrunePlanRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid retention prune-plan payload")
		return
	}
	plan, err := h.buildRetentionPrunePlan(req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"plan": plan})
}

func (h *Handler) NativeKVHistoryPlan(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	report := h.buildNativeKVHistoryPlan(r.URL.Query().Get("namespace"))
	writeJSON(w, http.StatusOK, map[string]any{"plan": report})
}

func (h *Handler) NativeKVHistoryMigrationPreview(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	namespace := normalizeNamespace(r.URL.Query().Get("namespace"))
	limit := parsePreviewLimit(r.URL.Query().Get("limit"))
	preview, err := h.buildNativeKVHistoryMigrationPreview(r.Context(), namespace, limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"kv_history_migration_preview": preview})
}

func (h *Handler) KVHistoryDualReadParity(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var req KVHistoryDualReadParityRequest
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid kv_history dual-read parity payload")
		return
	}
	if strings.TrimSpace(string(body)) != "" {
		if err := json.Unmarshal(body, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid kv_history dual-read parity payload")
			return
		}
	}
	report, err := h.buildKVHistoryDualReadParity(r.Context(), req)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"parity": report})
}

func (h *Handler) buildNativeKVHistoryMigrationPreview(ctx context.Context, namespace string, limit int) (NativeKVHistoryMigrationPreview, error) {
	namespace = normalizeNamespace(namespace)
	if limit <= 0 {
		limit = 50
	}
	if h.nativeKVHistoryPreviewer == nil {
		return NativeKVHistoryMigrationPreview{
			PackID:                      PackID,
			Namespace:                   namespace,
			GeneratedAt:                 h.now().UTC(),
			Stage:                       "native-kv-history-migration-preview-before-native-write",
			Status:                      "not_attached",
			SourceNamespace:             "__kv_history__",
			NativeTable:                 "kv_history",
			Limit:                       limit,
			NativeKVHistoryPreviewReady: false,
			WritesNativeKVHistory:       false,
			MigratesKVHistory:           false,
			UsesReservedKVNamespace:     true,
			Rows:                        []NativeKVHistoryRowPreview{},
			Artifacts:                   []string{"kv-history-migration-preview.json"},
			Actions:                     []string{"attach TemporalKVStore preview adapter before scanning reserved __kv_history__ documents"},
			Labels:                      []string{"memory-time-travel", "native-kv-history", "migration-preview", "not-attached", "no-native-write"},
			Notes:                       []string{"Native kv_history migration previewer is not attached to this pack instance; no scan was executed."},
		}, nil
	}
	preview, err := h.nativeKVHistoryPreviewer.PreviewNativeKVHistoryRows(ctx, temporalNamespaceFor(namespace), limit)
	if err != nil {
		return NativeKVHistoryMigrationPreview{}, err
	}
	preview.PackID = PackID
	preview.Stage = "native-kv-history-migration-preview-before-native-write"
	preview.Status = "preview_only"
	preview.NativeKVHistoryPreviewReady = true
	preview.WritesNativeKVHistory = false
	preview.MigratesKVHistory = false
	preview.UsesReservedKVNamespace = true
	preview.Artifacts = []string{"kv-history-migration-preview.json"}
	preview.Actions = []string{
		"review deterministic future kv_history rows expanded from reserved TemporalKV history documents",
		"keep native table creation, native row writes, migration execution, and reserved namespace cleanup blocked",
	}
	preview.Labels = []string{"memory-time-travel", "native-kv-history", "migration-preview", "preview-only", "no-native-write"}
	if len(preview.Rows) == 0 {
		preview.Rows = []NativeKVHistoryRowPreview{}
	}
	preview.Notes = append(preview.Notes,
		"This route is non-destructive: it scans the current reserved adapter and returns row previews only.",
		"native_kv_history_preview_ready=true does not mean native_kv_history_ready; writes_native_kv_history and migrates_kv_history remain false.",
	)
	return preview, nil
}

func (h *Handler) buildKVHistoryDualReadParity(ctx context.Context, req KVHistoryDualReadParityRequest) (KVHistoryDualReadParityReport, error) {
	namespace := normalizeNamespace(req.Namespace)
	at := req.At
	if at.IsZero() {
		at = h.now().UTC()
	}
	at = at.UTC()
	limit := req.Limit
	if limit <= 0 {
		limit = 200
	}
	if limit > 500 {
		limit = 500
	}

	preview, err := h.buildNativeKVHistoryMigrationPreview(ctx, namespace, limit)
	if err != nil {
		return KVHistoryDualReadParityReport{}, err
	}
	previewRows := preview.PreviewRowCount
	if previewRows == 0 {
		previewRows = len(preview.Rows)
	}
	returnedRows := preview.ReturnedRowCount
	if returnedRows == 0 {
		returnedRows = len(preview.Rows)
	}

	temporalNamespace := temporalNamespaceFor(namespace)
	reservedValues := map[string][]byte{}
	temporalAttached := h.temporalKV != nil
	if temporalAttached {
		raw, err := h.temporalKV.SnapshotRawAt(ctx, temporalNamespace, at)
		if err != nil {
			return KVHistoryDualReadParityReport{}, err
		}
		for key, value := range raw {
			reservedValues[key] = append([]byte(nil), value...)
		}
	}

	nativeValues := snapshotFromNativePreviewRows(preview.Rows, at)
	previewComplete := previewRows == returnedRows
	mismatches := compareKVHistoryParity(reservedValues, nativeValues)
	missing, extra, changed := classifyKVHistoryParityMismatches(mismatches)
	matched := 0
	for key, reserved := range reservedValues {
		native, ok := nativeValues[key]
		if ok && string(native) == string(reserved) {
			matched++
		}
	}
	parityPassed := temporalAttached && preview.NativeKVHistoryPreviewReady && previewComplete && len(mismatches) == 0
	status := "passed"
	if !temporalAttached || !preview.NativeKVHistoryPreviewReady {
		status = "not_attached"
	} else if !previewComplete {
		status = "incomplete_preview"
	} else if len(mismatches) > 0 {
		status = "mismatch"
	}

	blockedBy := []string{"native-kv-history-read-adapter-not-wired", "adapter-switch-not-enabled"}
	if !temporalAttached {
		blockedBy = append([]string{"reserved-temporal-kv-reader-not-attached"}, blockedBy...)
	}
	if !preview.NativeKVHistoryPreviewReady {
		blockedBy = append([]string{"native-kv-history-preview-adapter-not-attached"}, blockedBy...)
	}
	if !previewComplete {
		blockedBy = append([]string{"migration-preview-limit-truncated"}, blockedBy...)
	}
	if len(mismatches) > 0 {
		blockedBy = append([]string{"dual-read-parity-mismatch"}, blockedBy...)
	}
	return KVHistoryDualReadParityReport{
		PackID:                      PackID,
		Namespace:                   namespace,
		TemporalNamespace:           temporalNamespace,
		GeneratedAt:                 h.now().UTC(),
		At:                          at,
		Stage:                       "kv-history-dual-read-parity-before-adapter-switch",
		Status:                      status,
		Source:                      "reserved-temporal-kv-versus-native-row-preview",
		NativeTable:                 "kv_history",
		CurrentHistoryNamespace:     "__kv_history__",
		Limit:                       limit,
		PreviewRowCount:             previewRows,
		ReturnedPreviewRowCount:     returnedRows,
		TemporalKeyCount:            len(reservedValues),
		NativePreviewKeyCount:       len(nativeValues),
		MatchedKeyCount:             matched,
		MismatchCount:               len(mismatches),
		MissingFromNativeCount:      missing,
		ExtraInNativeCount:          extra,
		ValueMismatchCount:          changed,
		DualReadParityCheckReady:    temporalAttached && preview.NativeKVHistoryPreviewReady,
		DualReadParityReady:         parityPassed,
		ParityPassed:                parityPassed,
		PreviewComplete:             previewComplete,
		ReadsTemporalKV:             temporalAttached,
		ReadsNativeKVHistory:        false,
		ReadsNativeKVHistoryPreview: preview.NativeKVHistoryPreviewReady,
		SwitchesTemporalAdapter:     false,
		WritesLedgerKV:              false,
		WritesNativeKVHistory:       false,
		KVHistoryMigrationPreview:   preview,
		Mismatches:                  mismatches,
		Artifacts:                   []string{"kv-history-dual-read-parity.json", "kv-history-migration-preview.json"},
		Actions: []string{
			"compare reserved TemporalKV SnapshotRawAt output with reconstructed rows from native kv_history migration preview",
			"require complete preview coverage and zero mismatches before any future adapter switch can be considered",
			"keep TemporalKVReader pinned to the reserved __kv_history__ adapter; no native read adapter is enabled by this route",
		},
		BlockedBy: blockedBy,
		Labels:    []string{"memory-time-travel", "dual-read-parity", "read-only", "no-adapter-switch", "no-native-write"},
		Notes: []string{
			"This route is read-only: it reads the current reserved TemporalKV snapshot and migration row preview, then compares them in memory.",
			"dual_read_parity_ready=true only means the preview matches the reserved adapter for this timestamp; reads_native_kv_history, switches_temporal_adapter, writes_ledger_kv, and writes_native_kv_history remain false.",
			"Use kv-history-dual-read-parity.json as the evidence gate before a later native kv_history adapter implementation.",
		},
	}, nil
}

func (h *Handler) KVHistoryCutoverPlan(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var req KVHistoryCutoverPlanRequest
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid kv_history cutover plan payload")
		return
	}
	if strings.TrimSpace(string(body)) != "" {
		if err := json.Unmarshal(body, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid kv_history cutover plan payload")
			return
		}
	}
	plan, err := h.buildKVHistoryCutoverPlan(r.Context(), req)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"plan": plan})
}

func (h *Handler) KVHistoryCutoverReadiness(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var req KVHistoryCutoverReadinessRequest
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid kv_history cutover readiness payload")
		return
	}
	if strings.TrimSpace(string(body)) != "" {
		if err := json.Unmarshal(body, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid kv_history cutover readiness payload")
			return
		}
	}
	report, err := h.buildKVHistoryCutoverReadiness(r.Context(), req)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"readiness": report})
}

func (h *Handler) buildKVHistoryCutoverReadiness(ctx context.Context, req KVHistoryCutoverReadinessRequest) (KVHistoryCutoverReadinessReport, error) {
	namespace := normalizeNamespace(req.Namespace)
	at := req.At
	if at.IsZero() {
		at = h.now().UTC()
	}
	at = at.UTC()
	limit := req.Limit
	if limit <= 0 {
		limit = 500
	}
	if limit > 500 {
		limit = 500
	}

	cutoverPlan, err := h.buildKVHistoryCutoverPlan(ctx, KVHistoryCutoverPlanRequest{
		Namespace:   namespace,
		RequestedBy: req.RequestedBy,
		Reason:      req.Reason,
		Limit:       limit,
		DryRun:      true,
	})
	if err != nil {
		return KVHistoryCutoverReadinessReport{}, err
	}
	parity, err := h.buildKVHistoryDualReadParity(ctx, KVHistoryDualReadParityRequest{
		Namespace: namespace,
		At:        at,
		Limit:     limit,
	})
	if err != nil {
		return KVHistoryCutoverReadinessReport{}, err
	}

	gates := []KVHistoryCutoverReadinessGate{
		{
			Name:        "native-plan-contract",
			Ready:       cutoverPlan.NativeKVHistoryPlanReady,
			Required:    true,
			Status:      readinessStatus(cutoverPlan.NativeKVHistoryPlanReady),
			Evidence:    []string{"native-kv-history-plan.json", "kv-history-migration-plan.json", "kv-history-index-plan.json"},
			Description: "native kv_history table/index/migration contract is shaped and reviewable",
		},
		{
			Name:        "migration-preview-complete",
			Ready:       cutoverPlan.NativeKVHistoryPreviewReady && parity.PreviewComplete,
			Required:    true,
			Status:      readinessStatus(cutoverPlan.NativeKVHistoryPreviewReady && parity.PreviewComplete),
			Evidence:    []string{"kv-history-migration-preview.json"},
			Description: "migration preview must cover all preview rows before any native migration executor can be considered",
		},
		{
			Name:        "dual-read-parity",
			Ready:       parity.DualReadParityReady && parity.ParityPassed,
			Required:    true,
			Status:      readinessStatus(parity.DualReadParityReady && parity.ParityPassed),
			Evidence:    []string{"kv-history-dual-read-parity.json"},
			Description: "reserved TemporalKV SnapshotRawAt and reconstructed native row previews must match at the requested timestamp",
		},
		{
			Name:        "native-read-adapter",
			Ready:       false,
			Required:    true,
			Status:      "blocked",
			Evidence:    []string{"kv-history-dual-read-plan.json"},
			BlockedBy:   []string{"native-kv-history-read-adapter-not-wired", "adapter-switch-not-enabled"},
			Description: "TemporalKVReader must explicitly support native rows before reads can switch away from reserved __kv_history__",
		},
		{
			Name:        "native-write-path",
			Ready:       false,
			Required:    true,
			Status:      "blocked",
			Evidence:    []string{"kv-history-dual-write-plan.json"},
			BlockedBy:   []string{"migration-executor-not-wired", "dual-write-cutover-not-enabled"},
			Description: "dual-write mirror and native kv_history migration executor remain unwired",
		},
		{
			Name:        "approval-and-rollback",
			Ready:       false,
			Required:    true,
			Status:      "blocked",
			Evidence:    []string{"kv-history-cutover-rollback-plan.json"},
			BlockedBy:   []string{"approval-manager-not-wired", "cutover-rollback-executor-not-wired"},
			Description: "operator approval, cutover rollback executor, and rollback evidence chain must be connected before cutover",
		},
		{
			Name:        "audit-proof-linkage",
			Ready:       false,
			Required:    true,
			Status:      "blocked",
			Evidence:    []string{"audit-links.json", "audit-verification.json"},
			BlockedBy:   []string{"per-kv-merkle-proof-link-not-wired"},
			Description: "native kv_history rows are not yet linked to per-KV Merkle audit proofs",
		},
	}
	blockedBy := []string{}
	passedGates := 0
	blockedGates := 0
	requiredGates := 0
	for i := range gates {
		if gates[i].Required {
			requiredGates++
		}
		if gates[i].Ready {
			passedGates++
			continue
		}
		blockedGates++
		if len(gates[i].BlockedBy) == 0 {
			gates[i].BlockedBy = []string{gates[i].Name + "-not-ready"}
		}
		blockedBy = append(blockedBy, gates[i].BlockedBy...)
	}
	blockedBy = dedupeStrings(blockedBy)
	cutoverReady := false

	return KVHistoryCutoverReadinessReport{
		PackID:                      PackID,
		Namespace:                   namespace,
		TemporalNamespace:           temporalNamespaceFor(namespace),
		GeneratedAt:                 h.now().UTC(),
		At:                          at,
		Stage:                       "kv-history-cutover-readiness-before-adapter-switch",
		Status:                      "blocked",
		DryRun:                      true,
		RequestedBy:                 strings.TrimSpace(req.RequestedBy),
		Reason:                      strings.TrimSpace(req.Reason),
		Source:                      "cutover-plan-plus-dual-read-parity",
		NativeTable:                 "kv_history",
		CurrentHistoryNamespace:     "__kv_history__",
		CutoverReadinessCheckReady:  true,
		CutoverReady:                cutoverReady,
		NativeKVHistoryPlanReady:    cutoverPlan.NativeKVHistoryPlanReady,
		NativeKVHistoryPreviewReady: cutoverPlan.NativeKVHistoryPreviewReady,
		DualReadParityCheckReady:    parity.DualReadParityCheckReady,
		DualReadParityReady:         parity.DualReadParityReady,
		ParityPassed:                parity.ParityPassed,
		PreviewComplete:             parity.PreviewComplete,
		MigrationExecutorReady:      false,
		NativeReadAdapterReady:      false,
		NativeWritePathReady:        false,
		ApprovalManagerReady:        false,
		RollbackExecutorReady:       false,
		AuditProofLinkReady:         false,
		SwitchesTemporalAdapter:     false,
		WritesLedgerKV:              false,
		WritesNativeKVHistory:       false,
		ConsumesCutoverPlan:         true,
		ConsumesDualReadParity:      true,
		RequiredGateCount:           requiredGates,
		PassedGateCount:             passedGates,
		BlockedGateCount:            blockedGates,
		Gates:                       gates,
		CutoverPlan:                 cutoverPlan,
		DualReadParity:              parity,
		Artifacts:                   []string{"kv-history-cutover-readiness.json", "kv-history-cutover-plan.json", "kv-history-dual-read-parity.json"},
		Actions:                     []string{"review cutover readiness gates before any adapter switch", "keep native reads, native writes, Ledger writes, and cutover rollback execution blocked until every required gate is ready"},
		BlockedBy:                   blockedBy,
		Labels:                      []string{"memory-time-travel", "kv-history-cutover-readiness", "read-only", "no-adapter-switch", "no-native-write"},
		Notes: []string{
			"This readiness report is a gate aggregator only: it consumes cutover plan and dual-read parity evidence but never switches adapters or writes Ledger/native rows.",
			"cutover_ready remains false until native read adapter, native write path, approval manager, rollback executor, and per-KV audit proof linkage are wired in later slices.",
		},
	}, nil
}

func (h *Handler) buildKVHistoryCutoverPlan(ctx context.Context, req KVHistoryCutoverPlanRequest) (KVHistoryCutoverPlanReport, error) {
	namespace := normalizeNamespace(req.Namespace)
	limit := req.Limit
	if limit <= 0 {
		limit = 50
	}
	if limit > 500 {
		limit = 500
	}
	nativePlan := h.buildNativeKVHistoryPlan(namespace)
	preview, err := h.buildNativeKVHistoryMigrationPreview(ctx, namespace, limit)
	if err != nil {
		return KVHistoryCutoverPlanReport{}, err
	}
	previewRows := preview.PreviewRowCount
	if previewRows == 0 {
		previewRows = len(preview.Rows)
	}
	returnedRows := preview.ReturnedRowCount
	if returnedRows == 0 {
		returnedRows = len(preview.Rows)
	}
	blockedBy := []string{
		"ledger-native-kv-history-schema-not-wired",
		"migration-executor-not-wired",
		"dual-read-adapter-not-wired",
		"dual-write-cutover-not-enabled",
		"per-kv-merkle-proof-link-not-wired",
		"cutover-rollback-executor-not-wired",
	}
	phases := []KVHistoryCutoverPhasePlan{
		{Step: 1, Name: "schema-readiness-gate", From: "native-kv-history-plan.json", To: "Ledger kv_history DDL", Gate: "manual_schema_review", Ready: false, Writes: false, Status: "blocked", Description: "review table and index contract before Ledger owns the native kv_history schema", BlockedBy: []string{"ledger-native-kv-history-schema-not-wired"}},
		{Step: 2, Name: "migration-preview-gate", From: "__kv_history__", To: "kv_history row preview", Gate: "row_digest_review", Ready: preview.NativeKVHistoryPreviewReady, Writes: false, Status: "plan_only", Description: "compare deterministic row previews and value hashes before any migration executor exists"},
		{Step: 3, Name: "dual-read-adapter-gate", From: "__kv_history__ fallback", To: "native kv_history preferred read", Gate: "shadow_read_parity", Ready: false, Writes: false, Status: "blocked", Description: "future adapter must prove SnapshotRawAt/ListVersions parity before native reads are preferred", BlockedBy: []string{"dual-read-adapter-not-wired"}},
		{Step: 4, Name: "dual-write-cutover-gate", From: "TemporalKVStore reserved adapter", To: "reserved adapter + native kv_history mirror", Gate: "approval_required", Ready: false, Writes: false, Status: "blocked", Description: "future writer must mirror versioned puts only after schema, migration and read parity gates pass", BlockedBy: []string{"dual-write-cutover-not-enabled"}},
		{Step: 5, Name: "rollback-and-proof-link-gate", From: "cutover plan", To: "audited rollback executor", Gate: "approval_and_merkle_proof", Ready: false, Writes: false, Status: "blocked", Description: "cutover rollback and per-KV audit proof linkage remain separate executor work", BlockedBy: []string{"cutover-rollback-executor-not-wired", "per-kv-merkle-proof-link-not-wired"}},
	}
	dualRead := KVHistoryDualReadPlan{
		PlanReady:                true,
		Ready:                    false,
		PreferredSource:          "native kv_history rows",
		FallbackSource:           "reserved __kv_history__ Ledger KV namespace",
		ReadsNativeKVHistory:     false,
		ReadsReservedKVNamespace: true,
		SwitchesAdapter:          false,
		Validation: []string{
			"compare GetRawAt for namespace/key/time across reserved adapter and native kv_history row preview",
			"compare SnapshotRawAt namespace reconstruction against current TemporalKVReader output",
			"require row digest and version ordering parity before preferring native reads",
		},
		BlockedBy: []string{"dual-read-adapter-not-wired", "shadow-read-parity-tests-not-wired"},
		Notes: []string{
			"dual_read_plan_ready=true only means the adapter cutover contract is shaped.",
			"dual_read_ready remains false; this plan does not switch TemporalKVReader to native rows.",
		},
	}
	dualWrite := KVHistoryDualWritePlan{
		PlanReady:                 true,
		Ready:                     false,
		PrimaryTarget:             "reserved __kv_history__ Ledger KV namespace",
		MirrorTarget:              "native kv_history table",
		WritesNativeKVHistory:     false,
		WritesReservedKVNamespace: false,
		WritesLedgerKV:            false,
		MigrationExecutorReady:    false,
		Guardrails: []string{
			"require explicit operator approval before enabling mirror writes",
			"require native schema migration and rollback executor before dual-write can become ready",
			"preserve reserved __kv_history__ documents until native read parity and proof linkage pass",
		},
		BlockedBy: []string{"migration-executor-not-wired", "dual-write-cutover-not-enabled", "global-approval-manager-not-consumed"},
		Notes: []string{
			"dual_write_plan_ready=true is a non-destructive contract; writes_native_kv_history and writes_ledger_kv stay false.",
			"reserved adapter writes are also not performed by this route.",
		},
	}
	rollback := KVHistoryCutoverRollbackPlan{
		PlanReady:                  true,
		Ready:                      false,
		RequiresApproval:           true,
		RestoresReservedAdapter:    true,
		DropsNativeRows:            false,
		DeletesReservedKVNamespace: false,
		Actions: []string{
			"keep TemporalKVReader pinned to reserved adapter until cutover_ready=true",
			"if a future cutover fails, restore reserved adapter as primary read source",
			"preserve native rows for forensic comparison instead of deleting them automatically",
		},
		BlockedBy: []string{"cutover-rollback-executor-not-wired", "approval-manager-not-wired"},
		Notes: []string{
			"rollback_ready=false because no executor mutates adapters, tables, rows, or reserved documents in this slice.",
		},
	}
	return KVHistoryCutoverPlanReport{
		PackID:                      PackID,
		Namespace:                   namespace,
		GeneratedAt:                 h.now().UTC(),
		Stage:                       "kv-history-cutover-plan-before-dual-read-write",
		Status:                      "plan_only",
		DryRun:                      true,
		RequestedBy:                 strings.TrimSpace(req.RequestedBy),
		Reason:                      strings.TrimSpace(req.Reason),
		Source:                      "native-plan-plus-migration-preview",
		NativeTable:                 "kv_history",
		CurrentHistoryNamespace:     "__kv_history__",
		ConsumesNativeKVHistoryPlan: true,
		ConsumesMigrationPreview:    true,
		NativeKVHistoryPlanReady:    nativePlan.NativeKVHistoryPlanReady,
		NativeKVHistoryPreviewReady: preview.NativeKVHistoryPreviewReady,
		KVHistoryCutoverPlanReady:   true,
		DualReadPlanReady:           true,
		DualWritePlanReady:          true,
		NativeKVHistoryReady:        false,
		WritesNativeKVHistory:       false,
		MigratesKVHistory:           false,
		DualReadReady:               false,
		DualWriteReady:              false,
		CutoverReady:                false,
		RollbackReady:               false,
		CreatesNativeTable:          false,
		DeletesReservedKVNamespace:  false,
		SwitchesTemporalAdapter:     false,
		PreviewRowCount:             previewRows,
		ReturnedPreviewRowCount:     returnedRows,
		NativeKVHistoryPlan:         nativePlan,
		KVHistoryMigrationPreview:   preview,
		Phases:                      phases,
		DualReadPlan:                dualRead,
		DualWritePlan:               dualWrite,
		CutoverRollbackPlan:         rollback,
		Artifacts: []string{
			"kv-history-cutover-plan.json",
			"kv-history-dual-read-plan.json",
			"kv-history-dual-write-plan.json",
			"kv-history-cutover-rollback-plan.json",
			"native-kv-history-plan.json",
			"kv-history-migration-preview.json",
		},
		Actions: []string{
			"review native kv_history schema/index/migration plan before any DDL work",
			"review migration preview row digests before any migration executor writes native rows",
			"keep native reads, native writes, adapter switching, reserved namespace cleanup, and rollback execution blocked",
		},
		BlockedBy: blockedBy,
		Labels:    []string{"memory-time-travel", "kv-history-cutover", "dual-read-plan", "dual-write-plan", "plan-only", "no-native-write", "no-adapter-switch"},
		Notes: []string{
			"This route is non-destructive: it does not create tables, migrate rows, switch adapters, delete reserved __kv_history__ documents, write Ledger KV, or append Merkle proofs.",
			"Use kv-history-cutover-plan.json with the dual-read/dual-write artifacts as the contract for a later native Ledger kv_history cutover executor.",
			"cutover_ready, dual_read_ready, dual_write_ready, native_kv_history_ready, rollback_ready, writes_native_kv_history, and migrates_kv_history intentionally remain false.",
		},
	}, nil
}

func (h *Handler) AuditLinks(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	report := h.buildKVAuditLinksReport(r.URL.Query().Get("namespace"))
	writeJSON(w, http.StatusOK, map[string]any{"links": report})
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
		"files":       []string{"snapshot.json", "summary.json", "rollback-plan.json", "approved-rollback-plan.json", "rollback-writeback-plan.json", "approval-request-plan.json", "retention-plan.json", "retention-prune-plan.json", "native-kv-history-plan.json", "kv-history-migration-plan.json", "kv-history-index-plan.json", "kv-history-migration-preview.json", "kv-history-dual-read-parity.json", "kv-history-cutover-plan.json", "kv-history-cutover-readiness.json", "kv-history-dual-read-plan.json", "kv-history-dual-write-plan.json", "kv-history-cutover-rollback-plan.json", "audit-links.json", "audit-verification.json"},
		"snapshot":    snapshot,
		"history":     truncateSnapshots(snapshots, h.policy.EvidenceMaxSnapshots),
	}
	if rollbackPlan, err := h.buildRollbackPlan(RollbackPlanRequest{Namespace: snapshot.Namespace, SnapshotID: snapshot.ID, DryRun: true}); err != nil {
		payload["rollback_plan_error"] = err.Error()
	} else {
		payload["rollback_plan"] = rollbackPlan
	}
	if approvedRollbackPlan, err := h.buildApprovedRollbackPlan(RollbackExecutionPlanRequest{
		Namespace:   snapshot.Namespace,
		SnapshotID:  snapshot.ID,
		RequestedBy: "evidence-export",
		Reason:      "evidence-export-preview",
		DryRun:      true,
	}); err != nil {
		payload["approved_rollback_plan_error"] = err.Error()
	} else {
		payload["approved_rollback_plan"] = approvedRollbackPlan
		payload["rollback_writeback_plan"] = approvedRollbackPlan.WritebackActions
		payload["approval_request_plan"] = approvedRollbackPlan.ProposedApprovalRequest
	}
	if retentionPlan, err := h.buildRetentionPlan(snapshot.Namespace); err != nil {
		payload["retention_plan_error"] = err.Error()
	} else {
		payload["retention_plan"] = retentionPlan
		payload["retention_prune_plan"] = h.buildRetentionPrunePlanFromRetention(retentionPlan, RetentionPrunePlanRequest{
			Namespace: retentionPlan.Namespace,
			Reason:    "evidence-export-preview",
			DryRun:    true,
		})
	}
	auditLinks := h.buildKVAuditLinksReport(snapshot.Namespace)
	payload["kv_audit_link_schema"] = auditLinks
	payload["kv_audit_links"] = auditLinks.KVAuditLinks
	nativeKVHistoryPlan := h.buildNativeKVHistoryPlan(snapshot.Namespace)
	payload["native_kv_history_plan"] = nativeKVHistoryPlan
	payload["kv_history_migration_plan"] = nativeKVHistoryPlan.KVHistoryMigrationPlan
	payload["kv_history_index_plan"] = nativeKVHistoryPlan.KVHistoryIndexPlan
	if cutoverPlan, err := h.buildKVHistoryCutoverPlan(r.Context(), KVHistoryCutoverPlanRequest{
		Namespace:   snapshot.Namespace,
		RequestedBy: "evidence-export",
		Reason:      "evidence-export-preview",
		Limit:       50,
		DryRun:      true,
	}); err != nil {
		payload["kv_history_cutover_plan_error"] = err.Error()
	} else {
		payload["kv_history_cutover_plan"] = cutoverPlan
		payload["kv_history_dual_read_plan"] = cutoverPlan.DualReadPlan
		payload["kv_history_dual_write_plan"] = cutoverPlan.DualWritePlan
		payload["kv_history_cutover_rollback_plan"] = cutoverPlan.CutoverRollbackPlan
	}
	if parity, err := h.buildKVHistoryDualReadParity(r.Context(), KVHistoryDualReadParityRequest{
		Namespace: snapshot.Namespace,
		At:        snapshot.CreatedAt,
		Limit:     500,
	}); err != nil {
		payload["kv_history_dual_read_parity_error"] = err.Error()
	} else {
		payload["kv_history_dual_read_parity"] = parity
	}
	if readiness, err := h.buildKVHistoryCutoverReadiness(r.Context(), KVHistoryCutoverReadinessRequest{
		Namespace:   snapshot.Namespace,
		At:          snapshot.CreatedAt,
		RequestedBy: "evidence-export",
		Reason:      "evidence-export-preview",
		Limit:       500,
		DryRun:      true,
	}); err != nil {
		payload["kv_history_cutover_readiness_error"] = err.Error()
	} else {
		payload["kv_history_cutover_readiness"] = readiness
	}
	if h.nativeKVHistoryPreviewer != nil {
		preview, err := h.buildNativeKVHistoryMigrationPreview(r.Context(), snapshot.Namespace, 50)
		if err != nil {
			payload["kv_history_migration_preview_error"] = err.Error()
		} else {
			payload["kv_history_migration_preview"] = preview
		}
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
	notes := []string{"Pack-local snapshot store, point-in-time reconstruction, drift diff, dry-run rollback planning, approved rollback write-back planning, retention dry-run planning, and evidence export are available."}
	if h.temporalKV != nil {
		if h.memoryPersisterWriteback {
			notes = append(notes, "Ledger KV temporal history reader is attached and Memory Persister mirrors Mid/Long flushes into memory_snapshot; approved rollback write-back planning is shaped, while native kv_history tables, retention prune execution, cron scheduling, global Approval Manager enqueue, Merkle append, and rollback execution remain follow-up wiring.")
		} else {
			notes = append(notes, "Ledger KV temporal history reader is attached for snapshot-at reconstruction; approved rollback write-back planning is shaped, while Memory Persister write-back, native kv_history tables, retention prune execution, cron scheduling, global Approval Manager enqueue, Merkle append, and rollback execution remain follow-up wiring.")
		}
	} else {
		notes = append(notes, "Ledger KV kv_history reader is not attached; snapshot-at reconstruction falls back to pack-local snapshots, and approved rollback write-back planning remains a non-destructive contract preview.")
	}
	notes = append(notes, "Retention planning and prune-plan generation are dry-run only and currently target pack-local snapshots; Ledger temporal KV deletion is intentionally not connected yet.")
	notes = append(notes, "Native kv_history table, migration, index, and dual-read/dual-write cutover plans are available as plan-only artifacts; the current TemporalKV adapter still uses the reserved __kv_history__ Ledger KV namespace and does not migrate rows, write native rows, or switch adapters.")
	if h.nativeKVHistoryPreviewer != nil {
		notes = append(notes, "Native kv_history migration preview can scan reserved __kv_history__ documents into deterministic future row previews, but it remains read-only and does not create tables, migrate rows, or change TemporalKVStore behavior.")
	} else {
		notes = append(notes, "Native kv_history migration preview route is shaped, but the TemporalKVStore preview adapter is not attached to this pack instance.")
	}
	if h.temporalKV != nil && h.nativeKVHistoryPreviewer != nil {
		notes = append(notes, "Dual-read parity checks can compare reserved TemporalKV SnapshotRawAt output with native kv_history row previews as a read-only gate; this still does not enable adapter switching.")
	}
	notes = append(notes, "Cutover readiness checks aggregate native plan, migration preview, dual-read parity, adapter, write-path, approval, rollback, and audit-proof gates into a read-only report; cutover_ready remains false.")
	notes = append(notes, "KV audit proof-link schema is exposed as a placeholder; native kv_history rows are not yet joined to per-KV Merkle proofs.")
	if h.merkleVerifier != nil {
		notes = append(notes, "Read-only Merkle audit-chain verification is attached through Pack Runtime; individual KV-history entries are not yet linked to audit proofs.")
	} else {
		notes = append(notes, "Merkle audit-chain verification is not attached to this pack instance yet.")
	}
	return notes
}

func (h *Handler) buildNativeKVHistoryPlan(namespace string) NativeKVHistoryPlanReport {
	normalizedNamespace := "all"
	if strings.TrimSpace(namespace) != "" {
		normalizedNamespace = normalizeNamespace(namespace)
	}
	columns := []NativeKVHistoryColumnPlan{
		{Name: "id", Type: "text", Nullable: false, Purpose: "stable row id derived from namespace/key/version for idempotent migration"},
		{Name: "namespace", Type: "text", Nullable: false, Purpose: "Ledger KV namespace"},
		{Name: "key", Type: "text", Nullable: false, Purpose: "Ledger KV key"},
		{Name: "version", Type: "integer", Nullable: false, Purpose: "monotonic version per namespace/key"},
		{Name: "value", Type: "blob", Nullable: false, Purpose: "raw value bytes as stored by TemporalKV"},
		{Name: "value_sha256", Type: "text", Nullable: false, Purpose: "content digest used by Merkle/audit proof links"},
		{Name: "updated_at", Type: "timestamp", Nullable: false, Purpose: "logical value update time"},
		{Name: "archived_at", Type: "timestamp", Nullable: true, Purpose: "time when this value was superseded"},
		{Name: "current", Type: "boolean", Nullable: false, Purpose: "marks the latest materialized value for fast snapshot-at reads"},
		{Name: "audit_seq", Type: "integer", Nullable: true, Purpose: "future Merkle audit-chain sequence link"},
		{Name: "audit_hash", Type: "text", Nullable: true, Purpose: "future Merkle audit-chain hash link"},
		{Name: "source_adapter", Type: "text", Nullable: false, Purpose: "migration provenance, for example reserved-ledger-kv-namespace"},
	}
	indexes := []NativeKVHistoryIndexPlan{
		{Name: "kv_history_namespace_key_version_uq", Columns: []string{"namespace", "key", "version"}, Unique: true, Purpose: "idempotent replay and duplicate prevention during migration"},
		{Name: "kv_history_namespace_key_updated_at_idx", Columns: []string{"namespace", "key", "updated_at"}, Purpose: "point-in-time GetRawAt lookup"},
		{Name: "kv_history_namespace_updated_at_idx", Columns: []string{"namespace", "updated_at"}, Purpose: "SnapshotRawAt namespace reconstruction"},
		{Name: "kv_history_retention_idx", Columns: []string{"namespace", "archived_at", "current"}, Purpose: "retention prune candidate selection"},
		{Name: "kv_history_audit_idx", Columns: []string{"audit_seq", "audit_hash"}, Purpose: "per-KV audit proof linkage once Merkle append is wired"},
	}
	migration := []KVHistoryMigrationStepPlan{
		{Step: 1, Name: "scan-reserved-ledger-kv-history", From: "__kv_history__", To: "migration-buffer", DryRun: true, Writes: false, Status: "planned", Description: "scan TemporalKV history documents without deleting or rewriting the reserved namespace"},
		{Step: 2, Name: "expand-history-documents", From: "migration-buffer", To: "native kv_history row preview", DryRun: true, Writes: false, Status: "planned", Description: "expand each namespace/key versions array into idempotent kv_history rows"},
		{Step: 3, Name: "dual-read-adapter-gate", From: "__kv_history__ + kv_history", To: "TemporalKVReader", DryRun: true, Writes: false, Status: "planned", Description: "teach SnapshotRawAt/ListVersions to prefer native rows while falling back to the reserved adapter"},
		{Step: 4, Name: "dual-write-cutover-gate", From: "TemporalKVStore", To: "kv_history", DryRun: true, Writes: false, Status: "blocked", Description: "blocked until Ledger backend owns native kv_history schema and migration tests"},
	}
	return NativeKVHistoryPlanReport{
		PackID:                      PackID,
		Namespace:                   normalizedNamespace,
		GeneratedAt:                 h.now().UTC(),
		Stage:                       "native-kv-history-plan-before-schema-migration",
		Status:                      "plan_only",
		Source:                      "temporal-kv-adapter-readiness",
		CurrentAdapter:              "reserved-ledger-kv-namespace",
		CurrentHistoryNamespace:     "__kv_history__",
		NativeTable:                 "kv_history",
		TemporalKVAdapterReady:      true,
		NativeKVHistoryPlanReady:    true,
		KVHistoryMigrationPlanReady: true,
		KVHistoryIndexPlanReady:     true,
		NativeKVHistoryReady:        false,
		WritesNativeKVHistory:       false,
		MigratesKVHistory:           false,
		UsesReservedKVNamespace:     true,
		SnapshotStoreReady:          true,
		RetentionPlanReady:          true,
		AuditProofLinkSchemaReady:   true,
		SchemaPlan:                  columns,
		KVHistoryIndexPlan:          indexes,
		KVHistoryMigrationPlan:      migration,
		Artifacts:                   []string{"native-kv-history-plan.json", "kv-history-migration-plan.json", "kv-history-index-plan.json"},
		Actions: []string{
			"document the native kv_history row shape required by TemporalKVReader",
			"map reserved __kv_history__ documents into idempotent future kv_history rows",
			"keep schema migration, native writes, retention purge, and per-KV audit proof linkage blocked until Ledger owns the native table",
		},
		BlockedBy: []string{"ledger-native-kv-history-schema-not-wired", "migration-executor-not-wired", "dual-write-cutover-not-enabled", "per-kv-merkle-proof-link-not-wired"},
		Labels:    []string{"memory-time-travel", "native-kv-history", "migration-plan", "index-plan", "plan-only", "no-ledger-migration", "no-native-write"},
		Notes: []string{
			"This route is non-destructive: it does not create tables, migrate rows, delete reserved __kv_history__ documents, or change TemporalKVStore read/write behavior.",
			"The current TemporalKV adapter remains the source for snapshot-at reconstruction until a native Ledger kv_history table is implemented and dual-read tests pass.",
			"KV audit proof links need audit_seq/audit_hash backfill from future Merkle append wiring before linkage_ready can become true.",
		},
	}
}

func (h *Handler) buildKVAuditLinksReport(namespace string) KVAuditLinksReport {
	normalizedNamespace := "all"
	if strings.TrimSpace(namespace) != "" {
		normalizedNamespace = normalizeNamespace(namespace)
	}
	return KVAuditLinksReport{
		PackID:                  PackID,
		Namespace:               normalizedNamespace,
		GeneratedAt:             h.now().UTC(),
		SchemaReady:             true,
		LinkageReady:            false,
		NativeKVHistoryReady:    false,
		MerkleVerificationReady: h.merkleVerifier != nil,
		Source:                  "schema-placeholder-before-native-kv-history",
		KVAuditLinks:            []KVAuditProofLink{},
		RequiredFields: []string{
			"namespace",
			"key",
			"kv_version_at",
			"value_hash",
			"audit_seq",
			"audit_hash",
			"proof_status",
		},
		Notes: []string{
			"This report intentionally exposes only the stable KV audit proof-link schema.",
			"kv_audit_links is empty until native Ledger kv_history rows can be joined with Merkle audit records.",
			"The current Merkle verification route remains read-only audit-chain verification, not per-KV proof validation.",
		},
	}
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

func snapshotFromNativePreviewRows(rows []NativeKVHistoryRowPreview, at time.Time) map[string][]byte {
	if at.IsZero() {
		at = time.Now().UTC()
	}
	at = at.UTC()
	type candidate struct {
		value     []byte
		version   int
		updatedAt time.Time
	}
	selected := map[string]candidate{}
	for _, row := range rows {
		updatedAt := row.UpdatedAt.UTC()
		if updatedAt.IsZero() || updatedAt.After(at) {
			continue
		}
		current, ok := selected[row.Key]
		if ok {
			if current.version > row.Version {
				continue
			}
			if current.version == row.Version && !updatedAt.After(current.updatedAt) {
				continue
			}
		}
		selected[row.Key] = candidate{value: append([]byte(nil), row.Value...), version: row.Version, updatedAt: updatedAt}
	}
	out := make(map[string][]byte, len(selected))
	for key, item := range selected {
		out[key] = append([]byte(nil), item.value...)
	}
	return out
}

func compareKVHistoryParity(reserved, nativePreview map[string][]byte) []KVHistoryDualReadParityMismatch {
	keys := map[string]bool{}
	for key := range reserved {
		keys[key] = true
	}
	for key := range nativePreview {
		keys[key] = true
	}
	sorted := make([]string, 0, len(keys))
	for key := range keys {
		sorted = append(sorted, key)
	}
	sort.Strings(sorted)
	mismatches := make([]KVHistoryDualReadParityMismatch, 0)
	for _, key := range sorted {
		reservedValue, hasReserved := reserved[key]
		nativeValue, hasNative := nativePreview[key]
		switch {
		case hasReserved && !hasNative:
			mismatches = append(mismatches, KVHistoryDualReadParityMismatch{
				Key:           key,
				Kind:          "missing_from_native_preview",
				ReservedValue: decodeTemporalValue(reservedValue),
				ReservedHash:  valueHash(string(reservedValue)),
			})
		case !hasReserved && hasNative:
			mismatches = append(mismatches, KVHistoryDualReadParityMismatch{
				Key:                key,
				Kind:               "extra_in_native_preview",
				NativePreviewValue: decodeTemporalValue(nativeValue),
				NativePreviewHash:  valueHash(string(nativeValue)),
			})
		case hasReserved && hasNative && string(reservedValue) != string(nativeValue):
			mismatches = append(mismatches, KVHistoryDualReadParityMismatch{
				Key:                key,
				Kind:               "value_mismatch",
				ReservedValue:      decodeTemporalValue(reservedValue),
				NativePreviewValue: decodeTemporalValue(nativeValue),
				ReservedHash:       valueHash(string(reservedValue)),
				NativePreviewHash:  valueHash(string(nativeValue)),
			})
		}
	}
	return mismatches
}

func classifyKVHistoryParityMismatches(mismatches []KVHistoryDualReadParityMismatch) (missingFromNative, extraInNative, valueMismatch int) {
	for _, mismatch := range mismatches {
		switch mismatch.Kind {
		case "missing_from_native_preview":
			missingFromNative++
		case "extra_in_native_preview":
			extraInNative++
		case "value_mismatch":
			valueMismatch++
		}
	}
	return missingFromNative, extraInNative, valueMismatch
}

func readinessStatus(ready bool) string {
	if ready {
		return "passed"
	}
	return "blocked"
}

func dedupeStrings(values []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	sort.Strings(out)
	return out
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

func parsePreviewLimit(raw string) int {
	limit := 50
	if strings.TrimSpace(raw) != "" {
		if parsed, err := strconv.Atoi(strings.TrimSpace(raw)); err == nil {
			limit = parsed
		}
	}
	if limit <= 0 {
		return 50
	}
	if limit > 500 {
		return 500
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

func (h *Handler) buildApprovedRollbackPlan(req RollbackExecutionPlanRequest) (ApprovedRollbackExecutionPlanReport, error) {
	rollbackPlan, err := h.buildRollbackPlan(RollbackPlanRequest{
		Namespace:  req.Namespace,
		SnapshotID: req.SnapshotID,
		DryRun:     true,
	})
	if err != nil {
		return ApprovedRollbackExecutionPlanReport{}, err
	}
	generatedAt := h.now().UTC()
	requestedBy := strings.TrimSpace(req.RequestedBy)
	if requestedBy == "" {
		requestedBy = "operator"
	}
	reason := strings.TrimSpace(req.Reason)
	if reason == "" {
		reason = "approved rollback execution requires operator review"
	}
	approvalID := strings.TrimSpace(req.ApprovalID)
	values := make(map[string]string, len(rollbackPlan.PreviewValues))
	keys := make([]string, 0, len(rollbackPlan.PreviewValues))
	for key, value := range rollbackPlan.PreviewValues {
		values[key] = value
		keys = append(keys, key)
	}
	sort.Strings(keys)

	writebacks := make([]RollbackWritebackActionPlan, 0, len(keys))
	for idx, key := range keys {
		value := values[key]
		writebacks = append(writebacks, RollbackWritebackActionPlan{
			Operation:        "ledger_kv_put_versioned_preview",
			Namespace:        rollbackPlan.Namespace,
			Key:              key,
			ValueHash:        valueHash(value),
			ValueBytes:       len([]byte(value)),
			TargetSnapshotID: rollbackPlan.SnapshotID,
			TemporalVersion:  idx + 1,
			AuditAction:      "memory_time_travel.rollback_writeback.plan",
			RequiresApproval: true,
			ApprovalID:       approvalID,
			GeneratedAt:      generatedAt,
		})
	}
	status := "approved_rollback_writeback_plan"
	if approvalID == "" {
		status = "approval_required_before_writeback"
	}
	approvalRequest := h.rollbackApprovalRequestPlan(rollbackPlan, requestedBy, reason, approvalID, generatedAt, keys)
	return ApprovedRollbackExecutionPlanReport{
		PackID:                         PackID,
		GeneratedAt:                    generatedAt,
		Stage:                          "pack-shell-before-approved-rollback-writeback",
		Status:                         status,
		Namespace:                      rollbackPlan.Namespace,
		SnapshotID:                     rollbackPlan.SnapshotID,
		RequestedBy:                    requestedBy,
		Reason:                         reason,
		ApprovalID:                     approvalID,
		DryRun:                         true,
		ApprovalRequired:               true,
		ApprovalRequestPlanReady:       true,
		ApprovalManagerBridgePlanReady: true,
		GlobalApprovalEnqueueReady:     false,
		ApprovedRollbackPlanReady:      true,
		RollbackWritebackPlanReady:     true,
		RollbackWritebackReady:         false,
		WritesLedgerKV:                 false,
		WritesTemporalKV:               false,
		MerkleAppendReady:              false,
		AuditProofLinkReady:            false,
		ActionCount:                    len(writebacks),
		PreviewValues:                  values,
		RollbackPlan:                   rollbackPlan,
		ProposedApprovalRequest:        approvalRequest,
		WritebackActions:               writebacks,
		Artifacts:                      []string{"approved-rollback-plan.json", "rollback-writeback-plan.json", "approval-request-plan.json", "rollback-plan.json", "snapshot.json"},
		Actions: []string{
			"map the selected snapshot into versioned Ledger KV put previews",
			"shape a future global Approval Manager request for the rollback execution",
			"keep Ledger KV writes, temporal KV writes, Merkle append, and runtime memory mutation blocked until explicit execution wiring consumes an approved request",
		},
		Labels: []string{"memory-time-travel", "approved-rollback", "writeback-plan", "plan-only", "no-ledger-write", "no-merkle-append"},
		Notes: []string{
			"This route is a non-destructive approved rollback execution plan; it does not write Ledger KV, Temporal KV, pack-local snapshots, or live memory.",
			"The proposed approval request follows the global Approval Manager field shape (risk_level/requester/details), but global_approval_enqueue_ready remains false.",
			"Use approved-rollback-plan.json and rollback-writeback-plan.json as the contract for a later audited rollback executor.",
		},
	}, nil
}

func (h *Handler) rollbackApprovalRequestPlan(plan RollbackPlan, requestedBy, reason, approvalID string, generatedAt time.Time, keys []string) GlobalApprovalRequestPlan {
	requestKey := strings.TrimSpace(approvalID)
	if requestKey == "" {
		requestKey = deterministicID("memory-rollback-request", plan.Namespace, plan.SnapshotID, strings.Join(keys, ","), requestedBy, reason)
	}
	requestID := requestKey
	if len(requestID) > 48 {
		requestID = requestID[:48]
	}
	details := map[string]any{
		"pack_id":             PackID,
		"namespace":           plan.Namespace,
		"snapshot_id":         plan.SnapshotID,
		"key_count":           len(keys),
		"keys":                keys,
		"dry_run":             true,
		"writes_ledger_kv":    false,
		"writes_temporal_kv":  false,
		"merkle_append_ready": false,
		"generated_at":        generatedAt,
	}
	return GlobalApprovalRequestPlan{
		RequestID:                   requestID,
		RequestKey:                  requestKey,
		QueueName:                   "memory_time_travel_rollback",
		Category:                    "data_mutation",
		RiskLevel:                   "high",
		Summary:                     fmt.Sprintf("Approve Memory Time Travel rollback to snapshot %s", plan.SnapshotID),
		Details:                     details,
		Requester:                   requestedBy,
		Reason:                      reason,
		RequiredFields:              []string{"id", "category", "risk_level", "summary", "details", "requester", "tenant_id"},
		DecisionStates:              []string{"pending", "approved", "denied", "expired"},
		ApprovalManagerEnqueueReady: false,
		GlobalApprovalEnqueueReady:  false,
		ActionReleaseReady:          false,
		SourceStore:                 "pack-local-memory-snapshot",
		SourceArtifact:              "approved-rollback-plan.json",
		Payload: map[string]any{
			"rollback_plan":           plan,
			"rollback_writeback_plan": "rollback-writeback-plan.json",
		},
		Notes: []string{
			"Approval request is plan-only and is not enqueued into the global Approval Manager.",
			"Ledger KV write-back, Merkle audit append, and runtime memory mutation remain blocked until a later executor consumes an approved request.",
		},
	}
}

func (h *Handler) buildRetentionPlan(namespace string) (RetentionPlanReport, error) {
	normalizedNamespace := ""
	if strings.TrimSpace(namespace) != "" {
		normalizedNamespace = normalizeNamespace(namespace)
	}
	snapshots, err := h.listSnapshots(normalizedNamespace)
	if err != nil {
		return RetentionPlanReport{}, err
	}
	generatedAt := h.now().UTC()
	cutoffAt := generatedAt.AddDate(0, 0, -h.policy.RetentionDays)
	seenByNamespace := map[string]int{}
	candidates := make([]RetentionCandidate, 0)
	reclaimableBytes := 0

	for _, snapshot := range snapshots {
		seenByNamespace[snapshot.Namespace]++
		var reasons []string
		if snapshot.CreatedAt.Before(cutoffAt) {
			reasons = append(reasons, fmt.Sprintf("older_than_retention_days:%d", h.policy.RetentionDays))
		}
		if h.policy.MaxSnapshotsPerNamespace > 0 && seenByNamespace[snapshot.Namespace] > h.policy.MaxSnapshotsPerNamespace {
			reasons = append(reasons, fmt.Sprintf("exceeds_max_snapshots_per_namespace:%d", h.policy.MaxSnapshotsPerNamespace))
		}
		if len(reasons) == 0 {
			continue
		}
		action := fmt.Sprintf("would delete pack-local snapshot %s/%s", snapshot.Namespace, snapshot.ID)
		candidates = append(candidates, RetentionCandidate{
			ID:        snapshot.ID,
			Namespace: snapshot.Namespace,
			CreatedAt: snapshot.CreatedAt,
			Hash:      snapshot.Hash,
			SizeBytes: snapshot.SizeBytes,
			KeyCount:  snapshot.KeyCount,
			Reasons:   reasons,
			Action:    action,
		})
		reclaimableBytes += snapshot.SizeBytes
	}

	actions := make([]string, 0, len(candidates)+1)
	for _, candidate := range candidates {
		actions = append(actions, fmt.Sprintf("%s (reclaim %d bytes; %s)", candidate.Action, candidate.SizeBytes, strings.Join(candidate.Reasons, ",")))
	}
	if len(actions) == 0 {
		actions = append(actions, "no pack-local snapshot prune action required under the current policy")
	}

	namespaceLabel := "all"
	if normalizedNamespace != "" {
		namespaceLabel = normalizedNamespace
	}
	return RetentionPlanReport{
		PackID:                   PackID,
		Namespace:                namespaceLabel,
		GeneratedAt:              generatedAt,
		DryRun:                   true,
		Status:                   "dry_run",
		Policy:                   h.policy,
		CutoffAt:                 cutoffAt,
		Scopes:                   []string{"pack-local-snapshots"},
		MaxSnapshotsPerNamespace: h.policy.MaxSnapshotsPerNamespace,
		SnapshotCount:            len(snapshots),
		KeepCount:                len(snapshots) - len(candidates),
		CandidateCount:           len(candidates),
		ReclaimableBytes:         reclaimableBytes,
		TemporalHistoryReady:     h.temporalKV != nil,
		TemporalPruneReady:       false,
		Candidates:               candidates,
		Actions:                  actions,
		Notes: []string{
			"Retention planning is dry-run only; no snapshot files or Ledger KV history entries are deleted by this route.",
			"Pack-local snapshot retention uses retention_days and max_snapshots_per_namespace; native Ledger kv_history purge and cron scheduling remain follow-up wiring.",
		},
	}, nil
}

func (h *Handler) buildRetentionPrunePlan(req RetentionPrunePlanRequest) (RetentionPrunePlanReport, error) {
	retentionPlan, err := h.buildRetentionPlan(req.Namespace)
	if err != nil {
		return RetentionPrunePlanReport{}, err
	}
	return h.buildRetentionPrunePlanFromRetention(retentionPlan, req), nil
}

func (h *Handler) buildRetentionPrunePlanFromRetention(retentionPlan RetentionPlanReport, req RetentionPrunePlanRequest) RetentionPrunePlanReport {
	selectedIDs := map[string]bool{}
	for _, id := range req.CandidateIDs {
		trimmed := strings.TrimSpace(id)
		if trimmed != "" {
			selectedIDs[trimmed] = true
		}
	}

	selected := make([]RetentionCandidate, 0, len(retentionPlan.Candidates))
	reclaimableBytes := 0
	for _, candidate := range retentionPlan.Candidates {
		if len(selectedIDs) > 0 && !selectedIDs[candidate.ID] {
			continue
		}
		selected = append(selected, candidate)
		reclaimableBytes += candidate.SizeBytes
	}

	actions := make([]string, 0, len(selected)+1)
	for _, candidate := range selected {
		actions = append(actions, fmt.Sprintf("requires approval before deleting pack-local snapshot %s/%s", candidate.Namespace, candidate.ID))
	}
	if len(actions) == 0 {
		actions = append(actions, "no retention prune candidate selected; no write action will be executed")
	}

	status := "approval_plan"
	if len(selected) == 0 {
		status = "no_op"
	}
	return RetentionPrunePlanReport{
		PackID:                   PackID,
		Namespace:                retentionPlan.Namespace,
		GeneratedAt:              h.now().UTC(),
		DryRun:                   true,
		Status:                   status,
		ApprovalRequired:         len(selected) > 0,
		PruneReady:               false,
		TemporalPruneReady:       false,
		CandidateCount:           retentionPlan.CandidateCount,
		SelectedCandidateCount:   len(selected),
		ReclaimableBytes:         reclaimableBytes,
		ActionCount:              len(actions),
		RequestedBy:              strings.TrimSpace(req.RequestedBy),
		Reason:                   strings.TrimSpace(req.Reason),
		RetentionPlanGeneratedAt: retentionPlan.GeneratedAt,
		Candidates:               selected,
		Actions:                  actions,
		Notes: []string{
			"Retention prune-plan is non-destructive and does not delete pack-local snapshots or Ledger temporal KV entries.",
			"Execution must remain blocked until explicit approval, native kv_history pruning, and audit write-back are wired in a later slice.",
		},
	}
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

func deterministicID(prefix string, parts ...string) string {
	seed := strings.Join(parts, ":")
	sum := sha256.Sum256([]byte(seed))
	return fmt.Sprintf("%s-%s", prefix, hex.EncodeToString(sum[:])[:16])
}

func normalizeNamespace(namespace string) string {
	namespace = strings.ToLower(strings.TrimSpace(namespace))
	if namespace == "" {
		return "memory-snapshot"
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

func temporalNamespaceFor(namespace string) string {
	normalized := normalizeNamespace(namespace)
	if normalized == "memory-snapshot" {
		return "memory_snapshot"
	}
	return normalized
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
	if policy.MaxSnapshotsPerNamespace <= 0 {
		policy.MaxSnapshotsPerNamespace = 100
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
