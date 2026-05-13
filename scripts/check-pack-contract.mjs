import { existsSync, readdirSync, readFileSync } from "node:fs";
import { spawnSync } from "node:child_process";
import { join, relative, resolve, sep } from "node:path";

const repoRoot = resolve(import.meta.dirname, "..");
const failures = [];

function fail(message) {
  failures.push(message);
}

function readText(path) {
  const fullPath = resolve(repoRoot, path);
  if (!existsSync(fullPath)) {
    fail(`missing file: ${path}`);
    return "";
  }
  return readFileSync(fullPath, "utf8");
}

function readJSON(path) {
  const text = readText(path);
  if (!text) return undefined;
  try {
    return JSON.parse(text);
  } catch (error) {
    fail(`invalid json: ${path}: ${error.message}`);
    return undefined;
  }
}

function walk(dir) {
  const fullDir = resolve(repoRoot, dir);
  if (!existsSync(fullDir)) return [];
  const out = [];
  for (const entry of readdirSync(fullDir, { withFileTypes: true })) {
    const fullPath = join(fullDir, entry.name);
    const rel = relative(repoRoot, fullPath).split(sep).join("/");
    if (entry.isDirectory()) out.push(...walk(rel));
    else out.push(rel);
  }
  return out;
}

const authoring = readText("packs/AUTHORING.md");
const englishGuide = readText("docs/guide/pack-runtime.md") + "\n" + readText("docs/guide/pack-runtime-state.md");
const chineseGuide = readText("docs/zh/guide/pack-runtime.md") + "\n" + readText("docs/zh/guide/pack-runtime-state.md");
const docsConfig = readText("docs/.vitepress/config.ts");
const scaffoldScript = readText("scripts/scaffold-pack.mjs");
const scaffoldCheck = readText("scripts/check-pack-scaffold.mjs");
const completionCheck = readText("scripts/check-pack-runtime-completion.mjs");
for (const token of [
  "Pack Authoring Contract",
  "packruntime.BackendModule",
  "GatewayConfig.BackendPacks",
  "RegisterBackendPack",
  "/v1/packs/enabled",
  "frontend.menus",
  "frontend.routes",
  "sdk.typescript",
  "scripts/scaffold-pack.mjs",
]) {
  if (!authoring.includes(token)) fail(`AUTHORING.md missing token: ${token}`);
}

for (const [name, text] of [["docs/guide/pack-runtime.md", englishGuide], ["docs/zh/guide/pack-runtime.md", chineseGuide]]) {
  for (const token of ["Pack Runtime", "packruntime.BackendModule", "frontend.menus", "sdk.typescript", "check-pack-contract.mjs", "scaffold-pack.mjs"]) {
    if (!text.includes(token)) fail(`${name} missing token: ${token}`);
  }
}

for (const token of ["packs/examples", "internal/packs", "heroui-web/src/app/packs", "update: { channel: \"stable\", rollback: true }", "sdk: { typescript: sdk }", "distribution:", "packageUrl", "frontendUrl", "sha256", "RegisterBackendPack", "--dry-run", "--json", "dryRun", "jsonOutput"]) {
  if (!scaffoldScript.includes(token)) fail(`scaffold-pack.mjs missing token: ${token}`);
}
for (const token of ["verifier-pack", "--dry-run", "--json", "manifest.frontend.menus", "manifest.frontend.routes", "manifest.sdk.typescript", "manifest.distribution.packageUrl", "manifest.distribution.frontendUrl", "manifest.distribution.sha256", "manifest.update.rollback"]) {
  if (!scaffoldCheck.includes(token)) fail(`check-pack-scaffold.mjs missing token: ${token}`);
}
for (const token of ["Pack Runtime completion audit", "RegisterBackendPack", "frontendSync()", "PruneArtifacts", "TypeScript packs SDK", "backup-pack 示例包"]) {
  if (!completionCheck.includes(token)) fail(`check-pack-runtime-completion.mjs missing token: ${token}`);
}
const scaffoldCheckResult = spawnSync(process.execPath, ["scripts/check-pack-scaffold.mjs"], { cwd: repoRoot, encoding: "utf8" });
if (scaffoldCheckResult.status !== 0) {
  fail(`check-pack-scaffold.mjs failed: ${scaffoldCheckResult.stderr || scaffoldCheckResult.stdout}`);
}
if (!process.env.PACK_COMPLETION_AUDIT_CHILD) {
  const completionCheckResult = spawnSync(process.execPath, ["scripts/check-pack-runtime-completion.mjs"], { cwd: repoRoot, encoding: "utf8", env: { ...process.env, PACK_COMPLETION_AUDIT_CHILD: "1" } });
  if (completionCheckResult.status !== 0) {
    fail(`check-pack-runtime-completion.mjs failed: ${completionCheckResult.stderr || completionCheckResult.stdout}`);
  }
}

if (!docsConfig.includes("/guide/pack-runtime") || !docsConfig.includes("/zh/guide/pack-runtime") || !docsConfig.includes("/guide/pack-runtime-state") || !docsConfig.includes("/zh/guide/pack-runtime-state")) {
  fail("docs vitepress config must expose Pack Runtime guide and state pages in both locales");
}

const gatewaySource = readText("internal/controlplane/gateway/handlers_packs.go")
  + "\n"
  + readText("internal/controlplane/gateway/gateway.go")
  + "\n"
  + readText("internal/controlplane/gateway/gateway_setters.go")
  + "\n"
  + readText("internal/controlplane/gateway/handlers_packs_test.go");
for (const token of ["BackendPacks", "RegisterBackendPack", "registerBackendPack", "requirePackRoute", "backendPackRoutes", "backendPackRouteInfos", "BackendRouteInfo{Method", "route.Method = strings.ToUpper", "must declare an HTTP method", "handlePackBackendModules", "handlePackPrune", "/v1/packs/prune", "Download     bool", "CacheDistribution", "PruneArtifacts", "InstallWithArtifacts", "route conflict", "TestRegisterBackendPackMountsModuleAfterGatewayConstruction", "TestRegisterBackendPackIsIdempotentForSamePackRoute", "TestRegisterBackendPackPanicsOnRouteConflict", "TestRegisterBackendPackPanicsOnMissingRouteMethod", "TestPackBackendModulesExposeMountedRoutes", "expected mounted route method to be preserved", "expected downloaded artifacts to be recorded"]) {
  if (!gatewaySource.includes(token)) fail(`gateway pack registration missing token: ${token}`);
}
if (/must be called before Gateway routes are registered/.test(gatewaySource)) {
  fail("RegisterBackendPack must remain usable after Gateway construction");
}

const backendContract = readText("pkg/packruntime/backend.go") + "\n" + readText("pkg/packruntime/registry.go");
for (const token of ["type BackendRoute", "Method  string", "Path    string", "type BackendRouteInfo", "Method string `json:\"method,omitempty\"`", "type BackendModuleInfo", "type BackendModule", "PackID() string", "Routes() []BackendRoute"]) {
  if (!backendContract.includes(token)) fail(`packruntime backend contract missing token: ${token}`);
}

const packFiles = [
  ...walk("packs/examples").filter((path) => path.endsWith("/pack.json")),
  ...walk("packs/templates").filter((path) => path.endsWith("/pack.json")),
].sort();

if (packFiles.length === 0) {
  fail("no pack manifests found under packs/examples or packs/templates");
}

for (const path of packFiles) {
  const manifest = readJSON(path);
  if (!manifest) continue;
  if (!manifest.id || !/^yunque\.pack\.[a-z0-9][a-z0-9.-]*$/.test(manifest.id)) {
    fail(`${path}: id must use yunque.pack.<name> format`);
  }
  if (!manifest.name) fail(`${path}: name is required`);
  if (!manifest.version) fail(`${path}: version is required`);
  if (manifest.defaultState && !["enabled", "disabled"].includes(manifest.defaultState)) {
    fail(`${path}: defaultState must be enabled or disabled`);
  }
  const routes = manifest.backend?.routes ?? [];
  if (!Array.isArray(routes) || routes.length === 0) fail(`${path}: backend.routes must not be empty`);
  for (const route of routes) {
    if (typeof route !== "string" || !route.startsWith("/")) fail(`${path}: invalid backend route: ${route}`);
  }
  const menus = manifest.frontend?.menus ?? [];
  if (!Array.isArray(menus) || menus.length === 0) fail(`${path}: frontend.menus must not be empty`);
  const frontendRoutes = manifest.frontend?.routes ?? [];
  const frontendRoutePaths = new Set(Array.isArray(frontendRoutes) ? frontendRoutes.map((route) => route?.path).filter(Boolean) : []);
  for (const [index, menu] of menus.entries()) {
    for (const key of ["key", "label", "path"]) {
      if (!menu?.[key]) fail(`${path}: frontend.menus[${index}].${key} is required`);
    }
    if (menu?.path && !String(menu.path).startsWith("/packs/")) {
      fail(`${path}: frontend.menus[${index}].path must live under /packs/ to keep pack UI out of the main shell`);
    }
    if (menu?.path && !frontendRoutePaths.has(menu.path)) {
      fail(`${path}: frontend.menus[${index}].path must have a matching frontend.routes entry`);
    }
  }
  if (!Array.isArray(frontendRoutes) || frontendRoutes.length === 0) fail(`${path}: frontend.routes must not be empty`);
  for (const [index, route] of frontendRoutes.entries()) {
    if (!route?.path) fail(`${path}: frontend.routes[${index}].path is required`);
    if (route?.path && !String(route.path).startsWith("/packs/")) {
      fail(`${path}: frontend.routes[${index}].path must live under /packs/ to keep pack UI out of the main shell`);
    }
    if (!route?.component) fail(`${path}: frontend.routes[${index}].component is required`);
  }
  if (!manifest.sdk?.typescript) fail(`${path}: sdk.typescript is required for lightweight external callers`);
  if (!manifest.distribution?.packageUrl) fail(`${path}: distribution.packageUrl is required for downloadable incremental packs`);
  if (!manifest.distribution?.frontendUrl) fail(`${path}: distribution.frontendUrl is required for frontend synchronized updates`);
  if (!manifest.distribution?.sha256) fail(`${path}: distribution.sha256 is required for package verification`);
  if (manifest.update?.rollback !== true) fail(`${path}: update.rollback should be true for reversible pack updates`);
}

const backupManifest = readJSON("packs/examples/backup-pack/pack.json");
const backupSource = readText("internal/packs/backup/handler.go");
if (backupManifest) {
  if (!backupSource.includes(`const PackID = "${backupManifest.id}"`)) {
    fail("backup pack handler PackID must match packs/examples/backup-pack/pack.json");
  }
  for (const route of backupManifest.backend?.routes ?? []) {
    if (!backupSource.includes(route)) fail(`backup handler missing route declared in manifest: ${route}`);
  }
}

if (failures.length > 0) {
  console.error("Pack contract check failed:");
  for (const failure of failures) console.error(`- ${failure}`);
  process.exit(1);
}

console.log(`Pack contract ok: ${packFiles.length} manifests verified, backend module registry contract documented`);
