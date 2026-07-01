"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import Link from "next/link";
import { Card, Spinner, Chip, Input, Button } from "@heroui/react";
import { Activity, Bot, ClipboardList, History, Search, Send, AlertTriangle } from "lucide-react";
import PageHeader from "@/components/page-header";
import { formatErrorMessage } from "@/lib/error-utils";
import { chatPromptHref, taskDetailHref, traceTaskHref } from "@/lib/pack-action-links";
import { microAgentTaskPrompt, microAgentTraceSummaryPrompt, previewTracePayload } from "@/lib/micro-agent-pack-actions";
import { PackAbout, PackStepsGrid, type PackBoundaryItem, type PackStep } from "@/components/packs/pack-page-kit";
import {
  createMicroAgentPackClient,
  type AgentEntry,
  type TraceEntry,
} from "@/lib/micro-agent-pack-client";

const microClient = createMicroAgentPackClient();

const userFacingSteps: PackStep[] = [
  { key: "view", label: "查看已注册微代理", detail: "确认哪些领域提示会常驻或按触发词注入到任务里。" },
  { key: "trigger", label: "试一条触发消息", detail: "输入消息，预览会激活哪些微代理，再决定是否用它开始任务。" },
  { key: "replay", label: "回放任务推理", detail: "输入 task_id 查看 ReAct 轨迹，并把轨迹交给 Chat 总结。" },
];

const boundaryItems: PackBoundaryItem[] = [
  { key: "exec", label: "不自动执行", detail: "不会自动执行微代理内容。" },
  { key: "confirm", label: "不绕过确认", detail: "不会绕过 Chat 或任务确认直接开工。" },
  { key: "disabled", label: "不注入禁用项", detail: "不会把禁用的微代理注入上下文。" },
  { key: "source", label: "不改源文件", detail: "不会修改 data/microagents 下的源文件。" },
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

function AgentCard({ agent, highlighted }: { agent: AgentEntry; highlighted?: boolean }) {
  const scopeColor: "accent" | "success" | "default" =
    agent.scope === "global" ? "accent" : agent.scope === "repo" ? "success" : "default";
  return (
    <div className={`rounded-xl px-3 py-2 text-sm bg-surface-secondary border ${highlighted ? "border-accent" : "border-border"}`}>
      <div className="flex items-center gap-2 mb-1 flex-wrap">
        <Chip size="sm" color={scopeColor}>
          {agent.scope}
        </Chip>
        <span className="font-semibold text-foreground">
          {agent.name}
        </span>
        {!agent.enabled ? (
          <Chip size="sm" color="danger">
            禁用
          </Chip>
        ) : null}
        {agent.trigger ? (
          <Chip size="sm">触发词: {agent.trigger}</Chip>
        ) : (
          <Chip size="sm">常驻</Chip>
        )}
        <span className="text-xs ml-auto text-muted">
          优先级 {agent.priority}
        </span>
      </div>
      {agent.description ? (
        <div className="text-xs mb-1 text-muted">
          {agent.description}
        </div>
      ) : null}
      <details>
        <summary className="text-xs cursor-pointer text-muted">
          查看内容（{agent.content.length} 字）
        </summary>
        <pre className="text-xs mt-1 whitespace-pre-wrap break-words text-foreground">
          {agent.content}
        </pre>
      </details>
      <div className="mt-2">
        <Link href={chatPromptHref(microAgentTaskPrompt(agent))}>
          <Button size="sm" variant="ghost">
            <Send size={13} /> 用它开始任务
          </Button>
        </Link>
      </div>
    </div>
  );
}

function AgentsColumn({
  agents,
  matched,
  loading,
}: {
  agents: AgentEntry[];
  matched: Set<string>;
  loading: boolean;
}) {
  return (
    <Card className="section-card p-4">
      <ColumnHeader icon={<Bot size={16} />} title="微代理" hint={`${agents.length} 个已注册`} />
      {loading ? (
        <div className="flex items-center gap-2 text-xs text-muted">
          <Spinner size="sm" /> 加载中…
        </div>
      ) : agents.length === 0 ? (
        <EmptyState text="还没有加载任何微代理。可以在 data/microagents/ 目录下添加 .md 文件。" />
      ) : (
        <div className="space-y-2 max-h-[640px] overflow-y-auto">
          {agents.map((a) => (
            <AgentCard key={a.name} agent={a} highlighted={matched.has(a.name)} />
          ))}
        </div>
      )}
    </Card>
  );
}

function ResolveColumn({
  message,
  setMessage,
  matched,
  onResolve,
  loading,
}: {
  message: string;
  setMessage: (v: string) => void;
  matched: AgentEntry[];
  onResolve: () => void;
  loading: boolean;
}) {
  return (
    <Card className="section-card p-4">
      <ColumnHeader icon={<Search size={16} />} title="试触发" hint="模拟一条消息" />
      <div className="flex items-center gap-2 mb-3">
        <Input
          placeholder="输入一段消息，看会激活哪些微代理"
          value={message}
          onChange={(e) => setMessage(e.target.value)}
          className="flex-1"
        />
        <Button size="sm" onPress={onResolve} isDisabled={!message || loading}>
          解析
        </Button>
      </div>
      {loading ? (
        <div className="flex items-center gap-2 text-xs text-muted">
          <Spinner size="sm" /> 加载中…
        </div>
      ) : matched.length === 0 ? (
        <EmptyState text="未激活任何微代理。试试包含触发词的消息。" />
      ) : (
        <div className="space-y-2 max-h-[560px] overflow-y-auto">
          {matched.map((a) => (
            <AgentCard key={a.name} agent={a} highlighted />
          ))}
        </div>
      )}
    </Card>
  );
}

function TraceColumn({
  taskId,
  setTaskId,
  entries,
  onLookup,
  loading,
}: {
  taskId: string;
  setTaskId: (v: string) => void;
  entries: TraceEntry[];
  onLookup: () => void;
  loading: boolean;
}) {
  return (
    <Card className="section-card p-4">
      <ColumnHeader icon={<History size={16} />} title="ReAct 回放" hint="任务推理轨迹" />
      <div className="flex items-center gap-2 mb-3">
        <Input
          placeholder="输入 task_id"
          value={taskId}
          onChange={(e) => setTaskId(e.target.value)}
          className="flex-1"
        />
        <Button size="sm" onPress={onLookup} isDisabled={!taskId || loading}>
          回放
        </Button>
      </div>
      {loading ? (
        <div className="flex items-center gap-2 text-xs text-muted">
          <Spinner size="sm" /> 加载中…
        </div>
      ) : entries.length === 0 ? (
        <EmptyState text="输入一个任务 ID 来查看其 ReAct 推理轨迹。" />
      ) : (
        <div className="space-y-1 max-h-[560px] overflow-y-auto">
          <div className="mb-2 flex flex-wrap gap-2">
            <Link href={chatPromptHref(microAgentTraceSummaryPrompt(taskId, entries))}>
              <Button size="sm" variant="ghost">
                <Send size={13} /> 总结这段推理
              </Button>
            </Link>
            <Link href={taskDetailHref(taskId)}>
              <Button size="sm" variant="ghost">
                <ClipboardList size={13} /> 查看任务
              </Button>
            </Link>
            <Link href={traceTaskHref(taskId)}>
              <Button size="sm" variant="ghost">
                <Activity size={13} /> 查看执行轨迹
              </Button>
            </Link>
          </div>
          {entries.map((e) => {
            const kind = e.kind.replace(/^reasoning\./, "");
            const color: "accent" | "success" | "danger" | "warning" | "default" =
              kind === "thought"
                ? "accent"
                : kind === "decision"
                  ? "success"
                  : kind === "observe"
                    ? "default"
                    : kind === "backtrack"
                      ? "danger"
                      : kind === "reflect"
                        ? "warning"
                        : "accent";
            const summary = previewTracePayload(e.payload);
            return (
              <div
                key={e.id}
                className="rounded-xl px-3 py-2 text-sm bg-surface-secondary border border-border"
              >
                <div className="flex items-center gap-2 mb-1 flex-wrap">
                  <Chip size="sm" color={color}>
                    {kind}
                  </Chip>
                  <span className="text-xs text-muted">
                    {e.actor}
                  </span>
                  <span className="text-xs ml-auto text-muted">
                    {formatTimestamp(e.created_at)}
                  </span>
                </div>
                {summary ? (
                  <div className="text-xs text-foreground">
                    {summary}
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

export default function MicroAgentPackPage() {
  const [agents, setAgents] = useState<AgentEntry[]>([]);
  const [message, setMessage] = useState("");
  const [matched, setMatched] = useState<AgentEntry[]>([]);
  const [taskId, setTaskId] = useState("");
  const [trace, setTrace] = useState<TraceEntry[]>([]);
  const [loadingAgents, setLoadingAgents] = useState(true);
  const [resolveLoading, setResolveLoading] = useState(false);
  const [traceLoading, setTraceLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const matchedSet = useMemo(() => new Set(matched.map((m) => m.name)), [matched]);

  const refreshAgents = useCallback(async () => {
    try {
      setLoadingAgents(true);
      setError(null);
      const r = await microClient.agents();
      setAgents(r.agents);
    } catch (e: unknown) {
      setError(formatErrorMessage(e, "加载微代理列表失败"));
    } finally {
      setLoadingAgents(false);
    }
  }, []);

  const resolve = useCallback(async () => {
    if (!message) return;
    try {
      setResolveLoading(true);
      setError(null);
      const r = await microClient.resolve(message);
      setMatched(r.matched);
    } catch (e: unknown) {
      setError(formatErrorMessage(e, "解析微代理触发失败"));
    } finally {
      setResolveLoading(false);
    }
  }, [message]);

  const lookupTrace = useCallback(async () => {
    if (!taskId) return;
    try {
      setTraceLoading(true);
      setError(null);
      const r = await microClient.trace(taskId, 200);
      setTrace(r.entries);
    } catch (e: unknown) {
      setError(formatErrorMessage(e, "查询推理轨迹失败"));
    } finally {
      setTraceLoading(false);
    }
  }, [taskId]);

  useEffect(() => {
    refreshAgents();
  }, [refreshAgents]);

  return (
    <div className="page-root space-y-6 animate-fade-in-up">
      <PageHeader icon={<Bot size={20} />} title="微代理" />

      <PackAbout
        chips={<>
          <Chip size="sm" color="success">可直接使用</Chip>
          <Chip size="sm" variant="soft">可试触发</Chip>
          <Chip size="sm" variant="soft">可回放轨迹</Chip>
        </>}
        description="它用于查看哪些轻量专家提示会参与任务，并在真正发起前预览触发结果。当前可以查看微代理内容、模拟消息触发、用匹配到的微代理开始任务，也可以按 task_id 回放推理轨迹。"
        boundaries={boundaryItems}
      />

      <Card variant="default">
        <Card.Content className="flex flex-col gap-4">
          <PackStepsGrid steps={userFacingSteps} columns={3} />
          <div className="text-sm leading-6 text-muted">
            微代理是按需注入的领域提示。Agent 收到消息时会按触发词激活相应的微代理；ReAct 引擎记录每一步的推理轨迹，支持事后回放。
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
        <AgentsColumn agents={agents} matched={matchedSet} loading={loadingAgents} />
        <ResolveColumn
          message={message}
          setMessage={setMessage}
          matched={matched}
          onResolve={resolve}
          loading={resolveLoading}
        />
        <TraceColumn
          taskId={taskId}
          setTaskId={setTaskId}
          entries={trace}
          onLookup={lookupTrace}
          loading={traceLoading}
        />
      </div>
    </div>
  );
}
