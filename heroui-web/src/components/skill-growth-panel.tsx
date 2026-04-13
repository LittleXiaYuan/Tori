"use client";

import { Button, Chip } from "@heroui/react";
import { Sparkles, Workflow } from "lucide-react";
import type { SkillSuggestion } from "@/lib/api-types";

interface SkillGrowthPanelProps {
  suggestions: SkillSuggestion[];
  onSave?: (suggestion: SkillSuggestion) => void;
}

export function SkillGrowthPanel({ suggestions, onSave }: SkillGrowthPanelProps) {
  if (!suggestions?.length) return null;

  return (
    <div
      className="mt-3 rounded-[18px] border p-3"
      style={{
        background: "linear-gradient(180deg, rgba(139,92,246,0.1), rgba(139,92,246,0.03))",
        borderColor: "rgba(139,92,246,0.18)",
      }}
    >
      <div className="flex items-center gap-2">
        <div
          className="flex h-9 w-9 items-center justify-center rounded-2xl"
          style={{ background: "rgba(139,92,246,0.16)", color: "#c4b5fd" }}
        >
          <Workflow size={16} />
        </div>
        <div>
          <div className="text-sm font-semibold" style={{ color: "var(--yunque-text)" }}>
            Skill growth proposal
          </div>
          <div className="text-xs" style={{ color: "var(--yunque-text-muted)" }}>
            This looks like a reusable workflow instead of a one-off chat reply.
          </div>
        </div>
      </div>

      <div className="mt-3 space-y-2.5">
        {suggestions.map((suggestion) => (
          <div
            key={`${suggestion.name}-${suggestion.trigger}`}
            className="rounded-[16px] border p-3"
            style={{ background: "rgba(255,255,255,0.03)", borderColor: "rgba(255,255,255,0.06)" }}
          >
            <div className="flex flex-wrap items-center gap-2">
              <div className="text-sm font-medium" style={{ color: "var(--yunque-text-secondary)" }}>
                {suggestion.name}
              </div>
              <Chip size="sm" style={{ background: "rgba(139,92,246,0.14)", color: "#c4b5fd" }}>
                {suggestion.confidence}/10
              </Chip>
            </div>
            <div className="mt-1.5 text-xs leading-6" style={{ color: "var(--yunque-text-muted)" }}>
              {suggestion.description}
            </div>
            <div className="mt-2 rounded-xl px-2.5 py-2 text-[11px]" style={{ background: "rgba(15,23,42,0.35)", color: "var(--yunque-text-secondary)" }}>
              Trigger: {suggestion.trigger}
            </div>
            <div className="mt-3 flex flex-wrap gap-2">
              <Button size="sm" className="rounded-full px-3" onPress={() => onSave?.(suggestion)}>
                <Sparkles size={14} />
                Save as skill
              </Button>
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}

export default SkillGrowthPanel;
