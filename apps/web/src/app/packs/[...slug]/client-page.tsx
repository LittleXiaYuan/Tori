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
import { packDeliveryProfile, packExamples, packFeatureFlags, packReadiness, packUsability, riskProfileForPack } from "@/lib/pack-presentation";

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

function deliveryColor(tone: ReturnType<typeof packDeliveryProfile>["tone"]): "success" | "default" | "warning" | "danger" {
  if (tone === "success") return "success";
  if (tone === "warning") return "warning";
  if (tone === "danger") return "danger";
  return "default";
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
      <div className="page-root space-y-5 animate-fade-in-up">
        <PageHeader icon={<Boxes size={20} />} title="能力包入口同步失败" description="无法从已启用能力包列表加载入口，请回能力包中心刷新状态。" />
        <Card className="section-card p-5 text-sm" style={{ color: "var(--yunque-danger)" }}>{error}</Card>
      </div>
    );
  }

  if (!match) {
    return (
      <div className="page-root space-y-5 animate-fade-in-up">
        <PageHeader icon={<PackageOpen size={20} />} title="能力包入口未启用" description="这个入口还没有在已启用能力包中声明，请先安装并启用对应能力。" />
        <Card className="section-card p-6 space-y-3">
          <div className="text-sm font-semibold" style={{ color: "var(--yunque-text)" }}>未找到可打开的能力包页面</div>
          <div className="text-xs" style={{ color: "var(--yunque-text-muted)" }}>
            当前路径 <code>{pathname}</code> 需要先安装并启用对应能力包。云雀不会为未启用能力暴露页面入口，避免把可选能力写死进主系统。
          </div>
          <Link href="/packs" className="btn-accent inline-flex w-fit items-center rounded-xl px-4 py-2 text-sm">返回能力包中心</Link>
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
  const risk = riskProfileForPack(manifest);
  const flags = packFeatureFlags(manifest);
  const examples = packExamples(manifest, 3);
  const usagePrompt = [
    `我正在使用云雀能力包：${manifest.name} (${manifest.id})。`,
    `请告诉我它现在能帮我做什么、适合放在哪个工作流里、我可以怎么下第一条指令。`,
    `能力包说明：${manifest.description || "暂无说明"}`,
    `可用性：${usability.label}；${usability.description}`,
    `交付状态：${delivery.label}；${delivery.description}`,
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

  return (
    <div className="page-root space-y-5 animate-fade-in-up">
      <PageHeader
        icon={<Boxes size={20} />}
        title={match.title || manifest.name}
        description={isIframeBundle
          ? "这个能力包提供独立界面，已在沙箱中动态加载。"
          : "这是能力包声明同步出来的通用入口；没有专属页面时，先展示用途、入口、权限和可继续打磨的路径。"}
        actions={<Link href={packCenterFocusHref(manifest.id)} className="inline-flex items-center rounded-xl px-4 py-2 text-sm" style={{ border: "1px solid var(--yunque-border)", color: "var(--yunque-text)" }}>回中心定位</Link>}
      />

      <Card className="section-card p-5">
        <div className="flex flex-wrap items-start justify-between gap-4">
          <div className="min-w-0 max-w-3xl">
            <div className="mb-2 flex flex-wrap items-center gap-2">
              <Chip size="sm" color={usability.kind === "experimental" ? "warning" : usability.kind === "actionable" ? "success" : "default"}>
                {usability.label}
              </Chip>
              <Chip size="sm" color={readiness.level === "complete" ? "success" : readiness.level === "needs_context" ? "warning" : "danger"}>
                {readiness.label}
              </Chip>
              <Chip size="sm" color={deliveryColor(delivery.tone)}>
                {delivery.label}
              </Chip>
              <Chip size="sm" color={risk.level === "high" ? "danger" : risk.level === "medium" ? "warning" : "success"}>
                {risk.label}
              </Chip>
              {flags.isIframeBundle && <Chip size="sm" variant="soft">独立界面包</Chip>}
              {flags.hasWasm && <Chip size="sm" variant="soft">WASM</Chip>}
            </div>
            <div className="text-base font-semibold" style={{ color: "var(--yunque-text)" }}>这个能力包能帮你做什么</div>
            <div className="mt-2 text-sm leading-6" style={{ color: "var(--yunque-text-secondary)" }}>
              {manifest.description || usability.description}
            </div>
            <div className="mt-2 text-xs leading-5" style={{ color: "var(--yunque-text-muted)" }}>
              {usability.description}
              {usability.limitation ? ` 当前限制：${usability.limitation}` : ""}
            </div>
            <div className="mt-3 rounded-lg px-3 py-2 text-xs leading-5" style={{ background: "var(--yunque-bg-hover)", color: "var(--yunque-text-secondary)" }}>
              交付状态：{delivery.label}。{delivery.description} 下一步：{delivery.nextStep}
            </div>
            <div className="mt-3 grid gap-2 md:grid-cols-3">
              {(examples.length > 0 ? examples : ["还没有写清使用示例，可以交给小羽补齐。"]).map((example) => (
                <div key={example} className="rounded-lg px-3 py-2 text-xs leading-5" style={{ background: "var(--yunque-bg-hover)", color: "var(--yunque-text-secondary)" }}>
                  {example}
                </div>
              ))}
            </div>
            {readiness.missing.length > 0 && (
              <div className="mt-3 rounded-lg px-3 py-2 text-xs leading-5" style={{ background: "rgba(245,158,11,0.10)", color: "var(--yunque-warning)" }}>
                还缺：{readiness.missing.join("、")}。这不会阻止启用，但会让用户更难判断它该怎么用。
              </div>
            )}
          </div>
          <div className="flex w-full flex-wrap gap-2 lg:w-auto lg:max-w-xs">
            {usability.primaryActionPath && usability.primaryActionPath !== pathname && (
              <Link href={usability.primaryActionPath}>
                <Button size="sm" className="btn-accent">
                  <ExternalLink size={13} /> 打开入口
                </Button>
              </Link>
            )}
            <Link href={chatPromptHref(usagePrompt)}>
              <Button size="sm" variant="outline">
                <MessageSquare size={13} /> 问云雀怎么用
              </Button>
            </Link>
            <Link href={`/packs/detail?id=${encodeURIComponent(manifest.id)}`}>
              <Button size="sm" variant="outline">
                <ShieldCheck size={13} /> 权限与详情
              </Button>
            </Link>
            <Link href={studioHref}>
              <Button size="sm" variant="ghost">
                <Wrench size={13} /> 交给小羽补齐
              </Button>
            </Link>
          </div>
        </div>
      </Card>

      {isIframeBundle && (
        <Card className="section-card overflow-hidden p-0">
          <div className="grid gap-0 lg:grid-cols-[minmax(0,1fr)_320px]">
            <div className="p-5">
              <div className="flex flex-wrap items-center gap-2">
                <Chip size="sm" style={{ background: "rgba(56,189,248,0.12)", color: "#38bdf8" }}>
                  独立界面包
                </Chip>
                <Chip size="sm" variant="soft">沙箱隔离</Chip>
                <Chip size="sm" variant="soft">按声明路由调用</Chip>
              </div>
              <div className="mt-3 text-base font-semibold" style={{ color: "var(--yunque-text)" }}>
                这个能力界面来自能力包本身
              </div>
              <div className="mt-2 max-w-3xl text-sm leading-6" style={{ color: "var(--yunque-text-secondary)" }}>
                它不是写死在云雀主前端里的页面，而是随能力包一起下载的独立界面。
                启用后，云雀只负责加载沙箱、注入主题和转发白名单内的调用；界面内容、交互和升级都由该能力包提供。
              </div>
              {manifest.metadata?.limitation && (
                <div className="mt-3 rounded-lg p-3 text-xs leading-5" style={{ background: "rgba(245,158,11,0.10)", color: "var(--yunque-warning)" }}>
                  {manifest.metadata.limitation}
                </div>
              )}
            </div>
            <div className="p-5" style={{ background: "rgba(34,197,94,0.06)", borderLeft: "1px solid var(--yunque-border)" }}>
              <div className="mb-3 flex items-center gap-2 text-sm font-semibold" style={{ color: "var(--yunque-text)" }}>
                <ShieldCheck size={16} style={{ color: "var(--yunque-success)" }} />
                沙箱边界
              </div>
              <div className="space-y-2">
                {dlcBoundaryItems.map((item) => (
                  <div key={item} className="text-xs leading-5" style={{ color: "var(--yunque-text-secondary)" }}>
                    {item}
                  </div>
                ))}
              </div>
            </div>
          </div>
          <div className="px-5 pb-5">
            <PackDlcHost
              packId={manifest.id}
              entry={assets?.entry}
              title={match.title || manifest.name}
              allowedRoutes={(manifest.backend?.routeSpecs || []).map((r) => ({ method: r.method, path: r.path }))}
              allowedNavPaths={(manifest.frontend?.routes || []).map((r) => r.path)}
              allowedEventPaths={eventPathsFromPermissions(manifest.backend?.permissions)}
            />
          </div>
        </Card>
      )}

      <div className="grid grid-cols-1 md:grid-cols-3 gap-3">
        <Card className="section-card p-4">
          <div className="kpi-label">能力包</div>
          <div className="text-sm font-mono mt-1" style={{ color: "var(--yunque-text)" }}>{manifest.id}</div>
          <Chip size="sm" className="mt-3" style={{ background: "rgba(34,197,94,0.10)", color: "var(--yunque-success)" }}>{pack.status}</Chip>
        </Card>
        <Card className="section-card p-4">
          <div className="kpi-label">版本</div>
          <div className="kpi-value">{manifest.version}</div>
          <div className="kpi-sub">previous: {pack.previousVersion || "-"}</div>
        </Card>
        <Card className="section-card p-4">
          <div className="kpi-label">前端组件</div>
          <div className="text-sm font-mono mt-1" style={{ color: "var(--yunque-text)" }}>{match.component}</div>
          <div className="kpi-sub">asset: {assets?.entry || "-"}</div>
        </Card>
      </div>

      <div className="grid grid-cols-1 xl:grid-cols-2 gap-4">
        <Card className="section-card p-5 space-y-3">
          <div className="flex items-center gap-2 text-sm font-semibold" style={{ color: "var(--yunque-text)" }}>
            <Route size={15} /> 入口同步详情
          </div>
          <div className="text-xs space-y-2" style={{ color: "var(--yunque-text-muted)" }}>
            <div>当前路径：<code>{pathname}</code></div>
            <div>声明组件：<code>{match.component}</code></div>
            <div>菜单入口：{(manifest.frontend?.menus || []).map((menu) => <code key={menu.key} className="mx-1">{menu.label}:{menu.path}</code>)}</div>
            <div>后端路由：{(manifest.backend?.routeSpecs?.length ? manifest.backend.routeSpecs : manifest.backend?.routes || []).map((item) => <code key={typeof item === "string" ? item : `${item.method}:${item.path}`} className="mx-1">{formatBackendRouteSpec(item)}</code>)}</div>
          </div>
        </Card>

        <Card className="section-card p-5 space-y-3">
          <div className="flex items-center gap-2 text-sm font-semibold" style={{ color: "var(--yunque-text)" }}>
            <ExternalLink size={15} /> 界面资源与安装包
          </div>
          <div className="text-xs space-y-2" style={{ color: "var(--yunque-text-muted)" }}>
            <div>资源类型：<code>{assets?.type || "builtin"}</code></div>
            <div>资源入口：<code>{assets?.entry || distribution?.frontendUrl || "-"}</code></div>
            <div>远程前端：<code>{distribution?.frontendUrl || "-"}</code></div>
            <div>能力包：<code>{distribution?.packageUrl || "-"}</code></div>
            <div>SHA-256：<code>{distribution?.sha256 || pack.artifacts?.sha256 || "-"}</code></div>
          </div>
        </Card>
      </div>

      <Card className="section-card p-5 space-y-3">
        <div className="flex items-center gap-2 text-sm font-semibold" style={{ color: "var(--yunque-text)" }}>
          <TerminalSquare size={15} /> 开发者 SDK 能力
        </div>
        {entries.length > 0 ? (
          <div className="flex flex-wrap gap-2">
            {entries.map((entry) => (
              <code key={`${entry.language}:${entry.importPath}`} className="rounded-lg px-3 py-2 text-xs" style={{ background: "rgba(0,111,238,0.10)", color: "var(--yunque-accent)" }}>
                {packSdkImportSnippet(entry.language, entry.importPath)}
              </code>
            ))}
          </div>
        ) : (
          <div className="text-xs" style={{ color: "var(--yunque-text-muted)" }}>该能力包尚未声明开发者 SDK 入口。</div>
        )}
      </Card>

      <Card className="section-card p-5 flex items-start gap-3">
        <ShieldCheck size={16} className="mt-0.5 shrink-0" style={{ color: "var(--yunque-success)" }} />
        <div className="text-xs" style={{ color: "var(--yunque-text-muted)" }}>
          技术说明：这个页面只读取已启用能力包返回的 manifest，不把新功能硬编码进主导航。后续某个能力包提供独立前端包或专属页面时，可以覆盖同一路径；未覆盖前，仍可通过这个通用入口查看菜单、路由、界面资源和 SDK 能力。
        </div>
      </Card>
    </div>
  );
}
