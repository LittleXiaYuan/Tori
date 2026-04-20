import { describe, expect, it } from "vitest";
import {
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
    ["no provider configured for this request", /model provider/i],
    ["planner_error: budget exceeded", /planning step failed/i],
    ["context deadline exceeded after 30s", /timed out/i],
    ["429 Too Many Requests", /Too many requests/i],
    ["401 Unauthorized: invalid api key", /API key/i],
    ["502 Bad Gateway — upstream", /upstream model service/i],
    ["failed to fetch", /network connection lost/i],
    ["Request failed with status 500", /request failed/i],
  ];
  for (const [input, pattern] of cases) {
    it(`maps "${input}" to a friendly sentence`, () => {
      const out = friendlyError(input);
      expect(out).toMatch(pattern);
    });
  }

  it("falls through untouched when no pattern matches", () => {
    const odd = "some bespoke situation that does not match any heuristic";
    expect(friendlyError(odd)).toBe(odd);
  });

  it("tolerates empty / non-string-ish inputs", () => {
    expect(friendlyError("")).toBe("");
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
