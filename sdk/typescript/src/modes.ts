/**
 * Lightweight Persona Modes SDK slice.
 *
 * This keeps persona mode listing/switching usable without importing the full
 * generated OpenAPI SDK:
 *
 *   import { createModesClient } from "yunque-client/modes";
 */

export type PersonaMode = "assistant" | "coder" | "researcher" | "operator" | string;

export type PersonaModeInfo = {
  mode?: PersonaMode;
  id?: string;
  name?: string;
  name_en?: string;
  description?: string;
  features?: string[];
  active?: boolean;
  [key: string]: unknown;
};

export type ModesQuery = {
  tenant_id?: string;
  session_id?: string;
};

export type ListModesResponse = {
  modes: PersonaModeInfo[];
  total: number;
};

export type CurrentModeResponse = PersonaModeInfo & {
  mode: PersonaMode;
};

export type SetModeRequest = ModesQuery & {
  mode: PersonaMode;
};

export type SetModeResponse = {
  success?: boolean;
  current_mode?: PersonaMode;
  modes?: PersonaModeInfo[];
  [key: string]: unknown;
};

export type ModesClientOptions = {
  baseUrl: string;
  token?: string;
  apiKey?: string;
  headers?: HeadersInit;
  fetch?: typeof fetch;
};

export class ModesClientError extends Error {
  readonly status: number;
  readonly body: unknown;

  constructor(status: number, body: unknown, message?: string) {
    super(message || `Modes request failed with HTTP ${status}`);
    this.name = "ModesClientError";
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

function setOptionalQuery(url: URL, key: string, value: string | undefined): void {
  if (!value) return;
  url.searchParams.set(key, value);
}

export class ModesClient {
  private readonly baseUrl: string;
  private readonly fetchImpl: typeof fetch;
  private readonly headers: HeadersInit | undefined;
  private readonly token: string | undefined;
  private readonly apiKey: string | undefined;

  constructor(options: ModesClientOptions) {
    if (!options.baseUrl) throw new Error("ModesClient requires baseUrl");
    const fetchImpl = options.fetch ?? globalThis.fetch;
    if (!fetchImpl) throw new Error("ModesClient requires a fetch implementation");
    this.baseUrl = trimBaseUrl(options.baseUrl);
    this.fetchImpl = fetchImpl.bind(globalThis) as typeof fetch;
    this.headers = options.headers;
    this.token = options.token;
    this.apiKey = options.apiKey;
  }

  list(query?: ModesQuery): Promise<ListModesResponse> {
    return this.request<ListModesResponse>("GET", "/v1/persona/modes", undefined, query);
  }

  current(query?: ModesQuery): Promise<CurrentModeResponse> {
    return this.request<CurrentModeResponse>("GET", "/v1/persona/mode/current", undefined, query);
  }

  set(body: SetModeRequest): Promise<SetModeResponse> {
    return this.request<SetModeResponse>("POST", "/v1/persona/mode", body);
  }

  private async request<T>(method: "GET" | "POST", path: string, body?: unknown, query?: ModesQuery): Promise<T> {
    const url = new URL(`${this.baseUrl}${path}`);
    setOptionalQuery(url, "tenant_id", query?.tenant_id);
    setOptionalQuery(url, "session_id", query?.session_id);

    const headers = mergeHeaders(this.headers);
    if (this.token && !headers.has("authorization")) headers.set("Authorization", `Bearer ${this.token}`);
    if (this.apiKey && !headers.has("x-api-key")) headers.set("X-API-Key", this.apiKey);

    const init: RequestInit = { method, headers };
    if (body !== undefined) {
      headers.set("Content-Type", "application/json");
      init.body = JSON.stringify(body);
    }

    const response = await this.fetchImpl(url, init);
    const parsed = await parseResponse(response);
    if (!response.ok) throw new ModesClientError(response.status, parsed, messageFromErrorBody(parsed));
    return parsed as T;
  }
}

export function createModesClient(options: ModesClientOptions): ModesClient {
  return new ModesClient(options);
}
