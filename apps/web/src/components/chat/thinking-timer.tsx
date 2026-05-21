"use client";

import { useEffect, useState } from "react";

export function ThinkingTimer({
  startMs,
  endMs,
  isStreaming,
}: {
  startMs?: number;
  endMs?: number;
  isStreaming: boolean;
}) {
  const [elapsed, setElapsed] = useState(0);
  useEffect(() => {
    if (!startMs) return;
    if (endMs) {
      setElapsed((endMs - startMs) / 1000);
      return;
    }
    if (!isStreaming) return;
    const tick = () => setElapsed((Date.now() - startMs) / 1000);
    tick();
    const id = setInterval(tick, 100);
    return () => clearInterval(id);
  }, [startMs, endMs, isStreaming]);
  if (!startMs || elapsed <= 0) return null;
  return (
    <span style={{ color: "var(--yunque-text-muted)", fontSize: "var(--text-xs)" }}>
      （用时 {elapsed.toFixed(1)} 秒）
    </span>
  );
}

export default ThinkingTimer;
