"use client";

import { useState } from "react";
import {
  Brain, BookOpen, RefreshCw, Layers, ChevronDown, ChevronRight,
  Database, Lightbulb, Sparkles, GitBranch,
} from "lucide-react";

export interface MemoryAccess {
  key: string;
  category: "episodic" | "semantic" | "procedural" | "working";
  relevance: number;
  summary?: string;
}

export interface ReflectionEvent {
  type: "self_check" | "strategy_shift" | "insight" | "correction";
  summary: string;
  confidence?: number;
  ts?: string;
}

export interface ContextLayer {
  name: string;
  source: "skill" | "memory" | "user_profile" | "environment";
  weight: number;
}

export interface CognitiveStatusBarProps {
  memories?: MemoryAccess[];
  reflections?: ReflectionEvent[];
  contextLayers?: ContextLayer[];
  activeSkills?: string[];
  isLive?: boolean;
}

const categoryMeta: Record<MemoryAccess["category"], { icon: typeof Brain; color: string; label: string }> = {
  episodic:   { icon: BookOpen,  color: "var(--cogni-memory)",  label: "情景记忆" },
  semantic:   { icon: Database,  color: "var(--yunque-info)",    label: "语义记忆" },
  procedural: { icon: GitBranch, color: "var(--cogni-skill)",   label: "程序记忆" },
  working:    { icon: Layers,    color: "var(--yunque-warning)", label: "工作记忆" },
};

const reflectMeta: Record<ReflectionEvent["type"], { icon: typeof Brain; color: string; label: string }> = {
  self_check:     { icon: RefreshCw,  color: "var(--cogni-reflect)", label: "自检" },
  strategy_shift: { icon: GitBranch,  color: "var(--yunque-warning)", label: "策略切换" },
  insight:        { icon: Lightbulb,  color: "var(--evo-streak)",    label: "洞察" },
  correction:     { icon: RefreshCw,  color: "var(--yunque-danger)",  label: "纠偏" },
};

export function CognitiveStatusBar({
  memories = [],
  reflections = [],
  contextLayers = [],
  activeSkills = [],
  isLive = false,
}: CognitiveStatusBarProps) {
  const [expanded, setExpanded] = useState(false);

  const hasContent = memories.length > 0 || reflections.length > 0 || contextLayers.length > 0 || activeSkills.length > 0;
  if (!hasContent) return null;

  const memorySummary = memories.length > 0
    ? `${memories.length} 条记忆`
    : null;
  const reflectionSummary = reflections.length > 0
    ? `${reflections.length} 次反思`
    : null;

  return (
    <div
      className="mt-1.5 rounded-xl border overflow-hidden transition-all"
      style={{
        background: "var(--cogni-bar-bg)",
        borderColor: "var(--cogni-bar-border)",
      }}
    >
      {/* Collapsed summary row */}
      <button
        className="w-full flex items-center gap-2 px-2.5 py-1.5 text-left hover:bg-[rgba(255,255,255,0.03)] transition-colors"
        onClick={() => setExpanded(!expanded)}
        aria-expanded={expanded}
        aria-label="展开认知状态详情"
      >
        <Brain size={13} style={{ color: "var(--cogni-memory)", flexShrink: 0 }} />

        {isLive && (
          <span
            className="w-1.5 h-1.5 rounded-full animate-pulse"
            style={{ background: "var(--cogni-memory)", flexShrink: 0 }}
          />
        )}

        <div className="flex items-center gap-1.5 flex-1 min-w-0 overflow-x-auto">
          {memorySummary && (
            <span
              className="inline-flex items-center gap-1 px-2 py-0.5 rounded-full text-[10px] font-medium whitespace-nowrap"
              style={{ background: "var(--cogni-memory-muted)", color: "var(--cogni-memory)" }}
            >
              <Database size={10} />
              {memorySummary}
            </span>
          )}
          {reflectionSummary && (
            <span
              className="inline-flex items-center gap-1 px-2 py-0.5 rounded-full text-[10px] font-medium whitespace-nowrap"
              style={{ background: "var(--cogni-reflect-muted)", color: "var(--cogni-reflect)" }}
            >
              <RefreshCw size={10} />
              {reflectionSummary}
            </span>
          )}
          {activeSkills.length > 0 && (
            <span
              className="inline-flex items-center gap-1 px-2 py-0.5 rounded-full text-[10px] font-medium whitespace-nowrap"
              style={{ background: "var(--cogni-skill-muted)", color: "var(--cogni-skill)" }}
            >
              <Sparkles size={10} />
              {activeSkills.length} 技能
            </span>
          )}
          {contextLayers.length > 0 && (
            <span
              className="inline-flex items-center gap-1 px-2 py-0.5 rounded-full text-[10px] font-medium whitespace-nowrap"
              style={{ background: "var(--cogni-context-muted)", color: "var(--cogni-context)" }}
            >
              <Layers size={10} />
              {contextLayers.length} 层上下文
            </span>
          )}
        </div>

        {expanded
          ? <ChevronDown size={12} style={{ color: "var(--yunque-text-muted)", flexShrink: 0 }} />
          : <ChevronRight size={12} style={{ color: "var(--yunque-text-muted)", flexShrink: 0 }} />
        }
      </button>

      {/* Expanded detail panel */}
      {expanded && (
        <div className="px-3 pb-2.5 space-y-2 border-t" style={{ borderColor: "var(--cogni-bar-border)" }}>
          {/* Memories */}
          {memories.length > 0 && (
            <div className="pt-2">
              <div className="text-[10px] font-semibold mb-1.5 uppercase tracking-wider" style={{ color: "var(--yunque-text-muted)" }}>
                调用的记忆
              </div>
              <div className="space-y-1">
                {memories.map((mem) => {
                  const meta = categoryMeta[mem.category];
                  const Icon = meta.icon;
                  return (
                    <div
                      key={mem.key}
                      className="flex items-start gap-2 rounded-lg px-2 py-1.5"
                      style={{ background: "rgba(255,255,255,0.02)" }}
                    >
                      <Icon size={12} style={{ color: meta.color, marginTop: 2, flexShrink: 0 }} />
                      <div className="min-w-0 flex-1">
                        <div className="flex items-center gap-1.5">
                          <span className="text-[11px] font-medium truncate" style={{ color: "var(--yunque-text-secondary)" }}>
                            {mem.key}
                          </span>
                          <span
                            className="px-1.5 py-px rounded text-[9px]"
                            style={{ background: `${meta.color}18`, color: meta.color }}
                          >
                            {meta.label}
                          </span>
                          <span className="text-[9px] ml-auto" style={{ color: "var(--yunque-text-muted)" }}>
                            相关度 {(mem.relevance * 100).toFixed(0)}%
                          </span>
                        </div>
                        {mem.summary && (
                          <div className="text-[10px] mt-0.5 leading-4" style={{ color: "var(--yunque-text-muted)" }}>
                            {mem.summary}
                          </div>
                        )}
                      </div>
                    </div>
                  );
                })}
              </div>
            </div>
          )}

          {/* Reflections */}
          {reflections.length > 0 && (
            <div className="pt-1">
              <div className="text-[10px] font-semibold mb-1.5 uppercase tracking-wider" style={{ color: "var(--yunque-text-muted)" }}>
                反思过程
              </div>
              <div className="space-y-1">
                {reflections.map((ref, i) => {
                  const meta = reflectMeta[ref.type];
                  const Icon = meta.icon;
                  return (
                    <div
                      key={`${ref.type}-${i}`}
                      className="flex items-start gap-2 rounded-lg px-2 py-1.5"
                      style={{ background: "rgba(255,255,255,0.02)" }}
                    >
                      <Icon size={12} style={{ color: meta.color, marginTop: 2, flexShrink: 0 }} />
                      <div className="min-w-0 flex-1">
                        <div className="flex items-center gap-1.5">
                          <span
                            className="px-1.5 py-px rounded text-[9px] font-medium"
                            style={{ background: `${meta.color}18`, color: meta.color }}
                          >
                            {meta.label}
                          </span>
                          {ref.confidence !== undefined && (
                            <span className="text-[9px]" style={{ color: "var(--yunque-text-muted)" }}>
                              置信度 {(ref.confidence * 100).toFixed(0)}%
                            </span>
                          )}
                        </div>
                        <div className="text-[11px] mt-0.5 leading-4" style={{ color: "var(--yunque-text-secondary)" }}>
                          {ref.summary}
                        </div>
                      </div>
                    </div>
                  );
                })}
              </div>
            </div>
          )}

          {/* Context layers */}
          {contextLayers.length > 0 && (
            <div className="pt-1">
              <div className="text-[10px] font-semibold mb-1.5 uppercase tracking-wider" style={{ color: "var(--yunque-text-muted)" }}>
                上下文分层
              </div>
              <div className="flex flex-wrap gap-1">
                {contextLayers
                  .sort((a, b) => b.weight - a.weight)
                  .map((layer) => {
                    const opacity = 0.3 + layer.weight * 0.7;
                    return (
                      <span
                        key={layer.name}
                        className="inline-flex items-center gap-1 px-2 py-0.5 rounded-full text-[10px]"
                        style={{
                          background: "var(--cogni-context-muted)",
                          color: "var(--cogni-context)",
                          opacity,
                        }}
                      >
                        {layer.name}
                        <span className="text-[8px]" style={{ opacity: 0.7 }}>{(layer.weight * 100).toFixed(0)}%</span>
                      </span>
                    );
                  })}
              </div>
            </div>
          )}
        </div>
      )}
    </div>
  );
}

export default CognitiveStatusBar;
