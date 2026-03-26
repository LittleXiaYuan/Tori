"use client";

import { memo } from "react";
import { Handle, Position, type NodeProps } from "@xyflow/react";
import {
  Wrench, Brain, GitBranch, Layers, Merge, MessageSquare,
  RefreshCw, Box, Globe, Code2, BookOpen,
} from "lucide-react";

// Color scheme for each node type
const nodeStyles: Record<string, { color: string; bg: string; border: string; icon: React.ElementType }> = {
  skill:     { color: "#3b82f6", bg: "#3b82f610", border: "#3b82f630", icon: Wrench },
  llm:       { color: "#8b5cf6", bg: "#8b5cf610", border: "#8b5cf630", icon: Brain },
  condition: { color: "#eab308", bg: "#eab30810", border: "#eab30830", icon: GitBranch },
  parallel:  { color: "#22c55e", bg: "#22c55e10", border: "#22c55e30", icon: Layers },
  join:      { color: "#14b8a6", bg: "#14b8a610", border: "#14b8a630", icon: Merge },
  input:     { color: "#f97316", bg: "#f9731610", border: "#f9731630", icon: MessageSquare },
  transform: { color: "#06b6d4", bg: "#06b6d410", border: "#06b6d430", icon: RefreshCw },
  subflow:   { color: "#ec4899", bg: "#ec489910", border: "#ec489930", icon: Box },
  browser:   { color: "#6366f1", bg: "#6366f110", border: "#6366f130", icon: Globe },
  code:      { color: "#84cc16", bg: "#84cc1610", border: "#84cc1630", icon: Code2 },
  knowledge: { color: "#f43f5e", bg: "#f43f5e10", border: "#f43f5e30", icon: BookOpen },
};

// Execution state overlay colors
const execStateColors: Record<string, string> = {
  pending: "transparent",
  running: "#3b82f6",
  done: "#22c55e",
  failed: "#ef4444",
  skipped: "#6b7280",
  waiting: "#eab308",
};

interface WorkflowNodeData {
  label: string;
  nodeType: string;
  config?: Record<string, any>;
  execState?: string;
  [key: string]: unknown;
}

function WorkflowNode({ data, selected }: NodeProps) {
  const d = data as unknown as WorkflowNodeData;
  const nodeType = d.nodeType || "skill";
  const style = nodeStyles[nodeType] || nodeStyles.skill;
  const Icon = style.icon;
  const execState = d.execState;
  const execColor = execState ? execStateColors[execState] : undefined;
  const isRunning = execState === "running";

  return (
    <div
      className="relative rounded-xl border px-4 py-3 min-w-[160px] max-w-[220px] transition-all"
      style={{
        background: style.bg,
        borderColor: selected ? style.color : style.border,
        boxShadow: isRunning
          ? `0 0 20px ${style.color}40, 0 0 40px ${style.color}20`
          : selected
          ? `0 0 0 2px ${style.color}30`
          : "none",
        animation: isRunning ? "pulse 2s ease-in-out infinite" : undefined,
      }}
    >
      {/* Execution state indicator */}
      {execColor && execColor !== "transparent" && (
        <div
          className="absolute -top-1 -right-1 w-3 h-3 rounded-full border-2"
          style={{ background: execColor, borderColor: "var(--bg, #0a0a0a)" }}
        />
      )}

      {/* Input handle */}
      <Handle
        type="target"
        position={Position.Top}
        className="!w-3 !h-3 !rounded-full !border-2"
        style={{ background: style.bg, borderColor: style.color }}
      />

      {/* Content */}
      <div className="flex items-center gap-2.5">
        <div
          className="w-8 h-8 rounded-lg flex items-center justify-center shrink-0"
          style={{ background: style.color + "20" }}
        >
          <Icon size={16} style={{ color: style.color }} />
        </div>
        <div className="min-w-0">
          <div className="text-xs font-medium truncate" style={{ color: "var(--text, #e5e5e5)" }}>
            {d.label || nodeType}
          </div>
          <div className="text-[10px] truncate" style={{ color: "var(--text-muted, #888)" }}>
            {getSubtitle(nodeType, d.config)}
          </div>
        </div>
      </div>

      {/* Output handle(s) */}
      {nodeType === "condition" ? (
        <>
          <Handle
            type="source"
            position={Position.Bottom}
            id="true"
            className="!w-3 !h-3 !rounded-full !border-2"
            style={{ background: "#22c55e20", borderColor: "#22c55e", left: "30%" }}
          />
          <Handle
            type="source"
            position={Position.Bottom}
            id="false"
            className="!w-3 !h-3 !rounded-full !border-2"
            style={{ background: "#ef444420", borderColor: "#ef4444", left: "70%" }}
          />
          <div className="flex justify-between px-2 mt-1">
            <span className="text-[9px]" style={{ color: "#22c55e" }}>✓</span>
            <span className="text-[9px]" style={{ color: "#ef4444" }}>✗</span>
          </div>
        </>
      ) : (
        <Handle
          type="source"
          position={Position.Bottom}
          className="!w-3 !h-3 !rounded-full !border-2"
          style={{ background: style.bg, borderColor: style.color }}
        />
      )}
    </div>
  );
}

function getSubtitle(type: string, config?: Record<string, any>): string {
  if (!config) return type;
  switch (type) {
    case "skill": return config.skill_name || "选择技能";
    case "llm": return config.user_prompt?.slice(0, 25) || "LLM 调用";
    case "condition": return config.variable || "条件分支";
    case "transform": return config.template?.slice(0, 20) || "数据转换";
    case "subflow": return config.definition_id || "子工作流";
    case "browser": return config.action || "浏览器操作";
    case "code": return config.language || "代码执行";
    case "knowledge": return config.query?.slice(0, 20) || "知识检索";
    default: return type;
  }
}

export const MemoizedWorkflowNode = memo(WorkflowNode);
export { nodeStyles };
export type { WorkflowNodeData };
