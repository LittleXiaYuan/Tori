"use client";

import { useEffect, useMemo, useState } from "react";
import Link from "next/link";
import { Button, Card, Chip, Input, Label, Spinner, TextField } from "@heroui/react";
import {
  ArrowRight,
  Boxes,
  ChevronDown,
  ChevronUp,
  Download,
  ExternalLink,
  FolderInput,
  Globe2,
  LockKeyhole,
  PackageCheck,
  PackageX,
  Pin,
  PinOff,
  Power,
  RotateCcw,
  Search,
  ShieldAlert,
  ShieldCheck,
  SlidersHorizontal,
  Store,
  Wrench,
  X,
} from "lucide-react";
import PageHeader from "@/components/page-header";
import { showToast } from "@/components/toast-provider";
import { createPacksClient, type InstalledPack, type PackCatalogEntry, type PackManifest, type PackReleaseCatalogEntry } from "yunque-client/packs";
import { createYunqueSDKClientOptions } from "@/lib/sdk-client";
import { useApiData } from "@/lib/use-api-data";
import { buildPackNavItems } from "@/lib/pack-sync";
import { useNavigationPreferences } from "@/hooks/use-user-preferences";
import {
  capabilitySurfaceLabels,
  catalogActionForEntry,
  entryInstallRequest,
  formatPackInstallError,
  groupPackPermissions,
  packExamples,
  packFeatureFlags,
  packReadiness,
  packUsageExplanation,
  packUsability,
  riskProfileForPack,
} from "@/lib/pack-presentation";

const PACK_RELEASE_SOURCES = [
  {
    label: "云雀官方能力包源",
    url: "https://github.com/LittleXiaYuan/Tori/releases/tag/pack%2Fmicro-agent%2Fv0.1.0",
    note: "官方发布的 .yqpack 包，安装前会展示版本、权限和风险。",
  },
];
const OFFICIAL_BACKUP_MANIFEST = "packs/official/backup-pack/pack.json";
const packsClient = createPacksClient(createYunqueSDKClientOptions());
const PAGE_SIZE = 12;

type KindFilter = "all" | "actionable" | "infrastructure" | "experimental";
type InstallFilter = "all" | "installed" | "enabled" | "disabled" | "available";
type RiskFilter = "all" | "low" | "medium" | "high";
type SourceFilter = "all" | "installed" | "official" | "private";
type ReadinessFilter = "all" | "complete" | "needs_context" | "needs_entry";
type SortMode = "name" | "kind" | "risk" | "readiness" | "status";

const KIND_FILTER_LABELS: Record<KindFilter, string> = {
  all: "全部类型",
  actionable: "可直接使用",
  infrastructure: "基础能力",
  experimental: "实验中",
};
const INSTALL_FILTER_LABELS: Record<InstallFilter, string> = {
  all: "全部状态",
  installed: "已安装",
  enabled: "已启用",
  disabled: "已禁用",
  available: "可安装",
};
const RISK_FILTER_LABELS: Record<RiskFilter, string> = {
  all: "全部风险",
  low: "低风险",
  medium: "需留意",
  high: "需要授权",
};
const SOURCE_FILTER_LABELS: Record<SourceFilter, string> = {
  all: "全部来源",
  installed: "已安装",
  official: "官方源",
  private: "私有源",
};
const READINESS_FILTER_LABELS: Record<ReadinessFilter, string> = {
  all: "全部体检",
  complete: "说明完整",
  needs_context: "需补说明",
  needs_entry: "需补入口",
};
const SORT_MODE_LABELS: Record<SortMode, string> = {
  name: "按名称",
  kind: "按类型",
  risk: "按风险",
  readiness: "按体检",
  status: "按阶段",
};

function formatTime(value?: string): string {
  if (!value) return "-";
  const d = new Date(value);
  if (Number.isNaN(d.getTime())) return value;
  return d.toLocaleString();
}

function formatBytes(bytes?: number): string {
  if (!bytes || bytes <= 0) return "";
  const units = ["B", "KB", "MB", "GB"];
  const index = Math.min(Math.floor(Math.log(bytes) / Math.log(1024)), units.length - 1);
  return `${(bytes / Math.pow(1024, index)).toFixed(index === 0 ? 0 : 1)} ${units[index]}`;
}

function statusTone(status: string): { label: string; color: string; bg: string } {
  if (status === "enabled") return { label: "已启用", color: "var(--yunque-success)", bg: "rgba(34,197,94,0.10)" };
  if (status === "disabled") return { label: "已禁用", color: "var(--yunque-text-muted)", bg: "rgba(255,255,255,0.05)" };
  return { label: status || "未知", color: "var(--yunque-warning)", bg: "rgba(245,158,11,0.12)" };
}

function packStatusBadge(packStatus?: string): { label: string; color: string; bg: string } {
  if (packStatus === "stable") return { label: "正式版", color: "var(--yunque-success)", bg: "rgba(34,197,94,0.10)" };
  if (packStatus === "beta") return { label: "测试版", color: "var(--yunque-warning)", bg: "rgba(245,158,11,0.12)" };
  if (packStatus === "alpha") return { label: "开发中", color: "var(--yunque-text-muted)", bg: "rgba(255,255,255,0.05)" };
  return { label: "未知", color: "var(--yunque-text-muted)", bg: "rgba(255,255,255,0.05)" };
}

function sourceName(url: string): string {
  try {
    return new URL(url).host;
  } catch {
    return url;
  }
}

function packStudioHref(manifest: PackManifest, options?: { packageUrl?: string; sha256?: string }): string {
  const readiness = packReadiness(manifest);
  const gap = readiness.missing.length > 0
    ? `重点补齐：${readiness.missing.join("、")}。`
    : "继续打磨更具体的用户场景和入口反馈。";
  const goal = `让 ${manifest.name} 更像一个用户能直接理解和使用的能力包，${gap}补齐用途、入口、权限说明和可回滚改造建议。`;
  const params = new URLSearchParams({
    packId: manifest.id,
    goal,
  });
  if (options?.packageUrl) params.set("packageUrl", options.packageUrl);
  if (options?.sha256) params.set("sha256", options.sha256);
  return `/packs/studio?${params.toString()}`;
}

function packSearchText(manifest: PackManifest): string {
  return [
    manifest.id,
    manifest.name,
    manifest.description,
    manifest.version,
    manifest.status,
    manifest.metadata?.usageSurface,
    manifest.metadata?.primaryActionLabel,
    manifest.metadata?.limitation,
    ...packExamples(manifest, 5),
    ...(manifest.backend?.capabilities || []),
    ...(manifest.backend?.permissions || []),
    ...(manifest.backend?.routes || []),
    ...(manifest.backend?.routeSpecs || []).map((route) => `${route.method} ${route.path} ${route.description || ""}`),
  ].filter(Boolean).join(" ").toLowerCase();
}

function sortPacks<T>(items: T[], manifestOf: (item: T) => PackManifest, sortMode: SortMode): T[] {
  const riskOrder = { high: 0, medium: 1, low: 2 } as const;
  const kindOrder = { actionable: 0, infrastructure: 1, experimental: 2, documented: 3 } as const;
  const readinessOrder = { needs_entry: 0, needs_context: 1, complete: 2 } as const;
  return [...items].sort((a, b) => {
    const ma = manifestOf(a);
    const mb = manifestOf(b);
    if (sortMode === "risk") return riskOrder[riskProfileForPack(ma).level] - riskOrder[riskProfileForPack(mb).level] || ma.name.localeCompare(mb.name);
    if (sortMode === "kind") return kindOrder[packUsability(ma).kind] - kindOrder[packUsability(mb).kind] || ma.name.localeCompare(mb.name);
    if (sortMode === "readiness") return readinessOrder[packReadiness(ma).level] - readinessOrder[packReadiness(mb).level] || ma.name.localeCompare(mb.name);
    if (sortMode === "status") return String(ma.status || "").localeCompare(String(mb.status || "")) || ma.name.localeCompare(mb.name);
    return ma.name.localeCompare(mb.name);
  });
}

function pageCountFor(total: number): number {
  return Math.max(1, Math.ceil(total / PAGE_SIZE));
}

function paginate<T>(items: T[], page: number): T[] {
  return items.slice((page - 1) * PAGE_SIZE, page * PAGE_SIZE);
}

function renderFilterGroup(
  label: string,
  options: Array<[string, string]>,
  value: string,
  onChange: (value: string) => void,
) {
  return (
    <div>
      <div className="mb-2 text-xs font-medium" style={{ color: "var(--yunque-text-muted)" }}>{label}</div>
      <div className="flex flex-wrap gap-1.5">
        {options.map(([key, text]) => (
          <Button
            key={key}
            size="sm"
            variant="ghost"
            className={value === key ? "btn-accent" : undefined}
            aria-pressed={value === key}
            onPress={() => onChange(key)}
          >
            {text}
          </Button>
        ))}
      </div>
    </div>
  );
}

function renderPagination(
  label: string,
  currentPage: number,
  pageCount: number,
  total: number,
  onPrev: () => void,
  onNext: () => void,
) {
  if (total <= PAGE_SIZE) return null;
  return (
    <div className="flex flex-wrap items-center justify-between gap-3">
      <div className="text-xs" style={{ color: "var(--yunque-text-muted)" }}>
        {label} · 第 {currentPage} / {pageCount} 页 · 共 {total} 个
      </div>
      <div className="flex items-center gap-2">
        <Button size="sm" variant="ghost" isDisabled={currentPage <= 1} onPress={onPrev}>
          上一页
        </Button>
        <Button size="sm" variant="ghost" isDisabled={currentPage >= pageCount} onPress={onNext}>
          下一页
        </Button>
      </div>
    </div>
  );
}

export default function PacksPageOptimized() {
  const navigationPrefs = useNavigationPreferences();
  const { data, loading, refresh } = useApiData(async () => packsClient.installed(), { packs: [], count: 0 });
  const { data: catalog, loading: catalogLoading, refresh: refreshCatalog } = useApiData(
    async () => packsClient.catalog(),
    { generated_at: "", sources: [], count: 0, installed: 0, enabled: 0, downloadable: 0, capabilities: 0, entries: [] },
  );
  const { data: releaseCatalog, loading: releaseLoading, refresh: refreshReleaseCatalog } = useApiData(
    async () => packsClient.releaseCatalog(PACK_RELEASE_SOURCES.map((source) => source.url)),
    { generated_at: "", releases: PACK_RELEASE_SOURCES.map((source) => source.url), count: 0, entries: [] as PackReleaseCatalogEntry[] },
  );
  const [manifestPath, setManifestPath] = useState(OFFICIAL_BACKUP_MANIFEST);
  const [busy, setBusy] = useState<string | null>(null);
  const [showAdvanced, setShowAdvanced] = useState(false);
  const [query, setQuery] = useState("");
  const [kindFilter, setKindFilter] = useState<KindFilter>("all");
  const [installFilter, setInstallFilter] = useState<InstallFilter>("all");
  const [riskFilter, setRiskFilter] = useState<RiskFilter>("all");
  const [sourceFilter, setSourceFilter] = useState<SourceFilter>("all");
  const [readinessFilter, setReadinessFilter] = useState<ReadinessFilter>("all");
  const [sortMode, setSortMode] = useState<SortMode>("name");
  const [installedPage, setInstalledPage] = useState(1);
  const [releasePage, setReleasePage] = useState(1);
  const [privatePage, setPrivatePage] = useState(1);

  const packs = data?.packs || [];
  const catalogEntries = catalog?.entries || [];
  const privateCatalogEntries = catalogEntries.filter((entry) => catalogActionForEntry(entry).kind !== "use");
  const releaseEntries = releaseCatalog.entries || [];
  const stats = useMemo(() => ({
    available: releaseEntries.filter((entry) => catalogActionForEntry(entry).kind !== "use").length + privateCatalogEntries.length,
    installed: packs.length,
    enabled: packs.filter((p) => p.status === "enabled").length,
  }), [packs, privateCatalogEntries.length, releaseEntries]);
  const packKindStats = useMemo(() => {
    const manifests = new Map<string, PackManifest>();
    for (const pack of packs) manifests.set(pack.manifest.id, pack.manifest);
    for (const entry of releaseEntries) manifests.set(entry.manifest.id, entry.manifest);
    for (const entry of catalogEntries) manifests.set(entry.manifest.id, entry.manifest);
    const counts = { actionable: 0, infrastructure: 0, experimental: 0, documented: 0 };
    for (const manifest of manifests.values()) counts[packUsability(manifest).kind] += 1;
    return counts;
  }, [packs, releaseEntries, catalogEntries]);
  const normalizedQuery = query.trim().toLowerCase();
  const matchesFilters = (manifest: PackManifest, options?: { installedStatus?: string; source: SourceFilter }) => {
    const usability = packUsability(manifest);
    const risk = riskProfileForPack(manifest);
    const readiness = packReadiness(manifest);
    if (normalizedQuery && !packSearchText(manifest).includes(normalizedQuery)) return false;
    if (kindFilter !== "all") {
      if (kindFilter === "infrastructure") {
        if (usability.kind !== "infrastructure" && usability.kind !== "documented") return false;
      } else if (usability.kind !== kindFilter) {
        return false;
      }
    }
    if (riskFilter !== "all" && risk.level !== riskFilter) return false;
    if (sourceFilter !== "all" && options?.source !== sourceFilter) return false;
    if (readinessFilter !== "all" && readiness.level !== readinessFilter) return false;
    if (installFilter === "installed" && !options?.installedStatus) return false;
    if (installFilter === "enabled" && options?.installedStatus !== "enabled") return false;
    if (installFilter === "disabled" && options?.installedStatus !== "disabled") return false;
    if (installFilter === "available" && options?.installedStatus) return false;
    return true;
  };
  const filteredInstalledPacks = useMemo(
    () => sortPacks(
      packs.filter((pack) => matchesFilters(pack.manifest, { installedStatus: pack.status, source: "installed" })),
      (pack) => pack.manifest,
      sortMode,
    ),
    [packs, normalizedQuery, kindFilter, installFilter, riskFilter, sourceFilter, readinessFilter, sortMode],
  );
  const filteredReleaseEntries = useMemo(
    () => sortPacks(
      releaseEntries.filter((entry) => matchesFilters(entry.manifest, { source: "official" })),
      (entry) => entry.manifest,
      sortMode,
    ),
    [releaseEntries, normalizedQuery, kindFilter, installFilter, riskFilter, sourceFilter, readinessFilter, sortMode],
  );
  const filteredPrivateCatalogEntries = useMemo(
    () => sortPacks(
      privateCatalogEntries.filter((entry) => matchesFilters(entry.manifest, { source: "private" })),
      (entry) => entry.manifest,
      sortMode,
    ),
    [privateCatalogEntries, normalizedQuery, kindFilter, installFilter, riskFilter, sourceFilter, readinessFilter, sortMode],
  );
  const totalMatches = filteredInstalledPacks.length + filteredReleaseEntries.length + filteredPrivateCatalogEntries.length;
  const installedPageCount = pageCountFor(filteredInstalledPacks.length);
  const currentInstalledPage = Math.min(installedPage, installedPageCount);
  const pagedInstalledPacks = paginate(filteredInstalledPacks, currentInstalledPage);
  const releasePageCount = pageCountFor(filteredReleaseEntries.length);
  const currentReleasePage = Math.min(releasePage, releasePageCount);
  const pagedReleaseEntries = paginate(filteredReleaseEntries, currentReleasePage);
  const privatePageCount = pageCountFor(filteredPrivateCatalogEntries.length);
  const currentPrivatePage = Math.min(privatePage, privatePageCount);
  const pagedPrivateCatalogEntries = paginate(filteredPrivateCatalogEntries, currentPrivatePage);

  useEffect(() => {
    setInstalledPage(1);
    setReleasePage(1);
    setPrivatePage(1);
  }, [normalizedQuery, kindFilter, installFilter, riskFilter, sourceFilter, readinessFilter, sortMode]);

  const refreshAll = async () => {
    await Promise.all([refresh(), refreshCatalog(), refreshReleaseCatalog()]);
  };

  const run = async (label: string, op: () => Promise<unknown>, successMsg = "操作成功") => {
    setBusy(label);
    try {
      await op();
      showToast(successMsg, "success");
      await refreshAll();
      window.dispatchEvent(new CustomEvent("yunque:packs-changed"));
    } catch (e) {
      showToast(label.startsWith("install") ? formatPackInstallError(e) : formatPackInstallError(e, "操作失败"), "error");
    } finally {
      setBusy(null);
    }
  };

  const installLocal = () => run("install:local", () => packsClient.install({ manifestPath, download: false }), "已安装，可在详情页启用");
  const installRelease = (entry: PackReleaseCatalogEntry) => {
    const action = catalogActionForEntry(entry);
    if (action.kind === "enable") return enable(entry.manifest.id);
    const request = entryInstallRequest({ ...entry, source: entry.release_url });
    if (!request) {
      showToast("此能力包没有可用的安装源", "error");
      return;
    }
    return run(`install:${entry.package_url}`, () => packsClient.install(request), "已安装，可继续启用或打开详情");
  };
  const installCatalogEntry = (entry: PackCatalogEntry) => {
    const action = catalogActionForEntry(entry);
    if (action.kind === "enable") return enable(entry.manifest.id);
    const request = entryInstallRequest(entry);
    if (!request) {
      showToast("此能力包没有可用的安装源", "error");
      return;
    }
    return run(`install:${entry.manifest.id}`, () => packsClient.install(request), "已安装，可继续启用或打开详情");
  };
  const enable = (id: string) => run(`enable:${id}`, () => packsClient.enable(id), "已启用，可在命令菜单、扩展分组或本页入口打开");
  const disable = (id: string) => run(`disable:${id}`, () => packsClient.disable(id), "已禁用");
  const rollback = (id: string) => run(`rollback:${id}`, () => packsClient.rollback(id), "已回滚");

  const navItemsForPack = (pack: InstalledPack) => buildPackNavItems([pack]);
  const isPackPinned = (pack: InstalledPack) => {
    const navItems = navItemsForPack(pack);
    return navItems.length > 0 && navItems.every((item) => navigationPrefs.pinnedItems.includes(`pack-${item.packId}-${item.href}`));
  };
  const togglePackPinned = (pack: InstalledPack) => {
    const navItems = navItemsForPack(pack);
    if (navItems.length === 0) {
      showToast("这个能力包没有可固定的页面入口", "warning");
      return;
    }
    const pinned = isPackPinned(pack);
    navItems.forEach((item) => {
      const id = `pack-${item.packId}-${item.href}`;
      if (pinned) navigationPrefs.unpinItem(id);
      else navigationPrefs.pinItem(id);
    });
    showToast(pinned ? "已从侧边栏移除" : "已固定到侧边栏", "success");
  };
  const activeFilters = [
    ...(query.trim() ? [{
      key: "query",
      label: `搜索：${query.trim()}`,
      clearLabel: "清除搜索",
      clear: () => setQuery(""),
    }] : []),
    ...(kindFilter !== "all" ? [{
      key: "kind",
      label: `类型：${KIND_FILTER_LABELS[kindFilter]}`,
      clearLabel: "清除类型",
      clear: () => setKindFilter("all"),
    }] : []),
    ...(installFilter !== "all" ? [{
      key: "install",
      label: `状态：${INSTALL_FILTER_LABELS[installFilter]}`,
      clearLabel: "清除状态",
      clear: () => setInstallFilter("all"),
    }] : []),
    ...(riskFilter !== "all" ? [{
      key: "risk",
      label: `风险：${RISK_FILTER_LABELS[riskFilter]}`,
      clearLabel: "清除风险",
      clear: () => setRiskFilter("all"),
    }] : []),
    ...(sourceFilter !== "all" ? [{
      key: "source",
      label: `来源：${SOURCE_FILTER_LABELS[sourceFilter]}`,
      clearLabel: "清除来源",
      clear: () => setSourceFilter("all"),
    }] : []),
    ...(readinessFilter !== "all" ? [{
      key: "readiness",
      label: `体检：${READINESS_FILTER_LABELS[readinessFilter]}`,
      clearLabel: "清除体检",
      clear: () => setReadinessFilter("all"),
    }] : []),
    ...(sortMode !== "name" ? [{
      key: "sort",
      label: `排序：${SORT_MODE_LABELS[sortMode]}`,
      clearLabel: "恢复默认排序",
      clear: () => setSortMode("name"),
    }] : []),
  ];

  if (loading) {
    return <div className="flex items-center justify-center h-[60vh]"><Spinner size="lg" /></div>;
  }

  return (
    <div className="flex flex-col h-screen overflow-hidden">
      <div className="flex-shrink-0 p-5 border-b" style={{ borderColor: "var(--yunque-border)" }}>
        <PageHeader
          icon={<Boxes size={20} />}
          title="能力包中心"
          description="需要什么取什么，安装、启用、查看权限和打开入口都在这里完成"
          onRefresh={refreshAll}
        />

        <div className="grid grid-cols-3 gap-3 mt-4">
          <Card className="section-card p-4">
            <div className="kpi-label">可安装</div>
            <div className="kpi-value">{releaseLoading || catalogLoading ? "…" : stats.available}</div>
          </Card>
          <Card className="section-card p-4">
            <div className="kpi-label">已安装</div>
            <div className="kpi-value">{stats.installed}</div>
          </Card>
          <Card className="section-card p-4">
            <div className="kpi-label">已启用</div>
            <div className="kpi-value">{stats.enabled}</div>
          </Card>
        </div>

        <div className="mt-4 rounded-lg border p-4" style={{ borderColor: "var(--yunque-border)", background: "var(--yunque-bg-hover)" }}>
          <div className="mb-3 flex items-start justify-between gap-3">
            <div>
              <div className="text-sm font-semibold" style={{ color: "var(--yunque-text)" }}>能力包不是都要单独打开</div>
              <div className="mt-1 text-xs leading-5" style={{ color: "var(--yunque-text-muted)" }}>
                云雀会把能力包分成三类：能直接操作的入口、支撑 Chat/任务/知识的基础能力，以及仍在验证的实验能力。
              </div>
            </div>
            <Chip size="sm" variant="soft">按当前来源统计</Chip>
          </div>
          <div className="grid gap-3 md:grid-cols-3">
            <div className="rounded-md p-3" style={{ background: "rgba(34,197,94,0.08)", border: "1px solid rgba(34,197,94,0.18)" }}>
              <div className="flex items-center justify-between gap-2">
                <span className="text-sm font-medium" style={{ color: "var(--yunque-text)" }}>可直接使用</span>
                <span className="text-lg font-semibold" style={{ color: "var(--yunque-success)" }}>{packKindStats.actionable}</span>
              </div>
              <div className="mt-2 text-xs leading-5" style={{ color: "var(--yunque-text-muted)" }}>
                有明确页面或主入口，适合用户查看、编辑、执行或继续处理。
              </div>
            </div>
            <div className="rounded-md p-3" style={{ background: "rgba(59,130,246,0.08)", border: "1px solid rgba(59,130,246,0.18)" }}>
              <div className="flex items-center justify-between gap-2">
                <span className="text-sm font-medium" style={{ color: "var(--yunque-text)" }}>基础能力</span>
                <span className="text-lg font-semibold" style={{ color: "var(--yunque-primary)" }}>{packKindStats.infrastructure + packKindStats.documented}</span>
              </div>
              <div className="mt-2 text-xs leading-5" style={{ color: "var(--yunque-text-muted)" }}>
                通常不单独当应用打开，而是在 Chat、任务、记忆、知识或设置页里生效。
              </div>
            </div>
            <div className="rounded-md p-3" style={{ background: "rgba(245,158,11,0.08)", border: "1px solid rgba(245,158,11,0.20)" }}>
              <div className="flex items-center justify-between gap-2">
                <span className="text-sm font-medium" style={{ color: "var(--yunque-text)" }}>实验中</span>
                <span className="text-lg font-semibold" style={{ color: "var(--yunque-warning)" }}>{packKindStats.experimental}</span>
              </div>
              <div className="mt-2 text-xs leading-5" style={{ color: "var(--yunque-text-muted)" }}>
                可体验但不作为稳定主路径；启用前先看限制、权限和风险说明。
              </div>
            </div>
          </div>
        </div>

        <div className="flex items-center gap-3 mt-4">
          <Link href="/packs/studio">
            <Button size="sm" variant="outline">
              <Wrench size={14} /> Pack Studio
            </Button>
          </Link>
          <Button size="sm" variant="ghost" onPress={() => setShowAdvanced(!showAdvanced)}>
            {showAdvanced ? <ChevronUp size={14} /> : <ChevronDown size={14} />}
            {showAdvanced ? "隐藏" : "显示"}技术详情
          </Button>
        </div>

        <Card className="mt-4 p-4" style={{ background: "var(--yunque-surface)", border: "1px solid var(--yunque-border)" }}>
          <div className="mb-3 flex flex-wrap items-center justify-between gap-3">
            <div className="flex items-center gap-2 text-sm font-semibold" style={{ color: "var(--yunque-text)" }}>
              <SlidersHorizontal size={15} style={{ color: "var(--yunque-accent)" }} />
              商店筛选
            </div>
            <div className="text-xs" style={{ color: "var(--yunque-text-muted)" }}>
              匹配 {totalMatches} 个 · 已安装 {filteredInstalledPacks.length} · 官方源 {filteredReleaseEntries.length} · 私有源 {filteredPrivateCatalogEntries.length}
            </div>
          </div>
          <div className="grid gap-3 lg:grid-cols-[minmax(220px,1fr)_auto]">
            <TextField value={query} onChange={setQuery}>
              <Label>搜索能力包</Label>
              <Input placeholder="搜索名称、用途、权限、能力、入口" />
            </TextField>
            <Button
              variant="outline"
              className="self-end"
              onPress={() => {
                setQuery("");
                setKindFilter("all");
                setInstallFilter("all");
                setRiskFilter("all");
                setSourceFilter("all");
                setReadinessFilter("all");
                setSortMode("name");
              }}
            >
              <Search size={14} /> 重置
            </Button>
          </div>
          <div className="mt-3 grid gap-3 xl:grid-cols-6">
            {renderFilterGroup("类型", [
              ["all", "全部"],
              ["actionable", "可用"],
              ["infrastructure", "基础"],
              ["experimental", "实验"],
            ], kindFilter, (value) => setKindFilter(value as KindFilter))}
            {renderFilterGroup("状态", [
              ["all", "全部"],
              ["installed", "已安装"],
              ["enabled", "已启用"],
              ["disabled", "已禁用"],
              ["available", "可安装"],
            ], installFilter, (value) => setInstallFilter(value as InstallFilter))}
            {renderFilterGroup("风险", [
              ["all", "全部"],
              ["low", "低"],
              ["medium", "留意"],
              ["high", "授权"],
            ], riskFilter, (value) => setRiskFilter(value as RiskFilter))}
            {renderFilterGroup("来源", [
              ["all", "全部"],
              ["installed", "已安装"],
              ["official", "官方"],
              ["private", "私有"],
            ], sourceFilter, (value) => setSourceFilter(value as SourceFilter))}
            {renderFilterGroup("体检", [
              ["all", "全部"],
              ["complete", "完整"],
              ["needs_context", "补说明"],
              ["needs_entry", "补入口"],
            ], readinessFilter, (value) => setReadinessFilter(value as ReadinessFilter))}
            {renderFilterGroup("排序", [
              ["name", "名称"],
              ["kind", "类型"],
              ["risk", "风险"],
              ["readiness", "体检"],
              ["status", "阶段"],
            ], sortMode, (value) => setSortMode(value as SortMode))}
          </div>
          <div className="mt-3 flex flex-wrap items-center gap-2">
            <span className="text-xs font-medium" style={{ color: "var(--yunque-text-muted)" }}>当前条件</span>
            {activeFilters.length === 0 ? (
              <Chip size="sm" variant="soft">未启用筛选</Chip>
            ) : activeFilters.map((filter) => (
              <Button key={filter.key} size="sm" variant="ghost" onPress={filter.clear} aria-label={filter.clearLabel}>
                <span className="text-xs">{filter.label}</span>
                <X size={13} />
              </Button>
            ))}
          </div>
        </Card>
      </div>

      <div className="flex-1 overflow-y-auto p-5 space-y-4">
        <section className="space-y-3">
          <div className="flex items-center justify-between gap-3 mb-3">
            <div className="flex items-start gap-2">
              <Store size={17} style={{ color: "var(--yunque-accent)" }} />
              <div>
                <div className="text-sm font-semibold" style={{ color: "var(--yunque-text)" }}>官方源</div>
                <div className="text-xs mt-1" style={{ color: "var(--yunque-text-muted)" }}>
                  从可信发布读取可安装能力包，安装前先看用途、入口和权限。
                </div>
              </div>
            </div>
            <Button size="sm" variant="ghost" onPress={refreshReleaseCatalog} isDisabled={releaseLoading}>
              {releaseLoading ? <Spinner size="sm" /> : <RotateCcw size={14} />}
              刷新
            </Button>
          </div>

          <div className="grid grid-cols-1 md:grid-cols-2 gap-3 mb-4">
            {PACK_RELEASE_SOURCES.map((source) => (
              <div key={source.url} className="rounded-md p-3 border" style={{ borderColor: "var(--yunque-border)" }}>
                <div className="flex items-center gap-2 text-sm" style={{ color: "var(--yunque-text)" }}>
                  <Globe2 size={14} style={{ color: "var(--yunque-primary)" }} />
                  <span className="font-medium">{source.label}</span>
                </div>
                <div className="text-xs mt-1" style={{ color: "var(--yunque-text-muted)" }}>{source.note}</div>
                <div className="text-[11px] font-mono break-all mt-2" style={{ color: "var(--yunque-text-muted)" }}>
                  {sourceName(source.url)}
                </div>
              </div>
            ))}
          </div>

          {releaseLoading ? (
            <div className="flex items-center gap-2 text-xs py-6" style={{ color: "var(--yunque-text-muted)" }}>
              <Spinner size="sm" /> 正在读取发布内容
            </div>
          ) : filteredReleaseEntries.length > 0 ? (
            <div className="space-y-3">
              <div className="grid grid-cols-1 lg:grid-cols-2 gap-4">
                {pagedReleaseEntries.map((entry) => renderInstallableCard(entry.manifest, {
                  key: entry.package_url,
                  source: entry.release_tag || sourceName(entry.release_url),
                  size: formatBytes(entry.size_bytes),
                  action: catalogActionForEntry(entry),
                  busyKey: `install:${entry.package_url}`,
                  onInstall: () => installRelease(entry),
                  packageUrl: typeof entry.package_url === "string" ? entry.package_url : undefined,
                  sha256: typeof entry.sha256 === "string" ? entry.sha256 : undefined,
                }))}
              </div>
              {renderPagination(
                "官方源",
                currentReleasePage,
                releasePageCount,
                filteredReleaseEntries.length,
                () => setReleasePage((value) => Math.max(1, value - 1)),
                () => setReleasePage((value) => Math.min(releasePageCount, value + 1)),
              )}
            </div>
          ) : (
            <div className="text-xs py-4" style={{ color: "var(--yunque-text-muted)" }}>
              {releaseEntries.length > 0 ? "没有符合筛选条件的官方源能力包。" : "暂时没有读到可安装的 .yqpack 发布包。"}
            </div>
          )}
        </section>

        <section className="space-y-3">
          <div className="flex items-center justify-between gap-3 mb-3">
            <div className="flex items-start gap-2">
              <LockKeyhole size={17} style={{ color: "var(--yunque-warning)" }} />
              <div>
                <div className="text-sm font-semibold" style={{ color: "var(--yunque-text)" }}>私有源</div>
                <div className="text-xs mt-1" style={{ color: "var(--yunque-text-muted)" }}>
                  读取后端配置的 catalog 源，可接入私有 OSS、团队源或本地目录。
                </div>
              </div>
            </div>
            <Button size="sm" variant="ghost" onPress={refreshCatalog} isDisabled={catalogLoading}>
              {catalogLoading ? <Spinner size="sm" /> : <RotateCcw size={14} />}
              刷新
            </Button>
          </div>

          {(catalog.source_reports?.length ?? 0) > 0 && (
            <div className="grid grid-cols-1 md:grid-cols-2 gap-3 mb-4">
              {catalog.source_reports?.map((report) => (
                <div key={report.source} className="rounded-md p-3 border" style={{ borderColor: "var(--yunque-border)" }}>
                  <div className="flex items-center gap-2 text-sm" style={{ color: report.ok ? "var(--yunque-success)" : "var(--yunque-warning)" }}>
                    {report.ok ? <ShieldCheck size={14} /> : <ShieldAlert size={14} />}
                    <span className="font-medium">{report.ok ? "源可用" : "源需要处理"}</span>
                  </div>
                  <div className="text-[11px] font-mono break-all mt-1" style={{ color: "var(--yunque-text-muted)" }}>{report.source}</div>
                  <div className="text-xs mt-2" style={{ color: "var(--yunque-text-muted)" }}>
                    manifest {report.manifest_count} 个，匹配 {report.matched_entries} 个
                  </div>
                  {(report.errors?.length ?? 0) > 0 && (
                    <div className="text-xs mt-2" style={{ color: "var(--yunque-warning)" }}>
                      {report.errors?.[0]}
                    </div>
                  )}
                </div>
              ))}
            </div>
          )}

          {catalogLoading ? (
            <div className="flex items-center gap-2 text-xs py-6" style={{ color: "var(--yunque-text-muted)" }}>
              <Spinner size="sm" /> 正在读取私有源
            </div>
          ) : filteredPrivateCatalogEntries.length > 0 ? (
            <div className="space-y-3">
              <div className="grid grid-cols-1 lg:grid-cols-2 gap-4">
                {pagedPrivateCatalogEntries.map((entry) => renderInstallableCard(entry.manifest, {
                  key: entry.manifest.id,
                  source: [entry.source, entry.manifest_path, entry.manifest_url, entry.package_url].find((value): value is string => typeof value === "string"),
                  action: catalogActionForEntry(entry),
                  busyKey: `install:${entry.manifest.id}`,
                  onInstall: () => installCatalogEntry(entry),
                  packageUrl: typeof entry.package_url === "string" ? entry.package_url : undefined,
                  sha256: typeof entry.sha256 === "string" ? entry.sha256 : undefined,
                }))}
              </div>
              {renderPagination(
                "私有源",
                currentPrivatePage,
                privatePageCount,
                filteredPrivateCatalogEntries.length,
                () => setPrivatePage((value) => Math.max(1, value - 1)),
                () => setPrivatePage((value) => Math.min(privatePageCount, value + 1)),
              )}
            </div>
          ) : (
            <div className="text-xs py-4" style={{ color: "var(--yunque-text-muted)" }}>
              {privateCatalogEntries.length > 0 ? "没有符合筛选条件的私有源能力包。" : "当前没有来自私有源的待安装能力包。可以在后端 catalog 配置中接入 OSS 或团队源。"}
            </div>
          )}
        </section>

        <Card className="section-card p-4">
          <button
            type="button"
            onClick={() => setShowAdvanced(!showAdvanced)}
            className="flex items-center justify-between w-full"
          >
            <div className="flex items-start gap-2 text-left">
              <FolderInput size={17} style={{ color: "var(--yunque-text-muted)" }} />
              <div>
                <div className="text-sm font-semibold" style={{ color: "var(--yunque-text)" }}>本地高级安装</div>
                <div className="text-xs mt-1" style={{ color: "var(--yunque-text-muted)" }}>
                  适合开发者从本地 pack.json 安装。普通用户优先使用上方来源。
                </div>
              </div>
            </div>
            {showAdvanced ? <ChevronUp size={14} /> : <ChevronDown size={14} />}
          </button>
          {showAdvanced && (
            <div className="mt-4 pt-4 border-t" style={{ borderColor: "var(--yunque-border)" }}>
              <div className="flex gap-3">
                <TextField value={manifestPath} onChange={(v: string) => setManifestPath(v)} className="flex-1">
                  <Label>pack.json 路径</Label>
                  <Input placeholder={OFFICIAL_BACKUP_MANIFEST} />
                </TextField>
                <Button className="btn-accent self-end" isDisabled={!manifestPath || busy === "install:local"} onPress={installLocal}>
                  <Download size={14} /> 安装
                </Button>
              </div>
            </div>
          )}
        </Card>

        {packs.length === 0 ? (
          <Card className="section-card p-12 text-center">
            <Boxes size={40} className="mx-auto mb-3" style={{ color: "var(--yunque-text-muted)" }} />
            <div className="text-sm font-medium" style={{ color: "var(--yunque-text)" }}>还没有安装能力包</div>
            <div className="text-xs mt-1" style={{ color: "var(--yunque-text-muted)" }}>
              先从官方源或私有源选择一个能力包
            </div>
          </Card>
        ) : filteredInstalledPacks.length === 0 ? (
          <Card className="section-card p-8 text-center">
            <Boxes size={32} className="mx-auto mb-3" style={{ color: "var(--yunque-text-muted)" }} />
            <div className="text-sm font-medium" style={{ color: "var(--yunque-text)" }}>没有符合筛选条件的已安装能力包</div>
            <div className="text-xs mt-1" style={{ color: "var(--yunque-text-muted)" }}>
              可以清空搜索，或切换类型、状态、风险和来源筛选。
            </div>
          </Card>
        ) : (
          <div className="space-y-6">
            {renderInstalledSection("已安装能力包", pagedInstalledPacks)}
            {renderPagination(
              "已安装",
              currentInstalledPage,
              installedPageCount,
              filteredInstalledPacks.length,
              () => setInstalledPage((value) => Math.max(1, value - 1)),
              () => setInstalledPage((value) => Math.min(installedPageCount, value + 1)),
            )}
          </div>
        )}
      </div>
    </div>
  );

  function renderInstalledSection(title: string, sectionPacks: InstalledPack[], note?: string) {
    if (sectionPacks.length === 0) return null;
    return (
      <div>
        <div className="flex items-center gap-2 mb-3">
          <h3 className="text-sm font-semibold" style={{ color: "var(--yunque-text)" }}>{title} · {sectionPacks.length} 个</h3>
          {note && <Chip size="sm" style={{ background: "rgba(245,158,11,0.12)", color: "var(--yunque-warning)" }}>{note}</Chip>}
        </div>
        <div className="grid grid-cols-1 lg:grid-cols-2 gap-4">
          {sectionPacks.map((pack) => renderPackCard(pack))}
        </div>
      </div>
    );
  }

  function renderInstallableCard(
    manifest: PackManifest,
    options: { key: string; source?: string; size?: string; action: ReturnType<typeof catalogActionForEntry>; busyKey: string; onInstall: () => void; packageUrl?: string; sha256?: string },
  ) {
    const badge = packStatusBadge(manifest.status);
    const risk = riskProfileForPack(manifest);
    const examples = packExamples(manifest);
    const permissionGroups = groupPackPermissions(manifest.backend?.permissions || []);
    const labels = capabilitySurfaceLabels(manifest);
    const usability = packUsability(manifest);
    const readiness = packReadiness(manifest);
    const usageLines = packUsageExplanation(manifest).slice(0, 3);
    const actionBusyKey = options.action.kind === "enable" ? `enable:${manifest.id}` : options.busyKey;
    const disabled = options.action.disabled || busy === actionBusyKey;
    const primaryPath = usability.primaryActionPath || manifest.frontend?.menus?.[0]?.path || manifest.frontend?.routes?.[0]?.path;

    return (
      <Card key={options.key} className="section-card p-4 hover-lift">
        <div className="flex items-start justify-between gap-3 mb-3">
          <div className="min-w-0 flex-1">
            <div className="flex items-center gap-2 flex-wrap">
              <PackageCheck size={16} style={{ color: "var(--yunque-accent)" }} />
              <span className="font-semibold text-sm" style={{ color: "var(--yunque-text)" }}>{manifest.name}</span>
              <Chip size="sm" style={{ background: badge.bg, color: badge.color }}>{badge.label}</Chip>
              <Chip size="sm" style={{ background: "rgba(59,130,246,0.08)", color: "var(--yunque-primary)" }}>{usability.label}</Chip>
              <Chip size="sm" style={{
                background: risk.level === "high" ? "rgba(239,68,68,0.12)" : risk.level === "medium" ? "rgba(245,158,11,0.12)" : "rgba(34,197,94,0.10)",
                color: risk.level === "high" ? "var(--yunque-danger)" : risk.level === "medium" ? "var(--yunque-warning)" : "var(--yunque-success)",
              }}>
                {risk.label}
              </Chip>
              <Chip size="sm" style={{
                background: readiness.level === "complete" ? "rgba(34,197,94,0.10)" : readiness.level === "needs_context" ? "rgba(245,158,11,0.12)" : "rgba(239,68,68,0.10)",
                color: readiness.level === "complete" ? "var(--yunque-success)" : readiness.level === "needs_context" ? "var(--yunque-warning)" : "var(--yunque-danger)",
              }}>
                {readiness.label}
              </Chip>
            </div>
            {manifest.description && (
              <div className="text-xs mt-2" style={{ color: "var(--yunque-text-secondary)" }}>{manifest.description}</div>
            )}
          </div>
          <Button
            size="sm"
            className={options.action.kind === "install" || options.action.kind === "update" || options.action.kind === "enable" ? "btn-accent" : undefined}
            variant={options.action.kind === "use" ? "outline" : undefined}
            isDisabled={disabled}
            onPress={options.onInstall}
          >
            {options.action.kind === "enable" ? <Power size={14} /> : <Download size={14} />} {options.action.label}
          </Button>
        </div>

        {examples.length > 0 && (
          <div className="mb-3 space-y-1">
            {examples.map((example) => (
              <div key={example} className="flex items-start gap-2 text-xs" style={{ color: "var(--yunque-text-muted)" }}>
                <span style={{ color: "var(--yunque-accent)" }}>•</span>
                <span>{example}</span>
              </div>
            ))}
          </div>
        )}

        <div className="flex flex-wrap gap-1.5 mb-3">
          {labels.map((label) => (
            <Chip key={label} size="sm" style={{ background: "rgba(59,130,246,0.08)", color: "var(--yunque-primary)" }}>{label}</Chip>
          ))}
          {primaryPath && (
            <Chip size="sm" style={{ background: "rgba(255,255,255,0.05)", color: "var(--yunque-text-muted)" }}>
              入口 {primaryPath}
            </Chip>
          )}
        </div>

        <div className="text-xs mb-3" style={{ color: "var(--yunque-text-muted)" }}>
          {usability.description}
          {usability.limitation ? ` 当前限制：${usability.limitation}` : ""}
        </div>

        {usageLines.length > 0 && (
          <div className="mb-3 rounded-md p-3" style={{ background: "var(--yunque-bg-hover)", border: "1px solid var(--yunque-border)" }}>
            <div className="mb-2 text-xs font-medium" style={{ color: "var(--yunque-text)" }}>怎么用它</div>
            <div className="space-y-1">
              {usageLines.map((line) => (
                <div key={line} className="flex items-start gap-2 text-xs leading-5" style={{ color: "var(--yunque-text-secondary)" }}>
                  <span style={{ color: "var(--yunque-accent)" }}>•</span>
                  <span>{line}</span>
                </div>
              ))}
            </div>
          </div>
        )}

        {readiness.missing.length > 0 && (
          <div className="mb-3 rounded-md p-3 text-xs" style={{ background: "rgba(245,158,11,0.08)", border: "1px solid rgba(245,158,11,0.18)", color: "var(--yunque-text-secondary)" }}>
            可用性体检：还缺 {readiness.missing.join("、")}。可以交给小羽优化补齐用途、入口或使用说明。
          </div>
        )}

        {permissionGroups.length > 0 && (
          <div className="flex flex-wrap gap-1.5 mb-3">
            {permissionGroups.slice(0, 4).map((group) => (
              <Chip key={group.key} size="sm" style={{ background: "rgba(245,158,11,0.10)", color: "var(--yunque-warning)" }}>
                {group.label}
              </Chip>
            ))}
            {permissionGroups.length > 4 && <Chip size="sm" variant="soft">+{permissionGroups.length - 4}</Chip>}
          </div>
        )}

        <div className="text-xs" style={{ color: "var(--yunque-text-muted)" }}>
          v{manifest.version}
          {options.size ? ` · ${options.size}` : ""}
          {options.source ? ` · ${options.source}` : ""}
        </div>
        <div className="flex items-center gap-2 mt-3">
          <Link href={`/packs/detail?id=${encodeURIComponent(manifest.id)}`}>
            <Button size="sm" variant="ghost">查看详情 <ArrowRight size={14} /></Button>
          </Link>
          <Link href={packStudioHref(manifest, { packageUrl: options.packageUrl, sha256: options.sha256 })}>
            <Button size="sm" variant="ghost">小羽优化 <Wrench size={14} /></Button>
          </Link>
          {primaryPath && (
            <Link href={primaryPath}>
              <Button size="sm" variant="ghost">{usability.primaryActionLabel || "打开入口"} <ExternalLink size={14} /></Button>
            </Link>
          )}
          {risk.requiresAuthorization && (
            <span className="text-xs" style={{ color: "var(--yunque-warning)" }}>启用前建议确认授权</span>
          )}
        </div>
      </Card>
    );
  }

  function renderPackCard(pack: InstalledPack) {
    const tone = statusTone(pack.status);
    const badge = packStatusBadge(pack.manifest.status);
    const manifest = pack.manifest;
    const risk = riskProfileForPack(manifest);
    const examples = packExamples(manifest);
    const labels = capabilitySurfaceLabels(manifest);
    const permissionGroups = groupPackPermissions(manifest.backend?.permissions || []);
    const usability = packUsability(manifest);
    const readiness = packReadiness(manifest);
    const usageLines = packUsageExplanation(manifest).slice(0, 3);
    const navItems = navItemsForPack(pack);
    const openPath = usability.primaryActionPath || manifest.frontend?.menus?.[0]?.path || manifest.frontend?.routes?.[0]?.path;

    return (
      <Card key={manifest.id} className="section-card p-4 hover-lift">
        <Link href={`/packs/detail?id=${encodeURIComponent(manifest.id)}`} className="block cursor-pointer">
          <div className="flex items-start justify-between gap-3 mb-3">
            <div className="min-w-0 flex-1">
              <div className="flex items-center gap-2 flex-wrap">
                <PackageCheck size={16} style={{ color: "var(--yunque-accent)" }} />
                <span className="font-semibold text-sm" style={{ color: "var(--yunque-text)" }}>{manifest.name}</span>
                <Chip size="sm" style={{ background: tone.bg, color: tone.color }}>{tone.label}</Chip>
                <Chip size="sm" style={{ background: badge.bg, color: badge.color }}>{badge.label}</Chip>
                <Chip size="sm" style={{ background: "rgba(59,130,246,0.08)", color: "var(--yunque-primary)" }}>{usability.label}</Chip>
                <Chip size="sm" style={{
                  background: risk.level === "high" ? "rgba(239,68,68,0.12)" : risk.level === "medium" ? "rgba(245,158,11,0.12)" : "rgba(34,197,94,0.10)",
                  color: risk.level === "high" ? "var(--yunque-danger)" : risk.level === "medium" ? "var(--yunque-warning)" : "var(--yunque-success)",
                }}>
                  {risk.label}
                </Chip>
                <Chip size="sm" style={{
                  background: readiness.level === "complete" ? "rgba(34,197,94,0.10)" : readiness.level === "needs_context" ? "rgba(245,158,11,0.12)" : "rgba(239,68,68,0.10)",
                  color: readiness.level === "complete" ? "var(--yunque-success)" : readiness.level === "needs_context" ? "var(--yunque-warning)" : "var(--yunque-danger)",
                }}>
                  {readiness.label}
                </Chip>
              </div>
              <div className="text-xs mt-1 font-mono" style={{ color: "var(--yunque-text-muted)" }}>{manifest.id}</div>
              {manifest.description && (
                <div className="text-xs mt-2" style={{ color: "var(--yunque-text-secondary)" }}>{manifest.description}</div>
              )}
            </div>
          </div>

          {examples.length > 0 && (
            <div className="mb-3 space-y-1">
              {examples.map((example) => (
                <div key={example} className="flex items-start gap-2 text-xs" style={{ color: "var(--yunque-text-muted)" }}>
                  <span style={{ color: "var(--yunque-accent)" }}>•</span>
                  <span>{example}</span>
                </div>
              ))}
            </div>
          )}

          <div className="flex flex-wrap gap-1.5 mb-3">
            {labels.map((label) => (
              <Chip key={label} size="sm" style={{ background: "rgba(59,130,246,0.08)", color: "var(--yunque-primary)" }}>{label}</Chip>
            ))}
            {openPath && (
              <Chip size="sm" style={{ background: "rgba(255,255,255,0.05)", color: "var(--yunque-text-muted)" }}>
                入口 {openPath}
              </Chip>
            )}
          </div>

          <div className="text-xs mb-3" style={{ color: "var(--yunque-text-muted)" }}>
            {usability.description}
            {usability.limitation ? ` 当前限制：${usability.limitation}` : ""}
          </div>

          {usageLines.length > 0 && (
            <div className="mb-3 rounded-md p-3" style={{ background: "var(--yunque-bg-hover)", border: "1px solid var(--yunque-border)" }}>
              <div className="mb-2 text-xs font-medium" style={{ color: "var(--yunque-text)" }}>怎么用它</div>
              <div className="space-y-1">
                {usageLines.map((line) => (
                  <div key={line} className="flex items-start gap-2 text-xs leading-5" style={{ color: "var(--yunque-text-secondary)" }}>
                    <span style={{ color: "var(--yunque-accent)" }}>•</span>
                    <span>{line}</span>
                  </div>
                ))}
              </div>
            </div>
          )}

          {readiness.missing.length > 0 && (
            <div className="mb-3 rounded-md p-3 text-xs" style={{ background: "rgba(245,158,11,0.08)", border: "1px solid rgba(245,158,11,0.18)", color: "var(--yunque-text-secondary)" }}>
              可用性体检：还缺 {readiness.missing.join("、")}。可以交给小羽优化补齐用途、入口或使用说明。
            </div>
          )}

          {permissionGroups.length > 0 && (
            <div className="flex flex-wrap gap-1.5 mb-3">
              {permissionGroups.slice(0, 4).map((group) => (
                <Chip key={group.key} size="sm" style={{ background: "rgba(245,158,11,0.10)", color: "var(--yunque-warning)" }}>
                  {group.label}
                </Chip>
              ))}
              {permissionGroups.length > 4 && <Chip size="sm" variant="soft">+{permissionGroups.length - 4}</Chip>}
            </div>
          )}
        </Link>

        <div className="flex items-center gap-2 flex-wrap">
          {pack.status === "enabled" ? (
            <Button size="sm" variant="outline" isDisabled={busy === `disable:${manifest.id}`} onPress={() => disable(manifest.id)}>
              <PackageX size={14} /> 禁用
            </Button>
          ) : (
            <Button size="sm" className="btn-accent" isDisabled={busy === `enable:${manifest.id}`} onPress={() => enable(manifest.id)}>
              <Power size={14} /> 启用
            </Button>
          )}
          {pack.status === "enabled" && openPath && (
            <Link href={openPath}>
              <Button size="sm" variant="ghost">
                <ExternalLink size={14} /> {usability.primaryActionLabel || "打开"}
              </Button>
            </Link>
          )}
          {manifest.update?.rollback && pack.previousVersion && (
            <Button size="sm" variant="ghost" isDisabled={busy === `rollback:${manifest.id}`} onPress={() => rollback(manifest.id)}>
              <RotateCcw size={14} /> 回滚
            </Button>
          )}
          {pack.status === "enabled" && navItems.length > 0 && (
            <Button size="sm" variant="ghost" onPress={() => togglePackPinned(pack)}>
              {isPackPinned(pack) ? <PinOff size={14} /> : <Pin size={14} />}
              {isPackPinned(pack) ? "取消侧栏" : "固定侧栏"}
            </Button>
          )}
          <Link href={packStudioHref(manifest)}>
            <Button size="sm" variant="ghost">
              <Wrench size={14} /> 小羽优化
            </Button>
          </Link>
          <Link href={`/packs/detail?id=${encodeURIComponent(manifest.id)}`} className="ml-auto">
            <Button size="sm" variant="ghost">详情 <ArrowRight size={14} /></Button>
          </Link>
        </div>

        {showAdvanced && (
          <div className="mt-3 pt-3 border-t text-xs space-y-1" style={{ borderColor: "var(--yunque-border)", color: "var(--yunque-text-muted)" }}>
            <div>版本：v{manifest.version}</div>
            <div>更新时间：{formatTime(pack.updatedAt)}</div>
            {pack.previousVersion && <div>上一版本：{pack.previousVersion}</div>}
            {(manifest.backend?.capabilities?.length ?? 0) > 0 && <div>能力：{manifest.backend?.capabilities?.join(", ")}</div>}
          </div>
        )}
      </Card>
    );
  }
}
