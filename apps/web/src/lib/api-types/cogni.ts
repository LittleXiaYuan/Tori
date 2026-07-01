// Cogni declarative AI-cognition types.
// Mirrors pkg/cogni.Declaration (Go).  Fields are deliberately loose where
// the backend accepts extensions so newer declarations don't break older UIs.

export interface CogniActivation {
  min_score?: number;
  keywords?: string[];
  keyword_weight?: number;
  regex?: string[];
  regex_weight?: number;
  channels?: string[];
  tenants?: string[];
  always_on?: boolean;
  handover_on?: string[];
}

export interface CogniToolSurface {
  only?: string[];
  include?: string[];
  exclude?: string[];
  from_capsules?: string[];
  max_tools?: number;
}

export interface CogniContextInjection {
  static?: string;
  memory_query?: string;
  memory_top_k?: number;
  template?: string;
}

export interface CogniActivationCheck {
  name?: string;
  message: string;
  tenant?: string;
  channel?: string;
  expect_active?: boolean;
  expect_score_at_least?: number;
  expect_reason_contains?: string[];
}

export interface CogniDeclaration {
  id: string;
  display_name?: string;
  description?: string;
  capsule?: string;
  activation?: CogniActivation;
  surface?: CogniToolSurface;
  context?: CogniContextInjection;
  priority?: number;
  exclusive?: string;
  checks?: CogniActivationCheck[];
  workflows?: CogniWorkflowDef[];
}

export interface CogniEntryStatus {
  id: string;
  display_name?: string;
  description?: string;
  capsule?: string;
  enabled: boolean;
  source?: string;
  loaded_at?: string;
  load_error?: string;
  priority: number;
  exclusive?: string;
  always_on: boolean;
}

export interface CogniHealthMetrics {
  id: string;
  score: number;
  status: "healthy" | "warn" | "unhealthy" | "idle";
  window: number;
  evaluations: number;
  activations: number;
  suppressed: number;
  activation_rate: number;
  suppression_rate: number;
  avg_duration_ms: number;
  avg_context_bytes: number;
  template_fallback_rate: number;
  tool_filter_ratio: number;
  last_seen_at?: string;
}

export interface CogniAlert {
  cogni_id: string;
  kind: string;
  severity: "info" | "warn" | "critical";
  message: string;
  score: number;
  since: string;
  last_checked_at: string;
  auto_action_taken?: string;
}

export interface CogniTraceActivation {
  id: string;
  display_name?: string;
  score: number;
  activated: boolean;
  reasons?: string[];
  suppressed?: boolean;
  suppressed_by?: string;
}

export interface CogniTraceContext {
  bytes: number;
  sources?: string[];
  template_fallbacks?: number;
}

export interface CogniTraceToolFilter {
  before: number;
  after: number;
  removed?: string[];
  applied_by?: string[];
  fellback_to_input?: boolean;
}

export interface CogniTrace {
  timestamp: string;
  tenant?: string;
  channel?: string;
  message_hash?: string;
  message_len: number;
  activations?: CogniTraceActivation[];
  context?: CogniTraceContext;
  tool_filter?: CogniTraceToolFilter;
  duration_ms: number;
}

export interface CogniListResponse {
  cognis: CogniEntryStatus[];
  health?: Record<string, CogniHealthMetrics>;
  count: number;
  version: number;
  dir: string;
}

export interface CogniReloadResponse {
  status: string;
  dir: string;
  added: number;
  updated: number;
  removed: number;
  errors?: Array<{ file: string; path: string; error: string }>;
  version: number;
}

// ── Workflow Types ──
export interface CogniWorkflowStep {
  name?: string;
  skill: string;
  args?: Record<string, unknown>;
  output?: string;
  condition?: string;
  timeout?: string;
  on_error?: string;
}

export interface CogniWorkflowDef {
  name: string;
  description?: string;
  steps: CogniWorkflowStep[];
}

export interface CogniWorkflowResult {
  workflow_name: string;
  success: boolean;
  outputs: Record<string, unknown>;
  step_results: Array<{
    step_index: number;
    step_name: string;
    skill: string;
    skipped?: boolean;
    output?: unknown;
    duration: number;
    error?: string;
  }>;
  duration: number;
  error?: string;
}

// ── Experience Types ──
export interface CogniExperienceStats {
  tool_memories: number;
  patterns_total: number;
  patterns_confirmed: number;
  patterns_pending?: number;
  domain_facts: number;
}

// ── Evolution Types ──
export interface CogniExperiment {
  id: string;
  cogni_id: string;
  date: string;
  change: string;
  baseline_score: number;
  result_score: number;
  delta: number;
  status: "kept" | "reverted" | "pending";
  reason?: string;
  affected_tasks?: string[];
}

// ── Verify Types ──
export interface CogniCheckResult {
  cogni_id: string;
  check_name?: string;
  check_index: number;
  passed: boolean;
  reason?: string;
  got_active: boolean;
  got_score: number;
}

export interface CogniVerifyResponse {
  results: Record<string, CogniCheckResult[]>;
  failures: Array<{ cogni_id: string; check_name?: string; check_index: number; reason?: string }>;
}

// ── Experience Response ──
export interface CogniExperiencePattern {
  id?: string;
  trigger: string;
  response: string;
  confirmed?: boolean;
  used_count?: number;
  success_rate?: number;
  created_at?: string;
  last_used?: string;
}

export interface CogniToolExperience {
  tool: string;
  context?: string;
  result?: string;
  learned?: string;
  confidence?: number;
  verified_by?: string;
  used_count?: number;
  success_rate?: number;
  created_at?: string;
  last_used?: string;
}

export interface CogniDomainFact {
  fact: string;
  source?: string;
  used_count?: number;
  created_at?: string;
  last_used?: string;
}

export interface CogniExperienceSummary {
  stats?: CogniExperienceStats;
  top_tools?: CogniToolExperience[];
  top_facts?: CogniDomainFact[];
  pending_patterns?: CogniExperiencePattern[];
  updated_at?: string;
}

export interface CogniExperienceResponse {
  id: string;
  enabled: boolean;
  summary?: CogniExperienceSummary;
  stats?: CogniExperienceStats;
  tool_memory?: CogniToolExperience[];
  patterns?: CogniExperiencePattern[];
  domain_facts?: CogniDomainFact[];
}

// ── Evolution Response ──
export interface CogniEvolutionResponse {
  running: boolean;
  experiments: CogniExperiment[];
}

// ── Federation Types ──
export interface CogniFederationPeer {
  id: string;
  name: string;
  url: string;
  last_seen?: string;
  status: "online" | "offline" | "unknown";
  cognis?: string[];
}
