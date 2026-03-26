"use client";

import { nodeStyles, type WorkflowNodeData } from "./WorkflowNode";
import { X } from "lucide-react";

interface PropertyPanelProps {
  zh: boolean;
  node: { id: string; data: WorkflowNodeData } | null;
  onUpdate: (id: string, data: Partial<WorkflowNodeData>) => void;
  onClose: () => void;
}

export default function PropertyPanel({ zh, node, onUpdate, onClose }: PropertyPanelProps) {
  if (!node) return null;

  const d = node.data;
  const nodeType = d.nodeType || "skill";
  const style = nodeStyles[nodeType] || nodeStyles.skill;
  const Icon = style.icon;

  const update = (key: string, value: any) => {
    onUpdate(node.id, { config: { ...d.config, [key]: value } });
  };

  const inputClass = "w-full px-3 py-2 rounded-lg text-sm border outline-none";
  const inputStyle = { background: "var(--bg)", borderColor: "var(--border)", color: "var(--text)" };

  return (
    <div className="w-72 border-l h-full overflow-y-auto" style={{ borderColor: "var(--border)", background: "var(--bg-card)" }}>
      {/* Header */}
      <div className="flex items-center justify-between px-4 py-3 border-b" style={{ borderColor: "var(--border)" }}>
        <div className="flex items-center gap-2">
          <Icon size={14} style={{ color: style.color }} />
          <span className="text-sm font-medium">{zh ? "属性" : "Properties"}</span>
        </div>
        <button onClick={onClose} className="p-1 rounded cursor-pointer" style={{ color: "var(--text-muted)" }}>
          <X size={14} />
        </button>
      </div>

      <div className="p-4 space-y-4">
        {/* Name */}
        <div>
          <label className="text-xs mb-1.5 block" style={{ color: "var(--text-muted)" }}>{zh ? "节点名称" : "Name"}</label>
          <input className={inputClass} style={inputStyle} value={d.label || ""}
            onChange={e => onUpdate(node.id, { label: e.target.value })} />
        </div>

        {/* Type-specific config */}
        {nodeType === "skill" && (
          <div>
            <label className="text-xs mb-1.5 block" style={{ color: "var(--text-muted)" }}>{zh ? "技能名称" : "Skill Name"}</label>
            <input className={inputClass} style={inputStyle} placeholder="e.g. web_search"
              value={d.config?.skill_name || ""} onChange={e => update("skill_name", e.target.value)} />
          </div>
        )}

        {nodeType === "llm" && (
          <>
            <div>
              <label className="text-xs mb-1.5 block" style={{ color: "var(--text-muted)" }}>{zh ? "系统提示词" : "System Prompt"}</label>
              <textarea className={inputClass + " h-20 resize-none"} style={inputStyle}
                value={d.config?.system_prompt || ""} onChange={e => update("system_prompt", e.target.value)} />
            </div>
            <div>
              <label className="text-xs mb-1.5 block" style={{ color: "var(--text-muted)" }}>{zh ? "用户提示词" : "User Prompt"}</label>
              <textarea className={inputClass + " h-24 resize-none"} style={inputStyle} placeholder="{variable_name}"
                value={d.config?.user_prompt || ""} onChange={e => update("user_prompt", e.target.value)} />
            </div>
          </>
        )}

        {nodeType === "condition" && (
          <>
            <div>
              <label className="text-xs mb-1.5 block" style={{ color: "var(--text-muted)" }}>{zh ? "变量名" : "Variable"}</label>
              <input className={inputClass} style={inputStyle} placeholder="_node_xxx"
                value={d.config?.variable || ""} onChange={e => update("variable", e.target.value)} />
            </div>
            <div className="text-[10px] p-2 rounded-lg" style={{ background: "var(--bg-hover)", color: "var(--text-muted)" }}>
              {zh ? "✓ 真：非空/非 false/非 0\n✗ 假：空/false/0/不存在" : "✓ True: non-empty/non-false/non-0\n✗ False: empty/false/0/missing"}
            </div>
          </>
        )}

        {nodeType === "transform" && (
          <div>
            <label className="text-xs mb-1.5 block" style={{ color: "var(--text-muted)" }}>{zh ? "模板" : "Template"}</label>
            <textarea className={inputClass + " h-24 resize-none font-mono text-xs"} style={inputStyle}
              placeholder={"结果: {_node_xxx}"}
              value={d.config?.template || ""} onChange={e => update("template", e.target.value)} />
          </div>
        )}

        {nodeType === "subflow" && (
          <div>
            <label className="text-xs mb-1.5 block" style={{ color: "var(--text-muted)" }}>{zh ? "工作流 ID" : "Workflow ID"}</label>
            <input className={inputClass} style={inputStyle}
              value={d.config?.definition_id || ""} onChange={e => update("definition_id", e.target.value)} />
          </div>
        )}

        {nodeType === "browser" && (
          <>
            <div>
              <label className="text-xs mb-1.5 block" style={{ color: "var(--text-muted)" }}>{zh ? "操作" : "Action"}</label>
              <select className={inputClass} style={inputStyle}
                value={d.config?.action || "navigate"} onChange={e => update("action", e.target.value)}>
                <option value="navigate">{zh ? "导航到 URL" : "Navigate"}</option>
                <option value="click">{zh ? "点击元素" : "Click"}</option>
                <option value="type">{zh ? "输入文本" : "Type"}</option>
                <option value="screenshot">{zh ? "截图" : "Screenshot"}</option>
                <option value="read_text">{zh ? "读取文本" : "Read Text"}</option>
                <option value="eval">{zh ? "执行 JS" : "Eval JS"}</option>
              </select>
            </div>
            <div>
              <label className="text-xs mb-1.5 block" style={{ color: "var(--text-muted)" }}>{zh ? "参数" : "Target"}</label>
              <input className={inputClass} style={inputStyle} placeholder={zh ? "URL / CSS 选择器" : "URL / CSS selector"}
                value={d.config?.target || ""} onChange={e => update("target", e.target.value)} />
            </div>
          </>
        )}

        {nodeType === "code" && (
          <>
            <div>
              <label className="text-xs mb-1.5 block" style={{ color: "var(--text-muted)" }}>{zh ? "语言" : "Language"}</label>
              <select className={inputClass} style={inputStyle}
                value={d.config?.language || "javascript"} onChange={e => update("language", e.target.value)}>
                <option value="javascript">JavaScript</option>
                <option value="python">Python</option>
              </select>
            </div>
            <div>
              <label className="text-xs mb-1.5 block" style={{ color: "var(--text-muted)" }}>{zh ? "代码" : "Code"}</label>
              <textarea className={inputClass + " h-32 resize-none font-mono text-xs"} style={inputStyle}
                value={d.config?.code || ""} onChange={e => update("code", e.target.value)} />
            </div>
          </>
        )}

        {nodeType === "knowledge" && (
          <div>
            <label className="text-xs mb-1.5 block" style={{ color: "var(--text-muted)" }}>{zh ? "检索查询" : "Query"}</label>
            <input className={inputClass} style={inputStyle} placeholder="{_node_xxx}"
              value={d.config?.query || ""} onChange={e => update("query", e.target.value)} />
          </div>
        )}

        {/* Timeout & Retry — all node types */}
        <div className="border-t pt-3" style={{ borderColor: "var(--border)" }}>
          <div className="text-[10px] uppercase tracking-wider mb-2 font-medium" style={{ color: "var(--text-muted)" }}>
            {zh ? "高级" : "Advanced"}
          </div>
          <div className="grid grid-cols-2 gap-2">
            <div>
              <label className="text-[10px] mb-1 block" style={{ color: "var(--text-muted)" }}>{zh ? "超时" : "Timeout"}</label>
              <input className={inputClass + " text-xs"} style={inputStyle} placeholder="30s"
                value={d.config?.timeout || ""} onChange={e => update("timeout", e.target.value)} />
            </div>
            <div>
              <label className="text-[10px] mb-1 block" style={{ color: "var(--text-muted)" }}>{zh ? "重试" : "Retries"}</label>
              <input className={inputClass + " text-xs"} style={inputStyle} type="number" min="0" max="5"
                value={d.config?.max_retries || ""} onChange={e => update("max_retries", parseInt(e.target.value) || 0)} />
            </div>
          </div>
        </div>

        {/* Node ID */}
        <div className="text-[10px] font-mono px-1" style={{ color: "var(--text-muted)" }}>
          ID: {node.id}
        </div>
      </div>
    </div>
  );
}
