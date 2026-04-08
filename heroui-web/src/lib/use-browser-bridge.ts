"use client";

import { useCallback, useEffect, useState } from "react";
import type { BrowserBridgeState, BrowserSessionNotice } from "@/components/browser-session-card";

interface UseBrowserBridgeOptions {
  onActionStart?: (type: string, extra: Record<string, unknown>) => void;
  onActionSuccess?: (type: string | undefined, result: unknown, successText: string) => void;
  onActionError?: (type: string | undefined, payload: unknown, message: string) => void;
}

function actionSuccessText(action: string | undefined) {
  return action === "bridge/switch-to-tab"
    ? "????????????"
    : action === "bridge/takeover"
      ? "??????????"
      : action === "bridge/resume"
        ? "???????"
        : "????????";
}

export function useBrowserBridge(options: UseBrowserBridgeOptions = {}) {
  const { onActionStart, onActionSuccess, onActionError } = options;
  const [bridgeState, setBridgeState] = useState<BrowserBridgeState | null>(null);
  const [bridgeActionPending, setBridgeActionPending] = useState<string | null>(null);
  const [bridgeNotice, setBridgeNotice] = useState<BrowserSessionNotice | null>(null);

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
    setBridgeNotice({ tone: "info", text: "??????????" });
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
          const message = result?.error || "???????";
          setBridgeNotice({ tone: "error", text: message });
          onActionError?.(action, result, message);
        } else {
          const successText = actionSuccessText(action);
          setBridgeNotice({ tone: action === "bridge/takeover" ? "warning" : "success", text: successText });
          onActionSuccess?.(action, result, successText);
        }
        return;
      }

      if (data.type === "bridge/error") {
        setBridgeActionPending(null);
        const message = data.payload?.error || "???????";
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
    sendBridgeAction,
    syncBridgeState,
    setBridgeNotice,
  };
}

export default useBrowserBridge;
