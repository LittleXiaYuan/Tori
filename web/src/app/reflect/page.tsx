"use client";

import { useEffect, useState, useCallback } from "react";
import { api, type ExperienceItem, type ExperienceStats } from "@/lib/api";
import { BlurFade } from "@/components/ui/blur-fade";
import { NumberTicker } from "@/components/ui/number-ticker";
import { useI18n } from "@/lib/i18n";
import {
  Lightbulb,
  Search,
  CheckCircle2,
  XCircle,
  Tag,
  Clock,
  Sparkles,
  RefreshCw,
} from "lucide-react";

/* ── Helpers ── */

const outcomeColor: Record<string, string> = {
  success: "#22c55e",
  failure: "#ef4444",
  neutral: "#9ca3af",
};

const categoryEmoji: Record<string, string> = {
  task_success: "✅",
  task_failure: "❌",
  gap_resolved: "🔧",
  retry_pattern: "🔄",
  skill_gap: "📦",
  llm_insight: "💡",
};

function relTime(ts?: string): string {
  if (!ts) return "";
  const d = Date.now() - new Date(ts).getTime();
  if (d < 60000) return `${Math.floor(d / 1000)}s ago`;
  if (d < 3600000) return `${Math.floor(d / 60000)}m ago`;
  if (d < 86400000) return `${Math.floor(d / 3600000)}h ago`;
  return `${Math.floor(d / 86400000)}d ago`;
}

/* ── Page ── */

export default function ReflectPage() {
  const { t } = useI18n();
  const [experiences, setExperiences] = useState<ExperienceItem[]>([]);
  const [stats, setStats] = useState<ExperienceStats | null>(null);
  const [strategies, setStrategies] = useState("");
  const [loading, setLoading] = useState(true);

  // Filters
  const [source, setSource] = useState("");
  const [category, setCategory] = useState("");
  const [outcome, setOutcome] = useState("");
  const [search, setSearch] = useState("");

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const [statsRes, listRes, stratRes] = await Promise.all([
        api.getExperiences({ stats: true }),
        api.getExperiences({
          source: source || undefined,
          category: category || undefined,
          outcome: outcome || undefined,
          q: search || undefined,
        }),
        api.getStrategies(),
      ]);
      if ("total" in statsRes && "by_source" in statsRes) {
        setStats(statsRes as ExperienceStats);
      }
      if ("experiences" in listRes) {
        setExperiences(
          (listRes as { experiences: ExperienceItem[] }).experiences || [],
        );
      }
      setStrategies(typeof stratRes === "string" ? stratRes : (stratRes as { strategies: string }).strategies || "");
    } catch {
      /* silent */
    } finally {
      setLoading(false);
    }
  }, [source, category, outcome, search]);

  useEffect(() => {
    load();
  }, [load]);

  /* ── Stats Cards ── */
  const statCards = stats
    ? [
        { label: t("reflect.total"), value: stats.total, color: "var(--accent)" },
        { label: t("reflect.recent"), value: stats.recent_7d, color: "#a78bfa" },
        {
          label: t("reflect.successes"),
          value: stats.by_outcome?.success || 0,
          color: "#22c55e",
        },
        {
          label: t("reflect.failures"),
          value: stats.by_outcome?.failure || 0,
          color: "#ef4444",
        },
      ]
    : [];

  return (
    <div className="space-y-6 max-w-6xl mx-auto">
      {/* Header */}
      <BlurFade delay={0.05}>
        <div className="flex items-center gap-3 mb-2">
          <Lightbulb size={24} style={{ color: "var(--accent)" }} />
          <h1 className="text-2xl font-bold">{t("reflect.title")}</h1>
          <button
            onClick={load}
            className="ml-auto p-2 rounded-lg hover:bg-white/5 transition"
            title="Refresh"
          >
            <RefreshCw size={16} className={loading ? "animate-spin" : ""} style={{ color: "var(--text-muted)" }} />
          </button>
        </div>
      </BlurFade>

      {/* Stats Grid */}
      {stats && (
        <BlurFade delay={0.1}>
          <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
            {statCards.map((s, i) => (
              <div
                key={i}
                className="rounded-xl p-4 border text-center"
                style={{ background: "var(--bg-card)", borderColor: "var(--border)" }}
              >
                <div className="text-xs mb-1" style={{ color: "var(--text-muted)" }}>
                  {s.label}
                </div>
                <div className="text-2xl font-bold" style={{ color: s.color }}>
                  <NumberTicker value={s.value} />
                </div>
              </div>
            ))}
          </div>
        </BlurFade>
      )}

      {/* Filters */}
      <BlurFade delay={0.15}>
        <div
          className="flex flex-wrap gap-3 rounded-xl p-4 border"
          style={{ background: "var(--bg-card)", borderColor: "var(--border)" }}
        >
          <select
            value={source}
            onChange={(e) => setSource(e.target.value)}
            className="rounded-lg px-3 py-1.5 text-sm border"
            style={{
              background: "var(--bg-main)",
              borderColor: "var(--border)",
              color: "var(--text-main)",
            }}
          >
            <option value="">{t("reflect.allSources")}</option>
            {stats &&
              Object.keys(stats.by_source || {}).map((k) => (
                <option key={k} value={k}>
                  {k}
                </option>
              ))}
          </select>
          <select
            value={category}
            onChange={(e) => setCategory(e.target.value)}
            className="rounded-lg px-3 py-1.5 text-sm border"
            style={{
              background: "var(--bg-main)",
              borderColor: "var(--border)",
              color: "var(--text-main)",
            }}
          >
            <option value="">{t("reflect.allCategories")}</option>
            {stats &&
              Object.keys(stats.by_category || {}).map((k) => (
                <option key={k} value={k}>
                  {k}
                </option>
              ))}
          </select>
          <select
            value={outcome}
            onChange={(e) => setOutcome(e.target.value)}
            className="rounded-lg px-3 py-1.5 text-sm border"
            style={{
              background: "var(--bg-main)",
              borderColor: "var(--border)",
              color: "var(--text-main)",
            }}
          >
            <option value="">{t("reflect.allOutcomes")}</option>
            <option value="success">success</option>
            <option value="failure">failure</option>
            <option value="neutral">neutral</option>
          </select>
          <div className="relative flex-1 min-w-[200px]">
            <Search
              size={14}
              className="absolute left-3 top-1/2 -translate-y-1/2"
              style={{ color: "var(--text-muted)" }}
            />
            <input
              type="text"
              value={search}
              onChange={(e) => setSearch(e.target.value)}
              placeholder={t("reflect.search")}
              className="w-full rounded-lg pl-8 pr-3 py-1.5 text-sm border"
              style={{
                background: "var(--bg-main)",
                borderColor: "var(--border)",
                color: "var(--text-main)",
              }}
            />
          </div>
        </div>
      </BlurFade>

      <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
        {/* Experience List */}
        <div className="lg:col-span-2 space-y-3">
          {experiences.length === 0 && !loading && (
            <BlurFade delay={0.2}>
              <div
                className="text-center py-12 rounded-xl border"
                style={{
                  background: "var(--bg-card)",
                  borderColor: "var(--border)",
                  color: "var(--text-muted)",
                }}
              >
                {t("reflect.empty")}
              </div>
            </BlurFade>
          )}
          {experiences.map((exp, i) => (
            <BlurFade key={exp.id} delay={0.2 + i * 0.03}>
              <div
                className="rounded-xl p-4 border"
                style={{ background: "var(--bg-card)", borderColor: "var(--border)" }}
              >
                <div className="flex items-start gap-3">
                  {/* Category emoji */}
                  <span className="text-xl mt-0.5">
                    {categoryEmoji[exp.category] || "📝"}
                  </span>
                  <div className="flex-1 min-w-0">
                    {/* Top row */}
                    <div className="flex items-center gap-2 mb-1 flex-wrap">
                      <span
                        className="inline-flex items-center gap-1 px-2 py-0.5 text-xs rounded-full font-medium"
                        style={{
                          background: `${outcomeColor[exp.outcome] || "#9ca3af"}20`,
                          color: outcomeColor[exp.outcome] || "#9ca3af",
                        }}
                      >
                        {exp.outcome === "success" ? (
                          <CheckCircle2 size={10} />
                        ) : exp.outcome === "failure" ? (
                          <XCircle size={10} />
                        ) : null}
                        {exp.outcome}
                      </span>
                      <span
                        className="text-xs px-2 py-0.5 rounded-full"
                        style={{
                          background: "var(--accent-dim)",
                          color: "var(--accent)",
                        }}
                      >
                        {exp.category}
                      </span>
                      <span className="text-xs" style={{ color: "var(--text-muted)" }}>
                        {exp.source}
                      </span>
                    </div>
                    {/* Lesson */}
                    <p className="text-sm mt-1" style={{ color: "var(--text-main)" }}>
                      {exp.lesson}
                    </p>
                    {/* Context */}
                    {exp.context && (
                      <p className="text-xs mt-1" style={{ color: "var(--text-muted)" }}>
                        {exp.context}
                      </p>
                    )}
                    {/* Bottom row: tags + time */}
                    <div className="flex items-center gap-2 mt-2 flex-wrap">
                      {exp.tags?.map((tag) => (
                        <span
                          key={tag}
                          className="inline-flex items-center gap-1 text-xs px-1.5 py-0.5 rounded"
                          style={{
                            background: "var(--bg-main)",
                            color: "var(--text-muted)",
                          }}
                        >
                          <Tag size={10} />
                          {tag}
                        </span>
                      ))}
                      <span className="text-xs ml-auto flex items-center gap-1" style={{ color: "var(--text-muted)" }}>
                        <Clock size={10} />
                        {relTime(exp.created_at)}
                      </span>
                    </div>
                  </div>
                </div>
              </div>
            </BlurFade>
          ))}
        </div>

        {/* Strategy Panel */}
        <div className="space-y-3">
          <BlurFade delay={0.2}>
            <div
              className="rounded-xl p-5 border"
              style={{ background: "var(--bg-card)", borderColor: "var(--border)" }}
            >
              <div className="flex items-center gap-2 mb-3">
                <Sparkles size={16} style={{ color: "var(--accent)" }} />
                <span className="font-semibold text-sm">{t("reflect.strategies")}</span>
              </div>
              {strategies ? (
                <div
                  className="text-sm whitespace-pre-wrap leading-relaxed"
                  style={{ color: "var(--text-main)" }}
                >
                  {strategies}
                </div>
              ) : (
                <div className="text-sm" style={{ color: "var(--text-muted)" }}>
                  {t("reflect.noStrategies")}
                </div>
              )}
            </div>
          </BlurFade>
        </div>
      </div>
    </div>
  );
}
