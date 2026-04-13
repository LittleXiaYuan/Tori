"use client";

import { Suspense, useEffect, useState, useCallback, useRef } from "react";
import { useRouter, useSearchParams } from "next/navigation";
import { Card, Button, Spinner, Chip, Tooltip, Tabs, ProgressBar, Table, Input } from "@heroui/react";
import EmptyState from "@/components/empty-state";
import { api, type TaskInfo, type TaskWorkingMemory, type CostTaskSummary, type CostUsageEvent } from "@/lib/api";
import {
  ArrowLeft, CheckCircle2, Clock, XCircle, RefreshCw, Play, Pause, PlayCircle,
  Lock, Send, MessageSquare, FileText, DollarSign, GitBranch, Info,
  AlertTriangle, Trash2, Zap, Loader2,
} from "lucide-react";
import { showToast } from "@/components/toast-provider";
import { STATUS_COLORS, fmtTime, dur } from "@/lib/constants";
import { usePolling } from "@/lib/use-polling";

const stepStatusIcon: Record<string, React.ReactNode> = {
  pending: <Clock size={14} className="text-gray-400" />,
  running: <Loader2 size={14} className="text-blue-400 animate-spin" />,
  retrying: <RefreshCw size={14} className="text-amber-400 animate-spin" />,
  done: <CheckCircle2 size={14} className="text-green-400" />,
  failed: <XCircle size={14} className="text-red-400" />,
  skipped: <Clock size={14} className="text-gray-300" />,
};

/* ── Overview Panel ── */
function OverviewPanel({ task, wm }: { task: TaskInfo; wm: TaskWorkingMemory | null }) {
  const steps = task.steps || [];
  const doneSteps = steps.filter((s) => s.status === "done").length;

  return (
    <div className="space-y-4">
      <Card className="section-card p-5">
        <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
          <div>
            <div className="text-xs mb-1" style={{ color: "var(--yunque-text-muted)" }}>状态</div>
            <Chip size="sm" style={{ background: `${STATUS_COLORS[task.status] || "#9ca3af"}20`, color: STATUS_COLORS[task.status] || "#9ca3af" }}>
              {task.status}
            </Chip>
          </div>
          <div>
            <div className="text-xs mb-1" style={{ color: "var(--yunque-text-muted)" }}>步骤进度</div>
            <div className="text-sm font-medium" style={{ color: "var(--yunque-text)" }}>{doneSteps}/{steps.length}</div>
          </div>
          <div>
            <div className="text-xs mb-1" style={{ color: "var(--yunque-text-muted)" }}>创建时间</div>
            <div className="text-sm" style={{ color: "var(--yunque-text)" }}>{fmtTime(task.created_at)}</div>
          </div>
          <div>
            <div className="text-xs mb-1" style={{ color: "var(--yunque-text-muted)" }}>耗时</div>
            <div className="text-sm" style={{ color: "var(--yunque-text)" }}>{dur(task.started_at, task.finished_at)}</div>
          </div>
        </div>
        {steps.length > 0 && (
          <div className="mt-4">
            <ProgressBar value={doneSteps} maxValue={steps.length} aria-label="步骤进度" className="h-2">
              <ProgressBar.Track>
                <ProgressBar.Fill />
              </ProgressBar.Track>
            </ProgressBar>
          </div>
        )}
      </Card>

      <Card className="section-card p-4">
        <div className="text-xs font-medium mb-2" style={{ color: "var(--yunque-text-muted)" }}>任务描述</div>
        <div className="text-sm whitespace-pre-wrap" style={{ color: "var(--yunque-text)" }}>{task.description || "无"}</div>
      </Card>

      {task.error && (
        <Card className="p-4 border-red-500/30" style={{ background: "#ef444410" }}>
          <div className="flex items-center gap-2 mb-2">
            <AlertTriangle size={14} className="text-red-400" />
            <span className="text-sm font-medium text-red-400">错误</span>
          </div>
          <div className="text-sm text-red-300">{task.error}</div>
        </Card>
      )}

      {wm && wm.Goal && (
        <Card className="section-card p-4">
          <div className="flex items-center gap-2 mb-3">
            <Zap size={14} style={{ color: "var(--yunque-accent)" }} />
            <span className="text-sm font-medium" style={{ color: "var(--yunque-text)" }}>工作记忆</span>
            <span className="text-xs" style={{ color: "var(--yunque-text-muted)" }}>~{wm.TokenEstimate} tokens</span>
          </div>
          <div className="space-y-2 text-sm" style={{ color: "var(--yunque-text)" }}>
            <div><span className="font-medium">目标:</span>{wm.Goal}</div>
            {wm.CompletedWork?.length > 0 && (
              <div>
                <span className="font-medium">已完成：</span>
                <ul className="list-disc list-inside mt-1">
                  {wm.CompletedWork.map((w: string, i: number) => <li key={i} className="text-xs" style={{ color: "var(--yunque-text-muted)" }}>{w}</li>)}
                </ul>
              </div>
            )}
            {wm.Blockers?.length > 0 && (
              <div>
                <span className="font-medium text-red-400">阻塞项:</span>
                <ul className="list-disc list-inside mt-1">
                  {wm.Blockers.map((b: string, i: number) => <li key={i} className="text-xs text-red-300">{b}</li>)}
                </ul>
              </div>
            )}
            {wm.NextAction && <div><span className="font-medium">下一步：</span>{wm.NextAction}</div>}
          </div>
        </Card>
      )}
    </div>
  );
}

/* ── Execution Panel ── */
function ExecutionPanel({ task }: { task: TaskInfo }) {
  const steps = task.steps || [];
  if (steps.length === 0) {
    return <EmptyState icon={<GitBranch size={24} style={{ color: "var(--yunque-accent)" }} />} title="暂无执行步骤" description="任务开始运行后将在此显示执行流" />;
  }
  return (
    <div className="relative">
      <div className="absolute left-[19px] top-4 bottom-4 w-0.5" style={{ background: "var(--yunque-border)" }} />
      <div className="space-y-1">
        {steps.map((step) => (
          <div key={step.id} className="relative flex gap-3">
            <div className="flex-shrink-0 w-10 flex items-start justify-center pt-4 z-10">
              <div className="w-6 h-6 rounded-full flex items-center justify-center"
                style={{
                  background: step.status === "done" ? "#22c55e20" : step.status === "failed" ? "#ef444420" : step.status === "running" ? "#3b82f620" : "var(--yunque-card)",
                  border: "2px solid var(--yunque-border)",
                }}>
                {stepStatusIcon[step.status] || stepStatusIcon.pending}
              </div>
            </div>
            <Card className="section-card flex-1 p-4 mb-1">
              <div className="flex items-center justify-between mb-1">
                <div className="flex items-center gap-2">
                  <span className="text-sm font-medium" style={{ color: "var(--yunque-text)" }}>Step {step.id}</span>
                  <Chip size="sm" style={{ background: `${STATUS_COLORS[step.status === "done" ? "completed" : step.status] || "#9ca3af"}20`, color: STATUS_COLORS[step.status === "done" ? "completed" : step.status] || "#9ca3af" }}>
                    {step.status}
                  </Chip>
                  {step.skill_name && <Chip size="sm" variant="soft" style={{ background: "rgba(0,111,238,0.15)", color: "var(--yunque-accent)" }}>{step.skill_name}</Chip>}
                </div>
                <div className="flex items-center gap-2 text-xs" style={{ color: "var(--yunque-text-muted)" }}>
                  {step.retry_count ? <span className="text-amber-400">retry ×{step.retry_count}</span> : null}
                  <span>{dur(step.started_at, step.done_at)}</span>
                </div>
              </div>
              <div className="text-sm" style={{ color: "var(--yunque-text-muted)" }}>{step.action}</div>
              {step.result && (
                <div className="mt-2 text-xs p-3 rounded-lg whitespace-pre-wrap break-words max-h-40 overflow-y-auto"
                  style={{ background: "var(--yunque-bg)", color: "var(--yunque-text-muted)" }}>
                  {step.result.length > 800 ? step.result.slice(0, 800) + "…" : step.result}
                </div>
              )}
              {step.error && (
                <div className="mt-2 text-xs p-3 rounded-lg text-red-400" style={{ background: "#ef444410" }}>{step.error}</div>
              )}
            </Card>
          </div>
        ))}
      </div>
    </div>
  );
}

/* ── Thread Tab ── */
function ThreadTab({ taskId }: { taskId: string }) {
  const [messages, setMessages] = useState<Array<{ role: string; content: string }>>([]);
  const [info, setInfo] = useState<{ state: string; messages: number } | null>(null);
  const [input, setInput] = useState("");
  const [sending, setSending] = useState(false);
  const [loading, setLoading] = useState(true);
  const endRef = useRef<HTMLDivElement>(null);

  const load = useCallback(async () => {
    try {
      const data = await api.getTaskThread(taskId);
      setInfo(data.info);
      setMessages(data.messages || []);
    } catch { /* ignore */ } finally { setLoading(false); }
  }, [taskId]);

  useEffect(() => { load(); }, [load]);
  usePolling(load, 3000);
  useEffect(() => { endRef.current?.scrollIntoView({ behavior: "smooth" }); }, [messages]);

  const send = async () => {
    const text = input.trim();
    if (!text || sending) return;
    setSending(true); setInput("");
    try { await api.postTaskThread(taskId, text); await load(); } catch (e) { showToast(e instanceof Error ? e.message : "发送失败", "error"); } finally { setSending(false); }
  };

  const changeState = async (s: "open" | "paused" | "closed") => {
    try { await api.updateThreadState(taskId, s); await load(); } catch (e) { showToast(e instanceof Error ? e.message : "操作失败", "error"); }
  };

  const isClosed = info?.state === "closed";
  const isPaused = info?.state === "paused";

  return (
    <Card className="flex flex-col" style={{ background: "var(--yunque-card)", borderColor: "var(--yunque-border)", height: "500px" }}>
      <div className="flex items-center gap-2 px-4 py-3 border-b flex-shrink-0" style={{ borderColor: "var(--yunque-border)" }}>
        <MessageSquare size={14} style={{ color: "var(--yunque-accent)" }} />
        <span className="text-sm font-medium flex-1" style={{ color: "var(--yunque-text)" }}>任务线程</span>
        {info && (
          <>
            <Chip size="sm" style={{ background: `${info.state === "open" ? "#22c55e" : info.state === "paused" ? "#f59e0b" : "#9ca3af"}20`, color: info.state === "open" ? "#22c55e" : info.state === "paused" ? "#f59e0b" : "#9ca3af" }}>
              {info.state === "open" ? "活跃" : info.state === "paused" ? "已暂停" : "已关闭"}
            </Chip>
            <span className="text-xs" style={{ color: "var(--yunque-text-muted)" }}>{info.messages} 条</span>
          </>
        )}
        <div className="flex gap-1">
          {!isClosed && !isPaused && <Button size="sm" variant="ghost" isIconOnly aria-label="暂停" onPress={() => changeState("paused")}><Pause size={12} className="text-amber-400" /></Button>}
          {isPaused && <Button size="sm" variant="ghost" isIconOnly aria-label="继续" onPress={() => changeState("open")}><PlayCircle size={12} className="text-green-400" /></Button>}
          {!isClosed && <Button size="sm" variant="ghost" isIconOnly aria-label="关闭" onPress={() => changeState("closed")}><Lock size={12} /></Button>}
        </div>
      </div>

      <div className="flex-1 overflow-y-auto px-4 py-3 space-y-2">
        {loading ? (
          <div className="flex items-center justify-center h-full"><Spinner size="sm" /></div>
        ) : messages.length === 0 ? (
          <div className="flex flex-col items-center justify-center h-full">
            <EmptyState icon={<MessageSquare size={24} style={{ color: "var(--yunque-accent)" }} />} title="暂无消息" description="在下方输入框向 Agent 发送第一条消息" />
          </div>
        ) : (
          messages.map((msg, i) => {
            const isUser = msg.role === "user";
            const isSystem = msg.role === "system";
            return (
              <div key={i} className="flex" style={{ justifyContent: isUser ? "flex-end" : isSystem ? "center" : "flex-start" }}>
                <div className={`max-w-[75%] px-3 py-2 rounded-xl text-sm whitespace-pre-wrap break-words ${isSystem ? "w-full text-center" : ""}`}
                  style={{ background: isSystem ? "transparent" : isUser ? "var(--yunque-accent)" : "#374151", color: isSystem ? "var(--yunque-text-muted)" : "#fff", fontSize: isSystem ? "12px" : "14px", fontStyle: isSystem ? "italic" : "normal" }}>
                  {!isSystem && <div className="text-xs opacity-60 mb-0.5">{isUser ? "[U]" : "[A]"} {msg.role}</div>}
                  {msg.content}
                </div>
              </div>
            );
          })
        )}
        <div ref={endRef} />
      </div>

      <div className="px-4 py-3 border-t flex-shrink-0" style={{ borderColor: "var(--yunque-border)" }}>
        {isClosed ? (
          <div className="text-center text-sm py-1" style={{ color: "var(--yunque-text-muted)" }}><Lock size={14} className="inline mr-1" />线程已关闭</div>
        ) : (
          <div className="flex gap-2">
            <Input className="flex-1"
              placeholder={isPaused ? "线程已暂停" : "输入消息…"} disabled={isPaused}
              value={input} onChange={(e) => setInput(e.target.value)}
              onKeyDown={(e) => e.key === "Enter" && !e.shiftKey && send()} />
            <Button size="sm" isIconOnly aria-label="发送" isDisabled={!input.trim() || sending || isPaused} onPress={send}
              className="btn-accent">
              {sending ? <Spinner size="sm" /> : <Send size={14} />}
            </Button>
          </div>
        )}
      </div>
    </Card>
  );
}

/* ── Artifacts Panel ── */
function ArtifactsPanel({ task }: { task: TaskInfo }) {
  const artifacts = task.artifacts || [];
  if (artifacts.length === 0) {
    return <EmptyState icon={<FileText size={24} style={{ color: "var(--yunque-accent)" }} />} title="暂无产物" description="任务执行产生的文件将在此展示" />;
  }
  return (
    <div className="space-y-2">
      {artifacts.map((a, i) => (
        <Card key={i} className="section-card flex items-center gap-3 p-4">
          <FileText size={18} style={{ color: "var(--yunque-accent)" }} />
          <div className="flex-1 min-w-0">
            <div className="text-sm font-medium truncate" style={{ color: "var(--yunque-text)" }}>{a.path}</div>
            <div className="text-xs" style={{ color: "var(--yunque-text-muted)" }}>{a.type}</div>
          </div>
        </Card>
      ))}
    </div>
  );
}

/* ── Cost Panel ── */
function CostPanel({ cost, timeline }: { cost: CostTaskSummary | null; timeline: CostUsageEvent[] }) {
  if (!cost) {
    return <EmptyState icon={<DollarSign size={24} style={{ color: "var(--yunque-accent)" }} />} title="暂无成本数据" description="任务调用 LLM 后将在此显示费用明细" />;
  }
  const bySkill = cost.by_skill ? Object.entries(cost.by_skill).sort((a, b) => b[1] - a[1]) : [];
  const byModel = cost.by_model ? Object.entries(cost.by_model).sort((a, b) => b[1] - a[1]) : [];
  const totalCost = cost.total_cost_usd || 0;

  return (
    <div className="space-y-4">
      <Card className="section-card p-5">
        <div className="grid grid-cols-2 md:grid-cols-5 gap-4">
          <div>
            <div className="text-xs mb-1" style={{ color: "var(--yunque-text-muted)" }}>总费用</div>
            <div className="kpi-value" style={{ color: "var(--yunque-accent)" }}>${totalCost.toFixed(4)}</div>
          </div>
          <div>
            <div className="text-xs mb-1" style={{ color: "var(--yunque-text-muted)" }}>输入 Tokens</div>
            <div className="text-lg font-semibold" style={{ color: "var(--yunque-text)" }}>{(cost.total_tokens_in || 0).toLocaleString()}</div>
          </div>
          <div>
            <div className="text-xs mb-1" style={{ color: "var(--yunque-text-muted)" }}>输出 Tokens</div>
            <div className="text-lg font-semibold" style={{ color: "var(--yunque-text)" }}>{(cost.total_tokens_out || 0).toLocaleString()}</div>
          </div>
          <div>
            <div className="text-xs mb-1" style={{ color: "var(--yunque-text-muted)" }}>调用次数</div>
            <div className="text-lg font-semibold" style={{ color: "var(--yunque-text)" }}>{cost.calls || 0}</div>
          </div>
          <div>
            <div className="text-xs mb-1" style={{ color: "var(--yunque-text-muted)" }}>平均延迟</div>
            <div className="text-lg font-semibold" style={{ color: "var(--yunque-text)" }}>{cost.avg_latency_ms || 0}ms</div>
          </div>
        </div>
      </Card>

      <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
        {bySkill.length > 0 && (
          <Card className="section-card p-4">
            <div className="text-xs font-medium mb-3" style={{ color: "var(--yunque-text-muted)" }}>按技能分布</div>
            <div className="space-y-2">
              {bySkill.map(([skill, usd]) => (
                <div key={skill}>
                  <div className="flex justify-between items-center mb-1">
                    <span className="text-sm truncate" style={{ color: "var(--yunque-text)" }}>{skill}</span>
                    <span className="text-xs font-mono" style={{ color: "var(--yunque-text)" }}>${usd.toFixed(4)}</span>
                  </div>
                  <ProgressBar value={usd} maxValue={totalCost || 1} aria-label={skill} className="h-1.5">
                    <ProgressBar.Track>
                      <ProgressBar.Fill />
                    </ProgressBar.Track>
                  </ProgressBar>
                </div>
              ))}
            </div>
          </Card>
        )}
        {byModel.length > 0 && (
          <Card className="section-card p-4">
            <div className="text-xs font-medium mb-3" style={{ color: "var(--yunque-text-muted)" }}>按模型分布</div>
            <div className="space-y-2">
              {byModel.map(([model, usd]) => (
                <div key={model}>
                  <div className="flex justify-between items-center mb-1">
                    <span className="text-sm truncate" style={{ color: "var(--yunque-text)" }}>{model}</span>
                    <span className="text-xs font-mono" style={{ color: "var(--yunque-text)" }}>${usd.toFixed(4)}</span>
                  </div>
                  <ProgressBar value={usd} maxValue={totalCost || 1} aria-label={model} className="h-1.5">
                    <ProgressBar.Track>
                      <ProgressBar.Fill />
                    </ProgressBar.Track>
                  </ProgressBar>
                </div>
              ))}
            </div>
          </Card>
        )}
      </div>

      {timeline.length > 0 && (
        <Card className="section-card p-4">
          <div className="text-xs font-medium mb-3" style={{ color: "var(--yunque-text-muted)" }}>成本时间线</div>
          <Table>
            <Table.ScrollContainer>
              <Table.Content aria-label="成本时间线" className="min-w-[600px]">
                <Table.Header>
                  <Table.Column isRowHeader>步骤</Table.Column>
                  <Table.Column>技能</Table.Column>
                  <Table.Column>模型</Table.Column>
                  <Table.Column>Tokens</Table.Column>
                  <Table.Column>费用</Table.Column>
                  <Table.Column>延迟</Table.Column>
                </Table.Header>
                <Table.Body>
                  {timeline.map((evt, i) => (
                    <Table.Row key={i}>
                      <Table.Cell><span className="text-xs font-mono" style={{ color: "var(--yunque-text)" }}>{evt.step_id || "-"}</span></Table.Cell>
                      <Table.Cell><span className="text-xs" style={{ color: "var(--yunque-text)" }}>{evt.skill_name || "LLM"}</span></Table.Cell>
                      <Table.Cell><span className="text-xs" style={{ color: "var(--yunque-text-muted)" }}>{evt.model}</span></Table.Cell>
                      <Table.Cell><span className="text-xs font-mono" style={{ color: "var(--yunque-text)" }}>{(evt.tokens_in + evt.tokens_out).toLocaleString()}</span></Table.Cell>
                      <Table.Cell><span className="text-xs font-mono" style={{ color: "var(--yunque-accent)" }}>${evt.cost_usd.toFixed(4)}</span></Table.Cell>
                      <Table.Cell><span className="text-xs" style={{ color: "var(--yunque-text-muted)" }}>{Math.round(evt.latency / 1000000)}ms</span></Table.Cell>
                    </Table.Row>
                  ))}
                </Table.Body>
              </Table.Content>
            </Table.ScrollContainer>
          </Table>
        </Card>
      )}
    </div>
  );
}

/* ── Main Detail Content ── */
function TaskDetailContent() {
  const router = useRouter();
  const searchParams = useSearchParams();
  const taskId = searchParams.get("id") || "";

  const [task, setTask] = useState<TaskInfo | null>(null);
  const [wm, setWm] = useState<TaskWorkingMemory | null>(null);
  const [cost, setCost] = useState<CostTaskSummary | null>(null);
  const [costTimeline, setCostTimeline] = useState<CostUsageEvent[]>([]);
  const [activeTab, setActiveTab] = useState("overview");
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");

  const refresh = useCallback(async () => {
    if (!taskId) { setError("缺少任务 ID"); setLoading(false); return; }
    try {
      const t = await api.taskGet(taskId);
      setTask(t);
      const [wmData, costData, tlData] = await Promise.all([
        api.getTaskWorkingMemory(taskId).catch(() => null),
        api.getCostByTask(taskId).catch(() => null),
        api.getCostTaskTimeline(taskId).catch(() => []),
      ]);
      setWm(wmData);
      setCost(costData);
      setCostTimeline(tlData || []);
    } catch { setError("任务不存在或加载失败"); }
    finally { setLoading(false); }
  }, [taskId]);

  useEffect(() => { refresh(); }, [refresh]);
  usePolling(refresh, 5000);

  const handleRun = async () => { if (!task) return; try { await api.taskRun(task.id); setTimeout(refresh, 500); } catch (e) { showToast(e instanceof Error ? e.message : "运行失败", "error"); } };
  const handleCancel = async () => { if (!task) return; try { await api.taskCancel(task.id); setTimeout(refresh, 500); } catch (e) { showToast(e instanceof Error ? e.message : "取消失败", "error"); } };
  const handlePause = async () => { if (!task) return; try { await api.taskPause(task.id); setTimeout(refresh, 500); } catch (e) { showToast(e instanceof Error ? e.message : "暂停失败", "error"); } };
  const handleResume = async () => { if (!task) return; try { await api.taskResume(task.id); setTimeout(refresh, 500); } catch (e) { showToast(e instanceof Error ? e.message : "恢复失败", "error"); } };
  const handleRestart = async () => { if (!task) return; try { await api.taskRestart(task.id); setTimeout(refresh, 500); } catch (e) { showToast(e instanceof Error ? e.message : "重启失败", "error"); } };
  const handleDelete = async () => { if (!task) return; try { await api.taskDelete(task.id); router.push("/task-run"); } catch (e) { showToast(e instanceof Error ? e.message : "删除失败", "error"); } };

  if (loading) return <div className="flex items-center justify-center h-[60vh]"><Spinner size="lg" /></div>;

  if (error || !task) {
    return (
      <div className="text-center py-16" style={{ color: "var(--yunque-text-muted)" }}>
        <AlertTriangle size={48} className="mx-auto mb-3 opacity-30" />
        <p>{error || "任务未找到"}</p>
        <Button className="mt-4" onPress={() => router.push("/task-run")}>返回任务列表</Button>
      </div>
    );
  }

  return (
    <div className="page-root space-y-6 animate-fade-in-up">
      {/* Header */}
      <div className="flex items-center gap-3">
        <Button isIconOnly aria-label="返回" variant="ghost" size="sm" onPress={() => router.push("/task-run")}><ArrowLeft size={18} /></Button>
        <div className="flex-1 min-w-0">
          <div className="flex items-center gap-3">
            <h1 className="text-xl font-bold truncate" style={{ color: "var(--yunque-text)" }}>{task.title}</h1>
            <Chip size="sm" style={{ background: `${STATUS_COLORS[task.status] || "#9ca3af"}20`, color: STATUS_COLORS[task.status] || "#9ca3af" }}>
              {task.status}
            </Chip>
          </div>
          <div className="text-xs mt-0.5" style={{ color: "var(--yunque-text-muted)" }}>ID: {task.id} · {fmtTime(task.created_at)}</div>
        </div>
        <div className="flex items-center gap-2 flex-shrink-0">
          <Tooltip delay={0}><Button isIconOnly variant="ghost" size="sm" onPress={refresh}><RefreshCw size={16} /></Button><Tooltip.Content>刷新</Tooltip.Content></Tooltip>
          {task.status === "pending" && <Button size="sm" onPress={handleRun} style={{ background: "#22c55e", color: "#fff" }}><Play size={14} /> 运行</Button>}
          {(task.status === "running" || task.status === "planning") && (
            <>
              <Button size="sm" onPress={handlePause} style={{ background: "#a78bfa", color: "#fff" }}><Pause size={14} /> 暂停</Button>
              <Button size="sm" onPress={handleCancel} style={{ background: "#f59e0b", color: "#fff" }}><XCircle size={14} /> 取消</Button>
            </>
          )}
          {(task.status === "paused" || task.status === "interrupted" || task.status === "failed") && (
            <Button size="sm" onPress={handleResume} style={{ background: "#3b82f6", color: "#fff" }}><PlayCircle size={14} /> 恢复</Button>
          )}
          {["completed", "failed", "cancelled", "paused", "interrupted"].includes(task.status) && (
            <Button size="sm" onPress={handleRestart} style={{ background: "#6366f1", color: "#fff" }}><RefreshCw size={14} /> 重启</Button>
          )}
          {["completed", "failed", "cancelled"].includes(task.status) && (
            <Button size="sm" variant="ghost" onPress={handleDelete}><Trash2 size={14} /> 删除</Button>
          )}
        </div>
      </div>

      {/* Tabs */}
      <Tabs selectedKey={activeTab} onSelectionChange={(k) => setActiveTab(k as string)}>
        <Tabs.ListContainer>
          <Tabs.List aria-label="任务详情">
            <Tabs.Tab id="overview"><Info size={14} /> 概览<Tabs.Indicator /></Tabs.Tab>
            <Tabs.Tab id="execution"><Tabs.Separator /><GitBranch size={14} /> 执行链<Tabs.Indicator /></Tabs.Tab>
            <Tabs.Tab id="thread"><Tabs.Separator /><MessageSquare size={14} /> 线程<Tabs.Indicator /></Tabs.Tab>
            <Tabs.Tab id="artifacts"><Tabs.Separator /><FileText size={14} /> 产物<Tabs.Indicator /></Tabs.Tab>
            <Tabs.Tab id="cost"><Tabs.Separator /><DollarSign size={14} /> 成本<Tabs.Indicator /></Tabs.Tab>
          </Tabs.List>
        </Tabs.ListContainer>

        <Tabs.Panel id="overview"><OverviewPanel task={task} wm={wm} /></Tabs.Panel>
        <Tabs.Panel id="execution"><ExecutionPanel task={task} /></Tabs.Panel>
        <Tabs.Panel id="thread"><ThreadTab taskId={task.id} /></Tabs.Panel>
        <Tabs.Panel id="artifacts"><ArtifactsPanel task={task} /></Tabs.Panel>
        <Tabs.Panel id="cost"><CostPanel cost={cost} timeline={costTimeline} /></Tabs.Panel>
      </Tabs>
    </div>
  );
}

export default function TaskDetailPage() {
  return (
    <Suspense fallback={<div className="flex items-center justify-center h-[60vh]"><Spinner size="lg" /></div>}>
      <TaskDetailContent />
    </Suspense>
  );
}
