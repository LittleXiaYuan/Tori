"use client";

import { useState } from "react";
import { api, type TrustEntry } from "@/lib/api";
import { Card, Button, Spinner, Tooltip, Chip, ProgressBar } from "@heroui/react";
import { ShieldCheck, Zap, RotateCcw, RefreshCw } from "lucide-react";
import { useApiData } from "@/lib/use-api-data";

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

export default function TrustPage() {
  const { data: scores, loading, refresh } = useApiData(
    async () => { const r = await api.trustScores(); return r.scores || {}; },
    {} as Record<string, TrustEntry>,
  );
  const [acting, setActing] = useState("");
  const [error, setError] = useState("");

  const handleGrant = async (slug: string) => {
    setActing(slug);
    try { await api.trustGrant(slug); refresh(); } catch (e: unknown) { setError(String((e as Error)?.message || "授权失败")); }
    setActing("");
  };

  const handleReset = async (slug: string) => {
    setActing(slug);
    try { await api.trustReset(slug); refresh(); } catch (e: unknown) { setError(String((e as Error)?.message || "重置失败")); }
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
