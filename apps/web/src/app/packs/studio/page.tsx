"use client";

import { useEffect, useMemo, useState } from "react";
import Link from "next/link";
import { useSearchParams } from "next/navigation";
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
  packReadiness,
  packUsability,
  riskProfileForPack,
} from "@/lib/pack-presentation";
import {
  packStudioWorkspaceMatches,
  parsePackStudioBatchDraftRequestPrompt,
  parsePackStudioPatchDraftRequestPrompt,
  parsePackStudioPatchDraftPrompt,
  parsePackStudioPatchPlanPrompt,
} from "@/lib/pack-studio-chat";
import { resolvePackReleaseSources } from "@/lib/pack-release-sources";
import { createPacksClient, type InstalledPack, type PackManifest, type PackStudioAuditReport, type PackStudioPatchReport, type PackStudioPlanReport, type PackStudioRepackReport, type PackStudioWorkspaceReport, type YqpackInspectReport } from "yunque-client/packs";

const packsClient = createPacksClient(createYunqueSDKClientOptions());
const PACK_RELEASE_SOURCES = resolvePackReleaseSources();
const DEFAULT_STUDIO_GOAL = "让这个能力包更像一个用户能直接理解和使用的功能，而不是只看到存在。";

type PackCandidate = {
  manifest: PackManifest;
  source: "installed" | "catalog" | "release";
  enabled: boolean;
  installed: boolean;
  packageUrl?: string;
  sha256?: string;
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
  readinessGaps: string[];
  riskLevel: "low" | "medium" | "high";
  gates: string[];
  applyable: boolean;
};

type StudioWorkflowStep = {
  key: string;
  title: string;
  state: "done" | "active" | "blocked" | "pending";
  detail: string;
  action: string;
};

function workflowStateLabel(state: StudioWorkflowStep["state"]): string {
  if (state === "done") return "已完成";
  if (state === "active") return "当前";
  if (state === "blocked") return "需处理";
  return "待开始";
}

function workflowStateColor(state: StudioWorkflowStep["state"]): "success" | "warning" | "danger" | "default" {
  if (state === "done") return "success";
  if (state === "active") return "warning";
  if (state === "blocked") return "danger";
  return "default";
}

function buildDeliverySummary(params: {
  manifest?: PackManifest;
  goal: string;
  workspaceReport: PackStudioWorkspaceReport | null;
  patchReport: PackStudioPatchReport | null;
  auditReport: PackStudioAuditReport | null;
  repackReport: PackStudioRepackReport | null;
  reinspectReport: YqpackInspectReport | null;
  installedRepack: InstalledPack | null;
  workflowSteps: StudioWorkflowStep[];
}): string {
  const { manifest, goal, workspaceReport, patchReport, auditReport, repackReport, reinspectReport, installedRepack, workflowSteps } = params;
  const lines = [
    "# Pack Studio 改包交付摘要",
    "",
    `- 能力包：${manifest?.name || "-"} (${manifest?.id || "-"})`,
    `- 版本：${manifest?.version || "-"}`,
    `- 改包目标：${goal}`,
    `- 工作区：${workspaceReport?.workspace_path || "尚未准备"}`,
    `- 原始 SHA：${workspaceReport?.original_sha256 || "-"}`,
    "",
    "## 流程状态",
    ...workflowSteps.map((step) => `- ${step.title}：${workflowStateLabel(step.state)}；下一步：${step.action}`),
    "",
    "## diff 与审计",
    patchReport
      ? `- diff：${patchReport.applied ? "已应用到工作区副本" : "仅预览"}；文件：${patchReport.relative_path}；新 SHA：${patchReport.new_sha256}`
      : "- diff：尚未生成",
    auditReport
      ? `- 审计：${auditReport.allowed ? "通过" : "阻断"}；风险：${auditReport.risk_level}；改动：${auditReport.change_count}；可改：${auditReport.editable_change_count}；需源码/专项审计：${auditReport.guarded_change_count}`
      : "- 审计：尚未运行",
    ...(auditReport?.warnings || []).map((warning) => `- 审计警告：${warning}`),
    "",
    "## 新 yqpack",
    repackReport
      ? `- 包路径：${repackReport.package_path}`
      : "- 包路径：尚未重新打包",
    repackReport
      ? `- SHA256：${repackReport.sha256}`
      : "- SHA256：-",
    repackReport
      ? `- 大小：${repackReport.size_bytes.toLocaleString()} bytes`
      : "- 大小：-",
    reinspectReport
      ? `- 复检：${reinspectReport.sha256_match ? "SHA 匹配" : "SHA 不匹配"}；文件：${reinspectReport.entry_count}；可改：${reinspectReport.editable_count}；需审计：${reinspectReport.guarded_count}`
      : "- 复检：尚未运行",
    "",
    "## 安装与回滚",
    installedRepack
      ? `- 安装状态：${installedRepack.status}；安装包：${installedRepack.manifest.name} (${installedRepack.manifest.id})`
      : "- 安装状态：尚未安装",
    "- 回滚策略：新包应作为显式安装版本处理；验证失败时先禁用，再回滚到上一版本或原始 yqpack。",
    "",
    "## 安全边界",
    "- 小羽输出不会自动写入文件，必须先进入 diff 预览。",
    "- 工作区是 yqpack 的可编辑副本，不直接修改已安装能力包。",
    "- 高风险或审计阻断改动不得继续打包/安装。",
    "- 上传 OSS 或发布前必须保留 package path、SHA256、审计结果和回滚路径。",
  ];
  return `${lines.join("\n")}\n`;
}

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

function buildManifestDraftContent(manifest: PackManifest, goal: string, readinessGaps: string[] = []): string {
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
  if (readinessGaps.includes("用户感知位置")) metadata.usageSurface ||= `用户可在 ${primaryPath} 感知和使用这个能力。`;
  if (readinessGaps.includes("后端能力声明")) {
    metadata.backendCapabilityNote ||= "当前未声明后端能力。若实际没有后端执行，请明确标注为前端/说明型能力；若要新增后端能力，必须补 routeSpecs、权限和对应测试，不能只改文案伪造能力。";
  }
  metadata.studioGoal = safeGoal;
  draft.metadata = metadata;

  return `${JSON.stringify(draft, null, 2)}\n`;
}

function buildFrontendDraftContent(manifest: PackManifest, goal: string, readinessGaps: string[] = []): string {
  const safeGoal = goal.trim() || "让这个能力包更像一个用户能直接理解和使用的功能，而不是只看到存在。";
  const primaryPath = packPrimaryPath(manifest) || "/chat";
  const capabilities = (manifest.backend?.capabilities || []).join(", ") || "无";
  const permissions = (manifest.backend?.permissions || []).join(", ") || "无";
  const gapText = readinessGaps.length > 0 ? readinessGaps.join("、") : "无";

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
    "    <section>",
    "      <h2>这次补齐的体检缺口</h2>",
    `      <p>${gapText}</p>`,
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
  const readiness = packReadiness(workspace.manifest);
  const manifestGaps = readiness.missing;
  const frontendGaps = readiness.missing.filter((gap) => ["使用示例", "用户感知位置", "打开/使用入口"].includes(gap));
  for (const filePath of workspace.editable_files || []) {
    const normalized = filePath.replace(/\\/g, "/").toLowerCase();
    if (seen.has(normalized)) continue;
    seen.add(normalized);
    if (normalized.endsWith("/pack.json") || normalized.endsWith("pack.json")) {
      candidates.push({
        key: `manifest:${filePath}`,
        label: manifestGaps.length > 0 ? "体检缺口 manifest 草稿" : "manifest 草稿",
        filePath,
        content: buildManifestDraftContent(workspace.manifest, goal, manifestGaps),
        summary: manifestGaps.length > 0
          ? `按体检缺口补 ${manifestGaps.join("、")}。`
          : "补用途、入口、示例、限制和 Studio 目标。",
        reason: "manifest 是能力包契约入口，适合先补用户能理解的用途、入口、限制和回滚提示。",
        readinessGaps: manifestGaps,
        riskLevel: "low",
        gates: ["预览 diff", "内置审计", "Pack 可用性扫描", ...(manifestGaps.length > 0 ? ["复跑体检缺口"] : [])],
        applyable: true,
      });
      continue;
    }
    if (normalized.includes("/frontend/") && normalized.endsWith(".html")) {
      candidates.push({
        key: `frontend:${filePath}`,
        label: frontendGaps.length > 0 ? "体检缺口前端草稿" : "前端界面草稿",
        filePath,
        content: buildFrontendDraftContent(workspace.manifest, goal, frontendGaps),
        summary: frontendGaps.length > 0
          ? `把 ${frontendGaps.join("、")} 落到可见界面里。`
          : "补一个可感知的沙箱界面草稿。",
        reason: "HTML 前端资源可在 yqpack 工作区内预览和替换，适合补独立界面、权限说明和结果区。",
        readinessGaps: frontendGaps,
        riskLevel: "medium",
        gates: ["预览 diff", "内置审计", "重新打包", "复检 yqpack", ...(frontendGaps.length > 0 ? ["复跑体检缺口"] : [])],
        applyable: true,
      });
    }
  }
  return candidates;
}

function stableStringHash(value: string): string {
  let hash = 0;
  for (let index = 0; index < value.length; index += 1) {
    hash = (hash * 31 + value.charCodeAt(index)) >>> 0;
  }
  return hash.toString(16).padStart(8, "0");
}

function buildPatchPlan(workspace: PackStudioWorkspaceReport, candidates: StudioDraftCandidate[], goal: string) {
  return {
    kind: "yunque.pack_studio.patch_plan.v1",
    pack: {
      id: workspace.manifest.id,
      name: workspace.manifest.name,
      version: workspace.manifest.version,
    },
    goal,
    workspace: {
      id: workspace.workspace_id,
      path: workspace.workspace_path,
      original_sha256: workspace.original_sha256,
    },
    rules: [
      "Only load one candidate into the workspace patch editor at a time.",
      "Preview diff before applying.",
      "Run built-in audit after applying.",
      "Repack, reinspect sha256, then install or rollback explicitly.",
    ],
    candidates: candidates.map((candidate) => ({
      key: candidate.key,
      label: candidate.label,
      file_path: candidate.filePath,
      risk_level: candidate.riskLevel,
      applyable: candidate.applyable,
      reason: candidate.reason,
      gates: candidate.gates,
      content_summary: {
        length: candidate.content.length,
        hash: stableStringHash(candidate.content),
      },
    })),
  };
}

function buildPatchPlanPrompt(prompt: string, patchPlan: ReturnType<typeof buildPatchPlan>): string {
  return [
    prompt,
    "",
    "下面是 Pack Studio 已准备好的 Patch Plan。请只把它当作结构化导航和安全约束，不要假设里面包含完整文件内容。",
    "你需要引导用户在 Pack Studio 中载入草稿、预览 diff、应用到工作区、运行内置审计、重新打包、复检 SHA，再决定安装或回滚。",
    "如果需要修改具体内容，请先要求用户在 Pack Studio 打开对应候选并查看 diff；不要绕过权限、签名、审计或回滚流程。",
    "",
    "```json",
    JSON.stringify(patchPlan, null, 2),
    "```",
  ].join("\n");
}

function buildPatchDraftRequestPrompt(prompt: string, workspace: PackStudioWorkspaceReport, candidate: StudioDraftCandidate, goal: string): string {
  const request = {
    kind: "yunque.pack_studio.patch_draft_request.v1",
    pack: {
      id: workspace.manifest.id,
      name: workspace.manifest.name,
      version: workspace.manifest.version,
    },
    goal,
    workspace: {
      id: workspace.workspace_id,
      path: workspace.workspace_path,
      original_sha256: workspace.original_sha256,
    },
    target: {
      file_path: candidate.filePath,
      label: candidate.label,
      reason: candidate.reason,
      readiness_gaps: candidate.readinessGaps,
      risk_level: candidate.riskLevel,
      gates: candidate.gates,
      content_summary: {
        length: candidate.content.length,
        hash: stableStringHash(candidate.content),
      },
    },
    starter_content: candidate.content,
    expected_output: {
      kind: "yunque.pack_studio.patch_draft.v1",
      file_path: candidate.filePath,
      content: "完整的新文件内容，不是 diff，也不是片段",
      risk_level: candidate.riskLevel,
      gates: candidate.gates,
    },
  };

  return [
    prompt,
    "",
    "下面是 Pack Studio 的 Patch Draft Request。请基于 starter_content 和目标，生成一个单文件 Patch Draft。",
    candidate.readinessGaps.length > 0
      ? `这次必须优先补齐体检缺口：${candidate.readinessGaps.join("、")}。如果某个缺口需要后端源码或新 route，请明确写成限制/待办，不要伪造能力。`
      : "这次没有强制体检缺口；请继续打磨真实可用路径和用户反馈。",
    "输出必须只包含一段 fenced JSON，kind 必须是 yunque.pack_studio.patch_draft.v1。",
    "content 必须是完整的新文件内容，不要输出 diff、片段或解释文本。",
    "不要声称已经应用改动；用户回到 Studio 导入后仍要预览 diff、运行内置审计、重新打包和复检 SHA。",
    "",
    "```json",
    JSON.stringify(request, null, 2),
    "```",
  ].join("\n");
}

function sourceLabel(candidate: PackCandidate): string {
  if (candidate.installed && candidate.enabled) return "已启用";
  if (candidate.installed) return "已安装";
  if (candidate.source === "release") return "官方源";
  if (candidate.source === "catalog") return "私有源";
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
  const readiness = packReadiness(manifest);
  const routes = packRoutes(manifest);
  const paths = packPaths(manifest);
  const permissions = manifest.backend?.permissions || [];
  const capabilities = manifest.backend?.capabilities || [];

  const editable = [
    "用途说明、起手示例、入口文案、可用度分层和权限解释可以从 manifest/前端展示层优化。",
  ];
  if (readiness.missing.length > 0) {
    editable.push(`能力包体检缺口：${readiness.missing.join("、")}，优先补齐这些用户可感知信息。`);
  }
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
  if (readiness.missing.length > 0) warnings.push(`能力包体检：${readiness.label}，还缺 ${readiness.missing.join("、")}。`);
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
  const readiness = packReadiness(manifest);
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
    `- 能力包体检：${readiness.label}${readiness.missing.length > 0 ? `；还缺 ${readiness.missing.join("、")}` : "；说明已基本完整"}`,
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

function withReadinessPrompt(prompt: string, manifest: PackManifest): string {
  if (prompt.includes("能力包体检：")) return prompt;
  const readiness = packReadiness(manifest);
  return [
    prompt,
    "",
    `能力包体检：${readiness.label}${readiness.missing.length > 0 ? `；还缺 ${readiness.missing.join("、")}` : "；说明已基本完整"}`,
    readiness.missing.length > 0
      ? "请优先把这些缺口落实到 pack.json metadata、前端入口文案、示例或能力边界说明里。"
      : "可以继续打磨更具体的用户路径、结果反馈和回滚提示。",
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
  const searchParams = useSearchParams();
  const { data, loading, refresh } = useApiData(async () => {
    const [installed, catalog, releaseCatalog] = await Promise.all([
      packsClient.installed(),
      packsClient.catalog(),
      packsClient.releaseCatalog(PACK_RELEASE_SOURCES.map((source) => source.url)),
    ]);
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
    for (const entry of releaseCatalog.entries || []) {
      if (!map.has(entry.manifest.id)) {
        map.set(entry.manifest.id, {
          manifest: entry.manifest,
          source: "release",
          enabled: Boolean(entry.enabled),
          installed: Boolean(entry.installed),
          packageUrl: entry.package_url,
          sha256: entry.sha256,
        });
      }
    }
    return { packs: [...map.values()].sort((a, b) => a.manifest.name.localeCompare(b.manifest.name)) };
  }, { packs: [] as PackCandidate[] });
  const [selectedId, setSelectedId] = useState(() => searchParams.get("packId") || "");
  const [goal, setGoal] = useState(() => searchParams.get("goal") || DEFAULT_STUDIO_GOAL);
  const [packagePath, setPackagePath] = useState(() => searchParams.get("packagePath") || "");
  const [packageUrl, setPackageUrl] = useState(() => searchParams.get("packageUrl") || "");
  const [packageSHA, setPackageSHA] = useState(() => searchParams.get("sha256") || "");
  const hasPackageSource = Boolean(packagePath.trim() || packageUrl.trim() || packageSHA.trim());
  const [inspecting, setInspecting] = useState(false);
  const [inspectReport, setInspectReport] = useState<YqpackInspectReport | null>(null);
  const [preparingWorkspace, setPreparingWorkspace] = useState(false);
  const [workspaceReport, setWorkspaceReport] = useState<PackStudioWorkspaceReport | null>(null);
  const [patchFile, setPatchFile] = useState("");
  const [patchContent, setPatchContent] = useState("");
  const [importedBatchText, setImportedBatchText] = useState("");
  const [importedPatchPlanText, setImportedPatchPlanText] = useState("");
  const [importedPatchDraftText, setImportedPatchDraftText] = useState("");
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

  useEffect(() => {
    if (!selected || selected.source !== "release") return;
    if (selected.packageUrl && !packageUrl.trim()) setPackageUrl(selected.packageUrl);
    if (selected.sha256 && !packageSHA.trim()) setPackageSHA(selected.sha256);
  }, [packageSHA, packageUrl, selected]);

  const selectCandidate = (candidate: PackCandidate) => {
    setSelectedId(candidate.manifest.id);
    if (candidate.source === "release") {
      if (candidate.packageUrl) {
        setPackagePath("");
        setPackageUrl(candidate.packageUrl);
      }
      if (candidate.sha256) setPackageSHA(candidate.sha256);
    }
    setInspectReport(null);
    setWorkspaceReport(null);
    setPatchReport(null);
    setAuditReport(null);
    setRepackReport(null);
    setReinspectReport(null);
    setInstalledRepack(null);
  };

  const selectBatchPack = (pack: { id: string; name: string; studioUrl: string; packageUrl: string; sha256: string }) => {
    const candidate = candidates.find((item) => item.manifest.id === pack.id);
    if (candidate) selectCandidate(candidate);
    if (pack.packageUrl) {
      setPackagePath("");
      setPackageUrl(pack.packageUrl);
    }
    if (pack.sha256) setPackageSHA(pack.sha256);
    if (pack.studioUrl) {
      try {
        const url = new URL(pack.studioUrl, window.location.origin);
        const linkedGoal = url.searchParams.get("goal");
        if (linkedGoal) setGoal(linkedGoal);
      } catch {
        // Keep the pasted batch visible even if a generated URL is malformed.
      }
    }
  };

  const manifest = selected?.manifest;
  const readiness = manifest ? packReadiness(manifest) : null;
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
  const prompt = manifest ? withReadinessPrompt(analysis?.prompt || buildXiaoyuPrompt(manifest, goal), manifest) : "";
  const chatHref = `/chat?q=${encodeURIComponent(prompt)}`;
  const draftCandidates = useMemo(
    () => workspaceReport ? buildStudioDraftCandidates(workspaceReport, goal) : [],
    [workspaceReport, goal],
  );
  const readinessDraftCandidate = useMemo(
    () => draftCandidates.find((candidate) => candidate.readinessGaps.length > 0) || draftCandidates[0],
    [draftCandidates],
  );
  const patchPlan = useMemo(
    () => workspaceReport && draftCandidates.length > 0 ? buildPatchPlan(workspaceReport, draftCandidates, goal) : null,
    [workspaceReport, draftCandidates, goal],
  );
  const patchPlanChatHref = patchPlan ? `/chat?q=${encodeURIComponent(buildPatchPlanPrompt(prompt, patchPlan))}` : "";
  const importedBatchRequest = useMemo(
    () => parsePackStudioBatchDraftRequestPrompt(importedBatchText),
    [importedBatchText],
  );
  const importedPatchPlan = useMemo(
    () => parsePackStudioPatchPlanPrompt(importedPatchPlanText),
    [importedPatchPlanText],
  );
  const importedPatchDraft = useMemo(
    () => parsePackStudioPatchDraftPrompt(importedPatchDraftText),
    [importedPatchDraftText],
  );
  const importedPatchDraftRequest = useMemo(
    () => parsePackStudioPatchDraftRequestPrompt(importedPatchDraftText),
    [importedPatchDraftText],
  );
  const importedPatchPlanMatchesWorkspace = useMemo(() => {
    return packStudioWorkspaceMatches(importedPatchPlan?.workspace, workspaceReport);
  }, [importedPatchPlan, workspaceReport]);
  const importedPatchDraftMatchesWorkspace = useMemo(() => {
    return packStudioWorkspaceMatches(importedPatchDraft?.workspace, workspaceReport);
  }, [importedPatchDraft, workspaceReport]);
  const importedPatchDraftRequestMatchesWorkspace = useMemo(() => {
    return packStudioWorkspaceMatches(importedPatchDraftRequest?.workspace, workspaceReport);
  }, [importedPatchDraftRequest, workspaceReport]);
  const workflowSteps = useMemo<StudioWorkflowStep[]>(() => {
    const hasDraftQueue = draftCandidates.length > 0;
    const hasPreparedDraft = Boolean(patchContent.trim())
      || Boolean(importedPatchDraft && importedPatchDraftMatchesWorkspace);
    const hasAppliedPatch = Boolean(patchReport?.applied);
    const auditBlocked = auditReport?.allowed === false;
    const auditPassed = auditReport?.allowed === true;
    const repackReady = Boolean(repackReport);
    const reinspectPassed = Boolean(reinspectReport?.sha256_match);
    const installed = Boolean(installedRepack);
    const enabled = installedRepack?.status === "enabled";

    return [
      {
        key: "inspect",
        title: "只读检查",
        state: inspectReport ? "done" : "active",
        detail: inspectReport ? `${inspectReport.entry_count} 个文件 · ${inspectReport.editable_count} 可改` : "先检查 yqpack 内容和 SHA，不安装、不启用。",
        action: inspectReport ? "可继续准备工作区" : "填写路径/URL 后点击只读检查",
      },
      {
        key: "workspace",
        title: "准备工作区",
        state: workspaceReport ? "done" : inspectReport?.sha256_match ? "active" : "pending",
        detail: workspaceReport ? workspaceReport.workspace_id : "创建可编辑副本，避免直接改已安装包。",
        action: workspaceReport ? "可生成或导入草稿" : "SHA 匹配后准备工作区",
      },
      {
        key: "draft",
        title: "Plan / Draft",
        state: hasPreparedDraft ? "done" : workspaceReport ? "active" : "pending",
        detail: hasPreparedDraft ? "已有草稿内容，仍需先看 diff。" : hasDraftQueue ? "已有候选队列，尚未载入单文件草稿。" : "让小羽给计划，或从候选里载入单文件草稿。",
        action: hasPreparedDraft ? "点击预览 diff" : hasDraftQueue ? "载入草稿或交给小羽生成 Draft" : "导入 Plan、导入 Draft，或载入草稿",
      },
      {
        key: "diff",
        title: "diff 预览 / 应用",
        state: hasAppliedPatch ? "done" : patchReport ? "active" : hasPreparedDraft ? "active" : "pending",
        detail: patchReport ? (patchReport.applied ? "改动已写入工作区副本。" : "已有 diff 预览，尚未写入。") : "先预览，再由用户确认应用。",
        action: hasAppliedPatch ? "运行内置审计" : patchReport ? "确认后应用到工作区" : "预览 diff",
      },
      {
        key: "audit",
        title: "内置审计",
        state: auditBlocked ? "blocked" : auditPassed ? "done" : hasAppliedPatch ? "active" : "pending",
        detail: auditReport ? `${auditReport.change_count} 个改动 · 风险 ${auditReport.risk_level}` : "检查越权文件、不可改内容和高风险权限。",
        action: auditBlocked ? "按审计提示回退或改小范围" : auditPassed ? "可以重新打包" : "运行内置审计",
      },
      {
        key: "repack",
        title: "重新打包",
        state: repackReady ? "done" : auditBlocked ? "blocked" : auditPassed ? "active" : "pending",
        detail: repackReady ? `${repackReport?.size_bytes.toLocaleString()} bytes` : "生成新的 yqpack，不覆盖原包。",
        action: repackReady ? "复检新包 SHA" : auditBlocked ? "审计阻断时不能继续打包" : "重新打包",
      },
      {
        key: "install",
        title: "复检 / 安装",
        state: installed ? "done" : reinspectPassed ? "active" : repackReady ? "active" : "pending",
        detail: installed ? "新包已进入本地能力包列表。" : reinspectPassed ? "SHA 已匹配，可显式安装。" : "安装前必须复检新 yqpack。",
        action: installed ? "确认权限后启用或回滚" : reinspectPassed ? "安装新包" : "复检新包",
      },
      {
        key: "enable",
        title: "启用 / 回滚",
        state: enabled ? "done" : installed ? "active" : "pending",
        detail: enabled ? "新能力已启用，保留禁用和回滚路径。" : "启用仍需用户明确确认。",
        action: enabled ? "打开入口或查看详情" : installed ? "启用、禁用或回滚" : "安装后再处理启用",
      },
    ];
  }, [
    auditReport,
    draftCandidates.length,
    importedPatchDraft,
    importedPatchDraftMatchesWorkspace,
    importedPatchPlan,
    importedPatchPlanMatchesWorkspace,
    inspectReport,
    installedRepack,
    patchContent,
    patchReport,
    reinspectReport,
    repackReport,
    workspaceReport,
  ]);
  const releaseChecklist = useMemo(() => [
    {
      label: "原包已只读检查",
      ready: Boolean(inspectReport?.sha256_match),
      detail: inspectReport
        ? `${inspectReport.entry_count} 个文件，${inspectReport.editable_count} 个可改`
        : "先确认来源包、manifest、SHA 和文件分类。",
    },
    {
      label: "可编辑工作区已准备",
      ready: Boolean(workspaceReport),
      detail: workspaceReport
        ? workspaceReport.workspace_id
        : "工作区是改包副本，不直接修改已安装能力包。",
    },
    {
      label: "内置审计已通过",
      ready: auditReport?.allowed === true,
      detail: auditReport
        ? `风险 ${auditReport.risk_level}，${auditReport.change_count} 个改动`
        : "确认没有越权文件、不可改内容和高风险权限扩大。",
    },
    {
      label: "重打包产物已生成",
      ready: Boolean(repackReport),
      detail: repackReport
        ? `${repackReport.size_bytes.toLocaleString()} bytes`
        : "重新打包不会覆盖原包。",
    },
    {
      label: "新 yqpack 已复检",
      ready: Boolean(reinspectReport?.sha256_match),
      detail: reinspectReport
        ? `${reinspectReport.entry_count} 个文件，SHA ${reinspectReport.sha256_match ? "匹配" : "不匹配"}`
        : "上传 OSS 或安装前必须复检新包 SHA。",
    },
    {
      label: "回滚路径已记录",
      ready: Boolean(workspaceReport?.rollback_commands?.length),
      detail: workspaceReport?.rollback_commands?.length
        ? `已记录 ${workspaceReport.rollback_commands.length} 条禁用/回滚命令`
        : "保留禁用、回滚或恢复上一版本的路径。",
    },
  ], [auditReport, inspectReport, reinspectReport, repackReport, workspaceReport]);
  const releaseReady = releaseChecklist.every((item) => item.ready);
  const deliverySummary = useMemo(() => buildDeliverySummary({
    manifest,
    goal,
    workspaceReport,
    patchReport,
    auditReport,
    repackReport,
    reinspectReport,
    installedRepack,
    workflowSteps,
  }), [auditReport, goal, installedRepack, manifest, patchReport, reinspectReport, repackReport, workflowSteps, workspaceReport]);

  const copyPrompt = async () => {
    if (!prompt) return;
    await navigator.clipboard?.writeText(prompt);
    showToast("已复制小羽改包任务", "success");
  };

  const copyDeliverySummary = async () => {
    await navigator.clipboard?.writeText(deliverySummary);
    showToast("已复制改包交付摘要", "success");
  };

  const copyDraftPlan = async () => {
    if (!patchPlan) return;
    await navigator.clipboard?.writeText(JSON.stringify(patchPlan, null, 2));
    showToast("已复制结构化 Patch Plan", "success");
  };

  const copyPatchDraftRequest = async (candidate: StudioDraftCandidate) => {
    if (!workspaceReport) return;
    await navigator.clipboard?.writeText(buildPatchDraftRequestPrompt(prompt, workspaceReport, candidate, goal));
    showToast("已复制 Patch Draft 请求", "success");
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

  const fillImportedPatchCandidate = (candidate: NonNullable<typeof importedPatchPlan>["candidates"][number]) => {
    if (!importedPatchPlanMatchesWorkspace) {
      showToast("Patch Plan 与当前工作区不匹配，请重新从当前工作区生成", "warning");
      return;
    }
    setPatchFile(candidate.filePath);
    setPatchContent("");
    setPatchReport(null);
    showToast("已填入 Patch Plan 目标文件；请补入新内容后再预览 diff", "success");
  };

  const fillImportedPatchDraft = () => {
    if (!importedPatchDraft) return;
    if (!importedPatchDraftMatchesWorkspace) {
      showToast("Patch Draft 与当前工作区不匹配，请重新从当前工作区生成", "warning");
      return;
    }
    setPatchFile(importedPatchDraft.filePath);
    setPatchContent(importedPatchDraft.content);
    setPatchReport(null);
    showToast("已载入 Patch Draft，请先预览 diff 再应用", "success");
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
                    onClick={() => selectCandidate(candidate)}
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

        <Card className="section-card p-4">
          <div className="mb-3 flex flex-wrap items-start justify-between gap-3">
            <div className="flex items-start gap-2">
              <ClipboardCheck size={16} style={{ color: "var(--yunque-accent)" }} />
              <div>
                <div className="text-sm font-semibold" style={{ color: "var(--yunque-text)" }}>导入批量补肉任务</div>
                <div className="mt-1 text-xs leading-5" style={{ color: "var(--yunque-text-muted)" }}>
                  从能力包中心或 Chat 粘贴 yunque.pack_studio.batch_draft_request.v1；Studio 会拆成逐包处理入口，不会批量自动改包。
                </div>
              </div>
            </div>
            {importedBatchRequest && (
              <div className="flex flex-wrap gap-2">
                <Chip size="sm" color="success">{importedBatchRequest.packs.length} 个包</Chip>
                <Chip size="sm" variant="soft">逐包处理</Chip>
              </div>
            )}
          </div>
          <TextArea
            aria-label="导入批量补肉任务 JSON"
            value={importedBatchText}
            onChange={(event) => setImportedBatchText(event.target.value)}
            rows={4}
          >
            <Label>批量任务 JSON 或 Chat 消息</Label>
          </TextArea>
          {importedBatchText.trim() && !importedBatchRequest && (
            <div className="mt-2 rounded px-2 py-1 text-[11px]" style={{ background: "rgba(248,113,113,0.08)", color: "var(--yunque-danger)" }}>
              未识别到 yunque.pack_studio.batch_draft_request.v1。请粘贴能力包中心生成的完整 JSON fenced block 或原始 Chat 消息。
            </div>
          )}
          {importedBatchRequest && (
            <div className="mt-3 space-y-3">
              <div className="grid gap-2 text-[11px] lg:grid-cols-[minmax(0,1fr)_minmax(0,1.2fr)]" style={{ color: "var(--yunque-text-muted)" }}>
                <div className="rounded px-2 py-2" style={{ background: "var(--yunque-bg-hover)" }}>
                  目标：{importedBatchRequest.goal || "逐包补齐用途、入口、示例、权限边界和回滚说明。"}
                </div>
                <div className="rounded px-2 py-2" style={{ background: "var(--yunque-bg-hover)" }}>
                  规则：{importedBatchRequest.rules.slice(0, 2).join("；") || "不要自动应用改动，先回到 Studio 预览 diff / 审计 / 重新打包。"}
                </div>
              </div>
              <div className="grid gap-2 lg:grid-cols-2">
                {importedBatchRequest.packs.map((pack) => {
                  const candidate = candidates.find((item) => item.manifest.id === pack.id);
                  const href = pack.studioUrl || `/packs/studio?packId=${encodeURIComponent(pack.id)}`;
                  return (
                    <div key={`${pack.id}:${pack.studioUrl}`} className="rounded-md border p-3" style={{ borderColor: "var(--yunque-border)", background: "var(--yunque-surface)" }}>
                      <div className="flex flex-wrap items-start justify-between gap-2">
                        <div className="min-w-0">
                          <div className="truncate text-xs font-medium" style={{ color: "var(--yunque-text)" }}>{pack.name || pack.id}</div>
                          <div className="mt-1 truncate font-mono text-[11px]" style={{ color: "var(--yunque-text-muted)" }}>{pack.id}</div>
                        </div>
                        <div className="flex flex-wrap gap-1">
                          {pack.readiness && <Chip size="sm" color={pack.readiness.includes("入口") ? "danger" : "warning"}>{pack.readiness}</Chip>}
                          <Chip size="sm" variant="soft">{pack.source || "来源未知"}</Chip>
                        </div>
                      </div>
                      {pack.missing.length > 0 && (
                        <div className="mt-2 flex flex-wrap gap-1">
                          {pack.missing.map((gap) => (
                            <Chip key={`${pack.id}:${gap}`} size="sm" variant="soft">补：{gap}</Chip>
                          ))}
                        </div>
                      )}
                      {(pack.packageUrl || pack.sha256) && (
                        <div className="mt-2 space-y-1 text-[11px]" style={{ color: "var(--yunque-text-muted)" }}>
                          {pack.packageUrl && <div className="truncate">yqpack：{pack.packageUrl}</div>}
                          {pack.sha256 && <div className="truncate font-mono">SHA：{pack.sha256}</div>}
                        </div>
                      )}
                      <div className="mt-3 flex flex-wrap gap-2">
                        <Button size="sm" variant="outline" onPress={() => selectBatchPack(pack)} isDisabled={!candidate}>
                          载入本页
                        </Button>
                        <Link href={href}>
                          <Button size="sm" variant="ghost">
                            打开 Studio <ArrowRight size={13} />
                          </Button>
                        </Link>
                      </div>
                      {!candidate && (
                        <div className="mt-2 text-[11px]" style={{ color: "var(--yunque-warning)" }}>
                          当前候选列表未找到这个包；请先安装或刷新官方/私有源。
                        </div>
                      )}
                    </div>
                  );
                })}
              </div>
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
              {readiness && (
                <div className="mt-3 rounded-md border p-3 text-xs" style={{ borderColor: "var(--yunque-border)", background: "var(--yunque-bg-hover)", color: "var(--yunque-text-secondary)" }}>
                  <div className="mb-1 flex flex-wrap items-center gap-2">
                    <span className="font-medium" style={{ color: "var(--yunque-text)" }}>能力包体检</span>
                    <Chip size="sm" color={readiness.level === "complete" ? "success" : readiness.level === "needs_context" ? "warning" : "danger"}>
                      {readiness.label}
                    </Chip>
                  </div>
                  {readiness.missing.length > 0
                    ? `小羽会优先补齐：${readiness.missing.join("、")}。`
                    : "说明、入口、示例与能力边界已基本完整，可继续打磨更具体的用户路径。"}
                </div>
              )}
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
                {patchPlanChatHref && (
                  <Link href={patchPlanChatHref}>
                    <Button variant="outline">
                      交给 Chat 里的小羽（带 Patch Plan） <ArrowRight size={14} />
                    </Button>
                  </Link>
                )}
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
              {hasPackageSource && !inspectReport && (
                <div className="mt-3 flex flex-col gap-3 rounded-md border p-3 text-xs md:flex-row md:items-start md:justify-between" style={{ borderColor: "var(--yunque-border)", background: "var(--yunque-bg-hover)", color: "var(--yunque-text-secondary)" }}>
                  <div className="flex min-w-0 gap-2">
                    <ShieldCheck size={15} style={{ color: "var(--yunque-primary)", flex: "0 0 auto" }} />
                    <div className="min-w-0">
                      <div className="font-medium" style={{ color: "var(--yunque-text)" }}>已从能力包中心接入这个 yqpack</div>
                      <div className="mt-1">不用回到商店手动找包；先在这里做只读检查，再进入工作区、diff 预览、审计和重新打包。这一步只校验 SHA、manifest 与文件分类，不会安装、启用或改动本地能力包。</div>
                      <div className="mt-2 space-y-1 font-mono text-[11px]" style={{ color: "var(--yunque-text-muted)" }}>
                        {packageUrl.trim() && <div className="break-all">URL: {packageUrl.trim()}</div>}
                        {packageSHA.trim() && <div className="break-all">SHA256: {packageSHA.trim()}</div>}
                      </div>
                    </div>
                  </div>
                  <Button size="sm" className="btn-accent shrink-0" onPress={inspectYqpack} isDisabled={inspecting}>
                    {inspecting ? <Spinner size="sm" /> : <FileSearch size={14} />} 立即只读检查
                  </Button>
                </div>
              )}
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
                    <div className="mb-3 flex flex-wrap items-center justify-between gap-2">
                      <div>
                        <div className="text-xs font-medium" style={{ color: "var(--yunque-text)" }}>改包工作流状态</div>
                        <div className="mt-1 text-[11px]" style={{ color: "var(--yunque-text-muted)" }}>
                          小羽可以帮你生成计划和草稿，但每一步都必须经过 diff、审计、复检和显式安装确认。
                        </div>
                      </div>
                      <div className="flex flex-wrap items-center gap-2">
                        <Chip size="sm" variant="soft">不自动应用</Chip>
                        <Button size="sm" variant="outline" onPress={copyDeliverySummary}>
                          <Copy size={13} /> 复制交付摘要
                        </Button>
                        {workspaceReport && readinessDraftCandidate && readinessDraftCandidate.readinessGaps.length > 0 && (
                          <Link href={`/chat?q=${encodeURIComponent(buildPatchDraftRequestPrompt(prompt, workspaceReport, readinessDraftCandidate, goal))}`}>
                            <Button size="sm" className="btn-accent">
                              <Sparkles size={13} /> 按体检缺口交给小羽生成 Draft
                            </Button>
                          </Link>
                        )}
                      </div>
                    </div>
                    <div className="grid gap-2 lg:grid-cols-4">
                      {workflowSteps.map((step) => (
                        <div key={step.key} className="rounded-md border p-2" style={{ borderColor: "var(--yunque-border)", background: "var(--yunque-surface)" }}>
                          <div className="flex items-start justify-between gap-2">
                            <div className="text-xs font-medium" style={{ color: "var(--yunque-text)" }}>{step.title}</div>
                            <Chip size="sm" color={workflowStateColor(step.state)}>{workflowStateLabel(step.state)}</Chip>
                          </div>
                          <div className="mt-2 text-[11px] leading-5" style={{ color: "var(--yunque-text-muted)" }}>{step.detail}</div>
                          <div className="mt-2 text-[11px]" style={{ color: step.state === "blocked" ? "var(--yunque-danger)" : "var(--yunque-text-secondary)" }}>
                            下一步：{step.action}
                          </div>
                        </div>
                      ))}
                    </div>
                    <div className="mt-4 rounded-md border p-3" style={{ borderColor: "var(--yunque-border)", background: "var(--yunque-surface)" }}>
                      <div className="mb-3 flex flex-wrap items-start justify-between gap-2">
                        <div>
                          <div className="text-xs font-medium" style={{ color: "var(--yunque-text)" }}>上传 OSS 前检查清单</div>
                          <div className="mt-1 text-[11px]" style={{ color: "var(--yunque-text-muted)" }}>
                            全部就绪后再把新 yqpack 放到 Release 或 OSS；清单不会替你上传，也不会自动启用能力包。
                          </div>
                        </div>
                        <Chip size="sm" color={releaseReady ? "success" : "warning"}>
                          {releaseReady ? "可上传/发布" : "继续检查"}
                        </Chip>
                      </div>
                      <div className="grid gap-2 md:grid-cols-2 xl:grid-cols-3">
                        {releaseChecklist.map((item) => (
                          <div key={item.label} className="rounded px-2 py-2 text-xs" style={{ background: "var(--yunque-bg-hover)", color: "var(--yunque-text-secondary)" }}>
                            <div className="mb-1 flex items-center justify-between gap-2">
                              <span className="font-medium" style={{ color: "var(--yunque-text)" }}>{item.label}</span>
                              <Chip size="sm" color={item.ready ? "success" : "warning"}>{item.ready ? "就绪" : "待完成"}</Chip>
                            </div>
                            <div className="break-all text-[11px] leading-5">{item.detail}</div>
                          </div>
                        ))}
                      </div>
                    </div>
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
                    <div id="import-plan" className="mt-3 scroll-mt-24 rounded-md border p-3" style={{ borderColor: "var(--yunque-border)" }}>
                      <div className="mb-2 flex flex-wrap items-center justify-between gap-2">
                        <div>
                          <div className="text-xs font-medium" style={{ color: "var(--yunque-text)" }}>从 Chat 导入 Patch Plan</div>
                          <div className="mt-1 text-[11px]" style={{ color: "var(--yunque-text-muted)" }}>
                            粘贴小羽整理出的 Patch Plan，只解析工作区、候选文件、风险和门禁；不会自动应用内容。
                          </div>
                        </div>
                        {importedPatchPlan && (
                          <div className="flex flex-wrap gap-2">
                            <Chip size="sm" color={importedPatchPlanMatchesWorkspace ? "success" : "warning"}>
                              {importedPatchPlanMatchesWorkspace ? "工作区匹配" : "工作区待确认"}
                            </Chip>
                            <Chip size="sm" color="success">{importedPatchPlan.candidates.length} 个候选</Chip>
                          </div>
                        )}
                      </div>
                      <TextArea
                        aria-label="导入 Patch Plan JSON"
                        value={importedPatchPlanText}
                        onChange={(event) => setImportedPatchPlanText(event.target.value)}
                        rows={4}
                      >
                        <Label>Patch Plan JSON 或 Chat 消息</Label>
                      </TextArea>
                      {importedPatchPlanText.trim() && !importedPatchPlan && (
                        <div className="mt-2 rounded px-2 py-1 text-[11px]" style={{ background: "rgba(248,113,113,0.08)", color: "var(--yunque-danger)" }}>
                          未识别到 yunque.pack_studio.patch_plan.v1。请粘贴完整 JSON fenced block 或原始 Chat 消息。
                        </div>
                      )}
                      {importedPatchPlan && (
                        <div className="mt-3 space-y-2">
                          {!importedPatchPlanMatchesWorkspace && (
                            <div className="rounded px-2 py-1 text-[11px]" style={{ background: "rgba(245,158,11,0.10)", color: "var(--yunque-warning)" }}>
                              这个 Patch Plan 的工作区或原始 SHA 与当前工作区不一致。请回到当前工作区重新生成计划，或先确认你没有切换 yqpack。
                            </div>
                          )}
                          <div className="grid gap-2 text-[11px] lg:grid-cols-2" style={{ color: "var(--yunque-text-muted)" }}>
                            <div className="rounded px-2 py-1" style={{ background: "var(--yunque-bg-hover)" }}>
                              包：{importedPatchPlan.pack.name || importedPatchPlan.pack.id} · {importedPatchPlan.pack.version || "unknown"}
                            </div>
                            <div className="rounded px-2 py-1 font-mono" style={{ background: "var(--yunque-bg-hover)" }}>
                              工作区：{importedPatchPlan.workspace.id || importedPatchPlan.workspace.path}
                            </div>
                          </div>
                          <div className="grid gap-2 lg:grid-cols-2">
                            {importedPatchPlan.candidates.map((candidate) => (
                              <div key={`${candidate.key}:${candidate.filePath}`} className="rounded-md border p-2" style={{ borderColor: "var(--yunque-border)" }}>
                                <div className="flex flex-wrap items-center justify-between gap-2">
                                  <div className="flex flex-wrap items-center gap-2">
                                    <span className="text-xs font-medium" style={{ color: "var(--yunque-text)" }}>{candidate.label || "候选改动"}</span>
                                    {candidate.riskLevel && <Chip size="sm" variant="soft">风险：{candidate.riskLevel}</Chip>}
                                    {candidate.contentSummary && <Chip size="sm" variant="soft">摘要：{candidate.contentSummary.hash}</Chip>}
                                  </div>
                                  <Button size="sm" variant="outline" onPress={() => fillImportedPatchCandidate(candidate)} isDisabled={!candidate.applyable || !importedPatchPlanMatchesWorkspace}>
                                    <FileSearch size={13} /> 填入文件
                                  </Button>
                                </div>
                                <div className="mt-1 truncate font-mono text-[11px]" style={{ color: "var(--yunque-text-muted)" }}>{candidate.filePath}</div>
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
                      <div id="import-draft" className="mt-3 scroll-mt-24 rounded-md border p-3" style={{ borderColor: "var(--yunque-border)" }}>
                        <div className="mb-2 flex flex-wrap items-center justify-between gap-2">
                          <div>
                            <div className="text-xs font-medium" style={{ color: "var(--yunque-text)" }}>从 Chat 导入 Patch Draft</div>
                            <div className="mt-1 text-[11px]" style={{ color: "var(--yunque-text-muted)" }}>
                              Patch Draft 可以携带单个文件的新内容；载入后仍只进入 diff 预览框，不会自动应用。
                            </div>
                          </div>
                          {importedPatchDraft && (
                            <Chip size="sm" color={importedPatchDraftMatchesWorkspace ? "success" : "warning"}>
                              {importedPatchDraftMatchesWorkspace ? "Draft 工作区匹配" : "Draft 待确认"}
                            </Chip>
                          )}
                        </div>
                        <TextArea
                          aria-label="导入 Patch Draft JSON"
                          value={importedPatchDraftText}
                          onChange={(event) => setImportedPatchDraftText(event.target.value)}
                          rows={4}
                        >
                          <Label>Patch Draft JSON 或 Chat 消息</Label>
                        </TextArea>
                        {importedPatchDraftText.trim() && !importedPatchDraft && !importedPatchDraftRequest && (
                          <div className="mt-2 rounded px-2 py-1 text-[11px]" style={{ background: "rgba(248,113,113,0.08)", color: "var(--yunque-danger)" }}>
                            未识别到 yunque.pack_studio.patch_draft.v1。Patch Draft 必须包含 file_path 和 content。
                          </div>
                        )}
                        {!importedPatchDraft && importedPatchDraftRequest && (
                          <div className="mt-3 rounded-md border p-2" style={{ borderColor: "rgba(245,158,11,0.35)", background: "rgba(245,158,11,0.08)" }}>
                            <div className="mb-2 flex flex-wrap items-center justify-between gap-2">
                              <div>
                                <div className="text-xs font-medium" style={{ color: "var(--yunque-text)" }}>这是 Patch Draft Request，还不是可导入 Draft</div>
                                <div className="mt-1 text-[11px]" style={{ color: "var(--yunque-text-muted)" }}>
                                  Request 只告诉小羽要生成什么文件；生成出的 {importedPatchDraftRequest.expectedKind || "yunque.pack_studio.patch_draft.v1"} 才能载入 diff 预览。
                                </div>
                              </div>
                              <Chip size="sm" color={importedPatchDraftRequestMatchesWorkspace ? "success" : "warning"}>
                                {importedPatchDraftRequestMatchesWorkspace ? "Request 工作区匹配" : "Request 待确认"}
                              </Chip>
                            </div>
                            {!importedPatchDraftRequestMatchesWorkspace && (
                              <div className="mb-2 rounded px-2 py-1 text-[11px]" style={{ background: "rgba(245,158,11,0.12)", color: "var(--yunque-warning)" }}>
                                这个 Request 的工作区或原始 SHA 与当前工作区不一致。请先确认你正在处理同一个 yqpack。
                              </div>
                            )}
                            <div className="flex flex-wrap items-center justify-between gap-2">
                              <div className="min-w-0">
                                <div className="truncate font-mono text-[11px]" style={{ color: "var(--yunque-text-muted)" }}>{importedPatchDraftRequest.target.filePath}</div>
                                <div className="mt-1 flex flex-wrap gap-1">
                                  {importedPatchDraftRequest.target.riskLevel && <Chip size="sm" variant="soft">风险：{importedPatchDraftRequest.target.riskLevel}</Chip>}
                                  <Chip size="sm" variant="soft">starter {importedPatchDraftRequest.starterContentLength.toLocaleString()} chars</Chip>
                                  {importedPatchDraftRequest.target.gates.map((gate) => (
                                    <Chip key={`draft-request-import:${gate}`} size="sm" variant="soft">{gate}</Chip>
                                  ))}
                                </div>
                              </div>
                              <Link href={`/chat?q=${encodeURIComponent(importedPatchDraftText)}`}>
                                <Button size="sm" className="btn-accent">
                                  <ArrowRight size={13} /> 交给 Chat 生成 Draft
                                </Button>
                              </Link>
                            </div>
                            {importedPatchDraftRequest.target.reason && (
                              <div className="mt-2 text-[11px]" style={{ color: "var(--yunque-text-muted)" }}>原因：{importedPatchDraftRequest.target.reason}</div>
                            )}
                          </div>
                        )}
                        {importedPatchDraft && (
                          <div className="mt-3 rounded-md border p-2" style={{ borderColor: "var(--yunque-border)" }}>
                            {!importedPatchDraftMatchesWorkspace && (
                              <div className="mb-2 rounded px-2 py-1 text-[11px]" style={{ background: "rgba(245,158,11,0.10)", color: "var(--yunque-warning)" }}>
                                这个 Patch Draft 的工作区或原始 SHA 与当前工作区不一致。请重新从当前工作区生成草稿。
                              </div>
                            )}
                            <div className="flex flex-wrap items-center justify-between gap-2">
                              <div className="min-w-0">
                                <div className="truncate font-mono text-[11px]" style={{ color: "var(--yunque-text-muted)" }}>{importedPatchDraft.filePath}</div>
                                <div className="mt-1 flex flex-wrap gap-1">
                                  {importedPatchDraft.riskLevel && <Chip size="sm" variant="soft">风险：{importedPatchDraft.riskLevel}</Chip>}
                                  <Chip size="sm" variant="soft">{importedPatchDraft.content.length.toLocaleString()} chars</Chip>
                                  {importedPatchDraft.gates.map((gate) => (
                                    <Chip key={`draft:${gate}`} size="sm" variant="soft">{gate}</Chip>
                                  ))}
                                </div>
                              </div>
                              <Button size="sm" variant="outline" onPress={fillImportedPatchDraft} isDisabled={!importedPatchDraftMatchesWorkspace}>
                                <Sparkles size={13} /> 载入 Draft
                              </Button>
                            </div>
                            {importedPatchDraft.reason && (
                              <div className="mt-2 text-[11px]" style={{ color: "var(--yunque-text-muted)" }}>原因：{importedPatchDraft.reason}</div>
                            )}
                          </div>
                        )}
                      </div>
                    </div>
                    {draftCandidates.length > 0 && (
                      <div id="draft-queue" className="mt-3 scroll-mt-24 rounded-md p-2" style={{ background: "var(--yunque-bg-hover)" }}>
                        <div className="mb-2 flex flex-wrap items-center justify-between gap-2">
                          <div className="text-xs font-medium" style={{ color: "var(--yunque-text)" }}>小羽改造草稿队列</div>
                          <Button size="sm" variant="ghost" onPress={copyDraftPlan}>
                            <Copy size={13} /> 复制 Patch Plan JSON
                          </Button>
                        </div>
                        <div className="mb-2 text-[11px]" style={{ color: "var(--yunque-text-muted)" }}>
                          结构化计划只包含目标文件、风险、原因、门禁和内容摘要；真正内容仍需载入草稿后预览 diff。
                        </div>
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
                                <div className="flex flex-wrap gap-1">
                                  <Button size="sm" variant="outline" onPress={() => fillDraftCandidate(candidate)} isDisabled={!candidate.applyable}>
                                    <Sparkles size={13} /> 载入草稿
                                  </Button>
                                  <Button size="sm" variant="ghost" onPress={() => copyPatchDraftRequest(candidate)} isDisabled={!candidate.applyable}>
                                    <Copy size={13} /> 复制 Draft 请求
                                  </Button>
                                  <Link href={`/chat?q=${encodeURIComponent(buildPatchDraftRequestPrompt(prompt, workspaceReport, candidate, goal))}`}>
                                    <Button size="sm" variant="ghost" isDisabled={!candidate.applyable}>
                                      <ArrowRight size={13} /> 交给小羽生成 Draft
                                    </Button>
                                  </Link>
                                </div>
                              </div>
                              <div className="mt-1 truncate font-mono text-[11px]" style={{ color: "var(--yunque-text-muted)" }}>{candidate.filePath}</div>
                              <div className="mt-1 text-[11px]" style={{ color: "var(--yunque-text-muted)" }}>{candidate.summary}</div>
                              <div className="mt-1 text-[11px]" style={{ color: "var(--yunque-text-muted)" }}>原因：{candidate.reason}</div>
                              {candidate.readinessGaps.length > 0 && (
                                <div className="mt-2 flex flex-wrap gap-1">
                                  {candidate.readinessGaps.map((gap) => (
                                    <Chip key={`${candidate.key}:gap:${gap}`} size="sm" color="warning">补：{gap}</Chip>
                                  ))}
                                </div>
                              )}
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
