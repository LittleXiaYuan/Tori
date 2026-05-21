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
  executor_plan_ready?: boolean;
  executor_ready: boolean;
  action_tracer_plan_ready?: boolean;
  action_tracer_ready?: boolean;
  browser_intent_gate_plan_ready?: boolean;
  browser_intent_ready?: boolean;
  consumes_browser_intent?: boolean;
  executes_browser_actions?: boolean;
  writes_browser_state?: boolean;
  writes_files?: boolean;
  network_access?: boolean;
  trace_count: number;
  active_recordings: number;
  store_dir?: string;
  capabilities: string[];
  notes?: string[];
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

export interface RPAReplayExecutorStepPlan {
  index: number;
  action: string;
  executor_action: string;
  selector?: string;
  value?: string;
  assertion?: RPAReplayStepAssertion;
  requires_browser_intent: boolean;
  requires_action_tracer: boolean;
  executes_browser_action: boolean;
  writes_browser_state: boolean;
  consumes_external_target: boolean;
}

export interface RPAReplayBrowserIntentGatePlan {
  target: string;
  required_pack_id: string;
  capability: string;
  browser_intent_gate_plan_ready: boolean;
  browser_intent_ready: boolean;
  consumes_browser_intent: boolean;
  executes_browser_actions: boolean;
  writes_browser_state: boolean;
  network_access: boolean;
  blocked_by: string[];
  notes?: string[];
}

export interface RPAReplayActionTracerHandoffPlan {
  target: string;
  action_tracer_plan_ready: boolean;
  action_tracer_ready: boolean;
  captures_runtime_trace: boolean;
  writes_trace_store: boolean;
  expected_artifacts: string[];
  blocked_by: string[];
  notes?: string[];
}

export interface RPAReplayExecutorHandoffPlan {
  target: string;
  dedup_key: string;
  trace_slug: string;
  executor: string;
  executor_plan_ready: boolean;
  executor_ready: boolean;
  action_tracer_plan_ready: boolean;
  action_tracer_ready: boolean;
  browser_intent_gate_plan_ready: boolean;
  browser_intent_ready: boolean;
  consumes_browser_intent: boolean;
  executes_browser_actions: boolean;
  writes_browser_state: boolean;
  writes_files: boolean;
  network_access: boolean;
  step_count: number;
  steps: RPAReplayExecutorStepPlan[];
  blocked_by: string[];
  notes?: string[];
}

export interface RPAReplayExecutorPlan {
  pack_id: string;
  generated_at: string;
  status: string;
  stage: string;
  dry_run: boolean;
  trace_slug: string;
  trace_name: string;
  executor: string;
  requested_by?: string;
  reason?: string;
  executor_plan_ready: boolean;
  executor_ready: boolean;
  action_tracer_plan_ready: boolean;
  action_tracer_ready: boolean;
  browser_intent_gate_plan_ready: boolean;
  browser_intent_ready: boolean;
  consumes_browser_intent: boolean;
  executes_browser_actions: boolean;
  writes_browser_state: boolean;
  writes_files: boolean;
  network_access: boolean;
  action_count: number;
  planned_steps: RPAReplayExecutorStepPlan[];
  executor_handoff_plan: RPAReplayExecutorHandoffPlan;
  browser_intent_gate_plan: RPAReplayBrowserIntentGatePlan;
  action_tracer_handoff_plan: RPAReplayActionTracerHandoffPlan;
  artifacts: string[];
  actions: string[];
  blocked_by: string[];
  labels: string[];
  notes?: string[];
}

export interface RPAReplayPackClient {
  status(): Promise<RPAReplayStatus>;
  traces(): Promise<{ traces: RPAReplayTraceSummary[]; count: number }>;
  createTrace(trace: Partial<RPAReplayTrace> & { slug: string; name: string; steps: RPAReplayTraceStep[] }): Promise<{ trace: RPAReplayTrace; status: string }>;
  trace(slug: string): Promise<{ trace: RPAReplayTrace }>;
  startRecording(input?: Partial<RPARecordingSession> & { parameters?: Record<string, RPAReplayParamDef> }): Promise<{ session: RPARecordingSession; status: string; note?: string }>;
  stopRecording(input: { session_id: string; slug?: string; name?: string; steps?: RPAReplayTraceStep[] }): Promise<{ trace: RPAReplayTrace; status: string }>;
  replay(input: { slug: string; params?: Record<string, string>; dry_run?: boolean }): Promise<{ result: RPAReplayResult; trace: string }>;
  executorPlan(input: { slug: string; params?: Record<string, string>; executor?: string; requested_by?: string; reason?: string; dry_run?: boolean }): Promise<{ plan: RPAReplayExecutorPlan }>;
  evidence(slug: string): Promise<{ pack_id: string; exported_at: string; format: string; files: string[]; trace: RPAReplayTrace; executor_plan?: RPAReplayExecutorPlan; executor_handoff_plan?: RPAReplayExecutorHandoffPlan; browser_intent_gate_plan?: RPAReplayBrowserIntentGatePlan; action_tracer_handoff_plan?: RPAReplayActionTracerHandoffPlan }>;
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
    executorPlan: (input) =>
      fetcher<{ plan: RPAReplayExecutorPlan }>("/v1/rpa-replay/executor/plan", {
        method: "POST",
        body: JSON.stringify(input),
      }),
    evidence: (slug) => fetcher<{ pack_id: string; exported_at: string; format: string; files: string[]; trace: RPAReplayTrace; executor_plan?: RPAReplayExecutorPlan; executor_handoff_plan?: RPAReplayExecutorHandoffPlan; browser_intent_gate_plan?: RPAReplayBrowserIntentGatePlan; action_tracer_handoff_plan?: RPAReplayActionTracerHandoffPlan }>(`/v1/rpa-replay/evidence/${enc(slug)}`),
  };
}
