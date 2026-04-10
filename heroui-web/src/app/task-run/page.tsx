"use client";

import { useEffect, useState, useCallback } from "react";
import { Card, Button, Spinner, Chip, Tooltip, ProgressBar } from "@heroui/react";
import { api, type TaskInfo, type CostTaskSummary, type LLMMessage, type SessionQueueInfo } from "@/lib/api";
import { Terminal, Clock, CheckCircle, XCircle, Pause, Play, Square, RefreshCw, FileText, DollarSign, MessageSquare, Archive, ListOrdered } from "lucide-react";
import { STATUS_COLORS } from "@/lib/constants";
import { usePolling } from "@/lib/use-polling";
import PageHeader from "@/components/page-header";
import EmptyState from "@/components/empty-state";
import { showToast } from "@/components/toast-provider";
import { ListSkeleton } from "@/components/skeleton-loader";

const QUEUE_STATUS_STYLE: Record<string, { bg: string; fg: string }> = {
  queued: { bg: "rgba(245,158,11,0.1)", fg: "#f59e0b" },
  running: { bg: "rgba(59,130,246,0.1)", fg: "#3b82f6" },
  completed: { bg: "rgba(34,197,94,0.1)", fg: "#22c55e" },
  cancelled: { bg: "rgba(107,114,128,0.1)", fg: "#6b7280" },
};

export default function TaskRunPage() {
  const [tasks, setTasks] = useState<TaskInfo[]>([]);
  const [selectedTask, setSelectedTask] = useState<TaskInfo | null>(null);
  const [taskRun, setTaskRun] = useState<TaskInfo | null>(null);
  const [threads, setThreads] = useState<LLMMessage[]>([]);
  const [cost, setCost] = useState<CostTaskSummary | null>(null);
  const [activeTab, setActiveTab] = useState("overview");
  const [loading, setLoading] = useState(true);
  const [viewMode, setViewMode] = useState<"tasks" | "queue">("tasks");
  const [queue, setQueue] = useState<SessionQueueInfo | null>(null);
  const [queueLoading, setQueueLoading] = useState(false);

  const load = useCallback(async () => {
    try {
      const res = await api.taskList();
      const allTasks = Array.isArray(res) ? res : (res as { tasks: TaskInfo[] }).tasks || [];
      const active = allTasks.filter((t) => ["running", "pending", "paused"].includes(t.status));
      const completed = allTasks.filter((t) => ["completed", "failed", "cancelled"].includes(t.status)).slice(0, 10);
      setTasks([...active, ...completed]);
    } catch { /* offline */ }
    finally { setLoading(false); }
  }, []);

  useEffect(() => { load(); }, [load]);

  usePolling(load, 3000);

  const selectTask = async (task: TaskInfo) => {
    setSelectedTask(task);
    setActiveTab("overview");
    try {
      const [run, threadRes, costRes] = await Promise.all([
        api.taskGet(task.id).catch(() => null),
        api.getTaskThread(task.id).catch(() => ({ info: null, messages: [] })),
        api.getCostByTask(task.id).catch(() => null),
      ]);
      setTaskRun(run);
      setThreads(threadRes?.messages || []);
      setCost(costRes);
    } catch { /* no data */ }
  };

  const cancelTask = async (id: string) => {
    await api.taskCancel(id);
    load();
  };

  const loadQueue = useCallback(async () => {
    setQueueLoading(true);
    try {
      const res = await api.sessionQueueStatus();
      setQueue(res);
    } catch { /* offline */ }
    finally { setQueueLoading(false); }
  }, []);

  const cancelQueueItem = async (sessionId: string, taskId: string) => {
    try {
      await api.sessionQueueCancel(sessionId, taskId);
      showToast("队列任务已取消", "success");
      loadQueue();
    } catch (e) {
      showToast(e instanceof Error ? e.message : "取消失败", "error");
    }
  };

  useEffect(() => {
    if (viewMode === "queue") loadQueue();
  }, [viewMode, loadQueue]);

  if (loading) {
    return <ListSkeleton rows={5} />;
  }

  return (
    <div className="page-root animate-fade-in-up">
      <div className="flex items-center justify-between mb-4">
        <PageHeader icon={<Terminal size={20} />} title="任务执行" onRefresh={() => { if (viewMode === "tasks") { setLoading(true); load(); } else { loadQueue(); } }} />
        <div className="flex items-center gap-1 p-1 rounded-lg" style={{ background: "rgba(255,255,255,0.04)" }}>
          {([
            { key: "tasks" as const, label: "任务", icon: <Terminal size={13} /> },
            { key: "queue" as const, label: "队列", icon: <ListOrdered size={13} /> },
          ]).map(({ key, label, icon }) => (
            <button key={key} onClick={() => setViewMode(key)}
              className="flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-xs font-medium transition-all"
              style={{
                background: viewMode === key ? "var(--yunque-accent)" : "transparent",
                color: viewMode === key ? "#fff" : "var(--yunque-text-muted)",
              }}>
              {icon} {label}
            </button>
          ))}
        </div>
      </div>

      {viewMode === "queue" ? (
        <div className="space-y-4 animate-fade-in-up">
          {queue && (
            <div className="flex items-center gap-4">
              <div className="flex items-center gap-2 px-3 py-1.5 rounded-lg" style={{ background: "rgba(59,130,246,0.08)" }}>
                <span className="text-xs" style={{ color: "var(--yunque-text-muted)" }}>运行中</span>
                <span className="text-sm font-medium" style={{ color: "#3b82f6" }}>{queue.running}</span>
              </div>
              <div className="flex items-center gap-2 px-3 py-1.5 rounded-lg" style={{ background: "rgba(245,158,11,0.08)" }}>
                <span className="text-xs" style={{ color: "var(--yunque-text-muted)" }}>总计</span>
                <span className="text-sm font-medium" style={{ color: "#f59e0b" }}>{queue.total}</span>
              </div>
              {queue.session_id && (
                <span className="text-xs font-mono" style={{ color: "var(--yunque-text-muted)" }}>
                  会话: {queue.session_id.slice(0, 12)}…
                </span>
              )}
            </div>
          )}

          {queueLoading ? (
            <div className="flex justify-center py-16"><Spinner /></div>
          ) : !queue || queue.tasks.length === 0 ? (
            <EmptyState icon={<ListOrdered size={32} />} title="队列为空" description="当前没有排队中的任务" />
          ) : (
            <div className="space-y-2 stagger-children">
              {queue.tasks.map((item) => {
                const style = QUEUE_STATUS_STYLE[item.status] || QUEUE_STATUS_STYLE.queued;
                return (
                  <Card key={item.id} className="section-card p-4 hover-lift">
                    <div className="flex items-center justify-between">
                      <div className="flex-1 min-w-0">
                        <div className="flex items-center gap-2 mb-1">
                          <span className="text-sm font-medium truncate" style={{ color: "var(--yunque-text)" }}>
                            {item.title || item.id}
                          </span>
                          <Chip size="sm" style={{ background: style.bg, color: style.fg, fontSize: 10 }}>
                            {item.status}
                          </Chip>
                          {item.priority > 0 && (
                            <Chip size="sm" style={{ background: "rgba(168,85,247,0.1)", color: "#a855f7", fontSize: 10 }}>
                              P{item.priority}
                            </Chip>
                          )}
                        </div>
                        <div className="flex items-center gap-3 text-[10px]" style={{ color: "var(--yunque-text-muted)" }}>
                          <span className="font-mono">{item.id.slice(0, 12)}</span>
                          <span className="flex items-center gap-1">
                            <Clock size={10} />{new Date(item.created_at).toLocaleString()}
                          </span>
                        </div>
                      </div>
                      {(item.status === "queued" || item.status === "running") && queue && (
                        <Tooltip delay={0}>
                          <Button isIconOnly variant="ghost" size="sm"
                            onPress={() => cancelQueueItem(queue.session_id, item.id)}
                            style={{ color: "#ef4444" }}>
                            <XCircle size={16} />
                          </Button>
                          <Tooltip.Content>取消</Tooltip.Content>
                        </Tooltip>
                      )}
                    </div>
                  </Card>
                );
              })}
            </div>
          )}
        </div>
      ) : (
      <div className="grid grid-cols-12 gap-6">
        {/* Task list */}
        <div className="col-span-4 space-y-2">
          {tasks.length === 0 ? (
            <EmptyState icon={<Terminal size={24} style={{ color: "var(--yunque-accent)" }} />} title="暂无任务" description="通过聊天或任务管理创建任务后在此查看" />
          ) : (
            <div className="space-y-2 stagger-children">
              {tasks.map((task) => (
                <Card
                  key={task.id}
                  className="p-3 cursor-pointer hover-lift"
                  style={{
                    background: selectedTask?.id === task.id ? "rgba(0,111,238,0.08)" : "var(--yunque-card)",
                    borderColor: selectedTask?.id === task.id ? "var(--yunque-accent)" : "var(--yunque-border)",
                    borderWidth: 1,
                    borderStyle: "solid",
                  }}
                  onClick={() => selectTask(task)}
                >
                  <div className="flex items-center justify-between mb-1">
                    <span className="text-sm font-medium truncate" style={{ color: "var(--yunque-text)" }}>{task.title || task.id}</span>
                    <span
                      className={task.status === "running" ? "animate-pulse-dot" : ""}
                      style={{
                        width: 8, height: 8, borderRadius: "50%",
                        background: STATUS_COLORS[task.status] || "var(--yunque-text-muted)",
                        display: "inline-block",
                      }}
                    />
                  </div>
                  <div className="flex items-center gap-2">
                    <Chip size="sm" style={{ background: `${STATUS_COLORS[task.status]}15`, color: STATUS_COLORS[task.status], fontSize: 10 }}>
                      {task.status}
                    </Chip>
                    <span className="text-xs" style={{ color: "var(--yunque-text-muted)" }}>
                      {task.created_at ? new Date(task.created_at).toLocaleString() : ""}
                    </span>
                  </div>
                </Card>
              ))}
            </div>
          )}
        </div>

        {/* Task detail */}
        <div className="col-span-8">
          {selectedTask ? (
            <div className="space-y-4 animate-fade-in-up">
              {/* Task header */}
              <Card className="section-card p-5">
                <div className="flex items-center justify-between mb-3">
                  <h2 className="text-lg font-medium" style={{ color: "var(--yunque-text)" }}>{selectedTask.title || selectedTask.id}</h2>
                  <div className="flex items-center gap-2">
                    {selectedTask.status === "running" && (
                      <Tooltip delay={0}>
                        <Button isIconOnly variant="ghost" size="sm" onPress={() => cancelTask(selectedTask.id)}
                          style={{ color: "var(--yunque-danger)" }}>
                          <Square size={14} />
                        </Button>
                        <Tooltip.Content>{"取消"}</Tooltip.Content>
                      </Tooltip>
                    )}
                    <Chip size="sm" style={{ background: `${STATUS_COLORS[selectedTask.status]}15`, color: STATUS_COLORS[selectedTask.status] }}>
                      {selectedTask.status}
                    </Chip>
                  </div>
                </div>
                {selectedTask.description && (
                  <div className="text-sm" style={{ color: "var(--yunque-text-secondary)" }}>{selectedTask.description}</div>
                )}
              </Card>

              {/* 5 Tabs */}
              <div className="flex items-center gap-1 p-1 rounded-lg" style={{ background: "rgba(255,255,255,0.04)" }}>
                {[
                  { key: "overview", label: "概览", icon: <FileText size={13} /> },
                  { key: "execution", label: "执行", icon: <Terminal size={13} /> },
                  { key: "thread", label: "线程", icon: <MessageSquare size={13} /> },
                  { key: "artifacts", label: "产物", icon: <Archive size={13} /> },
                  { key: "cost", label: "成本", icon: <DollarSign size={13} /> },
                ].map(({ key, label, icon }) => (
                  <button
                    key={key}
                    onClick={() => setActiveTab(key)}
                    className="flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-xs font-medium transition-all"
                    style={{
                      background: activeTab === key ? "var(--yunque-accent)" : "transparent",
                      color: activeTab === key ? "#fff" : "var(--yunque-text-muted)",
                    }}
                  >
                    {icon} {label}
                  </button>
                ))}
              </div>

              {/* Tab: Overview */}
              {activeTab === "overview" && (
                <Card className="section-card p-5">
                  <div className="grid grid-cols-2 gap-4">
                    {[
                      { label: "状态", value: selectedTask.status },
                      { label: "类型", value: selectedTask.type || "—" },
                      { label: "创建时间", value: selectedTask.created_at ? new Date(selectedTask.created_at).toLocaleString() : "—" },
                      { label: "更新时间", value: selectedTask.updated_at ? new Date(selectedTask.updated_at).toLocaleString() : "—" },
                      { label: "优先级", value: selectedTask.priority || "normal" },
                      { label: "ID", value: selectedTask.id },
                    ].map(({ label, value }) => (
                      <div key={label} className="p-3 rounded-lg" style={{ background: "rgba(255,255,255,0.03)" }}>
                        <div className="text-[11px] font-medium mb-1" style={{ color: "var(--yunque-text-muted)" }}>{label}</div>
                        <div className="text-sm font-mono" style={{ color: "var(--yunque-text)" }}>{value}</div>
                      </div>
                    ))}
                  </div>
                  {taskRun?.working_memory && (
                    <div className="mt-4 p-3 rounded-lg" style={{ background: "rgba(255,255,255,0.03)" }}>
                      <div className="text-[11px] font-medium mb-2" style={{ color: "var(--yunque-text-muted)" }}>{"工作记忆"}</div>
                      <pre className="text-xs font-mono whitespace-pre-wrap" style={{ color: "var(--yunque-text-secondary)" }}>
                        {JSON.stringify(taskRun.working_memory, null, 2)}
                      </pre>
                    </div>
                  )}
                </Card>
              )}

              {/* Tab: Execution timeline */}
              {activeTab === "execution" && (
                <Card className="section-card p-5">
                  <h3 className="text-sm font-medium mb-4" style={{ color: "var(--yunque-text)" }}>{"执行步骤"}</h3>
                  {taskRun?.steps && taskRun.steps.length > 0 ? (
                    <div className="space-y-3">
                      {taskRun.steps.map((step, i) => (
                        <div key={i} className="flex gap-3">
                          <div className="flex flex-col items-center">
                            <div className="w-6 h-6 rounded-full flex items-center justify-center shrink-0"
                              style={{ background: step.status === "completed" ? "rgba(34,197,94,0.2)" : step.status === "failed" ? "rgba(239,68,68,0.2)" : "rgba(0,111,238,0.2)" }}>
                              {step.status === "completed" ? (
                                <CheckCircle size={12} style={{ color: "var(--yunque-success)" }} />
                              ) : step.status === "failed" ? (
                                <XCircle size={12} style={{ color: "var(--yunque-danger)" }} />
                              ) : (
                                <Clock size={12} style={{ color: "var(--yunque-accent)" }} />
                              )}
                            </div>
                            {i < (taskRun.steps?.length ?? 0) - 1 && (
                              <div className="w-px flex-1 my-1" style={{ background: "var(--yunque-border)" }} />
                            )}
                          </div>
                          <div className="flex-1 pb-3">
                            <div className="text-sm font-medium" style={{ color: "var(--yunque-text)" }}>{step.name || step.tool || `Step ${i + 1}`}</div>
                            {step.output && (
                              <div className="text-xs mt-1 p-2 rounded-lg font-mono" style={{ background: "rgba(255,255,255,0.03)", color: "var(--yunque-text-secondary)" }}>
                                {step.output}
                              </div>
                            )}
                            {step.duration_ms && (
                              <span className="text-xs mt-1 inline-block" style={{ color: "var(--yunque-text-muted)" }}>
                                {step.duration_ms}ms
                              </span>
                            )}
                          </div>
                        </div>
                      ))}
                    </div>
                  ) : (
                    <div className="text-sm text-center py-6" style={{ color: "var(--yunque-text-muted)" }}>{"暂无执行步骤数据"}</div>
                  )}
                </Card>
              )}

              {/* Tab: Thread */}
              {activeTab === "thread" && (
                <Card className="section-card p-5">
                  <h3 className="text-sm font-medium mb-4" style={{ color: "var(--yunque-text)" }}>{"任务线程"}</h3>
                  {threads.length > 0 ? (
                    <div className="space-y-3">
                      {threads.map((t, i) => (
                        <div key={i} className="p-3 rounded-lg" style={{ background: "rgba(255,255,255,0.03)" }}>
                          <div className="flex items-center gap-2 mb-1">
                            <Chip size="sm" style={{ background: `${t.role === "user" ? "var(--yunque-accent)" : "var(--yunque-success)"}15`, color: t.role === "user" ? "var(--yunque-accent)" : "var(--yunque-success)", fontSize: 10 }}>
                              {t.role || "system"}
                            </Chip>
                            <span className="text-[10px]" style={{ color: "var(--yunque-text-muted)" }}>
                              {t.created_at ? new Date(t.created_at).toLocaleTimeString() : ""}
                            </span>
                          </div>
                          <div className="text-sm whitespace-pre-wrap" style={{ color: "var(--yunque-text-secondary)" }}>{t.content}</div>
                        </div>
                      ))}
                    </div>
                  ) : (
                    <div className="text-sm text-center py-6" style={{ color: "var(--yunque-text-muted)" }}>{"暂无线程消息"}</div>
                  )}
                </Card>
              )}

              {/* Tab: Artifacts */}
              {activeTab === "artifacts" && (
                <Card className="section-card p-5">
                  <h3 className="text-sm font-medium mb-4" style={{ color: "var(--yunque-text)" }}>{"产物"}</h3>
                  {taskRun?.artifacts && taskRun.artifacts.length > 0 ? (
                    <div className="space-y-2">
                      {taskRun.artifacts.map((a, i) => (
                        <div key={i} className="flex items-center gap-3 p-3 rounded-lg" style={{ background: "rgba(255,255,255,0.03)" }}>
                          <FileText size={16} style={{ color: "var(--yunque-accent)" }} />
                          <div className="flex-1">
                            <div className="text-sm font-medium" style={{ color: "var(--yunque-text)" }}>{a.name || a.path || `Artifact ${i + 1}`}</div>
                            <div className="text-xs" style={{ color: "var(--yunque-text-muted)" }}>{a.type || a.mime_type || ""}</div>
                          </div>
                          {a.size && <span className="text-xs" style={{ color: "var(--yunque-text-muted)" }}>{(a.size / 1024).toFixed(1)}KB</span>}
                        </div>
                      ))}
                    </div>
                  ) : (
                    <div className="text-sm text-center py-6" style={{ color: "var(--yunque-text-muted)" }}>{"暂无产物"}</div>
                  )}
                </Card>
              )}

              {/* Tab: Cost */}
              {activeTab === "cost" && (
                <Card className="section-card p-5">
                  <h3 className="text-sm font-medium mb-4" style={{ color: "var(--yunque-text)" }}>{"成本概览"}</h3>
                  {cost ? (
                    <div className="grid grid-cols-3 gap-4">
                      <div className="p-4 rounded-lg text-center" style={{ background: "rgba(255,255,255,0.03)" }}>
                        <div className="kpi-value" style={{ color: "var(--yunque-accent)" }}>${(cost.total_cost_usd || 0).toFixed(4)}</div>
                        <div className="kpi-sub mt-1">{"总成本"}</div>
                      </div>
                      <div className="p-4 rounded-lg text-center" style={{ background: "rgba(255,255,255,0.03)" }}>
                        <div className="kpi-value">{(cost.total_tokens_in || 0).toLocaleString()}</div>
                        <div className="kpi-sub mt-1">输入 Tokens</div>
                      </div>
                      <div className="p-4 rounded-lg text-center" style={{ background: "rgba(255,255,255,0.03)" }}>
                        <div className="kpi-value">{(cost.total_tokens_out || 0).toLocaleString()}</div>
                        <div className="kpi-sub mt-1">输出 Tokens</div>
                      </div>
                    </div>
                  ) : (
                    <div className="text-sm text-center py-6" style={{ color: "var(--yunque-text-muted)" }}>{"暂无成本数据"}</div>
                  )}
                </Card>
              )}
            </div>
          ) : (
            <Card className="section-card p-12 text-center">
              <Terminal size={40} className="mx-auto mb-3" style={{ color: "var(--yunque-text-muted)" }} />
              <div className="text-sm" style={{ color: "var(--yunque-text-muted)" }}>{"选择一个任务查看执行详情"}</div>
            </Card>
          )}
        </div>
      </div>
      )}
    </div>
  );
}
