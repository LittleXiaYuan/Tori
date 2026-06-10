"use client";

import { useEffect, useRef, useState } from "react";
import { Zap, Clock, DollarSign } from "lucide-react";

export interface ResourceSnapshot {
  tokensIn: number;
  tokensOut: number;
  costUsd: number;
  startMs: number;
  endMs?: number;
}

function useElapsed(startMs: number | undefined, endMs: number | undefined, isLive: boolean): number {
  const [now, setNow] = useState(Date.now());
  const rafRef = useRef<ReturnType<typeof setInterval> | undefined>(undefined);

  useEffect(() => {
    if (!isLive || !startMs) {
      if (rafRef.current) clearInterval(rafRef.current);
      return;
    }
    rafRef.current = setInterval(() => setNow(Date.now()), 250);
    return () => { if (rafRef.current) clearInterval(rafRef.current); };
  }, [isLive, startMs]);

  if (!startMs) return 0;
  if (endMs) return endMs - startMs;
  if (isLive) return now - startMs;
  return 0;
}

function formatDuration(ms: number): string {
  if (ms < 1000) return `${ms}ms`;
  const s = ms / 1000;
  if (s < 60) return `${s.toFixed(1)}s`;
  const m = Math.floor(s / 60);
  return `${m}m ${Math.floor(s % 60)}s`;
}

export interface TaskResourceMeterProps {
  snapshot: ResourceSnapshot | null;
  prevSnapshot?: ResourceSnapshot | null;
  isLive: boolean;
  costThresholdUsd?: number;
}

/** Compact single-row resource strip (Tokens · Time · Cost). Replaces the three
 *  tall stat cards — same data, far lower visual density. */
export function TaskResourceMeter({ snapshot, isLive }: TaskResourceMeterProps) {
  const tokensTotal = (snapshot?.tokensIn ?? 0) + (snapshot?.tokensOut ?? 0);
  const costUsd = snapshot?.costUsd ?? 0;
  const elapsed = useElapsed(snapshot?.startMs, snapshot?.endMs, isLive);

  if (!snapshot && !isLive) return null;

  return (
    <div className="task-meter" style={{ opacity: isLive ? 1 : 0.65 }}>
      <span className="task-meter__item">
        <Zap size={11} /> {tokensTotal.toLocaleString()}
        {isLive && tokensTotal > 0 && <i className="task-meter__pulse" />}
      </span>
      <span className="task-meter__sep" />
      <span className="task-meter__item">
        <Clock size={11} /> {formatDuration(elapsed)}
      </span>
      <span className="task-meter__sep" />
      <span className="task-meter__item">
        <DollarSign size={11} /> {costUsd > 0 ? `$${costUsd.toFixed(4)}` : "—"}
      </span>
    </div>
  );
}

export default TaskResourceMeter;
