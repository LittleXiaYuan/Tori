import { fetcher } from "./api-core";

export interface RPAReplayParamDef {
  type?: string;
  description?: string;
  required?: boolean;
  default?: string;
}

export interface RPAReplayStepAssertion {
  type: string;
  selector?: string;
  expected?: string;
}

export interface RPAReplayTraceStep {
  index?: number;
  action: string;
  selector?: string;
  value?: string;
  param_ref?: string;
  screenshot?: string;
  assertion?: RPAReplayStepAssertion;
  timestamp_ms?: number;
}

export interface RPAReplayTraceSummary {
  slug: string;
  name: string;
  description?: string;
  target_url?: string;
  recorded_at: string;
  step_count: number;
  success_rate?: number;
  avg_duration_ms?: number;
}

export interface RPAReplayTrace extends Omit<RPAReplayTraceSummary, "step_count"> {
  type: "rpa-replay";
  parameters?: Record<string, RPAReplayParamDef>;
  steps: RPAReplayTraceStep[];
}

export interface RPAReplayStatus {
  pack_id: string;
  stage: string;
  executor_ready: boolean;
  trace_count: number;
  active_recordings: number;
  store_dir?: string;
  capabilities: string[];
}

export interface RPARecordingSession {
  id: string;
  slug?: string;
  name?: string;
  description?: string;
  target_url?: string;
  started_at: string;
  status: string;
}

export interface RPAReplayResult {
  success: boolean;
  dry_run: boolean;
  output?: string;
  steps_run: number;
  failed_step: number;
  fail_reason?: string;
  duration_ms: number;
  planned_steps?: RPAReplayTraceStep[];
}

export interface RPAReplayPackClient {
  status(): Promise<RPAReplayStatus>;
  traces(): Promise<{ traces: RPAReplayTraceSummary[]; count: number }>;
  createTrace(trace: Partial<RPAReplayTrace> & { slug: string; name: string; steps: RPAReplayTraceStep[] }): Promise<{ trace: RPAReplayTrace; status: string }>;
  trace(slug: string): Promise<{ trace: RPAReplayTrace }>;
  startRecording(input?: Partial<RPARecordingSession> & { parameters?: Record<string, RPAReplayParamDef> }): Promise<{ session: RPARecordingSession; status: string; note?: string }>;
  stopRecording(input: { session_id: string; slug?: string; name?: string; steps?: RPAReplayTraceStep[] }): Promise<{ trace: RPAReplayTrace; status: string }>;
  replay(input: { slug: string; params?: Record<string, string>; dry_run?: boolean }): Promise<{ result: RPAReplayResult; trace: string }>;
  evidence(slug: string): Promise<{ pack_id: string; exported_at: string; format: string; files: string[]; trace: RPAReplayTrace }>;
}

function enc(value: string): string {
  return encodeURIComponent(value);
}

export function createRPAReplayPackClient(): RPAReplayPackClient {
  return {
    status: () => fetcher<RPAReplayStatus>("/v1/rpa-replay/status"),
    traces: () => fetcher<{ traces: RPAReplayTraceSummary[]; count: number }>("/v1/rpa-replay/traces"),
    createTrace: (trace) =>
      fetcher<{ trace: RPAReplayTrace; status: string }>("/v1/rpa-replay/traces", {
        method: "POST",
        body: JSON.stringify(trace),
      }),
    trace: (slug) => fetcher<{ trace: RPAReplayTrace }>(`/v1/rpa-replay/traces/${enc(slug)}`),
    startRecording: (input = {}) =>
      fetcher<{ session: RPARecordingSession; status: string; note?: string }>("/v1/rpa-replay/recordings/start", {
        method: "POST",
        body: JSON.stringify(input),
      }),
    stopRecording: (input) =>
      fetcher<{ trace: RPAReplayTrace; status: string }>("/v1/rpa-replay/recordings/stop", {
        method: "POST",
        body: JSON.stringify(input),
      }),
    replay: (input) =>
      fetcher<{ result: RPAReplayResult; trace: string }>("/v1/rpa-replay/replay", {
        method: "POST",
        body: JSON.stringify(input),
      }),
    evidence: (slug) => fetcher<{ pack_id: string; exported_at: string; format: string; files: string[]; trace: RPAReplayTrace }>(`/v1/rpa-replay/evidence/${enc(slug)}`),
  };
}
