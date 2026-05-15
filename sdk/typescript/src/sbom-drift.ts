/**
 * Lightweight SBOM Drift Pack SDK slice.
 *
 * This keeps dependency snapshots, drift diffing, and evidence export usable
 * without importing the full generated OpenAPI SDK:
 *
 *   import { createSBOMDriftClient } from "yunque-client/sbom-drift";
 */

export type SBOMDriftComponent = {
  ecosystem: string;
  name: string;
  version?: string;
  scope?: string;
  path?: string;
  direct: boolean;
};

export type SBOMDriftSnapshotSummary = {
  id: string;
  source: string;
  created_at: string;
  component_count: number;
  ecosystems: Record<string, number>;
};

export type SBOMDriftSnapshot = SBOMDriftSnapshotSummary & {
  components: SBOMDriftComponent[];
};

export type SBOMDriftStatusResponse = {
  pack_id: string;
  stage: string;
  scanner_ready: boolean;
  vulnerability_ready: boolean;
  snapshot_count: number;
  repo_root?: string;
  store_dir?: string;
  capabilities: string[];
  notes?: string[];
};

export type SBOMDriftSnapshotsResponse = {
  snapshots: SBOMDriftSnapshotSummary[];
  count: number;
};

export type SBOMDriftSnapshotResponse = {
  snapshot: SBOMDriftSnapshot;
};

export type SBOMDriftCreateSnapshotRequest = {
  id?: string;
  source?: string;
};

export type SBOMDriftCreateSnapshotResponse = SBOMDriftSnapshotResponse & {
  status: string;
};

export type SBOMDriftDiffRequest = {
  base_id: string;
  target_id?: string;
  target_current?: boolean;
};

export type SBOMDriftChange = {
  ecosystem: string;
  name: string;
  path?: string;
  old_version?: string;
  new_version?: string;
  risk: "none" | "low" | "medium" | "high" | "critical" | string;
};

export type SBOMDriftDiff = {
  base: SBOMDriftSnapshotSummary;
  target: SBOMDriftSnapshotSummary;
  added: SBOMDriftChange[];
  removed: SBOMDriftChange[];
  changed: SBOMDriftChange[];
  risk_level: string;
  notes?: string[];
};

export type SBOMDriftDiffResponse = {
  diff: SBOMDriftDiff;
};

export type SBOMDriftEvidenceResponse = {
  pack_id: string;
  exported_at: string;
  format: string;
  files: string[];
  snapshot: SBOMDriftSnapshot;
};

export type SBOMDriftClientOptions = {
  baseUrl: string;
  token?: string;
  apiKey?: string;
  headers?: HeadersInit;
  fetch?: typeof fetch;
};

export class SBOMDriftClientError extends Error {
  readonly status: number;
  readonly body: unknown;

  constructor(status: number, body: unknown, message?: string) {
    super(message || `SBOM Drift request failed with HTTP ${status}`);
    this.name = "SBOMDriftClientError";
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

export class SBOMDriftClient {
  private readonly baseUrl: string;
  private readonly fetchImpl: typeof fetch;
  private readonly headers: HeadersInit | undefined;
  private readonly token: string | undefined;
  private readonly apiKey: string | undefined;

  constructor(options: SBOMDriftClientOptions) {
    if (!options.baseUrl) throw new Error("SBOMDriftClient requires baseUrl");
    const fetchImpl = options.fetch ?? globalThis.fetch;
    if (!fetchImpl) throw new Error("SBOMDriftClient requires a fetch implementation");
    this.baseUrl = trimBaseUrl(options.baseUrl);
    this.fetchImpl = fetchImpl.bind(globalThis) as typeof fetch;
    this.headers = options.headers;
    this.token = options.token;
    this.apiKey = options.apiKey;
  }

  status(): Promise<SBOMDriftStatusResponse> {
    return this.request<SBOMDriftStatusResponse>("GET", "/v1/sbom-drift/status");
  }

  snapshots(): Promise<SBOMDriftSnapshotsResponse> {
    return this.request<SBOMDriftSnapshotsResponse>("GET", "/v1/sbom-drift/snapshots");
  }

  createSnapshot(input: SBOMDriftCreateSnapshotRequest = {}): Promise<SBOMDriftCreateSnapshotResponse> {
    return this.request<SBOMDriftCreateSnapshotResponse>("POST", "/v1/sbom-drift/snapshots", input);
  }

  snapshot(id: string): Promise<SBOMDriftSnapshotResponse> {
    return this.request<SBOMDriftSnapshotResponse>("GET", `/v1/sbom-drift/snapshots/${enc(id)}`);
  }

  diff(input: SBOMDriftDiffRequest): Promise<SBOMDriftDiffResponse> {
    return this.request<SBOMDriftDiffResponse>("POST", "/v1/sbom-drift/diff", input);
  }

  evidence(id: string): Promise<SBOMDriftEvidenceResponse> {
    return this.request<SBOMDriftEvidenceResponse>("GET", `/v1/sbom-drift/evidence/${enc(id)}`);
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
    if (!response.ok) throw new SBOMDriftClientError(response.status, parsed, messageFromErrorBody(parsed));
    return parsed as T;
  }
}

export function createSBOMDriftClient(options: SBOMDriftClientOptions): SBOMDriftClient {
  return new SBOMDriftClient(options);
}
