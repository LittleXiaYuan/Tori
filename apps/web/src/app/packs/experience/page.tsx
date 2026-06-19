"use client";

import { useCallback, useEffect, useState } from "react";
import Link from "next/link";
import { Button, Card, Spinner, Chip } from "@heroui/react";
import { History, ThumbsUp, User, Trophy, AlertTriangle, Send, ClipboardList } from "lucide-react";
import PageHeader from "@/components/page-header";
import { formatErrorMessage } from "@/lib/error-utils";
import { chatPromptHref, taskDetailHref } from "@/lib/pack-action-links";
import {
  createExperiencePackClient,
  type Recommendation,
  type ScoredLabel,
  type EvaluationEntry,
} from "@/lib/experience-pack-client";

const experience = createExperiencePackClient();

function useRecommendationPrompt(item: Recommendation): string {
  return [
    "请基于云雀当前推荐的能力，帮我设计下一步任务方案：",
    `推荐项：${item.item_id}`,
    `推荐原因：${item.reason}`,
    `置信度：${(item.confidence * 100).toFixed(0)}%`,
  ].join("\n");
}

function improveFromEvaluationPrompt(item: EvaluationEntry): string {
  return [
    "请根据这次任务自评，帮我总结可复用经验，并给出下一次怎么做会更好：",
    `任务 ID：${item.task_id}`,
    `质量：${(item.quality_score * 100).toFixed(0)}%`,
    `目标达成：${(item.goal_achieved * 100).toFixed(0)}%`,
    `效率：${(item.efficiency * 100).toFixed(0)}%`,
    item.reasoning ? `自评原因：${item.reasoning}` : "",
    item.suggestions?.length ? `建议：${item.suggestions.join("；")}` : "",
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

function RecommendationsColumn({ items, loading }: { items: Recommendation[]; loading: boolean }) {
  return (
    <Card className="section-card p-4">
      <ColumnHeader icon={<ThumbsUp size={16} />} title="推荐" hint="当前最值得用的技能" />
      {loading ? (
        <div className="flex items-center gap-2 text-xs" style={{ color: "var(--yunque-text-muted)" }}>
          <Spinner size="sm" /> 加载中…
        </div>
      ) : items.length === 0 ? (
        <EmptyState text="还没有可推荐的技能。多用几次后会出现。" />
      ) : (
        <div className="space-y-2">
          {items.map((r) => {
            const pct = (r.score * 100).toFixed(0);
            const color = r.score >= 0.7 ? "success" : r.score < 0.4 ? "danger" : "warning";
            return (
              <div
                key={r.item_id}
                className="rounded-md px-3 py-2 text-sm"
                style={{ background: "rgba(255,255,255,0.02)", border: "1px solid var(--yunque-border)" }}
              >
                <div className="flex items-center gap-2 mb-1">
                  <Chip size="sm" color={color}>
                    {pct}%
                  </Chip>
                  <span style={{ color: "var(--yunque-text)" }}>{r.item_id}</span>
                  <span className="text-xs ml-auto" style={{ color: "var(--yunque-text-muted)" }}>
                    {r.reason}
                  </span>
                </div>
                <div className="text-xs" style={{ color: "var(--yunque-text-muted)" }}>
                  置信度 {(r.confidence * 100).toFixed(0)}%
                </div>
                <div className="mt-2">
                  <Link href={chatPromptHref(useRecommendationPrompt(r))}>
                    <Button size="sm" variant="ghost">
                      <Send size={13} /> 用它规划任务
                    </Button>
                  </Link>
                </div>
              </div>
            );
          })}
        </div>
      )}
    </Card>
  );
}

function PreferencesColumn({
  preferred,
  tags,
  avoid,
  interactions,
  loading,
}: {
  preferred: ScoredLabel[];
  tags: ScoredLabel[];
  avoid: ScoredLabel[];
  interactions: number;
  loading: boolean;
}) {
  const total = preferred.length + tags.length + avoid.length;
  return (
    <Card className="section-card p-4">
      <ColumnHeader icon={<User size={16} />} title="偏好" hint={`累计 ${interactions} 次反馈`} />
      {loading ? (
        <div className="flex items-center gap-2 text-xs" style={{ color: "var(--yunque-text-muted)" }}>
          <Spinner size="sm" /> 加载中…
        </div>
      ) : total === 0 ? (
        <EmptyState text="尚未学到偏好。继续交互会逐渐建立画像。" />
      ) : (
        <div className="space-y-3">
          {preferred.length > 0 ? (
            <PreferenceList title="喜欢的类别" items={preferred.slice(0, 8)} positive />
          ) : null}
          {tags.length > 0 ? <PreferenceList title="喜欢的标签" items={tags.slice(0, 8)} positive /> : null}
          {avoid.length > 0 ? <PreferenceList title="想避免的类别" items={avoid.slice(0, 8)} positive={false} /> : null}
        </div>
      )}
    </Card>
  );
}

function PreferenceList({
  title,
  items,
  positive,
}: {
  title: string;
  items: ScoredLabel[];
  positive: boolean;
}) {
  return (
    <div>
      <div className="text-xs mb-1" style={{ color: "var(--yunque-text-muted)" }}>
        {title} · {items.length}
      </div>
      <div className="flex flex-wrap gap-1.5">
        {items.map((item) => (
          <Chip key={item.label} size="sm" color={positive ? "success" : "danger"}>
            {item.label} · {item.score.toFixed(1)}
          </Chip>
        ))}
      </div>
    </div>
  );
}

function EvaluationsColumn({ items, loading }: { items: EvaluationEntry[]; loading: boolean }) {
  return (
    <Card className="section-card p-4">
      <ColumnHeader icon={<Trophy size={16} />} title="自评" hint="任务完成后的评分" />
      {loading ? (
        <div className="flex items-center gap-2 text-xs" style={{ color: "var(--yunque-text-muted)" }}>
          <Spinner size="sm" /> 加载中…
        </div>
      ) : items.length === 0 ? (
        <EmptyState text="还没有自评结果。任务完成后会自动评估。" />
      ) : (
        <div className="space-y-2">
          {items.map((e) => {
            const qPct = (e.quality_score * 100).toFixed(0);
            const color = e.quality_score >= 0.7 ? "success" : e.quality_score < 0.4 ? "danger" : "warning";
            return (
              <div
                key={e.id}
                className="rounded-md px-3 py-2 text-sm"
                style={{ background: "rgba(255,255,255,0.02)", border: "1px solid var(--yunque-border)" }}
              >
                <div className="flex items-center gap-2 mb-1">
                  <Chip size="sm" color={color}>
                    质量 {qPct}%
                  </Chip>
                  <Chip size="sm">达成 {(e.goal_achieved * 100).toFixed(0)}%</Chip>
                  <Chip size="sm">效率 {(e.efficiency * 100).toFixed(0)}%</Chip>
                  <span className="text-xs ml-auto" style={{ color: "var(--yunque-text-muted)" }}>
                    {formatTimestamp(e.created_at)}
                  </span>
                </div>
                {e.reasoning ? (
                  <div className="text-xs" style={{ color: "var(--yunque-text-muted)" }}>
                    {e.reasoning}
                  </div>
                ) : null}
                {e.suggestions && e.suggestions.length > 0 ? (
                  <ul className="text-xs mt-1 list-disc list-inside" style={{ color: "var(--yunque-text-muted)" }}>
                    {e.suggestions.map((s, i) => (
                      <li key={i}>{s}</li>
                    ))}
                  </ul>
                ) : null}
                <div className="mt-2 flex flex-wrap gap-2">
                  <Link href={chatPromptHref(improveFromEvaluationPrompt(e))}>
                    <Button size="sm" variant="ghost">
                      <Send size={13} /> 让云雀改进
                    </Button>
                  </Link>
                  {e.task_id ? (
                    <Link href={taskDetailHref(e.task_id)}>
                      <Button size="sm" variant="ghost">
                        <ClipboardList size={13} /> 查看任务
                      </Button>
                    </Link>
                  ) : null}
                </div>
              </div>
            );
          })}
        </div>
      )}
    </Card>
  );
}

export default function ExperiencePackPage() {
  const [recs, setRecs] = useState<Recommendation[]>([]);
  const [preferred, setPreferred] = useState<ScoredLabel[]>([]);
  const [tags, setTags] = useState<ScoredLabel[]>([]);
  const [avoid, setAvoid] = useState<ScoredLabel[]>([]);
  const [interactions, setInteractions] = useState(0);
  const [evals, setEvals] = useState<EvaluationEntry[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const refresh = useCallback(async () => {
    try {
      setLoading(true);
      setError(null);
      const [r, p, ev] = await Promise.all([
        experience.recommendations(10),
        experience.preferences(),
        experience.evaluations(30),
      ]);
      setRecs(r.recommendations);
      setPreferred(p.preferred_categories);
      setTags(p.preferred_tags);
      setAvoid(p.avoid_categories);
      setInteractions(p.interaction_count);
      setEvals(ev.recent);
    } catch (e: unknown) {
      setError(formatErrorMessage(e, "加载经验数据失败"));
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    refresh();
  }, [refresh]);

  return (
    <div className="page-root space-y-6 animate-fade-in-up">
      <PageHeader icon={<History size={20} />} title="经验" />

      <Card className="section-card p-4 text-sm" style={{ color: "var(--yunque-text-muted)" }}>
        Agent 会根据你的反馈累积经验：动态推荐最值得使用的技能、维护对你的偏好画像，并对每次任务做自我评分。
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
        <RecommendationsColumn items={recs} loading={loading} />
        <PreferencesColumn
          preferred={preferred}
          tags={tags}
          avoid={avoid}
          interactions={interactions}
          loading={loading}
        />
        <EvaluationsColumn items={evals} loading={loading} />
      </div>
    </div>
  );
}
