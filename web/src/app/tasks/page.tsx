"use client";

import { useEffect, useState, useCallback, useRef } from "react";
import { useRouter } from "next/navigation";
import {
  api,
  type TaskInfo,
  type StateSnapshot,
  type GapStats,
  type TaskThreadInfo,
  type LLMMessage,
  type ThreadState,
} from "@/lib/api";
import { BlurFade } from "@/components/ui/blur-fade";
import {
  ListTodo,
  Play,
  XCircle,
  Trash2,
  ChevronDown,
  ChevronRight,
  Target,
  Crosshair,
  Cpu,
  AlertTriangle,
  CheckCircle2,
  Clock,
  Loader2,
  RefreshCw,
  Plus,
  MessageSquare,
  Send,
  Pause,
  PlayCircle,
  Lock,
  X,
} from "lucide-react";

/* ── Helpers ── */

const statusColor: Record<string, string> = {
  pending: "#9ca3af",
  planning: "#a78bfa",
  running: "#3b82f6",
  completed: "#22c55e",
  failed: "#ef4444",
  cancelled: "#f59e0b",
};

const stepStatusIcon: Record<string, React.ReactNode> = {
  pending: <Clock size={14} className="text-gray-400" />,
  running: <Loader2 size={14} className="text-blue-400 animate-spin" />,
  retrying: <RefreshCw size={14} className="text-amber-400 animate-spin" />,
  done: <CheckCircle2 size={14} className="text-green-400" />,
  failed: <XCircle size={14} className="text-red-400" />,
  skipped: <Clock size={14} className="text-gray-300" />,
};

function relTime(ts?: string): string {
  if (!ts) return "";
  const d = Date.now() - new Date(ts).getTime();
  if (d < 60000) return `${Math.floor(d / 1000)}s ago`;
  if (d < 3600000) return `${Math.floor(d / 60000)}m ago`;
  return `${Math.floor(d / 3600000)}h ago`;
}

/* ── Components ── */

function StatePanel({ state }: { state: StateSnapshot | null }) {
  if (!state) return null;
  const activeGoals = (state.goals || []).filter((g) => g.status === "active");
  return (
    <div
      className="rounded-xl p-5 border mb-6"
      style={{ background: "var(--bg-card)", borderColor: "var(--border)" }}
    >
      <div className="flex items-center gap-2 mb-4">
        <Target size={18} style={{ color: "var(--accent)" }} />
        <span className="font-semibold">状态内核</span>
      </div>
      <div className="grid grid-cols-1 md:grid-cols-4 gap-4">
        {/* Focus */}
        <div>
          <div className="text-xs mb-1" style={{ color: "var(--text-muted)" }}>
            <Crosshair size={12} className="inline mr-1" />
            焦点
          </div>
          <div className="text-sm font-medium">{state.focus || "—"}</div>
          {state.topics?.length > 0 && (
            <div className="flex flex-wrap gap-1 mt-1">
              {state.topics.map((t) => (
                <span
                  key={t}
                  className="text-xs px-1.5 py-0.5 rounded"
                  style={{ background: "var(--accent)15", color: "var(--accent)" }}
                >
                  {t}
                </span>
              ))}
            </div>
          )}
        </div>
        {/* Goals */}
        <div>
          <div className="text-xs mb-1" style={{ color: "var(--text-muted)" }}>
            <Target size={12} className="inline mr-1" />
            活跃目标
          </div>
          {activeGoals.length === 0 ? (
            <span className="text-sm" style={{ color: "var(--text-muted)" }}>无</span>
          ) : (
            activeGoals.map((g) => (
              <div key={g.id} className="text-sm">
                <span className="font-medium">[P{g.priority}]</span> {g.title}
                {g.progress > 0 && (
                  <span style={{ color: "var(--accent)" }}> {Math.round(g.progress * 100)}%</span>
                )}
              </div>
            ))
          )}
        </div>
        {/* Capabilities */}
        <div>
          <div className="text-xs mb-1" style={{ color: "var(--text-muted)" }}>
            <Cpu size={12} className="inline mr-1" />
            能力
          </div>
          <div className="text-sm">
            {state.capabilities?.total_skills || 0} 技能
            {state.capabilities?.dynamic_skills?.length ? (
              <span style={{ color: "var(--accent)" }}>
                {" "}(+{state.capabilities.dynamic_skills.length} 动态)
              </span>
            ) : null}
          </div>
          {(state.capabilities?.unresolved_gaps || 0) > 0 && (
            <div className="text-sm text-red-400">
              <AlertTriangle size={12} className="inline mr-1" />
              {state.capabilities.unresolved_gaps} 缺口
            </div>
          )}
        </div>
        {/* Recent Actions */}
        <div>
          <div className="text-xs mb-1" style={{ color: "var(--text-muted)" }}>
            近期动作
          </div>
          {(state.recent_actions || []).slice(-3).reverse().map((a, i) => (
            <div key={i} className="text-xs truncate" title={a.action}>
              {a.success ? "✓" : "✗"} {a.action.slice(0, 50)}
            </div>
          ))}
        </div>
      </div>
    </div>
  );
}

function GapPanel({ gaps }: { gaps: GapStats | null }) {
  if (!gaps || gaps.total === 0) return null;
  return (
    <div
      className="rounded-xl p-4 border mb-6"
      style={{ background: "var(--bg-card)", borderColor: "var(--border)" }}
    >
      <div className="flex items-center gap-2 mb-2">
        <AlertTriangle size={16} className="text-amber-400" />
        <span className="font-semibold text-sm">能力缺口</span>
      </div>
      <div className="grid grid-cols-5 gap-2 text-center text-xs">
        <div>
          <div className="text-lg font-bold">{gaps.total}</div>
          <div style={{ color: "var(--text-muted)" }}>总计</div>
        </div>
        <div>
          <div className="text-lg font-bold text-red-400">{gaps.unresolved}</div>
          <div style={{ color: "var(--text-muted)" }}>未解决</div>
        </div>
        <div>
          <div className="text-lg font-bold text-amber-400">{gaps.skill_missing}</div>
          <div style={{ color: "var(--text-muted)" }}>技能缺失</div>
        </div>
        <div>
          <div className="text-lg font-bold text-blue-400">{gaps.param_error}</div>
          <div style={{ color: "var(--text-muted)" }}>参数错误</div>
        </div>
        <div>
          <div className="text-lg font-bold text-purple-400">{gaps.env_error}</div>
          <div style={{ color: "var(--text-muted)" }}>环境错误</div>
        </div>
      </div>
    </div>
  );
}

function TaskRow({
  task,
  expanded,
  onToggle,
  onRun,
  onCancel,
  onDelete,
  onThread,
  onDetail,
}: {
  task: TaskInfo;
  expanded: boolean;
  onToggle: () => void;
  onRun: () => void;
  onCancel: () => void;
  onDelete: () => void;
  onThread: () => void;
  onDetail: () => void;
}) {
  const steps = task.steps || [];
  const doneSteps = steps.filter((s) => s.status === "done").length;
  const progress = steps.length > 0 ? (doneSteps / steps.length) * 100 : 0;

  return (
    <div
      className="rounded-xl border mb-3 overflow-hidden"
      style={{ background: "var(--bg-card)", borderColor: "var(--border)" }}
    >
      {/* Header */}
      <div
        className="flex items-center gap-3 px-4 py-3 cursor-pointer hover:opacity-80 transition-opacity"
        onClick={onToggle}
      >
        {expanded ? <ChevronDown size={16} /> : <ChevronRight size={16} />}
        <div
          className="w-2.5 h-2.5 rounded-full flex-shrink-0"
          style={{ background: statusColor[task.status] || "#9ca3af" }}
          title={task.status}
        />
        <div className="flex-1 min-w-0">
          <div
            className="font-medium text-sm truncate cursor-pointer hover:underline"
            onClick={(e) => { e.stopPropagation(); onDetail(); }}
            style={{ color: "var(--accent)" }}
          >
            {task.title}
          </div>
          <div className="text-xs truncate" style={{ color: "var(--text-muted)" }}>
            {task.description}
          </div>
        </div>
        <div className="flex items-center gap-2 flex-shrink-0">
          {steps.length > 0 && (
            <span className="text-xs" style={{ color: "var(--text-muted)" }}>
              {doneSteps}/{steps.length}
            </span>
          )}
          <span
            className="text-xs px-2 py-0.5 rounded-full"
            style={{
              background: `${statusColor[task.status] || "#9ca3af"}20`,
              color: statusColor[task.status] || "#9ca3af",
            }}
          >
            {task.status}
          </span>
          <span className="text-xs" style={{ color: "var(--text-muted)" }}>
            {relTime(task.updated_at)}
          </span>
        </div>
        {/* Actions */}
        <div className="flex items-center gap-1 ml-2" onClick={(e) => e.stopPropagation()}>
          <button
            onClick={onThread}
            className="p-1.5 rounded-lg hover:bg-white/10 transition-colors"
            title="任务线程"
          >
            <MessageSquare size={14} style={{ color: "var(--accent)" }} />
          </button>
          {task.status === "pending" && (
            <button
              onClick={onRun}
              className="p-1.5 rounded-lg hover:bg-white/10 transition-colors"
              title="运行"
            >
              <Play size={14} className="text-green-400" />
            </button>
          )}
          {task.status === "running" && (
            <button
              onClick={onCancel}
              className="p-1.5 rounded-lg hover:bg-white/10 transition-colors"
              title="取消"
            >
              <XCircle size={14} className="text-amber-400" />
            </button>
          )}
          {(task.status === "completed" || task.status === "failed" || task.status === "cancelled") && (
            <button
              onClick={onDelete}
              className="p-1.5 rounded-lg hover:bg-white/10 transition-colors"
              title="删除"
            >
              <Trash2 size={14} className="text-gray-400" />
            </button>
          )}
        </div>
      </div>

      {/* Progress bar */}
      {steps.length > 0 && (
        <div className="h-1 mx-4" style={{ background: "var(--border)" }}>
          <div
            className="h-full rounded-full transition-all duration-500"
            style={{
              width: `${progress}%`,
              background: task.status === "failed" ? "#ef4444" : "#22c55e",
            }}
          />
        </div>
      )}

      {/* Steps detail */}
      {expanded && steps.length > 0 && (
        <div className="px-4 py-3 border-t" style={{ borderColor: "var(--border)" }}>
          {steps.map((step) => (
            <div
              key={step.id}
              className="flex items-start gap-2 py-2 border-b last:border-b-0"
              style={{ borderColor: "var(--border)" }}
            >
              <div className="mt-0.5">{stepStatusIcon[step.status] || stepStatusIcon.pending}</div>
              <div className="flex-1 min-w-0">
                <div className="text-sm font-medium">
                  Step {step.id}: {step.action}
                </div>
                {step.skill_name && (
                  <span
                    className="text-xs px-1.5 py-0.5 rounded mt-0.5 inline-block"
                    style={{ background: "var(--accent)15", color: "var(--accent)" }}
                  >
                    {step.skill_name}
                  </span>
                )}
                {step.result && (
                  <div
                    className="text-xs mt-1 p-2 rounded whitespace-pre-wrap break-words max-h-32 overflow-y-auto"
                    style={{ background: "var(--bg-main)", color: "var(--text-muted)" }}
                  >
                    {step.result.length > 500 ? step.result.slice(0, 500) + "…" : step.result}
                  </div>
                )}
                {step.error && (
                  <div className="text-xs mt-1 p-2 rounded text-red-400" style={{ background: "#ef444410" }}>
                    {step.error}
                  </div>
                )}
                {step.gap_type && (
                  <span className="text-xs text-amber-400 mt-1 inline-block">
                    Gap: {step.gap_type}
                  </span>
                )}
              </div>
              <div className="text-xs flex-shrink-0" style={{ color: "var(--text-muted)" }}>
                {step.retry_count ? `retry ${step.retry_count}` : ""}
              </div>
            </div>
          ))}
          {task.error && (
            <div className="text-xs text-red-400 mt-2 p-2 rounded" style={{ background: "#ef444410" }}>
              {task.error}
            </div>
          )}
        </div>
      )}
    </div>
  );
}

function NewTaskForm({ onCreated }: { onCreated: () => void }) {
  const [show, setShow] = useState(false);
  const [title, setTitle] = useState("");
  const [desc, setDesc] = useState("");
  const [loading, setLoading] = useState(false);

  const submit = async () => {
    if (!title.trim()) return;
    setLoading(true);
    try {
      await api.taskCreate(title, desc);
      setTitle("");
      setDesc("");
      setShow(false);
      onCreated();
    } catch {
      /* ignore */
    } finally {
      setLoading(false);
    }
  };

  if (!show) {
    return (
      <button
        onClick={() => setShow(true)}
        className="flex items-center gap-2 px-4 py-2 rounded-lg text-sm font-medium transition-colors"
        style={{ background: "var(--accent)", color: "#fff" }}
      >
        <Plus size={16} /> 新建任务
      </button>
    );
  }

  return (
    <div
      className="rounded-xl p-4 border mb-4"
      style={{ background: "var(--bg-card)", borderColor: "var(--border)" }}
    >
      <input
        className="w-full mb-2 px-3 py-2 rounded-lg text-sm border bg-transparent"
        style={{ borderColor: "var(--border)" }}
        placeholder="任务标题"
        value={title}
        onChange={(e) => setTitle(e.target.value)}
        onKeyDown={(e) => e.key === "Enter" && submit()}
      />
      <textarea
        className="w-full mb-3 px-3 py-2 rounded-lg text-sm border bg-transparent resize-none"
        style={{ borderColor: "var(--border)" }}
        placeholder="任务描述（LLM 将自动规划执行步骤）"
        rows={2}
        value={desc}
        onChange={(e) => setDesc(e.target.value)}
      />
      <div className="flex gap-2">
        <button
          onClick={submit}
          disabled={loading || !title.trim()}
          className="px-4 py-1.5 rounded-lg text-sm font-medium disabled:opacity-50"
          style={{ background: "var(--accent)", color: "#fff" }}
        >
          {loading ? "创建中…" : "创建"}
        </button>
        <button
          onClick={() => setShow(false)}
          className="px-4 py-1.5 rounded-lg text-sm"
          style={{ color: "var(--text-muted)" }}
        >
          取消
        </button>
      </div>
    </div>
  );
}

/* ── Thread Panel ── */

const threadStateColor: Record<string, string> = {
  open: "#22c55e",
  paused: "#f59e0b",
  closed: "#9ca3af",
};

const msgRoleStyle: Record<string, { bg: string; align: string }> = {
  user: { bg: "var(--accent)", align: "flex-end" },
  assistant: { bg: "#374151", align: "flex-start" },
  system: { bg: "#1e293b", align: "center" },
};

function ThreadPanel({
  taskId,
  taskTitle,
  onClose,
}: {
  taskId: string;
  taskTitle: string;
  onClose: () => void;
}) {
  const [info, setInfo] = useState<TaskThreadInfo | null>(null);
  const [messages, setMessages] = useState<LLMMessage[]>([]);
  const [input, setInput] = useState("");
  const [sending, setSending] = useState(false);
  const [loading, setLoading] = useState(true);
  const messagesEndRef = useRef<HTMLDivElement>(null);

  const loadThread = useCallback(async () => {
    try {
      const data = await api.getTaskThread(taskId);
      setInfo(data.info);
      setMessages(data.messages || []);
    } catch {
      /* ignore */
    } finally {
      setLoading(false);
    }
  }, [taskId]);

  useEffect(() => {
    loadThread();
    const id = setInterval(loadThread, 3000);
    return () => clearInterval(id);
  }, [loadThread]);

  useEffect(() => {
    messagesEndRef.current?.scrollIntoView({ behavior: "smooth" });
  }, [messages]);

  const send = async () => {
    const text = input.trim();
    if (!text || sending) return;
    setSending(true);
    setInput("");
    try {
      await api.postTaskThread(taskId, text);
      await loadThread();
    } catch {
      /* ignore */
    } finally {
      setSending(false);
    }
  };

  const changeState = async (state: ThreadState) => {
    try {
      await api.updateThreadState(taskId, state);
      await loadThread();
    } catch {
      /* ignore */
    }
  };

  const isClosed = info?.state === "closed";
  const isPaused = info?.state === "paused";

  return (
    <div
      className="rounded-xl border flex flex-col"
      style={{
        background: "var(--bg-card)",
        borderColor: "var(--border)",
        height: "480px",
      }}
    >
      {/* Header */}
      <div
        className="flex items-center gap-2 px-4 py-3 border-b flex-shrink-0"
        style={{ borderColor: "var(--border)" }}
      >
        <MessageSquare size={16} style={{ color: "var(--accent)" }} />
        <span className="font-medium text-sm flex-1 truncate">
          线程: {taskTitle}
        </span>
        {info && (
          <>
            <span
              className="text-xs px-2 py-0.5 rounded-full"
              style={{
                background: `${threadStateColor[info.state] || "#9ca3af"}20`,
                color: threadStateColor[info.state] || "#9ca3af",
              }}
            >
              {info.state}
            </span>
            {info.binding && (
              <span
                className="text-xs px-2 py-0.5 rounded"
                style={{ background: "var(--accent)15", color: "var(--accent)" }}
              >
                {info.binding.channel_type}
              </span>
            )}
          </>
        )}
        {/* State controls */}
        <div className="flex gap-1">
          {!isClosed && !isPaused && (
            <button
              onClick={() => changeState("paused")}
              className="p-1 rounded hover:bg-white/10"
              title="暂停线程"
            >
              <Pause size={14} className="text-amber-400" />
            </button>
          )}
          {isPaused && (
            <button
              onClick={() => changeState("open")}
              className="p-1 rounded hover:bg-white/10"
              title="恢复线程"
            >
              <PlayCircle size={14} className="text-green-400" />
            </button>
          )}
          {!isClosed && (
            <button
              onClick={() => changeState("closed")}
              className="p-1 rounded hover:bg-white/10"
              title="关闭线程"
            >
              <Lock size={14} className="text-gray-400" />
            </button>
          )}
        </div>
        <button
          onClick={onClose}
          className="p-1 rounded hover:bg-white/10"
          title="关闭面板"
        >
          <X size={14} />
        </button>
      </div>

      {/* Messages area */}
      <div className="flex-1 overflow-y-auto px-4 py-3 space-y-2">
        {loading ? (
          <div className="flex items-center justify-center h-full">
            <Loader2 size={20} className="animate-spin" style={{ color: "var(--text-muted)" }} />
          </div>
        ) : messages.length === 0 ? (
          <div
            className="flex flex-col items-center justify-center h-full text-center"
            style={{ color: "var(--text-muted)" }}
          >
            <MessageSquare size={32} className="mb-2 opacity-30" />
            <p className="text-sm">暂无消息</p>
            <p className="text-xs mt-1">在下方发送消息开始对话</p>
          </div>
        ) : (
          messages.map((msg, i) => {
            const style = msgRoleStyle[msg.role] || msgRoleStyle.system;
            const isSystem = msg.role === "system";
            return (
              <div
                key={i}
                className="flex"
                style={{ justifyContent: style.align }}
              >
                <div
                  className={`max-w-[80%] px-3 py-2 rounded-xl text-sm whitespace-pre-wrap break-words ${
                    isSystem ? "text-center w-full" : ""
                  }`}
                  style={{
                    background: isSystem ? "transparent" : style.bg,
                    color: isSystem ? "var(--text-muted)" : "#fff",
                    fontSize: isSystem ? "12px" : "14px",
                    fontStyle: isSystem ? "italic" : "normal",
                  }}
                >
                  {!isSystem && (
                    <div className="text-xs opacity-60 mb-0.5">
                      {msg.role === "user" ? "👤" : "🤖"} {msg.role}
                    </div>
                  )}
                  {msg.content}
                </div>
              </div>
            );
          })
        )}
        <div ref={messagesEndRef} />
      </div>

      {/* Input area */}
      <div
        className="px-4 py-3 border-t flex-shrink-0"
        style={{ borderColor: "var(--border)" }}
      >
        {isClosed ? (
          <div
            className="text-center text-sm py-2"
            style={{ color: "var(--text-muted)" }}
          >
            <Lock size={14} className="inline mr-1" />
            线程已关闭（只读）
          </div>
        ) : (
          <div className="flex gap-2">
            <input
              className="flex-1 px-3 py-2 rounded-lg text-sm border bg-transparent"
              style={{ borderColor: "var(--border)" }}
              placeholder={isPaused ? "线程已暂停" : "输入消息…"}
              disabled={isPaused}
              value={input}
              onChange={(e) => setInput(e.target.value)}
              onKeyDown={(e) => e.key === "Enter" && !e.shiftKey && send()}
            />
            <button
              onClick={send}
              disabled={!input.trim() || sending || isPaused}
              className="px-3 py-2 rounded-lg disabled:opacity-50"
              style={{ background: "var(--accent)", color: "#fff" }}
            >
              {sending ? (
                <Loader2 size={16} className="animate-spin" />
              ) : (
                <Send size={16} />
              )}
            </button>
          </div>
        )}
      </div>
    </div>
  );
}

/* ── Page ── */

export default function TasksPage() {
  const router = useRouter();
  const [tasks, setTasks] = useState<TaskInfo[]>([]);
  const [state, setState] = useState<StateSnapshot | null>(null);
  const [gaps, setGaps] = useState<GapStats | null>(null);
  const [expanded, setExpanded] = useState<Set<string>>(new Set());
  const [loading, setLoading] = useState(true);
  const [activeThread, setActiveThread] = useState<{ id: string; title: string } | null>(null);

  const refresh = useCallback(async () => {
    try {
      const [taskList, snap, gapData] = await Promise.all([
        api.taskList().catch(() => []),
        api.stateSnapshot().catch(() => null),
        api.taskGaps(true).catch(() => null),
      ]);
      setTasks(Array.isArray(taskList) ? taskList : []);
      setState(snap as StateSnapshot | null);
      setGaps(gapData as GapStats | null);
    } catch {
      /* ignore */
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    refresh();
    const id = setInterval(refresh, 5000); // auto-refresh every 5s
    return () => clearInterval(id);
  }, [refresh]);

  const toggleExpand = (id: string) => {
    setExpanded((prev) => {
      const next = new Set(prev);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      return next;
    });
  };

  const handleRun = async (id: string) => {
    await api.taskRun(id);
    setExpanded((prev) => new Set(prev).add(id));
    setTimeout(refresh, 500);
  };

  const handleCancel = async (id: string) => {
    await api.taskCancel(id);
    setTimeout(refresh, 500);
  };

  const handleDelete = async (id: string) => {
    await api.taskDelete(id);
    setExpanded((prev) => {
      const next = new Set(prev);
      next.delete(id);
      return next;
    });
    setTimeout(refresh, 300);
  };

  // Sort: running first, then pending, then by updated_at desc
  const sorted = [...tasks].sort((a, b) => {
    const pri: Record<string, number> = { running: 0, planning: 1, pending: 2, failed: 3, completed: 4, cancelled: 5 };
    const pa = pri[a.status] ?? 9;
    const pb = pri[b.status] ?? 9;
    if (pa !== pb) return pa - pb;
    return new Date(b.updated_at).getTime() - new Date(a.updated_at).getTime();
  });

  return (
    <div className="max-w-5xl mx-auto px-4 py-8">
      <BlurFade delay={0.05}>
        <div className="flex items-center justify-between mb-6">
          <div className="flex items-center gap-3">
            <ListTodo size={24} style={{ color: "var(--accent)" }} />
            <h1 className="text-2xl font-bold">任务运行时</h1>
          </div>
          <div className="flex items-center gap-3">
            <button
              onClick={refresh}
              className="p-2 rounded-lg hover:bg-white/10 transition-colors"
              title="刷新"
            >
              <RefreshCw size={16} />
            </button>
            <NewTaskForm onCreated={refresh} />
          </div>
        </div>
      </BlurFade>

      <BlurFade delay={0.1}>
        <StatePanel state={state} />
      </BlurFade>

      <BlurFade delay={0.15}>
        <GapPanel gaps={gaps} />
      </BlurFade>

      <BlurFade delay={0.2}>
        {loading ? (
          <div className="text-center py-12" style={{ color: "var(--text-muted)" }}>
            <Loader2 size={24} className="animate-spin mx-auto mb-2" />
            加载中…
          </div>
        ) : sorted.length === 0 ? (
          <div className="text-center py-12" style={{ color: "var(--text-muted)" }}>
            <ListTodo size={48} className="mx-auto mb-3 opacity-30" />
            <p>暂无任务</p>
            <p className="text-xs mt-1">点击"新建任务"创建第一个任务</p>
          </div>
        ) : (
          sorted.map((task) => (
            <TaskRow
              key={task.id}
              task={task}
              expanded={expanded.has(task.id)}
              onToggle={() => toggleExpand(task.id)}
              onRun={() => handleRun(task.id)}
              onCancel={() => handleCancel(task.id)}
              onDelete={() => handleDelete(task.id)}
              onThread={() => setActiveThread({ id: task.id, title: task.title })}
              onDetail={() => router.push(`/task-detail?id=${task.id}`)}
            />
          ))
        )}
      </BlurFade>

      {/* Thread Panel — slides in from right as overlay */}
      {activeThread && (
        <BlurFade delay={0.05}>
          <div className="fixed right-4 bottom-4 w-[420px] z-50 shadow-2xl">
            <ThreadPanel
              taskId={activeThread.id}
              taskTitle={activeThread.title}
              onClose={() => setActiveThread(null)}
            />
          </div>
        </BlurFade>
      )}
    </div>
  );
}
