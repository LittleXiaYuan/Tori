import type { AgentEvent } from "@/components/execution-trace";
import { browserActionPhase } from "@/lib/browser-action-labels";
import type { Message } from "@/lib/chat-types";

let msgId = 0;

/** Generate a unique message id, safe across reloads within a tab. */
export function newId(): string {
  return `msg-${++msgId}-${Date.now()}-${Math.random().toString(36).slice(2, 6)}`;
}

/** Short textual phase label for browser-runtime trace events. */
export function browserTraceSummary(
  action?: string | null,
  stage: "start" | "success" | "error" | "handoff" = "success",
): string {
  return browserActionPhase(action, stage);
}

/** Build a synthetic AgentEvent for a browser-bridge action. */
export function makeBrowserTraceEvent(
  summary: string,
  detail?: unknown,
  kind: "tool_start" | "tool_result" | "reflect" = "tool_result",
): AgentEvent {
  const now = new Date().toISOString();
  return {
    id: `browser-trace-${Date.now()}-${Math.random().toString(36).slice(2, 8)}`,
    trace_id: "browser-bridge",
    ts: now,
    domain: "planner",
    type: kind,
    summary,
    detail,
    meta: {
      skill: "browser_runtime",
    },
  };
}

/** Map technical error messages to user-friendly text. */
export function friendlyError(msg: string): string {
  const m = (msg || "").toLowerCase();
  if (m.includes("no provider") || m.includes("provider not"))
    return "No model provider is configured yet. Add an API key in Settings first.";
  if (m.includes("planner_error") || m.includes("planner error"))
    return "The planning step failed. Retry or switch models.";
  if (m.includes("context deadline") || m.includes("timeout"))
    return "The request timed out. Please try again.";
  if (m.includes("rate limit") || m.includes("429"))
    return "Too many requests right now. Please retry in a moment.";
  if (m.includes("401") || m.includes("unauthorized") || m.includes("invalid api key"))
    return "The API key looks invalid or expired. Check provider settings.";
  if (m.includes("502") || m.includes("503") || m.includes("bad gateway"))
    return "The upstream model service is temporarily unavailable.";
  if (
    m.includes("failed to fetch") ||
    m.includes("network") ||
    m.includes("err_connection") ||
    m.includes("load failed")
  )
    return "Network connection lost. Please check your connection and try again.";
  if (m.includes("request failed"))
    return "The request failed. Check network or service status and try again.";
  return msg;
}

/** Collect files produced by tool events into a deduplicated list. */
export function collectGeneratedFiles(
  traceEvents?: AgentEvent[],
): Array<{ path: string; name: string; size?: number }> {
  const files: Array<{ path: string; name: string; size?: number }> = [];
  for (const evt of traceEvents || []) {
    const detail = evt.detail as Record<string, unknown> | undefined;
    if (detail && Array.isArray(detail.files)) {
      for (const file of detail.files as Array<{
        path: string;
        name: string;
        size?: number;
      }>) {
        if (!files.some((entry) => entry.path === file.path)) files.push(file);
      }
    }
  }
  return files;
}

/** Derive a compact summary of assistant work for a message. */
export function summarizeAssistantWork(message: Message): {
  toolCount: number;
  skillCount: number;
  primarySkill: string;
  fileCount: number;
  warningCount: number;
} {
  const traceEvents = message.traceEvents || [];
  const toolEvents = traceEvents.filter(
    (event) => event.type === "tool_start" || event.type === "tool_result",
  );
  const skills = [
    ...new Set(toolEvents.map((event) => event.meta?.skill).filter(Boolean)),
  ];
  const files = collectGeneratedFiles(traceEvents);
  const warnings = traceEvents.filter((event) => {
    const summary = (event.summary || "").toLowerCase();
    return (
      event.type === "plan" &&
      (summary.includes("warning") ||
        summary.includes("risk") ||
        summary.includes("blocked") ||
        summary.includes("needs review"))
    );
  });

  return {
    toolCount: toolEvents.length,
    skillCount: skills.length,
    primarySkill: String(skills[skills.length - 1] || ""),
    fileCount: files.length,
    warningCount: warnings.length,
  };
}
