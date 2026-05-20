/**
 * Lightweight Memory Time Travel Pack SDK slice.
 * Covers snapshots, rollback/retention plans, native kv_history previews,
 * proof-link writeback handoffs, cutover gates, audit verification, and evidence
 * without importing the full generated OpenAPI SDK.
 */

export type MemoryTimeTravelPolicy = {
  retention_days: number;
  max_versions_per_key: number;
  max_snapshots_per_namespace: number;
  max_snapshot_bytes: number;
  max_keys_per_snapshot: number;
  evidence_max_snapshots: number;
};

export type MemoryTimeTravelSnapshot = {
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
};

export type MemoryTimeTravelSnapshotSummary = {
  id: string;
  namespace: string;
  created_at: string;
  source?: string;
  reason?: string;
  hash: string;
  size_bytes: number;
  key_count: number;
  version: number;
};

export type MemoryTimeTravelStatusResponse = {
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
  kv_history_cutover_readiness_ready?: boolean;
  dual_read_plan_ready?: boolean;
  dual_read_parity_check_ready?: boolean;
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
  retention_pack_local_prune_ready?: boolean;
  writes_pack_local_snapshot_store?: boolean;
  kv_audit_link_schema_ready?: boolean;
  kv_audit_link_preview_ready?: boolean;
  kv_audit_link_writeback_plan_ready?: boolean;
  kv_audit_link_writeback_store_ready?: boolean;
  kv_audit_link_writeback_executor_plan_ready?: boolean;
  executor_input_contract_ready?: boolean;
  audit_proof_link_executor_ready?: boolean;
  kv_audit_link_writeback_ready?: boolean;
  kv_audit_linkage_ready?: boolean;
  snapshot_count: number;
  namespace_count: number;
  store_dir?: string;
  policy: MemoryTimeTravelPolicy;
  last_snapshot?: MemoryTimeTravelSnapshotSummary | null;
  capabilities: string[];
  notes?: string[];
};

export type MemoryTimeTravelAuditRecord = {
  seq: number;
  timestamp: string;
  type: string;
  actor?: string;
  action: string;
  prev_hash?: string;
  hash: string;
};

export type MemoryTimeTravelAuditVerificationResponse = {
  ready: boolean;
  valid: boolean;
  invalid_index: number;
  record_count: number;
  last_hash?: string;
  last_seq?: number;
  checked_at: string;
  recent_records?: MemoryTimeTravelAuditRecord[];
  notes?: string[];
};

export type MemoryTimeTravelKVAuditProofLink = {
  namespace: string;
  key: string;
  snapshot_id?: string;
  kv_version_at?: string;
  value_hash?: string;
  audit_seq?: number;
  audit_hash?: string;
  proof_status: string;
  matched_by?: string;
  native_row_id?: string;
  native_row_version?: number;
  audit_action?: string;
  notes?: string[];
};

export type MemoryTimeTravelKVAuditLinksResponse = {
  pack_id: string;
  namespace: string;
  generated_at: string;
  schema_ready: boolean;
  linkage_ready: boolean;
  preview_ready?: boolean;
  native_kv_history_ready: boolean;
  merkle_verification_ready: boolean;
  source: string;
  kv_audit_links: MemoryTimeTravelKVAuditProofLink[];
  required_fields: string[];
  notes?: string[];
};

export type MemoryTimeTravelKVAuditLinksEnvelope = {
  links: MemoryTimeTravelKVAuditLinksResponse;
};

export type MemoryTimeTravelKVAuditProofLinkPreviewRequest = {
  namespace?: string;
  at?: string;
  limit?: number;
  dry_run?: boolean;
};

export type MemoryTimeTravelNativeKVHistoryColumnPlan = {
  name: string;
  type: string;
  nullable: boolean;
  purpose: string;
};

export type MemoryTimeTravelNativeKVHistoryIndexPlan = {
  name: string;
  columns: string[];
  unique?: boolean;
  purpose: string;
};

export type MemoryTimeTravelKVHistoryMigrationStepPlan = {
  step: number;
  name: string;
  from: string;
  to: string;
  dry_run: boolean;
  writes: boolean;
  status: string;
  description: string;
};

export type MemoryTimeTravelNativeKVHistoryPlan = {
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
};

export type MemoryTimeTravelNativeKVHistoryPlanResponse = {
  plan: MemoryTimeTravelNativeKVHistoryPlan;
};

export type MemoryTimeTravelNativeKVHistoryRowPreview = {
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
};

export type MemoryTimeTravelNativeKVHistoryMigrationPreview = {
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
};

export type MemoryTimeTravelNativeKVHistoryMigrationPreviewResponse = {
  kv_history_migration_preview: MemoryTimeTravelNativeKVHistoryMigrationPreview;
};

export type MemoryTimeTravelKVHistoryCutoverPlanRequest = {
  namespace?: string;
  requested_by?: string;
  reason?: string;
  limit?: number;
  dry_run?: boolean;
};

export type MemoryTimeTravelKVHistoryCutoverReadinessRequest = {
  namespace?: string;
  at?: string;
  requested_by?: string;
  reason?: string;
  limit?: number;
  dry_run?: boolean;
};

export type MemoryTimeTravelKVHistoryDualReadParityRequest = {
  namespace?: string;
  at?: string;
  limit?: number;
};

export type MemoryTimeTravelKVHistoryCutoverPhasePlan = {
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
};

export type MemoryTimeTravelKVHistoryDualReadPlan = {
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
};

export type MemoryTimeTravelKVHistoryDualWritePlan = {
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
};

export type MemoryTimeTravelKVHistoryCutoverRollbackPlan = {
  plan_ready: boolean;
  ready: boolean;
  requires_approval: boolean;
  restores_reserved_adapter: boolean;
  drops_native_rows: boolean;
  deletes_reserved_kv_namespace: boolean;
  actions: string[];
  blocked_by: string[];
  notes?: string[];
};

export type MemoryTimeTravelKVHistoryCutoverPlan = {
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
};

export type MemoryTimeTravelKVHistoryCutoverPlanResponse = {
  plan: MemoryTimeTravelKVHistoryCutoverPlan;
};

export type MemoryTimeTravelKVHistoryDualReadParityMismatch = {
  key: string;
  kind: string;
  reserved_value?: string;
  native_preview_value?: string;
  reserved_hash?: string;
  native_preview_hash?: string;
};

export type MemoryTimeTravelKVHistoryDualReadParity = {
  pack_id: string;
  namespace: string;
  temporal_namespace: string;
  generated_at: string;
  at: string;
  stage: string;
  status: string;
  source: string;
  native_table: string;
  current_history_namespace: string;
  limit: number;
  preview_row_count: number;
  returned_preview_row_count: number;
  temporal_key_count: number;
  native_preview_key_count: number;
  matched_key_count: number;
  mismatch_count: number;
  missing_from_native_count: number;
  extra_in_native_count: number;
  value_mismatch_count: number;
  dual_read_parity_check_ready: boolean;
  dual_read_parity_ready: boolean;
  parity_passed: boolean;
  preview_complete: boolean;
  reads_temporal_kv: boolean;
  reads_native_kv_history: boolean;
  reads_native_kv_history_preview: boolean;
  switches_temporal_adapter: boolean;
  writes_ledger_kv: boolean;
  writes_native_kv_history: boolean;
  kv_history_migration_preview: MemoryTimeTravelNativeKVHistoryMigrationPreview;
  mismatches: MemoryTimeTravelKVHistoryDualReadParityMismatch[];
  artifacts: string[];
  actions: string[];
  blocked_by: string[];
  labels: string[];
  notes?: string[];
};

export type MemoryTimeTravelKVHistoryDualReadParityResponse = {
  parity: MemoryTimeTravelKVHistoryDualReadParity;
};

export type MemoryTimeTravelKVHistoryCutoverReadinessGate = {
  name: string;
  ready: boolean;
  required: boolean;
  status: string;
  evidence: string[];
  blocked_by?: string[];
  description: string;
};

export type MemoryTimeTravelKVHistoryCutoverReadiness = {
  pack_id: string;
  namespace: string;
  temporal_namespace: string;
  generated_at: string;
  at: string;
  stage: string;
  status: string;
  dry_run: boolean;
  requested_by?: string;
  reason?: string;
  source: string;
  native_table: string;
  current_history_namespace: string;
  cutover_readiness_check_ready: boolean;
  cutover_ready: boolean;
  native_kv_history_plan_ready: boolean;
  native_kv_history_preview_ready: boolean;
  dual_read_parity_check_ready: boolean;
  dual_read_parity_ready: boolean;
  parity_passed: boolean;
  preview_complete: boolean;
  migration_executor_ready: boolean;
  native_read_adapter_ready: boolean;
  native_write_path_ready: boolean;
  approval_manager_ready: boolean;
  rollback_executor_ready: boolean;
  audit_proof_link_ready: boolean;
  switches_temporal_adapter: boolean;
  writes_ledger_kv: boolean;
  writes_native_kv_history: boolean;
  consumes_cutover_plan: boolean;
  consumes_dual_read_parity: boolean;
  required_gate_count: number;
  passed_gate_count: number;
  blocked_gate_count: number;
  gates: MemoryTimeTravelKVHistoryCutoverReadinessGate[];
  cutover_plan: MemoryTimeTravelKVHistoryCutoverPlan;
  dual_read_parity: MemoryTimeTravelKVHistoryDualReadParity;
  artifacts: string[];
  actions: string[];
  blocked_by: string[];
  labels: string[];
  notes?: string[];
};

export type MemoryTimeTravelKVHistoryCutoverReadinessResponse = {
  readiness: MemoryTimeTravelKVHistoryCutoverReadiness;
};

export type MemoryTimeTravelKVAuditProofLinkPreview = {
  pack_id: string;
  namespace: string;
  temporal_namespace: string;
  generated_at: string;
  at: string;
  stage: string;
  status: string;
  dry_run: boolean;
  source: string;
  native_table: string;
  preview_ready: boolean;
  linkage_ready: boolean;
  kv_audit_link_preview_ready: boolean;
  kv_audit_linkage_ready: boolean;
  native_kv_history_preview_ready: boolean;
  merkle_verification_ready: boolean;
  merkle_append_ready: boolean;
  writes_ledger_kv: boolean;
  writes_native_kv_history: boolean;
  merges_audit_proofs: boolean;
  limit: number;
  preview_row_count: number;
  returned_preview_row_count: number;
  recent_audit_record_count: number;
  candidate_link_count: number;
  matched_link_count: number;
  pending_link_count: number;
  unmatched_row_count: number;
  unmatched_audit_record_count: number;
  schema: MemoryTimeTravelKVAuditLinksResponse;
  kv_history_migration_preview: MemoryTimeTravelNativeKVHistoryMigrationPreview;
  audit_verification: MemoryTimeTravelAuditVerificationResponse;
  candidate_links: MemoryTimeTravelKVAuditProofLink[];
  unmatched_rows: MemoryTimeTravelNativeKVHistoryRowPreview[];
  unmatched_audit_records: MemoryTimeTravelAuditRecord[];
  artifacts: string[];
  actions: string[];
  blocked_by: string[];
  labels: string[];
  notes?: string[];
};

export type MemoryTimeTravelKVAuditProofLinkPreviewResponse = {
  preview: MemoryTimeTravelKVAuditProofLinkPreview;
};

export type MemoryTimeTravelKVAuditProofLinkWritebackPlanRequest = {
  namespace?: string;
  at?: string;
  limit?: number;
  requested_by?: string;
  reason?: string;
  approval_id?: string;
  dry_run?: boolean;
};

export type MemoryTimeTravelKVAuditProofLinkWritebackActionPlan = {
  operation: string;
  namespace: string;
  key: string;
  native_row_id: string;
  native_row_version: number;
  kv_version_at?: string;
  value_hash?: string;
  audit_seq?: number;
  audit_hash?: string;
  audit_action?: string;
  proof_status: string;
  requires_approval: boolean;
  approval_id?: string;
  generated_at: string;
};

type MTTWB = {
  kv_audit_link_writeback_ready: boolean;
  kv_audit_linkage_ready: boolean;
  audit_proof_link_ready: boolean;
  writes_ledger_kv: boolean;
  writes_native_kv_history: boolean;
  backfills_audit_seq: boolean;
  backfills_audit_hash: boolean;
  merkle_append_ready: boolean;
};

type MTTAB = {
  approval_request_plan_ready: boolean;
  approval_manager_bridge_plan_ready: boolean;
  global_approval_enqueue_ready: boolean;
};

type MTTAF = {
  artifacts: string[];
  actions: string[];
  blocked_by: string[];
  labels: string[];
};

export type MemoryTimeTravelKVAuditProofLinkWritebackPlan = MTTWB & MTTAB & MTTAF & {
  pack_id: string;
  namespace: string;
  temporal_namespace: string;
  generated_at: string;
  at: string;
  stage: string;
  status: string;
  dry_run: boolean;
  requested_by?: string;
  reason?: string;
  approval_id?: string;
  kv_audit_link_writeback_plan_ready: boolean;
  consumes_audit_link_preview: boolean;
  native_kv_history_preview_ready: boolean;
  merkle_verification_ready: boolean;
  appends_merkle: boolean;
  candidate_link_count: number;
  matched_link_count: number;
  pending_link_count: number;
  unmatched_row_count: number;
  unmatched_audit_record_count: number;
  action_count: number;
  audit_link_preview: MemoryTimeTravelKVAuditProofLinkPreview;
  proposed_approval_request: MemoryTimeTravelGlobalApprovalRequestPlan;
  writeback_actions: MemoryTimeTravelKVAuditProofLinkWritebackActionPlan[];
};

export type MemoryTimeTravelKVAuditProofLinkWritebackPlanResponse = {
  plan: MemoryTimeTravelKVAuditProofLinkWritebackPlan;
};

export type MemoryTimeTravelKVAuditProofLinkWritebackStoreRequest = MemoryTimeTravelKVAuditProofLinkWritebackPlanRequest;

export type MemoryTimeTravelKVAuditProofLinkWritebackStoreSummary = Pick<MTTWB, "kv_audit_link_writeback_ready" | "kv_audit_linkage_ready" | "writes_ledger_kv" | "writes_native_kv_history" | "backfills_audit_seq" | "backfills_audit_hash" | "merkle_append_ready"> & {
  pack_id: string;
  store: string;
  store_ready: boolean;
  kv_audit_link_writeback_store_ready: boolean;
  writes_audit_link_writeback_store: boolean;
  record_count: number;
  artifact: string;
  record_artifact: string;
};

export type MemoryTimeTravelKVAuditProofLinkWritebackRecord = MTTWB & MTTAB & MTTAF & {
  pack_id: string;
  store_name: string;
  record_id: string;
  request_id: string;
  request_key: string;
  namespace: string;
  temporal_namespace: string;
  status: string;
  requested_by?: string;
  reason?: string;
  approval_id?: string;
  created_at: string;
  updated_at: string;
  kv_audit_link_writeback_store_ready: boolean;
  kv_audit_link_writeback_plan_ready: boolean;
  writes_audit_link_writeback_store: boolean;
  appends_merkle: boolean;
  candidate_link_count: number;
  matched_link_count: number;
  pending_link_count: number;
  action_count: number;
  proposed_approval_request: MemoryTimeTravelGlobalApprovalRequestPlan;
  writeback_actions: MemoryTimeTravelKVAuditProofLinkWritebackActionPlan[];
  plan_summary: MemoryTimeTravelKVAuditProofLinkWritebackPlan;
  store_artifact: string;
  record_artifact: string;
};

export type MemoryTimeTravelKVAuditProofLinkWritebackStore = MTTWB & MTTAB & MTTAF & {
  pack_id: string;
  generated_at: string;
  status: string;
  kv_audit_link_writeback_store_ready: boolean;
  kv_audit_link_writeback_plan_ready: boolean;
  writes_audit_link_writeback_store: boolean;
  appends_merkle: boolean;
  request_id: string;
  request_key: string;
  approval_id?: string;
  record_id: string;
  namespace: string;
  temporal_namespace: string;
  candidate_link_count: number;
  matched_link_count: number;
  pending_link_count: number;
  action_count: number;
  writeback_actions: MemoryTimeTravelKVAuditProofLinkWritebackActionPlan[];
  approval_queue_record: MemoryTimeTravelKVAuditProofLinkWritebackRecord;
  audit_link_writeback_store: MemoryTimeTravelKVAuditProofLinkWritebackStoreSummary;
  plan_summary: MemoryTimeTravelKVAuditProofLinkWritebackPlan;
};

export type MemoryTimeTravelKVAuditProofLinkWritebackStoreResponse = {
  writeback: MemoryTimeTravelKVAuditProofLinkWritebackStore;
};

export type MemoryTimeTravelKVAuditProofLinkWritebackExecutorPlanRequest = Partial<Record<"record_id" | "request_id" | "request_key" | "namespace" | "requested_by" | "reason", string>> & {
  dry_run?: boolean;
};

export type MemoryTimeTravelKVAuditProofLinkExecutorHandoffPlan = {
  target: string;
  source_store: string;
  source_record_artifact: string;
  record_id: string;
  request_id: string;
  request_key: string;
  namespace: string;
  temporal_namespace: string;
  dedup_key: string;
  consumes_audit_link_writeback_store: boolean;
  executor_input_contract_ready: boolean;
  audit_proof_link_executor_ready: boolean;
  action_count: number;
  action_keys: string[];
  actions: string[];
  blocked_by: string[];
};

export type MemoryTimeTravelKVAuditProofLinkExecutorAuditAppendPlan = {
  audit_append_plan_ready: boolean;
  merkle_append_ready: boolean;
  chain: string;
  event_type: string;
  subject: string;
  payload_digest: string;
  dedup_key: string;
  writes_audit_chain: boolean;
  actions: string[];
  blocked_by: string[];
};

export type MemoryTimeTravelKVAuditProofLinkWritebackExecutorPlan = MTTWB & MTTAB & MTTAF & {
  pack_id: string;
  generated_at: string;
  status: string;
  stage: string;
  dry_run: boolean;
  record_id: string;
  request_id: string;
  request_key: string;
  namespace: string;
  temporal_namespace: string;
  requested_by?: string;
  reason?: string;
  kv_audit_link_writeback_executor_plan_ready: boolean;
  executor_input_contract_ready: boolean;
  consumes_audit_link_writeback_store: boolean;
  kv_audit_link_writeback_store_ready: boolean;
  audit_proof_link_executor_ready: boolean;
  audit_append_plan_ready: boolean;
  writes_audit_chain: boolean;
  appends_merkle: boolean;
  action_count: number;
  audit_link_writeback_record: MemoryTimeTravelKVAuditProofLinkWritebackRecord;
  audit_link_writeback_store: MemoryTimeTravelKVAuditProofLinkWritebackStoreSummary;
  executor_handoff_plan: MemoryTimeTravelKVAuditProofLinkExecutorHandoffPlan;
  audit_append_plan: MemoryTimeTravelKVAuditProofLinkExecutorAuditAppendPlan;
  writeback_actions: MemoryTimeTravelKVAuditProofLinkWritebackActionPlan[];
};

export type MemoryTimeTravelKVAuditProofLinkWritebackExecutorPlanResponse = {
  plan: MemoryTimeTravelKVAuditProofLinkWritebackExecutorPlan;
};

export type MemoryTimeTravelSnapshotsResponse = {
  snapshots: MemoryTimeTravelSnapshotSummary[];
  count: number;
};

export type MemoryTimeTravelSaveSnapshotRequest = {
  id?: string;
  namespace?: string;
  source?: string;
  reason?: string;
  values: Record<string, string>;
  dry_run?: boolean;
};

export type MemoryTimeTravelSaveSnapshotResponse = {
  snapshot: MemoryTimeTravelSnapshot;
  status: string;
};

export type MemoryTimeTravelSnapshotResponse = {
  snapshot: MemoryTimeTravelSnapshot;
};

export type MemoryTimeTravelSnapshotAtRequest = {
  namespace?: string;
  at?: string;
};

export type MemoryTimeTravelSnapshotAtResponse = {
  namespace: string;
  at: string;
  snapshot?: MemoryTimeTravelSnapshot;
  values: Record<string, string>;
  matched_id?: string;
  status: string;
};

export type MemoryTimeTravelDiffRequest = {
  namespace?: string;
  base_id: string;
  target_id: string;
};

export type MemoryTimeTravelDiffEntry = {
  key: string;
  change: string;
  before?: string;
  after?: string;
  before_hash?: string;
  after_hash?: string;
  impact_level: string;
};

export type MemoryTimeTravelDiffReport = {
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
};

export type MemoryTimeTravelDiffResponse = {
  diff: MemoryTimeTravelDiffReport;
};

export type MemoryTimeTravelRollbackPlanRequest = {
  namespace?: string;
  snapshot_id: string;
  dry_run?: boolean;
};

export type MemoryTimeTravelRollbackPlan = {
  pack_id: string;
  namespace: string;
  snapshot_id: string;
  dry_run: boolean;
  action_count: number;
  actions: string[];
  preview_values?: Record<string, string>;
  status: string;
  notes?: string[];
};

export type MemoryTimeTravelRollbackPlanResponse = {
  plan: MemoryTimeTravelRollbackPlan;
};

export type MemoryTimeTravelApprovedRollbackPlanRequest = {
  namespace?: string;
  snapshot_id: string;
  requested_by?: string;
  reason?: string;
  approval_id?: string;
  dry_run?: boolean;
};

export type MemoryTimeTravelRollbackWritebackActionPlan = {
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
};

export type MemoryTimeTravelGlobalApprovalRequestPlan = {
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
};

export type MemoryTimeTravelApprovedRollbackPlan = {
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
};

export type MemoryTimeTravelApprovedRollbackPlanResponse = {
  plan: MemoryTimeTravelApprovedRollbackPlan;
};

export type MemoryTimeTravelRetentionCandidate = {
  id: string;
  namespace: string;
  created_at: string;
  hash: string;
  size_bytes: number;
  key_count: number;
  reasons: string[];
  action: string;
};

export type MemoryTimeTravelRetentionPlan = {
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
};

export type MemoryTimeTravelRetentionPlanResponse = {
  plan: MemoryTimeTravelRetentionPlan;
};

export type MemoryTimeTravelRetentionPrunePlanRequest = {
  namespace?: string;
  candidate_ids?: string[];
  reason?: string;
  requested_by?: string;
  dry_run?: boolean;
};

export type MemoryTimeTravelRetentionPrunePlan = {
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
};

export type MemoryTimeTravelRetentionPrunePlanResponse = {
  plan: MemoryTimeTravelRetentionPrunePlan;
};

export type MemoryTimeTravelRetentionPruneExecuteRequest = {
  namespace?: string;
  candidate_ids?: string[];
  requested_by?: string;
  reason?: string;
  approval_id?: string;
  approved?: boolean;
  dry_run?: boolean;
};

export type MemoryTimeTravelRetentionPruneSkippedCandidate = {
  id: string;
  namespace?: string;
  reason: string;
};

export type MemoryTimeTravelRetentionPruneExecute = {
  pack_id: string;
  namespace: string;
  generated_at: string;
  stage: string;
  status: string;
  dry_run: boolean;
  approved: boolean;
  approval_required: boolean;
  approval_id?: string;
  requested_by?: string;
  reason?: string;
  pack_local_prune_ready: boolean;
  retention_pack_local_prune_ready: boolean;
  retention_prune_ready: boolean;
  temporal_prune_ready: boolean;
  writes_pack_local_snapshot_store: boolean;
  writes_ledger_kv: boolean;
  writes_temporal_kv: boolean;
  writes_native_kv_history: boolean;
  merkle_append_ready: boolean;
  cron_ready: boolean;
  candidate_count: number;
  selected_candidate_count: number;
  deleted_candidate_count: number;
  skipped_candidate_count: number;
  reclaimable_bytes: number;
  deleted_bytes: number;
  action_count: number;
  snapshot_count_after: number;
  retention_plan_generated_at: string;
  retention_prune_plan: MemoryTimeTravelRetentionPrunePlan;
  deleted_candidates: MemoryTimeTravelRetentionCandidate[];
  skipped_candidates: MemoryTimeTravelRetentionPruneSkippedCandidate[];
  artifacts: string[];
  actions: string[];
  blocked_by: string[];
  labels: string[];
  notes?: string[];
};

export type MemoryTimeTravelRetentionPruneExecuteResponse = {
  prune: MemoryTimeTravelRetentionPruneExecute;
};

export type MemoryTimeTravelEvidenceResponse = {
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
  retention_prune_execute?: MemoryTimeTravelRetentionPruneExecute;
  native_kv_history_plan?: MemoryTimeTravelNativeKVHistoryPlan;
  kv_history_migration_plan?: MemoryTimeTravelKVHistoryMigrationStepPlan[];
  kv_history_index_plan?: MemoryTimeTravelNativeKVHistoryIndexPlan[];
  kv_history_migration_preview?: MemoryTimeTravelNativeKVHistoryMigrationPreview;
  kv_history_migration_preview_error?: string;
  kv_history_dual_read_parity?: MemoryTimeTravelKVHistoryDualReadParity;
  kv_history_dual_read_parity_error?: string;
  kv_history_cutover_plan?: MemoryTimeTravelKVHistoryCutoverPlan;
  kv_history_cutover_plan_error?: string;
  kv_history_cutover_readiness?: MemoryTimeTravelKVHistoryCutoverReadiness;
  kv_history_cutover_readiness_error?: string;
  kv_history_dual_read_plan?: MemoryTimeTravelKVHistoryDualReadPlan;
  kv_history_dual_write_plan?: MemoryTimeTravelKVHistoryDualWritePlan;
  kv_history_cutover_rollback_plan?: MemoryTimeTravelKVHistoryCutoverRollbackPlan;
  kv_audit_link_schema?: MemoryTimeTravelKVAuditLinksResponse;
  kv_audit_links?: MemoryTimeTravelKVAuditProofLink[];
  kv_audit_link_preview?: MemoryTimeTravelKVAuditProofLinkPreview;
  kv_audit_link_preview_error?: string;
  kv_audit_link_writeback_plan?: MemoryTimeTravelKVAuditProofLinkWritebackPlan;
  kv_audit_link_writeback_plan_error?: string;
  kv_audit_link_writeback_actions?: MemoryTimeTravelKVAuditProofLinkWritebackActionPlan[];
  kv_audit_link_writeback_store?: MemoryTimeTravelKVAuditProofLinkWritebackStoreSummary;
  kv_audit_link_writeback_store_error?: string;
  kv_audit_link_writeback_records?: MemoryTimeTravelKVAuditProofLinkWritebackRecord[];
  kv_audit_link_writeback_executor_plan?: MemoryTimeTravelKVAuditProofLinkWritebackExecutorPlan;
  kv_audit_link_writeback_executor_plan_error?: string;
  audit_link_executor_handoff_plan?: MemoryTimeTravelKVAuditProofLinkExecutorHandoffPlan;
  audit_link_executor_audit_plan?: MemoryTimeTravelKVAuditProofLinkExecutorAuditAppendPlan;
  audit_verification?: MemoryTimeTravelAuditVerificationResponse;
  audit_verification_error?: string;
};

export type MemoryTimeTravelClientOptions = {
  baseUrl: string;
  token?: string;
  apiKey?: string;
  headers?: HeadersInit;
  fetch?: typeof fetch;
};

export class MemoryTimeTravelClientError extends Error {
  readonly status: number;
  readonly body: unknown;

  constructor(status: number, body: unknown, message?: string) {
    super(message || `Memory Time Travel request failed with HTTP ${status}`);
    this.name = "MemoryTimeTravelClientError";
    this.status = status;
    this.body = body;
  }
}

function trimBaseUrl(baseUrl: string): string {
  return baseUrl.replace(/\/+$/, "");
}

function mergeHeaders(base: HeadersInit | undefined, extra?: HeadersInit): Headers {
  const headers = new Headers(base);
  if (!extra) return headers;
  new Headers(extra).forEach((value, key) => headers.set(key, value));
  return headers;
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

function messageFromErrorBody(body: unknown): string | undefined {
  if (typeof body === "string" && body.trim()) return body.trim();
  if (!isRecord(body)) return undefined;
  for (const key of ["message", "detail", "error", "reason"]) {
    const value = body[key];
    if (typeof value === "string" && value.trim()) return value;
    if (key === "error" && isRecord(value)) {
      const nested = messageFromErrorBody(value);
      if (nested) return nested;
    }
  }
  return undefined;
}

async function parseResponse(response: Response): Promise<unknown> {
  const text = await response.text();
  if (!text) return undefined;
  try {
    return JSON.parse(text);
  } catch {
    return text;
  }
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

export class MemoryTimeTravelClient {
  private readonly baseUrl: string;
  private readonly fetchImpl: typeof fetch;
  private readonly headers: HeadersInit | undefined;
  private readonly token: string | undefined;
  private readonly apiKey: string | undefined;

  constructor(options: MemoryTimeTravelClientOptions) {
    if (!options.baseUrl) throw new Error("MemoryTimeTravelClient requires baseUrl");
    const fetchImpl = options.fetch ?? globalThis.fetch;
    if (!fetchImpl) throw new Error("MemoryTimeTravelClient requires a fetch implementation");
    this.baseUrl = trimBaseUrl(options.baseUrl);
    this.fetchImpl = fetchImpl.bind(globalThis) as typeof fetch;
    this.headers = options.headers;
    this.token = options.token;
    this.apiKey = options.apiKey;
  }

  status(): Promise<MemoryTimeTravelStatusResponse> {
    return this.request<MemoryTimeTravelStatusResponse>("GET", "/v1/memory-time-travel/status");
  }

  snapshots(namespace?: string): Promise<MemoryTimeTravelSnapshotsResponse> {
    return this.request<MemoryTimeTravelSnapshotsResponse>("GET", `/v1/memory-time-travel/snapshots${query({ namespace })}`);
  }

  saveSnapshot(input: MemoryTimeTravelSaveSnapshotRequest): Promise<MemoryTimeTravelSaveSnapshotResponse> {
    return this.request<MemoryTimeTravelSaveSnapshotResponse>("POST", "/v1/memory-time-travel/snapshots", input);
  }

  snapshot(id: string): Promise<MemoryTimeTravelSnapshotResponse> {
    return this.request<MemoryTimeTravelSnapshotResponse>("GET", `/v1/memory-time-travel/snapshots/${enc(id)}`);
  }

  snapshotAt(input: MemoryTimeTravelSnapshotAtRequest): Promise<MemoryTimeTravelSnapshotAtResponse> {
    return this.request<MemoryTimeTravelSnapshotAtResponse>("POST", "/v1/memory-time-travel/snapshot-at", input);
  }

  diff(input: MemoryTimeTravelDiffRequest): Promise<MemoryTimeTravelDiffResponse> {
    return this.request<MemoryTimeTravelDiffResponse>("POST", "/v1/memory-time-travel/diff", input);
  }

  rollbackPlan(input: MemoryTimeTravelRollbackPlanRequest): Promise<MemoryTimeTravelRollbackPlanResponse> {
    return this.request<MemoryTimeTravelRollbackPlanResponse>("POST", "/v1/memory-time-travel/rollback-plan", input);
  }

  approvedRollbackPlan(input: MemoryTimeTravelApprovedRollbackPlanRequest): Promise<MemoryTimeTravelApprovedRollbackPlanResponse> {
    return this.request<MemoryTimeTravelApprovedRollbackPlanResponse>("POST", "/v1/memory-time-travel/rollback/approved-plan", input);
  }

  retentionPlan(namespace?: string): Promise<MemoryTimeTravelRetentionPlanResponse> {
    return this.request<MemoryTimeTravelRetentionPlanResponse>("GET", `/v1/memory-time-travel/retention/plan${query({ namespace })}`);
  }

  retentionPrunePlan(input: MemoryTimeTravelRetentionPrunePlanRequest = {}): Promise<MemoryTimeTravelRetentionPrunePlanResponse> {
    return this.request<MemoryTimeTravelRetentionPrunePlanResponse>("POST", "/v1/memory-time-travel/retention/prune-plan", input);
  }

  retentionPruneExecute(input: MemoryTimeTravelRetentionPruneExecuteRequest = {}): Promise<MemoryTimeTravelRetentionPruneExecuteResponse> {
    return this.request<MemoryTimeTravelRetentionPruneExecuteResponse>("POST", "/v1/memory-time-travel/retention/prune/execute", input);
  }

  nativeKVHistoryPlan(namespace?: string): Promise<MemoryTimeTravelNativeKVHistoryPlanResponse> {
    return this.request<MemoryTimeTravelNativeKVHistoryPlanResponse>("GET", `/v1/memory-time-travel/kv-history/native-plan${query({ namespace })}`);
  }

  nativeKVHistoryMigrationPreview(namespace?: string, limit?: number): Promise<MemoryTimeTravelNativeKVHistoryMigrationPreviewResponse> {
    return this.request<MemoryTimeTravelNativeKVHistoryMigrationPreviewResponse>("GET", `/v1/memory-time-travel/kv-history/migration-preview${query({ namespace, limit: limit ? String(limit) : undefined })}`);
  }

  kvHistoryDualReadParity(input: MemoryTimeTravelKVHistoryDualReadParityRequest = {}): Promise<MemoryTimeTravelKVHistoryDualReadParityResponse> {
    return this.request<MemoryTimeTravelKVHistoryDualReadParityResponse>("POST", "/v1/memory-time-travel/kv-history/dual-read/parity", input);
  }

  kvHistoryCutoverPlan(input: MemoryTimeTravelKVHistoryCutoverPlanRequest = {}): Promise<MemoryTimeTravelKVHistoryCutoverPlanResponse> {
    return this.request<MemoryTimeTravelKVHistoryCutoverPlanResponse>("POST", "/v1/memory-time-travel/kv-history/cutover/plan", input);
  }

  kvHistoryCutoverReadiness(input: MemoryTimeTravelKVHistoryCutoverReadinessRequest = {}): Promise<MemoryTimeTravelKVHistoryCutoverReadinessResponse> {
    return this.request<MemoryTimeTravelKVHistoryCutoverReadinessResponse>("POST", "/v1/memory-time-travel/kv-history/cutover/readiness", input);
  }

  auditLinks(namespace?: string): Promise<MemoryTimeTravelKVAuditLinksEnvelope> {
    return this.request<MemoryTimeTravelKVAuditLinksEnvelope>("GET", `/v1/memory-time-travel/audit/links${query({ namespace })}`);
  }

  auditLinksPreview(input: MemoryTimeTravelKVAuditProofLinkPreviewRequest = {}): Promise<MemoryTimeTravelKVAuditProofLinkPreviewResponse> {
    return this.request<MemoryTimeTravelKVAuditProofLinkPreviewResponse>("POST", "/v1/memory-time-travel/audit/links/preview", input);
  }

  auditLinksWritebackPlan(input: MemoryTimeTravelKVAuditProofLinkWritebackPlanRequest = {}): Promise<MemoryTimeTravelKVAuditProofLinkWritebackPlanResponse> {
    return this.request<MemoryTimeTravelKVAuditProofLinkWritebackPlanResponse>("POST", "/v1/memory-time-travel/audit/links/writeback-plan", input);
  }

  auditLinksWritebackStore(input: MemoryTimeTravelKVAuditProofLinkWritebackStoreRequest = {}): Promise<MemoryTimeTravelKVAuditProofLinkWritebackStoreResponse> {
    return this.request<MemoryTimeTravelKVAuditProofLinkWritebackStoreResponse>("POST", "/v1/memory-time-travel/audit/links/writeback/store", input);
  }

  auditLinksWritebackExecutorPlan(input: MemoryTimeTravelKVAuditProofLinkWritebackExecutorPlanRequest = {}): Promise<MemoryTimeTravelKVAuditProofLinkWritebackExecutorPlanResponse> {
    return this.request<MemoryTimeTravelKVAuditProofLinkWritebackExecutorPlanResponse>("POST", "/v1/memory-time-travel/audit/links/writeback/executor/plan", input);
  }

  evidence(id: string): Promise<MemoryTimeTravelEvidenceResponse> {
    return this.request<MemoryTimeTravelEvidenceResponse>("GET", `/v1/memory-time-travel/evidence/${enc(id)}`);
  }

  auditVerify(limit?: number): Promise<MemoryTimeTravelAuditVerificationResponse> {
    return this.request<MemoryTimeTravelAuditVerificationResponse>("GET", `/v1/memory-time-travel/audit/verify${query({ limit: limit ? String(limit) : undefined })}`);
  }

  private async request<T>(method: "GET" | "POST", path: string, body?: unknown): Promise<T> {
    const headers = mergeHeaders(this.headers);
    if (this.token && !headers.has("authorization")) headers.set("Authorization", `Bearer ${this.token}`);
    if (this.apiKey && !headers.has("x-api-key")) headers.set("X-API-Key", this.apiKey);

    const init: RequestInit = { method, headers };
    if (body !== undefined) {
      headers.set("Content-Type", "application/json");
      init.body = JSON.stringify(body);
    }

    const response = await this.fetchImpl(new URL(`${this.baseUrl}${path}`), init);
    const parsed = await parseResponse(response);
    if (!response.ok) throw new MemoryTimeTravelClientError(response.status, parsed, messageFromErrorBody(parsed));
    return parsed as T;
  }
}

export function createMemoryTimeTravelClient(options: MemoryTimeTravelClientOptions): MemoryTimeTravelClient {
  return new MemoryTimeTravelClient(options);
}
