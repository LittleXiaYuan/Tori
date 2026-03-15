"use client";

import { useEffect, useState } from "react";
import { api, type MetricsSnapshot, type VersionInfo, getApiKey, setApiKey } from "@/lib/api";
import { BlurFade } from "@/components/ui/blur-fade";
import {
  Activity,
  Zap,
  Clock,
  AlertTriangle,
  ArrowUpRight,
  ArrowDownRight,
} from "lucide-react";

function StatCard({
  label,
  value,
  sub,
  icon: Icon,
  color,
}: {
  label: string;
  value: string | number;
  sub?: string;
  icon: React.ElementType;
  color: string;
}) {
  return (
    <div
      className="rounded-xl p-5 border"
      style={{ background: "var(--bg-card)", borderColor: "var(--border)" }}
    >
      <div className="flex items-center justify-between mb-3">
        <span className="text-sm" style={{ color: "var(--text-muted)" }}>
          {label}
        </span>
        <div
          className="w-8 h-8 rounded-lg flex items-center justify-center"
          style={{ background: `${color}15`, color }}
        >
          <Icon size={16} />
        </div>
      </div>
      <div className="text-2xl font-bold">{value}</div>
      {sub && (
        <div className="text-xs mt-1" style={{ color: "var(--text-muted)" }}>
          {sub}
        </div>
      )}
    </div>
  );
}

function SkillTable({ skills }: { skills: MetricsSnapshot["skills"] }) {
  if (!skills || skills.length === 0) {
    return (
      <div className="text-sm py-8 text-center" style={{ color: "var(--text-muted)" }}>
        No skill calls recorded yet
      </div>
    );
  }
  return (
    <table className="w-full text-sm">
      <thead>
        <tr style={{ color: "var(--text-muted)" }}>
          <th className="text-left py-2 font-medium">Skill</th>
          <th className="text-right py-2 font-medium">Calls</th>
          <th className="text-right py-2 font-medium">Success</th>
          <th className="text-right py-2 font-medium">P50</th>
        </tr>
      </thead>
      <tbody>
        {skills.map((s) => (
          <tr key={s.name} className="border-t" style={{ borderColor: "var(--border)" }}>
            <td className="py-2.5 font-medium">{s.name}</td>
            <td className="py-2.5 text-right">{s.total}</td>
            <td className="py-2.5 text-right">
              <span
                className="inline-flex items-center gap-1"
                style={{
                  color: s.success_rate >= 0.9 ? "var(--success)" : s.success_rate >= 0.7 ? "var(--warning)" : "var(--danger)",
                }}
              >
                {s.success_rate >= 0.9 ? <ArrowUpRight size={12} /> : <ArrowDownRight size={12} />}
                {(s.success_rate * 100).toFixed(0)}%
              </span>
            </td>
            <td className="py-2.5 text-right" style={{ color: "var(--text-muted)" }}>
              {s.latency.p50_ms.toFixed(0)}ms
            </td>
          </tr>
        ))}
      </tbody>
    </table>
  );
}

function ErrorList({ errors }: { errors: MetricsSnapshot["recent_errors"] }) {
  if (!errors || errors.length === 0) {
    return (
      <div className="text-sm py-8 text-center" style={{ color: "var(--text-muted)" }}>
        No errors
      </div>
    );
  }
  return (
    <div className="space-y-2">
      {errors.slice(0, 5).map((e, i) => (
        <div
          key={i}
          className="flex items-start gap-3 p-3 rounded-lg"
          style={{ background: "var(--bg-hover)" }}
        >
          <AlertTriangle size={14} className="mt-0.5 shrink-0" style={{ color: "var(--danger)" }} />
          <div className="min-w-0">
            <div className="text-sm truncate">{e.message}</div>
            <div className="text-xs mt-0.5" style={{ color: "var(--text-muted)" }}>
              {e.count}x &middot; {new Date(e.last).toLocaleTimeString()}
            </div>
          </div>
        </div>
      ))}
    </div>
  );
}

function formatUptime(ns: number): string {
  const secs = Math.floor(ns / 1e9);
  const h = Math.floor(secs / 3600);
  const m = Math.floor((secs % 3600) / 60);
  if (h > 0) return `${h}h ${m}m`;
  return `${m}m`;
}

export default function DashboardPage() {
  const [metrics, setMetrics] = useState<MetricsSnapshot | null>(null);
  const [version, setVersion] = useState<VersionInfo | null>(null);
  const [error, setError] = useState("");

  useEffect(() => {
    const load = async () => {
      try {
        const [m, v] = await Promise.all([api.metrics(), api.version()]);
        setMetrics(m);
        setVersion(v);
        setError("");
      } catch (e: unknown) {
        setError(e instanceof Error ? e.message : "Failed to connect");
      }
    };
    load();
    const interval = setInterval(load, 5000);
    return () => clearInterval(interval);
  }, []);

  const [inputKey, setInputKey] = useState("");

  const handleSetKey = () => {
    setApiKey(inputKey);
    setError("");
    window.location.reload();
  };

  if (error) {
    const needsKey = error.includes("401") || error.includes("UNAUTHORIZED") || error.includes("credentials");
    return (
      <div className="flex flex-col items-center justify-center h-[60vh] gap-4 animate-in">
        <AlertTriangle size={48} style={{ color: "var(--warning)" }} />
        <h2 className="text-lg font-medium">{needsKey ? "API Key Required" : "Cannot connect to agent"}</h2>
        <p className="text-sm max-w-md text-center" style={{ color: "var(--text-muted)" }}>
          {needsKey ? "Enter your API key to authenticate with the agent." : error}
        </p>
        {needsKey ? (
          <div className="flex gap-2 mt-2">
            <input
              type="password"
              value={inputKey}
              onChange={(e) => setInputKey(e.target.value)}
              onKeyDown={(e) => e.key === "Enter" && handleSetKey()}
              placeholder="tori_default_key_2024"
              className="px-4 py-2.5 rounded-xl border text-sm outline-none w-64"
              style={{ background: "var(--bg-card)", borderColor: "var(--border)", color: "var(--text)" }}
            />
            <button onClick={handleSetKey}
              className="btn-glow px-4 py-2.5 rounded-xl text-sm font-medium">
              Connect
            </button>
          </div>
        ) : (
          <p className="text-xs" style={{ color: "var(--text-muted)" }}>
            Make sure yunque-agent is running on localhost:9090
          </p>
        )}
      </div>
    );
  }

  return (
    <div>
      <BlurFade delay={0}>
        <div className="flex items-center justify-between mb-6">
          <h1 className="text-xl font-semibold tracking-tight">Dashboard</h1>
          {version && (
            <span
              className="text-xs px-2.5 py-1 rounded-full"
              style={{ background: "var(--bg-hover)", color: "var(--text-muted)" }}
            >
              v{version.version} ({version.git_commit})
            </span>
          )}
        </div>
      </BlurFade>

      {metrics && (
        <>
          <BlurFade delay={0.05}>
            <div className="grid grid-cols-4 gap-4 mb-6">
              <StatCard
                label="Total Requests"
                value={metrics.requests_total}
                sub={`${metrics.requests_success} success / ${metrics.requests_failed} failed`}
                icon={Activity}
                color="#fafafa"
              />
              <StatCard
                label="Tokens Used"
                value={metrics.tokens_total.toLocaleString()}
                sub={`${metrics.tokens_in.toLocaleString()} in / ${metrics.tokens_out.toLocaleString()} out`}
                icon={Zap}
                color="#d4d4d4"
              />
              <StatCard
                label="Avg Latency"
                value={`${metrics.request_latency.avg_ms.toFixed(0)}ms`}
                sub={`P95: ${metrics.request_latency.p95_ms.toFixed(0)}ms`}
                icon={Clock}
                color="#a3a3a3"
              />
              <StatCard
                label="Uptime"
                value={formatUptime(metrics.uptime)}
                sub={`${metrics.skills.length} active skills`}
                icon={ArrowUpRight}
                color="#737373"
              />
            </div>
          </BlurFade>

          <BlurFade delay={0.1}>
            <div className="grid grid-cols-2 gap-4">
              <div
                className="rounded-xl p-5 border"
                style={{ background: "var(--bg-card)", borderColor: "var(--border)" }}
              >
                <h2 className="text-sm font-medium mb-4" style={{ color: "var(--text-muted)" }}>
                  Skill Performance
                </h2>
                <SkillTable skills={metrics.skills} />
              </div>
              <div
                className="rounded-xl p-5 border"
                style={{ background: "var(--bg-card)", borderColor: "var(--border)" }}
              >
                <h2 className="text-sm font-medium mb-4" style={{ color: "var(--text-muted)" }}>
                  Recent Errors
                </h2>
                <ErrorList errors={metrics.recent_errors} />
              </div>
            </div>
          </BlurFade>
        </>
      )}
    </div>
  );
}
