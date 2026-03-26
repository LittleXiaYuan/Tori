"use client";

import { useEffect, useState, useCallback } from "react";
import { api, type CronJob, type TriggerDef, type TriggerRun, type TriggerLogEvent, type TriggerAction } from "@/lib/api";
import { useI18n } from "@/lib/i18n";
import { BlurFade } from "@/components/ui/blur-fade";
import {
  Zap, Clock, Plus, X, Trash2, Play, Timer,
  ChevronDown, ChevronRight, Brain, Radio, FlaskConical,
  Pause, RotateCcw, Activity, History,
} from "lucide-react";

type Tab = "cron" | "triggers";
type TrigSubTab = "list" | "runs" | "events";

const triggerTypeIcon: Record<string, React.ReactNode> = {
  time: <Clock size={14} className="text-blue-400" />,
  event: <Radio size={14} className="text-green-400" />,
  condition: <FlaskConical size={14} className="text-amber-400" />,
  cognitive: <Brain size={14} className="text-purple-400" />,
};

const actionTypeLabels: Record<string, string> = {
  create_task: "创建任务",
  continue_task: "继续任务",
  send_message: "发消息",
  call_skill: "调用技能",
  write_memory: "写记忆",
  run_workflow: "执行工作流",
};

const statusColor: Record<string, string> = {
  active: "#22c55e",
  paused: "#f59e0b",
  disabled: "#6b7280",
  completed: "#22c55e",
  failed: "#ef4444",
  running: "#3b82f6",
  skipped: "#9ca3af",
};

export default function AutomationPage() {
  const { t } = useI18n();
  const [tab, setTab] = useState<Tab>("cron");
  const [trigSubTab, setTrigSubTab] = useState<TrigSubTab>("list");

  // ── Cron state ──
  const [jobs, setJobs] = useState<CronJob[]>([]);
  const [cronLoading, setCronLoading] = useState(true);
  const [showAddCron, setShowAddCron] = useState(false);
  const [cronName, setCronName] = useState("");
  const [schedType, setSchedType] = useState<"every" | "cron">("every");
  const [everyMin, setEveryMin] = useState("60");
  const [cronExpr, setCronExpr] = useState("*/5 * * * *");
  const [cronMsg, setCronMsg] = useState("");
  const [runOutput, setRunOutput] = useState<string | null>(null);

  // ── Trigger V2 state ──
  const [triggers, setTriggers] = useState<TriggerDef[]>([]);
  const [trigRuns, setTrigRuns] = useState<TriggerRun[]>([]);
  const [trigEvents, setTrigEvents] = useState<TriggerLogEvent[]>([]);
  const [trigLoading, setTrigLoading] = useState(true);
  const [expanded, setExpanded] = useState<Set<string>>(new Set());
  const [showAddTrig, setShowAddTrig] = useState(false);
  const [trigForm, setTrigForm] = useState({
    name: "", description: "",
    type: "event" as TriggerDef["type"],
    tenant_id: "default",
    // Event config
    event_type: "task_completed",
    source_id: "",
    // Condition config
    check_type: "cost_threshold",
    target_id: "day",
    operator: "gt",
    cond_value: "10",
    // Cognitive config
    source_type: "reverie_insight",
    min_significance: "0.6",
    // Time config
    cron_expression: "",
    interval: "",
    // Actions
    action_type: "create_task" as TriggerAction["type"],
    action_task_title: "",
    action_task_desc: "",
    action_message: "",
    action_skill: "",
    action_memory: "",
    // Channel
    channel_id: "",
    thread_id: "",
  });

  // ── Load data ──
  const loadCron = useCallback(() => {
    api.cronList().then(r => setJobs(r.jobs || [])).catch(() => {}).finally(() => setCronLoading(false));
  }, []);
  const loadTriggers = useCallback(() => {
    setTrigLoading(true);
    Promise.all([
      api.getTriggersV2(),
      api.getTriggerRuns(undefined, 50),
      api.getTriggerEvents(undefined, 100),
    ]).then(([t, r, e]) => {
      setTriggers(t.triggers || []);
      setTrigRuns(r.runs || []);
      setTrigEvents(e.events || []);
    }).catch(() => {}).finally(() => setTrigLoading(false));
  }, []);

  useEffect(() => { loadCron(); loadTriggers(); }, [loadCron, loadTriggers]);

  // ── Cron actions ──
  const addCron = async () => {
    if (!cronName.trim() || !cronMsg.trim()) return;
    const schedule = schedType === "every"
      ? { type: "every", every_ms: parseInt(everyMin) * 60000 }
      : { type: "cron", cron_expr: cronExpr };
    await api.cronAdd(cronName, schedule, { kind: "agentTurn", message: cronMsg });
    setShowAddCron(false); setCronName(""); setCronMsg(""); loadCron();
  };
  const runCron = async (id: string) => {
    const r = await api.cronRun(id);
    setRunOutput(r.run.output || r.run.error || "No output"); loadCron();
  };
  const removeCron = async (id: string) => { await api.cronRemove(id); loadCron(); };

  // ── Trigger V2 actions ──
  const createTrigV2 = async () => {
    if (!trigForm.name) return;
    const def: Partial<TriggerDef> = {
      name: trigForm.name,
      description: trigForm.description || undefined,
      type: trigForm.type,
      status: "active",
      tenant_id: trigForm.tenant_id || "default",
      channel_id: trigForm.channel_id || undefined,
      thread_id: trigForm.thread_id || undefined,
      actions: [buildAction()],
    };

    // Config by type
    if (trigForm.type === "event") {
      def.event_config = { event_type: trigForm.event_type, source_id: trigForm.source_id || undefined };
    } else if (trigForm.type === "condition") {
      def.condition_config = {
        check_type: trigForm.check_type, target_id: trigForm.target_id || undefined,
        operator: trigForm.operator, value: trigForm.cond_value,
      };
    } else if (trigForm.type === "cognitive") {
      def.cognitive_config = {
        source_type: trigForm.source_type,
        min_significance: parseFloat(trigForm.min_significance) || 0.6,
      };
    } else if (trigForm.type === "time") {
      def.time_config = {
        cron_expr: trigForm.cron_expression || undefined,
        interval: trigForm.interval || undefined,
      };
    }

    await api.createTriggerV2(def);
    setShowAddTrig(false);
    loadTriggers();
  };

  const buildAction = (): TriggerAction => {
    const base: TriggerAction = { type: trigForm.action_type };
    switch (trigForm.action_type) {
      case "create_task":
        base.task_title = trigForm.action_task_title;
        base.task_description = trigForm.action_task_desc;
        break;
      case "continue_task":
        base.task_id = trigForm.target_id;
        base.message = trigForm.action_message;
        break;
      case "send_message":
        base.message = trigForm.action_message;
        break;
      case "call_skill":
        base.skill_name = trigForm.action_skill;
        break;
      case "write_memory":
        base.memory_content = trigForm.action_memory;
        break;
    }
    return base;
  };

  const deleteTrigV2 = async (id: string) => { await api.deleteTriggerV2(id); loadTriggers(); };
  const toggleExpand = (id: string) =>
    setExpanded(prev => { const s = new Set(prev); s.has(id) ? s.delete(id) : s.add(id); return s; });
  const fmtTime = (ts?: string) => ts ? new Date(ts).toLocaleString("zh-CN") : "—";

  return (
    <div className="animate-in">
      {/* Header */}
      <div className="flex items-center justify-between mb-6">
        <div className="flex items-center gap-3">
          <div className="w-10 h-10 rounded-xl flex items-center justify-center" style={{ background: "var(--accent-subtle)" }}>
            <Zap size={20} style={{ color: "var(--accent)" }} />
          </div>
          <div>
            <h1 className="text-xl font-semibold tracking-tight">{t("automation.title")}</h1>
            <p className="text-xs mt-0.5" style={{ color: "var(--text-muted)" }}>
              {jobs.length} {t("automation.jobs")} · {triggers.length} {t("automation.triggers")} · {trigRuns.length} runs
            </p>
          </div>
        </div>
        <button
          onClick={() => tab === "cron" ? setShowAddCron(!showAddCron) : setShowAddTrig(!showAddTrig)}
          className="btn-glow px-4 py-2.5 rounded-xl text-xs font-medium flex items-center gap-1.5"
        >
          {(tab === "cron" ? showAddCron : showAddTrig) ? <X size={13} /> : <Plus size={13} />}
          {tab === "cron" ? t("cron.addJob") : t("triggers.create")}
        </button>
      </div>

      {/* Main Tabs */}
      <div className="flex gap-1 mb-6 p-1 rounded-xl" style={{ background: "var(--bg-hover)" }}>
        {([["cron", <Clock key="c" size={14} />, t("automation.tabCron")],
           ["triggers", <Zap key="t" size={14} />, t("automation.tabTriggers")]] as [Tab, React.ReactNode, string][]).map(([key, icon, label]) => (
          <button key={key} onClick={() => setTab(key)}
            className="flex-1 flex items-center justify-center gap-2 px-4 py-2.5 rounded-lg text-xs font-medium transition-all"
            style={{
              background: tab === key ? "var(--bg-card)" : "transparent",
              color: tab === key ? "var(--accent)" : "var(--text-muted)",
              boxShadow: tab === key ? "var(--shadow-sm)" : "none",
            }}>
            {icon} {label}
          </button>
        ))}
      </div>

      {/* ── Cron Tab ── */}
      {tab === "cron" && (
        <>
          {showAddCron && (
            <BlurFade delay={0.1}>
              <div className="rounded-xl border p-6 mb-6 space-y-4" style={{ background: "var(--bg-card)", borderColor: "var(--border)", boxShadow: "var(--shadow-md)" }}>
                <input value={cronName} onChange={e => setCronName(e.target.value)} placeholder={t("cron.jobName")}
                  className="w-full px-4 py-3 rounded-xl border text-sm outline-none" style={{ background: "var(--bg)", borderColor: "var(--border)", color: "var(--text)" }} />
                <div className="flex gap-3 items-center">
                  <select value={schedType} onChange={e => setSchedType(e.target.value as "every" | "cron")}
                    className="px-4 py-3 rounded-xl border text-sm outline-none" style={{ background: "var(--bg)", borderColor: "var(--border)", color: "var(--text)" }}>
                    <option value="every">{t("cron.everyNMin")}</option>
                    <option value="cron">{t("cron.cronExpr")}</option>
                  </select>
                  {schedType === "every" ? (
                    <input value={everyMin} onChange={e => setEveryMin(e.target.value)} type="number" min="1" placeholder="Minutes"
                      className="w-28 px-4 py-3 rounded-xl border text-sm outline-none" style={{ background: "var(--bg)", borderColor: "var(--border)", color: "var(--text)" }} />
                  ) : (
                    <input value={cronExpr} onChange={e => setCronExpr(e.target.value)} placeholder="*/5 * * * *"
                      className="flex-1 px-4 py-3 rounded-xl border text-sm outline-none font-mono" style={{ background: "var(--bg)", borderColor: "var(--border)", color: "var(--text)" }} />
                  )}
                </div>
                <input value={cronMsg} onChange={e => setCronMsg(e.target.value)} placeholder={t("cron.prompt")}
                  className="w-full px-4 py-3 rounded-xl border text-sm outline-none" style={{ background: "var(--bg)", borderColor: "var(--border)", color: "var(--text)" }} />
                <button onClick={addCron} className="btn-glow px-5 py-3 rounded-xl text-sm font-medium">{t("cron.create")}</button>
              </div>
            </BlurFade>
          )}

          {runOutput && (
            <div className="animate-in rounded-xl border p-5 mb-5 relative" style={{ background: "var(--bg-card)", borderColor: "var(--accent)", boxShadow: "var(--shadow-glow)" }}>
              <button onClick={() => setRunOutput(null)} className="absolute top-4 right-4 p-1 rounded-lg hover:bg-[var(--bg-hover)]" style={{ color: "var(--text-muted)" }}><X size={14} /></button>
              <div className="text-[11px] uppercase tracking-wider mb-2 font-medium" style={{ color: "var(--accent)" }}>Output</div>
              <pre className="text-xs font-mono whitespace-pre-wrap max-h-48 overflow-auto" style={{ color: "var(--text-secondary)" }}>{runOutput}</pre>
            </div>
          )}

          {cronLoading ? (
            <div className="space-y-3"><div className="skeleton h-20 w-full" /><div className="skeleton h-20 w-full" /></div>
          ) : (
            <div className="space-y-2 stagger">
              {jobs.map(j => (
                <div key={j.id} className="card-hover animate-in rounded-xl border px-5 py-4 flex items-center gap-4" style={{ background: "var(--bg-card)", borderColor: "var(--border)" }}>
                  <div className="w-9 h-9 rounded-lg flex items-center justify-center" style={{ background: j.enabled ? "var(--accent-subtle)" : "var(--bg-hover)" }}>
                    <Timer size={16} style={{ color: j.enabled ? "var(--accent)" : "var(--text-muted)" }} />
                  </div>
                  <div className="flex-1 min-w-0">
                    <div className="text-sm font-medium">{j.name}</div>
                    <div className="text-xs flex gap-3 mt-1" style={{ color: "var(--text-muted)" }}>
                      <span className="badge" style={{ background: "var(--bg-hover)" }}>
                        {j.schedule.type === "every" ? `${(j.schedule.every_ms || 0) / 60000}m` : j.schedule.cron_expr}
                      </span>
                      <span>{t("cron.runs")}: {j.run_count}</span>
                      <span>{t("cron.last")}: {fmtTime(j.last_run_at)}</span>
                    </div>
                  </div>
                  <button onClick={() => runCron(j.id)} className="p-2.5 rounded-lg hover:bg-[var(--accent-subtle)]" style={{ color: "var(--accent)" }} title="Run"><Play size={15} /></button>
                  <button onClick={() => removeCron(j.id)} className="p-2.5 rounded-lg hover:bg-[var(--danger-bg)]" style={{ color: "var(--text-muted)" }} title="Delete"><Trash2 size={15} /></button>
                </div>
              ))}
              {jobs.length === 0 && (
                <div className="text-sm text-center py-16 rounded-xl border" style={{ color: "var(--text-muted)", borderColor: "var(--border)", borderStyle: "dashed" }}>
                  <Clock size={32} className="mx-auto mb-3 opacity-30" />{t("cron.noJobs")}
                </div>
              )}
            </div>
          )}
        </>
      )}

      {/* ── Triggers V2 Tab ── */}
      {tab === "triggers" && (
        <>
          {/* Sub-tabs */}
          <div className="flex gap-2 mb-4">
            {([["list", <Zap key="l" size={13} />, `触发器 (${triggers.length})`],
               ["runs", <Activity key="r" size={13} />, `执行记录 (${trigRuns.length})`],
               ["events", <History key="e" size={13} />, `事件日志`]] as [TrigSubTab, React.ReactNode, string][]).map(([key, icon, label]) => (
              <button key={key} onClick={() => setTrigSubTab(key)}
                className="px-3 py-1.5 rounded-lg text-xs font-medium flex items-center gap-1.5 transition-all"
                style={{
                  background: trigSubTab === key ? "var(--accent-subtle)" : "transparent",
                  color: trigSubTab === key ? "var(--accent)" : "var(--text-muted)",
                }}>
                {icon} {label}
              </button>
            ))}
          </div>

          {/* ── Create Form ── */}
          {showAddTrig && trigSubTab === "list" && (
            <BlurFade delay={0.1}>
              <div className="rounded-xl border p-5 mb-6 space-y-4" style={{ background: "var(--bg-card)", borderColor: "var(--border)" }}>
                <div className="grid grid-cols-2 gap-3">
                  <div>
                    <label className="text-xs mb-1 block" style={{ color: "var(--text-muted)" }}>名称</label>
                    <input className="w-full px-3 py-2 rounded-lg text-sm border bg-transparent" style={{ borderColor: "var(--border)" }}
                      value={trigForm.name} onChange={e => setTrigForm({ ...trigForm, name: e.target.value })} />
                  </div>
                  <div>
                    <label className="text-xs mb-1 block" style={{ color: "var(--text-muted)" }}>类型</label>
                    <select className="w-full px-3 py-2 rounded-lg text-sm border bg-transparent" style={{ borderColor: "var(--border)" }}
                      value={trigForm.type} onChange={e => setTrigForm({ ...trigForm, type: e.target.value as TriggerDef["type"] })}>
                      <option value="time">⏰ 时间触发</option>
                      <option value="event">📡 事件触发</option>
                      <option value="condition">🧪 条件触发</option>
                      <option value="cognitive">🧠 认知触发</option>
                    </select>
                  </div>
                </div>

                {/* Type-specific config */}
                {trigForm.type === "time" && (
                  <div className="grid grid-cols-2 gap-3">
                    <div>
                      <label className="text-xs mb-1 block" style={{ color: "var(--text-muted)" }}>Cron 表达式</label>
                      <input className="w-full px-3 py-2 rounded-lg text-sm border bg-transparent font-mono" style={{ borderColor: "var(--border)" }}
                        value={trigForm.cron_expression} onChange={e => setTrigForm({ ...trigForm, cron_expression: e.target.value })} placeholder="0 9 * * *" />
                    </div>
                    <div>
                      <label className="text-xs mb-1 block" style={{ color: "var(--text-muted)" }}>或间隔</label>
                      <input className="w-full px-3 py-2 rounded-lg text-sm border bg-transparent" style={{ borderColor: "var(--border)" }}
                        value={trigForm.interval} onChange={e => setTrigForm({ ...trigForm, interval: e.target.value })} placeholder="1h, 30m" />
                    </div>
                  </div>
                )}

                {trigForm.type === "event" && (
                  <div className="grid grid-cols-2 gap-3">
                    <div>
                      <label className="text-xs mb-1 block" style={{ color: "var(--text-muted)" }}>事件类型</label>
                      <select className="w-full px-3 py-2 rounded-lg text-sm border bg-transparent" style={{ borderColor: "var(--border)" }}
                        value={trigForm.event_type} onChange={e => setTrigForm({ ...trigForm, event_type: e.target.value })}>
                        <option value="task_completed">task_completed</option>
                        <option value="task_failed">task_failed</option>
                        <option value="task_status_changed">task_status_changed</option>
                        <option value="memory_updated">memory_updated</option>
                        <option value="knowledge_updated">knowledge_updated</option>
                        <option value="cost_alert">cost_alert</option>
                        <option value="skill_installed">skill_installed</option>
                        <option value="reverie_insight">reverie_insight</option>
                        <option value="emotion_shift">emotion_shift</option>
                      </select>
                    </div>
                    <div>
                      <label className="text-xs mb-1 block" style={{ color: "var(--text-muted)" }}>源 ID (可选)</label>
                      <input className="w-full px-3 py-2 rounded-lg text-sm border bg-transparent" style={{ borderColor: "var(--border)" }}
                        value={trigForm.source_id} onChange={e => setTrigForm({ ...trigForm, source_id: e.target.value })} placeholder="task-xxx" />
                    </div>
                  </div>
                )}

                {trigForm.type === "condition" && (
                  <div className="grid grid-cols-4 gap-3">
                    <div>
                      <label className="text-xs mb-1 block" style={{ color: "var(--text-muted)" }}>检查类型</label>
                      <select className="w-full px-3 py-2 rounded-lg text-sm border bg-transparent" style={{ borderColor: "var(--border)" }}
                        value={trigForm.check_type} onChange={e => setTrigForm({ ...trigForm, check_type: e.target.value })}>
                        <option value="task_status">任务状态</option>
                        <option value="cost_threshold">成本阈值</option>
                        <option value="memory_count">记忆数量</option>
                        <option value="custom">自定义</option>
                      </select>
                    </div>
                    <div>
                      <label className="text-xs mb-1 block" style={{ color: "var(--text-muted)" }}>目标 ID</label>
                      <input className="w-full px-3 py-2 rounded-lg text-sm border bg-transparent" style={{ borderColor: "var(--border)" }}
                        value={trigForm.target_id} onChange={e => setTrigForm({ ...trigForm, target_id: e.target.value })} placeholder="day / task-id" />
                    </div>
                    <div>
                      <label className="text-xs mb-1 block" style={{ color: "var(--text-muted)" }}>运算符</label>
                      <select className="w-full px-3 py-2 rounded-lg text-sm border bg-transparent" style={{ borderColor: "var(--border)" }}
                        value={trigForm.operator} onChange={e => setTrigForm({ ...trigForm, operator: e.target.value })}>
                        <option value="eq">= (eq)</option>
                        <option value="neq">≠ (neq)</option>
                        <option value="gt">&gt; (gt)</option>
                        <option value="lt">&lt; (lt)</option>
                        <option value="gte">≥ (gte)</option>
                        <option value="lte">≤ (lte)</option>
                        <option value="contains">contains</option>
                      </select>
                    </div>
                    <div>
                      <label className="text-xs mb-1 block" style={{ color: "var(--text-muted)" }}>值</label>
                      <input className="w-full px-3 py-2 rounded-lg text-sm border bg-transparent" style={{ borderColor: "var(--border)" }}
                        value={trigForm.cond_value} onChange={e => setTrigForm({ ...trigForm, cond_value: e.target.value })} placeholder="10" />
                    </div>
                  </div>
                )}

                {trigForm.type === "cognitive" && (
                  <div className="grid grid-cols-2 gap-3">
                    <div>
                      <label className="text-xs mb-1 block" style={{ color: "var(--text-muted)" }}>认知来源</label>
                      <select className="w-full px-3 py-2 rounded-lg text-sm border bg-transparent" style={{ borderColor: "var(--border)" }}
                        value={trigForm.source_type} onChange={e => setTrigForm({ ...trigForm, source_type: e.target.value })}>
                        <option value="reverie_insight">Reverie 高洞察</option>
                        <option value="emotion_shift">情绪变化</option>
                      </select>
                    </div>
                    <div>
                      <label className="text-xs mb-1 block" style={{ color: "var(--text-muted)" }}>最低显著度</label>
                      <input className="w-full px-3 py-2 rounded-lg text-sm border bg-transparent" style={{ borderColor: "var(--border)" }}
                        type="number" step="0.1" min="0" max="1"
                        value={trigForm.min_significance} onChange={e => setTrigForm({ ...trigForm, min_significance: e.target.value })} />
                    </div>
                  </div>
                )}

                {/* Action config */}
                <div className="border-t pt-4" style={{ borderColor: "var(--border)" }}>
                  <div className="text-xs font-medium mb-3" style={{ color: "var(--text-muted)" }}>动作配置</div>
                  <div className="grid grid-cols-2 gap-3">
                    <div>
                      <label className="text-xs mb-1 block" style={{ color: "var(--text-muted)" }}>动作类型</label>
                      <select className="w-full px-3 py-2 rounded-lg text-sm border bg-transparent" style={{ borderColor: "var(--border)" }}
                        value={trigForm.action_type} onChange={e => setTrigForm({ ...trigForm, action_type: e.target.value as TriggerAction["type"] })}>
                        <option value="create_task">创建任务</option>
                        <option value="continue_task">继续任务</option>
                        <option value="send_message">发送消息</option>
                        <option value="call_skill">调用技能</option>
                        <option value="write_memory">写记忆</option>
                        <option value="run_workflow">执行工作流</option>
                      </select>
                    </div>
                    {trigForm.action_type === "create_task" && (
                      <div>
                        <label className="text-xs mb-1 block" style={{ color: "var(--text-muted)" }}>任务标题</label>
                        <input className="w-full px-3 py-2 rounded-lg text-sm border bg-transparent" style={{ borderColor: "var(--border)" }}
                          value={trigForm.action_task_title} onChange={e => setTrigForm({ ...trigForm, action_task_title: e.target.value })} />
                      </div>
                    )}
                    {(trigForm.action_type === "send_message" || trigForm.action_type === "continue_task") && (
                      <div>
                        <label className="text-xs mb-1 block" style={{ color: "var(--text-muted)" }}>消息</label>
                        <input className="w-full px-3 py-2 rounded-lg text-sm border bg-transparent" style={{ borderColor: "var(--border)" }}
                          value={trigForm.action_message} onChange={e => setTrigForm({ ...trigForm, action_message: e.target.value })} />
                      </div>
                    )}
                    {trigForm.action_type === "call_skill" && (
                      <div>
                        <label className="text-xs mb-1 block" style={{ color: "var(--text-muted)" }}>技能名称</label>
                        <input className="w-full px-3 py-2 rounded-lg text-sm border bg-transparent" style={{ borderColor: "var(--border)" }}
                          value={trigForm.action_skill} onChange={e => setTrigForm({ ...trigForm, action_skill: e.target.value })} />
                      </div>
                    )}
                    {trigForm.action_type === "write_memory" && (
                      <div>
                        <label className="text-xs mb-1 block" style={{ color: "var(--text-muted)" }}>记忆内容</label>
                        <input className="w-full px-3 py-2 rounded-lg text-sm border bg-transparent" style={{ borderColor: "var(--border)" }}
                          value={trigForm.action_memory} onChange={e => setTrigForm({ ...trigForm, action_memory: e.target.value })} />
                      </div>
                    )}
                  </div>
                </div>

                <button onClick={createTrigV2} disabled={!trigForm.name}
                  className="px-4 py-2 rounded-lg text-xs font-medium transition-opacity disabled:opacity-40"
                  style={{ background: "var(--accent)", color: "#000" }}>
                  创建触发器
                </button>
              </div>
            </BlurFade>
          )}

          {/* ── Trigger List ── */}
          {trigSubTab === "list" && (
            trigLoading ? (
              <div className="text-center py-12" style={{ color: "var(--text-muted)" }}>Loading...</div>
            ) : triggers.length === 0 ? (
              <div className="text-sm text-center py-16 rounded-xl border" style={{ color: "var(--text-muted)", borderColor: "var(--border)", borderStyle: "dashed" }}>
                <Zap size={32} className="mx-auto mb-3 opacity-30" />{t("triggers.empty")}
              </div>
            ) : (
              <div className="space-y-2">
                {triggers.map((trig, i) => (
                  <BlurFade key={trig.id} delay={0.03 + i * 0.02}>
                    <div className="rounded-xl border overflow-hidden" style={{ background: "var(--bg-card)", borderColor: "var(--border)" }}>
                      <div className="flex items-center gap-3 px-4 py-3 cursor-pointer hover:opacity-80 transition-opacity"
                        onClick={() => toggleExpand(trig.id)}>
                        {expanded.has(trig.id) ? <ChevronDown size={14} /> : <ChevronRight size={14} />}
                        {triggerTypeIcon[trig.type]}
                        <div className="flex-1 min-w-0">
                          <span className="font-medium text-sm">{trig.name}</span>
                          {trig.description && <span className="text-xs ml-2" style={{ color: "var(--text-muted)" }}>{trig.description}</span>}
                        </div>
                        <div className="flex items-center gap-3" onClick={e => e.stopPropagation()}>
                          <span className="text-[10px] px-2 py-0.5 rounded-full" style={{ background: statusColor[trig.status] + "22", color: statusColor[trig.status] }}>
                            {trig.status}
                          </span>
                          <span className="text-xs" style={{ color: "var(--text-muted)" }}>{trig.run_count}x</span>
                          <button onClick={() => deleteTrigV2(trig.id)} className="p-1 rounded hover:bg-white/10 transition-colors">
                            <Trash2 size={13} className="text-gray-400" />
                          </button>
                        </div>
                      </div>
                      {expanded.has(trig.id) && (
                        <div className="px-4 py-3 border-t text-xs space-y-2" style={{ borderColor: "var(--border)", color: "var(--text-muted)" }}>
                          <div className="flex gap-6 flex-wrap">
                            <span><strong>类型:</strong> {trig.type}</span>
                            <span><strong>租户:</strong> {trig.tenant_id}</span>
                            {trig.channel_id && <span><strong>渠道:</strong> {trig.channel_id}</span>}
                            {trig.thread_id && <span><strong>线程:</strong> {trig.thread_id}</span>}
                            <span><strong>创建:</strong> {fmtTime(trig.created_at)}</span>
                            {trig.last_run_at && <span><strong>上次执行:</strong> {fmtTime(trig.last_run_at)}</span>}
                          </div>
                          {trig.event_config && (
                            <div><strong>事件:</strong> {trig.event_config.event_type} {trig.event_config.source_id && `(${trig.event_config.source_id})`}</div>
                          )}
                          {trig.condition_config && (
                            <div><strong>条件:</strong> {trig.condition_config.check_type} {trig.condition_config.operator} {trig.condition_config.value}</div>
                          )}
                          {trig.cognitive_config && (
                            <div><strong>认知:</strong> {trig.cognitive_config.source_type} (≥{trig.cognitive_config.min_significance})</div>
                          )}
                          {trig.time_config && (
                            <div><strong>时间:</strong> {trig.time_config.cron_expr || trig.time_config.interval}</div>
                          )}
                          <div className="border-t pt-2 mt-2" style={{ borderColor: "var(--border)" }}>
                            <strong>动作 ({trig.actions.length}):</strong>
                            {trig.actions.map((a, j) => (
                              <span key={j} className="inline-block ml-2 px-2 py-0.5 rounded text-[10px]" style={{ background: "var(--bg-hover)" }}>
                                {actionTypeLabels[a.type] || a.type}
                                {a.task_title && `: ${a.task_title}`}
                                {a.skill_name && `: ${a.skill_name}`}
                                {a.message && `: ${a.message.slice(0, 30)}`}
                              </span>
                            ))}
                          </div>
                          {trig.budget && (
                            <div><strong>预算:</strong> {trig.budget.max_runs_per_day && `${trig.budget.max_runs_per_day}/天`} {trig.budget.max_total_cost && `$${trig.budget.max_total_cost}`}</div>
                          )}
                          {trig.last_error && <div className="text-red-400"><strong>错误:</strong> {trig.last_error}</div>}
                        </div>
                      )}
                    </div>
                  </BlurFade>
                ))}
              </div>
            )
          )}

          {/* ── Runs List ── */}
          {trigSubTab === "runs" && (
            <div className="space-y-2">
              {trigRuns.length === 0 ? (
                <div className="text-sm text-center py-12" style={{ color: "var(--text-muted)" }}>暂无执行记录</div>
              ) : trigRuns.map((run, i) => (
                <BlurFade key={run.id} delay={0.03 + i * 0.02}>
                  <div className="rounded-xl border px-4 py-3" style={{ background: "var(--bg-card)", borderColor: "var(--border)" }}>
                    <div className="flex items-center gap-3">
                      <span className="text-[10px] px-2 py-0.5 rounded-full font-medium" style={{ background: statusColor[run.status] + "22", color: statusColor[run.status] }}>
                        {run.status}
                      </span>
                      <span className="text-xs font-mono" style={{ color: "var(--text-muted)" }}>{run.trigger_source}</span>
                      <span className="flex-1" />
                      <span className="text-xs" style={{ color: "var(--text-muted)" }}>
                        {run.actions_succeeded}/{run.actions_executed} ok
                        {run.total_cost ? ` · $${run.total_cost.toFixed(4)}` : ""}
                        {run.duration ? ` · ${run.duration}` : ""}
                      </span>
                      <span className="text-xs" style={{ color: "var(--text-muted)" }}>{fmtTime(run.started_at)}</span>
                    </div>
                    {run.error && <div className="text-xs text-red-400 mt-1">{run.error}</div>}
                    {run.action_results && run.action_results.length > 0 && (
                      <div className="flex gap-2 mt-2 flex-wrap">
                        {run.action_results.map((ar, j) => (
                          <span key={j} className="text-[10px] px-2 py-0.5 rounded" style={{
                            background: ar.status === "success" ? "#22c55e22" : "#ef444422",
                            color: ar.status === "success" ? "#22c55e" : "#ef4444",
                          }}>
                            {actionTypeLabels[ar.action_type] || ar.action_type}
                            {ar.result && ` → ${ar.result.slice(0, 20)}`}
                            {ar.error && ` ✗ ${ar.error.slice(0, 30)}`}
                          </span>
                        ))}
                      </div>
                    )}
                  </div>
                </BlurFade>
              ))}
            </div>
          )}

          {/* ── Events Log ── */}
          {trigSubTab === "events" && (
            <div className="space-y-1">
              {trigEvents.length === 0 ? (
                <div className="text-sm text-center py-12" style={{ color: "var(--text-muted)" }}>暂无事件日志</div>
              ) : trigEvents.map((evt, i) => (
                <BlurFade key={evt.id} delay={0.02 + i * 0.01}>
                  <div className="flex items-center gap-3 px-4 py-2 rounded-lg text-xs" style={{ color: "var(--text-muted)" }}>
                    <span className="w-20 text-[10px] font-mono">{evt.event_type}</span>
                    <span className="flex-1 truncate">{evt.message}</span>
                    <span className="text-[10px]">{fmtTime(evt.timestamp)}</span>
                  </div>
                </BlurFade>
              ))}
            </div>
          )}
        </>
      )}
    </div>
  );
}
