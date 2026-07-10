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
  recent(limit?: number, raw?: boolean): Promise<TraceEventsResponse>;
  byTask(taskId: string, raw?: boolean): Promise<TraceEventsResponse>;
  byTrace(traceId: string, raw?: boolean): Promise<TraceEventsResponse>;
}

const enc = encodeURIComponent;

export function createTracePackClient(): TracePackClient {
  return {
    recent: (limit = 50, raw = false) => fetcher<TraceEventsResponse>(`/v1/trace/recent?limit=${limit}${raw ? "&raw=1" : ""}`),
    byTask: (taskId, raw = false) => fetcher<TraceEventsResponse>(`/v1/trace/task/${enc(taskId)}${raw ? "?raw=1" : ""}`),
    byTrace: (traceId, raw = false) => fetcher<TraceEventsResponse>(`/v1/trace/${enc(traceId)}${raw ? "?raw=1" : ""}`),
  };
}
