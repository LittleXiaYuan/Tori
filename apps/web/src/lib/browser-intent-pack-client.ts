import { fetcher } from "./api-core";
import type { BrowserScenario, BrowserStatus, OPPItem, ScenarioStepResult } from "./api-types/integrations";

export type { BrowserScenario, BrowserStatus, OPPItem, ScenarioStepResult } from "./api-types/integrations";

export interface BrowserConfig {
  mode?: string;
  connected?: boolean;
  headless?: boolean;
  [key: string]: unknown;
}

export interface BrowserScreenshotResponse {
  screenshot?: string;
  timestamp?: string;
}

export interface BrowserOCRResponse {
  text?: string;
  result?: string;
}

export interface BrowserExtensionStatus {
  connected: boolean;
  version?: string;
  pending?: number;
  error?: string;
}

export interface BrowserExtensionSession {
  ok: boolean;
  ws_url: string;
  ticket: string;
  nonce: string;
  expires_at: string;
  ttl_sec: number;
}

export interface BrowserDesktopSandbox {
  id: string;
  stream_url: string;
  created_at: string;
  vnc_log?: string[];
}

export interface BrowserActPlanRequest {
  intent?: string;
  target_url?: string;
  selector?: string;
  text?: string;
  value?: string;
  requested_by?: string;
  reason?: string;
  dry_run?: boolean;
  metadata?: Record<string, unknown>;
}

export interface BrowserActPlannedAction {
  index: number;
  intent: string;
  executor_action: string;
  target_url?: string;
  selector?: string;
  text?: string;
  value?: string;
  requires_permission: string;
  requires_runtime_skill: string;
  requires_opp_gate: boolean;
  consumes_browser_session: boolean;
  executes_browser_action: boolean;
  writes_browser_state: boolean;
  network_access: boolean;
}

export interface BrowserActGatePlan {
  gate: string;
  blocked_by: string[];
  notes?: string[];
  [key: string]: unknown;
}

export interface BrowserActPlan {
  pack_id: string;
  generated_at: string;
  stage: string;
  status: string;
  dry_run: boolean;
  intent: string;
  requested_by?: string;
  reason?: string;
  browser_act_plan_ready: boolean;
  browser_act_ready: boolean;
  permission_gate_ready: boolean;
  runtime_skill_gate_ready: boolean;
  opp_gate_ready: boolean;
  consumes_browser_session: boolean;
  executes_browser_actions: boolean;
  writes_browser_state: boolean;
  writes_files: boolean;
  network_access: boolean;
  requires_human_approval: boolean;
  action_count: number;
  planned_actions: BrowserActPlannedAction[];
  permission_gate: BrowserActGatePlan;
  runtime_skill_gate: BrowserActGatePlan;
  opp_gate: BrowserActGatePlan;
  artifacts: string[];
  actions: string[];
  blocked_by: string[];
  labels: string[];
  notes?: string[];
}

export interface BrowserIntentPackClient {
  status(): Promise<BrowserStatus>;
  config(): Promise<BrowserConfig>;
  browserActPlan(input: BrowserActPlanRequest): Promise<{ plan: BrowserActPlan }>;
  navigate(url: string): Promise<{ screenshot?: string; title?: string; url?: string }>;
  screenshot(): Promise<BrowserScreenshotResponse>;
  screenshotLatest(): Promise<BrowserScreenshotResponse>;
  ocr(mode: string): Promise<BrowserOCRResponse>;
  oppPending(): Promise<{ items: OPPItem[]; total: number }>;
  oppDecide(id: string, decision: "allow" | "deny"): Promise<{ status: string }>;
  extensionStatus(): Promise<BrowserExtensionStatus>;
  extensionSession(): Promise<BrowserExtensionSession>;
  extensionAction(action: Record<string, unknown>): Promise<{ ok: boolean; error?: string; screenshot?: string }>;
  scenarios(): Promise<{ scenarios: BrowserScenario[] }>;
  runScenario(scenarioId: string): Promise<{ ok: boolean; scenario: string; results: ScenarioStepResult[] }>;
  desktopStatus(): Promise<{ ok: boolean; running: boolean; sandbox?: BrowserDesktopSandbox; alive?: boolean }>;
  desktopCreate(): Promise<{ ok: boolean; sandbox?: BrowserDesktopSandbox; message?: string }>;
  desktopDestroy(): Promise<{ ok: boolean; message?: string }>;
}

export function createBrowserIntentPackClient(): BrowserIntentPackClient {
  return {
    status: () => fetcher<BrowserStatus>("/v1/browser/status"),
    config: () => fetcher<BrowserConfig>("/v1/browser/config"),
    browserActPlan: (input) =>
      fetcher<{ plan: BrowserActPlan }>("/v1/browser/intent/plan", {
        method: "POST",
        body: JSON.stringify(input),
      }),
    navigate: (url) =>
      fetcher<{ screenshot?: string; title?: string; url?: string }>("/v1/browser/navigate", {
        method: "POST",
        body: JSON.stringify({ url }),
      }),
    screenshot: () => fetcher<BrowserScreenshotResponse>("/v1/browser/screenshot"),
    screenshotLatest: () => fetcher<BrowserScreenshotResponse>("/v1/browser/screenshot/latest"),
    ocr: (mode) =>
      fetcher<BrowserOCRResponse>("/v1/browser/ocr", {
        method: "POST",
        body: JSON.stringify({ mode }),
      }),
    oppPending: () => fetcher<{ items: OPPItem[]; total: number }>("/v1/browser/opp/pending"),
    oppDecide: (id, decision) =>
      fetcher<{ status: string }>("/v1/browser/opp/decide", {
        method: "POST",
        body: JSON.stringify({ id, decision }),
      }),
    extensionStatus: () => fetcher<BrowserExtensionStatus>("/api/browser/ext/status"),
    extensionSession: () => fetcher<BrowserExtensionSession>("/api/browser/ext/session", { method: "POST" }),
    extensionAction: (action) =>
      fetcher<{ ok: boolean; error?: string; screenshot?: string }>("/api/browser/ext/action", {
        method: "POST",
        body: JSON.stringify(action),
      }),
    scenarios: () => fetcher<{ scenarios: BrowserScenario[] }>("/api/browser/ext/scenarios"),
    runScenario: (scenarioId) =>
      fetcher<{ ok: boolean; scenario: string; results: ScenarioStepResult[] }>("/api/browser/ext/scenarios/run", {
        method: "POST",
        body: JSON.stringify({ scenario_id: scenarioId }),
      }),
    desktopStatus: () => fetcher<{ ok: boolean; running: boolean; sandbox?: BrowserDesktopSandbox; alive?: boolean }>("/v1/sandbox/desktop/status"),
    desktopCreate: () => fetcher<{ ok: boolean; sandbox?: BrowserDesktopSandbox; message?: string }>("/v1/sandbox/desktop", { method: "POST" }),
    desktopDestroy: () => fetcher<{ ok: boolean; message?: string }>("/v1/sandbox/desktop/destroy", { method: "POST" }),
  };
}
