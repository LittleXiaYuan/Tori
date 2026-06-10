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
    // CRITICAL: drive the stream's lifetime with an AbortController. The old
    // implementation only flipped a `cancelled` flag that was checked *after*
    // `reader.read()`, which blocks while the SSE is idle — so the underlying
    // connection never closed on unmount/re-run. Those leaked connections piled
    // up against the browser's ~6-per-origin limit and starved every other
    // request (including /healthz), which surfaced as "本地服务超时" that only a
    // full restart could clear.
    const controller = new AbortController();
    let reader: ReadableStreamDefaultReader<Uint8Array> | null = null;

    (async () => {
      try {
        const res = await fetch(`${BASE}/v1/events/stream`, {
          headers: getAuthHeaders(),
          signal: controller.signal,
        });
        if (!res.ok || !res.body) return;
        reader = res.body.getReader();
        const decoder = new TextDecoder();
        let buf = "";
        while (true) {
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
        if (!controller.signal.aborted) {
          console.warn("[chat] SSE connection failed, trace events unavailable:", e);
        }
      }
    })();

    return () => {
      controller.abort();
      reader?.cancel().catch(() => {});
    };
  }, []);
}
