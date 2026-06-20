"use client";

import { useEffect, useMemo, useState } from "react";
import Link from "next/link";
import { useSearchParams, useRouter } from "next/navigation";
import { Button, Card, Chip, Spinner } from "@heroui/react";
import {
  ArrowLeft,
  ArrowRight,
  Boxes,
  ChevronDown,
  ChevronUp,
  Download,
  ExternalLink,
  Info,
  LockKeyhole,
  MessageSquare,
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
  type PackReleaseCatalogEntry,
} from "yunque-client/packs";
import { createYunqueSDKClientOptions } from "@/lib/sdk-client";
import { formatErrorMessage } from "@/lib/error-utils";
import {
  capabilitySurfaceLabels,
  entryInstallRequest,
  formatPackInstallError,
  groupPackPermissions,
  packInstallChecklist,
  packDeliveryProfile,
  packFeatureFlags,
  packPermissionSummary,
  packReadiness,
  packUsageExplanation,
  packUsability,
  packVerificationSteps,
  riskProfileForPack,
} from "@/lib/pack-presentation";
import { chatPromptHref } from "@/lib/pack-action-links";
import { resolvePackReleaseSources } from "@/lib/pack-release-sources";

const packsClient = createPacksClient(createYunqueSDKClientOptions());
const PACK_RELEASE_SOURCES = resolvePackReleaseSources();

type DetailState = {
  manifest: PackManifest;
  installed: boolean;
  enabled: boolean;
  installedPack?: InstalledPack;
  catalogEntry?: PackCatalogEntry;
  releaseEntry?: PackReleaseCatalogEntry;
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

function deliveryToneStyle(tone: ReturnType<typeof packDeliveryProfile>["tone"]): { background: string; borderColor: string; color: string } {
  if (tone === "success") return { background: "rgba(34,197,94,0.10)", borderColor: "rgba(34,197,94,0.28)", color: "var(--yunque-success)" };
  if (tone === "primary") return { background: "rgba(59,130,246,0.10)", borderColor: "rgba(59,130,246,0.24)", color: "var(--yunque-primary)" };
  if (tone === "warning") return { background: "rgba(245,158,11,0.12)", borderColor: "rgba(245,158,11,0.28)", color: "var(--yunque-warning)" };
  return { background: "rgba(239,68,68,0.10)", borderColor: "rgba(239,68,68,0.26)", color: "var(--yunque-danger)" };
}

function displayDeliveryLabel(label?: string, level?: string): string {
  const value = label || level || "";
  if (value === "待补肉" || value === "needs_meat") return "需打磨";
  return value || "未知";
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

function sourceName(url: string): string {
  try {
    return new URL(url).host;
  } catch {
    return url;
  }
}

function studioHrefForPack(params: {
  manifest: PackManifest;
  goal: string;
  catalogEntry?: PackCatalogEntry;
  releaseEntry?: PackReleaseCatalogEntry;
}): string {
  const query = new URLSearchParams({
    packId: params.manifest.id,
    goal: params.goal,
  });
  const catalogSHA = typeof params.catalogEntry?.sha256 === "string" ? params.catalogEntry.sha256 : undefined;
  const packageUrl = params.catalogEntry?.package_url || params.releaseEntry?.package_url;
  const sha256 = catalogSHA || params.releaseEntry?.sha256;
  if (packageUrl) query.set("packageUrl", packageUrl);
  if (sha256) query.set("sha256", sha256);
  return `/packs/studio?${query.toString()}`;
}

function packCenterFocusHref(packId: string): string {
  return `/packs?q=${encodeURIComponent(packId)}`;
}

function installSourceForPack(params: {
  manifest: PackManifest;
  catalogEntry?: PackCatalogEntry;
  releaseEntry?: PackReleaseCatalogEntry;
}): { label: string; url?: string; sha256?: string; sizeBytes?: number } | null {
  const catalogSHA = typeof params.catalogEntry?.sha256 === "string" ? params.catalogEntry.sha256 : undefined;
  if (params.releaseEntry) {
    return {
      label: `官方发布源 · ${sourceName(params.releaseEntry.release_url)}`,
      url: params.releaseEntry.package_url,
      sha256: params.releaseEntry.sha256,
      sizeBytes: params.releaseEntry.size_bytes,
    };
  }
  if (params.catalogEntry) {
    const url = params.catalogEntry.package_url || params.catalogEntry.manifest_url || params.catalogEntry.manifest_path || params.catalogEntry.source;
    return {
      label: params.catalogEntry.source ? `私有源 · ${sourceName(params.catalogEntry.source)}` : "私有源",
      url,
      sha256: catalogSHA,
    };
  }
  if (params.manifest.distribution?.packageUrl) {
    return {
      label: "Manifest 分发源",
      url: params.manifest.distribution.packageUrl,
      sha256: params.manifest.distribution.sha256,
      sizeBytes: params.manifest.distribution.sizeBytes,
    };
  }
  return null;
}

type NextStep = {
  key: string;
  label: string;
  detail: string;
  href?: string;
  actionLabel?: string;
  action?: () => void;
  disabled?: boolean;
};

type ActionNotice = {
  title: string;
  detail: string;
  href?: string;
  actionLabel?: string;
  secondaryHref?: string;
  secondaryActionLabel?: string;
  inlineActionLabel?: string;
  inlineAction?: () => void;
};

export default function PackDetailClientPage() {
  const searchParams = useSearchParams();
  const router = useRouter();
  const id = searchParams.get("id") || "";
  const [state, setState] = useState<DetailState | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState<string | null>(null);
  const [showAdvanced, setShowAdvanced] = useState(false);
  const [actionNotice, setActionNotice] = useState<ActionNotice | null>(null);

  const reload = async () => {
    if (!id) {
      setLoading(false);
      setError("缺少 Pack ID");
      return;
    }
    try {
      setLoading(true);
      const [installedRes, catalogRes, releaseRes] = await Promise.all([
        packsClient.installed(),
        packsClient.catalog(),
        packsClient.releaseCatalog(PACK_RELEASE_SOURCES.map((source) => source.url)),
      ]);
      const installedPack = installedRes.packs.find((p) => p.manifest.id === id);
      const catalogEntry = catalogRes.entries.find((e) => e.manifest.id === id);
      const releaseEntry = releaseRes.entries.find((e) => e.manifest.id === id);
      const manifest = installedPack?.manifest || catalogEntry?.manifest || releaseEntry?.manifest;
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
        releaseEntry,
      });
      setError(null);
    } catch (e) {
      setError(formatErrorMessage(e, "加载失败"));
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    setActionNotice(null);
    void reload();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [id]);

  const run = async (label: string, op: () => Promise<unknown>, notice?: ActionNotice) => {
    setBusy(label);
    try {
      await op();
      if (notice) setActionNotice(notice);
      showToast(notice?.title || "操作成功", "success");
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

  const { manifest, installed, enabled, installedPack, catalogEntry, releaseEntry } = state;
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
  const permissionSummary = packPermissionSummary(manifest);
  const featureFlags = packFeatureFlags(manifest);
  const surfaceLabels = capabilitySurfaceLabels(manifest);
  const usageExplanation = packUsageExplanation(manifest);
  const verificationSteps = packVerificationSteps(manifest);
  const usability = packUsability(manifest);
  const readiness = packReadiness(manifest);
  const delivery = packDeliveryProfile(manifest);
  const deliveryLabel = displayDeliveryLabel(delivery.label, delivery.level);
  const deliveryStyle = deliveryToneStyle(delivery.tone);
  const openPath = usability.primaryActionPath || menus[0]?.path || routesFrontend[0]?.path;
  const installSource = installSourceForPack({ manifest, catalogEntry, releaseEntry });
  const installChecklist = packInstallChecklist(manifest, {
    sourceLabel: installSource?.label,
    installed,
    enabled,
  });
  const usagePrompt = [
    `我正在查看云雀能力包：${manifest.name} (${manifest.id})。`,
    "请用普通用户能理解的话告诉我：它能做什么、适合哪些任务、第一句话该怎么说、启用前要注意什么。",
    `能力包说明：${manifest.description || "暂无说明"}`,
    `可用性：${usability.label}；${usability.description}`,
    `交付状态：${deliveryLabel}；${delivery.description}`,
    `建议下一步：${delivery.nextStep}`,
    readiness.missing.length > 0 ? `当前还缺：${readiness.missing.join("、")}` : "当前体检：说明基本完整",
    usability.limitation ? `当前限制：${usability.limitation}` : "",
    examples.length > 0 ? `已有示例：${examples.join(" / ")}` : "",
    "如果它只是后台支撑能力，请告诉我应该从 Chat、任务、记忆、知识或能力包详情哪里感知它，不要把实验能力说成稳定能力。",
  ].filter(Boolean).join("\n");
  const studioGoal = delivery.level === "plan_only"
    ? `把 ${manifest.name} 从实验/计划能力打磨到用户能验证的路径：明确当前不执行什么、结果在哪里看、如何验证和回滚。`
    : delivery.level === "needs_meat"
      ? `让 ${manifest.name} 更像一个用户能直接理解和使用的能力包，优先补齐 ${readiness.missing.join("、") || "用途、入口、示例、权限边界和回滚说明"}。`
      : `让 ${manifest.name} 更像一个用户能直接理解和使用的能力包，补齐用途、入口、示例、权限边界和回滚说明。`;
  const studioHref = studioHrefForPack({ manifest, goal: studioGoal, catalogEntry, releaseEntry });
  const studioHandoffItems = [
    {
      label: "补强目标",
      detail: studioGoal,
    },
    {
      label: "体检缺口",
      detail: readiness.missing.length > 0
        ? readiness.missing.join("、")
        : "说明、入口、示例与能力边界已基本完整，可继续打磨更具体的用户路径。",
    },
    {
      label: "来源与包",
      detail: installSource
        ? `${installSource.label}${installSource.sha256 ? ` · SHA ${installSource.sha256}` : ""}`
        : installed
          ? "本机已安装记录；可在工坊先做只读检查，再准备工作区。"
          : "暂未找到可安装来源；先回能力包中心选择官方源、私有源或本地高级安装。",
    },
    {
      label: "验收路径",
      detail: verificationSteps.map((step) => step.label).join(" -> "),
    },
  ];

  const installFromCatalog = () => {
    const installEntry = catalogEntry || (releaseEntry ? { ...releaseEntry, source: releaseEntry.release_url } : undefined);
    if (!installEntry) {
      showToast("此能力包没有可用的安装源", "error");
      return;
    }
    const request = entryInstallRequest(installEntry);
    if (!request) {
      showToast("此能力包没有可用的安装源", "error");
      return;
    }
    return run("install", () =>
      packsClient.install(request), {
        title: "能力包已安装",
        detail: "下一步先确认权限并启用；也可以回能力包中心聚焦这个包，查看入口、固定侧栏或继续交给小羽打磨。",
        href: packCenterFocusHref(manifest.id),
        actionLabel: "回中心管理",
        inlineActionLabel: "立即启用",
        inlineAction: () => enable(),
      },
    );
  };

  const enable = () => run("enable", () => packsClient.enable(manifest.id), {
    title: "能力包已启用",
    detail: openPath ? "现在可以打开能力入口验证结果；也可以回能力包中心固定侧栏或继续查看权限来源。" : "这个包没有独立入口，启用后会在 Chat、任务、记忆或知识流程中被云雀感知；可回中心确认状态。",
    href: openPath || packCenterFocusHref(manifest.id),
    actionLabel: openPath ? (usability.primaryActionLabel || "打开能力入口") : "回中心管理",
    secondaryHref: openPath ? packCenterFocusHref(manifest.id) : undefined,
    secondaryActionLabel: "回中心管理",
  });
  const disable = () => run("disable", () => packsClient.disable(manifest.id), {
    title: "能力包已禁用",
    detail: "云雀不会再把它纳入可用能力；你可以回中心确认状态，或稍后重新启用。",
    href: packCenterFocusHref(manifest.id),
    actionLabel: "回中心确认",
  });
  const rollback = () => run("rollback", () => packsClient.rollback(manifest.id), {
    title: "能力包已回滚",
    detail: openPath ? "建议重新打开入口验证结果、权限和产物是否回到预期；如果仍有问题，回中心继续禁用或交给小羽检查。" : "建议回中心确认版本与状态；如果仍有问题，继续禁用或交给小羽检查。",
    href: openPath || packCenterFocusHref(manifest.id),
    actionLabel: openPath ? "打开入口复验" : "回中心确认",
    secondaryHref: openPath ? packCenterFocusHref(manifest.id) : undefined,
    secondaryActionLabel: "回中心排查",
  });
  const nextSteps: NextStep[] = !installed
    ? [
        {
          key: "inspect",
          label: "先做只读检查",
          detail: installSource
            ? "在 Studio 里检查 yqpack 来源、SHA、能力声明、权限和入口，不会安装或改写本机能力。"
            : "先确认这个能力包的能力声明、权限和入口是否完整；没有可用安装源时不要直接安装。",
          href: studioHref,
        },
        {
          key: "install",
          label: "安装能力包",
          detail: installSource
            ? "确认来源可信后再安装；失败时会提示下载失败、SHA 不匹配、签名失败、能力声明不合法或平台不支持。"
            : "当前没有可用安装源，请回能力包中心换官方源、私有源或本地高级安装。",
          actionLabel: "安装能力包",
          action: installSource ? installFromCatalog : undefined,
          disabled: !installSource || busy === "install",
        },
        {
          key: "after-install",
          label: "安装后启用和管理",
          detail: "安装成功后状态会刷新；可以留在详情页启用，也可以回能力包中心并自动聚焦这个包。",
          href: packCenterFocusHref(manifest.id),
        },
      ]
    : !enabled
      ? [
          {
            key: "review",
            label: "确认权限与风险",
            detail: "先看清它会读写什么、是否联网、是否涉及浏览器/电脑使用，以及能否禁用或回滚。",
          },
          {
            key: "enable",
            label: "启用能力包",
            detail: "启用后云雀才会把它纳入可用能力；高风险能力仍需要具体动作授权。",
            actionLabel: "启用能力包",
            action: enable,
            disabled: busy === "enable",
          },
          {
            key: "open-later",
            label: "启用后打开入口",
            detail: openPath ? "如果它有界面入口，启用后可以直接打开；没有入口的包会在 Chat、任务、记忆或知识流程中被感知。" : "这个包没有独立界面入口，启用后主要由 Chat、任务、记忆或知识流程感知。",
            href: openPath || packCenterFocusHref(manifest.id),
          },
        ]
      : [
          {
            key: "open",
            label: openPath ? "打开能力入口" : "回能力包中心管理",
            detail: openPath ? "进入它的界面开始使用；没有界面的后台能力，也可以从 Chat 或任务里自然调用。" : "这个包没有独立界面入口，可以回中心查看状态、禁用或继续管理。",
            href: openPath || packCenterFocusHref(manifest.id),
          },
          {
            key: "manage",
            label: "回中心管理和固定",
            detail: "回能力包中心查看启用状态、入口提示、侧栏固定建议和可回滚信息。",
            href: packCenterFocusHref(manifest.id),
          },
          {
            key: "improve",
            label: "交给小羽打磨",
            detail: "如果用户觉得它像空壳，可以让小羽只读检查 yqpack，再打磨用途、示例、权限边界和入口说明。",
            href: studioHref,
          },
        ];

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
          <Chip size="sm" style={{ background: deliveryStyle.background, color: deliveryStyle.color }}>
            {deliveryLabel}
          </Chip>
          <Chip size="sm" variant="soft">v{manifest.version}</Chip>
          <Chip size="sm" style={{ background: "rgba(59,130,246,0.08)", color: "var(--yunque-primary)" }}>
            {usability.label}
          </Chip>
          <Chip size="sm" variant="soft" className="font-mono">{manifest.id}</Chip>
        </div>

        {/* 主操作区 */}
        <div className="flex items-center gap-2 mt-4">
          {!installed && (catalogEntry || releaseEntry) && (
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
        {actionNotice && (
          <Card className="section-card p-4" style={{ borderColor: "rgba(34,197,94,0.22)", background: "rgba(34,197,94,0.07)" }}>
            <div className="flex flex-wrap items-start justify-between gap-3">
              <div>
                <div className="flex items-center gap-2 text-sm font-semibold" style={{ color: "var(--yunque-text)" }}>
                  <PackageCheck size={16} style={{ color: "var(--yunque-success)" }} />
                  {actionNotice.title}
                </div>
                <div className="mt-1 text-xs leading-5" style={{ color: "var(--yunque-text-secondary)" }}>
                  {actionNotice.detail}
                </div>
              </div>
              <div className="flex flex-wrap gap-2">
                {actionNotice.inlineAction && (
                  <Button size="sm" className="btn-accent" onPress={actionNotice.inlineAction} isDisabled={busy === "enable"}>
                    <Power size={14} /> {actionNotice.inlineActionLabel || "继续"}
                  </Button>
                )}
                {actionNotice.href && (
                  <Link href={actionNotice.href}>
                    <Button size="sm" variant="outline">
                      {actionNotice.href.startsWith("/packs/") && actionNotice.href !== packCenterFocusHref(manifest.id) ? <ExternalLink size={14} /> : <ArrowRight size={14} />}
                      {actionNotice.actionLabel || "继续"}
                    </Button>
                  </Link>
                )}
                {actionNotice.secondaryHref && (
                  <Link href={actionNotice.secondaryHref}>
                    <Button size="sm" variant="ghost">
                      <ArrowRight size={14} />
                      {actionNotice.secondaryActionLabel || "回中心"}
                    </Button>
                  </Link>
                )}
              </div>
            </div>
          </Card>
        )}

        <Card className="section-card p-4">
          <div className="flex items-center gap-2 mb-3">
            <ArrowRight size={16} style={{ color: "var(--yunque-accent)" }} />
            <div className="text-sm font-semibold" style={{ color: "var(--yunque-text)" }}>
              从这里继续
            </div>
          </div>
          <div className="grid grid-cols-1 md:grid-cols-3 gap-3">
            {nextSteps.map((step, idx) => (
              <div
                key={step.key}
                className="rounded-md border p-3 flex flex-col gap-2"
                style={{ borderColor: "var(--yunque-border)", background: "var(--yunque-bg-hover)" }}
              >
                <div className="flex items-center gap-2">
                  <span
                    className="inline-flex h-5 w-5 items-center justify-center rounded-full text-[11px] font-semibold"
                    style={{ background: "rgba(59,130,246,0.12)", color: "var(--yunque-primary)" }}
                  >
                    {idx + 1}
                  </span>
                  <div className="text-xs font-semibold" style={{ color: "var(--yunque-text)" }}>{step.label}</div>
                </div>
                <div className="text-xs leading-5 flex-1" style={{ color: "var(--yunque-text-secondary)" }}>{step.detail}</div>
                {step.href && (
                  <Link href={step.href}>
                    <Button size="sm" variant={idx === 0 && !enabled ? "outline" : "ghost"}>
                      {step.key === "improve" ? <Sparkles size={14} /> : step.key === "open" ? <ExternalLink size={14} /> : <ArrowRight size={14} />}
                      {step.label}
                    </Button>
                  </Link>
                )}
                {step.action && step.actionLabel && (
                  <Button size="sm" className="btn-accent" isDisabled={step.disabled} onPress={step.action}>
                    {step.key === "install" ? <Download size={14} /> : <Power size={14} />}
                    {step.actionLabel}
                  </Button>
                )}
              </div>
            ))}
          </div>
        </Card>

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
          <div className="mt-3 rounded-md border p-3" style={{ borderColor: deliveryStyle.borderColor, background: deliveryStyle.background }}>
            <div className="mb-1 text-xs font-medium" style={{ color: deliveryStyle.color }}>
              交付状态：{deliveryLabel}
            </div>
            <div className="text-xs leading-5" style={{ color: "var(--yunque-text-secondary)" }}>
              {delivery.description}
            </div>
            <div className="mt-1 text-xs leading-5" style={{ color: "var(--yunque-text-muted)" }}>
              下一步：{delivery.nextStep}
            </div>
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
          <div className="mt-3 flex flex-wrap gap-2">
            <Link href={chatPromptHref(usagePrompt)}>
              <Button size="sm" variant="outline">
                <MessageSquare size={14} /> 问云雀怎么用
              </Button>
            </Link>
            <Link href={studioHref}>
              <Button size="sm" variant="ghost">
                <Sparkles size={14} /> 交给小羽打磨
              </Button>
            </Link>
          </div>
        </Card>

        <Card className="section-card p-4">
          <div className="flex items-center gap-2 mb-3">
            <Sparkles size={16} style={{ color: "var(--yunque-accent)" }} />
            <div className="text-sm font-semibold" style={{ color: "var(--yunque-text)" }}>
              交接给小羽改包
            </div>
          </div>
          <div className="text-xs leading-5" style={{ color: "var(--yunque-text-muted)" }}>
            从详情进入工坊时，会带上当前能力包、来源、体检缺口和验收路径；小羽只生成计划和草稿，真正安装、启用、回滚都需要你确认。
          </div>
          <div className="mt-3 grid gap-2 md:grid-cols-2">
            {studioHandoffItems.map((item) => (
              <div key={item.label} className="rounded-md border p-3" style={{ borderColor: "var(--yunque-border)", background: "var(--yunque-bg-hover)" }}>
                <div className="text-xs font-medium" style={{ color: "var(--yunque-text)" }}>{item.label}</div>
                <div className="mt-1 text-[11px] leading-5 break-words" style={{ color: "var(--yunque-text-secondary)" }}>{item.detail}</div>
              </div>
            ))}
          </div>
          <div className="mt-3 flex flex-wrap gap-2">
            <Link href={studioHref}>
              <Button size="sm" className="btn-accent">
                <Sparkles size={14} /> 带上下文进入工坊
              </Button>
            </Link>
            <Link href={packCenterFocusHref(manifest.id)}>
              <Button size="sm" variant="ghost">
                回中心看队列 <ArrowRight size={14} />
              </Button>
            </Link>
          </div>
        </Card>

        <Card className="section-card p-4">
          <div className="flex items-center gap-2 mb-3">
            <Workflow size={16} style={{ color: "var(--yunque-primary)" }} />
            <div className="text-sm font-semibold" style={{ color: "var(--yunque-text)" }}>
              怎么验证它有用
            </div>
          </div>
          <div className="mb-3 rounded-md border px-3 py-2 text-xs leading-5" style={{ borderColor: "rgba(59,130,246,0.22)", background: "rgba(59,130,246,0.07)", color: "var(--yunque-text-secondary)" }}>
            <span className="font-semibold" style={{ color: "var(--yunque-text)" }}>验收出口：</span>
            回中心确认状态，进详情复查权限{openPath ? "，再打开入口复验。" : "；这个包没有独立入口，需从 Chat、任务、记忆或知识流程触发并观察结果。"}
          </div>
          <div className="grid grid-cols-1 md:grid-cols-3 gap-3">
            {verificationSteps.map((step, idx) => (
              <div
                key={step.key}
                className="rounded-md border p-3"
                style={{ borderColor: "var(--yunque-border)", background: "var(--yunque-bg-hover)" }}
              >
                <div className="flex items-center gap-2 mb-2">
                  <span
                    className="inline-flex h-5 w-5 items-center justify-center rounded-full text-[11px] font-semibold"
                    style={{ background: "rgba(59,130,246,0.12)", color: "var(--yunque-primary)" }}
                  >
                    {idx + 1}
                  </span>
                  <div className="text-xs font-semibold" style={{ color: "var(--yunque-text)" }}>{step.label}</div>
                </div>
                <div className="text-xs leading-5" style={{ color: "var(--yunque-text-secondary)" }}>{step.detail}</div>
                {step.href && (
                  <Link href={step.href} className="mt-2 inline-flex items-center gap-1 text-xs font-medium" style={{ color: "var(--yunque-accent)" }}>
                    打开验证 <ArrowRight size={12} />
                  </Link>
                )}
              </div>
            ))}
          </div>
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
              <div>还缺：{readiness.missing.join("、")}。可以让小羽打磨用途、入口、示例或边界说明。</div>
              <Link href={studioHref} className="mt-2 inline-flex items-center gap-1 font-medium" style={{ color: "var(--yunque-accent)" }}>
                交给小羽打磨 <Sparkles size={12} />
              </Link>
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

        {installSource && (
          <Card className="section-card p-4">
            <div className="flex items-center gap-2 mb-3">
              <Download size={16} style={{ color: "var(--yunque-primary)" }} />
              <div className="text-sm font-semibold" style={{ color: "var(--yunque-text)" }}>
                来源与安装包
              </div>
            </div>
            <div className="text-xs mb-2" style={{ color: "var(--yunque-text-secondary)" }}>
              {installSource.label}
            </div>
            {installSource.url && (
              <div className="font-mono break-all text-[11px]" style={{ color: "var(--yunque-text-muted)" }}>
                {installSource.url}
              </div>
            )}
            <div className="mt-2 flex flex-wrap gap-2">
              {installSource.sha256 && <Chip size="sm" variant="soft">SHA256 {installSource.sha256}</Chip>}
              {installSource.sizeBytes && <Chip size="sm" variant="soft">{formatBytes(installSource.sizeBytes)}</Chip>}
            </div>
            <div className="mt-3 flex flex-wrap gap-2">
              <Link href={studioHref}>
                <Button size="sm" variant="outline">
                  <Sparkles size={14} /> 先在工坊只读检查
                </Button>
              </Link>
              <Link href={packCenterFocusHref(manifest.id)}>
                <Button size="sm" variant="ghost">
                  回能力包中心 <ArrowRight size={14} />
                </Button>
              </Link>
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
              <div className="text-xs mt-2 font-medium" style={{ color: risk.requiresAuthorization ? "var(--yunque-warning)" : "var(--yunque-text-secondary)" }}>
                {permissionSummary}
              </div>
              <div className="text-xs mt-2" style={{ color: "var(--yunque-text-muted)" }}>{risk.description}</div>
              <div className="text-xs mt-2" style={{ color: "var(--yunque-text-muted)" }}>
                不会做什么：启用能力包不会自动泄露 API Key，不会绕过云雀的权限声明，也不会获得未声明后端路由的调用能力。
              </div>
              <div className="text-xs mt-2" style={{ color: "var(--yunque-text-muted)" }}>
                {update?.rollback ? "支持回滚到上一版本；也可以随时禁用。" : "可以随时禁用；此包未声明版本回滚。"}
              </div>
            </div>
          </div>
          <div className="mt-3 grid gap-2 md:grid-cols-2 xl:grid-cols-4">
            {installChecklist.map((item) => (
              <div
                key={item.key}
                className="rounded-md border p-3 text-xs"
                style={{
                  borderColor: item.tone === "danger" ? "rgba(239,68,68,0.22)" : item.tone === "warning" ? "rgba(245,158,11,0.22)" : "var(--yunque-border)",
                  background: item.tone === "danger" ? "rgba(239,68,68,0.08)" : item.tone === "warning" ? "rgba(245,158,11,0.08)" : "var(--yunque-bg-hover)",
                }}
              >
                <div className="mb-1 font-medium" style={{ color: "var(--yunque-text)" }}>{item.label}</div>
                <div className="leading-5" style={{ color: "var(--yunque-text-secondary)" }}>{item.detail}</div>
              </div>
            ))}
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
                独立界面包运行在沙箱隔离环境中：不直接获得云雀 token，默认隔离页面能力，只能通过自身声明的后端路由与云雀通信，越权调用会被拒绝并记录。
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
