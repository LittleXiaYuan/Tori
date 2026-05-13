import { BASE, fetcher, getApiKey } from "./api-core";
import type {
  CogniAlert,
  CogniDeclaration,
  CogniEvolutionResponse,
  CogniExperienceResponse,
  CogniHealthMetrics,
  CogniListResponse,
  CogniReloadResponse,
  CogniTrace,
  CogniVerifyResponse,
  CogniWorkflowDef,
  CogniWorkflowResult,
} from "./api-types/cogni";

export interface CogniImportSummary {
  added?: string[];
  updated?: string[];
  skipped?: string[];
  failed?: Array<{ id?: string; error?: string }>;
  [key: string]: unknown;
}

export interface CogniKernelPackClient {
  list(): Promise<CogniListResponse>;
  get(id: string): Promise<{ id: string; declaration: CogniDeclaration; enabled: boolean }>;
  add(declaration: CogniDeclaration): Promise<{ status: string; id: string }>;
  remove(id: string): Promise<{ status: string; id: string }>;
  setEnabled(id: string, enabled: boolean): Promise<{ status: string; id: string }>;
  reload(): Promise<CogniReloadResponse>;
  health(id?: string): Promise<{ health: CogniHealthMetrics[]; count: number }>;
  alerts(): Promise<{ alerts: CogniAlert[]; count: number }>;
  scanAlerts(): Promise<{ alerts: CogniAlert[]; count: number }>;
  verify(id?: string): Promise<CogniVerifyResponse>;
  traces(limit?: number): Promise<{ traces: CogniTrace[]; count: number }>;
  tracesByID(id: string, limit?: number): Promise<{ id: string; traces: CogniTrace[]; count: number }>;
  generate(description: string, autoSave?: boolean): Promise<{ status: string; declaration: CogniDeclaration; saved: boolean }>;
  exportBundle(): Promise<void>;
  importBundle(bundle: Record<string, unknown>): Promise<CogniImportSummary>;
  workflows(id: string): Promise<{ id: string; workflows: CogniWorkflowDef[]; count: number }>;
  runWorkflow(id: string, workflowName: string, input?: Record<string, unknown>): Promise<CogniWorkflowResult>;
  experience(id: string): Promise<CogniExperienceResponse>;
  confirmExperiencePattern(id: string, patternID: string): Promise<{ status: string; id: string; confirmed: boolean }>;
  triggerEvolution(id: string): Promise<{ status: string; id: string }>;
  evolution(id: string): Promise<CogniEvolutionResponse>;
}

export function createCogniKernelPackClient(): CogniKernelPackClient {
  return {
    list: () => fetcher<CogniListResponse>("/v1/cognis"),
    get: (id) => fetcher<{ id: string; declaration: CogniDeclaration; enabled: boolean }>(`/v1/cognis/${enc(id)}`),
    add: (declaration) =>
      fetcher<{ status: string; id: string }>("/v1/cognis", {
        method: "POST",
        body: JSON.stringify(declaration),
      }),
    remove: (id) => fetcher<{ status: string; id: string }>(`/v1/cognis/${enc(id)}`, { method: "DELETE" }),
    setEnabled: (id, enabled) =>
      fetcher<{ status: string; id: string }>(`/v1/cognis/${enc(id)}/${enabled ? "enable" : "disable"}`, { method: "POST" }),
    reload: () => fetcher<CogniReloadResponse>("/v1/cognis/reload", { method: "POST" }),
    health: (id) => fetcher<{ health: CogniHealthMetrics[]; count: number }>(id ? `/v1/cognis/${enc(id)}/health` : "/v1/cognis/health"),
    alerts: () => fetcher<{ alerts: CogniAlert[]; count: number }>("/v1/cognis/alerts"),
    scanAlerts: () => fetcher<{ alerts: CogniAlert[]; count: number }>("/v1/cognis/alerts/scan", { method: "POST" }),
    verify: (id) => fetcher<CogniVerifyResponse>(id ? `/v1/cognis/${enc(id)}/verify` : "/v1/cognis/verify", { method: "POST" }),
    traces: (limit = 50) => fetcher<{ traces: CogniTrace[]; count: number }>(`/v1/cognis/traces?limit=${limit}`),
    tracesByID: (id, limit = 50) => fetcher<{ id: string; traces: CogniTrace[]; count: number }>(`/v1/cognis/${enc(id)}/trace?limit=${limit}`),
    generate: (description, autoSave = false) =>
      fetcher<{ status: string; declaration: CogniDeclaration; saved: boolean }>("/v1/cognis/generate", {
        method: "POST",
        body: JSON.stringify({ description, auto_save: autoSave }),
      }),
    exportBundle,
    importBundle: (bundle) =>
      fetcher<CogniImportSummary>("/v1/cognis/import", {
        method: "POST",
        body: JSON.stringify(bundle),
      }),
    workflows: (id) => fetcher<{ id: string; workflows: CogniWorkflowDef[]; count: number }>(`/v1/cognis/${enc(id)}/workflows`),
    runWorkflow: (id, workflowName, input) =>
      fetcher<CogniWorkflowResult>(`/v1/cognis/${enc(id)}/workflow/${enc(workflowName)}`, {
        method: "POST",
        body: input ? JSON.stringify(input) : undefined,
      }),
    experience: (id) => fetcher<CogniExperienceResponse>(`/v1/cognis/${enc(id)}/experience`),
    confirmExperiencePattern: (id, patternID) =>
      fetcher<{ status: string; id: string; confirmed: boolean }>(`/v1/cognis/${enc(id)}/experience/patterns/${enc(patternID)}/confirm`, { method: "POST" }),
    triggerEvolution: (id) => fetcher<{ status: string; id: string }>(`/v1/cognis/${enc(id)}/evolve`, { method: "POST" }),
    evolution: (id) => fetcher<CogniEvolutionResponse>(`/v1/cognis/${enc(id)}/evolution`),
  };
}

function enc(value: string): string {
  return encodeURIComponent(value);
}

async function exportBundle(): Promise<void> {
  const key = getApiKey();
  const token = typeof window !== "undefined" ? localStorage.getItem("yunque_token") : "";
  const res = await fetch(`${BASE}/v1/cognis/export`, {
    headers: { ...(token ? { Authorization: `Bearer ${token}` } : key ? { "X-API-Key": key } : {}) },
  });
  if (!res.ok) throw new Error(`${res.status}: ${await res.text()}`);
  const blob = await res.blob();
  const url = URL.createObjectURL(blob);
  const a = document.createElement("a");
  a.href = url;
  const cd = res.headers.get("Content-Disposition");
  const match = cd?.match(/filename="(.+)"/);
  a.download = match?.[1] || "cogni-bundle.json";
  a.click();
  URL.revokeObjectURL(url);
}
