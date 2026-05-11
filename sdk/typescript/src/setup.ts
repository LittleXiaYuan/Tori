/**
 * Lightweight Setup SDK slice.
 *
 * This keeps first-run configuration and provider bootstrap usable without
 * importing the full generated OpenAPI SDK:
 *
 *   import { createSetupClient } from "yunque-client/setup";
 */

export type SetupProviderProbe = {
  id?: string;
  name?: string;
  base_url?: string;
  model?: string;
  available?: boolean;
  error?: string;
  [key: string]: unknown;
};

export type SetupDetectResponse = {
  providers?: SetupProviderProbe[];
  has_docker?: boolean;
  has_gpu?: boolean;
  has_ollama?: boolean;
  [key: string]: unknown;
};

export type SetupHealthResponse = {
  providers?: SetupProviderProbe[];
  has_docker?: boolean;
  has_gpu?: boolean;
  has_ollama?: boolean;
};

export type SetupTemplate = {
  id: string;
  name?: string;
  description?: string;
  sandbox_tier?: string;
  env_vars?: Record<string, string>;
  [key: string]: unknown;
};

export type SetupTemplatesResponse = {
  templates: SetupTemplate[];
  count: number;
};

export type SetupTestProviderRequest = {
  base_url: string;
  api_key?: string;
  model?: string;
};

export type SetupTestProviderResponse = {
  ok: boolean;
  provider?: SetupProviderProbe;
};

export type SetupApplyRequest = {
  template_id: string;
  api_key?: string;
  base_url?: string;
  model?: string;
  overrides?: Record<string, unknown>;
};

export type SetupApplyResponse = {
  ok?: boolean;
  status?: string;
  applied?: string;
  persisted?: boolean;
  restart_required?: boolean;
  template?: SetupTemplate;
  env_content?: string;
  message?: string;
  [key: string]: unknown;
};

export type SetupInstallComponentResponse = {
  success?: boolean;
  message?: string;
  error?: string;
  [key: string]: unknown;
};

export type SetupInstallProgress = {
  stage?: string;
  detail?: string;
  progress?: number;
  [key: string]: unknown;
};

export type SetupClientOptions = {
  baseUrl: string;
  token?: string;
  apiKey?: string;
  headers?: HeadersInit;
  fetch?: typeof fetch;
};

export class SetupClientError extends Error {
  readonly status: number;
  readonly body: unknown;

  constructor(status: number, body: unknown, message?: string) {
    super(message || `Setup request failed with HTTP ${status}`);
    this.name = "SetupClientError";
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

function parseSSEEvent(raw: string): unknown {
  const dataLines: string[] = [];
  for (const line of raw.split(/\r?\n/)) {
    if (line.startsWith("data:")) dataLines.push(line.slice(5).trimStart());
  }
  const data = dataLines.join("\n").trim();
  if (!data) return undefined;
  try {
    return JSON.parse(data);
  } catch {
    return data;
  }
}

async function* parseSSEStream(stream: ReadableStream<Uint8Array>): AsyncGenerator<unknown> {
  const reader = stream.getReader();
  const decoder = new TextDecoder();
  let buffer = "";

  try {
    for (;;) {
      const { value, done } = await reader.read();
      if (done) break;
      buffer += decoder.decode(value, { stream: true });
      const parts = buffer.split(/\r?\n\r?\n/);
      buffer = parts.pop() ?? "";
      for (const part of parts) {
        const parsed = parseSSEEvent(part);
        if (parsed !== undefined) yield parsed;
      }
    }

    buffer += decoder.decode();
    if (buffer.trim()) {
      const parsed = parseSSEEvent(buffer);
      if (parsed !== undefined) yield parsed;
    }
  } finally {
    reader.releaseLock();
  }
}

export class SetupClient {
  private readonly baseUrl: string;
  private readonly fetchImpl: typeof fetch;
  private readonly headers: HeadersInit | undefined;
  private readonly token: string | undefined;
  private readonly apiKey: string | undefined;

  constructor(options: SetupClientOptions) {
    if (!options.baseUrl) throw new Error("SetupClient requires baseUrl");
    const fetchImpl = options.fetch ?? globalThis.fetch;
    if (!fetchImpl) throw new Error("SetupClient requires a fetch implementation");
    this.baseUrl = trimBaseUrl(options.baseUrl);
    this.fetchImpl = fetchImpl.bind(globalThis) as typeof fetch;
    this.headers = options.headers;
    this.token = options.token;
    this.apiKey = options.apiKey;
  }

  detect(): Promise<SetupDetectResponse> {
    return this.request<SetupDetectResponse>("GET", "/v1/setup/detect");
  }

  health(): Promise<SetupHealthResponse> {
    return this.request<SetupHealthResponse>("GET", "/v1/setup/health");
  }

  templates(): Promise<SetupTemplatesResponse> {
    return this.request<SetupTemplatesResponse>("GET", "/v1/setup/templates");
  }

  testProvider(body: SetupTestProviderRequest): Promise<SetupTestProviderResponse> {
    return this.request<SetupTestProviderResponse>("POST", "/v1/setup/test-provider", body);
  }

  apply(body: SetupApplyRequest): Promise<SetupApplyResponse> {
    return this.request<SetupApplyResponse>("POST", "/v1/setup/apply", body);
  }

  installComponent(componentId: string): Promise<SetupInstallComponentResponse> {
    return this.request<SetupInstallComponentResponse>("POST", "/v1/setup/install-component", { component_id: componentId });
  }

  async *installComponentStream(componentId: string): AsyncGenerator<SetupInstallProgress | string> {
    const headers = this.authHeaders({ Accept: "text/event-stream" });
    headers.set("Content-Type", "application/json");
    const response = await this.fetchImpl(new URL(`${this.baseUrl}/v1/setup/install-component`), {
      method: "POST",
      headers,
      body: JSON.stringify({ component_id: componentId }),
    });
    if (!response.ok) {
      const parsed = await parseResponse(response);
      throw new SetupClientError(response.status, parsed, messageFromErrorBody(parsed));
    }
    if (!response.body) return;
    for await (const event of parseSSEStream(response.body)) {
      yield event as SetupInstallProgress | string;
    }
  }

  private authHeaders(extra?: HeadersInit): Headers {
    const headers = mergeHeaders(this.headers, extra);
    if (this.token && !headers.has("authorization")) headers.set("Authorization", `Bearer ${this.token}`);
    if (this.apiKey && !headers.has("x-api-key")) headers.set("X-API-Key", this.apiKey);
    return headers;
  }

  private async request<T>(method: "GET" | "POST", path: string, body?: unknown): Promise<T> {
    const headers = this.authHeaders();
    const init: RequestInit = { method, headers };
    if (body !== undefined) {
      headers.set("Content-Type", "application/json");
      init.body = JSON.stringify(body);
    }

    const response = await this.fetchImpl(new URL(`${this.baseUrl}${path}`), init);
    const parsed = await parseResponse(response);
    if (!response.ok) throw new SetupClientError(response.status, parsed, messageFromErrorBody(parsed));
    return parsed as T;
  }
}

export function createSetupClient(options: SetupClientOptions): SetupClient {
  return new SetupClient(options);
}
