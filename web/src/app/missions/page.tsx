"use client";

import { useState, useEffect, useCallback, useRef } from "react";
import Link from "next/link";
import { api } from "@/lib/api";
import { BlurFade } from "@/components/ui/blur-fade";
import {
  Zap, Plus, Trash2, Play, Clock, CheckCircle2,
  XCircle, Pause, ChevronRight, ChevronDown,
  GitBranch, ListTodo, Layers, RefreshCw,
  Sparkles, Send, X, Radio, Brain, FlaskConical,
  Wrench, Eye, Calendar, Activity,
} from "lucide-react";

// ── Types ──

interface MissionItem {
  id: string;
  name: string;
  description: string;
  type: "task" | "workflow" | "cron" | "trigger" | "template";
  status: string;
  trigger_label: string;
  execution_label: string;
  run_count: number;
  last_run?: string;
  created_at: string;
  updated_at?: string;
  // source data for linking
  source_id: string;
  workflow_node_count?: number;
  template_vars?: string[];
}

type FilterTab = "all" | "active" | "scheduled" | "event" | "templates" | "completed";

const filterConfig: Record<FilterTab, { label: string; icon: React.ReactNode }> = {
  all: { label: "全部", icon: null },
  active: { label: "运行中", icon: <Play size={12} /> },
  scheduled: { label: "定时", icon: <Clock size={12} /> },
  event: { label: "事件驱动", icon: <Radio size={12} /> },
  templates: { label: "模板", icon: <Layers size={12} /> },
  completed: { label: "已完成", icon: <CheckCircle2 size={12} /> },
};

const typeIcon: Record<string, { icon: React.ReactNode; color: string; label: string }> = {
  task: { icon: <ListTodo size={16} />, color: "#3b82f6", label: "Agent 任务" },
  workflow: { icon: <GitBranch size={16} />, color: "#8b5cf6", label: "工作流" },
  cron: { icon: <Clock size={16} />, color: "#f59e0b", label: "定时任务" },
  trigger: { icon: <Zap size={16} />, color: "#22c55e", label: "触发器" },
  template: { icon: <Layers size={16} />, color: "#06b6d4", label: "模板" },
};

const statusColors: Record<string, string> = {
  running: "#3b82f6",
  active: "#22c55e",
  completed: "#22c55e",
  done: "#22c55e",
  pending: "#9ca3af",
  paused: "#f59e0b",
  failed: "#ef4444",
  disabled: "#6b7280",
  idle: "#9ca3af",
};

function fmtTime(ts?: string) {
  if (!ts) return "—";
  return new Date(ts).toLocaleString("zh-CN", { hour12: false, month: "2-digit", day: "2-digit", hour: "2-digit", minute: "2-digit" });
}

// ── Main Component ──

export default function MissionsPage() {
  const [missions, setMissions] = useState<MissionItem[]>([]);
  const [loading, setLoading] = useState(true);
  const [filter, setFilter] = useState<FilterTab>("all");
  const [showCreate, setShowCreate] = useState(false);
  const [nlInput, setNlInput] = useState("");
  const [nlLoading, setNlLoading] = useState(false);
  const nlRef = useRef<HTMLTextAreaElement>(null);
  const [parsedResult, setParsedResult] = useState<{
    type: string; name: string; description: string;
    config: Record<string, unknown>; confidence: number; explanation: string;
  } | null>(null);

  // Load data from all sources and unify
  const loadAll = useCallback(async () => {
    setLoading(true);
    try {
      const [tasksRes, wfRes, cronRes, trigRes, tplRes] = await Promise.all([
        fetch("/v1/tasks", { headers: apiHeaders() }).then(r => r.json()).catch(() => []),
        fetch("/v1/workflows", { headers: apiHeaders() }).then(r => r.json()).catch(() => ({ workflows: [] })),
        api.cronList().catch(() => ({ jobs: [] })),
        api.getTriggersV2().catch(() => ({ triggers: [] })),
        fetch("/v1/tasks/templates", { headers: apiHeaders() }).then(r => r.json()).catch(() => ({ templates: [] })),
      ]);

      const items: MissionItem[] = [];

      // Tasks
      const tasks = Array.isArray(tasksRes) ? tasksRes : (tasksRes.tasks || []);
      for (const t of tasks) {
        items.push({
          id: `task_${t.id}`,
          name: t.title || t.id,
          description: t.description || "",
          type: "task",
          status: t.status || "pending",
          trigger_label: "手动",
          execution_label: "Agent 自主规划",
          run_count: t.steps || 0,
          last_run: t.updated_at,
          created_at: t.created_at || new Date().toISOString(),
          updated_at: t.updated_at,
          source_id: t.id,
        });
      }

      // Workflows
      const wfs = wfRes.workflows || [];
      for (const w of wfs) {
        items.push({
          id: `wf_${w.id}`,
          name: w.name,
          description: w.description || "",
          type: "workflow",
          status: "idle",
          trigger_label: "手动 / API",
          execution_label: `${w.nodes?.length || 0} 步 DAG`,
          run_count: 0,
          created_at: w.created_at || new Date().toISOString(),
          updated_at: w.updated_at,
          source_id: w.id,
          workflow_node_count: w.nodes?.length || 0,
        });
      }

      // Cron jobs
      const jobs = cronRes.jobs || [];
      for (const j of jobs) {
        const sched = j.schedule?.type === "every"
          ? `每 ${Math.round((j.schedule.every_ms || 0) / 60000)} 分钟`
          : `${j.schedule?.cron_expr || "cron"}`;
        items.push({
          id: `cron_${j.id}`,
          name: j.name,
          description: j.payload?.message || "",
          type: "cron",
          status: j.enabled ? "active" : "paused",
          trigger_label: sched,
          execution_label: "Agent 对话",
          run_count: j.run_count || 0,
          last_run: j.last_run_at,
          created_at: j.created_at || new Date().toISOString(),
          source_id: j.id,
        });
      }

      // Triggers
      const trigs = trigRes.triggers || [];
      for (const trig of trigs) {
        const trigLabel = trig.type === "time" ? `${trig.time_config?.cron_expr || trig.time_config?.interval || "定时"}`
          : trig.type === "event" ? `${trig.event_config?.event_type || "事件"}`
          : trig.type === "condition" ? `${trig.condition_config?.check_type || "条件"}`
          : `${trig.cognitive_config?.source_type || "认知"}`;

        const actionLabels = (trig.actions || []).map((a: { type: string }) => {
          const map: Record<string, string> = {
            create_task: "创建任务", continue_task: "继续任务",
            send_message: "发消息", call_skill: "调技能",
            write_memory: "写记忆", run_workflow: "执行工作流",
          };
          return map[a.type] || a.type;
        }).join(" → ");

        items.push({
          id: `trig_${trig.id}`,
          name: trig.name,
          description: trig.description || "",
          type: "trigger",
          status: trig.status || "active",
          trigger_label: trigLabel,
          execution_label: actionLabels || "无动作",
          run_count: trig.run_count || 0,
          last_run: trig.last_run_at,
          created_at: trig.created_at || new Date().toISOString(),
          source_id: trig.id,
        });
      }

      // Templates
      const tpls = tplRes.templates || [];
      for (const tpl of tpls) {
        items.push({
          id: `tpl_${tpl.id}`,
          name: tpl.name || tpl.title,
          description: tpl.description || "",
          type: "template",
          status: "idle",
          trigger_label: "模板",
          execution_label: tpl.steps ? `${tpl.steps.length} 步骤` : "Agent 任务",
          run_count: 0,
          created_at: tpl.created_at || new Date().toISOString(),
          source_id: tpl.id,
          template_vars: tpl.variables ? Object.keys(tpl.variables) : [],
        });
      }

      // Sort: running first, then by updated_at desc
      items.sort((a, b) => {
        const aActive = ["running", "active"].includes(a.status) ? 1 : 0;
        const bActive = ["running", "active"].includes(b.status) ? 1 : 0;
        if (aActive !== bActive) return bActive - aActive;
        return new Date(b.updated_at || b.created_at).getTime() - new Date(a.updated_at || a.created_at).getTime();
      });

      setMissions(items);
    } catch (e) {
      console.error("Failed to load missions", e);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => { loadAll(); }, [loadAll]);

  // Filter
  const filtered = missions.filter(m => {
    if (filter === "all") return true;
    if (filter === "active") return ["running", "active"].includes(m.status);
    if (filter === "scheduled") return m.type === "cron" || (m.type === "trigger" && m.trigger_label.includes("⏰"));
    if (filter === "event") return m.type === "trigger" && !m.trigger_label.includes("⏰");
    if (filter === "templates") return m.type === "template";
    if (filter === "completed") return ["completed", "done"].includes(m.status);
    return true;
  });

  // NL Create: Phase 1 — Parse intent
  const handleNlCreate = async () => {
    const text = nlInput.trim();
    if (!text || nlLoading) return;
    setNlLoading(true);
    setParsedResult(null);
    try {
      const result = await api.missionParse(text);
      setParsedResult(result);
    } catch (e) {
      console.error("NL parse failed", e);
    } finally {
      setNlLoading(false);
    }
  };

  // NL Create: Phase 2 — Confirm and create
  const handleConfirmCreate = async () => {
    if (!parsedResult) return;
    setNlLoading(true);
    try {
      if (parsedResult.type === "cron") {
        const cronExpr = (parsedResult.config.cron_expr as string) || "0 9 * * *";
        const message = (parsedResult.config.message as string) || parsedResult.description;
        await api.cronAdd(parsedResult.name, { type: "cron", cron_expr: cronExpr }, { message });
      } else if (parsedResult.type === "trigger") {
        await api.createTriggerV2({
          name: parsedResult.name,
          description: parsedResult.description,
          type: (parsedResult.config.event_type ? "event" : "condition") as "event" | "condition",
          status: "active",
        });
      } else if (parsedResult.type === "workflow") {
        await fetch("/v1/workflows", {
          method: "POST", headers: apiHeaders(),
          body: JSON.stringify({ name: parsedResult.name, description: parsedResult.description }),
        });
      } else {
        await api.taskCreate(parsedResult.name, parsedResult.description);
      }
      setParsedResult(null);
      setNlInput("");
      setShowCreate(false);
      setTimeout(loadAll, 1000);
    } catch (e) {
      console.error("Mission create failed", e);
    } finally {
      setNlLoading(false);
    }
  };

  // Quick actions
  const handleDelete = async (m: MissionItem) => {
    try {
      if (m.type === "task") await fetch(`/v1/tasks?id=${m.source_id}`, { method: "DELETE", headers: apiHeaders() });
      else if (m.type === "workflow") await fetch(`/v1/workflows?id=${m.source_id}`, { method: "DELETE", headers: apiHeaders() });
      else if (m.type === "cron") await api.cronRemove(m.source_id);
      else if (m.type === "trigger") await api.deleteTriggerV2(m.source_id);
      else if (m.type === "template") await fetch(`/v1/tasks/templates?id=${m.source_id}`, { method: "DELETE", headers: apiHeaders() });
      loadAll();
    } catch { /* ignore */ }
  };

  const handleRun = async (m: MissionItem) => {
    try {
      if (m.type === "workflow") {
        await fetch("/v1/workflows/run", {
          method: "POST", headers: apiHeaders(),
          body: JSON.stringify({ definition_id: m.source_id }),
        });
      } else if (m.type === "cron") {
        await api.cronRun(m.source_id);
      } else if (m.type === "task") {
        await fetch("/v1/tasks/run", {
          method: "POST", headers: apiHeaders(),
          body: JSON.stringify({ id: m.source_id }),
        });
      }
      setTimeout(loadAll, 1000);
    } catch { /* ignore */ }
  };

  const handleCancel = async (m: MissionItem) => {
    try {
      if (m.type === "task") {
        await fetch("/v1/tasks/cancel", {
          method: "POST", headers: apiHeaders(),
          body: JSON.stringify({ id: m.source_id }),
        });
      }
      setTimeout(loadAll, 1000);
    } catch { /* ignore */ }
  };

  const handlePause = async (m: MissionItem) => {
    try {
      if (m.type === "task") {
        await fetch("/v1/tasks/pause", {
          method: "POST", headers: apiHeaders(),
          body: JSON.stringify({ id: m.source_id }),
        });
      }
      setTimeout(loadAll, 1000);
    } catch { /* ignore */ }
  };

  const getDetailLink = (m: MissionItem): string => {
    if (m.type === "workflow") return `/workflows/${m.source_id}`;
    if (m.type === "task") return `/task-run?id=${m.source_id}`;
    return "#";
  };

  const counts = {
    all: missions.length,
    active: missions.filter(m => ["running", "active"].includes(m.status)).length,
    scheduled: missions.filter(m => m.type === "cron" || (m.type === "trigger" && m.trigger_label.includes("⏰"))).length,
    event: missions.filter(m => m.type === "trigger" && !m.trigger_label.includes("⏰")).length,
    templates: missions.filter(m => m.type === "template").length,
    completed: missions.filter(m => ["completed", "done"].includes(m.status)).length,
  };

  return (
    <div className="animate-in">
      {/* Header */}
      <div className="flex items-center justify-between mb-6">
        <div className="flex items-center gap-3">
          <div className="w-10 h-10 rounded-xl flex items-center justify-center"
            style={{ background: "var(--accent-subtle)" }}>
            <Zap size={20} style={{ color: "var(--accent)" }} />
          </div>
          <div>
            <h1 className="text-xl font-semibold tracking-tight">任务中心</h1>
            <p className="text-xs mt-0.5" style={{ color: "var(--text-muted)" }}>
              {missions.length} 个编排 · {counts.active} 活跃 · {counts.scheduled} 定时
            </p>
          </div>
        </div>
        <div className="flex items-center gap-2">
          <button onClick={() => loadAll()} className="p-2.5 rounded-xl transition-colors"
            style={{ color: "var(--text-muted)" }} title="刷新">
            <RefreshCw size={16} className={loading ? "animate-spin" : ""} />
          </button>
          <button onClick={() => setShowCreate(!showCreate)}
            className="btn-glow px-4 py-2.5 rounded-xl text-xs font-medium flex items-center gap-1.5">
            {showCreate ? <X size={13} /> : <Plus size={13} />}
            新建编排
          </button>
        </div>
      </div>

      {/* NL Create Panel */}
      {showCreate && (
        <BlurFade delay={0.05}>
          <div className="rounded-xl border p-5 mb-6" style={{
            background: "var(--bg-card)",
            borderColor: "var(--border)",
            boxShadow: "var(--shadow-md)",
          }}>
            <div className="flex items-center gap-2 mb-3">
              <Sparkles size={16} style={{ color: "var(--accent)" }} />
              <span className="text-sm font-medium">用自然语言描述你的编排需求</span>
            </div>
            <div className="flex gap-3 items-end">
              <textarea
                ref={nlRef}
                value={nlInput}
                onChange={e => setNlInput(e.target.value)}
                placeholder="例如：每天早上9点自动生成销售日报并发送到钉钉群 / 当任务失败时发邮件通知 / 创建一个客户合同审批工作流..."
                rows={2}
                className="flex-1 resize-none rounded-xl px-4 py-3 text-sm outline-none border"
                style={{ background: "var(--bg)", borderColor: "var(--border)", color: "var(--text)" }}
                onKeyDown={e => { if (e.key === "Enter" && !e.shiftKey) { e.preventDefault(); handleNlCreate(); } }}
              />
              <button
                onClick={handleNlCreate}
                disabled={!nlInput.trim() || nlLoading}
                className="rounded-xl px-4 py-3 text-sm font-medium flex items-center gap-2 transition-all"
                style={{
                  background: nlInput.trim() && !nlLoading ? "var(--accent)" : "var(--bg-hover)",
                  color: nlInput.trim() && !nlLoading ? "white" : "var(--text-muted)",
                  cursor: nlInput.trim() && !nlLoading ? "pointer" : "not-allowed",
                }}
              >
                {nlLoading ? <RefreshCw size={14} className="animate-spin" /> : <Send size={14} />}
                创建
              </button>
            </div>

            {/* Quick create buttons */}
            <div className="flex gap-2 mt-3 flex-wrap">
              {[
                { label: "一次性任务", filter: "active" as FilterTab },
                { label: "定时任务", filter: "scheduled" as FilterTab },
                { label: "事件触发", filter: "event" as FilterTab },
                { label: "模板", filter: "templates" as FilterTab },
              ].map(item => (
                <button key={item.label}
                  className="px-3 py-1.5 rounded-lg text-xs border transition-all hover:scale-[1.02]"
                  style={{ borderColor: "var(--border)", color: "var(--text-muted)", background: "var(--bg)" }}
                  onClick={() => setFilter(item.filter)}>
                  {item.label}
                </button>
              ))}
            </div>

            {/* NL Parse Preview Panel */}
            {parsedResult && (
              <div className="mt-4 rounded-xl border p-4" style={{
                background: "var(--bg)",
                borderColor: parsedResult.confidence >= 0.7 ? "var(--accent)" : "var(--warning)",
                boxShadow: "var(--shadow-sm)",
              }}>
                <div className="flex items-center justify-between mb-3">
                  <div className="flex items-center gap-2">
                    <span style={{ color: typeIcon[parsedResult.type]?.color || "var(--text-muted)" }}>
                      {typeIcon[parsedResult.type]?.icon || <ListTodo size={16} />}
                    </span>
                    <span className="text-sm font-medium">{parsedResult.name}</span>
                    <span className="text-[10px] px-1.5 py-0.5 rounded-full font-medium"
                      style={{
                        background: (typeIcon[parsedResult.type]?.color || "#9ca3af") + "18",
                        color: typeIcon[parsedResult.type]?.color || "#9ca3af",
                      }}>
                      {typeIcon[parsedResult.type]?.label || parsedResult.type}
                    </span>
                  </div>
                  <span className="text-xs px-2 py-0.5 rounded-full font-medium"
                    style={{
                      background: parsedResult.confidence >= 0.7 ? "#22c55e18" : parsedResult.confidence >= 0.5 ? "#f59e0b18" : "#ef444418",
                      color: parsedResult.confidence >= 0.7 ? "#22c55e" : parsedResult.confidence >= 0.5 ? "#f59e0b" : "#ef4444",
                    }}>
                    {Math.round(parsedResult.confidence * 100)}% 置信
                  </span>
                </div>
                {parsedResult.description && (
                  <p className="text-xs mb-2" style={{ color: "var(--text-muted)" }}>{parsedResult.description}</p>
                )}
                {parsedResult.config && Object.keys(parsedResult.config).length > 0 && (
                  <div className="text-xs space-y-1 mb-3 px-3 py-2 rounded-lg" style={{ background: "var(--bg-hover)" }}>
                    {Object.entries(parsedResult.config).map(([k, v]) => (
                      <div key={k} className="flex gap-2">
                        <span className="font-mono" style={{ color: "var(--text-muted)" }}>{k}:</span>
                        <span style={{ color: "var(--text)" }}>{typeof v === "string" ? v : JSON.stringify(v)}</span>
                      </div>
                    ))}
                  </div>
                )}
                <p className="text-xs mb-3" style={{ color: "var(--text-muted)" }}>
                  💡 {parsedResult.explanation}
                </p>
                <div className="flex gap-2">
                  <button onClick={handleConfirmCreate} disabled={nlLoading}
                    className="px-4 py-1.5 rounded-lg text-xs font-medium flex items-center gap-1.5 transition-all"
                    style={{ background: "var(--accent)", color: "white", cursor: nlLoading ? "not-allowed" : "pointer" }}>
                    {nlLoading ? <RefreshCw size={12} className="animate-spin" /> : <CheckCircle2 size={12} />}
                    确认创建
                  </button>
                  <button onClick={() => setParsedResult(null)}
                    className="px-4 py-1.5 rounded-lg text-xs font-medium transition-all border"
                    style={{ borderColor: "var(--border)", color: "var(--text-muted)" }}>
                    取消
                  </button>
                </div>
              </div>
            )}
          </div>
        </BlurFade>
      )}

      {/* Filter Tabs */}
      <div className="flex gap-1 mb-5 p-1 rounded-xl overflow-x-auto" style={{ background: "var(--bg-hover)" }}>
        {(Object.entries(filterConfig) as [FilterTab, { label: string; icon: React.ReactNode }][]).map(([key, cfg]) => (
          <button key={key} onClick={() => setFilter(key)}
            className="flex items-center gap-1.5 px-3 py-2 rounded-lg text-xs font-medium transition-all whitespace-nowrap"
            style={{
              background: filter === key ? "var(--bg-card)" : "transparent",
              color: filter === key ? "var(--accent)" : "var(--text-muted)",
              boxShadow: filter === key ? "var(--shadow-sm)" : "none",
            }}>
            {cfg.icon} {cfg.label}
            {counts[key] > 0 && (
              <span className="ml-0.5 text-[10px] px-1.5 py-0.5 rounded-full"
                style={{ background: filter === key ? "var(--accent-subtle)" : "var(--bg-hover)" }}>
                {counts[key]}
              </span>
            )}
          </button>
        ))}
      </div>

      {/* Mission List */}
      {loading ? (
        <div className="space-y-3">
          {[1, 2, 3].map(i => <div key={i} className="skeleton h-24 w-full rounded-xl" />)}
        </div>
      ) : filtered.length === 0 ? (
        <div className="flex flex-col items-center justify-center py-20 rounded-xl border"
          style={{ borderColor: "var(--border)", borderStyle: "dashed", color: "var(--text-muted)" }}>
          <Zap size={40} className="mb-3 opacity-20" />
          <p className="text-sm">暂无编排任务</p>
          <p className="text-xs mt-1">点击「新建编排」用自然语言创建，或手动配置</p>
        </div>
      ) : (
        <div className="space-y-2">
          {filtered.map((m, i) => {
            const ti = typeIcon[m.type] || typeIcon.task;
            const sc = statusColors[m.status] || "#9ca3af";

            return (
              <BlurFade key={m.id} delay={0.03 + i * 0.015}>
                <div className="card-hover rounded-xl border px-5 py-4 transition-all"
                  style={{ background: "var(--bg-card)", borderColor: "var(--border)" }}>
                  <div className="flex items-center gap-4">
                    {/* Type icon */}
                    <div className="w-10 h-10 rounded-lg flex items-center justify-center shrink-0"
                      style={{ background: ti.color + "15" }}>
                      <span style={{ color: ti.color }}>{ti.icon}</span>
                    </div>

                    {/* Content */}
                    <div className="flex-1 min-w-0">
                      <div className="flex items-center gap-2">
                        <span className="text-sm font-medium truncate">{m.name}</span>
                        <span className="text-[10px] px-1.5 py-0.5 rounded-full font-medium"
                          style={{ background: sc + "18", color: sc }}>
                          {m.status}
                        </span>
                        <span className="text-[10px] px-1.5 py-0.5 rounded"
                          style={{ background: "var(--bg-hover)", color: "var(--text-muted)" }}>
                          {ti.label}
                        </span>
                      </div>
                      {m.description && (
                        <p className="text-xs mt-0.5 truncate max-w-md" style={{ color: "var(--text-muted)" }}>
                          {m.description}
                        </p>
                      )}
                      <div className="flex items-center gap-4 mt-1.5 text-[11px]" style={{ color: "var(--text-muted)" }}>
                        <span>{m.trigger_label}</span>
                        <span style={{ color: "var(--border)" }}>→</span>
                        <span>{m.execution_label}</span>
                        {m.run_count > 0 && (
                          <span className="flex items-center gap-1">
                            <Activity size={10} /> {m.run_count}次
                          </span>
                        )}
                        {m.last_run && (
                          <span className="flex items-center gap-1">
                            <Calendar size={10} /> {fmtTime(m.last_run)}
                          </span>
                        )}
                      </div>
                    </div>

                    {/* Actions */}
                    <div className="flex items-center gap-1.5 shrink-0">
                      {(m.type === "workflow" || m.type === "task" || m.type === "cron") && (
                        (m.status === "running" || m.status === "active") && m.type === "task" ? (
                          <>
                            <button onClick={() => handlePause(m)}
                              className="p-2 rounded-lg transition-colors hover:bg-[rgba(245,158,11,0.1)]"
                              style={{ color: "#f59e0b" }} title="暂停 (Pause)">
                              <Pause size={14} />
                            </button>
                            <button onClick={() => handleCancel(m)}
                              className="p-2 rounded-lg transition-colors hover:bg-[rgba(239,68,68,0.1)]"
                              style={{ color: "#ef4444" }} title="强行停止 (Stop)">
                              <XCircle size={14} />
                            </button>
                          </>
                        ) : (
                          <button onClick={() => handleRun(m)}
                            className="p-2 rounded-lg transition-colors hover:bg-[var(--accent-subtle)]"
                            style={{ color: "var(--accent)" }} title="执行">
                            <Play size={14} />
                          </button>
                        )
                      )}
                      {m.type === "workflow" && (
                        <Link href={getDetailLink(m)}
                          className="p-2 rounded-lg transition-colors hover:bg-[var(--accent-subtle)]"
                          style={{ color: "var(--text-muted)" }} title="可视化编辑">
                          <Eye size={14} />
                        </Link>
                      )}
                      {m.type === "task" && (
                        <Link href={getDetailLink(m)}
                          className="p-2 rounded-lg transition-colors"
                          style={{ color: "var(--text-muted)" }} title="查看详情">
                          <ChevronRight size={14} />
                        </Link>
                      )}
                      <button onClick={() => handleDelete(m)}
                        className="p-2 rounded-lg transition-colors hover:bg-[#ef444418]"
                        style={{ color: "var(--text-muted)" }} title="删除">
                        <Trash2 size={14} />
                      </button>
                    </div>
                  </div>
                </div>
              </BlurFade>
            );
          })}
        </div>
      )}
    </div>
  );
}

function apiHeaders(): Record<string, string> {
  const token = typeof window !== "undefined"
    ? localStorage.getItem("yunque_api_key") || localStorage.getItem("yunque_token") || ""
    : "";
  return { "Content-Type": "application/json", "X-API-Key": token };
}
