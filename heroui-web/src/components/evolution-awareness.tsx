"use client";

import { useState } from "react";
import { Button, Chip } from "@heroui/react";
import {
  TrendingUp, Award, Flame, Star, ChevronDown, ChevronRight,
  Zap, Target, BookOpen, Shield, Brain, ArrowUpRight,
} from "lucide-react";

export interface EvolutionMilestone {
  id: string;
  type: "skill_mastered" | "streak" | "insight" | "level_up" | "first_time";
  label: string;
  description?: string;
  achievedAt: string;
  xpGained?: number;
}

export interface SkillProgress {
  name: string;
  level: number;
  xp: number;
  xpToNext: number;
  recentDelta?: number;
}

export interface WeeklyEvolutionReport {
  weekLabel: string;
  totalInteractions: number;
  memoriesFormed: number;
  skillsImproved: number;
  reflections: number;
  topSkills: SkillProgress[];
  milestones: EvolutionMilestone[];
  growthRate?: number;
}

export interface EvolutionAwarenessProps {
  report?: WeeklyEvolutionReport;
  recentMilestones?: EvolutionMilestone[];
  currentLevel?: number;
  totalXp?: number;
  xpToNextLevel?: number;
  compact?: boolean;
}

const milestoneIcon: Record<EvolutionMilestone["type"], typeof Award> = {
  skill_mastered: Star,
  streak:         Flame,
  insight:        Brain,
  level_up:       TrendingUp,
  first_time:     Zap,
};

const milestoneColor: Record<EvolutionMilestone["type"], string> = {
  skill_mastered: "var(--evo-badge)",
  streak:         "var(--evo-streak)",
  insight:        "var(--cogni-reflect)",
  level_up:       "var(--evo-xp)",
  first_time:     "var(--yunque-info)",
};

function XpBar({ current, max, label }: { current: number; max: number; label?: string }) {
  const pct = Math.min((current / max) * 100, 100);
  return (
    <div className="w-full">
      {label && (
        <div className="flex items-center justify-between mb-1">
          <span className="text-[10px] font-medium" style={{ color: "var(--yunque-text-secondary)" }}>{label}</span>
          <span className="text-[10px]" style={{ color: "var(--yunque-text-muted)" }}>{current}/{max} XP</span>
        </div>
      )}
      <div className="h-1.5 rounded-full overflow-hidden" style={{ background: "rgba(255,255,255,0.06)" }}>
        <div
          className="h-full rounded-full transition-all duration-700"
          style={{
            width: `${pct}%`,
            background: `linear-gradient(90deg, var(--evo-xp), var(--evo-badge))`,
          }}
        />
      </div>
    </div>
  );
}

function MilestoneBadge({ milestone }: { milestone: EvolutionMilestone }) {
  const Icon = milestoneIcon[milestone.type];
  const color = milestoneColor[milestone.type];

  return (
    <div
      className="inline-flex items-center gap-1.5 px-2.5 py-1 rounded-full text-[10px] font-medium"
      style={{ background: `${color}18`, color, border: `1px solid ${color}28` }}
      title={milestone.description || milestone.label}
    >
      <Icon size={11} />
      {milestone.label}
    </div>
  );
}

export function EvolutionAwareness({
  report,
  recentMilestones = [],
  currentLevel = 1,
  totalXp = 0,
  xpToNextLevel = 100,
  compact = false,
}: EvolutionAwarenessProps) {
  const [expanded, setExpanded] = useState(false);

  if (!report && recentMilestones.length === 0) return null;

  if (compact) {
    return (
      <div className="flex items-center gap-2 flex-wrap">
        <span
          className="inline-flex items-center gap-1 px-2 py-0.5 rounded-full text-[10px] font-semibold"
          style={{ background: "var(--evo-xp-muted)", color: "var(--evo-xp)" }}
        >
          <TrendingUp size={10} />
          Lv.{currentLevel}
        </span>
        {recentMilestones.slice(0, 3).map((m) => (
          <MilestoneBadge key={m.id} milestone={m} />
        ))}
      </div>
    );
  }

  return (
    <div
      className="rounded-[18px] border overflow-hidden"
      style={{
        background: "var(--evo-card-bg)",
        borderColor: "var(--evo-card-border)",
      }}
    >
      {/* Header */}
      <div className="p-3">
        <div className="flex items-center gap-2.5">
          <div
            className="flex h-9 w-9 items-center justify-center rounded-2xl"
            style={{ background: "var(--evo-xp-muted)", color: "var(--evo-xp)" }}
          >
            <TrendingUp size={16} />
          </div>
          <div className="flex-1 min-w-0">
            <div className="flex items-center gap-2">
              <span className="text-sm font-semibold" style={{ color: "var(--yunque-text)" }}>
                进化感知
              </span>
              <Chip
                size="sm"
                style={{ background: "var(--evo-xp-muted)", color: "var(--evo-xp)" }}
              >
                Lv.{currentLevel}
              </Chip>
              {report?.growthRate !== undefined && report.growthRate > 0 && (
                <span
                  className="inline-flex items-center gap-0.5 text-[10px] font-medium"
                  style={{ color: "var(--evo-xp)" }}
                >
                  <ArrowUpRight size={10} />
                  +{report.growthRate}%
                </span>
              )}
            </div>
            <XpBar current={totalXp % xpToNextLevel} max={xpToNextLevel} />
          </div>
        </div>
      </div>

      {/* Recent milestones */}
      {recentMilestones.length > 0 && (
        <div className="px-3 pb-2">
          <div className="flex flex-wrap gap-1.5">
            {recentMilestones.slice(0, 6).map((m) => (
              <MilestoneBadge key={m.id} milestone={m} />
            ))}
          </div>
        </div>
      )}

      {/* Weekly report toggle */}
      {report && (
        <>
          <button
            className="w-full flex items-center gap-2 px-3 py-2 text-left border-t hover:bg-[rgba(255,255,255,0.02)] transition-colors"
            style={{ borderColor: "var(--evo-card-border)" }}
            onClick={() => setExpanded(!expanded)}
            aria-expanded={expanded}
            aria-label="展开周报详情"
          >
            <BookOpen size={12} style={{ color: "var(--yunque-text-muted)" }} />
            <span className="text-[11px] font-medium flex-1" style={{ color: "var(--yunque-text-secondary)" }}>
              {report.weekLabel} 周报
            </span>
            {expanded
              ? <ChevronDown size={12} style={{ color: "var(--yunque-text-muted)" }} />
              : <ChevronRight size={12} style={{ color: "var(--yunque-text-muted)" }} />
            }
          </button>

          {expanded && (
            <div className="px-3 pb-3 space-y-3 border-t" style={{ borderColor: "var(--evo-card-border)" }}>
              {/* Stats grid */}
              <div className="grid grid-cols-4 gap-2 pt-2.5">
                {[
                  { label: "对话", value: report.totalInteractions, icon: Target, color: "var(--yunque-accent)" },
                  { label: "记忆", value: report.memoriesFormed, icon: Brain, color: "var(--cogni-memory)" },
                  { label: "技能", value: report.skillsImproved, icon: Star, color: "var(--evo-streak)" },
                  { label: "反思", value: report.reflections, icon: Shield, color: "var(--cogni-reflect)" },
                ].map((stat) => (
                  <div
                    key={stat.label}
                    className="flex flex-col items-center gap-1 rounded-xl py-2"
                    style={{ background: "rgba(255,255,255,0.03)" }}
                  >
                    <stat.icon size={14} style={{ color: stat.color }} />
                    <span className="text-lg font-bold" style={{ color: "var(--yunque-text)" }}>
                      {stat.value}
                    </span>
                    <span className="text-[10px]" style={{ color: "var(--yunque-text-muted)" }}>
                      {stat.label}
                    </span>
                  </div>
                ))}
              </div>

              {/* Skill progress */}
              {report.topSkills.length > 0 && (
                <div>
                  <div className="text-[10px] font-semibold mb-2 uppercase tracking-wider" style={{ color: "var(--yunque-text-muted)" }}>
                    技能进度
                  </div>
                  <div className="space-y-2">
                    {report.topSkills.map((skill) => (
                      <div key={skill.name}>
                        <div className="flex items-center gap-2 mb-0.5">
                          <span className="text-[11px] font-medium" style={{ color: "var(--yunque-text-secondary)" }}>
                            {skill.name}
                          </span>
                          <Chip
                            size="sm"
                            className="text-[9px] h-4"
                            style={{ background: "var(--evo-badge-muted)", color: "var(--evo-badge)" }}
                          >
                            Lv.{skill.level}
                          </Chip>
                          {skill.recentDelta !== undefined && skill.recentDelta > 0 && (
                            <span className="text-[9px] ml-auto" style={{ color: "var(--evo-xp)" }}>
                              +{skill.recentDelta} XP
                            </span>
                          )}
                        </div>
                        <XpBar current={skill.xp} max={skill.xpToNext} />
                      </div>
                    ))}
                  </div>
                </div>
              )}

              {/* Milestones timeline */}
              {report.milestones.length > 0 && (
                <div>
                  <div className="text-[10px] font-semibold mb-2 uppercase tracking-wider" style={{ color: "var(--yunque-text-muted)" }}>
                    成就时间线
                  </div>
                  <div className="space-y-1.5">
                    {report.milestones.map((m) => {
                      const Icon = milestoneIcon[m.type];
                      const color = milestoneColor[m.type];
                      return (
                        <div
                          key={m.id}
                          className="flex items-start gap-2 rounded-lg px-2 py-1.5"
                          style={{ background: "rgba(255,255,255,0.02)" }}
                        >
                          <div
                            className="flex items-center justify-center w-5 h-5 rounded-md mt-0.5"
                            style={{ background: `${color}18` }}
                          >
                            <Icon size={11} style={{ color }} />
                          </div>
                          <div className="flex-1 min-w-0">
                            <div className="text-[11px] font-medium" style={{ color: "var(--yunque-text-secondary)" }}>
                              {m.label}
                            </div>
                            {m.description && (
                              <div className="text-[10px] mt-0.5" style={{ color: "var(--yunque-text-muted)" }}>
                                {m.description}
                              </div>
                            )}
                          </div>
                          <div className="flex items-center gap-1.5">
                            {m.xpGained !== undefined && (
                              <span className="text-[9px]" style={{ color: "var(--evo-xp)" }}>
                                +{m.xpGained} XP
                              </span>
                            )}
                            <span className="text-[9px]" style={{ color: "var(--yunque-text-muted)" }}>
                              {new Date(m.achievedAt).toLocaleDateString("zh-CN", { month: "short", day: "numeric" })}
                            </span>
                          </div>
                        </div>
                      );
                    })}
                  </div>
                </div>
              )}
            </div>
          )}
        </>
      )}
    </div>
  );
}

export default EvolutionAwareness;
