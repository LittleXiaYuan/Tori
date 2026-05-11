/** Lightweight Cron SDK slice. */
export type CronScheduleType = "at" | "every" | "cron" | (string & {});
export type CronPayloadKind = "systemEvent" | "agentTurn" | (string & {});
export type CronRunStatus = "success" | "failed" | "skipped" | (string & {});
export type CronDeliveryMode = "announce" | "webhook" | "none" | (string & {});

export type CronSchedule = {
  type: CronScheduleType;
  at?: string;
  every_ms?: number;
  cron_expr?: string;
  timezone?: string;
};

export type CronPayload = {
  kind: CronPayloadKind;
  message?: string;
  data?: Record<string, unknown>;
};

export type CronJob = {
  id: string;
  name: string;
  schedule: CronSchedule;
  payload: CronPayload;
  agent_id?: string;
  session_target?: string;
  delivery?: CronDeliveryMode;
  enabled: boolean;
  created_at: string;
  last_run_at?: string;
  next_run_at?: string;
  run_count: number;
  [key: string]: unknown;
};

export type CronRunRecord = {
  job_id: string;
  run_id: string;
  started_at: string;
  ended_at: string;
  status: CronRunStatus;
  output?: string;
  error?: string;
  [key: string]: unknown;
};

export type CronListResponse = { jobs: CronJob[]; [key: string]: unknown };
export type CronAddRequest = { name: string; schedule: CronSchedule; payload: CronPayload };
export type CronAddResponse = { job: CronJob; [key: string]: unknown };
export type CronRemoveResponse = { deleted: string; [key: string]: unknown };
export type CronRunResponse = { run: CronRunRecord; [key: string]: unknown };
export type CronClientOptions = { baseUrl: string; token?: string; apiKey?: string; headers?: HeadersInit; fetch?: typeof fetch };

export class CronClientError extends Error {
  readonly status: number;
  readonly body: unknown;
  constructor(status: number, body: unknown, message?: string) {
    super(message || `Cron request failed with HTTP ${status}`);
    this.name = "CronClientError";
    this.status = status;
    this.body = body;
  }
}

function trimBaseUrl(baseUrl: string): string { return baseUrl.replace(/\/+$/, ""); }
function mergeHeaders(base: HeadersInit | undefined, extra?: HeadersInit): Headers {
  const headers = new Headers(base);
  if (!extra) return headers;
  new Headers(extra).forEach((value, key) => headers.set(key, value));
  return headers;
}
function isRecord(value: unknown): value is Record<string, unknown> { return typeof value === "object" && value !== null && !Array.isArray(value); }
function messageFromErrorBody(body: unknown): string | undefined {
  if (typeof body === "string" && body.trim()) return body.trim();
  if (!isRecord(body)) return undefined;
  for (const key of ["message", "detail", "error", "reason"]) {
    const value = body[key];
    if (typeof value === "string" && value.trim()) return value;
  }
  return undefined;
}
async function parseResponse(response: Response): Promise<unknown> {
  const text = await response.text();
  if (!text) return undefined;
  try { return JSON.parse(text); } catch { return text; }
}

export class CronClient {
  private readonly baseUrl: string;
  private readonly fetchImpl: typeof fetch;
  private readonly headers: HeadersInit | undefined;
  private readonly token: string | undefined;
  private readonly apiKey: string | undefined;

  constructor(options: CronClientOptions) {
    if (!options.baseUrl) throw new Error("CronClient requires baseUrl");
    const fetchImpl = options.fetch ?? globalThis.fetch;
    if (!fetchImpl) throw new Error("CronClient requires a fetch implementation");
    this.baseUrl = trimBaseUrl(options.baseUrl);
    this.fetchImpl = fetchImpl.bind(globalThis) as typeof fetch;
    this.headers = options.headers;
    this.token = options.token;
    this.apiKey = options.apiKey;
  }

  list(): Promise<CronListResponse> { return this.json<CronListResponse>("/v1/cron/list"); }
  add(request: CronAddRequest): Promise<CronAddResponse> { return this.json<CronAddResponse>("/v1/cron/add", { method: "POST", body: JSON.stringify(request) }); }
  remove(id: string): Promise<CronRemoveResponse> { const url = new URL(`${this.baseUrl}/v1/cron/remove`); url.searchParams.set("id", id); return this.json<CronRemoveResponse>(url, { method: "POST" }); }
  run(id: string): Promise<CronRunResponse> { const url = new URL(`${this.baseUrl}/v1/cron/run`); url.searchParams.set("id", id); return this.json<CronRunResponse>(url, { method: "POST" }); }

  private authHeaders(extra?: HeadersInit): Headers {
    const headers = mergeHeaders(this.headers, extra);
    if (this.token && !headers.has("authorization")) headers.set("Authorization", `Bearer ${this.token}`);
    if (this.apiKey && !headers.has("x-api-key")) headers.set("X-API-Key", this.apiKey);
    return headers;
  }

  private async json<T>(pathOrUrl: string | URL, init: RequestInit = {}): Promise<T> {
    const url = typeof pathOrUrl === "string" ? `${this.baseUrl}${pathOrUrl}` : pathOrUrl;
    const headers = this.authHeaders(init.headers);
    if (init.body !== undefined && !headers.has("content-type")) headers.set("Content-Type", "application/json");
    const response = await this.fetchImpl(url, { ...init, method: init.method ?? "GET", headers });
    const parsed = await parseResponse(response);
    if (!response.ok) throw new CronClientError(response.status, parsed, messageFromErrorBody(parsed));
    return parsed as T;
  }
}

export function createCronClient(options: CronClientOptions): CronClient { return new CronClient(options); }
