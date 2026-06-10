import type { ChatDispatch } from "@/lib/chat-state";
import type { AgentEvent } from "@/components/execution-trace";
import type { Message } from "@/lib/chat-types";
import type {
  BrowserActionArtifactSummary,
  BrowserSessionNotice,
} from "@/components/browser-session-card";
import type { createBrowserIntentPackClient } from "@/lib/browser-intent-pack-client";
import {
  newId,
  browserTraceSummary,
  makeBrowserTraceEvent,
  friendlyError,
} from "@/lib/chat-utils";
import {
  parseSlashBrowserCommand,
  buildSlashBrowserAction,
  summarizeSlashBrowserResult,
  formatSlashBrowserResponse,
} from "@/lib/slash-commands";
import {
  buildSocialPublishActions,
  detectSocialPublishIntent,
  formatSocialPublishConnectorRequired,
  formatSocialPublishResult,
  socialPublishStepLabel,
  stepRecordFromResult,
  type SocialPublishStepRecord,
} from "@/lib/social-publish-intent";

type SuggestedTab = "terminal" | "browser" | "editor" | "thinking" | undefined;

/**
 * ChatBrowserActionContext bundles the chat-page state/dispatch handles the
 * browser-intent flows need. It exists so the slash-command and social-publish
 * branches can live outside the 500-line `sendMessage` closure while keeping the
 * exact same side effects. All setters mirror the corresponding `useState`/
 * `useBrowserBridge` handles from the chat page.
 */
export interface ChatBrowserActionContext {
  browserIntentClient: ReturnType<typeof createBrowserIntentPackClient>;
  chatD: ChatDispatch;
  pushBrowserTrace: (event: AgentEvent) => void;
  syncBridgeState: () => void;
  setBridgeNotice: (notice: BrowserSessionNotice | null) => void;
  setLastArtifact: (artifact: BrowserActionArtifactSummary | null) => void;
  setSuggestedTab: (tab: SuggestedTab) => void;
  setShowComputer: (show: boolean) => void;
  setShowConnectors: (show: boolean) => void;
  setResumePromptForBrowser: (prompt: string | null) => void;
  setActiveSlashCommand: (command: string | null) => void;
  setShowSlashMenu: (show: boolean) => void;
}

/**
 * runSlashBrowserCommand handles `/browser …`-style slash commands by driving
 * the browser-intent pack directly. Returns true when the message was a browser
 * slash command (handled here), false when the caller should keep processing.
 */
export async function runSlashBrowserCommand(
  ctx: ChatBrowserActionContext,
  displayText: string,
  text: string,
): Promise<boolean> {
  const slashBrowserCommand = parseSlashBrowserCommand(displayText);
  if (!slashBrowserCommand) return false;

  const {
    browserIntentClient, chatD, pushBrowserTrace, syncBridgeState,
    setBridgeNotice, setLastArtifact, setSuggestedTab, setShowComputer,
    setShowConnectors, setResumePromptForBrowser, setActiveSlashCommand,
    setShowSlashMenu,
  } = ctx;

  setSuggestedTab("browser");
  setShowComputer(true);
  const extStatus = await browserIntentClient.extensionStatus().catch(() => ({ connected: false }));
  if (!extStatus.connected) {
    setShowConnectors(true);
    setResumePromptForBrowser(text);
    setBridgeNotice({ tone: "warning", text: "Browser extension not connected. Opened install guide for you." });
    pushBrowserTrace(makeBrowserTraceEvent(
      "Browser extension required",
      { command: slashBrowserCommand.command, args: slashBrowserCommand.args, summary: slashBrowserCommand.summary },
      "reflect",
    ));
    const userMsg: Message = { role: "user", content: displayText, id: newId(), timestamp: Date.now() };
    const asstMsg: Message = {
      role: "assistant",
      content: [
        "The browser extension is not connected yet.",
        "I opened the browser install guide for you. Connect **Yunque Browser Connector**, then run this command again.",
        "",
        "Open the workspace here: [/packs/browser](/packs/browser)",
      ].join("\n"),
      id: newId(),
      browserRequirement: {
        required: true,
        reason: "browser_connector_required",
        message: "This command needs the live Yunque Browser Connector before it can operate your real browser tab.",
        install_path: "/packs/browser",
        settings_path: "/packs/browser",
      },
      traceEvents: [makeBrowserTraceEvent("Opened browser install guide", { source: "chat-slash", command: slashBrowserCommand.command }, "reflect")],
    };
    chatD({ type: "SET_INPUT", value: "" });
    chatD({ type: "ADD_PAIR", userMsg, asstMsg });
    setActiveSlashCommand(null);
    setShowSlashMenu(false);
    if (typeof window !== "undefined") {
      window.setTimeout(() => window.open("/packs/browser", "_blank", "noopener,noreferrer"), 80);
    }
    return true;
  }

  const builtAction = buildSlashBrowserAction(slashBrowserCommand);
  if ("error" in builtAction) {
    const errorMessage = builtAction.error || "Browser command needs clarification.";
    const userMsg: Message = { role: "user", content: displayText, id: newId(), timestamp: Date.now() };
    const asstMsg: Message = {
      role: "assistant",
      content: errorMessage,
      id: newId(),
      traceEvents: [makeBrowserTraceEvent("Browser command needs clarification", { command: slashBrowserCommand.command, args: slashBrowserCommand.args }, "reflect")],
    };
    chatD({ type: "SET_INPUT", value: "" });
    chatD({ type: "ADD_PAIR", userMsg, asstMsg });
    setActiveSlashCommand(null);
    setShowSlashMenu(false);
    return true;
  }

  const userMsg: Message = { role: "user", content: displayText, id: newId(), timestamp: Date.now() };
  const asstMsg: Message = { role: "assistant", content: "", id: newId(), timestamp: Date.now(), traceEvents: [] };
  setActiveSlashCommand(null);
  setShowSlashMenu(false);
  chatD({ type: "START_SEND" });
  chatD({ type: "ADD_PAIR", userMsg, asstMsg });
  pushBrowserTrace(makeBrowserTraceEvent(
    browserTraceSummary(slashBrowserCommand.command, "start"),
    { command: slashBrowserCommand.command, args: slashBrowserCommand.args, action: builtAction.action },
    "tool_start",
  ));

  try {
    const result = await browserIntentClient.extensionAction(builtAction.action);
    if (!result?.ok) {
      throw new Error(result?.error || "Browser action failed.");
    }
    const artifact = summarizeSlashBrowserResult(String(builtAction.action.type), result);
    const content = formatSlashBrowserResponse(slashBrowserCommand, artifact, result);
    chatD({ type: "UPDATE_LAST", updates: { content, browserSummary: artifact } });
    setResumePromptForBrowser(null);
    setLastArtifact(artifact);
    setBridgeNotice({ tone: "success", text: browserTraceSummary(slashBrowserCommand.command, "success") });
    pushBrowserTrace(makeBrowserTraceEvent(
      browserTraceSummary(slashBrowserCommand.command, "success"),
      { command: slashBrowserCommand.command, args: slashBrowserCommand.args, result },
      "tool_result",
    ));
    syncBridgeState();
  } catch (e: unknown) {
    const message = friendlyError((e as Error).message || "Browser action failed.");
    chatD({ type: "ERROR_LAST", error: message });
    setBridgeNotice({ tone: "error", text: message });
    pushBrowserTrace(makeBrowserTraceEvent(
      browserTraceSummary(slashBrowserCommand.command, "error"),
      { command: slashBrowserCommand.command, args: slashBrowserCommand.args, error: message },
      "reflect",
    ));
  } finally {
    chatD({ type: "FINISH_SEND" });
  }
  return true;
}

/**
 * runSocialPublish handles natural-language "publish to <platform>" intents by
 * executing the multi-step browser publish plan. Returns true when a social
 * publish intent was detected (handled here), false otherwise.
 */
export async function runSocialPublish(
  ctx: ChatBrowserActionContext,
  displayText: string,
  text: string,
): Promise<boolean> {
  const socialPublishIntent = detectSocialPublishIntent(displayText);
  if (!socialPublishIntent) return false;

  const {
    browserIntentClient, chatD, pushBrowserTrace, syncBridgeState,
    setBridgeNotice, setLastArtifact, setSuggestedTab, setShowComputer,
    setShowConnectors, setResumePromptForBrowser, setActiveSlashCommand,
    setShowSlashMenu,
  } = ctx;

  setSuggestedTab("browser");
  setShowComputer(true);
  const extStatus = await browserIntentClient.extensionStatus().catch(() => ({ connected: false }));
  if (!extStatus.connected) {
    setShowConnectors(true);
    setResumePromptForBrowser(text);
    setBridgeNotice({ tone: "warning", text: `${socialPublishIntent.platformName}直发需要先连接浏览器。` });
    pushBrowserTrace(makeBrowserTraceEvent(
      "Social publish waiting for browser connector",
      { platform: socialPublishIntent.platform, scenario: socialPublishIntent.scenarioId },
      "reflect",
    ));
    const userMsg: Message = { role: "user", content: displayText, id: newId(), timestamp: Date.now() };
    const asstMsg: Message = {
      role: "assistant",
      content: formatSocialPublishConnectorRequired(socialPublishIntent),
      id: newId(),
      browserRequirement: {
        required: true,
        reason: "browser_connector_required",
        message: `${socialPublishIntent.platformName}直发需要连接 Yunque Browser Connector，才能在你的真实登录会话中打开页面、填写内容并点击发布。`,
        install_path: "/packs/browser",
        settings_path: "/packs/browser",
      },
      traceEvents: [makeBrowserTraceEvent("Opened browser install guide", { source: "chat-social-publish", platform: socialPublishIntent.platform }, "reflect")],
    };
    chatD({ type: "SET_INPUT", value: "" });
    chatD({ type: "ADD_PAIR", userMsg, asstMsg });
    setActiveSlashCommand(null);
    setShowSlashMenu(false);
    if (typeof window !== "undefined") {
      window.setTimeout(() => window.open("/packs/browser", "_blank", "noopener,noreferrer"), 80);
    }
    return true;
  }

  const actions = buildSocialPublishActions(socialPublishIntent);
  const userMsg: Message = { role: "user", content: displayText, id: newId(), timestamp: Date.now() };
  const asstMsg: Message = { role: "assistant", content: "", id: newId(), timestamp: Date.now(), traceEvents: [] };
  setActiveSlashCommand(null);
  setShowSlashMenu(false);
  chatD({ type: "START_SEND" });
  chatD({ type: "ADD_PAIR", userMsg, asstMsg });
  const startEvent = makeBrowserTraceEvent(
    `${socialPublishIntent.platformName}直发计划开始`,
    {
      source: "chat-social-publish",
      platform: socialPublishIntent.platform,
      scenario: socialPublishIntent.scenarioId,
      targetUrl: socialPublishIntent.targetUrl,
      steps: actions.map((action, index) => ({ index, label: socialPublishStepLabel(action, socialPublishIntent), type: action.type })),
    },
    "tool_start",
  );
  pushBrowserTrace(startEvent);
  chatD({ type: "APPEND_LAST_TRACE", event: startEvent });

  const stepRecords: SocialPublishStepRecord[] = [];
  try {
    for (const [index, action] of actions.entries()) {
      const label = socialPublishStepLabel(action, socialPublishIntent);
      const stepStart = makeBrowserTraceEvent(
        label,
        { source: "chat-social-publish", platform: socialPublishIntent.platform, action },
        "tool_start",
      );
      pushBrowserTrace(stepStart);
      chatD({ type: "APPEND_LAST_TRACE", event: stepStart });

      const result = await browserIntentClient.extensionAction(action);
      const record = stepRecordFromResult(index, label, action, result);
      stepRecords.push(record);
      if (!result?.ok) {
        throw new Error(result?.error || `${label}失败`);
      }

      const stepDone = makeBrowserTraceEvent(
        `${label}完成`,
        { source: "chat-social-publish", platform: socialPublishIntent.platform, result: record },
        "tool_result",
      );
      pushBrowserTrace(stepDone);
      chatD({ type: "APPEND_LAST_TRACE", event: stepDone });
    }

    const lastStep = stepRecords[stepRecords.length - 1];
    const artifact: BrowserActionArtifactSummary = {
      action: `${socialPublishIntent.platform}_publish_direct`,
      url: lastStep?.url || socialPublishIntent.targetUrl,
      title: socialPublishIntent.title || `${socialPublishIntent.platformName}直发`,
      hasScreenshot: stepRecords.some((step) => step.hasScreenshot),
      preview: socialPublishIntent.body.slice(0, 240),
      updatedAt: Date.now(),
    };
    const content = formatSocialPublishResult(socialPublishIntent, stepRecords);
    chatD({ type: "UPDATE_LAST", updates: { content, browserSummary: artifact } });
    setResumePromptForBrowser(null);
    setLastArtifact(artifact);
    setBridgeNotice({ tone: "success", text: `${socialPublishIntent.platformName}直发已完成。` });
    pushBrowserTrace(makeBrowserTraceEvent(
      `${socialPublishIntent.platformName}直发完成`,
      { source: "chat-social-publish", platform: socialPublishIntent.platform, steps: stepRecords },
      "tool_result",
    ));
    syncBridgeState();
  } catch (e: unknown) {
    const message = friendlyError((e as Error).message || `${socialPublishIntent.platformName}直发失败。`);
    const lastStep = stepRecords[stepRecords.length - 1];
    const artifact: BrowserActionArtifactSummary = {
      action: `${socialPublishIntent.platform}_publish_direct`,
      url: lastStep?.url || socialPublishIntent.targetUrl,
      title: socialPublishIntent.title || `${socialPublishIntent.platformName}直发`,
      hasScreenshot: stepRecords.some((step) => step.hasScreenshot),
      preview: socialPublishIntent.body.slice(0, 240),
      updatedAt: Date.now(),
    };
    chatD({ type: "UPDATE_LAST", updates: { content: formatSocialPublishResult(socialPublishIntent, stepRecords, message), browserSummary: artifact } });
    setLastArtifact(artifact);
    setBridgeNotice({ tone: "error", text: message });
    pushBrowserTrace(makeBrowserTraceEvent(
      `${socialPublishIntent.platformName}直发被中断`,
      { source: "chat-social-publish", platform: socialPublishIntent.platform, error: message, steps: stepRecords },
      "reflect",
    ));
  } finally {
    chatD({ type: "FINISH_SEND" });
  }
  return true;
}
