"use client";

import { useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { api, type MetricsSnapshot, type VersionInfo, type SkillInfo } from "@/lib/api";
import { BlurFade } from "@/components/ui/blur-fade";
import { useI18n } from "@/lib/i18n";
import {
  Activity,
  Zap,
  Clock,
  Package,
  AlertTriangle,
  Server,
  Cpu,
  ArrowRight,
  RefreshCw,
} from "lucide-react";

function formatUptime(seconds: number): string {
  const d = Math.floor(seconds / 86400);
  const h = Math.floor((seconds % 86400) / 3600);
  const m = Math.floor((seconds % 3600) / 60);
  if (d > 0) return `${d}d ${h}h`;
  if (h > 0) return `${h}h ${m}m`;
  return `${m}m`;
}

export default function DashboardPage() {
  const router = useRouter();
  const { locale } = useI18n();
  const [metrics, setMetrics] = useState<MetricsSnapshot | null>(null);
  const [version, setVersion] = useState<VersionInfo | null>(null);
  const [skills, setSkills] = useState<SkillInfo[]>([]);
  const [loading, setLoading] = useState(true);

  const load = async () => {
    try {
      const [m, v, s] = await Promise.all([
        api.metrics(),
        api.version(),
        api.skills(),
      ]);
      setMetrics(m);
      setVersion(v);
      setSkills(s);
    } catch { /* offline */ }
    setLoading(false);
  };

  useEffect(() => {
    load();
    const interval = setInterval(load, 8000);
    return () => clearInterval(interval);
  }, []);

  const online = !!metrics;
  const zh = locale === "zh";

  return (
    <div className="space-y-6 max-w-5xl mx-auto">
      {/* Header */}
      <BlurFade delay={0.03}>
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-3">
            <div
              className="w-10 h-10 rounded-xl flex items-center justify-center"
              style={{ background: "var(--accent)", color: "#fff", fontWeight: 700, fontSize: 16 }}
            >
              Y
            </div>
            <div>
              <h1 className="text-xl font-bold" style={{ color: "var(--text)" }}>
                {zh ? "控制台" : "Dashboard"}
              </h1>
              <div className="flex items-center gap-2 mt-0.5">
                <span
                  style={{
                    width: 6, height: 6, borderRadius: "50%",
                    background: online ? "var(--success)" : "var(--text-muted)",
                    display: "inline-block",
                  }}
                />
                <span className="text-xs" style={{ color: "var(--text-muted)" }}>
                  {online ? (zh ? "在线" : "Online") : (zh ? "离线" : "Offline")}
                  {version && ` · v${version.version}`}
                </span>
              </div>
            </div>
          </div>
          <button
            onClick={() => { setLoading(true); load(); }}
            className="p-2 rounded-lg transition-colors"
            style={{ color: "var(--text-muted)" }}
            title="Refresh"
          >
            <RefreshCw size={16} className={loading ? "animate-spin" : ""} />
          </button>
        </div>
      </BlurFade>

      {/* Stats Grid */}
      <BlurFade delay={0.06}>
        <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
          <StatCard
            icon={Activity}
            label={zh ? "请求总数" : "Requests"}
            value={metrics?.requests_total ?? 0}
            sub={
              metrics
                ? `${metrics.requests_success} ${zh ? "成功" : "ok"} / ${metrics.requests_failed} ${zh ? "失败" : "fail"}`
                : "—"
            }
            color="var(--accent)"
          />
          <StatCard
            icon={Zap}
            label={zh ? "Token 消耗" : "Tokens"}
            value={metrics?.tokens_total ?? 0}
            sub={
              metrics
                ? `${metrics.tokens_in.toLocaleString()} in / ${metrics.tokens_out.toLocaleString()} out`
                : "—"
            }
            color="var(--warning)"
          />
          <StatCard
            icon={Clock}
            label={zh ? "平均延迟" : "Avg Latency"}
            value={metrics?.request_latency.avg_ms ? `${metrics.request_latency.avg_ms.toFixed(0)}ms` : "—"}
            sub={
              metrics?.request_latency.count
                ? `P95: ${metrics.request_latency.p95_ms.toFixed(0)}ms`
                : "—"
            }
            color="var(--success)"
          />
          <StatCard
            icon={Cpu}
            label={zh ? "运行时间" : "Uptime"}
            value={metrics ? formatUptime(metrics.uptime) : "—"}
            sub={version ? `Go ${version.go_version}` : "—"}
            color="#a78bfa"
          />
        </div>
      </BlurFade>

      {/* Skills Table */}
      <BlurFade delay={0.09}>
        <div
          className="rounded-xl border p-5"
          style={{ background: "var(--bg-card)", borderColor: "var(--border)" }}
        >
          <div className="flex items-center justify-between mb-4">
            <div className="flex items-center gap-2">
              <Package size={15} style={{ color: "var(--accent)" }} />
              <span className="text-sm font-medium">{zh ? "技能概览" : "Skills"}</span>
              <span className="text-xs px-2 py-0.5 rounded-full" style={{ background: "var(--bg-hover)", color: "var(--text-muted)" }}>
                {skills.length}
              </span>
            </div>
            <button
              onClick={() => router.push("/skills")}
              className="text-xs flex items-center gap-1 transition-colors"
              style={{ color: "var(--accent)" }}
            >
              {zh ? "管理" : "Manage"} <ArrowRight size={12} />
            </button>
          </div>
          {skills.length > 0 ? (
            <div className="space-y-1">
              {skills.slice(0, 8).map((skill) => (
                <div
                  key={skill.name}
                  className="flex items-center justify-between px-3 py-2 rounded-lg text-sm"
                  style={{ background: "transparent" }}
                  onMouseEnter={(e) => { e.currentTarget.style.background = "var(--bg-hover)"; }}
                  onMouseLeave={(e) => { e.currentTarget.style.background = "transparent"; }}
                >
                  <span style={{ color: "var(--text)" }}>{skill.name}</span>
                  <span className="text-xs truncate ml-4 max-w-[300px]" style={{ color: "var(--text-muted)" }}>
                    {skill.description}
                  </span>
                </div>
              ))}
              {skills.length > 8 && (
                <div className="text-xs text-center pt-2" style={{ color: "var(--text-muted)" }}>
                  +{skills.length - 8} {zh ? "更多" : "more"}
                </div>
              )}
            </div>
          ) : (
            <div className="text-sm text-center py-6" style={{ color: "var(--text-muted)" }}>
              {zh ? "暂无技能数据" : "No skills loaded"}
            </div>
          )}
        </div>
      </BlurFade>

      {/* Recent Errors */}
      {metrics && metrics.recent_errors.length > 0 && (
        <BlurFade delay={0.12}>
          <div
            className="rounded-xl border p-5"
            style={{ background: "var(--bg-card)", borderColor: "var(--border)" }}
          >
            <div className="flex items-center gap-2 mb-4">
              <AlertTriangle size={14} style={{ color: "var(--danger)" }} />
              <span className="text-sm font-medium" style={{ color: "var(--text-muted)" }}>
                {zh ? "最近错误" : "Recent Errors"}
              </span>
            </div>
            <div className="space-y-2">
              {metrics.recent_errors.map((e, i) => (
                <div
                  key={i}
                  className="text-xs p-3 rounded-lg flex justify-between"
                  style={{ background: "var(--bg-hover)" }}
                >
                  <span className="truncate flex-1" style={{ color: "var(--text-secondary)" }}>{e.message}</span>
                  <span className="shrink-0 ml-3" style={{ color: "var(--text-muted)" }}>{e.count}x</span>
                </div>
              ))}
            </div>
          </div>
        </BlurFade>
      )}

      {/* System Info */}
      {version && (
        <BlurFade delay={0.15}>
          <div
            className="rounded-xl border p-5"
            style={{ background: "var(--bg-card)", borderColor: "var(--border)" }}
          >
            <div className="flex items-center gap-2 mb-3">
              <Server size={14} style={{ color: "var(--text-muted)" }} />
              <span className="text-sm font-medium" style={{ color: "var(--text-muted)" }}>
                {zh ? "系统信息" : "System Info"}
              </span>
            </div>
            <div className="grid grid-cols-2 md:grid-cols-4 gap-3 text-xs">
              {[
                { label: zh ? "版本" : "Version", value: version.version },
                { label: "Git", value: version.git_commit?.slice(0, 8) || "—" },
                { label: "OS", value: `${version.os}/${version.arch}` },
                { label: zh ? "构建日期" : "Built", value: version.build_date || "—" },
              ].map((item) => (
                <div key={item.label} className="p-3 rounded-lg" style={{ background: "var(--bg-hover)" }}>
                  <div style={{ color: "var(--text-muted)" }}>{item.label}</div>
                  <div className="font-mono mt-1" style={{ color: "var(--text)" }}>{item.value}</div>
                </div>
              ))}
            </div>
          </div>
        </BlurFade>
      )}
    </div>
  );
}

function StatCard({
  icon: Icon,
  label,
  value,
  sub,
  color,
}: {
  icon: React.ElementType;
  label: string;
  value: number | string;
  sub: string;
  color: string;
}) {
  return (
    <div
      className="rounded-xl border p-4"
      style={{ background: "var(--bg-card)", borderColor: "var(--border)" }}
    >
      <div className="flex items-center gap-2 mb-2">
        <Icon size={15} style={{ color }} />
        <span className="text-xs" style={{ color: "var(--text-muted)" }}>{label}</span>
      </div>
      <div className="text-2xl font-bold" style={{ color: "var(--text)" }}>
        {typeof value === "number" ? value.toLocaleString() : value}
      </div>
      <div className="text-xs mt-1" style={{ color: "var(--text-muted)" }}>{sub}</div>
    </div>
  );
}
