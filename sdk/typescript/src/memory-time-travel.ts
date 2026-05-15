/**
 * Lightweight Memory Time Travel Pack SDK slice.
 *
 * This keeps memory snapshot storage, point-in-time reconstruction, drift diff,
 * rollback plan generation, and evidence export usable without importing the
 * full generated OpenAPI SDK:
 *
 *   import { createMemoryTimeTravelClient } from "yunque-client/memory-time-travel";
 */

export type MemoryTimeTravelPolicy = {
  retention_days: number;
  max_versions_per_key: number;
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
  merkle_verification_ready: boolean;
  rollback_writeback_ready: boolean;
  snapshot_count: number;
  namespace_count: number;
  store_dir?: string;
  policy: MemoryTimeTravelPolicy;
  last_snapshot?: MemoryTimeTravelSnapshotSummary | null;
  capabilities: string[];
  notes?: string[];
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

export type MemoryTimeTravelEvidenceResponse = {
  pack_id: string;
  exported_at: string;
  format: string;
  files: string[];
  snapshot: MemoryTimeTravelSnapshot;
  history: MemoryTimeTravelSnapshotSummary[];
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

  evidence(id: string): Promise<MemoryTimeTravelEvidenceResponse> {
    return this.request<MemoryTimeTravelEvidenceResponse>("GET", `/v1/memory-time-travel/evidence/${enc(id)}`);
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
