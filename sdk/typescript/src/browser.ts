/**
 * Lightweight Browser SDK slice.
 *
 * This keeps browser-extension automation, screenshots, OCR/content extraction,
 * and OPP decisions usable without importing the full generated OpenAPI SDK:
 *
 *   import { createBrowserClient } from "yunque-client/browser";
 */

export type BrowserStatusResponse = {
  enabled?: boolean;
  connected?: boolean;
  extension_connected?: boolean;
  state?: string;
  version?: string;
  pending?: number;
  error?: string;
  [key: string]: unknown;
};

export type BrowserConfigResponse = {
  mode?: string;
  connected?: boolean;
  headless?: boolean;
  [key: string]: unknown;
};

export type BrowserNavigateResponse = {
  screenshot?: string;
  title?: string;
  url?: string;
  [key: string]: unknown;
};

export type BrowserScreenshotResponse = {
  screenshot?: string;
  timestamp?: string;
};

export type BrowserOCRResponse = {
  text?: string;
  result?: string;
};

export type BrowserOPPItem = {
  id?: string;
  problem_id?: string;
  title?: string;
  description?: string;
  [key: string]: unknown;
};

export type BrowserOPPPendingResponse = {
  items: BrowserOPPItem[];
  total: number;
};

export type BrowserOPPDecisionResponse = {
  status?: string;
  problem_id?: string;
  [key: string]: unknown;
};

export type BrowserExtensionSessionResponse = {
  ok?: boolean;
  ws_url?: string;
  ticket?: string;
  nonce?: string;
  expires_at?: string;
  ttl_sec?: number;
  error?: string;
};

export type BrowserAction = {
  type: string;
  url?: string;
  selector?: string;
  text?: string;
  value?: string;
  [key: string]: unknown;
};

export type BrowserActionResult = {
  ok?: boolean;
  error?: string;
  screenshot?: string;
  title?: string;
  url?: string;
  content?: string;
  [key: string]: unknown;
};

export type BrowserScenario = {
  id?: string;
  name?: string;
  description?: string;
  steps?: Array<Record<string, unknown>>;
  [key: string]: unknown;
};

export type BrowserScenariosResponse = {
  scenarios: BrowserScenario[];
};

export type BrowserRunScenarioResponse = {
  ok?: boolean;
  scenario?: string;
  results?: Array<Record<string, unknown>>;
  error?: string;
};

export type BrowserClientOptions = {
  baseUrl: string;
  token?: string;
  apiKey?: string;
  headers?: HeadersInit;
  fetch?: typeof fetch;
};

export class BrowserClientError extends Error {
  readonly status: number;
  readonly body: unknown;

  constructor(status: number, body: unknown, message?: string) {
    super(message || `Browser request failed with HTTP ${status}`);
    this.name = "BrowserClientError";
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

export class BrowserClient {
  private readonly baseUrl: string;
  private readonly fetchImpl: typeof fetch;
  private readonly headers: HeadersInit | undefined;
  private readonly token: string | undefined;
  private readonly apiKey: string | undefined;

  constructor(options: BrowserClientOptions) {
    if (!options.baseUrl) throw new Error("BrowserClient requires baseUrl");
    const fetchImpl = options.fetch ?? globalThis.fetch;
    if (!fetchImpl) throw new Error("BrowserClient requires a fetch implementation");
    this.baseUrl = trimBaseUrl(options.baseUrl);
    this.fetchImpl = fetchImpl.bind(globalThis) as typeof fetch;
    this.headers = options.headers;
    this.token = options.token;
    this.apiKey = options.apiKey;
  }

  status(): Promise<BrowserStatusResponse> {
    return this.request<BrowserStatusResponse>("GET", "/v1/browser/status");
  }

  config(): Promise<BrowserConfigResponse> {
    return this.request<BrowserConfigResponse>("GET", "/v1/browser/config");
  }

  navigate(url: string): Promise<BrowserNavigateResponse> {
    return this.request<BrowserNavigateResponse>("POST", "/v1/browser/navigate", { url });
  }

  screenshot(): Promise<BrowserScreenshotResponse> {
    return this.request<BrowserScreenshotResponse>("GET", "/v1/browser/screenshot");
  }

  latestScreenshot(): Promise<BrowserScreenshotResponse> {
    return this.request<BrowserScreenshotResponse>("GET", "/v1/browser/screenshot/latest");
  }

  ocr(): Promise<BrowserOCRResponse> {
    return this.request<BrowserOCRResponse>("POST", "/v1/browser/ocr", {});
  }

  oppPending(): Promise<BrowserOPPPendingResponse> {
    return this.request<BrowserOPPPendingResponse>("GET", "/v1/browser/opp/pending");
  }

  oppDecide(input: { problem_id?: string; id?: string; decision: string }): Promise<BrowserOPPDecisionResponse> {
    return this.request<BrowserOPPDecisionResponse>("POST", "/v1/browser/opp/decide", input);
  }

  extensionStatus(): Promise<BrowserStatusResponse> {
    return this.request<BrowserStatusResponse>("GET", "/api/browser/ext/status");
  }

  extensionSession(): Promise<BrowserExtensionSessionResponse> {
    return this.request<BrowserExtensionSessionResponse>("POST", "/api/browser/ext/session", {});
  }

  extensionAction(action: BrowserAction): Promise<BrowserActionResult> {
    return this.request<BrowserActionResult>("POST", "/api/browser/ext/action", action);
  }

  scenarios(): Promise<BrowserScenariosResponse> {
    return this.request<BrowserScenariosResponse>("GET", "/api/browser/ext/scenarios");
  }

  runScenario(scenarioId: string): Promise<BrowserRunScenarioResponse> {
    return this.request<BrowserRunScenarioResponse>("POST", "/api/browser/ext/scenarios/run", { scenario_id: scenarioId });
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
    if (!response.ok) throw new BrowserClientError(response.status, parsed, messageFromErrorBody(parsed));
    return parsed as T;
  }
}

export function createBrowserClient(options: BrowserClientOptions): BrowserClient {
  return new BrowserClient(options);
}
