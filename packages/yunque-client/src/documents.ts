/**
 * Lightweight Documents SDK slice.
 *
 * This keeps direct DOCX/XLSX/PPTX/HTML generation usable without importing
 * the full generated OpenAPI SDK:
 *
 *   import { createDocumentsClient } from "yunque-client/documents";
 */

export type DocumentFormat = "docx" | "xlsx" | "pptx" | "html" | string;

export type DocumentGenerateRequest = {
  format: DocumentFormat;
  content: string;
  path?: string;
  title?: string;
  sheet_name?: string;
};

export type DocumentGenerateResponse = {
  result?: string;
  path: string;
  format: DocumentFormat;
};

export type DocumentTemplate = {
  id?: string;
  name?: string;
  format?: DocumentFormat;
  description?: string;
  [key: string]: unknown;
};

export type DocumentTemplatesResponse = {
  templates: DocumentTemplate[];
  [key: string]: unknown;
};

export type DocumentsClientOptions = {
  baseUrl: string;
  token?: string;
  apiKey?: string;
  headers?: HeadersInit;
  fetch?: typeof fetch;
};

export class DocumentsClientError extends Error {
  readonly status: number;
  readonly body: unknown;

  constructor(status: number, body: unknown, message?: string) {
    super(message || `Documents request failed with HTTP ${status}`);
    this.name = "DocumentsClientError";
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

export class DocumentsClient {
  private readonly baseUrl: string;
  private readonly fetchImpl: typeof fetch;
  private readonly headers: HeadersInit | undefined;
  private readonly token: string | undefined;
  private readonly apiKey: string | undefined;

  constructor(options: DocumentsClientOptions) {
    if (!options.baseUrl) throw new Error("DocumentsClient requires baseUrl");
    const fetchImpl = options.fetch ?? globalThis.fetch;
    if (!fetchImpl) throw new Error("DocumentsClient requires a fetch implementation");
    this.baseUrl = trimBaseUrl(options.baseUrl);
    this.fetchImpl = fetchImpl.bind(globalThis) as typeof fetch;
    this.headers = options.headers;
    this.token = options.token;
    this.apiKey = options.apiKey;
  }

  templates(): Promise<DocumentTemplatesResponse> {
    return this.request<DocumentTemplatesResponse>("GET", "/v1/documents/templates");
  }

  generate(body: DocumentGenerateRequest): Promise<DocumentGenerateResponse> {
    return this.request<DocumentGenerateResponse>("POST", "/v1/documents/generate", body);
  }

  generateDocx(body: Omit<DocumentGenerateRequest, "format">): Promise<DocumentGenerateResponse> {
    return this.generate({ ...body, format: "docx" });
  }

  generateXlsx(body: Omit<DocumentGenerateRequest, "format">): Promise<DocumentGenerateResponse> {
    return this.generate({ ...body, format: "xlsx" });
  }

  generatePptx(body: Omit<DocumentGenerateRequest, "format">): Promise<DocumentGenerateResponse> {
    return this.generate({ ...body, format: "pptx" });
  }

  generateHtml(body: Omit<DocumentGenerateRequest, "format">): Promise<DocumentGenerateResponse> {
    return this.generate({ ...body, format: "html" });
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
    if (!response.ok) throw new DocumentsClientError(response.status, parsed, messageFromErrorBody(parsed));
    return parsed as T;
  }
}

export function createDocumentsClient(options: DocumentsClientOptions): DocumentsClient {
  return new DocumentsClient(options);
}
