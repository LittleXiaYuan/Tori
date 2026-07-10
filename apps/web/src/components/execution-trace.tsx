"use client";

import { useState } from "react";
import type { ReactNode } from "react";
import Link from "next/link";
import {
  Brain, Wrench, CheckCircle2, XCircle, RefreshCw,
  MessageCircle, Settings2, ClipboardList, Lock, LockOpen,
  Ban, MapPin, ChevronDown, ChevronRight, Plus, Minus,
} from "lucide-react";
import { formatErrorMessage } from "@/lib/error-utils";
import { resolvePlannerRecoveryTargetFromDetail } from "@/lib/planner-recovery-target";

/** Mirrors observe.AgentEvent from the Go backend. */
export interface AgentEvent {
  id: string;
  trace_id: string;
  span_id?: string;
  ts: string;
  domain: string;   // "planner" | "workflow" | "approval" | "agent"
  type: string;     // "thinking" | "tool_start" | "tool_result" | "reflect" | "node_start" | etc.
  summary: string;
  detail?: unknown;
  meta: {
    tenant_id?: string;
    session_id?: string;
    task_id?: string;
    skill?: string;
    node_id?: string;
    node_name?: string;
    instance_id?: string;
  };
}

function domainIcon(domain: string, type: string) {
  if (domain === "planner") {
    switch (type) {
      case "thinking": return <Brain size={14} />;
      case "tool_start": return <Wrench size={14} />;
      case "tool_result": return type.includes("fail") ? <XCircle size={14} /> : <CheckCircle2 size={14} />;
      case "reflect": return <RefreshCw size={14} />;
      default: return <MessageCircle size={14} />;
    }
  }
  if (domain === "workflow") {
    switch (type) {
      case "node_start": return <Settings2 size={14} />;
      case "node_done": return <CheckCircle2 size={14} />;
      case "node_failed": return <XCircle size={14} />;
      default: return <ClipboardList size={14} />;
    }
  }
  if (domain === "approval") {
    switch (type) {
      case "request": return <Lock size={14} />;
      case "approved": return <LockOpen size={14} />;
      case "denied": return <Ban size={14} />;
      default: return <ClipboardList size={14} />;
    }
  }
  return <MapPin size={14} />;
}

function domainColor(domain: string, type: string): string {
  if (domain === "planner") {
    switch (type) {
      case "thinking": return "var(--yunque-accent)";
      case "tool_start": return "#fbbf24";
      case "tool_result":
        return type.includes("fail") ? "#f87171" : "#34d399";
      case "reflect": return "#a78bfa";
      default: return "var(--yunque-accent)";
    }
  }
  if (domain === "workflow") {
    if (type === "node_failed") return "#f87171";
    if (type === "node_done") return "#34d399";
    return "#22d3ee";
  }
  if (domain === "approval") {
    if (type === "denied") return "#f87171";
    if (type === "approved") return "#34d399";
    return "#fb923c";
  }
  return "#6b7280";
}

function formatDuration(startTs: string, evtTs: string): string {
  const diff = new Date(evtTs).getTime() - new Date(startTs).getTime();
  if (diff < 1000) return `${diff}ms`;
  return `${(diff / 1000).toFixed(1)}s`;
}

interface ExecutionTraceProps {
  events: AgentEvent[];
  isLive?: boolean;
  onRecoveryPrompt?: (prompt: string) => void;
  // When true, the backend already returned raw (unsanitized) events — skip
  // the client-side friendly-text rewrite so the real underlying error (e.g.
  // literal HTTP status/body) is visible instead of the generic "现场已保留"
  // wording. Only ever set from Trace page's explicit raw-mode toggle.
  raw?: boolean;
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

function asNumber(value: unknown): number | undefined {
  return typeof value === "number" && Number.isFinite(value) ? value : undefined;
}

function asString(value: unknown): string | undefined {
  return typeof value === "string" && value.trim() ? value : undefined;
}

function asStringArray(value: unknown): string[] {
  return Array.isArray(value) ? value.filter((item): item is string => typeof item === "string" && item.trim().length > 0) : [];
}

function compactList(items: string[], max = 4): string {
  if (items.length === 0) return "无";
  const head = items.slice(0, max).join("、");
  return items.length > max ? `${head} 等 ${items.length} 项` : head;
}

function basenameFromAttachmentPath(path: string): string {
  const normalized = path.trim().replace(/\\/g, "/");
  return normalized.split("/").filter(Boolean).pop() || normalized || "附件";
}

function maskParsedAttachmentMarker(line: string): string {
  return line
    .replace(/\[Parsed document:\s*([^\]]+)\]/gi, (_match, file: string) => `附件内容：${file.trim()}`)
    .replace(/^Workspace path:\s*(.+)$/i, (_match, file: string) => `附件名称：${basenameFromAttachmentPath(file)}`)
    .replace(/^Parser:\s*(.+)$/i, (_match, parser: string) => `附件解析器：${parser.trim()}`)
    .replace(/^Status:\s*(.+)$/i, (_match, status: string) => `附件状态：${status.trim()}`)
    .replace(/^Note:\s*(.+)$/i, (_match, note: string) => `附件说明：${note.trim()}`);
}

function displayTraceText(text?: string, fallback = ""): string {
  const raw = (text || "").trim();
  if (!raw) return fallback;
  return raw
    .split(/\r?\n/)
    .map((line) => formatErrorMessage(line, line))
    .map(maskParsedAttachmentMarker)
    .join("\n")
    .trim();
}

function detailKind(detail: unknown): "cogni" | "checkpoint" | "model_fallback" | "handoff" | "failure_recovery" | "partial_result" | null {
  if (!isRecord(detail)) return null;
  if ("recoverable" in detail && "steps" in detail && ("completed_count" in detail || "failed_count" in detail)) return "partial_result";
  if ("plan_id" in detail && "plan_snapshot" in detail) return "checkpoint";
  if ("model" in detail && "attempt" in detail && ("reason" in detail || !("summary" in detail))) return "model_fallback";
  if ("agent" in detail && ("input" in detail || "reply" in detail || "error" in detail || "next_step" in detail || "recoverable" in detail)) return "handoff";
  if ("failed_count" in detail && ("ruled_out" in detail || "failed_tools" in detail || "next_step" in detail)) return "failure_recovery";
  if ("context_bytes" in detail || "tool_before" in detail || "tool_after" in detail || "activated" in detail) return "cogni";
  return null;
}

function DetailPill({ label, value, tone = "default" }: { label: string; value: ReactNode; tone?: "default" | "good" | "warn" | "bad" }) {
  const toneColor = tone === "good" ? "#34d399" : tone === "warn" ? "#fbbf24" : tone === "bad" ? "#f87171" : "var(--yunque-text)";
  return (
    <div className="rounded-lg px-2.5 py-1.5" style={{ background: "rgba(255,255,255,0.045)", border: "1px solid var(--yunque-border)" }}>
      <div className="text-[10px] mb-0.5" style={{ color: "var(--yunque-text-muted)" }}>{label}</div>
      <div className="text-[12px] font-medium break-words" style={{ color: toneColor }}>{value}</div>
    </div>
  );
}

function displaySummary(summary: string): string {
  if (!/当前模型响应失败|备用模型|调用栈降级|级联唤醒|context canceled|context cancelled|context deadline exceeded|execution failed|handoff agent|all fallback llm clients failed|planner fc|unknown skill|tool panic|blocked by trust gate|EOF|Parsed document|Workspace path/i.test(summary)) {
    return summary;
  }
  return displayTraceText(summary, summary);
}

function CogniDetailCard({ detail }: { detail: Record<string, unknown> }) {
  const activated = asStringArray(detail.activated);
  const before = asNumber(detail.tool_before);
  const after = asNumber(detail.tool_after);
  const removed = asStringArray(detail.removed);
  const contextBytes = asNumber(detail.context_bytes) ?? 0;
  const fallback = detail.fell_back_to_input === true;
  return (
    <div className="mt-1.5 rounded-lg p-2.5 space-y-2" style={{ background: "rgba(96,165,250,0.08)", border: "1px solid rgba(96,165,250,0.18)" }}>
      <div className="grid grid-cols-2 gap-2">
        <DetailPill label="激活 Cogni" value={compactList(activated)} tone={activated.length > 0 ? "good" : "default"} />
        <DetailPill label="上下文" value={`${contextBytes} 字节`} />
        <DetailPill label="工具面" value={before !== undefined || after !== undefined ? `${before ?? "?"} → ${after ?? "?"}` : "未调整"} tone={fallback ? "warn" : "default"} />
        <DetailPill label="移除工具" value={compactList(removed)} />
      </div>
      {fallback && <div className="text-[11px]" style={{ color: "#fbbf24" }}>工具过滤为空，已回退到原始工具面，避免任务被锁死。</div>}
    </div>
  );
}


function stepLabel(step: Record<string, unknown>, idx: number): string {
  const status = asString(step.status) ?? "pending";
  const action = displayTraceText(asString(step.action) ?? asString(step.skill) ?? `步骤 ${idx + 1}`);
  const skill = asString(step.skill);
  const error = asString(step.error);
  return `- ${status} · ${action}${skill ? ` · skill=${skill}` : ""}${error ? ` · error=${displayTraceText(error, "这一步没有顺利完成，已保留现场。")}` : ""}`;
}

function buildCheckpointRecoveryPrompt(detail: Record<string, unknown>, mode: "continue" | "retry_failed" | "partial"): string {
  const snapshot = Array.isArray(detail.plan_snapshot) ? detail.plan_snapshot.filter(isRecord) : [];
  const failed = snapshot.find((step) => asString(step.status) === "failed");
  const lines = snapshot.slice(0, 12).map(stepLabel).join("\n") || "无可见步骤快照";
  const base = [
    `Plan ID：${asString(detail.plan_id) ?? "未知"}`,
    `Task ID：${asString(detail.task_id) ?? "未知"}`,
    `进度：${asNumber(detail.completed) ?? 0}/${asNumber(detail.total) ?? 0}`,
    asString(detail.error) ? `失败原因：${displayTraceText(asString(detail.error), "任务暂时没有顺利完成，已保留现场。")}` : "失败原因：未记录",
    "步骤快照：",
    lines,
  ].join("\n");
  if (mode === "retry_failed") {
    const target = failed ? stepLabel(failed, snapshot.indexOf(failed)) : "未找到失败步骤，请自行定位最小失败步骤";
    return `请基于下面的长程规划失败现场，重试失败步骤。不要重复已经完成的步骤，先缩小输入和工具面；如果同一路径再次失败，请换策略并返回阶段结果。\n\n${base}\n\n本次优先重试：\n${target}`;
  }
  if (mode === "partial") {
    return `请基于下面的长程规划失败现场，先整理并返回已经完成的部分。明确说明哪些步骤已完成、哪些失败、下一步最小可执行动作是什么。\n\n${base}`;
  }
  return `请基于下面的长程规划 checkpoint 继续执行。不要从头重跑，优先复用已完成步骤，只处理 pending/failed 部分；必要时调整工具或降低粒度。\n\n${base}`;
}

function CheckpointDetailCard({ detail, onRecoveryPrompt }: { detail: Record<string, unknown>; onRecoveryPrompt?: (prompt: string) => void }) {
  const completed = asNumber(detail.completed) ?? 0;
  const total = asNumber(detail.total) ?? 0;
  const status = asString(detail.status) ?? "unknown";
  const error = asString(detail.error);
  const hint = asString(detail.resume_hint);
  const recoverable = detail.recoverable === true || Boolean(error);
  const planId = asString(detail.plan_id);
  const snapshot = Array.isArray(detail.plan_snapshot) ? detail.plan_snapshot.filter(isRecord).slice(0, 4) : [];
  return (
    <div className="mt-1.5 rounded-lg p-2.5 space-y-2" style={{ background: "rgba(167,139,250,0.08)", border: "1px solid rgba(167,139,250,0.18)" }}>
      <div className="grid grid-cols-2 gap-2">
        <DetailPill label="进度" value={`${completed}/${total}`} tone={completed === total && total > 0 ? "good" : "default"} />
        <DetailPill label="状态" value={status} tone={error ? "bad" : "default"} />
        <DetailPill label="Plan ID" value={planId ?? "—"} />
        <DetailPill label="已用步骤" value={asNumber(detail.steps_used) ?? 0} />
      </div>
      {error && <div className="text-[11px]" style={{ color: "#f87171" }}>失败原因：{displayTraceText(error, "任务暂时没有顺利完成，已保留现场。")}</div>}
      {hint && <div className="text-[11px]" style={{ color: "var(--yunque-text-muted)" }}>{displayTraceText(hint)}</div>}
      {planId && (
        <Link
          href={`/planner-checkpoint?plan_id=${encodeURIComponent(planId)}`}
          className="inline-flex w-fit items-center gap-1 rounded-full px-2.5 py-1 text-[11px] font-medium"
          style={{ background: "rgba(167,139,250,0.14)", color: "#c4b5fd", border: "1px solid rgba(167,139,250,0.28)" }}
          onClick={(e) => e.stopPropagation()}
        >
          详情页
          <ChevronRight size={11} aria-hidden />
        </Link>
      )}
      {snapshot.length > 0 && (
        <div className="space-y-1.5">
          {snapshot.map((step, idx) => (
            <div key={`${asNumber(step.id) ?? idx}-${asString(step.action) ?? idx}`} className="rounded-md px-2 py-1.5 text-[11px]" style={{ background: "rgba(0,0,0,0.16)", color: "var(--yunque-text-muted)" }}>
              <span className="font-medium" style={{ color: step.status === "failed" ? "#f87171" : "var(--yunque-text)" }}>
                {asString(step.status) ?? "pending"}
              </span>
              <span className="mx-1">·</span>
              {displayTraceText(asString(step.action) ?? asString(step.skill) ?? `步骤 ${idx + 1}`)}
            </div>
          ))}
        </div>
      )}
      {recoverable && onRecoveryPrompt && (
        <div className="flex flex-wrap gap-1.5 pt-1">
          <button type="button" className="rounded-full px-2.5 py-1 text-[11px] font-medium" style={{ background: "rgba(167,139,250,0.14)", color: "#c4b5fd", border: "1px solid rgba(167,139,250,0.28)" }} onClick={(e) => { e.stopPropagation(); onRecoveryPrompt(buildCheckpointRecoveryPrompt(detail, "continue")); }}>
            继续执行
          </button>
          <button type="button" className="rounded-full px-2.5 py-1 text-[11px] font-medium" style={{ background: "rgba(251,191,36,0.12)", color: "#fcd34d", border: "1px solid rgba(251,191,36,0.25)" }} onClick={(e) => { e.stopPropagation(); onRecoveryPrompt(buildCheckpointRecoveryPrompt(detail, "retry_failed")); }}>
            重试失败步骤
          </button>
          <button type="button" className="rounded-full px-2.5 py-1 text-[11px] font-medium" style={{ background: "rgba(255,255,255,0.06)", color: "var(--yunque-text-muted)", border: "1px solid var(--yunque-border)" }} onClick={(e) => { e.stopPropagation(); onRecoveryPrompt(buildCheckpointRecoveryPrompt(detail, "partial")); }}>
            返回已完成部分
          </button>
        </div>
      )}
    </div>
  );
}



function buildHandoffRecoveryPrompt(detail: Record<string, unknown>, mode: "direct" | "partial"): string {
  const agent = asString(detail.agent) ?? "子代理";
  const input = displayTraceText(asString(detail.input), "未记录");
  const error = asString(detail.error);
  const displayError = error ? displayTraceText(error, "任务暂时没有顺利完成，已保留现场。") : "未记录";
  if (mode === "partial") {
    return `子代理 ${agent} 没有完成。请不要重复同一路径，先根据已知上下文返回阶段性结论，并列出下一步最小可执行动作。\n\n委派内容：${input}\n失败原因：${displayError}`;
  }
  return `子代理 ${agent} 没有完成。请切换策略：不要再次委派同一个子代理，改用主规划器可用的直接工具或更小粒度步骤继续。\n\n委派内容：${input}\n失败原因：${displayError}`;
}

function HandoffDetailCard({ detail, onRecoveryPrompt }: { detail: Record<string, unknown>; onRecoveryPrompt?: (prompt: string) => void }) {
  const agent = asString(detail.agent) ?? "子代理";
  const input = asString(detail.input);
  const reply = asString(detail.reply);
  const error = asString(detail.error);
  const nextStep = asString(detail.next_step);
  const recoverable = detail.recoverable === true;
  const durMs = asNumber(detail.dur_ms);
  return (
    <div className="mt-1.5 rounded-lg p-2.5 space-y-2" style={{ background: "rgba(34,211,238,0.08)", border: "1px solid rgba(34,211,238,0.18)" }}>
      <div className="grid grid-cols-2 gap-2">
        <DetailPill label="子代理" value={agent} tone={error ? "warn" : "good"} />
        <DetailPill label="状态" value={error ? (recoverable ? "可恢复" : "未完成") : "已完成"} tone={error ? "warn" : "good"} />
        {durMs !== undefined && <DetailPill label="耗时" value={`${durMs} ms`} />}
        {recoverable && <DetailPill label="后续策略" value="切换策略" tone="warn" />}
      </div>
      {input && <div className="text-[11px]" style={{ color: "var(--yunque-text-muted)" }}>委派内容：{displayTraceText(input)}</div>}
      {reply && <div className="text-[11px]" style={{ color: "var(--yunque-text-muted)" }}>返回摘要：{displayTraceText(reply)}</div>}
      {error && <div className="text-[11px]" style={{ color: "#fbbf24" }}>原因已保留：{displayTraceText(error, "任务暂时没有顺利完成，已保留现场。")}</div>}
      {nextStep && <div className="text-[11px]" style={{ color: "var(--yunque-text-muted)" }}>{displayTraceText(nextStep)}</div>}
      {recoverable && onRecoveryPrompt && (
        <div className="flex flex-wrap gap-1.5 pt-1">
          <button type="button" className="rounded-full px-2.5 py-1 text-[11px] font-medium" style={{ background: "rgba(34,211,238,0.12)", color: "#67e8f9", border: "1px solid rgba(34,211,238,0.25)" }} onClick={(e) => { e.stopPropagation(); onRecoveryPrompt(buildHandoffRecoveryPrompt(detail, "direct")); }}>
            改用直接工具
          </button>
          <button type="button" className="rounded-full px-2.5 py-1 text-[11px] font-medium" style={{ background: "rgba(255,255,255,0.06)", color: "var(--yunque-text-muted)", border: "1px solid var(--yunque-border)" }} onClick={(e) => { e.stopPropagation(); onRecoveryPrompt(buildHandoffRecoveryPrompt(detail, "partial")); }}>
            先返回阶段结果
          </button>
        </div>
      )}
    </div>
  );
}

function ModelFallbackDetailCard({ detail }: { detail: Record<string, unknown> }) {
  return (
    <div className="mt-1.5 rounded-lg p-2.5 grid grid-cols-2 gap-2" style={{ background: "rgba(251,191,36,0.08)", border: "1px solid rgba(251,191,36,0.2)" }}>
      <DetailPill label="切换到" value={asString(detail.model) ?? "可用模型"} tone="warn" />
      <DetailPill label="尝试轮次" value={asNumber(detail.attempt) ?? "—"} />
      {asString(detail.reason) && <div className="col-span-2 text-[11px]" style={{ color: "var(--yunque-text-muted)" }}>原因：{displayTraceText(asString(detail.reason), "模型暂时没有回应，正在切换到可用模型。")}</div>}
    </div>
  );
}

function buildFailureRecoveryPrompt(detail: Record<string, unknown>, mode: "continue" | "partial"): string {
  const tried = asStringArray(detail.tried).map((item) => displayTraceText(item));
  const ruledOut = asStringArray(detail.ruled_out).map((item) => displayTraceText(item, item));
  const failedTools = asStringArray(detail.failed_tools);
  const failurePattern = displayTraceText(asString(detail.failure_pattern));
  const recommendation = displayTraceText(asString(detail.recommendation));
  const nextStep = displayTraceText(asString(detail.next_step), "换一个工具、降低任务粒度，或先返回阶段结果。");
  const lines = [
    `已完成：${asNumber(detail.completed_count) ?? tried.length}`,
    `未完成：${asNumber(detail.failed_count) ?? ruledOut.length}`,
    failurePattern ? `失败模式：${failurePattern}` : "",
    recommendation ? `推荐策略：${recommendation}` : "",
    failedTools.length ? `暂不重复：${failedTools.join("、")}` : "",
    tried.length ? `已获得结果：\n${tried.slice(0, 6).map((item) => `- ${item}`).join("\n")}` : "",
    ruledOut.length ? `已暂时排除：\n${ruledOut.slice(0, 6).map((item) => `- ${item}`).join("\n")}` : "",
    `下一步：${nextStep}`,
  ].filter(Boolean).join("\n");
  if (mode === "partial") {
    return `请不要继续重复失败路径。基于下面的执行现场，先返回阶段性结论，并列出下一步最小可执行动作。\n\n${lines}`;
  }
  return `请基于下面的执行现场继续，但不要重复已经失败的同一路径；优先换直接工具、降低任务粒度，或先汇总已获得证据。\n\n${lines}`;
}

function buildPartialResultPrompt(detail: Record<string, unknown>, mode: "continue" | "summary"): string {
  const steps = Array.isArray(detail.steps) ? detail.steps.filter(isRecord) : [];
  const lines = steps.slice(0, 10).map((step, idx) => {
    const status = asString(step.status) ?? "recorded";
    const label = displayTraceText(asString(step.skill) ?? asString(step.action) ?? `步骤 ${idx + 1}`);
    const result = displayTraceText(asString(step.result));
    const error = asString(step.error);
    return `- ${status} · ${label}${result ? ` · ${result}` : ""}${error ? ` · ${displayTraceText(error, "这一步没有顺利完成，已保留现场。")}` : ""}`;
  }).join("\n") || "暂无可见步骤。";
  const nextStep = displayTraceText(asString(detail.next_step), "基于已完成步骤继续，必要时降低粒度。");
  if (mode === "summary") {
    return `请基于下面的阶段结果，先整理已经完成的部分，并列出下一步最小可执行动作。\n\n${lines}\n\n下一步：${nextStep}`;
  }
  return `请基于下面的阶段结果继续执行。不要从头重跑，优先复用已完成步骤；如果同一路径再次失败，请换策略并返回阶段结果。\n\n${lines}\n\n下一步：${nextStep}`;
}

function PartialResultDetailCard({ detail, onRecoveryPrompt }: { detail: Record<string, unknown>; onRecoveryPrompt?: (prompt: string) => void }) {
  const completedCount = asNumber(detail.completed_count) ?? 0;
  const failedCount = asNumber(detail.failed_count) ?? 0;
  const reason = asString(detail.reason);
  const nextStep = asString(detail.next_step);
  const steps = Array.isArray(detail.steps) ? detail.steps.filter(isRecord).slice(0, 5) : [];
  return (
    <div className="mt-1.5 rounded-lg p-2.5 space-y-2" style={{ background: "rgba(34,197,94,0.08)", border: "1px solid rgba(34,197,94,0.18)" }}>
      <div className="grid grid-cols-2 gap-2">
        <DetailPill label="已完成" value={completedCount} tone={completedCount > 0 ? "good" : "default"} />
        <DetailPill label="待处理" value={failedCount} tone={failedCount > 0 ? "warn" : "default"} />
      </div>
      {reason && <div className="text-[11px]" style={{ color: "var(--yunque-text-muted)" }}>原因：{displayTraceText(reason, reason)}</div>}
      {steps.length > 0 && (
        <div className="space-y-1">
          {steps.map((step, idx) => {
            const status = asString(step.status) ?? "recorded";
            const label = displayTraceText(asString(step.skill) ?? asString(step.action) ?? `步骤 ${idx + 1}`);
            const result = displayTraceText(asString(step.result));
            const error = asString(step.error);
            return (
              <div key={`${label}-${idx}`} className="rounded-md px-2 py-1 text-[11px]" style={{ background: "rgba(255,255,255,0.045)", color: "var(--yunque-text-muted)" }}>
                <span style={{ color: status === "failed" ? "#fbbf24" : "#86efac" }}>{status}</span>
                <span> · {label}</span>
                {result && <span>：{result}</span>}
                {error && <span>：{displayTraceText(error, "这一步没有顺利完成，已保留现场。")}</span>}
              </div>
            );
          })}
        </div>
      )}
      {nextStep && <div className="text-[11px]" style={{ color: "var(--yunque-text-muted)" }}>{displayTraceText(nextStep)}</div>}
      {onRecoveryPrompt && (
        <div className="flex flex-wrap gap-1.5 pt-1">
          <button type="button" className="rounded-full px-2.5 py-1 text-[11px] font-medium" style={{ background: "rgba(34,197,94,0.12)", color: "#86efac", border: "1px solid rgba(34,197,94,0.25)" }} onClick={(e) => { e.stopPropagation(); onRecoveryPrompt(buildPartialResultPrompt(detail, "continue")); }}>
            基于阶段结果继续
          </button>
          <button type="button" className="rounded-full px-2.5 py-1 text-[11px] font-medium" style={{ background: "rgba(255,255,255,0.06)", color: "var(--yunque-text-muted)", border: "1px solid var(--yunque-border)" }} onClick={(e) => { e.stopPropagation(); onRecoveryPrompt(buildPartialResultPrompt(detail, "summary")); }}>
            整理已完成部分
          </button>
        </div>
      )}
    </div>
  );
}

function FailureRecoveryDetailCard({ detail, onRecoveryPrompt }: { detail: Record<string, unknown>; onRecoveryPrompt?: (prompt: string) => void }) {
  const failedCount = asNumber(detail.failed_count) ?? 0;
  const completedCount = asNumber(detail.completed_count) ?? asStringArray(detail.tried).length;
  const failedTools = asStringArray(detail.failed_tools);
  const ruledOut = asStringArray(detail.ruled_out).map((item) => displayTraceText(item, item));
  const tried = asStringArray(detail.tried).map((item) => displayTraceText(item));
  const failurePattern = displayTraceText(asString(detail.failure_pattern));
  const recommendation = displayTraceText(asString(detail.recommendation));
  const nextStep = displayTraceText(asString(detail.next_step));
  const recoveryTarget = resolvePlannerRecoveryTargetFromDetail(detail);
  return (
    <div className="mt-1.5 rounded-lg p-2.5 space-y-2" style={{ background: "rgba(251,191,36,0.08)", border: "1px solid rgba(251,191,36,0.2)" }}>
      <div className="grid grid-cols-2 gap-2">
        <DetailPill label="已完成" value={completedCount} tone={completedCount > 0 ? "good" : "default"} />
        <DetailPill label="未完成" value={failedCount} tone={failedCount > 0 ? "warn" : "default"} />
        <DetailPill label="暂不重复" value={compactList(failedTools)} tone={failedTools.length > 0 ? "warn" : "default"} />
        <DetailPill label="后续策略" value="切换策略" tone="warn" />
      </div>
      {failurePattern && <div className="text-[11px]" style={{ color: "#fcd34d" }}>失败模式：{failurePattern}</div>}
      {recommendation && <div className="text-[11px]" style={{ color: "var(--yunque-text-muted)" }}>推荐策略：{recommendation}</div>}
      {tried.length > 0 && <div className="text-[11px]" style={{ color: "var(--yunque-text-muted)" }}>已获得结果：{tried.slice(0, 3).join("；")}</div>}
      {ruledOut.length > 0 && <div className="text-[11px]" style={{ color: "#fbbf24" }}>已暂时排除：{ruledOut.slice(0, 3).join("；")}</div>}
      {nextStep && <div className="text-[11px]" style={{ color: "var(--yunque-text-muted)" }}>{nextStep}</div>}
      {recoveryTarget && (
        <Link
          href={recoveryTarget.href}
          className="inline-flex w-fit items-center gap-1 rounded-full px-2.5 py-1 text-[11px] font-medium"
          style={{ background: "rgba(251,191,36,0.12)", color: "#fcd34d", border: "1px solid rgba(251,191,36,0.25)" }}
          onClick={(e) => e.stopPropagation()}
        >
          {recoveryTarget.label}
          <ChevronRight size={11} aria-hidden />
        </Link>
      )}
      {onRecoveryPrompt && (
        <div className="flex flex-wrap gap-1.5 pt-1">
          <button type="button" className="rounded-full px-2.5 py-1 text-[11px] font-medium" style={{ background: "rgba(251,191,36,0.12)", color: "#fcd34d", border: "1px solid rgba(251,191,36,0.25)" }} onClick={(e) => { e.stopPropagation(); onRecoveryPrompt(buildFailureRecoveryPrompt(detail, "continue")); }}>
            换策略继续
          </button>
          <button type="button" className="rounded-full px-2.5 py-1 text-[11px] font-medium" style={{ background: "rgba(255,255,255,0.06)", color: "var(--yunque-text-muted)", border: "1px solid var(--yunque-border)" }} onClick={(e) => { e.stopPropagation(); onRecoveryPrompt(buildFailureRecoveryPrompt(detail, "partial")); }}>
            先返回阶段结果
          </button>
        </div>
      )}
    </div>
  );
}

function TraceDetailView({ detail, onRecoveryPrompt, raw }: { detail: unknown; onRecoveryPrompt?: (prompt: string) => void; raw?: boolean }) {
  const forDisplay = raw ? detail : sanitizeTraceDetailForDisplay(detail);
  if (!isRecord(detail)) {
    return (
      <pre className="mt-1.5 p-2.5 rounded-lg font-mono text-[11px] whitespace-pre-wrap break-all max-h-[200px] overflow-y-auto" style={{ background: "rgba(0,0,0,0.2)", color: "var(--yunque-text-muted)", border: "1px solid var(--yunque-border)" }}>
        {JSON.stringify(forDisplay, null, 2)}
      </pre>
    );
  }
  // Structured cards (cogni/checkpoint/handoff/…) render friendly summaries
  // by construction — raw mode only affects the generic JSON fallback below,
  // where the actual error strings live.
  const kind = detailKind(detail);
  if (kind === "cogni") return <CogniDetailCard detail={detail} />;
  if (kind === "checkpoint") return <CheckpointDetailCard detail={detail} onRecoveryPrompt={onRecoveryPrompt} />;
  if (kind === "model_fallback") return <ModelFallbackDetailCard detail={detail} />;
  if (kind === "handoff") return <HandoffDetailCard detail={detail} onRecoveryPrompt={onRecoveryPrompt} />;
  if (kind === "partial_result") return <PartialResultDetailCard detail={detail} onRecoveryPrompt={onRecoveryPrompt} />;
  if (kind === "failure_recovery") return <FailureRecoveryDetailCard detail={detail} onRecoveryPrompt={onRecoveryPrompt} />;
  return (
    <pre
      className="mt-1.5 p-2.5 rounded-lg font-mono text-[11px] whitespace-pre-wrap break-all max-h-[200px] overflow-y-auto"
      style={{ background: "rgba(0,0,0,0.2)", color: "var(--yunque-text-muted)", border: "1px solid var(--yunque-border)" }}
    >
      {JSON.stringify(forDisplay, null, 2)}
    </pre>
  );
}

function sanitizeTraceDetailForDisplay(value: unknown): unknown {
  if (typeof value === "string") {
    return displayTraceText(value, value);
  }
  if (Array.isArray(value)) {
    return value.map((item) => sanitizeTraceDetailForDisplay(item));
  }
  if (isRecord(value)) {
    return Object.fromEntries(
      Object.entries(value).map(([key, item]) => [key, sanitizeTraceDetailForDisplay(item)]),
    );
  }
  return value;
}

export function ExecutionTrace({ events, isLive, onRecoveryPrompt, raw }: ExecutionTraceProps) {
  const [expanded, setExpanded] = useState(false);

  if (events.length === 0) return null;

  const firstTs = events[0]?.ts;
  const lastTs = events[events.length - 1]?.ts;
  const totalDuration = firstTs && lastTs ? formatDuration(firstTs, lastTs) : "—";
  const traceId = events[0]?.trace_id ?? "";
  const shortTrace = traceId.length > 12 ? traceId.slice(0, 12) + "…" : traceId;

  return (
    <div className="rounded-xl overflow-hidden text-[13px]" style={{ background: "var(--yunque-card)", border: "1px solid var(--yunque-border)" }}>
      <button
        className="flex items-center gap-2 w-full px-3.5 py-2.5 text-left transition-colors"
        onClick={() => setExpanded(!expanded)}
        aria-expanded={expanded}
        style={{ color: "var(--yunque-text)" }}
        onMouseEnter={(e) => e.currentTarget.style.background = "rgba(255,255,255,0.04)"}
        onMouseLeave={(e) => e.currentTarget.style.background = "transparent"}
      >
        <span className="text-[10px]" style={{ color: "var(--yunque-text-muted)" }}>
          {expanded ? <ChevronDown size={12} /> : <ChevronRight size={12} />}
        </span>
        <span className="font-medium text-[13px]">
          执行过程 ({events.length} 步, {totalDuration})
        </span>
        {isLive && (
          <span className="text-[10px] font-semibold animate-pulse" style={{ color: "#34d399" }}>
            ● LIVE
          </span>
        )}
        <span className="ml-auto font-mono text-[11px] opacity-60" style={{ color: "var(--yunque-text-muted)" }}>
          {shortTrace}
        </span>
      </button>

      {expanded && (
        <div className="px-3.5 pb-3.5 pt-1">
          {events.map((evt, idx) => (
            <TraceItem key={`${evt.id}-${idx}`} event={evt} startTs={firstTs} onRecoveryPrompt={onRecoveryPrompt} raw={raw} />
          ))}
        </div>
      )}
    </div>
  );
}

function TraceItem({ event, startTs, onRecoveryPrompt, raw }: { event: AgentEvent; startTs: string; onRecoveryPrompt?: (prompt: string) => void; raw?: boolean }) {
  const [detailOpen, setDetailOpen] = useState(false);
  const icon = domainIcon(event.domain, event.type);
  const color = domainColor(event.domain, event.type);
  const offset = formatDuration(startTs, event.ts);
  const hasDetail = event.detail !== null && event.detail !== undefined;
  const summary = raw ? (event.summary || "") : displaySummary(event.summary);

  return (
    <div className="relative pl-7 pb-1.5">
      {/* Vertical line */}
      <div className="absolute left-[7px] top-0 bottom-0 w-0.5" style={{ background: "var(--yunque-border)" }} />
      {/* Dot */}
      <div
        className="absolute left-[3px] top-1.5 w-2.5 h-2.5 rounded-full"
        style={{ background: color, boxShadow: `0 0 6px ${color}66` }}
      />
      <div>
        <div
          className="flex items-center gap-1.5 leading-relaxed"
          onClick={() => hasDetail && setDetailOpen(!detailOpen)}
          style={{ cursor: hasDetail ? "pointer" : "default" }}
        >
          <span style={{ color }}>{icon}</span>
          <span className="font-medium text-xs whitespace-nowrap" style={{ color }}>
            {event.domain}.{event.type}
          </span>
          {event.meta.skill && (
            <span
              className="text-[11px] px-1.5 py-0.5 rounded max-w-[150px] truncate"
              style={{ background: "rgba(255,255,255,0.06)", color: "var(--yunque-text-muted)" }}
            >
              {event.meta.skill}
            </span>
          )}
          {event.meta.node_name && (
            <span
              className="text-[11px] px-1.5 py-0.5 rounded max-w-[150px] truncate"
              style={{ background: "rgba(255,255,255,0.06)", color: "var(--yunque-text-muted)" }}
            >
              {event.meta.node_name}
            </span>
          )}
          <span className="ml-auto font-mono text-[11px] opacity-60 shrink-0" style={{ color: "var(--yunque-text-muted)" }}>
            +{offset}
          </span>
          {hasDetail && (
            <span className="text-xs shrink-0" style={{ color: "var(--yunque-text-muted)" }}>
              {detailOpen ? <Minus size={12} /> : <Plus size={12} />}
            </span>
          )}
        </div>
        <div
          className="text-xs mt-0.5 leading-relaxed"
          onClick={() => hasDetail && setDetailOpen(!detailOpen)}
          style={{ color: "var(--yunque-text-muted)", cursor: hasDetail ? "pointer" : "default" }}
        >
          {summary}
        </div>
        {detailOpen && hasDetail && <TraceDetailView detail={event.detail} onRecoveryPrompt={onRecoveryPrompt} raw={raw} />}
      </div>
    </div>
  );
}

export default ExecutionTrace;
