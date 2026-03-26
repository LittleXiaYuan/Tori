const BASE = process.env.NEXT_PUBLIC_API_BASE || "";

let apiKey = "";

export function setApiKey(key: string) {
  apiKey = key;
  if (typeof window !== "undefined") localStorage.setItem("yunque_api_key", key);
}

export function getApiKey(): string {
  if (apiKey) return apiKey;
  if (typeof window !== "undefined") {
    apiKey = localStorage.getItem("yunque_api_key") || "";
  }
  return apiKey;
}

function getAuthHeaders(): Record<string, string> {
  const token = typeof window !== "undefined" ? localStorage.getItem("yunque_token") : "";
  if (token) return { Authorization: `Bearer ${token}` };
  const key = getApiKey();
  if (key) return { "X-API-Key": key };
  return {};
}

async function fetcher<T>(path: string, opts?: RequestInit): Promise<T> {
  const res = await fetch(`${BASE}${path}`, {
    ...opts,
    headers: {
      "Content-Type": "application/json",
      ...getAuthHeaders(),
      ...opts?.headers,
    },
  });
  if (res.status === 401 && typeof window !== "undefined" && !path.includes("/auth/")) {
    localStorage.removeItem("yunque_token");
    window.location.href = "/login";
    throw new Error("unauthorized");
  }
  if (!res.ok) {
    const text = await res.text();
    throw new Error(`${res.status}: ${text}`);
  }
  return res.json();
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

export const api = {
  healthz: () => fetcher<{ status: string; version: string }>("/healthz"),
  version: () => fetcher<VersionInfo>("/v1/version"),
  metrics: () => fetcher<MetricsSnapshot>("/v1/metrics"),

  skills: () => fetcher<{ skills: SkillInfo[]; count: number }>("/v1/skills").then((r) => Array.isArray(r.skills) ? r.skills : []),

  tenants: () => fetcher<{ tenants: TenantInfo[]; count: number }>("/v1/tenants"),
  createTenant: (name: string) =>
    fetcher<TenantInfo>("/v1/tenants", {
      method: "POST",
      body: JSON.stringify({ name }),
    }),

  chat: (messages: Array<{ role: string; content: string }>, sessionId?: string) =>
    fetcher<ChatResponse>("/v1/chat", {
      method: "POST",
      body: JSON.stringify({ messages, session_id: sessionId }),
    }),

  chatStream: async function* (
    messages: Array<{ role: string; content: string }>,
    sessionId?: string
  ): AsyncGenerator<string> {
    const key = getApiKey();
    const res = await fetch(`${BASE}/v1/chat/agentic`, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        ...(key ? { "X-API-Key": key } : {}),
      },
      body: JSON.stringify({ messages, session_id: sessionId }),
    });
    if (!res.ok) throw new Error(`${res.status}`);
    const reader = res.body?.getReader();
    if (!reader) return;
    const decoder = new TextDecoder();
    let buf = "";
    let currentEvent = "";
    while (true) {
      const { done, value } = await reader.read();
      if (done) break;
      buf += decoder.decode(value, { stream: true });
      const lines = buf.split("\n");
      buf = lines.pop() || "";
      for (const line of lines) {
        if (line.startsWith("event: ")) {
          currentEvent = line.slice(7).trim();
        } else if (line.startsWith("data: ")) {
          const raw = line.slice(6);
          if (raw === "[DONE]") return;
          if (currentEvent === "error") {
            try {
              const err = JSON.parse(raw);
              throw new Error(err.message || raw);
            } catch (e) {
              if (e instanceof Error && e.message !== raw) throw e;
              throw new Error(raw);
            }
          }
          if (currentEvent === "done") {
            // Parse final summary which may include emotion data
            try {
              const done = JSON.parse(raw);
              if (done.emotion) {
                yield `\n<!--emotion:${JSON.stringify(done.emotion)}-->`;
              }
              if (done.sticker_suggestion) {
                yield `\n<!--sticker:${JSON.stringify(done.sticker_suggestion)}-->`;
              }
              if (done.sticker_suggestions) {
                yield `\n<!--stickers:${JSON.stringify(done.sticker_suggestions)}-->`;
              }
            } catch { /* ignore parse errors */ }
            continue;
          }
          // Parse delta or step events
          try {
            const parsed = JSON.parse(raw);
            if (parsed.content) {
              yield parsed.content;
            } else if (parsed.id && parsed.domain) {
              yield raw;
            }
          } catch {
            if (raw.trim()) yield raw;
          }
        } else if (line.trim() === "") {
          currentEvent = ""; // reset on empty line (SSE event boundary)
        }
      }
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
  kbIngest: (name: string, content: string) =>
    fetcher<{ source: KBSource; stats: KBStats }>("/v1/knowledge/ingest", { method: "POST", body: JSON.stringify({ name, content }) }),
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

  // Speech (TTS)
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

  // Reflection Loop — experiences and strategies
  getExperiences: (opts?: { source?: string; category?: string; outcome?: string; q?: string; stats?: boolean }) => {
    const params = new URLSearchParams();
    if (opts?.source) params.set("source", opts.source);
    if (opts?.category) params.set("category", opts.category);
    if (opts?.outcome) params.set("outcome", opts.outcome);
    if (opts?.q) params.set("q", opts.q);
    if (opts?.stats) params.set("stats", "true");
    return fetcher<ExperienceStats | { experiences: ExperienceItem[]; total: number }>(`/v1/reflect/experiences?${params}`);
  },
  getStrategies: () =>
    fetcher<{ strategies: string }>("/v1/reflect/strategies"),

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
};

// --- Types ---

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
  started_at: string;
  completed_at?: string;
  duration?: string;
}

export interface BotInfo {
  id: string;
  name: string;
  description: string;
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

// Knowledge
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

// Cron
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

// Tools
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

// Audit
export interface AuditRecord {
  index: number;
  timestamp: string;
  action: string;
  actor: string;
  detail: string;
  hash: string;
}

export interface AuditStats {
  total: number;
  first_at?: string;
  last_at?: string;
  actors: Record<string, number>;
}

// Reverie
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

// Settings
export interface ConfigField {
  key: string;
  label: string;
  label_zh: string;
  type: "text" | "password" | "select" | "number";
  placeholder?: string;
  options?: string[];
  sensitive?: boolean;
  required?: boolean;
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
  api_ok: boolean;
  setup_needed: boolean;
}

// SkillHub
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

// Backup
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

// Conversation management
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

// Sticker URL utilities
export function lineStickerUrl(stickerId: string): string {
  return `https://stickershop.line-scdn.net/stickershop/v1/sticker/${stickerId}/iPhone/sticker.png`;
}

export function parseStickerPattern(text: string): { packageId: string; stickerId: string } | null {
  const match = text.match(/\[贴图: packageId=(\d+), stickerId=(\d+)\]/);
  if (!match) return null;
  return { packageId: match[1], stickerId: match[2] };
}

// Task Runtime
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
}

export interface TaskInfo {
  id: string;
  title: string;
  description: string;
  status: string;
  steps: TaskStep[] | null;
  artifacts?: Array<{ path: string; type: string }>;
  error?: string;
  tenant_id: string;
  created_at: string;
  updated_at: string;
  started_at?: string;
  finished_at?: string;
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

// State Kernel
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

// Plugin UI tab
export interface PluginUITab {
  key: string;
  label: string;
  label_en?: string;
  icon: string;
  description?: string;
  plugin: string;
}

// QQ Chat Analyzer
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

// Task Thread
export interface LLMMessage {
  role: string;
  content: string;
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

// Task Working Memory
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

// Cost
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

// Triggers
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

// Trigger V2 — Unified types
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
