"use client";

import { useMemo, useState } from "react";
import {
  CheckCircle2, Circle, Loader2, XCircle,
  Globe, FileText, Code2, Search, Bot, ChevronDown, ChevronRight,
} from "lucide-react";
import type { AgentEvent } from "./execution-trace";
import { formatErrorMessage } from "@/lib/error-utils";

interface TaskStep {
  id: string;
  agent: string;
  label: string;
  status: "pending" | "running" | "done" | "error";
  subSteps: { id: string; summary: string; status: "done" | "running" | "error" }[];
  durMs?: number;
}

const agentMeta: Record<string, { icon: React.ElementType; label: string; color: string }> = {
  browser_exec:  { icon: Globe,    label: "浏览器操作", color: "#3b82f6" },
  file_exec:     { icon: FileText, label: "文件处理",   color: "#f59e0b" },
  code_exec:     { icon: Code2,    label: "代码执行",   color: "#8b5cf6" },
  research_exec: { icon: Search,   label: "信息搜集",   color: "#10b981" },
  general_exec:  { icon: Bot,      label: "通用任务",   color: "#6366f1" },
};

function friendlyProgressText(text: string): string {
  const stripped = text
    .replace(/^🤖\s*委派\s*\[.*?\][：:]\s*/, "")
    .replace(/^❌\s*\[.*?\]\s*失败[：:]\s*/, "")
    .trim();
  return formatErrorMessage(stripped || text, stripped || text);
}

function buildSteps(events: AgentEvent[]): TaskStep[] {
  const steps: TaskStep[] = [];
  let current: TaskStep | null = null;
  const runningAgents = new Set<string>();

  for (const evt of events) {
    if (evt.domain === "agent" && evt.type === "handoff_start") {
      const detail = evt.detail as { agent?: string; input?: string } | undefined;
      const agent = detail?.agent || evt.meta?.skill || "unknown";

      if (runningAgents.has(agent)) {
        continue;
      }
      runningAgents.add(agent);

      current = {
        id: evt.id,
        agent,
        label: friendlyProgressText(detail?.input || evt.summary),
        status: "running",
        subSteps: [],
      };
      steps.push(current);
      continue;
    }

    if (evt.domain === "agent" && evt.type === "handoff_done") {
      const detail = evt.detail as { agent?: string; error?: string; dur_ms?: number } | undefined;
      const agent = detail?.agent || evt.meta?.skill || "";
      const match = steps.findLast((s) => s.agent === agent && s.status === "running");
      if (match) {
        match.status = detail?.error ? "error" : "done";
        match.durMs = detail?.dur_ms;
      }
      runningAgents.delete(agent);
      current = null;
      continue;
    }

    if (current && (evt.type === "tool_start" || evt.type === "tool_result" || evt.type === "thinking" || evt.type === "reflect")) {
      const status = evt.type === "tool_start" || evt.type === "thinking" ? "running" : "done";
      if (evt.type === "tool_result" || evt.type === "reflect") {
        const prev = current.subSteps.findLast((s) => s.status === "running");
        if (prev) prev.status = "done";
      }
      if (evt.type === "tool_start" || evt.type === "thinking") {
        current.subSteps.push({ id: evt.id, summary: friendlyProgressText(evt.summary), status });
      }
    }
  }

  return steps;
}

function statusIcon(status: string) {
  switch (status) {
    case "done":    return <CheckCircle2 size={14} style={{ color: "#34d399" }} />;
    case "running": return <Loader2 size={14} className="animate-spin" style={{ color: "#60a5fa" }} />;
    case "error":   return <XCircle size={14} style={{ color: "#f87171" }} />;
    default:        return <Circle size={14} style={{ color: "var(--yunque-text-muted)", opacity: 0.4 }} />;
  }
}

interface TaskProgressPanelProps {
  events: AgentEvent[];
  isLive?: boolean;
}

export function TaskProgressPanel({ events, isLive }: TaskProgressPanelProps) {
  const steps = useMemo(() => buildSteps(events), [events]);
  const [collapsed, setCollapsed] = useState(false);

  if (steps.length === 0) return null;

  const done = steps.filter((s) => s.status === "done").length;
  const total = steps.length;

  return (
    <div
      className="rounded-xl overflow-hidden text-[13px]"
      style={{ background: "var(--yunque-card)", border: "1px solid var(--yunque-border)" }}
    >
      <button
        type="button"
        className="flex items-center justify-between w-full px-4 py-2.5 cursor-pointer hover:opacity-80 transition-opacity"
        style={{ borderBottom: collapsed ? "none" : "1px solid var(--yunque-border)", background: "transparent" }}
        onClick={() => setCollapsed((c) => !c)}
      >
        <div className="flex items-center gap-2">
          {collapsed ? <ChevronRight size={14} style={{ color: "var(--yunque-text-muted)" }} /> : <ChevronDown size={14} style={{ color: "var(--yunque-text-muted)" }} />}
          <span className="font-semibold text-sm" style={{ color: "var(--yunque-text)" }}>任务进度</span>
          {isLive && (
            <span className="text-[10px] font-semibold animate-pulse" style={{ color: "#34d399" }}>● LIVE</span>
          )}
        </div>
        <span className="text-xs font-mono" style={{ color: "var(--yunque-text-muted)" }}>
          {done} / {total}
        </span>
      </button>

      {!collapsed && (
        <>
          <div className="px-4 py-3 space-y-1">
            {steps.map((step, i) => {
              const meta = agentMeta[step.agent] || { icon: Bot, label: step.agent, color: "#6b7280" };
              const Icon = meta.icon;
              const isActive = step.status === "running";

              return (
                <div key={step.id} className="relative">
                  {i < steps.length - 1 && (
                    <div
                      className="absolute left-[6.5px] top-6 bottom-0 w-0.5"
                      style={{ background: step.status === "done" ? "#34d399" : "var(--yunque-border)" }}
                    />
                  )}

                  <div className="flex items-start gap-2.5 py-1.5">
                    <div className="mt-0.5 shrink-0">{statusIcon(step.status)}</div>
                    <div className="flex-1 min-w-0">
                      <div className="flex items-center gap-1.5">
                        <Icon size={12} style={{ color: meta.color }} />
                        <span className="text-xs font-medium truncate" style={{ color: "var(--yunque-text)" }}>
                          {meta.label}
                        </span>
                        {step.durMs != null && (
                          <span className="text-[10px] ml-auto font-mono shrink-0" style={{ color: "var(--yunque-text-muted)" }}>
                            {step.durMs < 1000 ? `${step.durMs}ms` : `${(step.durMs / 1000).toFixed(1)}s`}
                          </span>
                        )}
                      </div>
                      <p className="text-[11px] mt-0.5 line-clamp-2" style={{ color: "var(--yunque-text-muted)" }}>
                        {step.label}
                      </p>

                      {isActive && step.subSteps.length > 0 && (
                        <div className="mt-1.5 ml-1 space-y-0.5">
                          {step.subSteps.slice(-4).map((sub) => (
                            <div key={sub.id} className="flex items-center gap-1.5 text-[11px]" style={{ color: "var(--yunque-text-muted)" }}>
                              {sub.status === "running" ? (
                                <Loader2 size={10} className="animate-spin shrink-0" style={{ color: "#60a5fa" }} />
                              ) : (
                                <CheckCircle2 size={10} className="shrink-0" style={{ color: "#34d399" }} />
                              )}
                              <span className="truncate">{sub.summary}</span>
                            </div>
                          ))}
                        </div>
                      )}
                    </div>
                  </div>
                </div>
              );
            })}
          </div>

          <div className="px-4 pb-3">
            <div className="h-1 rounded-full overflow-hidden" style={{ background: "var(--yunque-border)" }}>
              <div
                className="h-full rounded-full transition-all duration-500"
                style={{ width: `${total > 0 ? (done / total) * 100 : 0}%`, background: "linear-gradient(90deg, #34d399, #3b82f6)" }}
              />
            </div>
          </div>
        </>
      )}
    </div>
  );
}

export default TaskProgressPanel;
