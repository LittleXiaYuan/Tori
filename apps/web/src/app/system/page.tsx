"use client";

import { useCallback, useState } from "react";
import { useSearchParams } from "next/navigation";
import { Button, Card, Chip, Spinner } from "@heroui/react";
import { Segment } from "@heroui-pro/react";
import {
  Activity,
  BarChart3,
  Blocks,
  CircleDollarSign,
  ClipboardCheck,
  FileSearch,
  RefreshCw,
  RotateCcw,
  Server,
  ShieldCheck,
  SlidersHorizontal,
  TerminalSquare,
  Zap,
} from "lucide-react";
import Link from "next/link";
import PageHeader from "@/components/page-header";
import EmptyState from "@/components/empty-state";
import { useApiData } from "@/lib/use-api-data";
import { usePolling } from "@/lib/use-polling";
import { formatErrorMessage } from "@/lib/error-utils";
import {
  fetchSystemStats,
  fetchTrustScores,
  fetchNotifyChannels,
  fetchStateSnapshot,
  fetchCostSummary,
  fetchCostBreakdown,
  fetchModules,
  trustGrant,
  trustReset,
  type SystemStats,
  type TrustEntry,
  type NotifyChannel,
  type StateSnapshot,
  type CostSummary,
  type CostBreakdown,
  type ModulesResponse,
} from "@/lib/system-page-client";

type SystemTab = "metrics" | "trust" | "channels" | "state" | "cost" | "modules";

const TAB_LABELS: Array<[SystemTab, string]> = [
  ["metrics", "指标"],
  ["trust", "信任"],
  ["channels", "渠道"],
  ["state", "状态"],
  ["cost", "成本"],
  ["modules", "模块"],
];

// ─── Helpers ───────────────────────────────────────────────────────────────

const LEVEL_COLORS: Record<string, string> = {
  shell: "#ef4444",
  network: "#f59e0b",
  write: "#3b82f6",
  "read-only": "#6b7280",
};
const LEVEL_LABELS: Record<string, string> = {
  shell: "Shell (80+)",
  network: "Network (60+)",
  write: "Write (30+)",
  "read-only": "ReadOnly (0-29)",
};

function permLevel(score: number): string {
  if (score >= 80) return "shell";
  if (score >= 60) return "network";
  if (score >= 30) return "write";
  return "read-only";
}

function fmtUsd(n?: number): string {
  if (n == null) return "—";
  return `$${n.toFixed(4)}`;
}

function fmtNum(n?: number): string {
  if (n == null) return "—";
  return n.toLocaleString();
}

// ─── Tab panels ────────────────────────────────────────────────────────────

function MetricsTab() {
  const { data: metrics, loading, refresh } = useApiData<SystemStats>(
    fetchSystemStats,
    {},
  );
  usePolling(refresh, 5000);

  const latency = (metrics?.request_latency || {}) as Record<string, number>;

  if (loading) {
    return <div className="flex items-center justify-center py-24"><Spinner size="lg" /></div>;
  }

  return (
    <div className="space-y-6">
      <div className="flex justify-end">
        <Button size="sm" variant="ghost" onPress={refresh}><RefreshCw size={14} /></Button>
      </div>
      <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
        {[
          { label: "请求总数", value: fmtNum(metrics?.requests_total), icon: <Activity size={14} /> },
          { label: "Token 消耗", value: fmtNum(metrics?.tokens_total), icon: <Zap size={14} /> },
          { label: "技能数", value: fmtNum(metrics?.skills), icon: <Blocks size={14} /> },
          { label: "会话数", value: fmtNum(metrics?.conversations), icon: <Server size={14} /> },
        ].map((kpi) => (
          <Card key={kpi.label} className="p-4">
            <div className="flex items-center gap-2 mb-1 text-xs" style={{ color: "var(--yunque-text-muted)" }}>
              {kpi.icon}{kpi.label}
            </div>
            <div className="text-xl font-semibold tabular-nums" style={{ color: "var(--yunque-text)" }}>{kpi.value}</div>
          </Card>
        ))}
      </div>

      {Object.keys(latency).length > 0 && (
        <Card className="p-5">
          <div className="text-sm font-medium mb-4" style={{ color: "var(--yunque-text)" }}>延迟分布</div>
          <div className="space-y-3">
            {(["p50_ms", "p90_ms", "p95_ms", "p99_ms", "avg_ms"] as const).map((key) => {
              const val = latency[key] as number | undefined;
              const max = Math.max(latency.p99_ms ?? 1000, 1);
              return (
                <div key={key} className="flex items-center gap-3 text-xs">
                  <span className="w-10 text-right tabular-nums" style={{ color: "var(--yunque-text-muted)" }}>
                    {key.replace("_ms", "").toUpperCase()}
                  </span>
                  <div className="flex-1 h-1.5 rounded-full overflow-hidden" style={{ background: "var(--yunque-bg-muted)" }}>
                    <div
                      className="h-full rounded-full transition-all"
                      style={{ width: `${Math.min(((val ?? 0) / max) * 100, 100)}%`, background: "var(--yunque-accent)" }}
                    />
                  </div>
                  <span className="w-16 text-right tabular-nums" style={{ color: "var(--yunque-text)" }}>
                    {val != null ? `${val.toFixed(0)}ms` : "—"}
                  </span>
                </div>
              );
            })}
          </div>
        </Card>
      )}

      {(metrics?.recent_errors?.length ?? 0) > 0 && (
        <Card className="p-5">
          <div className="text-sm font-medium mb-3" style={{ color: "var(--yunque-danger)" }}>最近错误</div>
          <div className="space-y-2">
            {metrics!.recent_errors!.map((err, i) => (
              <div key={i} className="flex items-center justify-between p-2 rounded-lg text-sm" style={{ background: "rgba(239,68,68,0.05)" }}>
                <span className="truncate flex-1" style={{ color: "var(--yunque-text)" }}>{err.message}</span>
                <Chip size="sm" style={{ background: "rgba(239,68,68,0.12)", color: "var(--yunque-danger)", flexShrink: 0 }}>{err.count}x</Chip>
              </div>
            ))}
          </div>
        </Card>
      )}
    </div>
  );
}

function TrustTab() {
  const { data: resp, loading, refresh } = useApiData<{ scores: Record<string, TrustEntry> }>(
    fetchTrustScores,
    { scores: {} },
  );
  const [acting, setActing] = useState("");
  const [error, setError] = useState("");

  const doGrant = useCallback(async (slug: string) => {
    setActing(slug);
    try { await trustGrant(slug); refresh(); } catch (e) { setError(formatErrorMessage(e, "授权失败")); }
    setActing("");
  }, [refresh]);

  const doReset = useCallback(async (slug: string) => {
    setActing(slug);
    try { await trustReset(slug); refresh(); } catch (e) { setError(formatErrorMessage(e, "重置失败")); }
    setActing("");
  }, [refresh]);

  const entries = Object.entries(resp.scores).sort((a, b) => b[1].score - a[1].score);

  const governanceLinks = [
    { href: "/approvals", label: "处理待审批", desc: "高风险动作先到这里确认。", icon: <ClipboardCheck size={16} /> },
    { href: "/audit", label: "查看审计", desc: "回看操作日志与审计链。", icon: <FileSearch size={16} /> },
    { href: "/system?tab=metrics", label: "观察指标", desc: "检查用量、指标和健康状态。", icon: <Activity size={16} /> },
    { href: "/settings/providers", label: "管理模型", desc: "配置 Provider 与模型连接。", icon: <SlidersHorizontal size={16} /> },
    { href: "/tools", label: "查看工具执行", desc: "查看终端/工具会话。", icon: <TerminalSquare size={16} /> },
  ];

  if (loading) {
    return <div className="flex items-center justify-center py-24"><Spinner size="lg" /></div>;
  }

  return (
    <div className="space-y-5">
      {error && <div className="text-xs rounded-lg px-3 py-2.5" style={{ background: "rgba(239,68,68,0.08)", color: "var(--yunque-danger)" }}>{error}</div>}

      <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-5">
        {governanceLinks.map((item) => (
          <Link key={item.href} href={item.href} className="block">
            <Card className="h-full p-4 transition-colors hover:bg-[var(--yunque-bg-muted)]">
              <div className="flex items-center gap-2 text-sm font-semibold" style={{ color: "var(--yunque-text)" }}>{item.icon}{item.label}</div>
              <div className="mt-2 text-xs leading-5" style={{ color: "var(--yunque-text-muted)" }}>{item.desc}</div>
            </Card>
          </Link>
        ))}
      </div>

      <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
        {Object.entries(LEVEL_LABELS).map(([key, label]) => {
          const count = entries.filter(([, e]) => permLevel(e.score) === key).length;
          return (
            <Card key={key} className="p-4 flex items-center gap-3">
              <div className="w-2.5 h-10 rounded-full" style={{ background: LEVEL_COLORS[key] }} />
              <div>
                <div className="text-xl font-semibold tabular-nums" style={{ color: "var(--yunque-text)" }}>{count}</div>
                <div className="text-xs" style={{ color: "var(--yunque-text-muted)" }}>{label}</div>
              </div>
            </Card>
          );
        })}
      </div>

      <Card className="overflow-hidden p-0">
        {entries.length === 0 ? (
          <EmptyState icon={<ShieldCheck size={36} />} title="暂无信任记录" description="尚无信任分数条目。" />
        ) : (
          <div className="divide-y" style={{ borderColor: "var(--yunque-border)" }}>
            {entries.map(([slug, entry]) => {
              const level = permLevel(entry.score);
              return (
                <div key={slug} className="flex items-center gap-3 px-4 py-2.5">
                  <div className="w-2 h-2 rounded-full shrink-0" style={{ background: LEVEL_COLORS[level] }} />
                  <span className="text-sm font-medium truncate flex-1" style={{ color: "var(--yunque-text)" }}>{slug}</span>
                  <div className="w-24 h-1.5 rounded-full overflow-hidden" style={{ background: "var(--yunque-bg-muted)" }}>
                    <div className="h-full rounded-full" style={{ width: `${entry.score}%`, background: LEVEL_COLORS[level] }} />
                  </div>
                  <Chip size="sm" style={{ background: `${LEVEL_COLORS[level]}15`, color: LEVEL_COLORS[level] }}>{entry.score}</Chip>
                  <Button size="sm" variant="ghost" isDisabled={acting === slug} onPress={() => doGrant(slug)} aria-label="授权"><Zap size={11} /></Button>
                  <Button size="sm" variant="ghost" isDisabled={acting === slug} onPress={() => doReset(slug)} aria-label="重置"><RotateCcw size={11} /></Button>
                </div>
              );
            })}
          </div>
        )}
      </Card>
    </div>
  );
}

function ChannelsTab() {
  const { data, loading, refresh } = useApiData<{ channels: NotifyChannel[] }>(
    fetchNotifyChannels,
    { channels: [] },
  );
  const channels = data.channels ?? [];

  if (loading) {
    return <div className="flex items-center justify-center py-24"><Spinner size="lg" /></div>;
  }

  return (
    <div className="space-y-4">
      <div className="flex justify-end">
        <Button size="sm" variant="ghost" onPress={refresh}><RefreshCw size={14} /></Button>
      </div>
      {channels.length === 0 ? (
        <EmptyState icon={<Server size={36} />} title="暂无渠道配置" description="在设置里添加消息渠道后可在此查看状态。" />
      ) : (
        <div className="grid gap-3 md:grid-cols-2">
          {channels.map((ch, i) => (
            <Card key={ch.id ?? i} className="p-4 flex items-center gap-3">
              <div
                className="w-2.5 h-2.5 rounded-full shrink-0"
                style={{ background: ch.enabled ? "var(--yunque-success)" : "var(--yunque-text-muted)" }}
              />
              <div className="flex-1 min-w-0">
                <div className="text-sm font-medium truncate" style={{ color: "var(--yunque-text)" }}>{ch.name || ch.type}</div>
                <div className="text-xs" style={{ color: "var(--yunque-text-muted)" }}>{ch.type}</div>
              </div>
              <Chip size="sm" color={ch.enabled ? "success" : "default"}>{ch.enabled ? "启用" : "禁用"}</Chip>
            </Card>
          ))}
        </div>
      )}
    </div>
  );
}

function StateTab() {
  const { data, loading, refresh } = useApiData<StateSnapshot>(
    fetchStateSnapshot,
    {},
  );

  if (loading) {
    return <div className="flex items-center justify-center py-24"><Spinner size="lg" /></div>;
  }

  const goals = data.goals ?? [];
  const resources = data.resources ?? [];
  const caps = data.capabilities;

  return (
    <div className="space-y-5">
      <div className="flex justify-end">
        <Button size="sm" variant="ghost" onPress={refresh}><RefreshCw size={14} /></Button>
      </div>

      {data.focus && (
        <Card className="p-4">
          <div className="text-xs font-medium mb-1" style={{ color: "var(--yunque-text-muted)" }}>当前焦点</div>
          <div className="text-sm" style={{ color: "var(--yunque-text)" }}>{data.focus}</div>
        </Card>
      )}

      {caps && (
        <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
          {[
            { label: "技能总数", value: fmtNum(caps.total_skills) },
            { label: "动态技能", value: fmtNum(caps.dynamic_skills?.length) },
            { label: "未解差距", value: fmtNum(caps.unresolved_gaps) },
            { label: "话题数", value: fmtNum(data.topics?.length) },
          ].map((kpi) => (
            <Card key={kpi.label} className="p-4">
              <div className="text-xs mb-1" style={{ color: "var(--yunque-text-muted)" }}>{kpi.label}</div>
              <div className="text-xl font-semibold tabular-nums" style={{ color: "var(--yunque-text)" }}>{kpi.value}</div>
            </Card>
          ))}
        </div>
      )}

      {goals.length > 0 && (
        <Card className="overflow-hidden p-0">
          <div className="px-4 py-3 border-b text-sm font-medium" style={{ borderColor: "var(--yunque-border)", color: "var(--yunque-text)" }}>
            当前目标 ({goals.length})
          </div>
          <div className="divide-y" style={{ borderColor: "var(--yunque-border)" }}>
            {goals.slice(0, 10).map((g, i) => (
              <div key={g.id ?? i} className="flex items-center gap-3 px-4 py-2.5">
                <Chip size="sm" color={g.status === "done" ? "success" : g.status === "active" ? "accent" : "default"}>{g.status ?? "—"}</Chip>
                <span className="text-sm flex-1 truncate" style={{ color: "var(--yunque-text)" }}>{g.title}</span>
                {g.progress != null && (
                  <span className="text-xs tabular-nums" style={{ color: "var(--yunque-text-muted)" }}>{g.progress}%</span>
                )}
              </div>
            ))}
          </div>
        </Card>
      )}

      {resources.length > 0 && (
        <Card className="overflow-hidden p-0">
          <div className="px-4 py-3 border-b text-sm font-medium" style={{ borderColor: "var(--yunque-border)", color: "var(--yunque-text)" }}>
            资源快照 ({resources.length})
          </div>
          <div className="divide-y" style={{ borderColor: "var(--yunque-border)" }}>
            {resources.slice(0, 8).map((r, i) => (
              <div key={r.id ?? i} className="flex items-center gap-3 px-4 py-2.5">
                <span className="text-xs w-16 truncate" style={{ color: "var(--yunque-text-muted)" }}>{r.type ?? "file"}</span>
                <span className="text-sm flex-1 truncate font-mono text-xs" style={{ color: "var(--yunque-text)" }}>{r.path}</span>
                {r.status && <Chip size="sm" color={r.status === "ready" ? "success" : "default"}>{r.status}</Chip>}
              </div>
            ))}
          </div>
        </Card>
      )}

      {goals.length === 0 && resources.length === 0 && !data.focus && (
        <EmptyState icon={<Blocks size={36} />} title="状态为空" description="当前没有活跃目标或资源快照。" />
      )}
    </div>
  );
}

function CostTab() {
  const { data: summary, loading: summaryLoading, refresh: refreshSummary } = useApiData<CostSummary>(
    fetchCostSummary,
    {},
  );
  const { data: breakdown, loading: breakdownLoading } = useApiData<CostBreakdown>(
    fetchCostBreakdown,
    {},
  );

  if (summaryLoading || breakdownLoading) {
    return <div className="flex items-center justify-center py-24"><Spinner size="lg" /></div>;
  }

  const byProvider = breakdown.by_provider ?? {};
  const byRunnerType = breakdown.by_runner_type ?? {};

  return (
    <div className="space-y-5">
      <div className="flex justify-end">
        <Button size="sm" variant="ghost" onPress={refreshSummary}><RefreshCw size={14} /></Button>
      </div>

      <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
        <Card className="p-4">
          <div className="text-xs mb-1" style={{ color: "var(--yunque-text-muted)" }}>今日消耗</div>
          <div className="text-2xl font-semibold tabular-nums" style={{ color: "var(--yunque-text)" }}>{fmtUsd(summary.today_cost)}</div>
        </Card>
        <Card className="p-4">
          <div className="text-xs mb-1" style={{ color: "var(--yunque-text-muted)" }}>本月消耗</div>
          <div className="text-2xl font-semibold tabular-nums" style={{ color: "var(--yunque-text)" }}>{fmtUsd(summary.month_cost)}</div>
        </Card>
        <Card className="p-4">
          <div className="text-xs mb-1" style={{ color: "var(--yunque-text-muted)" }}>状态</div>
          <div className="text-sm font-medium" style={{ color: summary.status === "ok" ? "var(--yunque-success)" : "var(--yunque-text)" }}>
            {summary.status ?? "—"}
          </div>
        </Card>
      </div>

      {Object.keys(byProvider).length > 0 && (
        <Card className="p-5">
          <div className="text-sm font-medium mb-3" style={{ color: "var(--yunque-text)" }}>按提供商</div>
          <div className="space-y-2">
            {Object.entries(byProvider).sort(([, a], [, b]) => b - a).map(([k, v]) => (
              <div key={k} className="flex items-center justify-between text-sm">
                <span className="truncate" style={{ color: "var(--yunque-text)" }}>{k}</span>
                <span className="tabular-nums font-mono text-xs" style={{ color: "var(--yunque-text-muted)" }}>{fmtUsd(v)}</span>
              </div>
            ))}
          </div>
        </Card>
      )}

      {Object.keys(byRunnerType).length > 0 && (
        <Card className="p-5">
          <div className="text-sm font-medium mb-3" style={{ color: "var(--yunque-text)" }}>按运行类型</div>
          <div className="space-y-2">
            {Object.entries(byRunnerType).sort(([, a], [, b]) => b - a).map(([k, v]) => (
              <div key={k} className="flex items-center justify-between text-sm">
                <span style={{ color: "var(--yunque-text)" }}>{k}</span>
                <span className="tabular-nums font-mono text-xs" style={{ color: "var(--yunque-text-muted)" }}>{fmtUsd(v)}</span>
              </div>
            ))}
          </div>
        </Card>
      )}

      {!summary.today_cost && !summary.month_cost && Object.keys(byProvider).length === 0 && (
        <EmptyState icon={<CircleDollarSign size={36} />} title="暂无成本数据" description="成本数据尚未采集，或当前未配置计费追踪。" />
      )}
    </div>
  );
}

function ModulesTab() {
  const { data, loading, refresh } = useApiData<ModulesResponse>(
    fetchModules,
    { modules: [] },
  );
  const modules = data.modules ?? [];

  if (loading) {
    return <div className="flex items-center justify-center py-24"><Spinner size="lg" /></div>;
  }

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        {data.profile && (
          <span className="text-xs" style={{ color: "var(--yunque-text-muted)" }}>
            Profile: <span style={{ color: "var(--yunque-text)" }}>{data.profile}</span>
          </span>
        )}
        <Button size="sm" variant="ghost" onPress={refresh}><RefreshCw size={14} /></Button>
      </div>

      {modules.length === 0 ? (
        <EmptyState icon={<Blocks size={36} />} title="暂无模块数据" description="当前没有已加载的运行时模块信息。" />
      ) : (
        <Card className="overflow-hidden p-0">
          <div className="divide-y" style={{ borderColor: "var(--yunque-border)" }}>
            {modules.map((m, i) => {
              const name = m.name ?? m.id ?? `module-${i}`;
              const enabled = m.enabled ?? (m.status === "active" || m.status === "enabled");
              return (
                <div key={name} className="flex items-center gap-3 px-4 py-2.5">
                  <div
                    className="w-2 h-2 rounded-full shrink-0"
                    style={{ background: enabled ? "var(--yunque-success)" : "var(--yunque-text-muted)" }}
                  />
                  <span className="text-sm flex-1 truncate" style={{ color: "var(--yunque-text)" }}>{name}</span>
                  {m.status && (
                    <Chip size="sm" color={m.status === "active" || m.status === "enabled" ? "success" : "default"}>
                      {m.status}
                    </Chip>
                  )}
                </div>
              );
            })}
          </div>
        </Card>
      )}
    </div>
  );
}

// ─── Page ───────────────────────────────────────────────────────────────────

export default function SystemPage() {
  const searchParams = useSearchParams();
  const initialTab = (searchParams.get("tab") as SystemTab | null) ?? "metrics";
  const [tab, setTab] = useState<SystemTab>(initialTab);

  return (
    <div className="flex flex-col min-h-0" style={{ height: "100%", overflowY: "auto" }}>
      <div className="p-5 border-b" style={{ borderColor: "var(--yunque-border)" }}>
        <PageHeader
          icon={<Server size={20} />}
          title="系统状态"
          description="指标、信任、渠道、状态、成本、模块。"
        />
        <div className="mt-4">
          <Segment
            size="sm"
            aria-label="系统状态标签页"
            selectedKey={tab}
            onSelectionChange={(key) => {
              if (key != null && key !== "") setTab(key as SystemTab);
            }}
          >
            {TAB_LABELS.map(([key, label]) => (
              <Segment.Item key={key} id={key}>{label}</Segment.Item>
            ))}
          </Segment>
        </div>
      </div>

      <div className="p-5 flex-1 min-h-0">
        {tab === "metrics" && <MetricsTab />}
        {tab === "trust" && <TrustTab />}
        {tab === "channels" && <ChannelsTab />}
        {tab === "state" && <StateTab />}
        {tab === "cost" && <CostTab />}
        {tab === "modules" && <ModulesTab />}
      </div>
    </div>
  );
}
