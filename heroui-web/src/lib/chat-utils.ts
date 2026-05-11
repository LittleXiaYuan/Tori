import type { AgentEvent } from "@/components/execution-trace";
import { browserActionPhase } from "@/lib/browser-action-labels";
import type { Message } from "@/lib/chat-types";
import { formatErrorMessage } from "@/lib/error-utils";

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
  const raw = msg || "";
  const formatted = formatErrorMessage(raw, raw);
  if (formatted !== raw) return formatted;

  const m = raw.toLowerCase();
  if (m.includes("no provider") || m.includes("provider not"))
    return "还没有配置可用模型，请先到模型设置里添加密钥。";
  if (m.includes("planner_error") || m.includes("planner error"))
    return "规划这一步暂时没有顺利完成，已保留现场，可重试或切换策略继续。";
  if (m.includes("context deadline") || m.includes("timeout"))
    return "响应暂时超时，已保留现场，可稍后重试或继续。";
  if (m.includes("任务已执行但连接中断") || m.includes("connection closed") || m.includes("connection reset"))
    return "连接暂时中断，现场已保留；如果任务已经推进，可以从最近可恢复任务继续。";
  if (m.includes("rate limit") || m.includes("429"))
    return "当前请求较多，请稍后重试。";
  if (
    m.includes("401") ||
    m.includes("unauthorized") ||
    m.includes("invalid api key") ||
    m.includes("token not found") ||
    (m.includes("404") && m.includes("token"))
  )
    return "模型密钥可能无效或已过期，请检查模型设置。";
  if (m.includes("502") || m.includes("503") || m.includes("bad gateway"))
    return "模型服务暂时不可用，现场已保留，可稍后重试或换用其它模型。";
  if (
    m.includes("failed to fetch") ||
    m.includes("network") ||
    m.includes("err_connection") ||
    m.includes("load failed")
  )
    return "连接暂时中断，现场已保留；请检查服务状态后重试，或从最近可恢复任务继续。";
  if (m.includes("request failed"))
    return "请求暂时没有完成，已保留现场，可检查服务状态后重试。";
  return raw;
}

function responseStatusText(resp: Response): string {
  return `请求失败 (${resp.status}${resp.statusText ? " " + resp.statusText : ""})`;
}

function normalizeErrorPayload(value: unknown): unknown {
  if (typeof value !== "object" || value === null || Array.isArray(value)) return value;
  const record = value as Record<string, unknown>;
  if (typeof record.message === "string" || typeof record.detail === "string" || typeof record.error === "string" || typeof record.reason === "string") {
    return record;
  }
  if (typeof record.error === "object" && record.error !== null) return normalizeErrorPayload(record.error);
  if (typeof record.detail === "object" && record.detail !== null) return normalizeErrorPayload(record.detail);
  return record;
}

/** Read a failed chat HTTP response body and preserve backend friendly errors. */
export async function chatHttpErrorMessage(resp: Response): Promise<string> {
  const fallback = responseStatusText(resp);
  let body = "";
  try {
    body = (await resp.clone().text()).trim();
  } catch {
    return fallback;
  }
  if (!body) return fallback;

  let payload: unknown = body;
  try {
    payload = normalizeErrorPayload(JSON.parse(body));
  } catch {
    payload = body;
  }
  return formatErrorMessage(payload, fallback);
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
  latestSummary: string;
  sourceKinds: string[];
} {
  const traceEvents = message.traceEvents || [];
  const toolEvents = traceEvents.filter(
    (event) => event.type === "tool_start" || event.type === "tool_result",
  );
  const skills = [
    ...new Set(toolEvents.map((event) => event.meta?.skill).filter(Boolean)),
  ];
  const files = collectGeneratedFiles(traceEvents);
  const sourceKinds = [
    ...new Set(
      toolEvents.map((event) => {
        const text = `${event.meta?.skill || ""} ${event.type || ""} ${event.summary || ""}`.toLowerCase();
        if (text.includes("browser") || text.includes("navigate") || text.includes("screenshot") || text.includes("search")) return "浏览器";
        if (text.includes("shell") || text.includes("exec") || text.includes("command") || text.includes("run") || text.includes("terminal")) return "终端";
        if (text.includes("read") || text.includes("write") || text.includes("edit") || text.includes("file") || text.includes("grep") || text.includes("workspace")) return "文件";
        if (text.includes("sandbox") || text.includes("computer") || text.includes("daytona") || text.includes("e2b") || text.includes("cloud")) return "云电脑";
        if (text.includes("worker") || text.includes("ide") || text.includes("cursor") || text.includes("windsurf")) return "AI IDE";
        return "工具";
      }),
    ),
  ];
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
    latestSummary: traceEvents[traceEvents.length - 1]?.summary || "",
    sourceKinds,
  };
}
