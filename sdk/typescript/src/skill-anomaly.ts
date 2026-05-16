/**
 * Lightweight Skill Anomaly Pack SDK slice.
 *
 * This keeps skill behavior profiles, anomaly dry-runs, NeedsApproval plans,
 * audit-hook / Trust mutation plans, pack-local Approval queue writeback
 * persistence, Approval Manager bridge plans, and evidence export usable without
 * importing the full generated OpenAPI SDK:
 *
 *   import { createSkillAnomalyClient } from "yunque-client/skill-anomaly";
 */

export const SKILL_ANOMALY_APPROVAL_QUEUE_STORE_ARTIFACT =
  "approval-queue-store.json";

export const SKILL_ANOMALY_APPROVAL_QUEUE_WRITEBACK_CAPABILITY =
  "skill.approval_queue.writeback";

export const SKILL_ANOMALY_APPROVAL_MANAGER_BRIDGE_PLAN_ARTIFACT =
  "approval-manager-bridge-plan.json";

export const SKILL_ANOMALY_APPROVAL_MANAGER_BRIDGE_PLAN_CAPABILITY =
  "skill.approval_manager.bridge.plan";

export type SkillAnomalyPolicy = {
  window_size: number;
  min_observations: number;
  new_action_score: number;
  new_param_score: number;
  failure_burst_score: number;
  duration_spike_score: number;
  needs_approval_score: number;
  block_score: number;
  duration_spike_factor: number;
};

export type SkillAnomalyEvent = {
  id: string;
  skill_slug: string;
  actor?: string;
  action: string;
  param_keys?: string[];
  success: boolean;
  duration_ms?: number;
  timestamp: string;
};

export type SkillAnomalyProfileSummary = {
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
};

export type SkillAnomalyProfile = SkillAnomalyProfileSummary & {
  window_size: number;
  recent: SkillAnomalyEvent[];
};

export type SkillAnomalyStatusResponse = {
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
};

export type SkillAnomalyReason = {
  name: string;
  score: number;
  severity: string;
  detail?: string;
};

export type SkillAnomalyResult = {
  skill_slug: string;
  score: number;
  severity: string;
  needs_approval: boolean;
  block: boolean;
  reasons?: SkillAnomalyReason[];
  profile: SkillAnomalyProfileSummary;
  event: SkillAnomalyEvent;
  notes?: string[];
};

export type SkillAnomalyObservationRequest = {
  skill_slug: string;
  actor?: string;
  action: string;
  params?: Record<string, unknown>;
  param_keys?: string[];
  success?: boolean;
  duration_ms?: number;
  timestamp?: string;
  dry_run?: boolean;
};

export type SkillAnomalyAuditHookPlanRequest = SkillAnomalyObservationRequest & {
  requested_by?: string;
  reason?: string;
  request_id?: string;
  request_key?: string;
};

export type SkillAnomalyEventsResponse = {
  events: SkillAnomalyEvent[];
  count: number;
};

export type SkillAnomalyObserveResponse = {
  event: SkillAnomalyEvent;
  result: SkillAnomalyResult;
  status: string;
};

export type SkillAnomalyProfilesResponse = {
  profiles: SkillAnomalyProfileSummary[];
  count: number;
};

export type SkillAnomalyProfileResponse = {
  profile: SkillAnomalyProfile;
};

export type SkillAnomalyDetectResponse = {
  result: SkillAnomalyResult;
};

export type SkillAnomalyAuditRecordPlan = {
  event_type: string;
  action: string;
  subject: string;
  severity: string;
  merkle_append_ready: boolean;
  payload: Record<string, unknown>;
};

export type SkillAnomalyTrustMutationPlan = {
  target_skill: string;
  mutation: string;
  delta: number;
  record_failure_ready: boolean;
  reason: string;
};

export type SkillAnomalyApprovalQueuePlan = {
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
};

export type SkillAnomalyAuditHookPlan = {
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
};

export type SkillAnomalyAuditHookPlanResponse = {
  plan: SkillAnomalyAuditHookPlan;
};

export type SkillAnomalyApprovalQueueStoreSummary = {
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
};

export type SkillAnomalyApprovalQueueRecord = {
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
};

export type SkillAnomalyApprovalQueueWriteback = {
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
};

export type SkillAnomalyApprovalQueueWritebackRequest =
  SkillAnomalyAuditHookPlanRequest;

export type SkillAnomalyApprovalQueueWritebackResponse = {
  writeback: SkillAnomalyApprovalQueueWriteback;
};

export type SkillAnomalyGlobalApprovalRequestPlan = {
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

export type SkillAnomalyApprovalManagerBridgePlan = {
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
};

export type SkillAnomalyApprovalManagerBridgePlanRequest =
  SkillAnomalyAuditHookPlanRequest;

export type SkillAnomalyApprovalManagerBridgePlanResponse = {
  plan: SkillAnomalyApprovalManagerBridgePlan;
};

export type SkillAnomalyEvidenceResponse = {
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
};

export type SkillAnomalyClientOptions = {
  baseUrl: string;
  token?: string;
  apiKey?: string;
  headers?: HeadersInit;
  fetch?: typeof fetch;
};

export class SkillAnomalyClientError extends Error {
  readonly status: number;
  readonly body: unknown;

  constructor(status: number, body: unknown, message?: string) {
    super(message || `Skill Anomaly request failed with HTTP ${status}`);
    this.name = "SkillAnomalyClientError";
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

function query(input?: { skill_slug?: string; limit?: number }): string {
  const params = new URLSearchParams();
  if (input?.skill_slug) params.set("skill_slug", input.skill_slug);
  if (input?.limit) params.set("limit", String(input.limit));
  const value = params.toString();
  return value ? `?${value}` : "";
}

export class SkillAnomalyClient {
  private readonly baseUrl: string;
  private readonly fetchImpl: typeof fetch;
  private readonly headers: HeadersInit | undefined;
  private readonly token: string | undefined;
  private readonly apiKey: string | undefined;

  constructor(options: SkillAnomalyClientOptions) {
    if (!options.baseUrl) throw new Error("SkillAnomalyClient requires baseUrl");
    const fetchImpl = options.fetch ?? globalThis.fetch;
    if (!fetchImpl) throw new Error("SkillAnomalyClient requires a fetch implementation");
    this.baseUrl = trimBaseUrl(options.baseUrl);
    this.fetchImpl = fetchImpl.bind(globalThis) as typeof fetch;
    this.headers = options.headers;
    this.token = options.token;
    this.apiKey = options.apiKey;
  }

  status(): Promise<SkillAnomalyStatusResponse> {
    return this.request<SkillAnomalyStatusResponse>("GET", "/v1/skill-anomaly/status");
  }

  events(input?: { skill_slug?: string; limit?: number }): Promise<SkillAnomalyEventsResponse> {
    return this.request<SkillAnomalyEventsResponse>("GET", `/v1/skill-anomaly/events${query(input)}`);
  }

  observe(input: SkillAnomalyObservationRequest): Promise<SkillAnomalyObserveResponse> {
    return this.request<SkillAnomalyObserveResponse>("POST", "/v1/skill-anomaly/events", input);
  }

  profiles(): Promise<SkillAnomalyProfilesResponse> {
    return this.request<SkillAnomalyProfilesResponse>("GET", "/v1/skill-anomaly/profiles");
  }

  profile(skillSlug: string): Promise<SkillAnomalyProfileResponse> {
    return this.request<SkillAnomalyProfileResponse>("GET", `/v1/skill-anomaly/profiles/${enc(skillSlug)}`);
  }

  detect(input: SkillAnomalyObservationRequest): Promise<SkillAnomalyDetectResponse> {
    return this.request<SkillAnomalyDetectResponse>("POST", "/v1/skill-anomaly/detect", input);
  }

  auditHookPlan(input: SkillAnomalyAuditHookPlanRequest): Promise<SkillAnomalyAuditHookPlanResponse> {
    return this.request<SkillAnomalyAuditHookPlanResponse>("POST", "/v1/skill-anomaly/audit-hook/plan", input);
  }

  approvalQueueWriteback(input: SkillAnomalyApprovalQueueWritebackRequest): Promise<SkillAnomalyApprovalQueueWritebackResponse> {
    return this.request<SkillAnomalyApprovalQueueWritebackResponse>("POST", "/v1/skill-anomaly/approval-queue/writeback", input);
  }

  approvalManagerBridgePlan(input: SkillAnomalyApprovalManagerBridgePlanRequest): Promise<SkillAnomalyApprovalManagerBridgePlanResponse> {
    return this.request<SkillAnomalyApprovalManagerBridgePlanResponse>("POST", "/v1/skill-anomaly/approval-queue/bridge/plan", input);
  }

  evidence(skillSlug: string): Promise<SkillAnomalyEvidenceResponse> {
    return this.request<SkillAnomalyEvidenceResponse>("GET", `/v1/skill-anomaly/evidence/${enc(skillSlug)}`);
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
    if (!response.ok) throw new SkillAnomalyClientError(response.status, parsed, messageFromErrorBody(parsed));
    return parsed as T;
  }
}

export function createSkillAnomalyClient(options: SkillAnomalyClientOptions): SkillAnomalyClient {
  return new SkillAnomalyClient(options);
}
