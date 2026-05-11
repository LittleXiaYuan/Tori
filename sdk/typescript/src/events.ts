/**
 * Lightweight Events SDK slice.
 *
 * Streams /v1/events/stream Server-Sent Events without importing the full
 * generated OpenAPI SDK:
 *
 *   import { createEventsClient } from "yunque-client/events";
 */

export type EventStreamClientOptions = {
  baseUrl: string;
  token?: string;
  apiKey?: string;
  headers?: HeadersInit;
  fetch?: typeof fetch;
};

export type EventStreamMessage<T = unknown> = {
  event: string;
  data?: T;
  id?: string;
  retry?: number;
  raw: string;
};

export type EventStreamOptions = {
  signal?: AbortSignal;
  headers?: HeadersInit;
};

export class EventsClientError extends Error {
  readonly status: number;
  readonly body: unknown;

  constructor(status: number, body: unknown, message?: string) {
    super(message || `Events request failed with HTTP ${status}`);
    this.name = "EventsClientError";
    this.status = status;
    this.body = body;
  }
}

type SSEFrame = {
  event?: string;
  data: string;
  id?: string;
  retry?: number;
  raw: string;
};

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

function parseFrame(rawFrame: string): SSEFrame | undefined {
  const lines = rawFrame.split(/\r?\n/);
  const data: string[] = [];
  let event: string | undefined;
  let id: string | undefined;
  let retry: number | undefined;

  for (const line of lines) {
    if (!line || line.startsWith(":")) continue;
    const colon = line.indexOf(":");
    const field = colon === -1 ? line : line.slice(0, colon);
    const value = colon === -1 ? "" : line.slice(colon + 1).replace(/^ /, "");
    if (field === "event") event = value;
    else if (field === "data") data.push(value);
    else if (field === "id") id = value;
    else if (field === "retry") {
      const parsed = Number(value);
      if (Number.isFinite(parsed)) retry = parsed;
    }
  }

  if (!event && data.length === 0 && !id && retry === undefined) return undefined;
  return { event, data: data.join("\n"), id, retry, raw: rawFrame };
}

async function* readSSEFrames(body: ReadableStream<Uint8Array>): AsyncGenerator<SSEFrame> {
  const reader = body.getReader();
  const decoder = new TextDecoder();
  let buffer = "";
  try {
    while (true) {
      const { value, done } = await reader.read();
      if (done) break;
      buffer += decoder.decode(value, { stream: true });
      buffer = buffer.replace(/\r\n/g, "\n");
      let splitAt = buffer.indexOf("\n\n");
      while (splitAt !== -1) {
        const rawFrame = buffer.slice(0, splitAt);
        buffer = buffer.slice(splitAt + 2);
        const frame = parseFrame(rawFrame);
        if (frame) yield frame;
        splitAt = buffer.indexOf("\n\n");
      }
    }
    buffer += decoder.decode();
    const frame = parseFrame(buffer.trimEnd());
    if (frame) yield frame;
  } finally {
    reader.releaseLock();
  }
}

function parseEventData(data: string): unknown {
  if (!data) return undefined;
  try {
    return JSON.parse(data);
  } catch {
    return data;
  }
}

export class EventsClient {
  private readonly baseUrl: string;
  private readonly fetchImpl: typeof fetch;
  private readonly headers: HeadersInit | undefined;
  private readonly token: string | undefined;
  private readonly apiKey: string | undefined;

  constructor(options: EventStreamClientOptions) {
    if (!options.baseUrl) throw new Error("EventsClient requires baseUrl");
    const fetchImpl = options.fetch ?? globalThis.fetch;
    if (!fetchImpl) throw new Error("EventsClient requires a fetch implementation");
    this.baseUrl = trimBaseUrl(options.baseUrl);
    this.fetchImpl = fetchImpl.bind(globalThis) as typeof fetch;
    this.headers = options.headers;
    this.token = options.token;
    this.apiKey = options.apiKey;
  }

  async *stream<T = unknown>(options: EventStreamOptions = {}): AsyncGenerator<EventStreamMessage<T>> {
    const headers = mergeHeaders(this.headers, options.headers);
    if (this.token && !headers.has("authorization")) headers.set("Authorization", `Bearer ${this.token}`);
    if (this.apiKey && !headers.has("x-api-key")) headers.set("X-API-Key", this.apiKey);

    const response = await this.fetchImpl(new URL(`${this.baseUrl}/v1/events/stream`), {
      method: "GET",
      headers,
      signal: options.signal,
    });

    if (!response.ok) {
      const parsed = await parseResponse(response);
      throw new EventsClientError(response.status, parsed, messageFromErrorBody(parsed));
    }
    if (!response.body) throw new EventsClientError(response.status, undefined, "Events stream response has no body");

    for await (const frame of readSSEFrames(response.body)) {
      yield {
        event: frame.event || "message",
        data: parseEventData(frame.data) as T,
        id: frame.id,
        retry: frame.retry,
        raw: frame.raw,
      };
    }
  }

  parseStream<T = unknown>(body: ReadableStream<Uint8Array>): AsyncGenerator<EventStreamMessage<T>> {
    return this.parseFrames(body);
  }

  private async *parseFrames<T = unknown>(body: ReadableStream<Uint8Array>): AsyncGenerator<EventStreamMessage<T>> {
    for await (const frame of readSSEFrames(body)) {
      yield {
        event: frame.event || "message",
        data: parseEventData(frame.data) as T,
        id: frame.id,
        retry: frame.retry,
        raw: frame.raw,
      };
    }
  }
}

export function createEventsClient(options: EventStreamClientOptions): EventsClient {
  return new EventsClient(options);
}
