"use client";

import { useEffect, useMemo, useState } from "react";
import { Button, Card, Chip, Spinner, TextField, Input, Label } from "@heroui/react";
import {
  ArchiveRestore,
  Boxes,
  CheckCircle2,
  ClipboardCopy,
  ClipboardList,
  DatabaseZap,
  Download,
  ExternalLink,
  ListChecks,
  PackageCheck,
  PackageX,
  Power,
  RotateCcw,
  Route,
  ShieldCheck,
  TerminalSquare,
} from "lucide-react";
import Link from "next/link";
import PageHeader from "@/components/page-header";
import { showToast } from "@/components/toast-provider";
import { createPacksClient, summarizeCapabilityPrepare, type InstalledPack, type PackBackendRouteSpec, type PackCapabilityPlanReport, type PackCapabilityPrepareReport } from "yunque-client/packs";
import { createYunqueSDKClientOptions } from "@/lib/sdk-client";
import {
  buildWorkloadFeedbackPrompt,
  createWorkloadFeedbackEntry,
  emptyWorkloadFeedbackDraft,
  formatWorkloadCapabilities,
  formatWorkloadFeedbackExport,
  formatWorkloadFeedbackFindability,
  hasWorkloadFeedbackContent,
  parseWorkloadFeedbackEntries,
  serializeWorkloadFeedbackEntries,
  WORKLOAD_FEEDBACK_STORAGE_KEY,
  WORKLOAD_PRESETS,
  type WorkloadFeedbackDraft,
  type WorkloadFeedbackEntry,
  type WorkloadFeedbackFindability,
  type WorkloadPreset,
} from "@/lib/workload-presets";
import { useApiData } from "@/lib/use-api-data";
import { formatErrorMessage } from "@/lib/error-utils";
import { formatBackendRouteSpec } from "@/lib/pack-sync";

const EXAMPLE_BACKUP_MANIFEST = "packs/examples/backup-pack/pack.json";
const packsClient = createPacksClient(createYunqueSDKClientOptions());

function formatTime(value?: string): string {
  if (!value) return "-";
  const d = new Date(value);
  if (Number.isNaN(d.getTime())) return value;
  return d.toLocaleString();
}

function sdkImportSnippet(language: string, entry: string): string {
  if (language === "typescript") return `import * as packSdk from "${entry}";`;
  return `${language}:${entry}`;
}

async function copyText(text: string, label: string) {
  try {
    await navigator.clipboard.writeText(text);
    showToast(`${label} 已复制`, "success");
  } catch {
    showToast("复制失败，请手动复制代码片段", "error");
  }
}

function declaredBackendRouteSpecs(pack: InstalledPack): PackBackendRouteSpec[] {
  const specs = pack.manifest.backend?.routeSpecs || [];
  if (specs.length > 0) return specs;
  return (pack.manifest.backend?.routes || []).map((path) => ({ method: "*", path }));
}

function backendRouteKey(route: Pick<PackBackendRouteSpec, "method" | "path">): string {
  return `${route.method.toUpperCase()} ${route.path}`;
}

function statusTone(status: string): { label: string; color: string; bg: string } {
  if (status === "enabled") return { label: "已启用", color: "var(--yunque-success)", bg: "rgba(34,197,94,0.10)" };
  if (status === "disabled") return { label: "已禁用", color: "var(--yunque-text-muted)", bg: "rgba(255,255,255,0.05)" };
  return { label: status || "未知", color: "var(--yunque-warning)", bg: "rgba(245,158,11,0.12)" };
}

export default function PacksPage() {
  const { data, loading, refresh } = useApiData(async () => packsClient.installed(), { packs: [], count: 0 });
  const { data: catalogData, loading: catalogLoading, refresh: refreshCatalog } = useApiData(async () => packsClient.catalog(), {
    generated_at: "",
    sources: [],
    count: 0,
    installed: 0,
    enabled: 0,
    downloadable: 0,
    capabilities: 0,
    entries: [],
  });
  const { data: capabilityData, loading: capabilityLoading, refresh: refreshCapabilities } = useApiData(async () => packsClient.capabilities(), {
    generated_at: "",
    packs: 0,
    enabled_packs: 0,
    capabilities: 0,
    enabled_capabilities: 0,
    entries: [],
  });
  const { data: backendModulesData, loading: backendModulesLoading, refresh: refreshBackendModules } = useApiData(async () => packsClient.backendModules(), { modules: [], count: 0 });
  const { data: routeAuditData, loading: routeAuditLoading, refresh: refreshRouteAudit } = useApiData(async () => packsClient.backendRouteAudit(), {
    generated_at: "",
    packs: 0,
    enabled_packs: 0,
    mounted_modules: 0,
    declared_routes: 0,
    mounted_routes: 0,
    ok_routes: 0,
    missing_routes: 0,
    method_mismatches: 0,
    undeclared_routes: 0,
    entries: [],
  });
  const [manifestPath, setManifestPath] = useState(EXAMPLE_BACKUP_MANIFEST);
  const [manifestUrl, setManifestUrl] = useState("");
  const [downloadArtifact, setDownloadArtifact] = useState(true);
  const [busy, setBusy] = useState<string | null>(null);
  const [capabilityPlanInput, setCapabilityPlanInput] = useState("browser.intent.plan, rpa.replay.plan");
  const [capabilityPlan, setCapabilityPlan] = useState<PackCapabilityPlanReport | null>(null);
  const [capabilityPrepare, setCapabilityPrepare] = useState<PackCapabilityPrepareReport | null>(null);
  const [capabilityPlanBusy, setCapabilityPlanBusy] = useState(false);
  const [activeWorkloadId, setActiveWorkloadId] = useState(WORKLOAD_PRESETS[0]?.id || "");
  const [workloadFeedback, setWorkloadFeedback] = useState<WorkloadFeedbackDraft>(() => emptyWorkloadFeedbackDraft());
  const [workloadFeedbackEntries, setWorkloadFeedbackEntries] = useState<WorkloadFeedbackEntry[]>([]);

  const packs = data?.packs || [];
  const catalog = catalogData;
  const catalogSourceReports = catalog.source_reports || [];
  const catalogSourceIssueCount = catalogSourceReports.filter((report) => !report.ok || (report.errors?.length || 0) > 0).length + (catalog.errors?.length || 0);
  const capabilityCatalogSourceReports = capabilityPrepare?.catalog_source_reports || capabilityPlan?.catalog_source_reports || [];
  const capabilityIndex = capabilityData;
  const backendModules = backendModulesData?.modules || [];
  const routeAudit = routeAuditData;
  const activeWorkload = useMemo(
    () => WORKLOAD_PRESETS.find((preset) => preset.id === activeWorkloadId) || WORKLOAD_PRESETS[0],
    [activeWorkloadId],
  );
  const capabilityEntriesByPack = useMemo(() => {
    const byPack = new Map<string, typeof capabilityIndex.entries>();
    for (const entry of capabilityIndex.entries || []) {
      const current = byPack.get(entry.pack_id) || [];
      current.push(entry);
      byPack.set(entry.pack_id, current);
    }
    return byPack;
  }, [capabilityIndex.entries]);
  const backendModuleByPack = useMemo(() => new Map(backendModules.map((module) => [module.pack_id, module])), [backendModules]);
  const routeAuditByPack = useMemo(() => {
    const byPack = new Map<string, typeof routeAudit.entries>();
    for (const entry of routeAudit.entries || []) {
      const current = byPack.get(entry.pack_id) || [];
      current.push(entry);
      byPack.set(entry.pack_id, current);
    }
    return byPack;
  }, [routeAudit.entries]);
  const stats = useMemo(() => ({
    total: packs.length,
    enabled: packs.filter((p) => p.status === "enabled").length,
    rollbackable: packs.filter((p) => p.manifest.update?.rollback).length,
    frontendMenus: packs.reduce((n, p) => n + (p.manifest.frontend?.menus?.length || 0), 0),
    capabilities: capabilityIndex.capabilities || 0,
    enabledCapabilities: capabilityIndex.enabled_capabilities || 0,
    catalog: catalog.count || 0,
    catalogDownloadable: catalog.downloadable || 0,
    backendModules: backendModules.length,
    backendRoutes: backendModules.reduce((n, m) => n + (m.routes?.length || 0), 0),
    routeAuditIssues: (routeAudit.missing_routes || 0) + (routeAudit.method_mismatches || 0) + (routeAudit.undeclared_routes || 0),
  }), [packs, capabilityIndex.capabilities, capabilityIndex.enabled_capabilities, catalog.count, catalog.downloadable, backendModules, routeAudit.missing_routes, routeAudit.method_mismatches, routeAudit.undeclared_routes]);

  useEffect(() => {
    if (typeof window === "undefined") return;
    setWorkloadFeedbackEntries(parseWorkloadFeedbackEntries(localStorage.getItem(WORKLOAD_FEEDBACK_STORAGE_KEY)));
  }, []);

  const run = async (label: string, op: () => Promise<unknown>) => {
    setBusy(label);
    try {
      await op();
      showToast("Pack registry 已更新，前端菜单会跟随已启用包同步。", "success");
      await refresh();
      await refreshCatalog();
      await refreshCapabilities();
      await refreshBackendModules();
      await refreshRouteAudit();
    } catch (e) {
      showToast(formatErrorMessage(e, "Pack 操作失败"), "error");
    } finally {
      setBusy(null);
    }
  };

  const install = () => run("install", () => packsClient.install({ manifestPath, download: false }));
  const installFromURL = () => run("install-url", () => packsClient.install({ manifestUrl, download: downloadArtifact }));
  const enable = (id: string) => run(`enable:${id}`, () => packsClient.enable(id));
  const disable = (id: string) => run(`disable:${id}`, () => packsClient.disable(id));
  const rollback = (id: string) => run(`rollback:${id}`, () => packsClient.rollback(id));
  const prune = () => run("prune", () => packsClient.prune());
  const runCapabilityPlan = async () => {
    const capabilities = capabilityPlanInput.split(/[,\n]/).map((item) => item.trim()).filter(Boolean);
    if (capabilities.length === 0) {
      showToast("请输入至少一个 capability", "error");
      return;
    }
    setCapabilityPlanBusy(true);
    try {
      const plan = await packsClient.planCapabilities(capabilities);
      setCapabilityPlan(plan);
      setCapabilityPrepare(null);
      showToast(plan.allowed ? "能力预检通过" : "能力预检已生成准备清单", plan.allowed ? "success" : "info");
    } catch (e) {
      showToast(formatErrorMessage(e, "能力预检失败"), "error");
    } finally {
      setCapabilityPlanBusy(false);
    }
  };
  const runCapabilityPrepare = async () => {
    const capabilities = capabilityPlanInput.split(/[,\n]/).map((item) => item.trim()).filter(Boolean);
    if (capabilities.length === 0) {
      showToast("请输入至少一个 capability", "error");
      return;
    }
    setCapabilityPlanBusy(true);
    try {
      const prepare = await packsClient.prepareCapabilities(capabilities);
      setCapabilityPlan(prepare.plan);
      setCapabilityPrepare(prepare);
      showToast(prepare.allowed ? "增量包准备完成" : "增量包准备清单已生成", prepare.allowed ? "success" : "info");
    } catch (e) {
      showToast(formatErrorMessage(e, "准备清单生成失败"), "error");
    } finally {
      setCapabilityPlanBusy(false);
    }
  };

  const updateWorkloadFeedback = <K extends keyof WorkloadFeedbackDraft>(key: K, value: WorkloadFeedbackDraft[K]) => {
    setWorkloadFeedback((current) => ({ ...current, [key]: value }));
  };

  const persistWorkloadFeedbackEntries = (entries: WorkloadFeedbackEntry[]) => {
    const next = entries.slice(0, 30);
    setWorkloadFeedbackEntries(next);
    if (typeof window !== "undefined") {
      localStorage.setItem(WORKLOAD_FEEDBACK_STORAGE_KEY, serializeWorkloadFeedbackEntries(next));
    }
  };

  const saveWorkloadFeedback = () => {
    if (!activeWorkload) return;
    if (!hasWorkloadFeedbackContent(workloadFeedback)) {
      showToast("先记录一点真实体验，再保存反馈。", "error");
      return;
    }
    const entry = createWorkloadFeedbackEntry(activeWorkload, workloadFeedback);
    persistWorkloadFeedbackEntries([entry, ...workloadFeedbackEntries]);
    setWorkloadFeedback(emptyWorkloadFeedbackDraft());
    showToast("工作负载反馈已保存到本地，可一键复制给维护者。", "success");
  };

  const clearWorkloadFeedback = () => {
    persistWorkloadFeedbackEntries([]);
    if (typeof window !== "undefined") localStorage.removeItem(WORKLOAD_FEEDBACK_STORAGE_KEY);
    showToast("本地工作负载反馈已清空", "success");
  };

  const applyWorkloadPreset = (preset: WorkloadPreset) => {
    setActiveWorkloadId(preset.id);
    setCapabilityPlanInput(formatWorkloadCapabilities(preset));
    setCapabilityPlan(null);
    setCapabilityPrepare(null);
  };

  if (loading) {
    return <div className="flex items-center justify-center h-[60vh]"><Spinner size="lg" /></div>;
  }

  return (
    <div className="page-root space-y-5 animate-fade-in-up">
      <PageHeader
        icon={<Boxes size={20} />}
        title="增量包运行时"
        description="Pack Runtime 以后端 registry 为能力来源：安装、启用、禁用、回滚后，前端菜单和入口自动跟随已启用包同步。SDK 在这里按“工作负载”理解：不是只给开发者，而是给用户选用的一组可选能力。"
        onRefresh={() => { void refresh(); void refreshCatalog(); void refreshCapabilities(); void refreshBackendModules(); void refreshRouteAudit(); }}
      />

      <div className="grid grid-cols-2 md:grid-cols-6 gap-3">
        <Card className="section-card p-4">
          <div className="kpi-label">已安装 Pack</div>
          <div className="kpi-value">{stats.total}</div>
        </Card>
        <Card className="section-card p-4">
          <div className="kpi-label">已启用</div>
          <div className="kpi-value">{stats.enabled}</div>
        </Card>
        <Card className="section-card p-4">
          <div className="kpi-label">可回滚</div>
          <div className="kpi-value">{stats.rollbackable}</div>
        </Card>
        <Card className="section-card p-4">
          <div className="kpi-label">能力索引</div>
          <div className="kpi-value">{stats.enabledCapabilities}/{stats.capabilities}</div>
        </Card>
        <Card className="section-card p-4">
          <div className="kpi-label">可选目录</div>
          <div className="kpi-value">{stats.catalogDownloadable}/{stats.catalog}</div>
        </Card>
        <Card className="section-card p-4">
          <div className="kpi-label">路由审计问题</div>
          <div className="kpi-value">{stats.routeAuditIssues}</div>
        </Card>
      </div>

      <Card className="section-card p-5 space-y-4">
        <div className="flex items-start justify-between gap-3">
          <div>
            <div className="text-sm font-semibold" style={{ color: "var(--yunque-text)" }}>工作负载 / SDK 面</div>
            <div className="text-xs mt-1" style={{ color: "var(--yunque-text-muted)" }}>
              像 Visual Studio workloads 一样，先选一条用户真的会走的路，再逐步展开能力。这里的 SDK 不是“开发者专属工具箱”，而是“可以按场景启用的增量工作负载”。
            </div>
          </div>
          <Chip size="sm" style={{ background: "rgba(59,130,246,0.12)", color: "var(--yunque-primary)" }}>
            Workloads
          </Chip>
        </div>

        <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-3">
          {WORKLOAD_PRESETS.map((preset) => (
            <Card key={preset.id} className="section-card p-4 space-y-3" style={{ background: "rgba(255,255,255,0.02)" }}>
              <div className="flex items-start justify-between gap-2">
                <div>
                  <div className="text-sm font-semibold" style={{ color: "var(--yunque-text)" }}>{preset.title}</div>
                  <div className="text-[11px] mt-1" style={{ color: "var(--yunque-text-muted)" }}>{preset.subtitle}</div>
                </div>
                <Chip size="sm" style={{ background: "rgba(255,255,255,0.06)", color: "var(--yunque-text-muted)" }}>
                  工作负载
                </Chip>
              </div>

              <div className="text-xs leading-5" style={{ color: "var(--yunque-text-secondary)" }}>
                {preset.description}
              </div>

              <div className="flex flex-wrap gap-1.5">
                {preset.capabilities.map((capability) => (
                  <Chip key={`${preset.id}:${capability}`} size="sm" style={{ background: "rgba(59,130,246,0.08)", color: "var(--yunque-primary)" }}>
                    {capability}
                  </Chip>
                ))}
              </div>

              <div className="flex flex-wrap gap-2">
                <Button size="sm" variant="primary" onPress={() => applyWorkloadPreset(preset)} className="btn-accent">
                  套用
                </Button>
                <Button size="sm" variant="ghost" onPress={() => void copyText(buildWorkloadFeedbackPrompt(preset), `${preset.title} 反馈模板`)}>
                  <ClipboardCopy size={14} className="mr-1" />
                  复制反馈模板
                </Button>
              </div>
            </Card>
          ))}
        </div>

        <div className="rounded-xl p-4 space-y-3" style={{ background: "rgba(255,255,255,0.03)", border: "1px solid var(--yunque-border)" }}>
          <div className="flex items-start justify-between gap-3">
            <div>
              <div className="text-sm font-semibold" style={{ color: "var(--yunque-text)" }}>体验反馈采集</div>
              <div className="text-xs mt-1" style={{ color: "var(--yunque-text-muted)" }}>
                先让自己或朋友按一个 workload 跑真实任务，再记录“找不到 / 多一步 / 不顺手”的地方。反馈只保存在本地，方便你复制成 issue、PR 描述或产品复盘。
              </div>
            </div>
            <Chip size="sm" style={{ background: "rgba(34,197,94,0.10)", color: "var(--yunque-success)" }}>
              {workloadFeedbackEntries.length} 条
            </Chip>
          </div>

          <div className="grid gap-3 lg:grid-cols-[minmax(0,1fr)_minmax(0,1fr)]">
            <div className="space-y-3">
              <div className="grid gap-3 md:grid-cols-2">
                <label className="text-xs space-y-1" style={{ color: "var(--yunque-text-muted)" }}>
                  <span className="block">工作负载</span>
                  <select
                    value={activeWorkloadId}
                    onChange={(event) => setActiveWorkloadId(event.target.value)}
                    className="w-full rounded-lg px-3 py-2 text-xs outline-none"
                    style={{ background: "rgba(255,255,255,0.05)", border: "1px solid var(--yunque-border)", color: "var(--yunque-text)" }}
                  >
                    {WORKLOAD_PRESETS.map((preset) => (
                      <option key={preset.id} value={preset.id}>{preset.title}</option>
                    ))}
                  </select>
                </label>
                <label className="text-xs space-y-1" style={{ color: "var(--yunque-text-muted)" }}>
                  <span className="block">30 秒内能找到入口吗</span>
                  <select
                    value={workloadFeedback.foundIn30Seconds}
                    onChange={(event) => updateWorkloadFeedback("foundIn30Seconds", event.target.value as WorkloadFeedbackFindability)}
                    className="w-full rounded-lg px-3 py-2 text-xs outline-none"
                    style={{ background: "rgba(255,255,255,0.05)", border: "1px solid var(--yunque-border)", color: "var(--yunque-text)" }}
                  >
                    <option value="unknown">未记录</option>
                    <option value="yes">能</option>
                    <option value="partial">大致能</option>
                    <option value="no">不能</option>
                  </select>
                </label>
              </div>

              <div className="grid gap-3 md:grid-cols-2">
                <TextField value={workloadFeedback.triedScenario} onChange={(value: string) => updateWorkloadFeedback("triedScenario", value)}>
                  <Label>真实场景</Label>
                  <Input placeholder="例如：用浏览器/RPA 自动完成一次网页资料收集" />
                </TextField>
                <TextField value={workloadFeedback.mostUseful} onChange={(value: string) => updateWorkloadFeedback("mostUseful", value)}>
                  <Label>最顺手</Label>
                  <Input placeholder="例如：套用后能直接看到准备清单" />
                </TextField>
              </div>

              <label className="text-xs space-y-1 block" style={{ color: "var(--yunque-text-muted)" }}>
                <span className="block">最不顺手 / 卡点</span>
                <textarea
                  value={workloadFeedback.friction}
                  onChange={(event) => updateWorkloadFeedback("friction", event.target.value)}
                  className="w-full min-h-20 rounded-lg px-3 py-2 text-xs outline-none"
                  placeholder="例如：不知道下一步要点“能力预检”还是“生成准备清单”"
                  style={{ background: "rgba(255,255,255,0.05)", border: "1px solid var(--yunque-border)", color: "var(--yunque-text)" }}
                />
              </label>

              <TextField value={workloadFeedback.nextStepToRemove} onChange={(value: string) => updateWorkloadFeedback("nextStepToRemove", value)}>
                <Label>下次希望少掉哪一步</Label>
                <Input placeholder="例如：套用 workload 后自动跑预检" />
              </TextField>

              <div className="flex flex-wrap gap-2">
                <Button size="sm" className="btn-accent" isDisabled={!hasWorkloadFeedbackContent(workloadFeedback)} onPress={saveWorkloadFeedback}>
                  保存本地反馈
                </Button>
                <Button size="sm" variant="ghost" onPress={() => void copyText(formatWorkloadFeedbackExport(workloadFeedbackEntries), "工作负载反馈汇总")}>
                  <ClipboardCopy size={14} className="mr-1" />
                  复制汇总
                </Button>
                <Button size="sm" variant="ghost" isDisabled={workloadFeedbackEntries.length === 0} onPress={clearWorkloadFeedback}>
                  清空
                </Button>
              </div>
            </div>

            <div className="space-y-2">
              <div className="text-xs font-semibold" style={{ color: "var(--yunque-text)" }}>最近反馈</div>
              {workloadFeedbackEntries.length === 0 ? (
                <div className="text-xs rounded-xl p-3" style={{ color: "var(--yunque-text-muted)", background: "rgba(255,255,255,0.03)", border: "1px dashed var(--yunque-border)" }}>
                  还没有反馈。建议从“浏览器 / RPA”或“记忆 / 回溯”开始，跑一个真实任务后只记录一个卡点；别急着做大闭环，先保证信号足够真实。
                </div>
              ) : (
                workloadFeedbackEntries.slice(0, 4).map((entry) => (
                  <div key={entry.id} className="rounded-xl p-3 text-xs space-y-1" style={{ background: "rgba(255,255,255,0.03)", border: "1px solid var(--yunque-border)", color: "var(--yunque-text-muted)" }}>
                    <div className="flex items-center gap-2 flex-wrap">
                      <Chip size="sm" style={{ background: "rgba(59,130,246,0.10)", color: "var(--yunque-primary)" }}>{entry.workloadTitle}</Chip>
                      <span>{formatWorkloadFeedbackFindability(entry.foundIn30Seconds)}</span>
                      <span className="font-mono">{formatTime(entry.createdAt)}</span>
                    </div>
                    {entry.triedScenario && <div>场景：{entry.triedScenario}</div>}
                    {entry.friction && <div>卡点：{entry.friction}</div>}
                    {entry.nextStepToRemove && <div>少一步：{entry.nextStepToRemove}</div>}
                  </div>
                ))
              )}
            </div>
          </div>
        </div>
      </Card>

      <Card className="section-card p-5 space-y-4">
        <div className="flex items-start justify-between gap-3">
          <div>
            <div className="text-sm font-semibold" style={{ color: "var(--yunque-text)" }}>安装本地 pack manifest</div>
            <div className="text-xs mt-1" style={{ color: "var(--yunque-text-muted)" }}>
              最小闭环先支持本地 manifest；后续下载源/签名校验可以继续沉淀到 Pack Runtime，而不是压进主系统。
            </div>
          </div>
          <div className="flex items-center gap-2">
            <Chip size="sm" style={{ background: "rgba(0,111,238,0.10)", color: "var(--yunque-accent)" }}>
              backend registry source-of-truth
            </Chip>
            <Button size="sm" variant="ghost" isDisabled={busy === "prune"} onPress={prune}>
              清理未引用缓存
            </Button>
          </div>
        </div>
        <div className="grid grid-cols-1 xl:grid-cols-2 gap-3">
          <div className="flex flex-col md:flex-row gap-3">
            <TextField value={manifestPath} onChange={(v: string) => setManifestPath(v)} className="flex-1">
              <Label>manifest_path</Label>
              <Input placeholder={EXAMPLE_BACKUP_MANIFEST} />
            </TextField>
            <Button className="btn-accent md:self-end" isDisabled={!manifestPath || busy === "install"} onPress={install}>
              <Download size={14} /> 安装本地
            </Button>
          </div>
          <div className="flex flex-col md:flex-row gap-3">
            <TextField value={manifestUrl} onChange={(v: string) => setManifestUrl(v)} className="flex-1">
              <Label>manifest_url</Label>
              <Input placeholder="https://packs.example/backup-pack/pack.json" />
            </TextField>
            <div className="flex flex-col gap-2 md:self-end">
              <label className="text-xs flex items-center gap-2" style={{ color: "var(--yunque-text-muted)" }}>
                <input type="checkbox" checked={downloadArtifact} onChange={(event) => setDownloadArtifact(event.target.checked)} />
                下载并校验 distribution.packageUrl
              </label>
              <Button variant="outline" isDisabled={!manifestUrl || busy === "install-url"} onPress={installFromURL}>
                <Download size={14} /> 下载安装
              </Button>
            </div>
          </div>
        </div>
      </Card>

      <Card className="section-card p-5 space-y-4">
        <div className="flex items-start justify-between gap-3">
          <div>
            <div className="flex items-center gap-2 text-sm font-semibold" style={{ color: "var(--yunque-text)" }}>
              <PackageCheck size={15} /> 可选增量包目录
            </div>
            <div className="text-xs mt-1" style={{ color: "var(--yunque-text-muted)" }}>
              从 Pack catalog 读取可安装 manifest，和已安装 registry 对齐；用户可以按 capability 选择需要下载或启用的增量包。
            </div>
          </div>
          <Chip
            size="sm"
            style={{
              background: catalogSourceIssueCount ? "rgba(245,158,11,0.12)" : "rgba(0,111,238,0.10)",
              color: catalogSourceIssueCount ? "var(--yunque-warning)" : "var(--yunque-accent)",
            }}
          >
            {catalogLoading ? "扫描中" : catalogSourceIssueCount ? `${catalogSourceIssueCount} source issues` : `${catalog.downloadable}/${catalog.count} downloadable`}
          </Chip>
        </div>
        <div className="rounded-xl p-3 space-y-3" style={{ background: "rgba(255,255,255,0.03)", border: "1px solid var(--yunque-border)" }}>
          <div className="flex items-start justify-between gap-3">
            <div>
              <div className="text-xs font-semibold" style={{ color: "var(--yunque-text)" }}>当前目录源</div>
              <div className="text-[11px] mt-1" style={{ color: "var(--yunque-text-muted)" }}>
                源扫描结果来自 `/v1/packs/catalog` 的 source_reports，便于定位私有 pack 源、嵌套目录或单个 pack.json 的读取问题。
              </div>
            </div>
            <Chip size="sm" style={{ background: "rgba(255,255,255,0.05)", color: "var(--yunque-text-muted)" }}>
              {catalog.sources.length} sources
            </Chip>
          </div>
          <div className="flex flex-wrap gap-1.5">
            {catalog.sources.length ? catalog.sources.map((source) => (
              <Chip key={source} size="sm" style={{ background: "rgba(0,111,238,0.10)", color: "var(--yunque-accent)" }}>
                {source}
              </Chip>
            )) : <span className="text-xs" style={{ color: "var(--yunque-text-muted)" }}>未配置 catalog source，运行时会回退默认源。</span>}
          </div>
          {catalogSourceReports.length > 0 && (
            <div className="grid grid-cols-1 lg:grid-cols-2 gap-2">
              {catalogSourceReports.map((report) => (
                <div key={report.source} className="rounded-lg p-3 space-y-2" style={{ background: report.ok ? "rgba(34,197,94,0.05)" : "rgba(245,158,11,0.08)", border: `1px solid ${report.ok ? "rgba(34,197,94,0.18)" : "rgba(245,158,11,0.25)"}` }}>
                  <div className="flex items-center justify-between gap-2">
                    <code className="text-[11px] truncate" style={{ color: "var(--yunque-text)" }}>{report.source}</code>
                    <Chip size="sm" style={{ background: report.ok ? "rgba(34,197,94,0.10)" : "rgba(245,158,11,0.12)", color: report.ok ? "var(--yunque-success)" : "var(--yunque-warning)" }}>
                      {report.ok ? "ok" : "error"}
                    </Chip>
                  </div>
                  <div className="flex flex-wrap gap-1.5">
                    <Chip size="sm">manifest_count {report.manifest_count}</Chip>
                    <Chip size="sm">matched_entries {report.matched_entries}</Chip>
                  </div>
                  {(report.errors || []).length > 0 && (
                    <div className="space-y-1">
                      {report.errors?.map((error) => (
                        <div key={error} className="text-[11px] font-mono break-all" style={{ color: "var(--yunque-warning)" }}>
                          {error}
                        </div>
                      ))}
                    </div>
                  )}
                </div>
              ))}
            </div>
          )}
          {(catalog.errors || []).length > 0 && (
            <div className="rounded-lg p-3 space-y-1" style={{ background: "rgba(245,158,11,0.08)", border: "1px solid rgba(245,158,11,0.25)" }}>
              <div className="text-xs font-semibold" style={{ color: "var(--yunque-warning)" }}>Catalog 顶层错误</div>
              {catalog.errors?.map((error) => (
                <div key={error} className="text-[11px] font-mono break-all" style={{ color: "var(--yunque-text-muted)" }}>
                  {error}
                </div>
              ))}
            </div>
          )}
        </div>
        {catalog.entries.length === 0 ? (
          <div className="text-xs rounded-xl p-3" style={{ color: "var(--yunque-text-muted)", background: "rgba(255,255,255,0.03)", border: "1px solid var(--yunque-border)" }}>
            暂无 catalog manifest。请确认 `packs/examples` 或企业私有 pack 源已配置。
          </div>
        ) : (
          <div className="grid grid-cols-1 lg:grid-cols-2 xl:grid-cols-3 gap-3">
            {catalog.entries.slice(0, 9).map((entry) => (
              <div key={entry.manifest.id} className="rounded-xl p-3 space-y-2" style={{ background: "rgba(255,255,255,0.03)", border: "1px solid var(--yunque-border)" }}>
                <div className="flex items-center justify-between gap-2">
                  <div className="min-w-0">
                    <div className="text-xs font-semibold truncate" style={{ color: "var(--yunque-text)" }}>{entry.manifest.name}</div>
                    <div className="text-[11px] font-mono truncate" style={{ color: "var(--yunque-text-muted)" }}>{entry.manifest.id}</div>
                  </div>
                  <Chip size="sm" style={{ background: entry.installed ? "rgba(34,197,94,0.10)" : "rgba(0,111,238,0.10)", color: entry.installed ? "var(--yunque-success)" : "var(--yunque-accent)" }}>
                    {entry.update_action}
                  </Chip>
                </div>
                <div className="flex flex-wrap gap-1.5">
                  {(entry.manifest.backend?.capabilities || []).slice(0, 4).map((capability) => <Chip key={capability} size="sm">{capability}</Chip>)}
                  {entry.downloadable && <Chip size="sm" style={{ background: "rgba(34,197,94,0.10)", color: "var(--yunque-success)" }}>downloadable</Chip>}
                </div>
                <div className="text-[11px] font-mono truncate" style={{ color: "var(--yunque-text-muted)" }}>
                  {entry.manifest.distribution?.packageUrl || entry.manifest_path || "-"}
                </div>
                <div className="flex gap-2">
                  {entry.update_action === "enable" ? (
                    <Button size="sm" className="btn-accent" isDisabled={busy === `enable:${entry.manifest.id}`} onPress={() => enable(entry.manifest.id)}>
                      <Power size={13} /> 启用
                    </Button>
                  ) : (
                    <Button
                      size="sm"
                      variant="outline"
                      isDisabled={!entry.manifest_path || entry.update_action === "use" || busy === `catalog-install:${entry.manifest.id}`}
                      onPress={() => run(`catalog-install:${entry.manifest.id}`, () => packsClient.install({ manifestPath: entry.manifest_path || "", source: entry.source, download: false }))}
                    >
                      <Download size={13} /> 安装
                    </Button>
                  )}
                  {entry.manifest_path && (
                    <Button size="sm" variant="ghost" onPress={() => { setManifestPath(entry.manifest_path || ""); showToast("manifest_path 已填入安装栏", "success"); }}>
                      <ClipboardCopy size={13} /> 填入
                    </Button>
                  )}
                </div>
              </div>
            ))}
          </div>
        )}
      </Card>

      <Card className="section-card p-5 space-y-4">
        <div className="flex items-start justify-between gap-3">
          <div>
            <div className="flex items-center gap-2 text-sm font-semibold" style={{ color: "var(--yunque-text)" }}>
              <DatabaseZap size={15} /> 后端模块 Registry
            </div>
            <div className="text-xs mt-1" style={{ color: "var(--yunque-text-muted)" }}>
              对齐 pack manifest 与 Gateway 实际挂载路由；这里为空时，说明前端能看到包，但后端能力还没有通过 RegisterBackendPack 接入。
            </div>
          </div>
          <Chip size="sm" style={{ background: "rgba(34,197,94,0.10)", color: "var(--yunque-success)" }}>
            {backendModulesLoading ? "同步中" : `${stats.backendModules} modules / ${stats.backendRoutes} routes`}
          </Chip>
        </div>
        {backendModules.length === 0 ? (
          <div className="text-xs rounded-xl p-3" style={{ color: "var(--yunque-text-muted)", background: "rgba(255,255,255,0.03)", border: "1px solid var(--yunque-border)" }}>
            暂无已挂载 backend pack module。请确认 Gateway 启动时已注册内置 pack，或外部包已调用 RegisterBackendPack。
          </div>
        ) : (
          <div className="grid grid-cols-1 lg:grid-cols-2 gap-3">
            {backendModules.map((module) => (
              <div key={module.pack_id} className="rounded-xl p-3" style={{ background: "rgba(255,255,255,0.03)", border: "1px solid var(--yunque-border)" }}>
                <div className="text-xs font-mono mb-2" style={{ color: "var(--yunque-text)" }}>{module.pack_id}</div>
                <div className="flex flex-wrap gap-1.5">
                  {module.routes.map((route) => (
                    <Chip key={route.path} size="sm" style={{ background: "rgba(0,111,238,0.10)", color: "var(--yunque-accent)" }}>
                      {route.method ? `${route.method} ` : ""}{route.path}
                    </Chip>
                  ))}
                </div>
              </div>
            ))}
          </div>
        )}
      </Card>

      <Card className="section-card p-5 space-y-4">
        <div className="flex items-start justify-between gap-3">
          <div>
            <div className="flex items-center gap-2 text-sm font-semibold" style={{ color: "var(--yunque-text)" }}>
              <TerminalSquare size={15} /> Pack 能力索引
            </div>
            <div className="text-xs mt-1" style={{ color: "var(--yunque-text-muted)" }}>
              从 manifest 的 backend.capabilities 生成只读索引，后续 runtime skill gate、插件市场和自动化脚本都可以按 capability 查到所属 pack、启停状态、路由、权限与 SDK 入口。
            </div>
          </div>
          <Chip size="sm" style={{ background: "rgba(0,111,238,0.10)", color: "var(--yunque-accent)" }}>
            {capabilityLoading ? "索引中" : `${stats.enabledCapabilities}/${stats.capabilities} enabled`}
          </Chip>
        </div>
        {capabilityIndex.entries.length === 0 ? (
          <div className="text-xs rounded-xl p-3" style={{ color: "var(--yunque-text-muted)", background: "rgba(255,255,255,0.03)", border: "1px solid var(--yunque-border)" }}>
            暂无 backend.capabilities 声明；能力包需要先在 manifest 中声明 capability，才会进入运行时索引。
          </div>
        ) : (
          <div className="grid grid-cols-1 lg:grid-cols-2 gap-3">
            {capabilityIndex.entries.slice(0, 12).map((entry) => (
              <div key={`${entry.pack_id}:${entry.capability}`} className="rounded-xl p-3" style={{ background: "rgba(255,255,255,0.03)", border: "1px solid var(--yunque-border)" }}>
                <div className="flex items-center gap-2 flex-wrap">
                  <code className="text-xs" style={{ color: "var(--yunque-text)" }}>{entry.capability}</code>
                  <Chip size="sm" style={{ background: entry.enabled ? "rgba(34,197,94,0.10)" : "rgba(255,255,255,0.05)", color: entry.enabled ? "var(--yunque-success)" : "var(--yunque-text-muted)" }}>
                    {entry.enabled ? "enabled" : entry.pack_status}
                  </Chip>
                </div>
                <div className="text-[11px] mt-2 font-mono" style={{ color: "var(--yunque-text-muted)" }}>{entry.pack_id}</div>
                <div className="flex flex-wrap gap-1.5 mt-2">
                  {(entry.routes || []).slice(0, 3).map((route) => <Chip key={route} size="sm">{route}</Chip>)}
                  {entry.sdk_typescript && <Chip size="sm" style={{ background: "rgba(0,111,238,0.10)", color: "var(--yunque-accent)" }}>{entry.sdk_typescript}</Chip>}
                </div>
              </div>
            ))}
          </div>
        )}
      </Card>

      <Card className="section-card p-5 space-y-4">
        <div className="flex items-start justify-between gap-3">
          <div>
            <div className="flex items-center gap-2 text-sm font-semibold" style={{ color: "var(--yunque-text)" }}>
              <ListChecks size={15} /> 工作流能力预检
            </div>
            <div className="text-xs mt-1" style={{ color: "var(--yunque-text-muted)" }}>
              多能力任务执行前先生成 Pack 准备清单，保持只读，不自动启用、不安装、不下载。
            </div>
          </div>
          <Chip size="sm" style={{ background: capabilityPlan?.allowed ? "rgba(34,197,94,0.10)" : "rgba(0,111,238,0.10)", color: capabilityPlan?.allowed ? "var(--yunque-success)" : "var(--yunque-accent)" }}>
            {capabilityPlan ? capabilityPlan.action : "planCapabilities()"}
          </Chip>
        </div>
        <div className="flex flex-col lg:flex-row gap-3">
          <TextField value={capabilityPlanInput} onChange={(v: string) => setCapabilityPlanInput(v)} className="flex-1">
            <Label>capabilities</Label>
            <Input placeholder="browser.intent.plan, rpa.replay.plan, wasm.remote_install.plan" />
          </TextField>
          <div className="flex gap-2 lg:self-end">
            <Button
              variant="ghost"
              isDisabled={capabilityIndex.entries.length === 0}
              onPress={() => setCapabilityPlanInput(capabilityIndex.entries.slice(0, 5).map((entry) => entry.capability).join(", "))}
            >
              <TerminalSquare size={14} /> 使用索引
            </Button>
            <Button className="btn-accent" isDisabled={capabilityPlanBusy} onPress={() => void runCapabilityPlan()}>
              <ListChecks size={14} /> 预检
            </Button>
            <Button className="btn-accent" isDisabled={capabilityPlanBusy} onPress={() => void runCapabilityPrepare()}>
              <ClipboardList size={14} /> 准备清单
            </Button>
          </div>
        </div>
        {capabilityPrepare && (
          <div className="rounded-xl p-3 space-y-3" style={{ background: "rgba(0,111,238,0.05)", border: "1px solid rgba(0,111,238,0.18)" }}>
            <div className="flex items-center justify-between gap-2">
              <div>
                <div className="text-xs font-semibold" style={{ color: "var(--yunque-accent)" }}>增量包准备清单</div>
                <div className="text-[11px] mt-1" style={{ color: "var(--yunque-text-muted)" }}>
                  只读计划，不自动下载、不自动启用；用户按需选择下载、安装或启用。
                </div>
              </div>
              <div className="flex items-center gap-2">
                <Button size="sm" variant="ghost" onPress={() => void copyText(JSON.stringify(summarizeCapabilityPrepare(capabilityPrepare.plan, capabilityPrepare), null, 2), "准备清单摘要")}>
                  <ClipboardCopy size={13} /> 复制准备清单摘要
                </Button>
                <Chip size="sm" style={{ background: capabilityPrepare.allowed ? "rgba(34,197,94,0.10)" : "rgba(245,158,11,0.12)", color: capabilityPrepare.allowed ? "var(--yunque-success)" : "var(--yunque-warning)" }}>
                  {capabilityPrepare.action} · {capabilityPrepare.step_count} steps
                </Chip>
              </div>
            </div>
            <div className="grid grid-cols-2 md:grid-cols-5 gap-2">
              {[
                ["ready", capabilityPrepare.ready_count],
                ["enable", capabilityPrepare.enable_count],
                ["install", capabilityPrepare.install_count],
                ["download", capabilityPrepare.download_count],
                ["audit", capabilityPrepare.route_audit_issue_count],
              ].map(([label, value]) => (
                <div key={label} className="rounded-lg p-3" style={{ background: "rgba(255,255,255,0.035)", border: "1px solid var(--yunque-border)" }}>
                  <div className="text-[11px] uppercase tracking-wide" style={{ color: "var(--yunque-text-muted)" }}>{label}</div>
                  <div className="text-base font-semibold" style={{ color: "var(--yunque-text)" }}>{value}</div>
                </div>
              ))}
            </div>
            <div className="grid grid-cols-1 lg:grid-cols-2 gap-2">
              {capabilityPrepare.steps.map((step, index) => (
                <div key={`${step.action}:${step.pack_id || step.capability || index}:${step.package_url || step.manifest_path || ""}`} className="rounded-lg p-3 space-y-2" style={{ background: "rgba(255,255,255,0.035)", border: "1px solid var(--yunque-border)" }}>
                  <div className="flex items-center justify-between gap-2">
                    <div className="min-w-0">
                      <div className="text-xs font-semibold truncate" style={{ color: "var(--yunque-text)" }}>{step.pack_name || step.capability || step.action}</div>
                      <div className="text-[11px] font-mono truncate" style={{ color: "var(--yunque-text-muted)" }}>{step.pack_id || step.capability || "-"}</div>
                    </div>
                    <Chip size="sm" style={{ background: step.action === "use" ? "rgba(34,197,94,0.10)" : "rgba(0,111,238,0.10)", color: step.action === "use" ? "var(--yunque-success)" : "var(--yunque-accent)" }}>
                      {step.action}
                    </Chip>
                  </div>
                  {step.reason && <div className="text-[11px]" style={{ color: "var(--yunque-text-muted)" }}>{step.reason}</div>}
                  {(step.package_url || step.manifest_path || step.sha256) && (
                    <div className="space-y-1 text-[11px] font-mono" style={{ color: "var(--yunque-text-muted)" }}>
                      {step.package_url && <div className="truncate">pkg {step.package_url}</div>}
                      {step.manifest_path && <div className="truncate">manifest {step.manifest_path}</div>}
                      {step.sha256 && <div className="truncate">sha256 {step.sha256}</div>}
                    </div>
                  )}
                  <div className="flex flex-wrap gap-2">
                    {step.action === "enable" && step.pack_id && (
                      <Button size="sm" className="btn-accent" isDisabled={busy === `enable:${step.pack_id}`} onPress={() => enable(step.pack_id || "")}>
                        <Power size={13} /> 启用
                      </Button>
                    )}
                    {step.action === "install" && step.manifest_path && (
                      <Button size="sm" className="btn-accent" isDisabled={busy === `prepare-install:${step.pack_id}`} onPress={() => run(`prepare-install:${step.pack_id}`, () => packsClient.install({ manifestPath: step.manifest_path || "", source: "prepare", download: false }))}>
                        <Download size={13} /> 安装
                      </Button>
                    )}
                    {step.action === "download" && step.manifest_path && (
                      <Button size="sm" variant="outline" isDisabled={busy === `prepare-download:${step.pack_id}`} onPress={() => run(`prepare-download:${step.pack_id}`, () => packsClient.install({ manifestPath: step.manifest_path || "", source: "prepare", download: true }))}>
                        <Download size={13} /> 下载并安装
                      </Button>
                    )}
                    {step.manifest_path && (
                      <Button size="sm" variant="ghost" onPress={() => { setManifestPath(step.manifest_path || ""); showToast("准备清单 manifest_path 已填入安装栏", "success"); }}>
                        <ClipboardCopy size={13} /> 填入
                      </Button>
                    )}
                  </div>
                </div>
              ))}
            </div>
          </div>
        )}
        {capabilityPlan && (
          <div className="space-y-3">
            {!capabilityPrepare && (
              <div className="flex justify-end">
                <Button size="sm" variant="ghost" onPress={() => void copyText(JSON.stringify(summarizeCapabilityPrepare(capabilityPlan), null, 2), "预检摘要")}>
                  <ClipboardCopy size={13} /> 复制预检摘要
                </Button>
              </div>
            )}
            <div className="grid grid-cols-2 md:grid-cols-5 gap-2">
              {[
                ["allowed", capabilityPlan.allowed_count],
                ["blocked", capabilityPlan.blocked_count],
                ["enable", capabilityPlan.enable_count],
                ["install", capabilityPlan.install_count],
                ["audit", capabilityPlan.route_audit_issue_count],
              ].map(([label, value]) => (
                <div key={label} className="rounded-xl p-3" style={{ background: "rgba(255,255,255,0.03)", border: "1px solid var(--yunque-border)" }}>
                  <div className="text-[11px] uppercase tracking-wide" style={{ color: "var(--yunque-text-muted)" }}>{label}</div>
                  <div className="text-lg font-semibold" style={{ color: "var(--yunque-text)" }}>{value}</div>
                </div>
              ))}
            </div>
            <div className="grid grid-cols-1 lg:grid-cols-3 gap-3">
              <div className="rounded-xl p-3" style={{ background: "rgba(34,197,94,0.06)", border: "1px solid rgba(34,197,94,0.16)" }}>
                <div className="text-xs font-semibold mb-2" style={{ color: "var(--yunque-success)" }}>可直接使用</div>
                <div className="flex flex-wrap gap-1.5">
                  {(capabilityPlan.required_packs || []).length > 0 ? capabilityPlan.required_packs?.map((entry) => (
                    <Chip key={`${entry.pack_id}:${entry.capability}`} size="sm">{entry.capability}</Chip>
                  )) : <span className="text-xs" style={{ color: "var(--yunque-text-muted)" }}>暂无</span>}
                </div>
              </div>
              <div className="rounded-xl p-3" style={{ background: "rgba(0,111,238,0.06)", border: "1px solid rgba(0,111,238,0.16)" }}>
                <div className="text-xs font-semibold mb-2" style={{ color: "var(--yunque-accent)" }}>需要启用</div>
                <div className="flex flex-wrap gap-1.5">
                  {(capabilityPlan.enable_packs || []).length > 0 ? capabilityPlan.enable_packs?.map((entry) => (
                    <Chip key={`${entry.pack_id}:${entry.capability}`} size="sm">{entry.pack_name || entry.pack_id}</Chip>
                  )) : <span className="text-xs" style={{ color: "var(--yunque-text-muted)" }}>暂无</span>}
                </div>
              </div>
              <div className="rounded-xl p-3" style={{ background: "rgba(245,158,11,0.08)", border: "1px solid rgba(245,158,11,0.22)" }}>
                <div className="text-xs font-semibold mb-2" style={{ color: "var(--yunque-warning)" }}>需要安装或修复</div>
                <div className="flex flex-wrap gap-1.5">
                  {(capabilityPlan.install_capabilities || []).map((capability) => <Chip key={capability} size="sm">{capability}</Chip>)}
                  {(capabilityPlan.route_audit_issues || []).map((entry) => <Chip key={`${entry.pack_id}:${entry.path}:${entry.status}`} size="sm">{entry.status} {entry.path}</Chip>)}
                  {(capabilityPlan.catalog_download_hints || []).map((entry) => <Chip key={`download:${entry.manifest.id}`} size="sm" style={{ background: "rgba(34,197,94,0.10)", color: "var(--yunque-success)" }}>可下载 {entry.manifest.name}</Chip>)}
                  {!(capabilityPlan.install_capabilities?.length || capabilityPlan.route_audit_issues?.length || capabilityPlan.catalog_download_hints?.length) && <span className="text-xs" style={{ color: "var(--yunque-text-muted)" }}>暂无</span>}
                </div>
              </div>
            </div>
            {capabilityCatalogSourceReports.length > 0 && (
              <div className="rounded-xl p-3 space-y-3" style={{ background: "rgba(255,255,255,0.03)", border: "1px solid var(--yunque-border)" }}>
                <div className="flex items-center justify-between gap-2">
                  <div>
                    <div className="text-xs font-semibold" style={{ color: "var(--yunque-text)" }}>能力预检 Catalog 源诊断</div>
                    <div className="text-[11px] mt-1" style={{ color: "var(--yunque-text-muted)" }}>
                      来自 plan/prepare 的 catalog_source_reports：即使没有匹配到推荐包，也能看到每个 source 是否可读。
                    </div>
                  </div>
                  <Chip size="sm" style={{ background: "rgba(255,255,255,0.05)", color: "var(--yunque-text-muted)" }}>
                    {capabilityCatalogSourceReports.length} sources
                  </Chip>
                </div>
                <div className="grid grid-cols-1 lg:grid-cols-2 gap-2">
                  {capabilityCatalogSourceReports.map((report) => (
                    <div key={report.source} className="rounded-lg p-3 space-y-1" style={{ background: report.ok ? "rgba(34,197,94,0.05)" : "rgba(245,158,11,0.08)", border: "1px solid var(--yunque-border)" }}>
                      <div className="flex items-center justify-between gap-2">
                        <div className="text-[11px] font-mono truncate" style={{ color: "var(--yunque-text-muted)" }}>{report.source}</div>
                        <Chip size="sm" style={{ background: report.ok ? "rgba(34,197,94,0.10)" : "rgba(245,158,11,0.12)", color: report.ok ? "var(--yunque-success)" : "var(--yunque-warning)" }}>
                          {report.ok ? "ok" : "error"}
                        </Chip>
                      </div>
                      <div className="text-[11px]" style={{ color: "var(--yunque-text-muted)" }}>
                        manifests {report.manifest_count} · matched {report.matched_entries}
                      </div>
                      {(report.errors || []).map((error) => (
                        <div key={error} className="text-[11px] font-mono break-all" style={{ color: "var(--yunque-warning)" }}>{error}</div>
                      ))}
                    </div>
                  ))}
                </div>
              </div>
            )}
            {(capabilityPlan.catalog_install_hints || []).length > 0 && (
              <div className="rounded-xl p-3 space-y-3" style={{ background: "rgba(0,111,238,0.05)", border: "1px solid rgba(0,111,238,0.18)" }}>
                <div className="flex items-center justify-between gap-2">
                  <div>
                    <div className="text-xs font-semibold" style={{ color: "var(--yunque-accent)" }}>推荐增量包</div>
                    <div className="text-[11px] mt-1" style={{ color: "var(--yunque-text-muted)" }}>
                      根据缺失 capability 从 catalog 匹配到可安装包，安装后再启用即可补齐工作流能力。
                    </div>
                  </div>
                  <Chip size="sm" style={{ background: "rgba(0,111,238,0.10)", color: "var(--yunque-accent)" }}>
                    {capabilityPlan.catalog_install_hints?.length || 0} hints
                  </Chip>
                </div>
                <div className="grid grid-cols-1 lg:grid-cols-2 gap-2">
                  {(capabilityPlan.catalog_install_hints || []).map((entry) => (
                    <div key={entry.manifest.id} className="rounded-lg p-3 space-y-2" style={{ background: "rgba(255,255,255,0.035)", border: "1px solid var(--yunque-border)" }}>
                      <div className="flex items-center justify-between gap-2">
                        <div className="min-w-0">
                          <div className="text-xs font-semibold truncate" style={{ color: "var(--yunque-text)" }}>{entry.manifest.name}</div>
                          <div className="text-[11px] font-mono truncate" style={{ color: "var(--yunque-text-muted)" }}>{entry.manifest.id}</div>
                        </div>
                        {entry.downloadable && <Chip size="sm" style={{ background: "rgba(34,197,94,0.10)", color: "var(--yunque-success)" }}>downloadable</Chip>}
                      </div>
                      <div className="flex flex-wrap gap-1.5">
                        {(entry.manifest.backend?.capabilities || []).slice(0, 5).map((capability) => <Chip key={capability} size="sm">{capability}</Chip>)}
                      </div>
                      <div className="text-[11px] font-mono truncate" style={{ color: "var(--yunque-text-muted)" }}>
                        {entry.manifest.distribution?.packageUrl || entry.manifest_path || "-"}
                      </div>
                      <div className="flex gap-2">
                        <Button
                          size="sm"
                          className="btn-accent"
                          isDisabled={!entry.manifest_path || busy === `plan-install:${entry.manifest.id}`}
                        onPress={() => run(`plan-install:${entry.manifest.id}`, () => packsClient.install({ manifestPath: entry.manifest_path || "", source: entry.source, download: Boolean(entry.downloadable) }))}
                        >
                          <Download size={13} /> 安装推荐包
                        </Button>
                        {entry.manifest_path && (
                          <Button size="sm" variant="ghost" onPress={() => { setManifestPath(entry.manifest_path || ""); showToast("推荐包 manifest_path 已填入安装栏", "success"); }}>
                            <ClipboardCopy size={13} /> 填入
                          </Button>
                        )}
                      </div>
                    </div>
                  ))}
                </div>
              </div>
            )}
            <div className="space-y-2">
              {capabilityPlan.gates.map((gate) => (
                <div key={gate.capability} className="rounded-xl p-3 text-xs" style={{ background: gate.allowed ? "rgba(34,197,94,0.05)" : "rgba(255,255,255,0.03)", border: "1px solid var(--yunque-border)", color: "var(--yunque-text-muted)" }}>
                  <div className="flex items-center gap-2 flex-wrap">
                    <Chip size="sm" style={{ background: gate.allowed ? "rgba(34,197,94,0.10)" : "rgba(245,158,11,0.12)", color: gate.allowed ? "var(--yunque-success)" : "var(--yunque-warning)" }}>
                      {gate.allowed ? "allowed" : gate.action}
                    </Chip>
                    <code>{gate.capability}</code>
                    {gate.resolution.preferred?.pack_id && <span className="font-mono">{gate.resolution.preferred.pack_id}</span>}
                  </div>
                  {gate.reason && <div className="mt-1">{gate.reason}</div>}
                </div>
              ))}
            </div>
          </div>
        )}
      </Card>

      <Card className="section-card p-5 space-y-4">
        <div className="flex items-start justify-between gap-3">
          <div>
            <div className="flex items-center gap-2 text-sm font-semibold" style={{ color: "var(--yunque-text)" }}>
              <ShieldCheck size={15} /> 后端路由审计
            </div>
            <div className="text-xs mt-1" style={{ color: "var(--yunque-text-muted)" }}>
              对齐 manifest routeSpecs 与 Gateway 实际挂载：missing / method-mismatch / undeclared 都会暴露为可追踪问题，避免 pack 看似启用但 HTTP surface 漂移。
            </div>
          </div>
          <Chip size="sm" style={{ background: stats.routeAuditIssues ? "rgba(245,158,11,0.12)" : "rgba(34,197,94,0.10)", color: stats.routeAuditIssues ? "var(--yunque-warning)" : "var(--yunque-success)" }}>
            {routeAuditLoading ? "审计中" : `${routeAudit.ok_routes} ok / ${stats.routeAuditIssues} issues`}
          </Chip>
        </div>
        <div className="grid grid-cols-2 md:grid-cols-5 gap-2">
          {[
            ["declared", routeAudit.declared_routes],
            ["mounted", routeAudit.mounted_routes],
            ["missing", routeAudit.missing_routes],
            ["method-mismatch", routeAudit.method_mismatches],
            ["undeclared", routeAudit.undeclared_routes],
          ].map(([label, value]) => (
            <div key={label} className="rounded-xl p-3" style={{ background: "rgba(255,255,255,0.03)", border: "1px solid var(--yunque-border)" }}>
              <div className="text-[11px] uppercase tracking-wide" style={{ color: "var(--yunque-text-muted)" }}>{label}</div>
              <div className="text-lg font-semibold" style={{ color: "var(--yunque-text)" }}>{value}</div>
            </div>
          ))}
        </div>
        {(routeAudit.entries || []).filter((entry) => entry.status !== "ok").slice(0, 8).length > 0 ? (
          <div className="space-y-2">
            {(routeAudit.entries || []).filter((entry) => entry.status !== "ok").slice(0, 8).map((entry) => (
              <div key={`${entry.pack_id}:${entry.method}:${entry.path}:${entry.status}`} className="rounded-xl p-3 text-xs" style={{ background: "rgba(245,158,11,0.08)", border: "1px solid rgba(245,158,11,0.25)", color: "var(--yunque-text-muted)" }}>
                <div className="flex items-center gap-2 flex-wrap">
                  <Chip size="sm" style={{ background: "rgba(245,158,11,0.12)", color: "var(--yunque-warning)" }}>{entry.status}</Chip>
                  <code>{entry.method || "*" } {entry.path}</code>
                  <span>{entry.pack_name || entry.pack_id}</span>
                </div>
                {entry.issues?.length ? <div className="mt-1">{entry.issues.join("；")}</div> : null}
              </div>
            ))}
          </div>
        ) : (
          <div className="text-xs rounded-xl p-3" style={{ color: "var(--yunque-text-muted)", background: "rgba(34,197,94,0.06)", border: "1px solid rgba(34,197,94,0.18)" }}>
            当前 manifest routeSpecs 与已挂载 backend modules 对齐；该审计只读，不修改 Pack registry。
          </div>
        )}
      </Card>

      {packs.length === 0 ? (
        <Card className="section-card p-12 text-center">
          <Boxes size={40} className="mx-auto mb-3" style={{ color: "var(--yunque-text-muted)" }} />
          <div className="text-sm font-medium" style={{ color: "var(--yunque-text)" }}>还没有安装增量包</div>
          <div className="text-xs mt-1" style={{ color: "var(--yunque-text-muted)" }}>
            先安装 backup-pack 示例包，验证 install → enable → frontend sync 最小闭环。
          </div>
          <Button className="btn-accent mt-4" isDisabled={busy === "install"} onPress={install}>
            <Download size={14} /> 安装 backup-pack
          </Button>
        </Card>
      ) : (
        <div className="grid grid-cols-1 xl:grid-cols-2 gap-4">
          {packs.map((pack) => {
            const tone = statusTone(pack.status);
            const manifest = pack.manifest;
            const menus = manifest.frontend?.menus || [];
            const routes = manifest.frontend?.routes || [];
            const caps = manifest.backend?.capabilities || [];
            const capabilityEntries = capabilityEntriesByPack.get(manifest.id) || [];
            const backendModule = backendModuleByPack.get(manifest.id);
            const mountedRoutes = backendModule?.routes || [];
            const routeAuditEntries = routeAuditByPack.get(manifest.id) || [];
            const routeAuditIssues = routeAuditEntries.filter((entry) => entry.status !== "ok");
            const sdkEntries = Object.entries(manifest.sdk || {}).filter((entry): entry is [string, string] => typeof entry[1] === "string" && entry[1].trim().length > 0);
            const distribution = manifest.distribution;
            const declaredBackendSpecs = declaredBackendRouteSpecs(pack);
            const mountedRouteKeySet = new Set(mountedRoutes.map((route) => backendRouteKey({ method: route.method || "*", path: route.path })));
            const mountedPathSet = new Set(mountedRoutes.map((route) => route.path));
            const missingMountedRoutes = declaredBackendSpecs.filter((route) => !mountedRouteKeySet.has(backendRouteKey(route)) && !mountedPathSet.has(route.path));
            return (
              <Card key={manifest.id} className="section-card p-5 space-y-4 hover-lift">
                <div className="flex items-start justify-between gap-3">
                  <div className="min-w-0">
                    <div className="flex items-center gap-2 flex-wrap">
                      <PackageCheck size={16} style={{ color: "var(--yunque-accent)" }} />
                      <span className="font-semibold" style={{ color: "var(--yunque-text)" }}>{manifest.name}</span>
                      <Chip size="sm" style={{ background: tone.bg, color: tone.color }}>{tone.label}</Chip>
                      <Chip size="sm" style={{ background: "rgba(255,255,255,0.05)", color: "var(--yunque-text-muted)" }}>v{manifest.version}</Chip>
                    </div>
                    <div className="text-xs mt-1 font-mono" style={{ color: "var(--yunque-text-muted)" }}>{manifest.id}</div>
                    {manifest.description && (
                      <div className="text-xs mt-2" style={{ color: "var(--yunque-text-muted)" }}>{manifest.description}</div>
                    )}
                  </div>
                  <div className="flex items-center gap-2 shrink-0">
                    {pack.status === "enabled" ? (
                      <Button size="sm" variant="outline" isDisabled={busy === `disable:${manifest.id}`} onPress={() => disable(manifest.id)}>
                        <PackageX size={14} /> 禁用
                      </Button>
                    ) : (
                      <Button size="sm" className="btn-accent" isDisabled={busy === `enable:${manifest.id}`} onPress={() => enable(manifest.id)}>
                        <Power size={14} /> 启用
                      </Button>
                    )}
                    <Button
                      size="sm"
                      variant="ghost"
                      isDisabled={!manifest.update?.rollback || !pack.previousVersion || busy === `rollback:${manifest.id}`}
                      onPress={() => rollback(manifest.id)}
                    >
                      <RotateCcw size={14} /> 回滚
                    </Button>
                  </div>
                </div>

                <div className="grid grid-cols-1 md:grid-cols-3 gap-3">
                  <div className="rounded-xl p-3" style={{ background: "rgba(255,255,255,0.03)", border: "1px solid var(--yunque-border)" }}>
                    <div className="flex items-center gap-1.5 text-xs font-medium mb-2" style={{ color: "var(--yunque-text)" }}>
                      <ShieldCheck size={12} /> 后端能力
                    </div>
                    <div className="flex flex-wrap gap-1.5">
                      {caps.length ? caps.map((cap) => <Chip key={cap} size="sm">{cap}</Chip>) : <span className="text-xs" style={{ color: "var(--yunque-text-muted)" }}>未声明</span>}
                      {capabilityEntries.length > 0 && <Chip size="sm" style={{ background: "rgba(0,111,238,0.10)", color: "var(--yunque-accent)" }}>indexed {capabilityEntries.length}</Chip>}
                      {mountedRoutes.length > 0 && <Chip size="sm" style={{ background: "rgba(34,197,94,0.10)", color: "var(--yunque-success)" }}>已挂载 {mountedRoutes.length}</Chip>}
                      {declaredBackendSpecs.length > 0 && mountedRoutes.length === 0 && <Chip size="sm" style={{ background: "rgba(245,158,11,0.12)", color: "var(--yunque-warning)" }}>未挂载</Chip>}
                      <Chip size="sm" style={{ background: routeAuditIssues.length ? "rgba(245,158,11,0.12)" : "rgba(34,197,94,0.10)", color: routeAuditIssues.length ? "var(--yunque-warning)" : "var(--yunque-success)" }}>
                        route audit {routeAuditIssues.length ? `${routeAuditIssues.length} issues` : "ok"}
                      </Chip>
                    </div>
                  </div>
                  <div className="rounded-xl p-3" style={{ background: "rgba(255,255,255,0.03)", border: "1px solid var(--yunque-border)" }}>
                    <div className="flex items-center gap-1.5 text-xs font-medium mb-2" style={{ color: "var(--yunque-text)" }}>
                      <Route size={12} /> 前端入口
                    </div>
                    <div className="space-y-1">
                      {menus.length ? menus.map((menu) => (
                        <Link key={menu.key} href={menu.path} className="flex items-center gap-1.5 text-xs hover:underline" style={{ color: "var(--yunque-accent)" }}>
                          <ExternalLink size={11} /> {menu.label} <span className="font-mono opacity-60">{menu.path}</span>
                        </Link>
                      )) : <span className="text-xs" style={{ color: "var(--yunque-text-muted)" }}>未声明</span>}
                    </div>
                  </div>
                  <div className="rounded-xl p-3" style={{ background: "rgba(255,255,255,0.03)", border: "1px solid var(--yunque-border)" }}>
                    <div className="flex items-center gap-1.5 text-xs font-medium mb-2" style={{ color: "var(--yunque-text)" }}>
                      <ArchiveRestore size={12} /> 版本状态
                    </div>
                    <div className="text-xs space-y-1" style={{ color: "var(--yunque-text-muted)" }}>
                      <div>来源：<span className="font-mono">{pack.source || "-"}</span></div>
                      <div>上次更新：{formatTime(pack.updatedAt)}</div>
                      <div>上一版本：{pack.previousVersion || "-"}</div>
                      <div>增量包：<span className="font-mono">{distribution?.packageUrl || "-"}</span></div>
                      <div>前端资源：<span className="font-mono">{distribution?.frontendUrl || "-"}</span></div>
                      <div>校验：<span className="font-mono">{pack.artifacts?.sha256 ? pack.artifacts.sha256.slice(0, 12) + "…" : distribution?.sha256 ? distribution.sha256.slice(0, 12) + "…" : "-"}</span></div>
                      <div>缓存：<span className="font-mono">{pack.artifacts?.packagePath || "-"}</span></div>
                      <div>上一缓存：<span className="font-mono">{pack.previousArtifacts?.packagePath || "-"}</span></div>
                    </div>
                  </div>
                </div>

                {sdkEntries.length > 0 && (
                  <div className="text-xs flex items-start gap-2" style={{ color: "var(--yunque-text-muted)" }}>
                    <TerminalSquare size={13} className="mt-0.5 shrink-0" style={{ color: "var(--yunque-accent)" }} />
                    <span className="space-y-1">
                      <span className="block">SDK 调用入口：</span>
                      {sdkEntries.map(([language, entry]) => {
                        const snippet = sdkImportSnippet(language, entry);
                        return (
                          <button
                            key={language}
                            type="button"
                            className="inline-flex items-center gap-1 rounded-md px-2 py-1 mr-1 mt-1 text-left hover:opacity-80"
                            style={{ background: "rgba(0,111,238,0.10)", color: "var(--yunque-accent)" }}
                            onClick={() => void copyText(snippet, `${language} SDK import`)}
                            title="复制 SDK import 示例"
                          >
                            <ClipboardCopy size={11} />
                            <code>{snippet}</code>
                          </button>
                        );
                      })}
                    </span>
                  </div>
                )}

                {(routes.length > 0 || declaredBackendSpecs.length > 0) && (
                  <div className="text-xs flex items-start gap-2" style={{ color: "var(--yunque-text-muted)" }}>
                    <CheckCircle2 size={13} className="mt-0.5 shrink-0" style={{ color: "var(--yunque-success)" }} />
                    <span>
                      Registry 已声明 {routes.length} 个前端路由、{declaredBackendSpecs.length} 个后端路由。
                      {routes.map((r) => <code key={`fe:${r.path}`} className="mx-1">{r.path}</code>)}
                      {declaredBackendSpecs.map((route) => <code key={`be:${backendRouteKey(route)}`} className="mx-1">{formatBackendRouteSpec(route)}</code>)}
                    </span>
                  </div>
                )}

                {missingMountedRoutes.length > 0 && (
                  <div className="text-xs flex items-start gap-2" style={{ color: "var(--yunque-warning)" }}>
                    <ShieldCheck size={13} className="mt-0.5 shrink-0" />
                    <span>manifest 声明但尚未挂载：{missingMountedRoutes.map((route) => <code key={backendRouteKey(route)} className="mx-1">{formatBackendRouteSpec(route)}</code>)}</span>
                  </div>
                )}

                {routeAuditIssues.length > 0 && (
                  <div className="text-xs flex items-start gap-2" style={{ color: "var(--yunque-warning)" }}>
                    <ShieldCheck size={13} className="mt-0.5 shrink-0" />
                    <span>路由审计问题：{routeAuditIssues.slice(0, 4).map((entry) => <code key={`${entry.status}:${entry.method}:${entry.path}`} className="mx-1">{entry.status} {entry.method || "*"} {entry.path}</code>)}</span>
                  </div>
                )}
              </Card>
            );
          })}
        </div>
      )}
    </div>
  );
}

