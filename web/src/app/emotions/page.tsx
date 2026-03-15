"use client";

import { useEffect, useState, useCallback } from "react";
import { api, type EmotionHistoryEntry } from "@/lib/api";
import { BlurFade } from "@/components/ui/blur-fade";
import { SmilePlus, RefreshCw } from "lucide-react";

const emotionColors: Record<string, string> = {
  happy: "#facc15",
  sad: "#60a5fa",
  angry: "#f87171",
  neutral: "#a1a1aa",
  surprised: "#c084fc",
  fearful: "#fb923c",
  disgusted: "#86efac",
  loving: "#f472b6",
};

const emotionEmoji: Record<string, string> = {
  happy: "😊",
  sad: "😢",
  angry: "😠",
  neutral: "😐",
  surprised: "😮",
  fearful: "😰",
  disgusted: "🤢",
  loving: "🥰",
};

export default function EmotionsPage() {
  const [entries, setEntries] = useState<EmotionHistoryEntry[]>([]);
  const [summary, setSummary] = useState<Record<string, number>>({});
  const [total, setTotal] = useState(0);
  const [loading, setLoading] = useState(true);
  const [limit, setLimit] = useState(200);
  const [sessionFilter, setSessionFilter] = useState("");

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const res = await api.getEmotionHistory({
        session_id: sessionFilter || undefined,
        limit,
      });
      setEntries(res.entries || []);
      setSummary(res.summary || {});
      setTotal(res.total || 0);
    } catch {
      /* offline */
    }
    setLoading(false);
  }, [limit, sessionFilter]);

  useEffect(() => { load(); }, [load]);

  const maxCount = Math.max(...Object.values(summary), 1);

  // Group entries by hour for trend
  const hourlyTrend = entries.reduce<Record<string, Record<string, number>>>((acc, e) => {
    const hour = e.timestamp.slice(0, 13); // "2025-01-01T12"
    if (!acc[hour]) acc[hour] = {};
    acc[hour][e.emotion] = (acc[hour][e.emotion] || 0) + 1;
    return acc;
  }, {});
  const trendHours = Object.keys(hourlyTrend).sort();

  if (loading) {
    return (
      <div className="flex items-center justify-center h-[60vh]">
        <div className="w-5 h-5 border-2 border-t-transparent rounded-full animate-spin" style={{ borderColor: "var(--text-muted)", borderTopColor: "transparent" }} />
      </div>
    );
  }

  return (
    <div>
      <BlurFade delay={0}>
        <div className="flex items-center gap-3 mb-6">
          <SmilePlus size={20} />
          <h1 className="text-xl font-semibold tracking-tight">Emotion History</h1>
          <button onClick={load} className="ml-auto p-2 rounded-lg transition-colors cursor-pointer" style={{ color: "var(--text-muted)" }}>
            <RefreshCw size={16} />
          </button>
        </div>
      </BlurFade>

      {/* Filters */}
      <BlurFade>
        <div className="flex gap-3 items-end flex-wrap mb-4">
          <div>
            <label className="text-xs block mb-1" style={{ color: "var(--text-muted)" }}>Session ID</label>
            <input
              value={sessionFilter}
              onChange={(e) => setSessionFilter(e.target.value)}
              placeholder="all sessions"
              className="bg-transparent border rounded-lg px-3 py-1.5 text-sm focus:outline-none"
              style={{ borderColor: "var(--border)", width: 200 }}
            />
          </div>
          <div>
            <label className="text-xs block mb-1" style={{ color: "var(--text-muted)" }}>Limit</label>
            <select
              value={limit}
              onChange={(e) => setLimit(Number(e.target.value))}
              className="bg-transparent border rounded-lg px-3 py-1.5 text-sm focus:outline-none"
              style={{ borderColor: "var(--border)" }}
            >
              <option value={50}>50</option>
              <option value={100}>100</option>
              <option value={200}>200</option>
              <option value={500}>500</option>
            </select>
          </div>
        </div>
      </BlurFade>

      {/* Summary Bar Chart */}
      <BlurFade delay={0.05}>
        <div className="rounded-xl border p-5 mb-4" style={{ background: "var(--bg-card)", borderColor: "var(--border)" }}>
          <div className="text-xs font-medium uppercase tracking-wider mb-4" style={{ color: "var(--text-muted)" }}>
            Emotion Distribution ({total} events)
          </div>
          {Object.keys(summary).length === 0 ? (
            <div className="text-sm text-center py-8" style={{ color: "var(--text-muted)" }}>No emotion data recorded yet</div>
          ) : (
            <div className="space-y-2">
              {Object.entries(summary)
                .sort(([, a], [, b]) => b - a)
                .map(([emo, count]) => (
                  <div key={emo} className="flex items-center gap-3">
                    <span className="text-base w-6 text-center">{emotionEmoji[emo] || "•"}</span>
                    <span className="text-sm w-20 capitalize">{emo}</span>
                    <div className="flex-1 h-5 rounded-full overflow-hidden" style={{ background: "var(--bg-hover)" }}>
                      <div
                        className="h-full rounded-full transition-all duration-500"
                        style={{ width: `${(count / maxCount) * 100}%`, background: emotionColors[emo] || "var(--accent)" }}
                      />
                    </div>
                    <span className="text-sm tabular-nums w-10 text-right" style={{ color: "var(--text-muted)" }}>{count}</span>
                  </div>
                ))}
            </div>
          )}
        </div>
      </BlurFade>

      {/* Hourly Trend */}
      {trendHours.length > 1 && (
        <BlurFade delay={0.1}>
          <div className="rounded-xl border p-5 mb-4" style={{ background: "var(--bg-card)", borderColor: "var(--border)" }}>
            <div className="text-xs font-medium uppercase tracking-wider mb-4" style={{ color: "var(--text-muted)" }}>
              Hourly Trend
            </div>
            <div className="flex gap-1 items-end overflow-x-auto pb-2" style={{ minHeight: 120 }}>
              {trendHours.map((hour) => {
                const hourData = hourlyTrend[hour];
                const hourTotal = Object.values(hourData).reduce((a, b) => a + b, 0);
                const maxHourTotal = Math.max(...trendHours.map((h) => Object.values(hourlyTrend[h]).reduce((a, b) => a + b, 0)), 1);
                return (
                  <div key={hour} className="flex flex-col items-center gap-1" style={{ minWidth: 32 }}>
                    <div className="flex flex-col-reverse rounded overflow-hidden" style={{ width: 20, height: 80 }}>
                      {Object.entries(hourData).map(([emo, count]) => (
                        <div
                          key={emo}
                          style={{
                            height: `${(count / maxHourTotal) * 80}px`,
                            background: emotionColors[emo] || "var(--accent)",
                          }}
                        />
                      ))}
                    </div>
                    <span className="text-[10px] tabular-nums" style={{ color: "var(--text-muted)" }}>
                      {hour.slice(11, 13)}h
                    </span>
                  </div>
                );
              })}
            </div>
            {/* Legend */}
            <div className="flex gap-3 mt-3 flex-wrap">
              {Object.keys(summary).map((emo) => (
                <div key={emo} className="flex items-center gap-1">
                  <div className="w-3 h-3 rounded-sm" style={{ background: emotionColors[emo] || "var(--accent)" }} />
                  <span className="text-xs capitalize" style={{ color: "var(--text-muted)" }}>{emo}</span>
                </div>
              ))}
            </div>
          </div>
        </BlurFade>
      )}

      {/* Recent Events Table */}
      <BlurFade delay={0.15}>
        <div className="rounded-xl border p-5" style={{ background: "var(--bg-card)", borderColor: "var(--border)" }}>
          <div className="text-xs font-medium uppercase tracking-wider mb-4" style={{ color: "var(--text-muted)" }}>
            Recent Events
          </div>
          {entries.length === 0 ? (
            <div className="text-sm text-center py-8" style={{ color: "var(--text-muted)" }}>No events</div>
          ) : (
            <div className="overflow-x-auto">
              <table className="w-full text-sm">
                <thead>
                  <tr className="text-left" style={{ color: "var(--text-muted)" }}>
                    <th className="py-2 pr-3 font-medium text-xs uppercase">Time</th>
                    <th className="py-2 pr-3 font-medium text-xs uppercase">Emotion</th>
                    <th className="py-2 pr-3 font-medium text-xs uppercase">Confidence</th>
                    <th className="py-2 pr-3 font-medium text-xs uppercase">Source</th>
                    <th className="py-2 font-medium text-xs uppercase">Session</th>
                  </tr>
                </thead>
                <tbody>
                  {entries.slice(-50).reverse().map((e, i) => (
                    <tr key={i} className="border-t" style={{ borderColor: "var(--border)" }}>
                      <td className="py-2 pr-3 tabular-nums text-xs" style={{ color: "var(--text-muted)" }}>
                        {new Date(e.timestamp).toLocaleString()}
                      </td>
                      <td className="py-2 pr-3">
                        <span className="inline-flex items-center gap-1">
                          {emotionEmoji[e.emotion] || "•"}
                          <span className="capitalize">{e.emotion}</span>
                        </span>
                      </td>
                      <td className="py-2 pr-3 tabular-nums">{(e.confidence * 100).toFixed(0)}%</td>
                      <td className="py-2 pr-3" style={{ color: "var(--text-muted)" }}>{e.source}</td>
                      <td className="py-2 text-xs font-mono truncate max-w-[120px]" style={{ color: "var(--text-muted)" }}>{e.session_id}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </div>
      </BlurFade>
    </div>
  );
}
