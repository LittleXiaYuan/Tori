// ══════════════════════════════════════════════════════════════════════════
// External Integrations / Setup Types
// ══════════════════════════════════════════════════════════════════════════
// Anything that talks to the outside world: setup wizard + environment
// detection, LLM provider registry / presets / connectivity tests, browser
// connector status, OPP (operator-permission-prompt) approvals, browser
// extension scenarios, federation peers, Tori binding/health/usage,
// connectors (third-party action providers), notify channels, sync
// manifest, and the orchestrator's worker / project registries.

// --- Settings / Setup ---

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
  tier?: "common" | "advanced" | "expert"; // visibility level; empty treated as advanced
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

// --- Provider / Browser / OPP ---

export interface ProviderInfo {
  id: string;
  display_name?: string;
  type: string;
  source?: string;
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

// --- Tori (云鸢中央) ---

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

// --- Sync manifest ---

export interface SyncManifestItem {
  key: string;
  version: number;
  size: number;
  updated_at: string;
}

// --- Connectors (third-party action providers) ---

export interface ConnectorParam {
  name: string;
  type: string;
  description: string;
  required?: boolean;
}

export interface ConnectorAction {
  id: string;
  name: string;
  description: string;
  parameters?: ConnectorParam[];
}

export interface ConnectorEvent {
  kind: string;
  connector_id: string;
  action_id?: string;
  status: string;
  message?: string;
  at: string;
}

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
  allowlist_count?: number;
  allowed_actions?: string[];
  last_event?: ConnectorEvent;
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

export interface NotifyChannel {
  id: string;
  type: string; // "webhook" | "dingtalk" | "feishu" | "wechat_work"
  name: string;
  url: string;
  secret?: string;
  enabled: boolean;
}

export interface NotifyShareFile {
  name: string;
  path: string;
  size?: number;
}

export interface NotifyShareRequest {
  channel_id: string;
  title: string;
  message?: string;
  session_id?: string;
  task_id?: string;
  url?: string;
  files?: NotifyShareFile[];
}

export interface NotifyShareResponse {
  ok: boolean;
  sent_at: string;
  share?: {
    code: string;
    session_id: string;
    created_at: string;
  };
  channel: {
    id: string;
    type: string;
    name: string;
  };
}

// --- Orchestrator: workers + projects ---

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

export interface ProjectInfo {
  id: string;
  name: string;
  repo_path: string;
  repo_url?: string;
  description?: string;
  default_caps?: string[];
  meta?: Record<string, string>;
  created_at: string;
  updated_at: string;
}
