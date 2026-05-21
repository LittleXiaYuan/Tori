/**
 * Lightweight Knowledge SDK slice.
 *
 * This keeps RAG/knowledge-base integrations usable without importing the full
 * generated OpenAPI SDK:
 *
 *   import { createKnowledgeClient } from "yunque-client/knowledge";
 */

export type KnowledgeSourceType = "file" | "url" | "repo" | "text" | string;

export type KnowledgeChunk = {
  id?: string;
  source_id?: string;
  source?: string;
  file?: string;
  path?: string;
  lang?: string;
  content?: string;
  text?: string;
  score?: number;
  metadata?: Record<string, unknown>;
  [key: string]: unknown;
};

export type KnowledgeSource = {
  id: string;
  name?: string;
  type?: KnowledgeSourceType;
  path?: string;
  trigger?: string;
  chunks?: number;
  size?: number;
  created_at?: string;
  updated_at?: string;
  metadata?: Record<string, unknown>;
  [key: string]: unknown;
};

export type KnowledgeStats = {
  sources?: number;
  chunks?: number;
  files?: number;
  urls?: number;
  repos?: number;
  bytes?: number;
  [key: string]: unknown;
};

export type KnowledgeSearchRequest = {
  query: string;
  limit?: number;
  file?: string;
  lang?: string;
};

export type KnowledgeSearchResponse = {
  chunks: KnowledgeChunk[];
  count: number;
};

export type KnowledgeSourcesResponse = {
  sources: KnowledgeSource[];
};

export type KnowledgeIngestRequest = {
  name?: string;
  trigger?: string;
  content: string;
};

export type KnowledgeUpdateSourceRequest = {
  id: string;
  name?: string;
  trigger?: string;
  content?: string;
};

export type KnowledgeMutationResponse = {
  source?: KnowledgeSource;
  stats?: KnowledgeStats;
  [key: string]: unknown;
};

export type KnowledgeDeleteResponse = {
  deleted?: string;
  stats?: KnowledgeStats;
  [key: string]: unknown;
};

export type KnowledgeImportUrlRequest = {
  url: string;
  name?: string;
  crawl_children?: boolean;
  max_pages?: number;
};

export type KnowledgeImportUrlResponse = KnowledgeMutationResponse & {
  sources?: KnowledgeSource[];
  imported?: number;
  tree?: Record<string, unknown>;
};

export type KnowledgeImportRepoRequest = {
  path: string;
  max_files?: number;
};

export type KnowledgeUploadRequest = {
  file: Blob;
  filename?: string;
};

export type KnowledgeClientOptions = {
  baseUrl: string;
  token?: string;
  apiKey?: string;
  headers?: HeadersInit;
  fetch?: typeof fetch;
};

export class KnowledgeClientError extends Error {
  readonly status: number;
  readonly body: unknown;

  constructor(status: number, body: unknown, message?: string) {
    super(message || `Knowledge request failed with HTTP ${status}`);
    this.name = "KnowledgeClientError";
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

function setOptionalQuery(url: URL, key: string, value: string | number | undefined): void {
  if (value === undefined || value === "") return;
  url.searchParams.set(key, String(value));
}

export class KnowledgeClient {
  private readonly baseUrl: string;
  private readonly fetchImpl: typeof fetch;
  private readonly headers: HeadersInit | undefined;
  private readonly token: string | undefined;
  private readonly apiKey: string | undefined;

  constructor(options: KnowledgeClientOptions) {
    if (!options.baseUrl) throw new Error("KnowledgeClient requires baseUrl");
    const fetchImpl = options.fetch ?? globalThis.fetch;
    if (!fetchImpl) throw new Error("KnowledgeClient requires a fetch implementation");
    this.baseUrl = trimBaseUrl(options.baseUrl);
    this.fetchImpl = fetchImpl.bind(globalThis) as typeof fetch;
    this.headers = options.headers;
    this.token = options.token;
    this.apiKey = options.apiKey;
  }

  stats(): Promise<KnowledgeStats> {
    return this.request<KnowledgeStats>("GET", "/v1/knowledge/stats");
  }

  sources(): Promise<KnowledgeSourcesResponse> {
    return this.request<KnowledgeSourcesResponse>("GET", "/v1/knowledge/sources");
  }

  search(request: KnowledgeSearchRequest): Promise<KnowledgeSearchResponse> {
    const query: Record<string, string | undefined> = {
      q: request.query,
      n: request.limit === undefined ? undefined : String(request.limit),
      file: request.file,
      lang: request.lang,
    };
    return this.request<KnowledgeSearchResponse>("GET", "/v1/knowledge/search", undefined, query);
  }

  ingest(body: KnowledgeIngestRequest): Promise<KnowledgeMutationResponse> {
    return this.request<KnowledgeMutationResponse>("POST", "/v1/knowledge/ingest", body);
  }

  updateSource(body: KnowledgeUpdateSourceRequest): Promise<KnowledgeMutationResponse> {
    return this.request<KnowledgeMutationResponse>("POST", "/v1/knowledge/source/update", body);
  }

  deleteSource(id: string): Promise<KnowledgeDeleteResponse> {
    return this.request<KnowledgeDeleteResponse>("DELETE", "/v1/knowledge/source", undefined, { id });
  }

  importUrl(body: KnowledgeImportUrlRequest): Promise<KnowledgeImportUrlResponse> {
    return this.request<KnowledgeImportUrlResponse>("POST", "/v1/knowledge/import-url", body);
  }

  importRepo(body: KnowledgeImportRepoRequest): Promise<KnowledgeMutationResponse> {
    return this.request<KnowledgeMutationResponse>("POST", "/v1/knowledge/import-repo", body);
  }

  upload(body: KnowledgeUploadRequest): Promise<KnowledgeMutationResponse> {
    const form = new FormData();
    if (body.filename) {
      form.append("file", body.file, body.filename);
    } else {
      form.append("file", body.file);
    }
    return this.request<KnowledgeMutationResponse>("POST", "/v1/knowledge/upload", form);
  }

  private async request<T>(
    method: "DELETE" | "GET" | "POST",
    path: string,
    body?: unknown,
    query?: Record<string, string | undefined>,
  ): Promise<T> {
    const url = new URL(`${this.baseUrl}${path}`);
    if (query) {
      for (const [key, value] of Object.entries(query)) {
        setOptionalQuery(url, key, value);
      }
    }

    const headers = mergeHeaders(this.headers);
    if (this.token && !headers.has("authorization")) headers.set("Authorization", `Bearer ${this.token}`);
    if (this.apiKey && !headers.has("x-api-key")) headers.set("X-API-Key", this.apiKey);

    const init: RequestInit = { method, headers };
    if (body !== undefined) {
      if (body instanceof FormData) {
        init.body = body;
      } else {
        headers.set("Content-Type", "application/json");
        init.body = JSON.stringify(body);
      }
    }

    const response = await this.fetchImpl(url, init);
    const parsed = await parseResponse(response);
    if (!response.ok) throw new KnowledgeClientError(response.status, parsed, messageFromErrorBody(parsed));
    return parsed as T;
  }
}

export function createKnowledgeClient(options: KnowledgeClientOptions): KnowledgeClient {
  return new KnowledgeClient(options);
}
