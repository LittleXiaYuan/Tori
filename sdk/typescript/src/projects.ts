/** Lightweight Projects SDK slice: project workspace CRUD without importing the generated bundle. */
export type ProjectMeta = Record<string, string>;
export type Project = {
  id: string;
  name: string;
  repo_path: string;
  repo_url?: string;
  description?: string;
  default_caps?: string[];
  meta?: ProjectMeta;
  created_at: string;
  updated_at: string;
};
export type ProjectsListResponse = { projects: Project[]; [key: string]: unknown };
export type CreateProjectRequest = { name: string; repo_path: string; repo_url?: string; description?: string; default_caps?: string[]; meta?: ProjectMeta };
export type UpdateProjectRequest = Partial<CreateProjectRequest>;
export type DeleteProjectResponse = { status: "deleted" | string; [key: string]: unknown };
export type ProjectsClientOptions = { baseUrl: string; token?: string; apiKey?: string; headers?: HeadersInit; fetch?: typeof fetch };

export class ProjectsClientError extends Error { readonly status: number; readonly body: unknown; constructor(status: number, body: unknown, message?: string) { super(message || `Projects request failed with HTTP ${status}`); this.name = "ProjectsClientError"; this.status = status; this.body = body; } }
function trimBaseUrl(baseUrl: string): string { return baseUrl.replace(/\/+$/, ""); }
function mergeHeaders(base: HeadersInit | undefined, extra?: HeadersInit): Headers { const headers = new Headers(base); if (!extra) return headers; new Headers(extra).forEach((value, key) => headers.set(key, value)); return headers; }
function isRecord(value: unknown): value is Record<string, unknown> { return typeof value === "object" && value !== null && !Array.isArray(value); }
function messageFromErrorBody(body: unknown): string | undefined { if (typeof body === "string" && body.trim()) return body.trim(); if (!isRecord(body)) return undefined; for (const key of ["message", "detail", "error", "reason"]) { const value = body[key]; if (typeof value === "string" && value.trim()) return value; if (key === "error" && isRecord(value)) { const nested = messageFromErrorBody(value); if (nested) return nested; } } return undefined; }
async function parseResponse(response: Response): Promise<unknown> { const text = await response.text(); if (!text) return undefined; try { return JSON.parse(text); } catch { return text; } }
function appendQuery(url: URL, query?: Record<string, string | undefined>): void { if (!query) return; for (const [key, value] of Object.entries(query)) if (value !== undefined) url.searchParams.set(key, value); }

export class ProjectsClient {
  private readonly baseUrl: string; private readonly fetchImpl: typeof fetch; private readonly headers: HeadersInit | undefined; private readonly token: string | undefined; private readonly apiKey: string | undefined;
  constructor(options: ProjectsClientOptions) { if (!options.baseUrl) throw new Error("ProjectsClient requires baseUrl"); const fetchImpl = options.fetch ?? globalThis.fetch; if (!fetchImpl) throw new Error("ProjectsClient requires a fetch implementation"); this.baseUrl = trimBaseUrl(options.baseUrl); this.fetchImpl = fetchImpl.bind(globalThis) as typeof fetch; this.headers = options.headers; this.token = options.token; this.apiKey = options.apiKey; }

  list(): Promise<ProjectsListResponse> { return this.request<ProjectsListResponse>("GET", "/v1/projects"); }
  create(request: CreateProjectRequest): Promise<Project> { return this.request<Project>("POST", "/v1/projects", { body: request }); }
  detail(id: string): Promise<Project> { return this.request<Project>("GET", "/v1/projects/detail", { query: { id } }); }
  update(id: string, patch: UpdateProjectRequest): Promise<Project> { return this.request<Project>("PUT", "/v1/projects/detail", { query: { id }, body: patch }); }
  remove(id: string): Promise<DeleteProjectResponse> { return this.request<DeleteProjectResponse>("POST", "/v1/projects/remove", { body: { id } }); }

  private async request<T>(method: "GET" | "POST" | "PUT", path: string, options: { body?: unknown; query?: Record<string, string | undefined>; headers?: HeadersInit } = {}): Promise<T> {
    const url = new URL(`${this.baseUrl}${path}`); appendQuery(url, options.query); const headers = mergeHeaders(this.headers, options.headers); if (this.token && !headers.has("authorization")) headers.set("Authorization", `Bearer ${this.token}`); if (this.apiKey && !headers.has("x-api-key")) headers.set("X-API-Key", this.apiKey);
    const init: RequestInit = { method, headers }; if (options.body !== undefined) { headers.set("Content-Type", "application/json"); init.body = JSON.stringify(options.body); }
    const response = await this.fetchImpl(url, init); const parsed = await parseResponse(response); if (!response.ok) throw new ProjectsClientError(response.status, parsed, messageFromErrorBody(parsed)); return parsed as T;
  }
}

export function createProjectsClient(options: ProjectsClientOptions): ProjectsClient { return new ProjectsClient(options); }
