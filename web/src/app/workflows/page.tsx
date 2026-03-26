"use client";

import { useState, useEffect, useCallback } from "react";
import Link from "next/link";
import { useI18n } from "@/lib/i18n";
import { useToast } from "@/components/ui/toast";
import { BlurFade } from "@/components/ui/blur-fade";
import {
  GitBranch, Plus, Trash2, Play, Clock, CheckCircle2,
  XCircle, Pause, RefreshCw, ChevronRight, ChevronDown,
  Activity, AlertCircle, Layers,
} from "lucide-react";

function apiHeaders() {
  return {
    "Content-Type": "application/json",
    "X-API-Key": typeof window !== "undefined" ? localStorage.getItem("yunque_api_key") || "" : "",
  };
}

interface WorkflowDef {
  id: string;
  name: string;
  description: string;
  version: number;
  nodes: any[];
  edges: any[];
  created_at: string;
  updated_at: string;
}

interface NodeState {
  node_id: string;
  status: string;
  input?: any;
  output?: any;
  error?: string;
  retry_count: number;
  started_at?: string;
  finished_at?: string;
}

interface WorkflowInstance {
  id: string;
  definition_id: string;
  status: string;
  node_states?: Record<string, NodeState>;
  error?: string;
  created_at: string;
  started_at?: string;
  finished_at?: string;
}

const statusConfig: Record<string, { color: string; icon: React.ElementType; label: string }> = {
  pending: { color: "#9ca3af", icon: Clock, label: "等待中" },
  running: { color: "#3b82f6", icon: RefreshCw, label: "运行中" },
  paused: { color: "#eab308", icon: Pause, label: "已暂停" },
  completed: { color: "#22c55e", icon: CheckCircle2, label: "已完成" },
  failed: { color: "#ef4444", icon: XCircle, label: "失败" },
  cancelled: { color: "#6b7280", icon: XCircle, label: "已取消" },
  done: { color: "#22c55e", icon: CheckCircle2, label: "完成" },
  skipped: { color: "#9ca3af", icon: Pause, label: "跳过" },
};

function fmtDuration(start?: string, end?: string): string {
  if (!start) return "—";
  const s = new Date(start).getTime();
  const e = end ? new Date(end).getTime() : Date.now();
  const ms = e - s;
  if (ms < 1000) return `${ms}ms`;
  if (ms < 60000) return `${(ms / 1000).toFixed(1)}s`;
  return `${Math.floor(ms / 60000)}m ${Math.floor((ms % 60000) / 1000)}s`;
}

function fmtTime(ts?: string) {
  return ts ? new Date(ts).toLocaleString("zh-CN", { hour12: false }) : "—";
}

// ── Node execution detail row ──
function NodeStateRow({ ns, nodeName }: { ns: NodeState; nodeName: string }) {
  const [open, setOpen] = useState(false);
  const sc = statusConfig[ns.status] || statusConfig.pending;
  const Icon = sc.icon;
  return (
    <div className="rounded-lg border" style={{ borderColor: "var(--border)", background: "var(--bg)" }}>
      <div className="flex items-center gap-2 px-3 py-1.5 cursor-pointer" onClick={() => setOpen(!open)}>
        {open ? <ChevronDown size={10} /> : <ChevronRight size={10} />}
        <span className="text-[10px] font-mono w-4 h-4 rounded flex items-center justify-center"
          style={{ background: sc.color + "22", color: sc.color }}><Icon size={10} /></span>
        <span className="text-[11px] font-medium flex-1">{nodeName || ns.node_id}</span>
        <span className="text-[10px]" style={{ color: "var(--text-muted)" }}>
          {fmtDuration(ns.started_at, ns.finished_at)}
        </span>
      </div>
      {open && (
        <div className="px-3 py-2 border-t text-[10px] space-y-1" style={{ borderColor: "var(--border)", color: "var(--text-muted)" }}>
          {ns.error && <div className="text-red-400">⚠ {ns.error}</div>}
          {ns.output && (
            <div>
              <span className="font-medium">输出: </span>
              <span className="font-mono break-all">
                {typeof ns.output === "string" ? ns.output.slice(0, 200) : JSON.stringify(ns.output).slice(0, 200)}
              </span>
            </div>
          )}
          {ns.retry_count > 0 && <div>重试: {ns.retry_count}次</div>}
        </div>
      )}
    </div>
  );
}

// ── Single run detail ──
function RunDetail({ inst, workflowNodes }: { inst: WorkflowInstance; workflowNodes: any[] }) {
  const [open, setOpen] = useState(false);
  const sc = statusConfig[inst.status] || statusConfig.pending;
  const Icon = sc.icon;
  const nodeStates = inst.node_states ? Object.values(inst.node_states) : [];
  const doneCount = nodeStates.filter(n => n.status === "done").length;

  return (
    <div className="rounded-lg border overflow-hidden" style={{ borderColor: "var(--border)", background: "var(--bg-card)" }}>
      <div className="flex items-center gap-2 px-3 py-2 cursor-pointer hover:opacity-80 transition-opacity"
        onClick={() => setOpen(!open)}>
        {open ? <ChevronDown size={12} /> : <ChevronRight size={12} />}
        <span className="text-[10px] px-1.5 py-0.5 rounded-full font-medium"
          style={{ background: sc.color + "22", color: sc.color }}>
          <Icon size={10} className="inline mr-0.5" style={{ verticalAlign: "-1px" }} />
          {sc.label}
        </span>
        <span className="text-[10px] font-mono flex-1" style={{ color: "var(--text-muted)" }}>
          {inst.id.slice(0, 8)}
        </span>
        <span className="text-[10px]" style={{ color: "var(--text-muted)" }}>
          {doneCount}/{nodeStates.length} 节点
        </span>
        <span className="text-[10px]" style={{ color: "var(--text-muted)" }}>
          {fmtDuration(inst.started_at || inst.created_at, inst.finished_at)}
        </span>
        <span className="text-[10px]" style={{ color: "var(--text-muted)" }}>
          {fmtTime(inst.created_at)}
        </span>
      </div>
      {open && (
        <div className="border-t px-3 py-2 space-y-1.5" style={{ borderColor: "var(--border)" }}>
          {inst.error && (
            <div className="flex items-center gap-1.5 text-[11px] text-red-400 px-2 py-1.5 rounded"
              style={{ background: "#ef444410" }}>
              <AlertCircle size={12} /> {inst.error}
            </div>
          )}
          {nodeStates.length > 0 ? (
            <div className="space-y-1">
              {nodeStates.map(ns => {
                const nodeInfo = workflowNodes.find((n: any) => n.id === ns.node_id);
                return <NodeStateRow key={ns.node_id} ns={ns} nodeName={nodeInfo?.name || ns.node_id} />;
              })}
            </div>
          ) : (
            <div className="text-[10px] py-2 text-center" style={{ color: "var(--text-muted)" }}>无执行详情</div>
          )}
        </div>
      )}
    </div>
  );
}

export default function WorkflowsPage() {
  const { locale } = useI18n();
  const zh = locale === "zh";
  const toast = useToast();
  const [workflows, setWorkflows] = useState<WorkflowDef[]>([]);
  const [instances, setInstances] = useState<WorkflowInstance[]>([]);
  const [loading, setLoading] = useState(true);
  const [creating, setCreating] = useState(false);
  const [newName, setNewName] = useState("");
  const [newDesc, setNewDesc] = useState("");
  const [expandedWf, setExpandedWf] = useState<Set<string>>(new Set());

  const load = useCallback(async () => {
    try {
      const [wRes, iRes] = await Promise.all([
        fetch("/v1/workflows", { headers: apiHeaders() }).then(r => r.json()),
        fetch("/v1/workflows/instances", { headers: apiHeaders() }).then(r => r.json()).catch(() => ({ instances: [] })),
      ]);
      setWorkflows(wRes.workflows || []);
      setInstances(iRes.instances || []);
    } catch (e) { toast.error(zh ? "加载工作流失败" : "Failed to load workflows"); } finally { setLoading(false); }
  }, []);

  useEffect(() => { load(); }, [load]);

  const createWorkflow = async () => {
    if (!newName.trim()) return;
    try {
      await fetch("/v1/workflows", {
        method: "POST", headers: apiHeaders(),
        body: JSON.stringify({ name: newName, description: newDesc, nodes: [], edges: [] }),
      });
      toast.success(zh ? "工作流已创建" : "Workflow created");
      setCreating(false); setNewName(""); setNewDesc("");
      load();
    } catch (e) { toast.error(zh ? "创建失败" : "Failed to create workflow"); }
  };

  const deleteWorkflow = async (id: string) => {
    try {
      await fetch(`/v1/workflows?id=${id}`, { method: "DELETE", headers: apiHeaders() });
      toast.success(zh ? "已删除" : "Deleted");
      load();
    } catch { toast.error(zh ? "删除失败" : "Failed to delete"); }
  };

  const toggleExpand = (id: string) => {
    setExpandedWf(prev => {
      const s = new Set(prev);
      s.has(id) ? s.delete(id) : s.add(id);
      return s;
    });
  };

  if (loading) return (
    <div className="flex items-center justify-center py-20">
      <RefreshCw size={20} className="animate-spin" style={{ color: "var(--text-muted)" }} />
    </div>
  );

  return (
    <div className="animate-in">
      {/* Header */}
      <div className="flex items-center justify-between mb-6">
        <div className="flex items-center gap-3">
          <div className="w-10 h-10 rounded-xl flex items-center justify-center" style={{ background: "var(--accent-subtle)" }}>
            <GitBranch size={20} style={{ color: "var(--accent)" }} />
          </div>
          <div>
            <h1 className="text-xl font-semibold tracking-tight">{zh ? "工作流" : "Workflows"}</h1>
            <p className="text-xs mt-0.5" style={{ color: "var(--text-muted)" }}>
              {workflows.length} {zh ? "个工作流" : "workflows"} · {instances.length} {zh ? "次运行" : "runs"}
            </p>
          </div>
        </div>
        <button onClick={() => setCreating(!creating)}
          className="btn-glow px-4 py-2.5 rounded-xl text-xs font-medium flex items-center gap-1.5">
          {creating ? <XCircle size={13} /> : <Plus size={13} />}
          {zh ? "新建工作流" : "New Workflow"}
        </button>
      </div>

      {/* Create Form */}
      {creating && (
        <BlurFade delay={0.05}>
          <div className="rounded-xl border p-5 mb-4 space-y-3" style={{ background: "var(--bg-card)", borderColor: "var(--border)" }}>
            <input value={newName} onChange={e => setNewName(e.target.value)} placeholder={zh ? "工作流名称" : "Workflow name"}
              className="w-full px-4 py-3 rounded-xl border text-sm outline-none" style={{ background: "var(--bg)", borderColor: "var(--border)", color: "var(--text)" }} />
            <input value={newDesc} onChange={e => setNewDesc(e.target.value)} placeholder={zh ? "描述（可选）" : "Description (optional)"}
              className="w-full px-4 py-3 rounded-xl border text-sm outline-none" style={{ background: "var(--bg)", borderColor: "var(--border)", color: "var(--text)" }} />
            <button onClick={createWorkflow} disabled={!newName.trim()}
              className="px-4 py-2 rounded-lg text-xs font-medium disabled:opacity-40" style={{ background: "var(--accent)", color: "#000" }}>
              {zh ? "创建" : "Create"}
            </button>
          </div>
        </BlurFade>
      )}

      {/* Workflow Cards */}
      <div className="space-y-2">
        {workflows.map((w, i) => {
          const runs = instances.filter(inst => inst.definition_id === w.id);
          const lastRun = runs[0];
          const sc = lastRun ? statusConfig[lastRun.status] || statusConfig.pending : null;
          const isExpanded = expandedWf.has(w.id);
          return (
            <BlurFade key={w.id} delay={0.05 + i * 0.03}>
              <div className="rounded-xl border overflow-hidden"
                style={{ background: "var(--bg-card)", borderColor: isExpanded ? "var(--accent)" + "40" : "var(--border)" }}>
                {/* Workflow header row */}
                <div className="card-hover px-5 py-4 flex items-center gap-4">
                  <div className="w-10 h-10 rounded-lg flex items-center justify-center" style={{ background: "var(--accent-subtle)" }}>
                    <GitBranch size={18} style={{ color: "var(--accent)" }} />
                  </div>
                  <div className="flex-1 min-w-0">
                    <div className="flex items-center gap-2">
                      <span className="text-sm font-medium">{w.name}</span>
                      <span className="text-[10px] px-1.5 py-0.5 rounded" style={{ background: "var(--bg-hover)", color: "var(--text-muted)" }}>
                        v{w.version}
                      </span>
                    </div>
                    <div className="text-xs mt-1 flex gap-3" style={{ color: "var(--text-muted)" }}>
                      {w.description && <span className="truncate max-w-[200px]">{w.description}</span>}
                      <span><Layers size={11} className="inline mr-0.5" style={{ verticalAlign: "-1px" }} />{w.nodes?.length || 0} {zh ? "节点" : "nodes"}</span>
                      {sc && lastRun && (
                        <span className="flex items-center gap-1" style={{ color: sc.color }}>
                          <sc.icon size={11} />
                          {sc.label}
                        </span>
                      )}
                    </div>
                  </div>
                  {/* Run history button */}
                  <button onClick={() => toggleExpand(w.id)}
                    className="px-2.5 py-1.5 rounded-lg text-[11px] flex items-center gap-1 border transition-all cursor-pointer"
                    style={{
                      borderColor: isExpanded ? "var(--accent)" + "40" : "var(--border)",
                      color: isExpanded ? "var(--accent)" : "var(--text-muted)",
                      background: isExpanded ? "var(--accent-subtle)" : "transparent",
                    }}>
                    <Activity size={12} />
                    {runs.length} {zh ? "次" : "runs"}
                    {isExpanded ? <ChevronDown size={12} /> : <ChevronRight size={12} />}
                  </button>
                  <Link href={`/workflows/${w.id}`}
                    className="p-2.5 rounded-lg transition-colors flex items-center gap-1 text-xs"
                    style={{ color: "var(--accent)" }}>
                    {zh ? "编辑" : "Edit"} <ChevronRight size={14} />
                  </Link>
                  <button onClick={() => deleteWorkflow(w.id)}
                    className="p-2.5 rounded-lg hover:bg-[var(--danger-bg)]" style={{ color: "var(--text-muted)" }}>
                    <Trash2 size={15} />
                  </button>
                </div>

                {/* Expanded execution history */}
                {isExpanded && (
                  <div className="border-t px-5 py-3 space-y-2" style={{ borderColor: "var(--border)", background: "var(--bg)" }}>
                    <div className="flex items-center justify-between">
                      <span className="text-[11px] font-medium flex items-center gap-1.5" style={{ color: "var(--text-muted)" }}>
                        <Activity size={12} /> {zh ? "执行记录" : "Execution History"}
                      </span>
                      <span className="text-[10px]" style={{ color: "var(--text-muted)" }}>
                        {zh ? `最近 ${runs.length} 次` : `Last ${runs.length}`}
                      </span>
                    </div>
                    {runs.length === 0 ? (
                      <div className="text-[11px] text-center py-6 rounded-lg border" style={{ color: "var(--text-muted)", borderColor: "var(--border)", borderStyle: "dashed" }}>
                        {zh ? "暂无执行记录" : "No runs yet"}
                      </div>
                    ) : (
                      <div className="space-y-1.5">
                        {runs.slice(0, 10).map(inst => (
                          <RunDetail key={inst.id} inst={inst} workflowNodes={w.nodes || []} />
                        ))}
                        {runs.length > 10 && (
                          <div className="text-[10px] text-center py-1" style={{ color: "var(--text-muted)" }}>
                            {zh ? `还有 ${runs.length - 10} 条记录...` : `${runs.length - 10} more...`}
                          </div>
                        )}
                      </div>
                    )}
                  </div>
                )}
              </div>
            </BlurFade>
          );
        })}
        {workflows.length === 0 && (
          <div className="text-sm text-center py-16 rounded-xl border" style={{ color: "var(--text-muted)", borderColor: "var(--border)", borderStyle: "dashed" }}>
            <GitBranch size={32} className="mx-auto mb-3 opacity-30" />
            {zh ? "暂无工作流，点击「新建」开始创建" : "No workflows yet. Click 'New' to create one."}
          </div>
        )}
      </div>
    </div>
  );
}

