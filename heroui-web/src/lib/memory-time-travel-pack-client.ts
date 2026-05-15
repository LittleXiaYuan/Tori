import { fetcher } from "./api-core";

export interface MemoryTimeTravelPolicy {
  retention_days: number;
  max_versions_per_key: number;
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
  merkle_verification_ready: boolean;
  memory_persister_writeback_ready?: boolean;
  rollback_writeback_ready: boolean;
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

export interface MemoryTimeTravelPackClient {
  status(): Promise<MemoryTimeTravelStatus>;
  snapshots(namespace?: string): Promise<{ snapshots: MemoryTimeTravelSnapshotSummary[]; count: number }>;
  saveSnapshot(input: MemoryTimeTravelSaveSnapshotInput): Promise<{ snapshot: MemoryTimeTravelSnapshot; status: string }>;
  snapshot(id: string): Promise<{ snapshot: MemoryTimeTravelSnapshot }>;
  snapshotAt(input: MemoryTimeTravelSnapshotAtInput): Promise<MemoryTimeTravelSnapshotAtResponse>;
  diff(input: MemoryTimeTravelDiffInput): Promise<{ diff: MemoryTimeTravelDiffReport }>;
  rollbackPlan(input: MemoryTimeTravelRollbackPlanInput): Promise<{ plan: MemoryTimeTravelRollbackPlan }>;
  auditVerify(limit?: number): Promise<MemoryTimeTravelAuditVerification>;
  evidence(id: string): Promise<{ pack_id: string; exported_at: string; format: string; files: string[]; snapshot: MemoryTimeTravelSnapshot; history: MemoryTimeTravelSnapshotSummary[]; audit_verification?: MemoryTimeTravelAuditVerification; audit_verification_error?: string }>;
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
    auditVerify: (limit) =>
      fetcher<MemoryTimeTravelAuditVerification>(`/v1/memory-time-travel/audit/verify${query({ limit: limit ? String(limit) : undefined })}`),
    evidence: (id) =>
      fetcher<{ pack_id: string; exported_at: string; format: string; files: string[]; snapshot: MemoryTimeTravelSnapshot; history: MemoryTimeTravelSnapshotSummary[]; audit_verification?: MemoryTimeTravelAuditVerification; audit_verification_error?: string }>(
        `/v1/memory-time-travel/evidence/${enc(id)}`,
      ),
  };
}
