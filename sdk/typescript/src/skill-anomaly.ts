/**
 * Lightweight Skill Anomaly Pack SDK slice.
 *
 * This keeps skill behavior profiles, anomaly dry-runs, NeedsApproval plans,
 * and evidence export usable without importing the full generated OpenAPI SDK:
 *
 *   import { createSkillAnomalyClient } from "yunque-client/skill-anomaly";
 */

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
  audit_hook_ready: boolean;
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

export type SkillAnomalyEvidenceResponse = {
  pack_id: string;
  exported_at: string;
  format: string;
  files: string[];
  profile: SkillAnomalyProfile;
  events: SkillAnomalyEvent[];
  policy: SkillAnomalyPolicy;
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
