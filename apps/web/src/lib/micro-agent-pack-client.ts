import { fetcher } from "./api-core";

export interface AgentEntry {
  name: string;
  description?: string;
  scope: string;
  trigger?: string;
  content: string;
  enabled: boolean;
  priority: number;
  tags?: string[];
  metadata?: Record<string, string>;
}

export interface AgentsResponse {
  agents: AgentEntry[];
  total: number;
}

export interface ResolveResponse {
  message: string;
  matched: AgentEntry[];
}

export interface TraceEntry {
  id: string;
  kind: string;
  actor: string;
  created_at: string;
  payload?: Record<string, unknown>;
}

export interface TraceResponse {
  task_id: string;
  entries: TraceEntry[];
}

export interface MicroAgentPackClient {
  agents(scope?: string): Promise<AgentsResponse>;
  resolve(message: string): Promise<ResolveResponse>;
  trace(taskId: string, limit?: number): Promise<TraceResponse>;
}

function buildQuery(params: Record<string, string | number | undefined>): string {
  const parts: string[] = [];
  for (const [k, v] of Object.entries(params)) {
    if (v === undefined || v === "") continue;
    parts.push(`${k}=${encodeURIComponent(v)}`);
  }
  return parts.length ? `?${parts.join("&")}` : "";
}

export function createMicroAgentPackClient(): MicroAgentPackClient {
  return {
    agents: (scope) => fetcher<AgentsResponse>(`/v1/micro-agent/agents${buildQuery({ scope })}`),
    resolve: (message) =>
      fetcher<ResolveResponse>(`/v1/micro-agent/resolve${buildQuery({ message })}`),
    trace: (taskId, limit) =>
      fetcher<TraceResponse>(`/v1/micro-agent/react/trace${buildQuery({ task_id: taskId, limit })}`),
  };
}
