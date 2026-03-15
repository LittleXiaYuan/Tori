"use client";

import { useEffect, useState, useCallback, useRef } from "react";
import { useRouter, useSearchParams } from "next/navigation";
import { Suspense } from "react";
import {
  api,
  type TaskInfo,
  type TaskWorkingMemory,
  type CostTaskSummary,
  type CostUsageEvent,
  type TaskThreadInfo,
  type LLMMessage,
  type ThreadState,
} from "@/lib/api";
import { BlurFade } from "@/components/ui/blur-fade";
import {
  ArrowLeft,
  CheckCircle2,
  Clock,
  Loader2,
  XCircle,
  RefreshCw,
  Play,
  Pause,
  PlayCircle,
  Lock,
  Send,
  MessageSquare,
  FileText,
  DollarSign,
  GitBranch,
  Info,
  AlertTriangle,
  Trash2,
  Zap,
} from "lucide-react";

/* ── Helpers ── */

const statusColor: Record<string, string> = {
  pending: "#9ca3af",
  planning: "#a78bfa",
  running: "#3b82f6",
  paused: "#f59e0b",
  completed: "#22c55e",
  failed: "#ef4444",
  cancelled: "#f59e0b",
  interrupted: "#f97316",
};

const stepStatusIcon: Record<string, React.ReactNode> = {
  pending: <Clock size={14} className="text-gray-400" />,
  running: <Loader2 size={14} className="text-blue-400 animate-spin" />,
  retrying: <RefreshCw size={14} className="text-amber-400 animate-spin" />,
  done: <CheckCircle2 size={14} className="text-green-400" />,
  failed: <XCircle size={14} className="text-red-400" />,
  skipped: <Clock size={14} className="text-gray-300" />,
};

const threadStateColor: Record<string, string> = {
  open: "#22c55e",
  paused: "#f59e0b",
  closed: "#9ca3af",
};

function fmtTime(ts?: string): string {
  if (!ts) return "—";
  return new Date(ts).toLocaleString("zh-CN", {
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
    second: "2-digit",
  });
}

function dur(start?: string, end?: string): string {
  if (!start) return "—";
  const s = new Date(start).getTime();
  const e = end ? new Date(end).getTime() : Date.now();
  const d = Math.max(0, e - s);
  if (d < 1000) return `${d}ms`;
  if (d < 60000) return `${(d / 1000).toFixed(1)}s`;
  return `${(d / 60000).toFixed(1)}min`;
}

/* ── Tabs ── */

type TabKey = "overview" | "execution" | "thread" | "artifacts" | "cost";

const tabDefs: { key: TabKey; label: string; icon: React.ReactNode }[] = [
  { key: "overview", label: "概览", icon: <Info size={14} /> },
  { key: "execution", label: "执行流", icon: <GitBranch size={14} /> },
  { key: "thread", label: "线程", icon: <MessageSquare size={14} /> },
  { key: "artifacts", label: "产物", icon: <FileText size={14} /> },
  { key: "cost", label: "成本", icon: <DollarSign size={14} /> },
];

/* ── Overview Panel ── */

function OverviewPanel({ task, wm }: { task: TaskInfo; wm: TaskWorkingMemory | null }) {
  const steps = task.steps || [];
  const doneSteps = steps.filter((s) => s.status === "done").length;

  return (
    <div className="space-y-4">
      {/* Status metrics */}
      <div className="rounded-xl p-5 border" style={{ background: "var(--bg-card)", borderColor: "var(--border)" }}>
        <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
          <div>
            <div className="text-xs mb-1" style={{ color: "var(--text-muted)" }}>状态</div>
            <span
              className="text-sm px-2.5 py-1 rounded-full font-medium"
              style={{ background: `${statusColor[task.status] || "#9ca3af"}20`, color: statusColor[task.status] || "#9ca3af" }}
            >
              {task.status}
            </span>
          </div>
          <div>
            <div className="text-xs mb-1" style={{ color: "var(--text-muted)" }}>步骤进度</div>
            <div className="text-sm font-medium">{doneSteps}/{steps.length}</div>
          </div>
          <div>
            <div className="text-xs mb-1" style={{ color: "var(--text-muted)" }}>创建时间</div>
            <div className="text-sm">{fmtTime(task.created_at)}</div>
          </div>
          <div>
            <div className="text-xs mb-1" style={{ color: "var(--text-muted)" }}>耗时</div>
            <div className="text-sm">{dur(task.started_at, task.finished_at)}</div>
          </div>
        </div>
        {steps.length > 0 && (
          <div className="mt-4">
            <div className="h-2 rounded-full" style={{ background: "var(--border)" }}>
              <div
                className="h-full rounded-full transition-all duration-500"
                style={{ width: `${(doneSteps / steps.length) * 100}%`, background: task.status === "failed" ? "#ef4444" : "#22c55e" }}
              />
            </div>
          </div>
        )}
      </div>

      {/* Description */}
      <div className="rounded-xl p-4 border" style={{ background: "var(--bg-card)", borderColor: "var(--border)" }}>
        <div className="text-xs font-medium mb-2" style={{ color: "var(--text-muted)" }}>任务描述</div>
        <div className="text-sm whitespace-pre-wrap">{task.description || "—"}</div>
      </div>

      {/* Error */}
      {task.error && (
        <div className="rounded-xl p-4 border border-red-500/30" style={{ background: "#ef444410" }}>
          <div className="flex items-center gap-2 mb-2">
            <AlertTriangle size={14} className="text-red-400" />
            <span className="text-sm font-medium text-red-400">错误</span>
          </div>
          <div className="text-sm text-red-300">{task.error}</div>
        </div>
      )}

      {/* Working Memory */}
      {wm && wm.Goal && (
        <div className="rounded-xl p-4 border" style={{ background: "var(--bg-card)", borderColor: "var(--border)" }}>
          <div className="flex items-center gap-2 mb-3">
            <Zap size={14} style={{ color: "var(--accent)" }} />
            <span className="text-sm font-medium">工作记忆</span>
            <span className="text-xs" style={{ color: "var(--text-muted)" }}>~{wm.TokenEstimate} tokens</span>
          </div>
          <div className="space-y-2 text-sm">
            <div><span className="font-medium">目标：</span>{wm.Goal}</div>
            {wm.CompletedWork?.length > 0 && (
              <div>
                <span className="font-medium">已完成：</span>
                <ul className="list-disc list-inside mt-1">
                  {wm.CompletedWork.map((w, i) => <li key={i} className="text-xs" style={{ color: "var(--text-muted)" }}>{w}</li>)}
                </ul>
              </div>
            )}
            {wm.Blockers?.length > 0 && (
              <div>
                <span className="font-medium text-red-400">阻塞：</span>
                <ul className="list-disc list-inside mt-1">
                  {wm.Blockers.map((b, i) => <li key={i} className="text-xs text-red-300">{b}</li>)}
                </ul>
              </div>
            )}
            {wm.Confirmed?.length > 0 && (
              <div>
                <span className="font-medium text-green-400">已确认：</span>
                <ul className="list-disc list-inside mt-1">
                  {wm.Confirmed.map((c, i) => <li key={i} className="text-xs text-green-300">{c}</li>)}
                </ul>
              </div>
            )}
            {wm.Pending?.length > 0 && (
              <div>
                <span className="font-medium text-amber-400">待确认：</span>
                <ul className="list-disc list-inside mt-1">
                  {wm.Pending.map((p, i) => <li key={i} className="text-xs text-amber-300">{p}</li>)}
                </ul>
              </div>
            )}
            {wm.Artifacts?.length > 0 && (
              <div>
                <span className="font-medium">产物：</span>
                <ul className="list-disc list-inside mt-1">
                  {wm.Artifacts.map((a, i) => <li key={i} className="text-xs" style={{ color: "var(--text-muted)" }}>{a}</li>)}
                </ul>
              </div>
            )}
            {wm.NextAction && <div><span className="font-medium">下一步：</span>{wm.NextAction}</div>}
          </div>
        </div>
      )}
    </div>
  );
}

/* ── Execution Flow Panel ── */

function ExecutionPanel({ task }: { task: TaskInfo }) {
  const steps = task.steps || [];
  if (steps.length === 0) {
    return (
      <div className="text-center py-12" style={{ color: "var(--text-muted)" }}>
        <GitBranch size={32} className="mx-auto mb-2 opacity-30" />
        <p className="text-sm">暂无执行步骤</p>
        <p className="text-xs mt-1">运行任务后自动生成</p>
      </div>
    );
  }

  return (
    <div className="relative">
      <div className="absolute left-[19px] top-4 bottom-4 w-0.5" style={{ background: "var(--border)" }} />
      <div className="space-y-1">
        {steps.map((step, idx) => (
          <div key={step.id} className="relative flex gap-3">
            {/* Timeline dot */}
            <div className="flex-shrink-0 w-10 flex items-start justify-center pt-4 z-10">
              <div
                className="w-6 h-6 rounded-full flex items-center justify-center"
                style={{
                  background: step.status === "done" ? "#22c55e20" : step.status === "failed" ? "#ef444420" : step.status === "running" ? "#3b82f620" : "var(--bg-card)",
                  border: "2px solid var(--border)",
                }}
              >
                {stepStatusIcon[step.status] || stepStatusIcon.pending}
              </div>
            </div>
            {/* Step card */}
            <div className="flex-1 rounded-xl p-4 border mb-1" style={{ background: "var(--bg-card)", borderColor: "var(--border)" }}>
              <div className="flex items-center justify-between mb-1">
                <div className="flex items-center gap-2">
                  <span className="text-sm font-medium">Step {step.id}</span>
                  <span
                    className="text-xs px-2 py-0.5 rounded-full"
                    style={{
                      background: `${statusColor[step.status === "done" ? "completed" : step.status] || "#9ca3af"}20`,
                      color: statusColor[step.status === "done" ? "completed" : step.status] || "#9ca3af",
                    }}
                  >
                    {step.status}
                  </span>
                  {step.skill_name && (
                    <span className="text-xs px-2 py-0.5 rounded" style={{ background: "var(--accent)15", color: "var(--accent)" }}>
                      {step.skill_name}
                    </span>
                  )}
                  {step.gap_type && <span className="text-xs px-2 py-0.5 rounded bg-amber-500/20 text-amber-400">gap: {step.gap_type}</span>}
                </div>
                <div className="flex items-center gap-2 text-xs" style={{ color: "var(--text-muted)" }}>
                  {step.retry_count ? <span className="text-amber-400">retry ×{step.retry_count}</span> : null}
                  <span>{dur(step.started_at, step.done_at)}</span>
                </div>
              </div>
              <div className="text-sm" style={{ color: "var(--text-muted)" }}>{step.action}</div>
              {step.result && (
                <div className="mt-2 text-xs p-3 rounded-lg whitespace-pre-wrap break-words max-h-40 overflow-y-auto" style={{ background: "var(--bg-main)", color: "var(--text-muted)" }}>
                  {step.result.length > 800 ? step.result.slice(0, 800) + "…" : step.result}
                </div>
              )}
              {step.error && (
                <div className="mt-2 text-xs p-3 rounded-lg text-red-400" style={{ background: "#ef444410" }}>{step.error}</div>
              )}
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}

/* ── Thread Tab ── */

function ThreadTab({ taskId }: { taskId: string }) {
  const [info, setInfo] = useState<TaskThreadInfo | null>(null);
  const [messages, setMessages] = useState<LLMMessage[]>([]);
  const [input, setInput] = useState("");
  const [sending, setSending] = useState(false);
  const [loading, setLoading] = useState(true);
  const endRef = useRef<HTMLDivElement>(null);

  const load = useCallback(async () => {
    try {
      const data = await api.getTaskThread(taskId);
      setInfo(data.info);
      setMessages(data.messages || []);
    } catch { /* ignore */ } finally { setLoading(false); }
  }, [taskId]);

  useEffect(() => { load(); const id = setInterval(load, 3000); return () => clearInterval(id); }, [load]);
  useEffect(() => { endRef.current?.scrollIntoView({ behavior: "smooth" }); }, [messages]);

  const send = async () => {
    const text = input.trim();
    if (!text || sending) return;
    setSending(true); setInput("");
    try { await api.postTaskThread(taskId, text); await load(); } catch { /* ignore */ } finally { setSending(false); }
  };

  const changeState = async (s: ThreadState) => {
    try { await api.updateThreadState(taskId, s); await load(); } catch { /* ignore */ }
  };

  const isClosed = info?.state === "closed";
  const isPaused = info?.state === "paused";

  return (
    <div className="rounded-xl border flex flex-col" style={{ background: "var(--bg-card)", borderColor: "var(--border)", height: "500px" }}>
      {/* Header */}
      <div className="flex items-center gap-2 px-4 py-3 border-b flex-shrink-0" style={{ borderColor: "var(--border)" }}>
        <MessageSquare size={14} style={{ color: "var(--accent)" }} />
        <span className="text-sm font-medium flex-1">任务线程</span>
        {info && (
          <>
            <span className="text-xs px-2 py-0.5 rounded-full" style={{ background: `${threadStateColor[info.state] || "#9ca3af"}20`, color: threadStateColor[info.state] || "#9ca3af" }}>
              {info.state === "open" ? "活跃" : info.state === "paused" ? "已暂停" : "已关闭"}
            </span>
            {info.binding && <span className="text-xs px-2 py-0.5 rounded" style={{ background: "var(--accent)15", color: "var(--accent)" }}>{info.binding.channel_type}</span>}
            <span className="text-xs" style={{ color: "var(--text-muted)" }}>{info.messages} 条</span>
          </>
        )}
        <div className="flex gap-1">
          {!isClosed && !isPaused && <button onClick={() => changeState("paused")} className="p-1 rounded hover:bg-white/10" title="暂停"><Pause size={14} className="text-amber-400" /></button>}
          {isPaused && <button onClick={() => changeState("open")} className="p-1 rounded hover:bg-white/10" title="恢复"><PlayCircle size={14} className="text-green-400" /></button>}
          {!isClosed && <button onClick={() => changeState("closed")} className="p-1 rounded hover:bg-white/10" title="关闭"><Lock size={14} className="text-gray-400" /></button>}
        </div>
      </div>

      {/* Messages */}
      <div className="flex-1 overflow-y-auto px-4 py-3 space-y-2">
        {loading ? (
          <div className="flex items-center justify-center h-full"><Loader2 size={20} className="animate-spin" style={{ color: "var(--text-muted)" }} /></div>
        ) : messages.length === 0 ? (
          <div className="flex flex-col items-center justify-center h-full" style={{ color: "var(--text-muted)" }}>
            <MessageSquare size={28} className="mb-2 opacity-30" />
            <p className="text-sm">暂无消息</p>
          </div>
        ) : (
          messages.map((msg, i) => {
            const isUser = msg.role === "user";
            const isSystem = msg.role === "system";
            return (
              <div key={i} className="flex" style={{ justifyContent: isUser ? "flex-end" : isSystem ? "center" : "flex-start" }}>
                <div
                  className={`max-w-[75%] px-3 py-2 rounded-xl text-sm whitespace-pre-wrap break-words ${isSystem ? "w-full text-center" : ""}`}
                  style={{ background: isSystem ? "transparent" : isUser ? "var(--accent)" : "#374151", color: isSystem ? "var(--text-muted)" : "#fff", fontSize: isSystem ? "12px" : "14px", fontStyle: isSystem ? "italic" : "normal" }}
                >
                  {!isSystem && <div className="text-xs opacity-60 mb-0.5">{isUser ? "👤" : "🤖"} {msg.role}</div>}
                  {msg.content}
                </div>
              </div>
            );
          })
        )}
        <div ref={endRef} />
      </div>

      {/* Input */}
      <div className="px-4 py-3 border-t flex-shrink-0" style={{ borderColor: "var(--border)" }}>
        {isClosed ? (
          <div className="text-center text-sm py-1" style={{ color: "var(--text-muted)" }}><Lock size={14} className="inline mr-1" />线程已关闭</div>
        ) : (
          <div className="flex gap-2">
            <input className="flex-1 px-3 py-2 rounded-lg text-sm border bg-transparent" style={{ borderColor: "var(--border)" }} placeholder={isPaused ? "线程已暂停" : "输入消息…"} disabled={isPaused} value={input} onChange={(e) => setInput(e.target.value)} onKeyDown={(e) => e.key === "Enter" && !e.shiftKey && send()} />
            <button onClick={send} disabled={!input.trim() || sending || isPaused} className="px-3 py-2 rounded-lg disabled:opacity-50" style={{ background: "var(--accent)", color: "#fff" }}>
              {sending ? <Loader2 size={16} className="animate-spin" /> : <Send size={16} />}
            </button>
          </div>
        )}
      </div>
    </div>
  );
}

/* ── Artifacts Tab ── */

function ArtifactsPanel({ task }: { task: TaskInfo }) {
  const artifacts = task.artifacts || [];
  if (artifacts.length === 0) {
    return (
      <div className="text-center py-12" style={{ color: "var(--text-muted)" }}>
        <FileText size={32} className="mx-auto mb-2 opacity-30" />
        <p className="text-sm">暂无产物</p>
      </div>
    );
  }
  return (
    <div className="space-y-2">
      {artifacts.map((a, i) => (
        <div key={i} className="flex items-center gap-3 rounded-xl p-4 border" style={{ background: "var(--bg-card)", borderColor: "var(--border)" }}>
          <FileText size={18} style={{ color: "var(--accent)" }} />
          <div className="flex-1 min-w-0">
            <div className="text-sm font-medium truncate">{a.path}</div>
            <div className="text-xs" style={{ color: "var(--text-muted)" }}>{a.type}</div>
          </div>
        </div>
      ))}
    </div>
  );
}

/* ── Cost Tab (enhanced with timeline + breakdowns) ── */

function CostPanel({ cost, timeline }: { cost: CostTaskSummary | null; timeline: CostUsageEvent[] }) {
  if (!cost) {
    return (
      <div className="text-center py-12" style={{ color: "var(--text-muted)" }}>
        <DollarSign size={32} className="mx-auto mb-2 opacity-30" />
        <p className="text-sm">暂无成本数据</p>
      </div>
    );
  }
  const bySkill = cost.by_skill ? Object.entries(cost.by_skill).sort((a, b) => b[1] - a[1]) : [];
  const byModel = cost.by_model ? Object.entries(cost.by_model).sort((a, b) => b[1] - a[1]) : [];
  const totalCost = cost.total_cost_usd || 0;

  return (
    <div className="space-y-4">
      {/* Summary cards */}
      <div className="rounded-xl p-5 border" style={{ background: "var(--bg-card)", borderColor: "var(--border)" }}>
        <div className="grid grid-cols-2 md:grid-cols-5 gap-4">
          <div>
            <div className="text-xs mb-1" style={{ color: "var(--text-muted)" }}>总费用</div>
            <div className="text-2xl font-bold" style={{ color: "var(--accent)" }}>${totalCost.toFixed(4)}</div>
          </div>
          <div>
            <div className="text-xs mb-1" style={{ color: "var(--text-muted)" }}>输入 Tokens</div>
            <div className="text-lg font-semibold">{(cost.total_tokens_in || 0).toLocaleString()}</div>
          </div>
          <div>
            <div className="text-xs mb-1" style={{ color: "var(--text-muted)" }}>输出 Tokens</div>
            <div className="text-lg font-semibold">{(cost.total_tokens_out || 0).toLocaleString()}</div>
          </div>
          <div>
            <div className="text-xs mb-1" style={{ color: "var(--text-muted)" }}>调用次数</div>
            <div className="text-lg font-semibold">{cost.calls || 0}</div>
          </div>
          <div>
            <div className="text-xs mb-1" style={{ color: "var(--text-muted)" }}>平均延迟</div>
            <div className="text-lg font-semibold">{cost.avg_latency_ms || 0}ms</div>
          </div>
        </div>
      </div>

      {/* Breakdowns row */}
      <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
        {/* By Skill */}
        {bySkill.length > 0 && (
          <div className="rounded-xl p-4 border" style={{ background: "var(--bg-card)", borderColor: "var(--border)" }}>
            <div className="text-xs font-medium mb-3" style={{ color: "var(--text-muted)" }}>按技能分解</div>
            <div className="space-y-2">
              {bySkill.map(([skill, usd]) => (
                <div key={skill} className="flex items-center gap-2">
                  <div className="flex-1 min-w-0">
                    <div className="flex justify-between items-center mb-1">
                      <span className="text-sm truncate">{skill}</span>
                      <span className="text-xs font-mono">${usd.toFixed(4)}</span>
                    </div>
                    <div className="h-1.5 rounded-full" style={{ background: "var(--border)" }}>
                      <div className="h-full rounded-full" style={{ width: `${totalCost > 0 ? (usd / totalCost) * 100 : 0}%`, background: "var(--accent)" }} />
                    </div>
                  </div>
                </div>
              ))}
            </div>
          </div>
        )}

        {/* By Model */}
        {byModel.length > 0 && (
          <div className="rounded-xl p-4 border" style={{ background: "var(--bg-card)", borderColor: "var(--border)" }}>
            <div className="text-xs font-medium mb-3" style={{ color: "var(--text-muted)" }}>按模型分解</div>
            <div className="space-y-2">
              {byModel.map(([model, usd]) => (
                <div key={model} className="flex items-center gap-2">
                  <div className="flex-1 min-w-0">
                    <div className="flex justify-between items-center mb-1">
                      <span className="text-sm truncate">{model}</span>
                      <span className="text-xs font-mono">${usd.toFixed(4)}</span>
                    </div>
                    <div className="h-1.5 rounded-full" style={{ background: "var(--border)" }}>
                      <div className="h-full rounded-full" style={{ width: `${totalCost > 0 ? (usd / totalCost) * 100 : 0}%`, background: "#a78bfa" }} />
                    </div>
                  </div>
                </div>
              ))}
            </div>
          </div>
        )}
      </div>

      {/* Cost Timeline */}
      {timeline.length > 0 && (
        <div className="rounded-xl p-4 border" style={{ background: "var(--bg-card)", borderColor: "var(--border)" }}>
          <div className="text-xs font-medium mb-3" style={{ color: "var(--text-muted)" }}>成本时间线</div>
          <div className="overflow-x-auto">
            <table className="w-full text-sm" style={{ borderCollapse: "collapse" }}>
              <thead>
                <tr style={{ borderBottom: "1px solid var(--border)" }}>
                  <th className="text-left py-2 px-2 text-xs font-medium" style={{ color: "var(--text-muted)" }}>步骤</th>
                  <th className="text-left py-2 px-2 text-xs font-medium" style={{ color: "var(--text-muted)" }}>技能</th>
                  <th className="text-left py-2 px-2 text-xs font-medium" style={{ color: "var(--text-muted)" }}>模型</th>
                  <th className="text-right py-2 px-2 text-xs font-medium" style={{ color: "var(--text-muted)" }}>Tokens</th>
                  <th className="text-right py-2 px-2 text-xs font-medium" style={{ color: "var(--text-muted)" }}>费用</th>
                  <th className="text-right py-2 px-2 text-xs font-medium" style={{ color: "var(--text-muted)" }}>延迟</th>
                </tr>
              </thead>
              <tbody>
                {timeline.map((evt, i) => (
                  <tr key={i} style={{ borderBottom: "1px solid var(--border)" }}>
                    <td className="py-2 px-2 text-xs font-mono">{evt.step_id || "—"}</td>
                    <td className="py-2 px-2 text-xs">{evt.skill_name || "LLM"}</td>
                    <td className="py-2 px-2 text-xs" style={{ color: "var(--text-muted)" }}>{evt.model}</td>
                    <td className="py-2 px-2 text-xs text-right font-mono">{(evt.tokens_in + evt.tokens_out).toLocaleString()}</td>
                    <td className="py-2 px-2 text-xs text-right font-mono" style={{ color: "var(--accent)" }}>${evt.cost_usd.toFixed(4)}</td>
                    <td className="py-2 px-2 text-xs text-right" style={{ color: "var(--text-muted)" }}>{Math.round(evt.latency / 1000000)}ms</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>
      )}
    </div>
  );
}

/* ── Main Detail Content ── */

function TaskDetailContent() {
  const router = useRouter();
  const searchParams = useSearchParams();
  const taskId = searchParams.get("id") || "";

  const [task, setTask] = useState<TaskInfo | null>(null);
  const [wm, setWm] = useState<TaskWorkingMemory | null>(null);
  const [cost, setCost] = useState<CostTaskSummary | null>(null);
  const [costTimeline, setCostTimeline] = useState<CostUsageEvent[]>([]);
  const [activeTab, setActiveTab] = useState<TabKey>("overview");
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");

  const refresh = useCallback(async () => {
    if (!taskId) { setError("缺少任务 ID"); setLoading(false); return; }
    try {
      const t = await api.taskGet(taskId);
      setTask(t);
      const [wmData, costData, tlData] = await Promise.all([
        api.getTaskWorkingMemory(taskId).catch(() => null),
        api.getCostByTask(taskId).catch(() => null),
        api.getCostTaskTimeline(taskId).catch(() => []),
      ]);
      setWm(wmData);
      setCost(costData);
      setCostTimeline(tlData || []);
    } catch {
      setError("任务不存在或加载失败");
    } finally {
      setLoading(false);
    }
  }, [taskId]);

  useEffect(() => {
    refresh();
    const id = setInterval(refresh, 5000);
    return () => clearInterval(id);
  }, [refresh]);

  const handleRun = async () => { if (!task) return; await api.taskRun(task.id); setTimeout(refresh, 500); };
  const handleCancel = async () => { if (!task) return; await api.taskCancel(task.id); setTimeout(refresh, 500); };
  const handlePause = async () => { if (!task) return; await api.taskPause(task.id); setTimeout(refresh, 500); };
  const handleResume = async () => { if (!task) return; await api.taskResume(task.id); setTimeout(refresh, 500); };
  const handleRestart = async () => { if (!task) return; await api.taskRestart(task.id); setTimeout(refresh, 500); };
  const handleDelete = async () => { if (!task) return; await api.taskDelete(task.id); router.push("/tasks"); };

  if (loading) {
    return <div className="flex items-center justify-center h-[60vh]"><Loader2 size={28} className="animate-spin" style={{ color: "var(--text-muted)" }} /></div>;
  }

  if (error || !task) {
    return (
      <div className="max-w-4xl mx-auto px-4 py-8">
        <div className="text-center py-16" style={{ color: "var(--text-muted)" }}>
          <AlertTriangle size={48} className="mx-auto mb-3 opacity-30" />
          <p>{error || "任务未找到"}</p>
          <button onClick={() => router.push("/tasks")} className="mt-4 px-4 py-2 rounded-lg text-sm" style={{ background: "var(--accent)", color: "#fff" }}>
            返回任务列表
          </button>
        </div>
      </div>
    );
  }

  return (
    <div className="max-w-5xl mx-auto px-4 py-8">
      {/* Header */}
      <BlurFade delay={0.05}>
        <div className="flex items-center gap-3 mb-6">
          <button onClick={() => router.push("/tasks")} className="p-2 rounded-lg hover:bg-white/10 transition-colors" title="返回">
            <ArrowLeft size={18} />
          </button>
          <div className="flex-1 min-w-0">
            <div className="flex items-center gap-3">
              <h1 className="text-xl font-bold truncate">{task.title}</h1>
              <span className="text-xs px-2.5 py-1 rounded-full font-medium flex-shrink-0" style={{ background: `${statusColor[task.status] || "#9ca3af"}20`, color: statusColor[task.status] || "#9ca3af" }}>
                {task.status}
              </span>
            </div>
            <div className="text-xs mt-0.5" style={{ color: "var(--text-muted)" }}>
              ID: {task.id} · {fmtTime(task.created_at)}
            </div>
          </div>
          <div className="flex items-center gap-2 flex-shrink-0">
            <button onClick={refresh} className="p-2 rounded-lg hover:bg-white/10" title="刷新"><RefreshCw size={16} /></button>
            {task.status === "pending" && (
              <button onClick={handleRun} className="flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-sm font-medium" style={{ background: "#22c55e", color: "#fff" }}>
                <Play size={14} /> 运行
              </button>
            )}
            {(task.status === "running" || task.status === "planning") && (
              <>
                <button onClick={handlePause} className="flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-sm font-medium" style={{ background: "#a78bfa", color: "#fff" }}>
                  <Pause size={14} /> 暂停
                </button>
                <button onClick={handleCancel} className="flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-sm font-medium" style={{ background: "#f59e0b", color: "#fff" }}>
                  <XCircle size={14} /> 取消
                </button>
              </>
            )}
            {(task.status === "paused" || task.status === "interrupted" || task.status === "failed") && (
              <button onClick={handleResume} className="flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-sm font-medium" style={{ background: "#3b82f6", color: "#fff" }}>
                <PlayCircle size={14} /> 恢复
              </button>
            )}
            {(task.status === "completed" || task.status === "failed" || task.status === "cancelled" || task.status === "paused" || task.status === "interrupted") && (
              <button onClick={handleRestart} className="flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-sm font-medium" style={{ background: "#6366f1", color: "#fff" }}>
                <RefreshCw size={14} /> 重启
              </button>
            )}
            {(task.status === "completed" || task.status === "failed" || task.status === "cancelled") && (
              <button onClick={handleDelete} className="flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-sm" style={{ color: "var(--text-muted)" }}>
                <Trash2 size={14} /> 删除
              </button>
            )}
          </div>
        </div>
      </BlurFade>

      {/* Tabs */}
      <BlurFade delay={0.1}>
        <div className="flex gap-1 mb-6 p-1 rounded-xl border" style={{ background: "var(--bg-card)", borderColor: "var(--border)" }}>
          {tabDefs.map((t) => (
            <button
              key={t.key}
              onClick={() => setActiveTab(t.key)}
              className="flex items-center gap-1.5 px-4 py-2 rounded-lg text-sm transition-all"
              style={{ background: activeTab === t.key ? "var(--accent)" : "transparent", color: activeTab === t.key ? "#fff" : "var(--text-muted)", fontWeight: activeTab === t.key ? 600 : 400 }}
            >
              {t.icon} {t.label}
            </button>
          ))}
        </div>
      </BlurFade>

      {/* Tab Content */}
      <BlurFade delay={0.15}>
        {activeTab === "overview" && <OverviewPanel task={task} wm={wm} />}
        {activeTab === "execution" && <ExecutionPanel task={task} />}
        {activeTab === "thread" && <ThreadTab taskId={task.id} />}
        {activeTab === "artifacts" && <ArtifactsPanel task={task} />}
        {activeTab === "cost" && <CostPanel cost={cost} timeline={costTimeline} />}
      </BlurFade>
    </div>
  );
}

/* ── Page (with Suspense for useSearchParams) ── */

export default function TaskDetailPage() {
  return (
    <Suspense fallback={<div className="flex items-center justify-center h-[60vh]"><Loader2 size={28} className="animate-spin" style={{ color: "var(--text-muted)" }} /></div>}>
      <TaskDetailContent />
    </Suspense>
  );
}
