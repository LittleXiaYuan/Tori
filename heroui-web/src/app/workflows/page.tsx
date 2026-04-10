"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import { api, type WorkflowDef, type WorkflowInstance } from "@/lib/api";
import { Card, Button, Spinner, Chip, Tooltip, Table } from "@heroui/react";
import {
  GitBranch, Plus, Trash2, Play, Eye, RefreshCw, Clock,
  CheckCircle2, XCircle, Pause, Square, Layers, Settings2,
} from "lucide-react";
import { showToast } from "@/components/toast-provider";
import { STATUS_COLORS, relTime } from "@/lib/constants";
import { useApiData } from "@/lib/use-api-data";
import EmptyState from "@/components/empty-state";

const statusLabel: Record<string, string> = {
  pending: "待执行", running: "运行中", paused: "已暂停",
  completed: "已完成", failed: "失败", cancelled: "已取消",
};

export default function WorkflowsPage() {
  const router = useRouter();
  const { data, loading, refresh } = useApiData(
    async () => {
      const [wfRes, instRes] = await Promise.all([
        api.workflowList().catch(() => ({ workflows: [] as WorkflowDef[], total: 0 })),
        api.workflowInstances().catch(() => ({ instances: [] as WorkflowInstance[], total: 0 })),
      ]);
      const instData = instRes as { instances: WorkflowInstance[]; total: number };
      return { workflows: wfRes.workflows || [], instances: instData.instances || [] };
    },
    { workflows: [] as WorkflowDef[], instances: [] as WorkflowInstance[] },
  );
  const { workflows, instances } = data;
  const [tab, setTab] = useState<"definitions" | "instances">("definitions");
  const [runningId, setRunningId] = useState<string | null>(null);

  const handleRun = async (defId: string) => {
    setRunningId(defId);
    try {
      await api.workflowRun(defId);
      await refresh();
    } catch (e) { showToast(e instanceof Error ? e.message : "启动失败", "error"); }
    setRunningId(null);
  };

  const handleDelete = async (id: string) => {
    try {
      await api.workflowDelete(id);
      refresh();
    } catch (e) { showToast(e instanceof Error ? e.message : "删除失败", "error"); }
  };

  const handleCancel = async (instanceId: string) => {
    try {
      await api.workflowCancel(instanceId);
      await refresh();
    } catch (e) { showToast(e instanceof Error ? e.message : "取消失败", "error"); }
  };

  if (loading) {
    return (
      <div className="flex items-center justify-center h-[60vh]">
        <Spinner size="lg" />
      </div>
    );
  }

  return (
    <div className="page-root space-y-6 animate-fade-in-up">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-3">
          <div className="w-9 h-9 rounded-lg flex items-center justify-center" style={{ background: "rgba(139,92,246,0.12)" }}>
            <GitBranch size={18} style={{ color: "#8b5cf6" }} />
          </div>
          <div>
            <h1 className="page-title">工作流</h1>
            <p className="text-xs" style={{ color: "var(--yunque-text-muted)" }}>管理 DAG 工作流定义与执行实例</p>
          </div>
        </div>
        <div className="flex items-center gap-2">
          <Tooltip delay={0}>
            <Button isIconOnly variant="ghost" size="sm" onPress={refresh} style={{ color: "var(--yunque-text-muted)" }}>
              <RefreshCw size={14} />
            </Button>
            <Tooltip.Content>刷新</Tooltip.Content>
          </Tooltip>
          <Button
            size="sm"
            className="gap-1.5 rounded-lg btn-accent"
            onPress={() => router.push("/workflow-editor")}
          >
            <Plus size={14} /> 新建工作流
          </Button>
        </div>
      </div>

      {/* Stat Cards */}
      <div className="grid grid-cols-1 sm:grid-cols-3 gap-4 stagger-children">
        <Card className="section-card p-4">
          <div className="flex items-center gap-1.5 kpi-label mb-1">
            <Layers size={13} /> 工作流定义
          </div>
          <div className="kpi-value">{workflows.length}</div>
        </Card>
        <Card className="section-card p-4">
          <div className="flex items-center gap-1.5 kpi-label mb-1">
            <Play size={13} /> 运行实例
          </div>
          <div className="kpi-value">{instances.length}</div>
        </Card>
        <Card className="section-card p-4">
          <div className="flex items-center gap-1.5 kpi-label mb-1">
            <CheckCircle2 size={13} /> 成功率
          </div>
          <div className="kpi-value">
            {instances.length > 0
              ? `${Math.round((instances.filter((i) => i.status === "completed").length / instances.length) * 100)}%`
              : "-"}
          </div>
        </Card>
      </div>

      {/* Tabs */}
      <div className="flex items-center gap-1.5">
        {[
          { key: "definitions" as const, label: "定义", count: workflows.length },
          { key: "instances" as const, label: "实例", count: instances.length },
        ].map(({ key, label, count }) => (
          <button
            key={key}
            onClick={() => setTab(key)}
            className="filter-pill"
            data-active={tab === key}
          >
            {label}
            <span className="text-[10px] opacity-70">{count}</span>
          </button>
        ))}
      </div>

      {/* Definitions Tab */}
      {tab === "definitions" && (
        <Card>
          {workflows.length === 0 ? (
            <EmptyState
              icon={<GitBranch size={24} style={{ color: "#8b5cf6" }} />}
              title="还没有工作流"
              description="创建你的第一个自动化工作流"
              actionLabel="新建工作流"
              onAction={() => router.push("/workflow-editor")}
            />
          ) : (
            <Table>
              <Table.ScrollContainer>
                <Table.Content aria-label="工作流列表" className="min-w-[600px]">
                  <Table.Header>
                    <Table.Column isRowHeader>名称</Table.Column>
                    <Table.Column>节点数</Table.Column>
                    <Table.Column>版本</Table.Column>
                    <Table.Column>更新时间</Table.Column>
                    <Table.Column>操作</Table.Column>
                  </Table.Header>
                  <Table.Body>
                    {workflows.map((wf) => (
                      <Table.Row key={wf.id}>
                        <Table.Cell>
                          <div className="flex items-center gap-2.5">
                            <div className="w-8 h-8 rounded-lg flex items-center justify-center shrink-0" style={{ background: "rgba(139,92,246,0.12)" }}>
                              <GitBranch size={14} style={{ color: "#8b5cf6" }} />
                            </div>
                            <div className="min-w-0">
                              <div className="text-sm font-medium truncate" style={{ color: "var(--yunque-text)" }}>{wf.name}</div>
                              <div className="text-[11px] truncate" style={{ color: "var(--yunque-text-muted)" }}>{wf.description || wf.id}</div>
                            </div>
                          </div>
                        </Table.Cell>
                        <Table.Cell>
                          <Chip size="sm" style={{ background: "rgba(255,255,255,0.06)", color: "var(--yunque-text-secondary)" }}>
                            {wf.nodes?.length || 0} 节点
                          </Chip>
                        </Table.Cell>
                        <Table.Cell>
                          <span className="text-xs font-mono" style={{ color: "var(--yunque-text-secondary)" }}>v{wf.version}</span>
                        </Table.Cell>
                        <Table.Cell>
                          <span className="text-xs" style={{ color: "var(--yunque-text-muted)" }}>{relTime(wf.updated_at)}</span>
                        </Table.Cell>
                        <Table.Cell>
                          <div className="flex items-center gap-1">
                            <Tooltip delay={0}>
                              <Button isIconOnly variant="ghost" size="sm" onPress={() => router.push(`/workflow-editor?id=${wf.id}`)}>
                                <Settings2 size={13} />
                              </Button>
                              <Tooltip.Content>编辑</Tooltip.Content>
                            </Tooltip>
                            <Tooltip delay={0}>
                              <Button
                                isIconOnly variant="ghost" size="sm"
                                isPending={runningId === wf.id}
                                onPress={() => handleRun(wf.id)}
                              >
                                <Play size={13} style={{ color: "#22c55e" }} />
                              </Button>
                              <Tooltip.Content>运行</Tooltip.Content>
                            </Tooltip>
                            <Tooltip delay={0}>
                              <Button isIconOnly variant="ghost" size="sm" onPress={() => handleDelete(wf.id)}>
                                <Trash2 size={13} style={{ color: "#ef4444" }} />
                              </Button>
                              <Tooltip.Content>删除</Tooltip.Content>
                            </Tooltip>
                          </div>
                        </Table.Cell>
                      </Table.Row>
                    ))}
                  </Table.Body>
                </Table.Content>
              </Table.ScrollContainer>
            </Table>
          )}
        </Card>
      )}

      {/* Instances Tab */}
      {tab === "instances" && (
        <Card>
          {instances.length === 0 ? (
            <EmptyState icon={<Play size={24} style={{ color: "#3b82f6" }} />} title="暂无运行实例" description="在「工作流定义」标签中选择一个工作流并点击运行，执行记录将显示在这里。" />
          ) : (
            <Table>
              <Table.ScrollContainer>
                <Table.Content aria-label="工作流实例" className="min-w-[600px]">
                  <Table.Header>
                    <Table.Column isRowHeader>实例 ID</Table.Column>
                    <Table.Column>工作流</Table.Column>
                    <Table.Column>状态</Table.Column>
                    <Table.Column>开始时间</Table.Column>
                    <Table.Column>操作</Table.Column>
                  </Table.Header>
                  <Table.Body>
                    {instances.map((inst) => {
                      const color = STATUS_COLORS[inst.status] || STATUS_COLORS.pending;
                      const label = statusLabel[inst.status] || inst.status;
                      const wfName = workflows.find((w) => w.id === inst.definition_id)?.name || inst.definition_id;
                      return (
                        <Table.Row key={inst.id}>
                          <Table.Cell>
                            <span className="text-xs font-mono" style={{ color: "var(--yunque-text-secondary)" }}>{inst.id.slice(0, 8)}</span>
                          </Table.Cell>
                          <Table.Cell>
                            <span className="text-sm" style={{ color: "var(--yunque-text)" }}>{wfName}</span>
                          </Table.Cell>
                          <Table.Cell>
                            <Chip size="sm" style={{ background: `${color}18`, color }}>
                              {label}
                            </Chip>
                          </Table.Cell>
                          <Table.Cell>
                            <span className="text-xs" style={{ color: "var(--yunque-text-muted)" }}>{relTime(inst.started_at || inst.created_at)}</span>
                          </Table.Cell>
                          <Table.Cell>
                            <div className="flex items-center gap-1">
                              {inst.status === "running" && (
                                <Tooltip delay={0}>
                                  <Button isIconOnly variant="ghost" size="sm" onPress={() => handleCancel(inst.id)}>
                                    <Square size={13} style={{ color: "#ef4444" }} />
                                  </Button>
                                  <Tooltip.Content>取消</Tooltip.Content>
                                </Tooltip>
                              )}
                            </div>
                          </Table.Cell>
                        </Table.Row>
                      );
                    })}
                  </Table.Body>
                </Table.Content>
              </Table.ScrollContainer>
            </Table>
          )}
        </Card>
      )}
    </div>
  );
}
