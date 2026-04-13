"use client";

import { Suspense, useEffect, useState, useCallback, useRef } from "react";
import { useSearchParams, useRouter } from "next/navigation";
import { api } from "@/lib/api";
import { showToast } from "@/components/toast-provider";
import { Button, Spinner, Select, ListBox, TextField, Input, Label, TextArea } from "@heroui/react";
import { ArrowLeft, Save, X, Trash2, ZoomIn, ZoomOut, RotateCcw } from "lucide-react";

/* ---------- types ---------- */
interface DefNode { id: string; name: string; type: string; config?: Record<string, unknown>; position: { x: number; y: number }; }
interface DefEdge { id: string; from_node: string; to_node: string; condition?: string; label?: string; }
interface WorkflowDef { id: string; name: string; description: string; version: number; nodes: DefNode[]; edges: DefEdge[]; tenant_id: string; }

/* ---------- palette: 14 node types ---------- */
const NODE_TYPES = [
  { type: "start",     label: "开始",     color: "#22c55e", icon: "▶" },
  { type: "end",       label: "结束",     color: "#ef4444", icon: "■" },
  { type: "llm",       label: "LLM",      color: "#8b5cf6", icon: "" },
  { type: "skill",     label: "技能",     color: "#3b82f6", icon: "[FIX]" },
  { type: "condition", label: "条件",     color: "#f59e0b", icon: "⑂" },
  { type: "parallel",  label: "并行",     color: "#22c55e", icon: "⫸" },
  { type: "join",      label: "汇合",     color: "#14b8a6", icon: "⫷" },
  { type: "input",     label: "用户输入", color: "#f97316", icon: "" },
  { type: "transform", label: "数据转换", color: "#06b6d4", icon: "↻" },
  { type: "code",      label: "代码",     color: "#84cc16", icon: "{ }" },
  { type: "knowledge", label: "知识检索", color: "#f43f5e", icon: "" },
  { type: "browser",   label: "浏览器",   color: "#6366f1", icon: "" },
  { type: "subflow",   label: "子工作流", color: "#ec4899", icon: "[PKG]" },
  { type: "loop",      label: "循环",     color: "#06b6d4", icon: "[RETRY]" },
] as const;

const typeMap = Object.fromEntries(NODE_TYPES.map(n => [n.type, n]));
function typeColor(t: string) { return typeMap[t]?.color || "#9ca3af"; }
function typeIcon(t: string) { return typeMap[t]?.icon || "●"; }
function typeLabel(t: string) { return typeMap[t]?.label || t; }

const NODE_W = 180;
const NODE_H = 56;

function WorkflowEditorContent() {
  const searchParams = useSearchParams();
  const router = useRouter();
  const id = searchParams.get("id") || "";

  const [workflow, setWorkflow] = useState<WorkflowDef | null>(null);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);

  const [pan, setPan] = useState({ x: 40, y: 40 });
  const [zoom, setZoom] = useState(1);
  const [draggingNode, setDraggingNode] = useState<string | null>(null);
  const [dragOffset, setDragOffset] = useState({ x: 0, y: 0 });
  const [selectedNode, setSelectedNode] = useState<string | null>(null);
  const [connecting, setConnecting] = useState<string | null>(null);
  const [mousePos, setMousePos] = useState({ x: 0, y: 0 });
  const svgRef = useRef<SVGSVGElement>(null);

  const [isPanning, setIsPanning] = useState(false);
  const [panStart, setPanStart] = useState({ x: 0, y: 0 });

  const fetchWorkflow = useCallback(async () => {
    if (!id) {
      // New workflow — initialize a blank template with start + end nodes
      setWorkflow({
        id: `wf_${Date.now()}`,
        name: "新工作流",
        description: "",
        version: 0,
        nodes: [
          { id: "start", name: "开始", type: "start", position: { x: 300, y: 60 } },
          { id: "end",   name: "结束", type: "end",   position: { x: 300, y: 400 } },
        ],
        edges: [],
        tenant_id: "",
      });
      setLoading(false);
      return;
    }
    try {
      const data = await api.workflowList();
      const wf = data.workflows?.find((w: WorkflowDef) => w.id === id);
      if (wf) {
        wf.nodes = (wf.nodes || []).map((n: DefNode, i: number) => ({
          ...n, position: n.position || { x: 100 + (i % 4) * 220, y: 100 + Math.floor(i / 4) * 120 },
        }));
        wf.edges = wf.edges || [];
        setWorkflow(wf);
      }
    } catch { /* ignore */ }
    finally { setLoading(false); }
  }, [id]);

  useEffect(() => { fetchWorkflow(); }, [fetchWorkflow]);

  const saveWorkflow = async () => {
    if (!workflow) return;
    setSaving(true);
    try { await api.workflowSave(workflow); showToast("已保存", "success"); } catch (e) { showToast(e instanceof Error ? e.message : "保存失败", "error"); }
    finally { setSaving(false); }
  };

  const svgPoint = useCallback((clientX: number, clientY: number) => {
    const rect = svgRef.current?.getBoundingClientRect();
    if (!rect) return { x: 0, y: 0 };
    return { x: (clientX - rect.left - pan.x) / zoom, y: (clientY - rect.top - pan.y) / zoom };
  }, [pan, zoom]);

  const addNode = (type: string, x?: number, y?: number) => {
    if (!workflow) return;
    const px = x ?? 200 + Math.random() * 200;
    const py = y ?? 100 + workflow.nodes.length * 80;
    const newNode: DefNode = { id: `node_${Date.now()}`, name: typeLabel(type), type, position: { x: px, y: py } };
    setWorkflow({ ...workflow, nodes: [...workflow.nodes, newNode] });
    setSelectedNode(newNode.id);
  };

  const removeNode = (nodeId: string) => {
    if (!workflow) return;
    setWorkflow({
      ...workflow,
      nodes: workflow.nodes.filter(n => n.id !== nodeId),
      edges: workflow.edges.filter(e => e.from_node !== nodeId && e.to_node !== nodeId),
    });
    if (selectedNode === nodeId) setSelectedNode(null);
  };

  const updateNode = (nodeId: string, patch: Partial<DefNode>) => {
    if (!workflow) return;
    setWorkflow({ ...workflow, nodes: workflow.nodes.map(n => n.id === nodeId ? { ...n, ...patch } : n) });
  };

  const removeEdge = (edgeId: string) => {
    if (!workflow) return;
    setWorkflow({ ...workflow, edges: workflow.edges.filter(e => e.id !== edgeId) });
  };

  const handleWheel = useCallback((e: React.WheelEvent) => {
    e.preventDefault();
    setZoom(z => Math.min(3, Math.max(0.2, z * (e.deltaY > 0 ? 0.9 : 1.1))));
  }, []);

  const handleMouseDown = (e: React.MouseEvent) => {
    if (connecting) { setConnecting(null); return; }
    if (e.button === 1 || (e.button === 0 && e.altKey)) {
      setIsPanning(true);
      setPanStart({ x: e.clientX - pan.x, y: e.clientY - pan.y });
    }
  };

  const handleMouseMove = useCallback((e: React.MouseEvent) => {
    setMousePos({ x: e.clientX, y: e.clientY });
    if (isPanning) {
      setPan({ x: e.clientX - panStart.x, y: e.clientY - panStart.y });
      return;
    }
    if (draggingNode && workflow) {
      const p = svgPoint(e.clientX, e.clientY);
      setWorkflow(prev => prev ? {
        ...prev,
        nodes: prev.nodes.map(n => n.id === draggingNode ? { ...n, position: { x: p.x - dragOffset.x, y: p.y - dragOffset.y } } : n),
      } : prev);
    }
  }, [isPanning, panStart, draggingNode, workflow, svgPoint, dragOffset]);

  const handleMouseUp = useCallback(() => {
    setIsPanning(false);
    setDraggingNode(null);
  }, []);

  const startNodeDrag = (nodeId: string, e: React.MouseEvent) => {
    e.stopPropagation();
    const node = workflow?.nodes.find(n => n.id === nodeId);
    if (!node) return;
    const p = svgPoint(e.clientX, e.clientY);
    setDragOffset({ x: p.x - node.position.x, y: p.y - node.position.y });
    setDraggingNode(nodeId);
    setSelectedNode(nodeId);
  };

  const startConnect = (nodeId: string, e: React.MouseEvent) => {
    e.stopPropagation();
    setConnecting(nodeId);
  };

  const finishConnect = (nodeId: string) => {
    if (connecting && connecting !== nodeId && workflow) {
      if (!workflow.edges.some(e => e.from_node === connecting && e.to_node === nodeId)) {
        setWorkflow({ ...workflow, edges: [...workflow.edges, { id: `edge_${Date.now()}`, from_node: connecting, to_node: nodeId }] });
      }
    }
    setConnecting(null);
  };

  const handleDrop = (e: React.DragEvent) => {
    e.preventDefault();
    const type = e.dataTransfer.getData("text/plain");
    if (!type) return;
    const p = svgPoint(e.clientX, e.clientY);
    addNode(type, p.x - NODE_W / 2, p.y - NODE_H / 2);
  };

  const edgePath = (from: DefNode, to: DefNode) => {
    const sx = from.position.x + NODE_W / 2, sy = from.position.y + NODE_H;
    const ex = to.position.x + NODE_W / 2, ey = to.position.y;
    const dy = Math.abs(ey - sy) / 2;
    return `M ${sx} ${sy} C ${sx} ${sy + dy}, ${ex} ${ey - dy}, ${ex} ${ey}`;
  };

  const selNode = workflow?.nodes.find(n => n.id === selectedNode);

  if (loading) return <div className="flex items-center justify-center h-[80vh]"><Spinner size="lg" /></div>;

  if (!workflow) {
    return (
      <div className="flex flex-col items-center justify-center h-[60vh] gap-4" style={{ color: "var(--yunque-text-muted)" }}>
        <svg className="w-12 h-12 opacity-30" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={1.5}><path d="M7.5 21L3 16.5m0 0L7.5 12M3 16.5h13.5m0-13.5L21 7.5m0 0L16.5 12M21 7.5H7.5" /></svg>
        <p>工作流未找到</p>
        <Button variant="ghost" onPress={() => router.push("/workflows")}>返回列表</Button>
      </div>
    );
  }

  return (
    <div className="flex h-[calc(100vh-56px)] overflow-hidden" style={{ background: "var(--yunque-bg)" }}>
      {/* Left: Palette */}
      <div className="w-48 border-r overflow-y-auto shrink-0 flex flex-col" style={{ background: "var(--yunque-card)", borderColor: "var(--yunque-border)" }}>
        <div className="p-3 border-b" style={{ borderColor: "var(--yunque-border)" }}>
          <div className="text-xs font-semibold uppercase tracking-wider" style={{ color: "var(--yunque-text-muted)" }}>节点库</div>
        </div>
        <div className="p-2 space-y-1 flex-1">
          {NODE_TYPES.map(nt => (
            <div key={nt.type} draggable
              onDragStart={(e) => { e.dataTransfer.setData("text/plain", nt.type); e.dataTransfer.effectAllowed = "move"; }}
              className="flex items-center gap-2 p-2 rounded-lg text-xs cursor-grab active:cursor-grabbing transition-colors select-none"
              style={{ color: "var(--yunque-text)" }}>
              <div className="w-6 h-6 rounded flex items-center justify-center text-white text-[10px] shrink-0" style={{ background: nt.color }}>{nt.icon}</div>
              <span>{nt.label}</span>
            </div>
          ))}
        </div>
        <div className="p-3 border-t" style={{ borderColor: "var(--yunque-border)" }}>
          <div className="text-[10px] leading-relaxed" style={{ color: "var(--yunque-text-muted)" }}>拖拽到画布 · Alt+拖拽平移 · 滚轮缩放 · 拖出端口连线</div>
        </div>
      </div>

      {/* Center: Canvas */}
      <div className="flex-1 relative"
        style={{ background: "var(--yunque-bg)" }}
        onDrop={handleDrop} onDragOver={(e) => { e.preventDefault(); e.dataTransfer.dropEffect = "move"; }}>
        {/* Toolbar */}
        <div className="absolute top-3 left-3 z-10 flex items-center gap-2">
          <Button size="sm" variant="ghost" onPress={() => router.push("/missions")}
            style={{ background: "var(--yunque-card)", border: "1px solid var(--yunque-border)", color: "var(--yunque-text)" }}>
            <ArrowLeft size={14} />
            返回
          </Button>
          <div className="px-3 py-1.5 rounded-lg text-xs"
            style={{ background: "var(--yunque-card)", border: "1px solid var(--yunque-border)" }}>
            <span className="font-medium" style={{ color: "var(--yunque-text)" }}>{workflow.name}</span>
            <span className="ml-2" style={{ color: "var(--yunque-text-muted)" }}>v{workflow.version} · {workflow.nodes.length}节点 · {workflow.edges.length}连线</span>
          </div>
        </div>
        <div className="absolute top-3 right-3 z-10 flex items-center gap-2">
          <Button isIconOnly aria-label="放大" size="sm" variant="ghost" onPress={() => setZoom(z => Math.min(3, z * 1.2))}
            style={{ background: "var(--yunque-card)", border: "1px solid var(--yunque-border)", color: "var(--yunque-text)" }}><ZoomIn size={14} /></Button>
          <span className="text-xs w-10 text-center" style={{ color: "var(--yunque-text-muted)" }}>{Math.round(zoom * 100)}%</span>
          <Button isIconOnly aria-label="缩小" size="sm" variant="ghost" onPress={() => setZoom(z => Math.max(0.2, z * 0.8))}
            style={{ background: "var(--yunque-card)", border: "1px solid var(--yunque-border)", color: "var(--yunque-text)" }}><ZoomOut size={14} /></Button>
          <Button size="sm" variant="ghost" onPress={() => { setPan({ x: 40, y: 40 }); setZoom(1); }}
            style={{ background: "var(--yunque-card)", border: "1px solid var(--yunque-border)", color: "var(--yunque-text)" }}><RotateCcw size={14} /> 重置</Button>
          <Button size="sm" isPending={saving} onPress={saveWorkflow} className="btn-accent">
            <Save size={14} />
            {saving ? "保存中..." : "保存"}
          </Button>
        </div>

        <svg ref={svgRef} className="w-full h-full"
          onWheel={handleWheel} onMouseDown={handleMouseDown} onMouseMove={handleMouseMove}
          onMouseUp={handleMouseUp} onMouseLeave={handleMouseUp}
          style={{ cursor: isPanning ? "grabbing" : draggingNode ? "grabbing" : "default" }}>
          <defs>
            <pattern id="grid" width={20 * zoom} height={20 * zoom} patternUnits="userSpaceOnUse" x={pan.x % (20 * zoom)} y={pan.y % (20 * zoom)}>
              <circle cx={1} cy={1} r={0.5} fill="rgba(255,255,255,0.08)" />
            </pattern>
            <marker id="ah" markerWidth={10} markerHeight={7} refX={10} refY={3.5} orient="auto">
              <polygon points="0 0, 10 3.5, 0 7" fill="rgba(255,255,255,0.25)" />
            </marker>
          </defs>
          <rect width="100%" height="100%" fill="url(#grid)" />
          <g transform={`translate(${pan.x}, ${pan.y}) scale(${zoom})`}>
            {/* Edges */}
            {workflow.edges.map(edge => {
              const from = workflow.nodes.find(n => n.id === edge.from_node);
              const to = workflow.nodes.find(n => n.id === edge.to_node);
              if (!from || !to) return null;
              const d = edgePath(from, to);
              return (
                <g key={edge.id} className="cursor-pointer" onClick={() => removeEdge(edge.id)}>
                  <path d={d} fill="none" stroke="transparent" strokeWidth={12} />
                  <path d={d} fill="none" stroke={typeColor(from.type)} strokeWidth={2} strokeOpacity={0.4} markerEnd="url(#ah)" />
                  {edge.condition && (
                    <text x={(from.position.x + to.position.x + NODE_W) / 2} y={(from.position.y + NODE_H + to.position.y) / 2 - 4}
                      fontSize={10} fill="rgba(255,255,255,0.45)" textAnchor="middle">{edge.condition}</text>
                  )}
                </g>
              );
            })}
            {/* Connecting line */}
            {connecting && (() => {
              const from = workflow.nodes.find(n => n.id === connecting);
              if (!from) return null;
              const mp = svgPoint(mousePos.x, mousePos.y);
              return <line x1={from.position.x + NODE_W / 2} y1={from.position.y + NODE_H} x2={mp.x} y2={mp.y}
                stroke="#3b82f6" strokeWidth={2} strokeDasharray="6 3" strokeOpacity={0.6} />;
            })()}
            {/* Nodes */}
            {workflow.nodes.map(node => {
              const c = typeColor(node.type);
              const isSel = selectedNode === node.id;
              return (
                <g key={node.id} transform={`translate(${node.position.x}, ${node.position.y})`}>
                  <circle cx={NODE_W / 2} cy={0} r={5} fill={c} fillOpacity={0.2} stroke={c} strokeWidth={1.5}
                    className="cursor-pointer" onMouseUp={() => finishConnect(node.id)} />
                  <rect width={NODE_W} height={NODE_H} rx={10} ry={10}
                    fill="var(--yunque-card, #1a1a2e)" stroke={isSel ? c : "rgba(255,255,255,0.1)"}
                    strokeWidth={isSel ? 2 : 1} className="cursor-move"
                    filter={isSel ? `drop-shadow(0 0 6px ${c}40)` : undefined}
                    onMouseDown={(e) => startNodeDrag(node.id, e)}
                    onClick={(e) => { e.stopPropagation(); setSelectedNode(node.id); }}
                  />
                  <rect x={0} y={8} width={4} height={NODE_H - 16} rx={2} fill={c} />
                  <text x={20} y={NODE_H / 2 + 1} fontSize={16} textAnchor="middle" dominantBaseline="central">{typeIcon(node.type)}</text>
                  <text x={36} y={NODE_H / 2 - 6} fontSize={12} fontWeight={500} fill="rgba(255,255,255,0.9)" className="pointer-events-none">
                    {node.name.length > 12 ? node.name.slice(0, 12) + "…" : node.name}
                  </text>
                  <text x={36} y={NODE_H / 2 + 10} fontSize={10} fill="rgba(255,255,255,0.45)" className="pointer-events-none">{typeLabel(node.type)}</text>
                  <circle cx={NODE_W / 2} cy={NODE_H} r={5} fill={c} fillOpacity={0.3} stroke={c} strokeWidth={1.5}
                    className="cursor-crosshair" onMouseDown={(e) => startConnect(node.id, e)} />
                  {isSel && (
                    <g transform={`translate(${NODE_W - 8}, -8)`} className="cursor-pointer" onClick={(e) => { e.stopPropagation(); removeNode(node.id); }}>
                      <circle r={10} fill="#ef4444" fillOpacity={0.9} />
                      <text fontSize={12} fill="white" textAnchor="middle" dominantBaseline="central">×</text>
                    </g>
                  )}
                </g>
              );
            })}
          </g>
        </svg>
      </div>

      {/* Right: Property Panel */}
      {selNode && (
        <div className="w-72 border-l overflow-y-auto shrink-0" style={{ background: "var(--yunque-card)", borderColor: "var(--yunque-border)" }}>
          <div className="p-4 border-b flex items-center justify-between" style={{ borderColor: "var(--yunque-border)" }}>
            <div className="text-sm font-semibold" style={{ color: "var(--yunque-text)" }}>节点属性</div>
            <Button isIconOnly aria-label="关闭" size="sm" variant="ghost" onPress={() => setSelectedNode(null)}>
              <X size={16} />
            </Button>
          </div>
          <div className="p-4 space-y-4">
            <div>
              <TextField>
                <Label>名称</Label>
                <Input value={selNode.name} onChange={(e) => updateNode(selNode.id, { name: e.target.value })} />
              </TextField>
            </div>
            <div>
              <Select selectedKey={selNode.type} onSelectionChange={(k) => updateNode(selNode.id, { type: String(k) })} aria-label="类型">
                <Select.Trigger><Select.Value /><Select.Indicator /></Select.Trigger>
                <Select.Popover>
                  <ListBox>
                    {NODE_TYPES.map(nt => <ListBox.Item key={nt.type} id={nt.type} textValue={`${nt.label} (${nt.type})`}>{nt.label} ({nt.type})</ListBox.Item>)}
                  </ListBox>
                </Select.Popover>
              </Select>
            </div>

            {/* Type-specific config */}
            {selNode.type === "llm" && (<>
              <TextField>
                <Label>模型</Label>
                <Input value={(selNode.config?.model as string) || ""} onChange={(e) => updateNode(selNode.id, { config: { ...selNode.config, model: e.target.value } })} placeholder="gpt-4o" />
              </TextField>
              <TextField>
                <Label>系统提示</Label>
                <TextArea value={(selNode.config?.system_prompt as string) || ""} onChange={(e) => updateNode(selNode.id, { config: { ...selNode.config, system_prompt: e.target.value } })} rows={3} />
              </TextField>
              <TextField>
                <Label>用户提示</Label>
                <TextArea value={(selNode.config?.user_prompt as string) || ""} onChange={(e) => updateNode(selNode.id, { config: { ...selNode.config, user_prompt: e.target.value } })} rows={3} />
              </TextField>
              <TextField>
                <Label>温度</Label>
                <Input type="number" step={0.1} min={0} max={2} value={String((selNode.config?.temperature as number) ?? 0.7)}
                  onChange={(e) => updateNode(selNode.id, { config: { ...selNode.config, temperature: parseFloat(e.target.value) } })} />
              </TextField>
            </>)}
            {selNode.type === "skill" && (
              <TextField>
                <Label>技能名称</Label>
                <Input value={(selNode.config?.skill_name as string) || ""} onChange={(e) => updateNode(selNode.id, { config: { ...selNode.config, skill_name: e.target.value } })} placeholder="web_search" />
              </TextField>
            )}
            {selNode.type === "condition" && (<>
              <TextField>
                <Label>变量</Label>
                <Input value={(selNode.config?.variable as string) || ""} onChange={(e) => updateNode(selNode.id, { config: { ...selNode.config, variable: e.target.value } })} placeholder="{{result}}" />
              </TextField>
              <Select selectedKey={(selNode.config?.operator as string) || "eq"} onSelectionChange={(k) => updateNode(selNode.id, { config: { ...selNode.config, operator: String(k) } })} aria-label="运算符">
                <Select.Trigger><Select.Value /><Select.Indicator /></Select.Trigger>
                <Select.Popover>
                  <ListBox>
                    {[["eq","等于"],["neq","不等于"],["gt","大于"],["lt","小于"],["contains","包含"],["is_true","为真"]].map(([v,l]) => (
                      <ListBox.Item key={v} id={v} textValue={l}>{l}</ListBox.Item>
                    ))}
                  </ListBox>
                </Select.Popover>
              </Select>
              <TextField>
                <Label>比较值</Label>
                <Input value={(selNode.config?.value as string) || ""} onChange={(e) => updateNode(selNode.id, { config: { ...selNode.config, value: e.target.value } })} />
              </TextField>
            </>)}
            {selNode.type === "transform" && (
              <TextField>
                <Label>模板</Label>
                <TextArea value={(selNode.config?.template as string) || ""} onChange={(e) => updateNode(selNode.id, { config: { ...selNode.config, template: e.target.value } })}
                  rows={4} placeholder="Go template {{.input}}" className="font-mono" />
              </TextField>
            )}
            {selNode.type === "code" && (<>
              <Select selectedKey={(selNode.config?.language as string) || "javascript"} onSelectionChange={(k) => updateNode(selNode.id, { config: { ...selNode.config, language: String(k) } })} aria-label="语言">
                <Select.Trigger><Select.Value /><Select.Indicator /></Select.Trigger>
                <Select.Popover>
                  <ListBox>
                    <ListBox.Item id="javascript" textValue="JavaScript">JavaScript</ListBox.Item>
                    <ListBox.Item id="python" textValue="Python">Python</ListBox.Item>
                  </ListBox>
                </Select.Popover>
              </Select>
              <TextField>
                <Label>代码</Label>
                <TextArea value={(selNode.config?.code as string) || ""} onChange={(e) => updateNode(selNode.id, { config: { ...selNode.config, code: e.target.value } })}
                  rows={6} className="text-xs font-mono" />
              </TextField>
            </>)}
            {selNode.type === "knowledge" && (
              <TextField>
                <Label>查询</Label>
                <TextArea value={(selNode.config?.query as string) || ""} onChange={(e) => updateNode(selNode.id, { config: { ...selNode.config, query: e.target.value } })} rows={3} />
              </TextField>
            )}
            {selNode.type === "browser" && (<>
              <Select selectedKey={(selNode.config?.action as string) || "navigate"} onSelectionChange={(k) => updateNode(selNode.id, { config: { ...selNode.config, action: String(k) } })} aria-label="动作">
                <Select.Trigger><Select.Value /><Select.Indicator /></Select.Trigger>
                <Select.Popover>
                  <ListBox>
                    {[["navigate","导航"],["click","点击"],["type","输入"],["screenshot","截图"],["extract","提取"]].map(([v,l]) => (
                      <ListBox.Item key={v} id={v} textValue={l}>{l}</ListBox.Item>
                    ))}
                  </ListBox>
                </Select.Popover>
              </Select>
              <TextField>
                <Label>URL / 选择器</Label>
                <Input value={(selNode.config?.target as string) || ""} onChange={(e) => updateNode(selNode.id, { config: { ...selNode.config, target: e.target.value } })} />
              </TextField>
            </>)}
            {selNode.type === "input" && (
              <TextField>
                <Label>提示文本</Label>
                <Input value={(selNode.config?.prompt as string) || ""} onChange={(e) => updateNode(selNode.id, { config: { ...selNode.config, prompt: e.target.value } })} />
              </TextField>
            )}
            {selNode.type === "subflow" && (
              <TextField>
                <Label>子工作流 ID</Label>
                <Input value={(selNode.config?.definition_id as string) || ""} onChange={(e) => updateNode(selNode.id, { config: { ...selNode.config, definition_id: e.target.value } })} />
              </TextField>
            )}
            {selNode.type === "loop" && (<>
              <TextField>
                <Label>最大迭代次数</Label>
                <Input type="number" min={1} value={String((selNode.config?.max_iterations as number) ?? 10)}
                  onChange={(e) => updateNode(selNode.id, { config: { ...selNode.config, max_iterations: parseInt(e.target.value) } })} />
              </TextField>
              <TextField>
                <Label>退出条件</Label>
                <Input value={(selNode.config?.exit_condition as string) || ""} onChange={(e) => updateNode(selNode.id, { config: { ...selNode.config, exit_condition: e.target.value } })} />
              </TextField>
            </>)}

            <div className="border-t pt-4" style={{ borderColor: "var(--yunque-border)" }}>
              <label className="text-xs mb-1 block" style={{ color: "var(--yunque-text-muted)" }}>位置</label>
              <div className="grid grid-cols-2 gap-2">
                <TextField>
                  <Label>X</Label>
                  <Input type="number" value={String(Math.round(selNode.position.x))} onChange={(e) => updateNode(selNode.id, { position: { ...selNode.position, x: parseInt(e.target.value) || 0 } })} />
                </TextField>
                <TextField>
                  <Label>Y</Label>
                  <Input type="number" value={String(Math.round(selNode.position.y))} onChange={(e) => updateNode(selNode.id, { position: { ...selNode.position, y: parseInt(e.target.value) || 0 } })} />
                </TextField>
              </div>
            </div>
            <div><label className="text-xs mb-1 block" style={{ color: "var(--yunque-text-muted)" }}>节点 ID</label>
              <div className="text-xs font-mono truncate" style={{ color: "var(--yunque-text-muted)" }}>{selNode.id}</div></div>
            <div className="border-t pt-4" style={{ borderColor: "var(--yunque-border)" }}>
              <label className="text-xs mb-2 block" style={{ color: "var(--yunque-text-muted)" }}>连线</label>
              {workflow.edges.filter(e => e.from_node === selNode.id || e.to_node === selNode.id).length === 0
                ? <div className="text-xs" style={{ color: "var(--yunque-text-muted)" }}>无连线</div>
                : <div className="space-y-1">{workflow.edges.filter(e => e.from_node === selNode.id || e.to_node === selNode.id).map(edge => {
                    const other = edge.from_node === selNode.id ? workflow.nodes.find(n => n.id === edge.to_node) : workflow.nodes.find(n => n.id === edge.from_node);
                    return (<div key={edge.id} className="flex items-center justify-between p-1.5 rounded text-xs" style={{ background: "rgba(255,255,255,0.03)", color: "var(--yunque-text)" }}>
                      <span>{edge.from_node === selNode.id ? "→" : "←"} {other?.name || "?"}</span>
                      <Button isIconOnly aria-label="关闭" size="sm" variant="ghost" onPress={() => removeEdge(edge.id)} style={{ color: "var(--yunque-text-muted)" }}>
                        <X size={12} />
                      </Button></div>);
                  })}</div>}
            </div>
            <Button variant="ghost" onPress={() => removeNode(selNode.id)}
              className="w-full text-danger bg-danger/10 hover:bg-danger/20 mt-2">
              <Trash2 size={14} /> 删除节点
            </Button>
          </div>
        </div>
      )}
    </div>
  );
}

export default function WorkflowEditorPage() {
  return (
    <Suspense fallback={<div className="flex items-center justify-center h-[60vh]"><Spinner size="lg" /></div>}>
      <WorkflowEditorContent />
    </Suspense>
  );
}
