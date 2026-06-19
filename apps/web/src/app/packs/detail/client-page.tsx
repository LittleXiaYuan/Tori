"use client";

import { useEffect, useMemo, useState } from "react";
import Link from "next/link";
import { useSearchParams, useRouter } from "next/navigation";
import { Button, Card, Chip, Spinner } from "@heroui/react";
import {
  ArrowLeft,
  Boxes,
  ChevronDown,
  ChevronUp,
  Download,
  ExternalLink,
  Info,
  LockKeyhole,
  PackageCheck,
  PackageX,
  Power,
  RotateCcw,
  ShieldAlert,
  ShieldCheck,
  Sparkles,
  Workflow,
} from "lucide-react";
import PageHeader from "@/components/page-header";
import { showToast } from "@/components/toast-provider";
import {
  createPacksClient,
  type InstalledPack,
  type PackCatalogEntry,
  type PackManifest,
} from "yunque-client/packs";
import { createYunqueSDKClientOptions } from "@/lib/sdk-client";
import { formatErrorMessage } from "@/lib/error-utils";
import {
  capabilitySurfaceLabels,
  entryInstallRequest,
  formatPackInstallError,
  groupPackPermissions,
  packFeatureFlags,
  packReadiness,
  packUsageExplanation,
  packUsability,
  riskProfileForPack,
} from "@/lib/pack-presentation";

const packsClient = createPacksClient(createYunqueSDKClientOptions());

type DetailState = {
  manifest: PackManifest;
  installed: boolean;
  enabled: boolean;
  installedPack?: InstalledPack;
  catalogEntry?: PackCatalogEntry;
};

function statusTone(status: string): { label: string; color: string; bg: string } {
  if (status === "enabled") return { label: "已启用", color: "var(--yunque-success)", bg: "rgba(34,197,94,0.10)" };
  if (status === "disabled") return { label: "已禁用", color: "var(--yunque-text-muted)", bg: "rgba(255,255,255,0.05)" };
  return { label: status || "未知", color: "var(--yunque-warning)", bg: "rgba(245,158,11,0.12)" };
}

function packStatusBadge(packStatus?: string): { icon: string; label: string; color: string; bg: string } {
  if (packStatus === "stable") return { icon: "✅", label: "完整可用", color: "var(--yunque-success)", bg: "rgba(34,197,94,0.10)" };
  if (packStatus === "beta") return { icon: "⚠️", label: "部分可用", color: "var(--yunque-warning)", bg: "rgba(245,158,11,0.12)" };
  if (packStatus === "alpha") return { icon: "🚧", label: "开发中", color: "var(--yunque-text-muted)", bg: "rgba(255,255,255,0.05)" };
  return { icon: "❓", label: "未知", color: "var(--yunque-text-muted)", bg: "rgba(255,255,255,0.05)" };
}

function formatTime(value?: string): string {
  if (!value) return "-";
  const d = new Date(value);
  if (Number.isNaN(d.getTime())) return value;
  return d.toLocaleString();
}

function formatBytes(bytes?: number): string {
  if (!bytes || bytes <= 0) return "-";
  const k = 1024;
  const sizes = ["B", "KB", "MB", "GB"];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return `${parseFloat((bytes / Math.pow(k, i)).toFixed(1))} ${sizes[i]}`;
}

export default function PackDetailClientPage() {
  const searchParams = useSearchParams();
  const router = useRouter();
  const id = searchParams.get("id") || "";
  const [state, setState] = useState<DetailState | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState<string | null>(null);
  const [showAdvanced, setShowAdvanced] = useState(false);

  const reload = async () => {
    if (!id) {
      setLoading(false);
      setError("缺少 Pack ID");
      return;
    }
    try {
      setLoading(true);
      const [installedRes, catalogRes] = await Promise.all([
        packsClient.installed(),
        packsClient.catalog(),
      ]);
      const installedPack = installedRes.packs.find((p) => p.manifest.id === id);
      const catalogEntry = catalogRes.entries.find((e) => e.manifest.id === id);
      const manifest = installedPack?.manifest || catalogEntry?.manifest;
      if (!manifest) {
        setError(`未找到能力包：${id}`);
        setState(null);
        return;
      }
      setState({
        manifest,
        installed: Boolean(installedPack),
        enabled: installedPack?.status === "enabled",
        installedPack,
        catalogEntry,
      });
      setError(null);
    } catch (e) {
      setError(formatErrorMessage(e, "加载失败"));
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    void reload();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [id]);

  const run = async (label: string, op: () => Promise<unknown>) => {
    setBusy(label);
    try {
      await op();
      showToast("操作成功", "success");
      await reload();
    } catch (e) {
      showToast(label === "install" ? formatPackInstallError(e) : formatErrorMessage(e, "操作失败"), "error");
    } finally {
      setBusy(null);
    }
  };

  const examples = useMemo(() => {
    const m = state?.manifest.metadata || {};
    return [m.example1, m.example2, m.example3, m.example4, m.example5].filter(
      (v): v is string => typeof v === "string" && v.trim().length > 0,
    );
  }, [state]);

  if (loading) {
    return (
      <div className="flex items-center justify-center h-[60vh]">
        <Spinner size="lg" />
      </div>
    );
  }

  if (error || !state) {
    return (
      <div className="p-6">
        <Button variant="ghost" size="sm" onPress={() => router.push("/packs")}>
          <ArrowLeft size={14} /> 返回能力包列表
        </Button>
        <Card className="section-card p-12 text-center mt-4">
          <Info size={40} className="mx-auto mb-3" style={{ color: "var(--yunque-text-muted)" }} />
          <div className="text-sm font-medium" style={{ color: "var(--yunque-text)" }}>
            {error || "未找到能力包"}
          </div>
        </Card>
      </div>
    );
  }

  const { manifest, installed, enabled, installedPack, catalogEntry } = state;
  const tone = statusTone(installedPack?.status || "not-installed");
  const packBadge = packStatusBadge(manifest.status);
  const caps = manifest.backend?.capabilities || [];
  const routes = manifest.backend?.routes || [];
  const routeSpecs = manifest.backend?.routeSpecs || [];
  const permissions = manifest.backend?.permissions || [];
  const menus = manifest.frontend?.menus || [];
  const routesFrontend = manifest.frontend?.routes || [];
  const distribution = manifest.distribution;
  const sdk = manifest.sdk || {};
  const update = manifest.update;
  const sdkLanguages = Object.entries(sdk).filter(([, v]) => typeof v === "string" && v.trim());
  const permissionGroups = groupPackPermissions(permissions);
  const risk = riskProfileForPack(manifest);
  const featureFlags = packFeatureFlags(manifest);
  const surfaceLabels = capabilitySurfaceLabels(manifest);
  const usageExplanation = packUsageExplanation(manifest);
  const usability = packUsability(manifest);
  const readiness = packReadiness(manifest);
  const openPath = usability.primaryActionPath || menus[0]?.path || routesFrontend[0]?.path;

  const installFromCatalog = () => {
    if (!catalogEntry) {
      showToast("此能力包没有可用的安装源", "error");
      return;
    }
    const request = entryInstallRequest(catalogEntry);
    if (!request) {
      showToast("此能力包没有可用的安装源", "error");
      return;
    }
    return run("install", () =>
      packsClient.install(request),
    );
  };

  const enable = () => run("enable", () => packsClient.enable(manifest.id));
  const disable = () => run("disable", () => packsClient.disable(manifest.id));
  const rollback = () => run("rollback", () => packsClient.rollback(manifest.id));

  return (
    <div className="flex flex-col h-screen overflow-hidden">
      {/* 头部：返回 + 标题 + 状态 */}
      <div className="flex-shrink-0 p-5 border-b" style={{ borderColor: "var(--yunque-border)" }}>
        <div className="flex items-center gap-2 mb-3">
          <Button variant="ghost" size="sm" onPress={() => router.push("/packs")}>
            <ArrowLeft size={14} /> 返回
          </Button>
        </div>
        <PageHeader
          icon={<Boxes size={20} />}
          title={manifest.name}
          description={manifest.description || "可选能力包"}
        />
        <div className="flex items-center gap-2 flex-wrap mt-3">
          <Chip size="sm" style={{ background: packBadge.bg, color: packBadge.color }}>
            {packBadge.icon} {packBadge.label}
          </Chip>
          {installed ? (
            <Chip size="sm" style={{ background: tone.bg, color: tone.color }}>{tone.label}</Chip>
          ) : (
            <Chip size="sm" style={{ background: "rgba(255,255,255,0.05)", color: "var(--yunque-text-muted)" }}>
              未安装
            </Chip>
          )}
          <Chip size="sm" variant="soft">v{manifest.version}</Chip>
          <Chip size="sm" style={{ background: "rgba(59,130,246,0.08)", color: "var(--yunque-primary)" }}>
            {usability.label}
          </Chip>
          <Chip size="sm" variant="soft" className="font-mono">{manifest.id}</Chip>
        </div>

        {/* 主操作区 */}
        <div className="flex items-center gap-2 mt-4">
          {!installed && catalogEntry && (
            <Button className="btn-accent" isDisabled={busy === "install"} onPress={installFromCatalog}>
              <Download size={14} /> 安装
            </Button>
          )}
          {installed && !enabled && (
            <Button className="btn-accent" isDisabled={busy === "enable"} onPress={enable}>
              <Power size={14} /> 启用
            </Button>
          )}
          {installed && enabled && (
            <Button variant="outline" isDisabled={busy === "disable"} onPress={disable}>
              <PackageX size={14} /> 禁用
            </Button>
          )}
          {installed && update?.rollback && installedPack?.previousVersion && (
            <Button variant="ghost" isDisabled={busy === "rollback"} onPress={rollback}>
              <RotateCcw size={14} /> 回滚到 v{installedPack.previousVersion}
            </Button>
          )}
          {installed && enabled && openPath && (
            <Link href={openPath}>
              <Button variant="outline">
                <ExternalLink size={14} /> {usability.primaryActionLabel || "打开能力界面"}
              </Button>
            </Link>
          )}
        </div>
      </div>

      {/* 可滚动内容 */}
      <div className="flex-1 overflow-y-auto p-5 space-y-4">
        {/* 场景化能做什么 */}
        <Card className="section-card p-4">
          <div className="flex items-center gap-2 mb-3">
            <Sparkles size={16} style={{ color: "var(--yunque-accent)" }} />
            <div className="text-sm font-semibold" style={{ color: "var(--yunque-text)" }}>
              用户能拿它做什么
            </div>
          </div>
          <div className="text-sm" style={{ color: "var(--yunque-text-secondary)" }}>
            {usability.description}
          </div>
          {openPath && (
            <div className="flex items-center gap-2 mt-3">
              <Link href={openPath}>
                <Button size="sm" className="btn-accent">
                  <ExternalLink size={14} /> {usability.primaryActionLabel || "打开入口"}
                </Button>
              </Link>
              <code className="text-xs" style={{ color: "var(--yunque-text-muted)" }}>{openPath}</code>
            </div>
          )}
          {usability.limitation && (
            <div className="mt-3 rounded-md p-3 text-xs" style={{ background: "rgba(245,158,11,0.10)", color: "var(--yunque-warning)" }}>
              当前限制：{usability.limitation}
            </div>
          )}
        </Card>

        <Card className="section-card p-4">
          <div className="flex items-center gap-2 mb-3">
            <ShieldCheck size={16} style={{ color: "var(--yunque-primary)" }} />
            <div className="text-sm font-semibold" style={{ color: "var(--yunque-text)" }}>
              能力包体检
            </div>
          </div>
          <div className="flex flex-wrap items-center gap-2">
            <Chip size="sm" style={{
              background: readiness.level === "complete" ? "rgba(34,197,94,0.10)" : readiness.level === "needs_context" ? "rgba(245,158,11,0.12)" : "rgba(239,68,68,0.10)",
              color: readiness.level === "complete" ? "var(--yunque-success)" : readiness.level === "needs_context" ? "var(--yunque-warning)" : "var(--yunque-danger)",
            }}>
              {readiness.label}
            </Chip>
            <span className="text-sm" style={{ color: "var(--yunque-text-secondary)" }}>{readiness.description}</span>
          </div>
          {readiness.missing.length > 0 ? (
            <div className="mt-3 rounded-md p-3 text-xs" style={{ background: "rgba(245,158,11,0.08)", color: "var(--yunque-text-secondary)" }}>
              还缺：{readiness.missing.join("、")}。可以回到能力包中心点“小羽优化”，补齐用途、入口、示例或边界说明。
            </div>
          ) : (
            <div className="mt-3 text-xs" style={{ color: "var(--yunque-text-muted)" }}>
              已声明用途、入口或使用面、示例和能力边界；仍可继续用小羽优化文案或补更具体的场景。
            </div>
          )}
        </Card>

        {examples.length > 0 && (
          <Card className="section-card p-4">
            <div className="flex items-center gap-2 mb-3">
              <Sparkles size={16} style={{ color: "var(--yunque-accent)" }} />
              <div className="text-sm font-semibold" style={{ color: "var(--yunque-text)" }}>
                启用后，云雀能帮你
              </div>
            </div>
            <div className="space-y-2">
              {examples.map((example, idx) => (
                <div key={idx} className="flex items-start gap-2 text-sm" style={{ color: "var(--yunque-text-secondary)" }}>
                  <span style={{ color: "var(--yunque-accent)" }}>•</span>
                  <span>{example}</span>
                </div>
              ))}
            </div>
          </Card>
        )}

        <Card className="section-card p-4">
          <div className="flex items-center gap-2 mb-3">
            <ShieldCheck size={16} style={{ color: "var(--yunque-accent)" }} />
            <div className="text-sm font-semibold" style={{ color: "var(--yunque-text)" }}>
              安装前确认
            </div>
          </div>
          <div className="grid grid-cols-1 md:grid-cols-2 gap-3">
            <div className="rounded-md p-3 border" style={{ borderColor: "var(--yunque-border)" }}>
              <div className="text-xs font-medium mb-2" style={{ color: "var(--yunque-text)" }}>它会获得什么能力</div>
              {permissionGroups.length > 0 ? (
                <div className="space-y-2">
                  {permissionGroups.map((group) => (
                    <div key={group.key}>
                      <div className="flex items-center gap-2 text-xs" style={{ color: "var(--yunque-text-secondary)" }}>
                        <LockKeyhole size={12} style={{ color: "var(--yunque-warning)" }} />
                        <span className="font-medium">{group.label}</span>
                        <span style={{ color: "var(--yunque-text-muted)" }}>{group.permissions.length} 项</span>
                      </div>
                      <div className="text-[11px] mt-1" style={{ color: "var(--yunque-text-muted)" }}>{group.description}</div>
                    </div>
                  ))}
                </div>
              ) : (
                <div className="text-xs" style={{ color: "var(--yunque-text-muted)" }}>未声明额外权限。</div>
              )}
            </div>
            <div className="rounded-md p-3 border" style={{ borderColor: "var(--yunque-border)" }}>
              <div className="text-xs font-medium mb-2" style={{ color: "var(--yunque-text)" }}>风险与回滚</div>
              <Chip size="sm" style={{
                background: risk.level === "high" ? "rgba(239,68,68,0.12)" : risk.level === "medium" ? "rgba(245,158,11,0.12)" : "rgba(34,197,94,0.10)",
                color: risk.level === "high" ? "var(--yunque-danger)" : risk.level === "medium" ? "var(--yunque-warning)" : "var(--yunque-success)",
              }}>
                {risk.label}
              </Chip>
              <div className="text-xs mt-2" style={{ color: "var(--yunque-text-muted)" }}>{risk.description}</div>
              <div className="text-xs mt-2" style={{ color: "var(--yunque-text-muted)" }}>
                不会做什么：启用能力包不会自动泄露 API Key，不会绕过云雀的权限声明，也不会获得未声明 route 的调用能力。
              </div>
              <div className="text-xs mt-2" style={{ color: "var(--yunque-text-muted)" }}>
                {update?.rollback ? "支持回滚到上一版本；也可以随时禁用。" : "可以随时禁用；此包未声明版本回滚。"}
              </div>
            </div>
          </div>
          {risk.requiresAuthorization && (
            <div className="mt-3 flex items-start gap-2 rounded-md p-3" style={{ background: "rgba(239,68,68,0.10)", color: "var(--yunque-warning)" }}>
              <ShieldAlert size={15} className="mt-0.5" />
              <div className="text-xs">
                这个能力包涉及高风险能力。请确认来源可信，并在启用后按需授权具体动作。
              </div>
            </div>
          )}
        </Card>

        <Card className="section-card p-4">
          <div className="flex items-center gap-2 mb-3">
            <Sparkles size={16} style={{ color: "var(--yunque-primary)" }} />
            <div className="text-sm font-semibold" style={{ color: "var(--yunque-text)" }}>
              云雀如何使用它
            </div>
          </div>
          <div className="space-y-2">
            {usageExplanation.map((line) => (
              <div key={line} className="flex items-start gap-2 text-sm" style={{ color: "var(--yunque-text-secondary)" }}>
                <span style={{ color: "var(--yunque-accent)" }}>•</span>
                <span>{line}</span>
              </div>
            ))}
          </div>
        </Card>

        {/* 能力清单 */}
        {caps.length > 0 && (
          <Card className="section-card p-4">
            <div className="flex items-center gap-2 mb-3">
              <Workflow size={16} style={{ color: "var(--yunque-primary)" }} />
              <div className="text-sm font-semibold" style={{ color: "var(--yunque-text)" }}>
                提供的能力 ({caps.length})
              </div>
            </div>
            <div className="flex flex-wrap gap-1.5">
              {caps.map((cap) => (
                <Chip
                  key={cap}
                  size="sm"
                  className="font-mono"
                  style={{ background: "rgba(59,130,246,0.08)", color: "var(--yunque-primary)" }}
                >
                  {cap}
                </Chip>
              ))}
            </div>
          </Card>
        )}

        {surfaceLabels.length > 0 && (
          <Card className="section-card p-4">
            <div className="flex items-center gap-2 mb-3">
              <Workflow size={16} style={{ color: "var(--yunque-accent)" }} />
              <div className="text-sm font-semibold" style={{ color: "var(--yunque-text)" }}>
                能力形态
              </div>
            </div>
            <div className="flex flex-wrap gap-1.5">
              {surfaceLabels.map((label) => (
                <Chip key={label} size="sm" style={{ background: "rgba(59,130,246,0.08)", color: "var(--yunque-primary)" }}>
                  {label}
                </Chip>
              ))}
            </div>
            {featureFlags.isIframeBundle && (
              <div className="mt-3 rounded-md p-3 text-xs" style={{ background: "rgba(59,130,246,0.08)", color: "var(--yunque-text-secondary)" }}>
                独立界面包运行在 iframe 沙箱中：不直接获得云雀 token，默认隔离页面能力，只能通过自身声明的 route 与云雀通信，越权 bridge call 会被拒绝并记录。
              </div>
            )}
          </Card>
        )}

        {/* 前端入口 */}
        {(menus.length > 0 || routesFrontend.length > 0) && (
          <Card className="section-card p-4">
            <div className="text-sm font-semibold mb-3" style={{ color: "var(--yunque-text)" }}>
              界面入口
            </div>
            <div className="space-y-2">
              {[...menus, ...routesFrontend.map((route) => ({ key: route.path, label: route.title || route.path, path: route.path }))].map((menu) => (
                <Link key={menu.key} href={menu.path}>
                  <div
                    className="flex items-center justify-between p-2 rounded hover:bg-white/5 transition-colors text-sm"
                    style={{ color: "var(--yunque-text-secondary)" }}
                  >
                    <div className="flex items-center gap-2">
                      <ExternalLink size={14} style={{ color: "var(--yunque-accent)" }} />
                      <span>{menu.label}</span>
                    </div>
                    <span className="text-xs font-mono" style={{ color: "var(--yunque-text-muted)" }}>
                      {menu.path}
                    </span>
                  </div>
                </Link>
              ))}
            </div>
          </Card>
        )}

        {/* SDK 入口 */}
        {sdkLanguages.length > 0 && (
          <Card className="section-card p-4">
            <div className="text-sm font-semibold mb-3" style={{ color: "var(--yunque-text)" }}>
              开发者 SDK
            </div>
            <div className="space-y-1">
              {sdkLanguages.map(([lang, importPath]) => (
                <div key={lang} className="flex items-center gap-3 text-xs">
                  <Chip size="sm" variant="soft">{lang}</Chip>
                  <code className="font-mono" style={{ color: "var(--yunque-text-secondary)" }}>
                    {String(importPath)}
                  </code>
                </div>
              ))}
            </div>
          </Card>
        )}

        {/* 高级技术详情 */}
        <Card className="section-card p-4">
          <button
            type="button"
            onClick={() => setShowAdvanced(!showAdvanced)}
            className="flex items-center gap-2 w-full text-sm font-semibold"
            style={{ color: "var(--yunque-text)" }}
          >
            {showAdvanced ? <ChevronUp size={14} /> : <ChevronDown size={14} />}
            技术详情
          </button>
          {showAdvanced && (
            <div className="mt-3 pt-3 border-t text-xs space-y-3" style={{ borderColor: "var(--yunque-border)" }}>
              <div className="grid grid-cols-2 gap-3" style={{ color: "var(--yunque-text-muted)" }}>
                <div>
                  <div className="text-[10px] uppercase opacity-60">ID</div>
                  <div className="font-mono">{manifest.id}</div>
                </div>
                <div>
                  <div className="text-[10px] uppercase opacity-60">版本</div>
                  <div>v{manifest.version}</div>
                </div>
                <div>
                  <div className="text-[10px] uppercase opacity-60">最低核心版本</div>
                  <div>{manifest.requiresCore || "-"}</div>
                </div>
                <div>
                  <div className="text-[10px] uppercase opacity-60">默认状态</div>
                  <div>{manifest.defaultState || "-"}</div>
                </div>
                {installedPack && (
                  <>
                    <div>
                      <div className="text-[10px] uppercase opacity-60">安装时间</div>
                      <div>{formatTime(installedPack.installedAt)}</div>
                    </div>
                    <div>
                      <div className="text-[10px] uppercase opacity-60">更新时间</div>
                      <div>{formatTime(installedPack.updatedAt)}</div>
                    </div>
                    {installedPack.previousVersion && (
                      <div className="col-span-2">
                        <div className="text-[10px] uppercase opacity-60">上一版本</div>
                        <div>v{installedPack.previousVersion}</div>
                      </div>
                    )}
                  </>
                )}
                {distribution?.sizeBytes ? (
                  <div>
                    <div className="text-[10px] uppercase opacity-60">体积</div>
                    <div>{formatBytes(distribution.sizeBytes)}</div>
                  </div>
                ) : null}
                {update?.channel && (
                  <div>
                    <div className="text-[10px] uppercase opacity-60">更新通道</div>
                    <div>{update.channel}</div>
                  </div>
                )}
              </div>

              {routeSpecs.length > 0 && (
                <div>
                  <div className="text-[10px] uppercase opacity-60 mb-1">后端路由 ({routeSpecs.length})</div>
                  <div className="space-y-1 font-mono" style={{ color: "var(--yunque-text-secondary)" }}>
                    {routeSpecs.map((spec, idx) => (
                      <div key={idx} className="flex items-start gap-2">
                        <Chip size="sm" variant="soft">{spec.method}</Chip>
                        <span>{spec.path}</span>
                      </div>
                    ))}
                  </div>
                </div>
              )}
              {routeSpecs.length === 0 && routes.length > 0 && (
                <div>
                  <div className="text-[10px] uppercase opacity-60 mb-1">后端路由 ({routes.length})</div>
                  <div className="space-y-1 font-mono" style={{ color: "var(--yunque-text-secondary)" }}>
                    {routes.map((path, idx) => (
                      <div key={idx}>{path}</div>
                    ))}
                  </div>
                </div>
              )}

              {distribution?.packageUrl && (
                <div>
                  <div className="text-[10px] uppercase opacity-60 mb-1">分发源</div>
                  <div className="font-mono break-all" style={{ color: "var(--yunque-text-secondary)" }}>
                    {distribution.packageUrl}
                  </div>
                  {distribution.sha256 && (
                    <div className="font-mono text-[10px] mt-1" style={{ color: "var(--yunque-text-muted)" }}>
                      sha256: {distribution.sha256}
                    </div>
                  )}
                </div>
              )}
            </div>
          )}
        </Card>
      </div>
    </div>
  );
}
