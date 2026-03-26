"use client";

import { useEffect, useState, useCallback, useRef } from "react";
import { useParams, useRouter } from "next/navigation";
import {
  ReactFlow,
  ReactFlowProvider,
  Background,
  Controls,
  MiniMap,
  Panel,
  useNodesState,
  useEdgesState,
  addEdge,
  useReactFlow,
  type Connection,
  type Edge as RFEdge,
  type Node as RFNode,
} from "@xyflow/react";
import "@xyflow/react/dist/style.css";
import { Save, ArrowLeft } from "lucide-react";

import { MemoizedWorkflowNode, type WorkflowNodeData } from "@/components/workflow/WorkflowNode";
import NodePalette from "@/components/workflow/NodePalette";
import PropertyPanel from "@/components/workflow/PropertyPanel";

const nodeTypes = {
  custom: MemoizedWorkflowNode,
};

// Backend Types
interface DefNode {
  id: string;
  name: string;
  type: string;
  config?: any;
  position: { x: number; y: number };
  timeout?: string;
  retry_policy?: { max_retries: number; backoff_ms: number; multiplier: number };
}
interface DefEdge {
  id: string;
  from_node: string;
  to_node: string;
  condition?: string;
  label?: string;
}
interface WorkflowDef {
  id: string;
  name: string;
  description: string;
  version: number;
  nodes: DefNode[];
  edges: DefEdge[];
  tenant_id: string;
}

function EditorCanvas() {
  const params = useParams();
  const router = useRouter();
  const id = params?.id as string;
  const { screenToFlowPosition } = useReactFlow();

  const [nodes, setNodes, onNodesChange] = useNodesState<RFNode>([]);
  const [edges, setEdges, onEdgesChange] = useEdgesState<RFEdge>([]);
  const [workflow, setWorkflow] = useState<WorkflowDef | null>(null);
  const [loading, setLoading] = useState(true);
  const [selectedNodeId, setSelectedNodeId] = useState<string | null>(null);
  
  const containerRef = useRef<HTMLDivElement>(null);

  const apiHeaders = () => {
    let token = "";
    if (typeof window !== "undefined") {
      token = localStorage.getItem("yunque_api_key") || localStorage.getItem("yunque_token") || "";
    }
    return {
      "Content-Type": "application/json",
      "X-API-Key": token,
    };
  };

  useEffect(() => {
    if (id) fetchWorkflow();
  }, [id]);

  const fetchWorkflow = async () => {
    try {
      const res = await fetch("/v1/workflows", { headers: apiHeaders() });
      const data = await res.json();
      const wf = data.workflows?.find((w: any) => w.id === id);
      if (wf) {
        setWorkflow(wf);
        // Translate format
        const rfNodes: RFNode[] = (wf.nodes || []).map((n: DefNode) => ({
          id: n.id,
          type: "custom",
          position: n.position || { x: 0, y: 0 },
          data: {
            label: n.name,
            nodeType: n.type,
            config: n.config || {},
            timeout: n.timeout,
            max_retries: n.retry_policy?.max_retries,
          }
        }));
        const rfEdges: RFEdge[] = (wf.edges || []).map((e: DefEdge) => ({
          id: e.id,
          source: e.from_node,
          target: e.to_node,
          sourceHandle: e.condition || undefined,
          label: e.label,
        }));
        setNodes(rfNodes);
        setEdges(rfEdges);
      } else {
        console.error("Workflow not found");
      }
    } catch (e) {
      console.error("Failed to load workflow", e);
    } finally {
      setLoading(false);
    }
  };

  const onConnect = useCallback((params: Connection) => {
    setEdges((eds) => addEdge(params, eds));
  }, [setEdges]);

  const onDragOver = useCallback((event: React.DragEvent) => {
    event.preventDefault();
    event.dataTransfer.dropEffect = "move";
  }, []);

  const onDrop = useCallback(
    (event: React.DragEvent) => {
      event.preventDefault();
      const type = event.dataTransfer.getData("application/yunque-node-type");
      const label = event.dataTransfer.getData("application/yunque-node-label");
      if (!type) return;

      const position = screenToFlowPosition({ x: event.clientX, y: event.clientY });
      
      const newNode: RFNode = {
        id: `node_${Date.now()}`,
        type: "custom",
        position,
        data: { nodeType: type, label: label, config: {} },
      };
      setNodes((nds) => nds.concat(newNode));
    },
    [screenToFlowPosition, setNodes]
  );

  const onNodeClick = (_: React.MouseEvent, node: RFNode) => {
    setSelectedNodeId(node.id);
  };
  const onPaneClick = () => {
    setSelectedNodeId(null);
  };

  const handleUpdateNode = (nodeId: string, newData: Partial<WorkflowNodeData>) => {
    setNodes((nds) =>
      nds.map((n) => {
        if (n.id === nodeId) {
          const mergedData = { ...n.data, ...newData };
          if (newData.config && n.data.config) {
            mergedData.config = { ...(n.data.config as any), ...(newData.config as any) };
          }
          return { ...n, data: mergedData };
        }
        return n;
      })
    );
  };

  const saveWorkflow = async () => {
    if (!workflow) return;
    
    const defNodes: DefNode[] = nodes.map(n => ({
      id: n.id,
      name: n.data.label as string || n.data.nodeType as string,
      type: n.data.nodeType as string,
      position: { x: n.position.x, y: n.position.y },
      config: n.data.config,
      timeout: n.data.timeout as string,
      retry_policy: n.data.max_retries ? { max_retries: Number(n.data.max_retries), backoff_ms: 500, multiplier: 2.0 } : undefined
    }));

    const defEdges: DefEdge[] = edges.map(e => ({
      id: e.id,
      from_node: e.source,
      to_node: e.target,
      condition: e.sourceHandle || "",
      label: e.label as string,
    }));

    const updatedDef = { ...workflow, nodes: defNodes, edges: defEdges };

    try {
      const res = await fetch("/v1/workflows", {
        method: "POST", 
        headers: apiHeaders(),
        body: JSON.stringify(updatedDef)
      });
      if (res.ok) {
        alert("工作流保存成功！");
      } else {
        alert("工作流保存失败，请检查网络！");
      }
    } catch (e) {
      alert("工作流保存时发生错误！");
    }
  };

  const selectedNode = nodes.find(n => n.id === selectedNodeId);

  if (loading) return <div className="p-8 text-neutral-500">正在载入流程引擎...</div>;

  return (
    <div className="flex h-[calc(100vh-64px)] overflow-hidden w-full relative" style={{ background: "var(--bg, #fafafa)" }}>
      {/* 侧边拖拽组件库 */}
      <NodePalette zh={true} />
      
      {/* 主画布区域 */}
      <div className="flex-1 relative" ref={containerRef}>
        <ReactFlow
          nodes={nodes}
          edges={edges}
          onNodesChange={onNodesChange}
          onEdgesChange={onEdgesChange}
          onConnect={onConnect}
          nodeTypes={nodeTypes}
          onDrop={onDrop}
          onDragOver={onDragOver}
          onNodeClick={onNodeClick}
          onPaneClick={onPaneClick}
          fitView
          colorMode="system"
        >
          <Background color="#aeaeae" gap={20} size={1} />
          <Controls className="!bg-white dark:!bg-neutral-900 border border-neutral-200 dark:border-neutral-800" />
          <MiniMap className="!bg-white dark:!bg-neutral-900 border border-neutral-200 dark:border-neutral-800" maskColor="var(--bg-card)" />
          
          <Panel position="top-left" className="m-4">
            <button onClick={() => router.push("/workflows")} className="flex items-center gap-2 px-3 py-1.5 rounded-lg border bg-white dark:bg-neutral-900 shadow-sm hover:opacity-80 transition-opacity text-sm">
              <ArrowLeft size={16} /> 返回
            </button>
          </Panel>

          <Panel position="top-right" className="m-4 flex gap-2">
            <button onClick={saveWorkflow} className="flex items-center gap-2 px-4 py-2 rounded-lg shadow-sm hover:opacity-90 transition-opacity text-sm text-white bg-blue-600 font-medium">
              <Save size={16} /> 保存发布
            </button>
          </Panel>
        </ReactFlow>
      </div>

      {/* 右侧属性面板 */}
      {selectedNode && (
        <PropertyPanel 
          zh={true} 
          node={{ id: selectedNode.id, data: selectedNode.data as unknown as WorkflowNodeData }} 
          onUpdate={handleUpdateNode} 
          onClose={() => setSelectedNodeId(null)}
        />
      )}
    </div>
  );
}

export default function WorkflowEditorPage() {
  return (
    <ReactFlowProvider>
      <EditorCanvas />
    </ReactFlowProvider>
  );
}
