import type { BrowserActionArtifactSummary } from "@/components/browser-session-card";
import { browserActionLabel } from "@/lib/browser-action-labels";

/** UI state for the slash-command menu. */
export function getSlashState(input: string): { visible: boolean; query: string } {
  const trimmed = input.trimStart();
  if (!trimmed.startsWith("/")) return { visible: false, query: "" };
  const firstLine = trimmed.split("\n")[0];
  const commandPart = firstLine.slice(1);
  if (commandPart.includes(" ")) return { visible: false, query: "" };
  return { visible: true, query: commandPart };
}

/** Returns the active slash command name (without "/"), or null if none. */
export function getActiveSlashCommand(input: string): string | null {
  const trimmed = input.trimStart();
  if (!trimmed.startsWith("/")) return null;
  const firstLine = trimmed.split("\n")[0];
  const match = firstLine.match(/^\/([^\s]+)/);
  return match?.[1] || null;
}

/**
 * Map an opaque browser summary blob from the backend into the artifact
 * shape consumed by BrowserSessionCard.
 */
export function mapBrowserSummary(
  summary: unknown,
): BrowserActionArtifactSummary {
  const s = (summary ?? {}) as Record<string, unknown>;
  return {
    action: s.skill as string | undefined,
    url: s.url as string | undefined,
    title: s.title as string | undefined,
    elementCount: s.element_count as number | undefined,
    tabId: s.tab_id as number | null | undefined,
    hasScreenshot: s.has_screenshot as boolean | undefined,
    textLength: s.text_length as number | undefined,
    preview: s.preview as string | undefined,
    suggestedCommand: s.next_command as string | undefined,
    suggestedLabel: s.next_label as string | undefined,
    updatedAt: Date.now(),
  };
}

export interface ParsedSlashBrowserCommand {
  command: string;
  args: string;
  summary: string;
}

/** Parse a user slash command. Returns null if it's not a recognised browser command. */
export function parseSlashBrowserCommand(input: string): ParsedSlashBrowserCommand | null {
  const trimmed = input.trim();
  if (!trimmed.startsWith("/")) return null;
  const [cmdRaw, ...restParts] = trimmed.split(/\s+/);
  const cmd = cmdRaw.toLowerCase();
  const args = restParts.join(" ").trim();
  const browserCommands: Record<string, { summary: string }> = {
    "/navigate": { summary: args ? `Open page: ${args}` : "Open page" },
    "/screenshot": { summary: "Capture page" },
    "/content": { summary: "Read page content" },
    "/mark": { summary: "Mark page elements" },
    "/unmark": { summary: "Clear marked elements" },
    "/scroll": { summary: args ? `Scroll page: ${args}` : "Scroll page" },
    "/click": { summary: args ? `Click target: ${args}` : "Click element" },
    "/type": { summary: args ? `Type input: ${args.slice(0, 32)}` : "Type input" },
  };
  if (!browserCommands[cmd]) return null;
  return { command: cmd, args, ...browserCommands[cmd] };
}

export function normalizeBrowserUrl(raw: string): string {
  const value = raw.trim();
  if (!value) return "";
  if (/^https?:\/\//i.test(value)) return value;
  return `https://${value}`;
}

export type SlashBrowserActionResult =
  | { action: Record<string, unknown>; error?: undefined }
  | { error: string; action?: undefined };

/**
 * Translate a parsed slash command into the action payload expected by the
 * browser bridge, or an error string with correct usage hints.
 */
export function buildSlashBrowserAction(command: {
  command: string;
  args: string;
}): SlashBrowserActionResult {
  switch (command.command) {
    case "/navigate": {
      const url = normalizeBrowserUrl(command.args);
      if (!url) return { error: "Usage: /navigate https://example.com" };
      return { action: { type: "browser_navigate", url } };
    }
    case "/screenshot":
      return { action: { type: "browser_screenshot" } };
    case "/content":
      return { action: { type: "browser_get_content" } };
    case "/mark":
      return { action: { type: "browser_mark_elements" } };
    case "/unmark":
      return { action: { type: "browser_unmark_elements" } };
    case "/scroll": {
      const value = command.args.trim().toLowerCase();
      if (!value) return { action: { type: "browser_scroll", direction: "down" } };
      if (value === "top")
        return { action: { type: "browser_scroll", direction: "up", to_end: true } };
      if (value === "bottom")
        return { action: { type: "browser_scroll", direction: "down", to_end: true } };
      if (["up", "down", "left", "right"].includes(value)) {
        return { action: { type: "browser_scroll", direction: value } };
      }
      return { error: "Usage: /scroll up|down|left|right|top|bottom" };
    }
    case "/click": {
      const value = command.args.trim();
      if (!value) return { error: "Usage: /click 3 or /click .selector" };
      if (/^\d+$/.test(value)) {
        return {
          action: {
            type: "browser_click",
            target: { strategy: "byIndex", index: Number(value) },
          },
        };
      }
      if (/^[#.\[]|^[a-z]+[\w\-.:#\[\]\(\)="']*$/i.test(value)) {
        return {
          action: {
            type: "browser_click",
            target: { strategy: "bySelector", selector: value },
          },
        };
      }
      return {
        error:
          "Use /mark first, then /click <number>. CSS selectors are also supported.",
      };
    }
    case "/type": {
      const value = command.args.trim();
      if (!value)
        return { error: "Usage: /type your text or /type <selector> => <text>" };
      const selectorMatch = value.match(/^(.*?)\s*(?:=>|::)\s*([\s\S]+)$/);
      if (selectorMatch) {
        const selector = selectorMatch[1].trim();
        const text = selectorMatch[2].trim();
        if (!selector || !text) return { error: "Usage: /type <selector> => <text>" };
        return {
          action: {
            type: "browser_input",
            target: { strategy: "bySelector", selector },
            text,
          },
        };
      }
      return { action: { type: "browser_input", text: value } };
    }
    default:
      return { error: "Unsupported browser command." };
  }
}

export function summarizeSlashBrowserResult(
  actionType: string,
  result: unknown,
): BrowserActionArtifactSummary {
  const r = (result ?? {}) as Record<string, unknown>;
  const rs = (r.state as Record<string, unknown> | undefined)?.runtimeSession as
    | Record<string, unknown>
    | undefined;
  const content = typeof r.content === "string" ? r.content : "";
  const preview = content.replace(/\s+/g, " ").trim();
  return {
    action: actionType,
    url: (r.url as string) || (r.currentUrl as string) || (rs?.currentUrl as string) || "",
    title: (r.title as string) || (rs?.title as string) || "",
    elementCount:
      typeof r.total === "number"
        ? (r.total as number)
        : Array.isArray(r.elements)
          ? (r.elements as unknown[]).length
          : undefined,
    tabId:
      (r.tabId as number | null | undefined) ??
      (rs?.currentTabId as number | null | undefined) ??
      null,
    hasScreenshot: Boolean(r.screenshot),
    textLength: content.length,
    preview: preview
      ? preview.length > 240
        ? `${preview.slice(0, 240)}...`
        : preview
      : "",
    suggestedCommand:
      actionType === "browser_navigate"
        ? "/content"
        : actionType === "browser_mark_elements"
          ? "/click "
          : actionType === "browser_get_content"
            ? "/mark"
            : undefined,
    suggestedLabel:
      actionType === "browser_navigate"
        ? "Read this page"
        : actionType === "browser_mark_elements"
          ? "Click a marked element"
          : actionType === "browser_get_content"
            ? "Mark interactive elements"
            : undefined,
    updatedAt: Date.now(),
  };
}

export function formatSlashBrowserResponse(
  command: { command: string; args: string },
  artifact: BrowserActionArtifactSummary,
  result: unknown,
): string {
  const lines: string[] = [];
  lines.push(`**${browserActionLabel(artifact.action)}** completed.`);
  if (artifact.title) lines.push(`- Title: ${artifact.title}`);
  if (artifact.url) lines.push(`- URL: ${artifact.url}`);
  if (typeof artifact.elementCount === "number")
    lines.push(`- Elements: ${artifact.elementCount}`);
  if (artifact.textLength) lines.push(`- Content length: ${artifact.textLength} chars`);
  if (artifact.preview) {
    lines.push("");
    lines.push(artifact.preview);
  }
  if (artifact.hasScreenshot) {
    lines.push("");
    lines.push("A fresh screenshot was captured in the browser panel.");
  }
  if (
    command.command === "/click" &&
    !artifact.preview &&
    !artifact.title &&
    !artifact.url
  ) {
    lines.push("");
    lines.push("Tip: use `/content` or `/screenshot` next to inspect the result.");
  }
  const errMsg = (result as { error?: string } | null)?.error;
  if (errMsg) {
    lines.push("");
    lines.push(`Warning: ${errMsg}`);
  }
  return lines.join("\n");
}
