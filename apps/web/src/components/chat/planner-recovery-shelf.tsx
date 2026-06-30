"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import Link from "next/link";
import { Button, Spinner, Tooltip } from "@heroui/react";
import { ArchiveRestore, ChevronDown, ExternalLink, RefreshCw, RotateCcw } from "lucide-react";
import { api } from "@/lib/api";
import type { PlannerCheckpointRecoverResponse, PlannerCheckpointRecoveryAction, PlannerCheckpointRecoveryPlan, PlannerCheckpointResumePlanJob, PlannerCheckpointResumePlanJobResponse, PlannerCheckpointResumePlanResponse, PlannerCheckpointResumeTaskResponse, PlannerCheckpointSummary, PlannerExecutionStateRecoveryTarget, PlannerExecutionStateResponse, TaskInfo } from "@/lib/api-types";
import { formatErrorMessage } from "@/lib/error-utils";
import { useI18n } from "@/lib/i18n";
import { resolvePlannerRecoveryTarget } from "@/lib/planner-recovery-target";

interface PlannerRecoveryShelfProps {
  onSend: (text: string) => void;
  disabled?: boolean;
  fetchOnMount?: boolean;
  refreshSignal?: number | string;
  initialCheckpoints?: PlannerCheckpointSummary[];
  recoverCheckpoint?: (planId: string, action: PlannerCheckpointRecoveryAction) => Promise<Pick<PlannerCheckpointRecoverResponse, "prompt" | "recovery_plan">>;
  resumeCheckpoint?: (planId: string, action: PlannerCheckpointRecoveryAction, options?: { run?: boolean }) => Promise<Pick<PlannerCheckpointResumeTaskResponse, "task_id" | "status" | "recovery_plan">>;
  resumePlan?: (planId: string, action: PlannerCheckpointRecoveryAction, options?: { async?: boolean }) => Promise<Pick<PlannerCheckpointResumePlanResponse, "status" | "job_id" | "result" | "recovery_plan" | "friendly_error" | "recoverable" | "next_action" | "primary_target">>;
  getResumePlanJob?: (jobIdOrParams: string | { jobId?: string; planId?: string }) => Promise<PlannerCheckpointResumePlanJobResponse>;
  getTask?: (taskId: string) => Promise<Pick<TaskInfo, "id" | "title" | "status" | "error">>;
  getCheckpointDetails?: (planId: string) => Promise<PlannerCheckpointSummary | undefined>;
  getExecutionState?: (planId: string) => Promise<PlannerExecutionStateResponse>;
}

export function fallbackPlannerCheckpointPrompt(cp: PlannerCheckpointSummary, action: PlannerCheckpointRecoveryAction = "continue"): string {
  const error = cp.error ? `\n失败原因：${checkpointErrorLabel(cp.error)}` : "";
  const base = [
    action === "retry_failed" ? "请重试这个可恢复规划里的失败步骤。" : action === "partial" ? "请先返回这个可恢复规划已经完成的部分。" : "请继续这个可恢复规划。",
    "不要从头重跑，优先复用已完成步骤，只处理 pending/failed 部分。",
    "",
    `Plan ID：${cp.plan_id}`,
    `Task ID：${cp.task_id || "未知"}`,
    ...(cp.goal ? [`原始目标：${cp.goal}`] : []),
    `状态：${cp.status || "unknown"}`,
    `进度：${cp.completed ?? 0}/${cp.total ?? 0}${error}`,
  ].join("\n");
  return base;
}

function statusLabel(cp: PlannerCheckpointSummary): string {
  if (cp.error || cp.status === "failed") return "需要恢复";
  if (cp.recoverable) return "可继续";
  if (cp.status === "completed" || cp.completed === cp.total) return "已完成";
  return cp.status || "进行中";
}

function formatTime(ts?: string): string {
  if (!ts) return "";
  const d = new Date(ts);
  if (Number.isNaN(d.getTime())) return "";
  return d.toLocaleString("zh-CN", { month: "2-digit", day: "2-digit", hour: "2-digit", minute: "2-digit" });
}

function recoveryPlanLabel(plan?: PlannerCheckpointRecoveryPlan, action?: PlannerCheckpointRecoveryAction): string {
  if (!plan) return "";
  if (plan.mode === "partial" || action === "partial") return "将先返回已完成部分";
  const selected = plan.steps?.filter((step) => step.selected).length ?? 0;
  const selectedWithDeps = plan.steps?.filter((step) => step.selected && (step.depends_on?.length ?? 0) > 0).length ?? 0;
  if (!plan.executable && plan.reason) return `已生成恢复方案：${plan.reason}`;
  if (selected > 0) {
    if (selectedWithDeps > 0) return `将继续 ${selected} 个步骤，按依赖顺序执行`;
    return `将继续 ${selected} 个步骤`;
  }
  return "已生成恢复方案";
}

function taskStatusLabel(status?: string): string {
  switch (status) {
    case "pending": return "等待执行";
    case "planning": return "正在规划";
    case "running": return "执行中";
    case "completed": return "已完成";
    case "failed": return "需要处理";
    case "cancelled": return "已取消";
    case "paused": return "已暂停";
    case "interrupted": return "可恢复";
    default: return status || "已创建";
  }
}

function taskRecoveryHint(status?: string, error?: string): string {
  if (status === "interrupted" && error?.includes("等待依赖步骤完成")) {
    return "等待前置步骤完成，可进入任务详情确认依赖后继续。";
  }
  if (status === "interrupted") {
    return "任务已保留现场，可进入详情继续处理。";
  }
  if (error) {
    return "任务遇到问题，可进入详情查看处理。";
  }
  return "";
}

function resumePlanJobStatusLabel(status?: string): string {
  switch (status) {
    case "accepted":
    case "pending": return "等待续跑";
    case "running": return "续跑中";
    case "completed": return "已完成";
    case "failed": return "需要处理";
    case "cancelled": return "已取消";
    default: return status || "已开始";
  }
}

function resumePlanJobProgress(job?: Pick<PlannerCheckpointResumePlanJob, "result"> | null): string {
  const plan = job?.result?.plan;
  if (!plan?.length) return "";
  const done = plan.filter((step) => step.status === "done" || step.status === "completed" || step.status === "skipped").length;
  return `完成 ${done}/${plan.length}`;
}

function resumePlanJobHint(job?: Pick<PlannerCheckpointResumePlanJob, "status" | "friendly_error" | "error" | "events" | "result"> | null): string {
  if (!job) return "";
  if (job.status === "failed") return checkpointErrorLabel(job.friendly_error || job.error || "原规划续跑暂未完成，现场已保留。");
  const progress = resumePlanJobProgress(job);
  if (job.status === "completed") return progress ? `原规划续跑已完成，${progress}。` : "原规划续跑已完成。";
  const eventCount = job.events?.length ?? 0;
  if (progress && eventCount > 0) return `现场仍在更新，${progress}，已记录 ${eventCount} 条事件。`;
  if (eventCount > 0) return `现场仍在更新，已记录 ${eventCount} 条事件。`;
  return "现场已保留，可进入详情页查看完整过程。";
}

function resumePlanJobEventSummaries(job?: Pick<PlannerCheckpointResumePlanJob, "events"> | null): string[] {
  const events = job?.events || [];
  return events
    .slice(-2)
    .map((event) => checkpointErrorLabel(event.summary || event.type || "续跑现场已更新。"))
    .filter(Boolean);
}

function checkpointErrorLabel(error?: string): string {
  if (!error) return "";
  return formatErrorMessage(error, "任务暂时没有顺利完成，已保留现场。");
}

function displayRecoveryText(text?: string): string {
  const raw = (text || "").trim();
  if (!raw) return "";
  return raw
    .split(/\r?\n/)
    .map((line) => formatErrorMessage(line, line))
    .join("\n")
    .trim();
}

function compactSessionLabel(sessionId: string | undefined): string {
  const normalized = sessionId?.trim();
  if (!normalized) return "";
  return `会话 ${normalized.length > 8 ? normalized.slice(-8) : normalized}`;
}

function stepStatusLabel(status?: string): string {
  switch (status) {
    case "done":
    case "completed": return "已完成";
    case "skipped": return "已跳过";
    case "failed": return "需要处理";
    case "running": return "执行中";
    case "pending": return "待执行";
    default: return status || "待确认";
  }
}

function isRecoveryStepComplete(status?: string): boolean {
  return status === "done" || status === "completed" || status === "skipped";
}

function recoveryStepDependencyState(
  step: NonNullable<PlannerCheckpointSummary["plan_snapshot"]>[number],
  steps: NonNullable<PlannerCheckpointSummary["plan_snapshot"]>,
): { label: string; color: string; hint?: string; blockedDeps: number[]; completedDeps: number[] } {
  const byId = new Map(steps.map((item) => [item.id, item]));
  const deps = Array.from(new Set(step.depends_on || []));
  const completedDeps = deps.filter((dep) => isRecoveryStepComplete(byId.get(dep)?.status));
  const blockedDeps = deps.filter((dep) => !isRecoveryStepComplete(byId.get(dep)?.status));

  if (isRecoveryStepComplete(step.status)) {
    return { label: "已完成", color: "#86efac", blockedDeps, completedDeps };
  }
  if (step.status === "failed") {
    return { label: "需处理", color: "#fca5a5", blockedDeps, completedDeps };
  }
  if (step.status === "running") {
    return { label: "执行中", color: "var(--yunque-accent-strong)", blockedDeps, completedDeps };
  }
  if (blockedDeps.length > 0) {
    return {
      label: "被阻塞",
      color: "#fcd34d",
      hint: `阻塞依赖：${blockedDeps.map((dep) => `#${dep}`).join("、")}`,
      blockedDeps,
      completedDeps,
    };
  }
  return {
    label: "可执行",
    color: "#7dd3fc",
    hint: deps.length > 0 ? `前置已完成：${completedDeps.map((dep) => `#${dep}`).join("、")}` : "无前置依赖，可直接继续。",
    blockedDeps,
    completedDeps,
  };
}

function recoveryStepGraphCounts(steps: NonNullable<PlannerCheckpointSummary["plan_snapshot"]>): Record<string, number> {
  return steps.reduce<Record<string, number>>((acc, step) => {
    const state = recoveryStepDependencyState(step, steps).label;
    acc[state] = (acc[state] || 0) + 1;
    return acc;
  }, {});
}

function stepResultPreview(result?: string): string {
  const normalized = displayRecoveryText(result).replace(/\s+/g, " ").trim();
  if (!normalized) return "";
  return normalized.length > 96 ? `${normalized.slice(0, 96)}…` : normalized;
}

export function PlannerRecoveryShelf({
  onSend,
  disabled = false,
  fetchOnMount = true,
  refreshSignal,
  initialCheckpoints = [],
  recoverCheckpoint = api.plannerCheckpointRecover,
  resumeCheckpoint = api.plannerCheckpointResumeTask,
  resumePlan = api.plannerCheckpointResumePlan,
  getResumePlanJob = api.plannerCheckpointResumePlanJob,
  getTask = api.taskGet,
  getCheckpointDetails = async (planId: string) => {
    const res = await api.plannerCheckpoints({ limit: 1, includeSnapshot: true, planId });
    return (res.checkpoints || []).find((cp) => cp.plan_id === planId);
  },
  getExecutionState,
}: PlannerRecoveryShelfProps) {
  const { t } = useI18n();
  const [items, setItems] = useState<PlannerCheckpointSummary[]>(initialCheckpoints);
  const [loading, setLoading] = useState(fetchOnMount);
  const [recoveringKey, setRecoveringKey] = useState<string | null>(null);
  const [planNotice, setPlanNotice] = useState("");
  const [resumeTask, setResumeTask] = useState<{ id: string; status: string; title?: string; error?: string } | null>(null);
  const [resumePlanJob, setResumePlanJob] = useState<(Pick<PlannerCheckpointResumePlanJob, "id" | "plan_id" | "status" | "friendly_error" | "error" | "events" | "result" | "next_action" | "recoverable" | "session_id" | "primary_target"> & { planId: string }) | null>(null);
  const [refreshingTask, setRefreshingTask] = useState(false);
  const [refreshingResumePlanJob, setRefreshingResumePlanJob] = useState(false);
  const [expandedPlanId, setExpandedPlanId] = useState<string | null>(null);
  const [detailsOpen, setDetailsOpen] = useState(false);
  const [detailCheckpoint, setDetailCheckpoint] = useState<PlannerCheckpointSummary | null>(null);
  const [detailLoadingPlanId, setDetailLoadingPlanId] = useState<string | null>(null);
  const [detailError, setDetailError] = useState("");
  const [recoveryTargets, setRecoveryTargets] = useState<Record<string, PlannerExecutionStateRecoveryTarget | null>>({});
  const requestedRecoveryTargetsRef = useRef<Set<string>>(new Set());

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const res = await api.plannerCheckpoints({ limit: 5 });
      setItems(res.checkpoints || []);
    } catch {
      // Silent by design: the shelf is an assistive recovery affordance, not a
      // blocking error surface for chat.
      setItems([]);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    if (fetchOnMount) void load();
  }, [fetchOnMount, load]);

  useEffect(() => {
    if (fetchOnMount && refreshSignal !== undefined) void load();
  }, [fetchOnMount, load, refreshSignal]);

  const recoverable = useMemo(
    () => items.filter((cp) => cp.recoverable || cp.error || cp.status === "failed").slice(0, 3),
    [items],
  );
  const resumePlanJobCheckpoint = useMemo(
    () => resumePlanJob ? items.find((cp) => cp.plan_id === resumePlanJob.planId) || recoverable.find((cp) => cp.plan_id === resumePlanJob.planId) : undefined,
    [items, recoverable, resumePlanJob],
  );
  const fetchExecutionState = getExecutionState || (fetchOnMount ? api.plannerExecutionState : undefined);

  useEffect(() => {
    if (!detailsOpen || !fetchExecutionState || recoverable.length === 0) return;
    const missing = recoverable
      .map((cp) => cp.plan_id)
      .filter((planId) => !requestedRecoveryTargetsRef.current.has(planId));
    if (missing.length === 0) return;
    let cancelled = false;
    missing.forEach((planId) => requestedRecoveryTargetsRef.current.add(planId));
    missing.forEach((planId) => {
      void (async () => {
        try {
          const state = await fetchExecutionState(planId);
          const target = resolvePlannerRecoveryTarget(state.failure_summary?.primary_target, state.plan_id || planId);
          if (!cancelled) {
            setRecoveryTargets((prev) => ({ ...prev, [planId]: target?.href ? target : null }));
          }
        } catch {
          if (!cancelled) {
            setRecoveryTargets((prev) => ({ ...prev, [planId]: null }));
          }
        }
      })();
    });
    return () => {
      cancelled = true;
    };
  }, [detailsOpen, fetchExecutionState, recoverable]);

  if (!loading && recoverable.length === 0) return null;

  async function sendRecovery(cp: PlannerCheckpointSummary, action: PlannerCheckpointRecoveryAction) {
    const key = `${cp.plan_id}:${action}`;
    setRecoveringKey(key);
    setResumePlanJob(null);
    try {
      const res = await recoverCheckpoint(cp.plan_id, action);
      const label = recoveryPlanLabel(res.recovery_plan, action);
      if (label) setPlanNotice(label);
      onSend(res.prompt);
    } catch {
      // Keep the user unblocked if an older sidecar lacks the semantic API.
      setPlanNotice(action === "partial" ? "将先返回已完成部分" : "已生成兼容恢复提示");
      onSend(fallbackPlannerCheckpointPrompt(cp, action));
    } finally {
      setRecoveringKey(null);
    }
  }

  async function createResumeTask(cp: PlannerCheckpointSummary) {
    const key = `${cp.plan_id}:resume_task`;
    setRecoveringKey(key);
    setResumePlanJob(null);
    try {
      const res = await resumeCheckpoint(cp.plan_id, "continue", { run: true });
      const label = recoveryPlanLabel(res.recovery_plan, "continue");
      setPlanNotice(`已创建后台恢复任务 ${res.task_id}${label ? `：${label}` : ""}`);
      setResumeTask({ id: res.task_id, status: res.status });
      try {
        const task = await getTask(res.task_id);
        setResumeTask({ id: task.id, status: task.status, title: task.title, error: task.error });
      } catch {
        // The task was accepted; status refresh is best-effort.
      }
    } catch {
      setPlanNotice("暂时不能创建后台恢复任务，可先用“继续执行”恢复。");
    } finally {
      setRecoveringKey(null);
    }
  }

  async function runResumePlan(cp: PlannerCheckpointSummary) {
    const key = `${cp.plan_id}:resume_plan`;
    setRecoveringKey(key);
    setResumePlanJob(null);
    try {
      const res = await resumePlan(cp.plan_id, "continue", { async: true });
      if (res.status === "accepted" && res.job_id) {
        setPlanNotice(`已开始原规划续跑：${res.job_id}`);
        setResumePlanJob({ id: res.job_id, plan_id: cp.plan_id, planId: cp.plan_id, status: "running", session_id: cp.session_id, primary_target: res.primary_target });
        return;
      }
      if (res.status === "failed") {
        setPlanNotice(res.friendly_error || "原规划续跑暂未完成，现场已保留，可进入详情页继续处理。");
        return;
      }
      const label = recoveryPlanLabel(res.recovery_plan, "continue");
      const done = res.result?.plan?.filter((step) => step.status === "done" || step.status === "completed" || step.status === "skipped").length;
      const total = res.result?.plan?.length;
      const progress = typeof done === "number" && typeof total === "number" && total > 0 ? `，完成 ${done}/${total}` : "";
      setPlanNotice(`已按原规划续跑完成${progress}${label ? `：${label}` : ""}`);
    } catch {
      setPlanNotice("暂时不能直接按原规划续跑，可先用“后台续跑”保留现场。");
    } finally {
      setRecoveringKey(null);
    }
  }

  async function loadLatestResumePlanJob(cp: PlannerCheckpointSummary) {
    const key = `${cp.plan_id}:latest_resume_plan`;
    setRecoveringKey(key);
    try {
      const res = await getResumePlanJob({ planId: cp.plan_id });
      const job = res.job;
      if (!job?.id) {
        setPlanNotice("暂时没有最近续跑状态，可直接从当前恢复点继续。");
        return;
      }
      setResumePlanJob({
        id: job.id,
        plan_id: job.plan_id,
        planId: job.plan_id,
        status: job.status,
        friendly_error: job.friendly_error,
        error: job.error,
        events: job.events,
        result: job.result,
        next_action: job.next_action,
        recoverable: job.recoverable,
        session_id: job.session_id,
        primary_target: job.primary_target,
      });
      setPlanNotice(`已读取最近续跑 ${job.id}：${resumePlanJobStatusLabel(job.status)}`);
    } catch {
      setPlanNotice("暂时不能读取最近续跑状态，可直接从当前恢复点继续。");
    } finally {
      setRecoveringKey(null);
    }
  }

  async function refreshResumePlanJobStatus(jobId: string) {
    setRefreshingResumePlanJob(true);
    try {
      const res = await getResumePlanJob(jobId);
      const job = res.job;
      setResumePlanJob({
        id: job.id,
        plan_id: job.plan_id,
        planId: job.plan_id,
        status: job.status,
        friendly_error: job.friendly_error,
        error: job.error,
        events: job.events,
        result: job.result,
        next_action: job.next_action,
        recoverable: job.recoverable,
        session_id: job.session_id,
        primary_target: job.primary_target,
      });
      const hint = resumePlanJobHint(job);
      setPlanNotice(`原规划续跑 ${job.id}：${resumePlanJobStatusLabel(job.status)}${hint ? "，现场已更新" : ""}`);
    } catch {
      setResumePlanJob((prev) => prev && prev.id === jobId ? { ...prev, friendly_error: "状态暂时无法刷新，可进入详情页查看。" } : prev);
      setPlanNotice("状态暂时无法刷新，续跑现场仍已保留，可进入详情页查看。");
    } finally {
      setRefreshingResumePlanJob(false);
    }
  }

  async function refreshResumeTaskStatus(taskId: string) {
    setRefreshingTask(true);
    try {
      const task = await getTask(taskId);
      setResumeTask({ id: task.id, status: task.status, title: task.title, error: task.error });
    } catch {
      setResumeTask((prev) => prev && prev.id === taskId ? { ...prev, error: "状态暂时无法刷新，可稍后再试。" } : prev);
    } finally {
      setRefreshingTask(false);
    }
  }

  async function toggleCheckpointDetails(cp: PlannerCheckpointSummary) {
    if (expandedPlanId === cp.plan_id) {
      setExpandedPlanId(null);
      setDetailError("");
      return;
    }
    setExpandedPlanId(cp.plan_id);
    setDetailError("");
    if (cp.plan_snapshot?.length) {
      setDetailCheckpoint(cp);
      return;
    }
    setDetailLoadingPlanId(cp.plan_id);
    try {
      const detailed = await getCheckpointDetails(cp.plan_id);
      setDetailCheckpoint(detailed || cp);
      if (!detailed?.plan_snapshot?.length) setDetailError("暂时没有步骤快照，可先用恢复动作继续。");
    } catch {
      setDetailCheckpoint(cp);
      setDetailError("暂时不能读取步骤快照，可稍后再试。");
    } finally {
      setDetailLoadingPlanId(null);
    }
  }

  async function inspectCheckpointDependencies(cp: PlannerCheckpointSummary) {
    if (expandedPlanId !== cp.plan_id) {
      await toggleCheckpointDetails(cp);
    }
    setPlanNotice("已展开依赖视图，请先确认被阻塞步骤的前置依赖。");
  }

  const resumeTaskHint = taskRecoveryHint(resumeTask?.status, resumeTask?.error);
  const firstRecoverable = recoverable[0];
  const firstRecoverableSessionLabel = compactSessionLabel(firstRecoverable?.session_id);
  const summaryLabel = firstRecoverable
    ? [statusLabel(firstRecoverable), firstRecoverableSessionLabel, `${firstRecoverable.completed ?? 0}/${firstRecoverable.total ?? 0}`].filter(Boolean).join(" · ")
    : loading ? "正在检查" : "";
  const resumePlanJobSessionLabel = compactSessionLabel(
    resumePlanJob?.session_id ||
    resumePlanJob?.events?.find((event) => event.session_id)?.session_id ||
    resumePlanJobCheckpoint?.session_id,
  );
  const resumePlanJobRecoveryTarget = resolvePlannerRecoveryTarget(resumePlanJob?.primary_target, resumePlanJob?.planId);

  return (
    <div
      className="planner-recovery-shelf mx-5 mt-2 rounded-2xl border px-3 py-2 xl:mx-6"
      data-open={detailsOpen || undefined}
      style={{ background: "rgba(167,139,250,0.045)", borderColor: "rgba(167,139,250,0.14)" }}
    >
      <div className="flex items-center justify-between gap-3">
        <div className="flex items-center gap-2">
          <span className="flex h-6 w-6 items-center justify-center rounded-xl" style={{ background: "rgba(167,139,250,0.12)", color: "#c4b5fd" }}>
            <ArchiveRestore size={13} />
          </span>
          <div className="min-w-0">
            <div className="flex min-w-0 items-center gap-2">
              <span className="text-xs font-semibold" style={{ color: "var(--yunque-text)" }}>最近可恢复任务</span>
              {recoverable.length > 0 && (
                <span className="rounded-full px-2 py-0.5 text-[10px]" style={{ color: "#c4b5fd", background: "rgba(167,139,250,0.10)" }}>
                  {recoverable.length} 个
                </span>
              )}
              {summaryLabel && (
                <span className="truncate text-[11px]" style={{ color: "var(--yunque-text-muted)" }}>{summaryLabel}</span>
              )}
            </div>
            <div className="truncate text-[11px]" style={{ color: "var(--yunque-text-muted)" }}>
              现场已保留，需要时展开继续；默认不打断当前对话。
            </div>
          </div>
        </div>
        <div className="flex items-center gap-1">
          <Tooltip delay={0}>
            <Button isIconOnly variant="ghost" size="sm" onPress={() => void load()} isDisabled={loading || disabled} aria-label={t("planner.refreshRecoverable")}>
              {loading ? <Spinner size="sm" /> : <RefreshCw size={13} />}
            </Button>
            <Tooltip.Content>{t("planner.refreshRecoverable")}</Tooltip.Content>
          </Tooltip>
          <Button
            size="sm"
            variant="ghost"
            className="rounded-full px-2.5 text-[11px]"
            onPress={() => setDetailsOpen((open) => !open)}
            aria-expanded={detailsOpen}
          >
            {detailsOpen ? "收起" : "展开恢复任务"}
            <ChevronDown size={12} className={detailsOpen ? "rotate-180 transition-transform" : "transition-transform"} />
          </Button>
        </div>
      </div>

      {planNotice && (
        <div className="mt-2 flex flex-wrap items-center gap-2 rounded-xl px-2.5 py-1.5 text-[11px]" style={{ color: "#c4b5fd", background: "rgba(167,139,250,0.1)", border: "1px solid rgba(167,139,250,0.16)" }}>
          <span>{planNotice}</span>
          {resumePlanJob && (
            <>
              <span className="rounded-full px-2 py-0.5" style={{ color: "#7dd3fc", background: "rgba(14,165,233,0.08)", border: "1px solid rgba(14,165,233,0.18)" }}>
                {resumePlanJobStatusLabel(resumePlanJob.status)}
              </span>
              {resumePlanJobSessionLabel && <span style={{ color: "var(--yunque-text-muted)" }}>{resumePlanJobSessionLabel}</span>}
              {resumePlanJobHint(resumePlanJob) && <span style={{ color: "var(--yunque-text-muted)" }}>{resumePlanJobHint(resumePlanJob)}</span>}
              {resumePlanJobEventSummaries(resumePlanJob).map((summary, index) => (
                <span key={`${resumePlanJob.id}:event:${index}`} className="basis-full truncate" style={{ color: "var(--yunque-text-muted)" }}>
                  最近事件：{summary}
                </span>
              ))}
              <button
                type="button"
                disabled={disabled || refreshingResumePlanJob}
                onClick={() => void refreshResumePlanJobStatus(resumePlanJob.id)}
                className="rounded-full px-2 py-0.5 font-medium disabled:opacity-50"
                style={{ color: "#7dd3fc", background: "rgba(14,165,233,0.1)", border: "1px solid rgba(14,165,233,0.2)" }}
              >
                {refreshingResumePlanJob ? "刷新中" : "刷新续跑状态"}
              </button>
              <Link
                href={`/planner-checkpoint?plan_id=${encodeURIComponent(resumePlanJob.planId)}&job_id=${encodeURIComponent(resumePlanJob.id)}`}
                className="inline-flex items-center gap-1 rounded-full px-2 py-0.5 font-medium"
                style={{ color: "#7dd3fc", background: "rgba(14,165,233,0.1)", border: "1px solid rgba(14,165,233,0.2)" }}
              >
                查看续跑 <ExternalLink size={10} />
              </Link>
              {resumePlanJobRecoveryTarget?.href && (
                <Link
                  href={resumePlanJobRecoveryTarget.href}
                  className="inline-flex items-center gap-1 rounded-full px-2 py-0.5 font-medium"
                  style={{ color: "#c4b5fd", background: "rgba(167,139,250,0.1)", border: "1px solid rgba(167,139,250,0.22)" }}
                >
                  {resumePlanJobRecoveryTarget.label || "打开恢复目标"} <ExternalLink size={10} aria-hidden />
                </Link>
              )}
              {resumePlanJob.status === "failed" && resumePlanJobCheckpoint && (
                <>
                  {(resumePlanJob.next_action === "retry_failed" || (!resumePlanJob.next_action && resumePlanJob.recoverable)) && (
                    <button
                      type="button"
                      disabled={disabled || Boolean(recoveringKey)}
                      onClick={() => void sendRecovery(resumePlanJobCheckpoint, "retry_failed")}
                      className="rounded-full px-2 py-0.5 font-medium disabled:opacity-50"
                      style={{ color: "#fcd34d", background: "rgba(251,191,36,0.1)", border: "1px solid rgba(251,191,36,0.22)" }}
                    >
                      按建议重试
                    </button>
                  )}
                  {resumePlanJob.next_action === "create_task" && (
                    <button
                      type="button"
                      disabled={disabled || Boolean(recoveringKey)}
                      onClick={() => void createResumeTask(resumePlanJobCheckpoint)}
                      className="rounded-full px-2 py-0.5 font-medium disabled:opacity-50"
                      style={{ color: "#86efac", background: "rgba(34,197,94,0.1)", border: "1px solid rgba(34,197,94,0.22)" }}
                    >
                      转后台任务
                    </button>
                  )}
                  {resumePlanJob.next_action === "partial" && (
                    <button
                      type="button"
                      disabled={disabled || Boolean(recoveringKey)}
                      onClick={() => void sendRecovery(resumePlanJobCheckpoint, "partial")}
                      className="rounded-full px-2 py-0.5 font-medium disabled:opacity-50"
                      style={{ color: "#cbd5e1", background: "rgba(148,163,184,0.12)", border: "1px solid rgba(148,163,184,0.24)" }}
                    >
                      返回阶段结果
                    </button>
                  )}
                  {resumePlanJob.next_action === "inspect_dependencies" && (
                    <button
                      type="button"
                      disabled={disabled || detailLoadingPlanId === resumePlanJobCheckpoint.plan_id}
                      onClick={() => void inspectCheckpointDependencies(resumePlanJobCheckpoint)}
                      className="rounded-full px-2 py-0.5 font-medium disabled:opacity-50"
                      style={{ color: "#fcd34d", background: "rgba(251,191,36,0.1)", border: "1px solid rgba(251,191,36,0.22)" }}
                    >
                      查看依赖关系
                    </button>
                  )}
                </>
              )}
            </>
          )}
        </div>
      )}
      {resumeTask && (
        <div className="mt-2 flex flex-wrap items-center gap-2 rounded-xl px-2.5 py-1.5 text-[11px]" style={{ color: "var(--yunque-text-muted)", background: "rgba(34,197,94,0.08)", border: "1px solid rgba(34,197,94,0.16)" }}>
          <span style={{ color: "#86efac" }}>后台任务 {resumeTask.id}：{taskStatusLabel(resumeTask.status)}</span>
          {resumeTask.title && <span className="truncate">· {resumeTask.title}</span>}
          {resumeTaskHint && <span style={{ color: "#fca5a5" }}>· {resumeTaskHint}</span>}
          <button
            type="button"
            disabled={disabled || refreshingTask}
            onClick={() => void refreshResumeTaskStatus(resumeTask.id)}
            className="rounded-full px-2 py-0.5 font-medium disabled:opacity-50"
            style={{ color: "#86efac", background: "rgba(34,197,94,0.1)", border: "1px solid rgba(34,197,94,0.22)" }}
          >
            {refreshingTask ? "刷新中" : "刷新状态"}
          </button>
          <Link
            href={`/task-detail?id=${encodeURIComponent(resumeTask.id)}`}
            className="inline-flex items-center gap-1 rounded-full px-2 py-0.5 font-medium"
            style={{ color: "#c4b5fd", background: "rgba(167,139,250,0.1)", border: "1px solid rgba(167,139,250,0.2)" }}
          >
            查看任务 <ExternalLink size={10} />
          </Link>
        </div>
      )}
      {detailsOpen && (
        loading && recoverable.length === 0 ? (
          <div className="mt-2 flex items-center gap-2 rounded-xl px-2.5 py-2 text-[11px]" style={{ color: "var(--yunque-text-muted)", background: "rgba(255,255,255,0.04)" }}>
            <Spinner size="sm" /> 正在检查可恢复任务
          </div>
        ) : (
        <div className="mt-2 flex flex-col gap-2">
          {recoverable.map((cp) => {
            const sessionLabel = compactSessionLabel(cp.session_id);
            const recoveryTarget = recoveryTargets[cp.plan_id];
            return (
            <div key={`${cp.plan_id}:${cp.updated_at || ""}`} className="rounded-xl px-2.5 py-2" style={{ background: "rgba(0,0,0,0.16)", border: "1px solid var(--yunque-border)" }}>
              <div className="flex flex-wrap items-center gap-2">
                <div className="min-w-0 flex-1">
                  <div className="flex flex-wrap items-center gap-2">
                    <span className="truncate text-xs font-medium" style={{ color: "var(--yunque-text)" }}>{cp.goal || cp.plan_id}</span>
                    <span className="rounded-full px-2 py-0.5 text-[10px]" style={{ color: cp.error || cp.status === "failed" ? "#fca5a5" : "#c4b5fd", background: "rgba(255,255,255,0.06)" }}>
                      {statusLabel(cp)}
                    </span>
                    <span className="text-[10px]" style={{ color: "var(--yunque-text-muted)" }}>{cp.completed}/{cp.total}</span>
                    {sessionLabel && <span className="text-[10px]" style={{ color: "var(--yunque-text-muted)" }}>{sessionLabel}</span>}
                    {cp.updated_at && <span className="text-[10px]" style={{ color: "var(--yunque-text-muted)" }}>{formatTime(cp.updated_at)}</span>}
                  </div>
                  {cp.error && <div className="mt-1 truncate text-[11px]" style={{ color: "#fca5a5" }}>{checkpointErrorLabel(cp.error)}</div>}
                </div>
                <button
                  type="button"
                  disabled={disabled || detailLoadingPlanId === cp.plan_id}
                  onClick={() => void toggleCheckpointDetails(cp)}
                  className="rounded-full px-2.5 py-1 text-[11px] font-medium disabled:opacity-50"
                  style={{ background: "rgba(255,255,255,0.05)", color: "#cbd5e1", border: "1px solid rgba(148,163,184,0.2)" }}
                >
                  {expandedPlanId === cp.plan_id ? "收起步骤" : detailLoadingPlanId === cp.plan_id ? "读取中" : "查看步骤"}
                </button>
                <Link
                  href={`/planner-checkpoint?plan_id=${encodeURIComponent(cp.plan_id)}`}
                  className="inline-flex items-center gap-1 rounded-full px-2.5 py-1 text-[11px] font-medium"
                  style={{ background: "rgba(255,255,255,0.05)", color: "#cbd5e1", border: "1px solid rgba(148,163,184,0.2)" }}
                >
                  详情页 <ExternalLink size={10} />
                </Link>
                {recoveryTarget?.href && (
                  <Link
                    href={recoveryTarget.href}
                    className="inline-flex items-center gap-1 rounded-full px-2.5 py-1 text-[11px] font-medium"
                    style={{ background: "rgba(167,139,250,0.12)", color: "#c4b5fd", border: "1px solid rgba(167,139,250,0.26)" }}
                  >
                    {recoveryTarget.label || "打开恢复目标"} <ExternalLink size={10} aria-hidden />
                  </Link>
                )}
                <button
                  type="button"
                  disabled={disabled || Boolean(recoveringKey)}
                  onClick={() => void sendRecovery(cp, "continue")}
                  className="rounded-full px-2.5 py-1 text-[11px] font-medium disabled:opacity-50"
                  style={{ background: "rgba(167,139,250,0.14)", color: "#c4b5fd", border: "1px solid rgba(167,139,250,0.28)" }}
                >
                  {recoveringKey === `${cp.plan_id}:continue` ? "准备中" : "继续执行"}
                </button>
                <button
                  type="button"
                  disabled={disabled || Boolean(recoveringKey)}
                  onClick={() => void sendRecovery(cp, "retry_failed")}
                  className="flex items-center gap-1 rounded-full px-2.5 py-1 text-[11px] font-medium disabled:opacity-50"
                  style={{ background: "rgba(251,191,36,0.12)", color: "#fcd34d", border: "1px solid rgba(251,191,36,0.25)" }}
                >
                  <RotateCcw size={11} /> {recoveringKey === `${cp.plan_id}:retry_failed` ? "准备中" : "重试失败"}
                </button>
                <button
                  type="button"
                  disabled={disabled || Boolean(recoveringKey)}
                  onClick={() => void sendRecovery(cp, "partial")}
                  className="rounded-full px-2.5 py-1 text-[11px] font-medium disabled:opacity-50"
                  style={{ background: "rgba(148,163,184,0.12)", color: "#cbd5e1", border: "1px solid rgba(148,163,184,0.24)" }}
                >
                  {recoveringKey === `${cp.plan_id}:partial` ? "准备中" : "先返回阶段结果"}
                </button>
                <button
                  type="button"
                  disabled={disabled || Boolean(recoveringKey)}
                  onClick={() => void createResumeTask(cp)}
                  className="rounded-full px-2.5 py-1 text-[11px] font-medium disabled:opacity-50"
                  style={{ background: "rgba(34,197,94,0.12)", color: "#86efac", border: "1px solid rgba(34,197,94,0.24)" }}
                >
                  {recoveringKey === `${cp.plan_id}:resume_task` ? "创建中" : "后台续跑"}
                </button>
                <button
                  type="button"
                  disabled={disabled || Boolean(recoveringKey)}
                  onClick={() => void runResumePlan(cp)}
                  className="rounded-full px-2.5 py-1 text-[11px] font-medium disabled:opacity-50"
                  style={{ background: "rgba(14,165,233,0.12)", color: "#7dd3fc", border: "1px solid rgba(14,165,233,0.24)" }}
                >
                  {recoveringKey === `${cp.plan_id}:resume_plan` ? "续跑中" : "原规划续跑"}
                </button>
                <button
                  type="button"
                  disabled={disabled || Boolean(recoveringKey)}
                  onClick={() => void loadLatestResumePlanJob(cp)}
                  className="rounded-full px-2.5 py-1 text-[11px] font-medium disabled:opacity-50"
                  style={{ background: "var(--yunque-accent-muted)", color: "var(--yunque-accent-strong)", border: "1px solid var(--yunque-border-accent)" }}
                >
                  {recoveringKey === `${cp.plan_id}:latest_resume_plan` ? "读取中" : "最近续跑"}
                </button>
              </div>
              {expandedPlanId === cp.plan_id && (
                <div className="mt-2 rounded-xl px-2.5 py-2 text-[11px]" style={{ background: "rgba(255,255,255,0.035)", border: "1px solid rgba(148,163,184,0.14)", color: "var(--yunque-text-muted)" }}>
                  {detailLoadingPlanId === cp.plan_id ? (
                    <div className="flex items-center gap-2"><Spinner size="sm" /> 正在读取步骤快照</div>
                  ) : detailError ? (
                    <div>{detailError}</div>
                  ) : ((detailCheckpoint?.plan_id === cp.plan_id ? detailCheckpoint.plan_snapshot : cp.plan_snapshot) || []).length > 0 ? (
                    (() => {
                      const snapshot = (detailCheckpoint?.plan_id === cp.plan_id ? detailCheckpoint.plan_snapshot : cp.plan_snapshot) || [];
                      const counts = recoveryStepGraphCounts(snapshot);
                      return (
                        <div className="flex flex-col gap-2">
                          <div className="flex flex-wrap gap-1.5 text-[10px]">
                            <span className="rounded-full px-2 py-0.5" style={{ color: "#7dd3fc", background: "rgba(14,165,233,0.1)" }}>可执行 {counts["可执行"] || 0}</span>
                            <span className="rounded-full px-2 py-0.5" style={{ color: "#fcd34d", background: "rgba(251,191,36,0.1)" }}>被阻塞 {counts["被阻塞"] || 0}</span>
                            <span className="rounded-full px-2 py-0.5" style={{ color: "#86efac", background: "rgba(34,197,94,0.1)" }}>已完成 {counts["已完成"] || 0}</span>
                            <span className="rounded-full px-2 py-0.5" style={{ color: "#fca5a5", background: "rgba(239,68,68,0.1)" }}>需处理 {counts["需处理"] || 0}</span>
                          </div>
                          <div className="flex flex-col gap-1.5">
                            {snapshot.map((step) => {
                              const graph = recoveryStepDependencyState(step, snapshot);
                              return (
                                <div
                                  key={step.id}
                                  className="flex flex-wrap items-center gap-2 rounded-xl px-2 py-1.5"
                                  style={{
                                    background: "rgba(0,0,0,0.12)",
                                    border: graph.label === "执行中" ? "1px solid var(--yunque-border-accent)" : `1px solid ${graph.color}33`,
                                  }}
                                >
                                  <span className="rounded-full px-1.5 py-0.5 text-[10px]" style={{ color: "#cbd5e1", background: "rgba(255,255,255,0.06)" }}>#{step.id}</span>
                                  <span className="rounded-full px-1.5 py-0.5 text-[10px]" style={{ color: graph.color, background: "rgba(255,255,255,0.06)" }}>{graph.label}</span>
                                  <span className="rounded-full px-1.5 py-0.5 text-[10px]" style={{ color: step.status === "failed" ? "#fca5a5" : "#c4b5fd", background: "rgba(255,255,255,0.06)" }}>{stepStatusLabel(step.status)}</span>
                                  {step.skill && <span className="text-[10px]" style={{ color: "var(--yunque-accent-strong)" }}>{step.skill}</span>}
                                  <span className="min-w-0 flex-1 truncate" style={{ color: "var(--yunque-text)" }}>{step.action}</span>
                                  {step.depends_on?.length ? <span className="text-[10px]">依赖：{step.depends_on.join(", ")}</span> : null}
                                  {graph.hint && <span className="basis-full text-[10px]" style={{ color: graph.color }}>{graph.hint}</span>}
                                  {stepResultPreview(step.result) && (
                                    <span className="basis-full truncate text-[10px]" style={{ color: "#86efac" }}>
                                      已保留证据：{stepResultPreview(step.result)}
                                    </span>
                                  )}
                                </div>
                              );
                            })}
                          </div>
                        </div>
                      );
                    })()
                  ) : (
                    <div>暂无步骤快照，可先用恢复动作继续。</div>
                  )}
                </div>
              )}
            </div>
            );
          })}
        </div>
        )
      )}
    </div>
  );
}
