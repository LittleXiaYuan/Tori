"use client";

import { useEffect, useMemo, useState } from "react";
import Link from "next/link";
import { Button, Card, Chip, Spinner, TextField, Input, Label } from "@heroui/react";
import {
  ArrowRight,
  Boxes,
  ChevronDown,
  ChevronUp,
  Download,
  Pin,
  PinOff,
  PackageCheck,
  PackageX,
  Power,
  RotateCcw,
  ShieldCheck,
} from "lucide-react";
import PageHeader from "@/components/page-header";
import { showToast } from "@/components/toast-provider";
import { createPacksClient, type InstalledPack, type PackReleaseCatalogEntry } from "yunque-client/packs";
import { createYunqueSDKClientOptions } from "@/lib/sdk-client";
import { useApiData } from "@/lib/use-api-data";
import { formatErrorMessage } from "@/lib/error-utils";
import { buildPackNavItems } from "@/lib/pack-sync";
import { useNavigationPreferences } from "@/hooks/use-user-preferences";

const OFFICIAL_PACK_RELEASES = [
  "https://github.com/LittleXiaYuan/Tori/releases/tag/pack%2Fmicro-agent%2Fv0.1.0",
];
const OFFICIAL_BACKUP_MANIFEST = "packs/official/backup-pack/pack.json";
const packsClient = createPacksClient(createYunqueSDKClientOptions());

function formatTime(value?: string): string {
  if (!value) return "-";
  const d = new Date(value);
  if (Number.isNaN(d.getTime())) return value;
  return d.toLocaleString();
}

function statusTone(status: string): { label: string; color: string; bg: string } {
  if (status === "enabled") return { label: "已启用", color: "var(--yunque-success)", bg: "rgba(34,197,94,0.10)" };
  if (status === "disabled") return { label: "已禁用", color: "var(--yunque-text-muted)", bg: "rgba(255,255,255,0.05)" };
  return { label: status || "未知", color: "var(--yunque-warning)", bg: "rgba(245,158,11,0.12)" };
}

function packStatusBadge(packStatus?: string): { icon: string; label: string; color: string; bg: string } {
  if (packStatus === "stable") return { icon: "✅", label: "正式版", color: "var(--yunque-success)", bg: "rgba(34,197,94,0.10)" };
  if (packStatus === "beta") return { icon: "🧪", label: "测试版", color: "var(--yunque-warning)", bg: "rgba(245,158,11,0.12)" };
  if (packStatus === "alpha") return { icon: "🚧", label: "开发中", color: "var(--yunque-text-muted)", bg: "rgba(255,255,255,0.05)" };
  return { icon: "❓", label: "未知", color: "var(--yunque-text-muted)", bg: "rgba(255,255,255,0.05)" };
}

function countLabel(count: number): string {
  return `${count} 个`;
}

export default function PacksPageOptimized() {
  const navigationPrefs = useNavigationPreferences();
  const { data, loading, refresh } = useApiData(async () => packsClient.installed(), { packs: [], count: 0 });
  const { data: releaseCatalog, loading: releaseLoading, refresh: refreshReleaseCatalog } = useApiData(
    async () => packsClient.releaseCatalog(OFFICIAL_PACK_RELEASES),
    { generated_at: "", releases: OFFICIAL_PACK_RELEASES, count: 0, entries: [] as PackReleaseCatalogEntry[] },
  );
  const [manifestPath, setManifestPath] = useState(OFFICIAL_BACKUP_MANIFEST);
  const [busy, setBusy] = useState<string | null>(null);
  const [showAdvanced, setShowAdvanced] = useState(false);
  const [showAlpha, setShowAlpha] = useState(false);

  const packs = data?.packs || [];

  // 按 Pack 状态分组
  const groupedPacks = useMemo(() => {
    const stable = packs.filter((p) => p.manifest.status === "stable");
    const beta = packs.filter((p) => p.manifest.status === "beta");
    const alpha = packs.filter((p) => p.manifest.status === "alpha");
    return { stable, beta, alpha };
  }, [packs]);

  const stats = useMemo(() => ({
    total: packs.length,
    enabled: packs.filter((p) => p.status === "enabled").length,
    available: releaseCatalog.count || releaseCatalog.entries.length,
  }), [packs, releaseCatalog]);

  const run = async (label: string, op: () => Promise<unknown>) => {
    setBusy(label);
    try {
      await op();
      showToast("操作成功", "success");
      await refreshAll();
    } catch (e) {
      showToast(formatErrorMessage(e, "操作失败"), "error");
    } finally {
      setBusy(null);
    }
  };

  const refreshAll = async () => {
    await Promise.all([refresh(), refreshReleaseCatalog()]);
  };

  const installLocal = () => run("install:local", () => packsClient.install({ manifestPath, download: false }));
  const installRelease = (entry: PackReleaseCatalogEntry) => run(`install-release:${entry.package_url}`, () => packsClient.install({
    packageUrl: entry.package_url,
    sha256: entry.sha256,
    source: entry.release_url,
  }));
  const enable = (id: string) => run(`enable:${id}`, () => packsClient.enable(id));
  const disable = (id: string) => run(`disable:${id}`, () => packsClient.disable(id));
  const rollback = (id: string) => run(`rollback:${id}`, () => packsClient.rollback(id));

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
      {/* 固定头部 */}
      <div className="flex-shrink-0 p-5 border-b" style={{ borderColor: "var(--yunque-border)" }}>
        <PageHeader
          icon={<Boxes size={20} />}
          title="能力包"
          description="从官方发布中选择、安装和管理可选能力"
          onRefresh={refreshAll}
        />

        {/* 统计卡片 */}
        <div className="grid grid-cols-3 gap-3 mt-4">
          <Card className="section-card p-4">
            <div className="kpi-label">可安装</div>
            <div className="kpi-value">{releaseLoading ? "…" : stats.available}</div>
          </Card>
          <Card className="section-card p-4">
            <div className="kpi-label">已安装</div>
            <div className="kpi-value">{stats.total}</div>
          </Card>
          <Card className="section-card p-4">
            <div className="kpi-label">已启用</div>
            <div className="kpi-value">{stats.enabled}</div>
          </Card>
        </div>

        {/* 显示选项 */}
        <div className="flex items-center gap-3 mt-4">
          <Button
            size="sm"
            variant="ghost"
            onPress={() => setShowAdvanced(!showAdvanced)}
          >
            {showAdvanced ? <ChevronUp size={14} /> : <ChevronDown size={14} />}
            {showAdvanced ? "隐藏" : "显示"}技术详情
          </Button>
          <Button
            size="sm"
            variant="ghost"
            onPress={() => setShowAlpha(!showAlpha)}
          >
            {showAlpha ? "隐藏" : "显示"}开发中功能
          </Button>
        </div>
      </div>

      {/* 可滚动内容区域 */}
      <div className="flex-1 overflow-y-auto p-5 space-y-4">
        {/* 官方发布 */}
        <Card className="section-card p-4">
          <div className="flex items-center justify-between gap-3 mb-3">
            <div>
              <div className="text-sm font-semibold" style={{ color: "var(--yunque-text)" }}>官方能力包</div>
              <div className="text-xs mt-1" style={{ color: "var(--yunque-text-muted)" }}>
                从 GitHub Release 读取可安装包，选择卡片即可安装。
              </div>
            </div>
            <Button size="sm" variant="ghost" onPress={refreshReleaseCatalog} isDisabled={releaseLoading}>
              {releaseLoading ? <Spinner size="sm" /> : <RotateCcw size={14} />}
              刷新
            </Button>
          </div>

          {releaseLoading ? (
            <div className="flex items-center gap-2 text-xs py-6" style={{ color: "var(--yunque-text-muted)" }}>
              <Spinner size="sm" /> 正在读取发布内容…
            </div>
          ) : releaseCatalog.entries.length > 0 ? (
            <div className="grid grid-cols-1 lg:grid-cols-2 gap-4">
              {releaseCatalog.entries.map((entry) => renderReleasePackCard(entry))}
            </div>
          ) : (
            <div className="text-xs py-4" style={{ color: "var(--yunque-text-muted)" }}>
              暂时没有读到可安装的 .yqpack 发布包。
            </div>
          )}

          {showAdvanced && (
            <div className="mt-4 pt-4 border-t" style={{ borderColor: "var(--yunque-border)" }}>
              <div className="text-xs font-medium mb-2" style={{ color: "var(--yunque-text-secondary)" }}>本地 Manifest 安装</div>
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

        {/* 已安装的包列表 */}
        {packs.length === 0 ? (
          <Card className="section-card p-12 text-center">
            <Boxes size={40} className="mx-auto mb-3" style={{ color: "var(--yunque-text-muted)" }} />
            <div className="text-sm font-medium" style={{ color: "var(--yunque-text)" }}>还没有安装能力包</div>
            <div className="text-xs mt-1" style={{ color: "var(--yunque-text-muted)" }}>
              先从上方官方发布里选择一个能力包
            </div>
          </Card>
        ) : (
          <div className="space-y-6">
            {/* 正式版 Pack */}
            {groupedPacks.stable.length > 0 && (
              <div>
                <div className="flex items-center gap-2 mb-3">
                  <span className="text-lg">✅</span>
                  <h3 className="text-sm font-semibold" style={{ color: "var(--yunque-text)" }}>
                    已安装的正式版能力包 · {countLabel(groupedPacks.stable.length)}
                  </h3>
                </div>
                <div className="grid grid-cols-1 lg:grid-cols-2 gap-4">
                  {groupedPacks.stable.map((pack) => renderPackCard(pack))}
                </div>
              </div>
            )}

            {/* 测试版 Pack */}
            {groupedPacks.beta.length > 0 && (
              <div>
                <div className="flex items-center gap-2 mb-3">
                  <span className="text-lg">⚠️</span>
                  <h3 className="text-sm font-semibold" style={{ color: "var(--yunque-text)" }}>
                    已安装的测试版能力包 · {countLabel(groupedPacks.beta.length)}
                  </h3>
                </div>
                <div className="grid grid-cols-1 lg:grid-cols-2 gap-4">
                  {groupedPacks.beta.map((pack) => renderPackCard(pack))}
                </div>
              </div>
            )}

            {/* 开发中的 Pack（可选显示） */}
            {showAlpha && groupedPacks.alpha.length > 0 && (
              <div>
                <div className="flex items-center gap-2 mb-3">
                  <span className="text-lg">🚧</span>
                  <h3 className="text-sm font-semibold" style={{ color: "var(--yunque-text)" }}>
                    已安装的开发中能力包 · {countLabel(groupedPacks.alpha.length)}
                  </h3>
                  <Chip size="sm" style={{ background: "rgba(245,158,11,0.12)", color: "var(--yunque-warning)" }}>
                    仅供测试
                  </Chip>
                </div>
                <div className="grid grid-cols-1 lg:grid-cols-2 gap-4">
                  {groupedPacks.alpha.map((pack) => renderPackCard(pack))}
                </div>
              </div>
            )}
          </div>
        )}
      </div>
    </div>
  );

  function renderReleasePackCard(entry: PackReleaseCatalogEntry) {
    const manifest = entry.manifest;
    const packBadge = packStatusBadge(manifest.status as string | undefined);
    const metadata = manifest.metadata || {};
    const examples = [metadata.example1, metadata.example2, metadata.example3].filter(Boolean);
    const caps = manifest.backend?.capabilities || [];
    const actionLabel =
      entry.update_action === "use" ? "已安装" :
      entry.update_action === "enable" ? "已安装，待启用" :
      entry.update_action === "update" ? "更新" :
      "安装";
    const disabled = entry.update_action === "use" || !entry.downloadable || busy === `install-release:${entry.package_url}`;

    return (
      <Card key={entry.package_url} className="section-card p-4 hover-lift">
        <div className="flex items-start justify-between gap-3 mb-3">
          <div className="min-w-0 flex-1">
            <div className="flex items-center gap-2 flex-wrap">
              <PackageCheck size={16} style={{ color: "var(--yunque-accent)" }} />
              <span className="font-semibold text-sm" style={{ color: "var(--yunque-text)" }}>{manifest.name}</span>
              <Chip size="sm" style={{ background: packBadge.bg, color: packBadge.color }}>
                {packBadge.icon} {packBadge.label}
              </Chip>
              <Chip size="sm" style={{ background: "rgba(34,197,94,0.10)", color: "var(--yunque-success)" }}>
                <ShieldCheck size={12} /> 官方发布
              </Chip>
            </div>
            {manifest.description && (
              <div className="text-xs mt-2" style={{ color: "var(--yunque-text-secondary)" }}>{manifest.description}</div>
            )}
          </div>
          <Button
            size="sm"
            className={entry.update_action === "update" ? "btn-accent" : undefined}
            variant={entry.update_action === "update" ? undefined : "outline"}
            isDisabled={disabled}
            onPress={() => installRelease(entry)}
          >
            <Download size={14} /> {actionLabel}
          </Button>
        </div>

        {examples.length > 0 && (
          <div className="mb-3 space-y-1">
            {examples.map((example, idx) => (
              <div key={idx} className="flex items-start gap-2 text-xs" style={{ color: "var(--yunque-text-muted)" }}>
                <span style={{ color: "var(--yunque-accent)" }}>•</span>
                <span>{example}</span>
              </div>
            ))}
          </div>
        )}

        <div className="flex items-center gap-2 flex-wrap text-xs" style={{ color: "var(--yunque-text-muted)" }}>
          <span>v{manifest.version}</span>
          {entry.release_tag && <span>· {entry.release_tag}</span>}
          {entry.size_bytes ? <span>· {(entry.size_bytes / 1024).toFixed(1)} KB</span> : null}
        </div>

        {showAdvanced && (
          <div className="mt-3 pt-3 border-t text-xs space-y-2" style={{ borderColor: "var(--yunque-border)", color: "var(--yunque-text-muted)" }}>
            {caps.length > 0 && (
              <div className="flex flex-wrap gap-1.5">
                {caps.map((cap) => (
                  <Chip key={cap} size="sm" style={{ background: "rgba(59,130,246,0.08)", color: "var(--yunque-primary)" }}>
                    {cap}
                  </Chip>
                ))}
              </div>
            )}
            <div className="font-mono break-all">{entry.asset_name || entry.package_url}</div>
            {entry.sha256 && <div className="font-mono break-all">SHA256：{entry.sha256}</div>}
          </div>
        )}
      </Card>
    );
  }

  function renderPackCard(pack: InstalledPack) {
    const tone = statusTone(pack.status);
    const packBadge = packStatusBadge(pack.manifest.status as string | undefined);
    const manifest = pack.manifest;
    const caps = manifest.backend?.capabilities || [];
    const menus = manifest.frontend?.menus || [];
    const metadata = manifest.metadata || {};

    // 提取用户友好的功能描述
    const friendlyExamples = [
      metadata.example1,
      metadata.example2,
      metadata.example3,
    ].filter(Boolean);

    return (
      <Card key={manifest.id} className="section-card p-4 hover-lift">
        <Link
          href={`/packs/detail?id=${encodeURIComponent(manifest.id)}`}
          className="block cursor-pointer"
        >
          <div className="flex items-start justify-between gap-3 mb-3">
            <div className="min-w-0 flex-1">
              <div className="flex items-center gap-2 flex-wrap">
                <PackageCheck size={16} style={{ color: "var(--yunque-accent)" }} />
                <span className="font-semibold text-sm" style={{ color: "var(--yunque-text)" }}>{manifest.name}</span>
                <Chip size="sm" style={{ background: tone.bg, color: tone.color }}>{tone.label}</Chip>
                <Chip size="sm" style={{ background: packBadge.bg, color: packBadge.color }}>
                  {packBadge.icon} {packBadge.label}
                </Chip>
              </div>
              <div className="text-xs mt-1 font-mono" style={{ color: "var(--yunque-text-muted)" }}>{manifest.id}</div>
              {manifest.description && (
                <div className="text-xs mt-2" style={{ color: "var(--yunque-text-secondary)" }}>{manifest.description}</div>
              )}
            </div>
          </div>

          {/* 用户友好的功能说明 */}
          {friendlyExamples.length > 0 && (
            <div className="mb-3 space-y-1">
              {friendlyExamples.map((example, idx) => (
                <div key={idx} className="flex items-start gap-2 text-xs" style={{ color: "var(--yunque-text-muted)" }}>
                  <span style={{ color: "var(--yunque-accent)" }}>•</span>
                  <span>{example}</span>
                </div>
              ))}
            </div>
          )}

          {/* 技术能力标签（仅在高级模式显示） */}
          {showAdvanced && caps.length > 0 && (
            <div className="flex flex-wrap gap-1.5 mb-3">
              {caps.slice(0, 4).map((cap) => (
                <Chip key={cap} size="sm" style={{ background: "rgba(59,130,246,0.08)", color: "var(--yunque-primary)" }}>
                  {cap}
                </Chip>
              ))}
              {caps.length > 4 && (
                <Chip size="sm" style={{ background: "rgba(255,255,255,0.05)", color: "var(--yunque-text-muted)" }}>
                  +{caps.length - 4}
                </Chip>
              )}
            </div>
          )}
        </Link>

        {/* 操作按钮（不在 Link 内，避免点击穿透） */}
        <div className="flex items-center gap-2">
          {pack.status === "enabled" ? (
            <Button size="sm" variant="outline" isDisabled={busy === `disable:${manifest.id}`} onPress={() => disable(manifest.id)}>
              <PackageX size={14} /> 禁用
            </Button>
          ) : (
            <Button size="sm" className="btn-accent" isDisabled={busy === `enable:${manifest.id}`} onPress={() => enable(manifest.id)}>
              <Power size={14} /> 启用
            </Button>
          )}
          {manifest.update?.rollback && pack.previousVersion && (
            <Button
              size="sm"
              variant="ghost"
              isDisabled={busy === `rollback:${manifest.id}`}
              onPress={() => rollback(manifest.id)}
            >
              <RotateCcw size={14} /> 回滚
            </Button>
          )}
          {pack.status === "enabled" && navItemsForPack(pack).length > 0 && (
            <Button size="sm" variant="ghost" onPress={() => togglePackPinned(pack)}>
              {isPackPinned(pack) ? <PinOff size={14} /> : <Pin size={14} />}
              {isPackPinned(pack) ? "取消侧栏" : "固定侧栏"}
            </Button>
          )}
          <Link href={`/packs/detail?id=${encodeURIComponent(manifest.id)}`} className="ml-auto">
            <Button size="sm" variant="ghost">
              详情 <ArrowRight size={14} />
            </Button>
          </Link>
        </div>

        {/* 展开详情 */}
        {showAdvanced && (
          <div className="mt-3 pt-3 border-t text-xs space-y-1" style={{ borderColor: "var(--yunque-border)", color: "var(--yunque-text-muted)" }}>
            <div>版本：v{manifest.version}</div>
            <div>更新时间：{formatTime(pack.updatedAt)}</div>
            {pack.previousVersion && <div>上一版本：{pack.previousVersion}</div>}
            {menus.length > 0 && (
              <div>前端入口：{menus.map(m => m.label).join(", ")}</div>
            )}
          </div>
        )}
      </Card>
    );
  }
}
