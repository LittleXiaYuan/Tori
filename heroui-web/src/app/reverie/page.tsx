"use client";

import { useEffect, useState, useCallback } from "react";
import { api, type ReverieThought, type ReverieStats, type ReverieConfig, type ActionRecord } from "@/lib/api";
import { Card, Button, Switch, Chip, Select, Label, ListBox, Slider } from "@heroui/react";
import EmptyState from "@/components/empty-state";
import {
  BrainCircuit, Play, Circle, Trash2, Filter, Send, ChevronDown,
  RefreshCw, Zap, CheckCircle2, XCircle,
} from "lucide-react";
import { showErrorToast } from "@/components/toast-provider";

const categoryEmojis: Record<string, string> = {
  reflection: "--", insight: "*", question: "?", creative: "~",
  concern: "!", memory: "#", observation: "o",
};

export default function ReveriePage() {
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
  const [filterCategory, setFilterCategory] = useState<string>("");
  const [filterMinSig, setFilterMinSig] = useState<number>(0);

  const load = useCallback(async () => {
    try {
      const [journal, st, cfg, acts] = await Promise.all([
        api.getReverieJournal({ category: filterCategory || undefined, min_significance: filterMinSig || undefined, limit: 50 }),
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
    } catch { /* offline */ }
    finally { setLoading(false); }
  }, [filterCategory, filterMinSig]);

  useEffect(() => { load(); }, [load]);

  const triggerThink = async () => {
    setThinking(true);
    try { await api.triggerReverieThink(); await load(); }
    finally { setThinking(false); }
  };

  const deleteThought = async (id: string) => {
    try { await api.deleteReverieThought(id); setThoughts((prev) => prev.filter((t) => t.id !== id)); setTotal((prev) => prev - 1); }
    catch (e) { showErrorToast(e, "删除失败"); }
  };

  const toggleEnabled = async () => {
    if (!config) return;
    try { const res = await api.updateReverieConfig({ enabled: !config.enabled }); setConfig(res.config); setRunning(res.running); }
    catch (e) { showErrorToast(e, "切换失败"); }
  };

  const updateInterval = async (minutes: number) => {
    try { const res = await api.updateReverieConfig({ interval_minutes: minutes }); setConfig(res.config); }
    catch (e) { showErrorToast(e, "更新失败"); }
  };

  const updateMinSignificance = async (val: number) => {
    try { const res = await api.updateReverieConfig({ min_significance: val }); setConfig(res.config); }
    catch (e) { showErrorToast(e, "更新失败"); }
  };

  const sigStars = (sig: number) => { const count = Math.round(sig * 5); return "*".repeat(count) + "-".repeat(5 - count); };

  return (
    <div className="page-root space-y-5 animate-fade-in-up">
      <div className="page-header">
        <h1 className="page-title" style={{ display: "flex", alignItems: "center", gap: 8 }}>
          <span style={{ color: "var(--yunque-accent)", display: "flex" }}><BrainCircuit size={20} /></span>
          内心独白
        </h1>
        <div className="flex items-center gap-2">
          <Button isIconOnly aria-label="刷新" variant="ghost" size="sm" onPress={() => load()} isPending={loading}>
            <RefreshCw size={14} />
          </Button>
          <Button size="sm" onPress={triggerThink} isPending={thinking} className="btn-accent">
            <Play size={12} /> 触发思考
          </Button>
        </div>
      </div>

      {/* Two-column: left sidebar controls, right journal */}
      <div className="grid grid-cols-1 lg:grid-cols-3 gap-5">

        {/* ── Left sidebar ── */}
        <div className="space-y-4">

          {/* Status + Config */}
          <Card>
            <Button variant="ghost" onPress={() => setConfigOpen(!configOpen)} className="w-full flex items-center justify-between p-5 h-auto">
              <div className="flex items-center gap-3">
                <div className="relative">
                  <Circle size={10} fill={running ? "#17c964" : "var(--yunque-text-muted)"} style={{ color: running ? "#17c964" : "var(--yunque-text-muted)" }} />
                  {running && <div className="absolute inset-0 rounded-full animate-ping" style={{ background: "#17c964", opacity: 0.3 }} />}
                </div>
                <span className="text-sm font-medium" style={{ color: "var(--yunque-text)" }}>{running ? "运行中" : "已停止"}</span>
              </div>
              <div className="flex items-center gap-2">
                <Button size="sm" variant="outline" onPress={(e) => { toggleEnabled(); }}>{running ? "停止" : "启动"}</Button>
                <ChevronDown size={16} style={{ color: "var(--yunque-text-muted)", transform: configOpen ? "rotate(180deg)" : "none", transition: "transform 0.2s" }} />
              </div>
            </Button>
            {configOpen && config && (
              <div className="px-5 pb-5 pt-0 space-y-4 border-t" style={{ borderColor: "var(--yunque-border)" }}>
                <div className="text-xs font-medium uppercase tracking-wider pt-4" style={{ color: "var(--yunque-text-muted)" }}>配置</div>
                <div className="flex items-center justify-between">
                  <span className="text-sm" style={{ color: "var(--yunque-text)" }}>间隔</span>
                  <Select
                    selectedKey={String(config.interval_minutes)}
                    onSelectionChange={(key) => updateInterval(Number(key))}
                    className="w-[140px]"
                  >
                    <Label className="sr-only">间隔</Label>
                    <Select.Trigger><Select.Value /><Select.Indicator /></Select.Trigger>
                    <Select.Popover>
                      <ListBox>
                        {[5, 10, 15, 30, 60, 120].map((v) => <ListBox.Item key={v} id={String(v)} textValue={`${v} 分钟`}>{v} 分钟<ListBox.ItemIndicator /></ListBox.Item>)}
                      </ListBox>
                    </Select.Popover>
                  </Select>
                </div>
                <Slider value={config.min_significance} onChange={(v) => updateMinSignificance(v as number)}
                  minValue={0} maxValue={1} step={0.1} className="w-full">
                  <div className="flex items-center justify-between">
                    <Label>最低重要度</Label>
                    <Slider.Output />
                  </div>
                  <Slider.Track><Slider.Fill /><Slider.Thumb /></Slider.Track>
                </Slider>
                <div className="flex items-center justify-between">
                  <span className="text-sm" style={{ color: "var(--yunque-text)" }}>安静时段</span>
                  <span className="text-sm tabular-nums" style={{ color: "var(--yunque-text-muted)" }}>{config.quiet_start}:00 — {config.quiet_end}:00</span>
                </div>
              </div>
            )}
          </Card>

          {/* Stats */}
          {stats && (
            <div className="grid grid-cols-2 gap-2">
              {[
                { label: "总想法", value: stats.total_thoughts },
                { label: "已推送", value: stats.delivered },
                { label: "平均重要度", value: stats.avg_significance ?? 0, fmt: (n: number) => n.toFixed(2) },
                { label: "行动数", value: actionLog.length },
              ].map((s) => (
                <Card key={s.label} className="section-card p-3 text-center">
                  <div className="kpi-value" style={{ fontSize: "var(--text-xl)" }}>
                    {"fmt" in s && s.fmt ? s.fmt(s.value) : s.value}
                  </div>
                  <div className="kpi-sub mt-0.5">{s.label}</div>
                </Card>
              ))}
            </div>
          )}

          {/* Filters */}
          <Card className="section-card p-4 space-y-3">
            <div className="text-xs font-medium uppercase tracking-wider" style={{ color: "var(--yunque-text-muted)" }}>筛选</div>
            <Select selectedKey={filterCategory} onSelectionChange={(key) => setFilterCategory(key as string)} className="w-full" placeholder="全部分类">
              <Label className="sr-only">分类</Label>
              <Select.Trigger><Select.Value /><Select.Indicator /></Select.Trigger>
              <Select.Popover>
                <ListBox>
                  <ListBox.Item id="" textValue="全部分类">全部分类<ListBox.ItemIndicator /></ListBox.Item>
                  {Object.keys(stats?.categories || {}).map((c) => <ListBox.Item key={c} id={c} textValue={c}>{categoryEmojis[c] || ">"} {c}<ListBox.ItemIndicator /></ListBox.Item>)}
                </ListBox>
              </Select.Popover>
            </Select>
            <Select selectedKey={String(filterMinSig)} onSelectionChange={(key) => setFilterMinSig(Number(key))} className="w-full" placeholder="任意重要度">
              <Label className="sr-only">重要度</Label>
              <Select.Trigger><Select.Value /><Select.Indicator /></Select.Trigger>
              <Select.Popover>
                <ListBox>
                  <ListBox.Item id="0" textValue="任意重要度">任意重要度<ListBox.ItemIndicator /></ListBox.Item>
                  <ListBox.Item id="0.3" textValue="≥0.3">≥0.3<ListBox.ItemIndicator /></ListBox.Item>
                  <ListBox.Item id="0.5" textValue="≥0.5">≥0.5<ListBox.ItemIndicator /></ListBox.Item>
                  <ListBox.Item id="0.7" textValue="≥0.7">≥0.7<ListBox.ItemIndicator /></ListBox.Item>
                  <ListBox.Item id="0.9" textValue="≥0.9">≥0.9<ListBox.ItemIndicator /></ListBox.Item>
                </ListBox>
              </Select.Popover>
            </Select>
            <div className="text-xs" style={{ color: "var(--yunque-text-muted)" }}>共 {total} 条</div>
          </Card>

          {/* Action Log */}
          <Card>
            <Button variant="ghost" onPress={() => setActionsOpen(!actionsOpen)} className="w-full flex items-center justify-between p-4 h-auto">
              <div className="flex items-center gap-2">
                <Zap size={14} style={{ color: "var(--yunque-accent)" }} />
                <span className="text-xs font-medium uppercase tracking-wider" style={{ color: "var(--yunque-text)" }}>行动日志</span>
                <Chip size="sm" style={{ background: "rgba(255,255,255,0.04)" }}>{actionLog.length}</Chip>
              </div>
              <ChevronDown size={16} style={{ color: "var(--yunque-text-muted)", transform: actionsOpen ? "rotate(180deg)" : "none", transition: "transform 0.2s" }} />
            </Button>
            {actionsOpen && (
              <div className="px-4 pb-4 border-t" style={{ borderColor: "var(--yunque-border)" }}>
                {actionLog.length === 0 ? (
                  <EmptyState icon={<Zap size={24} style={{ color: "var(--yunque-accent)" }} />} title="暂无行动记录" description="Reverie 启用后，云雀的自主行动将在此展示。可在右侧配置中启用并设置态度阈值。" />
                ) : (
                  <div className="space-y-2 pt-3">
                    {actionLog.slice().reverse().slice(0, 50).map((rec, i) => (
                      <div key={`${rec.thought_id}-${i}`} className="flex items-start gap-3 p-3 rounded-lg" style={{ background: "rgba(255,255,255,0.02)" }}>
                        <div className="mt-0.5 shrink-0">
                          {rec.success ? <CheckCircle2 size={14} style={{ color: "#17c964" }} /> : <XCircle size={14} style={{ color: "#f31260" }} />}
                        </div>
                        <div className="min-w-0 flex-1">
                          <div className="flex items-center gap-2 mb-1 flex-wrap">
                            <Chip size="sm" style={{ background: "rgba(255,255,255,0.04)", color: "var(--yunque-text-muted)" }}>{rec.action.type}</Chip>
                            <span className="text-[10px]" style={{ color: rec.success ? "#17c964" : "#f31260" }}>{rec.success ? "成功" : "失败"}</span>
                          </div>
                          <div className="text-sm" style={{ color: "var(--yunque-text)" }}>
                            <span className="font-medium">{rec.action.key}</span>
                            {rec.action.value && <span className="ml-2" style={{ color: "var(--yunque-text-muted)" }}>→ {rec.action.value}</span>}
                          </div>
                          {rec.error && <div className="text-xs mt-1" style={{ color: "#f31260" }}>{rec.error}</div>}
                          <div className="text-xs mt-1" style={{ color: "var(--yunque-text-muted)" }}>{new Date(rec.at).toLocaleString()}</div>
                        </div>
                      </div>
                    ))}
                  </div>
                )}
              </div>
            )}
          </Card>

        </div>{/* end left sidebar */}

        {/* ── Right main: Thought Journal ── */}
        <div className="lg:col-span-2">
          <Card className="section-card p-5">
            <div className="text-xs font-medium uppercase tracking-wider mb-4" style={{ color: "var(--yunque-text-muted)" }}>思维日记</div>
            {thoughts.length === 0 ? (
              <EmptyState icon={<BrainCircuit size={24} style={{ color: "var(--yunque-accent)" }} />} title="暂无想法记录" description="启用 Reverie 并配置好 LLM 后，云雀会定期产生内心想法。也可以点击右侧「触发思考」手动运行一次。" />
            ) : (
              <div className="space-y-2">
                {thoughts.map((th) => (
                  <div key={th.id} className="flex items-start gap-3 p-3 rounded-lg group" style={{ background: "rgba(255,255,255,0.02)" }}>
                    <div className="mt-1 shrink-0 text-base">{categoryEmojis[th.category] || "..."}</div>
                    <div className="min-w-0 flex-1">
                      <div className="flex items-center gap-2 mb-1 flex-wrap">
                        <Chip size="sm" style={{ background: "rgba(255,255,255,0.04)", color: "var(--yunque-text-muted)" }}>{th.category}</Chip>
                        <span className="text-[10px]" style={{ color: "var(--yunque-text-muted)" }} title={`重要度: ${th.significance}`}>{sigStars(th.significance)}</span>
                        {th.delivered && <Send size={10} style={{ color: "var(--yunque-text-muted)" }} />}
                        {th.trigger && th.trigger !== "timer" && (
                          <Chip size="sm" style={{ background: "rgba(255,255,255,0.04)", color: "var(--yunque-text-muted)" }}>源: {th.trigger}</Chip>
                        )}
                      </div>
                      <div className="text-sm whitespace-pre-wrap" style={{ color: "var(--yunque-text)" }}>{th.content}</div>
                      {th.actions && th.actions.length > 0 && (
                        <div className="flex items-center gap-1.5 mt-2 flex-wrap">
                          {th.actions.map((a, i) => (
                            <Chip key={i} size="sm" style={{ background: "rgba(255,255,255,0.04)", color: "var(--yunque-text-muted)" }}>
                              <Zap size={8} /> {a.type}
                            </Chip>
                          ))}
                        </div>
                      )}
                      <div className="text-xs mt-1" style={{ color: "var(--yunque-text-muted)" }}>{new Date(th.timestamp).toLocaleString()}</div>
                    </div>
                    <Button isIconOnly aria-label="删除" variant="ghost" size="sm" onPress={() => deleteThought(th.id)}
                      className="opacity-0 group-hover:opacity-100">
                      <Trash2 size={12} />
                    </Button>
                  </div>
                ))}
              </div>
            )}
          </Card>
        </div>{/* end right main */}

      </div>{/* end two-col grid */}
    </div>
  );
}
