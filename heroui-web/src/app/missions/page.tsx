"use client";

import { useState, useEffect, useCallback } from "react";
import { useRouter } from "next/navigation";
import { api, type TaskInfo, type CronJob, type TriggerDef, type TaskTemplate } from "@/lib/api";
import { Card, Button, Spinner, Tabs, Chip, Tooltip } from "@heroui/react";
import {
  Zap, Plus, Trash2, Play, Clock, CheckCircle2, ListTodo,
  GitBranch, RefreshCw, Send, X, AlertTriangle, Pause,
  RotateCcw, Timer, Radio, FileText, Sparkles, ChevronDown,
  ChevronRight, Calendar, Power, PowerOff, Copy, Eye,
} from "lucide-react";
import { showToast } from "@/components/toast-provider";
import { STATUS_COLORS } from "@/lib/constants";
import EmptyState from "@/components/empty-state";

type MissionTab = "tasks" | "cron" | "triggers" | "templates";
type FilterTab = "all" | "active" | "scheduled" | "event" | "completed" | "failed";

export default function MissionsPage() {
  const router = useRouter();
  const [mTab, setMTab] = useState<MissionTab>("tasks");
  const [tasks, setTasks] = useState<TaskInfo[]>([]);
  const [cronJobs, setCronJobs] = useState<CronJob[]>([]);
  const [triggers, setTriggers] = useState<TriggerDef[]>([]);
  const [templates, setTemplates] = useState<TaskTemplate[]>([]);
  const [loading, setLoading] = useState(true);
  const [filter, setFilter] = useState<FilterTab>("all");

  // NL parse
  const [showCreate, setShowCreate] = useState(false);
  const [nlInput, setNlInput] = useState("");
  const [nlLoading, setNlLoading] = useState(false);
  const [parsedResult, setParsedResult] = useState<{ type: string; name: string; description: string; config: Record<string, unknown>; confidence: number; explanation: string } | null>(null);

  // Cron create
  const [showCronCreate, setShowCronCreate] = useState(false);
  const [cronName, setCronName] = useState("");
  const [cronExpr, setCronExpr] = useState("");
  const [cronMessage, setCronMessage] = useState("");
  const [cronCreating, setCronCreating] = useState(false);

  // Trigger create
  const [showTriggerCreate, setShowTriggerCreate] = useState(false);
  const [triggerName, setTriggerName] = useState("");
  const [triggerType, setTriggerType] = useState<string>("event");
  const [triggerEvent, setTriggerEvent] = useState("");
  const [triggerAction, setTriggerAction] = useState("");
  const [triggerCreating, setTriggerCreating] = useState(false);

  // Template instantiate
  const [instantiating, setInstantiating] = useState<string | null>(null);
  const [templateVars, setTemplateVars] = useState<Record<string, string>>({});

  const [deleting, setDeleting] = useState<string | null>(null);
  const [acting, setActing] = useState<string | null>(null);

  const loadAll = useCallback(async () => {
    setLoading(true);
    try {
      const [taskRes, cronRes, trigRes, tplRes] = await Promise.allSettled([
        api.taskList(),
        api.cronList(),
        api.getTriggersV2(),
        api.getTemplates(),
      ]);
      if (taskRes.status === "fulfilled") setTasks(Array.isArray(taskRes.value) ? taskRes.value : []);
      if (cronRes.status === "fulfilled") setCronJobs(cronRes.value.jobs || []);
      if (trigRes.status === "fulfilled") setTriggers(trigRes.value.triggers || []);
      if (tplRes.status === "fulfilled") setTemplates(tplRes.value.templates || []);
    } catch { /* ignore */ }
    setLoading(false);
  }, []);

  useEffect(() => { loadAll(); }, [loadAll]);

  // NL parse then create
  const handleNLParse = async () => {
    if (!nlInput.trim()) return;
    setNlLoading(true);
    setParsedResult(null);
    try {
      const result = await api.missionParse(nlInput);
      setParsedResult(result);
    } catch {
      // Fallback: direct create task
      try {
        await api.taskCreate(nlInput, nlInput);
        setNlInput("");
        setShowCreate(false);
        loadAll();
      } catch (e) { showToast(e instanceof Error ? e.message : "创建失败", "error"); }
    }
    setNlLoading(false);
  };

  const handleConfirmParsed = async () => {
    if (!parsedResult) return;
    setNlLoading(true);
    try {
      if (parsedResult.type === "cron") {
        await api.cronAdd(parsedResult.name, parsedResult.config.schedule as Record<string, unknown> || {}, parsedResult.config.payload as Record<string, unknown> || { kind: "message", message: parsedResult.description });
      } else if (parsedResult.type === "trigger") {
        await api.createTriggerV2({ name: parsedResult.name, description: parsedResult.description, type: "event", status: "active", actions: [{ type: "send_message", message: parsedResult.description }] });
      } else {
        await api.taskCreate(parsedResult.name, parsedResult.description);
      }
      setNlInput("");
      setParsedResult(null);
      setShowCreate(false);
      loadAll();
    } catch (e) { showToast(e instanceof Error ? e.message : "创建失败", "error"); }
    setNlLoading(false);
  };

  // Cron CRUD
  const handleCreateCron = async () => {
    if (!cronName.trim() || !cronExpr.trim()) return;
    setCronCreating(true);
    try {
      await api.cronAdd(cronName, { type: "cron", cron_expr: cronExpr }, { kind: "message", message: cronMessage || cronName });
      setCronName(""); setCronExpr(""); setCronMessage("");
      setShowCronCreate(false);
      loadAll();
    } catch (e) { showToast(e instanceof Error ? e.message : "创建失败", "error"); }
    setCronCreating(false);
  };

  const handleDeleteCron = async (id: string) => {
    setDeleting(id);
    try { await api.cronRemove(id); loadAll(); } catch (e) { showToast(e instanceof Error ? e.message : "删除失败", "error"); }
    setDeleting(null);
  };

  const handleRunCron = async (id: string) => {
    setActing(id);
    try { await api.cronRun(id); } catch (e) { showToast(e instanceof Error ? e.message : "执行失败", "error"); }
    setActing(null);
  };

  // Trigger CRUD
  const handleCreateTrigger = async () => {
    if (!triggerName.trim()) return;
    setTriggerCreating(true);
    try {
      await api.createTriggerV2({
        name: triggerName,
        type: triggerType as TriggerDef["type"],
        status: "active",
        event_config: triggerType === "event" ? { event_type: triggerEvent } : undefined,
        actions: [{ type: "send_message", message: triggerAction || triggerName }],
      });
      setTriggerName(""); setTriggerEvent(""); setTriggerAction("");
      setShowTriggerCreate(false);
      loadAll();
    } catch (e) { showToast(e instanceof Error ? e.message : "创建失败", "error"); }
    setTriggerCreating(false);
  };

  const handleDeleteTrigger = async (id: string) => {
    setDeleting(id);
    try { await api.deleteTriggerV2(id); loadAll(); } catch (e) { showToast(e instanceof Error ? e.message : "删除失败", "error"); }
    setDeleting(null);
  };

  // Task actions
  const deleteTask = async (id: string) => {
    setDeleting(id);
    try { await api.taskDelete(id); loadAll(); } catch (e) { showToast(e instanceof Error ? e.message : "删除失败", "error"); }
    setDeleting(null);
  };

  const taskAction = async (id: string, action: "run" | "pause" | "resume" | "cancel" | "restart") => {
    setActing(id);
    try {
      if (action === "run") { await api.taskRun(id); router.push(`/task-run?id=${id}`); return; }
      if (action === "pause") await api.taskPause(id);
      if (action === "resume") await api.taskResume(id);
      if (action === "cancel") await api.taskCancel(id);
      if (action === "restart") await api.taskRestart(id);
      loadAll();
    } catch (e) { showToast(e instanceof Error ? e.message : "操作失败", "error"); }
    setActing(null);
  };

  // Template instantiate
  const handleInstantiate = async (tplId: string) => {
    setActing(tplId);
    try {
      const res = await api.instantiateTemplate(tplId, templateVars);
      if (res?.id) router.push(`/task-run?id=${res.id}`);
      setInstantiating(null);
      setTemplateVars({});
      loadAll();
    } catch (e) { showToast(e instanceof Error ? e.message : "实例化失败", "error"); }
    setActing(null);
  };

  const handleDeleteTemplate = async (id: string) => {
    setDeleting(id);
    try { await api.deleteTemplate(id); loadAll(); } catch (e) { showToast(e instanceof Error ? e.message : "删除失败", "error"); }
    setDeleting(null);
  };

  // Filtered views
  const filtered = tasks.filter((t) => {
    if (filter === "all") return true;
    if (filter === "active") return ["running", "pending", "planning"].includes(t.status);
    if (filter === "scheduled") return t.status === "paused";
    if (filter === "completed") return ["completed", "done"].includes(t.status);
    if (filter === "failed") return t.status === "failed";
    return true;
  });

  const runningCount = tasks.filter((t) => ["running", "pending", "planning"].includes(t.status)).length;
  const doneCount = tasks.filter((t) => ["completed", "done"].includes(t.status)).length;
  const failedCount = tasks.filter((t) => t.status === "failed").length;

  if (loading) return <div className="flex-1 flex items-center justify-center"><Spinner size="lg" /></div>;

  return (
    <div className="page-root space-y-5 animate-fade-in-up" style={{ color: "var(--yunque-text)" }}>
      {/* Header */}
      <div className="flex items-center justify-between">
        <h1 className="page-title flex items-center gap-2"><Zap size={20} /> 任务中心</h1>
        <div className="flex gap-2">
          <Tooltip delay={0}>
            <Button variant="ghost" size="sm" onPress={loadAll}><RefreshCw size={14} /></Button>
            <Tooltip.Content>刷新</Tooltip.Content>
          </Tooltip>
          <Button size="sm" onPress={() => setShowCreate(true)} className="btn-accent">
            <Sparkles size={14} /> 智能创建
          </Button>
        </div>
      </div>

      {/* Summary stats */}
      <div className="stat-grid-5 stagger-children">
        <Card className="section-card px-3 py-2 hover-lift">
          <div className="kpi-sub stat-card-header"><Zap size={11} style={{ color: "var(--yunque-accent)" }} />总任务</div>
          <div className="kpi-value" style={{ fontSize: "var(--text-lg)" }}>{tasks.length}</div>
        </Card>
        <Card className="section-card px-3 py-2 hover-lift">
          <div className="kpi-sub stat-card-header"><Play size={11} style={{ color: "#3b82f6" }} />进行中</div>
          <div className="kpi-value" style={{ fontSize: "var(--text-lg)", color: "#3b82f6" }}>{runningCount}</div>
        </Card>
        <Card className="section-card px-3 py-2 hover-lift">
          <div className="kpi-sub stat-card-header"><CheckCircle2 size={11} style={{ color: "#22c55e" }} />已完成</div>
          <div className="kpi-value" style={{ fontSize: "var(--text-lg)", color: "#22c55e" }}>{doneCount}</div>
        </Card>
        <Card className="section-card px-3 py-2 hover-lift">
          <div className="kpi-sub stat-card-header"><Clock size={11} style={{ color: "#f59e0b" }} />定时任务</div>
          <div className="kpi-value" style={{ fontSize: "var(--text-lg)", color: "#f59e0b" }}>{cronJobs.length}</div>
        </Card>
        <Card className="section-card px-3 py-2 hover-lift">
          <div className="kpi-sub stat-card-header"><Radio size={11} style={{ color: "#a78bfa" }} />触发器</div>
          <div className="kpi-value" style={{ fontSize: "var(--text-lg)", color: "#a78bfa" }}>{triggers.length}</div>
        </Card>
      </div>

      {/* NL Smart Create Panel */}
      {showCreate && (
        <Card className="section-card animate-scale-in">
          <Card.Header className="flex items-center justify-between pb-2">
            <span className="text-sm font-medium flex items-center gap-2"><Sparkles size={14} /> 智能创建任务</span>
            <Button isIconOnly aria-label="关闭" variant="ghost" size="sm" onPress={() => { setShowCreate(false); setParsedResult(null); }}><X size={14} /></Button>
          </Card.Header>
          <Card.Content className="space-y-3 pt-0">
            <div className="flex gap-2">
              <input
                value={nlInput}
                onChange={(e) => setNlInput(e.target.value)}
                placeholder="用自然语言描述任务... 如 '每天早上9点生成日报' 或 '当收到邮件时自动回复'"
                onKeyDown={(e) => e.key === "Enter" && handleNLParse()}
                className="flex-1 px-3 py-2 rounded-lg text-sm bg-transparent outline-none"
                style={{ border: "1px solid var(--yunque-border)", color: "var(--yunque-text)" }}
                autoFocus
              />
              <Button size="sm" isPending={nlLoading} onPress={handleNLParse} className="btn-accent">
                <Send size={14} /> 解析
              </Button>
            </div>

            {/* AI Parse Result */}
            {parsedResult && (
              <div className="rounded-lg p-3 space-y-2 animate-fade-in" style={{ background: "rgba(0,111,238,0.05)", border: "1px solid rgba(0,111,238,0.15)" }}>
                <div className="flex items-center gap-2">
                  <Sparkles size={13} style={{ color: "var(--yunque-accent)" }} />
                  <span className="text-xs font-medium" style={{ color: "var(--yunque-accent)" }}>AI 解析结果</span>
                  <Chip style={{ background: "rgba(0,111,238,0.1)", color: "var(--yunque-accent)", fontSize: "var(--text-2xs)" }}>
                    {parsedResult.type} · {(parsedResult.confidence * 100).toFixed(0)}% 置信度
                  </Chip>
                </div>
                <div className="text-sm font-medium" style={{ color: "var(--yunque-text)" }}>{parsedResult.name}</div>
                <div className="text-xs" style={{ color: "var(--yunque-text-secondary)" }}>{parsedResult.explanation}</div>
                <div className="flex gap-2 mt-2">
                  <Button size="sm" onPress={handleConfirmParsed} isPending={nlLoading} className="btn-accent">
                    <CheckCircle2 size={13} /> 确认创建
                  </Button>
                  <Button size="sm" variant="ghost" onPress={() => setParsedResult(null)}>修改</Button>
                </div>
              </div>
            )}
          </Card.Content>
        </Card>
      )}

      {/* Mission type tabs */}
      <div className="flex gap-1.5 flex-wrap">
        {([
          { key: "tasks" as const, label: "任务", icon: <ListTodo size={13} />, count: tasks.length },
          { key: "cron" as const, label: "定时", icon: <Timer size={13} />, count: cronJobs.length },
          { key: "triggers" as const, label: "触发器", icon: <Radio size={13} />, count: triggers.length },
          { key: "templates" as const, label: "模板", icon: <FileText size={13} />, count: templates.length },
        ]).map(({ key, label, icon, count }) => (
          <button
            key={key}
            onClick={() => setMTab(key)}
            className="filter-pill"
            data-active={mTab === key}
          >
            {icon} {label}
            <span className="text-[10px] opacity-70">{count}</span>
          </button>
        ))}
      </div>

      {/* ===== TASKS TAB ===== */}
      {mTab === "tasks" && (
        <>
          {/* Filter pills */}
          <div className="flex gap-1.5 flex-wrap">
            {([
              { key: "all" as const, label: "全部" },
              { key: "active" as const, label: "进行中" },
              { key: "completed" as const, label: "已完成" },
              { key: "failed" as const, label: "失败" },
            ]).map(({ key, label }) => (
              <button
                key={key}
                onClick={() => setFilter(key)}
                className="filter-pill filter-pill-subtle"
                data-active={filter === key}
              >
                {label}
              </button>
            ))}
          </div>

          <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 gap-4 stagger-children">
            {filtered.map((t) => (
              <Card key={t.id} className="section-card hover-lift transition-all duration-200">
                <Card.Content className="py-3.5 px-4 space-y-2">
                  <div className="flex items-start justify-between gap-1">
                    <div className="flex items-center gap-1.5 min-w-0 flex-1">
                      <span className="inline-block w-2 h-2 rounded-full shrink-0" style={{ background: STATUS_COLORS[t.status] || "#9ca3af" }} />
                      <span className="text-sm font-medium truncate">{t.title || t.description || "未命名任务"}</span>
                    </div>
                    <div className="flex shrink-0">
                      <Tooltip delay={0}>
                        <Button size="sm" variant="ghost" onPress={() => taskAction(t.id, "run")} className="!p-0.5 !min-w-0"><Eye size={11} /></Button>
                        <Tooltip.Content>查看</Tooltip.Content>
                      </Tooltip>
                      <Tooltip delay={0}>
                        <Button size="sm" variant="ghost" isPending={deleting === t.id} onPress={() => deleteTask(t.id)} className="!p-0.5 !min-w-0"><Trash2 size={11} /></Button>
                        <Tooltip.Content>删除</Tooltip.Content>
                      </Tooltip>
                    </div>
                  </div>
                  <div className="flex items-center gap-1.5 flex-wrap">
                    <Chip size="sm" style={{ background: `${STATUS_COLORS[t.status] || "#9ca3af"}15`, color: STATUS_COLORS[t.status] || "#9ca3af", fontSize: "var(--text-2xs)" }}>
                      {t.status}
                    </Chip>
                    {t.type && <Chip size="sm" style={{ background: "rgba(255,255,255,0.05)", color: "var(--yunque-text-muted)", fontSize: "var(--text-2xs)" }}>{t.type}</Chip>}
                    {t.priority && <Chip size="sm" style={{ background: "rgba(255,255,255,0.05)", color: "var(--yunque-text-muted)", fontSize: "var(--text-2xs)" }}>P{t.priority}</Chip>}
                  </div>
                  <div className="flex items-center gap-2 flex-wrap">
                    {t.steps && <span className="text-xs" style={{ color: "var(--yunque-text-muted)" }}>{t.steps.length} 步</span>}
                    <span className="text-xs" style={{ color: "var(--yunque-text-muted)" }}>
                      {t.created_at ? new Date(t.created_at).toLocaleDateString() : ""}
                    </span>
                    {typeof t.constraints?.extra?.claimed_by === "string" && (
                      <Chip size="sm" style={{ fontSize: "var(--text-2xs)", background: "rgba(59,130,246,0.1)", color: "#3b82f6" }}>
                        ⚙ Worker: {t.constraints.extra.claimed_by}
                      </Chip>
                    )}
                    {t.error && (
                      <Tooltip delay={0}>
                        <AlertTriangle size={11} style={{ color: "#ef4444" }} />
                        <Tooltip.Content className="max-w-xs text-xs">{t.error}</Tooltip.Content>
                      </Tooltip>
                    )}
                  </div>
                  <div className="flex gap-1">
                    {t.status === "running" && (
                      <Button size="sm" variant="ghost" isPending={acting === t.id} onPress={() => taskAction(t.id, "pause")} className="text-xs !h-6"><Pause size={12} /> 暂停</Button>
                    )}
                    {t.status === "paused" && (
                      <Button size="sm" variant="ghost" isPending={acting === t.id} onPress={() => taskAction(t.id, "resume")} className="text-xs !h-6"><Play size={12} /> 继续</Button>
                    )}
                    {["failed", "completed", "done", "cancelled"].includes(t.status) && (
                      <Button size="sm" variant="ghost" isPending={acting === t.id} onPress={() => taskAction(t.id, "restart")} className="text-xs !h-6"><RotateCcw size={12} /> 重新执行</Button>
                    )}
                  </div>
                </Card.Content>
              </Card>
            ))}
            {filtered.length === 0 && (
              <div className="col-span-full">
                <EmptyState icon={<Zap size={24} style={{ color: "var(--yunque-accent)" }} />} title="暂无任务" description="点击「智能创建」开始" />
              </div>
            )}
          </div>
        </>
      )}

      {/* ===== CRON TAB ===== */}
      {mTab === "cron" && (
        <>
          <div className="flex justify-end">
            <Button size="sm" onPress={() => setShowCronCreate(true)} className="btn-accent">
              <Plus size={14} /> 新建定时任务
            </Button>
          </div>

          {showCronCreate && (
            <Card className="section-card animate-scale-in">
              <Card.Header className="flex items-center justify-between pb-2">
                <span className="text-sm font-medium flex items-center gap-2"><Timer size={14} /> 新建定时任务</span>
                <Button isIconOnly aria-label="关闭" variant="ghost" size="sm" onPress={() => setShowCronCreate(false)}><X size={14} /></Button>
              </Card.Header>
              <Card.Content className="space-y-3 pt-0">
                <input
                  value={cronName} onChange={(e) => setCronName(e.target.value)}
                  placeholder="任务名称"
                  className="w-full px-3 py-2 rounded-lg text-sm bg-transparent outline-none"
                  style={{ border: "1px solid var(--yunque-border)", color: "var(--yunque-text)" }}
                />
                <div className="flex gap-2">
                  <input
                    value={cronExpr} onChange={(e) => setCronExpr(e.target.value)}
                    placeholder="Cron 表达式, 如 0 9 * * * (每天9点)"
                    className="flex-1 px-3 py-2 rounded-lg text-sm bg-transparent outline-none"
                    style={{ border: "1px solid var(--yunque-border)", color: "var(--yunque-text)" }}
                  />
                </div>
                <input
                  value={cronMessage} onChange={(e) => setCronMessage(e.target.value)}
                  placeholder="执行消息 (发送给 Agent 的指令)"
                  className="w-full px-3 py-2 rounded-lg text-sm bg-transparent outline-none"
                  style={{ border: "1px solid var(--yunque-border)", color: "var(--yunque-text)" }}
                />
                <div className="flex gap-2 text-[10px]" style={{ color: "var(--yunque-text-muted)" }}>
                  常用: <Button size="sm" variant="ghost" onPress={() => setCronExpr("*/5 * * * *")} className="underline p-0 min-w-0 h-auto">每5分钟</Button>
                  <Button size="sm" variant="ghost" onPress={() => setCronExpr("0 * * * *")} className="underline p-0 min-w-0 h-auto">每小时</Button>
                  <Button size="sm" variant="ghost" onPress={() => setCronExpr("0 9 * * *")} className="underline p-0 min-w-0 h-auto">每天9点</Button>
                  <Button size="sm" variant="ghost" onPress={() => setCronExpr("0 9 * * 1")} className="underline p-0 min-w-0 h-auto">每周一9点</Button>
                </div>
                <Button size="sm" isPending={cronCreating} isDisabled={!cronName.trim() || !cronExpr.trim()} onPress={handleCreateCron} className="btn-accent">
                  创建
                </Button>
              </Card.Content>
            </Card>
          )}

          <div className="space-y-2 stagger-children">
            {cronJobs.map((job) => (
              <Card key={job.id} className="section-card hover-lift transition-all duration-200">
                <Card.Content className="flex items-center justify-between py-3">
                  <div className="flex items-center gap-3 min-w-0 flex-1">
                    <div className="w-8 h-8 rounded-lg flex items-center justify-center shrink-0" style={{ background: job.enabled ? "rgba(34,197,94,0.1)" : "rgba(255,255,255,0.04)" }}>
                      <Timer size={15} style={{ color: job.enabled ? "#22c55e" : "#9ca3af" }} />
                    </div>
                    <div className="min-w-0 flex-1">
                      <div className="text-sm font-medium truncate">{job.name}</div>
                      <div className="flex items-center gap-2 mt-0.5 flex-wrap">
                        <Chip size="sm" style={{ background: "rgba(0,111,238,0.05)", color: "var(--yunque-text-muted)", fontSize: "var(--text-2xs)" }}>
                          <Calendar size={9} className="mr-1" />{job.schedule.cron_expr || `${job.schedule.every_ms}ms`}
                        </Chip>
                        <Chip size="sm" style={{ background: job.enabled ? "rgba(34,197,94,0.1)" : "rgba(255,255,255,0.05)", color: job.enabled ? "#22c55e" : "#9ca3af", fontSize: "var(--text-2xs)" }}>
                          {job.enabled ? "启用" : "禁用"}
                        </Chip>
                        <span className="text-[10px]" style={{ color: "var(--yunque-text-muted)" }}>执行 {job.run_count} 次</span>
                        {job.next_run_at && <span className="text-[10px]" style={{ color: "var(--yunque-text-muted)" }}>下次: {new Date(job.next_run_at).toLocaleString()}</span>}
                      </div>
                    </div>
                  </div>
                  <div className="flex gap-0.5 shrink-0">
                    <Tooltip delay={0}>
                      <Button size="sm" variant="ghost" isPending={acting === job.id} onPress={() => handleRunCron(job.id)}><Play size={12} /></Button>
                      <Tooltip.Content>立即执行</Tooltip.Content>
                    </Tooltip>
                    <Tooltip delay={0}>
                      <Button size="sm" variant="ghost" isPending={deleting === job.id} onPress={() => handleDeleteCron(job.id)}><Trash2 size={12} /></Button>
                      <Tooltip.Content>删除</Tooltip.Content>
                    </Tooltip>
                  </div>
                </Card.Content>
              </Card>
            ))}
            {cronJobs.length === 0 && (
              <div className="text-center py-16" style={{ color: "var(--yunque-text-muted)" }}>
                <Timer size={40} className="mx-auto mb-3 opacity-30" />
                <div>暂无定时任务</div>
              </div>
            )}
          </div>
        </>
      )}

      {/* ===== TRIGGERS TAB ===== */}
      {mTab === "triggers" && (
        <>
          <div className="flex justify-end">
            <Button size="sm" onPress={() => setShowTriggerCreate(true)} className="btn-accent">
              <Plus size={14} /> 新建触发器
            </Button>
          </div>

          {showTriggerCreate && (
            <Card className="section-card animate-scale-in">
              <Card.Header className="flex items-center justify-between pb-2">
                <span className="text-sm font-medium flex items-center gap-2"><Radio size={14} /> 新建触发器</span>
                <Button isIconOnly aria-label="关闭" variant="ghost" size="sm" onPress={() => setShowTriggerCreate(false)}><X size={14} /></Button>
              </Card.Header>
              <Card.Content className="space-y-3 pt-0">
                <input
                  value={triggerName} onChange={(e) => setTriggerName(e.target.value)}
                  placeholder="触发器名称"
                  className="w-full px-3 py-2 rounded-lg text-sm bg-transparent outline-none"
                  style={{ border: "1px solid var(--yunque-border)", color: "var(--yunque-text)" }}
                />
                <div className="flex gap-2 flex-wrap">
                  {(["event", "time", "condition", "cognitive"] as const).map((t) => (
                    <button
                      key={t}
                      onClick={() => setTriggerType(t)}
                      className="filter-pill"
                      data-active={triggerType === t}
                    >
                      {t === "event" ? "事件" : t === "time" ? "时间" : t === "condition" ? "条件" : "认知"}
                    </button>
                  ))}
                </div>
                {triggerType === "event" && (
                  <input
                    value={triggerEvent} onChange={(e) => setTriggerEvent(e.target.value)}
                    placeholder="事件类型, 如 email.received, message.incoming"
                    className="w-full px-3 py-2 rounded-lg text-sm bg-transparent outline-none"
                    style={{ border: "1px solid var(--yunque-border)", color: "var(--yunque-text)" }}
                  />
                )}
                <input
                  value={triggerAction} onChange={(e) => setTriggerAction(e.target.value)}
                  placeholder="触发动作 (发送给 Agent 的消息)"
                  className="w-full px-3 py-2 rounded-lg text-sm bg-transparent outline-none"
                  style={{ border: "1px solid var(--yunque-border)", color: "var(--yunque-text)" }}
                />
                <Button size="sm" isPending={triggerCreating} isDisabled={!triggerName.trim()} onPress={handleCreateTrigger} className="btn-accent">
                  创建
                </Button>
              </Card.Content>
            </Card>
          )}

          <div className="space-y-2 stagger-children">
            {triggers.map((trig) => {
              const typeColor = trig.type === "event" ? "#3b82f6" : trig.type === "time" ? "#f59e0b" : trig.type === "condition" ? "#06b6d4" : "#a78bfa";
              return (
                <Card key={trig.id} className="section-card hover-lift transition-all duration-200">
                  <Card.Content className="flex items-center justify-between py-3">
                    <div className="flex items-center gap-3 min-w-0 flex-1">
                      <div className="w-8 h-8 rounded-lg flex items-center justify-center shrink-0" style={{ background: `${typeColor}15` }}>
                        <Radio size={15} style={{ color: typeColor }} />
                      </div>
                      <div className="min-w-0 flex-1">
                        <div className="text-sm font-medium truncate">{trig.name}</div>
                        {trig.description && <div className="text-[11px] truncate mt-0.5" style={{ color: "var(--yunque-text-muted)" }}>{trig.description}</div>}
                        <div className="flex items-center gap-2 mt-0.5 flex-wrap">
                          <Chip size="sm" style={{ background: `${typeColor}15`, color: typeColor, fontSize: "var(--text-2xs)" }}>{trig.type}</Chip>
                          <Chip size="sm" style={{ background: trig.status === "active" ? "rgba(34,197,94,0.1)" : "rgba(255,255,255,0.05)", color: trig.status === "active" ? "#22c55e" : "#9ca3af", fontSize: "var(--text-2xs)" }}>
                            {trig.status}
                          </Chip>
                          <span className="text-[10px]" style={{ color: "var(--yunque-text-muted)" }}>触发 {trig.run_count} 次</span>
                          {trig.last_error && (
                            <Tooltip delay={0}>
                              <AlertTriangle size={11} style={{ color: "#ef4444" }} />
                              <Tooltip.Content className="max-w-xs text-xs">{trig.last_error}</Tooltip.Content>
                            </Tooltip>
                          )}
                        </div>
                      </div>
                    </div>
                    <div className="flex gap-0.5 shrink-0">
                      <Tooltip delay={0}>
                        <Button size="sm" variant="ghost" isPending={deleting === trig.id} onPress={() => handleDeleteTrigger(trig.id)}><Trash2 size={12} /></Button>
                        <Tooltip.Content>删除</Tooltip.Content>
                      </Tooltip>
                    </div>
                  </Card.Content>
                </Card>
              );
            })}
            {triggers.length === 0 && (
              <div className="text-center py-16" style={{ color: "var(--yunque-text-muted)" }}>
                <Radio size={40} className="mx-auto mb-3 opacity-30" />
                <div>暂无触发器</div>
              </div>
            )}
          </div>
        </>
      )}

      {/* ===== TEMPLATES TAB ===== */}
      {mTab === "templates" && (
        <div className="space-y-2 stagger-children">
          {templates.map((tpl) => (
            <Card key={tpl.id} className="section-card hover-lift transition-all duration-200">
              <Card.Content className="py-3">
                <div className="flex items-center justify-between">
                  <div className="flex items-center gap-3 min-w-0 flex-1">
                    <div className="w-8 h-8 rounded-lg flex items-center justify-center shrink-0" style={{ background: "rgba(0,111,238,0.08)" }}>
                      <FileText size={15} style={{ color: "var(--yunque-accent)" }} />
                    </div>
                    <div className="min-w-0 flex-1">
                      <div className="text-sm font-medium truncate">{tpl.name}</div>
                      <div className="text-[11px] truncate" style={{ color: "var(--yunque-text-muted)" }}>{tpl.description}</div>
                      <div className="flex items-center gap-2 mt-1 flex-wrap">
                        {tpl.variables?.map((v) => (
                          <Chip key={v.name} size="sm" style={{ background: "rgba(0,111,238,0.05)", color: "var(--yunque-text-muted)", fontSize: "var(--text-2xs)" }}>
                            ${`{${v.name}}`}{v.required && " *"}
                          </Chip>
                        ))}
                        {tpl.tags?.map((tag) => (
                          <Chip key={tag} size="sm" style={{ background: "rgba(255,255,255,0.04)", color: "var(--yunque-text-muted)", fontSize: "var(--text-2xs)" }}>#{tag}</Chip>
                        ))}
                        <span className="text-[10px]" style={{ color: "var(--yunque-text-muted)" }}>{tpl.steps?.length || 0} 步骤</span>
                      </div>
                    </div>
                  </div>
                  <div className="flex gap-0.5 shrink-0">
                    <Tooltip delay={0}>
                      <Button size="sm" variant="ghost" onPress={() => { setInstantiating(tpl.id); setTemplateVars({}); }}><Copy size={12} /></Button>
                      <Tooltip.Content>使用模板</Tooltip.Content>
                    </Tooltip>
                    <Tooltip delay={0}>
                      <Button size="sm" variant="ghost" isPending={deleting === tpl.id} onPress={() => handleDeleteTemplate(tpl.id)}><Trash2 size={12} /></Button>
                      <Tooltip.Content>删除</Tooltip.Content>
                    </Tooltip>
                  </div>
                </div>

                {/* Template variable inputs */}
                {instantiating === tpl.id && (
                  <div className="mt-3 pt-3 space-y-2 animate-fade-in" style={{ borderTop: "1px solid var(--yunque-border)" }}>
                    <div className="text-xs font-medium" style={{ color: "var(--yunque-text-muted)" }}>填写变量</div>
                    {tpl.variables?.map((v) => (
                      <div key={v.name} className="flex items-center gap-2">
                        <span className="text-xs w-24 shrink-0" style={{ color: "var(--yunque-text-secondary)" }}>{v.name}{v.required ? " *" : ""}:</span>
                        <input
                          value={templateVars[v.name] || ""}
                          onChange={(e) => setTemplateVars((prev) => ({ ...prev, [v.name]: e.target.value }))}
                          placeholder={v.description || v.default || ""}
                          className="flex-1 px-2.5 py-1 rounded text-xs bg-transparent outline-none"
                          style={{ border: "1px solid var(--yunque-border)", color: "var(--yunque-text)" }}
                        />
                      </div>
                    ))}
                    <div className="flex gap-2">
                      <Button size="sm" isPending={acting === tpl.id} onPress={() => handleInstantiate(tpl.id)} className="btn-accent">
                        <Play size={12} /> 创建并执行
                      </Button>
                      <Button size="sm" variant="ghost" onPress={() => setInstantiating(null)}>取消</Button>
                    </div>
                  </div>
                )}
              </Card.Content>
            </Card>
          ))}
          {templates.length === 0 && (
            <EmptyState icon={<FileText size={24} style={{ color: "var(--yunque-accent)" }} />} title="暂无任务模板" description="模板可在「智能创建」中自动生成" />
          )}
        </div>
      )}
    </div>
  );
}
