"use client";

import { useEffect, useState } from "react";
import { api, type MetricsSnapshot, type VersionInfo } from "@/lib/api";
import { BlurFade } from "@/components/ui/blur-fade";
import { CircleProgress } from "@/components/ui/circle-progress";
import { Modal } from "@/components/ui/modal";
import {
  Activity,
  Zap,
  Clock,
  AlertTriangle,
  ArrowUpRight,
  ArrowDownRight,
  Server,
  Brain,
  Sparkles,
  Quote,
  Info,
  Cpu,
  BarChart3,
  CheckCircle2,
  XCircle,
  Timer,
} from "lucide-react";

function formatUptime(ns: number): string {
  const secs = Math.floor(ns / 1e9);
  const d = Math.floor(secs / 86400);
  const h = Math.floor((secs % 86400) / 3600);
  const m = Math.floor((secs % 3600) / 60);
  if (d > 0) return `${d}天 ${h}时`;
  if (h > 0) return `${h}时 ${m}分`;
  return `${m}分`;
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
  value: string | number;
  sub?: string;
  color?: string;
}) {
  return (
    <div
      className="rounded-xl border card-hover"
      style={{ background: "var(--bg-card)", borderColor: "var(--border)", padding: "20px" }}
    >
      <div style={{ display: "flex", alignItems: "center", gap: 10, marginBottom: 12 }}>
        <div
          style={{
            width: 36,
            height: 36,
            borderRadius: 10,
            display: "flex",
            alignItems: "center",
            justifyContent: "center",
            background: color ? `${color}15` : "var(--accent-subtle)",
          }}
        >
          <Icon size={18} style={{ color: color || "var(--accent)" }} />
        </div>
        <span style={{ fontSize: 12, color: "var(--text-muted)", fontWeight: 500 }}>{label}</span>
      </div>
      <div style={{ fontSize: 28, fontWeight: 700, letterSpacing: "-0.02em" }} className="count-up">
        {value}
      </div>
      {sub && (
        <div style={{ fontSize: 11, color: "var(--text-muted)", marginTop: 4 }}>{sub}</div>
      )}
    </div>
  );
}

function SystemInfoCard({ version, metrics }: { version: VersionInfo | null; metrics: MetricsSnapshot | null }) {
  const [showModal, setShowModal] = useState(false);

  return (
    <>
      <div
        className="rounded-xl border card-hover"
        style={{ background: "var(--bg-card)", borderColor: "var(--border)", padding: "20px" }}
      >
        <div style={{ display: "flex", alignItems: "center", gap: 10, marginBottom: 16 }}>
          <div style={{
            width: 44,
            height: 44,
            borderRadius: 12,
            background: "var(--accent-subtle)",
            display: "flex",
            alignItems: "center",
            justifyContent: "center",
          }}>
            <Brain size={24} style={{ color: "var(--accent)" }} />
          </div>
          <div>
            <div style={{ display: "flex", alignItems: "center", gap: 8 }}>
              <span style={{ fontSize: 18, fontWeight: 700 }}>云雀 Agent</span>
              <span className="breathe" style={{
                width: 8,
                height: 8,
                borderRadius: "50%",
                background: "var(--success)",
                display: "inline-block",
              }} />
            </div>
            {version && (
              <span style={{ fontSize: 12, color: "var(--text-muted)" }}>v{version.version}</span>
            )}
          </div>
        </div>

        <div style={{ display: "flex", flexDirection: "column", gap: 10 }}>
          <InfoRow icon={Server} label="版本" value={version ? `v${version.version}` : "—"}
            tag={version?.git_commit?.slice(0, 7)} onTagClick={() => setShowModal(true)} />
          <InfoRow icon={Activity} label="运行时间" value={metrics ? formatUptime(metrics.uptime) : "—"} />
          <InfoRow icon={Zap} label="活跃技能" value={`${metrics?.skills?.length ?? 0} 个`} />
          <InfoRow icon={Cpu} label="平台" value={version ? `${version.os}/${version.arch}` : "—"} />
        </div>
      </div>

      <Modal open={showModal} onClose={() => setShowModal(false)} title="版本信息" width="420px">
        {version ? (
          <div style={{ display: "flex", flexDirection: "column", gap: 16 }}>
            <div style={{ display: "flex", alignItems: "center", gap: 12 }}>
              <span style={{ fontSize: 13, color: "var(--text-muted)" }}>当前版本</span>
              <span style={{
                fontSize: 13, fontWeight: 600, padding: "2px 10px", borderRadius: 999,
                background: "var(--accent-subtle)", color: "var(--accent)",
              }}>v{version.version}</span>
            </div>
            <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: 10 }}>
              <DetailItem label="Git Commit" value={version.git_commit || "—"} />
              <DetailItem label="构建时间" value={version.build_date || "—"} />
              <DetailItem label="Go 版本" value={version.go_version || "—"} />
              <DetailItem label="操作系统" value={`${version.os || "—"}/${version.arch || "—"}`} />
            </div>
          </div>
        ) : (
          <p style={{ fontSize: 13, color: "var(--text-muted)" }}>加载中...</p>
        )}
      </Modal>
    </>
  );
}

function GaugesCard({ metrics }: { metrics: MetricsSnapshot | null }) {
  const successRate = metrics
    ? metrics.requests_total > 0
      ? (metrics.requests_success / metrics.requests_total) * 100
      : 100
    : 0;
  const tokenBudget = 1_000_000;
  const tokenUsage = metrics ? Math.min((metrics.tokens_total / tokenBudget) * 100, 100) : 0;

  return (
    <div
      className="rounded-xl border card-hover"
      style={{
        background: "var(--bg-card)",
        borderColor: "var(--border)",
        padding: "20px",
        display: "flex",
        alignItems: "center",
        justifyContent: "space-around",
      }}
    >
      <CircleProgress value={successRate} size={100} strokeWidth={8} color="var(--success)" label="请求成功率" />
      <div style={{ width: 1, height: 56, background: "var(--border)" }} />
      <CircleProgress value={tokenUsage} size={100} strokeWidth={8} color="var(--accent)" label="Token 用量" />
    </div>
  );
}

function SkillTable({ skills }: { skills: MetricsSnapshot["skills"] }) {
  if (!skills || skills.length === 0) {
    return (
      <div style={{ fontSize: 13, padding: "32px 0", textAlign: "center", color: "var(--text-muted)" }}>
        暂无技能调用记录
      </div>
    );
  }
  return (
    <div style={{ overflowX: "auto" }}>
      <table style={{ width: "100%", fontSize: 13, borderCollapse: "collapse" }}>
        <thead>
          <tr style={{ color: "var(--text-muted)" }}>
            <th style={{ textAlign: "left", padding: "8px 0", fontWeight: 500 }}>技能</th>
            <th style={{ textAlign: "right", padding: "8px 0", fontWeight: 500 }}>调用</th>
            <th style={{ textAlign: "right", padding: "8px 0", fontWeight: 500 }}>成功率</th>
            <th style={{ textAlign: "right", padding: "8px 0", fontWeight: 500 }}>P50</th>
          </tr>
        </thead>
        <tbody>
          {skills.map((s) => (
            <tr key={s.name} style={{ borderTop: "1px solid var(--border)" }}>
              <td style={{ padding: "10px 0", fontWeight: 500 }}>{s.name}</td>
              <td style={{ padding: "10px 0", textAlign: "right" }}>{s.total}</td>
              <td style={{ padding: "10px 0", textAlign: "right" }}>
                <span style={{
                  display: "inline-flex",
                  alignItems: "center",
                  gap: 4,
                  color: s.success_rate >= 0.9 ? "var(--success)" : s.success_rate >= 0.7 ? "var(--warning)" : "var(--danger)",
                }}>
                  {s.success_rate >= 0.9 ? <ArrowUpRight size={12} /> : <ArrowDownRight size={12} />}
                  {(s.success_rate * 100).toFixed(0)}%
                </span>
              </td>
              <td style={{ padding: "10px 0", textAlign: "right", color: "var(--text-muted)" }}>
                {s.latency.p50_ms.toFixed(0)}ms
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

function ErrorList({ errors }: { errors: MetricsSnapshot["recent_errors"] }) {
  if (!errors || errors.length === 0) {
    return (
      <div style={{ fontSize: 13, padding: "32px 0", textAlign: "center", color: "var(--text-muted)" }}>
        暂无错误
      </div>
    );
  }
  return (
    <div style={{ display: "flex", flexDirection: "column", gap: 8 }}>
      {errors.slice(0, 5).map((e, i) => (
        <div
          key={i}
          style={{
            display: "flex",
            alignItems: "flex-start",
            gap: 10,
            padding: 10,
            borderRadius: 8,
            background: "var(--bg-hover)",
          }}
        >
          <AlertTriangle size={14} style={{ color: "var(--danger)", marginTop: 2, flexShrink: 0 }} />
          <div style={{ minWidth: 0 }}>
            <div style={{ fontSize: 13, overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>{e.message}</div>
            <div style={{ fontSize: 11, color: "var(--text-muted)", marginTop: 2 }}>
              {e.count}× · {new Date(e.last).toLocaleTimeString()}
            </div>
          </div>
        </div>
      ))}
    </div>
  );
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

  if (error) {
    return (
      <div className="animate-in" style={{
        display: "flex", flexDirection: "column", alignItems: "center",
        justifyContent: "center", height: "60vh", gap: 16,
      }}>
        <AlertTriangle size={48} style={{ color: "var(--warning)" }} />
        <h2 style={{ fontSize: 18, fontWeight: 500 }}>无法连接到 Agent</h2>
        <p style={{ fontSize: 13, color: "var(--text-muted)", maxWidth: 400, textAlign: "center" }}>{error}</p>
      </div>
    );
  }

  return (
    <div className="animate-in stagger">
      {/* Page header */}
      <BlurFade delay={0}>
        <div style={{ marginBottom: 24 }}>
          <h1 style={{ fontSize: 22, fontWeight: 700, letterSpacing: "-0.02em" }}>仪表盘</h1>
          <p style={{ fontSize: 13, color: "var(--text-muted)", marginTop: 4 }}>
            系统概览与实时监控
          </p>
        </div>
      </BlurFade>

      {/* Top: Agent Profile + Gauges */}
      <BlurFade delay={0.02}>
        <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: 16, marginBottom: 16 }}>
          <SystemInfoCard version={version} metrics={metrics} />
          <GaugesCard metrics={metrics} />
        </div>
      </BlurFade>

      {/* Stats grid */}
      {metrics && (
        <BlurFade delay={0.04}>
          <div style={{
            display: "grid",
            gridTemplateColumns: "repeat(auto-fit, minmax(180px, 1fr))",
            gap: 12,
            marginBottom: 16,
          }}>
            <StatCard icon={BarChart3} label="总请求" value={metrics.requests_total} color="var(--accent)" />
            <StatCard icon={CheckCircle2} label="成功" value={metrics.requests_success} color="var(--success)" />
            <StatCard icon={XCircle} label="失败" value={metrics.requests_failed} color="var(--danger)" />
            <StatCard
              icon={Zap}
              label="总 Token"
              value={metrics.tokens_total >= 1000 ? `${(metrics.tokens_total / 1000).toFixed(1)}K` : metrics.tokens_total}
              sub={`入: ${metrics.tokens_in} / 出: ${metrics.tokens_out}`}
            />
            <StatCard
              icon={Timer}
              label="平均延迟"
              value={`${metrics.request_latency.avg_ms.toFixed(0)}ms`}
              sub={`P95: ${metrics.request_latency.p95_ms.toFixed(0)}ms`}
              color="var(--warning)"
            />
            <StatCard icon={Clock} label="运行时间" value={formatUptime(metrics.uptime)} />
          </div>
        </BlurFade>
      )}

      {/* Bottom: Tables */}
      {metrics && (
        <BlurFade delay={0.06}>
          <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: 16, marginBottom: 16 }}>
            <div
              className="rounded-xl border card-hover"
              style={{ background: "var(--bg-card)", borderColor: "var(--border)", padding: "20px" }}
            >
              <div style={{
                display: "flex", alignItems: "center", gap: 8, marginBottom: 16,
                fontSize: 13, fontWeight: 500, color: "var(--text-secondary)",
              }}>
                <Sparkles size={14} style={{ color: "var(--accent)" }} />
                技能性能
              </div>
              <SkillTable skills={metrics.skills} />
            </div>

            <div
              className="rounded-xl border card-hover"
              style={{ background: "var(--bg-card)", borderColor: "var(--border)", padding: "20px" }}
            >
              <div style={{
                display: "flex", alignItems: "center", gap: 8, marginBottom: 16,
                fontSize: 13, fontWeight: 500, color: "var(--text-secondary)",
              }}>
                <AlertTriangle size={14} style={{ color: "var(--danger)" }} />
                近期错误
              </div>
              <ErrorList errors={metrics.recent_errors} />
            </div>
          </div>
        </BlurFade>
      )}

      {/* Quote */}
      <BlurFade delay={0.08}>
        <div
          className="rounded-xl border card-hover"
          style={{
            background: "var(--bg-card)",
            borderColor: "var(--border)",
            padding: "24px",
            textAlign: "center",
          }}
        >
          <Quote size={20} style={{ color: "var(--accent)", opacity: 0.4, margin: "0 auto 12px" }} />
          <p style={{ fontSize: 13, fontStyle: "italic", color: "var(--text-secondary)" }}>
            "凡是过往，皆为序章。"
          </p>
          <p style={{ fontSize: 11, color: "var(--text-muted)", marginTop: 8 }}>
            —— 云雀 · 内心独白
          </p>
        </div>
      </BlurFade>
    </div>
  );
}

function InfoRow({
  icon: Icon,
  label,
  value,
  tag,
  onTagClick,
}: {
  icon: React.ElementType;
  label: string;
  value: string;
  tag?: string;
  onTagClick?: () => void;
}) {
  return (
    <div style={{ display: "flex", alignItems: "center", gap: 8, fontSize: 12 }}>
      <Icon size={13} style={{ color: "var(--text-muted)", flexShrink: 0 }} />
      <span style={{ color: "var(--text-muted)", minWidth: 48 }}>{label}</span>
      <span style={{ fontWeight: 500 }}>{value}</span>
      {tag && (
        <button
          onClick={onTagClick}
          style={{
            padding: "1px 6px",
            borderRadius: 4,
            fontSize: 10,
            fontFamily: "monospace",
            background: "var(--bg-hover)",
            color: "var(--accent)",
            border: "none",
            cursor: "pointer",
            transition: "background 0.15s",
          }}
        >
          {tag}
        </button>
      )}
    </div>
  );
}

function DetailItem({ label, value }: { label: string; value: string }) {
  return (
    <div style={{ padding: 12, borderRadius: 8, background: "var(--bg-hover)" }}>
      <div style={{ fontSize: 10, color: "var(--text-muted)", marginBottom: 4 }}>{label}</div>
      <div style={{ fontSize: 12, fontFamily: "monospace", overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>{value}</div>
    </div>
  );
}
