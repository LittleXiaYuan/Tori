"use client";

import { useCallback, useEffect, useState } from "react";
import Link from "next/link";
import { Button, Card, Spinner, Chip } from "@heroui/react";
import { Sparkles, Compass, Brain, MoonStar, AlertTriangle, Send } from "lucide-react";
import PageHeader from "@/components/page-header";
import { formatErrorMessage } from "@/lib/error-utils";
import { chatPromptHref } from "@/lib/pack-action-links";
import {
  createInnerLifePackClient,
  type CuriosityQuestion,
  type TimelineEntry,
} from "@/lib/inner-life-pack-client";

const innerLife = createInnerLifePackClient();

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

function CuriosityColumn({ items, loading }: { items: CuriosityQuestion[]; loading: boolean }) {
  return (
    <Card className="section-card p-4">
      <ColumnHeader icon={<Compass size={16} />} title="好奇心" hint="待探索的问题" />
      {loading ? (
        <div className="flex items-center gap-2 text-xs" style={{ color: "var(--yunque-text-muted)" }}>
          <Spinner size="sm" /> 加载中…
        </div>
      ) : items.length === 0 ? (
        <EmptyState text="目前没有待探索的问题。空闲时段会自动生成。" />
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
        <EmptyState text="还没有反思记录。每次对话结束后会自动生成。" />
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
        <EmptyState text="还没有夜游记录。02:00–05:00 期间会自动运行。" />
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
      <PageHeader icon={<Sparkles size={20} />} title="内在生活" />

      <Card className="section-card p-4 text-sm" style={{ color: "var(--yunque-text-muted)" }}>
        在这里查看 Agent 的内心活动：好奇心想去探索的问题、每次对话后的自我反思，以及夜间梦境周期发现的事实与技能缺口。
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
