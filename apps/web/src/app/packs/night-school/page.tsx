"use client";

import { useCallback, useEffect, useState } from "react";
import Link from "next/link";
import { Button, Card, Spinner, Chip } from "@heroui/react";
import { GraduationCap, MoonStar, Lightbulb, User, AlertTriangle, Send, ClipboardList } from "lucide-react";
import PageHeader from "@/components/page-header";
import { formatErrorMessage } from "@/lib/error-utils";
import { chatPromptHref, taskDetailHref } from "@/lib/pack-action-links";
import {
  createNightSchoolPackClient,
  type DreamEntry,
  type DistillEntry,
  type TraitEntry,
} from "@/lib/night-school-pack-client";

const nightSchool = createNightSchoolPackClient();

const userFacingSteps = [
  {
    title: "1. 看夜间复盘",
    body: "查看云雀从已完成任务中整理出的梦境、规则和模式。",
  },
  {
    title: "2. 应用蒸馏经验",
    body: "把某条经验带回 Chat，让云雀按它改进下一次任务。",
  },
  {
    title: "3. 检查学到的画像",
    body: "确认偏好特征是否合理，避免长期学习方向跑偏。",
  },
];

const boundaryItems = [
  "不会在夜间自动执行新任务。",
  "不会直接改你的真实文件或配置。",
  "不会把低置信度画像当成硬规则。",
  "不会替代你对经验是否有用的判断。",
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
    const d = new Date(iso);
    return d.toLocaleString();
  } catch {
    return iso;
  }
}

function ColumnHeader({
  icon,
  title,
  hint,
}: {
  icon: React.ReactNode;
  title: string;
  hint?: string;
}) {
  return (
    <div className="flex items-center gap-2 mb-3">
      <span style={{ color: "var(--yunque-accent)" }}>{icon}</span>
      <span className="text-sm font-semibold" style={{ color: "var(--yunque-text)" }}>
        {title}
      </span>
      {hint ? (
        <span className="text-xs" style={{ color: "var(--yunque-text-muted)" }}>
          {hint}
        </span>
      ) : null}
    </div>
  );
}

function EmptyState({ text }: { text: string }) {
  return (
    <div
      className="text-xs px-3 py-6 text-center rounded-md"
      style={{ color: "var(--yunque-text-muted)", background: "rgba(255,255,255,0.02)" }}
    >
      {text}
    </div>
  );
}

function DreamsColumn({ items, loading }: { items: DreamEntry[]; loading: boolean }) {
  return (
    <Card className="section-card p-4">
      <ColumnHeader icon={<MoonStar size={16} />} title="梦境" hint="夜间学习周期" />
      {loading ? (
        <div className="flex items-center gap-2 text-xs" style={{ color: "var(--yunque-text-muted)" }}>
          <Spinner size="sm" /> 加载中…
        </div>
      ) : items.length === 0 ? (
        <EmptyState text="还没有夜间学习记录。02:00–05:00 期间会自动运行。" />
      ) : (
        <div className="space-y-2">
          {items.map((d) => (
            <div
              key={d.id}
              className="rounded-md px-3 py-2 text-sm"
              style={{ background: "rgba(255,255,255,0.02)", border: "1px solid var(--yunque-border)" }}
            >
              <div className="flex items-center gap-2 mb-1 flex-wrap">
                <Chip size="sm">探索 {d.explorations_run}</Chip>
                <Chip size="sm">事实 {d.facts_discovered}</Chip>
                <Chip size="sm">想法 {d.thoughts_generated}</Chip>
                <Chip size="sm">技能 {d.skills_suggested}</Chip>
                <span className="text-xs ml-auto" style={{ color: "var(--yunque-text-muted)" }}>
                  {formatTimestamp(d.created_at)}
                </span>
              </div>
            </div>
          ))}
        </div>
      )}
    </Card>
  );
}

function DistillColumn({
  rules,
  patterns,
  insights,
  loading,
}: {
  rules: DistillEntry[];
  patterns: DistillEntry[];
  insights: DistillEntry[];
  loading: boolean;
}) {
  const total = rules.length + patterns.length + insights.length;
  return (
    <Card className="section-card p-4">
      <ColumnHeader icon={<Lightbulb size={16} />} title="经验" hint="任务蒸馏" />
      {loading ? (
        <div className="flex items-center gap-2 text-xs" style={{ color: "var(--yunque-text-muted)" }}>
          <Spinner size="sm" /> 加载中…
        </div>
      ) : total === 0 ? (
        <EmptyState text="还没有蒸馏出的经验。每次任务完成后会自动分析。" />
      ) : (
        <div className="space-y-3">
          {patterns.length > 0 ? (
            <div>
              <div className="text-xs mb-1" style={{ color: "var(--yunque-text-muted)" }}>模式 · {patterns.length}</div>
              <div className="space-y-2">
                {patterns.map((p) => (
                  <DistillCard key={p.id} entry={p} accent="success" />
                ))}
              </div>
            </div>
          ) : null}
          {rules.length > 0 ? (
            <div>
              <div className="text-xs mb-1" style={{ color: "var(--yunque-text-muted)" }}>规则 · {rules.length}</div>
              <div className="space-y-2">
                {rules.map((r) => (
                  <DistillCard key={r.id} entry={r} accent="warning" />
                ))}
              </div>
            </div>
          ) : null}
          {insights.length > 0 ? (
            <div>
              <div className="text-xs mb-1" style={{ color: "var(--yunque-text-muted)" }}>工具洞见 · {insights.length}</div>
              <div className="space-y-2">
                {insights.map((i) => (
                  <DistillCard key={i.id} entry={i} accent="default" />
                ))}
              </div>
            </div>
          ) : null}
        </div>
      )}
    </Card>
  );
}

function DistillCard({
  entry,
  accent,
}: {
  entry: DistillEntry;
  accent: "success" | "warning" | "danger" | "default";
}) {
  return (
    <div
      className="rounded-md px-3 py-2 text-sm"
      style={{ background: "rgba(255,255,255,0.02)", border: "1px solid var(--yunque-border)" }}
    >
      <div className="flex items-center gap-2 mb-1">
        <Chip size="sm" color={accent}>
          {(entry.confidence * 100).toFixed(0)}%
        </Chip>
        <span className="text-xs" style={{ color: "var(--yunque-text-muted)" }}>
          {entry.key}
        </span>
        <span className="text-xs ml-auto" style={{ color: "var(--yunque-text-muted)" }}>
          {formatTimestamp(entry.created_at)}
        </span>
      </div>
      <div className="whitespace-pre-line" style={{ color: "var(--yunque-text)" }}>
        {entry.content}
      </div>
      <div className="mt-2 flex flex-wrap gap-2">
        <Link href={chatPromptHref(applyDistillPrompt(entry))}>
          <Button size="sm" variant="ghost">
            <Send size={13} /> 应用这条经验
          </Button>
        </Link>
        {entry.task_id ? (
          <Link href={taskDetailHref(entry.task_id)}>
            <Button size="sm" variant="ghost">
              <ClipboardList size={13} /> 查看来源任务
            </Button>
          </Link>
        ) : null}
      </div>
    </div>
  );
}

function TraitsColumn({ items, loading }: { items: TraitEntry[]; loading: boolean }) {
  return (
    <Card className="section-card p-4">
      <ColumnHeader icon={<User size={16} />} title="画像" hint="对你的偏好" />
      {loading ? (
        <div className="flex items-center gap-2 text-xs" style={{ color: "var(--yunque-text-muted)" }}>
          <Spinner size="sm" /> 加载中…
        </div>
      ) : items.length === 0 ? (
        <EmptyState text="还没有学到关于你的偏好。多聊几次会逐渐建立画像。" />
      ) : (
        <div className="space-y-2">
          {items.map((t) => {
            const pct = (t.confidence * 100).toFixed(0);
            const color = t.confidence >= 0.7 ? "success" : t.confidence < 0.4 ? "danger" : "warning";
            return (
              <div
                key={t.id}
                className="rounded-md px-3 py-2 text-sm"
                style={{ background: "rgba(255,255,255,0.02)", border: "1px solid var(--yunque-border)" }}
              >
                <div className="flex items-center gap-2 mb-1">
                  <Chip size="sm">{t.dimension}</Chip>
                  <Chip size="sm" color={color}>
                    {pct}%
                  </Chip>
                  <span className="text-xs ml-auto" style={{ color: "var(--yunque-text-muted)" }}>
                    命中 {t.hit_count}
                  </span>
                </div>
                <div style={{ color: "var(--yunque-text)" }}>{t.preference}</div>
              </div>
            );
          })}
        </div>
      )}
    </Card>
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

  useEffect(() => {
    refresh();
  }, [refresh]);

  return (
    <div className="page-root space-y-6 animate-fade-in-up">
      <PageHeader icon={<GraduationCap size={20} />} title="夜校" />

      <Card className="section-card overflow-hidden p-0">
        <div className="grid gap-0 lg:grid-cols-[minmax(0,1fr)_320px]">
          <div className="p-5">
            <div className="flex flex-wrap items-center gap-2">
              <Chip size="sm" style={{ background: "rgba(34,197,94,0.10)", color: "var(--yunque-success)" }}>可直接使用</Chip>
              <Chip size="sm" variant="soft">复盘任务</Chip>
              <Chip size="sm" variant="soft">可带回 Chat</Chip>
            </div>
            <div className="mt-3 text-base font-semibold" style={{ color: "var(--yunque-text)" }}>
              这个能力包现在适合做什么
            </div>
            <div className="mt-2 max-w-3xl text-sm leading-6" style={{ color: "var(--yunque-text-secondary)" }}>
              它用于把已完成任务里的经验、失败模式和用户偏好整理出来，让云雀下次做得更稳。当前可以查看夜间复盘、蒸馏规则、工具洞察和画像特征，并把任一条经验带回 Chat 应用到新任务。
            </div>
            <div className="mt-4 grid gap-3 md:grid-cols-3">
              {userFacingSteps.map((item) => (
                <div key={item.title} className="rounded-lg p-3" style={{ background: "var(--yunque-bg-hover)", border: "1px solid var(--yunque-border)" }}>
                  <div className="text-sm font-medium" style={{ color: "var(--yunque-text)" }}>{item.title}</div>
                  <div className="mt-2 text-xs leading-5" style={{ color: "var(--yunque-text-muted)" }}>{item.body}</div>
                </div>
              ))}
            </div>
          </div>
          <div className="p-5" style={{ background: "rgba(245,158,11,0.08)", borderLeft: "1px solid var(--yunque-border)" }}>
            <div className="mb-3 text-sm font-semibold" style={{ color: "var(--yunque-text)" }}>当前不会做什么</div>
            <div className="space-y-2 text-xs leading-5" style={{ color: "var(--yunque-text-secondary)" }}>
              {boundaryItems.map((item) => <div key={item}>{item}</div>)}
            </div>
          </div>
        </div>
      </Card>

      <Card className="section-card p-4 text-sm" style={{ color: "var(--yunque-text-muted)" }}>
        Agent 会在夜间复盘：从已完成任务中蒸馏可复用的规则与模式，并通过对话学习关于你的偏好画像。
      </Card>

      {error ? (
        <Card className="p-4" style={{ background: "rgba(239,68,68,0.05)" }}>
          <div className="flex items-center gap-2">
            <AlertTriangle size={16} style={{ color: "var(--yunque-danger)" }} />
            <span className="text-sm" style={{ color: "var(--yunque-danger)" }}>
              {error}
            </span>
          </div>
        </Card>
      ) : null}

      <div className="grid grid-cols-1 lg:grid-cols-3 gap-4">
        <DreamsColumn items={dreams} loading={loading} />
        <DistillColumn rules={rules} patterns={patterns} insights={insights} loading={loading} />
        <TraitsColumn items={traits} loading={loading} />
      </div>
    </div>
  );
}
