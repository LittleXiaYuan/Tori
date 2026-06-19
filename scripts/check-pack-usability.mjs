#!/usr/bin/env node
// Audits official capability packs for user-visible usefulness signals.
// It does not prove a pack is feature-complete; it catches the common "there
// is a menu, but no clear action or destination" failure mode.
import { existsSync, readFileSync, readdirSync } from "node:fs";
import { dirname, join, relative, resolve, sep } from "node:path";

const repoRoot = resolve(import.meta.dirname, "..");
const argv = parseArgs(process.argv.slice(2));
const officialDir = resolveArgPath(argv["source-dir"] || "packs/official");
const appDir = resolveArgPath(argv["app-dir"] || "apps/web/src/app");
const strict = argv.strict === true;

const manifests = findPackManifests(officialDir)
  .map((manifestPath) => ({ manifestPath, manifest: readJson(manifestPath) }))
  .sort((a, b) => String(a.manifest.id).localeCompare(String(b.manifest.id)));

const rows = manifests.map(auditPack);
const groups = groupBy(rows, (row) => row.grade);
const issueCounts = {};
for (const row of rows) {
  for (const issue of row.issues) issueCounts[issue.code] = (issueCounts[issue.code] || 0) + 1;
}

console.log(JSON.stringify({
  total: rows.length,
  groups: Object.fromEntries([...groups.entries()].map(([key, value]) => [key, value.length])),
  issueCounts,
}, null, 2));

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

const blocking = rows.flatMap((row) => row.issues.filter((issue) => issue.blocking).map((issue) => `${row.id}: ${issue.code} - ${issue.message}`));
if (strict && blocking.length > 0) {
  console.error(`\n[pack-usability] ${blocking.length} blocking issue(s):`);
  for (const item of blocking) console.error(`  - ${item}`);
  process.exit(1);
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
  if (typeof routePath !== "string" || !routePath.startsWith("/")) return false;
  const segments = routePath.split("/").filter(Boolean);
  const pagePath = join(appDir, ...segments, "page.tsx");
  return existsSync(pagePath);
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
