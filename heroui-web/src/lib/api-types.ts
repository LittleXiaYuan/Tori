// ══════════════════════════════════════════════════════════════════════════
// Yunque Agent API Type Definitions
// ══════════════════════════════════════════════════════════════════════════

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

export interface SkillInfo {
  name: string;
  description: string;
  parameters: Record<string, unknown>;
  category?: string;
  usage_total?: number;
  success_rate?: number;
}

export interface SkillCategory {
  id: string;
  name: string;
  description: string;
}

export interface DynamicSkillDef {
  name: string;
  description: string;
  parameters: Record<string, unknown>;
  instruction: string;
  composed_of: string[];
  source: string;
  approval_status: string; // "draft" | "approved"
}

export interface PersonaMemoryBlock {
  id: string;
  content: string;
  label: string;
  max_chars: number;
  read_only: boolean;
  created_at: string;
  updated_at: string;
  version: number;
}

export interface PersonaMemoryEditRequest {
  id: string;
  label: string;
  content: string; // empty means delete
}

export interface PluginInfo {
  name: string;
  description: string;
  enabled: boolean;
  skill_count: number;
}

export interface TenantInfo {
  id: string;
  name: string;
  api_key: string;
  created_at: string;
}

export interface EmotionResult {
  emotion: string;
  confidence: number;
  source: string;
}

export interface StickerSuggestion {
  package_id: string;
  sticker_id: string;
  platform: string;
  emotion: string;
  file_id?: string;
  set_name?: string;
  cdnurl?: string;
  emoji?: string;
}

export interface ChatResponse {
  reply: string;
  skills_used: string[];
  steps: number;
  emotion?: EmotionResult;
  sticker_suggestion?: StickerSuggestion;
  sticker_suggestions?: Record<string, StickerSuggestion>;
  plan?: Array<{
    id: number;
    action: string;
    skill: string;
    status: string;
    result?: string;
    error?: string;
  }>;
}

// --- Persona / Presets ---

export interface PersonaSkill {
  name: string;
  description: string;
  content: string;
  enabled: boolean;
}

export interface PresetInfo {
  id: string;
  name: string;
  description: string;
  tone: string;
  style: string;
  greeting: string;
  system_note: string;
  features?: Record<string, boolean>;
}

export interface EmotionHistoryEntry {
  timestamp: string;
  session_id: string;
  emotion: string;
  confidence: number;
  source: string;
  trigger?: string;
  created_at?: string;
}

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

export interface PluginMeta {
  name: string;
  description: string;
  enabled: boolean;
  skill_count: number;
  source: "builtin" | "script";
  language?: string;
}

export interface PluginFile {
  name: string;
  content: string;
  size: number;
}

// --- Knowledge ---

export interface KBChunk {
  id: string;
  source_id: string;
  content: string;
  index: number;
  metadata?: Record<string, string>;
}

export interface KBSource {
  id: string;
  name: string;
  type: string;
  path?: string;
  trigger?: string;
  chunk_count: number;
  added_at: string;
}

export interface KBImportTreeNode {
  title: string;
  url?: string;
  path?: string;
  children?: KBImportTreeNode[];
}

export interface KBStats {
  sources: number;
  chunks: number;
  total_chars: number;
}

// --- Cron ---

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

// --- Tools ---

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

// --- Audit ---

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

// --- Reverie ---

export interface ReverieAction {
  type: string;    // "write_memory" | "create_task" | "update_profile"
  key: string;
  value: string;
}

export interface ActionRecord {
  thought_id: string;
  action: ReverieAction;
  success: boolean;
  error?: string;
  at: string;
}

export interface ReverieThought {
  id: string;
  content: string;
  category: string;
  significance: number;
  trigger: string;
  timestamp: string;
  delivered: boolean;
  delivered_at?: string;
  actions?: ReverieAction[];
}

export interface ReverieStats {
  total_thoughts: number;
  delivered: number;
  avg_significance: number;
  categories: Record<string, number>;
  last_thought_at?: string;
  uptime_seconds: number;
}

export interface ReverieConfig {
  enabled: boolean;
  interval_minutes: number;
  min_significance: number;
  quiet_start: number;
  quiet_end: number;
}

// --- Models ---

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

// --- Settings ---

export interface ConfigField {
  key: string;
  label: string;
  label_zh: string;
  type: "text" | "password" | "select" | "number";
  placeholder?: string;
  options?: string[];
  sensitive?: boolean;
  required?: boolean;
  hint?: string;
}

export interface ConfigGroup {
  key: string;
  label: string;
  label_zh: string;
  fields: ConfigField[];
}

export interface SetupCheck {
  env_exists: boolean;
  has_llm_key: boolean;
  has_llm_url: boolean;
  has_llm_model?: boolean;
  api_ok: boolean;
  setup_needed: boolean;
}

// --- SkillHub ---

export interface SkillHubItem {
  name: string;
  description: string;
  version: string;
  author: string;
  rating: number;
  source: string; // "local" | "clawhub"
  installed: boolean;
}

export interface SkillHubInstalledItem {
  slug: string;
  name: string;
  version: string;
  description: string;
  source: string;
  security_score: number;
  installed_at: string;
  updated_at: string;
  enabled: boolean;
}

export interface AuditFinding {
  layer: string;
  severity: number;
  rule: string;
  detail: string;
}

export interface AuditReport {
  slug: string;
  score: number;
  passed: boolean;
  auto_approve: boolean;
  findings: AuditFinding[];
  static_score: number;
  perm_score: number;
  sandbox_score: number;
}

export interface SkillHubDetail {
  slug: string;
  name: string;
  description: string;
  version: string;
  author: string;
  rating: number;
  rating_count: number;
  installs: number;
  category: string;
  tags: string[];
  license: string;
  installed: boolean;
  source: string;
  permissions?: string[];
  security_score: number;
  audit_report?: AuditReport;
  content?: string;
  installed_at?: string;
  updated_at?: string;
}

export interface SkillUpdateInfo {
  slug: string;
  name: string;
  current_version: string;
  latest_version: string;
  has_update: boolean;
}

export interface SkillVersionInfo {
  version: string;
  installed_at?: string;
  current: boolean;
}

export interface SkillPolicy {
  min_score: number;
  trusted_authors: string[];
  blocked_authors: string[];
  allowed_slugs: string[];
  blocked_slugs: string[];
  max_perm_level: string;
  require_audit: boolean;
  auto_approve_min: number;
}

export interface PolicyCheckResult {
  allowed: boolean;
  reason?: string;
  auto_approve?: boolean;
}

export interface MarketAnalyticsSkill {
  slug: string;
  name: string;
  author: string;
  version: string;
  installs: number;
  rating: number;
  security_score: number;
  enabled: boolean;
}

export interface MarketAnalytics {
  total_skills: number;
  installed_count: number;
  total_installs: number;
  avg_score: number;
  categories: Record<string, number>;
  top_installed: MarketAnalyticsSkill[];
  top_rated: MarketAnalyticsSkill[];
  security_stats: Record<string, number>;
}

// --- Backup ---

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

// --- Task Runtime ---

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

// --- State Kernel ---

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

export interface ExperienceItem {
  id: string;
  source: string;
  source_id: string;
  category: string;
  outcome: string;
  lesson: string;
  context: string;
  tags?: string[];
  created_at: string;
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

export interface ExperienceStats {
  total: number;
  by_source: Record<string, number>;
  by_category: Record<string, number>;
  by_outcome: Record<string, number>;
  recent_7d: number;
}

// --- Plugin UI ---

export interface PluginUITab {
  key: string;
  label: string;
  label_en?: string;
  icon: string;
  description?: string;
  plugin: string;
}

// --- QQ Chat Analyzer ---

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

// --- Task Thread ---

export interface LLMMessage {
  role: string;
  content: string;
  created_at?: string;
}

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

// --- Task Working Memory ---

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

// --- Cost ---

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

// --- Triggers ---

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

// --- Approvals / Setup / Queue ---

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

export interface SetupProviderStatus {
  name: string;
  base_url: string;
  model: string;
  available: boolean;
  latency: string;
  error?: string;
}

export interface SetupComponent {
  id: string;
  name: string;
  description: string;
  category: string;
  installed: boolean;
  version?: string;
  size?: string;
  installable: boolean;
  required?: boolean;
}

export interface SetupEnvironment {
  os: string;
  arch: string;
  num_cpu: number;
  has_docker: boolean;
  has_gpu: boolean;
  gpu_info?: string;
  has_ollama: boolean;
  ollama_models?: string[];
  has_python: boolean;
  python_version?: string;
  has_node: boolean;
  node_version?: string;
  providers: SetupProviderStatus[];
  data_dir: string;
  config_exists: boolean;
  first_run: boolean;
  components: SetupComponent[];
}

export interface SetupHealthResult {
  providers: SetupProviderStatus[];
  has_docker: boolean;
  has_gpu: boolean;
  has_ollama: boolean;
}

export interface SetupTemplate {
  id: string;
  name: string;
  description: string;
  category: string;
  env_vars: Record<string, string>;
  skills: string[];
  channels: string[];
  sandbox_tier: string;
}

export interface SetupTemplatesResponse {
  templates: SetupTemplate[];
  count: number;
}

export interface SetupApplyResponse {
  ok: boolean;
  status: string;
  applied: string;
  persisted: boolean;
  env_content: string;
  restart_required: boolean;
  message: string;
  template?: SetupTemplate;
}

export interface SetupTestProviderResult {
  ok: boolean;
  provider: SetupProviderStatus;
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

// --- Trust ---

export interface TrustEntry {
  score: number;
  executions: number;
  failures: number;
  last_promoted?: string;
}

// --- RBAC ---

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

// --- Iterate ---

export interface IterateProposal {
  id: string;
  type: string;
  title: string;
  description: string;
  status: "pending" | "approved" | "rejected" | "applied";
  created_at: string;
}

export interface IterateStatus {
  enabled: boolean;
  running: boolean;
  last_run?: string;
  proposals_pending: number;
  token_budget: number;
  tokens_used: number;
}

// --- SkillGrow ---

export interface SkillGrowPattern {
  pattern: string;
  count: number;
  suggestion: string;
  first_seen: string;
  last_seen: string;
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

// --- Provider / Browser / OPP ---

export interface ProviderInfo {
  id: string;
  display_name?: string;
  type: string;
  model: string;
  base_url: string;
  enabled: boolean;
  tier?: string;
  priority: number;
  capabilities?: string[];
  key_count: number;
  breaker_state: string;
}

export interface ProviderTestResult {
  status: string;
  latency_ms: number;
  model?: string;
  error?: string;
}

export interface BrowserStatus {
  connected: boolean;
  current_url?: string;
  page_title?: string;
  session_id?: string;
}

export interface BrowserConfig {
  headless: boolean;
  viewport: { width: number; height: number };
  proxy?: string;
}

export interface OPPItem {
  id: string;
  action: string;
  url?: string;
  detail?: string;
  risk_level: string;
  created_at: string;
}

// --- Browser Extension Scenarios ---

export interface BrowserScenario {
  id: string;
  name: string;
  description: string;
  icon: string;
  steps: Record<string, unknown>[];
}

export interface ScenarioStepResult {
  step: number;
  action: string;
  ok: boolean;
  error?: string;
  has_screenshot?: boolean;
}

// --- Cost Summary ---

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

// --- Graph ---

export interface GraphEntity {
  id: string;
  name: string;
  type: string;
  properties?: Record<string, string>;
  mentions?: number;
  created_at: string;
}

export interface GraphRelation {
  id: string;
  from_id: string;
  to_id: string;
  type: string;
  weight?: number;
  context?: string;
  created_at?: string;
}

export interface GraphStats {
  entities: number;
  relations: number;
  entity_types: Record<string, number>;
  relation_types: Record<string, number>;
}

// --- Memory ---

export interface MemorySearchResult {
  id: string;
  content: string;
  score: number;
  metadata?: Record<string, string>;
  created_at: string;
}

// --- System ---

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

// --- Persona Modes ---

export interface PersonaMode {
  id: string;
  name: string;
  description: string;
  active: boolean;
}

// --- Search ---

export interface SearchResult {
  id: string;
  type: string;
  title: string;
  content: string;
  score: number;
  source: string;
}

// --- Federation ---

export interface FederationPeer {
  id: string;
  name: string;
  address: string;
  status: "connected" | "disconnected" | "pending";
  last_seen: string;
}

export interface FederationStats {
  peers: number;
  connected: number;
  messages_sent: number;
  messages_received: number;
}

// --- Provider Presets ---

export interface ProviderPreset {
  id: string;
  name: string;
  base_url: string;
  type: string;
  description?: string;
  docs_url?: string;
  is_aggregator?: boolean;
  models: Array<{
    id: string;
    name: string;
    type: string;
    tier?: string;
    capabilities?: string[];
    context_window?: number;
  }>;
}

// --- Tori ---

export interface ToriBindingStatus {
  bound: boolean;
  username?: string;
  tori_url?: string;
  api_key?: string;
  expires_at?: string;
}

export interface ToriHealthStatus {
  status: string;
  version?: string;
  uptime?: number;
  db?: boolean;
  error?: string;
}

export interface ToriUsageSummary {
  user_id?: number;
  remain_quota?: number;
  used_quota?: number;
  request_count?: number;
  prompt_tokens?: number;
  completion_tokens?: number;
  total_tokens?: number;
  error?: string;
}

export interface SkillSuggestion {
  name: string;
  description: string;
  trigger: string;
  confidence: number;
}

export interface SyncManifestItem {
  key: string;
  version: number;
  size: number;
  updated_at: string;
}

// --- Connectors ---

export interface ConnectorView {
  id: string;
  name: string;
  description: string;
  icon: string;
  category: string;
  auth_type: string;
  beta?: boolean;
  supported: boolean;
  status: "disconnected" | "connecting" | "connected" | "error";
  user_info?: string;
  error?: string;
  action_count: number;
}

export interface ConnectorDef {
  id: string;
  name: string;
  description: string;
  icon: string;
  category: string;
  auth_type: string;
  beta?: boolean;
  supported?: boolean;
  scopes?: string[];
  actions: ConnectorAction[];
}

export interface ConnectorAction {
  id: string;
  name: string;
  description: string;
  parameters?: ConnectorParam[];
}

export interface ConnectorParam {
  name: string;
  type: string;
  description: string;
  required?: boolean;
}

export interface NotifyChannel {
  id: string;
  type: string; // "webhook" | "dingtalk" | "feishu" | "wechat_work"
  name: string;
  url: string;
  secret?: string;
  enabled: boolean;
}

export interface WorkerInfo {
  id: string;
  name: string;
  type: string; // "cursor" | "claude_code" | "windsurf" | "custom"
  capabilities: string[];
  max_concurrency: number;
  active_tasks: number;
  status: string; // "online" | "busy" | "offline"
  registered_at: string;
  last_heartbeat: string;
  metadata?: Record<string, string>;
}
