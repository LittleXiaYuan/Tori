"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import Link from "next/link";
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
import { Button, Chip, Disclosure, DisclosureGroup, SearchField, Switch, Tooltip } from "@heroui/react";
import { KPI, KPIGroup, Segment, ItemCard, ItemCardGroup, PromptInput } from "@heroui-pro/react";
import {
  AlertTriangle,
  CheckCircle2,
  Download,
  FlaskConical,
  Link as LinkIcon,
  Play,
  RefreshCw,
  Share2,
  Sparkles,
  Trash2,
  Upload,
  Wand2,
  Workflow,
} from "lucide-react";
import PageHeader from "@/components/page-header";
import EmptyState from "@/components/empty-state";
import { confirmAction } from "@/components/confirm-dialog";
import { showToast } from "@/components/toast-provider";
import { formatErrorMessage } from "@/lib/error-utils";
import { fetcher } from "@/lib/api-core";
import { createCognisClient } from "yunque-client/cognis";
import { createSkillsClient, type SkillInfo, type SkillCategory } from "yunque-client/skills";
import { createYunqueSDKClientOptions } from "@/lib/sdk-client";
import { CherryModal } from "@/components/cherry/overlay";
import { CapabilityDetailModal } from "@/components/capability-detail-modal";
import { useRouter } from "next/navigation";

type HealthMap = Record<string, CogniHealthMetrics>;
type ConnectorHint = { id: string; name: string; status: string };
type DetailTab = "overview" | "config" | "logs";
type ChipColor = "success" | "warning" | "danger" | "default";

// Intent keyword → suggested skill labels + connector IDs to surface.
const INTENT_HINTS: Array<{ re: RegExp; skills: string[]; connectors: string[] }> = [
  { re: /代码|编程|开发|审查|git|github/i, skills: ["代码分析"], connectors: ["github"] },
  { re: /数据|分析|报表|图表|excel|表格/i, skills: ["数据处理", "图表生成"], connectors: [] },
  { re: /文档|ppt|演示|word|markdown|周报/i, skills: ["文档生成"], connectors: ["google-drive", "notion"] },
  { re: /邮件|email|mail/i, skills: [], connectors: ["gmail"] },
  { re: /日历|会议|日程|提醒/i, skills: [], connectors: ["google-calendar"] },
  { re: /搜索|查资料|网络|爬虫/i, skills: ["网络搜索"], connectors: [] },
  { re: /客服|回复|聊天/i, skills: ["对话管理"], connectors: [] },
];

const cogniPack = createCogniKernelPackClient();
// Only used for id-filtered export (per-assistant share); hits the same /v1/cognis API.
const cognisClient = createCognisClient(createYunqueSDKClientOptions());
// Real installed capabilities (skills include MCP tools + plugin skills, registered at boot).
const skillsClient = createSkillsClient(createYunqueSDKClientOptions());

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

const COGNI_DELIVERY_SECTIONS: Array<{ title: string; tone: ChipColor; items: string[] }> = [
  {
    title: "现在可稳定交付",
    tone: "success",
    items: ["创建、导入、导出、启用和停用 Cogni 声明。", "查看健康、运行轨迹、工作流、经验与演化建议。", "通过 Cogni 内核感知能力包启停状态和运行门禁。"],
  },
  {
    title: "Beta 继续观察",
    tone: "warning",
    items: ["Planner 自动选择 Cogni 的命中质量。", "经验确认和演化建议是否真的减少重复说明。", "Cogni 与记忆、任务、知识、Skill/MCP 的组合效果。"],
  },
  {
    title: "暂不作为稳定承诺",
    tone: "danger",
    items: ["自主执行高风险动作或本机电脑控制。", "完全替代 Skill 与 MCP 生态。", "不经能力包权限和门禁直接调用底层能力。"],
  },
];

const COGNI_USAGE_NOTES = [
  "能力包扩展云雀底座：提供路由、界面、权限、WASM/DLC 或后端能力。",
  "Cogni 增设模型侧能力声明：告诉模型何时选择 Skill、MCP、能力包、记忆和工作流。",
  "如果某个能力包被禁用，Cogni 只能看到受限状态，不能绕过 Pack Runtime。",
];

// Map runtime health status → semantic Chip color.
function healthChipColor(status: string): ChipColor {
  switch (status) {
    case "healthy":
      return "success";
    case "warn":
      return "warning";
    case "unhealthy":
      return "danger";
    default:
      return "default";
  }
}

function severityChipColor(sev: string): ChipColor {
  switch (sev) {
    case "critical":
      return "danger";
    case "warn":
      return "warning";
    default:
      return "default";
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
  const router = useRouter();
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

  // Bottom accordion sections (collapsed by default — page leads with assistants).
  const [showTemplates, setShowTemplates] = useState(false);
  const [showDeliveryNotes, setShowDeliveryNotes] = useState(false);
  const [showUsageNotes, setShowUsageNotes] = useState(false);
  const [advancedOpen, setAdvancedOpen] = useState(false);
  const [runtimePackState, setRuntimePackState] = useState<CogniRuntimePackStateReport | null>(null);
  const fileInput = useRef<HTMLInputElement>(null);
  const [connectors, setConnectors] = useState<ConnectorHint[]>([]);
  const connectorsFetched = useRef(false);

  // Real installed capabilities (skills = built-in + plugin + MCP tools).
  const [skills, setSkills] = useState<SkillInfo[]>([]);
  const [skillCategories, setSkillCategories] = useState<SkillCategory[]>([]);

  // Share + import-preview modals (ported from the old market page).
  const [shareID, setShareID] = useState<string | null>(null);
  const [importPreview, setImportPreview] = useState<Record<string, unknown> | null>(null);

  // Detail drawer.
  const [detailID, setDetailID] = useState<string | null>(null);
  const [detailTab, setDetailTab] = useState<DetailTab>("overview");
  const [detailTraces, setDetailTraces] = useState<CogniTrace[]>([]);
  const [detailWorkflows, setDetailWorkflows] = useState<CogniWorkflowDef[]>([]);
  const [detailExperience, setDetailExperience] = useState<CogniExperienceResponse | null>(null);
  const [detailEvolution, setDetailEvolution] = useState<CogniEvolutionResponse | null>(null);
  const [detailDeclaration, setDetailDeclaration] = useState<CogniDeclaration | null>(null);
  const [confirmingPatternID, setConfirmingPatternID] = useState<string | null>(null);

  const focusHeroCreate = useCallback(() => {
    const el = document.getElementById("cogni-hero");
    el?.scrollIntoView({ behavior: "smooth", block: "center" });
    el?.querySelector("textarea")?.focus();
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
      // Real installed capabilities — non-critical, never blocks the page.
      skillsClient
        .list()
        .then((res) => {
          setSkills(res.skills || []);
          setSkillCategories(res.categories || []);
        })
        .catch(() => { /* skills are a hint surface; ignore failures */ });
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

  // Lazy-fetch connectors once on first description keystroke.
  const loadConnectors = useCallback(async () => {
    if (connectorsFetched.current) return;
    connectorsFetched.current = true;
    try {
      const res = await fetcher<{ connectors: ConnectorHint[] }>("/api/connectors");
      setConnectors(res.connectors || []);
    } catch { /* non-critical */ }
  }, []);

  // Recommend REAL installed skills that relate to the description, plus
  // connector hints. Skill matching is bidirectional substring (Chinese has no
  // word boundaries): a skill is suggested when its name/category overlaps the
  // description, or an INTENT_HINTS keyword for that intent fires.
  const recommendations = useMemo(() => {
    const desc = generateDesc.trim();
    if (!desc) return null;
    const lower = desc.toLowerCase();

    // Connector hints still come from INTENT_HINTS (connectors aren't skills).
    const connectorIds = new Set<string>();
    const hintCategories = new Set<string>();
    for (const hint of INTENT_HINTS) {
      if (hint.re.test(desc)) {
        hint.connectors.forEach((c) => connectorIds.add(c));
        hint.skills.forEach((s) => hintCategories.add(s.toLowerCase()));
      }
    }

    // Match real skills: name/category overlaps the description (either way),
    // or the skill's category matches an intent keyword we detected.
    const matchedSkills = skills
      .filter((sk) => {
        const name = (sk.name || "").toLowerCase();
        const cat = (sk.category || "").toLowerCase();
        if (!name && !cat) return false;
        const nameHit = name.length > 1 && (lower.includes(name) || name.includes(lower));
        const catHit = cat.length > 1 && (lower.includes(cat) || hintCategories.has(cat));
        return nameHit || catHit;
      })
      .slice(0, 6);

    const matchedConnectors = [...connectorIds]
      .map((cid) => connectors.find((c) => c.id === cid || c.name.toLowerCase().includes(cid)))
      .filter((c): c is ConnectorHint => !!c);

    if (matchedSkills.length === 0 && matchedConnectors.length === 0) return null;
    return { skills: matchedSkills, connectors: matchedConnectors };
  }, [generateDesc, skills, connectors]);

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
    const confirmed = await confirmAction({
      title: "删除 Cogni",
      body: `确定删除 Cogni「${id}」？此操作不可撤销。`,
      confirmLabel: "删除",
      tone: "danger",
    });
    if (!confirmed) return;
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

  const downloadAllCognis = async () => {
    try {
      await cogniPack.exportBundle();
      showToast("已导出全部 Cogni", "success");
    } catch (e) {
      showToast(formatErrorMessage(e, "导出 Cogni 失败"), "error");
    }
  };

  const openDetail = async (id: string) => {
    setDetailID(id);
    setDetailTab("overview");
    setDetailDeclaration(null);
    const [traces, workflows, experience, evolution, decl] = await Promise.all([
      cogniPack.tracesByID(id, 20).catch(() => ({ traces: [] as CogniTrace[] })),
      cogniPack.workflows(id).catch(() => ({ workflows: [] as CogniWorkflowDef[] })),
      cogniPack.experience(id).catch(() => null),
      cogniPack.evolution(id).catch(() => null),
      cogniPack.get(id).catch(() => null),
    ]);
    setDetailTraces(traces.traces || []);
    setDetailWorkflows(workflows.workflows || []);
    setDetailExperience(experience);
    setDetailEvolution(evolution);
    setDetailDeclaration(decl?.declaration ?? null);
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

  const experienceStats = detailExperience?.summary?.stats ?? detailExperience?.stats ?? {};
  const pendingPatterns =
    detailExperience?.summary?.pending_patterns ??
    detailExperience?.patterns?.filter((p) => !p.confirmed).slice(0, 5) ??
    [];

  const enabledCount = useMemo(() => cognis.filter((c) => c.enabled).length, [cognis]);
  const detailEntry = detailID ? cognis.find((c) => c.id === detailID) : undefined;

  return (
    <div className="page-root mx-auto max-w-6xl space-y-6 animate-fade-in-up">
      <PageHeader
        icon={<Sparkles size={20} aria-hidden="true" />}
        title="Cogni"
        description="用大白话描述你想要的 Cogni，云雀自动配好技能、触发方式和人设。"
        onRefresh={load}
        actions={
          <Button size="sm" className="btn-accent" onPress={focusHeroCreate} isDisabled={packOff}>
            <Wand2 size={12} aria-hidden="true" /> 新建 Cogni
          </Button>
        }
      />

      {/* Stats row */}
      <KPIGroup className="grid grid-cols-2 gap-3 sm:grid-cols-4">
        <KPI>
          <KPI.Header>
            <KPI.Icon><Sparkles size={16} /></KPI.Icon>
            <KPI.Title>Cogni 总数</KPI.Title>
          </KPI.Header>
          <KPI.Content>
            <KPI.Value value={cognis.length} maximumFractionDigits={0} />
          </KPI.Content>
        </KPI>
        <KPI>
          <KPI.Header>
            <KPI.Icon status="success"><CheckCircle2 size={16} /></KPI.Icon>
            <KPI.Title>已启用</KPI.Title>
          </KPI.Header>
          <KPI.Content>
            <KPI.Value value={enabledCount} maximumFractionDigits={0} />
          </KPI.Content>
        </KPI>
        <KPI>
          <KPI.Header>
            <KPI.Icon><Wand2 size={16} /></KPI.Icon>
            <KPI.Title>可用能力</KPI.Title>
          </KPI.Header>
          <KPI.Content>
            <KPI.Value value={skills.length} maximumFractionDigits={0} />
          </KPI.Content>
        </KPI>
        <KPI>
          <KPI.Header>
            <KPI.Icon status={alerts.length > 0 ? "danger" : undefined}><AlertTriangle size={16} /></KPI.Icon>
            <KPI.Title>活跃告警</KPI.Title>
          </KPI.Header>
          <KPI.Content>
            <KPI.Value value={alerts.length} maximumFractionDigits={0} />
          </KPI.Content>
        </KPI>
      </KPIGroup>

      {/* Pack-disabled banner */}
      {packOff && (
        <div className="section-card rounded-xl p-4">
          <div className="flex items-center gap-2 text-sm" style={{ color: "var(--yunque-text)" }}>
            <AlertTriangle size={15} style={{ color: "var(--yunque-warning)" }} aria-hidden="true" />
            Cogni 功能由「Cogni 内核」能力包提供，当前未启用。
          </div>
          <p className="text-xs mt-1" style={{ color: "var(--yunque-text-muted)" }}>
            前往「能力包」启用 <span translate="no">yunque.pack.cogni-kernel</span> 后即可创建和管理 Cogni。
          </p>
          <Link href="/packs" className="mt-3 inline-block">
            <Button size="sm" variant="outline">去启用</Button>
          </Link>
        </div>
      )}

      {/* Hero — natural-language Cogni creation */}
      <div id="cogni-hero" className="section-card rounded-xl p-5">
        <div className="flex items-center gap-2 mb-1" style={{ color: "var(--yunque-text)" }}>
          <Wand2 size={16} style={{ color: "var(--yunque-accent)" }} aria-hidden="true" />
          <span className="text-base font-medium">描述你想要的 Cogni，云雀帮你造一个</span>
        </div>
        <p className="text-xs mb-4" style={{ color: "var(--yunque-text-muted)" }}>
          一句话说清它要做什么 —— 云雀会自动配好该用的技能、激活关键词和说话风格，创建后立即可用。
        </p>
        <PromptInput
          value={generateDesc}
          onValueChange={(v) => { setGenerateDesc(v); loadConnectors(); }}
          onSubmit={generateCogni}
          status={generating ? "submitted" : "ready"}
          isDisabled={packOff}
        >
          <PromptInput.Shell>
            <PromptInput.Content>
              <PromptInput.TextArea
                placeholder="例如：一个帮我整理周报、能查资料还能做成 PPT 的助手…"
                aria-label="描述你想要的 Cogni"
              />
            </PromptInput.Content>
            <PromptInput.Toolbar>
              <PromptInput.ToolbarStart>
                <span className="text-[11px]" style={{ color: "var(--yunque-text-muted)" }}>
                  {generating ? "云雀正在造 Cogni…" : "回车创建"}
                </span>
              </PromptInput.ToolbarStart>
              <PromptInput.ToolbarEnd>
                <PromptInput.Send aria-label="创建 Cogni">
                  <Sparkles className="size-4" aria-hidden="true" />
                </PromptInput.Send>
              </PromptInput.ToolbarEnd>
            </PromptInput.Toolbar>
          </PromptInput.Shell>
        </PromptInput>

        {/* Example starters + live recommendations */}
        <div className="mt-3 flex flex-wrap items-center gap-1.5">
          {ASSISTANT_EXAMPLES.map((ex) => (
            <Chip
              key={ex}
              size="sm"
              color="default"
              className="cursor-pointer"
              onClick={() => { setGenerateDesc(ex); loadConnectors(); }}
            >
              {ex.length > 18 ? ex.slice(0, 18) + "…" : ex}
            </Chip>
          ))}
        </div>

        {/* Real capability hint — the skills/MCP tools this agent actually has */}
        {skills.length > 0 && (
          <div className="mt-3 rounded-lg p-3" style={{ background: "var(--yunque-bg-muted)" }}>
            <div className="flex items-center justify-between gap-2 mb-2">
              <span className="text-[11px]" style={{ color: "var(--yunque-text-muted)" }}>
                你现在有 <span style={{ color: "var(--yunque-text)" }}>{skills.length}</span> 项能力（含技能、插件与已连接的 MCP 工具），云雀会按需挑选：
              </span>
              <Link href="/skills" className="text-[11px] underline shrink-0" style={{ color: "var(--yunque-text-muted)" }}>
                管理能力
              </Link>
            </div>
            <div className="flex flex-wrap gap-1.5">
              {(skillCategories.length > 0
                ? skillCategories.map((c) => c.name)
                : Array.from(new Set(skills.map((s) => s.category).filter(Boolean) as string[]))
              ).slice(0, 10).map((cat) => (
                <Chip key={cat} size="sm" color="default">{cat}</Chip>
              ))}
            </div>
          </div>
        )}
        {recommendations && (
          <div className="mt-3 flex flex-wrap items-center gap-1.5">
            <span className="text-[11px]" style={{ color: "var(--yunque-text-muted)" }}>
              将用到你的能力：
            </span>
            {recommendations.skills.map((s) => (
              <Chip key={s.name} size="sm" color="success" title={s.description}>
                {s.name}
              </Chip>
            ))}
            {recommendations.connectors.map((c) => (
              <Chip key={c.id} size="sm" color={c.status === "connected" ? "success" : "warning"}>
                {c.name}
                {c.status !== "connected" && (
                  <Link href="/knowledge" className="ml-1 underline text-[10px]">配置</Link>
                )}
              </Chip>
            ))}
          </div>
        )}
        {generatePreview && (
          <div className="mt-4 p-3 rounded-lg" style={{ background: "var(--yunque-bg-muted)" }}>
            <div className="flex items-center gap-2 mb-1">
              <CheckCircle2 size={14} style={{ color: "var(--yunque-success)" }} aria-hidden="true" />
              <span className="text-sm font-medium" style={{ color: "var(--yunque-text)" }}>
                已为你创建：{generatePreview.display_name ?? generatePreview.id}
              </span>
            </div>
            {generatePreview.description && (
              <p className="text-xs" style={{ color: "var(--yunque-text-muted)" }}>{generatePreview.description}</p>
            )}
          </div>
        )}
      </div>

      {/* Alerts banner */}
      {alerts.length > 0 && (
        <div className="section-card rounded-xl p-4" role="alert" aria-live="polite">
          <div className="flex items-center gap-2 mb-2">
            <AlertTriangle size={14} style={{ color: "var(--yunque-danger)" }} aria-hidden="true" />
            <span className="text-sm font-medium" style={{ color: "var(--yunque-text)" }}>
              {alerts.length} 条活跃告警
            </span>
          </div>
          <div className="space-y-1.5">
            {alerts.slice(0, 5).map((a) => (
              <div key={`${a.cogni_id}|${a.kind}`} className="flex items-start gap-3 text-sm">
                <Chip size="sm" color={severityChipColor(a.severity)}>{a.severity.toUpperCase()}</Chip>
                <span className="text-xs" style={{ color: "var(--yunque-text-muted)" }} translate="no">{a.cogni_id}</span>
                <span style={{ color: "var(--yunque-text)" }}>{a.message}</span>
              </div>
            ))}
          </div>
        </div>
      )}

      {/* Toolbar: search + import/export */}
      <div className="flex flex-wrap items-center gap-2">
        <SearchField className="flex-1 max-w-sm" value={filter} onChange={setFilter} aria-label="搜索 Cogni">
          <SearchField.Group>
            <SearchField.SearchIcon />
            <SearchField.Input placeholder="搜索 Cogni…" />
          </SearchField.Group>
        </SearchField>
        {filter && (
          <span className="text-xs" style={{ color: "var(--yunque-text-muted)" }}>
            {filteredCognis.length} / {cognis.length}
          </span>
        )}
        <div className="flex-1" />
        <Tooltip delay={0}>
          <Button size="sm" variant="ghost" isIconOnly onPress={downloadAllCognis} isDisabled={packOff} aria-label="导出全部">
            <Download size={13} aria-hidden="true" />
          </Button>
          <Tooltip.Content>导出全部</Tooltip.Content>
        </Tooltip>
        <Tooltip delay={0}>
          <Button size="sm" variant="ghost" isIconOnly onPress={() => fileInput.current?.click()} isDisabled={packOff} aria-label="导入">
            <Upload size={13} aria-hidden="true" />
          </Button>
          <Tooltip.Content>导入</Tooltip.Content>
        </Tooltip>
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

      {/* Cogni list */}
      {loading ? (
        <div className="section-card rounded-xl p-10 text-center text-sm" style={{ color: "var(--yunque-text-muted)" }}>
          加载中…
        </div>
      ) : filteredCognis.length === 0 ? (
        <EmptyState
          icon={<Sparkles size={28} aria-hidden="true" />}
          title={packOff ? "Cogni 内核未启用" : "还没有 Cogni"}
          description={
            packOff
              ? "启用「Cogni 内核」能力包后即可创建 Cogni。"
              : "在上面用一句话描述你想要的，云雀帮你造一个。"
          }
          actionLabel={packOff ? undefined : "新建 Cogni"}
          onAction={packOff ? undefined : focusHeroCreate}
        />
      ) : (
        <ItemCardGroup layout="grid" columns={2}>
          {filteredCognis.map((c) => {
            const hm = health[c.id];
            return (
              <ItemCard
                key={c.id}
                role="button"
                tabIndex={0}
                aria-label={`查看 Cogni ${c.display_name ?? c.id} 详情`}
                className="cursor-pointer"
                onClick={() => openDetail(c.id)}
                onKeyDown={(e: React.KeyboardEvent) => {
                  if (e.key === "Enter" || e.key === " ") {
                    e.preventDefault();
                    openDetail(c.id);
                  }
                }}
              >
                <ItemCard.Icon>
                  <div
                    className="flex items-center justify-center rounded-xl font-semibold select-none"
                    style={{ width: 40, height: 40, background: avatarGradient(c.id), color: "#fff", fontSize: 16 }}
                    aria-hidden="true"
                  >
                    {avatarInitial(c.display_name ?? c.id)}
                  </div>
                </ItemCard.Icon>
                <ItemCard.Content>
                  <ItemCard.Title>
                    <span className="inline-flex items-center gap-2">
                      {c.display_name ?? c.id}
                      {c.always_on && <Chip size="sm" color="default">常驻</Chip>}
                    </span>
                  </ItemCard.Title>
                  {c.description && (
                    <ItemCard.Description className="line-clamp-2">{c.description}</ItemCard.Description>
                  )}
                  {c.load_error && (
                    <span className="text-xs" style={{ color: "var(--yunque-danger)" }}>
                      {formatErrorMessage(c.load_error, "加载失败")}
                    </span>
                  )}
                </ItemCard.Content>
                <ItemCard.Action>
                  <div className="flex flex-col items-end gap-2" onClick={(e) => e.stopPropagation()}>
                    <Chip size="sm" color={healthChipColor(hm?.status ?? "idle")}>
                      {hm ? `${hm.status} · ${hm.score}` : "未激活"}
                    </Chip>
                    <Switch isSelected={c.enabled} onChange={(v) => toggle(c.id, v)} size="sm" aria-label={`启用 ${c.id}`}>
                      <Switch.Control>
                        <Switch.Thumb />
                      </Switch.Control>
                    </Switch>
                    <div className="flex items-center gap-1">
                      <Tooltip delay={0}>
                        <Button size="sm" variant="ghost" isIconOnly onPress={() => setShareID(c.id)} aria-label={`分享 ${c.id}`}>
                          <Share2 size={12} aria-hidden="true" />
                        </Button>
                        <Tooltip.Content>分享</Tooltip.Content>
                      </Tooltip>
                      <Tooltip delay={0}>
                        <Button
                          size="sm"
                          variant="ghost"
                          isIconOnly
                          isDisabled={busy === `remove:${c.id}`}
                          onPress={() => remove(c.id)}
                          aria-label={`删除 ${c.id}`}
                        >
                          <Trash2 size={12} style={{ color: "var(--yunque-danger)" }} aria-hidden="true" />
                        </Button>
                        <Tooltip.Content>删除</Tooltip.Content>
                      </Tooltip>
                    </div>
                  </div>
                </ItemCard.Action>
              </ItemCard>
            );
          })}
        </ItemCardGroup>
      )}

      {/* ── Bottom disclosure sections ─────────────────────────────── */}

      {/* Templates */}
      <Disclosure isExpanded={showTemplates} onExpandedChange={setShowTemplates}>
        <Disclosure.Heading>
          <Button slot="trigger" variant="ghost" size="sm" className="px-1">
            从模板快速创建
            <Disclosure.Indicator />
          </Button>
        </Disclosure.Heading>
        <Disclosure.Content>
          <Disclosure.Body className="pt-2">
            <ItemCardGroup layout="grid" columns={2}>
              {BUILTIN_TEMPLATES.map((t) => {
                const installed = installedIDs.has(t.id);
                return (
                  <ItemCard key={t.id} variant="outline">
                    <ItemCard.Content>
                      <ItemCard.Title>{t.display_name}</ItemCard.Title>
                      <ItemCard.Description>{t.description}</ItemCard.Description>
                      <Chip size="sm" color="default" className="mt-2 w-fit">{t.category}</Chip>
                    </ItemCard.Content>
                    <ItemCard.Action>
                      {installed ? (
                        <Chip size="sm" color="success">已添加</Chip>
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
                    </ItemCard.Action>
                  </ItemCard>
                );
              })}
            </ItemCardGroup>
          </Disclosure.Body>
        </Disclosure.Content>
      </Disclosure>

      {/* Delivery notes */}
      <Disclosure isExpanded={showDeliveryNotes} onExpandedChange={setShowDeliveryNotes}>
        <Disclosure.Heading>
          <Button slot="trigger" variant="ghost" size="sm" className="px-1">
            Cogni 的正式交付口径
            <Disclosure.Indicator />
          </Button>
        </Disclosure.Heading>
        <Disclosure.Content>
          <Disclosure.Body className="section-card rounded-xl p-4 mt-2">
            <div className="flex flex-col gap-4 lg:flex-row lg:items-start lg:justify-between">
              <p className="text-sm leading-6 max-w-2xl" style={{ color: "var(--yunque-text-secondary)" }}>
                Cogni 现在适合交付为「模型能力组织层」：它兼容 Skill、MCP 和能力包，把可用工具、经验、记忆与触发方式整理成更省上下文的声明。它不是能力包商店，也不是无边界的电脑控制。
              </p>
              <div className="flex flex-wrap gap-2">
                <Chip size="sm" color="success">可交付</Chip>
                <Chip size="sm" color="warning">需观察</Chip>
                <Chip size="sm" color="danger">不夸大</Chip>
              </div>
            </div>
            <div className="mt-4 grid gap-3 lg:grid-cols-3">
              {COGNI_DELIVERY_SECTIONS.map((section) => (
                <section
                  key={section.title}
                  className="rounded-lg p-3"
                  aria-labelledby={`cogni-delivery-${section.tone}`}
                  style={{ background: "var(--yunque-bg-muted)" }}
                >
                  <div className="flex items-center gap-2">
                    <Chip size="sm" color={section.tone}>·</Chip>
                    <h3 id={`cogni-delivery-${section.tone}`} className="text-sm font-medium" style={{ color: "var(--yunque-text)" }}>
                      {section.title}
                    </h3>
                  </div>
                  <ul className="mt-2 list-disc space-y-1 pl-5 text-xs leading-5" style={{ color: "var(--yunque-text-secondary)" }}>
                    {section.items.map((item) => (
                      <li key={item}>{item}</li>
                    ))}
                  </ul>
                </section>
              ))}
            </div>
          </Disclosure.Body>
        </Disclosure.Content>
      </Disclosure>

      {/* Usage notes */}
      <Disclosure isExpanded={showUsageNotes} onExpandedChange={setShowUsageNotes}>
        <Disclosure.Heading>
          <Button slot="trigger" variant="ghost" size="sm" className="px-1">
            云雀如何使用它
            <Disclosure.Indicator />
          </Button>
        </Disclosure.Heading>
        <Disclosure.Content>
          <Disclosure.Body className="section-card rounded-xl p-4 mt-2">
            <ul className="list-disc space-y-1 pl-5 text-sm leading-6" style={{ color: "var(--yunque-text-secondary)" }}>
              {COGNI_USAGE_NOTES.map((item) => (
                <li key={item}>{item}</li>
              ))}
            </ul>
          </Disclosure.Body>
        </Disclosure.Content>
      </Disclosure>

      {/* Developer diagnostics */}
      <Disclosure isExpanded={advancedOpen} onExpandedChange={setAdvancedOpen}>
        <Disclosure.Heading>
          <Button slot="trigger" variant="ghost" size="sm" className="px-1 text-xs" style={{ color: "var(--yunque-text-muted)" }}>
            开发者诊断（Cogni 运行态）
            <Disclosure.Indicator />
          </Button>
        </Disclosure.Heading>
        <Disclosure.Content>
          <Disclosure.Body className="pt-2">
            {runtimePackState ? (
              <div className="section-card rounded-xl p-4">
                <div className="flex flex-wrap items-center gap-2">
                  <span className="text-sm font-medium" style={{ color: "var(--yunque-text)" }}>运行态 Gate</span>
                  <Chip size="sm" color="default" translate="no">{runtimePackState.pack_status || "unknown"}</Chip>
                  <Chip size="sm" color={runtimePackState.runtime_loop_running ? "success" : "warning"}>
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
              </div>
            ) : (
              <div className="text-sm py-2" style={{ color: "var(--yunque-text-muted)" }}>暂无运行态数据</div>
            )}
          </Disclosure.Body>
        </Disclosure.Content>
      </Disclosure>

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
          <div className="text-sm" style={{ color: "var(--yunque-text-secondary)" }}>选择分享方式：</div>
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
            <div className="text-sm" style={{ color: "var(--yunque-text-secondary)" }}>将要导入以下 Cogni：</div>
            <div className="space-y-2 max-h-96 overflow-y-auto" style={{ overscrollBehavior: "contain" }}>
              {((importPreview.cognis as Array<Record<string, unknown>>) ?? []).map((cogni, index) => (
                <div key={index} className="section-card rounded-xl p-3">
                  <div className="font-semibold text-sm" style={{ color: "var(--yunque-text)" }}>
                    {String(cogni.display_name ?? cogni.id ?? "未命名")}
                  </div>
                  {cogni.description ? (
                    <div className="text-xs mt-1" style={{ color: "var(--yunque-text-muted)" }}>
                      {String(cogni.description)}
                    </div>
                  ) : null}
                </div>
              ))}
            </div>
            <div className="flex justify-end gap-2 pt-3 border-t" style={{ borderColor: "var(--yunque-border)" }}>
              <Button size="sm" variant="ghost" onPress={() => setImportPreview(null)}>取消</Button>
              <Button size="sm" className="btn-accent" onPress={importFromPreview}>
                <CheckCircle2 size={14} aria-hidden="true" /> 确认导入
              </Button>
            </div>
          </div>
        )}
      </CherryModal>

      {/* Detail modal — unified capability detail component */}
      <CapabilityDetailModal
        id={detailID}
        displayName={detailEntry?.display_name}
        type="cogni"
        onClose={() => setDetailID(null)}
        declaration={detailDeclaration}
        health={detailID ? health[detailID] : null}
        trace={detailTraces.length > 0 ? { sessions: detailTraces } : null}
        experience={detailExperience}
        evolution={detailEvolution}
        onConfirmPattern={async (patternId: string) => {
          if (!detailID) return;
          try {
            await cogniPack.confirmExperiencePattern(detailID, patternId);
            showToast("模式已确认", "success");
            const exp = await cogniPack.experience(detailID);
            setDetailExperience(exp);
          } catch (e) {
            showToast(formatErrorMessage(e, "确认模式失败"), "error");
          }
        }}
        onRunWorkflow={async (workflowName: string) => {
          if (!detailID) return { success: false, error: "No cogni selected" };
          try {
            const r = await cogniPack.runWorkflow(detailID, workflowName);
            showToast(
              r.success ? `工作流完成：${r.workflow_name}` : `工作流失败：${r.error}`,
              r.success ? "success" : "error"
            );
            return r;
          } catch (e) {
            showToast(formatErrorMessage(e, "执行失败"), "error");
            return { success: false, error: String(e) };
          }
        }}
        onTriggerEvolution={async () => {
          if (!detailID) return;
          try {
            await cogniPack.triggerEvolution(detailID);
            showToast("进化触发成功", "success");
            const evo = await cogniPack.evolution(detailID);
            setDetailEvolution(evo);
          } catch (e) {
            showToast(formatErrorMessage(e, "触发进化失败"), "error");
          }
        }}
        onStartChat={() => {
          router.push(`/chat?cogni=${encodeURIComponent(detailID!)}`);
          setDetailID(null);
        }}
        skillOptions={skills}
        onSaveConfig={async (decl) => {
          if (!detailID) return;
          try {
            await cogniPack.update(detailID, decl);
            showToast("配置已保存", "success");
            const fresh = await cogniPack.get(detailID).catch(() => null);
            setDetailDeclaration(fresh?.declaration ?? decl);
            await load();
          } catch (e) {
            showToast(formatErrorMessage(e, "保存失败"), "error");
          }
        }}
      />
    </div>
  );
}
