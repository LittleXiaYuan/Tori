"use client";

import { useEffect, useMemo, useState } from "react";
import { useRouter, useSearchParams } from "next/navigation";
import { Button, Card, Chip, Description, FieldError, Input, Label, ListBox, Select, Spinner, TextArea, TextField, Tooltip } from "@heroui/react";
import { ArrowDown, ArrowLeft, ArrowUp, GitBranch, Play, Plus, Save, Trash2 } from "lucide-react";
import { api, type WorkflowDef, type WorkflowNode } from "@/lib/api";
import { showToast } from "@/components/toast-provider";

type StepType = "llm" | "knowledge" | "skill" | "browser" | "transform" | "code";

interface DesignerStep {
  id: string;
  name: string;
  type: StepType;
  instruction: string;
}

const stepTypeLabels: Record<StepType, string> = {
  llm: "模型处理",
  knowledge: "检索知识",
  skill: "调用能力",
  browser: "浏览器动作",
  transform: "整理输出",
  code: "代码处理",
};

const nodeTypeLabels: Record<string, string> = {
  start: "开始",
  end: "结束",
  ...stepTypeLabels,
};

const newStep = (index: number): DesignerStep => ({
  id: `step_${index + 1}`,
  name: `步骤 ${index + 1}`,
  type: "llm",
  instruction: "",
});

function sanitizeStepID(value: string, fallback: string): string {
  const id = value
    .trim()
    .toLowerCase()
    .replace(/[^a-z0-9_]+/g, "_")
    .replace(/^_+|_+$/g, "");
  return id || fallback;
}

function nodeInstruction(node: WorkflowNode): string {
  const config = node.config || {};
  if (typeof config.user_prompt === "string") return config.user_prompt;
  if (typeof config.query === "string") return config.query;
  if (typeof config.template === "string") return config.template;
  if (typeof config.code === "string") return config.code;
  if (typeof config.target === "string") return config.target;
  if (typeof config.text === "string") return config.text;
  if (typeof config.skill_name === "string") return config.skill_name;
  return "";
}

function workflowToSteps(workflow: WorkflowDef): DesignerStep[] {
  return (workflow.nodes || [])
    .filter((node) => node.type !== "start" && node.type !== "end")
    .map((node, index) => ({
      id: sanitizeStepID(node.id, `step_${index + 1}`),
      name: node.name || `步骤 ${index + 1}`,
      type: (["llm", "knowledge", "skill", "browser", "transform", "code"].includes(node.type) ? node.type : "llm") as StepType,
      instruction: nodeInstruction(node),
    }));
}

function stepConfig(step: DesignerStep): Record<string, unknown> {
  const instruction = step.instruction.trim();
  switch (step.type) {
    case "knowledge":
      return { query: instruction, top_k: 5 };
    case "skill":
      return { skill_name: instruction || "todo_skill", args: {} };
    case "browser":
      return { action: "navigate", target: instruction };
    case "transform":
      return { template: instruction };
    case "code":
      return { language: "javascript", code: instruction };
    case "llm":
    default:
      return {
        system_prompt: "你是云雀工作流中的执行节点，请只完成当前步骤并输出可复用结果。",
        user_prompt: instruction,
      };
  }
}

function buildWorkflow(params: {
  existing?: WorkflowDef | null;
  name: string;
  description: string;
  steps: DesignerStep[];
}): Partial<WorkflowDef> {
  const usableSteps = params.steps.filter((step) => step.name.trim() || step.instruction.trim());
  const nodes: WorkflowNode[] = [
    { id: "start", name: "开始", type: "start", position: { x: 80, y: 180 }, config: {} },
    ...usableSteps.map((step, index) => ({
      id: sanitizeStepID(step.id, `step_${index + 1}`),
      name: step.name.trim() || `步骤 ${index + 1}`,
      type: step.type,
      position: { x: 300 + index * 220, y: 180 + (index % 2) * 96 },
      config: stepConfig(step),
    })),
    { id: "end", name: "结束", type: "end", position: { x: 300 + usableSteps.length * 220, y: 180 }, config: {} },
  ];
  const edges = nodes.slice(0, -1).map((node, index) => ({
    id: `edge_${node.id}_${nodes[index + 1].id}`,
    from_node: node.id,
    to_node: nodes[index + 1].id,
    label: index === 0 ? "开始" : "下一步",
  }));
  const def: Partial<WorkflowDef> = {
    name: params.name.trim(),
    description: params.description.trim(),
    version: params.existing?.version || 1,
    variables: params.existing?.variables || [],
    nodes,
    edges,
  };
  if (params.existing?.id) def.id = params.existing.id;
  if (params.existing?.tenant_id) def.tenant_id = params.existing.tenant_id;
  if (params.existing?.created_at) def.created_at = params.existing.created_at;
  if (params.existing?.updated_at) def.updated_at = params.existing.updated_at;
  return def;
}

export default function WorkflowEditorPage() {
  const router = useRouter();
  const searchParams = useSearchParams();
  const workflowID = searchParams.get("id") || "";
  const [loading, setLoading] = useState(Boolean(workflowID));
  const [saving, setSaving] = useState(false);
  const [running, setRunning] = useState(false);
  const [existing, setExisting] = useState<WorkflowDef | null>(null);
  const [name, setName] = useState("新的工作流");
  const [description, setDescription] = useState("把重复任务拆成可保存、可运行的步骤。");
  const [steps, setSteps] = useState<DesignerStep[]>([
    { ...newStep(0), name: "理解目标", instruction: "理解用户目标，列出输入、输出和验收标准。" },
    { ...newStep(1), name: "生成结果", instruction: "根据目标生成第一版可交付结果。" },
  ]);
  const [error, setError] = useState("");

  useEffect(() => {
    if (!workflowID) return;
    let cancelled = false;
    setLoading(true);
    api.workflowGet(workflowID)
      .then((workflow) => {
        if (cancelled) return;
        setExisting(workflow);
        setName(workflow.name || "未命名工作流");
        setDescription(workflow.description || "");
        const loadedSteps = workflowToSteps(workflow);
        setSteps(loadedSteps.length > 0 ? loadedSteps : [newStep(0)]);
      })
      .catch((e) => {
        if (!cancelled) showToast(e instanceof Error ? e.message : "工作流加载失败", "error");
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [workflowID]);

  const preview = useMemo(() => buildWorkflow({ existing, name, description, steps }), [description, existing, name, steps]);

  const updateStep = (index: number, patch: Partial<DesignerStep>) => {
    setSteps((prev) => prev.map((step, i) => (i === index ? { ...step, ...patch } : step)));
  };

  const moveStep = (index: number, direction: -1 | 1) => {
    setSteps((prev) => {
      const nextIndex = index + direction;
      if (nextIndex < 0 || nextIndex >= prev.length) return prev;
      const next = [...prev];
      [next[index], next[nextIndex]] = [next[nextIndex], next[index]];
      return next;
    });
  };

  const saveWorkflow = async () => {
    if (!name.trim()) {
      setError("工作流名称不能为空");
      return;
    }
    if (steps.filter((step) => step.instruction.trim()).length === 0) {
      setError("至少写一个可执行步骤");
      return;
    }
    setError("");
    setSaving(true);
    try {
      const saved = await api.workflowSave(preview);
      setExisting(saved);
      showToast("工作流已保存", "success");
      if (!workflowID) router.replace(`/workflow-editor?id=${encodeURIComponent(saved.id)}`);
    } catch (e) {
      showToast(e instanceof Error ? e.message : "保存失败", "error");
    } finally {
      setSaving(false);
    }
  };

  const runWorkflow = async () => {
    const id = existing?.id;
    if (!id) {
      showToast("请先保存工作流，再运行", "warning");
      return;
    }
    setRunning(true);
    try {
      await api.workflowRun(id);
      showToast("工作流已启动", "success");
      router.push("/workflows");
    } catch (e) {
      showToast(e instanceof Error ? e.message : "启动失败", "error");
    } finally {
      setRunning(false);
    }
  };

  if (loading) {
    return <div className="flex h-[60vh] items-center justify-center"><Spinner size="lg" /></div>;
  }

  return (
    <section className="desktop-page-scroll page-root animate-fade-in-up space-y-5" aria-labelledby="workflow-editor-title">
      <header className="flex flex-wrap items-center justify-between gap-3">
        <div className="flex min-w-0 items-center gap-3">
          <Tooltip delay={0}>
            <Button isIconOnly aria-label="返回工作流列表" variant="ghost" onPress={() => router.push("/workflows")}>
              <ArrowLeft size={16} />
            </Button>
            <Tooltip.Content>返回</Tooltip.Content>
          </Tooltip>
          <div className="min-w-0">
            <h1 id="workflow-editor-title" className="page-title flex items-center gap-2">
              <GitBranch size={20} /> 工作流设计
            </h1>
            <p className="mt-1 text-sm" style={{ color: "var(--yunque-text-muted)" }}>
              用步骤搭出可保存、可运行的流程。先把常用重复工作整理成清晰顺序。
            </p>
          </div>
        </div>
        <div className="flex flex-wrap gap-2">
          <Button variant="secondary" onPress={() => router.push("/workflows")}>查看列表</Button>
          <Button variant="outline" isPending={running} onPress={runWorkflow}>
            <Play size={14} /> 运行
          </Button>
          <Button className="btn-accent" isPending={saving} onPress={saveWorkflow}>
            <Save size={14} /> 保存
          </Button>
        </div>
      </header>

      {error && (
        <div role="status" className="rounded-lg px-4 py-3 text-sm" style={{ border: "1px solid rgba(239,68,68,0.25)", color: "#ef4444" }}>
          {error}
        </div>
      )}

      <div className="grid grid-cols-1 gap-5 xl:grid-cols-[minmax(0,1fr)_320px]">
        <section className="space-y-4" aria-labelledby="workflow-basics-heading">
          <Card className="section-card">
            <Card.Header>
              <Card.Title id="workflow-basics-heading">基础信息</Card.Title>
              <Card.Description>给流程一个清晰目标，方便后续从任务中心或工作流列表找到它。</Card.Description>
            </Card.Header>
            <Card.Content className="grid grid-cols-1 gap-4 md:grid-cols-2">
              <TextField fullWidth isRequired isInvalid={Boolean(error && !name.trim())} value={name} onChange={setName}>
                <Label>工作流名称</Label>
                <Input placeholder="例如：周报自动生成" />
                <FieldError>工作流名称不能为空</FieldError>
              </TextField>
              <TextField fullWidth value={description} onChange={setDescription}>
                <Label>说明</Label>
                <Input placeholder="它解决什么重复工作？" />
                <Description>一句话说明输入、输出或使用场景。</Description>
              </TextField>
            </Card.Content>
          </Card>

          <section className="space-y-3" aria-labelledby="workflow-steps-heading">
            <div className="flex flex-wrap items-center justify-between gap-3">
              <div>
                <h2 id="workflow-steps-heading" className="text-base font-semibold">执行步骤</h2>
                <p className="mt-1 text-sm" style={{ color: "var(--yunque-text-muted)" }}>按顺序执行。每一步都会保存成后端工作流节点。</p>
              </div>
              <Button variant="secondary" onPress={() => setSteps((prev) => [...prev, newStep(prev.length)])}>
                <Plus size={14} /> 添加步骤
              </Button>
            </div>

            {steps.map((step, index) => (
              <Card key={`${step.id}-${index}`} className="section-card">
                <Card.Content className="space-y-4 p-4">
                  <div className="flex flex-wrap items-start justify-between gap-3">
                    <div className="flex items-center gap-2">
                      <Chip size="sm">{index + 1}</Chip>
                      <Chip size="sm" style={{ background: "rgba(139,92,246,0.12)", color: "#a78bfa" }}>
                        {stepTypeLabels[step.type]}
                      </Chip>
                    </div>
                    <div className="flex gap-1">
                      <Tooltip delay={0}>
                        <Button isIconOnly aria-label={`上移 ${step.name || `步骤 ${index + 1}`}`} variant="ghost" size="sm" isDisabled={index === 0} onPress={() => moveStep(index, -1)}>
                          <ArrowUp size={14} />
                        </Button>
                        <Tooltip.Content>上移</Tooltip.Content>
                      </Tooltip>
                      <Tooltip delay={0}>
                        <Button isIconOnly aria-label={`下移 ${step.name || `步骤 ${index + 1}`}`} variant="ghost" size="sm" isDisabled={index === steps.length - 1} onPress={() => moveStep(index, 1)}>
                          <ArrowDown size={14} />
                        </Button>
                        <Tooltip.Content>下移</Tooltip.Content>
                      </Tooltip>
                      <Tooltip delay={0}>
                        <Button isIconOnly aria-label={`删除 ${step.name || `步骤 ${index + 1}`}`} variant="danger" size="sm" isDisabled={steps.length <= 1} onPress={() => setSteps((prev) => prev.filter((_, i) => i !== index))}>
                          <Trash2 size={14} />
                        </Button>
                        <Tooltip.Content>删除</Tooltip.Content>
                      </Tooltip>
                    </div>
                  </div>

                  <div className="grid grid-cols-1 gap-4 md:grid-cols-[minmax(0,1fr)_180px]">
                    <TextField fullWidth value={step.name} onChange={(value) => updateStep(index, { name: value })}>
                      <Label>步骤名称</Label>
                      <Input placeholder="例如：检索资料" />
                    </TextField>
                    <Select
                      selectedKey={step.type}
                      onSelectionChange={(key) => updateStep(index, { type: key as StepType })}
                      fullWidth
                    >
                      <Label>步骤类型</Label>
                      <Select.Trigger>
                        <Select.Value />
                        <Select.Indicator />
                      </Select.Trigger>
                      <Select.Popover>
                        <ListBox>
                          {Object.entries(stepTypeLabels).map(([value, label]) => (
                            <ListBox.Item key={value} id={value} textValue={label}>
                              {label}
                              <ListBox.ItemIndicator />
                            </ListBox.Item>
                          ))}
                        </ListBox>
                      </Select.Popover>
                      <Description>决定这一步交给模型、知识库、能力或浏览器来完成。</Description>
                    </Select>
                  </div>

                  <TextField fullWidth value={step.instruction} onChange={(value) => updateStep(index, { instruction: value })}>
                    <Label>{step.type === "skill" ? "能力名称" : step.type === "browser" ? "目标 URL 或动作说明" : "执行说明"}</Label>
                    <TextArea rows={3} placeholder="写清楚这一步要做什么、需要哪些输入、输出什么。" />
                    <Description>{step.type === "llm" ? "云雀会按这段说明完成当前步骤。" : "保存后会成为流程中的一个可执行步骤。"}</Description>
                  </TextField>
                </Card.Content>
              </Card>
            ))}
          </section>
        </section>

        <aside className="space-y-4" aria-labelledby="workflow-preview-heading">
          <Card className="section-card sticky top-4">
            <Card.Header>
              <Card.Title id="workflow-preview-heading">保存预览</Card.Title>
              <Card.Description>确认云雀将保存的执行链路。</Card.Description>
            </Card.Header>
            <Card.Content className="space-y-4">
              <div className="space-y-2">
                {(preview.nodes || []).map((node, index) => (
                  <div key={node.id} className="flex items-center gap-2 text-sm">
                    <span className="flex h-6 w-6 shrink-0 items-center justify-center rounded-full text-xs" style={{ background: "rgba(255,255,255,0.06)" }}>{index + 1}</span>
                    <span className="min-w-0 flex-1 truncate">{node.name}</span>
                    <Chip size="sm">{nodeTypeLabels[node.type] || "步骤"}</Chip>
                  </div>
                ))}
              </div>
              <div className="rounded-lg p-3 text-xs" style={{ background: "rgba(255,255,255,0.04)", color: "var(--yunque-text-muted)" }}>
                保存后可在「工作流」列表运行；运行记录会回到同一页面查看。当前设计器先覆盖最常用的顺序流程。
              </div>
            </Card.Content>
          </Card>
        </aside>
      </div>
    </section>
  );
}
