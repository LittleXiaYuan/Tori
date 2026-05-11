import { describe, expect, it } from "vitest";

import { ChatStreamTimeoutError, legacyChatStreamChunks, parseAgenticChatStream } from "../chat-sse";

function streamFromChunks(chunks: string[]): ReadableStream<Uint8Array> {
  const encoder = new TextEncoder();
  return new ReadableStream<Uint8Array>({
    start(controller) {
      for (const chunk of chunks) controller.enqueue(encoder.encode(chunk));
      controller.close();
    },
  });
}

async function collect(chunks: string[]) {
  const items = [];
  for await (const item of parseAgenticChatStream(streamFromChunks(chunks))) items.push(item);
  return items;
}

describe("parseAgenticChatStream", () => {
  it("parses deltas split across chunks", async () => {
    const items = await collect([
      "data: {\"content\":\"hel",
      "lo\"}\n\n",
      "data: [DONE]\n\n",
    ]);

    expect(items).toEqual([{ kind: "delta", content: "hello" }]);
  });

  it("parses named done and actions events", async () => {
    const items = await collect([
      "event: actions\n",
      "data: [{\"type\":\"save\"}]\n\n",
      "event: done\n",
      "data: {\"emotion\":{\"name\":\"happy\"},\"actions\":[{\"type\":\"open\"}]}\n\n",
    ]);

    expect(items[0]).toMatchObject({ kind: "actions", actions: [{ type: "save" }] });
    expect(items[1]).toMatchObject({ kind: "done", data: { emotion: { name: "happy" } } });
    expect(legacyChatStreamChunks(items[1])).toEqual([
      '\n<!--emotion:{"name":"happy"}-->',
      '\n<!--actions:[{"type":"open"}]-->',
    ]);
  });

  it("parses agent trace events", async () => {
    const items = await collect([
      "data: {\"id\":\"evt-1\",\"domain\":\"planner\",\"type\":\"thinking\",\"summary\":\"working\",\"meta\":{}}\n\n",
    ]);

    expect(items[0]).toMatchObject({ kind: "agent_event", event: { id: "evt-1", domain: "planner" } });
    expect(legacyChatStreamChunks(items[0])).toEqual([
      '{"id":"evt-1","domain":"planner","type":"thinking","summary":"working","meta":{}}',
    ]);
  });

  it("ignores empty ping frames used as keepalives", async () => {
    const items = await collect([
      "event: ping\n",
      "data: \n\n",
      "event: delta\n",
      "data: {\"content\":\"ok\"}\n\n",
    ]);

    expect(items).toEqual([{ kind: "delta", content: "ok" }]);
  });

  it("formats error frames before they are thrown by callers", async () => {
    const items = await collect([
      "event: error\n",
      "data: {\"message\":\"handoff agent execution failed: context deadline exceeded\"}\n\n",
    ]);

    expect(items[0]).toMatchObject({
      kind: "error",
      message: "响应暂时超时，已保留现场，可稍后重试或继续。",
    });
    expect(items[0]?.kind === "error" ? items[0].message : "").not.toContain("context deadline exceeded");
  });

  it("uses a friendly idle-timeout message", () => {
    const err = new ChatStreamTimeoutError(60000);
    expect(err.message).toBe("响应暂时超时，已保留现场；60 秒内没有收到新内容，可以稍后重试或继续。");
    expect(err.message).not.toContain("chat stream idle timeout");
  });
});
