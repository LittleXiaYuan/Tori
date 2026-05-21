"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { Button, Card, Chip, Spinner } from "@heroui/react";
import { ArrowLeft, ExternalLink, FileText, GitBranch, Play, RefreshCw } from "lucide-react";
import { api, BASE, getAuthHeaders } from "@/lib/api";
import type { PlannerCheckpointRecoveryAction, PlannerCheckpointResumePlanJob, PlannerCheckpointResumePlanJobEvent, PlannerCheckpointResumePlanJobResponse, PlannerCheckpointResumePlanResponse, PlannerCheckpointResumeTaskResponse, PlannerCheckpointStep, PlannerCheckpointSummary, PlannerExecutionStateResponse } from "@/lib/api-types";
import { formatErrorMessage } from "@/lib/error-utils";

interface PlannerCheckpointDetailProps {
  planId?: string;
  initialCheckpoint?: PlannerCheckpointSummary | null;
  fetchCheckpoint?: (planId: string) => Promise<PlannerCheckpointSummary | null>;
  resumeCheckpoint?: (planId: string, action: PlannerCheckpointRecoveryAction, options?: { run?: boolean }) => Promise<Pick<PlannerCheckpointResumeTaskResponse, "task_id" | "status" | "recovery_plan">>;
  resumePlan?: (planId: string, action: PlannerCheckpointRecoveryAction, options?: { async?: boolean }) => Promise<Pick<PlannerCheckpointResumePlanResponse, "status" | "job_id" | "result" | "recovery_plan">>;
  getResumePlanJob?: (jobIdOrParams: string | { jobId?: string; planId?: string }) => Promise<PlannerCheckpointResumePlanJobResponse>;
  fetchExecutionState?: (planId: string) => Promise<PlannerExecutionStateResponse | null>;
  initialResumePlanJobId?: string;
  subscribeResumePlanEvents?: (jobId: string, onEvent: (event: PlannerCheckpointResumePlanJobEvent) => void) => () => void;
}


function appendResumePlanEvent(events: PlannerCheckpointResumePlanJobEvent[], event: PlannerCheckpointResumePlanJobEvent): PlannerCheckpointResumePlanJobEvent[] {
  if (!event || (!event.id && !event.summary)) return events;
  const exists = event.id ? events.some((item) => item.id === event.id) : false;
  if (exists) return events;
  const next = [...events, event];
  return next.length > 80 ? next.slice(next.length - 80) : next;
}

function parseSSEFrame(frame: string): { event: string; data: string } | null {
  let event = "message";
  const data: string[] = [];
  for (const rawLine of frame.split(/\r?\n/)) {
    const line = rawLine.trimEnd();
    if (!line || line.startsWith(":")) continue;
    if (line.startsWith("event:")) event = line.slice(6).trim();
    if (line.startsWith("data:")) data.push(line.slice(5).trimStart());
  }
  if (data.length === 0) return null;
  return { event, data: data.join("\n") };
}

function subscribeResumePlanEventsDefault(jobId: string, onEvent: (event: PlannerCheckpointResumePlanJobEvent) => void): () => void {
  const controller = new AbortController();
  let cancelled = false;
  (async () => {
    try {
      const res = await fetch(`${BASE}/v1/events/stream`, { headers: getAuthHeaders(), signal: controller.signal });
      if (!res.ok || !res.body) return;
      const reader = res.body.getReader();
      const decoder = new TextDecoder();
      let buffer = "";
      while (!cancelled) {
        const { done, value } = await reader.read();
        if (done) break;
        buffer += decoder.decode(value, { stream: true });
        const frames = buffer.split(/\r?\n\r?\n/);
        buffer = frames.pop() || "";
        for (const frame of frames) {
          const parsed = parseSSEFrame(frame);
          if (!parsed || parsed.event !== "planner.resume_plan_event") continue;
          try {
            const payload = JSON.parse(parsed.data) as { job_id?: string; event?: PlannerCheckpointResumePlanJobEvent };
            if (payload.job_id === jobId && payload.event) onEvent(payload.event);
          } catch { /* ignore malformed event frame */ }
        }
      }
    } catch { /* event stream is best-effort; manual refresh still works */ }
  })();
  return () => {
    cancelled = true;
    controller.abort();
  };
}

async function fetchExecutionStateDefault(planId: string): Promise<PlannerExecutionStateResponse | null> {
  return api.plannerExecutionState(planId);
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

function stepStatusColor(status?: string): string {
  switch (status) {
    case "done":
    case "completed": return "#86efac";
    case "failed": return "#fca5a5";
    case "running": return "#93c5fd";
    case "skipped": return "#cbd5e1";
    default: return "#c4b5fd";
  }
}

function eventSummaryLabel(summary?: string): string {
  return formatErrorMessage(summary, "这一步状态已更新，现场已保留。");
}

function recoveryActionLabel(action?: string): string {
  switch (action) {
    case "retry_failed": return "重试失败步骤";
    case "create_task": return "转为后台任务";
    case "partial": return "先返回阶段结果";
    case "inspect_dependencies": return "先查看依赖关系";
    case "continue": return "继续执行";
    default: return action ? formatErrorMessage(action, "继续恢复") : "";
  }
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

function isStepComplete(status?: string): boolean {
  return status === "done" || status === "completed" || status === "skipped";
}

type PlannerGraphState = "done" | "ready" | "blocked" | "running" | "failed" | "pending" | "unknown";

function getPlannerGraphState(step: PlannerCheckpointStep, stepById: Map<number, PlannerCheckpointStep>): { state: PlannerGraphState; label: string; color: string; hint?: string } {
  const deps = Array.from(new Set(step.depends_on || []));
  const actualStatus = (step.status || "").toLowerCase();
  if (isStepComplete(actualStatus)) {
    return { state: "done", label: "已完成", color: "#86efac", hint: "这一步已完成，现场已保留。" };
  }
  if (actualStatus === "running") {
    return { state: "running", label: "执行中", color: "#93c5fd", hint: "这一步正在执行。" };
  }
  if (actualStatus === "failed") {
    return { state: "failed", label: "需处理", color: "#fca5a5", hint: "这一步失败了，建议先看失败原因。" };
  }
  const unmetDeps = deps.filter((depId) => {
    const dep = stepById.get(depId);
    return !dep || !isStepComplete(dep.status);
  });
  if (deps.length > 0 && unmetDeps.length > 0) {
    return {
      state: "blocked",
      label: "被阻塞",
      color: "#fcd34d",
      hint: `阻塞于：${unmetDeps.map((dep) => `#${dep}`).join("、")}`,
    };
  }
  if (deps.length > 0) {
    return {
      state: "ready",
      label: "可执行",
      color: "#7dd3fc",
      hint: "前置步骤已满足，可以继续推进。",
    };
  }
  if (actualStatus === "pending" || actualStatus === "") {
    return { state: "pending", label: "待执行", color: "#c4b5fd", hint: "还未开始执行。" };
  }
  return { state: "unknown", label: actualStatus || "待确认", color: "#cbd5e1" };
}

function graphStateBackground(state: PlannerGraphState): string {
  switch (state) {
    case "done": return "rgba(34,197,94,0.08)";
    case "ready": return "rgba(14,165,233,0.08)";
    case "blocked": return "rgba(251,191,36,0.08)";
    case "running": return "rgba(59,130,246,0.08)";
    case "failed": return "rgba(239,68,68,0.08)";
    case "pending": return "rgba(167,139,250,0.08)";
    default: return "rgba(255,255,255,0.03)";
  }
}

function isResumePlanTerminalEvent(event: PlannerCheckpointResumePlanJobEvent): boolean {
  if (event.type === "planner.resume_plan_done") return true;
  const summary = event.summary || "";
  return /续跑已完成|续跑没有顺利完成|现场已保留/.test(summary);
}

export function PlannerCheckpointDetail({
  planId = "",
  initialCheckpoint = null,
  fetchCheckpoint = async (id: string) => {
    const res = await api.plannerCheckpoints({ limit: 1, includeSnapshot: true, planId: id });
    return (res.checkpoints || []).find((cp) => cp.plan_id === id) || null;
  },
  resumeCheckpoint = api.plannerCheckpointResumeTask,
  resumePlan = api.plannerCheckpointResumePlan,
  getResumePlanJob = api.plannerCheckpointResumePlanJob,
  fetchExecutionState = fetchExecutionStateDefault,
  initialResumePlanJobId = "",
  subscribeResumePlanEvents = subscribeResumePlanEventsDefault,
}: PlannerCheckpointDetailProps) {
  const [checkpoint, setCheckpoint] = useState<PlannerCheckpointSummary | null>(initialCheckpoint);
  const [loading, setLoading] = useState(Boolean(planId) && !initialCheckpoint);
  const [error, setError] = useState("");
  const [actionKey, setActionKey] = useState("");
  const [notice, setNotice] = useState("");
  const [resumeTaskId, setResumeTaskId] = useState("");
  const [resumePlanJobId, setResumePlanJobId] = useState(initialResumePlanJobId);
  const [resumePlanJob, setResumePlanJob] = useState<PlannerCheckpointResumePlanJob | null>(null);
  const [resumePlanEvents, setResumePlanEvents] = useState<PlannerCheckpointResumePlanJobEvent[]>([]);
  const [resumePlanResultSteps, setResumePlanResultSteps] = useState<PlannerCheckpointStep[]>([]);
  const [executionState, setExecutionState] = useState<PlannerExecutionStateResponse | null>(null);
  const [partialReply, setPartialReply] = useState("");

  const load = useCallback(async () => {
    if (!planId) {
      setError("缺少 plan_id，无法读取恢复点。");
      setLoading(false);
      return;
    }
    setLoading(true);
    setError("");
    try {
      const cp = await fetchCheckpoint(planId);
      setCheckpoint(cp);
      if (!cp) setError("没有找到这个恢复点，可能已被清理或尚未写入。");
    } catch {
      setError("暂时不能读取恢复点，请稍后重试。");
    } finally {
      setLoading(false);
    }
  }, [fetchCheckpoint, planId]);

  useEffect(() => {
    if (!initialCheckpoint) void load();
  }, [initialCheckpoint, load]);

  useEffect(() => {
    if (!resumePlanJobId) return;
    return subscribeResumePlanEvents(resumePlanJobId, (event) => {
      setResumePlanEvents((prev) => {
        const next = appendResumePlanEvent(prev, event);
        setExecutionState((state) => state ? { ...state, events: next } : state);
        return next;
      });
      if (isResumePlanTerminalEvent(event)) {
        void refreshResumePlanJob(resumePlanJobId, true);
      }
    });
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [resumePlanJobId, subscribeResumePlanEvents]);

  const steps = resumePlanResultSteps.length > 0 ? resumePlanResultSteps : checkpoint?.plan_snapshot || [];
  const done = useMemo(() => steps.filter((step) => step.status === "done" || step.status === "completed" || step.status === "skipped").length, [steps]);

  function rememberResumePlanJob(jobId: string) {
    if (!jobId || typeof window === "undefined") return;
    const url = new URL(window.location.href);
    url.searchParams.set("job_id", jobId);
    window.history.replaceState(null, "", `${url.pathname}?${url.searchParams.toString()}`);
  }

  function syncExecutionStateJob(job: PlannerCheckpointResumePlanJob) {
    setExecutionState((prev) => {
      if (!prev && !checkpoint) return prev;
      const stateCheckpoint = prev?.checkpoint || checkpoint || undefined;
      return {
        plan_id: job.plan_id || stateCheckpoint?.plan_id || planId,
        status: job.status || prev?.status || stateCheckpoint?.status || "running",
        action: job.action || prev?.action || "continue",
        next_action: job.next_action || prev?.next_action,
        updated_at: job.finished_at || job.started_at || prev?.updated_at,
        checkpoint: stateCheckpoint,
        latest_job: job,
        recovery_plan: prev?.recovery_plan,
        failure_summary: prev?.failure_summary,
        events: job.events || prev?.events || [],
      };
    });
  }

  function applyResumePlanJob(job: PlannerCheckpointResumePlanJob, quiet = false) {
    setResumePlanJob(job);
    setResumePlanJobId(job.id);
    rememberResumePlanJob(job.id);
    setResumePlanEvents(job.events || []);
    setResumePlanResultSteps(job.result?.plan || []);
    syncExecutionStateJob(job);
    if (job.status === "completed") {
      const resultSteps = job.result?.plan || [];
      const resultDone = resultSteps.filter((step) => step.status === "done" || step.status === "completed" || step.status === "skipped").length;
      setNotice(resultSteps.length > 0 ? `原规划续跑已完成：${resultDone}/${resultSteps.length}` : "原规划续跑已完成。");
    } else if (job.status === "failed") {
      setNotice("原规划续跑遇到问题，可选择下面的恢复方式继续。");
    } else if (!quiet) {
      setNotice(`原规划续跑仍在进行：${job.id}`);
    }
  }

  async function createResumeTask() {
    if (!checkpoint) return;
    setActionKey("task");
    setNotice("");
    setResumeTaskId("");
    setResumePlanJob(null);
    setResumePlanEvents([]);
    setResumePlanResultSteps([]);
    try {
      const res = await resumeCheckpoint(checkpoint.plan_id, "continue", { run: true });
      setResumeTaskId(res.task_id);
      setPartialReply("");
      setNotice(`已创建后台恢复任务：${res.task_id}`);
    } catch {
      setNotice("暂时不能创建后台恢复任务，请稍后重试。");
    } finally {
      setActionKey("");
    }
  }

  async function runResumePlan(action: PlannerCheckpointRecoveryAction = "continue") {
    if (!checkpoint) return;
    setActionKey(action === "retry_failed" ? "plan_retry" : "plan");
    setNotice("");
    setResumePlanJobId("");
    setResumePlanJob(null);
    setResumePlanEvents([]);
    setResumePlanResultSteps([]);
    try {
      const res = await resumePlan(checkpoint.plan_id, action, { async: true });
      if (res.status === "accepted" && res.job_id) {
        setResumePlanJobId(res.job_id);
        const runningJob: PlannerCheckpointResumePlanJob = {
          id: res.job_id,
          status: "running",
          action,
          plan_id: checkpoint.plan_id,
          task_id: checkpoint.task_id,
          started_at: new Date().toISOString(),
        };
        setResumePlanJob(runningJob);
        syncExecutionStateJob(runningJob);
        rememberResumePlanJob(res.job_id);
        setResumePlanEvents([]);
        setPartialReply("");
        setNotice(action === "retry_failed" ? `已开始重试失败步骤：${res.job_id}` : `已开始原规划续跑：${res.job_id}`);
        return;
      }
      const resultSteps = res.result?.plan || [];
      setResumePlanResultSteps(resultSteps);
      const resultDone = resultSteps.filter((step) => step.status === "done" || step.status === "completed" || step.status === "skipped").length;
      setPartialReply("");
      setNotice(resultSteps.length > 0 ? `已按原规划续跑完成：${resultDone}/${resultSteps.length}` : "已按原规划续跑完成。");
    } catch {
      setNotice("暂时不能按原规划直接续跑，可先创建后台恢复任务。");
    } finally {
      setActionKey("");
    }
  }


  async function returnPartialResult() {
    if (!checkpoint) return;
    setActionKey("partial");
    setNotice("");
    setPartialReply("");
    setResumePlanJob(null);
    setResumePlanEvents([]);
    setResumePlanResultSteps([]);
    try {
      const res = await resumePlan(checkpoint.plan_id, "partial");
      const reply = (res.result?.reply || "").trim();
      setPartialReply(reply || "暂时没有可展示的阶段结果，可先刷新恢复点或创建后台恢复任务。");
      setNotice("已整理当前阶段结果，不会继续执行未完成步骤。");
    } catch {
      setNotice("暂时不能整理阶段结果，请稍后重试或返回聊天页继续恢复。");
    } finally {
      setActionKey("");
    }
  }

  async function refreshResumePlanJob(jobId = resumePlanJobId, quiet = false) {
    if (!jobId) return;
    if (!quiet) setActionKey("plan_job");
    try {
      const res = await getResumePlanJob(jobId);
      applyResumePlanJob(res.job, quiet);
    } catch {
      if (!quiet) setNotice("暂时不能刷新原规划续跑状态，请稍后重试。");
    } finally {
      if (!quiet) setActionKey("");
    }
  }

  useEffect(() => {
    if (!initialResumePlanJobId) return;
    setResumePlanJobId(initialResumePlanJobId);
    void refreshResumePlanJob(initialResumePlanJobId, true);
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [initialResumePlanJobId]);

  useEffect(() => {
    if (!planId || initialResumePlanJobId || resumePlanJobId) return;
    let cancelled = false;
    (async () => {
      try {
        const res = await getResumePlanJob({ planId });
        if (cancelled || !res.job?.id) return;
        applyResumePlanJob(res.job, true);
      } catch { /* latest job lookup is best-effort; users can still start a new resume */ }
    })();
    return () => {
      cancelled = true;
    };
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [planId, initialResumePlanJobId, resumePlanJobId]);

  useEffect(() => {
    if (!planId || !checkpoint) return;
    let cancelled = false;
    (async () => {
      try {
        const state = await fetchExecutionState(planId);
        if (cancelled) return;
        setExecutionState(state);
      } catch { /* execution-state is a read-model convenience; existing detail UI remains usable */ }
    })();
    return () => {
      cancelled = true;
    };
  }, [checkpoint, fetchExecutionState, planId]);

  const failedResumePlanJob = resumePlanJob?.status === "failed" ? resumePlanJob : null;
  const failedResumePlanMessage = failedResumePlanJob
    ? failedResumePlanJob.friendly_error || formatErrorMessage(failedResumePlanJob.error, "原规划续跑遇到问题，可选择下面的恢复方式继续。")
    : "";
  const stepById = useMemo(() => new Map(steps.map((step) => [step.id, step])), [steps]);
  const graphNodes = useMemo(() => steps.map((step) => {
    const deps = Array.from(new Set(step.depends_on || []));
    const outgoing = steps.filter((next) => next.depends_on?.includes(step.id)).map((next) => next.id);
    const graph = getPlannerGraphState(step, stepById);
    const completedDeps = deps.filter((depId) => isStepComplete(stepById.get(depId)?.status));
    return {
      step,
      deps,
      outgoing,
      graph,
      completedDeps,
      blockedDeps: graph.state === "blocked" ? deps.filter((depId) => !isStepComplete(stepById.get(depId)?.status)) : [],
    };
  }), [steps, stepById]);
  const graphCounts = useMemo(() => graphNodes.reduce((acc, node) => {
    acc[node.graph.state] = (acc[node.graph.state] || 0) + 1;
    return acc;
  }, {} as Record<PlannerGraphState, number>), [graphNodes]);

  return (
    <main className="min-h-screen px-5 py-6" style={{ background: "var(--yunque-bg)", color: "var(--yunque-text)" }}>
      <div className="mx-auto flex max-w-5xl flex-col gap-4">
        <div className="flex flex-wrap items-center justify-between gap-3">
          <div className="flex items-center gap-3">
            <a href="/chat" className="inline-flex items-center gap-1 rounded-full px-3 py-1.5 text-sm" style={{ color: "var(--yunque-text-muted)", border: "1px solid var(--yunque-border)" }}>
              <ArrowLeft size={14} /> 返回聊天
            </a>
            <div>
              <h1 className="text-xl font-semibold">Planner 恢复点详情</h1>
              <p className="text-sm" style={{ color: "var(--yunque-text-muted)" }}>查看原始 DAG 步骤、依赖关系和可恢复现场。</p>
            </div>
          </div>
          <Button size="sm" variant="ghost" onPress={() => void load()} isDisabled={loading || !planId}>
            {loading ? <Spinner size="sm" /> : <RefreshCw size={14} />} 刷新
          </Button>
        </div>

        {loading ? (
          <Card className="section-card p-5">
            <div className="flex items-center gap-2 text-sm" style={{ color: "var(--yunque-text-muted)" }}>
              <Spinner size="sm" /> 正在读取恢复点
            </div>
          </Card>
        ) : error ? (
          <Card className="section-card p-5">
            <div className="text-sm" style={{ color: "#fca5a5" }}>{error}</div>
          </Card>
        ) : checkpoint ? (
          <>
            <Card className="section-card p-5">
              <div className="flex flex-wrap items-start justify-between gap-3">
                <div>
                  <div className="mb-1 text-xs" style={{ color: "var(--yunque-text-muted)" }}>Plan ID</div>
                  <div className="font-mono text-sm">{checkpoint.plan_id}</div>
                  {checkpoint.goal && <div className="mt-2 text-sm" style={{ color: "var(--yunque-text-muted)" }}>{checkpoint.goal}</div>}
                </div>
                <div className="flex flex-wrap gap-2">
                  <Chip size="sm" style={{ background: "rgba(167,139,250,0.12)", color: "#c4b5fd" }}>{checkpoint.status || "unknown"}</Chip>
                  <Chip size="sm" style={{ background: "rgba(34,197,94,0.1)", color: "#86efac" }}>{done}/{checkpoint.total || steps.length}</Chip>
                  {checkpoint.recoverable && <Chip size="sm" style={{ background: "rgba(251,191,36,0.1)", color: "#fcd34d" }}>可恢复</Chip>}
                </div>
              </div>
              {checkpoint.recoverable && (
                <div className="mt-4 flex flex-wrap items-center gap-2">
                  <Button size="sm" onPress={() => void createResumeTask()} isDisabled={Boolean(actionKey)}>
                    {actionKey === "task" ? <Spinner size="sm" /> : <Play size={14} />} 后台续跑
                  </Button>
                  <Button size="sm" variant="ghost" onPress={() => void runResumePlan()} isDisabled={Boolean(actionKey)}>
                    {actionKey === "plan" ? <Spinner size="sm" /> : <GitBranch size={14} />} 原规划续跑
                  </Button>
                  <Button size="sm" variant="ghost" onPress={() => void returnPartialResult()} isDisabled={Boolean(actionKey)}>
                    {actionKey === "partial" ? <Spinner size="sm" /> : <FileText size={14} />} 阶段结果
                  </Button>
                  {resumePlanJobId && (
                    <Button size="sm" variant="ghost" onPress={() => void refreshResumePlanJob()} isDisabled={Boolean(actionKey)}>
                      {actionKey === "plan_job" ? <Spinner size="sm" /> : <RefreshCw size={14} />} 刷新续跑状态
                    </Button>
                  )}
                  {resumeTaskId && (
                    <a
                      href={`/task-detail?id=${encodeURIComponent(resumeTaskId)}`}
                      className="inline-flex items-center gap-1 rounded-full px-3 py-1.5 text-sm"
                      style={{ color: "#c4b5fd", border: "1px solid rgba(167,139,250,0.2)", background: "rgba(167,139,250,0.08)" }}
                    >
                      查看任务 <ExternalLink size={12} />
                    </a>
                  )}
                </div>
              )}
              {notice && <div className="mt-3 rounded-xl px-3 py-2 text-sm" style={{ color: "#c4b5fd", background: "rgba(167,139,250,0.08)" }}>{notice}</div>}
              {failedResumePlanJob && (
                <div className="mt-3 rounded-2xl border p-3" style={{ borderColor: "rgba(251,191,36,0.28)", background: "rgba(251,191,36,0.08)" }}>
                  <div className="text-sm font-medium" style={{ color: "#fcd34d" }}>续跑没有顺利完成，现场已保留</div>
                  <div className="mt-1 text-sm" style={{ color: "var(--yunque-text-muted)" }}>{failedResumePlanMessage}</div>
                  <div className="mt-3 flex flex-wrap gap-2">
                    <Button size="sm" variant="ghost" onPress={() => void runResumePlan("retry_failed")} isDisabled={Boolean(actionKey)}>
                      {actionKey === "plan_retry" ? <Spinner size="sm" /> : <RefreshCw size={14} />} 重试失败步骤
                    </Button>
                    <Button size="sm" variant="ghost" onPress={() => void createResumeTask()} isDisabled={Boolean(actionKey)}>
                      {actionKey === "task" ? <Spinner size="sm" /> : <Play size={14} />} 转为后台任务
                    </Button>
                    <Button size="sm" variant="ghost" onPress={() => void returnPartialResult()} isDisabled={Boolean(actionKey)}>
                      {actionKey === "partial" ? <Spinner size="sm" /> : <FileText size={14} />} 返回阶段结果
                    </Button>
                    {failedResumePlanJob.next_action === "inspect_dependencies" && (
                      <Button size="sm" variant="ghost" onPress={() => document.getElementById("dependency-view")?.scrollIntoView({ behavior: "smooth", block: "start" })}>
                        <GitBranch size={14} /> 查看依赖关系
                      </Button>
                    )}
                  </div>
                </div>
              )}
              {partialReply && (
                <div className="mt-3 rounded-xl px-3 py-2" style={{ color: "var(--yunque-text)", background: "rgba(255,255,255,0.04)", border: "1px solid var(--yunque-border)" }}>
                  <div className="mb-1 text-xs font-medium" style={{ color: "var(--yunque-text-muted)" }}>阶段结果（已保留证据）</div>
                  <pre className="max-h-64 overflow-auto whitespace-pre-wrap text-sm">
                    {displayRecoveryText(partialReply)}
                  </pre>
                </div>
              )}
              {resumePlanEvents.length > 0 && (
                <div className="mt-3 rounded-2xl border p-3" style={{ borderColor: "var(--yunque-border)", background: "rgba(255,255,255,0.03)" }}>
                  <div className="mb-2 text-sm font-medium">原规划续跑过程</div>
                  <div className="flex flex-col gap-2">
                    {resumePlanEvents.slice(-8).map((evt) => (
                      <div key={evt.id || `${evt.type}-${evt.timestamp}`} className="flex flex-wrap items-center gap-2 text-xs" style={{ color: "var(--yunque-text-muted)" }}>
                        <span className="rounded-full px-2 py-0.5" style={{ background: "rgba(167,139,250,0.1)", color: "#c4b5fd" }}>{evt.type || "planner"}</span>
                        {evt.skill && <span className="rounded-full px-2 py-0.5" style={{ background: "rgba(14,165,233,0.1)", color: "#7dd3fc" }}>{evt.skill}</span>}
                        <span>{eventSummaryLabel(evt.summary)}</span>
                      </div>
                    ))}
                  </div>
                </div>
              )}
              {checkpoint.error && (
                <div className="mt-3 rounded-xl px-3 py-2 text-sm" style={{ color: "#fca5a5", background: "rgba(239,68,68,0.08)" }}>
                  {formatErrorMessage(checkpoint.error, "已保留失败现场，可从聊天页继续恢复。")}
                </div>
              )}
            </Card>

            {executionState && (
              <Card className="section-card p-5">
                <div className="mb-3 flex flex-wrap items-center justify-between gap-2">
                  <div>
                    <div className="text-sm font-medium">统一执行现场</div>
                    <div className="mt-1 text-xs" style={{ color: "var(--yunque-text-muted)" }}>由 checkpoint、续跑 Job、事件和失败摘要合并生成。</div>
                  </div>
                  <div className="flex flex-wrap gap-2">
                    <Chip size="sm" style={{ background: "rgba(167,139,250,0.12)", color: "#c4b5fd" }}>{executionState.status || "unknown"}</Chip>
                    <Chip size="sm" style={{ background: "rgba(14,165,233,0.1)", color: "#7dd3fc" }}>{executionState.action || "continue"}</Chip>
                    {executionState.next_action && <Chip size="sm" style={{ background: "rgba(251,191,36,0.1)", color: "#fcd34d" }}>下一步：{recoveryActionLabel(executionState.next_action)}</Chip>}
                  </div>
                </div>
                <div className="grid gap-3 md:grid-cols-3">
                  <div className="rounded-2xl border p-3" style={{ borderColor: "var(--yunque-border)", background: "rgba(255,255,255,0.03)" }}>
                    <div className="text-xs" style={{ color: "var(--yunque-text-muted)" }}>恢复点</div>
                    <div className="mt-1 text-sm">{executionState.checkpoint?.completed ?? checkpoint.completed}/{executionState.checkpoint?.total ?? checkpoint.total} 已完成</div>
                    {executionState.checkpoint?.recoverable && <div className="mt-1 text-xs" style={{ color: "#fcd34d" }}>现场可恢复</div>}
                  </div>
                  <div className="rounded-2xl border p-3" style={{ borderColor: "var(--yunque-border)", background: "rgba(255,255,255,0.03)" }}>
                    <div className="text-xs" style={{ color: "var(--yunque-text-muted)" }}>最近续跑</div>
                    <div className="mt-1 break-all text-sm">{executionState.latest_job?.id || "暂无 Job"}</div>
                    {executionState.latest_job?.friendly_error && <div className="mt-1 text-xs" style={{ color: "#fcd34d" }}>{executionState.latest_job.friendly_error}</div>}
                  </div>
                  <div className="rounded-2xl border p-3" style={{ borderColor: "var(--yunque-border)", background: "rgba(255,255,255,0.03)" }}>
                    <div className="text-xs" style={{ color: "var(--yunque-text-muted)" }}>失败摘要</div>
                    <div className="mt-1 text-sm">
                      失败 {executionState.failure_summary?.failed_count ?? 0} · 完成 {executionState.failure_summary?.completed_count ?? done}
                    </div>
                    {executionState.failure_summary?.next_step && <div className="mt-1 text-xs" style={{ color: "var(--yunque-text-muted)" }}>{executionState.failure_summary.next_step}</div>}
                  </div>
                </div>
                {executionState.failure_summary?.ruled_out?.length ? (
                  <div className="mt-3 rounded-xl px-3 py-2 text-xs" style={{ color: "var(--yunque-text-muted)", background: "rgba(239,68,68,0.08)" }}>
                    已暂时排除：{executionState.failure_summary.ruled_out.slice(0, 3).join("；")}
                  </div>
                ) : null}
                {executionState.cogni ? (
                  <div className="mt-3 rounded-xl px-3 py-2 text-xs" style={{ color: "var(--yunque-text-muted)", background: "rgba(14,165,233,0.08)" }}>
                    <div className="font-medium" style={{ color: "#7dd3fc" }}>Cogni 已参与本轮规划</div>
                    <div className="mt-1">
                      {executionState.cogni.activated?.length ? `激活：${executionState.cogni.activated.join("、")}` : "激活信息已记录"}
                      {typeof executionState.cogni.tool_before === "number" && typeof executionState.cogni.tool_after === "number"
                        ? ` · 工具面 ${executionState.cogni.tool_before} → ${executionState.cogni.tool_after}`
                        : ""}
                      {executionState.cogni.context_bytes ? ` · 上下文 ${executionState.cogni.context_bytes} 字节` : ""}
                    </div>
                    {executionState.cogni.last_summary && <div className="mt-1">{executionState.cogni.last_summary}</div>}
                  </div>
                ) : null}
                {executionState.events?.length ? (
                  <div className="mt-3 text-xs" style={{ color: "var(--yunque-text-muted)" }}>已汇总 {executionState.events.length} 条续跑事件，可在下方过程区域查看最新记录。</div>
                ) : null}
              </Card>
            )}

            {steps.length > 0 && (
              <Card id="dependency-view" className="section-card p-5">
                <div className="mb-4 flex items-center gap-2">
                  <GitBranch size={16} style={{ color: "var(--yunque-accent)" }} />
                  <div className="text-sm font-medium">依赖视图</div>
                  <span className="text-xs" style={{ color: "var(--yunque-text-muted)" }}>按 depends_on 串起原始规划现场</span>
                </div>
                <div className="mb-3 flex flex-wrap gap-2 text-xs">
                  <Chip size="sm" style={{ background: "rgba(14,165,233,0.1)", color: "#7dd3fc" }}>可执行 {graphCounts.ready || 0}</Chip>
                  <Chip size="sm" style={{ background: "rgba(251,191,36,0.1)", color: "#fcd34d" }}>被阻塞 {graphCounts.blocked || 0}</Chip>
                  <Chip size="sm" style={{ background: "rgba(34,197,94,0.1)", color: "#86efac" }}>已完成 {graphCounts.done || 0}</Chip>
                  <Chip size="sm" style={{ background: "rgba(239,68,68,0.1)", color: "#fca5a5" }}>需处理 {graphCounts.failed || 0}</Chip>
                </div>
                <div className="flex flex-wrap items-stretch gap-2">
                  {graphNodes.map(({ step, outgoing, graph, deps, blockedDeps, completedDeps }) => (
                    <div key={`graph-${step.id}`} className="min-w-44 flex-1 rounded-2xl border px-3 py-3" style={{ borderColor: graph.color, background: graphStateBackground(graph.state) }}>
                      <div className="mb-2 flex items-center justify-between gap-2">
                        <span className="font-mono text-xs" style={{ color: graph.color }}>#{step.id}</span>
                        <span className="rounded-full px-2 py-0.5 text-[11px]" style={{ color: graph.color, background: "rgba(255,255,255,0.05)" }}>{graph.label}</span>
                      </div>
                      <div className="line-clamp-2 text-sm">{step.action || step.skill || "未命名步骤"}</div>
                      <div className="mt-2 text-xs" style={{ color: "var(--yunque-text-muted)" }}>
                        入：{deps.length ? deps.map((dep) => `#${dep}`).join("、") : "无"} · 出：{outgoing.length ? outgoing.map((dep) => `#${dep}`).join("、") : "无"}
                      </div>
                      {graph.hint && (
                        <div className="mt-2 rounded-xl px-2 py-1.5 text-xs" style={{ color: graph.color, background: "rgba(255,255,255,0.04)" }}>
                          {graph.hint}
                        </div>
                      )}
                      {graph.state === "blocked" && blockedDeps.length > 0 && (
                        <div className="mt-2 text-xs" style={{ color: "#fcd34d" }}>
                          阻塞依赖：{blockedDeps.map((dep) => `#${dep}`).join("、")}
                        </div>
                      )}
                      {graph.state === "ready" && completedDeps.length > 0 && (
                        <div className="mt-2 text-xs" style={{ color: "#7dd3fc" }}>
                          前置已完成：{completedDeps.map((dep) => `#${dep}`).join("、")}
                        </div>
                      )}
                    </div>
                  ))}
                </div>
              </Card>
            )}

            <Card className="section-card p-5">
              <div className="mb-4 flex items-center gap-2">
                <GitBranch size={16} style={{ color: "var(--yunque-accent)" }} />
                <div className="text-sm font-medium">DAG 步骤</div>
                {resumePlanResultSteps.length > 0 && <span className="text-xs" style={{ color: "var(--yunque-text-muted)" }}>展示最近续跑结果</span>}
              </div>
              {steps.length === 0 ? (
                <div className="text-sm" style={{ color: "var(--yunque-text-muted)" }}>暂无步骤快照。</div>
              ) : (
                <div className="flex flex-col gap-2">
                  {steps.map((step) => (
                    <div key={step.id} className="rounded-2xl border px-3 py-3" style={{ borderColor: "var(--yunque-border)", background: "rgba(255,255,255,0.03)" }}>
                      <div className="flex flex-wrap items-center gap-2">
                        <span className="rounded-full px-2 py-0.5 text-xs" style={{ background: "rgba(255,255,255,0.06)", color: "#cbd5e1" }}>#{step.id}</span>
                        <span className="rounded-full px-2 py-0.5 text-xs" style={{ background: "rgba(255,255,255,0.06)", color: stepStatusColor(step.status) }}>{stepStatusLabel(step.status)}</span>
                        {step.skill && <span className="rounded-full px-2 py-0.5 text-xs" style={{ background: "rgba(14,165,233,0.1)", color: "#7dd3fc" }}>{step.skill}</span>}
                        {step.depends_on?.length ? <span className="text-xs" style={{ color: "var(--yunque-text-muted)" }}>依赖：{step.depends_on.join(", ")}</span> : null}
                      </div>
                      <div className="mt-2 text-sm">{step.action || step.skill || "未命名步骤"}</div>
                      {step.result && (
                        <div className="mt-2 rounded-xl p-3 text-xs" style={{ background: "rgba(0,0,0,0.18)", color: "var(--yunque-text-muted)" }}>
                          <div className="mb-1 font-medium" style={{ color: "#86efac" }}>已保留证据</div>
                          <pre className="max-h-40 overflow-auto whitespace-pre-wrap">{displayRecoveryText(step.result)}</pre>
                        </div>
                      )}
                      {step.error && <div className="mt-2 rounded-xl px-3 py-2 text-xs" style={{ background: "rgba(239,68,68,0.08)", color: "#fca5a5" }}>{formatErrorMessage(step.error, "这一步没有顺利完成，已保留现场。")}</div>}
                    </div>
                  ))}
                </div>
              )}
            </Card>
          </>
        ) : null}
      </div>
    </main>
  );
}


