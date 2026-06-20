#!/usr/bin/env node
// Audits official capability packs for user-visible usefulness signals.
// It does not prove a pack is feature-complete; it catches the common "there
// is a menu, but no clear action or destination" failure mode.
import { existsSync, mkdirSync, readFileSync, readdirSync, writeFileSync } from "node:fs";
import { dirname, join, relative, resolve, sep } from "node:path";

const repoRoot = resolve(import.meta.dirname, "..");
const argv = parseArgs(process.argv.slice(2));
const officialDir = resolveArgPath(argv["source-dir"] || "packs/official");
const appDir = resolveArgPath(argv["app-dir"] || "apps/web/src/app");
const surfaceGuidanceFile = resolveArgPath(argv["surface-guidance"] || "apps/web/src/lib/pack-surface-guidance.ts");
const strict = argv.strict === true;
const jsonReport = argv["json-report"] === true;
const outputPath = typeof argv.output === "string" ? resolveArgPath(argv.output) : "";
const surfaceGuidance = existsSync(surfaceGuidanceFile) ? readFileSync(surfaceGuidanceFile, "utf8") : "";

const manifests = findPackManifests(officialDir)
  .map((manifestPath) => ({ manifestPath, manifest: readJson(manifestPath) }))
  .sort((a, b) => String(a.manifest.id).localeCompare(String(b.manifest.id)));

const rows = manifests.map(auditPack);
const groups = groupBy(rows, (row) => row.grade);
const issueCounts = {};
for (const row of rows) {
  for (const issue of row.issues) issueCounts[issue.code] = (issueCounts[issue.code] || 0) + 1;
}
const report = buildReport(rows, groups, issueCounts);

if (outputPath) {
  mkdirSync(dirname(outputPath), { recursive: true });
  writeFileSync(outputPath, `${JSON.stringify(report, null, 2)}\n`);
}

if (jsonReport) {
  console.log(JSON.stringify(report, null, 2));
} else {
  console.log(JSON.stringify(report.summary, null, 2));

  for (const row of rows) {
    const issues = row.issues.map((issue) => issue.code).join(",") || "-";
    console.log([
      row.grade,
      row.id,
      `status=${row.status || "-"}`,
      `visible=${row.userVisible}`,
      `paths=${row.entryPaths.length}`,
      `examples=${row.examples.length}`,
      `api=${row.backendApiCount}`,
      `page=${row.hasConcretePage}`,
      `usability=${row.usability || "-"}`,
      `primary=${row.primaryActionPath || "-"}`,
      `issues=${issues}`,
    ].join("\t"));
  }
}

const blocking = rows.flatMap((row) => row.issues.filter((issue) => issue.blocking).map((issue) => `${row.id}: ${issue.code} - ${issue.message}`));
if (strict && blocking.length > 0) {
  console.error(`\n[pack-usability] ${blocking.length} blocking issue(s):`);
  for (const item of blocking) console.error(`  - ${item}`);
  process.exit(1);
}

function buildReport(rows, groups, issueCounts) {
  const items = rows.map((row) => {
    const priority = polishPriority(row);
    const open = row.primaryActionPath || row.entryPaths[0] || null;
    const missing = row.issues.map((issue) => issue.code);
    return {
      id: row.id,
      name: row.name,
      status: row.status || "",
      manifest_path: row.manifestPath,
      grade: row.grade,
      priority,
      user_visible: row.userVisible,
      usability: row.usability || "",
      primary_action_path: row.primaryActionPath || "",
      entry_paths: row.entryPaths,
      examples: row.examples,
      backend_api_count: row.backendApiCount,
      has_concrete_page: row.hasConcretePage,
      uses_runtime_host: row.usesRuntimeHost,
      issues: row.issues.map((issue) => ({
        code: issue.code,
        message: issue.message,
        blocking: issue.blocking,
      })),
      handoff_links: {
        center: `/packs?q=${encodeURIComponent(row.id)}&from=usability-audit`,
        detail: `/packs/detail?id=${encodeURIComponent(row.id)}`,
        open,
        studio: `/packs/studio?packId=${encodeURIComponent(row.id)}&goal=${encodeURIComponent(polishGoal(row, missing))}`,
      },
      next_step: nextStepFor(row, priority, missing),
      verify: verifyStepFor(row, open),
    };
  });
  const queue = items
    .filter((item) => item.priority.level !== "P3")
    .sort((a, b) => a.priority.order - b.priority.order || a.grade.localeCompare(b.grade) || a.name.localeCompare(b.name));

  return {
    kind: "yunque.pack_usability_report.v1",
    generated_at: new Date().toISOString(),
    summary: {
      total: rows.length,
      groups: Object.fromEntries([...groups.entries()].map(([key, value]) => [key, value.length])),
      issueCounts,
      queue: {
        total: queue.length,
        p0: queue.filter((item) => item.priority.level === "P0").length,
        p1: queue.filter((item) => item.priority.level === "P1").length,
        p2: queue.filter((item) => item.priority.level === "P2").length,
      },
    },
    queue,
    packs: items,
  };
}

function polishPriority(row) {
  if (row.issues.some((issue) => issue.blocking)) {
    return {
      level: "P0",
      label: "P0 修硬阻塞",
      order: 0,
      reason: "存在会阻止用户理解、打开或验证能力包的硬性缺口。",
    };
  }
  if (row.grade === "experimental") {
    return {
      level: "P1",
      label: "P1 实验能力补边界",
      order: 1,
      reason: "实验能力可以展示，但必须清楚说明限制、结果位置和转稳定待办。",
    };
  }
  if (row.grade === "infrastructure") {
    return {
      level: "P2",
      label: "P2 后台能力验收",
      order: 2,
      reason: "后台支撑能力不一定单独打开，重点验证它在 Chat、任务、记忆、知识或设置中的效果。",
    };
  }
  return {
    level: "P3",
    label: "P3 常规巡检",
    order: 3,
    reason: "当前已具备入口、示例和可见页面，后续按真实反馈继续打磨。",
  };
}

function polishGoal(row, missing) {
  if (missing.length > 0) {
    return `修复 ${row.name} 的能力包可用性缺口：${missing.join("、")}。不要伪造能力，改完必须回中心、详情和入口验收。`;
  }
  if (row.grade === "experimental") {
    return `打磨 ${row.name} 的实验能力说明：补齐限制、结果位置、验证步骤和转稳定待办，不包装成稳定执行。`;
  }
  if (row.grade === "infrastructure") {
    return `打磨 ${row.name} 的后台支撑说明：说明它在用户主路径哪里生效、如何触发、如何观察结果。`;
  }
  return `继续打磨 ${row.name} 的用户路径、验收出口和回滚说明。`;
}

function nextStepFor(row, priority, missing) {
  if (missing.length > 0) {
    return `先进工坊修复：${missing.join("、")}；再跑 check-pack-usability --strict。`;
  }
  if (priority.level === "P1") {
    return "保留实验边界，补真实结果位置、限制说明、验证步骤和转稳定的最小待办。";
  }
  if (priority.level === "P2") {
    return "从声明的用户感知位置触发一次，确认主路径能看到状态、结果或下一步提示。";
  }
  return "打开入口或从 Chat 主路径触发一次，确认结果、产物、状态或错误提示可见。";
}

function verifyStepFor(row, open) {
  if (open) {
    return `回到 ${open} 复验入口、提示、结果位置和回滚路径。`;
  }
  if (row.grade === "infrastructure") {
    return "从 Chat、任务、记忆、知识或设置流程触发，观察结果、状态或提示是否出现。";
  }
  return "回能力包中心和详情页确认用户能理解如何触发、观察结果和回滚。";
}

function auditPack({ manifestPath, manifest }) {
  const metadata = manifest.metadata || {};
  const menus = manifest.frontend?.menus || [];
  const routes = manifest.frontend?.routes || [];
  const frontendAssetsType = manifest.frontend?.assets?.type || "";
  const routeSpecs = manifest.backend?.routeSpecs || [];
  const backendRoutes = manifest.backend?.routes || [];
  const entryPaths = unique([
    ...menus.map((menu) => menu.path),
    ...routes.map((route) => route.path),
    metadata.primaryActionPath,
  ].filter((value) => typeof value === "string" && value.trim().length > 0));
  const examples = Object.keys(metadata)
    .filter((key) => /^example\d+$/.test(key))
    .map((key) => metadata[key])
    .filter((value) => typeof value === "string" && value.trim().length > 0);
  const backendApiCount = routeSpecs.length + backendRoutes.length;
  const userVisible = entryPaths.length > 0 || metadata.usability === "actionable" || metadata.usability === "experimental";
  const hasConcretePage = entryPaths.some((entryPath) => appRouteExists(entryPath));
  const usesRuntimeHost = frontendAssetsType === "iframe-bundle" || routes.some((route) => String(route.component || "").includes("PackDlcHost"));
  const dedicatedPageProof = readDedicatedPackPageProof(entryPaths);
  const hasActionableSurface = userVisible && examples.length >= 2 && (hasConcretePage || usesRuntimeHost);
  const isBackendSupportPack =
    backendApiCount > 0 &&
    !hasActionableSurface &&
    metadata.usability !== "actionable" &&
    metadata.usability !== "experimental" &&
    manifest.status !== "alpha";
  const issues = [];

  if (userVisible && examples.length < 2) {
    issues.push({
      code: "missing-user-examples",
      message: "user-visible pack needs at least two concrete examples",
      blocking: true,
    });
  }
  if (userVisible && entryPaths.length === 0) {
    issues.push({
      code: "missing-entry-path",
      message: "user-visible pack needs an entry path",
      blocking: true,
    });
  }
  if (userVisible && !hasConcretePage && !usesRuntimeHost) {
    issues.push({
      code: "missing-concrete-page",
      message: "entry path does not map to an app page or runtime host",
      blocking: true,
    });
  }
  if (dedicatedPageProof.needsProof && !dedicatedPageProof.hasUsefulnessCopy) {
    issues.push({
      code: "missing-dedicated-page-usefulness-copy",
      message: "dedicated pack page should explain what the pack is useful for",
      blocking: true,
    });
  }
  if (dedicatedPageProof.needsProof && !dedicatedPageProof.hasActionCopy) {
    issues.push({
      code: "missing-dedicated-page-action-copy",
      message: "dedicated pack page should give the user a visible action or next step",
      blocking: true,
    });
  }
  if (dedicatedPageProof.needsProof && !dedicatedPageProof.hasBoundaryCopy) {
    issues.push({
      code: "missing-dedicated-page-boundary-copy",
      message: "dedicated pack page should state current limitations or safety boundaries",
      blocking: true,
    });
  }
  if (!userVisible && routeSpecs.length === 0 && backendRoutes.length === 0 && metadata.usability !== "infrastructure") {
    issues.push({
      code: "pure-declaration",
      message: "pack has no visible entry and no declared backend routes",
      blocking: true,
    });
  }
  if (isBackendSupportPack && metadata.usability !== "infrastructure") {
    issues.push({
      code: "missing-infrastructure-usability",
      message: "backend support pack should declare metadata.usability=infrastructure",
      blocking: true,
    });
  }
  if (isBackendSupportPack && examples.length < 2) {
    issues.push({
      code: "missing-infrastructure-examples",
      message: "backend support pack needs at least two examples of where users feel its effect",
      blocking: true,
    });
  }
  if (isBackendSupportPack && metadata.internalOnly !== "true" && !metadata.primaryActionPath) {
    issues.push({
      code: "missing-infrastructure-primary-path",
      message: "backend support pack needs a primary action path to its consuming surface",
      blocking: true,
    });
  }
  if (isBackendSupportPack && metadata.internalOnly !== "true" && !metadata.primaryActionLabel) {
    issues.push({
      code: "missing-infrastructure-primary-label",
      message: "backend support pack needs a user-readable primary action label",
      blocking: true,
    });
  }
  if (metadata.usability === "infrastructure" && metadata.internalOnly !== "true" && !metadata.usageSurface) {
    issues.push({
      code: "missing-infrastructure-usage-surface",
      message: "infrastructure pack should describe where users feel its effect",
      blocking: true,
    });
  }
  if (
    metadata.usability === "infrastructure" &&
    metadata.internalOnly !== "true" &&
    !surfaceGuidance.includes(String(manifest.id))
  ) {
    issues.push({
      code: "missing-surface-guidance",
      message: "infrastructure pack should be explained on its consuming user surface",
      blocking: true,
    });
  }
  if (isActionableUserPack({ manifest, metadata, userVisible, hasConcretePage, usesRuntimeHost, isBackendSupportPack })) {
    if (metadata.usability !== "actionable") {
      issues.push({
        code: "missing-actionable-usability",
        message: "user-actionable pack should declare metadata.usability=actionable",
        blocking: true,
      });
    }
    if (!metadata.primaryActionPath) {
      issues.push({
        code: "missing-actionable-primary-path",
        message: "user-actionable pack needs a primary action path",
        blocking: true,
      });
    }
    if (!metadata.primaryActionLabel) {
      issues.push({
        code: "missing-actionable-primary-label",
        message: "user-actionable pack needs a user-readable primary action label",
        blocking: true,
      });
    }
    if (!metadata.usageSurface) {
      issues.push({
        code: "missing-actionable-usage-surface",
        message: "user-actionable pack should describe where users feel its effect",
        blocking: true,
      });
    }
  }
  if ((manifest.status === "alpha" || metadata.usability === "experimental") && !metadata.primaryActionPath) {
    issues.push({
      code: "missing-experimental-primary-path",
      message: "experimental pack should still tell users where to inspect it",
      blocking: true,
    });
  }
  if ((manifest.status === "alpha" || metadata.usability === "experimental") && !metadata.primaryActionLabel) {
    issues.push({
      code: "missing-experimental-primary-label",
      message: "experimental pack should still have a user-readable inspect action",
      blocking: true,
    });
  }
  if ((manifest.status === "alpha" || metadata.usability === "experimental") && !metadata.limitation) {
    issues.push({
      code: "missing-experimental-limitation",
      message: "experimental pack should state its current limitation",
      blocking: false,
    });
  }
  if (manifest.id === "yunque.pack.mcp-dispatch" && !entryPaths.includes("/workers")) {
    issues.push({
      code: "worker-pack-missing-workers-entry",
      message: "MCP dispatch pack must open the AI IDE / Worker surface at /workers",
      blocking: true,
    });
  }
  if (manifest.id === "yunque.pack.orchestrator" && !entryPaths.includes("/workers")) {
    issues.push({
      code: "orchestrator-pack-missing-workers-entry",
      message: "orchestrator pack must open the AI IDE / Worker surface at /workers",
      blocking: true,
    });
  }

  return {
    id: manifest.id,
    name: manifest.name,
    status: manifest.status || "",
    manifestPath: slash(relative(repoRoot, manifestPath)),
    grade: gradePack({ manifest, userVisible, examples, hasConcretePage, usesRuntimeHost, routeSpecs, backendRoutes, issues }),
    usability: metadata.usability || "",
    primaryActionPath: metadata.primaryActionPath || "",
    userVisible,
    entryPaths,
    examples,
    routeSpecCount: routeSpecs.length,
    backendApiCount,
    hasConcretePage,
    usesRuntimeHost,
    issues,
  };
}

function gradePack({ manifest, userVisible, examples, hasConcretePage, usesRuntimeHost, routeSpecs, backendRoutes, issues }) {
  if (issues.some((issue) => issue.blocking)) return "needs-work";
  if (manifest.metadata?.usability === "infrastructure" || (!userVisible && (routeSpecs.length > 0 || backendRoutes.length > 0))) return "infrastructure";
  if (manifest.status === "alpha" || manifest.metadata?.usability === "experimental") return "experimental";
  if (userVisible && examples.length >= 2 && (hasConcretePage || usesRuntimeHost)) return "actionable";
  return "documented";
}

function isActionableUserPack({ manifest, metadata, userVisible, hasConcretePage, usesRuntimeHost, isBackendSupportPack }) {
  if (metadata.usability === "experimental" || manifest.status === "alpha") return false;
  if (metadata.usability === "infrastructure" || isBackendSupportPack) return false;
  if (metadata.usability === "actionable") return true;
  return userVisible && (hasConcretePage || usesRuntimeHost);
}

function appRouteExists(routePath) {
  return Boolean(appPagePath(routePath));
}

function appPagePath(routePath) {
  if (typeof routePath !== "string" || !routePath.startsWith("/")) return false;
  const pathname = stripQueryAndHash(routePath);
  const segments = pathname.split("/").filter(Boolean);
  const pagePath = join(appDir, ...segments, "page.tsx");
  return existsSync(pagePath) ? pagePath : "";
}

function readDedicatedPackPageProof(entryPaths) {
  const pagePaths = unique(entryPaths
    .map((entryPath) => ({ entryPath, pagePath: appPagePath(entryPath) }))
    .filter(({ entryPath, pagePath }) => pagePath && stripQueryAndHash(entryPath).startsWith("/packs/"))
    .map(({ pagePath }) => pagePath));

  if (pagePaths.length === 0) {
    return {
      needsProof: false,
      hasUsefulnessCopy: true,
      hasActionCopy: true,
      hasBoundaryCopy: true,
    };
  }

  const content = pagePaths.map((pagePath) => readFileSync(pagePath, "utf8")).join("\n");
  return {
    needsProof: true,
    hasUsefulnessCopy: /这个能力包|能力包能做什么|现在适合做什么|现在能做什么|有什么用/.test(content),
    hasActionCopy: /可以：|导出|导入|查看|生成|运行|打开|编辑|下载|上传|扫描|创建|审批|刷新|搜索|固定|启用|禁用|回滚|计划/.test(content),
    hasBoundaryCopy: /当前不会做什么|当前边界|不会：|不会|限制|边界|实验|alpha|Beta 关闭|当前不执行|仅计划|只读/.test(content),
  };
}

function stripQueryAndHash(routePath) {
  return routePath.split(/[?#]/, 1)[0] || "/";
}

function findPackManifests(root) {
  const out = [];
  walk(root, (path, dirent) => {
    if (dirent.isFile() && dirent.name === "pack.json") out.push(path);
  });
  return out;
}

function walk(dir, visit) {
  for (const entry of readdirSync(dir, { withFileTypes: true })) {
    const path = join(dir, entry.name);
    if (entry.isDirectory()) walk(path, visit);
    else visit(path, entry);
  }
}

function groupBy(items, keyFn) {
  const groups = new Map();
  for (const item of items) {
    const key = keyFn(item);
    groups.set(key, [...(groups.get(key) || []), item]);
  }
  return groups;
}

function unique(values) {
  return [...new Set(values)];
}

function readJson(path) {
  return JSON.parse(readFileSync(path, "utf8"));
}

function parseArgs(args) {
  const out = {};
  for (let i = 0; i < args.length; i++) {
    const arg = args[i];
    if (!arg.startsWith("--")) continue;
    const key = arg.slice(2);
    const next = args[i + 1];
    if (next === undefined || next.startsWith("--")) {
      out[key] = true;
    } else {
      out[key] = next;
      i++;
    }
  }
  return out;
}

function resolveArgPath(path) {
  return resolve(repoRoot, String(path));
}

function slash(path) {
  return path.split(sep).join("/");
}
