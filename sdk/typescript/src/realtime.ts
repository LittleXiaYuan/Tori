/**
 * Lightweight Realtime SDK slice for /v1/ws.
 *
 * It keeps browser/desktop chat WebSocket wiring usable without importing the
 * full generated OpenAPI SDK:
 *
 *   import { createRealtimeClient } from "yunque-client/realtime";
 */

export type RealtimeClientOptions = {
  baseUrl: string;
  token?: string;
  apiKey?: string;
  headers?: HeadersInit;
  WebSocket?: WebSocketConstructor;
};

export type RealtimeConnectOptions = {
  token?: string;
  apiKey?: string;
  query?: Record<string, string | number | boolean | undefined>;
  protocols?: string | string[];
  WebSocket?: WebSocketConstructor;
};

export type RealtimeOutboundMessage =
  | { type: "ping"; [key: string]: unknown }
  | { type: "chat"; content: string; session?: string; [key: string]: unknown };

export type RealtimeInboundMessage = {
  type: string;
  content?: string;
  error?: string;
  session?: string;
  [key: string]: unknown;
};

type WebSocketConstructor = {
  new (url: string | URL, protocols?: string | string[]): WebSocket;
};

function trimBaseUrl(baseUrl: string): string {
  return baseUrl.replace(/\/+$/, "");
}

function toWebSocketUrl(baseUrl: string, path: string): URL {
  const url = new URL(`${trimBaseUrl(baseUrl)}${path}`);
  if (url.protocol === "http:") url.protocol = "ws:";
  else if (url.protocol === "https:") url.protocol = "wss:";
  else if (url.protocol !== "ws:" && url.protocol !== "wss:") throw new Error(`Unsupported realtime baseUrl protocol: ${url.protocol}`);
  return url;
}

function appendQuery(url: URL, query?: Record<string, string | number | boolean | undefined>): void {
  if (!query) return;
  for (const [key, value] of Object.entries(query)) {
    if (value !== undefined && value !== "") url.searchParams.set(key, String(value));
  }
}

function selectToken(defaults: { token?: string; apiKey?: string }, connect?: RealtimeConnectOptions): { name: string; value: string } | undefined {
  const apiKey = connect?.apiKey ?? defaults.apiKey;
  if (apiKey) return { name: "api_key", value: apiKey };
  const token = connect?.token ?? defaults.token;
  if (token) return { name: "access_token", value: token };
  return undefined;
}

export class RealtimeClient {
  private readonly baseUrl: string;
  private readonly token: string | undefined;
  private readonly apiKey: string | undefined;
  private readonly WebSocketCtor: WebSocketConstructor | undefined;

  constructor(options: RealtimeClientOptions) {
    if (!options.baseUrl) throw new Error("RealtimeClient requires baseUrl");
    this.baseUrl = options.baseUrl;
    this.token = options.token;
    this.apiKey = options.apiKey;
    this.WebSocketCtor = options.WebSocket ?? globalThis.WebSocket;
  }

  wsUrl(options: RealtimeConnectOptions = {}): string {
    const url = toWebSocketUrl(this.baseUrl, "/v1/ws");
    appendQuery(url, options.query);
    const token = selectToken({ token: this.token, apiKey: this.apiKey }, options);
    if (token && !url.searchParams.has("key") && !url.searchParams.has("api_key") && !url.searchParams.has("token") && !url.searchParams.has("access_token")) {
      url.searchParams.set(token.name, token.value);
    }
    return url.toString();
  }

  connect(options: RealtimeConnectOptions = {}): WebSocket {
    const Ctor = options.WebSocket ?? this.WebSocketCtor;
    if (!Ctor) throw new Error("RealtimeClient requires a WebSocket implementation");
    return new Ctor(this.wsUrl(options), options.protocols);
  }

  ping(extra: Record<string, unknown> = {}): RealtimeOutboundMessage {
    return { type: "ping", ...extra };
  }

  chat(content: string, options: { session?: string } & Record<string, unknown> = {}): RealtimeOutboundMessage {
    const { session, ...extra } = options;
    return session ? { type: "chat", content, session, ...extra } : { type: "chat", content, ...extra };
  }

  send(socket: { send(data: string): void }, message: RealtimeOutboundMessage): void {
    socket.send(JSON.stringify(message));
  }

  parse(data: string): RealtimeInboundMessage {
    const parsed = JSON.parse(data) as unknown;
    if (typeof parsed !== "object" || parsed === null || Array.isArray(parsed)) throw new Error("Realtime message must be an object");
    return parsed as RealtimeInboundMessage;
  }
}

export function createRealtimeClient(options: RealtimeClientOptions): RealtimeClient {
  return new RealtimeClient(options);
}
