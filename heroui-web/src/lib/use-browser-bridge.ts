"use client";

import { useCallback, useEffect, useState } from "react";
import type { BrowserActionArtifactSummary, BrowserBridgeState, BrowserSessionNotice } from "@/components/browser-session-card";
import { browserActionLabel } from "@/lib/browser-action-labels";

function buildArtifactPreview(content: unknown) {
  if (typeof content !== "string") return "";
  const normalized = content.replace(/\s+/g, " ").trim();
  if (!normalized) return "";
  return normalized.length > 220 ? `${normalized.slice(0, 220)}...` : normalized;
}

function suggestNextBrowserAction(action: string | undefined, result: any): Pick<BrowserActionArtifactSummary, "suggestedCommand" | "suggestedLabel"> {
  const normalized = action || "";
  if (normalized.includes("mark") || typeof result?.total === "number") {
    return { suggestedCommand: "/click ", suggestedLabel: "Click a marked element" };
  }
  if (normalized.includes("navigate")) {
    return { suggestedCommand: "/content", suggestedLabel: "Read this page" };
  }
  if (normalized.includes("click") || normalized.includes("resume")) {
    return { suggestedCommand: "/content", suggestedLabel: "Inspect current page" };
  }
  if (normalized.includes("content")) {
    return { suggestedCommand: "/mark", suggestedLabel: "Mark interactive elements" };
  }
  return {};
}

interface UseBrowserBridgeOptions {
  onActionStart?: (type: string, extra: Record<string, unknown>) => void;
  onActionSuccess?: (type: string | undefined, result: unknown, successText: string) => void;
  onActionError?: (type: string | undefined, payload: unknown, message: string) => void;
}

function summarizeActionArtifact(action: string | undefined, result: any): BrowserActionArtifactSummary | null {
  if (!result || typeof result !== "object") return null;
  return {
    action,
    url: result.url || result.currentUrl || result.state?.runtimeSession?.currentUrl || "",
    title: result.title || result.state?.runtimeSession?.title || "",
    elementCount: typeof result.total === "number" ? result.total : Array.isArray(result.elements) ? result.elements.length : undefined,
    tabId: result.tabId ?? result.state?.runtimeSession?.currentTabId ?? null,
    hasScreenshot: Boolean(result.screenshot),
    textLength: typeof result.content === "string" ? result.content.length : 0,
    preview: buildArtifactPreview(result.content),
    ...suggestNextBrowserAction(action, result),
    updatedAt: Date.now(),
  };
}

function actionSuccessText(action: string | undefined) {
  if (action === "bridge/switch-to-tab") return `${browserActionLabel(action)}.`;
  if (action === "bridge/takeover") return "Browser handed over to you.";
  if (action === "bridge/resume") return "Browser run resumed.";
  return `${browserActionLabel(action)} completed.`;
}

export function useBrowserBridge(options: UseBrowserBridgeOptions = {}) {
  const { onActionStart, onActionSuccess, onActionError } = options;
  const [bridgeState, setBridgeState] = useState<BrowserBridgeState | null>(null);
  const [bridgeActionPending, setBridgeActionPending] = useState<string | null>(null);
  const [bridgeNotice, setBridgeNotice] = useState<BrowserSessionNotice | null>(null);
  const [lastArtifact, setLastArtifact] = useState<BrowserActionArtifactSummary | null>(null);

  const postBridgeMessage = useCallback((type: string, extra: Record<string, unknown> = {}, requestId?: string) => {
    if (typeof window === "undefined") return;
    window.postMessage({
      source: "yunque-app",
      type,
      requestId: requestId || `bridge-${Date.now()}-${Math.random().toString(36).slice(2, 8)}`,
      ...extra,
    }, "*");
  }, []);

  const sendBridgeAction = useCallback((type: string, extra: Record<string, unknown> = {}) => {
    if (typeof window === "undefined") return;
    setBridgeActionPending(type);
    setBridgeNotice({ tone: "info", text: `${browserActionLabel(type)}...` });
    onActionStart?.(type, extra);
    postBridgeMessage(type, extra, `bridge-action-${Date.now()}-${Math.random().toString(36).slice(2, 8)}`);
  }, [onActionStart, postBridgeMessage]);

  const syncBridgeState = useCallback(() => {
    postBridgeMessage("bridge/ping");
    postBridgeMessage("bridge/get-state");
  }, [postBridgeMessage]);

  useEffect(() => {
    if (typeof window === "undefined") return;

    const onMessage = (event: MessageEvent) => {
      if (event.source !== window) return;
      const data = event.data;
      if (!data || data.source !== "yunque-bridge") return;

      if (data.type === "bridge/ready" || data.type === "bridge/state" || data.type === "bridge/state-update") {
        setBridgeState(data.payload?.state || null);
        return;
      }

      if (data.type === "bridge/action-result") {
        setBridgeActionPending(null);
        const result = data.payload?.result;
        const action = data.payload?.action as string | undefined;
        if (result?.state) setBridgeState(result.state);
        if (result?.ok === false || result?.error) {
          const message = result?.error || "Browser action failed.";
          setBridgeNotice({ tone: "error", text: message });
          onActionError?.(action, result, message);
        } else {
          const successText = actionSuccessText(action);
          setBridgeNotice({ tone: action === "bridge/takeover" ? "warning" : "success", text: successText });
          setLastArtifact(summarizeActionArtifact(action, result));
          onActionSuccess?.(action, result, successText);
        }
        return;
      }

      if (data.type === "bridge/error") {
        setBridgeActionPending(null);
        const message = data.payload?.error || "Browser action failed.";
        setBridgeNotice({ tone: "error", text: message });
        onActionError?.(undefined, data.payload, message);
      }
    };

    const onVisibility = () => {
      if (!document.hidden) syncBridgeState();
    };

    window.addEventListener("message", onMessage);
    window.addEventListener("focus", syncBridgeState);
    document.addEventListener("visibilitychange", onVisibility);
    syncBridgeState();

    return () => {
      window.removeEventListener("message", onMessage);
      window.removeEventListener("focus", syncBridgeState);
      document.removeEventListener("visibilitychange", onVisibility);
    };
  }, [onActionError, onActionSuccess, syncBridgeState]);

  useEffect(() => {
    if (!bridgeNotice) return;
    const timer = window.setTimeout(() => setBridgeNotice(null), bridgeNotice.tone === "error" ? 5000 : 3200);
    return () => window.clearTimeout(timer);
  }, [bridgeNotice]);

  return {
    bridgeState,
    bridgeActionPending,
    bridgeNotice,
    lastArtifact,
    sendBridgeAction,
    syncBridgeState,
    setBridgeNotice,
    setLastArtifact,
  };
}

export default useBrowserBridge;
