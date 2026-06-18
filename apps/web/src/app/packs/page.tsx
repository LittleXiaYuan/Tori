"use client";

import { useMemo, useState } from "react";
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
  ShieldAlert,
  ShieldCheck,
  Store,
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
  const [showAlpha, setShowAlpha] = useState(false);

  const packs = data?.packs || [];
  const groupedPacks = useMemo(() => ({
    stable: packs.filter((p) => p.manifest.status === "stable"),
    beta: packs.filter((p) => p.manifest.status === "beta"),
    alpha: packs.filter((p) => p.manifest.status === "alpha"),
    other: packs.filter((p) => !["stable", "beta", "alpha"].includes(String(p.manifest.status || ""))),
  }), [packs]);
  const catalogEntries = catalog?.entries || [];
  const privateCatalogEntries = catalogEntries.filter((entry) => catalogActionForEntry(entry).kind !== "use");
  const stats = useMemo(() => ({
    available: (releaseCatalog.entries || []).filter((entry) => catalogActionForEntry(entry).kind !== "use").length + privateCatalogEntries.length,
    installed: packs.length,
    enabled: packs.filter((p) => p.status === "enabled").length,
  }), [packs, privateCatalogEntries.length, releaseCatalog.entries]);

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

        <div className="flex items-center gap-3 mt-4">
          <Button size="sm" variant="ghost" onPress={() => setShowAdvanced(!showAdvanced)}>
            {showAdvanced ? <ChevronUp size={14} /> : <ChevronDown size={14} />}
            {showAdvanced ? "隐藏" : "显示"}技术详情
          </Button>
          <Button size="sm" variant="ghost" onPress={() => setShowAlpha(!showAlpha)}>
            {showAlpha ? "隐藏" : "显示"}开发中能力
          </Button>
        </div>
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
          ) : releaseCatalog.entries.length > 0 ? (
            <div className="grid grid-cols-1 lg:grid-cols-2 gap-4">
              {releaseCatalog.entries.map((entry) => renderInstallableCard(entry.manifest, {
                key: entry.package_url,
                source: entry.release_tag || sourceName(entry.release_url),
                size: formatBytes(entry.size_bytes),
                action: catalogActionForEntry(entry),
                busyKey: `install:${entry.package_url}`,
                onInstall: () => installRelease(entry),
              }))}
            </div>
          ) : (
            <div className="text-xs py-4" style={{ color: "var(--yunque-text-muted)" }}>
              暂时没有读到可安装的 .yqpack 发布包。
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
          ) : privateCatalogEntries.length > 0 ? (
            <div className="grid grid-cols-1 lg:grid-cols-2 gap-4">
              {privateCatalogEntries.map((entry) => renderInstallableCard(entry.manifest, {
                key: entry.manifest.id,
                source: entry.source || entry.manifest_path || entry.manifest_url || entry.package_url,
                action: catalogActionForEntry(entry),
                busyKey: `install:${entry.manifest.id}`,
                onInstall: () => installCatalogEntry(entry),
              }))}
            </div>
          ) : (
            <div className="text-xs py-4" style={{ color: "var(--yunque-text-muted)" }}>
              当前没有来自私有源的待安装能力包。可以在后端 catalog 配置中接入 OSS 或团队源。
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
        ) : (
          <div className="space-y-6">
            {renderInstalledSection("已安装的正式版能力包", groupedPacks.stable)}
            {renderInstalledSection("已安装的测试版能力包", groupedPacks.beta)}
            {renderInstalledSection("已安装的其他能力包", groupedPacks.other)}
            {showAlpha && renderInstalledSection("已安装的开发中能力包", groupedPacks.alpha, "仅供测试")}
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
    options: { key: string; source?: string; size?: string; action: ReturnType<typeof catalogActionForEntry>; busyKey: string; onInstall: () => void },
  ) {
    const badge = packStatusBadge(manifest.status);
    const risk = riskProfileForPack(manifest);
    const examples = packExamples(manifest);
    const permissionGroups = groupPackPermissions(manifest.backend?.permissions || []);
    const labels = capabilitySurfaceLabels(manifest);
    const actionBusyKey = options.action.kind === "enable" ? `enable:${manifest.id}` : options.busyKey;
    const disabled = options.action.disabled || busy === actionBusyKey;
    const primaryEntry = manifest.frontend?.menus?.[0] || manifest.frontend?.routes?.[0];

    return (
      <Card key={options.key} className="section-card p-4 hover-lift">
        <div className="flex items-start justify-between gap-3 mb-3">
          <div className="min-w-0 flex-1">
            <div className="flex items-center gap-2 flex-wrap">
              <PackageCheck size={16} style={{ color: "var(--yunque-accent)" }} />
              <span className="font-semibold text-sm" style={{ color: "var(--yunque-text)" }}>{manifest.name}</span>
              <Chip size="sm" style={{ background: badge.bg, color: badge.color }}>{badge.label}</Chip>
              <Chip size="sm" style={{
                background: risk.level === "high" ? "rgba(239,68,68,0.12)" : risk.level === "medium" ? "rgba(245,158,11,0.12)" : "rgba(34,197,94,0.10)",
                color: risk.level === "high" ? "var(--yunque-danger)" : risk.level === "medium" ? "var(--yunque-warning)" : "var(--yunque-success)",
              }}>
                {risk.label}
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
          {primaryEntry?.path && (
            <Chip size="sm" style={{ background: "rgba(255,255,255,0.05)", color: "var(--yunque-text-muted)" }}>
              入口 {primaryEntry.path}
            </Chip>
          )}
        </div>

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
    const navItems = navItemsForPack(pack);
    const openPath = manifest.frontend?.menus?.[0]?.path || manifest.frontend?.routes?.[0]?.path;

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
                <Chip size="sm" style={{
                  background: risk.level === "high" ? "rgba(239,68,68,0.12)" : risk.level === "medium" ? "rgba(245,158,11,0.12)" : "rgba(34,197,94,0.10)",
                  color: risk.level === "high" ? "var(--yunque-danger)" : risk.level === "medium" ? "var(--yunque-warning)" : "var(--yunque-success)",
                }}>
                  {risk.label}
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
                <ExternalLink size={14} /> 打开
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
