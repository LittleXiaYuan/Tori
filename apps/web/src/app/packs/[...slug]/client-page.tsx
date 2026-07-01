"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { useEffect, useMemo, useState } from "react";
import { Button, Card, Chip, Spinner } from "@heroui/react";
import { Boxes, ExternalLink, MessageSquare, PackageOpen, Route, ShieldCheck, TerminalSquare, Wrench } from "lucide-react";
import PageHeader from "@/components/page-header";
import { formatErrorMessage } from "@/lib/error-utils";
import type { InstalledPack } from "@/lib/pack-types";
import { buildPackSdkEntrypoints, fetchEnabledPacks, findPackRouteBinding, formatBackendRouteSpec, packSdkImportSnippet } from "@/lib/pack-sync";
import { PackDlcHost } from "@/lib/pack-dlc-host";
import { eventPathsFromPermissions } from "@/lib/pack-bridge";
import { chatPromptHref } from "@/lib/pack-action-links";
import { packBoundarySummary, packDeliveryProfile, packExamples, packFeatureFlags, packReadiness, packUsability, packVerificationSteps, riskProfileForPack } from "@/lib/pack-presentation";
import {
  PackHero,
  PackBoundaryGrid,
  PackStepsGrid,
  PackInfoAccordion,
  PackKeyValue,
  PackSectionTitle,
  type PackBoundaryItem,
  type Tone,
} from "@/components/packs/pack-page-kit";

const dlcBoundaryItems = [
  "独立界面拿不到云雀本地登录态或宿主 token。",
  "沙箱只允许脚本运行，不开放同源、弹窗或本机桌面能力。",
  "它只能调用自己声明过的后端路由。",
  "nav.push 只能跳转到该能力包声明的前端路径。",
  "越权调用会被拒绝并留下审计线索。",
];

function packCenterFocusHref(packId?: string): string {
  return packId ? `/packs?q=${encodeURIComponent(packId)}` : "/packs";
}

function displayDeliveryLabel(label?: string, level?: string): string {
  const value = label || level || "";
  if (value === "待补肉" || value === "needs_meat") return "需打磨";
  return value || "未知";
}

export default function PackRuntimeRouteClientPage() {
  const pathname = usePathname() || "/packs";
  const [packs, setPacks] = useState<InstalledPack[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    setLoading(true);
    fetchEnabledPacks()
      .then((res) => {
        if (cancelled) return;
        setPacks(res);
        setError(null);
      })
      .catch((err) => {
        if (cancelled) return;
        setError(formatErrorMessage(err, "加载已启用能力包失败"));
        setPacks([]);
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => { cancelled = true; };
  }, []);

  const match = useMemo(() => findPackRouteBinding(packs, pathname), [packs, pathname]);

  if (loading) {
    return <div className="flex items-center justify-center h-[60vh]"><Spinner size="lg" /></div>;
  }

  if (error) {
    return (
      <div className="page-root space-y-6 animate-fade-in-up">
        <PageHeader icon={<Boxes size={20} />} title="能力包入口同步失败" description="无法从已启用能力包列表加载入口，请回能力包中心刷新状态。" />
        <Card variant="secondary">
          <Card.Content className="text-sm text-danger">{error}</Card.Content>
        </Card>
      </div>
    );
  }

  if (!match) {
    return (
      <div className="page-root space-y-6 animate-fade-in-up">
        <PageHeader icon={<PackageOpen size={20} />} title="能力包入口未启用" description="这个入口还没有在已启用能力包中声明，请先安装并启用对应能力。" />
        <Card variant="secondary">
          <Card.Header>
            <Card.Title className="text-base">未找到可打开的能力包页面</Card.Title>
            <Card.Description className="text-sm leading-6 text-muted">
              当前路径 <code className="font-mono">{pathname}</code> 需要先安装并启用对应能力包。云雀不会为未启用能力暴露页面入口，避免把可选能力写死进主系统。
            </Card.Description>
          </Card.Header>
          <Card.Footer>
            <Link href="/packs"><Button className="btn-accent" size="sm"><Boxes size={14} /> 返回能力包中心</Button></Link>
          </Card.Footer>
        </Card>
      </div>
    );
  }

  const { pack } = match;
  const manifest = pack.manifest;
  const distribution = match.distribution;
  const entries = match.sdk.length > 0 ? match.sdk : buildPackSdkEntrypoints(pack);
  const assets = match.assets;
  const isIframeBundle = assets?.type === "iframe-bundle";
  const usability = packUsability(manifest);
  const readiness = packReadiness(manifest);
  const delivery = packDeliveryProfile(manifest);
  const deliveryLabel = displayDeliveryLabel(delivery.label, delivery.level);
  const risk = riskProfileForPack(manifest);
  const flags = packFeatureFlags(manifest);
  const examples = packExamples(manifest, 3);
  const verificationSteps = packVerificationSteps(manifest);
  const boundarySummary = packBoundarySummary(manifest);
  const usagePrompt = [
    `我正在使用云雀能力包：${manifest.name} (${manifest.id})。`,
    `请告诉我它现在能帮我做什么、适合放在哪个工作流里、我可以怎么下第一条指令。`,
    `能力包说明：${manifest.description || "暂无说明"}`,
    `可用性：${usability.label}；${usability.description}`,
    `交付状态：${deliveryLabel}；${delivery.description}`,
    `建议下一步：${delivery.nextStep}`,
    readiness.missing.length > 0 ? `当前还缺：${readiness.missing.join("、")}` : "当前体检：说明基本完整",
    usability.limitation ? `当前限制：${usability.limitation}` : "",
    examples.length > 0 ? `已有示例：${examples.join(" / ")}` : "",
    "请不要夸大实验能力；如果它只是后台支撑能力，请告诉我应该从 Chat、任务、记忆、知识或能力包详情哪里感知它。",
  ].filter(Boolean).join("\n");
  const studioGoal = delivery.level === "plan_only"
    ? `把 ${manifest.name} 从实验/计划能力打磨到用户能验证的路径：明确当前不执行什么、结果在哪里看、如何验证和回滚。`
    : delivery.level === "needs_meat"
      ? `让 ${manifest.name} 更像一个用户能直接理解和使用的能力包，优先补齐 ${readiness.missing.join("、") || "用途、入口、示例、权限边界和回滚说明"}。`
      : `让 ${manifest.name} 更像一个用户能直接理解和使用的能力包，补齐用途、入口、示例、权限边界和回滚说明。`;
  const studioHref = `/packs/studio?packId=${encodeURIComponent(manifest.id)}&goal=${encodeURIComponent(studioGoal)}`;

  // Keep only the two signals a user actually decides on: can I use it, and is
  // it risky. Readiness/delivery are polish-meta, surfaced via the studio path.
  const heroChips = (
    <>
      <Chip size="sm" color={usability.kind === "experimental" ? "warning" : usability.kind === "actionable" ? "success" : "default"}>{usability.label}</Chip>
      <Chip size="sm" color={risk.level === "high" ? "danger" : risk.level === "medium" ? "warning" : "success"}>{risk.label}</Chip>
      {flags.isIframeBundle && <Chip size="sm" variant="soft">独立界面包</Chip>}
      {flags.hasWasm && <Chip size="sm" variant="soft">WASM</Chip>}
    </>
  );

  // Terse: only name what's missing, and only when something is. No prose tail.
  const heroNote = readiness.missing.length > 0
    ? <span className="text-warning">还缺：{readiness.missing.join("、")}</span>
    : null;

  const heroActions = (
    <>
      {usability.primaryActionPath && usability.primaryActionPath !== pathname && (
        <Link href={usability.primaryActionPath}><Button size="sm" className="btn-accent"><ExternalLink size={14} /> 打开入口</Button></Link>
      )}
      <Link href={chatPromptHref(usagePrompt)}><Button size="sm" variant="outline"><MessageSquare size={14} /> 问云雀怎么用</Button></Link>
      <Link href={`/packs/detail?id=${encodeURIComponent(manifest.id)}`}><Button size="sm" variant="outline"><ShieldCheck size={14} /> 权限与详情</Button></Link>
      <Link href={studioHref}><Button size="sm" variant="ghost"><Wrench size={14} /> 交给小羽打磨</Button></Link>
    </>
  );

  const boundaryItems: PackBoundaryItem[] = boundarySummary.map((item) => ({
    key: item.key,
    label: item.label,
    detail: item.detail,
    tone: (item.tone === "danger" ? "danger" : item.tone === "warning" ? "warning" : "neutral") as Tone,
  }));

  const infoSections = [
    {
      key: "verify",
      icon: <Wrench size={16} />,
      title: "如何验收与继续打磨",
      body: (
        <div className="flex flex-col gap-3">
          <PackStepsGrid steps={verificationSteps.map((s) => ({ key: s.key, label: s.label, detail: s.detail }))} />
          <div className="text-sm leading-6 text-muted">
            <span className="font-semibold text-foreground">验收：</span>回中心看状态、详情复查权限{usability.primaryActionPath ? "，再打开入口复验。" : "。"}
          </div>
        </div>
      ),
    },
    {
      key: "sync",
      icon: <Route size={16} />,
      title: "入口同步详情",
      body: (
        <div className="flex flex-col gap-1">
          <PackKeyValue label="当前路径">{pathname}</PackKeyValue>
          <PackKeyValue label="声明组件">{match.component}</PackKeyValue>
          <div className="flex flex-wrap items-baseline gap-2 py-1">
            <span className="text-xs text-muted">菜单入口</span>
            <span className="flex flex-wrap gap-1">{(manifest.frontend?.menus || []).map((menu) => <code key={menu.key} className="font-mono text-xs text-foreground">{menu.label}:{menu.path}</code>)}</span>
          </div>
          <div className="flex flex-wrap items-baseline gap-2 py-1">
            <span className="text-xs text-muted">后端路由</span>
            <span className="flex flex-wrap gap-1">{(manifest.backend?.routeSpecs?.length ? manifest.backend.routeSpecs : manifest.backend?.routes || []).map((item) => <code key={typeof item === "string" ? item : `${item.method}:${item.path}`} className="font-mono text-xs text-foreground">{formatBackendRouteSpec(item)}</code>)}</span>
          </div>
        </div>
      ),
    },
    {
      key: "assets",
      icon: <ExternalLink size={16} />,
      title: "界面资源与安装包",
      body: (
        <div className="flex flex-col gap-1">
          <PackKeyValue label="资源类型">{assets?.type || "builtin"}</PackKeyValue>
          <PackKeyValue label="资源入口">{assets?.entry || distribution?.frontendUrl || "-"}</PackKeyValue>
          <PackKeyValue label="远程前端">{distribution?.frontendUrl || "-"}</PackKeyValue>
          <PackKeyValue label="能力包">{distribution?.packageUrl || "-"}</PackKeyValue>
          <PackKeyValue label="SHA-256">{distribution?.sha256 || pack.artifacts?.sha256 || "-"}</PackKeyValue>
        </div>
      ),
    },
    {
      key: "meta",
      icon: <Boxes size={16} />,
      title: "能力包元数据",
      body: (
        <div className="flex flex-col gap-1">
          <PackKeyValue label="能力包">{manifest.id}</PackKeyValue>
          <PackKeyValue label="版本">{manifest.version}</PackKeyValue>
          <PackKeyValue label="上一版本">{pack.previousVersion || "-"}</PackKeyValue>
          <PackKeyValue label="前端组件">{match.component}</PackKeyValue>
          <PackKeyValue label="状态">{pack.status}</PackKeyValue>
        </div>
      ),
    },
    {
      key: "sdk",
      icon: <TerminalSquare size={16} />,
      title: "开发者 SDK 能力",
      body: entries.length > 0 ? (
        <div className="flex flex-wrap gap-2">
          {entries.map((entry) => (
            <code key={`${entry.language}:${entry.importPath}`} className="rounded-lg bg-surface-secondary px-3 py-2 font-mono text-xs text-accent">
              {packSdkImportSnippet(entry.language, entry.importPath)}
            </code>
          ))}
        </div>
      ) : (
        <span className="text-muted">该能力包尚未声明开发者 SDK 入口。</span>
      ),
    },
  ];

  return (
    <div className="page-root space-y-6 animate-fade-in-up">
      <PageHeader
        icon={<Boxes size={20} />}
        title={match.title || manifest.name}
        description={isIframeBundle
          ? "这个能力包提供独立界面，已在沙箱中动态加载。"
          : "这是能力包声明同步出来的通用入口；没有专属页面时，先展示用途、入口、权限和可继续打磨的路径。"}
        actions={<Link href={packCenterFocusHref(manifest.id)}><Button size="sm" variant="outline"><Boxes size={14} /> 回中心定位</Button></Link>}
      />

      <PackHero
        chips={heroChips}
        title="这个能力包能帮你做什么"
        description={manifest.description || usability.description}
        note={heroNote}
        examples={examples}
        actions={heroActions}
      />

      <PackBoundaryGrid title="启用前边界" icon={<ShieldCheck size={15} />} items={boundaryItems} />

      {isIframeBundle && (
        <Card variant="secondary">
          <Card.Header className="gap-3">
            <div className="flex flex-wrap items-center gap-2">
              <Chip size="sm" variant="soft">独立界面包</Chip>
              <Chip size="sm" variant="soft">沙箱隔离</Chip>
              <Chip size="sm" variant="soft">按声明路由调用</Chip>
            </div>
            <Card.Title className="text-base">这个能力界面来自能力包本身</Card.Title>
            <Card.Description className="max-w-3xl text-sm leading-6 text-muted">
              它不是写死在云雀主前端里的页面，而是随能力包一起下载的独立界面。启用后，云雀只负责加载沙箱、注入主题和转发白名单内的调用；界面内容、交互和升级都由该能力包提供。
            </Card.Description>
          </Card.Header>
          <Card.Content className="flex flex-col gap-4">
            {manifest.metadata?.limitation && (
              <div className="rounded-xl bg-surface-secondary px-4 py-3 text-sm leading-6 text-warning">{manifest.metadata.limitation}</div>
            )}
            <div className="rounded-xl bg-surface-secondary px-4 py-3">
              <PackSectionTitle icon={<ShieldCheck size={15} />} tone="success">沙箱边界</PackSectionTitle>
              <div className="mt-2 flex flex-col gap-1.5 text-sm leading-6 text-muted">
                {dlcBoundaryItems.map((item) => <div key={item}>{item}</div>)}
              </div>
            </div>
            <PackDlcHost
              packId={manifest.id}
              entry={assets?.entry}
              title={match.title || manifest.name}
              allowedRoutes={(manifest.backend?.routeSpecs || []).map((r) => ({ method: r.method, path: r.path }))}
              allowedNavPaths={(manifest.frontend?.routes || []).map((r) => r.path)}
              allowedEventPaths={eventPathsFromPermissions(manifest.backend?.permissions)}
            />
          </Card.Content>
        </Card>
      )}

      <PackInfoAccordion sections={infoSections} />

      <Card variant="transparent">
        <Card.Content className="flex items-start gap-3 text-sm leading-6 text-muted">
          <ShieldCheck size={16} className="mt-0.5 shrink-0 text-success" />
          <span>技术说明：这个页面只读取已启用能力包返回的 manifest，不把新功能硬编码进主导航。后续某个能力包提供独立前端包或专属页面时，可以覆盖同一路径；未覆盖前，仍可通过这个通用入口查看菜单、路由、界面资源和 SDK 能力。</span>
        </Card.Content>
      </Card>
    </div>
  );
}
