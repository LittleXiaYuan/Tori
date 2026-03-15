"use client";

import { useEffect, useState } from "react";
import { api, type MetricsSnapshot } from "@/lib/api";
import { Activity, Clock, Zap, AlertTriangle } from "lucide-react";

function MetricBar({ label, value, max, color }: { label: string; value: number; max: number; color: string }) {
  const pct = max > 0 ? Math.min((value / max) * 100, 100) : 0;
  return (
    <div className="flex items-center gap-3">
      <span className="text-xs w-20 shrink-0" style={{ color: "var(--text-muted)" }}>{label}</span>
      <div className="flex-1 h-2 rounded-full" style={{ background: "var(--bg-hover)" }}>
        <div className="h-2 rounded-full transition-all" style={{ width: `${pct}%`, background: color }} />
      </div>
      <span className="text-xs w-16 text-right font-mono">{value.toFixed(1)}ms</span>
    </div>
  );
}

export default function MetricsPage() {
  const [metrics, setMetrics] = useState<MetricsSnapshot | null>(null);
  const [promText, setPromText] = useState("");

  useEffect(() => {
    const load = async () => {
      try {
        const m = await api.metrics();
        setMetrics(m);
        const res = await fetch("/api/v1/metrics/prometheus");
        if (res.ok) setPromText(await res.text());
      } catch {}
    };
    load();
    const interval = setInterval(load, 5000);
    return () => clearInterval(interval);
  }, []);

  if (!metrics) {
    return (
      <div className="flex items-center justify-center h-[60vh]" style={{ color: "var(--text-muted)" }}>
        Loading metrics...
      </div>
    );
  }

  const lat = metrics.request_latency;

  return (
    <div>
      <h1 className="text-xl font-semibold mb-6">Metrics</h1>

      <div className="grid grid-cols-3 gap-4 mb-6">
        <div className="rounded-xl border p-5" style={{ background: "var(--bg-card)", borderColor: "var(--border)" }}>
          <div className="flex items-center gap-2 mb-3">
            <Activity size={16} style={{ color: "var(--accent)" }} />
            <span className="text-sm font-medium">Requests</span>
          </div>
          <div className="text-3xl font-bold mb-1">{metrics.requests_total}</div>
          <div className="flex gap-3 text-xs" style={{ color: "var(--text-muted)" }}>
            <span style={{ color: "var(--success)" }}>{metrics.requests_success} ok</span>
            <span style={{ color: "var(--danger)" }}>{metrics.requests_failed} fail</span>
          </div>
        </div>

        <div className="rounded-xl border p-5" style={{ background: "var(--bg-card)", borderColor: "var(--border)" }}>
          <div className="flex items-center gap-2 mb-3">
            <Zap size={16} style={{ color: "var(--warning)" }} />
            <span className="text-sm font-medium">Tokens</span>
          </div>
          <div className="text-3xl font-bold mb-1">{metrics.tokens_total.toLocaleString()}</div>
          <div className="flex gap-3 text-xs" style={{ color: "var(--text-muted)" }}>
            <span>{metrics.tokens_in.toLocaleString()} in</span>
            <span>{metrics.tokens_out.toLocaleString()} out</span>
          </div>
        </div>

        <div className="rounded-xl border p-5" style={{ background: "var(--bg-card)", borderColor: "var(--border)" }}>
          <div className="flex items-center gap-2 mb-3">
            <Clock size={16} style={{ color: "var(--success)" }} />
            <span className="text-sm font-medium">Latency</span>
          </div>
          <div className="text-3xl font-bold mb-1">{lat.avg_ms.toFixed(0)}ms</div>
          <div className="text-xs" style={{ color: "var(--text-muted)" }}>
            avg of {lat.count} requests
          </div>
        </div>
      </div>

      {lat.count > 0 && (
        <div className="rounded-xl border p-5 mb-6" style={{ background: "var(--bg-card)", borderColor: "var(--border)" }}>
          <h2 className="text-sm font-medium mb-4" style={{ color: "var(--text-muted)" }}>Latency Distribution</h2>
          <div className="space-y-3">
            <MetricBar label="P50" value={lat.p50_ms} max={lat.max_ms} color="var(--success)" />
            <MetricBar label="P95" value={lat.p95_ms} max={lat.max_ms} color="var(--warning)" />
            <MetricBar label="P99" value={lat.p99_ms} max={lat.max_ms} color="var(--danger)" />
            <MetricBar label="Max" value={lat.max_ms} max={lat.max_ms} color="var(--danger)" />
          </div>
        </div>
      )}

      {metrics.recent_errors.length > 0 && (
        <div className="rounded-xl border p-5 mb-6" style={{ background: "var(--bg-card)", borderColor: "var(--border)" }}>
          <h2 className="text-sm font-medium mb-4 flex items-center gap-2">
            <AlertTriangle size={14} style={{ color: "var(--danger)" }} />
            <span style={{ color: "var(--text-muted)" }}>Recent Errors</span>
          </h2>
          <div className="space-y-2">
            {metrics.recent_errors.map((e, i) => (
              <div key={i} className="text-xs p-3 rounded-lg flex justify-between"
                style={{ background: "var(--bg-hover)" }}>
                <span className="truncate flex-1">{e.message}</span>
                <span className="shrink-0 ml-3" style={{ color: "var(--text-muted)" }}>{e.count}x</span>
              </div>
            ))}
          </div>
        </div>
      )}

      {promText && (
        <div className="rounded-xl border p-5" style={{ background: "var(--bg-card)", borderColor: "var(--border)" }}>
          <h2 className="text-sm font-medium mb-3" style={{ color: "var(--text-muted)" }}>
            Prometheus Export
          </h2>
          <pre className="text-xs p-3 rounded-lg overflow-auto max-h-60"
            style={{ background: "var(--bg-hover)", color: "var(--text-muted)" }}>
            {promText}
          </pre>
        </div>
      )}
    </div>
  );
}
