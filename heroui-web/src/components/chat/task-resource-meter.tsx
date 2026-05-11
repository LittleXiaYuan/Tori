"use client";

import { useEffect, useRef, useState } from "react";
import { Zap, Clock, DollarSign, TrendingUp, TrendingDown } from "lucide-react";
import { useI18n } from "@/lib/i18n";

export interface ResourceSnapshot {
  tokensIn: number;
  tokensOut: number;
  costUsd: number;
  startMs: number;
  endMs?: number;
}

interface MeterCardProps {
  icon: React.ReactNode;
  label: string;
  value: string;
  sub?: string;
  accent: string;
  bar?: { value: number; max: number; color: string };
  change?: { direction: "up" | "down"; pct: number };
  pulse?: boolean;
}

function MeterCard({ icon, label, value, sub, accent, bar, change, pulse }: MeterCardProps) {
  return (
    <div
      className="flex-1 min-w-0 rounded-lg px-3 py-2"
      style={{ background: "var(--yunque-bg-muted)", border: "1px solid var(--yunque-border)" }}
    >
      <div className="flex items-center gap-1.5 mb-1">
        <span style={{ color: accent, display: "flex", filter: `drop-shadow(0 0 3px ${accent})` }}>{icon}</span>
        <span className="text-[10px] font-medium truncate" style={{ color: "var(--yunque-text-muted)" }}>{label}</span>
        {pulse && <span className="w-1.5 h-1.5 rounded-full animate-pulse" style={{ background: accent }} />}
      </div>
      <div className="text-base font-bold font-mono" style={{ color: "var(--yunque-text)", fontVariantNumeric: "tabular-nums" }}>
        {value}
      </div>
      <div className="flex items-center gap-2 mt-0.5">
        {sub && <span className="text-[10px] truncate" style={{ color: "var(--yunque-text-muted)" }}>{sub}</span>}
        {change && change.pct !== 0 && (
          <span className="text-[10px] font-semibold flex items-center gap-0.5 ml-auto shrink-0" style={{ color: change.direction === "up" ? "var(--yunque-danger)" : "var(--yunque-success)" }}>
            {change.direction === "up" ? <TrendingUp size={9} /> : <TrendingDown size={9} />}
            {change.direction === "up" ? "+" : "-"}{change.pct.toFixed(0)}%
          </span>
        )}
      </div>
      {bar && (
        <div className="mt-1.5 h-1 rounded-full overflow-hidden" style={{ background: "var(--yunque-border)" }}>
          <div
            className="h-full rounded-full transition-all duration-500"
            style={{ width: `${bar.max > 0 ? Math.min((bar.value / bar.max) * 100, 100) : 0}%`, background: bar.color }}
          />
        </div>
      )}
    </div>
  );
}

function useElapsed(startMs: number | undefined, endMs: number | undefined, isLive: boolean): number {
  const [now, setNow] = useState(Date.now());
  const rafRef = useRef<ReturnType<typeof setInterval> | undefined>(undefined);

  useEffect(() => {
    if (!isLive || !startMs) {
      if (rafRef.current) clearInterval(rafRef.current);
      return;
    }
    rafRef.current = setInterval(() => setNow(Date.now()), 200);
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

function durationColor(ms: number): string {
  if (ms < 5000) return "var(--yunque-success)";
  if (ms < 30000) return "var(--yunque-warning)";
  return "var(--yunque-danger)";
}

export interface TaskResourceMeterProps {
  snapshot: ResourceSnapshot | null;
  prevSnapshot?: ResourceSnapshot | null;
  isLive: boolean;
  costThresholdUsd?: number;
}

export function TaskResourceMeter({ snapshot, prevSnapshot, isLive, costThresholdUsd = 0.05 }: TaskResourceMeterProps) {
  const { t } = useI18n();

  const tokensIn = snapshot?.tokensIn ?? 0;
  const tokensOut = snapshot?.tokensOut ?? 0;
  const tokensTotal = tokensIn + tokensOut;
  const costUsd = snapshot?.costUsd ?? 0;

  const elapsed = useElapsed(snapshot?.startMs, snapshot?.endMs, isLive);
  const durColor = durationColor(elapsed);

  const prevCost = prevSnapshot?.costUsd ?? 0;
  const costChange = prevCost > 0 ? ((costUsd - prevCost) / prevCost) * 100 : 0;
  const costAccent = costUsd > costThresholdUsd ? "var(--yunque-warning)" : "var(--yunque-success)";

  if (!snapshot && !isLive) return null;

  return (
    <div
      className="flex gap-2 flex-wrap"
      style={{ opacity: isLive ? 1 : 0.7, transition: "opacity 0.3s ease" }}
    >
      <MeterCard
        icon={<Zap size={12} />}
        label="Tokens"
        value={tokensTotal.toLocaleString()}
        sub={`in:${tokensIn.toLocaleString()} / out:${tokensOut.toLocaleString()}`}
        accent="var(--yunque-accent)"
        bar={tokensTotal > 0 ? { value: tokensOut, max: tokensTotal, color: "var(--yunque-success)" } : undefined}
        pulse={isLive && tokensTotal > 0}
      />
      <MeterCard
        icon={<Clock size={12} />}
        label={t("chat.time") || "Time"}
        value={formatDuration(elapsed)}
        accent={durColor}
        bar={{ value: Math.min(elapsed, 60000), max: 60000, color: durColor }}
        pulse={isLive}
      />
      <MeterCard
        icon={<DollarSign size={12} />}
        label={t("chat.cost") || "Cost"}
        value={costUsd > 0 ? `$${costUsd.toFixed(4)}` : "—"}
        accent={costAccent}
        change={Math.abs(costChange) > 1 ? { direction: costChange > 0 ? "up" : "down", pct: Math.abs(costChange) } : undefined}
      />
    </div>
  );
}

export default TaskResourceMeter;
