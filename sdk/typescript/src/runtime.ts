/**
 * Lightweight Runtime SDK slice.
 *
 * This keeps session queue inspection/cancellation and global event streaming
 * usable without importing the full generated OpenAPI SDK:
 *
 *   import { createRuntimeClient } from "yunque-client/runtime";
 */

export type QueueTask = {
  id?: string;
  task_id?: string;
  status?: string;
  title?: string;
  created_at?: string;
  [key: string]: unknown;
};

export type QueueOverviewResponse = {
  queues?: Record<string, number>;
};

export type QueueSessionResponse = {
  session_id: string;
  tasks: QueueTask[];
};

export type QueueCancelResponse = {
  cancelled: boolean;
};

export type RuntimeEvent = {
  event?: string;
  type?: string;
  data?: unknown;
  id?: string;
  retry?: string;
  raw?: string;
};

export type RuntimeClientOptions = {
  baseUrl: string;
  token?: string;
  apiKey?: string;
  headers?: HeadersInit;
  fetch?: typeof fetch;
};

export class RuntimeClientError extends Error {
  readonly status: number;
  readonly body: unknown;

  constructor(status: number, body: unknown, message?: string) {
    super(message || `Runtime request failed with HTTP ${status}`);
    this.name = "RuntimeClientError";
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

function parseSSEEvent(raw: string): RuntimeEvent | undefined {
  const event: RuntimeEvent = { raw };
  const dataLines: string[] = [];
  for (const line of raw.split(/\r?\n/)) {
    if (line.startsWith(":")) continue;
    const index = line.indexOf(":");
    const field = index >= 0 ? line.slice(0, index) : line;
    const value = index >= 0 ? line.slice(index + 1).trimStart() : "";
    if (field === "event") event.event = value;
    if (field === "id") event.id = value;
    if (field === "retry") event.retry = value;
    if (field === "data") dataLines.push(value);
  }
  const dataText = dataLines.join("\n").trim();
  if (dataText) {
    try {
      event.data = JSON.parse(dataText);
    } catch {
      event.data = dataText;
    }
  }
  if (event.event) event.type = event.event;
  if (!event.event && event.data === undefined) return undefined;
  return event;
}

async function* parseSSEStream(stream: ReadableStream<Uint8Array>): AsyncGenerator<RuntimeEvent> {
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
        if (parsed) yield parsed;
      }
    }
    buffer += decoder.decode();
    if (buffer.trim()) {
      const parsed = parseSSEEvent(buffer);
      if (parsed) yield parsed;
    }
  } finally {
    reader.releaseLock();
  }
}

export class RuntimeClient {
  private readonly baseUrl: string;
  private readonly fetchImpl: typeof fetch;
  private readonly headers: HeadersInit | undefined;
  private readonly token: string | undefined;
  private readonly apiKey: string | undefined;

  constructor(options: RuntimeClientOptions) {
    if (!options.baseUrl) throw new Error("RuntimeClient requires baseUrl");
    const fetchImpl = options.fetch ?? globalThis.fetch;
    if (!fetchImpl) throw new Error("RuntimeClient requires a fetch implementation");
    this.baseUrl = trimBaseUrl(options.baseUrl);
    this.fetchImpl = fetchImpl.bind(globalThis) as typeof fetch;
    this.headers = options.headers;
    this.token = options.token;
    this.apiKey = options.apiKey;
  }

  queues(): Promise<QueueOverviewResponse> {
    return this.request<QueueOverviewResponse>("GET", "/v1/sessions/queue");
  }

  sessionQueue(sessionId: string): Promise<QueueSessionResponse> {
    const url = `/v1/sessions/queue?id=${encodeURIComponent(sessionId)}`;
    return this.request<QueueSessionResponse>("GET", url);
  }

  cancelQueuedTask(sessionId: string, taskId: string): Promise<QueueCancelResponse> {
    return this.request<QueueCancelResponse>("POST", "/v1/sessions/queue/cancel", {
      session_id: sessionId,
      task_id: taskId,
    });
  }

  async *events(): AsyncGenerator<RuntimeEvent> {
    const headers = this.authHeaders({ Accept: "text/event-stream" });
    const response = await this.fetchImpl(new URL(`${this.baseUrl}/v1/events/stream`), { method: "GET", headers });
    if (!response.ok) {
      const parsed = await parseResponse(response);
      throw new RuntimeClientError(response.status, parsed, messageFromErrorBody(parsed));
    }
    if (!response.body) return;
    yield* parseSSEStream(response.body);
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
    if (!response.ok) throw new RuntimeClientError(response.status, parsed, messageFromErrorBody(parsed));
    return parsed as T;
  }
}

export function createRuntimeClient(options: RuntimeClientOptions): RuntimeClient {
  return new RuntimeClient(options);
}
