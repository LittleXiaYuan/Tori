"use client";

import { useEffect, useState, useCallback } from "react";
import { motion, AnimatePresence } from "motion/react";
import {
  api,
  type ReverieThought,
  type ReverieStats,
  type ReverieConfig,
  type ActionRecord,
} from "@/lib/api";
import { BlurFade } from "@/components/ui/blur-fade";
import { AnimatedList } from "@/components/ui/animated-list";
import { NumberTicker } from "@/components/ui/number-ticker";
import { useI18n } from "@/lib/i18n";
import {
  BrainCircuit,
  Play,
  Circle,
  Trash2,
  Filter,
  Send,
  ChevronDown,
  RefreshCw,
  Zap,
  CheckCircle2,
  XCircle,
} from "lucide-react";

const categoryEmojis: Record<string, string> = {
  reflection: "🪞",
  insight: "💡",
  question: "❓",
  creative: "🎨",
  concern: "⚠️",
  memory: "📝",
  observation: "👁️",
};

export default function ReveriePage() {
  const { t } = useI18n();
  const [thoughts, setThoughts] = useState<ReverieThought[]>([]);
  const [total, setTotal] = useState(0);
  const [stats, setStats] = useState<ReverieStats | null>(null);
  const [config, setConfig] = useState<ReverieConfig | null>(null);
  const [actionLog, setActionLog] = useState<ActionRecord[]>([]);
  const [running, setRunning] = useState(false);
  const [loading, setLoading] = useState(true);
  const [thinking, setThinking] = useState(false);
  const [configOpen, setConfigOpen] = useState(false);
  const [actionsOpen, setActionsOpen] = useState(false);

  // Filters
  const [filterCategory, setFilterCategory] = useState<string>("");
  const [filterMinSig, setFilterMinSig] = useState<number>(0);

  const load = useCallback(async () => {
    try {
      const [journal, st, cfg, acts] = await Promise.all([
        api.getReverieJournal({
          category: filterCategory || undefined,
          min_significance: filterMinSig || undefined,
          limit: 50,
        }),
        api.getReverieStats(),
        api.getReverieConfig(),
        api.getReverieActions(),
      ]);
      setThoughts(journal.thoughts || []);
      setTotal(journal.total);
      setStats(st);
      setConfig(cfg.config);
      setRunning(cfg.running);
      setActionLog(acts.actions || []);
    } catch {
      /* offline */
    } finally {
      setLoading(false);
    }
  }, [filterCategory, filterMinSig]);

  useEffect(() => {
    load();
  }, [load]);

  const triggerThink = async () => {
    setThinking(true);
    try {
      await api.triggerReverieThink();
      await load();
    } finally {
      setThinking(false);
    }
  };

  const deleteThought = async (id: string) => {
    try {
      await api.deleteReverieThought(id);
      setThoughts((prev) => prev.filter((t) => t.id !== id));
      setTotal((prev) => prev - 1);
    } catch {
      /* */
    }
  };

  const toggleEnabled = async () => {
    if (!config) return;
    try {
      const res = await api.updateReverieConfig({ enabled: !config.enabled });
      setConfig(res.config);
      setRunning(res.running);
    } catch {
      /* */
    }
  };

  const updateInterval = async (minutes: number) => {
    try {
      const res = await api.updateReverieConfig({ interval_minutes: minutes });
      setConfig(res.config);
    } catch {
      /* */
    }
  };

  const updateMinSignificance = async (val: number) => {
    try {
      const res = await api.updateReverieConfig({ min_significance: val });
      setConfig(res.config);
    } catch {
      /* */
    }
  };

  const sigStars = (sig: number) => {
    const count = Math.round(sig * 5);
    return "★".repeat(count) + "☆".repeat(5 - count);
  };

  if (loading) {
    return (
      <div className="flex items-center justify-center h-[60vh]">
        <div
          className="w-5 h-5 border-2 border-t-transparent rounded-full animate-spin"
          style={{
            borderColor: "var(--text-muted)",
            borderTopColor: "transparent",
          }}
        />
      </div>
    );
  }

  return (
    <div className="max-w-4xl">
      {/* Header */}
      <BlurFade delay={0}>
        <div className="flex items-center justify-between mb-8">
          <div className="flex items-center gap-3">
            <BrainCircuit size={20} />
            <h1 className="text-xl font-semibold tracking-tight">
              {t("reverie.title")}
            </h1>
          </div>
          <div className="flex items-center gap-2">
            <button
              onClick={() => load()}
              className="flex items-center gap-2 px-3 py-2 rounded-lg text-xs font-medium transition-colors cursor-pointer border"
              style={{
                borderColor: "var(--border)",
                color: "var(--text-muted)",
              }}
            >
              <RefreshCw size={12} />
            </button>
            <button
              onClick={triggerThink}
              disabled={thinking}
              className="flex items-center gap-2 px-4 py-2 rounded-lg text-xs font-medium transition-colors cursor-pointer"
              style={{ background: "var(--text)", color: "var(--bg)" }}
            >
              <Play size={12} />
              {thinking ? t("reverie.thinking") : t("reverie.triggerThink")}
            </button>
          </div>
        </div>
      </BlurFade>

      {/* Status pill — collapsible config */}
      <BlurFade delay={0.05}>
        <div
          className="rounded-xl border mb-6 overflow-hidden transition-shadow"
          style={{
            background: "var(--bg-card)",
            borderColor: "var(--border)",
          }}
        >
          {/* Status row */}
          <button
            onClick={() => setConfigOpen(!configOpen)}
            className="w-full flex items-center justify-between p-5 cursor-pointer"
          >
            <div className="flex items-center gap-3">
              <div className="relative">
                <Circle
                  size={10}
                  fill={running ? "var(--text)" : "var(--text-muted)"}
                  style={{
                    color: running ? "var(--text)" : "var(--text-muted)",
                  }}
                />
                {running && (
                  <div
                    className="absolute inset-0 rounded-full animate-ping"
                    style={{ background: "var(--text)", opacity: 0.3 }}
                  />
                )}
              </div>
              <span className="text-sm font-medium">
                {running ? t("reverie.running") : t("reverie.stopped")}
              </span>
            </div>
            <div className="flex items-center gap-2">
              <button
                onClick={(e) => {
                  e.stopPropagation();
                  toggleEnabled();
                }}
                className="px-4 py-1.5 rounded-lg text-xs font-medium transition-colors cursor-pointer border"
                style={{
                  borderColor: "var(--border)",
                  color: "var(--text-muted)",
                }}
              >
                {running ? t("reverie.stop") : t("reverie.start")}
              </button>
              <motion.div
                animate={{ rotate: configOpen ? 180 : 0 }}
                transition={{ duration: 0.3, ease: [0.16, 1, 0.3, 1] }}
              >
                <ChevronDown
                  size={16}
                  style={{ color: "var(--text-muted)" }}
                />
              </motion.div>
            </div>
          </button>

          {/* Collapsible config panel */}
          <AnimatePresence initial={false}>
            {configOpen && config && (
              <motion.div
                initial={{ height: 0, opacity: 0 }}
                animate={{ height: "auto", opacity: 1 }}
                exit={{ height: 0, opacity: 0 }}
                transition={{
                  height: { duration: 0.35, ease: [0.16, 1, 0.3, 1] },
                  opacity: { duration: 0.25, delay: 0.05 },
                }}
                style={{ overflow: "hidden" }}
              >
                <div
                  className="px-5 pb-5 pt-0 space-y-4 border-t"
                  style={{ borderColor: "var(--border)" }}
                >
                  <div
                    className="text-xs font-medium uppercase tracking-wider pt-4"
                    style={{ color: "var(--text-muted)" }}
                  >
                    {t("reverie.config")}
                  </div>
                  <div className="flex items-center justify-between">
                    <span className="text-sm">{t("reverie.interval")}</span>
                    <select
                      value={config.interval_minutes}
                      onChange={(e) => updateInterval(Number(e.target.value))}
                      className="text-sm px-3 py-1.5 rounded-lg border bg-transparent"
                      style={{ borderColor: "var(--border)" }}
                    >
                      {[5, 10, 15, 30, 60, 120].map((v) => (
                        <option key={v} value={v}>
                          {t("reverie.minutes").replace("{n}", String(v))}
                        </option>
                      ))}
                    </select>
                  </div>
                  <div className="flex items-center justify-between">
                    <span className="text-sm">
                      {t("reverie.minSignificance")}
                    </span>
                    <div className="flex items-center gap-2">
                      <input
                        type="range"
                        min={0}
                        max={1}
                        step={0.1}
                        value={config.min_significance}
                        onChange={(e) =>
                          updateMinSignificance(Number(e.target.value))
                        }
                        className="w-32"
                      />
                      <span
                        className="text-xs w-8 text-right tabular-nums"
                        style={{ color: "var(--text-muted)" }}
                      >
                        {config.min_significance}
                      </span>
                    </div>
                  </div>
                  <div className="flex items-center justify-between">
                    <span className="text-sm">{t("reverie.quietHours")}</span>
                    <span
                      className="text-sm tabular-nums"
                      style={{ color: "var(--text-muted)" }}
                    >
                      {config.quiet_start}:00 – {config.quiet_end}:00
                    </span>
                  </div>
                </div>
              </motion.div>
            )}
          </AnimatePresence>
        </div>
      </BlurFade>

      {/* Stats summary with NumberTicker */}
      {stats && (
        <BlurFade delay={0.1}>
          <div className="grid grid-cols-5 gap-4 mb-6">
            {[
              {
                label: t("reverie.totalThoughts"),
                value: stats.total_thoughts,
              },
              { label: t("reverie.delivered"), value: stats.delivered },
              {
                label: t("reverie.avgSignificance"),
                value: stats.avg_significance,
                format: (n: number) => n.toFixed(2),
              },
              {
                label: t("reverie.categories"),
                value: Object.keys(stats.categories || {}).length,
              },
              {
                label: t("reverie.totalActions"),
                value: actionLog.length,
              },
            ].map((s) => (
              <motion.div
                key={s.label}
                className="rounded-xl border p-4 text-center"
                style={{
                  background: "var(--bg-card)",
                  borderColor: "var(--border)",
                }}
                whileHover={{ scale: 1.03, y: -2 }}
                transition={{
                  type: "spring",
                  stiffness: 400,
                  damping: 25,
                }}
              >
                <div className="text-2xl font-semibold">
                  {"format" in s && s.format ? (
                    <NumberTicker
                      value={s.value}
                      format={s.format}
                      duration={1000}
                    />
                  ) : (
                    <NumberTicker value={s.value} duration={1000} />
                  )}
                </div>
                <div
                  className="text-xs mt-1"
                  style={{ color: "var(--text-muted)" }}
                >
                  {s.label}
                </div>
              </motion.div>
            ))}
          </div>
        </BlurFade>
      )}

      {/* Filters */}
      <BlurFade delay={0.15}>
        <div className="flex items-center gap-3 mb-4">
          <Filter size={14} style={{ color: "var(--text-muted)" }} />
          <select
            value={filterCategory}
            onChange={(e) => setFilterCategory(e.target.value)}
            className="text-xs px-3 py-1.5 rounded-lg border bg-transparent"
            style={{ borderColor: "var(--border)" }}
          >
            <option value="">{t("reverie.allCategories")}</option>
            {Object.keys(stats?.categories || {}).map((c) => (
              <option key={c} value={c}>
                {categoryEmojis[c] || "📌"} {c}
              </option>
            ))}
          </select>
          <select
            value={filterMinSig}
            onChange={(e) => setFilterMinSig(Number(e.target.value))}
            className="text-xs px-3 py-1.5 rounded-lg border bg-transparent"
            style={{ borderColor: "var(--border)" }}
          >
            <option value={0}>{t("reverie.anySignificance")}</option>
            <option value={0.3}>≥ 0.3</option>
            <option value={0.5}>≥ 0.5</option>
            <option value={0.7}>≥ 0.7</option>
            <option value={0.9}>≥ 0.9</option>
          </select>
          <span className="text-xs" style={{ color: "var(--text-muted)" }}>
            {t("reverie.total").replace("{n}", String(total))}
          </span>
        </div>
      </BlurFade>

      {/* Action Log — collapsible */}
      <BlurFade delay={0.17}>
        <div
          className="rounded-xl border mb-6 overflow-hidden"
          style={{
            background: "var(--bg-card)",
            borderColor: "var(--border)",
          }}
        >
          <button
            onClick={() => setActionsOpen(!actionsOpen)}
            className="w-full flex items-center justify-between p-5 cursor-pointer"
          >
            <div className="flex items-center gap-2">
              <Zap size={14} />
              <span className="text-xs font-medium uppercase tracking-wider">
                {t("reverie.actionLog")}
              </span>
              <span
                className="text-xs px-2 py-0.5 rounded-full"
                style={{
                  background: "var(--bg)",
                  color: "var(--text-muted)",
                }}
              >
                {actionLog.length}
              </span>
            </div>
            <motion.div
              animate={{ rotate: actionsOpen ? 180 : 0 }}
              transition={{ duration: 0.3, ease: [0.16, 1, 0.3, 1] }}
            >
              <ChevronDown size={16} style={{ color: "var(--text-muted)" }} />
            </motion.div>
          </button>

          <AnimatePresence initial={false}>
            {actionsOpen && (
              <motion.div
                initial={{ height: 0, opacity: 0 }}
                animate={{ height: "auto", opacity: 1 }}
                exit={{ height: 0, opacity: 0 }}
                transition={{
                  height: { duration: 0.35, ease: [0.16, 1, 0.3, 1] },
                  opacity: { duration: 0.25, delay: 0.05 },
                }}
                style={{ overflow: "hidden" }}
              >
                <div
                  className="px-5 pb-5 border-t"
                  style={{ borderColor: "var(--border)" }}
                >
                  {actionLog.length === 0 ? (
                    <div
                      className="text-sm text-center py-8"
                      style={{ color: "var(--text-muted)" }}
                    >
                      {t("reverie.noActions")}
                    </div>
                  ) : (
                    <div className="space-y-2 pt-3">
                      {actionLog
                        .slice()
                        .reverse()
                        .slice(0, 50)
                        .map((rec, i) => (
                          <div
                            key={`${rec.thought_id}-${i}`}
                            className="flex items-start gap-3 p-3 rounded-lg"
                            style={{ background: "var(--bg-hover)" }}
                          >
                            <div className="mt-0.5 shrink-0">
                              {rec.success ? (
                                <CheckCircle2
                                  size={14}
                                  style={{ color: "var(--text)" }}
                                />
                              ) : (
                                <XCircle
                                  size={14}
                                  style={{ color: "var(--text-muted)" }}
                                />
                              )}
                            </div>
                            <div className="min-w-0 flex-1">
                              <div className="flex items-center gap-2 mb-1 flex-wrap">
                                <span
                                  className="text-[10px] px-1.5 py-0.5 rounded-full font-medium"
                                  style={{
                                    background: "var(--bg)",
                                    color: "var(--text-muted)",
                                  }}
                                >
                                  {t(
                                    `reverie.actionType.${rec.action.type}` as "reverie.actionType.write_memory"
                                  ) || rec.action.type}
                                </span>
                                <span
                                  className="text-[10px]"
                                  style={{
                                    color: rec.success
                                      ? "var(--text)"
                                      : "var(--text-muted)",
                                  }}
                                >
                                  {rec.success
                                    ? t("reverie.actionSuccess")
                                    : t("reverie.actionFailed")}
                                </span>
                              </div>
                              <div className="text-sm">
                                <span className="font-medium">
                                  {rec.action.key}
                                </span>
                                {rec.action.value && (
                                  <span
                                    className="ml-2"
                                    style={{ color: "var(--text-muted)" }}
                                  >
                                    → {rec.action.value}
                                  </span>
                                )}
                              </div>
                              {rec.error && (
                                <div
                                  className="text-xs mt-1"
                                  style={{ color: "var(--text-muted)" }}
                                >
                                  {rec.error}
                                </div>
                              )}
                              <div
                                className="text-xs mt-1"
                                style={{ color: "var(--text-muted)" }}
                              >
                                {new Date(rec.at).toLocaleString()}
                              </div>
                            </div>
                          </div>
                        ))}
                    </div>
                  )}
                </div>
              </motion.div>
            )}
          </AnimatePresence>
        </div>
      </BlurFade>

      {/* Thought journal */}
      <BlurFade delay={0.2}>
        <div
          className="rounded-xl border p-5"
          style={{
            background: "var(--bg-card)",
            borderColor: "var(--border)",
          }}
        >
          <div
            className="text-xs font-medium uppercase tracking-wider mb-4"
            style={{ color: "var(--text-muted)" }}
          >
            {t("reverie.journal")}
          </div>
          {thoughts.length === 0 ? (
            <div
              className="text-sm text-center py-12"
              style={{ color: "var(--text-muted)" }}
            >
              {t("reverie.empty")}
            </div>
          ) : (
            <AnimatedList>
              {thoughts.map((th) => (
                <motion.div
                  key={th.id}
                  layout
                  className="flex items-start gap-3 p-3 rounded-lg group"
                  style={{ background: "var(--bg-hover)" }}
                  initial={{ opacity: 0, x: -12 }}
                  animate={{ opacity: 1, x: 0 }}
                  exit={{ opacity: 0, x: 12, height: 0, marginBottom: 0 }}
                  transition={{ duration: 0.3, ease: [0.16, 1, 0.3, 1] }}
                >
                  <div className="mt-1 shrink-0 text-base">
                    {categoryEmojis[th.category] || "💭"}
                  </div>
                  <div className="min-w-0 flex-1">
                    <div className="flex items-center gap-2 mb-1 flex-wrap">
                      <span
                        className="text-[10px] px-1.5 py-0.5 rounded-full"
                        style={{
                          background: "var(--bg)",
                          color: "var(--text-muted)",
                        }}
                      >
                        {th.category}
                      </span>
                      <span
                        className="text-[10px]"
                        style={{ color: "var(--text-muted)" }}
                        title={`${t("reverie.minSignificance")}: ${th.significance}`}
                      >
                        {sigStars(th.significance)}
                      </span>
                      {th.delivered && (
                        <span
                          className="inline-flex items-center gap-0.5"
                          title={t("reverie.pushed")}
                        >
                          <Send
                            size={10}
                            style={{ color: "var(--text-muted)" }}
                          />
                        </span>
                      )}
                      {th.trigger && th.trigger !== "timer" && (
                        <span
                          className="text-[10px] px-1.5 py-0.5 rounded-full"
                          style={{
                            background: "var(--bg)",
                            color: "var(--text-muted)",
                          }}
                        >
                          ⚡ {th.trigger}
                        </span>
                      )}
                    </div>
                    <div className="text-sm whitespace-pre-wrap">
                      {th.content}
                    </div>
                    {th.actions && th.actions.length > 0 && (
                      <div className="flex items-center gap-1.5 mt-2 flex-wrap">
                        {th.actions.map((a, i) => (
                          <span
                            key={i}
                            className="inline-flex items-center gap-1 text-[10px] px-1.5 py-0.5 rounded-full"
                            style={{
                              background: "var(--bg)",
                              color: "var(--text-muted)",
                            }}
                          >
                            <Zap size={8} />
                            {t(
                              `reverie.actionType.${a.type}` as "reverie.actionType.write_memory"
                            ) || a.type}
                          </span>
                        ))}
                      </div>
                    )}
                    <div
                      className="text-xs mt-1"
                      style={{ color: "var(--text-muted)" }}
                    >
                      {new Date(th.timestamp).toLocaleString()}
                    </div>
                  </div>
                  <button
                    onClick={() => deleteThought(th.id)}
                    className="opacity-0 group-hover:opacity-100 transition-opacity p-1 rounded cursor-pointer"
                    style={{ color: "var(--text-muted)" }}
                    title={t("reverie.delete")}
                  >
                    <Trash2 size={12} />
                  </button>
                </motion.div>
              ))}
            </AnimatedList>
          )}
        </div>
      </BlurFade>
    </div>
  );
}
