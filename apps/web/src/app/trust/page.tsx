"use client";

import { useState } from "react";
import Link from "next/link";
import { api, type TrustEntry } from "@/lib/api";
import { Card, Button, Spinner, Tooltip, Chip, ProgressBar } from "@heroui/react";
import { Activity, ClipboardCheck, FileSearch, ShieldCheck, SlidersHorizontal, TerminalSquare, Zap, RotateCcw, RefreshCw } from "lucide-react";
import { useApiData } from "@/lib/use-api-data";
import { formatErrorMessage } from "@/lib/error-utils";

const LEVEL_COLORS: Record<string, string> = {
  shell: "#ef4444", network: "#f59e0b", write: "#3b82f6", "read-only": "#6b7280",
};
const LEVEL_LABELS: Record<string, string> = {
  shell: "Shell (80+)", network: "Network (60+)", write: "Write (30+)", "read-only": "ReadOnly (0-29)",
};
function permLevel(score: number): string {
  if (score >= 80) return "shell";
  if (score >= 60) return "network";
  if (score >= 30) return "write";
  return "read-only";
}

const governanceLinks = [
  {
    href: "/approvals",
    label: "处理待审批",
    desc: "高风险工具、浏览器、远程包或写入动作先到这里确认。",
    icon: <ClipboardCheck size={16} />,
  },
  {
    href: "/audit",
    label: "查看审计",
    desc: "回看谁触发了什么操作，以及审计链是否能验证。",
    icon: <FileSearch size={16} />,
  },
  {
    href: "/metrics",
    label: "观察健康",
    desc: "检查用量、指标和服务健康，排查运行状态。",
    icon: <Activity size={16} />,
  },
  {
    href: "/settings/providers",
    label: "管理模型",
    desc: "配置 Provider、切换模型或测试连接。",
    icon: <SlidersHorizontal size={16} />,
  },
  {
    href: "/tools",
    label: "查看工具执行",
    desc: "查看终端/工具会话，必要时停止运行中的任务。",
    icon: <TerminalSquare size={16} />,
  },
];

export default function TrustPage() {
  const { data: scores, loading, refresh } = useApiData(
    async () => { const r = await api.trustScores(); return r.scores || {}; },
    {} as Record<string, TrustEntry>,
  );
  const [acting, setActing] = useState("");
  const [error, setError] = useState("");

  const handleGrant = async (slug: string) => {
    setActing(slug);
    try { await api.trustGrant(slug); refresh(); } catch (e: unknown) { setError(formatErrorMessage(e, "授权失败")); }
    setActing("");
  };

  const handleReset = async (slug: string) => {
    setActing(slug);
    try { await api.trustReset(slug); refresh(); } catch (e: unknown) { setError(formatErrorMessage(e, "重置失败")); }
    setActing("");
  };

  const entries = Object.entries(scores).sort((a, b) => b[1].score - a[1].score);

  if (loading) return <div className="flex-1 flex items-center justify-center"><Spinner size="lg" /></div>;

  return (
    <div className="page-root space-y-5 animate-fade-in-up" style={{ color: "var(--yunque-text)" }}>
      <div className="flex items-center justify-between">
        <h1 className="page-title flex items-center gap-2"><ShieldCheck size={20} /> {"信任管理"}</h1>
        <Tooltip delay={0}>
          <Button variant="ghost" size="sm" onPress={refresh}><RefreshCw size={14} /></Button>
          <Tooltip.Content>{"刷新"}</Tooltip.Content>
        </Tooltip>
      </div>

      <Card className="section-card overflow-hidden p-0">
        <div className="grid gap-0 lg:grid-cols-[minmax(0,1fr)_320px]">
          <div className="p-5">
            <div className="flex flex-wrap items-center gap-2">
              <Chip size="sm" style={{ background: "rgba(34,197,94,0.10)", color: "var(--yunque-success)" }}>Control Plane</Chip>
              <Chip size="sm" variant="soft">默认启用</Chip>
              <Chip size="sm" variant="soft">可回滚</Chip>
            </div>
            <div className="mt-3 text-base font-semibold" style={{ color: "var(--yunque-text)" }}>
              这个能力包用来管住高权限能力
            </div>
            <div className="mt-2 max-w-3xl text-sm leading-6" style={{ color: "var(--yunque-text-secondary)" }}>
              当云雀要调用工具、访问网络、写入内容、启用插件、调整模型或处理远程能力包时，Control Plane 提供信任分数、审批、审计、指标和运行状态入口。普通对话不需要天天来这里；当你想知道“它为什么被拦住、谁批准了、出了问题去哪查”时，从这里开始。
            </div>
          </div>
          <div className="p-5" style={{ background: "rgba(59,130,246,0.06)", borderLeft: "1px solid var(--yunque-border)" }}>
            <div className="mb-3 text-sm font-semibold" style={{ color: "var(--yunque-text)" }}>它不会替你自动放权</div>
            <div className="space-y-2 text-xs leading-5" style={{ color: "var(--yunque-text-secondary)" }}>
              <div>不会绕过审批直接允许高风险动作。</div>
              <div>不会把实验能力默认变成生产级权限。</div>
              <div>不会替代 /setup 的首次模型配置主路径。</div>
            </div>
          </div>
        </div>
      </Card>

      <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-5">
        {governanceLinks.map((item) => (
          <Link key={item.href} href={item.href} className="block">
            <Card className="section-card h-full p-4 hover-lift transition-all duration-200">
              <div className="flex items-center gap-2 text-sm font-semibold" style={{ color: "var(--yunque-text)" }}>
                {item.icon}
                {item.label}
              </div>
              <div className="mt-2 text-xs leading-5" style={{ color: "var(--yunque-text-muted)" }}>
                {item.desc}
              </div>
            </Card>
          </Link>
        ))}
      </div>

      {error && (
        <div className="text-xs text-red-400 bg-red-400/10 px-3 py-2.5 rounded-lg animate-fade-in">{error}</div>
      )}

      {/* Level summary */}
      <div className="kpi-grid stagger-children">
        {Object.entries(LEVEL_LABELS).map(([key, label]) => {
          const count = entries.filter(([, e]) => permLevel(e.score) === key).length;
          return (
            <Card key={key} className="section-card hover-lift transition-all duration-200">
              <Card.Content className="flex items-center gap-3 py-3">
                <div className="w-2.5 h-10 rounded-full transition-all duration-500" style={{ background: LEVEL_COLORS[key] }} />
                <div>
                  <div className="kpi-value" style={{ fontSize: "var(--text-xl)" }}>{count}</div>
                  <div className="kpi-sub">{label}</div>
                </div>
              </Card.Content>
            </Card>
          );
        })}
      </div>

      {/* Trust entries - compact table */}
      <Card className="section-card overflow-hidden">
        {entries.length === 0 ? (
          <div className="text-center py-16" style={{ color: "var(--yunque-text-muted)" }}>
            <ShieldCheck size={40} className="mx-auto mb-3 opacity-30" />
            <div>暂无信任记录</div>
          </div>
        ) : (
          <div className="divide-y" style={{ borderColor: "var(--yunque-border)" }}>
            {entries.map(([slug, entry]) => {
              const level = permLevel(entry.score);
              return (
                <div key={slug} className="flex items-center gap-3 px-4 py-2.5 hover:bg-white/[0.02] transition-colors">
                  <div className="w-2 h-2 rounded-full shrink-0" style={{ background: LEVEL_COLORS[level] }} />
                  <span className="text-sm font-medium truncate min-w-[120px]" style={{ color: "var(--yunque-text)" }}>{slug}</span>
                  <div className="flex-1 mx-2">
                    <div className="h-1.5 rounded-full overflow-hidden" style={{ background: "var(--yunque-bg)" }}>
                      <div className="h-full rounded-full transition-all duration-500" style={{ width: `${entry.score}%`, background: LEVEL_COLORS[level] }} />
                    </div>
                  </div>
                  <Chip size="sm" style={{ background: `${LEVEL_COLORS[level]}15`, color: LEVEL_COLORS[level], fontSize: "var(--text-2xs)", flexShrink: 0 }}>
                    {entry.score}
                  </Chip>
                  <div className="flex gap-0.5 shrink-0">
                    <Tooltip delay={0}>
                      <Button size="sm" variant="ghost" isDisabled={acting === slug} onPress={() => handleGrant(slug)}><Zap size={11} /></Button>
                      <Tooltip.Content>授权</Tooltip.Content>
                    </Tooltip>
                    <Tooltip delay={0}>
                      <Button size="sm" variant="ghost" isDisabled={acting === slug} onPress={() => handleReset(slug)}><RotateCcw size={11} /></Button>
                      <Tooltip.Content>重置</Tooltip.Content>
                    </Tooltip>
                  </div>
                </div>
              );
            })}
          </div>
        )}
      </Card>
    </div>
  );
}
