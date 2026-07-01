"use client";

import { useCallback, useEffect, useMemo, useState, type ReactNode } from "react";
import Link from "next/link";
import { useSearchParams } from "next/navigation";
import { Button, Card, Checkbox, Chip, Disclosure, Input, Label, Modal, Spinner, TextField } from "@heroui/react";
import { Segment, ActionBar } from "@heroui-pro/react";
import {
  ArrowRight,
  Boxes,
  ChevronDown,
  ChevronUp,
  Copy,
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
import { createPacksClient, type InstalledPack, type PackBackendRouteAuditEntry, type PackCatalogEntry, type PackManifest, type PackReleaseCatalogEntry } from "yunque-client/packs";
import { createYunqueSDKClientOptions } from "@/lib/sdk-client";
import { useApiData } from "@/lib/use-api-data";
import { buildPackNavItems } from "@/lib/pack-sync";
import { resolvePackReleaseSources } from "@/lib/pack-release-sources";
import { useNavigationPreferences } from "@/hooks/use-user-preferences";
import {
  capabilitySurfaceLabels,
  catalogActionForEntry,
  entryInstallRequest,
  formatPackInstallError,
  groupPackPermissions,
  packDeliveryProfile,
  packGroupKey,
  packGroupLabel,
  PACK_GROUP_UNGROUPED,
  packInstallChecklist,
  packInstallTroubleshooting,
  packExamples,
  packFeatureFlags,
  packManifestAudit,
  packPermissionSummary,
  packReadiness,
  packSafeOpenPath,
  packUsageExplanation,
  packUsability,
  packVerificationSteps,
  riskProfileForPack,
} from "@/lib/pack-presentation";

const PACK_RELEASE_SOURCES = resolvePackReleaseSources();
const OFFICIAL_BACKUP_MANIFEST = "packs/official/backup-pack/pack.json";
const packsClient = createPacksClient(createYunqueSDKClientOptions());
const PAGE_SIZE = 12;
const READINESS_QUEUE_PAGE_SIZE = 6;

type KindFilter = "all" | "actionable" | "infrastructure" | "experimental";
type InstallFilter = "all" | "installed" | "enabled" | "disabled" | "available";
type RiskFilter = "all" | "low" | "medium" | "high";
type SourceFilter = "all" | "installed" | "official" | "private";
type StabilityFilter = "all" | "stable" | "beta" | "alpha";
type ReadinessFilter = "all" | "complete" | "needs_context" | "needs_entry";
type SortMode = "name" | "kind" | "risk" | "readiness" | "status";
type ReadinessQueueItem = {
  manifest: PackManifest;
  sourceLabel: string;
  packageUrl?: string;
  sha256?: string;
};
type PackPolishGuidance = {
  reason: string;
  firstEdit: string;
  verify: string;
  handoff: string;
};
type PackPolishPriority = {
  level: "P0" | "P1" | "P2";
  label: string;
  reason: string;
  order: number;
};
type CenterActionNotice = {
  title: string;
  detail: string;
  href?: string;
  actionLabel?: string;
  inlineActionLabel?: string;
  inlineAction?: () => void;
  packId?: string;
};
type CurrentViewAction = {
  label: string;
  kind: "anchor" | "filter" | "link";
  href?: string;
  onPress?: () => void;
};

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
const STABILITY_FILTER_LABELS: Record<StabilityFilter, string> = {
  all: "全部稳定性",
  stable: "正式版",
  beta: "测试版",
  alpha: "开发中",
};
const READINESS_FILTER_LABELS: Record<ReadinessFilter, string> = {
  all: "全部体检",
  complete: "说明完整",
  needs_context: "需补说明",
  needs_entry: "需补入口",
};
// Group display names + helpers (packGroupKey / packGroupLabel / PACK_GROUP_UNGROUPED)
// are shared with the sidebar flyout — imported from pack-presentation above.
const SORT_MODE_LABELS: Record<SortMode, string> = {
  name: "按名称",
  kind: "按类型",
  risk: "按风险",
  readiness: "按体检",
  status: "按阶段",
};

const INSTALL_TROUBLESHOOTING = packInstallTroubleshooting();

// Tone styles resolve to the app's semantic *-muted tokens (which track the
// active theme / accent palette) instead of hardcoded rgba literals. Borders
// are intentionally dropped to a single subtle neutral edge per the Pro
// "minimize borders" principle — the soft tinted background already carries
// the semantic meaning.
function deliveryToneStyle(tone: ReturnType<typeof packDeliveryProfile>["tone"]): { background: string; borderColor: string; color: string } {
  if (tone === "danger") {
    return { background: "var(--yunque-danger-muted)", borderColor: "transparent", color: "var(--yunque-danger)" };
  }
  if (tone === "warning") {
    return { background: "var(--yunque-warning-muted)", borderColor: "transparent", color: "var(--yunque-warning)" };
  }
  if (tone === "primary") {
    return { background: "var(--yunque-accent-muted)", borderColor: "transparent", color: "var(--yunque-accent)" };
  }
  return { background: "var(--yunque-success-muted)", borderColor: "transparent", color: "var(--yunque-success)" };
}

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

// `chip` is the HeroUI Chip semantic color so call sites can render
// <Chip color={tone.chip} variant="soft"> instead of inline-styling bg/color.
type ChipColor = "success" | "warning" | "danger" | "accent" | "default";

function statusTone(status: string): { label: string; color: string; bg: string; chip: ChipColor } {
  if (status === "enabled") return { label: "已启用", color: "var(--yunque-success)", bg: "var(--yunque-success-muted)", chip: "success" };
  if (status === "disabled") return { label: "已禁用", color: "var(--yunque-text-muted)", bg: "var(--yunque-bg-muted)", chip: "default" };
  return { label: status || "未知", color: "var(--yunque-warning)", bg: "var(--yunque-warning-muted)", chip: "warning" };
}

function packStatusBadge(packStatus?: string): { label: string; color: string; bg: string; chip: ChipColor } {
  if (packStatus === "stable") return { label: "正式版", color: "var(--yunque-success)", bg: "var(--yunque-success-muted)", chip: "success" };
  if (packStatus === "beta") return { label: "测试版", color: "var(--yunque-warning)", bg: "var(--yunque-warning-muted)", chip: "warning" };
  if (packStatus === "alpha") return { label: "开发中", color: "var(--yunque-text-muted)", bg: "var(--yunque-bg-muted)", chip: "default" };
  return { label: "未知", color: "var(--yunque-text-muted)", bg: "var(--yunque-bg-muted)", chip: "default" };
}

function shouldShowPackStatusBadge(packStatus: string | undefined, advancedVisible: boolean): boolean {
  return advancedVisible || (packStatus !== "stable" && packStatus !== "beta");
}

type TrustTone = "safe" | "neutral" | "warning" | "danger" | "accent";

function trustToneStyle(tone: TrustTone): { borderColor: string; background: string; color: string } {
  if (tone === "danger") return { borderColor: "transparent", background: "var(--yunque-danger-muted)", color: "var(--yunque-danger)" };
  if (tone === "warning") return { borderColor: "transparent", background: "var(--yunque-warning-muted)", color: "var(--yunque-warning)" };
  if (tone === "accent") return { borderColor: "transparent", background: "var(--yunque-accent-muted)", color: "var(--yunque-accent)" };
  if (tone === "safe") return { borderColor: "transparent", background: "var(--yunque-success-muted)", color: "var(--yunque-success)" };
  return { borderColor: "transparent", background: "var(--yunque-bg-muted)", color: "var(--yunque-text-muted)" };
}

function deliveryToneToTrustTone(tone: ReturnType<typeof packDeliveryProfile>["tone"]): TrustTone {
  if (tone === "danger") return "danger";
  if (tone === "warning") return "warning";
  if (tone === "primary") return "accent";
  return "safe";
}

function sourceTrustHint(source: string): string {
  if (source.includes("私有源")) return "先验 SHA/权限";
  if (source.includes("官方源")) return "安装前只读检查";
  if (source.includes("已安装")) return "可禁用/回滚";
  return "先确认来源";
}

function PackTrustStrip({
  source,
  showSourceFact = true,
  showSourceHint = false,
  runtime,
  runtimeTone,
  risk,
  delivery,
  readiness,
}: {
  source: string;
  showSourceFact?: boolean;
  showSourceHint?: boolean;
  runtime: string;
  runtimeTone: TrustTone;
  risk: ReturnType<typeof riskProfileForPack>;
  delivery: ReturnType<typeof packDeliveryProfile>;
  readiness: ReturnType<typeof packReadiness>;
}) {
  const riskTone: TrustTone = risk.level === "high" ? "danger" : risk.level === "medium" ? "warning" : "safe";
  const deliveryTone = deliveryToneToTrustTone(delivery.tone);
  const facts = [
    ...(showSourceFact
      ? [{
        key: "source",
        label: "验源",
        value: source,
        hint: showSourceHint ? sourceTrustHint(source) : "",
        tone: "neutral" as const,
        icon: <Store size={13} aria-hidden />,
      }]
      : []),
    { key: "runtime", label: "运行", value: runtime, hint: "", tone: runtimeTone, icon: <Power size={13} aria-hidden /> },
    { key: "trust", label: "信任", value: risk.label, hint: "", tone: riskTone, icon: risk.requiresAuthorization ? <ShieldAlert size={13} aria-hidden /> : <ShieldCheck size={13} aria-hidden /> },
    {
      key: "delivery",
      label: "交付",
      value: readiness.level === "complete" ? delivery.label : `${delivery.label} · ${readiness.label}`,
      hint: "",
      tone: deliveryTone,
      icon: <PackageCheck size={13} aria-hidden />,
    },
  ];
  return (
    <div className="mt-3 flex flex-wrap items-center gap-x-4 gap-y-1.5 text-xs" aria-label="能力包状态摘要">
      {facts.map((fact) => {
        const style = trustToneStyle(fact.tone);
        return (
          <span key={fact.key} className="inline-flex items-center gap-1.5 min-w-0">
            <span style={{ color: style.color }} aria-hidden>{fact.icon}</span>
            <span style={{ color: "var(--yunque-text-muted)" }}>{fact.label}</span>
            <span className="truncate" title={fact.value} style={{ color: "var(--yunque-text)" }}>
              {fact.value}
            </span>
            {fact.hint && (
              <span className="truncate text-[11px]" title={fact.hint} style={{ color: "var(--yunque-text-muted)" }}>
                {fact.hint}
              </span>
            )}
          </span>
        );
      })}
    </div>
  );
}

function checklistToneStyle(tone: "safe" | "warning" | "danger"): { borderColor: string; background: string; color: string } {
  if (tone === "danger") {
    return {
      borderColor: "transparent",
      background: "var(--yunque-danger-muted)",
      color: "var(--yunque-danger)",
    };
  }
  if (tone === "warning") {
    return {
      borderColor: "transparent",
      background: "var(--yunque-warning-muted)",
      color: "var(--yunque-warning)",
    };
  }
  return {
    borderColor: "transparent",
    background: "var(--yunque-success-muted)",
    color: "var(--yunque-success)",
  };
}

function sourceName(url: string): string {
  try {
    return new URL(url).host;
  } catch {
    return url;
  }
}

function describePackEntry(label?: string, path?: string): string {
  if (!path) return "没有独立页面，会在对话、任务、记忆或知识流程中自动生效。";
  try {
    const url = new URL(path, "http://yunque.local");
    const q = url.searchParams.get("q")?.trim();
    if (url.pathname === "/chat" && q) {
      return `${label || "去 Chat"} · ${q}`;
    }
  } catch {
    // Keep the raw path below when it is not URL-like.
  }
  return `${label || "打开入口"} · ${path}`;
}

function releaseSourceLabel(entry: PackReleaseCatalogEntry): string {
  const configured = PACK_RELEASE_SOURCES.find((source) => source.url === entry.release_url);
  return configured?.label || `官方源 · ${sourceName(entry.release_url)}`;
}

function privateSourceLabel(entry: PackCatalogEntry): string {
  const source = [entry.source, entry.manifest_url, entry.manifest_path, entry.package_url]
    .find((value): value is string => typeof value === "string" && value.trim().length > 0);
  return source ? `私有源 · ${sourceName(source)}` : "私有源";
}

function packStudioHref(manifest: PackManifest, options?: { packageUrl?: string; sha256?: string }): string {
  const readiness = packReadiness(manifest);
  const delivery = packDeliveryProfile(manifest);
  const gap = readiness.missing.length > 0
    ? `重点打磨：${readiness.missing.join("、")}。`
    : delivery.level === "plan_only"
      ? "重点把实验/计划边界讲清楚：不要伪装成稳定执行能力，补全真实结果、限制、验证和回滚说明。"
    : "继续打磨更具体的用户场景和入口反馈。";
  const goal = `让 ${manifest.name} 更像一个用户能直接理解和使用的能力包，${gap}打磨用途、入口、权限说明和可回滚改造建议。`;
  const params = new URLSearchParams({
    packId: manifest.id,
    goal,
  });
  if (options?.packageUrl) params.set("packageUrl", options.packageUrl);
  if (options?.sha256) params.set("sha256", options.sha256);
  return `/packs/studio?${params.toString()}`;
}

function packCenterFocusHref(packId?: string): string {
  return packId ? `/packs?q=${encodeURIComponent(packId)}&from=studio` : "/packs?from=studio";
}

function packPolishGuidance(
  manifest: PackManifest,
  routeAuditEntries: readonly PackBackendRouteAuditEntry[] = [],
): PackPolishGuidance {
  const readiness = packReadiness(manifest);
  const delivery = packDeliveryProfile(manifest);
  const usability = packUsability(manifest);
  const audit = packManifestAudit(manifest, routeAuditEntries);
  const primaryPath = usability.primaryActionPath || manifest.frontend?.menus?.[0]?.path || manifest.frontend?.routes?.[0]?.path;
  const missing = readiness.missing;
  const reason = audit.issues.length > 0
    ? `Manifest 审计：${audit.issues.map((issue) => issue.label).join("、")}。`
    : missing.length > 0
    ? `体检缺口：${missing.join("、")}。`
    : `交付状态：${delivery.label}。${delivery.description}`;
  let firstEdit = "先补能力声明里的用途、入口、示例、限制和回滚说明。";

  if (audit.issues.some((issue) => issue.key === "static-pack-route")) {
    firstEdit = "先修打开入口：补对应静态页面、改到详情页，或改成 Chat/任务入口，避免用户点开 404。";
  } else if (audit.issues.some((issue) => issue.key === "missing-route-specs" || issue.key === "iframe-without-whitelist")) {
    firstEdit = "先补 backend.routeSpecs，把 method/path/description 白名单写清楚，再跑 route audit 和 bridge 验收。";
  } else if (audit.issues.some((issue) => issue.key === "capability-without-permission" || issue.key === "permission-without-capability")) {
    firstEdit = "先对齐 capability 与 permissions：让用户既知道云雀能调度什么，也知道它触达哪些资源。";
  } else if (missing.includes("后端能力声明")) {
    firstEdit = "先确认是否真有后端能力：有则补后端路由、权限和测试；没有就明确标为界面/说明型能力，不能伪造执行能力。";
  } else if (missing.includes("打开/使用入口")) {
    firstEdit = "先补主要入口或界面菜单，让用户知道启用后从哪里进入。";
  } else if (missing.includes("用户感知位置")) {
    firstEdit = "先补用户感知位置，说明它会在 Chat、任务、记忆、知识或设置中的哪个位置被看见。";
  } else if (missing.includes("使用示例")) {
    firstEdit = "先补真实使用示例，用用户动作描述它能产出什么结果。";
  } else if (delivery.level === "plan_only") {
    firstEdit = "先保留实验边界，补真实结果位置、当前限制、验证步骤和转稳定的最小待办。";
  } else if (delivery.level === "needs_meat") {
    firstEdit = "先补用途、入口、示例、权限边界和可回滚说明，避免看起来只是空壳。";
  }

  return {
    reason,
    firstEdit,
    verify: primaryPath
      ? `改完回到 ${primaryPath} 验证入口、提示、结果位置和回滚路径是否可见。`
      : "改完回到能力包详情与 Chat/任务主路径验证：用户是否知道怎么触发、结果在哪里、出问题怎么禁用或回滚。",
    handoff: "只读检查 -> 准备工作区 -> 预览差异 -> 审计 -> 重新打包 -> 复检 SHA -> 安装/启用/回滚。",
  };
}

function packPolishPriority(
  manifest: PackManifest,
  routeAuditEntries: readonly PackBackendRouteAuditEntry[] = [],
): PackPolishPriority {
  const readiness = packReadiness(manifest);
  const delivery = packDeliveryProfile(manifest);
  const risk = riskProfileForPack(manifest);
  const audit = packManifestAudit(manifest, routeAuditEntries);
  const missing = readiness.missing;

  if (audit.level === "blocked") {
    return {
      level: "P0",
      label: "P0 先修审计阻塞",
      reason: "入口、routeSpecs 或白名单存在结构性缺口，用户可能点到 404 或无法验收能力门禁。",
      order: 0,
    };
  }
  if (audit.level === "watch") {
    return {
      level: "P1",
      label: "P1 对齐能力边界",
      reason: "Manifest 能力、权限或白名单还不够清楚，需要先复核再继续扩大能力。",
      order: 1,
    };
  }
  if (missing.includes("后端能力声明") || missing.includes("打开/使用入口")) {
    return {
      level: "P0",
      label: "P0 先补可用路径",
      reason: "缺后端能力声明或打开入口，用户很难确认这个能力是否真的可用。",
      order: 0,
    };
  }
  if (risk.requiresAuthorization && delivery.level !== "ready") {
    return {
      level: "P0",
      label: "P0 先补授权边界",
      reason: "涉及高风险授权，但交付路径还不够清楚，启用前必须先讲明边界和回滚。",
      order: 0,
    };
  }
  if (delivery.level === "plan_only") {
    return {
      level: "P1",
      label: "P1 实验转可验证",
      reason: "当前仍是实验/计划能力，需要补真实结果位置、限制和转稳定待办。",
      order: 1,
    };
  }
  if (missing.includes("用户感知位置") || missing.includes("使用示例") || delivery.level === "needs_meat") {
    return {
      level: "P1",
      label: "P1 补用户理解",
      reason: "能力本体存在，但用户还缺少场景、示例或结果位置来判断价值。",
      order: 1,
    };
  }
  return {
    level: "P2",
    label: "P2 继续打磨",
    reason: "核心声明已基本完整，适合补更具体的文案、验收和发布说明。",
    order: 2,
  };
}

function buildBatchReadinessPrompt(
  items: ReadinessQueueItem[],
  batch: { page: number; pageCount: number; total: number; pageSize: number },
  routeAuditEntries: readonly PackBackendRouteAuditEntry[] = [],
): string {
  const request = {
    kind: "yunque.pack_studio.batch_draft_request.v1",
    goal: "批量把这些能力包从“看得到但不知道怎么用”推进到用户能理解、能打开、能验证、能回滚的状态。",
    batch: {
      page: batch.page,
      page_count: batch.pageCount,
      total: batch.total,
      page_size: batch.pageSize,
    },
    rules: [
      "不要自动应用改动。",
      "每个包先给独立改包草稿请求，再回到能力包工坊只读检查、准备工作区、预览差异、运行审计、重新打包和复检 SHA。",
      "缺后端能力声明时，不要伪造能力；如果需要新增 routeSpecs、权限或源码测试，请明确列为待办。",
      "实验/计划型能力不能包装成稳定承诺，高风险权限必须保留授权和回滚说明。",
      "如果交付状态是实验/计划，优先补真实结果位置、限制说明、验证步骤和后续转稳定的最小待办。",
    ],
    packs: items.map((item) => {
      const readiness = packReadiness(item.manifest);
      const delivery = packDeliveryProfile(item.manifest);
      const risk = riskProfileForPack(item.manifest);
      const audit = packManifestAudit(item.manifest, routeAuditEntries);
      const guidance = packPolishGuidance(item.manifest, routeAuditEntries);
      const priority = packPolishPriority(item.manifest, routeAuditEntries);
      const primaryPath = packSafeOpenPath(item.manifest);
      return {
        id: item.manifest.id,
        name: item.manifest.name,
        version: item.manifest.version,
        status: item.manifest.status,
        source: item.sourceLabel,
        priority: {
          level: priority.level,
          label: priority.label,
          reason: priority.reason,
        },
        missing: readiness.missing,
        readiness: readiness.label,
        risk: {
          level: risk.level,
          label: risk.label,
          requires_authorization: risk.requiresAuthorization,
        },
        permission_summary: packPermissionSummary(item.manifest),
        delivery: {
          level: delivery.level,
          label: delivery.label,
          description: delivery.description,
          next_step: delivery.nextStep,
        },
        manifest_audit: {
          level: audit.level,
          label: audit.label,
          issues: audit.issues,
          summary: audit.summary,
        },
        polish_guidance: {
          reason: guidance.reason,
          first_edit: guidance.firstEdit,
          verify: guidance.verify,
          handoff: guidance.handoff,
        },
        handoff_links: {
          center: packCenterFocusHref(item.manifest.id),
          detail: `/packs/detail?id=${encodeURIComponent(item.manifest.id)}`,
          open: primaryPath || null,
          studio: packStudioHref(item.manifest, { packageUrl: item.packageUrl, sha256: item.sha256 }),
        },
        studio_url: packStudioHref(item.manifest, { packageUrl: item.packageUrl, sha256: item.sha256 }),
        package_url: item.packageUrl,
        sha256: item.sha256,
      };
    }),
  };
  return [
    "请以“小羽改包”的方式批量处理下面这些能力包。",
    "目标是先打磨用户可感知的用途、入口、示例、权限边界和回滚说明，而不是直接扩大能力或绕过能力包工坊。",
    "请按优先级逐包输出计划；需要具体改单文件时，只输出 yunque.pack_studio.patch_draft_request.v1 或 patch_draft.v1，并要求用户回到能力包工坊预览差异 / 审计 / 重打包。",
    "每个包完成后必须按 handoff_links 回能力包中心、详情页和入口复验；没有 open 链接的包要说明应从 Chat、任务、记忆或知识流程验收。",
    "",
    "```json",
    JSON.stringify(request, null, 2),
    "```",
  ].join("\n");
}

function buildPackCenterUsabilityReport(
  items: ReadinessQueueItem[],
  queueItems: ReadinessQueueItem[],
  batch: { page: number; pageCount: number; total: number; pageSize: number },
  routeAuditEntries: readonly PackBackendRouteAuditEntry[] = [],
) {
  const groups = { actionable: 0, infrastructure: 0, experimental: 0, documented: 0 };
  const readiness = { complete: 0, needs_context: 0, needs_entry: 0 };
  const delivery = { ready: 0, support: 0, plan_only: 0, needs_meat: 0 };
  const manifest_audit = { clear: 0, watch: 0, blocked: 0, issues: 0 };
  for (const item of items) {
    groups[packUsability(item.manifest).kind] += 1;
    readiness[packReadiness(item.manifest).level] += 1;
    delivery[packDeliveryProfile(item.manifest).level] += 1;
    const audit = packManifestAudit(item.manifest, routeAuditEntries);
    manifest_audit[audit.level] += 1;
    manifest_audit.issues += audit.issues.length;
  }
  const queue = queueItems.map((item) => packCenterReportItem(item, routeAuditEntries));
  return {
    kind: "yunque.pack_usability_report.v1",
    source: "pack-center",
    generated_at: new Date().toISOString(),
    summary: {
      total: items.length,
      groups,
      readiness,
      delivery,
      manifest_audit,
      queue: {
        total: batch.total,
        page: batch.page,
        page_count: batch.pageCount,
        page_size: batch.pageSize,
        p0: queue.filter((item) => item.priority.level === "P0").length,
        p1: queue.filter((item) => item.priority.level === "P1").length,
        p2: queue.filter((item) => item.priority.level === "P2").length,
      },
    },
    queue,
    packs: items.map((item) => packCenterReportItem(item, routeAuditEntries)),
  };
}

function packCenterReportItem(
  item: ReadinessQueueItem,
  routeAuditEntries: readonly PackBackendRouteAuditEntry[] = [],
) {
  const manifest = item.manifest;
  const usability = packUsability(manifest);
  const readiness = packReadiness(manifest);
  const delivery = packDeliveryProfile(manifest);
  const priority = packPolishPriority(manifest, routeAuditEntries);
  const risk = riskProfileForPack(manifest);
  const guidance = packPolishGuidance(manifest, routeAuditEntries);
  const audit = packManifestAudit(manifest, routeAuditEntries);
  const open = packSafeOpenPath(manifest) || "";
  return {
    id: manifest.id,
    name: manifest.name,
    version: manifest.version,
    status: manifest.status || "",
    source: item.sourceLabel,
    package_url: item.packageUrl || "",
    sha256: item.sha256 || "",
    usability: {
      kind: usability.kind,
      label: usability.label,
      description: usability.description,
      limitation: usability.limitation || "",
    },
    readiness: {
      level: readiness.level,
      label: readiness.label,
      missing: readiness.missing,
    },
    delivery: {
      level: delivery.level,
      label: delivery.label,
      description: delivery.description,
      next_step: delivery.nextStep,
    },
    manifest_audit: {
      level: audit.level,
      label: audit.label,
      issues: audit.issues,
      summary: audit.summary,
    },
    priority,
    risk: {
      level: risk.level,
      label: risk.label,
      requires_authorization: risk.requiresAuthorization,
    },
    permission_summary: packPermissionSummary(manifest),
    next_step: guidance.firstEdit,
    verify: guidance.verify,
    handoff_links: {
      center: packCenterFocusHref(manifest.id),
      detail: `/packs/detail?id=${encodeURIComponent(manifest.id)}`,
      open: open || null,
      studio: packStudioHref(manifest, { packageUrl: item.packageUrl, sha256: item.sha256 }),
    },
  };
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

function pageCountFor(total: number, pageSize = PAGE_SIZE): number {
  return Math.max(1, Math.ceil(total / pageSize));
}

function paginate<T>(items: T[], page: number, pageSize = PAGE_SIZE): T[] {
  return items.slice((page - 1) * pageSize, page * pageSize);
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
      <Segment
        size="sm"
        aria-label={label}
        selectedKey={value}
        onSelectionChange={(key) => {
          // ToggleButtonGroup deselects on re-press, yielding an empty key.
          // Filters are single-select with an explicit reset option, so ignore
          // the empty case and keep the current value.
          if (key != null && key !== "") onChange(String(key));
        }}
      >
        {options.map(([key, text]) => (
          <Segment.Item key={key} id={key}>{text}</Segment.Item>
        ))}
      </Segment>
    </div>
  );
}

function FilterInsightCard({ label, value, detail }: { label: string; value: string; detail: string }) {
  return (
    <div className="rounded-md border px-3 py-2" style={{ borderColor: "var(--yunque-border)", background: "var(--yunque-bg-muted)" }}>
      <div className="text-[11px] font-medium" style={{ color: "var(--yunque-text-muted)" }}>{label}</div>
      <div className="mt-1 text-sm font-semibold" style={{ color: "var(--yunque-text)" }}>{value}</div>
      <div className="mt-1 text-[11px] leading-5" style={{ color: "var(--yunque-text-secondary)" }}>{detail}</div>
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
  pageSize = PAGE_SIZE,
) {
  if (total <= pageSize) return null;
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
  const searchParams = useSearchParams();
  const navigationPrefs = useNavigationPreferences();
  const { data, loading, refresh, setData } = useApiData(async () => packsClient.installed(), { packs: [], count: 0 });
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
  const [filtersOpen, setFiltersOpen] = useState(false);
  const [advancedFiltersOpen, setAdvancedFiltersOpen] = useState(false);
  const [sourcesOpen, setSourcesOpen] = useState(false);
  const [officialDiagnosticsOpen, setOfficialDiagnosticsOpen] = useState(false);
  const [privateDiagnosticsOpen, setPrivateDiagnosticsOpen] = useState(false);
  const [expandedInstalledCards, setExpandedInstalledCards] = useState<Set<string>>(new Set());
  const [expandedInstallableCards, setExpandedInstallableCards] = useState<Set<string>>(new Set());
  const [actionNotice, setActionNotice] = useState<CenterActionNotice | null>(null);
  const [query, setQuery] = useState("");
  const [kindFilter, setKindFilter] = useState<KindFilter>("all");
  const [installFilter, setInstallFilter] = useState<InstallFilter>("all");
  const [riskFilter, setRiskFilter] = useState<RiskFilter>("all");
  const [sourceFilter, setSourceFilter] = useState<SourceFilter>("all");
  const [stabilityFilter, setStabilityFilter] = useState<StabilityFilter>("all");
  const [readinessFilter, setReadinessFilter] = useState<ReadinessFilter>("all");
  const [sortMode, setSortMode] = useState<SortMode>("name");
  const [installedPage, setInstalledPage] = useState(1);
  const [selectedPackIds, setSelectedPackIds] = useState<Set<string>>(new Set());
  const [releasePage, setReleasePage] = useState(1);
  const [privatePage, setPrivatePage] = useState(1);
  const [readinessQueuePage, setReadinessQueuePage] = useState(1);
  // Collapsed catalog groups (by group key). Default expanded; a key present here
  // means the user folded that family away.
  const [collapsedGroups, setCollapsedGroups] = useState<Set<string>>(new Set());
  const toggleGroupCollapsed = (key: string) => {
    setCollapsedGroups((prev) => {
      const next = new Set(prev);
      if (next.has(key)) next.delete(key);
      else next.add(key);
      return next;
    });
  };
  const searchFocus = searchParams.get("q")?.trim() || searchParams.get("packId")?.trim() || "";
  const maintenanceMode = searchParams.get("maintenance") === "1";
  const advancedVisible = showAdvanced || maintenanceMode;
  const returnedFromStudio = searchParams.get("from") === "studio" && searchFocus.length > 0;
  const [routeAuditEntries, setRouteAuditEntries] = useState<PackBackendRouteAuditEntry[]>([]);
  const [routeAuditLoading, setRouteAuditLoading] = useState(false);
  const [routeAuditLoaded, setRouteAuditLoaded] = useState(false);
  const [routeAuditError, setRouteAuditError] = useState<string | null>(null);

  const refreshRouteAudit = useCallback(async () => {
    setRouteAuditLoading(true);
    try {
      const report = await packsClient.backendRouteAudit();
      setRouteAuditEntries(report.entries || []);
      setRouteAuditError(null);
      setRouteAuditLoaded(true);
    } catch (error) {
      setRouteAuditError(error instanceof Error ? error.message : String(error || "运行态路由审计加载失败"));
      setRouteAuditLoaded(true);
    } finally {
      setRouteAuditLoading(false);
    }
  }, []);

  const packs = data?.packs || [];
  const catalogEntries = catalog?.entries || [];
  const privateCatalogEntries = catalogEntries.filter((entry) => catalogActionForEntry(entry).kind !== "use");
  const releaseEntries = releaseCatalog.entries || [];
  const stats = useMemo(() => ({
    available: releaseEntries.filter((entry) => catalogActionForEntry(entry).kind !== "use").length + privateCatalogEntries.length,
    installed: packs.length,
    enabled: packs.filter((p) => p.status === "enabled").length,
  }), [packs, privateCatalogEntries.length, releaseEntries]);
  const centerReportItems = useMemo<ReadinessQueueItem[]>(() => {
    const seen = new Map<string, ReadinessQueueItem>();
    for (const pack of packs) {
      seen.set(pack.manifest.id, { manifest: pack.manifest, sourceLabel: pack.status === "enabled" ? "已安装 · 已启用" : "已安装" });
    }
    for (const entry of releaseEntries) {
      if (!seen.has(entry.manifest.id)) {
        seen.set(entry.manifest.id, {
          manifest: entry.manifest,
          sourceLabel: releaseSourceLabel(entry),
          packageUrl: typeof entry.package_url === "string" ? entry.package_url : undefined,
          sha256: typeof entry.sha256 === "string" ? entry.sha256 : undefined,
        });
      }
    }
    for (const entry of privateCatalogEntries) {
      if (!seen.has(entry.manifest.id)) {
        seen.set(entry.manifest.id, {
          manifest: entry.manifest,
          sourceLabel: privateSourceLabel(entry),
          packageUrl: typeof entry.package_url === "string" ? entry.package_url : undefined,
          sha256: typeof entry.sha256 === "string" ? entry.sha256 : undefined,
        });
      }
    }
    return [...seen.values()];
  }, [packs, releaseEntries, privateCatalogEntries]);
  const readinessItems = useMemo<ReadinessQueueItem[]>(() => {
    const readinessOrder = { needs_entry: 0, needs_context: 1, complete: 2 } as const;
    const deliveryOrder = { needs_meat: 0, plan_only: 1, support: 2, ready: 3 } as const;
    return centerReportItems
      .filter((item) => {
        const readiness = packReadiness(item.manifest);
        const delivery = packDeliveryProfile(item.manifest);
        const audit = packManifestAudit(item.manifest, routeAuditEntries);
        return audit.issues.length > 0 || readiness.missing.length > 0 || delivery.level === "needs_meat" || delivery.level === "plan_only";
      })
      .sort((a, b) => {
        const ra = packReadiness(a.manifest);
        const rb = packReadiness(b.manifest);
        const da = packDeliveryProfile(a.manifest);
        const db = packDeliveryProfile(b.manifest);
        const aa = packManifestAudit(a.manifest, routeAuditEntries);
        const ab = packManifestAudit(b.manifest, routeAuditEntries);
        const pa = packPolishPriority(a.manifest, routeAuditEntries);
        const pb = packPolishPriority(b.manifest, routeAuditEntries);
        const auditOrder = { blocked: 0, watch: 1, clear: 2 } as const;
        return pa.order - pb.order
          || auditOrder[aa.level] - auditOrder[ab.level]
          || deliveryOrder[da.level] - deliveryOrder[db.level]
          || readinessOrder[ra.level] - readinessOrder[rb.level]
          || rb.missing.length - ra.missing.length
          || a.manifest.name.localeCompare(b.manifest.name);
      });
  }, [centerReportItems, routeAuditEntries]);
  const readinessQueueTotal = readinessItems.length;
  const readinessQueuePageCount = pageCountFor(readinessQueueTotal, READINESS_QUEUE_PAGE_SIZE);
  const currentReadinessQueuePage = Math.min(readinessQueuePage, readinessQueuePageCount);
  const readinessQueue = useMemo(
    () => paginate(readinessItems, currentReadinessQueuePage, READINESS_QUEUE_PAGE_SIZE),
    [currentReadinessQueuePage, readinessItems],
  );
  const readinessBatchSummary = useMemo(() => {
    const summary = {
      p0: 0,
      p1: 0,
      p2: 0,
      highRisk: 0,
      needsEntry: 0,
      needsContext: 0,
      planOnly: 0,
      withOpenPath: 0,
      auditBlocked: 0,
      auditWatch: 0,
    };
    for (const item of readinessQueue) {
      const priority = packPolishPriority(item.manifest, routeAuditEntries);
      const risk = riskProfileForPack(item.manifest);
      const readiness = packReadiness(item.manifest);
      const delivery = packDeliveryProfile(item.manifest);
      const usability = packUsability(item.manifest);
      const audit = packManifestAudit(item.manifest, routeAuditEntries);
      if (priority.level === "P0") summary.p0 += 1;
      if (priority.level === "P1") summary.p1 += 1;
      if (priority.level === "P2") summary.p2 += 1;
      if (risk.requiresAuthorization) summary.highRisk += 1;
      if (readiness.level === "needs_entry") summary.needsEntry += 1;
      if (readiness.level === "needs_context") summary.needsContext += 1;
      if (delivery.level === "plan_only") summary.planOnly += 1;
      if (usability.primaryActionPath || item.manifest.frontend?.menus?.[0]?.path || item.manifest.frontend?.routes?.[0]?.path) summary.withOpenPath += 1;
      if (audit.level === "blocked") summary.auditBlocked += 1;
      if (audit.level === "watch") summary.auditWatch += 1;
    }
    return summary;
  }, [readinessQueue, routeAuditEntries]);
  const packKindStats = useMemo(() => {
    const manifests = new Map<string, PackManifest>();
    for (const pack of packs) manifests.set(pack.manifest.id, pack.manifest);
    for (const entry of releaseEntries) manifests.set(entry.manifest.id, entry.manifest);
    for (const entry of catalogEntries) manifests.set(entry.manifest.id, entry.manifest);
    const counts = { actionable: 0, infrastructure: 0, experimental: 0, documented: 0 };
    for (const manifest of manifests.values()) counts[packUsability(manifest).kind] += 1;
    return counts;
  }, [packs, releaseEntries, catalogEntries]);
  const readinessStats = useMemo(() => {
    const manifests = new Map<string, PackManifest>();
    for (const pack of packs) manifests.set(pack.manifest.id, pack.manifest);
    for (const entry of releaseEntries) manifests.set(entry.manifest.id, entry.manifest);
    for (const entry of catalogEntries) manifests.set(entry.manifest.id, entry.manifest);
    const counts = {
      total: manifests.size,
      complete: 0,
      needs_context: 0,
      needs_entry: 0,
      missingExamples: 0,
      missingSurface: 0,
      missingEntry: 0,
      missingBackend: 0,
    };
    for (const manifest of manifests.values()) {
      const readiness = packReadiness(manifest);
      counts[readiness.level] += 1;
      if (readiness.missing.includes("使用示例")) counts.missingExamples += 1;
      if (readiness.missing.includes("用户感知位置")) counts.missingSurface += 1;
      if (readiness.missing.includes("打开/使用入口")) counts.missingEntry += 1;
      if (readiness.missing.includes("后端能力声明")) counts.missingBackend += 1;
    }
    return counts;
  }, [packs, releaseEntries, catalogEntries]);
  const manifestAuditStats = useMemo(() => {
    const manifests = new Map<string, PackManifest>();
    for (const pack of packs) manifests.set(pack.manifest.id, pack.manifest);
    for (const entry of releaseEntries) manifests.set(entry.manifest.id, entry.manifest);
    for (const entry of catalogEntries) manifests.set(entry.manifest.id, entry.manifest);
    const counts = {
      total: manifests.size,
      clear: 0,
      watch: 0,
      blocked: 0,
      issues: 0,
    };
    for (const manifest of manifests.values()) {
      const audit = packManifestAudit(manifest, routeAuditEntries);
      counts[audit.level] += 1;
      counts.issues += audit.issues.length;
    }
    return counts;
  }, [packs, releaseEntries, catalogEntries, routeAuditEntries]);
  const deliveryStats = useMemo(() => {
    const manifests = new Map<string, PackManifest>();
    for (const pack of packs) manifests.set(pack.manifest.id, pack.manifest);
    for (const entry of releaseEntries) manifests.set(entry.manifest.id, entry.manifest);
    for (const entry of catalogEntries) manifests.set(entry.manifest.id, entry.manifest);
    const counts = { ready: 0, support: 0, plan_only: 0, needs_meat: 0 };
    for (const manifest of manifests.values()) counts[packDeliveryProfile(manifest).level] += 1;
    return counts;
  }, [packs, releaseEntries, catalogEntries]);
  const normalizedQuery = query.trim().toLowerCase();
  const matchesFilters = (manifest: PackManifest, options?: { installedStatus?: string; source: SourceFilter }) => {
    const usability = packUsability(manifest);
    const risk = riskProfileForPack(manifest);
    const readiness = packReadiness(manifest);
    if (normalizedQuery && !packSearchText(manifest).includes(normalizedQuery)) return false;
    // Demo / reference packs (e.g. dlc-demo, the DLC mechanism's reference impl) opt
    // out of the production catalog via metadata.catalogHidden. Still reachable when
    // installed or in advanced/maintenance view —— 隐藏而非删除，后端与测试不受影响。
    if (!advancedVisible && !options?.installedStatus && manifest.metadata?.catalogHidden === "true") return false;
    if (kindFilter !== "all") {
      if (kindFilter === "infrastructure") {
        if (usability.kind !== "infrastructure" && usability.kind !== "documented") return false;
      } else if (usability.kind !== kindFilter) {
        return false;
      }
    } else if (!advancedVisible && !options?.installedStatus) {
      // Default "all" view is for ordinary users: show only packs they'd
      // actually open and use. Two things are hidden (both revealed by the
      // 「显示高级」toggle / maintenance mode, and never hidden for installed
      // packs so users can always find & disable what they installed):
      //   1. infrastructure / documented packs —— 内核"内脏"（graph / retrieval /
      //      scheduler / files / state…）。它们在使用中被云雀自动调用，普通用户
      //      不需要、也看不懂，不该摆在能力包中心台面上。
      //   2. packs with NO user-facing entry at all —— 连入口都没有的纯后台件。
      if (usability.kind === "infrastructure" || usability.kind === "documented") return false;
      const entryPath = usability.primaryActionPath || manifest.frontend?.menus?.[0]?.path || manifest.frontend?.routes?.[0]?.path;
      if (!entryPath) return false;
    }
    if (riskFilter !== "all" && risk.level !== riskFilter) return false;
    if (sourceFilter !== "all" && options?.source !== sourceFilter) return false;
    if (stabilityFilter !== "all" && manifest.status !== stabilityFilter) return false;
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
    [packs, normalizedQuery, kindFilter, installFilter, riskFilter, sourceFilter, stabilityFilter, readinessFilter, sortMode, advancedVisible],
  );
  const filteredReleaseEntries = useMemo(
    () => sortPacks(
      releaseEntries.filter((entry) => matchesFilters(entry.manifest, { source: "official" })),
      (entry) => entry.manifest,
      sortMode,
    ),
    [releaseEntries, normalizedQuery, kindFilter, installFilter, riskFilter, sourceFilter, stabilityFilter, readinessFilter, sortMode, advancedVisible],
  );
  const filteredPrivateCatalogEntries = useMemo(
    () => sortPacks(
      privateCatalogEntries.filter((entry) => matchesFilters(entry.manifest, { source: "private" })),
      (entry) => entry.manifest,
      sortMode,
    ),
    [privateCatalogEntries, normalizedQuery, kindFilter, installFilter, riskFilter, sourceFilter, stabilityFilter, readinessFilter, sortMode, advancedVisible],
  );
  const totalMatches = filteredInstalledPacks.length + filteredReleaseEntries.length + filteredPrivateCatalogEntries.length;
  const visibleKindStats = useMemo(() => {
    const manifests = [
      ...filteredInstalledPacks.map((pack) => pack.manifest),
      ...filteredReleaseEntries.map((entry) => entry.manifest),
      ...filteredPrivateCatalogEntries.map((entry) => entry.manifest),
    ];
    const counts = { actionable: 0, infrastructure: 0, experimental: 0 };
    for (const manifest of manifests) {
      const kind = packUsability(manifest).kind;
      if (kind === "actionable" || kind === "experimental") counts[kind] += 1;
      else counts.infrastructure += 1;
    }
    return counts;
  }, [filteredInstalledPacks, filteredReleaseEntries, filteredPrivateCatalogEntries]);
  const visibleDeliveryStats = useMemo(() => {
    const manifests = [
      ...filteredInstalledPacks.map((pack) => pack.manifest),
      ...filteredReleaseEntries.map((entry) => entry.manifest),
      ...filteredPrivateCatalogEntries.map((entry) => entry.manifest),
    ];
    const counts = { ready: 0, support: 0, plan_only: 0, needs_meat: 0 };
    for (const manifest of manifests) counts[packDeliveryProfile(manifest).level] += 1;
    return counts;
  }, [filteredInstalledPacks, filteredReleaseEntries, filteredPrivateCatalogEntries]);
  const visibleReadinessStats = useMemo(() => {
    const manifests = [
      ...filteredInstalledPacks.map((pack) => pack.manifest),
      ...filteredReleaseEntries.map((entry) => entry.manifest),
      ...filteredPrivateCatalogEntries.map((entry) => entry.manifest),
    ];
    const counts = { complete: 0, needs_context: 0, needs_entry: 0 };
    for (const manifest of manifests) counts[packReadiness(manifest).level] += 1;
    return counts;
  }, [filteredInstalledPacks, filteredReleaseEntries, filteredPrivateCatalogEntries]);
  const currentViewAdvice = useMemo(() => {
    if (totalMatches === 0) return "建议清空搜索或放宽筛选，先恢复可见候选。";
    if (visibleDeliveryStats.needs_meat > 0) return "建议先进入打磨队列：P0 修可用路径，P1/P2 补说明、边界和主路径验收。";
    if (visibleDeliveryStats.plan_only > 0) return "建议先查看实验限制、风险和验证路径，不要把计划能力当成稳定主路径。";
    if (filteredReleaseEntries.length + filteredPrivateCatalogEntries.length > 0) return "建议先打开详情或工坊只读检查，再安装、启用并回到中心验证入口。";
    if (filteredInstalledPacks.some((pack) => pack.status !== "enabled")) return "建议先查看详情确认权限，再启用；启用后按入口提示验证结果。";
    return "建议从卡片里的入口或 Chat 主路径触发一次，确认结果、产物或状态变化可见。";
  }, [filteredInstalledPacks, filteredPrivateCatalogEntries.length, filteredReleaseEntries.length, totalMatches, visibleDeliveryStats]);
  const currentViewAction = useMemo<CurrentViewAction>(() => {
    if (totalMatches === 0) {
      return {
        label: "清空筛选",
        kind: "filter",
        onPress: () => {
          setQuery("");
          setKindFilter("all");
          setInstallFilter("all");
          setRiskFilter("all");
          setSourceFilter("all");
          setStabilityFilter("all");
          setReadinessFilter("all");
          setSortMode("name");
        },
      };
    }
    if (visibleDeliveryStats.needs_meat > 0) {
      return { label: "去打磨队列", kind: "anchor", href: "#readiness-queue" };
    }
    if (visibleDeliveryStats.plan_only > 0) {
      return {
        label: "只看实验能力",
        kind: "filter",
        onPress: () => {
          setKindFilter("experimental");
          setStabilityFilter("alpha");
          setSortMode("kind");
        },
      };
    }
    if (filteredReleaseEntries.length + filteredPrivateCatalogEntries.length > 0) {
      return {
        label: "只看可安装",
        kind: "filter",
        onPress: () => {
          setInstallFilter("available");
          setSourceFilter("all");
          setSortMode("status");
        },
      };
    }
    if (filteredInstalledPacks.some((pack) => pack.status !== "enabled")) {
      return {
        label: "只看未启用",
        kind: "filter",
        onPress: () => {
          setInstallFilter("disabled");
          setSourceFilter("installed");
          setSortMode("status");
        },
      };
    }
    return { label: "去 Chat 验证", kind: "link", href: "/chat" };
  }, [filteredInstalledPacks, filteredPrivateCatalogEntries.length, filteredReleaseEntries.length, totalMatches, visibleDeliveryStats]);
  const installedPageCount = pageCountFor(filteredInstalledPacks.length);
  const currentInstalledPage = Math.min(installedPage, installedPageCount);
  const pagedInstalledPacks = paginate(filteredInstalledPacks, currentInstalledPage);
  const releasePageCount = pageCountFor(filteredReleaseEntries.length);
  const currentReleasePage = Math.min(releasePage, releasePageCount);
  const pagedReleaseEntries = paginate(filteredReleaseEntries, currentReleasePage);
  const privatePageCount = pageCountFor(filteredPrivateCatalogEntries.length);
  const currentPrivatePage = Math.min(privatePage, privatePageCount);
  const pagedPrivateCatalogEntries = paginate(filteredPrivateCatalogEntries, currentPrivatePage);
  const batchReadinessPrompt = useMemo(
    () => buildBatchReadinessPrompt(readinessQueue, {
      page: currentReadinessQueuePage,
      pageCount: readinessQueuePageCount,
      total: readinessQueueTotal,
      pageSize: READINESS_QUEUE_PAGE_SIZE,
    }, routeAuditEntries),
    [currentReadinessQueuePage, readinessQueue, readinessQueuePageCount, readinessQueueTotal, routeAuditEntries],
  );
  const batchReadinessChatHref = readinessQueue.length > 0 ? `/chat?q=${encodeURIComponent(batchReadinessPrompt)}` : "";
  const batchReadinessStudioHref = readinessQueue.length > 0 ? `/packs/studio?batch=${encodeURIComponent(batchReadinessPrompt)}` : "";
  const packUsabilityReport = useMemo(
    () => buildPackCenterUsabilityReport(centerReportItems, readinessQueue, {
      page: currentReadinessQueuePage,
      pageCount: readinessQueuePageCount,
      total: readinessQueueTotal,
      pageSize: READINESS_QUEUE_PAGE_SIZE,
    }, routeAuditEntries),
    [centerReportItems, currentReadinessQueuePage, readinessQueue, readinessQueuePageCount, readinessQueueTotal, routeAuditEntries],
  );

  useEffect(() => {
    if (searchFocus) setQuery(searchFocus);
  }, [searchFocus]);

  useEffect(() => {
    if (!advancedVisible || routeAuditLoaded || routeAuditLoading) return;
    void refreshRouteAudit();
  }, [advancedVisible, refreshRouteAudit, routeAuditLoaded, routeAuditLoading]);

  useEffect(() => {
    setInstalledPage(1);
    setReleasePage(1);
    setPrivatePage(1);
  }, [normalizedQuery, kindFilter, installFilter, riskFilter, sourceFilter, stabilityFilter, readinessFilter, sortMode]);

  useEffect(() => {
    setReadinessQueuePage(1);
  }, [readinessQueueTotal]);

  const refreshAll = async () => {
    await Promise.all([
      refresh(),
      refreshCatalog(),
      refreshReleaseCatalog(),
      advancedVisible ? refreshRouteAudit() : Promise.resolve(),
    ]);
  };

  const copyBatchReadinessPrompt = async () => {
    if (readinessQueue.length === 0) return;
    await navigator.clipboard?.writeText(batchReadinessPrompt);
    showToast("已复制批量打磨任务", "success");
  };

  const copyPackUsabilityReport = async () => {
    await navigator.clipboard?.writeText(JSON.stringify(packUsabilityReport, null, 2));
    showToast("已复制能力包体检报告", "success");
  };

  const noticeForInstalled = (manifest: PackManifest): CenterActionNotice => ({
    title: "能力包已安装",
    detail: "下一步先查看详情确认权限和入口，再启用；也可以继续筛选、固定或交给小羽打磨。",
    href: `/packs/detail?id=${encodeURIComponent(manifest.id)}`,
    actionLabel: "查看详情并启用",
    inlineActionLabel: "立即启用",
    inlineAction: () => enable(manifest.id),
    packId: manifest.id,
  });
  const noticeForEnabled = (manifest: PackManifest): CenterActionNotice => {
    const usability = packUsability(manifest);
    const openPath = packSafeOpenPath(manifest);
    return {
      title: "能力包已启用",
      detail: openPath ? "可以打开入口验证结果；如果它有侧栏入口，也可以固定到侧栏减少下次寻找。" : "这个包没有独立入口，启用后会在 Chat、任务、记忆或知识流程中被云雀感知。",
      href: openPath || `/packs/detail?id=${encodeURIComponent(manifest.id)}`,
      actionLabel: openPath ? (usability.primaryActionLabel || "打开入口") : "查看详情",
      packId: manifest.id,
    };
  };
  const noticeForDisabled = (id: string): CenterActionNotice => ({
    title: "能力包已禁用",
    detail: "云雀不会再把它纳入可用能力；如需恢复，可在本页搜索这个能力包后重新启用。",
    href: `/packs?q=${encodeURIComponent(id)}`,
    actionLabel: "回到这个包",
    packId: id,
  });
  const noticeForRollback = (id: string): CenterActionNotice => ({
    title: "能力包已回滚",
    detail: "建议回到这个包确认版本、权限和入口；如果仍不符合预期，可以继续禁用或交给小羽检查。",
    href: `/packs?q=${encodeURIComponent(id)}`,
    actionLabel: "回到这个包",
    packId: id,
  });

  const run = async (
    label: string,
    op: () => Promise<unknown>,
    successMsg = "操作成功",
    notice?: CenterActionNotice,
    onResult?: (result: unknown) => void,
  ) => {
    setBusy(label);
    try {
      const result = await op();
      // 乐观更新：用后端返回的新状态立刻刷新 UI，不必等 refreshAll，
      // 否则用户点完启用/禁用要等几百毫秒~几秒列表才变，感觉像没生效。
      onResult?.(result);
      if (notice) setActionNotice(notice);
      showToast(successMsg, "success");
      await refreshAll();
      window.dispatchEvent(new CustomEvent("yunque:packs-changed"));
    } catch (e) {
      showToast(label.startsWith("install") ? formatPackInstallError(e) : formatPackInstallError(e, "操作失败"), "error");
    } finally {
      setBusy(null);
    }
  };

  const installLocal = () => run("install:local", () => packsClient.install({ manifestPath, download: false }), "已安装，可在详情页启用", {
    title: "能力包已安装",
    detail: "下一步回到详情页确认权限和入口，再决定是否启用。若这个本地包还像空壳，可交给小羽先做只读检查。",
    href: "/packs/studio",
    actionLabel: "去工坊检查",
  });
  const installRelease = (entry: PackReleaseCatalogEntry) => {
    const action = catalogActionForEntry(entry);
    if (action.kind === "enable") return enable(entry.manifest.id);
    const request = entryInstallRequest({ ...entry, source: entry.release_url });
    if (!request) {
      showToast("此能力包没有可用的安装源", "error");
      return;
    }
    return run(`install:${entry.package_url}`, () => packsClient.install(request), "已安装，可继续启用或打开详情", noticeForInstalled(entry.manifest));
  };
  const installCatalogEntry = (entry: PackCatalogEntry) => {
    const action = catalogActionForEntry(entry);
    if (action.kind === "enable") return enable(entry.manifest.id);
    const request = entryInstallRequest(entry);
    if (!request) {
      showToast("此能力包没有可用的安装源", "error");
      return;
    }
    return run(`install:${entry.manifest.id}`, () => packsClient.install(request), "已安装，可继续启用或打开详情", noticeForInstalled(entry.manifest));
  };
  const manifestById = (id: string) =>
    packs.find((pack) => pack.manifest.id === id)?.manifest
    || releaseEntries.find((entry) => entry.manifest.id === id)?.manifest
    || privateCatalogEntries.find((entry) => entry.manifest.id === id)?.manifest;
  const studioReturnManifest = returnedFromStudio ? manifestById(searchFocus) : undefined;
  const studioReturnOpenPath = studioReturnManifest
    ? packSafeOpenPath(studioReturnManifest)
    : undefined;
  const studioReturnDelivery = studioReturnManifest ? packDeliveryProfile(studioReturnManifest) : undefined;
  const studioReturnRisk = studioReturnManifest ? riskProfileForPack(studioReturnManifest) : undefined;
  const studioReturnVerificationSteps = studioReturnManifest
    ? packVerificationSteps(studioReturnManifest).slice(0, 2)
    : [];
  // 用 enable/disable 返回的新状态就地更新本地已安装列表，实现乐观刷新。
  const patchPackStatus = (id: string, result: unknown) => {
    const status = (result as { status?: string } | null)?.status;
    if (!status) return;
    setData((prev) => ({
      ...prev,
      packs: prev.packs.map((p) => (p.manifest?.id === id ? { ...p, status } : p)),
    }));
  };
  const enable = (id: string) => {
    const manifest = manifestById(id);
    return run(`enable:${id}`, () => packsClient.enable(id), "已启用，可在命令菜单、扩展分组或本页入口打开", manifest ? noticeForEnabled(manifest) : undefined, (r) => patchPackStatus(id, r));
  };
  const disable = (id: string) => run(`disable:${id}`, () => packsClient.disable(id), "已禁用", noticeForDisabled(id), (r) => patchPackStatus(id, r));

  // 批量启用/禁用：用后端批量接口一次处理选中的多个能力包。
  const runBatch = async (label: string, op: (ids: string[]) => Promise<{ results: { id: string; ok: boolean; status?: string }[]; succeeded: number; total: number }>, verb: string) => {
    const ids = [...selectedPackIds];
    if (ids.length === 0) return;
    setBusy(label);
    try {
      const res = await op(ids);
      // 乐观更新：把成功项的新状态就地写入本地列表。
      setData((prev) => ({
        ...prev,
        packs: prev.packs.map((p) => {
          const hit = res.results.find((r) => r.ok && r.status && r.id === p.manifest?.id);
          return hit ? { ...p, status: hit.status as string } : p;
        }),
      }));
      showToast(`已${verb} ${res.succeeded}/${res.total} 个能力包`, res.succeeded === res.total ? "success" : "warning");
      setSelectedPackIds(new Set());
      await refreshAll();
      window.dispatchEvent(new CustomEvent("yunque:packs-changed"));
    } catch (e) {
      showToast(formatPackInstallError(e, "批量操作失败"), "error");
    } finally {
      setBusy(null);
    }
  };
  const batchEnable = () => runBatch("batch:enable", (ids) => packsClient.batchEnable(ids), "启用");
  const batchDisable = () => runBatch("batch:disable", (ids) => packsClient.batchDisable(ids), "禁用");
  const togglePackSelected = (id: string, selected: boolean) => {
    setSelectedPackIds((prev) => {
      const next = new Set(prev);
      if (selected) next.add(id); else next.delete(id);
      return next;
    });
  };
  const rollback = (id: string) => run(`rollback:${id}`, () => packsClient.rollback(id), "已回滚", noticeForRollback(id));

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
  const resetFilters = () => {
    setQuery("");
    setKindFilter("all");
    setInstallFilter("all");
    setRiskFilter("all");
    setSourceFilter("all");
    setStabilityFilter("all");
    setReadinessFilter("all");
    setSortMode("name");
  };
  const toggleInstallableDetails = (key: string) => {
    setExpandedInstallableCards((current) => {
      const next = new Set(current);
      if (next.has(key)) next.delete(key);
      else next.add(key);
      return next;
    });
  };
  const toggleInstalledDetails = (id: string) => {
    setExpandedInstalledCards((current) => {
      const next = new Set(current);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      return next;
    });
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
    ...(stabilityFilter !== "all" ? [{
      key: "stability",
      label: `稳定性：${STABILITY_FILTER_LABELS[stabilityFilter]}`,
      clearLabel: "清除稳定性",
      clear: () => setStabilityFilter("all"),
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
    <div className="flex flex-col min-h-0" style={{ height: "100%", overflowY: "auto" }}>
      <div className="p-5 border-b" style={{ borderColor: "var(--yunque-border)" }}>
        <PageHeader
          icon={<Boxes size={20} />}
          title="能力包中心"
          description="安装、启用、权限、入口。"
          onRefresh={refreshAll}
          actions={
            <div className="flex flex-wrap items-center gap-2">
              <Button size="sm" className="btn-accent" onPress={() => setSourcesOpen(true)}>
                <Download size={14} /> 添加能力包
              </Button>
              {advancedVisible && (
                <Link href="/packs/studio">
                  <Button size="sm" variant="outline">
                    <Wrench size={14} /> 能力包工坊
                  </Button>
                </Link>
              )}
            </div>
          }
        />

        <div className="mt-4 flex flex-wrap items-center gap-x-5 gap-y-1 text-sm">
          <span className="inline-flex items-baseline gap-1.5">
            <span style={{ color: "var(--yunque-text-muted)" }}>可安装</span>
            <span className="font-semibold tabular-nums" style={{ color: "var(--yunque-text)" }}>{releaseLoading || catalogLoading ? "…" : stats.available}</span>
          </span>
          <span className="inline-flex items-baseline gap-1.5">
            <span style={{ color: "var(--yunque-text-muted)" }}>已安装</span>
            <span className="font-semibold tabular-nums" style={{ color: "var(--yunque-text)" }}>{stats.installed}</span>
          </span>
          <span className="inline-flex items-baseline gap-1.5">
            <span style={{ color: "var(--yunque-text-muted)" }}>已启用</span>
            <span className="font-semibold tabular-nums" style={{ color: "var(--yunque-text)" }}>{stats.enabled}</span>
          </span>
        </div>

        {actionNotice && (
          <div className="mt-4 rounded-lg border p-4" style={{ borderColor: "var(--yunque-success-border)", background: "var(--yunque-success-soft)" }}>
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
                  <Button size="sm" className="btn-accent" onPress={actionNotice.inlineAction} isDisabled={busy === `enable:${actionNotice.packId}`}>
                    <Power size={14} /> {actionNotice.inlineActionLabel || "继续"}
                  </Button>
                )}
                {actionNotice.href && (
                  <Link href={actionNotice.href}>
                    <Button size="sm" variant="outline">
                      {actionNotice.href.startsWith("/packs/") && !actionNotice.href.startsWith("/packs?") ? <ExternalLink size={14} /> : <ArrowRight size={14} />}
                      {actionNotice.actionLabel || "继续"}
                    </Button>
                  </Link>
                )}
                {actionNotice.packId && (
                  <Link href={packStudioHref(manifestById(actionNotice.packId) || { id: actionNotice.packId, name: actionNotice.packId, version: "", status: "beta" } as PackManifest)}>
                    <Button size="sm" variant="ghost">
                      <Wrench size={14} /> 交给小羽打磨
                    </Button>
                  </Link>
                )}
              </div>
            </div>
          </div>
        )}

        {returnedFromStudio && (
          <div className="mt-4 rounded-lg border p-4" style={{ borderColor: "var(--yunque-accent-border)", background: "var(--yunque-accent-soft)" }}>
            <div className="flex flex-wrap items-start justify-between gap-3">
              <div className="min-w-0 flex-1">
                <div className="flex items-center gap-2 text-sm font-semibold" style={{ color: "var(--yunque-text)" }}>
                  <Wrench size={16} style={{ color: "var(--yunque-primary)" }} />
                  工坊返回验收
                </div>
                <div className="mt-1 text-xs leading-5" style={{ color: "var(--yunque-text-secondary)" }}>
                  {studioReturnManifest
                    ? `已聚焦 ${studioReturnManifest.name}。查权限，复验入口；不符就回工坊或禁用。`
                    : `已按 ${searchFocus} 聚焦。找不到时刷新来源，或回工坊复检包 ID。`}
                </div>
                {studioReturnManifest && (
                  <div className="mt-2 flex flex-wrap gap-2">
                    <Chip size="sm" color={studioReturnDelivery?.level === "ready" ? "success" : studioReturnDelivery?.level === "needs_meat" ? "danger" : "warning"}>
                      {studioReturnDelivery?.label}
                    </Chip>
                    <Chip size="sm" color={studioReturnRisk?.level === "high" ? "danger" : studioReturnRisk?.level === "medium" ? "warning" : "success"}>
                      {studioReturnRisk?.label}
                    </Chip>
                    <Chip size="sm" variant="soft">搜索已聚焦</Chip>
                  </div>
                )}
                {studioReturnManifest && (
                  <div className="mt-3 grid gap-2 md:grid-cols-3">
                    {studioReturnVerificationSteps.map((step) => (
                      <div
                        key={step.key}
                        className="rounded-md border p-3"
                        style={{
                          borderColor: "var(--yunque-accent-border)",
                          background: "var(--yunque-accent-soft)",
                        }}
                      >
                        <div className="text-xs font-medium" style={{ color: "var(--yunque-text)" }}>
                          {step.label}
                        </div>
                        <div className="mt-1 text-[11px] leading-5" style={{ color: "var(--yunque-text-muted)" }}>
                          {step.detail}
                        </div>
                      </div>
                    ))}
                    <div
                      className="rounded-md border p-3"
                      style={{
                        borderColor: "var(--yunque-warning-border)",
                        background: "var(--yunque-warning-soft)",
                      }}
                    >
                      <div className="text-xs font-medium" style={{ color: "var(--yunque-text)" }}>
                        复验失败怎么退
                      </div>
                      <div className="mt-1 text-[11px] leading-5" style={{ color: "var(--yunque-text-muted)" }}>
                        先禁用；有上一版再回滚。仍不符就交给小羽改。
                      </div>
                    </div>
                  </div>
                )}
              </div>
              <div className="flex flex-wrap gap-2">
                {studioReturnManifest && (
                  <Link href={`/packs/detail?id=${encodeURIComponent(studioReturnManifest.id)}`}>
                    <Button size="sm" variant="outline">
                      <ShieldCheck size={14} /> 权限与详情
                    </Button>
                  </Link>
                )}
                {studioReturnOpenPath && (
                  <Link href={studioReturnOpenPath}>
                    <Button size="sm" className="btn-accent">
                      <ExternalLink size={14} /> 打开入口复验
                    </Button>
                  </Link>
                )}
                {studioReturnManifest && (
                  <Link href={packStudioHref(studioReturnManifest)}>
                    <Button size="sm" variant="ghost">
                      <Wrench size={14} /> 继续让小羽改
                    </Button>
                  </Link>
                )}
              </div>
            </div>
          </div>
        )}

        {advancedVisible && (
          <>
        <div className="mt-4 rounded-lg border p-4" style={{ borderColor: "var(--yunque-border)", background: "var(--yunque-bg-hover)" }}>
          <div className="mb-3 flex items-start justify-between gap-3">
            <div>
              <div className="text-sm font-semibold" style={{ color: "var(--yunque-text)" }}>能力包不是都要单独打开</div>
              <div className="mt-1 text-xs leading-5" style={{ color: "var(--yunque-text-muted)" }}>
                按入口、底座、实验、待打磨分流。
              </div>
            </div>
            <Chip size="sm" variant="soft">按当前来源统计</Chip>
          </div>
          <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-4">
            <button
              type="button"
              className="rounded-md p-3 text-left transition-colors hover:bg-white/5"
              style={{ background: "var(--yunque-success-soft)", border: "1px solid var(--yunque-success-border)" }}
              onClick={() => {
                setKindFilter("actionable");
                setSortMode("kind");
              }}
            >
              <div className="flex items-center justify-between gap-2">
                <span className="text-sm font-medium" style={{ color: "var(--yunque-text)" }}>可直接使用</span>
                <span className="text-lg font-semibold" style={{ color: "var(--yunque-success)" }}>{packKindStats.actionable}</span>
              </div>
              <div className="mt-2 text-xs leading-5" style={{ color: "var(--yunque-text-muted)" }}>
                有入口，可直接验证。
              </div>
            </button>
            <button
              type="button"
              className="rounded-md p-3 text-left transition-colors hover:bg-white/5"
              style={{ background: "var(--yunque-accent-soft)", border: "1px solid var(--yunque-accent-border)" }}
              onClick={() => {
                setKindFilter("infrastructure");
                setSortMode("kind");
              }}
            >
              <div className="flex items-center justify-between gap-2">
                <span className="text-sm font-medium" style={{ color: "var(--yunque-text)" }}>基础能力</span>
                <span className="text-lg font-semibold" style={{ color: "var(--yunque-primary)" }}>{packKindStats.infrastructure + packKindStats.documented}</span>
              </div>
              <div className="mt-2 text-xs leading-5" style={{ color: "var(--yunque-text-muted)" }}>
                在 Chat、任务、记忆、知识或设置页生效。
              </div>
            </button>
            <button
              type="button"
              className="rounded-md p-3 text-left transition-colors hover:bg-white/5"
              style={{ background: "var(--yunque-warning-soft)", border: "1px solid var(--yunque-warning-border)" }}
              onClick={() => {
                setKindFilter("experimental");
                setStabilityFilter("alpha");
                setSortMode("kind");
              }}
            >
              <div className="flex items-center justify-between gap-2">
                <span className="text-sm font-medium" style={{ color: "var(--yunque-text)" }}>实验中</span>
                <span className="text-lg font-semibold" style={{ color: "var(--yunque-warning)" }}>{packKindStats.experimental}</span>
              </div>
              <div className="mt-2 text-xs leading-5" style={{ color: "var(--yunque-text-muted)" }}>
                先看限制、权限和风险。
              </div>
            </button>
            <button
              type="button"
              className="rounded-md p-3 text-left transition-colors hover:bg-white/5"
              style={{ background: "var(--yunque-danger-soft)", border: "1px solid var(--yunque-danger-border)" }}
              onClick={() => {
                setReadinessFilter("needs_entry");
                setSortMode("readiness");
              }}
            >
              <div className="flex items-center justify-between gap-2">
                <span className="text-sm font-medium" style={{ color: "var(--yunque-text)" }}>优先打磨</span>
                <span className="text-lg font-semibold" style={{ color: "var(--yunque-danger)" }}>{readinessQueueTotal}</span>
              </div>
              <div className="mt-2 text-xs leading-5" style={{ color: "var(--yunque-text-muted)" }}>
                先补入口、边界、验收。
              </div>
            </button>
          </div>
        </div>

        <div className="mt-4 rounded-lg border p-4" style={{ borderColor: "var(--yunque-border)", background: "var(--yunque-surface)" }}>
          <div className="mb-3 flex flex-wrap items-start justify-between gap-3">
            <div>
              <div className="text-sm font-semibold" style={{ color: "var(--yunque-text)" }}>能力包体检总览</div>
              <div className="mt-1 text-xs leading-5" style={{ color: "var(--yunque-text-muted)" }}>
                已体检 {readinessStats.total} 个；先看 P0。
              </div>
              <div className="mt-2">
                <Chip size="sm" color="success">说明完整</Chip>
              </div>
            </div>
            <a href="#readiness-queue">
              <Button
                size="sm"
                variant="outline"
                isDisabled={readinessQueue.length === 0}
                onPress={() => {
                  if (readinessQueue.length === 0) return;
                  setSortMode("readiness");
                }}
              >
                查看打磨队列 <ArrowRight size={14} />
              </Button>
            </a>
            <Button size="sm" variant="ghost" onPress={copyPackUsabilityReport}>
              <Copy size={14} /> 复制体检报告 JSON
            </Button>
          </div>
          <div className="grid gap-3 md:grid-cols-3">
            <button
              type="button"
              className="rounded-md p-3 text-left transition-colors hover:bg-white/5"
              style={{ background: "var(--yunque-success-soft)", border: "1px solid var(--yunque-success-border)" }}
              onClick={() => setReadinessFilter("complete")}
            >
              <div className="flex items-center justify-between gap-2">
                <span className="text-sm font-medium" style={{ color: "var(--yunque-text)" }}>说明完整</span>
                <span className="text-lg font-semibold" style={{ color: "var(--yunque-success)" }}>{readinessStats.complete}</span>
              </div>
              <div className="mt-2 text-xs leading-5" style={{ color: "var(--yunque-text-muted)" }}>
                清楚，可展示。
              </div>
            </button>
            <button
              type="button"
              className="rounded-md p-3 text-left transition-colors hover:bg-white/5"
              style={{ background: "var(--yunque-warning-soft)", border: "1px solid var(--yunque-warning-border)" }}
              onClick={() => setReadinessFilter("needs_context")}
            >
              <div className="flex items-center justify-between gap-2">
                <span className="text-sm font-medium" style={{ color: "var(--yunque-text)" }}>需补说明</span>
                <span className="text-lg font-semibold" style={{ color: "var(--yunque-warning)" }}>{readinessStats.needs_context}</span>
              </div>
              <div className="mt-2 text-xs leading-5" style={{ color: "var(--yunque-text-muted)" }}>
                缺示例 {readinessStats.missingExamples} · 感知位置 {readinessStats.missingSurface}
              </div>
            </button>
            <button
              type="button"
              className="rounded-md p-3 text-left transition-colors hover:bg-white/5"
              style={{ background: "var(--yunque-danger-soft)", border: "1px solid var(--yunque-danger-border)" }}
              onClick={() => setReadinessFilter("needs_entry")}
            >
              <div className="flex items-center justify-between gap-2">
                <span className="text-sm font-medium" style={{ color: "var(--yunque-text)" }}>需补入口</span>
                <span className="text-lg font-semibold" style={{ color: "var(--yunque-danger)" }}>{readinessStats.needs_entry}</span>
              </div>
              <div className="mt-2 text-xs leading-5" style={{ color: "var(--yunque-text-muted)" }}>
                缺入口 {readinessStats.missingEntry} · 后端声明 {readinessStats.missingBackend}
              </div>
            </button>
          </div>
          <div className="mt-4 mb-3">
            <div className="text-sm font-semibold" style={{ color: "var(--yunque-text)" }}>Manifest 审计</div>
            <div className="mt-1 text-xs leading-5" style={{ color: "var(--yunque-text-muted)" }}>
              拦 404、未挂载路由和权限缺口。
            </div>
            <div className="mt-2 flex flex-wrap items-center gap-2">
              <Chip size="sm" variant="soft">
                {routeAuditLoading ? "运行态审计加载中" : routeAuditLoaded ? `运行态 ${routeAuditEntries.filter((entry) => entry.status !== "ok").length} 个问题` : "运行态待加载"}
              </Chip>
              {routeAuditError && <Chip size="sm" color="warning">运行态加载失败</Chip>}
              <Button size="sm" variant="ghost" onPress={refreshRouteAudit} isDisabled={routeAuditLoading}>
                刷新运行态审计
              </Button>
            </div>
            {routeAuditError && (
              <div className="mt-2 text-xs leading-5" style={{ color: "var(--yunque-warning)" }}>
                {routeAuditError}
              </div>
            )}
          </div>
          <div className="grid gap-3 md:grid-cols-3">
            {([
              ["clear", "审计清晰", manifestAuditStats.clear, "结构清晰。"],
              ["watch", "需要复核", manifestAuditStats.watch, "需对齐。"],
              ["blocked", "阻塞验收", manifestAuditStats.blocked, "会阻塞验收。"],
            ] as const).map(([key, label, value, detail]) => {
              const style = key === "blocked"
                ? { borderColor: "var(--yunque-danger-border)", background: "var(--yunque-danger-soft)", color: "var(--yunque-danger)" }
                : key === "watch"
                  ? { borderColor: "var(--yunque-warning-border)", background: "var(--yunque-warning-soft)", color: "var(--yunque-warning)" }
                  : { borderColor: "var(--yunque-success-border)", background: "var(--yunque-success-soft)", color: "var(--yunque-success)" };
              return (
                <div key={key} className="rounded-md border p-3" style={{ borderColor: style.borderColor, background: style.background }}>
                  <div className="flex items-center justify-between gap-2">
                    <span className="text-sm font-medium" style={{ color: "var(--yunque-text)" }}>{label}</span>
                    <span className="text-lg font-semibold" style={{ color: style.color }}>{value}</span>
                  </div>
                  <div className="mt-2 text-xs leading-5" style={{ color: "var(--yunque-text-muted)" }}>{detail}</div>
                </div>
              );
            })}
          </div>
          <div className="mt-4 mb-3">
            <div className="text-sm font-semibold" style={{ color: "var(--yunque-text)" }}>交付状态分布</div>
            <div className="mt-1 text-xs leading-5" style={{ color: "var(--yunque-text-muted)" }}>
              能否直接验价值。
            </div>
          </div>
          <div className="grid gap-3 md:grid-cols-4">
            {([
              ["ready", "可直接交付", deliveryStats.ready, "入口和结果可验。"],
              ["support", "后台支撑", deliveryStats.support, "在主流程生效。"],
              ["plan_only", "实验/计划", deliveryStats.plan_only, "不当稳定主路径。"],
              ["needs_meat", "需打磨", deliveryStats.needs_meat, "缺入口或验收路径。"],
            ] as const).map(([key, label, value, detail]) => {
              const style = deliveryToneStyle(key === "ready" ? "success" : key === "support" ? "primary" : key === "plan_only" ? "warning" : "danger");
              return (
                <div key={key} className="rounded-md border p-3" style={{ borderColor: style.borderColor, background: style.background }}>
                  <div className="flex items-center justify-between gap-2">
                    <span className="text-sm font-medium" style={{ color: "var(--yunque-text)" }}>{label}</span>
                    <span className="text-lg font-semibold" style={{ color: style.color }}>{value}</span>
                  </div>
                  <div className="mt-2 text-xs leading-5" style={{ color: "var(--yunque-text-muted)" }}>{detail}</div>
                </div>
              );
            })}
          </div>
        </div>

        {readinessQueue.length > 0 && (
          <div id="readiness-queue" className="mt-4 scroll-mt-24 rounded-lg border p-4" style={{ borderColor: "var(--yunque-warning-border)", background: "var(--yunque-warning-soft)" }}>
            <div className="mb-3 flex flex-wrap items-start justify-between gap-3">
              <div>
                <div className="text-sm font-semibold" style={{ color: "var(--yunque-text)" }}>打磨与验收队列</div>
                <div className="mt-1 text-xs leading-5" style={{ color: "var(--yunque-text-muted)" }}>
                  第 {currentReadinessQueuePage} / {readinessQueuePageCount} 批 · {readinessQueue.length} / {readinessQueueTotal} 个。
                </div>
              </div>
              <Button size="sm" variant="outline" onPress={() => {
                setReadinessFilter("needs_entry");
                setSortMode("readiness");
              }}>
                只看需补入口
              </Button>
              <Button size="sm" variant="outline" onPress={copyBatchReadinessPrompt}>
                复制批量打磨任务
              </Button>
              <Button size="sm" variant="ghost" onPress={copyPackUsabilityReport}>
                复制体检报告 JSON <Copy size={14} />
              </Button>
              <Link href={batchReadinessStudioHref}>
                <Button size="sm" variant="outline">
                  导入工坊逐包处理 <Wrench size={14} />
                </Button>
              </Link>
              <Link href={batchReadinessChatHref}>
                <Button size="sm" className="btn-accent">
                  交给 Chat 批量打磨 <ArrowRight size={14} />
                </Button>
              </Link>
            </div>
            <div className="mb-3 grid gap-2 lg:grid-cols-4">
              <div className="rounded-md border p-3" style={{ borderColor: "var(--yunque-danger-border)", background: "var(--yunque-danger-soft)" }}>
                <div className="text-xs font-medium" style={{ color: "var(--yunque-text)" }}>本批焦点</div>
                <div className="mt-1 text-[11px] leading-5" style={{ color: "var(--yunque-text-secondary)" }}>
                  P0 {readinessBatchSummary.p0} · P1 {readinessBatchSummary.p1} · P2 {readinessBatchSummary.p2}；缺入口 {readinessBatchSummary.needsEntry} · 补说明 {readinessBatchSummary.needsContext}
                </div>
              </div>
              <div className="rounded-md border p-3" style={{ borderColor: "var(--yunque-accent-border)", background: "var(--yunque-accent-soft)" }}>
                <div className="text-xs font-medium" style={{ color: "var(--yunque-text)" }}>处理顺序</div>
                <div className="mt-1 text-[11px] leading-5" style={{ color: "var(--yunque-text-secondary)" }}>
                  P0 进工坊；P1/P2 先复验入口再重包。
                </div>
              </div>
              <div className="rounded-md border p-3" style={{ borderColor: "var(--yunque-success-border)", background: "var(--yunque-success-soft)" }}>
                <div className="text-xs font-medium" style={{ color: "var(--yunque-text)" }}>验收</div>
                <div className="mt-1 text-[11px] leading-5" style={{ color: "var(--yunque-text-secondary)" }}>
                  {readinessBatchSummary.withOpenPath} 个有入口；其余走 Chat、任务、记忆或知识。
                </div>
              </div>
              <div className="rounded-md border p-3" style={{ borderColor: readinessBatchSummary.highRisk > 0 ? "var(--yunque-danger-border)" : "var(--yunque-warning-border)", background: readinessBatchSummary.highRisk > 0 ? "var(--yunque-danger-soft)" : "var(--yunque-warning-soft)" }}>
                <div className="text-xs font-medium" style={{ color: "var(--yunque-text)" }}>边界提醒</div>
                <div className="mt-1 text-[11px] leading-5" style={{ color: "var(--yunque-text-secondary)" }}>
                  高风险 {readinessBatchSummary.highRisk} · 计划态 {readinessBatchSummary.planOnly} · 审计阻塞 {readinessBatchSummary.auditBlocked}。
                </div>
              </div>
            </div>
            <div className="grid gap-2 md:grid-cols-2 xl:grid-cols-3">
              {readinessQueue.map((item) => {
                const readiness = packReadiness(item.manifest);
                const delivery = packDeliveryProfile(item.manifest);
                const deliveryStyle = deliveryToneStyle(delivery.tone);
                const guidance = packPolishGuidance(item.manifest, routeAuditEntries);
                const priority = packPolishPriority(item.manifest, routeAuditEntries);
                const risk = riskProfileForPack(item.manifest);
                const audit = packManifestAudit(item.manifest, routeAuditEntries);
                const permissionSummary = packPermissionSummary(item.manifest);
                const queueReason = readiness.missing.length > 0
                  ? `还缺：${readiness.missing.join("、")}`
                  : `交付状态：${delivery.label}。${delivery.nextStep}`;
                return (
                  <div key={item.manifest.id} className="rounded-md border p-3" style={{ borderColor: "var(--yunque-border)", background: "var(--yunque-surface)" }}>
                    <div className="flex items-start justify-between gap-2">
                      <div className="min-w-0">
                        <div className="truncate text-sm font-medium" style={{ color: "var(--yunque-text)" }}>{item.manifest.name}</div>
                        <div className="mt-1 truncate text-[11px]" style={{ color: "var(--yunque-text-muted)" }}>{item.sourceLabel}</div>
                      </div>
                      <div className="flex shrink-0 flex-wrap justify-end gap-1">
                        <Chip size="sm" color={priority.level === "P0" ? "danger" : priority.level === "P1" ? "warning" : "default"}>{priority.level}</Chip>
                        <Chip size="sm" color={readiness.level === "needs_entry" ? "danger" : readiness.level === "needs_context" ? "warning" : "success"}>{readiness.label}</Chip>
                        <Chip size="sm" variant="soft" color={delivery.tone === "danger" ? "danger" : delivery.tone === "warning" ? "warning" : delivery.tone === "primary" ? "accent" : "success"}>{delivery.label}</Chip>
                        <Chip size="sm" color={risk.level === "high" ? "danger" : risk.level === "medium" ? "warning" : "success"}>{risk.label}</Chip>
                      </div>
                    </div>
                    <div className="mt-2 text-xs leading-5" style={{ color: "var(--yunque-text-secondary)" }}>
                      {queueReason}
                    </div>
                    <div
                      className="mt-2 rounded-md border p-2 text-[11px] leading-5"
                      style={{
                        borderColor: risk.requiresAuthorization ? "var(--yunque-danger-border)" : "var(--yunque-accent-border)",
                        background: risk.requiresAuthorization ? "var(--yunque-danger-soft)" : "var(--yunque-accent-soft)",
                        color: "var(--yunque-text-secondary)",
                      }}
                    >
                      <div><span className="font-medium" style={{ color: "var(--yunque-text)" }}>来源：</span>{item.sourceLabel}{item.packageUrl ? ` · ${item.packageUrl}` : ""}</div>
                      <div><span className="font-medium" style={{ color: "var(--yunque-text)" }}>权限：</span>{permissionSummary}</div>
                      <div><span className="font-medium" style={{ color: "var(--yunque-text)" }}>先做：</span>{priority.level === "P0" ? "先看详情确认是否缺入口或能力声明，再进工坊只读检查。" : risk.requiresAuthorization ? "先看详情复查权限和回滚，再进入工坊打磨边界。" : "先按入口或主路径复验；仍说不清楚时再交给小羽打磨。"}</div>
                    </div>
                    {audit.issues.length > 0 && (
                      <div
                        className="mt-2 rounded-md border p-2 text-[11px] leading-5"
                        style={{
                          borderColor: audit.level === "blocked" ? "var(--yunque-danger-border)" : "var(--yunque-warning-border)",
                          background: audit.level === "blocked" ? "var(--yunque-danger-soft)" : "var(--yunque-warning-soft)",
                          color: "var(--yunque-text-secondary)",
                        }}
                      >
                        <div className="font-medium" style={{ color: audit.level === "blocked" ? "var(--yunque-danger)" : "var(--yunque-warning)" }}>
                          Manifest 审计：{audit.label}
                        </div>
                        <div>{audit.issues.slice(0, 2).map((issue) => issue.label).join("、")}</div>
                      </div>
                    )}
                    <div className="mt-2 rounded-md border p-2 text-[11px] leading-5" style={{ borderColor: "var(--yunque-warning-border)", background: "var(--yunque-warning-soft)", color: "var(--yunque-text-secondary)" }}>
                      <div><span className="font-medium" style={{ color: "var(--yunque-text)" }}>{priority.label}：</span>{priority.reason}</div>
                      <div><span className="font-medium" style={{ color: "var(--yunque-text)" }}>为什么进队列：</span>{guidance.reason}</div>
                      <div><span className="font-medium" style={{ color: "var(--yunque-text)" }}>优先修改：</span>{guidance.firstEdit}</div>
                      <div><span className="font-medium" style={{ color: "var(--yunque-text)" }}>验收路径：</span>{guidance.verify}</div>
                    </div>
                    <div className="mt-2 flex flex-wrap gap-2">
                      <Link href={`/packs/detail?id=${encodeURIComponent(item.manifest.id)}`}>
                        <Button size="sm" variant={risk.requiresAuthorization ? "outline" : "ghost"}>权限与详情 <ShieldCheck size={14} /></Button>
                      </Link>
                      <Link href={packStudioHref(item.manifest, { packageUrl: item.packageUrl, sha256: item.sha256 })}>
                        <Button size="sm" variant="ghost">交给小羽打磨 <Wrench size={14} /></Button>
                      </Link>
                    </div>
                  </div>
                );
              })}
            </div>
            <div className="mt-3">
              {renderPagination(
                "打磨队列",
                currentReadinessQueuePage,
                readinessQueuePageCount,
                readinessQueueTotal,
                () => setReadinessQueuePage((value) => Math.max(1, value - 1)),
                () => setReadinessQueuePage((value) => Math.min(readinessQueuePageCount, value + 1)),
                READINESS_QUEUE_PAGE_SIZE,
              )}
            </div>
          </div>
        )}

          </>
        )}

        {advancedVisible && !maintenanceMode && (
          <div className="mt-4 flex items-center gap-3">
            <Button size="sm" variant="ghost" onPress={() => setShowAdvanced(false)}>
              <ChevronUp size={14} /> 隐藏维护视图
            </Button>
          </div>
        )}

        <Card className="mt-4 p-4" style={{ background: "var(--yunque-surface)", border: "1px solid var(--yunque-border)" }}>
          <div className="flex flex-wrap items-center justify-between gap-3">
            <div>
              <div className="text-sm font-semibold" style={{ color: "var(--yunque-text)" }}>
                当前匹配 {totalMatches} 个
              </div>
              <div className="mt-1 text-xs" style={{ color: "var(--yunque-text-muted)" }}>
                已安装 {filteredInstalledPacks.length} 个 · 官方源 {filteredReleaseEntries.length} 个 · 私有源 {filteredPrivateCatalogEntries.length} 个
              </div>
            </div>
            <Button
              size="sm"
              variant="outline"
              onPress={() => setFiltersOpen(true)}
            >
              <SlidersHorizontal size={14} /> 找能力包
            </Button>
          </div>
          {!filtersOpen && (
            <div className="mt-3 flex flex-wrap items-center gap-2">
              {activeFilters.length === 0 ? (
                <Chip size="sm" variant="soft">未启用筛选</Chip>
              ) : activeFilters.map((filter) => (
                <Button key={filter.key} size="sm" variant="ghost" onPress={filter.clear} aria-label={filter.clearLabel}>
                  <span className="text-xs">{filter.label}</span>
                  <X size={13} />
                </Button>
              ))}
            </div>
          )}
          {(installFilter === "available" || sourceFilter === "official" || sourceFilter === "private") && (
            <div className="mt-3 rounded-md border px-3 py-2 text-xs leading-5" style={{ borderColor: "var(--yunque-accent-border)", background: "var(--yunque-accent-soft)", color: "var(--yunque-text-secondary)" }}>
              包含可安装来源；展开看官方、私有或本地。
            </div>
          )}
        </Card>

        <Modal.Backdrop isOpen={filtersOpen} onOpenChange={setFiltersOpen} variant="blur">
          <Modal.Container scroll="inside" size="lg" placement="top">
            <Modal.Dialog className="sm:max-w-[760px]">
              <Modal.CloseTrigger />
              <Modal.Header className="gap-3">
                <Modal.Heading>筛选能力包</Modal.Heading>
                <div className="flex flex-wrap items-center gap-2">
                  <Chip size="sm" variant="soft">结果 {totalMatches} 个</Chip>
                  <Chip size="sm" variant="soft">已安装 {filteredInstalledPacks.length}</Chip>
                  {(filteredReleaseEntries.length + filteredPrivateCatalogEntries.length) > 0 && (
                    <Chip size="sm" color="accent">可安装 {filteredReleaseEntries.length + filteredPrivateCatalogEntries.length}</Chip>
                  )}
                </div>
              </Modal.Header>
              <Modal.Body className="space-y-5">
                <div className="grid gap-3 lg:grid-cols-[minmax(220px,1fr)_auto]">
                  <TextField value={query} onChange={setQuery} fullWidth>
                    <Label>找能力包</Label>
                    <Input placeholder="搜索名称、用途、权限、能力、入口" />
                  </TextField>
                  <Button variant="outline" className="self-end" onPress={resetFilters}>
                    <Search size={14} aria-hidden /> 重置
                  </Button>
                </div>

                <div className="rounded-lg border p-3" style={{ borderColor: "var(--yunque-accent-border)", background: "var(--yunque-accent-soft)" }}>
                  <div className="flex flex-wrap items-start justify-between gap-3">
                    <div>
                      <div className="text-xs font-medium" style={{ color: "var(--yunque-text)" }}>当前建议</div>
                      <div className="mt-1 text-xs leading-5" style={{ color: "var(--yunque-text-secondary)" }}>{currentViewAdvice}</div>
                    </div>
                    {currentViewAction.kind === "anchor" && currentViewAction.href ? (
                      <a href={currentViewAction.href}>
                        <Button size="sm" variant="outline" onPress={() => setFiltersOpen(false)}>
                          {currentViewAction.label} <ArrowRight size={14} aria-hidden />
                        </Button>
                      </a>
                    ) : currentViewAction.kind === "link" && currentViewAction.href ? (
                      <Link href={currentViewAction.href}>
                        <Button size="sm" variant="outline" onPress={() => setFiltersOpen(false)}>
                          {currentViewAction.label} <ArrowRight size={14} aria-hidden />
                        </Button>
                      </Link>
                    ) : (
                      <Button size="sm" variant="outline" onPress={currentViewAction.onPress}>
                        {currentViewAction.label}
                      </Button>
                    )}
                  </div>
                </div>

                <div className="grid gap-4 md:grid-cols-3">
                  {renderFilterGroup("状态", [
                    ["all", "全部"],
                    ["installed", "已安装"],
                    ["enabled", "已启用"],
                    ["disabled", "已禁用"],
                    ["available", "可安装"],
                  ], installFilter, (value) => setInstallFilter(value as InstallFilter))}
                  {renderFilterGroup("来源信任", [
                    ["all", "全部"],
                    ["installed", "已安装"],
                    ["official", "官方源"],
                    ["private", "私有源"],
                  ], sourceFilter, (value) => setSourceFilter(value as SourceFilter))}
                  {renderFilterGroup("类型", [
                    ["all", "全部"],
                    ["actionable", "可用"],
                    ["infrastructure", "基础"],
                    ["experimental", "实验"],
                  ], kindFilter, (value) => setKindFilter(value as KindFilter))}
                </div>

                <div className="grid gap-2 md:grid-cols-3">
                  <FilterInsightCard
                    label="已安装"
                    value={`${filteredInstalledPacks.length} 个`}
                    detail="已经进入本地运行面；优先看启用状态、入口和权限边界。"
                  />
                  <FilterInsightCard
                    label="官方源"
                    value={`${filteredReleaseEntries.length} 个`}
                    detail="来自配置的发布源；安装前先看版本、摘要和回滚路径。"
                  />
                  <FilterInsightCard
                    label="私有源"
                    value={`${filteredPrivateCatalogEntries.length} 个`}
                    detail="来自团队或本地 catalog；适合只读检查后再启用。"
                  />
                </div>

                <div className="flex flex-wrap items-center gap-2" aria-label="当前筛选">
                  {activeFilters.length === 0 ? (
                    <Chip size="sm" variant="soft">未启用筛选</Chip>
                  ) : activeFilters.map((filter) => (
                    <Button key={filter.key} size="sm" variant="ghost" onPress={filter.clear} aria-label={filter.clearLabel}>
                      <span className="text-xs">{filter.label}</span>
                      <X size={13} aria-hidden />
                    </Button>
                  ))}
                </div>

                <Disclosure isExpanded={advancedFiltersOpen} onExpandedChange={setAdvancedFiltersOpen}>
                  <Disclosure.Heading>
                    <Button slot="trigger" size="sm" variant="ghost">
                      更多筛选
                      <Disclosure.Indicator />
                    </Button>
                  </Disclosure.Heading>
                  <Disclosure.Content>
                    <Disclosure.Body className="mt-3 space-y-4 rounded-lg border p-3" style={{ borderColor: "var(--yunque-border)", background: "var(--yunque-bg-muted)" }}>
                      <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-4">
                        {renderFilterGroup("风险", [
                          ["all", "全部"],
                          ["low", "低"],
                          ["medium", "留意"],
                          ["high", "授权"],
                        ], riskFilter, (value) => setRiskFilter(value as RiskFilter))}
                        {renderFilterGroup("稳定性", [
                          ["all", "全部"],
                          ["stable", "正式"],
                          ["beta", "测试"],
                          ["alpha", "开发中"],
                        ], stabilityFilter, (value) => setStabilityFilter(value as StabilityFilter))}
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
                      <div className="grid gap-2 md:grid-cols-3">
                        <FilterInsightCard
                          label="来源构成"
                          value={`已安装 ${filteredInstalledPacks.length} · 官方 ${filteredReleaseEntries.length} · 私有 ${filteredPrivateCatalogEntries.length}`}
                          detail="用于判断是直接启用、从发布源安装，还是先检查团队源。"
                        />
                        <FilterInsightCard
                          label="交付构成"
                          value={`可交付 ${visibleDeliveryStats.ready} · 后台 ${visibleDeliveryStats.support} · 实验 ${visibleDeliveryStats.plan_only} · 待打磨 ${visibleDeliveryStats.needs_meat}`}
                          detail="用于判断这批结果能否直接给用户验证。"
                        />
                        <FilterInsightCard
                          label="当前视图"
                          value={`共 ${totalMatches} 个 · 可用 ${visibleKindStats.actionable} · 基础 ${visibleKindStats.infrastructure} · 实验 ${visibleKindStats.experimental}`}
                          detail="用于快速决定下一步打开、安装、授权或打磨。"
                        />
                      </div>
                    </Disclosure.Body>
                  </Disclosure.Content>
                </Disclosure>
              </Modal.Body>
              <Modal.Footer>
                <Button variant="secondary" onPress={resetFilters}>重置</Button>
                <Button className="btn-accent" slot="close" onPress={() => setFiltersOpen(false)}>完成</Button>
              </Modal.Footer>
            </Modal.Dialog>
          </Modal.Container>
        </Modal.Backdrop>
      </div>

      <div className="p-5 space-y-4">
        <section className="section-card rounded-lg border p-4" style={{ borderColor: "var(--yunque-border)", background: "var(--yunque-surface)" }}>
          <Disclosure isExpanded={sourcesOpen} onExpandedChange={setSourcesOpen}>
            <Disclosure.Heading>
              <Button slot="trigger" variant="ghost" className="flex h-auto w-full justify-between gap-3 px-0 py-0 text-left">
                <div className="flex items-start gap-2">
                  <Store size={17} style={{ color: "var(--yunque-accent)" }} />
                  <div>
                    <div className="text-sm font-semibold" style={{ color: "var(--yunque-text)" }}>
                      {sourcesOpen ? "收起来源与安装" : "展开来源与安装"}
                    </div>
                    <div className="mt-1 text-xs" style={{ color: "var(--yunque-text-muted)" }}>
                      查看官方源、私有源和本地安装入口；当前可安装 {filteredReleaseEntries.length + filteredPrivateCatalogEntries.length} 个。
                    </div>
                  </div>
                </div>
                <Disclosure.Indicator />
              </Button>
            </Disclosure.Heading>
          </Disclosure>
        </section>

        <div hidden={!sourcesOpen} className="space-y-4">
        <section className="space-y-3">
          <div className="flex items-center justify-between gap-3 mb-3">
            <div className="flex items-start gap-2">
              <Store size={17} style={{ color: "var(--yunque-accent)" }} />
              <div>
                <div className="text-sm font-semibold" style={{ color: "var(--yunque-text)" }}>官方源</div>
                <div className="text-xs mt-1" style={{ color: "var(--yunque-text-muted)" }}>
                  可信发布源，先看用途、入口、权限。
                </div>
              </div>
            </div>
            <Button size="sm" variant="ghost" onPress={refreshReleaseCatalog} isDisabled={releaseLoading}>
              {releaseLoading ? <Spinner size="sm" /> : <RotateCcw size={14} />}
              刷新
            </Button>
          </div>

          <div className="mb-4 rounded-md border p-3" style={{ borderColor: "var(--yunque-border)", background: "var(--yunque-bg-muted)" }}>
            <button
              type="button"
              className="flex w-full items-center justify-between gap-3 text-left text-xs font-medium"
              style={{ color: "var(--yunque-text)" }}
              aria-expanded={officialDiagnosticsOpen}
              onClick={() => setOfficialDiagnosticsOpen((value) => !value)}
            >
              来源与安装诊断
              {officialDiagnosticsOpen ? <ChevronUp size={14} /> : <ChevronDown size={14} />}
            </button>
            {officialDiagnosticsOpen && (
              <div className="mt-3 space-y-3">
                <div className="grid grid-cols-1 md:grid-cols-2 gap-3">
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
                <div className="rounded-md border p-3" style={{ borderColor: "var(--yunque-accent-border)", background: "var(--yunque-accent-soft)" }}>
                  <div className="text-xs font-medium" style={{ color: "var(--yunque-text)" }}>安装失败怎么处理</div>
                  <div className="mt-2 grid grid-cols-1 md:grid-cols-3 gap-2">
                    {INSTALL_TROUBLESHOOTING.slice(0, 3).map((item) => {
                      const style = checklistToneStyle(item.tone);
                      return (
                        <div key={item.key} className="rounded-md border p-2" style={{ borderColor: style.borderColor, background: style.background }}>
                          <div className="text-[11px] font-medium" style={{ color: style.color }}>{item.label}</div>
                          <div className="mt-1 text-[11px] leading-4" style={{ color: "var(--yunque-text-muted)" }}>{item.detail}</div>
                        </div>
                      );
                    })}
                  </div>
                </div>
              </div>
            )}
          </div>

          {releaseLoading ? (
            <div className="flex items-center gap-2 text-xs py-6" style={{ color: "var(--yunque-text-muted)" }}>
              <Spinner size="sm" /> 正在读取发布内容
            </div>
          ) : filteredReleaseEntries.length > 0 ? (
            <div className="space-y-3">
              {renderGroupedInstallable(pagedReleaseEntries, (entry) => renderInstallableCard(entry.manifest, {
                key: entry.package_url,
                source: entry.release_tag || sourceName(entry.release_url),
                sourceLabel: releaseSourceLabel(entry),
                sourceDetail: entry.package_url || entry.release_url,
                size: formatBytes(entry.size_bytes),
                action: catalogActionForEntry(entry),
                busyKey: `install:${entry.package_url}`,
                onInstall: () => installRelease(entry),
                packageUrl: typeof entry.package_url === "string" ? entry.package_url : undefined,
                sha256: typeof entry.sha256 === "string" ? entry.sha256 : undefined,
              }))}
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

          <div className="mb-4 rounded-md border p-3" style={{ borderColor: "var(--yunque-border)", background: "var(--yunque-bg-muted)" }}>
            <button
              type="button"
              className="flex w-full items-center justify-between gap-3 text-left text-xs font-medium"
              style={{ color: "var(--yunque-text)" }}
              aria-expanded={privateDiagnosticsOpen}
              onClick={() => setPrivateDiagnosticsOpen((value) => !value)}
            >
              私有源诊断
              {privateDiagnosticsOpen ? <ChevronUp size={14} /> : <ChevronDown size={14} />}
            </button>
            {privateDiagnosticsOpen && (
              <div className="mt-3 space-y-3">
                {(catalog.source_reports?.length ?? 0) > 0 && (
                  <div className="grid grid-cols-1 md:grid-cols-2 gap-3">
                    {catalog.source_reports?.map((report) => (
                      <div key={report.source} className="rounded-md p-3 border" style={{ borderColor: "var(--yunque-border)" }}>
                        <div className="flex items-center gap-2 text-sm" style={{ color: report.ok ? "var(--yunque-success)" : "var(--yunque-warning)" }}>
                          {report.ok ? <ShieldCheck size={14} /> : <ShieldAlert size={14} />}
                          <span className="font-medium">{report.ok ? "源可用" : "源需要处理"}</span>
                        </div>
                        <div className="text-[11px] font-mono break-all mt-1" style={{ color: "var(--yunque-text-muted)" }}>{report.source}</div>
                        <div className="text-xs mt-2" style={{ color: "var(--yunque-text-muted)" }}>
                          声明 {report.manifest_count} 个，匹配 {report.matched_entries} 个
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
                <div className="rounded-md border p-3" style={{ borderColor: "var(--yunque-warning-border)", background: "var(--yunque-warning-soft)" }}>
                  <div className="text-xs font-medium" style={{ color: "var(--yunque-text)" }}>私有源安装前确认</div>
                  <div className="mt-2 grid grid-cols-1 md:grid-cols-2 gap-2">
                    {INSTALL_TROUBLESHOOTING.slice(1).map((item) => {
                      const style = checklistToneStyle(item.tone);
                      return (
                        <div key={item.key} className="rounded-md border p-2" style={{ borderColor: style.borderColor, background: style.background }}>
                          <div className="text-[11px] font-medium" style={{ color: style.color }}>{item.label}</div>
                          <div className="mt-1 text-[11px] leading-4" style={{ color: "var(--yunque-text-muted)" }}>{item.detail}</div>
                        </div>
                      );
                    })}
                  </div>
                </div>
              </div>
            )}
          </div>

          {catalogLoading ? (
            <div className="flex items-center gap-2 text-xs py-6" style={{ color: "var(--yunque-text-muted)" }}>
              <Spinner size="sm" /> 正在读取私有源
            </div>
          ) : filteredPrivateCatalogEntries.length > 0 ? (
            <div className="space-y-3">
              {renderGroupedInstallable(pagedPrivateCatalogEntries, (entry) => renderInstallableCard(entry.manifest, {
                key: entry.manifest.id,
                source: [entry.source, entry.manifest_path, entry.manifest_url, entry.package_url].find((value): value is string => typeof value === "string"),
                sourceLabel: privateSourceLabel(entry),
                sourceDetail: [entry.package_url, entry.manifest_url, entry.manifest_path, entry.source].find((value): value is string => typeof value === "string"),
                action: catalogActionForEntry(entry),
                busyKey: `install:${entry.manifest.id}`,
                onInstall: () => installCatalogEntry(entry),
                packageUrl: typeof entry.package_url === "string" ? entry.package_url : undefined,
                sha256: typeof entry.sha256 === "string" ? entry.sha256 : undefined,
              }))}
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
            onClick={() => setShowAdvanced((value) => !value)}
            className="flex items-center justify-between w-full"
          >
            <div className="flex items-start gap-2 text-left">
              <FolderInput size={17} style={{ color: "var(--yunque-text-muted)" }} />
              <div>
                <div className="text-sm font-semibold" style={{ color: "var(--yunque-text)" }}>本地高级安装</div>
                <div className="text-xs mt-1" style={{ color: "var(--yunque-text-muted)" }}>
                  开发者本地安装；普通用户用上方来源。
                </div>
              </div>
            </div>
            {advancedVisible ? <ChevronUp size={14} /> : <ChevronDown size={14} />}
          </button>
          {advancedVisible && (
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
        </div>

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
              可以清空搜索，或切换类型、状态、风险和来源信任。
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

      <ActionBar isOpen={selectedPackIds.size > 0} aria-label="批量操作能力包">
        <ActionBar.Prefix>
          <span className="text-sm" style={{ color: "var(--yunque-text)" }}>已选 {selectedPackIds.size} 个</span>
        </ActionBar.Prefix>
        <ActionBar.Content>
          <Button size="sm" className="btn-accent" isDisabled={busy === "batch:enable"} onPress={batchEnable}>
            <Power size={14} /> 批量启用
          </Button>
          <Button size="sm" variant="outline" isDisabled={busy === "batch:disable"} onPress={batchDisable}>
            <PackageX size={14} /> 批量禁用
          </Button>
        </ActionBar.Content>
        <ActionBar.Suffix>
          <Button size="sm" variant="ghost" onPress={() => setSelectedPackIds(new Set())}>取消</Button>
        </ActionBar.Suffix>
      </ActionBar>
    </div>
  );

  // Groups installable entries by manifest.metadata.group and renders each family
  // as a collapsible section, so the catalog reads as a handful of named groups
  // instead of a 60-card flat grid. A single group (or only "其他") renders as a
  // plain grid with no chrome. Operates on the current page's entries — pagination
  // stays unchanged, grouping just organizes what's already on screen.
  function renderGroupedInstallable<E extends { manifest: PackManifest }>(
    entries: E[],
    renderCard: (entry: E) => ReactNode,
  ): ReactNode {
    const order: string[] = [];
    const byGroup = new Map<string, E[]>();
    for (const entry of entries) {
      const key = packGroupKey(entry.manifest);
      if (!byGroup.has(key)) {
        byGroup.set(key, []);
        order.push(key);
      }
      byGroup.get(key)!.push(entry);
    }
    // Stable, readable order: named groups first (by label), ungrouped "其他" last.
    order.sort((a, b) => {
      if (a === PACK_GROUP_UNGROUPED) return 1;
      if (b === PACK_GROUP_UNGROUPED) return -1;
      return packGroupLabel(a).localeCompare(packGroupLabel(b), "zh-Hans-CN");
    });
    const meaningfulGroups = order.filter((k) => k !== PACK_GROUP_UNGROUPED);
    // Nothing to organize: one family (or only ungrouped) → plain grid, no chrome.
    if (meaningfulGroups.length <= 1) {
      return (
        <div className="grid grid-cols-1 lg:grid-cols-2 gap-4">
          {entries.map((entry) => renderCard(entry))}
        </div>
      );
    }
    return (
      <div className="space-y-3">
        {order.map((key) => {
          const groupEntries = byGroup.get(key)!;
          const collapsed = collapsedGroups.has(key);
          return (
            <Disclosure key={key} isExpanded={!collapsed} onExpandedChange={() => toggleGroupCollapsed(key)}>
              <Disclosure.Heading>
                <Button slot="trigger" size="sm" variant="ghost" className="w-full justify-start">
                  <Disclosure.Indicator />
                  <span className="text-sm font-semibold" style={{ color: "var(--yunque-text)" }}>{packGroupLabel(key)}</span>
                  <Chip size="sm" variant="soft">{groupEntries.length}</Chip>
                </Button>
              </Disclosure.Heading>
              <Disclosure.Content>
                <Disclosure.Body className="mt-2 grid grid-cols-1 lg:grid-cols-2 gap-4">
                  {groupEntries.map((entry) => renderCard(entry))}
                </Disclosure.Body>
              </Disclosure.Content>
            </Disclosure>
          );
        })}
      </div>
    );
  }

  function renderInstalledSection(title: string, sectionPacks: InstalledPack[], note?: string) {
    if (sectionPacks.length === 0) return null;
    const allIds = sectionPacks.map((p) => p.manifest.id);
    const selectedInSection = allIds.filter((id) => selectedPackIds.has(id));
    const allSelected = selectedInSection.length === allIds.length && allIds.length > 0;
    const someSelected = selectedInSection.length > 0 && !allSelected;
    const toggleAll = (checked: boolean) => {
      setSelectedPackIds((prev) => {
        const next = new Set(prev);
        if (checked) allIds.forEach((id) => next.add(id));
        else allIds.forEach((id) => next.delete(id));
        return next;
      });
    };
    return (
      <div>
        <div className="flex items-center gap-3 mb-3">
          <Checkbox isSelected={allSelected} isIndeterminate={someSelected} onChange={toggleAll} aria-label={`全选${title}`}>
            <Checkbox.Control><Checkbox.Indicator /></Checkbox.Control>
          </Checkbox>
          <h3 className="text-sm font-semibold" style={{ color: "var(--yunque-text)" }}>{title} · {sectionPacks.length} 个</h3>
          {note && <Chip size="sm" variant="soft" color="warning">{note}</Chip>}
        </div>
        {advancedVisible ? (
          // 维护/高级模式：保留全功能卡片（展开详情、交付状态、入口、信任条等）。
          <div className="grid grid-cols-1 lg:grid-cols-2 gap-4">
            {sectionPacks.map((pack) => renderPackCard(pack))}
          </div>
        ) : (
          // 普通模式：轻量行列表，降低对零基础用户的复杂度。
          <div className="flex flex-col rounded-lg border overflow-hidden" style={{ borderColor: "var(--yunque-border)" }}>
            {sectionPacks.map((pack) => renderPackRow(pack))}
          </div>
        )}
      </div>
    );
  }

  function renderPackRow(pack: InstalledPack) {
    const manifest = pack.manifest;
    const tone = statusTone(pack.status);
    const risk = riskProfileForPack(manifest);
    const openPath = packSafeOpenPath(manifest);
    const usability = packUsability(manifest);
    const selected = selectedPackIds.has(manifest.id);
    const enabled = pack.status === "enabled";
    return (
      <div
        key={manifest.id}
        data-pack-row={manifest.id}
        className="pack-row flex items-center gap-3 px-4 py-3 border-b last:border-b-0 transition-colors"
        style={{ borderColor: "var(--yunque-border)", background: selected ? "var(--yunque-accent-soft)" : "transparent" }}
      >
        <Checkbox isSelected={selected} onChange={(c) => togglePackSelected(manifest.id, c)} aria-label={`选择 ${manifest.name}`}>
          <Checkbox.Control><Checkbox.Indicator /></Checkbox.Control>
        </Checkbox>
        <div className="min-w-0 flex-1">
          <div className="flex items-center gap-2 flex-wrap">
            <Link href={`/packs/detail?id=${encodeURIComponent(manifest.id)}`} className="text-sm font-medium truncate hover:underline" style={{ color: "var(--yunque-text)" }}>{manifest.name}</Link>
            <Chip size="sm" variant="soft" color={tone.chip}>{tone.label}</Chip>
            {advancedVisible && risk.requiresAuthorization && (
              <Chip size="sm" variant="soft" color="danger">需要授权</Chip>
            )}
          </div>
          {manifest.description && (
            <div className="mt-0.5 text-xs truncate" style={{ color: "var(--yunque-text-muted)" }}>{manifest.description}</div>
          )}
        </div>
        <div className="flex items-center gap-2 shrink-0">
          {enabled ? (
            <Button size="sm" variant="outline" isDisabled={busy === `disable:${manifest.id}`} onPress={() => disable(manifest.id)}>
              <PackageX size={14} /> 禁用
            </Button>
          ) : (
            <Button size="sm" className="btn-accent" isDisabled={busy === `enable:${manifest.id}`} onPress={() => enable(manifest.id)}>
              <Power size={14} /> 启用
            </Button>
          )}
          {enabled && openPath && (
            <Link href={openPath}>
              <Button size="sm" variant="ghost"><ExternalLink size={14} /> {usability.primaryActionLabel || "打开"}</Button>
            </Link>
          )}
          <Link href={`/packs/detail?id=${encodeURIComponent(manifest.id)}`}>
            <Button size="sm" variant="ghost" className="px-3">详情 <ArrowRight size={14} /></Button>
          </Link>
        </div>
      </div>
    );
  }

  function renderInstallableCard(
    manifest: PackManifest,
    options: { key: string; source?: string; sourceLabel?: string; sourceDetail?: string; size?: string; action: ReturnType<typeof catalogActionForEntry>; busyKey: string; onInstall: () => void; packageUrl?: string; sha256?: string },
  ) {
    const badge = packStatusBadge(manifest.status);
    const risk = riskProfileForPack(manifest);
    const examples = packExamples(manifest);
    const permissionGroups = groupPackPermissions(manifest.backend?.permissions || []);
    const labels = capabilitySurfaceLabels(manifest);
    const usability = packUsability(manifest);
    const readiness = packReadiness(manifest);
    const delivery = packDeliveryProfile(manifest);
    const permissionSummary = packPermissionSummary(manifest);
    const deliveryStyle = deliveryToneStyle(delivery.tone);
    const usageLines = packUsageExplanation(manifest).slice(0, 3);
    const verificationSteps = packVerificationSteps(manifest).slice(0, 2);
    const installChecklist = packInstallChecklist(manifest, {
      sourceLabel: options.sourceLabel || options.source,
      installed: options.action.kind === "enable" || options.action.kind === "use",
      enabled: options.action.kind === "use",
    });
    const actionBusyKey = options.action.kind === "enable" ? `enable:${manifest.id}` : options.busyKey;
    const disabled = options.action.disabled || busy === actionBusyKey;
    const primaryPath = packSafeOpenPath(manifest);
    const sourceSummary = options.sourceLabel || options.source || "来源待确认";
    const runtimeTone: TrustTone = options.action.kind === "use" ? "safe" : options.action.kind === "enable" ? "accent" : "warning";
    const showStabilityBadge = shouldShowPackStatusBadge(manifest.status, advancedVisible);
    const showUsabilityChip = advancedVisible || usability.kind === "experimental";
    const visibleLabels = labels.slice(0, advancedVisible ? labels.length : 2);

    return (
      <Card
        key={options.key}
        className="section-card pack-card hover-lift transition-all duration-300"
        style={{
          background: "var(--yunque-surface-1)",
          border: "1px solid var(--glass-edge)",
          borderRadius: "1.25rem",
          overflow: "hidden"
        }}
      >
        <Card.Header className="flex flex-row gap-4 items-start p-5 pb-3">
          <div
            className="flex h-12 w-12 shrink-0 items-center justify-center rounded-xl"
            style={{
              background: "var(--yunque-bg-muted)",
              border: "1px solid var(--glass-edge)"
            }}
          >
            <PackageCheck size={24} style={{ color: "var(--yunque-accent)" }} />
          </div>
          <div className="flex flex-1 flex-col gap-1.5">
            <div className="flex items-center gap-2 flex-wrap">
              <Card.Title className="text-base font-semibold tracking-tight" style={{ color: "var(--yunque-text)" }}>
                {manifest.name}
              </Card.Title>
              {showStabilityBadge && (
                <Chip size="sm" variant="soft" color={badge.chip}>
                  {badge.label}
                </Chip>
              )}
              {showUsabilityChip && (
                <Chip size="sm" variant="soft" color="accent">
                  {usability.label}
                </Chip>
              )}
              {risk.requiresAuthorization && (
                <Chip size="sm" variant="soft" color="danger">
                  需要授权
                </Chip>
              )}
            </div>
            {manifest.description && (
              <Card.Description className="text-sm leading-relaxed line-clamp-2" style={{ color: "var(--yunque-text-secondary)" }}>
                {manifest.description}
              </Card.Description>
            )}
          </div>
        </Card.Header>

        <Card.Content className="px-5 py-2">
          <div className="flex items-center justify-between">
            <div className="flex flex-wrap gap-1.5">
              {visibleLabels.map((label) => (
                <Chip key={label} size="sm" variant="soft">
                  {label}
                </Chip>
              ))}
            </div>
            <Button
              size="sm"

              className="px-4 font-medium"
              style={{
                background: options.action.kind === "install" || options.action.kind === "update" || options.action.kind === "enable" ? "var(--yunque-text)" : "transparent",
                color: options.action.kind === "install" || options.action.kind === "update" || options.action.kind === "enable" ? "var(--yunque-bg)" : "var(--yunque-text)",
                border: options.action.kind === "use" ? "1px solid var(--yunque-border)" : "none"
              }}
              isDisabled={disabled}
              onPress={options.onInstall}
            >
              {options.action.kind === "enable" ? <Power size={14} /> : <Download size={14} />} {options.action.label}
            </Button>
          </div>
          <PackTrustStrip
            source={sourceSummary}
            showSourceHint={advancedVisible}
            runtime={options.action.kind === "use" ? "已可用" : options.action.label}
            runtimeTone={runtimeTone}
            risk={risk}
            delivery={delivery}
            readiness={readiness}
          />
        </Card.Content>

        <Card.Footer className="px-5 pb-4 pt-0">
            <div className="w-full">
              {advancedVisible && (
                <Button
                  size="sm"
                  variant="ghost"
                  className="w-full justify-between mt-3 bg-white/5 border border-white/5"
                  aria-expanded={expandedInstallableCards.has(options.key)}
                  onPress={() => toggleInstallableDetails(options.key)}
                >
                  {expandedInstallableCards.has(options.key) ? "收起详情" : "展开详情"}
                  {expandedInstallableCards.has(options.key) ? <ChevronUp size={14} /> : <ChevronDown size={14} />}
                </Button>
              )}
              <div hidden={!advancedVisible || !expandedInstallableCards.has(options.key)} className="mt-3 space-y-3">
                  <div className="pl-1 text-xs space-y-1.5" style={{ color: "var(--yunque-text-muted)" }}>
                      {(options.sourceLabel || options.source) && <div>来源：{options.sourceLabel || options.source}</div>}
                      {options.sourceDetail && <div>{options.sourceDetail}</div>}
                      {options.sha256 && <div>SHA256 {options.sha256}</div>}
                      {options.size && <div>大小：{options.size}</div>}
                      {permissionSummary && <div>{permissionSummary}</div>}
                  </div>
                  <div className="rounded-xl border p-3" style={{ borderColor: deliveryStyle.borderColor, background: deliveryStyle.background }}>
                    <div className="mb-1 flex flex-wrap items-center gap-2 text-sm font-medium" style={{ color: deliveryStyle.color }}>
                      <span>交付状态：{delivery.label}</span>
                      <Chip size="sm" color={readiness.level === "complete" ? "success" : readiness.level === "needs_context" ? "warning" : "danger"}>
                        {readiness.label}
                      </Chip>
                    </div>
                    <div className="text-sm leading-relaxed" style={{ color: "var(--yunque-text-secondary)" }}>{delivery.description}</div>
                    <div className="mt-2 text-xs" style={{ color: "var(--yunque-text-muted)" }}>下一步：{delivery.nextStep}</div>
                  </div>

                  {usageLines.length > 0 && (
                    <div className="rounded-xl p-3" style={{ background: "var(--yunque-surface-2)", border: "1px solid var(--yunque-border)" }}>
                      <div className="mb-2 text-sm font-medium" style={{ color: "var(--yunque-text)" }}>如何使用</div>
                      <ul className="space-y-1.5">
                        {usageLines.map((line) => (
                          <li key={line} className="flex items-start gap-2 text-sm leading-relaxed" style={{ color: "var(--yunque-text-secondary)" }}>
                            <span style={{ color: "var(--yunque-accent)" }}>•</span>
                            <span>{line}</span>
                          </li>
                        ))}
                      </ul>
                    </div>
                  )}

                  {installChecklist.length > 0 && (
                    <div className="rounded-xl p-3" style={{ background: "var(--yunque-bg-muted)", border: "1px solid var(--yunque-border)" }}>
                      <div className="mb-3 text-sm font-medium" style={{ color: "var(--yunque-text)" }}>安装前看这几点</div>
                      <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
                        {installChecklist.map((item) => {
                          const style = checklistToneStyle(item.tone);
                          return (
                            <div key={item.key} className="rounded-lg border p-3" style={{ borderColor: style.borderColor, background: style.background }}>
                              <div className="text-sm font-medium" style={{ color: style.color }}>{item.label}</div>
                              <div className="mt-1 text-xs leading-relaxed" style={{ color: "var(--yunque-text-muted)" }}>{item.detail}</div>
                            </div>
                          );
                        })}
                      </div>
                    </div>
                  )}

                  <div className="text-xs space-y-1.5" style={{ color: "var(--yunque-text-muted)" }}>
                    {usability.description && <div>{usability.description}</div>}
                    {usability.limitation && <div>当前限制：{usability.limitation}</div>}
                    {primaryPath && <div>默认入口：{primaryPath}</div>}
                    {permissionSummary && <div>权限提示：{permissionSummary}</div>}
                    <div className="pt-2 flex items-center gap-2 border-t border-white/5">
                      <Link href={`/packs/detail?id=${encodeURIComponent(manifest.id)}`}>
                        <Button size="sm" variant="ghost">查看详情 <ArrowRight size={14} /></Button>
                      </Link>
                      <Link href={packStudioHref(manifest, { packageUrl: options.packageUrl, sha256: options.sha256 })}>
                        <Button size="sm" variant="ghost">小羽优化 <Wrench size={14} /></Button>
                      </Link>
                    </div>
                  </div>
              </div>
            </div>
          </Card.Footer>
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
    const delivery = packDeliveryProfile(manifest);
    const permissionSummary = packPermissionSummary(manifest);
    const deliveryStyle = deliveryToneStyle(delivery.tone);
    const usageLines = packUsageExplanation(manifest).slice(0, 3);
    const verificationSteps = packVerificationSteps(manifest).slice(0, 2);
    const navItems = navItemsForPack(pack);
    const declaredOpenPath = usability.primaryActionPath || manifest.frontend?.menus?.[0]?.path || manifest.frontend?.routes?.[0]?.path;
    const openPath = packSafeOpenPath(manifest, declaredOpenPath);
    const entryHint = describePackEntry(usability.primaryActionLabel, declaredOpenPath);
    const pinHint = navItems.length > 0
      ? pack.status === "enabled"
        ? "可固定到侧栏，之后从侧栏或命令菜单直接打开。"
        : "启用后可固定到侧栏，减少下次寻找成本。"
      : "没有独立侧栏入口，通常在 Chat、任务或其他能力里自动生效。";
    const sourceSummary = pack.status === "enabled" ? "已安装 · 已启用" : "已安装";
    const runtimeTone: TrustTone = pack.status === "enabled" ? "safe" : pack.status === "disabled" ? "neutral" : "warning";
    const detailsExpanded = expandedInstalledCards.has(manifest.id);
    const detailsId = `installed-pack-details-${manifest.id}`;
    const showStabilityBadge = shouldShowPackStatusBadge(manifest.status, advancedVisible);
    const showUsabilityChip = advancedVisible || usability.kind === "experimental";
    const visibleLabels = labels.slice(0, advancedVisible ? labels.length : 2);

    return (
      <Card
        key={manifest.id}
        className="section-card pack-card hover-lift transition-all duration-300"
        style={{
          background: "var(--yunque-surface-1)",
          border: "1px solid var(--glass-edge)",
          borderRadius: "1.25rem",
          overflow: "hidden",
          opacity: pack.status === "disabled" ? 0.8 : 1
        }}
      >
        <Link href={`/packs/detail?id=${encodeURIComponent(manifest.id)}`} className="block cursor-pointer">
          <Card.Header className="flex flex-row gap-4 items-start p-5 pb-3">
            <div
              className="flex h-12 w-12 shrink-0 items-center justify-center rounded-xl"
              style={{
                background: "var(--yunque-bg-muted)",
                border: "1px solid var(--glass-edge)",
                filter: pack.status === "disabled" ? "grayscale(100%) opacity(50%)" : "none"
              }}
            >
              <PackageCheck size={24} style={{ color: pack.status === "disabled" ? "var(--yunque-text-muted)" : "var(--yunque-accent)" }} />
            </div>
            <div className="flex flex-1 flex-col gap-1.5">
              <div className="flex items-center gap-2 flex-wrap">
                <Card.Title className="text-base font-semibold tracking-tight" style={{ color: "var(--yunque-text)" }}>
                  {manifest.name}
                </Card.Title>
                <Chip size="sm" variant="soft" color={tone.chip}>{tone.label}</Chip>
                {showStabilityBadge && (
                  <Chip size="sm" variant="soft" color={badge.chip}>{badge.label}</Chip>
                )}
                {showUsabilityChip && (
                  <Chip size="sm" variant="soft" color="accent">{usability.label}</Chip>
                )}
                {risk.requiresAuthorization && (
                  <Chip size="sm" variant="soft" color="danger">
                    需要授权
                  </Chip>
                )}
              </div>
              {advancedVisible && <div className="text-[10px] mt-0.5 font-mono" style={{ color: "var(--yunque-text-muted)" }}>{manifest.id}</div>}
              {manifest.description && (
                <Card.Description className="text-sm leading-relaxed line-clamp-2" style={{ color: "var(--yunque-text-secondary)" }}>
                  {manifest.description}
                </Card.Description>
              )}
            </div>
          </Card.Header>

          <Card.Content className="px-5 py-2">
            <div className="flex items-center justify-between">
              <div className="flex flex-wrap gap-1.5">
                {visibleLabels.map((label) => (
                  <Chip key={label} size="sm" variant="soft">
                    {label}
                  </Chip>
                ))}
                {advancedVisible && openPath && (
                  <Chip size="sm" variant="soft">
                    入口 {openPath}
                  </Chip>
                )}
              </div>
            </div>
            {advancedVisible && (
              <PackTrustStrip
                source={sourceSummary}
                showSourceFact={advancedVisible}
                showSourceHint={advancedVisible}
                runtime={tone.label}
                runtimeTone={runtimeTone}
                risk={risk}
                delivery={delivery}
                readiness={readiness}
              />
            )}
          </Card.Content>
        </Link>

        {advancedVisible && (
          <Card.Footer className="px-5 pb-4 pt-0">
            <div className="mt-3 w-full space-y-3">
              <Button
                size="sm"
                variant="ghost"
                className="w-full justify-between bg-white/5 border border-white/5"
                aria-controls={detailsId}
                aria-expanded={detailsExpanded}
                onPress={() => toggleInstalledDetails(manifest.id)}
              >
                {detailsExpanded ? "收起详情" : "展开详情"}
                {detailsExpanded ? <ChevronUp size={14} aria-hidden={true} /> : <ChevronDown size={14} aria-hidden={true} />}
              </Button>
              <div id={detailsId} hidden={!detailsExpanded} className="space-y-3">
                <div className="rounded-xl border p-3" style={{ borderColor: deliveryStyle.borderColor, background: deliveryStyle.background }}>
                    <div className="mb-1 text-sm font-medium" style={{ color: deliveryStyle.color }}>交付状态：{delivery.label}</div>
                    <div className="text-sm leading-relaxed" style={{ color: "var(--yunque-text-secondary)" }}>{delivery.description}</div>
                    <div className="mt-2 text-xs" style={{ color: "var(--yunque-text-muted)" }}>下一步：{delivery.nextStep}</div>
                </div>
                {readiness.missing.length > 0 && (
                  <div className="rounded-xl border p-3 text-sm leading-relaxed" style={{ borderColor: "var(--yunque-warning-border)", background: "var(--yunque-warning-soft)", color: "var(--yunque-text-secondary)" }}>
                    可用性体检：还缺 {readiness.missing.join("、")}。可以交给小羽打磨用途、入口或使用说明。
                  </div>
                )}

                {usageLines.length > 0 && (
                  <div className="rounded-xl p-3" style={{ background: "var(--yunque-surface-2)", border: "1px solid var(--yunque-border)" }}>
                    <div className="mb-2 text-sm font-medium" style={{ color: "var(--yunque-text)" }}>如何使用</div>
                    <ul className="space-y-1.5">
                      {usageLines.map((line) => (
                        <li key={line} className="flex items-start gap-2 text-sm leading-relaxed" style={{ color: "var(--yunque-text-secondary)" }}>
                          <span style={{ color: "var(--yunque-accent)" }}>•</span>
                          <span>{line}</span>
                        </li>
                      ))}
                    </ul>
                  </div>
                )}

                <div className="rounded-xl p-3" style={{ background: "var(--yunque-success-soft)", border: "1px solid var(--yunque-success-border)" }}>
                  <div className="mb-2 text-sm font-medium" style={{ color: "var(--yunque-text)" }}>怎么判断有用</div>
                  <ul className="space-y-1.5">
                    {verificationSteps.map((step) => (
                      <li key={step.key} className="flex items-start gap-2 text-sm leading-relaxed" style={{ color: "var(--yunque-text-secondary)" }}>
                        <span style={{ color: "var(--yunque-success)" }}>•</span>
                        <span>{step.label}：{step.detail}</span>
                      </li>
                    ))}
                  </ul>
                </div>

                <div className="rounded-xl p-3" style={{ background: "var(--yunque-accent-soft)", border: "1px solid var(--yunque-accent-border)" }}>
                  <div className="mb-2 text-sm font-medium" style={{ color: "var(--yunque-text)" }}>启用后去哪用</div>
                  <ul className="space-y-1.5">
                    <li className="flex items-start gap-2 text-sm leading-relaxed" style={{ color: "var(--yunque-text-secondary)" }}>
                      <span style={{ color: "var(--yunque-primary)" }}>•</span>
                      <span>主入口：{entryHint}</span>
                    </li>
                    <li className="flex items-start gap-2 text-sm leading-relaxed" style={{ color: "var(--yunque-text-secondary)" }}>
                      <span style={{ color: "var(--yunque-primary)" }}>•</span>
                      <span>固定方式：{pinHint}</span>
                    </li>
                  </ul>
                </div>

                <div className="text-xs space-y-1.5" style={{ color: "var(--yunque-text-muted)" }}>
                  {usability.description && <div>{usability.description}</div>}
                  {usability.limitation && <div>当前限制：{usability.limitation}</div>}
                  {permissionSummary && <div>权限提示：{permissionSummary}</div>}
                  <div className="pt-2 flex items-center gap-2 border-t border-white/5">
                    <Link href={packStudioHref(manifest)}>
                      <Button size="sm" variant="ghost">小羽优化 <Wrench size={14} /></Button>
                    </Link>
                  </div>
                </div>
              </div>
            </div>
          </Card.Footer>
        )}

        <div className="px-5 py-4 border-t" style={{ borderColor: "var(--glass-edge)", background: "rgba(0,0,0,0.1)" }}>
          <div className="flex items-center gap-2 flex-wrap">
            {pack.status === "enabled" ? (
              <Button size="sm" variant="outline" className="px-4" isDisabled={busy === `disable:${manifest.id}`} onPress={() => disable(manifest.id)}>
                <PackageX size={14} /> 禁用
              </Button>
            ) : (
              <Button size="sm" className="px-4" style={{ background: "var(--yunque-text)", color: "var(--yunque-bg)" }} isDisabled={busy === `enable:${manifest.id}`} onPress={() => enable(manifest.id)}>
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
                {isPackPinned(pack) ? "取消固定" : "固定侧栏"}
              </Button>
            )}
            {advancedVisible && (
              <Link href={packStudioHref(manifest)}>
                <Button size="sm" variant="ghost">
                  <Wrench size={14} /> 优化
                </Button>
              </Link>
            )}
            <div className="ml-auto flex items-center gap-3">
              {advancedVisible && (
                <div className="text-xs" style={{ color: "var(--yunque-text-muted)" }}>
                  v{manifest.version}
                </div>
              )}
              <Link href={`/packs/detail?id=${encodeURIComponent(manifest.id)}`}>
                <Button size="sm" variant="ghost" className="px-3">详情 <ArrowRight size={14} /></Button>
              </Link>
            </div>
          </div>
        </div>
      </Card>
    );
  }
}
