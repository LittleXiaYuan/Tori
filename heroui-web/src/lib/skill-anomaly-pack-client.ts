import { fetcher } from "./api-core";

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
  queue_writeback_ready: boolean;
  requested_by?: string;
  reason?: string;
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
}

export interface SkillAnomalyPackClient {
  status(): Promise<SkillAnomalyStatus>;
  events(input?: { skill_slug?: string; limit?: number }): Promise<{ events: SkillAnomalyEvent[]; count: number }>;
  observe(input: SkillAnomalyObservationInput): Promise<{ event: SkillAnomalyEvent; result: SkillAnomalyResult; status: string }>;
  profiles(): Promise<{ profiles: SkillAnomalyProfileSummary[]; count: number }>;
  profile(skillSlug: string): Promise<{ profile: SkillAnomalyProfile }>;
  detect(input: SkillAnomalyObservationInput): Promise<{ result: SkillAnomalyResult }>;
  auditHookPlan(input: SkillAnomalyAuditHookPlanInput): Promise<{ plan: SkillAnomalyAuditHookPlan }>;
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
    evidence: (skillSlug) =>
      fetcher<SkillAnomalyEvidence>(`/v1/skill-anomaly/evidence/${enc(skillSlug)}`),
  };
}
