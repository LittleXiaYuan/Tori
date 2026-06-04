"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import {
  createCogniKernelPackClient,
  type CogniRuntimePackStateReport,
} from "@/lib/cogni-kernel-pack-client";
import type {
  CogniAlert,
  CogniCheckResult,
  CogniDeclaration,
  CogniEntryStatus,
  CogniEvolutionResponse,
  CogniExperiencePattern,
  CogniExperienceResponse,
  CogniHealthMetrics,
  CogniTrace,
  CogniVerifyResponse,
  CogniWorkflowDef,
  CogniWorkflowStep,
  CogniExperiment,
} from "@/lib/api-types/cogni";
import { Button, Card, Chip, Switch } from "@heroui/react";
import {
  Activity,
  AlertTriangle,
  CheckCircle2,
  ChevronDown,
  Download,
  FlaskConical,
  Globe,
  Lightbulb,
  Play,
  RefreshCw,
  Search,
  ShieldCheck,
  Sparkles,
  Target,
  Trash2,
  Upload,
  Wand2,
  Workflow,
  XCircle,
} from "lucide-react";
import PageHeader from "@/components/page-header";
import { showToast } from "@/components/toast-provider";
import { formatErrorMessage } from "@/lib/error-utils";

type HealthMap = Record<string, CogniHealthMetrics>;

// Color for health status chip.
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

function runtimeGateColor(ready?: boolean): { bg: string; fg: string } {
  return ready
    ? { bg: "rgba(23,201,100,0.12)", fg: "#17c964" }
    : { bg: "rgba(255,170,0,0.12)", fg: "#ffaa00" };
}

const DEMO_COGNI_ID = "code-reviewer";

const ASSISTANT_EXAMPLES = [
  "一个帮我整理每周工作周报、能查资料还能做成 PPT 的助手",
  "一个专门审查代码、盯安全漏洞和风格问题的助手",
  "一个帮我做数据分析、把表格画成图表的助手",
  "一个回复客户咨询、语气亲切专业的客服助手",
];

const cogniPack = createCogniKernelPackClient();

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
  const [cognis, setCognis] = useState<CogniEntryStatus[]>([]);
  const [health, setHealth] = useState<HealthMap>({});
  const [alerts, setAlerts] = useState<CogniAlert[]>([]);
  const [filter, setFilter] = useState("");
  const [loading, setLoading] = useState(true);
  const [reloading, setReloading] = useState(false);
  const [detailID, setDetailID] = useState<string | null>(null);
  const [detailTraces, setDetailTraces] = useState<CogniTrace[]>([]);
  const [detailTab, setDetailTab] = useState<"traces" | "workflows" | "experience" | "evolution">("traces");
  const [detailWorkflows, setDetailWorkflows] = useState<CogniWorkflowDef[]>([]);
  const [detailExperience, setDetailExperience] = useState<CogniExperienceResponse | null>(null);
  const [detailEvolution, setDetailEvolution] = useState<CogniEvolutionResponse | null>(null);
  const [generateDesc, setGenerateDesc] = useState("");
  const [generating, setGenerating] = useState(false);
  const [generatePreview, setGeneratePreview] = useState<CogniDeclaration | null>(null);
  const heroInputRef = useRef<HTMLTextAreaElement>(null);
  const fileInput = useRef<HTMLInputElement>(null);
  const [demoVerify, setDemoVerify] = useState<CogniVerifyResponse | null>(null);
  const [demoTraces, setDemoTraces] = useState<CogniTrace[]>([]);
  const [verifying, setVerifying] = useState(false);
  const [refreshingTrace, setRefreshingTrace] = useState(false);
  const [refreshingHealth, setRefreshingHealth] = useState(false);
  const [confirmingPatternID, setConfirmingPatternID] = useState<string | null>(null);
  const [runtimePackState, setRuntimePackState] = useState<CogniRuntimePackStateReport | null>(null);
  const [advancedOpen, setAdvancedOpen] = useState(false);

  const focusHeroCreate = useCallback(() => {
    heroInputRef.current?.focus();
    heroInputRef.current?.scrollIntoView({ behavior: "smooth", block: "center" });
  }, []);

  const load = useCallback(async () => {
    try {
      const [list, alertsRes, demoTracesRes, runtimeStateRes] = await Promise.all([
        cogniPack.list(),
        cogniPack.alerts().catch(() => ({ alerts: [] as CogniAlert[], count: 0 })),
        cogniPack.tracesByID(DEMO_COGNI_ID, 5).catch(() => ({ traces: [] as CogniTrace[] })),
        cogniPack.runtimePackState().catch(() => null),
      ]);
      setCognis(list.cognis || []);
      setHealth(list.health || {});
      setAlerts(alertsRes.alerts || []);
      setDemoTraces(demoTracesRes.traces || []);
      setRuntimePackState(runtimeStateRes);
    } catch (e) {
      showToast(e instanceof Error ? e.message : "加载失败", "error");
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    load();
  }, [load]);

  const reload = async () => {
    setReloading(true);
    try {
      const r = await cogniPack.reload();
      showToast(
        `已重载：新增 ${r.added} · 更新 ${r.updated} · 移除 ${r.removed}${r.errors?.length ? ` · 错误 ${r.errors.length}` : ""}`,
        r.errors?.length ? "error" : "success",
      );
      await load();
    } catch (e) {
      showToast(e instanceof Error ? e.message : "重载失败", "error");
    } finally {
      setReloading(false);
    }
  };

  const scanAlerts = async () => {
    try {
      const r = await cogniPack.scanAlerts();
      setAlerts(r.alerts || []);
      showToast(`扫描完成：${r.count} 条告警`, "success");
    } catch (e) {
      showToast(e instanceof Error ? e.message : "扫描失败", "error");
    }
  };

  const toggle = async (id: string, enabled: boolean) => {
    try {
      await cogniPack.setEnabled(id, enabled);
      setCognis((prev) =>
        prev.map((c) => (c.id === id ? { ...c, enabled } : c)),
      );
    } catch (e) {
      showToast(e instanceof Error ? e.message : "操作失败", "error");
    }
  };

  const remove = async (id: string) => {
    if (!confirm(`删除智体 ${id}？`)) return;
    try {
      await cogniPack.remove(id);
      showToast("已删除", "success");
      await load();
    } catch (e) {
      showToast(e instanceof Error ? e.message : "删除失败", "error");
    }
  };

  const exportBundle = () => {
    cogniPack.exportBundle();
  };

  const importBundle = async (file: File) => {
    try {
      const text = await file.text();
      const bundle = JSON.parse(text);
      const sum = await cogniPack.importBundle(bundle);
      showToast(
        `导入：新增 ${sum.added?.length || 0} · 更新 ${sum.updated?.length || 0} · 跳过 ${sum.skipped?.length || 0} · 失败 ${sum.failed?.length || 0}`,
        "success",
      );
      await load();
    } catch (e) {
      showToast(e instanceof Error ? e.message : "导入失败", "error");
    }
  };

  const generateCogni = async () => {
    if (!generateDesc.trim()) return;
    setGenerating(true);
    try {
      const r = await cogniPack.generate(generateDesc, true);
      setGeneratePreview(r.declaration);
      showToast(`助手「${r.declaration.display_name ?? r.declaration.id}」已创建`, "success");
      setGenerateDesc("");
      await load();
    } catch (e) {
      showToast(e instanceof Error ? e.message : "创建失败，请换个说法再试", "error");
    } finally {
      setGenerating(false);
    }
  };

  const openDetail = async (id: string) => {
    setDetailID(id);
    setDetailTab("traces");
    try {
      const [traces, workflows, experience, evolution] = await Promise.all([
        cogniPack.tracesByID(id, 20).catch(() => ({ traces: [] })),
        cogniPack.workflows(id).catch(() => ({ workflows: [] })),
        cogniPack.experience(id).catch(() => null),
        cogniPack.evolution(id).catch(() => null),
      ]);
      setDetailTraces(traces.traces || []);
      setDetailWorkflows(workflows.workflows || []);
      setDetailExperience(experience);
      setDetailEvolution(evolution);
    } catch {
      setDetailTraces([]);
    }
  };

  const confirmExperiencePattern = async (patternID: string) => {
    if (!detailID || confirmingPatternID) return;
    setConfirmingPatternID(patternID);
    try {
      await cogniPack.confirmExperiencePattern(detailID, patternID);
      const refreshed = await cogniPack.experience(detailID);
      setDetailExperience(refreshed);
      showToast("经验模式已确认", "success");
    } catch (e) {
      showToast(e instanceof Error ? e.message : "确认失败", "error");
    } finally {
      setConfirmingPatternID(null);
    }
  };

  const runDemoVerify = async () => {
    setVerifying(true);
    try {
      const r = await cogniPack.verify();
      setDemoVerify(r);
      const failed = (r.failures || []).length;
      showToast(failed === 0 ? "所有检查通过" : `${failed} 项检查失败`, failed === 0 ? "success" : "error");
    } catch (e) {
      showToast(e instanceof Error ? e.message : "验证失败", "error");
    } finally {
      setVerifying(false);
    }
  };

  const refreshDemoTraces = async () => {
    setRefreshingTrace(true);
    try {
      const r = await cogniPack.tracesByID(DEMO_COGNI_ID, 5);
      setDemoTraces(r.traces || []);
      showToast(`获取 ${(r.traces || []).length} 条 trace`, "success");
    } catch (e) {
      showToast(e instanceof Error ? e.message : "刷新失败", "error");
    } finally {
      setRefreshingTrace(false);
    }
  };

  const refreshDemoHealth = async () => {
    setRefreshingHealth(true);
    try {
      const r = await cogniPack.health();
      const hmap: HealthMap = {};
      for (const h of r.health || []) hmap[h.id] = h;
      setHealth((prev) => ({ ...prev, ...hmap }));
      showToast("健康状态已刷新", "success");
    } catch (e) {
      showToast(e instanceof Error ? e.message : "刷新失败", "error");
    } finally {
      setRefreshingHealth(false);
    }
  };

  const demoCogni = cognis.find((c) => c.id === DEMO_COGNI_ID);
  const demoHealthData = health[DEMO_COGNI_ID];
  const experienceSummary = detailExperience?.summary;
  const experienceStats = experienceSummary?.stats ?? detailExperience?.stats ?? {};
  const topTools = experienceSummary?.top_tools ?? detailExperience?.tool_memory?.slice(0, 5) ?? [];
  const topFacts = experienceSummary?.top_facts ?? detailExperience?.domain_facts?.slice(0, 5) ?? [];
  const pendingPatterns = experienceSummary?.pending_patterns ?? detailExperience?.patterns?.filter((p) => !p.confirmed).slice(0, 5) ?? [];
  const confirmedPatterns = detailExperience?.patterns?.filter((p) => p.confirmed).slice(0, 5) ?? [];

  const filteredCognis = cognis.filter(
    (c) =>
      !filter ||
      c.id.toLowerCase().includes(filter.toLowerCase()) ||
      (c.display_name ?? "").toLowerCase().includes(filter.toLowerCase()),
  );

  return (
    <div className="page-root flex flex-col gap-5 animate-fade-in-up">
      <PageHeader
        icon={<Sparkles size={20} />}
        title="我的助手"
        description="用大白话描述你想要的助手，云雀自动配好技能、触发方式和人设。"
        onRefresh={load}
        actions={
          <Button size="sm" className="btn-accent" onPress={focusHeroCreate}>
            <Wand2 size={12} /> 新建助手
          </Button>
        }
      />

      {/* Hero — natural-language assistant creation */}
      <Card className="section-card p-5" style={{ borderTop: "2px solid var(--yunque-accent)" }}>
        <div className="flex items-center gap-2 mb-1" style={{ color: "var(--yunque-text)" }}>
          <Wand2 size={16} style={{ color: "var(--yunque-accent)" }} />
          <span className="text-base font-medium">描述你想要的助手，云雀帮你造一个</span>
        </div>
        <p className="text-xs mb-3" style={{ color: "var(--yunque-text-muted)" }}>
          一句话说清它要做什么 —— 云雀会自动配好该用的技能、激活关键词和说话风格，创建后立即可用。
        </p>
        <textarea
          ref={heroInputRef}
          value={generateDesc}
          onChange={(e) => setGenerateDesc(e.target.value)}
          placeholder="例如：一个帮我整理周报、能查资料还能做成 PPT 的助手"
          rows={3}
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
            <Lightbulb size={11} /> 试试：
          </span>
          {ASSISTANT_EXAMPLES.map((ex) => (
            <button
              key={ex}
              onClick={() => setGenerateDesc(ex)}
              className="text-[11px] px-2 py-1 rounded-full transition-colors hover:opacity-80"
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
            isDisabled={!generateDesc.trim()}
          >
            <Sparkles size={12} /> {generating ? "云雀正在造助手…" : "创建助手"}
          </Button>
        </div>
        {generatePreview && (
          <div
            className="mt-4 p-3 rounded-lg"
            style={{ background: "rgba(23,201,100,0.08)", border: "1px solid rgba(23,201,100,0.2)" }}
          >
            <div className="flex items-center gap-2 mb-1">
              <CheckCircle2 size={14} style={{ color: "#17c964" }} />
              <span className="text-sm font-medium" style={{ color: "var(--yunque-text)" }}>
                已为你创建：{generatePreview.display_name ?? generatePreview.id}
              </span>
            </div>
            {generatePreview.description && (
              <p className="text-xs mb-2" style={{ color: "var(--yunque-text-muted)" }}>
                {generatePreview.description}
              </p>
            )}
            {(generatePreview.surface?.only ?? []).length > 0 && (
              <div className="flex flex-wrap items-center gap-1.5">
                <span className="text-[11px]" style={{ color: "var(--yunque-text-muted)" }}>会用到：</span>
                {(generatePreview.surface?.only ?? []).slice(0, 8).map((s) => (
                  <Chip key={s} size="sm" style={{ background: "rgba(255,255,255,0.05)", color: "var(--yunque-text-muted)" }}>
                    {s}
                  </Chip>
                ))}
              </div>
            )}
          </div>
        )}
      </Card>

      {/* Developer diagnostics — pushed to the page bottom via flex order so the page leads with assistants */}
      <div className="flex items-center justify-between gap-2" style={{ order: 90 }}>
        <button
          onClick={() => setAdvancedOpen((v) => !v)}
          className="flex items-center gap-1.5 text-xs"
          style={{ color: "var(--yunque-text-muted)" }}
        >
          <ChevronDown
            size={13}
            style={{ transform: advancedOpen ? "none" : "rotate(-90deg)", transition: "transform .15s" }}
          />
          开发者诊断（运行态 Gate · 演示证据 · 导入导出）
        </button>
        {advancedOpen && (
          <div className="flex gap-1.5">
            <Button size="sm" variant="ghost" onPress={reload} isPending={reloading}>
              <RefreshCw size={11} /> 热重载
            </Button>
            <Button size="sm" variant="ghost" onPress={exportBundle}>
              <Download size={11} /> 导出
            </Button>
            <Button size="sm" variant="ghost" onPress={() => fileInput.current?.click()}>
              <Upload size={11} /> 导入
            </Button>
            <input
              ref={fileInput}
              type="file"
              accept=".json"
              hidden
              onChange={(e) => {
                const f = e.target.files?.[0];
                if (f) importBundle(f);
                e.target.value = "";
              }}
            />
          </div>
        )}
      </div>

      {advancedOpen && runtimePackState && (
        <Card className="section-card p-4 border-l-4" style={{ order: 91, borderLeftColor: runtimePackState.runtime_loop_running ? "#17c964" : "#ffaa00" }}>
          <div className="flex flex-wrap items-start justify-between gap-3">
            <div className="space-y-2">
              <div className="flex flex-wrap items-center gap-2">
                <ShieldCheck size={14} style={{ color: runtimeGateColor(runtimePackState.runtime_loop_pack_state_ready).fg }} />
                <span className="text-sm font-medium" style={{ color: "var(--yunque-text)" }}>
                  运行态 Gate
                </span>
                <Chip size="sm" style={{ background: runtimeGateColor(runtimePackState.runtime_loop_pack_state_ready).bg, color: runtimeGateColor(runtimePackState.runtime_loop_pack_state_ready).fg }}>
                  runtime_loop_pack_state_ready: {String(runtimePackState.runtime_loop_pack_state_ready)}
                </Chip>
                <Chip
                  size="sm"
                  style={{
                    background: runtimePackState.runtime_loop_running ? "rgba(23,201,100,0.12)" : "rgba(255,170,0,0.12)",
                    color: runtimePackState.runtime_loop_running ? "#17c964" : "#ffaa00",
                  }}
                >
                  loop {runtimePackState.runtime_loop_running ? "running" : "stopped"}
                </Chip>
                <Chip size="sm">{runtimePackState.pack_status || "unknown"}</Chip>
              </div>
              <div className="flex flex-wrap gap-2 text-[11px]">
                {[
                  ["pack_enabled", runtimePackState.pack_enabled],
                  ["starts_runtime_loops", runtimePackState.starts_runtime_loops],
                  ["stops_runtime_loops", runtimePackState.stops_runtime_loops],
                  ["clears_runtime_state", runtimePackState.clears_runtime_state],
                  ["sentinel_ready", runtimePackState.sentinel_ready],
                  ["scheduler_ready", runtimePackState.scheduler_ready],
                  ["bus_ready", runtimePackState.bus_ready],
                  ["experience_store_ready", runtimePackState.experience_store_ready],
                ].map(([label, value]) => (
                  <Chip
                    key={String(label)}
                    size="sm"
                    style={{
                      background: value ? "rgba(23,201,100,0.1)" : "rgba(255,255,255,0.04)",
                      color: value ? "#17c964" : "var(--yunque-text-muted)",
                    }}
                  >
                    {label}: {String(value)}
                  </Chip>
                ))}
              </div>
              <div className="flex flex-wrap gap-x-4 gap-y-1 text-[11px]" style={{ color: "var(--yunque-text-muted)" }}>
                <span>active_bus_cognis {runtimePackState.active_bus_cognis}</span>
                <span>experience_store_count {runtimePackState.experience_store_count}</span>
                <span>artifact {runtimePackState.artifacts?.[0] || "cogni-runtime-pack-state.json"}</span>
              </div>
            </div>
            <div className="text-[11px] text-right max-w-md" style={{ color: "var(--yunque-text-muted)" }}>
              该只读报告证明 Cogni runtime loop 已跟随 yunque.pack.cogni-kernel 启停状态；真正切换仍走 /v1/packs/enable 与 /v1/packs/disable。
            </div>
          </div>
        </Card>
      )}

      {/* Demo Evidence Panel */}
      {advancedOpen && !loading && (
        <Card className="section-card p-4 border-l-4" style={{ order: 92, borderLeftColor: "#0091ff" }}>
          <div className="flex items-center justify-between mb-3">
            <div className="flex items-center gap-2">
              <Target size={14} style={{ color: "#0091ff" }} />
              <span className="text-sm font-medium" style={{ color: "var(--yunque-text)" }}>
                演示证据
              </span>
              <span className="text-xs font-mono" style={{ color: "var(--yunque-text-muted)" }}>
                {DEMO_COGNI_ID}
              </span>
            </div>
            <div className="flex gap-1.5">
              <Button size="sm" variant="ghost" onPress={reload} isPending={reloading}>
                <RefreshCw size={10} /> Reload
              </Button>
              <Button size="sm" variant="ghost" onPress={runDemoVerify} isPending={verifying}>
                <ShieldCheck size={10} /> Verify
              </Button>
              <Button size="sm" variant="ghost" onPress={refreshDemoTraces} isPending={refreshingTrace}>
                <Activity size={10} /> Trace
              </Button>
              <Button size="sm" variant="ghost" onPress={refreshDemoHealth} isPending={refreshingHealth}>
                <Globe size={10} /> Health
              </Button>
            </div>
          </div>

          {demoCogni ? (
            <div className="space-y-3">
              <div className="flex flex-wrap items-center gap-2 text-xs">
                <Chip size="sm" style={{ background: "rgba(23,201,100,0.12)", color: "#17c964" }}>
                  已注册
                </Chip>
                <Chip
                  size="sm"
                  style={{
                    background: demoCogni.enabled ? "rgba(23,201,100,0.12)" : "rgba(243,18,96,0.12)",
                    color: demoCogni.enabled ? "#17c964" : "#f31260",
                  }}
                >
                  {demoCogni.enabled ? "已启用" : "已禁用"}
                </Chip>
                {demoHealthData && (
                  <Chip
                    size="sm"
                    style={{
                      background: healthColor(demoHealthData.status).bg,
                      color: healthColor(demoHealthData.status).fg,
                    }}
                  >
                    {demoHealthData.status} · {demoHealthData.score}
                  </Chip>
                )}
              </div>

              {demoHealthData && demoHealthData.evaluations > 0 && (
                <div
                  className="flex flex-wrap gap-x-4 gap-y-1 text-[11px]"
                  style={{ color: "var(--yunque-text-muted)" }}
                >
                  <span>评估 {demoHealthData.evaluations}</span>
                  <span>激活 {demoHealthData.activations}/{demoHealthData.evaluations}</span>
                  <span>上下文 {demoHealthData.avg_context_bytes}B</span>
                  {demoHealthData.tool_filter_ratio > 0 && (
                    <span>工具过滤 {(demoHealthData.tool_filter_ratio * 100).toFixed(0)}%</span>
                  )}
                  {demoHealthData.avg_duration_ms > 0 && (
                    <span>耗时 {demoHealthData.avg_duration_ms}ms</span>
                  )}
                  {demoHealthData.suppressed > 0 && (
                    <span style={{ color: "#ffaa00" }}>抑制 {demoHealthData.suppressed}</span>
                  )}
                </div>
              )}

              {demoVerify ? (
                <div
                  className="p-2.5 rounded-lg"
                  style={{ background: "rgba(255,255,255,0.02)" }}
                >
                  <div className="flex items-center justify-between mb-1.5">
                    <span className="text-xs font-medium" style={{ color: "var(--yunque-text)" }}>
                      Verify 结果
                    </span>
                    {(() => {
                      const checks = demoVerify.results[DEMO_COGNI_ID] || [];
                      const passed = checks.filter((c) => c.passed).length;
                      return (
                        <Chip
                          size="sm"
                          style={{
                            background: passed === checks.length ? "rgba(23,201,100,0.12)" : "rgba(243,18,96,0.12)",
                            color: passed === checks.length ? "#17c964" : "#f31260",
                          }}
                        >
                          {passed}/{checks.length} 通过
                        </Chip>
                      );
                    })()}
                  </div>
                  <div className="space-y-1">
                    {(demoVerify.results[DEMO_COGNI_ID] || []).map((ck, i) => (
                      <div key={i} className="flex items-center gap-2 text-xs">
                        {ck.passed ? (
                          <CheckCircle2 size={12} style={{ color: "#17c964" }} />
                        ) : (
                          <XCircle size={12} style={{ color: "#f31260" }} />
                        )}
                        <span className="font-mono" style={{ color: "var(--yunque-text)" }}>
                          {ck.check_name || `check-${ck.check_index}`}
                        </span>
                        <span style={{ color: "var(--yunque-text-muted)" }}>
                          score={ck.got_score.toFixed(2)} active={String(ck.got_active)}
                        </span>
                      </div>
                    ))}
                  </div>
                </div>
              ) : (
                <div className="text-[11px]" style={{ color: "var(--yunque-text-muted)" }}>
                  点击 Verify 运行声明式自测
                </div>
              )}

              {demoTraces.length > 0 ? (
                <div>
                  <div className="text-xs font-medium mb-1" style={{ color: "var(--yunque-text)" }}>
                    最近 Trace ({demoTraces.length})
                  </div>
                  <div className="space-y-1.5">
                    {demoTraces.slice(0, 3).map((t, i) => {
                      const own = t.activations?.find((a) => a.id === DEMO_COGNI_ID);
                      return (
                        <div
                          key={i}
                          className="p-2 rounded text-xs"
                          style={{ background: "rgba(255,255,255,0.03)" }}
                        >
                          <div className="flex flex-wrap items-center gap-2">
                            <Chip
                              size="sm"
                              style={{
                                background: own?.activated
                                  ? "rgba(23,201,100,0.12)"
                                  : "rgba(255,255,255,0.04)",
                                color: own?.activated ? "#17c964" : "var(--yunque-text-muted)",
                              }}
                            >
                              {own?.activated ? "激活" : "未激活"}
                            </Chip>
                            <span style={{ color: "var(--yunque-text-muted)" }}>
                              {new Date(t.timestamp).toLocaleTimeString()}
                            </span>
                            {t.context && t.context.bytes > 0 && (
                              <span style={{ color: "var(--yunque-text-muted)" }}>
                                上下文 {t.context.bytes}B
                              </span>
                            )}
                            {t.tool_filter?.applied_by?.includes(DEMO_COGNI_ID) && (
                              <span style={{ color: "var(--yunque-text)" }}>
                                工具 {t.tool_filter.before}→{t.tool_filter.after}
                                {t.tool_filter.removed?.length
                                  ? ` 移除 ${t.tool_filter.removed.join(",")}`
                                  : ""}
                              </span>
                            )}
                          </div>
                          {own?.reasons && own.reasons.length > 0 && (
                            <div className="mt-1" style={{ color: "var(--yunque-text-muted)" }}>
                              {own.reasons.join("; ")}
                            </div>
                          )}
                        </div>
                      );
                    })}
                  </div>
                </div>
              ) : (
                <div className="text-xs" style={{ color: "var(--yunque-text-muted)" }}>
                  还没有 trace — 请先在聊天页发送：帮我审查一下这个 PR 的代码
                </div>
              )}
            </div>
          ) : (
            <div
              className="text-xs text-center py-2"
              style={{ color: "var(--yunque-text-muted)" }}
            >
              code-reviewer 未注册 — 将 code-reviewer.cogni.yaml 放入 data/cognis/ 后点击 Reload
            </div>
          )}
        </Card>
      )}

      {/* Alerts banner */}
      {alerts.length > 0 && (
        <Card
          className="section-card p-4 border-l-4"
          style={{ borderLeftColor: "#f31260" }}
        >
          <div className="flex items-center justify-between mb-2">
            <div className="flex items-center gap-2">
              <AlertTriangle size={14} style={{ color: "#f31260" }} />
              <span
                className="text-sm font-medium"
                style={{ color: "var(--yunque-text)" }}
              >
                {alerts.length} 条活跃告警
              </span>
            </div>
            <Button size="sm" variant="ghost" onPress={scanAlerts}>
              立即扫描
            </Button>
          </div>
          <div className="space-y-1.5">
            {alerts.slice(0, 5).map((a) => {
              const sc = severityColor(a.severity);
              return (
                <div
                  key={`${a.cogni_id}|${a.kind}`}
                  className="flex items-start gap-3 text-sm"
                >
                  <Chip
                    size="sm"
                    style={{ background: sc.bg, color: sc.fg }}
                  >
                    {a.severity.toUpperCase()}
                  </Chip>
                  <span
                    className="font-mono text-xs"
                    style={{ color: "var(--yunque-text-muted)" }}
                  >
                    {a.cogni_id}
                  </span>
                  <span style={{ color: "var(--yunque-text)" }}>{a.message}</span>
                  {a.auto_action_taken && (
                    <Chip
                      size="sm"
                      style={{
                        background: "rgba(255,170,0,0.12)",
                        color: "#ffaa00",
                      }}
                    >
                      已{a.auto_action_taken === "disabled" ? "自动禁用" : a.auto_action_taken}
                    </Chip>
                  )}
                </div>
              );
            })}
          </div>
        </Card>
      )}

      {/* Filter */}
      <div className="flex items-center gap-2">
        <div className="relative flex-1 max-w-md">
          <Search
            size={14}
            className="absolute left-3 top-1/2 -translate-y-1/2"
            style={{ color: "var(--yunque-text-muted)" }}
          />
          <input
            type="text"
            value={filter}
            onChange={(e) => setFilter(e.target.value)}
            placeholder="搜索助手…"
            className="w-full pl-9 pr-3 py-1.5 text-sm rounded-md"
            style={{
              background: "rgba(255,255,255,0.04)",
              border: "1px solid rgba(255,255,255,0.08)",
              color: "var(--yunque-text)",
            }}
          />
        </div>
        <span className="text-xs" style={{ color: "var(--yunque-text-muted)" }}>
          {filteredCognis.length} / {cognis.length}
        </span>
      </div>

      {/* Cogni list */}
      {loading ? (
        <Card className="section-card p-10 text-center text-sm" style={{ color: "var(--yunque-text-muted)" }}>
          加载中…
        </Card>
      ) : filteredCognis.length === 0 ? (
        <Card
          className="section-card p-10 text-center text-sm"
          style={{ color: "var(--yunque-text-muted)" }}
        >
          还没有助手 —— 在上面用一句话描述你想要的，云雀帮你造一个。
        </Card>
      ) : (
        <div className="grid grid-cols-1 md:grid-cols-2 gap-3">
          {filteredCognis.map((c) => {
            const hm = health[c.id];
            const hc = healthColor(hm?.status ?? "idle");
            return (
              <Card
                key={c.id}
                className="section-card p-4 cursor-pointer"
                onClick={() => openDetail(c.id)}
              >
                <div className="flex items-start justify-between gap-3 mb-2">
                  <div className="flex items-start gap-3 min-w-0 flex-1">
                    <div
                      className="flex items-center justify-center rounded-xl shrink-0 font-semibold select-none"
                      style={{
                        width: 42,
                        height: 42,
                        background: avatarGradient(c.id),
                        color: "#fff",
                        fontSize: 17,
                      }}
                      aria-hidden
                    >
                      {avatarInitial(c.display_name ?? c.id)}
                    </div>
                    <div className="min-w-0 flex-1">
                    <div className="flex items-center gap-2 mb-1">
                      <span
                        className="font-medium truncate"
                        style={{ color: "var(--yunque-text)" }}
                      >
                        {c.display_name ?? c.id}
                      </span>
                      {c.always_on && (
                        <Chip
                          size="sm"
                          style={{
                            background: "rgba(0,145,255,0.12)",
                            color: "#0091ff",
                          }}
                        >
                          always-on
                        </Chip>
                      )}
                      {c.exclusive && (
                        <Chip
                          size="sm"
                          style={{ background: "rgba(255,255,255,0.04)" }}
                        >
                          g:{c.exclusive}
                        </Chip>
                      )}
                    </div>
                    <div
                      className="text-xs font-mono truncate"
                      style={{ color: "var(--yunque-text-muted)" }}
                    >
                      {c.id}
                    </div>
                    {c.description && (
                      <div
                        className="text-sm mt-2"
                        style={{ color: "var(--yunque-text-muted)" }}
                      >
                        {c.description}
                      </div>
                    )}
                    </div>
                  </div>
                  <div
                    className="flex flex-col items-end gap-2"
                    onClick={(e) => e.stopPropagation()}
                  >
                    <Chip size="sm" style={{ background: hc.bg, color: hc.fg }}>
                      {hm ? `${hm.status} · ${hm.score}` : "idle"}
                    </Chip>
                    <Switch
                      isSelected={c.enabled}
                      onChange={(v) => toggle(c.id, v)}
                      size="sm"
                    >
                      <Switch.Control>
                        <Switch.Thumb />
                      </Switch.Control>
                    </Switch>
                  </div>
                </div>

                {c.load_error && (
                  <div
                    className="mt-2 text-xs p-2 rounded"
                    style={{
                      background: "rgba(243,18,96,0.1)",
                      color: "#f31260",
                    }}
                  >
                    <ShieldCheck size={10} className="inline mr-1" />
                    {formatErrorMessage(c.load_error, "加载失败")}
                  </div>
                )}

                {hm && hm.activations > 0 && (
                  <div
                    className="mt-2 text-[11px] flex gap-3"
                    style={{ color: "var(--yunque-text-muted)" }}
                  >
                    <span>激活 {hm.activations}/{hm.evaluations}</span>
                    {hm.tool_filter_ratio > 0 && (
                      <span>工具比 {(hm.tool_filter_ratio * 100).toFixed(0)}%</span>
                    )}
                    {hm.avg_duration_ms > 0 && (
                      <span>耗时 {hm.avg_duration_ms}ms</span>
                    )}
                    {hm.template_fallback_rate > 0 && (
                      <span style={{ color: "#ffaa00" }}>
                        模板失败 {(hm.template_fallback_rate * 100).toFixed(0)}%
                      </span>
                    )}
                  </div>
                )}

                <div
                  className="mt-2 flex justify-end"
                  onClick={(e) => e.stopPropagation()}
                >
                  <Button
                    size="sm"
                    variant="ghost"
                    isIconOnly
                    onPress={() => remove(c.id)}
                    aria-label={`删除 ${c.id}`}
                  >
                    <Trash2 size={12} style={{ color: "#f31260" }} />
                  </Button>
                </div>
              </Card>
            );
          })}
        </div>
      )}

      {/* Detail drawer */}
      {detailID && (
        <div
          className="fixed inset-0 z-50 flex justify-end"
          style={{ background: "rgba(0,0,0,0.5)" }}
          onClick={() => setDetailID(null)}
        >
          <Card
            className="h-full overflow-y-auto p-5 section-card"
            style={{ width: "min(560px, 100%)" }}
            onClick={(e) => e.stopPropagation()}
          >
            <div className="flex items-center justify-between mb-4">
              <div className="font-mono text-sm" style={{ color: "var(--yunque-text)" }}>
                {detailID}
              </div>
              <Button size="sm" variant="ghost" onPress={() => setDetailID(null)}>关闭</Button>
            </div>

            {/* Tabs */}
            <div className="flex gap-1 mb-4 border-b" style={{ borderColor: "rgba(255,255,255,0.08)" }}>
              {(["traces", "workflows", "experience", "evolution"] as const).map((tab) => (
                <button
                  key={tab}
                  onClick={() => setDetailTab(tab)}
                  className="px-3 py-1.5 text-xs font-medium rounded-t-md transition-colors"
                  style={{
                    color: detailTab === tab ? "var(--yunque-text)" : "var(--yunque-text-muted)",
                    background: detailTab === tab ? "rgba(255,255,255,0.06)" : "transparent",
                    borderBottom: detailTab === tab ? "2px solid var(--yunque-accent)" : "2px solid transparent",
                  }}
                >
                  {tab === "traces" && "Trace"}
                  {tab === "workflows" && "工作流"}
                  {tab === "experience" && "经验"}
                  {tab === "evolution" && "进化"}
                </button>
              ))}
            </div>

            {/* Traces tab */}
            {detailTab === "traces" && (
              <>
                <div className="text-xs mb-2" style={{ color: "var(--yunque-text-muted)" }}>
                  最近 Trace ({detailTraces.length})
                </div>
                {detailTraces.length === 0 ? (
                  <div className="text-sm py-6 text-center" style={{ color: "var(--yunque-text-muted)" }}>
                    暂无 trace — 该智体尚未参与对话
                  </div>
                ) : (
                  <div className="space-y-2">
                    {detailTraces.map((t, i) => {
                      const ownEntry = t.activations?.find((a) => a.id === detailID);
                      const activated = !!ownEntry?.activated;
                      return (
                        <div key={i} className="p-3 rounded-lg text-xs" style={{ background: "rgba(255,255,255,0.03)" }}>
                          <div className="flex items-center gap-2 mb-1">
                            <Chip size="sm" style={{
                              background: activated ? "rgba(23,201,100,0.12)" : "rgba(255,255,255,0.04)",
                              color: activated ? "#17c964" : "var(--yunque-text-muted)",
                            }}>
                              {activated ? "激活" : "未激活"}
                            </Chip>
                            <span style={{ color: "var(--yunque-text-muted)" }}>
                              {new Date(t.timestamp).toLocaleTimeString()}
                            </span>
                            {t.duration_ms > 0 && <span style={{ color: "var(--yunque-text-muted)" }}>{t.duration_ms}ms</span>}
                          </div>
                          {ownEntry?.reasons && ownEntry.reasons.length > 0 && (
                            <div style={{ color: "var(--yunque-text-muted)" }}>{ownEntry.reasons.join("; ")}</div>
                          )}
                          {ownEntry?.suppressed && <div style={{ color: "#ffaa00" }}>被 {ownEntry.suppressed_by} 排他抑制</div>}
                          {t.tool_filter?.applied_by?.includes(detailID!) && (
                            <div style={{ color: "var(--yunque-text)" }}>
                              工具 {t.tool_filter.before} → {t.tool_filter.after}
                              {t.tool_filter.removed && t.tool_filter.removed.length > 0 &&
                                ` · 移除 ${t.tool_filter.removed.slice(0, 3).join(", ")}${t.tool_filter.removed.length > 3 ? "…" : ""}`}
                            </div>
                          )}
                        </div>
                      );
                    })}
                  </div>
                )}
              </>
            )}

            {/* Workflows tab */}
            {detailTab === "workflows" && (
              <>
                <div className="text-xs mb-2" style={{ color: "var(--yunque-text-muted)" }}>
                  <Workflow size={12} className="inline mr-1" />
                  工作流 ({detailWorkflows.length})
                </div>
                {detailWorkflows.length === 0 ? (
                  <div className="text-sm py-6 text-center" style={{ color: "var(--yunque-text-muted)" }}>
                    该智体未定义工作流
                  </div>
                ) : (
                  <div className="space-y-3">
                    {detailWorkflows.map((wf: CogniWorkflowDef) => (
                      <div key={wf.name} className="p-3 rounded-lg" style={{ background: "rgba(255,255,255,0.03)" }}>
                        <div className="flex items-center justify-between mb-2">
                          <span className="text-sm font-medium" style={{ color: "var(--yunque-text)" }}>{wf.name}</span>
                          <Button size="sm" variant="ghost" onPress={async () => {
                            try {
                              const r = await cogniPack.runWorkflow(detailID!, wf.name);
                              showToast(r.success ? `工作流完成：${r.workflow_name}` : `工作流失败：${r.error}`, r.success ? "success" : "error");
                            } catch (e) {
                              showToast(e instanceof Error ? e.message : "执行失败", "error");
                            }
                          }}>
                            <Play size={10} /> 执行
                          </Button>
                        </div>
                        {wf.description && <div className="text-xs mb-2" style={{ color: "var(--yunque-text-muted)" }}>{wf.description}</div>}
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

            {/* Experience tab */}
            {detailTab === "experience" && (
              <>
                <div className="text-xs mb-2" style={{ color: "var(--yunque-text-muted)" }}>
                  经验累积
                </div>
                {!detailExperience?.enabled ? (
                  <div className="text-sm py-6 text-center" style={{ color: "var(--yunque-text-muted)" }}>
                    该智体未启用经验引擎
                  </div>
                ) : (
                  <div className="space-y-3">
                    <div className="grid grid-cols-2 gap-2">
                      {Object.entries(experienceStats).map(([k, v]) => (
                        <div key={k} className="p-2 rounded-lg text-center" style={{ background: "rgba(255,255,255,0.03)" }}>
                          <div className="text-lg font-medium" style={{ color: "var(--yunque-text)" }}>{String(v)}</div>
                          <div className="text-[10px]" style={{ color: "var(--yunque-text-muted)" }}>{k.replace(/_/g, " ")}</div>
                        </div>
                      ))}
                    </div>
                    {experienceSummary?.updated_at && (
                      <div className="text-[11px]" style={{ color: "var(--yunque-text-muted)" }}>
                        Profile 最近更新：{new Date(experienceSummary.updated_at).toLocaleString()}
                      </div>
                    )}
                    {topTools.length > 0 && (
                      <div>
                        <div className="text-xs mb-1 font-medium" style={{ color: "var(--yunque-text)" }}>高频工具经验</div>
                        {topTools.map((tool, i) => (
                          <div key={`${tool.tool}-${i}`} className="text-xs p-2 rounded mb-1" style={{ background: "rgba(34,211,238,0.08)", color: "var(--yunque-text-muted)" }}>
                            <div className="flex items-center justify-between gap-2">
                              <span className="font-medium" style={{ color: "var(--yunque-text)" }}>{tool.tool || "unknown tool"}</span>
                              <span>复用 {tool.used_count ?? 0}</span>
                            </div>
                            {tool.learned && <div>{tool.learned}</div>}
                            {tool.context && <div>场景：{tool.context}</div>}
                          </div>
                        ))}
                      </div>
                    )}
                    {topFacts.length > 0 && (
                      <div>
                        <div className="text-xs mb-1 font-medium" style={{ color: "var(--yunque-text)" }}>高频领域事实</div>
                        {topFacts.map((fact, i) => (
                          <div key={`${fact.fact}-${i}`} className="text-xs p-2 rounded mb-1" style={{ background: "rgba(167,139,250,0.08)", color: "var(--yunque-text-muted)" }}>
                            <div style={{ color: "var(--yunque-text)" }}>{fact.fact}</div>
                            <div>来源 {fact.source || "unknown"} · 复用 {fact.used_count ?? 0}</div>
                          </div>
                        ))}
                      </div>
                    )}
                    {pendingPatterns.length > 0 && (
                      <div>
                        <div className="text-xs mb-1 font-medium" style={{ color: "var(--yunque-text)" }}>待确认模式</div>
                        {pendingPatterns.map((p: CogniExperiencePattern, i: number) => (
                          <div key={p.id || i} className="text-xs p-2 rounded mb-1" style={{ background: "rgba(255,170,0,0.08)", color: "var(--yunque-text-muted)" }}>
                            <div className="flex items-start justify-between gap-2">
                              <span>{p.trigger} → {p.response}</span>
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
                    {confirmedPatterns.length > 0 && (
                      <div>
                        <div className="text-xs mb-1 font-medium" style={{ color: "var(--yunque-text)" }}>已确认行为模式</div>
                        {confirmedPatterns.map((p: CogniExperiencePattern, i: number) => (
                          <div key={i} className="text-xs p-2 rounded mb-1" style={{ background: "rgba(255,255,255,0.03)", color: "var(--yunque-text-muted)" }}>
                            {p.trigger} → {p.response} ✓
                          </div>
                        ))}
                      </div>
                    )}
                  </div>
                )}
              </>
            )}

            {/* Evolution tab */}
            {detailTab === "evolution" && (
              <>
                <div className="flex items-center justify-between mb-2">
                  <div className="text-xs" style={{ color: "var(--yunque-text-muted)" }}>
                    <FlaskConical size={12} className="inline mr-1" />
                    Skill 进化
                  </div>
                  <Button size="sm" variant="ghost" onPress={async () => {
                    try {
                      await cogniPack.triggerEvolution(detailID!);
                      showToast("进化已启动", "success");
                    } catch (e) {
                      showToast(e instanceof Error ? e.message : "启动失败", "error");
                    }
                  }}>
                    <FlaskConical size={10} /> 触发进化
                  </Button>
                </div>
                {detailEvolution?.running && (
                  <Chip size="sm" style={{ background: "rgba(0,145,255,0.12)", color: "#0091ff" }}>进化中…</Chip>
                )}
                {(!detailEvolution?.experiments || detailEvolution.experiments.length === 0) ? (
                  <div className="text-sm py-6 text-center" style={{ color: "var(--yunque-text-muted)" }}>
                    尚无进化实验记录
                  </div>
                ) : (
                  <div className="space-y-2 mt-2">
                    {detailEvolution.experiments.map((exp: CogniExperiment) => (
                      <div key={exp.id} className="p-3 rounded-lg text-xs" style={{ background: "rgba(255,255,255,0.03)" }}>
                        <div className="flex items-center gap-2 mb-1">
                          <Chip size="sm" style={{
                            background: exp.status === "kept" ? "rgba(23,201,100,0.12)" : "rgba(243,18,96,0.12)",
                            color: exp.status === "kept" ? "#17c964" : "#f31260",
                          }}>
                            {exp.status === "kept" ? "保留" : "回滚"}
                          </Chip>
                          <span style={{ color: "var(--yunque-text-muted)" }}>
                            {new Date(exp.date).toLocaleDateString()}
                          </span>
                          <span style={{ color: exp.delta >= 0 ? "#17c964" : "#f31260" }}>
                            {exp.delta >= 0 ? "+" : ""}{exp.delta.toFixed(1)}%
                          </span>
                        </div>
                        <div style={{ color: "var(--yunque-text-muted)" }}>{exp.change}</div>
                        {exp.reason && <div style={{ color: "var(--yunque-text-muted)" }}>{exp.reason}</div>}
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
