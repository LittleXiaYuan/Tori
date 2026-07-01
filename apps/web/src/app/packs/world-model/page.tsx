"use client";

import { useCallback, useEffect, useState } from "react";
import Link from "next/link";
import { Card, Spinner, Chip, Input, Button } from "@heroui/react";
import { Globe, Clock, AlertOctagon, GitBranch, AlertTriangle, Send, ClipboardList, Activity } from "lucide-react";
import PageHeader from "@/components/page-header";
import { formatErrorMessage } from "@/lib/error-utils";
import { chatPromptHref, taskDetailHref, traceTaskHref } from "@/lib/pack-action-links";
import { PackAbout, PackStepsGrid, type PackBoundaryItem, type PackStep } from "@/components/packs/pack-page-kit";
import {
  createWorldModelPackClient,
  type WorldStateEntry,
  type FailurePattern,
  type CausalChain,
} from "@/lib/world-model-pack-client";

const worldClient = createWorldModelPackClient();

function investigatePatternPrompt(pattern: FailurePattern): string {
  return [
    "请基于这条世界模型发现的失败模式，给我一个规避方案和下一步验证计划：",
    `因果：${pattern.cause_kind} → ${pattern.effect_kind}`,
    `机制：${pattern.mechanism}`,
    `出现次数：${pattern.occurrences}`,
    pattern.task_ids?.length ? `相关任务：${pattern.task_ids.join("、")}` : "",
  ].filter(Boolean).join("\n");
}

function fixRootCausePrompt(taskId: string, chain: CausalChain): string {
  return [
    "请基于这条根因链，帮我制定修复方案。先说明根因，再给出最小可执行下一步：",
    `任务 ID：${taskId}`,
    `根因：${chain.root_cause}`,
    `最终影响：${chain.final_effect}`,
    chain.links.length
      ? `因果链：${chain.links.map((link) => `${link.cause_kind} -> ${link.effect_kind}: ${link.mechanism}`).join("；")}`
      : "",
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

const userFacingSteps: PackStep[] = [
  { key: "facts", label: "查看云雀相信的事实", detail: "检查文件、API、配置等世界状态是否过期或置信度太低。" },
  { key: "patterns", label: "发现反复失败模式", detail: "把跨任务出现的因果共性找出来，减少同类错误反复发生。" },
  { key: "rootcause", label: "追溯单个任务根因", detail: "输入失败任务 ID，生成修复方案、打开任务详情或查看轨迹。" },
];

const boundaryItems: PackBoundaryItem[] = [
  { key: "write", label: "不改真实资源", detail: "不会自动改文件、数据库、API 或配置。" },
  { key: "lowconf", label: "不当事实执行", detail: "不会把低置信度状态当成事实直接执行。" },
  { key: "rerun", label: "不自动重跑", detail: "不会自动重跑失败任务。" },
  { key: "judge", label: "不替你判断", detail: "不会替代人工确认根因和修复方案。" },
];

function StateColumn({
  entries,
  staleKeys,
  loading,
}: {
  entries: WorldStateEntry[];
  staleKeys: Set<string>;
  loading: boolean;
}) {
  return (
    <Card className="section-card p-4">
      <ColumnHeader icon={<Globe size={16} />} title="世界状态" hint={`共 ${entries.length} 条`} />
      {loading ? (
        <div className="flex items-center gap-2 text-xs text-muted">
          <Spinner size="sm" /> 加载中…
        </div>
      ) : entries.length === 0 ? (
        <EmptyState text="还没有跟踪的世界状态。" />
      ) : (
        <div className="space-y-2 max-h-[600px] overflow-y-auto">
          {entries.map((s) => {
            const stale = staleKeys.has(s.key);
            const conf = (s.confidence * 100).toFixed(0);
            return (
              <div
                key={s.key}
                className="rounded-xl px-3 py-2 text-sm bg-surface-secondary border border-border"
              >
                <div className="flex items-center gap-2 mb-1 flex-wrap">
                  <Chip size="sm">{s.kind}</Chip>
                  <Chip size="sm" color={s.confidence >= 0.7 ? "success" : "warning"}>
                    {conf}%
                  </Chip>
                  {stale ? (
                    <Chip size="sm" color="danger">
                      过期
                    </Chip>
                  ) : null}
                  <span className="text-xs ml-auto text-muted">
                    {formatTimestamp(s.last_verified)}
                  </span>
                </div>
                <div className="font-mono text-xs text-foreground">
                  {s.key}
                </div>
                <div className="text-xs mt-1 break-words line-clamp-3 text-muted">
                  {s.value}
                </div>
              </div>
            );
          })}
        </div>
      )}
    </Card>
  );
}

function FailurePatternsColumn({ items, loading }: { items: FailurePattern[]; loading: boolean }) {
  return (
    <Card className="section-card p-4">
      <ColumnHeader icon={<AlertOctagon size={16} />} title="失败模式" hint="跨任务的因果共性" />
      {loading ? (
        <div className="flex items-center gap-2 text-xs text-muted">
          <Spinner size="sm" /> 加载中…
        </div>
      ) : items.length === 0 ? (
        <EmptyState text="还没有发现失败模式。任务失败累积后会自动聚合。" />
      ) : (
        <div className="space-y-2 max-h-[600px] overflow-y-auto">
          {items.map((p, idx) => (
            <div
              key={`${p.cause_kind}-${p.effect_kind}-${idx}`}
              className="rounded-xl px-3 py-2 text-sm bg-surface-secondary border border-border"
            >
              <div className="flex items-center gap-2 mb-1 flex-wrap">
                <Chip size="sm" color="danger">
                  ×{p.occurrences}
                </Chip>
                <span className="text-xs font-mono text-muted">
                  {p.cause_kind} → {p.effect_kind}
                </span>
              </div>
              <div className="text-xs text-foreground">
                {p.mechanism}
              </div>
              {p.task_ids && p.task_ids.length > 0 ? (
                <div className="text-xs mt-1 font-mono text-muted">
                  涉及任务: {p.task_ids.slice(0, 3).join(", ")}
                  {p.task_ids.length > 3 ? ` (+${p.task_ids.length - 3})` : ""}
                </div>
              ) : null}
              <div className="mt-2 flex flex-wrap gap-2">
                <Link href={chatPromptHref(investigatePatternPrompt(p))}>
                  <Button size="sm" variant="ghost">
                    <Send size={13} /> 生成规避方案
                  </Button>
                </Link>
                {p.task_ids?.[0] ? (
                  <Link href={taskDetailHref(p.task_ids[0])}>
                    <Button size="sm" variant="ghost">
                      <ClipboardList size={13} /> 查看样例任务
                    </Button>
                  </Link>
                ) : null}
              </div>
            </div>
          ))}
        </div>
      )}
    </Card>
  );
}

function RootCauseColumn({
  taskId,
  setTaskId,
  chain,
  loading,
  onLookup,
}: {
  taskId: string;
  setTaskId: (v: string) => void;
  chain: CausalChain | null;
  loading: boolean;
  onLookup: () => void;
}) {
  return (
    <Card className="section-card p-4">
      <ColumnHeader icon={<GitBranch size={16} />} title="根因追溯" hint="给定失败任务" />
      <div className="flex items-center gap-2 mb-3">
        <Input
          placeholder="输入 task_id"
          value={taskId}
          onChange={(e) => setTaskId(e.target.value)}
          className="flex-1"
        />
        <Button size="sm" onPress={onLookup} isDisabled={!taskId || loading}>
          查询
        </Button>
      </div>
      {loading ? (
        <div className="flex items-center gap-2 text-xs text-muted">
          <Spinner size="sm" /> 加载中…
        </div>
      ) : !chain ? (
        <EmptyState text="输入一个失败任务的 ID 来追溯根因链条。" />
      ) : chain.links.length === 0 ? (
        <EmptyState text="没有发现因果链条。" />
      ) : (
        <div className="space-y-2 max-h-[480px] overflow-y-auto">
          <div className="rounded-xl px-3 py-2 text-xs bg-surface-secondary border border-border">
            <div className="mb-1 text-muted">
              根因 → 最终效应
            </div>
            <div className="font-mono break-all text-foreground">
              {chain.root_cause}
            </div>
            <div className="font-mono break-all mt-1 text-muted">
              ↓
            </div>
            <div className="font-mono break-all text-foreground">
              {chain.final_effect}
            </div>
          </div>
          {chain.links.map((link, idx) => (
            <div
              key={`${link.cause_event_id}-${link.effect_event_id}-${idx}`}
              className="rounded-xl px-3 py-2 text-sm bg-surface-secondary border border-border"
            >
              <div className="flex items-center gap-2 mb-1 flex-wrap">
                <Chip size="sm">强度 {(link.strength * 100).toFixed(0)}%</Chip>
                <span className="text-xs font-mono text-muted">
                  {link.cause_kind} → {link.effect_kind}
                </span>
              </div>
              <div className="text-xs text-foreground">
                {link.mechanism}
              </div>
            </div>
          ))}
          <div className="flex flex-wrap gap-2 pt-1">
            <Link href={chatPromptHref(fixRootCausePrompt(taskId, chain))}>
              <Button size="sm" variant="ghost">
                <Send size={13} /> 生成修复方案
              </Button>
            </Link>
            <Link href={taskDetailHref(taskId)}>
              <Button size="sm" variant="ghost">
                <ClipboardList size={13} /> 查看任务
              </Button>
            </Link>
            <Link href={traceTaskHref(taskId)}>
              <Button size="sm" variant="ghost">
                <Activity size={13} /> 查看轨迹
              </Button>
            </Link>
          </div>
        </div>
      )}
    </Card>
  );
}

export default function WorldModelPackPage() {
  const [stateEntries, setStateEntries] = useState<WorldStateEntry[]>([]);
  const [staleKeys, setStaleKeys] = useState<Set<string>>(new Set());
  const [patterns, setPatterns] = useState<FailurePattern[]>([]);
  const [taskId, setTaskId] = useState("");
  const [chain, setChain] = useState<CausalChain | null>(null);
  const [loading, setLoading] = useState(true);
  const [chainLoading, setChainLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const refresh = useCallback(async () => {
    try {
      setLoading(true);
      setError(null);
      const [s, stale, fp] = await Promise.all([
        worldClient.state(),
        worldClient.stale("24h"),
        worldClient.failurePatterns(50),
      ]);
      setStateEntries(s.entries);
      setStaleKeys(new Set(stale.keys));
      setPatterns(fp.patterns);
    } catch (e: unknown) {
      setError(formatErrorMessage(e, "加载世界模型数据失败"));
    } finally {
      setLoading(false);
    }
  }, []);

  const lookupRootCause = useCallback(async () => {
    if (!taskId) return;
    try {
      setChainLoading(true);
      setError(null);
      const r = await worldClient.rootCause(taskId);
      setChain(r.chain ?? null);
    } catch (e: unknown) {
      setError(formatErrorMessage(e, "查询根因失败"));
    } finally {
      setChainLoading(false);
    }
  }, [taskId]);

  useEffect(() => {
    refresh();
  }, [refresh]);

  return (
    <div className="page-root space-y-6 animate-fade-in-up">
      <PageHeader icon={<Globe size={20} />} title="世界模型" />

      <PackAbout
        chips={<>
          <Chip size="sm" color="success">可直接使用</Chip>
          <Chip size="sm" variant="soft">看事实</Chip>
          <Chip size="sm" variant="soft">查根因</Chip>
        </>}
        description="它用于把云雀对外部环境的理解变成可查看、可质疑、可追溯的事实和因果线索。当前可以查看世界状态、识别过期事实、发现跨任务失败模式，并把根因链交给 Chat 生成修复方案。"
        boundaries={boundaryItems}
      />

      <Card variant="default">
        <Card.Content className="flex flex-col gap-4">
          <PackStepsGrid steps={userFacingSteps} columns={3} />
          <div className="text-sm leading-6 text-muted">
            Agent 维护对外部世界（文件、数据库、API、配置）的认知，并通过因果引擎分析失败任务的根因与跨任务模式。
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
        <StateColumn entries={stateEntries} staleKeys={staleKeys} loading={loading} />
        <FailurePatternsColumn items={patterns} loading={loading} />
        <RootCauseColumn
          taskId={taskId}
          setTaskId={setTaskId}
          chain={chain}
          loading={chainLoading}
          onLookup={lookupRootCause}
        />
      </div>

      <Card className="section-card p-4 text-xs flex items-center gap-2 text-muted">
        <Clock size={14} />
        过期阈值：24h（超过此时间未核实的世界状态会被标记）
      </Card>
    </div>
  );
}
