/**
 * Thin fetcher-based client for the /system page's six data tabs.
 * Follows the same pattern as trace-pack-client.ts — wraps /v1 endpoints
 * via the shared `fetcher` so auth, base-URL resolution, and error handling
 * are handled centrally rather than per-tab.
 */
import { fetcher } from "./api-core";

// --- Metrics (system stats + request metrics) ---
export interface SystemStats {
  requests_total?: number;
  requests_success?: number;
  requests_error?: number;
  tokens_total?: number;
  tokens_in?: number;
  tokens_out?: number;
  tenants?: number;
  skills?: number;
  plugins?: number;
  scheduler_jobs?: number;
  conversations?: number;
  request_latency?: {
    count?: number;
    avg_ms?: number;
    p50_ms?: number;
    p90_ms?: number;
    p95_ms?: number;
    p99_ms?: number;
  };
  recent_errors?: Array<{ message: string; count: number }>;
  [key: string]: unknown;
}
export interface SystemHealth {
  status?: string;
  version?: string;
  uptime_sec?: number;
  breaker_state?: string;
  [key: string]: unknown;
}

// --- Trust ---
export interface TrustEntry {
  score: number;
  level?: string;
  [key: string]: unknown;
}
export interface TrustScoresResponse {
  scores: Record<string, TrustEntry>;
  [key: string]: unknown;
}

// --- Notification channels ---
export interface NotifyChannel {
  id?: string;
  type: string;
  name?: string;
  enabled?: boolean;
  config?: Record<string, unknown>;
  [key: string]: unknown;
}
export interface NotifyChannelsResponse {
  channels: NotifyChannel[];
  [key: string]: unknown;
}

// --- State snapshot ---
export interface StateGoal {
  id?: string;
  title: string;
  status?: string;
  priority?: number;
  progress?: number;
  [key: string]: unknown;
}
export interface StateResource {
  id?: string;
  path: string;
  type?: string;
  status?: string;
  [key: string]: unknown;
}
export interface StateCapabilities {
  total_skills?: number;
  dynamic_skills?: string[];
  unresolved_gaps?: number;
  recent_gaps?: string[];
  [key: string]: unknown;
}
export interface StateSnapshot {
  goals?: StateGoal[];
  resources?: StateResource[];
  focus?: string;
  topics?: string[];
  capabilities?: StateCapabilities;
  updated_at?: string;
  [key: string]: unknown;
}

// --- Cost ---
export interface CostSummary {
  today_cost?: number;
  month_cost?: number;
  status?: string;
  [key: string]: unknown;
}
export interface CostBreakdown {
  by_channel?: Record<string, number>;
  by_tier?: Record<string, number>;
  by_runner_type?: Record<string, number>;
  by_provider?: Record<string, number>;
  [key: string]: unknown;
}

// --- Modules ---
export interface ModuleEntry {
  name?: string;
  id?: string;
  status?: string;
  enabled?: boolean;
  [key: string]: unknown;
}
export interface ModulesResponse {
  modules: ModuleEntry[];
  profile?: string;
  [key: string]: unknown;
}

// --- RBAC roles (for the trust/rbac tab) ---
export interface RBACRole {
  id: string;
  name: string;
  description?: string;
  is_built_in?: boolean;
  permissions?: Array<{ resource: string; action: string }>;
  [key: string]: unknown;
}
export interface RBACRolesResponse {
  roles: RBACRole[];
  total: number;
  [key: string]: unknown;
}

// --- Client functions ---

export function fetchSystemStats(): Promise<SystemStats> {
  return fetcher<SystemStats>("/v1/metrics");
}

export function fetchSystemHealth(): Promise<SystemHealth> {
  return fetcher<SystemHealth>("/healthz");
}

export function fetchTrustScores(): Promise<TrustScoresResponse> {
  return fetcher<TrustScoresResponse>("/v1/trust/scores");
}

export function trustGrant(slug: string): Promise<unknown> {
  return fetcher<unknown>(`/v1/trust/grant`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ slug }),
  });
}

export function trustReset(slug: string): Promise<unknown> {
  return fetcher<unknown>(`/v1/trust/reset`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ slug }),
  });
}

export function fetchNotifyChannels(): Promise<NotifyChannelsResponse> {
  return fetcher<NotifyChannelsResponse>("/v1/notify/channels");
}

export function fetchStateSnapshot(): Promise<StateSnapshot> {
  return fetcher<StateSnapshot>("/v1/state");
}

export function fetchCostSummary(): Promise<CostSummary> {
  return fetcher<CostSummary>("/v1/cost/summary");
}

export function fetchCostBreakdown(): Promise<CostBreakdown> {
  return fetcher<CostBreakdown>("/v1/cost/breakdown");
}

export function fetchModules(): Promise<ModulesResponse> {
  return fetcher<ModulesResponse>("/v1/modules");
}

export function fetchRbacRoles(): Promise<RBACRolesResponse> {
  return fetcher<RBACRolesResponse>("/v1/rbac/roles");
}
