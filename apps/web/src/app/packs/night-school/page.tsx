"use client";

import { useCallback, useEffect, useState } from "react";
import Link from "next/link";
import { Button, Card, Spinner, Chip } from "@heroui/react";
import { GraduationCap, MoonStar, Lightbulb, User, AlertTriangle, Send, ClipboardList, Trash2 } from "lucide-react";
import PageHeader from "@/components/page-header";
import { formatErrorMessage } from "@/lib/error-utils";
import { chatPromptHref, taskDetailHref } from "@/lib/pack-action-links";
import { PackAbout, PackSectionTitle, type PackBoundaryItem } from "@/components/packs/pack-page-kit";
import {
  createNightSchoolPackClient,
  type DreamEntry,
  type DistillEntry,
  type TraitEntry,
} from "@/lib/night-school-pack-client";

const nightSchool = createNightSchoolPackClient();

const boundaryItems: PackBoundaryItem[] = [
  { key: "exec", label: "不自动执行任务", detail: "不会在夜间自动执行新任务。" },
  { key: "files", label: "不改真实文件", detail: "不会直接改你的真实文件或配置。" },
  { key: "trust", label: "不当硬规则", detail: "不会把低置信度画像当成硬规则。" },
  { key: "judge", label: "不替你判断", detail: "不会替代你对经验是否有用的判断。" },
];

function applyDistillPrompt(entry: DistillEntry): string {
  return [
    "请把这条夜校蒸馏出的经验应用到我接下来的任务里，并告诉我应该怎么改进工作方式：",
    `类型：${entry.key}`,
    `经验：${entry.content}`,
    entry.task_id ? `来源任务：${entry.task_id}` : "",
  ].filter(Boolean).join("\n");
}

function formatTimestamp(iso: string): string {
  if (!iso) return "—";
  try {
    return new Date(iso).toLocaleString();
  } catch {
    return iso;
  }
}

function EmptyState({ text, action }: { text: string; action?: React.ReactNode }) {
  return (
    <div className="flex flex-col items-center gap-3 rounded-xl bg-surface-secondary px-4 py-8 text-center">
      <span className="text-sm leading-6 text-muted">{text}</span>
      {action}
    </div>
  );
}

function ColumnCard({ icon, title, hint, loading, empty, emptyText, emptyAction, children }: {
  icon: React.ReactNode;
  title: string;
  hint?: string;
  loading: boolean;
  empty: boolean;
  emptyText: string;
  emptyAction?: React.ReactNode;
  children?: React.ReactNode;
}) {
  return (
    <Card variant="default">
      <Card.Header className="flex-row items-center gap-2">
        <PackSectionTitle icon={icon} tone="accent">{title}</PackSectionTitle>
        {hint ? <span className="text-xs text-muted">{hint}</span> : null}
      </Card.Header>
      <Card.Content>
        {loading ? (
          <div className="flex items-center gap-2 text-sm text-muted"><Spinner size="sm" /> 加载中…</div>
        ) : empty ? (
          <EmptyState text={emptyText} action={emptyAction} />
        ) : (
          <div className="flex flex-col gap-2.5">{children}</div>
        )}
      </Card.Content>
    </Card>
  );
}

function DreamsColumn({ items, loading }: { items: DreamEntry[]; loading: boolean }) {
  return (
    <ColumnCard icon={<MoonStar size={16} />} title="梦境" hint="夜间学习周期" loading={loading} empty={items.length === 0} emptyText="夜校在凌晨 02:00–05:00 自动复盘已完成的任务。还没有记录——先去完成一个任务。" emptyAction={<Link href="/missions"><Button size="sm" variant="outline"><ClipboardList size={13} /> 去任务中心</Button></Link>}>
      {items.map((d) => (
        <div key={d.id} className="rounded-xl bg-surface-secondary px-4 py-3">
          <div className="flex flex-wrap items-center gap-1.5">
            <Chip size="sm" variant="soft">探索 {d.explorations_run}</Chip>
            <Chip size="sm" variant="soft">事实 {d.facts_discovered}</Chip>
            <Chip size="sm" variant="soft">想法 {d.thoughts_generated}</Chip>
            <Chip size="sm" variant="soft">技能 {d.skills_suggested}</Chip>
            <span className="ml-auto text-xs text-muted">{formatTimestamp(d.created_at)}</span>
          </div>
        </div>
      ))}
    </ColumnCard>
  );
}

function DistillCard({ entry, accent }: { entry: DistillEntry; accent: "success" | "warning" | "danger" | "default" }) {
  return (
    <div className="rounded-xl bg-surface-secondary px-4 py-3">
      <div className="mb-1.5 flex items-center gap-2">
        <Chip size="sm" color={accent}>{(entry.confidence * 100).toFixed(0)}%</Chip>
        <span className="text-xs text-muted">{entry.key}</span>
        <span className="ml-auto text-xs text-muted">{formatTimestamp(entry.created_at)}</span>
      </div>
      <div className="whitespace-pre-line text-sm leading-6 text-foreground">{entry.content}</div>
      <div className="mt-2 flex flex-wrap gap-2">
        <Link href={chatPromptHref(applyDistillPrompt(entry))}>
          <Button size="sm" variant="ghost"><Send size={13} /> 应用这条经验</Button>
        </Link>
        {entry.task_id ? (
          <Link href={taskDetailHref(entry.task_id)}>
            <Button size="sm" variant="ghost"><ClipboardList size={13} /> 查看来源任务</Button>
          </Link>
        ) : null}
      </div>
    </div>
  );
}

function DistillColumn({ rules, patterns, insights, loading }: { rules: DistillEntry[]; patterns: DistillEntry[]; insights: DistillEntry[]; loading: boolean }) {
  const total = rules.length + patterns.length + insights.length;
  const group = (label: string, items: DistillEntry[], accent: "success" | "warning" | "default") =>
    items.length > 0 ? (
      <div className="flex flex-col gap-2">
        <div className="text-xs text-muted">{label} · {items.length}</div>
        {items.map((it) => <DistillCard key={it.id} entry={it} accent={accent} />)}
      </div>
    ) : null;
  return (
    <ColumnCard icon={<Lightbulb size={16} />} title="经验" hint="任务蒸馏" loading={loading} empty={total === 0} emptyText="完成任务后会自动蒸馏经验。现在还没有——去 Chat 派个任务让云雀跑起来。" emptyAction={<Link href={chatPromptHref("帮我规划并完成一个任务；完成后我想在夜校看到蒸馏出的经验。")}><Button size="sm" variant="outline"><Send size={13} /> 去 Chat 派任务</Button></Link>}>
      {group("模式", patterns, "success")}
      {group("规则", rules, "warning")}
      {group("工具洞见", insights, "default")}
    </ColumnCard>
  );
}

function TraitsColumn({ items, loading, onForget, forgetting }: { items: TraitEntry[]; loading: boolean; onForget: (t: TraitEntry) => void; forgetting: string | null }) {
  return (
    <ColumnCard icon={<User size={16} />} title="画像" hint="对你的偏好" loading={loading} empty={items.length === 0} emptyText="多在 Chat 交流，云雀会逐步建立你的偏好画像。" emptyAction={<Link href="/chat"><Button size="sm" variant="outline"><Send size={13} /> 去 Chat 聊聊</Button></Link>}>
      {items.map((t) => {
        const pct = (t.confidence * 100).toFixed(0);
        const color = t.confidence >= 0.7 ? "success" : t.confidence < 0.4 ? "danger" : "warning";
        return (
          <div key={t.id} className="rounded-xl bg-surface-secondary px-4 py-3">
            <div className="mb-1.5 flex items-center gap-2">
              <Chip size="sm" variant="soft">{t.dimension}</Chip>
              <Chip size="sm" color={color}>{pct}%</Chip>
              <span className="ml-auto text-xs text-muted">命中 {t.hit_count}</span>
            </div>
            <div className="text-sm leading-6 text-foreground">{t.preference}</div>
            <div className="mt-2 flex justify-end">
              <Button size="sm" variant="ghost" isPending={forgetting === t.id} onPress={() => onForget(t)}>
                <Trash2 size={13} /> 不准，忘掉
              </Button>
            </div>
          </div>
        );
      })}
    </ColumnCard>
  );
}

export default function NightSchoolPackPage() {
  const [dreams, setDreams] = useState<DreamEntry[]>([]);
  const [rules, setRules] = useState<DistillEntry[]>([]);
  const [patterns, setPatterns] = useState<DistillEntry[]>([]);
  const [insights, setInsights] = useState<DistillEntry[]>([]);
  const [traits, setTraits] = useState<TraitEntry[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [forgetting, setForgetting] = useState<string | null>(null);

  const refresh = useCallback(async () => {
    try {
      setLoading(true);
      setError(null);
      const [d, dist, t] = await Promise.all([
        nightSchool.dreams(30),
        nightSchool.distill(50),
        nightSchool.traits(50),
      ]);
      setDreams(d.recent);
      setRules(dist.rules);
      setPatterns(dist.patterns);
      setInsights(dist.tool_insights);
      setTraits(t.traits);
    } catch (e: unknown) {
      setError(formatErrorMessage(e, "加载夜校数据失败"));
    } finally {
      setLoading(false);
    }
  }, []);

  const handleForget = useCallback(async (t: TraitEntry) => {
    setForgetting(t.id);
    try {
      await nightSchool.forgetTrait(t.dimension, t.preference);
      setTraits((prev) => prev.filter((x) => x.id !== t.id));
    } catch (e) {
      setError(formatErrorMessage(e, "删除画像失败"));
    } finally {
      setForgetting(null);
    }
  }, []);

  useEffect(() => {
    refresh();
  }, [refresh]);

  return (
    <div className="page-root space-y-6 animate-fade-in-up">
      <PageHeader
        icon={<GraduationCap size={20} />}
        title="夜校"
        actions={<div className="flex flex-wrap gap-2">
          <Link href={chatPromptHref("请从夜校里挑一条最有价值的经验，应用到我接下来的任务计划里，并列出需要验证的检查项。")}>
            <Button size="sm" className="btn-accent"><Send size={14} /> 带回 Chat</Button>
          </Link>
          <Link href="/missions"><Button size="sm" variant="outline"><ClipboardList size={14} /> 看任务</Button></Link>
        </div>}
      />

      <PackAbout
        chips={<>
          <Chip size="sm" color="success">可直接使用</Chip>
          <Chip size="sm" variant="soft">复盘任务</Chip>
          <Chip size="sm" variant="soft">可带回 Chat</Chip>
        </>}
        description="把已完成任务里的经验、失败模式和偏好整理出来，让云雀下次做得更稳。可查看夜间复盘、蒸馏规则、工具洞察和画像，并把任一条经验带回 Chat。"
        boundaries={boundaryItems}
      />

      <div className="flex items-center gap-2 px-1">
        <PackSectionTitle icon={<MoonStar size={15} />} tone="accent">夜间复盘 · 经验 · 画像</PackSectionTitle>
      </div>

      {error ? (
        <Card variant="secondary">
          <Card.Content className="flex items-center gap-2 text-sm text-danger">
            <AlertTriangle size={16} /> {error}
          </Card.Content>
        </Card>
      ) : null}

      <div className="grid grid-cols-1 gap-4 lg:grid-cols-3">
        <DreamsColumn items={dreams} loading={loading} />
        <DistillColumn rules={rules} patterns={patterns} insights={insights} loading={loading} />
        <TraitsColumn items={traits} loading={loading} onForget={handleForget} forgetting={forgetting} />
      </div>
    </div>
  );
}
