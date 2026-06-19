import { describe, expect, it } from "vitest";
import { chatPromptHref, taskDetailHref, traceTaskHref } from "../pack-action-links";

describe("pack action links", () => {
  it("builds chat handoff links with encoded prompts", () => {
    expect(chatPromptHref("继续探索：好奇心 / 任务")).toBe(
      "/chat?q=%E7%BB%A7%E7%BB%AD%E6%8E%A2%E7%B4%A2%EF%BC%9A%E5%A5%BD%E5%A5%87%E5%BF%83%20%2F%20%E4%BB%BB%E5%8A%A1",
    );
  });

  it("builds task and trace links from ids", () => {
    expect(taskDetailHref("task 1")).toBe("/task-detail?id=task%201");
    expect(traceTaskHref("task 1")).toBe("/trace?task=task%201");
  });
});
