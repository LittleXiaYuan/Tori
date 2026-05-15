"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { Button, Card, Chip, Input, Spinner, TextArea, TextField } from "@heroui/react";
import { AlertTriangle, Cpu, Download, FileCode2, Play, Power, RefreshCw, ShieldCheck } from "lucide-react";
import PageHeader from "@/components/page-header";
import { showToast } from "@/components/toast-provider";
import { formatErrorMessage } from "@/lib/error-utils";
import { createWASMPluginPackClient, type WASMPluginExecuteResult, type WASMPluginStatus, type WASMPluginSummary } from "@/lib/wasm-plugin-pack-client";

const wasmPluginPack = createWASMPluginPackClient();

function statusTone(status: WASMPluginStatus | null): { bg: string; fg: string } {
  if (!status) return { bg: "rgba(255,255,255,0.06)", fg: "var(--yunque-text-muted)" };
  if (status.runtime_ready && status.abi_ready) return { bg: "rgba(34,197,94,0.12)", fg: "#22c55e" };
  if (status.runtime_ready) return { bg: "rgba(250,204,21,0.12)", fg: "#facc15" };
  return { bg: "rgba(239,68,68,0.12)", fg: "#ef4444" };
}

function sampleManifest(slug: string) {
  return JSON.stringify({
    slug,
    name: "Calculator WASM",
    description: "TinyGo/Rust/AssemblyScript 编译出的 WASM 插件元数据样例。",
    version: "0.1.0",
    module_path: `${slug}.wasm`,
    entrypoint: "plugin_exec",
    permissions: {
      ledger_kv: true,
      memory_search: false,
      http_fetch: false,
      allowed_hosts: [],
      env_allowlist: ["YUNQUE_PLUGIN_MODE"],
      max_memory_mb: 64,
      timeout_seconds: 30,
    },
    capabilities: ["tool.calculator", "wasm.sandbox.execute"],
    tags: ["wasm", "sandbox"],
    dry_run: true,
  }, null, 2);
}

export default function WASMPluginPackPage() {
  const [status, setStatus] = useState<WASMPluginStatus | null>(null);
  const [plugins, setPlugins] = useState<WASMPluginSummary[]>([]);
  const [loading, setLoading] = useState(true);
  const [busy, setBusy] = useState<"install" | "load" | "unload" | "execute" | "evidence" | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [slug, setSlug] = useState("calculator");
  const [manifestJSON, setManifestJSON] = useState(() => sampleManifest("calculator"));
  const [inputJSON, setInputJSON] = useState(() => JSON.stringify({ a: 1, b: 2 }, null, 2));
  const [result, setResult] = useState<WASMPluginExecuteResult | null>(null);
  const tone = statusTone(status);

  const selectedPlugin = useMemo(() => plugins.find((plugin) => plugin.slug === slug) || plugins[0] || null, [plugins, slug]);

  const load = useCallback(async () => {
    setError(null);
    try {
      const [statusRes, pluginsRes] = await Promise.all([wasmPluginPack.status(), wasmPluginPack.plugins()]);
      setStatus(statusRes);
      setPlugins(pluginsRes.plugins || []);
      if (!slug && pluginsRes.plugins?.[0]?.slug) setSlug(pluginsRes.plugins[0].slug);
    } catch (e) {
      const msg = formatErrorMessage(e, "加载 WASM Plugin Pack 失败");
      setError(msg.includes("pack route is not enabled") ? "WASM Plugin Pack 当前未启用。请到「增量包」控制台启用 yunque.pack.wasm-plugin 后再使用。" : msg);
    } finally {
      setLoading(false);
    }
  }, [slug]);

  useEffect(() => { load(); }, [load]);

  const installPlugin = async () => {
    setBusy("install");
    setError(null);
    try {
      const payload = JSON.parse(manifestJSON);
      const res = await wasmPluginPack.installPlugin(payload);
      setSlug(res.plugin.slug || slug);
      showToast(payload.dry_run ? "WASM 插件元数据校验通过" : "WASM 插件已注册", "success");
      if (!payload.dry_run) await load();
      setResult(null);
    } catch (e) {
      setError(formatErrorMessage(e, "注册 WASM 插件失败"));
    } finally {
      setBusy(null);
    }
  };

  const setLoaded = async (loaded: boolean) => {
    const target = slug || selectedPlugin?.slug;
    if (!target) return;
    setBusy(loaded ? "load" : "unload");
    setError(null);
    try {
      if (loaded) await wasmPluginPack.load(target);
      else await wasmPluginPack.unload(target);
      showToast(loaded ? "WASM 插件已标记为 loaded" : "WASM 插件已卸载", "success");
      await load();
    } catch (e) {
      setError(formatErrorMessage(e, loaded ? "加载 WASM 插件失败" : "卸载 WASM 插件失败"));
    } finally {
      setBusy(null);
    }
  };

  const executeDryRun = async () => {
    const target = slug || selectedPlugin?.slug;
    if (!target) {
      setError("请先注册或选择一个 WASM 插件。");
      return;
    }
    setBusy("execute");
    setError(null);
    try {
      const res = await wasmPluginPack.execute({ slug: target, input: inputJSON, dry_run: true });
      setResult(res.result);
      showToast("已生成 WASM 沙箱执行计划", "success");
    } catch (e) {
      setError(formatErrorMessage(e, "生成 WASM 执行计划失败"));
    } finally {
      setBusy(null);
    }
  };

  const exportEvidence = async () => {
    const target = slug || selectedPlugin?.slug;
    if (!target) return;
    setBusy("evidence");
    setError(null);
    try {
      const evidence = await wasmPluginPack.evidence(target);
      const blob = new Blob([JSON.stringify(evidence, null, 2)], { type: "application/json" });
      const url = URL.createObjectURL(blob);
      const a = document.createElement("a");
      a.href = url;
      a.download = `${target}-wasm-plugin-evidence.json`;
      document.body.appendChild(a);
      a.click();
      a.remove();
      URL.revokeObjectURL(url);
      showToast("WASM 插件证据包已导出", "success");
    } catch (e) {
      setError(formatErrorMessage(e, "导出 WASM 插件证据包失败"));
    } finally {
      setBusy(null);
    }
  };

  if (loading) {
    return <div className="flex h-[60vh] items-center justify-center"><Spinner size="lg" /></div>;
  }

  return (
    <div className="page-root space-y-6 animate-fade-in-up">
      <PageHeader icon={<Cpu size={20} />} title="WASM 插件引擎" />

      <Card className="section-card p-4">
        <div className="flex flex-wrap items-start justify-between gap-3">
          <div>
            <div className="mb-1 flex items-center gap-2">
              <Chip size="sm" style={{ background: tone.bg, color: tone.fg }}>
                {status?.abi_ready ? "ABI ready" : status?.runtime_ready ? "Runtime shell" : "Disabled"}
              </Chip>
              <Chip size="sm" style={{ background: status?.abi_plan_ready ? "rgba(56,189,248,0.12)" : "rgba(250,204,21,0.12)", color: status?.abi_plan_ready ? "#38bdf8" : "#facc15" }}>
                {status?.abi_plan_ready ? "Host ABI plan" : "ABI plan pending"}
              </Chip>
              <span className="text-xs" style={{ color: "var(--yunque-text-muted)" }}>{status?.pack_id || "yunque.pack.wasm-plugin"}</span>
            </div>
            <div className="text-sm" style={{ color: "var(--yunque-text-muted)" }}>
              当前切片先把 WASM 插件注册、load/unload 生命周期、沙箱执行 dry-run、权限计划、Host ABI plan preview 和证据包放进可选 Pack。Host ABI 权限强执行、远程签名包安装和 TinyGo 示例会在后续切片继续接入。
            </div>
          </div>
          <Button size="sm" variant="ghost" onPress={load}><RefreshCw size={14} />刷新</Button>
        </div>
      </Card>

      {error && (
        <Card className="p-4" style={{ background: "rgba(239,68,68,0.06)" }}>
          <div className="flex items-center gap-2 text-sm" style={{ color: "var(--yunque-danger)" }}><AlertTriangle size={16} />{error}</div>
        </Card>
      )}

      <div className="grid grid-cols-1 gap-4 md:grid-cols-5">
        <Card className="section-card p-4"><div className="kpi-label">插件数量</div><div className="kpi-value">{status?.plugin_count ?? plugins.length}</div></Card>
        <Card className="section-card p-4"><div className="kpi-label">已加载</div><div className="kpi-value">{status?.loaded_count ?? plugins.filter((p) => p.status === "loaded").length}</div></Card>
        <Card className="section-card p-4"><div className="kpi-label">Runtime</div><div className="kpi-value text-lg">{status?.runtime_ready ? "wazero" : "pending"}</div></Card>
        <Card className="section-card p-4"><div className="kpi-label">Host ABI</div><div className="kpi-value text-lg">{status?.abi_plan_ready ? "plan" : "pending"}</div></Card>
        <Card className="section-card p-4"><div className="kpi-label">阶段</div><div className="kpi-value text-lg">{status?.stage || "pack-shell"}</div></Card>
      </div>

      <div className="grid grid-cols-1 gap-4 xl:grid-cols-[360px_1fr]">
        <Card className="section-card overflow-hidden">
          <div className="flex items-center justify-between border-b px-4 py-3" style={{ borderColor: "var(--yunque-border)" }}>
            <div className="flex items-center gap-2 text-sm font-semibold"><FileCode2 size={16} />已注册 WASM 插件</div>
            <Chip size="sm">{plugins.length}</Chip>
          </div>
          <div className="max-h-[520px] divide-y overflow-auto" style={{ borderColor: "var(--yunque-border)" }}>
            {plugins.length === 0 ? <div className="p-6 text-center text-sm" style={{ color: "var(--yunque-text-muted)" }}>还没有插件。可以先 dry-run 校验右侧样例，确认后去掉 dry_run 注册。</div> : plugins.map((plugin) => (
              <button key={plugin.slug} onClick={() => setSlug(plugin.slug)} className="block w-full px-4 py-3 text-left hover:bg-white/5">
                <div className="flex items-center justify-between gap-2"><div className="font-medium">{plugin.name || plugin.slug}</div><Chip size="sm">{plugin.status}</Chip></div>
                <div className="mt-1 truncate text-xs" style={{ color: "var(--yunque-text-muted)" }}>{plugin.slug} · {plugin.entrypoint}</div>
              </button>
            ))}
          </div>
        </Card>

        <div className="space-y-4">
          <Card className="section-card p-4">
            <div className="mb-3 flex items-center justify-between gap-3">
              <div className="flex items-center gap-2 text-sm font-semibold"><ShieldCheck size={16} />注册 / 校验插件元数据</div>
              <TextField className="w-56" value={slug} onChange={setSlug}><Input placeholder="plugin slug" /></TextField>
            </div>
            <TextField value={manifestJSON} onChange={setManifestJSON}>
              <TextArea rows={12} aria-label="WASM plugin manifest JSON" className="font-mono text-xs" />
            </TextField>
            <div className="mt-3 flex justify-end"><Button className="btn-accent" isPending={busy === "install"} onPress={installPlugin}>校验 / 注册插件</Button></div>
          </Card>

          <Card className="section-card p-4">
            <div className="mb-3 flex flex-wrap items-center justify-between gap-3">
              <div>
                <div className="flex items-center gap-2 text-sm font-semibold"><Play size={16} />沙箱执行计划</div>
                <div className="mt-1 text-xs" style={{ color: "var(--yunque-text-muted)" }}>目标插件：{selectedPlugin?.slug || slug}</div>
              </div>
              <div className="flex gap-2">
                <Button variant="outline" isPending={busy === "unload"} onPress={() => setLoaded(false)} isDisabled={!selectedPlugin}><Power size={14} />Unload</Button>
                <Button variant="outline" isPending={busy === "load"} onPress={() => setLoaded(true)} isDisabled={!selectedPlugin}><Power size={14} />Load</Button>
                <Button variant="outline" isPending={busy === "evidence"} onPress={exportEvidence} isDisabled={!selectedPlugin && !slug}><Download size={14} />导出证据包</Button>
                <Button className="btn-accent" isPending={busy === "execute"} onPress={executeDryRun} isDisabled={!selectedPlugin && !slug}>Dry-run</Button>
              </div>
            </div>
            <TextField value={inputJSON} onChange={setInputJSON}>
              <TextArea rows={4} aria-label="WASM plugin input JSON" className="font-mono text-xs" />
            </TextField>
            {result ? (
              <Card className="mt-3 p-3" style={{ background: "rgba(255,255,255,0.03)" }}>
                <div className="mb-2 flex items-center gap-2 text-sm font-medium"><Chip size="sm">{result.dry_run ? "dry-run" : "execute"}</Chip><span>{result.entrypoint}</span></div>
                <div className="mb-2 flex flex-wrap gap-2">
                  <Chip size="sm">abi_plan_ready: {String(result.host_abi_plan?.plan_ready)}</Chip>
                  <Chip size="sm">abi_ready: {String(result.host_abi_plan?.ready)}</Chip>
                  <Chip size="sm">enforcement_ready: {String(result.host_abi_plan?.enforcement_ready)}</Chip>
                  <Chip size="sm">writes_files: {String(result.host_abi_plan?.writes_files)}</Chip>
                  <Chip size="sm">network_access: {String(result.host_abi_plan?.network_access)}</Chip>
                  <Chip size="sm">enabled ABI: {result.host_abi_plan?.summary?.enabled_count ?? 0}/{result.host_abi_plan?.summary?.function_count ?? 0}</Chip>
                </div>
                <TextField value={JSON.stringify(result, null, 2)} onChange={() => undefined}>
                  <TextArea rows={10} aria-label="WASM Plugin execution result" className="font-mono text-xs" readOnly />
                </TextField>
                <div className="mt-2 text-xs" style={{ color: "var(--yunque-text-muted)" }}>
                  Host ABI plan preview 仅固定后续 host function 权限强执行契约：当前不会绑定 wazero host functions、不会写文件，也不会绕过 Pack Runtime gate。
                </div>
              </Card>
            ) : (
              <div className="mt-3 rounded-xl border border-dashed p-6 text-center text-sm" style={{ borderColor: "var(--yunque-border)", color: "var(--yunque-text-muted)" }}>选择 loaded 插件后，可以先生成权限计划与沙箱 dry-run 执行计划。</div>
            )}
          </Card>
        </div>
      </div>
    </div>
  );
}
