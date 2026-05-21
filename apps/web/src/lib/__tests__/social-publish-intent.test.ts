import { describe, expect, it } from "vitest";
import {
  buildSocialPublishActions,
  detectSocialPublishIntent,
  formatSocialPublishResult,
} from "../social-publish-intent";

describe("social-publish-intent", () => {
  it("detects a Xiaohongshu direct publish request from natural language", () => {
    const intent = detectSocialPublishIntent("帮我在小红书发布一条效率演示笔记，标题：云雀效率演示，正文：今天展示从对话到直发的完整链路。");

    expect(intent).toMatchObject({
      platform: "xiaohongshu",
      platformName: "小红书",
      scenarioId: "xiaohongshu-post-direct",
      title: "云雀效率演示",
    });
    expect(intent?.body).toContain("今天展示从对话到直发的完整链路");
  });

  it("does not treat draft-only copywriting as a direct publish action", () => {
    expect(detectSocialPublishIntent("只帮我写小红书草稿不要发布")).toBeNull();
    expect(detectSocialPublishIntent("帮我生成小红书文案，先不要发出去")).toBeNull();
  });

  it("detects X/Twitter direct publishing and builds executable browser steps", () => {
    const intent = detectSocialPublishIntent("发一条 X 推文：云雀现在可以从 Chat 直接执行浏览器发布。");

    expect(intent).toMatchObject({
      platform: "x",
      platformName: "X/Twitter",
      scenarioId: "x-post-direct",
    });
    expect(intent?.body).toContain("云雀现在可以从 Chat 直接执行浏览器发布");

    const actions = buildSocialPublishActions(intent!);
    expect(actions.map((action) => action.type)).toEqual([
      "browser_navigate",
      "browser_screenshot",
      "browser_click",
      "browser_input",
      "browser_screenshot",
      "browser_click",
      "browser_screenshot",
    ]);
  });

  it("formats completed result with direct-publish boundary", () => {
    const intent = detectSocialPublishIntent("帮我在小红书发一条关于云雀浏览器自动化的帖子")!;
    const content = formatSocialPublishResult(intent, [
      { index: 0, label: "打开小红书发布入口", actionType: "browser_navigate", ok: true, hasScreenshot: true },
      { index: 1, label: "点击发布", actionType: "browser_click", ok: true, hasScreenshot: true },
    ]);

    expect(content).toContain("小红书直发流程已从对话执行完成");
    expect(content).toContain("不是只生成草稿");
  });
});
