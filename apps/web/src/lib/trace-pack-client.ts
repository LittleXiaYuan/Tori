import type { AgentEvent } from "@/components/execution-trace";
import { fetcher } from "./api-core";

export interface TraceEventsResponse {
  count: number;
  raw: boolean;
  events: AgentEvent[];
  task_id?: string;
  trace_id?: string;
}

export interface TracePackClient {
  recent(limit?: number): Promise<TraceEventsResponse>;
  byTask(taskId: string): Promise<TraceEventsResponse>;
  byTrace(traceId: string): Promise<TraceEventsResponse>;
}

const enc = encodeURIComponent;

export function createTracePackClient(): TracePackClient {
  return {
    recent: (limit = 50) => fetcher<TraceEventsResponse>(`/v1/trace/recent?limit=${limit}`),
    byTask: (taskId) => fetcher<TraceEventsResponse>(`/v1/trace/task/${enc(taskId)}`),
    byTrace: (traceId) => fetcher<TraceEventsResponse>(`/v1/trace/${enc(traceId)}`),
  };
}
