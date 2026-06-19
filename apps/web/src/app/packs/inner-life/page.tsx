"use client";

import { useCallback, useEffect, useState } from "react";
import Link from "next/link";
import { Button, Card, Spinner, Chip } from "@heroui/react";
import { Sparkles, Compass, Brain, MoonStar, AlertTriangle, Send, ClipboardList, RefreshCw } from "lucide-react";
import PageHeader from "@/components/page-header";
import PackSurfaceGuide from "@/components/pack-surface-guide";
import { formatErrorMessage } from "@/lib/error-utils";
import { chatPromptHref } from "@/lib/pack-action-links";
import {
  createInnerLifePackClient,
  type CuriosityQuestion,
  type TimelineEntry,
} from "@/lib/inner-life-pack-client";

const innerLife = createInnerLifePackClient();

const useCaseCards = [
  {
    title: "把好奇心变成任务",
    desc: "看到一个待探索问题后，可以直接让云雀继续调研、拆成任务，或沉淀到知识库。",
  },
  {
    title: "把反思变成改进",
    desc: "反思记录用于回看一次对话哪里做得好、哪里卡住，再把经验写进下一次提示。",
  },
  {
    title: "把夜游变成缺口清单",
    desc: "夜间周期会汇总事实、想法和技能缺口，适合第二天挑选值得补的能力。",
  },
];

function formatTimestamp(iso: string): string {
  if (!iso) return "—";
  try {
    const d = new Date(iso);
    return d.toLocaleString();
  } catch {
    return iso;
  }
}

function asString(v: unknown, fallback = ""): string {
  if (typeof v === "string") return v;
  if (typeof v === "number" || typeof v === "boolean") return String(v);
  return fallback;
}

function asNumber(v: unknown): number | null {
  if (typeof v === "number") return v;
  if (typeof v === "string" && v !== "" && !Number.isNaN(Number(v))) return Number(v);
  return null;
}

function exploreCuriosityPrompt(q: CuriosityQuestion): string {
  return [
    "请基于这条好奇心问题继续探索，并给出可执行的下一步：",
    `问题：${q.question}`,
    q.context ? `背景：${q.context}` : "",
    q.related_to && q.related_to.length > 0 ? `相关线索：${q.related_to.join("、")}` : "",
  ].filter(Boolean).join("\n");
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

function EmptyState({ text, action }: { text: string; action?: React.ReactNode }) {
  return (
    <div
      className="text-xs px-3 py-6 text-center rounded-md"
      style={{ color: "var(--yunque-text-muted)", background: "rgba(255,255,255,0.02)" }}
    >
      <div>{text}</div>
      {action ? <div className="mt-3 flex justify-center">{action}</div> : null}
    </div>
  );
}

function CuriosityColumn({ items, loading }: { items: CuriosityQuestion[]; loading: boolean }) {
  return (
    <Card className="section-card p-4">
      <ColumnHeader icon={<Compass size={16} />} title="好奇心" hint="待探索的问题" />
      {loading ? (
        <div className="flex items-center gap-2 text-xs" style={{ color: "var(--yunque-text-muted)" }}>
          <Spinner size="sm" /> 加载中…
        </div>
      ) : items.length === 0 ? (
        <EmptyState
          text="目前没有待探索的问题。你也可以先从一个开放问题开始，让云雀帮你生成探索方向。"
          action={(
            <Link href={chatPromptHref("帮我基于当前工作内容生成 3 个值得探索的问题，并说明每个问题能带来什么行动价值。")}>
              <Button size="sm" variant="ghost">
                <Send size={13} /> 生成探索问题
              </Button>
            </Link>
          )}
        />
      ) : (
        <div className="space-y-2">
          {items.map((q, idx) => (
            <div
              key={`${q.question}-${idx}`}
              className="rounded-md px-3 py-2 text-sm"
              style={{ background: "rgba(255,255,255,0.02)", border: "1px solid var(--yunque-border)" }}
            >
              <div className="flex items-center gap-2 mb-1">
                <Chip size="sm">
                  {q.category}
                </Chip>
                <span className="text-xs" style={{ color: "var(--yunque-text-muted)" }}>
                  优先级 {q.priority.toFixed(2)}
                </span>
              </div>
              <div style={{ color: "var(--yunque-text)" }}>{q.question}</div>
              {q.context ? (
                <div className="text-xs mt-1" style={{ color: "var(--yunque-text-muted)" }}>
                  {q.context}
                </div>
              ) : null}
              <div className="mt-2">
                <Link href={chatPromptHref(exploreCuriosityPrompt(q))}>
                  <Button size="sm" variant="ghost">
                    <Send size={13} /> 继续探索
                  </Button>
                </Link>
              </div>
            </div>
          ))}
        </div>
      )}
    </Card>
  );
}

function ReflectionColumn({ items, loading }: { items: TimelineEntry[]; loading: boolean }) {
  return (
    <Card className="section-card p-4">
      <ColumnHeader icon={<Brain size={16} />} title="反思" hint="对话后的自我评估" />
      {loading ? (
        <div className="flex items-center gap-2 text-xs" style={{ color: "var(--yunque-text-muted)" }}>
          <Spinner size="sm" /> 加载中…
        </div>
      ) : items.length === 0 ? (
        <EmptyState
          text="还没有反思记录。完成一次对话或任务后，这里会显示可复用经验。"
          action={(
            <Link href={chatPromptHref("请回顾我们最近一次任务，把做得好的地方、卡住的地方和下一次改进建议整理成经验。")}>
              <Button size="sm" variant="ghost">
                <Send size={13} /> 生成一次复盘
              </Button>
            </Link>
          )}
        />
      ) : (
        <div className="space-y-2">
          {items.map((e) => {
            const quality = asNumber(e.payload?.quality);
            const satisfied = e.payload?.satisfied === true;
            const intent = asString(e.payload?.user_intent);
            return (
              <div
                key={e.id}
                className="rounded-md px-3 py-2 text-sm"
                style={{ background: "rgba(255,255,255,0.02)", border: "1px solid var(--yunque-border)" }}
              >
                <div className="flex items-center gap-2 mb-1">
                  {quality !== null ? (
                    <Chip size="sm" color={quality >= 8 ? "success" : quality < 5 ? "danger" : "warning"}>
                      质量 {quality}/10
                    </Chip>
                  ) : null}
                  <Chip size="sm" color={satisfied ? "success" : "default"}>
                    {satisfied ? "满意" : "未满意"}
                  </Chip>
                  <span className="text-xs ml-auto" style={{ color: "var(--yunque-text-muted)" }}>
                    {formatTimestamp(e.created_at)}
                  </span>
                </div>
                {intent ? (
                  <div className="text-xs" style={{ color: "var(--yunque-text-muted)" }}>
                    {intent}
                  </div>
                ) : null}
              </div>
            );
          })}
        </div>
      )}
    </Card>
  );
}

function DreamingColumn({ items, loading }: { items: TimelineEntry[]; loading: boolean }) {
  return (
    <Card className="section-card p-4">
      <ColumnHeader icon={<MoonStar size={16} />} title="夜游" hint="夜间梦境周期" />
      {loading ? (
        <div className="flex items-center gap-2 text-xs" style={{ color: "var(--yunque-text-muted)" }}>
          <Spinner size="sm" /> 加载中…
        </div>
      ) : items.length === 0 ? (
        <EmptyState
          text="还没有夜游记录。夜间周期会自动运行；也可以先让云雀列出当前能力缺口。"
          action={(
            <Link href={chatPromptHref("请基于当前任务和记忆，列出云雀最值得补齐的 5 个能力缺口，并给出优先级。")}>
              <Button size="sm" variant="ghost">
                <ClipboardList size={13} /> 查看能力缺口
              </Button>
            </Link>
          )}
        />
      ) : (
        <div className="space-y-2">
          {items.map((e) => {
            const facts = asNumber(e.payload?.facts_discovered) ?? 0;
            const explorations = asNumber(e.payload?.explorations_run) ?? 0;
            const thoughts = asNumber(e.payload?.thoughts_generated) ?? 0;
            const skills = asNumber(e.payload?.skills_suggested) ?? 0;
            return (
              <div
                key={e.id}
                className="rounded-md px-3 py-2 text-sm"
                style={{ background: "rgba(255,255,255,0.02)", border: "1px solid var(--yunque-border)" }}
              >
                <div className="flex items-center gap-2 mb-1 flex-wrap">
                  <Chip size="sm">探索 {explorations}</Chip>
                  <Chip size="sm">事实 {facts}</Chip>
                  <Chip size="sm">想法 {thoughts}</Chip>
                  <Chip size="sm">技能 {skills}</Chip>
                  <span className="text-xs ml-auto" style={{ color: "var(--yunque-text-muted)" }}>
                    {formatTimestamp(e.created_at)}
                  </span>
                </div>
              </div>
            );
          })}
        </div>
      )}
    </Card>
  );
}

export default function InnerLifePackPage() {
  const [pendingQuestions, setPendingQuestions] = useState<CuriosityQuestion[]>([]);
  const [reflections, setReflections] = useState<TimelineEntry[]>([]);
  const [dreams, setDreams] = useState<TimelineEntry[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const refresh = useCallback(async () => {
    try {
      setLoading(true);
      setError(null);
      const [c, r, d] = await Promise.all([
        innerLife.curiosity(8),
        innerLife.reflection(30),
        innerLife.dreaming(30),
      ]);
      setPendingQuestions(c.pending);
      setReflections(r.recent);
      setDreams(d.recent);
    } catch (e: unknown) {
      setError(formatErrorMessage(e, "加载内在生活数据失败"));
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    refresh();
  }, [refresh]);

  return (
    <div className="page-root space-y-6 animate-fade-in-up">
      <PageHeader
        icon={<Sparkles size={20} />}
        title="内在生活"
        description="把云雀的好奇心、反思和夜间学习变成可继续探索的任务线索。"
        actions={(
          <Button variant="ghost" onPress={refresh}>
            <RefreshCw size={14} /> 刷新
          </Button>
        )}
      />

      <PackSurfaceGuide surface="innerLife" compact />

      <Card className="section-card overflow-hidden p-0">
        <div className="grid gap-0 lg:grid-cols-[minmax(0,1fr)_260px]">
          <div className="p-5">
            <div className="mb-4">
              <div className="text-base font-semibold" style={{ color: "var(--yunque-text)" }}>这个能力包有什么用</div>
              <div className="mt-2 max-w-3xl text-sm leading-6" style={{ color: "var(--yunque-text-secondary)" }}>
                它不是给你看“云雀在想什么”而已，而是把空闲探索、任务复盘和夜间学习整理成下一步可执行线索。
              </div>
            </div>
            <div className="grid gap-3 md:grid-cols-3">
              {useCaseCards.map((item) => (
                <div key={item.title} className="rounded-lg p-3" style={{ background: "var(--yunque-bg-hover)", border: "1px solid var(--yunque-border)" }}>
                  <div className="text-sm font-medium" style={{ color: "var(--yunque-text)" }}>{item.title}</div>
                  <div className="mt-2 text-xs leading-5" style={{ color: "var(--yunque-text-muted)" }}>{item.desc}</div>
                </div>
              ))}
            </div>
          </div>
          <div className="grid grid-cols-3 gap-2 p-5 lg:grid-cols-1" style={{ background: "rgba(59,130,246,0.05)", borderLeft: "1px solid var(--yunque-border)" }}>
            <div>
              <div className="text-2xl font-semibold" style={{ color: "var(--yunque-text)" }}>{pendingQuestions.length}</div>
              <div className="text-xs" style={{ color: "var(--yunque-text-muted)" }}>待探索问题</div>
            </div>
            <div>
              <div className="text-2xl font-semibold" style={{ color: "var(--yunque-text)" }}>{reflections.length}</div>
              <div className="text-xs" style={{ color: "var(--yunque-text-muted)" }}>反思记录</div>
            </div>
            <div>
              <div className="text-2xl font-semibold" style={{ color: "var(--yunque-text)" }}>{dreams.length}</div>
              <div className="text-xs" style={{ color: "var(--yunque-text-muted)" }}>夜游记录</div>
            </div>
          </div>
        </div>
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
        <CuriosityColumn items={pendingQuestions} loading={loading} />
        <ReflectionColumn items={reflections} loading={loading} />
        <DreamingColumn items={dreams} loading={loading} />
      </div>
    </div>
  );
}
