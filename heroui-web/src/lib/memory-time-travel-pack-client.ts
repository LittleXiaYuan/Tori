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
  matched_by?: string;
  native_row_id?: string;
  native_row_version?: number;
  audit_action?: string;
  notes?: string[];
}

export interface MemoryTimeTravelKVAuditLinksReport {
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

export interface MemoryTimeTravelKVHistoryCutoverReadinessInput {
  namespace?: string;
  at?: string;
  requested_by?: string;
  reason?: string;
  limit?: number;
  dry_run?: boolean;
}

export interface MemoryTimeTravelKVAuditProofLinkPreviewInput {
  namespace?: string;
  at?: string;
  limit?: number;
  dry_run?: boolean;
}

export interface MemoryTimeTravelKVHistoryDualReadParityInput {
  namespace?: string;
  at?: string;
  limit?: number;
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

export interface MemoryTimeTravelKVHistoryDualReadParityMismatch {
  key: string;
  kind: string;
  reserved_value?: string;
  native_preview_value?: string;
  reserved_hash?: string;
  native_preview_hash?: string;
}

export interface MemoryTimeTravelKVHistoryDualReadParity {
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
}

export interface MemoryTimeTravelKVHistoryCutoverReadinessGate {
  name: string;
  ready: boolean;
  required: boolean;
  status: string;
  evidence: string[];
  blocked_by?: string[];
  description: string;
}

export interface MemoryTimeTravelKVHistoryCutoverReadiness {
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
}

export interface MemoryTimeTravelKVAuditProofLinkPreview {
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
  schema: MemoryTimeTravelKVAuditLinksReport;
  kv_history_migration_preview: MemoryTimeTravelNativeKVHistoryMigrationPreview;
  audit_verification: MemoryTimeTravelAuditVerification;
  candidate_links: MemoryTimeTravelKVAuditProofLink[];
  unmatched_rows: MemoryTimeTravelNativeKVHistoryRowPreview[];
  unmatched_audit_records: MemoryTimeTravelAuditRecord[];
  artifacts: string[];
  actions: string[];
  blocked_by: string[];
  labels: string[];
  notes?: string[];
}

export interface MemoryTimeTravelKVAuditProofLinkWritebackPlanInput {
  namespace?: string;
  at?: string;
  limit?: number;
  requested_by?: string;
  reason?: string;
  approval_id?: string;
  dry_run?: boolean;
}

export interface MemoryTimeTravelKVAuditProofLinkWritebackActionPlan {
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
}

export interface MemoryTimeTravelKVAuditProofLinkWritebackPlan {
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
  kv_audit_link_writeback_ready: boolean;
  kv_audit_linkage_ready: boolean;
  audit_proof_link_ready: boolean;
  approval_request_plan_ready: boolean;
  approval_manager_bridge_plan_ready: boolean;
  global_approval_enqueue_ready: boolean;
  consumes_audit_link_preview: boolean;
  native_kv_history_preview_ready: boolean;
  merkle_verification_ready: boolean;
  merkle_append_ready: boolean;
  writes_ledger_kv: boolean;
  writes_native_kv_history: boolean;
  backfills_audit_seq: boolean;
  backfills_audit_hash: boolean;
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
  artifacts: string[];
  actions: string[];
  blocked_by: string[];
  labels: string[];
  notes?: string[];
}

export interface MemoryTimeTravelKVAuditProofLinkWritebackStoreInput extends MemoryTimeTravelKVAuditProofLinkWritebackPlanInput {}

export interface MemoryTimeTravelKVAuditProofLinkWritebackStoreSummary {
  pack_id: string;
  store: string;
  store_ready: boolean;
  kv_audit_link_writeback_store_ready: boolean;
  kv_audit_link_writeback_ready: boolean;
  kv_audit_linkage_ready: boolean;
  writes_audit_link_writeback_store: boolean;
  writes_ledger_kv: boolean;
  writes_native_kv_history: boolean;
  backfills_audit_seq: boolean;
  backfills_audit_hash: boolean;
  merkle_append_ready: boolean;
  record_count: number;
  artifact: string;
  record_artifact: string;
  notes?: string[];
}

export interface MemoryTimeTravelKVAuditProofLinkWritebackRecord {
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
  kv_audit_link_writeback_ready: boolean;
  kv_audit_linkage_ready: boolean;
  audit_proof_link_ready: boolean;
  approval_request_plan_ready: boolean;
  approval_manager_bridge_plan_ready: boolean;
  global_approval_enqueue_ready: boolean;
  writes_audit_link_writeback_store: boolean;
  writes_ledger_kv: boolean;
  writes_native_kv_history: boolean;
  backfills_audit_seq: boolean;
  backfills_audit_hash: boolean;
  merkle_append_ready: boolean;
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
  artifacts: string[];
  actions: string[];
  blocked_by: string[];
  labels: string[];
  notes?: string[];
}

export interface MemoryTimeTravelKVAuditProofLinkWritebackStore {
  pack_id: string;
  generated_at: string;
  status: string;
  kv_audit_link_writeback_store_ready: boolean;
  kv_audit_link_writeback_plan_ready: boolean;
  kv_audit_link_writeback_ready: boolean;
  kv_audit_linkage_ready: boolean;
  audit_proof_link_ready: boolean;
  approval_request_plan_ready: boolean;
  approval_manager_bridge_plan_ready: boolean;
  global_approval_enqueue_ready: boolean;
  writes_audit_link_writeback_store: boolean;
  writes_ledger_kv: boolean;
  writes_native_kv_history: boolean;
  backfills_audit_seq: boolean;
  backfills_audit_hash: boolean;
  merkle_append_ready: boolean;
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
  artifacts: string[];
  actions: string[];
  blocked_by: string[];
  labels: string[];
  notes?: string[];
}


export interface MemoryTimeTravelKVAuditProofLinkWritebackExecutorPlanInput {
  record_id?: string;
  request_id?: string;
  request_key?: string;
  namespace?: string;
  requested_by?: string;
  reason?: string;
  dry_run?: boolean;
}

export interface MemoryTimeTravelKVAuditProofLinkExecutorHandoffPlan {
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
  approval_required: boolean;
  global_approval_enqueue_ready: boolean;
  writes_native_kv_history: boolean;
  writes_ledger_kv: boolean;
  backfills_audit_seq: boolean;
  backfills_audit_hash: boolean;
  merkle_append_ready: boolean;
  appends_merkle: boolean;
  action_count: number;
  action_keys: string[];
  actions: string[];
  blocked_by: string[];
  notes?: string[];
}

export interface MemoryTimeTravelKVAuditProofLinkExecutorAuditAppendPlan {
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
  notes?: string[];
}

export interface MemoryTimeTravelKVAuditProofLinkWritebackExecutorPlan {
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
  kv_audit_link_writeback_ready: boolean;
  kv_audit_linkage_ready: boolean;
  audit_proof_link_executor_ready: boolean;
  audit_proof_link_ready: boolean;
  approval_request_plan_ready: boolean;
  approval_manager_bridge_plan_ready: boolean;
  global_approval_enqueue_ready: boolean;
  audit_append_plan_ready: boolean;
  merkle_append_ready: boolean;
  writes_audit_chain: boolean;
  writes_ledger_kv: boolean;
  writes_native_kv_history: boolean;
  backfills_audit_seq: boolean;
  backfills_audit_hash: boolean;
  appends_merkle: boolean;
  action_count: number;
  audit_link_writeback_record: MemoryTimeTravelKVAuditProofLinkWritebackRecord;
  audit_link_writeback_store: MemoryTimeTravelKVAuditProofLinkWritebackStoreSummary;
  executor_handoff_plan: MemoryTimeTravelKVAuditProofLinkExecutorHandoffPlan;
  audit_append_plan: MemoryTimeTravelKVAuditProofLinkExecutorAuditAppendPlan;
  writeback_actions: MemoryTimeTravelKVAuditProofLinkWritebackActionPlan[];
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
  kv_history_dual_read_parity?: MemoryTimeTravelKVHistoryDualReadParity;
  kv_history_dual_read_parity_error?: string;
  kv_history_cutover_plan?: MemoryTimeTravelKVHistoryCutoverPlan;
  kv_history_cutover_plan_error?: string;
  kv_history_cutover_readiness?: MemoryTimeTravelKVHistoryCutoverReadiness;
  kv_history_cutover_readiness_error?: string;
  kv_history_dual_read_plan?: MemoryTimeTravelKVHistoryDualReadPlan;
  kv_history_dual_write_plan?: MemoryTimeTravelKVHistoryDualWritePlan;
  kv_history_cutover_rollback_plan?: MemoryTimeTravelKVHistoryCutoverRollbackPlan;
  kv_audit_link_schema?: MemoryTimeTravelKVAuditLinksReport;
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
  kvHistoryDualReadParity(input?: MemoryTimeTravelKVHistoryDualReadParityInput): Promise<{ parity: MemoryTimeTravelKVHistoryDualReadParity }>;
  kvHistoryCutoverPlan(input?: MemoryTimeTravelKVHistoryCutoverPlanInput): Promise<{ plan: MemoryTimeTravelKVHistoryCutoverPlan }>;
  kvHistoryCutoverReadiness(input?: MemoryTimeTravelKVHistoryCutoverReadinessInput): Promise<{ readiness: MemoryTimeTravelKVHistoryCutoverReadiness }>;
  auditLinks(namespace?: string): Promise<{ links: MemoryTimeTravelKVAuditLinksReport }>;
  auditLinksPreview(input?: MemoryTimeTravelKVAuditProofLinkPreviewInput): Promise<{ preview: MemoryTimeTravelKVAuditProofLinkPreview }>;
  auditLinksWritebackPlan(input?: MemoryTimeTravelKVAuditProofLinkWritebackPlanInput): Promise<{ plan: MemoryTimeTravelKVAuditProofLinkWritebackPlan }>;
  auditLinksWritebackStore(input?: MemoryTimeTravelKVAuditProofLinkWritebackStoreInput): Promise<{ writeback: MemoryTimeTravelKVAuditProofLinkWritebackStore }>;
  auditLinksWritebackExecutorPlan(input?: MemoryTimeTravelKVAuditProofLinkWritebackExecutorPlanInput): Promise<{ plan: MemoryTimeTravelKVAuditProofLinkWritebackExecutorPlan }>;
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
    kvHistoryDualReadParity: (input = {}) =>
      fetcher<{ parity: MemoryTimeTravelKVHistoryDualReadParity }>("/v1/memory-time-travel/kv-history/dual-read/parity", {
        method: "POST",
        body: JSON.stringify(input),
      }),
    kvHistoryCutoverPlan: (input = {}) =>
      fetcher<{ plan: MemoryTimeTravelKVHistoryCutoverPlan }>("/v1/memory-time-travel/kv-history/cutover/plan", {
        method: "POST",
        body: JSON.stringify(input),
      }),
    kvHistoryCutoverReadiness: (input = {}) =>
      fetcher<{ readiness: MemoryTimeTravelKVHistoryCutoverReadiness }>("/v1/memory-time-travel/kv-history/cutover/readiness", {
        method: "POST",
        body: JSON.stringify(input),
      }),
    auditLinks: (namespace) =>
      fetcher<{ links: MemoryTimeTravelKVAuditLinksReport }>(`/v1/memory-time-travel/audit/links${query({ namespace })}`),
    auditLinksPreview: (input = {}) =>
      fetcher<{ preview: MemoryTimeTravelKVAuditProofLinkPreview }>("/v1/memory-time-travel/audit/links/preview", {
        method: "POST",
        body: JSON.stringify(input),
      }),
    auditLinksWritebackPlan: (input = {}) =>
      fetcher<{ plan: MemoryTimeTravelKVAuditProofLinkWritebackPlan }>("/v1/memory-time-travel/audit/links/writeback-plan", {
        method: "POST",
        body: JSON.stringify(input),
      }),
    auditLinksWritebackStore: (input = {}) =>
      fetcher<{ writeback: MemoryTimeTravelKVAuditProofLinkWritebackStore }>("/v1/memory-time-travel/audit/links/writeback/store", {
        method: "POST",
        body: JSON.stringify(input),
      }),
    auditLinksWritebackExecutorPlan: (input = {}) =>
      fetcher<{ plan: MemoryTimeTravelKVAuditProofLinkWritebackExecutorPlan }>("/v1/memory-time-travel/audit/links/writeback/executor/plan", {
        method: "POST",
        body: JSON.stringify(input),
      }),
    auditVerify: (limit) =>
      fetcher<MemoryTimeTravelAuditVerification>(`/v1/memory-time-travel/audit/verify${query({ limit: limit ? String(limit) : undefined })}`),
    evidence: (id) =>
      fetcher<MemoryTimeTravelEvidenceResponse>(`/v1/memory-time-travel/evidence/${enc(id)}`),
  };
}
