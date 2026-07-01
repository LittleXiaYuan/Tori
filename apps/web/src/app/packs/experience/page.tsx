"use client";

import { useCallback, useEffect, useState } from "react";
import Link from "next/link";
import { Button, Card, Spinner, Chip } from "@heroui/react";
import { History, ThumbsUp, User, Trophy, AlertTriangle, Send, ClipboardList } from "lucide-react";
import PageHeader from "@/components/page-header";
import { formatErrorMessage } from "@/lib/error-utils";
import { chatPromptHref, taskDetailHref } from "@/lib/pack-action-links";
import { PackAbout, PackSectionTitle, PackStepsGrid, type PackBoundaryItem, type PackStep } from "@/components/packs/pack-page-kit";
import {
  createExperiencePackClient,
  type Recommendation,
  type ScoredLabel,
  type EvaluationEntry,
} from "@/lib/experience-pack-client";

const experience = createExperiencePackClient();

const userFacingSteps: PackStep[] = [
  { key: "recs", label: "看推荐能力", detail: "根据近期反馈，挑出当前最值得继续使用的技能或能力。" },
  { key: "prefs", label: "检查偏好画像", detail: "查看云雀学到的偏好、标签和需要避免的类别。" },
  { key: "review", label: "复盘任务自评", detail: "把任务评分和建议交回 Chat，沉淀成下一次可复用经验。" },
];

const boundaryItems: PackBoundaryItem[] = [
  { key: "profile", label: "不改偏好画像", detail: "不会替你自动修改偏好画像。" },
  { key: "skills", label: "不动技能开关", detail: "不会自动启用或禁用技能。" },
  { key: "lowconf", label: "不当硬决定", detail: "不会把低置信度推荐当成必须执行的决定。" },
  { key: "hide", label: "不隐藏失败", detail: "不会隐藏任务自评中的失败原因。" },
];

const workflowLoopItems: PackStep[] = [
  { key: "recs", label: "看推荐", detail: "先确认云雀建议继续使用哪些能力，以及理由是否可信。" },
  { key: "plan", label: "规划下一步", detail: "把推荐或自评带回 Chat，生成下一次任务计划。" },
  { key: "verify", label: "验证偏好", detail: "在任务结果里确认偏好画像是否真的改善了输出。" },
  { key: "settle", label: "继续沉淀", detail: "把有效经验留在记忆里；不准的地方回到工坊补说明或入口。" },
];

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
      <span className="text-accent">{icon}</span>
      <span className="text-sm font-semibold text-foreground">
        {title}
      </span>
      {hint ? (
        <span className="text-xs text-muted">
          {hint}
        </span>
      ) : null}
    </div>
  );
}

function EmptyState({ text }: { text: string }) {
  return (
    <div className="text-xs px-3 py-6 text-center rounded-xl text-muted bg-surface-secondary">
      {text}
    </div>
  );
}

function RecommendationsColumn({ items, loading }: { items: Recommendation[]; loading: boolean }) {
  return (
    <Card className="section-card p-4">
      <ColumnHeader icon={<ThumbsUp size={16} />} title="推荐" hint="当前最值得用的技能" />
      {loading ? (
        <div className="flex items-center gap-2 text-xs text-muted">
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
                className="rounded-xl px-3 py-2 text-sm bg-surface-secondary border border-border"
              >
                <div className="flex items-center gap-2 mb-1">
                  <Chip size="sm" color={color}>
                    {pct}%
                  </Chip>
                  <span className="text-foreground">{r.item_id}</span>
                  <span className="text-xs ml-auto text-muted">
                    {r.reason}
                  </span>
                </div>
                <div className="text-xs text-muted">
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
        <div className="flex items-center gap-2 text-xs text-muted">
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
      <div className="text-xs mb-1 text-muted">
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
        <div className="flex items-center gap-2 text-xs text-muted">
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
                className="rounded-xl px-3 py-2 text-sm bg-surface-secondary border border-border"
              >
                <div className="flex items-center gap-2 mb-1">
                  <Chip size="sm" color={color}>
                    质量 {qPct}%
                  </Chip>
                  <Chip size="sm">达成 {(e.goal_achieved * 100).toFixed(0)}%</Chip>
                  <Chip size="sm">效率 {(e.efficiency * 100).toFixed(0)}%</Chip>
                  <span className="text-xs ml-auto text-muted">
                    {formatTimestamp(e.created_at)}
                  </span>
                </div>
                {e.reasoning ? (
                  <div className="text-xs text-muted">
                    {e.reasoning}
                  </div>
                ) : null}
                {e.suggestions && e.suggestions.length > 0 ? (
                  <ul className="text-xs mt-1 list-disc list-inside text-muted">
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

      <PackAbout
        chips={<>
          <Chip size="sm" color="success">可直接使用</Chip>
          <Chip size="sm" variant="soft">推荐下一步</Chip>
          <Chip size="sm" variant="soft">可回到 Chat</Chip>
        </>}
        description="它用于把云雀完成任务后的反馈、偏好和自评分数变成可查看的经验面板。当前可以查看推荐能力、偏好画像和任务自评，并把任一条推荐或自评带回 Chat 继续规划下一步。"
        boundaries={boundaryItems}
      />

      <Card variant="default">
        <Card.Content className="flex flex-col gap-4">
          <PackStepsGrid steps={userFacingSteps} columns={3} />
          <div className="text-sm leading-6 text-muted">
            Agent 会根据你的反馈累积经验：动态推荐最值得使用的技能、维护对你的偏好画像，并对每次任务做自我评分。
          </div>
        </Card.Content>
      </Card>

      <Card variant="default">
        <Card.Header className="flex-row flex-wrap items-start justify-between gap-3">
          <div className="flex flex-col gap-1">
            <PackSectionTitle icon={<Trophy size={15} />} tone="accent">从经验到下一次交付</PackSectionTitle>
            <span className="text-xs leading-5 text-muted">经验页不是分数墙；它要帮助你把推荐、偏好和自评变成下一次更稳的任务方案。</span>
          </div>
          <div className="flex flex-wrap gap-2">
            <Link href={chatPromptHref("请根据经验页里的推荐、偏好和最近任务自评，帮我规划下一次任务，并说明哪些经验应该继续保留。")}>
              <Button size="sm" className="btn-accent">
                <Send size={13} /> 带回 Chat
              </Button>
            </Link>
            <Link href="/missions">
              <Button size="sm" variant="outline">
                <ClipboardList size={13} /> 看任务
              </Button>
            </Link>
          </div>
        </Card.Header>
        <Card.Content className="flex flex-col gap-3">
          <PackStepsGrid steps={workflowLoopItems} columns={4} />
          <div className="flex flex-wrap gap-2">
            <Link href="/memory"><Button size="sm" variant="ghost">查看记忆画像</Button></Link>
            <Link href="/trace"><Button size="sm" variant="ghost">核对执行轨迹</Button></Link>
            <Link href="/packs/studio?packId=yunque.pack.experience"><Button size="sm" variant="ghost">让小羽继续改</Button></Link>
          </div>
        </Card.Content>
      </Card>

      {error ? (
        <Card variant="secondary">
          <Card.Content className="flex items-center gap-2 text-sm text-danger">
            <AlertTriangle size={16} /> {error}
          </Card.Content>
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
