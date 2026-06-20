"use client";

import { useEffect, useState, useCallback } from "react";
import { Card, Button, Spinner, Chip, Tooltip, Tabs, ProgressBar, Switch, TextField, Input, Label, TextArea } from "@heroui/react";
import { api, type EmotionHistoryEntry, type HeartbeatLog, type PersonaMemoryBlock, type MemorySearchResult } from "@/lib/api";
import { Brain, Heart, Clock, RefreshCw, Layers, Zap, Settings2, Search, Plus, Trash2, Archive } from "lucide-react";
import { showToast } from "@/components/toast-provider";
import EmptyState from "@/components/empty-state";
import PageHeader from "@/components/page-header";

export default function MemoryPage() {
  const [emotions, setEmotions] = useState<EmotionHistoryEntry[]>([]);
  const [heartbeats, setHeartbeats] = useState<HeartbeatLog[]>([]);
  const [memoryBlocks, setMemoryBlocks] = useState<PersonaMemoryBlock[]>([]);
  const [heartbeatRunning, setHeartbeatRunning] = useState(false);
  const [triggering, setTriggering] = useState(false);
  const [loading, setLoading] = useState(true);
  const [tab, setTab] = useState("memory");
  const [memStats, setMemStats] = useState<{ short: number; mid: number; long: number }>({ short: 0, mid: 0, long: 0 });
  // Search
  const [searchQuery, setSearchQuery] = useState("");
  const [searchResults, setSearchResults] = useState<MemorySearchResult[]>([]);
  const [searching, setSearching] = useState(false);
  // Add memory
  const [showAdd, setShowAdd] = useState(false);
  const [addContent, setAddContent] = useState("");
  const [adding, setAdding] = useState(false);
  // Compact
  const [compacting, setCompacting] = useState(false);

  const load = useCallback(async () => {
    try {
      const [emo, hb, mb, hbStatus, ms] = await Promise.all([
        api.getEmotionHistory().catch(() => ({ entries: [], summary: {}, total: 0 })),
        api.getHeartbeatLogs().catch(() => []),
        api.getMemoryPersona().catch(() => []),
        api.getHeartbeat().catch(() => ({ running: false })),
        api.memoryStats().catch(() => ({ short: 0, mid: 0, long: 0 })),
      ]);
      setEmotions(emo.entries || []);
      setHeartbeats(Array.isArray(hb) ? hb : []);
      setMemoryBlocks(Array.isArray(mb) ? mb : []);
      setHeartbeatRunning(hbStatus.running);
      setMemStats(ms);
    } catch { /* offline */ }
    finally { setLoading(false); }
  }, []);

  useEffect(() => { load(); }, [load]);

  const toggleHeartbeat = async (val: boolean) => {
    try { await api.updateHeartbeat(val); setHeartbeatRunning(val); } catch (e) { showToast(e instanceof Error ? e.message : "操作失败", "error"); }
  };

  const triggerHeartbeatManual = async () => {
    setTriggering(true);
    try { const log = await api.triggerHeartbeat(); setHeartbeats((prev) => [log, ...prev]); } catch (e) { showToast(e instanceof Error ? e.message : "触发失败", "error"); }
    setTriggering(false);
  };

  const handleSearch = async () => {
    if (!searchQuery.trim()) return;
    setSearching(true);
    try {
      const res = await api.memorySearch(searchQuery);
      setSearchResults(res.results || []);
    } catch { setSearchResults([]); }
    setSearching(false);
  };

  const handleAddMemory = async () => {
    if (!addContent.trim()) return;
    setAdding(true);
    try {
      await api.memoryAdd(addContent);
      setAddContent("");
      setShowAdd(false);
      load();
    } catch (e) { showToast(e instanceof Error ? e.message : "保存失败", "error"); }
    setAdding(false);
  };

  const handleCompact = async () => {
    setCompacting(true);
    try { await api.memoryCompact(); load(); } catch (e) { showToast(e instanceof Error ? e.message : "整理失败", "error"); }
    setCompacting(false);
  };

  const emotionEmoji: Record<string, string> = {
    joy: "😊", sadness: "😢", anger: "😠",
    fear: "😰", disgust: "🤢", surprise: "😮",
    love: "🥰", neutral: "😐",
  };

  if (loading) {
    return <div className="flex items-center justify-center h-[60vh]"><Spinner size="lg" /></div>;
  }

  return (
    <div className="page-root space-y-6 animate-fade-in-up">
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-3">
          <Brain size={20} style={{ color: "var(--yunque-accent)" }} />
          <h1 className="page-title">{"记忆"}</h1>
        </div>
        <div className="flex gap-2">
          <Tooltip delay={0}>
            <Button isIconOnly variant="ghost" size="sm" onPress={() => setShowAdd(!showAdd)}>
              <Plus size={14} />
            </Button>
            <Tooltip.Content>{"添加记忆"}</Tooltip.Content>
          </Tooltip>
          <Tooltip delay={0}>
            <Button isIconOnly variant="ghost" size="sm" isPending={compacting} onPress={handleCompact}>
              <Archive size={14} />
            </Button>
            <Tooltip.Content>{"整理记忆"}</Tooltip.Content>
          </Tooltip>
          <Tooltip delay={0}>
            <Button isIconOnly variant="ghost" size="sm" onPress={() => { setLoading(true); load(); }}>
              <RefreshCw size={14} />
            </Button>
            <Tooltip.Content>{"刷新"}</Tooltip.Content>
          </Tooltip>
        </div>
      </div>


      {/* Add memory */}
      {showAdd && (
        <Card className="section-card p-5 space-y-3 animate-scale-in">
          <TextField value={addContent} onChange={setAddContent}>
            <Label>{"新记忆内容"}</Label>
            <TextArea rows={3} placeholder={"输入要存储的记忆内容..."} />
          </TextField>
          <div className="flex justify-end gap-2">
            <Button variant="ghost" size="sm" onPress={() => setShowAdd(false)}>{"取消"}</Button>
            <Button size="sm" isPending={adding} onPress={handleAddMemory} className="btn-accent">{"保存"}</Button>
          </div>
        </Card>
      )}

      {/* Search bar */}
      <div className="flex items-center gap-2">
        <div className="flex-1">
          <TextField value={searchQuery} onChange={setSearchQuery}>
            <Input placeholder={"搜索记忆..."} onKeyDown={(e: React.KeyboardEvent) => e.key === "Enter" && handleSearch()} />
          </TextField>
        </div>
        <Button size="sm" isPending={searching} onPress={handleSearch} className="btn-accent">
          <Search size={14} /> {"搜索"}
        </Button>
      </div>

      {/* Search results */}
      {searchResults.length > 0 && (
        <Card className="section-card p-5">
          <h3 className="text-sm font-medium mb-3" style={{ color: "var(--yunque-text)" }}>{"搜索结果"} ({searchResults.length})</h3>
          <div className="space-y-2">
            {searchResults.map((r) => (
              <div key={r.id} className="p-3 rounded-lg hover-lift" style={{ background: "rgba(255,255,255,0.02)" }}>
                <div className="flex items-center justify-between mb-1">
                  <Chip size="sm" style={{ background: "rgba(0,111,238,0.1)", color: "var(--yunque-accent)", fontSize: "var(--text-2xs)" }}>
                    {(r.score * 100).toFixed(0)}% 匹配
                  </Chip>
                  <span className="text-xs" style={{ color: "var(--yunque-text-muted)" }}>
                    {r.created_at ? new Date(r.created_at).toLocaleString() : ""}
                  </span>
                </div>
                <div className="text-sm" style={{ color: "var(--yunque-text)" }}>{r.content}</div>
              </div>
            ))}
          </div>
        </Card>
      )}

      {/* Stats */}
      <div className="grid grid-cols-2 lg:grid-cols-4 gap-4 stagger-children">
        {(() => {
          const total = memStats.short + memStats.mid + memStats.long || 1;
          return [
            { icon: <Layers size={14} style={{ color: "var(--yunque-warning)" }} />, label: "短期记忆", value: memStats.short, pct: memStats.short / total, color: "var(--yunque-warning)" },
            { icon: <Brain size={14} style={{ color: "var(--yunque-accent)" }} />, label: "中期记忆", value: memStats.mid, pct: memStats.mid / total, color: "var(--yunque-accent)" },
            { icon: <Heart size={14} style={{ color: "#f472b6" }} />, label: "长期记忆", value: memStats.long, pct: memStats.long / total, color: "#f472b6" },
            { icon: <Layers size={14} style={{ color: "var(--yunque-success)" }} />, label: "人格记忆块", value: memoryBlocks.length, pct: null, color: "var(--yunque-success)" },
          ].map((s) => (
            <Card key={s.label} className="section-card p-4 hover-lift">
              <div className="flex items-center gap-2 mb-2">
                {s.icon}
                <span className="kpi-label">{s.label}</span>
              </div>
              <div className="kpi-value" style={{ fontSize: "var(--text-xl)" }}>{s.value}</div>
              {s.pct !== null && (
                <ProgressBar
                  value={s.pct * 100}
                  maxValue={100}
                  aria-label={s.label}
                  style={{ "--progressbar-fill-color": s.color } as any}
                >
                  <ProgressBar.Track>
                    <ProgressBar.Fill />
                  </ProgressBar.Track>
                </ProgressBar>
              )}
            </Card>
          ));
        })()}
      </div>

      {/* Tabs */}
      <Tabs selectedKey={tab} onSelectionChange={(k) => setTab(k as string)}>
        <Tabs.ListContainer>
          <Tabs.List aria-label="记忆与情感">
            <Tabs.Tab id="memory">{"记忆块"}<Tabs.Indicator /></Tabs.Tab>
            <Tabs.Tab id="emotions"><Tabs.Separator />{"情感历史"}<Tabs.Indicator /></Tabs.Tab>
            <Tabs.Tab id="heartbeat"><Tabs.Separator />{"心跳日志"}<Tabs.Indicator /></Tabs.Tab>
          </Tabs.List>
        </Tabs.ListContainer>

        <Tabs.Panel id="memory">
        <Card className="section-card p-5 mt-4">
          {memoryBlocks.length === 0 ? (
            <EmptyState icon={<Brain size={24} style={{ color: "var(--yunque-accent)" }} />} title="暂无记忆块" description="对话中的关键信息会自动提取并存储为记忆块，试试先聊天吧" />
          ) : (
            <div className="space-y-3 stagger-children">
              {memoryBlocks.map((b, i) => (
                <div key={i} className="p-4 rounded-lg hover-lift" style={{ background: "rgba(255,255,255,0.02)" }}>
                  <div className="flex items-center gap-2 mb-2">
                    <Chip size="sm" style={{ background: "rgba(167,139,250,0.1)", color: "#a78bfa", fontSize: 10 }}>{b.label || `Block ${i + 1}`}</Chip>
                    {b.version && <span className="text-xs" style={{ color: "var(--yunque-text-muted)" }}>v{b.version}</span>}
                  </div>
                  <div className="text-sm whitespace-pre-wrap" style={{ color: "var(--yunque-text)" }}>{b.content}</div>
                </div>
              ))}
            </div>
          )}
        </Card>
        </Tabs.Panel>

        <Tabs.Panel id="emotions">
        <div className="space-y-4 mt-4">
          {/* Emotion distribution bar chart */}
          {emotions.length > 0 && (
            <Card className="section-card p-5">
              <h3 className="text-sm font-medium mb-3" style={{ color: "var(--yunque-text)" }}>{"情感分布"}</h3>
              <div className="space-y-2">
                {Object.entries(
                  emotions.reduce((acc: Record<string, number>, e) => {
                    acc[e.emotion] = (acc[e.emotion] || 0) + 1;
                    return acc;
                  }, {})
                ).sort(([, a], [, b]) => (b as number) - (a as number)).map(([emotion, count]) => {
                  const pct = ((count as number) / emotions.length) * 100;
                  return (
                    <div key={emotion} className="flex items-center gap-3">
                      <span className="text-lg w-8">{emotionEmoji[emotion] || "\u{1F610}"}</span>
                      <span className="text-xs w-16 capitalize" style={{ color: "var(--yunque-text-secondary)" }}>{emotion}</span>
                      <div className="flex-1 h-4 rounded-full overflow-hidden" style={{ background: "rgba(255,255,255,0.05)" }}>
                        <div className="h-full rounded-full transition-all duration-500" style={{ width: `${pct}%`, background: "var(--yunque-accent)" }} />
                      </div>
                      <span className="text-xs font-mono w-10 text-right" style={{ color: "var(--yunque-text-muted)" }}>{pct.toFixed(0)}%</span>
                    </div>
                  );
                })}
              </div>
            </Card>
          )}

          {/* Emotion list */}
          <Card className="section-card p-5">
          {emotions.length === 0 ? (
            <EmptyState icon={<Heart size={24} style={{ color: "var(--yunque-accent)" }} />} title="暂无情感记录" description="与云雀对话后，情感分析结果将自动展示在这里" />
          ) : (
            <div className="space-y-2 stagger-children">
              {emotions.slice(0, 30).map((e, i) => (
                <div key={i} className="flex items-center gap-3 p-3 rounded-lg hover-lift" style={{ background: "rgba(255,255,255,0.02)" }}>
                  <span className="text-xl">{emotionEmoji[e.emotion] || "😐"}</span>
                  <div className="flex-1 min-w-0">
                    <div className="flex items-center gap-2">
                      <Chip size="sm" style={{ background: "rgba(0,111,238,0.1)", color: "var(--yunque-accent)", fontSize: 10 }}>{e.emotion}</Chip>
                      {e.confidence && (
                        <span className="text-xs" style={{ color: "var(--yunque-text-muted)" }}>
                          {(e.confidence * 100).toFixed(0)}%
                        </span>
                      )}
                    </div>
                    {e.trigger && <div className="text-xs mt-1 truncate" style={{ color: "var(--yunque-text-secondary)" }}>{e.trigger}</div>}
                  </div>
                  <span className="text-xs shrink-0" style={{ color: "var(--yunque-text-muted)" }}>
                    {e.created_at ? new Date(e.created_at).toLocaleString() : ""}
                  </span>
                </div>
              ))}
            </div>
          )}
        </Card>
        </div>
        </Tabs.Panel>

        <Tabs.Panel id="heartbeat">
        <div className="space-y-4 mt-4">
          {/* Heartbeat controls */}
          <Card className="section-card p-5">
            <div className="flex items-center justify-between">
              <div className="flex items-center gap-3">
                <Settings2 size={16} style={{ color: "var(--yunque-accent)" }} />
                <span className="text-sm font-medium" style={{ color: "var(--yunque-text)" }}>{"心跳配置"}</span>
              </div>
              <div className="flex items-center gap-3">
                <Button
                  size="sm"
                  isPending={triggering}
                  onPress={triggerHeartbeatManual}
                  className="gap-1.5"
                  style={{ background: "rgba(0,111,238,0.12)", color: "var(--yunque-accent)" }}
                >
                  <Zap size={12} /> {"手动触发"}
                </Button>
                <div className="flex items-center gap-2">
                  <span className="text-xs" style={{ color: "var(--yunque-text-muted)" }}>{heartbeatRunning ? "已启用" : "已禁用"}</span>
                  <Switch isSelected={heartbeatRunning} onChange={toggleHeartbeat} size="sm">
                    <Switch.Control><Switch.Thumb /></Switch.Control>
                  </Switch>
                </div>
              </div>
            </div>
          </Card>

          {/* Heartbeat log list */}
          <Card className="section-card p-5">
          {heartbeats.length === 0 ? (
            <EmptyState icon={<Zap size={24} style={{ color: "var(--yunque-accent)" }} />} title="暂无心跳日志" description="在设置中启用心跳自检后，云雀会定期自我检查并记录日志" />
          ) : (
            <div className="space-y-2 stagger-children">
              {heartbeats.slice(0, 30).map((h, i) => (
                <div key={i} className="p-3 rounded-lg hover-lift" style={{ background: "rgba(255,255,255,0.02)" }}>
                  <div className="flex items-center justify-between mb-1">
                    <Chip size="sm" style={{ background: h.status === "ok" ? "rgba(34,197,94,0.1)" : "rgba(239,68,68,0.1)", color: h.status === "ok" ? "var(--yunque-success)" : "var(--yunque-danger)", fontSize: 10 }}>
                      {h.status || "ok"}
                    </Chip>
                    <span className="text-xs" style={{ color: "var(--yunque-text-muted)" }}>
                      {h.timestamp ? new Date(h.timestamp).toLocaleString() : ""}
                    </span>
                  </div>
                  {h.summary && <div className="text-sm" style={{ color: "var(--yunque-text)" }}>{h.summary}</div>}
                </div>
              ))}
            </div>
          )}
        </Card>
        </div>
        </Tabs.Panel>
      </Tabs>
    </div>
  );
}
