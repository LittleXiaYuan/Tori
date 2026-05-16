// ══════════════════════════════════════════════════════════════════════════
// Runtime Types — Conversations / Tasks / Bots / Triggers / Workflow / Cost
// ══════════════════════════════════════════════════════════════════════════
// Operational state: anything that represents an in-flight piece of agent
// work (a task, a thread, a workflow node, a queued tool call, an audit
// row, a scheduled trigger). The largest module by design — these types
// share a lot of structural references (TaskInfo→TaskStep→TaskWorkingMemory,
// TriggerDef→TriggerAction, WorkflowDef→WorkflowNode→Edge→Variable, etc.).

// --- Channels (inbox / bots / heartbeat / qq) ---

export interface InboxItem {
  id: string;
  source: string;
  content: string;
  action: string;
  is_read: boolean;
  created_at: string;
  read_at?: string;
}

export interface InboxResponse {
  items: InboxItem[];
  count: { unread: number; total: number };
}

export interface HeartbeatLog {
  id: string;
  status: string;
  result?: string;
  error?: string;
  summary?: string;
  timestamp?: string;
  started_at: string;
  completed_at?: string;
  duration?: string;
}

export interface BotInfo {
  id: string;
  name: string;
  description: string;
  system_prompt?: string;
  is_active: boolean;
  status: string;
  config: Record<string, unknown>;
  created_at: string;
}

export interface BotsResponse {
  bots: BotInfo[];
  total: number;
  active: number;
}

export interface QQAnalysis {
  id: string;
  file_name: string;
  total_messages: number;
  participants: string[];
  time_range: string;
  summary: string;
  persona_profiles: Record<string, string>;
  top_topics: string[];
  sentiment: string;
  analyzed_at: string;
}

// --- Conversations ---

export interface ConversationInfo {
  id: string;
  tenant_id: string;
  name?: string;
  summary?: string;
  pinned?: boolean;
  archived_at?: string;
  created_at: string;
  updated_at: string;
}

// --- Tasks ---

export interface TaskWorkingMemory {
  TaskID: string;
  Goal: string;
  CompletedWork: string[];
  Blockers: string[];
  Confirmed: string[];
  Pending: string[];
  Artifacts: string[];
  NextAction: string;
  TokenEstimate: number;
}

export interface TaskStep {
  id: number;
  action: string;
  skill_name?: string;
  args?: Record<string, string>;
  status: string;
  result?: string;
  error?: string;
  input?: string;
  retry_count?: number;
  max_retries?: number;
  gap_type?: string;
  depends_on?: number[];
  metadata?: Record<string, unknown>;
  started_at?: string;
  done_at?: string;
  name?: string;
  tool?: string;
  output?: string;
  duration_ms?: number;
}

export interface TaskInfo {
  id: string;
  title: string;
  description: string;
  status: string;
  type?: string;
  priority?: string;
  steps: TaskStep[] | null;
  artifacts?: Array<{ path: string; type: string; name?: string; mime_type?: string; size?: number }>;
  error?: string;
  tenant_id: string;
  created_at: string;
  updated_at: string;
  started_at?: string;
  finished_at?: string;
  working_memory?: TaskWorkingMemory;
  constraints?: {
    extra?: Record<string, unknown>;
    tags?: string[];
    priority?: string;
    [key: string]: unknown;
  };
}

export interface GapRecord {
  id: string;
  task_id: string;
  step_id: number;
  gap_type: string;
  skill_name: string;
  error_message: string;
  suggestion: string;
  resolved: boolean;
  created_at: string;
}

export interface GapStats {
  total: number;
  unresolved: number;
  skill_missing: number;
  param_error: number;
  env_error: number;
  unknown: number;
}

// --- Task templates / state kernel / experience ---

export interface TemplateVar {
  name: string;
  description?: string;
  default?: string;
  required: boolean;
}

export interface TemplateStep {
  action: string;
  skill_name?: string;
  args?: Record<string, unknown>;
  group?: number;
}

export interface TaskTemplate {
  id: string;
  name: string;
  description: string;
  variables: TemplateVar[];
  steps: TemplateStep[];
  tags?: string[];
  created_at: string;
}

export interface StateGoal {
  id: string;
  title: string;
  description?: string;
  priority: number;
  status: string;
  progress: number;
  parent_goal?: string;
  sub_goals?: string[];
  task_ids?: string[];
  created_at: string;
  updated_at: string;
}

export interface StateSnapshot {
  goals: StateGoal[];
  resources: Array<{ id: string; type: string; path: string; status: string }>;
  focus: string;
  topics: string[];
  recent_actions: Array<{ timestamp: string; action: string; result?: string; success: boolean }>;
  capabilities: { total_skills: number; dynamic_skills?: string[]; unresolved_gaps: number; recent_gaps?: string[] };
  updated_at: string;
}

export type ExperienceOutcome = "success" | "failure" | "partial" | "neutral" | (string & {});

export interface ExperienceItem {
  id: string;
  source: string;
  source_id: string;
  category: string;
  outcome: ExperienceOutcome;
  lesson: string;
  context: string;
  tags?: string[];
  created_at: string;
}

export interface ExperienceStats {
  total: number;
  by_source: Record<string, number>;
  by_category: Record<string, number>;
  by_outcome: Record<string, number>;
  recent_7d: number;
}

// --- Task threads ---

export type ThreadState = "open" | "paused" | "closed";

export interface ChannelBinding {
  channel_type: string;
  channel_id: string;
  user_id?: string;
  user_name?: string;
  message_id?: string;
}

export interface TaskThreadInfo {
  task_id: string;
  session_id: string;
  state: ThreadState;
  binding?: ChannelBinding;
  tenant_id: string;
  messages: number;
  created_at: string;
  updated_at: string;
}

// --- Cost tracking (per-task + summary) ---

export interface CostTaskSummary {
  task_id: string;
  total_cost_usd: number;
  total_tokens_in: number;
  total_tokens_out: number;
  calls: number;
  avg_latency_ms: number;
  by_skill?: Record<string, number>;
  by_model?: Record<string, number>;
}

export interface CostUsageEvent {
  model: string;
  tenant_id: string;
  task_id: string;
  step_id: string;
  skill_name: string;
  provider_id: string;
  channel: string;
  runner_type: string;
  tier: string;
  tokens_in: number;
  tokens_out: number;
  cost_usd: number;
  timestamp: string;
  latency: number;
}

export interface CostBreakdown {
  by_channel: Record<string, { total_cost: number; calls: number }>;
  by_tier: Record<string, { total_cost: number; calls: number }>;
  by_runner_type: Record<string, { total_cost: number; calls: number }>;
  by_provider?: Record<string, { total_cost: number; calls: number }>;
}

export interface CostSummary {
  total_cost_usd: number;
  total_tokens_in: number;
  total_tokens_out: number;
  total_calls: number;
  avg_cost_per_call: number;
  period: string;
}

export interface CostHistoryEntry {
  date: string;
  cost_usd: number;
  tokens_in: number;
  tokens_out: number;
  calls: number;
}

export interface CostAlert {
  id: string;
  type: string;
  message: string;
  severity: "info" | "warning" | "critical";
  created_at: string;
}

export interface CostBudget {
  daily_limit_usd: number;
  monthly_limit_usd: number;
  alert_threshold: number;
  current_daily: number;
  current_monthly: number;
}

// --- Cron + Triggers ---

export interface CronJob {
  id: string;
  name: string;
  schedule: { type: string; every_ms?: number; cron_expr?: string; at?: string };
  payload: { kind: string; message?: string };
  enabled: boolean;
  created_at: string;
  last_run_at?: string;
  next_run_at?: string;
  run_count: number;
}

export interface CronRun {
  job_id: string;
  run_id: string;
  started_at: string;
  ended_at: string;
  status: string;
  output?: string;
  error?: string;
}

export interface TriggerItem {
  id: string;
  name: string;
  kind: "time" | "event" | "condition";
  enabled: boolean;
  created_at: string;
  event?: string;
  event_filter?: string;
  condition_expr?: string;
  check_interval?: number;
  action: {
    type: "agent_turn" | "thread_post" | "webhook" | "log";
    message?: string;
    task_id?: string;
    data?: Record<string, unknown>;
  };
  last_fired_at?: string;
  fire_count: number;
}

// --- Trigger V2 ---

export type TriggerType = "time" | "event" | "condition" | "cognitive";
export type TriggerStatus = "active" | "paused" | "disabled";
export type TriggerActionType = "create_task" | "continue_task" | "send_message" | "call_skill" | "write_memory";

export interface TriggerAction {
  type: TriggerActionType;
  task_title?: string;
  task_description?: string;
  task_id?: string;
  message?: string;
  skill_name?: string;
  skill_args?: Record<string, unknown>;
  memory_content?: string;
  profile_key?: string;
  profile_value?: string;
}

export interface TriggerDef {
  id: string;
  name: string;
  description?: string;
  type: TriggerType;
  status: TriggerStatus;
  tenant_id: string;
  thread_id?: string;
  channel_id?: string;
  time_config?: { cron_expr?: string; interval?: string; timezone?: string };
  event_config?: { event_type: string; source_id?: string; filter?: Record<string, string> };
  condition_config?: { check_type: string; target_id?: string; operator: string; value: string; check_interval?: string };
  cognitive_config?: { source_type: string; min_significance?: number; emotion_from?: string; emotion_to?: string; thought_categories?: string[] };
  actions: TriggerAction[];
  budget?: { max_runs_per_day?: number; max_runs_per_week?: number; max_cost_per_run?: number; max_total_cost?: number };
  created_at: string;
  updated_at: string;
  last_run_at?: string;
  next_run_at?: string;
  run_count: number;
  fail_count: number;
  last_error?: string;
  created_by?: string;
}

export interface TriggerRun {
  id: string;
  trigger_id: string;
  tenant_id: string;
  status: "running" | "completed" | "failed" | "skipped";
  started_at: string;
  finished_at?: string;
  duration?: string;
  trigger_type: TriggerType;
  trigger_source: string;
  actions_executed: number;
  actions_succeeded: number;
  actions_failed: number;
  action_results: { action_type: TriggerActionType; status: string; result?: string; error?: string; cost?: number; duration?: string }[];
  total_cost?: number;
  error?: string;
}

export interface TriggerLogEvent {
  id: string;
  trigger_id: string;
  tenant_id: string;
  event_type: string;
  timestamp: string;
  message: string;
  data?: Record<string, unknown>;
  run_id?: string;
  task_id?: string;
}

export interface TriggerEventPayload {
  event: string;
  data?: Record<string, unknown>;
  text?: string;
  tenant_id?: string;
  task_id?: string;
  thread_id?: string;
  channel_id?: string;
}

// --- Approvals / Trust / RBAC / Session queue ---

export interface ApprovalRequest {
  id: string;
  tool_name: string;
  action: string;
  args: Record<string, unknown>;
  risk_level: "safe" | "caution" | "danger" | "critical";
  status: "pending" | "approved" | "denied";
  requester: string;
  reason?: string;
  created_at: string;
  decided_at?: string;
}

export interface ApprovalRule {
  id: string;
  pattern: string;
  scope: "session" | "user" | "global";
  action: "allow" | "deny";
  created_at: string;
}

export interface SessionQueueInfo {
  session_id: string;
  tasks: Array<{
    id: string;
    title: string;
    status: "queued" | "running" | "completed" | "cancelled";
    priority: number;
    created_at: string;
  }>;
  total: number;
  running: number;
}

export interface TrustEntry {
  score: number;
  executions: number;
  failures: number;
  last_promoted?: string;
}

export interface RBACPermission {
  resource: string;
  action: string;
  conditions?: string[];
}

export interface RBACRole {
  id: string;
  name: string;
  description: string;
  permissions: RBACPermission[];
  is_built_in: boolean;
  created_at: string;
}

// --- Workflow Engine ---

export interface WorkflowNodePosition { x: number; y: number; }

export interface WorkflowNode {
  id: string;
  name: string;
  type: string; // "skill" | "llm" | "condition" | "parallel" | "join" | "subflow" | "input" | "transform" | "browser" | "code" | "knowledge"
  config?: Record<string, unknown>;
  position: WorkflowNodePosition;
  timeout?: string;
  retry?: { max_retries: number; delay: string };
}

export interface WorkflowEdge {
  id: string;
  from_node: string;
  to_node: string;
  condition?: string;
  label?: string;
}

export interface WorkflowVariable {
  name: string;
  type: string;
  default?: string;
  required?: boolean;
}

export interface WorkflowDef {
  id: string;
  name: string;
  description: string;
  version: number;
  nodes: WorkflowNode[];
  edges: WorkflowEdge[];
  variables?: WorkflowVariable[];
  tenant_id: string;
  created_at: string;
  updated_at: string;
}

export interface WorkflowNodeState {
  node_id: string;
  status: string; // "pending" | "running" | "done" | "failed" | "skipped" | "waiting"
  input?: unknown;
  output?: unknown;
  error?: string;
  retry_count?: number;
  started_at?: string;
  finished_at?: string;
}

export interface WorkflowGenerateResponse {
  ok: boolean;
  workflow: WorkflowDef;
  generated_by: "llm" | "template" | string;
  message?: string;
}

export interface WorkflowInstance {
  id: string;
  definition_id: string;
  version: number;
  status: string; // "pending" | "running" | "paused" | "completed" | "failed" | "cancelled"
  variables?: Record<string, unknown>;
  node_states?: Record<string, WorkflowNodeState>;
  tenant_id: string;
  created_at: string;
  updated_at: string;
  started_at?: string;
  finished_at?: string;
}

// --- Planner recovery checkpoints ---

export interface PlannerCheckpointStep {
  id: number;
  action: string;
  skill?: string;
  args?: Record<string, unknown>;
  depends_on?: number[];
  status?: string;
  result?: string;
  error?: string;
}

export interface PlannerCheckpointSummary {
  plan_id: string;
  task_id?: string;
  goal?: string;
  status: string;
  current_step: number;
  completed: number;
  total: number;
  steps_used: number;
  revisions: number;
  error?: string;
  recoverable: boolean;
  resume_hint?: string;
  updated_at?: string;
  plan_snapshot?: PlannerCheckpointStep[];
}

export interface PlannerCheckpointListResponse {
  checkpoints: PlannerCheckpointSummary[];
  limit: number;
  count: number;
}

export type PlannerCheckpointRecoveryAction = "continue" | "retry_failed" | "partial";

export interface PlannerCheckpointRecoverRequest {
  plan_id: string;
  action: PlannerCheckpointRecoveryAction;
}

export interface PlannerCheckpointRecoveryPlanStep {
  id: number;
  action: string;
  skill?: string;
  status: string;
  depends_on?: number[];
  selected: boolean;
  reason?: string;
}

export interface PlannerCheckpointRecoveryPlan {
  mode: PlannerCheckpointRecoveryAction;
  executable: boolean;
  reason?: string;
  plan_id: string;
  task_id?: string;
  steps: PlannerCheckpointRecoveryPlanStep[];
  prompt: string;
}

export interface PlannerCheckpointRecoverResponse {
  action: PlannerCheckpointRecoveryAction;
  plan_id: string;
  task_id?: string;
  prompt: string;
  recovery_plan?: PlannerCheckpointRecoveryPlan;
  checkpoint: PlannerCheckpointSummary;
}

export interface PlannerCheckpointResumeTaskResponse {
  status: string;
  task_id: string;
  run: boolean;
  recovery_plan: PlannerCheckpointRecoveryPlan;
  checkpoint: PlannerCheckpointSummary;
}

export interface PlannerCheckpointResumePlanResponse {
  status: string;
  action: PlannerCheckpointRecoveryAction;
  plan_id: string;
  job_id?: string;
  friendly_error?: string;
  recoverable?: boolean;
  next_action?: "retry_failed" | "create_task" | "partial" | "inspect_dependencies" | string;
  result?: {
    reply?: string;
    skills_used?: string[];
    steps?: number;
    plan?: PlannerCheckpointStep[];
  };
  recovery_plan: PlannerCheckpointRecoveryPlan;
  checkpoint: PlannerCheckpointSummary;
}

export interface PlannerCheckpointResumePlanJobEvent {
  id: string;
  type: string;
  summary: string;
  skill?: string;
  timestamp: string;
}

export interface PlannerCheckpointResumePlanJob {
  id: string;
  status: string;
  action: PlannerCheckpointRecoveryAction;
  plan_id: string;
  task_id?: string;
  error?: string;
  friendly_error?: string;
  recoverable?: boolean;
  next_action?: "retry_failed" | "create_task" | "partial" | "inspect_dependencies" | string;
  result?: PlannerCheckpointResumePlanResponse["result"];
  events?: PlannerCheckpointResumePlanJobEvent[];
  started_at: string;
  finished_at?: string;
}

export interface PlannerCheckpointResumePlanJobResponse {
  job: PlannerCheckpointResumePlanJob;
}

export interface PlannerExecutionStateFailureSummary {
  failed_count: number;
  completed_count: number;
  failed_tools?: string[];
  tried?: string[];
  ruled_out?: string[];
  next_step?: string;
}

export interface PlannerExecutionStateCogniSummary {
  activated?: string[];
  context_bytes?: number;
  tool_before?: number;
  tool_after?: number;
  removed?: string[];
  last_summary?: string;
  event_count: number;
  events?: PlannerCheckpointResumePlanJobEvent[];
}

export interface PlannerExecutionStateResponse {
  plan_id: string;
  status: string;
  action: PlannerCheckpointRecoveryAction;
  next_action?: "retry_failed" | "create_task" | "partial" | "inspect_dependencies" | string;
  updated_at?: string;
  checkpoint?: PlannerCheckpointSummary;
  latest_job?: PlannerCheckpointResumePlanJob;
  recovery_plan?: PlannerCheckpointRecoveryPlan;
  failure_summary?: PlannerExecutionStateFailureSummary;
  cogni?: PlannerExecutionStateCogniSummary;
  events?: PlannerCheckpointResumePlanJobEvent[];
}

// --- Tools execution + Audit + Backup ---

export interface ToolResult {
  output: string;
  exit_code: number;
  state: string;
  session_id?: string;
}

export interface ToolSession {
  id: string;
  command: string;
  state: string;
  exit_code: number;
  started_at: string;
  ended_at?: string;
}

export interface AuditRecord {
  seq: number;
  timestamp: string;
  type?: string;
  action: string;
  actor: string;
  detail: string;
  prev_hash?: string;
  hash: string;
}

export interface AuditStats {
  total: number;
  first_at?: string;
  last_at?: string;
  actors: Record<string, number>;
  in_memory?: number;
  max_size?: number;
  last_hash?: string;
  type_counts?: Record<string, number>;
  has_file?: boolean;
}

export interface BackupInfo {
  files: Record<string, number>;
  file_count: number;
  total_bytes: number;
  version: string;
}

export interface BackupRestoreResult {
  status: string;
  files_restored: number;
  from_version: string;
  size_bytes: number;
  warning?: string;
}
