"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { useEffect, useMemo, useState } from "react";
import { Card, Chip, Spinner } from "@heroui/react";
import { Boxes, ExternalLink, PackageOpen, Route, ShieldCheck, TerminalSquare } from "lucide-react";
import PageHeader from "@/components/page-header";
import { type InstalledPack } from "@/lib/api";
import { formatErrorMessage } from "@/lib/error-utils";
import { buildPackSdkEntrypoints, fetchEnabledPacks, findPackRouteBinding, packSdkImportSnippet } from "@/lib/pack-sync";

export default function PackRuntimeRoutePage() {
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
        setError(formatErrorMessage(err, "加载已启用 Pack 失败"));
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
        <PageHeader icon={<Boxes size={20} />} title="Pack 路由同步" description="从后端 enabled pack registry 加载前端入口失败。" />
        <Card className="section-card p-5 text-sm" style={{ color: "var(--yunque-danger)" }}>{error}</Card>
      </div>
    );
  }

  if (!match) {
    return (
      <div className="page-root space-y-5 animate-fade-in-up">
        <PageHeader icon={<PackageOpen size={20} />} title="Pack 路由未启用" description="该前端入口未在后端 enabled packs 的 frontend.routes 中声明。" />
        <Card className="section-card p-6 space-y-3">
          <div className="text-sm font-semibold" style={{ color: "var(--yunque-text)" }}>未找到可同步的 Pack 页面</div>
          <div className="text-xs" style={{ color: "var(--yunque-text-muted)" }}>
            当前路径 <code>{pathname}</code> 需要先安装并启用对应 pack。前端不会为未启用包暴露页面入口，避免继续把可选能力写死进主系统。
          </div>
          <Link href="/packs" className="btn-accent inline-flex w-fit items-center rounded-xl px-4 py-2 text-sm">返回增量包运行时</Link>
        </Card>
      </div>
    );
  }

  const { pack } = match;
  const manifest = pack.manifest;
  const distribution = match.distribution;
  const entries = match.sdk.length > 0 ? match.sdk : buildPackSdkEntrypoints(pack);
  const assets = match.assets;

  return (
    <div className="page-root space-y-5 animate-fade-in-up">
      <PageHeader
        icon={<Boxes size={20} />}
        title={match.title || manifest.name}
        description="这是由后端 enabled pack registry 同步出来的通用 Pack 页面。专属页面尚未随前端包加载时，先展示 manifest、资源入口和 SDK 调用面。"
        actions={<Link href="/packs" className="inline-flex items-center rounded-xl px-4 py-2 text-sm" style={{ border: "1px solid var(--yunque-border)", color: "var(--yunque-text)" }}>管理增量包</Link>}
      />

      <div className="grid grid-cols-1 md:grid-cols-3 gap-3">
        <Card className="section-card p-4">
          <div className="kpi-label">Pack</div>
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
            <Route size={15} /> Registry 同步入口
          </div>
          <div className="text-xs space-y-2" style={{ color: "var(--yunque-text-muted)" }}>
            <div>当前路径：<code>{pathname}</code></div>
            <div>声明组件：<code>{match.component}</code></div>
            <div>菜单入口：{(manifest.frontend?.menus || []).map((menu) => <code key={menu.key} className="mx-1">{menu.label}:{menu.path}</code>)}</div>
            <div>后端路由：{(manifest.backend?.routes || []).map((item) => <code key={item} className="mx-1">{item}</code>)}</div>
          </div>
        </Card>

        <Card className="section-card p-5 space-y-3">
          <div className="flex items-center gap-2 text-sm font-semibold" style={{ color: "var(--yunque-text)" }}>
            <ExternalLink size={15} /> UI 资源与增量包
          </div>
          <div className="text-xs space-y-2" style={{ color: "var(--yunque-text-muted)" }}>
            <div>资源类型：<code>{assets?.type || "builtin"}</code></div>
            <div>资源入口：<code>{assets?.entry || distribution?.frontendUrl || "-"}</code></div>
            <div>远程前端：<code>{distribution?.frontendUrl || "-"}</code></div>
            <div>增量包：<code>{distribution?.packageUrl || "-"}</code></div>
            <div>SHA-256：<code>{distribution?.sha256 || pack.artifacts?.sha256 || "-"}</code></div>
          </div>
        </Card>
      </div>

      <Card className="section-card p-5 space-y-3">
        <div className="flex items-center gap-2 text-sm font-semibold" style={{ color: "var(--yunque-text)" }}>
          <TerminalSquare size={15} /> SDK 调用能力
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
          <div className="text-xs" style={{ color: "var(--yunque-text-muted)" }}>该 pack 尚未声明 SDK 入口。</div>
        )}
      </Card>

      <Card className="section-card p-5 flex items-start gap-3">
        <ShieldCheck size={16} className="mt-0.5 shrink-0" style={{ color: "var(--yunque-success)" }} />
        <div className="text-xs" style={{ color: "var(--yunque-text-muted)" }}>
          这个页面只消费 <code>/v1/packs/enabled</code> 返回的 manifest，不把新功能硬编码进主导航。后续某个 pack 提供独立前端包或专属页面时，可以覆盖同一路径；未覆盖前，仍可通过此通用入口完成菜单、路由、UI 资源和 SDK 能力同步展示。
        </div>
      </Card>
    </div>
  );
}
