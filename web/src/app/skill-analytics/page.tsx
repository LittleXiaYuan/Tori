"use client";

import { useEffect, useState } from "react";
import { api, type MarketAnalytics } from "@/lib/api";
import { BlurFade } from "@/components/ui/blur-fade";
import {
  BarChart2, Package, Download, Star, Shield, ShieldCheck, ShieldAlert,
  ArrowLeft, TrendingUp, Layers,
} from "lucide-react";
import Link from "next/link";

export default function SkillAnalyticsPage() {
  const [data, setData] = useState<MarketAnalytics | null>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    api.skillHubAnalytics().then(setData).catch(() => {}).finally(() => setLoading(false));
  }, []);

  if (loading) {
    return (
      <div className="flex items-center justify-center h-[60vh]">
        <div className="w-5 h-5 border-2 border-t-transparent rounded-full animate-spin"
          style={{ borderColor: "var(--text-muted)", borderTopColor: "transparent" }} />
      </div>
    );
  }

  if (!data) {
    return (
      <div className="text-center py-20" style={{ color: "var(--text-muted)" }}>
        <BarChart2 size={48} className="mx-auto mb-3 opacity-30" />
        <p>暂无分析数据</p>
      </div>
    );
  }

  const catEntries = Object.entries(data.categories || {}).sort((a, b) => b[1] - a[1]);
  const maxCat = catEntries.length > 0 ? catEntries[0][1] : 1;

  return (
    <div className="max-w-4xl">
      <BlurFade delay={0}>
        <Link href="/skills" className="inline-flex items-center gap-1 text-xs mb-6" style={{ color: "var(--accent)" }}>
          <ArrowLeft size={14} /> 返回技能市场
        </Link>
      </BlurFade>

      <BlurFade delay={0.05}>
        <div className="flex items-center gap-3 mb-6">
          <BarChart2 size={20} />
          <h1 className="text-xl font-semibold tracking-tight">市场分析</h1>
        </div>
      </BlurFade>

      {/* Summary cards */}
      <BlurFade delay={0.1}>
        <div className="grid grid-cols-2 md:grid-cols-4 gap-3 mb-6">
          <SummaryCard icon={<Package size={16} />} label="技能总数" value={data.total_skills} />
          <SummaryCard icon={<Download size={16} />} label="已安装" value={data.installed_count} />
          <SummaryCard icon={<TrendingUp size={16} />} label="总安装量" value={data.total_installs} />
          <SummaryCard icon={<Shield size={16} />} label="平均安全分"
            value={data.avg_score > 0 ? data.avg_score.toFixed(1) : "—"} />
        </div>
      </BlurFade>

      {/* Security distribution */}
      <BlurFade delay={0.15}>
        <div className="rounded-xl border p-5 mb-4" style={{ background: "var(--bg-card)", borderColor: "var(--border)" }}>
          <div className="flex items-center gap-2 mb-4">
            <Shield size={16} style={{ color: "var(--accent)" }} />
            <span className="text-sm font-medium">安全分数分布</span>
          </div>
          <div className="grid grid-cols-3 gap-3">
            <div className="rounded-lg p-3 text-center" style={{ background: "rgba(34,197,94,0.06)" }}>
              <ShieldCheck size={20} className="mx-auto text-green-400 mb-1" />
              <div className="text-lg font-bold text-green-400">{data.security_stats.high || 0}</div>
              <div className="text-[10px]" style={{ color: "var(--text-muted)" }}>高 (≥80)</div>
            </div>
            <div className="rounded-lg p-3 text-center" style={{ background: "rgba(245,158,11,0.06)" }}>
              <Shield size={20} className="mx-auto text-amber-400 mb-1" />
              <div className="text-lg font-bold text-amber-400">{data.security_stats.medium || 0}</div>
              <div className="text-[10px]" style={{ color: "var(--text-muted)" }}>中 (60-79)</div>
            </div>
            <div className="rounded-lg p-3 text-center" style={{ background: "rgba(239,68,68,0.06)" }}>
              <ShieldAlert size={20} className="mx-auto text-red-400 mb-1" />
              <div className="text-lg font-bold text-red-400">{data.security_stats.low || 0}</div>
              <div className="text-[10px]" style={{ color: "var(--text-muted)" }}>低 (&lt;60)</div>
            </div>
          </div>
        </div>
      </BlurFade>

      {/* Category breakdown */}
      {catEntries.length > 0 && (
        <BlurFade delay={0.2}>
          <div className="rounded-xl border p-5 mb-4" style={{ background: "var(--bg-card)", borderColor: "var(--border)" }}>
            <div className="flex items-center gap-2 mb-4">
              <Layers size={16} style={{ color: "var(--accent)" }} />
              <span className="text-sm font-medium">分类统计</span>
            </div>
            <div className="space-y-2">
              {catEntries.map(([cat, count]) => (
                <div key={cat} className="flex items-center gap-3">
                  <span className="text-xs w-20 truncate" style={{ color: "var(--text-muted)" }}>{cat || "未分类"}</span>
                  <div className="flex-1 h-5 rounded-full overflow-hidden" style={{ background: "var(--bg-hover)" }}>
                    <div className="h-full rounded-full transition-all"
                      style={{ width: `${(count / maxCat) * 100}%`, background: "var(--accent)", opacity: 0.7 }} />
                  </div>
                  <span className="text-xs font-medium w-8 text-right">{count}</span>
                </div>
              ))}
            </div>
          </div>
        </BlurFade>
      )}

      <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
        {/* Top Installed */}
        {data.top_installed?.length > 0 && (
          <BlurFade delay={0.25}>
            <div className="rounded-xl border p-5" style={{ background: "var(--bg-card)", borderColor: "var(--border)" }}>
              <div className="flex items-center gap-2 mb-4">
                <Download size={16} style={{ color: "var(--accent)" }} />
                <span className="text-sm font-medium">热门安装</span>
              </div>
              <div className="space-y-2">
                {data.top_installed.slice(0, 8).map((s, i) => (
                  <div key={s.slug} className="flex items-center gap-2 text-xs">
                    <span className="w-5 text-right font-bold" style={{ color: "var(--text-muted)" }}>{i + 1}</span>
                    <Link href={`/skill-detail?slug=${encodeURIComponent(s.slug)}`}
                      className="flex-1 truncate hover:underline" style={{ color: "var(--accent)" }}>
                      {s.name}
                    </Link>
                    <span className="flex items-center gap-0.5" style={{ color: "var(--text-muted)" }}>
                      <Download size={10} /> {s.installs}
                    </span>
                  </div>
                ))}
              </div>
            </div>
          </BlurFade>
        )}

        {/* Top Rated */}
        {data.top_rated?.length > 0 && (
          <BlurFade delay={0.3}>
            <div className="rounded-xl border p-5" style={{ background: "var(--bg-card)", borderColor: "var(--border)" }}>
              <div className="flex items-center gap-2 mb-4">
                <Star size={16} style={{ color: "var(--accent)" }} />
                <span className="text-sm font-medium">最高评分</span>
              </div>
              <div className="space-y-2">
                {data.top_rated.slice(0, 8).map((s, i) => (
                  <div key={s.slug} className="flex items-center gap-2 text-xs">
                    <span className="w-5 text-right font-bold" style={{ color: "var(--text-muted)" }}>{i + 1}</span>
                    <Link href={`/skill-detail?slug=${encodeURIComponent(s.slug)}`}
                      className="flex-1 truncate hover:underline" style={{ color: "var(--accent)" }}>
                      {s.name}
                    </Link>
                    <span className="flex items-center gap-0.5 text-amber-400">
                      <Star size={10} fill="currentColor" /> {s.rating.toFixed(1)}
                    </span>
                  </div>
                ))}
              </div>
            </div>
          </BlurFade>
        )}
      </div>
    </div>
  );
}

function SummaryCard({ icon, label, value }: { icon: React.ReactNode; label: string; value: string | number }) {
  return (
    <div className="rounded-xl border p-4" style={{ background: "var(--bg-card)", borderColor: "var(--border)" }}>
      <div className="flex items-center gap-1.5 text-xs mb-1" style={{ color: "var(--text-muted)" }}>
        {icon} {label}
      </div>
      <div className="text-xl font-bold">{value}</div>
    </div>
  );
}
