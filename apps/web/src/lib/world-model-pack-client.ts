import { fetcher } from "./api-core";

export interface WorldStateEntry {
  key: string;
  kind: string;
  value: string;
  confidence: number;
  last_verified: string;
  updated_by: string;
  dependencies?: string[];
}

export interface StateResponse {
  entries: WorldStateEntry[];
  total: number;
}

export interface StaleResponse {
  keys: string[];
  max_age: number;
}

export interface CausalLink {
  cause_event_id: string;
  effect_event_id: string;
  cause_kind: string;
  effect_kind: string;
  strength: number;
  mechanism: string;
}

export interface CausalChain {
  links: CausalLink[];
  root_cause: string;
  final_effect: string;
}

export interface RootCauseResponse {
  task_id: string;
  chain?: CausalChain | null;
}

export interface TimelineEvent {
  id: string;
  kind: string;
  actor: string;
  created_at: string;
}

export interface TimelineEntry {
  event: TimelineEvent;
  gap_before: number;
}

export interface TimelineResponse {
  task_id: string;
  entries: TimelineEntry[];
}

export interface FailurePattern {
  cause_kind: string;
  effect_kind: string;
  mechanism: string;
  occurrences: number;
  task_ids: string[];
}

export interface FailurePatternsResponse {
  patterns: FailurePattern[];
}

export interface WorldModelPackClient {
  state(kind?: string): Promise<StateResponse>;
  stale(maxAge?: string): Promise<StaleResponse>;
  timeline(taskId: string): Promise<TimelineResponse>;
  rootCause(taskId: string): Promise<RootCauseResponse>;
  failurePatterns(limit?: number): Promise<FailurePatternsResponse>;
}

function buildQuery(params: Record<string, string | number | undefined>): string {
  const parts: string[] = [];
  for (const [k, v] of Object.entries(params)) {
    if (v === undefined || v === "") continue;
    parts.push(`${k}=${encodeURIComponent(v)}`);
  }
  return parts.length ? `?${parts.join("&")}` : "";
}

export function createWorldModelPackClient(): WorldModelPackClient {
  return {
    state: (kind) => fetcher<StateResponse>(`/v1/world/state${buildQuery({ kind })}`),
    stale: (maxAge) => fetcher<StaleResponse>(`/v1/world/state/stale${buildQuery({ max_age: maxAge })}`),
    timeline: (taskId) =>
      fetcher<TimelineResponse>(`/v1/world/causal/timeline${buildQuery({ task_id: taskId })}`),
    rootCause: (taskId) =>
      fetcher<RootCauseResponse>(`/v1/world/causal/root-cause${buildQuery({ task_id: taskId })}`),
    failurePatterns: (limit) =>
      fetcher<FailurePatternsResponse>(`/v1/world/causal/failure-patterns${buildQuery({ limit })}`),
  };
}
