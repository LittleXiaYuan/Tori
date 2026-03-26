"use client";

import { nodeStyles } from "./WorkflowNode";
import { DragEvent } from "react";

interface NodeTypeInfo {
  type: string;
  label: string;
  labelEn: string;
  group: string;
}

const nodeTypes: NodeTypeInfo[] = [
  { type: "skill",     label: "技能调用",   labelEn: "Skill",       group: "action" },
  { type: "llm",       label: "LLM 调用",   labelEn: "LLM",         group: "action" },
  { type: "browser",   label: "浏览器操作", labelEn: "Browser",     group: "action" },
  { type: "code",      label: "代码执行",   labelEn: "Code",        group: "action" },
  { type: "knowledge", label: "知识检索",   labelEn: "Knowledge",   group: "action" },
  { type: "condition", label: "条件分支",   labelEn: "Condition",   group: "flow" },
  { type: "parallel",  label: "并行分支",   labelEn: "Parallel",    group: "flow" },
  { type: "join",      label: "聚合等待",   labelEn: "Join",        group: "flow" },
  { type: "input",     label: "等待输入",   labelEn: "Input",       group: "flow" },
  { type: "transform", label: "数据转换",   labelEn: "Transform",   group: "data" },
  { type: "subflow",   label: "子工作流",   labelEn: "Subflow",     group: "data" },
];

const groups = [
  { key: "action", label: "执行", labelEn: "Actions" },
  { key: "flow",   label: "流程", labelEn: "Flow" },
  { key: "data",   label: "数据", labelEn: "Data" },
];

interface NodePaletteProps {
  zh: boolean;
}

export default function NodePalette({ zh }: NodePaletteProps) {
  const onDragStart = (e: DragEvent, nodeType: string, label: string) => {
    e.dataTransfer.setData("application/yunque-node-type", nodeType);
    e.dataTransfer.setData("application/yunque-node-label", label);
    e.dataTransfer.effectAllowed = "move";
  };

  return (
    <div className="w-56 border-r h-full overflow-y-auto py-4 px-3" style={{ borderColor: "var(--border)", background: "var(--bg-card)" }}>
      <div className="text-xs font-semibold mb-3 px-1" style={{ color: "var(--text-muted)" }}>
        {zh ? "节点" : "Nodes"}
      </div>

      {groups.map((group) => (
        <div key={group.key} className="mb-4">
          <div className="text-[10px] uppercase tracking-wider mb-2 px-1 font-medium" style={{ color: "var(--text-muted)" }}>
            {zh ? group.label : group.labelEn}
          </div>
          <div className="space-y-1">
            {nodeTypes.filter(n => n.group === group.key).map((nt) => {
              const style = nodeStyles[nt.type] || nodeStyles.skill;
              const Icon = style.icon;
              return (
                <div
                  key={nt.type}
                  draggable
                  onDragStart={(e) => onDragStart(e, nt.type, zh ? nt.label : nt.labelEn)}
                  className="flex items-center gap-2.5 px-3 py-2 rounded-lg text-xs cursor-grab active:cursor-grabbing transition-colors hover:opacity-80"
                  style={{ background: style.bg, border: `1px solid ${style.border}` }}
                >
                  <Icon size={14} style={{ color: style.color }} />
                  <span style={{ color: "var(--text)" }}>{zh ? nt.label : nt.labelEn}</span>
                </div>
              );
            })}
          </div>
        </div>
      ))}

      <div className="mt-4 px-1 text-[10px]" style={{ color: "var(--text-muted)" }}>
        {zh ? "拖拽节点到画布上" : "Drag nodes to canvas"}
      </div>
    </div>
  );
}
