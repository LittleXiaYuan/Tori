import { fetcher } from "./api-core";

export const SKILL_ANOMALY_APPROVAL_QUEUE_STORE_ARTIFACT =
  "approval-queue-store.json";

export const SKILL_ANOMALY_APPROVAL_QUEUE_RECORD_ARTIFACT =
  "approval-queue-record.json";

export const SKILL_ANOMALY_APPROVAL_QUEUE_WRITEBACK_CAPABILITY =
  "skill.approval_queue.writeback";

export const SKILL_ANOMALY_APPROVAL_MANAGER_BRIDGE_PLAN_ARTIFACT =
  "approval-manager-bridge-plan.json";

export const SKILL_ANOMALY_APPROVAL_MANAGER_BRIDGE_PLAN_CAPABILITY =
  "skill.approval_manager.bridge.plan";

export interface SkillAnomalyPolicy {
  window_size: number;
  min_observations: number;
  new_action_score: number;
  new_param_score: number;
  failure_burst_score: number;
  duration_spike_score: number;
  needs_approval_score: number;
  block_score: number;
  duration_spike_factor: number;
}

export interface SkillAnomalyEvent {
  id: string;
  skill_slug: string;
  actor?: string;
  action: string;
  param_keys?: string[];
  success: boolean;
  duration_ms?: number;
  timestamp: string;
}

export interface SkillAnomalyProfileSummary {
  skill_slug: string;
  observed: number;
  calls_per_minute: number;
  action_distrib: Record<string, number>;
  param_key_set: Record<string, number>;
  success_rate: number;
  avg_duration_ms: number;
  last_anomaly_at?: string;
  anomaly_count: number;
  updated_at: string;
}

export interface SkillAnomalyProfile extends SkillAnomalyProfileSummary {
  window_size: number;
  recent: SkillAnomalyEvent[];
}

export interface SkillAnomalyStatus {
  pack_id: string;
  stage: string;
  detector_ready: boolean;
  audit_hook_plan_ready: boolean;
  audit_hook_ready: boolean;
  trust_mutation_plan_ready: boolean;
  trust_mutation_ready: boolean;
  approval_writeback_ready: boolean;
  approval_queue_store_ready: boolean;
  approval_manager_bridge_plan_ready: boolean;
  global_approval_enqueue_ready: boolean;
  approval_queue_store?: SkillAnomalyApprovalQueueStoreSummary;
  profile_count: number;
  active_profiles: number;
  anomaly_count: number;
  store_dir?: string;
  policy: SkillAnomalyPolicy;
  capabilities: string[];
  notes?: string[];
}

export interface SkillAnomalyReason {
  name: string;
  score: number;
  severity: string;
  detail?: string;
}

export interface SkillAnomalyResult {
  skill_slug: string;
  score: number;
  severity: string;
  needs_approval: boolean;
  block: boolean;
  reasons?: SkillAnomalyReason[];
  profile: SkillAnomalyProfileSummary;
  event: SkillAnomalyEvent;
  notes?: string[];
}

export interface SkillAnomalyObservationInput {
  skill_slug: string;
  actor?: string;
  action: string;
  params?: Record<string, unknown>;
  param_keys?: string[];
  success?: boolean;
  duration_ms?: number;
  timestamp?: string;
  dry_run?: boolean;
}

export interface SkillAnomalyAuditHookPlanInput extends SkillAnomalyObservationInput {
  requested_by?: string;
  reason?: string;
  request_id?: string;
  request_key?: string;
}

export interface SkillAnomalyAuditRecordPlan {
  event_type: string;
  action: string;
  subject: string;
  severity: string;
  merkle_append_ready: boolean;
  payload: Record<string, unknown>;
}

export interface SkillAnomalyTrustMutationPlan {
  target_skill: string;
  mutation: string;
  delta: number;
  record_failure_ready: boolean;
  reason: string;
}

export interface SkillAnomalyApprovalQueuePlan {
  required: boolean;
  queue_name: string;
  queue_writeback_ready: boolean;
  writes_approval_queue: boolean;
  writes_queue_store: boolean;
  request_id: string;
  request_key: string;
  status: string;
  requested_by?: string;
  reason?: string;
  store_artifact: string;
}

export interface SkillAnomalyAuditHookPlan {
  pack_id: string;
  skill_slug: string;
  generated_at: string;
  dry_run: boolean;
  status: string;
  approval_required: boolean;
  audit_hook_plan_ready: boolean;
  audit_hook_ready: boolean;
  trust_mutation_plan_ready: boolean;
  trust_mutation_ready: boolean;
  approval_writeback_ready: boolean;
  detection: SkillAnomalyResult;
  audit_record: SkillAnomalyAuditRecordPlan;
  trust_mutation: SkillAnomalyTrustMutationPlan;
  approval_queue: SkillAnomalyApprovalQueuePlan;
  actions: string[];
  notes?: string[];
}

export interface SkillAnomalyApprovalQueueStoreSummary {
  pack_id: string;
  queue_name: string;
  store: string;
  store_ready: boolean;
  record_count: number;
  artifact: string;
  writes_approval_queue: boolean;
  writes_approval_queue_file: boolean;
  merkle_append_ready: boolean;
  trust_mutation_ready: boolean;
  notes?: string[];
}

export interface SkillAnomalyApprovalQueueRecord {
  pack_id: string;
  queue_name: string;
  request_id: string;
  request_key: string;
  skill_slug: string;
  status: string;
  severity: string;
  score: number;
  approval_required: boolean;
  requested_by?: string;
  reason?: string;
  created_at: string;
  updated_at: string;
  audit_hook_plan_ready: boolean;
  audit_hook_ready: boolean;
  merkle_append_ready: boolean;
  trust_mutation_plan_ready: boolean;
  trust_mutation_ready: boolean;
  approval_writeback_ready: boolean;
  writes_approval_queue: boolean;
  writes_approval_queue_file: boolean;
  action_allowed: boolean;
  execution_blocked: boolean;
  detection: SkillAnomalyResult;
  audit_record: SkillAnomalyAuditRecordPlan;
  trust_mutation: SkillAnomalyTrustMutationPlan;
  approval_queue: SkillAnomalyApprovalQueuePlan;
  store_artifact: string;
  artifacts: string[];
  labels: string[];
  notes?: string[];
}

export interface SkillAnomalyApprovalQueueWriteback {
  pack_id: string;
  generated_at: string;
  status: string;
  approval_required: boolean;
  approval_writeback_ready: boolean;
  writes_approval_queue: boolean;
  writes_approval_queue_file: boolean;
  audit_hook_plan_ready: boolean;
  audit_hook_ready: boolean;
  merkle_append_ready: boolean;
  trust_mutation_plan_ready: boolean;
  trust_mutation_ready: boolean;
  action_allowed: boolean;
  execution_blocked: boolean;
  request_id: string;
  request_key: string;
  approval_queue_record: SkillAnomalyApprovalQueueRecord;
  approval_queue_store: SkillAnomalyApprovalQueueStoreSummary;
  plan_summary: SkillAnomalyAuditHookPlan;
  artifacts: string[];
  actions: string[];
  notes?: string[];
}

export type SkillAnomalyApprovalQueueWritebackInput =
  SkillAnomalyAuditHookPlanInput;

export interface SkillAnomalyGlobalApprovalRequestPlan {
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

export interface SkillAnomalyApprovalManagerBridgePlan {
  pack_id: string;
  generated_at: string;
  status: string;
  approval_manager_bridge_plan_ready: boolean;
  global_approval_enqueue_ready: boolean;
  merkle_append_ready: boolean;
  trust_mutation_ready: boolean;
  action_release_ready: boolean;
  approval_queue_store_ready: boolean;
  source_queue_record_persisted: boolean;
  request_id: string;
  request_key: string;
  source_approval_queue_record: SkillAnomalyApprovalQueueRecord;
  proposed_global_approval_request: SkillAnomalyGlobalApprovalRequestPlan;
  detection: SkillAnomalyResult;
  audit_record: SkillAnomalyAuditRecordPlan;
  trust_mutation: SkillAnomalyTrustMutationPlan;
  plan_summary: SkillAnomalyAuditHookPlan;
  artifacts: string[];
  actions: string[];
  labels: string[];
  notes?: string[];
}

export type SkillAnomalyApprovalManagerBridgePlanInput =
  SkillAnomalyAuditHookPlanInput;

export interface SkillAnomalyEvidence {
  pack_id: string;
  exported_at: string;
  format: string;
  files: string[];
  profile: SkillAnomalyProfile;
  events: SkillAnomalyEvent[];
  policy: SkillAnomalyPolicy;
  audit_hook_plan?: SkillAnomalyAuditHookPlan;
  trust_mutation_plan?: SkillAnomalyTrustMutationPlan;
  approval_queue_plan?: SkillAnomalyApprovalQueuePlan;
  approval_queue_store?: SkillAnomalyApprovalQueueStoreSummary;
  approval_queue_record?: SkillAnomalyApprovalQueueRecord;
  approval_manager_bridge_plan?: SkillAnomalyApprovalManagerBridgePlan;
}

export interface SkillAnomalyPackClient {
  status(): Promise<SkillAnomalyStatus>;
  events(input?: { skill_slug?: string; limit?: number }): Promise<{ events: SkillAnomalyEvent[]; count: number }>;
  observe(input: SkillAnomalyObservationInput): Promise<{ event: SkillAnomalyEvent; result: SkillAnomalyResult; status: string }>;
  profiles(): Promise<{ profiles: SkillAnomalyProfileSummary[]; count: number }>;
  profile(skillSlug: string): Promise<{ profile: SkillAnomalyProfile }>;
  detect(input: SkillAnomalyObservationInput): Promise<{ result: SkillAnomalyResult }>;
  auditHookPlan(input: SkillAnomalyAuditHookPlanInput): Promise<{ plan: SkillAnomalyAuditHookPlan }>;
  approvalQueueWriteback(input: SkillAnomalyApprovalQueueWritebackInput): Promise<{ writeback: SkillAnomalyApprovalQueueWriteback }>;
  approvalManagerBridgePlan(input: SkillAnomalyApprovalManagerBridgePlanInput): Promise<{ plan: SkillAnomalyApprovalManagerBridgePlan }>;
  evidence(skillSlug: string): Promise<SkillAnomalyEvidence>;
}

function enc(value: string): string {
  return encodeURIComponent(value);
}

function query(input?: { skill_slug?: string; limit?: number }): string {
  const params = new URLSearchParams();
  if (input?.skill_slug) params.set("skill_slug", input.skill_slug);
  if (input?.limit) params.set("limit", String(input.limit));
  const value = params.toString();
  return value ? `?${value}` : "";
}

export function createSkillAnomalyPackClient(): SkillAnomalyPackClient {
  return {
    status: () => fetcher<SkillAnomalyStatus>("/v1/skill-anomaly/status"),
    events: (input) => fetcher<{ events: SkillAnomalyEvent[]; count: number }>(`/v1/skill-anomaly/events${query(input)}`),
    observe: (input) =>
      fetcher<{ event: SkillAnomalyEvent; result: SkillAnomalyResult; status: string }>("/v1/skill-anomaly/events", {
        method: "POST",
        body: JSON.stringify(input),
      }),
    profiles: () => fetcher<{ profiles: SkillAnomalyProfileSummary[]; count: number }>("/v1/skill-anomaly/profiles"),
    profile: (skillSlug) => fetcher<{ profile: SkillAnomalyProfile }>(`/v1/skill-anomaly/profiles/${enc(skillSlug)}`),
    detect: (input) =>
      fetcher<{ result: SkillAnomalyResult }>("/v1/skill-anomaly/detect", {
        method: "POST",
        body: JSON.stringify(input),
      }),
    auditHookPlan: (input) =>
      fetcher<{ plan: SkillAnomalyAuditHookPlan }>("/v1/skill-anomaly/audit-hook/plan", {
        method: "POST",
        body: JSON.stringify(input),
      }),
    approvalQueueWriteback: (input) =>
      fetcher<{ writeback: SkillAnomalyApprovalQueueWriteback }>("/v1/skill-anomaly/approval-queue/writeback", {
        method: "POST",
        body: JSON.stringify(input),
      }),
    approvalManagerBridgePlan: (input) =>
      fetcher<{ plan: SkillAnomalyApprovalManagerBridgePlan }>("/v1/skill-anomaly/approval-queue/bridge/plan", {
        method: "POST",
        body: JSON.stringify(input),
      }),
    evidence: (skillSlug) =>
      fetcher<SkillAnomalyEvidence>(`/v1/skill-anomaly/evidence/${enc(skillSlug)}`),
  };
}
