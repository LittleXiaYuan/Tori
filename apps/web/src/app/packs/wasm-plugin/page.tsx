"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import Link from "next/link";
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
  ClipboardList,
  Cpu,
  Download,
  FileCode2,
  PackageCheck,
  Play,
  Power,
  RefreshCw,
  Send,
  ShieldCheck,
} from "lucide-react";
import PageHeader from "@/components/page-header";
import ReadinessBadges from "@/components/readiness-badges";
import { JsonViewer } from "@/components/json-viewer";
import { showToast } from "@/components/toast-provider";
import { formatErrorMessage } from "@/lib/error-utils";
import { chatPromptHref } from "@/lib/pack-action-links";
import {
  createWASMPluginClient as createWASMPluginPackClient,
  type WASMPluginExecuteResult,
  type WASMPluginRemoteInstallApprovalDecisionPlan,
  type WASMPluginRemoteInstallApprovalPlan,
  type WASMPluginRemoteInstallApprovalQueueWriteback,
  type WASMPluginRemoteInstallApprovalWritebackPlan,
  type WASMPluginRemoteInstallInstallerContinuationPlan,
  type WASMPluginRemoteInstallInstallerDownloadWriteback,
  type WASMPluginRemoteInstallInstallerRegistrationPlan,
  type WASMPluginRemoteInstallPackageInspectWriteback,
  type WASMPluginRemoteInstallPlan,
  type WASMPluginRemoteInstallSignatureVerificationWriteback,
  type WASMPluginStatusResponse as WASMPluginStatus,
  type WASMPluginSummary,
} from "yunque-client/wasm-plugin";
import { createYunqueSDKClientOptions } from "@/lib/sdk-client";
import { PackAbout, PackSectionTitle, PackStepsGrid, type PackBoundaryItem, type PackStep } from "@/components/packs/pack-page-kit";

const wasmPluginPack = createWASMPluginPackClient(createYunqueSDKClientOptions());

type ChipColor = "success" | "warning" | "danger" | "default";

// The status response is ~30 boolean readiness flags. Rendering each as a
// KPI-size card overflowed and truncated technical tokens; instead we surface
// the meaningful acceptance gates as a compact badge strip (ready / pending).
const READINESS_FLAGS: {
  label: string;
  hint: string;
  ready: (s: WASMPluginStatus | null) => boolean | undefined;
}[] = [
  { label: "Host ABI 计划", hint: "Host ABI plan preview 就绪", ready: (s) => s?.abi_plan_ready },
  { label: "执行 Gate", hint: "真实执行前的 Host ABI execution gate 就绪", ready: (s) => s?.host_abi_execution_gate_ready },
  { label: "模块完整性", hint: "模块 SHA-256 完整性 gate 就绪", ready: (s) => s?.module_integrity_gate_ready },
  { label: "签名校验", hint: "远程包签名校验计划就绪", ready: (s) => s?.signature_verification_plan_ready },
  { label: "远程安装", hint: "远程安装计划就绪", ready: (s) => s?.remote_install_plan_ready },
  { label: "审批 Gate", hint: "远程安装审批 gate 计划就绪", ready: (s) => s?.approval_gate_plan_ready },
  { label: "审批决策", hint: "审批决策计划就绪", ready: (s) => s?.approval_decision_plan_ready },
  { label: "审批写回", hint: "审批写回计划 / 队列就绪", ready: (s) => s?.approval_writeback_plan_ready || s?.approval_queue_store_ready },
  { label: "Installer", hint: "Installer 注册与接线就绪", ready: (s) => s?.installer_ready },
];

function statusColor(status: WASMPluginStatus | null): ChipColor {
  if (!status) return "default";
  if (status.runtime_ready && status.abi_ready) return "success";
  if (status.runtime_ready) return "warning";
  return "danger";
}

const userFacingSteps: PackStep[] = [
  { key: "check", label: "先看能不能接入", detail: "校验插件清单、权限、模块 SHA-256 和 Host ABI 请求，先发现风险。" },
  { key: "preview", label: "再预演远程安装", detail: "对 packageUrl、签名、审批、下载缓存、验签和包结构逐段生成计划。" },
  { key: "evidence", label: "最后留证据再放行", detail: "导出证据包，明确哪些记录只写在 pack-local，哪些还不能进入真实执行。" },
];

const boundaryItems: PackBoundaryItem[] = [
  { key: "approval", label: "不绕审批", detail: "不会绕过审批直接安装远程包。", tone: "danger" },
  { key: "unpack", label: "不解未验签包", detail: "不会把未验签包解包到 plugin_dir。", tone: "danger" },
  { key: "abi", label: "不放开特权函数", detail: "不会在 Host ABI 强执行未就绪时放开特权函数。", tone: "danger" },
  { key: "market", label: "不冒充插件市场", detail: "不会把实验链路包装成稳定第三方插件市场。", tone: "warning" },
];

const workflowSteps: PackStep[] = [
  { key: "gate", label: "检查接入条件", detail: "先用清单、签名、SHA-256、审批和 Host ABI gate 判断这个能力能不能安全进入云雀。" },
  { key: "chat", label: "带回 Chat", detail: "把检查结果交给云雀解释，拆出需要修的权限、签名、入口或 WASM 模块问题。" },
  { key: "evidence", label: "看证据位置", detail: "远程安装计划、写回记录和证据包是验收材料，不等于已经安装或执行。" },
  { key: "improve", label: "继续交给小羽改", detail: "如果能力不完整，把缺口带进工坊，让小羽改 yqpack 后再验收、打包和回滚。" },
];

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

function sampleInstallerDownloadWriteback(slug: string) {
  return JSON.stringify(
    {
      slug,
      request_key: "preview-request-key",
      approved: true,
      approved_by: "security",
      reason:
        "download approved package into pack-local installer cache only; keep signature verification and registration blocked",
      metadata: { ticket: "WASM-REMOTE-DOWNLOAD-1" },
    },
    null,
    2,
  );
}

function sampleSignatureVerificationWriteback(slug: string) {
  return JSON.stringify(
    {
      slug,
      request_key: "preview-request-key",
      approved: true,
      verified_by: "security",
      reason:
        "verify the cached package signature and write only the pack-local signature verification store",
      metadata: { ticket: "WASM-REMOTE-SIGNATURE-1" },
    },
    null,
    2,
  );
}

function samplePackageInspectWriteback(slug: string) {
  return JSON.stringify(
    {
      slug,
      request_key: "preview-request-key",
      approved: true,
      inspected_by: "security",
      reason:
        "inspect the verified package archive for safe manifest/module layout; keep plugin_dir writes and registration blocked",
      metadata: { ticket: "WASM-REMOTE-PACKAGE-INSPECT-1" },
    },
    null,
    2,
  );
}

function sampleInstallerRegistrationPlan(slug: string) {
  return JSON.stringify(
    {
      slug,
      request_key: "preview-request-key",
      approved: true,
      approved_by: "security",
      reason:
        "plan plugin registration handoff from verified package inspect record; keep extraction and plugin metadata writeback blocked",
      metadata: { ticket: "WASM-REMOTE-REGISTRATION-1" },
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
    | "remote-installer-download"
    | "remote-signature-verification"
    | "remote-package-inspect"
    | "remote-installer-registration"
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
  const [remoteInstallerDownloadJSON, setRemoteInstallerDownloadJSON] =
    useState(() => sampleInstallerDownloadWriteback("calculator-remote"));
  const [remoteInstallerDownload, setRemoteInstallerDownload] =
    useState<WASMPluginRemoteInstallInstallerDownloadWriteback | null>(null);
  const [remoteSignatureVerificationJSON, setRemoteSignatureVerificationJSON] =
    useState(() => sampleSignatureVerificationWriteback("calculator-remote"));
  const [remoteSignatureVerification, setRemoteSignatureVerification] =
    useState<WASMPluginRemoteInstallSignatureVerificationWriteback | null>(
      null,
    );
  const [remotePackageInspectJSON, setRemotePackageInspectJSON] = useState(() =>
    samplePackageInspectWriteback("calculator-remote"),
  );
  const [remotePackageInspect, setRemotePackageInspect] =
    useState<WASMPluginRemoteInstallPackageInspectWriteback | null>(null);
  const [remoteInstallerRegistrationJSON, setRemoteInstallerRegistrationJSON] =
    useState(() => sampleInstallerRegistrationPlan("calculator-remote"));
  const [remoteInstallerRegistration, setRemoteInstallerRegistration] =
    useState<WASMPluginRemoteInstallInstallerRegistrationPlan | null>(null);
  const [inputJSON, setInputJSON] = useState(() =>
    JSON.stringify({ a: 1, b: 2 }, null, 2),
  );
  const [result, setResult] = useState<WASMPluginExecuteResult | null>(null);
  const runtimeStatusColor = statusColor(status);

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
          ? "WASM Plugin Pack 当前未启用。请到「能力包」控制台启用 yunque.pack.wasm-plugin 后再使用。"
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

  const writeInstallerDownload = async () => {
    setBusy("remote-installer-download");
    setError(null);
    try {
      const payload = JSON.parse(remoteInstallerDownloadJSON);
      const res =
        await wasmPluginPack.remoteInstallInstallerDownloadWriteback(payload);
      setRemoteInstallerDownload(res.writeback);
      showToast("已写入 pack-local installer 下载缓存", "success");
      await load();
    } catch (e) {
      setError(formatErrorMessage(e, "写入 installer 下载缓存失败"));
    } finally {
      setBusy(null);
    }
  };

  const writeSignatureVerification = async () => {
    setBusy("remote-signature-verification");
    setError(null);
    try {
      const payload = JSON.parse(remoteSignatureVerificationJSON);
      const res =
        await wasmPluginPack.remoteInstallSignatureVerificationWriteback(
          payload,
        );
      setRemoteSignatureVerification(res.writeback);
      showToast("已写入 pack-local 签名验证记录", "success");
      await load();
    } catch (e) {
      setError(formatErrorMessage(e, "写入签名验证记录失败"));
    } finally {
      setBusy(null);
    }
  };

  const writePackageInspect = async () => {
    setBusy("remote-package-inspect");
    setError(null);
    try {
      const payload = JSON.parse(remotePackageInspectJSON);
      const res =
        await wasmPluginPack.remoteInstallPackageInspectWriteback(payload);
      setRemotePackageInspect(res.writeback);
      showToast("已写入 pack-local 包结构检查记录", "success");
      await load();
    } catch (e) {
      setError(formatErrorMessage(e, "写入包结构检查记录失败"));
    } finally {
      setBusy(null);
    }
  };

  const planInstallerRegistration = async () => {
    setBusy("remote-installer-registration");
    setError(null);
    try {
      const payload = JSON.parse(remoteInstallerRegistrationJSON);
      const res =
        await wasmPluginPack.remoteInstallInstallerRegistrationPlan(payload);
      setRemoteInstallerRegistration(res.plan);
      showToast("已生成 installer registration handoff 计划", "success");
    } catch (e) {
      setError(
        formatErrorMessage(e, "生成 installer registration handoff 计划失败"),
      );
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
      <PageHeader
        icon={<Cpu size={20} />}
        title="WASM 插件引擎"
        actions={<Button size="sm" variant="ghost" onPress={load}><RefreshCw size={14} />刷新</Button>}
      />

      <PackAbout
        chips={<>
          <Chip size="sm" color="danger">高风险实验能力</Chip>
          <Chip size="sm" variant="soft">远程安装先计划</Chip>
          <Chip size="sm" variant="soft">沙箱执行先 dry-run</Chip>
        </>}
        description="第三方 WASM 能力进入云雀前的验收台：先检查插件清单、权限、签名、包结构和 Host ABI 边界，再决定是否继续安装或执行。当前更适合开发者和 Pack 作者验证接入链路，不是给普通用户直接下载应用的商店页。"
        boundaries={boundaryItems}
      />

      <div className="grid gap-3 md:grid-cols-3">
        {userFacingSteps.map((item, idx) => (
          <div key={item.key} className="rounded-xl border border-border bg-surface-secondary p-4">
            <div className="text-sm font-medium text-foreground">{idx + 1}. {item.label}</div>
            <div className="mt-2 text-xs leading-5 text-muted">{item.detail}</div>
          </div>
        ))}
      </div>

      <Card variant="default">
        <Card.Header className="flex-row flex-wrap items-start justify-between gap-3">
          <div className="flex flex-col gap-1">
            <PackSectionTitle icon={<PackageCheck size={15} />} tone="accent">从 WASM 验收到可用能力</PackSectionTitle>
            <span className="text-xs leading-5 text-muted">
              WASM 能力包的价值不是让用户读一堆技术 gate，而是把第三方能力先验收、留证据，再交给 Chat、任务中心和小羽逐步补成可用能力。
            </span>
          </div>
          <div className="flex flex-wrap gap-2">
            <Link
              href={chatPromptHref(
                "请检查 WASM 能力包当前的清单、签名、权限、Host ABI 和 dry-run 结果，指出能安全接入的部分、仍被阻断的原因，并把下一步拆成任务。",
              )}
            >
              <Button size="sm" className="btn-accent">
                <Send size={13} /> 带回 Chat
              </Button>
            </Link>
            <Link href="/missions">
              <Button size="sm" variant="outline">
                <ClipboardList size={13} /> 看任务
              </Button>
            </Link>
          </div>
        </Card.Header>
        <Card.Content className="flex flex-col gap-3">
          <PackStepsGrid steps={workflowSteps} columns={4} />
          <div className="flex flex-wrap gap-2 text-xs">
            <Link href="/trace">
              <Button size="sm" variant="ghost">核对执行轨迹</Button>
            </Link>
            <Link href="/packs/studio?packId=yunque.pack.wasm-plugin">
              <Button size="sm" variant="ghost">让小羽继续改</Button>
            </Link>
          </div>
        </Card.Content>
      </Card>

      <Card className="section-card p-4">
        <div className="flex flex-wrap items-start justify-between gap-3">
          <div>
            <div className="mb-3 text-sm font-semibold text-foreground">
              技术状态
            </div>
            <div className="mb-1 flex items-center gap-2">
              <Chip size="sm" color={runtimeStatusColor}>
                {status?.abi_ready
                  ? "ABI ready"
                  : status?.runtime_ready
                    ? "Runtime shell"
                    : "Disabled"}
              </Chip>
              <Chip size="sm" color={status?.abi_plan_ready ? "default" : "warning"} variant="soft">
                {status?.abi_plan_ready ? "Host ABI plan" : "ABI plan pending"}
              </Chip>
              <Chip size="sm" color={status?.host_abi_execution_gate_ready ? "success" : "warning"}>
                {status?.host_abi_execution_gate_ready
                  ? "Host ABI execution gate"
                  : "Host ABI gate pending"}
              </Chip>
              <Chip size="sm" color={status?.remote_install_plan_ready ? "default" : "warning"} variant="soft">
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
                installer_download_writeback_ready:{" "}
                {String(status?.installer_download_writeback_ready ?? false)}
              </Chip>
              <Chip size="sm">
                signature_verification_writeback_ready:{" "}
                {String(
                  status?.signature_verification_writeback_ready ?? false,
                )}
              </Chip>
              <Chip size="sm">
                package_inspect_writeback_ready:{" "}
                {String(status?.package_inspect_writeback_ready ?? false)}
              </Chip>
              <Chip size="sm">
                installer_registration_plan_ready:{" "}
                {String(status?.installer_registration_plan_ready ?? false)}
              </Chip>
              <Chip size="sm">
                registration_ready:{" "}
                {String(status?.registration_ready ?? false)}
              </Chip>
              <Chip size="sm">
                installer_blocked_until_registration:{" "}
                {String(status?.installer_blocked_until_registration ?? true)}
              </Chip>
              <Chip size="sm">
                installer_ready: {String(status?.installer_ready ?? false)}
              </Chip>
              <span className="text-xs text-muted">
                {status?.pack_id || "yunque.pack.wasm-plugin"}
              </span>
            </div>
            <details className="mt-3 rounded-lg border border-border bg-surface-secondary p-3 text-sm text-muted">
              <summary className="cursor-pointer font-medium text-foreground">
                查看技术链路详情
              </summary>
              <div className="mt-3 leading-6 text-muted">
              当前切片先把 WASM 插件注册、load/unload 生命周期、沙箱执行
              dry-run、权限计划、Host ABI plan preview、真实执行前 Host ABI
              execution gate、远程签名包安装计划、远程安装审批 gate
              计划、审批决策 plan-only 预览、审批队列写回桥接计划、pack-local
              审批队列 JSON 持久化和证据包放进可选 Pack。请求 ledger_kv /
              memory_search / http_fetch / env_get 的插件在
              host_abi_enforcement_ready=false 时会被真实执行前阻断；本地 WASM
              模块 SHA-256 与注册元数据不一致时也会被 module integrity gate
              阻断，不进入 sandbox；当前审批写回只落 pack-local queue store，
              installer continuation 产出 handoff 计划；download writeback
              只把已审批包下载到 pack-local installer cache 并校验 SHA-256；
              signature verification writeback 再消费 installer-download-store，
              对缓存包执行 Ed25519 验签，只写 signature-verification-store.json
              / record，不解包、不写 plugin_dir、不注册插件；package inspect
              writeback 再读取已验签缓存包， 只检查 tar.gz 内 manifest 与 WASM
              module 布局并写 package-inspect-store.json / record，不解包到
              plugin_dir、不注册插件； installer registration handoff 继续消费
              package-inspect-store.json 生成
              installer-registration-handoff-plan.json /
              plugin-registration-handoff-plan.json /
              installer-audit-handoff-plan.json，
              只说明后续注册写回需要的字段，不解包、不写插件文件、不注册元数据；
              继续保持 remote_install_ready=false / installer_ready=false /
              installer_blocked_until_registration=true。 后续 install
              写回、插件注册、Host ABI 权限强执行和 TinyGo 示例会继续接入。
              </div>
            </details>
          </div>
          <Button size="sm" variant="ghost" onPress={load}>
            <RefreshCw size={14} />
            刷新
          </Button>
        </div>
      </Card>

      {error && (
        <Card variant="secondary">
          <Card.Content className="flex items-center gap-2 p-4 text-sm text-danger">
            <AlertTriangle size={16} />
            {error}
          </Card.Content>
        </Card>
      )}

      {/* Two real counts kept as compact metric tiles; everything else is a
          readiness flag, not a metric — those move into the status strip below
          so technical tokens (wazero / sha256 / plan) never get KPI-size font. */}
      <div className="grid grid-cols-2 gap-4 sm:grid-cols-4">
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
          <div className="kpi-label">阶段</div>
          <div className="mt-1 text-sm font-medium text-foreground">
            {status?.stage || "pack-shell"}
          </div>
        </Card>
        <Card className="section-card p-4">
          <div className="kpi-label">Runtime</div>
          <div className="mt-1 text-sm font-medium text-foreground">
            {status?.runtime_ready ? "wazero" : "未就绪"}
          </div>
        </Card>
      </div>

      <Card className="section-card p-4">
        <div className="flex items-center gap-2 text-sm font-semibold text-foreground">
          <ShieldCheck size={16} className="text-accent" />
          就绪状态
        </div>
        <p className="mt-1 text-xs text-muted">
          各验收 gate 与计划环节的就绪情况 —— 绿色表示已就绪，灰色表示仍待接通。
        </p>
        <div className="mt-3">
          <ReadinessBadges flags={READINESS_FLAGS.map((f) => ({ label: f.label, hint: f.hint, ready: f.ready(status) }))} />
        </div>
      </Card>

      <div className="grid grid-cols-1 gap-4 xl:grid-cols-[360px_1fr]">
        <Card className="section-card overflow-hidden">
          <div className="flex items-center justify-between border-b border-border px-4 py-3">
            <div className="flex items-center gap-2 text-sm font-semibold">
              <FileCode2 size={16} />
              已注册 WASM 插件
            </div>
            <Chip size="sm">{plugins.length}</Chip>
          </div>
          <div className="max-h-[520px] divide-y divide-border overflow-auto">
            {plugins.length === 0 ? (
              <div className="p-6 text-center text-sm text-muted">
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
                  <div className="mt-1 truncate text-xs text-muted">
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
              <TextField aria-label="WASM plugin slug" className="w-56" value={slug} onChange={setSlug}>
                <Input placeholder="plugin slug" />
              </TextField>
            </div>
            <TextField aria-label="WASM plugin manifest JSON" value={manifestJSON} onChange={setManifestJSON}>
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
                  <PackageCheck size={16} />
                  Package inspect writeback
                </div>
                <div
                  className="mt-1 text-xs text-muted"
                >
                  包结构检查边界：消费 pack-local
                  signature-verification-store.json / record，读取已验签缓存
                  tar.gz，只确认 manifest.json / plugin.json 与目标 .wasm
                  模块存在、路径安全、SHA-256 证据可复核；只写
                  package-inspect-store.json / package-inspect-record.json。
                  它不解包到 plugin_dir、不注册插件、不加载模块，也不会把
                  remote_install_ready / installer_ready 置为 true。
                </div>
              </div>
              <Button
                className="btn-accent"
                isPending={busy === "remote-package-inspect"}
                onPress={writePackageInspect}
              >
                写入包检查
              </Button>
            </div>
            <TextField
              aria-label="WASM remote install package inspect writeback JSON"
              value={remotePackageInspectJSON}
              onChange={setRemotePackageInspectJSON}
            >
              <TextArea
                rows={7}
                aria-label="WASM remote install package inspect writeback JSON"
                className="font-mono text-xs"
              />
            </TextField>
            <div className="mt-3 flex flex-wrap gap-2">
              <Chip size="sm">
                package_inspect_writeback_ready:{" "}
                {String(
                  remotePackageInspect?.package_inspect_writeback_ready ??
                    status?.package_inspect_writeback_ready ??
                    false,
                )}
              </Chip>
              <Chip size="sm">
                package_inspect_ready:{" "}
                {String(remotePackageInspect?.package_inspect_ready ?? false)}
              </Chip>
              <Chip size="sm">
                package_layout_ready:{" "}
                {String(remotePackageInspect?.package_layout_ready ?? false)}
              </Chip>
              <Chip size="sm">
                manifest_found:{" "}
                {String(remotePackageInspect?.manifest_found ?? false)}
              </Chip>
              <Chip size="sm">
                wasm_module_found:{" "}
                {String(remotePackageInspect?.wasm_module_found ?? false)}
              </Chip>
              <Chip size="sm">
                writes_package_inspect_store:{" "}
                {String(
                  remotePackageInspect?.writes_package_inspect_store ?? false,
                )}
              </Chip>
              <Chip size="sm">
                writes_files:{" "}
                {String(remotePackageInspect?.writes_files ?? false)}
              </Chip>
              <Chip size="sm">
                remote_install_ready:{" "}
                {String(remotePackageInspect?.remote_install_ready ?? false)}
              </Chip>
              <Chip size="sm">
                installs_plugin:{" "}
                {String(remotePackageInspect?.installs_plugin ?? false)}
              </Chip>
              <Chip size="sm">
                installer_blocked_until_registration:{" "}
                {String(
                  remotePackageInspect?.installer_blocked_until_registration ??
                    status?.installer_blocked_until_registration ??
                    true,
                )}
              </Chip>
              <Chip size="sm">artifact: package-inspection.json</Chip>
              <Chip size="sm">artifact: package-inspect-record.json</Chip>
              <Chip size="sm">artifact: package-inspect-store.json</Chip>
            </div>
            {remotePackageInspect && (
              <div className="mt-3">
                <JsonViewer title="Package inspect writeback result JSON" value={remotePackageInspect} rows={11} />
              </div>
            )}
          </Card>

          <Card className="section-card p-4">
            <div className="mb-3 flex flex-wrap items-center justify-between gap-3">
              <div>
                <div className="flex items-center gap-2 text-sm font-semibold">
                  <ShieldCheck size={16} />
                  Installer registration handoff 计划
                </div>
                <div
                  className="mt-1 text-xs text-muted"
                >
                  plan-only 契约：消费 package-inspect-store.json /
                  package-inspect-record.json，确认 package_inspect_ready /
                  signature_verified / manifest_found / wasm_module_found
                  后，只生成 installer-registration-handoff-plan.json、
                  plugin-registration-handoff-plan.json 与
                  installer-audit-handoff-plan.json。它不解包、不写
                  plugin_dir、不注册插件元数据、不加载 WASM，也不会把
                  remote_install_ready / installer_ready 置为 true。
                </div>
              </div>
              <Button
                className="btn-accent"
                isPending={busy === "remote-installer-registration"}
                onPress={planInstallerRegistration}
              >
                生成注册 handoff
              </Button>
            </div>
            <TextField
              aria-label="WASM remote install installer registration plan JSON"
              value={remoteInstallerRegistrationJSON}
              onChange={setRemoteInstallerRegistrationJSON}
            >
              <TextArea
                rows={7}
                aria-label="WASM remote install installer registration plan JSON"
                className="font-mono text-xs"
              />
            </TextField>
            <div className="mt-3 flex flex-wrap gap-2">
              <Chip size="sm">
                installer_registration_plan_ready:{" "}
                {String(
                  remoteInstallerRegistration
                    ?.installer_registration_plan_ready ??
                    status?.installer_registration_plan_ready ??
                    false,
                )}
              </Chip>
              <Chip size="sm">
                consumes_package_inspect_store:{" "}
                {String(
                  remoteInstallerRegistration
                    ?.consumes_package_inspect_store ?? false,
                )}
              </Chip>
              <Chip size="sm">
                package_inspect_record_found:{" "}
                {String(
                  remoteInstallerRegistration
                    ?.package_inspect_record_found ?? false,
                )}
              </Chip>
              <Chip size="sm">
                package_layout_ready:{" "}
                {String(
                  remoteInstallerRegistration?.package_layout_ready ?? false,
                )}
              </Chip>
              <Chip size="sm">
                signature_verified:{" "}
                {String(
                  remoteInstallerRegistration?.signature_verified ?? false,
                )}
              </Chip>
              <Chip size="sm">
                approval_provided:{" "}
                {String(
                  remoteInstallerRegistration?.approval_provided ?? false,
                )}
              </Chip>
              <Chip size="sm">
                would_register_plugin:{" "}
                {String(
                  remoteInstallerRegistration?.would_register_plugin ?? false,
                )}
              </Chip>
              <Chip size="sm">
                registration_ready:{" "}
                {String(
                  remoteInstallerRegistration?.registration_ready ??
                    status?.registration_ready ??
                    false,
                )}
              </Chip>
              <Chip size="sm">
                writes_files:{" "}
                {String(remoteInstallerRegistration?.writes_files ?? false)}
              </Chip>
              <Chip size="sm">
                remote_install_ready:{" "}
                {String(
                  remoteInstallerRegistration?.remote_install_ready ?? false,
                )}
              </Chip>
              <Chip size="sm">
                installs_plugin:{" "}
                {String(remoteInstallerRegistration?.installs_plugin ?? false)}
              </Chip>
              <Chip size="sm">
                artifact: installer-registration-handoff-plan.json
              </Chip>
              <Chip size="sm">
                artifact: plugin-registration-handoff-plan.json
              </Chip>
              <Chip size="sm">artifact: installer-audit-handoff-plan.json</Chip>
            </div>
            {remoteInstallerRegistration && (
              <div className="mt-3">
                <JsonViewer title="Installer registration handoff plan result JSON" value={remoteInstallerRegistration} rows={11} />
              </div>
            )}
          </Card>

          <Card className="section-card p-4">
            <div className="mb-3 flex flex-wrap items-center justify-between gap-3">
              <div>
                <div className="flex items-center gap-2 text-sm font-semibold">
                  <ShieldCheck size={16} />
                  Installer continuation handoff 计划
                </div>
                <div
                  className="mt-1 text-xs text-muted"
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
              aria-label="WASM remote install installer continuation plan JSON"
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
                  remoteInstallerPlan?.installer_blocked_until_installer_wiring ??
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
              <div className="mt-3">
                <JsonViewer title="Installer continuation plan result JSON" value={remoteInstallerPlan} rows={11} />
              </div>
            )}
          </Card>

          <Card className="section-card p-4">
            <div className="mb-3 flex flex-wrap items-center justify-between gap-3">
              <div>
                <div className="flex items-center gap-2 text-sm font-semibold">
                  <Download size={16} />
                  Installer download writeback
                </div>
                <div
                  className="mt-1 text-xs text-muted"
                >
                  真实写入边界：读取已 approved 的 pack-local approval queue
                  record，并要求本次请求显式 approved=true；下载包后只写
                  pack-local installer-cache 与 installer-download-record.json，
                  校验 SHA-256 匹配。它不会验签、不会解包、不会写 plugin_dir、
                  不会注册插件，也不会把 remote_install_ready / installer_ready
                  置为 true。
                </div>
              </div>
              <Button
                className="btn-accent"
                isPending={busy === "remote-installer-download"}
                onPress={writeInstallerDownload}
              >
                写入下载缓存
              </Button>
            </div>
            <TextField
              aria-label="WASM remote install installer download writeback JSON"
              value={remoteInstallerDownloadJSON}
              onChange={setRemoteInstallerDownloadJSON}
            >
              <TextArea
                rows={7}
                aria-label="WASM remote install installer download writeback JSON"
                className="font-mono text-xs"
              />
            </TextField>
            <div className="mt-3 flex flex-wrap gap-2">
              <Chip size="sm">
                installer_download_writeback_ready:{" "}
                {String(
                  remoteInstallerDownload?.installer_download_writeback_ready ??
                    status?.installer_download_writeback_ready ??
                    false,
                )}
              </Chip>
              <Chip size="sm">
                download_ready:{" "}
                {String(remoteInstallerDownload?.download_ready ?? false)}
              </Chip>
              <Chip size="sm">
                downloads: {String(remoteInstallerDownload?.downloads ?? false)}
              </Chip>
              <Chip size="sm">
                network_access:{" "}
                {String(remoteInstallerDownload?.network_access ?? false)}
              </Chip>
              <Chip size="sm">
                writes_package_cache:{" "}
                {String(remoteInstallerDownload?.writes_package_cache ?? false)}
              </Chip>
              <Chip size="sm">
                writes_files:{" "}
                {String(remoteInstallerDownload?.writes_files ?? false)}
              </Chip>
              <Chip size="sm">
                signature_verify_ready:{" "}
                {String(
                  remoteInstallerDownload?.signature_verify_ready ?? false,
                )}
              </Chip>
              <Chip size="sm">
                remote_install_ready:{" "}
                {String(remoteInstallerDownload?.remote_install_ready ?? false)}
              </Chip>
              <Chip size="sm">
                installs_plugin:{" "}
                {String(remoteInstallerDownload?.installs_plugin ?? false)}
              </Chip>
              <Chip size="sm">
                installer_blocked_until_signature_verify:{" "}
                {String(
                  remoteInstallerDownload?.installer_blocked_until_signature_verify ??
                    status?.installer_blocked_until_signature_verify ??
                    true,
                )}
              </Chip>
              <Chip size="sm">
                installer_download_record sha256_match:{" "}
                {String(
                  remoteInstallerDownload?.download_record?.sha256_match ??
                    false,
                )}
              </Chip>
              <Chip size="sm">artifact: installer-download-record.json</Chip>
              <Chip size="sm">artifact: installer-package-cache.tgz</Chip>
            </div>
            {remoteInstallerDownload && (
              <div className="mt-3">
                <JsonViewer title="Installer download writeback result JSON" value={remoteInstallerDownload} rows={11} />
              </div>
            )}
          </Card>

          <Card className="section-card p-4">
            <div className="mb-3 flex flex-wrap items-center justify-between gap-3">
              <div>
                <div className="flex items-center gap-2 text-sm font-semibold">
                  <ShieldCheck size={16} />
                  Signature verification writeback
                </div>
                <div
                  className="mt-1 text-xs text-muted"
                >
                  验签写回边界：消费 pack-local installer-download-store.json /
                  installer-download-record.json， 读取缓存包并用审批记录中的
                  Ed25519 签名材料验证；只写 signature-verification-store.json
                  与 signature-verification-record.json。它不下载、不解包、不写
                  plugin_dir、不注册插件，也不会把 remote_install_ready /
                  installer_ready 置为 true。
                </div>
              </div>
              <Button
                className="btn-accent"
                isPending={busy === "remote-signature-verification"}
                onPress={writeSignatureVerification}
              >
                写入验签记录
              </Button>
            </div>
            <TextField
              aria-label="WASM remote install signature verification writeback JSON"
              value={remoteSignatureVerificationJSON}
              onChange={setRemoteSignatureVerificationJSON}
            >
              <TextArea
                rows={7}
                aria-label="WASM remote install signature verification writeback JSON"
                className="font-mono text-xs"
              />
            </TextField>
            <div className="mt-3 flex flex-wrap gap-2">
              <Chip size="sm">
                signature_verification_writeback_ready:{" "}
                {String(
                  remoteSignatureVerification?.signature_verification_writeback_ready ??
                    status?.signature_verification_writeback_ready ??
                    false,
                )}
              </Chip>
              <Chip size="sm">
                package_cache_ready:{" "}
                {String(
                  remoteSignatureVerification?.package_cache_ready ?? false,
                )}
              </Chip>
              <Chip size="sm">
                signature_verify_ready:{" "}
                {String(
                  remoteSignatureVerification?.signature_verify_ready ?? false,
                )}
              </Chip>
              <Chip size="sm">
                signature_verified:{" "}
                {String(
                  remoteSignatureVerification?.signature_verified ?? false,
                )}
              </Chip>
              <Chip size="sm">
                allows_installer_writeback:{" "}
                {String(
                  remoteSignatureVerification?.allows_installer_writeback ??
                    false,
                )}
              </Chip>
              <Chip size="sm">
                writes_signature_verification_store:{" "}
                {String(
                  remoteSignatureVerification?.writes_signature_verification_store ??
                    false,
                )}
              </Chip>
              <Chip size="sm">
                writes_files:{" "}
                {String(remoteSignatureVerification?.writes_files ?? false)}
              </Chip>
              <Chip size="sm">
                remote_install_ready:{" "}
                {String(
                  remoteSignatureVerification?.remote_install_ready ?? false,
                )}
              </Chip>
              <Chip size="sm">
                installs_plugin:{" "}
                {String(remoteSignatureVerification?.installs_plugin ?? false)}
              </Chip>
              <Chip size="sm">
                installer_blocked_until_registration:{" "}
                {String(
                  remoteSignatureVerification?.installer_blocked_until_registration ??
                    status?.installer_blocked_until_registration ??
                    true,
                )}
              </Chip>
              <Chip size="sm">
                artifact: signature-verification-record.json
              </Chip>
              <Chip size="sm">artifact: signature-verification-store.json</Chip>
            </div>
            {remoteSignatureVerification && (
              <div className="mt-3">
                <JsonViewer title="Signature verification writeback result JSON" value={remoteSignatureVerification} rows={11} />
              </div>
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
                  className="mt-1 text-xs text-muted"
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
              aria-label="WASM remote install approval queue writeback JSON"
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
                  remoteQueueWriteback?.installer_blocked_until_installer_wiring ??
                    true,
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
              <div className="mt-3">
                <JsonViewer title="Approval queue writeback result JSON" value={remoteQueueWriteback} rows={11} />
              </div>
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
                  className="mt-1 text-xs text-muted"
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
              aria-label="WASM remote install approval decision plan JSON"
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
              <div className="mt-3">
                <JsonViewer title="Approval decision plan result JSON" value={remoteDecisionPlan} rows={11} />
              </div>
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
                  className="mt-1 text-xs text-muted"
                >
                  plan-only 契约：只把 approval-queue-entry.json 与
                  approval-decision-plan.json 串成 approval-writeback-plan.json
                  预览；该计划本身不写队列、不应用决策。真实队列写回请使用上方
                  pack-local queue store 路由，installer continuation
                  仍需后续显式接线。
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
              aria-label="WASM remote install approval writeback plan JSON"
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
              <div className="mt-3">
                <JsonViewer title="Approval writeback plan result JSON" value={remoteWritebackPlan} rows={11} />
              </div>
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
                  className="mt-1 text-xs text-muted"
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
              aria-label="WASM remote install approval gate plan JSON"
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
              <div className="mt-3">
                <JsonViewer title="Approval gate plan result JSON" value={remoteApprovalPlan} rows={11} />
              </div>
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
                  className="mt-1 text-xs text-muted"
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
              aria-label="WASM remote signed package install plan JSON"
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
              <div className="mt-3">
                <JsonViewer title="Remote install plan result JSON" value={remoteInstallPlan} rows={12} />
              </div>
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
                  className="mt-1 text-xs text-muted"
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
            <TextField aria-label="WASM plugin input JSON" value={inputJSON} onChange={setInputJSON}>
              <TextArea
                rows={4}
                aria-label="WASM plugin input JSON"
                className="font-mono text-xs"
              />
            </TextField>
            {result ? (
              <Card className="mt-3 bg-surface-secondary p-3">
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
                <JsonViewer title="WASM Plugin execution result JSON" value={result} rows={10} />
                <div
                  className="mt-2 text-xs text-muted"
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
              <div className="mt-3 rounded-xl border border-dashed border-border p-6 text-center text-sm text-muted">
                选择 loaded 插件后，可以先生成权限计划与沙箱 dry-run 执行计划。
              </div>
            )}
          </Card>
        </div>
      </div>
    </div>
  );
}
