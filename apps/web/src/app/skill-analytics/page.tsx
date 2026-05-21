"use client";

import { useEffect, useState } from "react";
import { api, type MarketAnalytics } from "@/lib/api";
import { Card, Chip } from "@heroui/react";
import { BarChart2, Package, Download, Star, Shield, ShieldCheck, ShieldAlert, TrendingUp, Layers } from "lucide-react";
import Link from "next/link";
import StatCard from "@/components/stat-card";

export default function SkillAnalyticsPage() {
  const [data, setData] = useState<MarketAnalytics | null>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    api.skillHubAnalytics().then(setData).catch(() => {}).finally(() => setLoading(false));
  }, []);

  if (loading) return (
    <div className="flex items-center justify-center h-[60vh]">
      <div className="w-5 h-5 border-2 border-t-transparent rounded-full animate-spin" style={{ borderColor: "var(--yunque-text-muted)", borderTopColor: "transparent" }} />
    </div>
  );

  if (!data) return (
    <div className="text-center py-20" style={{ color: "var(--yunque-text-muted)" }}>
      <BarChart2 size={48} className="mx-auto mb-3 opacity-30" /><p>暂无分析数据</p>
    </div>
  );

  const catEntries = Object.entries(data.categories || {}).sort((a, b) => b[1] - a[1]);
  const maxCat = catEntries.length > 0 ? catEntries[0][1] : 1;

  return (
    <div className="page-root space-y-5 animate-fade-in-up">
      <div className="flex items-center gap-3 mb-2">
        <Link href="/skills" className="text-xs" style={{ color: "var(--yunque-accent)" }}>← 返回技能市场</Link>
      </div>
      <div className="page-header">
        <h1 className="page-title" style={{ display: "flex", alignItems: "center", gap: 8 }}>
          <span style={{ color: "var(--yunque-accent)", display: "flex" }}><BarChart2 size={20} /></span>
          市场分析
        </h1>
      </div>

      {/* Summary Cards */}
      <div className="kpi-grid">
        <StatCard icon={<Package size={16} />} label="技能总数" value={data.total_skills} />
        <StatCard icon={<Download size={16} />} label="已安装" value={data.installed_count} />
        <StatCard icon={<TrendingUp size={16} />} label="总安装量" value={data.total_installs} />
        <StatCard icon={<Shield size={16} />} label="平均安全分" value={data.avg_score > 0 ? data.avg_score.toFixed(1) : "—"} />
      </div>

      {/* Security + Category side by side */}
      <div className="grid grid-cols-1 lg:grid-cols-2 gap-4">
        {/* Security Distribution */}
        <Card className="section-card p-5">
          <div className="flex items-center gap-2 mb-4">
            <Shield size={16} style={{ color: "var(--yunque-accent)" }} />
            <span className="text-sm font-medium" style={{ color: "var(--yunque-text)" }}>安全分数分布</span>
          </div>
          <div className="grid grid-cols-3 gap-3">
            <div className="rounded-lg p-3 text-center" style={{ background: "rgba(23,201,100,0.06)" }}>
              <ShieldCheck size={20} className="mx-auto mb-1" style={{ color: "#17c964" }} />
              <div className="text-lg font-bold" style={{ color: "#17c964" }}>{data.security_stats.high || 0}</div>
              <div className="text-[10px]" style={{ color: "var(--yunque-text-muted)" }}>高 (≥80)</div>
            </div>
            <div className="rounded-lg p-3 text-center" style={{ background: "rgba(245,165,36,0.06)" }}>
              <Shield size={20} className="mx-auto mb-1" style={{ color: "#f5a524" }} />
              <div className="text-lg font-bold" style={{ color: "#f5a524" }}>{data.security_stats.medium || 0}</div>
              <div className="text-[10px]" style={{ color: "var(--yunque-text-muted)" }}>中 (60-79)</div>
            </div>
            <div className="rounded-lg p-3 text-center" style={{ background: "rgba(243,18,96,0.06)" }}>
              <ShieldAlert size={20} className="mx-auto mb-1" style={{ color: "#f31260" }} />
              <div className="text-lg font-bold" style={{ color: "#f31260" }}>{data.security_stats.low || 0}</div>
              <div className="text-[10px]" style={{ color: "var(--yunque-text-muted)" }}>低 (&lt;60)</div>
            </div>
          </div>
        </Card>

        {/* Category Breakdown */}
        {catEntries.length > 0 && (
          <Card className="section-card p-5">
            <div className="flex items-center gap-2 mb-4">
              <Layers size={16} style={{ color: "var(--yunque-accent)" }} />
              <span className="text-sm font-medium" style={{ color: "var(--yunque-text)" }}>分类统计</span>
            </div>
            <div className="space-y-2">
              {catEntries.map(([cat, count]) => (
                <div key={cat} className="flex items-center gap-3">
                  <span className="text-xs w-20 truncate" style={{ color: "var(--yunque-text-muted)" }}>{cat || "未分类"}</span>
                  <div className="flex-1 h-5 rounded-full overflow-hidden" style={{ background: "rgba(255,255,255,0.04)" }}>
                    <div className="h-full rounded-full transition-all" style={{ width: `${(count / maxCat) * 100}%`, background: "var(--yunque-accent)", opacity: 0.7 }} />
                  </div>
                  <span className="text-xs font-medium w-8 text-right" style={{ color: "var(--yunque-text)" }}>{count}</span>
                </div>
              ))}
            </div>
          </Card>
        )}
      </div>

      <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
        {/* Top Installed */}
        {data.top_installed?.length > 0 && (
          <Card className="section-card p-5">
            <div className="flex items-center gap-2 mb-4">
              <Download size={16} style={{ color: "var(--yunque-accent)" }} />
              <span className="text-sm font-medium" style={{ color: "var(--yunque-text)" }}>热门安装</span>
            </div>
            <div className="space-y-2">
              {data.top_installed.slice(0, 8).map((s, i) => (
                <div key={s.slug} className="flex items-center gap-2 text-xs">
                  <span className="w-5 text-right font-bold" style={{ color: "var(--yunque-text-muted)" }}>{i + 1}</span>
                  <Link href={`/skill-detail?slug=${encodeURIComponent(s.slug)}`} className="flex-1 truncate hover:underline" style={{ color: "var(--yunque-accent)" }}>{s.name}</Link>
                  <span className="flex items-center gap-0.5" style={{ color: "var(--yunque-text-muted)" }}><Download size={10} /> {s.installs}</span>
                </div>
              ))}
            </div>
          </Card>
        )}
        {/* Top Rated */}
        {data.top_rated?.length > 0 && (
          <Card className="section-card p-5">
            <div className="flex items-center gap-2 mb-4">
              <Star size={16} style={{ color: "#f5a524" }} />
              <span className="text-sm font-medium" style={{ color: "var(--yunque-text)" }}>最高评分</span>
            </div>
            <div className="space-y-2">
              {data.top_rated.slice(0, 8).map((s, i) => (
                <div key={s.slug} className="flex items-center gap-2 text-xs">
                  <span className="w-5 text-right font-bold" style={{ color: "var(--yunque-text-muted)" }}>{i + 1}</span>
                  <Link href={`/skill-detail?slug=${encodeURIComponent(s.slug)}`} className="flex-1 truncate hover:underline" style={{ color: "var(--yunque-accent)" }}>{s.name}</Link>
                  <span className="flex items-center gap-0.5" style={{ color: "#f5a524" }}><Star size={10} fill="currentColor" /> {s.rating.toFixed(1)}</span>
                </div>
              ))}
            </div>
          </Card>
        )}
      </div>
    </div>
  );
}
