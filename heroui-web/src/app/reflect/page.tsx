"use client";

import { useState, useRef } from "react";
import { api, type ExperienceItem, type ExperienceStats } from "@/lib/api";
import { Card, Button, Chip, Select, Label, ListBox, Input } from "@heroui/react";
import EmptyState from "@/components/empty-state";
import { Lightbulb, Search, CheckCircle2, XCircle, Tag, Clock, Sparkles, RefreshCw } from "lucide-react";
import { relTime } from "@/lib/constants";
import PageHeader from "@/components/page-header";
import { useApiData } from "@/lib/use-api-data";

const EXPERIENCE_LIST_LIMIT = 50;
const STRATEGY_LIMIT = 8;

const outcomeColor: Record<string, string> = { success: "#17c964", failure: "#f31260", partial: "#ffaa00", neutral: "#9ca3af" };
const outcomeLabel: Record<string, string> = { success: "成功", failure: "失败", partial: "部分成功", neutral: "中性" };
const categoryEmoji: Record<string, string> = {
  task_success: "[OK]", task_failure: "[FAIL]", gap_resolved: "[FIX]", retry_pattern: "[RETRY]", skill_gap: "[PKG]", llm_insight: "*",
};

export default function ReflectPage() {
  const [source, setSource] = useState("");
  const [category, setCategory] = useState("");
  const [outcome, setOutcome] = useState("");
  const [search, setSearch] = useState("");
  const searchTimer = useRef<ReturnType<typeof setTimeout>>(undefined);
  const debouncedSearch = (val: string) => {
    setSearch(val);
    clearTimeout(searchTimer.current);
    searchTimer.current = setTimeout(() => refresh(), 400);
  };

  const { data, loading, refresh } = useApiData(
    async () => {
      const [statsRes, listRes, stratRes] = await Promise.all([
        api.getExperiences({ stats: true }),
        api.getExperiences({ source: source || undefined, category: category || undefined, outcome: outcome || undefined, q: search || undefined, limit: EXPERIENCE_LIST_LIMIT }),
        api.getStrategies({ limit: STRATEGY_LIMIT }),
      ]);
      const stats = ("total" in statsRes && "by_source" in statsRes) ? statsRes as ExperienceStats : null;
      const experiences = ("experiences" in listRes) ? ((listRes as { experiences: ExperienceItem[] }).experiences || []) : [];
      const strategies = typeof stratRes === "string" ? stratRes : ((stratRes as { strategies: string }).strategies || "");
      return { stats, experiences, strategies };
    },
    { stats: null as ExperienceStats | null, experiences: [] as ExperienceItem[], strategies: "" },
    [source, category, outcome, search],
  );
  const { experiences, stats, strategies } = data;

  const statCards = stats ? [
    { label: "总计", value: stats.total, color: "var(--yunque-accent)" },
    { label: "近7天", value: stats.recent_7d, color: "#a78bfa" },
    { label: "成功", value: stats.by_outcome?.success || 0, color: "#17c964" },
    { label: "失败", value: stats.by_outcome?.failure || 0, color: "#f31260" },
  ] : [];

  return (
    <div className="page-root space-y-5 animate-fade-in-up">
      <PageHeader icon={<Lightbulb size={20} />} title="反思学习" onRefresh={refresh} />

      {/* Stats Grid */}
      {stats && (
        <div className="kpi-grid">
          {statCards.map((s, i) => (
            <Card key={i} className="section-card p-4 text-center">
              <div className="kpi-label mb-1">{s.label}</div>
              <div className="kpi-value" style={{ color: s.color }}>{s.value}</div>
            </Card>
          ))}
        </div>
      )}

      {/* Filters */}
      <Card className="section-card p-4">
        <div className="flex flex-wrap gap-3">
          <Select selectedKey={source} onSelectionChange={(key) => setSource(key as string)} className="w-[180px]" placeholder="全部来源">
            <Label className="sr-only">来源</Label>
            <Select.Trigger><Select.Value /><Select.Indicator /></Select.Trigger>
            <Select.Popover>
              <ListBox>
                <ListBox.Item id="" textValue="全部来源">全部来源<ListBox.ItemIndicator /></ListBox.Item>
                {stats && Object.keys(stats.by_source || {}).map((k) => <ListBox.Item key={k} id={k} textValue={k}>{k}<ListBox.ItemIndicator /></ListBox.Item>)}
              </ListBox>
            </Select.Popover>
          </Select>
          <Select selectedKey={category} onSelectionChange={(key) => setCategory(key as string)} className="w-[180px]" placeholder="全部分类">
            <Label className="sr-only">分类</Label>
            <Select.Trigger><Select.Value /><Select.Indicator /></Select.Trigger>
            <Select.Popover>
              <ListBox>
                <ListBox.Item id="" textValue="全部分类">全部分类<ListBox.ItemIndicator /></ListBox.Item>
                {stats && Object.keys(stats.by_category || {}).map((k) => <ListBox.Item key={k} id={k} textValue={k}>{k}<ListBox.ItemIndicator /></ListBox.Item>)}
              </ListBox>
            </Select.Popover>
          </Select>
          <Select selectedKey={outcome} onSelectionChange={(key) => setOutcome(key as string)} className="w-[180px]" placeholder="全部结果">
            <Label className="sr-only">结果</Label>
            <Select.Trigger><Select.Value /><Select.Indicator /></Select.Trigger>
            <Select.Popover>
              <ListBox>
                <ListBox.Item id="" textValue="全部结果">全部结果<ListBox.ItemIndicator /></ListBox.Item>
                <ListBox.Item id="success" textValue="成功">成功<ListBox.ItemIndicator /></ListBox.Item>
                <ListBox.Item id="failure" textValue="失败">失败<ListBox.ItemIndicator /></ListBox.Item>
                <ListBox.Item id="partial" textValue="部分成功">部分成功<ListBox.ItemIndicator /></ListBox.Item>
                <ListBox.Item id="neutral" textValue="中性">中性<ListBox.ItemIndicator /></ListBox.Item>
              </ListBox>
            </Select.Popover>
          </Select>
          <div className="relative flex-1 min-w-[200px]">
            <Search size={14} className="absolute left-3 top-1/2 -translate-y-1/2" style={{ color: "var(--yunque-text-muted)" }} />
            <Input type="text" value={search} onChange={(e) => debouncedSearch(e.target.value)} placeholder="搜索经验..."
              className="w-full pl-8" />
          </div>
        </div>
      </Card>

      <div className="grid grid-cols-1 lg:grid-cols-3 gap-4">
        {/* Experience List */}
        <div className="lg:col-span-2 space-y-3">
          {experiences.length === 0 && !loading && (
            <EmptyState icon={<Lightbulb size={24} style={{ color: "var(--yunque-accent)" }} />} title="暂无经验数据" description="完成几次任务后，云雀会自动反思并生成经验总结与策略建议。试试在聊天或任务中完成一些操作吧！" />
          )}
          {experiences.map((exp) => (
            <Card key={exp.id} className="section-card p-4">
              <div className="flex items-start gap-3">
                <span className="text-xl mt-0.5">{categoryEmoji[exp.category] || "#"}</span>
                <div className="flex-1 min-w-0">
                  <div className="flex items-center gap-2 mb-1 flex-wrap">
                    <Chip size="sm"
                      style={{ background: `${outcomeColor[exp.outcome] || "#9ca3af"}20`, color: outcomeColor[exp.outcome] || "#9ca3af" }}>
                      {exp.outcome === "success" ? <CheckCircle2 size={10} /> : exp.outcome === "failure" ? <XCircle size={10} /> : null} {outcomeLabel[exp.outcome] || exp.outcome}
                    </Chip>
                    <Chip size="sm" style={{ background: "rgba(0,111,238,0.1)", color: "var(--yunque-accent)" }}>{exp.category}</Chip>
                    <span className="text-xs" style={{ color: "var(--yunque-text-muted)" }}>{exp.source}</span>
                  </div>
                  <p className="text-sm mt-1" style={{ color: "var(--yunque-text)" }}>{exp.lesson}</p>
                  {exp.context && <p className="text-xs mt-1" style={{ color: "var(--yunque-text-muted)" }}>{exp.context}</p>}
                  <div className="flex items-center gap-2 mt-2 flex-wrap">
                    {exp.tags?.map((tag) => (
                      <span key={tag} className="inline-flex items-center gap-1 text-xs px-1.5 py-0.5 rounded"
                        style={{ background: "rgba(255,255,255,0.04)", color: "var(--yunque-text-muted)" }}>
                        <Tag size={10} />{tag}
                      </span>
                    ))}
                    <span className="text-xs ml-auto flex items-center gap-1" style={{ color: "var(--yunque-text-muted)" }}>
                      <Clock size={10} />{relTime(exp.created_at)}
                    </span>
                  </div>
                </div>
              </div>
            </Card>
          ))}
        </div>

        {/* Strategy Panel */}
        <div>
          <Card className="section-card p-5">
            <div className="flex items-center gap-2 mb-3">
              <Sparkles size={16} style={{ color: "var(--yunque-accent)" }} />
              <span className="font-semibold text-sm" style={{ color: "var(--yunque-text)" }}>编译策略</span>
            </div>
            {strategies ? (
              <div className="text-sm whitespace-pre-wrap leading-relaxed" style={{ color: "var(--yunque-text)" }}>{strategies}</div>
            ) : (
              <EmptyState icon={<Sparkles size={24} style={{ color: "var(--yunque-accent)" }} />} title="暂无策略" description="触发反思后将生成策略" />
            )}
          </Card>
        </div>
      </div>
    </div>
  );
}
