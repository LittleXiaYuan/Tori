"use client";

import { useEffect, useState, useCallback, useRef } from "react";
import { useRouter, useSearchParams } from "next/navigation";
import { Suspense } from "react";
import {
  api,
  type TaskInfo,
  type TaskStep,
  type TaskWorkingMemory,
  type LLMMessage,
} from "@/lib/api";
import { ComputerPanel } from "@/components/computer-panel";
import { BlurFade } from "@/components/ui/blur-fade";
import { useI18n } from "@/lib/i18n";
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
  Send,
  Sparkles,
  FileText,
  AlertTriangle,
  ChevronRight,
  Wrench,
  Brain,
  Zap,
  Eye,
  EyeOff,
  StopCircle,
} from "lucide-react";

/* ── status maps ── */
const statusMeta: Record<string, { color: string; label: string; bgAlpha: string }> = {
  pending:     { color: "#9ca3af", label: "待运行", bgAlpha: "15" },
  planning:    { color: "#a78bfa", label: "规划中", bgAlpha: "18" },
  running:     { color: "#3b82f6", label: "执行中", bgAlpha: "18" },
  paused:      { color: "#f59e0b", label: "已暂停", bgAlpha: "20" },
  completed:   { color: "#22c55e", label: "已完成", bgAlpha: "20" },
  failed:      { color: "#ef4444", label: "失败",   bgAlpha: "18" },
  cancelled:   { color: "#f59e0b", label: "已取消", bgAlpha: "18" },
  interrupted: { color: "#f97316", label: "中断",   bgAlpha: "18" },
};

function dur(start?: string, end?: string): string {
  if (!start) return "—";
  const d = Math.max(0, (end ? new Date(end).getTime() : Date.now()) - new Date(start).getTime());
  if (d < 1000) return `${d}ms`;
  if (d < 60000) return `${(d / 1000).toFixed(1)}s`;
  return `${(d / 60000).toFixed(1)}min`;
}

/* ─────────────────────────────────────────────
   Step Item — single execution step in timeline
   ───────────────────────────────────────────── */
function StepItem({ step, isLast, isActive }: { step: TaskStep; isLast: boolean; isActive: boolean }) {
  const [expanded, setExpanded] = useState(false);
  const isDone = step.status === "done";
  const isFailed = step.status === "failed";
  const isRunning = step.status === "running" || step.status === "retrying";

  const dotColor = isDone ? "#22c55e" : isFailed ? "#ef4444" : isRunning ? "#3b82f6" : "var(--border)";

  return (
    <div className="relative flex gap-3 group">
      {/* Timeline line + dot */}
      <div className="flex flex-col items-center flex-shrink-0" style={{ width: 24 }}>
        <div
          className="w-3 h-3 rounded-full border-2 z-10 flex-shrink-0"
          style={{
            borderColor: dotColor,
            background: isDone || isFailed ? dotColor : "var(--bg-main)",
            boxShadow: isRunning ? `0 0 8px ${dotColor}` : "none",
          }}
        >
          {isRunning && (
            <div className="w-full h-full rounded-full animate-ping" style={{ background: dotColor, opacity: 0.4 }} />
          )}
        </div>
        {!isLast && (
          <div className="w-0.5 flex-1 min-h-[20px]" style={{ background: isDone ? "#22c55e40" : "var(--border)" }} />
        )}
      </div>

      {/* Content */}
      <div className="flex-1 pb-4 min-w-0">
        <button
          className="flex items-center gap-2 w-full text-left"
          onClick={() => setExpanded(!expanded)}
        >
          <div className="flex items-center gap-2 flex-1 min-w-0">
            {isRunning ? (
              <Loader2 size={13} className="animate-spin flex-shrink-0" style={{ color: "#3b82f6" }} />
            ) : isDone ? (
              <CheckCircle2 size={13} className="flex-shrink-0" style={{ color: "#22c55e" }} />
            ) : isFailed ? (
              <XCircle size={13} className="flex-shrink-0" style={{ color: "#ef4444" }} />
            ) : (
              <Clock size={13} className="flex-shrink-0" style={{ color: "var(--text-muted)" }} />
            )}
            <span className="text-sm font-medium truncate" style={{ color: isRunning ? "var(--text)" : isDone ? "var(--text-secondary)" : "var(--text-muted)" }}>
              {step.action}
            </span>
          </div>
          <div className="flex items-center gap-2 flex-shrink-0">
            {step.skill_name && (
              <span className="text-[10px] px-1.5 py-0.5 rounded" style={{ background: "var(--accent-subtle, var(--bg-hover))", color: "var(--accent)" }}>
                {step.skill_name}
              </span>
            )}
            <span className="text-[11px] font-mono" style={{ color: "var(--text-muted)" }}>
              {dur(step.started_at, step.done_at)}
            </span>
            <ChevronRight
              size={12}
              className="transition-transform"
              style={{ color: "var(--text-muted)", transform: expanded ? "rotate(90deg)" : "none" }}
            />
          </div>
        </button>

        {/* Expanded detail */}
        {expanded && (
          <div className="mt-2 ml-5 space-y-2 animate-in">
            {step.result && (
              <div
                className="text-xs p-3 rounded-lg whitespace-pre-wrap break-words max-h-48 overflow-y-auto"
                style={{ background: "var(--bg-hover)", color: "var(--text-secondary)" }}
              >
                {step.result.length > 2000 ? step.result.slice(0, 2000) + "…" : step.result}
              </div>
            )}
            {step.error && (
              <div className="text-xs p-3 rounded-lg text-red-400" style={{ background: "#ef444410" }}>
                {step.error}
              </div>
            )}
            {step.args && Object.keys(step.args).length > 0 && (
              <div className="text-xs p-3 rounded-lg" style={{ background: "var(--bg-hover)" }}>
                <div className="font-medium mb-1" style={{ color: "var(--text-muted)" }}>参数</div>
                {Object.entries(step.args).map(([k, v]) => (
                  <div key={k}>
                    <span style={{ color: "var(--accent)" }}>{k}</span>
                    <span style={{ color: "var(--text-muted)" }}> = </span>
                    <span style={{ color: "var(--text-secondary)" }}>{String(v)}</span>
                  </div>
                ))}
              </div>
            )}
            {step.retry_count ? (
              <div className="text-[11px]" style={{ color: "#f59e0b" }}>重试 ×{step.retry_count}</div>
            ) : null}
          </div>
        )}
      </div>
    </div>
  );
}

/* ─────────────────────────────────────────────
   Main: Task Execution View (Manus-style)
   ───────────────────────────────────────────── */
function TaskRunContent() {
  const router = useRouter();
  const searchParams = useSearchParams();
  const taskId = searchParams.get("id") || "";
  const { locale } = useI18n();
  const zh = locale === "zh";

  const [task, setTask] = useState<TaskInfo | null>(null);
  const [wm, setWm] = useState<TaskWorkingMemory | null>(null);
  const [threadMessages, setThreadMessages] = useState<LLMMessage[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [threadInput, setThreadInput] = useState("");
  const [sending, setSending] = useState(false);
  const [showPanel, setShowPanel] = useState(true);
  const stepsEndRef = useRef<HTMLDivElement>(null);

  const refresh = useCallback(async () => {
    if (!taskId) { setError("缺少任务 ID"); setLoading(false); return; }
    try {
      const t = await api.taskGet(taskId);
      setTask(t);
      const [wmData, threadData] = await Promise.all([
        api.getTaskWorkingMemory(taskId).catch(() => null),
        api.getTaskThread(taskId).catch(() => ({ messages: [] })),
      ]);
      setWm(wmData);
      setThreadMessages(threadData.messages || []);
    } catch {
      setError("任务不存在或加载失败");
    } finally {
      setLoading(false);
    }
  }, [taskId]);

  // Poll for updates
  useEffect(() => {
    refresh();
    const id = setInterval(refresh, 3000);
    return () => clearInterval(id);
  }, [refresh]);

  // Auto-scroll to latest step
  useEffect(() => {
    if (task?.status === "running" || task?.status === "planning") {
      stepsEndRef.current?.scrollIntoView({ behavior: "smooth" });
    }
  }, [task?.steps?.length, task?.status]);

  const handleAction = async (action: "run" | "pause" | "resume" | "cancel" | "restart") => {
    if (!task) return;
    const fn = { run: api.taskRun, pause: api.taskPause, resume: api.taskResume, cancel: api.taskCancel, restart: api.taskRestart };
    await fn[action](task.id);
    setTimeout(refresh, 500);
  };

  const sendThread = async () => {
    const text = threadInput.trim();
    if (!text || sending) return;
    setSending(true);
    setThreadInput("");
    try {
      await api.postTaskThread(taskId, text);
      await refresh();
    } catch { /* ignore */ }
    finally { setSending(false); }
  };

  /* ── Loading / Error states ── */
  if (loading) {
    return (
      <div className="flex items-center justify-center h-[80vh]">
        <div className="text-center">
          <Loader2 size={28} className="animate-spin mx-auto mb-3" style={{ color: "var(--accent)" }} />
          <p className="text-sm" style={{ color: "var(--text-muted)" }}>
            {zh ? "加载任务..." : "Loading task..."}
          </p>
        </div>
      </div>
    );
  }

  if (error || !task) {
    return (
      <div className="flex items-center justify-center h-[80vh]">
        <div className="text-center">
          <AlertTriangle size={40} className="mx-auto mb-3 opacity-30" style={{ color: "var(--text-muted)" }} />
          <p className="text-sm mb-4" style={{ color: "var(--text-muted)" }}>{error || "任务未找到"}</p>
          <button
            onClick={() => router.push("/")}
            className="px-4 py-2 rounded-lg text-sm"
            style={{ background: "var(--accent)", color: "#fff" }}
          >
            {zh ? "返回首页" : "Go Home"}
          </button>
        </div>
      </div>
    );
  }

  const steps = task.steps || [];
  const sm = statusMeta[task.status] || statusMeta.pending;
  const isActive = task.status === "running" || task.status === "planning";
  const isPaused = task.status === "paused" || task.status === "interrupted";
  const isDone = task.status === "completed";
  const isFailed = task.status === "failed";

  return (
    <div className="flex flex-col" style={{ height: "calc(100vh - 32px)" }}>
      {/* ── Header bar ── */}
      <div
        className="flex items-center gap-3 px-5 py-3 border-b flex-shrink-0"
        style={{ borderColor: "var(--border)" }}
      >
        <button
          onClick={() => router.push("/missions")}
          className="p-1.5 rounded-lg transition-colors"
          style={{ color: "var(--text-muted)" }}
          onMouseEnter={(e) => { e.currentTarget.style.background = "var(--bg-hover)"; }}
          onMouseLeave={(e) => { e.currentTarget.style.background = "transparent"; }}
        >
          <ArrowLeft size={16} />
        </button>

        <div className="flex-1 min-w-0">
          <div className="flex items-center gap-2">
            <h1 className="text-sm font-semibold truncate" style={{ color: "var(--text)" }}>
              {task.title}
            </h1>
            <span
              className="text-[10px] px-2 py-0.5 rounded-full font-medium flex-shrink-0"
              style={{ background: `${sm.color}${sm.bgAlpha}`, color: sm.color }}
            >
              {sm.label}
            </span>
          </div>
          <div className="text-[11px]" style={{ color: "var(--text-muted)" }}>
            {task.description?.slice(0, 100) || task.id}
          </div>
        </div>

        {/* Action buttons */}
        <div className="flex items-center gap-1.5 flex-shrink-0">
          <button
            onClick={() => setShowPanel(!showPanel)}
            className="p-1.5 rounded-lg transition-colors"
            style={{ color: "var(--text-muted)" }}
            title={showPanel ? "隐藏面板" : "显示面板"}
          >
            {showPanel ? <EyeOff size={15} /> : <Eye size={15} />}
          </button>
          <button
            onClick={() => refresh()}
            className="p-1.5 rounded-lg transition-colors"
            style={{ color: "var(--text-muted)" }}
          >
            <RefreshCw size={15} />
          </button>

          {task.status === "pending" && (
            <button
              onClick={() => handleAction("run")}
              className="flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-xs font-medium"
              style={{ background: "#22c55e", color: "#fff" }}
            >
              <Play size={12} /> {zh ? "运行" : "Run"}
            </button>
          )}
          {isActive && (
            <>
              <button
                onClick={() => handleAction("pause")}
                className="p-1.5 rounded-lg"
                style={{ background: "#a78bfa20", color: "#a78bfa" }}
                title={zh ? "暂停" : "Pause"}
              >
                <Pause size={14} />
              </button>
              <button
                onClick={() => handleAction("cancel")}
                className="p-1.5 rounded-lg"
                style={{ background: "#ef444420", color: "#ef4444" }}
                title={zh ? "停止" : "Stop"}
              >
                <StopCircle size={14} />
              </button>
            </>
          )}
          {isPaused && (
            <button
              onClick={() => handleAction("resume")}
              className="flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-xs font-medium"
              style={{ background: "#3b82f6", color: "#fff" }}
            >
              <PlayCircle size={12} /> {zh ? "恢复" : "Resume"}
            </button>
          )}
          {(isDone || isFailed || task.status === "cancelled") && (
            <button
              onClick={() => handleAction("restart")}
              className="flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-xs font-medium"
              style={{ background: "#6366f1", color: "#fff" }}
            >
              <RefreshCw size={12} /> {zh ? "重新执行" : "Restart"}
            </button>
          )}
        </div>
      </div>

      {/* ── Main content area (split view) ── */}
      <div className="flex flex-1 min-h-0">
        {/* Left: Step timeline */}
        <div className={`flex flex-col min-h-0 ${showPanel ? "w-3/5 border-r" : "w-full"}`} style={{ borderColor: "var(--border)" }}>
          {/* Steps scroll area */}
          <div className="flex-1 overflow-y-auto px-6 py-5">
            {steps.length === 0 ? (
              <div className="flex flex-col items-center justify-center h-full" style={{ color: "var(--text-muted)" }}>
                {isActive ? (
                  <>
                    <Loader2 size={32} className="animate-spin mb-3" style={{ color: "var(--accent)" }} />
                    <p className="text-sm font-medium">{zh ? "云雀正在思考..." : "Yunque is thinking..."}</p>
                    <p className="text-xs mt-1">{zh ? "正在分析任务并制定执行计划" : "Analyzing task and creating execution plan"}</p>
                  </>
                ) : task.status === "pending" ? (
                  <>
                    <Play size={32} className="mb-3 opacity-30" />
                    <p className="text-sm">{zh ? "点击「运行」开始执行" : "Click Run to start"}</p>
                  </>
                ) : (
                  <>
                    <Wrench size={32} className="mb-3 opacity-30" />
                    <p className="text-sm">{zh ? "暂无执行步骤" : "No steps yet"}</p>
                  </>
                )}
              </div>
            ) : (
              <BlurFade delay={0.05}>
                <div>
                  {/* Yunque monologue at start */}
                  <div className="flex items-center gap-2 mb-5 pb-4 border-b" style={{ borderColor: "var(--border)" }}>
                    <Sparkles size={14} style={{ color: "var(--accent)", opacity: 0.6 }} />
                    <span className="text-xs italic" style={{ color: "var(--text-muted)" }}>
                      {zh ? `好的，让我来处理「${task.title}」...` : `Alright, let me work on "${task.title}"...`}
                    </span>
                  </div>

                  {/* Step items */}
                  {steps.map((step, idx) => (
                    <StepItem
                      key={step.id}
                      step={step}
                      isLast={idx === steps.length - 1}
                      isActive={step.status === "running" || step.status === "retrying"}
                    />
                  ))}

                  {/* Running indicator at bottom */}
                  {isActive && steps.length > 0 && (
                    <div className="flex items-center gap-2 pl-8 pt-2">
                      <Loader2 size={12} className="animate-spin" style={{ color: "var(--accent)" }} />
                      <span className="text-xs" style={{ color: "var(--text-muted)" }}>
                        {zh ? "云雀正在执行..." : "Yunque is working..."}
                      </span>
                    </div>
                  )}

                  {/* Completion message */}
                  {isDone && (
                    <div className="flex items-center gap-2 pl-8 pt-3 mt-2 border-t" style={{ borderColor: "var(--border)" }}>
                      <CheckCircle2 size={14} style={{ color: "#22c55e" }} />
                      <span className="text-sm font-medium" style={{ color: "#22c55e" }}>
                        {zh ? "任务已完成" : "Task completed"}
                      </span>
                      <span className="text-xs ml-2" style={{ color: "var(--text-muted)" }}>
                        {dur(task.started_at, task.finished_at)}
                      </span>
                    </div>
                  )}

                  {/* Error message */}
                  {isFailed && (
                    <div className="ml-8 mt-3 p-3 rounded-lg border-l-2" style={{ background: "#ef444410", borderColor: "#ef4444" }}>
                      <div className="flex items-center gap-1.5 mb-1">
                        <XCircle size={13} style={{ color: "#ef4444" }} />
                        <span className="text-xs font-medium text-red-400">{zh ? "执行失败" : "Failed"}</span>
                      </div>
                      {task.error && (
                        <p className="text-xs text-red-300">{task.error}</p>
                      )}
                    </div>
                  )}

                  <div ref={stepsEndRef} />
                </div>
              </BlurFade>
            )}
          </div>

          {/* Thread input at bottom */}
          <div className="px-5 py-3 border-t flex-shrink-0" style={{ borderColor: "var(--border)" }}>
            <div className="flex gap-2">
              <input
                className="flex-1 px-3 py-2 rounded-lg text-sm border bg-transparent outline-none"
                style={{ borderColor: "var(--border)", color: "var(--text)" }}
                placeholder={zh ? "向云雀补充说明或追问..." : "Add context or ask Yunque..."}
                value={threadInput}
                onChange={(e) => setThreadInput(e.target.value)}
                onKeyDown={(e) => { if (e.key === "Enter" && !e.shiftKey) { e.preventDefault(); sendThread(); } }}
              />
              <button
                onClick={sendThread}
                disabled={!threadInput.trim() || sending}
                className="px-3 py-2 rounded-lg disabled:opacity-40 transition-colors"
                style={{ background: "var(--accent)", color: "#fff" }}
              >
                {sending ? <Loader2 size={16} className="animate-spin" /> : <Send size={16} />}
              </button>
            </div>
          </div>
        </div>

        {/* Right: Computer panel */}
        {showPanel && (
          <div className="w-2/5 min-h-0">
            <ComputerPanel steps={task.steps || []} taskStatus={task.status} />
          </div>
        )}
      </div>
    </div>
  );
}

/* ── Page wrapper ── */
export default function TaskRunPage() {
  return (
    <Suspense
      fallback={
        <div className="flex items-center justify-center h-[80vh]">
          <Loader2 size={28} className="animate-spin" style={{ color: "var(--text-muted)" }} />
        </div>
      }
    >
      <TaskRunContent />
    </Suspense>
  );
}
