"use client";

import { useEffect, useMemo, useState } from "react";
import { useRouter, useSearchParams } from "next/navigation";
import { Button, Card, Chip, Input, Label, Spinner, TextArea, TextField } from "@heroui/react";
import { ArrowLeft, GitBranch, Play, Save, Wand2 } from "lucide-react";
import PageHeader from "@/components/page-header";
import { showToast } from "@/components/toast-provider";
import { api, type WorkflowDef, type WorkflowEdge, type WorkflowNode, type WorkflowVariable } from "@/lib/api";
import { formatErrorMessage } from "@/lib/error-utils";

const DEFAULT_NODES: WorkflowNode[] = [
  {
    id: "start",
    name: "接收目标",
    type: "input",
    config: { source: "manual" },
    position: { x: 80, y: 80 },
  },
  {
    id: "agent",
    name: "让云雀执行",
    type: "llm",
    config: { prompt: "根据输入目标完成任务，并输出结果。" },
    position: { x: 340, y: 80 },
  },
];

const DEFAULT_EDGES: WorkflowEdge[] = [
  { id: "start-agent", from_node: "start", to_node: "agent", label: "next" },
];

function pretty(value: unknown): string {
  return JSON.stringify(value, null, 2);
}

function parseJsonArray<T>(label: string, value: string): T[] {
  const parsed = JSON.parse(value);
  if (!Array.isArray(parsed)) throw new Error(`${label} 必须是 JSON 数组`);
  return parsed as T[];
}

export default function WorkflowEditorPage() {
  const router = useRouter();
  const searchParams = useSearchParams();
  const id = searchParams.get("id") || "";
  const isEditing = id.trim().length > 0;
  const [loading, setLoading] = useState(isEditing);
  const [saving, setSaving] = useState(false);
  const [running, setRunning] = useState(false);
  const [error, setError] = useState("");
  const [workflow, setWorkflow] = useState<Partial<WorkflowDef>>({
    name: "",
    description: "",
    version: 1,
    nodes: DEFAULT_NODES,
    edges: DEFAULT_EDGES,
    variables: [],
  });
  const [nodesText, setNodesText] = useState(pretty(DEFAULT_NODES));
  const [edgesText, setEdgesText] = useState(pretty(DEFAULT_EDGES));
  const [variablesText, setVariablesText] = useState("[]");

  useEffect(() => {
    if (!isEditing) return;
    let cancelled = false;
    setLoading(true);
    api.workflowGet(id)
      .then((def) => {
        if (cancelled) return;
        setWorkflow(def);
        setNodesText(pretty(def.nodes || []));
        setEdgesText(pretty(def.edges || []));
        setVariablesText(pretty(def.variables || []));
        setError("");
      })
      .catch((e) => {
        if (!cancelled) setError(formatErrorMessage(e, "加载工作流失败"));
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [id, isEditing]);

  const stats = useMemo(() => ({
    nodes: workflow.nodes?.length || 0,
    edges: workflow.edges?.length || 0,
    variables: workflow.variables?.length || 0,
  }), [workflow]);

  const buildPayload = (): Partial<WorkflowDef> => {
    const name = String(workflow.name || "").trim();
    if (!name) throw new Error("请先填写工作流名称");
    const nodes = parseJsonArray<WorkflowNode>("节点", nodesText);
    const edges = parseJsonArray<WorkflowEdge>("连线", edgesText);
    const variables = parseJsonArray<WorkflowVariable>("变量", variablesText);
    return {
      ...workflow,
      id: workflow.id || id || undefined,
      name,
      description: String(workflow.description || "").trim(),
      version: Number(workflow.version || 1),
      nodes,
      edges,
      variables,
    };
  };

  const handleSave = async () => {
    setSaving(true);
    setError("");
    try {
      const saved = await api.workflowSave(buildPayload());
      setWorkflow(saved);
      setNodesText(pretty(saved.nodes || []));
      setEdgesText(pretty(saved.edges || []));
      setVariablesText(pretty(saved.variables || []));
      showToast("工作流已保存", "success");
      if (!isEditing && saved.id) router.replace(`/workflow-editor?id=${encodeURIComponent(saved.id)}`);
    } catch (e) {
      setError(formatErrorMessage(e, "保存失败"));
    } finally {
      setSaving(false);
    }
  };

  const handleRun = async () => {
    const workflowId = workflow.id || id;
    if (!workflowId) {
      showToast("请先保存工作流再运行", "warning");
      return;
    }
    setRunning(true);
    try {
      const res = await api.workflowRun(workflowId);
      showToast("工作流已启动", "success");
      if (res.instance_id) router.push(`/workflows`);
    } catch (e) {
      setError(formatErrorMessage(e, "运行失败"));
    } finally {
      setRunning(false);
    }
  };

  if (loading) {
    return (
      <div className="flex h-[60vh] items-center justify-center">
        <Spinner size="lg" />
      </div>
    );
  }

  return (
    <div className="page-root space-y-5 animate-fade-in-up">
      <div className="flex items-center gap-2">
        <Button size="sm" variant="ghost" onPress={() => router.push("/workflows")}>
          <ArrowLeft size={14} /> 返回工作流
        </Button>
      </div>

      <PageHeader
        icon={<GitBranch size={20} />}
        title={isEditing ? "编辑工作流" : "新建工作流"}
        description="用结构化 JSON 编辑 DAG 节点、连线和变量；保存后可立即运行。"
        actions={(
          <div className="flex items-center gap-2">
            <Button size="sm" variant="outline" isDisabled={!workflow.id && !id} isPending={running} onPress={handleRun}>
              <Play size={14} /> 运行
            </Button>
            <Button size="sm" className="btn-accent" isPending={saving} onPress={handleSave}>
              <Save size={14} /> 保存
            </Button>
          </div>
        )}
      />

      {error && (
        <div className="rounded-lg px-4 py-3 text-sm" style={{ background: "rgba(239,68,68,0.1)", color: "#ef4444" }}>
          {error}
        </div>
      )}

      <div className="grid grid-cols-1 gap-4 lg:grid-cols-[minmax(0,1fr)_320px]">
        <div className="space-y-4">
          <Card className="section-card p-4">
            <div className="grid grid-cols-1 gap-3 md:grid-cols-2">
              <TextField value={workflow.name || ""} onChange={(name: string) => setWorkflow((prev) => ({ ...prev, name }))}>
                <Label>名称</Label>
                <Input placeholder="例如：每日项目日报" />
              </TextField>
              <TextField value={String(workflow.version || 1)} onChange={(version: string) => setWorkflow((prev) => ({ ...prev, version: Number(version) || 1 }))}>
                <Label>版本</Label>
                <Input type="number" min={1} />
              </TextField>
            </div>
            <div className="mt-3">
              <TextField value={workflow.description || ""} onChange={(description: string) => setWorkflow((prev) => ({ ...prev, description }))}>
                <Label>描述</Label>
                <Input placeholder="这个工作流会做什么、产物在哪里看" />
              </TextField>
            </div>
          </Card>

          <Card className="section-card p-4">
            <div className="mb-3 flex items-center justify-between">
              <div className="text-sm font-semibold" style={{ color: "var(--yunque-text)" }}>节点 JSON</div>
              <Chip size="sm" variant="soft">{stats.nodes} 节点</Chip>
            </div>
            <TextArea aria-label="节点 JSON" rows={14} value={nodesText} onChange={(e) => setNodesText(e.target.value)} className="font-mono text-xs" />
          </Card>

          <Card className="section-card p-4">
            <div className="mb-3 flex items-center justify-between">
              <div className="text-sm font-semibold" style={{ color: "var(--yunque-text)" }}>连线 JSON</div>
              <Chip size="sm" variant="soft">{stats.edges} 条</Chip>
            </div>
            <TextArea aria-label="连线 JSON" rows={8} value={edgesText} onChange={(e) => setEdgesText(e.target.value)} className="font-mono text-xs" />
          </Card>

          <Card className="section-card p-4">
            <div className="mb-3 flex items-center justify-between">
              <div className="text-sm font-semibold" style={{ color: "var(--yunque-text)" }}>变量 JSON</div>
              <Chip size="sm" variant="soft">{stats.variables} 个</Chip>
            </div>
            <TextArea aria-label="变量 JSON" rows={6} value={variablesText} onChange={(e) => setVariablesText(e.target.value)} className="font-mono text-xs" />
          </Card>
        </div>

        <div className="space-y-4">
          <Card className="section-card p-4">
            <div className="mb-3 flex items-center gap-2 text-sm font-semibold" style={{ color: "var(--yunque-text)" }}>
              <Wand2 size={15} /> 使用方式
            </div>
            <div className="space-y-2 text-sm" style={{ color: "var(--yunque-text-secondary)" }}>
              <div>1. 填名称和描述，让工作流列表可读。</div>
              <div>2. 在节点 JSON 中编辑每一步要做什么。</div>
              <div>3. 在连线 JSON 中声明执行顺序。</div>
              <div>4. 保存后可从这里或工作流列表运行。</div>
            </div>
          </Card>

          <Card className="section-card p-4">
            <div className="text-sm font-semibold" style={{ color: "var(--yunque-text)" }}>当前结构</div>
            <div className="mt-3 grid grid-cols-3 gap-2 text-center">
              <div className="rounded-lg p-3" style={{ background: "rgba(255,255,255,0.04)" }}>
                <div className="kpi-value text-base">{stats.nodes}</div>
                <div className="kpi-label">节点</div>
              </div>
              <div className="rounded-lg p-3" style={{ background: "rgba(255,255,255,0.04)" }}>
                <div className="kpi-value text-base">{stats.edges}</div>
                <div className="kpi-label">连线</div>
              </div>
              <div className="rounded-lg p-3" style={{ background: "rgba(255,255,255,0.04)" }}>
                <div className="kpi-value text-base">{stats.variables}</div>
                <div className="kpi-label">变量</div>
              </div>
            </div>
          </Card>
        </div>
      </div>
    </div>
  );
}
