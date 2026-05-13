import type {
  VersionInfo, MetricsSnapshot, SkillInfo, DynamicSkillDef,
  PersonaMemoryBlock, PersonaMemoryEditRequest, PluginInfo, TenantInfo,
  ChatResponse, DocTemplate, PersonaSkill, PresetInfo,
  EmotionHistoryEntry, InboxItem, InboxResponse, HeartbeatLog, BotsResponse, BotInfo,
  PluginMeta, PluginFile, KBChunk, KBSource, KBImportTreeNode, KBStats,
  CronJob, CronRun, ToolResult, ToolSession, AuditRecord, AuditReport, AuditStats,
  ReverieThought, ReverieStats, ReverieConfig, ActionRecord,
  ModelInfo, ConfigGroup, SetupCheck,
  SkillHubItem, SkillHubInstalledItem, SkillHubDetail, SkillUpdateInfo,
  SkillVersionInfo, SkillPolicy, PolicyCheckResult, MarketAnalytics,
  BackupInfo, BackupRestoreResult, ConversationInfo,
  TaskInfo, GapRecord, GapStats, StateGoal,
  StateSnapshot, ExperienceItem, ExperienceOutcome, ExperienceStats, TaskTemplate,
  PluginUITab, PackListResponse, PackMutationResponse, PackBackendModulesResponse, QQAnalysis, LLMMessage, ThreadState, TaskThreadInfo, TaskWorkingMemory,
  CostTaskSummary, CostUsageEvent, CostBreakdown,
  TriggerItem, TriggerDef, TriggerRun, TriggerLogEvent, TriggerEventPayload,
  ApprovalRequest, ApprovalRule,
  SetupEnvironment, SetupHealthResult, SetupTemplate, SetupTemplatesResponse, SetupApplyResponse, SetupTestProviderResult, SessionQueueInfo,
  TrustEntry, RBACRole,
  IterateProposal, IterateStatus, SkillGrowPattern,
  WorkflowDef, WorkflowInstance,
  ProviderInfo, ProviderTestResult, BrowserStatus, BrowserConfig, OPPItem, BrowserScenario, ScenarioStepResult,
  CostSummary, CostHistoryEntry, CostAlert, CostBudget,
  GraphEntity, GraphRelation, GraphStats, MemorySearchResult,
  SystemInfo, CacheStats, RouterStats, PersonaMode, SearchResult,
  FederationPeer, FederationStats, ProviderPreset, ToriBindingStatus, ToriHealthStatus, ToriUsageSummary, SkillSuggestion, SyncManifestItem,
  ConnectorView, ConnectorDef, NotifyChannel, NotifyShareRequest, NotifyShareResponse, WorkerInfo, ProjectInfo,
  PlannerCheckpointListResponse, PlannerCheckpointRecoverResponse, PlannerCheckpointResumeTaskResponse, PlannerCheckpointResumePlanResponse, PlannerCheckpointResumePlanJobResponse, PlannerCheckpointRecoveryAction, PlannerExecutionStateResponse,
  FilePreviewResponse, FileUploadResponse,
} from "./api-types";

export type * from "./api-types";

// Core utilities are defined in api-core.ts and re-exported here
// for backward compatibility. New domain-specific API files should
// import directly from "./api-core".
export { setApiKey, getApiKey, getAuthHeaders, BASE } from "./api-core";
import { fetcher, getAuthHeaders, getApiKey, BASE } from "./api-core";
import { legacyChatStreamChunks, parseAgenticChatStream } from "./chat-sse";

export const DEFAULT_CHAT_STREAM_IDLE_TIMEOUT_MS = 180_000;

export const api = {
  healthz: () => fetcher<{ status: string; version: string }>("/healthz"),
  version: () => fetcher<VersionInfo>("/v1/version"),
  metrics: () => fetcher<MetricsSnapshot>("/v1/metrics"),

  skills: () => fetcher<{ skills: SkillInfo[]; count: number; categories?: Array<{ id: string; name: string; description: string }> }>("/v1/skills"),
  getDynamicSkills: () => fetcher<{ skills: DynamicSkillDef[] }>("/v1/skills/dynamic").then((r) => Array.isArray(r.skills) ? r.skills : []),
  approveDynamicSkill: (name: string, instruction?: string) =>
    fetcher<{ status: string }>("/v1/skills/approve", { method: "POST", body: JSON.stringify({ name, instruction }) }),
  rejectDynamicSkill: (name: string) =>
    fetcher<{ status: string }>("/v1/skills/reject", { method: "POST", body: JSON.stringify({ name }) }),

  getMemoryPersona: () => fetcher<{ blocks: PersonaMemoryBlock[] }>("/v1/memory/persona").then((r) => Array.isArray(r.blocks) ? r.blocks : []),
  updateMemoryPersona: (req: PersonaMemoryEditRequest) =>
    fetcher<{ success: boolean; error?: string }>("/v1/memory/update", { method: "POST", body: JSON.stringify(req) }),

  tenants: () => fetcher<{ tenants: TenantInfo[]; count: number }>("/v1/tenants"),
  createTenant: (name: string) =>
    fetcher<TenantInfo>("/v1/tenants", {
      method: "POST",
      body: JSON.stringify({ name }),
    }),

  chat: (messages: Array<{ role: string; content: string }>, sessionId?: string, thinkingLevel?: string) =>
    fetcher<ChatResponse>("/v1/chat", {
      method: "POST",
      body: JSON.stringify({ messages, session_id: sessionId, ...(thinkingLevel ? { thinking_level: thinkingLevel } : {}) }),
    }),

  plannerCheckpoints: (params?: { limit?: number; includeSnapshot?: boolean; planId?: string }) => {
    const q = new URLSearchParams();
    q.set("limit", String(params?.limit ?? 20));
    if (params?.includeSnapshot) q.set("include_snapshot", "1");
    if (params?.planId) q.set("plan_id", params.planId);
    return fetcher<PlannerCheckpointListResponse>(`/v1/planner/checkpoints?${q.toString()}`);
  },
  plannerCheckpointRecover: (planId: string, action: PlannerCheckpointRecoveryAction) =>
    fetcher<PlannerCheckpointRecoverResponse>("/v1/planner/checkpoints/recover", {
      method: "POST",
      body: JSON.stringify({ plan_id: planId, action }),
    }),
  plannerCheckpointResumeTask: (planId: string, action: PlannerCheckpointRecoveryAction, options?: { run?: boolean }) =>
    fetcher<PlannerCheckpointResumeTaskResponse>("/v1/planner/checkpoints/resume", {
      method: "POST",
      body: JSON.stringify({ plan_id: planId, action, ...(options?.run === undefined ? {} : { run: options.run }) }),
    }),
  plannerCheckpointResumePlan: (planId: string, action: PlannerCheckpointRecoveryAction, options?: { async?: boolean }) =>
    fetcher<PlannerCheckpointResumePlanResponse>("/v1/planner/checkpoints/resume-plan", {
      method: "POST",
      body: JSON.stringify({ plan_id: planId, action, ...(options?.async === undefined ? {} : { async: options.async }) }),
    }),
  plannerCheckpointResumePlanJob: (jobIdOrParams: string | { jobId?: string; planId?: string }) => {
    const params = typeof jobIdOrParams === "string" ? { jobId: jobIdOrParams } : jobIdOrParams;
    const q = new URLSearchParams();
    if (params.jobId) q.set("id", params.jobId);
    if (params.planId) q.set("plan_id", params.planId);
    return fetcher<PlannerCheckpointResumePlanJobResponse>(`/v1/planner/checkpoints/resume-plan/jobs?${q.toString()}`);
  },
  plannerExecutionState: (planId: string, action?: PlannerCheckpointRecoveryAction) => {
    const q = new URLSearchParams();
    q.set("plan_id", planId);
    if (action) q.set("action", action);
    return fetcher<PlannerExecutionStateResponse>(`/v1/planner/execution-state?${q.toString()}`);
  },

  chatStream: async function* (
    messages: Array<{ role: string; content: string }>,
    sessionId?: string,
    thinkingLevel?: string,
    signal?: AbortSignal,
    options?: {
      /** Cherry 🌐 drawer: ask planner to prefer the web_search skill. */
      webSearch?: boolean;
      /** Cherry 🔨 drawer: restrict planner to these skill names. */
      toolIds?: string[];
      /** Cherry 📎 drawer: inline files to splice into the last user message. */
      attachments?: Array<{ name: string; mime: string; dataB64: string }>;
      /** Stop waiting if the stream goes quiet for too long. */
      idleTimeoutMs?: number;
    },
  ): AsyncGenerator<string> {
    const body: Record<string, unknown> = { messages, session_id: sessionId };
    if (thinkingLevel) body.thinking_level = thinkingLevel;
    if (options?.webSearch) body.web_search = true;
    if (options?.toolIds && options.toolIds.length > 0) body.tool_ids = options.toolIds;
    if (options?.attachments && options.attachments.length > 0) {
      body.attachments = options.attachments.map((a) => ({
        name: a.name,
        mime: a.mime,
        data_b64: a.dataB64,
      }));
    }
    const res = await fetch(`${BASE}/v1/chat/agentic`, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        ...getAuthHeaders(),
      },
      body: JSON.stringify(body),
      signal,
    });
    if (!res.ok) throw new Error(`${res.status}`);
    if (!res.body) return;
    for await (const item of parseAgenticChatStream(res.body, { idleTimeoutMs: options?.idleTimeoutMs ?? DEFAULT_CHAT_STREAM_IDLE_TIMEOUT_MS })) {
      if (item.kind === "error") throw new Error(item.message);
      for (const chunk of legacyChatStreamChunks(item)) yield chunk;
    }
  },

  memoryStats: () => fetcher<{ short: number; mid: number; long: number }>("/v1/memory/stats"),
  systemStats: () => fetcher<Record<string, unknown>>("/v1/system/stats"),

  // Persona
  getPersona: () => fetcher<{ identity: string; soul: string }>("/v1/persona"),
  updatePersona: (identity: string, soul: string) =>
    fetcher<{ status: string }>("/v1/persona", { method: "PUT", body: JSON.stringify({ identity, soul }) }),
  getPersonaSkills: () => fetcher<{ skills: PersonaSkill[] }>("/v1/persona/skills"),
  addPersonaSkill: (name: string, description: string, content: string) =>
    fetcher<{ status: string }>("/v1/persona/skills", { method: "POST", body: JSON.stringify({ name, description, content }) }),
  deletePersonaSkill: (name: string) =>
    fetcher<{ status: string }>("/v1/persona/skills", { method: "DELETE", body: JSON.stringify({ name }) }),

  // Persona presets
  getPresets: () => fetcher<{ presets: PresetInfo[]; active: string }>("/v1/persona/presets"),
  switchPreset: (id: string) =>
    fetcher<{ status: string; active: string }>("/v1/persona/presets", { method: "POST", body: JSON.stringify({ id }) }),
  updatePresetFeatures: (id: string, features: Record<string, boolean>) =>
    fetcher<{ status: string }>("/v1/persona/presets/features", { method: "PUT", body: JSON.stringify({ id, features }) }),
  createCustomPreset: (preset: Omit<PresetInfo, "features"> & { features?: Record<string, boolean> }) =>
    fetcher<{ status: string; id: string }>("/v1/persona/presets/custom", { method: "POST", body: JSON.stringify(preset) }),
  deleteCustomPreset: (id: string) =>
    fetcher<{ status: string }>("/v1/persona/presets/custom", { method: "DELETE", body: JSON.stringify({ id }) }),

  // Sticker map management
  getStickerMap: () =>
    fetcher<Record<string, Record<string, { package_id: string; sticker_id: string; platform: string; emotion: string }[]>>>("/v1/emotion/stickers"),
  updateStickerMapping: (platform: string, emotion: string, stickers: { package_id: string; sticker_id: string }[]) =>
    fetcher<{ status: string }>("/v1/emotion/stickers", { method: "PUT", body: JSON.stringify({ platform, emotion, stickers }) }),
  deleteStickerMapping: (platform: string, emotion: string) =>
    fetcher<{ status: string }>("/v1/emotion/stickers", { method: "DELETE", body: JSON.stringify({ platform, emotion }) }),

  // Emotion history
  getEmotionHistory: (params?: { session_id?: string; from?: string; to?: string; limit?: number }) => {
    const q = new URLSearchParams();
    if (params?.session_id) q.set("session_id", params.session_id);
    if (params?.from) q.set("from", params.from);
    if (params?.to) q.set("to", params.to);
    if (params?.limit) q.set("limit", String(params.limit));
    return fetcher<{ entries: EmotionHistoryEntry[]; summary: Record<string, number>; total: number }>(`/v1/emotion/history?${q.toString()}`);
  },

  // Inbox
  getInbox: (unread = false) => fetcher<InboxResponse>(`/v1/inbox?unread=${unread}`),
  pushInbox: (source: string, content: string, action: string) =>
    fetcher<InboxItem>("/v1/inbox", { method: "POST", body: JSON.stringify({ source, content, action }) }),
  markInboxRead: (ids: string[]) =>
    fetcher<{ marked: number }>("/v1/inbox/read", { method: "POST", body: JSON.stringify({ ids }) }),
  markAllInboxRead: () =>
    fetcher<{ marked: number }>("/v1/inbox/read", { method: "POST", body: JSON.stringify({ all: true }) }),

  // Heartbeat
  getHeartbeat: () => fetcher<{ running: boolean }>("/v1/heartbeat"),
  updateHeartbeat: (enabled?: boolean, interval_minutes?: number) =>
    fetcher<{ status: string }>("/v1/heartbeat", { method: "PUT", body: JSON.stringify({ enabled, interval_minutes }) }),
  triggerHeartbeat: () => fetcher<HeartbeatLog>("/v1/heartbeat/trigger", { method: "POST" }),
  getHeartbeatLogs: (limit = 20) => fetcher<HeartbeatLog[]>(`/v1/heartbeat/logs?limit=${limit}`),

  // Agent output files
  listFiles: (path?: string) =>
    fetcher<{ files: Array<{ name: string; path: string; size: number; is_dir: boolean }> }>(`/api/files${path ? `?path=${encodeURIComponent(path)}` : ""}`),
  fileDownloadUrl: (path: string) => `/api/files/download?path=${encodeURIComponent(path)}`,
  previewFile: (path: string) =>
    fetcher<FilePreviewResponse>(`/api/files/preview?path=${encodeURIComponent(path)}`),

  // File upload (saves to agent workspace)
  uploadFile: async (file: File): Promise<FileUploadResponse> => {
    const form = new FormData();
    form.append("file", file);
    const res = await fetch(`${BASE}/v1/upload`, {
      method: "POST",
      headers: { ...getAuthHeaders() },
      body: form,
    });
    if (!res.ok) throw new Error(`upload failed: ${res.status}`);
    return res.json();
  },

  // Models
  getModels: () => fetcher<{ models: ModelInfo[] }>("/v1/models"),
  addModel: (model: Partial<ModelInfo>) =>
    fetcher<ModelInfo>("/v1/models", { method: "POST", body: JSON.stringify(model) }),
  deleteModel: (id: string) =>
    fetcher<{ status: string }>(`/v1/models?id=${id}`, { method: "DELETE" }),

  // Plugins (script)
  getPlugins: () => fetcher<{ plugins: PluginMeta[] }>("/v1/plugins"),
  togglePlugin: (name: string, enabled: boolean) =>
    fetcher<{ name: string; enabled: boolean }>("/v1/plugins/toggle", { method: "POST", body: JSON.stringify({ name, enabled }) }),
  getPluginFiles: (name: string) =>
    fetcher<{ files: PluginFile[] }>(`/v1/plugins/files?name=${name}`),
  savePluginFile: (pluginName: string, fileName: string, content: string) =>
    fetcher<{ status: string }>("/v1/plugins/files", { method: "PUT", body: JSON.stringify({ plugin: pluginName, file: fileName, content }) }),
  createPlugin: (manifest: Record<string, unknown>) =>
    fetcher<{ status: string }>("/v1/plugins/create", { method: "POST", body: JSON.stringify(manifest) }),
  deletePlugin: (name: string) =>
    fetcher<{ status: string }>(`/v1/plugins/delete?name=${name}`, { method: "DELETE" }),
  reloadPlugins: () =>
    fetcher<{ status: string; skills: number }>("/v1/plugins/reload", { method: "POST" }),
  scanSkills: () =>
    fetcher<{ status: string; skills_loaded: number; total_skills: number }>("/v1/skills/scan", { method: "POST" }),
  openPluginFolder: (name?: string) =>
    fetcher<{ ok: boolean; path: string }>(`/v1/plugins/open-folder${name ? `?name=${name}` : ""}`),

  // Bots
  getBots: () => fetcher<BotsResponse>("/v1/bots"),
  createBot: (name: string, description: string) =>
    fetcher<BotInfo>("/v1/bots", { method: "POST", body: JSON.stringify({ name, description }) }),
  getBot: (id: string) => fetcher<BotInfo>(`/v1/bots/detail?id=${id}`),
  updateBot: (id: string, data: Partial<BotInfo>) =>
    fetcher<BotInfo>(`/v1/bots/detail?id=${id}`, { method: "PUT", body: JSON.stringify(data) }),
  deleteBot: (id: string) =>
    fetcher<{ status: string }>(`/v1/bots/detail?id=${id}`, { method: "DELETE" }),

  // Knowledge base
  kbSearch: (q: string, n = 10, filters?: { file?: string; lang?: string }) => {
    const params = new URLSearchParams({ q, n: String(n) });
    if (filters?.file) params.set("file", filters.file);
    if (filters?.lang) params.set("lang", filters.lang);
    return fetcher<{ chunks: KBChunk[]; count: number }>(`/v1/knowledge/search?${params.toString()}`);
  },
  kbSources: () => fetcher<{ sources: KBSource[] }>("/v1/knowledge/sources"),
  kbStats: () => fetcher<KBStats>("/v1/knowledge/stats"),
  kbIngest: (name: string, content: string, trigger?: string) =>
    fetcher<{ source: KBSource; stats: KBStats }>("/v1/knowledge/ingest", { method: "POST", body: JSON.stringify({ name, content, trigger }) }),
  kbUpdate: (id: string, name: string, trigger: string, content: string) =>
    fetcher<{ source: KBSource; stats: KBStats }>("/v1/knowledge/source/update", { method: "POST", body: JSON.stringify({ id, name, trigger, content }) }),
  kbImportURL: (url: string, name?: string, options?: { crawlChildren?: boolean; maxPages?: number }) =>
    fetcher<{ source: KBSource; sources?: KBSource[]; imported?: number; tree?: KBImportTreeNode; stats: KBStats }>("/v1/knowledge/import-url", {
      method: "POST",
      body: JSON.stringify({ url, name, crawl_children: options?.crawlChildren, max_pages: options?.maxPages }),
    }),
  kbImportRepo: (path: string, maxFiles?: number) =>
    fetcher<{ source: KBSource; stats: KBStats }>("/v1/knowledge/import-repo", {
      method: "POST",
      body: JSON.stringify({ path, max_files: maxFiles }),
    }),
  kbDelete: (id: string) =>
    fetcher<{ deleted: string; stats: KBStats }>(`/v1/knowledge/source?id=${id}`, { method: "DELETE" }),
  kbUpload: async (file: File) => {
    const key = getApiKey();
    const form = new FormData();
    form.append("file", file);
    const res = await fetch(`${BASE}/v1/knowledge/upload`, {
      method: "POST",
      headers: { ...(key ? { "X-API-Key": key } : {}) },
      body: form,
    });
    if (!res.ok) throw new Error(`${res.status}`);
    return res.json() as Promise<{ source: KBSource; stats: KBStats }>;
  },

  // Cron jobs
  cronList: () => fetcher<{ jobs: CronJob[] }>("/v1/cron/list"),
  cronAdd: (name: string, schedule: Record<string, unknown>, payload: Record<string, unknown>) =>
    fetcher<{ job: CronJob }>("/v1/cron/add", { method: "POST", body: JSON.stringify({ name, schedule, payload }) }),
  cronRemove: (id: string) => fetcher<{ deleted: string }>(`/v1/cron/remove?id=${id}`, { method: "DELETE" }),
  cronRun: (id: string) => fetcher<{ run: CronRun }>(`/v1/cron/run?id=${id}`, { method: "POST" }),

  // Tools
  toolExec: (command: string, cwd?: string, background?: boolean) =>
    fetcher<ToolResult>("/v1/tools/exec", { method: "POST", body: JSON.stringify({ command, cwd, background }) }),
  toolList: () => fetcher<{ sessions: ToolSession[] }>("/v1/tools/list"),
  toolPoll: (id: string) => fetcher<{ lines: string[]; state: string }>(`/v1/tools/poll?id=${id}`),
  toolKill: (id: string) => fetcher<{ killed: string }>(`/v1/tools/kill?id=${id}`, { method: "POST" }),

  // Audit
  auditTail: (n = 20) => fetcher<{ records: AuditRecord[] }>(`/v1/audit/tail?n=${n}`),
  auditVerify: () => fetcher<{ valid: boolean; checked: number; broken_at?: number }>("/v1/audit/verify"),
  auditStats: () => fetcher<AuditStats>("/v1/audit/stats"),

  // SkillHub marketplace (supports source filter: "clawhub" | "torihub" | "" for all)
  skillHubSearch: (q: string, limit = 20, source = "") =>
    fetcher<{ results: SkillHubItem[]; count: number }>(`/api/skillhub/search?q=${encodeURIComponent(q)}&limit=${limit}${source ? `&source=${source}` : ""}`),
  skillHubInstall: (slug: string) =>
    fetcher<{ status: string; slug: string; report: unknown }>("/api/skillhub/install", { method: "POST", body: JSON.stringify({ slug }) }),
  skillHubInstalled: () =>
    fetcher<{ skills: SkillHubInstalledItem[]; count: number }>("/api/skillhub/installed"),
  skillHubUninstall: (slug: string) =>
    fetcher<{ status: string; slug: string }>("/api/skillhub/uninstall", { method: "POST", body: JSON.stringify({ slug }) }),
  skillHubTrending: (limit = 20, source = "", cursor = "") =>
    fetcher<{ skills: SkillHubItem[]; count: number; next_cursor?: string }>(
      `/api/skillhub/trending?limit=${limit}${source ? `&source=${source}` : ""}${cursor ? `&cursor=${cursor}` : ""}`
    ),
  skillHubDetail: (slug: string) =>
    fetcher<SkillHubDetail>(`/api/skillhub/detail?slug=${encodeURIComponent(slug)}`),
  skillHubCheckUpdates: () =>
    fetcher<{ updates: SkillUpdateInfo[] }>("/api/skillhub/check-updates"),
  skillHubUpdate: (slug: string) =>
    fetcher<{ ok: boolean; report?: AuditReport }>("/api/skillhub/update", {
      method: "POST", headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ slug }),
    }),
  skillHubRollback: (slug: string, version: string) =>
    fetcher<{ ok: boolean }>("/api/skillhub/rollback", {
      method: "POST", headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ slug, version }),
    }),
  skillHubVersions: (slug: string) =>
    fetcher<{ versions: SkillVersionInfo[] }>(`/api/skillhub/versions?slug=${encodeURIComponent(slug)}`),
  skillHubGetPolicy: () =>
    fetcher<SkillPolicy>("/api/skillhub/policy"),
  skillHubSetPolicy: (policy: SkillPolicy) =>
    fetcher<{ ok: boolean }>("/api/skillhub/policy", {
      method: "POST", headers: { "Content-Type": "application/json" },
      body: JSON.stringify(policy),
    }),
  skillHubPolicyCheck: (slug: string) =>
    fetcher<PolicyCheckResult>(`/api/skillhub/policy/check?slug=${encodeURIComponent(slug)}`),
  skillHubAnalytics: () =>
    fetcher<MarketAnalytics>("/api/skillhub/analytics"),

  // Settings / Config
  getConfigSchema: () => fetcher<{ groups: ConfigGroup[] }>("/api/settings/schema"),
  getConfig: () => fetcher<{ values: Record<string, string> }>("/api/settings/config"),
  saveConfig: (values: Record<string, string>) =>
    fetcher<{ success: boolean; restart_required: boolean; message: string; error?: string }>("/api/settings/config", {
      method: "PUT",
      body: JSON.stringify({ values }),
    }),
  checkSetup: () =>
    fetcher<SetupCheck>("/api/settings/check"),
  configReload: () =>
    fetcher<{ success: boolean; reloaded?: string[]; message?: string; error?: string }>("/v1/config/reload", { method: "POST" }),
  detectDirs: () =>
    fetcher<{ dirs: Array<{ label: string; label_zh: string; path: string; exists: boolean; kind: string }>; default_paths: string[]; current_read: string; current_write: string }>("/api/settings/detect-dirs"),

  // Backup & Restore
  backupInfo: () => fetcher<BackupInfo>("/v1/backup/info"),

  // Conversations
  conversations: (archived = false) =>
    fetcher<{ sessions: ConversationInfo[]; count: number }>(`/v1/conversations?archived=${archived}`),
  conversationMessages: (sessionId: string) =>
    fetcher<{ messages: Array<{ role: string; content: string }>; count: number }>(`/v1/conversations/messages?session_id=${encodeURIComponent(sessionId)}`),
  deleteConversation: (sessionId: string) =>
    fetcher<{ status: string }>(`/v1/conversations/messages?session_id=${encodeURIComponent(sessionId)}`, { method: "DELETE" }),
  manageConversation: (sessionId: string, opts: { name?: string; pinned?: boolean; archive?: boolean }) =>
    fetcher<{ status: string; session: ConversationInfo }>("/v1/conversations/manage", {
      method: "PUT",
      body: JSON.stringify({ session_id: sessionId, ...opts }),
    }),
  backupExport: async () => {
    const key = getApiKey();
    const res = await fetch(`${BASE}/v1/backup/export`, {
      headers: { ...(key ? { "X-API-Key": key } : {}) },
    });
    if (!res.ok) throw new Error(`${res.status}`);
    const blob = await res.blob();
    const url = URL.createObjectURL(blob);
    const a = document.createElement("a");
    a.href = url;
    const cd = res.headers.get("Content-Disposition");
    const match = cd?.match(/filename="(.+)"/);
    a.download = match?.[1] || "yunque-backup.zip";
    a.click();
    URL.revokeObjectURL(url);
  },
  backupImport: async (file: File) => {
    const key = getApiKey();
    const form = new FormData();
    form.append("backup", file);
    const res = await fetch(`${BASE}/v1/backup/import`, {
      method: "POST",
      headers: { ...(key ? { "X-API-Key": key } : {}) },
      body: form,
    });
    if (!res.ok) {
      const text = await res.text();
      throw new Error(`${res.status}: ${text}`);
    }
    return res.json() as Promise<BackupRestoreResult>;
  },

  // Speech (TTS/STT)
  tts: async (text: string, voice?: string): Promise<ArrayBuffer> => {
    const key = getApiKey();
    const res = await fetch(`${BASE}/v1/speech/tts`, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        ...(key ? { "X-API-Key": key } : {}),
      },
      body: JSON.stringify({ text, voice }),
    });
    if (!res.ok) {
      const msg = await res.text();
      throw new Error(`${res.status}: ${msg}`);
    }
    return res.arrayBuffer();
  },
  stt: async (audio: Blob, language?: string): Promise<{ text: string; emotion?: { emotion: string; confidence: number } }> => {
    const key = getApiKey();
    const params = new URLSearchParams();
    if (language) params.set("language", language);
    params.set("detect_emotion", "true");
    const res = await fetch(`${BASE}/v1/speech/stt?${params}`, {
      method: "POST",
      headers: {
        ...(key ? { "X-API-Key": key } : {}),
      },
      body: audio,
    });
    if (!res.ok) {
      const msg = await res.text();
      throw new Error(`${res.status}: ${msg}`);
    }
    return res.json();
  },

  voices: () =>
    fetcher<{ voices: Array<{ id: string; name: string; language: string; gender?: string }>; providers: string[] }>("/v1/speech/voices"),

  // Reverie (inner monologue)
  getReverieJournal: (opts?: { category?: string; min_significance?: number; delivered?: boolean; limit?: number; offset?: number }) => {
    const params = new URLSearchParams();
    if (opts?.category) params.set("category", opts.category);
    if (opts?.min_significance) params.set("min_significance", String(opts.min_significance));
    if (opts?.delivered !== undefined) params.set("delivered", String(opts.delivered));
    if (opts?.limit) params.set("limit", String(opts.limit));
    if (opts?.offset) params.set("offset", String(opts.offset));
    return fetcher<{ thoughts: ReverieThought[]; total: number; limit: number; offset: number }>(`/v1/reverie/journal?${params}`);
  },
  getReverieStats: () => fetcher<ReverieStats>("/v1/reverie/stats"),
  getReverieConfig: () => fetcher<{ config: ReverieConfig; running: boolean }>("/v1/reverie/config"),
  updateReverieConfig: (cfg: Partial<{ enabled: boolean; interval_minutes: number; min_significance: number; quiet_start: number; quiet_end: number }>) =>
    fetcher<{ config: ReverieConfig; running: boolean }>("/v1/reverie/config", { method: "PUT", body: JSON.stringify(cfg) }),
  triggerReverieThink: (eventType?: string, trigger?: string) =>
    fetcher<{ thought: ReverieThought }>("/v1/reverie/think", { method: "POST", body: JSON.stringify({ event_type: eventType, trigger }) }),
  deleteReverieThought: (id: string) =>
    fetcher<{ deleted: boolean; id: string }>(`/v1/reverie/thought?id=${encodeURIComponent(id)}`, { method: "DELETE" }),
  getReverieTargets: () =>
    fetcher<{ targets: Array<{ channel: string; targets: string[]; env_var: string }>; count: number }>("/v1/reverie/targets"),
  getReverieActions: () =>
    fetcher<{ actions: ActionRecord[]; total: number }>("/v1/reverie/actions"),

  // Tasks
  taskCreate: (title: string, description: string) =>
    fetcher<TaskInfo>("/v1/tasks", { method: "POST", body: JSON.stringify({ title, description }) }),
  taskList: () =>
    fetcher<TaskInfo[]>("/v1/tasks"),
  taskGet: (id: string) =>
    fetcher<TaskInfo>(`/v1/tasks?id=${encodeURIComponent(id)}`),
  taskRun: (id: string) =>
    fetcher<{ status: string; task_id: string }>("/v1/tasks/run", { method: "POST", body: JSON.stringify({ id }) }),
  taskCancel: (id: string) =>
    fetcher<{ status: string }>("/v1/tasks/cancel", { method: "POST", body: JSON.stringify({ id }) }),
  taskPause: (id: string) =>
    fetcher<{ status: string }>("/v1/tasks/pause", { method: "POST", body: JSON.stringify({ id }) }),
  taskResume: (id: string) =>
    fetcher<{ status: string }>("/v1/tasks/resume", { method: "POST", body: JSON.stringify({ id }) }),
  taskRestart: (id: string) =>
    fetcher<{ status: string }>("/v1/tasks/restart", { method: "POST", body: JSON.stringify({ id }) }),
  taskDelete: (id: string) =>
    fetcher<{ status: string }>(`/v1/tasks?id=${encodeURIComponent(id)}`, { method: "DELETE" }),
  taskGaps: (stats = false) =>
    fetcher<GapStats | GapRecord[]>(`/v1/tasks/gaps${stats ? "?stats=true" : ""}`),

  // State Kernel
  stateSnapshot: () =>
    fetcher<StateSnapshot>("/v1/state"),
  stateGoals: () =>
    fetcher<StateGoal[]>("/v1/state/goals"),
  stateAddGoal: (title: string, description?: string, priority?: number) =>
    fetcher<{ id: string; status: string }>("/v1/state/goals", { method: "POST", body: JSON.stringify({ title, description, priority }) }),
  stateDeleteGoal: (id: string) =>
    fetcher<{ status: string }>(`/v1/state/goals?id=${encodeURIComponent(id)}`, { method: "DELETE" }),
  stateSetFocus: (focus: string, topics?: string[]) =>
    fetcher<{ status: string }>("/v1/state/focus", { method: "POST", body: JSON.stringify({ focus, topics }) }),

  // Reflection Loop 鈥?experiences and strategies
  getExperiences: (opts?: { source?: string; category?: string; outcome?: ExperienceOutcome; tag?: string; q?: string; stats?: boolean; limit?: number }) => {
    const params = new URLSearchParams();
    if (opts?.source) params.set("source", opts.source);
    if (opts?.category) params.set("category", opts.category);
    if (opts?.outcome) params.set("outcome", opts.outcome);
    if (opts?.tag) params.set("tag", opts.tag);
    if (opts?.q) params.set("q", opts.q);
    if (opts?.stats) params.set("stats", "true");
    if (opts?.limit && Number.isFinite(opts.limit) && opts.limit > 0) params.set("limit", String(Math.trunc(opts.limit)));
    const suffix = params.toString();
    return fetcher<ExperienceStats | { experiences: ExperienceItem[]; total: number }>(`/v1/reflect/experiences${suffix ? `?${suffix}` : ""}`);
  },
  getStrategies: (opts?: { source?: string; category?: string; outcome?: ExperienceOutcome; tag?: string; q?: string; limit?: number }) => {
    const params = new URLSearchParams();
    if (opts?.source) params.set("source", opts.source);
    if (opts?.category) params.set("category", opts.category);
    if (opts?.outcome) params.set("outcome", opts.outcome);
    if (opts?.tag) params.set("tag", opts.tag);
    if (opts?.q) params.set("q", opts.q);
    if (opts?.limit && Number.isFinite(opts.limit) && opts.limit > 0) params.set("limit", String(Math.trunc(opts.limit)));
    const suffix = params.toString();
    return fetcher<{ strategies: string }>(`/v1/reflect/strategies${suffix ? `?${suffix}` : ""}`);
  },

  // Task Templates
  getTemplates: () =>
    fetcher<{ templates: TaskTemplate[]; total: number }>("/v1/tasks/templates"),
  getTemplate: (id: string) =>
    fetcher<TaskTemplate>(`/v1/tasks/templates?id=${encodeURIComponent(id)}`),
  createTemplate: (tpl: Omit<TaskTemplate, "id" | "created_at">) =>
    fetcher<TaskTemplate>("/v1/tasks/templates", { method: "POST", body: JSON.stringify(tpl) }),
  deleteTemplate: (id: string) =>
    fetcher<{ deleted: string }>(`/v1/tasks/templates?id=${encodeURIComponent(id)}`, { method: "DELETE" }),
  instantiateTemplate: (templateId: string, variables: Record<string, string>) =>
    fetcher<TaskInfo>("/v1/tasks/templates/instantiate", {
      method: "POST",
      body: JSON.stringify({ template_id: templateId, variables }),
    }),

  // Document Generation
  generateDocument: (opts: { format: string; path?: string; title?: string; content: string; sheet_name?: string }) =>
    fetcher<{ result: string; path: string; format: string }>("/v1/documents/generate", {
      method: "POST",
      body: JSON.stringify(opts),
    }),

  // Task Threads
  getTaskThread: (taskId: string) =>
    fetcher<{ task_id: string; info: TaskThreadInfo | null; messages: LLMMessage[] }>(`/v1/tasks/threads?id=${encodeURIComponent(taskId)}`),
  listTaskThreads: (state?: ThreadState) =>
    fetcher<{ threads: TaskThreadInfo[]; total: number }>(`/v1/tasks/threads${state ? `?state=${state}` : ""}`),
  postTaskThread: (taskId: string, content: string) =>
    fetcher<{ status: string; task_id: string }>("/v1/tasks/threads", {
      method: "POST",
      body: JSON.stringify({ task_id: taskId, content }),
    }),
  updateThreadState: (taskId: string, state: ThreadState) =>
    fetcher<{ status: string; task_id: string; state: string }>("/v1/tasks/threads", {
      method: "PUT",
      body: JSON.stringify({ task_id: taskId, state }),
    }),

  // Task Working Memory
  getTaskWorkingMemory: (taskId: string) =>
    fetcher<TaskWorkingMemory>(`/v1/tasks/memory?id=${encodeURIComponent(taskId)}`),

  // Cost Breakdown
  getCostByTask: (taskId: string) =>
    fetcher<CostTaskSummary>(`/v1/cost/task?id=${encodeURIComponent(taskId)}`),
  getCostTaskTimeline: (taskId: string) =>
    fetcher<CostUsageEvent[]>(`/v1/cost/task/timeline?id=${encodeURIComponent(taskId)}`),
  getCostBreakdown: () =>
    fetcher<CostBreakdown>("/v1/cost/breakdown"),

  // Triggers (legacy)
  getTriggers: () =>
    fetcher<{ triggers: TriggerItem[]; total: number }>("/v1/triggers"),
  getTrigger: (id: string) =>
    fetcher<TriggerItem>(`/v1/triggers?id=${encodeURIComponent(id)}`),
  createTrigger: (t: Partial<TriggerItem>) =>
    fetcher<TriggerItem>("/v1/triggers", { method: "POST", body: JSON.stringify(t) }),
  deleteTrigger: (id: string) =>
    fetcher<{ deleted: string }>(`/v1/triggers?id=${encodeURIComponent(id)}`, { method: "DELETE" }),
  emitTrigger: (event: string, text?: string, data?: Record<string, unknown>) =>
    fetcher<{ status: string; event: string }>("/v1/triggers/emit", {
      method: "POST",
      body: JSON.stringify({ event, text, data }),
    }),

  // Triggers V2 (unified)
  getTriggersV2: (params?: { tenant_id?: string; type?: string; status?: string }) => {
    const q = new URLSearchParams();
    if (params?.tenant_id) q.set("tenant_id", params.tenant_id);
    if (params?.type) q.set("type", params.type);
    if (params?.status) q.set("status", params.status);
    const qs = q.toString();
    return fetcher<{ triggers: TriggerDef[]; total: number }>(`/v1/triggers/v2${qs ? "?" + qs : ""}`);
  },
  getTriggerV2: (id: string) =>
    fetcher<TriggerDef>(`/v1/triggers/v2?id=${encodeURIComponent(id)}`),
  createTriggerV2: (t: Partial<TriggerDef>) =>
    fetcher<TriggerDef>("/v1/triggers/v2", { method: "POST", body: JSON.stringify(t) }),
  updateTriggerV2: (t: Partial<TriggerDef>) =>
    fetcher<TriggerDef>("/v1/triggers/v2", { method: "PUT", body: JSON.stringify(t) }),
  deleteTriggerV2: (id: string) =>
    fetcher<{ deleted: string }>(`/v1/triggers/v2?id=${encodeURIComponent(id)}`, { method: "DELETE" }),
  emitTriggerV2: (payload: Partial<TriggerEventPayload>) =>
    fetcher<{ status: string; event: string }>("/v1/triggers/v2/emit", {
      method: "POST",
      body: JSON.stringify(payload),
    }),
  getTriggerRuns: (triggerID?: string, limit?: number) => {
    const q = new URLSearchParams();
    if (triggerID) q.set("trigger_id", triggerID);
    if (limit) q.set("limit", String(limit));
    const qs = q.toString();
    return fetcher<{ runs: TriggerRun[]; total: number }>(`/v1/triggers/v2/runs${qs ? "?" + qs : ""}`);
  },
  getTriggerEvents: (triggerID?: string, limit?: number) => {
    const q = new URLSearchParams();
    if (triggerID) q.set("trigger_id", triggerID);
    if (limit) q.set("limit", String(limit));
    const qs = q.toString();
    return fetcher<{ events: TriggerLogEvent[]; total: number }>(`/v1/triggers/v2/events${qs ? "?" + qs : ""}`);
  },

  // Plugin UI
  pluginUITabs: () =>
    fetcher<{ tabs: PluginUITab[] }>("/v1/plugins/ui"),

  // Pack Runtime: installed/enabled capability packs are the backend source of truth
  packsInstalled: () =>
    fetcher<PackListResponse>("/v1/packs/installed"),
  packsEnabled: () =>
    fetcher<PackListResponse>("/v1/packs/enabled"),
  packBackendModules: () =>
    fetcher<PackBackendModulesResponse>("/v1/packs/backend-modules"),
  packInstall: (manifestPath: string, source?: string, download?: boolean) =>
    fetcher<PackMutationResponse>("/v1/packs/install", { method: "POST", body: JSON.stringify({ manifest_path: manifestPath, source, download }) }),
  packInstallFromURL: (manifestUrl: string, source?: string, download?: boolean) =>
    fetcher<PackMutationResponse>("/v1/packs/install", { method: "POST", body: JSON.stringify({ manifest_url: manifestUrl, source, download }) }),
  packEnable: (id: string) =>
    fetcher<PackMutationResponse>("/v1/packs/enable", { method: "POST", body: JSON.stringify({ id }) }),
  packDisable: (id: string) =>
    fetcher<PackMutationResponse>("/v1/packs/disable", { method: "POST", body: JSON.stringify({ id }) }),
  packRollback: (id: string) =>
    fetcher<PackMutationResponse>("/v1/packs/rollback", { method: "POST", body: JSON.stringify({ id }) }),
  packPrune: () =>
    fetcher<{ removed: string[]; kept: string[]; errors?: string[]; removed_count: number; kept_count: number }>("/v1/packs/prune", { method: "POST", body: JSON.stringify({}) }),

  // QQ Chat Analyzer (ext plugin)
  qqUpload: (content: string, filename?: string) =>
    fetcher<QQAnalysis>("/v1/ext/qqchat/upload", { method: "POST", body: JSON.stringify({ content, filename }) }),
  qqAnalyses: () =>
    fetcher<{ analyses: QQAnalysis[] }>("/v1/ext/qqchat/analyses"),
  qqAnalysis: (id: string) =>
    fetcher<QQAnalysis>(`/v1/ext/qqchat/analysis?id=${encodeURIComponent(id)}`),
  qqRoleplay: (id: string, persona: string, message: string) =>
    fetcher<{ persona: string; reply: string }>("/v1/ext/qqchat/roleplay", { method: "POST", body: JSON.stringify({ id, persona, message }) }),
  qqDelete: (id: string) =>
    fetcher<{ status: string }>(`/v1/ext/qqchat/delete?id=${encodeURIComponent(id)}`, { method: "DELETE" }),

  // Mission NL Parse
  missionParse: (description: string) =>
    fetcher<{ type: string; name: string; description: string; config: Record<string, unknown>; confidence: number; explanation: string }>("/v1/missions/parse", {
      method: "POST",
      body: JSON.stringify({ description }),
    }),

  // 鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲
  // Approval System
  // 鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲

  approvalsList: (status?: string) => {
    const q = status ? `?status=${status}` : "";
    return fetcher<{ approvals: ApprovalRequest[]; total: number }>(`/v1/approvals${q}`);
  },
  approvalDecide: (id: string, decision: "allow_once" | "allow_always" | "deny_always", reason?: string) =>
    fetcher<{ status: string; approval_id: string }>("/v1/approvals/decide", {
      method: "POST",
      body: JSON.stringify({ id, decision, reason }),
    }),
  approvalRules: () =>
    fetcher<{ rules: ApprovalRule[]; total: number }>("/v1/approvals/rules"),
  approvalRuleCreate: (rule: Partial<ApprovalRule>) =>
    fetcher<ApprovalRule>("/v1/approvals/rules", {
      method: "POST",
      body: JSON.stringify(rule),
    }),
  approvalRuleDelete: (id: string) =>
    fetcher<{ deleted: string }>(`/v1/approvals/rules?id=${encodeURIComponent(id)}`, { method: "DELETE" }),

  // 鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲
  // Setup / Environment Detection
  // 鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲

  setupDetect: () =>
    fetcher<SetupEnvironment>("/v1/setup/detect"),
  setupHealth: () =>
    fetcher<SetupHealthResult>("/v1/setup/health"),
  setupTemplates: () =>
    fetcher<SetupTemplatesResponse>("/v1/setup/templates"),
  setupTestProvider: (req: { base_url: string; api_key?: string; model?: string }) =>
    fetcher<SetupTestProviderResult>("/v1/setup/test-provider", {
      method: "POST",
      body: JSON.stringify(req),
    }),
  setupApply: (templateId: string, overrides?: Record<string, unknown>) =>
    fetcher<SetupApplyResponse>("/v1/setup/apply", {
      method: "POST",
      body: JSON.stringify({ template_id: templateId, overrides }),
    }),

  // 鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲
  // Session Queue
  // 鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲

  sessionQueueStatus: (sessionId?: string) => {
    const q = sessionId ? `?session_id=${encodeURIComponent(sessionId)}` : "";
    return fetcher<SessionQueueInfo>(`/v1/sessions/queue${q}`);
  },
  sessionQueueCancel: (sessionId: string, taskId: string) =>
    fetcher<{ status: string; cancelled: string }>("/v1/sessions/queue/cancel", {
      method: "POST",
      body: JSON.stringify({ session_id: sessionId, task_id: taskId }),
    }),

  // 鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲
  // Trust 鈥?progressive trust & one-click delegation
  // 鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲

  trustScores: () =>
    fetcher<{ scores: Record<string, TrustEntry>; count: number }>("/api/trust/scores"),
  trustReset: (slug: string) =>
    fetcher<{ status: string; slug: string }>("/api/trust/reset", {
      method: "POST",
      body: JSON.stringify({ slug }),
    }),
  /** Grant full trust to one skill, or pass slug="*" for all skills. */
  trustGrant: (slug: string) =>
    fetcher<{ status: string; slug?: string; upgraded?: number; level?: string }>("/api/trust/grant", {
      method: "POST",
      body: JSON.stringify({ slug }),
    }),

  // 鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲
  // RBAC 鈥?role-based access control
  // 鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲

  rbacRoles: () =>
    fetcher<{ roles: RBACRole[] }>("/v1/rbac/roles"),
  rbacCreateRole: (role: Partial<RBACRole>) =>
    fetcher<{ status: string }>("/v1/rbac/roles", {
      method: "POST",
      body: JSON.stringify(role),
    }),
  rbacAssign: (subjectId: string, roleId: string, tenantId?: string) =>
    fetcher<{ status: string }>("/v1/rbac/assign", {
      method: "POST",
      body: JSON.stringify({ subject_id: subjectId, role_id: roleId, tenant_id: tenantId }),
    }),
  rbacRevoke: (subjectId: string, roleId: string, tenantId?: string) =>
    fetcher<{ status: string }>("/v1/rbac/revoke", {
      method: "POST",
      body: JSON.stringify({ subject_id: subjectId, role_id: roleId, tenant_id: tenantId }),
    }),
  rbacCheck: (subjectId: string, resource: string, action: string, tenantId?: string) =>
    fetcher<{ allowed: boolean; resource: string; action: string }>("/v1/rbac/check", {
      method: "POST",
      body: JSON.stringify({ subject_id: subjectId, resource, action, tenant_id: tenantId }),
    }),
  rbacMyRoles: () =>
    fetcher<{ roles: RBACRole[]; subject_id: string }>("/v1/rbac/my-roles"),

  // 鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲
  // Iterate 鈥?self-improvement engine
  // 鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲

  iterateProposals: () =>
    fetcher<{ proposals: IterateProposal[]; count: number }>("/api/iterate/proposals"),
  iterateApprove: (id: string) =>
    fetcher<{ status: string }>("/api/iterate/approve", {
      method: "POST",
      body: JSON.stringify({ id }),
    }),
  iterateReject: (id: string) =>
    fetcher<{ status: string }>("/api/iterate/reject", {
      method: "POST",
      body: JSON.stringify({ id }),
    }),
  iterateTrigger: () =>
    fetcher<{ status: string }>("/api/iterate/trigger", { method: "POST" }),
  iterateStatus: () =>
    fetcher<IterateStatus>("/api/iterate/status"),

  // 鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲
  // SkillGrow 鈥?skill growth detection
  // 鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲

  skillGrowPatterns: () =>
    fetcher<{ patterns: SkillGrowPattern[]; count: number }>("/api/skillgrow/patterns"),

  // 鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲
  // Review 鈥?security review gate
  // 鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲

  reviewStatus: () =>
    fetcher<{ enabled: boolean; trust_enabled: boolean }>("/api/review/status"),

  // Browser automation
  browserNavigate: (url: string) =>
    fetcher<{ screenshot?: string }>("/v1/browser/navigate", { method: "POST", body: JSON.stringify({ url }) }),
  browserScreenshot: () =>
    fetcher<{ screenshot?: string }>("/v1/browser/screenshot"),
  browserOcr: (mode: string) =>
    fetcher<{ text?: string; result?: string }>("/v1/browser/ocr", { method: "POST", body: JSON.stringify({ mode }) }),
  browserStatus: () =>
    fetcher<BrowserStatus>("/v1/browser/status"),
  browserConfig: () =>
    fetcher<BrowserConfig>("/v1/browser/config"),
  browserScreenshotLatest: () =>
    fetcher<{ screenshot?: string; timestamp?: string }>("/v1/browser/screenshot/latest"),
  browserOPPPending: () =>
    fetcher<{ items: OPPItem[]; total: number }>("/v1/browser/opp/pending"),
  browserOPPDecide: (id: string, decision: "allow" | "deny") =>
    fetcher<{ status: string }>("/v1/browser/opp/decide", { method: "POST", body: JSON.stringify({ id, decision }) }),

  // Browser Extension (Connector)
  browserExtStatus: () =>
    fetcher<{ connected: boolean; version?: string; pending?: number }>("/api/browser/ext/status"),
  browserExtAction: (action: Record<string, unknown>) =>
    fetcher<{ ok: boolean; error?: string; screenshot?: string }>("/api/browser/ext/action", { method: "POST", body: JSON.stringify(action) }),
  browserExtScenarios: () =>
    fetcher<{ scenarios: BrowserScenario[] }>("/api/browser/ext/scenarios"),
  browserExtRunScenario: (scenarioId: string) =>
    fetcher<{ ok: boolean; scenario: string; results: ScenarioStepResult[] }>("/api/browser/ext/scenarios/run", { method: "POST", body: JSON.stringify({ scenario_id: scenarioId }) }),

  // Document generation
  docgenExport: (params: { title: string; content: string; format: string }) =>
    fetcher<{ result?: string; path?: string; format?: string }>("/v1/documents/generate", { method: "POST", body: JSON.stringify(params) }),
  docgenTemplates: () =>
    fetcher<{ templates: DocTemplate[] }>("/v1/documents/templates"),

  // 鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲
  // Provider Management
  // 鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲

  providerList: () =>
    fetcher<{ providers: ProviderInfo[]; count: number }>("/api/providers"),
  providerEnable: (id: string) =>
    fetcher<{ status: string }>("/api/providers/enable", { method: "POST", body: JSON.stringify({ id }) }),
  providerDisable: (id: string) =>
    fetcher<{ status: string }>("/api/providers/disable", { method: "POST", body: JSON.stringify({ id }) }),
  providerTest: (id: string) =>
    fetcher<ProviderTestResult>("/api/providers/test", { method: "POST", body: JSON.stringify({ id }) }),
  providerSwitchModel: (id: string, model: string) =>
    fetcher<{ ok: boolean; model: string }>("/api/providers/switch-model", { method: "POST", body: JSON.stringify({ id, model }) }),
  providerLocalDiscover: () =>
    fetcher<{ backend: string; base_url: string; available: boolean; models?: { id: string; name: string; backend: string; base_url: string }[]; error?: string; latency: number }>("/api/providers/local/discover"),
  providerLocalRegister: (name: string, baseUrl: string, models: string[]) =>
    fetcher<{ status: string }>("/api/providers/local/register", { method: "POST", body: JSON.stringify({ name, base_url: baseUrl, models }) }),
  providerSessionOverride: (providerId: string, sessionId?: string) =>
    fetcher<{ status: string }>("/api/providers/session", { method: "POST", body: JSON.stringify({ provider_id: providerId, session_id: sessionId }) }),

  providerMode: () =>
    fetcher<{ mode: string }>("/api/providers/mode"),
  setProviderMode: (mode: string) =>
    fetcher<{ mode: string }>("/api/providers/mode", { method: "POST", body: JSON.stringify({ mode }) }),
  providerPresets: () =>
    fetcher<{ presets: ProviderPreset[] }>("/api/providers/presets"),
  providerRegister: (req: { preset_id?: string; id?: string; base_url?: string; api_key?: string; model?: string; name?: string; tier?: string }) =>
    fetcher<{ ok: boolean; provider_id?: string }>("/api/providers/register", { method: "POST", body: JSON.stringify(req) }),
  providerDelete: (id: string) =>
    fetcher<{ ok: boolean }>("/api/providers/delete", { method: "POST", body: JSON.stringify({ id }) }),
  breakerReset: () =>
    fetcher<{ ok: boolean; reset_count: number }>("/api/breaker/reset", { method: "POST" }),
  execProvider: () =>
    fetcher<{ exec_provider: string; available_providers: string[] }>("/api/providers/exec"),
  setExecProvider: (providerId: string) =>
    fetcher<{ ok: boolean; exec_provider: string }>("/api/providers/exec", { method: "POST", body: JSON.stringify({ provider_id: providerId }) }),
  toriDiscover: (autoRegister = false) =>
    fetcher<{ models: Array<{ id: string }>; registered?: number }>(`/api/providers/tori/discover?auto_register=${autoRegister}`),

  toriBind: (toriURL: string) =>
    fetcher<{ status: string; auth_url?: string }>("/v1/tori/bind", { method: "POST", body: JSON.stringify({ tori_url: toriURL }) }),
  toriStatus: () =>
    fetcher<ToriBindingStatus>("/v1/tori/status"),
  toriUnbind: () =>
    fetcher<{ status: string }>("/v1/tori/unbind", { method: "POST" }),
  toriHealth: () =>
    fetcher<ToriHealthStatus>("/v1/tori/health"),
  toriUsage: () =>
    fetcher<ToriUsageSummary>("/v1/tori/usage"),
  skillSuggestions: (sessionId: string) =>
    fetcher<{ suggestions: SkillSuggestion[] }>(`/v1/skill-suggestions?session_id=${encodeURIComponent(sessionId)}`),

  syncStatus: () =>
    fetcher<{ items: SyncManifestItem[]; count: number }>("/api/sync/status"),

  // 鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲
  // Cost 鈥?extended
  // 鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲

  costSummary: () =>
    fetcher<CostSummary>("/v1/cost/summary"),
  costHistory: (days?: number) => {
    const q = days ? `?days=${days}` : "";
    return fetcher<CostHistoryEntry[]>(`/v1/cost/history${q}`);
  },
  costAlerts: () =>
    fetcher<{ alerts: CostAlert[]; total: number }>("/v1/cost/alerts"),
  costBudget: () =>
    fetcher<CostBudget>("/v1/cost/budget"),
  costSetBudget: (budget: Partial<CostBudget>) =>
    fetcher<{ status: string }>("/v1/cost/budget", { method: "POST", body: JSON.stringify(budget) }),

  // 鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲
  // Knowledge Graph
  // 鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲

  graphEntities: (limit = 50) =>
    fetcher<{ entities: GraphEntity[]; total: number }>(`/v1/graph/entities?limit=${limit}`),
  graphRelations: (entityId?: string, limit = 500) => {
    const q = new URLSearchParams({ limit: String(limit) });
    if (entityId) q.set("entity_id", entityId);
    return fetcher<{ relations: GraphRelation[] }>(`/v1/graph/relations?${q}`);
  },
  graphStats: () =>
    fetcher<GraphStats>("/v1/graph/stats"),
  graphDeleteEntity: (id: string) =>
    fetcher<{ ok: boolean }>(`/v1/graph/entities?id=${encodeURIComponent(id)}`, { method: "DELETE" }),
  graphContext: (query: string, limit = 10) =>
    fetcher<{ context: string; entities: GraphEntity[] }>(`/v1/graph/context?q=${encodeURIComponent(query)}&limit=${limit}`),

  // 鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲
  // Memory 鈥?extended operations
  // 鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲

  memoryAdd: (content: string, metadata?: Record<string, string>) =>
    fetcher<{ id: string; status: string }>("/v1/memory/add", { method: "POST", body: JSON.stringify({ content, metadata }) }),
  memorySearch: (query: string, limit = 10) =>
    fetcher<{ results: MemorySearchResult[]; total: number }>(`/v1/memory/search?q=${encodeURIComponent(query)}&limit=${limit}`),
  memoryCompact: () =>
    fetcher<{ status: string; before: number; after: number }>("/v1/memory/compact", { method: "POST" }),

  // 鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲
  // System
  // 鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲

  systemInfo: () =>
    fetcher<SystemInfo>("/v1/system/info"),
  cacheStats: () =>
    fetcher<CacheStats>("/v1/cache/stats"),
  routerStats: () =>
    fetcher<RouterStats>("/v1/router/stats"),

  // 鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲
  // Persona Modes
  // 鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲

  personaModes: () =>
    fetcher<{ modes: PersonaMode[] }>("/v1/persona/modes"),
  personaCurrentMode: () =>
    fetcher<{ mode: string }>("/v1/persona/mode/current"),
  personaSetMode: (mode: string) =>
    fetcher<{ status: string; mode: string }>("/v1/persona/mode", { method: "POST", body: JSON.stringify({ mode }) }),

  // 鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲
  // Search
  // 鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲

  search: (query: string, limit = 10) =>
    fetcher<{ results: SearchResult[]; total: number }>(`/v1/search?q=${encodeURIComponent(query)}&limit=${limit}`),
  searchProviders: () =>
    fetcher<{ providers: string[] }>("/v1/search/providers"),

  // 鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲
  // Federation
  // 鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲

  federationPeers: () =>
    fetcher<{ peers: FederationPeer[]; total: number }>("/v1/federation/peers"),
  federationStats: () =>
    fetcher<FederationStats>("/v1/federation/stats"),

  // 鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲
  // Workflows 鈥?DAG workflow engine
  // 鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲鈺愨晲

  workflowList: () =>
    fetcher<{ workflows: WorkflowDef[]; total: number }>("/v1/workflows"),
  workflowGet: (id: string) =>
    fetcher<WorkflowDef>(`/v1/workflows?id=${encodeURIComponent(id)}`),
  workflowSave: (def: Partial<WorkflowDef>) =>
    fetcher<WorkflowDef>("/v1/workflows", { method: "POST", body: JSON.stringify(def) }),
  workflowDelete: (id: string) =>
    fetcher<{ deleted: string }>(`/v1/workflows?id=${encodeURIComponent(id)}`, { method: "DELETE" }),
  workflowRun: (definitionId: string, variables?: Record<string, unknown>) =>
    fetcher<{ status: string; instance_id: string; instance: WorkflowInstance }>("/v1/workflows/run", {
      method: "POST",
      body: JSON.stringify({ definition_id: definitionId, variables }),
    }),
  workflowInstances: (id?: string) => {
    const q = id ? `?id=${encodeURIComponent(id)}` : "";
    return fetcher<{ instances: WorkflowInstance[]; total: number } | WorkflowInstance>(`/v1/workflows/instances${q}`);
  },
  workflowCancel: (instanceId: string) =>
    fetcher<{ status: string; instance_id: string }>("/v1/workflows/cancel", {
      method: "POST",
      body: JSON.stringify({ instance_id: instanceId }),
    }),

  // ── Connectors ─────────────────────────────────────
  connectorList: () =>
    fetcher<{ connectors: ConnectorView[] }>("/api/connectors"),
  connectorDetail: (id: string) =>
    fetcher<{ connector: ConnectorDef; supported: boolean; status: string; user_info: string; error: string }>(
      `/api/connectors/detail?id=${encodeURIComponent(id)}`
    ),
  connectorConnect: (connectorId: string, token: string) =>
    fetcher<{ ok: boolean; status: string; user_info: string }>("/api/connectors/connect", {
      method: "POST",
      body: JSON.stringify({ connector_id: connectorId, token }),
    }),
  connectorDisconnect: (connectorId: string) =>
    fetcher<{ ok: boolean }>("/api/connectors/disconnect", {
      method: "POST",
      body: JSON.stringify({ connector_id: connectorId }),
    }),
  connectorExecute: (connectorId: string, actionId: string, params: Record<string, unknown>) =>
    fetcher<{ ok: boolean; result: unknown }>("/api/connectors/execute", {
      method: "POST",
      body: JSON.stringify({ connector_id: connectorId, action_id: actionId, params }),
    }),

  // ── Notifications ──────────────────────────────────────
  notifyChannels: () =>
    fetcher<{ channels: NotifyChannel[] }>("/api/notify/channels"),
  notifyAdd: (channel: Partial<NotifyChannel>) =>
    fetcher<{ ok: boolean }>("/api/notify/add", {
      method: "POST",
      body: JSON.stringify(channel),
    }),
  notifyRemove: (id: string) =>
    fetcher<{ ok: boolean }>("/api/notify/remove", {
      method: "POST",
      body: JSON.stringify({ id }),
    }),
  notifyToggle: (id: string, enabled: boolean) =>
    fetcher<{ ok: boolean }>("/api/notify/toggle", {
      method: "POST",
      body: JSON.stringify({ id, enabled }),
    }),
  notifyTest: (id: string) =>
    fetcher<{ ok: boolean }>("/api/notify/test", {
      method: "POST",
      body: JSON.stringify({ id }),
    }),
  notifyShare: (payload: NotifyShareRequest) =>
    fetcher<NotifyShareResponse>("/api/notify/share", {
      method: "POST",
      body: JSON.stringify(payload),
    }),

  // Cloud Desktop Sandbox (E2B)
  desktopCreate: () =>
    fetcher<{ ok: boolean; sandbox?: { id: string; stream_url: string; created_at: string; vnc_log?: string[] }; message?: string }>("/v1/sandbox/desktop", { method: "POST" }),
  desktopStatus: () =>
    fetcher<{ ok: boolean; running: boolean; sandbox?: { id: string; stream_url: string; created_at: string; vnc_log?: string[] }; alive?: boolean }>("/v1/sandbox/desktop/status"),
  desktopDestroy: () =>
    fetcher<{ ok: boolean; message?: string }>("/v1/sandbox/desktop/destroy", { method: "POST" }),

  // Workers (MCP Dispatch)
  listWorkers: () =>
    fetcher<{ workers: WorkerInfo[]; count: number }>("/v1/workers"),
  getWorkerDetail: (id: string) =>
    fetcher<WorkerInfo>(`/v1/workers/detail?id=${encodeURIComponent(id)}`),
  removeWorker: (id: string) =>
    fetcher<{ status: string }>("/v1/workers/remove", { method: "POST", body: JSON.stringify({ id }) }),
  getWorkerConfig: (type: string) =>
    fetcher<{ type: string; mcp_config: string; instructions: string; server_url: string }>(`/v1/workers/config?type=${encodeURIComponent(type)}`),

  // Projects (Orchestrator)
  listProjects: () =>
    fetcher<{ projects: ProjectInfo[] }>("/v1/projects"),
  createProject: (data: { name: string; repo_path: string; repo_url?: string; description?: string; default_caps?: string[] }) =>
    fetcher<ProjectInfo>("/v1/projects", { method: "POST", body: JSON.stringify(data) }),
  getProject: (id: string) =>
    fetcher<ProjectInfo>(`/v1/projects/detail?id=${encodeURIComponent(id)}`),
  updateProject: (id: string, data: Record<string, unknown>) =>
    fetcher<ProjectInfo>(`/v1/projects/detail?id=${encodeURIComponent(id)}`, { method: "PUT", body: JSON.stringify(data) }),
  removeProject: (id: string) =>
    fetcher<{ status: string }>("/v1/projects/remove", { method: "POST", body: JSON.stringify({ id }) }),

  // Orchestrator
  orchestratorStatus: () =>
    fetcher<{ running: boolean; adapters: string[]; active_sessions: number }>("/v1/orchestrator/status"),
  orchestratorToggle: (action: "start" | "stop") =>
    fetcher<{ status: string }>("/v1/orchestrator/toggle", { method: "POST", body: JSON.stringify({ action }) }),
  orchestratorSessions: () =>
    fetcher<{ sessions: Array<{ session_id: string; adapter: string; task_id: string; started_at: string }> }>("/v1/orchestrator/sessions"),
  detectIDEs: () =>
    fetcher<{ ides: Array<{ name: string; binary: string; available: boolean; path?: string }> }>("/v1/orchestrator/detect"),

  // ── Cogni (declarative AI-cognition shells) ──
  listCognis: () =>
    fetcher<import("./api-types/cogni").CogniListResponse>("/v1/cognis"),
  getCogni: (id: string) =>
    fetcher<{ id: string; declaration: import("./api-types/cogni").CogniDeclaration; enabled: boolean }>(
      `/v1/cognis/${encodeURIComponent(id)}`,
    ),
  addCogni: (decl: import("./api-types/cogni").CogniDeclaration) =>
    fetcher<{ status: string; id: string }>("/v1/cognis", {
      method: "POST",
      body: JSON.stringify(decl),
    }),
  removeCogni: (id: string) =>
    fetcher<{ status: string; id: string }>(`/v1/cognis/${encodeURIComponent(id)}`, { method: "DELETE" }),
  setCogniEnabled: (id: string, enabled: boolean) =>
    fetcher<{ status: string; id: string }>(
      `/v1/cognis/${encodeURIComponent(id)}/${enabled ? "enable" : "disable"}`,
      { method: "POST" },
    ),
  reloadCognis: () =>
    fetcher<import("./api-types/cogni").CogniReloadResponse>("/v1/cognis/reload", { method: "POST" }),
  getCogniHealth: () =>
    fetcher<{ health: import("./api-types/cogni").CogniHealthMetrics[]; count: number }>("/v1/cognis/health"),
  getCogniAlerts: () =>
    fetcher<{ alerts: import("./api-types/cogni").CogniAlert[]; count: number }>("/v1/cognis/alerts"),
  scanCogniAlerts: () =>
    fetcher<{ alerts: import("./api-types/cogni").CogniAlert[]; count: number }>("/v1/cognis/alerts/scan", {
      method: "POST",
    }),
  verifyCognis: () =>
    fetcher<{
      results: Record<string, Array<{ cogni_id: string; check_name?: string; check_index: number; passed: boolean; reason?: string; got_active: boolean; got_score: number }>>;
      failures: Array<{ cogni_id: string; check_name?: string; check_index: number; reason?: string }>;
    }>("/v1/cognis/verify", { method: "POST" }),
  getCogniTraces: (limit = 50) =>
    fetcher<{ traces: import("./api-types/cogni").CogniTrace[]; count: number }>(
      `/v1/cognis/traces?limit=${limit}`,
    ),
  getCogniTracesByID: (id: string, limit = 50) =>
    fetcher<{ id: string; traces: import("./api-types/cogni").CogniTrace[]; count: number }>(
      `/v1/cognis/${encodeURIComponent(id)}/trace?limit=${limit}`,
    ),

  // ── Cogni Self-Genesis ──
  generateCogni: (description: string, autoSave = false) =>
    fetcher<{ status: string; declaration: import("./api-types/cogni").CogniDeclaration; saved: boolean }>(
      "/v1/cognis/generate",
      { method: "POST", body: JSON.stringify({ description, auto_save: autoSave }) },
    ),

  // ── Cogni Workflows ──
  getCogniWorkflows: (id: string) =>
    fetcher<{ id: string; workflows: import("./api-types/cogni").CogniWorkflowDef[]; count: number }>(
      `/v1/cognis/${encodeURIComponent(id)}/workflows`,
    ),
  runCogniWorkflow: (id: string, workflowName: string, input?: Record<string, unknown>) =>
    fetcher<import("./api-types/cogni").CogniWorkflowResult>(
      `/v1/cognis/${encodeURIComponent(id)}/workflow/${encodeURIComponent(workflowName)}`,
      { method: "POST", body: input ? JSON.stringify(input) : undefined },
    ),

  // ── Cogni Experience ──
  getCogniExperience: (id: string) =>
    fetcher<import("./api-types/cogni").CogniExperienceResponse>(`/v1/cognis/${encodeURIComponent(id)}/experience`),
  confirmCogniExperiencePattern: (id: string, patternID: string) =>
    fetcher<{ status: string; id: string; confirmed: boolean }>(
      `/v1/cognis/${encodeURIComponent(id)}/experience/patterns/${encodeURIComponent(patternID)}/confirm`,
      { method: "POST" },
    ),

  // ── Cogni Evolution ──
  triggerCogniEvolution: (id: string) =>
    fetcher<{ status: string; id: string }>(
      `/v1/cognis/${encodeURIComponent(id)}/evolve`,
      { method: "POST" },
    ),
  getCogniEvolution: (id: string) =>
    fetcher<{ id: string; experiments: import("./api-types/cogni").CogniExperiment[]; running: boolean }>(
      `/v1/cognis/${encodeURIComponent(id)}/evolution`,
    ),

  // ── Cogni Federation ──
  getCogniFederation: () =>
    fetcher<Record<string, unknown>>("/v1/cognis/federation"),
  getCogniFederationPeers: () =>
    fetcher<{ peers: import("./api-types/cogni").CogniFederationPeer[]; count: number }>(
      "/v1/cognis/federation/peers",
    ),
  exposeCogni: (id: string, expose: boolean) =>
    fetcher<{ status: string; id: string }>(
      `/v1/cognis/${encodeURIComponent(id)}/${expose ? "expose" : "unexpose"}`,
      { method: "POST" },
    ),

  // ── LoRA & evolution ──
  getLoRAStatus: () => fetcher<import("./api-types/lora").LoRAStatus>("/v1/lora/status"),
  getLoRAHistory: () =>
    fetcher<{ records: import("./api-types/lora").TrainingRecord[]; count: number }>("/v1/lora/history"),
  getLoRASummary: () =>
    fetcher<{ summary: import("./api-types/lora").TrainingSummary }>("/v1/lora/summary"),
  previewLoRATrainingData: (tenantId?: string) =>
    fetcher<{ preview: import("./api-types/lora").TrainingDataPreview }>(
      `/v1/lora/preview${tenantId ? `?tenant_id=${encodeURIComponent(tenantId)}` : ""}`,
    ),
  triggerLoRATraining: (tenantId?: string) =>
    fetcher<{ status: string; tenant_id: string }>("/v1/lora/trigger", {
      method: "POST",
      body: JSON.stringify(tenantId ? { tenant_id: tenantId } : {}),
    }),
  rollbackLoRA: () => fetcher<{ status: string }>("/v1/lora/rollback", { method: "POST" }),
  getEvolutionState: () =>
    fetcher<{ state: import("./api-types/lora").EvolutionState }>("/v1/lora/evolution"),

  getLoRAConfig: () =>
    fetcher<{ config: import("./api-types/lora").LoRAConfig }>("/v1/lora/config"),

  updateLoRAConfig: (patch: Partial<{
    min_samples: number;
    min_interval: string;
    eval_min_score: number;
    max_adapters: number;
    base_model: string;
    training_data_dir: string;
    adapter_dir: string;
    ab_test_duration: string;
  }>) =>
    fetcher<{ config: import("./api-types/lora").LoRAConfig; status: string }>(
      "/v1/lora/config",
      { method: "PUT", body: JSON.stringify(patch) },
    ),
};

// Sticker URL utilities
export function lineStickerUrl(stickerId: string): string {
  return `https://stickershop.line-scdn.net/stickershop/v1/sticker/${stickerId}/iPhone/sticker.png`;
}

export function parseStickerPattern(text: string): { packageId: string; stickerId: string } | null {
  const match = text.match(/\[贴图: packageId=(\d+), stickerId=(\d+)\]/);
  if (!match) return null;
  return { packageId: match[1], stickerId: match[2] };
}
