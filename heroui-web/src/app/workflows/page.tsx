"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import { api, type WorkflowDef, type WorkflowInstance } from "@/lib/api";
import { Card, Button, Spinner, Chip, Tooltip, Table, TextArea } from "@heroui/react";
import {
  GitBranch, Plus, Trash2, Play, RefreshCw,
  CheckCircle2, Square, Layers, Settings2, Sparkles, Wand2, ArrowRight,
} from "lucide-react";
import { showToast } from "@/components/toast-provider";
import { STATUS_COLORS, relTime } from "@/lib/constants";
import { useApiData } from "@/lib/use-api-data";
import EmptyState from "@/components/empty-state";

const statusLabel: Record<string, string> = {
  pending: "待执行", running: "运行中", paused: "已暂停",
  completed: "已完成", failed: "失败", cancelled: "已取消",
};

const workflowExamples = [
  "每天早上 9 点读取知识库中的项目周报模板，汇总昨天任务，生成日报并发送给团队。",
  "当客户反馈里出现高风险关键词时，检索历史工单，生成处理建议，并创建跟进任务。",
  "每周五整理本周研发进度，生成 PPT 大纲，列出风险、下周计划和负责人。",
];

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
  const [requirement, setRequirement] = useState(workflowExamples[0]);
  const [generating, setGenerating] = useState(false);
  const [generatedWorkflow, setGeneratedWorkflow] = useState<WorkflowDef | null>(null);
  const [generatedBy, setGeneratedBy] = useState<string | null>(null);
  const [generateMessage, setGenerateMessage] = useState<string>("");

  const handleGenerate = async () => {
    const trimmed = requirement.trim();
    if (!trimmed) {
      showToast("请先描述你想自动化的流程", "warning");
      return;
    }
    setGenerating(true);
    try {
      const res = await api.workflowGenerate(trimmed);
      setGeneratedWorkflow(res.workflow);
      setGeneratedBy(res.generated_by);
      setGenerateMessage(res.message || "工作流已生成");
      setTab("definitions");
      await refresh();
      showToast(res.generated_by === "template" ? "已用内置模板生成工作流" : "已通过模型生成工作流", "success");
    } catch (e) {
      showToast(e instanceof Error ? e.message : "生成失败", "error");
    } finally {
      setGenerating(false);
    }
  };

  const handleRun = async (defId: string) => {
    setRunningId(defId);
    try {
      await api.workflowRun(defId);
      await refresh();
      showToast("工作流已启动", "success");
    } catch (e) { showToast(e instanceof Error ? e.message : "启动失败", "error"); }
    setRunningId(null);
  };

  const handleDelete = async (id: string) => {
    if (!confirm("确定要删除这个工作流吗？此操作不可恢复。")) return;
    try {
      await api.workflowDelete(id);
      refresh();
      showToast("工作流已删除", "success");
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

      {/* NL2Workflow */}
      <Card className="section-card overflow-hidden" style={{ border: "1px solid rgba(139,92,246,0.24)" }}>
        <div className="grid grid-cols-1 lg:grid-cols-[minmax(0,1fr)_280px] gap-0">
          <div className="p-5 space-y-4">
            <div className="flex flex-wrap items-center justify-between gap-3">
              <div className="flex items-center gap-3">
                <div className="w-10 h-10 rounded-xl flex items-center justify-center" style={{ background: "linear-gradient(135deg, rgba(139,92,246,0.2), rgba(59,130,246,0.14))", color: "#a78bfa" }}>
                  <Wand2 size={18} />
                </div>
                <div>
                  <div className="flex items-center gap-2">
                    <h2 className="text-base font-semibold" style={{ color: "var(--yunque-text)" }}>自然语言生成工作流</h2>
                    <Chip size="sm" style={{ background: "rgba(139,92,246,0.14)", color: "#a78bfa" }}>NL2Workflow</Chip>
                  </div>
                  <p className="text-xs mt-1" style={{ color: "var(--yunque-text-muted)" }}>描述目标即可生成可编辑 DAG；模型不可用时自动使用模板兜底，演示不会断流。</p>
                </div>
              </div>
              {generatedWorkflow && (
                <Button
                  size="sm"
                  variant="ghost"
                  className="gap-1.5"
                  onPress={() => router.push(`/workflow-editor?id=${generatedWorkflow.id}`)}
                >
                  打开编辑器 <ArrowRight size={13} />
                </Button>
              )}
            </div>

            <TextArea
              aria-label="自然语言工作流需求"
              rows={4}
              value={requirement}
              onChange={(e) => setRequirement(e.target.value)}
              placeholder="例如：每天早上 9 点读取知识库中的项目周报模板，汇总昨天任务，生成日报并发送给团队。"
              style={{ fontSize: "var(--text-sm)" }}
            />

            <div className="flex flex-wrap items-center gap-2">
              {workflowExamples.map((example, index) => (
                <button
                  key={index}
                  className="filter-pill"
                  data-active={requirement === example}
                  onClick={() => setRequirement(example)}
                  type="button"
                >
                  示例 {index + 1}
                </button>
              ))}
              <div className="flex-1" />
              <Button
                className="gap-1.5 rounded-lg btn-accent"
                isPending={generating}
                onPress={handleGenerate}
              >
                <Sparkles size={14} /> 生成工作流
              </Button>
            </div>

            {generatedWorkflow && (
              <div
                className="flex flex-wrap items-center justify-between gap-3 rounded-xl px-4 py-3"
                style={{ background: "rgba(34,197,94,0.08)", border: "1px solid rgba(34,197,94,0.18)" }}
              >
                <div className="min-w-0">
                  <div className="flex items-center gap-2 text-sm font-medium" style={{ color: "var(--yunque-text)" }}>
                    <CheckCircle2 size={15} style={{ color: "#22c55e" }} />
                    已生成：{generatedWorkflow.name}
                    <Chip size="sm" style={{ background: "rgba(255,255,255,0.06)", color: "var(--yunque-text-secondary)" }}>
                      {generatedBy === "llm" ? "模型生成" : "模板兜底"}
                    </Chip>
                  </div>
                  <div className="text-xs mt-1 truncate" style={{ color: "var(--yunque-text-muted)" }}>{generateMessage || generatedWorkflow.description}</div>
                </div>
                <Button size="sm" variant="outline" onPress={() => router.push(`/workflow-editor?id=${generatedWorkflow.id}`)}>
                  查看 DAG
                </Button>
              </div>
            )}
          </div>

          <div className="p-5 space-y-3" style={{ background: "linear-gradient(180deg, rgba(139,92,246,0.10), rgba(59,130,246,0.04))", borderLeft: "1px solid var(--yunque-border)" }}>
            {[
              ["1", "理解意图", "把自然语言拆成节点与依赖"],
              ["2", "保存定义", "直接进入工作流列表与编辑器"],
              ["3", "试运行", "复用现有实例与审计记录"],
            ].map(([n, title, desc]) => (
              <div key={n} className="flex gap-3">
                <div className="w-7 h-7 rounded-full flex items-center justify-center text-xs font-bold" style={{ background: "rgba(139,92,246,0.16)", color: "#a78bfa" }}>{n}</div>
                <div>
                  <div className="text-sm font-medium" style={{ color: "var(--yunque-text)" }}>{title}</div>
                  <div className="text-xs mt-0.5" style={{ color: "var(--yunque-text-muted)" }}>{desc}</div>
                </div>
              </div>
            ))}
          </div>
        </div>
      </Card>

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
