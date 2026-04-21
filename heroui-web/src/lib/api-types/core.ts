// ══════════════════════════════════════════════════════════════════════════
// Core / Observability Types
// ══════════════════════════════════════════════════════════════════════════
// System metadata, version + metrics, model registry, search, tenant, and
// the shared LLMMessage shape. Kept lean so it can be imported from any
// other api-types module without creating cycles.

export interface DocTemplate {
  id: string;
  name: string;
  description: string;
  category: string;
  format: string;
  icon: string;
  content: string;
}

export interface VersionInfo {
  version: string;
  git_commit: string;
  build_date: string;
  go_version: string;
  os: string;
  arch: string;
}

export interface MetricsSnapshot {
  uptime: number;
  requests_total: number;
  requests_success: number;
  requests_failed: number;
  tokens_in: number;
  tokens_out: number;
  tokens_total: number;
  request_latency: { count: number; avg_ms: number; p50_ms: number; p95_ms: number; p99_ms: number; max_ms: number };
  skills: Array<{
    name: string;
    total: number;
    success: number;
    failed: number;
    success_rate: number;
    latency: { avg_ms: number; p50_ms: number };
  }>;
  recent_errors: Array<{ message: string; count: number; last: string }>;
}

export interface TenantInfo {
  id: string;
  name: string;
  api_key: string;
  created_at: string;
}

export interface ModelInfo {
  id: string;
  model_id: string;
  name: string;
  type: string;
  client_type: string;
  base_url?: string;
  input_modalities?: string[];
  supports_reasoning: boolean;
  dimensions?: number;
}

export interface SystemInfo {
  version: string;
  go_version: string;
  os: string;
  arch: string;
  uptime_seconds: number;
  memory_mb: number;
  goroutines: number;
  cpu_count: number;
  hostname: string;
}

export interface CacheStats {
  hits: number;
  misses: number;
  size: number;
  max_size: number;
  hit_rate: number;
}

export interface RouterStats {
  total_requests: number;
  routes: Record<string, { count: number; avg_ms: number }>;
}

export interface SearchResult {
  id: string;
  type: string;
  title: string;
  content: string;
  score: number;
  source: string;
}

export interface LLMMessage {
  role: string;
  content: string;
  created_at?: string;
}
