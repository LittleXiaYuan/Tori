"use client";

import { useEffect, useState } from "react";
import { Button, Chip, Disclosure, Input, Label, NumberField, Slider, Switch, TextArea, TextField } from "@heroui/react";
import { NumberValue, Segment } from "@heroui-pro/react";
import { Play, FlaskConical, Save, X } from "lucide-react";
import { CherryModal } from "@/components/cherry/overlay";
import type {
  CogniDeclaration,
  CogniHealthMetrics,
  CogniTrace,
  CogniExperienceResponse,
  CogniExperiencePattern,
  CogniWorkflowDef,
  CogniWorkflowStep,
  CogniEvolutionResponse,
  CogniExperiment,
} from "@/lib/api-types/cogni";

type DetailTab = "overview" | "config" | "logs";

// CogniSkillOption is the minimal skill shape the config form needs to offer a
// tool multi-select. It is structurally compatible with yunque-client/skills
// SkillInfo, so callers can pass the live skills list directly.
export interface CogniSkillOption {
  name: string;
  description?: string;
  category?: string;
}

// CogniConfigForm is the editable projection of a CogniDeclaration. It exposes
// only the handful of fields a non-technical user tunes; every other field on
// the declaration is preserved untouched across a save.
interface CogniConfigForm {
  displayName: string;
  description: string;
  behavior: string;
  keywords: string[];
  minScore: number;
  alwaysOn: boolean;
  surfaceOnly: string[];
  injectMemory: boolean;
  memoryTopK: number;
  priority: number;
}

function declarationToForm(d: CogniDeclaration): CogniConfigForm {
  return {
    displayName: d.display_name ?? "",
    description: d.description ?? "",
    behavior: d.context?.static ?? "",
    keywords: d.activation?.keywords ?? [],
    // min_score is omitempty on the wire, so an unset threshold arrives as
    // undefined; surface the evaluator default (0.5) instead of a bare 0.
    minScore: d.activation?.min_score ?? 0.5,
    alwaysOn: d.activation?.always_on ?? false,
    surfaceOnly: d.surface?.only ?? [],
    injectMemory: Boolean(d.context?.memory_query && d.context.memory_query.trim()),
    memoryTopK: d.context?.memory_top_k ?? 5,
    priority: d.priority ?? 0,
  };
}

// formToDeclaration merges the edited fields back onto the original declaration
// so advanced fields the form does not surface (mcp, workflows, economics, …)
// survive the round trip. List/string fields are sent verbatim — including
// empty ones — so removing a keyword or clearing text actually clears it.
function formToDeclaration(base: CogniDeclaration, f: CogniConfigForm): CogniDeclaration {
  return {
    ...base,
    id: base.id,
    display_name: f.displayName.trim(),
    description: f.description.trim(),
    priority: f.priority > 0 ? f.priority : undefined,
    activation: {
      ...(base.activation ?? {}),
      keywords: f.keywords,
      min_score: f.minScore,
      always_on: f.alwaysOn,
    },
    surface: {
      ...(base.surface ?? {}),
      only: f.surfaceOnly,
    },
    context: {
      ...(base.context ?? {}),
      static: f.behavior.trim(),
      memory_query: f.injectMemory ? (base.context?.memory_query?.trim() || "{message}") : "",
      memory_top_k: f.injectMemory ? f.memoryTopK : undefined,
    },
  };
}

export interface CapabilityDetailModalProps {
  id: string | null;
  displayName?: string;
  type?: "cogni" | "skill" | "mcp";
  onClose: () => void;

  declaration?: CogniDeclaration | null;
  health?: CogniHealthMetrics | null;
  trace?: { sessions: CogniTrace[] } | null;
  experience?: CogniExperienceResponse | null;
  evolution?: CogniEvolutionResponse | null;

  onConfirmPattern?: (patternId: string) => Promise<void>;
  onRunWorkflow?: (workflowName: string) => Promise<{ success: boolean; workflow_name?: string; error?: string }>;
  onTriggerEvolution?: () => Promise<void>;
  onStartChat?: () => void;

  // Installed skills offered as the tool surface multi-select (cogni only).
  skillOptions?: CogniSkillOption[];
  // When provided (cogni only), the Config tab becomes an editable form that
  // calls this with the merged declaration on save.
  onSaveConfig?: (declaration: CogniDeclaration) => Promise<void>;
}

function healthChipColor(status: string): "success" | "warning" | "danger" | "default" {
  if (status === "healthy") return "success";
  if (status === "idle") return "default";
  if (status === "unhealthy") return "danger";
  return "warning";
}

export function CapabilityDetailModal({
  id,
  displayName,
  type = "cogni",
  onClose,
  declaration,
  health,
  trace,
  experience,
  evolution,
  onConfirmPattern,
  onRunWorkflow,
  onTriggerEvolution,
  onStartChat,
  skillOptions,
  onSaveConfig,
}: CapabilityDetailModalProps) {
  const [activeTab, setActiveTab] = useState<DetailTab>("overview");
  const [confirmingPatternID, setConfirmingPatternID] = useState<string | null>(null);

  const [form, setForm] = useState<CogniConfigForm | null>(null);
  const [keywordDraft, setKeywordDraft] = useState("");
  const [savingConfig, setSavingConfig] = useState(false);
  const [showAdvanced, setShowAdvanced] = useState(false);
  const [showRawDecl, setShowRawDecl] = useState(false);

  const canEditConfig = type === "cogni" && !!onSaveConfig;

  // Re-seed the form whenever a different declaration loads (open another cogni,
  // or refresh after a save). Editing in place never triggers this.
  useEffect(() => {
    setForm(declaration ? declarationToForm(declaration) : null);
    setKeywordDraft("");
    setShowAdvanced(false);
    setShowRawDecl(false);
  }, [declaration]);

  const isOpen = !!id;

  // CogniExperienceResponse carries counts under summary.stats (or the flat
  // stats fallback); pending patterns live under summary.pending_patterns or
  // are derived from the unconfirmed patterns list.
  const expStats = experience?.summary?.stats ?? experience?.stats;
  const pendingPatterns =
    experience?.summary?.pending_patterns ??
    experience?.patterns?.filter((p) => !p.confirmed) ??
    [];

  function updateForm(patch: Partial<CogniConfigForm>) {
    setForm((prev) => (prev ? { ...prev, ...patch } : prev));
  }

  function addKeyword() {
    const kw = keywordDraft.trim();
    if (!kw || !form) return;
    if (!form.keywords.includes(kw)) updateForm({ keywords: [...form.keywords, kw] });
    setKeywordDraft("");
  }

  function removeKeyword(kw: string) {
    if (!form) return;
    updateForm({ keywords: form.keywords.filter((k) => k !== kw) });
  }

  function toggleSurface(name: string) {
    if (!form) return;
    updateForm({
      surfaceOnly: form.surfaceOnly.includes(name)
        ? form.surfaceOnly.filter((n) => n !== name)
        : [...form.surfaceOnly, name],
    });
  }

  async function saveConfig() {
    if (!declaration || !form || !onSaveConfig) return;
    setSavingConfig(true);
    try {
      await onSaveConfig(formToDeclaration(declaration, form));
    } finally {
      setSavingConfig(false);
    }
  }

  const labels = {
    cogni: { singular: "Cogni", chatAction: "发起对话" },
    skill: { singular: "技能", chatAction: "使用技能" },
    mcp: { singular: "工具", chatAction: "使用工具" },
  }[type];

  async function confirmPattern(patternId: string) {
    if (!onConfirmPattern) return;
    setConfirmingPatternID(patternId);
    try {
      await onConfirmPattern(patternId);
    } finally {
      setConfirmingPatternID(null);
    }
  }

  return (
    <CherryModal
      open={isOpen}
      onClose={onClose}
      size="xl"
      ariaLabel={`${displayName ?? id} 详情`}
      header={
        <div className="flex items-start justify-between gap-3 w-full">
          <div className="min-w-0 flex-1">
            <h2 className="text-lg font-semibold truncate">{displayName ?? id}</h2>
            <div className="text-xs mt-0.5 truncate" style={{ color: "var(--yunque-text-muted)" }} translate="no">
              {id}
            </div>
          </div>
          {onStartChat && (
            <Button size="sm" className="btn-accent shrink-0" onPress={onStartChat}>
              <Play size={11} aria-hidden="true" /> {labels.chatAction}
            </Button>
          )}
        </div>
      }
    >
      <div className="mb-4">
        <Segment
          size="sm"
          aria-label="详情标签"
          selectedKey={activeTab}
          onSelectionChange={(key) => { if (key) setActiveTab(String(key) as DetailTab); }}
        >
          <Segment.Item id="overview">概览</Segment.Item>
          <Segment.Item id="config">配置</Segment.Item>
          <Segment.Item id="logs">日志</Segment.Item>
        </Segment>
      </div>

      {activeTab === "overview" && (
        <div className="space-y-6">
          {health && (
            <div>
              <div className="flex items-center justify-between mb-3">
                <div className="text-xs font-medium" style={{ color: "var(--yunque-text-muted)" }}>
                  运行状态
                </div>
                <Chip size="sm" color={healthChipColor(health.status ?? "idle")}>
                  {health.status ?? "未激活"}
                </Chip>
              </div>
              <div className="grid grid-cols-3 gap-4">
                <div>
                  <NumberValue className="block text-2xl font-semibold" value={health.score ?? 0} maximumFractionDigits={2} />
                  <div className="text-xs" style={{ color: "var(--yunque-text-muted)" }}>评分</div>
                </div>
                <div>
                  <NumberValue className="block text-2xl font-semibold" value={health.evaluations ?? 0} maximumFractionDigits={0} />
                  <div className="text-xs" style={{ color: "var(--yunque-text-muted)" }}>评估</div>
                </div>
                <div>
                  <NumberValue className="block text-2xl font-semibold" value={health.activations ?? 0} maximumFractionDigits={0} />
                  <div className="text-xs" style={{ color: "var(--yunque-text-muted)" }}>激活</div>
                </div>
              </div>
            </div>
          )}

          {experience && (
            <div>
              <div className="text-xs font-medium mb-2" style={{ color: "var(--yunque-text-muted)" }}>经验</div>
              <div className="grid grid-cols-3 gap-4">
                <div>
                  <NumberValue className="block text-2xl font-semibold" value={expStats?.domain_facts ?? 0} maximumFractionDigits={0} />
                  <div className="text-xs" style={{ color: "var(--yunque-text-muted)" }}>领域事实</div>
                </div>
                <div>
                  <NumberValue className="block text-2xl font-semibold" value={expStats?.patterns_confirmed ?? 0} maximumFractionDigits={0} />
                  <div className="text-xs" style={{ color: "var(--yunque-text-muted)" }}>已确认模式</div>
                </div>
                <div>
                  <NumberValue className="block text-2xl font-semibold" value={expStats?.tool_memories ?? 0} maximumFractionDigits={0} />
                  <div className="text-xs" style={{ color: "var(--yunque-text-muted)" }}>工具记忆</div>
                </div>
              </div>

              {pendingPatterns.length > 0 && (
                <div className="mt-3">
                  <div className="text-xs font-medium mb-1.5" style={{ color: "var(--yunque-text-muted)" }}>
                    待确认模式
                  </div>
                  {pendingPatterns.map((p: CogniExperiencePattern, i: number) => (
                    <div
                      key={p.id || i}
                      className="text-xs p-2 rounded mb-1"
                      style={{ background: "var(--yunque-bg-muted)", color: "var(--yunque-text-muted)" }}
                    >
                      <div className="flex items-start justify-between gap-2">
                        <span>{p.trigger} → {p.response}</span>
                        {p.id && onConfirmPattern && (
                          <Button
                            size="sm"
                            variant="ghost"
                            isPending={confirmingPatternID === p.id}
                            isDisabled={!!confirmingPatternID && confirmingPatternID !== p.id}
                            onPress={() => p.id && confirmPattern(p.id)}
                          >
                            {confirmingPatternID === p.id ? "确认中" : "确认"}
                          </Button>
                        )}
                      </div>
                    </div>
                  ))}
                </div>
              )}
            </div>
          )}

          {declaration?.workflows && declaration.workflows.length > 0 && (
            <div>
              <div className="text-xs font-medium mb-2" style={{ color: "var(--yunque-text-muted)" }}>
                工作流
              </div>
              <div className="space-y-2">
                {declaration.workflows.map((wf: CogniWorkflowDef) => (
                  <div key={wf.name} className="p-2.5 rounded-lg" style={{ background: "var(--yunque-bg-muted)" }}>
                    <div className="flex items-center justify-between mb-0.5">
                      <span className="text-xs font-medium" style={{ color: "var(--yunque-text)" }}>
                        {wf.name}
                      </span>
                      {onRunWorkflow && (
                        <Button
                          size="sm"
                          variant="ghost"
                          onPress={() => onRunWorkflow(wf.name)}
                        >
                          <Play size={10} aria-hidden="true" /> 执行
                        </Button>
                      )}
                    </div>
                    {wf.description && (
                      <div className="text-xs" style={{ color: "var(--yunque-text-muted)" }}>
                        {wf.description}
                      </div>
                    )}
                    {wf.steps && wf.steps.length > 0 && (
                      <div className="text-xs mt-1" style={{ color: "var(--yunque-text-muted)" }}>
                        {wf.steps.map((s: CogniWorkflowStep, i: number) => (
                          <span key={i}>{i > 0 && " · "}{s.skill}</span>
                        ))}
                      </div>
                    )}
                  </div>
                ))}
              </div>
            </div>
          )}

          {evolution && (
            <div>
              <div className="flex items-center justify-between mb-2">
                <div className="text-xs font-medium" style={{ color: "var(--yunque-text-muted)" }}>
                  <FlaskConical size={12} className="inline mr-1" aria-hidden="true" />
                  技能进化
                </div>
                {onTriggerEvolution && (
                  <Button size="sm" variant="ghost" onPress={onTriggerEvolution}>
                    <FlaskConical size={10} aria-hidden="true" /> 触发进化
                  </Button>
                )}
              </div>
              {!evolution.experiments || evolution.experiments.length === 0 ? (
                <div className="text-sm py-2 text-center" style={{ color: "var(--yunque-text-muted)" }}>
                  尚无进化实验记录
                </div>
              ) : (
                <div className="space-y-1.5">
                  {evolution.experiments.map((exp: CogniExperiment) => (
                    <div
                      key={exp.id}
                      className="p-2.5 rounded-lg text-xs"
                      style={{ background: "var(--yunque-bg-muted)" }}
                    >
                      <div className="flex items-center gap-2 mb-0.5">
                        <Chip size="sm" color={exp.status === "kept" ? "success" : "danger"}>
                          {exp.status === "kept" ? "保留" : "回滚"}
                        </Chip>
                        <span
                          className="tabular-nums"
                          style={{
                            color: exp.delta >= 0 ? "var(--yunque-success)" : "var(--yunque-danger)",
                          }}
                        >
                          {exp.delta >= 0 ? "+" : ""}{exp.delta.toFixed(1)}%
                        </span>
                      </div>
                      <div style={{ color: "var(--yunque-text-muted)" }}>{exp.change}</div>
                    </div>
                  ))}
                </div>
              )}
            </div>
          )}
        </div>
      )}

      {activeTab === "config" && (
        <div className="space-y-5">
          {!declaration ? (
            <div className="text-sm py-6 text-center" style={{ color: "var(--yunque-text-muted)" }}>
              加载配置中…
            </div>
          ) : canEditConfig && form ? (
            <>
              {/* Basics */}
              <div className="space-y-3">
                <TextField value={form.displayName} onChange={(v) => updateForm({ displayName: v })} aria-label="显示名称">
                  <Label className="text-xs font-medium" style={{ color: "var(--yunque-text-muted)" }}>显示名称</Label>
                  <Input placeholder="给这个 Cogni 起个名字" />
                </TextField>
                <div>
                  <Label className="text-xs font-medium block mb-1" style={{ color: "var(--yunque-text-muted)" }}>一句话描述</Label>
                  <TextArea
                    aria-label="一句话描述"
                    value={form.description}
                    onChange={(e) => updateForm({ description: e.target.value })}
                    rows={2}
                    placeholder="它是做什么的"
                    fullWidth
                  />
                </div>
              </div>

              {/* Activation */}
              <div className="space-y-3">
                <div className="text-xs font-semibold" style={{ color: "var(--yunque-text)" }}>激活条件</div>
                <div>
                  <Label className="text-xs font-medium block mb-1.5" style={{ color: "var(--yunque-text-muted)" }}>触发关键词</Label>
                  <div className="flex flex-wrap gap-1 mb-2">
                    {form.keywords.length === 0 && (
                      <span className="text-xs" style={{ color: "var(--yunque-text-muted)" }}>暂无关键词</span>
                    )}
                    {form.keywords.map((kw) => (
                      <Chip key={kw} size="sm" color="default">
                        <span className="inline-flex items-center gap-1">
                          {kw}
                          <button
                            type="button"
                            onClick={() => removeKeyword(kw)}
                            aria-label={`移除关键词 ${kw}`}
                            className="opacity-60 hover:opacity-100"
                          >
                            <X size={11} aria-hidden="true" />
                          </button>
                        </span>
                      </Chip>
                    ))}
                  </div>
                  <form onSubmit={(e) => { e.preventDefault(); addKeyword(); }} className="flex gap-2">
                    <TextField value={keywordDraft} onChange={setKeywordDraft} className="flex-1" aria-label="新增关键词">
                      <Input placeholder="输入关键词，回车添加" />
                    </TextField>
                    <Button type="submit" size="sm" variant="outline">添加</Button>
                  </form>
                </div>

                <Slider
                  value={Math.round(form.minScore * 100)}
                  onChange={(v) => updateForm({ minScore: (Array.isArray(v) ? v[0] : v) / 100 })}
                  minValue={0}
                  maxValue={100}
                  step={5}
                  className="w-full"
                >
                  <div className="flex items-center justify-between">
                    <Label className="text-xs font-medium" style={{ color: "var(--yunque-text-muted)" }}>最低匹配分</Label>
                    <Slider.Output className="text-xs tabular-nums" style={{ color: "var(--yunque-text-muted)" }} />
                  </div>
                  <Slider.Track className="h-1.5 rounded-full mt-1" style={{ background: "var(--yunque-bg-muted)" }}>
                    <Slider.Fill className="rounded-full" style={{ background: "var(--yunque-accent)" }} />
                    <Slider.Thumb className="size-4 rounded-full border-2" style={{ borderColor: "var(--yunque-surface-1)", background: "var(--yunque-accent)" }} />
                  </Slider.Track>
                </Slider>
                <p className="text-[11px]" style={{ color: "var(--yunque-text-muted)" }}>分数越高越严格，越不容易被触发；开启「常驻」后此项忽略。</p>

                <div className="flex items-center justify-between">
                  <div>
                    <div className="text-xs font-medium" style={{ color: "var(--yunque-text)" }}>常驻激活</div>
                    <div className="text-[11px]" style={{ color: "var(--yunque-text-muted)" }}>无视关键词，始终参与（适合人设、护栏类）。</div>
                  </div>
                  <Switch isSelected={form.alwaysOn} onChange={(v) => updateForm({ alwaysOn: v })} size="sm" aria-label="常驻激活">
                    <Switch.Control><Switch.Thumb /></Switch.Control>
                  </Switch>
                </div>
              </div>

              {/* Tool surface */}
              <div className="space-y-2">
                <div className="text-xs font-semibold" style={{ color: "var(--yunque-text)" }}>能力范围</div>
                <p className="text-[11px]" style={{ color: "var(--yunque-text-muted)" }}>勾选它激活时该用哪些工具；不选则默认可用全部。</p>
                {skillOptions && skillOptions.length > 0 ? (
                  <div className="flex flex-wrap gap-1.5 max-h-44 overflow-y-auto" style={{ overscrollBehavior: "contain" }}>
                    {skillOptions.map((s) => {
                      const selected = form.surfaceOnly.includes(s.name);
                      return (
                        <Chip
                          key={s.name}
                          size="sm"
                          color={selected ? "success" : "default"}
                          className="cursor-pointer"
                          onClick={() => toggleSurface(s.name)}
                          title={s.description}
                        >
                          {s.name}
                        </Chip>
                      );
                    })}
                  </div>
                ) : (
                  <div className="text-xs" style={{ color: "var(--yunque-text-muted)" }}>未检测到可用技能；可在「能力」页安装后再来勾选。</div>
                )}
                {form.surfaceOnly.length > 0 && (
                  <div className="text-[11px] tabular-nums" style={{ color: "var(--yunque-text-muted)" }}>已选 {form.surfaceOnly.length} 项</div>
                )}
              </div>

              {/* Context & behavior */}
              <div className="space-y-3">
                <div className="text-xs font-semibold" style={{ color: "var(--yunque-text)" }}>上下文与行为</div>
                <div>
                  <Label className="text-xs font-medium block mb-1" style={{ color: "var(--yunque-text-muted)" }}>行为指导（写给智能体的话）</Label>
                  <TextArea
                    aria-label="行为指导"
                    value={form.behavior}
                    onChange={(e) => updateForm({ behavior: e.target.value })}
                    rows={4}
                    placeholder="例如：你负责审查代码质量、安全风险和测试缺口，回答要给出可执行的修改建议。"
                    fullWidth
                  />
                </div>
                <div className="flex items-center justify-between">
                  <div>
                    <div className="text-xs font-medium" style={{ color: "var(--yunque-text)" }}>注入相关记忆</div>
                    <div className="text-[11px]" style={{ color: "var(--yunque-text-muted)" }}>激活时自动带上与当前对话相关的记忆。</div>
                  </div>
                  <Switch isSelected={form.injectMemory} onChange={(v) => updateForm({ injectMemory: v })} size="sm" aria-label="注入相关记忆">
                    <Switch.Control><Switch.Thumb /></Switch.Control>
                  </Switch>
                </div>
                {form.injectMemory && (
                  <Slider
                    value={form.memoryTopK}
                    onChange={(v) => updateForm({ memoryTopK: Array.isArray(v) ? v[0] : v })}
                    minValue={1}
                    maxValue={20}
                    step={1}
                    className="w-full"
                  >
                    <div className="flex items-center justify-between">
                      <Label className="text-xs font-medium" style={{ color: "var(--yunque-text-muted)" }}>注入条数</Label>
                      <Slider.Output className="text-xs tabular-nums" style={{ color: "var(--yunque-text-muted)" }} />
                    </div>
                    <Slider.Track className="h-1.5 rounded-full mt-1" style={{ background: "var(--yunque-bg-muted)" }}>
                      <Slider.Fill className="rounded-full" style={{ background: "var(--yunque-accent)" }} />
                      <Slider.Thumb className="size-4 rounded-full border-2" style={{ borderColor: "var(--yunque-surface-1)", background: "var(--yunque-accent)" }} />
                    </Slider.Track>
                  </Slider>
                )}
              </div>

              {/* Advanced */}
              <Disclosure isExpanded={showAdvanced} onExpandedChange={setShowAdvanced}>
                <Disclosure.Heading>
                  <Button slot="trigger" variant="ghost" size="sm" className="px-1 text-xs" style={{ color: "var(--yunque-text-muted)" }}>
                    高级
                    <Disclosure.Indicator />
                  </Button>
                </Disclosure.Heading>
                <Disclosure.Content>
                  <Disclosure.Body className="pt-2">
                    <NumberField
                      value={form.priority > 0 ? form.priority : undefined}
                      onChange={(v) => updateForm({ priority: v ?? 0 })}
                      minValue={0}
                      aria-label="优先级"
                      className="max-w-[12rem]"
                    >
                      <Label className="text-xs font-medium" style={{ color: "var(--yunque-text-muted)" }}>优先级（数字越小越优先，留空用默认）</Label>
                      <NumberField.Group>
                        <NumberField.DecrementButton />
                        <NumberField.Input placeholder="100" />
                        <NumberField.IncrementButton />
                      </NumberField.Group>
                    </NumberField>
                  </Disclosure.Body>
                </Disclosure.Content>
              </Disclosure>

              {/* Raw declaration escape hatch */}
              <Disclosure isExpanded={showRawDecl} onExpandedChange={setShowRawDecl}>
                <Disclosure.Heading>
                  <Button slot="trigger" variant="ghost" size="sm" className="px-1 text-xs" style={{ color: "var(--yunque-text-muted)" }}>
                    查看原始声明（只读）
                    <Disclosure.Indicator />
                  </Button>
                </Disclosure.Heading>
                <Disclosure.Content>
                  <Disclosure.Body className="pt-2">
                    <pre
                      className="p-3 rounded-lg text-[11px] overflow-x-auto"
                      style={{ background: "var(--yunque-bg-muted)", color: "var(--yunque-text-muted)" }}
                      translate="no"
                    >
                      {JSON.stringify(formToDeclaration(declaration, form), null, 2)}
                    </pre>
                  </Disclosure.Body>
                </Disclosure.Content>
              </Disclosure>

              {/* Save */}
              <div className="flex justify-end gap-2 pt-3 border-t" style={{ borderColor: "var(--yunque-border)" }}>
                <Button size="sm" variant="ghost" onPress={onClose}>取消</Button>
                <Button size="sm" className="btn-accent" isPending={savingConfig} onPress={saveConfig}>
                  <Save size={13} aria-hidden="true" /> 保存配置
                </Button>
              </div>
            </>
          ) : (
            <>
              {declaration.activation?.keywords && declaration.activation.keywords.length > 0 && (
                <div>
                  <div className="text-xs font-medium mb-1.5" style={{ color: "var(--yunque-text-muted)" }}>
                    激活关键词
                  </div>
                  <div className="flex flex-wrap gap-1">
                    {declaration.activation.keywords.map((kw) => (
                      <Chip key={kw} size="sm" color="default">{kw}</Chip>
                    ))}
                  </div>
                </div>
              )}

              {(declaration.surface?.only || declaration.surface?.include || declaration.surface?.exclude) && (
                <div>
                  <div className="text-xs font-medium mb-1" style={{ color: "var(--yunque-text-muted)" }}>
                    工具过滤
                  </div>
                  {declaration.surface?.only && (
                    <div className="text-xs mb-1" style={{ color: "var(--yunque-text-muted)" }}>
                      仅限：{declaration.surface.only.join("、")}
                    </div>
                  )}
                  {declaration.surface?.include && (
                    <div className="text-xs mb-1" style={{ color: "var(--yunque-text-muted)" }}>
                      包含：{declaration.surface.include.join("、")}
                    </div>
                  )}
                  {declaration.surface?.exclude && (
                    <div className="text-xs mb-1" style={{ color: "var(--yunque-text-muted)" }}>
                      排除：{declaration.surface.exclude.join("、")}
                    </div>
                  )}
                </div>
              )}

              {declaration.description && (
                <div>
                  <div className="text-xs font-medium mb-1" style={{ color: "var(--yunque-text-muted)" }}>
                    描述
                  </div>
                  <div className="text-sm" style={{ color: "var(--yunque-text)" }}>
                    {declaration.description}
                  </div>
                </div>
              )}
            </>
          )}
        </div>
      )}

      {activeTab === "logs" && (
        <div>
          {!trace || !trace.sessions || trace.sessions.length === 0 ? (
            <div className="text-sm py-6 text-center" style={{ color: "var(--yunque-text-muted)" }}>
              暂无运行轨迹
            </div>
          ) : (
            <div className="space-y-2">
              {trace.sessions.map((s, i) => {
                const activated = (s.activations ?? []).filter((a) => a.activated);
                return (
                  <div
                    key={`${s.timestamp}-${i}`}
                    className="p-2.5 rounded-lg text-xs"
                    style={{ background: "var(--yunque-bg-muted)" }}
                  >
                    <div className="flex items-center justify-between mb-1">
                      <span className="font-mono" style={{ color: "var(--yunque-text-muted)" }}>
                        {s.timestamp ? new Date(s.timestamp).toLocaleString() : `#${i + 1}`}
                      </span>
                      <Chip size="sm" color={activated.length > 0 ? "success" : "default"}>
                        {activated.length > 0 ? `激活 ${activated.length}` : "未激活"} · {s.duration_ms}ms
                      </Chip>
                    </div>
                    {(s.activations ?? []).length > 0 && (
                      <div className="flex flex-wrap gap-1">
                        {s.activations!.map((a) => (
                          <Chip key={a.id} size="sm" color={a.activated ? "success" : "default"}>
                            {(a.display_name ?? a.id)} · {a.score.toFixed(2)}
                          </Chip>
                        ))}
                      </div>
                    )}
                  </div>
                );
              })}
            </div>
          )}
        </div>
      )}
    </CherryModal>
  );
}
