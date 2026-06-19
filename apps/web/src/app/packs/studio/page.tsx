"use client";

import { useMemo, useState } from "react";
import Link from "next/link";
import { Button, Card, Chip, Input, Label, Spinner, TextArea, TextField } from "@heroui/react";
import {
  ArrowRight,
  Boxes,
  ClipboardCheck,
  Copy,
  ExternalLink,
  FileSearch,
  PackageCheck,
  RefreshCw,
  RotateCcw,
  ShieldCheck,
  Sparkles,
  Wrench,
} from "lucide-react";
import PageHeader from "@/components/page-header";
import { showToast } from "@/components/toast-provider";
import { createYunqueSDKClientOptions } from "@/lib/sdk-client";
import { useApiData } from "@/lib/use-api-data";
import {
  capabilitySurfaceLabels,
  groupPackPermissions,
  packFeatureFlags,
  packUsability,
  riskProfileForPack,
} from "@/lib/pack-presentation";
import { createPacksClient, type InstalledPack, type PackManifest, type PackStudioAuditReport, type PackStudioPatchReport, type PackStudioPlanReport, type PackStudioRepackReport, type PackStudioWorkspaceReport, type YqpackInspectReport } from "yunque-client/packs";

const packsClient = createPacksClient(createYunqueSDKClientOptions());

type PackCandidate = {
  manifest: PackManifest;
  source: "installed" | "catalog";
  enabled: boolean;
  installed: boolean;
};

type StudioAnalysis = {
  packId?: string;
  goal?: string;
  editable: string[];
  guarded: string[];
  tests: string[];
  warnings: string[];
  editableFiles: string[];
  diffPreview: string;
  auditSteps: string[];
  packageSteps: string[];
  rollbackSteps: string[];
  prompt?: string;
};

type StudioDraftCandidate = {
  key: string;
  label: string;
  filePath: string;
  content: string;
  summary: string;
  reason: string;
  riskLevel: "low" | "medium" | "high";
  gates: string[];
  applyable: boolean;
};

function packSlug(manifest: PackManifest): string {
  return manifest.id.replace(/^yunque\.pack\./, "");
}

function goPackDir(manifest: PackManifest): string {
  return packSlug(manifest).replace(/-/g, "");
}

function packPaths(manifest: PackManifest): string[] {
  return Array.from(new Set([
    ...(manifest.frontend?.menus || []).map((menu) => menu.path),
    ...(manifest.frontend?.routes || []).map((route) => route.path),
  ].filter(Boolean))).sort();
}

function packRoutes(manifest: PackManifest): string[] {
  const specs = manifest.backend?.routeSpecs || [];
  if (specs.length > 0) return specs.map((route) => `${route.method} ${route.path}`);
  return manifest.backend?.routes || [];
}

function packPrimaryPath(manifest: PackManifest): string | undefined {
  return packUsability(manifest).primaryActionPath || manifest.frontend?.menus?.[0]?.path || manifest.frontend?.routes?.[0]?.path;
}

function buildManifestDraftContent(manifest: PackManifest, goal: string): string {
  const draft = JSON.parse(JSON.stringify(manifest)) as PackManifest;
  const safeGoal = goal.trim() || "让这个能力包更像一个用户能直接理解和使用的功能，而不是只看到存在。";
  const primaryPath = packPrimaryPath(manifest) || "/chat";
  const metadata = { ...(draft.metadata || {}) };

  draft.description = safeGoal;
  metadata.descriptionStyle ||= "one-line-plus-three-examples";
  metadata.primaryActionLabel ||= `打开并验证 ${manifest.name}`;
  metadata.primaryActionPath ||= primaryPath;
  metadata.example1 ||= "从 Chat 说明目标，让云雀调用该能力并返回可查看结果。";
  metadata.example2 ||= "在能力界面查看执行状态、产物、限制与下一步操作。";
  metadata.example3 ||= "完成后把结果保存到记忆或知识，方便下次复用。";
  metadata.limitation ||= "改包前必须经过 diff 预览、内置审计和重新打包，不直接修改已安装版本。";
  metadata.studioGoal = safeGoal;
  draft.metadata = metadata;

  return `${JSON.stringify(draft, null, 2)}\n`;
}

function buildFrontendDraftContent(manifest: PackManifest, goal: string): string {
  const safeGoal = goal.trim() || "让这个能力包更像一个用户能直接理解和使用的功能，而不是只看到存在。";
  const primaryPath = packPrimaryPath(manifest) || "/chat";
  const capabilities = (manifest.backend?.capabilities || []).join(", ") || "无";
  const permissions = (manifest.backend?.permissions || []).join(", ") || "无";

  return [
    "<!doctype html>",
    "<html lang=\"zh-CN\">",
    "<head>",
    "  <meta charset=\"utf-8\" />",
    "  <meta name=\"viewport\" content=\"width=device-width, initial-scale=1\" />",
    `  <title>${manifest.name}</title>`,
    "  <style>",
    "    body { margin: 0; font-family: system-ui, sans-serif; background: #0f172a; color: #e5e7eb; }",
    "    main { max-width: 760px; margin: 0 auto; padding: 32px 20px; }",
    "    section { border: 1px solid rgba(148,163,184,.28); border-radius: 8px; padding: 16px; margin-top: 16px; background: rgba(15,23,42,.72); }",
    "    .muted { color: #94a3b8; }",
    "    .pill { display: inline-block; margin: 4px 6px 0 0; padding: 4px 8px; border-radius: 999px; background: rgba(59,130,246,.16); color: #bfdbfe; font-size: 12px; }",
    "  </style>",
    "</head>",
    "<body>",
    "  <main>",
    `    <p class=\"muted\">能力包界面草稿 · ${manifest.id}</p>`,
    `    <h1>${manifest.name}</h1>`,
    `    <p>${safeGoal}</p>`,
    "    <section>",
    "      <h2>用户可以在这里做什么</h2>",
    `      <p>从云雀对话或能力入口进入，查看目标、进度、结果和下一步操作。入口：${primaryPath}</p>`,
    "    </section>",
    "    <section>",
    "      <h2>能力与权限</h2>",
    `      <span class=\"pill\">能力：${capabilities}</span>`,
    `      <span class=\"pill\">权限：${permissions}</span>`,
    "      <p class=\"muted\">这只是 Pack Studio 草稿；接入真实 bridge/API 前必须先预览 diff、运行审计并重新打包。</p>",
    "    </section>",
    "  </main>",
    "</body>",
    "</html>",
    "",
  ].join("\n");
}

function buildStudioDraftCandidates(workspace: PackStudioWorkspaceReport, goal: string): StudioDraftCandidate[] {
  const seen = new Set<string>();
  const candidates: StudioDraftCandidate[] = [];
  for (const filePath of workspace.editable_files || []) {
    const normalized = filePath.replace(/\\/g, "/").toLowerCase();
    if (seen.has(normalized)) continue;
    seen.add(normalized);
    if (normalized.endsWith("/pack.json") || normalized.endsWith("pack.json")) {
      candidates.push({
        key: `manifest:${filePath}`,
        label: "manifest 草稿",
        filePath,
        content: buildManifestDraftContent(workspace.manifest, goal),
        summary: "补用途、入口、示例、限制和 Studio 目标。",
        reason: "manifest 是能力包契约入口，适合先补用户能理解的用途、入口、限制和回滚提示。",
        riskLevel: "low",
        gates: ["预览 diff", "内置审计", "Pack 可用性扫描"],
        applyable: true,
      });
      continue;
    }
    if (normalized.includes("/frontend/") && normalized.endsWith(".html")) {
      candidates.push({
        key: `frontend:${filePath}`,
        label: "前端界面草稿",
        filePath,
        content: buildFrontendDraftContent(workspace.manifest, goal),
        summary: "补一个可感知的沙箱界面草稿。",
        reason: "HTML 前端资源可在 yqpack 工作区内预览和替换，适合补独立界面、权限说明和结果区。",
        riskLevel: "medium",
        gates: ["预览 diff", "内置审计", "重新打包", "复检 yqpack"],
        applyable: true,
      });
    }
  }
  return candidates;
}

function sourceLabel(candidate: PackCandidate): string {
  if (candidate.installed && candidate.enabled) return "已启用";
  if (candidate.installed) return "已安装";
  return "源内可安装";
}

function buildEditableFiles(manifest: PackManifest): string[] {
  const slug = packSlug(manifest);
  const routes = packPaths(manifest);
  const files = [
    `packs/official/${slug}-pack/pack.json`,
  ];
  for (const route of routes) {
    const match = route.match(/^\/packs\/([^/?#]+)/);
    if (match?.[1]) files.push(`apps/web/src/app/packs/${match[1]}/page.tsx`);
  }
  if ((manifest.backend?.routeSpecs || []).length > 0 || (manifest.backend?.routes || []).length > 0) {
    files.push(`internal/packs/${goPackDir(manifest)}/`);
  }
  files.push(`apps/web/src/app/__tests__/${slug}-pack-page.test.tsx`);
  return Array.from(new Set(files));
}

function buildDiffPreview(manifest: PackManifest, goal: string): string {
  const slug = packSlug(manifest);
  const currentDescription = manifest.description || "未填写用途说明";
  const safeGoal = goal.trim() || "让这个能力包更有用、更可感知，并补齐用户入口、结果反馈和限制说明。";
  const primaryPath = packUsability(manifest).primaryActionPath || packPaths(manifest)[0] || "/chat";
  return [
    `diff --git a/packs/official/${slug}-pack/pack.json b/packs/official/${slug}-pack/pack.json`,
    "--- a/packs/official/" + slug + "-pack/pack.json",
    "+++ b/packs/official/" + slug + "-pack/pack.json",
    "@@ metadata @@",
    `- \"description\": \"${currentDescription}\"`,
    `+ \"description\": \"${safeGoal}\"`,
    `+ \"metadata.primaryActionLabel\": \"打开并验证 ${manifest.name} 的结果\"`,
    `+ \"metadata.primaryActionPath\": \"${primaryPath}\"`,
    "+ \"metadata.example1\": \"从 Chat 说明目标，让云雀调用该能力并返回可查看结果。\"",
    "+ \"metadata.example2\": \"在能力界面查看执行状态、产物、限制与下一步操作。\"",
    "+ \"metadata.limitation\": \"改包前必须经过 diff 预览、测试和重新打包，不直接修改已安装版本。\"",
    "",
    `diff --git a/apps/web/src/app/packs/${slug}/page.tsx b/apps/web/src/app/packs/${slug}/page.tsx`,
    "--- a/apps/web/src/app/packs/" + slug + "/page.tsx",
    "+++ b/apps/web/src/app/packs/" + slug + "/page.tsx",
    "@@ user-facing surface @@",
    "+ 增加结果区、权限说明、失败提示和回到 Chat/任务中心的入口。",
    "+ 对 WASM/iframe/后端能力保留沙箱、授权和 route 边界说明。",
  ].join("\n");
}

function buildStudioAnalysis(manifest: PackManifest, goal = ""): StudioAnalysis {
  const flags = packFeatureFlags(manifest);
  const risk = riskProfileForPack(manifest);
  const usability = packUsability(manifest);
  const routes = packRoutes(manifest);
  const paths = packPaths(manifest);
  const permissions = manifest.backend?.permissions || [];
  const capabilities = manifest.backend?.capabilities || [];

  const editable = [
    "用途说明、起手示例、入口文案、可用度分层和权限解释可以从 manifest/前端展示层优化。",
  ];
  if (paths.length > 0) editable.push("已有前端入口，可优先改页面文案、交互提示、空态、结果区和任务入口。");
  if (flags.isIframeBundle) editable.push("这是独立界面包；若 yqpack 内含 iframe 静态资源，可在沙箱边界内优化界面。");
  if (flags.hasWasm) editable.push("WASM 能力可以扩展 host 调用说明、输入输出 schema 和审计提示；改二进制逻辑需要源码。");
  if (capabilities.length > 0) editable.push("能力声明可用于生成更清楚的 Cogni/Planner 使用说明，但第一阶段不改决策算法。");

  const guarded = [
    "不直接修改已签名或已安装包；先生成 diff 方案，用户确认后再打包为新 yqpack。",
    "不扩大权限、不新增高风险 route，除非用户明确授权并更新权限说明。",
  ];
  if (routes.length > 0) guarded.push("后端路由逻辑属于运行时能力，改行为需要对应源码和 Go/Pack 测试。");
  if (flags.hasWasm) guarded.push("不要反编译后硬改 WASM；需要源码、ABI 说明和 wasm-plugin 回归测试。");
  if (flags.isIframeBundle) guarded.push("iframe 仍无宿主 token，只能调用 manifest 声明的 route。");
  if (risk.level === "high") guarded.push("高风险能力必须保留授权说明、审计线索和可回滚路径。");

  const tests = [
    "npm run typecheck",
    "npm test -- packs-page.test.tsx",
    "node scripts\\check-pack-usability.mjs --strict",
  ];
  if (routes.length > 0) tests.push("go test ./pkg/packruntime ./internal/controlplane/gateway ./internal/packs/... ./cmd/agent -count=1");
  if (flags.hasWasm) tests.push("go test ./internal/packs/wasmplugin ./internal/controlplane/gateway -run WASM -count=1");

  const warnings = [];
  if (usability.kind === "infrastructure") warnings.push("这个包主要是基础能力，改造目标应落到 Chat/任务/知识等实际使用面。");
  if (usability.kind === "experimental") warnings.push("这个包仍是实验能力，改造时不要把它包装成稳定承诺。");
  if (permissions.length === 0) warnings.push("manifest 未声明权限，若新增能力必须先补权限与风险说明。");

  const editableFiles = buildEditableFiles(manifest);
  const diffPreview = buildDiffPreview(manifest, goal);
  const auditSteps = [
    "只读展开 yqpack 或源码目录，确认 pack.json、frontend、backend、sdk 文件是否齐全。",
    "检查 diff 是否扩大权限、改变签名信任、绕过 routeSpecs 或隐藏高风险动作。",
    "按建议门禁运行测试，并记录失败原因和回滚建议。",
  ];
  const packageSteps = [
    `node scripts\\release-pack.mjs --pack packs\\official\\${packSlug(manifest)}-pack --dry-run`,
    `go run ./cmd/yunque-plugin pack packs\\official\\${packSlug(manifest)}-pack --out dist\\packs\\${packSlug(manifest)}-${manifest.version}.yqpack`,
    "重新计算 sha256/sizeBytes，刷新 catalog/release 元数据后再安装。",
  ];
  const rollbackSteps = [
    "保留原始 yqpack、原始 pack.json 和 installed registry 里的 previousVersion。",
    "新包作为 fork/local 版本安装；验证失败时禁用新版本并回滚上一版本。",
    "如果涉及高风险权限，回滚后重新跑 backend-route-audit 和 Pack 可用性审计。",
  ];

  return { packId: manifest.id, goal, editable, guarded, tests, warnings, editableFiles, diffPreview, auditSteps, packageSteps, rollbackSteps };
}

function buildXiaoyuPrompt(manifest: PackManifest, goal: string): string {
  const routes = packRoutes(manifest);
  const paths = packPaths(manifest);
  const flags = packFeatureFlags(manifest);
  const permissions = manifest.backend?.permissions || [];
  const capabilities = manifest.backend?.capabilities || [];
  const analysis = buildStudioAnalysis(manifest, goal);
  return [
    `请以“小羽改包”的方式改造能力包 ${manifest.name} (${manifest.id}) v${manifest.version}。`,
    "",
    `用户目标：${goal.trim() || "让这个能力包变得更有用、更可感知，并补齐用户能理解的入口、说明和结果反馈。"}`,
    "",
    "当前包信息：",
    `- 状态：${manifest.status || "unknown"}`,
    `- 前端入口：${paths.length > 0 ? paths.join(", ") : "无"}`,
    `- 后端路由：${routes.length > 0 ? routes.join(", ") : "无"}`,
    `- 能力声明：${capabilities.length > 0 ? capabilities.join(", ") : "无"}`,
    `- 权限声明：${permissions.length > 0 ? permissions.join(", ") : "无"}`,
    `- 形态：${flags.isIframeBundle ? "iframe-bundle " : ""}${flags.hasWasm ? "WASM " : ""}${flags.hasBackend ? "backend " : ""}${flags.hasFrontend ? "frontend" : ""}`.trim(),
    "",
    "请按这个安全流程执行：",
    "1. 先只读检查 yqpack/pack.json/前端入口/SDK/后端 routeSpecs，列出真实可改文件。",
    "2. 明确哪些能直接改，哪些需要源码，哪些属于已编译 WASM/native Go 不能硬改。",
    "3. 先给 diff 预览和风险说明，不要直接扩大权限或绕过签名。",
    "4. 用户确认后再修改、跑测试、重新打包为新的 yqpack，并保留旧版本回滚。",
    "",
    "本包建议优先改：",
    ...analysis.editable.map((item) => `- ${item}`),
    "",
    "可改文件候选：",
    ...analysis.editableFiles.map((item) => `- ${item}`),
    "",
    "diff 预览草案：",
    "```diff",
    analysis.diffPreview,
    "```",
    "",
    "必须遵守：",
    ...analysis.guarded.map((item) => `- ${item}`),
    "",
    "审计步骤：",
    ...analysis.auditSteps.map((item) => `- ${item}`),
    "",
    "重新打包与回滚：",
    ...analysis.packageSteps.map((item) => `- ${item}`),
    ...analysis.rollbackSteps.map((item) => `- ${item}`),
    "",
    "建议门禁：",
    ...analysis.tests.map((item) => `- ${item}`),
  ].join("\n");
}

function mapStudioPlanReport(report: PackStudioPlanReport): StudioAnalysis {
  return {
    packId: report.pack_id,
    goal: report.goal,
    editable: report.editable || [],
    guarded: report.guarded || [],
    tests: [],
    warnings: report.warnings || [],
    editableFiles: report.editable_files || [],
    diffPreview: report.diff_preview || "",
    auditSteps: report.audit_steps || [],
    packageSteps: report.package_steps || [],
    rollbackSteps: report.rollback_steps || [],
    prompt: report.xiaoyu_prompt,
  };
}

export default function PackStudioPage() {
  const { data, loading, refresh } = useApiData(async () => {
    const [installed, catalog] = await Promise.all([packsClient.installed(), packsClient.catalog()]);
    const map = new Map<string, PackCandidate>();
    for (const pack of installed.packs || []) {
      map.set(pack.manifest.id, {
        manifest: pack.manifest,
        source: "installed",
        enabled: pack.status === "enabled",
        installed: true,
      });
    }
    for (const entry of catalog.entries || []) {
      if (!map.has(entry.manifest.id)) {
        map.set(entry.manifest.id, {
          manifest: entry.manifest,
          source: "catalog",
          enabled: Boolean(entry.enabled),
          installed: Boolean(entry.installed),
        });
      }
    }
    return { packs: [...map.values()].sort((a, b) => a.manifest.name.localeCompare(b.manifest.name)) };
  }, { packs: [] as PackCandidate[] });
  const [selectedId, setSelectedId] = useState("");
  const [goal, setGoal] = useState("让这个能力包更像一个用户能直接理解和使用的功能，而不是只看到存在。");
  const [packagePath, setPackagePath] = useState("");
  const [packageUrl, setPackageUrl] = useState("");
  const [packageSHA, setPackageSHA] = useState("");
  const [inspecting, setInspecting] = useState(false);
  const [inspectReport, setInspectReport] = useState<YqpackInspectReport | null>(null);
  const [preparingWorkspace, setPreparingWorkspace] = useState(false);
  const [workspaceReport, setWorkspaceReport] = useState<PackStudioWorkspaceReport | null>(null);
  const [patchFile, setPatchFile] = useState("");
  const [patchContent, setPatchContent] = useState("");
  const [patching, setPatching] = useState(false);
  const [patchReport, setPatchReport] = useState<PackStudioPatchReport | null>(null);
  const [auditing, setAuditing] = useState(false);
  const [auditReport, setAuditReport] = useState<PackStudioAuditReport | null>(null);
  const [repacking, setRepacking] = useState(false);
  const [repackReport, setRepackReport] = useState<PackStudioRepackReport | null>(null);
  const [reinspectReport, setReinspectReport] = useState<YqpackInspectReport | null>(null);
  const [reinspecting, setReinspecting] = useState(false);
  const [installingRepack, setInstallingRepack] = useState(false);
  const [installedRepack, setInstalledRepack] = useState<InstalledPack | null>(null);
  const [postInstallBusy, setPostInstallBusy] = useState<string | null>(null);

  const candidates = data?.packs || [];
  const selected = useMemo(
    () => candidates.find((item) => item.manifest.id === selectedId) || candidates[0],
    [candidates, selectedId],
  );
  const manifest = selected?.manifest;
  const { data: studioPlan, refresh: refreshStudioPlan } = useApiData(async () => {
    if (!manifest) return null;
    try {
      const report = await packsClient.studioPlan({ packId: manifest.id, goal });
      return mapStudioPlanReport(report);
    } catch {
      return buildStudioAnalysis(manifest, goal);
    }
  }, null as StudioAnalysis | null, [manifest?.id, goal]);
  const fallbackAnalysis = manifest ? buildStudioAnalysis(manifest, goal) : null;
  const analysis = studioPlan && studioPlan.packId === manifest?.id && studioPlan.goal === goal ? studioPlan : fallbackAnalysis;
  const prompt = manifest ? (analysis?.prompt || buildXiaoyuPrompt(manifest, goal)) : "";
  const chatHref = `/chat?q=${encodeURIComponent(prompt)}`;
  const draftCandidates = useMemo(
    () => workspaceReport ? buildStudioDraftCandidates(workspaceReport, goal) : [],
    [workspaceReport, goal],
  );

  const copyPrompt = async () => {
    if (!prompt) return;
    await navigator.clipboard?.writeText(prompt);
    showToast("已复制小羽改包任务", "success");
  };

  const inspectYqpack = async () => {
    const path = packagePath.trim();
    const url = packageUrl.trim();
    if (!path && !url) {
      showToast("请填写本地 yqpack 路径或 OSS/Release URL", "warning");
      return;
    }
    setInspecting(true);
    try {
      const report = await packsClient.studioInspect({
        packagePath: path || undefined,
        packageUrl: url || undefined,
        sha256: packageSHA.trim() || undefined,
        goal,
      });
      setInspectReport(report);
      setWorkspaceReport(null);
      showToast("已完成 yqpack 只读检查", "success");
    } catch (error) {
      showToast(error instanceof Error ? error.message : "yqpack 检查失败", "error");
    } finally {
      setInspecting(false);
    }
  };

  const prepareWorkspace = async () => {
    const path = packagePath.trim();
    const url = packageUrl.trim();
    if (!path && !url) {
      showToast("请先填写本地 yqpack 路径或 OSS/Release URL", "warning");
      return;
    }
    setPreparingWorkspace(true);
    try {
      const report = await packsClient.studioWorkspace({
        packagePath: path || undefined,
        packageUrl: url || undefined,
        sha256: packageSHA.trim() || inspectReport?.sha256,
        goal,
      });
      setWorkspaceReport(report);
      setPatchFile(report.editable_files[0] || "");
      setPatchContent("");
      setPatchReport(null);
      setAuditReport(null);
      setRepackReport(null);
      setReinspectReport(null);
      setInstalledRepack(null);
      showToast("已准备 Pack Studio 工作区", "success");
    } catch (error) {
      showToast(error instanceof Error ? error.message : "准备工作区失败", "error");
    } finally {
      setPreparingWorkspace(false);
    }
  };

  const submitPatch = async (apply: boolean) => {
    if (!workspaceReport) return;
    if (!patchFile.trim()) {
      showToast("请选择要修改的工作区文件", "warning");
      return;
    }
    setPatching(true);
    try {
      const report = await packsClient.studioPatch({
        workspacePath: workspaceReport.workspace_path,
        filePath: patchFile,
        content: patchContent,
        reason: goal,
        apply,
      });
      setPatchReport(report);
      if (apply) {
        setAuditReport(null);
        setRepackReport(null);
        setReinspectReport(null);
        setInstalledRepack(null);
      }
      showToast(apply ? "已应用到工作区" : "已生成 diff 预览", "success");
    } catch (error) {
      showToast(error instanceof Error ? error.message : "工作区改动失败", "error");
    } finally {
      setPatching(false);
    }
  };

  const fillDraftCandidate = (candidate: StudioDraftCandidate) => {
    setPatchFile(candidate.filePath);
    setPatchContent(candidate.content);
    setPatchReport(null);
    showToast(`已生成 ${candidate.label}，请先预览 diff 再应用`, "success");
  };

  const auditWorkspace = async () => {
    if (!workspaceReport) return;
    setAuditing(true);
    try {
      const report = await packsClient.studioAudit({
        workspacePath: workspaceReport.workspace_path,
        goal,
      });
      setAuditReport(report);
      showToast(report.allowed ? "内置审计通过" : "内置审计发现高风险改动", report.allowed ? "success" : "warning");
    } catch (error) {
      showToast(error instanceof Error ? error.message : "工作区审计失败", "error");
    } finally {
      setAuditing(false);
    }
  };

  const repackWorkspace = async () => {
    if (!workspaceReport) return;
    setRepacking(true);
    try {
      const report = await packsClient.studioRepack({
        workspacePath: workspaceReport.workspace_path,
        goal,
      });
      setRepackReport(report);
      setReinspectReport(null);
      setInstalledRepack(null);
      showToast("已生成新的 yqpack", "success");
    } catch (error) {
      showToast(error instanceof Error ? error.message : "重新打包失败", "error");
    } finally {
      setRepacking(false);
    }
  };

  const reinspectRepack = async () => {
    if (!repackReport) return;
    setReinspecting(true);
    try {
      const report = await packsClient.studioInspect({
        packagePath: repackReport.package_path,
        sha256: repackReport.sha256,
        goal,
      });
      setReinspectReport(report);
      setInstalledRepack(null);
      showToast("新 yqpack 复检通过", "success");
    } catch (error) {
      showToast(error instanceof Error ? error.message : "新包复检失败", "error");
    } finally {
      setReinspecting(false);
    }
  };

  const installRepack = async () => {
    if (!repackReport || !reinspectReport?.sha256_match) return;
    setInstallingRepack(true);
    try {
      const response = await packsClient.install({
        packagePath: repackReport.package_path,
        sha256: repackReport.sha256,
        source: `pack-studio:${repackReport.package_path}`,
      });
      setInstalledRepack(response.pack);
      showToast("已安装新的能力包，启用前请再次确认权限", "success");
      refresh();
      refreshStudioPlan();
    } catch (error) {
      showToast(error instanceof Error ? error.message : "安装新包失败", "error");
    } finally {
      setInstallingRepack(false);
    }
  };

  const mutateInstalledRepack = async (action: "enable" | "disable" | "rollback") => {
    const id = installedRepack?.manifest.id;
    if (!id) return;
    setPostInstallBusy(action);
    try {
      const response = action === "enable"
        ? await packsClient.enable(id)
        : action === "disable"
          ? await packsClient.disable(id)
          : await packsClient.rollback(id);
      setInstalledRepack(response.pack);
      showToast(action === "enable" ? "已启用新能力包" : action === "disable" ? "已禁用新能力包" : "已回滚能力包", "success");
      refresh();
      refreshStudioPlan();
    } catch (error) {
      showToast(error instanceof Error ? error.message : "能力包状态更新失败", "error");
    } finally {
      setPostInstallBusy(null);
    }
  };

  if (loading) {
    return <div className="flex items-center justify-center h-[60vh]"><Spinner size="lg" /></div>;
  }

  return (
    <div className="flex flex-col h-screen overflow-hidden">
      <div className="flex-shrink-0 p-5 border-b" style={{ borderColor: "var(--yunque-border)" }}>
        <PageHeader
          icon={<Wrench size={20} />}
          title="Pack Studio"
          description="让小羽先读懂能力包，再生成可审计、可测试、可回滚的改包方案"
          onRefresh={refresh}
        />
        <div className="mt-4 grid gap-3 md:grid-cols-3">
          <Card className="section-card p-4">
            <div className="kpi-label">可分析</div>
            <div className="kpi-value">{candidates.length}</div>
          </Card>
          <Card className="section-card p-4">
            <div className="kpi-label">执行模式</div>
            <div className="kpi-value text-base">只读规划</div>
          </Card>
          <Card className="section-card p-4">
            <div className="kpi-label">交付物</div>
            <div className="kpi-value text-base">diff 方案</div>
          </Card>
        </div>
      </div>

      <div className="flex-1 overflow-y-auto p-5 space-y-4">
        <Card className="section-card p-4">
          <div className="flex items-center gap-2 mb-3">
            <FileSearch size={16} style={{ color: "var(--yunque-accent)" }} />
            <div className="text-sm font-semibold" style={{ color: "var(--yunque-text)" }}>选择要分析的能力包</div>
          </div>
          {candidates.length === 0 ? (
            <div className="text-sm" style={{ color: "var(--yunque-text-muted)" }}>还没有可分析的能力包。</div>
          ) : (
            <div className="grid gap-2 md:grid-cols-2 xl:grid-cols-3">
              {candidates.map((candidate) => {
                const usability = packUsability(candidate.manifest);
                const active = candidate.manifest.id === (manifest?.id || "");
                return (
                  <button
                    key={candidate.manifest.id}
                    type="button"
                    onClick={() => setSelectedId(candidate.manifest.id)}
                    className="rounded-md border p-3 text-left transition-colors"
                    style={{
                      borderColor: active ? "var(--yunque-accent)" : "var(--yunque-border)",
                      background: active ? "rgba(59,130,246,0.08)" : "var(--yunque-surface)",
                    }}
                  >
                    <div className="flex items-center justify-between gap-2">
                      <div className="truncate text-sm font-medium" style={{ color: "var(--yunque-text)" }}>{candidate.manifest.name}</div>
                      <Chip size="sm" variant="soft">{sourceLabel(candidate)}</Chip>
                    </div>
                    <div className="mt-1 truncate text-xs" style={{ color: "var(--yunque-text-muted)" }}>{candidate.manifest.id}</div>
                    <div className="mt-2 flex flex-wrap gap-1">
                      <Chip size="sm" style={{ background: "rgba(59,130,246,0.08)", color: "var(--yunque-primary)" }}>{usability.label}</Chip>
                      <Chip size="sm" variant="soft">v{candidate.manifest.version}</Chip>
                    </div>
                  </button>
                );
              })}
            </div>
          )}
        </Card>

        {manifest && analysis && (
          <>
            <Card className="section-card p-4">
              <div className="flex flex-wrap items-start justify-between gap-3">
                <div>
                  <div className="flex items-center gap-2 text-sm font-semibold" style={{ color: "var(--yunque-text)" }}>
                    <Boxes size={16} style={{ color: "var(--yunque-accent)" }} />
                    {manifest.name}
                  </div>
                  <div className="mt-1 text-xs font-mono" style={{ color: "var(--yunque-text-muted)" }}>{manifest.id}</div>
                </div>
                <div className="flex flex-wrap gap-1.5">
                  {capabilitySurfaceLabels(manifest).map((label) => (
                    <Chip key={label} size="sm" variant="soft">{label}</Chip>
                  ))}
                </div>
              </div>
              <div className="mt-3 grid gap-3 md:grid-cols-2">
                <div className="rounded-md border p-3" style={{ borderColor: "var(--yunque-border)" }}>
                  <div className="text-xs font-medium" style={{ color: "var(--yunque-text)" }}>可以让小羽优先优化</div>
                  <div className="mt-2 space-y-2">
                    {analysis.editable.map((item) => (
                      <div key={item} className="flex items-start gap-2 text-xs" style={{ color: "var(--yunque-text-secondary)" }}>
                        <span style={{ color: "var(--yunque-accent)" }}>•</span><span>{item}</span>
                      </div>
                    ))}
                  </div>
                </div>
                <div className="rounded-md border p-3" style={{ borderColor: "var(--yunque-border)" }}>
                  <div className="text-xs font-medium" style={{ color: "var(--yunque-text)" }}>需要守住的边界</div>
                  <div className="mt-2 space-y-2">
                    {analysis.guarded.map((item) => (
                      <div key={item} className="flex items-start gap-2 text-xs" style={{ color: "var(--yunque-text-secondary)" }}>
                        <ShieldCheck size={12} className="mt-0.5" style={{ color: "var(--yunque-success)" }} /><span>{item}</span>
                      </div>
                    ))}
                  </div>
                </div>
              </div>
              {analysis.warnings.length > 0 && (
                <div className="mt-3 rounded-md p-3 text-xs" style={{ background: "rgba(245,158,11,0.10)", color: "var(--yunque-warning)" }}>
                  {analysis.warnings.join(" ")}
                </div>
              )}
            </Card>

            <Card className="section-card p-4">
              <div className="flex items-center gap-2 mb-3">
                <Sparkles size={16} style={{ color: "var(--yunque-primary)" }} />
                <div className="text-sm font-semibold" style={{ color: "var(--yunque-text)" }}>交给小羽的改包目标</div>
              </div>
              <TextField value={goal} onChange={setGoal}>
                <Label>这次想补强什么</Label>
                <Input placeholder="例如：增加结果查看入口、让能力更像一个可用工具、补齐权限说明" />
              </TextField>
              <div className="mt-3">
                <TextArea aria-label="小羽改包任务" value={prompt} readOnly rows={14} className="font-mono text-xs" />
              </div>
              <div className="mt-3 flex flex-wrap gap-2">
                <Button variant="outline" onPress={copyPrompt}>
                  <Copy size={14} /> 复制任务
                </Button>
                <Link href={chatHref}>
                  <Button className="btn-accent">
                    交给 Chat 里的小羽 <ArrowRight size={14} />
                  </Button>
                </Link>
                <Button variant="ghost" onPress={refresh}>
                  <RefreshCw size={14} /> 重新读取
                </Button>
                <Button variant="ghost" onPress={refreshStudioPlan}>
                  <FileSearch size={14} /> 刷新计划
                </Button>
              </div>
            </Card>

            <Card className="section-card p-4">
              <div className="flex items-center gap-2 mb-3">
                <PackageCheck size={16} style={{ color: "var(--yunque-primary)" }} />
                <div className="text-sm font-semibold" style={{ color: "var(--yunque-text)" }}>检查 yqpack 包内容</div>
              </div>
              <div className="grid gap-3 lg:grid-cols-[minmax(0,1fr)_minmax(0,1fr)_220px]">
                <TextField value={packagePath} onChange={setPackagePath}>
                  <Label>本地 yqpack 路径</Label>
                  <Input placeholder="C:\\packs\\demo-0.1.0.yqpack" />
                </TextField>
                <TextField value={packageUrl} onChange={setPackageUrl}>
                  <Label>OSS / Release URL</Label>
                  <Input placeholder="https://oss.example.com/packs/demo.yqpack" />
                </TextField>
                <TextField value={packageSHA} onChange={setPackageSHA}>
                  <Label>SHA256</Label>
                  <Input placeholder="可选" />
                </TextField>
              </div>
              <div className="mt-3 flex flex-wrap gap-2">
                <Button variant="outline" onPress={inspectYqpack} isDisabled={inspecting}>
                  {inspecting ? <Spinner size="sm" /> : <FileSearch size={14} />} 只读检查
                </Button>
                <Button variant="outline" onPress={prepareWorkspace} isDisabled={preparingWorkspace || !inspectReport?.sha256_match}>
                  {preparingWorkspace ? <Spinner size="sm" /> : <Wrench size={14} />} 准备工作区
                </Button>
              </div>
              {inspectReport && (
                <div className="mt-4 grid gap-3 lg:grid-cols-[260px_minmax(0,1fr)]">
                  <div className="rounded-md border p-3" style={{ borderColor: "var(--yunque-border)" }}>
                    <div className="text-xs font-medium" style={{ color: "var(--yunque-text)" }}>{inspectReport.manifest.name}</div>
                    <div className="mt-1 font-mono text-[11px]" style={{ color: "var(--yunque-text-muted)" }}>{inspectReport.manifest.id}</div>
                    <div className="mt-3 flex flex-wrap gap-1">
                      <Chip size="sm" color={inspectReport.sha256_match ? "success" : "warning"}>{inspectReport.sha256_match ? "SHA 匹配" : "SHA 不匹配"}</Chip>
                      <Chip size="sm" variant="soft">{inspectReport.entry_count} 个文件</Chip>
                      <Chip size="sm" variant="soft">{inspectReport.editable_count} 可改</Chip>
                      <Chip size="sm" variant="soft">{inspectReport.guarded_count} 需源码/审计</Chip>
                    </div>
                    <div className="mt-3 break-all font-mono text-[11px]" style={{ color: "var(--yunque-text-muted)" }}>{inspectReport.sha256}</div>
                    {(inspectReport.warnings || []).length > 0 && (
                      <div className="mt-3 rounded p-2 text-xs" style={{ background: "rgba(245,158,11,0.10)", color: "var(--yunque-warning)" }}>
                        {inspectReport.warnings?.join(" ")}
                      </div>
                    )}
                  </div>
                  <div className="rounded-md border p-3" style={{ borderColor: "var(--yunque-border)" }}>
                    <div className="mb-2 text-xs font-medium" style={{ color: "var(--yunque-text)" }}>包内文件分类</div>
                    <div className="max-h-72 overflow-y-auto space-y-1">
                      {inspectReport.entries.slice(0, 24).map((entry) => (
                        <div key={entry.path} className="grid gap-2 rounded px-2 py-1 text-xs md:grid-cols-[90px_minmax(0,1fr)_120px]" style={{ background: "var(--yunque-bg-hover)", color: "var(--yunque-text-secondary)" }}>
                          <span>{entry.kind}</span>
                          <span className="truncate font-mono">{entry.path}</span>
                          <span style={{ color: entry.editable ? "var(--yunque-success)" : "var(--yunque-warning)" }}>
                            {entry.editable ? "可改" : entry.needs_source ? "需源码" : "需审计"}
                          </span>
                        </div>
                      ))}
                    </div>
                    <div className="mt-3 text-xs" style={{ color: "var(--yunque-text-muted)" }}>
                      只读检查不会安装能力包；它只告诉小羽真实包内有哪些文件、哪些能改、哪些必须保留边界。
                    </div>
                  </div>
                </div>
              )}
              {workspaceReport && (
                <div className="mt-4 rounded-md border p-3" style={{ borderColor: "var(--yunque-border)" }}>
                  <div className="flex flex-wrap items-start justify-between gap-3">
                    <div>
                      <div className="text-xs font-medium" style={{ color: "var(--yunque-text)" }}>Pack Studio 工作区</div>
                      <div className="mt-1 break-all font-mono text-[11px]" style={{ color: "var(--yunque-text-muted)" }}>{workspaceReport.workspace_path}</div>
                    </div>
                    <Chip size="sm" variant="soft">{workspaceReport.workspace_id}</Chip>
                  </div>
                  <div className="mt-3 grid gap-3 lg:grid-cols-3">
                    <div>
                      <div className="mb-2 text-xs font-medium" style={{ color: "var(--yunque-text)" }}>下一步</div>
                      <div className="space-y-1">
                        {workspaceReport.next_steps.map((step) => (
                          <div key={step} className="rounded px-2 py-1 text-xs" style={{ background: "var(--yunque-bg-hover)", color: "var(--yunque-text-secondary)" }}>{step}</div>
                        ))}
                      </div>
                    </div>
                    <div>
                      <div className="mb-2 text-xs font-medium" style={{ color: "var(--yunque-text)" }}>重打包命令</div>
                      <div className="space-y-1">
                        {workspaceReport.repack_commands.map((command) => (
                          <div key={command} className="rounded px-2 py-1 font-mono text-[11px]" style={{ background: "var(--yunque-bg-hover)", color: "var(--yunque-text-secondary)" }}>{command}</div>
                        ))}
                      </div>
                    </div>
                    <div>
                      <div className="mb-2 text-xs font-medium" style={{ color: "var(--yunque-text)" }}>回滚命令</div>
                      <div className="space-y-1">
                        {workspaceReport.rollback_commands.map((command) => (
                          <div key={command} className="rounded px-2 py-1 text-xs" style={{ background: "var(--yunque-bg-hover)", color: "var(--yunque-text-secondary)" }}>{command}</div>
                        ))}
                      </div>
                    </div>
                  </div>
                  <div className="mt-3 text-xs" style={{ color: "var(--yunque-text-muted)" }}>
                    工作区是可编辑副本，不会启用能力包；安装新 yqpack 前仍需重新检查、测试和确认回滚路径。
                  </div>
                  <div className="mt-4 rounded-md border p-3" style={{ borderColor: "var(--yunque-border)" }}>
                    <div className="mb-3 text-xs font-medium" style={{ color: "var(--yunque-text)" }}>工作区改动预览</div>
                    <div className="grid gap-3 lg:grid-cols-[minmax(0,1fr)_minmax(0,1.4fr)]">
                      <TextField value={patchFile} onChange={setPatchFile}>
                        <Label>要修改的文件</Label>
                        <Input placeholder={workspaceReport.editable_files[0] || "选择 editable_files 中的文件"} />
                      </TextField>
                      <TextArea
                        aria-label="新的文件内容"
                        value={patchContent}
                        onChange={(event) => setPatchContent(event.target.value)}
                        rows={5}
                      >
                        <Label>新的文件内容</Label>
                      </TextArea>
                    </div>
                    <div className="mt-2 text-xs" style={{ color: "var(--yunque-text-muted)" }}>
                      草稿只会填入工作区改动框；真正写入仍需先预览 diff，并在应用后运行内置审计。
                    </div>
                    {draftCandidates.length > 0 && (
                      <div className="mt-3 rounded-md p-2" style={{ background: "var(--yunque-bg-hover)" }}>
                        <div className="mb-2 text-xs font-medium" style={{ color: "var(--yunque-text)" }}>小羽改造草稿队列</div>
                        <div className="grid gap-2 lg:grid-cols-2">
                          {draftCandidates.map((candidate) => (
                            <div key={candidate.key} className="rounded-md border p-2" style={{ borderColor: "var(--yunque-border)" }}>
                              <div className="flex flex-wrap items-center justify-between gap-2">
                                <div className="flex flex-wrap items-center gap-2">
                                  <span className="text-xs font-medium" style={{ color: "var(--yunque-text)" }}>{candidate.label}</span>
                                  <Chip size="sm" color={candidate.riskLevel === "high" ? "danger" : candidate.riskLevel === "medium" ? "warning" : "success"}>
                                    风险：{candidate.riskLevel}
                                  </Chip>
                                  <Chip size="sm" variant="soft">{candidate.applyable ? "可预览应用" : "只读说明"}</Chip>
                                </div>
                                <Button size="sm" variant="outline" onPress={() => fillDraftCandidate(candidate)} isDisabled={!candidate.applyable}>
                                  <Sparkles size={13} /> 载入草稿
                                </Button>
                              </div>
                              <div className="mt-1 truncate font-mono text-[11px]" style={{ color: "var(--yunque-text-muted)" }}>{candidate.filePath}</div>
                              <div className="mt-1 text-[11px]" style={{ color: "var(--yunque-text-muted)" }}>{candidate.summary}</div>
                              <div className="mt-1 text-[11px]" style={{ color: "var(--yunque-text-muted)" }}>原因：{candidate.reason}</div>
                              <div className="mt-2 flex flex-wrap gap-1">
                                {candidate.gates.map((gate) => (
                                  <Chip key={`${candidate.key}:${gate}`} size="sm" variant="soft">{gate}</Chip>
                                ))}
                              </div>
                            </div>
                          ))}
                        </div>
                      </div>
                    )}
                    <div className="mt-3 flex flex-wrap gap-2">
                      <Button variant="outline" onPress={() => submitPatch(false)} isDisabled={patching}>
                        {patching ? <Spinner size="sm" /> : <FileSearch size={14} />} 预览 diff
                      </Button>
                      <Button variant="outline" onPress={() => submitPatch(true)} isDisabled={patching}>
                        {patching ? <Spinner size="sm" /> : <Wrench size={14} />} 应用到工作区
                      </Button>
                      <Button variant="outline" onPress={auditWorkspace} isDisabled={auditing}>
                        {auditing ? <Spinner size="sm" /> : <ShieldCheck size={14} />} 运行内置审计
                      </Button>
                      <Button variant="outline" onPress={repackWorkspace} isDisabled={repacking || auditReport?.allowed === false}>
                        {repacking ? <Spinner size="sm" /> : <PackageCheck size={14} />} 重新打包
                      </Button>
                    </div>
                    {patchReport && (
                      <div className="mt-3 grid gap-3 lg:grid-cols-[minmax(0,1fr)_220px]">
                        <TextArea aria-label="工作区 diff 预览" value={patchReport.diff_preview} readOnly rows={10} className="font-mono text-xs" />
                        <div className="space-y-2 text-xs" style={{ color: "var(--yunque-text-secondary)" }}>
                          <Chip size="sm" color={patchReport.applied ? "success" : "warning"}>{patchReport.applied ? "已应用" : "仅预览"}</Chip>
                          <div className="break-all font-mono">{patchReport.relative_path}</div>
                          <div>旧 SHA：{patchReport.old_sha256 || "-"}</div>
                          <div>新 SHA：{patchReport.new_sha256}</div>
                          {(patchReport.warnings || []).map((warning) => (
                            <div key={warning} style={{ color: "var(--yunque-warning)" }}>{warning}</div>
                          ))}
                        </div>
                      </div>
                    )}
                    {auditReport && (
                      <div className="mt-3 rounded-md border p-3 text-xs" style={{ borderColor: "var(--yunque-border)", color: "var(--yunque-text-secondary)" }}>
                        <div className="mb-2 flex flex-wrap items-center gap-2">
                          <Chip size="sm" color={auditReport.allowed ? "success" : "danger"}>
                            {auditReport.allowed ? "审计通过" : "审计阻断"}
                          </Chip>
                          <Chip size="sm" variant="soft">风险：{auditReport.risk_level}</Chip>
                          <span>{auditReport.change_count} 个改动 · {auditReport.editable_change_count} 可改 · {auditReport.guarded_change_count} 需源码/专项审计</span>
                        </div>
                        <div className="break-all font-mono">当前 SHA：{auditReport.current_sha256}</div>
                        {auditReport.changes.length > 0 && (
                          <div className="mt-2 grid gap-1">
                            {auditReport.changes.slice(0, 6).map((change) => (
                              <div key={`${change.status}:${change.path}`} className="rounded px-2 py-1" style={{ background: "var(--yunque-bg-hover)" }}>
                                <span className="font-mono">{change.status}</span> · <span className="font-mono">{change.path}</span> · {change.kind}
                              </div>
                            ))}
                          </div>
                        )}
                        {(auditReport.warnings || []).map((warning) => (
                          <div key={warning} className="mt-1" style={{ color: "var(--yunque-danger)" }}>{warning}</div>
                        ))}
                      </div>
                    )}
                    {repackReport && (
                      <div className="mt-3 rounded-md border p-3 text-xs" style={{ borderColor: "var(--yunque-border)", color: "var(--yunque-text-secondary)" }}>
                        <div className="mb-2 flex flex-wrap items-center gap-2">
                          <Chip size="sm" color="success">新 yqpack 已生成</Chip>
                          <span>{repackReport.size_bytes.toLocaleString()} bytes</span>
                        </div>
                        <div className="break-all font-mono">{repackReport.package_path}</div>
                        <div className="mt-1 break-all font-mono">SHA256：{repackReport.sha256}</div>
                        <div className="mt-3 flex flex-wrap gap-2">
                          <Button variant="outline" onPress={reinspectRepack} isDisabled={reinspecting}>
                            {reinspecting ? <Spinner size="sm" /> : <FileSearch size={14} />} 复检新包
                          </Button>
                          <Button
                            variant="outline"
                            onPress={installRepack}
                            isDisabled={installingRepack || !reinspectReport?.sha256_match}
                          >
                            {installingRepack ? <Spinner size="sm" /> : <PackageCheck size={14} />} 安装新包
                          </Button>
                        </div>
                        {reinspectReport && (
                          <div className="mt-2 flex flex-wrap items-center gap-2">
                            <Chip size="sm" color={reinspectReport.sha256_match ? "success" : "danger"}>
                              {reinspectReport.sha256_match ? "复检 SHA 匹配" : "复检 SHA 不匹配"}
                            </Chip>
                            <span>{reinspectReport.entry_count} 个文件 · {reinspectReport.editable_count} 可改 · {reinspectReport.guarded_count} 需审计</span>
                          </div>
                        )}
                        {installedRepack && (
                          <div className="mt-3 rounded-md border p-3" style={{ borderColor: "var(--yunque-border)" }}>
                            <div className="mb-2 flex flex-wrap items-center gap-2">
                              <Chip size="sm" color={installedRepack.status === "enabled" ? "success" : "warning"}>
                                {installedRepack.status === "enabled" ? "已启用" : "已安装未启用"}
                              </Chip>
                              <span className="font-medium" style={{ color: "var(--yunque-text)" }}>{installedRepack.manifest.name}</span>
                              <span className="font-mono">{installedRepack.manifest.version}</span>
                            </div>
                            <div className="text-xs" style={{ color: "var(--yunque-text-muted)" }}>
                              启用前再次确认权限；出问题可在这里禁用或回滚，也可以回到能力包中心继续管理。
                            </div>
                            <div className="mt-3 flex flex-wrap gap-2">
                              {packPrimaryPath(installedRepack.manifest) && (
                                <Link href={packPrimaryPath(installedRepack.manifest)!}>
                                  <Button size="sm" variant="outline">
                                    <ExternalLink size={14} /> 打开入口
                                  </Button>
                                </Link>
                              )}
                              <Button size="sm" variant="outline" onPress={() => mutateInstalledRepack("enable")} isDisabled={postInstallBusy === "enable" || installedRepack.status === "enabled"}>
                                {postInstallBusy === "enable" ? <Spinner size="sm" /> : <Sparkles size={14} />} 启用
                              </Button>
                              <Button size="sm" variant="outline" onPress={() => mutateInstalledRepack("disable")} isDisabled={postInstallBusy === "disable" || installedRepack.status !== "enabled"}>
                                {postInstallBusy === "disable" ? <Spinner size="sm" /> : <ShieldCheck size={14} />} 禁用
                              </Button>
                              <Button size="sm" variant="ghost" onPress={() => mutateInstalledRepack("rollback")} isDisabled={postInstallBusy === "rollback"}>
                                {postInstallBusy === "rollback" ? <Spinner size="sm" /> : <RotateCcw size={14} />} 回滚
                              </Button>
                              <Link href={`/packs/detail?id=${encodeURIComponent(installedRepack.manifest.id)}`}>
                                <Button size="sm" variant="ghost">查看详情 <ArrowRight size={14} /></Button>
                              </Link>
                            </div>
                          </div>
                        )}
                        <div className="mt-2 space-y-1">
                          {repackReport.next_steps.map((step) => (
                            <div key={step}>{step}</div>
                          ))}
                        </div>
                      </div>
                    )}
                  </div>
                </div>
              )}
            </Card>

            <Card className="section-card p-4">
              <div className="flex items-center gap-2 mb-3">
                <ClipboardCheck size={16} style={{ color: "var(--yunque-primary)" }} />
                <div className="text-sm font-semibold" style={{ color: "var(--yunque-text)" }}>diff 预览</div>
              </div>
              <div className="grid gap-3 lg:grid-cols-[minmax(0,1fr)_260px]">
                <TextArea aria-label="改包 diff 预览" value={analysis.diffPreview} readOnly rows={15} className="font-mono text-xs" />
                <div className="space-y-3">
                  <div>
                    <div className="text-xs font-medium mb-2" style={{ color: "var(--yunque-text)" }}>可改文件候选</div>
                    <div className="space-y-1">
                      {analysis.editableFiles.map((file) => (
                        <div key={file} className="rounded px-2 py-1 font-mono text-[11px]" style={{ background: "var(--yunque-bg-hover)", color: "var(--yunque-text-secondary)" }}>
                          {file}
                        </div>
                      ))}
                    </div>
                  </div>
                  <div className="text-xs leading-5" style={{ color: "var(--yunque-text-muted)" }}>
                    这是预览，不会写入文件。真正改包必须在用户确认 diff 后执行，并重新跑审计与打包。
                  </div>
                </div>
              </div>
            </Card>

            <Card className="section-card p-4">
              <div className="text-sm font-semibold mb-3" style={{ color: "var(--yunque-text)" }}>建议门禁</div>
              <div className="grid gap-3 lg:grid-cols-3">
                <div>
                  <div className="mb-2 flex items-center gap-2 text-xs font-medium" style={{ color: "var(--yunque-text)" }}>
                    <ClipboardCheck size={13} /> 审计测试
                  </div>
                  <div className="space-y-1">
                    {[...analysis.auditSteps, ...analysis.tests].map((command) => (
                      <div key={command} className="rounded px-2 py-1 text-xs" style={{ background: "var(--yunque-bg-hover)", color: "var(--yunque-text-secondary)" }}>
                        {command}
                      </div>
                    ))}
                  </div>
                </div>
                <div>
                  <div className="mb-2 flex items-center gap-2 text-xs font-medium" style={{ color: "var(--yunque-text)" }}>
                    <PackageCheck size={13} /> 重新打包
                  </div>
                  <div className="space-y-1">
                    {analysis.packageSteps.map((command) => (
                      <div key={command} className="rounded px-2 py-1 font-mono text-[11px]" style={{ background: "var(--yunque-bg-hover)", color: "var(--yunque-text-secondary)" }}>
                        {command}
                      </div>
                    ))}
                  </div>
                </div>
                <div>
                  <div className="mb-2 flex items-center gap-2 text-xs font-medium" style={{ color: "var(--yunque-text)" }}>
                    <RotateCcw size={13} /> 回滚策略
                  </div>
                  <div className="space-y-1">
                    {analysis.rollbackSteps.map((step) => (
                      <div key={step} className="rounded px-2 py-1 text-xs" style={{ background: "var(--yunque-bg-hover)", color: "var(--yunque-text-secondary)" }}>
                        {step}
                      </div>
                    ))}
                  </div>
                </div>
              </div>
              <div className="mt-3 text-xs" style={{ color: "var(--yunque-text-muted)" }}>
                当前仍是安全规划模式：生成 diff、审计、打包和回滚计划，不直接修改已安装包。
              </div>
            </Card>
          </>
        )}
      </div>
    </div>
  );
}
