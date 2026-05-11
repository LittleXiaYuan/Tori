/** Lightweight WebChat SDK slice: embeddable widget script and snippet helpers. */
export type WebChatPosition = "bottom-right" | "bottom-left" | string;
export type WebChatTheme = "light" | "dark" | string;
export type WebChatWidgetOptions = { baseUrl: string; headers?: HeadersInit; fetch?: typeof fetch };
export type WebChatEmbedOptions = { apiKey: string; apiBase?: string; title?: string; placeholder?: string; position?: WebChatPosition; theme?: WebChatTheme; tenantId?: string; scriptPath?: string };

export class WebChatClientError extends Error { readonly status: number; readonly body: unknown; constructor(status: number, body: unknown, message?: string) { super(message || `WebChat request failed with HTTP ${status}`); this.name = "WebChatClientError"; this.status = status; this.body = body; } }
function trimBaseUrl(baseUrl: string): string { return baseUrl.replace(/\/+$/, ""); }
function mergeHeaders(base: HeadersInit | undefined, extra?: HeadersInit): Headers { const headers = new Headers(base); if (!extra) return headers; new Headers(extra).forEach((value, key) => headers.set(key, value)); return headers; }
function escapeAttr(value: string): string { return value.replace(/&/g, "&amp;").replace(/"/g, "&quot;").replace(/</g, "&lt;").replace(/>/g, "&gt;"); }
async function parseResponse(response: Response): Promise<unknown> { const text = await response.text(); if (!text) return undefined; try { return JSON.parse(text); } catch { return text; } }
function isRecord(value: unknown): value is Record<string, unknown> { return typeof value === "object" && value !== null && !Array.isArray(value); }
function messageFromBody(body: unknown): string | undefined { if (typeof body === "string" && body.trim()) return body.trim(); if (!isRecord(body)) return undefined; for (const key of ["message", "detail", "error", "reason"]) { const value = body[key]; if (typeof value === "string" && value.trim()) return value; if (key === "error" && isRecord(value)) { const nested = messageFromBody(value); if (nested) return nested; } } return undefined; }

export class WebChatClient {
  private readonly baseUrl: string; private readonly headers: HeadersInit | undefined; private readonly fetchImpl: typeof fetch;
  constructor(options: WebChatWidgetOptions) { if (!options.baseUrl) throw new Error("WebChatClient requires baseUrl"); const fetchImpl = options.fetch ?? globalThis.fetch; if (!fetchImpl) throw new Error("WebChatClient requires a fetch implementation"); this.baseUrl = trimBaseUrl(options.baseUrl); this.headers = options.headers; this.fetchImpl = fetchImpl.bind(globalThis) as typeof fetch; }
  widgetUrl(): string { return `${this.baseUrl}/v1/webchat/widget.js`; }
  embedSnippet(options: WebChatEmbedOptions): string { return buildWebChatEmbedSnippet({ ...options, scriptPath: options.scriptPath ?? this.widgetUrl(), apiBase: options.apiBase ?? this.baseUrl }); }
  async widgetScript(origin?: string): Promise<string> { const headers = mergeHeaders(this.headers, origin ? { Origin: origin } : undefined); const response = await this.fetchImpl(new URL(this.widgetUrl()), { method: "GET", headers }); const parsed = await parseResponse(response); if (!response.ok) throw new WebChatClientError(response.status, parsed, messageFromBody(parsed)); return String(parsed ?? ""); }
}

export function buildWebChatEmbedSnippet(options: WebChatEmbedOptions): string {
  if (!options.apiKey) throw new Error("buildWebChatEmbedSnippet requires apiKey");
  const attrs: Record<string, string | undefined> = { src: options.scriptPath ?? "/v1/webchat/widget.js", "data-api-key": options.apiKey, "data-api-base": options.apiBase, "data-title": options.title, "data-placeholder": options.placeholder, "data-position": options.position, "data-theme": options.theme, "data-tenant-id": options.tenantId };
  const rendered = Object.entries(attrs).filter(([, value]) => value !== undefined && value !== "").map(([key, value]) => `${key}="${escapeAttr(String(value))}"`).join(" ");
  return `<script ${rendered}></script>`;
}
export function createWebChatClient(options: WebChatWidgetOptions): WebChatClient { return new WebChatClient(options); }
