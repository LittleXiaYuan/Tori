"use client";

import { useEffect, useState, useCallback } from "react";
import { Card, Button, Spinner, Chip, Tooltip, ProgressBar } from "@heroui/react";
import { api, type MetricsSnapshot } from "@/lib/api";
import { BarChart3, RefreshCw, Activity, Clock, Zap, TrendingUp } from "lucide-react";
import { usePolling } from "@/lib/use-polling";
import PageHeader from "@/components/page-header";

export default function MetricsPage() {
  const [metrics, setMetrics] = useState<MetricsSnapshot | null>(null);
  const [loading, setLoading] = useState(true);

  const load = useCallback(async () => {
    try {
      const m = await api.metrics();
      setMetrics(m);
    } catch { /* offline */ }
    finally { setLoading(false); }
  }, []);

  useEffect(() => { load(); }, [load]);

  usePolling(load, 5000);

  if (loading) {
    return <div className="flex items-center justify-center h-[60vh]"><Spinner size="lg" /></div>;
  }

  const latency: Record<string, number> = (metrics?.request_latency || {}) as Record<string, number>;

  return (
    <div className="page-root space-y-6 animate-fade-in-up">
      <PageHeader icon={<BarChart3 size={20} />} title="性能指标" onRefresh={() => { setLoading(true); load(); }} />

      {!metrics ? (
        <Card className="section-card p-12 text-center">
          <div className="text-sm" style={{ color: "var(--yunque-text-muted)" }}>{"无法连接到后端"}</div>
        </Card>
      ) : (
        <>
          {/* Request stats */}
          <div className="grid grid-cols-2 md:grid-cols-4 gap-4 stagger-children">
            <Card className="section-card p-4 hover-lift">
              <div className="flex items-center gap-2 mb-2">
                <Activity size={14} style={{ color: "var(--yunque-accent)" }} />
                <span className="kpi-label">{"请求总数"}</span>
              </div>
              <div className="kpi-value">{(metrics.requests_total || 0).toLocaleString()}</div>
            </Card>
            <Card className="section-card p-4 hover-lift">
              <div className="flex items-center gap-2 mb-2">
                <TrendingUp size={14} style={{ color: "var(--yunque-success)" }} />
                <span className="kpi-label">{"成功率"}</span>
              </div>
              <div className="kpi-value" style={{ color: "var(--yunque-success)" }}>
                {metrics.requests_total > 0 ? ((metrics.requests_success / metrics.requests_total) * 100).toFixed(1) : "0"}%
              </div>
            </Card>
            <Card className="section-card p-4 hover-lift">
              <div className="flex items-center gap-2 mb-2">
                <Zap size={14} style={{ color: "var(--yunque-warning)" }} />
                <span className="kpi-label">Token {"总消耗"}</span>
              </div>
              <div className="kpi-value">{(metrics.tokens_total || 0).toLocaleString()}</div>
            </Card>
            <Card className="section-card p-4 hover-lift">
              <div className="flex items-center gap-2 mb-2">
                <Clock size={14} style={{ color: "#a78bfa" }} />
                <span className="kpi-label">{"请求数量"}</span>
              </div>
              <div className="kpi-value">{latency.count || 0}</div>
            </Card>
          </div>

          {/* Latency + Token side by side */}
          <div className="grid grid-cols-1 lg:grid-cols-2 gap-4">
            {/* Latency distribution */}
            <Card className="section-card p-5">
              <h2 className="section-title">{"延迟分布"}</h2>
              <div className="space-y-4">
                {[
                  { label: "P50", value: latency.p50_ms, color: "var(--yunque-success)" },
                  { label: "P90", value: latency.p90_ms, color: "var(--yunque-accent)" },
                  { label: "P95", value: latency.p95_ms, color: "var(--yunque-warning)" },
                  { label: "P99", value: latency.p99_ms, color: "var(--yunque-danger)" },
                  { label: "Avg", value: latency.avg_ms, color: "#a78bfa" },
                ].map((p) => (
                  <div key={p.label} className="space-y-1">
                    <div className="flex items-center justify-between text-xs">
                      <span style={{ color: "var(--yunque-text-muted)" }}>{p.label}</span>
                      <span style={{ color: p.color }}>{p.value ? `${p.value.toFixed(0)}ms` : "—"}</span>
                    </div>
                    <ProgressBar
                      value={p.value || 0}
                      maxValue={Math.max(latency.p99_ms || 1000, 1)}
                      aria-label={p.label}
                      style={{ "--progressbar-fill-color": p.color } as any}
                    >
                      <ProgressBar.Track>
                        <ProgressBar.Fill />
                      </ProgressBar.Track>
                    </ProgressBar>
                  </div>
                ))}
              </div>
            </Card>

            {/* Token breakdown */}
            <Card className="section-card p-5 flex flex-col">
              <h2 className="text-sm font-medium mb-4" style={{ color: "var(--yunque-text)" }}>Token {"明细"}</h2>
              <div className="space-y-4 flex-1">
                <div className="p-4 rounded-lg flex items-center justify-between" style={{ background: "rgba(255,255,255,0.03)" }}>
                  <div>
                    <div className="kpi-label mb-1">Input Tokens</div>
                    <div className="kpi-value" style={{ fontSize: "var(--text-xl)" }}>{(metrics.tokens_in || 0).toLocaleString()}</div>
                  </div>
                  <Zap size={28} style={{ color: "#22c55e", opacity: 0.4 }} />
                </div>
                <div className="p-4 rounded-lg flex items-center justify-between" style={{ background: "rgba(255,255,255,0.03)" }}>
                  <div>
                    <div className="kpi-label mb-1">Output Tokens</div>
                    <div className="kpi-value" style={{ fontSize: "var(--text-xl)" }}>{(metrics.tokens_out || 0).toLocaleString()}</div>
                  </div>
                  <Zap size={28} style={{ color: "#f59e0b", opacity: 0.4 }} />
                </div>
                {metrics.tokens_total > 0 && (
                  <div className="space-y-1">
                    <div className="flex items-center justify-between text-xs">
                      <span style={{ color: "var(--yunque-text-muted)" }}>Input {"占比"}</span>
                      <span style={{ color: "var(--yunque-text-muted)" }}>
                        {((metrics.tokens_in / metrics.tokens_total) * 100).toFixed(1)}%
                      </span>
                    </div>
                    <ProgressBar
                      value={metrics.tokens_in || 0}
                      maxValue={metrics.tokens_total}
                      aria-label="input ratio"
                      style={{ "--progressbar-fill-color": "#22c55e" } as any}
                    >
                      <ProgressBar.Track>
                        <ProgressBar.Fill />
                      </ProgressBar.Track>
                    </ProgressBar>
                  </div>
                )}
              </div>
            </Card>
          </div>

          {/* Errors */}
          {metrics.recent_errors?.length > 0 && (
            <Card className="section-card p-5">
              <h2 className="text-sm font-medium mb-4" style={{ color: "var(--yunque-danger)" }}>{"最近错误"}</h2>
              <div className="space-y-2">
                {metrics.recent_errors.map((err: { message: string; count: number }, i: number) => (
                  <div key={i} className="flex items-center justify-between p-3 rounded-lg" style={{ background: "rgba(239,68,68,0.05)" }}>
                    <span className="text-sm truncate flex-1" style={{ color: "var(--yunque-text)" }}>{err.message}</span>
                    <Chip size="sm" style={{ background: "rgba(239,68,68,0.1)", color: "var(--yunque-danger)" }}>{err.count}x</Chip>
                  </div>
                ))}
              </div>
            </Card>
          )}
        </>
      )}
    </div>
  );
}
