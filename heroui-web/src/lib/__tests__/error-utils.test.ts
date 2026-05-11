import { describe, expect, it } from "vitest";
import { formatErrorMessage } from "@/lib/error-utils";

describe("formatErrorMessage", () => {
  it("passes through strings", () => {
    expect(formatErrorMessage("network down")).toBe("network down");
  });

  it("formats backend error objects", () => {
    expect(formatErrorMessage({ code: "provider_error", detail: "missing provider", message: "No provider" })).toBe("provider_error: No provider");
  });

  it("falls back to detail when message is absent", () => {
    expect(formatErrorMessage({ code: "bad_request", detail: "invalid token" })).toBe("bad_request: invalid token");
  });

  it("serializes unknown objects instead of returning React-hostile values", () => {
    expect(formatErrorMessage({ retryable: false })).toBe('{"retryable":false}');
  });

  it("uses the fallback for empty values", () => {
    expect(formatErrorMessage(null, "fallback")).toBe("fallback");
  });

  it("maps dependency-block runner errors to a recovery hint", () => {
    expect(formatErrorMessage("步骤 2 等待依赖步骤完成：1")).toBe("等待前置步骤完成，可进入任务详情确认依赖后继续。");
  });

  it("hides raw timeout and handoff implementation errors", () => {
    expect(formatErrorMessage(new Error("handoff agent execution failed: context deadline exceeded"))).toBe("响应暂时超时，已保留现场，可稍后重试或继续。");
    expect(formatErrorMessage({ code: "planner_failed", detail: "all fallback LLM clients failed (FC): EOF" })).toBe("任务暂时没有完成，已保留现场，可切换策略或稍后继续。");
    expect(formatErrorMessage("context canceled")).toBe("连接暂时中断，已保留现场，可稍后继续或先查看阶段结果。");
    expect(formatErrorMessage("context cancelled")).toBe("连接暂时中断，已保留现场，可稍后继续或先查看阶段结果。");
  });

  it("hides model fallback escalation wording", () => {
    expect(formatErrorMessage("当前模型响应失败，正在尝试备用模型 qwen3.5:4b...")).toBe("模型暂时没有回应，正在换用可用模型继续。");
    expect(formatErrorMessage("调用栈降级，正在级联唤醒备用引擎")).toBe("模型暂时没有回应，正在换用可用模型继续。");
  });

  it("hides raw tool execution implementation errors", () => {
    expect(formatErrorMessage("unknown skill: file_exec")).toBe("所需工具暂时不可用，已保留现场，可换用可用工具或调整步骤继续。");
    expect(formatErrorMessage({ reason: "blocked by trust gate: needs approval" })).toBe("这一步需要更高信任或确认，已保留现场，可确认后继续。");
    expect(formatErrorMessage(new Error("tool panic: nil pointer"))).toBe("工具运行时遇到异常，已保留现场，可重试或切换策略继续。");
  });
});
