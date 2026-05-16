import { fetcher } from "./api-core";

export interface MemoryTimeTravelPolicy {
  retention_days: number;
  max_versions_per_key: number;
  max_snapshots_per_namespace: number;
  max_snapshot_bytes: number;
  max_keys_per_snapshot: number;
  evidence_max_snapshots: number;
}

export interface MemoryTimeTravelSnapshot {
  id: string;
  namespace: string;
  created_at: string;
  source?: string;
  reason?: string;
  values: Record<string, string>;
  hash: string;
  size_bytes: number;
  key_count: number;
  version: number;
}

export interface MemoryTimeTravelSnapshotSummary {
  id: string;
  namespace: string;
  created_at: string;
  source?: string;
  reason?: string;
  hash: string;
  size_bytes: number;
  key_count: number;
  version: number;
}

export interface MemoryTimeTravelStatus {
  pack_id: string;
  stage: string;
  snapshot_store_ready: boolean;
  temporal_query_ready: boolean;
  ledger_history_ready: boolean;
  temporal_kv_adapter_ready?: boolean;
  native_kv_history_plan_ready?: boolean;
  kv_history_migration_plan_ready?: boolean;
  kv_history_index_plan_ready?: boolean;
  native_kv_history_preview_ready?: boolean;
  kv_history_cutover_plan_ready?: boolean;
  dual_read_plan_ready?: boolean;
  dual_write_plan_ready?: boolean;
  native_kv_history_ready?: boolean;
  writes_native_kv_history?: boolean;
  migrates_kv_history?: boolean;
  dual_read_ready?: boolean;
  dual_write_ready?: boolean;
  cutover_ready?: boolean;
  rollback_ready?: boolean;
  merkle_verification_ready: boolean;
  memory_persister_writeback_ready?: boolean;
  approved_rollback_plan_ready?: boolean;
  approval_request_plan_ready?: boolean;
  approval_manager_bridge_plan_ready?: boolean;
  global_approval_enqueue_ready?: boolean;
  rollback_writeback_plan_ready?: boolean;
  rollback_writeback_ready: boolean;
  writes_ledger_kv?: boolean;
  writes_temporal_kv?: boolean;
  retention_plan_ready?: boolean;
  retention_prune_plan_ready?: boolean;
  retention_prune_ready?: boolean;
  kv_audit_link_schema_ready?: boolean;
  kv_audit_linkage_ready?: boolean;
  snapshot_count: number;
  namespace_count: number;
  store_dir?: string;
  policy: MemoryTimeTravelPolicy;
  last_snapshot?: MemoryTimeTravelSnapshotSummary | null;
  capabilities: string[];
  notes?: string[];
}

export interface MemoryTimeTravelAuditRecord {
  seq: number;
  timestamp: string;
  type: string;
  actor?: string;
  action: string;
  prev_hash?: string;
  hash: string;
}

export interface MemoryTimeTravelAuditVerification {
  ready: boolean;
  valid: boolean;
  invalid_index: number;
  record_count: number;
  last_hash?: string;
  last_seq?: number;
  checked_at: string;
  recent_records?: MemoryTimeTravelAuditRecord[];
  notes?: string[];
}

export interface MemoryTimeTravelKVAuditProofLink {
  namespace: string;
  key: string;
  snapshot_id?: string;
  kv_version_at?: string;
  value_hash?: string;
  audit_seq?: number;
  audit_hash?: string;
  proof_status: string;
  notes?: string[];
}

export interface MemoryTimeTravelKVAuditLinksReport {
  pack_id: string;
  namespace: string;
  generated_at: string;
  schema_ready: boolean;
  linkage_ready: boolean;
  native_kv_history_ready: boolean;
  merkle_verification_ready: boolean;
  source: string;
  kv_audit_links: MemoryTimeTravelKVAuditProofLink[];
  required_fields: string[];
  notes?: string[];
}

export interface MemoryTimeTravelNativeKVHistoryColumnPlan {
  name: string;
  type: string;
  nullable: boolean;
  purpose: string;
}

export interface MemoryTimeTravelNativeKVHistoryIndexPlan {
  name: string;
  columns: string[];
  unique?: boolean;
  purpose: string;
}

export interface MemoryTimeTravelKVHistoryMigrationStepPlan {
  step: number;
  name: string;
  from: string;
  to: string;
  dry_run: boolean;
  writes: boolean;
  status: string;
  description: string;
}

export interface MemoryTimeTravelNativeKVHistoryPlan {
  pack_id: string;
  namespace: string;
  generated_at: string;
  stage: string;
  status: string;
  source: string;
  current_adapter: string;
  current_history_namespace: string;
  native_table: string;
  temporal_kv_adapter_ready: boolean;
  native_kv_history_plan_ready: boolean;
  kv_history_migration_plan_ready: boolean;
  kv_history_index_plan_ready: boolean;
  native_kv_history_ready: boolean;
  writes_native_kv_history: boolean;
  migrates_kv_history: boolean;
  uses_reserved_kv_namespace: boolean;
  snapshot_store_ready: boolean;
  retention_plan_ready: boolean;
  audit_proof_link_schema_ready: boolean;
  schema_plan: MemoryTimeTravelNativeKVHistoryColumnPlan[];
  kv_history_index_plan: MemoryTimeTravelNativeKVHistoryIndexPlan[];
  kv_history_migration_plan: MemoryTimeTravelKVHistoryMigrationStepPlan[];
  artifacts: string[];
  actions: string[];
  blocked_by: string[];
  labels: string[];
  notes?: string[];
}

export interface MemoryTimeTravelNativeKVHistoryRowPreview {
  id: string;
  namespace: string;
  key: string;
  version: number;
  value_base64: string;
  value_sha256: string;
  updated_at: string;
  archived_at?: string;
  current: boolean;
  audit_seq?: number;
  audit_hash?: string;
  source_adapter: string;
}

export interface MemoryTimeTravelNativeKVHistoryMigrationPreview {
  pack_id?: string;
  namespace: string;
  generated_at: string;
  stage?: string;
  status?: string;
  source_namespace: string;
  native_table: string;
  scanned_document_count: number;
  preview_row_count: number;
  returned_row_count: number;
  limit?: number;
  native_kv_history_preview_ready: boolean;
  writes_native_kv_history: boolean;
  migrates_kv_history: boolean;
  uses_reserved_kv_namespace: boolean;
  rows: MemoryTimeTravelNativeKVHistoryRowPreview[];
  artifacts?: string[];
  actions?: string[];
  labels?: string[];
  notes?: string[];
}

export interface MemoryTimeTravelKVHistoryCutoverPlanInput {
  namespace?: string;
  requested_by?: string;
  reason?: string;
  limit?: number;
  dry_run?: boolean;
}

export interface MemoryTimeTravelKVHistoryCutoverPhasePlan {
  step: number;
  name: string;
  from: string;
  to: string;
  gate: string;
  ready: boolean;
  writes: boolean;
  status: string;
  description: string;
  blocked_by?: string[];
}

export interface MemoryTimeTravelKVHistoryDualReadPlan {
  plan_ready: boolean;
  ready: boolean;
  preferred_source: string;
  fallback_source: string;
  reads_native_kv_history: boolean;
  reads_reserved_kv_namespace: boolean;
  switches_adapter: boolean;
  validation: string[];
  blocked_by: string[];
  notes?: string[];
}

export interface MemoryTimeTravelKVHistoryDualWritePlan {
  plan_ready: boolean;
  ready: boolean;
  primary_target: string;
  mirror_target: string;
  writes_native_kv_history: boolean;
  writes_reserved_kv_namespace: boolean;
  writes_ledger_kv: boolean;
  migration_executor_ready: boolean;
  guardrails: string[];
  blocked_by: string[];
  notes?: string[];
}

export interface MemoryTimeTravelKVHistoryCutoverRollbackPlan {
  plan_ready: boolean;
  ready: boolean;
  requires_approval: boolean;
  restores_reserved_adapter: boolean;
  drops_native_rows: boolean;
  deletes_reserved_kv_namespace: boolean;
  actions: string[];
  blocked_by: string[];
  notes?: string[];
}

export interface MemoryTimeTravelKVHistoryCutoverPlan {
  pack_id: string;
  namespace: string;
  generated_at: string;
  stage: string;
  status: string;
  dry_run: boolean;
  requested_by?: string;
  reason?: string;
  source: string;
  native_table: string;
  current_history_namespace: string;
  consumes_native_kv_history_plan: boolean;
  consumes_migration_preview: boolean;
  native_kv_history_plan_ready: boolean;
  native_kv_history_preview_ready: boolean;
  kv_history_cutover_plan_ready: boolean;
  dual_read_plan_ready: boolean;
  dual_write_plan_ready: boolean;
  native_kv_history_ready: boolean;
  writes_native_kv_history: boolean;
  migrates_kv_history: boolean;
  dual_read_ready: boolean;
  dual_write_ready: boolean;
  cutover_ready: boolean;
  rollback_ready: boolean;
  creates_native_table: boolean;
  deletes_reserved_kv_namespace: boolean;
  switches_temporal_adapter: boolean;
  preview_row_count: number;
  returned_preview_row_count: number;
  native_kv_history_plan: MemoryTimeTravelNativeKVHistoryPlan;
  kv_history_migration_preview: MemoryTimeTravelNativeKVHistoryMigrationPreview;
  phases: MemoryTimeTravelKVHistoryCutoverPhasePlan[];
  dual_read_plan: MemoryTimeTravelKVHistoryDualReadPlan;
  dual_write_plan: MemoryTimeTravelKVHistoryDualWritePlan;
  cutover_rollback_plan: MemoryTimeTravelKVHistoryCutoverRollbackPlan;
  artifacts: string[];
  actions: string[];
  blocked_by: string[];
  labels: string[];
  notes?: string[];
}

export interface MemoryTimeTravelSaveSnapshotInput {
  id?: string;
  namespace?: string;
  source?: string;
  reason?: string;
  values: Record<string, string>;
  dry_run?: boolean;
}

export interface MemoryTimeTravelSnapshotAtInput {
  namespace?: string;
  at?: string;
}

export interface MemoryTimeTravelSnapshotAtResponse {
  namespace: string;
  at: string;
  snapshot?: MemoryTimeTravelSnapshot;
  values: Record<string, string>;
  matched_id?: string;
  status: string;
}

export interface MemoryTimeTravelDiffInput {
  namespace?: string;
  base_id: string;
  target_id: string;
}

export interface MemoryTimeTravelDiffEntry {
  key: string;
  change: string;
  before?: string;
  after?: string;
  before_hash?: string;
  after_hash?: string;
  impact_level: string;
}

export interface MemoryTimeTravelDiffReport {
  id: string;
  pack_id: string;
  namespace: string;
  created_at: string;
  stage: string;
  base_id: string;
  target_id: string;
  added_count: number;
  removed_count: number;
  changed_count: number;
  drift_score: number;
  risk_level: string;
  entries: MemoryTimeTravelDiffEntry[];
  rollback_plan: string[];
  recommendations?: string[];
  notes?: string[];
}

export interface MemoryTimeTravelRollbackPlanInput {
  namespace?: string;
  snapshot_id: string;
  dry_run?: boolean;
}

export interface MemoryTimeTravelRollbackPlan {
  pack_id: string;
  namespace: string;
  snapshot_id: string;
  dry_run: boolean;
  action_count: number;
  actions: string[];
  preview_values?: Record<string, string>;
  status: string;
  notes?: string[];
}

export interface MemoryTimeTravelApprovedRollbackPlanInput {
  namespace?: string;
  snapshot_id: string;
  requested_by?: string;
  reason?: string;
  approval_id?: string;
  dry_run?: boolean;
}

export interface MemoryTimeTravelRollbackWritebackActionPlan {
  operation: string;
  namespace: string;
  key: string;
  value_hash?: string;
  value_bytes?: number;
  target_snapshot_id: string;
  temporal_version: number;
  audit_action: string;
  requires_approval: boolean;
  approval_id?: string;
  generated_at: string;
}

export interface MemoryTimeTravelGlobalApprovalRequestPlan {
  request_id: string;
  request_key: string;
  task_id?: string;
  workflow_id?: string;
  step_index?: number;
  queue_name: string;
  category: string;
  risk_level: string;
  summary: string;
  details: Record<string, unknown>;
  requester: string;
  tenant_id?: string;
  reason: string;
  required_fields: string[];
  decision_states: string[];
  approval_manager_enqueue_ready: boolean;
  global_approval_enqueue_ready: boolean;
  action_release_ready: boolean;
  source_store: string;
  source_artifact: string;
  payload: Record<string, unknown>;
  notes?: string[];
}

export interface MemoryTimeTravelApprovedRollbackPlan {
  pack_id: string;
  generated_at: string;
  stage: string;
  status: string;
  namespace: string;
  snapshot_id: string;
  requested_by?: string;
  reason?: string;
  approval_id?: string;
  dry_run: boolean;
  approval_required: boolean;
  approval_request_plan_ready: boolean;
  approval_manager_bridge_plan_ready: boolean;
  global_approval_enqueue_ready: boolean;
  approved_rollback_plan_ready: boolean;
  rollback_writeback_plan_ready: boolean;
  rollback_writeback_ready: boolean;
  writes_ledger_kv: boolean;
  writes_temporal_kv: boolean;
  merkle_append_ready: boolean;
  audit_proof_link_ready: boolean;
  action_count: number;
  preview_values?: Record<string, string>;
  rollback_plan: MemoryTimeTravelRollbackPlan;
  proposed_approval_request: MemoryTimeTravelGlobalApprovalRequestPlan;
  writeback_actions: MemoryTimeTravelRollbackWritebackActionPlan[];
  artifacts: string[];
  actions: string[];
  labels: string[];
  notes?: string[];
}

export interface MemoryTimeTravelRetentionCandidate {
  id: string;
  namespace: string;
  created_at: string;
  hash: string;
  size_bytes: number;
  key_count: number;
  reasons: string[];
  action: string;
}

export interface MemoryTimeTravelRetentionPlan {
  pack_id: string;
  namespace: string;
  generated_at: string;
  dry_run: boolean;
  status: string;
  policy: MemoryTimeTravelPolicy;
  cutoff_at: string;
  scopes: string[];
  snapshot_count: number;
  keep_count: number;
  candidate_count: number;
  reclaimable_bytes: number;
  temporal_history_ready: boolean;
  temporal_prune_ready: boolean;
  candidates: MemoryTimeTravelRetentionCandidate[];
  actions: string[];
  notes?: string[];
}

export interface MemoryTimeTravelRetentionPrunePlanInput {
  namespace?: string;
  candidate_ids?: string[];
  reason?: string;
  requested_by?: string;
  dry_run?: boolean;
}

export interface MemoryTimeTravelRetentionPrunePlan {
  pack_id: string;
  namespace: string;
  generated_at: string;
  dry_run: boolean;
  status: string;
  approval_required: boolean;
  prune_ready: boolean;
  temporal_prune_ready: boolean;
  candidate_count: number;
  selected_candidate_count: number;
  reclaimable_bytes: number;
  action_count: number;
  requested_by?: string;
  reason?: string;
  retention_plan_generated_at: string;
  candidates: MemoryTimeTravelRetentionCandidate[];
  actions: string[];
  notes?: string[];
}

export interface MemoryTimeTravelEvidenceResponse {
  pack_id: string;
  exported_at: string;
  format: string;
  files: string[];
  snapshot: MemoryTimeTravelSnapshot;
  history: MemoryTimeTravelSnapshotSummary[];
  rollback_plan?: MemoryTimeTravelRollbackPlan;
  rollback_plan_error?: string;
  approved_rollback_plan?: MemoryTimeTravelApprovedRollbackPlan;
  approved_rollback_plan_error?: string;
  rollback_writeback_plan?: MemoryTimeTravelRollbackWritebackActionPlan[];
  approval_request_plan?: MemoryTimeTravelGlobalApprovalRequestPlan;
  retention_plan?: MemoryTimeTravelRetentionPlan;
  retention_plan_error?: string;
  retention_prune_plan?: MemoryTimeTravelRetentionPrunePlan;
  native_kv_history_plan?: MemoryTimeTravelNativeKVHistoryPlan;
  kv_history_migration_plan?: MemoryTimeTravelKVHistoryMigrationStepPlan[];
  kv_history_index_plan?: MemoryTimeTravelNativeKVHistoryIndexPlan[];
  kv_history_migration_preview?: MemoryTimeTravelNativeKVHistoryMigrationPreview;
  kv_history_migration_preview_error?: string;
  kv_history_cutover_plan?: MemoryTimeTravelKVHistoryCutoverPlan;
  kv_history_cutover_plan_error?: string;
  kv_history_dual_read_plan?: MemoryTimeTravelKVHistoryDualReadPlan;
  kv_history_dual_write_plan?: MemoryTimeTravelKVHistoryDualWritePlan;
  kv_history_cutover_rollback_plan?: MemoryTimeTravelKVHistoryCutoverRollbackPlan;
  kv_audit_link_schema?: MemoryTimeTravelKVAuditLinksReport;
  kv_audit_links?: MemoryTimeTravelKVAuditProofLink[];
  audit_verification?: MemoryTimeTravelAuditVerification;
  audit_verification_error?: string;
}

export interface MemoryTimeTravelPackClient {
  status(): Promise<MemoryTimeTravelStatus>;
  snapshots(namespace?: string): Promise<{ snapshots: MemoryTimeTravelSnapshotSummary[]; count: number }>;
  saveSnapshot(input: MemoryTimeTravelSaveSnapshotInput): Promise<{ snapshot: MemoryTimeTravelSnapshot; status: string }>;
  snapshot(id: string): Promise<{ snapshot: MemoryTimeTravelSnapshot }>;
  snapshotAt(input: MemoryTimeTravelSnapshotAtInput): Promise<MemoryTimeTravelSnapshotAtResponse>;
  diff(input: MemoryTimeTravelDiffInput): Promise<{ diff: MemoryTimeTravelDiffReport }>;
  rollbackPlan(input: MemoryTimeTravelRollbackPlanInput): Promise<{ plan: MemoryTimeTravelRollbackPlan }>;
  approvedRollbackPlan(input: MemoryTimeTravelApprovedRollbackPlanInput): Promise<{ plan: MemoryTimeTravelApprovedRollbackPlan }>;
  retentionPlan(namespace?: string): Promise<{ plan: MemoryTimeTravelRetentionPlan }>;
  retentionPrunePlan(input?: MemoryTimeTravelRetentionPrunePlanInput): Promise<{ plan: MemoryTimeTravelRetentionPrunePlan }>;
  nativeKVHistoryPlan(namespace?: string): Promise<{ plan: MemoryTimeTravelNativeKVHistoryPlan }>;
  nativeKVHistoryMigrationPreview(namespace?: string, limit?: number): Promise<{ kv_history_migration_preview: MemoryTimeTravelNativeKVHistoryMigrationPreview }>;
  kvHistoryCutoverPlan(input?: MemoryTimeTravelKVHistoryCutoverPlanInput): Promise<{ plan: MemoryTimeTravelKVHistoryCutoverPlan }>;
  auditLinks(namespace?: string): Promise<{ links: MemoryTimeTravelKVAuditLinksReport }>;
  auditVerify(limit?: number): Promise<MemoryTimeTravelAuditVerification>;
  evidence(id: string): Promise<MemoryTimeTravelEvidenceResponse>;
}

function enc(value: string): string {
  return encodeURIComponent(value);
}

function query(params: Record<string, string | undefined>): string {
  const search = new URLSearchParams();
  for (const [key, value] of Object.entries(params)) if (value) search.set(key, value);
  const text = search.toString();
  return text ? `?${text}` : "";
}

export function createMemoryTimeTravelPackClient(): MemoryTimeTravelPackClient {
  return {
    status: () => fetcher<MemoryTimeTravelStatus>("/v1/memory-time-travel/status"),
    snapshots: (namespace) =>
      fetcher<{ snapshots: MemoryTimeTravelSnapshotSummary[]; count: number }>(`/v1/memory-time-travel/snapshots${query({ namespace })}`),
    saveSnapshot: (input) =>
      fetcher<{ snapshot: MemoryTimeTravelSnapshot; status: string }>("/v1/memory-time-travel/snapshots", {
        method: "POST",
        body: JSON.stringify(input),
      }),
    snapshot: (id) => fetcher<{ snapshot: MemoryTimeTravelSnapshot }>(`/v1/memory-time-travel/snapshots/${enc(id)}`),
    snapshotAt: (input) =>
      fetcher<MemoryTimeTravelSnapshotAtResponse>("/v1/memory-time-travel/snapshot-at", {
        method: "POST",
        body: JSON.stringify(input),
      }),
    diff: (input) =>
      fetcher<{ diff: MemoryTimeTravelDiffReport }>("/v1/memory-time-travel/diff", {
        method: "POST",
        body: JSON.stringify(input),
      }),
    rollbackPlan: (input) =>
      fetcher<{ plan: MemoryTimeTravelRollbackPlan }>("/v1/memory-time-travel/rollback-plan", {
        method: "POST",
        body: JSON.stringify(input),
      }),
    approvedRollbackPlan: (input) =>
      fetcher<{ plan: MemoryTimeTravelApprovedRollbackPlan }>("/v1/memory-time-travel/rollback/approved-plan", {
        method: "POST",
        body: JSON.stringify(input),
      }),
    retentionPlan: (namespace) =>
      fetcher<{ plan: MemoryTimeTravelRetentionPlan }>(`/v1/memory-time-travel/retention/plan${query({ namespace })}`),
    retentionPrunePlan: (input = {}) =>
      fetcher<{ plan: MemoryTimeTravelRetentionPrunePlan }>("/v1/memory-time-travel/retention/prune-plan", {
        method: "POST",
        body: JSON.stringify(input),
      }),
    nativeKVHistoryPlan: (namespace) =>
      fetcher<{ plan: MemoryTimeTravelNativeKVHistoryPlan }>(`/v1/memory-time-travel/kv-history/native-plan${query({ namespace })}`),
    nativeKVHistoryMigrationPreview: (namespace, limit) =>
      fetcher<{ kv_history_migration_preview: MemoryTimeTravelNativeKVHistoryMigrationPreview }>(
        `/v1/memory-time-travel/kv-history/migration-preview${query({ namespace, limit: limit ? String(limit) : undefined })}`,
      ),
    kvHistoryCutoverPlan: (input = {}) =>
      fetcher<{ plan: MemoryTimeTravelKVHistoryCutoverPlan }>("/v1/memory-time-travel/kv-history/cutover/plan", {
        method: "POST",
        body: JSON.stringify(input),
      }),
    auditLinks: (namespace) =>
      fetcher<{ links: MemoryTimeTravelKVAuditLinksReport }>(`/v1/memory-time-travel/audit/links${query({ namespace })}`),
    auditVerify: (limit) =>
      fetcher<MemoryTimeTravelAuditVerification>(`/v1/memory-time-travel/audit/verify${query({ limit: limit ? String(limit) : undefined })}`),
    evidence: (id) =>
      fetcher<MemoryTimeTravelEvidenceResponse>(`/v1/memory-time-travel/evidence/${enc(id)}`),
  };
}
