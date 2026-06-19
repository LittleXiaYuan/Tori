import type { AgentEntry, TraceEntry } from "./micro-agent-pack-client";

export function microAgentTaskPrompt(agent: AgentEntry): string {
  return [
    "请基于这个微代理开始一个任务。先说明它适合处理什么，再问我需要补充哪些输入：",
    `微代理：${agent.name}`,
    agent.description ? `说明：${agent.description}` : "",
    agent.trigger ? `触发词：${agent.trigger}` : "",
    agent.tags?.length ? `标签：${agent.tags.join("、")}` : "",
  ].filter(Boolean).join("\n");
}

export function microAgentTraceSummaryPrompt(taskId: string, entries: TraceEntry[]): string {
  const lines = entries.slice(0, 12).map((entry) => {
    const summary = previewTracePayload(entry.payload);
    return `- ${entry.kind} · ${entry.actor}${summary ? `：${summary}` : ""}`;
  });
  return [
    "请基于这段 ReAct 推理轨迹，帮我总结任务是怎么推进的、哪里可能卡住、下一步怎么做：",
    `任务 ID：${taskId}`,
    lines.length ? "轨迹摘要：" : "",
    ...lines,
  ].filter(Boolean).join("\n");
}

export function previewTracePayload(payload?: Record<string, unknown>): string {
  if (!payload) return "";
  const candidates = ["thought", "decision", "observation", "reason", "answer", "reflection", "summary"];
  for (const key of candidates) {
    const value = payload[key];
    if (typeof value === "string" && value.trim() !== "") return value;
  }
  for (const value of Object.values(payload)) {
    if (typeof value === "string" && value.trim() !== "") return value;
  }
  return "";
}
