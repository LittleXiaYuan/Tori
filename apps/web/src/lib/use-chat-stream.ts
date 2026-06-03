import { useEffect, useRef } from "react";
import type { AgentEvent } from "@/components/execution-trace";
import { BASE, getAuthHeaders } from "./api-core";

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

    (async () => {
      try {
        // Use the shared API base (NEXT_PUBLIC_API_BASE) + auth headers so the
        // SSE stream hits the Go backend (:9090) just like every other /v1 call.
        // A bare relative `/v1/events/stream` went to the Next dev server (:3001)
        // which doesn't serve it → "Failed to fetch" + an empty trace panel.
        const res = await fetch(`${BASE}/v1/events/stream`, { headers: getAuthHeaders() });
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
