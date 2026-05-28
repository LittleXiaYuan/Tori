"use client";

import { useCallback, useEffect, useState } from "react";
import { Card, Spinner, Chip, Input, Button } from "@heroui/react";
import { Globe, Clock, AlertOctagon, GitBranch, AlertTriangle } from "lucide-react";
import PageHeader from "@/components/page-header";
import { formatErrorMessage } from "@/lib/error-utils";
import {
  createWorldModelPackClient,
  type WorldStateEntry,
  type FailurePattern,
  type CausalChain,
} from "@/lib/world-model-pack-client";

const worldClient = createWorldModelPackClient();

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
        <div className="flex items-center gap-2 text-xs" style={{ color: "var(--yunque-text-muted)" }}>
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
                className="rounded-md px-3 py-2 text-sm"
                style={{ background: "rgba(255,255,255,0.02)", border: "1px solid var(--yunque-border)" }}
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
                  <span className="text-xs ml-auto" style={{ color: "var(--yunque-text-muted)" }}>
                    {formatTimestamp(s.last_verified)}
                  </span>
                </div>
                <div className="font-mono text-xs" style={{ color: "var(--yunque-text)" }}>
                  {s.key}
                </div>
                <div
                  className="text-xs mt-1 break-words line-clamp-3"
                  style={{ color: "var(--yunque-text-muted)" }}
                >
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
        <div className="flex items-center gap-2 text-xs" style={{ color: "var(--yunque-text-muted)" }}>
          <Spinner size="sm" /> 加载中…
        </div>
      ) : items.length === 0 ? (
        <EmptyState text="还没有发现失败模式。任务失败累积后会自动聚合。" />
      ) : (
        <div className="space-y-2 max-h-[600px] overflow-y-auto">
          {items.map((p, idx) => (
            <div
              key={`${p.cause_kind}-${p.effect_kind}-${idx}`}
              className="rounded-md px-3 py-2 text-sm"
              style={{ background: "rgba(255,255,255,0.02)", border: "1px solid var(--yunque-border)" }}
            >
              <div className="flex items-center gap-2 mb-1 flex-wrap">
                <Chip size="sm" color="danger">
                  ×{p.occurrences}
                </Chip>
                <span className="text-xs font-mono" style={{ color: "var(--yunque-text-muted)" }}>
                  {p.cause_kind} → {p.effect_kind}
                </span>
              </div>
              <div className="text-xs" style={{ color: "var(--yunque-text)" }}>
                {p.mechanism}
              </div>
              {p.task_ids && p.task_ids.length > 0 ? (
                <div className="text-xs mt-1 font-mono" style={{ color: "var(--yunque-text-muted)" }}>
                  涉及任务: {p.task_ids.slice(0, 3).join(", ")}
                  {p.task_ids.length > 3 ? ` (+${p.task_ids.length - 3})` : ""}
                </div>
              ) : null}
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
        <div className="flex items-center gap-2 text-xs" style={{ color: "var(--yunque-text-muted)" }}>
          <Spinner size="sm" /> 加载中…
        </div>
      ) : !chain ? (
        <EmptyState text="输入一个失败任务的 ID 来追溯根因链条。" />
      ) : chain.links.length === 0 ? (
        <EmptyState text="没有发现因果链条。" />
      ) : (
        <div className="space-y-2 max-h-[480px] overflow-y-auto">
          <div
            className="rounded-md px-3 py-2 text-xs"
            style={{ background: "rgba(255,255,255,0.02)", border: "1px solid var(--yunque-border)" }}
          >
            <div className="mb-1" style={{ color: "var(--yunque-text-muted)" }}>
              根因 → 最终效应
            </div>
            <div className="font-mono break-all" style={{ color: "var(--yunque-text)" }}>
              {chain.root_cause}
            </div>
            <div className="font-mono break-all mt-1" style={{ color: "var(--yunque-text-muted)" }}>
              ↓
            </div>
            <div className="font-mono break-all" style={{ color: "var(--yunque-text)" }}>
              {chain.final_effect}
            </div>
          </div>
          {chain.links.map((link, idx) => (
            <div
              key={`${link.cause_event_id}-${link.effect_event_id}-${idx}`}
              className="rounded-md px-3 py-2 text-sm"
              style={{ background: "rgba(255,255,255,0.02)", border: "1px solid var(--yunque-border)" }}
            >
              <div className="flex items-center gap-2 mb-1 flex-wrap">
                <Chip size="sm">强度 {(link.strength * 100).toFixed(0)}%</Chip>
                <span className="text-xs font-mono" style={{ color: "var(--yunque-text-muted)" }}>
                  {link.cause_kind} → {link.effect_kind}
                </span>
              </div>
              <div className="text-xs" style={{ color: "var(--yunque-text)" }}>
                {link.mechanism}
              </div>
            </div>
          ))}
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

      <Card className="section-card p-4 text-sm" style={{ color: "var(--yunque-text-muted)" }}>
        Agent 维护对外部世界（文件、数据库、API、配置）的认知，并通过因果引擎分析失败任务的根因与跨任务模式。
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

      <Card className="section-card p-4 text-xs flex items-center gap-2" style={{ color: "var(--yunque-text-muted)" }}>
        <Clock size={14} />
        过期阈值：24h（超过此时间未核实的世界状态会被标记）
      </Card>
    </div>
  );
}
