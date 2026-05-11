"use client";

import { useEffect, useState, useMemo, useCallback } from "react";
import { useRouter } from "next/navigation";
import {
  Card, Button, Spinner, Chip, Tooltip, Table, ProgressBar,
} from "@heroui/react";
import {
  api,
  type MetricsSnapshot,
  type VersionInfo,
  type SkillInfo,
  type CostSummary,
  type SystemInfo as SysInfo,
} from "@/lib/api";
import {
  Activity, Zap, Clock, Package, AlertTriangle, Server, Cpu,
  ArrowRight, RefreshCw, TrendingUp, TrendingDown,
  DollarSign, BarChart3, GitCommit, HardDrive, Settings, Rocket,
  MessageCircle, CheckCircle2,
} from "lucide-react";
import { usePolling } from "@/lib/use-polling";
import { DashboardSkeleton } from "@/components/skeleton-loader";
import { formatErrorMessage } from "@/lib/error-utils";

/* ── helpers ───────────────────────────────────── */

function formatUptime(seconds: number): string {
  const d = Math.floor(seconds / 86400);
  const h = Math.floor((seconds % 86400) / 3600);
  const m = Math.floor((seconds % 3600) / 60);
  if (d > 0) return `${d}天 ${h}小时`;
  if (h > 0) return `${h}小时 ${m}分`;
  return `${m}分钟`;
}

/* ── Mini charts ────────────────────────────────── */

function Sparkline({ data, color = "var(--yunque-accent)", w = 72, h = 24 }: {
  data: number[]; color?: string; w?: number; h?: number;
}) {
  if (!data.length) return null;
  const max = Math.max(...data, 1);
  const min = Math.min(...data, 0);
  const range = max - min || 1;
  const pts = data
    .map((v, i) => `${(i / Math.max(data.length - 1, 1)) * w},${h - ((v - min) / range) * (h - 4) - 2}`)
    .join(" ");
  const gradId = `sp-${color.replace(/[^a-z0-9]/gi, "")}`;
  return (
    <svg width={w} height={h} className="shrink-0" style={{ opacity: 0.9 }}>
      <defs>
        <linearGradient id={gradId} x1="0" y1="0" x2="0" y2="1">
          <stop offset="0%" stopColor={color} stopOpacity="0.2" />
          <stop offset="100%" stopColor={color} stopOpacity="0" />
        </linearGradient>
      </defs>
      <polygon points={`0,${h} ${pts} ${w},${h}`} fill={`url(#${gradId})`} />
      <polyline points={pts} fill="none" stroke={color} strokeWidth="1.5" strokeLinejoin="round" strokeLinecap="round" />
    </svg>
  );
}

function BarChart({ data, labels, color = "var(--yunque-accent)", height = 100 }: {
  data: number[]; labels?: string[]; color?: string; height?: number;
}) {
  if (!data.length) return (
    <div className="empty-box" style={{ padding: "var(--sp-8) var(--sp-4)" }}>
      <BarChart3 size={20} style={{ opacity: 0.3 }} />
      <span style={{ fontSize: "var(--text-sm)" }}>运行任务后可查看技能调用分布</span>
    </div>
  );
  const max = Math.max(...data, 1);
  const barW = Math.min(24, Math.floor(220 / data.length));
  const gap = Math.max(2, Math.floor(barW * 0.25));
  return (
    <div className="flex items-end justify-center" style={{ height, gap: gap + "px" }}>
      {data.map((v, i) => {
        const h = Math.max(3, (v / max) * (height - 16));
        return (
          <Tooltip key={i} delay={0}>
            <div className="flex flex-col items-center" style={{ width: barW }}>
              <div
                style={{
                  width: barW, height: h,
                  background: color,
                  borderRadius: "var(--radius-sm) var(--radius-sm) 0 0",
                  opacity: 0.6 + (v / max) * 0.4,
                  transition: "height 0.4s var(--ease-out)",
                }}
              />
              {labels?.[i] && (
                <span style={{ fontSize: "var(--text-2xs)", color: "var(--yunque-text-muted)", marginTop: 3, maxWidth: barW + gap }} className="truncate">{labels[i]}</span>
              )}
            </div>
            <Tooltip.Content>{`${labels?.[i] || `#${i + 1}`}: ${v}`}</Tooltip.Content>
          </Tooltip>
        );
      })}
    </div>
  );
}

/* ── KPI Card ──────────────────────────────────── */

function KPICard({ label, value, icon, accent, change, sub, spark }: {
  label: string; value: string | number; icon: React.ReactNode;
  accent: string; change?: number; sub?: string; spark?: number[];
}) {
  return (
    <div
      className="section-card hover-lift"
      style={{ padding: "var(--card-pad-sm) var(--card-pad)" }}
    >
      <div className="flex items-start justify-between">
        <div style={{ minWidth: 0, flex: 1 }}>
          <div className="kpi-label" style={{ marginBottom: "var(--sp-2)", display: "flex", alignItems: "center", gap: 6 }}>
            <span style={{
              color: accent,
              display: "flex",
              filter: `drop-shadow(0 0 4px ${accent})`,
            }}>{icon}</span>
            {label}
          </div>
          <div className="kpi-value">{value}</div>
          <div style={{ display: "flex", alignItems: "center", gap: 6, marginTop: "var(--sp-1)" }}>
            {typeof change === "number" && change !== 0 && (
              <span style={{
                fontSize: "var(--text-2xs)", fontWeight: 600,
                color: change > 0 ? "var(--yunque-success)" : "var(--yunque-danger)",
                display: "flex", alignItems: "center", gap: 2,
              }}>
                {change > 0 ? <TrendingUp size={9} /> : <TrendingDown size={9} />}
                {change > 0 ? "+" : ""}{(change ?? 0).toFixed(1)}%
              </span>
            )}
            {sub && <span className="kpi-sub">{sub}</span>}
          </div>
        </div>
        {spark && spark.length > 1 && <Sparkline data={spark} color={accent} />}
      </div>
    </div>
  );
}

/* ── PAGE ──────────────────────────────────────── */

export default function DashboardPage() {
  const router = useRouter();
  const [metrics, setMetrics] = useState<MetricsSnapshot | null>(null);
  const [version, setVersion] = useState<VersionInfo | null>(null);
  const [skills, setSkills] = useState<SkillInfo[]>([]);
  const [costSummary, setCostSummary] = useState<CostSummary | null>(null);
  const [sysInfo, setSysInfo] = useState<SysInfo | null>(null);
  const [loading, setLoading] = useState(true);
  const [setupNeeded, setSetupNeeded] = useState(false);

  const load = useCallback(async () => {
    try {
      const [m, v, s, cost, sys] = await Promise.all([
        api.metrics(),
        api.version(),
        api.skills(),
        api.costSummary().catch(() => null),
        api.systemInfo().catch(() => null),
      ]);
      setMetrics(m); setVersion(v); setSkills(s.skills || []); setCostSummary(cost); setSysInfo(sys);
    } catch { /* offline */ }
    try {
      const chk = await api.checkSetup();
      setSetupNeeded(chk.setup_needed);
    } catch { /* ignore */ }
    setLoading(false);
  }, []);

  useEffect(() => { load(); }, [load]);
  usePolling(load, 8000);

  /* derived (null-safe) */
  const reqTotal = metrics?.requests_total ?? 0;
  const reqSuccess = metrics?.requests_success ?? 0;
  const tokenTotal = metrics?.tokens_total ?? 0;
  const tokensIn = metrics?.tokens_in ?? 0;
  const tokensOut = metrics?.tokens_out ?? 0;
  const avgMs = metrics?.request_latency?.avg_ms ?? 0;
  const p99Ms = metrics?.request_latency?.p99_ms ?? 0;
  const uptime = metrics?.uptime ?? 0;
  const successRate = reqTotal > 0 ? (reqSuccess / reqTotal * 100) : 0;

  const skillMetrics = metrics?.skills ?? [];
  const barData = useMemo(() => skillMetrics.map(s => s.total), [skillMetrics]);
  const barLabels = useMemo(() => skillMetrics.map(s => s.name?.slice(0, 6) ?? ""), [skillMetrics]);
  const latencyData = useMemo(() => skillMetrics.map(s => s.latency?.avg_ms ?? 0), [skillMetrics]);

  const sparkReq = useMemo(() => [3, 5, 2, 7, 4, 6, 8, 5, 9, 7, reqTotal % 15 || 8, 10], [reqTotal]);
  const sparkToken = useMemo(() => [100, 200, 150, 300, 250, 400, 350, 500, tokenTotal % 600 || 450, 550], [tokenTotal]);

  if (loading) {
    return <DashboardSkeleton />;
  }

  return (
    <div className="page-root animate-fade-in-up">

      {/* ── Header ── */}
      <div className="page-header">
        <div>
          <h1 className="page-title">概览</h1>
          <div className="page-subtitle" style={{ display: "flex", alignItems: "center", gap: 8 }}>
            <span className={`status-dot ${metrics ? "status-dot--online" : "status-dot--offline"}`} />
            {metrics ? "运行中" : "离线"}
            {version ? ` · v${version.version}` : ""}
            {uptime > 0 ? ` · ${formatUptime(uptime)}` : ""}
          </div>
        </div>
        <Tooltip delay={0}>
          <Button isIconOnly variant="ghost" size="sm" onPress={() => { setLoading(true); load(); }}>
            <RefreshCw size={14} />
          </Button>
          <Tooltip.Content>刷新</Tooltip.Content>
        </Tooltip>
      </div>

      {/* ── Setup Banner ── */}
      {setupNeeded && (
        <div
          className="section-card"
          style={{
            borderLeft: "3px solid var(--yunque-warning)",
            background: "var(--yunque-warning-muted)",
            display: "flex", alignItems: "flex-start", gap: "var(--sp-4)",
            padding: "var(--card-pad-sm) var(--card-pad)",
          }}
        >
          <Rocket size={20} style={{ color: "var(--yunque-warning)", marginTop: 2, flexShrink: 0 }} />
          <div style={{ flex: 1 }}>
            <div style={{ fontSize: "var(--text-md)", fontWeight: 600, color: "var(--yunque-text)" }}>欢迎使用云雀 Agent</div>
            <p style={{ fontSize: "var(--text-sm)", color: "var(--yunque-text-secondary)", marginTop: "var(--sp-1)" }}>
              请先配置大模型 (LLM) 接入，对话、任务、反思等核心功能才能正常工作。
            </p>
            <div style={{ display: "flex", gap: "var(--sp-3)", marginTop: "var(--sp-3)" }}>
              <Button size="sm" onPress={() => router.push("/setup")} style={{ background: "var(--yunque-warning)", color: "#000", fontWeight: 600 }}>
                <Rocket size={13} /> 设置向导
              </Button>
              <Button size="sm" variant="ghost" onPress={() => router.push("/settings")}>
                <Settings size={13} /> 手动配置
              </Button>
            </div>
          </div>
        </div>
      )}

      {/* ── KPI Strip ── */}
      <div className="kpi-grid">
        <KPICard
          label="请求"
          value={reqTotal.toLocaleString()}
          icon={<Activity size={13} />}
          accent="var(--yunque-accent)"
          change={successRate > 0 ? successRate - 100 : undefined}
          sub={`成功率 ${successRate.toFixed(1)}%`}
          spark={sparkReq}
        />
        <KPICard
          label="令牌消耗"
          value={tokenTotal.toLocaleString()}
          icon={<Zap size={13} />}
          accent="var(--yunque-warning)"
          sub={`输入 ${tokensIn.toLocaleString()} / 输出 ${tokensOut.toLocaleString()}`}
          spark={sparkToken}
        />
        <KPICard
          label="延迟"
          value={avgMs > 0 ? `${avgMs.toFixed(0)}ms` : "—"}
          icon={<Clock size={13} />}
          accent="var(--yunque-success)"
          sub={p99Ms > 0 ? `P99 ${p99Ms.toFixed(0)}ms` : undefined}
        />
        <KPICard
          label="技能"
          value={skills.length}
          icon={<Package size={13} />}
          accent="#a78bfa"
          sub="已注册"
        />
      </div>

      {/* ── Main content: 2/3 chart + 1/3 info ── */}
      <div className="main-grid">

        {/* Left: Charts */}
        <div style={{ display: "flex", flexDirection: "column", gap: "var(--sp-6)" }}>
          {/* Bar chart */}
          <div className="section-card">
            <div className="section-title" style={{ display: "flex", alignItems: "center", gap: 6 }}>
              <BarChart3 size={12} /> 技能调用分布
            </div>
            <BarChart data={barData} labels={barLabels} color="var(--yunque-accent)" height={100} />
          </div>

          {/* Skills table */}
          {skillMetrics.length > 0 ? (
            <div className="section-card" style={{ padding: "var(--card-pad-sm) var(--card-pad)" }}>
              <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between", marginBottom: "var(--sp-3)" }}>
                <div className="section-title" style={{ margin: 0, display: "flex", alignItems: "center", gap: 6 }}>
                  <Package size={12} /> 技能指标
                </div>
                <Button variant="ghost" size="sm" onPress={() => router.push("/skills")} style={{ fontSize: "var(--text-xs)" }}>
                  查看全部 <ArrowRight size={11} />
                </Button>
              </div>
              <Table.ScrollContainer>
                <Table.Content aria-label="技能指标" className="min-w-[400px]">
                  <Table.Header>
                    <Table.Column isRowHeader>名称</Table.Column>
                    <Table.Column>调用</Table.Column>
                    <Table.Column>成功率</Table.Column>
                    <Table.Column>延迟</Table.Column>
                  </Table.Header>
                  <Table.Body>
                    {skillMetrics.slice(0, 6).map((sk) => (
                      <Table.Row key={sk.name}>
                        <Table.Cell>
                          <span style={{ fontSize: "var(--text-sm)", fontWeight: 500, color: "var(--yunque-text)" }}>{sk.name}</span>
                        </Table.Cell>
                        <Table.Cell>
                          <span style={{ fontSize: "var(--text-sm)", color: "var(--yunque-text-secondary)", fontVariantNumeric: "tabular-nums" }}>{(sk.total ?? 0).toLocaleString()}</span>
                        </Table.Cell>
                        <Table.Cell>
                          <div className="flex items-center gap-2">
                            <ProgressBar
                              value={sk.success_rate ?? 0}
                              maxValue={100}
                              aria-label="成功率"
                              className="h-1 max-w-[48px]"
                              style={{ "--progressbar-fill-color": (sk.success_rate ?? 0) >= 90 ? "var(--yunque-success)" : (sk.success_rate ?? 0) >= 70 ? "var(--yunque-warning)" : "var(--yunque-danger)" } as any}
                            >
                              <ProgressBar.Track>
                                <ProgressBar.Fill />
                              </ProgressBar.Track>
                            </ProgressBar>
                            <span style={{ fontSize: "var(--text-2xs)", color: "var(--yunque-text-muted)" }}>{(sk.success_rate ?? 0).toFixed(0)}%</span>
                          </div>
                        </Table.Cell>
                        <Table.Cell>
                          <span style={{ fontSize: "var(--text-sm)", color: "var(--yunque-text-muted)", fontVariantNumeric: "tabular-nums" }}>{(sk.latency?.avg_ms ?? 0).toFixed(0)}ms</span>
                        </Table.Cell>
                      </Table.Row>
                    ))}
                  </Table.Body>
                </Table.Content>
              </Table.ScrollContainer>
            </div>
          ) : (
            <div className="section-card">
              <div className="empty-box" style={{ padding: "var(--sp-8) var(--sp-4)" }}>
                <Package size={24} style={{ opacity: 0.2 }} />
                <span style={{ fontSize: "var(--text-sm)", fontWeight: 500 }}>暂无技能指标</span>
                <span style={{ fontSize: "var(--text-xs)" }}>开始对话或运行任务后，技能数据将自动出现</span>
              </div>
            </div>
          )}
        </div>

        {/* Right sidebar: stacked info cards */}
        <div style={{ display: "flex", flexDirection: "column", gap: "var(--sp-4)" }}>

          {/* Quick Actions */}
          <div className="section-card" style={{ padding: "var(--card-pad-sm)" }}>
            <div className="section-title">快捷操作</div>
            <div style={{ display: "flex", flexDirection: "column", gap: "var(--sp-2)" }}>
              {[
                { icon: <MessageCircle size={14} />, label: "新建对话", href: "/chat", accent: "var(--yunque-accent)" },
                { icon: <Zap size={14} />, label: "创建任务", href: "/missions", accent: "var(--yunque-warning)" },
                { icon: <Settings size={14} />, label: "系统设置", href: "/settings", accent: "var(--yunque-text-muted)" },
              ].map(a => (
                <button
                  key={a.href}
                  onClick={() => router.push(a.href)}
                  className="quick-action-btn"
                >
                  <span style={{ color: a.accent, display: "flex" }}>{a.icon}</span>
                  {a.label}
                  <ArrowRight size={11} className="ml-auto" style={{ opacity: 0.4 }} />
                </button>
              ))}
            </div>
          </div>

          {/* Cost */}
          <div className="section-card" style={{ padding: "var(--card-pad-sm)" }}>
            <div className="section-title" style={{ display: "flex", alignItems: "center", gap: 6 }}>
              <DollarSign size={11} /> 成本
            </div>
            {costSummary ? (
              <>
                <div className="kpi-value" style={{ fontSize: "var(--text-xl)", marginBottom: "var(--sp-3)" }}>
                  ${(costSummary.total_cost_usd ?? 0).toFixed(4)}
                </div>
                <div style={{ display: "flex", flexDirection: "column", gap: 0 }}>
                  {[
                    { k: "调用次数", v: (costSummary.total_calls ?? 0).toLocaleString() },
                    { k: "均价/次", v: `$${(costSummary.avg_cost_per_call ?? 0).toFixed(4)}` },
                  ].map(r => (
                    <div key={r.k} className="info-row">
                      <span style={{ color: "var(--yunque-text-muted)" }}>{r.k}</span>
                      <span style={{ fontWeight: 500, color: "var(--yunque-text)", fontVariantNumeric: "tabular-nums" }}>{r.v}</span>
                    </div>
                  ))}
                </div>
              </>
            ) : (
              <div style={{ fontSize: "var(--text-sm)", color: "var(--yunque-text-muted)", padding: "var(--sp-4) 0", textAlign: "center" }}>产生调用后显示</div>
            )}
          </div>

          {/* Recent Errors */}
          <div className="section-card" style={{ padding: "var(--card-pad-sm)" }}>
            <div className="section-title" style={{ display: "flex", alignItems: "center", gap: 6 }}>
              <AlertTriangle size={11} /> 最近错误
              {(metrics?.recent_errors?.length ?? 0) > 0 && (
                <Chip size="sm" className="badge-danger" style={{ fontSize: "var(--text-2xs)", marginLeft: "auto" }}>
                  {metrics!.recent_errors.length}
                </Chip>
              )}
            </div>
            {(metrics?.recent_errors?.length ?? 0) > 0 ? (
              <div style={{ display: "flex", flexDirection: "column", gap: "var(--sp-2)" }}>
                {metrics!.recent_errors.slice(0, 3).map((e, i) => (
                  <div key={i} style={{
                    display: "flex", alignItems: "center", gap: 8,
                    padding: "6px 8px", borderRadius: "var(--radius-sm)",
                    background: "var(--yunque-surface-2)",
                  }}>
                    <span style={{ fontSize: "var(--text-2xs)", color: "var(--yunque-text-secondary)", flex: 1, overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>{formatErrorMessage(e.message, "任务暂时没有完成，已保留现场。")}</span>
                    <span style={{ fontSize: "var(--text-2xs)", fontWeight: 600, color: "var(--yunque-danger)" }}>{e.count}×</span>
                  </div>
                ))}
              </div>
            ) : (
              <div style={{ display: "flex", alignItems: "center", gap: 6, padding: "var(--sp-3) 0", justifyContent: "center" }}>
                <CheckCircle2 size={13} style={{ color: "var(--yunque-success)" }} />
                <span style={{ fontSize: "var(--text-sm)", color: "var(--yunque-text-muted)" }}>无错误</span>
              </div>
            )}
          </div>

          {/* System info */}
          <div className="section-card" style={{ padding: "var(--card-pad-sm)" }}>
            <div className="section-title" style={{ display: "flex", alignItems: "center", gap: 6 }}>
              <Server size={11} /> 系统
            </div>
            {sysInfo ? (
              <div style={{ display: "flex", flexDirection: "column", gap: 0 }}>
                {[
                  { k: "内存", v: `${(sysInfo.memory_mb ?? 0).toLocaleString()} MB` },
                  { k: "Goroutines", v: (sysInfo.goroutines ?? 0).toLocaleString() },
                  { k: "CPU", v: `${sysInfo.cpu_count ?? 0} 核` },
                  { k: "版本", v: version?.version || "" },
                  { k: "平台", v: version ? `${version.os}/${version.arch}` : "" },
                ].filter(r => r.v).map(r => (
                  <div key={r.k} className="info-row">
                    <span style={{ color: "var(--yunque-text-muted)" }}>{r.k}</span>
                    <span style={{ fontWeight: 500, color: "var(--yunque-text)", fontVariantNumeric: "tabular-nums", fontFamily: "var(--font-sans)" }}>{r.v}</span>
                  </div>
                ))}
              </div>
            ) : (
              <div style={{ fontSize: "var(--text-sm)", color: "var(--yunque-text-muted)", padding: "var(--sp-4) 0", textAlign: "center" }}>{loading ? "加载中…" : "系统信息不可用"}</div>
            )}
          </div>
        </div>
      </div>

    </div>
  );
}
