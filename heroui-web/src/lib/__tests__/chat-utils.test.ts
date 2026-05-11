import { describe, expect, it } from "vitest";
import {
  chatHttpErrorMessage,
  collectGeneratedFiles,
  friendlyError,
  newId,
} from "../chat-utils";
import type { AgentEvent } from "@/components/execution-trace";

// Smoke tests for the pure helpers extracted out of chat/page.tsx. These
// used to live inside a 2.5k-line client component and were therefore
// impossible to test in isolation; now that they're in lib/chat-utils we
// exercise the observable contracts. None of these helpers touch the DOM
// or the network, so plain node-env Vitest is enough.

describe("chat-utils/newId", () => {
  it("returns a string that starts with msg- and is monotonically unique", () => {
    const a = newId();
    const b = newId();
    expect(a).toMatch(/^msg-\d+-\d+-[a-z0-9]{4}$/);
    expect(b).toMatch(/^msg-\d+-\d+-[a-z0-9]{4}$/);
    expect(a).not.toBe(b);
  });
});

describe("chat-utils/friendlyError", () => {
  const cases: Array<[string, RegExp]> = [
    ["no provider configured for this request", /还没有配置可用模型/],
    ["planner_error: budget exceeded", /规划这一步暂时没有顺利完成/],
    ["context deadline exceeded after 30s", /响应暂时超时/],
    ["429 Too Many Requests", /当前请求较多/],
    ["401 Unauthorized: invalid api key", /模型密钥/],
    ["llm api 404: {\"error_message\":\"404, token not found\"}", /模型密钥/],
    ["502 Bad Gateway — upstream", /模型服务暂时不可用/],
    ["failed to fetch", /连接暂时中断/],
    ["Request failed with status 500", /请求暂时没有完成/],
  ];
  for (const [input, pattern] of cases) {
    it(`maps "${input}" to a friendly sentence`, () => {
      const out = friendlyError(input);
      expect(out).toMatch(pattern);
    });
  }


  it("keeps dropped-task wording recoverable and hides raw transport detail", () => {
    const out = friendlyError("任务已执行但连接中断。");
    expect(out).toContain("现场已保留");
    expect(out).toContain("最近可恢复任务");
    expect(out).not.toContain("EOF");
    expect(out).not.toContain("fallback");
  });

  it("hides raw planner fallback and model-escalation traces seen in chat reasoning", () => {
    const fallback = friendlyError('planner fc step 1: all fallback LLM clients failed (FC): chat with tools: Post "https://api.moonshot.ai/v1/chat/completions": EOF');
    expect(fallback).toContain("已保留现场");
    expect(fallback).not.toMatch(/planner fc|all fallback|moonshot|EOF/i);

    const escalation = friendlyError("当前：调用栈降级，正在级联唤醒备用引擎 [qwen3.5:4b]...");
    expect(escalation).toBe("模型暂时没有回应，正在换用可用模型继续。");
    expect(escalation).not.toContain("qwen3.5");
  });

  it("falls through untouched when no pattern matches", () => {
    const odd = "some bespoke situation that does not match any heuristic";
    expect(friendlyError(odd)).toBe(odd);
  });

  it("tolerates empty / non-string-ish inputs", () => {
    expect(friendlyError("")).toBe("");
  });
});

describe("chat-utils/chatHttpErrorMessage", () => {
  it("preserves friendly backend planner errors from failed chat responses", async () => {
    const resp = new Response(JSON.stringify({
      code: "planner_failed",
      detail: 'planner fc step 1: all fallback LLM clients failed (FC): chat with tools: Post "https://api.moonshot.ai/v1/chat/completions": EOF',
    }), { status: 502, statusText: "Bad Gateway" });

    const out = await chatHttpErrorMessage(resp);
    expect(out).toContain("已保留现场");
    expect(out).not.toMatch(/planner fc|all fallback|moonshot|EOF/i);
  });

  it("unwraps nested backend error objects before formatting", async () => {
    const resp = new Response(JSON.stringify({
      error: { message: "handoff agent execution failed: context deadline exceeded" },
    }), { status: 500 });

    await expect(chatHttpErrorMessage(resp)).resolves.toBe("响应暂时超时，已保留现场，可稍后重试或继续。");
  });

  it("falls back to response status when the failed chat response has no body", async () => {
    const resp = new Response("", { status: 503, statusText: "Service Unavailable" });
    await expect(chatHttpErrorMessage(resp)).resolves.toBe("请求失败 (503 Service Unavailable)");
  });
});

describe("chat-utils/collectGeneratedFiles", () => {
  const makeEvt = (
    files?: Array<{ path: string; name: string; size?: number }>,
  ): AgentEvent => ({
    id: `evt-${Math.random()}`,
    trace_id: "t",
    ts: new Date().toISOString(),
    domain: "planner",
    type: "tool_result",
    summary: "done",
    detail: files ? { files } : undefined,
    meta: {},
  });

  it("returns an empty list for undefined / empty input", () => {
    expect(collectGeneratedFiles()).toEqual([]);
    expect(collectGeneratedFiles([])).toEqual([]);
  });

  it("flattens files across multiple events", () => {
    const events = [
      makeEvt([{ path: "/a.txt", name: "a.txt", size: 1 }]),
      makeEvt([{ path: "/b.md", name: "b.md" }]),
    ];
    const out = collectGeneratedFiles(events);
    expect(out.map((f) => f.path)).toEqual(["/a.txt", "/b.md"]);
  });

  it("dedupes by path so the same file produced twice appears once", () => {
    const events = [
      makeEvt([{ path: "/report.md", name: "report.md" }]),
      makeEvt([{ path: "/report.md", name: "report.md" }]),
      makeEvt([{ path: "/report.md", name: "report-v2.md" }]),
    ];
    const out = collectGeneratedFiles(events);
    expect(out).toHaveLength(1);
    expect(out[0].path).toBe("/report.md");
  });

  it("ignores events without a files detail", () => {
    const events = [
      makeEvt(undefined),
      makeEvt([{ path: "/kept.txt", name: "kept.txt" }]),
      makeEvt(undefined),
    ];
    const out = collectGeneratedFiles(events);
    expect(out).toHaveLength(1);
    expect(out[0].path).toBe("/kept.txt");
  });
});
