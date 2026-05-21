/**
 * Lightweight Chat SDK slice.
 *
 * This module is hand-written so embedders can import only the chat surface
 * without pulling in the large generated OpenAPI bundle:
 *
 *   import { createChatClient } from "yunque-client/chat";
 */

export type ChatRole = "system" | "user" | "assistant" | "tool" | string;

export type ChatMessage = {
  role: ChatRole;
  content: string;
  name?: string;
  tool_call_id?: string;
};

export type ChatRequest = {
  messages: ChatMessage[];
  session_id?: string;
  task_id?: string;
  class_id?: string;
  teacher_id?: string;
  student_id?: string;
  platform?: string;
  thinking_level?: "none" | "auto" | "deep" | string;
  stream?: boolean;
};

export type ChatPlanStep = {
  id?: number;
  action?: string;
  skill?: string;
  status?: string;
  result?: string;
  error?: string;
  depends_on?: number[];
};

export type ChatResponse = {
  reply: string;
  skills_used?: string[];
  steps?: number;
  actions?: unknown[];
  plan?: ChatPlanStep[];
  context_layers?: string[];
  emotion?: unknown;
  sticker_suggestion?: unknown;
  sticker_suggestions?: Record<string, unknown>;
  sandbox?: Record<string, unknown>;
  rich?: unknown;
};

export type ChatStreamItem =
  | { kind: "delta"; content: string }
  | { kind: "done"; data: Record<string, unknown>; raw: string }
  | { kind: "actions"; actions: unknown[]; raw: string }
  | { kind: "thinking"; content: string; data: Record<string, unknown> | null; raw: string }
  | { kind: "error"; message: string; data: unknown; raw: string }
  | { kind: "raw"; data: string; event: string };

export type ChatClientOptions = {
  baseUrl: string;
  token?: string;
  apiKey?: string;
  headers?: HeadersInit;
  fetch?: typeof fetch;
};

export class ChatClientError extends Error {
  readonly status: number;
  readonly body: unknown;

  constructor(status: number, body: unknown, message?: string) {
    super(message || `Chat request failed with HTTP ${status}`);
    this.name = "ChatClientError";
    this.status = status;
    this.body = body;
  }
}

type SSEFrame = { event: string; data: string };

function trimBaseUrl(baseUrl: string): string {
  return baseUrl.replace(/\/+$/, "");
}

function mergeHeaders(base: HeadersInit | undefined, extra?: HeadersInit): Headers {
  const headers = new Headers(base);
  if (!extra) return headers;
  new Headers(extra).forEach((value, key) => headers.set(key, value));
  return headers;
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

function parseJSONRecord(raw: string): Record<string, unknown> | null {
  try {
    const parsed = JSON.parse(raw);
    return isRecord(parsed) ? parsed : null;
  } catch {
    return null;
  }
}

function frameDataLine(line: string): string {
  let data = line.slice(5);
  if (data.startsWith(" ")) data = data.slice(1);
  return data;
}

async function* readSSEFrames(body: ReadableStream<Uint8Array>): AsyncGenerator<SSEFrame> {
  const reader = body.getReader();
  const decoder = new TextDecoder();
  let buffer = "";
  let event = "";
  let dataLines: string[] = [];

  const reset = () => {
    event = "";
    dataLines = [];
  };

  try {
    while (true) {
      const { done, value } = await reader.read();
      if (done) {
        buffer += decoder.decode();
        break;
      }
      buffer += decoder.decode(value, { stream: true });
      const lines = buffer.split("\n");
      buffer = lines.pop() || "";
      for (const rawLine of lines) {
        const line = rawLine.endsWith("\r") ? rawLine.slice(0, -1) : rawLine;
        if (line === "") {
          if (event || dataLines.length > 0) yield { event, data: dataLines.join("\n") };
          reset();
        } else if (line.startsWith("event:")) {
          event = line.slice(6).trim();
        } else if (line.startsWith("data:")) {
          dataLines.push(frameDataLine(line));
        }
      }
    }

    if (buffer) {
      const line = buffer.endsWith("\r") ? buffer.slice(0, -1) : buffer;
      if (line.startsWith("event:")) event = line.slice(6).trim();
      if (line.startsWith("data:")) dataLines.push(frameDataLine(line));
    }
    if (event || dataLines.length > 0) yield { event, data: dataLines.join("\n") };
  } finally {
    reader.releaseLock();
  }
}

function parseStreamFrame(frame: SSEFrame): ChatStreamItem[] {
  const raw = frame.data;
  const event = frame.event;
  if (raw === "[DONE]") return [];

  if (event === "error") {
    const parsed = parseJSONRecord(raw);
    return [{ kind: "error", message: messageFromErrorBody(parsed) || raw, data: parsed, raw }];
  }

  if (event === "done") {
    const data = parseJSONRecord(raw);
    return data ? [{ kind: "done", data, raw }] : [{ kind: "raw", data: raw, event }];
  }

  if (event === "actions") {
    let actions: unknown[] = [];
    try {
      const parsed = JSON.parse(raw);
      if (Array.isArray(parsed)) actions = parsed;
      else if (isRecord(parsed) && Array.isArray(parsed.actions)) actions = parsed.actions;
    } catch {
      actions = [];
    }
    return [{ kind: "actions", actions, raw }];
  }

  if (event === "thinking") {
    const data = parseJSONRecord(raw);
    const content = typeof data?.content === "string" ? data.content : raw;
    return [{ kind: "thinking", content, data, raw }];
  }

  const data = parseJSONRecord(raw);
  if (typeof data?.content === "string" || data?.type === "delta") {
    return [{ kind: "delta", content: typeof data.content === "string" ? data.content : "" }];
  }
  if (data && (typeof data.error === "string" || data.type === "error")) {
    return [{ kind: "error", message: messageFromErrorBody(data) || raw, data, raw }];
  }
  return raw.trim() ? [{ kind: "raw", data: raw, event }] : [];
}

export class ChatClient {
  private readonly baseUrl: string;
  private readonly fetchImpl: typeof fetch;
  private readonly headers: HeadersInit | undefined;
  private readonly token: string | undefined;
  private readonly apiKey: string | undefined;

  constructor(options: ChatClientOptions) {
    if (!options.baseUrl) throw new Error("ChatClient requires baseUrl");
    const fetchImpl = options.fetch ?? globalThis.fetch;
    if (!fetchImpl) throw new Error("ChatClient requires a fetch implementation");
    this.baseUrl = trimBaseUrl(options.baseUrl);
    this.fetchImpl = fetchImpl.bind(globalThis) as typeof fetch;
    this.headers = options.headers;
    this.token = options.token;
    this.apiKey = options.apiKey;
  }

  send(body: ChatRequest): Promise<ChatResponse> {
    return this.request<ChatResponse>("/v1/chat", body);
  }

  agentic(body: ChatRequest): Promise<ChatResponse> {
    return this.request<ChatResponse>("/v1/chat/agentic", body);
  }

  async *stream(body: ChatRequest): AsyncGenerator<ChatStreamItem> {
    const response = await this.post("/v1/chat/stream", { ...body, stream: true });
    if (!response.body) throw new ChatClientError(response.status, undefined, "Chat stream response has no body");
    if (!response.ok) {
      const parsed = await parseResponse(response);
      throw new ChatClientError(response.status, parsed, messageFromErrorBody(parsed));
    }
    yield* this.parseStream(response.body);
  }

  async *parseStream(body: ReadableStream<Uint8Array>): AsyncGenerator<ChatStreamItem> {
    for await (const frame of readSSEFrames(body)) {
      if (frame.data === "[DONE]") return;
      for (const item of parseStreamFrame(frame)) yield item;
    }
  }

  private async request<T>(path: string, body: unknown): Promise<T> {
    const response = await this.post(path, body);
    const parsed = await parseResponse(response);
    if (!response.ok) throw new ChatClientError(response.status, parsed, messageFromErrorBody(parsed));
    return parsed as T;
  }

  private post(path: string, body: unknown): Promise<Response> {
    const headers = mergeHeaders(this.headers);
    headers.set("Content-Type", "application/json");
    if (this.token && !headers.has("authorization")) headers.set("Authorization", `Bearer ${this.token}`);
    if (this.apiKey && !headers.has("x-api-key")) headers.set("X-API-Key", this.apiKey);
    return this.fetchImpl(`${this.baseUrl}${path}`, {
      method: "POST",
      headers,
      body: JSON.stringify(body),
    });
  }
}

export function createChatClient(options: ChatClientOptions): ChatClient {
  return new ChatClient(options);
}
