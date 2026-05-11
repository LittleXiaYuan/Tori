import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { TaskProgressPanel } from "../task-progress-panel";
import type { AgentEvent } from "../execution-trace";

function evt(id: string, type: string, summary: string, detail?: unknown): AgentEvent {
  return {
    id,
    trace_id: "trace-progress",
    ts: "2026-05-11T00:00:00.000Z",
    domain: "agent",
    type,
    summary,
    detail,
    meta: {},
  };
}

describe("TaskProgressPanel", () => {
  it("hides raw model fallback wording inside progress substeps", () => {
    render(<TaskProgressPanel isLive events={[
      evt("handoff-start", "handoff_start", "🤖 委派 [file_exec]：读取附件", { agent: "file_exec", input: "读取附件并总结" }),
      evt("thinking-1", "thinking", "当前模型响应失败，正在尝试备用模型 qwen3.5:4b..."),
    ]} />);

    expect(screen.getByText("任务进度")).toBeInTheDocument();
    expect(screen.getByText("读取附件并总结")).toBeInTheDocument();
    expect(screen.getByText("模型暂时没有回应，正在换用可用模型继续。")).toBeInTheDocument();
    expect(screen.queryByText(/备用模型|qwen3\.5|当前模型响应失败/)).toBeNull();
  });

  it("hides raw handoff failure wording when used as a progress label", () => {
    render(<TaskProgressPanel events={[
      evt("handoff-start", "handoff_start", "❌ [general_exec] 失败: handoff agent execution failed: context deadline exceeded", { agent: "general_exec" }),
    ]} />);

    expect(screen.getByText("响应暂时超时，已保留现场，可稍后重试或继续。")).toBeInTheDocument();
    expect(screen.queryByText(/context deadline exceeded|handoff agent|execution failed/)).toBeNull();
  });
});
