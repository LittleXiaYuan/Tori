import { describe, expect, it } from "vitest";
import { microAgentTaskPrompt, microAgentTraceSummaryPrompt, previewTracePayload } from "../micro-agent-pack-actions";

describe("micro-agent pack actions", () => {
  it("builds a chat prompt for starting work with a micro-agent", () => {
    const prompt = microAgentTaskPrompt({
      name: "code-reviewer",
      description: "审查代码风险",
      scope: "repo",
      trigger: "review",
      content: "You review code.",
      enabled: true,
      priority: 10,
      tags: ["code", "risk"],
    });

    expect(prompt).toContain("code-reviewer");
    expect(prompt).toContain("审查代码风险");
    expect(prompt).toContain("review");
    expect(prompt).toContain("code、risk");
  });

  it("summarizes a ReAct trace handoff prompt", () => {
    const prompt = microAgentTraceSummaryPrompt("task-1", [
      {
        id: "evt-1",
        kind: "reasoning.thought",
        actor: "planner",
        created_at: "2026-06-19T00:00:00Z",
        payload: { thought: "先读取需求" },
      },
      {
        id: "evt-2",
        kind: "reasoning.decision",
        actor: "reviewer",
        created_at: "2026-06-19T00:00:01Z",
        payload: { decision: "进入代码审查" },
      },
    ]);

    expect(prompt).toContain("task-1");
    expect(prompt).toContain("reasoning.thought");
    expect(prompt).toContain("先读取需求");
    expect(prompt).toContain("进入代码审查");
  });

  it("previews trace payloads from known or fallback string fields", () => {
    expect(previewTracePayload({ observation: "工具返回结果" })).toBe("工具返回结果");
    expect(previewTracePayload({ custom: "备用摘要" })).toBe("备用摘要");
    expect(previewTracePayload({ count: 1 })).toBe("");
  });
});
