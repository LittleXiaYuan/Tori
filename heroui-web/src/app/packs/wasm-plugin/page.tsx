"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import {
  Button,
  Card,
  Chip,
  Input,
  Spinner,
  TextArea,
  TextField,
} from "@heroui/react";
import {
  AlertTriangle,
  Cpu,
  Download,
  FileCode2,
  PackageCheck,
  Play,
  Power,
  RefreshCw,
  ShieldCheck,
} from "lucide-react";
import PageHeader from "@/components/page-header";
import { showToast } from "@/components/toast-provider";
import { formatErrorMessage } from "@/lib/error-utils";
import {
  createWASMPluginPackClient,
  type WASMPluginExecuteResult,
  type WASMPluginRemoteInstallApprovalDecisionPlan,
  type WASMPluginRemoteInstallApprovalPlan,
  type WASMPluginRemoteInstallApprovalQueueWriteback,
  type WASMPluginRemoteInstallApprovalWritebackPlan,
  type WASMPluginRemoteInstallInstallerContinuationPlan,
  type WASMPluginRemoteInstallPlan,
  type WASMPluginStatus,
  type WASMPluginSummary,
} from "@/lib/wasm-plugin-pack-client";

const wasmPluginPack = createWASMPluginPackClient();

function statusTone(status: WASMPluginStatus | null): {
  bg: string;
  fg: string;
} {
  if (!status)
    return { bg: "rgba(255,255,255,0.06)", fg: "var(--yunque-text-muted)" };
  if (status.runtime_ready && status.abi_ready)
    return { bg: "rgba(34,197,94,0.12)", fg: "#22c55e" };
  if (status.runtime_ready)
    return { bg: "rgba(250,204,21,0.12)", fg: "#facc15" };
  return { bg: "rgba(239,68,68,0.12)", fg: "#ef4444" };
}

function sampleManifest(slug: string) {
  return JSON.stringify(
    {
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
    },
    null,
    2,
  );
}

function sampleRemoteInstallPlan(slug: string) {
  return JSON.stringify(
    {
      slug,
      name: "Calculator Remote WASM",
      version: "0.2.0",
      package_url: `https://packs.yunque.local/wasm/${slug}-0.2.0.tgz`,
      manifest_url: `https://packs.yunque.local/wasm/${slug}.json`,
      module_path: `${slug}.wasm`,
      sha256:
        "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
      signature: "sig-ed25519-preview",
      signature_algorithm: "ed25519",
      public_key_id: "yunque-root-2026",
      trust_root: "yunque-root-bundle-2026",
      capabilities: ["tool.calculator", "wasm.sandbox.execute"],
      tags: ["remote", "signed-package"],
      requested_by: "operator",
      reason: "preview remote signed WASM package install contract",
    },
    null,
    2,
  );
}

const remoteSignatureBlockedStatus = "blocked_until_signature_verifier";

function sampleRemoteApprovalPlan(slug: string) {
  return JSON.stringify(
    {
      slug,
      name: "Calculator Remote WASM",
      version: "0.2.0",
      package_url: `https://packs.yunque.local/wasm/${slug}-0.2.0.tgz`,
      manifest_url: `https://packs.yunque.local/wasm/${slug}.json`,
      module_path: `${slug}.wasm`,
      sha256:
        "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
      signature: "sig-ed25519-preview",
      signature_algorithm: "ed25519",
      public_key_id: "yunque-root-2026",
      trust_root: "yunque-root-bundle-2026",
      requested_by: "operator",
      reason: "remote WASM package must be approved before install wiring",
      risk_tier: "high",
      approvers: ["security", "platform"],
      metadata: { ticket: "WASM-REMOTE-APPROVAL-1" },
    },
    null,
    2,
  );
}

function sampleRemoteApprovalDecisionPlan(slug: string) {
  return JSON.stringify(
    {
      slug,
      name: "Calculator Remote WASM",
      version: "0.2.0",
      package_url: `https://packs.yunque.local/wasm/${slug}-0.2.0.tgz`,
      manifest_url: `https://packs.yunque.local/wasm/${slug}.json`,
      module_path: `${slug}.wasm`,
      sha256:
        "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
      signature: "sig-ed25519-preview",
      signature_algorithm: "ed25519",
      public_key_id: "yunque-root-2026",
      trust_root: "yunque-root-bundle-2026",
      requested_by: "operator",
      reason: "remote WASM package must be approved before install wiring",
      risk_tier: "high",
      approvers: ["security", "platform"],
      request_id: "wasm-remote-install-preview",
      request_key: "preview-request-key",
      decision: "approved",
      decision_by: "security",
      decision_reason: "plan-only preview; do not apply or persist",
      metadata: { ticket: "WASM-REMOTE-DECISION-1" },
    },
    null,
    2,
  );
}

function sampleRemoteApprovalWritebackPlan(slug: string) {
  return JSON.stringify(
    {
      slug,
      name: "Calculator Remote WASM",
      version: "0.2.0",
      package_url: `https://packs.yunque.local/wasm/${slug}-0.2.0.tgz`,
      manifest_url: `https://packs.yunque.local/wasm/${slug}.json`,
      module_path: `${slug}.wasm`,
      sha256:
        "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
      signature: "sig-ed25519-preview",
      signature_algorithm: "ed25519",
      public_key_id: "yunque-root-2026",
      trust_root: "yunque-root-bundle-2026",
      requested_by: "operator",
      reason: "remote WASM package approval queue writeback bridge preview",
      risk_tier: "high",
      approvers: ["security", "platform"],
      request_id: "wasm-remote-install-preview",
      request_key: "preview-request-key",
      decision: "approved",
      decision_by: "security",
      decision_reason: "plan-only preview; do not write approval queue",
      metadata: { ticket: "WASM-REMOTE-WRITEBACK-1" },
    },
    null,
    2,
  );
}

function sampleRemoteApprovalQueueWriteback(slug: string) {
  return JSON.stringify(
    {
      slug,
      name: "Calculator Remote WASM",
      version: "0.2.0",
      package_url: `https://packs.yunque.local/wasm/${slug}-0.2.0.tgz`,
      manifest_url: `https://packs.yunque.local/wasm/${slug}.json`,
      module_path: `${slug}.wasm`,
      sha256:
        "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
      signature: "sig-ed25519-preview",
      signature_algorithm: "ed25519",
      public_key_id: "yunque-root-2026",
      trust_root: "yunque-root-bundle-2026",
      requested_by: "operator",
      reason: "persist remote WASM approval decision into pack-local queue",
      risk_tier: "high",
      approvers: ["security", "platform"],
      request_id: "wasm-remote-install-preview",
      request_key: "preview-request-key",
      decision: "approved",
      decision_by: "security",
      decision_reason:
        "write only the pack-local approval queue; installer remains blocked",
      metadata: { ticket: "WASM-REMOTE-QUEUE-WRITEBACK-1" },
    },
    null,
    2,
  );
}

function sampleInstallerContinuationPlan(slug: string) {
  return JSON.stringify(
    {
      slug,
      request_key: "preview-request-key",
    },
    null,
    2,
  );
}

export default function WASMPluginPackPage() {
  const [status, setStatus] = useState<WASMPluginStatus | null>(null);
  const [plugins, setPlugins] = useState<WASMPluginSummary[]>([]);
  const [loading, setLoading] = useState(true);
  const [busy, setBusy] = useState<
    | "install"
    | "load"
    | "unload"
    | "execute"
    | "evidence"
    | "remote-install"
    | "remote-approval"
    | "remote-approval-decision"
    | "remote-approval-writeback"
    | "remote-approval-queue-writeback"
    | "remote-installer-continuation"
    | null
  >(null);
  const [error, setError] = useState<string | null>(null);
  const [slug, setSlug] = useState("calculator");
  const [manifestJSON, setManifestJSON] = useState(() =>
    sampleManifest("calculator"),
  );
  const [remoteInstallJSON, setRemoteInstallJSON] = useState(() =>
    sampleRemoteInstallPlan("calculator-remote"),
  );
  const [remoteInstallPlan, setRemoteInstallPlan] =
    useState<WASMPluginRemoteInstallPlan | null>(null);
  const [remoteApprovalJSON, setRemoteApprovalJSON] = useState(() =>
    sampleRemoteApprovalPlan("calculator-remote"),
  );
  const [remoteApprovalPlan, setRemoteApprovalPlan] =
    useState<WASMPluginRemoteInstallApprovalPlan | null>(null);
  const [remoteDecisionJSON, setRemoteDecisionJSON] = useState(() =>
    sampleRemoteApprovalDecisionPlan("calculator-remote"),
  );
  const [remoteDecisionPlan, setRemoteDecisionPlan] =
    useState<WASMPluginRemoteInstallApprovalDecisionPlan | null>(null);
  const [remoteWritebackJSON, setRemoteWritebackJSON] = useState(() =>
    sampleRemoteApprovalWritebackPlan("calculator-remote"),
  );
  const [remoteWritebackPlan, setRemoteWritebackPlan] =
    useState<WASMPluginRemoteInstallApprovalWritebackPlan | null>(null);
  const [remoteQueueWritebackJSON, setRemoteQueueWritebackJSON] = useState(() =>
    sampleRemoteApprovalQueueWriteback("calculator-remote"),
  );
  const [remoteQueueWriteback, setRemoteQueueWriteback] =
    useState<WASMPluginRemoteInstallApprovalQueueWriteback | null>(null);
  const [remoteInstallerJSON, setRemoteInstallerJSON] = useState(() =>
    sampleInstallerContinuationPlan("calculator-remote"),
  );
  const [remoteInstallerPlan, setRemoteInstallerPlan] =
    useState<WASMPluginRemoteInstallInstallerContinuationPlan | null>(null);
  const [inputJSON, setInputJSON] = useState(() =>
    JSON.stringify({ a: 1, b: 2 }, null, 2),
  );
  const [result, setResult] = useState<WASMPluginExecuteResult | null>(null);
  const tone = statusTone(status);

  const selectedPlugin = useMemo(
    () => plugins.find((plugin) => plugin.slug === slug) || plugins[0] || null,
    [plugins, slug],
  );

  const load = useCallback(async () => {
    setError(null);
    try {
      const [statusRes, pluginsRes] = await Promise.all([
        wasmPluginPack.status(),
        wasmPluginPack.plugins(),
      ]);
      setStatus(statusRes);
      setPlugins(pluginsRes.plugins || []);
      if (!slug && pluginsRes.plugins?.[0]?.slug)
        setSlug(pluginsRes.plugins[0].slug);
    } catch (e) {
      const msg = formatErrorMessage(e, "加载 WASM Plugin Pack 失败");
      setError(
        msg.includes("pack route is not enabled")
          ? "WASM Plugin Pack 当前未启用。请到「增量包」控制台启用 yunque.pack.wasm-plugin 后再使用。"
          : msg,
      );
    } finally {
      setLoading(false);
    }
  }, [slug]);

  useEffect(() => {
    load();
  }, [load]);

  const installPlugin = async () => {
    setBusy("install");
    setError(null);
    try {
      const payload = JSON.parse(manifestJSON);
      const res = await wasmPluginPack.installPlugin(payload);
      setSlug(res.plugin.slug || slug);
      showToast(
        payload.dry_run ? "WASM 插件元数据校验通过" : "WASM 插件已注册",
        "success",
      );
      if (!payload.dry_run) await load();
      setResult(null);
    } catch (e) {
      setError(formatErrorMessage(e, "注册 WASM 插件失败"));
    } finally {
      setBusy(null);
    }
  };

  const planRemoteInstall = async () => {
    setBusy("remote-install");
    setError(null);
    try {
      const payload = JSON.parse(remoteInstallJSON);
      const res = await wasmPluginPack.remoteInstallPlan(payload);
      setRemoteInstallPlan(res.plan);
      showToast("已生成远程签名包安装计划", "success");
    } catch (e) {
      setError(formatErrorMessage(e, "生成远程签名包安装计划失败"));
    } finally {
      setBusy(null);
    }
  };

  const planRemoteApproval = async () => {
    setBusy("remote-approval");
    setError(null);
    try {
      const payload = JSON.parse(remoteApprovalJSON);
      const res = await wasmPluginPack.remoteInstallApprovalPlan(payload);
      setRemoteApprovalPlan(res.plan);
      showToast("已生成远程安装审批 gate 计划", "success");
    } catch (e) {
      setError(formatErrorMessage(e, "生成远程安装审批 gate 计划失败"));
    } finally {
      setBusy(null);
    }
  };

  const planRemoteApprovalDecision = async () => {
    setBusy("remote-approval-decision");
    setError(null);
    try {
      const payload = JSON.parse(remoteDecisionJSON);
      const res =
        await wasmPluginPack.remoteInstallApprovalDecisionPlan(payload);
      setRemoteDecisionPlan(res.plan);
      showToast("已生成远程安装审批决策计划", "success");
    } catch (e) {
      setError(formatErrorMessage(e, "生成远程安装审批决策计划失败"));
    } finally {
      setBusy(null);
    }
  };

  const planRemoteApprovalWriteback = async () => {
    setBusy("remote-approval-writeback");
    setError(null);
    try {
      const payload = JSON.parse(remoteWritebackJSON);
      const res =
        await wasmPluginPack.remoteInstallApprovalWritebackPlan(payload);
      setRemoteWritebackPlan(res.plan);
      showToast("已生成远程安装审批写回桥接计划", "success");
    } catch (e) {
      setError(formatErrorMessage(e, "生成远程安装审批写回桥接计划失败"));
    } finally {
      setBusy(null);
    }
  };

  const writeRemoteApprovalQueue = async () => {
    setBusy("remote-approval-queue-writeback");
    setError(null);
    try {
      const payload = JSON.parse(remoteQueueWritebackJSON);
      const res =
        await wasmPluginPack.remoteInstallApprovalQueueWriteback(payload);
      setRemoteQueueWriteback(res.writeback);
      showToast("已写入 pack-local 远程安装审批队列", "success");
      await load();
    } catch (e) {
      setError(formatErrorMessage(e, "写入远程安装审批队列失败"));
    } finally {
      setBusy(null);
    }
  };

  const planInstallerContinuation = async () => {
    setBusy("remote-installer-continuation");
    setError(null);
    try {
      const payload = JSON.parse(remoteInstallerJSON);
      const res =
        await wasmPluginPack.remoteInstallInstallerContinuationPlan(payload);
      setRemoteInstallerPlan(res.plan);
      showToast("已生成 installer continuation handoff 计划", "success");
    } catch (e) {
      setError(formatErrorMessage(e, "生成 installer continuation 计划失败"));
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
      showToast(
        loaded ? "WASM 插件已标记为 loaded" : "WASM 插件已卸载",
        "success",
      );
      await load();
    } catch (e) {
      setError(
        formatErrorMessage(
          e,
          loaded ? "加载 WASM 插件失败" : "卸载 WASM 插件失败",
        ),
      );
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
      const res = await wasmPluginPack.execute({
        slug: target,
        input: inputJSON,
        dry_run: true,
      });
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
      const blob = new Blob([JSON.stringify(evidence, null, 2)], {
        type: "application/json",
      });
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
    return (
      <div className="flex h-[60vh] items-center justify-center">
        <Spinner size="lg" />
      </div>
    );
  }

  return (
    <div className="page-root space-y-6 animate-fade-in-up">
      <PageHeader icon={<Cpu size={20} />} title="WASM 插件引擎" />

      <Card className="section-card p-4">
        <div className="flex flex-wrap items-start justify-between gap-3">
          <div>
            <div className="mb-1 flex items-center gap-2">
              <Chip size="sm" style={{ background: tone.bg, color: tone.fg }}>
                {status?.abi_ready
                  ? "ABI ready"
                  : status?.runtime_ready
                    ? "Runtime shell"
                    : "Disabled"}
              </Chip>
              <Chip
                size="sm"
                style={{
                  background: status?.abi_plan_ready
                    ? "rgba(56,189,248,0.12)"
                    : "rgba(250,204,21,0.12)",
                  color: status?.abi_plan_ready ? "#38bdf8" : "#facc15",
                }}
              >
                {status?.abi_plan_ready ? "Host ABI plan" : "ABI plan pending"}
              </Chip>
              <Chip
                size="sm"
                style={{
                  background: status?.host_abi_execution_gate_ready
                    ? "rgba(34,197,94,0.12)"
                    : "rgba(250,204,21,0.12)",
                  color: status?.host_abi_execution_gate_ready
                    ? "#22c55e"
                    : "#facc15",
                }}
              >
                {status?.host_abi_execution_gate_ready
                  ? "Host ABI execution gate"
                  : "Host ABI gate pending"}
              </Chip>
              <Chip
                size="sm"
                style={{
                  background: status?.remote_install_plan_ready
                    ? "rgba(168,85,247,0.12)"
                    : "rgba(250,204,21,0.12)",
                  color: status?.remote_install_plan_ready
                    ? "#c084fc"
                    : "#facc15",
                }}
              >
                {status?.remote_install_plan_ready
                  ? "Remote install plan"
                  : "Remote install pending"}
              </Chip>
              <Chip size="sm">
                remote_install_ready:{" "}
                {String(status?.remote_install_ready ?? false)}
              </Chip>
              <Chip size="sm">
                module_integrity_gate_ready:{" "}
                {String(status?.module_integrity_gate_ready ?? false)}
              </Chip>
              <Chip size="sm">
                approval_gate_ready:{" "}
                {String(status?.approval_gate_ready ?? false)}
              </Chip>
              <Chip size="sm">
                approval_decision_plan_ready:{" "}
                {String(status?.approval_decision_plan_ready ?? false)}
              </Chip>
              <Chip size="sm">
                approval_writeback_plan_ready:{" "}
                {String(status?.approval_writeback_plan_ready ?? false)}
              </Chip>
              <Chip size="sm">
                installer_continuation_plan_ready:{" "}
                {String(status?.installer_continuation_plan_ready ?? false)}
              </Chip>
              <Chip size="sm">
                installer_ready: {String(status?.installer_ready ?? false)}
              </Chip>
              <span
                className="text-xs"
                style={{ color: "var(--yunque-text-muted)" }}
              >
                {status?.pack_id || "yunque.pack.wasm-plugin"}
              </span>
            </div>
            <div
              className="text-sm"
              style={{ color: "var(--yunque-text-muted)" }}
            >
              当前切片先把 WASM 插件注册、load/unload 生命周期、沙箱执行
              dry-run、权限计划、Host ABI plan preview、真实执行前 Host ABI
              execution gate、远程签名包安装计划、远程安装审批 gate
              计划、审批决策 plan-only
              预览、审批队列写回桥接计划、pack-local 审批队列 JSON
              持久化和证据包放进可选 Pack。请求 ledger_kv /
              memory_search / http_fetch / env_get 的插件在
              host_abi_enforcement_ready=false 时会被真实执行前阻断；本地 WASM
              模块 SHA-256 与注册元数据不一致时也会被 module integrity gate
              阻断，不进入 sandbox；当前审批写回只落 pack-local queue store，
              installer continuation 现在只产出 handoff 计划并继续保持
              installer_ready=false；真实下载、签名验证、install 写回、插件注册、
              Host ABI 权限强执行和 TinyGo
              示例会在后续切片继续接入。
            </div>
          </div>
          <Button size="sm" variant="ghost" onPress={load}>
            <RefreshCw size={14} />
            刷新
          </Button>
        </div>
      </Card>

      {error && (
        <Card className="p-4" style={{ background: "rgba(239,68,68,0.06)" }}>
          <div
            className="flex items-center gap-2 text-sm"
            style={{ color: "var(--yunque-danger)" }}
          >
            <AlertTriangle size={16} />
            {error}
          </div>
        </Card>
      )}

      <div className="grid grid-cols-1 gap-4 md:grid-cols-4 xl:grid-cols-8">
        <Card className="section-card p-4">
          <div className="kpi-label">插件数量</div>
          <div className="kpi-value">
            {status?.plugin_count ?? plugins.length}
          </div>
        </Card>
        <Card className="section-card p-4">
          <div className="kpi-label">已加载</div>
          <div className="kpi-value">
            {status?.loaded_count ??
              plugins.filter((p) => p.status === "loaded").length}
          </div>
        </Card>
        <Card className="section-card p-4">
          <div className="kpi-label">Runtime</div>
          <div className="kpi-value text-lg">
            {status?.runtime_ready ? "wazero" : "pending"}
          </div>
        </Card>
        <Card className="section-card p-4">
          <div className="kpi-label">Host ABI</div>
          <div className="kpi-value text-lg">
            {status?.abi_plan_ready ? "plan" : "pending"}
          </div>
        </Card>
        <Card className="section-card p-4">
          <div className="kpi-label">执行 Gate</div>
          <div className="kpi-value text-lg">
            {status?.host_abi_execution_gate_ready ? "gate" : "pending"}
          </div>
          <div className="kpi-label mt-1">
            enforcement: {String(status?.host_abi_enforcement_ready ?? false)}
          </div>
        </Card>
        <Card className="section-card p-4">
          <div className="kpi-label">模块完整性</div>
          <div className="kpi-value text-lg">
            {status?.module_integrity_gate_ready ? "sha256" : "pending"}
          </div>
        </Card>
        <Card className="section-card p-4">
          <div className="kpi-label">远程安装</div>
          <div className="kpi-value text-lg">
            {status?.remote_install_plan_ready ? "plan" : "pending"}
          </div>
        </Card>
        <Card className="section-card p-4">
          <div className="kpi-label">审批 Gate</div>
          <div className="kpi-value text-lg">
            {status?.approval_gate_plan_ready ? "plan" : "pending"}
          </div>
        </Card>
        <Card className="section-card p-4">
          <div className="kpi-label">阶段</div>
          <div className="kpi-value text-lg">
            {status?.stage || "pack-shell"}
          </div>
        </Card>
        <Card className="section-card p-4">
          <div className="kpi-label">审批决策</div>
          <div className="kpi-value text-lg">
            {status?.approval_decision_plan_ready ? "plan" : "pending"}
          </div>
          <div className="kpi-label mt-1">
            ready: {String(status?.approval_decision_ready ?? false)}
          </div>
        </Card>
        <Card className="section-card p-4">
          <div className="kpi-label">审批写回</div>
          <div className="kpi-value text-lg">
            {status?.approval_queue_store_ready
              ? "queue"
              : status?.approval_writeback_plan_ready
                ? "plan"
                : "pending"}
          </div>
          <div className="kpi-label mt-1">
            ready: {String(status?.approval_writeback_ready ?? false)}
          </div>
        </Card>
        <Card className="section-card p-4">
          <div className="kpi-label">Installer</div>
          <div className="kpi-value text-lg">
            {status?.installer_continuation_plan_ready ? "handoff" : "pending"}
          </div>
          <div className="kpi-label mt-1">
            ready: {String(status?.installer_ready ?? false)}
          </div>
        </Card>
      </div>

      <div className="grid grid-cols-1 gap-4 xl:grid-cols-[360px_1fr]">
        <Card className="section-card overflow-hidden">
          <div
            className="flex items-center justify-between border-b px-4 py-3"
            style={{ borderColor: "var(--yunque-border)" }}
          >
            <div className="flex items-center gap-2 text-sm font-semibold">
              <FileCode2 size={16} />
              已注册 WASM 插件
            </div>
            <Chip size="sm">{plugins.length}</Chip>
          </div>
          <div
            className="max-h-[520px] divide-y overflow-auto"
            style={{ borderColor: "var(--yunque-border)" }}
          >
            {plugins.length === 0 ? (
              <div
                className="p-6 text-center text-sm"
                style={{ color: "var(--yunque-text-muted)" }}
              >
                还没有插件。可以先 dry-run 校验右侧样例，确认后去掉 dry_run
                注册。
              </div>
            ) : (
              plugins.map((plugin) => (
                <button
                  key={plugin.slug}
                  onClick={() => setSlug(plugin.slug)}
                  className="block w-full px-4 py-3 text-left hover:bg-white/5"
                >
                  <div className="flex items-center justify-between gap-2">
                    <div className="font-medium">
                      {plugin.name || plugin.slug}
                    </div>
                    <Chip size="sm">{plugin.status}</Chip>
                  </div>
                  <div
                    className="mt-1 truncate text-xs"
                    style={{ color: "var(--yunque-text-muted)" }}
                  >
                    {plugin.slug} · {plugin.entrypoint}
                  </div>
                </button>
              ))
            )}
          </div>
        </Card>

        <div className="space-y-4">
          <Card className="section-card p-4">
            <div className="mb-3 flex items-center justify-between gap-3">
              <div className="flex items-center gap-2 text-sm font-semibold">
                <ShieldCheck size={16} />
                注册 / 校验插件元数据
              </div>
              <TextField className="w-56" value={slug} onChange={setSlug}>
                <Input placeholder="plugin slug" />
              </TextField>
            </div>
            <TextField value={manifestJSON} onChange={setManifestJSON}>
              <TextArea
                rows={12}
                aria-label="WASM plugin manifest JSON"
                className="font-mono text-xs"
              />
            </TextField>
            <div className="mt-3 flex justify-end">
              <Button
                className="btn-accent"
                isPending={busy === "install"}
                onPress={installPlugin}
              >
                校验 / 注册插件
              </Button>
            </div>
          </Card>

          <Card className="section-card p-4">
            <div className="mb-3 flex flex-wrap items-center justify-between gap-3">
              <div>
                <div className="flex items-center gap-2 text-sm font-semibold">
                  <ShieldCheck size={16} />
                  Installer continuation handoff 计划
                </div>
                <div
                  className="mt-1 text-xs"
                  style={{ color: "var(--yunque-text-muted)" }}
                >
                  plan-only 契约：读取 pack-local approval-queue-store.json /
                  approval-queue-record.json，生成
                  installer-continuation-plan.json、
                  installer-download-handoff-plan.json、
                  installer-registration-handoff-plan.json 与
                  installer-audit-handoff-plan.json；不下载、不联网、不验签、不写插件文件、不注册插件。
                </div>
              </div>
              <Button
                className="btn-accent"
                isPending={busy === "remote-installer-continuation"}
                onPress={planInstallerContinuation}
              >
                生成 handoff
              </Button>
            </div>
            <TextField
              value={remoteInstallerJSON}
              onChange={setRemoteInstallerJSON}
            >
              <TextArea
                rows={5}
                aria-label="WASM remote install installer continuation plan JSON"
                className="font-mono text-xs"
              />
            </TextField>
            <div className="mt-3 flex flex-wrap gap-2">
              <Chip size="sm">
                installer_continuation_plan_ready:{" "}
                {String(
                  remoteInstallerPlan?.installer_continuation_plan_ready ??
                    status?.installer_continuation_plan_ready ??
                    false,
                )}
              </Chip>
              <Chip size="sm">
                consumes_approval_queue_store:{" "}
                {String(
                  remoteInstallerPlan?.consumes_approval_queue_store ?? false,
                )}
              </Chip>
              <Chip size="sm">
                approval_queue_record_found:{" "}
                {String(
                  remoteInstallerPlan?.approval_queue_record_found ?? false,
                )}
              </Chip>
              <Chip size="sm">
                approval_approved:{" "}
                {String(remoteInstallerPlan?.approval_approved ?? false)}
              </Chip>
              <Chip size="sm">
                would_allow_installer_continue:{" "}
                {String(
                  remoteInstallerPlan?.would_allow_installer_continue ?? false,
                )}
              </Chip>
              <Chip size="sm">
                blocks_installer:{" "}
                {String(remoteInstallerPlan?.blocks_installer ?? true)}
              </Chip>
              <Chip size="sm">
                installer_ready:{" "}
                {String(
                  remoteInstallerPlan?.installer_ready ??
                    status?.installer_ready ??
                    false,
                )}
              </Chip>
              <Chip size="sm">
                installer_blocked_until_installer_wiring:{" "}
                {String(
                  remoteInstallerPlan
                    ?.installer_blocked_until_installer_wiring ??
                    status?.installer_blocked_until_installer_wiring ??
                    true,
                )}
              </Chip>
              <Chip size="sm">
                download_ready:{" "}
                {String(remoteInstallerPlan?.download_ready ?? false)}
              </Chip>
              <Chip size="sm">
                signature_verify_ready:{" "}
                {String(remoteInstallerPlan?.signature_verify_ready ?? false)}
              </Chip>
              <Chip size="sm">
                downloads: {String(remoteInstallerPlan?.downloads ?? false)}
              </Chip>
              <Chip size="sm">
                writes_files:{" "}
                {String(remoteInstallerPlan?.writes_files ?? false)}
              </Chip>
              <Chip size="sm">
                installs_plugin:{" "}
                {String(remoteInstallerPlan?.installs_plugin ?? false)}
              </Chip>
              <Chip size="sm">artifact: installer-continuation-plan.json</Chip>
              <Chip size="sm">
                artifact: installer-download-handoff-plan.json
              </Chip>
              <Chip size="sm">
                artifact: installer-registration-handoff-plan.json
              </Chip>
              <Chip size="sm">artifact: installer-audit-handoff-plan.json</Chip>
            </div>
            {remoteInstallerPlan && (
              <TextField
                className="mt-3"
                value={JSON.stringify(remoteInstallerPlan, null, 2)}
                onChange={() => undefined}
              >
                <TextArea
                  rows={11}
                  aria-label="WASM installer continuation plan result"
                  className="font-mono text-xs"
                  readOnly
                />
              </TextField>
            )}
          </Card>

          <Card className="section-card p-4">
            <div className="mb-3 flex flex-wrap items-center justify-between gap-3">
              <div>
                <div className="flex items-center gap-2 text-sm font-semibold">
                  <ShieldCheck size={16} />
                  远程安装审批队列写回
                </div>
                <div
                  className="mt-1 text-xs"
                  style={{ color: "var(--yunque-text-muted)" }}
                >
                  真实写入边界：只把审批 entry / decision 写入 pack-local
                  approval-queue-store.json，生成 approval-queue-record.json
                  语义证据；不下载包、不联网、不验签、不写插件文件、不注册插件，installer
                  仍保持 blocked_until_installer_wiring。
                </div>
              </div>
              <Button
                className="btn-accent"
                isPending={busy === "remote-approval-queue-writeback"}
                onPress={writeRemoteApprovalQueue}
              >
                写入队列
              </Button>
            </div>
            <TextField
              value={remoteQueueWritebackJSON}
              onChange={setRemoteQueueWritebackJSON}
            >
              <TextArea
                rows={8}
                aria-label="WASM remote install approval queue writeback JSON"
                className="font-mono text-xs"
              />
            </TextField>
            <div className="mt-3 flex flex-wrap gap-2">
              <Chip size="sm">
                approval_queue_store_ready:{" "}
                {String(
                  remoteQueueWriteback?.approval_queue_store_ready ??
                    status?.approval_queue_store_ready ??
                    false,
                )}
              </Chip>
              <Chip size="sm">
                approval_writeback_ready:{" "}
                {String(
                  remoteQueueWriteback?.approval_writeback_ready ??
                    status?.approval_writeback_ready ??
                    false,
                )}
              </Chip>
              <Chip size="sm">
                writes_approval_queue:{" "}
                {String(remoteQueueWriteback?.writes_approval_queue ?? false)}
              </Chip>
              <Chip size="sm">
                writes_approval_queue_store:{" "}
                {String(
                  remoteQueueWriteback?.writes_approval_queue_store ?? false,
                )}
              </Chip>
              <Chip size="sm">
                approval_queue_ready:{" "}
                {String(
                  remoteQueueWriteback?.approval_queue_ready ??
                    status?.approval_queue_ready ??
                    false,
                )}
              </Chip>
              <Chip size="sm">
                approval_decision_ready:{" "}
                {String(
                  remoteQueueWriteback?.approval_decision_ready ??
                    status?.approval_decision_ready ??
                    false,
                )}
              </Chip>
              <Chip size="sm">
                applies_approval_decision:{" "}
                {String(
                  remoteQueueWriteback?.applies_approval_decision ?? false,
                )}
              </Chip>
              <Chip size="sm">
                installer_blocked_until_writeback:{" "}
                {String(
                  remoteQueueWriteback?.installer_blocked_until_writeback ??
                    true,
                )}
              </Chip>
              <Chip size="sm">
                installer_blocked_until_installer_wiring:{" "}
                {String(
                  remoteQueueWriteback
                    ?.installer_blocked_until_installer_wiring ?? true,
                )}
              </Chip>
              <Chip size="sm">
                downloads: {String(remoteQueueWriteback?.downloads ?? false)}
              </Chip>
              <Chip size="sm">
                writes_files:{" "}
                {String(remoteQueueWriteback?.writes_files ?? false)}
              </Chip>
              <Chip size="sm">
                network_access:{" "}
                {String(remoteQueueWriteback?.network_access ?? false)}
              </Chip>
              <Chip size="sm">
                installs_plugin:{" "}
                {String(remoteQueueWriteback?.installs_plugin ?? false)}
              </Chip>
              <Chip size="sm">
                store_records:{" "}
                {remoteQueueWriteback?.approval_queue_store?.record_count ??
                  status?.approval_queue_store?.record_count ??
                  0}
              </Chip>
              <Chip size="sm">
                approval_queue_record:{" "}
                {remoteQueueWriteback?.approval_queue_record?.status ||
                  "pending"}
              </Chip>
              <Chip size="sm">artifact: approval-queue-store.json</Chip>
              <Chip size="sm">artifact: approval-queue-record.json</Chip>
            </div>
            {remoteQueueWriteback && (
              <TextField
                className="mt-3"
                value={JSON.stringify(remoteQueueWriteback, null, 2)}
                onChange={() => undefined}
              >
                <TextArea
                  rows={11}
                  aria-label="WASM remote install approval queue writeback result"
                  className="font-mono text-xs"
                  readOnly
                />
              </TextField>
            )}
          </Card>

          <Card className="section-card p-4">
            <div className="mb-3 flex flex-wrap items-center justify-between gap-3">
              <div>
                <div className="flex items-center gap-2 text-sm font-semibold">
                  <ShieldCheck size={16} />
                  远程安装审批决策计划
                </div>
                <div
                  className="mt-1 text-xs"
                  style={{ color: "var(--yunque-text-muted)" }}
                >
                  plan-only 契约：固定 approved / denied / expired 的后续
                  installer 策略，只生成 approval-decision-plan.json
                  预览；不写审批队列、不应用决策、不下载、不联网、不安装。
                </div>
              </div>
              <Button
                className="btn-accent"
                isPending={busy === "remote-approval-decision"}
                onPress={planRemoteApprovalDecision}
              >
                生成决策计划
              </Button>
            </div>
            <TextField
              value={remoteDecisionJSON}
              onChange={setRemoteDecisionJSON}
            >
              <TextArea
                rows={8}
                aria-label="WASM remote install approval decision plan JSON"
                className="font-mono text-xs"
              />
            </TextField>
            <div className="mt-3 flex flex-wrap gap-2">
              <Chip size="sm">
                approval_decision_plan_ready:{" "}
                {String(
                  remoteDecisionPlan?.approval_decision_plan_ready ??
                    status?.approval_decision_plan_ready ??
                    false,
                )}
              </Chip>
              <Chip size="sm">
                approval_decision_ready:{" "}
                {String(
                  remoteDecisionPlan?.approval_decision_ready ??
                    status?.approval_decision_ready ??
                    false,
                )}
              </Chip>
              <Chip size="sm">
                applies_approval_decision:{" "}
                {String(remoteDecisionPlan?.applies_approval_decision ?? false)}
              </Chip>
              <Chip size="sm">
                writes_approval_queue:{" "}
                {String(remoteDecisionPlan?.writes_approval_queue ?? false)}
              </Chip>
              <Chip size="sm">
                decision: {remoteDecisionPlan?.decision || "approved"}
              </Chip>
              <Chip size="sm">
                decision_by: {remoteDecisionPlan?.decision_by || "security"}
              </Chip>
              <Chip size="sm">
                would_allow_installer_continue:{" "}
                {String(
                  remoteDecisionPlan?.would_allow_installer_continue ?? false,
                )}
              </Chip>
              <Chip size="sm">
                blocks_installer:{" "}
                {String(remoteDecisionPlan?.blocks_installer ?? true)}
              </Chip>
              <Chip size="sm">
                request_id: {remoteDecisionPlan?.request_id || "pending"}
              </Chip>
              <Chip size="sm">
                decision_key:{" "}
                {remoteDecisionPlan?.decision_plan?.decision_key || "pending"}
              </Chip>
              <Chip size="sm">
                downloads: {String(remoteDecisionPlan?.downloads ?? false)}
              </Chip>
              <Chip size="sm">
                writes_files:{" "}
                {String(remoteDecisionPlan?.writes_files ?? false)}
              </Chip>
              <Chip size="sm">
                installs_plugin:{" "}
                {String(remoteDecisionPlan?.installs_plugin ?? false)}
              </Chip>
              <Chip size="sm">artifact: approval-decision-plan.json</Chip>
            </div>
            {remoteDecisionPlan && (
              <TextField
                className="mt-3"
                value={JSON.stringify(remoteDecisionPlan, null, 2)}
                onChange={() => undefined}
              >
                <TextArea
                  rows={11}
                  aria-label="WASM remote install approval decision plan result"
                  className="font-mono text-xs"
                  readOnly
                />
              </TextField>
            )}
          </Card>

          <Card className="section-card p-4">
            <div className="mb-3 flex flex-wrap items-center justify-between gap-3">
              <div>
                <div className="flex items-center gap-2 text-sm font-semibold">
                  <ShieldCheck size={16} />
                  远程安装审批写回桥接计划
                </div>
                <div
                  className="mt-1 text-xs"
                  style={{ color: "var(--yunque-text-muted)" }}
                >
                  plan-only 契约：只把 approval-queue-entry.json 与
                  approval-decision-plan.json 串成 approval-writeback-plan.json
                  预览；该计划本身不写队列、不应用决策。真实队列写回请使用上方
                  pack-local queue store 路由，installer continuation 仍需后续显式接线。
                </div>
              </div>
              <Button
                className="btn-accent"
                isPending={busy === "remote-approval-writeback"}
                onPress={planRemoteApprovalWriteback}
              >
                生成写回计划
              </Button>
            </div>
            <TextField
              value={remoteWritebackJSON}
              onChange={setRemoteWritebackJSON}
            >
              <TextArea
                rows={8}
                aria-label="WASM remote install approval writeback plan JSON"
                className="font-mono text-xs"
              />
            </TextField>
            <div className="mt-3 flex flex-wrap gap-2">
              <Chip size="sm">
                approval_writeback_plan_ready:{" "}
                {String(
                  remoteWritebackPlan?.approval_writeback_plan_ready ??
                    status?.approval_writeback_plan_ready ??
                    false,
                )}
              </Chip>
              <Chip size="sm">
                approval_writeback_ready:{" "}
                {String(remoteWritebackPlan?.approval_writeback_ready ?? false)}
              </Chip>
              <Chip size="sm">
                writes_approval_queue:{" "}
                {String(remoteWritebackPlan?.writes_approval_queue ?? false)}
              </Chip>
              <Chip size="sm">
                approval_queue_ready:{" "}
                {String(remoteWritebackPlan?.approval_queue_ready ?? false)}
              </Chip>
              <Chip size="sm">
                approval_decision_ready:{" "}
                {String(remoteWritebackPlan?.approval_decision_ready ?? false)}
              </Chip>
              <Chip size="sm">
                applies_approval_decision:{" "}
                {String(
                  remoteWritebackPlan?.applies_approval_decision ?? false,
                )}
              </Chip>
              <Chip size="sm">
                installer_blocked_until_writeback:{" "}
                {String(
                  remoteWritebackPlan?.installer_blocked_until_writeback ??
                    true,
                )}
              </Chip>
              <Chip size="sm">
                writeback_store:{" "}
                {remoteWritebackPlan?.writeback_plan?.writeback_store ||
                  "approval_queue"}
              </Chip>
              <Chip size="sm">
                queue_operation:{" "}
                {remoteWritebackPlan?.writeback_plan?.queue_operation ||
                  "plan_upsert_queue_entry"}
              </Chip>
              <Chip size="sm">
                decision_operation:{" "}
                {remoteWritebackPlan?.writeback_plan?.decision_operation ||
                  "plan_append_decision"}
              </Chip>
              <Chip size="sm">
                decision_key: {remoteWritebackPlan?.decision_key || "pending"}
              </Chip>
              <Chip size="sm">
                downloads: {String(remoteWritebackPlan?.downloads ?? false)}
              </Chip>
              <Chip size="sm">
                writes_files:{" "}
                {String(remoteWritebackPlan?.writes_files ?? false)}
              </Chip>
              <Chip size="sm">artifact: approval-writeback-plan.json</Chip>
            </div>
            {remoteWritebackPlan && (
              <TextField
                className="mt-3"
                value={JSON.stringify(remoteWritebackPlan, null, 2)}
                onChange={() => undefined}
              >
                <TextArea
                  rows={11}
                  aria-label="WASM remote install approval writeback plan result"
                  className="font-mono text-xs"
                  readOnly
                />
              </TextField>
            )}
          </Card>

          <Card className="section-card p-4">
            <div className="mb-3 flex flex-wrap items-center justify-between gap-3">
              <div>
                <div className="flex items-center gap-2 text-sm font-semibold">
                  <ShieldCheck size={16} />
                  远程安装审批 Gate 计划
                </div>
                <div
                  className="mt-1 text-xs"
                  style={{ color: "var(--yunque-text-muted)" }}
                >
                  plan-only 契约：固定“远程签名 WASM
                  包安装必须先审批”的边界，只生成 approval-gate-plan.json /
                  approval-queue-entry.json
                  预览，不写审批队列、不下载、不联网、不安装。
                </div>
              </div>
              <Button
                className="btn-accent"
                isPending={busy === "remote-approval"}
                onPress={planRemoteApproval}
              >
                生成审批计划
              </Button>
            </div>
            <TextField
              value={remoteApprovalJSON}
              onChange={setRemoteApprovalJSON}
            >
              <TextArea
                rows={8}
                aria-label="WASM remote install approval gate plan JSON"
                className="font-mono text-xs"
              />
            </TextField>
            <div className="mt-3 flex flex-wrap gap-2">
              <Chip size="sm">
                approval_gate_plan_ready:{" "}
                {String(
                  remoteApprovalPlan?.approval_gate_plan_ready ??
                    status?.approval_gate_plan_ready ??
                    false,
                )}
              </Chip>
              <Chip size="sm">
                approval_gate_ready:{" "}
                {String(
                  remoteApprovalPlan?.approval_gate_ready ??
                    status?.approval_gate_ready ??
                    false,
                )}
              </Chip>
              <Chip size="sm">
                requires_approval:{" "}
                {String(remoteApprovalPlan?.requires_approval ?? true)}
              </Chip>
              <Chip size="sm">
                writes_approval_queue:{" "}
                {String(remoteApprovalPlan?.writes_approval_queue ?? false)}
              </Chip>
              <Chip size="sm">
                approval_queue_plan_ready:{" "}
                {String(remoteApprovalPlan?.approval_queue_plan_ready ?? false)}
              </Chip>
              <Chip size="sm">
                approval_queue_ready:{" "}
                {String(remoteApprovalPlan?.approval_queue_ready ?? false)}
              </Chip>
              <Chip size="sm">
                queue_status:{" "}
                {remoteApprovalPlan?.approval_queue_entry?.status ||
                  "blocked_until_approval_queue"}
              </Chip>
              <Chip size="sm">
                request_id:{" "}
                {remoteApprovalPlan?.approval_queue_entry?.request_id ||
                  "pending"}
              </Chip>
              <Chip size="sm">
                signature_gate:{" "}
                {remoteApprovalPlan?.signature_verification?.status ||
                  "pending"}
              </Chip>
              <Chip size="sm">
                downloads: {String(remoteApprovalPlan?.downloads ?? false)}
              </Chip>
              <Chip size="sm">
                writes_files:{" "}
                {String(remoteApprovalPlan?.writes_files ?? false)}
              </Chip>
              <Chip size="sm">
                installs_plugin:{" "}
                {String(remoteApprovalPlan?.installs_plugin ?? false)}
              </Chip>
              <Chip size="sm">artifact: approval-gate-plan.json</Chip>
              <Chip size="sm">artifact: approval-queue-entry.json</Chip>
            </div>
            {remoteApprovalPlan && (
              <TextField
                className="mt-3"
                value={JSON.stringify(remoteApprovalPlan, null, 2)}
                onChange={() => undefined}
              >
                <TextArea
                  rows={11}
                  aria-label="WASM remote install approval gate plan result"
                  className="font-mono text-xs"
                  readOnly
                />
              </TextField>
            )}
          </Card>

          <Card className="section-card p-4">
            <div className="mb-3 flex flex-wrap items-center justify-between gap-3">
              <div>
                <div className="flex items-center gap-2 text-sm font-semibold">
                  <PackageCheck size={16} />
                  远程签名包安装计划
                </div>
                <div
                  className="mt-1 text-xs"
                  style={{ color: "var(--yunque-text-muted)" }}
                >
                  plan-only 契约：只生成 remote-install-plan.json /
                  signature-verification.json 预览，并固定 signature
                  verification gate
                  的算法、信任根和阻断状态；不下载、不联网、不写文件、不注册插件、不验证真实签名。
                </div>
              </div>
              <Button
                className="btn-accent"
                isPending={busy === "remote-install"}
                onPress={planRemoteInstall}
              >
                生成安装计划
              </Button>
            </div>
            <TextField
              value={remoteInstallJSON}
              onChange={setRemoteInstallJSON}
            >
              <TextArea
                rows={9}
                aria-label="WASM remote signed package install plan JSON"
                className="font-mono text-xs"
              />
            </TextField>
            <div className="mt-3 flex flex-wrap gap-2">
              <Chip size="sm">
                remote_install_plan_ready:{" "}
                {String(
                  remoteInstallPlan?.remote_install_plan_ready ??
                    status?.remote_install_plan_ready ??
                    false,
                )}
              </Chip>
              <Chip size="sm">
                remote_install_ready:{" "}
                {String(
                  remoteInstallPlan?.remote_install_ready ??
                    status?.remote_install_ready ??
                    false,
                )}
              </Chip>
              <Chip size="sm">
                download_ready:{" "}
                {String(remoteInstallPlan?.download_ready ?? false)}
              </Chip>
              <Chip size="sm">
                signature_verify_ready:{" "}
                {String(remoteInstallPlan?.signature_verify_ready ?? false)}
              </Chip>
              <Chip size="sm">
                signature_verification_plan_ready:{" "}
                {String(
                  remoteInstallPlan?.signature_verification
                    ?.signature_verification_plan_ready ??
                    status?.signature_verification_plan_ready ??
                    false,
                )}
              </Chip>
              <Chip size="sm">
                verifier_gate_ready:{" "}
                {String(
                  remoteInstallPlan?.signature_verification
                    ?.verification_gate_ready ?? false,
                )}
              </Chip>
              <Chip size="sm">
                signature_status:{" "}
                {remoteInstallPlan?.signature_verification?.status ||
                  remoteSignatureBlockedStatus}
              </Chip>
              <Chip size="sm">
                allows_install:{" "}
                {String(
                  remoteInstallPlan?.signature_verification?.allows_install ??
                    false,
                )}
              </Chip>
              <Chip size="sm">
                signature_algorithm:{" "}
                {remoteInstallPlan?.signature_verification?.algorithm ||
                  "ed25519"}
              </Chip>
              <Chip size="sm">
                trust_root:{" "}
                {remoteInstallPlan?.signature_verification?.trust_root ||
                  "yunque-root-bundle-2026"}
              </Chip>
              <Chip size="sm">
                downloads: {String(remoteInstallPlan?.downloads ?? false)}
              </Chip>
              <Chip size="sm">
                writes_files: {String(remoteInstallPlan?.writes_files ?? false)}
              </Chip>
              <Chip size="sm">
                network_access:{" "}
                {String(remoteInstallPlan?.network_access ?? false)}
              </Chip>
              <Chip size="sm">artifact: remote-install-plan.json</Chip>
              <Chip size="sm">artifact: signature-verification.json</Chip>
            </div>
            {remoteInstallPlan && (
              <TextField
                className="mt-3"
                value={JSON.stringify(remoteInstallPlan, null, 2)}
                onChange={() => undefined}
              >
                <TextArea
                  rows={12}
                  aria-label="WASM remote install plan result"
                  className="font-mono text-xs"
                  readOnly
                />
              </TextField>
            )}
          </Card>

          <Card className="section-card p-4">
            <div className="mb-3 flex flex-wrap items-center justify-between gap-3">
              <div>
                <div className="flex items-center gap-2 text-sm font-semibold">
                  <Play size={16} />
                  沙箱执行计划
                </div>
                <div
                  className="mt-1 text-xs"
                  style={{ color: "var(--yunque-text-muted)" }}
                >
                  目标插件：{selectedPlugin?.slug || slug}
                </div>
              </div>
              <div className="flex gap-2">
                <Button
                  isPending={busy === "unload"}
                  onPress={() => setLoaded(false)}
                  isDisabled={!selectedPlugin}
                >
                  <Power size={14} />
                  Unload
                </Button>
                <Button
                  isPending={busy === "load"}
                  onPress={() => setLoaded(true)}
                  isDisabled={!selectedPlugin}
                >
                  <Power size={14} />
                  Load
                </Button>
                <Button
                  isPending={busy === "evidence"}
                  onPress={exportEvidence}
                  isDisabled={!selectedPlugin && !slug}
                >
                  <Download size={14} />
                  导出证据包
                </Button>
                <Button
                  className="btn-accent"
                  isPending={busy === "execute"}
                  onPress={executeDryRun}
                  isDisabled={!selectedPlugin && !slug}
                >
                  Dry-run
                </Button>
              </div>
            </div>
            <TextField value={inputJSON} onChange={setInputJSON}>
              <TextArea
                rows={4}
                aria-label="WASM plugin input JSON"
                className="font-mono text-xs"
              />
            </TextField>
            {result ? (
              <Card
                className="mt-3 p-3"
                style={{ background: "rgba(255,255,255,0.03)" }}
              >
                <div className="mb-2 flex items-center gap-2 text-sm font-medium">
                  <Chip size="sm">
                    {result.dry_run ? "dry-run" : "execute"}
                  </Chip>
                  <span>{result.entrypoint}</span>
                </div>
                <div className="mb-2 flex flex-wrap gap-2">
                  <Chip size="sm">
                    abi_plan_ready: {String(result.host_abi_plan?.plan_ready)}
                  </Chip>
                  <Chip size="sm">
                    abi_ready: {String(result.host_abi_plan?.ready)}
                  </Chip>
                  <Chip size="sm">
                    enforcement_ready:{" "}
                    {String(result.host_abi_plan?.enforcement_ready)}
                  </Chip>
                  <Chip size="sm">
                    writes_files: {String(result.host_abi_plan?.writes_files)}
                  </Chip>
                  <Chip size="sm">
                    network_access:{" "}
                    {String(result.host_abi_plan?.network_access)}
                  </Chip>
                  <Chip size="sm">
                    enabled ABI:{" "}
                    {result.host_abi_plan?.summary?.enabled_count ?? 0}/
                    {result.host_abi_plan?.summary?.function_count ?? 0}
                  </Chip>
                  <Chip size="sm">
                    execution_gate_ready:{" "}
                    {String(result.host_abi_gate?.execution_gate_ready)}
                  </Chip>
                  <Chip size="sm">
                    allows_execution:{" "}
                    {String(result.host_abi_gate?.allows_execution)}
                  </Chip>
                  <Chip size="sm">
                    blocked: {String(result.host_abi_gate?.blocked)}
                  </Chip>
                  <Chip size="sm">
                    gate: {result.host_abi_gate?.status || "unknown"}
                  </Chip>
                  <Chip size="sm">
                    integrity_gate_ready:{" "}
                    {String(result.module_integrity_gate?.integrity_gate_ready)}
                  </Chip>
                  <Chip size="sm">
                    integrity:{" "}
                    {result.module_integrity_gate?.status || "unknown"}
                  </Chip>
                  <Chip size="sm">
                    sha256_blocked:{" "}
                    {String(result.module_integrity_gate?.blocked)}
                  </Chip>
                </div>
                <TextField
                  value={JSON.stringify(result, null, 2)}
                  onChange={() => undefined}
                >
                  <TextArea
                    rows={10}
                    aria-label="WASM Plugin execution result"
                    className="font-mono text-xs"
                    readOnly
                  />
                </TextField>
                <div
                  className="mt-2 text-xs"
                  style={{ color: "var(--yunque-text-muted)" }}
                >
                  Host ABI plan preview 仅固定后续 host function
                  权限强执行契约；Host ABI execution gate
                  会在真实执行前阻断已请求特权 Host ABI 的插件，直到
                  enforcement_ready=true。module integrity gate 会在 sandbox
                  执行前比对本地 WASM SHA-256 与注册元数据，防止模块漂移。
                  当前不会绑定 wazero host functions、不会写文件，也不会绕过
                  Pack Runtime gate。
                </div>
              </Card>
            ) : (
              <div
                className="mt-3 rounded-xl border border-dashed p-6 text-center text-sm"
                style={{
                  borderColor: "var(--yunque-border)",
                  color: "var(--yunque-text-muted)",
                }}
              >
                选择 loaded 插件后，可以先生成权限计划与沙箱 dry-run 执行计划。
              </div>
            )}
          </Card>
        </div>
      </div>
    </div>
  );
}
