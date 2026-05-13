"use client";

import { useMemo, useState } from "react";
import { Button, Card, Chip, Spinner, TextField, Input, Label } from "@heroui/react";
import {
  ArchiveRestore,
  Boxes,
  CheckCircle2,
  ClipboardCopy,
  DatabaseZap,
  Download,
  ExternalLink,
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
import { api, type InstalledPack } from "@/lib/api";
import { useApiData } from "@/lib/use-api-data";
import { formatErrorMessage } from "@/lib/error-utils";

const EXAMPLE_BACKUP_MANIFEST = "packs/examples/backup-pack/pack.json";

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

function statusTone(status: string): { label: string; color: string; bg: string } {
  if (status === "enabled") return { label: "已启用", color: "var(--yunque-success)", bg: "rgba(34,197,94,0.10)" };
  if (status === "disabled") return { label: "已禁用", color: "var(--yunque-text-muted)", bg: "rgba(255,255,255,0.05)" };
  return { label: status || "未知", color: "var(--yunque-warning)", bg: "rgba(245,158,11,0.12)" };
}

export default function PacksPage() {
  const { data, loading, refresh } = useApiData(async () => api.packsInstalled(), { packs: [], count: 0 });
  const { data: backendModulesData, loading: backendModulesLoading, refresh: refreshBackendModules } = useApiData(async () => api.packBackendModules(), { modules: [], count: 0 });
  const [manifestPath, setManifestPath] = useState(EXAMPLE_BACKUP_MANIFEST);
  const [manifestUrl, setManifestUrl] = useState("");
  const [busy, setBusy] = useState<string | null>(null);

  const packs = data?.packs || [];
  const backendModules = backendModulesData?.modules || [];
  const backendModuleByPack = useMemo(() => new Map(backendModules.map((module) => [module.pack_id, module])), [backendModules]);
  const stats = useMemo(() => ({
    total: packs.length,
    enabled: packs.filter((p) => p.status === "enabled").length,
    rollbackable: packs.filter((p) => p.manifest.update?.rollback).length,
    frontendMenus: packs.reduce((n, p) => n + (p.manifest.frontend?.menus?.length || 0), 0),
    backendModules: backendModules.length,
    backendRoutes: backendModules.reduce((n, m) => n + (m.routes?.length || 0), 0),
  }), [packs, backendModules]);

  const run = async (label: string, op: () => Promise<unknown>) => {
    setBusy(label);
    try {
      await op();
      showToast("Pack registry 已更新，前端菜单会跟随已启用包同步。", "success");
      await refresh();
      await refreshBackendModules();
    } catch (e) {
      showToast(formatErrorMessage(e, "Pack 操作失败"), "error");
    } finally {
      setBusy(null);
    }
  };

  const install = () => run("install", () => api.packInstall(manifestPath));
  const installFromURL = () => run("install-url", () => api.packInstallFromURL(manifestUrl));
  const enable = (id: string) => run(`enable:${id}`, () => api.packEnable(id));
  const disable = (id: string) => run(`disable:${id}`, () => api.packDisable(id));
  const rollback = (id: string) => run(`rollback:${id}`, () => api.packRollback(id));

  if (loading) {
    return <div className="flex items-center justify-center h-[60vh]"><Spinner size="lg" /></div>;
  }

  return (
    <div className="page-root space-y-5 animate-fade-in-up">
      <PageHeader
        icon={<Boxes size={20} />}
        title="增量包运行时"
        description="Pack Runtime 以后端 registry 为能力来源：安装、启用、禁用、回滚后，前端菜单和入口自动跟随已启用包同步。"
        onRefresh={() => { void refresh(); void refreshBackendModules(); }}
      />

      <div className="grid grid-cols-2 md:grid-cols-4 gap-3">
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
          <div className="kpi-label">后端模块 / 路由</div>
          <div className="kpi-value">{stats.backendModules}/{stats.backendRoutes}</div>
        </Card>
      </div>

      <Card className="section-card p-5 space-y-4">
        <div className="flex items-start justify-between gap-3">
          <div>
            <div className="text-sm font-semibold" style={{ color: "var(--yunque-text)" }}>安装本地 pack manifest</div>
            <div className="text-xs mt-1" style={{ color: "var(--yunque-text-muted)" }}>
              最小闭环先支持本地 manifest；后续下载源/签名校验可以继续沉淀到 Pack Runtime，而不是压进主系统。
            </div>
          </div>
          <Chip size="sm" style={{ background: "rgba(0,111,238,0.10)", color: "var(--yunque-accent)" }}>
            backend registry source-of-truth
          </Chip>
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
            <Button variant="outline" className="md:self-end" isDisabled={!manifestUrl || busy === "install-url"} onPress={installFromURL}>
              <Download size={14} /> 下载安装
            </Button>
          </div>
        </div>
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
            const backendModule = backendModuleByPack.get(manifest.id);
            const mountedRoutes = backendModule?.routes || [];
            const sdkEntries = Object.entries(manifest.sdk || {}).filter((entry): entry is [string, string] => typeof entry[1] === "string" && entry[1].trim().length > 0);
            const declaredBackendRoutes = manifest.backend?.routes || [];
            const mountedPathSet = new Set(mountedRoutes.map((route) => route.path));
            const missingMountedRoutes = declaredBackendRoutes.filter((route) => !mountedPathSet.has(route));
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
                      {mountedRoutes.length > 0 && <Chip size="sm" style={{ background: "rgba(34,197,94,0.10)", color: "var(--yunque-success)" }}>已挂载 {mountedRoutes.length}</Chip>}
                      {declaredBackendRoutes.length > 0 && mountedRoutes.length === 0 && <Chip size="sm" style={{ background: "rgba(245,158,11,0.12)", color: "var(--yunque-warning)" }}>未挂载</Chip>}
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

                {(routes.length > 0 || declaredBackendRoutes.length > 0) && (
                  <div className="text-xs flex items-start gap-2" style={{ color: "var(--yunque-text-muted)" }}>
                    <CheckCircle2 size={13} className="mt-0.5 shrink-0" style={{ color: "var(--yunque-success)" }} />
                    <span>
                      Registry 已声明 {routes.length} 个前端路由、{declaredBackendRoutes.length} 个后端路由。
                      {routes.map((r) => <code key={`fe:${r.path}`} className="mx-1">{r.path}</code>)}
                      {declaredBackendRoutes.map((route) => <code key={`be:${route}`} className="mx-1">{route}</code>)}
                    </span>
                  </div>
                )}

                {missingMountedRoutes.length > 0 && (
                  <div className="text-xs flex items-start gap-2" style={{ color: "var(--yunque-warning)" }}>
                    <ShieldCheck size={13} className="mt-0.5 shrink-0" />
                    <span>manifest 声明但尚未挂载：{missingMountedRoutes.map((route) => <code key={route} className="mx-1">{route}</code>)}</span>
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

