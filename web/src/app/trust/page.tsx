"use client";

import { useEffect, useState, useCallback } from "react";
import { api, type TrustEntry } from "@/lib/api";
import { ShieldCheck, Zap, RotateCcw, ChevronRight } from "lucide-react";
import { useI18n } from "@/lib/i18n";

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

export default function TrustPage() {
  const [scores, setScores] = useState<Record<string, TrustEntry>>({});
  const [loading, setLoading] = useState(true);
  const [acting, setActing] = useState("");
  const [error, setError] = useState("");
  const { t } = useI18n();

  const refresh = useCallback(() => {
    setLoading(true);
    api.trustScores()
      .then((r) => { setScores(r.scores || {}); setError(""); })
      .catch((e) => setError(String(e?.message || "加载信任分数失败")))
      .finally(() => setLoading(false));
  }, []);

  useEffect(() => { refresh(); }, [refresh]);

  const handleGrant = async (slug: string) => {
    setActing(slug);
    try {
      await api.trustGrant(slug);
      refresh();
    } catch (e: unknown) { setError(String((e as Error)?.message || "授权失败")); }
    setActing("");
  };

  const handleGrantAll = async () => {
    if (!confirm("确认给所有技能授予最高信任（Shell 权限）？")) return;
    setActing("*");
    try {
      await api.trustGrant("*");
      refresh();
    } catch (e: unknown) { setError(String((e as Error)?.message || "批量授权失败")); }
    setActing("");
  };

  const handleReset = async (slug: string) => {
    setActing(slug);
    try {
      await api.trustReset(slug);
      refresh();
    } catch (e: unknown) { setError(String((e as Error)?.message || "重置失败")); }
    setActing("");
  };

  const entries = Object.entries(scores).sort((a, b) => b[1].score - a[1].score);
  const total = entries.length;
  const highTrust = entries.filter(([, e]) => e.score >= 80).length;
  const lowTrust = entries.filter(([, e]) => e.score < 30).length;

  return (
    <div className="animate-in">
      {/* Error Banner */}
      {error && (
        <div className="mb-4 px-4 py-2 rounded-lg text-sm flex items-center justify-between" style={{ background: "#ef444420", color: "#ef4444", border: "1px solid #ef444440" }}>
          <span>{error}</span>
          <button onClick={() => setError("")} className="ml-2 opacity-60 hover:opacity-100">&times;</button>
        </div>
      )}
      {/* Header */}
      <div className="flex items-center justify-between mb-8">
        <div className="flex items-center gap-3">
          <div className="w-10 h-10 rounded-xl flex items-center justify-center" style={{ background: "var(--accent-subtle)" }}>
            <ShieldCheck size={20} style={{ color: "var(--accent)" }} />
          </div>
          <div>
            <h1 className="text-xl font-semibold">{t("trust.title") || "信任管理"}</h1>
            <p className="text-sm opacity-60">{t("trust.subtitle") || "渐进式信任评分与一键授权"}</p>
          </div>
        </div>
        <div className="flex gap-2">
          <button onClick={handleGrantAll} disabled={acting === "*"}
            className="px-4 py-2 rounded-lg text-sm font-medium transition-all flex items-center gap-2"
            style={{ background: "#ef4444", color: "#fff", opacity: acting === "*" ? 0.5 : 1 }}>
            <Zap size={14} />
            {acting === "*" ? "..." : "一键全部授权"}
          </button>
          <button onClick={refresh}
            className="px-3 py-2 rounded-lg text-sm border transition-all hover:opacity-80"
            style={{ borderColor: "var(--border)" }}>
            <RotateCcw size={14} />
          </button>
        </div>
      </div>

      {/* Stats Cards */}
      <div className="grid grid-cols-3 gap-4 mb-6">
        <div className="rounded-xl p-4" style={{ background: "var(--card)", border: "1px solid var(--border)" }}>
          <div className="text-sm opacity-60 mb-1">{t("trust.total") || "已追踪技能"}</div>
          <div className="text-2xl font-bold">{total}</div>
        </div>
        <div className="rounded-xl p-4" style={{ background: "var(--card)", border: "1px solid var(--border)" }}>
          <div className="text-sm opacity-60 mb-1">{t("trust.high_trust") || "高信任 (Shell)"}</div>
          <div className="text-2xl font-bold" style={{ color: "#22c55e" }}>{highTrust}</div>
        </div>
        <div className="rounded-xl p-4" style={{ background: "var(--card)", border: "1px solid var(--border)" }}>
          <div className="text-sm opacity-60 mb-1">{t("trust.low_trust") || "低信任 (ReadOnly)"}</div>
          <div className="text-2xl font-bold" style={{ color: "#f59e0b" }}>{lowTrust}</div>
        </div>
      </div>

      {/* Trust Table */}
      {loading ? (
        <div className="text-center py-12 opacity-50">加载中...</div>
      ) : entries.length === 0 ? (
        <div className="text-center py-12 opacity-50">暂无信任追踪数据</div>
      ) : (
        <div className="rounded-xl overflow-hidden" style={{ border: "1px solid var(--border)" }}>
          <table className="w-full text-sm">
            <thead>
              <tr style={{ background: "var(--card)" }}>
                <th className="text-left px-4 py-3 font-medium opacity-60">技能</th>
                <th className="text-center px-4 py-3 font-medium opacity-60">评分</th>
                <th className="text-center px-4 py-3 font-medium opacity-60">权限</th>
                <th className="text-center px-4 py-3 font-medium opacity-60">执行次数</th>
                <th className="text-center px-4 py-3 font-medium opacity-60">失败</th>
                <th className="text-right px-4 py-3 font-medium opacity-60">操作</th>
              </tr>
            </thead>
            <tbody>
              {entries.map(([slug, entry]) => {
                const level = permLevel(entry.score);
                return (
                  <tr key={slug} className="border-t transition-colors hover:opacity-90"
                    style={{ borderColor: "var(--border)" }}>
                    <td className="px-4 py-3 font-mono text-xs flex items-center gap-2">
                      <ChevronRight size={12} className="opacity-40" />
                      {slug}
                    </td>
                    <td className="px-4 py-3 text-center">
                      <div className="flex items-center justify-center gap-2">
                        <div className="w-24 h-2 rounded-full overflow-hidden" style={{ background: "var(--border)" }}>
                          <div className="h-full rounded-full transition-all"
                            style={{ width: `${entry.score}%`, background: LEVEL_COLORS[level] }} />
                        </div>
                        <span className="text-xs font-mono w-8">{entry.score}</span>
                      </div>
                    </td>
                    <td className="px-4 py-3 text-center">
                      <span className="px-2 py-0.5 rounded text-xs font-medium"
                        style={{ background: LEVEL_COLORS[level] + "22", color: LEVEL_COLORS[level] }}>
                        {LEVEL_LABELS[level]}
                      </span>
                    </td>
                    <td className="px-4 py-3 text-center font-mono text-xs">{entry.executions}</td>
                    <td className="px-4 py-3 text-center font-mono text-xs" style={{ color: entry.failures > 0 ? "#ef4444" : undefined }}>
                      {entry.failures}
                    </td>
                    <td className="px-4 py-3 text-right">
                      <div className="flex justify-end gap-1">
                        {entry.score < 100 && (
                          <button onClick={() => handleGrant(slug)} disabled={acting === slug}
                            className="px-2 py-1 rounded text-xs transition-all hover:opacity-80"
                            style={{ background: "var(--accent-subtle)", color: "var(--accent)" }}>
                            {acting === slug ? "..." : "授权"}
                          </button>
                        )}
                        <button onClick={() => handleReset(slug)} disabled={acting === slug}
                          className="px-2 py-1 rounded text-xs transition-all hover:opacity-80"
                          style={{ background: "var(--border)" }}>
                          重置
                        </button>
                      </div>
                    </td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        </div>
      )}

      {/* Legend */}
      <div className="mt-6 flex gap-4 text-xs opacity-50">
        {Object.entries(LEVEL_LABELS).reverse().map(([key, label]) => (
          <div key={key} className="flex items-center gap-1">
            <div className="w-2 h-2 rounded-full" style={{ background: LEVEL_COLORS[key] }} />
            {label}
          </div>
        ))}
      </div>
    </div>
  );
}
