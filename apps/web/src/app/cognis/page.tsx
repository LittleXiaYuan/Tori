"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { useSearchParams } from "next/navigation";
import {
  createCogniKernelPackClient,
  type CogniRuntimePackStateReport,
} from "@/lib/cogni-kernel-pack-client";
import type {
  CogniAlert,
  CogniDeclaration,
  CogniEntryStatus,
  CogniEvolutionResponse,
  CogniExperiencePattern,
  CogniExperienceResponse,
  CogniHealthMetrics,
  CogniTrace,
  CogniWorkflowDef,
  CogniWorkflowStep,
  CogniExperiment,
} from "@/lib/api-types/cogni";
import { Button, Card, Chip, Switch } from "@heroui/react";
import {
  AlertTriangle,
  CheckCircle2,
  ChevronDown,
  Download,
  FlaskConical,
  Lightbulb,
  Link as LinkIcon,
  Play,
  Power,
  RefreshCw,
  Search,
  Share2,
  ShieldCheck,
  Sparkles,
  Trash2,
  Upload,
  Wand2,
  Workflow,
} from "lucide-react";
import PageHeader from "@/components/page-header";
import { showToast } from "@/components/toast-provider";
import { formatErrorMessage } from "@/lib/error-utils";
import { createCognisClient } from "yunque-client/cognis";
import { createYunqueSDKClientOptions } from "@/lib/sdk-client";
import { CherryModal } from "@/components/cherry/overlay";

type HealthMap = Record<string, CogniHealthMetrics>;

const cogniPack = createCogniKernelPackClient();
// Only used for id-filtered export (per-assistant share); hits the same /v1/cognis API.
const cognisClient = createCognisClient(createYunqueSDKClientOptions());

const ASSISTANT_EXAMPLES = [
  "一个帮我整理每周工作周报、能查资料还能做成 PPT 的助手",
  "一个专门审查代码、盯安全漏洞和风格问题的助手",
  "一个帮我做数据分析、把表格画成图表的助手",
  "一个回复客户咨询、语气亲切专业的客服助手",
];

interface TemplateMetadata {
  id: string;
  display_name: string;
  description: string;
  category: string;
}

const BUILTIN_TEMPLATES: TemplateMetadata[] = [
  { id: "code-reviewer", display_name: "代码审查助手", description: "审查代码质量、发现潜在问题、给出改进建议", category: "开发" },
  { id: "data-analyst", display_name: "数据分析助手", description: "分析数据、生成报表、把结果画成图表", category: "数据" },
  { id: "doc-generator", display_name: "文档生成助手", description: "生成技术文档、API 文档与用户手册", category: "文档" },
  { id: "monitor-alert", display_name: "监控告警助手", description: "监控系统状态、分析日志、发送告警", category: "运维" },
  { id: "task-scheduler", display_name: "任务调度助手", description: "智能调度任务、管理优先级、自动执行", category: "效率" },
];

function healthColor(status: string): { bg: string; fg: string } {
  switch (status) {
    case "healthy":
      return { bg: "rgba(23,201,100,0.12)", fg: "#17c964" };
    case "warn":
      return { bg: "rgba(255,170,0,0.12)", fg: "#ffaa00" };
    case "unhealthy":
      return { bg: "rgba(243,18,96,0.12)", fg: "#f31260" };
    default:
      return { bg: "rgba(255,255,255,0.04)", fg: "var(--yunque-text-muted)" };
  }
}

function severityColor(sev: string): { bg: string; fg: string } {
  switch (sev) {
    case "critical":
      return { bg: "rgba(243,18,96,0.15)", fg: "#f31260" };
    case "warn":
      return { bg: "rgba(255,170,0,0.15)", fg: "#ffaa00" };
    default:
      return { bg: "rgba(0,145,255,0.15)", fg: "#0091ff" };
  }
}

// Deterministic avatar so each assistant gets a stable, distinct colour.
function avatarGradient(seed: string): string {
  let h = 0;
  for (let i = 0; i < seed.length; i++) h = (h * 31 + seed.charCodeAt(i)) % 360;
  return `linear-gradient(135deg, hsl(${h}, 62%, 55%), hsl(${(h + 38) % 360}, 62%, 46%))`;
}

function avatarInitial(name: string): string {
  const s = name.trim();
  if (!s) return "·";
  const ch = s[0];
  return /[a-z]/i.test(ch) ? ch.toUpperCase() : ch;
}

export default function CognisPage() {
  const searchParams = useSearchParams();
  const [cognis, setCognis] = useState<CogniEntryStatus[]>([]);
  const [health, setHealth] = useState<HealthMap>({});
  const [alerts, setAlerts] = useState<CogniAlert[]>([]);
  const [filter, setFilter] = useState("");
  const [loading, setLoading] = useState(true);
  const [packOff, setPackOff] = useState(false);
  const [busy, setBusy] = useState<string | null>(null);

  const [generateDesc, setGenerateDesc] = useState("");
  const [generating, setGenerating] = useState(false);
  const [generatePreview, setGeneratePreview] = useState<CogniDeclaration | null>(null);
  const heroInputRef = useRef<HTMLTextAreaElement>(null);

  const [showTemplates, setShowTemplates] = useState(false);
  const [advancedOpen, setAdvancedOpen] = useState(false);
  const [runtimePackState, setRuntimePackState] = useState<CogniRuntimePackStateReport | null>(null);
  const fileInput = useRef<HTMLInputElement>(null);

  // Share + import-preview modals (ported from the old market page).
  const [shareID, setShareID] = useState<string | null>(null);
  const [importPreview, setImportPreview] = useState<Record<string, unknown> | null>(null);

  // Detail drawer.
  const [detailID, setDetailID] = useState<string | null>(null);
  const [detailTab, setDetailTab] = useState<"traces" | "workflows" | "experience" | "evolution">("traces");
  const [detailTraces, setDetailTraces] = useState<CogniTrace[]>([]);
  const [detailWorkflows, setDetailWorkflows] = useState<CogniWorkflowDef[]>([]);
  const [detailExperience, setDetailExperience] = useState<CogniExperienceResponse | null>(null);
  const [detailEvolution, setDetailEvolution] = useState<CogniEvolutionResponse | null>(null);
  const [confirmingPatternID, setConfirmingPatternID] = useState<string | null>(null);

  const focusHeroCreate = useCallback(() => {
    heroInputRef.current?.focus();
    heroInputRef.current?.scrollIntoView({ behavior: "smooth", block: "center" });
  }, []);

  const load = useCallback(async () => {
    try {
      const list = await cogniPack.list();
      setPackOff(false);
      setCognis(list.cognis || []);
      setHealth(list.health || {});
      const [alertsRes, runtimeStateRes] = await Promise.all([
        cogniPack.alerts().catch(() => ({ alerts: [] as CogniAlert[], count: 0 })),
        cogniPack.runtimePackState().catch(() => null),
      ]);
      setAlerts(alertsRes.alerts || []);
      setRuntimePackState(runtimeStateRes);
    } catch {
      // The /v1/cognis routes are gated by the cogni-kernel pack; a failure here
      // almost always means the pack is disabled. Degrade gracefully instead of
      // showing a broken page.
      setPackOff(true);
      setCognis([]);
      setHealth({});
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    load();
  }, [load]);

  // Inbound share link: /cognis?bundle=<base64>
  useEffect(() => {
    const bundleParam = searchParams.get("bundle");
    if (!bundleParam) return;
    try {
      const decoded = JSON.parse(atob(bundleParam));
      setImportPreview(decoded);
    } catch {
      showToast("无效的分享链接", "error");
    }
  }, [searchParams]);

  const installedIDs = useMemo(() => new Set(cognis.map((c) => c.id)), [cognis]);

  const filteredCognis = useMemo(
    () =>
      cognis.filter(
        (c) =>
          !filter ||
          c.id.toLowerCase().includes(filter.toLowerCase()) ||
          (c.display_name ?? "").toLowerCase().includes(filter.toLowerCase()),
      ),
    [cognis, filter],
  );

  const generateCogni = async () => {
    if (!generateDesc.trim()) return;
    setGenerating(true);
    try {
      const r = await cogniPack.generate(generateDesc, true);
      setGeneratePreview(r.declaration);
      showToast(`Cogni「${r.declaration.display_name ?? r.declaration.id}」已创建`, "success");
      setGenerateDesc("");
      await load();
    } catch (e) {
      showToast(formatErrorMessage(e, "创建失败，请换个说法再试"), "error");
    } finally {
      setGenerating(false);
    }
  };

  const installTemplate = async (templateId: string) => {
    setBusy(`install:${templateId}`);
    try {
      const response = await fetch(`/data/cogni/templates/${templateId}.json`);
      if (!response.ok) throw new Error("模板文件不存在");
      const declaration = await response.json();
      await cogniPack.add(declaration);
      showToast("Cogni 已添加", "success");
      await load();
    } catch (e) {
      showToast(formatErrorMessage(e, "添加失败"), "error");
    } finally {
      setBusy(null);
    }
  };

  const toggle = async (id: string, enabled: boolean) => {
    setCognis((prev) => prev.map((c) => (c.id === id ? { ...c, enabled } : c)));
    try {
      await cogniPack.setEnabled(id, enabled);
    } catch (e) {
      setCognis((prev) => prev.map((c) => (c.id === id ? { ...c, enabled: !enabled } : c)));
      showToast(formatErrorMessage(e, "操作失败"), "error");
    }
  };

  const remove = async (id: string) => {
    if (!confirm(`确定删除 Cogni「${id}」？此操作不可撤销。`)) return;
    setBusy(`remove:${id}`);
    try {
      await cogniPack.remove(id);
      showToast("已删除", "success");
      await load();
    } catch (e) {
      showToast(formatErrorMessage(e, "删除失败"), "error");
    } finally {
      setBusy(null);
    }
  };

  const importBundleFile = async (file: File) => {
    try {
      const bundle = JSON.parse(await file.text());
      const sum = await cogniPack.importBundle(bundle);
      showToast(
        `导入：新增 ${sum.added?.length || 0} · 更新 ${sum.updated?.length || 0} · 跳过 ${sum.skipped?.length || 0} · 失败 ${sum.failed?.length || 0}`,
        "success",
      );
      await load();
    } catch (e) {
      showToast(formatErrorMessage(e, "导入失败"), "error");
    }
  };

  const importFromPreview = async () => {
    if (!importPreview) return;
    try {
      await cogniPack.importBundle(importPreview);
      showToast("导入成功", "success");
      setImportPreview(null);
      window.history.replaceState({}, "", "/cognis");
      await load();
    } catch (e) {
      showToast(formatErrorMessage(e, "导入失败"), "error");
    }
  };

  const copyShareLink = async () => {
    if (!shareID) return;
    try {
      const bundle = await cognisClient.exportBundle([shareID]);
      const shareUrl = `${window.location.origin}/cognis?bundle=${btoa(JSON.stringify(bundle))}`;
      await navigator.clipboard.writeText(shareUrl);
      showToast("分享链接已复制", "success");
      setShareID(null);
    } catch (e) {
      showToast(formatErrorMessage(e, "生成分享链接失败"), "error");
    }
  };

  const downloadShare = async () => {
    if (!shareID) return;
    try {
      const bundle = await cognisClient.exportBundle([shareID]);
      const url = URL.createObjectURL(new Blob([JSON.stringify(bundle, null, 2)], { type: "application/json" }));
      const a = document.createElement("a");
      a.href = url;
      a.download = `cogni-${shareID}.json`;
      a.click();
      URL.revokeObjectURL(url);
      setShareID(null);
    } catch (e) {
      showToast(formatErrorMessage(e, "下载失败"), "error");
    }
  };

  const openDetail = async (id: string) => {
    setDetailID(id);
    setDetailTab("traces");
    const [traces, workflows, experience, evolution] = await Promise.all([
      cogniPack.tracesByID(id, 20).catch(() => ({ traces: [] as CogniTrace[] })),
      cogniPack.workflows(id).catch(() => ({ workflows: [] as CogniWorkflowDef[] })),
      cogniPack.experience(id).catch(() => null),
      cogniPack.evolution(id).catch(() => null),
    ]);
    setDetailTraces(traces.traces || []);
    setDetailWorkflows(workflows.workflows || []);
    setDetailExperience(experience);
    setDetailEvolution(evolution);
  };

  const confirmExperiencePattern = async (patternID: string) => {
    if (!detailID || confirmingPatternID) return;
    setConfirmingPatternID(patternID);
    try {
      await cogniPack.confirmExperiencePattern(detailID, patternID);
      setDetailExperience(await cogniPack.experience(detailID));
      showToast("经验模式已确认", "success");
    } catch (e) {
      showToast(formatErrorMessage(e, "确认失败"), "error");
    } finally {
      setConfirmingPatternID(null);
    }
  };

  const experienceSummary = detailExperience?.summary;
  const experienceStats = experienceSummary?.stats ?? detailExperience?.stats ?? {};
  const topTools = experienceSummary?.top_tools ?? detailExperience?.tool_memory?.slice(0, 5) ?? [];
  const topFacts = experienceSummary?.top_facts ?? detailExperience?.domain_facts?.slice(0, 5) ?? [];
  const pendingPatterns =
    experienceSummary?.pending_patterns ?? detailExperience?.patterns?.filter((p) => !p.confirmed).slice(0, 5) ?? [];

  return (
    <div className="page-root flex flex-col gap-5 animate-fade-in-up">
      <PageHeader
        icon={<Sparkles size={20} aria-hidden="true" />}
        title="Cogni"
        description="用大白话描述你想要的 Cogni，云雀自动配好技能、触发方式和人设。"
        onRefresh={load}
        actions={
          <Button size="sm" className="btn-accent" onPress={focusHeroCreate}>
            <Wand2 size={12} aria-hidden="true" /> 新建 Cogni
          </Button>
        }
      />

      {packOff && (
        <Card className="section-card p-4 border-l-4" style={{ borderLeftColor: "#ffaa00" }}>
          <div className="flex items-center gap-2 text-sm" style={{ color: "var(--yunque-text)" }}>
            <AlertTriangle size={15} style={{ color: "#ffaa00" }} aria-hidden="true" />
            Cogni 功能由「Cogni 内核」能力包提供，当前未启用。
          </div>
          <p className="text-xs mt-1" style={{ color: "var(--yunque-text-muted)" }}>
            前往「能力包」启用 <span translate="no">yunque.pack.cogni-kernel</span> 后即可创建和管理 Cogni。
          </p>
          <div className="mt-3">
            <Button size="sm" variant="ghost" onPress={() => (window.location.href = "/packs")}>
              去启用
            </Button>
          </div>
        </Card>
      )}

      {/* Hero — natural-language assistant creation */}
      <Card className="section-card p-5" style={{ borderTop: "2px solid var(--yunque-accent)" }}>
        <div className="flex items-center gap-2 mb-1" style={{ color: "var(--yunque-text)" }}>
          <Wand2 size={16} style={{ color: "var(--yunque-accent)" }} aria-hidden="true" />
          <span className="text-base font-medium">描述你想要的 Cogni，云雀帮你造一个</span>
        </div>
        <p className="text-xs mb-3" style={{ color: "var(--yunque-text-muted)" }}>
          一句话说清它要做什么 —— 云雀会自动配好该用的技能、激活关键词和说话风格，创建后立即可用。
        </p>
        <label htmlFor="assistant-desc" className="sr-only">
          描述你想要的 Cogni
        </label>
        <textarea
          id="assistant-desc"
          ref={heroInputRef}
          value={generateDesc}
          onChange={(e) => setGenerateDesc(e.target.value)}
          placeholder="例如：一个帮我整理周报、能查资料还能做成 PPT 的助手…"
          rows={3}
          disabled={packOff}
          className="w-full p-3 text-sm rounded-lg"
          style={{
            background: "rgba(255,255,255,0.04)",
            border: "1px solid rgba(255,255,255,0.08)",
            color: "var(--yunque-text)",
            resize: "vertical",
          }}
        />
        <div className="flex flex-wrap items-center gap-1.5 mt-2">
          <span className="text-[11px] flex items-center gap-1" style={{ color: "var(--yunque-text-muted)" }}>
            <Lightbulb size={11} aria-hidden="true" /> 试试：
          </span>
          {ASSISTANT_EXAMPLES.map((ex) => (
            <button
              key={ex}
              type="button"
              onClick={() => setGenerateDesc(ex)}
              className="text-[11px] px-2 py-1 rounded-full transition-colors hover:opacity-80 focus-visible:ring-2"
              style={{
                background: "rgba(255,255,255,0.04)",
                border: "1px solid rgba(255,255,255,0.08)",
                color: "var(--yunque-text-muted)",
              }}
            >
              {ex.length > 16 ? ex.slice(0, 16) + "…" : ex}
            </button>
          ))}
        </div>
        <div className="flex justify-end mt-3">
          <Button
            size="sm"
            className="btn-accent"
            onPress={generateCogni}
            isPending={generating}
            isDisabled={packOff || !generateDesc.trim()}
          >
            <Sparkles size={12} aria-hidden="true" /> {generating ? "云雀正在造 Cogni…" : "创建 Cogni"}
          </Button>
        </div>
        {generatePreview && (
          <div
            className="mt-4 p-3 rounded-lg"
            style={{ background: "rgba(23,201,100,0.08)", border: "1px solid rgba(23,201,100,0.2)" }}
          >
            <div className="flex items-center gap-2 mb-1">
              <CheckCircle2 size={14} style={{ color: "#17c964" }} aria-hidden="true" />
              <span className="text-sm font-medium" style={{ color: "var(--yunque-text)" }}>
                已为你创建：{generatePreview.display_name ?? generatePreview.id}
              </span>
            </div>
            {generatePreview.description && (
              <p className="text-xs" style={{ color: "var(--yunque-text-muted)" }}>
                {generatePreview.description}
              </p>
            )}
          </div>
        )}
      </Card>

      {/* Templates — quick start from a builtin assistant */}
      <div>
        <button
          type="button"
          onClick={() => setShowTemplates((v) => !v)}
          className="flex items-center gap-1.5 text-sm mb-2"
          style={{ color: "var(--yunque-text)" }}
        >
          <ChevronDown
            size={14}
            aria-hidden="true"
            style={{ transform: showTemplates ? "none" : "rotate(-90deg)", transition: "transform .15s" }}
          />
          从模板快速创建
        </button>
        {showTemplates && (
          <div className="grid grid-cols-1 md:grid-cols-2 gap-3">
            {BUILTIN_TEMPLATES.map((t) => {
              const installed = installedIDs.has(t.id);
              return (
                <Card key={t.id} className="section-card p-4">
                  <div className="flex items-start justify-between gap-3">
                    <div className="min-w-0">
                      <div className="font-medium text-sm" style={{ color: "var(--yunque-text)" }}>
                        {t.display_name}
                      </div>
                      <div className="text-xs mt-1" style={{ color: "var(--yunque-text-muted)" }}>
                        {t.description}
                      </div>
                      <Chip size="sm" className="mt-2" style={{ background: "rgba(0,145,255,0.1)", color: "#0091ff" }}>
                        {t.category}
                      </Chip>
                    </div>
                    {installed ? (
                      <Chip size="sm" style={{ background: "rgba(23,201,100,0.12)", color: "#17c964" }}>
                        已添加
                      </Chip>
                    ) : (
                      <Button
                        size="sm"
                        className="btn-accent shrink-0"
                        isDisabled={packOff || busy === `install:${t.id}`}
                        onPress={() => installTemplate(t.id)}
                      >
                        <Download size={13} aria-hidden="true" /> 添加
                      </Button>
                    )}
                  </div>
                </Card>
              );
            })}
          </div>
        )}
      </div>

      {/* Alerts banner */}
      {alerts.length > 0 && (
        <Card className="section-card p-4 border-l-4" style={{ borderLeftColor: "#f31260" }} role="alert" aria-live="polite">
          <div className="flex items-center gap-2 mb-2">
            <AlertTriangle size={14} style={{ color: "#f31260" }} aria-hidden="true" />
            <span className="text-sm font-medium" style={{ color: "var(--yunque-text)" }}>
              {alerts.length} 条活跃告警
            </span>
          </div>
          <div className="space-y-1.5">
            {alerts.slice(0, 5).map((a) => {
              const sc = severityColor(a.severity);
              return (
                <div key={`${a.cogni_id}|${a.kind}`} className="flex items-start gap-3 text-sm">
                  <Chip size="sm" style={{ background: sc.bg, color: sc.fg }}>
                    {a.severity.toUpperCase()}
                  </Chip>
                  <span className="text-xs" style={{ color: "var(--yunque-text-muted)" }} translate="no">
                    {a.cogni_id}
                  </span>
                  <span style={{ color: "var(--yunque-text)" }}>{a.message}</span>
                </div>
              );
            })}
          </div>
        </Card>
      )}

      {/* Toolbar: search + import/export */}
      <div className="flex flex-wrap items-center gap-2">
        <div className="relative flex-1 min-w-[200px] max-w-md">
          <Search
            size={14}
            className="absolute left-3 top-1/2 -translate-y-1/2"
            style={{ color: "var(--yunque-text-muted)" }}
            aria-hidden="true"
          />
          <input
            type="text"
            value={filter}
            onChange={(e) => setFilter(e.target.value)}
            placeholder="搜索 Cogni…"
            aria-label="搜索 Cogni"
            spellCheck={false}
            className="w-full pl-9 pr-3 py-1.5 text-sm rounded-md"
            style={{ background: "rgba(255,255,255,0.04)", border: "1px solid rgba(255,255,255,0.08)", color: "var(--yunque-text)" }}
          />
        </div>
        <span className="text-xs" style={{ color: "var(--yunque-text-muted)" }}>
          {filteredCognis.length} / {cognis.length}
        </span>
        <div className="flex-1" />
        <Button size="sm" variant="ghost" onPress={() => cogniPack.exportBundle()} isDisabled={packOff}>
          <Download size={13} aria-hidden="true" /> 导出全部
        </Button>
        <Button size="sm" variant="ghost" onPress={() => fileInput.current?.click()} isDisabled={packOff}>
          <Upload size={13} aria-hidden="true" /> 导入
        </Button>
        <input
          ref={fileInput}
          type="file"
          accept=".json"
          hidden
          onChange={(e) => {
            const f = e.target.files?.[0];
            if (f) importBundleFile(f);
            e.target.value = "";
          }}
        />
      </div>

      {/* Assistant list */}
      {loading ? (
        <Card className="section-card p-10 text-center text-sm" style={{ color: "var(--yunque-text-muted)" }}>
          加载中…
        </Card>
      ) : filteredCognis.length === 0 ? (
        <Card className="section-card p-10 text-center text-sm" style={{ color: "var(--yunque-text-muted)" }}>
          {packOff ? "启用「Cogni 内核」能力包后即可创建 Cogni。" : "还没有 Cogni —— 在上面用一句话描述你想要的，云雀帮你造一个。"}
        </Card>
      ) : (
        <div className="grid grid-cols-1 md:grid-cols-2 gap-3">
          {filteredCognis.map((c) => {
            const hm = health[c.id];
            const hc = healthColor(hm?.status ?? "idle");
            return (
              <Card
                key={c.id}
                role="button"
                tabIndex={0}
                aria-label={`查看 Cogni ${c.display_name ?? c.id} 详情`}
                className="section-card p-4 cursor-pointer focus-visible:ring-2"
                style={{ touchAction: "manipulation" }}
                onClick={() => openDetail(c.id)}
                onKeyDown={(e) => {
                  if (e.key === "Enter" || e.key === " ") {
                    e.preventDefault();
                    openDetail(c.id);
                  }
                }}
              >
                <div className="flex items-start justify-between gap-3 mb-2">
                  <div className="flex items-start gap-3 min-w-0 flex-1">
                    <div
                      className="flex items-center justify-center rounded-xl shrink-0 font-semibold select-none"
                      style={{ width: 42, height: 42, background: avatarGradient(c.id), color: "#fff", fontSize: 17 }}
                      aria-hidden="true"
                    >
                      {avatarInitial(c.display_name ?? c.id)}
                    </div>
                    <div className="min-w-0 flex-1">
                      <div className="flex items-center gap-2 mb-1">
                        <span className="font-medium truncate" style={{ color: "var(--yunque-text)" }}>
                          {c.display_name ?? c.id}
                        </span>
                        {c.always_on && (
                          <Chip size="sm" style={{ background: "rgba(0,145,255,0.12)", color: "#0091ff" }}>
                            常驻
                          </Chip>
                        )}
                      </div>
                      {c.description && (
                        <div className="text-sm mt-1 line-clamp-2" style={{ color: "var(--yunque-text-muted)" }}>
                          {c.description}
                        </div>
                      )}
                    </div>
                  </div>
                  <div className="flex flex-col items-end gap-2" onClick={(e) => e.stopPropagation()}>
                    <Chip size="sm" style={{ background: hc.bg, color: hc.fg }}>
                      {hm ? `${hm.status} · ${hm.score}` : "未激活"}
                    </Chip>
                    <Switch isSelected={c.enabled} onChange={(v) => toggle(c.id, v)} size="sm" aria-label={`启用 ${c.id}`}>
                      <Switch.Control>
                        <Switch.Thumb />
                      </Switch.Control>
                    </Switch>
                  </div>
                </div>

                {c.load_error && (
                  <div className="mt-2 text-xs p-2 rounded" style={{ background: "rgba(243,18,96,0.1)", color: "#f31260" }}>
                    {formatErrorMessage(c.load_error, "加载失败")}
                  </div>
                )}

                <div className="mt-2 flex items-center justify-end gap-1" onClick={(e) => e.stopPropagation()}>
                  <Button size="sm" variant="ghost" onPress={() => setShareID(c.id)} aria-label={`分享 ${c.id}`}>
                    <Share2 size={12} aria-hidden="true" />
                  </Button>
                  <Button
                    size="sm"
                    variant="ghost"
                    isIconOnly
                    isDisabled={busy === `remove:${c.id}`}
                    onPress={() => remove(c.id)}
                    aria-label={`删除 ${c.id}`}
                  >
                    <Trash2 size={12} style={{ color: "#f31260" }} aria-hidden="true" />
                  </Button>
                </div>
              </Card>
            );
          })}
        </div>
      )}

      {/* Developer diagnostics — folded so the page leads with assistants */}
      <div className="mt-2">
        <button
          type="button"
          onClick={() => setAdvancedOpen((v) => !v)}
          className="flex items-center gap-1.5 text-xs"
          style={{ color: "var(--yunque-text-muted)" }}
        >
          <ChevronDown
            size={13}
            aria-hidden="true"
            style={{ transform: advancedOpen ? "none" : "rotate(-90deg)", transition: "transform .15s" }}
          />
          开发者诊断（Cogni 运行态）
        </button>
        {advancedOpen && runtimePackState && (
          <Card
            className="section-card p-4 border-l-4 mt-2"
            style={{ borderLeftColor: runtimePackState.runtime_loop_running ? "#17c964" : "#ffaa00" }}
          >
            <div className="flex flex-wrap items-center gap-2">
              <ShieldCheck size={14} style={{ color: "var(--yunque-text-muted)" }} aria-hidden="true" />
              <span className="text-sm font-medium" style={{ color: "var(--yunque-text)" }}>
                运行态 Gate
              </span>
              <Chip size="sm" translate="no">{runtimePackState.pack_status || "unknown"}</Chip>
              <Chip
                size="sm"
                style={{
                  background: runtimePackState.runtime_loop_running ? "rgba(23,201,100,0.12)" : "rgba(255,170,0,0.12)",
                  color: runtimePackState.runtime_loop_running ? "#17c964" : "#ffaa00",
                }}
              >
                loop {runtimePackState.runtime_loop_running ? "running" : "stopped"}
              </Chip>
            </div>
            <div className="flex flex-wrap gap-x-4 gap-y-1 text-[11px] mt-2" style={{ color: "var(--yunque-text-muted)" }}>
              <span translate="no">active_bus_cognis {runtimePackState.active_bus_cognis}</span>
              <span translate="no">experience_store_count {runtimePackState.experience_store_count}</span>
            </div>
            <div className="mt-3">
              <Button size="sm" variant="ghost" onPress={load}>
                <RefreshCw size={11} aria-hidden="true" /> 刷新
              </Button>
            </div>
          </Card>
        )}
      </div>

      {/* Share modal */}
      <CherryModal
        open={!!shareID}
        onClose={() => setShareID(null)}
        size="md"
        ariaLabel="分享 Cogni"
        header={
          <div className="flex items-center gap-2">
            <Share2 size={18} style={{ color: "var(--yunque-accent)" }} aria-hidden="true" />
            <span>分享 Cogni</span>
          </div>
        }
      >
        <div className="space-y-3">
          <div className="text-sm" style={{ color: "var(--yunque-text-secondary)" }}>
            选择分享方式：
          </div>
          <Button className="w-full justify-start" variant="outline" onPress={copyShareLink}>
            <LinkIcon size={16} aria-hidden="true" />
            <span className="flex-1 text-left">复制分享链接</span>
          </Button>
          <Button className="w-full justify-start" variant="outline" onPress={downloadShare}>
            <Download size={16} aria-hidden="true" />
            <span className="flex-1 text-left">下载为文件</span>
          </Button>
        </div>
      </CherryModal>

      {/* Import preview modal */}
      <CherryModal
        open={!!importPreview}
        onClose={() => setImportPreview(null)}
        size="lg"
        ariaLabel="导入预览"
        header={<span>导入预览</span>}
      >
        {importPreview && (
          <div className="space-y-3">
            <div className="text-sm" style={{ color: "var(--yunque-text-secondary)" }}>
              将要导入以下 Cogni：
            </div>
            <div className="space-y-2 max-h-96 overflow-y-auto" style={{ overscrollBehavior: "contain" }}>
              {((importPreview.cognis as Array<Record<string, unknown>>) ?? []).map((cogni, index) => (
                <Card key={index} className="section-card p-3">
                  <div className="font-semibold text-sm" style={{ color: "var(--yunque-text)" }}>
                    {String(cogni.display_name ?? cogni.id ?? "未命名")}
                  </div>
                  {cogni.description ? (
                    <div className="text-xs mt-1" style={{ color: "var(--yunque-text-muted)" }}>
                      {String(cogni.description)}
                    </div>
                  ) : null}
                </Card>
              ))}
            </div>
            <div className="flex justify-end gap-2 pt-3 border-t" style={{ borderColor: "var(--yunque-border)" }}>
              <Button size="sm" variant="ghost" onPress={() => setImportPreview(null)}>
                取消
              </Button>
              <Button size="sm" className="btn-accent" onPress={importFromPreview}>
                <CheckCircle2 size={14} aria-hidden="true" /> 确认导入
              </Button>
            </div>
          </div>
        )}
      </CherryModal>

      {/* Detail drawer */}
      {detailID && (
        <div
          className="fixed inset-0 z-50 flex justify-end"
          style={{ background: "rgba(0,0,0,0.5)" }}
          role="dialog"
          aria-modal="true"
          aria-label={`Cogni ${detailID} 详情`}
          onClick={() => setDetailID(null)}
          onKeyDown={(e) => {
            if (e.key === "Escape") setDetailID(null);
          }}
        >
          <Card
            className="h-full overflow-y-auto p-5 section-card"
            style={{ width: "min(560px, 100%)", overscrollBehavior: "contain" }}
            onClick={(e) => e.stopPropagation()}
          >
            <div className="flex items-center justify-between mb-4">
              <div className="font-medium text-sm" style={{ color: "var(--yunque-text)" }} translate="no">
                {detailID}
              </div>
              <Button size="sm" variant="ghost" onPress={() => setDetailID(null)}>
                关闭
              </Button>
            </div>

            <div className="flex gap-1 mb-4 border-b" style={{ borderColor: "rgba(255,255,255,0.08)" }} role="tablist">
              {(["traces", "workflows", "experience", "evolution"] as const).map((tab) => (
                <button
                  key={tab}
                  type="button"
                  role="tab"
                  aria-selected={detailTab === tab}
                  onClick={() => setDetailTab(tab)}
                  className="px-3 py-1.5 text-xs font-medium rounded-t-md transition-colors"
                  style={{
                    color: detailTab === tab ? "var(--yunque-text)" : "var(--yunque-text-muted)",
                    background: detailTab === tab ? "rgba(255,255,255,0.06)" : "transparent",
                    borderBottom: detailTab === tab ? "2px solid var(--yunque-accent)" : "2px solid transparent",
                  }}
                >
                  {tab === "traces" && "活动记录"}
                  {tab === "workflows" && "工作流"}
                  {tab === "experience" && "经验"}
                  {tab === "evolution" && "进化"}
                </button>
              ))}
            </div>

            {detailTab === "traces" && (
              <>
                <div className="text-xs mb-2" style={{ color: "var(--yunque-text-muted)" }}>
                  最近活动 ({detailTraces.length})
                </div>
                {detailTraces.length === 0 ? (
                  <div className="text-sm py-6 text-center" style={{ color: "var(--yunque-text-muted)" }}>
                    暂无记录 —— 该 Cogni 还没参与过对话
                  </div>
                ) : (
                  <div className="space-y-2">
                    {detailTraces.map((t, i) => {
                      const own = t.activations?.find((a) => a.id === detailID);
                      const activated = !!own?.activated;
                      return (
                        <div key={i} className="p-3 rounded-lg text-xs" style={{ background: "rgba(255,255,255,0.03)" }}>
                          <div className="flex items-center gap-2 mb-1">
                            <Chip
                              size="sm"
                              style={{
                                background: activated ? "rgba(23,201,100,0.12)" : "rgba(255,255,255,0.04)",
                                color: activated ? "#17c964" : "var(--yunque-text-muted)",
                              }}
                            >
                              {activated ? "已激活" : "未激活"}
                            </Chip>
                            <span style={{ color: "var(--yunque-text-muted)" }}>
                              {new Date(t.timestamp).toLocaleTimeString()}
                            </span>
                          </div>
                          {own?.reasons && own.reasons.length > 0 && (
                            <div style={{ color: "var(--yunque-text-muted)" }}>{own.reasons.join("; ")}</div>
                          )}
                        </div>
                      );
                    })}
                  </div>
                )}
              </>
            )}

            {detailTab === "workflows" && (
              <>
                <div className="text-xs mb-2" style={{ color: "var(--yunque-text-muted)" }}>
                  <Workflow size={12} className="inline mr-1" aria-hidden="true" />
                  工作流 ({detailWorkflows.length})
                </div>
                {detailWorkflows.length === 0 ? (
                  <div className="text-sm py-6 text-center" style={{ color: "var(--yunque-text-muted)" }}>
                    该 Cogni 未定义工作流
                  </div>
                ) : (
                  <div className="space-y-3">
                    {detailWorkflows.map((wf) => (
                      <div key={wf.name} className="p-3 rounded-lg" style={{ background: "rgba(255,255,255,0.03)" }}>
                        <div className="flex items-center justify-between mb-2">
                          <span className="text-sm font-medium" style={{ color: "var(--yunque-text)" }}>
                            {wf.name}
                          </span>
                          <Button
                            size="sm"
                            variant="ghost"
                            onPress={async () => {
                              try {
                                const r = await cogniPack.runWorkflow(detailID!, wf.name);
                                showToast(r.success ? `工作流完成：${r.workflow_name}` : `工作流失败：${r.error}`, r.success ? "success" : "error");
                              } catch (e) {
                                showToast(formatErrorMessage(e, "执行失败"), "error");
                              }
                            }}
                          >
                            <Play size={10} aria-hidden="true" /> 执行
                          </Button>
                        </div>
                        {wf.description && (
                          <div className="text-xs mb-2" style={{ color: "var(--yunque-text-muted)" }}>
                            {wf.description}
                          </div>
                        )}
                        <div className="text-xs" style={{ color: "var(--yunque-text-muted)" }}>
                          {wf.steps?.length || 0} 个步骤
                          {wf.steps?.map((s: CogniWorkflowStep, i: number) => (
                            <span key={i}> · {s.skill}</span>
                          ))}
                        </div>
                      </div>
                    ))}
                  </div>
                )}
              </>
            )}

            {detailTab === "experience" && (
              <>
                <div className="text-xs mb-2" style={{ color: "var(--yunque-text-muted)" }}>
                  经验累积
                </div>
                {!detailExperience?.enabled ? (
                  <div className="text-sm py-6 text-center" style={{ color: "var(--yunque-text-muted)" }}>
                    该 Cogni 未启用经验引擎
                  </div>
                ) : (
                  <div className="space-y-3">
                    <div className="grid grid-cols-2 gap-2">
                      {Object.entries(experienceStats).map(([k, v]) => (
                        <div key={k} className="p-2 rounded-lg text-center" style={{ background: "rgba(255,255,255,0.03)" }}>
                          <div className="text-lg font-medium" style={{ color: "var(--yunque-text)" }}>
                            {String(v)}
                          </div>
                          <div className="text-[10px]" style={{ color: "var(--yunque-text-muted)" }}>
                            {k.replace(/_/g, " ")}
                          </div>
                        </div>
                      ))}
                    </div>
                    {topTools.length > 0 && (
                      <div>
                        <div className="text-xs mb-1 font-medium" style={{ color: "var(--yunque-text)" }}>
                          高频工具经验
                        </div>
                        {topTools.map((tool, i) => (
                          <div key={`${tool.tool}-${i}`} className="text-xs p-2 rounded mb-1" style={{ background: "rgba(34,211,238,0.08)", color: "var(--yunque-text-muted)" }}>
                            <div className="flex items-center justify-between gap-2">
                              <span className="font-medium" style={{ color: "var(--yunque-text)" }}>
                                {tool.tool || "unknown tool"}
                              </span>
                              <span>复用 {tool.used_count ?? 0}</span>
                            </div>
                            {tool.learned && <div>{tool.learned}</div>}
                          </div>
                        ))}
                      </div>
                    )}
                    {topFacts.length > 0 && (
                      <div>
                        <div className="text-xs mb-1 font-medium" style={{ color: "var(--yunque-text)" }}>
                          高频领域事实
                        </div>
                        {topFacts.map((fact, i) => (
                          <div key={`${fact.fact}-${i}`} className="text-xs p-2 rounded mb-1" style={{ background: "rgba(167,139,250,0.08)", color: "var(--yunque-text-muted)" }}>
                            <div style={{ color: "var(--yunque-text)" }}>{fact.fact}</div>
                          </div>
                        ))}
                      </div>
                    )}
                    {pendingPatterns.length > 0 && (
                      <div>
                        <div className="text-xs mb-1 font-medium" style={{ color: "var(--yunque-text)" }}>
                          待确认模式
                        </div>
                        {pendingPatterns.map((p: CogniExperiencePattern, i: number) => (
                          <div key={p.id || i} className="text-xs p-2 rounded mb-1" style={{ background: "rgba(255,170,0,0.08)", color: "var(--yunque-text-muted)" }}>
                            <div className="flex items-start justify-between gap-2">
                              <span>
                                {p.trigger} → {p.response}
                              </span>
                              {p.id && (
                                <Button
                                  size="sm"
                                  variant="ghost"
                                  isPending={confirmingPatternID === p.id}
                                  isDisabled={!!confirmingPatternID && confirmingPatternID !== p.id}
                                  onPress={() => p.id && confirmExperiencePattern(p.id)}
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
              </>
            )}

            {detailTab === "evolution" && (
              <>
                <div className="flex items-center justify-between mb-2">
                  <div className="text-xs" style={{ color: "var(--yunque-text-muted)" }}>
                    <FlaskConical size={12} className="inline mr-1" aria-hidden="true" />
                    技能进化
                  </div>
                  <Button
                    size="sm"
                    variant="ghost"
                    onPress={async () => {
                      try {
                        await cogniPack.triggerEvolution(detailID!);
                        showToast("进化已启动", "success");
                      } catch (e) {
                        showToast(formatErrorMessage(e, "启动失败"), "error");
                      }
                    }}
                  >
                    <FlaskConical size={10} aria-hidden="true" /> 触发进化
                  </Button>
                </div>
                {!detailEvolution?.experiments || detailEvolution.experiments.length === 0 ? (
                  <div className="text-sm py-6 text-center" style={{ color: "var(--yunque-text-muted)" }}>
                    尚无进化实验记录
                  </div>
                ) : (
                  <div className="space-y-2 mt-2">
                    {detailEvolution.experiments.map((exp: CogniExperiment) => (
                      <div key={exp.id} className="p-3 rounded-lg text-xs" style={{ background: "rgba(255,255,255,0.03)" }}>
                        <div className="flex items-center gap-2 mb-1">
                          <Chip
                            size="sm"
                            style={{
                              background: exp.status === "kept" ? "rgba(23,201,100,0.12)" : "rgba(243,18,96,0.12)",
                              color: exp.status === "kept" ? "#17c964" : "#f31260",
                            }}
                          >
                            {exp.status === "kept" ? "保留" : "回滚"}
                          </Chip>
                          <span style={{ color: exp.delta >= 0 ? "#17c964" : "#f31260" }}>
                            {exp.delta >= 0 ? "+" : ""}
                            {exp.delta.toFixed(1)}%
                          </span>
                        </div>
                        <div style={{ color: "var(--yunque-text-muted)" }}>{exp.change}</div>
                      </div>
                    ))}
                  </div>
                )}
              </>
            )}
          </Card>
        </div>
      )}
    </div>
  );
}
