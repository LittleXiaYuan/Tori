"use client";

import { useEffect, useMemo, useState } from "react";
import { Sparkles, Check, X, ChevronDown, ListChecks, Code2 } from "lucide-react";
import type { AgentEvent } from "@/components/execution-trace";
import { useI18n } from "@/lib/i18n";

type StepStatus = "done" | "running" | "pending" | "failed";

interface PlanStep {
  key: string;
  title: string;
  status: StepStatus;
  detail?: string;
}

interface SubAction {
  key: string;
  text: string;
  failed: boolean;
}

function isRecord(v: unknown): v is Record<string, unknown> {
  return typeof v === "object" && v !== null;
}
function asStr(v: unknown): string | undefined {
  return typeof v === "string" ? v : undefined;
}
function clean(s?: string): string {
  if (!s) return "";
  return s.replace(/\s+/g, " ").trim();
}
function mapStatus(raw?: string): StepStatus {
  const s = (raw || "").toLowerCase();
  if (["done", "completed", "complete", "success", "ok", "succeeded", "finished"].includes(s)) return "done";
  if (["failed", "error", "fail", "blocked"].includes(s)) return "failed";
  if (["running", "in_progress", "active", "executing", "started", "pending_result"].includes(s)) return "running";
  return "pending";
}

function structuredSteps(events: AgentEvent[]): PlanStep[] | null {
  for (let i = events.length - 1; i >= 0; i--) {
    const d = events[i].detail;
    if (isRecord(d) && Array.isArray(d.steps)) {
      const raw = d.steps.filter(isRecord);
      if (raw.length === 0) continue;
      return raw.map((st, idx) => ({
        key: String(asStr(st.id) ?? idx),
        title: clean(asStr(st.action) || asStr(st.skill) || `步骤 ${idx + 1}`),
        status: mapStatus(asStr(st.status)),
        detail: clean(asStr(st.result) || asStr(st.error)),
      }));
    }
  }
  return null;
}

function toolSteps(events: AgentEvent[], isLive: boolean): PlanStep[] {
  const steps: PlanStep[] = [];
  let runningIdx = -1;
  for (const e of events) {
    if (e.type === "tool_start") {
      steps.push({ key: e.id, title: clean(e.summary || e.meta?.skill || "执行步骤"), status: "running", detail: "" });
      runningIdx = steps.length - 1;
    } else if (e.type === "tool_result" && runningIdx >= 0) {
      const failed = /error|fail|失败|错误|无法/i.test(e.summary || "");
      steps[runningIdx].status = failed ? "failed" : "done";
      steps[runningIdx].detail = clean(e.summary);
      runningIdx = -1;
    }
  }
  if (!isLive) steps.forEach((s) => { if (s.status === "running") s.status = "done"; });
  return steps;
}

function fmtElapsed(ms: number): string {
  const sec = Math.max(0, Math.floor(ms / 1000));
  const mm = String(Math.floor(sec / 60)).padStart(2, "0");
  const ss = String(sec % 60).padStart(2, "0");
  return `${mm}:${ss}`;
}

function StatusDot({ status }: { status: StepStatus }) {
  return (
    <span className="task-plan-step__dot">
      {status === "done" ? (
        <Check size={11} strokeWidth={3} />
      ) : status === "failed" ? (
        <X size={11} strokeWidth={3} />
      ) : status === "running" ? (
        <span className="task-plan-spinner" />
      ) : null}
    </span>
  );
}

export function TaskPlanCard({ events, isLive }: { events: AgentEvent[]; isLive: boolean }) {
  const { t } = useI18n();
  const steps = useMemo(
    () => structuredSteps(events) ?? toolSteps(events, isLive),
    [events, isLive],
  );

  // Sub-actions (the live tool calls) shown in the right detail pane.
  const subActions = useMemo<SubAction[]>(
    () =>
      events
        .filter((e) => e.type === "tool_start" || e.type === "tool_result")
        .map((e, i) => ({
          key: `${e.id}-${i}`,
          text: clean(e.summary || e.meta?.skill || ""),
          failed: /error|fail|失败|错误|无法/i.test(e.summary || ""),
        }))
        .filter((a) => a.text),
    [events],
  );

  const [open, setOpen] = useState(isLive);
  // Auto-expand while live, auto-collapse to a single line once finished.
  useEffect(() => { setOpen(isLive); }, [isLive]);

  const [selectedKey, setSelectedKey] = useState<string | null>(null);
  const [now, setNow] = useState(() => Date.now());
  useEffect(() => {
    if (!isLive) return;
    const t = setInterval(() => setNow(Date.now()), 1000);
    return () => clearInterval(t);
  }, [isLive]);

  if (steps.length === 0) return null;

  const firstTs = events[0]?.ts;
  const lastTs = events[events.length - 1]?.ts;
  const startMs = firstTs ? Date.parse(firstTs) : now;
  const endMs = isLive ? now : lastTs ? Date.parse(lastTs) : now;
  const elapsed = fmtElapsed(endMs - startMs);

  const doneCount = steps.filter((s) => s.status === "done").length;
  const hasFailed = steps.some((s) => s.status === "failed");
  const runningIdx = steps.findIndex((s) => s.status === "running");
  const defaultIdx = runningIdx >= 0 ? runningIdx : steps.length - 1;
  const selIdx = selectedKey ? steps.findIndex((s) => s.key === selectedKey) : -1;
  const activeIdx = selIdx >= 0 ? selIdx : defaultIdx;
  const active = steps[activeIdx];
  const showActions = activeIdx === steps.length - 1 && subActions.length > 0;

  const headLabel = isLive
    ? runningIdx >= 0
      ? "云雀执行中"
      : "云雀规划中"
    : hasFailed
      ? "任务已记录"
      : "任务已完成";

  // ── Collapsed: a single status line (Qwen "任务已完成" pill). ──
  if (!open) {
    return (
      <div className="task-plan-card">
        <button type="button" className="task-plan-card__head" onClick={() => setOpen(true)} aria-expanded={false}>
          <span className="task-plan-card__title">
            {isLive ? <Sparkles size={14} className="chat-sparkle-anim" /> : <ListChecks size={14} />}
            {headLabel}
          </span>
          <span className="task-plan-card__meta">
            <span className="task-plan-card__count">{doneCount}/{steps.length}</span>
            <span className="task-plan-card__time">已用时 {elapsed}</span>
            <ChevronDown size={14} className="task-plan-card__chev" />
          </span>
        </button>
      </div>
    );
  }

  // ── Expanded: master (phases) / detail (selected phase contents). ──
  return (
    <div className="task-plan-card">
      <div className="task-plan-card__head task-plan-card__head--static">
        <span className="task-plan-card__title">
          {isLive ? <Sparkles size={14} className="chat-sparkle-anim" /> : <ListChecks size={14} />}
          {headLabel}
        </span>
        <span className="task-plan-card__meta">
          <span className="task-plan-card__count">{doneCount}/{steps.length}</span>
          <span className="task-plan-card__time">已用时 {elapsed}</span>
          <button type="button" className="task-plan-card__collapse" onClick={() => setOpen(false)} aria-label={t("common.collapse")}>
            <ChevronDown size={14} className="task-plan-card__chev" style={{ transform: "rotate(180deg)" }} />
          </button>
        </span>
      </div>

      <div className="task-plan-card__split">
        <div className="task-plan-card__phases">
          {steps.map((s, i) => (
            <button
              key={s.key}
              type="button"
              className="task-plan-step"
              data-status={s.status}
              data-active={i === activeIdx ? "true" : "false"}
              onClick={() => setSelectedKey(s.key)}
            >
              <StatusDot status={s.status} />
              <span className="task-plan-step__title">{s.title}</span>
            </button>
          ))}
        </div>

        <div className="task-plan-card__detail">
          {active && (
            <>
              <div className="task-plan-detail__title">{active.title}</div>
              {active.detail && <div className="task-plan-detail__desc">{active.detail}</div>}
              {showActions && (
                <div className="task-plan-detail__actions">
                  {subActions.map((a) => (
                    <div key={a.key} className="task-plan-action" data-failed={a.failed ? "true" : "false"}>
                      {a.failed ? <X size={12} /> : <Code2 size={12} />}
                      <span className="task-plan-action__text">{a.text}</span>
                    </div>
                  ))}
                </div>
              )}
              {!active.detail && !showActions && (
                <div className="task-plan-detail__empty">此阶段没有更多细节。</div>
              )}
            </>
          )}
        </div>
      </div>
    </div>
  );
}
