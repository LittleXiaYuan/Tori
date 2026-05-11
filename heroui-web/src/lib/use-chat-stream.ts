import { useEffect, useRef } from "react";
import type { AgentEvent } from "@/components/execution-trace";

interface UseChatStreamOptions {
  onTraceEvent: (event: AgentEvent) => void;
  onShouldOpenComputer: () => void;
}

export function useChatStream({ onTraceEvent, onShouldOpenComputer }: UseChatStreamOptions): void {
  const onTraceRef = useRef(onTraceEvent);
  onTraceRef.current = onTraceEvent;
  const onOpenRef = useRef(onShouldOpenComputer);
  onOpenRef.current = onShouldOpenComputer;

  useEffect(() => {
    let cancelled = false;
    const token = typeof window !== "undefined" ? localStorage.getItem("yunque_token") || "" : "";
    const key = typeof window !== "undefined" ? localStorage.getItem("yunque_api_key") || "" : "";
    const headers: Record<string, string> = {};
    if (token) headers["Authorization"] = `Bearer ${token}`;
    else if (key) headers["X-API-Key"] = key;

    (async () => {
      try {
        const res = await fetch(`/v1/events/stream`, { headers });
        if (!res.ok || !res.body) return;
        const reader = res.body.getReader();
        const decoder = new TextDecoder();
        let buf = "";
        while (!cancelled) {
          const { done, value } = await reader.read();
          if (done) break;
          buf += decoder.decode(value, { stream: true });
          const lines = buf.split("\n");
          buf = lines.pop() || "";
          for (const line of lines) {
            if (line.startsWith("data: ")) {
              try {
                const evt: AgentEvent = JSON.parse(line.slice(6));
                onTraceRef.current(evt);
                const evtType = (evt.type || "").toLowerCase();
                if (evtType === "tool_start" || evtType === "tool_result" || evtType === "thinking" || evtType === "handoff_start") {
                  onOpenRef.current();
                }
              } catch { /* ignore parse */ }
            }
          }
        }
      } catch (e) {
        console.warn("[chat] SSE connection failed, trace events unavailable:", e);
      }
    })();
    return () => { cancelled = true; };
  }, []);
}
